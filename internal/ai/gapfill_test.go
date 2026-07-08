package ai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dylanbr0wn/clockr/internal/ai"
)

func TestSuggestGapFill(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{
					"content": `{"category":"Deep Work","description":"Reviewed PR #42 and merged fixes"}`,
				}},
			},
		})
	}))
	defer server.Close()

	client := ai.NewClient(server.URL+"/v1", "")
	gap := ai.GapContext{
		Start:    time.Date(2026, 6, 2, 13, 0, 0, 0, time.UTC),
		End:      time.Date(2026, 6, 2, 14, 0, 0, 0, time.UTC),
		Duration: "1h0m0s",
	}
	evidence := []ai.EvidencePayload{{
		Provider: "github",
		Kind:     "commit",
		Summary:  "Merged PR #42",
		Detail:   "Full commit message with sensitive details",
		URL:      "https://github.com/org/repo/pull/42",
	}}

	category, description, err := ai.SuggestGapFill(
		context.Background(),
		client,
		"llama3",
		[]ai.CategoryDefinition{
			{Key: "Meetings", Name: "Meetings"},
			{Key: "deep-work", Name: "Deep Work"},
		},
		gap,
		evidence,
		true,
	)
	if err != nil {
		t.Fatalf("SuggestGapFill: %v", err)
	}
	if category != "deep-work" {
		t.Fatalf("category = %q want deep-work", category)
	}
	if description == "" {
		t.Fatal("expected non-empty description")
	}
}

func TestSuggestGapFillCloudOmitsDetailAndURL(t *testing.T) {
	var capturedUserPrompt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, msg := range body.Messages {
			if strings.Contains(msg.Content, "Activity evidence:") {
				capturedUserPrompt = msg.Content
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{
					"content": `{"category":"Deep Work","description":"Worked on PR"}`,
				}},
			},
		})
	}))
	defer server.Close()

	client := ai.NewClient(server.URL+"/v1", "")
	evidence := []ai.EvidencePayload{{
		Provider: "slack",
		Kind:     "message",
		Summary:  "Discussed deployment",
		Detail:   "Sensitive channel transcript",
		URL:      "https://slack.com/archives/C123/p456",
		Start:    "2026-06-02T13:00:00Z",
		End:      "2026-06-02T13:05:00Z",
	}}

	_, _, err := ai.SuggestGapFill(
		context.Background(),
		client,
		"llama3",
		[]ai.CategoryDefinition{{Key: "deep-work", Name: "Deep Work"}},
		ai.GapContext{Duration: "1h0m0s"},
		evidence,
		false,
	)
	if err != nil {
		t.Fatalf("SuggestGapFill: %v", err)
	}
	if strings.Contains(capturedUserPrompt, "Sensitive channel transcript") {
		t.Fatal("cloud prompt leaked evidence detail")
	}
	if strings.Contains(capturedUserPrompt, "slack.com") {
		t.Fatal("cloud prompt leaked evidence url")
	}
	if !strings.Contains(capturedUserPrompt, "Discussed deployment") {
		t.Fatal("cloud prompt should include evidence summary")
	}
}
