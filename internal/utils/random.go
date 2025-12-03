package utils

import (
	"crypto/rand"
	"encoding/hex"
)

func RandomHex(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
