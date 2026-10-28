package main

import (
	"io"
	"log/slog"
	"net"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/ghost-five/backend/internal/api"
)

func TestHealthcheck(t *testing.T) {
	srv := httptest.NewServer(api.NewHandler("dev", slog.New(slog.NewTextHandler(io.Discard, nil))))
	_, p, _ := net.SplitHostPort(srv.Listener.Addr().String())
	port, _ := strconv.Atoi(p)

	if code := healthcheck(port); code != 0 {
		t.Errorf("живой сервер: код %d", code)
	}
	srv.Close()
	if code := healthcheck(port); code != 1 {
		t.Errorf("остановленный сервер: код %d", code)
	}
}
