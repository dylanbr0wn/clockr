package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	brokerconfig "github.com/dylanbr0wn/shiet/internal/broker/config"
	"github.com/dylanbr0wn/shiet/internal/broker/httpapi"
	"github.com/dylanbr0wn/shiet/internal/broker/observe"
	"github.com/dylanbr0wn/shiet/internal/broker/ratelimit"
	"github.com/dylanbr0wn/shiet/internal/broker/store"
	applog "github.com/dylanbr0wn/shiet/internal/log"
)

func main() {
	cfg, err := brokerconfig.LoadFromEnv()
	if err != nil {
		log.Fatalf("op=broker.config.load reason=%s", applog.Reason(err))
	}

	ctx := context.Background()
	datastore, err := store.Open(ctx, cfg.DatastoreDSN)
	if err != nil {
		log.Fatalf("op=broker.datastore.open reason=%s", applog.Reason(err))
	}
	defer datastore.Close()

	logger := observe.NewLogger(os.Stdout)
	metrics := observe.NewMetrics()
	limiter := ratelimit.New(time.Minute, nil)

	srv := &http.Server{
		Addr: cfg.ListenAddr,
		Handler: httpapi.Server{
			Config:  cfg,
			Store:   datastore,
			Limiter: limiter,
			Metrics: metrics,
			Logger:  logger,
		}.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("shiet oauth broker listening on %s", cfg.ListenAddr)
		errCh <- srv.ListenAndServe()
	}()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-stopCh:
		log.Printf("shutting down after %s", sig)
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("op=broker.listen reason=%s", applog.Reason(err))
		}
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("op=broker.shutdown reason=%s", applog.Reason(err))
	}
}
