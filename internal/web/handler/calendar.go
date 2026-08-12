// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

type agendaView struct {
	Calendars    []calendarRef
	AccountID    string
	ColEnc       string
	Collection   discovery.Collection
	AccountLabel string
	From         string
	To           string
	Days         []agendaDay
	ReadOnly     bool
	Empty        bool
	NoCalendars  bool
	PrintDate    string
}

type agendaDay struct {
	Date   string
	Label  string
	Events []agendaRow
}

type agendaRow struct {
	UID, TimeLabel, Summary, Location string
	AllDay                            bool
}

// CalendarHome redirects to the first calendar or shows an empty state.
func (s *Server) CalendarHome(w http.ResponseWriter, r *http.Request) {
	sess := SessionFrom(r)
	accounts, err := s.Store.ListDAVAccounts(sess.UserID, sess.DEK())
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	refs := calendars(accounts)
	if len(refs) == 0 {
		v := s.View(r, "Agenda")
		v.Data = agendaView{Calendars: refs, NoCalendars: true}
		s.Render(w, "agenda.html", v)
		return
	}
	ref := refs[0]
	http.Redirect(w, r, s.Path("/app/calendar/"+ref.AccountID+"/"+ref.ColEnc), http.StatusSeeOther)
}

// CalendarAgenda renders occurrences in an inclusive local-date range.
func (s *Server) CalendarAgenda(w http.ResponseWriter, r *http.Request) {
	accountID, colEnc := r.PathValue("account"), r.PathValue("col")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	view, err := s.buildAgenda(r.Context(), sess, accountID, collection, colEnc, r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		v := s.View(r, "Agenda")
		v.Error = userFacingDAVError(err)
		v.Data = agendaView{Calendars: s.listCalendars(sess), AccountID: accountID, ColEnc: colEnc}
		s.RenderStatus(w, http.StatusBadRequest, "agenda.html", v)
		return
	}
	v := s.View(r, "Agenda")
	v.Data = view
	s.Render(w, "agenda.html", v)
}

func (s *Server) listCalendars(sess *session.Session) []calendarRef {
	accounts, err := s.Store.ListDAVAccounts(sess.UserID, sess.DEK())
	if err != nil {
		return nil
	}
	return calendars(accounts)
}

func (s *Server) buildAgenda(ctx context.Context, sess *session.Session, accountID, collection, colEnc, fromText, toText string) (agendaView, error) {
	loc := s.timezone()
	today := time.Now().In(loc)
	from, err := parseAgendaDate(fromText, time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc))
	if err != nil {
		return agendaView{}, err
	}
	to, err := parseAgendaDate(toText, from.AddDate(0, 0, 14))
	if err != nil || to.Before(from) {
		return agendaView{}, fmt.Errorf("agenda end date must not precede start date")
	}
	p, acc, err := s.calendarProvider(sess, accountID)
	if err != nil {
		return agendaView{}, err
	}
	col, err := findCalendar(acc, collection)
	if err != nil {
		return agendaView{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	result, err := p.Query(ctx, normalizeCollectionPath(col.Path), from, to.AddDate(0, 0, 1))
	if err != nil {
		return agendaView{}, err
	}
	byDate := make(map[string][]agendaRow)
	for _, occ := range result.Occurrences {
		start := occ.Start.In(loc)
		date := start.Format("2006-01-02")
		summary := strings.TrimSpace(occ.Summary)
		if summary == "" {
			summary = "(untitled event)"
		}
		label := "All day"
		if !occ.AllDay {
			label = start.Format("15:04")
			if !occ.End.IsZero() {
				label += "–" + occ.End.In(loc).Format("15:04")
			}
		}
		uid := occ.UID
		if uid == "" {
			uid = uidFromCalendarPath(occ.Path)
		}
		byDate[date] = append(byDate[date], agendaRow{
			UID: uid, TimeLabel: label, Summary: summary, Location: occ.Location, AllDay: occ.AllDay,
		})
	}
	keys := make([]string, 0, len(byDate))
	for date := range byDate {
		keys = append(keys, date)
	}
	sort.Strings(keys)
	days := make([]agendaDay, 0, len(keys))
	for _, date := range keys {
		day, _ := time.ParseInLocation("2006-01-02", date, loc)
		days = append(days, agendaDay{Date: date, Label: day.Format("Monday, 2 January 2006"), Events: byDate[date]})
	}
	return agendaView{
		Calendars: s.listCalendars(sess), AccountID: accountID, ColEnc: colEnc,
		Collection: col, AccountLabel: accountLabel(*acc),
		From: from.Format("2006-01-02"), To: to.Format("2006-01-02"),
		Days: days, ReadOnly: col.ReadOnly, Empty: len(days) == 0,
		PrintDate: time.Now().UTC().Format("2006-01-02 15:04 UTC"),
	}, nil
}

func parseAgendaDate(text string, fallback time.Time) (time.Time, error) {
	if strings.TrimSpace(text) == "" {
		return fallback, nil
	}
	t, err := time.ParseInLocation("2006-01-02", text, fallback.Location())
	if err != nil {
		return time.Time{}, fmt.Errorf("dates must use YYYY-MM-DD")
	}
	return t, nil
}
