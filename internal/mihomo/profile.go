package mihomo

// Profile holds the subscription-profile knobs of one config — how the rendered
// subscription is presented to the client: the profile title (Profile-Title header),
// the download filename (Content-Disposition) and the client's auto-update interval in
// hours (Profile-Update-Interval). ProxiesInterval is the auto-generated proxy-provider's
// refresh TTL in SECONDS — the cadence the mihomo core re-fetches the node list while the
// tunnel is up (core-level, distinct from the app-level Profile-Update-Interval). The
// values are operator-set and validated on save; there are no code defaults — an
// unconfigured config carries a zero Profile.
type Profile struct {
	Title           string
	Filename        string
	UpdateInterval  int // hours (app-level Profile-Update-Interval)
	ProxiesInterval int // seconds (core-level proxy-provider refresh TTL)
}
