// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/fanout"
	"gitea.mixdep.ru/mix/carrel/internal/merge"
	"gitea.mixdep.ru/mix/carrel/internal/model"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

// findMode is what a fan-out is for. It decides which sources are polled, what
// each source is asked, and how the merged answer is grouped.
type findMode string

const (
	// modeTime is the calendar section's merged agenda: every ticked calendar,
	// one date range.
	modeTime findMode = "time"
	// modePeople is the contacts section's merged directory: every ticked
	// address book.
	modePeople findMode = "people"
	// modeTasks is the tasks section's merged list: every ticked task list.
	modeTasks findMode = "tasks"
	// modeNotes is the notes section's merged list: every ticked notebook.
	modeNotes findMode = "notes"
	// modeSearch is the cross-source search of §16, over both kinds at once.
	modeSearch findMode = "search"
	// modeTimeline is one contact's history across every calendar (§23.9).
	modeTimeline findMode = "timeline"
	// modeDuplicates loads whole records rather than rows, because the scoring
	// of §15 needs the cards and the events themselves.
	modeDuplicates findMode = "duplicates"
)

func (m findMode) valid() bool {
	switch m {
	case modeTime, modePeople, modeTasks, modeNotes, modeSearch, modeTimeline, modeDuplicates:
		return true
	}
	return false
}

func (m findMode) isSection() bool {
	switch m {
	case modeTime, modePeople, modeTasks, modeNotes:
		return true
	}
	return false
}

func (m findMode) section() string {
	switch m {
	case modePeople:
		return "contacts"
	case modeTasks:
		return "tasks"
	case modeNotes:
		return "notes"
	default:
		return "calendar"
	}
}

func (m findMode) allLabel() string {
	switch m {
	case modePeople:
		return "All address books"
	case modeTasks:
		return "All lists"
	case modeNotes:
		return "All notebooks"
	default:
		return "All calendars"
	}
}

func (m findMode) collectionNoun() (one, many string) {
	switch m {
	case modePeople:
		return "book", "books"
	case modeTasks:
		return "list", "lists"
	case modeNotes:
		return "notebook", "notebooks"
	default:
		return "calendar", "calendars"
	}
}

func (m findMode) sectionHome() string {
	return "/app/" + m.section()
}

// view is the account.Views key a mode saves its source selection under, so the
// unified agenda and the search remember separate choices (§14).
func (m findMode) view() string {
	switch m {
	case modePeople:
		return account.ViewContacts
	case modeTasks:
		return account.ViewTasks
	case modeNotes:
		return account.ViewNotes
	case modeSearch, modeTimeline:
		return account.ViewSearch
	case modeDuplicates:
		return account.ViewDuplicates
	default:
		return account.ViewAgenda
	}
}

// findRequest is everything a fan-out screen needs to be rebuilt from a URL.
// Progress fragments carry it in the query string rather than in server state:
// a task then holds only what it polled, and a reload cannot desynchronise the
// page from the poll it is showing.
type findRequest struct {
	Mode findMode
	// Query is the search text, empty outside modeSearch.
	Query string
	// From and To bound modeTime, as YYYY-MM-DD.
	From string
	To   string
	// Kinds limits modeTime to events, tasks or notes.
	Kinds []string
	// Tab is the kind filter on search, timeline and duplicates screens.
	Tab string
	// Contact identifies the subject of modeTimeline.
	Account    string
	Collection string
	UID        string
	// Poll is set once the browser has fallen back from the stream, so every
	// fragment after that carries the poller instead of waiting for events
	// that are not coming (§16).
	Poll bool
}

func parseFindRequest(r *http.Request) findRequest {
	q := r.URL.Query()
	req := findRequest{
		Mode:       findMode(strings.TrimSpace(q.Get("mode"))),
		Query:      strings.TrimSpace(q.Get("q")),
		From:       strings.TrimSpace(q.Get("from")),
		To:         strings.TrimSpace(q.Get("to")),
		Account:    strings.TrimSpace(q.Get("account")),
		Collection: strings.TrimSpace(q.Get("col")),
		UID:        strings.TrimSpace(q.Get("uid")),
		Poll:       q.Get("poll") == "1",
	}
	if !req.Mode.valid() {
		req.Mode = modeTime
	}
	for _, kind := range q["kind"] {
		switch strings.ToLower(strings.TrimSpace(kind)) {
		case "events", "tasks", "notes":
			req.Kinds = append(req.Kinds, strings.ToLower(strings.TrimSpace(kind)))
		}
	}
	if tab := strings.ToLower(strings.TrimSpace(q.Get("tab"))); tab != "" {
		req.Tab = tab
	}
	return req
}

func (f findRequest) values() url.Values {
	v := url.Values{"mode": {string(f.Mode)}}
	for _, key := range []struct{ name, value string }{
		{"q", f.Query}, {"from", f.From}, {"to", f.To},
		{"account", f.Account}, {"col", f.Collection}, {"uid", f.UID},
	} {
		if key.value != "" {
			v.Set(key.name, key.value)
		}
	}
	for _, kind := range f.Kinds {
		v.Add("kind", kind)
	}
	if f.Tab != "" {
		v.Set("tab", f.Tab)
	}
	if f.Poll {
		v.Set("poll", "1")
	}
	return v
}

// Wants reports whether a kind is included in modeTime. It is called from the
// template as well as the poll, so the ticked boxes and the poll cannot disagree.
func (f findRequest) Wants(kind string) bool {
	if len(f.Kinds) == 0 {
		return true
	}
	for _, k := range f.Kinds {
		if k == kind {
			return true
		}
	}
	return false
}

// resultRow is one merged record, ready to print. The fan-out carries these as
// opaque data, so a source that is still running cannot change how an already
// rendered row looks.
type resultRow struct {
	Kind          string
	Title         string
	Subtitle      string
	TimeLabel     string
	TimeZoneLabel string
	GroupKey      string
	GroupLabel    string
	Sort          string
	URL           string
	Account       string
	Collection    string
	Color         string
	Tags          []string
	Done          bool
	Overdue       bool
	// MatchLabel says why a row matched on search or timeline screens (§1.8).
	MatchLabel string
	// Files lists attachment names carried on the parent row for the files tab.
	Files []string

	// The duplicate marks of §15. A row carries what it takes to score it, so
	// the merged list can collapse a group without loading anything again.
	Print merge.Fingerprint
	// DupGroup is the linked group the record belongs to, empty when it is in
	// none.
	DupGroup string
	// DupIgnored are the groups the record was decided against, so a pair the
	// person has rejected is not offered again (§21).
	DupIgnored []string
	// DupCount is how many records the row stands for: one, or the size of the
	// group it collapses.
	DupCount int
	// DupLinked reports that the group was linked by the person rather than
	// merely detected.
	DupLinked bool
	// DupURL is where the badge leads.
	DupURL string
	// Members are the rows a collapsed row expands into, in source order.
	Members []resultRow
	// Emails and Phones are the merged repeatable fields of a collapsed row:
	// §15 unions them rather than letting one record's win.
	Emails []string
	Phones []string
	Org    string
}

// Collapsed reports whether the row stands for more than one record.
func (r resultRow) Collapsed() bool { return r.DupCount > 1 }

func (r resultRow) Initials() string {
	var out []rune
	for _, word := range strings.Fields(r.Title) {
		for _, rr := range word {
			if unicode.IsLetter(rr) {
				out = append(out, unicode.ToUpper(rr))
				break
			}
		}
		if len(out) == 2 {
			break
		}
	}
	return string(out)
}

func (r resultRow) PhonesLabel() string { return strings.Join(r.Phones, ", ") }

func (r resultRow) EmailsLabel() string { return strings.Join(r.Emails, ", ") }

func (r resultRow) SourceLabel() string {
	if r.Account == "" {
		return r.Collection
	}
	if r.Collection == "" {
		return r.Account
	}
	return r.Account + " · " + r.Collection
}

func (r resultRow) PanelURL() string {
	if r.URL == "" {
		return ""
	}
	return r.URL + "/panel"
}

type resultGroup struct {
	Label string
	Rows  []resultRow
}

type findView struct {
	Request  findRequest
	Mode     findMode
	Title    string
	Subject  string
	TaskID   string
	Sources  []sourceRow
	Snapshot fanout.Snapshot
	Groups   []resultGroup
	// Duplicates is filled for modeDuplicates only: the groups of §15 built
	// from the records this poll has loaded so far.
	Duplicates duplicatesData
	UseSSE     bool
	PollMillis int
	StreamURL  string
	ResultsURL string
	PollURL    string
	RetryURL   string
	CancelURL  string
	SourcesURL string
	Base       string
	// Back is where the screen came from, when it has one place to return to.
	Back         string
	NoSources    bool
	Unusable     string
	FromLabel    string
	ToLabel      string
	SectionRail  sectionRail
	PrintDate    string
	PrintSection string
	CreateURL    string
	CreateLabel  string
	// Person is filled on the contact screen of §1.8.
	Person personPanel
}

// personPanel is the read-only contact summary beside a person's timeline.
type personPanel struct {
	AccountID    string
	ColEnc       string
	UID          string
	Contact      model.Contact
	PhotoURL     string
	AccountLabel string
	Collection   discovery.Collection
	ReadOnly     bool
	EditURL      string
	NoteURL      string
	EventURL     string
}

// kindTab is one segment in a kind filter bar.
type kindTab struct {
	Label  string
	Value  string
	Count  int
	Active bool
	URL    string
}

// SectionHome redirects legacy /app/unified URLs to the section that now owns
// the merged view (§1.7).
func (s *Server) SectionHome(w http.ResponseWriter, r *http.Request) {
	req := parseFindRequest(r)
	switch req.Mode {
	case modePeople:
		http.Redirect(w, r, s.Path("/app/contacts"), http.StatusSeeOther)
	default:
		q := url.Values{}
		for _, key := range []struct{ name, value string }{
			{"from", req.From}, {"to", req.To},
		} {
			if key.value != "" {
				q.Set(key.name, key.value)
			}
		}
		for _, kind := range req.Kinds {
			q.Add("kind", kind)
		}
		target := s.Path("/app/calendar")
		if encoded := q.Encode(); encoded != "" {
			target += "?" + encoded
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
	}
}

// Search is the cross-source search of §16.
func (s *Server) Search(w http.ResponseWriter, r *http.Request) {
	req := parseFindRequest(r)
	req.Mode = modeSearch
	if req.Query == "" {
		v := s.View(r, "Search")
		v.Data = findView{
			Request: req, Mode: modeSearch, Title: "Search", Base: s.Path(""),
			Sources: s.findSourcesOrNil(r, req), SourcesURL: s.Path("/app/search/sources"),
		}
		s.Render(w, "search.html", v)
		return
	}
	s.startFind(w, r, req, "search.html")
}

// ContactTimeline redirects the legacy timeline URL to the contact screen (§1.8).
func (s *Server) ContactTimeline(w http.ResponseWriter, r *http.Request) {
	colEnc := r.PathValue("col")
	if _, err := DecodeCollectionPath(colEnc); err != nil {
		http.NotFound(w, r)
		return
	}
	target := s.Path("/app/contacts/" + r.PathValue("account") + "/" + colEnc + "/" + urlPathEscape(r.PathValue("uid")))
	if q := r.URL.RawQuery; q != "" {
		target += "?" + q
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// startFind begins a poll and renders the page that will follow it.
func (s *Server) startFind(w http.ResponseWriter, r *http.Request, req findRequest, template string) {
	sess := SessionFrom(r)
	view := findView{
		Request: req, Mode: req.Mode, Title: findTitle(req),
		UseSSE: s.Progress.SSE(), PollMillis: s.pollMillis(), Base: s.Path(""),
		PrintDate: time.Now().UTC().Format("2006-01-02 15:04 UTC"),
	}
	view.PrintSection = findPrintSection(req.Mode)
	view.SourcesURL = s.sourcesURL(req.Mode)
	if req.Mode == modeTime {
		view.FromLabel, view.ToLabel = req.From, req.To
	}
	if req.Mode.isSection() {
		if rail, railErr := s.buildSectionRail(sess, req, "", ""); railErr == nil {
			view.SectionRail = rail
			s.fillSectionCreate(&view, sess, req, rail)
		}
	} else if req.Mode == modeTimeline {
		if rail, railErr := s.buildSectionRail(sess, findRequest{Mode: modeTimeline}, "", ""); railErr == nil {
			rail.RailTitle = "Where to look"
			rail.Mode = modeTimeline
			rail.SourcesURL = s.sourcesURL(modeTimeline)
			view.SectionRail = rail
		}
	}
	if s.Fanout == nil {
		view.Unusable = "Cross-source polling is not configured on this instance."
		s.renderFind(w, r, template, view)
		return
	}
	rows, err := s.findSources(sess, req)
	if err != nil {
		view.Unusable = userFacingDAVError(err)
		s.renderFind(w, r, template, view)
		return
	}
	view.Sources = rows
	if req.Mode == modeTimeline {
		if subject, subjErr := s.timelineSubject(r.Context(), sess, req); subjErr == nil {
			view.Subject = subject.name
			req.Query = strings.Join(subject.terms, "\n")
			view.Request = req
			view.Title = subject.name
		} else {
			view.Unusable = userFacingDAVError(subjErr)
			s.renderFind(w, r, template, view)
			return
		}
	}
	selected := selectedRows(rows)
	if len(selected) == 0 {
		view.NoSources = true
		s.renderFind(w, r, template, view)
		return
	}
	query, err := s.findQuery(sess, req)
	if err != nil {
		view.Unusable = userFacingDAVError(err)
		s.renderFind(w, r, template, view)
		return
	}
	task, err := s.Fanout.Start(sess.ID, fanoutSources(selected), query, s.fanoutOptions())
	if err != nil {
		if errors.Is(err, fanout.ErrNoSources) {
			view.NoSources = true
		} else {
			view.Unusable = userFacingDAVError(err)
		}
		s.renderFind(w, r, template, view)
		return
	}
	view.TaskID = task.ID
	s.fillFindURLs(&view, req, task.ID)
	s.fillResults(r, &view, req, task)
	s.renderFind(w, r, template, view)
}

// fillResults reads the poll and turns it into what the screen prints. It is the
// one place that does so, so an update pushed over the stream and the first
// render of the page cannot disagree about the same snapshot.
func (s *Server) fillResults(r *http.Request, view *findView, req findRequest, task *fanout.Task) {
	view.Snapshot = task.Snapshot()
	view.Groups = groupRows(req, view.Snapshot, s.timezone(), s.dupDisplay())
	if req.Mode == modeDuplicates {
		view.Duplicates = s.duplicateData(SessionFrom(r), view.Snapshot)
	}
}

func (s *Server) renderFind(w http.ResponseWriter, r *http.Request, template string, view findView) {
	v := s.View(r, view.Title)
	v.Data = view
	s.Render(w, template, v)
}

func (s *Server) fillFindURLs(view *findView, req findRequest, taskID string) {
	base := s.Path("/app/find/" + urlPathEscape(taskID))
	q := req.values().Encode()
	view.ResultsURL = base + "/results?" + q
	view.StreamURL = base + "/stream?" + q
	view.RetryURL = base + "/retry?" + q
	view.CancelURL = base + "/cancel?" + q
	polling := req
	polling.Poll = true
	view.PollURL = base + "/results?" + polling.values().Encode()
}

// FindResults is the progress-and-results fragment, used by the poll fallback
// and after a retry (§16).
func (s *Server) FindResults(w http.ResponseWriter, r *http.Request) {
	view, ok := s.findFragment(w, r)
	if !ok {
		return
	}
	s.RenderFragment(w, fragmentTemplate(view.Mode), s.fragmentView(r, view))
}

// fragmentTemplate is the partial a mode's updates arrive in. The duplicates
// screen prints groups rather than a merged list, so it follows its own poll with
// its own fragment (§15, §16).
func fragmentTemplate(mode findMode) string {
	if mode == modeDuplicates {
		return "duplicate_results.html"
	}
	return "find_results.html"
}

// fragmentView wraps a fan-out view in the frame data a template expects.
func (s *Server) fragmentView(r *http.Request, view findView) View {
	v := s.View(r, view.Title)
	v.Data = view
	return v
}

// FindRetry polls one source again without touching the others (§16).
func (s *Server) FindRetry(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sess := SessionFrom(r)
	task, err := s.findTask(sess, r.PathValue("task"))
	if err != nil {
		http.Error(w, "that poll is no longer running.", http.StatusGone)
		return
	}
	task.Retry(strings.TrimSpace(r.PostFormValue("source")))
	view, ok := s.findFragment(w, r)
	if !ok {
		return
	}
	s.RenderFragment(w, fragmentTemplate(view.Mode), s.fragmentView(r, view))
}

// FindCancel stops a poll. §16 requires the partial results to stay on screen,
// so the fragment is rendered from the snapshot after cancelling rather than
// replaced by an empty state.
func (s *Server) FindCancel(w http.ResponseWriter, r *http.Request) {
	sess := SessionFrom(r)
	taskID := r.PathValue("task")
	task, err := s.findTask(sess, taskID)
	if err != nil {
		http.Error(w, "that poll is no longer running.", http.StatusGone)
		return
	}
	task.Cancel()
	view := s.viewFromTask(r, parseFindRequest(r), task)
	s.RenderFragment(w, fragmentTemplate(view.Mode), s.fragmentView(r, view))
}

// findFragment resolves the task named in the path and builds the fragment view.
func (s *Server) findFragment(w http.ResponseWriter, r *http.Request) (findView, bool) {
	sess := SessionFrom(r)
	task, err := s.findTask(sess, r.PathValue("task"))
	if err != nil {
		// A task that has been swept or belongs to another session is gone for
		// good; say so once instead of letting the poll spin forever.
		http.Error(w, "that poll is no longer running.", http.StatusGone)
		return findView{}, false
	}
	return s.viewFromTask(r, parseFindRequest(r), task), true
}

func (s *Server) viewFromTask(r *http.Request, req findRequest, task *fanout.Task) findView {
	view := findView{
		Request: req, Mode: req.Mode, Title: findTitle(req), TaskID: task.ID,
		UseSSE: s.Progress.SSE() && !req.Poll, PollMillis: s.pollMillis(),
		SourcesURL: s.sourcesURL(req.Mode), Base: s.Path(""),
		PrintDate: time.Now().UTC().Format("2006-01-02 15:04 UTC"),
	}
	view.PrintSection = findPrintSection(req.Mode)
	s.fillFindURLs(&view, req, task.ID)
	s.fillResults(r, &view, req, task)
	return view
}

func (s *Server) findTask(sess *session.Session, taskID string) (*fanout.Task, error) {
	if s.Fanout == nil || sess == nil {
		return nil, fanout.ErrNoTask
	}
	return s.Fanout.Get(sess.ID, strings.TrimSpace(taskID))
}

// FindStream pushes the same fragment over one event-source connection (§16).
//
// The stream ends itself as soon as the task is finished: an idle connection per
// open tab is exactly what §13 asks not to keep. A heartbeat comment goes out
// meanwhile so a proxy with an idle timeout does not drop a live poll.
func (s *Server) FindStream(w http.ResponseWriter, r *http.Request) {
	if !s.Progress.SSE() {
		http.Error(w, "streaming is disabled", http.StatusNotFound)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is unavailable", http.StatusNotImplemented)
		return
	}
	sess := SessionFrom(r)
	task, err := s.findTask(sess, r.PathValue("task"))
	if err != nil {
		http.Error(w, "that poll is no longer running.", http.StatusGone)
		return
	}
	req := parseFindRequest(r)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	// Ask nginx not to buffer: a buffered event stream is a stream that
	// arrives all at once at the end, which is worse than polling.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	updates, release := task.Subscribe()
	defer release()

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	// A last resort ceiling: whatever happens to the task, this connection is
	// not held open for longer than a poll could possibly take.
	deadline := time.NewTimer(s.streamLimit())
	defer deadline.Stop()

	for {
		view := s.viewFromTask(r, req, task)
		if err := s.writeSSE(w, fragmentTemplate(view.Mode), "results", s.fragmentView(r, view)); err != nil {
			return
		}
		if !view.Snapshot.Running {
			_ = writeSSERaw(w, "done", "end")
			flusher.Flush()
			return
		}
		flusher.Flush()
		select {
		case <-updates:
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-deadline.C:
			_ = writeSSERaw(w, "done", "end")
			flusher.Flush()
			return
		case <-r.Context().Done():
			// The tab is gone. The task stays alive on purpose: the person may
			// be coming back with the same page, and the registry will sweep it
			// if they do not (§16).
			return
		}
	}
}

// writeSSE sends one rendered fragment as a single event. Every line of the
// fragment becomes its own data line, which is what the protocol requires and
// what keeps a multi-line HTML body from ending the event early.
func (s *Server) writeSSE(w http.ResponseWriter, template, event string, v View) error {
	html, err := s.Fragment(template, v)
	if err != nil {
		return err
	}
	return writeSSERaw(w, event, string(html))
}

func writeSSERaw(w http.ResponseWriter, event, payload string) error {
	var b strings.Builder
	b.WriteString("event: ")
	b.WriteString(event)
	b.WriteString("\n")
	for _, line := range strings.Split(strings.ReplaceAll(payload, "\r\n", "\n"), "\n") {
		b.WriteString("data: ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	_, err := w.Write([]byte(b.String()))
	return err
}

// FindSources saves which collections a fan-out screen polls (§14).
func (s *Server) FindSources(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sess := SessionFrom(r)
	req := parseFindRequest(r)
	if mode := findMode(strings.TrimSpace(r.PostFormValue("mode"))); mode.valid() {
		req.Mode = mode
	}
	rows, err := s.findSources(sess, req)
	if err != nil {
		http.Error(w, userFacingDAVError(err), http.StatusBadGateway)
		return
	}
	if err := s.saveSelection(sess, req.Mode.view(), r.PostForm["source"], rows); err != nil {
		http.Error(w, userFacingDAVError(err), http.StatusBadGateway)
		return
	}
	// Restarting the poll is the point of changing the selection, so the
	// browser goes back to the screen itself rather than to a fragment.
	target := SafeRedirect(r.PostFormValue("back"), s.screenURL(req))
	if IsHTMX(r) {
		w.Header().Set("HX-Redirect", target)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (s *Server) fillSectionCreate(view *findView, sess *session.Session, req findRequest, rail sectionRail) {
	dest, ok := s.defaultCollection(sess, req.Mode.view(), rail.Sources)
	if !ok {
		return
	}
	switch req.Mode {
	case modePeople:
		view.CreateURL = s.Path("/app/contacts/" + dest.AccountID + "/" + dest.ColEnc + "/new")
		view.CreateLabel = "New contact"
	case modeTasks:
		view.CreateURL = s.Path("/app/tasks/" + dest.AccountID + "/" + dest.ColEnc + "/new")
		view.CreateLabel = "New task"
	case modeNotes:
		view.CreateURL = s.Path("/app/notes/" + dest.AccountID + "/" + dest.ColEnc + "/new")
		view.CreateLabel = "New note"
	default:
		view.CreateURL = s.Path("/app/calendar/" + dest.AccountID + "/" + dest.ColEnc + "/new")
		view.CreateLabel = "New event"
	}
}

func (s *Server) sourcesURL(mode findMode) string {
	switch mode {
	case modeTime:
		return s.Path("/app/calendar/sources")
	case modePeople:
		return s.Path("/app/contacts/sources")
	case modeTasks:
		return s.Path("/app/tasks/sources")
	case modeNotes:
		return s.Path("/app/notes/sources")
	case modeDuplicates:
		return s.Path("/app/duplicates/sources")
	default:
		return s.Path("/app/search/sources")
	}
}

func (s *Server) screenURL(req findRequest) string {
	values := req.values()
	switch req.Mode {
	case modeSearch:
		return s.Path("/app/search") + "?" + values.Encode()
	case modeDuplicates:
		return s.Path("/app/duplicates")
	case modeTimeline:
		return s.Path("/app/contacts/" + req.Account + "/" + EncodeCollectionPath(req.Collection) + "/" + urlPathEscape(req.UID))
	case modePeople:
		return s.Path("/app/contacts")
	case modeTasks:
		return s.Path("/app/tasks")
	case modeNotes:
		return s.Path("/app/notes")
	default:
		target := s.Path("/app/calendar")
		q := url.Values{}
		for _, key := range []struct{ name, value string }{
			{"from", req.From}, {"to", req.To},
		} {
			if key.value != "" {
				q.Set(key.name, key.value)
			}
		}
		for _, kind := range req.Kinds {
			q.Add("kind", kind)
		}
		if encoded := q.Encode(); encoded != "" {
			target += "?" + encoded
		}
		return target
	}
}

// findSources lists the collections a mode can poll. A search spans both kinds,
// which is why the list is built from two calls and kept in one slice (§16).
func (s *Server) findSources(sess *session.Session, req findRequest) ([]sourceRow, error) {
	switch req.Mode {
	case modePeople:
		return s.collectionsOfKind(sess, discovery.KindAddressBook, req.Mode.view(), "")
	case modeTasks:
		return s.collectionsOfKind(sess, discovery.KindCalendar, req.Mode.view(), dav.CompTodo)
	case modeNotes:
		return s.collectionsOfKind(sess, discovery.KindCalendar, req.Mode.view(), dav.CompJournal)
	case modeTimeline:
		return s.collectionsOfKind(sess, discovery.KindCalendar, req.Mode.view(), "")
	case modeSearch, modeDuplicates:
		calendars, err := s.collectionsOfKind(sess, discovery.KindCalendar, req.Mode.view(), "")
		if err != nil {
			return nil, err
		}
		books, err := s.collectionsOfKind(sess, discovery.KindAddressBook, req.Mode.view(), "")
		if err != nil {
			return nil, err
		}
		storages, err := s.collectionsOfKind(sess, discovery.KindFiles, req.Mode.view(), "")
		if err != nil {
			return nil, err
		}
		return append(append(calendars, books...), storages...), nil
	default:
		return s.collectionsOfKind(sess, discovery.KindCalendar, req.Mode.view(), "")
	}
}

func (s *Server) findSourcesOrNil(r *http.Request, req findRequest) []sourceRow {
	rows, err := s.findSources(SessionFrom(r), req)
	if err != nil {
		return nil
	}
	return rows
}

func (s *Server) fanoutOptions() fanout.Options {
	return fanout.Options{
		SourceTimeout: s.Progress.SourceTimeout.Duration(),
		TotalTimeout:  s.Progress.TotalTimeout.Duration(),
	}
}

func (s *Server) pollMillis() int {
	if s.Progress.PollMillis > 0 {
		return s.Progress.PollMillis
	}
	return 700
}

// streamLimit keeps an event-source connection from outliving the poll it
// reports on by more than a margin.
func (s *Server) streamLimit() time.Duration {
	total := s.Progress.TotalTimeout.Duration()
	if total <= 0 {
		total = fanout.DefaultTotalTimeout
	}
	return total + 2*time.Minute
}

func defaultUnifiedRange(loc *time.Location) (string, string) {
	today := time.Now().In(loc)
	return today.Format("2006-01-02"), today.AddDate(0, 0, 14).Format("2006-01-02")
}

func findTitle(req findRequest) string {
	switch req.Mode {
	case modePeople:
		return "Contacts"
	case modeTasks:
		return "Tasks"
	case modeNotes:
		return "Notes"
	case modeSearch:
		if req.Query != "" {
			return "Search: " + req.Query
		}
		return "Search"
	case modeTimeline:
		return "Timeline"
	case modeDuplicates:
		return "Duplicates"
	default:
		return "Agenda"
	}
}

func findPrintSection(mode findMode) string {
	switch mode {
	case modePeople:
		return "contacts"
	case modeTasks:
		return "tasks"
	case modeNotes:
		return "notes"
	case modeSearch:
		return "search"
	case modeDuplicates:
		return "duplicates"
	default:
		return "agenda"
	}
}

// groupRows turns a snapshot into the printed groups. Ordering is by mode: an
// agenda reads forwards in time, a timeline backwards, and a search by kind.
func groupRows(req findRequest, snap fanout.Snapshot, loc *time.Location, dup dupDisplay) []resultGroup {
	rows := make([]resultRow, 0, len(snap.Items))
	for _, item := range snap.Items {
		if row, ok := item.Data.(resultRow); ok {
			rows = append(rows, row)
		}
	}
	if req.Mode == modeTimeline {
		rows = appendAttachmentRows(rows)
	}
	rows = filterRowsByTab(req, rows)
	descending := req.Mode == modeTimeline || req.Mode == modeNotes
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Sort == rows[j].Sort {
			return strings.ToLower(rows[i].Title) < strings.ToLower(rows[j].Title)
		}
		if descending {
			return rows[i].Sort > rows[j].Sort
		}
		return rows[i].Sort < rows[j].Sort
	})
	// The merged directory is where §15 asks for the group to be one row with a
	// badge. It happens after sorting, so a collapsed row keeps the place its
	// first record had rather than jumping to the end.
	if req.Mode == modePeople {
		rows = collapseDuplicates(rows, dup)
	}
	var groups []resultGroup
	index := make(map[string]int)
	for _, row := range rows {
		at, ok := index[row.GroupKey]
		if !ok {
			groups = append(groups, resultGroup{Label: row.GroupLabel})
			at = len(groups) - 1
			index[row.GroupKey] = at
		}
		groups[at].Rows = append(groups[at].Rows, row)
	}
	return groups
}

// timelineSubject is the person a timeline is about, and the terms to look for.
type timelineSubject struct {
	name  string
	terms []string
}

func (s *Server) timelineSubject(ctx context.Context, sess *session.Session, req findRequest) (timelineSubject, error) {
	p, acc, err := s.contactsProvider(sess, req.Account)
	if err != nil {
		return timelineSubject{}, err
	}
	col, err := findAddressBook(acc, req.Collection)
	if err != nil {
		return timelineSubject{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	obj, err := p.Get(ctx, normalizeCollectionPath(col.Path), objectPathForUID(normalizeCollectionPath(col.Path), req.UID))
	if err != nil {
		return timelineSubject{}, err
	}
	contact, err := obj.Contact()
	if err != nil {
		return timelineSubject{}, err
	}
	subject := timelineSubject{name: contact.DisplayName()}
	seen := make(map[string]bool)
	add := func(term string) {
		term = strings.TrimSpace(term)
		if term == "" || seen[strings.ToLower(term)] {
			return
		}
		seen[strings.ToLower(term)] = true
		subject.terms = append(subject.terms, term)
	}
	// Addresses first: an ATTENDEE is a mailto and matches exactly, where a
	// name match is a guess that happens to be worth making as well (§23.9).
	for _, address := range contact.NormalizedEmails() {
		add(address)
	}
	add(contact.DisplayName())
	if len(subject.terms) == 0 {
		return subject, errors.New("this contact has no name or address to look for")
	}
	return subject, nil
}

// findQuery builds the per-source poll for a mode. It is a closure over the
// session and the request, and is called once per source on its own goroutine;
// everything it touches must be safe to use from several at once, which is why
// each call makes its own provider.
func (s *Server) findQuery(sess *session.Session, req findRequest) (fanout.Query, error) {
	loc := s.timezone()
	switch req.Mode {
	case modeTime:
		from, to, err := unifiedRange(req, loc)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, src fanout.Source) ([]fanout.Item, bool, error) {
			return s.pollCalendarRange(ctx, sess, src, req, from, to, loc)
		}, nil
	case modePeople:
		// The marks are read once, before the poll starts: a linked group is a
		// stored decision, so every row can be stamped with it as it is built
		// instead of the merged list going back to the store per source (§15).
		marks := s.duplicateMarks(sess)
		return func(ctx context.Context, src fanout.Source) ([]fanout.Item, bool, error) {
			return s.pollContacts(ctx, sess, src, marks)
		}, nil
	case modeDuplicates:
		from, to := duplicateEventRange(loc)
		return func(ctx context.Context, src fanout.Source) ([]fanout.Item, bool, error) {
			return s.pollRecords(ctx, sess, src, from, to, loc)
		}, nil
	case modeSearch:
		if req.Query == "" {
			return nil, errors.New("nothing to search for")
		}
		match := searchContext{mode: modeSearch, query: req.Query}
		return func(ctx context.Context, src fanout.Source) ([]fanout.Item, bool, error) {
			return s.pollSearch(ctx, sess, src, []string{req.Query}, loc, match)
		}, nil
	case modeTimeline:
		terms := strings.Split(req.Query, "\n")
		match := searchContext{mode: modeTimeline, terms: matchTerms{}.fromStrings(terms)}
		return func(ctx context.Context, src fanout.Source) ([]fanout.Item, bool, error) {
			return s.pollSearch(ctx, sess, src, terms, loc, match)
		}, nil
	case modeTasks:
		return func(ctx context.Context, src fanout.Source) ([]fanout.Item, bool, error) {
			return s.pollTasksList(ctx, sess, src, loc)
		}, nil
	case modeNotes:
		return func(ctx context.Context, src fanout.Source) ([]fanout.Item, bool, error) {
			return s.pollNotesList(ctx, sess, src, loc)
		}, nil
	}
	return nil, fmt.Errorf("unknown view")
}

func unifiedRange(req findRequest, loc *time.Location) (time.Time, time.Time, error) {
	from, err := time.ParseInLocation("2006-01-02", req.From, loc)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("from must be a YYYY-MM-DD date")
	}
	to := from.AddDate(0, 0, 14)
	if req.To != "" {
		parsed, parseErr := time.ParseInLocation("2006-01-02", req.To, loc)
		if parseErr != nil {
			return time.Time{}, time.Time{}, errors.New("to must be a YYYY-MM-DD date")
		}
		to = parsed.AddDate(0, 0, 1)
	}
	if !to.After(from) {
		return time.Time{}, time.Time{}, errors.New("the range ends before it starts")
	}
	// A range of years across ten calendars is a way to hang the instance, not
	// a view anybody reads.
	if to.Sub(from) > 400*24*time.Hour {
		return time.Time{}, time.Time{}, errors.New("the range is longer than a year")
	}
	return from, to, nil
}

// pollCalendarRange asks one calendar for the events, tasks and notes of a
// range. Each component is a separate report, so a server that supports only
// events still answers for the part it has.
func (s *Server) pollCalendarRange(ctx context.Context, sess *session.Session, src fanout.Source, req findRequest, from, to time.Time, loc *time.Location) ([]fanout.Item, bool, error) {
	p, _, err := s.calendarProvider(sess, src.AccountID)
	if err != nil {
		return nil, false, err
	}
	var items []fanout.Item
	cached := true
	var firstErr error
	if req.Wants("events") {
		agenda, err := p.Query(ctx, src.Collection, from, to)
		switch {
		case err != nil:
			firstErr = err
		default:
			cached = cached && agenda.FromCache
			for _, occ := range agenda.Occurrences {
				items = append(items, occurrenceItem(src, occ, loc))
			}
		}
	}
	if req.Wants("tasks") {
		set, err := p.QueryComponent(ctx, src.Collection, dav.CompTodo, time.Time{}, time.Time{})
		switch {
		case err != nil:
			if firstErr == nil {
				firstErr = err
			}
		default:
			cached = cached && set.FromCache
			for _, obj := range set.Objects {
				todo, todoErr := obj.Todo(loc)
				if todoErr != nil || todo.Due.IsZero() || !withinRange(todo.Due, from, to) {
					continue
				}
				items = append(items, todoItem(src, todo, loc))
			}
		}
	}
	if req.Wants("notes") {
		set, err := p.QueryComponent(ctx, src.Collection, dav.CompJournal, time.Time{}, time.Time{})
		switch {
		case err != nil:
			if firstErr == nil {
				firstErr = err
			}
		default:
			cached = cached && set.FromCache
			for _, obj := range set.Objects {
				note, noteErr := obj.Note(loc)
				if noteErr != nil || !withinRange(note.Date, from, to) {
					continue
				}
				items = append(items, noteItem(src, note, loc))
			}
		}
	}
	// A partial answer is still an answer, but a source that gave nothing and
	// failed is reported as failed rather than as empty (§16).
	if firstErr != nil && len(items) == 0 {
		return nil, false, firstErr
	}
	return items, cached && len(items) > 0, nil
}

func (s *Server) pollTasksList(ctx context.Context, sess *session.Session, src fanout.Source, loc *time.Location) ([]fanout.Item, bool, error) {
	p, _, err := s.calendarProvider(sess, src.AccountID)
	if err != nil {
		return nil, false, err
	}
	set, err := p.QueryComponent(ctx, src.Collection, dav.CompTodo, time.Time{}, time.Time{})
	if err != nil {
		return nil, false, err
	}
	items := make([]fanout.Item, 0, len(set.Objects))
	for _, obj := range set.Objects {
		todo, todoErr := obj.Todo(loc)
		if todoErr != nil || todo.Done() {
			continue
		}
		item := todoItem(src, todo, loc)
		item.Data = withURL(item.Data, s.icalURL("tasks", src, todo.UID))
		items = append(items, item)
	}
	return items, set.FromCache && len(items) > 0, nil
}

func (s *Server) pollNotesList(ctx context.Context, sess *session.Session, src fanout.Source, loc *time.Location) ([]fanout.Item, bool, error) {
	p, _, err := s.calendarProvider(sess, src.AccountID)
	if err != nil {
		return nil, false, err
	}
	set, err := p.QueryComponent(ctx, src.Collection, dav.CompJournal, time.Time{}, time.Time{})
	if err != nil {
		return nil, false, err
	}
	items := make([]fanout.Item, 0, len(set.Objects))
	for _, obj := range set.Objects {
		note, noteErr := obj.Note(loc)
		if noteErr != nil {
			continue
		}
		item := noteItem(src, note, loc)
		item.Data = withURL(item.Data, s.icalURL("notes", src, note.UID))
		items = append(items, item)
	}
	return items, set.FromCache && len(items) > 0, nil
}

func (s *Server) pollContacts(ctx context.Context, sess *session.Session, src fanout.Source, marks duplicateMarks) ([]fanout.Item, bool, error) {
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
		items = append(items, contactItem(src, contact, s.contactURL(src, contact.UID), marks.of(src, contact), ""))
	}
	// A multiget answers from the cache card by card, so there is no honest
	// "all of it came from cache" to report here; the panel says nothing rather
	// than guessing (§16).
	return items, false, nil
}

// searchContext carries match labelling through a poll (§1.8).
type searchContext struct {
	mode  findMode
	terms matchTerms
	query string
}

// pollSearch searches one source for any of the terms. A calendar source is
// searched for events, tasks and notes; an address book for cards.
func (s *Server) pollSearch(ctx context.Context, sess *session.Session, src fanout.Source, terms []string, loc *time.Location, match searchContext) ([]fanout.Item, bool, error) {
	if src.Kind == string(discovery.KindFiles) {
		query := searchQuery(match, terms)
		return s.searchFiles(ctx, sess, src, query)
	}
	if src.Kind == string(discovery.KindAddressBook) {
		return s.searchContacts(ctx, sess, src, terms, match)
	}
	p, _, err := s.calendarProvider(sess, src.AccountID)
	if err != nil {
		return nil, false, err
	}
	seen := make(map[string]bool)
	var items []fanout.Item
	var firstErr error
	for _, term := range terms {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		set, err := p.Search(ctx, src.Collection, term)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, obj := range set.Objects {
			if seen[obj.Path] {
				continue
			}
			seen[obj.Path] = true
			if item, ok := s.icalItem(src, obj, loc, match); ok {
				items = append(items, item)
			}
		}
	}
	if firstErr != nil && len(items) == 0 {
		return nil, false, firstErr
	}
	return items, false, nil
}

func (s *Server) searchContacts(ctx context.Context, sess *session.Session, src fanout.Source, terms []string, match searchContext) ([]fanout.Item, bool, error) {
	p, _, err := s.contactsProvider(sess, src.AccountID)
	if err != nil {
		return nil, false, err
	}
	seen := make(map[string]bool)
	var items []fanout.Item
	var firstErr error
	for _, term := range terms {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		result, err := p.Search(ctx, src.Collection, term)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, obj := range result.Objects {
			if seen[obj.Path] {
				continue
			}
			seen[obj.Path] = true
			contact, contactErr := obj.Contact()
			if contactErr != nil {
				continue
			}
			items = append(items, contactItem(src, contact, s.contactURL(src, contact.UID), contactMarks{}, contactSearchLabel(contact, searchQuery(match, terms))))
		}
	}
	if firstErr != nil && len(items) == 0 {
		return nil, false, firstErr
	}
	return items, false, nil
}

// icalItem turns any calendar object into a row, whatever component it is.
func (s *Server) icalItem(src fanout.Source, obj *model.Object, loc *time.Location, match searchContext) (fanout.Item, bool) {
	switch obj.Component() {
	case "VEVENT":
		event, err := obj.Event(loc)
		if err != nil {
			return fanout.Item{}, false
		}
		item := eventItem(src, event, s.icalURL("calendar", src, event.UID), loc)
		if row, ok := item.Data.(resultRow); ok {
			row.MatchLabel = eventMatchLabel(event, match.terms)
			if match.mode == modeSearch && row.MatchLabel == "" && row.TimeLabel != "" {
				row.MatchLabel = row.TimeLabel
			}
			row.Files = attachmentNames(event.Attachments)
			item.Data = row
		}
		return item, true
	case "VTODO":
		todo, err := obj.Todo(loc)
		if err != nil {
			return fanout.Item{}, false
		}
		item := todoItem(src, todo, loc)
		if row, ok := item.Data.(resultRow); ok {
			row.MatchLabel = todoMatchLabel(todo, match.terms)
			if match.mode == modeSearch && row.MatchLabel == "" && row.TimeLabel != "" {
				row.MatchLabel = row.TimeLabel
			}
			item.Data = withURL(row, s.icalURL("tasks", src, todo.UID))
		}
		return item, true
	case "VJOURNAL":
		note, err := obj.Note(loc)
		if err != nil {
			return fanout.Item{}, false
		}
		item := noteItem(src, note, loc)
		if row, ok := item.Data.(resultRow); ok {
			row.MatchLabel = noteMatchLabel(note, match.terms)
			if match.mode == modeSearch && row.MatchLabel == "" && row.TimeLabel != "" {
				row.MatchLabel = row.TimeLabel
			}
			row.Files = attachmentNames(note.Attachments)
			item.Data = withURL(row, s.icalURL("notes", src, note.UID))
		}
		return item, true
	}
	return fanout.Item{}, false
}

func attachmentNames(list []model.Attachment) []string {
	out := make([]string, 0, len(list))
	for _, att := range list {
		if name := strings.TrimSpace(att.Filename); name != "" {
			out = append(out, name)
		}
	}
	return out
}

func (s *Server) contactURL(src fanout.Source, uid string) string {
	return s.Path("/app/contacts/" + src.AccountID + "/" + EncodeCollectionPath(src.Collection) + "/" + urlPathEscape(uid))
}

func (s *Server) icalURL(section string, src fanout.Source, uid string) string {
	return s.Path("/app/" + section + "/" + src.AccountID + "/" + EncodeCollectionPath(src.Collection) + "/" + urlPathEscape(uid))
}

func searchQuery(match searchContext, terms []string) string {
	if q := strings.TrimSpace(match.query); q != "" {
		return q
	}
	for _, term := range terms {
		if term = strings.TrimSpace(term); term != "" {
			return term
		}
	}
	return ""
}

func withURL(data any, url string) any {
	row, ok := data.(resultRow)
	if !ok {
		return data
	}
	row.URL = url
	return row
}
