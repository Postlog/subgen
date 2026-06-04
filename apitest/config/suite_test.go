//go:build apitest

// Package config_test drives subgen's mihomo-config endpoints over the real HTTP API:
// read (GET /admin/api/config/mihomo), schema (.../schema), save (.../save, with all
// its validation), and the rule-provider check probe (.../provider/check). None of
// these touch a 3x-ui panel — the config lives entirely in subgen's own store, and the
// provider-check probe is exercised against a local httptest file server started inside
// the test. So the suite boots a server WITHOUT registering nodes and does NOT gate on
// the panels being configured: it runs in plain CI.
//
// Each Test* (per endpoint/scenario) lives in its own *_test.go; corner cases are
// dotted subtests. Save-validation cases assert the EXACT Russian message per case.
package config_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/postlog/subgen/apitest/api"
)

// ConfigSuite boots a subgen server (fresh empty store, admin enabled) and drives the
// mihomo-config endpoints. It logs in once; every test runs against the same store, so
// cases scope their assertions (the round-trip is the only writer, and it runs in its
// own test).
type ConfigSuite struct {
	suite.Suite

	server *api.Server
	api    *api.Client
}

func TestConfigSuite(t *testing.T) {
	// No SkipUnlessConfigured: the config endpoints + the in-test provider server need
	// no docker panels, so this suite runs everywhere.
	suite.Run(t, new(ConfigSuite))
}

func (s *ConfigSuite) SetupSuite() {
	s.server = api.StartServer(s.T())

	s.api = api.New(s.server.BaseURL())
	res, err := s.api.Login(api.AdminUser, api.AdminPass)
	s.Require().NoError(err)
	s.Require().True(res.OK, "admin login must succeed: %s", res.Message())
}

// saveRejected POSTs a config that must be rejected and asserts {ok:false} carrying the
// exact friendly message. Returns nothing — the message is the assertion.
func (s *ConfigSuite) saveRejected(cfg api.Config, wantMsg string) {
	res, err := s.api.SaveConfig(cfg)
	s.Require().NoError(err)
	s.Require().False(res.OK, "config must be rejected (wanted %q)", wantMsg)
	s.Equal(wantMsg, res.Err)
}
