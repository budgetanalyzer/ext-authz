package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	serverReadHeaderTimeout = 5 * time.Second
	serverReadTimeout       = 10 * time.Second
	serverWriteTimeout      = 30 * time.Second
	serverIdleTimeout       = 60 * time.Second
	serverShutdownTimeout   = 5 * time.Second
)

type redisHealthStore interface {
	Ping(ctx context.Context) error
}

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

	// Constructing the store does not connect to Redis, so the liveness
	// endpoint can be available before dependency initialization begins.
	store, err := NewSessionStore(cfg)
	if err != nil {
		slog.Error("failed to initialize redis client", "error", err)
		os.Exit(1)
	}

	// Start health HTTP server in background
	healthServer := newHealthServer(":"+cfg.HealthPort, store)
	healthListener, err := net.Listen("tcp", healthServer.Addr)
	if err != nil {
		slog.Error("failed to start health listener", "error", err)
		os.Exit(1)
	}
	go func() {
		slog.Info("health server starting", "port", cfg.HealthPort)
		if err := healthServer.Serve(healthListener); err != nil && err != http.ErrServerClosed {
			slog.Error("health server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Retry Redis connectivity in the background for as long as necessary.
	redisRetryCtx, stopRedisRetries := context.WithCancel(context.Background())
	go func() {
		if err := store.WaitForRedis(redisRetryCtx); err != nil && redisRetryCtx.Err() == nil {
			slog.Error("redis connection monitor stopped", "error", err)
		}
	}()

	// Start HTTP ext_authz server (used by Envoy Gateway HTTP ext_authz mode)
	httpAuthServer := newHTTPAuthServer(":"+cfg.HTTPPort, store, cfg)
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

	stopRedisRetries()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), serverShutdownTimeout)
	defer cancelShutdown()
	if err := httpAuthServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("failed to shut down http ext_authz server", "error", err)
	}
	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("failed to shut down health server", "error", err)
	}
	if err := store.Close(); err != nil {
		slog.Error("failed to close redis client", "error", err)
	}
}

func newHealthHandler(store redisHealthStore) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), redisPingTimeout)
		defer cancel()
		if err := store.Ping(ctx); err != nil {
			slog.Warn("readiness check failed", "error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintln(w, "not ready")
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	return mux
}

func newHealthServer(addr string, store redisHealthStore) *http.Server {
	return newHTTPServer(addr, newHealthHandler(store))
}

func newHTTPAuthServer(addr string, store SessionLookupStore, cfg Config) *http.Server {
	return newHTTPServer(addr, NewHTTPAuthHandler(store, cfg))
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: serverReadHeaderTimeout,
		ReadTimeout:       serverReadTimeout,
		WriteTimeout:      serverWriteTimeout,
		IdleTimeout:       serverIdleTimeout,
	}
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
