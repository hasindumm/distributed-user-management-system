package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
	"user-service/pkg/userclient"

	"github.com/nats-io/nats.go"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("service starting",
		"service", "gateway-service",
	)

	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		logger.Error("failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer nc.Close()

	client := userclient.NewWithConn(nc, 5*time.Second, logger)
	defer client.Close()

	user, err := client.CreateUser(context.Background(), userclient.CreateUserRequest{
		FirstName: "hasindu1",
		LastName:  "muhandiram1",
		Email:     "hasindumuhandira2@gmail.com",
	})
	if err != nil {
		logger.Error("failed to create user", "error", err)
		os.Exit(1)
	}

	logger.Info("user created successfully",
		"user_id", user.UserID,
		"email", user.Email,
		"status", user.Status,
		"created_at", user.CreatedAt,
	)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan

	slog.Info("service shutting down",
		"service", "gateway-service",
	)
}
