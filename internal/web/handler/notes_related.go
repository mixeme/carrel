// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/model"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

const relatedSearchLimit = 20

type relatedSearchHit struct {
	UID   string `json:"uid"`
	Title string `json:"title"`
	Kind  string `json:"kind"`
	Meta  string `json:"meta"`
}

// NoteRelatedSearch answers typeahead queries for the note editor (wave 2.3).
// It searches the same selected sources as global search: calendar collections
// for events, tasks and notes, and address books for contacts.
func (s *Server) NoteRelatedSearch(w http.ResponseWriter, r *http.Request) {
	sess := SessionFrom(r)
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	exclude := strings.TrimSpace(r.URL.Query().Get("exclude"))
	if query == "" {
		writeRelatedHits(w, nil)
		return
	}
	rows, err := s.findSources(sess, findRequest{Mode: modeSearch})
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	selected := selectedRows(rows)
	if accountID, colEnc := r.PathValue("account"), r.PathValue("col"); accountID != "" && colEnc != "" {
		selected = s.ensureRelatedSource(selected, sess, accountID, colEnc)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	hits := s.searchRelated(ctx, sess, selected, query, exclude)
	writeRelatedHits(w, hits)
}

func writeRelatedHits(w http.ResponseWriter, hits []relatedSearchHit) {
	if hits == nil {
		hits = []relatedSearchHit{}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(hits)
}

func (s *Server) ensureRelatedSource(rows []sourceRow, sess *session.Session, accountID, colEnc string) []sourceRow {
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		return rows
	}
	collection = normalizeCollectionPath(collection)
	for _, row := range rows {
		if row.AccountID == accountID && normalizeCollectionPath(row.Path) == collection {
			return rows
		}
	}
	accounts, err := s.Store.ListDAVAccounts(sess.UserID, sess.DEK())
	if err != nil {
		return rows
	}
	for _, acc := range accounts {
		if acc.ID != accountID {
			continue
		}
		for _, col := range acc.Collections {
			if col.Kind != discovery.KindCalendar || normalizeCollectionPath(col.Path) != collection {
				continue
			}
			return append(rows, sourceRow{
				AccountID: acc.ID, AccountLabel: accountLabel(acc),
				ColEnc: colEnc, Path: col.Path, Name: col.DisplayName,
				Color: col.Color, Kind: col.Kind, ReadOnly: col.ReadOnly, Selected: true,
			})
		}
	}
	return rows
}

func (s *Server) searchRelated(ctx context.Context, sess *session.Session, sources []sourceRow, query, excludeUID string) []relatedSearchHit {
	seen := make(map[string]bool)
	var hits []relatedSearchHit
	loc := s.timezone()
	for _, row := range sources {
		if len(hits) >= relatedSearchLimit {
			break
		}
		var batch []relatedSearchHit
		var err error
		if row.Kind == discovery.KindAddressBook {
			batch, err = s.searchRelatedContacts(ctx, sess, row, query, loc)
		} else {
			batch, err = s.searchRelatedCalendar(ctx, sess, row, query, loc)
		}
		if err != nil {
			continue
		}
		for _, hit := range batch {
			if hit.UID == "" || hit.UID == excludeUID || seen[hit.UID] {
				continue
			}
			seen[hit.UID] = true
			hits = append(hits, hit)
			if len(hits) >= relatedSearchLimit {
				break
			}
		}
	}
	return hits
}

func (s *Server) searchRelatedCalendar(ctx context.Context, sess *session.Session, row sourceRow, query string, loc *time.Location) ([]relatedSearchHit, error) {
	p, _, err := s.calendarProvider(sess, row.AccountID)
	if err != nil {
		return nil, err
	}
	set, err := p.Search(ctx, normalizeCollectionPath(row.Path), query)
	if err != nil {
		return nil, err
	}
	metaBase := row.Label()
	if row.AccountLabel != "" && row.Name != "" {
		metaBase = row.Name + " · " + row.AccountLabel
	}
	out := make([]relatedSearchHit, 0, len(set.Objects))
	for _, obj := range set.Objects {
		hit, ok := relatedHitFromObject(obj, metaBase, loc)
		if !ok {
			continue
		}
		out = append(out, hit)
	}
	return out, nil
}

func relatedHitFromObject(obj *model.Object, metaBase string, loc *time.Location) (relatedSearchHit, bool) {
	switch obj.Component() {
	case "VEVENT":
		event, err := obj.Event(loc)
		if err != nil {
			return relatedSearchHit{}, false
		}
		return relatedSearchHit{
			UID: event.UID, Title: event.DisplayTitle(), Kind: "event",
			Meta: relatedMeta(metaBase, eventDateLabel(event.Start, event.AllDay, loc)),
		}, true
	case "VTODO":
		todo, err := obj.Todo(loc)
		if err != nil {
			return relatedSearchHit{}, false
		}
		return relatedSearchHit{
			UID: todo.UID, Title: todo.DisplayTitle(), Kind: "task",
			Meta: relatedMeta("task", metaBase),
		}, true
	case "VJOURNAL":
		note, err := obj.Note(loc)
		if err != nil {
			return relatedSearchHit{}, false
		}
		return relatedSearchHit{
			UID: note.UID, Title: note.DisplayTitle(), Kind: "note",
			Meta: relatedMeta(metaBase, eventDateLabel(note.Date, note.DateOnly, loc)),
		}, true
	default:
		return relatedSearchHit{}, false
	}
}

func (s *Server) searchRelatedContacts(ctx context.Context, sess *session.Session, row sourceRow, query string, loc *time.Location) ([]relatedSearchHit, error) {
	_ = loc
	p, _, err := s.contactsProvider(sess, row.AccountID)
	if err != nil {
		return nil, err
	}
	result, err := p.Search(ctx, normalizeCollectionPath(row.Path), query)
	if err != nil {
		return nil, err
	}
	meta := "contact"
	if row.Name != "" {
		meta = "contact · " + row.Label()
	}
	out := make([]relatedSearchHit, 0, len(result.Objects))
	for _, obj := range result.Objects {
		contact, err := obj.Contact()
		if err != nil {
			continue
		}
		out = append(out, relatedSearchHit{
			UID: contact.UID, Title: contact.DisplayName(), Kind: "contact", Meta: meta,
		})
	}
	return out, nil
}

func relatedMeta(parts ...string) string {
	var out []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return strings.Join(out, " · ")
}

func eventDateLabel(at time.Time, dateOnly bool, loc *time.Location) string {
	if at.IsZero() {
		return ""
	}
	local := at.In(loc)
	if dateOnly {
		return local.Format("2 Jan")
	}
	return local.Format("2 Jan 15:04")
}

func relatedPickerRows(formRelated string, resolved []relatedRow) []relatedRow {
	relations := model.ParseRelations(formRelated)
	if len(relations) == 0 {
		return nil
	}
	byUID := make(map[string]relatedRow, len(resolved))
	for _, row := range resolved {
		byUID[row.UID] = row
	}
	out := make([]relatedRow, 0, len(relations))
	for _, rel := range relations {
		if row, ok := byUID[rel.UID]; ok {
			out = append(out, row)
			continue
		}
		out = append(out, relatedRow{UID: rel.UID, Title: rel.UID, Kind: "unknown"})
	}
	return out
}
