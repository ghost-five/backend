// Сервер Ghost Five.
//
//	api              поднять сервер
//	api healthcheck  спросить у запущенного сервера /healthz; код выхода 0 — жив
//
// Вторая команда нужна docker'у: в образе нет ни оболочки, ни wget, ни curl.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ghost-five/backend/internal/api"
	"github.com/ghost-five/backend/internal/config"
)

// Docker после SIGTERM ждёт десять секунд и добивает процесс. Укладываемся в пять,
// чтобы успеть дописать начатые ответы.
const shutdownTimeout = 5 * time.Second

func main() {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "настройки:", err)
		os.Exit(1)
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "healthcheck":
			os.Exit(healthcheck(cfg.Port))
		default:
			fmt.Fprintf(os.Stderr, "неизвестная команда %q; есть только healthcheck\n", os.Args[1])
			os.Exit(2)
		}
	}

	log := newLogger(cfg.Env)
	if err := run(cfg, log); err != nil {
		log.Error("сервер остановился с ошибкой", "err", err)
		os.Exit(1)
	}
}

// В бою журнал — JSON, его удобно разбирать; на своей машине — читаемый текст.
func newLogger(env string) *slog.Logger {
	if env == "production" {
		return slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}

func run(cfg config.Config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Слушаем порт до запуска горутины: занятый порт — сразу понятная ошибка.
	ln, err := new(net.ListenConfig).Listen(ctx, "tcp", ":"+strconv.Itoa(cfg.Port))
	if err != nil {
		return err
	}
	srv := &http.Server{
		Handler:           api.NewHandler(cfg.Version, log),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       2 * time.Minute,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()
	log.Info("ghost-five api запущен", "port", cfg.Port, "version", cfg.Version, "env", cfg.Env)

	select {
	case err := <-served:
		return err
	case <-ctx.Done():
	}
	// Второй Ctrl+C после этого убивает процесс сразу, не дожидаясь запросов.
	stop()
	log.Info("останавливаюсь по сигналу")

	shCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shCtx); err != nil {
		return fmt.Errorf("не дождался открытых запросов: %w", err)
	}
	if err := <-served; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func healthcheck(port int) int {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	url := "http://127.0.0.1:" + strconv.Itoa(port) + "/healthz"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "сервер не отвечает:", err)
		return 1
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "сервер ответил", resp.Status)
		return 1
	}
	return 0
}
