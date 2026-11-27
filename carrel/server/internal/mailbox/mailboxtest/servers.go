// Package mailboxtest holds the mail servers the tests of people's own
// mailboxes talk to (carrel-mailbox.md M-3): an IMAP server in memory, a
// small SMTP server that answers EHLO and AUTH and sends nothing, and an
// IMAP server of four commands that announces the roles of its folders. All
// of them are plain text on 127.0.0.1; Dial points the ports a mail server
// may have at them, whatever the host, so a test types the hosts of a frame.
package mailboxtest

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
)

// Servers are one person's mail servers: IMAP and SMTP, signing in with
// Login and Password.
type Servers struct {
	IMAP, SMTP      string // host:port on 127.0.0.1
	Login, Password string
	refuseSMTP      atomic.Bool
	size            atomic.Int64
}

// Start runs the servers for login and password: the IMAP server holds
// folders (with their attributes when specialUse — the server in memory
// keeps none of them, see Announcing); the SMTP server takes AUTH PLAIN and
// says SIZE 10485760.
func Start(t testing.TB, login, password string, folders map[string][]imap.MailboxAttr, specialUse bool) *Servers {
	t.Helper()
	mem := imapmemserver.New()
	u := imapmemserver.NewUser(login, password)
	for name, attrs := range folders {
		if err := u.Create(name, &imap.CreateOptions{SpecialUse: attrs}); err != nil {
			t.Fatal(err)
		}
	}
	mem.AddUser(u)
	caps := imap.CapSet{imap.CapIMAP4rev1: {}}
	if specialUse {
		caps[imap.CapSpecialUse] = struct{}{}
		caps[imap.CapCreateSpecialUse] = struct{}{}
	}
	srv := imapserver.New(&imapserver.Options{
		NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return mem.NewSession(), nil, nil
		},
		Caps:         caps,
		InsecureAuth: true,
		Logger:       discard{},
	})
	imapLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(imapLn) }()
	t.Cleanup(func() { _ = srv.Close() })

	s := &Servers{IMAP: imapLn.Addr().String(), Login: login, Password: password}
	s.size.Store(10485760)
	smtpLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = smtpLn.Close() })
	s.SMTP = smtpLn.Addr().String()
	go s.serveSMTP(smtpLn)
	return s
}

// RefuseSMTP makes the SMTP server refuse the password, as a server that
// keeps a separate one for sending (s-mailbox-25), or take it again.
func (s *Servers) RefuseSMTP(refuse bool) { s.refuseSMTP.Store(refuse) }

// SetSize is what SIZE of EHLO says: «up to 25 MB» is 26214400.
func (s *Servers) SetSize(n int64) { s.size.Store(n) }

// Dial is a prober's dial to these servers: IMAP at 993 and 143, SMTP at
// 465 and 587, whatever the host; the connection is plain text even where
// the server is reached over TLS, since the dial is what does TLS.
func (s *Servers) Dial(ctx context.Context, host string, port int, implicitTLS bool, pin string) (net.Conn, error) {
	addr := map[int]string{993: s.IMAP, 143: s.IMAP, 465: s.SMTP, 587: s.SMTP}[port]
	if addr == "" {
		return nil, fmt.Errorf("mailboxtest: no server at port %d", port)
	}
	return (&net.Dialer{}).DialContext(ctx, "tcp", addr)
}

// serveSMTP answers EHLO, AUTH PLAIN — the login and password, unless
// refused — and QUIT; nothing else.
func (s *Servers) serveSMTP(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go func(conn net.Conn) {
			defer conn.Close()
			r := bufio.NewReader(conn)
			fmt.Fprintf(conn, "220 smtp.test ESMTP\r\n")
			for {
				line, err := r.ReadString('\n')
				if err != nil {
					return
				}
				cmd := strings.ToUpper(strings.TrimSpace(line))
				switch {
				case strings.HasPrefix(cmd, "EHLO"):
					fmt.Fprintf(conn, "250-smtp.test\r\n250-SIZE %d\r\n250 AUTH PLAIN\r\n", s.size.Load())
				case strings.HasPrefix(cmd, "AUTH PLAIN"):
					fields := strings.Fields(strings.TrimSpace(line))
					raw, _ := base64.StdEncoding.DecodeString(fields[len(fields)-1])
					if !s.refuseSMTP.Load() && string(raw) == "\x00"+s.Login+"\x00"+s.Password {
						fmt.Fprintf(conn, "235 2.7.0 Authentication successful\r\n")
					} else {
						fmt.Fprintf(conn, "535 5.7.8 Error: authentication failed\r\n")
					}
				case strings.HasPrefix(cmd, "QUIT"):
					fmt.Fprintf(conn, "221 bye\r\n")
					return
				default:
					fmt.Fprintf(conn, "502 not here\r\n")
				}
			}
		}(conn)
	}
}

type discard struct{}

func (discard) Printf(string, ...any) {}

// Announcing is an IMAP server of four commands that announces the roles of
// its folders (SPECIAL-USE, RFC 6154), and lets in any login. Each of
// folders is a line of LIST after its first attribute: «(\HasNoChildren
// \Sent) "/" Sent»; none — INBOX, Sent, Drafts, Bin as the trash, and
// Shared that cannot be selected.
func Announcing(t testing.TB, folders ...string) string {
	t.Helper()
	list := ""
	for _, f := range folders {
		list += "* LIST " + f + "\r\n"
	}
	if list == "" {
		list = "* LIST (\\HasNoChildren) \"/\" INBOX\r\n" +
			"* LIST (\\HasNoChildren \\Sent) \"/\" Sent\r\n" +
			"* LIST (\\HasNoChildren \\Drafts) \"/\" Drafts\r\n" +
			"* LIST (\\HasNoChildren \\Trash) \"/\" Bin\r\n" +
			"* LIST (\\Noselect \\HasChildren) \"/\" Shared\r\n"
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				r := bufio.NewReader(conn)
				fmt.Fprintf(conn, "* OK [CAPABILITY IMAP4rev1 SPECIAL-USE LIST-EXTENDED] ready\r\n")
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					f := strings.Fields(line)
					if len(f) < 2 {
						continue
					}
					tag, cmd := f[0], strings.ToUpper(f[1])
					switch cmd {
					case "CAPABILITY":
						fmt.Fprintf(conn, "* CAPABILITY IMAP4rev1 SPECIAL-USE LIST-EXTENDED\r\n%s OK done\r\n", tag)
					case "LOGIN":
						fmt.Fprintf(conn, "%s OK [CAPABILITY IMAP4rev1 SPECIAL-USE LIST-EXTENDED] in\r\n", tag)
					case "LIST":
						fmt.Fprintf(conn, "%s%s OK listed\r\n", list, tag)
					case "LOGOUT":
						fmt.Fprintf(conn, "* BYE\r\n%s OK out\r\n", tag)
						return
					default:
						fmt.Fprintf(conn, "%s BAD not here\r\n", tag)
					}
				}
			}(conn)
		}
	}()
	return ln.Addr().String()
}
