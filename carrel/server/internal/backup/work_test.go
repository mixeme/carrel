//go:build !race

package backup

import (
	"bytes"
	"regexp"
	"testing"
)

// The key of a sealed archive is stretched at 2^18, age's own factor: the
// race build lowers it to fit the detector into memory (work_race.go), and
// this is where the product's factor is held — read from the header of an
// archive as written.
func TestArchiveIsSealedAtFullWork(t *testing.T) {
	var buf bytes.Buffer
	if _, err := Write(&buf, Manifest{}, nil, "correct horse"); err != nil {
		t.Fatal(err)
	}
	stanza := regexp.MustCompile(`(?m)^-> scrypt \S+ (\d+)$`).FindSubmatch(buf.Bytes())
	if stanza == nil || string(stanza[1]) != "18" {
		t.Fatalf("the archive's key stanza names the work factor %q, want 18", stanza)
	}
}
