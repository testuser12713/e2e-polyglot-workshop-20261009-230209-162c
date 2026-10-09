// Package deps carries the process-wide dependencies shared by the API's
// handlers: the PostgreSQL pool, the Valkey queue client and the loaded
// configuration. They are installed once at startup so every handler can reach
// them without threading them through every signature.
package deps

import (
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"

	"workshop/internal/config"
	"workshop/internal/queue"
)

// Deps is the set of shared dependencies the API is built around.
type Deps struct {
	DB    *pgxpool.Pool
	Queue *queue.Client
	Cfg   *config.Config
}

var (
	mu      sync.RWMutex
	current *Deps
)

// Set installs the process-wide dependencies. It is called once from main
// before the server starts serving, and once per test before the router is
// built.
func Set(d *Deps) {
	mu.Lock()
	defer mu.Unlock()
	current = d
}

// Get returns the installed dependencies, or nil before Set runs.
func Get() *Deps {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// DB returns the shared PostgreSQL pool, or nil when dependencies are not
// installed yet.
func DB() *pgxpool.Pool {
	if d := Get(); d != nil {
		return d.DB
	}
	return nil
}

// Config returns the loaded start configuration, or nil when dependencies are
// not installed yet.
func Config() *config.Config {
	if d := Get(); d != nil {
		return d.Cfg
	}
	return nil
}

// QueueClient returns the shared Valkey queue client, or nil when dependencies
// are not installed yet.
func QueueClient() *queue.Client {
	if d := Get(); d != nil {
		return d.Queue
	}
	return nil
}
