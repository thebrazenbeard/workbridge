package config

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDefaultsAndRejectsUnknownFields(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	payload, _ := json.Marshal(map[string]any{
		"schema": Schema,
		"read_roots": []string{root},
		"write_roots": []string{},
		"limits": map[string]any{},
		"process": map[string]any{"enabled": false},
		"http": map[string]any{},
	})
	cfg, err := Parse(payload)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTP.Listen != "127.0.0.1:8765" || cfg.HTTP.Path != "/mcp" {
		t.Fatalf("defaults not applied: %#v", cfg.HTTP)
	}
	if cfg.Process.MaxArgs != 64 {
		t.Fatalf("process defaults not applied: %#v", cfg.Process)
	}
	_, err = Parse([]byte(`{"schema":"WORKBRIDGE_CONFIG_V1","read_roots":[],"write_roots":[],"limits":{},"process":{"enabled":false},"http":{},"surprise":true}`))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown field accepted: %v", err)
	}
}

func TestRejectsNonLoopbackHTTPAndUnpinnedProcessExecutable(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	cfg := Config{
		Schema: Schema,
		Process: ProcessConfig{
			Enabled: true,
			AllowedExecutables: []ExecutableGrant{{
				Name: "shell",
				Path: "cmd.exe",
				SHA256: strings.Repeat("a", 64),
			}},
			WorkingRoots: []string{root},
			MaxRuntimeSeconds: 10,
			MaxOutputBytes: 1024,
			MaxArgs: 8,
		},
		Limits: Limits{MaxReadBytes: 1024, MaxWriteBytes: 1024, MaxDirectoryEntries: 10},
		HTTP: HTTPConfig{Listen: "0.0.0.0:8765", Path: "/mcp"},
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("non-loopback listen accepted: %v", err)
	}
	cfg.HTTP.Listen = "127.0.0.1:8765"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("relative executable accepted: %v", err)
	}
	cfg.Process.AllowedExecutables[0].Path = filepath.Join(root, "tool.exe")
	cfg.Process.AllowedExecutables[0].SHA256 = "NOT-A-HASH"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("invalid executable identity accepted: %v", err)
	}
}

func TestRejectsDuplicateGrantNamesAndBadTokenEnv(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	grant := ExecutableGrant{Name: "tool", Path: filepath.Join(root, "tool.exe"), SHA256: strings.Repeat("a", 64)}
	cfg := Config{
		Schema: Schema,
		Limits: Limits{MaxReadBytes: 1, MaxWriteBytes: 1, MaxDirectoryEntries: 1},
		Process: ProcessConfig{
			Enabled: true, AllowedExecutables: []ExecutableGrant{grant, grant},
			WorkingRoots: []string{root}, MaxRuntimeSeconds: 1, MaxOutputBytes: 1, MaxArgs: 1,
		},
		HTTP: HTTPConfig{Listen: "127.0.0.1:1", Path: "/mcp"},
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate grant name accepted: %v", err)
	}
	cfg.Process.Enabled = false
	cfg.HTTP.BearerTokenEnv = "BAD=ENV"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "environment variable") {
		t.Fatalf("bad token env accepted: %v", err)
	}
}


func TestProcessMaxConcurrentDefaultsAndValidates(t *testing.T) {
	base := []byte(`{"schema":"WORKBRIDGE_CONFIG_V1","read_roots":[],"write_roots":[],"limits":{},"process":{"enabled":false},"http":{}}`)
	cfg, err := Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Process.MaxConcurrent != 4 {
		t.Fatalf("default max_concurrent = %d, want 4", cfg.Process.MaxConcurrent)
	}

	valid := []byte(`{"schema":"WORKBRIDGE_CONFIG_V1","read_roots":[],"write_roots":[],"limits":{},"process":{"enabled":false,"max_concurrent":1},"http":{}}`)
	cfg, err = Parse(valid)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Process.MaxConcurrent != 1 {
		t.Fatalf("explicit max_concurrent = %d, want 1", cfg.Process.MaxConcurrent)
	}

	tooHigh := []byte(`{"schema":"WORKBRIDGE_CONFIG_V1","read_roots":[],"write_roots":[],"limits":{},"process":{"enabled":false,"max_concurrent":33},"http":{}}`)
	if _, err := Parse(tooHigh); err == nil {
		t.Fatal("max_concurrent above 32 accepted")
	}
}
