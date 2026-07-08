package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/dylanbr0wn/clockr/internal/ai"
	"github.com/dylanbr0wn/clockr/internal/db/sqlc"
)

// ErrCategoryInUse is returned when a category cannot be deleted because it is referenced.
var ErrCategoryInUse = errors.New("category in use")

// CreateCategoryInput is the payload for creating a category.
type CreateCategoryInput struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Key          string `json:"key"`
	Color        string `json:"color"`
	IsDefaultGap bool   `json:"isDefaultGap"`
}

// UpdateCategoryInput is the payload for updating a category.
type UpdateCategoryInput struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Key          string `json:"key"`
	IsDefaultGap bool   `json:"isDefaultGap"`
}

func normalizeCategoryKey(name, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return strings.TrimSpace(name)
	}
	return key
}

func (s *Service) CreateCategory(ctx context.Context, input CreateCategoryInput) (Category, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Category{}, fmt.Errorf("create category: name is required")
	}

	key := normalizeCategoryKey(name, input.Key)
	if key == "" {
		return Category{}, fmt.Errorf("create category: key is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Category{}, fmt.Errorf("create category: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	q := s.q.WithTx(tx)
	if input.IsDefaultGap {
		if err := q.ClearDefaultGap(ctx); err != nil {
			return Category{}, mapErr("create category", err)
		}
	}

	gap := int64(0)
	if input.IsDefaultGap {
		gap = 1
	}

	color := strings.TrimSpace(input.Color)
	if color == "" {
		color = DefaultCategoryColor
	} else if err := ValidateCategoryColor(color); err != nil {
		return Category{}, fmt.Errorf("create category: %w", err)
	} else {
		color = NormalizeCategoryColor(color)
	}

	row, err := q.CreateCategory(ctx, sqlc.CreateCategoryParams{
		Name:         name,
		Description:  strings.TrimSpace(input.Description),
		Key:          key,
		IsDefaultGap: gap,
		Color:        color,
	})
	if err != nil {
		return Category{}, mapErr("create category", err)
	}

	if err := tx.Commit(); err != nil {
		return Category{}, fmt.Errorf("create category: %w", err)
	}
	return toCategory(row), nil
}

func (s *Service) UpdateCategory(ctx context.Context, input UpdateCategoryInput) (Category, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Category{}, fmt.Errorf("update category: name is required")
	}

	key := normalizeCategoryKey(name, input.Key)
	if key == "" {
		return Category{}, fmt.Errorf("update category: key is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Category{}, fmt.Errorf("update category: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	q := s.q.WithTx(tx)
	current, err := q.GetCategory(ctx, input.ID)
	if err != nil {
		return Category{}, mapErr("update category", err)
	}

	if input.IsDefaultGap {
		if err := q.ClearDefaultGap(ctx); err != nil {
			return Category{}, mapErr("update category", err)
		}
		if err := q.SetDefaultGap(ctx, input.ID); err != nil {
			return Category{}, mapErr("update category", err)
		}
	} else if current.IsDefaultGap != 0 {
		return Category{}, fmt.Errorf("update category: exactly one default-gap category is required")
	}

	if err := q.UpdateCategory(ctx, sqlc.UpdateCategoryParams{
		Name:        name,
		Description: strings.TrimSpace(input.Description),
		Key:         key,
		ID:          input.ID,
	}); err != nil {
		return Category{}, mapErr("update category", err)
	}

	if err := tx.Commit(); err != nil {
		return Category{}, fmt.Errorf("update category: %w", err)
	}

	updated, err := s.q.GetCategory(ctx, input.ID)
	if err != nil {
		return Category{}, mapErr("update category", err)
	}
	return toCategory(updated), nil
}

func (s *Service) DeleteCategory(ctx context.Context, id int64) error {
	current, err := s.q.GetCategory(ctx, id)
	if err != nil {
		return mapErr("delete category", err)
	}
	if current.IsDefaultGap != 0 {
		return fmt.Errorf("delete category: cannot delete the default-gap category")
	}

	ref := sql.NullInt64{Int64: id, Valid: true}

	if count, err := s.q.CountOverlayReferencesToCategory(ctx, ref); err != nil {
		return mapErr("delete category", err)
	} else if count > 0 {
		return fmt.Errorf("delete category: %w (overlay)", ErrCategoryInUse)
	}

	if count, err := s.q.CountMemoryReferencesToCategory(ctx, id); err != nil {
		return mapErr("delete category", err)
	} else if count > 0 {
		return fmt.Errorf("delete category: %w (memory)", ErrCategoryInUse)
	}

	if count, err := s.q.CountCalendarReferencesToCategory(ctx, ref); err != nil {
		return mapErr("delete category", err)
	} else if count > 0 {
		return fmt.Errorf("delete category: %w (calendar)", ErrCategoryInUse)
	}

	if count, err := s.q.CountGapFillReferencesToCategory(ctx, ref); err != nil {
		return mapErr("delete category", err)
	} else if count > 0 {
		return fmt.Errorf("delete category: %w (gap fill)", ErrCategoryInUse)
	}

	if err := s.q.DeleteCategory(ctx, id); err != nil {
		return mapErr("delete category", err)
	}
	return nil
}

func categoryDefinitionsForAI(categories []Category) []ai.CategoryDefinition {
	out := make([]ai.CategoryDefinition, len(categories))
	for i, category := range categories {
		out[i] = ai.CategoryDefinition{
			Key:         category.Key,
			Name:        category.Name,
			Description: category.Description,
		}
	}
	return out
}

func resolveCategoryKey(categories []Category, key string) (Category, bool) {
	for _, category := range categories {
		if strings.EqualFold(key, category.Key) {
			return category, true
		}
	}
	for _, category := range categories {
		if strings.EqualFold(key, category.Name) {
			return category, true
		}
	}
	return Category{}, false
}
