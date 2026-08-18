// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	rrule "github.com/teambition/rrule-go"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/model"
	"gitea.mixdep.ru/mix/carrel/internal/provider/calendar"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

type eventCardView struct {
	Calendars    []calendarRef
	AccountID    string
	ColEnc       string
	Collection   discovery.Collection
	AccountLabel string
	UID          string
	ETag         string
	Event        model.Event
	Form         eventForm
	Related      []relatedRow
	// Attachments are the ATTACH links of §23.10, each with the proxy that
	// opens it.
	Attachments []attachmentRow
	// CanAttach says a folder for attachments has been chosen, which is what
	// §23.10 asks for once and never again.
	CanAttach bool
	// Section is the URL segment the shared attachments block posts to.
	Section string
	// NoteURL opens a new note already linked to this event (§23.9).
	NoteURL   string
	ReadOnly  bool
	IsNew     bool
	PrintDate string
}

type eventForm struct {
	Summary, Description, Location, Status string
	StartDate, StartTime, EndDate, EndTime string
	AllDay                                 bool
	Categories                             string
	Attendee                               string
	RRuleFreq, RRuleInterval, RRuleUntil   string
	RRuleCount, RRule                      string
	RRuleByDay                             map[string]bool
}

func (s *Server) EventNew(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.eventSave(w, r, true)
		return
	}
	accountID, colEnc := r.PathValue("account"), r.PathValue("col")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	_, acc, err := s.calendarProvider(sess, accountID)
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
	now := time.Now().In(s.timezone()).Add(time.Hour).Truncate(time.Hour)
	form := eventForm{
		StartDate: now.Format("2006-01-02"), StartTime: now.Format("15:04"),
		EndDate: now.Add(time.Hour).Format("2006-01-02"), EndTime: now.Add(time.Hour).Format("15:04"),
		RRuleFreq: "NONE", RRuleInterval: "1", RRuleByDay: map[string]bool{},
		Attendee: strings.TrimSpace(r.URL.Query().Get("attendee")),
	}
	if summary := strings.TrimSpace(r.URL.Query().Get("summary")); summary != "" {
		form.Summary = summary
	}
	v := s.View(r, "New event")
	v.Data = eventCardView{
		Calendars: s.listCalendars(sess), AccountID: accountID, ColEnc: colEnc,
		Collection: col, AccountLabel: accountLabel(*acc), IsNew: true, Form: form,
	}
	s.Render(w, "event.html", v)
}

func (s *Server) EventCard(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.eventSave(w, r, false)
		return
	}
	accountID, colEnc, uid := r.PathValue("account"), r.PathValue("col"), r.PathValue("uid")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil || uid == "" {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	card, err := s.loadEventCard(r.Context(), sess, accountID, collection, colEnc, uid)
	if err != nil {
		s.renderCalendarError(w, r, err, accountID, colEnc)
		return
	}
	v := s.View(r, card.Event.DisplayTitle())
	v.Notice = strings.TrimSpace(r.URL.Query().Get("notice"))
	v.Data = card
	s.Render(w, "event.html", v)
}

func (s *Server) eventSave(w http.ResponseWriter, r *http.Request, isNew bool) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	accountID, colEnc, uid := r.PathValue("account"), r.PathValue("col"), r.PathValue("uid")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if r.PostFormValue(fieldAction) == "delete" {
		s.eventDelete(w, r, accountID, collection, colEnc, uid)
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
	form := parseEventForm(r)
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	var obj *model.Object
	if isNew {
		uid, err = model.NewUID()
		if err == nil {
			obj, err = model.NewEvent(uid)
		}
	} else {
		obj, err = p.Get(ctx, collection, calendarObjectPath(collection, uid))
		if err == nil && strings.TrimSpace(r.PostFormValue("etag")) != "" {
			obj.ETag = strings.TrimSpace(r.PostFormValue("etag"))
		}
	}
	if err != nil {
		s.renderCalendarError(w, r, err, accountID, colEnc)
		return
	}
	patch, err := form.toPatch(s.timezone())
	if err == nil {
		err = obj.Apply(patch)
	}
	if err != nil {
		v := s.View(r, map[bool]string{true: "New event", false: "Event"}[isNew])
		v.Error = capitalize(err.Error())
		v.Data = s.eventCardFromObject(sess, accountID, colEnc, col, *acc, obj, form, isNew)
		s.RenderStatus(w, http.StatusBadRequest, "event.html", v)
		return
	}
	var result *calendar.WriteResult
	if isNew {
		obj.Path = calendarObjectPath(collection, uid)
		result, err = p.Create(ctx, collection, obj)
	} else {
		result, err = p.Update(ctx, collection, obj)
	}
	if err != nil {
		if calendar.IsConflict(err) {
			s.showCalendarConflict(w, r, sess, accountID, collection, colEnc, uid, err)
			return
		}
		s.renderCalendarError(w, r, err, accountID, colEnc)
		return
	}
	notice := "Event saved."
	if result != nil && result.ReportLoss && !result.Loss.Empty() {
		notice = "Event saved, but " + result.Loss.String() + "."
	}
	s.redirectNotice(w, r, s.Path("/app/calendar/"+accountID+"/"+colEnc+"/"+urlPathEscape(uid)), notice)
}

func (s *Server) eventDelete(w http.ResponseWriter, r *http.Request, accountID, collection, colEnc, uid string) {
	sess := SessionFrom(r)
	p, acc, err := s.calendarProvider(sess, accountID)
	if err != nil {
		s.renderCalendarError(w, r, err, accountID, colEnc)
		return
	}
	col, err := findCalendar(acc, collection)
	if err != nil || col.ReadOnly {
		http.Error(w, "this calendar is read-only", http.StatusForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	err = p.Delete(ctx, normalizeCollectionPath(col.Path), calendarObjectPath(col.Path, uid), strings.TrimSpace(r.PostFormValue("etag")))
	if err != nil {
		if calendar.IsConflict(err) {
			s.showCalendarConflict(w, r, sess, accountID, col.Path, colEnc, uid, err)
			return
		}
		s.renderCalendarError(w, r, err, accountID, colEnc)
		return
	}
	http.Redirect(w, r, s.Path("/app/calendar/"+accountID+"/"+colEnc), http.StatusSeeOther)
}

func (s *Server) loadEventCard(ctx context.Context, sess *session.Session, accountID, collection, colEnc, uid string) (eventCardView, error) {
	p, acc, err := s.calendarProvider(sess, accountID)
	if err != nil {
		return eventCardView{}, err
	}
	col, err := findCalendar(acc, collection)
	if err != nil {
		return eventCardView{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	obj, err := p.Get(ctx, normalizeCollectionPath(col.Path), calendarObjectPath(col.Path, uid))
	if err != nil {
		return eventCardView{}, err
	}
	ev, err := obj.Event(s.timezone())
	if err != nil {
		return eventCardView{}, err
	}
	card := s.eventCardFromObject(sess, accountID, colEnc, col, *acc, obj, formFromEvent(ev), false)
	card.Related = s.resolveRelated(ctx, p, accountID, colEnc, normalizeCollectionPath(col.Path), ev.Related)
	// The minutes of a meeting are written from the meeting (§23.9): the link
	// carries the event's identity and date so the note arrives already tied to
	// it, with nothing to copy by hand.
	card.NoteURL = s.noteAboutURL(accountID, colEnc, ev)
	return card, nil
}

// noteAboutURL is the "write a note about this" link of an event card. It points
// at the notes collection §23.9 files into by default, and falls back to the
// event's own calendar when there is no separate one.
func (s *Server) noteAboutURL(accountID, colEnc string, ev model.Event) string {
	values := url.Values{"related": {ev.UID}, "summary": {"Notes: " + ev.DisplayTitle()}}
	if !ev.Start.IsZero() {
		values.Set("date", ev.Start.In(s.timezone()).Format("2006-01-02"))
	}
	return s.Path("/app/notes/"+accountID+"/"+colEnc+"/new") + "?" + values.Encode()
}

func (s *Server) eventCardFromObject(sess *session.Session, accountID, colEnc string, col discovery.Collection, acc account.Account, obj *model.Object, form eventForm, isNew bool) eventCardView {
	var ev model.Event
	if obj != nil {
		ev, _ = obj.Event(s.timezone())
	}
	card := eventCardView{
		Calendars: s.listCalendars(sess), AccountID: accountID, ColEnc: colEnc,
		Collection: col, AccountLabel: accountLabel(acc), UID: ev.UID,
		ETag: obj.ETag, Event: ev, Form: form, ReadOnly: col.ReadOnly, IsNew: isNew,
		Section:   sectionCalendar.Path,
		PrintDate: time.Now().UTC().Format("2006-01-02 15:04 UTC"),
	}
	if !isNew {
		card.Attachments = s.attachmentRows(sess, sectionCalendar, accountID, colEnc, ev.UID, ev.Attachments)
		_, card.CanAttach = s.attachmentTarget(sess)
	}
	return card
}

func parseEventForm(r *http.Request) eventForm {
	f := eventForm{
		Summary: strings.TrimSpace(r.PostFormValue("summary")), Description: strings.TrimSpace(r.PostFormValue("description")),
		Location: strings.TrimSpace(r.PostFormValue("location")), Status: strings.TrimSpace(r.PostFormValue("status")),
		StartDate: r.PostFormValue("start_date"), StartTime: r.PostFormValue("start_time"),
		EndDate: r.PostFormValue("end_date"), EndTime: r.PostFormValue("end_time"),
		AllDay: r.PostFormValue("all_day") != "", Categories: strings.TrimSpace(r.PostFormValue("categories")),
		RRuleFreq:     strings.ToUpper(strings.TrimSpace(r.PostFormValue("rrule_freq"))),
		RRuleInterval: strings.TrimSpace(r.PostFormValue("rrule_interval")),
		RRuleUntil:    strings.TrimSpace(r.PostFormValue("rrule_until")), RRuleCount: strings.TrimSpace(r.PostFormValue("rrule_count")),
		RRule: strings.TrimSpace(r.PostFormValue("rrule")), RRuleByDay: map[string]bool{},
		Attendee: strings.TrimSpace(r.PostFormValue("attendee")),
	}
	for _, day := range r.PostForm["rrule_byday"] {
		f.RRuleByDay[strings.ToUpper(day)] = true
	}
	return f
}

func (f eventForm) toPatch(loc *time.Location) (*model.Patch, error) {
	if loc == nil {
		loc = time.Local
	}
	p := &model.Patch{}
	setOrRemove := func(name, value string) {
		if strings.TrimSpace(value) == "" {
			p.Remove(name)
		} else {
			p.SetText(name, value)
		}
	}
	setOrRemove(ical.PropSummary, f.Summary)
	setOrRemove(ical.PropDescription, f.Description)
	setOrRemove(ical.PropLocation, f.Location)
	setOrRemove(ical.PropStatus, f.Status)
	if cats := cleanCommaList(f.Categories); cats == "" {
		p.Remove(ical.PropCategories)
	} else {
		p.SetText(ical.PropCategories, cats)
	}
	start, end, err := f.eventTimes(loc)
	if err != nil {
		return nil, err
	}
	if f.AllDay {
		p.Set(ical.PropDateTimeStart, model.Value{Text: start.Format("20060102"), Params: map[string][]string{"VALUE": {"DATE"}}})
		p.Set(ical.PropDateTimeEnd, model.Value{Text: end.Format("20060102"), Params: map[string][]string{"VALUE": {"DATE"}}})
	} else {
		params := map[string][]string{"TZID": {loc.String()}}
		p.Set(ical.PropDateTimeStart, model.Value{Text: start.Format("20060102T150405"), Params: params})
		p.Set(ical.PropDateTimeEnd, model.Value{Text: end.Format("20060102T150405"), Params: params})
	}
	rrule, err := f.buildRRule()
	if err != nil {
		return nil, err
	}
	if rrule == "" {
		p.Remove(ical.PropRecurrenceRule)
	} else {
		p.SetText(ical.PropRecurrenceRule, rrule)
	}
	p.SetText(ical.PropDateTimeStamp, time.Now().UTC().Format("20060102T150405Z"))
	if attendee := strings.TrimSpace(f.Attendee); attendee != "" {
		p.Set(ical.PropAttendee, model.Value{
			Text: attendee,
			Params: map[string][]string{
				"CUTYPE":   {"INDIVIDUAL"},
				"ROLE":     {"REQ-PARTICIPANT"},
				"PARTSTAT": {"NEEDS-ACTION"},
				"RSVP":     {"TRUE"},
			},
		})
	}
	return p, nil
}

func (f eventForm) eventTimes(loc *time.Location) (time.Time, time.Time, error) {
	if f.AllDay {
		start, err := time.ParseInLocation("2006-01-02", f.StartDate, loc)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("start date is required")
		}
		end, err := time.ParseInLocation("2006-01-02", f.EndDate, loc)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("end date is required")
		}
		if !end.After(start) {
			return time.Time{}, time.Time{}, fmt.Errorf("end date must be after start date")
		}
		return start, end, nil
	}
	start, err := time.ParseInLocation("2006-01-02 15:04", f.StartDate+" "+f.StartTime, loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("start date and time are required")
	}
	end, err := time.ParseInLocation("2006-01-02 15:04", f.EndDate+" "+f.EndTime, loc)
	if err != nil || !end.After(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("end must be after start")
	}
	return start, end, nil
}

func (f eventForm) buildRRule() (string, error) {
	if f.RRule != "" {
		rule := strings.TrimPrefix(strings.ToUpper(f.RRule), "RRULE:")
		if _, err := rrule.StrToROption(rule); err != nil {
			return "", fmt.Errorf("invalid RRULE: %w", err)
		}
		return rule, nil
	}
	freq := strings.ToUpper(f.RRuleFreq)
	if freq == "" || freq == "NONE" {
		return "", nil
	}
	switch freq {
	case "DAILY", "WEEKLY", "MONTHLY", "YEARLY":
	default:
		return "", fmt.Errorf("invalid recurrence frequency")
	}
	interval := 1
	if f.RRuleInterval != "" {
		var err error
		interval, err = strconv.Atoi(f.RRuleInterval)
		if err != nil || interval < 1 {
			return "", fmt.Errorf("recurrence interval must be at least 1")
		}
	}
	parts := []string{"FREQ=" + freq, fmt.Sprintf("INTERVAL=%d", interval)}
	if freq == "WEEKLY" {
		var days []string
		for _, d := range []string{"MO", "TU", "WE", "TH", "FR", "SA", "SU"} {
			if f.RRuleByDay[d] {
				days = append(days, d)
			}
		}
		if len(days) > 0 {
			parts = append(parts, "BYDAY="+strings.Join(days, ","))
		}
	}
	if f.RRuleUntil != "" {
		t, err := time.Parse("2006-01-02", f.RRuleUntil)
		if err != nil {
			return "", fmt.Errorf("recurrence until date is invalid")
		}
		parts = append(parts, "UNTIL="+t.Format("20060102"))
	} else if f.RRuleCount != "" {
		count, err := strconv.Atoi(f.RRuleCount)
		if err != nil || count < 1 {
			return "", fmt.Errorf("recurrence count must be at least 1")
		}
		parts = append(parts, fmt.Sprintf("COUNT=%d", count))
	}
	rule := strings.Join(parts, ";")
	if _, err := rrule.StrToROption(rule); err != nil {
		return "", fmt.Errorf("invalid recurrence: %w", err)
	}
	return rule, nil
}

func formFromEvent(ev model.Event) eventForm {
	f := eventForm{
		Summary: ev.Summary, Description: ev.Description, Location: ev.Location, Status: ev.Status,
		StartDate: ev.Start.Format("2006-01-02"), StartTime: ev.Start.Format("15:04"),
		EndDate: ev.End.Format("2006-01-02"), EndTime: ev.End.Format("15:04"), AllDay: ev.AllDay,
		Categories: strings.Join(ev.Categories, ", "),
		RRuleFreq:  "NONE", RRuleInterval: "1", RRuleByDay: map[string]bool{},
	}
	if ev.RRule != "" && !populateStructuredRRule(&f, ev.RRule) {
		f.RRule = ev.RRule
	}
	return f
}

func populateStructuredRRule(f *eventForm, rule string) bool {
	known := map[string]bool{"FREQ": true, "INTERVAL": true, "BYDAY": true, "UNTIL": true, "COUNT": true}
	values := map[string]string{}
	for _, part := range strings.Split(strings.TrimPrefix(strings.ToUpper(rule), "RRULE:"), ";") {
		key, value, ok := strings.Cut(part, "=")
		if !ok || !known[key] {
			return false
		}
		values[key] = value
	}
	freq := values["FREQ"]
	switch freq {
	case "DAILY", "WEEKLY", "MONTHLY", "YEARLY":
	default:
		return false
	}
	f.RRuleFreq = freq
	if values["INTERVAL"] != "" {
		f.RRuleInterval = values["INTERVAL"]
	}
	for _, day := range strings.Split(values["BYDAY"], ",") {
		switch day {
		case "MO", "TU", "WE", "TH", "FR", "SA", "SU":
			f.RRuleByDay[day] = true
		case "":
		default:
			return false
		}
	}
	if until := values["UNTIL"]; until != "" {
		t, err := time.Parse("20060102", strings.TrimSuffix(until, "T000000Z"))
		if err != nil {
			return false
		}
		f.RRuleUntil = t.Format("2006-01-02")
	}
	f.RRuleCount = values["COUNT"]
	return true
}

func cleanCommaList(value string) string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return strings.Join(out, ",")
}

func (s *Server) renderCalendarError(w http.ResponseWriter, r *http.Request, err error, accountID, colEnc string) {
	v := s.View(r, "Agenda")
	v.Error = userFacingDAVError(err)
	v.Data = agendaView{Calendars: s.listCalendars(SessionFrom(r)), AccountID: accountID, ColEnc: colEnc}
	s.RenderStatus(w, http.StatusBadRequest, "agenda.html", v)
}
