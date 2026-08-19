// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/fanout"
	"gitea.mixdep.ru/mix/carrel/internal/merge"
	"gitea.mixdep.ru/mix/carrel/internal/model"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

// The three states a group can be in on the screen. Only two of them are stored:
// a candidate is what detection offers, and it exists until somebody decides
// about it (§15).
const (
	dupCandidate = "candidate"
	dupLinked    = "linked"
	dupIgnored   = "ignored"
)

// duplicateEventPast and duplicateEventFuture bound the events a duplicate poll
// loads. Detection has to work on records that are loaded, and loading every
// event a calendar has ever held to compare them is a cost nobody asked for; a
// window around today is where a double invitation is worth finding.
const (
	duplicateEventPast   = 90
	duplicateEventFuture = 365
)

func duplicateEventRange(loc *time.Location) (time.Time, time.Time) {
	today := time.Now().In(loc)
	return today.AddDate(0, 0, -duplicateEventPast), today.AddDate(0, 0, duplicateEventFuture)
}

// dupDisplay is the detection setting a merged list collapses with, together with
// where its badges lead.
func (s *Server) dupDisplay() dupDisplay {
	return dupDisplay{Threshold: s.duplicateThreshold(), URL: s.Path("/app/duplicates")}
}

func (s *Server) duplicateThreshold() int {
	if s.Detection.Threshold > 0 {
		return s.Detection.Threshold
	}
	return merge.DefaultThreshold
}

// Duplicates is the screen of §15: every ticked address book and calendar is
// polled, the records that arrive are scored against each other, and the groups
// are offered with the three decisions. Nothing happens on its own.
func (s *Server) Duplicates(w http.ResponseWriter, r *http.Request) {
	s.startFind(w, r, findRequest{Mode: modeDuplicates}, "duplicates.html")
}

// dupRecordItem is what a duplicate poll puts into the fan-out: the record
// itself, because scoring needs the card, plus what it takes to print it.
type dupRecordItem struct {
	Record   merge.Record
	Kind     string
	Title    string
	Subtitle string
	URL      string
}

// pollRecords loads one source in full. It is the only poll that carries whole
// objects rather than finished rows: §15 scores on the cards, and the point of
// doing it on loaded records is that no second request is made for it.
func (s *Server) pollRecords(ctx context.Context, sess *session.Session, src fanout.Source, from, to time.Time, loc *time.Location) ([]fanout.Item, bool, error) {
	if src.Kind == string(discovery.KindAddressBook) {
		return s.pollBookRecords(ctx, sess, src)
	}
	return s.pollCalendarRecords(ctx, sess, src, from, to, loc)
}

func (s *Server) pollBookRecords(ctx context.Context, sess *session.Session, src fanout.Source) ([]fanout.Item, bool, error) {
	p, _, err := s.contactsProvider(sess, src.AccountID)
	if err != nil {
		return nil, false, err
	}
	listing, err := p.List(ctx, src.Collection)
	if err != nil {
		return nil, false, err
	}
	result, err := p.Multiget(ctx, src.Collection, listing.Paths(), listing.ETags)
	if err != nil {
		return nil, false, err
	}
	items := make([]fanout.Item, 0, len(result.Objects))
	for _, obj := range result.Objects {
		contact, contactErr := obj.Contact()
		if contactErr != nil {
			continue
		}
		record := recordOf(src, obj)
		items = append(items, fanout.Item{SourceID: src.ID, Key: record.Key(), Data: dupRecordItem{
			Record: record, Kind: account.KindContact,
			Title:    displayOr(contact.DisplayName(), "(no name)"),
			Subtitle: strings.Join(contact.NormalizedEmails(), ", "),
			URL:      s.contactURL(src, contact.UID),
		}})
	}
	return items, false, nil
}

func (s *Server) pollCalendarRecords(ctx context.Context, sess *session.Session, src fanout.Source, from, to time.Time, loc *time.Location) ([]fanout.Item, bool, error) {
	p, _, err := s.calendarProvider(sess, src.AccountID)
	if err != nil {
		return nil, false, err
	}
	set, err := p.QueryComponent(ctx, src.Collection, dav.CompEvent, from, to)
	if err != nil {
		return nil, false, err
	}
	items := make([]fanout.Item, 0, len(set.Objects))
	anyCache := false
	for _, obj := range set.Objects {
		event, eventErr := obj.Event(loc)
		if eventErr != nil {
			continue
		}
		record := recordOf(src, obj)
		items = append(items, fanout.Item{SourceID: src.ID, Key: record.Key(), Data: dupRecordItem{
			Record: record, Kind: account.KindEvent,
			Title:    event.DisplayTitle(),
			Subtitle: eventWhen(event, loc),
			URL:      s.icalURL("calendar", src, event.UID),
		}})
	}
	anyCache = set.FromCache && len(items) > 0

	if todoSet, todoErr := p.QueryComponent(ctx, src.Collection, dav.CompTodo, time.Time{}, time.Time{}); todoErr == nil {
		for _, obj := range todoSet.Objects {
			todo, todoErr := obj.Todo(loc)
			if todoErr != nil {
				continue
			}
			record := recordOf(src, obj)
			items = append(items, fanout.Item{SourceID: src.ID, Key: record.Key(), Data: dupRecordItem{
				Record: record, Kind: account.KindTodo,
				Title:    todo.DisplayTitle(),
				Subtitle: taskStatusLabel(todo),
				URL:      s.icalURL("tasks", src, todo.UID),
			}})
		}
		if todoSet.FromCache && len(todoSet.Objects) > 0 {
			anyCache = true
		}
	}

	if noteSet, noteErr := p.QueryComponent(ctx, src.Collection, dav.CompJournal, time.Time{}, time.Time{}); noteErr == nil {
		for _, obj := range noteSet.Objects {
			note, noteErr := obj.Note(loc)
			if noteErr != nil {
				continue
			}
			record := recordOf(src, obj)
			items = append(items, fanout.Item{SourceID: src.ID, Key: record.Key(), Data: dupRecordItem{
				Record: record, Kind: account.KindNote,
				Title:    note.DisplayTitle(),
				Subtitle: noteDate(note, loc),
				URL:      s.icalURL("notes", src, note.UID),
			}})
		}
		if noteSet.FromCache && len(noteSet.Objects) > 0 {
			anyCache = true
		}
	}

	return items, anyCache, nil
}

func recordOf(src fanout.Source, obj *model.Object) merge.Record {
	return merge.Record{
		AccountID: src.AccountID, Collection: src.Collection,
		AccountLabel: src.AccountLabel, CollectionLabel: src.CollectionLabel,
		Color: src.Color, ReadOnly: src.ReadOnly, Object: obj,
	}
}

func noteDate(note model.Note, loc *time.Location) string {
	if note.Date.IsZero() {
		return "No date"
	}
	if note.DateOnly {
		return note.Date.In(loc).Format("2 January 2006")
	}
	return note.Date.In(loc).Format("2 January 2006, 15:04")
}

func eventWhen(event model.Event, loc *time.Location) string {
	if event.Start.IsZero() {
		return "No date"
	}
	if event.AllDay {
		return event.Start.In(loc).Format("2 January 2006") + " · all day"
	}
	return event.Start.In(loc).Format("2 January 2006, 15:04")
}

// duplicatesData is what the duplicates screen prints.
type duplicatesData struct {
	Contacts   []dupGroupView
	Events     []dupGroupView
	Tasks      []dupGroupView
	Notes      []dupGroupView
	Threshold  int
	Records    int
	Candidates int
	Linked     int
	Ignored    int
	DecideURL  string
	MergeURL   string
	Back       string
}

// Empty reports whether there is nothing to show, which is the normal outcome and
// not a failure.
func (d duplicatesData) Empty() bool {
	return len(d.Contacts) == 0 && len(d.Events) == 0 && len(d.Tasks) == 0 && len(d.Notes) == 0
}

// dupSection is one heading on the screen. The two kinds are never compared with
// each other, so they are never mixed in one list either.
type dupSection struct {
	Label  string
	Groups []dupGroupView
}

// Sections is the screen in the order it is read, with the empty kind left out.
func (d duplicatesData) Sections() []dupSection {
	var out []dupSection
	if len(d.Contacts) > 0 {
		out = append(out, dupSection{Label: "Contacts", Groups: d.Contacts})
	}
	if len(d.Events) > 0 {
		out = append(out, dupSection{Label: "Events", Groups: d.Events})
	}
	if len(d.Tasks) > 0 {
		out = append(out, dupSection{Label: "Tasks", Groups: d.Tasks})
	}
	if len(d.Notes) > 0 {
		out = append(out, dupSection{Label: "Notes", Groups: d.Notes})
	}
	return out
}

// dupGroupView is one group on the screen: what it is about, why it was
// proposed, and what can be done with it.
type dupGroupView struct {
	// ID is the stored group, empty for a group nobody has decided about.
	ID      string
	Kind    string
	State   string
	Title   string
	Score   int
	Signals []string
	Members []dupMemberView
	// Fields are the merged fields of §15 and Conflicts the subset the records
	// disagree about, which are the only ones worth asking about.
	Fields    []merge.MergedField
	Conflicts []merge.MergedField
	// Missing marks a group whose members are not all loaded: either their
	// collection is not ticked, or they have gone (§15).
	Missing bool
	// Mergeable reports that at least one member could be the target of a
	// merge on the server. In collections without write access the option is
	// shown as unavailable rather than hidden.
	Mergeable bool
}

// Linked reports whether the person has linked this group.
func (g dupGroupView) Linked() bool { return g.State == dupLinked }

// Candidate reports whether nothing has been decided about the group.
func (g dupGroupView) Candidate() bool { return g.State == dupCandidate }

// dupMemberView is one record inside a group.
type dupMemberView struct {
	Token           string
	AccountLabel    string
	CollectionLabel string
	Color           string
	Title           string
	Subtitle        string
	URL             string
	ReadOnly        bool
	// Present is false for a member the current poll did not load.
	Present bool
}

// duplicateData builds the groups of §15 from what the poll has loaded so far
// and what the person has already decided.
func (s *Server) duplicateData(sess *session.Session, snap fanout.Snapshot) duplicatesData {
	data := duplicatesData{
		Threshold: s.duplicateThreshold(),
		DecideURL: s.Path("/app/duplicates/decide"),
		MergeURL:  s.Path("/app/duplicates/merge"),
		Back:      s.Path("/app/duplicates"),
	}
	if sess == nil {
		return data
	}

	loaded := make(map[string]dupRecordItem, len(snap.Items))
	var contacts, events, todos, notes []merge.Record
	for _, item := range snap.Items {
		record, ok := item.Data.(dupRecordItem)
		if !ok {
			continue
		}
		loaded[record.Record.Key()] = record
		switch record.Kind {
		case account.KindContact:
			contacts = append(contacts, record.Record)
		case account.KindEvent:
			events = append(events, record.Record)
		case account.KindTodo:
			todos = append(todos, record.Record)
		case account.KindNote:
			notes = append(notes, record.Record)
		}
	}
	data.Records = len(loaded)

	decisions, err := s.Store.Duplicates(sess.UserID, sess.DEK())
	if err != nil {
		s.logError("read duplicate decisions", err)
		return data
	}
	// A member whose collection answered the poll and which is not in the
	// answer has gone: another client deleted or moved it. §15 asks for that to
	// be silent, and for a group left with one member to dissolve.
	if !snap.Running {
		if gone := s.pruneDecisions(sess, &decisions, snap, loaded); gone {
			if refreshed, refreshErr := s.Store.Duplicates(sess.UserID, sess.DEK()); refreshErr == nil {
				decisions = refreshed
			}
		}
	}

	decided := make(map[string]bool, len(decisions.Groups))
	for _, group := range decisions.Groups {
		decided[group.Signature()] = true
		view := s.decidedGroup(group, loaded)
		switch group.Verdict {
		case account.VerdictLinked:
			data.Linked++
		case account.VerdictIgnored:
			data.Ignored++
		}
		data.append(view)
	}

	for _, candidate := range merge.DetectContacts(contacts, s.duplicateOptions(decisions, contacts)) {
		if view, ok := s.candidateGroup(candidate, loaded, decided); ok {
			data.Candidates++
			data.append(view)
		}
	}
	for _, candidate := range merge.DetectEvents(events, s.timezone(), s.duplicateOptions(decisions, events)) {
		if view, ok := s.candidateGroup(candidate, loaded, decided); ok {
			data.Candidates++
			data.append(view)
		}
	}
	for _, candidate := range merge.DetectTodos(todos, s.timezone(), s.duplicateOptions(decisions, todos)) {
		if view, ok := s.candidateGroup(candidate, loaded, decided); ok {
			data.Candidates++
			data.append(view)
		}
	}
	for _, candidate := range merge.DetectNotes(notes, s.timezone(), s.duplicateOptions(decisions, notes)) {
		if view, ok := s.candidateGroup(candidate, loaded, decided); ok {
			data.Candidates++
			data.append(view)
		}
	}
	sortDupGroups(data.Contacts)
	sortDupGroups(data.Events)
	sortDupGroups(data.Tasks)
	sortDupGroups(data.Notes)
	return data
}

func (d *duplicatesData) append(view dupGroupView) {
	switch view.Kind {
	case account.KindEvent:
		d.Events = append(d.Events, view)
	case account.KindTodo:
		d.Tasks = append(d.Tasks, view)
	case account.KindNote:
		d.Notes = append(d.Notes, view)
	default:
		d.Contacts = append(d.Contacts, view)
	}
}

// duplicateOptions carries the stored verdicts into detection: a pair the person
// has decided against is never scored again, and a pair already linked is shown as
// the group it is rather than proposed a second time.
func (s *Server) duplicateOptions(decisions account.Duplicates, records []merge.Record) merge.Options {
	return merge.Options{
		Threshold: s.duplicateThreshold(),
		Skip: func(a, b int) bool {
			if a >= len(records) || b >= len(records) {
				return false
			}
			left, right := memberOf(records[a]), memberOf(records[b])
			if decisions.Ignored(left, right) {
				return true
			}
			leftGroup, leftOK := decisions.Find(left)
			rightGroup, rightOK := decisions.Find(right)
			return leftOK && rightOK && leftGroup.ID == rightGroup.ID
		},
	}
}

func (s *Server) decidedGroup(group account.Group, loaded map[string]dupRecordItem) dupGroupView {
	view := dupGroupView{ID: group.ID, Kind: group.Kind, State: string(group.Verdict)}
	var present []merge.Record
	for _, member := range group.Members {
		item, ok := loaded[member.Key()]
		if !ok {
			view.Missing = true
			view.Members = append(view.Members, dupMemberView{
				Token: encodeStoredMember(member), Title: member.UID,
				CollectionLabel: member.Collection, Subtitle: "not loaded",
			})
			continue
		}
		present = append(present, item.Record)
		view.Members = append(view.Members, memberView(item))
		if !item.Record.ReadOnly && group.Kind != account.KindTodo && group.Kind != account.KindNote {
			view.Mergeable = true
		}
	}
	view.fill(present, group.Fields)
	return view
}

func (s *Server) candidateGroup(candidate merge.Candidate, loaded map[string]dupRecordItem, decided map[string]bool) (dupGroupView, bool) {
	members := make([]account.Member, 0, len(candidate.Members))
	for _, record := range candidate.Members {
		members = append(members, memberOf(record))
	}
	if decided[(account.Group{Members: members}).Signature()] {
		return dupGroupView{}, false
	}
	view := dupGroupView{
		Kind: string(candidate.Kind), State: dupCandidate,
		Score: candidate.Score, Signals: candidate.Signals,
	}
	for _, record := range candidate.Members {
		item, ok := loaded[record.Key()]
		if !ok {
			continue
		}
		view.Members = append(view.Members, memberView(item))
		if !record.ReadOnly && candidate.Kind != merge.KindTodo && candidate.Kind != merge.KindNote {
			view.Mergeable = true
		}
	}
	view.fill(candidate.Members, nil)
	return view, len(view.Members) > 1
}

// fill merges the fields of the records that are loaded and names the group.
func (g *dupGroupView) fill(records []merge.Record, prefs map[string]string) {
	if g.Kind == account.KindContact && len(records) > 0 {
		merged := merge.MergeContacts(records, prefs)
		g.Fields = merged.Fields
		g.Conflicts = merged.Conflicts()
		g.Title = merged.Title
	}
	if g.Title == "" {
		for _, member := range g.Members {
			if member.Title != "" {
				g.Title = member.Title
				break
			}
		}
	}
	if g.Title == "" {
		g.Title = "(no title)"
	}
}

func memberView(item dupRecordItem) dupMemberView {
	return dupMemberView{
		Token: encodeRecordMember(item.Record), AccountLabel: item.Record.AccountLabel,
		CollectionLabel: item.Record.CollectionLabel, Color: item.Record.Color,
		Title: item.Title, Subtitle: item.Subtitle, URL: item.URL,
		ReadOnly: item.Record.ReadOnly, Present: true,
	}
}

func memberOf(record merge.Record) account.Member {
	return account.Member{
		AccountID: record.AccountID, Collection: record.Collection, UID: record.UID(),
	}
}

// sortDupGroups puts what needs deciding first: candidates, then the links in
// force, then the groups already rejected.
func sortDupGroups(groups []dupGroupView) {
	rank := map[string]int{dupCandidate: 0, dupLinked: 1, dupIgnored: 2}
	sort.SliceStable(groups, func(i, j int) bool {
		if rank[groups[i].State] != rank[groups[j].State] {
			return rank[groups[i].State] < rank[groups[j].State]
		}
		if groups[i].Score != groups[j].Score {
			return groups[i].Score > groups[j].Score
		}
		return strings.ToLower(groups[i].Title) < strings.ToLower(groups[j].Title)
	})
}

// pruneDecisions drops the members that are no longer where they were.
//
// Only a collection that answered this poll can say that something is missing
// from it: a source that failed, timed out or was never ticked says nothing, and
// treating its silence as a deletion would dissolve groups because one server was
// briefly down (§17).
func (s *Server) pruneDecisions(sess *session.Session, decisions *account.Duplicates, snap fanout.Snapshot, loaded map[string]dupRecordItem) bool {
	answered := make(map[string]bool, len(snap.Sources))
	for _, source := range snap.Sources {
		if source.State == fanout.StateDone || source.State == fanout.StateEmpty {
			answered[source.ID] = true
		}
	}
	if len(answered) == 0 {
		return false
	}
	gone := func(member account.Member) bool {
		if !answered[member.Source().Key()] {
			return false
		}
		_, ok := loaded[member.Key()]
		return !ok
	}
	dry := decisions.Clone()
	if !dry.Prune(gone) {
		return false
	}
	err := s.Store.UpdateDuplicates(sess.UserID, sess.DEK(), func(stored *account.Duplicates) error {
		stored.Prune(gone)
		return nil
	})
	if err != nil {
		s.logError("prune duplicate decisions", err)
		return false
	}
	return true
}

// DuplicateDecide records one of the two stored verdicts, or forgets a group.
//
// Nothing here touches a server: linking is a decision about how records are
// shown, and §15 is explicit that it changes nothing on any of them.
func (s *Server) DuplicateDecide(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sess := SessionFrom(r)
	back := SafeRedirect(r.PostFormValue("back"), s.Path("/app/duplicates"))
	action := strings.TrimSpace(r.PostFormValue("action"))

	var notice string
	switch action {
	case "forget":
		id := strings.TrimSpace(r.PostFormValue("group"))
		err := s.Store.UpdateDuplicates(sess.UserID, sess.DEK(), func(stored *account.Duplicates) error {
			if !stored.Remove(id) {
				return errors.New("that group is no longer there")
			}
			return nil
		})
		if err != nil {
			s.duplicateFailed(w, r, back, err)
			return
		}
		notice = "The group is offered again."
	case "link", "ignore":
		verdict := account.VerdictLinked
		if action == "ignore" {
			verdict = account.VerdictIgnored
		}
		members, err := s.validateDupMembers(sess, r.PostForm["member"])
		if err != nil {
			s.duplicateFailed(w, r, back, err)
			return
		}
		kind := strings.TrimSpace(r.PostFormValue("kind"))
		fields := fieldPreferences(r.PostForm)
		id := strings.TrimSpace(r.PostFormValue("group"))
		err = s.Store.UpdateDuplicates(sess.UserID, sess.DEK(), func(stored *account.Duplicates) error {
			memberList := make([]account.Member, 0, len(members))
			for _, m := range members {
				memberList = append(memberList, m.member())
			}
			_, decideErr := stored.Decide(id, kind, verdict, memberList, fields, time.Now())
			return decideErr
		})
		if err != nil {
			s.duplicateFailed(w, r, back, err)
			return
		}
		if verdict == account.VerdictLinked {
			notice = "Linked. The records stay where they are; nothing was changed on any server."
		} else {
			notice = "Marked as different records. They will not be offered again."
		}
	default:
		s.duplicateFailed(w, r, back, errors.New("unknown action"))
		return
	}

	s.redirectNotice(w, r, back, notice)
}

// duplicateFailed says what went wrong on the screen the action came from. A
// refused decision is not an error page: the groups are still there and still
// worth looking at.
func (s *Server) duplicateFailed(w http.ResponseWriter, r *http.Request, back string, err error) {
	s.redirectNotice(w, r, back, capitalize(err.Error()))
}

// fieldPreferences reads the remembered field choices out of a form. They arrive
// as field:PROPERTY, which keeps them apart from the rest of the form without a
// second parser.
func fieldPreferences(form map[string][]string) map[string]string {
	out := make(map[string]string)
	for name, values := range form {
		property, ok := cutFieldPrefix(name)
		if !ok || len(values) == 0 {
			continue
		}
		if value := strings.TrimSpace(values[0]); value != "" {
			out[property] = value
		}
	}
	return out
}

func cutFieldPrefix(name string) (string, bool) {
	const prefix = "field:"
	if !strings.HasPrefix(name, prefix) {
		return "", false
	}
	return strings.ToUpper(strings.TrimSpace(name[len(prefix):])), true
}

// dupRef is a member as a form carries it: the tuple a decision is keyed by,
// plus the object path, which is what a write needs and what a UID alone does
// not give (§15).
type dupRef struct {
	AccountID  string
	Collection string
	UID        string
	Path       string
}

func (d dupRef) member() account.Member {
	return account.Member{AccountID: d.AccountID, Collection: d.Collection, UID: d.UID}
}

const dupTokenVersion = "1"

func encodeRecordMember(record merge.Record) string {
	return encodeDupRef(dupRef{
		AccountID: record.AccountID, Collection: record.Collection,
		UID: record.UID(), Path: record.Object.Path,
	})
}

func encodeStoredMember(member account.Member) string {
	return encodeDupRef(dupRef{
		AccountID: member.AccountID, Collection: member.Collection, UID: member.UID,
	})
}

func encodeDupRef(ref dupRef) string {
	raw := strings.Join([]string{dupTokenVersion, ref.AccountID, ref.Collection, ref.UID, ref.Path}, "\x1f")
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeDupRef(token string) (dupRef, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil {
		return dupRef{}, errors.New("that record reference is not readable")
	}
	parts := strings.Split(string(raw), "\x1f")
	if len(parts) != 5 || parts[0] != dupTokenVersion {
		return dupRef{}, errors.New("that record reference is not readable")
	}
	ref := dupRef{
		AccountID:  strings.TrimSpace(parts[1]),
		Collection: normalizeCollectionPath(parts[2]),
		UID:        strings.TrimSpace(parts[3]),
		Path:       strings.TrimSpace(parts[4]),
	}
	if ref.AccountID == "" || ref.Collection == "" || ref.UID == "" {
		return dupRef{}, errors.New("that record reference is incomplete")
	}
	return ref, nil
}

// validateDupMembers checks every reference a form submitted against the
// collections the person actually has.
//
// A submitted field names an account, a collection and an object path, and all
// three end up in a request to somebody's server: a path that is not inside the
// named collection is refused rather than sent, so a form cannot be used to reach
// the rest of a DAV server through the merge.
func (s *Server) validateDupMembers(sess *session.Session, tokens []string) ([]dupRef, error) {
	if len(tokens) < 2 {
		return nil, errors.New("a group needs at least two records")
	}
	accounts, err := s.Store.ListDAVAccounts(sess.UserID, sess.DEK())
	if err != nil {
		return nil, err
	}
	known := make(map[string]bool, len(accounts))
	for _, acc := range accounts {
		if !acc.Enabled {
			continue
		}
		for _, col := range acc.Collections {
			known[acc.ID+"|"+normalizeCollectionPath(col.Path)] = true
		}
	}
	out := make([]dupRef, 0, len(tokens))
	seen := make(map[string]bool, len(tokens))
	for _, token := range tokens {
		ref, refErr := decodeDupRef(token)
		if refErr != nil {
			return nil, refErr
		}
		if !known[ref.AccountID+"|"+ref.Collection] {
			return nil, errors.New("that collection is not one of yours")
		}
		if ref.Path != "" && !strings.HasPrefix(ref.Path, ref.Collection) {
			return nil, fmt.Errorf("%s is not inside %s", ref.Path, ref.Collection)
		}
		key := ref.AccountID + "|" + ref.Collection + "|" + ref.UID
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, ref)
	}
	if len(out) < 2 {
		return nil, errors.New("a group needs at least two records")
	}
	return out, nil
}

// duplicateMarks are the stored decisions in the form a row builder needs: the
// linked group a record is in, how large that group is, and the groups it was
// decided against.
type duplicateMarks struct {
	linked  map[string]string
	sizes   map[string]int
	ignored map[string][]string
}

// contactMarks are the marks of one record.
type contactMarks struct {
	Group   string
	Ignored []string
}

func (s *Server) duplicateMarks(sess *session.Session) duplicateMarks {
	decisions, err := s.Store.Duplicates(sess.UserID, sess.DEK())
	if err != nil {
		s.logError("read duplicate decisions", err)
		return duplicateMarks{}
	}
	return marksOf(decisions)
}

func marksOf(decisions account.Duplicates) duplicateMarks {
	marks := duplicateMarks{
		linked:  make(map[string]string, len(decisions.Groups)),
		sizes:   make(map[string]int, len(decisions.Groups)),
		ignored: make(map[string][]string, len(decisions.Groups)),
	}
	for _, group := range decisions.Groups {
		for _, member := range group.Members {
			key := member.Key()
			switch group.Verdict {
			case account.VerdictLinked:
				marks.linked[key] = group.ID
				marks.sizes[key] = len(group.Members)
			case account.VerdictIgnored:
				marks.ignored[key] = append(marks.ignored[key], group.ID)
			}
		}
	}
	return marks
}

func (m duplicateMarks) of(src fanout.Source, contact model.Contact) contactMarks {
	key := account.Member{AccountID: src.AccountID, Collection: src.Collection, UID: contact.UID}.Key()
	return contactMarks{Group: m.linked[key], Ignored: m.ignored[key]}
}

// linkedSize returns how many records the linked group of one object holds, or
// zero when it is in none. It is the number the badge in a single collection's
// list prints (§15).
func (m duplicateMarks) linkedSize(accountID, collection, uid string) int {
	return m.sizes[account.Member{AccountID: accountID, Collection: collection, UID: uid}.Key()]
}
