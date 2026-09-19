package dav

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptrace"
	"strings"
	"testing"
)

// docs/servers.md, SV-14 and S-5.1: a server closes a connection the pool kept
// idle, and the next request on it ends before the answer. Go sends a second
// time only what it knows as safe, and the DAV methods are not on that list,
// so the guard does it: once, for a read whatever the pool said, and for a
// write only when the connection came out of the pool idle — there the server
// had closed its side before the request and never saw it.

// flakyBase fails the first n calls with err and answers afterwards, keeping
// the body of every call. fromIdle is what it reports through httptrace about
// the connection the request went out on: a real transport reports it, and
// the guard reads it from there.
type flakyBase struct {
	err      error
	fail     int
	calls    int
	bodies   []string
	fromIdle bool
}

func (f *flakyBase) RoundTrip(req *http.Request) (*http.Response, error) {
	f.calls++
	if trace := httptrace.ContextClientTrace(req.Context()); trace != nil && trace.GotConn != nil {
		trace.GotConn(httptrace.GotConnInfo{Reused: f.fromIdle, WasIdle: f.fromIdle})
	}
	body := ""
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		body = string(b)
	}
	f.bodies = append(f.bodies, body)
	if f.calls <= f.fail {
		return nil, f.err
	}
	return &http.Response{
		StatusCode: http.StatusMultiStatus,
		Body:       io.NopCloser(strings.NewReader("<multistatus/>")),
		Request:    req,
	}, nil
}

func roundTrip(t *testing.T, base *flakyBase, method string, body io.Reader) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, "http://server.example/dav/", body)
	if err != nil {
		t.Fatal(err)
	}
	return (&limitTransport{base: base}).RoundTrip(req)
}

func TestReadOnlyRequestIsSentAgainAfterAnIdleClose(t *testing.T) {
	for _, method := range []string{"PROPFIND", "REPORT", http.MethodGet} {
		base := &flakyBase{err: io.EOF, fail: 1}
		resp, err := roundTrip(t, base, method, bytes.NewReader([]byte("<propfind/>")))
		if err != nil {
			t.Fatalf("%s: %v", method, err)
		}
		resp.Body.Close()
		if base.calls != 2 {
			t.Errorf("%s: %d calls, want a second one", method, base.calls)
		}
		if len(base.bodies) == 2 && base.bodies[0] != base.bodies[1] {
			t.Errorf("%s: the body was not rewound: %q then %q", method, base.bodies[0], base.bodies[1])
		}
	}
}

// A write on a connection dialled for it is not sent twice: the error says
// nothing about what the server did with the first one.
func TestWriteOnAFreshConnectionIsNotSentAgain(t *testing.T) {
	for _, method := range []string{http.MethodPut, http.MethodDelete, "MKCOL", "MKCALENDAR", "MOVE", "PROPPATCH"} {
		base := &flakyBase{err: io.EOF, fail: 1}
		if _, err := roundTrip(t, base, method, bytes.NewReader([]byte("x"))); err == nil {
			t.Errorf("%s: the error was swallowed", method)
		}
		if base.calls != 1 {
			t.Errorf("%s: %d calls, want one", method, base.calls)
		}
	}
}

// A write on a connection the pool had kept idle is sent again, with its body
// rewound: the server closed its side before the request and never saw it
// (S-5.1). One that did land answers the second try with a failed
// precondition — every write Carrel sends is conditional or idempotent (S-8).
func TestWriteOnAnIdleConnectionIsSentAgain(t *testing.T) {
	for _, method := range []string{http.MethodPut, http.MethodDelete, "MKCOL", "MKCALENDAR", "MOVE", "PROPPATCH"} {
		base := &flakyBase{err: io.EOF, fail: 1, fromIdle: true}
		resp, err := roundTrip(t, base, method, bytes.NewReader([]byte("<mkcol/>")))
		if err != nil {
			t.Fatalf("%s: %v", method, err)
		}
		resp.Body.Close()
		if base.calls != 2 {
			t.Errorf("%s: %d calls, want a second one", method, base.calls)
		}
		if len(base.bodies) == 2 && (base.bodies[1] != "<mkcol/>" || base.bodies[0] != base.bodies[1]) {
			t.Errorf("%s: the body was not rewound: %q then %q", method, base.bodies[0], base.bodies[1])
		}
	}
}

// A body that cannot be produced a second time — a file on its way to another
// server — is never sent twice, idle connection or not.
func TestAStreamedBodyIsNotSentAgain(t *testing.T) {
	base := &flakyBase{err: io.EOF, fail: 1, fromIdle: true}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPut, "http://server.example/dav/f",
		struct{ io.Reader }{strings.NewReader("a stream")})
	if err != nil {
		t.Fatal(err)
	}
	if req.GetBody != nil {
		t.Fatal("this body was rewindable after all; the test proves nothing")
	}
	if _, err := (&limitTransport{base: base}).RoundTrip(req); err == nil {
		t.Error("the error was swallowed")
	}
	if base.calls != 1 {
		t.Errorf("%d calls, want one", base.calls)
	}
}

// Only a connection that ended before the answer is worth a second try.
func TestOtherFailuresAreNotRetried(t *testing.T) {
	for _, err := range []error{context.DeadlineExceeded, context.Canceled, ErrTimeout, ErrResponseTooLarge} {
		base := &flakyBase{err: err, fail: 1}
		if _, got := roundTrip(t, base, "PROPFIND", nil); got == nil {
			t.Errorf("%v: the error was swallowed", err)
		}
		if base.calls != 1 {
			t.Errorf("%v: %d calls, want one", err, base.calls)
		}
	}
}

// A second close is the server's answer, not a race: it is not sent a third
// time.
func TestOneRetryOnly(t *testing.T) {
	base := &flakyBase{err: io.EOF, fail: 2}
	if _, err := roundTrip(t, base, "PROPFIND", nil); err == nil {
		t.Error("the error was swallowed")
	}
	if base.calls != 2 {
		t.Errorf("%d calls, want two", base.calls)
	}
}

// The error says which rule applied, because such a failure is read out of a
// log months later and "EOF" alone leaves the reader guessing (S-5.1).
func TestTheErrorSaysWhyItWasNotSentAgain(t *testing.T) {
	for _, c := range []struct {
		what, want string
		fromIdle   bool
		method     string
		body       io.Reader
	}{
		{"a write on a fresh connection", "a write on a connection dialled for it", false, http.MethodPut, bytes.NewReader([]byte("x"))},
		{"a body that cannot rewind", "its body cannot be produced a second time", true, http.MethodPut, struct{ io.Reader }{strings.NewReader("x")}},
	} {
		base := &flakyBase{err: io.EOF, fail: 1, fromIdle: c.fromIdle}
		req, err := http.NewRequestWithContext(context.Background(), c.method, "http://server.example/dav/f", c.body)
		if err != nil {
			t.Fatal(err)
		}
		_, got := (&limitTransport{base: base}).RoundTrip(req)
		if got == nil || !strings.Contains(got.Error(), c.want) {
			t.Errorf("%s: err = %v, want it to say %q", c.what, got, c.want)
		}
		if !errors.Is(got, io.EOF) {
			t.Errorf("%s: the error stopped being an EOF: %v", c.what, got)
		}
	}

	// Twice in a row is the server's answer, and the error says it was tried.
	base := &flakyBase{err: io.EOF, fail: 2, fromIdle: true}
	_, err := roundTrip(t, base, "MKCALENDAR", bytes.NewReader([]byte("<mkcalendar/>")))
	if err == nil || !strings.Contains(err.Error(), "ended the same way") {
		t.Errorf("err = %v, want it to say the second try ended the same way", err)
	}
	if base.calls != 2 {
		t.Errorf("%d calls, want two", base.calls)
	}
}

// A body that cannot be sent twice never rides a connection the pool kept:
// the server may have closed it a moment before, and then the upload is lost
// with nothing to do but tell the person (the SFTPGo attachment of the live
// run, 19.09.2026). Such a request dials a connection of its own; a body that
// rewinds keeps the pool and its retry.
func TestAStreamedBodyTakesAConnectionOfItsOwn(t *testing.T) {
	pooled, fresh := &flakyBase{}, &flakyBase{}
	lt := &limitTransport{base: pooled, fresh: fresh}
	stream, err := http.NewRequestWithContext(context.Background(), http.MethodPut, "http://server.example/dav/f",
		struct{ io.Reader }{strings.NewReader("a stream")})
	if err != nil {
		t.Fatal(err)
	}
	if resp, err := lt.RoundTrip(stream); err != nil {
		t.Fatal(err)
	} else {
		resp.Body.Close()
	}
	rewinds, err := http.NewRequestWithContext(context.Background(), http.MethodPut, "http://server.example/dav/g",
		bytes.NewReader([]byte("a record")))
	if err != nil {
		t.Fatal(err)
	}
	if resp, err := lt.RoundTrip(rewinds); err != nil {
		t.Fatal(err)
	} else {
		resp.Body.Close()
	}
	if fresh.calls != 1 || pooled.calls != 1 {
		t.Errorf("fresh %d, pooled %d: the stream goes on its own connection, the record through the pool", fresh.calls, pooled.calls)
	}
}
