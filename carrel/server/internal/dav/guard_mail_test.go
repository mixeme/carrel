package dav

import (
	"context"
	"errors"
	"net"
	"testing"
)

// countingResolver counts the names asked: a refusal before the dial asks
// none.
type countingResolver struct {
	addrs []net.IPAddr
	asked int
}

func (r *countingResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	r.asked++
	return r.addrs, nil
}

// The guard on a mailbox (S-5.1, carrel-mailbox.md M-10): port 25 and any
// port but 993, 143, 465, 587 is refused before the name is even resolved;
// an address inside is refused as for DAV; without TLS only to a host
// excepted.
func TestGuardOnMail(t *testing.T) {
	c := testConfig()
	c.DAVSSRFAllowlist = "mail.test"
	g := NewGuard(c)
	r := &countingResolver{addrs: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}}
	g.resolver = r
	ctx := context.Background()

	for _, port := range []int{25, 80, 2525} {
		if _, err := g.DialMail(ctx, "smtp.example.org", port, true, ""); !errors.Is(err, ErrMailPort) {
			t.Errorf("port %d: %v, want ErrMailPort", port, err)
		}
	}
	if r.asked != 0 {
		t.Errorf("a refused port resolved its name %d times", r.asked)
	}
	if _, err := g.DialMail(ctx, "imap.example.org", 993, true, ""); !errors.Is(err, ErrSSRF) {
		t.Errorf("an address inside: %v, want ErrSSRF", err)
	}
	// Excepted, the dial goes; nothing listens there, so it fails, but not
	// by the guard.
	if _, err := g.DialMail(ctx, "mail.test", 143, false, ""); err == nil || errors.Is(err, ErrSSRF) || errors.Is(err, ErrMailPort) {
		t.Errorf("an excepted host: %v, want a refusal of the network, not the guard's", err)
	}
	if g.MailPlainAllowed("imap.example.org") || !g.MailPlainAllowed("MAIL.test.") {
		t.Error("plain text: only the excepted host")
	}
}
