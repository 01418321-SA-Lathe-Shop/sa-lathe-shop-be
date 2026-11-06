package auth

import (
	"crypto/subtle"
	"net/http"
	"time"
)

// SessionCookie carries the same opaque session token as the Bearer header so
// a browser can restore its session after reload without exposing the token to
// JavaScript storage. It is HttpOnly and accepted only with CookieHeader, a
// custom header that forces a CORS preflight and so blocks cross-site forgery.
const (
	SessionCookie = "sa_session"
	CookieHeader  = "X-Session-Cookie"
)

// CookiePolicy is set once at startup from AUTH_COOKIE_SAMESITE/AUTH_COOKIE_SECURE.
var CookiePolicy = struct {
	SameSite http.SameSite
	Secure   bool
}{SameSite: http.SameSiteLaxMode}

func setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: token, Path: "/", Expires: expires, MaxAge: int(time.Until(expires).Seconds()),
		HttpOnly: true, Secure: CookiePolicy.Secure, SameSite: CookiePolicy.SameSite})
}
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: CookiePolicy.Secure, SameSite: CookiePolicy.SameSite})
}
func cookieToken(r *http.Request) string {
	if r.Header.Get(CookieHeader) != "1" {
		return ""
	}
	c, err := r.Cookie(SessionCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

// sessionToken prefers an explicit Bearer token, then the guarded cookie.
func sessionToken(r *http.Request) string {
	if token := bearer(r); token != "" {
		return token
	}
	return cookieToken(r)
}

// clearIfCurrent clears the browser cookie only when it holds the revoked token,
// so revoking an older session during an account switch keeps the new cookie.
func clearIfCurrent(w http.ResponseWriter, r *http.Request, revoked string) {
	if c, err := r.Cookie(SessionCookie); err == nil && subtle.ConstantTimeCompare([]byte(c.Value), []byte(revoked)) == 1 {
		clearSessionCookie(w)
	}
}
