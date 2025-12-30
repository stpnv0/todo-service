package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
	"todo-service/internal/models"
	"todo-service/internal/repository"
)

func assertStatus(t *testing.T, got, want int, msg string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: expected status %d, got %d", msg, want, got)
	}
}

func assertJSONContains(t *testing.T, body, substring, msg string) {
	t.Helper()
	if !strings.Contains(body, substring) {
		t.Errorf("%s: expected body to contain %q, got %q", msg, substring, body)
	}
}

func decodeJSON[T any](t *testing.T, r io.Reader) T {
	t.Helper()
	var res T
	if err := json.NewDecoder(r).Decode(&res); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	return res
}

func newTestHandler() (*Handler, *repository.Repository) {
	repo := repository.NewRepository()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(repo, log), repo
}

func TestHandler(t *testing.T) {
	t.Run("CreateTodo", testCreateTodo)
	t.Run("GetAllTodos", testGetAllTodos)
	t.Run("GetTodoByID", testGetTodoByID)
	t.Run("UpdateTodo", testUpdateTodo)
	t.Run("DeleteTodo", testDeleteTodo)
	t.Run("InternalErrors", testInternalErrors)
}

func testCreateTodo(t *testing.T) {
	h, _ := newTestHandler()

	t.Run("success", func(t *testing.T) {
		body := `{"title": "Test Task", "description": "Desc"}`
		req := httptest.NewRequest(http.MethodPost, "/todos", strings.NewReader(body))
		w := httptest.NewRecorder()

		h.CreateTodo(w, req)

		assertStatus(t, w.Code, http.StatusCreated, "Create success")
		res := decodeJSON[models.Todo](t, w.Body)
		if res.ID == 0 || res.Title != "Test Task" {
			t.Errorf("unexpected response: %+v", res)
		}
	})

	t.Run("validation error - empty title", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/todos", strings.NewReader(`{"title": ""}`))
		w := httptest.NewRecorder()
		h.CreateTodo(w, req)
		assertStatus(t, w.Code, http.StatusBadRequest, "Empty title")
		assertJSONContains(t, w.Body.String(), "title cannot be empty", "Error message")
	})

	t.Run("validation error - past deadline", func(t *testing.T) {
		past := time.Now().Add(-time.Hour).Format(time.RFC3339)
		body := `{"title": "Task", "deadline": "` + past + `"}`
		req := httptest.NewRequest(http.MethodPost, "/todos", strings.NewReader(body))
		w := httptest.NewRecorder()
		h.CreateTodo(w, req)
		assertStatus(t, w.Code, http.StatusBadRequest, "Past deadline")
		assertJSONContains(t, w.Body.String(), "deadline cannot be in the past", "Error message")
	})

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/todos", strings.NewReader(`{invalid}`))
		w := httptest.NewRecorder()
		h.CreateTodo(w, req)
		assertStatus(t, w.Code, http.StatusBadRequest, "Invalid JSON")
	})
}

func testGetAllTodos(t *testing.T) {
	h, repo := newTestHandler()
	repo.Create(models.Todo{Title: "Task 1", Completed: false})
	repo.Create(models.Todo{Title: "Task 2", Completed: true})

	t.Run("all todos", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/todos", nil)
		w := httptest.NewRecorder()
		h.GetAllTodos(w, req)
		assertStatus(t, w.Code, http.StatusOK, "GetAll success")
		res := decodeJSON[[]models.Todo](t, w.Body)
		if len(res) != 2 {
			t.Errorf("expected 2 todos, got %d", len(res))
		}
	})

	t.Run("filter completed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/todos?completed=true", nil)
		w := httptest.NewRecorder()
		h.GetAllTodos(w, req)
		res := decodeJSON[[]models.Todo](t, w.Body)
		if len(res) != 1 || !res[0].Completed {
			t.Errorf("filter failed, got: %+v", res)
		}
	})
}

func testGetTodoByID(t *testing.T) {
	h, repo := newTestHandler()
	td, _ := repo.Create(models.Todo{Title: "Find Me"})

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/todos/"+strconv.Itoa(td.ID), nil)
		req.SetPathValue("id", strconv.Itoa(td.ID))
		w := httptest.NewRecorder()
		h.GetTodoByID(w, req)
		assertStatus(t, w.Code, http.StatusOK, "GetByID success")
		res := decodeJSON[models.Todo](t, w.Body)
		if res.ID != td.ID {
			t.Errorf("expected ID %d, got %d", td.ID, res.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/todos/999", nil)
		req.SetPathValue("id", "999")
		w := httptest.NewRecorder()
		h.GetTodoByID(w, req)
		assertStatus(t, w.Code, http.StatusNotFound, "Not found")
	})

	t.Run("invalid id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/todos/abc", nil)
		req.SetPathValue("id", "abc")
		w := httptest.NewRecorder()
		h.GetTodoByID(w, req)
		assertStatus(t, w.Code, http.StatusBadRequest, "Invalid ID")
	})
}

func testUpdateTodo(t *testing.T) {
	h, repo := newTestHandler()
	td, _ := repo.Create(models.Todo{Title: "Old"})

	t.Run("success", func(t *testing.T) {
		body := `{"title": "New", "completed": true}`
		req := httptest.NewRequest(http.MethodPut, "/todos/"+strconv.Itoa(td.ID), strings.NewReader(body))
		req.SetPathValue("id", strconv.Itoa(td.ID))
		w := httptest.NewRecorder()
		h.UpdateTodo(w, req)
		assertStatus(t, w.Code, http.StatusOK, "Update success")
		res := decodeJSON[models.Todo](t, w.Body)
		if res.Title != "New" || !res.Completed {
			t.Error("fields not updated")
		}
	})
}

func testDeleteTodo(t *testing.T) {
	h, repo := newTestHandler()
	td, _ := repo.Create(models.Todo{Title: "Delete"})

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/todos/"+strconv.Itoa(td.ID), nil)
		req.SetPathValue("id", strconv.Itoa(td.ID))
		w := httptest.NewRecorder()
		h.DeleteTodo(w, req)
		assertStatus(t, w.Code, http.StatusNoContent, "Delete success")
	})
}

// Mock репо
type errRepo struct{}

func (e *errRepo) Create(td models.Todo) (models.Todo, error) {
	return models.Todo{}, errors.New("err")
}
func (e *errRepo) GetAll(opts repository.ListOptions) []models.Todo { return nil }
func (e *errRepo) GetByID(ID int) (models.Todo, error)              { return models.Todo{}, errors.New("err") }
func (e *errRepo) Update(ID int, update models.Todo) (models.Todo, error) {
	return models.Todo{}, errors.New("err")
}
func (e *errRepo) Delete(ID int) error { return errors.New("err") }

func testInternalErrors(t *testing.T) {
	h := NewHandler(&errRepo{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	t.Run("create 500", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/todos", strings.NewReader(`{"title":"x"}`))
		w := httptest.NewRecorder()
		h.CreateTodo(w, req)
		assertStatus(t, w.Code, http.StatusInternalServerError, "Internal error")
	})
}
