// Package sublinks builds the per-user list of copyable subscription links shown in the
// admin UI — the raw subscription URL for each engine plus the app deeplinks that embed
// it (clashmi today). The catalog of links lives here, so the admin SPA renders whatever
// the backend declares and hardcodes neither titles nor link formats; adding an engine
// or a deeplink is a one-line catalog change with no frontend edit.
package sublinks

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/token"
)

// linkSpec is one entry in the subscription-link catalog: the engine it targets, the
// display title and how to turn the user's resolved sub URL (+ the config's profile
// title) into the copyable value. Keeping the deeplink shape in build() — not as a magic
// string scattered across the code — confines escaping/format to one typed place.
type linkSpec struct {
	title string
	kind  entity.ConfigKind
	build func(subURL, profileTitle string) string
}

// catalog is the ordered set of links offered for every user. Extend it to add an engine
// or an app deeplink; the admin UI needs no change.
var catalog = []linkSpec{
	{title: "Mihomo", kind: entity.ConfigKindMihomo, build: rawURL},
	{title: "Clashmi", kind: entity.ConfigKindMihomo, build: clashmiDeeplink},
}

// rawURL is the identity build: the copyable value is the subscription URL itself.
func rawURL(subURL, _ string) string { return subURL }

// clashmiDeeplink wraps a mihomo subscription URL in clashmi's install-config deeplink.
// name is the config's profile title — what the app labels the imported profile with.
func clashmiDeeplink(subURL, profileTitle string) string {
	return "clashmi://install-config?url=" + url.QueryEscape(subURL) +
		"&name=" + url.QueryEscape(profileTitle) +
		"&overwrite=false"
}

// Service builds subscription links. It holds the HMAC secret and public base used to
// mint per-user sub URLs and resolves each user's effective profile title via the config
// repositories.
type Service struct {
	secret   string
	base     string
	configs  configResolver
	profiles profileReader
}

// New builds the service. base is the public base URL (trailing slash trimmed); secret is
// the HMAC secret behind the subscription token.
func New(secret, base string, configs configResolver, profiles profileReader) *Service {
	return &Service{
		secret:   secret,
		base:     strings.TrimRight(base, "/"),
		configs:  configs,
		profiles: profiles,
	}
}

// Links builds the ordered copyable links for each user, keyed by user id. For every
// catalog entry it composes the user's sub URL (base + /sub/<kind>/<token>) and, for
// deeplinks, the effective profile title of that engine's config. Titles are resolved
// per engine — the base once, customs only for users that have one — so a page of users
// is a handful of reads, not one per user.
func (s *Service) Links(ctx context.Context, users []entity.User) (map[int64][]entity.SubLink, error) {
	if len(users) == 0 {
		return map[int64][]entity.SubLink{}, nil
	}

	titles, err := s.titlesByUser(ctx, users)
	if err != nil {
		return nil, err
	}

	out := make(map[int64][]entity.SubLink, len(users))

	for i := range users {
		u := &users[i]

		links := make([]entity.SubLink, 0, len(catalog))
		for _, spec := range catalog {
			subURL := s.base + "/sub/" + string(spec.kind) + "/" + token.Make(s.secret, u.SubID)
			links = append(links, entity.SubLink{
				Title: spec.title,
				Value: spec.build(subURL, titles[u.ID][spec.kind]),
			})
		}

		out[u.ID] = links
	}

	return out, nil
}

// titlesByUser resolves each user's effective profile title for every engine the catalog
// references: the base title (read once per engine) unless the user has a custom config
// for that engine, whose title is read individually. Returns userID → kind → title.
func (s *Service) titlesByUser(ctx context.Context, users []entity.User) (map[int64]map[entity.ConfigKind]string, error) {
	out := make(map[int64]map[entity.ConfigKind]string, len(users))
	for i := range users {
		out[users[i].ID] = make(map[entity.ConfigKind]string)
	}

	for _, kind := range catalogKinds() {
		baseTitle, err := s.baseTitle(ctx, kind)
		if err != nil {
			return nil, err
		}

		custom, err := s.customUsers(ctx, kind)
		if err != nil {
			return nil, err
		}

		for i := range users {
			u := &users[i]

			title := baseTitle
			if _, ok := custom[u.ID]; ok {
				t, err := s.userTitle(ctx, u.ID, kind)
				if err != nil {
					return nil, err
				}

				title = t
			}

			out[u.ID][kind] = title
		}
	}

	return out, nil
}

// customUsers is the set of user ids that have a custom config for the engine.
func (s *Service) customUsers(ctx context.Context, kind entity.ConfigKind) (map[int64]struct{}, error) {
	ids, err := s.configs.UserConfigUserIDs(ctx, kind)
	if err != nil {
		return nil, fmt.Errorf("configs.UserConfigUserIDs: %w", err)
	}

	set := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}

	return set, nil
}

// baseTitle reads the engine's base-config profile title, or "" if there is no base yet.
func (s *Service) baseTitle(ctx context.Context, kind entity.ConfigKind) (string, error) {
	id, ok, err := s.configs.BaseConfigID(ctx, kind)
	if err != nil {
		return "", fmt.Errorf("configs.BaseConfigID: %w", err)
	}

	if !ok {
		return "", nil
	}

	return s.title(ctx, id)
}

// userTitle reads a user's custom-config profile title for the engine, or "" if none.
func (s *Service) userTitle(ctx context.Context, userID int64, kind entity.ConfigKind) (string, error) {
	id, ok, err := s.configs.UserConfigID(ctx, userID, kind)
	if err != nil {
		return "", fmt.Errorf("configs.UserConfigID: %w", err)
	}

	if !ok {
		return "", nil
	}

	return s.title(ctx, id)
}

// title reads a config's profile title. A config with no profile row yields "".
func (s *Service) title(ctx context.Context, configID int64) (string, error) {
	p, err := s.profiles.Profile(ctx, configID)
	if err != nil {
		return "", fmt.Errorf("profiles.Profile: %w", err)
	}

	return p.Title, nil
}

// catalogKinds returns the distinct engine kinds referenced by the catalog, preserving
// first-seen order.
func catalogKinds() []entity.ConfigKind {
	seen := make(map[entity.ConfigKind]struct{}, len(catalog))

	var out []entity.ConfigKind

	for _, spec := range catalog {
		if _, ok := seen[spec.kind]; ok {
			continue
		}

		seen[spec.kind] = struct{}{}
		out = append(out, spec.kind)
	}

	return out
}
