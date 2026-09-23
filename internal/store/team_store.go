package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"hack-ecc2e194-future/internal/domain"
)

type sqlTeamStore struct{ db *sql.DB }

func (s *sqlTeamStore) List(ctx context.Context) ([]domain.Team, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, skills, focus FROM teams ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query teams: %w", err)
	}
	defer rows.Close()

	teams := make([]domain.Team, 0)
	for rows.Next() {
		var t domain.Team
		var skillsRaw sql.NullString
		if err := rows.Scan(&t.ID, &t.Name, &skillsRaw, &t.Focus); err != nil {
			return nil, fmt.Errorf("scan team: %w", err)
		}
		if skillsRaw.Valid {
			_ = json.Unmarshal([]byte(skillsRaw.String), &t.Skills)
		}
		if t.Skills == nil {
			t.Skills = []string{}
		}
		teams = append(teams, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate teams: %w", err)
	}
	return teams, nil
}
