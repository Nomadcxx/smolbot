package provider

import (
	"testing"
)

func TestGeneratePKCE(t *testing.T) {
	pkce, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE() error = %v", err)
	}
	if len(pkce.Verifier) == 0 {
		t.Error("Verifier should not be empty")
	}
	if pkce.Challenge == "" {
		t.Error("Challenge should not be empty")
	}
	if pkce.Method != "S256" {
		t.Errorf("Method = %q, want S256", pkce.Method)
	}
	// Verify it's non-deterministic
	pkce2, _ := GeneratePKCE()
	if pkce.Verifier == pkce2.Verifier {
		t.Error("Two PKCE generations should produce different verifiers")
	}
}

func TestS256Challenge(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	challenge := S256Challenge(verifier)
	challenge2 := S256Challenge(verifier)
	if challenge != challenge2 {
		t.Error("S256Challenge should be deterministic")
	}
	if len(challenge) == 0 {
		t.Error("Challenge should not be empty")
	}
}
