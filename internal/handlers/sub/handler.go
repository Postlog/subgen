// Package sub handles GET /sub/{token} — the per-client mihomo subscription.
package sub

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo/render"
	"github.com/postlog/subgen/internal/token"
)

// Handler renders a subscriber's profile from the fleet snapshot. It depends on
// concrete bootstrap values (not a config object) plus the users/fleet/routing
// readers.
type Handler struct {
	users   subIDLister
	fleet   fleetReader
	routing mihomoReader

	secret         string
	publicBase     string
	profileTitle   string
	filename       string
	updateInterval int
}

// New builds the handler.
func New(users subIDLister, fleet fleetReader, routing mihomoReader, secret, publicBase, profileTitle, filename string, updateInterval int) *Handler {
	return &Handler{
		users: users, fleet: fleet, routing: routing,
		secret: secret, publicBase: publicBase,
		profileTitle: profileTitle, filename: filename, updateInterval: updateInterval,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tok := mux.Vars(r)["token"]
	if tok == "" {
		http.NotFound(w, r)
		return
	}

	// Resolve the token against service-owned users only — clients created
	// directly on a panel are not served.
	subIDs, err := h.users.SubIDs(r.Context())
	if err != nil {
		slog.Error("handler sub: list sub ids failed", "err", err)
		http.Error(w, "store unavailable", http.StatusInternalServerError)

		return
	}

	var subID string

	for _, id := range subIDs {
		if token.Match(h.secret, id, tok) {
			subID = id
			break
		}
	}

	if subID == "" {
		http.NotFound(w, r)
		return
	}

	fleet, err := h.fleet.Fleet(r.Context())
	if err != nil {
		slog.Error("handler sub: fleet fetch failed", "err", err)
		http.Error(w, "upstream unavailable", http.StatusBadGateway)

		return
	}

	sub := fleet.Sub(subID)
	if sub == nil {
		sub = &entity.Subscriber{SubID: subID} // provisioned clients missing; serve an empty profile
	}

	opts, err := h.renderOptions(r.Context())
	if err != nil {
		slog.Error("handler sub: load mihomo config failed", "err", err)
		http.Error(w, "config unavailable", http.StatusInternalServerError)

		return
	}

	body, err := render.Render(sub, opts)
	if err != nil {
		slog.Error("handler sub: render failed", "err", err)
		http.Error(w, "render error", http.StatusInternalServerError)

		return
	}

	title := h.profileTitle
	if title == "" {
		title = "Freedom"
	}

	filename := h.filename
	if filename == "" {
		filename = "freedom.yaml"
	}

	hdr := w.Header()
	hdr.Set("Content-Type", "text/yaml; charset=utf-8")
	hdr.Set("Profile-Update-Interval", fmt.Sprintf("%d", h.updateInterval))
	hdr.Set("Profile-Title", "base64:"+base64.StdEncoding.EncodeToString([]byte(title)))
	hdr.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	hdr.Set("Subscription-Userinfo", userinfo(sub.Up, sub.Down, sub.Total, sub.Expiry))

	_, _ = w.Write(body)
}

// renderOptions assembles the operator config render needs from the store.
func (h *Handler) renderOptions(ctx context.Context) (render.Options, error) {
	rules, err := h.routing.Rules(ctx)
	if err != nil {
		return render.Options{}, err
	}

	groups, err := h.routing.ProxyGroups(ctx)
	if err != nil {
		return render.Options{}, err
	}

	provs, err := h.routing.RuleProviders(ctx)
	if err != nil {
		return render.Options{}, err
	}

	base, err := h.routing.Setting(ctx, "base_yaml")
	if err != nil {
		return render.Options{}, err
	}

	return render.Options{
		BaseYAML:   base,
		Rules:      rules,
		Groups:     groups,
		Providers:  provs,
		PublicBase: h.publicBase,
	}, nil
}

// userinfo formats the Subscription-Userinfo header. Expiry is ms epoch; we emit
// seconds. expire is omitted when 0 (no expiry) so clients don't render "1970".
func userinfo(up, down, total, expiryMs int64) string {
	s := fmt.Sprintf("upload=%d; download=%d; total=%d", up, down, total)
	if expiryMs > 0 {
		s += fmt.Sprintf("; expire=%d", expiryMs/1000)
	}

	return s
}
