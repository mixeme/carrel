package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
	"github.com/emersion/go-webdav/carddav"
)

// C-14.2: the public pages a link opens (U-6.15, Пс1), one state for each
// kind of page the frames draw — s-publish-6 to -39, each link with what
// the frame's label says was ticked. The links are given out through the
// sheet, as a person does; the page is asked without a session.
func TestFixturePublicPages(t *testing.T) {
	app := newMockupApp(t)
	app.h.srv.SetClock(mockupNow(t))
	app.h.users.SetClock(mockupNow(t))
	give := func(name string, q url.Values) string {
		rec := app.h.do(postForm("/app/publish", app.sess, q))
		if rec.Code != 200 {
			t.Fatalf("publish %v: %d %s", q, rec.Code, rec.Body.String())
		}
		link := string(decode[view.PublishSheet](t, rec).Link)
		if !strings.Contains(link, "/p/") {
			t.Fatalf("publish %v: no link", q)
		}
		token := strings.TrimPrefix(link[strings.Index(link, "/p/"):], "/p/")
		app.byAccount[token] = "link-" + name + "-token"
		return token
	}
	// record gives a collection or a record out; sheet, in pairs, is what
	// its sheet chose.
	record := func(name, k, slug, uid string, sheet ...string) string {
		q := url.Values{"k": {k}, "a": {app.realAccount(t, slug)}, "c": {app.realPath(t, slug)}}
		if uid != "" {
			q.Set("uid", uid)
		}
		for i := 0; i+1 < len(sheet); i += 2 {
			q.Set(sheet[i], sheet[i+1])
		}
		return give(name, q)
	}
	file := func(name, rel string, dir bool) string {
		q := url.Values{"k": {"files"}, "a": {app.realAccount(t, "Documents")}, "c": {app.realPath(t, "Documents")}, "f": {rel}}
		if dir {
			q.Set("dir", "1")
		}
		return give(name, q)
	}
	// A file of the set's storage made for one frame: a text larger than
	// the page shows (s-publish-34), a folder with nothing in it (-36), a
	// file deleted after its link was made (-38).
	docs := app.realPath(t, "Documents")
	sftp := app.servers["sftpgo · vps"]
	sftp.SeedFile(docs+"carrel/server-log-august.txt", "2026-08-11 00:00:01 started\n")
	sftp.Grow(docs+"carrel/server-log-august.txt", 14600<<10)
	sftp.SetModified(docs+"carrel/server-log-august.txt", mockupDayAt(t, "2026-08-11 09:00"))
	sftp.SeedFolder(docs + "carrel/черновики/")
	// The note's recording beside its board (s-publish-8: two files).
	attachMockupRecording(t, app)
	// The published folder is August's of the attachments (note 180 of
	// Typecase), with a folder of six screenshots in it, as s-publish-12 and
	// -30 draw them.
	for name, at := range map[string]string{
		"2026-08-10-sync-board.png": "2026-08-10 11:07", "2026-08-10-timeline.png": "2026-08-10 11:05",
		"export-dialog.png": "2026-08-09 17:40", "import-wizard.png": "2026-08-09 17:32",
		"publish-sheet.png": "2026-08-09 17:20", "search-results.png": "2026-08-09 17:10",
	} {
		shot := docs + "carrel/attachments/2026-08/скриншоты/" + name
		sftp.SeedFile(shot, davtest.PNGSignature)
		sftp.SetModified(shot, mockupDayAt(t, at))
	}
	sftp.SetModified(docs+"carrel/attachments/2026-08/скриншоты", mockupDayAt(t, "2026-08-10 11:07"))
	sftp.SeedFile(docs+"carrel/отчёт-июль.pdf", "%PDF-1.4\n")
	gone := file("report", "carrel/отчёт-июль.pdf", false)
	sftp.RemoveFile(docs + "carrel/отчёт-июль.pdf")
	// The whole of «Личный», as the sheet stands (s-publish-24): the title
	// and the time, no .ics, no end.
	whole := record("lichnyj-whole", "calendar", "lichnyj", "", "how", "whole", "lives", "never", "title", "on")
	folder := file("august", "carrel/attachments/2026-08", true)
	for _, one := range []struct{ state, token string }{
		// «Личный» for 10–31 August with Location and Attendees ticked and
		// its .ics (s-publish-6).
		{"normal", record("lichnyj-shown", "calendar", "lichnyj", "", "how", "range", "from", "2026-08-10", "to", "2026-08-31",
			"lives", "30", "title", "on", "location", "on", "attendees", "on", "download", "on")},
		// «Команда» with the phones and the addresses ticked and its .vcf
		// (s-publish-7).
		{"addressbook", record("komanda", "contacts", "komanda", "", "lives", "30", "name", "on", "phones", "on", "emails", "on", "download", "on")},
		{"whole", whole}, // s-publish-24
		{"whole-empty", whole + "?from=2027-01-04"},                // s-publish-25: a week with nothing in it
		{"filtered", whole + "?filter=" + url.QueryEscape("синк")}, // s-publish-39
		// One event with all but the files and the tags, its .ics
		// (s-publish-16); one contact with the phones, the addresses and the
		// note, no .vcf (-26); one task with Location and Description (-27).
		{"event", record("sync", "calendar", "lichnyj", "8f1c-4a02-b7d3", "lives", "30", "title", "on", "location", "on", "description", "on", "attendees", "on", "download", "on")},
		{"contact", record("anna", "contacts", "kontakty", "4f2b-9c31-a0e7", "lives", "30", "name", "on", "phones", "on", "emails", "on", "note", "on")},
		{"task", record("mfc", "tasks", "dela", "aa10-77bc-0e31", "lives", "30", "title", "on", "location", "on", "description", "on")},
		{"tasks", record("dela", "tasks", "dela", "", "lives", "30", "title", "on")},         // s-publish-28
		{"notebook", record("zapisi", "notes", "zapisi", "", "lives", "30")},                 // s-publish-29: no .md
		{"pdf", file("smeta-page", "carrel/attachments/2026-08/смета-подрядчик.pdf", false)}, // s-publish-33
		{"text-large", file("server-log", "carrel/server-log-august.txt", false)},            // s-publish-34
		{"subfolder", folder + "?in=" + url.QueryEscape("скриншоты")},                        // s-publish-30
		{"folder-empty", file("drafts", "carrel/черновики", true)},                           // s-publish-36
		{"gone", gone}, // s-publish-38
		{"note", record("zametka", "notes", "zapisi", "5c81-77aa-0d64", "lives", "30", "download", "on", "attachments", "on")}, // s-publish-8: its files too
		{"note-files-kept", record("zametka-kept", "notes", "zapisi", "5c81-77aa-0d64")},                                       // s-publish-23: how many, not which
		{"image", file("sync-board", "carrel/attachments/2026-08/2026-08-10-sync-board.png", false)},                           // s-publish-9
		{"text", file("checklist", "carrel/attachments/2026-08/release-checklist.md", false)},                                  // s-publish-10
		{"download", file("panorama", "carrel/attachments/2026-07/панорама.tif", false)},                                       // s-publish-11
		{"folder", folder}, // s-publish-12
	} {
		t.Run(one.state, func(t *testing.T) {
			app.compare(t, "p-token/"+one.state+".json", app.h.do(get("/p/"+one.token, nil)))
		})
	}
	// The server of «Личный» does not answer (s-publish-37).
	t.Run("unanswered", func(t *testing.T) {
		app.servers["nextcloud"].Refuse("REPORT", 503, "")
		defer app.servers["nextcloud"].Accept("REPORT")
		app.compare(t, "p-token/unanswered.json", app.h.do(get("/p/"+whole, nil)))
	})
	// A folder larger than one page shows (s-publish-31): files only, as the
	// frame's, the listing cut and what it shows of how many said; the cap
	// lowered for this one page.
	t.Run("folder-cut", func(t *testing.T) {
		july := file("july", "carrel/attachments/2026-07", true)
		was := app.h.cfg.FilesMaxEntries
		app.h.cfg.FilesMaxEntries = 3
		defer func() { app.h.cfg.FilesMaxEntries = was }()
		app.compare(t, "p-token/folder-cut.json", app.h.do(get("/p/"+july, nil)))
	})
	// A book with nothing in it (s-publish-35): «Рассылки», found on
	// nextcloud by Update collection list, as a person adds one.
	t.Run("book-empty", func(t *testing.T) {
		nc := app.servers["nextcloud"]
		path := strings.Replace(app.realPath(t, "kontakty"), "kontakty", "rassylki", 1)
		nc.AddAddressBook(carddav.AddressBook{Path: path, Name: "Рассылки"})
		app.byPath[url.PathEscape(path)] = "rassylki"
		if rec := app.h.do(postForm("/app/settings/connections?"+url.Values{
			"op": {"rediscover"}, "account": {app.realAccount(t, "kontakty")}}.Encode(), app.sess, url.Values{})); rec.Code >= 400 {
			t.Fatalf("Update collection list: %d", rec.Code)
		}
		book := record("rassylki", "contacts", "rassylki", "", "lives", "30", "name", "on")
		app.compare(t, "p-token/book-empty.json", app.h.do(get("/p/"+book, nil)))
	})
}
