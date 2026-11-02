package web

import (
	"errors"
	"testing"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/provider/file"
	"carrel.dev/carrel/server/view"
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

	// A delete the server refused for some of the items (the owner's words of
	// 02.11.2026): how many of how many, one answer for all in the line, what
	// was deleted and what stays; each item with its answer in the details.
	forbidden := error(&dav.StatusError{Code: 403, Msg: "Forbidden"})
	stale := error(&dav.StatusError{Code: 412, Msg: "Precondition Failed"})
	for _, c := range []struct {
		total          int
		refused        []deleteRefusal
		title, body    string
		first, summary string
	}{
		{5, []deleteRefusal{{"отчёт.pdf", forbidden}, {"план.docx", stale}, {"смета.xlsx", forbidden}},
			"Delete failed for 3 of 5 items.", "2 deleted, 3 unchanged.",
			"отчёт.pdf · 403 Forbidden: this account has no write access here. " + forbidden.Error(), "Not deleted"},
		{5, []deleteRefusal{{"отчёт.pdf", forbidden}, {"смета.xlsx", forbidden}},
			"Delete failed for 2 of 5 items: 403 Forbidden: this account has no write access here.", "3 deleted, 2 unchanged.",
			"отчёт.pdf · 403 Forbidden: this account has no write access here. " + forbidden.Error(), "Not deleted"},
		{1, []deleteRefusal{{"план.docx", stale}},
			"Delete failed for 1 of 1 item: 412 Precondition Failed: changed since the folder was read.", "0 deleted, 1 unchanged.",
			"план.docx · 412 Precondition Failed: changed since the folder was read. " + stale.Error(), "Not deleted"},
	} {
		n := deleteFailed(c.total, c.refused)
		if string(n.Title) != c.title || string(n.Body) != c.body || n.Role != view.NoticeAlert {
			t.Errorf("deleteFailed: %q / %q (%s),\nwant %q / %q", n.Title, n.Body, n.Role, c.title, c.body)
		}
		if n.Details == nil || string(n.Details.Summary) != c.summary || len(n.Details.Lines) != len(c.refused) || string(n.Details.Lines[0]) != c.first {
			t.Errorf("deleteFailed details = %+v,\nwant first %q", n.Details, c.first)
		}
	}
}
