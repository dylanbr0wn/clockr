package catalog

import (
	"sort"

	appv1 "github.com/dylanbr0wn/shiet/gen/shiet/app/v1"
)

// Descriptor is compile-time product metadata for an integration provider.
type Descriptor struct {
	ID               string
	DisplayName      string
	Kind             appv1.IntegrationKind
	NeedsAccountHint bool
	SupportsPAT      bool
}

var entries = []Descriptor{
	{
		ID:               "google",
		DisplayName:      "Google Calendar",
		Kind:             appv1.IntegrationKind_INTEGRATION_KIND_CALENDAR_SOURCE,
		NeedsAccountHint: true,
	},
	{
		ID:          "github",
		DisplayName: "GitHub",
		Kind:        appv1.IntegrationKind_INTEGRATION_KIND_ACTIVITY_EVIDENCE,
		SupportsPAT: true,
	},
	{
		ID:          "slack",
		DisplayName: "Slack",
		Kind:        appv1.IntegrationKind_INTEGRATION_KIND_ACTIVITY_EVIDENCE,
	},
	{
		ID:          "bitbucket",
		DisplayName: "Bitbucket",
		Kind:        appv1.IntegrationKind_INTEGRATION_KIND_ACTIVITY_EVIDENCE,
	},
}

// All returns catalog descriptors in stable id order.
func All() []Descriptor {
	out := make([]Descriptor, len(entries))
	copy(out, entries)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Lookup returns a catalog descriptor by provider id.
func Lookup(id string) (Descriptor, bool) {
	for _, entry := range entries {
		if entry.ID == id {
			return entry, true
		}
	}
	return Descriptor{}, false
}

// ToProto maps a descriptor to the application protobuf shape.
func ToProto(entry Descriptor, oauthAvailable bool) *appv1.IntegrationDescriptor {
	return &appv1.IntegrationDescriptor{
		Id:          entry.ID,
		DisplayName: entry.DisplayName,
		Kind:        entry.Kind,
		Connect: &appv1.ConnectCapabilities{
			NeedsAccountHint: entry.NeedsAccountHint,
			SupportsPat:      entry.SupportsPAT,
			OauthAvailable:   oauthAvailable,
		},
	}
}
