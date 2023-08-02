package dblock

import (
	"crypto/rand"
	"encoding/base64"
	"io"
)

// RandomToken generates a random token.
func RandomToken() (string, error) {
	tmp := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, tmp); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(tmp), nil
}
