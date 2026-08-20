// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/session"
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
	// TestAccountID and TestDiag carry the result of "Test" on one saved
	// connection (2.6.C1), shown inline in that account's own block rather
	// than mixed into Trace, which belongs to the "Connect account" form.
	TestAccountID string
	TestDiag      string
	// EditAccountID and EditForm back "Edit" on a saved connection (2.6.C2):
	// which account's inline form is open, and what it is pre-filled with.
	EditAccountID string
	EditForm      connectForm
	// Sessions and CurrentSessionID back the self-service SESSIONS block of
	// 2.6.C3 — the admin panel could already list and kill a user's sessions;
	// a person could see nothing about their own.
	Sessions         []session.Info
	CurrentSessionID string
	// WeekStart is "monday" or "sunday", for the Dates and times block of
	// 2.6.C7. Populated even off the Appearance screen because appView is
	// shared, the same as Attachments already is.
	WeekStart string
	// CollectionForm backs the create/rename/delete sheets of §10.1.
	CollectionForm collectionFormView
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
		Escrow:           escrowStatusOf(s.Store.Settings(), user),
		Email:            user.Email,
		PendingEmail:     user.PendingEmail,
		Attachments:      s.attachmentSettingsView(sess),
		Sessions:         s.Sessions.Sessions(sess.UserID),
		CurrentSessionID: sess.ID,
		WeekStart:        user.WeekStart,
	}
	sort.Slice(out.Sessions, func(i, j int) bool { return out.Sessions[i].LastSeen.After(out.Sessions[j].LastSeen) })
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

// fillEditForm opens the inline edit form for one saved account when the
// Connections screen is loaded with ?edit=<id> (2.6.C2). The password field
// is left blank on purpose: submitting without typing a new one keeps the
// saved one, the same way the admin reset-password form never shows one back.
func (s *Server) fillEditForm(r *http.Request, data *appView, accountID string) {
	sess := SessionFrom(r)
	if sess == nil || accountID == "" {
		return
	}
	acc, err := s.Store.GetDAVAccount(sess.UserID, accountID, sess.DEK())
	if err != nil {
		return
	}
	data.EditAccountID = acc.ID
	data.EditForm = connectForm{Label: acc.Label, BaseURL: acc.BaseURL, Username: acc.Username}
}

// appTestDAV re-runs discovery against a saved account's own stored
// credentials and shows the trace, without saving anything (2.6.C1 — §6
// promises a diagnostic that names the step that failed, and today that is
// only available while adding an account, not after).
func (s *Server) appTestDAV(r *http.Request, actor store.Actor) (appView, error) {
	sess := SessionFrom(r)
	accountID := strings.TrimSpace(r.PostFormValue(fieldAccountID))
	data := s.buildAppView(r)
	if accountID == "" {
		return data, fmt.Errorf("choose an account to test")
	}
	acc, err := s.Store.GetDAVAccount(sess.UserID, accountID, sess.DEK())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return data, fmt.Errorf("account not found")
		}
		return data, err
	}
	if s.Guard == nil {
		return data, fmt.Errorf("DAV connections are not configured")
	}

	creds := discovery.Credentials{BaseURL: acc.BaseURL, Username: acc.Username, Password: acc.Password}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	result, trace, testErr := discovery.Discover(ctx, s.Guard, creds)

	data.TestAccountID = accountID
	data.TestDiag = formatDAVDiag(result, trace, testErr)
	_ = s.Store.Log(store.AuditEntry{
		Action:      store.ActionDAVTest,
		ActorID:     actor.ID,
		ActorLogin:  actor.Login,
		TargetID:    sess.UserID,
		TargetLogin: sess.Login,
		IP:          actor.IP,
		Detail:      diagSummary(result, testErr),
	})
	if testErr != nil {
		return data, testErr
	}
	return data, nil
}

// appUpdateDAV changes a saved connection's address, username or password in
// place, keeping its account ID — and with it every source selection and
// duplicate decision keyed on that ID (2.6.C2). A new discovery run is
// required to save, the same as adding an account, so a broken edit is never
// saved silently; its collections replace the stored ones, since that is what
// "the address changed" means. A collection whose path also changed on the
// server orphans any view selection or duplicate decision that named the old
// path — that risk belongs to pointing the account somewhere else, not to
// this feature, and nothing here can detect it in advance.
func (s *Server) appUpdateDAV(r *http.Request, actor store.Actor) (appView, error) {
	sess := SessionFrom(r)
	accountID := strings.TrimSpace(r.PostFormValue(fieldAccountID))
	form := connectForm{
		Label:    strings.TrimSpace(r.PostFormValue(fieldDAVLabel)),
		BaseURL:  strings.TrimSpace(r.PostFormValue(fieldDAVURL)),
		Username: strings.TrimSpace(r.PostFormValue(fieldDAVUser)),
		Password: r.PostFormValue(fieldDAVPass),
	}
	data := s.buildAppView(r)
	data.EditAccountID = accountID
	data.EditForm = form

	if accountID == "" {
		return data, fmt.Errorf("choose an account to edit")
	}
	existing, err := s.Store.GetDAVAccount(sess.UserID, accountID, sess.DEK())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return data, fmt.Errorf("account not found")
		}
		return data, err
	}
	if form.BaseURL == "" || form.Username == "" {
		return data, fmt.Errorf("enter the server URL and username")
	}
	if s.Guard == nil {
		return data, fmt.Errorf("DAV connections are not configured")
	}
	password := form.Password
	if password == "" {
		password = existing.Password
	}

	creds := discovery.Credentials{BaseURL: form.BaseURL, Username: form.Username, Password: password}
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
	updated := *existing
	updated.Label = label
	updated.BaseURL = result.BaseURL
	updated.Username = form.Username
	updated.Password = password
	updated.Principal = result.Principal
	updated.Collections = result.Collections
	if err := s.Store.PutDAVAccount(actor, sess.UserID, sess.DEK(), updated); err != nil {
		return data, err
	}

	return s.buildAppView(r), nil
}

// appSaveAppearance stores the first-day-of-week preference (2.6.C7). It is
// the one control on the Appearance screen that is not local storage: the
// week's boundary is worked out on the server before the page is sent.
func (s *Server) appSaveAppearance(r *http.Request) (appView, error) {
	sess := SessionFrom(r)
	start := r.PostFormValue("week_start")
	if start != "monday" && start != "sunday" {
		return s.buildAppView(r), fmt.Errorf("choose Monday or Sunday")
	}
	if err := s.Store.SetWeekStart(sess.UserID, start); err != nil {
		return s.buildAppView(r), err
	}
	return s.buildAppView(r), nil
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
