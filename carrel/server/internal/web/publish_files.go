package web

import (
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/dav"
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

func (s *Server) fileSheet(sess *session.Session, c *fileCtx, segs []string, dir bool) view.PublishSheet {
	what := "file"
	if dir {
		what = "folder"
	}
	back := c.href("", nil, true)
	if len(segs) > 0 {
		back = c.href("", segs[:len(segs)-1], true)
	}
	sheet := s.sheetFor(sess, view.KindFile, what, fileTitle(c, segs), c.stamp(), filePublishQuery(c, segs, dir), back, "")
	if dir {
		sheet.Expires += text(". Whoever has the link can also mount the folder as a read-only drive.")
	}
	return sheet
}

// postPublishFile makes the link to a file or a folder that is there now.
func (s *Server) postPublishFile(w http.ResponseWriter, r *http.Request, sess *session.Session, expires *time.Time) {
	c, segs, dir, ok := s.publishFileTarget(r, sess)
	if !ok {
		s.notFound(w, r)
		return
	}
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
	s.writeSheet(w, r, s.madeSheet(sess, s.fileSheet(sess, c, segs, dir), token, expires))
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
		Plain:    view.Plain{Title: text(link.Target.Title), Lede: line("Read-only. Published from Carrel.")},
		Kind:     view.KindFile,
		ReadOnly: true,
	}
	c, base, err := s.publicFiles(link)
	if err != nil {
		page.Notices = []view.Notice{publicTrouble()}
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	ctx := r.Context()
	if !link.Target.Folder {
		p, _ := c.path(base, false)
		e, err := c.files.Stat(ctx, p)
		switch {
		case file.NotFound(err):
			page.Notices = []view.Notice{publicGone()}
		case err != nil:
			page.Notices = []view.Notice{publicTrouble()}
		default:
			download := s.route("/p/" + token + "/download")
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
			if inlineTypes[mt] {
				page.File.Preview, page.File.Href = view.PreviewImage, download+"?inline=1"
			}
			// The one door of the page (s-publish-9); a file the browser is
			// not shown carries its size on it, before anyone presses it
			// (s-publish-11).
			label := view.Text("Download")
			if page.File.Preview == view.PreviewNone {
				label += " " + view.Text(page.File.Size)
			}
			page.Actions = []view.Action{{ID: "published.download", Label: label, Icon: "i-down", Href: download, Method: "GET", Primary: true, Enabled: true}}
		}
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
		page.Notices = []view.Notice{publicGone()}
		s.writePage(w, r, http.StatusOK, page)
		return
	case err != nil:
		page.Notices = []view.Notice{publicTrouble()}
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	here := s.route("/p/" + token)
	folder := &view.PublishedFolder{DAV: text(s.route("/p/" + token + "/dav/"))}
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
	if len(entries) > s.cfg.FilesMaxEntries {
		entries = entries[:s.cfg.FilesMaxEntries]
	}
	now := time.Now()
	for _, e := range entries {
		rel := strings.Join(append(slices.Clone(more), e.Name), "/")
		row := view.FileRow{Name: text(e.Name), IsDir: e.Dir, Modified: c.tf.formatWhen(e.Modified, false, now)}
		if e.Dir {
			row.Type, row.Href = "folder", s.postQuery("/p/"+token, url.Values{"in": {rel}})
		} else {
			mt := fileType(e.Name, e.Type)
			row.Type, row.Size = text(mt), formatSize(e.Size)
			row.Href = s.route("/p/" + token + "/file/" + escapeSegs(rel))
		}
		folder.Rows = append(folder.Rows, row)
	}
	page.Folder = folder
	dl := s.route("/p/" + token + "/download")
	if in != "" {
		dl = s.postQuery("/p/"+token+"/download", url.Values{"in": {strings.Join(more, "/")}})
	}
	page.Actions = []view.Action{{ID: "published.download", Label: "Download folder", Icon: "i-down", Href: dl, Method: "GET", Enabled: true}}
	s.writePage(w, r, http.StatusOK, page)
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
			if len(list) > s.cfg.FilesMaxEntries {
				list = list[:s.cfg.FilesMaxEntries]
			}
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
