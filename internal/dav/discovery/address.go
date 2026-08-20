// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package discovery

import (
	"fmt"
	"path"
	"strings"

	"gitea.mixdep.ru/mix/carrel/internal/model"
)

const defaultCollectionAddress = "collection"

// AddressFromName derives a collection address from a display name (§10.1).
func AddressFromName(name string) string {
	slug := model.Slug(name)
	if slug == "" {
		return defaultCollectionAddress
	}
	return slug
}

// ValidateAddress rejects addresses that would be unsafe on the wire (§24.4).
func ValidateAddress(addr string) error {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return fmt.Errorf("enter an address")
	}
	if strings.Contains(addr, "/") || strings.Contains(addr, "..") {
		return fmt.Errorf("the address must not contain / or ..")
	}
	if strings.HasPrefix(addr, ".") {
		return fmt.Errorf("the address must not start with a dot")
	}
	return nil
}

// UniqueAddress picks a free address under home from the desired slug, adding a
// numeric suffix on collision (§10.1).
func UniqueAddress(home, desired string, existing []Collection) string {
	base := strings.Trim(desired, "-")
	if base == "" {
		base = defaultCollectionAddress
	}
	taken := make(map[string]struct{}, len(existing))
	homeNorm := normalizePath(home)
	for _, col := range existing {
		taken[collectionLeaf(homeNorm, col.Path)] = struct{}{}
	}
	candidate := base
	for i := 0; ; i++ {
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", base, i+1)
		}
		if _, ok := taken[candidate]; !ok {
			return candidate
		}
	}
}

// CollectionHref joins a home-set path and collection address (§10.1).
func CollectionHref(home, address string) string {
	home = strings.TrimSuffix(normalizePath(home), "/")
	addr := strings.Trim(address, "/")
	return normalizePath(path.Join(home, addr))
}

func collectionLeaf(home, colPath string) string {
	home = normalizePath(home)
	colPath = normalizePath(colPath)
	if !strings.HasPrefix(colPath, home) {
		return strings.Trim(strings.TrimPrefix(colPath, "/"), "/")
	}
	rest := strings.TrimPrefix(colPath, home)
	return strings.Trim(rest, "/")
}
