package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultTimeout = 30 * time.Second
	// DefaultMaxTokens is the completion budget when none is configured.
	// Local reasoning models often spend most of this on thinking tokens.
	DefaultMaxTokens = 512
)

// Client talks to an OpenAI-compatible HTTP API.
type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

// NewClient builds a client for the given base URL and optional API key.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		APIKey:  strings.TrimSpace(apiKey),
		HTTP:    &http.Client{Timeout: defaultTimeout},
	}
}

type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// ListModels fetches model ids from GET /models.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req)

	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, fmt.Errorf("list models: %s: %s", res.Status, strings.TrimSpace(string(body)))
	}

	var payload modelsResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}

	out := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		if item.ID != "" {
			out = append(out, item.ID)
		}
	}
	return out, nil
}

type chatRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Validate sends a tiny chat completion to confirm the endpoint and model work.
// Any non-empty assistant reply counts as success — local models vary widely in
// tone, formatting, and verbosity.
func (c *Client) Validate(ctx context.Context, model string) error {
	body, err := json.Marshal(chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "user", Content: "Hi"},
		},
		MaxTokens: 32,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.applyHeaders(req)

	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("validate model: %s: %s", res.Status, strings.TrimSpace(string(payload)))
	}

	var payload chatResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return err
	}
	if len(payload.Choices) == 0 {
		return fmt.Errorf("validate model: empty response")
	}
	if !validationReplyOK(payload.Choices[0].Message.Content) {
		return fmt.Errorf("validate model: empty response")
	}
	return nil
}

func validationReplyOK(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false
	}
	// Strip common wrappers some chat templates add around short replies.
	trimmed = strings.Trim(trimmed, `"'`)
	trimmed = strings.TrimSpace(trimmed)
	return trimmed != ""
}

// ChatCompletion sends a chat request and returns the assistant text.
// maxTokens caps completion (and, on some local runtimes, thinking) tokens;
// values <= 0 fall back to DefaultMaxTokens.
func (c *Client) ChatCompletion(ctx context.Context, model, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}
	body, err := json.Marshal(chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens: maxTokens,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	c.applyHeaders(req)

	res, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return "", fmt.Errorf("chat completion: %s: %s", res.Status, strings.TrimSpace(string(payload)))
	}

	var payload chatResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return "", err
	}
	if len(payload.Choices) == 0 {
		return "", fmt.Errorf("chat completion: empty response")
	}
	return strings.TrimSpace(payload.Choices[0].Message.Content), nil
}

func (c *Client) applyHeaders(req *http.Request) {
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
}
