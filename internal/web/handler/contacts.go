// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/model"
	"gitea.mixdep.ru/mix/carrel/internal/provider/contacts"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

const contactsBatch = contacts.DefaultBatchSize

type contactsListView struct {
	Books        []addressBookRef
	AccountID    string
	ColEnc       string
	Collection   discovery.Collection
	AccountLabel string
	Contacts     []contactRow
	Offset       int
	NextOffset   int
	HasMore      bool
	ReadOnly     bool
	Empty        bool
	NoBooks      bool
	PrintDate    string
	SectionRail  sectionRail
	Mode         findMode
}

type contactRow struct {
	UID         string
	DisplayName string
	Phones      string
	Emails      string
	HasPhoto    bool
	PhotoURL    string
	// Linked is how many records the card is linked with, zero when it is in no
	// group. §15 asks the badge to be visible where the record is, not only on
	// the duplicates screen.
	Linked int
	// DupURL is where the badge leads.
	DupURL string
}

// ContactsHome shows every ticked address book at once, or an empty state.
func (s *Server) ContactsHome(w http.ResponseWriter, r *http.Request) {
	sess := SessionFrom(r)
	accounts, err := s.Store.ListDAVAccounts(sess.UserID, sess.DEK())
	if err != nil {
		s.logError("list DAV accounts", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	books := addressBooks(accounts)
	if len(books) == 0 {
		v := s.View(r, "Contacts")
		v.Data = contactsListView{NoBooks: true, Books: books}
		s.Render(w, "contacts.html", v)
		return
	}
	s.sectionFind(w, r, findRequest{Mode: modePeople}, "contacts.html")
}

// ContactsList shows one address book with the first batch of contacts.
func (s *Server) ContactsList(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	colEnc := r.PathValue("col")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	sess := SessionFrom(r)
	view, err := s.buildContactsList(r.Context(), sess, accountID, collection, colEnc, 0)
	if err != nil {
		v := s.View(r, "Contacts")
		v.Error = userFacingDAVError(err)
		data := contactsListView{Books: s.listBooks(sess), AccountID: accountID, ColEnc: colEnc}
		if rail, railErr := s.buildSectionRail(sess, findRequest{Mode: modePeople}, accountID, colEnc); railErr == nil {
			data.SectionRail = rail
		}
		v.Data = data
		s.RenderStatus(w, http.StatusBadRequest, "contacts.html", v)
		return
	}
	if rail, railErr := s.buildSectionRail(sess, findRequest{Mode: modePeople}, accountID, colEnc); railErr == nil {
		view.SectionRail = rail
	}
	v := s.View(r, "Contacts")
	v.Data = view
	s.Render(w, "contacts.html", v)
}

// ContactsPage returns the next batch of contact rows for htmx infinite scroll.
func (s *Server) ContactsPage(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	colEnc := r.PathValue("col")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	sess := SessionFrom(r)
	view, err := s.buildContactsList(r.Context(), sess, accountID, collection, colEnc, offset)
	if err != nil {
		http.Error(w, userFacingDAVError(err), http.StatusBadRequest)
		return
	}
	v := s.View(r, "Contacts")
	v.Data = view
	s.RenderFragment(w, "contacts_page.html", v)
}

func (s *Server) listBooks(sess *session.Session) []addressBookRef {
	accounts, err := s.Store.ListDAVAccounts(sess.UserID, sess.DEK())
	if err != nil {
		return nil
	}
	return addressBooks(accounts)
}

func (s *Server) buildContactsList(ctx context.Context, sess *session.Session, accountID, collection, colEnc string, offset int) (contactsListView, error) {
	books := s.listBooks(sess)
	p, acc, err := s.contactsProvider(sess, accountID)
	if err != nil {
		return contactsListView{Books: books}, err
	}
	col, err := findAddressBook(acc, collection)
	if err != nil {
		return contactsListView{Books: books}, err
	}
	collection = normalizeCollectionPath(col.Path)

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	listing, err := p.List(ctx, collection)
	if err != nil {
		return contactsListView{Books: books}, err
	}
	paths := listing.Paths()
	view := contactsListView{
		Books:        books,
		AccountID:    accountID,
		ColEnc:       colEnc,
		Collection:   col,
		AccountLabel: accountLabel(*acc),
		Offset:       offset,
		ReadOnly:     col.ReadOnly,
		Empty:        len(paths) == 0,
		PrintDate:    time.Now().UTC().Format("2006-01-02 15:04 UTC"),
	}
	if offset >= len(paths) {
		return view, nil
	}
	end := offset + contactsBatch
	if end > len(paths) {
		end = len(paths)
	}
	batch := paths[offset:end]
	result, err := p.Multiget(ctx, collection, batch, listing.ETags)
	if err != nil {
		return view, err
	}

	// The stored decisions are read once for the page rather than per card: they
	// all live in one sealed blob, so a badge per row costs nothing extra (§15).
	marks := s.duplicateMarks(sess)
	rows := make([]contactRow, 0, len(result.Objects))
	for _, obj := range result.Objects {
		c, err := obj.Contact()
		if err != nil {
			continue
		}
		uid := c.UID
		if uid == "" {
			uid = uidFromObjectPath(obj.Path)
		}
		row := contactRow{
			UID:         uid,
			DisplayName: c.DisplayName(),
			Phones:      joinLabeled(c.Phones),
			Emails:      joinLabeled(c.Emails),
			HasPhoto:    c.Photo.Present,
			PhotoURL:    s.Path("/c/" + accountID + "/" + colEnc + "/" + urlPathEscape(uid) + "/photo?size=thumb"),
			Linked:      marks.linkedSize(accountID, collection, uid),
			DupURL:      s.Path("/app/duplicates"),
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return strings.ToLower(rows[i].DisplayName) < strings.ToLower(rows[j].DisplayName)
	})
	view.Contacts = rows
	view.NextOffset = end
	view.HasMore = end < len(paths)
	return view, nil
}

func joinLabeled(values []model.LabeledValue) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v.Value) == "" {
			continue
		}
		parts = append(parts, v.Value)
	}
	return strings.Join(parts, ", ")
}

func urlPathEscape(s string) string {
	return strings.ReplaceAll(url.PathEscape(s), "+", "%20")
}
