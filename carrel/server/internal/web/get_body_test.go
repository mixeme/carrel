package web_test

import (
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// countingBody counts what was read of a request's body.
type countingBody struct {
	r io.Reader
	n int64
}

func (c *countingBody) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// Audit №4, А4-4: a GET has no ceiling on its body — the middleware caps
// only what can change something — and Go's FormValue reads a multipart
// body of any method: 32 MiB of it into memory and the rest into a
// temporary file. Every screen that reads a value with FormValue took a GET
// with a body of any size from anyone signed in, memory and disk alike. A
// GET or HEAD of Carrel carries nothing in its body, and nothing reads one.
func TestAGetBodyIsNeverRead(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	sess := h.login("alice", "password123")
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, path := range []string{"/app/settings/connections", "/app/settings/unlock", "/app/files/picker", "/login"} {
			pr, pw := io.Pipe()
			mw := multipart.NewWriter(pw)
			go func() {
				defer pw.Close()
				part, err := mw.CreateFormFile("f", "f.bin")
				if err != nil {
					return
				}
				buf := make([]byte, 1<<20)
				for range 8 {
					if _, err := part.Write(buf); err != nil {
						return
					}
				}
				_ = mw.Close()
			}()
			body := &countingBody{r: pr}
			req := httptest.NewRequest(method, path, body)
			req.Header.Set("Content-Type", mw.FormDataContentType())
			req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: sess.ID()})
			h.do(req)
			_ = pr.Close()
			if body.n != 0 {
				t.Errorf("%s %s read %d bytes of its body", method, path, body.n)
			}
		}
	}
}
