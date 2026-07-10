package main

import (
	"context"
	"database/sql"

	"github.com/dylanbr0wn/shiet/internal/config"
	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
	"github.com/dylanbr0wn/shiet/internal/integration/connection"
	"github.com/dylanbr0wn/shiet/internal/integration/github"
	"github.com/dylanbr0wn/shiet/internal/integration/google"
	"github.com/dylanbr0wn/shiet/internal/integration/secrets"
	"github.com/dylanbr0wn/shiet/internal/service"
)

type connectionAdapter struct {
	reg *connection.Registry
}

func (a connectionAdapter) ListByProvider(ctx context.Context, provider string) ([]service.IntegrationAccount, error) {
	rows, err := a.reg.ListByProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	out := make([]service.IntegrationAccount, len(rows))
	for i, row := range rows {
		out[i] = service.IntegrationAccount{
			Provider:  row.Provider,
			AccountID: row.AccountID,
			Status:    row.Status,
		}
	}
	return out, nil
}

func wireIntegrations(conn *sql.DB, svc *service.Service, cfg config.Config) (*google.Provider, *github.Provider, *connection.Registry) {
	registry := connection.NewRegistry(conn)
	store := secrets.NewKeyringStore()
	queries := sqlc.New(conn)

	auth := google.AuthSettingsFromConfig(cfg)
	githubAuth := github.AuthSettingsFromConfig(cfg)
	googleProvider := &google.Provider{
		Config:        google.OAuthConfig(auth.ClientID, auth.ClientSecret),
		AuthMode:      auth.Mode,
		BrokerBaseURL: auth.BrokerBaseURL,
		Store:         store,
		Registry:      registry,
		Queries:       queries,
	}
	githubProvider := &github.Provider{
		Config:        github.OAuthConfig(githubAuth.ClientID, githubAuth.ClientSecret),
		Store:         store,
		Registry:      registry,
		Queries:       queries,
		AuthMode:      githubAuth.Mode,
		BrokerBaseURL: githubAuth.BrokerBaseURL,
	}

	svc.SetCalendarSync(service.CalendarSyncConfig{
		Puller:      googleProvider,
		Connections: connectionAdapter{reg: registry},
	})
	svc.SetEvidence(service.EvidenceConfig{
		Providers: []service.EvidenceProvider{githubProvider},
	})
	return googleProvider, githubProvider, registry
}
