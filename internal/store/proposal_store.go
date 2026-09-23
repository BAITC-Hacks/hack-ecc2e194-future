package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"hack-ecc2e194-future/internal/domain"
)

type sqlProposalStore struct{ db *sql.DB }

func (s *sqlProposalStore) Create(ctx context.Context, p domain.Proposal) (domain.Proposal, error) {
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO proposals (id, task_id, team_id, solution_idea, plan, status)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, p.TaskID, p.TeamID, p.SolutionIdea, p.Plan, p.Status,
	); err != nil {
		return domain.Proposal{}, fmt.Errorf("insert proposal: %w", err)
	}
	// Resolve team name.
	_ = s.db.QueryRowContext(ctx, `SELECT name FROM teams WHERE id = ?`, p.TeamID).Scan(&p.TeamName)
	return p, nil
}

func (s *sqlProposalStore) ListByTask(ctx context.Context, taskID string) ([]domain.Proposal, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.task_id, p.team_id, COALESCE(t.name,''), p.solution_idea, p.plan, p.status
		FROM proposals p
		LEFT JOIN teams t ON t.id = p.team_id
		WHERE p.task_id = ?
		ORDER BY p.rowid DESC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("query proposals: %w", err)
	}
	defer rows.Close()

	list := make([]domain.Proposal, 0)
	for rows.Next() {
		var p domain.Proposal
		if err := rows.Scan(&p.ID, &p.TaskID, &p.TeamID, &p.TeamName, &p.SolutionIdea, &p.Plan, &p.Status); err != nil {
			return nil, fmt.Errorf("scan proposal: %w", err)
		}
		list = append(list, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate proposals: %w", err)
	}
	return list, nil
}

func (s *sqlProposalStore) Decide(ctx context.Context, id, status string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE proposals SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return fmt.Errorf("update proposal: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return errors.New("proposal not found")
	}
	return nil
}
