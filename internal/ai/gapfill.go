package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// SuggestGapFill asks the model to pick a category key and write a short timesheet
// description for an uncovered interval, optionally citing activity evidence.
func SuggestGapFill(
	ctx context.Context,
	client *Client,
	model string,
	categories []CategoryDefinition,
	gap GapContext,
	evidence []EvidencePayload,
	local bool,
	maxTokens int,
) (categoryKey string, description string, err error) {
	if len(categories) == 0 {
		return "", "", fmt.Errorf("no categories configured")
	}

	payload := evidence
	if !local {
		payload = minimizeEvidence(evidence)
	}

	gapJSON, err := json.Marshal(gap)
	if err != nil {
		return "", "", err
	}
	evidenceJSON, err := json.Marshal(payload)
	if err != nil {
		return "", "", err
	}
	categoriesJSON, err := json.Marshal(categories)
	if err != nil {
		return "", "", err
	}

	systemPrompt := "You suggest timesheet entries for uncovered work intervals. When activity evidence is clearly tied to a project or repository, and a category in the list corresponds to that project (key, name, or description), prefer that category. Do not invent categories; pick from the provided list only. Reply with JSON only: {\"key\":\"<one category key from the list>\",\"description\":\"<short note>\"}."
	userPrompt := fmt.Sprintf(
		"Categories: %s\nGap: %s\nActivity evidence: %s\nJSON:",
		string(categoriesJSON),
		string(gapJSON),
		string(evidenceJSON),
	)

	reply, err := client.ChatCompletion(ctx, model, systemPrompt, userPrompt, maxTokens)
	if err != nil {
		return "", "", err
	}

	var parsed struct {
		Key         string `json:"key"`
		Category    string `json:"category"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(extractJSONObject(reply)), &parsed); err != nil {
		return "", "", fmt.Errorf("parse gap-fill reply: %w", err)
	}

	candidate := strings.Trim(parsed.Key, `"' `)
	if candidate == "" {
		candidate = strings.Trim(parsed.Category, `"' `)
	}
	description = strings.TrimSpace(parsed.Description)
	if candidate == "" {
		return "", "", fmt.Errorf("model returned empty category key")
	}

	key, ok := MatchCategoryKey(candidate, categories)
	if !ok {
		return "", "", fmt.Errorf("model returned unknown category %q", candidate)
	}

	return key, description, nil
}

func minimizeEvidence(items []EvidencePayload) []EvidencePayload {
	out := make([]EvidencePayload, 0, len(items))
	for _, item := range items {
		out = append(out, EvidencePayload{
			Provider: item.Provider,
			Kind:     item.Kind,
			Summary:  item.Summary,
			Source:   item.Source,
		})
	}
	return out
}

func extractJSONObject(reply string) string {
	reply = strings.TrimSpace(reply)
	if strings.HasPrefix(reply, "```") {
		reply = strings.TrimPrefix(reply, "```json")
		reply = strings.TrimPrefix(reply, "```")
		reply = strings.TrimSuffix(reply, "```")
		reply = strings.TrimSpace(reply)
	}
	start := strings.Index(reply, "{")
	end := strings.LastIndex(reply, "}")
	if start >= 0 && end > start {
		return reply[start : end+1]
	}
	return reply
}
