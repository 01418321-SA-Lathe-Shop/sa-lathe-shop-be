package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionTokenNeedsGuardHeaderForCookie(t *testing.T) {
	r := httptest.NewRequest("GET", "/auth/me", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookie, Value: "cookie-token"})
	if got := sessionToken(r); got != "" {
		t.Fatalf("cookie without %s header must be ignored, got %q", CookieHeader, got)
	}
	r.Header.Set(CookieHeader, "1")
	if got := sessionToken(r); got != "cookie-token" {
		t.Fatalf("guarded cookie token = %q", got)
	}
	r.Header.Set("Authorization", "Bearer bearer-token")
	if got := sessionToken(r); got != "bearer-token" {
		t.Fatalf("bearer must win, got %q", got)
	}
}

func TestSessionCookieAttributesAndScopedClear(t *testing.T) {
	w := httptest.NewRecorder()
	setSessionCookie(w, "abc", time.Now().Add(time.Hour))
	c := w.Result().Cookies()[0]
	if c.Name != SessionCookie || c.Value != "abc" || !c.HttpOnly || c.Path != "/" || c.MaxAge <= 0 {
		t.Fatalf("unexpected cookie %+v", c)
	}
	r := httptest.NewRequest("POST", "/auth/logout", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookie, Value: "new"})
	w = httptest.NewRecorder()
	clearIfCurrent(w, r, "old")
	if len(w.Result().Cookies()) != 0 {
		t.Fatal("revoking another session must keep the current cookie")
	}
	w = httptest.NewRecorder()
	clearIfCurrent(w, r, "new")
	if cs := w.Result().Cookies(); len(cs) != 1 || cs[0].MaxAge >= 0 {
		t.Fatalf("revoking the cookie session must clear it, got %+v", cs)
	}
}
