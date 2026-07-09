package main

import (
	"context"
	"database/sql"

	"github.com/dylanbr0wn/clockr/internal/config"
	"github.com/dylanbr0wn/clockr/internal/db/sqlc"
	"github.com/dylanbr0wn/clockr/internal/integration/connection"
	"github.com/dylanbr0wn/clockr/internal/integration/google"
	"github.com/dylanbr0wn/clockr/internal/integration/secrets"
	"github.com/dylanbr0wn/clockr/internal/service"
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

func wireIntegrations(conn *sql.DB, svc *service.Service, cfg config.Config) (*google.Provider, *connection.Registry) {
	registry := connection.NewRegistry(conn)
	auth := google.AuthSettingsFromConfig(cfg)
	provider := &google.Provider{
		Config:        google.OAuthConfig(auth.ClientID, auth.ClientSecret),
		AuthMode:      auth.Mode,
		BrokerBaseURL: auth.BrokerBaseURL,
		Store:         secrets.NewKeyringStore(),
		Registry:      registry,
		Queries:       sqlc.New(conn),
	}
	svc.SetCalendarSync(service.CalendarSyncConfig{
		Puller:      provider,
		Connections: connectionAdapter{reg: registry},
	})
	svc.SetEvidence(service.EvidenceConfig{
		Providers: nil, // Slack/GitHub/Bitbucket providers wired in follow-up tickets.
	})
	return provider, registry
}
