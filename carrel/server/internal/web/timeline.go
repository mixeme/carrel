package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/fanout"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// The person's timeline (S-15.4): a search for one person over the ticked
// sources. A record matches by an address of the card in ORGANIZER or
// ATTENDEE, by an address of theirs standing behind one of those lines in
// SENT-BY, DELEGATED-FROM or DELEGATED-TO, or by RELATED-TO the card's UID
// — each way a personMatcher of timelineMatchers; a file by the person's
// name in its name. Every component of a record is
// read, not only the main one: an overridden occurrence of a series has an
// organiser and attendees of its own. It is gathered only when opened; the card's inset shows what this
// session gathered.

const (
	timelineBack      = 12 // months before today the window starts
	timelineAhead     = 3  // months after today it ends
	timelineInsetRows = 3
	timelineUndated   = "Without a date"
)

// timelineTask is the fanout of one person's timeline over one window.
type timelineTask struct {
	*fanout.Task
	account, col, uid string
	from, last        time.Time
	page              string
	person            timelinePerson
}

// timelinePerson is what a record is matched by.
type timelinePerson struct {
	uid    string
	name   string
	emails map[string]bool // lower case, without mailto:
	first  string          // the first address of the card, for New event
	// addresses are the card's addresses in its order, as the bar says them
	addresses []string
}

// has says whether an address is this person's.
func (p timelinePerson) has(addr string) bool {
	return addr != "" && p.emails[strings.ToLower(addr)]
}

// throughSomeoneElse says how this person stands behind another's line, or
// "" when they do not. The wording names the other person's part, because
// that is what the row has to explain: «sent for the organiser» is a
// different fact from «organiser».
func (p timelinePerson) throughSomeoneElse(a cal.Attendee, part string) string {
	if p.has(a.SentBy) {
		return "sent for " + part
	}
	for _, d := range a.DelegatedFrom {
		if p.has(d) {
			return "delegated this to " + part
		}
	}
	for _, d := range a.DelegatedTo {
		if p.has(d) {
			return "given this by " + part
		}
	}
	return ""
}

func timelineOwner(tt *timelineTask) pollOwner {
	return pollOwner{
		page:       tt.page,
		again:      withQuery(tt.page, "again", "1"),
		againLabel: "Look again",
		what:       "timeline",
		polled:     true,
		found:      func(n int) string { return plural(n, "record") },
		matched:    append([]string{}, tt.person.addresses...),
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

// Run asks the collection for each of its kinds, side by side and each
// under a slot of the fanout's ceiling, for the reason given on
// collectionSource.Run: the same number of requests may be in flight as
// before, and the person waits for the slowest kind instead of all of
// them in a row.
func (ts *timelineSource) Run(ctx context.Context, q fanout.Query) ([]fanout.Item, error) {
	c := ts.collectionSource
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
				return ts.askKind(ctx, k, q)
			})
		}(i, k)
	}
	wg.Wait()

	var items []fanout.Item
	var note error
	for i := range c.kinds {
		for _, it := range found[i] {
			if it.Path != "" {
				items = append(items, it)
			}
		}
		if err := errs[i]; err != nil {
			if errors.Is(err, fanout.ErrIncomplete) {
				note = err
				continue
			}
			return nil, err
		}
	}
	return items, note
}

// askKind is one kind of the collection under one slot. A kind this
// screen does not read — a contact, or a file when the person has no
// name to look for — takes no slot at all: it asks nobody.
func (ts *timelineSource) askKind(ctx context.Context, k view.Kind, q fanout.Query) ([]fanout.Item, error) {
	c := ts.collectionSource
	if k == view.KindContact || (k == view.KindFile && ts.person.name == "") {
		return nil, nil
	}
	release, err := fanout.Slot(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	if k == view.KindFile {
		found, err := c.findFiles(ctx, fanout.Query{Text: ts.person.name})
		for i := range found {
			if at := found[i].FileModified; !at.IsZero() && (at.Before(*q.From) || at.After(*q.To)) {
				found[i] = fanout.Item{}
			}
		}
		return found, err
	}
	window := fanout.Query{}
	if k == view.KindEvent {
		window.From, window.To = q.From, q.To
	}
	found, err := c.records(ctx, k, window)
	for i := range found {
		if _, at, ok := ts.person.match(k, found[i].Object, ts.tz); !ok || !at.IsZero() && (at.Before(*q.From) || at.After(*q.To)) {
			found[i] = fanout.Item{}
		}
	}
	return found, err
}

// match says why a record points to the person and when it stands; the
// strongest reason when there are several. Zero time: the record has no date.
func (p timelinePerson) match(k view.Kind, obj *model.Object, tz *time.Location) (string, time.Time, bool) {
	if obj == nil {
		return "", time.Time{}, false
	}
	// Everyone the record names, in every component it has: an overridden
	// occurrence carries its own organiser and attendees, and someone
	// added to one Thursday of a series is named nowhere else.
	rec := timelineRecord{obj: obj, people: cal.People(obj)}
	for _, m := range timelineMatchers {
		if why := m.why(p, rec); why != "" {
			return why, timelineWhen(k, obj, tz), true
		}
	}
	return "", time.Time{}, false
}

// personMatcher is one way a record points to a person: why says how, in
// the words of the row's Why, or "" when the record does not point to them
// this way. The timeline asks timelineMatchers in order, strongest first,
// and the first answer is the row's. A new way — the person named in the
// record's text, which Mentions will read — is one more matcher in the
// list, not another branch of match.
type personMatcher interface {
	why(p timelinePerson, rec timelineRecord) string
}

// timelineRecord is a record as the matchers read it: the object, and the
// people it names in every component, read once for all of them.
type timelineRecord struct {
	obj    *model.Object
	people []cal.Person
}

// timelineMatchers are the ways a record points to a person, strongest
// first (S-15.4): a line of their own, a line they stand behind, a link to
// their card.
var timelineMatchers = []personMatcher{ownLine{}, behindALine{}, relatedToCard{}}

// ownLine is the person's address in ORGANIZER or ATTENDEE: «organiser», or
// «attendee» with their answer when they gave one.
type ownLine struct{}

func (ownLine) why(p timelinePerson, rec timelineRecord) string {
	for _, who := range rec.people {
		if !p.has(who.Email) {
			continue
		}
		if who.Organiser {
			return "organiser"
		}
		why := "attendee"
		if st := strings.ToLower(who.Status); st != "" && st != "needs-action" {
			why += " · " + st
		}
		return why
	}
	return ""
}

// behindALine is the person standing behind another's line. A person can
// be in a record without being on a line of their own. SENT-BY is who
// really sent it — the assistant who put the meeting in for someone else —
// and DELEGATED-FROM names the one who handed participation over, who is
// often struck from the attendee list in the same breath. Reading only
// ORGANIZER and ATTENDEE left both of them with an empty timeline over a
// record that names them (RFC 5545 §3.2.18, §3.2.4, §3.2.5).
type behindALine struct{}

func (behindALine) why(p timelinePerson, rec timelineRecord) string {
	for _, who := range rec.people {
		part := "an attendee"
		if who.Organiser {
			part = "the organiser"
		}
		if w := p.throughSomeoneElse(who.Attendee, part); w != "" {
			return w
		}
	}
	return ""
}

// relatedToCard is RELATED-TO the card's UID: a task or a note made from
// the person's timeline, or tied to them by hand.
type relatedToCard struct{}

func (relatedToCard) why(p timelinePerson, rec timelineRecord) string {
	for _, rel := range cal.RelatedTo(rec.obj) {
		if rel == p.uid {
			return "related to this person"
		}
	}
	return ""
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
			if !p.emails[e] {
				p.addresses = append(p.addresses, e)
			}
			p.emails[e] = true
			if p.first == "" {
				p.first = e
			}
		}
	}
	return p
}

// timelineWindow is the window of the address, its first and its last day:
// from ?from=, else twelve months back; to ?to=, else three months ahead of
// today. The range on the strip sets both (s-contacts-7); a last day before
// the first is not a window, and the default stands.
func timelineWindow(r *http.Request, tz *time.Location, now time.Time) (from, last time.Time) {
	today := dayOf(now, tz)
	from, last = today.AddDate(0, -timelineBack, 0), today.AddDate(0, timelineAhead, 0)
	if v, err := time.ParseInLocation("2006-01-02", r.URL.Query().Get("from"), tz); err == nil && v.Before(today) {
		from = v
	}
	if v, err := time.ParseInLocation("2006-01-02", r.URL.Query().Get("to"), tz); err == nil && !v.Before(from) {
		last = v
	}
	return from, last
}

func dayOf(t time.Time, tz *time.Location) time.Time {
	t = t.In(tz)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, tz)
}

// timelineHere is the address of the timeline over a window: the default one
// has neither from nor to.
func timelineHere(card string, from, last, now time.Time, tz *time.Location) string {
	here := card + "/timeline"
	if !from.Equal(dayOf(now, tz).AddDate(0, -timelineBack, 0)) {
		here = withQuery(here, "from", from.Format("2006-01-02"))
	}
	if !last.Equal(dayOf(now, tz).AddDate(0, timelineAhead, 0)) {
		here = withQuery(here, "to", last.Format("2006-01-02"))
	}
	return here
}

// keptTimeline is this session's newest timeline of the person over the
// window; any window when from is zero (the card's inset).
func keptTimeline(sess *session.Session, account, col, uid string, from, last time.Time) *timelineTask {
	for _, t := range sess.Tasks() {
		if tt, ok := t.(*timelineTask); ok && tt.account == account && tt.col == col && tt.uid == uid &&
			(from.IsZero() || tt.from.Equal(from) && tt.last.Equal(last)) {
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
	from, last := timelineWindow(r, tz, s.now())
	person := personOf(uid, obj)
	card := c.recordPath(uid)
	page := timelineHere(card, from, last, s.now(), tz)

	var task *timelineTask
	if r.URL.Query().Get("again") == "" {
		task = keptTimeline(c.sess, c.conn.ID, c.col.Path, uid, from, last)
	} else {
		dropAllWholeReads(c.sess, s.catalogOf(c.sess)) // what the servers hold now (S-4.2)
	}
	var notices []view.Notice
	if task == nil {
		// The last day is in the window: the servers are asked up to the
		// midnight after it.
		task, err = s.startTimeline(c.sess, person, from, last.AddDate(0, 0, 1))
		switch {
		case err == nil:
			task.account, task.col, task.uid, task.from, task.last, task.page = c.conn.ID, c.col.Path, uid, from, last, page
			c.sess.KeepTask(task)
		case errors.Is(err, fanout.ErrBusy):
			notices = append(notices, view.Notice{Role: view.NoticeAlert, Title: "Too many searches",
				Body: text(fmt.Sprintf("This session already runs %d searches and lists. Stop one or wait for one to finish, then open the timeline again.", conf.MaxLiveFanouts))})
		default:
			http.Error(w, "could not start the timeline", http.StatusInternalServerError)
			return
		}
	}
	tl := s.timelinePage(r.Context(), c, obj, person, task, from, last, r.URL.Query().Get("kind"), notices)
	// Columns (s-contacts-25): Title, Date, Kind, Source; why a record is
	// here stands after its title always and is not a column (s-contacts-26).
	tl.Bar.Columns = s.columnsMenu(r, csrfField(c.sess), "timeline", card+"/timeline/columns", r.URL.Query(), false)
	// The panel's cross (s-contacts-7): without scripts the same timeline —
	// its range and kind — without the panel. The screen keeps its sense;
	// the other links of the screen do not carry card=0 and open it again.
	q := r.URL.Query()
	if q.Get("card") == "0" {
		tl.Card, tl.CardClosed = view.Details{}, true
	} else {
		q.Del("again") // closing asks the servers nothing
		q.Set("card", "0")
		tl.Card.Close = s.route(card+"/timeline") + "?" + q.Encode()
	}
	s.writePage(w, r, http.StatusOK, tl)
}

func (s *Server) startTimeline(sess *session.Session, person timelinePerson, from, to time.Time) (*timelineTask, error) {
	var sources []fanout.Source
	for _, cs := range s.tickedCollections(sess, s.catalogOf(sess)) {
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
		row := view.TimelineRow{Kind: it.Kind, Stamp: o.stamp(), Colour: catalogColour(o.col)}
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
			row.Target = render.TargetDetails // its card in the panel (s-contacts-7)
		}
		if !at.IsZero() {
			row.When = text(tf.shortDay(at)) // a list's day, «10 Aug» (s-contacts-7, -26; C-10)
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

// timelineRail is the rail of a person's timeline (s-contacts-7): the
// sections and the way back to the list, and under «Where to look» the
// calendars it looks in. Their ticks are the calendar's own — one selection
// for both (owner's decision 24.09.2026) — so what the rail shows is where
// the timeline looks. Each counts what it found of this person, the combined
// row their sum; a feed shows «ro» and is not in the sum; what did not answer
// says why, with Retry. Apply comes back here and asks again.
func (s *Server) timelineRail(c *collectionCtx, snap *fanout.Snapshot, back string) view.Rail {
	blob := s.catalogOf(c.sess)
	found := map[string]int{}
	if snap != nil {
		for _, st := range snap.Sources {
			if st.State == view.SourceDone || st.State == view.SourceEmpty {
				found[st.ID] = st.Count
			}
		}
	}
	count := func(id string) *int {
		if n, ok := found[id]; ok {
			return &n
		}
		return nil
	}
	sections, live, on := s.railSections(c.sess, blob, recordKinds["calendar"], s.troublesOf(snap), count)
	where := view.RailSection{Account: "Where to look", State: view.CollectionOK}
	combined := view.SourceRow{Href: s.route("/app/calendar"), Label: text(combinedTitles[view.KindEvent]),
		Tick: view.TickPart, State: view.CollectionOK}
	switch on {
	case 0:
		combined.Tick = view.TickOff
	case live:
		combined.Tick = view.TickOn
	}
	sum, answered := 0, false
	where.Rows = append(where.Rows, combined)
	for _, sec := range sections {
		for _, row := range sec.Rows {
			row.Menu = nil // what to do with a calendar is the calendar's rail
			if row.Count != nil && !row.Feed {
				sum, answered = sum+*row.Count, true
			}
			where.Rows = append(where.Rows, row)
		}
	}
	if answered {
		where.Rows[0].Count = &sum
	}
	// The timeline looks where the calendar's ticks say: the combined row is
	// the one open (s-contacts-7).
	where.Rows[0].On = true
	// The sections in full, as on every screen of the section, and the way
	// back after them in a group of its own (s-contacts-7): a timeline is a
	// list of records, not one record, so the rail keeps both.
	return view.Rail{
		Nav:      append(s.navRows(c.sess, blob, "contacts"), view.NavRow{Action: s.listBack(c)}),
		Sections: []view.RailSection{where},
		Foot:     []view.Action{},
		Ticks:    s.ticksForm(c.sess, "calendar", back),
		About:    s.aboutAction(),
		Davloom:  s.davloomMark(c.sess),
	}
}

// timelineShows are the kinds the timeline is filtered by, ?kind= as the
// frame's links write it (s-contacts-7): All is the timeline without it.
var timelineShows = []struct {
	label string
	kind  view.Kind
}{
	{"All", ""}, {"Events", view.KindEvent}, {"Tasks", view.KindTask},
	{"Notes", view.KindNote}, {"Files", view.KindFile},
}

func (s *Server) timelinePage(ctx context.Context, c *collectionCtx, obj *model.Object, person timelinePerson, task *timelineTask, from, last time.Time, kind string, notices []view.Notice) view.ContactTimelinePage {
	tf := c.tf
	card := c.recordPath(person.uid)
	sh := s.buildShell(c.sess, person.name+" · Timeline")
	sh.Rail = s.timelineRail(c, nil, card+"/timeline")
	sh.Notices = append(sh.Notices, notices...)
	page := view.ContactTimelinePage{Shell: sh, UID: person.uid, Name: text(person.name), Subtitle: text(contactSubtitle(obj))}
	// The card in the panel: Open the card and Edit on view, the deletion at
	// its foot, the rest under «⋯» — the menu of the card itself
	// (s-contacts-7, s-contacts-11).
	ops := s.recordOps(c.origin(), c.spec, person.uid, obj.ETag)
	page.Card = view.Details{Title: text(person.name), Source: c.source(), Menu: ops.menu,
		Actions: shown(&view.Action{ID: "timeline.card", Label: "Open the card", Href: s.route(card), Method: "GET", Enabled: true}, ops.edit, ops.del)}
	page.Card.Source.UID = text(person.uid)
	// The avatar of the head and the photograph of the panel (s-contacts-7).
	av := avatarFor(person.uid, person.name)
	av.Href, _ = s.cardPhoto(c, person.uid, obj)
	page.Card.Avatar = &av
	for _, f := range contactFieldsOf(obj) {
		// Where the person works and as what is the head's line, not the
		// panel's (s-contacts-7).
		if f.prop != "ORG" && f.prop != "TITLE" {
			page.Card.Fields = append(page.Card.Fields, f.Field)
		}
	}
	page.Card.Fields = withBirthdayAside(withTagLinks(page.Card.Fields, s.recordTags(c, obj)), c.tf)
	// A person in more than one book: «in 2 books» by the head, the others
	// in the panel (s-contacts-7, S-16).
	if lc, ok := s.linkedOf(ctx, c, person.uid); ok {
		page.Linked = &view.Badge{Label: text(fmt.Sprintf("in %d books", len(lc.others)+1)), Role: view.BadgeLinked}
		page.Card.Books = s.memberRows(lc, obj, differs(contactDiffers))
	}
	here := timelineHere(card, from, last, tf.now, tf.tz)
	// The range stays on the strip (s-contacts-7, s-contacts-14): the two
	// days of the window and Show. «Widen the range» in the note is
	// answered with it — there is no button of that name.
	form := s.plainForm(card+"/timeline", "GET",
		view.FormRow{Name: "from", Label: "From", Kind: view.InputDate, Value: text(from.Format("2006-01-02")), Mono: true},
		view.FormRow{Name: "to", Label: "To", Kind: view.InputDate, Value: text(last.Format("2006-01-02")), Mono: true},
	)
	form.Actions = []view.Action{{ID: "range.show", Label: "Show", Href: s.route(card + "/timeline"), Method: "GET", Enabled: true}}
	page.Range = view.Range{From: text(tf.date(from)), To: text(tf.date(last)), Presets: []view.Action{}, Form: &form}
	page.Note = text("Nothing was looked for before " + tf.date(from) + ". Widen the range if you need it.")
	q := url.Values{"related": {person.uid}}
	page.Actions = []view.Action{
		{ID: "timeline.note", Label: "Write a note", Icon: "i-quill", Href: s.postQuery("/app/notes/quick", q), Method: "GET", Primary: true, Enabled: true},
		// New event relates to the person, as the note and the task do, and
		// invites nobody (the owner's decision of 12.10.2026, А10 of the
		// audit of 11.10.2026: it carried the person as a hidden attendee,
		// and a server that sends invitations sent one). Who is invited is
		// said in the event's form.
		{ID: "timeline.event", Label: "New event", Icon: "i-cal", Href: s.postQuery("/app/calendar/quick", q), Method: "GET", Enabled: true},
		{ID: "timeline.task", Label: "New task", Icon: "i-task", Href: s.postQuery("/app/tasks/quick", q), Method: "GET", Enabled: true},
		{ID: "timeline.print", Label: "Print", Icon: "i-print", Href: s.route(card + "/timeline" + printQuery), Method: "GET", Enabled: true},
	}
	if task == nil {
		return page
	}
	snap := task.Snapshot()
	blob := s.catalogOf(c.sess)
	page.Rail = s.timelineRail(c, &snap, card+"/timeline")
	page.Poll = s.pollBar(tf, blob, snap, timelineOwner(task))
	rows := s.timelineRows(tf, blob, snap, person)
	counts := map[view.Kind]int{}
	for _, r := range rows {
		counts[r.row.Kind]++
	}
	current := timelineShows[0]
	for _, sh := range timelineShows {
		if string(sh.kind) == kind {
			current = sh
		}
	}
	for _, sh := range timelineShows {
		n, id, href := len(rows), "timeline.kind.all", here
		if sh.kind != "" {
			n, id, href = counts[sh.kind], "timeline.kind."+string(sh.kind), withQuery(here, "kind", string(sh.kind))
		}
		// «All 24» as the frame writes it, the label and its count apart:
		// a narrow strip folds the segment into a choice that writes them
		// apart (SegFold, from-typecase note 28). The one shown now is on
		// and stays a choice.
		page.Bar.Segments = append(page.Bar.Segments, view.Action{ID: id, Label: text(sh.label), Tally: text(strconv.Itoa(n)),
			Href: s.route(href), Method: "GET", Enabled: true, On: sh.kind == current.kind})
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
		g.Count = len(g.Rows)
	}
	page.Bar.Loaded = len(rows)
	return page
}

// timelineInset is the card's inset: the newest rows of the timeline this
// session gathered for the person and how many it found in all — the number
// in the inset's rubric (s-contacts-6); nothing when it gathered none
// (S-15.4).
func (s *Server) timelineInset(c *collectionCtx, uid string, obj *model.Object) ([]view.TimelineRow, *int) {
	task := keptTimeline(c.sess, c.conn.ID, c.col.Path, uid, time.Time{}, time.Time{})
	if task == nil {
		return nil, nil
	}
	var out []view.TimelineRow
	rows := s.timelineRows(c.tf, s.catalogOf(c.sess), task.Snapshot(), personOf(uid, obj))
	for _, r := range rows {
		if len(out) == timelineInsetRows {
			break
		}
		// On the card a row is a move to the record: the card has no panel.
		r.row.Target = ""
		out = append(out, r.row)
	}
	n := len(rows)
	return out, &n
}

// matchedSummary is the bar of a person's timeline in pieces (s-contacts-7):
// how many sources answered, and what the records were matched by — the
// card's addresses in mono, and the name, which files are looked for by.
// A source that did not answer is the bar's «N failed» (PollBar.Failed); one
// stopped by hand is counted after.
func matchedSummary(answered, total int, addresses []string, stopped int) view.Runs {
	out := view.Runs{{Text: text(fmt.Sprintf("Polled %d of %s · matched by ", answered, plural(total, "source")))}}
	for i, a := range addresses {
		if i > 0 {
			out = append(out, view.Run{Text: ", "})
		}
		out = append(out, view.Run{Text: text(a), Mark: view.RunMono})
	}
	if len(addresses) > 0 {
		out = append(out, view.Run{Text: " and the name"})
	} else {
		out = append(out, view.Run{Text: "the name"})
	}
	if stopped > 0 {
		out = append(out, view.Run{Text: text(fmt.Sprintf(" · %d stopped", stopped))})
	}
	return out
}
