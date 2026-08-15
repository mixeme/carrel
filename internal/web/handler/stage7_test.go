// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/store"
)

// One server carries both a calendar and a plain collection, because that is the
// arrangement §23.10 needs: the calendar holds the entry and the file collection
// holds what is attached to it.
const (
	filesAccount   = "files-acc"
	filesRoot      = "/dav/files/"
	filesCalendar  = "/dav/calendars/user/journal/"
	filesOtherRoot = "/dav/other/"
)

// davHost is a WebDAV server that answers for a file collection and a calendar.
type davHost struct {
	*httptest.Server

	mu sync.Mutex
	// files maps a path to its body; a path ending in a slash is a folder.
	files map[string][]byte
	// objects are the calendar resources.
	objects map[string]string
	etags   map[string]string
	puts    []string
	deletes []string
	ctag    int
	// downFiles makes every file request fail, which is the degradation of §17.
	downFiles bool
}

func startDAVHost(t *testing.T) *davHost {
	t.Helper()
	h := &davHost{
		files:   map[string][]byte{filesRoot: nil, filesOtherRoot: nil},
		objects: map[string]string{},
		etags:   map[string]string{},
		ctag:    1,
	}
	h.Server = httptest.NewServer(http.HandlerFunc(h.serve))
	t.Cleanup(h.Close)
	return h
}

func (h *davHost) putFile(path string, body []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.files[path] = body
}

func (h *davHost) putObject(path, body string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.objects[path] = body
	h.etags[path] = `"o1"`
}

func (h *davHost) object(t *testing.T, path string) string {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	body, ok := h.objects[path]
	if !ok {
		t.Fatalf("no object at %q", path)
	}
	return body
}

func (h *davHost) fileNames(prefix string) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []string
	for path := range h.files {
		if strings.HasPrefix(path, prefix) && path != prefix {
			out = append(out, strings.TrimPrefix(path, prefix))
		}
	}
	return out
}

func (h *davHost) hasFile(path string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.files[path]
	return ok
}

func (h *davHost) setFilesDown(down bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.downFiles = down
}

func (h *davHost) isFilePath(path string) bool {
	return strings.HasPrefix(path, filesRoot) || strings.HasPrefix(path, filesOtherRoot)
}

func (h *davHost) serve(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "" {
		path = "/"
	}
	h.mu.Lock()
	down := h.downFiles
	h.mu.Unlock()
	if down && h.isFilePath(path) {
		http.Error(w, "storage unavailable", http.StatusServiceUnavailable)
		return
	}
	switch {
	case h.isFilePath(path):
		h.serveFiles(w, r, path)
	default:
		h.serveCalendar(w, r, path)
	}
}

func (h *davHost) serveFiles(w http.ResponseWriter, r *http.Request, path string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch r.Method {
	case "PROPFIND":
		if _, ok := h.files[path]; !ok {
			http.NotFound(w, r)
			return
		}
		var b strings.Builder
		b.WriteString(`<?xml version="1.0"?><d:multistatus xmlns:d="DAV:">`)
		h.writeFileResponse(&b, path)
		if r.Header.Get("Depth") == "1" && strings.HasSuffix(path, "/") {
			for member := range h.files {
				rest := strings.TrimSuffix(strings.TrimPrefix(member, path), "/")
				if member == path || !strings.HasPrefix(member, path) || rest == "" || strings.Contains(rest, "/") {
					continue
				}
				h.writeFileResponse(&b, member)
			}
		}
		b.WriteString(`</d:multistatus>`)
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = io.WriteString(w, b.String())
	case http.MethodGet:
		body, ok := h.files[path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(body)
	case http.MethodPut:
		if r.Header.Get("If-None-Match") == "*" {
			if _, taken := h.files[path]; taken {
				w.WriteHeader(http.StatusPreconditionFailed)
				return
			}
		}
		raw, _ := io.ReadAll(r.Body)
		h.files[path] = raw
		w.Header().Set("ETag", `"f1"`)
		w.WriteHeader(http.StatusCreated)
	case "MKCOL":
		dir := path
		if !strings.HasSuffix(dir, "/") {
			dir += "/"
		}
		if _, ok := h.files[dir]; ok {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		h.files[dir] = nil
		w.WriteHeader(http.StatusCreated)
	case http.MethodDelete:
		if _, ok := h.files[path]; !ok {
			http.NotFound(w, r)
			return
		}
		delete(h.files, path)
		h.deletes = append(h.deletes, path)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "no", http.StatusMethodNotAllowed)
	}
}

func (h *davHost) writeFileResponse(b *strings.Builder, path string) {
	if strings.HasSuffix(path, "/") {
		fmt.Fprintf(b, `<d:response><d:href>%s</d:href><d:propstat><d:prop>`+
			`<d:resourcetype><d:collection/></d:resourcetype></d:prop>`+
			`<d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`, path)
		return
	}
	fmt.Fprintf(b, `<d:response><d:href>%s</d:href><d:propstat><d:prop>`+
		`<d:resourcetype/><d:getcontentlength>%d</d:getcontentlength>`+
		`<d:getcontenttype>image/png</d:getcontenttype><d:getetag>"f1"</d:getetag>`+
		`</d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`,
		path, len(h.files[path]))
}

func (h *davHost) serveCalendar(w http.ResponseWriter, r *http.Request, path string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch r.Method {
	case "PROPFIND", "REPORT":
		collection := path
		if !strings.HasSuffix(collection, "/") {
			collection += "/"
		}
		var b strings.Builder
		fmt.Fprintf(&b, `<d:response><d:href>%s</d:href><d:propstat><d:prop>`+
			`<cs:getctag>ctag-%d</cs:getctag></d:prop><d:status>HTTP/1.1 200 OK</d:status>`+
			`</d:propstat></d:response>`, collection, h.ctag)
		for objPath, body := range h.objects {
			if !strings.HasPrefix(objPath, collection) {
				continue
			}
			data := ""
			if r.Method == "REPORT" {
				data = `<cal:calendar-data>` + xmlEscapeText(body) + `</cal:calendar-data>`
			}
			fmt.Fprintf(&b, `<d:response><d:href>%s</d:href><d:propstat><d:prop>`+
				`<d:getetag>%s</d:getetag>%s</d:prop><d:status>HTTP/1.1 200 OK</d:status>`+
				`</d:propstat></d:response>`, objPath, h.etags[objPath], data)
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = io.WriteString(w, `<?xml version="1.0"?>`+
			`<d:multistatus xmlns:d="DAV:" xmlns:cs="http://calendarserver.org/ns/" `+
			`xmlns:cal="urn:ietf:params:xml:ns:caldav">`+b.String()+`</d:multistatus>`)
	case http.MethodGet:
		body, ok := h.objects[path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("ETag", h.etags[path])
		w.Header().Set("Content-Type", "text/calendar")
		_, _ = io.WriteString(w, body)
	case http.MethodPut:
		raw, _ := io.ReadAll(r.Body)
		h.puts = append(h.puts, path)
		if match := r.Header.Get("If-Match"); match != "" && h.etags[path] != match {
			w.WriteHeader(http.StatusPreconditionFailed)
			return
		}
		h.ctag++
		etag := fmt.Sprintf("%q", fmt.Sprintf("o%d", h.ctag))
		h.objects[path], h.etags[path] = string(raw), etag
		w.Header().Set("ETag", etag)
		w.WriteHeader(http.StatusCreated)
	case http.MethodDelete:
		delete(h.objects, path)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

// filesApp connects one account holding a calendar and two file collections, one
// of them read-only.
func filesApp(t *testing.T, h *davHost) *app {
	t.Helper()
	a := newApp(t, nil)
	a.Guard = dav.NewGuard(dav.GuardConfig{Allowlist: []string{"127.0.0.1"}})
	a.setupAdmin("root", "", testPassword)

	sess := a.session()
	acc := account.Account{
		ID: filesAccount, Label: "Work", BaseURL: h.URL + "/dav/",
		Username: "mix", Password: "secret", Enabled: true,
		Collections: []discovery.Collection{
			{Path: filesCalendar, DisplayName: "Journal", Kind: discovery.KindCalendar,
				SupportedComponents: []string{"VJOURNAL", "VEVENT"}},
			{Path: filesRoot, DisplayName: "Storage", Kind: discovery.KindFiles},
			{Path: filesOtherRoot, DisplayName: "Shared", Kind: discovery.KindFiles, ReadOnly: true},
		},
	}
	if err := a.Store.PutDAVAccount(store.Actor{ID: sess.UserID, Login: sess.Login}, sess.UserID, sess.DEK(), acc); err != nil {
		t.Fatal(err)
	}
	return a
}

func filesURL(rel string) string {
	base := "/app/files/" + filesAccount + "/" + EncodeCollectionPath(filesRoot)
	if rel == "" {
		return base
	}
	return base + "?" + url.Values{fieldPath: {rel}}.Encode()
}

func downloadURL(rel string) string {
	return "/d/" + filesAccount + "/" + EncodeCollectionPath(filesRoot) +
		"?" + url.Values{fieldPath: {rel}}.Encode()
}

func (a *app) postMultipart(path string, fields url.Values, fileField, filename string, body []byte) *httptest.ResponseRecorder {
	return a.postMultipartAs(path, fields, fileField, filename, body, true)
}

// postMultipartAs submits an upload the way a browser form does — the token in a
// field and no header — or the way htmx does, with the header. Both have to work:
// the file browser and the attach form are plain forms, so the field is the
// normal case, and the field is also the case where the CSRF check has to find
// the token without eating the upload.
func (a *app) postMultipartAs(path string, fields url.Values, fileField, filename string, body []byte, withHeader bool) *httptest.ResponseRecorder {
	a.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField(CSRFField, a.token())
	for name, values := range fields {
		for _, value := range values {
			_ = mw.WriteField(name, value)
		}
	}
	if fileField != "" {
		part, err := mw.CreateFormFile(fileField, filename)
		if err != nil {
			a.t.Fatal(err)
		}
		if _, err := part.Write(body); err != nil {
			a.t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		a.t.Fatal(err)
	}
	if withHeader {
		return a.postRaw(path, mw.FormDataContentType(), &buf)
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return a.do(req)
}

// A plain HTML form cannot set a header, so the token travels as a field — and a
// large upload must still get through. The body limit has to be decided before
// the CSRF check reads the form looking for the token, or the check truncates the
// upload it was meant to authorise and the answer is a bewildering 403.
func TestLargeUploadFromAPlainFormIsAccepted(t *testing.T) {
	h := startDAVHost(t)
	a := filesApp(t, h)
	payload := bytes.Repeat([]byte("Q"), 3<<20) // over the 1 MiB default of §24.4

	rec := a.postMultipartAs(filesURL(""), url.Values{fieldPath: {""}}, "file", "big.png", payload, false)
	if rec.Code == http.StatusForbidden {
		t.Fatalf("a form-field token on a 3 MiB upload was refused: %s", rec.Body.String())
	}
	if rec.Code >= 400 {
		t.Fatalf("upload = %d %s", rec.Code, rec.Body.String())
	}
	if got := len(h.files[filesRoot+"big.png"]); got != len(payload) {
		t.Fatalf("stored %d bytes, want %d", got, len(payload))
	}
}

// The ceiling is a ceiling: past it the upload is refused rather than accepted in
// part (§24.4).
func TestUploadOverTheCeilingIsRefused(t *testing.T) {
	h := startDAVHost(t)
	a := filesApp(t, h)
	a.Files.MaxUploadBytes = 1 << 10

	rec := a.postMultipartAs(filesURL(""), url.Values{fieldPath: {""}}, "file", "big.png",
		bytes.Repeat([]byte("Q"), 8<<10), false)
	if rec.Code < 400 {
		t.Fatalf("an over-sized upload = %d, want a refusal", rec.Code)
	}
	if h.hasFile(filesRoot + "big.png") {
		t.Fatal("part of an over-sized upload was stored")
	}
}

func TestFilesNavigationIsAlwaysVisible(t *testing.T) {
	h := startDAVHost(t)

	plain := newApp(t, nil)
	plain.setupAdmin("root", "", testPassword)
	if body := plain.get("/app/").Body.String(); !strings.Contains(body, `/app/files"`) {
		t.Fatalf("the Files entry is missing without any file collection:\n%s", body)
	}

	a := filesApp(t, h)
	if body := a.get("/app/").Body.String(); !strings.Contains(body, `/app/files"`) {
		t.Fatalf("the Files entry is missing with a file collection connected:\n%s", body)
	}
}

func TestFilesBrowseListsFoldersAndFiles(t *testing.T) {
	h := startDAVHost(t)
	h.putFile(filesRoot+"pictures/", nil)
	h.putFile(filesRoot+"report.png", bytes.Repeat([]byte("p"), 2048))
	h.putFile(filesRoot+"pictures/inner.png", []byte("i"))
	a := filesApp(t, h)

	page := a.get(filesURL(""))
	if page.Code != http.StatusOK {
		t.Fatalf("browse = %d %s", page.Code, page.Body.String())
	}
	body := page.Body.String()
	for _, want := range []string{"pictures/", "report.png", "2.0 kB", "Storage"} {
		if !strings.Contains(body, want) {
			t.Errorf("listing is missing %q:\n%s", want, body)
		}
	}
	// A member of the subfolder is not a member of this one.
	if strings.Contains(body, "inner.png") {
		t.Errorf("the listing reached into a subfolder:\n%s", body)
	}

	inner := a.get(filesURL("pictures"))
	if inner.Code != http.StatusOK {
		t.Fatalf("subfolder = %d %s", inner.Code, inner.Body.String())
	}
	if !strings.Contains(inner.Body.String(), "inner.png") {
		t.Errorf("the subfolder listing is empty:\n%s", inner.Body.String())
	}
	if !strings.Contains(inner.Body.String(), "up one folder") {
		t.Errorf("no way back up from a subfolder:\n%s", inner.Body.String())
	}
}

// §24.4: a user's own file is served with an explicit name and nosniff, and never
// as something the browser may decide to render for itself.
func TestFileDownloadHeadersAndBody(t *testing.T) {
	h := startDAVHost(t)
	payload := bytes.Repeat([]byte("Z"), 4096)
	h.putFile(filesRoot+"Отчёт.png", payload)
	a := filesApp(t, h)

	rec := a.get(downloadURL("Отчёт.png"))
	if rec.Code != http.StatusOK {
		t.Fatalf("download = %d %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", got)
	}
	cd := rec.Header().Get("Content-Disposition")
	// The picture is shown in place; the name travels in both the ASCII and the
	// UTF-8 form so a transliterated name is not the only one on offer.
	if !strings.HasPrefix(cd, "inline;") || !strings.Contains(cd, "filename*=UTF-8''") {
		t.Errorf("Content-Disposition = %q", cd)
	}
	if rec.Header().Get("Cache-Control") != "private, no-store" {
		t.Errorf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}
	if rec.Body.Len() != len(payload) {
		t.Errorf("body = %d bytes, want %d", rec.Body.Len(), len(payload))
	}
}

// §24.4: the path in a download URL cannot be used to leave the collection.
func TestFileDownloadRefusesTraversal(t *testing.T) {
	h := startDAVHost(t)
	h.putObject(filesCalendar+"secret.ics", journalBody("secret", "Private", ""))
	a := filesApp(t, h)

	for _, rel := range []string{"../calendars/user/journal/secret.ics", "..%2fcalendars", ""} {
		rec := a.get("/d/" + filesAccount + "/" + EncodeCollectionPath(filesRoot) +
			"?" + url.Values{fieldPath: {rel}}.Encode())
		if rec.Code != http.StatusNotFound {
			t.Errorf("download of %q = %d, want 404\n%s", rel, rec.Code, rec.Body.String())
		}
	}
}

func TestFileUploadRefusesToOverwrite(t *testing.T) {
	h := startDAVHost(t)
	h.putFile(filesRoot+"taken.png", []byte("original"))
	a := filesApp(t, h)

	rec := a.postMultipart(filesURL(""), url.Values{fieldPath: {""}}, "file", "fresh.png", []byte("new file"))
	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusOK {
		t.Fatalf("upload = %d %s", rec.Code, rec.Body.String())
	}
	if !h.hasFile(filesRoot + "fresh.png") {
		t.Fatalf("the upload did not arrive; files = %v", h.fileNames(filesRoot))
	}

	again := a.postMultipart(filesURL(""), url.Values{fieldPath: {""}}, "file", "taken.png", []byte("replacement"))
	if again.Code >= 500 {
		t.Fatalf("second upload = %d %s", again.Code, again.Body.String())
	}
	if got := string(h.files[filesRoot+"taken.png"]); got != "original" {
		t.Fatalf("the file already there became %q", got)
	}
}

func TestReadOnlyFileCollectionRefusesWrites(t *testing.T) {
	h := startDAVHost(t)
	a := filesApp(t, h)
	other := "/app/files/" + filesAccount + "/" + EncodeCollectionPath(filesOtherRoot)

	page := a.get(other)
	if page.Code != http.StatusOK {
		t.Fatalf("browse = %d", page.Code)
	}
	if !strings.Contains(page.Body.String(), "read-only") {
		t.Errorf("a read-only collection is not marked as one:\n%s", page.Body.String())
	}
	if strings.Contains(page.Body.String(), `name="action" value="mkdir"`) {
		t.Errorf("a read-only collection offers a create form:\n%s", page.Body.String())
	}
	rec := a.postMultipart(other, url.Values{fieldPath: {""}}, "file", "x.png", []byte("x"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("upload into a read-only collection = %d, want 403", rec.Code)
	}
}

func TestMkdirAndDelete(t *testing.T) {
	h := startDAVHost(t)
	h.putFile(filesRoot+"gone.png", []byte("bye"))
	a := filesApp(t, h)

	if rec := a.post(filesURL(""), url.Values{
		"action": {"mkdir"}, fieldPath: {""}, "name": {"invoices"},
	}); rec.Code >= 400 {
		t.Fatalf("mkdir = %d %s", rec.Code, rec.Body.String())
	}
	if !h.hasFile(filesRoot + "invoices/") {
		t.Fatalf("the folder was not created; files = %v", h.fileNames(filesRoot))
	}
	// A name that is a path is refused rather than creating a tree.
	if rec := a.post(filesURL(""), url.Values{
		"action": {"mkdir"}, fieldPath: {""}, "name": {"../escaped"},
	}); rec.Code >= 500 {
		t.Fatalf("mkdir with a traversal = %d", rec.Code)
	}
	if h.hasFile("/dav/escaped/") {
		t.Fatal("mkdir created a folder outside the collection")
	}

	if rec := a.post(filesURL(""), url.Values{
		"action": {"delete"}, fieldPath: {""}, "target": {"gone.png"}, "etag": {`"f1"`},
	}); rec.Code >= 400 {
		t.Fatalf("delete = %d %s", rec.Code, rec.Body.String())
	}
	if h.hasFile(filesRoot + "gone.png") {
		t.Fatal("the file was not deleted")
	}
}

func journalBody(uid, summary, extra string) string {
	lines := []string{
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Carrel//EN",
		"BEGIN:VJOURNAL", "UID:" + uid, "DTSTAMP:20260811T090000Z",
		"DTSTART;VALUE=DATE:20260811", "SUMMARY:" + summary,
	}
	if extra != "" {
		lines = append(lines, extra)
	}
	lines = append(lines, "END:VJOURNAL", "END:VCALENDAR", "")
	return strings.Join(lines, "\r\n")
}

// setAttachmentFolder is the once-only settings step of §23.10.
func (a *app) setAttachmentFolder(t *testing.T, folder string) {
	t.Helper()
	rec := a.post("/app/", url.Values{
		"action":            {"save_attachments"},
		"attach_collection": {filesAccount + "|" + filesRoot},
		"attach_folder":     {folder},
	})
	if rec.Code >= 400 {
		t.Fatalf("save attachment folder = %d %s", rec.Code, rec.Body.String())
	}
}

func noteURL(uid string) string {
	return "/app/notes/" + filesAccount + "/" + EncodeCollectionPath(filesCalendar) + "/" + uid
}

// The whole of §23.10 in one pass: the folder is named once, a picture pasted
// into a note lands on the WebDAV server under a readable name, and the note
// carries an `ATTACH` with a URI to it.
func TestAttachToNoteUploadsAndLinks(t *testing.T) {
	h := startDAVHost(t)
	h.putObject(filesCalendar+"note-1.ics", journalBody("note-1", "Kitchen rebuild", ""))
	a := filesApp(t, h)
	a.setAttachmentFolder(t, "shots")

	if !h.hasFile(filesRoot + "shots/") {
		t.Fatalf("the attachment folder was not created; files = %v", h.fileNames(filesRoot))
	}

	rec := a.postMultipart(noteURL("note-1")+"/attach", url.Values{"etag": {`"o1"`}},
		"file", "Screenshot 2026-08-11 at 09.14.png", []byte("PNGDATA"))
	if rec.Code >= 400 {
		t.Fatalf("attach = %d %s", rec.Code, rec.Body.String())
	}

	// §23.10: the name is built from the date and the entry's title so the folder
	// stays readable outside Carrel — not `image-1.png`.
	names := h.fileNames(filesRoot + "shots/")
	if len(names) != 1 || names[0] != "2026-08-11-kitchen-rebuild.png" {
		t.Fatalf("stored files = %v, want a date-and-title name", names)
	}
	if got := string(h.files[filesRoot+"shots/2026-08-11-kitchen-rebuild.png"]); got != "PNGDATA" {
		t.Fatalf("stored body = %q", got)
	}

	stored := h.object(t, filesCalendar+"note-1.ics")
	unfolded := strings.ReplaceAll(strings.ReplaceAll(stored, "\r\n ", ""), "\n ", "")
	if !strings.Contains(unfolded, "ATTACH") {
		t.Fatalf("the note carries no ATTACH:\n%s", stored)
	}
	// A URI, not base64 in the object.
	if !strings.Contains(unfolded, h.URL+filesRoot+"shots/2026-08-11-kitchen-rebuild.png") {
		t.Fatalf("ATTACH is not a link to the uploaded file:\n%s", unfolded)
	}
	if strings.Contains(unfolded, "ENCODING=BASE64") {
		t.Fatalf("the file was embedded rather than linked:\n%s", unfolded)
	}
	if !strings.Contains(unfolded, "FILENAME=2026-08-11-kitchen-rebuild.png") {
		t.Fatalf("ATTACH has no file name for other clients to show:\n%s", unfolded)
	}

	// The card shows it, with the proxy that opens it.
	card := a.get(noteURL("note-1"))
	if card.Code != http.StatusOK {
		t.Fatalf("note card = %d", card.Code)
	}
	if !strings.Contains(card.Body.String(), "2026-08-11-kitchen-rebuild.png") {
		t.Fatalf("the attachment is not on the card:\n%s", card.Body.String())
	}
	if !strings.Contains(card.Body.String(), "/a/notes/"+filesAccount) {
		t.Fatalf("the card offers no proxy link to open the attachment:\n%s", card.Body.String())
	}
}

// §23.10: opening an attachment goes through Carrel, and the bytes are the file's.
func TestAttachmentOpenProxiesTheFile(t *testing.T) {
	h := startDAVHost(t)
	uri := h.URL + filesRoot + "plan.png"
	h.putFile(filesRoot+"plan.png", []byte("PLANBYTES"))
	h.putObject(filesCalendar+"note-2.ics", journalBody("note-2", "Plans",
		"ATTACH;FMTTYPE=image/png;FILENAME=plan.png:"+uri))
	a := filesApp(t, h)

	rec := a.get("/a/notes/" + filesAccount + "/" + EncodeCollectionPath(filesCalendar) + "/note-2/0")
	if rec.Code != http.StatusOK {
		t.Fatalf("open attachment = %d %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "PLANBYTES" {
		t.Fatalf("proxied body = %q", rec.Body.String())
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", got)
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "plan.png") {
		t.Errorf("Content-Disposition = %q", rec.Header().Get("Content-Disposition"))
	}
}

// The proxy is not an open one: a link to somewhere this user has no collection
// is shown and not fetched (§24.2).
func TestAttachmentOnAForeignServerIsNotFetched(t *testing.T) {
	h := startDAVHost(t)
	h.putObject(filesCalendar+"note-3.ics", journalBody("note-3", "Foreign",
		"ATTACH;FMTTYPE=image/png:https://elsewhere.example/private/secret.png"))
	a := filesApp(t, h)

	card := a.get(noteURL("note-3"))
	if card.Code != http.StatusOK {
		t.Fatalf("note card = %d", card.Code)
	}
	body := card.Body.String()
	if !strings.Contains(body, "secret.png") {
		t.Fatalf("a foreign attachment should still be shown:\n%s", body)
	}
	if !strings.Contains(body, "cannot reach") {
		t.Fatalf("a foreign attachment is not marked as unreachable:\n%s", body)
	}
	rec := a.get("/a/notes/" + filesAccount + "/" + EncodeCollectionPath(filesCalendar) + "/note-3/0")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("opening a foreign attachment = %d, want 403", rec.Code)
	}
}

// §23.10: an inline attachment from another client is shown as it is and never
// rewritten into a link.
func TestInlineAttachmentIsReadOnly(t *testing.T) {
	h := startDAVHost(t)
	h.putObject(filesCalendar+"note-4.ics", journalBody("note-4", "Embedded",
		"ATTACH;ENCODING=BASE64;VALUE=BINARY;FILENAME=old.txt:aGk="))
	a := filesApp(t, h)

	body := a.get(noteURL("note-4")).Body.String()
	if !strings.Contains(body, "old.txt") || !strings.Contains(body, "stored inside the entry") {
		t.Fatalf("the inline attachment is not shown as such:\n%s", body)
	}
	rec := a.get("/a/notes/" + filesAccount + "/" + EncodeCollectionPath(filesCalendar) + "/note-4/0")
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("opening an inline attachment = %d, want 501", rec.Code)
	}
	// Nothing was written to the calendar just by looking at it.
	if len(h.puts) != 0 {
		t.Fatalf("viewing an inline attachment wrote %v", h.puts)
	}
}

// §23.10: removing a link is not deleting the file, and the interface says so.
func TestDetachLeavesTheFileOnTheServer(t *testing.T) {
	h := startDAVHost(t)
	uri := h.URL + filesRoot + "keep.png"
	h.putFile(filesRoot+"keep.png", []byte("KEEP"))
	h.putObject(filesCalendar+"note-5.ics", journalBody("note-5", "Attached",
		"ATTACH;FMTTYPE=image/png;FILENAME=keep.png:"+uri))
	a := filesApp(t, h)

	rec := a.post(noteURL("note-5")+"/attach", url.Values{
		"action": {"detach"}, "attachment": {"0"}, "etag": {`"o1"`},
	})
	if rec.Code >= 400 {
		t.Fatalf("detach = %d %s", rec.Code, rec.Body.String())
	}
	stored := h.object(t, filesCalendar+"note-5.ics")
	if strings.Contains(stored, "ATTACH") {
		t.Fatalf("the property is still on the note:\n%s", stored)
	}
	if !h.hasFile(filesRoot + "keep.png") {
		t.Fatal("detaching deleted the file from the WebDAV server")
	}
	// The interface says what just happened rather than leaving a person to
	// assume the file went with the link (§23.10).
	location := rec.Header().Get("Location")
	notice, err := url.Parse(location)
	if err != nil {
		t.Fatalf("Location = %q: %v", location, err)
	}
	if !strings.Contains(notice.Query().Get("notice"), "still on the server") {
		t.Errorf("the notice does not say the file is still there: %q", notice.Query().Get("notice"))
	}
	if !strings.Contains(a.get(location).Body.String(), "still on the server") {
		t.Error("following the redirect does not show the notice")
	}
}

// §17: a file server that is down marks the attachment, and does not break the
// entry it is on. A 500 on a failed source is a bug, so this asserts what the
// codes are.
func TestFileServerDownStillRendersTheNote(t *testing.T) {
	h := startDAVHost(t)
	uri := h.URL + filesRoot + "gone.png"
	h.putFile(filesRoot+"gone.png", []byte("X"))
	h.putObject(filesCalendar+"note-6.ics", journalBody("note-6", "Resilient",
		"ATTACH;FMTTYPE=image/png;FILENAME=gone.png:"+uri))
	a := filesApp(t, h)
	h.setFilesDown(true)

	card := a.get(noteURL("note-6"))
	if card.Code != http.StatusOK {
		t.Fatalf("note card with the file server down = %d %s", card.Code, card.Body.String())
	}
	if !strings.Contains(card.Body.String(), "gone.png") {
		t.Fatalf("the attachment vanished from the card:\n%s", card.Body.String())
	}
	// Opening it fails with a gateway error rather than a 500.
	rec := a.get("/a/notes/" + filesAccount + "/" + EncodeCollectionPath(filesCalendar) + "/note-6/0")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("opening an unreachable attachment = %d, want 502", rec.Code)
	}
	// The file browser says so too, and does not answer 500 either (§17).
	browse := a.get(filesURL(""))
	if browse.Code >= 500 {
		t.Fatalf("browsing an unreachable collection = %d, want a reported failure", browse.Code)
	}
}

// A card whose collection has no attachment folder yet says where to set one,
// rather than offering a form that cannot work.
func TestNoteWithoutAnAttachmentFolderSaysWhereToSetOne(t *testing.T) {
	h := startDAVHost(t)
	h.putObject(filesCalendar+"note-7.ics", journalBody("note-7", "Unset", ""))
	a := filesApp(t, h)

	body := a.get(noteURL("note-7")).Body.String()
	if !strings.Contains(body, "choose a folder for them") {
		t.Fatalf("the card does not point at the setting:\n%s", body)
	}
	if strings.Contains(body, `name="file"`) {
		t.Fatalf("an attach form is offered with no folder configured:\n%s", body)
	}
	rec := a.postMultipart(noteURL("note-7")+"/attach", nil, "file", "x.png", []byte("x"))
	if rec.Code >= 500 {
		t.Fatalf("attach with no folder = %d %s", rec.Code, rec.Body.String())
	}
}

// Block B8: a folder of Markdown already on the person's WebDAV imports without
// being downloaded and uploaded again.
func TestImportNotesFromWebDAVFolder(t *testing.T) {
	h := startDAVHost(t)
	h.putFile(filesRoot+"journal/", nil)
	h.putFile(filesRoot+"journal/2026-08-01-first.md", []byte("---\ntitle: First\ntags:\n  - work\n---\n\nBody one.\n"))
	h.putFile(filesRoot+"journal/second.md", []byte("# Second\n\nBody two.\n"))
	h.putFile(filesRoot+"journal/photo.png", []byte("not a note"))
	a := filesApp(t, h)

	colEnc := EncodeCollectionPath(filesCalendar)
	preview := a.post("/app/notes/"+filesAccount+"/"+colEnc+"/import", url.Values{
		"action":        {"webdav_import"},
		"webdav_source": {filesAccount + "|" + EncodeCollectionPath(filesRoot)},
		"webdav_folder": {"journal"},
	})
	if preview.Code != http.StatusOK {
		t.Fatalf("webdav preview = %d %s", preview.Code, preview.Body.String())
	}
	body := preview.Body.String()
	for _, want := range []string{"First", "Second", "work"} {
		if !strings.Contains(body, want) {
			t.Errorf("preview is missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "photo.png") {
		t.Errorf("a non-Markdown file was previewed:\n%s", body)
	}

	confirm := a.post("/app/notes/"+filesAccount+"/"+colEnc+"/import", url.Values{
		"action": {"confirm_import"},
	})
	if confirm.Code != http.StatusOK {
		t.Fatalf("confirm = %d %s", confirm.Code, confirm.Body.String())
	}
	if !strings.Contains(confirm.Body.String(), "Imported 2") {
		t.Fatalf("report = %s", confirm.Body.String())
	}
	// Nothing on the file server was touched by an import that only reads.
	if len(h.deletes) != 0 {
		t.Fatalf("the import deleted %v", h.deletes)
	}
	if !h.hasFile(filesRoot + "journal/2026-08-01-first.md") {
		t.Fatal("the import moved the source file")
	}
}
