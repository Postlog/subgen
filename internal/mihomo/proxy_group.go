package mihomo

// ProxyGroupType is a mihomo proxy-group policy.
type ProxyGroupType string

const (
	GroupSelect      ProxyGroupType = "select"       // manual switcher (shown in client)
	GroupURLTest     ProxyGroupType = "url-test"     // auto-pick lowest latency
	GroupFallback    ProxyGroupType = "fallback"     // first available
	GroupLoadBalance ProxyGroupType = "load-balance" // distribute
	GroupRelay       ProxyGroupType = "relay"        // chain in order
)

// ProxyGroupTypeOptions are a group type's admin-schema options: whether it takes
// url/interval health-check options, and whether it takes the url-test tolerance.
type ProxyGroupTypeOptions struct {
	UsesHealthCheck bool
	UsesTolerance   bool
}

// proxyGroupTypes is the known group-type registry (single source for validity,
// options and the admin schema).
var proxyGroupTypes = map[ProxyGroupType]ProxyGroupTypeOptions{
	GroupSelect:      {},
	GroupURLTest:     {UsesHealthCheck: true, UsesTolerance: true},
	GroupFallback:    {UsesHealthCheck: true},
	GroupLoadBalance: {UsesHealthCheck: true},
	GroupRelay:       {},
}

// ProxyGroupTypeCatalog returns the group-type options map (the admin-schema source).
func ProxyGroupTypeCatalog() map[ProxyGroupType]ProxyGroupTypeOptions { return proxyGroupTypes }

// Valid reports whether g is a known group type.
func (g ProxyGroupType) Valid() bool { _, ok := proxyGroupTypes[g]; return ok }

// UsesHealthCheck reports whether the type takes url/interval health-check options
// (url-test / fallback / load-balance).
func (g ProxyGroupType) UsesHealthCheck() bool { return proxyGroupTypes[g].UsesHealthCheck }

// String returns the wire value.
func (g ProxyGroupType) String() string { return string(g) }

// ProxyGroup is an operator-defined mihomo proxy-group. Its members are typed
// PolicyRefs resolved per-subscriber at render — the connection selector and any
// routing groups are ordinary ProxyGroup rows.
type ProxyGroup struct {
	ID        int64
	Position  int
	Name      string
	Type      ProxyGroupType
	URL       string // health-check url (url-test/fallback/load-balance)
	Interval  int    // health-check interval, seconds
	Tolerance int    // url-test tolerance, ms
	Lazy      bool
	Members   []PolicyRef
}
