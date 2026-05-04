package service

import (
	"crypto/rand"
	"encoding/hex"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

func NewID(prefix string) (string, error) {
	id, err := gonanoid.New(14)
	if err != nil {
		return "", err
	}
	return prefix + "_" + id, nil
}

func NewRunnerToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "rtkn_" + hex.EncodeToString(buf), nil
}
