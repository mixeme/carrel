package web_test

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// Attachments (S-20). Up to the limit the file is written into the record and
// travels with it; a larger one goes to the attachment folder and the record
// keeps a link. What another client stored is left exactly as it is.

const attachFiles = davtest.FileRoot + "docs/"

// attachFixture is a calendar and a file collection on one connection. limit
// lowers the collection's max-resource-size the way a server does; folder, when
// given, is the attachment folder under the file collection.
func attachFixture(t *testing.T, limit int64, folder []string, events ...string) davFixture {
	t.Helper()
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	work := davtest.CalendarHome + "work/"
	srv.AddCalendar(caldav.Calendar{Path: work, Name: "Work"})
	for i := 0; i+1 < len(events); i += 2 {
		srv.Seed(work+events[i], events[i+1])
	}
	srv.SeedFolder(attachFiles)
	f := connectFixture(t, srv, work, nil)
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		b.Collections = append(b.Collections, sealed.Collection{
			AccountID: f.id, Path: attachFiles, Name: "Docs", Kinds: []view.Kind{view.KindFile},
		})
		for i := range b.Collections {
			if b.Collections[i].Path == work {
				b.Collections[i].MaxResourceSize = limit
			}
		}
		if folder != nil {
			b.Attachments = &sealed.AttachmentFolder{AccountID: f.id, Path: attachFiles, Folder: folder}
		}
	})
	return f
}

func attachEvent(uid, summary, extra string) string {
	return ics("BEGIN:VEVENT\nUID:" + uid + "\nDTSTAMP:20260101T000000Z\nDTSTART:20260810T100000Z\nSUMMARY:" +
		summary + "\n" + extra + "END:VEVENT")
}

func eventAt(t *testing.T, f davFixture, uid string) view.EventPage {
	t.Helper()
	return decode[view.EventPage](t, f.h.do(get(f.path("calendar")+"/"+uid, f.sess)))
}

// folderRows is what the files section shows in a folder of the file
// collection.
func folderRows(t *testing.T, f davFixture, segs ...string) []view.FileRow {
	t.Helper()
	href := "/app/files/" + f.id + "/" + url.PathEscape(attachFiles) + "/"
	for _, s := range segs {
		href += url.PathEscape(s) + "/"
	}
	return decode[view.FilesPage](t, f.h.do(get(href, f.sess))).Rows
}

func rowAction(t *testing.T, row view.AttachRow, id string) string {
	t.Helper()
	return actionHref(t, row.Actions, id)
}

// A file that fits is written into the record: one object, one PUT, and a
// client with only the calendar gets the file with the event.
func TestSmallAttachmentGoesIntoTheRecord(t *testing.T) {
	f := attachFixture(t, 0, nil, "e1.ics", attachEvent("e1", "Sync-up", ""))
	rec := f.path("calendar") + "/e1"
	href := actionHref(t, eventAt(t, f, "e1").Actions, "attach.add")

	resp := f.h.do(uploadRequest(href, f.sess, false, "report.txt", []byte("small file")))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != rec {
		t.Fatalf("attach: %d %s %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	mustContain(t, body(t, f, "e1.ics"), "ATTACH;ENCODING=BASE64;", "FILENAME=report.txt", "VALUE=BINARY:"+
		base64.StdEncoding.EncodeToString([]byte("small file")))

	page := eventAt(t, f, "e1")
	if len(page.Attach) != 1 {
		t.Fatalf("attachments: %+v", page.Attach)
	}
	row := page.Attach[0]
	if !row.Inline || row.Name != "report.txt" || row.Size != "10 B" || row.State != view.RecordOK || row.Href == "" {
		t.Errorf("row = %+v", row)
	}
	got := f.h.do(get(row.Href, f.sess))
	if got.Code != http.StatusOK || got.Body.String() != "small file" {
		t.Fatalf("open: %d %q", got.Code, got.Body.String())
	}
	h := got.Header()
	if !strings.HasPrefix(h.Get("Content-Disposition"), "attachment") || !strings.Contains(h.Get("Content-Security-Policy"), "sandbox") ||
		h.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("headers = %v", h)
	}
	if none := f.h.do(get(strings.TrimSuffix(row.Href, "/0")+"/7", f.sess)); none.Code != http.StatusNotFound {
		t.Errorf("no such attachment: %d", none.Code)
	}
}

// Past the limit the file goes to the folder under a name made of the date of
// the record, its title and the name the file came with; a name already taken
// gets a number, and nothing is overwritten.
func TestLargeAttachmentGoesToTheFolder(t *testing.T) {
	f := attachFixture(t, 1024, []string{"carrel"}, "e1.ics", attachEvent("e1", "Синк-ап по релизу", ""))
	data := bytes.Repeat([]byte("x"), 2048)

	href := actionHref(t, eventAt(t, f, "e1").Actions, "attach.add")
	if resp := f.h.do(uploadRequest(href, f.sess, true, "report.pdf", data)); resp.Code != http.StatusSeeOther {
		t.Fatalf("attach: %d %s", resp.Code, resp.Body.String())
	}
	const want = "2026-08-10 Синк-ап по релизу - report.pdf"
	if got, ok := f.srv.File(attachFiles + "carrel/" + want); !ok || len(got) != len(data) {
		t.Fatalf("the folder holds %+v", folderRows(t, f, "carrel"))
	}
	doc := body(t, f, "e1.ics")
	mustContain(t, doc, "ATTACH;", "FILENAME=report.pdf", "SIZE=2048", "/docs/carrel/2026-08-10%20")
	if strings.Contains(doc, base64.StdEncoding.EncodeToString(data[:16])) {
		t.Errorf("the file went into the record as well:\n%s", doc)
	}

	page := eventAt(t, f, "e1")
	row := page.Attach[0]
	if row.Inline || row.Name != want || row.Size != "2.0 KB" || row.Href == "" {
		t.Fatalf("row = %+v", row)
	}
	if got := f.h.do(get(row.Href, f.sess)); got.Code != http.StatusOK || got.Body.Len() != len(data) {
		t.Errorf("open: %d %d bytes", got.Code, got.Body.Len())
	}

	href = actionHref(t, page.Actions, "attach.add")
	if resp := f.h.do(uploadRequest(href, f.sess, false, "report.pdf", data)); resp.Code != http.StatusSeeOther {
		t.Fatalf("second attach: %d %s", resp.Code, resp.Body.String())
	}
	if got, ok := f.srv.File(attachFiles + "carrel/2026-08-10 Синк-ап по релизу - report-2.pdf"); !ok || len(got) != len(data) {
		t.Errorf("the second file is not beside the first: %+v", folderRows(t, f, "carrel"))
	}
	if got, _ := f.srv.File(attachFiles + "carrel/" + want); len(got) != len(data) {
		t.Errorf("the first file was overwritten")
	}
	if names := len(folderRows(t, f, "carrel")); names != 2 {
		t.Errorf("the folder holds %d files, want 2", names)
	}
}

// A title a file system would refuse becomes a name it takes, and a long one
// is cut; the extension stays.
func TestAttachmentNameIsCleaned(t *testing.T) {
	long := strings.Repeat("я", 90)
	f := attachFixture(t, 1024, []string{"carrel"},
		"e2.ics", attachEvent("e2", `План: A/B "финал"`, ""),
		"e3.ics", attachEvent("e3", long, ""))
	data := bytes.Repeat([]byte("x"), 2048)
	for _, uid := range []string{"e2", "e3"} {
		href := actionHref(t, eventAt(t, f, uid).Actions, "attach.add")
		if resp := f.h.do(uploadRequest(href, f.sess, false, "report.pdf", data)); resp.Code != http.StatusSeeOther {
			t.Fatalf("%s: %d %s", uid, resp.Code, resp.Body.String())
		}
	}
	rows := folderRows(t, f, "carrel")
	if len(rows) != 2 {
		t.Fatalf("the folder holds %+v", rows)
	}
	for _, row := range rows {
		name := string(row.Name)
		if strings.ContainsAny(name, `/\:*?"<>|`) || !strings.HasPrefix(name, "2026-08-10 ") || !strings.HasSuffix(name, ".pdf") {
			t.Errorf("name = %q", name)
		}
		if len(name) > 200 {
			t.Errorf("name of %d bytes: %q", len(name), name)
		}
	}
}

// Without a folder nothing is written at all: the record is not left with a
// file it cannot point at, and the screen says where to choose one.
func TestLargeAttachmentWithoutAFolderWritesNothing(t *testing.T) {
	f := attachFixture(t, 1024, nil, "e1.ics", attachEvent("e1", "Sync-up", ""))
	href := actionHref(t, eventAt(t, f, "e1").Actions, "attach.add")
	page := decode[view.EventPage](t, f.h.do(uploadRequest(href, f.sess, false, "report.pdf", bytes.Repeat([]byte("x"), 2048))))
	if !hasNotice(page.Notices, "Nothing was attached: no folder for attachments is chosen") {
		t.Fatalf("notices = %+v", page.Notices)
	}
	if n := f.srv.CountRequests("PUT", davtest.CalendarHome); n != 0 {
		t.Errorf("%d PUTs, want none", n)
	}
	if strings.Contains(body(t, f, "e1.ics"), "ATTACH") {
		t.Errorf("the record carries an attachment")
	}
}

// The version the page showed is the version attached to (S-8).
func TestAttachOnAChangedRecordWritesNothing(t *testing.T) {
	f := attachFixture(t, 0, nil, "e1.ics", attachEvent("e1", "Sync-up", ""))
	href := actionHref(t, eventAt(t, f, "e1").Actions, "attach.add")
	f.srv.Seed(f.col+"e1.ics", attachEvent("e1", "Sync-up, moved", ""))

	page := decode[view.EventPage](t, f.h.do(uploadRequest(href, f.sess, false, "report.txt", []byte("x"))))
	if !hasNotice(page.Notices, "Changed elsewhere, so nothing was attached") {
		t.Fatalf("notices = %+v", page.Notices)
	}
	if strings.Contains(body(t, f, "e1.ics"), "ATTACH") {
		t.Errorf("the record carries an attachment")
	}
}

// What another client stored stays as it is, whatever its size: a large inline
// attachment is never moved to a folder, and a link elsewhere is shown and
// never fetched. A new attachment of a record already over the limit goes to
// the folder.
func TestForeignAttachmentsAreLeftAsStored(t *testing.T) {
	foreign := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte("F"), 1500))
	extra := "ATTACH;FMTTYPE=text/markdown;FILENAME=notes.md:https://elsewhere.example.org/notes.md\n" +
		"ATTACH;ENCODING=BASE64;VALUE=BINARY;FMTTYPE=image/png;FILENAME=big.png:" + foreign + "\n"
	f := attachFixture(t, 1024, []string{"carrel"}, "e1.ics", attachEvent("e1", "Sync-up", extra))

	page := eventAt(t, f, "e1")
	if len(page.Attach) != 2 {
		t.Fatalf("attachments: %+v", page.Attach)
	}
	if link := page.Attach[0]; link.Inline || link.Href != "" || link.Name != "notes.md" {
		t.Errorf("a link elsewhere: %+v", link)
	}
	if inline := page.Attach[1]; !inline.Inline || inline.Size != "1.5 KB" {
		t.Errorf("foreign inline: %+v", inline)
	}

	href := actionHref(t, page.Actions, "attach.add")
	if resp := f.h.do(uploadRequest(href, f.sess, false, "tiny.txt", []byte("tiny"))); resp.Code != http.StatusSeeOther {
		t.Fatalf("attach: %d %s", resp.Code, resp.Body.String())
	}
	doc := body(t, f, "e1.ics")
	if !strings.Contains(doc, foreign) {
		t.Errorf("the foreign attachment changed:\n%s", doc)
	}
	if !strings.Contains(doc, "FILENAME=tiny.txt") || !strings.Contains(doc, "/docs/carrel/") {
		t.Errorf("the new attachment did not go to the folder:\n%s", doc)
	}
	if _, ok := f.srv.File(attachFiles + "carrel/2026-08-10 Sync-up - tiny.txt"); !ok {
		t.Errorf("the folder holds %+v", folderRows(t, f, "carrel"))
	}
}

// Removing a link leaves the file where it is — another record may point at it
// — and the label says so; an inline attachment goes with its only copy.
func TestDetachLinkKeepsTheFile(t *testing.T) {
	f := attachFixture(t, 0, []string{"carrel"}, "e1.ics", attachEvent("e1", "Sync-up", ""))
	f.srv.SeedFile(attachFiles+"carrel/keep.txt", "keep")
	uri := f.srv.URL + attachFiles + "carrel/keep.txt"
	f.srv.Seed(f.col+"e1.ics", attachEvent("e1", "Sync-up",
		"ATTACH;FMTTYPE=text/plain;FILENAME=keep.txt:"+uri+"\n"+
			"ATTACH;ENCODING=BASE64;VALUE=BINARY;FILENAME=inline.txt:"+base64.StdEncoding.EncodeToString([]byte("inline"))+"\n"))

	page := eventAt(t, f, "e1")
	link, inline := page.Attach[0], page.Attach[1]
	if link.Href == "" || link.Inline {
		t.Fatalf("a link into a storage of the catalog: %+v", link)
	}
	if got := f.h.do(get(link.Href, f.sess)); got.Code != http.StatusOK || got.Body.String() != "keep" {
		t.Errorf("open: %d %q", got.Code, got.Body.String())
	}
	remove := rowAction(t, link, "attach.remove")
	if link.Actions[0].Label != "Remove link, keep the file" || link.Actions[0].Danger {
		t.Errorf("removing a link: %+v", link.Actions[0])
	}
	if !inline.Actions[0].Danger {
		t.Errorf("removing an inline attachment is the end of it: %+v", inline.Actions[0])
	}

	if resp := f.h.do(postForm(remove, f.sess, url.Values{})); resp.Code != http.StatusSeeOther {
		t.Fatalf("detach: %d %s", resp.Code, resp.Body.String())
	}
	doc := body(t, f, "e1.ics")
	if strings.Contains(doc, "keep.txt") || !strings.Contains(doc, "inline.txt") {
		t.Fatalf("after removing the link:\n%s", doc)
	}
	if got, ok := f.srv.File(attachFiles + "carrel/keep.txt"); !ok || got != "keep" {
		t.Errorf("the file went with the link")
	}

	page = eventAt(t, f, "e1")
	if resp := f.h.do(postForm(rowAction(t, page.Attach[0], "attach.remove"), f.sess, url.Values{})); resp.Code != http.StatusSeeOther {
		t.Fatalf("detach inline: %d %s", resp.Code, resp.Body.String())
	}
	if doc := body(t, f, "e1.ics"); strings.Contains(doc, "ATTACH") {
		t.Errorf("an attachment stayed:\n%s", doc)
	}
	if len(eventAt(t, f, "e1").Attach) != 0 {
		t.Errorf("the block still has rows")
	}
}

// A read-only collection is not offered attaching at all, and the address
// refuses it (C-2.1: the same rule as every other write there).
func TestReadOnlyCollectionHasNoAttaching(t *testing.T) {
	f := attachFixture(t, 0, []string{"carrel"}, "e1.ics", attachEvent("e1", "Sync-up",
		"ATTACH;ENCODING=BASE64;VALUE=BINARY;FILENAME=inline.txt:aW5saW5l\n"))
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		for i := range b.Collections {
			if b.Collections[i].Path == f.col {
				b.Collections[i].ReadOnly = true
			}
		}
	})
	page := eventAt(t, f, "e1")
	for _, a := range page.Actions {
		if a.ID == "attach.add" {
			t.Errorf("a read-only collection offers attaching")
		}
	}
	if len(page.Attach) != 1 || len(page.Attach[0].Actions) != 0 {
		t.Errorf("attachments of a read-only collection: %+v", page.Attach)
	}
	href := f.path("calendar") + "/e1/edit?" + url.Values{"op": {"attach"}, "etag": {"x"}}.Encode()
	if resp := f.h.do(uploadRequest(href, f.sess, false, "report.txt", []byte("x"))); resp.Code != http.StatusNotFound {
		t.Errorf("attach into a read-only collection: %d", resp.Code)
	}
}

// The folder is chosen once, in the one tree of the product, and the boundary
// is shown beside it and not editable (U-6.12).
func TestAttachmentsSettingsChooseTheFolder(t *testing.T) {
	f := attachFixture(t, 0, nil)
	page := decode[view.AttachmentsPage](t, f.h.do(get("/app/settings/attachments", f.sess)))
	if len(page.Folder.Nodes) != 1 || page.Folder.Nodes[0].Label != "Docs" {
		t.Fatalf("tree = %+v", page.Folder)
	}
	if !strings.Contains(string(page.Note), "does not delete its file") {
		t.Errorf("note = %q", page.Note)
	}
	limits := page.Form.Sets[0]
	if string(limits.Rows[0].Label) != "Up to 2 MB" || !limits.Rows[0].ReadOnly {
		t.Errorf("the boundary is shown and not editable: %+v", limits.Rows[0])
	}

	to := page.Folder.Nodes[0].ID
	if resp := f.h.do(postForm("/app/settings/attachments", f.sess, url.Values{"to": {to}, "sub": {"carrel/attachments"}})); resp.Code != http.StatusSeeOther {
		t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
	}
	af := catalogOf(t, f).Attachments
	if af == nil || af.AccountID != f.id || af.Path != attachFiles || strings.Join(af.Folder, "/") != "carrel/attachments" {
		t.Fatalf("stored = %+v", af)
	}

	for _, bad := range []url.Values{{"to": {to}, "sub": {".."}}, {"to": {""}}} {
		page = decode[view.AttachmentsPage](t, f.h.do(postForm("/app/settings/attachments", f.sess, bad)))
		if len(page.Form.Errors) == 0 {
			t.Errorf("%v was taken", bad)
		}
	}
	if af := catalogOf(t, f).Attachments; af == nil || strings.Join(af.Folder, "/") != "carrel/attachments" {
		t.Errorf("a refused form changed the folder: %+v", af)
	}
}
