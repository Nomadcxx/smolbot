package provider

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// PKCEParams holds code verifier and challenge
type PKCEParams struct {
	Verifier  string
	Challenge string
	Method    string // always "S256"
}

// GeneratePKCE creates a new PKCE code verifier and S256 challenge
func GeneratePKCE() (*PKCEParams, error) {
	verifier, err := generateRandomString(64)
	if err != nil {
		return nil, err
	}
	challenge := S256Challenge(verifier)
	return &PKCEParams{
		Verifier:  verifier,
		Challenge: challenge,
		Method:    "S256",
	}, nil
}

// generateRandomString generates a URL-safe random string
func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// S256Challenge creates the S256 code challenge from a verifier
func S256Challenge(verifier string) string {
	h := sha256.New()
	h.Write([]byte(verifier))
	digest := h.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(digest)
}
