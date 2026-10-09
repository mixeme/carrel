package web

import (
	"archive/zip"
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
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/provider/file"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/render"
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
	// When the folder was read is what its refresh says, and the root's —
	// the newest of its storages (S-22: kept from reads that happen anyway).
	at := s.now()
	c.sess.NoteReading(c.conn.ID, folder, session.Reading{At: at})
	c.sess.NoteReading(c.conn.ID, c.col.Path, session.Reading{At: at})
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
	return q.Has("filter") || q.Has("sort") || q.Has("view") || q.Has("select") || q.Has("details") || q.Has("print") || q.Has("queue")
}

// Markdown is not in Go's own table of types, and what a system's table says
// differs from one machine to the next: a folder of notes would read
// «text/markdown» on one and nothing on another (s-files-1 says it).
func init() {
	_ = mime.AddExtensionType(".md", "text/markdown; charset=utf-8")
}

// Markdown is not in Go's own table of types, and what a system's table says
// differs from one machine to the next: a folder of notes would read
// «text/markdown» on one and nothing on another (s-files-1 says it).
func init() {
	_ = mime.AddExtensionType(".md", "text/markdown; charset=utf-8")
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
	// The root asks no server: its refresh reads it again, and says when a
	// folder of a ticked storage was read last (s-files-6).
	page.Top.Refresh = view.Refresh{Action: view.Action{ID: "files.refresh", Label: "Refresh", Icon: "i-refresh", Href: s.route("/app/files"), Method: "GET", Enabled: true}}
	var last time.Time
	for _, col := range ticked(blob, view.KindFile) {
		if r, ok := sess.ReadingOf(col.AccountID, col.Path); ok && r.At.After(last) {
			last = r.At
		}
	}
	page.Top.Refresh.Answered = s.timeFor(sess).formatWhen(last, true, s.now())
	live, servers := 0, map[string]bool{}
	var off []string
	for _, col := range blob.Collections {
		if _, ok := blob.Connection(col.AccountID); !ok || col.Gone || !col.Has(view.KindFile) {
			continue
		}
		live++
		servers[col.AccountID] = true
		if !isTicked(blob, view.KindFile, col.AccountID, col.Path) {
			off = append(off, collectionLabel(col))
		}
	}
	page.Subtitle = text(fmt.Sprintf("%d of %s on %s", live-len(off), plural(live, "storage"), plural(len(servers), "server")))
	if live == len(off) {
		// Nothing ticked, the servers are not counted (states-35).
		page.Subtitle = text(fmt.Sprintf("0 of %s", plural(live, "storage")))
	}
	for _, col := range ticked(blob, view.KindFile) {
		conn, _ := blob.Connection(col.AccountID)
		row := view.FileCollectionRow{
			Label:    text(collectionLabel(col)),
			Stamp:    stamp(connLabel(conn), collectionLabel(col)),
			Server:   text(connLabel(conn)),
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
	} else {
		// What the root is, and which storages it leaves out (s-files-6).
		page.Note = line("This is where the section starts: the storages you ticked in the rail, as top-level folders. " +
			"Open one and its own tree opens — contents of different servers are never mixed in one folder, because a file has one path and belongs to its own storage.")
		if len(off) > 0 {
			parts := []any{" "}
			for i, name := range off {
				if i > 0 {
					parts = append(parts, ", ")
				}
				parts = append(parts, strongRun(name))
			}
			verb := " is not ticked, so it is not here."
			if len(off) > 1 {
				verb = " are not ticked, so they are not here."
			}
			page.Note = append(page.Note, join(append(parts, verb)...)...)
		}
	}
	s.writePage(w, r, http.StatusOK, page)
}

// filesEmpty says why the root is empty and what to do (S-17.2): the section
// never disappears.
func filesEmpty(s *Server, blob sealed.Blob) *view.Notice {
	for _, col := range blob.Collections {
		if _, ok := blob.Connection(col.AccountID); ok && !col.Gone && col.Has(view.KindFile) {
			// What to do is a button (U-6.0): the root row's tick, all of them.
			return &view.Notice{
				Role: view.NoticeInfo,
				// states-35.
				Title: "No storages are ticked",
				Body:  "Tick the storages to show in the rail, or all of them at once.",
				Actions: []view.Action{{ID: "rail.tick.all", Label: "Tick every storage", Method: "POST", Enabled: true, Primary: true,
					Href: s.postQuery("/app/sources/files", url.Values{"toggle": {s.route("/app/files")}, "back": {"/app/files"}})}},
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
	rail := view.Rail{About: s.aboutAction(), Davloom: s.davloomMark(sess), Nav: s.navRows(sess, blob, filesSpec.segment)}
	sections, live, on := s.railSections(sess, blob, filesSpec, nil, nil)
	root := view.SourceRow{Href: s.route("/app/files"), Label: "Selected collections", Tick: view.TickPart, State: view.CollectionOK}
	switch on {
	case 0:
		root.Tick = view.TickOff
	case live:
		root.Tick = view.TickOn
	}
	root.Meta = text(fmt.Sprintf("%d of %d", on, live))
	rail.Sections = append([]view.RailSection{{State: view.CollectionOK, Rows: []view.SourceRow{root}}}, sections...)
	// The row of what is on the screen is open (s-files-6: the root; s-files-1:
	// the storage of the folder).
	if open == nil {
		markOpen(rail.Sections, s.route("/app/files"))
	} else {
		markOpen(rail.Sections, s.route(fileHref(open.conn.ID, open.col.Path, "", nil, true)))
	}
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
	back := "/app/files"
	if open != nil {
		back = open.href("", open.segs, true)
		openHref = s.route(back)
	}
	rail.Ticks = s.ticksForm(sess, "files", back)
	rail.Foot = []view.Action{
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
// their count. Without an attachment folder the row stays, says «not set»
// and leads to where it is chosen (s-files-18).
func (s *Server) filesShortcuts(sess *session.Session, blob sealed.Blob, open string) []view.Shortcut {
	attach := view.Shortcut{Action: view.Action{ID: "files.attachments", Label: "Attachments", Icon: "i-file", Href: s.route("/app/settings/attachments"), Method: "GET", Enabled: true}, Place: "not set"}
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
	s.writeFolderWith(w, r, c, segs, pick, q, target, nil, notices...)
}

// writeFolderWith is writeFolder with edit applied to the page before it
// goes: the line of a new folder's name (postFilesMkcol).
func (s *Server) writeFolderWith(w http.ResponseWriter, r *http.Request, c *fileCtx, segs []string, pick string, q url.Values, target string, edit func(*view.FilesPage), notices ...view.Notice) {
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
			Body:  text("The server did not answer as expected, so nothing on this screen comes from it."),
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
	page := s.folderPage(r, c, segs, entries, q, notices)
	if err != nil {
		// Nothing was read (states-33): the head and the message alone.
		page.Unread, page.Count, page.Empty, page.Queue = true, "", nil, nil
	}
	if edit != nil {
		edit(&page)
	}
	if target != "" && render.WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, target, page, nil)
		return
	}
	s.writePage(w, r, http.StatusOK, page)
}

// querySuffix is "?" and the query, or nothing for an empty one.
func querySuffix(q url.Values) string {
	if enc := q.Encode(); enc != "" {
		return "?" + enc
	}
	return ""
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
func (s *Server) folderPage(r *http.Request, c *fileCtx, segs []string, entries []file.Entry, q url.Values, notices []view.Notice) view.FilesPage {
	now := s.now()
	// A folder put back in its default order because its column went says
	// so (note 25); the mark leaves the query the page's links are made of.
	q = cloneValues(q)
	if n := unsortedNoticeOf(q, "files"); n != nil {
		notices = append(notices, *n)
	}
	title := collectionLabel(c.col)
	if len(segs) > 0 {
		title = segs[len(segs)-1]
	}
	page := view.FilesPage{Shell: s.buildShell(c.sess, title), View: view.FileList}
	here := c.href("", segs, true)
	page.Rail = s.filesRail(c.sess, c.blob, c.at(segs))
	page.Notices = append(page.Notices, notices...)
	page.Top.Refresh = view.Refresh{Action: view.Action{ID: "folder.refresh", Label: "Refresh", Icon: "i-refresh", Href: s.route(here), Method: "GET", Enabled: true}}
	if folder, err := c.path(segs, true); err == nil {
		if r, ok := c.sess.ReadingOf(c.conn.ID, folder); ok {
			page.Top.Refresh.Answered = c.tf.formatWhen(r.At, true, now)
		}
	}

	// The line under the title: whose storage this is and, on a read-only
	// one, the one reason Upload and New folder stand off (s-files-15).
	// After it the folder counts what it holds (s-files-1), and a read-only
	// storage says so last (states-34).
	page.Subtitle, page.Count = text(connLabel(c.conn)), text(plural(len(entries), "item"))
	if c.col.ReadOnly {
		page.Access = "read-only: nothing can be uploaded or made here"
	}
	// «attachment folder» by the line (s-files-1): this folder is the one
	// attachments are written to (S-20).
	if fc := s.savedFolder(c.sess, c.blob); fc != nil && fc.href("", fc.segs, true) == here {
		page.Attachments = true
	}
	page.Crumbs = []view.Crumb{{Label: text(collectionLabel(c.col)), Href: s.route(c.href("", nil, true))}}
	for i, seg := range segs {
		page.Crumbs = append(page.Crumbs, view.Crumb{Label: text(seg), Href: s.route(c.href("", segs[:i+1], true))})
	}
	page.Crumbs[len(page.Crumbs)-1].Href = ""

	if q.Get("view") == string(view.FileTiles) {
		page.View = view.FileTiles
	}
	order := listSorts[view.KindFile][0].value
	if sortColumn("files", q.Get("sort")) != "" {
		order = q.Get("sort")
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
	chosen := -1 // the row of the one file chosen
	for _, e := range sorted {
		if needle != "" && !strings.Contains(strings.ToLower(e.Name), needle) {
			continue
		}
		rowSegs := append(slices.Clone(segs), e.Name)
		row := view.FileRow{
			Name:     text(e.Name),
			IsDir:    e.Dir,
			Modified: c.tf.numStamp(e.Modified),
			Selected: selected[e.Name],
			Menu:     s.fileMenu(c, segs, e),
		}
		row.Icon, row.Note = fileIcon(e), thumbNote(e)
		if e.Dir {
			row.Href, row.Type = s.route(c.href("", rowSegs, true)), "folder"
			row.Open = row.Href
			row.Items = s.folderItems(c, rowSegs, e)
			// A drop on the row goes into it (U-15.3, s-files-1).
			into := writeAction(c, view.Action{ID: "folder.upload", Label: text("Upload into " + e.Name), Icon: "i-up",
				Href: s.route(c.href("upload", rowSegs, true)), Method: "POST", Enabled: true, File: uploadField})
			row.Upload = &into
		} else {
			mt := fileType(e.Name, e.Type)
			row.Href, row.Type, row.Size = s.route(c.href("download", rowSegs, false)), text(mt), formatSize(e.Size)
			row.Open = s.route(c.href("", rowSegs, false))
			if inlineTypes[mt] && e.Size > 0 && e.Size <= conf.FileThumbMaxBytes {
				row.Thumb = s.route(c.href("download", rowSegs, false) + "?inline=1")
			}
		}
		if row.Selected {
			picked = append(picked, e)
			chosen = len(page.Rows)
		}
		page.Rows = append(page.Rows, row)
	}

	total := len(entries)
	page.Bar = view.ListBar{Filter: text(filter), Loaded: len(page.Rows), Total: &total, Selected: len(picked),
		Columns: s.columnsMenu(r, csrfField(c.sess), "files", c.href("columns", segs, true), q, false),
		Size:    sizeOf(entries)}
	if len(picked) > 0 {
		page.Bar.SelectedSize = sizeOf(picked)
	}
	// «Filter this folder» without scripts: a GET of the folder that keeps
	// its view and order (C-6.4; with scripts the client sifts the rows).
	var keep []view.FormRow
	for _, name := range []string{"view", "sort"} {
		if v := q.Get(name); v != "" {
			keep = append(keep, view.FormRow{Name: name, Kind: view.InputHidden, Value: text(v)})
		}
	}
	filterForm := s.plainForm(here, "GET", append(keep, view.FormRow{Name: "filter", Label: "Filter this folder", Kind: view.InputText, Value: text(filter)})...)
	page.Bar.FilterForm = &filterForm
	option := func(id, label, key, value, current string) view.Action {
		v := cloneValues(q)
		v.Set(key, value)
		return view.Action{ID: id, Label: text(label), Href: s.route(here + "?" + v.Encode()), Method: "GET", Enabled: true, On: value == current}
	}
	list := option("files.view.list", "List", "view", string(view.FileList), string(page.View))
	tiles := option("files.view.tiles", "Tiles", "view", string(view.FileTiles), string(page.View))
	list.Icon, tiles.Icon = "i-menu", "i-grid" // s-files-1, -5
	page.Bar.Segments = []view.Action{list, tiles}
	for _, o := range listSorts[view.KindFile] {
		page.Bar.Segments = append(page.Bar.Segments, option(o.id, o.label, "sort", o.value, order))
	}
	page.Bar.Actions = s.folderActions(c, segs, picked, len(entries) > 0)
	// An empty folder says what to do with it, with the head's own buttons
	// (states-32); a read-only one has nothing to offer.
	if len(entries) == 0 {
		page.Empty = &view.Notice{Role: view.NoticeInfo, Title: "This folder is empty."}
		if !c.col.ReadOnly {
			page.Empty.Body = "Upload files into it, or drop them onto the window."
			page.Empty.Actions = s.folderActions(c, segs, nil, false)
		}
	}
	// One file chosen opens its panel (s-files-1); Details shows and hides
	// it, Print is the «⋯» of the head (s-files-7).
	shown := len(picked) == 1 && !picked[0].Dir && q.Get("details") != "0"
	// One image chosen says its size (ФБ11): its row or tile and its panel,
	// from one ranged read of its head (S-17); the others say none.
	var dims view.Text
	if len(picked) == 1 && !picked[0].Dir {
		dims = s.imageSize(r.Context(), c, segs, picked[0])
		page.Rows[chosen].Dimensions = dims
	}
	if shown {
		page.Details = s.filePanel(c, segs, picked[0], here, q, dims)
	}
	page.Bar.Actions = append(page.Bar.Actions, s.detailsToggle(here, q, picked, shown), s.printAction(here))
	page.Select, page.SelectField = s.selectForm(c, segs, here, q), "item"
	// The queue hides into its head and shows again (s-files-2, -3): an
	// address of the folder either way.
	if page.Queue = s.transferQueue(c.sess); page.Queue != nil {
		v := cloneValues(q)
		toggle := view.Action{ID: "queue.hide", Label: "Hide", Method: "GET", Enabled: true}
		if page.Queue.Hidden = q.Get("queue") == "0"; page.Queue.Hidden {
			toggle.ID, toggle.Label = "queue.show", "Show"
			v.Del("queue")
		} else {
			v.Set("queue", "0")
		}
		toggle.Href = s.route(here + querySuffix(v))
		page.Queue.Toggle = &toggle
	}
	page.Print = view.Print{Title: page.Title, Range: printRange(string(c.stamp()), plural(total, "item"), filter), Taken: c.tf.taken(now)}
	return page
}

// fileIcon is the icon of a row by its type (s-files-1): a folder, an
// image, anything else.
func fileIcon(e file.Entry) string {
	switch {
	case e.Dir:
		return "i-file"
	case strings.HasPrefix(fileType(e.Name, e.Type), "image/"):
		return "i-img"
	}
	return "i-doc"
}

// thumbNote says why an image a browser could show has no thumbnail: it is
// larger than a thumbnail is allowed to be (s-files-5).
func thumbNote(e file.Entry) view.Text {
	if !e.Dir && inlineTypes[fileType(e.Name, e.Type)] && e.Size > conf.FileThumbMaxBytes {
		return "too large"
	}
	return ""
}

// folderItems is what the row of a folder says it holds (ФБ11: s-files-1
// «folder · 34 items», s-files-5 «12 items»): the count its server gave in
// the folder's one PROPFIND (docs/servers.md, SV-57), else the length of the
// listing of it this session holds (S-4.2) — whatever its age, as the rail's
// numbers are; empty otherwise. Never a request per row (S-17).
func (s *Server) folderItems(c *fileCtx, segs []string, e file.Entry) view.Text {
	if e.Items != nil {
		return text(plural(*e.Items, "item"))
	}
	folder, err := c.path(segs, true)
	if err != nil {
		return ""
	}
	if data, ok := c.sess.Cache().PeekETagMap(c.conn.ID, listKey(folder)); ok {
		var members []json.RawMessage
		if json.Unmarshal(data, &members) == nil {
			return text(plural(len(members), "item"))
		}
	}
	return ""
}

// imageSize is «4032×3024» of an image a browser shows (S-17, ФБ11): one
// GET of its head (file.Provider.Dimensions), kept by the file's version, so
// another look at the same version reads nothing. A file that is not such an
// image, a header that does not say within the cap, a server that did not
// answer — no size, and the page stands without it.
func (s *Server) imageSize(ctx context.Context, c *fileCtx, segs []string, e file.Entry) view.Text {
	if e.Dir || e.Size == 0 || !inlineTypes[fileType(e.Name, e.Type)] {
		return ""
	}
	p, err := c.path(append(slices.Clone(segs), e.Name), false)
	if err != nil {
		return ""
	}
	if e.ETag != "" {
		if data, ok := c.sess.Cache().GetImageSize(c.conn.ID, p, e.ETag); ok {
			return view.Text(data)
		}
	}
	w, h, ok, err := c.files.Dimensions(ctx, p)
	if err != nil {
		return ""
	}
	var dims view.Text
	if ok {
		dims = view.Text(fmt.Sprintf("%d×%d", w, h))
	}
	if e.ETag != "" {
		c.sess.Cache().PutImageSize(c.conn.ID, p, e.ETag, []byte(dims))
	}
	return dims
}

// sizeOf is what the files among entries weigh together; empty without a
// file.
func sizeOf(entries []file.Entry) view.Size {
	var n int64
	files := 0
	for _, e := range entries {
		if !e.Dir && e.Size > 0 {
			n += e.Size
			files++
		}
	}
	if files == 0 {
		return ""
	}
	return formatSize(n)
}

// filePanel is the panel of the one file chosen (s-files-1): where it is,
// its preview, Download first, Open (an image, in place), Rename and Move;
// size, type, an image's width and height when its head said them, when it
// changed and its path on the server; Delete. Close is the folder with
// nothing chosen.
func (s *Server) filePanel(c *fileCtx, segs []string, e file.Entry, here string, q url.Values, dims view.Text) *view.Details {
	at := append(slices.Clone(segs), e.Name)
	mt := fileType(e.Name, e.Type)
	download := s.route(c.href("download", at, false))
	d := &view.Details{Title: text(e.Name), Subtitle: text(string(c.stamp())), Preview: &view.Thumb{Icon: fileIcon(e)}}
	if inlineTypes[mt] && e.Size > 0 && e.Size <= conf.FileThumbMaxBytes {
		d.Preview.Href = download + "?inline=1"
	}
	open := view.Action{ID: "file.view", Label: "Open", Method: "GET"}
	if inlineTypes[mt] {
		open.Enabled, open.Href = true, download+"?inline=1"
	} else {
		open.Reason = "Carrel shows only an image in place: download the file to open it."
	}
	d.Actions = []view.Action{
		{ID: "file.download", Label: "Download", Icon: "i-down", Href: download, Method: "GET", Enabled: true, Primary: true},
		open,
		writeAction(c, view.Action{ID: "file.rename", Label: "Rename", Icon: "i-pencil", Href: s.opHref(c, segs, view.FileRename, e.Name), Method: "POST", Enabled: true}),
		writeAction(c, view.Action{ID: "file.move", Label: "Move", Icon: "i-move", Href: s.opHref(c, segs, view.FileMove, e.Name), Method: "POST", Enabled: true}),
		writeAction(c, view.Action{ID: "file.delete", Label: "Delete", Icon: "i-trash", Href: s.opHref(c, segs, view.FileDelete, e.Name), Method: "POST", Danger: true, Enabled: true}),
	}
	path, _ := c.path(at, false)
	d.Fields = []view.Field{
		{Label: "Size", Value: text(string(formatSize(e.Size))), Mono: true},
		{Label: "Type", Value: text(mt), Mono: true},
	}
	if dims != "" {
		d.Fields = append(d.Fields, view.Field{Label: "Dimensions", Value: dims, Mono: true})
	}
	d.Fields = append(d.Fields,
		view.Field{Label: "Changed", Value: text(string(c.tf.formatWhen(e.Modified, false, s.now()))), Mono: true},
		view.Field{Label: "Path", Value: text(path), Mono: true},
	)
	closed := cloneValues(q)
	closed.Del("select")
	closed.Del("details")
	d.Close = s.route(here + querySuffix(closed))
	return d
}

// detailsToggle is Details of the head (s-files-1): it hides the panel of
// the file chosen (details=0) and shows it again; with no one file chosen
// there is no panel to show.
func (s *Server) detailsToggle(here string, q url.Values, picked []file.Entry, shown bool) view.Action {
	a := view.Action{ID: "files.details", Label: "Details", Icon: "i-panel", Method: "GET", On: shown}
	if len(picked) != 1 || picked[0].Dir {
		a.Reason = "Choose one file to see its details."
		return a
	}
	v := cloneValues(q)
	if shown {
		v.Set("details", "0")
	} else {
		v.Del("details")
	}
	a.Enabled, a.Href = true, s.route(here+querySuffix(v))
	return a
}

// selectForm is the selection of a folder (s-files-5): the form of its
// operations — a POST of the folder's op with the token —, whose ticks are
// the fields «item» and whose buttons are the operations of the bar. Clear
// selection is the folder with its query and nothing selected by the
// address.
func (s *Server) selectForm(c *fileCtx, segs []string, here string, q url.Values) *view.Form {
	f := view.Form{Action: s.route(c.href("op", segs, true)), Method: "POST", Target: render.TargetSheet,
		Sets: []view.FieldSet{{Rows: []view.FormRow{csrfField(c.sess)}}}}
	if len(q["select"]) > 0 {
		clear := cloneValues(q)
		clear.Del("select")
		clear.Del("details")
		f.Actions = []view.Action{{ID: "files.select.clear", Label: "Clear selection", Href: s.route(here + querySuffix(clear)), Method: "GET", Enabled: true}}
	}
	return &f
}

// opHref opens the sheet of an operation over names of one folder.
func (s *Server) opHref(c *fileCtx, segs []string, op view.FileOp, names ...string) string {
	return s.postQuery(c.href("op", segs, true), url.Values{"op": {string(op)}, "item": names})
}

// uploadField is the file field of an upload (C-5.5): the route reads every
// file part whatever its name, and the actions name this one.
const uploadField = "file"

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
// — a folder «as .zip» — then rename, move, copy, publish and, behind a line, delete.
// Opening and downloading are two intents (components-5): Open is the file's
// address, its folder with it selected. A folder downloads as a zip.
// Downloading and copying out of a read-only storage are allowed.
func (s *Server) fileMenu(c *fileCtx, segs []string, e file.Entry) []view.MenuItem {
	at := append(slices.Clone(segs), e.Name)
	open := view.Action{ID: "file.open", Label: "Open", Icon: "i-doc", Href: s.route(c.href("", at, false)), Method: "GET", Enabled: true}
	if e.Dir {
		open.Icon, open.Href = "i-file", s.route(c.href("", at, true))
	}
	download := view.Action{ID: "file.download", Label: "Download", Icon: "i-down", Method: "GET", Enabled: true, Href: s.route(c.href("download", at, false))}
	if e.Dir {
		download.ID, download.Label = "file.download.zip", "Download as .zip"
		download.Href = s.postQuery(c.href("download", at, true), url.Values{"zip": {"1"}})
	}
	menu := []view.MenuItem{{Action: open}, {Action: download}}
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
func (s *Server) folderActions(c *fileCtx, segs []string, picked []file.Entry, bulk bool) []view.Action {
	// Upload has an arrow (s-files-8): Files, which the button is, and
	// Folder — the same POST, a folder arriving as files with their paths;
	// only the dialog of the system differs, and that is the client's.
	// Each carries the file field it sends (Action.File): without scripts
	// the client sends it as a form of its own with that field (U-8).
	upload := writeAction(c, view.Action{ID: "file.upload", Label: "Upload", Icon: "i-up", Href: s.route(c.href("upload", segs, true)), Method: "POST", Enabled: true, File: uploadField})
	files, folder := upload, upload
	files.Label, files.Icon = "Files", "i-doc"
	folder.ID, folder.Label, folder.Icon = "file.upload.folder", "Folder", "i-file"
	upload.Arrow = []view.MenuItem{{Action: files}, {Action: folder}}
	// The bar's primary button, read-only folder too (s-files-1, -15).
	upload.Primary = true
	actions := []view.Action{
		upload,
		writeAction(c, view.Action{ID: "folder.new", Label: "New folder", Icon: "i-plus", Href: s.route(c.href("mkcol", segs, true)), Method: "POST", Enabled: true}),
	}
	if !bulk {
		return actions
	}
	// The operations of the selection bar are buttons of the selection form
	// (FilesPage.Select): what is ticked goes with the one pressed, so they
	// name no item themselves and stand with nothing selected too — without
	// scripts the bar is always there (the owner's decision of 09.10.2026).
	// Download is a link, and has one only for the one file the address
	// selects.
	download := view.Action{ID: "file.download", Label: "Download", Icon: "i-down", Method: "GET", Enabled: len(picked) == 1 && !picked[0].Dir}
	if download.Enabled {
		download.Href = s.route(c.href("download", append(slices.Clone(segs), picked[0].Name), false))
	} else {
		download.Reason = "Download takes one file."
	}
	op := func(a view.Action, op view.FileOp) view.Action {
		a.Href, a.Method, a.Enabled, a.Submits = s.opHref(c, segs, op), "POST", true, true
		return a
	}
	// A rename takes one: more than one selected by the address, and it
	// stands off with why.
	rename := writeAction(c, op(view.Action{ID: "file.rename", Label: "Rename", Icon: "i-pencil"}, view.FileRename))
	if len(picked) > 1 {
		rename.Enabled, rename.Href, rename.Reason = false, "", "Rename takes one item."
	}
	return append(actions, download, rename,
		writeAction(c, op(view.Action{ID: "file.move", Label: "Move", Icon: "i-move"}, view.FileMove)),
		op(view.Action{ID: "file.copy", Label: "Copy", Icon: "i-dup"}, view.FileCopy),
		writeAction(c, op(view.Action{ID: "file.delete", Label: "Delete", Icon: "i-trash", Danger: true}, view.FileDelete)),
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
	if c.dir && r.URL.Query().Get("zip") != "" {
		s.downloadFolder(w, r, c)
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

// downloadFolder is «Download as .zip» of a folder's row (s-files-10): the
// folder, download/{path…}/?zip=1, as a zip written while it is read. It only
// reads, so a read-only storage gives it too. A folder that is not there is
// 404; a server that refuses to list it leaves the person on the folder
// above with the reason, like a file that could not be downloaded.
func (s *Server) downloadFolder(w http.ResponseWriter, r *http.Request, c *fileCtx) {
	p, err := c.path(c.segs, true)
	if err != nil {
		s.notFound(w, r)
		return
	}
	name := c.col.Name
	if len(c.segs) > 0 {
		name = c.segs[len(c.segs)-1]
	}
	switch err := s.sendZip(w, r, c, p, name); {
	case err == nil:
	case file.NotFound(err) || len(c.segs) == 0:
		s.notFound(w, r)
	default:
		s.writeFolder(w, r, c, c.segs[:len(c.segs)-1], "", url.Values{}, "", s.fileFailure(c, name+" could not be downloaded", "PROPFIND", p, err))
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

// sendZip streams the folder at p as a zip named name.zip, written while it
// is read: the files one at a time from the server into the response, the
// names relative to the folder (S-17.5). The folder is read first, and an
// error is the server's refusal to list it, before anything was written to
// w. At most CARREL_FILES_MAX_ENTRIES entries go in, like a folder's page; a
// break in the middle — the ceiling, a file that does not open — leaves a
// zip without its directory: a reader sees a broken archive, never one that
// silently lacks files. A private folder and a published one share it.
func (s *Server) sendZip(w http.ResponseWriter, r *http.Request, c *fileCtx, p, name string) error {
	ctx := r.Context()
	top, err := c.files.List(ctx, p)
	if err != nil {
		return err
	}
	h := w.Header()
	h.Set("Content-Type", "application/zip")
	if v := mime.FormatMediaType("attachment", map[string]string{"filename": name + ".zip"}); v != "" {
		h.Set("Content-Disposition", v)
	}
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	h.Set("Cache-Control", "private, no-cache")
	zw := zip.NewWriter(w)
	count := 0
	var walk func(entries []file.Entry, prefix string) bool
	walk = func(entries []file.Entry, prefix string) bool {
		for _, e := range entries {
			if count >= s.cfg.FilesMaxEntries {
				return false
			}
			count++
			if e.Dir {
				if _, err := zw.Create(prefix + e.Name + "/"); err != nil {
					return false
				}
				inner, err := c.files.List(ctx, e.Path)
				if err != nil || !walk(inner, prefix+e.Name+"/") {
					return false
				}
				continue
			}
			d, err := c.files.Open(ctx, e.Path, nil)
			if err != nil {
				return false
			}
			f, err := zw.CreateHeader(&zip.FileHeader{Name: prefix + e.Name, Method: zip.Deflate, Modified: e.Modified})
			if err == nil {
				_, err = io.Copy(f, d.Body)
			}
			d.Body.Close()
			if err != nil {
				return false
			}
		}
		return true
	}
	if walk(top, "") {
		_ = zw.Close()
	}
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
