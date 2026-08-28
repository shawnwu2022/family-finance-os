package server

import (
	"net/http"
	"net/netip"
	"strings"
)

type trustedProxyBrowserAuth struct {
	BrowserAuth
	trustedProxyCIDR string
}

// WithTrustedProxyCIDR enables forwarded client-address resolution for browser
// authentication only when the immediate TCP peer belongs to the configured
// proxy network. Apply it after WithBrowserAuth. Invalid CIDRs fail closed by
// leaving RemoteAddr as the throttle key; production config validates the CIDR.
func WithTrustedProxyCIDR(cidr string) HandlerOption {
	return func(cfg *handlerConfig) {
		if cfg == nil || cfg.auth == nil {
			return
		}
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			return
		}
		cfg.auth = trustedProxyBrowserAuth{BrowserAuth: cfg.auth, trustedProxyCIDR: cidr}
	}
}

func loginRemoteHostForAuth(r *http.Request, auth BrowserAuth) string {
	if r == nil {
		return "unknown"
	}
	peer := loginRemoteHost(r.RemoteAddr)
	trustedProxyCIDR := trustedProxyCIDRForBrowserAuth(auth)
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

func trustedProxyCIDRForBrowserAuth(auth BrowserAuth) string {
	wrapped, ok := auth.(trustedProxyBrowserAuth)
	if !ok {
		return ""
	}
	return wrapped.trustedProxyCIDR
}
