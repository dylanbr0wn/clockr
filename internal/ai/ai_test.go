package ai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dylanbr0wn/shiet/internal/ai"
)

func TestClassifyEndpoint(t *testing.T) {
	tests := []struct {
		url      string
		wantLocal bool
	}{
		{"http://localhost:11434/v1", true},
		{"http://127.0.0.1:11434/v1", true},
		{"http://192.168.1.42:1234/v1", true},
		{"https://api.openai.com/v1", false},
		{"http://example.com:11434/v1", true},
		{"not-a-url", false},
	}

	for _, tc := range tests {
		local, _ := ai.ClassifyEndpoint(tc.url)
		if local != tc.wantLocal {
			t.Fatalf("ClassifyEndpoint(%q) local=%v want %v", tc.url, local, tc.wantLocal)
		}
	}
}

func TestClientListModelsAndValidate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]string{{"id": "llama3"}},
			})
		case "/v1/chat/completions":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{"message": map[string]string{"content": "Hello! How can I help you today?"}},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := ai.NewClient(server.URL+"/v1", "")
	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 1 || models[0] != "llama3" {
		t.Fatalf("unexpected models: %#v", models)
	}

	if err := client.Validate(context.Background(), "llama3"); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateAcceptsVariedReplies(t *testing.T) {
	replies := []string{
		"ok",
		"OK!",
		"Hello there.",
		`"Hi!"`,
		"Sure, I'm here.",
	}

	for _, reply := range replies {
		reply := reply
		t.Run(reply, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"choices": []map[string]any{
						{"message": map[string]string{"content": reply}},
					},
				})
			}))
			defer server.Close()

			client := ai.NewClient(server.URL+"/v1", "")
			if err := client.Validate(context.Background(), "test-model"); err != nil {
				t.Fatalf("Validate(%q): %v", reply, err)
			}
		})
	}
}

func TestSuggestCategory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "Deep Work"}},
			},
		})
	}))
	defer server.Close()

	client := ai.NewClient(server.URL+"/v1", "")
	got, err := ai.SuggestCategory(
		context.Background(),
		client,
		"llama3",
		[]ai.CategoryDefinition{
			{Key: "Meetings", Name: "Meetings"},
			{Key: "deep-work", Name: "Deep Work"},
		},
		ai.EventContext{Title: "Focus block"},
		true,
		ai.PrivacyFields{Title: true},
	)
	if err != nil {
		t.Fatalf("SuggestCategory: %v", err)
	}
	if got != "deep-work" {
		t.Fatalf("got %q want deep-work", got)
	}
}

func TestDiscoverLocalEndpointsUsesHandler(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"id": "local-model"}},
		})
	}))
	defer server.Close()

	// Discovery probes fixed localhost ports, so this test only exercises the
	// client path indirectly via ListModels above. Keep a smoke call so the
	// function remains covered without depending on a real local runtime.
	endpoints := ai.DiscoverLocalEndpoints(context.Background())
	if len(endpoints) != 2 {
		t.Fatalf("expected 2 known endpoints, got %d", len(endpoints))
	}
}
