package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// kindSpec ties a C-5.4 {kind} segment to the records it holds.
type kindSpec struct {
	segment   string
	kind      view.Kind
	component string // iCalendar main component; empty for contacts
	ext       string
	noun      string
}

var recordKinds = map[string]kindSpec{
	"contacts": {segment: "contacts", kind: view.KindContact, ext: ".vcf", noun: "contact"},
	"calendar": {segment: "calendar", kind: view.KindEvent, component: "VEVENT", ext: ".ics", noun: "event"},
	"tasks":    {segment: "tasks", kind: view.KindTask, component: "VTODO", ext: ".ics", noun: "task"},
	"notes":    {segment: "notes", kind: view.KindNote, component: "VJOURNAL", ext: ".ics", noun: "note"},
}

func segmentOf(k view.Kind) string {
	for seg, spec := range recordKinds {
		if spec.kind == k {
			return seg
		}
	}
	return ""
}

// collectionCtx is one owned collection opened for a request: the catalog
// entry, its connection, and a provider over a guarded client. A single
// collection is not a fanout (S-11): nothing here polls.
type collectionCtx struct {
	sess *session.Session
	spec kindSpec
	conn sealed.Connection
	col  sealed.Collection
	card *card.Provider
	cal  *cal.Provider
	tf   timeFmt
}

// openCollection checks ownership and looks the collection up in the
// catalog. A collection that does not hold records of this kind answers 404,
// the same as a foreign one.
func (s *Server) openCollection(w http.ResponseWriter, r *http.Request, segment string, withUID bool) (*collectionCtx, bool) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return nil, false
	}
	spec, known := recordKinds[segment]
	if !known {
		s.notFound(w, r)
		return nil, false
	}
	account, col := r.PathValue("account"), r.PathValue("col")
	uid := ""
	if withUID {
		uid = r.PathValue("uid")
	}
	if !s.checkOwnership(w, r, sess, account, col, uid) {
		return nil, false
	}
	key, live := sess.DataKey()
	blob, opened := s.users.Catalog(sess.AccountID(), key)
	conn, hasConn := blob.Connection(account)
	entry, hasCol := blob.Collection(account, col)
	if !live || !opened || !hasConn || !hasCol || !entry.Has(spec.kind) {
		s.notFound(w, r)
		return nil, false
	}
	client, err := dav.NewClient(s.guard, conn.Server, conn.Login, conn.Password, conn.CertFingerprint)
	if err != nil {
		s.notFound(w, r)
		return nil, false
	}
	c := &collectionCtx{sess: sess, spec: spec, conn: conn, col: entry, tf: s.timeFor(sess)}
	if spec.kind == view.KindContact {
		c.card, err = card.New(client)
	} else {
		c.cal, err = cal.New(client)
	}
	if err != nil {
		s.notFound(w, r)
		return nil, false
	}
	return c, true
}

func (c *collectionCtx) listPath() string {
	return "/app/" + c.spec.segment + "/" + c.conn.ID + "/" + url.PathEscape(c.col.Path)
}

func (c *collectionCtx) recordPath(uid string) string {
	return c.listPath() + "/" + url.PathEscape(uid)
}

// kindRecordPath is a record of another kind in the same calendar collection.
func (c *collectionCtx) kindRecordPath(k view.Kind, uid string) string {
	return "/app/" + segmentOf(k) + "/" + c.conn.ID + "/" + url.PathEscape(c.col.Path) + "/" + url.PathEscape(uid)
}

func (c *collectionCtx) stamp() view.Stamp {
	return stamp(serverHost(c.conn.Server), collectionLabel(c.col))
}

func (c *collectionCtx) source() view.SourceBlock {
	return view.SourceBlock{Stamp: c.stamp(), Colour: catalogColour(c.col), ReadOnly: c.col.ReadOnly}
}

func (c *collectionCtx) find(ctx context.Context, uid string) (*model.Object, error) {
	if c.card != nil {
		return c.card.FindUID(ctx, c.col.Path, uid)
	}
	return c.cal.FindUID(ctx, c.col.Path, c.spec.component, uid)
}

func (c *collectionCtx) update(ctx context.Context, obj *model.Object) error {
	var err error
	if c.card != nil {
		_, err = c.card.Update(ctx, obj)
	} else {
		_, err = c.cal.Update(ctx, obj)
	}
	return err
}

func (c *collectionCtx) create(ctx context.Context, path string, obj *model.Object) error {
	var err error
	if c.card != nil {
		_, err = c.card.Create(ctx, path, obj)
	} else {
		_, err = c.cal.Create(ctx, path, obj)
	}
	return err
}

func (c *collectionCtx) remove(ctx context.Context, path, etag string) error {
	if c.card != nil {
		return c.card.Delete(ctx, path, etag)
	}
	return c.cal.Delete(ctx, path, etag)
}

// text is a property value for display and for the form: vCard values arrive
// unescaped, iCalendar TEXT is unescaped here.
func (c *collectionCtx) text(obj *model.Object, prop string) string {
	if c.card != nil {
		return firstValue(obj, prop)
	}
	return cal.Text(obj, prop)
}

func (c *collectionCtx) body(obj *model.Object) ([]byte, error) {
	if c.card != nil {
		var buf bytes.Buffer
		if err := card.Encode(&buf, obj); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	return obj.CalendarDoc(), nil
}

func (c *collectionCtx) parse(path, etag string, body []byte) (*model.Object, error) {
	if c.card != nil {
		return card.Parse(path, etag, bytes.NewReader(body))
	}
	return cal.Parse(path, etag, bytes.NewReader(body))
}

func notFoundErr(err error) bool {
	return errors.Is(err, card.ErrNotFound) || errors.Is(err, cal.ErrNotFound)
}

// indexEntry is one record of a list this session loaded. RELATED-TO resolves
// against these entries and nothing else: no second REPORT, no other
// collection (S-11).
type indexEntry struct {
	UID     string    `json:"uid"`
	Path    string    `json:"path"`
	ETag    string    `json:"etag"`
	Kind    view.Kind `json:"kind"`
	Title   string    `json:"title"`
	Related []string  `json:"related,omitempty"`
}

func (c *collectionCtx) indexKey(component string) string {
	return c.col.Path + "\x00" + component
}

// components are the index slots of the collection: one per record kind a
// calendar collection can hold, one for an address book.
func (c *collectionCtx) components() []string {
	if c.card != nil {
		return []string{""}
	}
	return []string{"VEVENT", "VTODO", "VJOURNAL"}
}

func (c *collectionCtx) putIndex(entries []indexEntry) {
	data, err := json.Marshal(entries)
	if err != nil {
		return
	}
	c.sess.Cache().PutETagMap(c.conn.ID, c.indexKey(c.spec.component), data)
}

func (c *collectionCtx) index() []indexEntry {
	var out []indexEntry
	for _, comp := range c.components() {
		data, ok := c.sess.Cache().GetETagMap(c.conn.ID, c.indexKey(comp))
		if !ok {
			continue
		}
		var part []indexEntry
		if json.Unmarshal(data, &part) == nil {
			out = append(out, part...)
		}
	}
	return out
}

// dropIndex forgets the loaded lists after a write: versions in them are stale.
func (c *collectionCtx) dropIndex() {
	for _, comp := range c.components() {
		c.sess.Cache().InvalidatePath(c.conn.ID, c.indexKey(comp))
	}
}

// rememberBody keeps the version an edit form was read at, so a rejected
// write can tell what the person changed from what the server changed (S-8).
func (s *Server) rememberBody(c *collectionCtx, obj *model.Object) {
	if body, err := c.body(obj); err == nil {
		c.sess.Cache().PutBody(c.conn.ID, obj.Path, obj.ETag, body)
	}
}

func (s *Server) readVersion(c *collectionCtx, path, etag string) (*model.Object, bool) {
	body, ok := c.sess.Cache().GetBody(c.conn.ID, path, etag)
	if !ok {
		return nil, false
	}
	obj, err := c.parse(path, etag, body)
	return obj, err == nil
}

// unavailableNotice is a source failure on the screen (C-9.1, S-21): what
// failed, the server's answer, and a retry of this source only.
func (s *Server) unavailableNotice(c *collectionCtx, retry string, err error) view.Notice {
	return view.Notice{
		Role:  view.NoticeAlert,
		Title: text(collectionLabel(c.col) + " is unavailable"),
		Body:  text("The server did not answer as expected, so nothing on this screen comes from it."),
		Details: &view.Diagnostic{
			Summary: text("REPORT " + c.col.Path),
			Lines:   []view.Text{text(serverHost(c.conn.Server)), text(err.Error())},
		},
		Actions: []view.Action{{ID: "source.retry", Label: "Try again", Href: s.route(retry), Method: "GET", Enabled: true}},
	}
}

func unreadableNotice(bad []cal.Unreadable) (view.Notice, bool) {
	if len(bad) == 0 {
		return view.Notice{}, false
	}
	n := view.Notice{
		Role:    view.NoticeAlert,
		Title:   text(plural(len(bad), "record") + " could not be read"),
		Body:    text("The server returned them, but they are not valid iCalendar. They are left as they are; other clients may still open them."),
		Details: &view.Diagnostic{Summary: text("Unreadable records")},
	}
	for _, b := range bad {
		n.Details.Lines = append(n.Details.Lines, text(b.Path+" · "+b.Err.Error()))
	}
	return n, true
}

// writeFailure is a refused write: nothing saved, the form keeps its values.
// A 403 says what it means (C-9.1).
func (s *Server) writeFailure(c *collectionCtx, method, path string, err error) view.Notice {
	n := view.Notice{
		Role:  view.NoticeAlert,
		Title: text("The server refused the change"),
		Body:  text("Nothing was saved. The form keeps what you entered."),
		Details: &view.Diagnostic{
			Summary: text(method + " " + path),
			Lines:   []view.Text{text(serverHost(c.conn.Server) + path), text(err.Error())},
		},
	}
	if dav.IsStatus(err, http.StatusForbidden) {
		n.Title = text("This account can read here but not write")
	}
	return n
}

func (s *Server) newRecordAction(c *collectionCtx) view.Action {
	a := view.Action{
		ID: "record.new", Label: text("New " + c.spec.noun),
		Href: s.route(c.listPath() + "/new"), Method: "GET", Enabled: !c.col.ReadOnly,
	}
	if c.col.ReadOnly {
		a.Reason = "This collection is read-only."
	}
	return a
}

func retryPath(c *collectionCtx, r *http.Request) string {
	if r.URL.RawQuery != "" {
		return c.listPath() + "?" + r.URL.RawQuery
	}
	return c.listPath()
}

func (s *Server) getKindList(w http.ResponseWriter, r *http.Request, kind string) {
	c, ok := s.openCollection(w, r, kind, false)
	if !ok {
		return
	}
	var page any
	switch c.spec.kind {
	case view.KindContact:
		page = s.contactsList(r, c)
	case view.KindEvent:
		page = s.agenda(r, c)
	case view.KindTask:
		page = s.tasksList(r, c)
	default:
		page = s.notesList(r, c)
	}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) contactsList(r *http.Request, c *collectionCtx) view.ContactsListPage {
	page := view.ContactsListPage{Shell: s.buildShell(c.sess, collectionLabel(c.col))}
	page.Bar.Actions = []view.Action{s.newRecordAction(c)}
	objs, err := c.card.Query(r.Context(), c.col.Path)
	if err != nil {
		page.Notices = append(page.Notices, s.unavailableNotice(c, retryPath(c, r), err))
		return page
	}
	names := make(map[*model.Object]string, len(objs))
	for _, o := range objs {
		names[o] = contactName(o)
	}
	sort.SliceStable(objs, func(i, j int) bool {
		ri, rj := rubricOf(names[objs[i]]), rubricOf(names[objs[j]])
		if ri != rj {
			// "#" gathers everything that does not start with a letter, last.
			return rj == "#" || (ri != "#" && ri < rj)
		}
		if a, b := strings.ToLower(names[objs[i]]), strings.ToLower(names[objs[j]]); a != b {
			return a < b
		}
		return card.UID(objs[i]) < card.UID(objs[j])
	})
	entries := make([]indexEntry, 0, len(objs))
	for _, o := range objs {
		uid, name := card.UID(o), names[o]
		row := view.ContactRow{
			Href:   s.route(c.recordPath(uid)),
			UID:    uid,
			Avatar: avatarFor(uid, name),
			Name:   text(name),
			Phone:  text(firstValue(o, "TEL")),
			Email:  text(firstValue(o, "EMAIL")),
			Org:    text(structured(firstValue(o, "ORG"))),
			State:  view.RecordOK,
		}
		rubric := rubricOf(name)
		if n := len(page.Groups); n == 0 || string(page.Groups[n-1].Rubric) != rubric {
			page.Groups = append(page.Groups, view.ContactGroup{Rubric: text(rubric)})
		}
		g := &page.Groups[len(page.Groups)-1]
		g.Rows = append(g.Rows, row)
		entries = append(entries, indexEntry{UID: uid, Path: o.Path, ETag: o.ETag, Kind: view.KindContact, Title: name})
	}
	c.putIndex(entries)
	total := len(objs)
	page.Bar.Loaded, page.Bar.Total = total, &total
	return page
}

// agenda is one week of one calendar in the instance zone. Without a window
// the calendar is never read whole (S-11); the window goes to the server as
// the REPORT time-range. Series are expanded here, and an instance another
// client rewrote (RECURRENCE-ID) is laid over the instant it replaces.
func (s *Server) agenda(r *http.Request, c *collectionCtx) view.AgendaPage {
	tf := c.tf
	tz := tf.tz
	from := tf.weekStart(time.Now().In(tz))
	if q := r.URL.Query().Get("from"); q != "" {
		if d, err := time.ParseInLocation("2006-01-02", q, tz); err == nil {
			from = d
		}
	}
	to := from.AddDate(0, 0, 7)

	page := view.AgendaPage{Shell: s.buildShell(c.sess, collectionLabel(c.col))}
	page.Bar.Actions = []view.Action{s.newRecordAction(c)}
	week := func(t time.Time) string { return s.route(c.listPath() + "?from=" + t.Format("2006-01-02")) }
	page.Bar.Range = &view.Range{
		From: text(tf.date(from)),
		To:   text(tf.date(to.AddDate(0, 0, -1))),
		Presets: []view.Action{
			{ID: "range.previous", Label: "Previous week", Href: week(from.AddDate(0, 0, -7)), Method: "GET", Enabled: true},
			{ID: "range.current", Label: "This week", Href: s.route(c.listPath()), Method: "GET", Enabled: true},
			{ID: "range.next", Label: "Next week", Href: week(to), Method: "GET", Enabled: true},
		},
	}

	objs, bad, err := c.cal.Query(r.Context(), c.col.Path, c.spec.component, &from, &to)
	if err != nil {
		page.Notices = append(page.Notices, s.unavailableNotice(c, retryPath(c, r), err))
		return page
	}

	type occurrence struct {
		at     time.Time
		allDay bool
		row    view.AgendaRow
	}
	var occs []occurrence
	entries := make([]indexEntry, 0, len(objs))
	for _, o := range objs {
		uid := cal.UID(o)
		summary := titleOr(cal.Text(o, "SUMMARY"), "Untitled event")
		entries = append(entries, indexEntry{UID: uid, Path: o.Path, ETag: o.ETag, Kind: view.KindEvent, Title: summary, Related: cal.RelatedTo(o)})

		tm, err := cal.TimingOf(o, tz)
		var instants []time.Time
		if err == nil {
			instants, err = cal.Expand(o, from, to, tz)
		}
		var overrides []cal.Override
		if err == nil {
			overrides, err = cal.Overrides(o, tz)
		}
		if err != nil {
			bad = append(bad, cal.Unreadable{Path: o.Path, UID: uid, Err: err})
			continue
		}
		for _, at := range instants {
			if at.Before(from) || !at.Before(to) {
				continue
			}
			occ := occurrence{at: at, allDay: tm.AllDay, row: view.AgendaRow{
				Href:     s.route(c.recordPath(uid)),
				UID:      uid,
				Time:     text(tf.timeLabel(at, tm.AllDay)),
				Zone:     text(tf.zone(tm.Zone)),
				Summary:  text(summary),
				Location: text(cal.Text(o, "LOCATION")),
				State:    view.RecordOK,
			}}
			for _, ov := range overrides {
				if !ov.RecurrenceID.Equal(at) {
					continue
				}
				if !ov.Timing.Start.IsZero() {
					occ.at, occ.allDay = ov.Timing.Start, ov.Timing.AllDay
					occ.row.Time = text(tf.timeLabel(ov.Timing.Start, ov.Timing.AllDay))
					occ.row.Zone = text(tf.zone(ov.Timing.Zone))
				}
				if ov.Summary != "" {
					occ.row.Summary = text(ov.Summary)
				}
				if ov.Location != "" {
					occ.row.Location = text(ov.Location)
				}
				if ov.Cancelled {
					occ.row.Badges = append(occ.row.Badges, view.Badge{Label: "Cancelled", Role: view.BadgeAlert})
				}
			}
			occs = append(occs, occ)
		}
	}

	sort.SliceStable(occs, func(i, j int) bool {
		if di, dj := tf.dayKey(occs[i].at), tf.dayKey(occs[j].at); di != dj {
			return di < dj
		}
		if occs[i].allDay != occs[j].allDay {
			return occs[i].allDay
		}
		if !occs[i].at.Equal(occs[j].at) {
			return occs[i].at.Before(occs[j].at)
		}
		return occs[i].row.Summary < occs[j].row.Summary
	})
	for _, o := range occs {
		rubric := tf.dayRubric(o.at)
		if n := len(page.Days); n == 0 || string(page.Days[n-1].Rubric) != rubric {
			page.Days = append(page.Days, view.AgendaDay{Rubric: text(rubric)})
		}
		d := &page.Days[len(page.Days)-1]
		d.Rows = append(d.Rows, o.row)
	}
	if n, ok := unreadableNotice(bad); ok {
		page.Notices = append(page.Notices, n)
	}
	c.putIndex(entries)
	page.Bar.Loaded = len(occs)
	return page
}

func (s *Server) tasksList(r *http.Request, c *collectionCtx) view.TasksPage {
	tf, now := c.tf, time.Now()
	tz := tf.tz
	show := r.URL.Query().Get("show")
	if show != "done" && show != "all" {
		show = "open"
	}
	page := view.TasksPage{Shell: s.buildShell(c.sess, collectionLabel(c.col))}
	page.Bar.Actions = []view.Action{s.newRecordAction(c)}
	objs, bad, err := c.cal.Query(r.Context(), c.col.Path, c.spec.component, nil, nil)
	if err != nil {
		page.Notices = append(page.Notices, s.unavailableNotice(c, retryPath(c, r), err))
		return page
	}

	type item struct {
		row view.TaskRow
		due time.Time
	}
	var items []item
	var open, done int
	entries := make([]indexEntry, 0, len(objs))
	for _, o := range objs {
		uid := cal.UID(o)
		summary := titleOr(cal.Text(o, "SUMMARY"), "Untitled task")
		entries = append(entries, indexEntry{UID: uid, Path: o.Path, ETag: o.ETag, Kind: view.KindTask, Title: summary, Related: cal.RelatedTo(o)})
		isDone := taskDone(o)
		if isDone {
			done++
		} else {
			open++
		}
		if (show == "open" && isDone) || (show == "done" && !isDone) {
			continue
		}
		due, err := cal.DueOf(o, tz)
		if err != nil {
			bad = append(bad, cal.Unreadable{Path: o.Path, UID: uid, Err: err})
			continue
		}
		row := view.TaskRow{
			Href: s.route(c.recordPath(uid)), UID: uid, Done: isDone, Summary: text(summary),
			Percent: percentOf(o), Tags: tagsOf(cal.TextList(o, "CATEGORIES")), State: view.RecordOK,
		}
		if !due.Start.IsZero() {
			row.Due = text(tf.timeOrDate(due))
			row.Overdue = !isDone && due.Start.Before(now)
		}
		items = append(items, item{row: row, due: due.Start})
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.row.Done != b.row.Done {
			return !a.row.Done
		}
		if a.due.IsZero() != b.due.IsZero() {
			return !a.due.IsZero()
		}
		if !a.due.Equal(b.due) {
			return a.due.Before(b.due)
		}
		return a.row.Summary < b.row.Summary
	})
	if len(items) > 0 {
		group := view.TaskGroup{}
		for _, it := range items {
			group.Rows = append(group.Rows, it.row)
		}
		page.Groups = []view.TaskGroup{group}
	}
	segment := func(id, label, value string, n int) view.Action {
		a := view.Action{ID: id, Label: text(fmt.Sprintf("%s · %d", label, n)), Href: s.route(c.listPath() + "?show=" + value), Method: "GET", Enabled: show != value}
		if show == value {
			a.Reason = "Shown now."
		}
		return a
	}
	page.Bar.Segments = []view.Action{
		segment("tasks.open", "Open", "open", open),
		segment("tasks.done", "Done", "done", done),
		segment("tasks.all", "All", "all", open+done),
	}
	if n, ok := unreadableNotice(bad); ok {
		page.Notices = append(page.Notices, n)
	}
	c.putIndex(entries)
	page.Bar.Loaded = len(items)
	return page
}

func (s *Server) notesList(r *http.Request, c *collectionCtx) view.NotesPage {
	tf := c.tf
	tz := tf.tz
	page := view.NotesPage{Shell: s.buildShell(c.sess, collectionLabel(c.col))}
	page.Bar.Actions = []view.Action{s.newRecordAction(c)}
	objs, bad, err := c.cal.Query(r.Context(), c.col.Path, c.spec.component, nil, nil)
	if err != nil {
		page.Notices = append(page.Notices, s.unavailableNotice(c, retryPath(c, r), err))
		return page
	}
	type item struct {
		row view.NoteRow
		at  time.Time
	}
	items := make([]item, 0, len(objs))
	entries := make([]indexEntry, 0, len(objs))
	for _, o := range objs {
		uid, body := cal.UID(o), cal.Description(o)
		title := noteTitle(cal.Text(o, "SUMMARY"), body)
		entries = append(entries, indexEntry{UID: uid, Path: o.Path, ETag: o.ETag, Kind: view.KindNote, Title: title, Related: cal.RelatedTo(o)})
		at := noteTime(o, tz)
		row := view.NoteRow{
			Href: s.route(c.recordPath(uid)), UID: uid, Title: text(title), Excerpt: text(noteExcerpt(body, title)),
			Tags: tagsOf(cal.TextList(o, "CATEGORIES")), State: view.RecordOK,
		}
		if !at.IsZero() {
			row.Date = text(tf.date(at))
		}
		items = append(items, item{row: row, at: at})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].at.Equal(items[j].at) {
			return items[i].at.After(items[j].at)
		}
		return items[i].row.Title < items[j].row.Title
	})
	if len(items) > 0 {
		group := view.NoteGroup{}
		for _, it := range items {
			group.Rows = append(group.Rows, it.row)
		}
		page.Groups = []view.NoteGroup{group}
	}
	if n, ok := unreadableNotice(bad); ok {
		page.Notices = append(page.Notices, n)
	}
	c.putIndex(entries)
	page.Bar.Loaded = len(items)
	return page
}

func firstValue(obj *model.Object, prop string) string {
	if v := obj.PropText(prop); len(v) > 0 {
		return v[0]
	}
	return ""
}

// structured shows a vCard structured value (ADR, ORG, N) as its non-empty
// parts.
func structured(v string) string {
	var parts []string
	for _, p := range strings.Split(v, ";") {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, ", ")
}

func contactName(obj *model.Object) string {
	if v := strings.TrimSpace(firstValue(obj, "FN")); v != "" {
		return v
	}
	if n := strings.Split(firstValue(obj, "N"), ";"); len(n) > 1 {
		if v := strings.TrimSpace(strings.TrimSpace(n[1]) + " " + strings.TrimSpace(n[0])); v != "" {
			return v
		}
	}
	if v := firstValue(obj, "EMAIL"); v != "" {
		return v
	}
	return card.UID(obj)
}

func rubricOf(name string) string {
	r, _ := utf8.DecodeRuneInString(strings.TrimSpace(name))
	if unicode.IsLetter(r) {
		return string(unicode.ToUpper(r))
	}
	return "#"
}

func initials(name string) string {
	words := strings.Fields(name)
	var out []rune
	for i, w := range words {
		if i != 0 && i != len(words)-1 {
			continue
		}
		if r, _ := utf8.DecodeRuneInString(w); unicode.IsLetter(r) {
			out = append(out, unicode.ToUpper(r))
		}
	}
	return string(out)
}

// avatarFor is the placeholder: initials on a colour derived from the UID.
func avatarFor(uid, name string) view.Avatar {
	return view.Avatar{Initials: text(initials(name)), Colour: collectionColour(uid)}
}

func titleOr(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func taskDone(obj *model.Object) bool {
	return strings.EqualFold(cal.Text(obj, "STATUS"), "COMPLETED") || len(obj.PropText("COMPLETED")) > 0
}

func percentOf(obj *model.Object) *int {
	n, err := strconv.Atoi(strings.TrimSpace(firstValue(obj, "PERCENT-COMPLETE")))
	if err != nil {
		return nil
	}
	return &n
}

func tagsOf(values []string) []view.Tag {
	var out []view.Tag
	for _, v := range values {
		out = append(out, view.Tag{Label: text(v)})
	}
	return out
}

// noteTitle is SUMMARY, else the first line of the body without its heading
// marks.
func noteTitle(summary, body string) string {
	if strings.TrimSpace(summary) != "" {
		return summary
	}
	for _, line := range strings.Split(body, "\n") {
		if line = strings.TrimSpace(strings.TrimLeft(line, "# ")); line != "" {
			return line
		}
	}
	return "Untitled note"
}

// noteExcerpt is the first line of the body that is not the title. The
// widget shortens it (C-10); the server does not.
func noteExcerpt(body, title string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(line, "# "))
		if line != "" && line != title {
			return line
		}
	}
	return ""
}

func noteTime(obj *model.Object, tz *time.Location) time.Time {
	for _, prop := range []string{"DTSTART", "LAST-MODIFIED", "DTSTAMP"} {
		if t, ok := cal.TimeOf(obj, prop, tz); ok {
			return t
		}
	}
	return time.Time{}
}
