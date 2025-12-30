package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"todo-service/internal/models"
	"todo-service/internal/repository"
)

type TodoRepository interface {
	Create(td models.Todo) (models.Todo, error)
	GetAll(opts repository.ListOptions) []models.Todo
	GetByID(ID int) (models.Todo, error)
	Update(ID int, update models.Todo) (models.Todo, error)
	Delete(ID int) error
}

type Handler struct {
	repo TodoRepository
	log  *slog.Logger
}

func NewHandler(repo TodoRepository, log *slog.Logger) *Handler {
	return &Handler{
		repo: repo,
		log:  log.With(slog.String("component", "handler")),
	}
}

// CreateTodo обрабатывает POST /todos
func (h *Handler) CreateTodo(w http.ResponseWriter, r *http.Request) {
	var td models.Todo
	if err := json.NewDecoder(r.Body).Decode(&td); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if err := td.Validate(); err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	created, err := h.repo.Create(td)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	h.respondJSON(w, http.StatusCreated, created)
}

// GetAllTodos обрабатывает GET /todos
func (h *Handler) GetAllTodos(w http.ResponseWriter, r *http.Request) {
	opts, err := parseListOptions(r)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	todos := h.repo.GetAll(opts)
	h.respondJSON(w, http.StatusOK, todos)
}

// GetTodoByID обрабатывает GET /todos/{id}
func (h *Handler) GetTodoByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.log.Warn("invalid ID", slog.String("error", err.Error()))
		h.respondError(w, http.StatusBadRequest, "invalid id")
		return
	}

	td, err := h.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			h.respondError(w, http.StatusNotFound, "not found")
			return
		}
		h.log.Error("failed to get todo",
			slog.Int("id", id),
			slog.String("error", err.Error()),
		)
		h.respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	h.respondJSON(w, http.StatusOK, td)
}

// UpdateTodo handles PUT /todos/{id}
func (h *Handler) UpdateTodo(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.log.Warn("invalid ID", slog.String("error", err.Error()))
		h.respondError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var td models.Todo
	if err = json.NewDecoder(r.Body).Decode(&td); err != nil {
		h.log.Warn("invalid JSON", slog.String("error", err.Error()))
		h.respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if err = td.Validate(); err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := h.repo.Update(id, td)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			h.respondError(w, http.StatusNotFound, "not found")
			return
		}
		h.log.Error("failed to update todo",
			slog.Int("id", id),
			slog.String("error", err.Error()),
		)
		h.respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	h.log.Info("todo updated",
		slog.Int("id", id),
		slog.String("title", updated.Title),
	)
	h.respondJSON(w, http.StatusOK, updated)
}

// DeleteTodo handles DELETE /todos/{id}
func (h *Handler) DeleteTodo(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.log.Warn("invalid ID", slog.String("error", err.Error()))
		h.respondError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err = h.repo.Delete(id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			h.respondError(w, http.StatusNotFound, "not found")
			return
		}
		h.log.Error("failed to delete todo",
			slog.Int("id", id),
			slog.String("error", err.Error()),
		)
		h.respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	h.log.Info("todo deleted",
		slog.Int("id", id),
	)
	w.WriteHeader(http.StatusNoContent)
}

// respondJSON записывает ответ JSON
func (h *Handler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.Error("failed to encode response",
			slog.String("error", err.Error()),
			slog.Int("status", status),
		)
	}
}

// respondError отправляет ответ со статусом и ошибкой
func (h *Handler) respondError(w http.ResponseWriter, status int, message string) {
	h.respondJSON(w, status, map[string]string{"error": message})
}

// parseListOptions разбирает query-параметры HTTP-запроса с opts
func parseListOptions(r *http.Request) (repository.ListOptions, error) {
	q := r.URL.Query()
	opts := repository.ListOptions{
		Limit:     -1,
		Offset:    0,
		SortField: "id",
	}

	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return opts, errors.New("invalid limit")
		}
		opts.Limit = n
	}
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return opts, errors.New("invalid offset")
		}
		opts.Offset = n
	}
	if v := q.Get("completed"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return opts, errors.New("invalid completed")
		}
		opts.CompletedFilter = &b
	}
	if v := q.Get("sort"); v != "" {
		opts.SortField = v
	}

	switch strings.ToLower(opts.SortField) {
	case "id", "title", "created_at", "updated_at", "completed":
	default:
		return opts, errors.New("invalid sort")
	}

	return opts, nil
}

// RegisterRoutes регитсрирует роуты
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /todos", h.CreateTodo)
	mux.HandleFunc("GET /todos", h.GetAllTodos)
	mux.HandleFunc("GET /todos/{id}", h.GetTodoByID)
	mux.HandleFunc("PUT /todos/{id}", h.UpdateTodo)
	mux.HandleFunc("DELETE /todos/{id}", h.DeleteTodo)

	h.log.Info("routes registered",
		slog.String("endpoints", "POST /todos, GET /todos, GET /todos/{id}, PUT /todos/{id}, DELETE /todos/{id}"),
	)
}

// Shutdown gracefully
func (h *Handler) Shutdown(ctx context.Context, server *http.Server) error {
	h.log.Info("initiating graceful shutdown")

	if err := server.Shutdown(ctx); err != nil {
		h.log.Error("shutdown error",
			slog.String("error", err.Error()),
		)
		return err
	}

	h.log.Info("shutdown completed successfully")
	return nil
}
