package dblock

import (
	"crypto/rand"
	"encoding/base64"
	"io"
)

func RandomToken(tmp []byte) (string, error) {
	if _, err := io.ReadFull(rand.Reader, tmp); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(tmp), nil
}
