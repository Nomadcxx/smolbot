package main

import "testing"

func TestDirectBootstrapParsesDefaultFlags(t *testing.T) {
	cfg, err := parseConfig(nil)
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	if cfg.Host != "127.0.0.1" {
		t.Fatalf("host = %q, want 127.0.0.1", cfg.Host)
	}
	if cfg.Port != 18790 {
		t.Fatalf("port = %d, want 18790", cfg.Port)
	}
	if cfg.Theme != "" {
		t.Fatalf("theme = %q, want empty default", cfg.Theme)
	}
	if cfg.Session != "" {
		t.Fatalf("session = %q, want empty default", cfg.Session)
	}
}
