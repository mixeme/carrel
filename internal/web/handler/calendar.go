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
	// EventCount is the total occurrences across every day, for the page-bar
	// counter of 2.6.B5 — the number already loaded, not the count in the
	// collection, which nothing here asks the server for.
	EventCount int
	// Segment, WeekURL and MonthURL back the Week/Month/Range presets of
	// 2.6.B6, layered on the same From/To the form always used. Segment is
	// "week" or "month" when From/To exactly match that preset's computed
	// range, "range" otherwise — which is also what a custom From/To lands
	// on, so Range reads as "on, and here is the form" rather than "off".
	Segment     string
	WeekURL     string
	MonthURL    string
	ReadOnly    bool
	Empty       bool
	NoCalendars bool
	PrintDate   string
	SectionRail sectionRail
	Mode        findMode
}

type agendaDay struct {
	Date   string
	Label  string
	Events []agendaRow
}

type agendaRow struct {
	UID, TimeLabel, TimeZoneLabel, Summary, Location string
	AllDay                                           bool
}

// CalendarHome shows every ticked calendar at once, or an empty state.
func (s *Server) CalendarHome(w http.ResponseWriter, r *http.Request) {
	sess := SessionFrom(r)
	refs := s.listCalendars(sess)
	if len(refs) == 0 {
		v := s.View(r, "Agenda")
		v.Data = agendaView{Calendars: refs, NoCalendars: true}
		s.Render(w, "agenda.html", v)
		return
	}
	req := parseFindRequest(r)
	req.Mode = modeTime
	s.sectionFind(w, r, req, "agenda.html")
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
		data := agendaView{Calendars: s.listCalendars(sess), AccountID: accountID, ColEnc: colEnc}
		s.fillAgendaRail(sess, &data, accountID, colEnc)
		v.Data = data
		s.RenderStatus(w, http.StatusBadRequest, "agenda.html", v)
		return
	}
	s.fillAgendaRail(sess, &view, accountID, colEnc)
	v := s.View(r, "Agenda")
	v.Data = view
	s.Render(w, "agenda.html", v)
}

func (s *Server) fillAgendaRail(sess *session.Session, view *agendaView, accountID, colEnc string) {
	if rail, err := s.buildSectionRail(sess, findRequest{Mode: modeTime}, accountID, colEnc); err == nil {
		view.SectionRail = rail
	}
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
			UID: uid, TimeLabel: label, TimeZoneLabel: sourceTZLabel(occ.StartTZID, loc),
			Summary: summary, Location: occ.Location, AllDay: occ.AllDay,
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
	weekStart, weekEnd := weekRange(today)
	monthStart, monthEnd := monthRange(today)
	base := s.Path("/app/calendar/" + accountID + "/" + colEnc)
	segment := "range"
	if sameDate(from, weekStart) && sameDate(to, weekEnd) {
		segment = "week"
	} else if sameDate(from, monthStart) && sameDate(to, monthEnd) {
		segment = "month"
	}

	return agendaView{
		Calendars: s.listCalendars(sess), AccountID: accountID, ColEnc: colEnc,
		Collection: col, AccountLabel: accountLabel(*acc),
		From: from.Format("2006-01-02"), To: to.Format("2006-01-02"),
		Days: days, EventCount: len(result.Occurrences), ReadOnly: col.ReadOnly, Empty: len(days) == 0,
		Segment:   segment,
		WeekURL:   base + "?from=" + weekStart.Format("2006-01-02") + "&to=" + weekEnd.Format("2006-01-02"),
		MonthURL:  base + "?from=" + monthStart.Format("2006-01-02") + "&to=" + monthEnd.Format("2006-01-02"),
		PrintDate: time.Now().UTC().Format("2006-01-02 15:04 UTC"),
	}, nil
}

// weekRange is the Monday-to-Sunday week containing t, for the Week preset
// of 2.6.B6.
func weekRange(t time.Time) (time.Time, time.Time) {
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	offset := (int(day.Weekday()) + 6) % 7 // days since Monday; Sunday is 6
	start := day.AddDate(0, 0, -offset)
	return start, start.AddDate(0, 0, 6)
}

// monthRange is the first and last day of t's month, for the Month preset of
// 2.6.B6.
func monthRange(t time.Time) (time.Time, time.Time) {
	start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	end := start.AddDate(0, 1, -1)
	return start, end
}

func sameDate(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
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
