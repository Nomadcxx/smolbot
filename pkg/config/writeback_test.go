package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/config"
)

func TestAtomicWriteConfig_WritesValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "test-model"

	if err := config.AtomicWriteConfig(path, &cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out config.Config
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("written file is not valid JSON: %v", err)
	}
	if out.Agents.Defaults.Model != "test-model" {
		t.Fatalf("model mismatch: got %q", out.Agents.Defaults.Model)
	}
}

func TestAtomicWriteConfig_CreatesFileWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.json")
	cfg := config.DefaultConfig()

	if err := config.AtomicWriteConfig(path, &cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("file not created")
	}
}

func TestAtomicWriteConfig_FilePermissions(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores file permissions")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := config.DefaultConfig()

	if err := config.AtomicWriteConfig(path, &cfg); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected 0o600, got %04o", got)
	}
}
