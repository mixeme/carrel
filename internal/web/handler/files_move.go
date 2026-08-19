// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/provider/files"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

type moveDest struct {
	AccountID  string
	ColEnc     string
	Collection string
	Folder     string
}

func (s *Server) parseMoveDest(sess *session.Session, accountID, colEnc, folder string) (moveDest, *files.Provider, discovery.Collection, error) {
	folder, err := files.CleanRelative(folder)
	if err != nil {
		return moveDest{}, nil, discovery.Collection{}, err
	}
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		return moveDest{}, nil, discovery.Collection{}, err
	}
	p, acc, err := s.filesProvider(sess, accountID)
	if err != nil {
		return moveDest{}, nil, discovery.Collection{}, err
	}
	col, err := findFileCollection(acc, collection)
	if err != nil {
		return moveDest{}, nil, discovery.Collection{}, err
	}
	return moveDest{AccountID: accountID, ColEnc: colEnc, Collection: collection, Folder: folder}, p, col, nil
}

func (s *Server) filesRename(ctx context.Context, p *files.Provider, col discovery.Collection, rel, newName string) error {
	if col.ReadOnly {
		return fmt.Errorf("this file collection is read-only")
	}
	return p.Rename(ctx, col.Path, rel, newName)
}

func (s *Server) filesMoveBatch(ctx context.Context, sess *session.Session, srcAccount, srcColEnc, srcFolder string, targets []string, dest moveDest) string {
	srcCollection, err := DecodeCollectionPath(srcColEnc)
	if err != nil {
		return "That move could not be read."
	}
	srcP, srcAcc, err := s.filesProvider(sess, srcAccount)
	if err != nil {
		return userFacingDAVError(err)
	}
	srcCol, err := findFileCollection(srcAcc, srcCollection)
	if err != nil {
		return userFacingDAVError(err)
	}
	if srcCol.ReadOnly {
		return "This collection is read-only."
	}
	destP, destAcc, err := s.filesProvider(sess, dest.AccountID)
	if err != nil {
		return userFacingDAVError(err)
	}
	destCol, err := findFileCollection(destAcc, dest.Collection)
	if err != nil {
		return userFacingDAVError(err)
	}
	if destCol.ReadOnly {
		return "The destination collection is read-only."
	}
	sameServer := srcAcc.BaseURL == destAcc.BaseURL && srcAccount == dest.AccountID && srcColEnc == dest.ColEnc
	var lines []string
	ok := 0
	for _, raw := range targets {
		rel, err := files.CleanRelative(raw)
		if err != nil || rel == "" {
			lines = append(lines, raw+": refused — bad path.")
			continue
		}
		name := files.Base(rel)
		toRel := files.Join(dest.Folder, name)
		if sameServer && srcAccount == dest.AccountID && srcColEnc == dest.ColEnc {
			if err := srcP.Move(ctx, srcCol.Path, rel, toRel, false); err != nil {
				lines = append(lines, name+": "+userFacingDAVError(err))
				continue
			}
			ok++
			continue
		}
		if err := s.filesTransfer(ctx, srcP, srcCol, destP, destCol, rel, toRel); err != nil {
			lines = append(lines, name+": "+userFacingDAVError(err))
			continue
		}
		ok++
	}
	if ok == len(targets) {
		if ok == 1 {
			return "Moved 1 item."
		}
		return fmt.Sprintf("Moved %d items.", ok)
	}
	if ok == 0 {
		return strings.Join(lines, " ")
	}
	return fmt.Sprintf("Moved %d of %d. %s", ok, len(targets), strings.Join(lines, " "))
}

func (s *Server) filesCopyBatch(ctx context.Context, sess *session.Session, srcAccount, srcColEnc, srcFolder string, targets []string, dest moveDest) string {
	srcCollection, err := DecodeCollectionPath(srcColEnc)
	if err != nil {
		return "That copy could not be read."
	}
	srcP, srcAcc, err := s.filesProvider(sess, srcAccount)
	if err != nil {
		return userFacingDAVError(err)
	}
	srcCol, err := findFileCollection(srcAcc, srcCollection)
	if err != nil {
		return userFacingDAVError(err)
	}
	destP, destAcc, err := s.filesProvider(sess, dest.AccountID)
	if err != nil {
		return userFacingDAVError(err)
	}
	destCol, err := findFileCollection(destAcc, dest.Collection)
	if err != nil {
		return userFacingDAVError(err)
	}
	if destCol.ReadOnly {
		return "The destination collection is read-only."
	}
	var lines []string
	ok := 0
	for _, raw := range targets {
		rel, err := files.CleanRelative(raw)
		if err != nil || rel == "" {
			lines = append(lines, raw+": refused — bad path.")
			continue
		}
		name := files.Base(rel)
		toRel := files.Join(dest.Folder, name)
		if err := s.filesCopyStream(ctx, srcP, srcCol, destP, destCol, rel, toRel); err != nil {
			lines = append(lines, name+": "+userFacingDAVError(err))
			continue
		}
		ok++
	}
	if ok == len(targets) {
		if ok == 1 {
			return "Copied 1 item."
		}
		return fmt.Sprintf("Copied %d items.", ok)
	}
	if ok == 0 {
		return strings.Join(lines, " ")
	}
	return fmt.Sprintf("Copied %d of %d. %s", ok, len(targets), strings.Join(lines, " "))
}

func crossStorageMove(srcAccount, srcColEnc, destAccount, destColEnc string) bool {
	return srcAccount != destAccount || srcColEnc != destColEnc
}

func (s *Server) filesTransfer(ctx context.Context, srcP *files.Provider, srcCol discovery.Collection, destP *files.Provider, destCol discovery.Collection, fromRel, toRel string) error {
	entry, err := srcP.Stat(ctx, srcCol.Path, fromRel)
	if err != nil {
		return err
	}
	if entry.Dir {
		return fmt.Errorf("moving folders across storages is not supported yet")
	}
	download, err := srcP.Open(ctx, srcCol.Path, fromRel, nil)
	if err != nil {
		return err
	}
	defer download.Body.Close()
	ctype := download.ContentType
	if ctype == "" {
		ctype = files.TypeForName(entry.Name)
	}
	if _, err := destP.Upload(ctx, destCol.Path, toRel, download.Body, ctype, "", true); err != nil {
		return err
	}
	return srcP.Remove(ctx, srcCol.Path, fromRel, entry.ETag)
}

func readMoveDest(r *http.Request) (moveDest, error) {
	accountID := strings.TrimSpace(r.PostFormValue("dest_account"))
	colEnc := strings.TrimSpace(r.PostFormValue("dest_col"))
	if accountID == "" || colEnc == "" {
		return moveDest{}, fmt.Errorf("choose a destination folder")
	}
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		return moveDest{}, err
	}
	folder, err := files.CleanRelative(r.PostFormValue("dest_folder"))
	if err != nil {
		return moveDest{}, err
	}
	return moveDest{AccountID: accountID, ColEnc: colEnc, Collection: collection, Folder: folder}, nil
}

// filesCopyStream copies one file between collections without removing the source.
func (s *Server) filesCopyStream(ctx context.Context, srcP *files.Provider, srcCol discovery.Collection, destP *files.Provider, destCol discovery.Collection, fromRel, toRel string) error {
	entry, err := srcP.Stat(ctx, srcCol.Path, fromRel)
	if err != nil {
		return err
	}
	if entry.Dir {
		return fmt.Errorf("copying folders is not supported yet")
	}
	download, err := srcP.Open(ctx, srcCol.Path, fromRel, nil)
	if err != nil {
		return err
	}
	defer download.Body.Close()
	ctype := download.ContentType
	if ctype == "" {
		ctype = files.TypeForName(entry.Name)
	}
	_, err = destP.Upload(ctx, destCol.Path, toRel, download.Body, ctype, "", true)
	return err
}
