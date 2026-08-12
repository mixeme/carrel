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
	"gitea.mixdep.ru/mix/carrel/internal/provider/calendar"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

type calendarImportView struct {
	Calendars      []calendarRef
	AccountID      string
	ColEnc         string
	Collection     discovery.Collection
	AccountLabel   string
	DraftKey       string
	Events         []calendarImportRow
	OKCount        int
	ErrorCount     int
	CollisionCount int
	TruncatedNote  string
	HasPreview     bool
}

type calendarImportRow struct {
	Source, DisplayName, OriginalUID, ParseError string
	UIDCollision                                 bool
}

type calendarImportReportView struct {
	Calendars    []calendarRef
	AccountID    string
	ColEnc       string
	Collection   discovery.Collection
	AccountLabel string
	Created      int
	Failed       []string
	Collisions   []string
}

// CalendarImport previews and confirms .ics imports.
func (s *Server) CalendarImport(w http.ResponseWriter, r *http.Request) {
	accountID, colEnc := r.PathValue("account"), r.PathValue("col")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	p, acc, err := s.calendarProvider(sess, accountID)
	if err != nil {
		s.renderCalendarError(w, r, err, accountID, colEnc)
		return
	}
	col, err := findCalendar(acc, collection)
	if err != nil {
		s.renderCalendarError(w, r, err, accountID, colEnc)
		return
	}
	if col.ReadOnly {
		http.Error(w, "this calendar is read-only", http.StatusForbidden)
		return
	}
	collection = normalizeCollectionPath(col.Path)
	key := "calendar-" + importDraftKey(accountID, collection)
	if r.Method == http.MethodGet {
		s.renderCalendarImport(w, r, calendarImportView{
			Calendars: s.listCalendars(sess), AccountID: accountID, ColEnc: colEnc,
			Collection: col, AccountLabel: accountLabel(*acc), DraftKey: key,
		})
		return
	}
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/") {
		s.previewCalendarImport(w, r, sess, p, acc, col, accountID, collection, colEnc, key)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	switch r.FormValue("action") {
	case "cancel_import":
		sess.ClearImport(key)
		http.Redirect(w, r, s.Path("/app/calendar/"+accountID+"/"+colEnc), http.StatusSeeOther)
	case "confirm_import":
		s.confirmCalendarImport(w, r, sess, p, acc, col, accountID, collection, colEnc, key)
	default:
		http.Error(w, "bad request", http.StatusBadRequest)
	}
}

func (s *Server) renderCalendarImport(w http.ResponseWriter, r *http.Request, data calendarImportView) {
	v := s.View(r, "Import calendar")
	v.Data = data
	s.Render(w, "calendar_import.html", v)
}

func (s *Server) previewCalendarImport(w http.ResponseWriter, r *http.Request, sess *session.Session, p *calendar.Provider, acc *account.Account, col discovery.Collection, accountID, collection, colEnc, key string) {
	maxBytes := s.importMaxBytes()
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		http.Error(w, "upload too large or invalid", http.StatusBadRequest)
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "choose an .ics, .ical, or .zip file", http.StatusBadRequest)
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
	maxEvents := s.Import.MaxCards
	if maxEvents <= 0 {
		maxEvents = 5000
	}
	parsed, truncErr := model.ReadICSImportPayload(filename, body, maxEvents)
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	listing, err := p.List(ctx, collection)
	if err != nil {
		s.renderCalendarError(w, r, err, accountID, colEnc)
		return
	}
	existing := map[string]bool{}
	for _, objectPath := range listing.Paths() {
		existing[uidFromEventPath(objectPath)] = true
	}
	draft := session.ImportDraft{Key: key, AccountID: accountID, Collection: collection}
	view := calendarImportView{
		Calendars: s.listCalendars(sess), AccountID: accountID, ColEnc: colEnc,
		Collection: col, AccountLabel: accountLabel(*acc), DraftKey: key, HasPreview: true,
	}
	if truncErr != nil {
		view.TruncatedNote = truncErr.Error()
	}
	for _, cal := range parsed {
		row := calendarImportRow{Source: cal.Source, ParseError: cal.Error}
		card := session.ImportCard{Source: cal.Source, ParseError: cal.Error}
		if cal.Error == "" && cal.Object != nil {
			ev, eventErr := cal.Object.Event(s.timezone())
			if eventErr != nil {
				row.ParseError, card.ParseError = eventErr.Error(), eventErr.Error()
			} else {
				uid := strings.TrimSpace(cal.Object.UID())
				row.DisplayName, row.OriginalUID = ev.DisplayTitle(), uid
				row.UIDCollision = uid != "" && existing[uid]
				if row.UIDCollision {
					view.CollisionCount++
				}
				raw, marshalErr := cal.Object.Marshal()
				if marshalErr != nil {
					row.ParseError, card.ParseError = marshalErr.Error(), marshalErr.Error()
				} else {
					card.Body, card.OriginalUID, card.DisplayName, card.UIDCollision = raw, uid, row.DisplayName, row.UIDCollision
					view.OKCount++
				}
			}
		}
		if row.ParseError != "" {
			view.ErrorCount++
		}
		draft.Cards = append(draft.Cards, card)
		view.Events = append(view.Events, row)
	}
	sess.PutImport(draft)
	s.renderCalendarImport(w, r, view)
}

func (s *Server) confirmCalendarImport(w http.ResponseWriter, r *http.Request, sess *session.Session, p *calendar.Provider, acc *account.Account, col discovery.Collection, accountID, collection, colEnc, key string) {
	draft, ok := sess.TakeImport(key)
	if !ok {
		http.Error(w, "no import in progress", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	listing, err := p.List(ctx, collection)
	if err != nil {
		s.renderCalendarError(w, r, err, accountID, colEnc)
		return
	}
	existing := map[string]bool{}
	for _, objectPath := range listing.Paths() {
		existing[uidFromEventPath(objectPath)] = true
	}
	report := calendarImportReportView{
		Calendars: s.listCalendars(sess), AccountID: accountID, ColEnc: colEnc,
		Collection: col, AccountLabel: accountLabel(*acc),
	}
	for _, item := range draft.Cards {
		if item.ParseError != "" || len(item.Body) == 0 {
			continue
		}
		obj, err := model.ParseICal("import", "", item.Body)
		if err != nil {
			report.Failed = append(report.Failed, item.Source+": "+err.Error())
			continue
		}
		uid := strings.TrimSpace(obj.UID())
		if uid == "" || existing[uid] {
			newUID, uidErr := model.NewUID()
			if uidErr != nil {
				report.Failed = append(report.Failed, item.Source+": "+uidErr.Error())
				continue
			}
			if uid != "" {
				report.Collisions = append(report.Collisions, fmt.Sprintf("%s (%s → %s)", item.DisplayName, uid, newUID))
			}
			if err := obj.AssignUID(newUID); err != nil {
				report.Failed = append(report.Failed, item.Source+": "+err.Error())
				continue
			}
			uid = newUID
		}
		obj.Path = eventPathForUID(collection, uid)
		if _, err := p.Create(ctx, collection, obj); err != nil {
			report.Failed = append(report.Failed, item.DisplayName+": "+userFacingDAVError(err))
			continue
		}
		existing[uid] = true
		report.Created++
	}
	v := s.View(r, "Calendar import report")
	v.Notice = fmt.Sprintf("Imported %d event(s).", report.Created)
	v.Data = report
	s.Render(w, "calendar_import_report.html", v)
}

// CalendarExport downloads all events, or events occurring in a requested range.
func (s *Server) CalendarExport(w http.ResponseWriter, r *http.Request) {
	accountID, colEnc := r.PathValue("account"), r.PathValue("col")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	p, acc, err := s.calendarProvider(sess, accountID)
	if err != nil {
		http.Error(w, userFacingDAVError(err), http.StatusBadRequest)
		return
	}
	col, err := findCalendar(acc, collection)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	collection = normalizeCollectionPath(col.Path)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	var paths []string
	etags := map[string]string{}
	fromText, toText := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	if fromText != "" || toText != "" {
		loc := s.timezone()
		from, fromErr := time.ParseInLocation("2006-01-02", fromText, loc)
		to, toErr := time.ParseInLocation("2006-01-02", toText, loc)
		if fromErr != nil || toErr != nil || to.Before(from) {
			http.Error(w, "from and to must be a valid YYYY-MM-DD range", http.StatusBadRequest)
			return
		}
		agenda, queryErr := p.Query(ctx, collection, from, to.AddDate(0, 0, 1))
		if queryErr != nil {
			http.Error(w, userFacingDAVError(queryErr), http.StatusBadGateway)
			return
		}
		seen := map[string]bool{}
		for _, occ := range agenda.Occurrences {
			if occ.Path != "" && !seen[occ.Path] {
				seen[occ.Path], etags[occ.Path] = true, occ.ETag
				paths = append(paths, occ.Path)
			}
		}
	} else {
		listing, listErr := p.List(ctx, collection)
		if listErr != nil {
			http.Error(w, userFacingDAVError(listErr), http.StatusBadGateway)
			return
		}
		paths, etags = listing.Paths(), listing.ETags
	}
	result, err := p.Multiget(ctx, collection, paths, etags)
	if err != nil {
		http.Error(w, userFacingDAVError(err), http.StatusBadGateway)
		return
	}
	var body strings.Builder
	for _, obj := range result.Objects {
		raw, marshalErr := obj.Marshal()
		if marshalErr == nil {
			body.Write(raw)
			if len(raw) > 0 && raw[len(raw)-1] != '\n' {
				body.WriteByte('\n')
			}
		}
	}
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="calendar.ics"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, body.String())
}
