package sub

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo/render"
)

// MihomoRenderer is the mihomo (Clash.Meta) engineRenderer: it loads the config's
// mihomo content (scoped by config id) and renders the per-subscriber YAML profile.
type MihomoRenderer struct {
	routing    routingRepo
	publicBase string
}

// NewMihomoRenderer builds the mihomo renderer. publicBase rewrites mirrored
// rule-provider URLs; the response metadata (filename, title, update interval) is read
// per-config from the store at render time.
func NewMihomoRenderer(routing routingRepo, publicBase string) *MihomoRenderer {
	return &MihomoRenderer{routing: routing, publicBase: publicBase}
}

// Kind reports the engine this renderer serves.
func (m *MihomoRenderer) Kind() entity.ConfigKind { return entity.ConfigKindMihomo }

// Render builds the mihomo YAML for one subscriber against the given config and reads
// the config's profile knobs (filename, title, update interval) for the response meta.
// The knobs are served as stored — there are no code defaults; an unconfigured config
// yields empty header values.
func (m *MihomoRenderer) Render(ctx context.Context, sub *entity.Subscriber, configID int64) ([]byte, RenderMeta, error) {
	opts, err := m.options(ctx, configID)
	if err != nil {
		return nil, RenderMeta{}, err
	}

	body, err := render.Render(sub, opts)
	if err != nil {
		return nil, RenderMeta{}, err
	}

	profile, err := m.routing.Profile(ctx, configID)
	if err != nil {
		return nil, RenderMeta{}, err
	}

	meta := RenderMeta{
		ContentType:    "text/yaml; charset=utf-8",
		Filename:       profile.Filename,
		ProfileTitle:   profile.Title,
		UpdateInterval: profile.UpdateInterval,
	}

	return body, meta, nil
}

// options assembles the config's mihomo content render needs from the store.
func (m *MihomoRenderer) options(ctx context.Context, configID int64) (render.Options, error) {
	rules, err := m.routing.Rules(ctx, configID)
	if err != nil {
		return render.Options{}, err
	}

	groups, err := m.routing.ProxyGroups(ctx, configID)
	if err != nil {
		return render.Options{}, err
	}

	provs, err := m.routing.RuleProviders(ctx, configID)
	if err != nil {
		return render.Options{}, err
	}

	base, err := m.routing.Setting(ctx, configID, "base_yaml")
	if err != nil {
		return render.Options{}, err
	}

	return render.Options{
		BaseYAML:   base,
		Rules:      rules,
		Groups:     groups,
		Providers:  provs,
		PublicBase: m.publicBase,
	}, nil
}
