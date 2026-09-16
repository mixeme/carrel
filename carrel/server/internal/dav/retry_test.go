package dav

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// docs/servers.md, SV-14: a server closes a connection the pool kept idle, and
// the next request on it ends before the answer. Go sends a second time only
// what it knows as safe, and the DAV methods are not on that list, so the
// guard does it: once, for a request that only reads and can produce its body
// again.

// flakyBase fails the first n calls with err and answers afterwards, keeping
// the body of every call.
type flakyBase struct {
	err    error
	fail   int
	calls  int
	bodies []string
}

func (f *flakyBase) RoundTrip(req *http.Request) (*http.Response, error) {
	f.calls++
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

// A write is never sent twice: the server may have taken the first one.
func TestWritesAreNotSentAgain(t *testing.T) {
	for _, method := range []string{http.MethodPut, http.MethodDelete, "MKCOL", "MOVE", "PROPPATCH"} {
		base := &flakyBase{err: io.EOF, fail: 1}
		if _, err := roundTrip(t, base, method, bytes.NewReader([]byte("x"))); err == nil {
			t.Errorf("%s: the error was swallowed", method)
		}
		if base.calls != 1 {
			t.Errorf("%s: %d calls, want one", method, base.calls)
		}
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
