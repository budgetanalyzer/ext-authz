package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
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

const (
	redisPingTimeout     = 2 * time.Second
	redisRetryStep       = 500 * time.Millisecond
	redisMaxRetryBackoff = 5 * time.Second
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

// NewSessionStore creates a Redis-backed session store without making a network
// connection. Call WaitForRedis in the background to monitor initial readiness.
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
	return &SessionStore{
		client:    client,
		keyPrefix: cfg.SessionKeyPrefix,
	}, nil
}

type redisPingFunc func(context.Context) error
type redisBackoffFunc func(int) time.Duration

// WaitForRedis retries until Redis responds or the context is canceled. A
// dependency outage never exhausts the retry loop or terminates the process.
func (s *SessionStore) WaitForRedis(ctx context.Context) error {
	return waitForRedis(ctx, s.Ping, redisRetryBackoff)
}

func waitForRedis(ctx context.Context, ping redisPingFunc, backoff redisBackoffFunc) error {
	for attempt := 1; ; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, redisPingTimeout)
		err := ping(pingCtx)
		cancel()
		if err == nil {
			slog.Info("connected to redis", "attempt", attempt)
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		delay := backoff(attempt)
		slog.Warn("redis not ready, retrying", "attempt", attempt, "backoff", delay, "error", err)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func redisRetryBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt >= int(redisMaxRetryBackoff/redisRetryStep) {
		return redisMaxRetryBackoff
	}
	delay := time.Duration(attempt) * redisRetryStep
	return delay
}

// sessionKey builds the Redis session key from the lowercase hex SHA-256 of
// the UTF-8 cookie value, matching Session Gateway's key derivation.
func sessionKey(prefix, sessionID string) string {
	digest := sha256.Sum256([]byte(sessionID))
	return prefix + hex.EncodeToString(digest[:])
}

// LookupSession retrieves a session from Redis by ID.
// Returns ErrSessionNotFound if the key doesn't exist,
// ErrSessionExpired if the session has passed its expires_at.
func (s *SessionStore) LookupSession(ctx context.Context, sessionID string) (*SessionData, error) {
	key := sessionKey(s.keyPrefix, sessionID)

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

// Close releases resources held by the Redis client.
func (s *SessionStore) Close() error {
	return s.client.Close()
}
