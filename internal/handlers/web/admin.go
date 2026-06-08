// Package web holds the shared HTTP plumbing for the admin/sub handlers: user-facing
// message mapping (lower layers return technical/sentinel errors only), node/inbound
// validation, the admin session/auth middleware, and the static HTML renderer
// (embedded shell/login pages + static assets). Each action lives in its own
// internal/handlers/<action> package; the JSON request/response shapes are owned by
// the generated ogen layer (internal/oas), not this package.
package web

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/postlog/subgen/internal/entity"
)

// ValidateNode checks a node and its inbounds, normalising the base path (mutates n).
// A node may expose any number of uniform inbounds; per-node both the name and the
// port must be unique. The node name allows ASCII letters/digits/-/space + country
// flags (e.g. "🇷🇺 RU1"); an inbound name allows ASCII letters/digits/- (e.g. "force").
func ValidateNode(n *entity.Node) error {
	if !isNodeName(n.Name) {
		return fmt.Errorf("имя узла: разрешены a-z, 0-9, -, пробел и флаги стран")
	}

	if !isHostOrIP(n.VPNHost) {
		return fmt.Errorf("адрес %q невалиден — ожидается хост или IP (без схемы и порта)", n.VPNHost)
	}

	if err := validatePanelURL(n.PanelBaseURL); err != nil {
		return err
	}

	if n.PanelBasePath == "" {
		return fmt.Errorf("укажите base path панели (например /secret/)")
	}

	if !strings.HasPrefix(n.PanelBasePath, "/") {
		n.PanelBasePath = "/" + n.PanelBasePath
	}

	if len(n.Inbounds) == 0 {
		return fmt.Errorf("укажите хотя бы один инбаунд")
	}

	seenPort := map[int]bool{}
	seenName := map[string]bool{}

	for _, in := range n.Inbounds {
		if !isInboundName(in.Name) {
			return fmt.Errorf("имя инбаунда %q: разрешены a-z, 0-9 и -", in.Name)
		}

		if in.Port < 1 || in.Port > 65535 {
			return fmt.Errorf("порт инбаунда %s должен быть числом 1–65535", in.Name)
		}

		if seenName[in.Name] {
			return fmt.Errorf("повторяющееся имя инбаунда %q", in.Name)
		}

		if seenPort[in.Port] {
			return fmt.Errorf("повторяющийся порт инбаунда %d", in.Port)
		}

		seenName[in.Name] = true
		seenPort[in.Port] = true
	}

	return nil
}

// isInboundName reports whether s is a valid inbound name: non-empty, ASCII
// letters/digits/-.
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

// isNodeName reports whether s is a valid node name: non-empty, ASCII
// letters/digits/-/space and country-flag emoji (regional indicators).
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

// isRegionalIndicator reports whether c is a regional-indicator symbol (a country
// flag is a pair of these).
func isRegionalIndicator(c rune) bool { return c >= 0x1F1E6 && c <= 0x1F1FF }

func validatePanelURL(s string) error {
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("3x-ui base URL %q невалиден — ожидается https://host:port", s)
	}

	if u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("3x-ui base URL %q должен быть только https://host:port (путь задаётся в base path)", s)
	}

	if !isHostOrIP(u.Hostname()) {
		return fmt.Errorf("3x-ui base URL: хост %q невалиден", u.Hostname())
	}

	if port := u.Port(); port != "" {
		if p, err := strconv.Atoi(port); err != nil || p < 1 || p > 65535 {
			return fmt.Errorf("3x-ui base URL: порт %q невалиден", port)
		}
	}

	return nil
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

// NodeConnChecker is the nodes-repo subset InboundsBlocking needs.
type NodeConnChecker interface {
	Get(ctx context.Context, id int64) (*entity.Node, error)
	ConnectionCountsByInbound(ctx context.Context, inboundIDs []int64) (map[int64]int, error)
}

// InboundRefChecker is the routing-repo subset InboundsBlocking needs: how many
// mihomo rules / proxy-group members point at each inbound.
type InboundRefChecker interface {
	InboundRefCounts(ctx context.Context, inboundIDs []int64) (map[int64]int, error)
}

// InboundsBlocking returns a non-empty message if any of the given node-inbound ids
// (all of the node's inbounds when inboundIDs==nil) is still referenced — by a user
// connection or by a mihomo rule / proxy-group member. Such an inbound can't be
// removed without detaching those references first (the FK would also RESTRICT it;
// this yields a friendly pre-check message).
func InboundsBlocking(ctx context.Context, nodes NodeConnChecker, routing InboundRefChecker, nodeID int64, inboundIDs []int64) (string, error) {
	n, err := nodes.Get(ctx, nodeID)
	if err != nil {
		return "", err
	}

	label := map[int64]string{}

	for _, in := range n.Inbounds {
		label[in.ID] = fmt.Sprintf("%s:%d", n.InboundLabel(in), in.Port)
	}

	ids := inboundIDs
	if ids == nil {
		ids = make([]int64, 0, len(n.Inbounds))
		for _, in := range n.Inbounds {
			ids = append(ids, in.ID)
		}
	}

	users, err := nodes.ConnectionCountsByInbound(ctx, ids)
	if err != nil {
		return "", err
	}

	refs, err := routing.InboundRefCounts(ctx, ids)
	if err != nil {
		return "", err
	}

	if len(users) == 0 && len(refs) == 0 {
		return "", nil
	}

	var parts []string

	for _, id := range ids {
		var reasons []string
		if c := users[id]; c > 0 {
			reasons = append(reasons, fmt.Sprintf("%d польз.", c))
		}

		if c := refs[id]; c > 0 {
			reasons = append(reasons, fmt.Sprintf("%d правил/групп", c))
		}

		if len(reasons) > 0 {
			parts = append(parts, label[id]+" — "+strings.Join(reasons, ", "))
		}
	}

	return "сначала отвяжите от инбаундов: " + strings.Join(parts, "; "), nil
}
