// Package deps holds the process-wide dependencies the handlers reach through:
// the PostgreSQL pool, the Valkey queue client and the start configuration.
//
// The graph is filled once in main after the schema is applied. Set also seeds
// the configured bootstrap employee, so the workshop login works on a fresh,
// empty database without any manual step.
package deps

import (
	"context"
	"log"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"

	"workshop/internal/config"
	"workshop/internal/queue"
	"workshop/internal/store"
)

// Deps is the set of shared dependencies one process runs with.
type Deps struct {
	DB    *pgxpool.Pool
	Queue *queue.Client
	Cfg   *config.Config
}

var (
	mu      sync.RWMutex
	current *Deps
)

// Set installs d as the process-wide dependency graph and seeds the bootstrap
// employee when the start configuration names one. Seeding is idempotent, so a
// restart on an existing database is a no-op. It is called once from main
// before the server starts serving, and once per test before the router is
// built.
func Set(d *Deps) {
	mu.Lock()
	current = d
	mu.Unlock()

	if d == nil || d.DB == nil || d.Cfg == nil {
		return
	}
	if err := store.EnsureBootstrapEmployee(context.Background(), d.DB, d.Cfg.BootstrapEmail, d.Cfg.BootstrapPassword); err != nil {
		// The value itself is never logged, only the fact that seeding failed.
		log.Printf("deps: bootstrap employee could not be created: %v", err)
	}
}

// Get returns the installed dependency graph, or nil when none was set.
func Get() *Deps {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// DB returns the shared PostgreSQL pool, or nil when dependencies are not
// installed yet.
func DB() *pgxpool.Pool {
	d := Get()
	if d == nil {
		return nil
	}
	return d.DB
}

// Queue returns the shared Valkey queue client, or nil before Set ran.
func Queue() *queue.Client {
	d := Get()
	if d == nil {
		return nil
	}
	return d.Queue
}

// QueueClient returns the shared Valkey queue client, or nil before Set ran.
func QueueClient() *queue.Client {
	return Queue()
}

// Cfg returns the loaded start configuration, or nil before Set ran.
func Cfg() *config.Config {
	d := Get()
	if d == nil {
		return nil
	}
	return d.Cfg
}

// Config returns the loaded start configuration, or nil before Set ran.
func Config() *config.Config {
	return Cfg()
}
