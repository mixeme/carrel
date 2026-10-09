package web_test

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// The client's second scan of the files section (client.md, «Повторный скан
// по кадрам Typecase 6833ef4», ФН1…ФН4): what the frames of a folder
// (s-files-1, states-32, -33, -34) draw that the view model did not carry.

// ФН1: the line under the title counts the folder — «sftpgo · vps ·
// ‹attachment folder› · 5 items» (s-files-1), «… · 2 items · read-only:
// nothing can be uploaded or made here» (states-34) — and says none when the
// storage did not answer (states-33).
func TestFolderHeadCountsTheFolder(t *testing.T) {
	line := func(id string) string {
		return frameText(frameOne(t, frameHeadSub, mockupFrame(t, id), "line under the title")[1])
	}
	count := regexp.MustCompile(` · (\d+ items?)( · (read-only.*))?$`)
	plain, ro := count.FindStringSubmatch(line("s-files-1")), count.FindStringSubmatch(line("states-34"))
	if plain == nil || ro == nil || ro[3] == "" || count.MatchString(line("states-33")) {
		t.Fatalf("lines %q, %q, %q: the frames changed, read them again", line("s-files-1"), line("states-34"), line("states-33"))
	}

	f := filesFixture(t, true)
	for _, name := range []string{"a.txt", "b.txt", "c.txt", "d.txt"} {
		f.srv.SeedFile(filesDocs+name, name)
	}
	f.srv.SeedFolder(filesDocs + "sub/")
	title := string(decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess))).Accounts[0].Title)
	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	if string(page.Subtitle) != title || string(page.Count) != plain[1] || page.Access != "" {
		t.Errorf("line = %q, %q, %q; want %q, %q and no access", page.Subtitle, page.Count, page.Access, title, plain[1])
	}
	// A filter does not change what the folder holds.
	if page = decode[view.FilesPage](t, f.h.do(get(f.at("")+"?filter=a", f.sess))); string(page.Count) != plain[1] {
		t.Errorf("filtered, the folder counts %q, want %q", page.Count, plain[1])
	}
	if one := decode[view.FilesPage](t, f.h.do(get(f.at("", "sub", ""), f.sess))); one.Count != "0 items" {
		t.Errorf("an empty folder counts %q", one.Count)
	}

	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		for i := range b.Collections {
			if b.Collections[i].Path == filesDocs {
				b.Collections[i].ReadOnly = true
			}
		}
	})
	page = decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	if string(page.Subtitle) != title || string(page.Count) != plain[1] || string(page.Access) != ro[3] {
		t.Errorf("read-only line = %q, %q, %q; want %q, %q, %q", page.Subtitle, page.Count, page.Access, title, plain[1], ro[3])
	}
}

// ФН2: an empty folder says what to do with it — «This folder is empty.
// Upload files into it, or drop them onto the window.» with Upload and New
// folder (states-32); a read-only one has nothing to offer.
func TestEmptyFolderIsTheFrame(t *testing.T) {
	m := frameOne(t, regexp.MustCompile(`(?s)<div class="tc-empty"><b>(.*?)</b>(.*?)<button`), mockupFrame(t, "states-32"), "empty folder")
	title, body := frameText(m[1]), frameText(m[2])

	f := filesFixture(t, true)
	f.srv.SeedFolder(filesDocs + "sub/")
	page := decode[view.FilesPage](t, f.h.do(get(f.at("", "sub", ""), f.sess)))
	if page.Empty == nil || string(page.Empty.Title) != title || string(page.Empty.Body) != body {
		t.Fatalf("empty = %+v, want %q %q", page.Empty, title, body)
	}
	up, nf := actionByID(page.Empty.Actions, "file.upload"), actionByID(page.Empty.Actions, "folder.new")
	if up == nil || nf == nil || !up.Primary || len(up.Arrow) != 2 || !up.Enabled || !nf.Enabled || len(page.Empty.Actions) != 2 {
		t.Errorf("the empty folder's buttons = %+v, want Upload with its arrow and New folder", page.Empty.Actions)
	}
	// A folder with something in it, and one whose filter hides everything,
	// are not empty.
	if full := decode[view.FilesPage](t, f.h.do(get(f.at("")+"?filter=zzz", f.sess))); full.Empty != nil || len(full.Rows) != 0 {
		t.Errorf("a folder filtered to nothing = %+v, %d rows", full.Empty, len(full.Rows))
	}
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		for i := range b.Collections {
			if b.Collections[i].Path == filesDocs {
				b.Collections[i].ReadOnly = true
			}
		}
	})
	page = decode[view.FilesPage](t, f.h.do(get(f.at("", "sub", ""), f.sess)))
	if page.Empty == nil || string(page.Empty.Title) != title || page.Empty.Body != "" || len(page.Empty.Actions) != 0 {
		t.Errorf("a read-only empty folder = %+v, want the title alone", page.Empty)
	}
}

// ФН3: a storage whose server did not answer says so and nothing else
// (states-33): the model says the folder was not read — no count, no rows,
// no queue —, and the message is the frame's.
func TestUnansweredFolderIsTheFrame(t *testing.T) {
	m := frameOne(t, regexp.MustCompile(`(?s)<div class="tc-msg is-alert"><b>(.*?)</b>(.*?)<button class="tc-btn is-sm" type="button">(.*?)</button>`), mockupFrame(t, "states-33"), "alert")
	title, body, again := frameText(m[1]), frameText(m[2]), frameText(m[3])
	if !strings.HasSuffix(title, " is unavailable.") {
		t.Fatalf("the alert of states-33 is %q: read the frame again", title)
	}

	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a.txt", "alpha")
	f.srv.Refuse("PROPFIND", http.StatusServiceUnavailable, "")
	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	if !page.Unread || page.Count != "" || len(page.Rows) != 0 || page.Queue != nil || page.Empty != nil {
		t.Errorf("unread %v, count %q, %d rows, queue %+v, empty %+v", page.Unread, page.Count, len(page.Rows), page.Queue, page.Empty)
	}
	if len(page.Notices) != 1 {
		t.Fatalf("notices = %+v", page.Notices)
	}
	n := page.Notices[0]
	if n.Role != view.NoticeAlert || string(n.Title)+"." != strings.Replace(title, "Documents", "Docs", 1) || string(n.Body) != body || n.Details == nil {
		t.Errorf("notice = %+v, want %q %q", n, title, body)
	}
	if a := actionByID(n.Actions, "source.retry"); a == nil || string(a.Label) != again || a.Href != f.at("") {
		t.Errorf("retry = %+v", n.Actions)
	}
}

// ФН4: the name of a row is a link — a folder's to the folder, a file's to
// its folder with it chosen (s-files-1: TableFile.Href) —, a field of its
// own: Href of a file stays its download.
func TestRowNameHasItsAddress(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a b.txt", "alpha")
	f.srv.SeedFolder(filesDocs + "sub/")
	page := decode[view.FilesPage](t, f.h.do(get(f.at("")+"?sort=size", f.sess)))
	if len(page.Rows) != 2 {
		t.Fatalf("rows = %+v", page.Rows)
	}
	dir, file := page.Rows[0], page.Rows[1]
	if dir.Open != dir.Href || dir.Open != f.at("", "sub", "") {
		t.Errorf("a folder's name leads to %q, the row to %q", dir.Open, dir.Href)
	}
	if file.Open != f.at("", "a b.txt") || file.Href == file.Open {
		t.Errorf("a file's name leads to %q (download %q), want %q", file.Open, file.Href, f.at("", "a b.txt"))
	}
	opened := decode[view.FilesPage](t, f.h.do(get(file.Open, f.sess)))
	if len(opened.Rows) != 2 || !opened.Rows[1].Selected || opened.Details == nil {
		t.Errorf("the name's address did not open the folder with the file chosen: %+v", opened.Rows)
	}
}
