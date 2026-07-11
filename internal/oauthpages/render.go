package oauthpages

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"sync"
)

//go:embed assets/*
var assets embed.FS

var (
	loadOnce sync.Once
	loadErr  error
	styles   string
	pages    struct {
		success *template.Template
		error   *template.Template
		close   *template.Template
	}
)

func load() error {
	loadOnce.Do(func() {
		stylesBytes, err := assets.ReadFile("assets/styles.css")
		if err != nil {
			loadErr = fmt.Errorf("read styles.css: %w", err)
			return
		}
		styles = string(stylesBytes)

		pages.success, loadErr = parsePage("success")
		if loadErr != nil {
			return
		}
		pages.error, loadErr = parsePage("error")
		if loadErr != nil {
			return
		}
		pages.close, loadErr = parsePage("close")
	})
	return loadErr
}

func parsePage(name string) (*template.Template, error) {
	raw, err := assets.ReadFile("assets/" + name + ".html")
	if err != nil {
		return nil, fmt.Errorf("read %s.html: %w", name, err)
	}
	tmpl, err := template.New(name).Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("parse %s.html: %w", name, err)
	}
	return tmpl, nil
}

func render(tmpl *template.Template, data any) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Success renders the hosted broker callback success page.
func Success(providerName, handoffURL string) (string, error) {
	if err := load(); err != nil {
		return "", err
	}
	return render(pages.success, struct {
		ProviderName string
		HandoffURL   string
		Styles       template.CSS
	}{
		ProviderName: providerName,
		HandoffURL:   handoffURL,
		Styles:       template.CSS(styles),
	})
}

// Error renders a branded broker callback error page.
func Error(message string) (string, error) {
	if err := load(); err != nil {
		return "", err
	}
	return render(pages.error, struct {
		Message string
		Styles  template.CSS
	}{
		Message: message,
		Styles:  template.CSS(styles),
	})
}

// Close renders a branded desktop loopback close-window page.
func Close(message string) (string, error) {
	if err := load(); err != nil {
		return "", err
	}
	return render(pages.close, struct {
		Message string
		Styles  template.CSS
	}{
		Message: message,
		Styles:  template.CSS(styles),
	})
}
