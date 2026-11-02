package web

import (
	"errors"
	"testing"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/provider/file"
)

// The words of a file that did not go, in the queue and in the notices of
// Files (the owner's decision of 02.11.2026: direct and technically precise):
// the server's answer by its code and standard text, the numbers that are
// known, what became of the data.
func TestFileWordsAreDirect(t *testing.T) {
	status := func(code int) error { return &dav.StatusError{Code: code} }
	for _, c := range []struct {
		err  error
		want string
	}{
		{status(403), "403 Forbidden: this account has no write access here."},
		{status(507), "507 Insufficient Storage."},
		{status(413), "413 Request Entity Too Large."},
		{status(409), "409 Conflict."},
		{errors.New("dial tcp: connection refused"), "No response from the server."},
		{file.TooMany{Max: 10000}, "The folder has more than 10 000 entries, Carrel's limit (CARREL_FILES_MAX_ENTRIES)."},
	} {
		if got := fileReason(c.err); got != c.want {
			t.Errorf("fileReason(%v) = %q, want %q", c.err, got, c.want)
		}
	}

	if got := plural(12, "entry") + ", " + plural(2, "day") + ", " + plural(1, "entry"); got != "12 entries, 2 days, 1 entry" {
		t.Errorf("plurals = %q", got)
	}

	full := status(507)
	for _, c := range []struct{ got, want string }{
		{partialNote("move", 12, full), "Move failed after 12 entries: 507 Insufficient Storage. Source unchanged"},
		{partialNote("copy", 1, full), "Copy failed after 1 entry: 507 Insufficient Storage. Copied entries removed"},
		{partialNote("move", 3, errors.New("EOF")), "Move failed after 3 entries: no response from the server. Source unchanged"},
		{folderNote(full), "could not create its folder: 507 Insufficient Storage"},
		{interruptedNote(3355443), "upload interrupted after 3.2 MB"},
		{takenNote("отчёт (2).pdf"), "New name «отчёт (2).pdf» also taken; nothing overwritten"},
		{changedNote("upload"), "The target changed while waiting for your answer; nothing overwritten. Upload again to choose"},
		{changedNote("move"), "The target changed while waiting for your answer; nothing overwritten. Move again to choose"},
		{overwriteNote(status(412)), "Overwrite refused: 412 Precondition Failed"},
		{badNameNote, "invalid file name"},
		{needsSizeShort, "server requires the file size"},
		{needsSizeNote, "The server accepts a file only with its size sent first; the page sent none. Turn on scripts or upload with another client"},
		{mergedNote, "merged into the existing folder"},
		{takenWhileNote, "The name was taken during the upload; nothing overwritten. Upload again to choose"},
	} {
		if c.got != c.want {
			t.Errorf("%q,\nwant %q", c.got, c.want)
		}
	}
}
