// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"fmt"
	"strings"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/fanout"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

// sectionRail is the source list of §1.7: an «All …» row and ticked collections
// shared by the merged and single-collection views of one section.
type sectionRail struct {
	Section    string
	AllLabel   string
	AllURL     string
	AllActive  bool
	AccountID  string
	ColEnc     string
	Sources    []sourceRow
	SourcesURL string
	Mode       findMode
	Selection  string
	RailTitle  string
	// NewCollectionURL and NewCollectionLabel are the rail-foot entry (§10.1).
	NewCollectionURL   string
	NewCollectionLabel string
}

func selectionState(rows []sourceRow) string {
	if len(rows) == 0 {
		return "none"
	}
	selected := 0
	for _, row := range rows {
		if row.Selected {
			selected++
		}
	}
	switch {
	case selected == 0:
		return "none"
	case selected == len(rows):
		return "all"
	default:
		return "part"
	}
}

func (s *Server) buildSectionRail(sess *session.Session, req findRequest, activeAccount, activeColEnc string) (sectionRail, error) {
	rows, err := s.findSources(sess, req)
	if err != nil {
		return sectionRail{}, err
	}
	return sectionRail{
		Section:            req.Mode.section(),
		AllLabel:           req.Mode.allLabel(),
		AllURL:             s.Path(req.Mode.sectionHome()),
		AllActive:          activeAccount == "",
		AccountID:          activeAccount,
		ColEnc:             activeColEnc,
		Sources:            rows,
		SourcesURL:         s.sourcesURL(req.Mode),
		Mode:               req.Mode,
		Selection:          selectionState(rows),
		NewCollectionURL:   s.sectionNewCollectionURL(req.Mode),
		NewCollectionLabel: sectionNewCollectionLabel(req.Mode),
	}, nil
}

// emptyHomeRail is the rail for a section that has no collections of that
// kind yet. The empty-home path used to skip buildSectionRail, which hid the
// New calendar / address book / list / notebook button §10.1 puts on an
// empty rail — the clean-Baikal case the wave was written for.
func (s *Server) emptyHomeRail(sess *session.Session, mode findMode) sectionRail {
	rail, err := s.buildSectionRail(sess, findRequest{Mode: mode}, "", "")
	if err != nil {
		return sectionRail{}
	}
	return rail
}

type sourceGroup struct {
	Label   string
	Sources []sourceRow
}

// AccountGroups keeps collections under the account they belong to, in the
// order the accounts first appear. The mockup's rail is a list of accounts,
// not a flat pile of collections.
func (r sectionRail) AccountGroups() []sourceGroup {
	var groups []sourceGroup
	index := make(map[string]int)
	for _, src := range r.Sources {
		key := src.AccountID
		if at, ok := index[key]; ok {
			groups[at].Sources = append(groups[at].Sources, src)
			continue
		}
		label := src.AccountLabel
		if label == "" {
			label = src.AccountID
		}
		index[key] = len(groups)
		groups = append(groups, sourceGroup{Label: label, Sources: []sourceRow{src}})
	}
	return groups
}

func (r sectionRail) SourceSummary() string {
	if len(r.Sources) == 0 {
		return ""
	}
	selected := 0
	accounts := make(map[string]struct{})
	for _, src := range r.Sources {
		accounts[src.AccountID] = struct{}{}
		if src.Selected {
			selected++
		}
	}
	one, many := r.Mode.collectionNoun()
	noun := many
	if len(r.Sources) == 1 {
		noun = one
	}
	acc := "account"
	if len(accounts) != 1 {
		acc = "accounts"
	}
	if selected == len(r.Sources) {
		return fmt.Sprintf("%d %s · %d %s", len(r.Sources), noun, len(accounts), acc)
	}
	return fmt.Sprintf("%d of %d %s · %d %s", selected, len(r.Sources), many, len(accounts), acc)
}

// sourceRow is one collection in a sidebar, with the tick §14 puts on it.
type sourceRow struct {
	AccountID    string
	AccountLabel string
	ColEnc       string
	Path         string
	Name         string
	Color        string
	Kind         discovery.Kind
	ReadOnly     bool
	Selected     bool
}

// Key matches account.SourceRef.Key, so a form field names a collection the
// same way the stored selection does.
func (r sourceRow) Key() string { return r.AccountID + "|" + normalizeCollectionPath(r.Path) }

// Label is what the sources panel prints: the collection, then the account, so
// two books of the same name in different accounts are told apart (§14).
func (r sourceRow) Label() string {
	if r.Name != "" {
		return r.Name
	}
	return r.Path
}

// collectionsOfKind lists every collection of one kind across the user's
// enabled accounts, marked against the saved selection of a view.
//
// A view with no saved selection means "all of them": on the first visit the
// screen shows something rather than nothing. Once a choice is made, an empty
// choice is honoured as an empty choice (§21).
func (s *Server) collectionsOfKind(sess *session.Session, kind discovery.Kind, view, component string) ([]sourceRow, error) {
	accounts, err := s.Store.ListDAVAccounts(sess.UserID, sess.DEK())
	if err != nil {
		return nil, err
	}
	views, err := s.Store.Views(sess.UserID, sess.DEK())
	if err != nil {
		return nil, err
	}
	chosen, hasChoice := views.Selection(view)
	picked := make(map[string]bool, len(chosen))
	for _, ref := range chosen {
		picked[ref.Key()] = true
	}
	var out []sourceRow
	for _, acc := range accounts {
		if !acc.Enabled {
			continue
		}
		for _, col := range acc.Collections {
			if col.Kind != kind || !supportsComponent(col, component) {
				continue
			}
			row := sourceRow{
				AccountID: acc.ID, AccountLabel: accountLabel(acc),
				ColEnc: EncodeCollectionPath(col.Path), Path: col.Path,
				Name: col.DisplayName, Color: col.Color, Kind: col.Kind,
				ReadOnly: col.ReadOnly,
			}
			row.Selected = !hasChoice || picked[row.Key()]
			out = append(out, row)
		}
	}
	return out, nil
}

// supportsComponent reports whether a calendar accepts a component kind. A
// server that does not advertise the set at all is taken at face value and
// offered: refusing to show a collection because it kept quiet would hide
// working task lists (§17).
func supportsComponent(col discovery.Collection, component string) bool {
	if component == "" || len(col.SupportedComponents) == 0 {
		return true
	}
	for _, name := range col.SupportedComponents {
		if strings.EqualFold(name, component) {
			return true
		}
	}
	return false
}

// selectedRows keeps only the ticked rows.
func selectedRows(rows []sourceRow) []sourceRow {
	out := make([]sourceRow, 0, len(rows))
	for _, row := range rows {
		if row.Selected {
			out = append(out, row)
		}
	}
	return out
}

// fanoutSources turns sidebar rows into the sources of a poll. The kind travels
// with each one so a single task can mix calendars and address books, which is
// what a cross-source search does (§16).
func fanoutSources(rows []sourceRow) []fanout.Source {
	out := make([]fanout.Source, 0, len(rows))
	for _, row := range rows {
		out = append(out, fanout.Source{
			ID:              row.Key(),
			Kind:            string(row.Kind),
			AccountID:       row.AccountID,
			AccountLabel:    row.AccountLabel,
			Collection:      normalizeCollectionPath(row.Path),
			CollectionLabel: row.Label(),
			Color:           row.Color,
			ReadOnly:        row.ReadOnly,
		})
	}
	return out
}

// saveSelection records the ticked collections of a view. The keys arrive from
// the form as account|collection pairs and are checked against the collections
// the user actually has, so a submitted field cannot add a source that was
// never discovered.
func (s *Server) saveSelection(sess *session.Session, view string, keys []string, known []sourceRow) error {
	wanted := make(map[string]bool, len(keys))
	for _, key := range keys {
		wanted[key] = true
	}
	refs := make([]account.SourceRef, 0, len(known))
	for _, row := range known {
		if wanted[row.Key()] {
			refs = append(refs, account.SourceRef{AccountID: row.AccountID, Collection: row.Path})
		}
	}
	return s.Store.UpdateViews(sess.UserID, sess.DEK(), func(v *account.Views) { v.Select(view, refs) })
}

// defaultCollection returns the collection a view creates records in: the one
// last used, or the first writable one. §23.9 is explicit that this is not asked
// for every time — a note that costs a form is a note nobody writes.
func (s *Server) defaultCollection(sess *session.Session, view string, rows []sourceRow) (sourceRow, bool) {
	views, err := s.Store.Views(sess.UserID, sess.DEK())
	if err == nil {
		if ref, ok := views.Default(view); ok {
			for _, row := range rows {
				if row.Key() == ref.Key() && !row.ReadOnly {
					return row, true
				}
			}
		}
	}
	for _, row := range rows {
		if !row.ReadOnly {
			return row, true
		}
	}
	return sourceRow{}, false
}

// rememberDefault records the collection a record was just filed in.
func (s *Server) rememberDefault(sess *session.Session, view, accountID, collection string) {
	err := s.Store.UpdateViews(sess.UserID, sess.DEK(), func(v *account.Views) {
		v.SetDefault(view, account.SourceRef{AccountID: accountID, Collection: collection})
	})
	if err != nil {
		s.logError("remember default collection", err)
	}
}
