package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoad exercises Load straight from the environment. It must NOT run in parallel
// (neither the table nor the subtests): each case uses t.Setenv, and the Go test
// runtime panics if t.Setenv is called from a test that has called t.Parallel.
//
// Note on isolation: caarlos0/env applies the `envDefault` when a variable is set to an
// empty string, so clearing an unwanted var with t.Setenv("X","") makes X take its
// default — except SUBGEN_SECRET / SUBGEN_TLS_* which have no default, so "" stays empty.
func TestLoad(t *testing.T) {
	tt := []struct {
		name string

		env map[string]string

		want    Config
		wantErr bool // validate() returns plain fmt.Errorf, no sentinel to ErrorIs against
	}{
		{
			name: "error.missing_secret",
			env: map[string]string{
				"SUBGEN_SECRET":   "",
				"SUBGEN_TLS_CERT": "",
				"SUBGEN_TLS_KEY":  "",
			},
			wantErr: true,
		},
		{
			name: "error.tls_cert_without_key",
			env: map[string]string{
				"SUBGEN_SECRET":   "s3cr3t",
				"SUBGEN_TLS_CERT": "/etc/cert.pem",
				"SUBGEN_TLS_KEY":  "",
			},
			wantErr: true,
		},
		{
			name: "error.tls_key_without_cert",
			env: map[string]string{
				"SUBGEN_SECRET":         "s3cr3t",
				"SUBGEN_ADMIN_PASSWORD": "pw",
				"SUBGEN_TLS_CERT":       "",
				"SUBGEN_TLS_KEY":        "/etc/key.pem",
			},
			wantErr: true,
		},
		{
			name: "error.missing_admin_password",
			env: map[string]string{
				"SUBGEN_SECRET":         "s3cr3t",
				"SUBGEN_ADMIN_PASSWORD": "",
			},
			wantErr: true,
		},
		{
			name: "success.defaults",
			env: map[string]string{
				"SUBGEN_SECRET":         "s3cr3t",
				"SUBGEN_ADMIN_PASSWORD": "pw",
				"SUBGEN_TLS_CERT":       "",
				"SUBGEN_TLS_KEY":        "",
			},
			want: Config{
				DBPath:                "db/subgen.db",
				StaticDir:             "",
				Listen:                "0.0.0.0:2097",
				Secret:                "s3cr3t",
				ProfileTitle:          "Freedom",
				Filename:              "freedom.yaml",
				ProfileUpdateInterval: 24,
				AdminUser:             "admin",
				AdminPassword:         "pw",
			},
		},
		{
			name: "success.static_dir_and_overrides",
			env: map[string]string{
				"SUBGEN_SECRET":         "s3cr3t",
				"SUBGEN_ADMIN_PASSWORD": "pw",
				"SUBGEN_STATIC_DIR":     "/srv/static",
				"SUBGEN_TLS_CERT":       "/etc/cert.pem",
				"SUBGEN_TLS_KEY":        "/etc/key.pem",
				"SUBGEN_LISTEN":         "127.0.0.1:9999",
			},
			want: Config{
				DBPath:                "db/subgen.db",
				StaticDir:             "/srv/static",
				Listen:                "127.0.0.1:9999",
				TLSCert:               "/etc/cert.pem",
				TLSKey:                "/etc/key.pem",
				Secret:                "s3cr3t",
				ProfileTitle:          "Freedom",
				Filename:              "freedom.yaml",
				ProfileUpdateInterval: 24,
				AdminUser:             "admin",
				AdminPassword:         "pw",
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Clear every variable Load reads, then set the case's overrides — keeps the
			// parse hermetic regardless of the operator's ambient environment.
			for _, k := range allKeys {
				t.Setenv(k, "")
			}

			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			got, err := Load("")

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// allKeys is every SUBGEN_* variable Load consults; the test clears them all per case.
var allKeys = []string{
	"SUBGEN_DB_PATH",
	"SUBGEN_STATIC_DIR",
	"SUBGEN_LISTEN",
	"SUBGEN_TLS_CERT",
	"SUBGEN_TLS_KEY",
	"SUBGEN_PUBLIC_BASE",
	"SUBGEN_SECRET",
	"SUBGEN_PROFILE_TITLE",
	"SUBGEN_FILENAME",
	"SUBGEN_PROFILE_UPDATE_INTERVAL",
	"SUBGEN_ADMIN_USER",
	"SUBGEN_ADMIN_PASSWORD",
}
