package auth

import (
	"bytes"
	"testing"
)

func TestTokens(t *testing.T) {
	a, hash, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	b, _, _ := newToken()
	if a == b || !validToken(a) || len(hash) != 32 || !bytes.Equal(hash, digest(a)) {
		t.Fatal("invalid token generation")
	}
	if validToken("password") || validToken(a+"=") {
		t.Fatal("non-canonical token accepted")
	}
}
