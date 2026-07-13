package service

import (
	"context"
	"fmt"
	"time"

	"github.com/dylanbr0wn/shiet/internal/ai"
)

// GapSuggestion is an AI-proposed category and description for an uncovered
// interval. It is not persisted until the user confirms a gap fill.
type GapSuggestion struct {
	Category      string `json:"category"`
	Description   string `json:"description"`
	EvidenceCount int    `json:"evidenceCount"`
}

// ListGapEvidence returns aggregated activity evidence for a gap window.
func (s *Service) ListGapEvidence(ctx context.Context, window TimeWindow) ([]ActivityEvidence, error) {
	if !window.Start.Before(window.End) {
		return nil, invalidInputf("list gap evidence: invalid time window")
	}
	return s.fetchGapEvidence(ctx, window)
}

// SuggestGapFill asks the configured model to propose a category and description
// for an uncovered interval, using aggregated activity evidence as context.
func (s *Service) SuggestGapFill(ctx context.Context, window TimeWindow) (GapSuggestion, error) {
	if !window.Start.Before(window.End) {
		return GapSuggestion{}, invalidInputf("suggest gap fill: invalid time window")
	}

	baseURL, model, ok := s.loadAIConfig(ctx)
	if !ok {
		return GapSuggestion{}, failedPreconditionf("suggest gap fill: ai not configured")
	}

	evidence, err := s.fetchGapEvidence(ctx, window)
	if err != nil {
		return GapSuggestion{}, fmt.Errorf("suggest gap fill: fetch evidence: %w", err)
	}

	categories, err := s.ListCategories(ctx)
	if err != nil {
		return GapSuggestion{}, err
	}
	definitions := categoryDefinitionsForAI(categories)

	local, _ := ai.ClassifyEndpoint(baseURL)

	probeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	client := ai.NewClient(baseURL, "")
	categoryKey, description, err := ai.SuggestGapFill(
		probeCtx,
		client,
		model,
		definitions,
		constToGapContext(window),
		buildEvidencePayload(evidence, local),
		local,
		s.loadAIMaxTokens(ctx),
	)
	if err != nil {
		return GapSuggestion{}, mapErr("suggest gap fill", err)
	}
	category, ok := resolveCategoryKey(categories, categoryKey)
	if !ok {
		return GapSuggestion{}, fmt.Errorf("suggest gap fill: model returned unknown category %q", categoryKey)
	}

	return GapSuggestion{
		Category:      category.Name,
		Description:   description,
		EvidenceCount: len(evidence),
	}, nil
}

func constToGapContext(window TimeWindow) ai.GapContext {
	duration := window.End.Sub(window.Start).Round(time.Minute)
	return ai.GapContext{
		Start:    window.Start,
		End:      window.End,
		Duration: duration.String(),
	}
}

func buildEvidencePayload(evidence []ActivityEvidence, local bool) []ai.EvidencePayload {
	out := make([]ai.EvidencePayload, 0, len(evidence))
	for _, item := range evidence {
		payload := ai.EvidencePayload{
			Provider: item.Provider,
			Kind:     item.Kind,
			Summary:  item.Summary,
			Source:   item.Source,
		}
		if local {
			payload.Detail = item.Detail
			payload.URL = item.URL
			payload.Start = item.Start.UTC().Format(time.RFC3339)
			payload.End = item.End.UTC().Format(time.RFC3339)
		}
		out = append(out, payload)
	}
	return out
}
