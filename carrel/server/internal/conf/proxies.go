package conf

import (
	"net/netip"
	"strings"
)

// ParseTrustedProxies reads CARREL_TRUSTED_PROXIES: addresses and CIDRs
// separated by commas, spaces or semicolons. Anything unparsable is ignored
// rather than trusted. Both listeners read the list through this one
// function: the web and davloom trusting different proxies from the same
// setting sent every device behind the proxy into one bucket (Ф3-5).
func ParseTrustedProxies(list string) []netip.Prefix {
	var out []netip.Prefix
	for _, item := range strings.FieldsFunc(list, func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
		if p, err := netip.ParsePrefix(item); err == nil {
			out = append(out, p.Masked())
			continue
		}
		if a, err := netip.ParseAddr(item); err == nil {
			out = append(out, netip.PrefixFrom(a, a.BitLen()))
		}
	}
	return out
}

// TrustedAddr reports whether a is in one of the prefixes; an IPv4-mapped
// IPv6 address is the IPv4 address it maps.
func TrustedAddr(proxies []netip.Prefix, a netip.Addr) bool {
	a = a.Unmap()
	for _, p := range proxies {
		if p.Contains(a) {
			return true
		}
	}
	return false
}
