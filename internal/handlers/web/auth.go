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

// IsAuthed reports whether the request carries a valid admin session cookie.
func (s *Session) IsAuthed(r *http.Request) bool {
	c, err := r.Cookie(adminCookie)
	return err == nil && s.valid(c.Value)
}

// Issue sets a fresh 12h admin session cookie.
func (s *Session) Issue(w http.ResponseWriter) {
	exp := time.Now().Add(adminTTL).Unix()
	http.SetCookie(w, &http.Cookie{
		Name: adminCookie, Value: s.sign(exp), Path: "/admin",
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		MaxAge: int(adminTTL.Seconds()),
	})
}

// Clear expires the admin session cookie (same attributes as Issue, so it's
// replaced cleanly).
func (s *Session) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: adminCookie, Value: "", Path: "/admin",
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		MaxAge: -1,
	})
}

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
