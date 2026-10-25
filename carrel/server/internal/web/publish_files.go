package web

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/file"
	"carrel.dev/carrel/server/internal/publish"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// S-25.1 for the fifth kind: a file or a folder. A folder's page lists it and
// goes down into its folders, never up; a file of it streams from
// /p/{token}/file/{path…}; /download is the file, or the folder as a zip
// written as it is read; and /p/{token}/dav/ mounts the folder as a drive
// that can only be read. Every path is checked a segment at a time and stays
// under what was published (S-17.4).

// relSegs reads a path relative to the published thing: names only.
func relSegs(rel string) ([]string, bool) {
	rel = strings.Trim(rel, "/")
	if rel == "" {
		return nil, true
	}
	var out []string
	for _, seg := range strings.Split(rel, "/") {
		if file.CheckName(seg) != nil {
			if file.Escapes(seg) {
				slog.Warn("web: a path that leaves its collection was refused", "path", rel)
			}
			return nil, false
		}
		out = append(out, seg)
	}
	return out, true
}

// filePublishQuery names a file or a folder for the sheet: f= is the path
// under the collection root, dir=1 a folder.
func filePublishQuery(c *fileCtx, segs []string, dir bool) url.Values {
	q := url.Values{"k": {filesSpec.segment}, "a": {c.conn.ID}, "c": {c.col.Path}, "f": {strings.Join(segs, "/")}}
	if dir {
		q.Set("dir", "1")
	}
	return q
}

func (s *Server) filePublishAction(c *fileCtx, segs []string, dir bool) view.Action {
	if c.col.Feed {
		return view.Action{ID: "publish", Label: "Publish a link", Icon: "i-link", Method: "GET", Reason: "Somebody else's folder is not published onwards."}
	}
	return view.Action{ID: "publish", Label: "Publish a link", Icon: "i-link", Method: "GET", Enabled: true,
		Href: s.postQuery("/app/publish", filePublishQuery(c, segs, dir))}
}

func (s *Server) publishFileTarget(r *http.Request, sess *session.Session) (*fileCtx, []string, bool, bool) {
	c, ok := s.fileCollection(sess, r.FormValue("a"), r.FormValue("c"))
	if !ok || c.col.Feed {
		return nil, nil, false, false // someone else's folder is not published onwards
	}
	segs, ok := relSegs(r.FormValue("f"))
	if !ok {
		return nil, nil, false, false
	}
	return c, segs, len(segs) == 0 || r.FormValue("dir") != "", true
}

func fileTitle(c *fileCtx, segs []string) string {
	if len(segs) == 0 {
		return collectionLabel(c.col)
	}
	return segs[len(segs)-1]
}

func (s *Server) fileSheet(ctx context.Context, sess *session.Session, c *fileCtx, segs []string, dir bool) view.PublishSheet {
	what, subject := "file", fileTitle(c, segs)
	if dir {
		what = "folder"
		// A folder says how much is in it (Пб1; s-publish-5: «Материалы к
		// релизу · 12 files»); unread, its name alone.
		if p, err := c.path(segs, true); err == nil {
			if entries, err := c.files.List(ctx, p); err == nil {
				subject += " · " + plural(len(entries), "file")
			}
		}
	}
	back := c.href("", nil, true)
	if len(segs) > 0 {
		back = c.href("", segs[:len(segs)-1], true)
	}
	sheet := s.sheetFor(sess, view.KindFile, what, subject, filePublishQuery(c, segs, dir), back, "")
	if dir {
		// What the visitor gets, said on the sheet before the link and after
		// it (s-publish-5): the same secret mounts the folder (S-25.1).
		sheet.Note = "The visitor gets the folder with everything inside it, read-only, and the same address mounts as a drive in Windows, macOS and Linux."
	}
	return sheet
}

// postPublishFile makes the link to a file or a folder that is there now.
func (s *Server) postPublishFile(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	c, segs, dir, ok := s.publishFileTarget(r, sess)
	if !ok {
		s.notFound(w, r)
		return
	}
	expires := s.publishExpiry(r, livesOf(view.KindFile, dir))
	p, err := c.path(segs, dir)
	if err != nil {
		s.notFound(w, r)
		return
	}
	if len(segs) > 0 {
		e, err := c.files.Stat(r.Context(), p)
		if err != nil || e.Dir != dir {
			s.notFound(w, r)
			return
		}
	}
	title := fileTitle(c, segs)
	key, _ := sess.DataKey()
	token, err := s.users.Publish(sess.AccountID(), key, c.conn, user.PublishTarget{
		AccountID: c.conn.ID, Kind: string(view.KindFile), Col: c.col.Path, Path: strings.Join(segs, "/"), Folder: dir, Title: title,
	}, expires)
	if err != nil {
		http.Error(w, "could not publish", http.StatusInternalServerError)
		return
	}
	s.writeSheet(w, r, s.madeSheet(r, sess, s.fileSheet(r.Context(), sess, c, segs, dir), token, expires))
}

// publicFiles is the file collection of a link, at the published thing.
func (s *Server) publicFiles(link user.OpenLink) (*fileCtx, []string, error) {
	client, err := dav.NewClient(s.guard, link.Conn.Server, link.Conn.Login, link.Conn.Password, link.Conn.CertFingerprint)
	if err != nil {
		return nil, nil, err
	}
	files, err := file.New(client)
	if err != nil {
		return nil, nil, err
	}
	base, _ := relSegs(link.Target.Path)
	col := sealed.Collection{AccountID: link.Target.AccountID, Path: link.Target.Col, Name: link.Target.Title, Kinds: []view.Kind{view.KindFile}}
	return &fileCtx{conn: link.Conn, col: col, files: files, tf: s.timeFor(nil), segs: base, dir: link.Target.Folder, root: col.Path}, base, nil
}

// inside is the server path of rel under a published folder, and its
// segments under the collection; false for anything that is not a name.
func inside(c *fileCtx, base []string, rel string, dir bool) (string, []string, bool) {
	more, ok := relSegs(rel)
	if !ok {
		return "", nil, false
	}
	segs := append(slices.Clone(base), more...)
	p, err := c.path(segs, dir)
	return p, more, err == nil
}

// publishedFilesPage is the page of a file or a folder link.
func (s *Server) publishedFilesPage(w http.ResponseWriter, r *http.Request, link user.OpenLink) {
	token := r.PathValue("token")
	page := view.PublishedPage{
		Plain:    view.Plain{Title: text(link.Target.Title)},
		Kind:     view.KindFile,
		ReadOnly: true,
	}
	c, base, err := s.publicFiles(link)
	if c != nil {
		page.Foot = publicFoot(c.tf, link)
	}
	if err != nil {
		page.Notices = []view.Notice{publicTrouble()}
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	ctx := r.Context()
	if !link.Target.Folder {
		p, _ := c.path(base, false)
		s.publicFile(ctx, c, p, s.route("/p/"+token+"/download"), &page)
		s.textWidth(r, "/p/"+token, &page)
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	// A file of the folder opens its own page inside the link (Пс21; the
	// caption of s-publish-12): the page of a file link, its Download the
	// file's stream.
	if rel := r.URL.Query().Get("file"); rel != "" {
		p, more, ok := inside(c, base, rel, false)
		if !ok || len(more) == 0 {
			s.tokenNotFound(w, r, time.Now())
			return
		}
		page.Title = text(more[len(more)-1])
		s.publicFile(ctx, c, p, s.route("/p/"+token+"/file/"+escapeSegs(strings.Join(more, "/"))), &page)
		s.textWidth(r, s.postQuery("/p/"+token, url.Values{"file": {rel}}), &page)
		s.writePage(w, r, http.StatusOK, page)
		return
	}

	in := r.URL.Query().Get("in")
	p, more, ok := inside(c, base, in, true)
	if !ok {
		s.tokenNotFound(w, r, time.Now())
		return
	}
	entries, err := c.files.List(ctx, p)
	switch {
	case file.NotFound(err):
		page.Notices, page.Foot = []view.Notice{publicGone()}, nil
		s.writePage(w, r, http.StatusOK, page)
		return
	case err != nil:
		page.Notices = []view.Notice{publicTrouble()}
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	here := s.route("/p/" + token)
	// The address that mounts the folder whole, with the host — what goes
	// into «Map network drive» (Пс17; s-publish-12) —, one for the whole
	// link, and how to mount it on each system (s-publish-32).
	page.Strip = &view.PublishedStrip{Label: "Mount as a drive", Address: text(s.absURL(r, "/p/"+token+"/dav/")), Mount: mountTexts, Note: "read-only WebDAV, no password"}
	folder := &view.PublishedFolder{}
	folder.Crumbs = []view.Crumb{{Label: text(link.Target.Title), Href: here}}
	for i, seg := range more {
		folder.Crumbs = append(folder.Crumbs, view.Crumb{Label: text(seg), Href: s.postQuery("/p/"+token, url.Values{"in": {strings.Join(more[:i+1], "/")}})})
	}
	folder.Crumbs[len(folder.Crumbs)-1].Href = ""
	slices.SortStableFunc(entries, func(a, b file.Entry) int {
		if a.Dir != b.Dir {
			if a.Dir {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	// After the crumb of where the guest is: how many and how much, of the
	// whole listing (Пс16; s-publish-12, -31). The folders count: «items».
	var bytes int64
	for _, e := range entries {
		bytes += max(e.Size, 0)
	}
	folder.Total = text(itemsWord(len(entries)))
	if len(entries) > 0 {
		folder.Total += " · " + text(string(formatSize(bytes)))
	}
	nested := strings.Join(more, "/")
	// «Filter this folder» sifts the whole listing by name, before the cut
	// (s-publish-31; note 122 p. 10).
	q := strings.TrimSpace(r.URL.Query().Get("filter"))
	if len(entries) > 0 || q != "" {
		page.Filter = s.publicFilter(here, "Filter this folder", q, nested)
	}
	if q != "" {
		entries = slices.DeleteFunc(entries, func(e file.Entry) bool {
			return !strings.Contains(strings.ToLower(e.Name), strings.ToLower(q))
		})
	}
	if n := len(entries); n > s.cfg.FilesMaxEntries {
		// Said, not cut silently (Пс22): the cut is the page's alone — the
		// archive, the drive and the filter have the whole folder (the
		// owner's decision of 24.10.2026), and the words name them.
		entries = entries[:s.cfg.FilesMaxEntries]
		page.Notices = append(page.Notices, view.Notice{
			Role:  view.NoticeInfo,
			Title: text("Showing " + grouped(s.cfg.FilesMaxEntries) + " of " + grouped(n) + "."),
			Body:  "Filter by name, download the folder or mount it to get all of them.",
		})
	}
	for _, e := range entries {
		rel := strings.Join(append(slices.Clone(more), e.Name), "/")
		// The moment in figures and the icon by type, as the owner's own
		// folder (Пс19, Пс20). The name opens it in the link — a folder its
		// listing, a file its page —; Download hands the file over, or the
		// folder's archive (s-publish-12).
		row := view.FileRow{Name: text(e.Name), IsDir: e.Dir, Modified: c.tf.numStamp(e.Modified), Icon: fileIcon(e)}
		if e.Dir {
			row.Type = "folder"
			if e.Items != nil {
				row.Items = text(itemsWord(*e.Items))
				row.Type += " · " + row.Items
			}
			row.Open = s.postQuery("/p/"+token, url.Values{"in": {rel}})
			row.Href = s.postQuery("/p/"+token+"/download", url.Values{"in": {rel}})
		} else {
			mt := fileType(e.Name, e.Type)
			row.Type, row.Size = text(mt), formatSize(e.Size)
			row.Href = s.route("/p/" + token + "/file/" + escapeSegs(rel))
			row.Open = s.postQuery("/p/"+token, url.Values{"file": {rel}}) // its page (Пс21)
		}
		folder.Rows = append(folder.Rows, row)
	}
	page.Folder = folder
	if len(folder.Rows) == 0 && q == "" {
		// Nothing in it: no archive to give (s-publish-36).
		page.Empty = "This folder is empty."
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	dl := s.route("/p/" + token + "/download")
	if in != "" {
		dl = s.postQuery("/p/"+token+"/download", url.Values{"in": {nested}})
	}
	page.Actions = []view.Action{{ID: "published.download", Label: "Download folder", Icon: "i-download", Href: dl, Method: "GET", Enabled: true}}
	s.writePage(w, r, http.StatusOK, page)
}

// textWidth gives a text read on the page its width by address (s-publish-10).
func (s *Server) textWidth(r *http.Request, here string, page *view.PublishedPage) {
	if f := page.File; f != nil && (f.Preview == view.PreviewText || f.Preview == view.PreviewMarkdown) {
		page.Width = s.publicWidth(r, here)
	}
}

// mountTexts are «How to mount» of a published folder, a line for each
// system (s-publish-32).
var mountTexts = []view.MenuText{
	{Title: "Windows", Text: "File Explorer › This PC › Map network drive, then paste the address."},
	{Title: "macOS", Text: "Finder › Go › Connect to Server, then paste the address."},
	{Title: "Linux", Text: "Files › Other Locations › Connect to Server, then type the address with davs:// in place of https://."},
}

// itemsWord counts what a folder holds — files and folders alike: «1
// item», «2 340 items» (s-publish-12, -31).
func itemsWord(n int) string {
	if n == 1 {
		return "1 item"
	}
	return grouped(n) + " items"
}

// publicFile fills a public page with one file at p (s-publish-9…11): its
// name, type, size and day under the title, the picture or the text in the
// page, Download at download — the link's file, or a file of its folder.
func (s *Server) publicFile(ctx context.Context, c *fileCtx, p, download string, page *view.PublishedPage) {
	e, err := c.files.Stat(ctx, p)
	switch {
	case file.NotFound(err):
		page.Notices, page.Foot = []view.Notice{publicGone()}, nil
		return
	case err != nil:
		page.Notices = []view.Notice{publicTrouble()}
		return
	}
	mt := fileType(e.Name, e.Type)
	page.File = &view.PublishedFile{Name: text(e.Name), Size: formatSize(e.Size), Type: text(mt), Preview: view.PreviewNone, Href: download}
	if !e.Modified.IsZero() {
		page.File.Changed = view.When(c.tf.dayKey(e.Modified)) // «changed 2026-08-10» (s-publish-9)
	}
	// The line under the name is one monospaced piece (s-publish-9…11):
	// the type, the size, and when it changed.
	head := []string{mt, string(page.File.Size)}
	if page.File.Changed != "" {
		head = append(head, "changed "+string(page.File.Changed))
	}
	page.Lede = runs(monoRun(strings.Join(head, " · ")))
	switch {
	case mt == "application/pdf":
		// The browser shows a PDF itself, in the page's frame, the stream
		// of Download with its own type (s-publish-33).
		page.File.Preview, page.File.Href = view.PreviewPDF, download+"?inline=1"
	case inlineTypes[mt]:
		page.File.Preview, page.File.Href = view.PreviewImage, download+"?inline=1"
	}
	// Text and Markdown are the body of the page (Пс14; s-publish-10): the
	// one kind read without a program; too large, it is a file without a
	// preview with its own line (s-publish-34).
	if !s.publicText(ctx, c, p, e, mt, page.File) && page.File.Preview == view.PreviewNone {
		page.File.Note = "A browser cannot show this file." // s-publish-11
		if mt == "text/plain" || mt == "text/markdown" {
			page.File.Note = "Too large to show on the page."
		}
	}
	// The one door of the page (s-publish-9); a file the browser is not
	// shown carries its size on it, before anyone presses it (s-publish-11).
	label := view.Text("Download")
	if page.File.Preview == view.PreviewNone {
		label += " " + view.Text(page.File.Size)
	}
	// Under a text read on the page the button is a second door, not the
	// main one (s-publish-10).
	reading := page.File.Preview == view.PreviewText || page.File.Preview == view.PreviewMarkdown
	page.Actions = []view.Action{{ID: "published.download", Label: label, Icon: "i-download", Href: download, Method: "GET", Primary: !reading, Enabled: true}}
}

func escapeSegs(rel string) string {
	parts := strings.Split(rel, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

// getPublishedFile streams one file of a published folder.
func (s *Server) getPublishedFile(w http.ResponseWriter, r *http.Request) {
	link, ok := s.openLink(w, r)
	if !ok {
		return
	}
	start := time.Now()
	if link.Target.Kind != string(view.KindFile) || !link.Target.Folder {
		s.tokenNotFound(w, r, start)
		return
	}
	c, base, err := s.publicFiles(link)
	if err != nil {
		s.tokenNotFound(w, r, start)
		return
	}
	rel := r.PathValue("path")
	p, more, ok := inside(c, base, rel, false)
	if !ok || len(more) == 0 {
		s.tokenNotFound(w, r, start)
		return
	}
	if err := s.sendFile(w, r, c, p, more[len(more)-1], r.URL.Query().Get("inline") != ""); err != nil {
		s.notFound(w, r)
	}
}

// publishedFilesDownload is the file of a file link, or the folder — or a
// folder in it, ?in= — as a zip written while it is read.
func (s *Server) publishedFilesDownload(w http.ResponseWriter, r *http.Request, link user.OpenLink) {
	start := time.Now()
	c, base, err := s.publicFiles(link)
	if err != nil {
		s.tokenNotFound(w, r, start)
		return
	}
	if !link.Target.Folder {
		p, _ := c.path(base, false)
		if err := s.sendFile(w, r, c, p, link.Target.Title, r.URL.Query().Get("inline") != ""); err != nil {
			s.notFound(w, r)
		}
		return
	}
	p, more, ok := inside(c, base, r.URL.Query().Get("in"), true)
	if !ok {
		s.tokenNotFound(w, r, start)
		return
	}
	name := link.Target.Title
	if len(more) > 0 {
		name = more[len(more)-1]
	}
	if err := s.sendZip(w, r, c, p, name); err != nil {
		s.notFound(w, r)
	}
}

// publishedDAV is the read-only mount of a published folder: OPTIONS, GET,
// HEAD and PROPFIND; any other method is 405. The listing is Carrel's own
// multistatus, not the storage server's.
func (s *Server) publishedDAV(w http.ResponseWriter, r *http.Request) {
	link, ok := s.openLink(w, r)
	if !ok {
		return
	}
	start := time.Now()
	if link.Target.Kind != string(view.KindFile) || !link.Target.Folder {
		s.tokenNotFound(w, r, start)
		return
	}
	w.Header().Set("DAV", "1")
	w.Header().Set("Allow", publish.DAVMethods)
	switch r.Method {
	case http.MethodOptions:
		w.WriteHeader(http.StatusOK)
		return
	case http.MethodGet, http.MethodHead, "PROPFIND":
	default:
		http.Error(w, "read-only", http.StatusMethodNotAllowed)
		return
	}
	c, base, err := s.publicFiles(link)
	if err != nil {
		http.Error(w, "unavailable", http.StatusBadGateway)
		return
	}
	rel := r.PathValue("path")
	dir := rel == "" || strings.HasSuffix(rel, "/")
	p, more, ok := inside(c, base, rel, dir)
	if !ok {
		s.notFound(w, r)
		return
	}
	ctx := r.Context()
	root := s.route("/p/" + r.PathValue("token") + "/dav/")
	hrefOf := func(segs []string, isDir bool) string {
		h := root + escapeSegs(strings.Join(segs, "/"))
		if isDir && len(segs) > 0 {
			h += "/"
		}
		return h
	}
	if r.Method != "PROPFIND" {
		if dir {
			http.Error(w, "a folder", http.StatusMethodNotAllowed)
			return
		}
		if err := s.sendFile(w, r, c, p, more[len(more)-1], false); err != nil {
			s.notFound(w, r)
		}
		return
	}
	var entries []publish.DAVEntry
	self := publish.DAVEntry{Href: hrefOf(more, dir), Dir: dir, Name: link.Target.Title}
	if len(more) > 0 {
		self.Name = more[len(more)-1]
	}
	if dir {
		list, err := c.files.List(ctx, p)
		if file.NotFound(err) {
			s.notFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "unavailable", http.StatusBadGateway)
			return
		}
		entries = append(entries, self)
		if r.Header.Get("Depth") != "0" {
			// The drive has the folder whole: the cap is the page's listing
			// alone (the owner's decision of 24.10.2026, s-publish-31).
			for _, e := range list {
				segs := append(slices.Clone(more), e.Name)
				entries = append(entries, publish.DAVEntry{Href: hrefOf(segs, e.Dir), Name: e.Name, Dir: e.Dir, Size: e.Size,
					Type: fileType(e.Name, e.Type), ETag: e.ETag, Modified: e.Modified})
			}
		}
	} else {
		e, err := c.files.Stat(ctx, p)
		if err != nil || e.Dir {
			s.notFound(w, r)
			return
		}
		self.Size, self.Type, self.ETag, self.Modified = e.Size, fileType(e.Name, e.Type), e.ETag, e.Modified
		entries = append(entries, self)
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = w.Write(publish.Multistatus(entries))
}

// publicTextMax is the largest text a public page shows as its body; a
// larger one is offered to download, as any file (s-publish-10).
const publicTextMax = 256 << 10

// publicText reads a text or Markdown file into the page: its words, or its
// blocks parsed. False — not text, too large, not UTF-8 or holding a NUL
// (binary under a text name), or not read: the page offers the file to
// download.
func (s *Server) publicText(ctx context.Context, c *fileCtx, p string, e file.Entry, mt string, f *view.PublishedFile) bool {
	kind := map[string]view.PreviewKind{"text/plain": view.PreviewText, "text/markdown": view.PreviewMarkdown}[mt]
	if kind == "" || e.Size > publicTextMax {
		return false
	}
	body, _, err := c.files.Get(ctx, p, nil)
	if err != nil {
		return false
	}
	defer body.Close()
	data, err := io.ReadAll(io.LimitReader(body, publicTextMax+1))
	if err != nil || len(data) > publicTextMax || !utf8.Valid(data) || slices.Contains(data, 0) {
		return false
	}
	f.Preview = kind
	if kind == view.PreviewMarkdown {
		f.Blocks = cal.RenderMarkdown(string(data))
	} else {
		f.Body = text(string(data))
	}
	return true
}
