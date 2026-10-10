package health

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeRepository struct {
	err  error
	wait bool
}

func (f fakeRepository) Check(ctx context.Context) error {
	if f.wait {
		<-ctx.Done()
		return ctx.Err()
	}
	return f.err
}

func TestReadyResponses(t *testing.T) {
	for name, repo := range map[string]fakeRepository{"ok": {}, "down": {err: errors.New("private connection detail")}, "timeout": {wait: true}} {
		t.Run(name, func(t *testing.T) {
			s := NewService(repo, 5*time.Millisecond)
			w := httptest.NewRecorder()
			s.Handler(w, httptest.NewRequest("GET", "/health/ready", nil))
			want := 200
			if name != "ok" {
				want = 503
			}
			if w.Code != want {
				t.Fatalf("got %d", w.Code)
			}
			if strings.Contains(w.Body.String(), "private connection detail") {
				t.Fatal("leaked error")
			}
		})
	}
}
