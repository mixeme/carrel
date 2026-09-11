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

func TestJoinRejectsLeadingDot(t *testing.T) {
	_, err := Join("/dav/", ".hidden")
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
