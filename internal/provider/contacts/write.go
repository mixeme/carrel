// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package contacts

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/model"
)

// WriteResult is the outcome of a write that the server accepted.
type WriteResult struct {
	// Object is the stored object as the server returns it, not the one that
	// was sent: it is what the next edit has to be based on.
	Object *model.Object
	ETag   string
	// Loss is what the server did to the object beyond storing it (§8). It
	// is empty when the stored object matches what was sent, and it is never
	// a reason to fail the write — the write already happened.
	Loss model.PropertyLoss
	// ReportLoss is true when this account has not lost these properties
	// before, which is when the person is told inline; after that it belongs
	// in the account's details (§8).
	ReportLoss bool
	// Verified is false when the stored object could not be read back, so
	// nothing is known about loss either way.
	Verified bool
}

// ConflictError is a refused precondition: the object changed on the server
// after it was read (§9).
//
// It carries both versions so the caller can show the difference and let the
// person choose. There is no automatic overwrite, in either direction.
type ConflictError struct {
	Path string
	// Local is the edit that was refused.
	Local *model.Object
	// Remote is the server's current version, nil when re-reading it also
	// failed.
	Remote *model.Object
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("contacts: %s changed on the server since it was read", e.Path)
}

// IsConflict reports whether err is a refused precondition.
func IsConflict(err error) bool {
	var conflict *ConflictError
	return errors.As(err, &conflict)
}

// Create stores a new object in a collection.
//
// The write is conditional on nothing being there yet, so a UID that already
// exists on the server comes back as a conflict instead of overwriting the
// contact that holds it.
func (p *Provider) Create(ctx context.Context, collection string, obj *model.Object) (*WriteResult, error) {
	collection = normalizeCollection(collection)
	if collection == "" {
		return nil, errors.New("contacts: collection path is required")
	}
	if obj == nil {
		return nil, errors.New("contacts: object is required")
	}
	if obj.UID() == "" {
		return nil, errors.New("contacts: object has no UID")
	}

	path := obj.Path
	if path == "" {
		path = collection + url.PathEscape(obj.UID()) + ".vcf"
	}
	return p.put(ctx, collection, path, obj, dav.PutOptions{
		ContentType: dav.MediaTypeVCard,
		IfNoneMatch: true,
	})
}

// Update stores an edited object.
//
// It requires the version the edit was based on: a write without a precondition
// is an unconditional overwrite of whatever is there now, which is exactly what
// §9 forbids.
func (p *Provider) Update(ctx context.Context, collection string, obj *model.Object) (*WriteResult, error) {
	collection = normalizeCollection(collection)
	if obj == nil {
		return nil, errors.New("contacts: object is required")
	}
	if obj.Path == "" {
		return nil, errors.New("contacts: object has no path")
	}
	if obj.ETag == "" {
		return nil, fmt.Errorf("contacts: refusing to overwrite %s without the version it was read at", obj.Path)
	}
	return p.put(ctx, collection, obj.Path, obj, dav.PutOptions{
		ContentType: dav.MediaTypeVCard,
		IfMatch:     obj.ETag,
	})
}

func (p *Provider) put(ctx context.Context, collection, path string, obj *model.Object, opts dav.PutOptions) (*WriteResult, error) {
	body, err := obj.Marshal()
	if err != nil {
		return nil, err
	}

	// Compare against a copy taken before the write: the object handed in
	// keeps being usable, and the comparison cannot be fooled by anything the
	// write does to it.
	sent := obj.Clone()
	sent.Path = path

	etag, err := p.client.PutOpts(ctx, path, bytes.NewReader(body), opts)
	if err != nil {
		if dav.IsPreconditionFailed(err) {
			return nil, p.conflict(ctx, collection, path, sent)
		}
		return nil, fmt.Errorf("contacts: write %s: %w", path, err)
	}

	p.invalidate(collection)

	result := &WriteResult{ETag: strings.TrimSpace(etag)}
	if !p.verify {
		stored := sent.Clone()
		stored.ETag = result.ETag
		result.Object = stored
		return result, nil
	}

	stored, err := p.Get(ctx, collection, path)
	if err != nil {
		// The write succeeded; not being able to read it back is worth
		// saying, but it is not a failed save.
		fallback := sent.Clone()
		fallback.ETag = result.ETag
		result.Object = fallback
		return result, nil
	}

	result.Object = stored
	if stored.ETag != "" {
		result.ETag = stored.ETag
	}
	loss, err := model.Compare(sent, stored)
	if err != nil {
		return result, nil
	}
	result.Verified = true
	result.Loss = loss
	result.ReportLoss = p.losses.Record(p.accountID, loss)
	return result, nil
}

// Delete removes an object, conditional on the version it was read at.
func (p *Provider) Delete(ctx context.Context, collection, objectPath, etag string) error {
	collection = normalizeCollection(collection)
	if objectPath == "" {
		return errors.New("contacts: object path is required")
	}
	if etag == "" {
		return fmt.Errorf("contacts: refusing to delete %s without the version it was read at", objectPath)
	}
	if err := p.client.Delete(ctx, objectPath, etag); err != nil {
		if dav.IsPreconditionFailed(err) {
			return p.conflict(ctx, collection, objectPath, nil)
		}
		return fmt.Errorf("contacts: delete %s: %w", objectPath, err)
	}
	p.invalidate(collection)
	return nil
}

// conflict builds the error for a refused precondition, re-reading the server's
// version so the caller has something to show the difference against (§9).
func (p *Provider) conflict(ctx context.Context, collection, path string, local *model.Object) error {
	p.invalidate(collection)
	conflict := &ConflictError{Path: path, Local: local}
	if remote, err := p.Get(ctx, collection, path); err == nil {
		conflict.Remote = remote
	}
	return conflict
}

// invalidate drops the cached view of a collection after a write. The ETag map
// and every body in it are one version behind as soon as the collection changes
// (§12).
func (p *Provider) invalidate(collection string) {
	if p.cache == nil {
		return
	}
	p.cache.InvalidateCollection(p.accountID, collection)
}

// LossReport returns what this account's server has been dropping (§8).
func (p *Provider) LossReport() model.LossReport {
	return p.losses.Report(p.accountID)
}

func normalizeCollection(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return path
}
