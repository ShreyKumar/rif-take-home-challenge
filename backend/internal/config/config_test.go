package config

import (
	"os"
	"testing"
)

// unset removes an env var for the duration of the test, restoring it on
// cleanup. t.Setenv registers the restore; the following Unsetenv makes the
// variable genuinely absent so defaults are exercised regardless of the host
// environment.
func unset(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		t.Setenv(k, "")
		if err := os.Unsetenv(k); err != nil {
			t.Fatalf("unset %s: %v", k, err)
		}
	}
}

func TestLoadDefaults(t *testing.T) {
	unset(t, "PORT", "DB_PATH", "FRONTEND_DIR")

	got := Load()
	want := Config{Port: "8080", DBPath: "mutant.db", FrontendDir: "../frontend"}
	if got != want {
		t.Fatalf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DB_PATH", "/data/custom.db")
	t.Setenv("FRONTEND_DIR", "/srv/frontend")

	got := Load()
	want := Config{Port: "9090", DBPath: "/data/custom.db", FrontendDir: "/srv/frontend"}
	if got != want {
		t.Fatalf("Load() = %+v, want %+v", got, want)
	}
}
