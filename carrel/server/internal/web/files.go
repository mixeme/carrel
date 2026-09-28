package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/provider/file"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// Files (S-17). A file has a path, and the path belongs to its storage: there
// is no combined view. The root of the section is the ticked file collections
// as top-level folders; past it one collection's own tree opens. A folder is
// one PROPFIND; a file goes through Carrel as a stream, never buffered and
// without a ceiling of Carrel's (C-12). Addresses are read a segment at a time:
// ".." is refused, not cleaned, and what is joined must still lie under the
// collection root (S-17.4).

var filesSpec = kindSpec{segment: "files", kind: view.KindFile, noun: "storage"}

// inlineTypes are the images a browser may show from Carrel as they are: a
// thumbnail is the original file (S-17.1). Anything else, SVG included, is
// only ever an attachment.
var inlineTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true}

// fileCtx is one live file collection of the catalog opened for a request,
// and the address in it.
type fileCtx struct {
	sess  *session.Session
	blob  sealed.Blob
	conn  sealed.Connection
	col   sealed.Collection
	files *file.Provider
	tf    timeFmt
	segs  []string // under the collection root, decoded
	dir   bool     // the address names a folder: it ends in "/"
	// root is the collection's root on its server: its path, or for a
	// subscribed folder the path of the address it was subscribed by —
	// {col} of a subscription is "feed", not a place on any server.
	root string
}

// parseFileAddress reads /app/files/{account}/{col}/[{marker}/]{path…} from an
// escaped path. Every segment is decoded on its own and must be a name: an
// empty one, "." or ".." is a refusal (S-17.4). A trailing slash names a
// folder; the root of the collection is always a folder.
func parseFileAddress(escaped, marker string) (account, col string, segs []string, dir, ok bool) {
	return parseAddress(escaped, "/app/files/", marker)
}

// parseAddress reads {account}/{col}/[{marker}/]{path…} after prefix, the
// same way for the files section and for /app/attach/ (S-20).
func parseAddress(escaped, prefix, marker string) (account, col string, segs []string, dir, ok bool) {
	rest, found := strings.CutPrefix(escaped, prefix)
	if !found {
		return "", "", nil, false, false
	}
	parts := strings.Split(rest, "/")
	if len(parts) < 3 || parts[0] == "" {
		return "", "", nil, false, false
	}
	account = parts[0]
	col, err := url.PathUnescape(parts[1])
	if err != nil || col == "" {
		return "", "", nil, false, false
	}
	parts = parts[2:]
	if marker != "" {
		if len(parts) < 2 || parts[0] != marker {
			return "", "", nil, false, false
		}
		parts = parts[1:]
	}
	dir = parts[len(parts)-1] == ""
	if dir {
		parts = parts[:len(parts)-1]
	}
	for _, p := range parts {
		name, err := url.PathUnescape(p)
		if err != nil || file.CheckName(name) != nil {
			if err == nil && file.Escapes(name) {
				slog.Warn("web: a path that leaves its collection was refused", "path", escaped)
			}
			return "", "", nil, false, false
		}
		segs = append(segs, name)
	}
	if len(segs) == 0 && !dir {
		return "", "", nil, false, false
	}
	return account, col, segs, dir, true
}

// fileHref is an address of the files section: the collection as one
// segment, the path under it a segment at a time (C-4.1). A folder ends in
// "/"; the root of a collection is "…/{col}/".
func fileHref(account, colPath, marker string, segs []string, dir bool) string {
	var b strings.Builder
	b.WriteString("/app/files/" + account + "/" + url.PathEscape(colPath) + "/")
	if marker != "" {
		b.WriteString(marker + "/")
	}
	for i, s := range segs {
		if i > 0 {
			b.WriteString("/")
		}
		b.WriteString(url.PathEscape(s))
	}
	if dir && len(segs) > 0 {
		b.WriteString("/")
	}
	return b.String()
}

// openFiles opens the file collection an address names and checks it is the
// person's. Anything foreign, gone or malformed is 404, like nothing at all.
func (s *Server) openFiles(w http.ResponseWriter, r *http.Request, marker string) (*fileCtx, bool) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return nil, false
	}
	account, col, segs, dir, ok := parseFileAddress(r.URL.EscapedPath(), marker)
	if !ok {
		s.notFound(w, r)
		return nil, false
	}
	if !s.checkOwnership(w, r, sess, account, col, "") {
		return nil, false
	}
	c, ok := s.fileCollection(sess, account, col)
	if !ok {
		s.notFound(w, r)
		return nil, false
	}
	c.segs, c.dir = segs, dir
	return c, true
}

// fileCollection is a live file collection of the catalog with a provider over
// a guarded client, at its root.
func (s *Server) fileCollection(sess *session.Session, account, colPath string) (*fileCtx, bool) {
	key, live := sess.DataKey()
	blob, opened := s.users.Catalog(sess.AccountID(), key)
	conn, hasConn := blob.Connection(account)
	col, hasCol := blob.Collection(account, colPath)
	if !live || !opened || !hasConn || !hasCol || col.Gone || !col.Has(view.KindFile) {
		return nil, false
	}
	client, err := dav.NewClient(s.guard, conn.Server, conn.Login, conn.Password, conn.CertFingerprint)
	if err != nil {
		return nil, false
	}
	files, err := file.New(client)
	if err != nil {
		return nil, false
	}
	return &fileCtx{sess: sess, blob: blob, conn: conn, col: col, files: files, tf: s.timeFor(sess), dir: true, root: fileRoot(conn, col)}, true
}

// fileRoot is where a file collection starts on its server.
func fileRoot(conn sealed.Connection, col sealed.Collection) string {
	if col.Feed {
		if u, err := url.Parse(conn.Server); err == nil {
			return u.Path
		}
	}
	return col.Path
}

// path is the server path of segs under the collection root.
func (c *fileCtx) path(segs []string, dir bool) (string, error) {
	if len(segs) == 0 {
		return strings.TrimSuffix(c.root, "/") + "/", nil
	}
	p, err := file.Join(c.root, segs...)
	if err != nil {
		return "", err
	}
	if dir {
		p += "/"
	}
	return p, nil
}

func (c *fileCtx) href(marker string, segs []string, dir bool) string {
	return fileHref(c.conn.ID, c.col.Path, marker, segs, dir)
}

func (c *fileCtx) stamp() view.Stamp {
	return stamp(connLabel(c.conn), collectionLabel(c.col))
}

// at is the same collection at another address.
func (c *fileCtx) at(segs []string) *fileCtx {
	d := *c
	d.segs, d.dir = slices.Clone(segs), true
	return &d
}

// Listings. A folder page reads the server; an address that only filters,
// sorts, switches the view or selects cuts the listing this session read last
// (C-6.4: the filter works on what already arrived).

func listKey(folder string) string { return "files\x00" + folder }

func (s *Server) readFolder(ctx context.Context, c *fileCtx, folder string, fromCache bool) ([]file.Entry, error) {
	if fromCache {
		if data, ok := c.sess.Cache().GetETagMap(c.conn.ID, listKey(folder)); ok {
			var entries []file.Entry
			if json.Unmarshal(data, &entries) == nil {
				return entries, nil
			}
		}
	}
	entries, err := c.files.List(ctx, folder)
	if err != nil {
		return nil, err
	}
	if data, err := json.Marshal(entries); err == nil {
		c.sess.Cache().PutETagMap(c.conn.ID, listKey(folder), data)
	}
	return entries, nil
}

// forgetFolder drops the listing of a folder something was written into.
func forgetFolder(sess *session.Session, connID, folder string) {
	sess.Cache().InvalidatePath(connID, listKey(folder))
}

func onlyCuts(q url.Values) bool {
	return q.Has("filter") || q.Has("sort") || q.Has("view") || q.Has("select")
}

// fileType is the media type of a file without its parameters: the one the
// server sent, else the one its extension names, else none.
func fileType(name, sent string) string {
	for _, mt := range []string{sent, mime.TypeByExtension(path.Ext(name))} {
		if t, _, err := mime.ParseMediaType(mt); err == nil && t != "" {
			return t
		}
	}
	return ""
}

// fileReason is a refusal of the far server in a sentence (C-9.1).
func fileReason(err error) string {
	var se *dav.StatusError
	switch {
	case dav.IsStatus(err, http.StatusForbidden):
		return "This account can read here but not write."
	case dav.IsStatus(err, http.StatusInsufficientStorage):
		return "The storage is full."
	case dav.IsStatus(err, http.StatusRequestEntityTooLarge):
		return "The server refused a file this large."
	case errors.Is(err, dav.ErrResponseTooLarge):
		return "The server's answer is larger than Carrel reads."
	case errors.Is(err, file.ErrTooMany):
		return "The folder holds more than Carrel copies at once (CARREL_FILES_MAX_ENTRIES)."
	case errors.As(err, &se):
		return fmt.Sprintf("The server answered %d %s.", se.Code, http.StatusText(se.Code))
	}
	return "The server could not be reached."
}

func (s *Server) fileFailure(c *fileCtx, title, method, p string, err error) view.Notice {
	return view.Notice{
		Role:  view.NoticeAlert,
		Title: text(title),
		Body:  text(fileReason(err)),
		Details: &view.Diagnostic{
			Summary: text(method + " " + p),
			Lines:   []view.Text{text(connLabel(c.conn) + p), text(err.Error())},
		},
	}
}

var readOnlyNotice = view.Notice{
	Role:  view.NoticeAlert,
	Title: "This storage is read-only",
	Body:  "The server lets this account read here but not write. Nothing was changed.",
}

const readOnlyReason = "This storage is read-only."

// The root of the section (S-17.2).

func (s *Server) getFilesRoot(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	blob := s.catalogOf(sess)
	page := view.FilesRootPage{Shell: s.buildShell(sess, "Files")}
	page.Rail = s.filesRail(sess, blob, nil)
	page.Print = view.Print{Title: page.Title}
	for _, col := range ticked(blob, view.KindFile) {
		conn, _ := blob.Connection(col.AccountID)
		row := view.FileCollectionRow{
			Label:    text(collectionLabel(col)),
			Stamp:    stamp(connLabel(conn), collectionLabel(col)),
			Colour:   catalogColour(col),
			ReadOnly: col.ReadOnly,
			State:    view.CollectionOK,
		}
		switch {
		case col.Gone:
			row.State = view.CollectionGone
		case col.ReadOnly:
			row.State = view.CollectionReadOnly
		}
		if !col.Gone {
			row.Href = s.route(fileHref(conn.ID, col.Path, "", nil, true))
		}
		// What the storage is chosen for: the column the frame names
		// «Purpose» (s-files-6). The contract has carried it all along and
		// nothing filled it.
		if a := blob.Attachments; a != nil && a.AccountID == col.AccountID && a.Path == col.Path {
			row.Purpose = "attachments"
		}
		page.Collections = append(page.Collections, row)
	}
	if len(page.Collections) == 0 {
		page.Empty = filesEmpty(s, blob)
	}
	s.writePage(w, r, http.StatusOK, page)
}

// filesEmpty says why the root is empty and what to do (S-17.2): the section
// never disappears.
func filesEmpty(s *Server, blob sealed.Blob) *view.Notice {
	for _, col := range blob.Collections {
		if _, ok := blob.Connection(col.AccountID); ok && !col.Gone && col.Has(view.KindFile) {
			return &view.Notice{
				Role:  view.NoticeInfo,
				Title: "No storage is ticked",
				Body:  "Tick the storages to show here in the rail. Each opens as its own folder: the trees of different servers are never mixed.",
			}
		}
	}
	return &view.Notice{
		Role:  view.NoticeInfo,
		Title: "No file storage is connected",
		Body:  "Files come from a WebDAV storage. Connect one, then tick its folders in the rail.",
		Actions: []view.Action{
			s.connectAction(),
		},
	}
}

// filesRail is the rail of the section (C-6.3): the sections, the root row
// whose ticks are what the root shows, and the file collections by connection
// with their ticks. No counts: a folder is not a list of records. open is the
// folder shown, nil on the root.
func (s *Server) filesRail(sess *session.Session, blob sealed.Blob, open *fileCtx) view.Rail {
	rail := view.Rail{About: s.aboutAction(), Nav: s.navRows(sess, blob, filesSpec.segment)}
	sections, live, on := s.railSections(sess, blob, filesSpec, nil, nil)
	root := view.SourceRow{Href: s.route("/app/files"), Label: "Selected collections", Tick: view.TickPart, State: view.CollectionOK}
	switch on {
	case 0:
		root.Tick = view.TickOff
	case live:
		root.Tick = view.TickOn
	}
	rail.Sections = append([]view.RailSection{{State: view.CollectionOK, Rows: []view.SourceRow{root}}}, sections...)
	// The foot of s-files-1: New folder where a folder is open, Connect
	// account, Subscribe to a feed — somebody's folder is such a feed, and
	// the form opens for a folder. Apply is the rail's submit without
	// scripts (C-6.3) and is not drawn.
	folder := view.Action{ID: "rail.folder.new", Label: "New folder", Icon: "i-plus", Method: "POST"}
	openHref := ""
	switch {
	case open == nil:
		folder.Reason = "Open a folder to make one in it."
	case open.col.ReadOnly:
		folder.Reason = readOnlyReason
	default:
		folder.Enabled, folder.Href = true, s.route(open.href("mkcol", open.segs, true))
	}
	if open != nil {
		openHref = s.route(open.href("", open.segs, true))
	}
	rail.Foot = []view.Action{
		{ID: "rail.apply", Label: "Apply", Href: s.route("/app/sources/files"), Method: "POST", Enabled: true},
		folder,
		s.connectAction(),
		{ID: "feed.subscribe", Label: "Subscribe to a feed", Icon: "i-link", Href: s.postQuery("/app/settings/connections", url.Values{"op": {"feed"}, "kind": {"folder"}}), Method: "GET", Enabled: true},
	}
	rail.Note = text(fmt.Sprintf("%d of %s ticked", on, plural(live, "storage")))
	rail.Shortcuts = s.filesShortcuts(sess, blob, openHref)
	return rail
}

// filesShortcuts are the rubric Shortcuts of the rail (s-files-1, s-files-6):
// the attachment folder with where it lies, and the published links with
// their count. Without an attachment folder the row leads to where it is
// chosen.
func (s *Server) filesShortcuts(sess *session.Session, blob sealed.Blob, open string) []view.Shortcut {
	attach := view.Shortcut{Action: view.Action{ID: "files.attachments", Label: "Attachments", Icon: "i-file", Href: s.route("/app/settings/attachments"), Method: "GET", Enabled: true}}
	if fc := s.savedFolder(sess, blob); fc != nil {
		attach.Href = s.route(fc.href("", fc.segs, true))
		place := []string{collectionLabel(fc.col)}
		if len(fc.segs) > 1 {
			place = append(place, fc.segs[:len(fc.segs)-1]...)
		}
		attach.Place = text(strings.Join(place, " / "))
		attach.On = open != "" && attach.Href == open
	}
	links := len(s.users.PublishedLinks(sess.AccountID()))
	published := view.Shortcut{Action: view.Action{ID: "files.published", Label: "Published", Icon: "i-link", Href: s.route("/app/files/published"), Method: "GET", Enabled: true}, Count: &links}
	return []view.Shortcut{attach, published}
}

// A folder (C-6.7).

func (s *Server) getFiles(w http.ResponseWriter, r *http.Request) {
	c, ok := s.openFiles(w, r, "")
	if !ok {
		return
	}
	s.showAddress(w, r, c)
}

// showAddress answers a folder, or the address of a file with its folder and
// the file selected — the address a search result links to.
func (s *Server) showAddress(w http.ResponseWriter, r *http.Request, c *fileCtx) {
	if c.dir {
		s.writeFolder(w, r, c, c.segs, "", r.URL.Query(), "")
		return
	}
	s.writeFolder(w, r, c, c.segs[:len(c.segs)-1], c.segs[len(c.segs)-1], r.URL.Query(), "")
}

// writeFolder reads a folder and answers its page: as a page, or as a
// fragment for target when one is asked for. A name to select that is not in
// the folder is 404; one that is a folder goes to that folder.
func (s *Server) writeFolder(w http.ResponseWriter, r *http.Request, c *fileCtx, segs []string, pick string, q url.Values, target string, notices ...view.Notice) {
	folder, err := c.path(segs, true)
	if err != nil {
		s.notFound(w, r)
		return
	}
	entries, err := s.readFolder(r.Context(), c, folder, onlyCuts(q))
	switch {
	case file.NotFound(err):
		s.notFound(w, r)
		return
	case err != nil:
		retry := c.href("", segs, true)
		notices = append(notices, view.Notice{
			Role:  view.NoticeAlert,
			Title: text(collectionLabel(c.col) + " is unavailable"),
			Body:  text(fileReason(err) + " Nothing on this screen comes from the server."),
			Details: &view.Diagnostic{
				Summary: text("PROPFIND " + folder),
				Lines:   []view.Text{text(connLabel(c.conn) + folder), text(err.Error())},
			},
			Actions: []view.Action{{ID: "source.retry", Label: "Try again", Href: s.route(retry), Method: "GET", Enabled: true}},
		})
	}
	if pick != "" {
		i := slices.IndexFunc(entries, func(e file.Entry) bool { return e.Name == pick })
		switch {
		case err == nil && i < 0:
			s.notFound(w, r)
			return
		case i >= 0 && entries[i].Dir:
			s.redirect(w, r, c.href("", append(slices.Clone(segs), pick), true))
			return
		}
		q = cloneValues(q)
		q.Add("select", pick)
	}
	page := s.folderPage(c, segs, entries, q, notices)
	if target != "" && WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, target, page, nil)
		return
	}
	s.writePage(w, r, http.StatusOK, page)
}

func cloneValues(q url.Values) url.Values {
	out := url.Values{}
	for k, v := range q {
		out[k] = slices.Clone(v)
	}
	return out
}

// folderPage lays a listing out: folders first, then by the chosen order; the
// filter keeps names that hold it; selected rows are the names ?select= gives.
func (s *Server) folderPage(c *fileCtx, segs []string, entries []file.Entry, q url.Values, notices []view.Notice) view.FilesPage {
	now := s.now()
	title := collectionLabel(c.col)
	if len(segs) > 0 {
		title = segs[len(segs)-1]
	}
	page := view.FilesPage{Shell: s.buildShell(c.sess, title), View: view.FileList}
	here := c.href("", segs, true)
	page.Rail = s.filesRail(c.sess, c.blob, c.at(segs))
	page.Notices = append(page.Notices, notices...)
	page.Top.Refresh = view.Refresh{Action: view.Action{ID: "folder.refresh", Label: "Refresh", Icon: "i-refresh", Href: s.route(here), Method: "GET", Enabled: true}}

	page.Crumbs = []view.Crumb{{Label: text(collectionLabel(c.col)), Href: s.route(c.href("", nil, true))}}
	for i, seg := range segs {
		page.Crumbs = append(page.Crumbs, view.Crumb{Label: text(seg), Href: s.route(c.href("", segs[:i+1], true))})
	}
	page.Crumbs[len(page.Crumbs)-1].Href = ""

	if q.Get("view") == string(view.FileTiles) {
		page.View = view.FileTiles
	}
	order := q.Get("sort")
	if order != "size" && order != "changed" {
		order = "name"
	}
	filter := strings.TrimSpace(q.Get("filter"))
	needle := strings.ToLower(filter)
	selected := map[string]bool{}
	for _, name := range q["select"] {
		selected[name] = true
	}

	sorted := slices.Clone(entries)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if a.Dir != b.Dir {
			return a.Dir
		}
		switch {
		case order == "size" && a.Size != b.Size:
			return a.Size > b.Size
		case order == "changed" && !a.Modified.Equal(b.Modified):
			return a.Modified.After(b.Modified)
		}
		if x, y := strings.ToLower(a.Name), strings.ToLower(b.Name); x != y {
			return x < y
		}
		return a.Name < b.Name
	})
	if n := len(sorted); n > s.cfg.FilesMaxEntries {
		sorted = sorted[:s.cfg.FilesMaxEntries]
		page.Notices = append(page.Notices, view.Notice{
			Role:  view.NoticeInfo,
			Title: text(fmt.Sprintf("Showing %d of %d", s.cfg.FilesMaxEntries, n)),
			Body:  "This folder holds more than a listing shows (CARREL_FILES_MAX_ENTRIES). Filter it by name, or open it in another client.",
		})
	}

	var picked []file.Entry
	for _, e := range sorted {
		if needle != "" && !strings.Contains(strings.ToLower(e.Name), needle) {
			continue
		}
		rowSegs := append(slices.Clone(segs), e.Name)
		row := view.FileRow{
			Name:     text(e.Name),
			IsDir:    e.Dir,
			Modified: c.tf.formatWhen(e.Modified, false, now),
			Selected: selected[e.Name],
			Menu:     s.fileMenu(c, segs, e),
		}
		if e.Dir {
			row.Href, row.Type = s.route(c.href("", rowSegs, true)), "folder"
		} else {
			mt := fileType(e.Name, e.Type)
			row.Href, row.Type, row.Size = s.route(c.href("download", rowSegs, false)), text(mt), formatSize(e.Size)
			if inlineTypes[mt] && e.Size > 0 && e.Size <= conf.FileThumbMaxBytes {
				row.Thumb = s.route(c.href("download", rowSegs, false) + "?inline=1")
			}
		}
		if row.Selected {
			picked = append(picked, e)
		}
		page.Rows = append(page.Rows, row)
	}

	total := len(entries)
	page.Bar = view.ListBar{Filter: text(filter), Loaded: len(page.Rows), Total: &total, Selected: len(picked)}
	option := func(id, label, key, value, current string) view.Action {
		v := cloneValues(q)
		v.Set(key, value)
		return view.Action{ID: id, Label: text(label), Href: s.route(here + "?" + v.Encode()), Method: "GET", Enabled: true, On: value == current}
	}
	page.Bar.Segments = []view.Action{
		option("files.view.list", "List", "view", string(view.FileList), string(page.View)),
		option("files.view.tiles", "Tiles", "view", string(view.FileTiles), string(page.View)),
		option("files.sort.name", "Name", "sort", "name", order),
		option("files.sort.size", "Size", "sort", "size", order),
		option("files.sort.changed", "Changed", "sort", "changed", order),
	}
	page.Bar.Actions = s.folderActions(c, segs, picked)
	page.Queue = s.transferQueue(c.sess)
	page.Print = view.Print{Title: page.Title, Range: printRange(string(c.stamp()), plural(total, "item"), filter), Taken: c.tf.taken(now)}
	return page
}

// opHref opens the sheet of an operation over names of one folder.
func (s *Server) opHref(c *fileCtx, segs []string, op view.FileOp, names ...string) string {
	return s.postQuery(c.href("op", segs, true), url.Values{"op": {string(op)}, "item": names})
}

// writeAction is an operation that changes the storage: shown on a read-only
// storage too, and disabled there with the reason (U-6.5: visible and not
// available).
func writeAction(c *fileCtx, a view.Action) view.Action {
	if c.col.ReadOnly {
		a.Enabled, a.Reason = false, readOnlyReason
	}
	return a
}

// fileMenu is the menu of one row (s-files-9, s-files-10): open, download
// for a file, then rename, move, copy, publish and, behind a line, delete.
// Opening and downloading are two intents (components-5): Open is the file's
// address, its folder with it selected. Copying out of a read-only storage
// is allowed. A folder as an archive is not offered yet (docs/plan/server.md).
func (s *Server) fileMenu(c *fileCtx, segs []string, e file.Entry) []view.MenuItem {
	at := append(slices.Clone(segs), e.Name)
	open := view.Action{ID: "file.open", Label: "Open", Icon: "i-doc", Href: s.route(c.href("", at, false)), Method: "GET", Enabled: true}
	if e.Dir {
		open.Icon, open.Href = "i-file", s.route(c.href("", at, true))
	}
	menu := []view.MenuItem{{Action: open}}
	if !e.Dir {
		menu = append(menu, view.MenuItem{Action: view.Action{
			ID: "file.download", Label: "Download", Icon: "i-down", Method: "GET", Enabled: true,
			Href: s.route(c.href("download", at, false)),
		}})
	}
	menu = append(menu,
		view.MenuItem{Action: writeAction(c, view.Action{ID: "file.rename", Label: "Rename", Icon: "i-pencil", Href: s.opHref(c, segs, view.FileRename, e.Name), Method: "POST", Enabled: true})},
		view.MenuItem{Action: writeAction(c, view.Action{ID: "file.move", Label: "Move", Icon: "i-move", Href: s.opHref(c, segs, view.FileMove, e.Name), Method: "POST", Enabled: true})},
		view.MenuItem{Action: view.Action{ID: "file.copy", Label: "Copy", Icon: "i-dup", Href: s.opHref(c, segs, view.FileCopy, e.Name), Method: "POST", Enabled: true}},
		view.MenuItem{Action: s.filePublishAction(c, at, e.Dir)},
		view.MenuItem{Separator: true, Action: writeAction(c, view.Action{ID: "file.delete", Label: "Delete", Icon: "i-trash", Href: s.opHref(c, segs, view.FileDelete, e.Name), Method: "POST", Danger: true, Enabled: true})},
	)
	return menu
}

// folderActions are the bar's operations: Upload and New folder, and over a
// selection Download (one file), Rename (one), Move, Copy and Delete. The bar
// publishes nothing (s-files-1): a folder is published from its row in the
// folder above.
func (s *Server) folderActions(c *fileCtx, segs []string, picked []file.Entry) []view.Action {
	// Upload has an arrow (s-files-8): Files, which the button is, and
	// Folder — the same POST, a folder arriving as files with their paths;
	// only the dialog of the system differs, and that is the client's.
	upload := writeAction(c, view.Action{ID: "file.upload", Label: "Upload", Icon: "i-up", Href: s.route(c.href("upload", segs, true)), Method: "POST", Enabled: true})
	files, folder := upload, upload
	files.Label, files.Icon = "Files", "i-doc"
	folder.ID, folder.Label, folder.Icon = "file.upload.folder", "Folder", "i-file"
	upload.Arrow = []view.MenuItem{{Action: files}, {Action: folder}}
	actions := []view.Action{
		upload,
		writeAction(c, view.Action{ID: "folder.new", Label: "New folder", Icon: "i-plus", Href: s.route(c.href("mkcol", segs, true)), Method: "POST", Enabled: true}),
	}
	if len(picked) == 0 {
		return actions
	}
	names := make([]string, len(picked))
	for i, e := range picked {
		names[i] = e.Name
	}
	download := view.Action{ID: "file.download", Label: "Download", Icon: "i-down", Method: "GET", Enabled: len(picked) == 1 && !picked[0].Dir}
	if download.Enabled {
		download.Href = s.route(c.href("download", append(slices.Clone(segs), names[0]), false))
	} else {
		download.Reason = "Download takes one file."
	}
	rename := writeAction(c, view.Action{ID: "file.rename", Label: "Rename", Icon: "i-pencil", Href: s.opHref(c, segs, view.FileRename, names[0]), Method: "POST", Enabled: len(picked) == 1})
	if len(picked) != 1 {
		rename.Href, rename.Reason = "", "Rename takes one item."
	}
	return append(actions, download, rename,
		writeAction(c, view.Action{ID: "file.move", Label: "Move", Icon: "i-move", Href: s.opHref(c, segs, view.FileMove, names...), Method: "POST", Enabled: true}),
		view.Action{ID: "file.copy", Label: "Copy", Icon: "i-dup", Href: s.opHref(c, segs, view.FileCopy, names...), Method: "POST", Enabled: true},
		writeAction(c, view.Action{ID: "file.delete", Label: "Delete", Icon: "i-trash", Href: s.opHref(c, segs, view.FileDelete, names...), Method: "POST", Danger: true, Enabled: true}),
	)
}

// Download (C-5.5): the file as it comes, with the server's length, version
// and range. It is always an attachment in a sandbox, whatever its type: a
// page served from Carrel's origin must not run what someone uploaded. Only
// ?inline=1 of one of the four raster types is shown in place, for a
// thumbnail.

func (s *Server) getFilesDownload(w http.ResponseWriter, r *http.Request) {
	c, ok := s.openFiles(w, r, "download")
	if !ok {
		return
	}
	if c.dir {
		// A folder address after download/ is the folder "download" at the root
		// of the collection: C-5.5 gives that segment to this route.
		c.segs = append([]string{"download"}, c.segs...)
		s.showAddress(w, r, c)
		return
	}
	p, err := c.path(c.segs, false)
	if err != nil {
		s.notFound(w, r)
		return
	}
	name := c.segs[len(c.segs)-1]
	inline := r.URL.Query().Get("inline") != ""
	switch err := s.sendFile(w, r, c, p, name, inline); {
	case err == nil:
	case file.NotFound(err) || inline:
		s.notFound(w, r)
	default:
		s.writeFolder(w, r, c, c.segs[:len(c.segs)-1], "", url.Values{}, "", s.fileFailure(c, name+" could not be downloaded", "GET", p, err))
	}
}

// downloadHeaders are the headers of a file handed to the browser: its type,
// its name, and the sandbox that keeps it from running on Carrel's origin.
// Only inline of one of the raster types is shown in place.
func downloadHeaders(h http.Header, name, sentType string, inline bool) {
	mt := fileType(name, sentType)
	disposition := "attachment"
	if inline && inlineTypes[mt] {
		disposition = "inline"
	}
	if mt == "" {
		mt = "application/octet-stream"
	}
	h.Set("Content-Type", mt)
	if v := mime.FormatMediaType(disposition, map[string]string{"filename": name}); v != "" {
		h.Set("Content-Disposition", v)
	} else {
		h.Set("Content-Disposition", disposition)
	}
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	h.Set("Cache-Control", "private, no-cache")
}

// sendFile streams the file at p as it comes. An error is the server's refusal
// to open it, before anything was written to w.
func (s *Server) sendFile(w http.ResponseWriter, r *http.Request, c *fileCtx, p, name string, inline bool) error {
	d, err := c.files.Open(r.Context(), p, requestRange(r))
	if err != nil {
		return err
	}
	defer d.Body.Close()
	h := w.Header()
	downloadHeaders(h, name, d.Type, inline)
	h.Set("Accept-Ranges", "bytes")
	if d.ETag != "" {
		h.Set("ETag", `"`+d.ETag+`"`)
	}
	if d.Modified != "" {
		h.Set("Last-Modified", d.Modified)
	}
	if d.Length >= 0 {
		h.Set("Content-Length", strconv.FormatInt(d.Length, 10))
	}
	status := http.StatusOK
	if d.Status == http.StatusPartialContent && d.Range != "" {
		status = http.StatusPartialContent
		h.Set("Content-Range", d.Range)
	}
	w.WriteHeader(status)
	_, _ = io.Copy(w, d.Body)
	return nil
}

// requestRange is one byte range the browser asked for, passed on to the
// server. Several ranges or a suffix range are not passed: the whole file is a
// valid answer to them (RFC 9110 §14.2).
func requestRange(r *http.Request) *dav.Range {
	v, ok := strings.CutPrefix(r.Header.Get("Range"), "bytes=")
	if !ok || strings.Contains(v, ",") {
		return nil
	}
	a, b, _ := strings.Cut(strings.TrimSpace(v), "-")
	start, err := strconv.ParseInt(a, 10, 64)
	if err != nil || start < 0 {
		return nil
	}
	if b == "" {
		return &dav.Range{Start: start, End: -1}
	}
	end, err := strconv.ParseInt(b, 10, 64)
	if err != nil || end < start {
		return nil
	}
	return &dav.Range{Start: start, End: end}
}
