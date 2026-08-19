// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/config"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/model"
	"gitea.mixdep.ru/mix/carrel/internal/provider/calendar"
	"gitea.mixdep.ru/mix/carrel/internal/provider/files"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

// attachTarget is where §23.10 puts a file that is being attached: an account,
// the file collection inside it, and the folder inside that.
//
// It is asked for once, on the profile page, and then never again — the whole of
// the requirement is that attaching a screenshot to a note is one action. A
// folder that has to be chosen every time makes the feature dead on arrival.
type attachTarget struct {
	AccountID string
	// Root is the file collection from discovery; Folder is the absolute path of
	// the attachment folder, which is Root or something under it.
	Root   string
	Folder string
	// Rel is Folder relative to Root, which is what the files provider takes.
	Rel string
}

// attachmentTarget reads the configured folder and checks it still exists among
// the user's collections. A reference to a collection that has gone simply stops
// matching, the same way §15 treats a stale one.
func (s *Server) attachmentTarget(sess *session.Session) (attachTarget, bool) {
	if sess == nil {
		return attachTarget{}, false
	}
	views, err := s.Store.Views(sess.UserID, sess.DEK())
	if err != nil {
		return attachTarget{}, false
	}
	ref, ok := views.Default(account.ViewAttachments)
	if !ok {
		return attachTarget{}, false
	}
	accounts, err := s.Store.ListDAVAccounts(sess.UserID, sess.DEK())
	if err != nil {
		return attachTarget{}, false
	}
	folder := normalizeCollectionPath(ref.Collection)
	for _, acc := range accounts {
		if acc.ID != ref.AccountID || !acc.Enabled {
			continue
		}
		for _, col := range acc.Collections {
			if col.Kind != discovery.KindFiles || col.ReadOnly {
				continue
			}
			rel, relErr := files.Relative(col.Path, folder)
			if relErr != nil {
				continue
			}
			return attachTarget{
				AccountID: acc.ID, Root: files.NormalizeDir(col.Path),
				Folder: folder, Rel: rel,
			}, true
		}
	}
	return attachTarget{}, false
}

// attachmentSettings is the form on the profile page.
type attachmentSettings struct {
	Collections []sourceRow
	// Selected is the key of the chosen collection, and Folder the path inside
	// it. They are stored joined and shown apart, because a person picks a
	// collection from a list and types a folder name.
	Selected string
	Folder   string
	// Configured says a target is set and usable; Label is what it is called.
	Configured bool
	Label      string
	Suggestion string
}

func (s *Server) attachmentSettingsView(sess *session.Session) attachmentSettings {
	out := attachmentSettings{Suggestion: config.DefaultAttachmentFolder}
	rows := s.fileCollections(sess)
	for _, row := range rows {
		if !row.ReadOnly {
			out.Collections = append(out.Collections, row)
		}
	}
	if target, ok := s.attachmentTarget(sess); ok {
		out.Configured = true
		out.Selected = target.AccountID + "|" + target.Root
		out.Folder = target.Rel
		out.Label = target.Folder
		return out
	}
	if len(out.Collections) > 0 {
		out.Selected = out.Collections[0].Key()
		out.Folder = config.DefaultAttachmentFolder
	}
	return out
}

// appSaveAttachments stores the attachment folder and creates it when it is not
// there yet (§23.10 — named once, not then left as homework).
func (s *Server) appSaveAttachments(r *http.Request) (appView, error) {
	sess := SessionFrom(r)
	data := s.buildAppView(r)
	key := strings.TrimSpace(r.PostFormValue("attach_collection"))
	folder, err := files.CleanRelative(r.PostFormValue("attach_folder"))
	if err != nil {
		return data, fmt.Errorf("that folder name cannot be used")
	}
	if key == "" {
		return data, fmt.Errorf("choose a file collection for attachments")
	}
	var chosen sourceRow
	for _, row := range s.fileCollections(sess) {
		if row.Key() == key && !row.ReadOnly {
			chosen = row
			break
		}
	}
	if chosen.AccountID == "" {
		return data, fmt.Errorf("that file collection is not one you can write to")
	}
	p, _, err := s.filesProvider(sess, chosen.AccountID)
	if err != nil {
		return data, err
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if err := p.EnsureDir(ctx, chosen.Path, folder); err != nil {
		return data, err
	}
	absolute, err := files.Resolve(chosen.Path, folder)
	if err != nil {
		return data, err
	}
	err = s.Store.UpdateViews(sess.UserID, sess.DEK(), func(v *account.Views) {
		v.SetDefault(account.ViewAttachments, account.SourceRef{
			AccountID: chosen.AccountID, Collection: files.NormalizeDir(absolute),
		})
	})
	if err != nil {
		return data, err
	}
	return s.buildAppView(r), nil
}

// attachmentRow is one ATTACH as the card shows it.
type attachmentRow struct {
	Index int
	Name  string
	Size  string
	Type  string
	// OpenURL is the proxy of §23.10. It is empty for an attachment Carrel
	// cannot reach — one on a server this account has no file collection on —
	// and the row then says so rather than offering a link into nowhere.
	OpenURL string
	// Inline marks an attachment another client embedded as base64. §23.10 makes
	// those read-only: they are shown and never rewritten into a link.
	Inline bool
	// External marks a link that is not in one of this user's own collections,
	// so opening it is not offered (§24.2 — Carrel does not become a fetcher of
	// arbitrary URLs on request).
	External bool
}

// attachmentRows turns the properties into rows, resolving each link against the
// user's own file collections.
func (s *Server) attachmentRows(sess *session.Session, section icalSection, accountID, colEnc, uid string, list []model.Attachment) []attachmentRow {
	out := make([]attachmentRow, 0, len(list))
	for i, att := range list {
		row := attachmentRow{
			Index: i, Name: att.DisplayName(), Size: att.SizeLabel(),
			Type: att.FmtType, Inline: att.Inline,
		}
		switch {
		case att.Inline:
			// There is nothing to open: the data is in the object, and this
			// build does not decode it back out.
		case s.resolveAttachment(sess, att.URI) == nil:
			row.External = true
		default:
			row.OpenURL = s.Path("/a/" + section.Path + "/" + accountID + "/" + colEnc + "/" +
				urlPathEscape(uid) + "/" + strconv.Itoa(i))
		}
		out = append(out, row)
	}
	return out
}

// attachmentLocation is a link resolved to a collection the user has.
type attachmentLocation struct {
	AccountID string
	Root      string
	Rel       string
	Path      string
}

// resolveAttachment finds which of the user's file collections an ATTACH URI
// points into, and returns nil when it points somewhere else.
//
// This is what keeps the proxy of §23.10 from being an open one. The alternative
// — taking a URL from the object and fetching it because the SSRF guard would
// probably say no to the bad ones — makes Carrel a fetcher of whatever anyone can
// write into somebody's calendar. A link is opened because it is in a collection
// this account holds credentials for, or it is not opened at all.
func (s *Server) resolveAttachment(sess *session.Session, uri string) *attachmentLocation {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return nil
	}
	parsed, err := url.Parse(uri)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil
	}
	accounts, err := s.Store.ListDAVAccounts(sess.UserID, sess.DEK())
	if err != nil {
		return nil
	}
	for _, acc := range accounts {
		if !acc.Enabled {
			continue
		}
		base, err := url.Parse(acc.BaseURL)
		if err != nil || !sameOrigin(base, parsed) {
			continue
		}
		for _, col := range acc.Collections {
			if col.Kind != discovery.KindFiles {
				continue
			}
			rel, relErr := files.Relative(col.Path, decodedPath(parsed))
			if relErr != nil || rel == "" {
				continue
			}
			return &attachmentLocation{
				AccountID: acc.ID, Root: files.NormalizeDir(col.Path),
				Rel: rel, Path: decodedPath(parsed),
			}
		}
	}
	return nil
}

func decodedPath(u *url.URL) string {
	if u.Path != "" {
		return u.Path
	}
	return "/"
}

// sameOrigin compares scheme, host and port. A default port written out
// explicitly is the same origin as one left off, and servers write both.
func sameOrigin(a, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	if !strings.EqualFold(a.Scheme, b.Scheme) {
		return false
	}
	return strings.EqualFold(withPort(a), withPort(b))
}

func withPort(u *url.URL) string {
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		switch strings.ToLower(u.Scheme) {
		case "https":
			port = "443"
		case "http":
			port = "80"
		}
	}
	return host + ":" + port
}

// AttachmentOpen streams one attachment of one object.
//
// The URI is not taken from the request: the object is read, the attachment at
// that index is looked at, and the link is resolved against the collections this
// user actually has. Handing the browser somebody else's address, or fetching
// whatever a URL parameter said, are the two things §23.10 rules out.
func (s *Server) AttachmentOpen(w http.ResponseWriter, r *http.Request) {
	accountID, colEnc, uid := r.PathValue("account"), r.PathValue("col"), r.PathValue("uid")
	index, err := strconv.Atoi(r.PathValue("index"))
	collection, decodeErr := DecodeCollectionPath(colEnc)
	_, sectionOK := attachSection(r.PathValue("section"))
	if err != nil || decodeErr != nil || uid == "" || index < 0 || !sectionOK {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	p, acc, err := s.calendarProvider(sess, accountID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	col, err := findCalendar(acc, collection)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	obj, err := p.Get(ctx, normalizeCollectionPath(col.Path), calendarObjectPath(col.Path, uid))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	list := obj.Attachments()
	if index >= len(list) {
		http.NotFound(w, r)
		return
	}
	att := list[index]
	if att.Inline {
		http.Error(w, "that attachment is stored inside the entry and is not offered as a file", http.StatusNotImplemented)
		return
	}
	location := s.resolveAttachment(sess, att.URI)
	if location == nil {
		http.Error(w, "that attachment is on a server Carrel has no account for", http.StatusForbidden)
		return
	}
	fp, _, err := s.filesProvider(sess, location.AccountID)
	if err != nil {
		http.Error(w, userFacingDAVError(err), http.StatusBadGateway)
		return
	}
	streamCtx, streamCancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer streamCancel()
	rng := parseSingleRange(r.Header.Get("Range"))
	download, err := fp.Open(streamCtx, location.Root, location.Rel, rng)
	if err != nil {
		// §17: a file server that is down is a missing attachment, not a broken
		// entry. The card already rendered; this is only the click on it.
		http.Error(w, userFacingDAVError(err), downloadStatus(err))
		return
	}
	defer download.Body.Close()
	name := att.DisplayName()
	if name == "" {
		name = path.Base(location.Rel)
	}
	s.serveStream(w, r, download, name, rng != nil)
}

// attachSection maps a URL segment onto the section it names.
func attachSection(name string) (icalSection, bool) {
	switch name {
	case sectionNotes.Path:
		return sectionNotes, true
	case sectionCalendar.Path:
		return sectionCalendar, true
	case sectionTasks.Path:
		return sectionTasks, true
	}
	return icalSection{}, false
}

// attachUpload puts a file on the WebDAV server and adds an ATTACH to an object.
//
// The order matters: the file goes up first and the property is written second,
// so a failed upload leaves the entry untouched. The reverse would leave a link
// to a file that is not there.
func (s *Server) attachUpload(w http.ResponseWriter, r *http.Request, section icalSection, accountID, colEnc, uid string) {
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil || uid == "" {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	back := s.objectURL(section, accountID, colEnc, uid)
	target, ok := s.attachmentTarget(sess)
	if !ok {
		s.redirectNotice(w, r, back, "Choose a folder for attachments on your profile page first.")
		return
	}
	p, acc, err := s.calendarProvider(sess, accountID)
	if err != nil {
		s.redirectNotice(w, r, back, userFacingDAVError(err))
		return
	}
	col, err := findCalendar(acc, collection)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if col.ReadOnly {
		http.Error(w, "this collection is read-only", http.StatusForbidden)
		return
	}
	fp, _, err := s.filesProvider(sess, target.AccountID)
	if err != nil {
		s.redirectNotice(w, r, back, userFacingDAVError(err))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.filesMaxUpload())
	// The part is spooled to a temporary file rather than streamed straight
	// through, because a conditional create may have to be retried under a
	// second name and a stream cannot be rewound. It is a temporary file with a
	// removal in a defer, which is what §24.4 asks for; §23.10's "no temporary
	// files" is about not buffering a whole upload before starting, and the file
	// browser proper does stream.
	part, header, err := s.multipartFile(r, "file")
	if err != nil {
		s.redirectNotice(w, r, back, capitalize(err.Error()))
		return
	}
	defer part.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
	defer cancel()
	obj, err := p.Get(ctx, normalizeCollectionPath(col.Path), calendarObjectPath(col.Path, uid))
	if err != nil {
		s.redirectNotice(w, r, back, userFacingDAVError(err))
		return
	}
	if etag := strings.TrimSpace(r.FormValue("etag")); etag != "" {
		obj.ETag = etag
	}
	filename := s.attachmentName(obj, header.Filename)
	spooled, size, err := rewindable(part, s.filesMaxUpload())
	if err != nil {
		s.redirectNotice(w, r, back, capitalize(err.Error()))
		return
	}
	defer spooled.Close()

	if err := fp.EnsureDir(ctx, target.Root, target.Rel); err != nil {
		s.redirectNotice(w, r, back, userFacingDAVError(err))
		return
	}
	ctype := contentTypeOf(header.Header.Get("Content-Type"), filename)
	stored, storedPath, err := fp.UploadNew(ctx, target.Root, target.Rel, filename, spooled, ctype)
	if err != nil {
		s.redirectNotice(w, r, back, userFacingDAVError(err))
		return
	}

	uri, err := s.attachmentURI(sess, target.AccountID, storedPath)
	if err != nil {
		s.redirectNotice(w, r, back, userFacingDAVError(err))
		return
	}
	list := append(obj.Attachments(), model.Attachment{
		URI:   uri,
		Value: model.AttachmentValue(uri, ctype, stored, size),
	})
	if err := obj.Apply(model.AttachPatch(&model.Patch{}, list)); err != nil {
		s.redirectNotice(w, r, back, capitalize(err.Error()))
		return
	}
	if _, err := p.Update(ctx, normalizeCollectionPath(col.Path), obj); err != nil {
		if calendar.IsConflict(err) {
			// The file is on the server and the property is not. Saying so is
			// the honest outcome: nothing is rolled back, because deleting the
			// upload could delete a file another entry already points at
			// (§23.10).
			s.redirectNotice(w, r, back,
				"The file was uploaded as "+stored+", but the entry changed on the server meanwhile and the link was not added. Attach it again.")
			return
		}
		s.redirectNotice(w, r, back, userFacingDAVError(err))
		return
	}
	fp.Invalidate(target.Root, target.Rel)
	s.redirectNotice(w, r, back, "Attached "+stored+".")
}

// attachDetach removes one ATTACH and leaves the file where it is.
//
// §23.10 is explicit: another entry may point at the same file, so removing a
// link is not a licence to delete it. The interface says as much rather than
// leaving a person to assume the file went with it.
func (s *Server) attachDetach(w http.ResponseWriter, r *http.Request, section icalSection, accountID, colEnc, uid string) {
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil || uid == "" {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	back := s.objectURL(section, accountID, colEnc, uid)
	index, err := strconv.Atoi(strings.TrimSpace(r.PostFormValue("attachment")))
	if err != nil || index < 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	p, acc, err := s.calendarProvider(sess, accountID)
	if err != nil {
		s.redirectNotice(w, r, back, userFacingDAVError(err))
		return
	}
	col, err := findCalendar(acc, collection)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if col.ReadOnly {
		http.Error(w, "this collection is read-only", http.StatusForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	obj, err := p.Get(ctx, normalizeCollectionPath(col.Path), calendarObjectPath(col.Path, uid))
	if err != nil {
		s.redirectNotice(w, r, back, userFacingDAVError(err))
		return
	}
	if etag := strings.TrimSpace(r.PostFormValue("etag")); etag != "" {
		obj.ETag = etag
	}
	list := obj.Attachments()
	if index >= len(list) {
		s.redirectNotice(w, r, back, "That attachment is no longer on this entry.")
		return
	}
	name := list[index].DisplayName()
	kept := append(append([]model.Attachment(nil), list[:index]...), list[index+1:]...)
	if err := obj.Apply(model.AttachPatch(&model.Patch{}, kept)); err != nil {
		s.redirectNotice(w, r, back, capitalize(err.Error()))
		return
	}
	if _, err := p.Update(ctx, normalizeCollectionPath(col.Path), obj); err != nil {
		if calendar.IsConflict(err) {
			s.showICalConflict(w, r, sess, section, accountID, normalizeCollectionPath(col.Path), colEnc, uid, err)
			return
		}
		s.redirectNotice(w, r, back, userFacingDAVError(err))
		return
	}
	s.redirectNotice(w, r, back, "Removed the link to "+name+". The file is still on the server — other entries may point at it.")
}

// attachmentName builds a name a person can find again outside Carrel: the date
// and the title of the entry, then the original extension (§23.10 — the folder
// has to stay readable, and `image-1.png` twenty times over is not).
func (s *Server) attachmentName(obj *model.Object, uploaded string) string {
	ext := strings.ToLower(path.Ext(path.Base(strings.ReplaceAll(uploaded, "\\", "/"))))
	title, when := s.objectTitleAndDate(obj)
	stem := model.Slug(title)
	if stem == "" {
		stem = "attachment"
	}
	prefix := ""
	if !when.IsZero() {
		prefix = when.In(s.timezone()).Format("2006-01-02") + "-"
	}
	name, err := files.CleanName(prefix + stem + ext)
	if err != nil {
		return "attachment" + ext
	}
	return name
}

func (s *Server) objectTitleAndDate(obj *model.Object) (string, time.Time) {
	loc := s.timezone()
	switch obj.Component() {
	case "VJOURNAL":
		if note, err := obj.Note(loc); err == nil {
			return note.DisplayTitle(), note.Date
		}
	case "VEVENT":
		if event, err := obj.Event(loc); err == nil {
			return event.DisplayTitle(), event.Start
		}
	case "VTODO":
		if todo, err := obj.Todo(loc); err == nil {
			return todo.DisplayTitle(), todo.Due
		}
	}
	return "", time.Time{}
}

// attachmentURI builds the absolute URI that goes into ATTACH. It is absolute
// because that is what the property means and what another client needs; §23.10
// names the honest limit that comes with it — a third-party client will show the
// link and may not be able to open it.
func (s *Server) attachmentURI(sess *session.Session, accountID, absolutePath string) (string, error) {
	acc, err := s.Store.GetDAVAccount(sess.UserID, accountID, sess.DEK())
	if err != nil {
		return "", fmt.Errorf("account not found")
	}
	base, err := url.Parse(acc.BaseURL)
	if err != nil {
		return "", err
	}
	out := &url.URL{Scheme: base.Scheme, Host: base.Host, Path: absolutePath}
	return out.String(), nil
}

func (s *Server) objectURL(section icalSection, accountID, colEnc, uid string) string {
	return s.Path("/app/" + section.Path + "/" + accountID + "/" + colEnc + "/" + urlPathEscape(uid))
}

// AttachmentAction is the one endpoint both cards post to: attach a file, or
// remove a link. Which one is the `action` field, as everywhere else.
func (s *Server) AttachmentAction(w http.ResponseWriter, r *http.Request, section icalSection) {
	accountID, colEnc, uid := r.PathValue("account"), r.PathValue("col"), r.PathValue("uid")
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/") {
		s.attachUpload(w, r, section, accountID, colEnc, uid)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if r.PostFormValue(fieldAction) != "detach" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	s.attachDetach(w, r, section, accountID, colEnc, uid)
}

// NoteAttachment, EventAttachment and TaskAttachment are the routes of §23.10.
func (s *Server) NoteAttachment(w http.ResponseWriter, r *http.Request) {
	s.AttachmentAction(w, r, sectionNotes)
}

func (s *Server) EventAttachment(w http.ResponseWriter, r *http.Request) {
	s.AttachmentAction(w, r, sectionCalendar)
}

func (s *Server) TaskAttachment(w http.ResponseWriter, r *http.Request) {
	s.AttachmentAction(w, r, sectionTasks)
}

// rewindable returns the upload as something that can be read more than once,
// with its length.
//
// Attaching needs the body twice in the worst case: a conditional create may be
// refused because the name is taken, and the retry under the next name has to
// send it again. When the part is already backed by a file — which it is whenever
// the CSRF check had to read the form to find a token in a field — that file is
// used as it is. Otherwise the stream is spooled to one, and it is removed on
// close whatever the outcome (§24.4).
func rewindable(src io.Reader, max int64) (io.ReadSeekCloser, int64, error) {
	seeker, ok := src.(io.ReadSeeker)
	if !ok {
		return spoolUpload(src, max)
	}
	size, err := seeker.Seek(0, io.SeekEnd)
	if err != nil {
		return spoolUpload(src, max)
	}
	if _, err := seeker.Seek(0, io.SeekStart); err != nil {
		return nil, 0, errUploadInvalid
	}
	switch {
	case size == 0:
		return nil, 0, errUploadNoFile
	case size > max:
		return nil, 0, errAttachTooLarge
	}
	return nopSeekCloser{seeker}, size, nil
}

// nopSeekCloser leaves closing to whoever owns the part; the request does.
type nopSeekCloser struct{ io.ReadSeeker }

func (nopSeekCloser) Close() error { return nil }

func spoolUpload(src io.Reader, max int64) (io.ReadSeekCloser, int64, error) {
	tmp, err := os.CreateTemp("", "carrel-attach-*")
	if err != nil {
		return nil, 0, errUploadInvalid
	}
	written, err := io.Copy(tmp, io.LimitReader(src, max+1))
	if err != nil {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, 0, errUploadInvalid
	}
	if written > max {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, 0, errAttachTooLarge
	}
	if written == 0 {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, 0, errUploadNoFile
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, 0, errUploadInvalid
	}
	return &spooledFile{File: tmp}, written, nil
}

type spooledFile struct{ *os.File }

func (f *spooledFile) Close() error {
	name := f.Name()
	err := f.File.Close()
	_ = os.Remove(name)
	return err
}

var errAttachTooLarge = errors.New("that file is larger than uploads are allowed to be")
