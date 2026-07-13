package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
)

// ErrProjectInUse is returned when a project cannot be deleted because it is referenced.
var ErrProjectInUse = errors.New("project in use")

// CreateProjectInput is the payload for creating a project.
type CreateProjectInput struct {
	Name  string `json:"name"`
	Key   string `json:"key"`
	Color string `json:"color"`
}

// UpdateProjectInput is the payload for updating a project.
type UpdateProjectInput struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Key   string `json:"key"`
	Color string `json:"color"`
}

func normalizeProjectKey(name, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return strings.TrimSpace(name)
	}
	return key
}

func projectColorParam(color string) (sql.NullString, error) {
	color = strings.TrimSpace(color)
	if color == "" {
		return sql.NullString{}, nil
	}
	if err := ValidateCategoryColor(color); err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: NormalizeCategoryColor(color), Valid: true}, nil
}

func (s *Service) CreateProject(ctx context.Context, input CreateProjectInput) (Project, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Project{}, fmt.Errorf("create project: name is required")
	}

	key := normalizeProjectKey(name, input.Key)
	if key == "" {
		return Project{}, fmt.Errorf("create project: key is required")
	}

	color, err := projectColorParam(input.Color)
	if err != nil {
		return Project{}, fmt.Errorf("create project: %w", err)
	}

	row, err := s.q.CreateProject(ctx, sqlc.CreateProjectParams{
		Name:  name,
		Key:   key,
		Color: color,
	})
	if err != nil {
		return Project{}, mapErr("create project", err)
	}
	return toProject(row), nil
}

func (s *Service) UpdateProject(ctx context.Context, input UpdateProjectInput) (Project, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Project{}, fmt.Errorf("update project: name is required")
	}

	key := normalizeProjectKey(name, input.Key)
	if key == "" {
		return Project{}, fmt.Errorf("update project: key is required")
	}

	if _, err := s.q.GetProject(ctx, input.ID); err != nil {
		return Project{}, mapErr("update project", err)
	}

	color, err := projectColorParam(input.Color)
	if err != nil {
		return Project{}, fmt.Errorf("update project: %w", err)
	}

	row, err := s.q.UpdateProject(ctx, sqlc.UpdateProjectParams{
		Name:  name,
		Key:   key,
		Color: color,
		ID:    input.ID,
	})
	if err != nil {
		return Project{}, mapErr("update project", err)
	}
	return toProject(row), nil
}

func (s *Service) DeleteProject(ctx context.Context, id int64) error {
	if _, err := s.q.GetProject(ctx, id); err != nil {
		return mapErr("delete project", err)
	}

	inUse, err := s.projectInUse(ctx, id)
	if err != nil {
		return err
	}
	if inUse {
		return fmt.Errorf("delete project: %w", ErrProjectInUse)
	}

	if err := s.q.DeleteProject(ctx, id); err != nil {
		return mapErr("delete project", err)
	}
	return nil
}

func (s *Service) ArchiveProject(ctx context.Context, id int64) (Project, error) {
	current, err := s.q.GetProject(ctx, id)
	if err != nil {
		return Project{}, mapErr("archive project", err)
	}
	if current.ArchivedAt.Valid {
		return toProject(current), nil
	}

	row, err := s.q.ArchiveProject(ctx, sqlc.ArchiveProjectParams{
		ArchivedAt: sql.NullString{String: time.Now().UTC().Format(time.RFC3339), Valid: true},
		ID:         id,
	})
	if err != nil {
		return Project{}, mapErr("archive project", err)
	}
	return toProject(row), nil
}
