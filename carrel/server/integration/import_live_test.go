//go:build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/view"
)

// S-30.2 for S-19: a file imported into a live collection and the collection
// handed back as a file, through the request layer. The file holds a series
// whose identity the calendar already has — its instance too must go under the
// new one — a new event, and a task the calendar of events does not take; the
// book file holds a card whose identity is taken. Nothing that was there
// changes, and the export carries everything in one envelope.
func TestRadicaleImportExport(t *testing.T) {
	runImportExport(t, radicaleEndpoint(t), radicaleUser, radicalePass)
}

func TestBaikalImportExport(t *testing.T) {
	endpoint, user, pass := baikalConfig(t)
	runImportExport(t, endpoint, user, pass)
}

func runImportExport(t *testing.T, endpoint, user, pass string) {
	ctx := context.Background()
	suffix := fmt.Sprintf("carrel-s19-%d-%d", os.Getpid(), time.Now().Unix()%100000)
	client := liveClient(t, endpoint, user, pass)
	calHome, bookHome := liveHomes(ctx, t, endpoint, user, pass)

	calPath := calHome + suffix + "-cal/"
	bookPath := bookHome + suffix + "-book/"
	liveRequest(ctx, t, endpoint, user, pass, "MKCALENDAR", calPath, `<?xml version="1.0" encoding="utf-8"?>
<C:mkcalendar xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set><D:prop>
    <D:displayname>Carrel S-19</D:displayname>
    <C:supported-calendar-component-set><C:comp name="VEVENT"/><C:comp name="VTODO"/></C:supported-calendar-component-set>
  </D:prop></D:set>
</C:mkcalendar>`)
	t.Cleanup(func() { _ = client.Delete(context.Background(), calPath, "") })
	liveRequest(ctx, t, endpoint, user, pass, "MKCOL", bookPath, `<?xml version="1.0" encoding="utf-8"?>
<D:mkcol xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <D:set><D:prop>
    <D:resourcetype><D:collection/><C:addressbook/></D:resourcetype>
    <D:displayname>Carrel S-19 book</D:displayname>
  </D:prop></D:set>
</D:mkcol>`)
	t.Cleanup(func() { _ = client.Delete(context.Background(), bookPath, "") })

	series := "series-" + suffix
	seedSeries(ctx, t, client, calPath, series)
	ada := "ada-" + suffix
	seedCard(ctx, t, client, bookPath, ada)
	task := "task-" + suffix
	todo := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other client//EN\r\nBEGIN:VTODO\r\nUID:" + task +
		"\r\nDTSTAMP:20260101T000000Z\r\nSUMMARY:Write report\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"
	if _, err := client.PutCreate(ctx, calPath+task+".ics", dav.Typed(strings.NewReader(todo), "text/calendar; charset=utf-8")); err != nil {
		t.Fatalf("seed task: %v", err)
	}

	app := newLiveApp(t)
	connID := connectLive(t, app, endpoint, user, pass)
	blob := liveCatalog(t, app)
	cal, book := catalogEntry(t, blob, calPath), catalogEntry(t, blob, bookPath)
	events := "/app/calendar/" + connID + "/" + url.PathEscape(cal.Path)
	contacts := "/app/contacts/" + connID + "/" + url.PathEscape(book.Path)

	t.Run("calendar", func(t *testing.T) {
		fresh := "fresh-" + suffix
		file := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other program//EN\r\n" +
			"BEGIN:VEVENT\r\nUID:" + series + "\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20261106T100000Z\r\n" +
			"DTEND:20261106T103000Z\r\nRRULE:FREQ=WEEKLY;COUNT=3\r\nSUMMARY:Standup imported\r\nX-CUSTOM:keep-me\r\nEND:VEVENT\r\n" +
			"BEGIN:VEVENT\r\nUID:" + series + "\r\nDTSTAMP:20260101T000000Z\r\nRECURRENCE-ID:20261113T100000Z\r\n" +
			"DTSTART:20261113T120000Z\r\nDTEND:20261113T123000Z\r\nSUMMARY:Standup imported moved\r\nEND:VEVENT\r\n" +
			"BEGIN:VEVENT\r\nUID:" + fresh + "\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20261110T090000Z\r\nSUMMARY:Ковка\r\nEND:VEVENT\r\n" +
			"BEGIN:VTODO\r\nUID:buy-" + suffix + "\r\nDTSTAMP:20260101T000000Z\r\nSUMMARY:Купить уголь\r\nEND:VTODO\r\n" +
			"END:VCALENDAR\r\n"
		page := liveDecode[view.ImportPage](t, "calendar wizard", app.get(events+"/import"))
		page = liveDecode[view.ImportPage](t, "calendar file", app.importFile(liveAction(t, page.Actions, "import.choose"), "other.ics", []byte(file)))
		if page.Step != view.ImportStepPreview || len(page.Notices) > 0 {
			t.Fatalf("preview: step %q, notices %+v", page.Step, page.Notices)
		}
		if row := liveRow(t, page.Preview, "Carrel S-19", "Standup imported"); !strings.Contains(string(row[2]), "Taken here") {
			t.Errorf("the taken series in the preview = %q", row)
		}
		if row := liveRow(t, page.Preview, "Carrel S-19", "Ковка"); string(row[2]) != "New" {
			t.Errorf("the new event in the preview = %q", row)
		}
		liveRow(t, page.Preview, "Skipped", "VTODO")

		page = liveDecode[view.ImportPage](t, "calendar run", app.post(liveAction(t, page.Actions, "import.run"), url.Values{}))
		if r := page.Report; r == nil || r.Created != 2 || r.Renamed != 1 || r.Skipped != 1 {
			t.Fatalf("report = %+v", page.Report)
		}
		note := string(liveRow(t, page.Report.Rows, "Created", "Standup imported")[2])
		m := regexp.MustCompile(`Created under ([^:]+):`).FindStringSubmatch(note)
		if m == nil {
			t.Fatalf("the renamed series does not say its identity: %q", note)
		}
		got, _ := rawBody(ctx, t, client, calPath+m[1]+".ics")
		mustHave(t, "renamed series", got, "RRULE:FREQ=WEEKLY;COUNT=3", "RECURRENCE-ID:20261113T100000Z", "SUMMARY:Standup imported moved", "X-CUSTOM:keep-me")
		if n := strings.Count(got, "UID:"+m[1]); n != 2 || strings.Contains(got, "UID:"+series+"\n") {
			t.Errorf("the series and its instance do not share the new identity (%d):\n%s", n, got)
		}
		got, _ = rawBody(ctx, t, client, calPath+series+".ics")
		mustHave(t, "the series that was there", got, "SUMMARY:Standup moved")
		if strings.Contains(got, "imported") {
			t.Errorf("an import changed the series that was there:\n%s", got)
		}
		got, _ = rawBody(ctx, t, client, calPath+fresh+".ics")
		mustHave(t, "new event", got, "UID:"+fresh, "SUMMARY:Ковка")
		if _, _, err := client.Get(ctx, calPath+"buy-"+suffix+".ics", nil); !dav.IsStatus(err, http.StatusNotFound) {
			t.Errorf("the task a calendar of events does not take was written: %v", err)
		}

		// The agenda's Export is the kind; whole=1 is every component.
		body := liveDownload(t, app, events+"/export?whole=1", ".ics")
		if strings.Count(body, "BEGIN:VCALENDAR") != 1 || strings.Count(body, "BEGIN:VTODO") != 1 {
			t.Errorf("whole export is not one envelope with the task once:\n%s", body)
		}
		mustHave(t, "whole export", body, "UID:"+series, "UID:"+m[1], "UID:"+fresh, "UID:"+task, "RECURRENCE-ID:20261013T100000Z")
		body = liveDownload(t, app, events+"/export", ".ics")
		if strings.Contains(body, "UID:"+task) || !strings.Contains(body, "UID:"+fresh) {
			t.Errorf("the events export:\n%s", body)
		}
	})

	t.Run("contacts", func(t *testing.T) {
		grace := "grace-" + suffix
		file := "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:" + ada + "\r\nFN:Ada Imported\r\nEMAIL:ada@import.carrel.test\r\nEND:VCARD\r\n" +
			"BEGIN:VCARD\r\nVERSION:3.0\r\nUID:" + grace + "\r\nFN:Grace Hopper\r\nX-CUSTOM:keep-me\r\nEND:VCARD\r\n"
		page := liveDecode[view.ImportPage](t, "book wizard", app.get(contacts+"/import"))
		page = liveDecode[view.ImportPage](t, "book file", app.importFile(liveAction(t, page.Actions, "import.choose"), "cards.vcf", []byte(file)))
		page = liveDecode[view.ImportPage](t, "book run", app.post(liveAction(t, page.Actions, "import.run"), url.Values{}))
		if r := page.Report; r == nil || r.Created != 2 || r.Renamed != 1 {
			t.Fatalf("report = %+v, notices %+v", page.Report, page.Notices)
		}
		got, _ := rawBody(ctx, t, client, bookPath+ada+".vcf")
		if strings.Contains(got, "Imported") {
			t.Errorf("an import changed the card that was there:\n%s", got)
		}
		got, _ = rawBody(ctx, t, client, bookPath+grace+".vcf")
		mustHave(t, "new card", got, "UID:"+grace, "FN:Grace Hopper", "X-CUSTOM:keep-me")

		body := liveDownload(t, app, contacts+"/export", ".vcf")
		if n := strings.Count(body, "BEGIN:VCARD"); n != 3 {
			t.Errorf("the book export holds %d cards:\n%s", n, body)
		}
		mustHave(t, "book export", body, "UID:"+ada, "UID:"+grace, "FN:Ada Imported", "X-CUSTOM:keep-me")
	})
}

// S-19: notes have a second source — a folder of Markdown files on another
// server of the same person, read through the guard. SFTPGo holds the folder,
// Baikal the notebook; the notebook then goes out as one .md.
func TestBaikalSFTPGoNotesFromFolder(t *testing.T) {
	bEndpoint, bUser, bPass := baikalConfig(t)
	sEndpoint, sUser, sPass := sftpgoConfig(t)
	sClient, files := sftpgoClient(t)
	ctx := context.Background()
	suffix := fmt.Sprintf("carrel-s19-%d-%d", os.Getpid(), time.Now().Unix()%100000)

	folder := "/" + suffix + "-notes/"
	if err := files.MkCol(ctx, folder); err != nil {
		t.Fatalf("mkcol: %v", err)
	}
	t.Cleanup(func() { _ = sClient.Delete(context.Background(), folder, "") })
	for name, body := range map[string]string{
		"kovka.md":  "---\ntitle: Ковка " + suffix + "\ncategories: work\n---\n\nПервая строка\n",
		"plain.md":  "# Уголь " + suffix + "\n\nЗаказать мешок.\n",
		"photo.jpg": "not a note",
	} {
		if _, err := files.UploadCreate(ctx, folder+name, strings.NewReader(body)); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}

	bClient := liveClient(t, bEndpoint, bUser, bPass)
	calHome, _ := liveHomes(ctx, t, bEndpoint, bUser, bPass)
	notebook := calHome + suffix + "-notebook/"
	liveRequest(ctx, t, bEndpoint, bUser, bPass, "MKCALENDAR", notebook, `<?xml version="1.0" encoding="utf-8"?>
<C:mkcalendar xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set><D:prop>
    <D:displayname>Carrel S-19 notebook</D:displayname>
    <C:supported-calendar-component-set><C:comp name="VJOURNAL"/></C:supported-calendar-component-set>
  </D:prop></D:set>
</C:mkcalendar>`)
	t.Cleanup(func() { _ = bClient.Delete(context.Background(), notebook, "") })

	app := newLiveApp(t)
	baiID := connectLive(t, app, bEndpoint, bUser, bPass)
	connectLive(t, app, sEndpoint, sUser, sPass)
	nb := catalogEntry(t, liveCatalog(t, app), notebook)
	notes := "/app/notes/" + baiID + "/" + url.PathEscape(nb.Path)

	page := liveDecode[view.ImportPage](t, "notes wizard", app.get(notes+"/import"))
	if page.Source.Folder == nil {
		t.Fatalf("no folder offered: %+v", page.Source)
	}
	source := ""
	for _, o := range page.Source.Folder.Options {
		u, err := url.Parse(o.ID)
		if err == nil && strings.HasSuffix(strings.TrimSuffix(u.Query().Get("path"), "/"), strings.TrimSuffix(folder, "/")) {
			source = o.ID
		}
	}
	if source == "" {
		t.Fatalf("the folder %s is not among %+v", folder, page.Source.Folder.Options)
	}
	page = liveDecode[view.ImportPage](t, "folder", app.post(source, url.Values{}))
	if page.Step != view.ImportStepPreview || len(page.Preview) == 0 || page.Preview[0].Count != 2 {
		t.Fatalf("preview: step %q, tables %+v, notices %+v", page.Step, page.Preview, page.Notices)
	}
	page = liveDecode[view.ImportPage](t, "notes run", app.post(liveAction(t, page.Actions, "import.run"), url.Values{}))
	if r := page.Report; r == nil || r.Created != 2 {
		t.Fatalf("report = %+v, notices %+v", page.Report, page.Notices)
	}

	body := liveDownload(t, app, notes+"/export", ".md")
	mustHave(t, "notebook export", body, "Ковка "+suffix, "Уголь "+suffix, "Заказать мешок.", "work")
}

// importFile sends a file the way a page without scripts does: _csrf first,
// then the file part.
func (a *liveApp) importFile(path, name string, data []byte) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("_csrf", a.sess.CSRFToken())
	part, _ := w.CreateFormFile("file", name)
	_, _ = part.Write(data)
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: a.sess.ID()})
	rec := httptest.NewRecorder()
	a.srv.Handler().ServeHTTP(rec, req)
	return rec
}

// liveRow is the row of a preview or a report whose first column holds what.
func liveRow(t *testing.T, tables []view.ImportTable, rubric, what string) []view.Text {
	t.Helper()
	for _, tb := range tables {
		if !strings.Contains(string(tb.Rubric), rubric) {
			continue
		}
		for _, row := range tb.Rows {
			if strings.Contains(string(row[0]), what) {
				return row
			}
		}
	}
	t.Fatalf("no row %q under %q in %+v", what, rubric, tables)
	return nil
}

// liveDownload is an export: an attachment of that extension in a sandbox,
// unfolded so a folded line still matches.
func liveDownload(t *testing.T, app *liveApp, path, ext string) string {
	t.Helper()
	rec := app.get(path)
	h := rec.Header()
	if rec.Code != http.StatusOK || !strings.HasPrefix(h.Get("Content-Disposition"), "attachment") ||
		!strings.Contains(h.Get("Content-Disposition"), ext) || !strings.Contains(h.Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("export %s: %d %v\n%s", path, rec.Code, h, rec.Body.String())
	}
	return strings.ReplaceAll(strings.ReplaceAll(rec.Body.String(), "\r\n ", ""), "\r\n", "\n")
}
