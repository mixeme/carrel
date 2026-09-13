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

// origin is where a listed record lives: one catalog entry of one connection.
type origin struct {
	conn sealed.Connection
	col  sealed.Collection
}

func (o origin) listPath(segment string) string {
	return "/app/" + segment + "/" + o.conn.ID + "/" + url.PathEscape(o.col.Path)
}

func (o origin) recordPath(segment, uid string) string {
	return o.listPath(segment) + "/" + url.PathEscape(uid)
}

func (o origin) stamp() view.Stamp {
	return stamp(serverHost(o.conn.Server), collectionLabel(o.col))
}

// listed is one record of a list with its origin. The rows of a combined view
// carry the origin (S-14); the rows of a single collection do not repeat it.
type listed struct {
	obj  *model.Object
	from origin
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

func (c *collectionCtx) origin() origin { return origin{conn: c.conn, col: c.col} }

func (c *collectionCtx) listPath() string { return c.origin().listPath(c.spec.segment) }

func (c *collectionCtx) recordPath(uid string) string {
	return c.origin().recordPath(c.spec.segment, uid)
}

// kindRecordPath is a record of another kind in the same calendar collection.
func (c *collectionCtx) kindRecordPath(k view.Kind, uid string) string {
	return c.origin().recordPath(segmentOf(k), uid)
}

func (c *collectionCtx) stamp() view.Stamp { return c.origin().stamp() }

// listed pairs the objects of this collection with it.
func (c *collectionCtx) listed(objs []*model.Object) []listed {
	out := make([]listed, len(objs))
	for i, o := range objs {
		out[i] = listed{obj: o, from: c.origin()}
	}
	return out
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

// indexOf is the index of listed records of kind k.
func indexOf(k view.Kind, items []listed) []indexEntry {
	out := make([]indexEntry, 0, len(items))
	for _, it := range items {
		o := it.obj
		e := indexEntry{Path: o.Path, ETag: o.ETag, Kind: k}
		switch k {
		case view.KindContact:
			e.UID, e.Title = card.UID(o), contactName(o)
		case view.KindEvent:
			e.UID, e.Title, e.Related = cal.UID(o), titleOr(cal.Text(o, "SUMMARY"), "Untitled event"), cal.RelatedTo(o)
		case view.KindTask:
			e.UID, e.Title, e.Related = cal.UID(o), titleOr(cal.Text(o, "SUMMARY"), "Untitled task"), cal.RelatedTo(o)
		default:
			e.UID, e.Title, e.Related = cal.UID(o), noteTitle(cal.Text(o, "SUMMARY"), cal.Description(o)), cal.RelatedTo(o)
		}
		out = append(out, e)
	}
	return out
}

func putIndex(sess *session.Session, accountID, path, component string, entries []indexEntry) {
	data, err := json.Marshal(entries)
	if err != nil {
		return
	}
	sess.Cache().PutETagMap(accountID, path+"\x00"+component, data)
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
	putIndex(c.sess, c.conn.ID, c.col.Path, c.spec.component, entries)
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

// noteWrite keeps the loaded list of this kind in step with a write the
// server accepted: the record at path is replaced by obj, or leaves when obj
// is nil. A count once shown stays until the list is read again (C-6.3); the
// other entries are as that read left them. Nothing loaded, nothing kept.
func (c *collectionCtx) noteWrite(path string, obj *model.Object) {
	key := c.indexKey(c.spec.component)
	data, ok := c.sess.Cache().GetETagMap(c.conn.ID, key)
	if !ok {
		return
	}
	var entries []indexEntry
	if json.Unmarshal(data, &entries) != nil {
		c.sess.Cache().InvalidatePath(c.conn.ID, key)
		return
	}
	kept := entries[:0]
	for _, e := range entries {
		if e.Path != path {
			kept = append(kept, e)
		}
	}
	if obj != nil {
		kept = append(kept, indexOf(c.spec.kind, c.listed([]*model.Object{obj}))...)
	}
	c.putIndex(kept)
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

// unreadable is a record that could not be read, with the stamp of its
// collection when the list is combined.
type unreadable struct {
	cal.Unreadable
	stamp view.Stamp
}

func unreadableOf(bad []cal.Unreadable) []unreadable {
	out := make([]unreadable, len(bad))
	for i, b := range bad {
		out[i] = unreadable{Unreadable: b}
	}
	return out
}

func appendUnreadable(notices []view.Notice, bad []unreadable) []view.Notice {
	if len(bad) == 0 {
		return notices
	}
	n := view.Notice{
		Role:    view.NoticeAlert,
		Title:   text(plural(len(bad), "record") + " could not be read"),
		Body:    text("The server returned them, but they are not valid iCalendar. They are left as they are; other clients may still open them."),
		Details: &view.Diagnostic{Summary: text("Unreadable records")},
	}
	for _, b := range bad {
		line := b.Path + " · " + b.Err.Error()
		if b.stamp != "" {
			line = string(b.stamp) + " · " + line
		}
		n.Details.Lines = append(n.Details.Lines, text(line))
	}
	return append(notices, n)
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
	// The rail is built after the list: the list is what fills its counts.
	// The list leaves the tags of what it read in the shell's rail.
	rail := func(tags []view.Tag) view.Rail {
		rl := s.kindRail(c.sess, s.catalogOf(c.sess), c.spec, nil)
		rl.Tags = tags
		return rl
	}
	var page any
	switch c.spec.kind {
	case view.KindContact:
		p := s.contactsList(r, c)
		p.Rail = rail(p.Rail.Tags)
		page = p
	case view.KindEvent:
		p := s.agenda(r, c)
		p.Rail = rail(p.Rail.Tags)
		page = p
	case view.KindTask:
		p := s.tasksList(r, c)
		p.Rail = rail(p.Rail.Tags)
		page = p
	default:
		p := s.notesList(r, c)
		p.Rail = rail(p.Rail.Tags)
		page = p
	}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) contactsList(r *http.Request, c *collectionCtx) view.ContactsListPage {
	page := view.ContactsListPage{Shell: s.buildShell(c.sess, collectionLabel(c.col))}
	page.Bar.Actions = []view.Action{s.newRecordAction(c)}
	filter := s.tagFilter(c.listPath(), r.URL.Query())
	objs, err := c.card.Query(r.Context(), c.col.Path)
	if err != nil {
		page.Notices = append(page.Notices, s.unavailableNotice(c, retryPath(c, r), err))
		return page
	}
	items := c.listed(objs)
	shown := filter.keep(view.KindContact, items)
	page.Groups = s.contactGroups(c.sess, shown, c.spec.segment, false)
	page.Rail.Tags = filter.rail(view.KindContact, items)
	c.putIndex(indexOf(view.KindContact, items))
	total := len(objs)
	page.Bar.Loaded, page.Bar.Total = len(shown), &total
	page.Print = view.Print{Title: page.Title, Range: printRange(plural(len(shown), "card"), filter.tag), Taken: c.tf.taken(time.Now())}
	return page
}

// contactGroups sorts contacts by name under letter rubrics; "#" gathers
// everything that does not start with a letter, last. mark puts the origin on
// every row. A row's avatar is its photo's thumbnail when there is one to show.
func (s *Server) contactGroups(sess *session.Session, items []listed, segment string, mark bool) []view.ContactGroup {
	avatars := s.listAvatars(sess, items)
	names := make([]string, len(items))
	order := make([]int, len(items))
	for i, it := range items {
		names[i], order[i] = contactName(it.obj), i
	}
	sort.SliceStable(order, func(a, b int) bool {
		i, j := order[a], order[b]
		ri, rj := rubricOf(names[i]), rubricOf(names[j])
		if ri != rj {
			return rj == "#" || (ri != "#" && ri < rj)
		}
		if x, y := strings.ToLower(names[i]), strings.ToLower(names[j]); x != y {
			return x < y
		}
		if x, y := card.UID(items[i].obj), card.UID(items[j].obj); x != y {
			return x < y
		}
		return items[i].from.listPath(segment) < items[j].from.listPath(segment)
	})
	var groups []view.ContactGroup
	for _, i := range order {
		it, name := items[i], names[i]
		uid := card.UID(it.obj)
		row := view.ContactRow{
			Href:   s.route(it.from.recordPath(segment, uid)),
			UID:    uid,
			Avatar: avatarFor(uid, name),
			Name:   text(name),
			Phone:  text(firstValue(it.obj, "TEL")),
			Email:  text(firstValue(it.obj, "EMAIL")),
			Org:    text(structured(firstValue(it.obj, "ORG"))),
			State:  view.RecordOK,
		}
		row.Avatar.Href = avatars[i]
		if mark {
			row.Stamp, row.Colour = it.from.stamp(), catalogColour(it.from.col)
		}
		rubric := rubricOf(name)
		if n := len(groups); n == 0 || string(groups[n-1].Rubric) != rubric {
			groups = append(groups, view.ContactGroup{Rubric: text(rubric)})
		}
		g := &groups[len(groups)-1]
		g.Rows = append(g.Rows, row)
	}
	return groups
}

// weekWindow is the agenda week in the instance zone: the week of today, or
// the one starting at from.
func weekWindow(tf timeFmt, from string) (time.Time, time.Time) {
	start := tf.weekStart(time.Now().In(tf.tz))
	if from != "" {
		if d, err := time.ParseInLocation("2006-01-02", from, tf.tz); err == nil {
			start = d
		}
	}
	return start, start.AddDate(0, 0, 7)
}

// agendaRange moves the week and keeps the category filter.
func (s *Server) agendaRange(tf timeFmt, base, tag string, from, to time.Time) *view.Range {
	f := tagFilter{tag: tag, base: base, query: url.Values{}, route: s.route}
	week := func(t time.Time) string {
		f.query = url.Values{"from": {t.Format("2006-01-02")}}
		return f.href(tag)
	}
	current := tagFilter{base: base, query: url.Values{}, route: s.route}.href(tag)
	return &view.Range{
		From: text(tf.date(from)),
		To:   text(tf.date(to.AddDate(0, 0, -1))),
		Presets: []view.Action{
			{ID: "range.previous", Label: "Previous week", Href: week(from.AddDate(0, 0, -7)), Method: "GET", Enabled: true},
			{ID: "range.current", Label: "This week", Href: current, Method: "GET", Enabled: true},
			{ID: "range.next", Label: "Next week", Href: week(to), Method: "GET", Enabled: true},
		},
	}
}

// agendaPrint is the week of an agenda in its print header.
func agendaPrint(tf timeFmt, from, to time.Time) string {
	return tf.date(from) + " – " + tf.date(to.AddDate(0, 0, -1))
}

// agenda is one week of one calendar in the instance zone. Without a window
// the calendar is never read whole (S-11); the window goes to the server as
// the REPORT time-range.
func (s *Server) agenda(r *http.Request, c *collectionCtx) view.AgendaPage {
	tf := c.tf
	from, to := weekWindow(tf, r.URL.Query().Get("from"))
	page := view.AgendaPage{Shell: s.buildShell(c.sess, collectionLabel(c.col))}
	page.Bar.Actions = []view.Action{s.newRecordAction(c)}
	filter := s.tagFilter(c.listPath(), r.URL.Query())
	page.Bar.Range = s.agendaRange(tf, c.listPath(), filter.tag, from, to)

	objs, bad, err := c.cal.Query(r.Context(), c.col.Path, c.spec.component, &from, &to)
	if err != nil {
		page.Notices = append(page.Notices, s.unavailableNotice(c, retryPath(c, r), err))
		return page
	}
	items := c.listed(objs)
	var more []unreadable
	page.Days, page.Bar.Loaded, more = s.agendaDays(tf, filter.keep(view.KindEvent, items), from, to, c.spec.segment, false)
	page.Notices = appendUnreadable(page.Notices, append(unreadableOf(bad), more...))
	page.Rail.Tags = filter.rail(view.KindEvent, items)
	c.putIndex(indexOf(view.KindEvent, items))
	page.Print = view.Print{
		Title: page.Title, Range: printRange(agendaPrint(tf, from, to), plural(page.Bar.Loaded, "event"), filter.tag),
		Taken: tf.taken(time.Now()),
	}
	return page
}

// agendaDays expands series into the window and lays an instance another
// client rewrote (RECURRENCE-ID) over the instant it replaces; the rows go
// under day rubrics, all-day first, then by time.
func (s *Server) agendaDays(tf timeFmt, items []listed, from, to time.Time, segment string, mark bool) ([]view.AgendaDay, int, []unreadable) {
	tz := tf.tz
	type occurrence struct {
		at     time.Time
		allDay bool
		order  string
		row    view.AgendaRow
	}
	var occs []occurrence
	var bad []unreadable
	for _, it := range items {
		o := it.obj
		uid := cal.UID(o)
		summary := titleOr(cal.Text(o, "SUMMARY"), "Untitled event")
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
			u := unreadable{Unreadable: cal.Unreadable{Path: o.Path, UID: uid, Err: err}}
			if mark {
				u.stamp = it.from.stamp()
			}
			bad = append(bad, u)
			continue
		}
		for _, at := range instants {
			if at.Before(from) || !at.Before(to) {
				continue
			}
			occ := occurrence{at: at, allDay: tm.AllDay, order: it.from.listPath(segment), row: view.AgendaRow{
				Href:     s.route(it.from.recordPath(segment, uid)),
				UID:      uid,
				Time:     text(tf.timeLabel(at, tm.AllDay)),
				Zone:     text(tf.zone(tm.Zone)),
				Summary:  text(summary),
				Location: text(cal.Text(o, "LOCATION")),
				State:    view.RecordOK,
			}}
			if mark {
				occ.row.Stamp, occ.row.Colour = it.from.stamp(), catalogColour(it.from.col)
			}
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
		if occs[i].row.Summary != occs[j].row.Summary {
			return occs[i].row.Summary < occs[j].row.Summary
		}
		return occs[i].order < occs[j].order
	})
	var days []view.AgendaDay
	for _, o := range occs {
		rubric := tf.dayRubric(o.at)
		if n := len(days); n == 0 || string(days[n-1].Rubric) != rubric {
			days = append(days, view.AgendaDay{Rubric: text(rubric)})
		}
		d := &days[len(days)-1]
		d.Rows = append(d.Rows, o.row)
	}
	return days, len(occs), bad
}

func taskShow(r *http.Request) string {
	show := r.URL.Query().Get("show")
	if show != "done" && show != "all" {
		show = "open"
	}
	return show
}

func (s *Server) tasksList(r *http.Request, c *collectionCtx) view.TasksPage {
	show := taskShow(r)
	page := view.TasksPage{Shell: s.buildShell(c.sess, collectionLabel(c.col))}
	page.Bar.Actions = []view.Action{s.newRecordAction(c)}
	filter := s.tagFilter(c.listPath(), r.URL.Query())
	objs, bad, err := c.cal.Query(r.Context(), c.col.Path, c.spec.component, nil, nil)
	if err != nil {
		page.Notices = append(page.Notices, s.unavailableNotice(c, retryPath(c, r), err))
		return page
	}
	items := c.listed(objs)
	var open, done int
	var more []unreadable
	page.Groups, open, done, page.Bar.Loaded, more = s.taskGroups(c.tf, show, filter.keep(view.KindTask, items), c.spec.segment, false, filter, time.Now())
	page.Bar.Segments = s.taskSegments(c.listPath(), filter.tag, show, open, done)
	page.Notices = appendUnreadable(page.Notices, append(unreadableOf(bad), more...))
	page.Rail.Tags = filter.rail(view.KindTask, items)
	c.putIndex(indexOf(view.KindTask, items))
	page.Print = view.Print{
		Title: page.Title, Range: printRange(showLabels[show], plural(page.Bar.Loaded, "task"), filter.tag),
		Taken: c.tf.taken(time.Now()),
	}
	return page
}

// taskGroups shows open, done or all tasks: open first, then by due date,
// undated last. A row's categories link to the list filtered to each.
func (s *Server) taskGroups(tf timeFmt, show string, items []listed, segment string, mark bool, filter tagFilter, now time.Time) (groups []view.TaskGroup, open, done, loaded int, bad []unreadable) {
	type item struct {
		row   view.TaskRow
		due   time.Time
		order string
	}
	var rows []item
	for _, it := range items {
		o := it.obj
		uid := cal.UID(o)
		isDone := taskDone(o)
		if isDone {
			done++
		} else {
			open++
		}
		if (show == "open" && isDone) || (show == "done" && !isDone) {
			continue
		}
		due, err := cal.DueOf(o, tf.tz)
		if err != nil {
			u := unreadable{Unreadable: cal.Unreadable{Path: o.Path, UID: uid, Err: err}}
			if mark {
				u.stamp = it.from.stamp()
			}
			bad = append(bad, u)
			continue
		}
		row := view.TaskRow{
			Href: s.route(it.from.recordPath(segment, uid)), UID: uid, Done: isDone,
			Summary: text(titleOr(cal.Text(o, "SUMMARY"), "Untitled task")),
			Percent: percentOf(o), Tags: filter.tags(categoriesOf(view.KindTask, o)), State: view.RecordOK,
		}
		if mark {
			row.Stamp, row.Colour = it.from.stamp(), catalogColour(it.from.col)
		}
		if !due.Start.IsZero() {
			row.Due = text(tf.timeOrDate(due))
			row.Overdue = !isDone && due.Start.Before(now)
		}
		rows = append(rows, item{row: row, due: due.Start, order: it.from.listPath(segment)})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.row.Done != b.row.Done {
			return !a.row.Done
		}
		if a.due.IsZero() != b.due.IsZero() {
			return !a.due.IsZero()
		}
		if !a.due.Equal(b.due) {
			return a.due.Before(b.due)
		}
		if a.row.Summary != b.row.Summary {
			return a.row.Summary < b.row.Summary
		}
		return a.order < b.order
	})
	if len(rows) > 0 {
		group := view.TaskGroup{}
		for _, it := range rows {
			group.Rows = append(group.Rows, it.row)
		}
		groups = []view.TaskGroup{group}
	}
	return groups, open, done, len(rows), bad
}

// taskSegments switch between open, done and all, and keep the category
// filter.
func (s *Server) taskSegments(base, tag, show string, open, done int) []view.Action {
	segment := func(id, label, value string, n int) view.Action {
		f := tagFilter{base: base, query: url.Values{"show": {value}}, route: s.route}
		a := view.Action{ID: id, Label: text(fmt.Sprintf("%s · %d", label, n)), Href: f.href(tag), Method: "GET", Enabled: show != value}
		if show == value {
			a.Reason = "Shown now."
		}
		return a
	}
	return []view.Action{
		segment("tasks.open", "Open", "open", open),
		segment("tasks.done", "Done", "done", done),
		segment("tasks.all", "All", "all", open+done),
	}
}

func (s *Server) notesList(r *http.Request, c *collectionCtx) view.NotesPage {
	page := view.NotesPage{Shell: s.buildShell(c.sess, collectionLabel(c.col))}
	page.Bar.Actions = []view.Action{s.newRecordAction(c)}
	filter := s.tagFilter(c.listPath(), r.URL.Query())
	objs, bad, err := c.cal.Query(r.Context(), c.col.Path, c.spec.component, nil, nil)
	if err != nil {
		page.Notices = append(page.Notices, s.unavailableNotice(c, retryPath(c, r), err))
		return page
	}
	items := c.listed(objs)
	var more []unreadable
	page.Groups, page.Bar.Loaded, more = s.noteGroups(c.tf, filter.keep(view.KindNote, items), c.spec.segment, false, filter)
	page.Notices = appendUnreadable(page.Notices, append(unreadableOf(bad), more...))
	page.Rail.Tags = filter.rail(view.KindNote, items)
	c.putIndex(indexOf(view.KindNote, items))
	page.Print = view.Print{Title: page.Title, Range: printRange(plural(page.Bar.Loaded, "note"), filter.tag), Taken: c.tf.taken(time.Now())}
	return page
}

// noteGroups lists notes newest first. A row's categories link to the list
// filtered to each.
func (s *Server) noteGroups(tf timeFmt, items []listed, segment string, mark bool, filter tagFilter) ([]view.NoteGroup, int, []unreadable) {
	type item struct {
		row   view.NoteRow
		at    time.Time
		order string
	}
	rows := make([]item, 0, len(items))
	for _, it := range items {
		o := it.obj
		uid, body := cal.UID(o), cal.Description(o)
		title := noteTitle(cal.Text(o, "SUMMARY"), body)
		at := noteTime(o, tf.tz)
		row := view.NoteRow{
			Href: s.route(it.from.recordPath(segment, uid)), UID: uid, Title: text(title), Excerpt: text(noteExcerpt(body, title)),
			Tags: filter.tags(categoriesOf(view.KindNote, o)), State: view.RecordOK,
		}
		if mark {
			row.Stamp, row.Colour = it.from.stamp(), catalogColour(it.from.col)
		}
		if !at.IsZero() {
			row.Date = text(tf.date(at))
		}
		rows = append(rows, item{row: row, at: at, order: it.from.listPath(segment)})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if !rows[i].at.Equal(rows[j].at) {
			return rows[i].at.After(rows[j].at)
		}
		if rows[i].row.Title != rows[j].row.Title {
			return rows[i].row.Title < rows[j].row.Title
		}
		return rows[i].order < rows[j].order
	})
	var groups []view.NoteGroup
	if len(rows) > 0 {
		group := view.NoteGroup{}
		for _, it := range rows {
			group.Rows = append(group.Rows, it.row)
		}
		groups = []view.NoteGroup{group}
	}
	return groups, len(rows), nil
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
