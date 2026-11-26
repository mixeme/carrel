package mailbox

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/sealed"
)

// Prober finds and checks the servers of a mailbox. Its seams — the SRV
// lookup and the fetch of an autoconfig file — are the network's by default
// and a test's in tests.
type Prober struct {
	Guard *dav.Guard
	// SRV looks up the records of a service of a domain (RFC 6186): the
	// system's resolver by default.
	SRV func(ctx context.Context, service, domain string) ([]*net.SRV, error)
	// Fetch reads an autoconfig file: a GET through the guard by default.
	Fetch func(ctx context.Context, address string) ([]byte, error)
	// Timeout bounds each step of the network.
	Timeout time.Duration
	// Dial opens a connection to a mail server: the guard's DialMail by
	// default (S-5.1). A test points the allowed ports at its own servers.
	Dial func(ctx context.Context, host string, port int, implicitTLS bool, pin string) (net.Conn, error)
}

// NewProber is a prober on the network, through the guard.
func NewProber(g *dav.Guard) *Prober {
	p := &Prober{Guard: g, Timeout: 15 * time.Second, Dial: g.DialMail}
	p.SRV = func(ctx context.Context, service, domain string) ([]*net.SRV, error) {
		_, recs, err := net.DefaultResolver.LookupSRV(ctx, service, "tcp", domain)
		return recs, err
	}
	p.Fetch = func(ctx context.Context, address string) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
		if err != nil {
			return nil, err
		}
		resp, err := g.HTTPClient().Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%s", resp.Status)
		}
		return io.ReadAll(io.LimitReader(resp.Body, maxAutoconfig))
	}
	return p
}

// maxAutoconfig bounds an autoconfig file: a few kilobytes in the wild.
const maxAutoconfig = 256 << 10

// Found are the servers discovery offers, in the order they are tried: the
// SRV records first, then the autoconfig file, then the guesses.
type Found struct {
	IMAP []sealed.MailServer
	SMTP []sealed.MailServer
	// Login is what the autoconfig file names as the login: the whole
	// address or its local part. Empty — the whole address.
	Login string
}

// Step is how discovery writes each of its steps into the trace: its name,
// what was asked and what came back, and whether it found anything.
type Step func(name, detail string, ok bool)

// Discover finds the servers of an address, by the order of M-3: SRV
// records of its domain (RFC 6186, RFC 8314), the autoconfig file on its
// domain, the guesses imap., smtp., mail. of it. Hosts and ports typed by
// hand are the fourth way and are not discovered: Check takes them as they
// are. A step that finds nothing is not a refusal; it is written and the
// next one goes.
func (p *Prober) Discover(ctx context.Context, address string, step Step) Found {
	var f Found
	domain := Domain(address)
	if domain == "" {
		step("address", "not an address: no domain after @", false)
		return f
	}

	srvs := []struct {
		service, security string
		smtp              bool
	}{
		{"imaps", sealed.MailTLS, false},
		{"imap", sealed.MailSTARTTLS, false},
		{"submissions", sealed.MailTLS, true},
		{"submission", sealed.MailSTARTTLS, true},
	}
	var said []string
	for _, s := range srvs {
		ctx, cancel := context.WithTimeout(ctx, p.Timeout)
		recs, err := p.SRV(ctx, s.service, domain)
		cancel()
		if err != nil || len(recs) == 0 {
			continue
		}
		for _, r := range recs {
			host := strings.TrimSuffix(r.Target, ".")
			if host == "" { // «.»: the service is not offered (RFC 2782)
				continue
			}
			srv := sealed.MailServer{Host: host, Port: int(r.Port), Security: s.security}
			if s.smtp {
				f.SMTP = appendServer(f.SMTP, srv)
			} else {
				f.IMAP = appendServer(f.IMAP, srv)
			}
			said = append(said, fmt.Sprintf("_%s._tcp → %s:%d", s.service, host, r.Port))
		}
	}
	if len(said) > 0 {
		step("SRV", strings.Join(said, "; "), true)
	} else {
		step("SRV", "_imaps, _imap, _submissions, _submission of "+domain+": none", false)
	}

	for _, at := range []string{
		"https://autoconfig." + domain + "/mail/config-v1.1.xml?emailaddress=" + url.QueryEscape(address),
		"https://" + domain + "/.well-known/autoconfig/mail/config-v1.1.xml",
	} {
		ctx, cancel := context.WithTimeout(ctx, p.Timeout)
		body, err := p.Fetch(ctx, at)
		cancel()
		shown := strings.SplitN(at, "?", 2)[0]
		if err != nil {
			step("autoconfig", "GET "+shown+" → "+short(err), false)
			continue
		}
		cfg, err := parseAutoconfig(body, address)
		if err != nil || len(cfg.IMAP) == 0 && len(cfg.SMTP) == 0 {
			step("autoconfig", "GET "+shown+" → no IMAP or SMTP server in it", false)
			continue
		}
		for _, s := range cfg.IMAP {
			f.IMAP = appendServer(f.IMAP, s)
		}
		for _, s := range cfg.SMTP {
			f.SMTP = appendServer(f.SMTP, s)
		}
		if f.Login == "" {
			f.Login = cfg.Login
		}
		step("autoconfig", "GET "+shown+" → "+serversSaid(cfg), true)
		break
	}

	// The guesses go last and only where nothing better was found: each is
	// a host of the address's own domain.
	if len(f.IMAP) == 0 {
		for _, h := range []string{"imap." + domain, "mail." + domain} {
			f.IMAP = appendServer(f.IMAP, sealed.MailServer{Host: h, Port: 993, Security: sealed.MailTLS})
		}
	}
	if len(f.SMTP) == 0 {
		for _, h := range []string{"smtp." + domain, "mail." + domain} {
			f.SMTP = appendServer(f.SMTP, sealed.MailServer{Host: h, Port: 465, Security: sealed.MailTLS})
			f.SMTP = appendServer(f.SMTP, sealed.MailServer{Host: h, Port: 587, Security: sealed.MailSTARTTLS})
		}
	}
	return f
}

// appendServer adds s unless it is there already, or its port is none of a
// mail server's the guard lets through (25 above all, S-5.1): offered by a
// record or a file, it would only be refused at the dial.
func appendServer(list []sealed.MailServer, s sealed.MailServer) []sealed.MailServer {
	s.Host = strings.ToLower(s.Host)
	if !dav.MailPortAllowed(s.Port) || slices.Contains(list, s) {
		return list
	}
	return append(list, s)
}

// serversSaid is the trace's words of what an autoconfig file named.
func serversSaid(f Found) string {
	var out []string
	for _, s := range f.IMAP {
		out = append(out, "IMAP "+serverSaid(s))
	}
	for _, s := range f.SMTP {
		out = append(out, "SMTP "+serverSaid(s))
	}
	return strings.Join(out, ", ")
}

func serverSaid(s sealed.MailServer) string {
	return s.Host + ":" + strconv.Itoa(s.Port) + " " + s.Security
}

// short is an error as the trace says it: its last part, the network's.
func short(err error) string {
	var nerr net.Error
	if errors.As(err, &nerr) && nerr.Timeout() {
		return "no answer"
	}
	s := err.Error()
	if i := strings.LastIndex(s, ": "); i >= 0 && len(s)-i > 6 {
		s = s[i+2:]
	}
	return s
}

// autoconfig is the file of Thunderbird's autoconfig (config-v1.1.xml):
// only what Carrel takes from it.
type autoconfig struct {
	Providers []struct {
		Incoming []acServer `xml:"incomingServer"`
		Outgoing []acServer `xml:"outgoingServer"`
	} `xml:"emailProvider"`
}

type acServer struct {
	Type     string `xml:"type,attr"`
	Hostname string `xml:"hostname"`
	Port     int    `xml:"port"`
	Socket   string `xml:"socketType"`
	Username string `xml:"username"`
}

// parseAutoconfig reads the IMAP and SMTP servers of an autoconfig file, the
// placeholders of the address filled in. POP3 is not taken: Carrel reads
// over IMAP only (M-2).
func parseAutoconfig(body []byte, address string) (Found, error) {
	var ac autoconfig
	if err := xml.Unmarshal(body, &ac); err != nil {
		return Found{}, err
	}
	local, domain := address, Domain(address)
	if at := strings.LastIndex(address, "@"); at >= 0 {
		local = address[:at]
	}
	fill := func(s string) string {
		r := strings.NewReplacer("%EMAILADDRESS%", address, "%EMAILLOCALPART%", local, "%EMAILDOMAIN%", domain)
		return strings.TrimSpace(r.Replace(s))
	}
	security := map[string]string{"SSL": sealed.MailTLS, "TLS": sealed.MailTLS, "STARTTLS": sealed.MailSTARTTLS, "PLAIN": sealed.MailPlain}
	var f Found
	for _, p := range ac.Providers {
		for _, s := range p.Incoming {
			sec, ok := security[strings.ToUpper(strings.TrimSpace(s.Socket))]
			if strings.ToLower(s.Type) != "imap" || !ok || fill(s.Hostname) == "" {
				continue
			}
			f.IMAP = appendServer(f.IMAP, sealed.MailServer{Host: fill(s.Hostname), Port: s.Port, Security: sec})
			if f.Login == "" {
				f.Login = fill(s.Username)
			}
		}
		for _, s := range p.Outgoing {
			sec, ok := security[strings.ToUpper(strings.TrimSpace(s.Socket))]
			if strings.ToLower(s.Type) != "smtp" || !ok || fill(s.Hostname) == "" {
				continue
			}
			f.SMTP = appendServer(f.SMTP, sealed.MailServer{Host: fill(s.Hostname), Port: s.Port, Security: sec})
		}
	}
	return f, nil
}
