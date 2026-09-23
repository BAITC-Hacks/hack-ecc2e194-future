package handler

import (
	"net/http"

	"hack-ecc2e194-future/internal/ai"
	"hack-ecc2e194-future/internal/store"
)

// NewRouter builds and returns the HTTP router with all routes registered.
// It depends only on the Store and Analyzer interfaces, not on concrete implementations.
func NewRouter(s store.Store, analyzer ai.Analyzer) http.Handler {
	mux := http.NewServeMux()

	th := &taskHandler{store: s}
	ph := &proposalHandler{store: s}
	tmh := &teamHandler{store: s}
	ah := &aiHandler{analyzer: analyzer}

	// ── Tasks ─────────────────────────────────────────────────────────
	// GET  /api/tasks              — list (supports ?topic= and ?level=)
	// POST /api/tasks              — create / publish
	mux.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			th.list(w, r)
		case http.MethodPost:
			th.create(w, r)
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// POST /api/tasks/calculate    — score preview (no persistence)
	mux.HandleFunc("/api/tasks/calculate", th.calculateRating)

	// GET  /api/tasks/{id}/proposals
	mux.HandleFunc("/api/tasks/", th.proposals)

	// ── Teams ──────────────────────────────────────────────────────────
	// GET /api/teams
	mux.HandleFunc("/api/teams", tmh.list)

	// ── Proposals ─────────────────────────────────────────────────────
	// POST /api/proposals
	mux.HandleFunc("/api/proposals", ph.create)

	// POST /api/proposals/{id}/decision
	mux.HandleFunc("/api/proposals/", ph.decide)

	// ── AI ────────────────────────────────────────────────────────────
	// POST /api/ai/analyze-draft
	mux.HandleFunc("/api/ai/analyze-draft", ah.analyzeDraft)

	// ── Static files ──────────────────────────────────────────────────
	mux.Handle("/", http.FileServer(http.Dir("./static")))

	return mux
}
