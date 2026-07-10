package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
	"github.com/dylanbr0wn/shiet/internal/service"
)

func TestSync_AppliesAISuggestion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "Meetings"}},
			},
		})
	}))
	defer server.Close()

	e := newSyncEnv(t)
	ctx := context.Background()

	if err := e.svc.SaveAIConfig(ctx, server.URL+"/v1", "llama3"); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	event := e.baseEvent()
	event.Title = "Quarterly planning"
	res, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{event})
	if err != nil {
		t.Fatalf("SyncEvents: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("expected 1 added, got %+v", res)
	}

	cats, err := e.svc.ListCategories(ctx)
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	meetingsID := e.catID
	for _, cat := range cats {
		if cat.Name == "Meetings" {
			meetingsID = cat.ID
			break
		}
	}

	o, err := e.q.GetOverlay(ctx, sqlc.GetOverlayParams{
		PeriodID:   e.periodID,
		Provider:   event.Provider,
		ExternalID: event.ExternalID,
		InstanceID: "",
		Kind:       "category",
	})
	if err != nil {
		t.Fatalf("expected ai overlay: %v", err)
	}
	if !o.CategoryID.Valid || o.CategoryID.Int64 != meetingsID {
		t.Fatalf("overlay category mismatch: %+v want %d", o, meetingsID)
	}
}

func TestSaveAIEndpointPersistsIndependently(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	if err := e.svc.SaveAIEndpoint(ctx, "http://127.0.0.1:1234/v1"); err != nil {
		t.Fatalf("SaveAIEndpoint: %v", err)
	}

	baseURL, err := e.svc.GetSetting(ctx, "ai.base_url")
	if err != nil {
		t.Fatalf("GetSetting base url: %v", err)
	}
	if baseURL != `"http://127.0.0.1:1234/v1"` {
		t.Fatalf("unexpected ai.base_url: %q", baseURL)
	}
}

func TestSaveAIConfigPersistsSettings(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	if err := e.svc.SaveAIConfig(ctx, "http://127.0.0.1:1234/v1", "local-model"); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	baseURL, err := e.svc.GetSetting(ctx, "ai.base_url")
	if err != nil {
		t.Fatalf("GetSetting base url: %v", err)
	}
	if baseURL != `"http://127.0.0.1:1234/v1"` {
		t.Fatalf("unexpected ai.base_url: %q", baseURL)
	}

	model, err := e.svc.GetSetting(ctx, "ai.model")
	if err != nil {
		t.Fatalf("GetSetting model: %v", err)
	}
	if model != `"local-model"` {
		t.Fatalf("unexpected ai.model: %q", model)
	}
}

func TestValidateAIConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "ok"}},
			},
		})
	}))
	defer server.Close()

	e := newSyncEnv(t)
	ctx := context.Background()

	result, err := e.svc.ValidateAIConfig(ctx, server.URL+"/v1", "", "llama3")
	if err != nil {
		t.Fatalf("ValidateAIConfig: %v", err)
	}
	if !result.OK {
		t.Fatalf("expected ok result, got %+v", result)
	}
}

func TestSync_UsesConfiguredMaxTokens(t *testing.T) {
	var gotMaxTokens int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			MaxTokens int `json:"max_tokens"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotMaxTokens = body.MaxTokens
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "Meetings"}},
			},
		})
	}))
	defer server.Close()

	e := newSyncEnv(t)
	ctx := context.Background()

	if err := e.svc.SaveAIConfig(ctx, server.URL+"/v1", "llama3"); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}
	if err := e.svc.SetSetting(ctx, "ai.max_tokens", "2048"); err != nil {
		t.Fatalf("SetSetting max_tokens: %v", err)
	}

	event := e.baseEvent()
	event.Title = "Quarterly planning"
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{event}); err != nil {
		t.Fatalf("SyncEvents: %v", err)
	}
	if gotMaxTokens != 2048 {
		t.Fatalf("max_tokens = %d want 2048", gotMaxTokens)
	}
}
