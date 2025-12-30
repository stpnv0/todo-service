package repository

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
	"todo-service/internal/models"
)

var (
	ErrNotFound = errors.New("todo not found")
)

type ListOptions struct {
	Limit           int
	Offset          int
	SortField       string
	CompletedFilter *bool
}

type Repository struct {
	mu       sync.RWMutex
	TodoSets map[int]models.Todo
	NextID   int
}

func NewRepository() *Repository {
	return &Repository{
		mu:       sync.RWMutex{},
		TodoSets: make(map[int]models.Todo),
		NextID:   1,
	}
}

// Create создает tоdo
func (r *Repository) Create(td models.Todo) (models.Todo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	td.ID = r.NextID
	r.NextID++

	now := time.Now()
	td.CreatedAt = now
	td.UpdatedAt = now

	r.TodoSets[td.ID] = td

	return td, nil
}

// GetAll возвращает все todos с учетом фильтрации, сортировки и пагинации
func (r *Repository) GetAll(opts ListOptions) []models.Todo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	filtered := make([]models.Todo, 0, len(r.TodoSets))
	for _, td := range r.TodoSets {
		if opts.CompletedFilter != nil && td.Completed != *opts.CompletedFilter {
			continue
		}
		filtered = append(filtered, td)
	}

	// Сортировка
	less := func(i, j int) bool { return filtered[i].ID < filtered[j].ID }
	switch strings.ToLower(opts.SortField) {
	case "id":
		less = func(i, j int) bool { return filtered[i].ID < filtered[j].ID }
	case "title":
		less = func(i, j int) bool { return filtered[i].Title < filtered[j].Title }
	case "created_at":
		less = func(i, j int) bool { return filtered[i].CreatedAt.Before(filtered[j].CreatedAt) }
	case "updated_at":
		less = func(i, j int) bool { return filtered[i].UpdatedAt.Before(filtered[j].UpdatedAt) }
	case "completed":
		less = func(i, j int) bool {
			if filtered[i].Completed == filtered[j].Completed {
				return filtered[i].ID < filtered[j].ID
			}
			return !filtered[i].Completed && filtered[j].Completed
		}
	}

	sort.Slice(filtered, func(i, j int) bool { return less(i, j) })

	// Пагинация
	start := opts.Offset
	if start > len(filtered) {
		start = len(filtered)
	}
	end := len(filtered)
	if opts.Limit >= 0 {
		end = start + opts.Limit
		if end > len(filtered) {
			end = len(filtered)
		}
	}

	return filtered[start:end]
}

// GetByID возвращает todo по id
func (r *Repository) GetByID(ID int) (models.Todo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	td, ok := r.TodoSets[ID]
	if !ok {
		return models.Todo{}, ErrNotFound
	}

	return td, nil
}

// Update обновляет все поля структуры
func (r *Repository) Update(ID int, update models.Todo) (models.Todo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	td, ok := r.TodoSets[ID]
	if !ok {
		return models.Todo{}, ErrNotFound
	}

td.Title = update.Title
td.Description = update.Description
td.Completed = update.Completed
td.Deadline = update.Deadline

	td.UpdatedAt = time.Now()

	r.TodoSets[ID] = td

	return td, nil
}

// Delete удаляет tоdo по id
func (r *Repository) Delete(ID int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.TodoSets[ID]; !ok {
		return ErrNotFound
	}

	delete(r.TodoSets, ID)
	return nil
}
