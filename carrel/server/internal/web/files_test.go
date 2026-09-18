package web_test

import (
	"bytes"
	"encoding/json"
	"image"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

const filesDocs = davtest.FileRoot + "docs/"

// filesFixture is a person with one connection whose file collection Docs is
// in the catalog; ticked says whether the rail ticked it.
func filesFixture(t *testing.T, ticked bool) davFixture {
	t.Helper()
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	f := connectFixture(t, srv, filesDocs, nil)
	docs := sealed.Collection{AccountID: f.id, Path: filesDocs, Name: "Docs", Kinds: []view.Kind{view.KindFile}}
	if ticked {
		tickAll(t, f.h, f.sess, docs)
	} else {
		editBlob(t, f.h, f.sess, func(b *sealed.Blob) { b.Collections = append(b.Collections, docs) })
	}
	return f
}

func editBlob(t *testing.T, h *harness, sess *session.Session, edit func(*sealed.Blob)) {
	t.Helper()
	key, _ := sess.DataKey()
	blob, ok := h.users.Catalog(sess.AccountID(), key)
	if !ok {
		t.Fatal("no catalog")
	}
	edit(&blob)
	sealedBlob, err := sealed.Seal(key, blob, sess.Login())
	if err != nil {
		t.Fatal(err)
	}
	if err := h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		st.UserByID(sess.AccountID()).SealedBlob = sealedBlob
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
}

// at is an address of the Docs collection: marker, then segments, a trailing
// "/" when the last one is empty.
func (f davFixture) at(marker string, segs ...string) string {
	h := "/app/files/" + f.id + "/" + url.PathEscape(filesDocs) + "/"
	if marker != "" {
		h += marker + "/"
	}
	for i, s := range segs {
		if i > 0 {
			h += "/"
		}
		h += url.PathEscape(s)
	}
	return h
}

func fileRow(t *testing.T, page view.FilesPage, name string) view.FileRow {
	t.Helper()
	for _, r := range page.Rows {
		if string(r.Name) == name {
			return r
		}
	}
	t.Fatalf("no row %q in %+v", name, page.Rows)
	return view.FileRow{}
}

func rowMenu(t *testing.T, row view.FileRow, id string) string {
	t.Helper()
	for _, m := range row.Menu {
		if m.ID == id {
			return m.Href
		}
	}
	t.Fatalf("no menu item %q in %+v", id, row.Menu)
	return ""
}

// S-17.2: the section is always there and says why it is empty; the root is
// the ticked storages as folders, and the rail ticks them the way it ticks
// record collections.
func TestFilesRootFollowsTheTicks(t *testing.T) {
	bare := contactsFixture(t, false, adaCard())
	root := decode[view.FilesRootPage](t, bare.h.do(get("/app/files", bare.sess)))
	if root.Empty == nil || root.Empty.Title != "No file storage is connected" || len(root.Collections) != 0 {
		t.Fatalf("no storage: %+v", root.Empty)
	}

	f := filesFixture(t, false)
	root = decode[view.FilesRootPage](t, f.h.do(get("/app/files", f.sess)))
	if root.Empty == nil || root.Empty.Title != "No storage is ticked" {
		t.Fatalf("nothing ticked: %+v", root.Empty)
	}
	_, row := railRow(t, root.Rail, "Docs")
	if row.Tick != view.TickOff || row.Href != f.at("") || row.Count != nil || len(row.Menu) != 0 {
		t.Errorf("rail row = %+v", row)
	}
	if root.Rail.Sections[0].Rows[0].Label != "Selected collections" || root.Rail.Sections[0].Rows[0].Tick != view.TickOff {
		t.Errorf("root row = %+v", root.Rail.Sections[0])
	}

	rec := f.h.do(postForm("/app/sources/files", f.sess, url.Values{"tick": {row.Href}}))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/app/files" {
		t.Fatalf("tick: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	root = decode[view.FilesRootPage](t, f.h.do(get("/app/files", f.sess)))
	if root.Empty != nil || len(root.Collections) != 1 {
		t.Fatalf("ticked root: %+v", root)
	}
	docs := root.Collections[0]
	if docs.Href != f.at("") || docs.Label != "Docs" || !strings.HasSuffix(string(docs.Stamp), " · Docs") || docs.State != view.CollectionOK {
		t.Errorf("row = %+v", docs)
	}
	if len(root.Shortcuts) != 2 || root.Shortcuts[0].ID != "files.attachments" || root.Shortcuts[1].Href != "/app/files/published" {
		t.Errorf("shortcuts = %+v", root.Shortcuts)
	}
	var filesOn bool
	for _, n := range root.Rail.Nav {
		filesOn = filesOn || (n.ID == "nav.files" && n.On)
	}
	if !filesOn || root.Rail.Sections[0].Rows[0].Tick != view.TickOn {
		t.Errorf("rail = %+v", root.Rail)
	}
	if contacts := selected(t, f.h, f.sess, view.KindContact); len(contacts) != 0 {
		t.Errorf("ticking files ticked contacts: %v", contacts)
	}

	frag := decode[railFragment](t, f.h.do(fragment(postForm("/app/sources/files", f.sess, url.Values{}), "rail")))
	if frag.Target != "rail" || frag.Body.Sections[0].Rows[0].Tick != view.TickOff {
		t.Errorf("untick fragment = %+v", frag)
	}
}

// S-17: a folder is one PROPFIND: folders first, files open as downloads, a
// thumbnail only for a small raster image, crumbs to the root; a filter, an
// order, the tiles or a selection cut the same listing; a search result's
// address opens the folder with the file selected; nothing outside the
// collection is reached.
func TestFolderPage(t *testing.T) {
	f := filesFixture(t, true)
	var png bytes.Buffer
	png.Write(pngOf(t, image.NewRGBA(image.Rect(0, 0, 4, 4))))
	f.srv.SeedFile(filesDocs+"b report.pdf", "0123456789")
	f.srv.SeedFile(filesDocs+"a.png", png.String())
	f.srv.SeedFile(filesDocs+"big.png", strings.Repeat("p", 600<<10))
	f.srv.SeedFile(filesDocs+"sub/deep.md", "x")

	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	var names []string
	for _, r := range page.Rows {
		names = append(names, string(r.Name))
	}
	if strings.Join(names, ",") != "sub,a.png,b report.pdf,big.png" {
		t.Fatalf("rows = %v", names)
	}
	sub, a, pdf, big := page.Rows[0], page.Rows[1], page.Rows[2], page.Rows[3]
	if !sub.IsDir || sub.Href != f.at("", "sub", "") || sub.Type != "folder" || sub.Size != "" {
		t.Errorf("folder = %+v", sub)
	}
	if a.Href != f.at("download", "a.png") || a.Thumb != a.Href+"?inline=1" || a.Type != "image/png" {
		t.Errorf("image = %+v", a)
	}
	if pdf.Thumb != "" || pdf.Type != "application/pdf" || pdf.Size != "10 B" || pdf.Modified == "" || big.Thumb != "" {
		t.Errorf("pdf %+v, big %+v", pdf, big)
	}
	if page.Title != "Docs" || len(page.Crumbs) != 1 || page.Crumbs[0].Href != "" || page.View != view.FileList || page.Queue != nil {
		t.Errorf("page = %q %+v %q", page.Title, page.Crumbs, page.View)
	}
	if page.Bar.Total == nil || *page.Bar.Total != 4 || actionHref(t, page.Bar.Actions, "file.upload") != f.at("upload") ||
		actionHref(t, page.Bar.Actions, "folder.new") != f.at("mkcol") {
		t.Errorf("bar = %+v", page.Bar)
	}
	if !strings.Contains(rowMenu(t, pdf, "file.delete"), "op=delete") {
		t.Errorf("menu = %+v", pdf.Menu)
	}

	listed := f.srv.CountRequests("PROPFIND", filesDocs)
	cut := decode[view.FilesPage](t, f.h.do(get(f.at("")+"?filter=PNG&sort=size&view=tiles&select=a.png", f.sess)))
	if len(cut.Rows) != 2 || cut.Rows[0].Name != "big.png" || cut.View != view.FileTiles || !cut.Rows[1].Selected || cut.Bar.Selected != 1 {
		t.Fatalf("cut = %+v", cut.Rows)
	}
	if actionHref(t, cut.Bar.Actions, "file.download") != f.at("download", "a.png") {
		t.Errorf("selection actions = %+v", cut.Bar.Actions)
	}
	if n := f.srv.CountRequests("PROPFIND", filesDocs); n != listed {
		t.Errorf("a filter asked the server again: %d → %d", listed, n)
	}

	deep := decode[view.FilesPage](t, f.h.do(get(f.at("", "sub", "deep.md"), f.sess)))
	if deep.Title != "sub" || len(deep.Rows) != 1 || !deep.Rows[0].Selected || len(deep.Crumbs) != 2 || deep.Crumbs[0].Href != f.at("") {
		t.Errorf("found file = %+v %+v", deep.Rows, deep.Crumbs)
	}
	if rec := f.h.do(get(f.at("", "sub"), f.sess)); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != f.at("", "sub", "") {
		t.Errorf("folder without its slash: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if n := f.srv.CountRequests("GET", filesDocs); n != 0 {
		t.Errorf("the listing read %d file bodies", n)
	}
	for _, path := range []string{
		f.at("", "none.txt"),
		f.at("", "nowhere", ""),
		f.at("") + "%2E%2E/",
		f.at("") + "sub%2F..%2F..%2F",
		"/app/files/a-nowhere000/" + url.PathEscape(filesDocs) + "/",
	} {
		if rec := f.h.do(get(path, f.sess)); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d", path, rec.Code)
		}
	}
}

// S-17.1, C-12: a download streams past CARREL_DAV_MAX_RESPONSE_BYTES, passes a
// range, and is an attachment in a sandbox; only a small raster image is shown
// in place.
func TestDownload(t *testing.T) {
	f := filesFixture(t, true)
	big := strings.Repeat("0123456789", (11<<20)/10)
	f.srv.SeedFile(filesDocs+"отчёт.bin", big)
	f.srv.SeedFile(filesDocs+"page.html", "<script>alert(1)</script>")
	f.srv.SeedFile(filesDocs+"a.png", string(pngOf(t, image.NewRGBA(image.Rect(0, 0, 2, 2)))))
	f.srv.SeedFile(filesDocs+"download/inside.txt", "x")

	rec := f.h.do(get(f.at("download", "отчёт.bin"), f.sess))
	h := rec.Header()
	if rec.Code != http.StatusOK || rec.Body.Len() != len(big) || h.Get("Content-Length") != "11534330" {
		t.Fatalf("download: %d, %d bytes, length %q", rec.Code, rec.Body.Len(), h.Get("Content-Length"))
	}
	if !strings.HasPrefix(h.Get("Content-Disposition"), "attachment") || !strings.Contains(h.Get("Content-Disposition"), "filename*=utf-8''%D0%BE") ||
		h.Get("Content-Security-Policy") != "sandbox; default-src 'none'" || h.Get("X-Content-Type-Options") != "nosniff" || h.Get("ETag") == "" {
		t.Errorf("headers = %v", h)
	}

	req := get(f.at("download", "отчёт.bin"), f.sess)
	req.Header.Set("Range", "bytes=10-19")
	part := f.h.do(req)
	if part.Code != http.StatusPartialContent || part.Body.String() != "0123456789" || !strings.HasPrefix(part.Header().Get("Content-Range"), "bytes 10-19/") {
		t.Errorf("range: %d %q %q", part.Code, part.Body.String(), part.Header().Get("Content-Range"))
	}

	html := f.h.do(get(f.at("download", "page.html")+"?inline=1", f.sess))
	if html.Code != http.StatusOK || !strings.HasPrefix(html.Header().Get("Content-Disposition"), "attachment") {
		t.Errorf("html inline: %d %q", html.Code, html.Header().Get("Content-Disposition"))
	}
	img := f.h.do(get(f.at("download", "a.png")+"?inline=1", f.sess))
	if img.Code != http.StatusOK || !strings.HasPrefix(img.Header().Get("Content-Disposition"), "inline") || img.Header().Get("Content-Type") != "image/png" {
		t.Errorf("png inline: %d %v", img.Code, img.Header())
	}
	if rec := f.h.do(get(f.at("download", "none.bin"), f.sess)); rec.Code != http.StatusNotFound {
		t.Errorf("missing file: %d", rec.Code)
	}
	// C-5.5 gives the segment download/ to this route; a folder address after
	// it is the folder of that name.
	folder := decode[view.FilesPage](t, f.h.do(get(f.at("download", ""), f.sess)))
	if folder.Title != "download" || len(folder.Rows) != 1 {
		t.Errorf("folder named download = %q %+v", folder.Title, folder.Rows)
	}
}

type upload struct{ name, body string }

// uploadForm sends files the way a page does: _csrf in the body in front of
// the files, or in X-CSRF-Token.
func uploadForm(path string, sess *session.Session, header bool, files ...upload) *http.Request {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if !header {
		_ = w.WriteField("_csrf", sess.CSRFToken())
	}
	for _, f := range files {
		part, _ := w.CreateFormFile("file", f.name)
		_, _ = part.Write([]byte(f.body))
	}
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if header {
		req.Header.Set("X-CSRF-Token", sess.CSRFToken())
	}
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: sess.ID()})
	return req
}

// S-17.1 and docs/servers.md, SV-15: a storage that writes only what it was
// told the length of. The row says why, and names what to do about it.
func TestUploadNeedsASizeOnSuchAServer(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.RefuseUnsizedPUT = true
	// The folder itself has to be there: the only write this test makes is the
	// one the server refuses, and a folder page of nothing is a 404.
	f.srv.SeedFolder(filesDocs)

	if rec := f.h.do(uploadForm(f.at("upload"), f.sess, false, upload{"plain.txt", "body"})); rec.Code != http.StatusSeeOther {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	queue := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess))).Queue
	if row := queueRow(t, queue, "plain.txt"); row.State != view.OpFailed {
		t.Fatalf("row = %+v", row)
	}
	if !strings.Contains(string(queue.Note), "size") {
		t.Errorf("the queue does not say why: %q", queue.Note)
	}
	if _, ok := f.srv.File(filesDocs + "plain.txt"); ok {
		t.Errorf("the file was written after all")
	}

	// With the size in front of it the same upload goes through.
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("_csrf", f.sess.CSRFToken())
	_ = w.WriteField("size.sized.txt", "4")
	part, _ := w.CreateFormFile("file", "sized.txt")
	_, _ = part.Write([]byte("body"))
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, f.at("upload"), &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: f.sess.ID()})
	if rec := f.h.do(req); rec.Code != http.StatusSeeOther {
		t.Fatalf("sized upload: %d %s", rec.Code, rec.Body.String())
	}
	if got, ok := f.srv.File(filesDocs + "sized.txt"); !ok || got != "body" {
		t.Errorf("sized upload wrote %q, %v", got, ok)
	}
}

func heldUploads(t *testing.T) int {
	t.Helper()
	m, _ := filepath.Glob(filepath.Join(os.TempDir(), "carrel-upload-*"))
	return len(m)
}

func queueRow(t *testing.T, q *view.TransferQueue, name string) view.TransferRow {
	t.Helper()
	if q == nil {
		t.Fatalf("no queue, want a row %q", name)
	}
	for _, r := range q.Rows {
		if string(r.Name) == name {
			return r
		}
	}
	t.Fatalf("no queue row %q in %+v", name, q.Rows)
	return view.TransferRow{}
}

func option(t *testing.T, row view.TransferRow, id string) string {
	t.Helper()
	if row.Question == nil {
		t.Fatalf("row %q asks nothing", row.Name)
	}
	return actionHref(t, row.Question.Options, id)
}

// S-17.3: an upload never overwrites. A free name goes straight to the server,
// past a form's 1 MiB; a taken one is a question that holds the file — also on
// a server that ignores If-None-Match — until Skip, Keep both or Overwrite
// this one, each answered once and only by this session; the end of the
// session lets the file go.
func TestUploadAsksAboutTakenNames(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.FilesIgnoreIfNoneMatch = true
	f.srv.SeedFile(filesDocs+"taken.txt", "original")
	held := heldUploads(t)
	large := strings.Repeat("L", 2<<20)

	rec := f.h.do(uploadForm(f.at("upload"), f.sess, false, upload{"new.txt", large}, upload{`C:\fakepath\taken.txt`, "mine"}))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != f.at("") {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	if got, _ := f.srv.File(filesDocs + "new.txt"); got != large {
		t.Errorf("new.txt holds %d bytes", len(got))
	}
	if got, _ := f.srv.File(filesDocs + "taken.txt"); got != "original" {
		t.Fatalf("taken.txt was overwritten: %q", got)
	}
	if heldUploads(t) != held+1 {
		t.Errorf("held uploads %d → %d", held, heldUploads(t))
	}

	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	done, ask := queueRow(t, page.Queue, "new.txt"), queueRow(t, page.Queue, "taken.txt")
	if done.State != view.OpDone || done.Percent != 100 || done.Size != "2.0 MB" || ask.State != view.OpIdle || len(ask.Question.Options) != 3 {
		t.Fatalf("queue = %+v", page.Queue)
	}
	again := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	if again.Queue == nil || len(again.Queue.Rows) != 1 || !strings.Contains(string(again.Queue.Note), "waiting for an answer") {
		t.Fatalf("the finished row stayed or the question went: %+v", again.Queue)
	}

	other := f.h.login("owner", "password123")
	if rec := f.h.do(postForm(option(t, ask, "transfer.both"), other, url.Values{})); rec.Code != http.StatusNotFound {
		t.Errorf("another session answered: %d", rec.Code)
	}
	if rec := f.h.do(postForm(option(t, ask, "transfer.both"), f.sess, url.Values{})); rec.Code != http.StatusSeeOther {
		t.Fatalf("keep both: %d %s", rec.Code, rec.Body.String())
	}
	if rec := f.h.do(postForm(option(t, ask, "transfer.both"), f.sess, url.Values{})); rec.Code != http.StatusNotFound {
		t.Errorf("one question answered twice: %d", rec.Code)
	}
	if got, _ := f.srv.File(filesDocs + "taken-2.txt"); got != "mine" {
		t.Errorf("taken-2.txt = %q", got)
	}
	if got, _ := f.srv.File(filesDocs + "taken.txt"); got != "original" || heldUploads(t) != held {
		t.Errorf("after keep both: taken.txt %q, held %d", got, heldUploads(t))
	}

	// With scripts: the token in the header, the queue as the answer.
	var q struct {
		Target string             `json:"target"`
		Body   view.TransferQueue `json:"body"`
	}
	rec = f.h.do(fragment(uploadForm(f.at("upload"), f.sess, true, upload{"taken.txt", "second"}), "queue"))
	if err := json.Unmarshal(rec.Body.Bytes(), &q); err != nil || q.Target != "queue" {
		t.Fatalf("fragment: %d %s", rec.Code, rec.Body.String())
	}
	ask = queueRow(t, &q.Body, "taken.txt")
	if rec := f.h.do(fragment(postForm(option(t, ask, "transfer.overwrite"), f.sess, url.Values{}), "queue")); rec.Code != http.StatusOK {
		t.Fatalf("overwrite: %d", rec.Code)
	}
	if got, _ := f.srv.File(filesDocs + "taken.txt"); got != "second" {
		t.Errorf("after overwrite: %q", got)
	}

	f.h.do(uploadForm(f.at("upload"), f.sess, false, upload{"taken.txt", "third"}))
	ask = queueRow(t, decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess))).Queue, "taken.txt")
	f.h.do(postForm(option(t, ask, "transfer.skip"), f.sess, url.Values{}))
	if got, _ := f.srv.File(filesDocs + "taken.txt"); got != "second" || heldUploads(t) != held {
		t.Errorf("after skip: %q, held %d", got, heldUploads(t))
	}

	f.h.do(uploadForm(f.at("upload"), f.sess, false, upload{"taken.txt", "fourth"}))
	if heldUploads(t) != held+1 {
		t.Fatal("no upload held for the question")
	}
	f.h.mgr.End(f.sess.ID())
	if heldUploads(t) != held {
		t.Error("the end of the session left the held upload")
	}
}

// C-11 with a stream: _csrf in front of the files, or in the header. A file
// before the token, or no token, is refused before anything reaches the
// server; a read-only storage takes nothing.
func TestUploadRefusals(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFolder(filesDocs)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("file", "early.txt")
	_, _ = part.Write([]byte("x"))
	_ = w.WriteField("_csrf", f.sess.CSRFToken())
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, f.at("upload"), &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: f.sess.ID()})
	if rec := f.h.do(req); rec.Code != http.StatusForbidden {
		t.Errorf("token after the file: %d", rec.Code)
	}
	if _, ok := f.srv.File(filesDocs + "early.txt"); ok {
		t.Error("a file without a checked token was written")
	}

	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		for i := range b.Collections {
			if b.Collections[i].Path == filesDocs {
				b.Collections[i].ReadOnly = true
			}
		}
	})
	page := decode[view.FilesPage](t, f.h.do(uploadForm(f.at("upload"), f.sess, false, upload{"ro.txt", "x"})))
	if !hasNotice(page.Notices, "This storage is read-only") {
		t.Errorf("read-only: %+v", page.Notices)
	}
	if _, ok := f.srv.File(filesDocs + "ro.txt"); ok || f.srv.CountRequests("PUT", filesDocs) != 0 {
		t.Error("a read-only storage was written")
	}
	for _, a := range page.Bar.Actions {
		if (a.ID == "file.upload" || a.ID == "folder.new") && (a.Enabled || a.Reason == "") {
			t.Errorf("%s on a read-only storage = %+v", a.ID, a)
		}
	}
}

func TestNewFolder(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"old/x.txt", "x")
	if rec := f.h.do(postForm(f.at("mkcol"), f.sess, url.Values{"name": {"Новая папка"}})); rec.Code != http.StatusSeeOther {
		t.Fatalf("mkcol: %d %s", rec.Code, rec.Body.String())
	}
	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	if row := fileRow(t, page, "Новая папка"); !row.IsDir {
		t.Errorf("row = %+v", row)
	}
	taken := decode[view.FilesPage](t, f.h.do(postForm(f.at("mkcol"), f.sess, url.Values{"name": {"old"}})))
	bad := decode[view.FilesPage](t, f.h.do(postForm(f.at("mkcol"), f.sess, url.Values{"name": {".."}})))
	if !hasNotice(taken.Notices, "old is already here") || !hasNotice(bad.Notices, "Nothing was created") {
		t.Errorf("taken %+v, bad %+v", taken.Notices, bad.Notices)
	}
}

func opSheet(t *testing.T, f davFixture, href string, form url.Values) view.FileOpSheet {
	t.Helper()
	return decode[view.FileOpSheet](t, f.h.do(postForm(href, f.sess, form)))
}

func confirmHref(t *testing.T, sheet view.FileOpSheet) string {
	t.Helper()
	a := sheet.Actions[0]
	if !a.Enabled || a.Href == "" {
		t.Fatalf("confirm = %+v (warning %q)", a, sheet.Warning)
	}
	return a.Href
}

// S-17.1: rename and move inside one server are MOVE, without traffic; a copy
// is GET then PUT; a folder does not go inside itself; a taken name is a
// question; delete sends the version the sheet read and keeps a file changed
// since.
func TestFileOperationsOnOneServer(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a.txt", "alpha")
	f.srv.SeedFile(filesDocs+"b.txt", "beta")
	f.srv.SeedFile(filesDocs+"sub/inner.txt", "i")
	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))

	rename := opSheet(t, f, rowMenu(t, fileRow(t, page, "a.txt"), "file.rename"), url.Values{})
	if rename.Op != view.FileRename || len(rename.Items) != 1 || rename.Target != nil {
		t.Fatalf("rename sheet = %+v", rename)
	}
	if rec := f.h.do(postForm(confirmHref(t, rename), f.sess, url.Values{"name": {"c.txt"}})); rec.Code != http.StatusSeeOther {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body.String())
	}
	if got, _ := f.srv.File(filesDocs + "c.txt"); got != "alpha" || f.srv.CountRequests("MOVE", filesDocs) != 1 {
		t.Fatalf("rename: c.txt %q, MOVE %d", got, f.srv.CountRequests("MOVE", filesDocs))
	}

	page = decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	ren := opSheet(t, f, rowMenu(t, fileRow(t, page, "c.txt"), "file.rename"), url.Values{})
	f.h.do(postForm(confirmHref(t, ren), f.sess, url.Values{"name": {"b.txt"}}))
	page = decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	ask := queueRow(t, page.Queue, "b.txt")
	if ask.State != view.OpIdle {
		t.Fatalf("rename onto a taken name = %+v", ask)
	}
	f.h.do(postForm(option(t, ask, "transfer.overwrite"), f.sess, url.Values{}))
	if got, _ := f.srv.File(filesDocs + "b.txt"); got != "alpha" {
		t.Errorf("after overwrite b.txt = %q", got)
	}

	move := opSheet(t, f, rowMenu(t, fileRow(t, page, "sub"), "file.move"), url.Values{})
	if move.Target == nil || move.Actions[0].Enabled || move.Actions[0].Reason != "These items are already here." {
		t.Errorf("move to its own folder = %+v", move.Actions[0])
	}
	into := opSheet(t, f, rowMenu(t, fileRow(t, page, "sub"), "file.move"), url.Values{"to": {f.at("", "sub", "")}})
	if into.Actions[0].Enabled || into.Actions[0].Reason != "A folder cannot go inside itself." {
		t.Errorf("move into itself = %+v", into.Actions[0])
	}
	if into.Target.Chosen != f.at("", "sub", "") || len(into.Target.Nodes) != 1 || !into.Target.Nodes[0].Expanded ||
		len(into.Target.Nodes[0].Children) != 1 || into.Target.Nodes[0].Children[0].ID != f.at("", "sub", "") {
		t.Errorf("tree = %+v", into.Target)
	}

	moveB := opSheet(t, f, rowMenu(t, fileRow(t, page, "b.txt"), "file.move"), url.Values{"to": {f.at("", "sub", "")}})
	if moveB.Warning != "" {
		t.Errorf("a move inside one server warned: %q", moveB.Warning)
	}
	moves, puts := f.srv.CountRequests("MOVE", filesDocs), f.srv.CountRequests("PUT", filesDocs)
	f.h.do(postForm(confirmHref(t, moveB), f.sess, url.Values{}))
	if got, _ := f.srv.File(filesDocs + "sub/b.txt"); got != "alpha" || f.srv.CountRequests("MOVE", filesDocs) != moves+1 || f.srv.CountRequests("PUT", filesDocs) != puts {
		t.Errorf("move: %q, MOVE %d, PUT %d", got, f.srv.CountRequests("MOVE", filesDocs)-moves, f.srv.CountRequests("PUT", filesDocs)-puts)
	}

	page = decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	copySub := opSheet(t, f, rowMenu(t, fileRow(t, page, "sub"), "file.copy"), url.Values{})
	f.h.do(postForm(confirmHref(t, copySub), f.sess, url.Values{}))
	page = decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	f.h.do(postForm(option(t, queueRow(t, page.Queue, "sub"), "transfer.both"), f.sess, url.Values{}))
	if got, _ := f.srv.File(filesDocs + "sub-2/inner.txt"); got != "i" {
		t.Errorf("copy of the folder: inner.txt %q", got)
	}
	if got, _ := f.srv.File(filesDocs + "sub-2/b.txt"); got != "alpha" {
		t.Errorf("copy of the folder: b.txt %q", got)
	}

	page = decode[view.FilesPage](t, f.h.do(get(f.at("", "sub", ""), f.sess)))
	del := opSheet(t, f, rowMenu(t, fileRow(t, page, "inner.txt"), "file.delete"), url.Values{})
	if !del.Actions[0].Danger || !strings.Contains(string(del.Warning), "no index") {
		t.Errorf("delete sheet = %+v", del)
	}
	f.srv.SeedFile(filesDocs+"sub/inner.txt", "changed elsewhere, longer")
	stale := decode[struct {
		Target string         `json:"target"`
		Body   view.FilesPage `json:"body"`
	}](t, f.h.do(fragment(postForm(confirmHref(t, del), f.sess, url.Values{}), "main")))
	if stale.Target != "main" || !hasNotice(stale.Body.Notices, "0 of 1 item deleted") {
		t.Errorf("stale delete: %+v", stale.Body.Notices)
	}
	if _, ok := f.srv.File(filesDocs + "sub/inner.txt"); !ok {
		t.Fatal("a file changed since the sheet was deleted")
	}
	del = opSheet(t, f, rowMenu(t, fileRow(t, page, "inner.txt"), "file.delete"), url.Values{})
	f.h.do(postForm(confirmHref(t, del), f.sess, url.Values{}))
	if _, ok := f.srv.File(filesDocs + "sub/inner.txt"); ok {
		t.Error("delete left the file")
	}
}

// S-17.1: between storages there is no MOVE. The sheet says so before it
// starts; the file travels as GET and PUT and the original goes only after.
func TestMoveBetweenStorages(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a.txt", "alpha")
	far := searchServer(t)
	far.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	far.SeedFolder(filesDocs)
	if rec := f.h.do(postForm("/app/settings/connections", f.sess, url.Values{
		"server": {far.URL}, "login": {far.Login}, "password": {far.Password},
	})); rec.Code != http.StatusSeeOther {
		t.Fatalf("second connection: %d", rec.Code)
	}
	key, _ := f.sess.DataKey()
	blob, _ := f.h.users.Catalog(f.sess.AccountID(), key)
	farID := blob.Connections[1].ID
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		b.Collections = append(b.Collections, sealed.Collection{AccountID: farID, Path: filesDocs, Name: "Far", Kinds: []view.Kind{view.KindFile}})
	})
	farRoot := "/app/files/" + farID + "/" + url.PathEscape(filesDocs) + "/"

	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	sheet := opSheet(t, f, rowMenu(t, fileRow(t, page, "a.txt"), "file.move"), url.Values{"to": {farRoot}})
	if !strings.Contains(string(sheet.Warning), "another storage") || len(sheet.Target.Nodes) != 2 {
		t.Fatalf("sheet = %+v", sheet)
	}
	f.h.do(postForm(confirmHref(t, sheet), f.sess, url.Values{}))
	if got, _ := far.File(filesDocs + "a.txt"); got != "alpha" {
		t.Errorf("far a.txt = %q", got)
	}
	if _, ok := f.srv.File(filesDocs + "a.txt"); ok {
		t.Error("the original stayed after the move")
	}
	if f.srv.CountRequests("MOVE", "") != 0 || far.CountRequests("MOVE", "") != 0 {
		t.Error("a move between storages sent MOVE")
	}
	if row := queueRow(t, decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess))).Queue, "a.txt"); row.State != view.OpDone {
		t.Errorf("row = %+v", row)
	}
}

// S-17.1: a folder arrives as a set of files, each named by its path inside
// it. The folders are created before the files, each file gets its own queue
// row named by that path, and a refusal on one is not a refusal of the rest.
// Part.FileName would have cut the path off (RFC 7578 §4.2), so the name is
// read from the part's own header.
func TestFolderUploadKeepsThePathsInside(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"trip/taken.txt", "original")

	rec := f.h.do(uploadForm(f.at("upload"), f.sess, false,
		upload{"trip/a.txt", "first"},
		upload{"trip/2020/b.txt", "second"},
		upload{"trip/2020/c.txt", "third"},
		upload{"trip/taken.txt", "mine"},
		upload{"loose.txt", "flat"}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	for path, want := range map[string]string{
		filesDocs + "trip/a.txt":      "first",
		filesDocs + "trip/2020/b.txt": "second",
		filesDocs + "trip/2020/c.txt": "third",
		filesDocs + "loose.txt":       "flat",
		filesDocs + "trip/taken.txt":  "original", // never overwritten (S-17.3)
	} {
		if got, ok := f.srv.File(path); got != want {
			t.Errorf("%s = %q, %v; want %q", path, got, ok, want)
		}
	}

	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	// The rows are named by the path, so two files called the same are told
	// apart, and the taken one asks about itself by that path.
	deep := queueRow(t, page.Queue, "trip/2020/b.txt")
	flat := queueRow(t, page.Queue, "loose.txt")
	ask := queueRow(t, page.Queue, "trip/taken.txt")
	if deep.State != view.OpDone || flat.State != view.OpDone || ask.State != view.OpIdle {
		t.Fatalf("queue = %+v", page.Queue)
	}
	if ask.Question == nil || string(ask.Question.Name) != "trip/taken.txt" {
		t.Fatalf("question = %+v", ask.Question)
	}

	// Keep both puts the second file beside the first one, in its own folder.
	if rec := f.h.do(postForm(option(t, ask, "transfer.both"), f.sess, url.Values{})); rec.Code != http.StatusSeeOther {
		t.Fatalf("keep both: %d %s", rec.Code, rec.Body.String())
	}
	if got, _ := f.srv.File(filesDocs + "trip/taken-2.txt"); got != "mine" {
		t.Errorf("trip/taken-2.txt = %q", got)
	}
	if got, _ := f.srv.File(filesDocs + "trip/taken.txt"); got != "original" {
		t.Errorf("trip/taken.txt = %q, want the original", got)
	}
}

// S-17.4 and S-17.1: a part that names a way out of the folder is refused, and
// says so in its own row; an address of the person's own machine is a name,
// not a path. Neither stops the file that came with it.
func TestFolderUploadRefusesAWayOut(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFolder(filesDocs)
	rec := f.h.do(uploadForm(f.at("upload"), f.sess, false,
		upload{"../escape.txt", "no"},
		upload{"trip/../../escape.txt", "no"},
		upload{`C:\fakepath\local.txt`, "yes"},
		upload{"/etc/passwd", "no"},
		upload{"good.txt", "yes"}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	for path, want := range map[string]string{
		filesDocs + "local.txt": "yes",
		filesDocs + "good.txt":  "yes",
		filesDocs + "passwd":    "no", // the leading slash made it a name
	} {
		if got, ok := f.srv.File(path); got != want {
			t.Errorf("%s = %q, %v; want %q", path, got, ok, want)
		}
	}
	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	for _, name := range []string{"../escape.txt", "trip/../../escape.txt"} {
		row := queueRow(t, page.Queue, name)
		if row.State != view.OpFailed {
			t.Errorf("%s = %+v, want a refusal", name, row)
		}
	}
	if _, ok := f.srv.File(davtest.FileRoot + "escape.txt"); ok {
		t.Error("a file landed outside the folder")
	}
}
