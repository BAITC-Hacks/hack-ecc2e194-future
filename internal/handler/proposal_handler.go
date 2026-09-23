package handler

import (
	"errors"
	"net/http"
	"strings"

	"hack-ecc2e194-future/internal/domain"
	"hack-ecc2e194-future/internal/store"
)

type proposalHandler struct {
	store store.Store
}

type proposalCreateInput struct {
	TaskID       string `json:"task_id"`
	TeamID       string `json:"team_id"`
	SolutionIdea string `json:"solution_idea"`
	Plan         string `json:"plan"`
}

type decisionInput struct {
	Status string `json:"status"`
}

func (h *proposalHandler) create(w http.ResponseWriter, r *http.Request) {
	if !methodOnly(w, r, http.MethodPost) {
		return
	}
	defer r.Body.Close()

	var input proposalCreateInput
	if !decodeJSON(w, r, &input) {
		return
	}

	input.TaskID = strings.TrimSpace(input.TaskID)
	input.TeamID = strings.TrimSpace(input.TeamID)
	input.SolutionIdea = strings.TrimSpace(input.SolutionIdea)

	if input.TaskID == "" || input.TeamID == "" || input.SolutionIdea == "" {
		http.Error(w, "task_id, team_id and solution_idea are required", http.StatusBadRequest)
		return
	}

	// Validate task exists via store (List with exact id isn't ideal — use a simple existence check via proposals quirk).
	// We re-use the task store's List with no filters and check membership to keep things simple.
	tasks, err := h.store.Tasks().List(r.Context(), "", "")
	if err != nil {
		http.Error(w, "failed to validate task", http.StatusInternalServerError)
		return
	}
	taskFound := false
	for _, t := range tasks {
		if t.ID == input.TaskID {
			taskFound = true
			break
		}
	}
	if !taskFound {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	teams, err := h.store.Teams().List(r.Context())
	if err != nil {
		http.Error(w, "failed to validate team", http.StatusInternalServerError)
		return
	}
	teamFound := false
	for _, t := range teams {
		if t.ID == input.TeamID {
			teamFound = true
			break
		}
	}
	if !teamFound {
		http.Error(w, "team not found", http.StatusNotFound)
		return
	}

	id, err := newID()
	if err != nil {
		http.Error(w, "failed to generate id", http.StatusInternalServerError)
		return
	}

	proposal := domain.Proposal{
		ID:           id,
		TaskID:       input.TaskID,
		TeamID:       input.TeamID,
		SolutionIdea: input.SolutionIdea,
		Plan:         strings.TrimSpace(input.Plan),
		Status:       "PENDING",
	}

	created, err := h.store.Proposals().Create(r.Context(), proposal)
	if err != nil {
		http.Error(w, "failed to save proposal", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *proposalHandler) decide(w http.ResponseWriter, r *http.Request) {
	if !methodOnly(w, r, http.MethodPost) {
		return
	}
	defer r.Body.Close()

	// Path: /api/proposals/{id}/decision
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 4 || parts[3] != "decision" {
		http.NotFound(w, r)
		return
	}
	proposalID := parts[2]

	var input decisionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Status != "ACCEPTED" && input.Status != "REJECTED" {
		http.Error(w, `status must be "ACCEPTED" or "REJECTED"`, http.StatusBadRequest)
		return
	}

	if err := h.store.Proposals().Decide(r.Context(), proposalID, input.Status); err != nil {
		if errors.Is(err, errNotFound) || strings.Contains(err.Error(), "not found") {
			http.Error(w, "proposal not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to update proposal", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"id": proposalID, "status": input.Status})
}

var errNotFound = errors.New("not found")
