package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/caarlos0/env/v11"
)

// Load builds the bootstrap config from the process environment. If envFile is
// non-empty and the file exists, its KEY=VALUE lines are loaded first (without
// overriding variables already set in the environment), so a local `.env` next to
// the binary and a systemd/Docker EnvironmentFile both work. The struct is then
// parsed from the environment via its `env` tags and validated.
//
// Recognised variables (all optional except SUBGEN_SECRET):
//
//	SUBGEN_SECRET                  HMAC key for sub tokens + admin sessions (required)
//	SUBGEN_LISTEN                  listen address           (default 0.0.0.0:2097)
//	SUBGEN_TLS_CERT, SUBGEN_TLS_KEY  TLS cert/key paths; both empty => plain HTTP
//	SUBGEN_PUBLIC_BASE             external base URL, e.g. https://host:2097/
//	SUBGEN_DB_PATH                 SQLite path              (default db/subgen.db)
//	SUBGEN_STATIC_DIR              serve admin assets from this on-disk dir (live,
//	                               no rebuild); empty => embedded copy (default)
//	SUBGEN_ADMIN_USER              admin login              (default admin)
//	SUBGEN_ADMIN_PASSWORD          admin password; empty => admin panel disabled
//	SUBGEN_PROFILE_TITLE           subscription profile title (default Freedom)
//	SUBGEN_FILENAME                subscription filename      (default freedom.yaml)
//	SUBGEN_CACHE_TTL               fleet cache TTL            (default 5m)
//	SUBGEN_PROFILE_UPDATE_INTERVAL client refresh hint, hours (default 24)
func Load(envFile string) (Config, error) {
	if envFile != "" {
		if err := loadDotenv(envFile); err != nil {
			return Config{}, err
		}
	}

	var c Config
	if err := env.Parse(&c); err != nil {
		return Config{}, fmt.Errorf("parse env: %w", err)
	}

	if err := validate(c); err != nil {
		return Config{}, err
	}

	return c, nil
}

func validate(c Config) error {
	if c.Listen == "" {
		return fmt.Errorf("SUBGEN_LISTEN is required")
	}

	if c.Secret == "" {
		return fmt.Errorf("SUBGEN_SECRET is required (openssl rand -hex 32)")
	}

	if (c.TLSCert == "") != (c.TLSKey == "") {
		return fmt.Errorf("set both SUBGEN_TLS_CERT and SUBGEN_TLS_KEY, or neither (plain HTTP)")
	}

	return nil
}

// loadDotenv parses a minimal .env file (KEY=VALUE per line, # comments, optional
// surrounding quotes) and sets any variable not already present in the environment.
func loadDotenv(path string) error {
	f, err := os.Open(path) //nolint:gosec // path is the operator-supplied -env flag
	if err != nil {
		if os.IsNotExist(err) {
			return nil // a missing .env is fine; rely on the real environment
		}

		return fmt.Errorf("read %s: %w", path, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		line = strings.TrimPrefix(line, "export ")

		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)

		val = strings.TrimSpace(val)
		if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[len(val)-1] == val[0] {
			val = val[1 : len(val)-1]
		}

		if _, set := os.LookupEnv(key); !set {
			_ = os.Setenv(key, val)
		}
	}

	return sc.Err()
}
