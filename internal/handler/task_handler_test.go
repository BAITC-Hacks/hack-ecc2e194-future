package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTaskProposalsRejectsUnknownPathSuffix(t *testing.T) {
	handler := &taskHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/tasks/task-1/not-proposals", nil)
	response := httptest.NewRecorder()

	handler.proposals(response, req)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestTaskProposalsRejectsMissingTaskID(t *testing.T) {
	handler := &taskHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/tasks//proposals", nil)
	response := httptest.NewRecorder()

	handler.proposals(response, req)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}
