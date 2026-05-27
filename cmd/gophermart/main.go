package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Radiushina/Cumulative-loyalty-system/cmd/gophermart/di"
)

func main() {
	// ctx отменится при Ctrl+C / SIGTERM → Run выйдет из ожидания → Start сделает Shutdown → Run вернётся.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	app, gracefulShutdown, err := di.NewApp()
	if err != nil {
		log.Fatalf("error injecting app: %v", err)
	}
	if err := app.Run(ctx); err != nil {
		log.Fatalf("error running app: %v", err)
	}
	// HTTP уже остановлен в Server.Start; здесь — БД, логгер, запасной Shutdown.
	gracefulShutdown()
	log.Println("Gracefully stopped application")
}
