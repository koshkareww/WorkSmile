package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupTestTasks() {
	tasksMu.Lock()
	defer tasksMu.Unlock()

	tasks = []Task{
		{
			ID:          1,
			Title:       "Learn Go",
			Description: "Study HTTP handlers",
			Status:      "todo",
		},
	}

	nextID = 2
}

func TestHandleTasksByID(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		id         string
		wantStatus int
	}{
		{
			name:       "existing task",
			method:     http.MethodGet,
			id:         "1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "task not found",
			method:     http.MethodGet,
			id:         "999",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "zero ID",
			method:     http.MethodGet,
			id:         "0",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "negative ID",
			method:     http.MethodGet,
			id:         "-1",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "non-numeric ID",
			method:     http.MethodGet,
			id:         "abc",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unsupported method",
			method:     http.MethodPost,
			id:         "1",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTestTasks()

			req := httptest.NewRequest(
				tt.method,
				"/tasks/"+tt.id,
				nil,
			)

			req.SetPathValue("id", tt.id)

			recorder := httptest.NewRecorder()

			handleTasksByID(recorder, req)

			if recorder.Code != tt.wantStatus {
				t.Fatalf(
					"Expected status %d, got %d",
					tt.wantStatus,
					recorder.Code,
				)
			}

			if tt.wantStatus == http.StatusOK {
				var got Task

				if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
					t.Fatalf("Failed to decode response JSON: %v", err)
				}

				if got.ID != 1 {
					t.Errorf("Expected task ID 1, got %d", got.ID)
				}

				if got.Title != "Learn Go" {
					t.Errorf("Expected title 'Learn Go', got %q", got.Title)
				}

				if got.Description != "Study HTTP handlers" {
					t.Errorf(
						"Expected description 'Study HTTP handlers', got %q",
						got.Description,
					)
				}

				if got.Status != "todo" {
					t.Errorf("Expected status 'todo', got %q", got.Status)
				}
			}
		})
	}
}
