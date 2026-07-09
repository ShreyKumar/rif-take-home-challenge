// Package config loads server configuration from the environment with
// sensible 12-factor defaults.
package config

import "os"

// Config holds the runtime configuration for the server.
type Config struct {
	// Port is the TCP port the HTTP server listens on.
	Port string
	// DBPath is the path to the SQLite database file.
	DBPath string
	// FrontendDir is the directory of static assets served at /.
	FrontendDir string
}

// getenv returns the value of the environment variable key, or def when the
// variable is not set.
func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

// Load reads configuration from the environment, applying defaults for any
// variable that is not set.
func Load() Config {
	return Config{
		Port:        getenv("PORT", "8080"),
		DBPath:      getenv("DB_PATH", "mutant.db"),
		FrontendDir: getenv("FRONTEND_DIR", "../frontend"),
	}
}
