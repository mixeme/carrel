// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/photo"
	"gitea.mixdep.ru/mix/carrel/internal/provider/files"
)

const (
	fileThumbMaxBytes = 16 << 20 // preview threshold, not an upload limit
	fileThumbSide     = 256
)

// FileThumb serves a downscaled preview for the file tiles view (wave 2.7).
// The browser never downloads the original for a tile; Carrel decodes on the server.
func (s *Server) FileThumb(w http.ResponseWriter, r *http.Request) {
	accountID, colEnc := r.PathValue("account"), r.PathValue("col")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	rel, err := files.CleanRelative(r.URL.Query().Get(fieldPath))
	if err != nil || rel == "" {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	p, acc, err := s.filesProvider(sess, accountID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	col, err := findFileCollection(acc, collection)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	entry, err := p.Stat(ctx, col.Path, rel)
	if err != nil || entry.Dir {
		http.NotFound(w, r)
		return
	}
	if entry.HasSize && entry.Size > fileThumbMaxBytes {
		http.Error(w, "too large for preview", http.StatusRequestEntityTooLarge)
		return
	}
	ctype := entry.ContentType
	if ctype == "" {
		ctype = files.TypeForName(entry.Name)
	}
	if !strings.HasPrefix(ctype, "image/") || !photo.AllowedMediaType(ctype) {
		http.NotFound(w, r)
		return
	}
	cacheKey := col.Path + "|" + rel
	if cached, ok := sess.Cache().GetThumb(accountID, cacheKey, entry.ETag); ok {
		w.Header().Set("Content-Type", cached.MediaType)
		w.Header().Set("Cache-Control", "private, max-age=3600")
		w.Write(cached.Bytes)
		return
	}
	download, err := p.Open(ctx, col.Path, rel, nil)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer download.Body.Close()
	limited := io.LimitReader(download.Body, fileThumbMaxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		http.Error(w, "preview failed", http.StatusBadGateway)
		return
	}
	if int64(len(data)) > fileThumbMaxBytes {
		http.Error(w, "too large for preview", http.StatusRequestEntityTooLarge)
		return
	}
	thumb, outType, err := photo.Thumbnail(data, fileThumbSide)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sess.Cache().PutThumb(accountID, cacheKey, entry.ETag, outType, thumb)
	w.Header().Set("Content-Type", outType)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Write(thumb)
}

func (s *Server) fileThumbURL(accountID, colEnc, rel string) string {
	return s.Path("/t/"+accountID+"/"+colEnc) + "?" + url.Values{fieldPath: {rel}}.Encode()
}
