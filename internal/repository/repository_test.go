package repository

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
	"todo-service/internal/models"
)

func assertEqual(t *testing.T, got, want interface{}, msg string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: got %v, want %v", msg, got, want)
	}
}

func assertNoError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", msg, err)
	}
}

func assertError(t *testing.T, err, wantErr error, msg string) {
	t.Helper()
	if !errors.Is(err, wantErr) {
		t.Errorf("%s: got error %v, want %v", msg, err, wantErr)
	}
}

func assertTrue(t *testing.T, condition bool, msg string) {
	t.Helper()
	if !condition {
		t.Errorf("%s: condition is false", msg)
	}
}

func TestRepository(t *testing.T) {
	t.Run("Create", testCreate)
	t.Run("GetByID", testGetByID)
	t.Run("Update", testUpdate)
	t.Run("Delete", testDelete)
	t.Run("GetAll", testGetAll)
	t.Run("ConcurrentAccess", testConcurrentAccess)
}

func testCreate(t *testing.T) {
	r := NewRepository()

	t.Run("success", func(t *testing.T) {
		todo := models.Todo{Title: "Test", Description: "Desc"}
		created, err := r.Create(todo)

		assertNoError(t, err, "Create")
		assertEqual(t, created.ID, 1, "ID increment")
		assertEqual(t, created.Title, "Test", "Title")
		assertTrue(t, !created.CreatedAt.IsZero(), "CreatedAt set")
		assertTrue(t, created.CreatedAt.Equal(created.UpdatedAt), "Timestamps equal on create")
	})

	t.Run("auto-increment", func(t *testing.T) {
		r.Create(models.Todo{Title: "Task 2"})
		td3, _ := r.Create(models.Todo{Title: "Task 3"})
		assertEqual(t, td3.ID, 3, "Sequential ID")
	})

	t.Run("with deadline", func(t *testing.T) {
		deadline := time.Now().Add(time.Hour).Truncate(time.Second)
		td, err := r.Create(models.Todo{Title: "Deadline", Deadline: &deadline})
		assertNoError(t, err, "Create with deadline")
		if td.Deadline == nil {
			t.Fatal("deadline is nil")
		}
		assertTrue(t, td.Deadline.Equal(deadline), "Deadline matches")
	})
}

func testGetByID(t *testing.T) {
	r := NewRepository()
	created, _ := r.Create(models.Todo{Title: "Find me"})

	tests := []struct {
		name    string
		id      int
		wantErr error
	}{
		{"existing item", created.ID, nil},
		{"non-existent", 999, ErrNotFound},
		{"zero id", 0, ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := r.GetByID(tt.id)
			assertError(t, err, tt.wantErr, "GetByID error")
			if err == nil {
				assertEqual(t, got.ID, tt.id, "ID match")
			}
		})
	}
}

func testUpdate(t *testing.T) {
	r := NewRepository()
	created, _ := r.Create(models.Todo{Title: "Old"})

	t.Run("full update", func(t *testing.T) {
		deadline := time.Now().Add(time.Hour).Truncate(time.Second)
		update := models.Todo{
			Title:       "New",
			Description: "New Desc",
			Completed:   true,
			Deadline:    &deadline,
		}
		updated, err := r.Update(created.ID, update)

		assertNoError(t, err, "Update")
		assertEqual(t, updated.Title, "New", "Title")
		assertEqual(t, updated.Description, "New Desc", "Desc")
		assertTrue(t, updated.Completed, "Completed")
		assertTrue(t, updated.Deadline.Equal(deadline), "Deadline")
		assertTrue(t, updated.UpdatedAt.After(created.CreatedAt) || updated.UpdatedAt.Equal(created.CreatedAt), "UpdatedAt updated")
	})

	t.Run("not found", func(t *testing.T) {
		_, err := r.Update(999, models.Todo{Title: "X"})
		assertError(t, err, ErrNotFound, "Update non-existent")
	})
}

func testDelete(t *testing.T) {
	r := NewRepository()
	td, _ := r.Create(models.Todo{Title: "Delete"})

	t.Run("success", func(t *testing.T) {
		err := r.Delete(td.ID)
		assertNoError(t, err, "Delete")
		_, err = r.GetByID(td.ID)
		assertError(t, err, ErrNotFound, "Should be gone")
	})

	t.Run("not found", func(t *testing.T) {
		err := r.Delete(999)
		assertError(t, err, ErrNotFound, "Delete non-existent")
	})
}

func testGetAll(t *testing.T) {
	r := NewRepository()
	baseTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	r.mu.Lock()
	r.TodoSets[1] = models.Todo{ID: 1, Title: "C", Completed: false, CreatedAt: baseTime}
	r.TodoSets[2] = models.Todo{ID: 2, Title: "A", Completed: true, CreatedAt: baseTime.Add(time.Hour)}
	r.TodoSets[3] = models.Todo{ID: 3, Title: "B", Completed: false, CreatedAt: baseTime.Add(-time.Hour)}
	r.NextID = 4
	r.mu.Unlock()

	t.Run("no filters", func(t *testing.T) {
		res := r.GetAll(ListOptions{Limit: -1})
		assertEqual(t, len(res), 3, "Total count")
	})

	t.Run("filter completed", func(t *testing.T) {
		completed := true
		res := r.GetAll(ListOptions{Limit: -1, CompletedFilter: &completed})
		assertEqual(t, len(res), 1, "Completed count")
		assertTrue(t, res[0].Completed, "Is completed")
	})

	t.Run("sort by title", func(t *testing.T) {
		res := r.GetAll(ListOptions{Limit: -1, SortField: "title"})
		assertEqual(t, res[0].Title, "A", "First item")
		assertEqual(t, res[1].Title, "B", "Second item")
		assertEqual(t, res[2].Title, "C", "Third item")
	})

	t.Run("sort by created_at", func(t *testing.T) {
		res := r.GetAll(ListOptions{Limit: -1, SortField: "created_at"})
		assertEqual(t, res[0].ID, 3, "Oldest (ID 3)")
		assertEqual(t, res[1].ID, 1, "Middle (ID 1)")
		assertEqual(t, res[2].ID, 2, "Newest (ID 2)")
	})

	t.Run("pagination", func(t *testing.T) {
		res := r.GetAll(ListOptions{Limit: 1, Offset: 1, SortField: "id"})
		assertEqual(t, len(res), 1, "Limit")
		assertEqual(t, res[0].ID, 2, "Offset item")
	})
}

func testConcurrentAccess(t *testing.T) {
	r := NewRepository()
	const count = 100
	var wg sync.WaitGroup
	wg.Add(count * 3)

	for i := 0; i < count; i++ {
		go func(n int) {
			defer wg.Done()
			r.Create(models.Todo{Title: fmt.Sprintf("Task %d", n)})
		}(i)
	}

	for i := 0; i < count; i++ {
		go func() {
			defer wg.Done()
			r.GetAll(ListOptions{Limit: -1})
		}()
	}

	for i := 0; i < count; i++ {
		go func(n int) {
			defer wg.Done()
			r.Update(n%10+1, models.Todo{Title: "Updated"})
		}(i)
	}

	wg.Wait()
	res := r.GetAll(ListOptions{Limit: -1})
	assertEqual(t, len(res), count, "Final count")
}
