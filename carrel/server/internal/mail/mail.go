// Package mail builds and sends Carrel's letters (S-26) with net/smtp: the
// instance's own invitations and links, and — when the instance lets them —
// letters people send. A letter always goes from the instance's address; a
// person may give it their name and, when their address is confirmed, a
// Reply-To, never their login as the sender.
//
// The package does not import internal/web and fills no view page.
package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// Security of the connection to the relay.
const (
	StartTLS = "starttls" // port 587: plain first, then STARTTLS, required
	TLS      = "tls"      // port 465: TLS from the first byte
	None     = "none"     // a relay on the same host or network; no password leaves in the clear
)

// The From of a person's letter (S-26.1): the administrator's choice.
const (
	SenderService = "noreply" // "Ann (via Carrel) <noreply@domain>"
	SenderLogin   = "login"   // "Ann <ann@domain>", on the instance's domain
)

// Relay is the instance's outgoing server and its sender (store.SMTP).
type Relay struct {
	Host     string
	Port     int
	Security string
	Username string
	Password string
	From     string // the instance's address: the envelope and the From
	FromName string
}

// Configured reports whether a letter can be sent at all.
func (r Relay) Configured() bool {
	_, err := mail.ParseAddress(r.From)
	return r.Host != "" && r.Port > 0 && err == nil
}

// Attachment is one file of a letter, in its exchange format.
type Attachment struct {
	Name string
	Type string
	Data []byte
}

// Message is a letter before it is written.
type Message struct {
	To      []string
	ReplyTo string // a confirmed address, or empty
	// Sender is the person's name for the From line: "Ann (via Carrel)".
	// Empty for the instance's own letters.
	Sender string
	// FromLogin, when set, makes the From the person's own address on the
	// instance's domain, "Ann <login@domain>". The envelope stays the
	// instance's address either way (Send), so SPF is not touched.
	FromLogin   string
	Subject     string
	Text        string
	Attachments []Attachment
}

// ErrHeader is a header value that would break the letter's headers.
var ErrHeader = errors.New("mail: a header holds a line break")

func clean(s string) (string, error) {
	if strings.ContainsAny(s, "\r\n") {
		return "", ErrHeader
	}
	return strings.TrimSpace(s), nil
}

// Addresses checks every recipient: one address each, no display name
// games, nothing that would add a header.
func Addresses(list []string) ([]string, error) {
	var out []string
	for _, a := range list {
		a, err := clean(a)
		if err != nil {
			return nil, err
		}
		if a == "" {
			continue
		}
		parsed, err := mail.ParseAddress(a)
		if err != nil {
			return nil, fmt.Errorf("mail: %q is not an address", a)
		}
		out = append(out, parsed.Address)
	}
	return out, nil
}

// Build writes the letter: headers without anything internal — no paths, no
// collections, no server of the sender's — a plain-text body, and the
// attachments as parts of their own.
func Build(relay Relay, m Message, now time.Time) ([]byte, error) {
	to, err := Addresses(m.To)
	if err != nil {
		return nil, err
	}
	if len(to) == 0 {
		return nil, errors.New("mail: no recipient")
	}
	subject, err := clean(m.Subject)
	if err != nil {
		return nil, err
	}
	name, err := clean(relay.FromName)
	if err != nil {
		return nil, err
	}
	if sender, err := clean(m.Sender); err != nil {
		return nil, err
	} else if sender != "" {
		if name == "" {
			name = "Carrel"
		}
		name = sender + " (via " + name + ")"
	}
	from := mail.Address{Name: name, Address: relay.From}
	domain := relay.From[strings.LastIndex(relay.From, "@")+1:]
	if m.FromLogin != "" {
		own, err := mail.ParseAddress(m.FromLogin + "@" + domain)
		if err != nil || strings.ContainsAny(m.FromLogin, "@<>\r\n") {
			return nil, fmt.Errorf("mail: the login %q is not an address on %s", m.FromLogin, domain)
		}
		from = mail.Address{Name: strings.TrimSpace(m.Sender), Address: own.Address}
	}
	id := make([]byte, 12)
	if _, err := rand.Read(id); err != nil {
		return nil, err
	}

	var b bytes.Buffer
	header := func(k, v string) { b.WriteString(k + ": " + v + "\r\n") }
	header("From", from.String())
	header("To", strings.Join(to, ", "))
	if m.ReplyTo != "" {
		rt, err := Addresses([]string{m.ReplyTo})
		if err != nil || len(rt) != 1 {
			return nil, errors.New("mail: the reply address is not an address")
		}
		header("Reply-To", rt[0])
	}
	header("Subject", mime.QEncoding.Encode("utf-8", subject))
	header("Date", now.Format(time.RFC1123Z))
	header("Message-ID", "<"+hex.EncodeToString(id)+"@"+domain+">")
	header("MIME-Version", "1.0")

	body := func(w *bytes.Buffer) {
		qp := quotedprintable.NewWriter(w)
		_, _ = qp.Write([]byte(strings.ReplaceAll(m.Text, "\n", "\r\n")))
		_ = qp.Close()
	}
	if len(m.Attachments) == 0 {
		header("Content-Type", "text/plain; charset=utf-8")
		header("Content-Transfer-Encoding", "quoted-printable")
		b.WriteString("\r\n")
		body(&b)
		return b.Bytes(), nil
	}
	boundary := "carrel-" + hex.EncodeToString(id)
	header("Content-Type", `multipart/mixed; boundary="`+boundary+`"`)
	b.WriteString("\r\n--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n")
	body(&b)
	for _, a := range m.Attachments {
		n, err := clean(a.Name)
		if err != nil {
			return nil, err
		}
		t := a.Type
		if t == "" {
			t = "application/octet-stream"
		}
		b.WriteString("\r\n--" + boundary + "\r\n")
		b.WriteString("Content-Type: " + mime.FormatMediaType(t, map[string]string{"name": n}) + "\r\n")
		b.WriteString("Content-Disposition: " + mime.FormatMediaType("attachment", map[string]string{"filename": n}) + "\r\n")
		b.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
		enc := base64.StdEncoding.EncodeToString(a.Data)
		for len(enc) > 76 {
			b.WriteString(enc[:76] + "\r\n")
			enc = enc[76:]
		}
		b.WriteString(enc + "\r\n")
	}
	b.WriteString("\r\n--" + boundary + "--\r\n")
	return b.Bytes(), nil
}

// Send hands the letter to the relay and returns what it last said. STARTTLS
// is required when asked for: a relay that does not offer it is an error,
// never a silent fall back to the clear. A password is sent only over TLS or
// to a relay on this host (net/smtp refuses otherwise).
func Send(ctx context.Context, relay Relay, to []string, data []byte) (string, error) {
	if !relay.Configured() {
		return "", errors.New("mail: SMTP is not configured")
	}
	addr := net.JoinHostPort(relay.Host, strconv.Itoa(relay.Port))
	d := net.Dialer{Timeout: 15 * time.Second}
	var conn net.Conn
	var err error
	tlsConf := &tls.Config{ServerName: relay.Host, MinVersion: tls.VersionTLS12}
	if relay.Security == TLS {
		conn, err = tls.DialWithDialer(&d, "tcp", addr, tlsConf)
	} else {
		conn, err = d.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return "", err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(60 * time.Second))
	}
	c, err := smtp.NewClient(conn, relay.Host)
	if err != nil {
		conn.Close()
		return "", err
	}
	defer c.Close()
	if err := c.Hello(relay.From[strings.LastIndex(relay.From, "@")+1:]); err != nil {
		return "", err
	}
	if relay.Security == StartTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return "", errors.New("mail: the relay does not offer STARTTLS")
		}
		if err := c.StartTLS(tlsConf); err != nil {
			return "", err
		}
	}
	if relay.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", relay.Username, relay.Password, relay.Host)); err != nil {
			return "", err
		}
	}
	if err := c.Mail(relay.From); err != nil {
		return "", err
	}
	for _, rcpt := range to {
		if err := c.Rcpt(rcpt); err != nil {
			return "", err
		}
	}
	w, err := c.Data()
	if err != nil {
		return "", err
	}
	if _, err := w.Write(data); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	_ = c.Quit()
	return fmt.Sprintf("250 accepted for %d recipient(s)", len(to)), nil
}
