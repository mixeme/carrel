// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/textproto"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/provider/files"
)

var (
	errUploadInvalid = errors.New("that upload could not be read")
	errUploadNoFile  = errors.New("choose a file to upload")
)

// FileDownload streams one file from a collection to the browser.
//
// Nothing is buffered on the way: the DAV body is copied straight into the
// response, which is what §7 fixes `Get` at an `io.ReadCloser` for. A 200 MB
// download costs one copy buffer rather than 200 MB of resident memory, and it
// starts arriving before the server has finished sending it.
//
// It is served with `nosniff` and an explicit `Content-Disposition` filename,
// because these are somebody else's files and a browser deciding for itself that
// one of them is HTML is the whole of §24.4's concern here.
func (s *Server) FileDownload(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	col, err := findFileCollection(acc, collection)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	rng := parseSingleRange(r.Header.Get("Range"))
	download, err := p.Open(ctx, col.Path, rel, rng)
	if err != nil {
		http.Error(w, userFacingDAVError(err), downloadStatus(err))
		return
	}
	defer download.Body.Close()
	s.serveStream(w, r, download, files.Base(rel), rng != nil)
}

// serveStream writes the headers a user's own file is served under and then
// copies the body. It is shared by the file browser and by opening an
// attachment, because the two differ only in how the path was arrived at.
func (s *Server) serveStream(w http.ResponseWriter, r *http.Request, download *files.Download, name string, ranged bool) {
	ctype := strings.TrimSpace(download.ContentType)
	if ctype == "" {
		ctype = files.TypeForName(name)
	}
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	h := w.Header()
	h.Set("Content-Type", ctype)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Disposition", inlineDisposition(name, ctype))
	h.Set("Cache-Control", "private, no-store")
	if download.ETag != "" {
		h.Set("ETag", download.ETag)
	}
	if ranged {
		w.WriteHeader(http.StatusPartialContent)
	}
	if r.Method == http.MethodHead {
		return
	}
	if _, err := io.Copy(w, download.Body); err != nil {
		// The status line is long gone by now; all that is left is a log entry.
		s.logError("stream file", err)
	}
}

// displayInline is the short list of types a browser may render in place. A
// picture is the reason attachments exist (§23.10), so it is shown rather than
// downloaded; everything else is offered as a file, which is also what keeps
// somebody else's HTML out of this origin.
var displayInline = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
	"text/plain": true,
	// application/pdf is deliberately absent: a PDF viewer is a scripting
	// surface, and §23.10 puts previews out of scope anyway.
}

func inlineDisposition(name, ctype string) string {
	safe := sanitizeFilename(name)
	if safe == "" {
		safe = "download"
	}
	kind := "attachment"
	if displayInline[ctype] {
		kind = "inline"
	}
	// The ASCII name is what every browser reads; the UTF-8 form carries a name
	// that does not survive transliteration (RFC 6266).
	out := kind + `; filename="` + safe + `"`
	if name != safe {
		out += "; filename*=UTF-8''" + escapeExtFilename(name)
	}
	return out
}

// escapeExtFilename percent-encodes a filename for the RFC 5987 form, where the
// set of characters that may go bare is narrower than in a path.
func escapeExtFilename(name string) string {
	const safe = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#$&+-.^_`|~"
	var b strings.Builder
	for i := 0; i < len(name); i++ {
		c := name[i]
		if strings.IndexByte(safe, c) >= 0 {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		const hex = "0123456789ABCDEF"
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0x0f])
	}
	return b.String()
}

// parseSingleRange reads the one range form worth passing on: a single byte
// range from the start offset. Multipart ranges are not forwarded, because
// nothing here needs them and a partial implementation of them would be worse
// than none.
func parseSingleRange(header string) *dav.Range {
	header = strings.TrimSpace(header)
	if !strings.HasPrefix(header, "bytes=") || strings.Contains(header, ",") {
		return nil
	}
	spec := strings.TrimPrefix(header, "bytes=")
	start, end, found := strings.Cut(spec, "-")
	if !found || strings.TrimSpace(start) == "" {
		return nil
	}
	rng := &dav.Range{}
	from, err := strconv.ParseInt(strings.TrimSpace(start), 10, 64)
	if err != nil {
		return nil
	}
	rng.Start = from
	if trimmed := strings.TrimSpace(end); trimmed != "" {
		to, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil {
			return nil
		}
		rng.End = to
	}
	if rng.Start < 0 || (rng.End > 0 && rng.End < rng.Start) {
		return nil
	}
	return rng
}

func downloadStatus(err error) int {
	if code := dav.StatusCode(err); code == http.StatusNotFound {
		return http.StatusNotFound
	}
	return http.StatusBadGateway
}

// fileUpload streams a multipart part onto the DAV server.
func (s *Server) fileUpload(w http.ResponseWriter, r *http.Request, p *files.Provider, col discovery.Collection, base string) {
	maxBytes := s.filesMaxUpload()
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	part, header, err := s.multipartFile(r, "file")
	if err != nil {
		http.Error(w, capitalize(err.Error()), http.StatusBadRequest)
		return
	}
	defer part.Close()
	rel, err := files.CleanRelative(r.FormValue(fieldPath))
	if err != nil {
		http.Error(w, "bad path", http.StatusForbidden)
		return
	}
	name, err := files.CleanName(path.Base(strings.ReplaceAll(header.Filename, "\\", "/")))
	if err != nil {
		http.Error(w, "That file name cannot be used.", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
	defer cancel()
	// A create rather than a blind write: something already at that name is a
	// refusal and a fresh name, never a silent replacement of somebody's file.
	target := files.Join(rel, name)
	if _, err := p.Upload(ctx, col.Path, target, part, contentTypeOf(header.Header.Get("Content-Type"), name), "", true); err != nil {
		if isAlreadyThere(err) {
			s.redirectNotice(w, r, folderURL(base, rel),
				"A file called "+name+" is already there. Nothing was overwritten — rename it and upload again.")
			return
		}
		http.Error(w, userFacingDAVError(err), http.StatusBadGateway)
		return
	}
	s.redirectNotice(w, r, folderURL(base, rel), "Uploaded "+name+".")
}

// multipartFile opens the named part of a multipart body as a stream.
//
// It prefers the reader over ParseMultipartForm, because parsing spools the whole
// upload before the first byte reaches the DAV server and §23.10 asks for a
// stream. That is possible whenever the token arrived in a header — every htmx
// post, and the paste and drop paths of §23.10.
//
// A plain HTML form cannot set a header, so its token is a field, and the CSRF
// check of §24.5 has to read the form to find it. When that has already happened
// the body is spooled and there is nothing to be gained by pretending otherwise:
// the part is taken from the parsed form, which Go has put in a temporary file
// rather than in memory, and the server removes it when the request ends (§24.4).
func (s *Server) multipartFile(r *http.Request, field string) (io.ReadCloser, *multipartHeader, error) {
	if r.MultipartForm != nil {
		file, header, err := r.FormFile(field)
		if err != nil || header == nil || strings.TrimSpace(header.Filename) == "" {
			return nil, nil, errUploadNoFile
		}
		return file, &multipartHeader{Filename: header.Filename, Header: textproto.MIMEHeader(header.Header)}, nil
	}
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, nil, errUploadInvalid
	}
	if r.Form == nil {
		r.Form = make(url.Values)
	}
	for {
		part, err := reader.NextPart()
		if err != nil {
			return nil, nil, errUploadNoFile
		}
		if part.FormName() != field {
			value, readErr := io.ReadAll(io.LimitReader(part, 1<<16))
			part.Close()
			if readErr != nil {
				return nil, nil, errUploadInvalid
			}
			r.Form[part.FormName()] = append(r.Form[part.FormName()], string(value))
			continue
		}
		if strings.TrimSpace(part.FileName()) == "" {
			part.Close()
			return nil, nil, errUploadNoFile
		}
		return part, &multipartHeader{Filename: part.FileName(), Header: part.Header}, nil
	}
}

type multipartHeader struct {
	Filename string
	Header   textproto.MIMEHeader
}

func contentTypeOf(claimed, name string) string {
	// The client's claim is a hint and nothing rests on it: what is served back
	// carries `nosniff` and an explicit disposition (§24.4). Storing it is still
	// worth doing, because it is what other clients will read.
	if t, _, err := mime.ParseMediaType(strings.TrimSpace(claimed)); err == nil && t != "" && t != "application/octet-stream" {
		return t
	}
	if t := files.TypeForName(name); t != "" {
		return t
	}
	return "application/octet-stream"
}

func isAlreadyThere(err error) bool {
	return dav.IsPreconditionFailed(err) || dav.StatusCode(err) == http.StatusMethodNotAllowed
}
