package catalog

import (
	"testing"

	appv1 "github.com/dylanbr0wn/shiet/gen/shiet/app/v1"
)

func TestAllReturnsStableCatalog(t *testing.T) {
	t.Parallel()
	items := All()
	if len(items) != 4 {
		t.Fatalf("len = %d", len(items))
	}
	if items[0].ID != "bitbucket" || items[1].ID != "github" || items[2].ID != "google" || items[3].ID != "slack" {
		t.Fatalf("order: %#v", items)
	}
}

func TestLookupKnownProviders(t *testing.T) {
	t.Parallel()
	github, ok := Lookup("github")
	if !ok || !github.SupportsPAT || github.Kind != appv1.IntegrationKind_INTEGRATION_KIND_ACTIVITY_EVIDENCE {
		t.Fatalf("github: %#v ok=%v", github, ok)
	}
	google, ok := Lookup("google")
	if !ok || !google.NeedsAccountHint || google.Kind != appv1.IntegrationKind_INTEGRATION_KIND_CALENDAR_SOURCE {
		t.Fatalf("google: %#v ok=%v", google, ok)
	}
	bitbucket, ok := Lookup("bitbucket")
	if !ok || bitbucket.SupportsPAT || bitbucket.Kind != appv1.IntegrationKind_INTEGRATION_KIND_ACTIVITY_EVIDENCE {
		t.Fatalf("bitbucket: %#v ok=%v", bitbucket, ok)
	}
}

func TestToProtoIncludesRuntimeOAuthAvailability(t *testing.T) {
	t.Parallel()
	entry, _ := Lookup("github")
	proto := ToProto(entry, false)
	if proto.Connect == nil || proto.Connect.OauthAvailable {
		t.Fatalf("connect: %#v", proto.Connect)
	}
	proto = ToProto(entry, true)
	if proto.Connect == nil || !proto.Connect.OauthAvailable || !proto.Connect.SupportsPat {
		t.Fatalf("connect: %#v", proto.Connect)
	}
}
