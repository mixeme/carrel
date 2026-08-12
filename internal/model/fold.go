// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"bytes"
	"unicode/utf8"
)

// foldLimit is the octet count a content line may reach before it is folded
// (RFC 6350 §3.2). The encoder Carrel builds on emits one line per property
// however long it gets, and an inline photo is tens of kilobytes on a single
// line — enough for some servers and other clients to refuse the object.
const foldLimit = 75

var crlf = []byte("\r\n")

// foldLines folds an encoded card so no content line exceeds foldLimit octets.
// A continuation line starts with one space, which the reader strips again.
func foldLines(src []byte) []byte {
	lines := bytes.Split(src, crlf)
	var out bytes.Buffer
	out.Grow(len(src) + len(src)/foldLimit*3)
	for i, line := range lines {
		if i == len(lines)-1 && len(line) == 0 {
			break
		}
		writeFolded(&out, line)
		out.Write(crlf)
	}
	return out.Bytes()
}

func writeFolded(out *bytes.Buffer, line []byte) {
	limit := foldLimit
	for len(line) > limit {
		cut := runeBoundary(line, limit)
		if cut == 0 {
			break
		}
		out.Write(line[:cut])
		out.Write(crlf)
		out.WriteByte(' ')
		line = line[cut:]
		// The leading space of a continuation line counts towards the limit.
		limit = foldLimit - 1
	}
	out.Write(line)
}

// runeBoundary returns the largest cut at or below limit that does not split a
// UTF-8 sequence.
func runeBoundary(line []byte, limit int) int {
	if limit >= len(line) {
		return len(line)
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(line[cut]) {
		cut--
	}
	return cut
}
