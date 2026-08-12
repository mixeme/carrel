// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package calendar

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

type WriteResult struct {
	Object     *model.Object
	ETag       string
	Loss       model.PropertyLoss
	ReportLoss bool
	Verified   bool
}

type ConflictError struct {
	Path   string
	Local  *model.Object
	Remote *model.Object
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("calendar: %s changed on the server since it was read", e.Path)
}

func IsConflict(err error) bool {
	var conflict *ConflictError
	return errors.As(err, &conflict)
}

func (p *Provider) Create(ctx context.Context, collection string, obj *model.Object) (*WriteResult, error) {
	collection = normalizeCollection(collection)
	if collection == "" {
		return nil, errors.New("calendar: collection path is required")
	}
	if obj == nil {
		return nil, errors.New("calendar: object is required")
	}
	if obj.UID() == "" {
		return nil, errors.New("calendar: object has no UID")
	}
	path := obj.Path
	if path == "" {
		path = collection + url.PathEscape(obj.UID()) + ".ics"
	}
	return p.put(ctx, collection, path, obj, dav.PutOptions{
		ContentType: dav.MediaTypeCalendar,
		IfNoneMatch: true,
	})
}

func (p *Provider) Update(ctx context.Context, collection string, obj *model.Object) (*WriteResult, error) {
	collection = normalizeCollection(collection)
	if obj == nil {
		return nil, errors.New("calendar: object is required")
	}
	if obj.Path == "" {
		return nil, errors.New("calendar: object has no path")
	}
	if obj.ETag == "" {
		return nil, fmt.Errorf("calendar: refusing to overwrite %s without the version it was read at", obj.Path)
	}
	return p.put(ctx, collection, obj.Path, obj, dav.PutOptions{
		ContentType: dav.MediaTypeCalendar,
		IfMatch:     obj.ETag,
	})
}

func (p *Provider) put(ctx context.Context, collection, path string, obj *model.Object, opts dav.PutOptions) (*WriteResult, error) {
	body, err := obj.Marshal()
	if err != nil {
		return nil, err
	}
	sent := obj.Clone()
	sent.Path = path
	etag, err := p.client.PutOpts(ctx, path, bytes.NewReader(body), opts)
	if err != nil {
		if dav.IsPreconditionFailed(err) {
			return nil, p.conflict(ctx, collection, path, sent)
		}
		return nil, fmt.Errorf("calendar: write %s: %w", path, err)
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

func (p *Provider) Delete(ctx context.Context, collection, objectPath, etag string) error {
	collection = normalizeCollection(collection)
	if objectPath == "" {
		return errors.New("calendar: object path is required")
	}
	if etag == "" {
		return fmt.Errorf("calendar: refusing to delete %s without the version it was read at", objectPath)
	}
	if err := p.client.Delete(ctx, objectPath, etag); err != nil {
		if dav.IsPreconditionFailed(err) {
			return p.conflict(ctx, collection, objectPath, nil)
		}
		return fmt.Errorf("calendar: delete %s: %w", objectPath, err)
	}
	p.invalidate(collection)
	return nil
}

func (p *Provider) conflict(ctx context.Context, collection, path string, local *model.Object) error {
	p.invalidate(collection)
	conflict := &ConflictError{Path: path, Local: local}
	if remote, err := p.Get(ctx, collection, path); err == nil {
		conflict.Remote = remote
	}
	return conflict
}

func (p *Provider) invalidate(collection string) {
	if p.cache != nil {
		p.cache.InvalidateCollection(p.accountID, collection)
	}
}

func (p *Provider) LossReport() model.LossReport {
	return p.losses.Report(p.accountID)
}
