package dav

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type mutateRecord struct {
	method  string
	path    string
	ifMatch string
	dest    string
}

type mutateFake struct {
	mu       sync.Mutex
	records  []mutateRecord
	deleteOK bool // when false, DELETE returns 404
}

func (f *mutateFake) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := mutateRecord{
			method:  r.Method,
			path:    r.URL.Path,
			ifMatch: r.Header.Get("If-Match"),
			dest:    r.Header.Get("Destination"),
		}
		if r.Header.Get("If-None-Match") != "" {
			rec.ifMatch = r.Header.Get("If-None-Match")
		}
		f.mu.Lock()
		f.records = append(f.records, rec)
		f.mu.Unlock()

		switch r.Method {
		case http.MethodPut:
			w.Header().Set("ETag", `"new-etag"`)
			w.WriteHeader(http.StatusCreated)
		case http.MethodDelete:
			if f.deleteOK {
				w.WriteHeader(http.StatusNoContent)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		case "MOVE":
			w.WriteHeader(http.StatusCreated)
		case "PROPPATCH":
			w.WriteHeader(http.StatusMultiStatus)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><multistatus xmlns="DAV:"/>`))
		case "MKCOL":
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func (f *mutateFake) last() mutateRecord {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.records) == 0 {
		return mutateRecord{}
	}
	return f.records[len(f.records)-1]
}

func TestPutSendsIfMatch(t *testing.T) {
	fake := &mutateFake{}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	client := testClient(t, srv.URL)
	etag, err := client.Put(t.Context(), "/cal/personal/a.ics", strings.NewReader("data"), `"old-etag"`)
	if err != nil {
		t.Fatal(err)
	}
	if etag != "new-etag" {
		t.Fatalf("etag = %q", etag)
	}
	got := fake.last()
	if got.method != http.MethodPut {
		t.Fatalf("method = %q", got.method)
	}
	if got.ifMatch != `"old-etag"` {
		t.Fatalf("If-Match = %q", got.ifMatch)
	}
}

func TestDelete404IsSuccess(t *testing.T) {
	fake := &mutateFake{deleteOK: false}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	client := testClient(t, srv.URL)
	if err := client.Delete(t.Context(), "/missing.ics", `"gone"`); err != nil {
		t.Fatalf("404 delete should succeed: %v", err)
	}
	got := fake.last()
	if got.method != http.MethodDelete {
		t.Fatalf("method = %q", got.method)
	}
	if got.ifMatch != `"gone"` {
		t.Fatalf("If-Match = %q", got.ifMatch)
	}
}

func TestDeleteSendsIfMatch(t *testing.T) {
	fake := &mutateFake{deleteOK: true}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	client := testClient(t, srv.URL)
	if err := client.Delete(t.Context(), "/cal/personal/a.ics", `"etag-1"`); err != nil {
		t.Fatal(err)
	}
	if got := fake.last(); got.ifMatch != `"etag-1"` {
		t.Fatalf("If-Match = %q", got.ifMatch)
	}
}

func TestMoveOverwrite(t *testing.T) {
	fake := &mutateFake{}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	client := testClient(t, srv.URL)
	if err := client.Move(t.Context(), "/src.ics", "/dst.ics", true); err != nil {
		t.Fatal(err)
	}
	got := fake.last()
	if got.method != "MOVE" {
		t.Fatalf("method = %q", got.method)
	}
	if !strings.Contains(got.dest, "dst.ics") {
		t.Fatalf("Destination = %q", got.dest)
	}
}

func TestPropPatchAgainstFakeServer(t *testing.T) {
	fake := &mutateFake{}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	client := testClient(t, srv.URL)
	set := []Prop{{Name: xml.Name{Space: "DAV:", Local: "displayname"}, Value: []byte("Cal")}}
	remove := []xml.Name{{Space: "DAV:", Local: "getcontentlength"}}
	if err := client.PropPatch(t.Context(), "/cal/personal/", set, remove); err != nil {
		t.Fatal(err)
	}
	if got := fake.last(); got.method != "PROPPATCH" {
		t.Fatalf("method = %q", got.method)
	}
}

func TestMkColPropsAgainstFakeServer(t *testing.T) {
	fake := &mutateFake{}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	client := testClient(t, srv.URL)
	props := []Prop{{Name: xml.Name{Space: "DAV:", Local: "resourcetype"}, Value: []byte(`<collection xmlns="DAV:"/>`)}}
	if err := client.MkColProps(t.Context(), "MKCOL", "/cal/new/", props); err != nil {
		t.Fatal(err)
	}
	if got := fake.last(); got.method != "MKCOL" {
		t.Fatalf("method = %q", got.method)
	}
}

func TestMkColAgainstFakeServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "MKCOL" {
			t.Fatalf("method = %q", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if len(body) != 0 {
			t.Fatalf("MkCol should not send a body, got %d bytes", len(body))
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	client := testClient(t, srv.URL)
	if err := client.MkCol(t.Context(), "/cal/new/"); err != nil {
		t.Fatal(err)
	}
}
