//go:build integration

package integration

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"os"
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	reply, err := cmail.Send(ctx, relay, []string{box}, data)
	if err != nil {
		t.Fatalf("the relay did not take the letter (From %s): %v", from, err)
	}
	t.Logf("relay: %s", reply)

	// Each look is a connection of its own: a server may close one that
	// only polls. A letter from a sender the receiving server has not seen
	// is deferred at first (a soft reject, greylisting), so the wait is long.
	var uid string
	var im *imapConn
	deadline := time.Now().Add(8 * time.Minute) // inside go test's default 10 minutes
	for uid == "" && time.Now().Before(deadline) {
		if im != nil {
			im.close()
		}
		if im, err = dialIMAP(imapHost, box, boxPass); err != nil {
			t.Fatalf("imap: %v", err)
		}
		lines, err := im.cmd("UID SEARCH SUBJECT " + quoteIMAP(token))
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		for _, l := range lines {
			if f := strings.Fields(l); len(f) > 2 && f[0] == "*" && f[1] == "SEARCH" {
				uid = f[len(f)-1]
			}
		}
		if uid == "" {
			time.Sleep(5 * time.Second)
		}
	}
	defer im.close()
	if uid == "" {
		t.Fatalf("the letter did not arrive in eight minutes (relay said %q)", reply)
	}
	header, err := im.cmd("UID FETCH " + uid + " BODY.PEEK[HEADER.FIELDS (FROM REPLY-TO)]")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(header, "\n")
	if !strings.Contains(joined, "reply@example.org") || !strings.Contains(joined, from) || !strings.Contains(joined, "via Carrel") {
		t.Errorf("headers as delivered:\n%s", joined)
	}
	_, _ = im.cmd("UID STORE " + uid + " +FLAGS (\\Deleted)")
	_, _ = im.cmd("EXPUNGE")
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
	_ = c.SetDeadline(time.Now().Add(2 * time.Minute))
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

func (im *imapConn) close() {
	_, _ = im.cmd("LOGOUT")
	im.c.Close()
}

func quoteIMAP(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
