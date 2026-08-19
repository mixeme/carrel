// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/session"
)

type folderPickerView struct {
	Roots    []folderPickerNode
	Selected folderPickerSel
	Title    string
	Action   string
}

type folderPickerSel struct {
	AccountID string
	ColEnc    string
	Folder    string
}

type folderPickerNode struct {
	AccountID    string
	ColEnc       string
	Folder       string
	Name         string
	ServerLabel  string
	ReadOnly     bool
	Children     []folderPickerNode
	HasChildren  bool
	Collection   bool
}

// FilesFolderPicker returns the shared folder tree for move, copy, backup and mail.
func (s *Server) FilesFolderPicker(w http.ResponseWriter, r *http.Request) {
	sess := SessionFrom(r)
	rows := s.fileCollections(sess)
	view := folderPickerView{
		Title:  strings.TrimSpace(r.URL.Query().Get("title")),
		Action: strings.TrimSpace(r.URL.Query().Get("action")),
		Selected: folderPickerSel{
			AccountID: r.URL.Query().Get("account"),
			ColEnc:    r.URL.Query().Get("col"),
			Folder:    r.URL.Query().Get("p"),
		},
	}
	if view.Title == "" {
		view.Title = "Choose folder"
	}
	for _, row := range rows {
		node := folderPickerNode{
			AccountID: row.AccountID, ColEnc: row.ColEnc, Name: row.Label(),
			ServerLabel: row.AccountLabel, ReadOnly: row.ReadOnly, Collection: true,
		}
		if !row.ReadOnly {
			ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
			node.Children = s.folderChildren(ctx, sess, row.AccountID, row.Path, row.ColEnc, "")
			cancel()
		}
		view.Roots = append(view.Roots, node)
	}
	v := s.View(r, "")
	v.Data = view
	s.Render(w, "folder_picker.html", v)
}

// FilesFolderChildren returns one level of subfolders for lazy tree expansion.
func (s *Server) FilesFolderChildren(w http.ResponseWriter, r *http.Request) {
	accountID, colEnc := r.URL.Query().Get("account"), r.URL.Query().Get("col")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	rel := r.URL.Query().Get("p")
	sess := SessionFrom(r)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	children := s.folderChildren(ctx, sess, accountID, collection, colEnc, rel)
	v := s.View(r, "")
	v.Data = struct {
		Children []folderPickerNode
		Parent   folderPickerSel
	}{Children: children, Parent: folderPickerSel{AccountID: accountID, ColEnc: colEnc, Folder: rel}}
	s.Render(w, "folder_picker_children.html", v)
}

func (s *Server) folderChildren(ctx context.Context, sess *session.Session, accountID, collection, colEnc, rel string) []folderPickerNode {
	p, acc, err := s.filesProvider(sess, accountID)
	if err != nil {
		return nil
	}
	col, err := findFileCollection(acc, collection)
	if err != nil || col.ReadOnly {
		return nil
	}
	listing, err := p.List(ctx, col.Path, rel)
	if err != nil {
		return nil
	}
	var out []folderPickerNode
	for _, entry := range listing.Entries {
		if !entry.Dir {
			continue
		}
		node := folderPickerNode{
			AccountID: accountID, ColEnc: colEnc, Folder: entry.Rel, Name: entry.Name,
			ServerLabel: accountLabel(*acc), ReadOnly: col.ReadOnly, HasChildren: true,
		}
		out = append(out, node)
	}
	return out
}

func (s *Server) folderPickerRoots(ctx context.Context, sess *session.Session) []folderPickerNode {
	rows := s.fileCollections(sess)
	var roots []folderPickerNode
	for _, row := range rows {
		node := folderPickerNode{
			AccountID: row.AccountID, ColEnc: row.ColEnc, Name: row.Label(),
			ServerLabel: row.AccountLabel, ReadOnly: row.ReadOnly, Collection: true,
		}
		if !row.ReadOnly {
			node.Children = s.folderChildren(ctx, sess, row.AccountID, row.Path, row.ColEnc, "")
		}
		roots = append(roots, node)
	}
	return roots
}
