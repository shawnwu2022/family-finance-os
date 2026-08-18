package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

const maxMCPTokenFileBytes = 4096

func loadMCPToken(path string) ([]byte, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("MCP token file path is required")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open MCP token file %q: %w", path, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat MCP token file %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("MCP token file %q must be a regular file", path)
	}
	if info.Size() > maxMCPTokenFileBytes {
		return nil, fmt.Errorf("MCP token file %q exceeds %d bytes", path, maxMCPTokenFileBytes)
	}

	raw, err := io.ReadAll(io.LimitReader(file, maxMCPTokenFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read MCP token file %q: %w", path, err)
	}
	if len(raw) > maxMCPTokenFileBytes {
		return nil, fmt.Errorf("MCP token file %q exceeds %d bytes", path, maxMCPTokenFileBytes)
	}

	token := bytes.TrimSpace(raw)
	if len(token) == 0 {
		return nil, fmt.Errorf("MCP token file %q is empty", path)
	}
	if strings.IndexFunc(string(token), unicode.IsSpace) >= 0 {
		return nil, fmt.Errorf("MCP token file %q contains whitespace", path)
	}
	return append([]byte(nil), token...), nil
}
