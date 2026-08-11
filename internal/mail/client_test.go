// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package mail

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/store"
)

// relay is a minimal SMTP server: enough of the conversation for the client to
// get a message through, and a way to make it refuse at a chosen step.
type relay struct {
	host string
	port int
	// rejectMailFrom, when set, is the reply sent instead of accepting the
	// sender.
	rejectMailFrom string

	mu       sync.Mutex
	body     strings.Builder
	authSeen bool
}

func startRelay(t *testing.T) *relay {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	host, portText, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split %q: %v", ln.Addr(), err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("port %q: %v", portText, err)
	}

	r := &relay{host: host, port: port}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go r.serve(conn)
		}
	}()
	return r
}

func (r *relay) serve(conn net.Conn) {
	defer conn.Close()
	in := bufio.NewReader(conn)
	say := func(format string, args ...any) {
		fmt.Fprintf(conn, format+"\r\n", args...)
	}

	say("220 fake ESMTP ready")
	for {
		line, err := in.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"):
			say("250-fake greets you\r\n250 AUTH PLAIN")
		case strings.HasPrefix(cmd, "HELO"):
			say("250 fake greets you")
		case strings.HasPrefix(cmd, "AUTH"):
			r.mu.Lock()
			r.authSeen = true
			r.mu.Unlock()
			say("235 authentication succeeded")
		case strings.HasPrefix(cmd, "MAIL FROM"):
			if r.rejectMailFrom != "" {
				say("%s", r.rejectMailFrom)
				continue
			}
			say("250 sender ok")
		case strings.HasPrefix(cmd, "RCPT TO"):
			say("250 recipient ok")
		case strings.HasPrefix(cmd, "DATA"):
			say("354 end with a dot")
			for {
				data, err := in.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimRight(data, "\r\n") == "." {
					break
				}
				r.mu.Lock()
				r.body.WriteString(data)
				r.mu.Unlock()
			}
			say("250 queued")
		case strings.HasPrefix(cmd, "QUIT"):
			say("221 closing")
			return
		default:
			say("500 unrecognised command")
		}
	}
}

func (r *relay) received() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.String()
}

func (r *relay) config() Config {
	return Config{
		Host:        r.host,
		Port:        r.port,
		TLS:         store.TLSNone,
		FromAddress: "carrel@example.org",
		FromName:    "Carrel",
	}
}

var testMessage = Message{
	To:      "admin@example.org",
	Subject: "Carrel test message",
	Text:    "If you are reading this, the relay works.",
}

func TestSendDeliversAndTranscribes(t *testing.T) {
	r := startRelay(t)

	res := Send(r.config(), testMessage)
	if !res.OK {
		t.Fatalf("send failed: %s", res.Diagnostic)
	}
	// The administrator is shown the whole conversation, not a verdict (§5.3).
	for _, step := range []string{"connecting to", "MAIL FROM", "RCPT TO", "message accepted"} {
		if !strings.Contains(res.Diagnostic, step) {
			t.Errorf("the transcript does not mention %q:\n%s", step, res.Diagnostic)
		}
	}

	got := r.received()
	for _, want := range []string{
		"To: " + testMessage.To,
		"Subject: " + testMessage.Subject,
		testMessage.Text,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the delivered message does not contain %q:\n%s", want, got)
		}
	}
}

// A relay that is not set up at all is reported as such rather than dialled.
func TestSendReportsAnUnconfiguredRelay(t *testing.T) {
	res := Send(Config{FromAddress: "carrel@example.org"}, testMessage)
	if res.OK {
		t.Fatal("sending succeeded with no relay configured")
	}
	if !strings.Contains(res.Diagnostic, "not configured") {
		t.Errorf("the diagnostic does not say the relay is unset:\n%s", res.Diagnostic)
	}

	res = Send(Config{Host: "127.0.0.1", Port: 25, FromAddress: "carrel@example.org"}, Message{Subject: "x"})
	if res.OK || !strings.Contains(res.Diagnostic, "recipient") {
		t.Errorf("an empty recipient was not reported:\n%s", res.Diagnostic)
	}
}

func TestSendReportsAnUnreachableRelay(t *testing.T) {
	// A port nobody is listening on: taken and released so it is free.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	_ = ln.Close()

	cfg := Config{Host: "127.0.0.1", Port: addr.Port, TLS: store.TLSNone, FromAddress: "carrel@example.org"}
	res := Send(cfg, testMessage)
	if res.OK {
		t.Fatal("sending succeeded against a closed port")
	}
	if !strings.Contains(res.Diagnostic, "connection failed") {
		t.Errorf("the diagnostic does not name the failure:\n%s", res.Diagnostic)
	}
	// The address that was tried is part of what makes the diagnostic useful.
	if !strings.Contains(res.Diagnostic, strconv.Itoa(addr.Port)) {
		t.Errorf("the diagnostic does not show the address tried:\n%s", res.Diagnostic)
	}
}

// When the server refuses, its own words are passed through: that is what the
// administrator needs to fix the settings (§21).
func TestSendPassesTheServersRefusalThrough(t *testing.T) {
	r := startRelay(t)
	r.rejectMailFrom = "550 5.7.1 sender denied by policy"

	res := Send(r.config(), testMessage)
	if res.OK {
		t.Fatal("sending succeeded although the server refused")
	}
	for _, want := range []string{"MAIL FROM failed", "550", "sender denied by policy"} {
		if !strings.Contains(res.Diagnostic, want) {
			t.Errorf("the diagnostic does not carry %q:\n%s", want, res.Diagnostic)
		}
	}
}

// The transcript goes on screen and into the administrator's clipboard, so the
// relay password may not appear in it (§24.4).
func TestSendKeepsTheRelayPasswordOutOfTheTranscript(t *testing.T) {
	r := startRelay(t)
	const password = "s3cret-relay-password"

	cfg := r.config()
	cfg.Username = "carrel"
	cfg.Password = password

	res := Send(cfg, testMessage)
	if !res.OK {
		t.Fatalf("send failed: %s", res.Diagnostic)
	}
	r.mu.Lock()
	authSeen := r.authSeen
	r.mu.Unlock()
	if !authSeen {
		t.Error("the client did not authenticate")
	}
	if strings.Contains(res.Diagnostic, password) {
		t.Errorf("the relay password appears in the transcript:\n%s", res.Diagnostic)
	}
	if !strings.Contains(res.Diagnostic, "authenticating as carrel") {
		t.Errorf("the transcript does not record the authentication:\n%s", res.Diagnostic)
	}
}
