package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/fanout"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
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
	from, to := weekWindow(tf, r.FormValue("from"))
	if r.FormValue("from") != "" {
		page += "?from=" + from.Format("2006-01-02")
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
		src := s.openSource(conn.ID+":"+col.Path, conn, col)
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
		found:      func(n int) string { return plural(n, "record") },
	}
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
}

func (s *Server) readCombined(r *http.Request, sess *session.Session, blob sealed.Blob, spec kindSpec) (combinedState, error) {
	tf, now := s.timeFor(sess), time.Now()
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
		st.poll.Summary = combinedNoTicks
	default:
		snap := task.Snapshot()
		st.task, st.snap = task, &snap
		// A poll asked again from a filtered view comes back filtered.
		owner, again := listOwner(task), task.page
		if tag := strings.TrimSpace(r.URL.Query().Get("tag")); tag != "" {
			owner.again, again = withQuery(owner.again, "tag", tag), withQuery(again, "tag", tag)
		}
		*st.poll = s.pollBar(tf, blob, snap, owner)
		s.indexAnswered(sess, blob, spec, snap)
		st.items = listedOf(blob, snap.Items)
		refresh := view.Action{ID: "poll.again", Label: "Refresh", Href: s.route(again), Method: "GET", Enabled: !snap.Running}
		if snap.Running {
			refresh.Reason = "The poll is still running."
		}
		st.shell.Top.Refresh = view.Refresh{Action: refresh, Running: snap.Running}
		if !snap.Running {
			st.shell.Top.Refresh.Answered = tf.formatWhen(snap.Finished, true, now)
		}
	}
	st.shell.Rail = s.kindRail(sess, blob, spec, st.snap)
	return st, nil
}

// combinedPage is /app/{kind}: the rows of the kind from every answered
// source, merged by the kind's natural key, never grouped by source.
func (s *Server) combinedPage(r *http.Request, sess *session.Session, blob sealed.Blob, spec kindSpec) (any, error) {
	st, err := s.readCombined(r, sess, blob, spec)
	if err != nil {
		return nil, err
	}
	tf, base := s.timeFor(sess), "/app/"+spec.segment
	actions := []view.Action{s.combinedNewAction(blob, spec)}
	// The filter's links name the task, so choosing a category reads the
	// records already polled instead of asking the servers again.
	query := r.URL.Query()
	if st.task != nil {
		query.Set("task", st.task.ID())
	}
	filter := s.tagFilter(base, query)
	shown := filter.keep(spec.kind, st.items)
	st.shell.Rail.Tags = filter.rail(spec.kind, st.items)

	// The print header says how many collections the view shows and, once the
	// poll is over, how many records; nothing ticked has nothing to print.
	var sources string
	var taken view.When
	if st.snap != nil {
		sources = plural(len(st.snap.Sources), collectionNouns[spec.kind])
		taken = tf.taken(time.Now())
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
		page.Groups = s.contactGroups(sess, shown, spec.segment, true)
		total := len(st.items)
		page.Bar = view.ListBar{Loaded: len(shown), Total: &total, Actions: actions}
		page.Print = header(counted(len(shown), "card"))
		return page, nil
	case view.KindEvent:
		page := view.AgendaPage{Shell: st.shell, Poll: st.poll}
		from, to := weekWindow(tf, r.FormValue("from"))
		var bad []unreadable
		page.Days, page.Bar.Loaded, bad = s.agendaDays(tf, shown, from, to, spec.segment, true)
		page.Bar.Range = s.agendaRange(tf, base, filter.tag, from, to)
		page.Bar.Actions = actions
		page.Notices = appendUnreadable(page.Notices, bad)
		page.Print = header(agendaPrint(tf, from, to), counted(page.Bar.Loaded, "event"))
		return page, nil
	case view.KindTask:
		page := view.TasksPage{Shell: st.shell, Poll: st.poll}
		show := taskShow(r)
		var open, done int
		var bad []unreadable
		page.Groups, open, done, page.Bar.Loaded, bad = s.taskGroups(tf, show, shown, spec.segment, true, filter, time.Now())
		page.Bar.Segments = s.taskSegments(base, filter.tag, show, open, done)
		page.Bar.Actions = actions
		page.Notices = appendUnreadable(page.Notices, bad)
		page.Print = header(showLabels[show], counted(page.Bar.Loaded, "task"))
		return page, nil
	default:
		page := view.NotesPage{Shell: st.shell, Poll: st.poll}
		var bad []unreadable
		page.Groups, page.Bar.Loaded, bad = s.noteGroups(tf, shown, spec.segment, true, filter)
		page.Bar.Actions = actions
		page.Notices = appendUnreadable(page.Notices, bad)
		page.Print = header(counted(page.Bar.Loaded, "note"))
		return page, nil
	}
}

// combinedNewAction opens the new-record sheet with nothing chosen: from a
// combined view the target collection is chosen, not guessed (S-14). The
// address carries the first writable collection only because the route needs
// one.
func (s *Server) combinedNewAction(blob sealed.Blob, spec kindSpec) view.Action {
	a := view.Action{ID: "record.new", Label: text("New " + spec.noun), Method: "GET"}
	for _, col := range blob.Collections {
		conn, ok := blob.Connection(col.AccountID)
		if !ok || col.Gone || col.ReadOnly || !col.Has(spec.kind) {
			continue
		}
		a.Href, a.Enabled = s.route(origin{conn: conn, col: col}.listPath(spec.segment)+"/new?pick=1"), true
		return a
	}
	a.Reason = text("No collection of this kind takes new records.")
	return a
}

// listedOf turns the items of a snapshot into rows' input with their origin.
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
		putIndex(sess, account, path, spec.component, indexOf(spec.kind, part))
		if !snap.Finished.IsZero() && fresh[id] {
			noteRead(sess, account, path, snap.Finished, part, spec.kind != view.KindEvent)
		}
	}
}

// collectionCount is the rail's number for one ticked collection (C-6.3): the
// records of the kind the session last read of the whole collection. The
// agenda reads a week, so events have no number, a dash rather than a guess.
func collectionCount(sess *session.Session, accountID string, col sealed.Collection, spec kindSpec) *int {
	if spec.kind == view.KindEvent {
		return nil
	}
	data, ok := sess.Cache().GetETagMap(accountID, col.Path+"\x00"+spec.component)
	if !ok {
		return nil
	}
	var entries []indexEntry
	if json.Unmarshal(data, &entries) != nil {
		return nil
	}
	n := len(entries)
	return &n
}

// kindCount is the number of a section: the sum of its ticked collections,
// and a dash while any of them has none.
func kindCount(sess *session.Session, blob sealed.Blob, spec kindSpec) *int {
	sum := 0
	for _, col := range ticked(blob, spec.kind) {
		if col.Gone {
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

// kindRail is the sections rail of a record kind (C-6.3): the kinds and
// Duplicates, the combined row, the collections of the kind by connection
// with their ticks, and the foot. A gone collection stays while it is ticked,
// marked gone and not asked; one whose server did not answer this poll is
// unavailable — the two are never the same state (S-21).
func (s *Server) kindRail(sess *session.Session, blob sealed.Blob, spec kindSpec, snap *fanout.Snapshot) view.Rail {
	rail := view.Rail{About: s.aboutAction(), Nav: s.navRows(sess, blob, spec.segment)}

	unavailable := map[string]bool{}
	if snap != nil {
		for _, st := range snap.Sources {
			if st.State == view.SourceUnavailable {
				unavailable[st.ID] = true
			}
		}
	}
	sections, live, on := s.railSections(sess, blob, spec, unavailable)
	combined := view.SourceRow{
		Href: s.route("/app/" + spec.segment), Label: text(combinedTitles[spec.kind]),
		Count: kindCount(sess, blob, spec), Tick: view.TickPart, State: view.CollectionOK,
	}
	switch on {
	case 0:
		combined.Tick, combined.Count = view.TickOff, nil
	case live:
		combined.Tick = view.TickOn
	}
	rail.Sections = append([]view.RailSection{{State: view.CollectionOK, Rows: []view.SourceRow{combined}}}, sections...)
	rail.Foot = []view.Action{
		{ID: "rail.apply", Label: "Apply", Href: s.route("/app/sources/" + spec.segment), Method: "POST", Enabled: true},
		{ID: "collection.new", Label: "New collection", Icon: "i-plus", Href: s.postQuery("/app/collections/new", url.Values{"kind": {string(spec.kind)}}), Method: "GET", Enabled: true},
		{ID: "account.connect", Label: "Connect account", Icon: "i-server", Href: s.route("/app/settings/connections"), Method: "GET", Enabled: true},
	}
	rail.Note = text(fmt.Sprintf("%d of %s ticked", on, plural(live, "collection")))
	return rail
}

// navRows are the sections of the rail, the open one marked.
func (s *Server) navRows(sess *session.Session, blob sealed.Blob, open string) []view.NavRow {
	var nav []view.NavRow
	for _, seg := range []string{"contacts", "calendar", "tasks", "notes"} {
		nav = append(nav, view.NavRow{
			Action: view.Action{ID: "nav." + seg, Label: text(kindTitle(seg)), Href: s.route("/app/" + seg), Method: "GET", Enabled: true},
			Count:  kindCount(sess, blob, recordKinds[seg]),
			On:     seg == open,
		})
	}
	return append(nav,
		view.NavRow{Action: view.Action{ID: "nav.files", Label: "Files", Href: s.route("/app/files"), Method: "GET", Enabled: true}, On: open == filesSpec.segment},
		view.NavRow{Action: view.Action{ID: "nav.duplicates", Label: "Duplicates", Href: s.route("/app/duplicates"), Method: "GET", Enabled: true}},
	)
}

// backRail is the back rail of a record screen (C-6.3): one row out of it and
// the same collections as the list, without the combined row, the tags and
// the foot — what to look at is already chosen here.
func (s *Server) backRail(sess *session.Session, blob sealed.Blob, spec kindSpec, back view.Action) view.Rail {
	sections, _, _ := s.railSections(sess, blob, spec, nil)
	return view.Rail{Nav: []view.NavRow{{Action: back}}, Sections: sections, About: s.aboutAction()}
}

// railSections are the collections of a kind by connection with their ticks
// and counts; live counts the rows that can be opened, on the ticked ones
// among them.
func (s *Server) railSections(sess *session.Session, blob sealed.Blob, spec kindSpec, unavailable map[string]bool) (sections []view.RailSection, live, on int) {
	for _, conn := range blob.Connections {
		sec := view.RailSection{Account: text(serverHost(conn.Server)), AccountID: conn.ID, State: view.CollectionOK}
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
			case unavailable[conn.ID+":"+col.Path]:
				row.State = view.CollectionUnavailable
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
					ID: "collection.rename", Label: "Rename…",
					Href:   s.postQuery("/app/collections/rename", url.Values{"a": {conn.ID}, "c": {col.Path}}),
					Method: "GET", Enabled: true,
				}}}
				if item, ok := s.collectionMoveItem(blob, conn, col); ok {
					row.Menu = append(row.Menu, item)
				}
				// The wizard and the file it hands over are one click from the
				// rail as well as from the list (S-19). A read-only collection
				// is handed over and not written into.
				list := origin{conn: conn, col: col}.listPath(spec.segment)
				if !col.ReadOnly {
					row.Menu = append(row.Menu, view.MenuItem{Separator: true, Action: view.Action{
						ID: "collection.import", Label: "Import…", Icon: "i-up",
						Href: s.route(list + "/import"), Method: "GET", Enabled: true}})
				}
				row.Menu = append(row.Menu, view.MenuItem{Separator: col.ReadOnly, Action: view.Action{
					ID: "collection.export", Label: "Export", Icon: "i-down",
					Href: s.route(list + "/export"), Method: "GET", Enabled: true}})
				if item, ok := s.collectionDeleteItem(conn, col); ok {
					row.Menu = append(row.Menu, item)
				}
				if tick {
					on++
					row.Count = collectionCount(sess, conn.ID, col, spec)
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
	return sections, live, on
}

// postSources writes the ticks of one kind (S-14). The rail is a form, and it
// sends the whole selection of that kind: the href of every ticked row, as
// the rail gave it. Other kinds keep their ticks. The answer is the rail and
// the poll of the new selection (C-7.3); a plain form goes back to the
// combined view.
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
	err := s.users.SetSelection(sess.AccountID(), key, spec.kind, s.tickPicks(spec, r.PostForm["tick"]))
	if errors.Is(err, user.ErrNotFound) {
		s.notFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "could not save the selection", http.StatusInternalServerError)
		return
	}
	if !WantsFragment(r) {
		s.redirect(w, r, "/app/"+spec.segment)
		return
	}
	if spec.kind == view.KindFile {
		_ = s.render.Fragment(w, r, http.StatusOK, TargetRail, s.filesRail(sess, s.catalogOf(sess)), nil)
		return
	}
	st, err := s.readCombined(r, sess, s.catalogOf(sess), spec)
	if err != nil {
		http.Error(w, "could not start the list", http.StatusInternalServerError)
		return
	}
	_ = s.render.Fragment(w, r, http.StatusOK, TargetRail, st.shell.Rail, []OOB{{Target: TargetPoll, Body: *st.poll}})
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
