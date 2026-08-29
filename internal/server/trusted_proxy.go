package server

import (
	"net/http"
	"net/netip"
	"strings"
)

// WithTrustedProxyCIDR enables forwarded client-address resolution for browser
// authentication only when the immediate TCP peer belongs to the configured
// proxy network. Invalid CIDRs fail closed by leaving RemoteAddr as the
// throttle key; production config validates the CIDR.
func WithTrustedProxyCIDR(cidr string) HandlerOption {
	return func(cfg *handlerConfig) {
		if cfg == nil {
			return
		}
		cfg.trustedProxyCIDR = strings.TrimSpace(cidr)
	}
}

func loginRemoteHostForAuth(r *http.Request, trustedProxyCIDR string) string {
	if r == nil {
		return "unknown"
	}
	peer := loginRemoteHost(r.RemoteAddr)
	if trustedProxyCIDR == "" {
		return peer
	}
	prefix, err := netip.ParsePrefix(trustedProxyCIDR)
	if err != nil {
		return peer
	}
	peerAddr, err := netip.ParseAddr(peer)
	if err != nil || !prefix.Contains(peerAddr.Unmap()) {
		return peer
	}

	forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if forwarded == "" {
		return peer
	}
	parts := strings.Split(forwarded, ",")
	candidate := strings.TrimSpace(parts[len(parts)-1])
	clientAddr, err := netip.ParseAddr(candidate)
	if err != nil {
		return peer
	}
	return clientAddr.Unmap().String()
}
