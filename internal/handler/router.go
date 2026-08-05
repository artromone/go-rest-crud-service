package handler

import (
	"context"
	"net/http"

	"github.com/artem/tasks/internal/middleware"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

func NewRouter(h *TaskHandler, db Pinger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/v1/tasks", h.Create)
	mux.HandleFunc("GET /api/v1/tasks", h.List)
	mux.HandleFunc("GET /api/v1/tasks/{id}", h.GetByID)
	mux.HandleFunc("PATCH /api/v1/tasks/{id}", h.Update)
	mux.HandleFunc("DELETE /api/v1/tasks/{id}", h.Delete)

	// healthz вне /api/v1 — это не часть публичного контракта,
	// а ручка для оркестратора и мониторинга.
	mux.HandleFunc("GET /healthz", healthz(db))

	return middleware.Chain(mux,
		middleware.RequestID,
		middleware.Logger,
		middleware.Recovery,
	)
}

// healthz проверяет, что живо не только приложение, но и база.
// как "Test connection" в вашей IDE.
func healthz(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			writeError(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
