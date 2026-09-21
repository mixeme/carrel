package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/provider/file"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// Writes to a file collection (S-17.1, S-17.3). An upload never overwrites: a
// taken name is a question in the transfer queue — Skip, Keep both, Overwrite
// this one — and so is a move or a copy onto one. The name is looked up
// before the conditional PUT: a server that ignores If-None-Match (SFTPGo,
// docs/servers.md, SV-8)
// would overwrite otherwise. An upload that asks keeps its file in the
// temporary directory until the answer or the end of the session.

// transfer is one row of the queue. It does not change once kept: an answer
// takes it out and keeps what came of it.
type transfer struct {
	id    string
	kind  string // upload, rename, move, copy
	state view.OpState
	note  string // why it failed, or what it did instead
	size  int64
	dir   bool

	// where it goes: a folder of a collection, the folders inside it a folder
	// upload adds (S-17.1), and the name
	account, col string
	folder       []string
	rel          []string
	name         string

	// what it moves: an upload's file on disk, or a member of a collection
	temp               string
	srcAccount, srcCol string
	src                []string
}

// label is what the queue row says: the path inside the chosen folder for a
// folder upload, so two files of one name are told apart, and the name alone
// for everything else.
func (t *transfer) label() string {
	return strings.Join(append(slices.Clone(t.rel), t.name), "/")
}

func (t *transfer) ID() string    { return t.id }
func (t *transfer) Waiting() bool { return t.state == view.OpIdle }
func (t *transfer) Discard() {
	if t.temp != "" {
		_ = os.Remove(t.temp)
	}
}

func newTransfer(kind string, dest *fileCtx, name string) (*transfer, error) {
	id, err := user.NewTransferID()
	if err != nil {
		return nil, err
	}
	return &transfer{id: id, kind: kind, account: dest.conn.ID, col: dest.col.Path, folder: slices.Clone(dest.segs), name: name}, nil
}

func (t *transfer) marker() string {
	if t.kind == "upload" {
		return "upload"
	}
	return "op"
}

// transferQueue is the session's queue as the queue target shows it: every
// question, and the rows finished since it was last shown. No rows, no queue.
func (s *Server) transferQueue(sess *session.Session) *view.TransferQueue {
	rows := sess.ReadTransfers()
	if len(rows) == 0 {
		return nil
	}
	q := &view.TransferQueue{}
	counts := map[view.OpState]int{}
	var reasons []string
	for _, row := range rows {
		t, ok := row.(*transfer)
		if !ok {
			continue
		}
		counts[t.state]++
		tr := view.TransferRow{Name: text(t.label()), State: t.state}
		if !t.dir {
			tr.Size = formatSize(t.size)
		}
		if t.state == view.OpDone {
			tr.Percent = 100
		}
		if t.note != "" {
			reasons = append(reasons, t.label()+": "+t.note)
		}
		if t.Waiting() {
			answer := func(id, label, value string, danger bool) view.Action {
				return view.Action{
					ID: id, Label: text(label), Method: "POST", Danger: danger, Enabled: true,
					Href: s.postQuery(fileHref(t.account, t.col, t.marker(), t.folder, true), url.Values{"transfer": {t.id}, "answer": {value}}),
				}
			}
			tr.Question = &view.NameClash{Name: text(t.label()), Options: []view.Action{
				answer("transfer.skip", "Skip", "skip", false),
				answer("transfer.both", "Keep both", "both", false),
				answer("transfer.overwrite", "Overwrite this one", "overwrite", true),
			}}
		}
		q.Rows = append(q.Rows, tr)
	}
	var parts []string
	for _, c := range []struct {
		state view.OpState
		what  string
	}{{view.OpIdle, "waiting for an answer: the name is taken"}, {view.OpDone, "done"}, {view.OpFailed, "failed"}, {view.OpCancelled, "skipped"}} {
		if n := counts[c.state]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, c.what))
		}
	}
	q.Note = text(strings.Join(append(parts, reasons...), " · "))
	return q
}

// Upload (C-5.5). The body is read as it arrives: the middleware read the
// fields in front of the first file for _csrf, and every file part goes to the
// server or, when its name is taken, to a temporary file for the question.

// uploadBody is a multipart upload whose files are still unread.
type uploadBody struct {
	fields url.Values
	mr     *multipart.Reader
	next   *multipart.Part // the first file part, opened; nil when none came
}

type uploadKey struct{}

// isFileUpload reports whether r is a multipart POST that carries a file to a
// storage: the upload route, or an attachment to a record (S-20), which goes
// to the attachment folder when it is large. Its body has no cap of Carrel's
// (C-12): only the fields in front of the files are bounded.
func isFileUpload(r *http.Request) bool {
	if r.Method != http.MethodPost || !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		return false
	}
	seg := strings.Split(r.URL.EscapedPath(), "/")
	switch {
	case len(seg) >= 7 && seg[1] == "app" && seg[2] == "files" && seg[5] == "upload":
		return true
	case len(seg) == 7 && seg[1] == "app" && seg[6] == "edit" && r.URL.Query().Get("op") == "attach":
		return recordKinds[seg[2]].component != ""
	case len(seg) == 4 && seg[1] == "app" && seg[2] == "settings" && seg[3] == "backup" && r.URL.Query().Get("op") == "restore-open":
		// An archive brought from this computer: as large as a copy is.
		return true
	}
	return false
}

// readUploadFields reads the fields in front of the first file, a form's worth
// of bytes at most (MaxFormBodyBytes), and stops at that file.
func readUploadFields(r *http.Request) (*uploadBody, error) {
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, err
	}
	up := &uploadBody{fields: url.Values{}, mr: mr}
	remain := int64(conf.MaxFormBodyBytes)
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			return up, nil
		}
		if err != nil {
			return nil, err
		}
		if part.FileName() != "" {
			up.next = part
			return up, nil
		}
		data, err := io.ReadAll(io.LimitReader(part, remain+1))
		if err != nil {
			return nil, err
		}
		if remain -= int64(len(data)); remain < 0 {
			return nil, &http.MaxBytesError{Limit: conf.MaxFormBodyBytes}
		}
		up.fields.Add(part.FormName(), string(data))
	}
}

func uploadOf(r *http.Request) (*uploadBody, error) {
	if up, ok := r.Context().Value(uploadKey{}).(*uploadBody); ok {
		return up, nil
	}
	return readUploadFields(r)
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func (s *Server) postFilesUpload(w http.ResponseWriter, r *http.Request) {
	c, ok := s.openFiles(w, r, "upload")
	if !ok {
		return
	}
	if !c.dir {
		s.notFound(w, r)
		return
	}
	if id := r.URL.Query().Get("transfer"); id != "" {
		s.answerTransfer(w, r, c, "upload", id)
		return
	}
	folder, _ := c.path(c.segs, true)
	if c.col.ReadOnly {
		s.answerUpload(w, r, c, readOnlyNotice)
		return
	}
	up, err := uploadOf(r)
	if err != nil {
		s.answerUpload(w, r, c, view.Notice{Role: view.NoticeAlert, Title: "Nothing was uploaded", Body: "The form did not carry a file."})
		return
	}
	n := 0
	made := map[string]bool{}
	for part := up.next; ; part = nil {
		if part == nil {
			if part, err = up.mr.NextPart(); err != nil {
				break
			}
		}
		if part.FileName() == "" {
			continue
		}
		n++
		if err := s.uploadOne(r, c, part, made); err != nil {
			http.Error(w, "could not keep the upload", http.StatusInternalServerError)
			return
		}
	}
	forgetFolder(c.sess, c.conn.ID, folder)
	switch {
	case err != nil && err != io.EOF:
		s.answerUpload(w, r, c, view.Notice{Role: view.NoticeAlert, Title: "The upload broke off", Body: "What arrived before is in the queue. Upload the rest again."})
	case n == 0:
		s.answerUpload(w, r, c, view.Notice{Role: view.NoticeAlert, Title: "No file was chosen", Body: "Choose the files to upload and send the form again."})
	default:
		s.answerUpload(w, r, c)
	}
}

// uploadOne sends one file part: straight to the server when its name is
// free, to a temporary file and a question when it is taken. The length goes
// with it when the page sent one (S-17.1). A folder upload names every part by
// its path inside the chosen folder, so the folders it needs are created
// first, and made keeps the ones this upload has already made.
func (s *Server) uploadOne(r *http.Request, c *fileCtx, part *multipart.Part, made map[string]bool) error {
	ctx := r.Context()
	rel, name := uploadPath(partName(part))
	t, err := newTransfer("upload", c, name)
	if err != nil {
		return err
	}
	t.rel = rel
	defer func() { c.sess.KeepTransfer(t) }()
	if name == "" {
		t.name, t.state, t.note = part.FileName(), view.OpFailed, "this cannot be the name of a file"
		return nil
	}
	target, err := c.path(append(append(slices.Clone(c.segs), rel...), name), false)
	if err != nil {
		t.state, t.note = view.OpFailed, "this cannot be the name of a file"
		return nil
	}
	if err := s.makeFolders(ctx, c, rel, made); err != nil {
		t.state, t.note = view.OpFailed, "the folder it goes in was not created: "+sentenceEnd(fileReason(err))
		return nil
	}
	_, err = c.files.Stat(ctx, target)
	switch {
	case err == nil:
		f, err := os.CreateTemp("", "carrel-upload-*")
		if err != nil {
			t.state, t.note = view.OpFailed, "Carrel could not keep it to ask about the name"
			return nil
		}
		t.size, err = io.Copy(f, part)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			_ = os.Remove(f.Name())
			t.state, t.note = view.OpFailed, "the upload broke off"
			return nil
		}
		t.temp, t.state = f.Name(), view.OpIdle
	case file.NotFound(err):
		body := &countingReader{r: part}
		var send io.Reader = dav.Typed(body, uploadType(part, name))
		declared := declaredSize(r, t.label())
		if declared >= 0 {
			send = dav.Sized(send, declared)
		}
		_, err = c.files.UploadCreate(ctx, target, send)
		t.size = body.n
		switch {
		case err == nil:
			t.state = view.OpDone
		case file.ErrOccupied(err):
			t.state, t.note = view.OpFailed, "the name was taken while it uploaded; nothing was overwritten. Upload it again to choose"
		case declared < 0 && needsLength(err):
			// S-17.1: the length was not sent and this server will not write
			// without one. Carrel does not keep the body to measure it — there
			// is no ceiling on a file — so it says so instead.
			t.state, t.note = view.OpFailed, "this server writes a file only when its size is sent first, and the page did not send it. Turn scripts on, or put the file there with another client"
		default:
			t.state, t.note = view.OpFailed, sentenceEnd(fileReason(err))
		}
	default:
		t.state, t.note = view.OpFailed, sentenceEnd(fileReason(err))
	}
	return nil
}

// partName is the file name a part carries, whole. Part.FileName cuts the
// directory off it — RFC 7578 §4.2 tells a server not to use path information,
// and Go obeys — but a folder upload carries the path of each file inside the
// chosen folder there, and that path is the one thing it must keep (S-17.1).
// What to make of the name is uploadPath's business, not this one's.
func partName(part *multipart.Part) string {
	if _, params, err := mime.ParseMediaType(part.Header.Get("Content-Disposition")); err == nil {
		if name := params["filename"]; name != "" {
			return name
		}
	}
	return part.FileName()
}

// uploadPath takes a part's file name apart: a folder upload names every file
// by its path inside the chosen folder ("photos/2020/a.jpg"), a plain upload
// by its name alone. Browsers write that path with "/" and older ones with
// "\\"; an empty segment and "." carry nothing and go. ".." stays, to be
// refused where the address is built rather than normalised away in silence
// (S-17.4).
func uploadPath(filename string) (rel []string, name string) {
	sep := func(r rune) bool { return r == '/' || r == '\\' }
	segs := strings.FieldsFunc(filename, sep)
	kept := make([]string, 0, len(segs))
	for _, seg := range segs {
		if seg == "." {
			continue
		}
		kept = append(kept, seg)
	}
	if len(kept) == 0 {
		return nil, ""
	}
	// An address on the person's own machine is not a path inside the chosen
	// folder: a browser that sends "C:\fakepath\photo.jpg" for one file
	// means the name alone, and so does a name that starts at the root.
	if sep(rune(filename[0])) || strings.Contains(kept[0], ":") {
		return nil, kept[len(kept)-1]
	}
	return kept[:len(kept)-1], kept[len(kept)-1]
}

// makeFolders creates the folders a file's path inside the chosen folder needs,
// outermost first and once each per upload (S-17.1). A folder already there
// answers 405, which is not a failure: a folder upload sent twice writes into
// what is there rather than refusing the lot.
func (s *Server) makeFolders(ctx context.Context, c *fileCtx, rel []string, made map[string]bool) error {
	for i := range rel {
		p, err := c.path(append(slices.Clone(c.segs), rel[:i+1]...), true)
		if err != nil {
			return err
		}
		if made[p] {
			continue
		}
		switch err := c.files.MkCol(ctx, p); {
		case err == nil, dav.IsStatus(err, http.StatusMethodNotAllowed):
			made[p] = true
		default:
			return err
		}
	}
	return nil
}

// declaredSize is the length of a file the page sent in front of the files —
// "size.<name>", where the name is the path a folder upload gave the file —
// or -1 when nothing said. A body of unknown length goes out
// without a Content-Length, and some servers refuse that (S-17.1,
// docs/servers.md, SV-15); measuring it here would mean keeping the whole file,
// and there is no ceiling on one (C-12).
func declaredSize(r *http.Request, name string) int64 {
	up, err := uploadOf(r)
	if err != nil {
		return -1
	}
	for _, key := range []string{"size." + name, "size." + path.Base(name), "size"} {
		if v := up.fields.Get(key); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
				return n
			}
		}
	}
	return -1
}

// needsLength reports the refusals a server gives a body whose length it was
// not told: OpenCloud behind Apache answers 400 over HTTP/1.1 and 502 over
// HTTP/2 (docs/servers.md, SV-15).
func needsLength(err error) bool {
	return dav.IsStatus(err, http.StatusBadRequest) || dav.IsStatus(err, http.StatusBadGateway) ||
		dav.IsStatus(err, http.StatusLengthRequired)
}

// sentenceEnd is a reason as part of a longer line: without its full stop.
func sentenceEnd(s string) string {
	return strings.TrimSuffix(s, ".")
}

// uploadType is the type the browser declared for a part, else the one its
// extension names.
func uploadType(part *multipart.Part, name string) string {
	if t := fileType("", part.Header.Get("Content-Type")); t != "" && t != "application/octet-stream" {
		return part.Header.Get("Content-Type")
	}
	if t := fileType(name, ""); t != "" {
		return t
	}
	return "application/octet-stream"
}

// answerUpload is the queue as a fragment (C-7.3), or the folder again for a
// plain form: its address after a success, the page with what went wrong
// otherwise.
func (s *Server) answerUpload(w http.ResponseWriter, r *http.Request, c *fileCtx, notices ...view.Notice) {
	if WantsFragment(r) {
		q := s.transferQueue(c.sess)
		if q == nil {
			q = &view.TransferQueue{}
		}
		for _, n := range notices {
			q.Note = text(strings.TrimPrefix(string(q.Note)+" · "+string(n.Title)+". "+string(n.Body), " · "))
		}
		_ = s.render.Fragment(w, r, http.StatusOK, TargetQueue, q, nil)
		return
	}
	if len(notices) == 0 {
		s.redirect(w, r, c.href("", c.segs, true))
		return
	}
	s.writeFolder(w, r, c, c.segs, "", url.Values{}, "", notices...)
}

// New folder (C-5.5): MKCOL of the name the form sends in the folder of the
// address.
func (s *Server) postFilesMkcol(w http.ResponseWriter, r *http.Request) {
	c, ok := s.openFiles(w, r, "mkcol")
	if !ok {
		return
	}
	if !c.dir {
		s.notFound(w, r)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	folder, _ := c.path(c.segs, true)
	var notices []view.Notice
	switch {
	case c.col.ReadOnly:
		notices = append(notices, readOnlyNotice)
	case file.CheckName(name) != nil:
		notices = append(notices, view.Notice{Role: view.NoticeAlert, Title: "Nothing was created", Body: "A folder name cannot be empty, “.” or “..”, and cannot hold “/” or “\\”."})
	default:
		p, _ := c.path(append(slices.Clone(c.segs), name), true)
		err := c.files.MkCol(r.Context(), p)
		forgetFolder(c.sess, c.conn.ID, folder)
		switch {
		case err == nil:
		// It is already here because this very request put it here: the
		// guard sent the MKCOL twice after a connection died before the
		// answer. The folder is made, and saying otherwise sends the
		// person looking for a name that is taken by their own click.
		case landedOnTheFirstTry(err):
		case dav.IsStatus(err, http.StatusMethodNotAllowed):
			notices = append(notices, view.Notice{Role: view.NoticeAlert, Title: text(name + " is already here"), Body: "A file or folder with this name is in this folder. Nothing was created."})
		case dav.IsStatus(err, http.StatusConflict):
			notices = append(notices, view.Notice{Role: view.NoticeAlert, Title: "This folder is no longer there", Body: "The server has no folder to create it in. Nothing was created."})
		default:
			notices = append(notices, s.fileFailure(c, "The folder was not created", "MKCOL", p, err))
		}
	}
	if len(notices) == 0 && !WantsFragment(r) {
		s.redirect(w, r, c.href("", c.segs, true))
		return
	}
	s.writeFolder(w, r, c, c.segs, "", url.Values{}, TargetMain, notices...)
}

// Operations (C-5.5): rename, move, copy and delete over names of one folder,
// one address with the named operation. Without go=1 the answer is the sheet,
// which says what will happen before it does; with it, the operation runs.

func (s *Server) postFilesOp(w http.ResponseWriter, r *http.Request) {
	c, ok := s.openFiles(w, r, "op")
	if !ok {
		return
	}
	q := r.URL.Query()
	if !c.dir {
		s.notFound(w, r)
		return
	}
	// The tree of the sheet chooses a folder with a form field; the confirmed
	// operation names it in its address.
	if to := r.PostFormValue("to"); to != "" {
		q.Set("to", to)
	}
	if id := q.Get("transfer"); id != "" {
		s.answerTransfer(w, r, c, "op", id)
		return
	}
	op := view.FileOp(q.Get("op"))
	names := q["item"]
	switch {
	case op != view.FileRename && op != view.FileMove && op != view.FileCopy && op != view.FileDelete,
		len(names) == 0, op == view.FileRename && len(names) != 1:
		s.notFound(w, r)
		return
	}
	folder, _ := c.path(c.segs, true)
	entries, err := s.readFolder(r.Context(), c, folder, false)
	switch {
	case file.NotFound(err):
		s.notFound(w, r)
		return
	case err != nil:
		s.writeFolder(w, r, c, c.segs, "", url.Values{}, TargetMain)
		return
	}
	var items []file.Entry
	var missing []string
	for _, name := range names {
		if i := slices.IndexFunc(entries, func(e file.Entry) bool { return e.Name == name }); i >= 0 {
			if !slices.ContainsFunc(items, func(e file.Entry) bool { return e.Name == name }) {
				items = append(items, entries[i])
			}
		} else {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		s.writeFolder(w, r, c, c.segs, "", url.Values{}, TargetMain, view.Notice{
			Role:  view.NoticeAlert,
			Title: "Nothing was done",
			Body:  text("No longer in this folder: " + strings.Join(missing, ", ") + "."),
		})
		return
	}
	if q.Get("go") == "" {
		sheet := s.opSheet(r, c, op, items, q)
		if WantsFragment(r) {
			_ = s.render.Fragment(w, r, http.StatusOK, TargetSheet, sheet, nil)
			return
		}
		s.writePage(w, r, http.StatusOK, sheet)
		return
	}
	s.runOp(w, r, c, op, items, q)
}

// destination is the folder a move or a copy goes to: ?to= names it by its
// address, and the folder of the address is the default. A folder the
// catalog does not hold as a live file collection is no destination.
func (s *Server) destination(c *fileCtx, q url.Values) (*fileCtx, bool) {
	to := q.Get("to")
	if to == "" {
		return c.at(c.segs), true
	}
	return s.folderAddress(c.sess, to)
}

// folderAddress is the folder a href of the files section names — a node of
// the tree (C-6.7) — in a live file collection of the catalog.
func (s *Server) folderAddress(sess *session.Session, href string) (*fileCtx, bool) {
	u, err := url.Parse(href)
	if err != nil || href == "" {
		return nil, false
	}
	escaped := u.EscapedPath()
	if s.basePath != "" {
		rest, ok := strings.CutPrefix(escaped, s.basePath)
		if !ok {
			return nil, false
		}
		escaped = rest
	}
	account, col, segs, dir, ok := parseFileAddress(escaped, "")
	if !ok || !dir {
		return nil, false
	}
	dest, ok := s.fileCollection(sess, account, col)
	if !ok {
		return nil, false
	}
	return dest.at(segs), true
}

// intoItself reports whether a folder would go under itself on its own
// server: a move there cannot happen, and a copy would never end.
func intoItself(c *fileCtx, e file.Entry, dest *fileCtx) bool {
	if !e.Dir || dest.conn.ID != c.conn.ID {
		return false
	}
	src, _ := c.path(append(slices.Clone(c.segs), e.Name), true)
	to, _ := dest.path(dest.segs, true)
	return strings.HasPrefix(to, src)
}

// moveBlock is why a move or a copy to dest cannot start, or "".
func moveBlock(c *fileCtx, op view.FileOp, items []file.Entry, dest *fileCtx) string {
	switch {
	case dest == nil:
		return "Choose a folder."
	case dest.col.ReadOnly:
		return "The chosen storage is read-only."
	case op == view.FileMove && c.col.ReadOnly:
		return readOnlyReason
	case op == view.FileMove && dest.conn.ID == c.conn.ID && dest.col.Path == c.col.Path && slices.Equal(dest.segs, c.segs):
		return "These items are already here."
	}
	for _, e := range items {
		if intoItself(c, e, dest) {
			return "A folder cannot go inside itself."
		}
	}
	return ""
}

// opSheet says what an operation will do before it runs: the names, where they
// go, and — between storages — that files travel through Carrel (S-17.1).
func (s *Server) opSheet(r *http.Request, c *fileCtx, op view.FileOp, items []file.Entry, q url.Values) view.FileOpSheet {
	sheet := view.FileOpSheet{Op: op}
	names := make([]string, len(items))
	for i, e := range items {
		names[i] = e.Name
		sheet.Items = append(sheet.Items, text(e.Name))
	}
	run := url.Values{"op": {string(op)}, "item": names, "go": {"1"}}
	confirm := view.Action{ID: "file." + string(op) + ".confirm", Method: "POST", Enabled: true}
	switch op {
	case view.FileRename:
		// The new name is the form's field "name".
		confirm.Label = "Rename"
		confirm = writeAction(c, confirm)
	case view.FileDelete:
		confirm.Label, confirm.Danger = "Delete", true
		for _, e := range items {
			run.Add("etag", e.ETag)
		}
		confirm = writeAction(c, confirm)
		sheet.Warning = "Carrel keeps no index of what points at a file: it may be the attachment of a note or an event, and nothing checks that before it goes. Deleting cannot be undone."
	default:
		confirm.Label = "Move here"
		if op == view.FileCopy {
			confirm.Label = "Copy here"
		}
		dest, ok := s.destination(c, q)
		if !ok {
			dest = nil
		}
		sheet.Target = s.fileTree(r.Context(), c, dest)
		if reason := moveBlock(c, op, items, dest); reason != "" {
			confirm.Enabled, confirm.Reason = false, text(reason)
		} else {
			run.Set("to", s.route(dest.href("", dest.segs, true)))
		}
		if dest != nil && dest.conn.ID != c.conn.ID {
			sheet.Warning = "This goes to another storage: Carrel downloads each file and uploads it there, one at a time."
			if op == view.FileMove {
				sheet.Warning = "This goes to another storage, so nothing moves on a server: Carrel downloads each file and uploads it there, and deletes the original only once its copy is complete."
			}
		}
	}
	if confirm.Enabled {
		confirm.Href = s.postQuery(c.href("op", c.segs, true), run)
	}
	sheet.Actions = []view.Action{confirm, {ID: "file.op.cancel", Label: "Cancel", Href: s.route(c.href("", c.segs, true)), Method: "GET", Enabled: true}}
	return sheet
}

// fileTree is where a move or a copy can go (C-6.7): every live file
// collection of the catalog as a root, read-only ones shown and marked, and
// the folders on the way to the chosen one opened, one PROPFIND a level.
func (s *Server) fileTree(ctx context.Context, c *fileCtx, chosen *fileCtx) *view.FileTree {
	tree := &view.FileTree{}
	for _, col := range c.blob.Collections {
		conn, ok := c.blob.Connection(col.AccountID)
		if !ok || col.Gone || !col.Has(view.KindFile) {
			continue
		}
		node := view.FileTreeNode{
			ID:       s.route(fileHref(conn.ID, col.Path, "", nil, true)),
			Label:    collectionLabel(col),
			Stamp:    stamp(connLabel(conn), collectionLabel(col)),
			ReadOnly: col.ReadOnly,
		}
		if chosen != nil && chosen.conn.ID == conn.ID && chosen.col.Path == col.Path {
			node.Expanded, node.Children = true, s.treeLevel(ctx, chosen, nil)
		}
		tree.Nodes = append(tree.Nodes, node)
	}
	if chosen != nil {
		tree.Chosen = s.route(chosen.href("", chosen.segs, true))
	}
	return tree
}

func (s *Server) treeLevel(ctx context.Context, dest *fileCtx, at []string) []view.FileTreeNode {
	folder, err := dest.path(at, true)
	if err != nil {
		return nil
	}
	entries, err := dest.files.List(ctx, folder)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if e.Dir {
			dirs = append(dirs, e.Name)
		}
	}
	slices.SortFunc(dirs, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	if len(dirs) > s.cfg.FilesMaxEntries {
		dirs = dirs[:s.cfg.FilesMaxEntries]
	}
	var out []view.FileTreeNode
	for _, name := range dirs {
		segs := append(slices.Clone(at), name)
		node := view.FileTreeNode{ID: s.route(dest.href("", segs, true)), Label: name, ReadOnly: dest.col.ReadOnly}
		if len(dest.segs) > len(at) && dest.segs[len(at)] == name {
			node.Expanded, node.Children = true, s.treeLevel(ctx, dest, segs)
		}
		out = append(out, node)
	}
	return out
}

// runOp runs a confirmed operation. Rename, move and copy report in the queue,
// one row an item, a taken name as a question; delete reports in notices.
// The folders written into are read again.
func (s *Server) runOp(w http.ResponseWriter, r *http.Request, c *fileCtx, op view.FileOp, items []file.Entry, q url.Values) {
	ctx := r.Context()
	folder, _ := c.path(c.segs, true)
	defer forgetFolder(c.sess, c.conn.ID, folder)
	var notices []view.Notice
	switch op {
	case view.FileDelete:
		if c.col.ReadOnly {
			notices = append(notices, readOnlyNotice)
			break
		}
		etags := q["etag"]
		failed := &view.Diagnostic{Summary: "Not deleted"}
		done := 0
		for i, e := range items {
			etag := ""
			if i < len(etags) {
				etag = etags[i]
			}
			p, _ := c.path(append(slices.Clone(c.segs), e.Name), e.Dir)
			err := c.files.Delete(ctx, p, etag)
			switch {
			case err == nil:
				done++
			case dav.IsStatus(err, http.StatusPreconditionFailed):
				failed.Lines = append(failed.Lines, text(e.Name+" · changed since the folder was read; it stays"))
			default:
				failed.Lines = append(failed.Lines, text(e.Name+" · "+fileReason(err)+" "+err.Error()))
			}
		}
		if len(failed.Lines) > 0 {
			notices = append(notices, view.Notice{
				Role:    view.NoticeAlert,
				Title:   text(fmt.Sprintf("%d of %s deleted", done, plural(len(items), "item"))),
				Body:    "The rest are still there.",
				Details: failed,
			})
		}
	case view.FileRename:
		name := strings.TrimSpace(r.FormValue("name"))
		switch {
		case c.col.ReadOnly:
			notices = append(notices, readOnlyNotice)
		case file.CheckName(name) != nil:
			notices = append(notices, view.Notice{Role: view.NoticeAlert, Title: "Nothing was renamed", Body: "A name cannot be empty, “.” or “..”, and cannot hold “/” or “\\”."})
		case name != items[0].Name:
			if err := s.startItem(ctx, c, c.at(c.segs), "rename", items[0], name); err != nil {
				http.Error(w, "could not keep the transfer", http.StatusInternalServerError)
				return
			}
		}
	default:
		dest, ok := s.destination(c, q)
		if !ok {
			dest = nil
		}
		if reason := moveBlock(c, op, items, dest); reason != "" {
			notices = append(notices, view.Notice{Role: view.NoticeAlert, Title: "Nothing was done", Body: text(reason)})
			break
		}
		destFolder, _ := dest.path(dest.segs, true)
		defer forgetFolder(c.sess, dest.conn.ID, destFolder)
		for _, e := range items {
			if err := s.startItem(ctx, c, dest, string(op), e, e.Name); err != nil {
				http.Error(w, "could not keep the transfer", http.StatusInternalServerError)
				return
			}
		}
	}
	forgetFolder(c.sess, c.conn.ID, folder)
	if len(notices) == 0 && !WantsFragment(r) {
		s.redirect(w, r, c.href("", c.segs, true))
		return
	}
	s.writeFolder(w, r, c, c.segs, "", url.Values{}, TargetMain, notices...)
}

// startItem moves, copies or renames one member of c's folder to name in
// dest's folder, and keeps the row: done, failed, or a question when the name
// is taken there. The name is looked up first, for servers that do not honour
// If-None-Match; a name taken in between still answers 412 or 405.
func (s *Server) startItem(ctx context.Context, c, dest *fileCtx, kind string, e file.Entry, name string) error {
	t, err := newTransfer(kind, dest, name)
	if err != nil {
		return err
	}
	t.srcAccount, t.srcCol, t.src, t.dir, t.size = c.conn.ID, c.col.Path, append(slices.Clone(c.segs), e.Name), e.Dir, e.Size
	defer func() { c.sess.KeepTransfer(t) }()
	target, _ := dest.path(append(slices.Clone(dest.segs), name), false)
	_, err = dest.files.Stat(ctx, target)
	switch {
	case err == nil:
		t.state = view.OpIdle
	case file.NotFound(err):
		s.finish(t, s.place(ctx, c.sess, dest, t, name, false))
	default:
		t.state, t.note = view.OpFailed, sentenceEnd(fileReason(err))
	}
	return nil
}

// finish sets a row from the result of place. A name taken after it was
// looked up becomes the question it would have been.
func (s *Server) finish(t *transfer, err error) {
	switch {
	case err == nil:
		t.state = view.OpDone
	case t.kind != "upload" && (file.ErrOccupied(err) || dav.IsStatus(err, http.StatusMethodNotAllowed)):
		t.state = view.OpIdle
	default:
		t.state, t.note = view.OpFailed, sentenceEnd(fileReason(err))
		var copied *partialCopy
		if errors.As(err, &copied) {
			t.note = fmt.Sprintf("%s after %s; the original stays where it was", t.note, plural(copied.n, "entry"))
			if t.kind == "copy" {
				t.note = fmt.Sprintf("%s after %s; what arrived stays there", sentenceEnd(fileReason(err)), plural(copied.n, "entry"))
			}
		}
	}
}

// partialCopy is a folder copy that stopped part way.
type partialCopy struct {
	n   int
	err error
}

func (p *partialCopy) Error() string { return p.err.Error() }
func (p *partialCopy) Unwrap() error { return p.err }

// place puts what a row moves at name in dest's folder. Inside one connection a
// move or a rename is MOVE; a copy, and anything between connections, is a GET
// streamed into a PUT, a folder at a time (S-17.1). A move between connections
// deletes the original only after the whole copy arrived.
func (s *Server) place(ctx context.Context, sess *session.Session, dest *fileCtx, t *transfer, name string, overwrite bool) error {
	dst, err := dest.path(append(append(slices.Clone(t.folder), t.rel...), name), t.dir)
	if err != nil {
		return err
	}
	if t.kind == "upload" {
		f, err := os.Open(t.temp)
		if err != nil {
			return err
		}
		defer f.Close()
		fi, err := f.Stat()
		if err != nil {
			return err
		}
		body := dav.Sized(f, fi.Size())
		if mt := fileType(name, ""); mt != "" {
			body = dav.Typed(body, mt)
		}
		if overwrite {
			_, err = dest.files.UploadOverwrite(ctx, dst, body)
		} else {
			_, err = dest.files.UploadCreate(ctx, dst, body)
		}
		return err
	}
	src, ok := s.fileCollection(sess, t.srcAccount, t.srcCol)
	if !ok {
		return errors.New("the storage it came from is no longer in the catalog")
	}
	srcPath, err := src.path(t.src, t.dir)
	if err != nil {
		return err
	}
	if t.kind != "copy" && t.srcAccount == dest.conn.ID {
		return dest.files.Move(ctx, srcPath, dst, overwrite)
	}
	if t.dir {
		// Overwriting a folder removes what was there, so the copy is written
		// beside it first and the old folder goes only when the new one is
		// whole: a copy that fails half way leaves the destination as it was.
		into := dst
		if overwrite {
			if into, err = dest.path(append(append(slices.Clone(t.folder), t.rel...), name+".carrel-"+t.id), true); err != nil {
				return err
			}
		}
		if n, err := file.CopyTree(ctx, src.files, srcPath, dest.files, into, s.cfg.FilesMaxEntries); err != nil {
			if overwrite {
				_ = dest.files.Delete(ctx, into, "")
			}
			if n == 0 && dav.IsStatus(err, http.StatusMethodNotAllowed) {
				return err
			}
			return &partialCopy{n: n, err: err}
		}
		if overwrite {
			if err := dest.files.Delete(ctx, dst, ""); err != nil {
				_ = dest.files.Delete(ctx, into, "")
				return err
			}
			if err := dest.files.Move(ctx, into, dst, false); err != nil {
				return fmt.Errorf("the copy is whole under %s, but it could not take the old name: %w", name+".carrel-"+t.id, err)
			}
		}
	} else if err := file.Copy(ctx, src.files, srcPath, dest.files, dst, overwrite); err != nil {
		return err
	}
	if t.kind == "move" {
		return src.files.Delete(ctx, srcPath, "")
	}
	return nil
}

// answerTransfer answers the question of one row. The row must be of this
// session and wait at this address; anything else is 404. Skip lets the file
// go; Keep both takes the first free name with a number; Overwrite this one is
// the one write without a precondition (S-17.1).
func (s *Server) answerTransfer(w http.ResponseWriter, r *http.Request, c *fileCtx, marker, id string) {
	answer := r.URL.Query().Get("answer")
	got, found := c.sess.TakeTransfer(id)
	t, ok := got.(*transfer)
	if !found || !ok || !t.Waiting() || t.marker() != marker || t.account != c.conn.ID || t.col != c.col.Path ||
		!slices.Equal(t.folder, c.segs) || (answer != "skip" && answer != "both" && answer != "overwrite") {
		if found {
			c.sess.KeepTransfer(got)
		}
		s.notFound(w, r)
		return
	}
	next := *t
	next.temp = ""
	defer t.Discard()
	ctx := r.Context()
	switch answer {
	case "skip":
		next.state, next.note = view.OpCancelled, ""
	case "overwrite":
		s.finish(&next, s.place(ctx, c.sess, c, t, t.name, true))
		if next.Waiting() {
			next.state, next.note = view.OpFailed, "the server refused to overwrite it"
		}
	case "both":
		name, err := s.freeName(ctx, c, t.rel, t.name)
		if err == nil {
			next.name = name
			err = s.place(ctx, c.sess, c, t, name, false)
		}
		s.finish(&next, err)
		if next.Waiting() || (next.kind == "upload" && file.ErrOccupied(err)) {
			next.state, next.note = view.OpFailed, "the new name was taken as well; nothing was overwritten"
		}
	}
	c.sess.KeepTransfer(&next)
	folder, _ := c.path(c.segs, true)
	forgetFolder(c.sess, c.conn.ID, folder)
	if t.kind == "move" || t.kind == "rename" {
		if src, ok := s.fileCollection(c.sess, t.srcAccount, t.srcCol); ok {
			from, _ := src.path(t.src[:len(t.src)-1], true)
			forgetFolder(c.sess, src.conn.ID, from)
		}
	}
	if marker == "upload" {
		s.answerUpload(w, r, c)
		return
	}
	if !WantsFragment(r) {
		s.redirect(w, r, c.href("", c.segs, true))
		return
	}
	_ = s.render.Fragment(w, r, http.StatusOK, TargetQueue, s.transferQueue(c.sess), nil)
}

// freeName is the first name with a number the folder does not hold, read
// from one listing.
func (s *Server) freeName(ctx context.Context, c *fileCtx, rel []string, name string) (string, error) {
	folder, _ := c.path(append(slices.Clone(c.segs), rel...), true)
	entries, err := c.files.List(ctx, folder)
	if err != nil {
		return "", err
	}
	taken := map[string]bool{}
	for _, e := range entries {
		taken[e.Name] = true
	}
	for n := 2; ; n++ {
		if candidate := file.Suffixed(name, n); !taken[candidate] {
			return candidate, nil
		}
	}
}
