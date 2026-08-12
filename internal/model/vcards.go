// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/emersion/go-vcard"
)

// ParsedCard is one vCard taken from an import stream together with where it
// came from, so the preview can name a bad file without losing the rest.
type ParsedCard struct {
	Object *Object
	Source string // file name or "upload" for a lone .vcf
	Error  string // set when this card could not be parsed
}

// ParseVCards reads every address object from a body that may hold more than
// one card. A bad card is reported and skipped; the rest are still returned.
func ParseVCards(body []byte) []ParsedCard {
	chunks := SplitVCards(body)
	if len(chunks) == 0 {
		return nil
	}
	out := make([]ParsedCard, 0, len(chunks))
	for i, chunk := range chunks {
		obj, err := ParseVCard(fmt.Sprintf("card-%d", i+1), "", chunk)
		if err != nil {
			out = append(out, ParsedCard{Error: err.Error()})
			continue
		}
		out = append(out, ParsedCard{Object: obj})
	}
	return out
}

// SplitVCards cuts a body into individual BEGIN:VCARD…END:VCARD blocks.
// Folded lines and mixed newlines are preserved inside each block.
func SplitVCards(body []byte) [][]byte {
	lines := splitKeepNL(body)
	var out [][]byte
	var cur [][]byte
	in := false
	for _, line := range lines {
		upper := bytes.ToUpper(bytes.TrimSpace(line))
		if bytes.HasPrefix(upper, []byte("BEGIN:VCARD")) {
			if in && len(cur) > 0 {
				out = append(out, bytes.Join(cur, nil))
			}
			cur = [][]byte{line}
			in = true
			continue
		}
		if !in {
			continue
		}
		cur = append(cur, line)
		if bytes.HasPrefix(upper, []byte("END:VCARD")) {
			out = append(out, bytes.Join(cur, nil))
			cur = nil
			in = false
		}
	}
	if in && len(cur) > 0 {
		out = append(out, bytes.Join(cur, nil))
	}
	return out
}

func splitKeepNL(body []byte) [][]byte {
	if len(body) == 0 {
		return nil
	}
	var lines [][]byte
	start := 0
	for i := 0; i < len(body); i++ {
		if body[i] == '\n' {
			lines = append(lines, body[start:i+1])
			start = i + 1
		}
	}
	if start < len(body) {
		lines = append(lines, body[start:])
	}
	return lines
}

// AssignUID replaces the object's identity. Patch refuses UID because an edit
// that quietly changes it creates a second contact; import is the one path that
// needs a new identity when the original already exists in the collection.
func (o *Object) AssignUID(uid string) error {
	if o == nil || o.raw == nil {
		return errors.New("model: object has no payload")
	}
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return errors.New("model: UID is required")
	}
	o.raw.SetValue(vcard.FieldUID, uid)
	return nil
}

// EnsureUID assigns a fresh identity when the card has none.
func (o *Object) EnsureUID() (string, error) {
	if o == nil {
		return "", errors.New("model: object is nil")
	}
	if id := strings.TrimSpace(o.UID()); id != "" {
		return id, nil
	}
	id, err := NewUID()
	if err != nil {
		return "", err
	}
	if err := o.AssignUID(id); err != nil {
		return "", err
	}
	return id, nil
}
