package dav

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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

func testAuthClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	c := testConfig()
	c.DAVSSRFAllowlist = "127.0.0.1"
	client, err := NewClient(NewGuard(c), srv.URL, "alice", "secret")
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestTransportMethodsSendBasicAuth(t *testing.T) {
	srv := authServer(t, "alice", "secret")
	defer srv.Close()
	client := testAuthClient(t, srv)

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

func TestHttpStatusErrorSanitizesBody(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Status:     "400 Bad Request",
		Body:       io.NopCloser(strings.NewReader("hello\x00\x07world" + strings.Repeat("x", 512))),
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
