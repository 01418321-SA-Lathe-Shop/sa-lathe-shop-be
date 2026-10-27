package auth

import (
	"errors"
	"testing"
)

func TestProfileTextIsPostgreSQLCompatible(t *testing.T) {
	for _, value := range []string{"name\x00hidden", string([]byte{0xff})} {
		if !errors.Is(validateName(value), ErrInvalidInput) || !errors.Is(validatePosition(value), ErrInvalidInput) {
			t.Fatal("NUL/invalid UTF-8 must be rejected before persistence")
		}
	}
	if validateName("ช่างกลึง") != nil || validatePosition("") != nil {
		t.Fatal("valid Unicode name or optional empty position was rejected")
	}
}
