package auth

import (
	"testing"
	"time"
)

func TestLoginLimiter(t *testing.T) {
	l := NewLoginLimiter()
	now := time.Now()
	l.now = func() time.Time { return now }
	for i := 0; i < 10; i++ {
		if !l.allow("127.0.0.1") {
			t.Fatal("blocked too early")
		}
	}
	if l.allow("127.0.0.1") {
		t.Fatal("unlimited attempts")
	}
	now = now.Add(time.Minute)
	if !l.allow("127.0.0.1") {
		t.Fatal("window did not reset")
	}
	for i := 0; i < 1024; i++ {
		l.clients[time.Unix(int64(i), 0).String()] = loginWindow{until: now.Add(time.Minute)}
	}
	if l.allow("another-client") {
		t.Fatal("unbounded client map")
	}
}
