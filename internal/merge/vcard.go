// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package merge

import (
	"errors"
	"sort"
	"strings"

	"gitea.mixdep.ru/mix/carrel/internal/model"
)

// repeatableProperties are the vCard properties a merge unions rather than
// choosing between. §15 is explicit that telephone numbers, addresses and mail
// addresses are joined with the exact duplicates removed — replacing them is how
// merging two cards loses half of somebody's contact details.
var repeatableProperties = map[string]bool{
	"TEL":     true,
	"EMAIL":   true,
	"ADR":     true,
	"URL":     true,
	"IMPP":    true,
	"RELATED": true,
	"MEMBER":  true,
	"LANG":    true,
}

// serverProperties are the ones the server maintains itself. Carrying another
// card's revision or product identifier into the merged card would claim
// something untrue about it.
var serverProperties = map[string]bool{
	"VERSION": true,
	"UID":     true,
	"PRODID":  true,
	"REV":     true,
}

// MergedPatch is the write that combines a group into one card (§15).
//
// The target's own raw card is the base and is never overwritten: what the other
// records carry and it does not is added, a repeatable property becomes the union
// with the exact duplicates removed, and a property they disagree about stays as
// the target has it — including the X- properties of every participant, which are
// kept precisely because this build does not know what they mean.
//
// The patch only ever adds. That matters for the destructive step it belongs to:
// if the PUT fails, nothing was removed from anywhere, and §15 requires the
// sources to survive a failed write.
func MergedPatch(target *model.Object, others []*model.Object) (*model.Patch, []string, error) {
	if target == nil || target.Kind() != model.KindVCard {
		return nil, nil, errors.New("merge: the target of a merge must be an address object")
	}
	patch := &model.Patch{}
	var added []string

	names := make([]string, 0, 16)
	seen := make(map[string]bool, 16)
	for _, other := range others {
		if other == nil || other.Kind() != model.KindVCard {
			return nil, nil, errors.New("merge: only address objects can be merged")
		}
		for _, name := range other.Names() {
			if serverProperties[name] || seen[name] {
				continue
			}
			seen[name] = true
			names = append(names, name)
		}
	}
	sort.Strings(names)

	for _, name := range names {
		existing := target.Property(name)
		switch {
		case len(existing) == 0:
			// Nothing to disagree with: take what the others have, deduplicated
			// between them.
			values := dedupeValues(name, nil, collect(others, name))
			if len(values) == 0 {
				continue
			}
			patch.Set(name, values...)
			added = append(added, name)
		case repeatableProperties[name]:
			values := dedupeValues(name, existing, collect(others, name))
			if len(values) == len(existing) {
				// Everything the others carry is already on the target.
				continue
			}
			patch.Set(name, values...)
			added = append(added, name)
		default:
			// The target wins, which is what §15 says about a conflict.
		}
	}
	return patch, added, nil
}

func collect(objects []*model.Object, name string) []model.Value {
	var out []model.Value
	for _, obj := range objects {
		out = append(out, obj.Property(name)...)
	}
	return out
}

// dedupeValues returns the base values followed by the incoming ones it does not
// already carry. Comparison folds the text the way the property is scored: a
// number written as +7 495 123-45-67 in one card and 84951234567 in another is
// one number, and putting both on the merged card would be the opposite of a
// merge.
func dedupeValues(name string, base, incoming []model.Value) []model.Value {
	fold := valueKey(name)
	out := make([]model.Value, 0, len(base)+len(incoming))
	seen := make(map[string]bool, len(base)+len(incoming))
	for _, values := range [][]model.Value{base, incoming} {
		for _, v := range values {
			text := strings.TrimSpace(v.Text)
			if text == "" {
				continue
			}
			folded := fold(text)
			if folded == "" || seen[folded] {
				continue
			}
			seen[folded] = true
			out = append(out, v)
		}
	}
	return out
}

func valueKey(name string) func(string) string {
	switch name {
	case "TEL":
		return func(text string) string {
			if key := PhoneKey(text); key != "" {
				return key
			}
			return foldText(text)
		}
	case "EMAIL":
		return func(text string) string {
			if key := model.NormalizeEmail(text); key != "" {
				return key
			}
			return foldText(text)
		}
	default:
		return foldText
	}
}

func foldText(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(text), " "))
}
