package conf_test

import (
	"net/netip"
	"testing"

	"carrel.dev/carrel/server/internal/conf"
)

// CARREL_TRUSTED_PROXIES is one list read by two listeners; commas, spaces
// and semicolons all separate, and what does not parse is ignored rather
// than trusted (audit №3, Ф3-5: davloom read commas only and trusted nobody
// behind a proxy the web trusted).
func TestTrustedProxiesSeparators(t *testing.T) {
	got := conf.ParseTrustedProxies("10.0.0.0/8; 127.0.0.1 ::1,192.0.2.1 , nonsense")
	want := []string{"10.0.0.0/8", "127.0.0.1/32", "::1/128", "192.0.2.1/32"}
	if len(got) != len(want) {
		t.Fatalf("parsed %v, want %v", got, want)
	}
	for i, p := range got {
		if p.String() != want[i] {
			t.Errorf("[%d] = %s, want %s", i, p, want[i])
		}
	}
	if !conf.TrustedAddr(got, netip.MustParseAddr("::ffff:10.1.2.3")) {
		t.Error("an IPv4-mapped address of a trusted range is not trusted")
	}
	if conf.TrustedAddr(got, netip.MustParseAddr("203.0.113.9")) {
		t.Error("a stranger is trusted")
	}
	if n := len(conf.ParseTrustedProxies("")); n != 0 {
		t.Errorf("empty list = %d prefixes", n)
	}
}
