package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	maxRequestBodySize = 1 << 20
	maxTitleLength     = 200
	maxDescriptionSize = 100 * 1024
)

type Task struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type CreateTaskRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

var (
	tasks  = make([]Task, 0)
	nextID = 1

	tasksMu sync.RWMutex
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/tasks", handleTasks)
	mux.HandleFunc("/tasks/{id}", handleTasksByID)

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("Server starting on %s", server.Addr)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("Server error: %v", err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getTasks(w, r)

	case http.MethodPost:
		createTask(w, r)

	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func handleTasksByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	idString := r.PathValue("id")

	id, err := strconv.Atoi(idString)
	if err != nil || id <= 0 {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	tasksMu.RLock()

	var foundTask Task
	found := false

	for i := range tasks {
		if tasks[i].ID == id {
			foundTask = tasks[i]
			found = true
			break
		}
	}

	tasksMu.RUnlock()

	if !found {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, foundTask)
}

func getTasks(w http.ResponseWriter, r *http.Request) {
	tasksMu.RLock()

	tasksSnapshot := make([]Task, len(tasks))
	copy(tasksSnapshot, tasks)

	tasksMu.RUnlock()

	writeJSON(w, http.StatusOK, tasksSnapshot)
}

func createTask(w http.ResponseWriter, r *http.Request) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		http.Error(
			w,
			"Content-Type must be application/json",
			http.StatusUnsupportedMediaType,
		)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)

	var request CreateTaskRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		var maxBytesError *http.MaxBytesError

		if errors.As(err, &maxBytesError) {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}

		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			http.Error(w, "Request must contain exactly one JSON object", http.StatusBadRequest)
			return
		}

		var maxBytesError *http.MaxBytesError

		if errors.As(err, &maxBytesError) {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}

		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	request.Title = strings.TrimSpace(request.Title)

	if request.Title == "" {
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}

	if utf8.RuneCountInString(request.Title) > maxTitleLength {
		http.Error(w, "Title must not exceed 200 characters", http.StatusBadRequest)
		return
	}

	if len(request.Description) > maxDescriptionSize {
		http.Error(w, "Description is too large", http.StatusBadRequest)
		return
	}

	task := Task{
		Title:       request.Title,
		Description: request.Description,
		Status:      "todo",
	}

	tasksMu.Lock()

	task.ID = nextID
	nextID++

	tasks = append(tasks, task)

	tasksMu.Unlock()

	writeJSON(w, http.StatusCreated, task)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("JSON response encoding error: %v", err)
	}
}
