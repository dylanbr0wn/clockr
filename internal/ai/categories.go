package ai

import "strings"

// CategoryDefinition is the structured category payload sent to models.
type CategoryDefinition struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// MatchCategoryKey resolves a model reply to a known category key.
// It accepts an exact key match first, then falls back to name matching
// for backward compatibility during transition.
func MatchCategoryKey(reply string, categories []CategoryDefinition) (string, bool) {
	reply = strings.Trim(reply, `"' `)
	if reply == "" {
		return "", false
	}

	for _, category := range categories {
		if strings.EqualFold(reply, category.Key) {
			return category.Key, true
		}
	}

	lowerReply := strings.ToLower(reply)
	for _, category := range categories {
		if strings.Contains(lowerReply, strings.ToLower(category.Key)) {
			return category.Key, true
		}
	}

	for _, category := range categories {
		if strings.EqualFold(reply, category.Name) {
			return category.Key, true
		}
	}

	lowerReply = strings.ToLower(reply)
	for _, category := range categories {
		if strings.Contains(lowerReply, strings.ToLower(category.Name)) {
			return category.Key, true
		}
	}

	return "", false
}
