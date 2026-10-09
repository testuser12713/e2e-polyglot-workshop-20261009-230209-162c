// Command api is the Kfz-Werkstatt customer portal API.
//
// It reads its start configuration from the environment, applies the schema
// idempotently, wires the shared dependencies (DB, Queue, Cfg) and serves every
// contracted route.
package main

import (
	"context"
	_ "embed"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"workshop/internal/config"
	"workshop/internal/deps"
	"workshop/internal/queue"
	"workshop/internal/server"
	"workshop/internal/store"
)

//go:embed migrations/0001_init.sql
var initSQL string

func main() {
	if err := run(); err != nil {
		log.Fatalf("api: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := store.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := store.ApplyMigrations(ctx, pool, initSQL); err != nil {
		return err
	}

	q, err := queue.New(ctx, cfg.ValkeyURL)
	if err != nil {
		return err
	}
	defer func() { _ = q.Close() }()

	deps.Set(&deps.Deps{DB: pool, Queue: q, Cfg: cfg})

	srv := &http.Server{
		Addr:              ":" + cfg.APIPort,
		Handler:           server.New(deps.Get()),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("api: listening on :%s (web origin %s)", cfg.APIPort, cfg.WebOrigin)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
