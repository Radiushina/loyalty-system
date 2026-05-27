package logger

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestLoggingMiddleware_StatusDefaultsTo200WhenWriteHeaderNotCalled(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core)

	body := "ok"
	h := LoggingMiddleware(log, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, body) // no explicit WriteHeader
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.test/some/path", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Message != "HTTP request" {
		t.Fatalf("unexpected log message: %q", e.Message)
	}

	fields := make(map[string]any, len(e.Context))
	for _, f := range e.Context {
		switch f.Key {
		case "status", "response_size":
			fields[f.Key] = int(f.Integer)
		case "method", "uri":
			fields[f.Key] = f.String
		default:
			fields[f.Key] = f.Interface
		}
	}

	if got, ok := fields["status"].(int); !ok || got != http.StatusOK {
		t.Fatalf("expected status=%d, got %#v", http.StatusOK, fields["status"])
	}
	if got, ok := fields["response_size"].(int); !ok || got != len(body) {
		t.Fatalf("expected response_size=%d, got %#v", len(body), fields["response_size"])
	}
	if got, ok := fields["method"].(string); !ok || got != http.MethodGet {
		t.Fatalf("expected method=%q, got %#v", http.MethodGet, fields["method"])
	}
	if got, ok := fields["uri"].(string); !ok || got != "/some/path" {
		t.Fatalf("expected uri=%q, got %#v", "/some/path", fields["uri"])
	}
}

func TestLoggingMiddleware_StatusFromExplicitWriteHeader(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core)

	h := LoggingMiddleware(log, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "http://example.test/value", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}

	var status int
	for _, f := range entries[0].Context {
		if f.Key == "status" {
			status = int(f.Integer)
		}
	}
	if status != http.StatusNoContent {
		t.Fatalf("expected status=%d, got %d", http.StatusNoContent, status)
	}
}
