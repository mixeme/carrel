package web

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// getKindRecord is the reading address of a record (C-5.4): it shows the
// record without touching it. Missing and foreign answer alike.
func (s *Server) getKindRecord(w http.ResponseWriter, r *http.Request, kind string) {
	c, ok := s.openCollection(w, r, kind, true)
	if !ok {
		return
	}
	uid := r.PathValue("uid")
	obj, err := c.find(r.Context(), uid)
	switch {
	case notFoundErr(err):
		s.notFound(w, r)
	case err != nil:
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, nil, s.unavailableNotice(c, c.recordPath(uid), err)))
	case render.WantsFragment(r) && render.FragmentTarget(r) == render.TargetDetails:
		// The panel beside a list (C-7.2): the row chosen there is asked
		// for at its own address, and what comes back is the card as a
		// panel — the same record, without the screen around it.
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetDetails, s.recordDetails(c, uid, obj), nil)
	default:
		page := s.withLinked(r.Context(), c, uid, obj, s.recordPage(c, uid, obj))
		if p, ok := page.(view.NotePage); ok {
			s.noteReadingControls(r, &p)
			page = p
		}
		// «Show 3» without scripts: the card with the kept properties open.
		if r.URL.Query().Get("foreign") == "1" {
			switch p := page.(type) {
			case view.ContactPage:
				p.ForeignShow = nil
				page = p
			case view.NotePage:
				p.Meta.ForeignShow = nil
				page = p
			}
		}
		s.writePage(w, r, http.StatusOK, page)
	}
}

// noteReadingControls keeps presentation choices in links, so they also work
// without scripts. These choices never change the note or saved preferences.
func (s *Server) noteReadingControls(r *http.Request, p *view.NotePage) {
	q := r.URL.Query()
	p.Look = lookOf(r)
	p.Doc.Source = q.Get("view") == "source"
	p.Doc.HideSide = q.Get("side") == "0"
	switch q.Get("width") {
	case "full":
		p.Look.Wide = true
	case "read", "reading":
		p.Look.Wide = false
	}
	link := func(id, label, key, value string, on bool) view.Action {
		values := url.Values{}
		if p.Doc.Source {
			values.Set("view", "source")
		}
		if p.Doc.HideSide {
			values.Set("side", "0")
		}
		if p.Look.Wide {
			values.Set("width", "full")
		} else {
			values.Set("width", "read")
		}
		values.Set(key, value)
		return view.Action{ID: id, Label: text(label), Href: s.postQuery(r.URL.EscapedPath(), values), Method: "GET", Enabled: true, On: on}
	}
	mode, label := "source", "Source"
	if p.Doc.Source {
		mode, label = "reading", "Rendered"
	}
	side, sideLabel := "0", "Hide the side column"
	if p.Doc.HideSide {
		side, sideLabel = "1", "Show the side column"
	}
	p.Doc.Controls = []view.Action{
		link("note.source", label, "view", mode, p.Doc.Source),
		link("note.width.full", "Full width", "width", "full", p.Look.Wide),
		link("note.width.read", "Reading width", "width", "read", !p.Look.Wide),
		link("note.side", sideLabel, "side", side, p.Doc.HideSide),
	}
}

// recordDetails is a record as the details panel of a list (C-6 Details):
// its name, the fields the card shows, where it came from, and what is done
// with it — the buttons the panel frames draw (s-contacts-1, s-agenda-1,
// s-tasks-1, s-notes-1) and the rest under «⋯» (s-contacts-8, s-agenda-4,
// s-tasks-4, s-notes-12). Nothing here is a second way to read a record: it
// is the record's own page, cut to a panel.
//
// The panel is for reading, for every kind alike (from-typecase note 9): the
// blocks its frame draws, of the same types the record's page uses — the
// text of a note rendered, the description of an event or a task,
// attendees, attachments, related records, a contact's foreign properties.
// The categories are the row «Tags» of its fields, where the frame draws it.
func (s *Server) recordDetails(c *collectionCtx, uid string, obj *model.Object) view.Details {
	d := view.Details{Title: text(recordTitle(c.spec.kind, obj)), Source: c.source()}
	d.Source.UID = text(uid) // «UID …» of the panel's source (s-contacts-1)
	d.Subtitle = view.Text(d.Source.Stamp)
	ops := s.recordOps(c.origin(), c.spec, uid, obj.ETag)
	attach := s.attachOf(c, uid, obj.ETag)
	d.Menu = ops.menu
	switch c.spec.kind {
	case view.KindContact:
		for _, f := range contactFieldsOf(obj) {
			d.Fields = append(d.Fields, f.Field)
		}
		d.Fields = withBirthdayAside(withTagLinks(d.Fields, s.recordTags(c, obj)), c.tf)
		av := avatarFor(uid, contactName(obj))
		av.Href, _ = s.cardPhoto(c, uid, obj)
		d.Avatar = &av
		// The panel shows the organisation and the title as fields, every
		// value; the name once.
		d.Foreign = keptBesides(obj, "FN", "N")
		d.ForeignShow = s.foreignShow("contact.foreign", d.Foreign, c.recordPath(uid))
		// All records: the person's timeline, beside Edit (s-contacts-1).
		d.Actions = shown(ops.edit, &view.Action{ID: "contact.timeline", Label: "All records",
			Href: s.route(c.recordPath(uid) + "/timeline"), Method: "GET", Enabled: true}, ops.del)
	case view.KindEvent:
		page := s.eventPage(c, uid, obj, nil)
		// The fields of the card, in its order (s-agenda-1).
		d.Fields = page.Fields
		d.Body, d.Attendees, d.Attach, d.Related = page.Body, page.Attendees, page.Attach, page.Related
		d.Source.Path, d.Source.Compared = page.Source.Path, page.Source.Compared
		d.Actions = shown(ops.edit, attach, ops.del)
		if ops.del != nil && cal.HasRecurrence(obj) {
			d.DeleteNote = seriesNote
		}
	case view.KindTask:
		page := s.taskPage(c, uid, obj, nil)
		// The fields of the panel (s-tasks-1): Status, Due, Complete,
		// Priority, Tags — the card's order, monospaced where the frame
		// sets them; and where the task lies, as its card says it.
		d.Fields, _ = taskFields(c.tf, obj, true, s.recordTags(c, obj))
		// How late, under the title, as the card says it (s-tasks-20;
		// Typecase's note 130 p. 2).
		if due, err := cal.DueOf(obj, c.tf.tz); err == nil {
			d.Late = text(overdueBy(c.tf, due, taskDone(obj)))
		}
		d.Body, d.Attendees, d.Attach, d.Related = page.Body, page.Attendees, page.Attach, page.Related
		d.Source.Path, d.Source.Compared = page.Source.Path, page.Source.Compared
		d.Actions = shown(ops.edit, attach, ops.del)
	default:
		page := s.notePage(c, uid, obj, nil)
		// Where the note lies and when it was written, in one line under
		// its title (s-notes-1); no field repeats the date.
		d.Date = text(noteWhen(obj, c.tf))
		d.Rendered, d.Attendees, d.Attach, d.Related = page.Doc.Rendered, page.Meta.Attendees, page.Meta.Attach, page.Meta.Related
		d.Fields = tagsField(s.recordTags(c, obj))
		// The page of the note is a bare icon beside Edit; the deletion is
		// the last item of the menu (s-notes-12), not the foot of the panel.
		d.Actions = shown(ops.edit, &view.Action{ID: "record.open", Label: "Open the note page", Icon: "i-expand",
			Href: s.route(c.recordPath(uid)), Method: "GET", Enabled: true}, attach)
		if ops.del != nil {
			d.Menu = append(slices.Clone(ops.menu), view.MenuItem{Separator: true, Action: *ops.del})
		}
	}
	return d
}

// panelFields are the fields of a record's page for its panel: more stands
// under When and its original zone.
func panelFields(own, more []view.Field) []view.Field {
	at := 0
	for at < len(own) && (own[at].Label == "When" || own[at].Label == "Original time zone") {
		at++
	}
	return append(append(slices.Clone(own[:at]), more...), own[at:]...)
}

// foreignShow is the button that opens the kept properties, with their
// count: «Show 3» (s-contacts-1, -6), «Show 4» of a note (s-notes-2); none
// when there are none. Without scripts it is the card with them open
// (?foreign=1); the panel beside a list points it at the list with the
// panel instead (openInPanel).
func (s *Server) foreignShow(id string, foreign []view.Field, card string) *view.Action {
	if len(foreign) == 0 {
		return nil
	}
	return &view.Action{ID: id, Label: text("Show " + strconv.Itoa(len(foreign))),
		Href: s.route(card + "?foreign=1"), Method: "GET", Enabled: true}
}

// noteWhen is when a note was written: its day, and the time when it has
// one.
func noteWhen(obj *model.Object, tf timeFmt) string {
	if tm, err := cal.TimingOf(obj, tf.tz); err == nil && !tm.Start.IsZero() {
		return tf.timeOrDate(tm)
	}
	if at := noteTime(obj, tf.tz); !at.IsZero() {
		return tf.date(at)
	}
	return ""
}

// primary is the button drawn as the primary one where it stands (C-6.2
// Action.Primary): Edit in the head of a card (s-contacts-6, s-agenda-4,
// s-tasks-4) and in the bar of a note (s-notes-2), not in the details panel
// or a row's «⋯», where the same Edit is plain. nil stays nil.
func primary(a *view.Action) *view.Action {
	if a == nil {
		return nil
	}
	p := *a
	p.Primary = true
	return &p
}

// shown is the buttons that exist, in their order.
func shown(actions ...*view.Action) []view.Action {
	out := []view.Action{}
	for _, a := range actions {
		if a != nil {
			out = append(out, *a)
		}
	}
	return out
}

// withLinked is the reading page of a record in a linked group, opened from
// its collection: that copy, the badge «linked» where another copy holds a
// value differently, and the others under it (S-16; Typecase's Р-58). A
// record in no group is its page as it is.
func (s *Server) withLinked(ctx context.Context, c *collectionCtx, uid string, obj *model.Object, page any) any {
	switch p := page.(type) {
	case view.ContactPage:
		s.linkedContact(ctx, c, uid, obj, &p)
		return p
	case view.EventPage:
		fields, _, rows, apart, ok := s.linkedCalendar(ctx, c, uid, obj)
		if ok {
			p.Fields, p.Linked = fields(p.Fields), rows
			s.editViaCopies(c, uid, p.Actions)
		}
		p.Shell.Notices = withNotice(p.Shell.Notices, apart)
		return p
	case view.TaskPage:
		fields, _, rows, apart, ok := s.linkedCalendar(ctx, c, uid, obj)
		if ok {
			p.Fields, p.Linked = fields(p.Fields), rows
			s.editViaCopies(c, uid, p.Actions)
		}
		p.Shell.Notices = withNotice(p.Shell.Notices, apart)
		return p
	case view.NotePage:
		_, tags, rows, apart, ok := s.linkedCalendar(ctx, c, uid, obj)
		p.Shell.Notices = withNotice(p.Shell.Notices, apart)
		if ok {
			p.Meta.Linked = rows
			s.editViaCopies(c, uid, p.Doc.Bar)
			if tags(p.Meta.Tags) {
				badge := linkedMark
				p.Meta.TagsBadge = &badge
			}
		}
		return p
	}
	return page
}

// cardAnswer is the card an action on a record answers with: the card as it
// is opened (withLinked — its other copies, «linked», Edit through the
// copies), the action's notices over it. A record that could not be read
// (nil) has nothing to link: its page carries the notice alone.
func (s *Server) cardAnswer(r *http.Request, c *collectionCtx, uid string, obj *model.Object, notices ...view.Notice) any {
	page := s.recordPage(c, uid, obj, notices...)
	if obj == nil {
		return page
	}
	return s.withLinked(r.Context(), c, uid, obj, page)
}

// withNotice is notices with n after them, when there is one.
func withNotice(notices []view.Notice, n *view.Notice) []view.Notice {
	if n == nil {
		return notices
	}
	return append(notices, *n)
}

// recordPage builds the reading page of the collection's kind. A nil obj is
// a record the server could not be asked for: the page carries the notice.
func (s *Server) recordPage(c *collectionCtx, uid string, obj *model.Object, notices ...view.Notice) any {
	switch c.spec.kind {
	case view.KindContact:
		return s.contactPage(c, uid, obj, notices)
	case view.KindEvent:
		return s.eventPage(c, uid, obj, notices)
	case view.KindTask:
		return s.taskPage(c, uid, obj, notices)
	default:
		return s.notePage(c, uid, obj, notices)
	}
}

// recordShell carries the back rail (C-6.3). The print header is filled by
// the page once the record is read: a record that could not be asked for has
// nothing to print.
// recordRefresh is the top bar's refresh of a record's page and of its edit
// (s-contacts-6, s-contacts-3, s-agenda-2, s-tasks-2, s-notes-2; the
// client's stop C17): the circle opens the page again, which reads the
// record from its server, and the words say when this session last read its
// collection — what «read 2 min ago» of its source says too.
func (s *Server) recordRefresh(c *collectionCtx, here string) view.Refresh {
	last, _ := readingOf(c.sess, c.tf, c.conn.ID, c.col.Path)
	return view.Refresh{
		Action:   view.Action{ID: "record.refresh", Label: "Refresh", Icon: "i-refresh", Href: s.route(here), Method: "GET", Enabled: true},
		Answered: last,
	}
}

func (s *Server) recordShell(c *collectionCtx, title string, notices []view.Notice) view.Shell {
	sh := s.buildShell(c.sess, title)
	sh.Rail = s.recordRail(c, s.listBack(c))
	sh.Notices = append(sh.Notices, notices...)
	return sh
}

// recordOps is what can be done with one record, split the way the frames
// split it (from-typecase note 1): Open, Edit and the deletion stand on their
// own; Move, Copy, Export and Publish a link are the «⋯» (s-contacts-10,
// components-1…4). A read-only collection has no edit at all, not a locked
// form: Edit, Move and the deletion are nil there, and what stays is a copy —
// it writes elsewhere and leaves this record as it is (S-18) —, the file and
// the link. A subscription is not published onwards: it is someone else's.
type recordOps struct {
	open, edit, del *view.Action
	menu            []view.MenuItem
}

// kindIcons are the icons of the four kinds: Open in a row's menu is the
// kind's (components-1…4).
var kindIcons = map[view.Kind]string{view.KindContact: "i-con", view.KindEvent: "i-cal", view.KindTask: "i-task", view.KindNote: "i-note"}

func (s *Server) recordOps(o origin, spec kindSpec, uid, etag string) recordOps {
	rec := o.recordPath(spec.segment, uid)
	move := rec + "/move"
	ops := recordOps{open: &view.Action{ID: "record.open", Label: "Open", Icon: kindIcons[spec.kind], Href: s.route(rec), Method: "GET", Enabled: true}}
	if !o.col.ReadOnly && !o.conn.Feed {
		ops.edit = &view.Action{ID: "record.edit", Label: "Edit", Href: s.route(rec + "/edit"), Method: "GET", Enabled: true}
		ops.del = &view.Action{ID: "record.delete", Label: text("Delete " + spec.noun), Icon: "i-trash",
			Href: s.postQuery(rec+"/delete", url.Values{"etag": {etag}}), Method: "POST", Danger: true, Enabled: true}
		ops.menu = append(ops.menu, view.MenuItem{Action: view.Action{ID: "record.move", Label: "Move", Icon: "i-move", Href: s.route(move), Method: "GET", Enabled: true}})
	}
	ops.menu = append(ops.menu,
		view.MenuItem{Action: view.Action{ID: "record.copy", Label: "Copy", Icon: "i-dup", Href: s.postQuery(move, url.Values{"copy": {"1"}}), Method: "GET", Enabled: true}},
		view.MenuItem{Action: s.recordExportAction(o, spec, uid)})
	if !o.conn.Feed {
		ops.menu = append(ops.menu, view.MenuItem{Action: s.publishActionAt(o, spec.segment, uid)})
	}
	return ops
}

// rowMenu is the «⋯» of a row in a list (components-1…4): open, edit, the
// card's menu, and the deletion last behind a line.
func (ops recordOps) rowMenu() []view.MenuItem {
	out := []view.MenuItem{{Action: *ops.open}}
	if ops.edit != nil {
		edit := *ops.edit
		edit.Icon = "i-pencil"
		out = append(out, view.MenuItem{Action: edit})
	}
	out = append(out, ops.menu...)
	if ops.del != nil {
		out = append(out, view.MenuItem{Separator: true, Action: *ops.del})
	}
	return out
}

// menuWithout is the menu with one item left out.
// noteMenu is the «⋯» of a note: its Export is the journal as the server
// holds it, .ics, as an event's and a task's (Р20 of scans/kinds-parity.md) —
// the bar's Export Markdown hands over its text.
func noteMenu(menu []view.MenuItem) []view.MenuItem {
	out := []view.MenuItem{}
	for _, it := range menu {
		if it.ID == "record.export" {
			it.Label = text("Export " + exportFormats[view.KindEvent].ext)
			it.Href += "&as=ics"
		}
		out = append(out, it)
	}
	return out
}

type displayField struct {
	prop  string
	label string
	href  func(string) string
	shape func(string) string
}

// contactFields shows every property card.knownProps keeps out of Foreign,
// apart from FN and N (the name) and CATEGORIES (the tags): a known property
// that is never shown would be lost from sight (S-7.2).
var contactFields = []displayField{
	{prop: "ORG", label: "Organisation", shape: structured},
	{prop: "TITLE", label: "Title"},
	{prop: "TEL", label: "Phone", href: telHref},
	{prop: "EMAIL", label: "Email", href: func(v string) string { return "mailto:" + v }},
	{prop: "ADR", label: "Address", shape: structured},
	{prop: "ROLE", label: "Role"},
	{prop: "URL", label: "Website", href: webHref},
	{prop: "IMPP", label: "Messaging"},
	{prop: "BDAY", label: "Birthday"},
	// The dates, each with its words (contact_dates.go): ANNIVERSARY stands
	// for all of them.
	{prop: "ANNIVERSARY", label: "Anniversary"},
	{prop: "NICKNAME", label: "Nickname"},
	{prop: "RELATED", label: "Related"},
	{prop: "GENDER", label: "Gender"},
	{prop: "LANG", label: "Language"},
	{prop: "TZ", label: "Time zone"},
	{prop: "GEO", label: "Location"},
	{prop: "CATEGORIES", label: "Tags", shape: listShown},
	{prop: "NOTE", label: "Note"},
}

// listShown is a comma-separated property read as the list it is: the
// commas of CATEGORIES separate values, and the card shows them apart.
func listShown(v string) string {
	return strings.Join(listItems(v), ", ")
}

// telHref is a number to call as the card's link writes it (s-contacts-6):
// the digits and the leading plus, without what is there only to be read —
// «+7 912 445-10-88» is tel:+79124451088. A number that is already a tel:
// URI (vCard 4.0, VALUE=uri) keeps its own form; one with no digit, or with
// anything but digits and separators, is no link.
func telHref(v string) string {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(strings.ToLower(v), "tel:") {
		return v
	}
	var b strings.Builder
	for i, r := range v {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '+' && i == 0:
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '.' || r == '(' || r == ')' || r == '/':
		default:
			// An extension, a pause, a letter: not a number to call as it
			// stands, and the link is not guessed at — the value is shown.
			return ""
		}
	}
	if strings.TrimPrefix(b.String(), "+") == "" {
		return ""
	}
	return "tel:" + b.String()
}

func webHref(v string) string {
	if l := strings.ToLower(v); strings.HasPrefix(l, "https://") || strings.HasPrefix(l, "http://") {
		return v
	}
	return ""
}

// typeWords are the words of the product for the words of the format: a
// telephone is mobile, not «cell» (C-10, s-contacts-6).
var typeWords = map[string]string{"cell": "mobile"}

func typedLabel(label string, params map[string]string) string {
	t := params["TYPE"]
	if t == "" {
		return label
	}
	var parts []string
	for _, one := range strings.Split(t, ",") {
		one = strings.ToLower(strings.TrimSpace(one))
		if w, ok := typeWords[one]; ok {
			one = w
		}
		if one != "" {
			parts = append(parts, one)
		}
	}
	if len(parts) == 0 {
		return label
	}
	return label + " · " + strings.Join(parts, ", ")
}

func (s *Server) contactPage(c *collectionCtx, uid string, obj *model.Object, notices []view.Notice) view.ContactPage {
	page := view.ContactPage{Shell: s.recordShell(c, "Contact", notices), UID: uid, Source: c.source(), ReadOnly: c.col.ReadOnly}
	page.Top.Refresh = s.recordRefresh(c, c.recordPath(uid))
	if obj == nil {
		return page
	}
	name := contactName(obj)
	page.Shell.Title, page.Name, page.Avatar = text(name), text(name), avatarFor(uid, name)
	page.Avatar.Href, page.PhotoHref = s.cardPhoto(c, uid, obj)
	page.Print = s.recordPrint(c, page.Name)
	page.Subtitle = text(contactSubtitle(obj))
	led := map[string]bool{}
	for _, f := range contactFieldsOf(obj) {
		// Where the person works and as what is the head of the card; the
		// panel beside a list has no such head and keeps them as fields.
		if f.prop == "ORG" || f.prop == "TITLE" {
			// On paper they are fields again, first (print-3); a second
			// value is kept, as on the screen.
			if !led[f.prop] {
				page.Print.Lead = append(page.Print.Lead, f.Field)
			}
			led[f.prop] = true
			continue
		}
		page.Fields = append(page.Fields, f.Field)
	}
	page.Fields = withBirthdayAside(withTagLinks(page.Fields, s.recordTags(c, obj)), c.tf)
	page.Foreign = keptBesides(obj, "FN", "N", "ORG", "TITLE")
	page.ForeignShow = s.foreignShow("contact.foreign", page.Foreign, c.recordPath(uid))
	// The inset shows the timeline this session gathered; opening the card
	// gathers nothing (S-15.4). The way to it is always there.
	page.Timeline, page.TimelineCount = s.timelineInset(c, uid, obj)
	page.More = view.Action{ID: "contact.timeline", Label: "Open the timeline", Icon: "i-all", Href: s.route(c.recordPath(uid) + "/timeline"), Method: "GET", Enabled: true}
	page.Source.Path, page.Source.Compared = text(obj.Path), s.compared(c, uid, obj)
	if photoOf(obj) == linkedPhoto {
		page.PhotoNote = externalPhotoNote
	}
	ops := s.recordOps(c.origin(), c.spec, uid, obj.ETag)
	send := s.sendFor(c, uid)
	// Edit, Send and Print stand in the head, the deletion at the foot; the
	// rest is the «⋯» (s-contacts-6, s-contacts-10). Print is the card with
	// print=1 (the client's plan, decision 4).
	edit := primary(ops.edit)
	if edit != nil {
		edit.Icon = "i-pencil"
	}
	print := &view.Action{ID: "record.print", Label: "Print", Icon: "i-print", Href: s.route(c.recordPath(uid) + printQuery), Method: "GET", Enabled: true}
	page.Actions, page.Menu = shown(edit, send, print, ops.del), ops.menu
	return page
}

// keptBesides is what the card keeps as the server sent it (card.Foreign)
// and, of the properties it shows once — once —, every value after the
// first, by the property's name: «TITLE · Countess» (the owner's decision
// of 02.11.2026, variant Б). Nothing of the record is out of sight; the
// form edits the first.
func keptBesides(obj *model.Object, once ...string) []view.Field {
	return keptWith(card.Foreign(obj), obj, once...)
}

// noteKept is what a note keeps as the server sent it (Carrel's task 253,
// s-notes-2): the properties Carrel does not know, a row a value, and a
// second DESCRIPTION — the note's text is the first.
func noteKept(obj *model.Object) []view.Field {
	var foreign []view.Field
	for _, name := range cal.NoteForeign(obj) {
		for _, v := range obj.PropText(name) {
			foreign = append(foreign, view.Field{Label: text(name), Value: text(v), Mono: true})
		}
	}
	return keptWith(foreign, obj, "DESCRIPTION")
}

// keptWith is the foreign properties and every value after the first of
// the properties shown once, by name, then value.
func keptWith(out []view.Field, obj *model.Object, once ...string) []view.Field {
	props := obj.Props()
	for _, name := range once {
		for _, v := range props[name][min(1, len(props[name])):] {
			out = append(out, view.Field{Label: text(name), Value: text(v.Text), Mono: true})
		}
	}
	slices.SortStableFunc(out, func(a, b view.Field) int {
		if c := strings.Compare(string(a.Label), string(b.Label)); c != 0 {
			return c
		}
		return strings.Compare(string(a.Value), string(b.Value))
	})
	return out
}

// contactSubtitle is the line under the name: where the person works and as
// what (s-contacts-6). It is the head of the card, not two rows of its field
// list — the client is not left to guess which two to lift out.
func contactSubtitle(obj *model.Object) string {
	return contactSubtitleOf(obj, obj)
}

// contactSubtitleOf builds the same line from two records: a linked card
// takes the organisation from one member and the title from another, by the
// answer the person gave when linking (S-16).
func contactSubtitleOf(org, title *model.Object) string {
	var parts []string
	if org != nil {
		if v := org.Props()["ORG"]; len(v) > 0 {
			if name := structured(v[0].Text); name != "" {
				parts = append(parts, name)
			}
		}
	}
	if title != nil {
		if v := title.Props()["TITLE"]; len(v) > 0 && v[0].Text != "" {
			parts = append(parts, v[0].Text)
		}
	}
	return strings.Join(parts, " · ")
}

// propField is a field of the card with the property it shows.
type propField struct {
	view.Field
	prop string
}

// contactFieldsOf are the fields of a contact card, in the order of
// contactFields.
func contactFieldsOf(obj *model.Object) []propField {
	var out []propField
	props := obj.Props()
	for _, f := range contactFields {
		if f.prop == "CATEGORIES" {
			// One row of tags, where the frames draw it: between Birthday and
			// Note (s-contacts-1, -6, -7). A page with links puts the list's
			// filter on them (withTagLinks).
			if tags := plainTags(categoriesOf(view.KindContact, obj)); len(tags) > 0 {
				out = append(out, propField{Field: view.Field{Label: "Tags", Tags: tags}, prop: f.prop})
			}
			continue
		}
		if f.prop == "ANNIVERSARY" {
			// A day read digit by digit, as the birthday (s-contacts-34):
			// «Свадьба 12.06.2015»; a date without words is a Date.
			for _, d := range contactDates(props) {
				label := d.mark
				if label == "" {
					label = "Date"
				}
				out = append(out, propField{Field: view.Field{Label: text(label), Value: text(birthdayShown(d.value.Text, d.value.Params)), Mono: true}, prop: f.prop})
			}
			continue
		}
		for _, v := range props[f.prop] {
			field := view.Field{Label: text(typedLabel(f.label, v.Params)), Value: text(v.Text)}
			if f.shape != nil {
				field.Value = text(f.shape(v.Text))
			}
			day := f.prop == "BDAY" || f.prop == "ANNIVERSARY"
			if day {
				field.Value = text(birthdayShown(v.Text, v.Params)) // Apple's day without a year too
			}
			if f.href != nil {
				field.Href = f.href(v.Text)
			}
			// A number and a day are read digit by digit: the frames set
			// them in mono (s-contacts-1, -6, -7); an anniversary is a day
			// as the birthday is (Р26).
			field.Mono = f.prop == "TEL" || day
			out = append(out, propField{Field: field, prop: f.prop})
		}
	}
	return out
}

func (s *Server) eventPage(c *collectionCtx, uid string, obj *model.Object, notices []view.Notice) view.EventPage {
	page := view.EventPage{Shell: s.recordShell(c, "Event", notices), UID: uid, Source: c.source(), ReadOnly: c.col.ReadOnly}
	page.Top.Refresh = s.recordRefresh(c, c.recordPath(uid))
	if obj == nil {
		return page
	}
	tf := c.tf
	tz := tf.tz
	summary := titleOr(cal.Text(obj, "SUMMARY"), "Untitled event")
	page.Shell.Title, page.Summary = text(summary), text(summary)
	page.Print = s.recordPrint(c, page.Summary)
	// When, Repeats, Reminders, then the record's own fields (s-agenda-2).
	tm, err := cal.TimingOf(obj, tz)
	if err != nil {
		page.Notices = append(page.Notices, readNotice(obj.Path, err))
	}
	page.Fields = append(eventHead(tf, obj, tm), calendarFields(obj)...)
	page.Fields = withTagLinks(page.Fields, s.recordTags(c, obj))
	page.Body = text(cal.Description(obj))
	if cal.HasRecurrence(obj) {
		page.Recurrence = s.recurrenceBlock(obj, tf)
	}
	page.Attendees = recordPeople(obj, s.knownPeople(c.sess))
	page.Attach = s.attachRows(c, uid, obj, obj.ETag, "")
	page.Source.Path, page.Source.Compared = text(obj.Path), s.eventCompared(c, uid, obj)
	page.Related = s.relatedRows(c, obj)
	ops := s.recordOps(c.origin(), c.spec, uid, obj.ETag)
	// Edit, Send and Print in the head, Attach in its block (by its name,
	// attach.add), the deletion at the foot; the rest is the «⋯»
	// (s-agenda-2, s-agenda-5). Print is the card with print=1 (U-10). A
	// task is made in the tasks, not from a card (note 69, task 168).
	edit := primary(ops.edit)
	if edit != nil {
		edit.Icon = "i-pencil"
	}
	print := &view.Action{ID: "record.print", Label: "Print", Icon: "i-print", Href: s.route(c.recordPath(uid) + printQuery), Method: "GET", Enabled: true}
	attach := s.attachOf(c, uid, obj.ETag)
	page.Actions, page.Menu = shown(edit, attach, s.sendFor(c, uid), print, ops.del), ops.menu
	page.Drop = s.attachDrop(c, obj, attach) // s-io-20
	return page
}

// sendFor is Send of a record (s-contacts-6): it goes into a letter by the
// attach sheet's own POST, and the letter opens (S-26.2). Every kind has it
// (Р23 of scans/kinds-parity.md: a card had it, an event, a task and a note
// went into a letter only from the letter's own Attach). nil — no letters
// on this instance, or a subscription's record.
func (s *Server) sendFor(c *collectionCtx, uid string) *view.Action {
	if !s.canSendLetters(c.sess) || c.feed != nil {
		return nil
	}
	// The letter stands over the record and goes back to it (С21).
	back := origin{conn: c.conn, col: c.col}.recordPath(segmentOf(c.spec.kind), uid)
	return &view.Action{ID: "record.send", Label: "Send", Icon: "i-mail", Method: "POST", Enabled: true,
		Href: s.postQuery("/app/mail/attach", url.Values{"what": {string(c.spec.kind)}, "a": {c.conn.ID}, "c": {c.col.Path}, "uid": {uid}, "back": {back}})}
}

// eventHead are the first fields of an event, in the order of its card
// (s-agenda-2): When in pieces, with the zone it was made in as a badge;
// Repeats in short words with its end as a badge; Reminders. The panel puts
// Reminders before Repeats (s-agenda-1).
func eventHead(tf timeFmt, obj *model.Object, tm cal.Timing) []view.Field {
	var out []view.Field
	if !tm.Start.IsZero() {
		out = append(out, view.Field{Label: "When", Runs: tf.whenRuns(tm), Badge: tf.createdIn(tm)})
	}
	if cal.HasRecurrence(obj) {
		words, end := tf.repeats(cal.RawRRULE(obj), tm.Start)
		out = append(out, view.Field{Label: "Repeats", Value: text(words), Badge: end})
	}
	alarms, _ := cal.Alarms(obj)
	if rows := reminderRows(alarms, tm.AllDay, tf); len(rows) > 0 {
		labels := make([]string, 0, len(rows))
		for _, r := range rows {
			labels = append(labels, string(r.Label))
		}
		// One line, the reminders apart by the dot (s-agenda-1, -2: «10
		// minutes before · 1 day before»).
		out = append(out, view.Field{Label: "Reminders", Value: text(strings.Join(labels, " · "))})
	}
	return out
}

// eventCompared is the second half of the line «Read 2 min ago · changed on
// server 3 days ago» of an event's source (s-agenda-1, s-agenda-2): changed
// since the list of this session read it, or else when the record says it
// last changed; without either, what the version says (compared).
func (s *Server) eventCompared(c *collectionCtx, uid string, obj *model.Object) view.Text {
	said := s.compared(c, uid, obj)
	if said == "changed on server since" {
		return said
	}
	if t, ok := cal.TimeOf(obj, "LAST-MODIFIED", c.tf.tz); ok {
		return text("changed on server " + c.tf.ago(t, c.tf.now))
	}
	return said
}

func (s *Server) taskPage(c *collectionCtx, uid string, obj *model.Object, notices []view.Notice) view.TaskPage {
	page := view.TaskPage{Shell: s.recordShell(c, "Task", notices), UID: uid, Source: c.source(), ReadOnly: c.col.ReadOnly}
	page.Top.Refresh = s.recordRefresh(c, c.recordPath(uid))
	page.Subtitle = taskSubtitle(c, obj)
	if obj == nil {
		return page
	}
	summary := titleOr(cal.Text(obj, "SUMMARY"), "Untitled task")
	page.Shell.Title, page.Summary = text(summary), text(summary)
	page.Print = s.recordPrint(c, page.Summary)
	// The fields in the order of the card (s-tasks-2).
	fields, err := taskFields(c.tf, obj, false, s.recordTags(c, obj))
	if err != nil {
		page.Notices = append(page.Notices, readNotice(obj.Path, err))
	}
	page.Fields = fields
	page.Body = text(cal.Description(obj))
	page.Done = taskDone(obj)
	// The people, as an event's card names them (Р3; no frame draws them
	// yet — Typecase's task 179).
	page.Attendees = recordPeople(obj, s.knownPeople(c.sess))
	page.Attach = s.attachRows(c, uid, obj, obj.ETag, "")
	page.Related = s.relatedRows(c, obj)
	page.Source.Path, page.Source.Compared = text(obj.Path), s.eventCompared(c, uid, obj)
	ops := s.recordOps(c.origin(), c.spec, uid, obj.ETag)
	var done *view.Action
	if c.feed == nil && !c.col.ReadOnly {
		a := s.doneAction(c.recordPath(uid)+"/edit", obj.ETag, page.Done, "")
		a.Hint = s.doneHint(c.tf, obj)
		done = &a
	}
	// Edit, Mark done and Print in the head, Attach in its block (by its
	// name, attach.add), the deletion at the foot; the rest is the «⋯»
	// (s-tasks-2, s-tasks-6). Print is the card with print=1 (U-10).
	edit := primary(ops.edit)
	if edit != nil {
		edit.Icon = "i-pencil"
	}
	print := &view.Action{ID: "record.print", Label: "Print", Icon: "i-print", Href: s.route(c.recordPath(uid) + printQuery), Method: "GET", Enabled: true}
	attach := s.attachOf(c, uid, obj.ETag)
	page.Actions, page.Menu = shown(edit, done, attach, s.sendFor(c, uid), print, ops.del), ops.menu
	page.Drop = s.attachDrop(c, obj, attach) // s-io-20
	return page
}

func (s *Server) notePage(c *collectionCtx, uid string, obj *model.Object, notices []view.Notice) view.NotePage {
	page := view.NotePage{Shell: s.recordShell(c, "Note", notices), UID: uid, Source: c.source()}
	page.Source.UID = text(uid) // «UID …» of the column's Source (s-notes-2)
	page.Top.Refresh = s.recordRefresh(c, c.recordPath(uid))
	page.Doc = view.NoteDoc{Mode: view.NoteReading, ReadOnly: c.col.ReadOnly}
	if obj == nil {
		return page
	}
	tf := c.tf
	body := cal.Description(obj)
	title := noteTitle(cal.Text(obj, "SUMMARY"), body)
	page.Shell.Title = text(title)
	page.Print = s.recordPrint(c, text(title))
	if when := noteWhen(obj, tf); when != "" {
		// On paper where the note lies and when it was written (print-5).
		page.Print.Sources += text(" · " + when)
	}
	page.Rail.Neighbours = s.noteNeighbours(c, uid)
	page.Doc.Title, page.Doc.Body, page.Doc.Rendered = text(title), text(body), cal.RenderMarkdown(body)
	// Edit with its pencil, Export Markdown — its icon and «.md», the whole
	// name its hint (note 69) —, Send and Print sit in the bar over the
	// text, Attach and the deletion in the column beside it; what the bar
	// has no room for is its «⋯» (s-notes-2, s-notes-14). Print is the note
	// with print=1 (U-10); Source is the client's: the source is Doc.Body.
	ops := s.recordOps(c.origin(), c.spec, uid, obj.ETag)
	edit := primary(ops.edit)
	if edit != nil {
		edit.Icon = "i-pencil"
	}
	page.Doc.Bar = shown(edit, &view.Action{ID: "note.export", Label: ".md", Hint: "Export Markdown", Icon: "i-export", Method: "GET", Enabled: true,
		Href: s.postQuery(c.listPath()+"/export", url.Values{"uid": {uid}})},
		s.sendFor(c, uid),
		&view.Action{ID: "record.print", Label: "Print", Icon: "i-print", Href: s.route(c.recordPath(uid) + printQuery), Method: "GET", Enabled: true})
	attach := s.attachOf(c, uid, obj.ETag)
	page.Meta.Actions = shown(attach, ops.del)
	page.Drop = s.attachDrop(c, obj, attach) // s-io-20
	page.Meta.Menu = noteMenu(ops.menu)
	// The day, and the time when the note has one (s-notes-2).
	page.Meta.Date = text(noteWhen(obj, tf))
	page.Meta.Tags = s.recordTags(c, obj)
	// The link, as an event's and a task's card shows it (Р8 of
	// scans/kinds-parity.md; no frame draws it yet).
	if v := firstValue(obj, "URL"); v != "" {
		page.Meta.Link = &view.Field{Label: "Link", Value: text(v), Href: webHref(v)}
	}
	// The people, as an event's and a task's card names them (Р3).
	page.Meta.Attendees = recordPeople(obj, s.knownPeople(c.sess))
	page.Meta.Attach = s.attachRows(c, uid, obj, obj.ETag, "")
	page.Meta.Related = s.relatedRows(c, obj)
	// What the note keeps as the server sent it, folded (Carrel's task 253).
	page.Meta.Foreign = noteKept(obj)
	page.Meta.ForeignShow = s.foreignShow("note.foreign", page.Meta.Foreign, c.recordPath(uid))
	return page
}

// calendarFields are the properties of an event or task shown as fields.
func calendarFields(obj *model.Object) []view.Field {
	var out []view.Field
	if v := cal.Text(obj, "LOCATION"); v != "" {
		out = append(out, view.Field{Label: "Where", Value: text(v)})
	}
	if v := cal.Text(obj, "STATUS"); v != "" {
		out = append(out, view.Field{Label: "Status", Value: text(statusLabel(v))})
	}
	// An event's organiser is the first of its people (recordPeople,
	// s-agenda-2), not a field.
	for _, v := range obj.Props()["ORGANIZER"] {
		if obj.MainComponent() == "VEVENT" {
			break
		}
		who := v.Params["CN"]
		if who == "" {
			who = strings.TrimPrefix(strings.TrimPrefix(v.Text, "mailto:"), "MAILTO:")
		}
		out = append(out, view.Field{Label: "Organiser", Value: text(who)})
	}
	if v := firstValue(obj, "URL"); v != "" {
		out = append(out, view.Field{Label: "Link", Value: text(v), Href: webHref(v)})
	}
	// «Tags» on every card, as in the panels and the forms (s-agenda-2,
	// s-tasks-2; owner's decision, from-typecase note 19): one row of tags.
	return append(out, tagsField(plainTags(cal.TextList(obj, "CATEGORIES")))...)
}

func readNotice(path string, err error) view.Notice {
	return view.Notice{
		Role:    view.NoticeAlert,
		Title:   "Part of this record could not be read",
		Body:    "What could be read is shown; the record on the server is unchanged.",
		Details: &view.Diagnostic{Summary: text(path), Lines: []view.Text{text(err.Error())}},
	}
}

// relatedRows resolves RELATED-TO both ways from the lists this session has
// already loaded for this collection: a target found there is a link, anything
// else stays a visible UID. No second REPORT, no other collection (S-11).
func (s *Server) relatedRows(c *collectionCtx, obj *model.Object) []view.TimelineRow {
	uid := cal.UID(obj)
	index := c.index()
	byUID := make(map[string]indexEntry, len(index))
	for _, e := range index {
		byUID[e.UID] = e
	}
	// The day the short way of a list, «10 Aug» (s-notes-2, C-10); a
	// record without one says only its kind.
	day := func(e indexEntry) view.Text {
		if e.At == 0 {
			return ""
		}
		return text(c.tf.shortDay(time.Unix(e.At, 0)))
	}
	var rows []view.TimelineRow
	for _, target := range cal.RelatedTo(obj) {
		if e, ok := byUID[target]; ok && target != uid {
			rows = append(rows, view.TimelineRow{Colour: catalogColour(c.col), Href: s.route(c.kindRecordPath(e.Kind, target)), Kind: e.Kind, When: day(e), Title: text(e.Title), Why: "Related to"})
			continue
		}
		// The target is in another collection, or in none this session has
		// read: a title when some list of this session holds it, the UID
		// itself when nothing does. No link either way — a record is opened
		// at its own collection's address (S-11); the frames draw it as
		// words, not a link (s-agenda-1, s-agenda-2).
		if e, ok := s.sessionIndex(c.sess, s.catalogOf(c.sess))[target]; ok && e.Title != "" {
			rows = append(rows, view.TimelineRow{Kind: e.Kind, When: day(e), Title: text(e.Title), Why: "Related to · in another collection"})
			continue
		}
		rows = append(rows, view.TimelineRow{Title: text(target), Why: "Not in the lists loaded for this collection"})
	}
	for _, e := range index {
		if e.UID == uid {
			continue
		}
		for _, target := range e.Related {
			if target == uid {
				rows = append(rows, view.TimelineRow{Colour: catalogColour(c.col), Href: s.route(c.kindRecordPath(e.Kind, e.UID)), Kind: e.Kind, When: day(e), Title: text(e.Title), Why: "Refers to this"})
				break
			}
		}
	}
	return rows
}

func reminderRows(alarms []cal.Alarm, allDay bool, tf timeFmt) []view.ReminderRow {
	var rows []view.ReminderRow
	for _, a := range alarms {
		rows = append(rows, view.ReminderRow{Label: text(tf.reminderWords(a, allDay))})
	}
	return rows
}

// partstatLabel is an attendee's answer as the frames write it after the
// name, in lower case: «accepted», «no reply» (s-agenda-1, s-agenda-2).
func partstatLabel(v string) string {
	switch strings.ToUpper(v) {
	case "ACCEPTED":
		return "accepted"
	case "DECLINED":
		return "declined"
	case "TENTATIVE":
		return "tentative"
	case "DELEGATED":
		return "delegated"
	case "", "NEEDS-ACTION":
		return "no reply"
	default:
		return strings.ToLower(v)
	}
}

// seriesNote is the line under the deletion of a series (s-agenda-2,
// s-agenda-3): what is edited and deleted is the whole series.
const seriesNote = "Series is edited and deleted whole."

// recurrenceBlock: the rule in words (C-10, here in web), the raw rule, the
// next instants, and the plain statement that the series is edited whole.
func (s *Server) recurrenceBlock(obj *model.Object, tf timeFmt) view.RecurrenceBlock {
	tz := tf.tz
	raw := cal.RawRRULE(obj)
	var start time.Time
	if tm, err := cal.TimingOf(obj, tz); err == nil {
		start = tm.Start
	}
	b := view.RecurrenceBlock{
		Summary: text(tf.recurrence(raw, start)),
		Raw:     text(raw),
		Note:    seriesNote,
	}
	now := tf.now
	times, err := cal.Expand(obj, now, now.AddDate(1, 0, 0), tz)
	if err != nil {
		return b
	}
	allDay := false
	if tm, err := cal.TimingOf(obj, tz); err == nil {
		allDay = tm.AllDay
	}
	for i, at := range times {
		if i == 3 {
			break
		}
		label := "Next"
		if i > 0 {
			label = "Then"
		}
		value := tf.date(at)
		if !allDay {
			value += ", " + tf.clock(at)
		}
		b.Fields = append(b.Fields, view.Field{Label: text(label), Value: text(value)})
	}
	return b
}

// serverStamp is when the server version was last changed, as the record says.
func (s *Server) serverStamp(c *collectionCtx, obj *model.Object) view.When {
	tf, now := c.tf, c.tf.now
	if c.card != nil {
		if t, err := time.Parse("20060102T150405Z", firstValue(obj, "REV")); err == nil {
			return tf.formatWhen(t, false, now)
		}
		return ""
	}
	for _, prop := range []string{"LAST-MODIFIED", "DTSTAMP"} {
		if t, ok := cal.TimeOf(obj, prop, tf.tz); ok {
			return tf.formatWhen(t, false, now)
		}
	}
	return ""
}
