package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/artem/tasks/internal/model"
)

// Хендлер не знает про *service.TaskService
type TaskService interface {
	Create(ctx context.Context, in model.CreateTaskRequest) (model.Task, error)
	GetByID(ctx context.Context, id int64) (model.Task, error)
	List(ctx context.Context, f model.ListFilter) ([]model.Task, error)
	Update(ctx context.Context, id int64, in model.UpdateTaskRequest) (model.Task, error)
	Delete(ctx context.Context, id int64) error
}

type TaskHandler struct {
	svc TaskService
}

func NewTaskHandler(svc TaskService) *TaskHandler {
	return &TaskHandler{svc: svc}
}

func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in model.CreateTaskRequest

	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "malformed JSON body")
		return
	}

	// r.Context() — не context.Background()
	task, err := h.svc.Create(r.Context(), in)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, task)
}

func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var filter model.ListFilter

	if raw := q.Get("done"); raw != "" {
		done, err := strconv.ParseBool(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "done must be true or false")
			return
		}
		filter.Done = &done
	}
	if raw := q.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "limit must be a number")
			return
		}
		// Границы лимита не проверяем — это бизнес-правило, оно в сервисе.
		filter.Limit = limit
	}

	tasks, err := h.svc.List(r.Context(), filter)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (h *TaskHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}

	task, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (h *TaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}

	var in model.UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "malformed JSON body")
		return
	}

	task, err := h.svc.Update(r.Context(), id, in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (h *TaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusNoContent, nil)
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "id must be a positive number")
		return 0, false
	}
	return id, true
}
