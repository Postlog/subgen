//go:build apitest

package api

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// SubProxies decodes a rendered mihomo subscription (the /sub YAML body) and returns
// its proxies as a name->uuid map. This is the over-the-wire equivalent of reaching
// into fleet.Sub(...).Proxies: it proves the subscription the client actually
// downloads carries the expected proxy nodes with the right credentials.
func SubProxies(body []byte) (map[string]string, error) {
	var doc struct {
		Proxies []struct {
			Name string `yaml:"name"`
			UUID string `yaml:"uuid"`
		} `yaml:"proxies"`
	}

	if err := yaml.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("subscription is not valid YAML: %w", err)
	}

	out := make(map[string]string, len(doc.Proxies))
	for _, p := range doc.Proxies {
		out[p.Name] = p.UUID
	}

	return out, nil
}
