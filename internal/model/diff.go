// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/emersion/go-vcard"
)

// DiffKind classifies one property difference between two objects (§9).
type DiffKind string

const (
	DiffOnlyLocal  DiffKind = "only_local"
	DiffOnlyRemote DiffKind = "only_remote"
	DiffChanged    DiffKind = "changed"
)

// DiffLine is one property that differs between the refused edit and the
// server's current version.
type DiffLine struct {
	Name   string
	Kind   DiffKind
	Local  string
	Remote string
}

// Diff compares a refused edit with the server's version for the conflict
// screen (§9). Volatile properties a server rewrites itself are ignored.
func Diff(local, remote *Object) ([]DiffLine, error) {
	if local == nil && remote == nil {
		return nil, fmt.Errorf("model: cannot diff missing objects")
	}
	names := make(map[string]struct{})
	if local != nil {
		for _, name := range local.Names() {
			if !volatileProperties[name] {
				names[name] = struct{}{}
			}
		}
	}
	if remote != nil {
		for _, name := range remote.Names() {
			if !volatileProperties[name] {
				names[name] = struct{}{}
			}
		}
	}
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)

	out := make([]DiffLine, 0)
	for _, name := range ordered {
		var localVals, remoteVals []Value
		if local != nil {
			localVals = local.Property(name)
		}
		if remote != nil {
			remoteVals = remote.Property(name)
		}
		localSig := signatures(localVals)
		remoteSig := signatures(remoteVals)
		if sameSignatures(localSig, remoteSig) {
			continue
		}
		line := DiffLine{
			Name:   name,
			Local:  displayValues(name, localVals),
			Remote: displayValues(name, remoteVals),
		}
		switch {
		case len(localVals) == 0:
			line.Kind = DiffOnlyRemote
		case len(remoteVals) == 0:
			line.Kind = DiffOnlyLocal
		default:
			line.Kind = DiffChanged
		}
		out = append(out, line)
	}
	return out, nil
}

func displayValues(name string, values []Value) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for _, v := range values {
		parts = append(parts, displayValue(name, v))
	}
	return strings.Join(parts, " · ")
}

func displayValue(name string, v Value) string {
	text := strings.TrimSpace(v.Text)
	if name == vcard.FieldPhoto {
		return truncatePhoto(text)
	}
	if len(text) > 200 {
		return text[:197] + "…"
	}
	return text
}

func truncatePhoto(text string) string {
	if text == "" {
		return "(empty)"
	}
	if strings.HasPrefix(text, "http://") || strings.HasPrefix(text, "https://") {
		return text
	}
	if strings.HasPrefix(text, "data:") {
		if i := strings.Index(text, ","); i > 0 && i < 40 {
			return text[:i] + ",…(" + fmt.Sprintf("%d bytes", len(text)-i-1) + ")"
		}
		return "data:…(" + fmt.Sprintf("%d bytes", len(text)) + ")"
	}
	return "inline…(" + fmt.Sprintf("%d bytes", len(text)) + ")"
}
