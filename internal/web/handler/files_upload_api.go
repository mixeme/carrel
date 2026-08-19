// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path"
	"strings"

	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/provider/files"
)

type uploadReply struct {
	OK       bool   `json:"ok"`
	Name     string `json:"name,omitempty"`
	Error    string `json:"error,omitempty"`
	Conflict bool   `json:"conflict,omitempty"`
}

func wantsJSONUpload(r *http.Request) bool {
	if strings.EqualFold(r.Header.Get("X-Requested-With"), "XMLHttpRequest") {
		return true
	}
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "application/json")
}

func writeUploadJSON(w http.ResponseWriter, status int, reply uploadReply) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(reply)
}

// fileUploadXHR streams one file from an XHR upload and answers in JSON so the
// queue can show progress and ask about name clashes.
func (s *Server) fileUploadXHR(w http.ResponseWriter, r *http.Request, p *files.Provider, col discovery.Collection, rel string) {
	maxBytes := s.filesMaxUpload()
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	part, header, err := s.multipartFile(r, "file")
	if err != nil {
		writeUploadJSON(w, http.StatusBadRequest, uploadReply{Error: capitalize(err.Error())})
		return
	}
	defer part.Close()
	name, err := files.CleanName(path.Base(strings.ReplaceAll(header.Filename, "\\", "/")))
	if err != nil {
		writeUploadJSON(w, http.StatusBadRequest, uploadReply{Error: "That file name cannot be used."})
		return
	}
	mode := strings.ToLower(strings.TrimSpace(r.FormValue("upload_mode")))
	target := files.Join(rel, name)
	ctype := contentTypeOf(header.Header.Get("Content-Type"), name)
	ctx := r.Context()

	switch mode {
	case "keep-both":
		seeker, ok := part.(io.ReadSeeker)
		if !ok {
			data, readErr := io.ReadAll(part)
			if readErr != nil {
				writeUploadJSON(w, http.StatusBadRequest, uploadReply{Error: "That upload could not be read."})
				return
			}
			seeker = newMemReadSeeker(data)
		}
		stored, _, err := p.UploadNew(ctx, col.Path, rel, name, seeker, ctype)
		if err != nil {
			writeUploadJSON(w, http.StatusBadGateway, uploadReply{Error: userFacingDAVError(err)})
			return
		}
		writeUploadJSON(w, http.StatusOK, uploadReply{OK: true, Name: stored})
		return
	case "replace":
		if _, err := p.Upload(ctx, col.Path, target, part, ctype, "", false); err != nil {
			writeUploadJSON(w, http.StatusBadGateway, uploadReply{Error: userFacingDAVError(err)})
			return
		}
		writeUploadJSON(w, http.StatusOK, uploadReply{OK: true, Name: name})
		return
	}

	if _, err := p.Upload(ctx, col.Path, target, part, ctype, "", true); err != nil {
		if isAlreadyThere(err) {
			writeUploadJSON(w, http.StatusConflict, uploadReply{Conflict: true, Name: name, Error: "A file called " + name + " is already there."})
			return
		}
		writeUploadJSON(w, http.StatusBadGateway, uploadReply{Error: userFacingDAVError(err)})
		return
	}
	writeUploadJSON(w, http.StatusOK, uploadReply{OK: true, Name: name})
}

type memReadSeeker struct {
	data []byte
	off  int64
}

func newMemReadSeeker(data []byte) *memReadSeeker { return &memReadSeeker{data: data} }

func (b *memReadSeeker) Read(p []byte) (int, error) {
	if b.off >= int64(len(b.data)) {
		return 0, io.EOF
	}
	n := copy(p, b.data[b.off:])
	b.off += int64(n)
	return n, nil
}

func (b *memReadSeeker) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = b.off + offset
	case io.SeekEnd:
		abs = int64(len(b.data)) + offset
	default:
		return 0, errors.New("files: invalid seek")
	}
	if abs < 0 {
		return 0, errors.New("files: negative position")
	}
	b.off = abs
	return abs, nil
}

func (b *memReadSeeker) Close() error { return nil }
