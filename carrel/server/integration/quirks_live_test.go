//go:build integration

package integration

// Watches for server quirks in docs/servers.md that had no test. Each asserts
// the quirk as it is today and fails the day the server changes; the entry is
// then revised first (docs/servers.md, «Как вести»).

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/discover"
	"carrel.dev/carrel/server/internal/provider/file"
)

// SV-8: SFTPGo (x/net/webdav) ignores If-None-Match on PUT and overwrites a
// taken name. Carrel looks a name up before a conditional PUT because of it.
func TestSFTPGoIgnoresIfNoneMatch(t *testing.T) {
	client, _ := sftpgoClient(t)
	ctx := context.Background()
	path := fmt.Sprintf("/carrel-sv8-%d.txt", os.Getpid())
	t.Cleanup(func() {
		c, _ := sftpgoClient(t)
		_ = c.Delete(context.Background(), path, "")
	})

	if _, err := client.PutCreate(ctx, path, strings.NewReader("first")); err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err := client.PutCreate(ctx, path, strings.NewReader("second"))
	if file.ErrOccupied(err) {
		t.Fatal("SFTPGo now refuses a taken name with 412: revise docs/servers.md, SV-8")
	}
	if err != nil {
		t.Fatalf("second conditional PUT: %v", err)
	}
	rc, _, err := client.Get(ctx, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(rc)
	rc.Close()
	if err != nil || string(got) != "second" {
		t.Errorf("after the second conditional PUT the file holds %q (%v)", got, err)
	}
}

// SV-9: the Apache in front of Baikal closes a kept-alive connection after
// about five seconds idle. A client kept longer sends its next PUT down a
// closed connection, and Go does not send a PUT twice: live tests take a fresh
// client, and web takes one a request. The test times the server's own close.
func TestBaikalIdleConnectionClosed(t *testing.T) {
	endpoint, _, _ := baikalConfig(t)
	u, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	port := u.Port()
	if port == "" {
		port = "443"
	}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", net.JoinHostPort(u.Hostname(), port),
		&tls.Config{ServerName: u.Hostname(), MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	if _, err := fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: %s\r\nConnection: keep-alive\r\n\r\n", path, u.Host); err != nil {
		t.Fatal(err)
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.Close {
		t.Fatal("the server closes the connection with its answer: revise docs/servers.md, SV-9")
	}

	start := time.Now()
	_ = conn.SetReadDeadline(start.Add(30 * time.Second))
	_, err = br.ReadByte()
	idle := time.Since(start)
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		t.Fatal("the connection stayed open 30 s idle: revise docs/servers.md, SV-9")
	}
	if idle < 3*time.Second || idle > 15*time.Second {
		t.Fatalf("closed after %v idle, docs/servers.md SV-9 says about 5 s: revise it", idle.Round(100*time.Millisecond))
	}
	t.Logf("closed after %v idle (%v)", idle.Round(100*time.Millisecond), err)
}

// SV-10: Baikal's well-known answers an https request with a redirect to
// http. Carrel follows no redirect from https to http, in discovery or in the
// guard, so a password never goes out in the clear; discovery stays on the
// entered address and still finds the homes.
func TestBaikalWellKnownDowngradesScheme(t *testing.T) {
	endpoint, user, pass := baikalConfig(t)
	u, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	hc := *testGuard(t).HTTPClient()
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	for _, wk := range []string{"/.well-known/caldav", "/.well-known/carddav"} {
		target := url.URL{Scheme: u.Scheme, Host: u.Host, Path: wk}
		resp, err := hc.Get(target.String())
		if err != nil {
			t.Fatalf("GET %s: %v", wk, err)
		}
		resp.Body.Close()
		loc, _ := resp.Location()
		if resp.StatusCode/100 != 3 || loc == nil || loc.Scheme != "http" {
			t.Errorf("%s answers %d, Location %v: no longer a downgrade to http, revise docs/servers.md, SV-10", wk, resp.StatusCode, loc)
			continue
		}
		t.Logf("%s over %s: %d to %s", wk, u.Scheme, resp.StatusCode, loc)
	}

	res, trace, err := discover.Run(context.Background(), testGuard(t), endpoint, user, pass, "")
	if err != nil || res.CalendarHome == "" || res.AddressBookHome == "" {
		t.Fatalf("discovery past the downgrade: %v %+v %+v", err, res, trace)
	}
	var said bool
	for _, s := range trace.Steps {
		said = said || s.Name == ".well-known" && strings.Contains(string(s.Detail), "downgrades HTTPS to HTTP")
	}
	if !said {
		t.Errorf("the trace does not say why well-known was not followed: %+v", trace.Steps)
	}
}
