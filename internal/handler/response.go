package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/artem/tasks/internal/model"
)

// errorResponse — единый формат ошибки. Клиенту всегда прилетает JSON
type errorResponse struct {
	Error string `json:"error"`
}

// writeJSON пишет ответ. Порядок трёх строк переставлять нельзя:
//
//  1. заголовки ставим до WriteHeader — после него они уже улетели по сети;
//  2. WriteHeader — до первого Write, потому что первый Write, если статус
//     не выставлен, сам поставит 200. Классический баг: человек пишет тело,
//     потом WriteHeader(201), получает 200 и warning в логах.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Ответ уже начали писать — статус не поменять, клиенту не сообщить.
		// Единственное осмысленное действие — записать в лог.
		slog.Error("encode response", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, model.ErrNotFound):
		writeError(w, http.StatusNotFound, "task not found")
	case errors.Is(err, model.ErrInvalidInput):
		// err.Error() отдаём наружу осознанно: это текст, который мы сами написали в сервисе для клиента.
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		// сюда попадает всё неожиданное — ошибки драйвера, паники
		// в другом месте. Наружу отдаём общий текст, подробности только
		// в лог: не надо светить клиенту внутренности.
		slog.Error("unhandled service error", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
