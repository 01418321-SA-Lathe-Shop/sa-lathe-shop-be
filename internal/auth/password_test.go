package auth

import (
	"context"
	"errors"
	"testing"
)

func TestPasswordHash(t *testing.T) {
	h := NewArgonHasher()
	ctx := context.Background()
	hash, err := h.Hash(ctx, "Test-password-1234")
	if err != nil {
		t.Fatal(err)
	}
	for password, want := range map[string]bool{"Test-password-1234": true, "wrong-password": false} {
		got, err := h.Verify(ctx, hash, password)
		if err != nil || got != want {
			t.Fatalf("verify: %v %v", got, err)
		}
	}
	if _, err = h.Verify(ctx, "$argon2id$v=19$m=999999999,t=999,p=99$x$x", "test"); err == nil {
		t.Fatal("unbounded parameters accepted")
	}
}
func TestPasswordPolicyAndCancellation(t *testing.T) {
	if validatePassword("กกกก") == nil {
		t.Fatal("byte length mistaken for character count")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewArgonHasher().Hash(ctx, "Test-password-1234"); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation ignored")
	}
}
