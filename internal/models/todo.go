package models

import (
	"errors"
	"time"
)

type Todo struct {
	ID          int        `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Completed   bool       `json:"completed"`
	Deadline    *time.Time `json:"deadline"` // указатель, т.к. deadline может быть не установлен
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (t *Todo) Validate() error {
	// В описании задания в пункте валидации было указано, что заголовок не должен отсутствовать,
	// про другие поля, например, Description, ничего не сказано, поэтому они не проверяются
	// за исключением добавленног
	if t.Title == "" {
		return errors.New("title cannot be empty")
	}
	if t.Deadline != nil && t.Deadline.Before(time.Now()) {
		return errors.New("deadline cannot be in the past")
	}
	return nil
}
