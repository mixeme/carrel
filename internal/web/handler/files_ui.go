// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"strconv"
	"strings"

	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/provider/files"
)

// fileIconKind picks the row icon from name and type. Folders, images and
// documents are the only shapes worth drawing in the list.
func fileIconKind(name, contentType string, dir bool) string {
	if dir {
		return "folder"
	}
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if strings.HasPrefix(ct, "image/") {
		return "image"
	}
	lower := strings.ToLower(name)
	if strings.HasPrefix(ct, "text/") || strings.Contains(ct, "pdf") ||
		strings.HasSuffix(lower, ".pdf") || strings.HasSuffix(lower, ".md") ||
		strings.HasSuffix(lower, ".markdown") || strings.HasSuffix(lower, ".txt") {
		return "doc"
	}
	return "file"
}

// filesDeleteBatch removes each selected path and reports per-item success.
func (s *Server) filesDeleteBatch(ctx context.Context, p *files.Provider, col discovery.Collection, folder string, targets, etags []string) string {
	if len(targets) == 0 {
		return "Nothing was selected."
	}
	var lines []string
	ok := 0
	for i, raw := range targets {
		target, err := files.CleanRelative(raw)
		if err != nil || target == "" {
			lines = append(lines, raw+": refused — bad path.")
			continue
		}
		etag := ""
		if i < len(etags) {
			etag = strings.TrimSpace(etags[i])
		}
		if err := p.Remove(ctx, col.Path, target, etag); err != nil {
			lines = append(lines, files.Base(target)+": "+userFacingDAVError(err))
			continue
		}
		ok++
	}
	if ok == len(targets) {
		if ok == 1 {
			return "Deleted 1 item from the server."
		}
		return "Deleted " + strconv.Itoa(ok) + " items from the server."
	}
	if ok == 0 {
		return strings.Join(lines, " ")
	}
	return "Deleted " + strconv.Itoa(ok) + " of " + strconv.Itoa(len(targets)) + ". " + strings.Join(lines, " ")
}
