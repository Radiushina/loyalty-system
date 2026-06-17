package order

import (
	"context"
	"sync"
	"time"

	"github.com/Radiushina/loyalty-system/pkg/accrualclient"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const defaultRateLimitBackoff = 60 * time.Second

// OrderJob — задача на опрос одного заказа во внешней системе.
type OrderJob struct {
	ID     uuid.UUID
	UserID uuid.UUID
	Number string
	Status Status
}

// AccrualEnqueuer ставит заказ в очередь на проверку.
type AccrualEnqueuer interface {
	Enqueue(job OrderJob)
}

// WorkerRepoProvider — методы репозитория для воркера.
type WorkerRepoProvider interface {
	SelectPendingOrders(ctx context.Context, limit int) ([]Order, error)
	GetOrderByID(ctx context.Context, orderID uuid.UUID) (Order, error)
	UpdateOrderAccrual(ctx context.Context, orderID uuid.UUID, status Status, accrual float64) error
}

// BalanceProvider — начисление баллов при PROCESSED.
type BalanceProvider interface {
	CreditAccrual(ctx context.Context, userID, orderID uuid.UUID, amount float64) error
}

// WorkerPoolConfig — настройки пула воркеров.
type WorkerPoolConfig struct {
	Workers      int
	PollInterval time.Duration
	PollBatch    int
}

// AccrualWorkerPool — пул воркеров опроса системы расчёта.
//
// Гибридная схема:
//   - после успешного InsertOrder заказ сразу попадает в очередь (Enqueue);
//   - фоновый scanner периодически подбирает NEW/PROCESSING из БД (рестарт, повторный опрос).
type AccrualWorkerPool struct {
	cfg      WorkerPoolConfig
	repo     WorkerRepoProvider
	client   accrualclient.Provider
	balance  BalanceProvider
	log      *zap.Logger
	jobs     chan OrderJob
	inflight sync.Map

	rateLimitMu    sync.Mutex
	rateLimitUntil time.Time
}

func NewAccrualWorkerPool(
	cfg WorkerPoolConfig,
	repo WorkerRepoProvider,
	client accrualclient.Provider,
	balance BalanceProvider,
	log *zap.Logger,
) *AccrualWorkerPool {
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 10 * time.Second
	}
	if cfg.PollBatch <= 0 {
		cfg.PollBatch = cfg.Workers * 2
	}

	queueSize := cfg.Workers * 4
	if queueSize < cfg.PollBatch {
		queueSize = cfg.PollBatch
	}

	return &AccrualWorkerPool{
		cfg:     cfg,
		repo:    repo,
		client:  client,
		balance: balance,
		log:     log,
		jobs:    make(chan OrderJob, queueSize),
	}
}

// Run запускает N воркеров и фоновый scanner до отмены ctx.
func (p *AccrualWorkerPool) Run(ctx context.Context) {
	for i := 0; i < p.cfg.Workers; i++ {
		go p.worker(ctx, i)
	}
	go p.scanner(ctx)

	<-ctx.Done()
}

// Enqueue ставит заказ в очередь сразу после успешного сохранения в БД.
func (p *AccrualWorkerPool) Enqueue(job OrderJob) {
	p.tryEnqueue(job)
}

func (p *AccrualWorkerPool) scanner(ctx context.Context) {
	ticker := time.NewTicker(p.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.scanPending(ctx)
		}
	}
}

func (p *AccrualWorkerPool) scanPending(ctx context.Context) {
	orders, err := p.repo.SelectPendingOrders(ctx, p.cfg.PollBatch)
	if err != nil {
		p.log.Error("select pending orders", zap.Error(err))
		return
	}

	for _, o := range orders {
		p.tryEnqueue(orderToJob(o))
	}
}

func (p *AccrualWorkerPool) worker(ctx context.Context, workerID int) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-p.jobs:
			p.processJob(ctx, workerID, job)
		}
	}
}

func (p *AccrualWorkerPool) tryEnqueue(job OrderJob) bool {
	if _, loaded := p.inflight.LoadOrStore(job.ID, struct{}{}); loaded {
		return false
	}

	select {
	case p.jobs <- job:
		return true
	default:
		p.inflight.Delete(job.ID)
		return false
	}
}

func (p *AccrualWorkerPool) processJob(ctx context.Context, workerID int, job OrderJob) {
	defer p.inflight.Delete(job.ID)

	p.waitRateLimit(ctx)

	outcome, info, retryAfter, err := PollOrder(ctx, p.client, job.Number)
	if err != nil {
		p.log.Warn("poll accrual system",
			zap.Int("worker", workerID),
			zap.String("order", job.Number),
			zap.Error(err),
		)
	}

	switch outcome {
	case AccrualPollRateLimited:
		p.setRateLimit(retryAfter)
		p.tryEnqueue(job)
		return

	case AccrualPollNotRegistered, AccrualPollTransientError:
		return

	case AccrualPollUpdated:
		if err := p.applyAccrualUpdate(ctx, job, info); err != nil {
			p.log.Error("apply accrual update",
				zap.Int("worker", workerID),
				zap.String("order", job.Number),
				zap.Error(err),
			)
		}
	}
}

func (p *AccrualWorkerPool) applyAccrualUpdate(ctx context.Context, job OrderJob, info accrualclient.OrderInfo) error {
	current, err := p.repo.GetOrderByID(ctx, job.ID)
	if err != nil {
		return err
	}

	updated := current
	if err := ApplyAccrualInfo(&updated, info); err != nil {
		return err
	}

	if updated.Status == current.Status && updated.Accrual == current.Accrual {
		return nil
	}

	if err := p.repo.UpdateOrderAccrual(ctx, job.ID, updated.Status, updated.Accrual); err != nil {
		return err
	}

	if p.balance != nil && updated.Status == Processed && updated.Accrual > 0 {
		if err := p.balance.CreditAccrual(ctx, job.UserID, job.ID, updated.Accrual); err != nil {
			return err
		}
	}

	p.log.Info("order accrual updated",
		zap.String("order", job.Number),
		zap.String("status", string(updated.Status)),
		zap.Float64("accrual", updated.Accrual),
	)

	// PROCESSING — нужен повторный опрос; NEW после REGISTERED тоже может смениться.
	if updated.Status == New || updated.Status == Processing {
		p.tryEnqueue(OrderJob{
			ID:     job.ID,
			UserID: job.UserID,
			Number: job.Number,
			Status: updated.Status,
		})
	}

	return nil
}

func (p *AccrualWorkerPool) waitRateLimit(ctx context.Context) {
	p.rateLimitMu.Lock()
	until := p.rateLimitUntil
	p.rateLimitMu.Unlock()

	wait := time.Until(until)
	if wait <= 0 {
		return
	}

	p.log.Warn("accrual rate limit pause", zap.Duration("wait", wait))

	select {
	case <-ctx.Done():
	case <-time.After(wait):
	}
}

func (p *AccrualWorkerPool) setRateLimit(retryAfter time.Duration) {
	if retryAfter <= 0 {
		retryAfter = defaultRateLimitBackoff
	}

	p.rateLimitMu.Lock()
	p.rateLimitUntil = time.Now().Add(retryAfter)
	p.rateLimitMu.Unlock()
}

func orderToJob(o Order) OrderJob {
	return OrderJob{
		ID:     o.ID,
		UserID: o.UserID,
		Number: o.Number,
		Status: o.Status,
	}
}
