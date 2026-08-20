// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package discovery

import (
	"hash/fnv"
	"strings"
)

// CollectionPalette is the set of calendar colours from the mockups (§10.1).
var CollectionPalette = []string{
	"#4A6B52", "#3F6E8C", "#6B4E7A", "#A05A2C", "#8C6239", "#9A9280",
}

// ColorFromAddress picks a deterministic colour for an address book (§2, §10.1).
func ColorFromAddress(address string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.ToLower(strings.TrimSpace(address))))
	return CollectionPalette[int(h.Sum32())%len(CollectionPalette)]
}
