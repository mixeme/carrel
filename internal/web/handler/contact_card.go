// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/emersion/go-vcard"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/model"
	"gitea.mixdep.ru/mix/carrel/internal/provider/contacts"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

type contactCardView struct {
	Books         []addressBookRef
	AccountID     string
	ColEnc        string
	Collection    discovery.Collection
	AccountLabel  string
	UID           string
	ETag          string
	Path          string
	Contact       model.Contact
	Form          contactForm
	ReadOnly      bool
	IsNew         bool
	PhotoURL      string
	PhotoEditable bool
	PhotoURI      bool
	Crop          *photoCropView
}

type contactForm struct {
	FormattedName string
	Family        string
	Given         string
	Additional    string
	Prefix        string
	Suffix        string
	Nickname      string
	Organization  string
	Title         string
	Role          string
	Birthday      string
	Note          string
	Categories    string
	Phones        []labeledForm
	Emails        []labeledForm
}

type labeledForm struct {
	Label string
	Value string
}

type photoCropView struct {
	Key     string
	PanX    float64
	PanY    float64
	Zoom    float64
	Rotate  int
	Preview string
}

// ContactNew shows or creates a new contact.
func (s *Server) ContactNew(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.contactSave(w, r, true)
		return
	}
	accountID := r.PathValue("account")
	colEnc := r.PathValue("col")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	_, acc, err := s.contactsProvider(sess, accountID)
	if err != nil {
		s.renderContactError(w, r, err, accountID, colEnc)
		return
	}
	col, err := findAddressBook(acc, collection)
	if err != nil {
		s.renderContactError(w, r, err, accountID, colEnc)
		return
	}
	if col.ReadOnly {
		http.Error(w, "this address book is read-only", http.StatusForbidden)
		return
	}
	v := s.View(r, "New contact")
	v.Data = contactCardView{
		Books:        s.listBooks(sess),
		AccountID:    accountID,
		ColEnc:       colEnc,
		Collection:   col,
		AccountLabel: accountLabel(*acc),
		IsNew:        true,
		Form: contactForm{
			Phones: []labeledForm{{}},
			Emails: []labeledForm{{}},
		},
	}
	s.Render(w, "contact.html", v)
}

// ContactCard shows or updates one contact.
func (s *Server) ContactCard(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.contactSave(w, r, false)
		return
	}
	accountID := r.PathValue("account")
	colEnc := r.PathValue("col")
	uid := r.PathValue("uid")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil || uid == "" {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	card, err := s.loadContactCard(r.Context(), sess, accountID, collection, colEnc, uid)
	if err != nil {
		s.renderContactError(w, r, err, accountID, colEnc)
		return
	}
	v := s.View(r, card.Contact.DisplayName())
	if notice := strings.TrimSpace(r.URL.Query().Get("notice")); notice != "" {
		v.Notice = notice
	}
	v.Data = card
	s.Render(w, "contact.html", v)
}

func (s *Server) contactSave(w http.ResponseWriter, r *http.Request, isNew bool) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
	}
	action := r.PostFormValue(fieldAction)
	accountID := r.PathValue("account")
	colEnc := r.PathValue("col")
	uid := r.PathValue("uid")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	switch action {
	case "delete":
		s.contactDelete(w, r, accountID, collection, colEnc, uid)
		return
	case "upload_photo", "crop_photo", "confirm_photo", "cancel_photo", "delete_photo":
		s.contactPhotoAction(w, r, accountID, collection, colEnc, uid, action)
		return
	case "resolve_conflict":
		s.resolveConflict(w, r)
		return
	}

	sess := SessionFrom(r)
	p, acc, err := s.contactsProvider(sess, accountID)
	if err != nil {
		s.renderContactError(w, r, err, accountID, colEnc)
		return
	}
	col, err := findAddressBook(acc, collection)
	if err != nil {
		s.renderContactError(w, r, err, accountID, colEnc)
		return
	}
	if col.ReadOnly {
		http.Error(w, "this address book is read-only", http.StatusForbidden)
		return
	}
	collection = normalizeCollectionPath(col.Path)

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	form := parseContactForm(r)
	var obj *model.Object
	if isNew {
		newUID, err := model.NewUID()
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		obj, err = model.NewVCard(model.DefaultVCardVersion, newUID)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		uid = newUID
	} else {
		path := objectPathForUID(collection, uid)
		obj, err = p.Get(ctx, collection, path)
		if err != nil {
			s.renderContactError(w, r, err, accountID, colEnc)
			return
		}
		if etag := strings.TrimSpace(r.PostFormValue("etag")); etag != "" {
			obj.ETag = etag
		}
	}

	patch := form.toPatch()
	if err := obj.Apply(patch); err != nil {
		v := s.View(r, "Contact")
		v.Error = capitalize(err.Error())
		v.Data = s.cardFromObject(sess, accountID, colEnc, col, *acc, obj, form, isNew)
		s.RenderStatus(w, http.StatusBadRequest, "contact.html", v)
		return
	}

	var result *contacts.WriteResult
	if isNew {
		result, err = p.Create(ctx, collection, obj)
	} else {
		result, err = p.Update(ctx, collection, obj)
	}
	if err != nil {
		if contacts.IsConflict(err) {
			s.showConflict(w, r, sess, accountID, collection, colEnc, uid, err)
			return
		}
		s.renderContactError(w, r, err, accountID, colEnc)
		return
	}

	notice := "Contact saved."
	if result != nil && result.ReportLoss && !result.Loss.Empty() {
		notice = "Contact saved, but " + result.Loss.String() + "."
	}
	targetUID := uid
	if result != nil && result.Object != nil && result.Object.UID() != "" {
		targetUID = result.Object.UID()
	}
	s.redirectNotice(w, r, s.Path("/app/contacts/"+accountID+"/"+colEnc+"/"+urlPathEscape(targetUID)), notice)
}

func (s *Server) contactDelete(w http.ResponseWriter, r *http.Request, accountID, collection, colEnc, uid string) {
	sess := SessionFrom(r)
	p, acc, err := s.contactsProvider(sess, accountID)
	if err != nil {
		s.renderContactError(w, r, err, accountID, colEnc)
		return
	}
	col, err := findAddressBook(acc, collection)
	if err != nil {
		s.renderContactError(w, r, err, accountID, colEnc)
		return
	}
	if col.ReadOnly {
		http.Error(w, "this address book is read-only", http.StatusForbidden)
		return
	}
	collection = normalizeCollectionPath(col.Path)
	etag := strings.TrimSpace(r.PostFormValue("etag"))
	path := objectPathForUID(collection, uid)

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if err := p.Delete(ctx, collection, path, etag); err != nil {
		if contacts.IsConflict(err) {
			s.showConflict(w, r, sess, accountID, collection, colEnc, uid, err)
			return
		}
		s.renderContactError(w, r, err, accountID, colEnc)
		return
	}
	http.Redirect(w, r, s.Path("/app/contacts/"+accountID+"/"+colEnc), http.StatusSeeOther)
}

func (s *Server) loadContactCard(ctx context.Context, sess *session.Session, accountID, collection, colEnc, uid string) (contactCardView, error) {
	p, acc, err := s.contactsProvider(sess, accountID)
	if err != nil {
		return contactCardView{}, err
	}
	col, err := findAddressBook(acc, collection)
	if err != nil {
		return contactCardView{}, err
	}
	collection = normalizeCollectionPath(col.Path)
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	obj, err := p.Get(ctx, collection, objectPathForUID(collection, uid))
	if err != nil {
		return contactCardView{}, err
	}
	c, err := obj.Contact()
	if err != nil {
		return contactCardView{}, err
	}
	return s.cardFromObject(sess, accountID, colEnc, col, *acc, obj, formFromContact(c), false), nil
}

func (s *Server) cardFromObject(sess *session.Session, accountID, colEnc string, col discovery.Collection, acc account.Account, obj *model.Object, form contactForm, isNew bool) contactCardView {
	uid := ""
	etag := ""
	path := ""
	var c model.Contact
	if obj != nil {
		uid = obj.UID()
		etag = obj.ETag
		path = obj.Path
		c, _ = obj.Contact()
	}
	photoURL := ""
	if uid != "" {
		photoURL = s.Path("/c/" + accountID + "/" + colEnc + "/" + urlPathEscape(uid) + "/photo?size=full")
	}
	view := contactCardView{
		Books:         s.listBooks(sess),
		AccountID:     accountID,
		ColEnc:        colEnc,
		Collection:    col,
		AccountLabel:  accountLabel(acc),
		UID:           uid,
		ETag:          etag,
		Path:          path,
		Contact:       c,
		Form:          form,
		ReadOnly:      col.ReadOnly,
		IsNew:         isNew,
		PhotoURL:      photoURL,
		PhotoEditable: !col.ReadOnly && (c.Photo.Editable || !c.Photo.Present),
		PhotoURI:      c.Photo.URI != "",
	}
	if uid != "" {
		if draft, ok := sess.PhotoDraft(photoDraftKey(accountID, col.Path, uid)); ok {
			view.Crop = &photoCropView{
				Key:     draft.Key,
				PanX:    draft.PanX,
				PanY:    draft.PanY,
				Zoom:    draft.Zoom,
				Rotate:  draft.Rotate,
				Preview: s.Path("/app/contacts/" + accountID + "/" + colEnc + "/" + urlPathEscape(uid) + "/photo-preview"),
			}
		}
	}
	return view
}

func (s *Server) renderContactError(w http.ResponseWriter, r *http.Request, err error, accountID, colEnc string) {
	sess := SessionFrom(r)
	v := s.View(r, "Contacts")
	v.Error = userFacingDAVError(err)
	v.Data = contactsListView{Books: s.listBooks(sess), AccountID: accountID, ColEnc: colEnc}
	s.RenderStatus(w, http.StatusBadRequest, "contacts.html", v)
}

func (s *Server) redirectNotice(w http.ResponseWriter, r *http.Request, target, notice string) {
	if notice != "" {
		u, err := url.Parse(target)
		if err == nil {
			q := u.Query()
			q.Set("notice", notice)
			u.RawQuery = q.Encode()
			target = u.String()
		}
	}
	if IsHTMX(r) {
		w.Header().Set("HX-Redirect", target)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func parseContactForm(r *http.Request) contactForm {
	f := contactForm{
		FormattedName: strings.TrimSpace(r.PostFormValue("fn")),
		Family:        strings.TrimSpace(r.PostFormValue("family")),
		Given:         strings.TrimSpace(r.PostFormValue("given")),
		Additional:    strings.TrimSpace(r.PostFormValue("additional")),
		Prefix:        strings.TrimSpace(r.PostFormValue("prefix")),
		Suffix:        strings.TrimSpace(r.PostFormValue("suffix")),
		Nickname:      strings.TrimSpace(r.PostFormValue("nickname")),
		Organization:  strings.TrimSpace(r.PostFormValue("org")),
		Title:         strings.TrimSpace(r.PostFormValue("title")),
		Role:          strings.TrimSpace(r.PostFormValue("role")),
		Birthday:      strings.TrimSpace(r.PostFormValue("bday")),
		Note:          strings.TrimSpace(r.PostFormValue("note")),
		Categories:    strings.TrimSpace(r.PostFormValue("categories")),
	}
	phones := r.PostForm["phone"]
	phoneLabels := r.PostForm["phone_label"]
	for i, v := range phones {
		label := ""
		if i < len(phoneLabels) {
			label = strings.TrimSpace(phoneLabels[i])
		}
		if strings.TrimSpace(v) == "" && label == "" {
			continue
		}
		f.Phones = append(f.Phones, labeledForm{Label: label, Value: strings.TrimSpace(v)})
	}
	emails := r.PostForm["email"]
	emailLabels := r.PostForm["email_label"]
	for i, v := range emails {
		label := ""
		if i < len(emailLabels) {
			label = strings.TrimSpace(emailLabels[i])
		}
		if strings.TrimSpace(v) == "" && label == "" {
			continue
		}
		f.Emails = append(f.Emails, labeledForm{Label: label, Value: strings.TrimSpace(v)})
	}
	if len(f.Phones) == 0 {
		f.Phones = []labeledForm{{}}
	}
	if len(f.Emails) == 0 {
		f.Emails = []labeledForm{{}}
	}
	return f
}

func formFromContact(c model.Contact) contactForm {
	f := contactForm{
		FormattedName: c.FormattedName,
		Family:        c.Name.Family,
		Given:         c.Name.Given,
		Additional:    c.Name.Additional,
		Prefix:        c.Name.Prefix,
		Suffix:        c.Name.Suffix,
		Nickname:      c.Nickname,
		Organization:  strings.Join(c.Organization, ";"),
		Title:         c.Title,
		Role:          c.Role,
		Birthday:      c.Birthday,
		Note:          c.Note,
		Categories:    strings.Join(c.Categories, ", "),
	}
	for _, p := range c.Phones {
		f.Phones = append(f.Phones, labeledForm{Label: p.Label, Value: p.Value})
	}
	for _, e := range c.Emails {
		f.Emails = append(f.Emails, labeledForm{Label: e.Label, Value: e.Value})
	}
	if len(f.Phones) == 0 {
		f.Phones = []labeledForm{{}}
	}
	if len(f.Emails) == 0 {
		f.Emails = []labeledForm{{}}
	}
	return f
}

func (f contactForm) toPatch() *model.Patch {
	p := &model.Patch{}
	setOrRemove := func(name, text string) {
		if strings.TrimSpace(text) == "" {
			p.Remove(name)
			return
		}
		p.SetText(name, text)
	}
	setOrRemove(vcard.FieldFormattedName, f.FormattedName)
	n := strings.Join([]string{f.Family, f.Given, f.Additional, f.Prefix, f.Suffix}, ";")
	if strings.Trim(n, ";") == "" {
		p.Remove(vcard.FieldName)
	} else {
		p.SetText(vcard.FieldName, n)
	}
	setOrRemove(vcard.FieldNickname, f.Nickname)
	setOrRemove(vcard.FieldOrganization, f.Organization)
	setOrRemove(vcard.FieldTitle, f.Title)
	setOrRemove(vcard.FieldRole, f.Role)
	setOrRemove(vcard.FieldBirthday, f.Birthday)
	setOrRemove(vcard.FieldNote, f.Note)
	if strings.TrimSpace(f.Categories) == "" {
		p.Remove(vcard.FieldCategories)
	} else {
		parts := strings.Split(f.Categories, ",")
		clean := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				clean = append(clean, part)
			}
		}
		if len(clean) == 0 {
			p.Remove(vcard.FieldCategories)
		} else {
			p.SetText(vcard.FieldCategories, strings.Join(clean, ","))
		}
	}

	phoneVals := labeledValues(f.Phones)
	if len(phoneVals) == 0 {
		p.Remove(vcard.FieldTelephone)
	} else {
		p.Set(vcard.FieldTelephone, phoneVals...)
	}
	emailVals := labeledValues(f.Emails)
	if len(emailVals) == 0 {
		p.Remove(vcard.FieldEmail)
	} else {
		p.Set(vcard.FieldEmail, emailVals...)
	}
	return p
}

func labeledValues(items []labeledForm) []model.Value {
	var out []model.Value
	for _, item := range items {
		if strings.TrimSpace(item.Value) == "" {
			continue
		}
		v := model.Text(item.Value)
		if label := strings.TrimSpace(item.Label); label != "" {
			v = v.WithParam(vcard.ParamType, label)
		}
		out = append(out, v)
	}
	return out
}
