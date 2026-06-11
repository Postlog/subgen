package ruleset

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"github.com/postlog/subgen/internal/entity"
)

// Checker probes a rule-provider URL without persisting anything. It is stateless
// (one HTTP client per process); the URL/format are call arguments.
type Checker struct {
	hc *http.Client
}

// NewChecker builds a Checker with a short timeout (a probe shouldn't hang the UI).
func NewChecker() *Checker {
	return &Checker{hc: &http.Client{
		Timeout:   15 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}},
	}}
}

// Check fetches url and reports whether it is reachable, the file is present, and the
// content matches the declared format (mrs / yaml / text). A URL that can't be probed —
// unreachable, malformed, or blank (they're the same category: nothing to talk to) — is an
// outcome (RulesetCheckUnreachable), not a Go error: the call never returns one.
func (c *Checker) Check(ctx context.Context, url, format string) entity.RulesetCheckResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return entity.RulesetCheckResult{Outcome: entity.RulesetCheckUnreachable, Detail: err.Error()}
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return entity.RulesetCheckResult{Outcome: entity.RulesetCheckUnreachable, Detail: netDetail(err)}
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return entity.RulesetCheckResult{Outcome: entity.RulesetCheckHTTPError, Status: resp.StatusCode}
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return entity.RulesetCheckResult{Outcome: entity.RulesetCheckUnreachable, Detail: err.Error()}
	}

	if len(data) == 0 {
		return entity.RulesetCheckResult{Outcome: entity.RulesetCheckEmpty}
	}

	if !matchesFormat(data, format) {
		return entity.RulesetCheckResult{Outcome: entity.RulesetCheckFormatMismatch, Size: len(data)}
	}

	return entity.RulesetCheckResult{Outcome: entity.RulesetCheckOK, Status: resp.StatusCode, Size: len(data)}
}

// netDetail trims a verbose net/url error down to its innermost message.
func netDetail(err error) string {
	msg := err.Error()
	if i := strings.LastIndex(msg, ": "); i >= 0 && i+2 < len(msg) {
		return msg[i+2:]
	}

	return msg
}

// matchesFormat reports whether data looks like the declared rule-provider format.
func matchesFormat(data []byte, format string) bool {
	switch strings.ToLower(format) {
	case "mrs":
		return isZstd(data) // .mrs is a zstd container (MRSv1 magic lives inside it)
	case "yaml":
		return isRuleProviderYAML(data)
	default: // text
		return isRuleText(data)
	}
}

// zstdMagic is the zstd frame magic; every mihomo .mrs file is zstd-compressed.
var zstdMagic = []byte{0x28, 0xB5, 0x2F, 0xFD}

func isZstd(b []byte) bool { return len(b) >= 4 && bytes.Equal(b[:4], zstdMagic) }

// isRuleProviderYAML checks the body parses as YAML with a `payload` list — the shape
// every mihomo yaml rule-provider has.
func isRuleProviderYAML(b []byte) bool {
	var m map[string]any
	if err := yaml.Unmarshal(b, &m); err != nil {
		return false
	}

	payload, ok := m["payload"]
	if !ok {
		return false
	}

	_, isList := payload.([]any)

	return isList
}

// isRuleText checks the body is non-empty UTF-8 text (not HTML/binary) with at least
// one rule line.
func isRuleText(b []byte) bool {
	if !utf8.Valid(b) {
		return false
	}

	s := strings.TrimSpace(string(b))
	if s == "" || strings.HasPrefix(s, "<") { // empty or HTML/XML error page
		return false
	}

	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			return true
		}
	}

	return false
}
