// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"strings"

	"gitea.mixdep.ru/mix/carrel/internal/fanout"
	"gitea.mixdep.ru/mix/carrel/internal/model"
	"gitea.mixdep.ru/mix/carrel/internal/provider/files"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

const fileSearchMaxHits = 200

// searchFiles matches filenames in one file collection. Only names are compared —
// Carrel does not index file contents.
func (s *Server) searchFiles(ctx context.Context, sess *session.Session, src fanout.Source, query string) ([]fanout.Item, bool, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil, false, nil
	}
	p, acc, err := s.filesProvider(sess, src.AccountID)
	if err != nil {
		return nil, false, err
	}
	col, err := findFileCollection(acc, src.Collection)
	if err != nil {
		return nil, false, err
	}
	var items []fanout.Item
	var queue []string
	queue = append(queue, "")
	for len(queue) > 0 && len(items) < fileSearchMaxHits {
		dir := queue[0]
		queue = queue[1:]
		listing, err := p.List(ctx, col.Path, dir)
		if err != nil {
			continue
		}
		for _, entry := range listing.Entries {
			if entry.Dir {
				queue = append(queue, entry.Rel)
				if strings.Contains(strings.ToLower(entry.Name), query) {
					items = append(items, fileSearchItem(src, entry, s.fileBrowseURL(src.AccountID, src.Collection, entry.Rel)))
					if len(items) >= fileSearchMaxHits {
						break
					}
				}
				continue
			}
			if strings.Contains(strings.ToLower(entry.Name), query) {
				items = append(items, fileSearchItem(src, entry, s.fileDownloadURL(src.AccountID, EncodeCollectionPath(src.Collection), entry.Rel)))
				if len(items) >= fileSearchMaxHits {
					break
				}
			}
		}
	}
	return items, false, nil
}

func fileSearchItem(src fanout.Source, entry files.Entry, url string) fanout.Item {
	subtitle := entry.ContentType
	if subtitle == "" && !entry.Dir {
		subtitle = files.TypeForName(entry.Name)
	}
	if entry.Dir {
		subtitle = "folder"
	}
	row := resultRow{
		Kind: "file", Title: entry.Name, Subtitle: subtitle, URL: url,
		Account: src.AccountLabel, Collection: src.CollectionLabel, Color: src.Color,
		MatchLabel: "filename",
	}
	if entry.HasSize && !entry.Dir {
		row.Subtitle = model.ByteSize(entry.Size) + " · " + subtitle
	}
	return fanout.Item{SourceID: src.ID, Key: strings.ToLower(entry.Name) + "|" + entry.Rel + "|" + src.ID, Data: row}
}

func (s *Server) fileBrowseURL(accountID, collection, rel string) string {
	colEnc := EncodeCollectionPath(collection)
	base := s.Path("/app/files/" + accountID + "/" + colEnc)
	if rel == "" {
		return base
	}
	return folderURL(base, rel)
}
