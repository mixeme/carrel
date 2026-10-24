package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// Strings of the frames the server did not send, or sent otherwise
// (plan/server.md, «Строки кадров, которых сервер не шлёт» and «Кадры, с
// которыми сервер расходится вне записок»). A constant word of a frame is
// the screen's (U-1.3): what depends on the data or the state is the
// server's, and is read here off the frame.

// s-files-1: the attachment folder says so under its title — the badge
// «attachment folder» is the screen's word, that this folder is it is the
// server's.
func TestAttachmentFolderSaysSo(t *testing.T) {
	if !strings.Contains(mockupFrame(t, "s-files-1"), `<span class="tc-badge">attachment folder</span>`) {
		t.Fatal("s-files-1 draws no «attachment folder» under its title: read the frame again")
	}
	f := filesFixture(t, true)
	f.srv.SeedFolder(filesDocs + "carrel/")
	f.srv.SeedFolder(filesDocs + "carrel/attachments/")
	folder := f.at("", "carrel", "attachments", "")
	if rec := f.h.do(postForm("/app/settings/attachments", f.sess, attachAt(t, folder))); rec.Code != http.StatusSeeOther {
		t.Fatalf("attachment folder: %d %s", rec.Code, rec.Body)
	}
	if page := decode[view.FilesPage](t, f.h.do(get(folder, f.sess))); !page.Attachments {
		t.Errorf("the attachment folder does not say it is one")
	}
	if page := decode[view.FilesPage](t, f.h.do(get(f.at("", "carrel", ""), f.sess))); page.Attachments {
		t.Errorf("the folder above says it is the attachment folder")
	}
}

// s-profile-1: the address line of a connection card — «… · mix · plain
// WebDAV, no principal · last answer 5 min ago». «last answer» is the
// screen's word; when, and that the server named no principal, are the
// server's.
func TestConnectionLineIsTheFrame(t *testing.T) {
	lines := regexp.MustCompile(`<div class="tc-mono tc-mono--t11-5-dusty">([^<]*)</div>`).FindAllStringSubmatch(mockupFrame(t, "s-profile-1"), -1)
	var plain string
	for _, l := range lines {
		if strings.Contains(l[1], "no principal") {
			parts := strings.Split(l[1], " · ")
			plain = parts[2]
		}
	}
	if plain == "" {
		t.Fatal("s-profile-1 draws no storage without a principal: read the frame again")
	}

	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a.txt", "alpha")
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		for i := range b.Connections {
			if b.Connections[i].ID == f.id {
				b.Connections[i].FileRoot = filesDocs
			}
		}
	})
	card := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess))).Accounts[0]
	if string(card.Plain) != plain || card.Answered != "" {
		t.Errorf("before any list: plain %q, answered %q; the frame %q and no answer yet", card.Plain, card.Answered, plain)
	}
	if rec := f.h.do(get("/app/contacts/"+f.id+"/"+url.PathEscape(searchBook), f.sess)); rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}
	card = decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess))).Accounts[0]
	if card.Answered == "" {
		t.Errorf("after a list was read: answered %q", card.Answered)
	}

	c := calendarFixture(t, nil)
	if other := decode[view.ConnectionsPage](t, c.h.do(get("/app/settings/connections", c.sess))).Accounts[0]; other.Plain != "" {
		t.Errorf("a server with a principal: plain %q", other.Plain)
	}
}

// s-profile-7: the note of the rename sheet is the frame's — the address
// stays (Р-77: only what the person would not think of); a book says the
// same.
func TestRenameNoteIsTheFrame(t *testing.T) {
	m := regexp.MustCompile(`<div class="tc-msg tc-msg--flush-top2">([^<]*)</div>`).FindStringSubmatch(mockupFrame(t, "s-profile-7"))
	if m == nil {
		t.Fatal("s-profile-7 draws no note: read the frame again")
	}
	f := calendarFixture(t, nil)
	page := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	var href string
	for _, row := range page.Accounts[0].Collections {
		if string(row.Path) == f.col {
			href = menuHref(t, row.Menu, "collection.rename")
		}
	}
	sheet := decode[view.CollectionRenameSheet](t, f.h.do(get(href, f.sess)))
	if said(sheet.Note) != frameText(m[1]) {
		t.Errorf("note = %q,\nthe frame %q", sheet.Note, frameText(m[1]))
	}
	book := frameText(m[1])
	b := contactsFixture(t, false)
	conns := decode[view.ConnectionsPage](t, b.h.do(get("/app/settings/connections", b.sess)))
	for _, row := range conns.Accounts[0].Collections {
		if string(row.Path) == b.col {
			href = menuHref(t, row.Menu, "collection.rename")
		}
	}
	if sheet := decode[view.CollectionRenameSheet](t, b.h.do(get(href, b.sess))); said(sheet.Note) != book {
		t.Errorf("a book's note = %q,\nwant %q", sheet.Note, book)
	}
}

// s-contacts-5: under the frame, what will be saved — «512 × 512 · JPEG ·
// about 90 KB».
func TestPhotoPreviewSaysWhatIsSaved(t *testing.T) {
	m := regexp.MustCompile(`<label>Saved as</label>\s*<input class="tc-in[^"]*" type="text" value="([^"]*)"`).FindStringSubmatch(mockupFrame(t, "s-contacts-5"))
	if m == nil {
		t.Fatal("s-contacts-5 draws no «Saved as»: read the frame again")
	}
	shape := regexp.MustCompile(`^` + strings.NewReplacer("512", `\d+`, "90 KB", `[\d.]+ (?:KB|MB|B)`).Replace(regexp.QuoteMeta(m[1])) + `$`)
	app := newMockupApp(t)
	rec := app.listPath(t, "contacts", "kontakty") + "/4f2b-9c31-a0e7"
	add := actionHref(t, decode[view.ContactEditPage](t, app.h.do(get(rec+"/edit", app.sess))).Photo.Actions, "photo.add")
	if resp := app.h.do(uploadRequest(add, app.sess, false, "anna.jpg", jpegOf(t, halvesImage(240, 180)))); resp.Code != http.StatusSeeOther {
		t.Fatalf("upload: %d", resp.Code)
	}
	page := decode[view.PhotoPreviewPage](t, app.h.do(get(rec+"/photo-preview", app.sess)))
	if !shape.MatchString(string(page.Saved)) {
		t.Errorf("saved as %q, the frame %q", page.Saved, m[1])
	}
}

// s-publish-9: the line of a public file — «image/png · 412 KB · changed
// 2026-08-10»; «changed» is the screen's word, the day the server's.
func TestPublicFileSaysWhenItChanged(t *testing.T) {
	m := regexp.MustCompile(`· changed (\d{4}-\d{2}-\d{2})<`).FindStringSubmatch(mockupFrame(t, "s-publish-9"))
	if m == nil {
		t.Fatal("s-publish-9 draws no «changed <day>»: read the frame again")
	}
	f := pubDocs(t)
	page := publicPage(t, f, publishFile(t, f, "share/readme.txt", false))
	if page.File == nil || !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`).MatchString(string(page.File.Changed)) {
		t.Errorf("file = %+v; the frame «changed %s»", page.File, m[1])
	}
}

// s-dup-1: a member of a pair of tasks says its day the short way of a
// list (C-10) — «12 Aug 18:00», «12 Aug» —, not the long one.
func TestDuplicateMembersSayTheDayShort(t *testing.T) {
	frame := mockupFrame(t, "s-dup-1")
	if !strings.Contains(frame, ">12 Aug 18:00<") || !strings.Contains(frame, ">12 Aug<") {
		t.Fatal("s-dup-1 draws no «12 Aug 18:00» and «12 Aug»: read the frame again")
	}
	f := calendarFixture(t, nil,
		"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Pack the case\nDUE:20260812T150000Z\nEND:VTODO"),
		"t2.ics", ics("BEGIN:VTODO\nUID:t2\nDTSTAMP:20260101T000000Z\nSUMMARY:Pack the case\nDUE;VALUE=DATE:20260812\nEND:VTODO"))
	_ = f.h.do(get(f.path("tasks")+"?show=all", f.sess))
	page := decode[view.DuplicatesPage](t, f.h.do(get("/app/duplicates", f.sess)))
	long := regexp.MustCompile(`August|Due `)
	seen := 0
	for _, g := range page.Groups {
		for _, m := range g.Rows {
			seen++
			if long.MatchString(string(m.Detail)) || !strings.HasPrefix(string(m.Detail), "12 Aug") {
				t.Errorf("member %q: %q, want the short day", m.Title, m.Detail)
			}
		}
	}
	if seen == 0 {
		t.Fatalf("no pair: %+v", page.Groups)
	}
}
