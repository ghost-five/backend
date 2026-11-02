// Package api — HTTP-часть сервера. Пока умеет только сказать, что жив: этого
// достаточно, чтобы собрать образ, выложить его на ВМ и убедиться, что весь
// конвейер работает. Дальше сюда приезжают маршруты из docs/api/openapi.yaml.
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

// NewHandler собирает все маршруты сервера.
func NewHandler(version string, log *slog.Logger) http.Handler {
	h := &handler{version: version, started: time.Now(), log: log}
	mux := http.NewServeMux()
	// Путь без префикса — для healthcheck внутри контейнера, с префиксом — наружу.
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /api/v1/healthz", h.health)
	mux.HandleFunc("/", notFound)
	return recoverer(log, mux)
}

type handler struct {
	version string
	started time.Time
	log     *slog.Logger
}

func (h *handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": h.version,
		"uptime":  int(time.Since(h.started).Round(time.Second).Seconds()),
	})
}

func notFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "Такого пути нет")
}

// recoverer превращает панику в обработчике в ответ 500 по контракту. Без него
// net/http просто рвёт соединение, и клиент не понимает, что случилось.
func recoverer(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			// Так net/http сам просит оборвать ответ — это не ошибка.
			if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(v)
			}
			log.Error("ошибка в обработчике", "path", r.URL.Path, "panic", v, "stack", string(debug.Stack()))
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка")
		}()
		next.ServeHTTP(w, r)
	})
}

// Ошибка по схеме Error из openapi.yaml: машинный code и текст для человека.
type errorBody struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: apiError{Code: code, Message: message}})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	data, err := json.Marshal(body)
	if err != nil {
		status = http.StatusInternalServerError
		data = []byte(`{"error":{"code":"internal","message":"Внутренняя ошибка"}}`)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
