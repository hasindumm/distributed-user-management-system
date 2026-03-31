package main

import (
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"user-service/internal/adapters/natsadaptor"
	"user-service/internal/adapters/postgresadaptor"
	"user-service/internal/service"
	"user-service/pkg/config"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/nats-io/nats.go"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("service starting", "service", "user-service")

	cfg, err := config.Load("config.yaml")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	if cfg.DatabaseURL == "" {
		slog.Error("database_url is not set")
		os.Exit(1)
	}

	// DB
	database, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		slog.Error("error connecting to the database", "error", err)
		os.Exit(1)
	}

	if err := database.Ping(); err != nil {
		slog.Error("failed to ping database", "error", err)
		os.Exit(1)
	}

	slog.Info("database connected successfully")

	m, err := migrate.New("file://db/migrations", cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to initialize migrations", "error", err)
		os.Exit(1)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	slog.Info("migrations applied successfully")

	// NATS
	natsURL := cfg.NATSURL
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		slog.Error("failed to connect to NATS", "url", natsURL, "error", err)
		os.Exit(1)
	}

	slog.Info("NATS connected", "url", natsURL)

	// Wire up layers
	repo := postgresadaptor.NewPostgresRepository(database)
	svc := service.New(repo, logger)
	handler := natsadaptor.NewHandler(svc, nc, logger)

	if err := handler.Subscribe(); err != nil {
		slog.Error("failed to subscribe NATS handler", "error", err)
		os.Exit(1)
	}

	slog.Info("user service ready")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	slog.Info("service shutting down", "service", "user-service")

	handler.Stop()

	if err := nc.Drain(); err != nil {
		slog.Error("failed to drain NATS connection", "error", err)
	}

	if err := database.Close(); err != nil {
		slog.Error("failed to close database connection", "error", err)
	}
}
