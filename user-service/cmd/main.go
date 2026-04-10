package main

import (
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/nats-io/nats.go"

	"user-service/internal/adapters/natsadaptor"
	"user-service/internal/adapters/postgresadaptor"
	"user-service/internal/config"
	"user-service/internal/service"
)

func main() {
	// load config first using a bootstrap logger
	bootstrapLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(bootstrapLogger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// setup real logger using config log level
	logLevel := slog.LevelInfo
	if cfg.LogLevel == "debug" {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	slog.Info("service starting", "service", "user-service")

	// DB
	// extract to a sepereate fucn or file
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
	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		slog.Error("failed to connect to NATS", "url", cfg.NATSURL, "error", err)
		os.Exit(1)
	}

	slog.Info("NATS connected", "url", cfg.NATSURL)

	// Wire up layers
	repo := postgresadaptor.NewPostgresRepository(database)
	svc := service.NewService(repo, logger)
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
