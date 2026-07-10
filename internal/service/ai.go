package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dylanbr0wn/shiet/internal/ai"
)

const (
	settingAIBaseURL   = "ai.base_url"
	settingAIModel     = "ai.model"
	settingAIMaxTokens = "ai.max_tokens"
	settingPrivacy     = "privacy.fields"
)

// DiscoverLocalAIEndpoints probes known local model runtimes.
func (s *Service) DiscoverLocalAIEndpoints(ctx context.Context) []ai.Endpoint {
	return ai.DiscoverLocalEndpoints(ctx)
}

// ClassifyAIEndpoint returns whether a base URL is local and the privacy verdict.
func (s *Service) ClassifyAIEndpoint(baseURL string) (bool, string) {
	return ai.ClassifyEndpoint(baseURL)
}

// ListAIModels fetches model ids from an OpenAI-compatible endpoint.
func (s *Service) ListAIModels(ctx context.Context, baseURL, apiKey string) ([]string, error) {
	client := ai.NewClient(baseURL, apiKey)
	models, err := client.ListModels(ctx)
	if err != nil {
		return nil, mapErr("list ai models", err)
	}
	return models, nil
}

// ValidateAIConfig checks connectivity and returns the privacy verdict.
func (s *Service) ValidateAIConfig(ctx context.Context, baseURL, apiKey, model string) (ai.ValidationResult, error) {
	local, verdict := ai.ClassifyEndpoint(baseURL)
	result := ai.ValidationResult{
		Local:   local,
		Verdict: verdict,
	}

	if strings.TrimSpace(baseURL) == "" {
		result.Message = "Base URL is required"
		return result, nil
	}
	if strings.TrimSpace(model) == "" {
		result.Message = "Model is required"
		return result, nil
	}

	client := ai.NewClient(baseURL, apiKey)
	if err := client.Validate(ctx, model); err != nil {
		result.Message = err.Error()
		return result, mapErr("validate ai config", err)
	}

	result.OK = true
	if local {
		result.Message = "Connected. Full event context stays on your device."
	} else {
		result.Message = "Connected. Cloud models receive minimized event data."
	}
	return result, nil
}

// SaveAIEndpoint persists the selected OpenAI-compatible base URL.
func (s *Service) SaveAIEndpoint(ctx context.Context, baseURL string) error {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return fmt.Errorf("save ai endpoint: base URL is required")
	}
	return s.SetSetting(ctx, settingAIBaseURL, jsonString(baseURL))
}

// SaveAIModel persists the selected model name.
func (s *Service) SaveAIModel(ctx context.Context, model string) error {
	model = strings.TrimSpace(model)
	if model == "" {
		return fmt.Errorf("save ai model: model is required")
	}
	return s.SetSetting(ctx, settingAIModel, jsonString(model))
}

// SaveAIConfig persists the selected endpoint and model.
func (s *Service) SaveAIConfig(ctx context.Context, baseURL, model string) error {
	baseURL = strings.TrimSpace(baseURL)
	model = strings.TrimSpace(model)
	if baseURL == "" || model == "" {
		return fmt.Errorf("save ai config: base URL and model are required")
	}
	if err := s.SaveAIEndpoint(ctx, baseURL); err != nil {
		return err
	}
	return s.SaveAIModel(ctx, model)
}

func (s *Service) loadAIConfig(ctx context.Context) (baseURL, model string, ok bool) {
	baseURL, err := s.readStringSetting(ctx, settingAIBaseURL)
	if err != nil || baseURL == "" {
		return "", "", false
	}
	model, err = s.readStringSetting(ctx, settingAIModel)
	if err != nil || model == "" {
		return "", "", false
	}
	return baseURL, model, true
}

func (s *Service) loadAIMaxTokens(ctx context.Context) int {
	raw, err := s.GetSetting(ctx, settingAIMaxTokens)
	if err != nil {
		return ai.DefaultMaxTokens
	}
	var value int
	if err := json.Unmarshal([]byte(raw), &value); err != nil || value <= 0 {
		return ai.DefaultMaxTokens
	}
	return value
}

func (s *Service) readStringSetting(ctx context.Context, key string) (string, error) {
	raw, err := s.GetSetting(ctx, key)
	if err != nil {
		return "", err
	}
	var value string
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func (s *Service) loadPrivacyFields(ctx context.Context) (ai.PrivacyFields, error) {
	raw, err := s.GetSetting(ctx, settingPrivacy)
	if err != nil {
		return ai.PrivacyFields{Title: true, Attendees: true}, err
	}
	var fields ai.PrivacyFields
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return ai.PrivacyFields{Title: true, Attendees: true}, err
	}
	return fields, nil
}

func jsonString(value string) string {
	b, _ := json.Marshal(value)
	return string(b)
}

func buildEventContext(inc IncomingEvent) ai.EventContext {
	ctx := ai.EventContext{
		Title:       inc.Title,
		Description: inc.Description,
		Location:    inc.Location,
		Organizer:   inc.Organizer,
	}
	if inc.Start != nil && inc.End != nil {
		d := inc.End.Sub(*inc.Start).Round(time.Minute)
		ctx.Duration = d.String()
	}
	domains := make([]string, 0, len(inc.Attendees))
	seen := make(map[string]struct{})
	for _, attendee := range inc.Attendees {
		email := strings.ToLower(strings.TrimSpace(attendee.Email))
		if email == "" || !strings.Contains(email, "@") {
			continue
		}
		domain := strings.TrimPrefix(email[strings.LastIndex(email, "@"):], "@")
		if domain == "" {
			continue
		}
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		domains = append(domains, domain)
	}
	ctx.AttendeeDomains = domains
	return ctx
}
