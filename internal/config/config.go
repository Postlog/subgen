// Package config holds subgen's bootstrap configuration, read entirely from the
// environment (optionally seeded from a local .env file) via struct tags. It
// carries NO operational data — nodes, rules, rule-providers, the connection
// selector and the base YAML live in the SQLite store and are read by the service
// layer. Config does not flow through the layers: main reads it and passes the
// concrete primitive fields each component needs.
package config

// Config is subgen's resolved bootstrap configuration.
type Config struct {
	DBPath                string `env:"SUBGEN_DB_PATH" envDefault:"db/subgen.db"`
	StaticDir             string `env:"SUBGEN_STATIC_DIR"`
	Listen                string `env:"SUBGEN_LISTEN" envDefault:"0.0.0.0:2097"`
	TLSCert               string `env:"SUBGEN_TLS_CERT"`
	TLSKey                string `env:"SUBGEN_TLS_KEY"`
	PublicBase            string `env:"SUBGEN_PUBLIC_BASE"`
	Secret                string `env:"SUBGEN_SECRET"`
	ProfileTitle          string `env:"SUBGEN_PROFILE_TITLE" envDefault:"Freedom"`
	Filename              string `env:"SUBGEN_FILENAME" envDefault:"freedom.yaml"`
	ProfileUpdateInterval int    `env:"SUBGEN_PROFILE_UPDATE_INTERVAL" envDefault:"24"`
	AdminUser             string `env:"SUBGEN_ADMIN_USER" envDefault:"admin"`
	AdminPassword         string `env:"SUBGEN_ADMIN_PASSWORD"`
}

// TLSEnabled reports whether HTTPS should be served (both cert and key set).
func (c Config) TLSEnabled() bool { return c.TLSCert != "" && c.TLSKey != "" }

// AdminEnabled reports whether the web admin panel should be mounted.
func (c Config) AdminEnabled() bool { return c.AdminUser != "" && c.AdminPassword != "" }
