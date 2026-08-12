// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/merge"
	"gitea.mixdep.ru/mix/carrel/internal/model"
	"gitea.mixdep.ru/mix/carrel/internal/provider/contacts"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

// dupMergeView is the confirmation of §15 and, afterwards, the report of what
// was actually done.
type dupMergeView struct {
	GroupID string
	Kind    string
	Title   string
	// Target is the card the merged one is written to; Sources are the ones
	// that will be deleted after it has been written.
	Target  dupMergeParty
	Sources []dupMergeParty
	// Adds names the properties the target does not carry yet and will gain.
	Adds []string
	// Tokens carry the group through the confirmation to the write, so the
	// screen after the button is built from the same records as the one before.
	Tokens []string
	// Done marks the report rather than the confirmation.
	Done bool
	// Written reports that the merged card reached the target.
	Written bool
	// Deleted and Kept are what happened to the sources: §15 requires both to
	// be named when a delete fails part way, and forbids an automatic rollback.
	Deleted  []dupMergeParty
	Kept     []dupMergeParty
	Failure  string
	Back     string
	MergeURL string
}

// dupMergeParty is one card in a merge, named the way a person can check it.
type dupMergeParty struct {
	Token           string
	AccountLabel    string
	CollectionLabel string
	Title           string
	Path            string
	ReadOnly        bool
}

// DuplicateMerge merges a group on the server, in the order §15 fixes: the
// merged card is written first, and the sources are deleted only after that write
// has been confirmed. There is no automatic rollback, and a delete that fails
// stops the rest — what has gone and what has not is said plainly.
func (s *Server) DuplicateMerge(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sess := SessionFrom(r)
	back := SafeRedirect(r.PostFormValue("back"), s.Path("/app/duplicates"))
	confirmed := strings.TrimSpace(r.PostFormValue("action")) == "apply"

	plan, err := s.planMerge(r.Context(), sess, r.PostForm["member"], strings.TrimSpace(r.PostFormValue("target")))
	if err != nil {
		s.duplicateFailed(w, r, back, err)
		return
	}
	plan.view.GroupID = strings.TrimSpace(r.PostFormValue("group"))
	plan.view.Back = back
	plan.view.MergeURL = s.Path("/app/duplicates/merge")

	if !confirmed {
		v := s.View(r, "Merge duplicates")
		v.Data = plan.view
		s.Render(w, "duplicate_merge.html", v)
		return
	}

	report := s.applyMerge(r.Context(), sess, plan)
	v := s.View(r, "Merge duplicates")
	if report.Failure != "" {
		v.Error = report.Failure
	} else {
		v.Notice = "The records were merged into one."
	}
	v.Data = report
	s.Render(w, "duplicate_merge.html", v)
}

// mergePlan is a merge about to happen: the objects as the server has them now,
// the patch that combines them, and the confirmation to show for it.
type mergePlan struct {
	target        dupRef
	targetObject  *model.Object
	sources       []dupRef
	sourceObjects []*model.Object
	patch         *model.Patch
	view          dupMergeView
}

// planMerge reads every participant again before anything is written.
//
// The versions a merge is conditional on have to be the current ones: the ETag a
// screen was rendered with is old by definition, and §15 wants both the write and
// every delete to carry a precondition that means something.
func (s *Server) planMerge(ctx context.Context, sess *session.Session, tokens []string, targetToken string) (*mergePlan, error) {
	refs, err := s.validateDupMembers(sess, tokens)
	if err != nil {
		return nil, err
	}
	target, err := decodeDupRef(targetToken)
	if err != nil {
		return nil, errors.New("choose which collection to merge into")
	}
	plan := &mergePlan{}
	for _, ref := range refs {
		if ref.member().Key() == target.member().Key() {
			plan.target = ref
			continue
		}
		plan.sources = append(plan.sources, ref)
	}
	if plan.target.AccountID == "" {
		return nil, errors.New("the collection to merge into is not one of the records")
	}
	if len(plan.sources) == 0 {
		return nil, errors.New("a merge needs more than one record")
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	targetObject, targetCol, err := s.readMergeCard(ctx, sess, plan.target)
	if err != nil {
		return nil, err
	}
	if targetCol.ReadOnly {
		// §15: in a collection without write access the option is unavailable,
		// and saying so is better than a failed PUT.
		return nil, fmt.Errorf("%s cannot be written to", collectionLabel(targetCol))
	}
	plan.targetObject = targetObject
	plan.view = dupMergeView{
		Kind:   account.KindContact,
		Tokens: tokens,
		Target: mergeParty(plan.target, targetObject, targetCol.ReadOnly, collectionLabel(targetCol), s.accountLabelOf(sess, plan.target.AccountID)),
	}

	for _, ref := range plan.sources {
		object, col, readErr := s.readMergeCard(ctx, sess, ref)
		if readErr != nil {
			return nil, readErr
		}
		if col.ReadOnly {
			return nil, fmt.Errorf("%s cannot be written to, so nothing can be deleted from it", collectionLabel(col))
		}
		plan.sourceObjects = append(plan.sourceObjects, object)
		plan.view.Sources = append(plan.view.Sources,
			mergeParty(ref, object, col.ReadOnly, collectionLabel(col), s.accountLabelOf(sess, ref.AccountID)))
	}

	patch, added, err := merge.MergedPatch(plan.targetObject, plan.sourceObjects)
	if err != nil {
		return nil, err
	}
	plan.patch = patch
	plan.view.Adds = added
	plan.view.Title = plan.view.Target.Title
	return plan, nil
}

// applyMerge carries out the plan. The order is the whole of the safety: a source
// is only deleted after the merged card is known to be stored (§15).
func (s *Server) applyMerge(ctx context.Context, sess *session.Session, plan *mergePlan) dupMergeView {
	view := plan.view
	view.Done = true
	view.Kept = append([]dupMergeParty(nil), plan.view.Sources...)

	p, _, err := s.contactsProvider(sess, plan.target.AccountID)
	if err != nil {
		view.Failure = userFacingDAVError(err)
		return view
	}

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	merged := plan.targetObject.Clone()
	if applyErr := merged.Apply(plan.patch); applyErr != nil {
		view.Failure = capitalize(applyErr.Error())
		return view
	}
	if _, writeErr := p.Update(ctx, plan.target.Collection, merged); writeErr != nil {
		// Nothing has been deleted: the sources are all still there, which is
		// exactly what §21 asks to be true after a failed PUT.
		view.Failure = "The merged record could not be written, so nothing was deleted: " + userFacingDAVError(writeErr)
		return view
	}
	view.Written = true

	view.Kept = nil
	for i, ref := range plan.sources {
		party := plan.view.Sources[i]
		provider, _, providerErr := s.contactsProvider(sess, ref.AccountID)
		if providerErr != nil {
			view.Failure = deleteFailure(party, userFacingDAVError(providerErr))
			view.Kept = append(view.Kept, plan.view.Sources[i:]...)
			break
		}
		object := plan.sourceObjects[i]
		if delErr := provider.Delete(ctx, ref.Collection, object.Path, object.ETag); delErr != nil {
			// §15: stop, say what has gone and what has not, and do not try to
			// undo the delete that worked.
			view.Failure = deleteFailure(party, mergeDeleteReason(delErr))
			view.Kept = append(view.Kept, plan.view.Sources[i:]...)
			break
		}
		view.Deleted = append(view.Deleted, party)
	}

	if view.Failure == "" && view.GroupID != "" {
		// The records are one object now, so the decision about them has
		// nothing left to describe.
		if err := s.Store.UpdateDuplicates(sess.UserID, sess.DEK(), func(stored *account.Duplicates) error {
			stored.Remove(view.GroupID)
			return nil
		}); err != nil {
			s.logError("drop merged duplicate group", err)
		}
	}
	return view
}

func deleteFailure(party dupMergeParty, reason string) string {
	return "The merged record was written, but " + party.CollectionLabel + " could not be cleaned up: " + reason +
		". Nothing was rolled back; the records still listed below are untouched."
}

func mergeDeleteReason(err error) string {
	if contacts.IsConflict(err) {
		return "it changed on the server since it was read"
	}
	return userFacingDAVError(err)
}

// readMergeCard reads one participant at its current version.
func (s *Server) readMergeCard(ctx context.Context, sess *session.Session, ref dupRef) (*model.Object, discovery.Collection, error) {
	p, acc, err := s.contactsProvider(sess, ref.AccountID)
	if err != nil {
		return nil, discovery.Collection{}, err
	}
	col, err := findAddressBook(acc, ref.Collection)
	if err != nil {
		return nil, discovery.Collection{}, err
	}
	path := ref.Path
	if path == "" {
		path = objectPathForUID(ref.Collection, ref.UID)
	}
	object, err := p.Get(ctx, ref.Collection, path)
	if err != nil {
		return nil, col, err
	}
	if object.ETag == "" {
		return nil, col, fmt.Errorf("%s does not report a version, so it cannot be merged safely", path)
	}
	return object, col, nil
}

func (s *Server) accountLabelOf(sess *session.Session, accountID string) string {
	acc, err := s.Store.GetDAVAccount(sess.UserID, accountID, sess.DEK())
	if err != nil {
		return accountID
	}
	return accountLabel(*acc)
}

func mergeParty(ref dupRef, object *model.Object, readOnly bool, collection, accountName string) dupMergeParty {
	title := ref.UID
	if contact, err := object.Contact(); err == nil {
		title = displayOr(contact.DisplayName(), ref.UID)
	}
	return dupMergeParty{
		Token: encodeDupRef(ref), AccountLabel: accountName, CollectionLabel: collection,
		Title: title, Path: object.Path, ReadOnly: readOnly,
	}
}
