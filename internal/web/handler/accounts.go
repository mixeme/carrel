// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/store"
)

const (
	fieldDAVLabel    = "dav_label"
	fieldDAVURL      = "dav_url"
	fieldDAVUser     = "dav_username"
	fieldDAVPass     = "dav_password"
	fieldAccountID   = "account_id"
	fieldTestDAVURL  = "dav_test_url"
	fieldTestDAVUser = "dav_test_username"
	fieldTestDAVPass = "dav_test_password"
)

type accountRow struct {
	account.Account
	AddressBooks    []sidebarBook
	Calendars       []sidebarBook
	FileCollections []sidebarBook
}

type sidebarBook struct {
	discovery.Collection
	ColEnc string
	Sync   collectionSyncView
}

type connectForm struct {
	Label    string
	BaseURL  string
	Username string
	Password string
}

type appView struct {
	Escrow       escrowStatus
	Email        string
	PendingEmail string
	Accounts     []accountRow
	Connect      connectForm
	Trace        *discovery.Trace
	ShowConnect  bool
	// Attachments is the one setting §23.10 needs: where a file goes when it is
	// attached to a note or an event.
	Attachments attachmentSettings
}

func (s *Server) buildAppView(r *http.Request) appView {
	sess := SessionFrom(r)
	if sess == nil {
		return appView{}
	}
	user, err := s.Store.User(sess.UserID)
	if err != nil {
		s.logError("load profile", err)
		return appView{}
	}
	out := appView{
		Escrow:       escrowStatusOf(s.Store.Settings(), user),
		Email:        user.Email,
		PendingEmail: user.PendingEmail,
		Attachments:  s.attachmentSettingsView(sess),
	}
	accounts, err := s.Store.ListDAVAccounts(sess.UserID, sess.DEK())
	if err != nil {
		s.logError("list DAV accounts", err)
		return out
	}
	for _, acc := range accounts {
		row := accountRow{Account: acc}
		for _, col := range acc.Collections {
			sync := s.collectionSource(sess, acc.ID, normalizeCollectionPath(col.Path))
			book := sidebarBook{
				Collection: col,
				ColEnc:     EncodeCollectionPath(col.Path),
				Sync: collectionSyncView{
					Known:     sync.Known,
					MetaLabel: sync.MetaLabel,
				},
			}
			switch col.Kind {
			case discovery.KindAddressBook:
				row.AddressBooks = append(row.AddressBooks, book)
			case discovery.KindCalendar:
				row.Calendars = append(row.Calendars, book)
			case discovery.KindFiles:
				row.FileCollections = append(row.FileCollections, book)
			}
		}
		out.Accounts = append(out.Accounts, row)
	}
	return out
}

func (s *Server) appSubmit(w http.ResponseWriter, r *http.Request) {
	s.appSubmitTo(w, r, "settings_connections.html", settingsSectionConnections)
}

func (s *Server) appConnectDAV(r *http.Request, actor store.Actor) (appView, error) {
	sess := SessionFrom(r)
	form := connectForm{
		Label:    strings.TrimSpace(r.PostFormValue(fieldDAVLabel)),
		BaseURL:  strings.TrimSpace(r.PostFormValue(fieldDAVURL)),
		Username: strings.TrimSpace(r.PostFormValue(fieldDAVUser)),
		Password: r.PostFormValue(fieldDAVPass),
	}
	data := s.buildAppView(r)
	data.Connect = form
	data.ShowConnect = true

	if form.BaseURL == "" || form.Username == "" || form.Password == "" {
		return data, fmt.Errorf("enter the server URL, username and password")
	}
	if s.Guard == nil {
		return data, fmt.Errorf("DAV connections are not configured")
	}

	creds := discovery.Credentials{
		BaseURL:  form.BaseURL,
		Username: form.Username,
		Password: form.Password,
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	result, trace, err := discovery.Discover(ctx, s.Guard, creds)
	data.Trace = trace
	if err != nil {
		return data, err
	}

	label := form.Label
	if label == "" {
		label = form.Username
	}
	if _, err := s.Store.PutDAVAccountFromDiscovery(actor, sess.UserID, sess.DEK(), label, creds, result); err != nil {
		return data, err
	}

	data = s.buildAppView(r)
	data.ShowConnect = false
	return data, nil
}

func (s *Server) appDeleteDAV(r *http.Request, actor store.Actor) (appView, error) {
	sess := SessionFrom(r)
	accountID := strings.TrimSpace(r.PostFormValue(fieldAccountID))
	if accountID == "" {
		return s.buildAppView(r), fmt.Errorf("choose an account to remove")
	}
	if err := s.Store.DeleteDAVAccount(actor, sess.UserID, accountID, sess.DEK()); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return s.buildAppView(r), fmt.Errorf("account not found")
		}
		return s.buildAppView(r), err
	}
	if cache := sess.Cache(); cache != nil {
		cache.InvalidateAll()
	}
	return s.buildAppView(r), nil
}

func (s *Server) appRefresh(w http.ResponseWriter, r *http.Request) {
	sess := SessionFrom(r)
	if cache := sess.Cache(); cache != nil {
		cache.InvalidateAll()
	}
	target := r.Header.Get("Referer")
	if target == "" {
		target = s.Path("/app/settings/connections")
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (s *Server) adminTestDAV(r *http.Request, actor store.Actor) (adminView, error) {
	url := strings.TrimSpace(r.PostFormValue(fieldTestDAVURL))
	user := strings.TrimSpace(r.PostFormValue(fieldTestDAVUser))
	pass := r.PostFormValue(fieldTestDAVPass)
	if url == "" || user == "" || pass == "" {
		return adminView{}, fmt.Errorf("enter the server URL, username and password")
	}
	if s.Guard == nil {
		return adminView{}, fmt.Errorf("DAV connections are not configured")
	}

	creds := discovery.Credentials{BaseURL: url, Username: user, Password: pass}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	result, trace, err := discovery.Discover(ctx, s.Guard, creds)
	diag := formatDAVDiag(result, trace, err)
	_ = s.Store.Log(store.AuditEntry{
		Action:     store.ActionDAVTest,
		ActorID:    actor.ID,
		ActorLogin: actor.Login,
		IP:         actor.IP,
		Detail:     diagSummary(result, err),
	})
	return adminView{DAVDiag: diag}, nil
}

func userFacingDAVError(err error) string {
	if errors.Is(err, dav.ErrSSRF) {
		return "That server address is not allowed."
	}
	if errors.Is(err, dav.ErrTooManyRedirects) {
		return "The server redirected too many times."
	}
	if errors.Is(err, dav.ErrResponseTooLarge) {
		return "The server response was too large."
	}
	return capitalize(err.Error())
}

func formatDAVDiag(result *discovery.Result, trace *discovery.Trace, err error) string {
	var b strings.Builder
	if err != nil {
		b.WriteString("Discovery failed: ")
		b.WriteString(err.Error())
		b.WriteByte('\n')
	} else if result != nil {
		fmt.Fprintf(&b, "Found %d collections at %s\n", len(result.Collections), result.BaseURL)
		for _, col := range result.Collections {
			name := col.DisplayName
			if name == "" {
				name = col.Path
			}
			fmt.Fprintf(&b, "  • %s (%s)\n", name, col.Kind)
		}
	}
	if trace != nil {
		for _, step := range trace.Steps {
			fmt.Fprintf(&b, "[%s] %s", step.Name, step.Detail)
			if step.StatusCode != 0 {
				fmt.Fprintf(&b, " (%d)", step.StatusCode)
			}
			if step.Target != "" {
				fmt.Fprintf(&b, " → %s", step.Target)
			}
			b.WriteByte('\n')
		}
	}
	return strings.TrimSpace(b.String())
}

func diagSummary(result *discovery.Result, err error) string {
	if err != nil {
		return "failed"
	}
	if result == nil {
		return "ok"
	}
	return fmt.Sprintf("ok:%d", len(result.Collections))
}
