//go:build apitest

package api

// ConfigRef is a PolicyRef on the wire: a built-in kind carries nothing extra; an
// inbound ref carries the (real) inbound id; a group ref carries the INDEX of the
// referenced group in the groups array (ids never leave the backend).
type ConfigRef struct {
	Kind      string `json:"kind"`
	InboundID *int64 `json:"inboundId,omitempty"`
	GroupIdx  *int   `json:"groupIdx,omitempty"`
}

// ConfigRule is one routing rule for read/save.
type ConfigRule struct {
	Type      string    `json:"type"`
	Value     string    `json:"value"`
	NoResolve bool      `json:"noResolve"`
	Target    ConfigRef `json:"target"`
}

// ConfigGroup is one proxy-group for read/save.
type ConfigGroup struct {
	Name      string      `json:"name"`
	Type      string      `json:"type"`
	URL       string      `json:"url"`
	Interval  int         `json:"interval"`
	Tolerance int         `json:"tolerance"`
	Lazy      bool        `json:"lazy"`
	Members   []ConfigRef `json:"members"`
}

// ConfigProvider is one rule-provider for read/save.
type ConfigProvider struct {
	Name           string `json:"name"`
	Behavior       string `json:"behavior"`
	Format         string `json:"format"`
	URL            string `json:"url"`
	Interval       int    `json:"interval"`
	Mirror         bool   `json:"mirror"`
	MirrorInterval int    `json:"mirrorInterval"`
}

// Config is the whole mihomo routing config (read via ReadConfig, posted via
// SaveConfig).
type Config struct {
	Rules     []ConfigRule     `json:"rules"`
	Groups    []ConfigGroup    `json:"groups"`
	Providers []ConfigProvider `json:"providers"`
	BaseYAML  string           `json:"baseYAML"`
}

// ReadConfig GETs /admin/api/config/mihomo.
func (c *Client) ReadConfig() (Config, error) {
	var cfg Config

	if err := c.getJSON("/admin/api/config/mihomo", &cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// SaveConfig POSTs /admin/api/config/mihomo/save with the whole config document and
// decodes the {ok,…} envelope.
func (c *Client) SaveConfig(cfg Config) (Result, error) {
	return c.post("/admin/api/config/mihomo/save", cfg)
}

// SaveConfigRaw POSTs an arbitrary JSON body to the save endpoint (for malformed-JSON
// and invalid-base-YAML cases the typed Config can't express). Returns the {ok,…}
// Result.
func (c *Client) SaveConfigRaw(body []byte) (Result, error) {
	resp, err := c.PostRaw("/admin/api/config/mihomo/save", "application/json", body)
	if err != nil {
		return Result{}, err
	}

	return decodeResult(resp.Body)
}

// Schema is the static mihomo-config UI taxonomy (GET .../schema), decoded as a
// generic map so callers can assert on well-known keys and their ordering.
type Schema map[string]any

// Schema GETs /admin/api/config/mihomo/schema.
func (c *Client) Schema() (Schema, error) {
	var schema Schema

	if err := c.getJSON("/admin/api/config/mihomo/schema", &schema); err != nil {
		return nil, err
	}

	return schema, nil
}

// CheckProvider POSTs /admin/api/config/mihomo/provider/check (a read-only reachability
// probe of a rule-provider URL). Returns the {ok,msg|err} Result.
func (c *Client) CheckProvider(url, format string) (Result, error) {
	return c.post("/admin/api/config/mihomo/provider/check", map[string]string{"url": url, "format": format})
}

// CheckProviderRaw POSTs an arbitrary body to the provider-check endpoint (for the
// malformed-JSON case) and returns the {ok,…} Result.
func (c *Client) CheckProviderRaw(body []byte) (Result, error) {
	resp, err := c.PostRaw("/admin/api/config/mihomo/provider/check", "application/json", body)
	if err != nil {
		return Result{}, err
	}

	return decodeResult(resp.Body)
}
