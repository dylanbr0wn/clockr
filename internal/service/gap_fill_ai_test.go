package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dylanbr0wn/shiet/internal/service"
)

type stubEvidenceProvider struct {
	provider string
	items    []service.ActivityEvidence
	err      error
	calls    int
}

func (s *stubEvidenceProvider) Provider() string {
	return s.provider
}

func (s *stubEvidenceProvider) FetchEvidence(ctx context.Context, window service.TimeWindow) ([]service.ActivityEvidence, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.items, nil
}

func TestAggregateEvidence(t *testing.T) {
	start := time.Date(2026, 6, 2, 13, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 2, 14, 0, 0, 0, time.UTC)
	window := service.TimeWindow{Start: start, End: end}

	github := &stubEvidenceProvider{
		provider: service.ProviderGitHub,
		items: []service.ActivityEvidence{{
			Provider: service.ProviderGitHub,
			Kind:     "commit",
			Start:    start.Add(10 * time.Minute),
			End:      start.Add(15 * time.Minute),
			Summary:  "Merged PR #42",
		}},
	}
	slack := &stubEvidenceProvider{
		provider: service.ProviderSlack,
		items: []service.ActivityEvidence{{
			Provider: service.ProviderSlack,
			Kind:     "message",
			Start:    start.Add(5 * time.Minute),
			End:      start.Add(6 * time.Minute),
			Summary:  "Discussed deployment",
		}},
	}
	failing := &stubEvidenceProvider{
		provider: service.ProviderBitbucket,
		err:      errors.New("offline"),
	}

	got, err := service.AggregateEvidence(context.Background(), []service.EvidenceProvider{github, failing, slack}, window)
	if err != nil {
		t.Fatalf("AggregateEvidence: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 evidence items, got %d", len(got))
	}
	if got[0].Provider != service.ProviderSlack {
		t.Fatalf("expected slack evidence first, got %s", got[0].Provider)
	}
	if github.calls != 1 || slack.calls != 1 || failing.calls != 1 {
		t.Fatalf("unexpected provider call counts: github=%d slack=%d failing=%d", github.calls, slack.calls, failing.calls)
	}
}

func TestListGapEvidenceReturnsAggregatedItems(t *testing.T) {
	start := time.Date(2026, 6, 2, 13, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 2, 14, 0, 0, 0, time.UTC)
	window := service.TimeWindow{Start: start, End: end}

	github := &stubEvidenceProvider{
		provider: service.ProviderGitHub,
		items: []service.ActivityEvidence{{
			Provider: service.ProviderGitHub,
			Kind:     "commit",
			Start:    start.Add(10 * time.Minute),
			End:      start.Add(15 * time.Minute),
			Summary:  "Merged PR #42",
			Source:   "acme/widget",
		}},
	}
	slack := &stubEvidenceProvider{
		provider: service.ProviderSlack,
		items: []service.ActivityEvidence{{
			Provider: service.ProviderSlack,
			Kind:     "message",
			Start:    start.Add(5 * time.Minute),
			End:      start.Add(6 * time.Minute),
			Summary:  "Discussed deployment",
			Source:   "#deploys",
		}},
	}

	s := newSvc(t)
	s.SetEvidence(service.EvidenceConfig{
		Providers: []service.EvidenceProvider{github, slack},
	})

	got, err := s.ListGapEvidence(context.Background(), window)
	if err != nil {
		t.Fatalf("ListGapEvidence: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 evidence items, got %d", len(got))
	}
	if got[0].Provider != service.ProviderSlack {
		t.Fatalf("expected slack evidence first, got %s", got[0].Provider)
	}
	if got[0].Source != "#deploys" {
		t.Fatalf("source = %q want #deploys", got[0].Source)
	}
}

func TestListGapEvidenceEmptyWhenNoProviders(t *testing.T) {
	s := newSvc(t)
	start := time.Date(2026, 6, 2, 13, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 2, 14, 0, 0, 0, time.UTC)

	got, err := s.ListGapEvidence(context.Background(), service.TimeWindow{Start: start, End: end})
	if err != nil {
		t.Fatalf("ListGapEvidence: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty evidence, got %d items", len(got))
	}
}

func TestListGapEvidenceRequiresValidWindow(t *testing.T) {
	s := newSvc(t)
	start := time.Date(2026, 6, 2, 14, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 2, 13, 0, 0, 0, time.UTC)

	_, err := s.ListGapEvidence(context.Background(), service.TimeWindow{Start: start, End: end})
	if err == nil {
		t.Fatal("expected error for invalid window")
	}
}

func TestSuggestGapFillUsesEvidence(t *testing.T) {
	var capturedUserPrompt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, msg := range body.Messages {
			if msg.Content != "" {
				capturedUserPrompt = msg.Content
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{
					"content": `{"category":"Deep Work","description":"Reviewed PR #42"}`,
				}},
			},
		})
	}))
	defer server.Close()

	s := newSvc(t)
	ctx := context.Background()
	start := time.Date(2026, 6, 2, 13, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 2, 14, 0, 0, 0, time.UTC)

	s.SetEvidence(service.EvidenceConfig{
		Providers: []service.EvidenceProvider{&stubEvidenceProvider{
			provider: service.ProviderGitHub,
			items: []service.ActivityEvidence{{
				Provider: service.ProviderGitHub,
				Kind:     "commit",
				Start:    start,
				End:      start.Add(5 * time.Minute),
				Summary:  "Merged PR #42",
				Detail:   "Sensitive commit body",
			}},
		}},
	})
	if err := s.SaveAIConfig(ctx, server.URL+"/v1", "llama3"); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	got, err := s.SuggestGapFill(ctx, service.TimeWindow{Start: start, End: end})
	if err != nil {
		t.Fatalf("SuggestGapFill: %v", err)
	}
	if got.Category != "Deep Work" {
		t.Fatalf("category = %q want Deep Work", got.Category)
	}
	if got.EvidenceCount != 1 {
		t.Fatalf("evidence count = %d want 1", got.EvidenceCount)
	}
	if capturedUserPrompt == "" {
		t.Fatal("expected ai prompt to be captured")
	}
	if !strings.Contains(capturedUserPrompt, "Merged PR #42") {
		t.Fatal("expected evidence summary in ai prompt")
	}
	if !strings.Contains(capturedUserPrompt, "Sensitive commit body") {
		t.Fatal("local model should receive full evidence detail")
	}
}

func TestSuggestGapFillRequiresAIConfig(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()
	start := time.Date(2026, 6, 2, 13, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 2, 14, 0, 0, 0, time.UTC)

	_, err := s.SuggestGapFill(ctx, service.TimeWindow{Start: start, End: end})
	if err == nil {
		t.Fatal("expected error without ai config")
	}
}
