package mailbox

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/sealed"
)

// An autoconfig file (Thunderbird's config-v1.1.xml): IMAP and SMTP taken
// with the address's placeholders filled, POP3 left — Carrel reads over IMAP
// only (M-2) —, port 25 left — the guard closes it (S-5.1).
func TestAutoconfigFile(t *testing.T) {
	body := []byte(`<?xml version="1.0"?>
<clientConfig version="1.1"><emailProvider id="example.org">
  <incomingServer type="pop3"><hostname>pop.example.org</hostname><port>995</port><socketType>SSL</socketType></incomingServer>
  <incomingServer type="imap"><hostname>imap.%EMAILDOMAIN%</hostname><port>993</port><socketType>SSL</socketType><username>%EMAILLOCALPART%</username></incomingServer>
  <outgoingServer type="smtp"><hostname>smtp.example.org</hostname><port>25</port><socketType>plain</socketType></outgoingServer>
  <outgoingServer type="smtp"><hostname>smtp.example.org</hostname><port>587</port><socketType>STARTTLS</socketType></outgoingServer>
</emailProvider></clientConfig>`)
	f, err := parseAutoconfig(body, "anna@example.org")
	if err != nil {
		t.Fatal(err)
	}
	want := Found{
		IMAP:  []sealed.MailServer{{Host: "imap.example.org", Port: 993, Security: sealed.MailTLS}},
		SMTP:  []sealed.MailServer{{Host: "smtp.example.org", Port: 587, Security: sealed.MailSTARTTLS}},
		Login: "anna",
	}
	if !slices.Equal(f.IMAP, want.IMAP) || !slices.Equal(f.SMTP, want.SMTP) || f.Login != want.Login {
		t.Errorf("autoconfig = %+v, want %+v", f, want)
	}
}

// Discovery by the order of M-3: SRV records, then the autoconfig file, the
// guesses only where nothing was found — and each step in the trace.
func TestDiscoverOrder(t *testing.T) {
	p := &Prober{Timeout: time.Second,
		SRV: func(ctx context.Context, service, domain string) ([]*net.SRV, error) {
			if service == "imaps" && domain == "example.org" {
				return []*net.SRV{{Target: "mx.example.org.", Port: 993}}, nil
			}
			return nil, errors.New("no such host")
		},
		Fetch: func(ctx context.Context, address string) ([]byte, error) {
			if strings.HasPrefix(address, "https://autoconfig.example.org/") {
				return nil, errors.New("404 Not Found")
			}
			return []byte(`<clientConfig><emailProvider><outgoingServer type="smtp"><hostname>out.example.org</hostname><port>465</port><socketType>SSL</socketType></outgoingServer></emailProvider></clientConfig>`), nil
		},
	}
	var steps []string
	f := p.Discover(context.Background(), "anna@example.org", func(name, detail string, ok bool) {
		steps = append(steps, fmt.Sprintf("%s %v %s", name, ok, detail))
	})
	if len(f.IMAP) != 1 || f.IMAP[0].Host != "mx.example.org" || len(f.SMTP) != 1 || f.SMTP[0].Host != "out.example.org" {
		t.Errorf("found = %+v: the SRV record and the file, no guesses", f)
	}
	if len(steps) != 3 || !strings.HasPrefix(steps[0], "SRV true _imaps._tcp → mx.example.org:993") ||
		!strings.HasPrefix(steps[1], "autoconfig false GET https://autoconfig.example.org/mail/config-v1.1.xml → 404") ||
		!strings.HasPrefix(steps[2], "autoconfig true") {
		t.Errorf("steps = %q", steps)
	}
	// Nothing found: the guesses of the address's own domain.
	p.SRV = func(context.Context, string, string) ([]*net.SRV, error) { return nil, errors.New("no") }
	p.Fetch = func(context.Context, string) ([]byte, error) { return nil, errors.New("no") }
	f = p.Discover(context.Background(), "anna@example.org", func(string, string, bool) {})
	if len(f.IMAP) != 2 || f.IMAP[0].Host != "imap.example.org" || f.SMTP[0] != (sealed.MailServer{Host: "smtp.example.org", Port: 465, Security: sealed.MailTLS}) {
		t.Errorf("guesses = %+v", f)
	}
}

// testServers are an IMAP server in memory and a small SMTP server, both
// plain on 127.0.0.1 (excepted by the guard), and a prober that points the
// allowed ports at them: 143 — IMAP, 587 — SMTP.
type testServers struct {
	prober   *Prober
	smtpAuth bool // the SMTP server accepts AUTH
}

func newTestServers(t *testing.T, folders map[string][]imap.MailboxAttr, specialUse bool) *testServers {
	t.Helper()
	mem := imapmemserver.New()
	u := imapmemserver.NewUser("anna@example.org", "secret")
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

	ts := &testServers{smtpAuth: true}
	smtpLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = smtpLn.Close() })
	go ts.serveSMTP(smtpLn)

	c := &conf.Config{DAVConnectTimeout: 5 * time.Second, DAVRequestTimeout: 10 * time.Second, DAVMaxResponseBytes: 1 << 20,
		DAVMaxRedirects: 5, DAVSSRFAllowlist: "127.0.0.1,localhost"}
	g := dav.NewGuard(c)
	ports := map[int]string{143: imapLn.Addr().String(), 587: smtpLn.Addr().String()}
	ts.prober = &Prober{Guard: g, Timeout: 5 * time.Second,
		SRV:   func(context.Context, string, string) ([]*net.SRV, error) { return nil, errors.New("no") },
		Fetch: func(context.Context, string) ([]byte, error) { return nil, errors.New("no") },
		Dial: func(ctx context.Context, host string, port int, implicitTLS bool, pin string) (net.Conn, error) {
			if _, err := g.DialMail(ctx, host, port, false, pin); errors.Is(err, dav.ErrMailPort) || errors.Is(err, dav.ErrSSRF) {
				return nil, err // the guard's word stands
			}
			return (&net.Dialer{}).DialContext(ctx, "tcp", ports[port])
		},
	}
	return ts
}

// serveSMTP answers EHLO and AUTH PLAIN, accepting «anna@example.org» with
// «secret» while smtpAuth, and QUIT; nothing else.
func (ts *testServers) serveSMTP(ln net.Listener) {
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
					fmt.Fprintf(conn, "250-smtp.test\r\n250-SIZE 10485760\r\n250 AUTH PLAIN\r\n")
				case strings.HasPrefix(cmd, "AUTH PLAIN"):
					fields := strings.Fields(strings.TrimSpace(line))
					raw, _ := base64.StdEncoding.DecodeString(fields[len(fields)-1])
					if ts.smtpAuth && string(raw) == "\x00anna@example.org\x00secret" {
						fmt.Fprintf(conn, "235 2.7.0 Authentication successful\r\n")
					} else {
						fmt.Fprintf(conn, "535 5.7.8 Username and Password not accepted\r\n")
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

var typed = Input{Address: "anna@example.org", Password: "secret",
	IMAP: sealed.MailServer{Host: "127.0.0.1", Port: 143, Security: sealed.MailPlain},
	SMTP: sealed.MailServer{Host: "127.0.0.1", Port: 587, Security: sealed.MailPlain}}

// specialUseIMAP is an IMAP server of four commands that announces the
// roles of its folders (SPECIAL-USE, RFC 6154) — the server in memory of
// go-imap keeps none. Plain, on 127.0.0.1.
func specialUseIMAP(t *testing.T) string {
	t.Helper()
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
						fmt.Fprintf(conn, "* LIST (\\HasNoChildren) \"/\" INBOX\r\n"+
							"* LIST (\\HasNoChildren \\Sent) \"/\" Sent\r\n"+
							"* LIST (\\HasNoChildren \\Drafts) \"/\" Drafts\r\n"+
							"* LIST (\\HasNoChildren \\Trash) \"/\" Bin\r\n"+
							"* LIST (\\Noselect \\HasChildren) \"/\" Shared\r\n%s OK listed\r\n", tag)
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

// Signing in (M-3): IMAP lets in, the roles come from SPECIAL-USE — a
// folder that cannot be selected is none —, SMTP accepts AUTH and nothing is
// sent; every step is in the trace.
func TestCheckSignsIn(t *testing.T) {
	ts := newTestServers(t, nil, true)
	announcing := specialUseIMAP(t)
	dial := ts.prober.Dial
	ts.prober.Dial = func(ctx context.Context, host string, port int, implicitTLS bool, pin string) (net.Conn, error) {
		if port == 143 {
			return (&net.Dialer{}).DialContext(ctx, "tcp", announcing)
		}
		return dial(ctx, host, port, implicitTLS, pin)
	}
	res, trace, err := ts.prober.Check(context.Background(), typed)
	if err != nil {
		t.Fatalf("check: %v %+v", err, trace)
	}
	if res.Folders.Sent != "Sent" || res.Folders.Drafts != "Drafts" || res.Folders.Trash != "Bin" || res.SendRefused != "" || !trace.OK {
		t.Errorf("result = %+v, trace %+v", res, trace)
	}
	if !slices.Equal(res.Unannounced, []string{"archive", "junk"}) || slices.Contains(res.All, "Shared") {
		t.Errorf("unannounced = %v, all %v: archive and junk asked, Shared not a folder", res.Unannounced, res.All)
	}
	var names []string
	for _, s := range trace.Steps {
		names = append(names, string(s.Name))
	}
	if !slices.Equal(names, []string{"IMAP", "folders", "SMTP"}) {
		t.Errorf("steps = %v", names)
	}
}

// A server that announces no roles: they are offered by the folders' names,
// and all of them are asked (s-mailbox-24).
func TestCheckOffersRolesByName(t *testing.T) {
	ts := newTestServers(t, map[string][]imap.MailboxAttr{"Отправленные": nil, "Spam": nil}, false)
	res, _, err := ts.prober.Check(context.Background(), typed)
	if err != nil {
		t.Fatal(err)
	}
	if res.Folders.Sent != "Отправленные" || res.Folders.Junk != "Spam" || len(res.Unannounced) != 5 {
		t.Errorf("result = %+v", res)
	}
}

// IMAP refusing the password is the refusal: no mailbox, the step in the
// trace. SMTP refusing it is not: reading works, and the result says why
// sending does not (s-mailbox-25).
func TestCheckRefusals(t *testing.T) {
	ts := newTestServers(t, nil, true)
	wrong := typed
	wrong.Password = "wrong"
	if _, trace, err := ts.prober.Check(context.Background(), wrong); !errors.Is(err, ErrRefused) || trace.OK {
		t.Errorf("a wrong password: %v %+v", err, trace)
	}
	ts.smtpAuth = false
	res, trace, err := ts.prober.Check(context.Background(), typed)
	if err != nil || res == nil || !strings.Contains(res.SendRefused, "535") || res.CanSend() || !trace.OK {
		t.Errorf("SMTP refused: %v %+v %+v", err, res, trace)
	}
}

// Microsoft closed signing in by password: nothing is asked of its servers,
// and the refusal says so (M-3, s-mailbox-23).
func TestCheckMicrosoft(t *testing.T) {
	p := &Prober{Timeout: time.Second,
		SRV:   func(context.Context, string, string) ([]*net.SRV, error) { t.Error("SRV asked"); return nil, nil },
		Fetch: func(context.Context, string) ([]byte, error) { t.Error("autoconfig asked"); return nil, nil },
	}
	if _, _, err := p.Check(context.Background(), Input{Address: "anna@outlook.com", Password: "x"}); !errors.Is(err, ErrClosed) {
		t.Errorf("outlook.com: %v", err)
	}
}
