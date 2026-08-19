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
	"gitea.mixdep.ru/mix/carrel/internal/config"
	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/model"
	"gitea.mixdep.ru/mix/carrel/internal/provider/files"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

// FilesConfig holds the file section limits (§7, §23.10).
type FilesConfig = config.Files

// fieldPath is the relative path inside a file collection, carried as a query
// parameter rather than a path segment. The collection root is already one
// encoded segment; making the rest of the path a second encoded segment would
// mean a URL nobody can read and a breadcrumb built out of nested encodings.
const fieldPath = "p"

func (s *Server) filesProvider(sess *session.Session, accountID string) (*files.Provider, *account.Account, error) {
	if s.Guard == nil {
		return nil, nil, fmt.Errorf("DAV connections are not configured")
	}
	acc, err := s.Store.GetDAVAccount(sess.UserID, accountID, sess.DEK())
	if err != nil {
		return nil, nil, fmt.Errorf("account not found")
	}
	if !acc.Enabled {
		return nil, nil, fmt.Errorf("account is disabled")
	}
	client, err := dav.NewClient(s.Guard, acc.BaseURL, acc.Username, acc.Password)
	if err != nil {
		return nil, nil, err
	}
	p, err := files.New(client, files.Options{
		AccountID:  acc.ID,
		Cache:      sess.Cache(),
		MaxEntries: s.Files.MaxEntries,
	})
	if err != nil {
		return nil, nil, err
	}
	return p, acc, nil
}

func findFileCollection(acc *account.Account, collection string) (discovery.Collection, error) {
	collection = normalizeCollectionPath(collection)
	for _, col := range acc.Collections {
		if col.Kind == discovery.KindFiles && normalizeCollectionPath(col.Path) == collection {
			return col, nil
		}
	}
	return discovery.Collection{}, fmt.Errorf("file collection not found")
}

// fileCollections lists every file collection the user has.
func (s *Server) fileCollections(sess *session.Session) []sourceRow {
	rows, err := s.collectionsOfKind(sess, discovery.KindFiles, account.ViewFiles, "")
	if err != nil {
		return nil
	}
	// The tick of §14 has no meaning here — a file browser shows one folder at a
	// time and polls nothing — so every collection is offered.
	for i := range rows {
		rows[i].Selected = true
	}
	return rows
}

type filesView struct {
	Sources      []sourceRow
	AccountID    string
	ColEnc       string
	Collection   discovery.Collection
	AccountLabel string
	// Rel is the folder inside the collection, "" at its root.
	Rel       string
	Crumbs    []fileCrumb
	ParentURL string
	Entries   []fileRow
	ReadOnly  bool
	Empty     bool
	NoFiles   bool
	Truncated bool
	MaxUpload string
	// IsAttachmentFolder marks the folder §23.10 puts attachments in, so a
	// person who browses to it can see that is what it is.
	IsAttachmentFolder bool
	PrintDate          string
	// AllCollections is the section root: every file collection as a row.
	AllCollections bool
	Collections    []fileCollectionRow
	ServerCount    int
	FolderTitle    string
	ItemCount      int
	TotalSizeLabel string
	PickerRoots    []folderPickerNode
	AttachmentsURL string
	AttachmentsHint string
	PublishedActive bool
}

// fileCollectionRow is one storage at the All collections root.
type fileCollectionRow struct {
	AccountID    string
	ColEnc       string
	Name         string
	ServerLabel  string
	ReadOnly     bool
	IsAttachment bool
	URL          string
}

type fileCrumb struct {
	Name string
	URL  string
	Last bool
}

type fileRow struct {
	Name        string
	Rel         string
	Dir         bool
	SizeLabel   string
	SizeBytes   int64
	TypeLabel   string
	ModLabel    string
	ETag        string
	URL         string
	DownloadURL string
	ThumbURL    string
	IconKind    string
}

// FilesHome is the section root: every connected file collection as a top-level
// folder. A specific collection opens from there or from the rail.
func (s *Server) FilesHome(w http.ResponseWriter, r *http.Request) {
	sess := SessionFrom(r)
	rows := s.fileCollections(sess)
	if len(rows) == 0 {
		v := s.View(r, "Files")
		v.Data = filesView{NoFiles: true}
		s.Render(w, "files.html", v)
		return
	}
	attach, hasAttach := s.attachmentTarget(sess)
	servers := make(map[string]struct{})
	view := filesView{
		Sources: rows, AllCollections: true,
		PrintDate: time.Now().UTC().Format("2006-01-02 15:04 UTC"),
	}
	for _, row := range rows {
		servers[row.AccountID] = struct{}{}
		cr := fileCollectionRow{
			AccountID: row.AccountID, ColEnc: row.ColEnc, Name: row.Label(),
			ServerLabel: row.AccountLabel, ReadOnly: row.ReadOnly,
			URL: s.Path("/app/files/" + row.AccountID + "/" + row.ColEnc),
		}
		if hasAttach && attach.AccountID == row.AccountID &&
			normalizeCollectionPath(attach.Root) == normalizeCollectionPath(row.Path) {
			cr.IsAttachment = true
		}
		view.Collections = append(view.Collections, cr)
	}
	view.ServerCount = len(servers)
	view.AttachmentsURL, view.AttachmentsHint = s.filesAttachmentsShortcut(sess)
	v := s.View(r, "Files")
	v.Data = view
	s.Render(w, "files.html", v)
}

// FilesBrowse lists one folder of one collection.
func (s *Server) FilesBrowse(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.filesAction(w, r)
		return
	}
	accountID, colEnc := r.PathValue("account"), r.PathValue("col")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	rel := r.URL.Query().Get(fieldPath)
	view, err := s.buildFiles(r.Context(), sess, accountID, collection, colEnc, rel)
	if err != nil {
		s.renderFilesError(w, r, err, accountID, colEnc)
		return
	}
	s.rememberDefault(sess, account.ViewFiles, accountID, collection)
	v := s.View(r, "Files")
	v.Notice = strings.TrimSpace(r.URL.Query().Get("notice"))
	v.Data = view
	s.Render(w, "files.html", v)
}

func (s *Server) buildFiles(ctx context.Context, sess *session.Session, accountID, collection, colEnc, rel string) (filesView, error) {
	p, acc, err := s.filesProvider(sess, accountID)
	if err != nil {
		return filesView{}, err
	}
	col, err := findFileCollection(acc, collection)
	if err != nil {
		return filesView{}, err
	}
	clean, err := files.CleanRelative(rel)
	if err != nil {
		return filesView{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	listing, err := p.List(ctx, col.Path, clean)
	if err != nil {
		return filesView{}, err
	}
	base := s.Path("/app/files/" + accountID + "/" + colEnc)
	view := filesView{
		Sources: s.fileCollections(sess), AccountID: accountID, ColEnc: colEnc,
		Collection: col, AccountLabel: accountLabel(*acc), Rel: clean,
		ReadOnly: col.ReadOnly, Truncated: listing.Truncated,
		MaxUpload: model.ByteSize(s.filesMaxUpload()),
		Crumbs:    fileCrumbs(base, collectionLabel(col), clean),
	}
	if parent, ok := files.Parent(clean); ok {
		view.ParentURL = folderURL(base, parent)
	}
	if target, ok := s.attachmentTarget(sess); ok {
		view.IsAttachmentFolder = target.AccountID == accountID &&
			normalizeCollectionPath(target.Folder) == normalizeCollectionPath(listing.Dir)
	}
	var totalBytes int64
	for _, entry := range listing.Entries {
		row := fileRow{
			Name: entry.Name, Rel: entry.Rel, Dir: entry.Dir, ETag: entry.ETag,
			TypeLabel: entry.ContentType,
			IconKind:  fileIconKind(entry.Name, entry.ContentType, entry.Dir),
		}
		if entry.HasSize && !entry.Dir {
			row.SizeBytes = entry.Size
			row.SizeLabel = model.ByteSize(entry.Size)
			totalBytes += entry.Size
		}
		if !entry.ModTime.IsZero() {
			row.ModLabel = entry.ModTime.In(s.timezone()).Format("2006-01-02 15:04")
		}
		if entry.Dir {
			row.URL = folderURL(base, entry.Rel)
		} else {
			row.DownloadURL = s.fileDownloadURL(accountID, colEnc, entry.Rel)
			if row.IconKind == "image" {
				row.ThumbURL = s.fileThumbURL(accountID, colEnc, entry.Rel)
			}
		}
		view.Entries = append(view.Entries, row)
	}
	view.Empty = len(view.Entries) == 0
	view.ItemCount = len(view.Entries)
	if totalBytes > 0 {
		view.TotalSizeLabel = model.ByteSize(totalBytes)
	}
	if clean != "" {
		view.FolderTitle = files.Base(clean)
	} else {
		view.FolderTitle = collectionLabel(col)
	}
	view.PrintDate = time.Now().UTC().Format("2006-01-02 15:04 UTC")
	view.PickerRoots = s.folderPickerRoots(ctx, sess)
	view.AttachmentsURL, view.AttachmentsHint = s.filesAttachmentsShortcut(sess)
	return view, nil
}

func (s *Server) fileDownloadURL(accountID, colEnc, rel string) string {
	return s.Path("/d/"+accountID+"/"+colEnc) + "?" + url.Values{fieldPath: {rel}}.Encode()
}

func folderURL(base, rel string) string {
	if rel == "" {
		return base
	}
	return base + "?" + url.Values{fieldPath: {rel}}.Encode()
}

func fileCrumbs(base, rootLabel, rel string) []fileCrumb {
	crumbs := []fileCrumb{{Name: rootLabel, URL: base}}
	if rel == "" {
		crumbs[0].Last = true
		return crumbs
	}
	walked := ""
	for _, segment := range strings.Split(rel, "/") {
		walked = files.Join(walked, segment)
		crumbs = append(crumbs, fileCrumb{Name: segment, URL: folderURL(base, walked)})
	}
	crumbs[len(crumbs)-1].Last = true
	return crumbs
}

// filesAction takes the three things this browser does to a collection: put a
// file in it, make a folder, remove something.
func (s *Server) filesAction(w http.ResponseWriter, r *http.Request) {
	accountID, colEnc := r.PathValue("account"), r.PathValue("col")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	p, acc, err := s.filesProvider(sess, accountID)
	if err != nil {
		s.renderFilesError(w, r, err, accountID, colEnc)
		return
	}
	col, err := findFileCollection(acc, collection)
	if err != nil {
		s.renderFilesError(w, r, err, accountID, colEnc)
		return
	}
	if col.ReadOnly {
		http.Error(w, "this file collection is read-only", http.StatusForbidden)
		return
	}
	base := s.Path("/app/files/" + accountID + "/" + colEnc)

	// An upload arrives as multipart and is streamed on: the body is never
	// parsed into memory, which is the whole reason §7 fixes Get and Put at a
	// reader (§23.10).
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/") {
		if wantsJSONUpload(r) {
			rel, relErr := files.CleanRelative(r.FormValue(fieldPath))
			if relErr != nil {
				writeUploadJSON(w, http.StatusForbidden, uploadReply{Error: "bad path"})
				return
			}
			s.fileUploadXHR(w, r, p, col, rel)
			return
		}
		s.fileUpload(w, r, p, col, base)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	rel, err := files.CleanRelative(r.PostFormValue(fieldPath))
	if err != nil {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	switch r.PostFormValue(fieldAction) {
	case "mkdir":
		name, nameErr := files.CleanName(r.PostFormValue("name"))
		if nameErr != nil {
			s.redirectNotice(w, r, folderURL(base, rel), "That folder name cannot be used.")
			return
		}
		if err := p.MakeDir(ctx, col.Path, files.Join(rel, name)); err != nil {
			s.renderFilesError(w, r, err, accountID, colEnc)
			return
		}
		s.redirectNotice(w, r, folderURL(base, rel), "Folder created.")
	case "delete":
		target, targetErr := files.CleanRelative(r.PostFormValue("target"))
		if targetErr != nil || target == "" {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		if err := p.Remove(ctx, col.Path, target, strings.TrimSpace(r.PostFormValue("etag"))); err != nil {
			s.renderFilesError(w, r, err, accountID, colEnc)
			return
		}
		s.redirectNotice(w, r, folderURL(base, rel), "Deleted from the server. Anything that linked to it now points at nothing.")
	case "delete-batch":
		notice := s.filesDeleteBatch(ctx, p, col, rel, r.PostForm["target"], r.PostForm["etag"])
		s.redirectNotice(w, r, folderURL(base, rel), notice)
	case "rename":
		target, targetErr := files.CleanRelative(r.PostFormValue("target"))
		if targetErr != nil || target == "" {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		if err := s.filesRename(ctx, p, col, target, r.PostFormValue("new_name")); err != nil {
			s.renderFilesError(w, r, err, accountID, colEnc)
			return
		}
		s.redirectNotice(w, r, folderURL(base, rel), "Renamed.")
	case "move":
		dest, destErr := readMoveDest(r)
		if destErr != nil {
			s.redirectNotice(w, r, folderURL(base, rel), destErr.Error())
			return
		}
		notice := s.filesMoveBatch(ctx, sess, accountID, colEnc, rel, r.PostForm["target"], dest)
		s.redirectNotice(w, r, folderURL(base, rel), notice)
	case "copy":
		dest, destErr := readMoveDest(r)
		if destErr != nil {
			s.redirectNotice(w, r, folderURL(base, rel), destErr.Error())
			return
		}
		notice := s.filesCopyBatch(ctx, sess, accountID, colEnc, rel, r.PostForm["target"], dest)
		s.redirectNotice(w, r, folderURL(base, rel), notice)
	default:
		http.Error(w, "bad request", http.StatusBadRequest)
	}
}

func (s *Server) filesMaxUpload() int64 {
	if s.Files.MaxUploadBytes > 0 {
		return s.Files.MaxUploadBytes
	}
	return config.DefaultFilesMaxUploadBytes
}

func (s *Server) renderFilesError(w http.ResponseWriter, r *http.Request, err error, accountID, colEnc string) {
	status := http.StatusBadRequest
	if errors.Is(err, files.ErrOutsideCollection) || errors.Is(err, files.ErrBadName) {
		status = http.StatusForbidden
	}
	v := s.View(r, "Files")
	v.Error = userFacingDAVError(err)
	v.Data = filesView{
		Sources: s.fileCollections(SessionFrom(r)), AccountID: accountID, ColEnc: colEnc,
		MaxUpload: model.ByteSize(s.filesMaxUpload()),
	}
	s.RenderStatus(w, status, "files.html", v)
}
