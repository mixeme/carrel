package file

import (
	"testing"
)

func TestJoinRejectsDotDot(t *testing.T) {
	_, err := Join("/dav/", "..", "secret")
	if err != ErrBadSegment {
		t.Fatalf("got %v", err)
	}
}

func TestJoinRejectsEmpty(t *testing.T) {
	_, err := Join("/dav/", "ok", "")
	if err != ErrBadSegment {
		t.Fatalf("got %v", err)
	}
}

func TestJoinRejectsDot(t *testing.T) {
	_, err := Join("/dav/", ".")
	if err != ErrBadSegment {
		t.Fatalf("got %v", err)
	}
}

func TestJoinAllowsDotfile(t *testing.T) {
	got, err := Join("/dav/", ".hidden")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/dav/.hidden" {
		t.Fatalf("got %q", got)
	}
}

func TestJoinRejectsSlash(t *testing.T) {
	_, err := Join("/dav/", "a/b")
	if err != ErrBadSegment {
		t.Fatalf("got %v", err)
	}
}

func TestJoinRejectsBackslash(t *testing.T) {
	_, err := Join("/dav/", "a\\b")
	if err != ErrBadSegment {
		t.Fatalf("got %v", err)
	}
}

func TestJoinBuildsPath(t *testing.T) {
	got, err := Join("/dav/home/", "docs", "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/dav/home/docs/a.txt" {
		t.Fatalf("got %q", got)
	}
}

func TestJoinStaysUnderRoot(t *testing.T) {
	_, err := Join("/dav/home", "..", "other")
	if err != ErrBadSegment {
		t.Fatalf("got %v", err)
	}
}
