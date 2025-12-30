package models

import (
	"testing"
	"time"
)

func TestTodo_Validate(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	future := time.Now().Add(1 * time.Hour)

	tests := []struct {
		name    string
		todo    Todo
		wantErr bool
	}{
		{
			name:    "valid todo without deadline",
			todo:    Todo{Title: "Task"},
			wantErr: false,
		},
		{
			name:    "valid todo with future deadline",
			todo:    Todo{Title: "Task", Deadline: &future},
			wantErr: false,
		},
		{
			name:    "empty title",
			todo:    Todo{Title: ""},
			wantErr: true,
		},
		{
			name:    "past deadline",
			todo:    Todo{Title: "Task", Deadline: &past},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.todo.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
