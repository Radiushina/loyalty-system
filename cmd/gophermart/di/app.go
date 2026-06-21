package di

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/Radiushina/loyalty-system/internal/auth"
	"github.com/Radiushina/loyalty-system/internal/balance"
	"github.com/Radiushina/loyalty-system/internal/config"
	"github.com/Radiushina/loyalty-system/internal/logger"
	"github.com/Radiushina/loyalty-system/internal/order"
	"github.com/Radiushina/loyalty-system/internal/user"
	"github.com/Radiushina/loyalty-system/migrations"
	"github.com/Radiushina/loyalty-system/pkg/accrualclient"
	"github.com/Radiushina/loyalty-system/pkg/pgmigrator"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type App struct {
	cfg         *config.Config
	logger      *zap.Logger
	db          *pgxpool.Pool
	server      *Server
	accrualPool *order.AccrualWorkerPool
}

func NewApp() (*App, func(), error) {
	app := &App{}
	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// HTTP уже останавливается в Server.Start (Run ждёт конец Start перед return).
		// Ниже — страховка, если Start не успел вызвать Shutdown.
		if app.server != nil {
			_ = app.server.Shutdown(ctx) // если HTTP ещё не остановили в Start
		}
		if app.db != nil {
			app.db.Close()
		}
		if app.logger != nil {
			_ = app.logger.Sync()
		}
	}
	return app, shutdown, nil
}

// Поочередно инициализируем данные для App
func (a *App) initialize(ctx context.Context) error {
	// 1. Инициализируем Config
	loader := config.NewLoader("")
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	a.cfg = cfg

	// 2. Инициализируем логгер
	zl, err := logger.New("info")
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	a.logger = zl

	dsn := cfg.Storage.DatabaseDSN()
	if err := pgmigrator.MigrateFromEmbeddedFS(migrations.Postgres, "postgres", dsn); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	// 3. Инициализируем БД. Не defer Close здесь — пул живёт до gracefulShutdown в NewApp.
	dbPool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	if err := dbPool.Ping(ctx); err != nil {
		dbPool.Close()
		return fmt.Errorf("ping postgres: %w", err)
	}
	a.db = dbPool
	zl.Info("postgres connected",
		zap.String("host", net.JoinHostPort(cfg.Storage.Host, cfg.Storage.Port)),
		zap.String("database", cfg.Storage.Database),
	)
	zl.Info("accrual system configured", zap.String("address", cfg.Accrual.Address))

	// 4. Инициализируем repo, service, handler
	authTTL, err := time.ParseDuration(cfg.Auth.TTL)
	if err != nil {
		return fmt.Errorf("parse auth ttl: %w", err)
	}

	// repository
	userRepo := user.NewRepository(dbPool)
	tokenProvider := auth.NewJWT(cfg.Auth.Secret, authTTL)
	orderRepo := order.NewRepository(dbPool)
	balanceRepo := balance.NewRepository(dbPool)

	// service
	hasher := user.NewHasher()
	userSvc := user.NewService(userRepo, tokenProvider, hasher)
	balanceSvc := balance.NewService(balanceRepo)

	accrualClient := accrualclient.New(accrualclient.Config{Address: cfg.Accrual.Address})
	pollInterval, err := cfg.Accrual.PollIntervalDuration()
	if err != nil {
		return fmt.Errorf("parse accrual poll interval: %w", err)
	}

	accrualPool := order.NewAccrualWorkerPool(
		order.WorkerPoolConfig{
			Workers:      cfg.Accrual.Workers,
			PollInterval: pollInterval,
		},
		orderRepo,
		accrualClient,
		balanceSvc,
		zl,
	)
	a.accrualPool = accrualPool

	orderSvc := order.NewService(orderRepo, accrualPool)

	// handlers
	userHandler := user.NewHandler(userSvc, zl)
	orderHandler := order.NewHandler(orderSvc, zl)
	balanceHandler := balance.NewHandler(balanceSvc, zl)

	mux := NewMux(tokenProvider, userHandler, orderHandler, balanceHandler)
	httpHandler := logger.LoggingMiddleware(zl, mux)

	// 5. Старт HTTP-сервера
	srv := http.Server{
		Addr:    cfg.Server.RunAddress,
		Handler: httpHandler,
	}

	a.server = NewServer(&srv)
	return nil
}

// Run - запуск App
func (a *App) Run(ctx context.Context) error {
	// инициализируем все
	if err := a.initialize(ctx); err != nil {
		return fmt.Errorf("run app error: %w", err)
	}

	/*
		Run ждёт два события по порядку:
		1) ctx отменён (Ctrl+C) — пора начинать остановку;
		2) Start полностью завершился (Shutdown HTTP уже отработал).

		Без wg.Wait Run выходил бы сразу по ctx.Done(), а Start ещё крутил бы Shutdown в фоне.
	*/
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		// Стартуем воркер
		a.accrualPool.Run(ctx)
	}()

	go func() {
		// done = wg.Done: Start вызовет его в defer, когда выйдет из функции (после Shutdown).
		a.server.Start(ctx, func() { wg.Done() })
	}()

	<-ctx.Done() // сигнал из main: SIGINT / SIGTERM
	wg.Wait()    // ждём остановку HTTP и воркеров

	return nil
}

func NewMux(
	jwt *auth.JWT,
	userHandler *user.Handler,
	orderHandler *order.Handler,
	balanceHandler *balance.Handler,
) http.Handler {
	r := chi.NewRouter()

	r.Post("/api/user/register", userHandler.CreateUser())
	r.Post("/api/user/login", userHandler.AuthUser())

	r.Group(func(r chi.Router) {
		r.Use(user.NewAuthMiddleware(jwt))
		r.Post("/api/user/orders", orderHandler.CreateOrder())
		r.Get("/api/user/orders", orderHandler.GetOrders())
		r.Post("/api/user/balance/withdraw", balanceHandler.WithdrawBalance())
		r.Get("/api/user/balance", balanceHandler.GetBalance())
		r.Get("/api/user/withdrawals", balanceHandler.GetWithdrawals())
	})

	return r
}
