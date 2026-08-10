// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ProxyTrust decides which forwarding headers to believe. Anything a client
// can set is worthless unless it arrived through a proxy the operator named,
// so the trusted list is explicit and empty by default (§4, §18.1).
type ProxyTrust struct {
	prefixes []netip.Prefix
}

// NewProxyTrust builds a trust set from IP addresses and CIDR blocks.
func NewProxyTrust(entries []string) (*ProxyTrust, error) {
	t := &ProxyTrust{}
	for _, raw := range entries {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if strings.Contains(raw, "/") {
			p, err := netip.ParsePrefix(raw)
			if err != nil {
				return nil, fmt.Errorf("handler: trusted proxy %q: %w", raw, err)
			}
			t.prefixes = append(t.prefixes, p.Masked())
			continue
		}
		addr, err := netip.ParseAddr(raw)
		if err != nil {
			return nil, fmt.Errorf("handler: trusted proxy %q: %w", raw, err)
		}
		t.prefixes = append(t.prefixes, netip.PrefixFrom(addr, addr.BitLen()))
	}
	return t, nil
}

// Trusted reports whether addr is one of the configured proxies.
func (t *ProxyTrust) Trusted(addr netip.Addr) bool {
	if t == nil || !addr.IsValid() {
		return false
	}
	addr = addr.Unmap()
	for _, p := range t.prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// ClientIP returns the address to attribute a request to. It walks
// X-Forwarded-For from the right, accepting a hop only while the one before it
// was a trusted proxy; the first untrusted address wins. A client that sends
// its own X-Forwarded-For therefore cannot forge its position in the chain,
// which matters because this address is what the rate limiter counts (§24.5).
func (t *ProxyTrust) ClientIP(r *http.Request) string {
	addr := remoteAddr(r)
	if !t.Trusted(addr) {
		return addrString(addr)
	}

	hops := forwardedFor(r)
	for i := len(hops) - 1; i >= 0; i-- {
		hop, err := netip.ParseAddr(hops[i])
		if err != nil {
			// An unparseable hop breaks the chain; trust nothing beyond it.
			return addrString(addr)
		}
		if !t.Trusted(hop) {
			return addrString(hop.Unmap())
		}
		addr = hop
	}
	return addrString(addr)
}

// IsHTTPS reports whether the browser reached the service over TLS. Only a
// trusted proxy's X-Forwarded-Proto counts; on that answer hang the Secure
// cookie flag and HSTS (§24.5).
func (t *ProxyTrust) IsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if !t.Trusted(remoteAddr(r)) {
		return false
	}
	proto := r.Header.Get("X-Forwarded-Proto")
	if i := strings.IndexByte(proto, ','); i >= 0 {
		proto = proto[:i]
	}
	return strings.EqualFold(strings.TrimSpace(proto), "https")
}

func forwardedFor(r *http.Request) []string {
	var out []string
	for _, header := range r.Header.Values("X-Forwarded-For") {
		for _, part := range strings.Split(header, ",") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func remoteAddr(r *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, err := netip.ParseAddr(strings.Trim(host, "[]"))
	if err != nil {
		return netip.Addr{}
	}
	return addr.Unmap()
}

func addrString(addr netip.Addr) string {
	if !addr.IsValid() {
		return ""
	}
	return addr.String()
}
