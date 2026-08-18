package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMCPTokenReadsTrimmedSecretFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp-token")
	if err := os.WriteFile(path, []byte("correct-horse-battery-staple\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	token, err := loadMCPToken(path)
	if err != nil {
		t.Fatalf("loadMCPToken: %v", err)
	}
	if !bytes.Equal(token, []byte("correct-horse-battery-staple")) {
		t.Fatalf("token=%q", token)
	}
}

func TestLoadMCPTokenRejectsInvalidSecretFilesWithoutLeakingContent(t *testing.T) {
	dir := t.TempDir()
	secret := "DO_NOT_LEAK_MCP_SECRET"

	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatalf("write empty: %v", err)
	}
	whitespace := filepath.Join(dir, "whitespace")
	if err := os.WriteFile(whitespace, []byte("  \n\t"), 0o600); err != nil {
		t.Fatalf("write whitespace: %v", err)
	}
	internalWhitespace := filepath.Join(dir, "internal-whitespace")
	if err := os.WriteFile(internalWhitespace, []byte(secret+" bad"), 0o600); err != nil {
		t.Fatalf("write internal whitespace: %v", err)
	}
	oversized := filepath.Join(dir, "oversized")
	if err := os.WriteFile(oversized, bytes.Repeat([]byte("x"), 4097), 0o600); err != nil {
		t.Fatalf("write oversized: %v", err)
	}

	tests := []struct {
		name string
		path string
	}{
		{name: "missing", path: filepath.Join(dir, "missing")},
		{name: "directory", path: dir},
		{name: "empty", path: empty},
		{name: "whitespace", path: whitespace},
		{name: "internal whitespace", path: internalWhitespace},
		{name: "oversized", path: oversized},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadMCPToken(tc.path)
			if err == nil {
				t.Fatalf("loadMCPToken accepted %s", tc.name)
			}
			if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "DO_NOT_LEAK") {
				t.Fatalf("secret content leaked in error: %q", err)
			}
		})
	}
}
