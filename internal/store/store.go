package store

import (
	"context"
	"database/sql"

	"hack-ecc2e194-future/internal/domain"
)

// TaskStore defines task persistence operations.
type TaskStore interface {
	List(ctx context.Context, topic, level string) ([]domain.Task, error)
	Create(ctx context.Context, task domain.Task) (domain.Task, error)
}

// TeamStore defines team persistence operations.
type TeamStore interface {
	List(ctx context.Context) ([]domain.Team, error)
}

// ProposalStore defines proposal persistence operations.
type ProposalStore interface {
	Create(ctx context.Context, p domain.Proposal) (domain.Proposal, error)
	ListByTask(ctx context.Context, taskID string) ([]domain.Proposal, error)
	Decide(ctx context.Context, id, status string) error
}

// Store is the top-level repository that groups all sub-stores.
type Store interface {
	Tasks() TaskStore
	Teams() TeamStore
	Proposals() ProposalStore
}

// sqlStore implements Store using a *sql.DB.
type sqlStore struct {
	tasks     TaskStore
	teams     TeamStore
	proposals ProposalStore
}

// New creates a Store backed by the given sql.DB.
// It does not own the database connection — the caller is responsible for Close.
func New(db *sql.DB) Store {
	return &sqlStore{
		tasks:     &sqlTaskStore{db: db},
		teams:     &sqlTeamStore{db: db},
		proposals: &sqlProposalStore{db: db},
	}
}

func (s *sqlStore) Tasks() TaskStore         { return s.tasks }
func (s *sqlStore) Teams() TeamStore         { return s.teams }
func (s *sqlStore) Proposals() ProposalStore { return s.proposals }
