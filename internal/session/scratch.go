// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"os"

	"gitea.mixdep.ru/mix/carrel/internal/model"
)

// ConflictDraft holds a refused edit so the conflict screen can offer to apply
// it against the server's current version (§9).
type ConflictDraft struct {
	Key        string
	AccountID  string
	Collection string
	Path       string
	UID        string
	Body       []byte
}

// PhotoDraft is an uploaded original waiting for crop confirmation (§11).
type PhotoDraft struct {
	Key        string
	AccountID  string
	Collection string
	UID        string
	ETag       string
	Path       string // temp file holding the oriented source image
	PanX       float64
	PanY       float64
	Zoom       float64
	Rotate     int // degrees, multiples of 90
}

// Losses returns the session's property-loss registry (§8).
func (s *Session) Losses() *model.LossRegistry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.losses == nil {
		s.losses = model.NewLossRegistry(nil)
	}
	return s.losses
}

// PutConflict stores a refused edit under key.
func (s *Session) PutConflict(d ConflictDraft) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conflicts == nil {
		s.conflicts = make(map[string]ConflictDraft)
	}
	s.conflicts[d.Key] = d
}

// TakeConflict returns and removes a conflict draft.
func (s *Session) TakeConflict(key string) (ConflictDraft, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.conflicts[key]
	if ok {
		delete(s.conflicts, key)
	}
	return d, ok
}

// PeekConflict returns a conflict draft without removing it.
func (s *Session) PeekConflict(key string) (ConflictDraft, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.conflicts[key]
	return d, ok
}

// ClearConflict drops a conflict draft.
func (s *Session) ClearConflict(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.conflicts, key)
}

// PutPhotoDraft stores an uploaded photo awaiting crop.
func (s *Session) PutPhotoDraft(d PhotoDraft) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.photos == nil {
		s.photos = make(map[string]PhotoDraft)
	}
	if old, ok := s.photos[d.Key]; ok && old.Path != "" && old.Path != d.Path {
		_ = os.Remove(old.Path)
	}
	s.photos[d.Key] = d
}

// PhotoDraft returns a photo draft by key.
func (s *Session) PhotoDraft(key string) (PhotoDraft, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.photos[key]
	return d, ok
}

// UpdatePhotoDraft replaces crop parameters on an existing draft.
func (s *Session) UpdatePhotoDraft(key string, panX, panY, zoom float64, rotate int) (PhotoDraft, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.photos[key]
	if !ok {
		return PhotoDraft{}, false
	}
	d.PanX = panX
	d.PanY = panY
	d.Zoom = zoom
	d.Rotate = rotate
	s.photos[key] = d
	return d, true
}

// TakePhotoDraft returns and removes a photo draft. The caller owns the temp file.
func (s *Session) TakePhotoDraft(key string) (PhotoDraft, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.photos[key]
	if ok {
		delete(s.photos, key)
	}
	return d, ok
}

// ClearPhotoDraft removes a draft and deletes its temp file.
func (s *Session) ClearPhotoDraft(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.photos[key]; ok {
		if d.Path != "" {
			_ = os.Remove(d.Path)
		}
		delete(s.photos, key)
	}
}

func (s *Session) wipeScratch() {
	for _, d := range s.photos {
		if d.Path != "" {
			_ = os.Remove(d.Path)
		}
	}
	s.photos = nil
	s.conflicts = nil
	s.losses = nil
}