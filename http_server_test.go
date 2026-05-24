package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubSessionLookupStore struct {
	sessionData *SessionData
	err         error
	lookupCount int
	lastID      string
}

func (s *stubSessionLookupStore) LookupSession(_ context.Context, sessionID string) (*SessionData, error) {
	s.lookupCount++
	s.lastID = sessionID
	if s.err != nil {
		return nil, s.err
	}
	return s.sessionData, nil
}

func TestHTTPAuthHandlerAllowsBASessionCookieByDefault(t *testing.T) {
	store := &stubSessionLookupStore{
		sessionData: &SessionData{
			UserID:      "user-123",
			Roles:       "ROLE_USER",
			Permissions: "transactions:read",
		},
	}

	req := httptest.NewRequest(http.MethodGet, "http://ext-authz/check", nil)
	req.AddCookie(&http.Cookie{Name: "BA_SESSION", Value: "session-123"})
	req.Header.Set("X-Envoy-Original-Path", "/api/v1/transactions")

	recorder := httptest.NewRecorder()
	NewHTTPAuthHandler(store, LoadConfig()).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if store.lookupCount != 1 {
		t.Fatalf("lookupCount = %d, want %d", store.lookupCount, 1)
	}
	if store.lastID != "session-123" {
		t.Fatalf("lastID = %q, want %q", store.lastID, "session-123")
	}
	if got := recorder.Header().Get("X-User-Id"); got != "user-123" {
		t.Fatalf("X-User-Id = %q, want %q", got, "user-123")
	}
	if got := recorder.Header().Get("X-Roles"); got != "ROLE_USER" {
		t.Fatalf("X-Roles = %q, want %q", got, "ROLE_USER")
	}
	if got := recorder.Header().Get("X-Permissions"); got != "transactions:read" {
		t.Fatalf("X-Permissions = %q, want %q", got, "transactions:read")
	}
}

func TestHTTPAuthHandlerRejectsLegacySessionCookieNameByDefault(t *testing.T) {
	store := &stubSessionLookupStore{
		sessionData: &SessionData{UserID: "user-123"},
	}

	req := httptest.NewRequest(http.MethodGet, "http://ext-authz/check", nil)
	req.AddCookie(&http.Cookie{Name: "SESSION", Value: "legacy-session"})

	recorder := httptest.NewRecorder()
	NewHTTPAuthHandler(store, LoadConfig()).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	if store.lookupCount != 0 {
		t.Fatalf("lookupCount = %d, want %d", store.lookupCount, 0)
	}
}

func TestHTTPAuthHandlerRejectsUnknownBASessionCookie(t *testing.T) {
	store := &stubSessionLookupStore{
		err: ErrSessionNotFound,
	}

	req := httptest.NewRequest(http.MethodGet, "http://ext-authz/check", nil)
	req.AddCookie(&http.Cookie{Name: "BA_SESSION", Value: "missing-session"})

	recorder := httptest.NewRecorder()
	NewHTTPAuthHandler(store, LoadConfig()).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	if store.lookupCount != 1 {
		t.Fatalf("lookupCount = %d, want %d", store.lookupCount, 1)
	}
	if store.lastID != "missing-session" {
		t.Fatalf("lastID = %q, want %q", store.lastID, "missing-session")
	}
}

func TestHTTPAuthHandlerUsesConfiguredCookieOverride(t *testing.T) {
	store := &stubSessionLookupStore{
		sessionData: &SessionData{UserID: "user-123"},
	}

	req := httptest.NewRequest(http.MethodGet, "http://ext-authz/check", nil)
	req.AddCookie(&http.Cookie{Name: "CUSTOM_SESSION", Value: "custom-session"})

	recorder := httptest.NewRecorder()
	NewHTTPAuthHandler(store, Config{SessionCookieName: "CUSTOM_SESSION"}).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if store.lastID != "custom-session" {
		t.Fatalf("lastID = %q, want %q", store.lastID, "custom-session")
	}
}

func TestHTTPAuthHandlerRejectsUnknownSessionErrors(t *testing.T) {
	store := &stubSessionLookupStore{
		err: errors.New("redis unavailable"),
	}

	req := httptest.NewRequest(http.MethodGet, "http://ext-authz/check", nil)
	req.AddCookie(&http.Cookie{Name: "BA_SESSION", Value: "session-123"})

	recorder := httptest.NewRecorder()
	NewHTTPAuthHandler(store, LoadConfig()).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}
