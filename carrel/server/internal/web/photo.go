package web

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/photo"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// Contact photos (S-13). One route serves them, /c/{account}/{col}/{uid}/photo:
// the photo as the card keeps it, its list thumbnail (?size=thumb), or the
// frame being chosen (?preview=). ?v= is the version of the card the link was
// made from, and the response's ETag is that version: an edit of the card
// changes the link and the tag. The response is private and always
// revalidated; its type is the one declared, never sniffed.
//
// A new photo goes through photo-preview: uploaded, framed on the server in
// steps, and written into the card as one patch with the If-Match of the
// version it was chosen for. The upload stays in the session draft until the
// frame is saved or cancelled.

const (
	photoFull  = "photo"
	photoThumb = "thumb"
)

const externalPhotoNote = "External photo link; editing is not available."

type photoKind int

const (
	noPhoto photoKind = iota
	keptPhoto
	linkedPhoto
)

func photoOf(obj *model.Object) photoKind {
	if len(obj.PropText("PHOTO")) == 0 {
		return noPhoto
	}
	if _, linked := card.PhotoURI(obj); linked {
		return linkedPhoto
	}
	return keptPhoto
}

func (o origin) photoPath(uid string) string {
	return "/c/" + o.conn.ID + "/" + url.PathEscape(o.col.Path) + "/" + url.PathEscape(uid) + "/photo"
}

// photoHref is the photo of a card as made from version etag.
func (s *Server) photoHref(o origin, uid, etag, size string) string {
	q := url.Values{}
	if size == photoThumb {
		q.Set("size", photoThumb)
	}
	if etag != "" {
		q.Set("v", etag)
	}
	if len(q) == 0 {
		return s.route(o.photoPath(uid))
	}
	return s.route(o.photoPath(uid) + "?" + q.Encode())
}

func photoAddress(c *collectionCtx, uid string) string {
	return c.recordPath(uid) + "/photo-preview"
}

// Both sizes live in the session's thumbnail cache, keyed by the version of
// the card: a card that changes needs no invalidation. An entry with no bytes
// is a photo kept in the card that cannot be shown. A card without a version
// is not cached at all.

func photoKey(col, uid, size string) string { return col + "\x00" + uid + "\x00" + size }

func cachedPhoto(sess *session.Session, o origin, uid, version, size string) (body []byte, mediaType string, ok bool) {
	if version == "" {
		return nil, "", false
	}
	data, ok := sess.Cache().GetThumb(o.conn.ID, photoKey(o.col.Path, uid, size), version)
	if !ok || len(data) == 0 {
		return nil, "", ok
	}
	if size == photoThumb {
		return data, "image/jpeg", true
	}
	mt, rest, found := bytes.Cut(data, []byte{0})
	if !found {
		return nil, "", true
	}
	return rest, string(mt), true
}

func putPhoto(sess *session.Session, o origin, uid, version, size string, body []byte, mediaType string) {
	if version == "" {
		return
	}
	data := body
	if size == photoFull && body != nil {
		data = append(append([]byte(mediaType), 0), body...)
	}
	sess.Cache().PutThumb(o.conn.ID, photoKey(o.col.Path, uid, size), version, data)
}

// photoFrom gives one size of a card's photo, made from the card as read. A
// photo the card keeps is checked (a declared type of the four, the bytes of
// that type, within the pixel cap) and one that fails is remembered as such
// for this version. A linked photo is fetched through the guard; a failed
// fetch is not remembered, the far server may answer next time.
func (s *Server) photoFrom(ctx context.Context, sess *session.Session, o origin, uid string, obj *model.Object, size string) ([]byte, string, bool) {
	if body, mt, ok := cachedPhoto(sess, o, uid, obj.ETag, size); ok {
		return body, mt, body != nil
	}
	var src photo.Source
	var err error
	switch photoOf(obj) {
	case keptPhoto:
		src, err = s.cardPhotoSource(obj)
	case linkedPhoto:
		link, _ := card.PhotoURI(obj)
		if src, err = photo.Fetch(ctx, s.guard.HTTPClient(), link, s.cfg.PhotoMaxPixels); err != nil {
			return nil, "", false
		}
	default:
		return nil, "", false
	}
	if err != nil {
		putPhoto(sess, o, uid, obj.ETag, photoFull, nil, "")
		putPhoto(sess, o, uid, obj.ETag, photoThumb, nil, "")
		return nil, "", false
	}
	putPhoto(sess, o, uid, obj.ETag, photoFull, src.Data, src.MediaType)
	if size == photoFull {
		return src.Data, src.MediaType, true
	}
	thumb, err := photo.Render(src, photo.DefaultCrop, s.cfg.PhotoThumbSide, s.cfg.PhotoJPEGQuality)
	if err != nil {
		thumb = nil
	}
	putPhoto(sess, o, uid, obj.ETag, photoThumb, thumb, "image/jpeg")
	return thumb, "image/jpeg", thumb != nil
}

// cardPhotoSource is the photo a card keeps, checked.
func (s *Server) cardPhotoSource(obj *model.Object) (photo.Source, error) {
	rc, declared, err := card.PhotoOpen(obj)
	if err != nil {
		return photo.Source{}, err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return photo.Source{}, err
	}
	return photo.Open(data, declared, s.cfg.PhotoMaxPixels)
}

// listAvatars are the thumbnails of the cards a list shows, made from the
// cards the list already read: a body a REPORT returned is not asked for again
// (S-11). A photo a card only links to is fetched when the browser asks for
// it, not while the list is built. A card whose photo cannot be shown keeps
// its initials.
func (s *Server) listAvatars(sess *session.Session, items []listed) []string {
	hrefs := make([]string, len(items))
	sem := make(chan struct{}, conf.FanoutParallelism)
	var wg sync.WaitGroup
	for i, it := range items {
		uid := card.UID(it.obj)
		switch photoOf(it.obj) {
		case linkedPhoto:
			hrefs[i] = s.photoHref(it.from, uid, it.obj.ETag, photoThumb)
		case keptPhoto:
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer func() { <-sem; wg.Done() }()
				// A photo is a far server's bytes through an image decoder;
				// a panic there loses one thumbnail, not the process.
				defer func() {
					if r := recover(); r != nil {
						slog.Warn("photo: thumbnail failed", "error", r)
					}
				}()
				if _, _, ok := s.photoFrom(context.Background(), sess, it.from, uid, it.obj, photoThumb); ok {
					hrefs[i] = s.photoHref(it.from, uid, it.obj.ETag, photoThumb)
				}
			}()
		}
	}
	wg.Wait()
	return hrefs
}

// cardPhoto is the thumbnail and the photo of a card on its own screens. A
// linked photo is not fetched to build the page.
func (s *Server) cardPhoto(c *collectionCtx, uid string, obj *model.Object) (thumb, full string) {
	o := c.origin()
	switch photoOf(obj) {
	case linkedPhoto:
		return s.photoHref(o, uid, obj.ETag, photoThumb), s.photoHref(o, uid, obj.ETag, photoFull)
	case keptPhoto:
		if _, _, ok := s.photoFrom(context.Background(), c.sess, o, uid, obj, photoThumb); ok {
			return s.photoHref(o, uid, obj.ETag, photoThumb), s.photoHref(o, uid, obj.ETag, photoFull)
		}
	}
	return "", ""
}

func photoETag(version, size string) string {
	if size == photoThumb {
		return `"` + version + `-thumb"`
	}
	return `"` + version + `"`
}

func notModified(r *http.Request, version, size string) bool {
	if version == "" {
		return false
	}
	want := photoETag(version, size)
	for _, tag := range strings.Split(r.Header.Get("If-None-Match"), ",") {
		if tag = strings.TrimPrefix(strings.TrimSpace(tag), "W/"); tag == want || tag == "*" {
			return true
		}
	}
	return false
}

func photoHeaders(w http.ResponseWriter, version, size string) {
	h := w.Header()
	h.Set("Cache-Control", "private, no-cache")
	h.Set("X-Content-Type-Options", "nosniff")
	if version != "" {
		h.Set("ETag", photoETag(version, size))
	}
}

func writePhoto(w http.ResponseWriter, body []byte, mediaType, version, size string) {
	photoHeaders(w, version, size)
	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func writeNotModified(w http.ResponseWriter, version, size string) {
	photoHeaders(w, version, size)
	w.WriteHeader(http.StatusNotModified)
}

// getContactPhoto streams a card's photo (C-5.4). Anything that cannot be
// shown — no photo, a type outside the four, a link the far server does not
// answer — is 404, and the page keeps the initials.
func (s *Server) getContactPhoto(w http.ResponseWriter, r *http.Request) {
	c, ok := s.openCollection(w, r, "contacts", true)
	if !ok {
		return
	}
	uid, q := r.PathValue("uid"), r.URL.Query()
	if q.Has("preview") {
		s.servePreview(w, r, c, uid)
		return
	}
	size := photoFull
	if q.Get("size") == photoThumb {
		size = photoThumb
	}
	o := c.origin()
	if v := q.Get("v"); v != "" {
		if notModified(r, v, size) {
			writeNotModified(w, v, size)
			return
		}
		if body, mt, ok := cachedPhoto(c.sess, o, uid, v, size); ok && body != nil {
			writePhoto(w, body, mt, v, size)
			return
		}
	}
	obj, err := c.find(r.Context(), uid)
	if err != nil {
		s.notFound(w, r)
		return
	}
	if notModified(r, obj.ETag, size) {
		writeNotModified(w, obj.ETag, size)
		return
	}
	body, mt, ok := s.photoFrom(r.Context(), c.sess, o, uid, obj, size)
	if !ok {
		s.notFound(w, r)
		return
	}
	writePhoto(w, body, mt, obj.ETag, size)
}

// servePreview is the frame of the draft as it would be written.
func (s *Server) servePreview(w http.ResponseWriter, r *http.Request, c *collectionCtx, uid string) {
	d, ok := loadPhotoDraft(c, uid)
	if !ok {
		s.notFound(w, r)
		return
	}
	body, err := s.renderDraft(d)
	if err != nil {
		s.notFound(w, r)
		return
	}
	writePhoto(w, body, "image/jpeg", "", photoFull)
}

func (s *Server) renderDraft(d photoDraft) ([]byte, error) {
	src, err := photo.Open(d.Data, d.MediaType, s.cfg.PhotoMaxPixels)
	if err != nil {
		return nil, err
	}
	return photo.Render(src, d.Crop, s.cfg.PhotoMaxSide, s.cfg.PhotoJPEGQuality)
}

// photoDraft is an upload waiting for its frame (DraftPendingPhoto): the card
// it is for and the version it was chosen at, the file as uploaded, and the
// frame so far. There is one slot: a photo chosen for another card replaces
// it.
type photoDraft struct {
	Account   string
	Col       string
	UID       string
	ETag      string
	Data      []byte
	MediaType string
	Crop      photo.Crop
	HadPhoto  bool   // the card keeps a photo of its own, so Remove photo is offered
	Name      string // whose photo it is: the card's name (s-contacts-5); empty for a new form
	Step      int    // the preview's address changes with every step
	// New: the draft of a new form (s-contacts-27), under the UID its
	// upload issued; there is no card and no version yet. Chosen: Save
	// photo was pressed, and the form shows and saves the frame Kept.
	New    bool
	Chosen bool
	Kept   photo.Crop
	// Values: what was typed into the new form when one of its photo
	// buttons sent it (Action.Submits) — the form comes back with it. A
	// draft of a new form with values and no Data is the typing alone: the
	// photo was cancelled or removed. A card's edit form sends itself the
	// same way (Б9 of the audit of 11.10.2026), and its typing waits here
	// for the form to be shown again (takeCardTyping)
	Values map[string][]string
	// TypedAt: the version of the card the Values were typed at. A photo
	// step that writes over that very version moves it to the version the
	// write left; a card changed elsewhere in between does not, and the form
	// comes back naming the old one — its Save meets the conflict screen
	TypedAt string
}

// cardTyping is what was typed into a card's edit form when one of its photo
// buttons sent it, and the version it was typed at.
type cardTyping struct {
	values formValues
	at     string
}

func (d photoDraft) typing() cardTyping { return cardTyping{values: d.Values, at: d.TypedAt} }

// loadFrame is the draft of this card when it holds a photo to frame: the
// typing alone is no frame.
func loadFrame(c *collectionCtx, uid string) (photoDraft, bool) {
	d, ok := loadPhotoDraft(c, uid)
	return d, ok && len(d.Data) > 0
}

// keepCardTyping ends the photo of a card's draft and leaves the typing
// alone under the card's UID, for its edit form. Nothing typed — nothing
// kept.
func keepCardTyping(c *collectionCtx, uid string, t cardTyping) {
	if t.values == nil {
		clearPhotoDraft(c, uid)
		return
	}
	savePhotoDraft(c, photoDraft{Account: c.conn.ID, Col: c.col.Path, UID: uid, Values: t.values, TypedAt: t.at})
}

// takeCardTyping is the typing kept for this card's edit form, taken: it is
// shown once, with the form a photo step comes back to. A form opened later
// is the card's. A frame left by Cancel — a link to the form (s-contacts-5)
// — is dropped with it.
func takeCardTyping(c *collectionCtx, uid string) (cardTyping, bool) {
	d, ok := loadPhotoDraft(c, uid)
	if ok && !d.New && len(d.Data) != 0 && d.Values == nil {
		clearPhotoDraft(c, uid)
	}
	if !ok || d.New || d.Values == nil {
		return cardTyping{}, false
	}
	clearPhotoDraft(c, uid)
	return d.typing(), true
}

// typedOver puts the typing over what the card holds: the form's values, and
// the version the form names.
func typedOver(t cardTyping, vals formValues, etag string) (formValues, string) {
	if t.values == nil {
		return vals, etag
	}
	if t.at != "" {
		etag = t.at
	}
	return t.values, etag
}

func loadPhotoDraft(c *collectionCtx, uid string) (photoDraft, bool) {
	data := c.sess.Draft(session.DraftPendingPhoto)
	if data == nil {
		return photoDraft{}, false
	}
	var d photoDraft
	if gob.NewDecoder(bytes.NewReader(data)).Decode(&d) != nil {
		return photoDraft{}, false
	}
	if d.Account != c.conn.ID || d.Col != c.col.Path || d.UID != uid {
		return photoDraft{}, false
	}
	return d, true
}

func savePhotoDraft(c *collectionCtx, d photoDraft) {
	var buf bytes.Buffer
	if gob.NewEncoder(&buf).Encode(d) == nil {
		c.sess.SetDraft(session.DraftPendingPhoto, buf.Bytes())
	}
}

// clearPhotoDraft drops the draft only when it is this card's.
func clearPhotoDraft(c *collectionCtx, uid string) {
	if _, ok := loadPhotoDraft(c, uid); ok {
		c.sess.SetDraft(session.DraftPendingPhoto, nil)
	}
}

// photoBlock is the photo column of the contact edit page: Add when the card
// keeps no photo; Replace, Crop and Remove when it keeps one; Replace and
// Remove when the one it keeps cannot be shown; nothing to do with a photo it
// only links to. Every step names the version of the edit form and sends the
// form itself (Action.Submits), as the new form's do: what was typed rides
// with the step and comes back with the form (Б9).
func (s *Server) photoBlock(c *collectionCtx, st editState) view.PhotoBlock {
	b := view.PhotoBlock{Initials: text(initials(first(st.values["fn"])))}
	if st.isNew {
		return s.newPhotoBlock(c, st, b)
	}
	if st.obj == nil {
		return b
	}
	o, uid, here := c.origin(), st.uid, photoAddress(c, st.uid)
	step := func(id, label, op string, danger bool) view.Action {
		a := view.Action{
			ID: id, Label: text(label), Method: "POST", Danger: danger, Enabled: true, Submits: true,
			Href: s.postQuery(here, url.Values{"op": {op}, "etag": {st.etag}}),
		}
		if op == "upload" {
			a.File = "photo" // the field the upload reads
		}
		return a
	}
	switch photoOf(st.obj) {
	case linkedPhoto:
		b.Href, b.Note = s.photoHref(o, uid, st.obj.ETag, photoFull), externalPhotoNote
	case keptPhoto:
		b.Editable = true
		if _, _, ok := s.photoFrom(context.Background(), c.sess, o, uid, st.obj, photoThumb); ok {
			b.Href = s.photoHref(o, uid, st.obj.ETag, photoFull)
			b.Actions = []view.Action{step("photo.replace", "Replace", "upload", false), step("photo.crop", "Crop", "crop", false), step("photo.remove", "Remove", "remove", true)}
			// What was done to it on the way in (S-13, s-contacts-3).
			b.Note = "EXIF orientation applied, metadata stripped."
			break
		}
		b.Note = "The photo in this card is not a JPEG, PNG, GIF or WebP image Carrel can show."
		b.Actions = []view.Action{step("photo.replace", "Replace", "upload", false), step("photo.remove", "Remove", "remove", true)}
	default:
		b.Editable, b.Note = true, "JPEG, PNG, GIF or WebP."
		b.Actions = []view.Action{step("photo.add", "Add", "upload", false)}
	}
	return b
}

// photoPreviewPage is the frame screen (s-contacts-5, Typecase's note 119):
// a form — the frame as the server drew it in hidden fields (zoom, x, y),
// which a script moves; the quarter turns a radio; Save photo its button;
// Remove photo the same form sent to its own address when the card keeps
// one; Cancel back to the form. Without a script the frame goes as drawn,
// and the turn and the save work.
func (s *Server) photoPreviewPage(c *collectionCtx, uid string, d photoDraft, notices ...view.Notice) view.PhotoPreviewPage {
	sh := s.buildShell(c.sess, "Photo")
	back := s.cardBack(c, uid)
	if d.New {
		back = s.backAction(c.spec, "Back to the form", newFormAddress(c, uid))
	}
	sh.Rail = s.recordRail(c, back)
	sh.Notices = append(sh.Notices, notices...)
	step := func(fields url.Values) string {
		if d.New {
			fields.Set("uid", uid)
			return s.postQuery(newPhotoAddress(c, ""), fields)
		}
		return s.postQuery(photoAddress(c, uid), fields)
	}
	// Whose photo, and that what the file says of itself does not go into
	// the card (s-contacts-5): the frame is written anew.
	who := d.Name
	if d.New {
		if who = strings.TrimSpace(first(d.Values["fn"])); who == "" {
			who = "New contact"
		}
	}
	// When the card's book was last read, as the card's edit says it; the
	// circle opens this page again (s-contacts-5).
	here := photoAddress(c, uid)
	if d.New {
		here = newPhotoAddress(c, "") + "?" + url.Values{"uid": {uid}}.Encode()
	}
	sh.Top.Refresh = s.recordRefresh(c, here)
	page := view.PhotoPreviewPage{
		Shell:    sh,
		Subtitle: text(who + " · metadata will be stripped"),
		Href:     s.route(c.origin().photoPath(uid) + "?preview=" + strconv.Itoa(d.Step)),
		Note:     "Drag to choose the frame.",
		// Without a script the frame is not dragged and the zoom not
		// moved: what is shown is saved (s-contacts-5; Typecase's note 223).
		NoScript: "Without scripts the frame and zoom stay as shown; rotating works.",
	}
	// What Save photo writes into the card, said before it is pressed
	// (s-contacts-5): the frame as it would be saved, measured.
	if body, err := s.renderDraft(d); err == nil {
		side := strconv.Itoa(s.cfg.PhotoMaxSide)
		page.Saved = text(side + " × " + side + " · JPEG · about " + string(formatSize(int64(len(body)))))
		page.SavedHint = "Kept inside the card, so it is made small."
	}
	turns := view.FormRow{Name: "rotate", Label: "Rotate", Kind: view.InputRadio, Value: text(strconv.Itoa(d.Crop.Rotate))}
	for _, deg := range []int{0, 90, 180, 270} {
		turns.Options = append(turns.Options, view.Option{Value: strconv.Itoa(deg), Label: text(strconv.Itoa(deg) + "°"), Selected: d.Crop.Rotate == deg})
	}
	num := func(n int) view.Text { return text(strconv.Itoa(n)) }
	page.Form = view.Form{Action: step(url.Values{"op": {"crop"}}), Method: "POST", Sets: []view.FieldSet{{Rows: []view.FormRow{
		csrfField(c.sess),
		{Name: "etag", Kind: view.InputHidden, Value: text(d.ETag)},
		{Name: "zoom", Kind: view.InputHidden, Value: num(d.Crop.Zoom)},
		{Name: "x", Kind: view.InputHidden, Value: num(d.Crop.OffsetX)},
		{Name: "y", Kind: view.InputHidden, Value: num(d.Crop.OffsetY)},
		turns,
	}}}}
	cancel := view.Action{ID: "photo.cancel", Label: "Cancel", Href: s.route(c.recordPath(uid) + "/edit"), Method: "GET", Enabled: true}
	if d.New {
		// A new form's typing travels with the draft: Cancel sends it.
		cancel = view.Action{ID: "photo.cancel", Label: "Cancel", Href: step(url.Values{"op": {"cancel"}}), Method: "POST", Enabled: true, Submits: true}
	}
	page.Actions = []view.Action{
		{ID: "photo.save", Label: "Save photo", Href: page.Form.Action, Method: "POST", Primary: true, Enabled: true, Submits: true},
		cancel,
	}
	if d.HadPhoto {
		page.Actions = append(page.Actions, view.Action{
			ID: "photo.remove", Label: "Remove photo", Method: "POST", Danger: true, Enabled: true, Submits: true,
			Href: step(url.Values{"op": {"remove"}}),
		})
	}
	return page
}

// framed is the frame page's own form sent (zoom is its field): Save photo
// to ?op=crop, as s-contacts-5 draws it; the edit form's Crop sends no frame.
func framed(r *http.Request) bool {
	return r.PostFormValue("zoom") != ""
}

// getPhotoPreview shows the frame of this card's draft. Without one there is
// nothing to frame: back to the edit page.
func (s *Server) getPhotoPreview(w http.ResponseWriter, r *http.Request) {
	c, ok := s.openCollection(w, r, "contacts", true)
	if !ok {
		return
	}
	if c.col.ReadOnly {
		s.notFound(w, r)
		return
	}
	uid := r.PathValue("uid")
	d, ok := loadFrame(c, uid)
	if !ok {
		s.redirect(w, r, c.recordPath(uid)+"/edit")
		return
	}
	s.writePage(w, r, http.StatusOK, s.photoPreviewPage(c, uid, d))
}

// postPhotoPreview takes one step, named by op in the address. A step of the
// edit form — upload, crop, remove — brings the form with it (Action.Submits);
// a step of the frame's own page brings the frame's numbers only.
func (s *Server) postPhotoPreview(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	c, ok := s.openCollection(w, r, "contacts", true)
	if !ok {
		return
	}
	if c.col.ReadOnly {
		s.notFound(w, r)
		return
	}
	uid := r.PathValue("uid")
	// The form as it was typed, at the version its step names.
	// A form that does not fit the body's cap is its photo, too large: the
	// form's own fields are a few lines.
	tooLarge := false
	typing := func() cardTyping {
		if r.MultipartForm == nil && strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			var over *http.MaxBytesError
			tooLarge = errors.As(parseMultipart(r), &over)
		}
		if typed, sent := s.postedNewForm(c, r); sent {
			return cardTyping{values: typed, at: strings.TrimSpace(r.URL.Query().Get("etag"))}
		}
		return cardTyping{}
	}
	switch r.URL.Query().Get("op") {
	case "upload":
		t := typing()
		if tooLarge {
			if obj, ok := s.photoCard(w, r, c, uid); ok {
				s.photoEditPage(w, r, c, uid, obj, t, s.photoRefusal(photo.ErrTooLarge))
			}
			return
		}
		s.photoUpload(w, r, c, uid, t)
	case "crop":
		t := typing()
		if tooLarge {
			if obj, ok := s.photoCard(w, r, c, uid); ok {
				s.photoEditPage(w, r, c, uid, obj, t, s.photoRefusal(photo.ErrTooLarge))
			}
			return
		}
		if framed(r) {
			s.photoSave(w, r, c, uid, false)
			return
		}
		// Replace waits for Save. If Crop brought that selection with the
		// edit, frame it rather than reopening the card's old photograph.
		if r.MultipartForm != nil {
			if files := r.MultipartForm.File["photo"]; len(files) > 0 && files[0].Filename != "" {
				s.photoUpload(w, r, c, uid, t)
				return
			}
		}
		s.photoCrop(w, r, c, uid, t)
	case "frame":
		d, ok := loadFrame(c, uid)
		if !ok {
			s.redirect(w, r, c.recordPath(uid)+"/edit")
			return
		}
		d.Crop = submittedCrop(r, d.Crop)
		d.Step++
		savePhotoDraft(c, d)
		s.redirect(w, r, photoAddress(c, uid))
	case "save":
		s.photoSave(w, r, c, uid, false)
	case "legacy":
		s.photoSave(w, r, c, uid, true)
	case "cancel":
		if d, ok := loadPhotoDraft(c, uid); ok {
			keepCardTyping(c, uid, d.typing())
		}
		s.redirect(w, r, c.recordPath(uid)+"/edit")
	case "remove":
		s.photoRemove(w, r, c, uid, typing())
	default:
		s.notFound(w, r)
	}
}

// photoCard reads the card a photo step is for. A card that is gone, and one
// whose photo is a link, has no photo to edit: 404, as for an address that
// does not exist. A server that does not answer is a notice on the card.
func (s *Server) photoCard(w http.ResponseWriter, r *http.Request, c *collectionCtx, uid string) (*model.Object, bool) {
	obj, err := c.find(r.Context(), uid)
	switch {
	case notFoundErr(err):
		s.notFound(w, r)
		return nil, false
	case err != nil:
		s.writePage(w, r, http.StatusOK, s.cardAnswer(r, c, uid, nil, s.unavailableNotice(c, c.recordPath(uid)+"/edit", err)))
		return nil, false
	case photoOf(obj) == linkedPhoto:
		s.notFound(w, r)
		return nil, false
	}
	return obj, true
}

// photoEditPage is the edit page of the card as the server holds it, with
// what the photo step has to say — and as it was typed, when the step brought
// the form.
func (s *Server) photoEditPage(w http.ResponseWriter, r *http.Request, c *collectionCtx, uid string, obj *model.Object, t cardTyping, notices ...view.Notice) {
	vals, alarms, err := s.currentValues(c, obj)
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.cardAnswer(r, c, uid, obj, append(notices, readNotice(obj.Path, err))...))
		return
	}
	s.rememberBody(c, obj)
	vals, etag := typedOver(t, vals, obj.ETag)
	s.writePage(w, r, http.StatusOK, s.editPage(c, editState{
		obj: obj, uid: uid, values: vals, alarms: alarms, etag: etag, readAt: time.Now(), notices: notices,
	}))
}

// photoEditAfter reads the card again for its edit page: a refused write
// leaves the object in hand patched.
func (s *Server) photoEditAfter(w http.ResponseWriter, r *http.Request, c *collectionCtx, uid string, t cardTyping, notice view.Notice) {
	obj, err := c.find(r.Context(), uid)
	if err != nil {
		keepCardTyping(c, uid, t)
		s.writePage(w, r, http.StatusOK, s.cardAnswer(r, c, uid, nil, notice, s.unavailableNotice(c, c.recordPath(uid)+"/edit", err)))
		return
	}
	s.photoEditPage(w, r, c, uid, obj, t, notice)
}

var noVersionNotice = view.Notice{
	Role: view.NoticeAlert, Title: "Nothing was saved",
	Body: "The form does not say which version it edits. Open the record again.",
}

var errNoPhoto = errors.New("no photo in the form")

// uploadedPhoto is the file part named photo. A body the middleware parsed
// for its _csrf is read from that form; one sent with X-CSRF-Token is read as
// it arrives.
func uploadedPhoto(r *http.Request) (io.ReadCloser, error) {
	if r.MultipartForm != nil {
		files := r.MultipartForm.File["photo"]
		if len(files) == 0 {
			return nil, errNoPhoto
		}
		return files[0].Open()
	}
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, errNoPhoto
	}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			return nil, errNoPhoto
		}
		if err != nil {
			return nil, err
		}
		if part.FormName() == "photo" && part.FileName() != "" {
			return part, nil
		}
	}
}

// photoRefusal says why an upload was not taken. Nothing was written, and
// no draft is left.
func (s *Server) photoRefusal(err error) view.Notice {
	n := view.Notice{Role: view.NoticeAlert}
	switch {
	case errors.Is(err, errNoPhoto):
		n.Title, n.Body = "No photo was chosen", "Choose a JPEG, PNG, GIF or WebP image and upload it again."
	case errors.Is(err, photo.ErrTooLarge):
		n.Title = "The photo is too large"
		n.Body = text("Nothing was saved. A photo can be up to " + string(formatSize(s.cfg.PhotoMaxUploadBytes)) + ".")
	case errors.Is(err, photo.ErrTooManyPixels):
		n.Title = "The photo has too many pixels"
		n.Body = text(fmt.Sprintf("Nothing was saved. A photo can have up to %g megapixels.", float64(s.cfg.PhotoMaxPixels)/1e6))
	case errors.Is(err, photo.ErrFormat), errors.Is(err, photo.ErrUnreadable):
		n.Title, n.Body = "This file is not a photo Carrel can use", "Nothing was saved. Upload a JPEG, PNG, GIF or WebP image."
	default:
		n.Title, n.Body = "The photo could not be read", "Nothing was saved. Try the upload again."
		n.Details = &view.Diagnostic{Summary: "Upload", Lines: []view.Text{text(err.Error())}}
	}
	return n
}

// photoUpload takes a file for the frame screen. The card is read first: a
// link cannot be replaced, and the version the page named must be there to
// hold the draft to. A refused file leaves the form as it was typed.
func (s *Server) photoUpload(w http.ResponseWriter, r *http.Request, c *collectionCtx, uid string, t cardTyping) {
	obj, ok := s.photoCard(w, r, c, uid)
	if !ok {
		return
	}
	etag := strings.TrimSpace(r.URL.Query().Get("etag"))
	if etag == "" {
		s.photoEditPage(w, r, c, uid, obj, t, noVersionNotice)
		return
	}
	file, err := uploadedPhoto(r)
	if err != nil {
		s.photoEditPage(w, r, c, uid, obj, t, s.photoRefusal(err))
		return
	}
	src, err := photo.Load(file, s.cfg.PhotoMaxUploadBytes, s.cfg.PhotoMaxPixels)
	file.Close()
	if err != nil {
		s.photoEditPage(w, r, c, uid, obj, t, s.photoRefusal(err))
		return
	}
	savePhotoDraft(c, photoDraft{
		Account: c.conn.ID, Col: c.col.Path, UID: uid, ETag: etag,
		Data: src.Data, MediaType: src.MediaType, Crop: photo.DefaultCrop,
		HadPhoto: photoOf(obj) == keptPhoto, Name: contactName(obj), Step: 1, Values: t.values, TypedAt: t.at,
	})
	s.redirect(w, r, photoAddress(c, uid))
}

// photoCrop frames again the photo the card keeps.
func (s *Server) photoCrop(w http.ResponseWriter, r *http.Request, c *collectionCtx, uid string, t cardTyping) {
	obj, ok := s.photoCard(w, r, c, uid)
	if !ok {
		return
	}
	etag := strings.TrimSpace(r.URL.Query().Get("etag"))
	switch {
	case photoOf(obj) != keptPhoto:
		keepCardTyping(c, uid, t)
		s.redirect(w, r, c.recordPath(uid)+"/edit")
		return
	case etag == "":
		s.photoEditPage(w, r, c, uid, obj, t, noVersionNotice)
		return
	}
	src, err := s.cardPhotoSource(obj)
	if err != nil {
		s.photoEditPage(w, r, c, uid, obj, t, view.Notice{
			Role: view.NoticeAlert, Title: "This photo cannot be framed",
			Body: "The photo in this card is not a JPEG, PNG, GIF or WebP image Carrel can read. Replace it or remove it.",
		})
		return
	}
	savePhotoDraft(c, photoDraft{
		Account: c.conn.ID, Col: c.col.Path, UID: uid, ETag: etag,
		Data: src.Data, MediaType: src.MediaType, Crop: photo.DefaultCrop, HadPhoto: true, Name: contactName(obj), Step: 1,
		Values: t.values, TypedAt: t.at,
	})
	s.redirect(w, r, photoAddress(c, uid))
}

// submittedCrop reads the frame fields. A turn's own value travels in its
// address and wins over the field the form sends along.
func submittedCrop(r *http.Request, crop photo.Crop) photo.Crop {
	read := func(name string, into *int) {
		v := r.URL.Query().Get(name)
		if v == "" {
			v = r.PostFormValue(name)
		}
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			*into = n
		}
	}
	read("zoom", &crop.Zoom)
	read("x", &crop.OffsetX)
	read("y", &crop.OffsetY)
	read("rotate", &crop.Rotate)
	return crop.Clamp()
}

// photoSave writes the frame into the card: PHOTO replaced by one patch, with
// the If-Match of the version the photo was chosen for (S-8). A card changed
// since keeps the draft, now at the new version, and says so: saving again is
// the person's choice, not an automatic one. older writes the older form,
// base64 with ENCODING, whatever the card's version: the answer to a loss
// report after a server cut the data: URI (S-13).
func (s *Server) photoSave(w http.ResponseWriter, r *http.Request, c *collectionCtx, uid string, older bool) {
	d, ok := loadFrame(c, uid)
	if !ok {
		s.redirect(w, r, c.recordPath(uid)+"/edit")
		return
	}
	d.Crop = submittedCrop(r, d.Crop)
	obj, ok := s.photoCard(w, r, c, uid)
	if !ok {
		return
	}
	jpeg, err := s.renderDraft(d)
	if err != nil {
		clearPhotoDraft(c, uid)
		s.photoEditPage(w, r, c, uid, obj, d.typing(), s.photoRefusal(err))
		return
	}
	current, had := obj.ETag, photoOf(obj) == keptPhoto
	// The write goes over the version the photo was chosen for. When that
	// is the version the form was typed at, the typing moves with it.
	own := d.Values != nil && d.TypedAt == d.ETag
	ops := card.PhotoOps(obj, jpeg)
	if older {
		ops = card.PhotoOpsBase64(obj, jpeg)
	}
	patch, err := model.NewPatch(ops...)
	if err == nil {
		err = patch.Apply(obj)
	}
	if err == nil {
		obj.ETag = d.ETag
		err = c.update(r.Context(), obj)
	}
	switch {
	case dav.IsStatus(err, http.StatusPreconditionFailed):
		d.ETag, d.HadPhoto = current, had
		d.Step++
		savePhotoDraft(c, d)
		s.writePage(w, r, http.StatusOK, s.photoPreviewPage(c, uid, d, view.Notice{
			Role:  view.NoticeAlert,
			Title: "Changed elsewhere, so the photo was not saved",
			Body:  "This contact changed after the photo was chosen. Check the card, then save the photo again if you still mean to.",
		}))
		return
	case err != nil:
		savePhotoDraft(c, d)
		s.writePage(w, r, http.StatusOK, s.photoPreviewPage(c, uid, d, s.writeFailure(c, "PUT", obj.Path, err)))
		return
	}
	// A server that cuts a data: URI (Radicale, through vobject) keeps no
	// image. The loss report offers the older form and names its risk; the
	// frame waits in the draft, at the version this write left
	// (docs/servers.md, SV-4).
	s.finishWriteWith(w, r, c, uid, obj, c.recordPath(uid)+"/edit", func(loss *model.Loss, got *model.Object) ([]view.Action, []view.Notice) {
		if own {
			d.TypedAt = ""
			if got != nil {
				d.TypedAt = got.ETag
			}
		}
		if older || got == nil || !lostProp(loss, "PHOTO") || !card.PhotoIsDataURI(obj) {
			keepCardTyping(c, uid, d.typing())
			return nil, nil
		}
		d.ETag, d.HadPhoto = got.ETag, photoOf(got) == keptPhoto
		d.Step++
		savePhotoDraft(c, d)
		return []view.Action{{
			ID: "photo.save.legacy", Label: "Save the photo in the older form",
			Href:   s.postQuery(photoAddress(c, uid), url.Values{"op": {"legacy"}, "etag": {got.ETag}}),
			Method: "POST", Enabled: true,
		}}, []view.Notice{{
			Role:  view.NoticeAccent,
			Title: "The server did not keep the photo",
			Body: "This server cuts a photo written the way vCard 4.0 asks. Carrel can save it in the older form instead: " +
				"this server keeps that, but apps that follow vCard 4.0 strictly may not show it.",
		}}
	})
}

// lostProp reports whether a loss names prop.
func lostProp(loss *model.Loss, prop string) bool {
	if loss == nil {
		return false
	}
	for _, p := range loss.Properties {
		if strings.EqualFold(p, prop) {
			return true
		}
	}
	return false
}

// photoRemove removes PHOTO from the card: the property, not an empty value
// (S-7), with the If-Match of the version the page showed.
func (s *Server) photoRemove(w http.ResponseWriter, r *http.Request, c *collectionCtx, uid string, t cardTyping) {
	obj, ok := s.photoCard(w, r, c, uid)
	if !ok {
		return
	}
	// Remove photo of the frame's page brings no form: the typing is the
	// draft's.
	if d, held := loadPhotoDraft(c, uid); held && t.values == nil {
		t = d.typing()
	}
	etag := strings.TrimSpace(r.URL.Query().Get("etag"))
	switch {
	case photoOf(obj) == noPhoto:
		keepCardTyping(c, uid, t)
		s.redirect(w, r, c.recordPath(uid)+"/edit")
		return
	case etag == "":
		s.photoEditPage(w, r, c, uid, obj, t, noVersionNotice)
		return
	}
	patch, err := model.NewPatch(model.RemoveProp{Prop: "PHOTO"})
	if err == nil {
		err = patch.Apply(obj)
	}
	if err == nil {
		obj.ETag = etag
		err = c.update(r.Context(), obj)
	}
	switch {
	case dav.IsStatus(err, http.StatusPreconditionFailed):
		s.photoEditAfter(w, r, c, uid, t, view.Notice{
			Role:  view.NoticeAlert,
			Title: "Changed elsewhere, so the photo was not removed",
			Body:  "This contact changed after you opened it. Check the current version below and remove the photo again if you still mean to.",
		})
		return
	case err != nil:
		s.photoEditAfter(w, r, c, uid, t, s.writeFailure(c, "PUT", obj.Path, err))
		return
	}
	own := t.values != nil && t.at == etag
	s.finishWriteWith(w, r, c, uid, obj, c.recordPath(uid)+"/edit", func(_ *model.Loss, got *model.Object) ([]view.Action, []view.Notice) {
		if own {
			t.at = ""
			if got != nil {
				t.at = got.ETag
			}
		}
		keepCardTyping(c, uid, t)
		return nil, nil
	})
}
