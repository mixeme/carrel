package mail_test

import (
	"context"
	"mime"
	"net"
	"net/mail"
	"strconv"
	"strings"
	"testing"
	"time"

	cmail "carrel.dev/carrel/server/internal/mail"
	"carrel.dev/carrel/server/internal/mail/mailtest"
)

func relay(s *mailtest.Server) cmail.Relay {
	return cmail.Relay{Host: s.Host, Port: s.Port, Security: cmail.None, From: "noreply@carrel.example", FromName: "Carrel"}
}

// S-26.1: the letter goes from the instance's address; a person gives it a
// name and, confirmed, a Reply-To; attachments are parts of their own.
func TestBuildAndSend(t *testing.T) {
	s := mailtest.New(t)
	r := relay(s)
	data, err := cmail.Build(r, cmail.Message{
		To: []string{"ann@example.org"}, ReplyTo: "mix@example.org", Sender: "Mix",
		Subject: "Встреча в пятницу", Text: "Привет!\nСм. вложение.",
		Attachments: []cmail.Attachment{{Name: "event.ics", Type: "text/calendar", Data: []byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n")}},
	}, time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cmail.Send(context.Background(), r, []string{"ann@example.org"}, data); err != nil {
		t.Fatal(err)
	}
	got := s.Letters()
	if len(got) != 1 || got[0].From != "noreply@carrel.example" || got[0].To[0] != "ann@example.org" {
		t.Fatalf("letters = %+v", got)
	}
	msg, err := mail.ReadMessage(strings.NewReader(got[0].Data))
	if err != nil {
		t.Fatal(err)
	}
	from, _ := mail.ParseAddress(msg.Header.Get("From"))
	if from.Address != "noreply@carrel.example" || from.Name != "Mix (via Carrel)" || msg.Header.Get("Reply-To") != "mix@example.org" {
		t.Errorf("From %q Reply-To %q", msg.Header.Get("From"), msg.Header.Get("Reply-To"))
	}
	subject, _ := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if subject != "Встреча в пятницу" || !strings.Contains(got[0].Data, `filename=event.ics`) {
		t.Errorf("subject %q, data:\n%s", subject, got[0].Data)
	}
}

// S-26.1: with the login form the From is the person's own address on the
// instance's domain, with their name alone; the envelope and the Reply-To
// do not change.
func TestFromLogin(t *testing.T) {
	s := mailtest.New(t)
	r := relay(s)
	data, err := cmail.Build(r, cmail.Message{To: []string{"ann@example.org"}, ReplyTo: "mix@example.org",
		Sender: "Михаил Енученко", FromLogin: "mix", Subject: "x", Text: "x"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	msg, _ := mail.ReadMessage(strings.NewReader(string(data)))
	from, err := mail.ParseAddress(msg.Header.Get("From"))
	if err != nil || from.Address != "mix@carrel.example" || from.Name != "Михаил Енученко" || msg.Header.Get("Reply-To") != "mix@example.org" {
		t.Errorf("From %q Reply-To %q", msg.Header.Get("From"), msg.Header.Get("Reply-To"))
	}
	if _, err := cmail.Send(context.Background(), r, []string{"ann@example.org"}, data); err != nil {
		t.Fatal(err)
	}
	if got := s.Letters(); got[0].From != "noreply@carrel.example" {
		t.Errorf("envelope = %q", got[0].From)
	}
	for _, bad := range []string{"a b", "x@evil.example", "x>"} {
		if _, err := cmail.Build(r, cmail.Message{To: []string{"a@example.org"}, FromLogin: bad}, time.Now()); err == nil {
			t.Errorf("login %q made an address", bad)
		}
	}
}

// A line break in any header is refused before anything is sent: the one
// way a form could add a recipient or a header.
func TestHeadersCannotBeInjected(t *testing.T) {
	r := cmail.Relay{Host: "x", Port: 25, From: "noreply@carrel.example"}
	for name, m := range map[string]cmail.Message{
		"subject":   {To: []string{"a@example.org"}, Subject: "hi\r\nBcc: evil@example.org"},
		"recipient": {To: []string{"a@example.org\r\nBcc: evil@example.org"}},
		"sender":    {To: []string{"a@example.org"}, Sender: "Mix\nBcc: evil@example.org"},
	} {
		if _, err := cmail.Build(r, m, time.Now()); err == nil {
			t.Errorf("%s: a line break went through", name)
		}
	}
	if _, err := cmail.Addresses([]string{"not an address"}); err == nil {
		t.Error("a non-address was taken")
	}
}

// The relay's refusals come back as errors: a password it wants, a mailbox
// it does not know; STARTTLS asked for and not offered is an error, never a
// fall back to the clear.
func TestRelayRefusals(t *testing.T) {
	s := mailtest.New(t)
	s.NeedAuth, s.Refuse = "carrel", "nobody@example.org"
	r := relay(s)
	data, _ := cmail.Build(r, cmail.Message{To: []string{"a@example.org"}, Subject: "x", Text: "x"}, time.Now())
	if _, err := cmail.Send(context.Background(), r, []string{"a@example.org"}, data); err == nil {
		t.Error("sent without the password the relay asks for")
	}
	r.Username, r.Password = "carrel", "pw"
	if _, err := cmail.Send(context.Background(), r, []string{"nobody@example.org"}, data); err == nil {
		t.Error("a refused mailbox is not an error")
	}
	if _, err := cmail.Send(context.Background(), r, []string{"a@example.org"}, data); err != nil {
		t.Errorf("with the password: %v", err)
	}
	r.Security = cmail.StartTLS
	if _, err := cmail.Send(context.Background(), r, []string{"a@example.org"}, data); err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Errorf("STARTTLS not offered: %v", err)
	}
	if _, err := cmail.Send(context.Background(), cmail.Relay{}, nil, nil); err == nil {
		t.Error("an unconfigured relay sent")
	}
}

// A relay on port 465 that takes the connection and never answers the TLS
// handshake: Send gives up when the caller's context does, not after a
// dialer's own quarter of a minute. The page that sends a letter waits on it.
func TestSendOverTLSHonoursTheContext(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			defer c.Close() // held open, silent
		}
	}()
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	p, _ := strconv.Atoi(port)
	relay := cmail.Relay{Host: host, Port: p, Security: cmail.TLS, From: "carrel@example.org"}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := cmail.Send(ctx, relay, []string{"a@example.org"}, []byte("x")); err == nil {
		t.Fatal("a relay that never answers was taken as sent")
	}
	if took := time.Since(start); took > 3*time.Second {
		t.Errorf("Send took %v with a context of 300ms", took)
	}
}
