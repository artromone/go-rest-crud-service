package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/artem/tasks/internal/config"
	"github.com/artem/tasks/internal/handler"
	"github.com/artem/tasks/internal/repo"
	"github.com/artem/tasks/internal/service"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

// run возвращает ошибку вместо log.Fatal внутри.
// log.Fatal вызывает os.Exit, а os.Exit не выполняет defer — все закрытия
// пула и отмены контекстов молча не отработают.
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx := context.Background()

	// ── Зависимости: pool → repo → service → handler → router ───────────

	pool, err := pgxpool.New(ctx, cfg.Postgres.DSN())
	if err != nil {
		return err
	}
	defer pool.Close()

	// Ping при старте: лучше упасть сразу и понятно, чем принимать трафик
	// и отдавать 500 на каждый запрос.
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		return err
	}
	slog.Info("connected to database")

	taskRepo := repo.NewTaskRepo(pool)
	taskSvc := service.New(taskRepo)
	taskHandler := handler.NewTaskHandler(taskSvc)
	router := handler.NewRouter(taskHandler, pool)

	// ── Сервер с таймаутами ─────────────────────────────────────────────
	// http.ListenAndServe удобен и не имеет ни одного таймаута.
	// Клиент открывает соединение, начинает медленно слать запрос и не
	// досылает — горутина висит вечно. Сотня таких клиентов кладёт сервер
	// при нулевой реальной нагрузке (атака Slowloris).
	srv := &http.Server{
		Addr:         cfg.HTTP.Addr,
		Handler:      router,
		ReadTimeout:  cfg.HTTP.ReadTimeout,  // сколько ждём весь запрос от клиента
		WriteTimeout: cfg.HTTP.WriteTimeout, // сколько у нас есть на ответ
		IdleTimeout:  cfg.HTTP.IdleTimeout,  // сколько держим молчащий keep-alive
	}

	// Сервер — в горутину, main блокируется на ожидании сигнала.
	serverErr := make(chan error, 1)
	go func() {
		slog.Info("server started", "addr", cfg.HTTP.Addr)
		// ErrServerClosed — штатный возврат после Shutdown, не ошибка.
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// ── Graceful shutdown ───────────────────────────────────────────────
	// SIGINT — это Ctrl+C. SIGTERM — то, что присылает Kubernetes, когда
	// выкатывает новую версию и гасит старый под. Без graceful shutdown
	// каждый деплой обрывает все выполняющиеся в этот момент запросы.
	stopCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErr:
		return err
	case <-stopCtx.Done():
		slog.Info("shutdown signal received")
	}

	// Shutdown делает две вещи: перестаёт принимать новые соединения
	// и ждёт, пока доработают текущие. Дедлайн обязателен — иначе один
	// зависший запрос не даст поду умереть никогда.
	shutdownCtx, cancelShutdown := context.WithTimeout(ctx, cfg.HTTP.ShutdownTimeout)
	defer cancelShutdown()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	slog.Info("server stopped gracefully")
	return nil
}
