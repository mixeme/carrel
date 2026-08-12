// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package mail

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/store"
)

// Config is the relay settings needed to send one message.
type Config struct {
	Host, Username, Password, FromAddress, FromName string
	Port                                            int
	TLS                                             store.TLSMode
}

// Message is one outbound email.
type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

// Result is the outcome of a send attempt. Diagnostic carries the full SMTP
// conversation for the admin test button (§5.3).
type Result struct {
	OK         bool
	Diagnostic string
}

// Send delivers one message synchronously and returns a diagnostic transcript.
func Send(cfg Config, msg Message) Result {
	var diag strings.Builder
	write := func(format string, args ...any) {
		fmt.Fprintf(&diag, format, args...)
		diag.WriteByte('\n')
	}

	if cfg.Host == "" || cfg.Port == 0 || cfg.FromAddress == "" {
		write("error: SMTP is not configured (host, port and from address are required)")
		return Result{Diagnostic: diag.String()}
	}
	if msg.To == "" {
		write("error: recipient address is empty")
		return Result{Diagnostic: diag.String()}
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	write("connecting to %s (%s)", addr, cfg.TLS)

	client, err := dial(cfg, &diag)
	if err != nil {
		write("connection failed: %v", err)
		return Result{Diagnostic: diag.String()}
	}
	defer client.Close()

	if cfg.Username != "" {
		write("authenticating as %s", cfg.Username)
		if err := client.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
			write("auth failed: %v", err)
			return Result{Diagnostic: diag.String()}
		}
		write("auth ok")
	}

	from := cfg.FromAddress
	if err := client.Mail(from); err != nil {
		write("MAIL FROM failed: %v", err)
		return Result{Diagnostic: diag.String()}
	}
	write("MAIL FROM <%s> ok", from)

	if err := client.Rcpt(msg.To); err != nil {
		write("RCPT TO failed: %v", err)
		return Result{Diagnostic: diag.String()}
	}
	write("RCPT TO <%s> ok", msg.To)

	wc, err := client.Data()
	if err != nil {
		write("DATA failed: %v", err)
		return Result{Diagnostic: diag.String()}
	}

	body := encodeMessage(cfg, msg)
	if _, err := wc.Write(body); err != nil {
		write("write body failed: %v", err)
		return Result{Diagnostic: diag.String()}
	}
	if err := wc.Close(); err != nil {
		write("close body failed: %v", err)
		return Result{Diagnostic: diag.String()}
	}
	write("message accepted")

	if err := client.Quit(); err != nil {
		write("QUIT: %v", err)
	} else {
		write("session closed")
	}
	return Result{OK: true, Diagnostic: diag.String()}
}

func dial(cfg Config, diag *strings.Builder) (*smtp.Client, error) {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	switch cfg.TLS {
	case store.TLSImplicit:
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 30 * time.Second}, "tcp", addr, tlsConfig(cfg.Host))
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(diag, "connected (implicit TLS)\n")
		return smtp.NewClient(conn, cfg.Host)
	case store.TLSNone:
		client, err := smtp.Dial(addr)
		if err == nil {
			fmt.Fprintf(diag, "connected (no TLS)\n")
		}
		return client, err
	default:
		client, err := smtp.Dial(addr)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(diag, "connected\n")
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsConfig(cfg.Host)); err != nil {
				fmt.Fprintf(diag, "STARTTLS failed: %v\n", err)
				client.Close()
				return nil, err
			}
			fmt.Fprintf(diag, "STARTTLS ok\n")
		} else {
			fmt.Fprintf(diag, "warning: server does not advertise STARTTLS\n")
		}
		return client, nil
	}
}

func tlsConfig(serverName string) *tls.Config {
	return &tls.Config{ServerName: serverName, MinVersion: tls.VersionTLS12}
}

func encodeMessage(cfg Config, msg Message) []byte {
	fromHeader := cfg.FromAddress
	if cfg.FromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", cfg.FromName, cfg.FromAddress)
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "From: %s\r\n", fromHeader)
	fmt.Fprintf(&buf, "To: %s\r\n", msg.To)
	fmt.Fprintf(&buf, "Subject: %s\r\n", msg.Subject)
	fmt.Fprintf(&buf, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	if msg.HTML != "" {
		boundary := fmt.Sprintf("carrel-%d", time.Now().UnixNano())
		fmt.Fprintf(&buf, "Content-Type: multipart/alternative; boundary=%q\r\n", boundary)
		buf.WriteString("\r\n")
		writePart(&buf, boundary, "text/plain; charset=utf-8", msg.Text)
		writePart(&buf, boundary, "text/html; charset=utf-8", msg.HTML)
		fmt.Fprintf(&buf, "--%s--\r\n", boundary)
	} else {
		buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
		buf.WriteString(msg.Text)
		if !strings.HasSuffix(msg.Text, "\n") {
			buf.WriteByte('\n')
		}
	}
	return buf.Bytes()
}

func writePart(buf *bytes.Buffer, boundary, contentType, body string) {
	fmt.Fprintf(buf, "--%s\r\n", boundary)
	fmt.Fprintf(buf, "Content-Type: %s\r\n\r\n", contentType)
	buf.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		buf.WriteByte('\n')
	}
}
