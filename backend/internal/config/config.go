// Package config reads the API's start configuration from the environment.
//
// Values are read when Load runs (startup), never at import time, so a missing
// variable produces a readable error that names it instead of a panic before
// anything can be logged. Placeholders that the runner could not resolve (an
// unresolved "${service:...}" literal, e.g. when the web app is not declared on
// this branch yet) are treated as unset and fall back to the dev default.
package config

import (
	"errors"
	"os"
	"strings"
)

// Config carries every value the API needs to boot. Secrets are only ever held
// in memory and are never logged.
type Config struct {
	DatabaseURL string
	ValkeyURL   string
	WebOrigin   string
	APIPort     string

	BootstrapEmail    string
	BootstrapPassword string
}

// DefaultWebOrigin is the Vite dev server default. The runner overrides it with
// the origin the web service really ended up on.
const DefaultWebOrigin = "http://localhost:5173"

// DefaultAPIPort is used when API_PORT is not set (for a fresh clone).
const DefaultAPIPort = "8080"

// Load reads the start configuration and validates the values the API cannot
// boot without. Every error names the variable and points at RUN.json.
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:       required("DATABASE_URL"),
		ValkeyURL:         required("VALKEY_URL"),
		WebOrigin:         optional("WEB_ORIGIN", DefaultWebOrigin),
		APIPort:           optional("API_PORT", DefaultAPIPort),
		BootstrapEmail:    optional("EMPLOYEE_BOOTSTRAP_EMAIL", ""),
		BootstrapPassword: optional("EMPLOYEE_BOOTSTRAP_PASSWORD", ""),
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.ValkeyURL == "" {
		missing = append(missing, "VALKEY_URL")
	}
	if len(missing) > 0 {
		return nil, errors.New("missing required configuration: " + strings.Join(missing, ", ") + " (see RUN.json)")
	}
	return cfg, nil
}

// required returns the environment value or "" when unset or still an
// unresolved runner placeholder.
func required(key string) string {
	return optional(key, "")
}

// optional returns the environment value, falling back to def when the variable
// is empty or an unresolved placeholder ("${...}").
func optional(key, def string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" || strings.HasPrefix(value, "${") {
		return def
	}
	return value
}
