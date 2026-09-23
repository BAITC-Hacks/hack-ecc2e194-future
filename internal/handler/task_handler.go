package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"hack-ecc2e194-future/internal/domain"
	"hack-ecc2e194-future/internal/rating"
	"hack-ecc2e194-future/internal/store"
)

type taskHandler struct {
	store store.Store
}

// taskCreateInput is the JSON body for POST /api/tasks.
type taskCreateInput struct {
	Title  string            `json:"title"`
	Topic  string            `json:"topic"`
	Fields domain.TaskFields `json:"fields"`
	Status string            `json:"status"`
}

func (h *taskHandler) list(w http.ResponseWriter, r *http.Request) {
	if !methodOnly(w, r, http.MethodGet) {
		return
	}
	q := r.URL.Query()
	tasks, err := h.store.Tasks().List(r.Context(), q.Get("topic"), q.Get("level"))
	if err != nil {
		http.Error(w, "failed to load tasks", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (h *taskHandler) create(w http.ResponseWriter, r *http.Request) {
	if !methodOnly(w, r, http.MethodPost) {
		return
	}
	defer r.Body.Close()

	var input taskCreateInput
	if !decodeJSON(w, r, &input) {
		return
	}

	input.Title = strings.TrimSpace(input.Title)
	input.Topic = strings.TrimSpace(input.Topic)
	if input.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	if input.Topic == "" {
		input.Topic = "Общее"
	}

	status := strings.ToLower(strings.TrimSpace(input.Status))
	if status != "published" && status != "draft" {
		status = "published"
	}

	score, level, _ := rating.Calculate(input.Fields)

	id, err := newID()
	if err != nil {
		http.Error(w, "failed to generate id", http.StatusInternalServerError)
		return
	}

	task := domain.Task{
		ID:             id,
		Title:          input.Title,
		Topic:          input.Topic,
		Fields:         input.Fields,
		Rating:         score,
		ReadinessLevel: level,
		Status:         status,
	}

	created, err := h.store.Tasks().Create(r.Context(), task)
	if err != nil {
		http.Error(w, "failed to save task", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *taskHandler) calculateRating(w http.ResponseWriter, r *http.Request) {
	if !methodOnly(w, r, http.MethodPost) {
		return
	}
	defer r.Body.Close()

	var fields domain.TaskFields
	if !decodeJSON(w, r, &fields) {
		return
	}

	score, level, missing := rating.Calculate(fields)
	writeJSON(w, http.StatusOK, map[string]any{
		"rating":          score,
		"readiness_level": level,
		"empty_fields":    missing,
	})
}

func (h *taskHandler) proposals(w http.ResponseWriter, r *http.Request) {
	if !methodOnly(w, r, http.MethodGet) {
		return
	}
	// Path: /api/tasks/{id}/proposals  → parts[2] = id
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 4 {
		http.NotFound(w, r)
		return
	}
	taskID := parts[2]

	list, err := h.store.Proposals().ListByTask(r.Context(), taskID)
	if err != nil {
		http.Error(w, "failed to load proposals", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// newID generates a random hex ID.
func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
