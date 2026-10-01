//go:build integration

package integration

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"mime"
	"net"
	netmail "net/mail"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	cmail "carrel.dev/carrel/server/internal/mail"
)

// S-26 live: a letter built by internal/mail goes through the SelfPost relay
// and is found in the test mailbox — delivered, not only sent. The From is
// the relay's, a person's name "(via Carrel)", and the Reply-To arrives as
// given. The letter is deleted from the mailbox after.
func TestSelfPostRelayDelivers(t *testing.T) {
	liveCredEnv()
	host, user, pass := os.Getenv("CARREL_SMTP_HOST"), os.Getenv("CARREL_SMTP_USER"), os.Getenv("CARREL_SMTP_PASS")
	imapHost, box, boxPass := os.Getenv("CARREL_IMAP_HOST"), os.Getenv("CARREL_IMAP_USER"), os.Getenv("CARREL_IMAP_PASS")
	if host == "" || user == "" || pass == "" || imapHost == "" || box == "" || boxPass == "" {
		t.Skip("CARREL_SMTP_* and CARREL_IMAP_* required")
	}
	port, _ := strconv.Atoi(os.Getenv("CARREL_SMTP_PORT"))
	security := cmail.StartTLS
	if port == 465 {
		security = cmail.TLS
	}
	// The relay sends only from an address the login owns, and which one
	// that is only its owner knows: the relay section of dev/credentials.txt
	// names it as "From:" (the relay refused both the login at its host and
	// the mailbox, 19.09.2026).
	from := os.Getenv("CARREL_SMTP_FROM")
	if from == "" {
		t.Skip("CARREL_SMTP_FROM required: the address the relay lets its login send from")
	}
	relay := cmail.Relay{Host: host, Port: port, Security: security, Username: user, Password: pass, From: from, FromName: "Carrel"}

	tag := make([]byte, 6)
	_, _ = rand.Read(tag)
	token := "carrel-live-" + hex.EncodeToString(tag)
	data, err := cmail.Build(relay, cmail.Message{To: []string{box}, ReplyTo: "reply@example.org", Sender: "Live check",
		Subject: "Проверка " + token, Text: "Письмо живой проверки S-26.\n" + token,
		Attachments: []cmail.Attachment{{Name: "check.txt", Type: "text/plain", Data: []byte(token)}}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// The box is opened before the letter goes: what earlier runs left in it
	// is swept first.
	im, err := dialIMAP(imapHost, box, boxPass)
	if err != nil {
		t.Fatalf("imap: %v", err)
	}
	sweepLiveLetters(t, im, from)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	reply, err := cmail.Send(ctx, relay, []string{box}, data)
	if err != nil {
		t.Fatalf("the relay did not take the letter (From %s): %v", from, err)
	}
	t.Logf("relay: %s", reply)

	// A letter from a sender the receiving server has not seen is deferred
	// at first (a soft reject, greylisting), so the wait is long — and one
	// connection does the whole of it. Signing in for every look, which is
	// what this did until 22.09.2026, is ninety-odd logins in six minutes:
	// to a mail server that is a password being guessed, and the run of
	// that day ended with «imap: login: EOF» from a host that had had
	// enough. The connection is only rebuilt when it actually breaks.
	//
	// How long: on 01.10.2026 the box's server (mc-ent.mixfed.ru) answered
	// «451 4.7.1 Greylisted» eight seconds after the relay took the letter,
	// and the relay's Postfix delivered it on its first retry 8 min 27 s
	// after — just past the eight minutes this waited then. Postfix retries
	// a deferred letter after minimal_backoff_time (300 s) at the next queue
	// run (queue_run_delay, 300 s), so the first retry comes five to ten
	// minutes on; fifteen covers it. The «write … i/o timeout» of that run,
	// and of one three days before, was this test's own and not the host's:
	// the connection got one two-minute deadline when it was opened, and
	// every command after that failed (a probe of 01.10.2026 met it on a
	// NOOP 120 s in, the host answering in 35–70 ms). The deadline is now
	// set for each command (imapConn.cmd), and the box is looked at every
	// thirty seconds instead of ten.
	var uid string
	defer func() {
		if im != nil {
			im.close()
		}
	}()
	wait := 15 * time.Minute
	if end, ok := t.Deadline(); ok && time.Until(end)-time.Minute < wait {
		wait = time.Until(end) - time.Minute
		t.Logf("go test's -timeout leaves %v for the letter; greylisting may want up to ten minutes", wait.Round(time.Second))
	}
	deadline := time.Now().Add(wait)
	for uid == "" && time.Now().Before(deadline) {
		if im == nil {
			if im, err = dialIMAP(imapHost, box, boxPass); err != nil {
				t.Fatalf("imap: %v", err)
			}
		}
		lines, err := im.cmd("UID SEARCH SUBJECT " + quoteIMAP(token))
		if err != nil {
			// The server closed a connection that was only watching. Take
			// that as its right, and come back on a new one.
			t.Logf("the mailbox closed the connection (%v); opening another", err)
			im.close()
			im = nil
			time.Sleep(30 * time.Second)
			continue
		}
		if uids := searched(lines); len(uids) > 0 {
			uid = uids[len(uids)-1]
		} else {
			time.Sleep(30 * time.Second)
		}
	}
	if uid == "" {
		t.Fatalf("the letter did not arrive in %v (relay said %q); the box's server greylists a new sender, and its first "+
			"delivery took 8 min 27 s on 01.10.2026 — the next run sweeps the letter when it comes", wait.Round(time.Second), reply)
	}
	header, err := im.cmd("UID FETCH " + uid + " BODY.PEEK[HEADER.FIELDS (FROM REPLY-TO)]")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(header, "\n")
	if !strings.Contains(joined, "reply@example.org") || !strings.Contains(joined, from) || !strings.Contains(joined, "via Carrel") {
		t.Errorf("headers as delivered:\n%s", joined)
	}
	if err := im.remove([]string{uid}); err != nil {
		t.Errorf("the letter %s stays in the mailbox: %v", token, err)
	}
}

// liveLetter is the subject this test gives its letters; nothing else in
// the mailbox is ever touched.
var liveLetter = regexp.MustCompile(`^Проверка carrel-live-[0-9a-f]{12}$`)

// sweepLiveLetters takes out the letters of earlier runs: those whose
// subject is this test's own and whose sender is the relay's address the
// test sends from. Until 01.10.2026 none was ever removed — a letter that
// came after the wait was over stayed, and so did those of the runs of
// 19.09.2026 — and the box is a person's to read.
func sweepLiveLetters(t *testing.T, im *imapConn, from string) {
	t.Helper()
	lines, err := im.cmd("UID SEARCH FROM " + quoteIMAP(from) + " SUBJECT " + quoteIMAP("carrel-live-"))
	if err != nil {
		t.Logf("sweep: search: %v", err)
		return
	}
	var ours, said []string
	for _, uid := range searched(lines) {
		header, err := im.cmd("UID FETCH " + uid + " BODY.PEEK[HEADER.FIELDS (FROM SUBJECT DATE)]")
		if err != nil {
			t.Logf("sweep: fetch %s: %v", uid, err)
			return
		}
		msg, err := netmail.ReadMessage(strings.NewReader(fetchedHeader(header) + "\r\n"))
		if err != nil {
			continue
		}
		subject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
		sender, _ := msg.Header.AddressList("From")
		if err != nil || len(sender) != 1 || !strings.EqualFold(sender[0].Address, from) || !liveLetter.MatchString(subject) {
			continue
		}
		ours = append(ours, uid)
		said = append(said, fmt.Sprintf("%q of %s", subject, msg.Header.Get("Date")))
	}
	if len(ours) == 0 {
		return
	}
	if err := im.remove(ours); err != nil {
		t.Errorf("sweep: %d letter(s) of earlier runs stay: %v", len(ours), err)
		return
	}
	t.Logf("swept %d letter(s) of earlier runs: %s", len(ours), strings.Join(said, ", "))
}

// searched is the UIDs of a UID SEARCH answer.
func searched(lines []string) []string {
	for _, l := range lines {
		if f := strings.Fields(l); len(f) >= 2 && f[0] == "*" && f[1] == "SEARCH" {
			return f[2:]
		}
	}
	return nil
}

// fetchedHeader is the header block of a UID FETCH answer: the lines of the
// literal, between the FETCH line and the closing parenthesis.
func fetchedHeader(lines []string) string {
	var out []string
	in := false
	for _, l := range lines {
		switch {
		case !in && strings.Contains(l, " FETCH "):
			in = true
		case in && strings.TrimSpace(l) == ")":
			in = false
		case in && l != "":
			out = append(out, l)
		}
	}
	return strings.Join(out, "\r\n") + "\r\n"
}

// imapConn is as much IMAP as the check needs: one tagged command at a time,
// its lines until the tagged answer.
type imapConn struct {
	c   net.Conn
	r   *bufio.Reader
	seq int
}

func dialIMAP(host, user, pass string) (*imapConn, error) {
	c, err := tls.DialWithDialer(&net.Dialer{Timeout: 15 * time.Second}, "tcp", net.JoinHostPort(host, "993"), &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return nil, err
	}
	_ = c.SetDeadline(time.Now().Add(time.Minute))
	im := &imapConn{c: c, r: bufio.NewReader(c)}
	if _, err := im.r.ReadString('\n'); err != nil {
		c.Close()
		return nil, err
	}
	if _, err := im.cmd("LOGIN " + quoteIMAP(user) + " " + quoteIMAP(pass)); err != nil {
		c.Close()
		return nil, fmt.Errorf("login: %w", err)
	}
	if _, err := im.cmd("SELECT INBOX"); err != nil {
		c.Close()
		return nil, err
	}
	return im, nil
}

func (im *imapConn) cmd(line string) ([]string, error) {
	im.seq++
	tag := fmt.Sprintf("c%d", im.seq)
	// A deadline for this command, not for the connection: the one set
	// when it was opened failed every write two minutes on.
	_ = im.c.SetDeadline(time.Now().Add(time.Minute))
	if _, err := fmt.Fprintf(im.c, "%s %s\r\n", tag, line); err != nil {
		return nil, err
	}
	var out []string
	for {
		l, err := im.r.ReadString('\n')
		if err != nil {
			return out, err
		}
		l = strings.TrimRight(l, "\r\n")
		if strings.HasPrefix(l, tag+" ") {
			if !strings.HasPrefix(l, tag+" OK") {
				return out, fmt.Errorf("%s", strings.TrimPrefix(l, tag+" "))
			}
			return out, nil
		}
		out = append(out, l)
	}
}

// remove deletes the letters by UID and nothing else: UID EXPUNGE (RFC
// 4315) leaves alone whatever other letter of the box carries \Deleted.
func (im *imapConn) remove(uids []string) error {
	set := strings.Join(uids, ",")
	if _, err := im.cmd("UID STORE " + set + " +FLAGS.SILENT (\\Deleted)"); err != nil {
		return err
	}
	_, err := im.cmd("UID EXPUNGE " + set)
	return err
}

func (im *imapConn) close() {
	_, _ = im.cmd("LOGOUT")
	im.c.Close()
}

func quoteIMAP(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
