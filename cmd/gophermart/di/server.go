package di

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"
)

type Server struct {
	http *http.Server
}

func NewServer(httpServer *http.Server) *Server {
	return &Server{
		http: httpServer,
	}
}

/**
Внутри Start снова запускается ещё одна горутина для ListenAndServe. Получается:

горутина 1 (из Run):  вся функция Start()
│
├─ горутина 2: ListenAndServe()  ← принимает HTTP-запросы
│
└─ основной поток Start: <-ctx.Done() → Shutdown()
*/

func (s *Server) Start(ctx context.Context, done func()) {
	// Сообщаем Run (через sync.WaitGroup): Start закончил работу, можно выходить из Run.
	defer done() // вызовется когда Start полностью выйдет (включая Shutdown)
	//Зачем go func()? Потому что ListenAndServe блокирует. Без горутины мы бы не смогли после него дойти до <-ctx.Done() и Shutdown.
	go func() {
		log.Printf("Starting HTTP server at %s", s.http.Addr)
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server error: %s", err)
			return
		}
		log.Println("HTTP server gracefully stopped")
	}()

	// ctx приходит из main через signal.NotifyContext — при Ctrl+C или SIGTERM контекст отменяется.
	//<-ctx.Done() — «застрять здесь, пока не попросили остановиться».
	//Пока пользователь не остановил приложение, эта строка не идёт дальше. HTTP при этом продолжает работать в горутине 2.
	<-ctx.Done()

	/*
		Shutdown говорит серверу:
		Новые запросы не принимать.
		Текущие дать дописать (ответ уйти клиенту).
		Ждать не дольше 10 секунд (WithTimeout).
		Успех → ListenAndServe в горутине 2 завершается с ErrServerClosed → печатается "gracefully stopped".
		Если за 10 с кто-то висит на долгом запросе → Shutdown вернёт ошибку (часто context deadline exceeded), ты её логируешь.
		context.Background() — отдельный контекст для shutdown: даже если ctx из main уже отменён, у shutdown свой таймер на 10 с.
		defer cancel() — освобождает таймер, чтобы не утекала память.
	*/
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.http.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.http == nil {
		return nil
	}
	return s.http.Shutdown(ctx)
}
