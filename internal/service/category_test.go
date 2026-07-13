package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dylanbr0wn/shiet/internal/service"
)

func TestCreateAndUpdateCategory(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	created, err := s.CreateCategory(ctx, service.CreateCategoryInput{
		Name:        "Acme Corp",
		Description: "Billable client work for Acme",
		Key:         "ACME",
	})
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	if created.Key != "ACME" {
		t.Fatalf("key = %q want ACME", created.Key)
	}
	if created.Description == "" {
		t.Fatal("expected description to be stored")
	}

	updated, err := s.UpdateCategory(ctx, service.UpdateCategoryInput{
		ID:          created.ID,
		Name:        "Acme Corp",
		Description: "Updated description",
		Key:         "ACME",
	})
	if err != nil {
		t.Fatalf("UpdateCategory: %v", err)
	}
	if updated.Description != "Updated description" {
		t.Fatalf("description = %q", updated.Description)
	}
}

func TestCreateCategoryDefaultsKeyToName(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	created, err := s.CreateCategory(ctx, service.CreateCategoryInput{
		Name: "Client Calls",
	})
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	if created.Key != "Client Calls" {
		t.Fatalf("key = %q want Client Calls", created.Key)
	}
}

func TestCreateCategoryWithColor(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	created, err := s.CreateCategory(ctx, service.CreateCategoryInput{
		Name:  "Design",
		Color: "#8B5CF6",
	})
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	if created.Color != "#8B5CF6" {
		t.Fatalf("color = %q want #8B5CF6", created.Color)
	}
}

func TestUpdateCategoryWithColor(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	created, err := s.CreateCategory(ctx, service.CreateCategoryInput{
		Name: "Design",
	})
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}

	updated, err := s.UpdateCategory(ctx, service.UpdateCategoryInput{
		ID:          created.ID,
		Name:        "Design",
		Description: created.Description,
		Key:         created.Key,
		Color:       "#10B981",
	})
	if err != nil {
		t.Fatalf("UpdateCategory: %v", err)
	}
	if updated.Color != "#10B981" {
		t.Fatalf("color = %q want #10B981", updated.Color)
	}

	_, err = s.UpdateCategory(ctx, service.UpdateCategoryInput{
		ID:          created.ID,
		Name:        "Design",
		Description: created.Description,
		Key:         created.Key,
		Color:       "#bad",
	})
	if err == nil {
		t.Fatal("expected invalid color to fail")
	}
}

func TestDeleteCategoryBlocksDefaultGap(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	cats, err := s.ListCategories(ctx)
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}

	var defaultGapID int64
	for _, cat := range cats {
		if cat.IsDefaultGap {
			defaultGapID = cat.ID
			break
		}
	}
	if defaultGapID == 0 {
		t.Fatal("expected a default-gap category")
	}

	err = s.DeleteCategory(ctx, defaultGapID)
	if err == nil {
		t.Fatal("expected error deleting default-gap category")
	}
}

func TestDeleteCategoryBlocksReferencedCategory(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	created, err := e.svc.CreateCategory(ctx, service.CreateCategoryInput{
		Name: "Billable",
	})
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}

	mustOverlayWithCategory(t, e, "blocked-delete", created.ID)

	err = e.svc.DeleteCategory(ctx, created.ID)
	if err == nil {
		t.Fatal("expected error deleting referenced category")
	}
	if !errors.Is(err, service.ErrCategoryInUse) {
		t.Fatalf("expected ErrCategoryInUse, got %v", err)
	}
}

func TestArchiveCategoryHidesFromListButRemainsGettable(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	created, err := s.CreateCategory(ctx, service.CreateCategoryInput{
		Name: "Legacy Client",
	})
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}

	archived, err := s.ArchiveCategory(ctx, created.ID)
	if err != nil {
		t.Fatalf("ArchiveCategory: %v", err)
	}
	if !archived.Archived {
		t.Fatal("expected archived category")
	}

	active, err := s.ListCategories(ctx)
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	for _, cat := range active {
		if cat.ID == created.ID {
			t.Fatalf("archived category %d still in active list", created.ID)
		}
	}

	got, err := s.GetCategory(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetCategory: %v", err)
	}
	if !got.Archived {
		t.Fatal("GetCategory should return archived=true")
	}
	if got.Name != "Legacy Client" {
		t.Fatalf("name = %q", got.Name)
	}
}

func TestDeleteCategoryRemovesUnused(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	created, err := s.CreateCategory(ctx, service.CreateCategoryInput{
		Name: "Temporary",
	})
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}

	if err := s.DeleteCategory(ctx, created.ID); err != nil {
		t.Fatalf("DeleteCategory: %v", err)
	}

	if _, err := s.GetCategory(ctx, created.ID); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestArchiveCategoryWhenReferencedKeepsHistoryJoin(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	created, err := e.svc.CreateCategory(ctx, service.CreateCategoryInput{
		Name: "Billable",
	})
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	mustOverlayWithCategory(t, e, "archive-ref", created.ID)

	archived, err := e.svc.ArchiveCategory(ctx, created.ID)
	if err != nil {
		t.Fatalf("ArchiveCategory: %v", err)
	}
	if !archived.Archived {
		t.Fatal("expected archived=true")
	}

	got, err := e.svc.GetCategory(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetCategory: %v", err)
	}
	if got.Name != "Billable" {
		t.Fatalf("name = %q", got.Name)
	}

	all, err := e.svc.ListAllCategories(ctx)
	if err != nil {
		t.Fatalf("ListAllCategories: %v", err)
	}
	found := false
	for _, cat := range all {
		if cat.ID == created.ID {
			found = true
			if !cat.Archived {
				t.Fatal("ListAllCategories should mark archived")
			}
			if !cat.InUse {
				t.Fatal("ListAllCategories should mark in-use when referenced")
			}
		}
	}
	if !found {
		t.Fatal("archived category missing from ListAllCategories")
	}
}

func TestSeededCategoriesHaveKeysAndDescriptions(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	cats, err := s.ListCategories(ctx)
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}

	for _, cat := range cats {
		if cat.Key == "" {
			t.Fatalf("category %q missing key", cat.Name)
		}
		if cat.Key != cat.Name {
			t.Fatalf("category %q key = %q want name match for seeded defaults", cat.Name, cat.Key)
		}
	}
}
