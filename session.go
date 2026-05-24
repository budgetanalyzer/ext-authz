package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session expired")
)

// SessionData holds the resolved session fields from Redis.
type SessionData struct {
	UserID      string
	Roles       string
	Permissions string
	ExpiresAt   time.Time
}

// SessionStore wraps a Redis client for session lookups.
type SessionStore struct {
	client    *redis.Client
	keyPrefix string
}

// NewSessionStore creates a Redis-backed session store and verifies connectivity.
// Retries the connection with backoff to handle startup ordering.
func NewSessionStore(cfg Config) (*SessionStore, error) {
	opts := &redis.Options{
		Addr:        cfg.RedisAddr,
		Username:    cfg.RedisUsername,
		Password:    cfg.RedisPassword,
		PoolSize:    10,
		ReadTimeout: 100 * time.Millisecond,
	}

	if cfg.RedisTLS {
		if cfg.RedisCACert == "" {
			return nil, errors.New("REDIS_CA_CERT is required when REDIS_TLS=true")
		}

		host, _, err := net.SplitHostPort(cfg.RedisAddr)
		if err != nil {
			return nil, fmt.Errorf("parse REDIS_ADDR for TLS server name: %w", err)
		}

		caPEM, err := os.ReadFile(cfg.RedisCACert)
		if err != nil {
			return nil, fmt.Errorf("read Redis CA cert: %w", err)
		}

		rootCAs := x509.NewCertPool()
		if !rootCAs.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("parse Redis CA cert at %s", cfg.RedisCACert)
		}

		opts.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    rootCAs,
			ServerName: host,
		}
	}

	client := redis.NewClient(opts)

	maxRetries := 10
	var lastErr error
	for i := range maxRetries {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		lastErr = client.Ping(ctx).Err()
		cancel()
		if lastErr == nil {
			return &SessionStore{
				client:    client,
				keyPrefix: cfg.SessionKeyPrefix,
			}, nil
		}
		backoff := time.Duration(i+1) * 500 * time.Millisecond
		slog.Warn("redis not ready, retrying", "attempt", i+1, "backoff", backoff, "error", lastErr)
		time.Sleep(backoff)
	}

	return nil, fmt.Errorf("redis connection failed after %d attempts: %w", maxRetries, lastErr)
}

// LookupSession retrieves a session from Redis by ID.
// Returns ErrSessionNotFound if the key doesn't exist,
// ErrSessionExpired if the session has passed its expires_at.
func (s *SessionStore) LookupSession(ctx context.Context, sessionID string) (*SessionData, error) {
	key := s.keyPrefix + sessionID

	result, err := s.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("redis error: %w", err)
	}

	if len(result) == 0 {
		return nil, ErrSessionNotFound
	}

	expiresAtUnix, err := strconv.ParseInt(result["expires_at"], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid expires_at: %w", err)
	}

	expiresAt := time.Unix(expiresAtUnix, 0)
	if time.Now().After(expiresAt) {
		return nil, ErrSessionExpired
	}

	return &SessionData{
		UserID:      result["user_id"],
		Roles:       result["roles"],
		Permissions: result["permissions"],
		ExpiresAt:   expiresAt,
	}, nil
}

// Ping checks Redis connectivity for health checks.
func (s *SessionStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}
