package di

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/Radiushina/loyalty-system/internal/auth"
	"github.com/Radiushina/loyalty-system/internal/config"
	"github.com/Radiushina/loyalty-system/internal/logger"
	"github.com/Radiushina/loyalty-system/internal/order"
	"github.com/Radiushina/loyalty-system/internal/user"
	"github.com/go-chi/chi/v5"
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

	// 3. Инициализируем БД. Не defer Close здесь — пул живёт до gracefulShutdown в NewApp.
	dbPool, err := pgxpool.New(ctx, postgresDSN(cfg.Storage))
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	if err := dbPool.Ping(ctx); err != nil {
		dbPool.Close()
		return fmt.Errorf("ping postgres: %w", err)
	}
	a.db = dbPool
	zl.Info("postgres connected",
		zap.String("host", cfg.Storage.Host),
		zap.String("database", cfg.Storage.Database),
	)

	// 4. Инициализируем repo, service, handler
	authTTL, err := time.ParseDuration(cfg.Auth.TTL)
	if err != nil {
		return fmt.Errorf("parse auth ttl: %w", err)
	}

	userRepo := user.NewRepository(dbPool)
	tokenProvider := auth.NewJWT(cfg.Auth.Secret, authTTL)
	userSvc := user.NewService(userRepo, tokenProvider)
	userHandler := user.NewHandler(userSvc, zl)

	orderRepo := order.NewRepository(dbPool)
	orderSvc := order.NewService(orderRepo)
	orderHandler := order.NewHandel(orderSvc, zl)

	mux := NewMux(ctx, tokenProvider, userHandler, orderHandler)

	httpHandler := logger.LoggingMiddleware(zl, mux)

	// 5. Старт HTTP-сервера
	srv := http.Server{
		Addr:    cfg.Server.Address,
		Handler: httpHandler,
	}

	a.server = NewServer(&srv)
	return nil
}

func postgresDSN(c config.PostgresConfig) string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.User, c.Password),
		Host:   net.JoinHostPort(c.Host, c.Port),
		Path:   "/" + c.Database,
	}
	q := u.Query()
	q.Set("sslmode", "disable")
	u.RawQuery = q.Encode()
	return u.String()
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

func NewMux(ctx context.Context, jwt *auth.JWT, uh *user.Handler, oh *order.Handler) http.Handler {
	r := chi.NewRouter()

	r.Post("/api/user/register", uh.CreateUser(ctx))
	r.Post("/api/user/login", uh.GetByLogin(ctx))

	r.Group(func(r chi.Router) {
		r.Use(user.NewAuthMiddleware(jwt))
		r.Post("/api/user/orders", oh.CreateOrder())
		r.Get("/api/user/orders", oh.GetOrders())
	})

	return r
}
