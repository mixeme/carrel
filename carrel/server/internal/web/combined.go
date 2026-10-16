package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/fanout"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/publish"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// The combined view (S-14): the ticked collections of one kind in one list,
// the view model of a single collection plus the poll bar and the origin on
// every row. The ticks live in the sealed blob, one set per kind, so they
// survive a restart; nothing ticked is an empty screen, not "all". Its cost is
// a fanout of the session (S-15).

var combinedTitles = map[view.Kind]string{
	view.KindContact: "Combined address book",
	view.KindEvent:   "Combined calendar",
	view.KindTask:    "Combined list",
	view.KindNote:    "Combined notebook",
}

const combinedNoTicks = "No collections are ticked, so there is nothing to show. Tick collections in the rail."

// listTask is the fanout behind a combined view. page is the address it
// answers, the agenda week included; key names its sources and window, so a
// reload while it runs reads the same task instead of asking again.
type listTask struct {
	*fanout.Task
	kind view.Kind
	page string
	key  string
}

// ticked returns the catalog entries ticked for kind k, in the order of the
// ticks, gone ones included. A tick whose entry or connection the catalog no
// longer holds, or whose entry does not hold k, is skipped.
func ticked(blob sealed.Blob, k view.Kind) []sealed.Collection {
	var out []sealed.Collection
	for _, pick := range blob.Selected {
		if pick.Kind != k {
			continue
		}
		col, hasCol := blob.Collection(pick.AccountID, pick.Path)
		if _, hasConn := blob.Connection(pick.AccountID); hasConn && hasCol && col.Has(k) {
			out = append(out, col)
		}
	}
	return out
}

func isTicked(blob sealed.Blob, k view.Kind, accountID, path string) bool {
	for _, pick := range blob.Selected {
		if pick.Kind == k && pick.AccountID == accountID && pick.Path == path {
			return true
		}
	}
	return false
}

func (s *Server) getKindRoot(w http.ResponseWriter, r *http.Request, segment string) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	spec, known := recordKinds[segment]
	if !known {
		s.notFound(w, r)
		return
	}
	if spec.kind == view.KindContact || spec.kind == view.KindEvent {
		if s.compactChoice(w, r, segment, "/app/"+segment) {
			return
		}
	}
	// ?open= of every list: the panel's row is no query the list keeps.
	r = withoutOpen(r)
	page, err := s.combinedPage(r, sess, s.catalogOf(sess), spec)
	if err != nil {
		http.Error(w, "could not start the list", http.StatusInternalServerError)
		return
	}
	s.writePage(w, r, http.StatusOK, page)
}

// combinedQuery is what every source is asked, and the page it answers: the
// agenda reads one week, the other kinds the whole collection.
func (s *Server) combinedQuery(r *http.Request, tf timeFmt, spec kindSpec) (fanout.Query, string) {
	page := "/app/" + spec.segment
	if spec.kind != view.KindEvent {
		return fanout.Query{}, page
	}
	kind, from, to := agendaWindow(tf, r.URL.Query())
	// The window of the screen is the window asked of the servers: a month
	// shown is a month read (review Р-12).
	q := url.Values{}
	if kind != agendaWeek {
		q.Set("view", kind)
	}
	if r.URL.Query().Get("from") != "" || kind != agendaWeek {
		q.Set("from", from.Format("2006-01-02"))
	}
	if kind == agendaRange {
		q.Set("to", to.AddDate(0, 0, -1).Format("2006-01-02"))
	}
	if len(q) > 0 {
		page += "?" + q.Encode()
	}
	return fanout.Query{From: &from, To: &to}, page
}

// combinedTask is the task of a combined view: the one still running for the
// same sources and window, the one named by ?task= (the page a Stop or a Retry
// returns to), or a new one, which stops the running list of this kind.
// Nothing ticked: no task. No slot: fanout.ErrBusy.
func (s *Server) combinedTask(r *http.Request, sess *session.Session, blob sealed.Blob, spec kindSpec) (*listTask, error) {
	q, page := s.combinedQuery(r, s.timeFor(sess), spec)
	var sources []fanout.Source
	var ids []string
	for _, col := range ticked(blob, spec.kind) {
		if col.Gone {
			continue // not asked (S-6.1): its rail row says gone
		}
		conn, _ := blob.Connection(col.AccountID)
		src := s.openSource(sess, conn.ID+":"+col.Path, conn, col)
		src.kinds = []view.Kind{spec.kind}
		sources = append(sources, src)
		ids = append(ids, src.id)
	}
	if len(sources) == 0 {
		return nil, nil
	}
	key := strings.Join(ids, "\n") + "\x00" + page
	asked := r.FormValue("task")
	var running []*listTask
	for _, t := range sess.Tasks() {
		lt, ok := t.(*listTask)
		if !ok || lt.kind != spec.kind {
			continue
		}
		if lt.key == key && (lt.Running() || lt.ID() == asked) {
			return lt, nil
		}
		if lt.Running() {
			running = append(running, lt)
		}
	}
	for _, lt := range running {
		lt.Cancel()
	}
	id, err := user.NewTaskID()
	if err != nil {
		return nil, err
	}
	if r.FormValue("again") != "" {
		dropAllWholeReads(sess, blob) // Refresh: nothing kept stands in for asking (S-4.2)
	}
	opts := s.searchOptions(sess)
	opts.Finished = nil // a list is not a search: no history row
	ft, err := fanout.Start(id, q, sources, opts)
	if err != nil {
		return nil, err
	}
	lt := &listTask{Task: ft, kind: spec.kind, page: page, key: key}
	sess.KeepTask(lt)
	return lt, nil
}

func listOwner(lt *listTask) pollOwner {
	return pollOwner{
		page:       withQuery(lt.page, "task", lt.ID()),
		again:      lt.page,
		againLabel: "Refresh",
		what:       "list",
		polled:     true,
		found:      func(n int) string { return plural(n, recordNouns[lt.kind]) },
		// And says how long the poll has run, in every section (the owner's
		// decision of 07.10.2026, Р-39: states-3, -8, -13, -19, -26).
		timed:   true,
		missing: func(n int) string { return missingLine(lt.kind, n) },
	}
}

// missingLine is what a combined list says under its rows when sources did
// not answer (states-6, states-9): what arrived is shown, and what is not
// there is unseen, not absent.
func missingLine(k view.Kind, n int) string {
	what := map[view.Kind][2]string{view.KindContact: {"Cards", "book"}, view.KindEvent: {"Records", "source"},
		view.KindTask: {"Tasks", "list"}, view.KindNote: {"Notes", "notebook"}}[k]
	from := "the unavailable " + what[1]
	if n > 1 {
		from += "s"
	}
	return "What arrived in time is shown. " + what[0] + " from " + from + " are missing — not “there are none”, but “we have not seen them”."
}

func withQuery(path, name, value string) string {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return path + sep + name + "=" + url.QueryEscape(value)
}

// combinedState is a combined view read once: the task, its snapshot, the
// poll bar and the shell around them.
type combinedState struct {
	task  *listTask
	snap  *fanout.Snapshot
	poll  *view.PollBar
	shell view.Shell
	items []listed
	// seen is how many records of the kind the view shows: its rows, a
	// linked record one of them; of a calendar, the events of the window.
	// The poll's bar and the line under the title count it (the owner's
	// decision of 09.10.2026: the table counts what a person sees, the rail
	// what lies in a collection). nil — nothing is asked.
	seen *int
}

// counted is seen once the poll is over, nil while it runs: the line under
// the title counts nothing before (states-8, -13, -19, -26).
func (st combinedState) counted() *int {
	if st.snap == nil || st.snap.Running {
		return nil
	}
	return st.seen
}

// seenRows counts the rows a combined view shows of what a poll brought:
// linked records folded into one, and of a calendar the events of the
// window on the screen.
func (s *Server) seenRows(r *http.Request, tf timeFmt, blob sealed.Blob, spec kindSpec, items []listed) int {
	rows := s.foldLinked(blob, spec.kind, items)
	if spec.kind != view.KindEvent {
		return len(rows)
	}
	_, from, to := agendaWindow(tf, r.URL.Query())
	days, _, _ := s.agendaDays(tf, kindedAll(rows, view.KindEvent), from, to, spec.segment, true, nil)
	return eventsIn(days)
}

func (s *Server) readCombined(r *http.Request, sess *session.Session, blob sealed.Blob, spec kindSpec) (combinedState, error) {
	tf, now := s.timeFor(sess), s.now()
	st := combinedState{shell: s.buildShell(sess, combinedTitles[spec.kind]), poll: &view.PollBar{}}
	task, err := s.combinedTask(r, sess, blob, spec)
	switch {
	case errors.Is(err, fanout.ErrBusy):
		st.shell.Notices = append(st.shell.Notices, view.Notice{
			Role:  view.NoticeAlert,
			Title: "Too many searches",
			Body:  text(fmt.Sprintf("This session already runs %d searches and lists. Stop one or wait for one to finish, then open the list again.", conf.MaxLiveFanouts)),
		})
	case err != nil:
		return st, err
	case task == nil:
		st.poll.Summary = line(combinedNoTicks)
		// The circle stands where every list draws it, and says why it has
		// nothing to ask (the client's scan of the calendar, А2).
		st.shell.Top.Refresh = view.Refresh{Action: view.Action{ID: "poll.again", Label: "Refresh", Icon: "i-refresh",
			Method: "GET", Href: s.route("/app/" + spec.segment), Reason: "Nothing is ticked to ask."}}
	default:
		snap := task.Snapshot()
		st.task, st.snap = task, &snap
		// A poll asked again from a filtered view comes back filtered.
		owner, again := listOwner(task), task.page
		if tag := strings.TrimSpace(r.URL.Query().Get("tag")); tag != "" {
			owner.again, again = withQuery(owner.again, "tag", tag), withQuery(again, "tag", tag)
		}
		// And it reads the servers again (S-4.2): a plain load within the soft
		// limit is answered from the kept reads of a server without a change
		// tag, and Refresh is how a person asks for what is there now — the
		// same again=1 a search and a timeline are run again by.
		owner.again, again = withQuery(owner.again, "again", "1"), withQuery(again, "again", "1")
		owner.query = pageQuery(r.URL.Query())
		st.items = listedOf(blob, snap.Items)
		seen := s.seenRows(r, tf, blob, spec, st.items)
		st.seen, owner.seen = &seen, &seen
		*st.poll = s.pollBar(tf, blob, snap, owner)
		s.indexAnswered(sess, blob, spec, snap)
		refresh := view.Action{ID: "poll.again", Label: "Refresh", Icon: "i-refresh", Href: s.route(again), Method: "GET", Enabled: !snap.Running}
		if snap.Running {
			refresh.Reason = "The poll is still running."
		}
		st.shell.Top.Refresh = view.Refresh{Action: refresh, Running: snap.Running}
		if !snap.Running {
			st.shell.Top.Refresh.Answered = tf.formatWhen(snap.Finished, true, now)
		}
	}
	st.shell.Rail = s.kindRail(sess, blob, spec, st.snap, "/app/"+spec.segment)
	return st, nil
}

// combinedPage is /app/{kind}: the rows of the kind from every answered
// source, merged by the kind's natural key, never grouped by source.
func (s *Server) combinedPage(r *http.Request, sess *session.Session, blob sealed.Blob, spec kindSpec) (any, error) {
	// A list put back in its default order because its column went says so
	// (note 25); the mark leaves the address before anything reads it.
	sorted := unsortedNotice(r, spec.segment)
	st, err := s.readCombined(r, sess, blob, spec)
	if err != nil {
		return nil, err
	}
	if sorted != nil {
		st.shell.Notices = append(st.shell.Notices, *sorted)
	}
	// The form of Columns names the poll too, so that the list it comes back
	// to reads the records already polled.
	columns := func() *view.Columns {
		q := r.URL.Query()
		if st.task != nil {
			q.Set("task", st.task.ID())
		}
		return s.columnsMenu(r, csrfField(sess), spec.segment, "/app/"+spec.segment+"/columns", q, false)
	}
	tf, base := s.timeFor(sess), "/app/"+spec.segment
	actions := append([]view.Action{s.combinedNewAction(blob, spec)}, s.combinedIOActions(blob, spec)...)
	drop := s.combinedDrop(blob, spec) // a file dropped here asks where it goes (Им4; s-io-1)
	// The filter's links name the task, so choosing a category reads the
	// records already polled instead of asking the servers again.
	query := r.URL.Query()
	if st.task != nil {
		query.Set("task", st.task.ID())
	}
	filter := s.tagFilter(base, query)
	shown := s.foldLinked(blob, spec.kind, filter.keep(spec.kind, st.items))
	st.shell.Rail.Tags = filter.rail(spec.kind, st.items)

	// The print header says how many collections the view shows and, once the
	// poll is over, how many records; nothing ticked has nothing to print.
	var sources string
	var taken view.When
	if st.snap != nil {
		sources = plural(len(st.snap.Sources), collectionNouns[spec.kind])
		taken = tf.taken(tf.now)
		if !st.snap.Running {
			taken = tf.taken(st.snap.Finished)
		}
	}
	counted := func(n int, unit string) string {
		if st.snap == nil || st.snap.Running {
			return ""
		}
		return plural(n, unit)
	}
	header := func(parts ...string) view.Print {
		if st.snap == nil {
			return view.Print{Title: st.shell.Title}
		}
		parts = append(append([]string{sources}, parts...), filter.tag)
		return view.Print{Title: st.shell.Title, Range: printRange(parts...), Taken: taken}
	}

	switch spec.kind {
	case view.KindContact:
		page := view.ContactsListPage{Shell: st.shell, Poll: st.poll}
		by := sortOf(r, view.KindContact)
		sift := strings.TrimSpace(r.URL.Query().Get("filter"))
		kept := filterContacts(shown, sift)
		page.Groups = s.contactGroups(sess, tf, kept, spec.segment, true, by, filter)
		countGroups(page.Groups)
		page.Details = s.openInPanel(r, sess, base, kept, page.Groups)
		total := len(st.items)
		page.Bar = view.ListBar{Filter: text(sift), Loaded: len(kept), Total: &total, Actions: append(actions, s.printAction(base)), Drop: drop,
			FilterForm: s.listFilter(base, query, sift), Compact: s.compactForm(r, spec.segment, base),
			Segments: s.sortSegments(base, view.KindContact, by, r.URL.Query()),
			Columns:  columns()}
		page.Subtitle = text(combinedSubtitle(page.Rail, spec, st.counted()))
		switch {
		case st.task == nil && len(page.Notices) == 0:
			// Nothing ticked: no poll, and the list says why (U-6.0).
			page.Poll, page.Empty = nil, s.nothingTicked(spec)
		case st.snap != nil && !st.snap.Running && total == 0:
			page.Empty = &view.Notice{Role: view.NoticeInfo, Title: "The ticked books are empty",
				Body: "Add a contact, or import a file of them.", Actions: []view.Action{}}
			for _, a := range actions {
				if a.ID == "record.new" || a.ID == "collection.import" {
					a.Arrow = nil
					page.Empty.Actions = append(page.Empty.Actions, a)
				}
			}
		}
		page.Print = header(counted(len(shown), "card"))
		return page, nil
	case view.KindEvent:
		page := view.AgendaPage{Shell: st.shell, Poll: st.poll}
		kind, from, to := agendaWindow(tf, r.URL.Query())
		page.Subtitle = agendaSubtitle(tf, combinedSubtitle(page.Rail, spec, nil), from, to, nil)
		var bad []unreadable
		rows := kindedAll(shown, view.KindEvent)
		page.Days, page.Bar.Loaded, bad = s.agendaDays(tf, rows, from, to, spec.segment, true, filter.tags)
		if st.snap != nil && !st.snap.Running {
			// The poll is over: the line counts the events of the window
			// (s-agenda-1; states-13 — while it runs there is no count).
			events := eventsIn(page.Days)
			page.Subtitle = agendaSubtitle(tf, combinedSubtitle(page.Rail, spec, nil), from, to, &events)
		}
		page.Details = s.openInAgenda(r, sess, base, rows, page.Days)
		page.Bar.Range = s.agendaRange(tf, base, filter.tag, kind, from, to, r.URL.Query())
		page.Bar.Segments = []view.Action{}
		page.Bar.Compact = s.compactForm(r, spec.segment, base)
		page.Bar.Columns = columns()
		page.Bar.Actions = append(actions, s.agendaPrintAction(base, query))
		page.Bar.Drop = drop
		page.Notices = appendUnreadable(page.Notices, bad)
		switch {
		case st.task == nil && len(page.Notices) == 0:
			// Nothing ticked: no poll, and the agenda says why (U-6.0) —
			// and where it would look, not when: nothing is shown
			// (states-14).
			page.Poll, page.Empty = nil, s.nothingTicked(spec)
			page.Subtitle = view.Runs{{Text: text(combinedSubtitle(page.Rail, spec, nil))}}
		case st.snap != nil && !st.snap.Running && len(page.Days) == 0 && len(page.Notices) == 0:
			page.Empty = windowEmpty(tf, kind, "the ticked calendars", from, to)
		}
		page.Print = header(agendaPrint(tf, from, to), counted(page.Bar.Loaded, "event"))
		return page, nil
	case view.KindTask:
		page := view.TasksPage{Shell: st.shell, Poll: st.poll}
		show := taskShow(r)
		var open, done int
		var bad []unreadable
		by := sortOf(r, view.KindTask)
		// «Filter by word» first in the bar, as the notes' (s-tasks-1; note
		// 69 of Typecase, task 167).
		sift := strings.TrimSpace(r.URL.Query().Get("filter"))
		kept := filterByWord(shown, sift)
		page.Groups, open, done, page.Bar.Loaded, bad = s.taskGroups(tf, show, kept, spec.segment, true, filter, tf.now, by)
		page.Details = s.openRow(r, sess, base, spec, kept, taskRowsOn(page.Groups))
		page.Bar.Filter, page.Bar.FilterForm = text(sift), s.listFilter(base, query, sift)
		page.Bar.Segments = append(s.taskSegments(base, filter.tag, show, open, done),
			s.sortSegments(base, view.KindTask, by, r.URL.Query())...)
		page.Bar.Actions = append(actions, s.printAction(base))
		page.Bar.Drop = drop
		page.Bar.Columns = columns()
		page.Notices = appendUnreadable(page.Notices, bad)
		page.Subtitle = text(combinedSubtitle(page.Rail, spec, st.counted()))
		if st.task == nil && len(page.Notices) == 0 {
			// Nothing ticked: no poll, and the list says why (U-6.0).
			page.Poll, page.Empty = nil, s.nothingTicked(spec)
		}
		page.Print = header(showLabels[show], counted(page.Bar.Loaded, "task"))
		return page, nil
	default:
		page := view.NotesPage{Shell: st.shell, Poll: st.poll}
		var bad []unreadable
		by := sortOf(r, view.KindNote)
		// «Filter by word» without scripts (s-notes-1).
		sift := strings.TrimSpace(r.URL.Query().Get("filter"))
		kept := filterByWord(shown, sift)
		page.Groups, page.Bar.Loaded, bad = s.noteGroups(tf, kept, spec.segment, true, filter, by)
		page.Details = s.openRow(r, sess, base, spec, kept, noteRowsOn(page.Groups))
		page.Bar.Filter, page.Bar.FilterForm = text(sift), s.listFilter(base, query, sift)
		page.Bar.Segments = s.sortSegments(base, view.KindNote, by, r.URL.Query())
		page.Bar.Actions = actions
		page.Bar.Drop = drop
		page.Bar.Columns = columns()
		page.Notices = appendUnreadable(page.Notices, bad)
		page.Subtitle = text(notebookSubtitle(combinedNotebooks(page.Rail, spec, st.counted())))
		if st.task == nil && len(page.Notices) == 0 {
			// Nothing ticked: no poll, and the notebook says why (U-6.0).
			page.Poll, page.Empty = nil, s.nothingTicked(spec)
		}
		page.Print = header(counted(page.Bar.Loaded, "note"))
		return page, nil
	}
}

// combinedNewAction opens the new-record sheet with nothing chosen: from a
// combined view the target collection is chosen, not guessed (S-14). The
// address carries the first writable collection only because the route needs
// one.
// combinedIOActions are Import and Export of a combined view: the wizard
// asks which collection, because there is more than one (s-contacts-1). The
// import opens on the first writable collection with its "Into" picker; the
// export opens a sheet of the collections to hand over.
func (s *Server) combinedIOActions(blob sealed.Blob, spec kindSpec) []view.Action {
	imp := view.Action{ID: "collection.import", Label: "Import", Icon: "i-import", Method: "GET",
		Href: s.route("/app/" + spec.segment + "/import")}
	exp := view.Action{ID: "collection.export", Label: "Export", Icon: "i-export", Method: "GET",
		Href: s.route("/app/" + spec.segment + "/export")}
	for _, col := range blob.Collections {
		conn, ok := blob.Connection(col.AccountID)
		if !ok || col.Gone || !col.Has(spec.kind) {
			continue
		}
		exp.Enabled = true
		if !col.ReadOnly && !conn.Feed {
			imp.Enabled = true
		}
	}
	if !imp.Enabled {
		imp.Reason = text("No collection of this kind takes records.")
	}
	if !exp.Enabled {
		exp.Reason = text("No collection of this kind to hand over.")
	}
	return []view.Action{imp, exp}
}

// The button writes into the collection last written to, when the view shows
// it; else into the first ticked one that takes records — never into one the
// view does not show (the client's scan of the calendar, Б4: «Дни рождения»,
// unticked, came first in the catalog); with none ticked, the first that
// takes records.
func (s *Server) combinedNewAction(blob sealed.Blob, spec kindSpec) view.Action {
	a := view.Action{ID: "record.new", Label: text("New " + spec.noun), Icon: "i-plus", Method: "GET", Primary: true}
	if o, ok := firstWritable(blob, spec.kind); ok {
		a.Href, a.Enabled = s.route(o.listPath(spec.segment)+"/new?pick=1"), true
		// In a combined view New asks where, by its arrow (s-contacts-9).
		s.whereTo(blob, spec, &a)
		return a
	}
	a.Reason = text("No collection of this kind takes new records.")
	return a
}

// firstWritable is the collection of a kind a new record goes to when
// nothing chose one: the first ticked one that takes records, else the
// first that takes them; ok is false when none does.
func firstWritable(blob sealed.Blob, k view.Kind) (origin, bool) {
	for _, ticks := range []bool{true, false} {
		for _, col := range blob.Collections {
			conn, ok := blob.Connection(col.AccountID)
			if !ok || conn.Feed || col.Gone || col.ReadOnly || !col.Has(k) || (ticks && !isTicked(blob, k, col.AccountID, col.Path)) {
				continue
			}
			return origin{conn: conn, col: col}, true
		}
	}
	return origin{}, false
}

// listedOf turns the items of a snapshot into rows' input with their origin.
// listedOfKind are the answers of one kind in a snapshot.
func listedOfKind(blob sealed.Blob, snap *fanout.Snapshot, k view.Kind) []listed {
	if snap == nil {
		return nil
	}
	var items []fanout.Item
	for _, it := range snap.Items {
		if it.Kind == k {
			items = append(items, it)
		}
	}
	return listedOf(blob, items)
}

func listedOf(blob sealed.Blob, items []fanout.Item) []listed {
	out := make([]listed, 0, len(items))
	for _, it := range items {
		if it.Object == nil {
			continue
		}
		conn, ok := blob.Connection(it.AccountID)
		if !ok {
			conn = sealed.Connection{ID: it.AccountID}
		}
		col, ok := blob.Collection(it.AccountID, it.Col)
		if !ok {
			col = sealed.Collection{AccountID: it.AccountID, Path: it.Col}
		}
		out = append(out, listed{obj: it.Object, from: origin{conn: conn, col: col}})
	}
	return out
}

// indexAnswered keeps what every answered source returned the way a single
// list does: RELATED-TO, hints and the rail's counts read it, and none of
// them asks a server. A finished round is also a read of each collection that
// answered from the network (S-22), at the time the round ended: a page drawn
// later from the same poll does not move it.
func (s *Server) indexAnswered(sess *session.Session, blob sealed.Blob, spec kindSpec, snap fanout.Snapshot) {
	bySource := map[string][]listed{}
	fresh := map[string]bool{}
	for _, st := range snap.Sources {
		if st.State == view.SourceDone || st.State == view.SourceEmpty {
			bySource[st.ID] = []listed{}
			fresh[st.ID] = !st.FromCache
		}
	}
	for _, it := range listedOf(blob, snap.Items) {
		id := it.from.conn.ID + ":" + it.from.col.Path
		if part, ok := bySource[id]; ok {
			bySource[id] = append(part, it)
		}
	}
	for id, part := range bySource {
		account, path, _ := strings.Cut(id, ":")
		putIndex(sess, account, path, spec.component, indexOf(s.timeFor(sess), spec.kind, part))
		if !snap.Finished.IsZero() && fresh[id] {
			noteRead(sess, account, path, snap.Finished, part, spec.kind != view.KindEvent)
		}
	}
	// A refusal is an answer too: the card of the connection says the last
	// one was a refusal, and what it means (Нс23, s-profile-1).
	for _, st := range snap.Sources {
		if st.State == view.SourceError && !snap.Finished.IsZero() {
			account, path, _ := strings.Cut(st.ID, ":")
			sess.NoteReading(account, path, session.Reading{Refused: snap.Finished,
				Unauthorized: passwordRefused(st.Err), Reason: refusalReason(st.Err)})
		}
	}
}

// passwordRefused is whether a server or a feed refused the saved password.
func passwordRefused(err error) bool {
	var fe *publish.StatusError
	return dav.IsStatus(err, http.StatusUnauthorized) || errors.As(err, &fe) && fe.Code == http.StatusUnauthorized
}

// collectionCount is the rail's number for one ticked collection (C-6.3): how
// many records of the kind the whole collection holds, as the session last
// learnt it — from a list read whole, or, for a calendar's events, from the
// count every visit to it takes (putCount). Until 25.09.2026 events had no
// number, because the agenda reads a week; C-6.3 always asked for the
// collection's, and the frames draw it.
func collectionCount(sess *session.Session, accountID string, col sealed.Collection, spec kindSpec) *int {
	if spec.kind == view.KindEvent {
		data, ok := sess.Cache().PeekETagMap(accountID, countKey(col.Path, spec.component))
		if !ok {
			return nil
		}
		n, err := strconv.Atoi(string(data))
		if err != nil {
			return nil
		}
		return &n
	}
	entries, ok := peekIndex(sess, accountID, col.Path, spec.component)
	if !ok {
		return nil
	}
	n := len(entries)
	return &n
}

// peekIndex is the index of a collection's component as the session last
// read it whole; false when it has not.
func peekIndex(sess *session.Session, accountID, path, component string) ([]indexEntry, bool) {
	data, ok := sess.Cache().PeekETagMap(accountID, path+"\x00"+component)
	if !ok {
		return nil, false
	}
	var entries []indexEntry
	if json.Unmarshal(data, &entries) != nil {
		return nil, false
	}
	return entries, true
}

// overdueCount is the number of the section Tasks (Р-47 of Typecase,
// 0bf5f71: «Tasks 2» — the overdue ones, where the other sections say how
// many records they hold): the tasks of the ticked lists that are late and
// not done, as the session last read them; nil while a list is not read.
func overdueCount(sess *session.Session, blob sealed.Blob, tf timeFmt) *int {
	spec := recordKinds[segmentOf(view.KindTask)]
	late := 0
	down := lastPoll(sess, spec.kind)
	for _, col := range ticked(blob, spec.kind) {
		if col.Gone || col.Feed || lastPollRefused(down, col) {
			continue
		}
		entries, ok := peekIndex(sess, col.AccountID, col.Path, spec.component)
		if !ok {
			return nil
		}
		for _, e := range entries {
			if e.At != 0 && overdueBy(tf, cal.Timing{Start: time.Unix(e.At, 0), AllDay: e.Day}, e.Done) != "" {
				late++
			}
		}
	}
	return &late
}

// countKey names the number a collection's component was last counted at.
func countKey(path, component string) string { return "count\x00" + path + "\x00" + component }

// putCount keeps the number a visit to a collection learnt of it.
func putCount(sess *session.Session, accountID, path, component string, n int) {
	if sess == nil {
		return
	}
	sess.Cache().PutETagMap(accountID, countKey(path, component), []byte(strconv.Itoa(n)))
}

// countEvents learns how many events a calendar holds, bodies unread
// (cal.Provider.Count), in the visit that read one week of it. A server that
// cannot count leaves the number as it was: shown once, it does not vanish
// (C-6.3).
func countEvents(ctx context.Context, sess *session.Session, p *cal.Provider, accountID, path string) {
	if sess == nil || p == nil {
		return
	}
	if n, err := p.Count(ctx, path, "VEVENT"); err == nil {
		putCount(sess, accountID, path, "VEVENT", n)
	}
}

// kindCount is the number of a section: the sum of its ticked collections,
// and a dash while any of them has none.
func kindCount(sess *session.Session, blob sealed.Blob, spec kindSpec) *int {
	sum := 0
	down := lastPoll(sess, spec.kind)
	for _, col := range ticked(blob, spec.kind) {
		// A feed is someone else's: its number stands in its own row, and
		// the section's sum is of the person's own collections. One that did not answer the last poll says why
		// in its own row, and the sum is of the ones that did (s-agenda-1:
		// «Общий» refuses, and the calendar still counts 14).
		if col.Gone || col.Feed || lastPollRefused(down, col) {
			continue
		}
		n := collectionCount(sess, col.AccountID, col, spec)
		if n == nil {
			return nil
		}
		sum += *n
	}
	return &sum
}

// lastPoll is the last poll of a kind this session ran — a combined view's —
// or nil. A screen that asks one collection only still shows, in its rail,
// what that poll could not read: the session knows it, and a row that looked
// fine until the combined view was opened again would be a lie.
func lastPoll(sess *session.Session, k view.Kind) *fanout.Snapshot {
	// The session keeps its tasks newest first: the first of the kind is the
	// last poll. Until v0.2.65 the loop kept the oldest instead, and a server
	// that had answered again still read as refusing.
	var last *listTask
	for _, t := range sess.Tasks() {
		if lt, ok := t.(*listTask); ok && lt.kind == k {
			last = lt
			break
		}
	}
	if last == nil {
		return nil
	}
	snap := last.Snapshot()
	return &snap
}

// lastPollRefused reports whether a collection did not answer the poll.
func lastPollRefused(snap *fanout.Snapshot, col sealed.Collection) bool {
	if snap == nil {
		return false
	}
	for _, st := range snap.Sources {
		if st.ID == col.AccountID+":"+col.Path {
			return st.State == view.SourceError || st.State == view.SourceTimeout || st.State == view.SourceUnavailable
		}
	}
	return false
}

// railTrouble is what a rail row says of a collection that did not answer
// this poll: only its mark of an error, the whole sentence its hint (the
// owner's decision of 02.10.2026: the fight with the length of a line is
// lost). Until then the row said a few words beside the name, «password ·
// 401», and offered a Retry of it alone; both are in the menu «N failed» of
// the poll bar now (PollBar.Failed, states-16).
type railTrouble struct {
	detail string
}

// troublesOf are the sources of a poll that did not answer, by source ID:
// refused, timed out, or unreachable. Stopped is not a trouble of the
// collection.
func (s *Server) troublesOf(snap *fanout.Snapshot) map[string]railTrouble {
	out := map[string]railTrouble{}
	if snap == nil {
		return out
	}
	for _, st := range snap.Sources {
		var t railTrouble
		switch st.State {
		case view.SourceError:
			t.detail = refusalDetail(st.Err)
		case view.SourceTimeout:
			t.detail = "The server did not answer in time."
		case view.SourceUnavailable:
			t.detail = "The server could not be reached."
		default:
			continue
		}
		out[st.ID] = t
	}
	return out
}

// refusalDetail is what the rail says of a collection whose server answered
// with a refusal: the reason, and for a password that stopped fitting where
// to mend it.
func refusalDetail(err error) string {
	if dav.IsStatus(err, http.StatusUnauthorized) {
		return "401 Unauthorized — the saved password was refused. Open Connections to update it."
	}
	return refusalReason(err)
}

// kindRail is the sections rail of a record kind (C-6.3): the kinds and
// Duplicates, the combined row, the collections of the kind by connection
// with their ticks, and the foot. A gone collection stays while it is ticked,
// marked gone and not asked; one whose server did not answer this poll is
// unavailable — the two are never the same state (S-21). open is the list
// on the screen, its row marked open, and where a tick comes back to.
func (s *Server) kindRail(sess *session.Session, blob sealed.Blob, spec kindSpec, snap *fanout.Snapshot, open string) view.Rail {
	rail := view.Rail{About: s.aboutAction(), Davloom: s.davloomMark(sess), Nav: s.navRows(sess, blob, spec.segment)}
	polling := snap // the poll of the view on the screen, when it has one
	if snap == nil {
		snap = lastPoll(sess, spec.kind) // one collection's page: what the last poll could not read
	}
	// While the poll of this view runs, a collection that has not answered
	// it yet has no number — a dash —, whatever the session knew of it
	// before, and neither has the row of the combined view: its sum is not
	// known until every one has answered. The sections above keep their
	// numbers (Typecase's note 44; states-8, -13, -19, -26).
	var count func(sourceID string) *int
	if polling != nil && polling.Running {
		waits := map[string]bool{}
		for _, src := range polling.Sources {
			if src.State == view.SourceWaiting || src.State == view.SourceQuerying {
				waits[src.ID] = true
			}
		}
		known := map[string]*int{}
		for _, conn := range blob.Connections {
			for _, col := range blob.CollectionsOf(conn.ID) {
				known[conn.ID+":"+col.Path] = collectionCount(sess, conn.ID, col, spec)
			}
		}
		for _, f := range blob.Subscriptions {
			known[f.ID+":"+f.FeedCollection().Path] = collectionCount(sess, f.ID, f.FeedCollection(), spec)
		}
		count = func(id string) *int {
			if waits[id] {
				return nil
			}
			return known[id]
		}
	}
	sections, live, on := s.railSections(sess, blob, spec, s.troublesOf(snap), count)
	combined := view.SourceRow{
		Href: s.route("/app/" + spec.segment), Label: text(combinedTitles[spec.kind]),
		Count: kindCount(sess, blob, spec), Tick: view.TickPart, State: view.CollectionOK,
	}
	if polling != nil && polling.Running {
		combined.Count = nil
	}
	switch on {
	case 0:
		combined.Tick, combined.Count = view.TickOff, nil
	case live:
		combined.Tick = view.TickOn
	}
	rail.Sections = append([]view.RailSection{{State: view.CollectionOK, Rows: []view.SourceRow{combined}}}, sections...)
	markOpen(rail.Sections, s.route(open))
	rail.Ticks = s.ticksForm(sess, spec.segment, open)
	rail.Foot = []view.Action{
		// The foot names what it makes (s-contacts-1, s-agenda-1, s-tasks-1,
		// s-notes-1): «New calendar», not «New collection».
		{ID: "collection.new", Label: text("New " + newCollectionTitle(spec.kind)), Icon: "i-plus", Href: s.postQuery("/app/collections/new", url.Values{"kind": {string(spec.kind)}}), Method: "GET", Enabled: true},
		s.connectAction(),
		{ID: "feed.subscribe", Label: "Subscribe to a feed", Icon: "i-link", Href: s.postQuery("/app/settings/connections", url.Values{"op": {"feed"}}), Method: "GET", Enabled: true},
	}
	rail.Note = text(fmt.Sprintf("%d of %s ticked", on, plural(live, "collection")))
	return rail
}

// navRows are the sections of the rail, the open one marked.
func (s *Server) navRows(sess *session.Session, blob sealed.Blob, open string) []view.NavRow {
	var nav []view.NavRow
	for _, seg := range []string{"contacts", "calendar", "tasks", "notes"} {
		count := kindCount(sess, blob, recordKinds[seg])
		if recordKinds[seg].kind == view.KindTask {
			count = overdueCount(sess, blob, s.timeFor(sess))
		}
		nav = append(nav, view.NavRow{
			Action: view.Action{ID: "nav." + seg, Label: text(kindTitle(seg)), Icon: kindIcons[recordKinds[seg].kind], Href: s.route("/app/" + seg), Method: "GET", Enabled: true, On: seg == open},
			Count:  count,
		})
	}
	return append(nav,
		view.NavRow{Action: view.Action{ID: "nav.files", Label: "Files", Icon: "i-file", Href: s.route("/app/files"), Method: "GET", Enabled: true, On: open == filesSpec.segment}},
		view.NavRow{
			Action: view.Action{ID: "nav.duplicates", Label: "Duplicates", Icon: "i-dup", Href: s.route("/app/duplicates"), Method: "GET", Enabled: true, On: open == "duplicates"},
			Count:  s.duplicateCount(sess),
		},
	)
}

// backRail is the back rail of a record screen (C-6.3): one row out of it and
// the same collections as the list, without the combined row, the tags and
// the foot — what to look at is already chosen here.
func (s *Server) backRail(sess *session.Session, blob sealed.Blob, spec kindSpec, accountID string, back view.Action) view.Rail {
	sections, _, _ := s.railSections(sess, blob, spec, nil, nil)
	// A way back stays where it is: the collections of the account this
	// record lives in, and no other. Every frame that draws «Back to list»
	// shows exactly one account rubric (s-contacts-3, s-notes-1,
	// s-conflict-1, all of s-io); the whole catalog belongs to the list.
	kept := sections[:0]
	for _, sec := range sections {
		if sec.AccountID == accountID {
			kept = append(kept, sec)
		}
	}
	return view.Rail{Nav: []view.NavRow{{Action: back}}, Sections: kept, About: s.aboutAction(), Davloom: s.davloomMark(sess)}
}

// markOpen marks the row of the collection open on the screen (s-contacts-1:
// the combined row; s-contacts-6: the book of the record).
func markOpen(sections []view.RailSection, href string) {
	for i := range sections {
		for j := range sections[i].Rows {
			sections[i].Rows[j].On = sections[i].Rows[j].Href == href
		}
	}
}

// ticksForm is the rail's form (U-8): every tick is a submit button of it and
// sends toggle with its row's address; back is where the answer leads.
func (s *Server) ticksForm(sess *session.Session, segment, back string) *view.Form {
	f := s.plainForm("/app/sources/"+segment, "POST", csrfField(sess),
		view.FormRow{Name: "back", Kind: view.InputHidden, Value: text(back)})
	return &f
}

// railSections are the collections of a kind by connection with their ticks
// and counts; live counts the rows that can be opened, on the ticked ones
// among them. troubles are the collections that did not answer the poll the
// rail stands beside, with what to say of each. count, when given, is what a
// ticked row counts instead of its records — a timeline counts what it found
// of one person there. A ticked feed has its count as a collection has (the
// frames draw it where a count stands and «ro» beside it — Р-34 of Typecase,
// from-typecase note 35); it is not in the section's sum.
func (s *Server) railSections(sess *session.Session, blob sealed.Blob, spec kindSpec, troubles map[string]railTrouble, count func(sourceID string) *int) (sections []view.RailSection, live, on int) {
	trouble := func(row *view.SourceRow, id string) bool {
		t, ok := troubles[id]
		if ok {
			row.State = view.CollectionUnavailable
			row.Detail = text(t.detail)
		}
		return ok
	}
	counted := func(accountID string, col sealed.Collection) *int {
		if count != nil {
			return count(accountID + ":" + col.Path)
		}
		return collectionCount(sess, accountID, col, spec)
	}
	for _, conn := range blob.Connections {
		sec := view.RailSection{Account: text(connLabel(conn)), AccountID: conn.ID, State: view.CollectionOK}
		down := 0
		for _, col := range blob.CollectionsOf(conn.ID) {
			tick := isTicked(blob, spec.kind, conn.ID, col.Path)
			if !col.Has(spec.kind) || (col.Gone && !tick) {
				continue
			}
			row := view.SourceRow{
				Label: text(collectionLabel(col)), Colour: catalogColour(col),
				Tick: view.TickOff, ReadOnly: col.ReadOnly, State: view.CollectionOK,
			}
			if tick {
				row.Tick = view.TickOn
			}
			switch {
			case col.Gone:
				row.State = view.CollectionGone
			case trouble(&row, conn.ID+":"+col.Path):
				down++
			case col.ReadOnly:
				row.State = view.CollectionReadOnly
			}
			switch {
			case col.Gone:
			case spec.kind == view.KindFile:
				// A file collection opens at its root folder; it has no count and
				// no name of its own to change (S-12 renames records' collections).
				live++
				row.Href = s.route(fileHref(conn.ID, col.Path, "", nil, true))
				if tick {
					on++
				}
			default:
				live++
				row.Href = s.route(origin{conn: conn, col: col}.listPath(spec.segment))
				row.Menu = []view.MenuItem{{Action: view.Action{
					ID: "collection.rename", Label: "Rename", Icon: "i-pencil",
					Href:   s.postQuery("/app/collections/rename", url.Values{"a": {conn.ID}, "c": {col.Path}}),
					Method: "GET", Enabled: true,
				}}}
				row.Menu = append(row.Menu, s.collectionMoveItems(blob, conn, col, "")...)
				// The wizard and the file it hands over are one click from the
				// rail as well as from the list (S-19). A read-only collection
				// is handed over and not written into.
				list := origin{conn: conn, col: col}.listPath(spec.segment)
				if !col.ReadOnly {
					row.Menu = append(row.Menu, view.MenuItem{Separator: true, Action: view.Action{
						ID: "collection.import", Label: "Import", Icon: "i-import",
						Href: s.route(list + "/import"), Method: "GET", Enabled: true}})
				}
				row.Menu = append(row.Menu, view.MenuItem{Separator: col.ReadOnly, Action: view.Action{
					ID: "collection.export", Label: "Export", Icon: "i-export",
					Href: s.route(list + "/export"), Method: "GET", Enabled: true}})
				if item, ok := s.collectionDeleteItem(conn, col, ""); ok {
					row.Menu = append(row.Menu, item)
				}
				if tick {
					on++
					row.Count = counted(conn.ID, col)
				}
			}
			sec.Rows = append(sec.Rows, row)
		}
		if len(sec.Rows) == 0 {
			continue
		}
		if down == len(sec.Rows) {
			sec.State = view.CollectionUnavailable
		}
		sections = append(sections, sec)
	}
	// Subscriptions last, as a section of their own: someone else's, read
	// only, renamed here and nowhere else (S-25.2).
	feeds := view.RailSection{Account: "Subscriptions", State: view.CollectionOK, Feeds: true}
	for _, f := range blob.Subscriptions {
		col, conn := f.FeedCollection(), f.FeedConnection()
		if !col.Has(spec.kind) {
			continue
		}
		tick := isTicked(blob, spec.kind, f.ID, col.Path)
		row := view.SourceRow{
			Label: text(collectionLabel(col)), Colour: catalogColour(col), Tick: view.TickOff, ReadOnly: true, Feed: true,
			State: view.CollectionReadOnly, Href: s.route(origin{conn: conn, col: col}.listPath(spec.segment)),
		}
		if spec.kind == view.KindFile {
			row.Href = s.route(fileHref(f.ID, col.Path, "", nil, true))
		}
		trouble(&row, f.ID+":"+col.Path)
		row.Menu = []view.MenuItem{
			{Action: view.Action{ID: "feed.edit", Label: "Rename", Href: s.postQuery("/app/collections/rename", url.Values{"a": {f.ID}, "c": {col.Path}}), Method: "GET", Enabled: true}},
		}
		if spec.kind != view.KindFile {
			row.Menu = append(row.Menu, view.MenuItem{Action: view.Action{ID: "collection.export", Label: "Export", Icon: "i-export", Href: s.route(origin{conn: conn, col: col}.listPath(spec.segment) + "/export"), Method: "GET", Enabled: true}})
		}
		live++
		if tick {
			row.Tick = view.TickOn
			on++
			// A feed's count stands where a count stands, «ro» beside it
			// (C-6.3, s-agenda-1, s-contacts-7: «0» and «ro»); it is not in
			// the section's sum (kindCount, timelineRail).
			row.Count = counted(f.ID, col)
		}
		feeds.Rows = append(feeds.Rows, row)
	}
	if len(feeds.Rows) > 0 {
		sections = append(sections, feeds)
	}
	return sections, live, on
}

// postSources writes the ticks of one kind (S-14). The rail is a form
// (Rail.Ticks), and a tick is its submit button: toggle names one row by its
// href, and that collection is flipped — the combined row ticks every one,
// or, all ticked, takes them all off (U-8). Without toggle the form sends the
// whole selection of that kind: the href of every ticked row, as the rail
// gave it. Other kinds keep their ticks. The answer is the rail and the poll
// of the new selection (C-7.3); a plain form goes back where back says — a
// page of this Carrel — or to the combined view.
func (s *Server) postSources(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	spec, known := recordKinds[r.PathValue("kind")]
	if r.PathValue("kind") == filesSpec.segment {
		spec, known = filesSpec, true // the ticks of the files root (S-17.2)
	}
	key, live := sess.DataKey()
	if !known || !live || r.ParseForm() != nil {
		s.notFound(w, r)
		return
	}
	picks := s.tickPicks(spec, r.PostForm["tick"])
	if v := r.FormValue("toggle"); v != "" {
		picks = s.toggled(s.catalogOf(sess), spec, v)
	}
	err := s.users.SetSelection(sess.AccountID(), key, spec.kind, picks)
	if errors.Is(err, user.ErrNotFound) {
		s.notFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "could not save the selection", http.StatusInternalServerError)
		return
	}
	if back, ok := timelineReturn(r.FormValue("back")); ok {
		// Apply from a timeline's rail: the same ticks, and back to the
		// timeline, which asks its sources again.
		s.redirect(w, r, back+"?again=1")
		return
	}
	if !render.WantsFragment(r) {
		if back, ok := pageReturn(r.FormValue("back"), spec.segment); ok {
			s.redirect(w, r, back)
			return
		}
		s.redirect(w, r, "/app/"+spec.segment)
		return
	}
	if spec.kind == view.KindFile {
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetRail, s.filesRail(sess, s.catalogOf(sess), nil), nil)
		return
	}
	st, err := s.readCombined(r, sess, s.catalogOf(sess), spec)
	if err != nil {
		http.Error(w, "could not start the list", http.StatusInternalServerError)
		return
	}
	_ = s.render.Fragment(w, r, http.StatusOK, render.TargetRail, st.shell.Rail, []render.OOB{{Target: render.TargetPoll, Body: *st.poll}})
}

// timelineReturn is the way back an Apply may take: only to a person's
// timeline in this Carrel — a path, no host, no query — never anywhere a
// posted field names.
func timelineReturn(v string) (string, bool) {
	u, err := url.Parse(v)
	if v == "" || err != nil || u.Scheme != "" || u.Host != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	// The escaped path: a collection's path sits in one segment, its slashes
	// encoded, and a decoded one would lead somewhere else.
	p := u.EscapedPath()
	if !strings.HasPrefix(p, "/app/contacts/") || !strings.HasSuffix(p, "/timeline") || strings.Contains(p, "..") || strings.Contains(p, "//") {
		return "", false
	}
	return p, true
}

// pageReturn is the way back a tick of the rail may take without scripts: a
// page of this Carrel in the section of the ticks — a path under
// /app/{segment}, no host, no query — never anywhere a posted field names.
func pageReturn(v, segment string) (string, bool) {
	u, err := url.Parse(v)
	if v == "" || err != nil || u.Scheme != "" || u.Host != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	p := u.EscapedPath()
	if (p != "/app/"+segment && !strings.HasPrefix(p, "/app/"+segment+"/")) || strings.Contains(p, "..") || strings.Contains(p, "//") {
		return "", false
	}
	return p, true
}

// toggled is the selection of a kind with the row href names flipped. The
// combined row (the kind's root) ticks every live collection of the kind,
// or, when every one is ticked, none.
func (s *Server) toggled(blob sealed.Blob, spec kindSpec, href string) []sealed.Selection {
	var now []sealed.Selection
	for _, p := range blob.Selected {
		if p.Kind == spec.kind {
			now = append(now, p)
		}
	}
	h, _, _ := strings.Cut(href, "?")
	if strings.TrimSuffix(h, "/") == s.route("/app/"+spec.segment) {
		var all []sealed.Selection
		for _, col := range blob.Collections {
			if col.Gone || !col.Has(spec.kind) {
				continue
			}
			if _, ok := blob.Connection(col.AccountID); ok {
				all = append(all, sealed.Selection{Kind: spec.kind, AccountID: col.AccountID, Path: col.Path})
			}
		}
		for _, f := range blob.Subscriptions {
			if col := f.FeedCollection(); col.Has(spec.kind) {
				all = append(all, sealed.Selection{Kind: spec.kind, AccountID: f.ID, Path: col.Path})
			}
		}
		for _, p := range all {
			if !isTicked(blob, spec.kind, p.AccountID, p.Path) {
				return all
			}
		}
		return nil
	}
	one := s.tickPicks(spec, []string{href})
	if len(one) != 1 {
		return now
	}
	out := now[:0:0]
	flipped := false
	for _, p := range now {
		if p.AccountID == one[0].AccountID && p.Path == one[0].Path {
			flipped = true
			continue
		}
		out = append(out, p)
	}
	if !flipped {
		out = append(out, one[0])
	}
	return out
}

// tickPicks reads ticked row hrefs back into catalog addresses. Whatever is
// not a collection route of this kind is not a tick; whether the catalog holds
// the collection is checked where the ticks are written.
func (s *Server) tickPicks(spec kindSpec, hrefs []string) []sealed.Selection {
	prefix := s.route("/app/" + spec.segment + "/")
	var out []sealed.Selection
	for _, h := range hrefs {
		h, _, _ = strings.Cut(h, "?")
		rest, ok := strings.CutPrefix(h, prefix)
		if !ok {
			continue
		}
		rest = strings.TrimSuffix(rest, "/") // a file collection's root ends in "/"
		account, seg, ok := strings.Cut(rest, "/")
		if !ok || account == "" || seg == "" || strings.Contains(seg, "/") {
			continue
		}
		path, err := url.PathUnescape(seg)
		if err != nil {
			continue
		}
		out = append(out, sealed.Selection{Kind: spec.kind, AccountID: account, Path: path})
	}
	return out
}
