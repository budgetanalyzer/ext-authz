package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg := LoadConfig()

	// Initialize structured logger
	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: parseLogLevel(cfg.LogLevel)}
	if cfg.LogFormat == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler))

	// Connect to Redis (retries with backoff)
	store, err := NewSessionStore(cfg)
	if err != nil {
		slog.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to redis", "addr", cfg.RedisAddr)

	// Start health HTTP server in background
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := store.Ping(r.Context()); err != nil {
			slog.Error("health check failed", "error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintln(w, "unhealthy")
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	healthServer := &http.Server{
		Addr:    ":" + cfg.HealthPort,
		Handler: healthMux,
	}
	go func() {
		slog.Info("health server starting", "port", cfg.HealthPort)
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("health server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Start HTTP ext_authz server (used by Envoy Gateway HTTP ext_authz mode)
	httpAuthServer := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: NewHTTPAuthHandler(store, cfg),
	}
	go func() {
		slog.Info("http ext_authz server starting", "port", cfg.HTTPPort)
		if err := httpAuthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http ext_authz server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown on SIGTERM/SIGINT
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	sig := <-sigCh
	slog.Info("shutting down", "signal", sig.String())

	httpAuthServer.Shutdown(context.Background())
	healthServer.Shutdown(context.Background())
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
