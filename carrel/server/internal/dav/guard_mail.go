package dav

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
)

// The guard on a person's own mailbox (carrel-mailbox.md M-10, S-5.1): IMAP
// and SMTP are not HTTP, but the name is resolved here, the address is
// checked, the connection goes to the address checked, and the certificate
// is matched against the host's name — as for a DAV server.

// ErrMailPort is a mail server's port other than 993 and 143 for reading,
// 465 and 587 for sending. Port 25 above all: with it Carrel would be a
// relay for bulk mail and a scanner of mail hosts.
var ErrMailPort = errors.New("dav: mail port not allowed")

// ErrMailPlain is a mail connection without TLS to a host outside the
// guard's exceptions (CARREL_DAV_ALLOW_HOSTS).
var ErrMailPlain = errors.New("dav: mail without TLS is allowed only to the hosts excepted")

// mailPorts are the ports a mailbox's servers may have.
var mailPorts = map[int]bool{993: true, 143: true, 465: true, 587: true}

// MailPortAllowed says whether port is one of a mail server's.
func MailPortAllowed(port int) bool { return mailPorts[port] }

// MailPlainAllowed says whether a connection without TLS may go to host: only
// to the hosts the guard excepts, as a DAV server without TLS.
func (g *Guard) MailPlainAllowed(host string) bool { return g.allowlisted(host) }

// DialMail opens a connection to a mail server through the guard: the port
// allowed, the name resolved and every address checked before the dial.
// With implicit TLS the connection comes back in TLS, the certificate held
// to the host's name or to the pinned fingerprint (F-13); without it the
// caller upgrades it with STARTTLS (TLSConfig) — and a plain one goes only to
// a host excepted.
func (g *Guard) DialMail(ctx context.Context, host string, port int, implicitTLS bool, pinnedFingerprint string) (net.Conn, error) {
	if !MailPortAllowed(port) {
		return nil, fmt.Errorf("%w: %d", ErrMailPort, port)
	}
	conn, err := g.dialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return nil, err
	}
	if !implicitTLS {
		return conn, nil
	}
	tc := tls.Client(conn, g.MailTLSConfig(host, pinnedFingerprint))
	if err := tc.HandshakeContext(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return tc, nil
}

// MailTLSConfig is the TLS of a mail connection: the host's name checked, or
// the pinned leaf accepted (F-13).
func (g *Guard) MailTLSConfig(host, pinnedFingerprint string) *tls.Config {
	return tlsConfigWithPin(host, pinnedFingerprint)
}
