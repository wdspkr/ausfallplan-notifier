package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoad_MissingFile asserts that a missing config file returns an empty Config and no error.
func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(filepath.Join(dir, "does_not_exist.json"))
	if err != nil {
		t.Fatalf("Load(missing): expected nil error, got %v", err)
	}
	if len(cfg.Blacklist) != 0 {
		t.Errorf("Load(missing): expected empty blacklist, got %v", cfg.Blacklist)
	}
}

// TestLoad_ValidJSON asserts that a valid JSON config file is parsed correctly.
func TestLoad_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	content := `{"blacklist":["1a","2b","3c"]}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load(valid): unexpected error: %v", err)
	}
	want := []string{"1a", "2b", "3c"}
	if len(cfg.Blacklist) != len(want) {
		t.Fatalf("Load(valid): blacklist length %d, want %d", len(cfg.Blacklist), len(want))
	}
	for i, v := range want {
		if cfg.Blacklist[i] != v {
			t.Errorf("Load(valid): blacklist[%d] = %q, want %q", i, cfg.Blacklist[i], v)
		}
	}
}

// TestLoad_MalformedJSON asserts that a malformed JSON config file returns an error.
func TestLoad_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := os.WriteFile(path, []byte(`{not valid json`), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load(malformed): expected error, got nil")
	}
}

// TestLoad_EmptyFile asserts that an empty file returns a JSON unmarshal error.
func TestLoad_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load(empty): expected error, got nil")
	}
}
