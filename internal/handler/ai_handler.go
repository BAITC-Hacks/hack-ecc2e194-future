package handler

import (
	"log"
	"net/http"
	"strings"

	"hack-ecc2e194-future/internal/ai"
)

type aiHandler struct {
	analyzer ai.Analyzer
}

type analyzeDraftInput struct {
	DraftText string `json:"draft_text"`
}

func (h *aiHandler) analyzeDraft(w http.ResponseWriter, r *http.Request) {
	if !methodOnly(w, r, http.MethodPost) {
		return
	}
	defer r.Body.Close()

	var input analyzeDraftInput
	if !decodeJSON(w, r, &input) {
		return
	}

	draft := strings.TrimSpace(input.DraftText)
	if draft == "" {
		http.Error(w, "draft_text is required", http.StatusBadRequest)
		return
	}

	result, err := h.analyzer.Analyze(r.Context(), draft)
	if err != nil {
		log.Printf("AI analysis failed: %v", err)
		http.Error(w, "analysis failed", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, result)
}
