package main

import (
	"net"
	"net/url"
	"strings"
)

// isLoopbackProviderBaseURL preserves the provider transport boundary used by
// local mode: TLS endpoints are allowed (for example a LAN P40 host behind
// HTTPS), while plaintext HTTP is only allowed for an IP-literal loopback.
func isLoopbackProviderBaseURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	if parsed.Scheme != "http" {
		return false
	}
	ip := net.ParseIP(parsed.Hostname())
	return ip != nil && ip.IsLoopback()
}
