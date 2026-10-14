package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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
	if got := recorder.Header().Get("Content-Type"); got != "" {
		t.Fatalf("Content-Type = %q, want empty", got)
	}
	if got := recorder.Body.String(); got != "" {
		t.Fatalf("body = %q, want empty", got)
	}
}

func TestHTTPAuthHandlerDenials(t *testing.T) {
	tests := []struct {
		name         string
		cookieName   string
		cookieValue  string
		storeError   error
		wantStatus   int
		wantResponse map[string]string
		wantLookups  int
		sensitive    []string
	}{
		{
			name:         "no cookie",
			wantStatus:   http.StatusUnauthorized,
			wantResponse: map[string]string{"type": "UNAUTHORIZED", "message": "Authentication required"},
		},
		{
			name:         "blank cookie",
			cookieName:   "BA_SESSION",
			wantStatus:   http.StatusUnauthorized,
			wantResponse: map[string]string{"type": "UNAUTHORIZED", "message": "Authentication required"},
		},
		{
			name:         "legacy cookie name",
			cookieName:   "SESSION",
			cookieValue:  "legacy-session",
			wantStatus:   http.StatusUnauthorized,
			wantResponse: map[string]string{"type": "UNAUTHORIZED", "message": "Authentication required"},
			wantLookups:  0,
			sensitive:    []string{"legacy-session"},
		},
		{
			name:         "unknown session",
			cookieName:   "BA_SESSION",
			cookieValue:  "missing-session",
			storeError:   ErrSessionNotFound,
			wantStatus:   http.StatusUnauthorized,
			wantResponse: map[string]string{"type": "UNAUTHORIZED", "message": "Authentication required"},
			wantLookups:  1,
			sensitive:    []string{"missing-session", "not found"},
		},
		{
			name:         "expired session",
			cookieName:   "BA_SESSION",
			cookieValue:  "expired-session",
			storeError:   ErrSessionExpired,
			wantStatus:   http.StatusUnauthorized,
			wantResponse: map[string]string{"type": "UNAUTHORIZED", "message": "Authentication required"},
			wantLookups:  1,
			sensitive:    []string{"expired-session", "expired"},
		},
		{
			name:         "unexpected store failure",
			cookieName:   "BA_SESSION",
			cookieValue:  "secret-session-id",
			storeError:   errors.New("redis unavailable at internal-host:6379"),
			wantStatus:   http.StatusServiceUnavailable,
			wantResponse: map[string]string{"type": "SERVICE_UNAVAILABLE", "message": "Service Unavailable"},
			wantLookups:  1,
			sensitive:    []string{"secret-session-id", "redis", "internal-host"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &stubSessionLookupStore{err: tt.storeError}
			req := httptest.NewRequest(http.MethodGet, "http://ext-authz/check", nil)
			if tt.cookieName != "" {
				req.AddCookie(&http.Cookie{Name: tt.cookieName, Value: tt.cookieValue})
			}

			recorder := httptest.NewRecorder()
			NewHTTPAuthHandler(store, LoadConfig()).ServeHTTP(recorder, req)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			if got := recorder.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("Content-Type = %q, want %q", got, "application/json")
			}
			if store.lookupCount != tt.wantLookups {
				t.Fatalf("lookupCount = %d, want %d", store.lookupCount, tt.wantLookups)
			}

			var gotResponse map[string]string
			if err := json.Unmarshal(recorder.Body.Bytes(), &gotResponse); err != nil {
				t.Fatalf("decode response body %q: %v", recorder.Body.String(), err)
			}
			if !reflect.DeepEqual(gotResponse, tt.wantResponse) {
				t.Fatalf("response = %#v, want %#v", gotResponse, tt.wantResponse)
			}
			for _, sensitive := range tt.sensitive {
				if strings.Contains(strings.ToLower(recorder.Body.String()), strings.ToLower(sensitive)) {
					t.Errorf("response body contains sensitive detail %q", sensitive)
				}
			}
		})
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

func TestHTTPAuthHandlerRejectsMissingSessionResultAsUnavailable(t *testing.T) {
	store := &stubSessionLookupStore{
		sessionData: nil,
	}

	req := httptest.NewRequest(http.MethodGet, "http://ext-authz/check", nil)
	req.AddCookie(&http.Cookie{Name: "BA_SESSION", Value: "session-123"})

	recorder := httptest.NewRecorder()
	NewHTTPAuthHandler(store, LoadConfig()).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	var gotResponse map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &gotResponse); err != nil {
		t.Fatalf("decode response body %q: %v", recorder.Body.String(), err)
	}
	wantResponse := map[string]string{"type": "SERVICE_UNAVAILABLE", "message": "Service Unavailable"}
	if !reflect.DeepEqual(gotResponse, wantResponse) {
		t.Fatalf("response = %#v, want %#v", gotResponse, wantResponse)
	}
}
