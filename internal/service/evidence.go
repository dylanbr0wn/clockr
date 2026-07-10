package service

import (
	"context"
	"time"
)

// Activity provider identifiers stored in integration_connection.provider.
const (
	ProviderSlack     = "slack"
	ProviderGitHub    = "github"
	ProviderBitbucket = "bitbucket"
)

// TimeWindow is a half-open UTC span [Start, End) used to query activity evidence.
type TimeWindow struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// ActivityEvidence is read-only context from an activity integration (Slack,
// GitHub, etc.) that may inform gap-fill suggestions. Detail holds the full
// local text; Summary is the minimized form safe for cloud models.
type ActivityEvidence struct {
	Provider string    `json:"provider"`
	Kind     string    `json:"kind"`
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	Summary  string    `json:"summary"`
	Source   string    `json:"source,omitempty"`
	Detail   string    `json:"detail,omitempty"`
	URL      string    `json:"url,omitempty"`
}

// EvidenceProvider fetches activity evidence for a time window. Implementations
// are provider-specific (Slack, GitHub, …) and consult connected accounts
// internally.
type EvidenceProvider interface {
	Provider() string
	FetchEvidence(ctx context.Context, window TimeWindow) ([]ActivityEvidence, error)
}

// EvidenceConfig wires activity evidence providers into the service layer.
type EvidenceConfig struct {
	Providers []EvidenceProvider
}

// SetEvidence wires activity evidence providers into the service.
func (s *Service) SetEvidence(cfg EvidenceConfig) {
	s.evidence = &cfg
}
