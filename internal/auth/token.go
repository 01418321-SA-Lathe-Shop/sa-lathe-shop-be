package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

func newToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, digest(token), nil
}
func digest(token string) []byte { h := sha256.Sum256([]byte(token)); return h[:] }
func validToken(token string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(raw) == 32 && base64.RawURLEncoding.EncodeToString(raw) == token
}
