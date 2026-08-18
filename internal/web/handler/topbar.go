// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"net/http"
	"net/url"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

// createMenuItem is one entry in the header create menu.
type createMenuItem struct {
	Label     string
	URL       string
	Shortcut  string
	Separator bool
	Disabled  bool
	QuickNote bool
}

// topbarView is shared shell chrome: back URL for sheets and create links.
type topbarView struct {
	Back         string
	QuickNoteURL string
	Create       []createMenuItem
}

func (s *Server) buildTopbar(r *http.Request, sess *session.Session) topbarView {
	back := r.URL.Path
	if q := r.URL.RawQuery; q != "" {
		back += "?" + q
	}
	if back == "" || back == "/" {
		back = s.Path("/app/calendar")
	}

	menu := []createMenuItem{
		{Label: "Note", Shortcut: "N", QuickNote: true},
		{Separator: true},
	}
	menu = append(menu, s.createKindItem(sess, "Contact", "C", account.ViewContacts, s.contactSourceRows(sess), "contacts", "new"))
	menu = append(menu, s.createKindItem(sess, "Event", "E", account.ViewAgenda, s.calendarSourceRows(sess), "calendar", "new"))
	menu = append(menu, s.createKindItem(sess, "Task", "T", account.ViewTasks, s.taskSources(sess), "tasks", "new"))
	menu = append(menu, createMenuItem{Separator: true})
	if url := s.defaultSectionURL(sess, account.ViewFiles, s.fileSourceRows(sess), "files"); url != "" {
		menu = append(menu,
			createMenuItem{Label: "Upload file…", URL: url},
			createMenuItem{Label: "New folder…", URL: url},
		)
	}
	return topbarView{
		Back:         back,
		QuickNoteURL: s.Path("/app/notes/quick") + "?back=" + url.QueryEscape(back),
		Create:       menu,
	}
}

func (s *Server) createKindItem(sess *session.Session, label, shortcut, view string, rows []sourceRow, section, action string) createMenuItem {
	item := createMenuItem{Label: label, Shortcut: shortcut}
	if url := s.defaultSectionURL(sess, view, rows, section, action); url != "" {
		item.URL = url
	} else {
		item.Disabled = true
	}
	return item
}

func (s *Server) defaultSectionURL(sess *session.Session, view string, rows []sourceRow, section string, action ...string) string {
	target, ok := s.defaultCollection(sess, view, rows)
	if !ok {
		return ""
	}
	path := "/app/" + section + "/" + target.AccountID + "/" + target.ColEnc
	if len(action) > 0 && action[0] != "" {
		path += "/" + action[0]
	}
	return s.Path(path)
}

func (s *Server) contactSourceRows(sess *session.Session) []sourceRow {
	books := s.listBooks(sess)
	rows := make([]sourceRow, 0, len(books))
	for _, book := range books {
		if book.Collection.ReadOnly {
			continue
		}
		rows = append(rows, sourceRow{
			AccountID: book.AccountID,
			ColEnc:    book.ColEnc,
			Path:      book.Collection.Path,
			Name:      book.Collection.DisplayName,
			ReadOnly:  book.Collection.ReadOnly,
		})
	}
	return rows
}

func (s *Server) calendarSourceRows(sess *session.Session) []sourceRow {
	rows, err := s.collectionsOfKind(sess, discovery.KindCalendar, account.ViewAgenda, dav.CompEvent)
	if err != nil {
		return nil
	}
	writable := rows[:0]
	for _, row := range rows {
		if !row.ReadOnly {
			writable = append(writable, row)
		}
	}
	return writable
}

func (s *Server) fileSourceRows(sess *session.Session) []sourceRow {
	rows, err := s.collectionsOfKind(sess, discovery.KindFiles, account.ViewFiles, "")
	if err != nil {
		return nil
	}
	writable := rows[:0]
	for _, row := range rows {
		if !row.ReadOnly {
			writable = append(writable, row)
		}
	}
	return writable
}
