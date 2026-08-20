// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"net/http"
	"strings"

	"gitea.mixdep.ru/mix/carrel/internal/session"
	"gitea.mixdep.ru/mix/carrel/internal/store"
)

const (
	settingsSectionAccount     = "account"
	settingsSectionConnections = "connections"
	settingsSectionAttachments = "attachments"
	settingsSectionAppearance  = "appearance"
)

// settingsView is shared chrome for the settings rail.
type settingsView struct {
	Section string
	appView
}

func (s *Server) settingsFrame(r *http.Request, title, section string) View {
	v := s.View(r, title)
	v.ShellLayout = "settings"
	s.firstLoginEscrowNotice(r, &v)
	return v
}

func (s *Server) renderSettings(w http.ResponseWriter, status int, name string, v View, data settingsView) {
	if data.Section == "" {
		data.Section = sectionFromTemplate(name)
	}
	v.Data = data
	s.RenderStatus(w, status, name, v)
}

func sectionFromTemplate(name string) string {
	switch name {
	case "settings_account.html":
		return settingsSectionAccount
	case "settings_connections.html":
		return settingsSectionConnections
	case "settings_attachments.html":
		return settingsSectionAttachments
	case "settings_appearance.html":
		return settingsSectionAppearance
	default:
		return ""
	}
}

// AppHome redirects the profile root to Connections; POST still handles forms
// posted to the legacy /app/ URL.
func (s *Server) AppHome(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.appSubmit(w, r)
		return
	}
	http.Redirect(w, r, s.Path("/app/settings/connections"), http.StatusSeeOther)
}

func (s *Server) SettingsConnections(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.settingsConnectionsSubmit(w, r)
		return
	}
	v := s.settingsFrame(r, "Connections", settingsSectionConnections)
	data := s.buildAppView(r)
	if editID := strings.TrimSpace(r.URL.Query().Get("edit")); editID != "" {
		s.fillEditForm(r, &data, editID)
	}
	v.Data = settingsView{Section: settingsSectionConnections, appView: data}
	s.Render(w, "settings_connections.html", v)
}

func (s *Server) settingsConnectionsSubmit(w http.ResponseWriter, r *http.Request) {
	s.appSubmitTo(w, r, "settings_connections.html", settingsSectionConnections)
}

func (s *Server) SettingsAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.settingsAccountSubmit(w, r)
		return
	}
	v := s.settingsFrame(r, "Account", settingsSectionAccount)
	v.Data = settingsView{Section: settingsSectionAccount, appView: s.buildAppView(r)}
	s.Render(w, "settings_account.html", v)
}

func (s *Server) SettingsAttachments(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.settingsAttachmentsSubmit(w, r)
		return
	}
	v := s.settingsFrame(r, "Attachments", settingsSectionAttachments)
	v.Data = settingsView{Section: settingsSectionAttachments, appView: s.buildAppView(r)}
	s.Render(w, "settings_attachments.html", v)
}

func (s *Server) settingsAttachmentsSubmit(w http.ResponseWriter, r *http.Request) {
	s.appSubmitTo(w, r, "settings_attachments.html", settingsSectionAttachments)
}

func (s *Server) SettingsAppearance(w http.ResponseWriter, r *http.Request) {
	v := s.settingsFrame(r, "Appearance", settingsSectionAppearance)
	v.Data = settingsView{Section: settingsSectionAppearance}
	s.Render(w, "settings_appearance.html", v)
}

func (s *Server) appSubmitTo(w http.ResponseWriter, r *http.Request, template, section string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sess := SessionFrom(r)
	actor := storeActorFrom(r, sess)

	var (
		data   appView
		err    error
		notice string
	)
	switch r.PostFormValue(fieldAction) {
	case "connect_dav":
		data, err = s.appConnectDAV(r, actor)
		if err == nil {
			notice = "Account connected."
		}
	case "delete_dav":
		data, err = s.appDeleteDAV(r, actor)
		if err == nil {
			notice = "Account removed."
		}
	case "test_dav":
		data, err = s.appTestDAV(r, actor)
		if err == nil {
			notice = "Connection test succeeded."
		}
	case "update_dav":
		data, err = s.appUpdateDAV(r, actor)
		if err == nil {
			notice = "Connection updated."
		}
	case "save_attachments":
		data, err = s.appSaveAttachments(r)
		if err == nil {
			notice = "Attachments will go in that folder."
		}
	case "refresh_cache":
		s.appRefresh(w, r)
		return
	default:
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	v := s.settingsFrame(r, titleForSettingsSection(section), section)
	if err != nil {
		v.Error = userFacingDAVError(err)
	}
	if notice != "" {
		v.Notice = notice
	}
	status := http.StatusOK
	if err != nil {
		status = http.StatusBadRequest
	}
	s.renderSettings(w, status, template, v, settingsView{Section: section, appView: data})
}

func titleForSettingsSection(section string) string {
	switch section {
	case settingsSectionAccount:
		return "Account"
	case settingsSectionConnections:
		return "Connections"
	case settingsSectionAttachments:
		return "Attachments"
	case settingsSectionAppearance:
		return "Appearance"
	default:
		return "Settings"
	}
}

func storeActorFrom(r *http.Request, sess *session.Session) store.Actor {
	return store.Actor{ID: sess.UserID, Login: sess.Login, IP: ClientIP(r)}
}
