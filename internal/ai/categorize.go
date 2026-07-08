package ai

import (
	"context"
	"encoding/json"
	"fmt"
)

// SuggestCategory asks the model to pick one category key for an event.
func SuggestCategory(
	ctx context.Context,
	client *Client,
	model string,
	categories []CategoryDefinition,
	event EventContext,
	local bool,
	privacy PrivacyFields,
) (string, error) {
	if len(categories) == 0 {
		return "", fmt.Errorf("no categories configured")
	}

	payload := event
	if !local {
		payload = minimizeEvent(event, privacy)
	}

	eventJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	categoriesJSON, err := json.Marshal(categories)
	if err != nil {
		return "", err
	}

	systemPrompt := "You categorize calendar events for a timesheet app. Reply with exactly one category key from the provided list and nothing else."
	userPrompt := fmt.Sprintf(
		"Categories: %s\nEvent: %s\nCategory key:",
		string(categoriesJSON),
		string(eventJSON),
	)

	reply, err := client.ChatCompletion(ctx, model, systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}

	key, ok := MatchCategoryKey(reply, categories)
	if !ok {
		return "", fmt.Errorf("model returned unknown category %q", reply)
	}
	return key, nil
}

func minimizeEvent(event EventContext, privacy PrivacyFields) EventContext {
	out := EventContext{}
	if privacy.Title {
		out.Title = event.Title
	}
	if privacy.Description {
		out.Description = event.Description
	}
	if privacy.Location {
		out.Location = event.Location
	}
	if privacy.Attendees {
		out.AttendeeDomains = event.AttendeeDomains
	}
	out.Duration = event.Duration
	return out
}
