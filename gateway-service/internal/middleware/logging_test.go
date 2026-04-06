package middleware_test

import (
	"bufio"
	"bytes"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gateway-service/internal/middleware"
)

type mockHijackWriter struct {
	*httptest.ResponseRecorder
}

func (m *mockHijackWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, nil
}

func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRequestLogger_PassesThroughResponse(t *testing.T) {
	handler := middleware.RequestLogger(noopLogger())(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
			w.Write([]byte("hello")) //nolint:errcheck
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTeapot {
		t.Errorf("expected status 418, got %d", w.Code)
	}
	if w.Body.String() != "hello" {
		t.Errorf("expected body 'hello', got %q", w.Body.String())
	}
}

func TestRequestLogger_DefaultStatus200(t *testing.T) {
	handler := middleware.RequestLogger(noopLogger())(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ok")) //nolint:errcheck
		}),
	)

	req := httptest.NewRequest(http.MethodPost, "/api", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestRequestLogger_LogsMethod(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	handler := middleware.RequestLogger(logger)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/123", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !strings.Contains(buf.String(), "DELETE") {
		t.Errorf("expected log to contain method 'DELETE', got: %s", buf.String())
	}
}

func TestRequestLogger_LogsPath(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	handler := middleware.RequestLogger(logger)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !strings.Contains(buf.String(), "/api/v1/users") {
		t.Errorf("expected log to contain path '/api/v1/users', got: %s", buf.String())
	}
}

func TestRequestLogger_LogsStatusCode(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	handler := middleware.RequestLogger(logger)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !strings.Contains(buf.String(), "404") {
		t.Errorf("expected log to contain status '404', got: %s", buf.String())
	}
}

func TestRequestLogger_HijackErrorPath(t *testing.T) {
	var hijackErr error
	handler := middleware.RequestLogger(noopLogger())(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h, ok := w.(http.Hijacker)
			if !ok {
				t.Error("expected middleware responseWriter to implement http.Hijacker")
				return
			}
			_, _, hijackErr = h.Hijack()
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if hijackErr == nil {
		t.Error("expected error from Hijack when underlying writer is not a Hijacker")
	}
}

func TestRequestLogger_HijackSuccessPath(t *testing.T) {
	hijackErr := io.EOF
	handler := middleware.RequestLogger(noopLogger())(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h, ok := w.(http.Hijacker)
			if !ok {
				t.Error("expected middleware responseWriter to implement http.Hijacker")
				return
			}
			_, _, hijackErr = h.Hijack()
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	handler.ServeHTTP(&mockHijackWriter{httptest.NewRecorder()}, req)

	if hijackErr != nil {
		t.Errorf("unexpected hijack error: %v", hijackErr)
	}
}
