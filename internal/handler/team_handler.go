package handler

import (
	"net/http"

	"hack-ecc2e194-future/internal/store"
)

type teamHandler struct {
	store store.Store
}

func (h *teamHandler) list(w http.ResponseWriter, r *http.Request) {
	if !methodOnly(w, r, http.MethodGet) {
		return
	}
	teams, err := h.store.Teams().List(r.Context())
	if err != nil {
		http.Error(w, "failed to load teams", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, teams)
}
