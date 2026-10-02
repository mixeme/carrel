package web

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/photo"
	"carrel.dev/carrel/server/view"
)

// The photo of a new card (s-contacts-27, S-13). Until Save there is no card
// and no version to hold a draft to, so the upload issues the UID the card
// will have, and the draft lives under it (photoDraft.New) in the session's
// one photo slot. The steps are the card's own — upload, frame, Save photo,
// Cancel — on the form's address, /app/contacts/{account}/{col}/new/
// photo-preview?uid= (C-5.4), and the frame is served by the card's photo
// route (?preview=). Save photo writes nothing: it marks the frame chosen,
// and the form, /new?uid=, shows it and carries the UID in a hidden field.
// Save of the form creates the card under that UID with the photo in it
// (createRecord). Cancel of the frame, Remove, and a form left unsaved
// write nothing; a photo chosen for another card replaces the draft, and the
// form says so at Save.
//
// The photo buttons of the new form send the form itself (Action.Submits,
// the lead's decision on s-contacts-27): what was typed rides with the file
// and is kept with the draft (photoDraft.Values), and the form comes back
// with it — after Save photo, after Cancel of a first upload, after Remove,
// whose draft then holds the typing alone under the same UID.

// newPhotoAddress is the photo steps of a new form; with a UID, of its draft.
func newPhotoAddress(c *collectionCtx, uid string) string {
	at := c.listPath() + "/new/photo-preview"
	if uid != "" {
		at += "?uid=" + uid
	}
	return at
}

// newFormAddress is the full form of a new record; with a UID, the form of
// that photo draft.
func newFormAddress(c *collectionCtx, uid string) string {
	if uid == "" {
		return c.listPath() + "/new"
	}
	return c.listPath() + "/new?uid=" + uid
}

// issuedUID is a UID as model.NewUID issues it, 32 lowercase hex digits, or
// nothing: a UID from an address or a form is taken only in the shape the
// server gives, and it becomes the name of a file ({uid}.vcf).
func issuedUID(v string) string {
	if len(v) != 32 {
		return ""
	}
	for _, r := range v {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return ""
		}
	}
	return v
}

// newPhotoDraft is the draft of a new form under uid.
func newPhotoDraft(c *collectionCtx, uid string) (photoDraft, bool) {
	if uid == "" {
		return photoDraft{}, false
	}
	d, ok := loadPhotoDraft(c, uid)
	return d, ok && d.New
}

// chosenUID is the UID a new form goes on with: uid, when the session holds
// a frame chosen for it; nothing otherwise.
func chosenUID(c *collectionCtx, uid string) string {
	if d, ok := newPhotoDraft(c, issuedUID(uid)); ok && d.Chosen {
		return d.UID
	}
	return ""
}

// newPhotoBlock is the photo column of a new form: as the card's without a
// photo — Add and the formats — until a frame is chosen; then the frame, and
// Replace, Crop and Remove, as a card's kept photo.
func (s *Server) newPhotoBlock(c *collectionCtx, st editState, b view.PhotoBlock) view.PhotoBlock {
	b.Editable = true
	step := func(id, label, op string, danger bool) view.Action {
		fields := url.Values{"op": {op}}
		if st.formUID != "" {
			fields.Set("uid", st.formUID)
		}
		return view.Action{ID: id, Label: text(label), Method: "POST", Danger: danger, Enabled: true, Submits: true, Href: s.postQuery(newPhotoAddress(c, ""), fields)}
	}
	if d, ok := newPhotoDraft(c, st.formUID); ok && d.Chosen {
		b.Href = s.route(c.origin().photoPath(d.UID) + "?preview=" + strconv.Itoa(d.Step))
		b.Note = "EXIF orientation applied, metadata stripped."
		b.Actions = []view.Action{step("photo.replace", "Replace", "upload", false), step("photo.crop", "Crop", "crop", false), step("photo.remove", "Remove", "remove", true)}
		b.Actions[0].File = "photo"
		return b
	}
	b.Note = "JPEG, PNG, GIF or WebP."
	add := step("photo.add", "Add", "upload", false)
	add.File = "photo"
	// The typing alone kept under a UID (a photo cancelled or removed):
	// the next photo keeps that UID and the typing with it.
	if st.formUID == "" && st.typedUID != "" {
		add.Href = s.postQuery(newPhotoAddress(c, ""), url.Values{"op": {"upload"}, "uid": {st.typedUID}})
	}
	b.Actions = []view.Action{add}
	return b
}

// newFormValues are the fields of an empty new form: a rubric that starts
// with one row has it (s-contacts-27).
func (s *Server) newFormValues(c *collectionCtx) formValues {
	values := s.emptyValues(c)
	for _, f := range editFields[c.spec.kind] {
		if f.newRow != "" {
			values[f.name], values[f.name+".type"] = []string{"", ""}, []string{f.newRow, ""}
		}
	}
	return values
}

// newPhotoForm is the new form with what a photo step has to say: as it was
// typed, when the step brought the form, else empty.
func (s *Server) newPhotoForm(w http.ResponseWriter, r *http.Request, c *collectionCtx, uid string, typed formValues, notices ...view.Notice) {
	values := typed
	if values == nil {
		values = s.newFormValues(c)
	}
	s.writePage(w, r, http.StatusOK, s.editPage(c, editState{
		values: values, isNew: true, readAt: s.now(), formUID: chosenUID(c, uid), notices: notices,
	}))
}

// postedNewForm is what the new form sent with a photo step, when it sent
// itself (Action.Submits): the form has its Display name, empty or not. A
// step of the frame's own forms carries no field of the card.
func (s *Server) postedNewForm(c *collectionCtx, r *http.Request) (formValues, bool) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if r.MultipartForm == nil && r.ParseMultipartForm(conf.MaxFormBodyBytes) != nil {
			return nil, false
		}
	} else if r.ParseForm() != nil {
		return nil, false
	}
	if _, sent := r.Form["fn"]; !sent {
		return nil, false
	}
	return s.submittedValues(c, r), true
}

// keepTyping leaves the typing alone under uid: the photo is gone, the form
// comes back as it was typed. Nothing typed — nothing kept.
func keepTyping(c *collectionCtx, uid string, values map[string][]string) string {
	if values == nil {
		clearPhotoDraft(c, uid)
		return newFormAddress(c, "")
	}
	savePhotoDraft(c, photoDraft{Account: c.conn.ID, Col: c.col.Path, UID: uid, New: true, Values: values})
	return newFormAddress(c, uid)
}

// getNewPhotoPreview shows the frame of a new form's draft. Without one there
// is nothing to frame: back to the form.
func (s *Server) getNewPhotoPreview(w http.ResponseWriter, r *http.Request) {
	c, ok := s.openCollection(w, r, "contacts", false)
	if !ok {
		return
	}
	if c.col.ReadOnly {
		s.notFound(w, r)
		return
	}
	d, ok := newPhotoDraft(c, issuedUID(r.URL.Query().Get("uid")))
	if !ok || len(d.Data) == 0 {
		s.redirect(w, r, newFormAddress(c, ""))
		return
	}
	s.writePage(w, r, http.StatusOK, s.photoPreviewPage(c, d.UID, d))
}

// postNewPhotoPreview takes one step of a new form's photo, named by op in
// the address, as postPhotoPreview does for a card. Nothing here reads the
// body before op says it is an upload, and nothing here writes to a server.
func (s *Server) postNewPhotoPreview(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	c, ok := s.openCollection(w, r, "contacts", false)
	if !ok {
		return
	}
	if c.col.ReadOnly {
		s.notFound(w, r)
		return
	}
	uid := issuedUID(r.URL.Query().Get("uid"))
	op := r.URL.Query().Get("op")
	typed, sent := s.postedNewForm(c, r)
	if op == "upload" {
		s.newPhotoUpload(w, r, c, uid, typed)
		return
	}
	d, ok := newPhotoDraft(c, uid)
	if !ok {
		s.redirect(w, r, newFormAddress(c, ""))
		return
	}
	if sent {
		d.Values = typed
	}
	if len(d.Data) == 0 {
		// The typing alone: no photo to frame, Remove or Cancel.
		s.redirect(w, r, keepTyping(c, uid, d.Values))
		return
	}
	switch op {
	case "crop":
		// The chosen frame again, from where it was left.
		d.Step++
		savePhotoDraft(c, d)
		s.redirect(w, r, newPhotoAddress(c, uid))
	case "frame":
		d.Crop = submittedCrop(r, d.Crop)
		d.Step++
		savePhotoDraft(c, d)
		s.redirect(w, r, newPhotoAddress(c, uid))
	case "save":
		d.Crop = submittedCrop(r, d.Crop)
		if _, err := s.renderDraft(d); err != nil {
			clearPhotoDraft(c, uid)
			s.newPhotoForm(w, r, c, "", d.Values, s.photoRefusal(err))
			return
		}
		d.Chosen, d.Kept = true, d.Crop
		d.Step++
		savePhotoDraft(c, d)
		s.redirect(w, r, newFormAddress(c, uid))
	case "cancel":
		// A frame chosen before stays as it was chosen; a first upload
		// leaves nothing.
		if !d.Chosen {
			s.redirect(w, r, keepTyping(c, uid, d.Values))
			return
		}
		d.Crop = d.Kept
		d.Step++
		savePhotoDraft(c, d)
		s.redirect(w, r, newFormAddress(c, uid))
	case "remove":
		s.redirect(w, r, keepTyping(c, uid, d.Values))
	default:
		s.notFound(w, r)
	}
}

// newPhotoUpload takes a file for a new form. Replace keeps the UID of the
// draft it replaces; Add issues one, or keeps the one the typing alone is
// kept under. The caps and the refusals are the card's (S-13): a refused
// file leaves the form as it was — as it was typed, when it came along.
func (s *Server) newPhotoUpload(w http.ResponseWriter, r *http.Request, c *collectionCtx, uid string, typed formValues) {
	held, ok := newPhotoDraft(c, uid)
	if !ok {
		uid = ""
	}
	if typed == nil {
		typed = held.Values
	}
	file, err := uploadedPhoto(r)
	if err != nil {
		s.newPhotoForm(w, r, c, uid, typed, s.photoRefusal(err))
		return
	}
	src, err := photo.Load(file, s.cfg.PhotoMaxUploadBytes, s.cfg.PhotoMaxPixels)
	file.Close()
	if err != nil {
		s.newPhotoForm(w, r, c, uid, typed, s.photoRefusal(err))
		return
	}
	if uid == "" {
		if uid, err = model.NewUID(); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	savePhotoDraft(c, photoDraft{
		Account: c.conn.ID, Col: c.col.Path, UID: uid, New: true,
		Data: src.Data, MediaType: src.MediaType, Crop: photo.DefaultCrop, Step: 1, Values: typed,
	})
	s.redirect(w, r, newPhotoAddress(c, uid))
}

// newCardPhoto is the photo a new form saves with the card: the frame chosen
// under its UID (chosenUID), rendered as Save photo would write it into a
// card. No UID, no photo.
func (s *Server) newCardPhoto(c *collectionCtx, uid string) ([]byte, error) {
	d, held := newPhotoDraft(c, uid)
	if !held || !d.Chosen {
		return nil, nil
	}
	d.Crop = d.Kept
	return s.renderDraft(d)
}

// lostPhotoNotice: the form names a photo the session no longer holds — a
// photo chosen for another card took its place. Nothing was saved; saving
// again saves the card without it.
var lostPhotoNotice = view.Notice{
	Role: view.NoticeAlert, Title: "Nothing was saved",
	Body: "The photo of this form was replaced by a photo chosen for another card. Add it again, or save without it.",
}
