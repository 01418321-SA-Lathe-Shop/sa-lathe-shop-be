package auth

import (
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	"net"
	"net/http"
	"sync"
	"time"
)

type loginWindow struct {
	until time.Time
	count int
}

// Per-process abuse protection, not a cache of stock, status or sessions.
// Trust only socket peer IP; proxy headers must be configured before deployment.
type LoginLimiter struct {
	mu      sync.Mutex
	clients map[string]loginWindow
	now     func() time.Time
}

func NewLoginLimiter() *LoginLimiter {
	return &LoginLimiter{clients: make(map[string]loginWindow), now: time.Now}
}
func (l *LoginLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	window, ok := l.clients[ip]
	if !ok || !now.Before(window.until) {
		if len(l.clients) >= 1024 {
			for key, w := range l.clients {
				if !now.Before(w.until) {
					delete(l.clients, key)
				}
			}
			if len(l.clients) >= 1024 {
				return false
			}
		}
		window = loginWindow{until: now.Add(time.Minute)}
	}
	if window.count >= 10 {
		return false
	}
	window.count++
	l.clients[ip] = window
	return true
}
func (l *LoginLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if !l.allow(ip) {
			w.Header().Set("Retry-After", "60")
			httpserver.WriteError(w, r, 429, "rate_limited", "Too many login attempts; try again later")
			return
		}
		next.ServeHTTP(w, r)
	})
}
