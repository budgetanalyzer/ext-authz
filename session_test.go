package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSessionKeyMatchesSessionGatewayGoldenVector(t *testing.T) {
	const token = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	const want = "session:16362f566387b3cf5a6e92fb0a986c76ca20eb3a0c12cbdfbd0b29501e0c18df"

	got := sessionKey("session:", token)
	if got != want {
		t.Fatalf("sessionKey = %q, want %q", got, want)
	}
	if strings.Contains(got, token) {
		t.Fatalf("sessionKey %q contains the raw token", got)
	}
}

func TestSessionKeyUsesConfiguredPrefix(t *testing.T) {
	got := sessionKey("custom:", "3f2504e0-4f89-41d3-9a0c-0305e82c3301")
	want := "custom:16362f566387b3cf5a6e92fb0a986c76ca20eb3a0c12cbdfbd0b29501e0c18df"
	if got != want {
		t.Fatalf("sessionKey = %q, want %q", got, want)
	}
}

func TestNewSessionStoreDoesNotRequireRedisAtStartup(t *testing.T) {
	store, err := NewSessionStore(Config{
		RedisAddr:        "127.0.0.1:1",
		SessionKeyPrefix: "session:",
	})
	if err != nil {
		t.Fatalf("NewSessionStore() error = %v, want nil without a Redis connection", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestWaitForRedisContinuesPastFormerRetryLimit(t *testing.T) {
	const failures = 11
	attempts := 0
	err := waitForRedis(
		context.Background(),
		func(context.Context) error {
			attempts++
			if attempts <= failures {
				return errors.New("redis unavailable")
			}
			return nil
		},
		func(int) time.Duration { return 0 },
	)

	if err != nil {
		t.Fatalf("waitForRedis() error = %v, want nil", err)
	}
	if attempts != failures+1 {
		t.Fatalf("waitForRedis() attempts = %d, want %d", attempts, failures+1)
	}
}

func TestWaitForRedisStopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	err := waitForRedis(
		ctx,
		func(context.Context) error {
			attempts++
			cancel()
			return errors.New("redis unavailable")
		},
		func(int) time.Duration { return time.Hour },
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("waitForRedis() error = %v, want %v", err, context.Canceled)
	}
	if attempts != 1 {
		t.Fatalf("waitForRedis() attempts = %d, want 1", attempts)
	}
}
