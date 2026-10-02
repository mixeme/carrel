//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/provider/file"
	"carrel.dev/carrel/server/view"
)

// S-30.2 for S-20, on two servers at once: the calendar is on a CalDAV server
// and the attachment folder on the file server, which is how a person's two
// connections actually stand. A file that fits goes into the record and comes
// back byte for byte; a larger one is streamed to the folder under a readable
// name and the record keeps a link Carrel opens. What another client wrote
// inline is not touched by any write of ours, and removing a link leaves its
// file where it is.

func TestBaikalAttachments(t *testing.T) {
	endpoint, user, pass := baikalConfig(t)
	sEndpoint, sUser, sPass := sftpgoConfig(t)
	runAttachments(t, endpoint, user, pass, sEndpoint, sUser, sPass)
}

// The folder on OpenCloud: there the storage is a space — a path under the
// host, not its root — and the server writes nothing whose length it was not
// told (docs/servers.md, SV-15). So the small file still travels inside the
// record, and the large one is not written at all: the screen says why, and
// the folder stays as it was. Carrel does not keep the file to measure it —
// there is no ceiling on a file (C-12, S-17.1).
func TestBaikalOpenCloudAttachments(t *testing.T) {
	endpoint, user, pass := baikalConfig(t)
	sEndpoint, sUser, sPass := opencloudConfig(t)
	sClient, files := liveFiles(t, sEndpoint, sUser, sPass)
	ctx := context.Background()
	suffix := fmt.Sprintf("carrel-s20-oc-%d-%d", os.Getpid(), time.Now().Unix()%100000)

	client := liveClient(t, endpoint, user, pass)
	calHome, _ := liveHomes(ctx, t, endpoint, user, pass)
	calPath := calHome + suffix + "-cal/"
	calPath = makeCollection(ctx, t, client, endpoint, user, pass, "MKCALENDAR", calPath, `<?xml version="1.0" encoding="utf-8"?>
<C:mkcalendar xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set><D:prop>
    <D:displayname>Carrel S-20 OpenCloud</D:displayname>
    <C:supported-calendar-component-set><C:comp name="VEVENT"/></C:supported-calendar-component-set>
  </D:prop></D:set>
</C:mkcalendar>`)

	uid := "event-" + suffix
	seedRecord(ctx, t, client, calPath+uid+".ics", "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other client//EN\r\n"+
		"BEGIN:VEVENT\r\nUID:"+uid+"\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20260810T100000Z\r\n"+
		"SUMMARY:Синк-ап "+suffix+"\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")

	app := newLiveApp(t)
	calID := connectLive(t, app, endpoint, user, pass)
	sID := connectLive(t, app, sEndpoint, sUser, sPass)
	cal := catalogEntry(t, liveCatalog(t, app), calPath)
	record := "/app/calendar/" + calID + "/" + url.PathEscape(cal.Path) + "/" + uid

	sub := suffix + "-attach"
	settings := liveDecode[view.AttachmentsPage](t, "attachment settings", app.get("/app/settings/attachments"))
	to := liveAttachCollection(t, settings, sID)
	redirected(t, "choose the folder", app.post("/app/settings/attachments", url.Values{"collection": {to}, "folder": {sub}}))
	af := liveCatalog(t, app).Attachments
	if af == nil {
		t.Fatal("the folder was not stored")
	}
	folder := strings.TrimSuffix(af.Path, "/") + "/" + sub + "/"
	t.Logf("space %q, attachment folder %q", af.Path, folder)
	t.Cleanup(func() { _ = sClient.Delete(context.Background(), folder, "") })

	// Small enough to travel in the record: nothing is asked of the file server.
	small := bytes.Repeat([]byte{0x00, 0x01, 0x02, 0xfe}, 800)
	page := liveDecode[view.EventPage](t, "event", app.get(record))
	redirected(t, "attach a small file", app.attach(liveAction(t, page.Actions, "attach.add"), "notes.bin", small))
	raw, _ := rawBody(ctx, t, client, calPath+uid+".ics")
	mustHave(t, "the record after a small attachment", raw, "FILENAME=notes.bin", liveBase64(small))

	// Larger than the record takes, and nothing said how long it is.
	big := bytes.Repeat([]byte("carrel-attachment-"), 150000)
	page = liveDecode[view.EventPage](t, "event with one attachment", app.get(record))
	refused := liveDecode[view.EventPage](t, "attach a large file",
		app.attach(liveAction(t, page.Actions, "attach.add"), "report.pdf", big))
	if !hasLiveNotice(refused.Notices, "needs the size of a file") {
		t.Fatalf("notices = %+v", refused.Notices)
	}
	if names := liveNames(ctx, t, files, folder); len(names) != 0 {
		t.Errorf("the folder holds %v, want nothing written", names)
	}
	if raw, _ = rawBody(ctx, t, client, calPath+uid+".ics"); strings.Contains(raw, "report.pdf") {
		t.Errorf("the record points at a file that was never written")
	}
}

// liveFiles is a file provider over one connection.
func liveFiles(t *testing.T, endpoint, user, pass string) (*dav.Client, *file.Provider) {
	t.Helper()
	client := liveClient(t, endpoint, user, pass)
	p, err := file.New(client)
	if err != nil {
		t.Fatal(err)
	}
	return client, p
}

func hasLiveNotice(notices []view.Notice, part string) bool {
	for _, n := range notices {
		if strings.Contains(string(n.Title), part) || strings.Contains(string(n.Body), part) {
			return true
		}
	}
	return false
}

func TestRadicaleAttachments(t *testing.T) {
	sEndpoint, sUser, sPass := sftpgoConfig(t)
	runAttachments(t, radicaleEndpoint(t), radicaleUser, radicalePass, sEndpoint, sUser, sPass)
}

func runAttachments(t *testing.T, endpoint, user, pass, sEndpoint, sUser, sPass string) {
	sClient := liveClient(t, sEndpoint, sUser, sPass)
	files, err := file.New(sClient)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	suffix := fmt.Sprintf("carrel-s20-%d-%d", os.Getpid(), time.Now().Unix()%100000)

	client := liveClient(t, endpoint, user, pass)
	calHome, _ := liveHomes(ctx, t, endpoint, user, pass)
	calPath := calHome + suffix + "-cal/"
	calPath = makeCollection(ctx, t, client, endpoint, user, pass, "MKCALENDAR", calPath, `<?xml version="1.0" encoding="utf-8"?>
<C:mkcalendar xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set><D:prop>
    <D:displayname>Carrel S-20</D:displayname>
    <C:supported-calendar-component-set><C:comp name="VEVENT"/></C:supported-calendar-component-set>
  </D:prop></D:set>
</C:mkcalendar>`)

	// An event another client wrote, with an inline attachment of its own:
	// whatever Carrel writes afterwards, this one stays as it is (S-20).
	foreign := strings.Repeat("Rk9SRUlHTg", 20000) // 200 000 base64 characters
	uid := "event-" + suffix
	event := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other client//EN\r\nBEGIN:VEVENT\r\nUID:" + uid +
		"\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20260810T100000Z\r\nDTEND:20260810T110000Z\r\n" +
		"SUMMARY:Синк-ап " + suffix + "\r\nX-CUSTOM:keep-me\r\n" +
		"ATTACH;ENCODING=BASE64;VALUE=BINARY;FMTTYPE=image/png;FILENAME=foreign.png:" + foreign + "\r\n" +
		"END:VEVENT\r\nEND:VCALENDAR\r\n"
	seedRecord(ctx, t, client, calPath+uid+".ics", event)

	app := newLiveApp(t)
	calID := connectLive(t, app, endpoint, user, pass)
	sID := connectLive(t, app, sEndpoint, sUser, sPass)
	blob := liveCatalog(t, app)
	cal := catalogEntry(t, blob, calPath)
	record := "/app/calendar/" + calID + "/" + url.PathEscape(cal.Path) + "/" + uid

	// The folder is chosen the way a person chooses it: a collection of the
	// list on the settings page, and a folder in it made on first use.
	sub := suffix + "-attach"
	settings := liveDecode[view.AttachmentsPage](t, "attachment settings", app.get("/app/settings/attachments"))
	to := liveAttachCollection(t, settings, sID)
	// Named by the record's title (attachments-3): the title of this run
	// makes the name, so a folder of another run is never in the way.
	redirected(t, "choose the folder", app.post("/app/settings/attachments", url.Values{
		"collection": {to}, "folder": {sub}, "name": {"titled"},
	}))
	// What the folder is, is what the catalog now holds: the storage chosen,
	// and the folder under its root — which is not always the root of the
	// server.
	af := liveCatalog(t, app).Attachments
	if af == nil || af.AccountID != sID || strings.Join(af.Folder, "/") != sub {
		t.Fatalf("the folder was not stored: %+v", af)
	}
	folder := strings.TrimSuffix(af.Path, "/") + "/" + sub + "/"
	t.Logf("storage %q, attachment folder %q", af.Path, folder)
	t.Cleanup(func() { _ = sClient.Delete(context.Background(), folder, "") })

	// A small file goes into the record itself.
	small := bytes.Repeat([]byte{0x00, 0x01, 0x02, 0xfe}, 800) // 3200 bytes, not text
	page := liveDecode[view.EventPage](t, "event", app.get(record))
	redirected(t, "attach a small file", app.attach(liveAction(t, page.Actions, "attach.add"), "notes.bin", small))

	raw, _ := rawBody(ctx, t, client, calPath+uid+".ics")
	mustHave(t, "the record after a small attachment", raw, "FILENAME=notes.bin", "ENCODING=BASE64", liveBase64(small))
	if !strings.Contains(raw, foreign) {
		t.Errorf("the attachment of the other client changed:\n%s", head(raw))
	}

	page = liveDecode[view.EventPage](t, "event with two attachments", app.get(record))
	if len(page.Attach) != 2 {
		t.Fatalf("attachments = %+v", page.Attach)
	}
	mine := liveAttachment(t, page.Attach, "notes.bin")
	if !mine.Inline {
		t.Errorf("a small file should travel in the record: %+v", mine)
	}
	if got := app.get(mine.Href); got.Code != http.StatusOK || !bytes.Equal(got.Body.Bytes(), small) {
		t.Errorf("open the inline attachment: %d, %d bytes", got.Code, got.Body.Len())
	}

	// A file past the boundary is streamed to the folder, and the record keeps
	// a link to it.
	big := bytes.Repeat([]byte("carrel-attachment-"), 150000) // 2 700 000 bytes
	redirected(t, "attach a large file", app.attach(liveAction(t, page.Actions, "attach.add"), "report.pdf", big))

	want := "Синк-ап " + suffix + " - report.pdf"
	if e, err := files.Stat(ctx, folder+want); err != nil || e.Size != int64(len(big)) {
		t.Fatalf("the folder holds %v (%v)", liveNames(ctx, t, files, folder), err)
	}
	raw, _ = rawBody(ctx, t, client, calPath+uid+".ics")
	mustHave(t, "the record after a large attachment", raw, "ATTACH;", "SIZE="+fmt.Sprint(len(big)), url.PathEscape(want))
	if strings.Contains(raw, liveBase64(big[:64])) {
		t.Errorf("the large file went into the record as well")
	}

	page = liveDecode[view.EventPage](t, "event with three attachments", app.get(record))
	link := liveAttachment(t, page.Attach, want)
	if link.Inline || !strings.HasPrefix(link.Href, "/app/files/") {
		t.Fatalf("the large attachment = %+v", link)
	}
	// A link into a storage opens in Files: its folder, the file chosen,
	// Download first (the owner's decision of 02.10.2026, v0.2.146).
	inFiles := liveDecode[view.FilesPage](t, "open the link", app.get(link.Href))
	if inFiles.Details == nil || string(inFiles.Details.Title) != want {
		t.Fatalf("the link opens the panel %+v, want %q chosen", inFiles.Details, want)
	}
	if got := app.get(liveAction(t, inFiles.Details.Actions, "file.download")); got.Code != http.StatusOK || got.Body.Len() != len(big) {
		t.Errorf("download the linked file: %d, %d bytes of %d", got.Code, got.Body.Len(), len(big))
	}

	// The same name again is not overwritten: the next one gets a number. It
	// has to be another large file — a small one would go into the record and
	// never reach the folder.
	redirected(t, "attach it again", app.attach(liveAction(t, page.Actions, "attach.add"), "report.pdf", big))
	second := "Синк-ап " + suffix + " - report-2.pdf"
	if _, err := files.Stat(ctx, folder+second); err != nil {
		t.Errorf("no second file beside the first: %v (%v)", liveNames(ctx, t, files, folder), err)
	}
	if e, err := files.Stat(ctx, folder+want); err != nil || e.Size != int64(len(big)) {
		t.Errorf("the first file was overwritten: %v", err)
	}

	// Removing a link removes the link; the file stays where it is. Remove
	// stands on the edit only: on the card the attachment is a link and
	// nothing else (s-agenda-2, -3; v0.2.146).
	page = liveDecode[view.EventPage](t, "event before removing", app.get(record))
	if card := liveAttachment(t, page.Attach, want); len(card.Actions) != 0 {
		t.Errorf("the card's attachment carries %+v, want no action", card.Actions)
	}
	edit := liveDecode[view.EventEditPage](t, "the edit before removing", app.get(record+"/edit"))
	redirected(t, "remove the link", app.post(liveAction(t, liveAttachment(t, edit.Attach, want).Actions, "attach.remove"), url.Values{}))
	raw, _ = rawBody(ctx, t, client, calPath+uid+".ics")
	if strings.Contains(raw, url.PathEscape(want)) {
		t.Errorf("the link stayed in the record")
	}
	if _, err := files.Stat(ctx, folder+want); err != nil {
		t.Errorf("the file went with its link: %v", err)
	}
	if !strings.Contains(raw, foreign) || !strings.Contains(raw, "X-CUSTOM:keep-me") {
		t.Errorf("what the other client wrote did not survive the writes:\n%s", head(raw))
	}
	if conf.AttachInlineMax != 2<<20 {
		t.Errorf("the boundary is not 2 MiB: %d", conf.AttachInlineMax)
	}
}

// attach sends a file the way a page without scripts does: _csrf in front of
// the file part.
func (a *liveApp) attach(path, name string, data []byte) *httptest.ResponseRecorder {
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

// seedRecord writes a record the test needs before it starts. A write is never
// sent twice by Carrel — the server may have taken the first one (SV-14) — but
// a seed that lost the race on a connection the pool kept is not a finding,
// and here nothing is at stake: the name is fresh, so the second try either
// creates it or says it is taken.
func seedRecord(ctx context.Context, t *testing.T, client *dav.Client, path, body string) {
	t.Helper()
	typed := func() io.Reader { return dav.Typed(strings.NewReader(body), "text/calendar; charset=utf-8") }
	_, err := client.PutCreate(ctx, path, typed())
	if err != nil && strings.Contains(err.Error(), "EOF") {
		t.Logf("seed %s: %v; sending it again (SV-9, SV-14)", path, err)
		_, err = client.PutCreate(ctx, path, typed())
	}
	if err != nil {
		t.Fatalf("seed %s: %v", path, err)
	}
}

func liveAttachment(t *testing.T, rows []view.AttachRow, name string) view.AttachRow {
	t.Helper()
	for _, row := range rows {
		if string(row.Name) == name {
			return row
		}
	}
	t.Fatalf("no attachment %q in %+v", name, rows)
	return view.AttachRow{}
}

func liveBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func liveNames(ctx context.Context, t *testing.T, p *file.Provider, folder string) []string {
	t.Helper()
	entries, err := p.List(ctx, folder)
	if err != nil {
		return []string{"(" + err.Error() + ")"}
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name)
	}
	return out
}

// head is the start of a record, for a message that must not print megabytes.
func head(s string) string {
	if len(s) > 400 {
		return s[:400] + "…"
	}
	return s
}

// liveAttachCollection is the value of a file collection of the connection
// that can be written to, as the list «Collection» of the attachment
// settings offers it (attachments-2).
func liveAttachCollection(t *testing.T, page view.AttachmentsPage, account string) string {
	t.Helper()
	var offered []view.Option
	for _, set := range page.Form.Sets {
		for _, row := range set.Rows {
			if row.Name != "collection" {
				continue
			}
			offered = row.Options
			for _, o := range row.Options {
				if strings.HasPrefix(o.Value, account+"|") && !o.Disabled {
					return o.Value
				}
			}
		}
	}
	t.Fatalf("no storage of the file server in Collection: %+v", offered)
	return ""
}
