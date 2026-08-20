// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/fanout"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

// ContactPerson is the full-width «everything about this person» screen (§1.8).
func (s *Server) ContactPerson(w http.ResponseWriter, r *http.Request) {
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
	req := findRequest{
		Mode: modeTimeline, Account: accountID,
		Collection: collection, UID: uid,
		Tab: strings.TrimSpace(r.URL.Query().Get("tab")),
	}
	s.startPersonFind(w, r, req, card)
}

func (s *Server) startPersonFind(w http.ResponseWriter, r *http.Request, req findRequest, card contactCardView) {
	sess := SessionFrom(r)
	view := findView{
		Request: req, Mode: modeTimeline, Title: card.Contact.DisplayName(),
		Subject: card.Contact.DisplayName(),
		UseSSE:  s.Progress.SSE(), PollMillis: s.pollMillis(), Base: s.BasePath,
		Person: personPanel{
			AccountID: card.AccountID, ColEnc: card.ColEnc, UID: card.UID,
			Contact: card.Contact, PhotoURL: card.PhotoURL,
			AccountLabel: card.AccountLabel, Collection: card.Collection,
			ReadOnly: card.ReadOnly,
			EditURL:  s.Path("/app/contacts/" + card.AccountID + "/" + card.ColEnc + "/" + urlPathEscape(card.UID) + "/edit"),
			NoteURL:  s.contactNoteAboutURL(sess, card),
			EventURL: s.contactEventURL(sess, card),
		},
	}
	view.SourcesURL = s.sourcesURL(modeTimeline)
	if rail, railErr := s.buildSectionRail(sess, findRequest{Mode: modeTimeline}, "", ""); railErr == nil {
		rail.RailTitle = "Where to look"
		rail.Mode = modeTimeline
		rail.SourcesURL = s.sourcesURL(modeTimeline)
		rail.AllURL = s.Path("/app/contacts/" + card.AccountID + "/" + card.ColEnc + "/" + urlPathEscape(card.UID))
		rail.AllActive = true
		view.SectionRail = rail
	}
	if s.Fanout == nil {
		view.Unusable = "Cross-source polling is not configured on this instance."
		s.renderFind(w, r, "person.html", view)
		return
	}
	rows, err := s.findSources(sess, req)
	if err != nil {
		view.Unusable = userFacingDAVError(err)
		s.renderFind(w, r, "person.html", view)
		return
	}
	view.Sources = rows
	if subject, subjErr := s.timelineSubject(r.Context(), sess, req); subjErr == nil {
		req.Query = strings.Join(subject.terms, "\n")
		view.Request = req
	} else {
		view.Unusable = userFacingDAVError(subjErr)
		s.renderFind(w, r, "person.html", view)
		return
	}
	selected := selectedRows(rows)
	if len(selected) == 0 {
		view.NoSources = true
		s.renderFind(w, r, "person.html", view)
		return
	}
	query, err := s.findQuery(sess, req)
	if err != nil {
		view.Unusable = userFacingDAVError(err)
		s.renderFind(w, r, "person.html", view)
		return
	}
	task, err := s.Fanout.Start(sess.ID, fanoutSources(selected), query, s.fanoutOptions())
	if err != nil {
		if errors.Is(err, fanout.ErrNoSources) {
			view.NoSources = true
		} else {
			view.Unusable = userFacingDAVError(err)
		}
		s.renderFind(w, r, "person.html", view)
		return
	}
	view.TaskID = task.ID
	s.fillFindURLs(&view, req, task.ID)
	s.fillResults(r, &view, req, task)
	s.renderFind(w, r, "person.html", view)
}

func (s *Server) contactNoteAboutURL(sess *session.Session, card contactCardView) string {
	rows, err := s.noteSources(sess)
	if err != nil {
		return ""
	}
	target, ok := s.defaultCollection(sess, account.ViewNotes, rows)
	if !ok {
		return ""
	}
	values := url.Values{
		"related": {card.UID},
		"summary": {"Notes: " + card.Contact.DisplayName()},
	}
	return s.Path("/app/notes/"+target.AccountID+"/"+target.ColEnc+"/new") + "?" + values.Encode()
}

func (s *Server) contactEventURL(sess *session.Session, card contactCardView) string {
	target, ok := s.defaultCollection(sess, account.ViewAgenda, s.calendarSourceRows(sess))
	if !ok {
		return ""
	}
	values := url.Values{"summary": {"Meeting with " + card.Contact.DisplayName()}}
	if emails := card.Contact.NormalizedEmails(); len(emails) > 0 {
		values.Set("attendee", "mailto:"+emails[0])
	}
	return s.Path("/app/calendar/"+target.AccountID+"/"+target.ColEnc+"/new") + "?" + values.Encode()
}
