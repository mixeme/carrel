package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/card"
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
	case WantsFragment(r) && FragmentTarget(r) == TargetDetails:
		// The panel beside a list (C-7.2): the row chosen there is asked
		// for at its own address, and what comes back is the card as a
		// panel — the same record, without the screen around it.
		_ = s.render.Fragment(w, r, http.StatusOK, TargetDetails, s.recordDetails(c, uid, obj), nil)
	default:
		s.writePage(w, r, http.StatusOK, s.withLinked(r.Context(), c, uid, obj, s.recordPage(c, uid, obj)))
	}
}

// recordDetails is a record as the details panel of a list (C-6 Details):
// its name, the fields the card shows, where it came from, and the actions
// that belong to it. Nothing here is a second way to read a record: it is
// the record's own page, cut to a panel.
func (s *Server) recordDetails(c *collectionCtx, uid string, obj *model.Object) view.Details {
	d := view.Details{Title: text(recordTitle(c.spec.kind, obj)), Source: c.source()}
	switch c.spec.kind {
	case view.KindContact:
		for _, f := range contactFieldsOf(obj) {
			d.Fields = append(d.Fields, f.Field)
		}
	case view.KindEvent:
		page := s.eventPage(c, uid, obj, nil)
		d.Fields, d.Actions = page.Fields, page.Actions
	case view.KindTask:
		page := s.taskPage(c, uid, obj, nil)
		d.Fields, d.Actions = page.Fields, page.Actions
	default:
		page := s.notePage(c, uid, obj, nil)
		d.Fields, d.Actions = []view.Field{{Label: "Date", Value: page.Meta.Date}}, page.Meta.Actions
		if page.Meta.Date == "" {
			d.Fields = nil
		}
	}
	if len(d.Actions) == 0 {
		d.Actions = s.withAttach(c, uid, obj.ETag, s.recordActions(c, uid, obj.ETag))
	}
	// The way to the whole record stands first: a panel is a look, not a
	// place to live in.
	d.Actions = append([]view.Action{{ID: "record.open", Label: text("Open the " + c.spec.noun),
		Href: s.route(c.recordPath(uid)), Method: "GET", Enabled: true}}, d.Actions...)
	return d
}

// withLinked is the reading page of a record in a linked group: the card of
// the entity (S-16). A record in no group is its page as it is.
func (s *Server) withLinked(ctx context.Context, c *collectionCtx, uid string, obj *model.Object, page any) any {
	switch p := page.(type) {
	case view.ContactPage:
		s.linkedContact(ctx, c, uid, obj, &p)
		return p
	case view.EventPage:
		if title, fields, rows, ok := s.linkedCalendar(ctx, c, uid, obj); ok {
			if title != "" {
				p.Shell.Title, p.Summary = text(title), text(title)
			}
			p.Fields, p.Linked = fields(p.Fields), rows
		}
		return p
	case view.TaskPage:
		if title, fields, rows, ok := s.linkedCalendar(ctx, c, uid, obj); ok {
			if title != "" {
				p.Shell.Title, p.Summary = text(title), text(title)
			}
			p.Fields, p.Linked = fields(p.Fields), rows
		}
		return p
	case view.NotePage:
		if title, _, rows, ok := s.linkedCalendar(ctx, c, uid, obj); ok {
			if title != "" {
				p.Shell.Title, p.Doc.Title = text(title), text(title)
			}
			p.Meta.Linked = rows
		}
		return p
	}
	return page
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
func (s *Server) recordShell(c *collectionCtx, title string, notices []view.Notice) view.Shell {
	sh := s.buildShell(c.sess, title)
	sh.Rail = s.recordRail(c, s.listBack(c))
	sh.Notices = append(sh.Notices, notices...)
	return sh
}

// recordActions: a read-only collection has no edit at all, not a locked form.
// What stays is a copy: it writes elsewhere and leaves this record as it is
// (S-18).
func (s *Server) recordActions(c *collectionCtx, uid, etag string) []view.Action {
	move := c.recordPath(uid) + "/move"
	copyAction := view.Action{ID: "record.copy", Label: "Copy…", Href: s.postQuery(move, url.Values{"copy": {"1"}}), Method: "GET", Enabled: true}
	if c.feed != nil {
		return []view.Action{copyAction}
	}
	if c.col.ReadOnly {
		return []view.Action{copyAction, s.publishAction(c, uid)}
	}
	// Edit first: it is what a person came to the record for. Publishing a
	// secret link is rare and stands past the moving and copying; the
	// deletion is last and marked (s-contacts-6).
	return []view.Action{
		{ID: "record.edit", Label: text("Edit " + c.spec.noun), Href: s.route(c.recordPath(uid) + "/edit"), Method: "GET", Enabled: true},
		{ID: "record.move", Label: "Move…", Href: s.route(move), Method: "GET", Enabled: true},
		copyAction,
		s.publishAction(c, uid),
		{ID: "record.delete", Label: text("Delete " + c.spec.noun), Href: s.postQuery(c.recordPath(uid)+"/delete", url.Values{"etag": {etag}}), Method: "POST", Danger: true, Enabled: true},
	}
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
	{prop: "TEL", label: "Phone", href: func(v string) string { return "tel:" + strings.ReplaceAll(v, " ", "") }},
	{prop: "EMAIL", label: "Email", href: func(v string) string { return "mailto:" + v }},
	{prop: "ADR", label: "Address", shape: structured},
	{prop: "ROLE", label: "Role"},
	{prop: "URL", label: "Website", href: webHref},
	{prop: "IMPP", label: "Messaging"},
	{prop: "BDAY", label: "Birthday"},
	{prop: "ANNIVERSARY", label: "Anniversary"},
	{prop: "NICKNAME", label: "Nickname"},
	{prop: "RELATED", label: "Related"},
	{prop: "GENDER", label: "Gender"},
	{prop: "LANG", label: "Language"},
	{prop: "TZ", label: "Time zone"},
	{prop: "GEO", label: "Location"},
	{prop: "CATEGORIES", label: "Categories", shape: listShown},
	{prop: "NOTE", label: "Note"},
}

// listShown is a comma-separated property read as the list it is: the
// commas of CATEGORIES separate values, and the card shows them apart.
func listShown(v string) string {
	return strings.Join(listItems(v), ", ")
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
	if obj == nil {
		return page
	}
	name := contactName(obj)
	page.Shell.Title, page.Name, page.Avatar = text(name), text(name), avatarFor(uid, name)
	page.Avatar.Href, page.PhotoHref = s.cardPhoto(c, uid, obj)
	page.Print = s.recordPrint(c, page.Name)
	page.Subtitle = text(contactSubtitle(obj))
	for _, f := range contactFieldsOf(obj) {
		// Where the person works and as what is the head of the card; the
		// panel beside a list has no such head and keeps them as fields.
		if f.prop == "ORG" || f.prop == "TITLE" {
			continue
		}
		page.Fields = append(page.Fields, f.Field)
	}
	page.Tags = s.recordTags(c, obj)
	page.Foreign = card.Foreign(obj)
	// The inset shows the timeline this session gathered; opening the card
	// gathers nothing (S-15.4). The way to it is always there.
	page.Timeline = s.timelineInset(c, uid, obj)
	page.More = view.Action{ID: "contact.timeline", Label: "Open the timeline", Href: s.route(c.recordPath(uid) + "/timeline"), Method: "GET", Enabled: true}
	if photoOf(obj) == linkedPhoto {
		page.PhotoNote = externalPhotoNote
	}
	page.Actions = s.recordActions(c, uid, obj.ETag)
	if s.mailOn() && c.feed == nil {
		// Send (s-contacts-6): the card goes into a letter by the attach
		// sheet's own POST, and the letter opens (S-26.2).
		page.Actions = append(page.Actions, view.Action{ID: "record.send", Label: "Send", Icon: "i-mail", Method: "POST", Enabled: true,
			Href: s.postQuery("/app/mail/attach", url.Values{"what": {"contact"}, "a": {c.conn.ID}, "c": {c.col.Path}, "uid": {uid}})})
	}
	return page
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
		for _, v := range props[f.prop] {
			field := view.Field{Label: text(typedLabel(f.label, v.Params)), Value: text(v.Text)}
			if f.shape != nil {
				field.Value = text(f.shape(v.Text))
			}
			if f.prop == "BDAY" {
				field.Value = text(birthdayShown(v.Text, v.Params)) // Apple's day without a year too
			}
			if f.href != nil {
				field.Href = f.href(v.Text)
			}
			out = append(out, propField{Field: field, prop: f.prop})
		}
	}
	return out
}

func (s *Server) eventPage(c *collectionCtx, uid string, obj *model.Object, notices []view.Notice) view.EventPage {
	page := view.EventPage{Shell: s.recordShell(c, "Event", notices), UID: uid, Source: c.source(), ReadOnly: c.col.ReadOnly}
	if obj == nil {
		return page
	}
	tf := c.tf
	tz := tf.tz
	summary := titleOr(cal.Text(obj, "SUMMARY"), "Untitled event")
	page.Shell.Title, page.Summary = text(summary), text(summary)
	page.Print = s.recordPrint(c, page.Summary)
	if tm, err := cal.TimingOf(obj, tz); err != nil {
		page.Notices = append(page.Notices, readNotice(obj.Path, err))
	} else if !tm.Start.IsZero() {
		page.Fields = append(page.Fields, view.Field{Label: "When", Value: text(tf.when(tm))})
		if z := tf.zone(tm.Zone); z != "" {
			page.Fields = append(page.Fields, view.Field{Label: "Original time zone", Value: text(z)})
		}
	}
	page.Fields = append(page.Fields, calendarFields(obj)...)
	page.Body = text(cal.Description(obj))
	if cal.HasRecurrence(obj) {
		page.Recurrence = s.recurrenceBlock(obj, tf)
	}
	alarms, _ := cal.Alarms(obj)
	page.Reminders = reminderRows(alarms, tf)
	for _, a := range cal.Attendees(obj) {
		page.Attendees = append(page.Attendees, view.AttendeeRow{Name: text(a.Name), Email: text(a.Email), Status: text(partstatLabel(a.Status))})
	}
	page.Attach = s.attachRows(c, uid, obj, obj.ETag, "")
	page.Related = s.relatedRows(c, obj)
	page.Actions = s.withAttach(c, uid, obj.ETag, s.recordActions(c, uid, obj.ETag))
	if c.feed == nil && !c.col.ReadOnly && c.col.Has(view.KindTask) {
		// New task (s-agenda-2): the quick sheet of a task in this calendar,
		// related to the event.
		page.Actions = append(page.Actions, view.Action{ID: "event.task", Label: "New task", Icon: "i-plus", Method: "GET", Enabled: true,
			Href: s.postQuery(c.origin().listPath(segmentOf(view.KindTask))+"/new/quick", url.Values{"related": {uid}})})
	}
	return page
}

func (s *Server) taskPage(c *collectionCtx, uid string, obj *model.Object, notices []view.Notice) view.TaskPage {
	page := view.TaskPage{Shell: s.recordShell(c, "Task", notices), UID: uid, Source: c.source(), ReadOnly: c.col.ReadOnly}
	if obj == nil {
		return page
	}
	tf := c.tf
	tz := tf.tz
	summary := titleOr(cal.Text(obj, "SUMMARY"), "Untitled task")
	page.Shell.Title, page.Summary = text(summary), text(summary)
	page.Print = s.recordPrint(c, page.Summary)
	if due, err := cal.DueOf(obj, tz); err != nil {
		page.Notices = append(page.Notices, readNotice(obj.Path, err))
	} else if !due.Start.IsZero() {
		page.Fields = append(page.Fields, view.Field{Label: "Due", Value: text(tf.timeOrDate(due))})
		if z := tf.zone(due.Zone); z != "" {
			page.Fields = append(page.Fields, view.Field{Label: "Original time zone", Value: text(z)})
		}
	}
	if start, err := cal.TimingOf(obj, tz); err == nil && !start.Start.IsZero() {
		page.Fields = append(page.Fields, view.Field{Label: "Starts", Value: text(tf.timeOrDate(start))})
	}
	if p := firstValue(obj, "PRIORITY"); p != "" && p != "0" {
		page.Fields = append(page.Fields, view.Field{Label: "Priority", Value: text(p)})
	}
	page.Fields = append(page.Fields, calendarFields(obj)...)
	page.Body = text(cal.Description(obj))
	page.Done, page.Percent = taskDone(obj), percentOf(obj)
	alarms, _ := cal.Alarms(obj)
	page.Reminders = reminderRows(alarms, tf)
	page.Attach = s.attachRows(c, uid, obj, obj.ETag, "")
	page.Related = s.relatedRows(c, obj)
	page.Actions = s.withAttach(c, uid, obj.ETag, s.recordActions(c, uid, obj.ETag))
	if c.feed == nil && !c.col.ReadOnly {
		page.Actions = append([]view.Action{s.doneAction(c.recordPath(uid)+"/edit", obj.ETag, page.Done, "")}, page.Actions...)
	}
	return page
}

func (s *Server) notePage(c *collectionCtx, uid string, obj *model.Object, notices []view.Notice) view.NotePage {
	page := view.NotePage{Shell: s.recordShell(c, "Note", notices), UID: uid, Source: c.source()}
	page.Doc = view.NoteDoc{Mode: view.NoteReading, ReadOnly: c.col.ReadOnly}
	if obj == nil {
		return page
	}
	tf := c.tf
	tz := tf.tz
	body := cal.Description(obj)
	title := noteTitle(cal.Text(obj, "SUMMARY"), body)
	page.Shell.Title = text(title)
	page.Print = s.recordPrint(c, text(title))
	page.Doc.Title, page.Doc.Body, page.Doc.Rendered = text(title), text(body), cal.RenderMarkdown(body)
	// Edit sits in the document bar; the rest belong to the note, beside it.
	for _, a := range s.withAttach(c, uid, obj.ETag, s.recordActions(c, uid, obj.ETag)) {
		if a.ID == "record.edit" {
			page.Doc.Bar = append(page.Doc.Bar, a)
		} else {
			page.Meta.Actions = append(page.Meta.Actions, a)
		}
	}
	if at := noteTime(obj, tz); !at.IsZero() {
		page.Meta.Date = text(tf.date(at))
	}
	page.Meta.Tags = s.recordTags(c, obj)
	page.Meta.Actions = append(page.Meta.Actions, view.Action{ID: "note.export", Label: "Export Markdown", Icon: "i-down", Method: "GET", Enabled: true,
		Href: s.postQuery(c.listPath()+"/export", url.Values{"uid": {uid}})})
	page.Meta.Attach = s.attachRows(c, uid, obj, obj.ETag, "")
	page.Meta.Related = s.relatedRows(c, obj)
	return page
}

// calendarFields are the properties of an event or task shown as fields.
func calendarFields(obj *model.Object) []view.Field {
	var out []view.Field
	if v := cal.Text(obj, "LOCATION"); v != "" {
		out = append(out, view.Field{Label: "Where", Value: text(v)})
	}
	if v := cal.Text(obj, "STATUS"); v != "" {
		out = append(out, view.Field{Label: "Status", Value: text(strings.ToLower(v))})
	}
	for _, v := range obj.Props()["ORGANIZER"] {
		who := v.Params["CN"]
		if who == "" {
			who = strings.TrimPrefix(strings.TrimPrefix(v.Text, "mailto:"), "MAILTO:")
		}
		out = append(out, view.Field{Label: "Organiser", Value: text(who)})
	}
	if v := firstValue(obj, "URL"); v != "" {
		out = append(out, view.Field{Label: "Link", Value: text(v), Href: webHref(v)})
	}
	if tags := cal.TextList(obj, "CATEGORIES"); len(tags) > 0 {
		out = append(out, view.Field{Label: "Categories", Value: text(strings.Join(tags, ", "))})
	}
	return out
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
	var rows []view.TimelineRow
	for _, target := range cal.RelatedTo(obj) {
		if e, ok := byUID[target]; ok && target != uid {
			rows = append(rows, view.TimelineRow{Href: s.route(c.kindRecordPath(e.Kind, target)), Kind: e.Kind, Title: text(e.Title), Why: "Related to"})
			continue
		}
		// The target is in another collection, or in none this session has
		// read: a title when some list of this session holds it, the UID
		// itself when nothing does. No link either way — a record is opened
		// at its own collection's address (S-11).
		if e, ok := s.sessionIndex(c.sess, s.catalogOf(c.sess))[target]; ok && e.Title != "" {
			rows = append(rows, view.TimelineRow{Kind: e.Kind, Title: text(e.Title), Why: "Related to · in another collection"})
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
				rows = append(rows, view.TimelineRow{Href: s.route(c.kindRecordPath(e.Kind, e.UID)), Kind: e.Kind, Title: text(e.Title), Why: "Refers to this"})
				break
			}
		}
	}
	return rows
}

func reminderRows(alarms []cal.Alarm, tf timeFmt) []view.ReminderRow {
	var rows []view.ReminderRow
	for _, a := range alarms {
		rows = append(rows, view.ReminderRow{Label: text(tf.reminder(a))})
	}
	return rows
}

func partstatLabel(v string) string {
	switch strings.ToUpper(v) {
	case "ACCEPTED":
		return "Accepted"
	case "DECLINED":
		return "Declined"
	case "TENTATIVE":
		return "Tentative"
	case "DELEGATED":
		return "Delegated"
	case "", "NEEDS-ACTION":
		return "No reply"
	default:
		return v
	}
}

// recurrenceBlock: the rule in words (C-10, here in web), the raw rule, the
// next instants, and the plain statement that the series is edited whole.
func (s *Server) recurrenceBlock(obj *model.Object, tf timeFmt) view.RecurrenceBlock {
	tz := tf.tz
	raw := cal.RawRRULE(obj)
	b := view.RecurrenceBlock{
		Summary: text(tf.recurrence(raw)),
		Raw:     text(raw),
		Note:    "Edited as a whole series",
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
