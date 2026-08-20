// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package account

import "strings"

// View names the screen a selection of sources belongs to. §14 keeps the sets
// independent per type: the calendars an agenda merges are not the address books
// a contact list merges.
const (
	ViewAgenda   = "agenda"
	ViewContacts = "contacts"
	ViewTasks    = "tasks"
	ViewNotes    = "notes"
	ViewSearch   = "search"
	// ViewDuplicates is the screen of §15. It keeps its own selection because
	// looking for duplicates spans both kinds of collection and is not the
	// same question as the one a search screen was last asked.
	ViewDuplicates = "duplicates"
	// ViewFiles is the file section of §7. Its default is the collection the
	// browser opens on.
	ViewFiles = "files"
	// ViewAttachments is not a screen: it is where §23.10 puts the files a note
	// or an event is given. It is kept as a default rather than a selection
	// because the whole requirement is that the folder is named once and never
	// asked for again — the Collection of the reference is the folder itself,
	// which is why it can be a path deeper than a collection root.
	ViewAttachments = "attachments"
)

// SourceRef points at one collection of one account. It is a reference and not
// a copy: a collection that has since been removed simply stops matching, which
// is what §15 asks for when an identifier goes stale.
type SourceRef struct {
	AccountID  string `json:"account_id"`
	Collection string `json:"collection"`
}

// Key is the form used to compare and to name a checkbox.
func (r SourceRef) Key() string { return r.AccountID + "|" + normalizePath(r.Collection) }

// Views are the interface choices that outlive a session. §14 puts the source
// selection in the settings rather than the session precisely so it survives a
// restart, and §23.9 wants the collection a note goes into remembered rather
// than asked for every time.
type Views struct {
	// Selected maps a view to its chosen sources. A view absent from the map
	// has never been chosen and means "all of them"; a view present with an
	// empty list means the person unticked everything, which is a different
	// thing and has to stay that way across a restart (§21).
	Selected map[string][]SourceRef `json:"selected,omitempty"`
	// Defaults maps a view to the collection new records go into.
	Defaults map[string]SourceRef `json:"defaults,omitempty"`
}

// Selection returns the chosen sources of a view and whether a choice was ever
// made.
func (v Views) Selection(view string) ([]SourceRef, bool) {
	if v.Selected == nil {
		return nil, false
	}
	refs, ok := v.Selected[view]
	if !ok {
		return nil, false
	}
	return append([]SourceRef(nil), refs...), true
}

// Select records the chosen sources of a view. An empty list is stored as such.
func (v *Views) Select(view string, refs []SourceRef) {
	if v.Selected == nil {
		v.Selected = make(map[string][]SourceRef)
	}
	normalised := make([]SourceRef, 0, len(refs))
	seen := make(map[string]bool, len(refs))
	for _, ref := range refs {
		ref.Collection = normalizePath(ref.Collection)
		if ref.AccountID == "" || ref.Collection == "" || seen[ref.Key()] {
			continue
		}
		seen[ref.Key()] = true
		normalised = append(normalised, ref)
	}
	v.Selected[view] = normalised
}

// Default returns the collection new records of a view go into.
func (v Views) Default(view string) (SourceRef, bool) {
	if v.Defaults == nil {
		return SourceRef{}, false
	}
	ref, ok := v.Defaults[view]
	return ref, ok && ref.AccountID != "" && ref.Collection != ""
}

// SetDefault remembers the collection last used by a view (§23.9).
func (v *Views) SetDefault(view string, ref SourceRef) {
	ref.Collection = normalizePath(ref.Collection)
	if ref.AccountID == "" || ref.Collection == "" {
		return
	}
	if v.Defaults == nil {
		v.Defaults = make(map[string]SourceRef)
	}
	v.Defaults[view] = ref
}

// Clone returns a deep copy, so a caller cannot reach into stored state.
func (v Views) Clone() Views {
	out := Views{}
	if v.Selected != nil {
		out.Selected = make(map[string][]SourceRef, len(v.Selected))
		for view, refs := range v.Selected {
			out.Selected[view] = append([]SourceRef(nil), refs...)
		}
	}
	if v.Defaults != nil {
		out.Defaults = make(map[string]SourceRef, len(v.Defaults))
		for view, ref := range v.Defaults {
			out.Defaults[view] = ref
		}
	}
	return out
}

// PurgeCollection removes every reference to one collection from saved views.
func (v *Views) PurgeCollection(ref SourceRef) {
	if v == nil {
		return
	}
	ref.Collection = normalizePath(ref.Collection)
	for view, refs := range v.Selected {
		out := refs[:0]
		for _, r := range refs {
			if r.AccountID == ref.AccountID && normalizePath(r.Collection) == ref.Collection {
				continue
			}
			out = append(out, r)
		}
		if len(out) != len(refs) {
			v.Selected[view] = out
		}
	}
	for view, def := range v.Defaults {
		if def.AccountID == ref.AccountID && normalizePath(def.Collection) == ref.Collection {
			delete(v.Defaults, view)
		}
	}
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return path
}
