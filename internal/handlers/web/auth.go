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

// IsAuthed reports whether the request carries a valid admin session cookie.
func (s *Session) IsAuthed(r *http.Request) bool {
	c, err := r.Cookie(adminCookie)
	return err == nil && s.valid(c.Value)
}

// Valid reports whether a raw cookie value is a currently-valid admin session — used
// by the ogen security handler, which extracts the cookie value itself.
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

// Issue sets a fresh 12h admin session cookie.
func (s *Session) Issue(w http.ResponseWriter) { http.SetCookie(w, s.IssueCookie()) }

// Clear expires the admin session cookie.
func (s *Session) Clear(w http.ResponseWriter) { http.SetCookie(w, s.ClearCookie()) }

// RequireAdmin gates a handler behind a valid admin session, redirecting to the
// login page otherwise.
func (s *Session) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.IsAuthed(r) {
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}

		next(w, r)
	}
}
