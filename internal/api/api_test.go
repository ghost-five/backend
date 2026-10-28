package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func do(t *testing.T, h http.Handler, method, path string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), method, path, nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("%s %s: ответ не JSON: %q", method, path, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("%s %s: Content-Type %q", method, path, ct)
	}
	return rec, body
}

func errorCode(body map[string]any) any {
	e, _ := body["error"].(map[string]any)
	return e["code"]
}

func TestHealth(t *testing.T) {
	h := NewHandler("abc123", quiet)
	for _, path := range []string{"/healthz", "/api/v1/healthz"} {
		rec, body := do(t, h, http.MethodGet, path)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: код %d", path, rec.Code)
		}
		if body["status"] != "ok" || body["version"] != "abc123" {
			t.Errorf("%s: %v", path, body)
		}
		if _, ok := body["uptime"].(float64); !ok {
			t.Errorf("%s: нет uptime: %v", path, body)
		}
	}
}

func TestNotFound(t *testing.T) {
	h := NewHandler("dev", quiet)
	for _, path := range []string{"/", "/api/v1/nope", "/healthz/extra"} {
		rec, body := do(t, h, http.MethodGet, path)
		if rec.Code != http.StatusNotFound || errorCode(body) != "not_found" {
			t.Errorf("%s: %d %v", path, rec.Code, body)
		}
	}
}

func TestPanicBecomes500(t *testing.T) {
	h := recoverer(quiet, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("сломалось")
	}))
	rec, body := do(t, h, http.MethodGet, "/")
	if rec.Code != http.StatusInternalServerError || errorCode(body) != "internal" {
		t.Errorf("%d %v", rec.Code, body)
	}
}
