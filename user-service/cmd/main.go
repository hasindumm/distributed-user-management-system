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
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("service starting",
		"service", "user-service",
	)

	dbURL := os.Getenv("DATABASE_URL")

	if dbURL == "" {
		slog.Error("DATABASE_URL is not set")
		os.Exit(1)
	}

	// connect to PostgreSQL
	database, err := sql.Open("postgres", dbURL)

	if err != nil {
		slog.Error("error connecting to the database", "error", err)
		os.Exit(1)
	}

	// verify connection
	if err := database.Ping(); err != nil {
		slog.Error("failed to ping database",
			"error", err,
		)
		os.Exit(1)
	}

	slog.Info("database connected successfully")

	// Run migrations
	m, err := migrate.New("file://db/migrations", dbURL)
	if err != nil {
		slog.Error("failed to initialize migrations", "error", err)
		os.Exit(1)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		slog.Error("failed to initialize migrations", "error", err)
	}

	slog.Info("migrations applied successfully",
		"service", "user-service",
	)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan

	slog.Info("service shutting down",
		"service", "user-service",
	)

	// close DB connection gracefully
	if err := database.Close(); err != nil {
		slog.Error("failed to close database connection",
			"error", err,
		)
	}
}
