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
	"sync/atomic"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/provider/file"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/render"
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
	short string // why it failed in a few words, beside the name (s-files-24); empty — the note says it
	size  int64
	dir   bool

	// where it goes: a folder of a collection, the folders inside it a folder
	// upload adds (S-17.1), and the name
	account, col string
	folder       []string
	rel          []string
	name         string

	// was is the version of what stood at the name when the question about
	// it arose (its ETag; empty — the server gave none): «Replace» answers
	// about that version and no other (S-17.1).
	was string

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
	kinds := map[string]bool{}
	var total int64
	sized := false
	var reasons []string
	for _, row := range rows {
		t, ok := row.(*transfer)
		if !ok {
			continue
		}
		counts[t.state]++
		kinds[t.kind] = true
		tr := view.TransferRow{Name: text(t.label()), State: t.state}
		if !t.dir {
			tr.Size = formatSize(t.size)
			total, sized = total+t.size, true
		}
		if t.state == view.OpDone {
			tr.Percent = 100
		}
		if t.note != "" {
			reasons = append(reasons, t.label()+": "+t.note)
		}
		// A row refused says why beside its name (s-files-24): in its own
		// few words where it has them, else as the note says it.
		if t.state == view.OpFailed {
			if tr.Reason = text(t.short); tr.Reason == "" {
				tr.Reason = text(t.note)
			}
		}
		if t.Waiting() {
			answer := func(id, label, value string) view.Action {
				return view.Action{
					ID: id, Label: text(label), Method: "POST", Enabled: true,
					Href: s.postQuery(fileHref(t.account, t.col, t.marker(), t.folder, true), url.Values{"transfer": {t.id}, "answer": {value}}),
				}
			}
			// The frame's words and order (s-files-2): Replace overwrites on
			// purpose, the one write without a precondition (S-17.1).
			tr.Question = &view.NameClash{Name: text(t.label()), Options: []view.Action{
				answer("transfer.both", "Keep both", "both"),
				answer("transfer.overwrite", "Replace", "overwrite"),
				answer("transfer.skip", "Skip", "skip"),
			}}
		}
		q.Rows = append(q.Rows, tr)
	}
	// The head (s-files-2): while a row waits for an answer, which one the
	// queue stands at; when all are answered, how many were done.
	n := len(q.Rows)
	verb := [2]string{"Transferring", "Transferred"}
	if len(kinds) == 1 {
		for k := range kinds {
			verb = map[string][2]string{"upload": {"Uploading", "Uploaded"}, "move": {"Moving", "Moved"},
				"copy": {"Copying", "Copied"}, "rename": {"Renaming", "Renamed"}}[k]
		}
	}
	if waiting := counts[view.OpIdle]; waiting > 0 {
		q.Title = text(fmt.Sprintf("%s %d of %d", verb[0], n-waiting+1, n))
	} else {
		q.Title = text(fmt.Sprintf("%s %d of %d", verb[1], counts[view.OpDone], n))
	}
	if sized {
		q.Total = formatSize(total)
	}
	q.Note = text(strings.Join(reasons, " · "))
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
	remain int64           // what is left of a form's worth of fields (MaxFormBodyBytes)
}

// keep reads a part that is no file into the fields, within what is left of
// a form's worth of bytes: the fields in front of the first file, and the
// ones between files — a second field of files has the sizes of its own
// files in front of it, after the first field's files (Typecase's note 41:
// Files and Folder of one form). A field of files nothing was chosen in
// comes as such a part too, empty.
func (up *uploadBody) keep(part *multipart.Part) error {
	data, err := io.ReadAll(io.LimitReader(part, up.remain+1))
	if err != nil {
		return err
	}
	if up.remain -= int64(len(data)); up.remain < 0 {
		return &http.MaxBytesError{Limit: conf.MaxFormBodyBytes}
	}
	up.fields.Add(part.FormName(), string(data))
	return nil
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
	case len(seg) == 6 && seg[1] == "app" && seg[5] == "new" && r.URL.Query().Get("op") == "attach":
		// A new form's Attach sends the form with its file (attach_new.go).
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
	up := &uploadBody{fields: url.Values{}, mr: mr, remain: int64(conf.MaxFormBodyBytes)}
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
		if err := up.keep(part); err != nil {
			return nil, err
		}
	}
}

func uploadOf(r *http.Request) (*uploadBody, error) {
	if up, ok := r.Context().Value(uploadKey{}).(*uploadBody); ok {
		return up, nil
	}
	return readUploadFields(r)
}

// countingReader counts what was read of an upload's body. The count is
// asked for when the upload has ended, and an upload that failed may have
// ended while the transport is still reading the body it was given — so it
// is counted atomically.
type countingReader struct {
	r io.Reader
	n atomic.Int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n.Add(int64(n))
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
			// A field between the files: the sizes of the files after it.
			if err = up.keep(part); err != nil {
				break
			}
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
		// An upload from the menu's sheet remembers its folder for the next
		// one (s-notes-11: «Change it here and the next upload from this
		// menu goes to the new one»).
		if r.URL.Query().Get("menu") != "" {
			key, _ := c.sess.DataKey()
			_ = s.users.SetUploadFolder(c.sess.AccountID(), key, sealed.AttachmentFolder{AccountID: c.conn.ID, Path: c.col.Path, Folder: c.segs})
		}
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
	there, err := c.files.Stat(ctx, target)
	switch {
	case err == nil:
		t.was = there.ETag
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
		t.size = body.n.Load()
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
			t.short = "the server needs the size" // s-files-24
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
	if render.WantsFragment(r) {
		q := s.transferQueue(c.sess)
		if q == nil {
			q = &view.TransferQueue{}
		}
		for _, n := range notices {
			q.Note = text(strings.TrimPrefix(string(q.Note)+" · "+string(n.Title)+". "+string(n.Body), " · "))
		}
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetQueue, q, nil)
		return
	}
	if len(notices) == 0 {
		s.redirect(w, r, c.href("", c.segs, true))
		return
	}
	s.writeFolder(w, r, c, c.segs, "", url.Values{}, "", notices...)
}

// New folder (C-5.5): MKCOL of the name the form sends in the folder of the
// address. New folder of the bar and of the rail sends no name: the answer
// is the sheet that asks for it (s-files-23), and its form sends it. A
// refusal is the sheet again, with what was typed; a folder made, the folder.
func (s *Server) postFilesMkcol(w http.ResponseWriter, r *http.Request) {
	c, ok := s.openFiles(w, r, "mkcol")
	if !ok {
		return
	}
	if !c.dir {
		s.notFound(w, r)
		return
	}
	// A read-only storage makes nothing, and its New folder stands off: a
	// request all the same is told so on the folder.
	if c.col.ReadOnly {
		s.writeFolder(w, r, c, c.segs, "", url.Values{}, render.TargetMain, readOnlyNotice)
		return
	}
	name := strings.TrimSpace(r.PostFormValue("name"))
	var failure *view.Notice
	if _, sent := r.PostForm["name"]; sent {
		if failure = s.makeFolder(r.Context(), c, name); failure == nil {
			s.redirect(w, r, c.href("", c.segs, true))
			return
		}
	}
	sheet := s.newFolderSheet(c, name, failure)
	if render.WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, sheet, nil)
		return
	}
	s.writePage(w, r, http.StatusOK, sheet)
}

// newFolderSheet is the sheet New folder of a folder's head and of the
// rail's foot opens (s-files-23): where the folder will be made, the field
// of its name with typed in it, why the last try made nothing, Create —
// the form's button — and Cancel, the folder.
func (s *Server) newFolderSheet(c *fileCtx, typed string, failure *view.Notice) view.FileOpSheet {
	create := s.route(c.href("mkcol", c.segs, true))
	form := view.Form{Action: create, Method: "POST", Target: render.TargetSheet,
		Sets: []view.FieldSet{{Rows: []view.FormRow{csrfField(c.sess),
			{Name: "name", Label: "Name", Kind: view.InputText, Value: text(typed), Required: true}}}}}
	form = focused(form, "name")
	return view.FileOpSheet{
		Op:      view.FileNewFolder,
		Title:   "New folder",
		Subject: text("in " + strings.Join(append([]string{collectionLabel(c.col)}, c.segs...), " / ")),
		Failure: failure,
		Form:    &form,
		Actions: []view.Action{
			{ID: "folder.new.create", Label: "Create", Href: create, Method: "POST", Primary: true, Enabled: true, Submits: true},
			{ID: "folder.new.cancel", Label: "Cancel", Href: s.route(c.href("", c.segs, true)), Method: "GET", Enabled: true},
		},
	}
}

// makeFolder is MKCOL of name in the folder c is at: nil when the folder is
// made, else why not. New folder of the bar and New folder of a tree (the
// move sheet, the upload picker) are this one step.
func (s *Server) makeFolder(ctx context.Context, c *fileCtx, name string) *view.Notice {
	switch {
	case c.col.ReadOnly:
		n := readOnlyNotice
		return &n
	case file.CheckName(name) != nil:
		return &view.Notice{Role: view.NoticeAlert, Title: "Nothing was created", Body: "A folder name cannot be empty, “.” or “..”, and cannot hold “/” or “\\”."}
	}
	folder, _ := c.path(c.segs, true)
	p, _ := c.path(append(slices.Clone(c.segs), name), true)
	err := c.files.MkCol(ctx, p)
	forgetFolder(c.sess, c.conn.ID, folder)
	switch {
	case err == nil:
	// It is already here because this very request put it here: the
	// guard sent the MKCOL twice after a connection died before the
	// answer. The folder is made, and saying otherwise sends the
	// person looking for a name that is taken by their own click.
	case landedOnTheFirstTry(err):
	case dav.IsStatus(err, http.StatusMethodNotAllowed):
		// The name is taken (s-files-13): said in one line, naming the
		// folder it is taken in, and whether a file or a folder holds it.
		what := "folder"
		if entries, lerr := c.files.List(ctx, folder); lerr == nil {
			if i := slices.IndexFunc(entries, func(e file.Entry) bool { return e.Name == name }); i >= 0 && !entries[i].Dir {
				what = "file"
			}
		}
		in := collectionLabel(c.col)
		if len(c.segs) > 0 {
			in = c.segs[len(c.segs)-1]
		}
		return &view.Notice{Role: view.NoticeAlert, Body: text("There is a " + what + " called «" + name + "» in " + in + " already. Choose another name.")}
	case dav.IsStatus(err, http.StatusConflict):
		return &view.Notice{Role: view.NoticeAlert, Title: "This folder is no longer there", Body: "The server has no folder to create it in. Nothing was created."}
	default:
		n := s.fileFailure(c, "The folder was not created", "MKCOL", p, err)
		return &n
	}
	return nil
}

// Operations (C-5.5): rename, move, copy and delete over names of one folder,
// one address with the named operation. Without go=1 the answer is the sheet,
// which says what will happen before it does; with it, the operation runs.
// A GET is only ever the sheet — a node of the tree chooses by its address
// (FileTreeNode.Href) — and runs, makes and answers nothing, go=1 or not.

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
	if r.Method == http.MethodGet {
		q.Del("go")
		q.Del("mkcol")
		q.Del("transfer")
	}
	// The tree of the sheet chooses a folder by the address of a node, or
	// with the form field to; the confirmed operation names it in its
	// address.
	if to := r.PostFormValue("to"); to != "" {
		q.Set("to", to)
	}
	// «Open the folder» of a sheet without scripts (note 61 of Typecase):
	// the same sheet with the folder ticked chosen and open, nothing done.
	if r.Method == http.MethodPost && r.PostFormValue("open") != "" {
		q.Del("go")
	}
	if id := q.Get("transfer"); id != "" {
		s.answerTransfer(w, r, c, "op", id)
		return
	}
	op := view.FileOp(q.Get("op"))
	// The names are those of the address and those ticked in the selection
	// form (FilesPage.Select): its fields «item».
	if r.Method == http.MethodPost {
		for _, name := range r.PostForm["item"] {
			if !slices.Contains(q["item"], name) {
				q.Add("item", name)
			}
		}
	}
	names := q["item"]
	if op != view.FileRename && op != view.FileMove && op != view.FileCopy && op != view.FileDelete {
		s.notFound(w, r)
		return
	}
	// A button of the selection form pressed over the wrong number of ticks
	// is told so on the folder.
	if reason := ""; len(names) == 0 || op == view.FileRename && len(names) != 1 {
		if reason = "Rename takes one item."; len(names) == 0 {
			reason = "Tick the files and folders first."
		}
		s.writeFolder(w, r, c, c.segs, "", url.Values{}, render.TargetMain, view.Notice{Role: view.NoticeAlert, Title: "Nothing was done", Body: text(reason)})
		return
	}
	folder, _ := c.path(c.segs, true)
	entries, err := s.readFolder(r.Context(), c, folder, false)
	switch {
	case file.NotFound(err):
		s.notFound(w, r)
		return
	case err != nil:
		s.writeFolder(w, r, c, c.segs, "", url.Values{}, render.TargetMain)
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
		s.writeFolder(w, r, c, c.segs, "", url.Values{}, render.TargetMain, view.Notice{
			Role:  view.NoticeAlert,
			Title: "Nothing was done",
			Body:  text("No longer in this folder: " + strings.Join(missing, ", ") + "."),
		})
		return
	}
	if q.Get("go") == "" {
		// New folder of the tree: MKCOL into the chosen folder, then the
		// same sheet with the new one chosen (s-files-4).
		var failure *view.Notice
		made := ""
		name := strings.TrimSpace(r.PostFormValue("name"))
		if q.Get("mkcol") != "" && (op == view.FileMove || op == view.FileCopy) {
			q.Del("mkcol")
			if dest, ok := s.destination(c, q); ok {
				if failure = s.makeFolder(r.Context(), dest, name); failure == nil {
					made = s.route(dest.href("", append(slices.Clone(dest.segs), name), true))
					q.Set("to", made)
				} else {
					// A refusal keeps the line of the name open (s-files-13).
					q.Set("new", "1")
				}
			}
		}
		sheet := s.opSheet(r, c, op, items, q)
		sheet.Failure = failure
		if failure != nil {
			keepTyped(sheet.Target, name)
		}
		if sheet.Target != nil {
			s.markTree(sheet.Target, c.blob, made)
		}
		if render.WantsFragment(r) {
			_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, sheet, nil)
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
	dest, dir, ok := s.fileAddress(sess, href)
	if !ok || !dir {
		return nil, false
	}
	return dest, true
}

// fileAddress is the file or the folder a href of the files section names,
// in a live file collection of the catalog; dir says which.
func (s *Server) fileAddress(sess *session.Session, href string) (*fileCtx, bool, bool) {
	u, err := url.Parse(href)
	if err != nil || href == "" {
		return nil, false, false
	}
	escaped := u.EscapedPath()
	if s.basePath != "" {
		rest, ok := strings.CutPrefix(escaped, s.basePath)
		if !ok {
			return nil, false, false
		}
		escaped = rest
	}
	account, col, segs, dir, ok := parseFileAddress(escaped, "")
	if !ok {
		return nil, false, false
	}
	at, ok := s.fileCollection(sess, account, col)
	if !ok {
		return nil, false, false
	}
	at = at.at(segs)
	at.dir = dir
	return at, dir, true
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

var fileOpVerbs = map[view.FileOp]string{view.FileMove: "Move", view.FileCopy: "Copy", view.FileRename: "Rename", view.FileDelete: "Delete"}

// newFolder is New folder of a tree (s-files-4, s-notes-11) and the line it
// opens over the tree (s-files-13). New is a GET of the sheet at sheet with
// the query back and new=1, which answers it with the line open; the line
// is a POST of the field "name" to address with q, which makes the folder
// in dest, the chosen one, and answers the sheet with it chosen; its Cancel
// is the sheet with the line closed. open says the line stands now (new=1,
// or a refusal). New is closed with the reason where nothing can be made,
// and there is no line then.
func (s *Server) newFolder(tree *view.FileTree, dest *fileCtx, address string, q url.Values, sheet string, back url.Values, open bool) {
	a := &view.Action{ID: "files.tree.new", Label: "New folder", Icon: "i-plus", Method: "GET"}
	tree.New, tree.Name = a, nil
	switch {
	case dest == nil:
		a.Reason = "Choose a folder in the tree first."
		return
	case dest.col.ReadOnly:
		a.Reason = readOnlyReason
		return
	}
	q.Set("to", s.route(dest.href("", dest.segs, true)))
	create := s.postQuery(address, q)
	opened := cloneValues(back)
	opened.Set("new", "1")
	a.Enabled, a.Href = true, s.postQuery(sheet, opened)
	if !open {
		return
	}
	cancel := view.Action{ID: "files.tree.new.cancel", Label: "Cancel", Href: s.postQuery(sheet, back), Method: "GET", Enabled: true}
	tree.Name = &view.Form{Action: create, Method: "POST",
		Sets: []view.FieldSet{{Rows: []view.FormRow{csrfField(dest.sess),
			{Name: "name", Label: "Name of the new folder", Kind: view.InputText, Required: true}}}},
		Actions: []view.Action{{ID: "files.tree.new.create", Label: "Create", Href: create, Method: "POST", Primary: true, Enabled: true}, cancel}}
	*tree.Name = focused(*tree.Name, "name")
}

// markTree marks the attachment folder among the nodes of a tree and the
// one just made, fresh (s-notes-11: «attachment folder», «just created»).
// The fresh one stands first among the nodes beside it, whatever its name
// (s-notes-11: «2026-08» above «2026-07»); the others keep their order.
func (s *Server) markTree(tree *view.FileTree, blob sealed.Blob, fresh string) {
	attach := ""
	if a := blob.Attachments; a != nil {
		attach = s.route(fileHref(a.AccountID, a.Path, "", a.Folder, true))
	}
	var walk func(nodes []view.FileTreeNode)
	walk = func(nodes []view.FileTreeNode) {
		for i := range nodes {
			id := nodes[i].ID
			nodes[i].Attachments = !nodes[i].File && id == attach
			nodes[i].Fresh = fresh != "" && id == fresh
			walk(nodes[i].Children)
		}
		if i := slices.IndexFunc(nodes, func(n view.FileTreeNode) bool { return n.Fresh }); i > 0 {
			one := nodes[i]
			copy(nodes[1:i+1], nodes[:i])
			nodes[0] = one
		}
	}
	walk(tree.Nodes)
}

// wherePath is a chosen folder the way the head of a picker names it:
// «Documents / carrel / 2026-08».
func wherePath(c *fileCtx) view.Text {
	if c == nil {
		return ""
	}
	return text(strings.Join(append([]string{collectionLabel(c.col)}, c.segs...), " / "))
}

// keepTyped opens the line of New folder again with the name a refusal
// turned down (s-files-13).
func keepTyped(tree *view.FileTree, name string) {
	if tree == nil || tree.Name == nil {
		return
	}
	for i, r := range tree.Name.Sets[0].Rows {
		if r.Name == "name" {
			tree.Name.Sets[0].Rows[i].Value = text(name)
		}
	}
}

// opSheet says what an operation will do before it runs: the names, where they
// go, and — between storages — that files travel through Carrel (S-17.1).
func (s *Server) opSheet(r *http.Request, c *fileCtx, op view.FileOp, items []file.Entry, q url.Values) view.FileOpSheet {
	sheet := view.FileOpSheet{Op: op, Title: text(fileOpVerbs[op] + " " + plural(len(items), "item"))}
	if op == view.FileRename {
		sheet.Title = "Rename"
	}
	// A move and a copy say where from (s-files-4); a rename and a deletion
	// say where the item is (s-files-21, -22).
	sheet.Subject = text(strings.Join(append([]string{collectionLabel(c.col)}, c.segs...), " / "))
	if op == view.FileMove || op == view.FileCopy {
		sheet.Subject = "from " + sheet.Subject
	}
	names := make([]string, len(items))
	for i, e := range items {
		names[i] = e.Name
		sheet.Items = append(sheet.Items, text(e.Name))
	}
	run := url.Values{"op": {string(op)}, "item": names, "go": {"1"}}
	// Move, Copy and Rename are the sheet's primary button
	// (s-files-4, s-files-11, s-files-21); the deletion's is destructive
	// (s-files-22).
	confirm := view.Action{ID: "file." + string(op) + ".confirm", Method: "POST", Primary: op != view.FileDelete, Enabled: true}
	switch op {
	case view.FileRename:
		// The new name is the form's field "name".
		confirm.Label = "Rename"
		confirm = writeAction(c, confirm)
		sheet.Warning = line("The name of a file is its address: a note or an event that keeps a link to it, and a published link, will not find it under the new name.")
	case view.FileDelete:
		confirm.Label, confirm.Danger = "Delete", true
		for _, e := range items {
			run.Add("etag", e.ETag)
		}
		confirm = writeAction(c, confirm)
		sheet.Alert = deletionAlert(items, connLabel(c.conn))
		warning := "Carrel keeps no index of what points at these files. They may be attached to a note or an event — check before deleting."
		if notes := s.deletionNotes(c, items); len(notes) > 0 {
			warning += " " + strings.Join(notes, " ")
		}
		sheet.Warning = line(warning)
	default:
		// «Move» and «Copy», not «Move here» (the owner's word of
		// 09.10.2026; the frames still say «here» — Typecase's task 147).
		confirm.Label = "Move"
		if op == view.FileCopy {
			confirm.Label = "Copy"
		}
		dest, ok := s.destination(c, q)
		if !ok {
			dest = nil
		}
		// The head chooses the server (s-files-4): the one asked for by the
		// address, else the chosen folder's. Another server asked for,
		// nothing of it is chosen yet.
		server := s.treeServer(c.blob, dest, q.Get("server"), false)
		if dest != nil && dest.conn.ID != server {
			dest = nil
		}
		sheet.Servers = s.fileServers(c.sess, c.blob, server, false, func(id string) string {
			return s.postQuery(c.href("op", c.segs, true), url.Values{"op": {string(op)}, "item": names, "server": {id}})
		})
		sheet.Target = s.fileTree(r.Context(), c, dest, server)
		linkTree(sheet.Target, func(id string) string {
			return s.postQuery(c.href("op", c.segs, true), url.Values{"op": {string(op)}, "item": names, "to": {id}})
		})
		back := url.Values{"op": {string(op)}, "item": names}
		if dest != nil {
			back.Set("to", s.route(dest.href("", dest.segs, true)))
		} else {
			back.Set("server", server)
		}
		s.newFolder(sheet.Target, dest, c.href("op", c.segs, true), url.Values{"op": {string(op)}, "item": names, "mkcol": {"1"}},
			c.href("op", c.segs, true), back, q.Get("new") != "")
		// The sheet is a form (s-files-4): a node is a tick of its field
		// «to», and the button sends it. What cannot be done is switched
		// off (the owner's rule of 09.10.2026): the button stands off, with
		// why, while the folder chosen cannot be the destination — a node's
		// address (Href) is the sheet with that node chosen, and the button
		// as it is for it.
		sheet.Target.Field = "to"
		confirm.Submits = true
		if reason := moveBlock(c, op, items, dest); reason != "" {
			confirm.Enabled, confirm.Reason = false, text(reason)
		} else {
			run.Set("to", s.route(dest.href("", dest.segs, true)))
		}
		// The message stands on the sheet (s-files-4): what the operation
		// will cost, said before it starts.
		switch {
		case op == view.FileCopy:
			sheet.Warning = line("A copy travels through Carrel even inside one server: each file is downloaded and uploaded again, one at a time.")
		case dest != nil && dest.conn.ID != c.conn.ID:
			sheet.Warning = line("This goes to another storage, so nothing moves on a server: Carrel downloads each file and uploads it there, and deletes the original only once its copy is complete.")
		default:
			// MOVE monospaced (s-files-4).
			sheet.Warning = join("Inside one server this is ", monoRun("MOVE"), ": instant, no traffic. To another storage the file has to be downloaded and uploaded again — Carrel says so before it starts, not after.")
		}
	}
	address := s.postQuery(c.href("op", c.segs, true), run)
	if confirm.Enabled {
		confirm.Href = address
	}
	// The form stands while the button is off too: its address is the
	// operation's, the destination the field.
	if op == view.FileMove || op == view.FileCopy {
		sheet.Form = &view.Form{Action: address, Method: "POST", Target: render.TargetSheet,
			Sets: []view.FieldSet{{Rows: []view.FormRow{csrfField(c.sess)}}}}
	}
	// The new name is the form's field: without it Rename has nothing to
	// send (U-8). It starts as the name there now.
	if op == view.FileRename && confirm.Enabled {
		form := view.Form{Action: confirm.Href, Method: "POST", Sets: []view.FieldSet{{Rows: []view.FormRow{csrfField(c.sess),
			{Name: "name", Label: "Name", Kind: view.InputText, Value: text(items[0].Name), Required: true}}}}}
		form = focused(form, "name")
		sheet.Form = &form
	}
	sheet.Actions = []view.Action{confirm}
	if op == view.FileMove || op == view.FileCopy {
		// The second button of the form on a page without scripts
		// (s-files-4, -13): it sends the folder ticked and asks for the
		// sheet with it open. With a script a node opens by itself.
		sheet.Actions = append(sheet.Actions, view.Action{ID: "file.op.open", Label: "Open the folder", Method: "POST", Submits: true, Enabled: true})
	}
	sheet.Actions = append(sheet.Actions, view.Action{ID: "file.op.cancel", Label: "Cancel", Href: s.route(c.href("", c.segs, true)), Method: "GET", Enabled: true})
	return sheet
}

// deletionAlert says what a deletion takes, how much and from which server,
// and that there is no copy to bring it back from (s-files-22): files by
// their count, strong, and their size; a folder with everything in it — its
// size nobody read.
func deletionAlert(items []file.Entry, server string) view.Runs {
	const tail = " on %s. Carrel holds no copy — it reads and writes, it does not store — so there is nothing here to undo it with."
	folders := 0
	for _, e := range items {
		if e.Dir {
			folders++
		}
	}
	n := strconv.Itoa(len(items))
	switch {
	case folders == 0 && len(items) == 1:
		return join(strongRun(n), fmt.Sprintf(" file, %s, is deleted"+tail, sizeOf(items), server))
	case folders == 0:
		return join(strongRun(n), fmt.Sprintf(" files, %s, are deleted"+tail, sizeOf(items), server))
	case len(items) == 1:
		return line(fmt.Sprintf("The folder and everything in it are deleted"+tail, server))
	case folders == 1:
		return join(strongRun(n), fmt.Sprintf(" items, the folder among them with everything in it, are deleted"+tail, server))
	}
	return join(strongRun(n), fmt.Sprintf(" items, the folders among them with everything in them, are deleted"+tail, server))
}

// fileTree is where a move or a copy can go (C-6.7): every live file
// collection of the catalog as a root, read-only ones shown and marked, and
// the folders on the way to the chosen one opened, one PROPFIND a level.
func (s *Server) fileTree(ctx context.Context, c *fileCtx, chosen *fileCtx, server string) *view.FileTree {
	return s.tree(ctx, c.blob, chosen, "", false, 0, server)
}

// treeCollection says whether a collection of the catalog stands in a tree:
// a live file collection — and, in a tree a file is chosen from, not a
// subscribed folder.
func treeCollection(blob sealed.Blob, col sealed.Collection, files bool) bool {
	_, ok := blob.Connection(col.AccountID)
	return ok && !col.Gone && col.Has(view.KindFile) && !(files && col.Feed)
}

// treeServer is the server a sheet's tree shows (s-files-4: the pick of the
// head): the one the address asks for, when it has storages; else the
// chosen folder's; else the first that has any.
func (s *Server) treeServer(blob sealed.Blob, chosen *fileCtx, asked string, files bool) string {
	first := ""
	for _, col := range blob.Collections {
		if !treeCollection(blob, col, files) {
			continue
		}
		if col.AccountID == asked {
			return asked
		}
		if first == "" {
			first = col.AccountID
		}
	}
	if chosen != nil {
		return chosen.conn.ID
	}
	return first
}

// fileServers are the servers the head of a sheet chooses among: every
// connection with a storage in the tree, in the catalog's order, the one
// shown on, the others the sheet's address with them (href).
func (s *Server) fileServers(sess *session.Session, blob sealed.Blob, on string, files bool, href func(id string) string) []view.FileServer {
	var out []view.FileServer
	seen := map[string]bool{}
	for _, col := range blob.Collections {
		if !treeCollection(blob, col, files) || seen[col.AccountID] {
			continue
		}
		seen[col.AccountID] = true
		conn, _ := blob.Connection(col.AccountID)
		// Every server is a link to the same sheet over it, the open one
		// too (s-files-26, note 83 of Typecase); one whose last answer this
		// session was a refusal is off with why — unless it is the open one.
		srv := view.FileServer{Label: text(connLabel(conn)), On: conn.ID == on, Href: href(conn.ID)}
		if !srv.On && lastRefused(sess, blob, conn.ID) {
			srv.Href, srv.Reason = "", text(connLabel(conn)+" did not answer.")
		}
		out = append(out, srv)
	}
	return out
}

// lastRefused is whether the newest answer of a server this session read
// was a refusal: none of its collections answered after it.
func lastRefused(sess *session.Session, blob sealed.Blob, account string) bool {
	if sess == nil {
		return false
	}
	var answered, refused time.Time
	for _, col := range blob.CollectionsOf(account) {
		if r, ok := sess.ReadingOf(account, col.Path); ok {
			if r.At.After(answered) {
				answered = r.At
			}
			if r.Refused.After(refused) {
				refused = r.Refused
			}
		}
	}
	return refused.After(answered)
}

// sourceTree is the tree a file is chosen from (From Files of Attach): the
// same roots and folders, and the files of every folder opened after its
// folders. A read-only storage is a source like any other, so nothing in it
// is closed; a subscribed folder is no source (postAttachFile) and is not
// shown. name is the file chosen in the folder at, "" when none is. Each
// file says its size and, by limit — the most a record holds inside (S-20)
// — whether it would go inside the record or as a link (s-files-16).
func (s *Server) sourceTree(ctx context.Context, blob sealed.Blob, at *fileCtx, name string, limit int64, server string) *view.FileTree {
	return s.tree(ctx, blob, at, name, true, limit, server)
}

// tree is the storages of one server — the one the head of the sheet
// chooses (s-files-4) — as roots, which therefore do not name it.
func (s *Server) tree(ctx context.Context, blob sealed.Blob, chosen *fileCtx, name string, files bool, limit int64, server string) *view.FileTree {
	tree := &view.FileTree{}
	for _, col := range blob.Collections {
		conn, _ := blob.Connection(col.AccountID)
		if !treeCollection(blob, col, files) || conn.ID != server {
			continue
		}
		node := view.FileTreeNode{
			ID:       s.route(fileHref(conn.ID, col.Path, "", nil, true)),
			Label:    collectionLabel(col),
			ReadOnly: col.ReadOnly && !files,
		}
		if chosen != nil && chosen.conn.ID == conn.ID && chosen.col.Path == col.Path {
			node.Expanded, node.Children = true, s.treeLevel(ctx, chosen, nil, files, limit)
		}
		tree.Nodes = append(tree.Nodes, node)
	}
	switch {
	case chosen != nil && name != "":
		tree.Chosen = s.route(chosen.href("", append(slices.Clone(chosen.segs), name), false))
	case chosen != nil:
		tree.Chosen = s.route(chosen.href("", chosen.segs, true))
	}
	return tree
}

// linkTree gives every node a reader can choose the address of its sheet
// with that node chosen (FileTreeNode.Href): a link chooses without scripts.
// A node closed for this purpose has none.
func linkTree(tree *view.FileTree, href func(id string) string) {
	var walk func(nodes []view.FileTreeNode)
	walk = func(nodes []view.FileTreeNode) {
		for i := range nodes {
			if !nodes[i].ReadOnly {
				nodes[i].Href = href(nodes[i].ID)
			}
			walk(nodes[i].Children)
		}
	}
	walk(tree.Nodes)
}

func (s *Server) treeLevel(ctx context.Context, dest *fileCtx, at []string, files bool, limit int64) []view.FileTreeNode {
	folder, err := dest.path(at, true)
	if err != nil {
		return nil
	}
	entries, err := dest.files.List(ctx, folder)
	if err != nil {
		return nil
	}
	var dirs, names []string
	sizes := map[string]int64{}
	for _, e := range entries {
		switch {
		case e.Dir:
			dirs = append(dirs, e.Name)
		case files:
			names = append(names, e.Name)
			sizes[e.Name] = e.Size
		}
	}
	byName := func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) }
	slices.SortFunc(dirs, byName)
	slices.SortFunc(names, byName)
	if len(dirs) > s.cfg.FilesMaxEntries {
		dirs = dirs[:s.cfg.FilesMaxEntries]
	}
	if room := s.cfg.FilesMaxEntries - len(dirs); len(names) > room {
		names = names[:room]
	}
	var out []view.FileTreeNode
	for _, name := range dirs {
		segs := append(slices.Clone(at), name)
		node := view.FileTreeNode{ID: s.route(dest.href("", segs, true)), Label: name, ReadOnly: dest.col.ReadOnly && !files}
		if len(dest.segs) > len(at) && dest.segs[len(at)] == name {
			node.Expanded, node.Children = true, s.treeLevel(ctx, dest, segs, files, limit)
		}
		out = append(out, node)
	}
	for _, name := range names {
		node := view.FileTreeNode{ID: s.route(dest.href("", append(slices.Clone(at), name), false)), Label: name, File: true}
		// What Attach would do with it (s-files-16): up to the limit it goes
		// inside the record, a larger one as a link. A size the server did
		// not say is not guessed.
		if n := sizes[name]; n >= 0 {
			node.Size, node.Lands = formatSize(n), "inside"
			if n > limit {
				node.Lands = "link"
			}
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
	if len(notices) == 0 && !render.WantsFragment(r) {
		s.redirect(w, r, c.href("", c.segs, true))
		return
	}
	s.writeFolder(w, r, c, c.segs, "", url.Values{}, render.TargetMain, notices...)
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
	there, err := dest.files.Stat(ctx, target)
	switch {
	case err == nil:
		t.state, t.was = view.OpIdle, there.ETag
	case file.NotFound(err):
		s.finish(t, s.place(ctx, c.sess, dest, t, name, false))
		s.afterPlaced(c.sess, t)
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
		var kept folderKept
		if errors.As(err, &kept) {
			t.note = string(kept)
		}
		var copied *partialCopy
		if errors.As(err, &copied) {
			t.note = fmt.Sprintf("%s after %s; the original stays where it was", t.note, plural(copied.n, "entry"))
			if t.kind == "copy" {
				t.note = fmt.Sprintf("%s after %s; what arrived was taken back", sentenceEnd(fileReason(err)), plural(copied.n, "entry"))
			}
		}
	}
}

// folderKept is why a folder that was copied whole was not moved after all —
// Carrel's own reason, in its words, not a refusal of the server.
type folderKept string

func (n folderKept) Error() string { return string(n) }

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
			// The version asked about, and no other: a file written there
			// since is not this answer's to replace.
			_, err = dest.files.UploadReplace(ctx, dst, body, t.was)
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
		// A folder is written beside its place first, under a name of this
		// transfer, and takes the place only when it is whole: a copy that
		// fails half way leaves the destination as it was — nothing stands
		// under the real name that is half a folder —, and overwriting
		// removes what was there only then.
		into, err := dest.path(append(append(slices.Clone(t.folder), t.rel...), name+".carrel-"+t.id), true)
		if err != nil {
			return err
		}
		// A move deletes the original, so the original is looked at before
		// the copy and after it: what it held when it was copied is what
		// may go (Б14 of the audit of 11.10.2026).
		var before []file.TreeFile
		if t.kind == "move" {
			if before, err = src.files.Tree(ctx, srcPath, s.cfg.FilesMaxEntries); err != nil {
				return err
			}
		}
		if n, err := file.CopyTree(ctx, src.files, srcPath, dest.files, into, s.cfg.FilesMaxEntries); err != nil {
			_ = dest.files.Delete(context.WithoutCancel(ctx), into, "")
			if n == 0 && dav.IsStatus(err, http.StatusMethodNotAllowed) {
				return err
			}
			return &partialCopy{n: n, err: err}
		}
		if t.kind == "move" {
			if why := movedWhole(ctx, src.files, srcPath, before, dest.files, into, s.cfg.FilesMaxEntries); why != "" {
				_ = dest.files.Delete(context.WithoutCancel(ctx), into, "")
				return folderKept(why + ", so it stays where it was and the copy was taken back. Move it again")
			}
		}
		if overwrite {
			if err := dest.files.Delete(ctx, dst, ""); err != nil {
				_ = dest.files.Delete(context.WithoutCancel(ctx), into, "")
				return err
			}
		}
		if err := dest.files.Move(ctx, into, dst, false); err != nil {
			return fmt.Errorf("the copy is whole under %s, but it could not take the name: %w", name+".carrel-"+t.id, err)
		}
	} else if err := file.Copy(ctx, src.files, srcPath, dest.files, dst, overwrite); err != nil {
		return err
	}
	if t.kind == "move" {
		return src.files.Delete(ctx, srcPath, "")
	}
	return nil
}

// movedWhole looks at a folder copied in order to be moved before its
// original is deleted: the original holds what it held before the copy — no
// file added, gone or written since —, and the copy holds every file of it
// at its size. The answer is why the original must stay, for the caller to
// finish the sentence with what it did about it; empty — it may go.
func movedWhole(ctx context.Context, from *file.Provider, src string, before []file.TreeFile, to *file.Provider, copied string, max int) string {
	after, err := from.Tree(ctx, src, max)
	if err != nil {
		return "the folder could not be read again after it was copied"
	}
	if at := file.TreeDiffers(before, after, true); at != "" {
		return "the folder changed while it was copied (" + at + ")"
	}
	got, err := to.Tree(ctx, copied, max)
	if err != nil {
		return "the copy could not be read back"
	}
	if at := file.TreeDiffers(after, got, false); at != "" {
		return "the copy is not whole (" + at + ")"
	}
	return ""
}

// changedSince reports whether what stands at a waiting row's name is no
// longer the version the question was asked about. A row that knows no
// version — the server gave none, or the name was found taken only by the
// write — has nothing to compare, and the answer is taken as it is.
func (s *Server) changedSince(ctx context.Context, dest *fileCtx, t *transfer) bool {
	if t.was == "" {
		return false
	}
	dst, err := dest.path(append(append(slices.Clone(t.folder), t.rel...), t.name), t.dir)
	if err != nil {
		return false
	}
	now, err := dest.files.Stat(ctx, dst)
	if file.NotFound(err) {
		return true
	}
	return err == nil && now.ETag != "" && strings.Trim(now.ETag, `"`) != strings.Trim(t.was, `"`)
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
		if t.dir && t.kind != "upload" {
			// «Replace» of a folder merges (А11): nothing that stood there
			// goes, and each name taken there is a question of its own.
			if err := s.mergeFolder(ctx, c, t); err != nil {
				next.state, next.note = view.OpFailed, sentenceEnd(fileReason(err))
				break
			}
			next.state, next.note = view.OpDone, "merged with the folder there"
			break
		}
		// «Replace» was said of what stood there when the question was
		// asked. Whatever stands there now is looked at first: another
		// version, or nothing at all, is not what the person agreed to
		// lose. An upload says it to the server as well (If-Match), so the
		// write itself is refused when a version slips in between.
		const changed = "what stood there changed while the question waited; nothing was overwritten. Send it again to choose"
		if s.changedSince(ctx, c, t) {
			next.state, next.note = view.OpFailed, changed
			break
		}
		err := s.place(ctx, c.sess, c, t, t.name, true)
		s.finish(&next, err)
		switch {
		case t.kind == "upload" && t.was != "" && file.ErrOccupied(err):
			next.state, next.note = view.OpFailed, changed
		case next.Waiting():
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
	s.afterPlaced(c.sess, &next)
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
	if !render.WantsFragment(r) {
		s.redirect(w, r, c.href("", c.segs, true))
		return
	}
	_ = s.render.Fragment(w, r, http.StatusOK, render.TargetQueue, s.transferQueue(c.sess), nil)
}

// mergeFolder answers «Replace» of a folder moved or copied onto a folder of
// its name (А11 of the audit of 11.10.2026; the owner's decision of
// 12.10.2026: «the usual behaviour of a file manager: merge the contents; on
// a conflict — a question»). Each member of the folder goes into the one
// standing there the way a member of the open folder goes into another:
// under a free name at once, under a taken one as a question of its own in
// the queue — a folder answered «Replace» there merges in its turn. Nothing
// that stood there is deleted. A move takes the folder it came from away
// once nothing is left in it; what waits for an answer keeps it.
func (s *Server) mergeFolder(ctx context.Context, c *fileCtx, t *transfer) error {
	src, ok := s.fileCollection(c.sess, t.srcAccount, t.srcCol)
	if !ok {
		return errors.New("the storage it came from is no longer in the catalog")
	}
	srcPath, err := src.path(t.src, true)
	if err != nil {
		return err
	}
	entries, err := src.files.List(ctx, srcPath)
	if err != nil {
		return err
	}
	from, into := *src, *c
	from.segs = slices.Clone(t.src)
	into.segs = append(append(slices.Clone(t.folder), t.rel...), t.name)
	for _, e := range entries {
		if err := s.startItem(ctx, &from, &into, t.kind, e, e.Name); err != nil {
			return err
		}
	}
	if t.kind == "move" {
		if left, err := src.files.List(ctx, srcPath); err == nil && len(left) == 0 {
			_ = src.files.Delete(ctx, srcPath, "")
		}
		forgetFolder(c.sess, src.conn.ID, srcPath)
	}
	dst, _ := into.path(into.segs, true)
	forgetFolder(c.sess, c.conn.ID, dst)
	return nil
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
