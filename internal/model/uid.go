// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"encoding/hex"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
)

// NewUID returns a random identifier for a new object, shaped like a version 4
// UUID because that is what other CardDAV clients expect to find in a UID.
func NewUID() (string, error) {
	b, err := crypto.Random(16)
	if err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	buf := make([]byte, 0, 36)
	hexAppend := func(part []byte) { buf = append(buf, []byte(hex.EncodeToString(part))...) }
	hexAppend(b[0:4])
	buf = append(buf, '-')
	hexAppend(b[4:6])
	buf = append(buf, '-')
	hexAppend(b[6:8])
	buf = append(buf, '-')
	hexAppend(b[8:10])
	buf = append(buf, '-')
	hexAppend(b[10:16])
	return string(buf), nil
}
