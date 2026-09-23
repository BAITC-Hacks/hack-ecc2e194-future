package store

import (
	"database/sql"
	"log"
)

// ApplySchema creates all tables if they do not already exist.
func ApplySchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS tasks (
			id              TEXT PRIMARY KEY,
			title           TEXT NOT NULL DEFAULT '',
			topic           TEXT NOT NULL DEFAULT '',
			fields          JSON,
			rating          INTEGER NOT NULL DEFAULT 0,
			readiness_level TEXT NOT NULL DEFAULT 'DRAFT',
			status          TEXT NOT NULL DEFAULT 'published'
		)`,
		`CREATE TABLE IF NOT EXISTS teams (
			id     TEXT PRIMARY KEY,
			name   TEXT NOT NULL DEFAULT '',
			skills JSON,
			focus  TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS proposals (
			id            TEXT PRIMARY KEY,
			task_id       TEXT NOT NULL,
			team_id       TEXT NOT NULL,
			solution_idea TEXT NOT NULL DEFAULT '',
			plan          TEXT NOT NULL DEFAULT '',
			status        TEXT NOT NULL DEFAULT 'PENDING'
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	log.Println("Schema applied.")
	return nil
}
