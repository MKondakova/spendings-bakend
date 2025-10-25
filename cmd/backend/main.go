package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	_ "go.uber.org/mock/mockgen/model"

	"eats-backend/internal/application"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	app := application.New()

	err := app.Start(ctx)
	if err != nil {
		log.Fatalf("can't start application: %s", err)
	}

	err = app.HandleGracefulShutdown(ctx, cancel)
	if err != nil {
		log.Fatalln("All systems closed with errors. LastError: ", err)
	}

	log.Println("All systems closed without errors")
}
