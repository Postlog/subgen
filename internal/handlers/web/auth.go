// Package web holds the shared HTTP plumbing for the admin/sub handlers: the admin
// session/auth middleware and the static HTML renderer (embedded shell/login pages +
// static assets). Each action lives in its own internal/handlers/<action> package; the
// JSON request/response shapes are owned by the generated ogen layer (internal/oas), not
// this package. User-facing message text lives in each action handler as local constants.
package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	adminCookie = "subgen_admin"
	adminTTL    = 12 * time.Hour
)

// Session signs and validates the admin session cookie (HMAC of the expiry with
// the server secret) and gates admin handlers.
type Session struct {
	secret string
}

// NewSession builds a session manager over the server secret.
func NewSession(secret string) *Session { return &Session{secret: secret} }

// sign returns a signed session value `<expiryUnix>.<hmac>`.
func (s *Session) sign(expiry int64) string {
	payload := strconv.FormatInt(expiry, 10)
	mac := hmac.New(sha256.New, []byte(s.secret))
	mac.Write([]byte("admin|" + payload))

	return payload + "." + hex.EncodeToString(mac.Sum(nil))
}

func (s *Session) valid(v string) bool {
	parts := strings.SplitN(v, ".", 2)
	if len(parts) != 2 {
		return false
	}

	exp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(s.sign(exp)), []byte(v)) == 1
}

// CookieName is the admin session cookie name (the ogen security scheme reads it).
const CookieName = adminCookie

// Valid reports whether a raw cookie value is a currently-valid admin session — used by
// the ogen security handler and the page handlers, which extract the cookie value
// themselves (the session cookie is an ogen parameter, not a *http.Request concern).
func (s *Session) Valid(value string) bool { return s.valid(value) }

// IssueCookie builds (does not write) a fresh 12h admin session cookie.
func (s *Session) IssueCookie() *http.Cookie {
	exp := time.Now().Add(adminTTL).Unix()

	return &http.Cookie{
		Name: adminCookie, Value: s.sign(exp), Path: "/admin",
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		MaxAge: int(adminTTL.Seconds()),
	}
}

// ClearCookie builds (does not write) a cookie that expires the admin session.
func (s *Session) ClearCookie() *http.Cookie {
	return &http.Cookie{
		Name: adminCookie, Value: "", Path: "/admin",
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		MaxAge: -1,
	}
}
