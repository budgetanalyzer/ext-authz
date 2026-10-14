package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

const (
	unauthorizedErrorType       = "UNAUTHORIZED"
	serviceUnavailableErrorType = "SERVICE_UNAVAILABLE"
	authenticationRequired      = "Authentication required"
	serviceUnavailable          = "Service Unavailable"
)

type apiErrorResponse struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

var (
	unauthorizedResponse = apiErrorResponse{
		Type:    unauthorizedErrorType,
		Message: authenticationRequired,
	}
	serviceUnavailableResponse = apiErrorResponse{
		Type:    serviceUnavailableErrorType,
		Message: serviceUnavailable,
	}
)

type SessionLookupStore interface {
	LookupSession(ctx context.Context, sessionID string) (*SessionData, error)
}

// NewHTTPAuthHandler returns an HTTP handler that implements ext_authz HTTP mode.
// Envoy forwards the original request headers; the handler extracts the session
// cookie, validates against Redis, and returns 200 with identity headers on
// success, 401 for authentication failures, or 503 for session-store failures.
func NewHTTPAuthHandler(store SessionLookupStore, cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.Header.Get("X-Envoy-Original-Path")
		if path == "" {
			path = r.URL.Path
		}
		method := r.Method

		// Extract session cookie
		cookie, err := r.Cookie(cfg.SessionCookieName)
		if err != nil || cookie.Value == "" {
			slog.Warn("no session cookie",
				"path", path,
				"method", method,
				"decision", "deny",
				"transport", "http",
			)
			writeJSONDenial(w, http.StatusUnauthorized, unauthorizedResponse)
			return
		}

		// Look up session in Redis
		session, lookupErr := store.LookupSession(r.Context(), cookie.Value)
		if lookupErr != nil {
			if errors.Is(lookupErr, ErrSessionNotFound) {
				slog.Warn("session not found",
					"path", path,
					"method", method,
					"decision", "deny",
					"transport", "http",
				)
				writeJSONDenial(w, http.StatusUnauthorized, unauthorizedResponse)
			} else if errors.Is(lookupErr, ErrSessionExpired) {
				slog.Warn("session expired",
					"path", path,
					"method", method,
					"decision", "deny",
					"transport", "http",
				)
				writeJSONDenial(w, http.StatusUnauthorized, unauthorizedResponse)
			} else {
				slog.Error("session lookup failed",
					"error", lookupErr,
					"path", path,
					"method", method,
					"decision", "deny",
					"transport", "http",
				)
				writeJSONDenial(w, http.StatusServiceUnavailable, serviceUnavailableResponse)
			}
			return
		}
		if session == nil {
			slog.Error("session lookup returned no result",
				"path", path,
				"method", method,
				"decision", "deny",
				"transport", "http",
			)
			writeJSONDenial(w, http.StatusServiceUnavailable, serviceUnavailableResponse)
			return
		}

		// Session valid — set identity headers on the response.
		// Envoy forwards these to the upstream based on headersToBackend.
		slog.Info("session found, injecting headers",
			"user_id", session.UserID,
			"path", path,
			"method", method,
			"decision", "allow",
			"transport", "http",
		)

		w.Header().Set("X-User-Id", session.UserID)
		w.Header().Set("X-Roles", session.Roles)
		w.Header().Set("X-Permissions", session.Permissions)
		w.WriteHeader(http.StatusOK)
	})
}

func writeJSONDenial(w http.ResponseWriter, status int, response apiErrorResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("failed to write authorization denial response",
			"error", err,
			"status", status,
			"decision", "deny",
			"transport", "http",
		)
	}
}
