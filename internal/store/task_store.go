package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"hack-ecc2e194-future/internal/domain"
)

type sqlTaskStore struct{ db *sql.DB }

func (s *sqlTaskStore) List(ctx context.Context, topic, level string) ([]domain.Task, error) {
	query := `SELECT id, title, topic, fields, rating, readiness_level, status
		FROM tasks WHERE lower(status) IN ('published','open')`
	args := make([]any, 0, 2)

	if t := strings.TrimSpace(topic); t != "" {
		query += ` AND lower(topic) LIKE lower(?)`
		args = append(args, "%"+t+"%")
	}
	if l := strings.ToUpper(strings.TrimSpace(level)); l != "" {
		query += ` AND upper(readiness_level) = ?`
		args = append(args, l)
	}

	query += ` ORDER BY
		CASE upper(readiness_level)
			WHEN 'PRIORITY'    THEN 0
			WHEN 'READY'       THEN 1
			WHEN 'IN_PROGRESS' THEN 2
			ELSE 3
		END, rating DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]domain.Task, 0)
	for rows.Next() {
		var t domain.Task
		var raw sql.NullString
		if err := rows.Scan(&t.ID, &t.Title, &t.Topic, &raw, &t.Rating, &t.ReadinessLevel, &t.Status); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		if raw.Valid {
			_ = json.Unmarshal([]byte(raw.String), &t.Fields)
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks: %w", err)
	}
	return tasks, nil
}

func (s *sqlTaskStore) Create(ctx context.Context, task domain.Task) (domain.Task, error) {
	fieldsJSON, err := json.Marshal(task.Fields)
	if err != nil {
		return domain.Task{}, fmt.Errorf("marshal task fields: %w", err)
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO tasks (id, title, topic, fields, rating, readiness_level, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.Title, task.Topic, string(fieldsJSON),
		task.Rating, task.ReadinessLevel, task.Status,
	); err != nil {
		return domain.Task{}, fmt.Errorf("insert task: %w", err)
	}
	return task, nil
}
