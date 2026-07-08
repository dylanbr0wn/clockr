package ai_test

import (
	"testing"

	"github.com/dylanbr0wn/clockr/internal/ai"
)

func TestMatchCategoryKeyPrefersKey(t *testing.T) {
	categories := []ai.CategoryDefinition{
		{Key: "ACME", Name: "Acme Corp", Description: "Client work"},
		{Key: "meetings", Name: "Meetings"},
	}

	key, ok := ai.MatchCategoryKey("ACME", categories)
	if !ok || key != "ACME" {
		t.Fatalf("MatchCategoryKey = %q, %v want ACME, true", key, ok)
	}
}

func TestMatchCategoryKeyFallsBackToName(t *testing.T) {
	categories := []ai.CategoryDefinition{
		{Key: "deep-work", Name: "Deep Work"},
	}

	key, ok := ai.MatchCategoryKey("Deep Work", categories)
	if !ok || key != "deep-work" {
		t.Fatalf("MatchCategoryKey = %q, %v want deep-work, true", key, ok)
	}
}
