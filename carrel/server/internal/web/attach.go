package web

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/file"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// Attachments (S-20). Up to AttachInlineMax — or the smaller max-resource-size
// of the collection — the file is written into the event, the task or the note
// as BINARY data, measured on the finished object before the PUT. A larger one
// goes to the attachment folder chosen once in settings, named after the date
// and title of the record and its own name, and the record keeps a link. An
// attachment another client wrote stays as it was stored, whatever its size.
// A file chosen in Files (From Files) follows the same rule, except that a
// large one is not copied: the record links to it where it lies.
// Attaching and removing are writes of their own on the edit address
// (?op=attach, ?op=attachfile, ?op=detach) with If-Match of the version the
// page showed, the way a contact photo is (S-13). A file is opened through
// /app/attach/: an inline one from its record, a link only into a file
// collection of the catalog; any other link is shown and never fetched.

// stampSlack is what a write adds to the object measured before it: DTSTAMP
// and LAST-MODIFIED of the write, a line each at most.
const stampSlack = 2 * len("LAST-MODIFIED:20060102T150405Z\r\n")

// attachNameTries is how many numbered names a large attachment tries in the
// folder before it gives up.
const attachNameTries = 100

// attachLimit is the largest object an attachment is written into.
func attachLimit(col sealed.Collection) int64 {
	if col.MaxResourceSize > 0 && col.MaxResourceSize < conf.AttachInlineMax {
		return col.MaxResourceSize
	}
	return conf.AttachInlineMax
}

// sizeText is a limit as a person reads it: whole megabytes without a
// fraction.
func sizeText(n int64) string {
	if n >= 1<<20 && n%(1<<20) == 0 {
		return fmt.Sprintf("%d MB", n>>20)
	}
	return string(formatSize(n))
}

// attachHref is the Carrel address of an attachment: a file under a file
// collection, or {uid}/{n} of an inline one under a calendar collection.
func attachHref(account, colPath string, segs []string) string {
	var b strings.Builder
	b.WriteString("/app/attach/" + account + "/" + url.PathEscape(colPath))
	for _, seg := range segs {
		b.WriteString("/" + url.PathEscape(seg))
	}
	return b.String()
}

// File names of large attachments, as the person chose them (S-20,
// attachments-3): "2026-08-10 report.pdf" by default, "report.pdf", or
// "Sync-up - report.pdf" — readable in the folder without Carrel. The title is
// cut to attachTitleRunes and attachTitleBytes, the whole name to
// attachNameBytes: many servers and file systems take no more than 255 bytes.
// The extension is never cut.
const (
	attachTitleRunes = 60
	attachTitleBytes = 120
	attachNameBytes  = 200
	attachExtBytes   = 16
)

func attachmentFileName(names string, day time.Time, title, original string) string {
	original = cleanFileName(original)
	ext := path.Ext(original)
	if ext == original || len(ext) > attachExtBytes || strings.Contains(ext, " ") {
		ext = ""
	}
	stem := strings.TrimSpace(strings.TrimSuffix(original, ext))
	if stem == "" {
		stem = "attachment"
	}
	var head string
	switch names {
	case sealed.AttachNamesOriginal:
	case sealed.AttachNamesTitled:
		// A record without a title gives the file's own name alone.
		if title = cutName(cleanFileName(title), attachTitleRunes, attachTitleBytes); title != "" {
			head = title + " - "
		}
	default:
		head = day.Format("2006-01-02") + " "
	}
	return head + cutName(stem, -1, attachNameBytes-len(head)-len(ext)) + ext
}

// cleanFileName takes out what a file system or a server refuses in a name,
// and runs of spaces.
func cleanFileName(s string) string {
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) || unicode.IsControl(r) {
			return '-'
		}
		return r
	}, s)
	return strings.Trim(strings.Join(strings.Fields(s), " "), " .")
}

// cutName keeps at most maxRunes letters (no limit when negative) and maxBytes
// bytes of s, on a letter boundary.
func cutName(s string, maxRunes, maxBytes int) string {
	end, count := 0, 0
	for i, r := range s {
		size := utf8.RuneLen(r)
		if size < 0 {
			size = 1
		}
		if (maxRunes >= 0 && count == maxRunes) || i+size > maxBytes {
			break
		}
		end, count = i+size, count+1
	}
	return strings.TrimRight(s[:end], " .")
}

// attachmentDay is the date a record's attachment is named by: the start of an
// event, the due date of a task (its start without one), the date of a note;
// today when the record has none.
func attachmentDay(c *collectionCtx, obj *model.Object) time.Time {
	tz := c.tf.tz
	if c.spec.kind == view.KindNote {
		if at := noteTime(obj, tz); !at.IsZero() {
			return at.In(tz)
		}
		return c.tf.now.In(tz)
	}
	if c.spec.kind == view.KindTask {
		if due, err := cal.DueOf(obj, tz); err == nil && !due.Start.IsZero() {
			return due.Start.In(tz)
		}
	}
	if tm, err := cal.TimingOf(obj, tz); err == nil && !tm.Start.IsZero() {
		return tm.Start.In(tz)
	}
	return c.tf.now.In(tz)
}

// attachmentTitle is the title in the name: SUMMARY, or the first line of a
// note's body.
func attachmentTitle(c *collectionCtx, obj *model.Object) string {
	summary := cal.Text(obj, "SUMMARY")
	if c.spec.kind == view.KindNote && strings.TrimSpace(summary) == "" {
		if t := noteTitle("", cal.Description(obj)); t != "Untitled note" {
			return t
		}
	}
	return summary
}

// uploadName is the name a browser sent, without the folders some put before
// it.
func uploadName(name string) string {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	return name
}

// sameVersion compares a version a page carried with the one the server has.
func sameVersion(shown, current string) bool {
	norm := func(v string) string { return strings.Trim(strings.TrimPrefix(strings.TrimSpace(v), "W/"), `"`) }
	return norm(shown) != "" && norm(shown) == norm(current)
}

// Rows and actions of the Attachments block.

func (s *Server) attachRows(c *collectionCtx, uid string, obj *model.Object, etag, from string) []view.AttachRow {
	var blob sealed.Blob
	loaded := false
	var folder *fileCtx
	var rows []view.AttachRow
	for _, a := range cal.Attachments(obj) {
		row := view.AttachRow{Inline: a.Inline, State: view.RecordOK}
		name := a.FileName
		if a.Inline {
			if name == "" {
				name = fmt.Sprintf("Attachment %d", a.Index+1)
			}
			row.Href = s.route(attachHref(c.conn.ID, c.col.Path, []string{uid, strconv.Itoa(a.Index)}))
		} else {
			if !loaded {
				blob, loaded = s.catalogOf(c.sess), true
				folder = s.savedFolder(c.sess, blob)
			}
			name = linkName(a)
			row.Href = s.attachLinkHref(blob, a.URI)
			row.Place = attachPlace(blob, folder, a.URI)
		}
		row.Name, row.Type = text(name), text(fileType(name, a.Type))
		if a.Size >= 0 {
			row.Size = formatSize(a.Size)
		}
		// Remove stands on the edit only (the owner's decision of
		// 02.10.2026, s-agenda-3): the card's name is a link, and nothing
		// else is done with it there.
		if !c.col.ReadOnly && from == "edit" {
			row.Actions = []view.Action{s.detachAction(c, uid, etag, from, a)}
		}
		rows = append(rows, row)
	}
	return rows
}

// linkName is the name of the file a link points at: its last segment, which
// in the attachment folder carries the number a taken name got.
func linkName(a cal.Attachment) string {
	if u, err := url.Parse(a.URI); err == nil && u.Path != "" {
		if base := path.Base(u.Path); base != "." && base != "/" {
			return base
		}
	}
	if a.FileName != "" {
		return a.FileName
	}
	return a.URI
}

// attachLinkHref is the Carrel address of a link into a live file collection
// of the catalog — its server and a path under its root — or "" for any other
// link: that one is shown and not fetched. The file opens in Files, its
// folder with it chosen (the owner's decision of 02.10.2026: an attachment
// in the record downloads, one in Files opens there).
func (s *Server) attachLinkHref(blob sealed.Blob, uri string) string {
	conn, col, segs, ok := attachLinkAt(blob, uri)
	if !ok {
		return ""
	}
	return s.route(fileHref(conn, col.Path, "", segs, false))
}

// attachPlace is where the file a link points at lies, as the frames of the
// notes say it by the row (s-notes-1, -2, -20): in the attachment folder —
// where that folder lies, as the rail of Files names it («Documents /
// carrel», filesShortcuts); elsewhere in Files — its storage and the
// folders above it; outside the catalog — nothing.
func attachPlace(blob sealed.Blob, folder *fileCtx, uri string) view.Text {
	_, col, segs, ok := attachLinkAt(blob, uri)
	if !ok {
		return ""
	}
	if folder != nil && folder.col.Path == col.Path && folder.conn.ID == col.AccountID &&
		len(segs) > len(folder.segs) && slices.Equal(segs[:len(folder.segs)], folder.segs) {
		place := []string{collectionLabel(folder.col)}
		if len(folder.segs) > 1 {
			place = append(place, folder.segs[:len(folder.segs)-1]...)
		}
		return text(strings.Join(place, " / "))
	}
	return text(strings.Join(append([]string{collectionLabel(col)}, segs[:len(segs)-1]...), " / "))
}

// attachLinkAt is the file collection of the catalog a link points into —
// its connection, the collection and the segments of the path under its
// root —; not ok for any other link.
func attachLinkAt(blob sealed.Blob, uri string) (string, sealed.Collection, []string, bool) {
	u, err := url.Parse(uri)
	if err != nil || !u.IsAbs() || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", sealed.Collection{}, nil, false
	}
	var bestConn string
	var bestCol sealed.Collection
	var bestSegs []string
	bestLen := -1
	for _, col := range blob.Collections {
		conn, ok := blob.Connection(col.AccountID)
		if !ok || col.Gone || !col.Has(view.KindFile) {
			continue
		}
		base, err := url.Parse(conn.Server)
		if err != nil || !strings.EqualFold(base.Scheme, u.Scheme) || !strings.EqualFold(base.Host, u.Host) {
			continue
		}
		root := strings.TrimSuffix(col.Path, "/") + "/"
		rest, under := strings.CutPrefix(u.Path, root)
		if !under || rest == "" || len(root) <= bestLen {
			continue
		}
		segs := strings.Split(rest, "/")
		if slices.ContainsFunc(segs, func(seg string) bool { return file.CheckName(seg) != nil }) {
			continue
		}
		bestConn, bestCol, bestSegs, bestLen = conn.ID, col, segs, len(root)
	}
	return bestConn, bestCol, bestSegs, bestLen >= 0
}

func (s *Server) opQuery(op, etag, from string) url.Values {
	q := url.Values{"op": {op}, "etag": {etag}}
	if from != "" {
		q.Set("from", from)
	}
	return q
}

// attachAction adds a file to the record: the button is a multipart POST of
// a file from this computer. Its arrow (s-agenda-6, s-tasks-7) holds that
// again and From Files — the tree of Files choosing a file that lies there
// already (/app/files/picker?for=attach), written by the same rule.
func (s *Server) attachAction(c *collectionCtx, uid, etag, from string) view.Action {
	// File names the field of the file without scripts; every file of the
	// form is taken, whatever its field is called (nextFile).
	a := view.Action{ID: "attach.add", Label: "Attach", Icon: "i-clip", Href: s.postQuery(c.recordPath(uid)+"/edit", s.opQuery("attach", etag, from)), Method: "POST", Enabled: true,
		File: "file"}
	here := a
	here.Label, here.Icon = "From this computer", "i-upload"
	q := url.Values{"for": {"attach"}, "k": {c.spec.segment}, "a": {c.conn.ID}, "c": {c.col.Path}, "uid": {uid}, "etag": {etag}}
	if from != "" {
		q.Set("from", from)
	}
	a.Arrow = []view.MenuItem{
		{Action: here},
		{Action: view.Action{ID: "attach.files", Label: "From Files", Icon: "i-file", Href: s.postQuery("/app/files/picker", q), Method: "GET", Enabled: true}},
	}
	return a
}

// detachAction takes one attachment off the record. A link goes and its file
// stays, and the label says so; an inline one goes with its only copy.
func (s *Server) detachAction(c *collectionCtx, uid, etag, from string, a cal.Attachment) view.Action {
	q := s.opQuery("detach", etag, from)
	q.Set("n", strconv.Itoa(a.Index))
	act := view.Action{ID: "attach.remove", Label: "Remove", Icon: "i-trash", Href: s.postQuery(c.recordPath(uid)+"/edit", q), Method: "POST", Danger: true, Enabled: true}
	if !a.Inline {
		act.Label, act.Danger = "Remove link, keep the file", false
	}
	return act
}

// attachOf is Attach of a writable record, or none: a contact holds no
// files, and a read-only collection is not written.
func (s *Server) attachOf(c *collectionCtx, uid, etag string) *view.Action {
	if c.col.ReadOnly || c.feed != nil || c.cal == nil {
		return nil
	}
	a := s.attachAction(c, uid, etag, "")
	return &a
}

// Writing.

func attachBack(c *collectionCtx, uid, from string) string {
	if from == "edit" {
		return c.recordPath(uid) + "/edit"
	}
	return c.recordPath(uid)
}

func changedNotice(nothing string) view.Notice {
	return view.Notice{
		Role:  view.NoticeAlert,
		Title: text("Changed elsewhere, so " + nothing),
		Body:  "This record changed after you opened it. Check the current version below and try again if you still mean to.",
	}
}

// nextFile is the next file part of an upload, in the order sent; nil when
// no more came. A field between the files is kept: the sizes of the files
// after it (uploadBody.keep).
func nextFile(up *uploadBody) (*multipart.Part, error) {
	for {
		part := up.next
		up.next = nil
		if part == nil {
			var err error
			if part, err = up.mr.NextPart(); err == io.EOF {
				return nil, nil
			} else if err != nil {
				return nil, err
			}
		}
		if part.FileName() != "" {
			return part, nil
		}
		if err := up.keep(part); err != nil {
			return nil, err
		}
	}
}

// takeBack removes the files put into the attachment folder for a write that
// was not made.
func takeBack(ctx context.Context, placed []*placedFile) {
	for _, p := range placed {
		if p != nil {
			p.remove(ctx)
		}
	}
}

// postAttach writes the files of the form into the record — every one of
// them, by one write (Б6 of the audit of 11.10.2026: only the first was
// taken, the rest dropped without a word). Each goes inside or, when the
// finished object would be larger than the limit, into the attachment folder
// with a link in the record, on the record the files before it made. A file
// that cannot be attached stops them all: the record is not written, and the
// files put in the folder for it are taken away again — nothing points at
// them.
func (s *Server) postAttach(w http.ResponseWriter, r *http.Request, c *collectionCtx, uid string) {
	ctx := r.Context()
	q := r.URL.Query()
	etag, back := strings.TrimSpace(q.Get("etag")), attachBack(c, uid, q.Get("from"))
	const nothing = "nothing was attached"
	var part *multipart.Part
	up, partErr := uploadOf(r)
	if partErr == nil {
		part, partErr = nextFile(up)
	}
	obj, err := c.find(ctx, uid)
	switch {
	case notFoundErr(err):
		s.notFound(w, r)
		return
	case err != nil:
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, nil, s.unavailableNotice(c, c.recordPath(uid), err)))
		return
	case partErr != nil || part == nil:
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, view.Notice{
			Role: view.NoticeAlert, Title: "No file was chosen", Body: "Choose the file to attach and send the form again.",
		}))
		return
	case !sameVersion(etag, obj.ETag):
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, changedNotice(nothing)))
		return
	}
	sent := obj
	var placed []*placedFile
	for n := 0; part != nil; n++ {
		name := uploadName(part.FileName())
		mediaType := fileType(name, part.Header.Get("Content-Type"))
		if mediaType == "application/octet-stream" {
			mediaType = fileType(name, "")
		}
		notice := &view.Notice{Role: view.NoticeAlert, Title: "The upload broke off", Body: "Nothing was attached. Send the file again."}
		head, err := io.ReadAll(io.LimitReader(part, attachLimit(c.col)+1))
		if err == nil {
			// A file that ended inside what was read travels in the record.
			// Past it, the only length this path can know is the one the page
			// sent in front of the files (S-17.1).
			var next *model.Object
			var put *placedFile
			next, put, notice = s.attached(ctx, c, sent, attachFile{
				name: name, mediaType: mediaType, head: head, rest: part, size: declaredSize(r, name),
			})
			if notice == nil {
				sent, placed = next, append(placed, put)
				if part, err = nextFile(up); err == nil && part != nil {
					// The next file is attached to the record as this one left it.
					sent, err = settled(sent)
				}
				if err != nil {
					takeBack(ctx, placed)
					s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, view.Notice{
						Role: view.NoticeAlert, Title: "The upload broke off", Body: "Nothing was attached. Send the files again.",
					}))
					return
				}
				continue
			}
		}
		if notice != nil {
			takeBack(ctx, placed)
			if n > 0 {
				// The files before it were taken and are not written either.
				notice.Body = text(name + " could not be attached, so none of the files was. " + string(notice.Body))
			}
			s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, *notice))
			return
		}
	}
	s.writeAttached(w, r, c, uid, obj, etag, back, sent, placed, nil)
}

// attachFile is a file on its way into a record: its name and type, and
// what was read of it — at most one byte past the limit; nil when its size
// said it could not fit. An upload has the rest of its body; a file that
// lies in Files already has link, its address as another client opens it.
type attachFile struct {
	name, mediaType string
	head            []byte
	rest            io.Reader // an upload past head; nil for a file of Files
	size            int64     // the length when known, else -1
	link            string    // where a file of Files lies; "" for an upload
}

// attached is the record with the file in it, by the one rule of S-20
// whatever the file came from: inside when the finished object stays within
// the limit, else a link — to where a file of Files lies, or to the copy of
// an upload put first into the attachment folder. placed is that copy, taken
// back when the write is refused; a notice says why nothing was done.
func (s *Server) attached(ctx context.Context, c *collectionCtx, obj *model.Object, in attachFile) (*model.Object, *placedFile, *view.Notice) {
	limit := attachLimit(c.col)
	if in.head != nil && int64(len(in.head)) <= limit {
		if sent := s.inlineTrial(c, obj, in.name, in.mediaType, in.head, limit); sent != nil {
			return sent, nil, nil
		}
	}
	uri, size := in.link, in.size
	var placed *placedFile
	if uri == "" {
		var notice *view.Notice
		placed, notice = s.placeAttachment(ctx, c, obj, in.name, in.mediaType, io.MultiReader(bytes.NewReader(in.head), in.rest), limit, in.size)
		if notice != nil {
			return nil, nil, notice
		}
		uri, size = placed.uri, placed.size
	}
	sent, err := patched(obj, cal.AttachLink(obj, uri, in.name, in.mediaType, size))
	if err != nil {
		if placed != nil {
			placed.remove(ctx)
		}
		n := attachFailure(s.writeFailure(c, "PUT", obj.Path, err))
		return nil, nil, &n
	}
	return sent, placed, nil
}

// postAttachFile attaches a file that lies in Files already (From Files):
// ?file= names it by its address in the files section. It is read only when
// its size lets it fit in the record, and never more than one byte past the
// limit; a larger one is linked where it lies, and nothing is copied. The
// storage may be read-only — it is only read. A subscribed folder is no
// source: a link to its file would carry someone's secret address.
func (s *Server) postAttachFile(w http.ResponseWriter, r *http.Request, c *collectionCtx, uid string) {
	ctx := r.Context()
	q := r.URL.Query()
	etag, back := strings.TrimSpace(q.Get("etag")), attachBack(c, uid, q.Get("from"))
	src, dir, chosen := s.fileAddress(c.sess, q.Get("file"))
	obj, err := c.find(ctx, uid)
	notChosen := view.Notice{Role: view.NoticeAlert, Title: "No file was chosen.", Body: "Choose the file to attach in Files and try again."}
	switch {
	case notFoundErr(err):
		s.notFound(w, r)
		return
	case err != nil:
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, nil, s.unavailableNotice(c, c.recordPath(uid), err)))
		return
	case !chosen || dir || src.col.Feed || len(src.segs) == 0:
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, notChosen))
		return
	case !sameVersion(etag, obj.ETag):
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, changedNotice("nothing was attached")))
		return
	}
	p, err := src.path(src.segs, false)
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, notChosen))
		return
	}
	failed := func(method string, err error) {
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, s.fileFailure(src, "Nothing was attached", method, p, err)))
	}
	e, err := src.files.Stat(ctx, p)
	switch {
	case file.NotFound(err):
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, view.Notice{
			Role: view.NoticeAlert, Title: "Nothing was attached: the file is no longer there.",
			Body: "It was moved or deleted after the tree was read. Choose it again.",
		}))
		return
	case err != nil:
		failed("PROPFIND", err)
		return
	case e.Dir:
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, notChosen))
		return
	}
	name := src.segs[len(src.segs)-1]
	in := attachFile{name: name, mediaType: fileType(name, e.Type), size: e.Size, link: fileURI(src.conn.Server, p)}
	if limit := attachLimit(c.col); e.Size <= limit {
		body, _, err := src.files.Get(ctx, p, nil)
		if err != nil {
			failed("GET", err)
			return
		}
		in.head, err = io.ReadAll(io.LimitReader(body, limit+1))
		_ = body.Close()
		if err != nil {
			failed("GET", err)
			return
		}
		if int64(len(in.head)) > limit {
			// The server said less than it holds: the length is not known.
			in.size = -1
		}
	}
	sent, placed, notice := s.attached(ctx, c, obj, in)
	s.writeAttached(w, r, c, uid, obj, etag, back, sent, []*placedFile{placed}, notice)
}

// writeAttached writes the record its attachments made, with If-Match of the
// version the page showed, and answers where the page came from. A refusal
// takes back the files put into the attachment folder for it.
func (s *Server) writeAttached(w http.ResponseWriter, r *http.Request, c *collectionCtx, uid string, obj *model.Object, etag, back string, sent *model.Object, placed []*placedFile, notice *view.Notice) {
	ctx := r.Context()
	if notice != nil {
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, *notice))
		return
	}
	sent.ETag = etag
	if err := c.update(ctx, sent); err != nil {
		takeBack(ctx, placed)
		n := attachFailure(s.writeFailure(c, "PUT", obj.Path, err))
		if dav.IsStatus(err, http.StatusPreconditionFailed) {
			n = changedNotice("nothing was attached")
		}
		if current, ferr := c.find(ctx, uid); ferr == nil {
			obj = current
		}
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, n))
		return
	}
	s.finishWrite(w, r, c, uid, sent, back)
}

func attachFailure(n view.Notice) view.Notice {
	n.Body = "Nothing was attached."
	return n
}

// patched is obj with ops applied; obj itself is left as it was read.
func patched(obj *model.Object, ops []model.PatchOp) (*model.Object, error) {
	out, err := cal.Parse(obj.Path, obj.ETag, bytes.NewReader(obj.CalendarDoc()))
	if err != nil {
		return nil, err
	}
	patch, err := model.NewPatch(ops...)
	if err != nil {
		return nil, err
	}
	if err := patch.Apply(out); err != nil {
		return nil, err
	}
	return out, nil
}

// settled is obj as it reads once written out: what patched makes of it
// holds the patches already applied to it.
func settled(obj *model.Object) (*model.Object, error) {
	var buf bytes.Buffer
	if err := cal.Encode(&buf, obj); err != nil {
		return nil, err
	}
	return cal.Parse(obj.Path, obj.ETag, &buf)
}

// inlineTrial is the record with the file inside, when the finished object
// stays within the limit; nil when it would not.
func (s *Server) inlineTrial(c *collectionCtx, obj *model.Object, name, mediaType string, data []byte, limit int64) *model.Object {
	trial, err := patched(obj, cal.AttachInline(obj, name, mediaType, data))
	if err != nil {
		return nil
	}
	var buf bytes.Buffer
	if err := cal.Encode(&buf, trial); err != nil || int64(buf.Len()+stampSlack) > limit {
		return nil
	}
	return trial
}

// placedFile is a large attachment written to the attachment folder.
type placedFile struct {
	files *fileCtx
	path  string
	uri   string
	size  int64
	etag  string
}

// remove takes back a file no record points at.
func (p *placedFile) remove(ctx context.Context) {
	_ = p.files.files.Delete(context.WithoutCancel(ctx), p.path, p.etag)
	forgetFolder(p.files.sess, p.files.conn.ID, path.Dir(p.path)+"/")
}

// placeAttachment writes body to the attachment folder under a free name: the
// folder is made on first use, a taken name gets a number and nothing is
// overwritten. A notice says why nothing was written.
// size is the length of body when it is known: the file ended inside what was
// read to measure it against the limit, or the page sent it. -1 means nothing
// said, and then the body goes out as a stream (S-17.1).
func (s *Server) placeAttachment(ctx context.Context, c *collectionCtx, obj *model.Object, name, mediaType string, body io.Reader, limit, size int64) (*placedFile, *view.Notice) {
	large := fmt.Sprintf("The file is larger than %s, the most %s takes inside a record, so it goes to the folder for attachments.", sizeText(limit), collectionLabel(c.col))
	settings := []view.Action{{ID: "settings.attachments", Label: "Choose the folder", Href: s.route("/app/settings/attachments"), Method: "GET", Enabled: true}}
	refuse := func(title, body string) (*placedFile, *view.Notice) {
		return nil, &view.Notice{Role: view.NoticeAlert, Title: text("Nothing was attached: " + title), Body: text(large + " " + body), Actions: settings}
	}
	fc := s.savedFolder(c.sess, s.catalogOf(c.sess))
	switch {
	case fc == nil && s.catalogOf(c.sess).Attachments == nil:
		return refuse("no folder for attachments is chosen", "Choose that folder once in Settings → Attachments.")
	case fc == nil:
		return refuse("the folder for attachments is not available", "Its storage is no longer among your collections. Choose another folder in Settings → Attachments.")
	case fc.col.ReadOnly:
		return refuse("the folder for attachments is read-only", "The server lets this account read that storage but not write. Choose another folder in Settings → Attachments.")
	}
	failed := func(method, p string, err error) (*placedFile, *view.Notice) {
		n := s.fileFailure(fc, "Nothing was attached", method, p, err)
		return nil, &n
	}
	day := attachmentDay(c, obj)
	if af := s.catalogOf(c.sess).Attachments; af != nil && af.Monthly {
		// A subfolder per month (attachments-1): the month the dated name
		// carries, made on first use like the folder itself.
		fc = fc.at(append(slices.Clone(fc.segs), day.Format("2006-01")))
	}
	folder, err := fc.path(fc.segs, true)
	if err != nil {
		return refuse("the folder for attachments cannot be used", "Choose another folder in Settings → Attachments.")
	}
	if _, err := fc.files.Stat(ctx, folder); file.NotFound(err) {
		for i := range fc.segs {
			p, _ := fc.path(fc.segs[:i+1], true)
			if err := fc.files.MkCol(ctx, p); err != nil && !dav.IsStatus(err, http.StatusMethodNotAllowed) {
				return failed("MKCOL", p, err)
			}
		}
	} else if err != nil {
		return failed("PROPFIND", folder, err)
	}
	base := attachmentFileName(s.catalogOf(c.sess).AttachNames, day, attachmentTitle(c, obj), name)
	for n := 1; n <= attachNameTries; n++ {
		candidate := base
		if n > 1 {
			candidate = file.Suffixed(base, n)
		}
		target, err := fc.path(append(slices.Clone(fc.segs), candidate), false)
		if err != nil {
			return refuse("the file has no usable name", "Rename it and attach it again.")
		}
		if _, err := fc.files.Stat(ctx, target); err == nil {
			continue
		} else if !file.NotFound(err) {
			return failed("PROPFIND", target, err)
		}
		counted := &countingReader{r: body}
		var send io.Reader = dav.Typed(counted, mediaType)
		if size >= 0 {
			send = dav.Sized(send, size)
		}
		etag, err := fc.files.UploadCreate(ctx, target, send)
		forgetFolder(c.sess, fc.conn.ID, folder)
		if err != nil {
			if size < 0 && needsLength(err) {
				// S-17.1: this server writes only what it was told the size of,
				// and Carrel does not keep the file to measure it.
				return refuse("this server needs the size of a file before it writes it",
					"The page did not send one. Turn scripts on, or attach a file small enough to travel inside the record.")
			}
			return failed("PUT", target, err)
		}
		if n > 1 {
			// A name already taken got a counter, and the queue says so
			// (attachments-1): a row of the files queue, done, naming what the
			// file was written as.
			if t, err := newTransfer("upload", fc, candidate); err == nil {
				t.state, t.size, t.note = view.OpDone, counted.n.Load(), base+" was taken, so a counter was added"
				c.sess.KeepTransfer(t)
			}
		}
		return &placedFile{files: fc, path: target, uri: fileURI(fc.conn.Server, target), size: counted.n.Load(), etag: etag}, nil
	}
	return refuse("every name for it is taken", fmt.Sprintf("The folder already holds %s and its numbered copies up to -%d.", base, attachNameTries))
}

// fileURI is the address of a path on the connection's server, as another
// client opens it.
func fileURI(server, p string) string {
	u, err := url.Parse(server)
	if err != nil {
		return p
	}
	return (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: p}).String()
}

// savedFolder is the attachment folder of the catalog in a live file
// collection; nil when none is chosen or its collection is gone.
func (s *Server) savedFolder(sess *session.Session, blob sealed.Blob) *fileCtx {
	af := blob.Attachments
	if af == nil {
		return nil
	}
	fc, ok := s.fileCollection(sess, af.AccountID, af.Path)
	if !ok {
		return nil
	}
	if _, err := fc.path(af.Folder, true); err != nil {
		return nil
	}
	return fc.at(af.Folder)
}

// postDetach takes the attachment at n off the record, with If-Match of the
// version the page showed; its index means what it meant only there. A file a
// link points at is not touched.
func (s *Server) postDetach(w http.ResponseWriter, r *http.Request, c *collectionCtx, uid string) {
	ctx := r.Context()
	q := r.URL.Query()
	etag, back := strings.TrimSpace(q.Get("etag")), attachBack(c, uid, q.Get("from"))
	n, nerr := strconv.Atoi(q.Get("n"))
	obj, err := c.find(ctx, uid)
	switch {
	case notFoundErr(err):
		s.notFound(w, r)
		return
	case err != nil:
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, nil, s.unavailableNotice(c, c.recordPath(uid), err)))
		return
	case !sameVersion(etag, obj.ETag):
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, changedNotice("nothing was removed")))
		return
	}
	ops, err := cal.Detach(obj, n)
	if nerr != nil || err != nil {
		s.notFound(w, r)
		return
	}
	sent, err := patched(obj, ops)
	if err == nil {
		sent.ETag = etag
		err = c.update(ctx, sent)
	}
	switch {
	case dav.IsStatus(err, http.StatusPreconditionFailed):
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, changedNotice("nothing was removed")))
	case err != nil:
		notice := s.writeFailure(c, "PUT", obj.Path, err)
		notice.Body = "Nothing was removed."
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, notice))
	default:
		s.finishWrite(w, r, c, uid, sent, back)
	}
}

// Opening.

// getAttach opens an attachment: a file under a file collection of the
// catalog, streamed like a download, or {uid}/{n} of a calendar collection,
// an inline file served from its record. Anything else is 404.
func (s *Server) getAttach(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	account, col, segs, dir, ok := parseAddress(r.URL.EscapedPath(), "/app/attach/", "")
	if !ok || dir || len(segs) == 0 {
		s.notFound(w, r)
		return
	}
	if !s.checkOwnership(w, r, sess, account, col, "") {
		return
	}
	blob := s.catalogOf(sess)
	entry, ok := blob.Collection(account, col)
	if !ok || entry.Gone {
		s.notFound(w, r)
		return
	}
	inline := r.URL.Query().Get("inline") != ""
	if entry.Has(view.KindFile) {
		fc, ok := s.fileCollection(sess, account, col)
		if !ok {
			s.notFound(w, r)
			return
		}
		p, err := fc.path(segs, false)
		if err != nil {
			s.notFound(w, r)
			return
		}
		name := segs[len(segs)-1]
		switch err := s.sendFile(w, r, fc, p, name, inline); {
		case err == nil:
		case file.NotFound(err) || inline:
			s.notFound(w, r)
		default:
			s.writeFolder(w, r, fc, segs[:len(segs)-1], "", url.Values{}, "", s.fileFailure(fc, name+" could not be opened", "GET", p, err))
		}
		return
	}
	s.sendInlineAttachment(w, r, sess, blob, entry, segs, inline)
}

func (s *Server) sendInlineAttachment(w http.ResponseWriter, r *http.Request, sess *session.Session, blob sealed.Blob, entry sealed.Collection, segs []string, inline bool) {
	n, err := strconv.Atoi(segsAt(segs, 1))
	conn, hasConn := blob.Connection(entry.AccountID)
	if len(segs) != 2 || err != nil || !hasConn {
		s.notFound(w, r)
		return
	}
	uid := segs[0]
	for _, k := range entry.Kinds {
		spec, ok := recordKinds[segmentOf(k)]
		if !ok || spec.component == "" {
			continue
		}
		c, err := s.newCollectionCtx(sess, spec, conn, entry)
		if err != nil {
			continue
		}
		obj, err := c.find(r.Context(), uid)
		switch {
		case notFoundErr(err):
			continue
		case err != nil:
			s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, nil, s.unavailableNotice(c, c.recordPath(uid), err)))
			return
		}
		a, data, err := cal.AttachmentData(obj, n)
		if err != nil {
			s.notFound(w, r)
			return
		}
		name := a.FileName
		if name == "" {
			name = fmt.Sprintf("attachment-%d", n+1)
		}
		downloadHeaders(w.Header(), name, a.Type, inline)
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
		return
	}
	s.notFound(w, r)
}

func segsAt(segs []string, i int) string {
	if i < len(segs) {
		return segs[i]
	}
	return ""
}
