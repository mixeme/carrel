// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package contacts

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/model"
)

// SearchResult is what an addressbook-query answered with.
type SearchResult struct {
	Collection string
	Objects    []*model.Object
}

// Search returns the cards of a collection whose name, address, number,
// organisation or note contains text (§16).
//
// CardDAV lets the client join property filters with "any of", so unlike the
// calendar side this is a single report per collection.
func (p *Provider) Search(ctx context.Context, collection, text string) (*SearchResult, error) {
	collection = normalizeCollection(collection)
	if collection == "" {
		return nil, errors.New("contacts: collection path is required")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, errors.New("contacts: search text is required")
	}
	ms, err := p.client.Report(ctx, collection, dav.DepthOne, dav.NewAddressBookQuery(text))
	if err != nil {
		return nil, fmt.Errorf("contacts: search %s: %w", collection, err)
	}
	var objects []*model.Object
	for _, resp := range ms.Responses {
		path, err := resp.Path()
		if err != nil || normalizeCollection(path) == collection {
			continue
		}
		var data dav.AddressData
		if err := resp.DecodeProp(&data); err != nil {
			continue
		}
		var etag dav.GetETag
		_ = resp.DecodeProp(&etag)
		body := []byte(data.Data)
		obj, err := model.ParseVCard(path, strings.TrimSpace(etag.ETag), body)
		if err != nil {
			continue
		}
		if obj.ETag != "" && p.cache != nil {
			p.cache.PutBody(p.accountID, collection, bodyKey(path, obj.ETag), body)
		}
		objects = append(objects, obj)
	}
	sort.SliceStable(objects, func(i, j int) bool { return objects[i].Path < objects[j].Path })
	return &SearchResult{Collection: collection, Objects: objects}, nil
}
