package ai

import "time"

// Endpoint is a known or user-supplied OpenAI-compatible API base URL.
type Endpoint struct {
	Name    string   `json:"name"`
	BaseURL string   `json:"baseUrl"`
	Local   bool     `json:"local"`
	Running bool     `json:"running"`
	Models  []string `json:"models,omitempty"`
}

// ValidationResult summarizes a connectivity + privacy check.
type ValidationResult struct {
	OK      bool   `json:"ok"`
	Local   bool   `json:"local"`
	Verdict string `json:"verdict"`
	Message string `json:"message"`
}

// EventContext is the event payload sent to a categorization model.
type EventContext struct {
	Title           string   `json:"title"`
	Description     string   `json:"description,omitempty"`
	Location        string   `json:"location,omitempty"`
	Organizer       string   `json:"organizer,omitempty"`
	AttendeeDomains []string `json:"attendeeDomains,omitempty"`
	Duration        string   `json:"duration,omitempty"`
}

// PrivacyFields controls which event fields may be sent to cloud models.
type PrivacyFields struct {
	Title       bool `json:"title"`
	Attendees   bool `json:"attendees"`
	Description bool `json:"description"`
	Location    bool `json:"location"`
}

// GapContext describes an uncovered interval sent to a gap-fill model.
type GapContext struct {
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	Duration string    `json:"duration"`
}

// EvidencePayload is minimized activity evidence for gap-fill prompts.
// Cloud models receive summaries only; local models also get detail, URLs,
// and timestamps.
type EvidencePayload struct {
	Provider string `json:"provider,omitempty"`
	Kind     string `json:"kind"`
	Summary  string `json:"summary"`
	Source   string `json:"source,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Start    string `json:"start,omitempty"`
	End      string `json:"end,omitempty"`
	URL      string `json:"url,omitempty"`
}
