package web

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/file"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// The attachments of a new event, task or note (s-agenda-15; the lead's
// decision of task 106, as the photo of a new card, v0.2.133). Until Save
// there is no record and no version to write a file to, so the first Attach
// issues the UID the record will have, and the files wait under it in the
// session's slot of new attachments (DraftNewAttach) with what was
// typed: Attach and its arrow send the form itself (Action.Submits), and the
// form comes back with the typing and the files. Save creates the record
// under that UID with the files in it. Nothing is written before: a file
// that would go to the attachment folder (larger than the limit, S-20) is
// not taken — the record is saved first, then such a file is attached to
// it; a file of Files larger than the limit is linked where it lies, as on
// a record. Remove takes a file off the draft; a form left unsaved writes
// nothing. Every new form has a draft of its own (attachDraftsKept).

// attachDraft is the files of a new form under the UID its record will have.
type attachDraft struct {
	Account string
	Col     string
	UID     string
	Values  map[string][]string
	Files   []draftFile
}

// draftFile is one file waiting for a new record: inside it (Data), or a
// link to where a file of Files lies (Link, Size).
type draftFile struct {
	Name string
	Type string
	Data []byte
	Link string
	Size int64
}

// attachDraftsKept is how many new forms may hold files at once. Each form
// has its own draft, found by the UID its record will have (Б7 of the audit
// of 11.10.2026: there was one draft a session, and the first Attach of a
// second new form threw away the files of the first). Past this many the
// oldest goes: a session does not hold files without end (C-12).
const attachDraftsKept = 4

// lostFilesNotice: the form names files the session no longer holds — newer
// forms took their place, or the session is another. Nothing was saved;
// saving again saves the record without them (s-agenda-32; Typecase's
// task 256).
var lostFilesNotice = view.Notice{
	Role: view.NoticeAlert, Title: "Nothing was saved:",
	Body: "the files attached here are no longer kept — newer forms took their place. Attach them again, or save without them.",
}

// lostDraft is what a draft pushed out by newer forms held: the files' names,
// types and sizes, without them (s-agenda-32). Its Save names them once;
// past attachDraftsKept the oldest goes, as the drafts' own.
type lostDraft struct {
	Account string
	Col     string
	UID     string
	Files   []draftFile
}

func lostDrafts(c *collectionCtx) []lostDraft {
	var ls []lostDraft
	if data := c.sess.Draft(session.DraftLostAttach); data == nil || gob.NewDecoder(bytes.NewReader(data)).Decode(&ls) != nil {
		return nil
	}
	return ls
}

func keepLostDrafts(c *collectionCtx, ls []lostDraft) {
	if len(ls) > attachDraftsKept {
		ls = ls[len(ls)-attachDraftsKept:]
	}
	var buf bytes.Buffer
	if len(ls) == 0 || gob.NewEncoder(&buf).Encode(ls) != nil {
		c.sess.SetDraft(session.DraftLostAttach, nil)
		return
	}
	c.sess.SetDraft(session.DraftLostAttach, buf.Bytes())
}

// lose keeps of the drafts pushed out what their Save will name.
func lose(c *collectionCtx, gone []attachDraft) {
	ls := lostDrafts(c)
	for _, d := range gone {
		l := lostDraft{Account: d.Account, Col: d.Col, UID: d.UID}
		for _, f := range d.Files {
			size := f.Size
			if f.Data != nil {
				size = int64(len(f.Data))
			}
			l.Files = append(l.Files, draftFile{Name: f.Name, Type: f.Type, Size: size})
		}
		if len(l.Files) > 0 {
			ls = append(ls, l)
		}
	}
	keepLostDrafts(c, ls)
}

// takeLost is what the form of uid lost, taken once: the rows of its files,
// each «not kept» (RecordUnavailable) with a Remove that takes the row off
// the page, nothing else (s-agenda-32) — Save again writes without them.
func (s *Server) takeLost(c *collectionCtx, uid string) []view.AttachRow {
	ls := lostDrafts(c)
	var rows []view.AttachRow
	kept := ls[:0]
	for _, l := range ls {
		if l.Account != c.conn.ID || l.Col != c.col.Path || l.UID != uid {
			kept = append(kept, l)
			continue
		}
		for _, f := range l.Files {
			rows = append(rows, view.AttachRow{Name: text(f.Name), Type: text(fileType(f.Name, f.Type)), Size: formatSize(f.Size),
				State: view.RecordUnavailable, Actions: []view.Action{{ID: "attach.forget", Label: "Remove", Danger: true, Enabled: true}}})
		}
	}
	keepLostDrafts(c, kept)
	return rows
}

// attachDrafts are the drafts of the session's new forms, the oldest first.
// A file chosen in the tree that is gone when it is attached (s-files-17).
const (
	fileGoneTitle = "Nothing was attached:"
	fileGoneBody  = "the file was moved or deleted after the tree was read. Choose it again."
)

func attachDrafts(c *collectionCtx) []attachDraft {
	data := c.sess.Draft(session.DraftNewAttach)
	if data == nil {
		return nil
	}
	var ds []attachDraft
	if gob.NewDecoder(bytes.NewReader(data)).Decode(&ds) != nil {
		return nil
	}
	return ds
}

func keepAttachDrafts(c *collectionCtx, ds []attachDraft) {
	if len(ds) == 0 {
		c.sess.SetDraft(session.DraftNewAttach, nil)
		return
	}
	var buf bytes.Buffer
	if gob.NewEncoder(&buf).Encode(ds) == nil {
		c.sess.SetDraft(session.DraftNewAttach, buf.Bytes())
	}
}

func (d attachDraft) is(c *collectionCtx, uid string) bool {
	return d.Account == c.conn.ID && d.Col == c.col.Path && d.UID == uid
}

func loadAttachDraft(c *collectionCtx, uid string) (attachDraft, bool) {
	if uid == "" {
		return attachDraft{}, false
	}
	for _, d := range attachDrafts(c) {
		if d.is(c, uid) {
			return d, true
		}
	}
	return attachDraft{}, false
}

// saveAttachDraft keeps a form's draft in place of the one it had, or as the
// newest.
func saveAttachDraft(c *collectionCtx, d attachDraft) {
	var kept []attachDraft
	for _, had := range attachDrafts(c) {
		if !had.is(c, d.UID) {
			kept = append(kept, had)
		}
	}
	kept = append(kept, d)
	if len(kept) > attachDraftsKept {
		lose(c, kept[:len(kept)-attachDraftsKept])
		kept = kept[len(kept)-attachDraftsKept:]
	}
	keepAttachDrafts(c, kept)
}

func clearAttachDraft(c *collectionCtx, uid string) {
	var kept []attachDraft
	for _, had := range attachDrafts(c) {
		if !had.is(c, uid) {
			kept = append(kept, had)
		}
	}
	keepAttachDrafts(c, kept)
}

// inlineBytes is how much the draft carries inside the record.
func (d attachDraft) inlineBytes() int64 {
	var n int64
	for _, f := range d.Files {
		n += int64(len(f.Data))
	}
	return n
}

// newAttachStep is a step of a new form's attachments: its address, the
// form's own with op and the draft's UID.
func (s *Server) newAttachStep(c *collectionCtx, op, uid string, more url.Values) string {
	q := url.Values{"op": {op}}
	if uid != "" {
		q.Set("uid", uid)
	}
	for k, v := range more {
		q[k] = v
	}
	return s.postQuery(c.listPath()+"/new", q)
}

// newAttachAction is Attach of a new form: it sends the form itself with the
// file (Submits, File), its arrow the same and From Files, which sends the
// form and opens the tree of Files for this draft (s-agenda-15).
func (s *Server) newAttachAction(c *collectionCtx, uid string) view.Action {
	a := view.Action{ID: "attach.add", Label: "Attach", Icon: "i-clip", Href: s.newAttachStep(c, "attach", uid, nil), Method: "POST", Enabled: true,
		File: "file", Submits: true}
	here := a
	here.Label, here.Icon = "From this computer", "i-upload"
	a.Arrow = []view.MenuItem{
		{Action: here},
		{Action: view.Action{ID: "attach.files", Label: "From Files", Icon: "i-file", Href: s.newAttachStep(c, "pickfile", uid, nil), Method: "POST", Enabled: true, Submits: true}},
	}
	return a
}

// draftRows are the rows of the files waiting for a new record, each with
// Remove, which sends the form.
func (s *Server) draftRows(c *collectionCtx, d attachDraft) []view.AttachRow {
	var rows []view.AttachRow
	var folder *fileCtx
	blob := s.catalogOf(c.sess)
	for i, f := range d.Files {
		row := view.AttachRow{Name: text(f.Name), Type: text(fileType(f.Name, f.Type)), Inline: f.Data != nil, State: view.RecordOK}
		size := f.Size
		if f.Data != nil {
			size = int64(len(f.Data))
		} else {
			if folder == nil {
				folder = s.savedFolder(c.sess, blob)
			}
			row.Href = s.attachLinkHref(blob, f.Link)
			row.Place = attachPlace(blob, folder, f.Link)
		}
		if size >= 0 {
			row.Size = formatSize(size)
		}
		remove := view.Action{ID: "attach.remove", Label: "Remove", Icon: "i-trash", Method: "POST", Danger: true, Enabled: true, Submits: true,
			Href: s.newAttachStep(c, "detach", d.UID, url.Values{"n": {strconv.Itoa(i)}})}
		if f.Data == nil {
			remove.Label, remove.Danger = "Remove link, keep the file", false
		}
		row.Actions = []view.Action{remove}
		rows = append(rows, row)
	}
	return rows
}

// postNewAttach takes one step of a new form's attachments, named by op in
// the address. Each brings the form as typed; nothing is written to a
// server.
func (s *Server) postNewAttach(w http.ResponseWriter, r *http.Request, c *collectionCtx, op string) {
	uid := issuedUID(r.URL.Query().Get("uid"))
	d, held := loadAttachDraft(c, uid)
	var uploads []draftFile
	var over []string
	if op == "attach" {
		// The files kept already take their room inside the record first.
		f, large, err := readNewAttachUpload(r, attachLimit(c.col)-d.inlineBytes())
		if err != nil {
			s.writePage(w, r, http.StatusOK, s.editPage(c, editState{values: s.newFormValues(c), isNew: true, readAt: s.now(),
				notices: []view.Notice{{Role: view.NoticeAlert, Title: "The upload broke off", Body: "Nothing was attached. Send the file again."}}}))
			return
		}
		uploads, over = f, large
	} else if r.ParseForm() != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// The form came along when a button of it sent it (Submits); the
	// tree's Attach and the form's own steps without it keep what was kept.
	_, sent := r.Form["summary"]
	typed := s.submittedValues(c, r)
	if !held {
		// The first step issues the UID the record will be made under.
		var err error
		if uid, err = model.NewUID(); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		d = attachDraft{Account: c.conn.ID, Col: c.col.Path, UID: uid}
	}
	if sent {
		d.Values = typed
	}
	var notice *view.Notice
	switch op {
	case "attach":
		saveFirst := "Up to " + sizeText(attachLimit(c.col)) + " together the files of a new record go inside it. A larger file goes to the attachments folder, and only a record that exists has one to link: save, then attach."
		d.Files = append(d.Files, uploads...)
		switch {
		case len(uploads) == 0 && len(over) == 0:
			notice = &view.Notice{Role: view.NoticeAlert, Title: "No file was chosen", Body: "Choose the file to attach and send the form again."}
		case len(uploads) == 0 && len(over) == 1:
			notice = &view.Notice{Role: view.NoticeAlert, Title: "Nothing was attached: save the record first", Body: text(saveFirst)}
		case len(over) > 0:
			// The files that fit wait for Save, and those that did not are
			// named (the owner's decision of 03.11.2026).
			total := len(uploads) + len(over)
			notice = &view.Notice{Role: view.NoticeAlert, Title: text(fmt.Sprintf("%d of %s attached.", len(uploads), plural(total, "file"))),
				Body: text(strings.Join(over, ", ") + ": save the record first. " + saveFirst)}
		}
	case "detach":
		if n, err := strconv.Atoi(r.URL.Query().Get("n")); err == nil && n >= 0 && n < len(d.Files) {
			d.Files = append(d.Files[:n:n], d.Files[n+1:]...)
		}
	case "pickfile":
		saveAttachDraft(c, d)
		q := url.Values{"for": {"attach"}, "k": {c.spec.segment}, "a": {c.conn.ID}, "c": {c.col.Path}, "new": {d.UID}}
		s.redirect(w, r, "/app/files/picker?"+q.Encode())
		return
	case "attachfile":
		n, err := s.draftFileOfFiles(r, c, d)
		if err != nil {
			notice = err
		} else {
			d.Files = append(d.Files, n)
		}
	default:
		s.notFound(w, r)
		return
	}
	saveAttachDraft(c, d)
	if notice != nil {
		s.writePage(w, r, http.StatusOK, s.editPage(c, editState{values: d.Values, isNew: true, readAt: s.now(), formUID: d.UID,
			notices: []view.Notice{*notice}}))
		return
	}
	s.redirect(w, r, newFormAddress(c, d.UID))
}

// draftBytes is how much files carry inside the record.
func draftBytes(files []draftFile) int64 {
	var n int64
	for _, f := range files {
		n += int64(len(f.Data))
	}
	return n
}

// readNewAttachUpload reads the form a new form's Attach sent with its files:
// the fields before and after them, a form's worth at most, and every file
// (Б6: the first alone was kept) that fits within the limit with those kept
// before it; over names the files that did not — each was left unread, and
// a smaller one after it may still fit. The middleware read the fields in
// front of the files for _csrf (isFileUpload).
func readNewAttachUpload(r *http.Request, limit int64) ([]draftFile, []string, error) {
	up, err := uploadOf(r)
	if err != nil {
		return nil, nil, err
	}
	fields := url.Values{}
	for k, v := range up.fields {
		fields[k] = v
	}
	remain := int64(conf.MaxFormBodyBytes)
	var got []draftFile
	var over []string
	for part := up.next; ; part = nil {
		if part == nil {
			if part, err = up.mr.NextPart(); err == io.EOF {
				break
			} else if err != nil {
				return nil, nil, err
			}
		}
		if part.FileName() != "" {
			name := uploadName(part.FileName())
			room := max(limit-draftBytes(got), 0)
			data, err := io.ReadAll(io.LimitReader(part, room+1))
			if err != nil {
				return nil, nil, err
			}
			if int64(len(data)) > room {
				over = append(over, name)
				if _, err := io.Copy(io.Discard, part); err != nil {
					return nil, nil, err
				}
				continue
			}
			got = append(got, draftFile{Name: name, Type: fileType(name, part.Header.Get("Content-Type")), Data: data, Size: int64(len(data))})
			continue
		}
		data, err := io.ReadAll(io.LimitReader(part, remain+1))
		if err != nil {
			return nil, nil, err
		}
		if remain -= int64(len(data)); remain < 0 {
			return nil, nil, &http.MaxBytesError{Limit: conf.MaxFormBodyBytes}
		}
		fields.Add(part.FormName(), string(data))
	}
	for k, v := range r.URL.Query() {
		if _, ok := fields[k]; !ok {
			fields[k] = v
		}
	}
	r.Form, r.PostForm = fields, fields
	return got, over, nil
}

// draftFileOfFiles is a file of Files chosen for a new form (From Files):
// inside the record when it fits with the files already waiting, else a link
// to where it lies, as on a record (S-20).
func (s *Server) draftFileOfFiles(r *http.Request, c *collectionCtx, d attachDraft) (draftFile, *view.Notice) {
	notChosen := &view.Notice{Role: view.NoticeAlert, Title: "No file was chosen.", Body: "Choose the file to attach in Files and try again."}
	src, dir, chosen := s.fileAddress(c.sess, r.URL.Query().Get("file"))
	if !chosen || dir || src.col.Feed || len(src.segs) == 0 {
		return draftFile{}, notChosen
	}
	p, err := src.path(src.segs, false)
	if err != nil {
		return draftFile{}, notChosen
	}
	e, err := src.files.Stat(r.Context(), p)
	switch {
	case file.NotFound(err):
		return draftFile{}, &view.Notice{Role: view.NoticeAlert, Title: fileGoneTitle,
			Body: fileGoneBody}
	case err != nil:
		n := s.fileFailure(src, "Nothing was attached", "PROPFIND", p, err)
		return draftFile{}, &n
	case e.Dir:
		return draftFile{}, notChosen
	}
	name := src.segs[len(src.segs)-1]
	f := draftFile{Name: name, Type: fileType(name, e.Type), Link: fileURI(src.conn.Server, p), Size: e.Size}
	if limit := attachLimit(c.col); e.Size >= 0 && d.inlineBytes()+e.Size <= limit {
		body, _, err := src.files.Get(r.Context(), p, nil)
		if err != nil {
			n := s.fileFailure(src, "Nothing was attached", "GET", p, err)
			return draftFile{}, &n
		}
		data, err := io.ReadAll(io.LimitReader(body, limit+1))
		_ = body.Close()
		if err != nil {
			n := s.fileFailure(src, "Nothing was attached", "GET", p, err)
			return draftFile{}, &n
		}
		if d.inlineBytes()+int64(len(data)) <= limit {
			f.Data, f.Link, f.Size = data, "", int64(len(data))
		}
	}
	return f, nil
}

// applyDraft writes the files of a new form's draft into the record Save
// creates, one after another: inside, or a link.
func applyDraft(obj *model.Object, d attachDraft) error {
	for _, f := range d.Files {
		ops := cal.AttachLink(obj, f.Link, f.Name, fileType(f.Name, f.Type), f.Size)
		if f.Data != nil {
			ops = cal.AttachInline(obj, f.Name, fileType(f.Name, f.Type), f.Data)
		}
		patch, err := model.NewPatch(ops...)
		if err != nil {
			return err
		}
		if err := patch.Apply(obj); err != nil {
			return err
		}
	}
	return nil
}

// newDraftUID is the UID a new form of a calendar kind goes on with: the
// draft's, when the session holds one under ?uid=.
func newDraftUID(c *collectionCtx, uid string) (attachDraft, bool) {
	if c.cal == nil {
		return attachDraft{}, false
	}
	return loadAttachDraft(c, issuedUID(strings.TrimSpace(uid)))
}
