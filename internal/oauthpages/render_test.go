package oauthpages

import (
	"strings"
	"testing"
)

func TestSuccessBrandedAndEscaped(t *testing.T) {
	body, err := Success(`Google"`, `http://127.0.0.1:9/oauth/handoff?broker_state=1&handoff_code=abc`)
	if err != nil {
		t.Fatalf("Success: %v", err)
	}
	for _, want := range []string{
		"shiet",
		"Authorization complete",
		"finish connecting Google&#34;",
		`href="http://127.0.0.1:9/oauth/handoff?broker_state=1&amp;handoff_code=abc"`,
		`content="0;url=http://127.0.0.1:9/oauth/handoff?broker_state=1&amp;handoff_code=abc"`,
		".shell",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
	if strings.Contains(body, `Google"`) {
		t.Fatal("provider name must be escaped")
	}
}

func TestErrorBrandedAndEscaped(t *testing.T) {
	body, err := Error(`Broker failed <script>alert(1)</script>. Return to shiet and retry.`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	for _, want := range []string{
		"shiet",
		"Authorization failed",
		"Broker failed &lt;script&gt;alert(1)&lt;/script&gt;",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
}

func TestCloseBrandedAndEscaped(t *testing.T) {
	body, err := Close("Authorization complete. You can close this window and return to shiet.")
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	for _, want := range []string{
		"shiet",
		"Return to shiet",
		"Authorization complete. You can close this window and return to shiet.",
		"You can close this window.",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
}
