package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/fanout"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/internal/provider/file"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// Search (S-15). One query is one task in the session; its sources are the
// ticked collections (Selected); the page reads the task and never waits for
// it. A query already in the session opens from memory.

const (
	searchFilesNote = "Files are searched by name only. Carrel does not look inside files, so “nothing found” does not mean the files do not contain it."
	searchNoTicks   = "No collections are ticked, so there is nothing to search. Tick collections in the rail of Contacts, Calendar, Tasks, Notes or Files."
	// sseKeepAlive is how often a waiting stream says it is alive, so a proxy
	// does not close it as idle.
	sseKeepAlive = 15 * time.Second
	jumpRowsMost = 8
)

func (s *Server) getSearch(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	page := view.SearchPage{Shell: s.buildShell(sess, "Search")}
	// The rail without numbers until there is a search to count them from;
	// the page with one gets its rail from it below.
	page.Rail = s.searchRail(sess, s.catalogOf(sess), nil)
	if q == "" {
		// An empty query searches nothing and is not written (S-15.3).
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	page.Title = text(q + " · Search")
	page.Query = text(q)
	page.Top.Search.Query = text(q)
	key, _ := sess.DataKey()
	blob, _ := s.users.Catalog(sess.AccountID(), key)
	page.Top.Search.Jump = s.jumpRows(sess, blob, q)

	var task *fanout.Task
	if r.URL.Query().Get("again") == "" {
		task = keptSearch(sess, q)
	} else {
		dropAllWholeReads(sess, blob) // what the servers hold now (S-4.2)
	}
	if task == nil {
		sources := s.searchSources(sess, blob)
		if len(sources) == 0 {
			page.Poll.Summary = line(searchNoTicks)
			s.writePage(w, r, http.StatusOK, page)
			return
		}
		stopSearches(sess)
		id, err := user.NewTaskID()
		if err != nil {
			http.Error(w, "could not start the search", http.StatusInternalServerError)
			return
		}
		task, err = fanout.Start(id, fanout.Query{Text: q}, sources, s.searchOptions(sess))
		if err != nil {
			page.Notices = append(page.Notices, view.Notice{
				Role:  view.NoticeAlert,
				Title: "Too many searches",
				Body:  text(fmt.Sprintf("This session already runs %d searches and lists. Stop one or wait for one to finish, then search again.", conf.MaxLiveFanouts)),
			})
			s.writePage(w, r, http.StatusOK, page)
			return
		}
		sess.KeepTask(task)
	}
	snap := task.Snapshot()
	s.fillResults(&page, sess, blob, snap, r)
	page.Rail = s.searchRail(sess, blob, &snap)
	s.writePage(w, r, http.StatusOK, page)
}

// keptSearch is the newest task of this session with the same query: the
// result opens from memory and no server is asked (S-15.3).
func keptSearch(sess *session.Session, q string) *fanout.Task {
	for _, t := range sess.Tasks() {
		if ft, ok := t.(*fanout.Task); ok && ft.Query().Text == q {
			return ft
		}
	}
	return nil
}

// stopSearches cancels the searches still running: a new query stops the one
// before it (S-15.1). What they found stays, marked.
func stopSearches(sess *session.Session) {
	for _, t := range sess.Tasks() {
		if ft, ok := t.(*fanout.Task); ok && ft.Running() && ft.Query().Text != "" {
			ft.Cancel()
		}
	}
}

func (s *Server) searchOptions(sess *session.Session) fanout.Options {
	return fanout.Options{
		Parallelism:   conf.FanoutParallelism,
		SourceTimeout: s.cfg.ProgressSourceTimeout,
		TotalTimeout:  s.cfg.ProgressTotalTimeout,
		Key:           searchKey(s.tz()),
		Acquire:       sess.AcquireFanout,
		Release:       sess.ReleaseFanout,
		Finished:      func(snap fanout.Snapshot) { sess.AddHistory(historyEntry(snap)) },
		Now:           s.now,
	}
}

func historyEntry(snap fanout.Snapshot) session.HistoryEntry {
	e := session.HistoryEntry{Query: snap.Query.Text, Hits: len(snap.Items), At: snap.Finished}
	for _, st := range snap.Sources {
		if st.State == view.SourceDone || st.State == view.SourceEmpty {
			e.None = e.Hits == 0
			break
		}
	}
	return e
}

func historyOutcome(e session.HistoryEntry) string {
	switch {
	case e.Hits > 0:
		return fmt.Sprintf("%d found", e.Hits)
	case e.None:
		return "nothing found"
	default:
		return "no source answered"
	}
}

func searchHref(q string) string {
	return "/app/search?q=" + url.QueryEscape(q)
}

// fillResults is the page over a snapshot. The segment and the filter cut
// what has already arrived; neither asks a server again (S-15.2).
func (s *Server) fillResults(page *view.SearchPage, sess *session.Session, blob sealed.Blob, snap fanout.Snapshot, r *http.Request) {
	tf := s.timeFor(sess)
	q := snap.Query.Text
	kind := view.Kind(r.URL.Query().Get("kind"))
	filter := strings.TrimSpace(r.URL.Query().Get("filter"))
	needle := strings.ToLower(filter)
	for _, it := range snap.Items {
		if kind != "" && it.Kind != kind {
			continue
		}
		row := s.findRow(tf, blob, it)
		if needle != "" && !strings.Contains(strings.ToLower(string(row.Title)+"\n"+string(row.Detail)), needle) {
			continue
		}
		page.Results = append(page.Results, row)
	}
	total := len(snap.Items)
	page.Bar = view.ListBar{Filter: text(filter), Loaded: len(page.Results), Total: &total}
	segment := func(k view.Kind, label string) view.Action {
		href := searchHref(q)
		if k != "" {
			href += "&kind=" + url.QueryEscape(string(k))
		}
		if filter != "" {
			href += "&filter=" + url.QueryEscape(filter)
		}
		// s-find-1 writes the kinds without counts; the one shown now is
		// on and stays a choice (C-6.2).
		return view.Action{ID: "search.kind", Label: text(label), Href: s.route(href), Method: "GET", Enabled: true, On: k == kind}
	}
	page.Bar.Segments = []view.Action{
		segment("", "All"),
		segment(view.KindContact, "Contacts"),
		segment(view.KindEvent, "Events"),
		segment(view.KindTask, "Tasks"),
		segment(view.KindNote, "Notes"),
		segment(view.KindFile, "Files"),
	}
	page.Poll = s.pollBar(tf, blob, snap, searchOwner(q))
	for _, st := range snap.Sources {
		account, path, _ := strings.Cut(st.ID, ":")
		if col, ok := blob.Collection(account, path); ok && col.Has(view.KindFile) {
			page.Note = searchFilesNote
			break
		}
	}
}

// findRow is one result in the look of its kind, with its origin (C-10).
func (s *Server) findRow(tf timeFmt, blob sealed.Blob, it fanout.Item) view.FindRow {
	st, colour := sourceOrigin(blob, it.AccountID, it.Col)
	row := view.FindRow{Kind: it.Kind, Colour: colour, Stamp: st}
	o := it.Object
	switch it.Kind {
	case view.KindFile:
		row.Title = text(it.FileName)
		row.Detail = text(string(formatSize(it.FileSize)))
		row.Href = s.route(foundFileHref(it))
		return row
	case view.KindContact:
		row.Title = text(contactName(o))
		detail := firstValue(o, "EMAIL")
		if detail == "" {
			detail = firstValue(o, "TEL")
		}
		row.Detail = text(detail)
	case view.KindEvent:
		row.Title = text(titleOr(cal.Text(o, "SUMMARY"), "Untitled event"))
		row.Detail = text(cal.Text(o, "LOCATION"))
		if tm, err := cal.TimingOf(o, tf.tz); err == nil && !tm.Start.IsZero() {
			row.When = text(tf.shortDue(tm))
		}
	case view.KindTask:
		row.Title = text(titleOr(cal.Text(o, "SUMMARY"), "Untitled task"))
		if taskDone(o) {
			row.Detail = "Done"
		}
		if due, err := cal.DueOf(o, tf.tz); err == nil && !due.Start.IsZero() {
			row.When = text(tf.shortDue(due))
		}
	case view.KindNote:
		body := cal.Description(o)
		title := noteTitle(cal.Text(o, "SUMMARY"), body)
		row.Title, row.Detail = text(title), text(noteExcerpt(body, title))
		if at := noteTime(o, tf.tz); !at.IsZero() {
			row.When = text(tf.shortDay(at))
		}
	}
	row.Href = s.route("/app/" + segmentOf(it.Kind) + "/" + it.AccountID + "/" + url.PathEscape(it.Col) + "/" + url.PathEscape(it.UID))
	return row
}

// foundFileHref is the folder route of a found file: its folder with the file
// selected (S-17).
func foundFileHref(it fanout.Item) string {
	rel := strings.TrimPrefix(it.Path, strings.TrimSuffix(it.Col, "/")+"/")
	return fileHref(it.AccountID, it.Col, "", strings.Split(rel, "/"), false)
}

// searchKey orders the result by kind — contacts, events, tasks, notes, files
// — and inside a kind by its natural key: name, time, newest note first,
// file name (S-14.1).
func searchKey(tz *time.Location) func(fanout.Item) string {
	return func(it fanout.Item) string {
		var key string
		switch it.Kind {
		case view.KindContact:
			key = "0" + strings.ToLower(contactName(it.Object))
		case view.KindEvent:
			tm, _ := cal.TimingOf(it.Object, tz)
			key = "1" + timeKey(tm.Start)
		case view.KindTask:
			due, _ := cal.DueOf(it.Object, tz)
			key = "2" + timeKey(due.Start)
		case view.KindNote:
			key = "3" + newestFirst(noteTime(it.Object, tz))
		default:
			key = "4" + strings.ToLower(it.FileName)
		}
		return key + "\x00" + it.AccountID + "\x00" + it.Path
	}
}

func timeKey(t time.Time) string {
	if t.IsZero() {
		return "~"
	}
	return t.UTC().Format("20060102T150405")
}

func newestFirst(t time.Time) string {
	if t.IsZero() {
		return "~"
	}
	u := t.Unix()
	if u < 0 {
		u = 0
	}
	return fmt.Sprintf("%019d", math.MaxInt64-u)
}

// pollOwner is what a poll bar says around its sources, which differs between
// a search and a combined list (S-14): the page the task answers, where asking
// again goes, and how the bar names what it collects.
type pollOwner struct {
	page       string // where a Stop or a Retry from a plain form returns
	again      string
	againLabel view.Text
	what       string
	// polled makes the bar speak of a poll, as the frames over a list and a
	// timeline do: «Polled 5 of 6 sources - 14 events», running or not. A
	// search says something else: it is asked once and answers once.
	polled bool
	found  func(n int) string
	// matched are the addresses a person's timeline is looked for by: its
	// bar says them, in mono, and names a source that did not answer as an
	// alarm instead of counting it (s-contacts-7).
	matched []string
	// named: the bar names a source that did not answer, as an alarm,
	// instead of counting it — «Polled 5 of 6 sources · 14 events ·
	// ‹baikal · Общий unavailable›» (s-agenda-1).
	named bool
	// missing is the line under the rows once the poll is over and a source
	// did not answer: what is shown and what is not (states-6, states-9),
	// for that many sources; nil — no line.
	missing func(sources int) string
	// query is the query of the page the bar stands on — a list's filter,
	// window and switches —, carried by the addresses of the stream and the
	// poll: the rail the end of the poll brings is that page's (C-7.3).
	query url.Values
}

// pageQuery is a page's query as the addresses of its poll carry it: what
// chooses what the page shows, not the task, a run asked again, the panel
// or the print.
func pageQuery(q url.Values) url.Values {
	out := url.Values{}
	for k, v := range q {
		switch k {
		case "task", "again", "open", "foreign", render.PrintParam, "card":
		default:
			out[k] = v
		}
	}
	return out
}

func searchOwner(q string) pollOwner {
	return pollOwner{
		page:       searchHref(q),
		again:      searchHref(q) + "&again=1",
		againLabel: "Search again",
		what:       "search",
		found:      func(n int) string { return fmt.Sprintf("%d found", n) },
	}
}

// pollBar is the fanout on the screen (C-8.1): every source with its state
// and count, a short reason and a retry of that source alone where it did not
// answer, and a summary that says whether the poll is over.
func (s *Server) pollBar(tf timeFmt, blob sealed.Blob, snap fanout.Snapshot, owner pollOwner) view.PollBar {
	now := s.now()
	bar := view.PollBar{TaskID: snap.ID, Running: snap.Running}
	base := "/app/search/progress/" + snap.ID
	// Where to follow it (C-7.4): the stream, unless the instance polls
	// instead, the poll and its interval — with the page's query.
	query := ""
	if len(owner.query) > 0 {
		query = "?" + owner.query.Encode()
	}
	if s.cfg.ProgressMode != "poll" {
		bar.Stream = s.route(base + query)
	}
	bar.Poll, bar.Every = s.route(base+"/poll"+query), s.cfg.ProgressPollMillis
	var answered, refused, timedOut, unreachable, stopped, cached int
	for _, st := range snap.Sources {
		if st.FromCache {
			cached++
		}
		account, path, _ := strings.Cut(st.ID, ":")
		origin, colour := sourceOrigin(blob, account, path)
		sp := view.SourceProgress{
			Stamp:     origin,
			Colour:    colour,
			State:     st.State,
			FromCache: st.FromCache,
			Message:   text(sentence(st.Note)),
		}
		switch st.State {
		case view.SourceDone, view.SourceEmpty:
			answered++
			n := st.Count
			sp.Count = &n
		case view.SourceError:
			refused++
			sp.Message = text(refusalReason(st.Err))
		case view.SourceTimeout:
			timedOut++
			sp.Message = "No answer in time."
		case view.SourceUnavailable:
			unreachable++
			sp.Message = "The server could not be reached."
		case view.SourceCancelled:
			stopped++
			sp.Message = "Stopped before it answered."
		}
		switch st.State {
		case view.SourceError, view.SourceTimeout, view.SourceUnavailable, view.SourceCancelled:
			sp.Retry = &view.Action{
				ID: "source.retry", Label: "Retry", Method: "POST", Enabled: true,
				Href: s.route(base + "/retry?source=" + url.QueryEscape(st.ID)),
			}
		}
		bar.Sources = append(bar.Sources, sp)
	}
	total := len(snap.Sources)
	if total > 0 {
		bar.Percent = (answered + refused + timedOut + unreachable + stopped) * 100 / total
	}
	switch {
	case owner.polled:
		// One sentence, running or not: the meter and the Stop button
		// already say whether it is still going, and a bar that rewords
		// itself under the same numbers reads as two different answers.
		parts := []string{
			fmt.Sprintf("Polled %d of %s", answered, plural(total, "source")),
			owner.found(len(snap.Items)),
		}
		bar.Summary = line(strings.Join(append(parts, pollTroubles(refused, unreachable, timedOut, stopped)...), " · "))
		if owner.named {
			bar.Summary = append(view.Runs{{Text: text(strings.Join(parts, " · "))}}, troubleRuns(bar.Sources)...)
		}
		if owner.matched != nil {
			bar.Summary = matchedSummary(answered, total, owner.matched, bar.Sources)
		}
		if !snap.Running {
			bar.AnsweredAt = tf.formatWhen(snap.Finished, false, now)
			// The right of the bar (s-agenda-1): when the answer was
			// gathered, and that it came from what the session held.
			bar.Aside = view.Text(tf.formatWhen(snap.Finished, true, now))
			if cached > 0 {
				bar.Aside = "from cache · " + bar.Aside
			}
			if down := refused + timedOut + unreachable + stopped; down > 0 && owner.missing != nil {
				bar.Note = text(owner.missing(down))
			}
		}
	case snap.Running:
		bar.Summary = line(fmt.Sprintf("Asking %s · %d answered · %s", plural(total, "source"), answered, owner.found(len(snap.Items))))
	default:
		parts := []string{
			fmt.Sprintf("%d of %s answered", answered, plural(total, "source")),
			owner.found(len(snap.Items)),
		}
		bar.Summary = line(strings.Join(append(parts, pollTroubles(refused, unreachable, timedOut, stopped)...), " · "))
		bar.AnsweredAt = tf.formatWhen(snap.Finished, false, now)
	}
	stop := view.Action{ID: "poll.stop", Label: "Stop", Href: s.route(base + "/cancel"), Method: "POST", Enabled: snap.Running}
	again := view.Action{ID: "poll.again", Label: owner.againLabel, Href: s.route(owner.again), Method: "GET", Enabled: !snap.Running}
	if snap.Running {
		again.Reason = text("The " + owner.what + " is still running.")
	} else {
		stop.Reason = text("The " + owner.what + " has finished.")
	}
	bar.Actions = []view.Action{stop, again}
	return bar
}

// pollTroubles names what went wrong, each kind of trouble once and only
// when there is any: «1 unavailable».
func pollTroubles(refused, unreachable, timedOut, stopped int) []string {
	var out []string
	for _, c := range []struct {
		n    int
		what string
	}{{refused, "refused"}, {unreachable, "unavailable"}, {timedOut, "timed out"}, {stopped, "stopped"}} {
		if c.n > 0 {
			out = append(out, fmt.Sprintf("%d %s", c.n, c.what))
		}
	}
	return out
}

func refusalReason(err error) string {
	var se *dav.StatusError
	switch {
	case err == nil:
		return ""
	case errors.As(err, &se):
		return fmt.Sprintf("The server answered %d %s.", se.Code, http.StatusText(se.Code))
	case errors.Is(err, card.ErrNoReporter), errors.Is(err, cal.ErrNoReporter):
		return "This connection cannot search: it offers no REPORT."
	}
	return sentence(err.Error())
}

func sentence(s string) string {
	s = strings.TrimSpace(s)
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return ""
	}
	return string(unicode.ToUpper(r)) + s[n:]
}

// jumpRows are the hints of the search box (S-15.3): collection names and
// records of lists this session has already loaded. No server is asked;
// nothing matching means no block.
func (s *Server) jumpRows(sess *session.Session, blob sealed.Blob, q string) []view.JumpRow {
	needle := strings.ToLower(q)
	var out []view.JumpRow
	for _, col := range blob.Collections {
		conn, ok := blob.Connection(col.AccountID)
		if col.Gone || !ok || len(col.Kinds) == 0 {
			continue
		}
		host, label := connLabel(conn), collectionLabel(col)
		if strings.Contains(strings.ToLower(label), needle) && len(out) < jumpRowsMost {
			out = append(out, view.JumpRow{Href: s.route(collectionHref(col)), Kind: col.Kinds[0], Title: text(label), Stamp: stamp(host, label)})
		}
		// The collection's name stands before its records, so this is
		// a walk of collections and not eachIndexed.
		for _, e := range indexedIn(sess, col) {
			if len(out) < jumpRowsMost && strings.Contains(strings.ToLower(e.Title), needle) {
				out = append(out, view.JumpRow{
					Href:  s.route("/app/" + segmentOf(e.Kind) + "/" + conn.ID + "/" + url.PathEscape(col.Path) + "/" + url.PathEscape(e.UID)),
					Kind:  e.Kind,
					Title: text(e.Title),
					Stamp: stamp(host, label),
				})
			}
		}
	}
	return out
}

func collectionHref(col sealed.Collection) string {
	seg := url.PathEscape(col.Path)
	if col.Has(view.KindFile) {
		return "/app/files/" + col.AccountID + "/" + seg + "/"
	}
	return "/app/" + segmentOf(col.Kinds[0]) + "/" + col.AccountID + "/" + seg
}

// collectionSource is one ticked collection in a fanout, asked for the kinds
// it is ticked for with the providers of its connection. Records a REPORT
// returned unreadable are left out of a search; the collection's own list
// names them.
type collectionSource struct {
	id         string
	conn       sealed.Connection
	col        sealed.Collection
	kinds      []view.Kind
	card       *card.Provider
	cal        *cal.Provider
	files      *file.Provider
	maxEntries int
	broken     error       // the connection could not be opened at all
	feed       *feedReader // a subscription: read whole and sifted here (S-25.2)
	root       string      // where a file collection starts on its server
	// sess, t and ttl are what a whole read needs to be kept (S-4.2,
	// wholeread.go): the session whose cache keeps it, the transport that
	// asks the collection's change tag, and the soft limit for a server with
	// none with the clock it is measured by. A nil sess reads straight through.
	sess *session.Session
	t    dav.Transport
	ttl  time.Duration
	now  func() time.Time
}

// searchRail is the rail of the search: every collection it asks, of every
// kind, in one list (s-find-1). A search that found nothing must say what
// it searched — an unticked collection is not asked, and a person looking
// for a record that is in one has to see why it was not found.
//
// snap is the search on the screen, nil without a query. Each source then
// counts its finds — a feed too: here the number is what was found in it,
// not what it holds — and one that did not answer says why, with a Retry
// of it alone, as in every rail. The numbers are those of the page's render:
// the stream of the poll moves the poll bar only (C-7.3).
//
// The rubric is «Sources a / b»: a sources asked of b ticked. A ticked
// collection that is gone is not asked (S-6.1) and stays in the list,
// marked gone. That one did not answer is the poll bar's to say, and
// s-find-1 draws «14 / 14» beside a source with no reply.
func (s *Server) searchRail(sess *session.Session, blob sealed.Blob, snap *fanout.Snapshot) view.Rail {
	rail := view.Rail{Nav: s.navRows(sess, blob, "search"), About: s.aboutAction(), Davloom: s.davloomMark(sess)}
	found := map[string]int{}
	if snap != nil {
		for _, st := range snap.Sources {
			if st.State == view.SourceDone || st.State == view.SourceEmpty {
				found[st.ID] = st.Count
			}
		}
	}
	troubles := s.troublesOf(snap)
	seen := map[string]bool{}
	sec := view.RailSection{State: view.CollectionOK}
	asked := 0
	for _, pick := range blob.Selected {
		id := pick.AccountID + ":" + pick.Path
		col, ok := blob.Collection(pick.AccountID, pick.Path)
		if _, conn := blob.Connection(pick.AccountID); !ok || !conn || !col.Has(pick.Kind) || seen[id] {
			continue // what tickedCollections would not ask either
		}
		seen[id] = true
		row := view.SourceRow{
			Label: text(collectionLabel(col)), Colour: catalogColour(col),
			Tick: view.TickOn, ReadOnly: col.ReadOnly, State: view.CollectionOK,
		}
		if col.Gone {
			row.State = view.CollectionGone
			sec.Rows = append(sec.Rows, row)
			continue
		}
		asked++
		if n, ok := found[id]; ok {
			row.Count = &n
		}
		if t, ok := troubles[id]; ok {
			row.State = view.CollectionUnavailable
			row.Message, row.Detail, row.Retry = text(t.message), text(t.detail), t.retry
		}
		sec.Rows = append(sec.Rows, row)
	}
	if len(sec.Rows) == 0 {
		return rail
	}
	sec.Account = text(fmt.Sprintf("Sources %d / %d", asked, len(sec.Rows)))
	rail.Sections = []view.RailSection{sec}
	return rail
}

// sourceOrigin is the stamp and colour of a source's rows and of its line in
// the poll bar (C-10): the one place that turns a source's account and path
// into how it is shown, so a source of another kind is named here too.
func sourceOrigin(blob sealed.Blob, account, path string) (view.Stamp, view.Colour) {
	col, found := blob.Collection(account, path)
	if !found {
		col = sealed.Collection{AccountID: account, Path: path}
	}
	conn, _ := blob.Connection(account)
	return stamp(connLabel(conn), collectionLabel(col)), catalogColour(col)
}

// searchSources are the sources a search asks: the ticked collections today.
// A source of another kind (a mailbox's folder, carrel-mailbox.md M-6) joins
// here, beside them; code that needs a DAV collection asks tickedCollections
// and never asserts a source's type.
func (s *Server) searchSources(sess *session.Session, blob sealed.Blob) []fanout.Source {
	var out []fanout.Source
	for _, c := range s.tickedCollections(sess, blob) {
		out = append(out, c)
	}
	return out
}

// tickedCollections opens one source per ticked collection, with the kinds it
// is ticked for.
func (s *Server) tickedCollections(sess *session.Session, blob sealed.Blob) []*collectionSource {
	var out []*collectionSource
	byID := map[string]*collectionSource{}
	for _, pick := range blob.Selected {
		conn, hasConn := blob.Connection(pick.AccountID)
		col, hasCol := blob.Collection(pick.AccountID, pick.Path)
		if !hasConn || !hasCol || col.Gone || !col.Has(pick.Kind) {
			continue
		}
		id := conn.ID + ":" + col.Path
		src := byID[id]
		if src == nil {
			src = s.openSource(sess, id, conn, col)
			byID[id] = src
			out = append(out, src)
		}
		if !slices.Contains(src.kinds, pick.Kind) {
			src.kinds = append(src.kinds, pick.Kind)
		}
	}
	return out
}

func (s *Server) openSource(sess *session.Session, id string, conn sealed.Connection, col sealed.Collection) *collectionSource {
	src := &collectionSource{id: id, conn: conn, col: col, maxEntries: s.cfg.FilesMaxEntries, sess: sess, ttl: s.cfg.CacheCollectionTTL, now: s.now}
	src.root = fileRoot(conn, col)
	if conn.Feed && !col.Has(view.KindFile) {
		src.feed = s.newFeedReader(sess, conn)
		return src
	}
	client, err := dav.NewClient(s.guard, conn.Server, conn.Login, conn.Password, conn.CertFingerprint)
	if err != nil {
		src.broken = err
		return src
	}
	src.t = client
	if src.card, err = card.New(client); err != nil {
		src.broken = err
	} else if src.cal, err = cal.New(client); err != nil {
		src.broken = err
	} else if src.files, err = file.New(client); err != nil {
		src.broken = err
	}
	if src.cal != nil {
		src.cal.ReadWhole = src.readWhole
	}
	if src.card != nil {
		src.card.ReadWhole = src.readBook
	}
	return src
}

func (c *collectionSource) ID() string { return c.id }

// readBook is the whole read a card search checks an empty answer against
// (SV-51), kept like any other (S-4.2).
func (c *collectionSource) readBook(ctx context.Context, collection string) ([]*model.Object, error) {
	return cachedWholeRead(ctx, c.sess, c.t, c.conn.ID, collection, "", c.ttl, c.now(), cardCodec,
		func() ([]*model.Object, bool, error) {
			got, err := c.card.Query(ctx, collection)
			return got, true, err
		})
}

// readWhole is the whole read a calendar search checks an empty answer
// against (SV-47), kept like any other (S-4.2): the next search of the
// unchanged calendar is sifted from it.
func (c *collectionSource) readWhole(ctx context.Context, collection, component string) ([]*model.Object, error) {
	return cachedWholeRead(ctx, c.sess, c.t, c.conn.ID, collection, component, c.ttl, c.now(), calCodec,
		func() ([]*model.Object, bool, error) {
			got, unreadable, err := c.cal.Query(ctx, collection, component, nil, nil)
			if err == nil && component == "VEVENT" {
				putCount(c.sess, c.conn.ID, collection, component, len(got)+len(unreadable))
			}
			return got, len(unreadable) == 0, err
		})
}

// Asks is one far request per kind the collection holds: Run walks them
// one after another. A calendar of events, tasks and notes asks three
// times, an address book once, and the budget follows the work
// (fanout.Asks).
func (c *collectionSource) Asks() int {
	if len(c.kinds) < 1 {
		return 1
	}
	return len(c.kinds)
}

// Run asks the collection for each of its kinds. The kinds go side by
// side, each taking a slot of the fanout's ceiling around its own request
// (fanout.Slot): a calendar of events, tasks and notes used to cost three
// waits where an address book cost one, and the ceiling stood idle
// meanwhile. The server is asked no harder — the same number of requests
// may be in flight as before — and the person waits for the slowest kind
// rather than for all of them in a row.
func (c *collectionSource) Run(ctx context.Context, q fanout.Query) ([]fanout.Item, error) {
	if c.broken != nil {
		return nil, c.broken
	}
	found := make([][]fanout.Item, len(c.kinds))
	errs := make([]error, len(c.kinds))
	var wg sync.WaitGroup
	for i, k := range c.kinds {
		wg.Add(1)
		go func(i int, k view.Kind) {
			defer wg.Done()
			found[i], errs[i] = fanout.AskingKind(c.id+" "+string(k), func() ([]fanout.Item, error) {
				return c.askKind(ctx, k, q)
			})
		}(i, k)
	}
	wg.Wait()

	var items []fanout.Item
	var note error
	for i := range c.kinds {
		items = append(items, found[i]...)
		if err := errs[i]; err != nil {
			// A note on one kind does not stop the others of the collection.
			if errors.Is(err, fanout.ErrIncomplete) {
				note = err
				continue
			}
			return nil, err
		}
	}
	return items, note
}

// askKind is one kind of the collection, under one slot of the ceiling.
// The slot is taken around the request and given back at once: a holder
// that waited for another slot while holding one would be a deadlock, and
// nothing here holds while waiting.
func (c *collectionSource) askKind(ctx context.Context, k view.Kind, q fanout.Query) ([]fanout.Item, error) {
	release, err := fanout.Slot(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	switch k {
	case view.KindContact:
		return c.contacts(ctx, q)
	case view.KindFile:
		return c.findFiles(ctx, q)
	default:
		return c.records(ctx, k, q)
	}
}

func (c *collectionSource) contacts(ctx context.Context, q fanout.Query) ([]fanout.Item, error) {
	var objs []*model.Object
	var err error
	switch {
	case c.feed != nil && q.Text == "":
		objs, err = c.feed.records(ctx, view.KindContact)
	case c.feed != nil:
		objs, err = c.feed.search(ctx, view.KindContact, q.Text)
	case q.Text == "":
		// A whole read: kept by the session and read again only when the
		// book changed (S-4.2, wholeread.go).
		objs, err = cachedWholeRead(ctx, c.sess, c.t, c.conn.ID, c.col.Path, "", c.ttl, c.now(), cardCodec,
			func() ([]*model.Object, bool, error) {
				got, err := c.card.Query(ctx, c.col.Path)
				return got, true, err
			})
	default:
		if kept, ok := keptWholeRead(ctx, c.sess, c.t, c.conn.ID, c.col.Path, "", c.ttl, c.now(), cardCodec); ok {
			objs = card.Sift(kept, q.Text) // the book is read and unchanged (S-4.2)
			break
		}
		objs, err = c.card.Search(ctx, c.col.Path, q.Text)
	}
	var note error
	if errors.Is(err, card.ErrServerDoesNotQuery) {
		// The book was listed and fetched because this server has no
		// addressbook-query at all (docs/servers.md, SV-23). The records are
		// good and the searching was ours, which is what the source says.
		note = fmt.Errorf("%w: this server does not search its address books, so Carrel sifted its answer", fanout.ErrIncomplete)
		err = nil
	}
	if errors.Is(err, card.ErrServerMissed) {
		note = fmt.Errorf("%w: the server's search found nothing here, so Carrel sifted the address book itself", fanout.ErrIncomplete)
		err = nil
	}
	if err != nil {
		return nil, err
	}
	items := make([]fanout.Item, 0, len(objs))
	for _, o := range objs {
		items = append(items, c.item(view.KindContact, card.UID(o), o))
	}
	return items, note
}

func (c *collectionSource) records(ctx context.Context, k view.Kind, q fanout.Query) ([]fanout.Item, error) {
	component := recordKinds[segmentOf(k)].component
	var objs []*model.Object
	var err error
	switch {
	case c.feed != nil && q.Text == "":
		objs, err = c.feed.records(ctx, k)
	case c.feed != nil:
		objs, err = c.feed.search(ctx, k, q.Text)
	case q.Text == "":
		from, to := q.From, q.To
		if k != view.KindEvent {
			// A task is placed by DUE and a note by its DTSTART; a server's
			// time-range filter answers for neither reliably, so the whole
			// component is read and the window is applied where the rows are
			// placed (review Р-12).
			from, to = nil, nil
		}
		if from == nil && to == nil {
			// A whole read: kept by the session and read again only when the
			// collection changed (S-4.2, wholeread.go).
			objs, err = cachedWholeRead(ctx, c.sess, c.t, c.conn.ID, c.col.Path, component, c.ttl, c.now(), calCodec,
				func() ([]*model.Object, bool, error) {
					got, unreadable, err := c.cal.Query(ctx, c.col.Path, component, nil, nil)
					if err == nil && component == "VEVENT" {
						putCount(c.sess, c.conn.ID, c.col.Path, component, len(got)+len(unreadable))
					}
					return got, len(unreadable) == 0, err
				})
			break
		}
		// A read by window, kept under the window (S-4.2).
		objs, err = cachedWholeRead(ctx, c.sess, c.t, c.conn.ID, c.col.Path, windowKey(component, from, to), c.ttl, c.now(), calCodec,
			func() ([]*model.Object, bool, error) {
				got, unreadable, err := c.cal.Query(ctx, c.col.Path, component, from, to)
				if err == nil && component == "VEVENT" {
					countEvents(ctx, c.sess, c.cal, c.conn.ID, c.col.Path) // the rail's number (C-6.3)
				}
				return got, len(unreadable) == 0, err
			})
	default:
		if kept, ok := keptWholeRead(ctx, c.sess, c.t, c.conn.ID, c.col.Path, component, c.ttl, c.now(), calCodec); ok {
			objs = cal.Sift(kept, component, q.Text) // the calendar is read and unchanged (S-4.2)
			break
		}
		objs, _, err = c.cal.Search(ctx, c.col.Path, component, q.Text)
	}
	var note error
	switch {
	case errors.Is(err, cal.ErrLatinCaseOnly):
		note = fmt.Errorf("%w: the server ignores letter case in Latin script only", fanout.ErrIncomplete)
		err = nil
	case errors.Is(err, cal.ErrServerDoesNotFilter):
		note = fmt.Errorf("%w: this server does not search its calendars, so Carrel sifted its answer", fanout.ErrIncomplete)
		err = nil
	}
	if err != nil {
		return nil, err
	}
	items := make([]fanout.Item, 0, len(objs))
	for _, o := range objs {
		items = append(items, c.item(k, cal.UID(o), o))
	}
	return items, note
}

func (c *collectionSource) item(k view.Kind, uid string, o *model.Object) fanout.Item {
	return fanout.Item{AccountID: c.conn.ID, Col: c.col.Path, Path: o.Path, UID: uid, ETag: o.ETag, Kind: k, Object: o}
}

// findFiles walks the folders of a file collection, one PROPFIND a folder,
// and keeps the files whose name holds the query (S-15.2). No body is read.
// The walk stops at CARREL_FILES_MAX_ENTRIES entries and says so. Without a
// query it lists the top folder only.
func (c *collectionSource) findFiles(ctx context.Context, q fanout.Query) ([]fanout.Item, error) {
	needle := strings.ToLower(q.Text)
	var items []fanout.Item
	queue := []string{c.root}
	seen := 0
	for len(queue) > 0 {
		folder := queue[0]
		queue = queue[1:]
		entries, err := c.files.List(ctx, folder)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			seen++
			if seen > c.maxEntries {
				return items, fmt.Errorf("%w: stopped after %d entries", fanout.ErrIncomplete, c.maxEntries)
			}
			if e.Dir {
				if needle != "" {
					queue = append(queue, e.Path)
				}
				continue
			}
			if needle == "" || strings.Contains(strings.ToLower(e.Name), needle) {
				items = append(items, fanout.Item{
					AccountID: c.conn.ID, Col: c.col.Path, Path: e.Path, ETag: e.ETag,
					Kind: view.KindFile, FileName: e.Name, FileSize: e.Size, FileModified: e.Modified,
				})
			}
		}
	}
	return items, nil
}

// openSearchTask finds a task of this session: a search or the list of a
// combined view, which share the progress routes (C-5.3). Another session's
// task, or an ID never issued, is not found (C-4).
func (s *Server) openSearchTask(w http.ResponseWriter, r *http.Request) (*session.Session, *fanout.Task, pollOwner, bool) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return nil, nil, pollOwner{}, false
	}
	t, _ := sess.Task(r.PathValue("task"))
	switch v := t.(type) {
	case *fanout.Task:
		return sess, v, searchOwner(v.Query().Text), true
	case *listTask:
		owner := listOwner(v)
		owner.query = pageQuery(r.URL.Query())
		return sess, v.Task, owner, true
	case *timelineTask:
		return sess, v.Task, timelineOwner(v), true
	}
	s.notFound(w, r)
	return nil, nil, pollOwner{}, false
}

func (s *Server) catalogOf(sess *session.Session) sealed.Blob {
	key, _ := sess.DataKey()
	blob, _ := s.users.Catalog(sess.AccountID(), key)
	return blob
}

// getSearchProgressSSE streams the poll bar while the task runs (C-5.3): a
// comment before anything is waited for, an event per change, poll-finished
// at the end. The stream closing while the task runs is the page being left,
// and cancels the task (S-15.1).
func (s *Server) getSearchProgressSSE(w http.ResponseWriter, r *http.Request) {
	sess, task, owner, ok := s.openSearchTask(w, r)
	if !ok {
		return
	}
	t, _ := sess.Task(r.PathValue("task"))
	if s.cfg.ProgressMode == "poll" {
		s.notFound(w, r)
		return
	}
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
		return
	}
	_ = rc.Flush()

	blob, tf, token := s.catalogOf(sess), s.timeFor(sess), sess.CSRFToken()
	ping := time.NewTicker(sseKeepAlive)
	defer ping.Stop()
	for {
		changed := task.Changed()
		snap := task.Snapshot()
		// An event is no page and goes past the stamping renderer: its
		// POSTs get the session's token and every action its target here.
		bar := stampView(s.pollBar(tf, blob, snap, owner), token, s.basePath)
		if err := s.render.Event(w, render.EventPoll, render.TargetPoll, bar); err != nil {
			return
		}
		if !snap.Running {
			// The end brings the page's rail: its counts and the states of
			// its rows are the poll's (C-7.3, C-7.4). A page that is gone
			// has no rail to bring, and the end is said with the bar.
			if rail := s.railAtTheEnd(sess, t, snap, r.URL.Query()); rail != nil {
				_ = s.render.Event(w, render.EventPollFinished, render.TargetRail, stampView(*rail, token, s.basePath))
			} else {
				_ = s.render.Event(w, render.EventPollFinished, render.TargetPoll, bar)
			}
			_ = rc.Flush()
			return
		}
		_ = rc.Flush()
		for waiting := true; waiting; {
			select {
			case <-changed:
				waiting = false
			case <-ping.C:
				if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
					return
				}
				_ = rc.Flush()
			case <-r.Context().Done():
				task.Cancel()
				return
			}
		}
	}
}

// getSearchProgressPoll is the fallback of the stream (CARREL_PROGRESS_MODE).
func (s *Server) getSearchProgressPoll(w http.ResponseWriter, r *http.Request) {
	sess, task, owner, ok := s.openSearchTask(w, r)
	if !ok {
		return
	}
	if !render.WantsFragment(r) {
		s.notFound(w, r)
		return
	}
	snap := task.Snapshot()
	var extra []render.OOB
	if !snap.Running {
		// Finished: the page's rail with it, as the stream's end brings it.
		t, _ := sess.Task(r.PathValue("task"))
		if rail := s.railAtTheEnd(sess, t, snap, r.URL.Query()); rail != nil {
			extra = append(extra, render.OOB{Target: render.TargetRail, Body: *rail})
		}
	}
	_ = s.render.Fragment(w, r, http.StatusOK, render.TargetPoll, s.pollBar(s.timeFor(sess), s.catalogOf(sess), snap, owner), extra)
}

// railAtTheEnd is the rail of the page a finished poll stands on, as that
// page draws it now: the counts of its rows and the nav, and the states of
// the rows that did not answer, are the poll's (C-7.3). query is the page's
// (PollBar.Stream carries it). nil — the page is not there any more.
func (s *Server) railAtTheEnd(sess *session.Session, t any, snap fanout.Snapshot, query url.Values) *view.Rail {
	blob := s.catalogOf(sess)
	var rail view.Rail
	switch v := t.(type) {
	case *fanout.Task:
		rail = s.searchRail(sess, blob, &snap)
	case *timelineTask:
		c, ok := s.collectionOf(sess, recordKinds["contacts"], v.account, v.col)
		if !ok {
			return nil
		}
		rail = s.timelineRail(c, &snap, c.recordPath(v.uid)+"/timeline")
	case *listTask:
		spec, ok := recordKinds[segmentOf(v.kind)]
		if !ok {
			return nil
		}
		// What the page does when it is read after the poll: the counts are
		// the kept index of what answered.
		s.indexAnswered(sess, blob, spec, snap)
		base := "/app/" + spec.segment
		rail = s.kindRail(sess, blob, spec, &snap, base)
		q := pageQuery(query)
		q.Set("task", v.ID())
		rail.Tags = s.tagFilter(base, q).rail(v.kind, listedOf(blob, snap.Items))
	default:
		return nil
	}
	return &rail
}

// postSearchProgressRetry asks one source again and nobody else (S-15.1).
func (s *Server) postSearchProgressRetry(w http.ResponseWriter, r *http.Request) {
	sess, task, owner, ok := s.openSearchTask(w, r)
	if !ok {
		return
	}
	source := r.FormValue("source")
	err := task.Retry(source)
	if errors.Is(err, fanout.ErrUnknownSource) {
		s.notFound(w, r)
		return
	}
	s.answerPollAction(w, r, sess, task, owner, source, err)
}

// postSearchProgressCancel stops the task; what arrived stays, marked.
func (s *Server) postSearchProgressCancel(w http.ResponseWriter, r *http.Request) {
	sess, task, owner, ok := s.openSearchTask(w, r)
	if !ok {
		return
	}
	task.Cancel()
	s.answerPollAction(w, r, sess, task, owner, "", nil)
}

// answerPollAction answers a button of the poll bar: the bar as a fragment,
// or the page of the task again for a plain form.
func (s *Server) answerPollAction(w http.ResponseWriter, r *http.Request, sess *session.Session, task *fanout.Task, owner pollOwner, source string, err error) {
	if !render.WantsFragment(r) {
		s.redirect(w, r, owner.page)
		return
	}
	snap := task.Snapshot()
	bar := s.pollBar(s.timeFor(sess), s.catalogOf(sess), snap, owner)
	if errors.Is(err, fanout.ErrBusy) {
		for i, st := range snap.Sources {
			if st.ID == source {
				bar.Sources[i].Message = text(fmt.Sprintf("This session already runs %d searches and lists. Stop one, then retry.", conf.MaxLiveFanouts))
			}
		}
	}
	_ = s.render.Fragment(w, r, http.StatusOK, render.TargetPoll, bar, nil)
}
