package proxy_test

import (
	"net/http"
	"testing"
)

// S-27.3: davloom passes DAV on, it is not a general proxy. A method it does
// not advertise in Allow — TRACE, CONNECT, whatever a client invents — is
// answered 405 here and never reaches the collection's server with the
// person's credentials (audit №3, Ф3-9).
func TestOnlyAdvertisedMethodsAreForwarded(t *testing.T) {
	f := newFixture(t)
	for _, method := range []string{"TRACE", "CONNECT", "PURGE", "SEARCH"} {
		resp, _ := f.do(method, f.col(work), "", nil)
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s: %d, want 405", method, resp.StatusCode)
		}
		if resp.Header.Get("Allow") == "" {
			t.Errorf("%s: no Allow header", method)
		}
		if n := f.far.CountRequests(method, ""); n != 0 {
			t.Errorf("%s reached the far server %d times", method, n)
		}
	}
	// What is advertised still goes through.
	if resp, _ := f.do("PROPFIND", f.col(work), "", map[string]string{"Depth": "0"}); resp.StatusCode != http.StatusMultiStatus {
		t.Errorf("PROPFIND: %d", resp.StatusCode)
	}
}
