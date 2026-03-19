package security

import "testing"

func TestValidateURLTarget_BlocksPrivate(t *testing.T) {
	blocked := []string{
		"http://127.0.0.1/secrets",
		"http://localhost/admin",
		"http://10.0.0.1/internal",
		"http://172.16.0.1/private",
		"http://192.168.1.1/router",
		"http://169.254.169.254/latest/meta-data/",
		"http://[::1]/ipv6-loopback",
		"http://0.0.0.0/",
	}

	for _, rawURL := range blocked {
		if err := ValidateURLTarget(rawURL); err == nil {
			t.Errorf("ValidateURLTarget(%q) should be blocked", rawURL)
		}
	}
}

func TestValidateURLTarget_AllowsPublic(t *testing.T) {
	allowed := []string{
		"https://example.com",
		"https://api.openai.com/v1/chat/completions",
		"http://8.8.8.8/dns",
	}

	for _, rawURL := range allowed {
		if err := ValidateURLTarget(rawURL); err != nil {
			t.Errorf("ValidateURLTarget(%q) should be allowed: %v", rawURL, err)
		}
	}
}

func TestContainsInternalURL(t *testing.T) {
	tests := []struct {
		text    string
		blocked bool
	}{
		{text: "curl http://169.254.169.254/meta-data", blocked: true},
		{text: "wget http://10.0.0.5/secret", blocked: true},
		{text: "echo hello world", blocked: false},
		{text: "curl https://example.com/api", blocked: false},
		{text: "fetch http://192.168.1.1 && echo done", blocked: true},
	}

	for _, tt := range tests {
		err := ContainsInternalURL(tt.text)
		if tt.blocked && err == nil {
			t.Errorf("ContainsInternalURL(%q) should be blocked", tt.text)
		}
		if !tt.blocked && err != nil {
			t.Errorf("ContainsInternalURL(%q) should be allowed: %v", tt.text, err)
		}
	}
}
