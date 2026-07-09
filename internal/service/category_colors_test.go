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
