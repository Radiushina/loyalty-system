package di

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Radiushina/Cumulative-loyalty-system/internal/config"
	"github.com/Radiushina/Cumulative-loyalty-system/internal/logger"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type App struct {
	cfg    *config.Config
	logger *zap.Logger
	db     *pgxpool.Pool
	server *Server
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

	// 3. Инициализируем бд
	var dbPool *pgxpool.Pool
	// Когда подключишь PostgreSQL: a.db = dbPool и закрывай пул в gracefulShutdown (NewApp).
	// Не defer Close здесь — иначе пул закроется сразу после initialize, до Run.
	_ = dbPool

	// хендлер
	mux := http.NewServeMux()
	handler := logger.LoggingMiddleware(zl, mux)

	// 4. Стар сервера
	srv := http.Server{
		Addr:    cfg.Server.Address,
		Handler: handler,
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
	wg.Add(1)

	go func() {
		// done = wg.Done: Start вызовет его в defer, когда выйдет из функции (после Shutdown).
		a.server.Start(ctx, wg.Done)
	}()

	<-ctx.Done() // сигнал из main: SIGINT / SIGTERM
	wg.Wait()    // ждём конец Server.Start, только потом return в main

	return nil
}
