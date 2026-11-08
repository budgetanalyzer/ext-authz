package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubRedisHealthStore struct {
	err       error
	pingCount int
}

func (s *stubRedisHealthStore) Ping(context.Context) error {
	s.pingCount++
	return s.err
}

func TestHealthHandlerSeparatesLivenessFromReadiness(t *testing.T) {
	store := &stubRedisHealthStore{err: errors.New("redis unavailable")}
	handler := newHealthHandler(store)

	liveRecorder := httptest.NewRecorder()
	handler.ServeHTTP(liveRecorder, httptest.NewRequest(http.MethodGet, "/livez", nil))
	if liveRecorder.Code != http.StatusOK {
		t.Fatalf("GET /livez status = %d, want %d", liveRecorder.Code, http.StatusOK)
	}
	if store.pingCount != 0 {
		t.Fatalf("GET /livez ping count = %d, want 0", store.pingCount)
	}

	readyRecorder := httptest.NewRecorder()
	handler.ServeHTTP(readyRecorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if readyRecorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /readyz status = %d, want %d", readyRecorder.Code, http.StatusServiceUnavailable)
	}
	if store.pingCount != 1 {
		t.Fatalf("GET /readyz ping count = %d, want 1", store.pingCount)
	}

	removedRecorder := httptest.NewRecorder()
	handler.ServeHTTP(removedRecorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if removedRecorder.Code != http.StatusNotFound {
		t.Fatalf("GET /healthz status = %d, want %d", removedRecorder.Code, http.StatusNotFound)
	}
}

func TestHealthHandlerReportsRedisReadiness(t *testing.T) {
	store := &stubRedisHealthStore{}
	recorder := httptest.NewRecorder()

	newHealthHandler(store).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /readyz status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestHTTPServersSetConnectionTimeouts(t *testing.T) {
	store := &stubRedisHealthStore{}
	servers := map[string]*http.Server{
		"health": newHealthServer(":8090", store),
		"auth":   newHTTPAuthServer(":9002", &stubSessionLookupStore{}, Config{}),
	}

	for name, server := range servers {
		t.Run(name, func(t *testing.T) {
			if server.ReadHeaderTimeout != serverReadHeaderTimeout {
				t.Errorf("ReadHeaderTimeout = %s, want %s", server.ReadHeaderTimeout, serverReadHeaderTimeout)
			}
			if server.ReadTimeout != serverReadTimeout {
				t.Errorf("ReadTimeout = %s, want %s", server.ReadTimeout, serverReadTimeout)
			}
			if server.WriteTimeout != serverWriteTimeout {
				t.Errorf("WriteTimeout = %s, want %s", server.WriteTimeout, serverWriteTimeout)
			}
			if server.IdleTimeout != serverIdleTimeout {
				t.Errorf("IdleTimeout = %s, want %s", server.IdleTimeout, serverIdleTimeout)
			}
		})
	}

	if serverWriteTimeout <= redisPingTimeout {
		t.Fatalf("WriteTimeout = %s, must exceed Redis call budget %s", serverWriteTimeout, redisPingTimeout)
	}
}
