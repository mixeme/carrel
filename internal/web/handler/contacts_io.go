// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/model"
	"gitea.mixdep.ru/mix/carrel/internal/provider/contacts"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

type importPreviewView struct {
	Books          []addressBookRef
	AccountID      string
	ColEnc         string
	Collection     discovery.Collection
	AccountLabel   string
	DraftKey       string
	Cards          []importCardRow
	OKCount        int
	ErrorCount     int
	CollisionCount int
	TruncatedNote  string
	HasPreview     bool
}

type importCardRow struct {
	Source       string
	DisplayName  string
	OriginalUID  string
	UIDCollision bool
	ParseError   string
}

type importReportView struct {
	Books        []addressBookRef
	AccountID    string
	ColEnc       string
	Collection   discovery.Collection
	AccountLabel string
	Created      int
	Failed       []string
	Collisions   []string
}

// ContactsImport handles GET (upload form) and POST (preview / confirm / cancel).
func (s *Server) ContactsImport(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	colEnc := r.PathValue("col")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		http.NotFound(w, r)
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
	key := importDraftKey(accountID, collection)

	if r.Method == http.MethodGet {
		v := s.View(r, "Import contacts")
		v.Data = importPreviewView{
			Books:        s.listBooks(sess),
			AccountID:    accountID,
			ColEnc:       colEnc,
			Collection:   col,
			AccountLabel: accountLabel(*acc),
			DraftKey:     key,
		}
		s.Render(w, "contacts_import.html", v)
		return
	}

	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(strings.ToLower(ct), "multipart/") {
		s.previewImport(w, r, sess, p, acc, col, accountID, collection, colEnc, key)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	switch r.FormValue("action") {
	case "cancel_import":
		sess.ClearImport(key)
		http.Redirect(w, r, s.Path("/app/contacts/"+accountID+"/"+colEnc), http.StatusSeeOther)
		return
	case "confirm_import":
		s.confirmImport(w, r, sess, p, acc, col, accountID, collection, colEnc, key)
		return
	default:
		http.Error(w, "bad request", http.StatusBadRequest)
	}
}

func (s *Server) previewImport(w http.ResponseWriter, r *http.Request, sess *session.Session, p *contacts.Provider, acc *account.Account, col discovery.Collection, accountID, collection, colEnc, key string) {
	maxBytes := s.importMaxBytes()
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		http.Error(w, "upload too large or invalid", http.StatusBadRequest)
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "choose a .vcf or .zip file", http.StatusBadRequest)
		return
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, maxBytes))
	if err != nil {
		http.Error(w, "could not read upload", http.StatusBadRequest)
		return
	}
	filename := ""
	if hdr != nil {
		filename = path.Base(hdr.Filename)
	}

	maxCards := s.Import.MaxCards
	if maxCards <= 0 {
		maxCards = 5000
	}
	parsed, truncErr := model.ReadImportPayload(filename, body, maxCards)

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	listing, err := p.List(ctx, collection)
	if err != nil {
		s.renderContactError(w, r, err, accountID, colEnc)
		return
	}
	existingUIDs := map[string]bool{}
	for _, objectPath := range listing.Paths() {
		existingUIDs[uidFromObjectPath(objectPath)] = true
	}

	draft := session.ImportDraft{Key: key, AccountID: accountID, Collection: collection}
	view := importPreviewView{
		Books:        s.listBooks(sess),
		AccountID:    accountID,
		ColEnc:       colEnc,
		Collection:   col,
		AccountLabel: accountLabel(*acc),
		DraftKey:     key,
		HasPreview:   true,
	}
	if truncErr != nil {
		view.TruncatedNote = truncErr.Error()
	}

	for _, card := range parsed {
		row := importCardRow{Source: card.Source, ParseError: card.Error}
		if card.Error != "" || card.Object == nil {
			view.ErrorCount++
			draft.Cards = append(draft.Cards, session.ImportCard{
				Source:     card.Source,
				ParseError: card.Error,
			})
			view.Cards = append(view.Cards, row)
			continue
		}
		c, _ := card.Object.Contact()
		uid := strings.TrimSpace(card.Object.UID())
		row.DisplayName = c.DisplayName()
		row.OriginalUID = uid
		collision := uid != "" && existingUIDs[uid]
		row.UIDCollision = collision
		if collision {
			view.CollisionCount++
		}
		view.OKCount++
		raw, err := card.Object.Marshal()
		if err != nil {
			row.ParseError = err.Error()
			view.ErrorCount++
			view.OKCount--
			draft.Cards = append(draft.Cards, session.ImportCard{
				Source:     card.Source,
				ParseError: err.Error(),
			})
			view.Cards = append(view.Cards, row)
			continue
		}
		draft.Cards = append(draft.Cards, session.ImportCard{
			Body:         raw,
			Source:       card.Source,
			OriginalUID:  uid,
			DisplayName:  row.DisplayName,
			UIDCollision: collision,
		})
		view.Cards = append(view.Cards, row)
	}
	sess.PutImport(draft)

	v := s.View(r, "Import contacts")
	v.Data = view
	s.Render(w, "contacts_import.html", v)
}

func (s *Server) confirmImport(w http.ResponseWriter, r *http.Request, sess *session.Session, p *contacts.Provider, acc *account.Account, col discovery.Collection, accountID, collection, colEnc, key string) {
	draft, ok := sess.TakeImport(key)
	if !ok {
		http.Error(w, "no import in progress", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	listing, err := p.List(ctx, collection)
	if err != nil {
		s.renderContactError(w, r, err, accountID, colEnc)
		return
	}
	existingUIDs := map[string]bool{}
	for _, objectPath := range listing.Paths() {
		existingUIDs[uidFromObjectPath(objectPath)] = true
	}

	report := importReportView{
		Books:        s.listBooks(sess),
		AccountID:    accountID,
		ColEnc:       colEnc,
		Collection:   col,
		AccountLabel: accountLabel(*acc),
	}

	for _, card := range draft.Cards {
		if card.ParseError != "" || len(card.Body) == 0 {
			continue
		}
		obj, err := model.ParseVCard("import", "", card.Body)
		if err != nil {
			report.Failed = append(report.Failed, card.Source+": "+err.Error())
			continue
		}
		uid := strings.TrimSpace(obj.UID())
		if uid == "" || existingUIDs[uid] {
			newUID, err := model.NewUID()
			if err != nil {
				report.Failed = append(report.Failed, card.Source+": "+err.Error())
				continue
			}
			if uid != "" {
				report.Collisions = append(report.Collisions,
					fmt.Sprintf("%s (%s → %s)", card.DisplayName, uid, newUID))
			}
			if err := obj.AssignUID(newUID); err != nil {
				report.Failed = append(report.Failed, card.Source+": "+err.Error())
				continue
			}
			uid = newUID
		}
		obj.Path = objectPathForUID(collection, uid)
		if _, err := p.Create(ctx, collection, obj); err != nil {
			report.Failed = append(report.Failed, card.DisplayName+": "+userFacingDAVError(err))
			continue
		}
		existingUIDs[uid] = true
		report.Created++
	}

	v := s.View(r, "Import report")
	v.Notice = fmt.Sprintf("Imported %d contact(s).", report.Created)
	v.Data = report
	s.Render(w, "contacts_import_report.html", v)
}

// ContactsExport downloads every contact in the address book as a .vcf file.
func (s *Server) ContactsExport(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	colEnc := r.PathValue("col")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	p, acc, err := s.contactsProvider(sess, accountID)
	if err != nil {
		http.Error(w, userFacingDAVError(err), http.StatusBadRequest)
		return
	}
	col, err := findAddressBook(acc, collection)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	collection = normalizeCollectionPath(col.Path)

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	listing, err := p.List(ctx, collection)
	if err != nil {
		http.Error(w, userFacingDAVError(err), http.StatusBadRequest)
		return
	}
	paths := listing.Paths()
	var buf strings.Builder
	for start := 0; start < len(paths); start += contactsBatch {
		end := start + contactsBatch
		if end > len(paths) {
			end = len(paths)
		}
		result, err := p.Multiget(ctx, collection, paths[start:end], listing.ETags)
		if err != nil {
			http.Error(w, userFacingDAVError(err), http.StatusBadGateway)
			return
		}
		for _, obj := range result.Objects {
			raw, err := obj.Marshal()
			if err != nil {
				continue
			}
			buf.Write(raw)
			if len(raw) > 0 && raw[len(raw)-1] != '\n' {
				buf.WriteByte('\n')
			}
		}
	}

	name := col.DisplayName
	if name == "" {
		name = "contacts"
	}
	name = sanitizeFilename(name) + ".vcf"
	w.Header().Set("Content-Type", "text/vcard; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, buf.String())
}

func (s *Server) importMaxBytes() int64 {
	if s.Import.MaxBytes > 0 {
		return s.Import.MaxBytes
	}
	return 16 << 20
}

func importDraftKey(accountID, collection string) string {
	return "import|" + accountID + "|" + normalizeCollectionPath(collection)
}

func sanitizeFilename(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "contacts"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('_')
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" || out == "." || out == ".." {
		return "contacts"
	}
	return out
}
