package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"gateway-service/internal/adapters/httphandler"
	"gateway-service/internal/adapters/userclientadapter"
	"gateway-service/internal/adapters/wshandler"
	"gateway-service/internal/app"
	"gateway-service/internal/config"
	"gateway-service/internal/middleware"
	"user-service/pkg/userclient"
)

func main() {

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logLevel := slog.LevelInfo
	if cfg.LogLevel == "debug" {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	slog.Info("gateway-service starting",
		"nats_url", cfg.NATSURL,
		"port", cfg.Port,
		"cache_enabled", cfg.CacheEnabled,
	)

	client, err := userclient.New(userclient.Config{
		NATSURL:      cfg.NATSURL,
		Timeout:      5 * time.Second,
		CacheEnabled: cfg.CacheEnabled,
	}, logger)
	if err != nil {
		logger.Error("failed to create user client", "error", err)
		os.Exit(1)
	}
	defer client.Close()

	adapter := userclientadapter.New(client)

	svc := app.New(adapter, logger)

	r := chi.NewRouter()
	r.Use(middleware.RequestLogger(logger))
	h := httphandler.New(svc, logger)
	h.RegisterRoutes(r)

	hub := wshandler.NewHub(logger)
	go hub.Run()

	wsCfg := wshandler.DefaultConfig()
	ws := wshandler.NewWSHandler(svc, adapter, hub, wsCfg, logger)
	r.Get("/api/v1/ws", ws.ServeHTTP)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		logger.Info("http server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server error", "error", err)
			os.Exit(1)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("shutdown signal received, draining...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("http server shutdown error", "error", err)
	}

	logger.Info("gateway-service stopped")
}
