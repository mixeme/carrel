package mail_test

import (
	"context"
	"mime"
	"net/mail"
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
	if from.Address != "noreply@carrel.example" || from.Name != "Mix via Carrel" || msg.Header.Get("Reply-To") != "mix@example.org" {
		t.Errorf("From %q Reply-To %q", msg.Header.Get("From"), msg.Header.Get("Reply-To"))
	}
	subject, _ := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if subject != "Встреча в пятницу" || !strings.Contains(got[0].Data, `filename=event.ics`) {
		t.Errorf("subject %q, data:\n%s", subject, got[0].Data)
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
