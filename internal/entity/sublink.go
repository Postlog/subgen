package entity

// SubLink is one copyable subscription link shown in the admin UI: a display title and
// the literal string to copy. Value is either a raw subscription URL or an app deeplink
// that embeds it (clashmi today). The catalog of which links exist — their titles and
// formats — lives in the sublinks service, so the SPA renders whatever the backend
// declares and hardcodes neither titles nor link formats.
type SubLink struct {
	Title string
	Value string
}
