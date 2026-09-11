package dav

import (
	"encoding/base64"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func expectedBasicAuth(user, pass string) string {
	creds := user + ":" + pass
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
}

func authServer(t *testing.T, user, pass string) *httptest.Server {
	t.Helper()
	want := expectedBasicAuth(user, pass)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != want {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("ETag", `"e1"`)
			io.WriteString(w, "body")
		case http.MethodPut:
			w.Header().Set("ETag", `"e2"`)
			w.WriteHeader(http.StatusCreated)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case "PROPFIND":
			w.WriteHeader(http.StatusMultiStatus)
			io.WriteString(w, `<?xml version="1.0"?><multistatus xmlns="DAV:"/>`)
		case "PROPPATCH":
			w.WriteHeader(http.StatusMultiStatus)
			io.WriteString(w, `<?xml version="1.0"?><multistatus xmlns="DAV:"/>`)
		case "MKCOL":
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
}

func testAuthClient(t *testing.T, endpoint string) *Client {
	t.Helper()
	c := testConfig()
	c.DAVSSRFAllowlist = "127.0.0.1,localhost"
	client, err := NewClient(NewGuard(c), endpoint, "alice", "secret", "")
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestTransportMethodsSendBasicAuth(t *testing.T) {
	srv := authServer(t, "alice", "secret")
	defer srv.Close()
	client := testAuthClient(t, srv.URL)

	if _, _, err := client.Get(t.Context(), "/a.ics", nil); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if _, err := client.Put(t.Context(), "/a.ics", strings.NewReader("x"), ""); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := client.Delete(t.Context(), "/a.ics", ""); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := client.PropFind(t.Context(), "/", DepthZero, []xml.Name{{Space: "DAV:", Local: "displayname"}}); err != nil {
		t.Fatalf("PropFind: %v", err)
	}
	if err := client.PropPatch(t.Context(), "/a.ics", nil, nil); err != nil {
		t.Fatalf("PropPatch: %v", err)
	}
	if err := client.MkColProps(t.Context(), "MKCOL", "/new/", nil); err != nil {
		t.Fatalf("MkColProps: %v", err)
	}
}

func TestBasicAuthNotSentOnCrossHostRedirect(t *testing.T) {
	want := expectedBasicAuth("alice", "secret")
	var targetAuth atomic.Value

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetAuth.Store(r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != want {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirector.Close()

	entry := strings.Replace(redirector.URL, "127.0.0.1", "localhost", 1)
	client := testAuthClient(t, entry)

	if _, _, err := client.Get(t.Context(), "/", nil); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got, _ := targetAuth.Load().(string); got != "" {
		t.Fatalf("cross-host redirect sent Authorization %q", got)
	}
}

func TestBasicAuthSentOnSameHostRedirect(t *testing.T) {
	want := expectedBasicAuth("alice", "secret")
	var secondAuth atomic.Value

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/first" {
			if r.Header.Get("Authorization") != want {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/second", http.StatusFound)
			return
		}
		secondAuth.Store(r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	entry := strings.Replace(srv.URL, "127.0.0.1", "localhost", 1) + "/first"
	client := testAuthClient(t, strings.TrimSuffix(entry, "/first"))

	if _, _, err := client.Get(t.Context(), "/first", nil); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got, _ := secondAuth.Load().(string); got != want {
		t.Fatalf("same-host redirect Authorization = %q, want %q", got, want)
	}
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func TestHttpStatusErrorSanitizesBody(t *testing.T) {
	cr := &countingReader{r: strings.NewReader("hello\x00\x07world" + strings.Repeat("x", 512))}
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Status:     "400 Bad Request",
		Body:       io.NopCloser(cr),
	}
	err := httpStatusError(resp)
	msg := err.Error()
	if strings.Contains(msg, "\x00") || strings.Contains(msg, "\x07") {
		t.Fatalf("error contains control chars: %q", msg)
	}
	if len(msg) > 320 {
		t.Fatalf("error too long: %d bytes", len(msg))
	}
}

func TestHttpStatusErrorLimitsBodyRead(t *testing.T) {
	cr := &countingReader{r: strings.NewReader(strings.Repeat("x", 900_000))}
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Status:     "500 Internal Server Error",
		Body:       io.NopCloser(cr),
	}
	_ = httpStatusError(resp)
	if cr.n > 4096 {
		t.Fatalf("read %d bytes from error body, want at most 4096", cr.n)
	}
}
