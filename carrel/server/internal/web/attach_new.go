package web

import (
	"bytes"
	"encoding/gob"
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
// session's one slot of new attachments (DraftNewAttach) with what was
// typed: Attach and its arrow send the form itself (Action.Submits), and the
// form comes back with the typing and the files. Save creates the record
// under that UID with the files in it. Nothing is written before: a file
// that would go to the attachment folder (larger than the limit, S-20) is
// not taken — the record is saved first, then such a file is attached to
// it; a file of Files larger than the limit is linked where it lies, as on
// a record. Remove takes a file off the draft; a form left unsaved writes
// nothing, and the next new form's first Attach replaces the draft.

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

func loadAttachDraft(c *collectionCtx, uid string) (attachDraft, bool) {
	if uid == "" {
		return attachDraft{}, false
	}
	data := c.sess.Draft(session.DraftNewAttach)
	if data == nil {
		return attachDraft{}, false
	}
	var d attachDraft
	if gob.NewDecoder(bytes.NewReader(data)).Decode(&d) != nil {
		return attachDraft{}, false
	}
	if d.Account != c.conn.ID || d.Col != c.col.Path || d.UID != uid {
		return attachDraft{}, false
	}
	return d, true
}

func saveAttachDraft(c *collectionCtx, d attachDraft) {
	var buf bytes.Buffer
	if gob.NewEncoder(&buf).Encode(d) == nil {
		c.sess.SetDraft(session.DraftNewAttach, buf.Bytes())
	}
}

func clearAttachDraft(c *collectionCtx, uid string) {
	if _, ok := loadAttachDraft(c, uid); ok {
		c.sess.SetDraft(session.DraftNewAttach, nil)
	}
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
	a := view.Action{ID: "attach.add", Label: "Attach", Icon: "i-up", Href: s.newAttachStep(c, "attach", uid, nil), Method: "POST", Enabled: true,
		File: "file", Submits: true}
	here := a
	here.Label = "From this computer"
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
	var upload *draftFile
	tooLarge := false
	if op == "attach" {
		f, large, err := readNewAttachUpload(r, attachLimit(c.col))
		if err != nil {
			s.writePage(w, r, http.StatusOK, s.editPage(c, editState{values: s.newFormValues(c), isNew: true, readAt: s.now(),
				notices: []view.Notice{{Role: view.NoticeAlert, Title: "The upload broke off", Body: "Nothing was attached. Send the file again."}}}))
			return
		}
		upload, tooLarge = f, large
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
		limit := attachLimit(c.col)
		switch {
		case upload == nil:
			notice = &view.Notice{Role: view.NoticeAlert, Title: "No file was chosen", Body: "Choose the file to attach and send the form again."}
		case tooLarge || d.inlineBytes()+int64(len(upload.Data)) > limit:
			notice = &view.Notice{Role: view.NoticeAlert, Title: "Nothing was attached: save the record first",
				Body: text("Up to " + sizeText(limit) + " together the files of a new record go inside it. A larger file goes to the attachments folder, and only a record that exists has one to link: save, then attach.")}
		default:
			d.Files = append(d.Files, *upload)
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

// readNewAttachUpload reads the form a new form's Attach sent with its file:
// the fields before and after the file, a form's worth at most, and the file
// itself up to the limit; large says it was over and was left unread. The
// middleware read the fields in front of the file for _csrf (isFileUpload).
func readNewAttachUpload(r *http.Request, limit int64) (*draftFile, bool, error) {
	up, err := uploadOf(r)
	if err != nil {
		return nil, false, err
	}
	fields := url.Values{}
	for k, v := range up.fields {
		fields[k] = v
	}
	remain := int64(conf.MaxFormBodyBytes)
	var got *draftFile
	large := false
	for part := up.next; ; part = nil {
		if part == nil {
			if part, err = up.mr.NextPart(); err == io.EOF {
				break
			} else if err != nil {
				return nil, false, err
			}
		}
		if part.FileName() != "" {
			if got != nil || large {
				if _, err := io.Copy(io.Discard, part); err != nil {
					return nil, false, err
				}
				continue
			}
			name := uploadName(part.FileName())
			data, err := io.ReadAll(io.LimitReader(part, limit+1))
			if err != nil {
				return nil, false, err
			}
			if int64(len(data)) > limit {
				large = true
				if _, err := io.Copy(io.Discard, part); err != nil {
					return nil, false, err
				}
				continue
			}
			got = &draftFile{Name: name, Type: fileType(name, part.Header.Get("Content-Type")), Data: data, Size: int64(len(data))}
			continue
		}
		data, err := io.ReadAll(io.LimitReader(part, remain+1))
		if err != nil {
			return nil, false, err
		}
		if remain -= int64(len(data)); remain < 0 {
			return nil, false, &http.MaxBytesError{Limit: conf.MaxFormBodyBytes}
		}
		fields.Add(part.FormName(), string(data))
	}
	for k, v := range r.URL.Query() {
		if _, ok := fields[k]; !ok {
			fields[k] = v
		}
	}
	r.Form, r.PostForm = fields, fields
	return got, large, nil
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
		return draftFile{}, &view.Notice{Role: view.NoticeAlert, Title: "Nothing was attached: the file is no longer there.",
			Body: "It was moved or deleted after the tree was read. Choose it again."}
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
