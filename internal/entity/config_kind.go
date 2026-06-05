package entity

// ConfigKind is the subscription-config engine discriminator: which client format a
// config targets. Code branches on these constants, never on a raw string. Today
// only mihomo exists; xray/sing-box are future kinds sharing the same ownership
// anchor (subscription_configs.kind) — base/custom configs are independent per kind.
type ConfigKind string

const (
	ConfigKindMihomo ConfigKind = "mihomo"
)

// Valid reports whether k is a known config kind.
func (k ConfigKind) Valid() bool {
	switch k {
	case ConfigKindMihomo:
		return true
	default:
		return false
	}
}

// String returns the wire/storage form of the kind.
func (k ConfigKind) String() string { return string(k) }
