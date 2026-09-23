package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"hack-ecc2e194-future/internal/ai"
	"hack-ecc2e194-future/internal/handler"
	"hack-ecc2e194-future/internal/store"

	_ "modernc.org/sqlite"
)

func main() {
	// ── Database ──────────────────────────────────────────────────────
	db, err := sql.Open("sqlite", "app.db")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	// ── Schema ────────────────────────────────────────────────────────
	if err := store.ApplySchema(db); err != nil {
		log.Fatalf("apply schema: %v", err)
	}

	// ── Seed ──────────────────────────────────────────────────────────
	if err := store.Seed(db); err != nil {
		log.Fatalf("seed: %v", err)
	}

	// ── Dependencies ──────────────────────────────────────────────────
	s := store.New(db)
	analyzer := ai.New(os.Getenv("OPENAI_API_KEY"))

	// ── Router ────────────────────────────────────────────────────────
	router := handler.NewRouter(s, analyzer)

	// ── Server ────────────────────────────────────────────────────────
	addr := ":8080"
	log.Printf("Server listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, router))
}
