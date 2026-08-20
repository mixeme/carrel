// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/session"
	"gitea.mixdep.ru/mix/carrel/internal/store"
)

const (
	fieldCollectionAccount = "account_id"
	fieldCollectionPath    = "collection_path"
	fieldCollectionName    = "collection_name"
	fieldCollectionAddress = "collection_address"
	fieldCollectionColor   = "collection_color"
	fieldCollectionKind    = "collection_kind"
	fieldCollectionComps   = "components"
	fieldConfirmCollection = "confirm_collection"
	fieldCollectionReturn  = "return"
)

type collectionFormView struct {
	Mode         string
	Kind         discovery.Kind
	Title        string
	AccountID    string
	AccountLabel string
	Collection   discovery.Collection
	ColPath      string
	DisplayName  string
	Address      string
	Color        string
	Components   []string
	AddressHint  string
	ReadOnlyAddr bool
	Palette      []string
	Accounts     []collectionAccountChoice
	ReturnURL    string
	Diag         string
	ExportURL    string
	Refs         collectionDeleteRefs
	Host         string
}

type collectionAccountChoice struct {
	ID       string
	Label    string
	Disabled bool
	Reason   string
}

type collectionDeleteRefs struct {
	PublishedLinks []string
	BackupJobs     []string
	DavloomDevices []string
}

func (s *Server) CollectionNew(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.collectionNewSubmit(w, r)
		return
	}
	kind := collectionKindFromQuery(r)
	data := collectionFormView{
		Mode:       "new",
		Kind:       kind,
		Title:      collectionSheetTitle(kind, "new"),
		Components: componentsFromQuery(r),
		Palette:    discovery.CollectionPalette,
		ReturnURL:  strings.TrimSpace(r.URL.Query().Get("return")),
	}
	if data.ReturnURL == "" {
		data.ReturnURL = s.Path("/app/settings/connections")
	}
	if accountID := strings.TrimSpace(r.URL.Query().Get("account")); accountID != "" {
		data.AccountID = accountID
	}
	s.renderCollectionForm(w, r, "new", data)
}

func (s *Server) CollectionRename(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.collectionRenameSubmit(w, r)
		return
	}
	data, err := s.loadCollectionForm(r, "rename")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.renderCollectionForm(w, r, "rename", data)
}

func (s *Server) CollectionDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.collectionDeleteSubmit(w, r)
		return
	}
	data, err := s.loadCollectionForm(r, "delete")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.renderCollectionForm(w, r, "delete", data)
}

func (s *Server) renderCollectionForm(w http.ResponseWriter, r *http.Request, mode string, data collectionFormView) {
	if data.Mode == "" {
		data.Mode = mode
	}
	if data.Palette == nil {
		data.Palette = discovery.CollectionPalette
	}
	if data.ReturnURL == "" {
		data.ReturnURL = strings.TrimSpace(r.URL.Query().Get("return"))
	}
	if data.ReturnURL == "" {
		data.ReturnURL = s.Path("/app/settings/connections")
	}
	if data.Kind == "" {
		data.Kind = collectionKindFromQuery(r)
	}
	if data.Title == "" {
		data.Title = collectionSheetTitle(data.Kind, mode)
	}
	if len(data.Accounts) == 0 && mode == "new" {
		data.Accounts = s.collectionAccountChoices(r, data.Kind)
	}
	v := s.View(r, data.Title)
	v.ShellLayout = "settings"
	s.firstLoginEscrowNotice(r, &v)
	v.Data = settingsView{Section: settingsSectionConnections, appView: appView{CollectionForm: data}}
	s.Render(w, "collection_form.html", v)
}

func (s *Server) collectionNewSubmit(w http.ResponseWriter, r *http.Request) {
	sess := SessionFrom(r)
	actor := storeActorFrom(r, sess)
	data := s.buildCollectionFormFromPost(r)
	data.Accounts = s.collectionAccountChoices(r, data.Kind)

	if data.AccountID == "" {
		s.renderCollectionFormError(w, r, data, fmt.Errorf("choose a server"))
		return
	}
	acc, client, err := s.accountClient(sess, data.AccountID)
	if err != nil {
		s.renderCollectionFormError(w, r, data, err)
		return
	}
	homes, err := discovery.ResolveHomes(r.Context(), client, acc.Principal)
	if err != nil {
		s.renderCollectionFormError(w, r, data, err)
		return
	}
	if data.Address == "" {
		data.Address = discovery.UniqueAddress(collectionHome(homes, data.Kind), discovery.AddressFromName(data.DisplayName), acc.Collections)
	}
	if err := discovery.ValidateAddress(data.Address); err != nil {
		s.renderCollectionFormError(w, r, data, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	col, err := discovery.CreateCollection(ctx, client, homes, acc.Collections, discovery.CreateParams{
		Kind:        data.Kind,
		DisplayName: data.DisplayName,
		Address:     data.Address,
		Color:       data.Color,
		Components:  data.Components,
	})
	if err != nil {
		data.Diag = discovery.FormatRequestDiag(err)
		s.renderCollectionFormError(w, r, data, userFacingCollectionError(err))
		return
	}
	if err := s.Store.UpsertCollection(actor, sess.UserID, sess.DEK(), acc.ID, col, store.ActionCollectionCreate); err != nil {
		s.renderCollectionFormError(w, r, data, err)
		return
	}
	sess.Cache().InvalidateCollection(acc.ID, col.Path)
	http.Redirect(w, r, collectionReturnURL(s, data.ReturnURL, acc.ID, col.Path), http.StatusSeeOther)
}

func (s *Server) collectionRenameSubmit(w http.ResponseWriter, r *http.Request) {
	sess := SessionFrom(r)
	actor := storeActorFrom(r, sess)
	data := s.buildCollectionFormFromPost(r)
	acc, client, col, err := s.loadCollectionTarget(sess, data.AccountID, data.ColPath)
	if err != nil {
		s.renderCollectionFormError(w, r, data, err)
		return
	}
	if col.ReadOnly {
		s.renderCollectionFormError(w, r, data, fmt.Errorf("this collection is read-only"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	updated, err := discovery.RenameCollection(ctx, client, col, discovery.RenameParams{
		DisplayName: data.DisplayName,
		Color:       data.Color,
	})
	if err != nil {
		data.Diag = discovery.FormatRequestDiag(err)
		s.renderCollectionFormError(w, r, data, userFacingCollectionError(err))
		return
	}
	if err := s.Store.UpsertCollection(actor, sess.UserID, sess.DEK(), acc.ID, updated, store.ActionCollectionRename); err != nil {
		s.renderCollectionFormError(w, r, data, err)
		return
	}
	sess.Cache().InvalidateCollection(acc.ID, col.Path)
	http.Redirect(w, r, data.ReturnURL, http.StatusSeeOther)
}

func (s *Server) collectionDeleteSubmit(w http.ResponseWriter, r *http.Request) {
	sess := SessionFrom(r)
	actor := storeActorFrom(r, sess)
	data := s.buildCollectionFormFromPost(r)
	acc, client, col, err := s.loadCollectionTarget(sess, data.AccountID, data.ColPath)
	if err != nil {
		s.renderCollectionFormError(w, r, data, err)
		return
	}
	confirm := strings.TrimSpace(r.PostFormValue(fieldConfirmCollection))
	want := col.DisplayName
	if want == "" {
		want = pathLeaf(col.Path)
	}
	if confirm != want {
		data = s.fillDeleteForm(*acc, col, data.ReturnURL)
		s.renderCollectionFormError(w, r, data, fmt.Errorf("type the collection name exactly to confirm"))
		return
	}
	if col.ReadOnly {
		s.renderCollectionFormError(w, r, data, fmt.Errorf("this collection is read-only"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if err := discovery.DeleteCollection(ctx, client, col.Path); err != nil {
		data.Diag = discovery.FormatRequestDiag(err)
		s.renderCollectionFormError(w, r, data, userFacingCollectionError(err))
		return
	}
	if err := s.Store.RemoveCollection(actor, sess.UserID, sess.DEK(), acc.ID, col.Path); err != nil {
		s.renderCollectionFormError(w, r, data, err)
		return
	}
	sess.Cache().InvalidateCollection(acc.ID, col.Path)
	http.Redirect(w, r, data.ReturnURL, http.StatusSeeOther)
}

func (s *Server) loadCollectionForm(r *http.Request, mode string) (collectionFormView, error) {
	sess := SessionFrom(r)
	accountID := strings.TrimSpace(r.URL.Query().Get("account"))
	colPathEnc := strings.TrimSpace(r.URL.Query().Get("col"))
	returnURL := strings.TrimSpace(r.URL.Query().Get("return"))
	if returnURL == "" {
		returnURL = s.Path("/app/settings/connections")
	}
	if accountID == "" {
		return collectionFormView{}, fmt.Errorf("account is required")
	}
	acc, err := s.Store.GetDAVAccount(sess.UserID, accountID, sess.DEK())
	if err != nil {
		return collectionFormView{}, fmt.Errorf("account not found")
	}
	colPath, err := DecodeCollectionPath(colPathEnc)
	if err != nil {
		return collectionFormView{}, err
	}
	col, ok := discovery.FindCollection(acc.Collections, colPath)
	if !ok {
		return collectionFormView{}, fmt.Errorf("collection not found")
	}
	label := acc.Label
	if label == "" {
		label = acc.Username
	}
	data := collectionFormView{
		Mode:         mode,
		Kind:         col.Kind,
		AccountID:    acc.ID,
		AccountLabel: label,
		Collection:   col,
		ColPath:      col.Path,
		DisplayName:  col.DisplayName,
		Color:        col.Color,
		Components:   col.SupportedComponents,
		ReadOnlyAddr: true,
		AddressHint:  col.Path,
		ReturnURL:    returnURL,
		Palette:      discovery.CollectionPalette,
	}
	if mode == "delete" {
		data = s.fillDeleteForm(*acc, col, returnURL)
	}
	if mode == "rename" && data.Color == "" && col.Kind == discovery.KindAddressBook {
		data.Color = discovery.ColorFromAddress(pathLeaf(col.Path))
	}
	data.Title = collectionSheetTitle(col.Kind, mode)
	return data, nil
}

func (s *Server) fillDeleteForm(acc account.Account, col discovery.Collection, returnURL string) collectionFormView {
	label := acc.Label
	if label == "" {
		label = acc.Username
	}
	host := acc.BaseURL
	if u, err := url.Parse(acc.BaseURL); err == nil && u.Host != "" {
		host = u.Host
	}
	exportURL := ""
	switch col.Kind {
	case discovery.KindCalendar:
		exportURL = s.Path("/app/calendar/" + acc.ID + "/" + EncodeCollectionPath(col.Path) + "/export")
	case discovery.KindAddressBook:
		exportURL = s.Path("/app/contacts/" + acc.ID + "/" + EncodeCollectionPath(col.Path) + "/export")
	}
	return collectionFormView{
		Mode:         "delete",
		Kind:         col.Kind,
		Title:        "Delete on the server",
		AccountID:    acc.ID,
		AccountLabel: label,
		Collection:   col,
		ColPath:      col.Path,
		DisplayName:  col.DisplayName,
		ReturnURL:    returnURL,
		ExportURL:    exportURL,
		Host:         host,
	}
}

func (s *Server) buildCollectionFormFromPost(r *http.Request) collectionFormView {
	kind := collectionKindFromQuery(r)
	if k := strings.TrimSpace(r.PostFormValue(fieldCollectionKind)); k != "" {
		kind = discovery.Kind(k)
	}
	comps := r.PostForm[fieldCollectionComps]
	if len(comps) == 0 {
		comps = defaultComponents(kind)
	}
	return collectionFormView{
		Mode:        strings.TrimSpace(r.PostFormValue("mode")),
		Kind:        kind,
		AccountID:   strings.TrimSpace(r.PostFormValue(fieldCollectionAccount)),
		ColPath:     strings.TrimSpace(r.PostFormValue(fieldCollectionPath)),
		DisplayName: strings.TrimSpace(r.PostFormValue(fieldCollectionName)),
		Address:     strings.TrimSpace(r.PostFormValue(fieldCollectionAddress)),
		Color:       strings.TrimSpace(r.PostFormValue(fieldCollectionColor)),
		Components:  comps,
		ReturnURL:   strings.TrimSpace(r.PostFormValue(fieldCollectionReturn)),
		Title:       collectionSheetTitle(kind, strings.TrimSpace(r.PostFormValue("mode"))),
		Palette:     discovery.CollectionPalette,
	}
}

func (s *Server) renderCollectionFormError(w http.ResponseWriter, r *http.Request, data collectionFormView, err error) {
	v := s.View(r, data.Title)
	v.ShellLayout = "settings"
	if data.Diag != "" {
		v.Error = err.Error()
	} else {
		v.Error = userFacingDAVError(err)
	}
	s.firstLoginEscrowNotice(r, &v)
	v.Data = settingsView{Section: settingsSectionConnections, appView: appView{CollectionForm: data}}
	s.RenderStatus(w, http.StatusBadRequest, "collection_form.html", v)
}

func (s *Server) collectionAccountChoices(r *http.Request, kind discovery.Kind) []collectionAccountChoice {
	sess := SessionFrom(r)
	accounts, err := s.Store.ListDAVAccounts(sess.UserID, sess.DEK())
	if err != nil {
		return nil
	}
	out := make([]collectionAccountChoice, 0, len(accounts))
	for _, acc := range accounts {
		if !acc.Enabled {
			continue
		}
		label := acc.Label
		if label == "" {
			label = acc.Username
		}
		choice := collectionAccountChoice{ID: acc.ID, Label: label}
		if acc.Principal == "" {
			choice.Disabled = true
			choice.Reason = "plain WebDAV — no calendar or address book home-set"
		}
		out = append(out, choice)
	}
	return out
}

func (s *Server) accountClient(sess *session.Session, accountID string) (*account.Account, *dav.Client, error) {
	if s.Guard == nil {
		return nil, nil, fmt.Errorf("DAV connections are not configured")
	}
	acc, err := s.Store.GetDAVAccount(sess.UserID, accountID, sess.DEK())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil, fmt.Errorf("account not found")
		}
		return nil, nil, err
	}
	client, err := dav.NewClient(s.Guard, acc.BaseURL, acc.Username, acc.Password)
	if err != nil {
		return nil, nil, err
	}
	return acc, client, nil
}

func (s *Server) loadCollectionTarget(sess *session.Session, accountID, colPath string) (*account.Account, *dav.Client, discovery.Collection, error) {
	acc, client, err := s.accountClient(sess, accountID)
	if err != nil {
		return nil, nil, discovery.Collection{}, err
	}
	col, ok := discovery.FindCollection(acc.Collections, colPath)
	if !ok {
		return nil, nil, discovery.Collection{}, fmt.Errorf("collection not found")
	}
	return acc, client, col, nil
}

func collectionKindFromQuery(r *http.Request) discovery.Kind {
	k := strings.TrimSpace(r.URL.Query().Get("kind"))
	if k == "" {
		k = strings.TrimSpace(r.FormValue("kind"))
	}
	if discovery.Kind(k) == discovery.KindAddressBook {
		return discovery.KindAddressBook
	}
	return discovery.KindCalendar
}

func collectionSheetTitle(kind discovery.Kind, mode string) string {
	switch mode {
	case "rename":
		return "Rename and colour"
	case "delete":
		return "Delete on the server"
	default:
		if kind == discovery.KindAddressBook {
			return "New address book"
		}
		return "New calendar"
	}
}

func defaultComponents(kind discovery.Kind) []string {
	if kind == discovery.KindAddressBook {
		return nil
	}
	return []string{"VEVENT"}
}

func componentsFromQuery(r *http.Request) []string {
	raw := strings.TrimSpace(r.URL.Query().Get("components"))
	if raw == "" {
		return defaultComponents(collectionKindFromQuery(r))
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(strings.ToUpper(p))
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return defaultComponents(collectionKindFromQuery(r))
	}
	return out
}

func collectionHome(homes discovery.Homes, kind discovery.Kind) string {
	if kind == discovery.KindAddressBook {
		return homes.AddressBook
	}
	return homes.Calendar
}

func collectionReturnURL(s *Server, returnURL, accountID, colPath string) string {
	if strings.Contains(returnURL, "/app/settings/") {
		return returnURL
	}
	enc := EncodeCollectionPath(colPath)
	if strings.Contains(returnURL, "/app/contacts") {
		return s.Path("/app/contacts/" + accountID + "/" + enc)
	}
	if strings.Contains(returnURL, "/app/tasks") {
		return s.Path("/app/tasks/" + accountID + "/" + enc)
	}
	if strings.Contains(returnURL, "/app/notes") {
		return s.Path("/app/notes/" + accountID + "/" + enc)
	}
	if strings.Contains(returnURL, "/app/calendar") {
		return s.Path("/app/calendar/" + accountID + "/" + enc)
	}
	return returnURL
}

func collectionNewURL(s *Server, kind discovery.Kind, components []string, returnURL string) string {
	q := url.Values{"kind": {string(kind)}, "return": {returnURL}}
	if len(components) > 0 {
		q.Set("components", strings.Join(components, ","))
	}
	return s.Path("/app/collections/new?" + q.Encode())
}

func (s *Server) sectionNewCollectionURL(mode findMode) string {
	home := s.Path(mode.sectionHome())
	switch mode {
	case modePeople:
		return collectionNewURL(s, discovery.KindAddressBook, nil, home)
	case modeTasks:
		return collectionNewURL(s, discovery.KindCalendar, []string{"VTODO"}, home)
	case modeNotes:
		return collectionNewURL(s, discovery.KindCalendar, []string{"VJOURNAL"}, home)
	case modeTime:
		return collectionNewURL(s, discovery.KindCalendar, []string{"VEVENT"}, home)
	default:
		return ""
	}
}

func sectionNewCollectionLabel(mode findMode) string {
	switch mode {
	case modePeople:
		return "New address book"
	case modeTasks:
		return "New list"
	case modeNotes:
		return "New notebook"
	case modeTime:
		return "New calendar"
	default:
		return ""
	}
}

func userFacingCollectionError(err error) error {
	var reqErr *dav.RequestError
	if errors.As(err, &reqErr) && reqErr.Err != nil {
		return reqErr.Err
	}
	return err
}

func pathLeaf(p string) string {
	p = strings.TrimSuffix(normalizeCollectionPath(p), "/")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func (s *Server) collectionManageURL(action, accountID, colPath, returnURL string) string {
	q := url.Values{
		"account": {accountID},
		"col":     {EncodeCollectionPath(colPath)},
		"return":  {returnURL},
	}
	return s.Path("/app/collections/" + action + "?" + q.Encode())
}
