package dav

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPutCreateSendsIfNoneMatch(t *testing.T) {
	fake := &mutateFake{}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	client := testClient(t, srv.URL)
	etag, err := client.PutCreate(t.Context(), "/files/new.txt", strings.NewReader("data"))
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
	if got.ifMatch != "*" {
		t.Fatalf("If-None-Match = %q, want *", got.ifMatch)
	}
}

func TestPutCreate412(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") != "*" {
			t.Fatalf("If-None-Match = %q", r.Header.Get("If-None-Match"))
		}
		w.WriteHeader(http.StatusPreconditionFailed)
	}))
	defer srv.Close()

	client := testClient(t, srv.URL)
	_, err := client.PutCreate(t.Context(), "/exists.txt", strings.NewReader("x"))
	if err == nil {
		t.Fatal("expected error on 412")
	}
	if !IsStatus(err, http.StatusPreconditionFailed) {
		t.Fatalf("got %v, want 412", err)
	}
}
