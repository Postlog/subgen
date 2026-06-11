package nodes

import (
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/postlog/subgen/internal/entity"
)

// validateNode checks a node and its inbounds, normalising the base path (mutates n) and
// returning an entity.ErrValidation* sentinel on the first problem. A node may expose any
// number of uniform inbounds; per-node both name and port must be unique. The node name
// allows ASCII letters/digits/-/space + country flags ("🇷🇺 RU1"); an inbound name allows
// ASCII letters/digits/- ("force").
func validateNode(n *entity.Node) error {
	if !isNodeName(n.Name) {
		return entity.ErrValidationNodeName
	}

	if !isHostOrIP(n.VPNHost) {
		return entity.ErrValidationHost
	}

	if !validPanelURL(n.PanelBaseURL) {
		return entity.ErrValidationPanelURL
	}

	if n.PanelBasePath == "" {
		return entity.ErrValidationBasePath
	}

	if !strings.HasPrefix(n.PanelBasePath, "/") {
		n.PanelBasePath = "/" + n.PanelBasePath
	}

	if len(n.Inbounds) == 0 {
		return entity.ErrValidationNoInbounds
	}

	seenPort := map[int]bool{}
	seenName := map[string]bool{}

	for _, in := range n.Inbounds {
		switch {
		case !isInboundName(in.Name):
			return entity.ErrValidationInboundName
		case in.Port < 1 || in.Port > 65535:
			return entity.ErrValidationInboundPort
		case seenName[in.Name]:
			return entity.ErrValidationInboundNameUq
		case seenPort[in.Port]:
			return entity.ErrValidationInboundPortUq
		}

		seenName[in.Name] = true
		seenPort[in.Port] = true
	}

	return nil
}

// isInboundName reports whether s is a valid inbound name: non-empty, ASCII letters/digits/-.
func isInboundName(s string) bool {
	if s == "" {
		return false
	}

	for _, c := range s {
		if !isASCIIAlnum(c) && c != '-' {
			return false
		}
	}

	return true
}

// isNodeName reports whether s is a valid node name: non-empty, ASCII letters/digits/-/space
// and country-flag emoji (regional indicators).
func isNodeName(s string) bool {
	if s == "" {
		return false
	}

	for _, c := range s {
		if !isASCIIAlnum(c) && c != '-' && c != ' ' && !isRegionalIndicator(c) {
			return false
		}
	}

	return true
}

func isASCIIAlnum(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// isRegionalIndicator reports whether c is a regional-indicator symbol (a country flag is
// a pair of these).
func isRegionalIndicator(c rune) bool { return c >= 0x1F1E6 && c <= 0x1F1FF }

// validPanelURL reports whether s is a bare 3x-ui base URL: https?://host[:port], no
// path/query/fragment.
func validPanelURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return false
	}

	if u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}

	if !isHostOrIP(u.Hostname()) {
		return false
	}

	if port := u.Port(); port != "" {
		if p, err := strconv.Atoi(port); err != nil || p < 1 || p > 65535 {
			return false
		}
	}

	return true
}

func isHostOrIP(s string) bool {
	if s == "" {
		return false
	}

	if net.ParseIP(s) != nil {
		return true
	}

	if len(s) > 253 {
		return false
	}

	for _, label := range strings.Split(s, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}

		for _, c := range label {
			if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' {
				return false
			}
		}
	}

	return true
}
