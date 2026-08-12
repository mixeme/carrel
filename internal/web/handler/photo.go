// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/model"
	"gitea.mixdep.ru/mix/carrel/internal/photo"
	"gitea.mixdep.ru/mix/carrel/internal/provider/contacts"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

const photoUploadMaxBody = 64 << 20 // 64 MiB soft ceiling when config has none

// ContactPhoto serves GET /c/{account}/{col}/{uid}/photo (§11).
func (s *Server) ContactPhoto(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	colEnc := r.PathValue("col")
	uid := r.PathValue("uid")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil || uid == "" {
		http.NotFound(w, r)
		return
	}
	size := strings.ToLower(r.URL.Query().Get("size"))
	if size == "" {
		size = "full"
	}

	sess := SessionFrom(r)
	p, acc, err := s.contactsProvider(sess, accountID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	col, err := findAddressBook(acc, collection)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	collection = normalizeCollectionPath(col.Path)

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	obj, err := p.Get(ctx, collection, objectPathForUID(collection, uid))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	c, _ := obj.Contact()
	etag := obj.ETag
	if etag != "" {
		w.Header().Set("ETag", etag)
		if match := r.Header.Get("If-None-Match"); match != "" && match == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if !c.Photo.Present {
		svg := photo.PlaceholderSVG(uid, c.DisplayName())
		w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
		w.Write(svg)
		return
	}

	if c.Photo.URI != "" {
		s.proxyPhotoURI(w, r, c.Photo.URI)
		return
	}

	inline, ok, err := obj.ExtractPhoto()
	if err != nil || !ok {
		svg := photo.PlaceholderSVG(uid, c.DisplayName())
		w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
		w.Write(svg)
		return
	}
	mt := inline.MediaType
	if mt == "" {
		mt = photo.SniffMediaType(inline.Bytes)
	}
	if !photo.AllowedMediaType(mt) {
		http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
		return
	}

	data := inline.Bytes
	outType := mt
	if size == "thumb" {
		thumb, t, err := photo.Thumbnail(data, s.photoOpts().ThumbSide)
		if err == nil {
			data = thumb
			outType = t
		}
	}
	w.Header().Set("Content-Type", outType)
	w.Write(data)
}

// ContactPhotoPreview serves the current crop preview from the session buffer.
func (s *Server) ContactPhotoPreview(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	colEnc := r.PathValue("col")
	uid := r.PathValue("uid")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	draft, ok := sess.PhotoDraft(photoDraftKey(accountID, collection, uid))
	if !ok || draft.Path == "" {
		http.NotFound(w, r)
		return
	}
	jpeg, err := photo.ProcessFile(draft.Path, photo.CropParams{
		PanX: draft.PanX, PanY: draft.PanY, Zoom: draft.Zoom, Rotate: draft.Rotate,
	}, s.photoOpts())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(jpeg)
}

func (s *Server) contactPhotoAction(w http.ResponseWriter, r *http.Request, accountID, collection, colEnc, uid, action string) {
	sess := SessionFrom(r)
	p, acc, err := s.contactsProvider(sess, accountID)
	if err != nil {
		s.renderContactError(w, r, err, accountID, colEnc)
		return
	}
	col, err := findAddressBook(acc, collection)
	if err != nil {
		s.renderContactError(w, r, err, accountID, colEnc)
		return
	}
	if col.ReadOnly {
		http.Error(w, "this address book is read-only", http.StatusForbidden)
		return
	}
	collection = normalizeCollectionPath(col.Path)
	key := photoDraftKey(accountID, collection, uid)

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	switch action {
	case "cancel_photo":
		sess.ClearPhotoDraft(key)
		http.Redirect(w, r, s.Path("/app/contacts/"+accountID+"/"+colEnc+"/"+urlPathEscape(uid)), http.StatusSeeOther)
		return

	case "upload_photo":
		maxBytes := s.Photo.MaxUploadBytes
		if maxBytes <= 0 {
			maxBytes = photoUploadMaxBody
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		if err := r.ParseMultipartForm(maxBytes); err != nil {
			http.Error(w, "upload too large or invalid", http.StatusBadRequest)
			return
		}
		file, _, err := r.FormFile("photo")
		if err != nil {
			http.Error(w, "choose a photo to upload", http.StatusBadRequest)
			return
		}
		defer file.Close()
		tmp, err := os.CreateTemp("", "carrel-photo-*")
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		tmpPath := tmp.Name()
		if _, err := io.Copy(tmp, file); err != nil {
			tmp.Close()
			_ = os.Remove(tmpPath)
			http.Error(w, "could not store upload", http.StatusBadRequest)
			return
		}
		tmp.Close()

		obj, err := p.Get(ctx, collection, objectPathForUID(collection, uid))
		if err != nil {
			_ = os.Remove(tmpPath)
			s.renderContactError(w, r, err, accountID, colEnc)
			return
		}
		// Validate the file can be processed before keeping it.
		if _, err := photo.ProcessFile(tmpPath, photo.CropParams{Zoom: 1}, s.photoOpts()); err != nil {
			_ = os.Remove(tmpPath)
			v := s.View(r, "Contact")
			v.Error = capitalize(err.Error())
			card, _ := s.loadContactCard(r.Context(), sess, accountID, collection, colEnc, uid)
			v.Data = card
			s.RenderStatus(w, http.StatusBadRequest, "contact.html", v)
			return
		}
		sess.PutPhotoDraft(session.PhotoDraft{
			Key:        key,
			AccountID:  accountID,
			Collection: collection,
			UID:        uid,
			ETag:       obj.ETag,
			Path:       tmpPath,
			Zoom:       1,
		})
		http.Redirect(w, r, s.Path("/app/contacts/"+accountID+"/"+colEnc+"/"+urlPathEscape(uid)), http.StatusSeeOther)
		return

	case "crop_photo":
		panX, _ := strconv.ParseFloat(r.PostFormValue("pan_x"), 64)
		panY, _ := strconv.ParseFloat(r.PostFormValue("pan_y"), 64)
		zoom, _ := strconv.ParseFloat(r.PostFormValue("zoom"), 64)
		rotate, _ := strconv.Atoi(r.PostFormValue("rotate"))
		if _, ok := sess.UpdatePhotoDraft(key, panX, panY, zoom, rotate); !ok {
			http.Error(w, "no upload in progress", http.StatusBadRequest)
			return
		}
		if IsHTMX(r) {
			card, err := s.loadContactCard(r.Context(), sess, accountID, collection, colEnc, uid)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			v := s.View(r, card.Contact.DisplayName())
			v.Data = card
			s.RenderFragment(w, "contact_crop.html", v)
			return
		}
		http.Redirect(w, r, s.Path("/app/contacts/"+accountID+"/"+colEnc+"/"+urlPathEscape(uid)), http.StatusSeeOther)
		return

	case "confirm_photo":
		draft, ok := sess.TakePhotoDraft(key)
		if !ok {
			http.Error(w, "no upload in progress", http.StatusBadRequest)
			return
		}
		defer func() {
			if draft.Path != "" {
				_ = os.Remove(draft.Path)
			}
		}()
		jpeg, err := photo.ProcessFile(draft.Path, photo.CropParams{
			PanX: draft.PanX, PanY: draft.PanY, Zoom: draft.Zoom, Rotate: draft.Rotate,
		}, s.photoOpts())
		if err != nil {
			http.Error(w, capitalize(err.Error()), http.StatusBadRequest)
			return
		}
		obj, err := p.Get(ctx, collection, objectPathForUID(collection, uid))
		if err != nil {
			s.renderContactError(w, r, err, accountID, colEnc)
			return
		}
		if etag := strings.TrimSpace(r.PostFormValue("etag")); etag != "" {
			obj.ETag = etag
		} else if draft.ETag != "" {
			obj.ETag = draft.ETag
		}
		patch := (&model.Patch{}).Set("PHOTO", model.PhotoValue(obj.Version(), jpeg))
		if err := obj.Apply(patch); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if _, err := p.Update(ctx, collection, obj); err != nil {
			if contacts.IsConflict(err) {
				s.showConflict(w, r, sess, accountID, collection, colEnc, uid, err)
				return
			}
			s.renderContactError(w, r, err, accountID, colEnc)
			return
		}
		http.Redirect(w, r, s.Path("/app/contacts/"+accountID+"/"+colEnc+"/"+urlPathEscape(uid)), http.StatusSeeOther)
		return

	case "delete_photo":
		obj, err := p.Get(ctx, collection, objectPathForUID(collection, uid))
		if err != nil {
			s.renderContactError(w, r, err, accountID, colEnc)
			return
		}
		if etag := strings.TrimSpace(r.PostFormValue("etag")); etag != "" {
			obj.ETag = etag
		}
		if err := obj.Apply((&model.Patch{}).Remove("PHOTO")); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if _, err := p.Update(ctx, collection, obj); err != nil {
			if contacts.IsConflict(err) {
				s.showConflict(w, r, sess, accountID, collection, colEnc, uid, err)
				return
			}
			s.renderContactError(w, r, err, accountID, colEnc)
			return
		}
		http.Redirect(w, r, s.Path("/app/contacts/"+accountID+"/"+colEnc+"/"+urlPathEscape(uid)), http.StatusSeeOther)
		return
	}
	http.Error(w, "bad request", http.StatusBadRequest)
}

func (s *Server) proxyPhotoURI(w http.ResponseWriter, r *http.Request, rawURL string) {
	if s.Guard == nil {
		http.Error(w, "not configured", http.StatusBadGateway)
		return
	}
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		http.Error(w, "bad photo URL", http.StatusBadRequest)
		return
	}
	if err := s.Guard.ValidateURL(r.Context(), u); err != nil {
		http.Error(w, "photo URL not allowed", http.StatusForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	resp, err := s.Guard.HTTPClient().Do(req)
	if err != nil {
		http.Error(w, "could not fetch photo", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		http.Error(w, "upstream photo error", http.StatusBadGateway)
		return
	}
	ct := resp.Header.Get("Content-Type")
	if i := strings.Index(ct, ";"); i > 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		http.Error(w, "could not read photo", http.StatusBadGateway)
		return
	}
	if ct == "" {
		ct = photo.SniffMediaType(data)
	}
	if !photo.AllowedMediaType(ct) {
		http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Write(data)
}

func (s *Server) photoOpts() photo.Options {
	opts := photo.Options{
		MaxSide:     s.Photo.MaxSide,
		JPEGQuality: s.Photo.JPEGQuality,
		MaxPixels:   s.Photo.MaxPixels,
		ThumbSide:   s.Photo.ThumbSide,
	}
	return opts
}
