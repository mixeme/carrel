package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/fanout"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// The person's timeline (S-15.4): a search for one person over the ticked
// sources. A record matches by an address of the card in ATTENDEE or
// ORGANIZER, or by RELATED-TO the card's UID; a file by the person's name in
// its name. It is gathered only when opened; the card's inset shows what this
// session gathered.

const (
	timelineBack      = 12 // months before today the window starts
	timelineAhead     = 3  // months after today it ends
	timelineWidenBy   = 3  // months "Widen the range" moves the start back
	timelineInsetRows = 3
	timelineUndated   = "Without a date"
)

// timelineTask is the fanout of one person's timeline over one window.
type timelineTask struct {
	*fanout.Task
	account, col, uid string
	from              time.Time
	page              string
	person            timelinePerson
}

// timelinePerson is what a record is matched by.
type timelinePerson struct {
	uid    string
	name   string
	emails map[string]bool // lower case, without mailto:
	first  string          // the first address of the card, for New event
}

func timelineOwner(tt *timelineTask) pollOwner {
	return pollOwner{
		page:       tt.page,
		again:      withQuery(tt.page, "again", "1"),
		againLabel: "Look again",
		what:       "timeline",
		found:      func(n int) string { return plural(n, "record") },
	}
}

// timelineSource asks one ticked collection for what points to the person:
// events of the window, tasks and notes whole — their dates are optional and
// RELATED-TO is not a filter every server answers — and files by name. The
// matching is Carrel's, so every server answers the same.
type timelineSource struct {
	*collectionSource
	person timelinePerson
	tz     *time.Location
}

func (ts *timelineSource) Run(ctx context.Context, q fanout.Query) ([]fanout.Item, error) {
	c := ts.collectionSource
	if c.broken != nil {
		return nil, c.broken
	}
	var items []fanout.Item
	var note error
	for _, k := range c.kinds {
		var found []fanout.Item
		var err error
		switch k {
		case view.KindContact:
			continue
		case view.KindFile:
			if ts.person.name == "" {
				continue
			}
			found, err = c.findFiles(ctx, fanout.Query{Text: ts.person.name})
			for i := range found {
				if at := found[i].FileModified; !at.IsZero() && (at.Before(*q.From) || at.After(*q.To)) {
					found[i] = fanout.Item{}
				}
			}
		default:
			window := fanout.Query{}
			if k == view.KindEvent {
				window.From, window.To = q.From, q.To
			}
			found, err = c.records(ctx, k, window)
			for i := range found {
				if _, at, ok := ts.person.match(k, found[i].Object, ts.tz); !ok || !at.IsZero() && (at.Before(*q.From) || at.After(*q.To)) {
					found[i] = fanout.Item{}
				}
			}
		}
		for _, it := range found {
			if it.Path != "" {
				items = append(items, it)
			}
		}
		if err != nil {
			if errors.Is(err, fanout.ErrIncomplete) {
				note = err
				continue
			}
			return nil, err
		}
	}
	return items, note
}

// match says why a record points to the person and when it stands; the
// strongest reason when there are several. Zero time: the record has no date.
func (p timelinePerson) match(k view.Kind, obj *model.Object, tz *time.Location) (string, time.Time, bool) {
	if obj == nil {
		return "", time.Time{}, false
	}
	why := ""
	if org := strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(firstValue(obj, "ORGANIZER"), "mailto:"), "MAILTO:")); org != "" && p.emails[org] {
		why = "organiser"
	}
	if why == "" {
		for _, a := range cal.Attendees(obj) {
			if p.emails[strings.ToLower(a.Email)] {
				why = "attendee"
				if st := strings.ToLower(a.Status); st != "" && st != "needs-action" {
					why += " · " + st
				}
				break
			}
		}
	}
	if why == "" {
		for _, rel := range cal.RelatedTo(obj) {
			if rel == p.uid {
				why = "related to this person"
			}
		}
	}
	if why == "" {
		return "", time.Time{}, false
	}
	return why, timelineWhen(k, obj, tz), true
}

func timelineWhen(k view.Kind, obj *model.Object, tz *time.Location) time.Time {
	switch k {
	case view.KindEvent:
		if tm, err := cal.TimingOf(obj, tz); err == nil {
			return tm.Start
		}
	case view.KindTask:
		if due, err := cal.DueOf(obj, tz); err == nil && !due.Start.IsZero() {
			return due.Start
		}
		if tm, err := cal.TimingOf(obj, tz); err == nil {
			return tm.Start
		}
	case view.KindNote:
		if t, ok := cal.TimeOf(obj, "DTSTART", tz); ok {
			return t
		}
	}
	return time.Time{}
}

func personOf(uid string, obj *model.Object) timelinePerson {
	p := timelinePerson{uid: uid, name: contactName(obj), emails: map[string]bool{}}
	for _, e := range obj.PropText("EMAIL") {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			p.emails[e] = true
			if p.first == "" {
				p.first = e
			}
		}
	}
	return p
}

// timelineWindow is the window of the address: from ?from=, else twelve
// months back; to three months ahead of today, whatever from says.
func timelineWindow(r *http.Request, tz *time.Location, now time.Time) (from, to time.Time) {
	today := dayOf(now, tz)
	from = today.AddDate(0, -timelineBack, 0)
	if v, err := time.ParseInLocation("2006-01-02", r.URL.Query().Get("from"), tz); err == nil && v.Before(today) {
		from = v
	}
	return from, today.AddDate(0, timelineAhead, 0)
}

func dayOf(t time.Time, tz *time.Location) time.Time {
	t = t.In(tz)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, tz)
}

// timelineHere is the address of the timeline over a window: the default one
// has no from.
func timelineHere(card string, from, now time.Time, tz *time.Location) string {
	here := card + "/timeline"
	if !from.Equal(dayOf(now, tz).AddDate(0, -timelineBack, 0)) {
		here = withQuery(here, "from", from.Format("2006-01-02"))
	}
	return here
}

// keptTimeline is this session's newest timeline of the person over the
// window; any window when from is zero (the card's inset).
func keptTimeline(sess *session.Session, account, col, uid string, from time.Time) *timelineTask {
	for _, t := range sess.Tasks() {
		if tt, ok := t.(*timelineTask); ok && tt.account == account && tt.col == col && tt.uid == uid &&
			(from.IsZero() || tt.from.Equal(from)) {
			return tt
		}
	}
	return nil
}

func (s *Server) getContactTimeline(w http.ResponseWriter, r *http.Request) {
	c, ok := s.openCollection(w, r, "contacts", true)
	if !ok {
		return
	}
	uid := r.PathValue("uid")
	obj, err := c.find(r.Context(), uid)
	switch {
	case notFoundErr(err):
		s.notFound(w, r)
		return
	case err != nil:
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, nil, s.unavailableNotice(c, c.recordPath(uid)+"/timeline", err)))
		return
	}
	tz := s.tz()
	from, to := timelineWindow(r, tz, s.now())
	person := personOf(uid, obj)
	card := c.recordPath(uid)
	page := timelineHere(card, from, s.now(), tz)

	var task *timelineTask
	if r.URL.Query().Get("again") == "" {
		task = keptTimeline(c.sess, c.conn.ID, c.col.Path, uid, from)
	}
	var notices []view.Notice
	if task == nil {
		task, err = s.startTimeline(c.sess, person, from, to)
		switch {
		case err == nil:
			task.account, task.col, task.uid, task.from, task.page = c.conn.ID, c.col.Path, uid, from, page
			c.sess.KeepTask(task)
		case errors.Is(err, fanout.ErrBusy):
			notices = append(notices, view.Notice{Role: view.NoticeAlert, Title: "Too many searches",
				Body: text(fmt.Sprintf("This session already runs %d searches and lists. Stop one or wait for one to finish, then open the timeline again.", conf.MaxLiveFanouts))})
		default:
			http.Error(w, "could not start the timeline", http.StatusInternalServerError)
			return
		}
	}
	s.writePage(w, r, http.StatusOK, s.timelinePage(c, obj, person, task, from, to, r.URL.Query().Get("show"), notices))
}

func (s *Server) startTimeline(sess *session.Session, person timelinePerson, from, to time.Time) (*timelineTask, error) {
	var sources []fanout.Source
	for _, src := range s.searchSources(s.catalogOf(sess)) {
		cs := src.(*collectionSource)
		sources = append(sources, &timelineSource{collectionSource: cs, person: person, tz: s.tz()})
	}
	id, err := user.NewTaskID()
	if err != nil {
		return nil, err
	}
	opts := s.searchOptions(sess)
	opts.Finished = nil // a timeline is not a query of the history (S-15.3)
	opts.Key = nil
	ft, err := fanout.Start(id, fanout.Query{Text: person.name, From: &from, To: &to}, sources, opts)
	if err != nil {
		return nil, err
	}
	return &timelineTask{Task: ft, person: person}, nil
}

// timelineRow is one row with the time it sorts by.
type timelineRow struct {
	row view.TimelineRow
	at  time.Time
}

func (s *Server) timelineRows(tf timeFmt, blob sealed.Blob, snap fanout.Snapshot, person timelinePerson) []timelineRow {
	var out []timelineRow
	for _, it := range snap.Items {
		o := origin{}
		o.conn, _ = blob.Connection(it.AccountID)
		o.col, _ = blob.Collection(it.AccountID, it.Col)
		o.conn.ID, o.col.Path = it.AccountID, it.Col
		row := view.TimelineRow{Kind: it.Kind, Stamp: o.stamp()}
		var at time.Time
		if it.Kind == view.KindFile {
			row.Title, row.Href, row.Why = text(it.FileName), s.route(foundFileHref(it)), "name in the file name"
			at = it.FileModified
		} else {
			why, when, ok := person.match(it.Kind, it.Object, tf.tz)
			if !ok {
				continue
			}
			row.Title, row.Why, at = text(recordTitle(it.Kind, it.Object)), text(why), when
			row.Href = s.route(o.recordPath(segmentOf(it.Kind), it.UID))
		}
		if !at.IsZero() {
			row.When = text(tf.date(at))
		}
		out = append(out, timelineRow{row: row, at: at})
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.at.IsZero() != b.at.IsZero() {
			return !a.at.IsZero()
		}
		if !a.at.Equal(b.at) {
			return a.at.After(b.at)
		}
		return a.row.Title < b.row.Title
	})
	return out
}

var timelineShows = []struct {
	value, label string
	kind         view.Kind
}{
	{"all", "All", ""}, {"events", "Events", view.KindEvent}, {"tasks", "Tasks", view.KindTask},
	{"notes", "Notes", view.KindNote}, {"files", "Files", view.KindFile},
}

func (s *Server) timelinePage(c *collectionCtx, obj *model.Object, person timelinePerson, task *timelineTask, from, to time.Time, show string, notices []view.Notice) view.ContactTimelinePage {
	tf := c.tf
	card := c.recordPath(person.uid)
	sh := s.buildShell(c.sess, person.name+" · Timeline")
	sh.Rail = s.recordRail(c, view.Action{ID: "nav.back", Label: text(person.name), Href: s.route(card), Method: "GET", Enabled: true})
	sh.Notices = append(sh.Notices, notices...)
	page := view.ContactTimelinePage{Shell: sh, UID: person.uid, Name: text(person.name)}
	page.Card = view.Details{Title: text(person.name), Source: c.source(),
		Actions: []view.Action{{ID: "timeline.card", Label: "Open the card", Href: s.route(card), Method: "GET", Enabled: true}}}
	for _, f := range contactFieldsOf(obj) {
		page.Card.Fields = append(page.Card.Fields, f.Field)
	}
	here := timelineHere(card, from, tf.now, tf.tz)
	page.Range = view.Range{From: text(tf.date(from)), To: text(tf.date(to)), Presets: []view.Action{{
		ID: "timeline.widen", Label: "Widen the range", Method: "GET", Enabled: true,
		Href: s.route(card + "/timeline?from=" + from.AddDate(0, -timelineWidenBy, 0).Format("2006-01-02")),
	}}}
	page.Note = text("Nothing was looked for before " + tf.date(from) + ". Widen the range if you need it.")
	q := url.Values{"related": {person.uid}}
	page.Actions = []view.Action{
		{ID: "timeline.note", Label: "Write a note", Href: s.postQuery("/app/notes/quick", q), Method: "GET", Enabled: true},
		{ID: "timeline.event", Label: "New event", Href: s.postQuery("/app/calendar/quick", url.Values{"attendee": {person.first}}), Method: "GET", Enabled: person.first != ""},
		{ID: "timeline.task", Label: "New task", Href: s.postQuery("/app/tasks/quick", q), Method: "GET", Enabled: true},
	}
	if person.first == "" {
		page.Actions[1].Reason = "The card has no address to invite."
	}
	if task == nil {
		return page
	}
	snap := task.Snapshot()
	blob := s.catalogOf(c.sess)
	page.Poll = s.pollBar(tf, blob, snap, timelineOwner(task))
	rows := s.timelineRows(tf, blob, snap, person)
	counts := map[view.Kind]int{}
	for _, r := range rows {
		counts[r.row.Kind]++
	}
	current := timelineShows[0]
	for _, sh := range timelineShows {
		if sh.value == show {
			current = sh
		}
	}
	for _, sh := range timelineShows {
		n := len(rows)
		if sh.kind != "" {
			n = counts[sh.kind]
		}
		href := withQuery(here, "show", sh.value)
		page.Bar.Segments = append(page.Bar.Segments, view.Action{ID: "timeline.show." + sh.value, Label: text(fmt.Sprintf("%s · %d", sh.label, n)),
			Href: s.route(href), Method: "GET", Enabled: sh.value != current.value})
	}
	for _, r := range rows {
		if current.kind != "" && r.row.Kind != current.kind {
			continue
		}
		rubric := timelineUndated
		if !r.at.IsZero() {
			rubric = r.at.In(tf.tz).Format("January 2006")
		}
		if n := len(page.Groups); n == 0 || string(page.Groups[n-1].Rubric) != rubric {
			page.Groups = append(page.Groups, view.TimelineGroup{Rubric: text(rubric)})
		}
		g := &page.Groups[len(page.Groups)-1]
		g.Rows = append(g.Rows, r.row)
	}
	page.Bar.Loaded = len(rows)
	return page
}

// timelineInset is the card's inset: the newest rows of the timeline this
// session gathered for the person, nothing when it gathered none (S-15.4).
func (s *Server) timelineInset(c *collectionCtx, uid string, obj *model.Object) []view.TimelineRow {
	task := keptTimeline(c.sess, c.conn.ID, c.col.Path, uid, time.Time{})
	if task == nil {
		return nil
	}
	var out []view.TimelineRow
	for _, r := range s.timelineRows(c.tf, s.catalogOf(c.sess), task.Snapshot(), personOf(uid, obj)) {
		if len(out) == timelineInsetRows {
			break
		}
		out = append(out, r.row)
	}
	return out
}
