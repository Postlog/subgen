package nodes

import (
	"database/sql"
	"encoding/json"

	"github.com/postlog/subgen/internal/entity"
)

// hysteria2SettingsDTO is the persistence shape of a hysteria2 inbound's creds — the JSON
// stored in node_inbounds.settings. It lives here, at the storage boundary, so the entity
// (entity.Hysteria2Settings) carries no serialization tags.
type hysteria2SettingsDTO struct {
	Password     string `json:"password"`
	Obfs         string `json:"obfs,omitempty"`
	ObfsPassword string `json:"obfs_password,omitempty"`
	SNI          string `json:"sni,omitempty"`
	Up           string `json:"up,omitempty"`
	Down         string `json:"down,omitempty"`
}

// inboundKindSettings maps an inbound to its stored columns: its `kind` (passed through —
// callers always set an explicit kind) and the NULLABLE `settings` blob (NULL for vless, the
// hysteria2 creds JSON otherwise).
func inboundKindSettings(in entity.Inbound) (kind string, settings sql.NullString) {
	kind = in.Kind

	if in.Kind == entity.InboundKindHysteria2 && in.Hysteria2 != nil {
		b, _ := json.Marshal(hysteria2SettingsDTO{
			Password: in.Hysteria2.Password, Obfs: in.Hysteria2.Obfs, ObfsPassword: in.Hysteria2.ObfsPassword,
			SNI: in.Hysteria2.SNI, Up: in.Hysteria2.Up, Down: in.Hysteria2.Down,
		})
		settings = sql.NullString{String: string(b), Valid: true}
	}

	return kind, settings
}

// applyKindSettings decodes a stored (kind, settings) pair onto an inbound.
func applyKindSettings(in *entity.Inbound, kind string, settings sql.NullString) {
	in.Kind = kind

	if kind == entity.InboundKindHysteria2 && settings.Valid && settings.String != "" {
		var dto hysteria2SettingsDTO
		if json.Unmarshal([]byte(settings.String), &dto) == nil {
			in.Hysteria2 = &entity.Hysteria2Settings{
				Password: dto.Password, Obfs: dto.Obfs, ObfsPassword: dto.ObfsPassword,
				SNI: dto.SNI, Up: dto.Up, Down: dto.Down,
			}
		}
	}
}
