// Package deps carries the process-wide dependencies shared by the API's
// handlers: the PostgreSQL pool, the Valkey queue client and the loaded
// configuration. They are installed once at startup so every handler can reach
// them without threading them through every signature.
package deps

import (
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

var current *Deps

// Set installs the process-wide dependencies. It is called once from main
// before the server starts serving.
func Set(d *Deps) {
	current = d
}

// Get returns the installed dependencies, or nil before Set runs.
func Get() *Deps {
	return current
}

// DB returns the shared PostgreSQL pool, or nil when dependencies are not
// installed yet.
func DB() *pgxpool.Pool {
	if current == nil {
		return nil
	}
	return current.DB
}

// Config returns the loaded start configuration, or nil when dependencies are
// not installed yet.
func Config() *config.Config {
	if current == nil {
		return nil
	}
	return current.Cfg
}

// QueueClient returns the shared Valkey queue client, or nil when dependencies
// are not installed yet.
func QueueClient() *queue.Client {
	if current == nil {
		return nil
	}
	return current.Queue
}
