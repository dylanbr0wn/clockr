package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dylanbr0wn/shiet/internal/oauthpages"
)

func main() {
	outDir := filepath.Join(os.TempDir(), "shiet-oauth-pages")
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", outDir, err)
		os.Exit(1)
	}

	pages := []struct {
		name   string
		render func() (string, error)
	}{
		{
			name: "success.html",
			render: func() (string, error) {
				return oauthpages.Success(
					"Google",
					"shiet://oauth/google/handoff?broker_state=preview-state&handoff_code=preview-code",
				)
			},
		},
		{
			name: "error.html",
			render: func() (string, error) {
				return oauthpages.Error("Google authorization failed. Return to shiet and retry.")
			},
		},
		{
			name: "close.html",
			render: func() (string, error) {
				return oauthpages.Close("Authorization complete. You can close this window and return to shiet.")
			},
		},
	}

	for _, page := range pages {
		body, err := page.render()
		if err != nil {
			fmt.Fprintf(os.Stderr, "render %s: %v\n", page.name, err)
			os.Exit(1)
		}
		path := filepath.Join(outDir, page.name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", path, err)
			os.Exit(1)
		}
		fmt.Println(path)
	}
}
