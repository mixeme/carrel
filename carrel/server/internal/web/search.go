package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strings"
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
	}
	if task == nil {
		sources := s.searchSources(blob)
		if len(sources) == 0 {
			page.Poll.Summary = searchNoTicks
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
	s.fillResults(&page, sess, blob, task.Snapshot(), r)
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
	counts := map[view.Kind]int{}
	for _, it := range snap.Items {
		counts[it.Kind]++
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
	segment := func(k view.Kind, label string, n int) view.Action {
		href := searchHref(q)
		if k != "" {
			href += "&kind=" + url.QueryEscape(string(k))
		}
		if filter != "" {
			href += "&filter=" + url.QueryEscape(filter)
		}
		a := view.Action{ID: "search.kind", Label: text(fmt.Sprintf("%s · %d", label, n)), Href: s.route(href), Method: "GET", Enabled: k != kind}
		if k == kind {
			a.Reason = "Shown now."
		}
		return a
	}
	page.Bar.Segments = []view.Action{
		segment("", "All", total),
		segment(view.KindContact, "Contacts", counts[view.KindContact]),
		segment(view.KindEvent, "Events", counts[view.KindEvent]),
		segment(view.KindTask, "Tasks", counts[view.KindTask]),
		segment(view.KindNote, "Notes", counts[view.KindNote]),
		segment(view.KindFile, "Files", counts[view.KindFile]),
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
	col, found := blob.Collection(it.AccountID, it.Col)
	if !found {
		col = sealed.Collection{AccountID: it.AccountID, Path: it.Col}
	}
	conn, _ := blob.Connection(it.AccountID)
	row := view.FindRow{Kind: it.Kind, Colour: catalogColour(col), Stamp: stamp(serverHost(conn.Server), collectionLabel(col))}
	o := it.Object
	switch it.Kind {
	case view.KindFile:
		row.Title = text(it.FileName)
		row.Detail = text(string(formatSize(it.FileSize)))
		row.Href = s.route(fileHref(it))
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
			row.When = text(tf.timeOrDate(tm))
		}
	case view.KindTask:
		row.Title = text(titleOr(cal.Text(o, "SUMMARY"), "Untitled task"))
		if taskDone(o) {
			row.Detail = "Done"
		}
		if due, err := cal.DueOf(o, tf.tz); err == nil && !due.Start.IsZero() {
			row.When = text(tf.timeOrDate(due))
		}
	case view.KindNote:
		body := cal.Description(o)
		title := noteTitle(cal.Text(o, "SUMMARY"), body)
		row.Title, row.Detail = text(title), text(noteExcerpt(body, title))
		if at := noteTime(o, tf.tz); !at.IsZero() {
			row.When = text(tf.date(at))
		}
	}
	row.Href = s.route("/app/" + segmentOf(it.Kind) + "/" + it.AccountID + "/" + url.PathEscape(it.Col) + "/" + url.PathEscape(it.UID))
	return row
}

// fileHref is the folder route of a file: the collection as one segment, the
// rest of the path as a path (C-4.1).
func fileHref(it fanout.Item) string {
	rel := strings.TrimPrefix(it.Path, strings.TrimSuffix(it.Col, "/")+"/")
	parts := strings.Split(rel, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return "/app/files/" + it.AccountID + "/" + url.PathEscape(it.Col) + "/" + strings.Join(parts, "/")
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
	found      func(n int) string
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
	now := time.Now()
	bar := view.PollBar{TaskID: snap.ID, Running: snap.Running}
	base := "/app/search/progress/" + snap.ID
	var answered, refused, timedOut, unreachable, stopped int
	for _, st := range snap.Sources {
		account, path, _ := strings.Cut(st.ID, ":")
		col, found := blob.Collection(account, path)
		if !found {
			col = sealed.Collection{AccountID: account, Path: path}
		}
		conn, _ := blob.Connection(account)
		sp := view.SourceProgress{
			Stamp:     stamp(serverHost(conn.Server), collectionLabel(col)),
			Colour:    catalogColour(col),
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
	if snap.Running {
		bar.Summary = text(fmt.Sprintf("Asking %s · %d answered · %s", plural(total, "source"), answered, owner.found(len(snap.Items))))
	} else {
		parts := []string{
			fmt.Sprintf("%d of %s answered", answered, plural(total, "source")),
			owner.found(len(snap.Items)),
		}
		for _, c := range []struct {
			n    int
			what string
		}{{refused, "refused"}, {unreachable, "unavailable"}, {timedOut, "timed out"}, {stopped, "stopped"}} {
			if c.n > 0 {
				parts = append(parts, fmt.Sprintf("%d %s", c.n, c.what))
			}
		}
		bar.Summary = text(strings.Join(parts, " · "))
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
		host, label := serverHost(conn.Server), collectionLabel(col)
		if strings.Contains(strings.ToLower(label), needle) && len(out) < jumpRowsMost {
			out = append(out, view.JumpRow{Href: s.route(collectionHref(col)), Kind: col.Kinds[0], Title: text(label), Stamp: stamp(host, label)})
		}
		for _, comp := range []string{"", "VEVENT", "VTODO", "VJOURNAL"} {
			data, ok := sess.Cache().GetETagMap(conn.ID, col.Path+"\x00"+comp)
			if !ok {
				continue
			}
			var entries []indexEntry
			if json.Unmarshal(data, &entries) != nil {
				continue
			}
			for _, e := range entries {
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
	broken     error // the connection could not be opened at all
}

// searchSources are the ticked collections, one source each, in the order of
// the ticks. A gone collection is not asked (S-6.1): the rail says gone.
func (s *Server) searchSources(blob sealed.Blob) []fanout.Source {
	var out []fanout.Source
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
			src = s.openSource(id, conn, col)
			byID[id] = src
			out = append(out, src)
		}
		if !slices.Contains(src.kinds, pick.Kind) {
			src.kinds = append(src.kinds, pick.Kind)
		}
	}
	return out
}

func (s *Server) openSource(id string, conn sealed.Connection, col sealed.Collection) *collectionSource {
	src := &collectionSource{id: id, conn: conn, col: col, maxEntries: s.cfg.FilesMaxEntries}
	client, err := dav.NewClient(s.guard, conn.Server, conn.Login, conn.Password, conn.CertFingerprint)
	if err != nil {
		src.broken = err
		return src
	}
	if src.card, err = card.New(client); err != nil {
		src.broken = err
	} else if src.cal, err = cal.New(client); err != nil {
		src.broken = err
	} else if src.files, err = file.New(client); err != nil {
		src.broken = err
	}
	return src
}

func (c *collectionSource) ID() string { return c.id }

func (c *collectionSource) Run(ctx context.Context, q fanout.Query) ([]fanout.Item, error) {
	if c.broken != nil {
		return nil, c.broken
	}
	var items []fanout.Item
	for _, k := range c.kinds {
		var found []fanout.Item
		var err error
		switch k {
		case view.KindContact:
			found, err = c.contacts(ctx, q)
		case view.KindFile:
			found, err = c.findFiles(ctx, q)
		default:
			found, err = c.records(ctx, k, q)
		}
		items = append(items, found...)
		if err != nil {
			if errors.Is(err, fanout.ErrIncomplete) {
				return items, err
			}
			return nil, err
		}
	}
	return items, nil
}

func (c *collectionSource) contacts(ctx context.Context, q fanout.Query) ([]fanout.Item, error) {
	var objs []*model.Object
	var err error
	if q.Text == "" {
		objs, err = c.card.Query(ctx, c.col.Path)
	} else {
		objs, err = c.card.Search(ctx, c.col.Path, q.Text)
	}
	if err != nil {
		return nil, err
	}
	items := make([]fanout.Item, 0, len(objs))
	for _, o := range objs {
		items = append(items, c.item(view.KindContact, card.UID(o), o))
	}
	return items, nil
}

func (c *collectionSource) records(ctx context.Context, k view.Kind, q fanout.Query) ([]fanout.Item, error) {
	component := recordKinds[segmentOf(k)].component
	var objs []*model.Object
	var err error
	if q.Text == "" {
		objs, _, err = c.cal.Query(ctx, c.col.Path, component, q.From, q.To)
	} else {
		objs, _, err = c.cal.Search(ctx, c.col.Path, component, q.Text)
	}
	if err != nil {
		return nil, err
	}
	items := make([]fanout.Item, 0, len(objs))
	for _, o := range objs {
		items = append(items, c.item(k, cal.UID(o), o))
	}
	return items, nil
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
	queue := []string{c.col.Path}
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
					Kind: view.KindFile, FileName: e.Name, FileSize: e.Size,
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
		return sess, v.Task, listOwner(v), true
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

	blob, tf := s.catalogOf(sess), s.timeFor(sess)
	ping := time.NewTicker(sseKeepAlive)
	defer ping.Stop()
	for {
		changed := task.Changed()
		snap := task.Snapshot()
		bar := s.pollBar(tf, blob, snap, owner)
		if err := s.render.Event(w, "poll", TargetPoll, bar); err != nil {
			return
		}
		if !snap.Running {
			_ = s.render.Event(w, "poll-finished", TargetPoll, bar)
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
	if !WantsFragment(r) {
		s.notFound(w, r)
		return
	}
	_ = s.render.Fragment(w, r, http.StatusOK, TargetPoll, s.pollBar(s.timeFor(sess), s.catalogOf(sess), task.Snapshot(), owner), nil)
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
	if !WantsFragment(r) {
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
	_ = s.render.Fragment(w, r, http.StatusOK, TargetPoll, bar, nil)
}
