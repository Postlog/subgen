package mihomo

// Profile holds the subscription-profile knobs of one config — how the rendered
// subscription is presented to the client: the profile title (Profile-Title header),
// the download filename (Content-Disposition) and the client's auto-update interval in
// hours (Profile-Update-Interval). Empty/zero fields mean "use the default"; the
// substitution is done explicitly where the value is consumed (renderer, config read),
// not hidden behind a method.
type Profile struct {
	Title          string
	Filename       string
	UpdateInterval int // hours
}

// Defaults for an unset profile field. Applied explicitly at the point of use.
const (
	DefaultProfileTitle   = "Freedom"
	DefaultFilename       = "freedom.yaml"
	DefaultUpdateInterval = 1 // hour
)
