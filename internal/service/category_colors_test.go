package service_test

import (
	"context"
	"testing"

	"github.com/dylanbr0wn/clockr/internal/service"
)

func TestValidateCategoryColor(t *testing.T) {
	t.Parallel()

	if err := service.ValidateCategoryColor("#0EA5E9"); err != nil {
		t.Fatalf("expected palette color to be valid: %v", err)
	}
	if err := service.ValidateCategoryColor("#0ea5e9"); err != nil {
		t.Fatalf("expected lowercase palette color to be valid: %v", err)
	}
	if err := service.ValidateCategoryColor("#123456"); err == nil {
		t.Fatal("expected arbitrary hex to be rejected")
	}
}

func TestSetCategoryColor(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	cats, err := s.ListCategories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) == 0 {
		t.Fatal("expected seeded categories")
	}

	cat := cats[0]
	if cat.Color == "" {
		t.Fatalf("expected seeded category color, got %+v", cat)
	}

	if err := s.SetCategoryColor(ctx, cat.ID, "#10B981"); err != nil {
		t.Fatalf("SetCategoryColor: %v", err)
	}

	got, err := s.GetCategory(ctx, cat.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Color != "#10B981" {
		t.Fatalf("color = %q, want #10B981", got.Color)
	}

	if err := s.SetCategoryColor(ctx, cat.ID, "#bad"); err == nil {
		t.Fatal("expected invalid color to fail")
	}
}

func TestListEventCategoryOverlays(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	periods, err := s.ListPeriods(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(periods) == 0 {
		t.Fatal("expected seeded period")
	}

	overlays, err := s.ListEventCategoryOverlays(ctx, periods[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if overlays == nil {
		t.Fatal("expected non-nil overlay slice")
	}
}
