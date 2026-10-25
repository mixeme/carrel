package web

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/internal/publish"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// S-25.1: a secret link out, read-only, to one thing. The sheet names the
// thing and the lifetime before anything is made; the link is shown once —
// only its hash is kept. /p/{token} is read live without a session, through
// the envelope of the one connection the link holds, and gives nothing but
// the thing: no neighbours, no other links, no owner. A revoked, expired or
// unknown link is one answer in one time.

// publishQuery names what a sheet publishes: the kind segment, the
// connection, the collection and, for one record, its UID.
func publishQuery(segment, account, col, uid string) url.Values {
	q := url.Values{"k": {segment}, "a": {account}, "c": {col}}
	if uid != "" {
		q.Set("uid", uid)
	}
	return q
}

// publishActionAt opens the sheet for a collection or, with a UID, a
// record. «Publish a link», without the ellipsis: it has one scenario (the
// mockup's rule, §6).
func (s *Server) publishActionAt(o origin, segment, uid string) view.Action {
	return view.Action{ID: "publish", Label: "Publish a link", Icon: "i-link", Method: "GET", Enabled: true,
		Href: s.postQuery("/app/publish", publishQuery(segment, o.conn.ID, o.col.Path, uid))}
}

// publishTarget reads what the request names and checks it is the person's.
func (s *Server) publishTarget(r *http.Request, sess *session.Session) (*collectionCtx, string, bool) {
	spec, ok := recordKinds[r.FormValue("k")]
	if !ok {
		return nil, "", false
	}
	c, ok := s.collectionOf(sess, spec, r.FormValue("a"), r.FormValue("c"))
	if !ok || c.col.Gone || c.feed != nil {
		return nil, "", false
	}
	return c, r.FormValue("uid"), true
}

// publishTitle is what the link is made under: the record's title as it
// stands, or the collection's name.
func (s *Server) publishTitle(ctx context.Context, c *collectionCtx, uid string) (string, error) {
	if uid == "" {
		return collectionLabel(c.col), nil
	}
	obj, err := c.find(ctx, uid)
	if err != nil {
		return "", err
	}
	return recordTitle(c.spec.kind, obj), nil
}

func recordTitle(k view.Kind, obj *model.Object) string {
	switch k {
	case view.KindContact:
		return contactName(obj)
	case view.KindNote:
		return noteTitle(cal.Text(obj, "SUMMARY"), cal.Description(obj))
	case view.KindTask:
		return titleOr(cal.Text(obj, "SUMMARY"), "Untitled task")
	default:
		return titleOr(cal.Text(obj, "SUMMARY"), "Untitled event")
	}
}

// writeSheet writes the sheet over the page it was opened from (the owner's
// decision of 15.10.2026; note 105 of Typecase): back, else the page its
// Cancel or Close leads to — Connections for a collection, the record's page,
// the folder of a file. Cancel and Close go there too.
func (s *Server) writeSheet(w http.ResponseWriter, r *http.Request, sheet view.PublishSheet) {
	if render.WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, sheet, nil)
		return
	}
	back := r.FormValue("back")
	for _, id := range []string{"publish.cancel", "publish.close"} {
		s.cancelTo(r, back, sheet.Actions, id)
		if a := actionOf(sheet.Actions, id); a != nil && sheet.Under == nil {
			sheet.Under = s.underOf(r, back, strings.TrimPrefix(a.Href, s.basePath))
		}
	}
	s.writePage(w, r, http.StatusOK, sheet)
}

// actionOf is the action of id among actions; nil without one.
func actionOf(actions []view.Action, id string) *view.Action {
	for i := range actions {
		if actions[i].ID == id {
			return &actions[i]
		}
	}
	return nil
}

func (s *Server) publishSheet(sess *session.Session, c *collectionCtx, uid, title, how string) view.PublishSheet {
	// The head names what is published and, at its end, which (Пб1;
	// s-publish-2, -3, -19): a collection by where it lies, a record by its
	// title.
	what, subject := collectionNouns[c.spec.kind], string(c.stamp())
	// A collection is published from Connections, a record from its page
	// (note 105 of Typecase: the page under the sheet, and where Cancel
	// goes).
	from := "/app/settings/connections"
	if uid != "" {
		what, subject, from = c.spec.noun, title, c.recordPath(uid)
	}
	return s.sheetFor(sess, c.spec.kind, what, subject, publishQuery(c.spec.segment, c.conn.ID, c.col.Path, uid), from, how)
}

// sheetFor is the sheet of any target: what, where, how long the link lives,
// and one Create link (owner's decision 24.09.2026, s-publish-1…5). The
// lifetime is a field — 7, 14 or 30 days, or no expiry, 30 by default — so
// the sheet says nothing of it before the link exists. A whole calendar is
// also asked how much of it the link shows — a date range, the days only
// then, or the whole calendar — whether its .ics may be downloaded and
// subscribed to (s-publish-2). A calendar, an event, a task list and a task
// are asked what the visitor sees — the time always, the title unless taken
// off, the rest when ticked (Б39) —, as a contact is (s-publish-3).
func (s *Server) sheetFor(sess *session.Session, kind view.Kind, what, subject string, q url.Values, cancel, how string) view.PublishSheet {
	create := view.Action{ID: "publish.create", Label: "Create link", Href: s.postQuery("/app/publish", q), Method: "POST", Primary: true, Enabled: true}
	back := view.Action{ID: "publish.cancel", Label: "Cancel", Href: s.route(cancel), Method: "GET", Enabled: true}
	sheet := view.PublishSheet{
		Kind:    kind,
		Title:   text("Publish " + article(what)),
		Target:  text(subject),
		Local:   s.instanceLocal(),
		Actions: []view.Action{create, back},
	}
	rows := []view.FormRow{csrfField(sess)}
	if kind == view.KindEvent && q.Get("uid") == "" {
		if how != "whole" {
			how = "range"
		}
		// The days stand whichever is chosen, and «The whole calendar»
		// switches them off (Пб7; s-publish-2, note 105 of Typecase): without
		// a script they go with it, and are not read. They open from today
		// to the month's last day (Пб6).
		from, to := monthLeft(s.timeFor(sess))
		rows = append(rows, view.FormRow{Name: "how", Label: "How much of it", Kind: view.InputRadio, Value: text(how), Options: []view.Option{
			{Value: "range", Label: "A date range", Selected: how == "range"},
			{Value: "whole", Label: "The whole calendar", Selected: how == "whole", Disables: "from to"},
		}},
			view.FormRow{Name: "from", Label: "From", Kind: view.InputDate, Mono: true, Value: text(from)},
			view.FormRow{Name: "to", Label: "To", Kind: view.InputDate, Mono: true, Value: text(to)})
	}
	lives := livesOf(kind, what == "folder")
	var terms []view.FormRow
	switch {
	// The download is off until asked for, the same for every kind (the
	// owner's decision of 23.10.2026): a link shows what it shows, and its
	// file — and with it a subscription — is given only when ticked.
	case kind == view.KindContact:
		terms = append(terms, view.FormRow{Name: "download", Label: "Allow .vcf download and subscription", LabelRuns: allowLabel(".vcf", " download and subscription"), Kind: view.InputCheck})
	case kind == view.KindNote:
		terms = append(terms, view.FormRow{Name: "download", Label: "Allow .md download", LabelRuns: allowLabel(".md", " download"), Kind: view.InputCheck})
	case isCalendarKind(kind):
		// A calendar's and one event's alike (s-publish-2, -19), after what
		// the visitor sees: it obeys the same ticks.
		terms = append(terms, view.FormRow{Name: "download", Label: "Allow .ics download and subscription", LabelRuns: allowLabel(".ics", " download and subscription"), Kind: view.InputCheck})
	}
	terms = append(terms, view.FormRow{Name: "lives", Label: "Link lives for", Kind: view.InputSelect, Value: text(lives), Options: []view.Option{
		{Value: "7", Label: "7 days", Selected: lives == "7"},
		{Value: "14", Label: "14 days", Selected: lives == "14"},
		{Value: "30", Label: "30 days", Selected: lives == "30"},
		{Value: "never", Label: "No expiry", Separator: true}, // s-publish-14: only Revoke closes it
	}})
	if !isCalendarKind(kind) {
		rows, terms = append(rows, terms...), nil
	}
	sheet.Form = s.plainForm("/app/publish", "POST", rows...)
	if kind == view.KindContact {
		// What the visitor sees, each field asked of apart and all but the
		// first off (s-publish-3): a phone and an address belong to a person
		// who does not know of the link. «Name and organisation» stands and
		// is the owner's to take off (his decision of 10.10.2026): the
		// visitor is then given the name alone — the name is what the page
		// is of, and no tick takes it away. A book's ticks are for every card
		// of it.
		sees := view.FieldSet{Legend: "What the visitor sees", Rows: []view.FormRow{
			{Name: "name", Label: "Name and organisation", Kind: view.InputCheck, Value: "on"}}}
		for _, f := range visitorFields {
			sees.Rows = append(sees.Rows, view.FormRow{Name: f.name, Label: text(f.label), Kind: view.InputCheck})
		}
		sheet.Form.Sets = append([]view.FieldSet{sees}, sheet.Form.Sets...)
	}
	if isCalendarKind(kind) {
		// How much of it, what of each record, then its terms (s-publish-2,
		// -19): first what from the calendar, then what from each event.
		noun := "the " + what
		if q.Get("uid") == "" {
			noun = article(recordKinds[segmentOf(kind)].noun)
		}
		sheet.Form.Sets = append(sheet.Form.Sets, calendarSees(noun), view.FieldSet{Rows: terms})
		if len(rows) == 1 {
			// One record asks nothing before what the visitor sees
			// (s-publish-19): its CSRF goes with the terms.
			last := len(sheet.Form.Sets) - 1
			sheet.Form.Sets[last].Rows = append(rows, terms...)
			sheet.Form.Sets = sheet.Form.Sets[1:]
		}
	}
	sheet.Form.Action = s.postQuery("/app/publish", q)
	sheet.Form.Actions = sheet.Actions
	return sheet
}

// monthLeft is the days a calendar's sheet opens with: today and the last
// day of this month, in the person's zone.
func monthLeft(tf timeFmt) (from, to string) {
	now := tf.now.In(tf.tz)
	y, m, _ := now.Date()
	return now.Format("2006-01-02"), time.Date(y, m+1, 0, 0, 0, 0, 0, tf.tz).Format("2006-01-02")
}

// publishWindow is the range a published calendar shows: both days or
// neither. A range that ends before it starts is no range at all — the whole
// calendar, as the sheet says by default.
func publishWindow(form url.Values) (from, to string) {
	from, to = strings.TrimSpace(form.Get("from")), strings.TrimSpace(form.Get("to"))
	f, errFrom := time.Parse("2006-01-02", from)
	t, errTo := time.Parse("2006-01-02", to)
	if errFrom != nil || errTo != nil || t.Before(f) {
		return "", ""
	}
	return from, to
}

// madeSheet is the sheet once the link is made (s-publish-13): the link,
// shown this once — the form of the sheet one read-only field «Link» with
// the whole address a person copies, with the host (Пб9, Пб11; Link stays
// the page's path, as PublishedRow.Link beside its Address) —, its day in
// figures, «Until 2026-09-09.» (Пб10).
func (s *Server) madeSheet(r *http.Request, sess *session.Session, sheet view.PublishSheet, token string, expires *time.Time) view.PublishSheet {
	link := s.route("/p/" + token)
	whole := s.absURL(r, "/p/"+token)
	sheet.Link = text(link)
	sheet.Form = view.Form{Sets: []view.FieldSet{{Rows: []view.FormRow{
		{Name: "link", Label: "Link", Kind: view.InputText, Value: text(whole), ReadOnly: true, Mono: true},
	}}}}
	sheet.Expires = "No expiry. Revoke it when it is no longer needed."
	if expires != nil {
		sheet.Expires = text("Until " + s.timeFor(sess).dayKey(*expires) + ".")
	}
	// The cross of the head closes it to the page under it, where Cancel
	// led (s-publish-13, -22).
	closeTo := s.route("/")
	if a := actionOf(sheet.Actions, "publish.cancel"); a != nil {
		closeTo = a.Href
	}
	sheet.Actions = []view.Action{
		{ID: "publish.open", Label: "Open the link", Href: link, Method: "GET", Enabled: true},
		{ID: "publish.list", Label: "All published links", Href: s.route("/app/files/published"), Method: "GET", Enabled: true},
		{ID: "publish.close", Label: "Close", Icon: "i-close", Href: closeTo, Method: "GET", Enabled: true},
	}
	// Send by letter leads, primary, where letters go out (s-publish-13):
	// «Send this link», a letter of its own (s-mail-4).
	if s.canSendLetters(sess) {
		if id := user.LinkID(token); id != "" {
			sheet.Actions = append([]view.Action{{ID: "publish.mail", Label: "Send by letter", Method: "GET", Primary: true, Enabled: true,
				Href: s.postQuery("/app/mail/link", url.Values{"id": {id}}), Target: render.TargetSheet}}, sheet.Actions...)
		}
	}
	return sheet
}

// livesOf is how long a link lives when the sheet is left as it stands, in
// days as the field «Link lives for» names them: a contact's a week
// (s-publish-3) — its fields are another person's —, a folder's two
// (s-publish-5), anything else a month (s-publish-2).
func livesOf(kind view.Kind, folder bool) string {
	switch {
	case kind == view.KindContact:
		return "7"
	case kind == view.KindFile && folder:
		return "14"
	}
	return "30"
}

// publishExpiry is the lifetime the form chose: 7, 14 or 30 days, or none.
// A post that names none — a plain POST, an old client — gets what the sheet
// of its kind stands at (livesOf). «expires=never» is how the sheet asked
// for no expiry before «Link lives for» (24.09.2026), and still means it.
func (s *Server) publishExpiry(r *http.Request, standing string) *time.Time {
	return s.expiryOf(r.Form, standing)
}

// expiryOf is publishExpiry of a sheet's form kept elsewhere: the terms a
// letter keeps for a link it will make (note 70 of Typecase).
func (s *Server) expiryOf(form url.Values, standing string) *time.Time {
	ttl := publish.DefaultTTL
	lives := form.Get("lives")
	if lives == "" {
		lives = standing
	}
	switch lives {
	case "never":
		return nil
	case "7":
		ttl = 7 * 24 * time.Hour
	case "14":
		ttl = 14 * 24 * time.Hour
	}
	if form.Get("expires") == "never" {
		return nil
	}
	t := s.now().Add(ttl)
	return &t
}

// livesWords are the terms of a link as the list of links says them: «30
// days», «No expiry» (s-mail-1).
func livesWords(lives string) string {
	if lives == "never" {
		return "No expiry"
	}
	return lives + " days"
}

// formCheck is checkbox of a form kept apart from its request.
func formCheck(form url.Values, name string) string {
	if v := form.Get(name); v != "" && v != "off" {
		return "on"
	}
	return ""
}

// publishChoices puts into a link what the sheet's form chose (s-publish-2,
// -3; Б39) — the sheet's own post, or the terms a letter keeps for a link it
// makes when it goes (note 70 of Typecase). A whole calendar: how much of
// it and whether its .ics goes; a calendar's kinds: what the visitor sees,
// the time always; a contact: its fields and its .vcf.
func publishChoices(form url.Values, kind view.Kind, uid string, target *user.PublishTarget) {
	if kind == view.KindEvent && uid == "" {
		// «how» is always sent by the sheet. A post without it — an old
		// client, a plain POST — publishes as before: the whole calendar,
		// downloadable, with its descriptions.
		if form.Get("how") == "range" {
			target.From, target.To = publishWindow(form)
		}
		if form.Get("how") != "" {
			target.NoDownload = formCheck(form, "download") != "on"
		}
	}
	if isCalendarKind(kind) && form.Has("lives") {
		// The sheet's post says of the .ics of any calendar kind, a
		// calendar's and one record's alike (s-publish-19).
		target.NoDownload = formCheck(form, "download") != "on"
	}
	if isCalendarKind(kind) {
		// The time always, and what is ticked. A post without the sheet's
		// form gives what the sheet ticks before anyone does.
		target.Shows = []string{calendarSeesTime}
		for _, f := range calendarVisitorFields {
			if form.Has("lives") && formCheck(form, f.name) == "on" || !form.Has("lives") && f.on {
				target.Shows = append(target.Shows, f.name)
			}
		}
	}
	if kind == view.KindContact {
		// What is not ticked is not shown, and the file is one of the
		// things asked for. «Name and organisation» stands on the sheet, so
		// only a post of the sheet takes it off: one that says how long the
		// link lives, as the sheet's form always does.
		target.HideOrganisation = form.Has("lives") && formCheck(form, "name") != "on"
		for _, f := range visitorFields {
			if formCheck(form, f.name) == "on" {
				target.Shows = append(target.Shows, f.name)
			}
		}
		target.NoDownload = formCheck(form, "download") != "on"
	}
	if kind == view.KindNote && form.Has("lives") {
		// The sheet's post says of the .md (s-publish-4).
		target.NoDownload = formCheck(form, "download") != "on"
	}
}

// getPublish is the sheet: what will be published and for how long, before
// anything is made.
func (s *Server) getPublish(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	if r.FormValue("k") == filesSpec.segment {
		fc, segs, dir, ok := s.publishFileTarget(r, sess)
		if !ok {
			s.notFound(w, r)
			return
		}
		s.writeLetterSheet(w, r, sess, s.fileSheet(r.Context(), sess, fc, segs, dir))
		return
	}
	c, uid, ok := s.publishTarget(r, sess)
	if !ok {
		s.notFound(w, r)
		return
	}
	title, err := s.publishTitle(r.Context(), c, uid)
	switch {
	case notFoundErr(err):
		s.notFound(w, r)
		return
	case err != nil:
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, nil, s.unavailableNotice(c, c.listPath(), err)))
		return
	}
	sheet := s.publishSheet(sess, c, uid, title, r.FormValue("how"))
	if r.FormValue("mail") == "" {
		// A letter carries the note's link and not its files (Пб3).
		s.withAttachments(r.Context(), sess, c, uid, &sheet)
	}
	s.writeLetterSheet(w, r, sess, sheet)
}

// writeLetterSheet writes the sheet, or, asked from a letter (mail= names
// the part; note 70 of Typecase, s-mail-1), the sheet over the letter: the
// terms the part keeps, «Apply» that keeps the ones chosen in the letter —
// the link is made when the letter goes —, and Cancel back to it.
func (s *Server) writeLetterSheet(w http.ResponseWriter, r *http.Request, sess *session.Session, sheet view.PublishSheet) {
	m := r.FormValue("mail")
	if m == "" {
		s.writeSheet(w, r, sheet)
		return
	}
	i, err := strconv.Atoi(m)
	d := loadMailDraft(sess)
	if err != nil || i < 0 || i >= len(d.Parts) || !d.Parts[i].AsLink {
		s.notFound(w, r)
		return
	}
	p := d.Parts[i]
	apply := s.postQuery("/app/mail/send", url.Values{"op": {"terms"}, "i": {m}})
	for j := range sheet.Form.Sets {
		for k := range sheet.Form.Sets[j].Rows {
			row := &sheet.Form.Sets[j].Rows[k]
			switch {
			case row.Name == "lives":
				row.Value = text(partLives(p))
			case p.Terms == nil || row.Name == csrfFormField:
				continue
			case row.Kind == view.InputCheck:
				row.Value = text(formCheck(p.Terms, row.Name))
			default:
				row.Value = text(p.Terms.Get(row.Name))
			}
			for o := range row.Options {
				row.Options[o].Selected = row.Options[o].Value == string(row.Value)
			}
		}
	}
	sheet.Actions = []view.Action{
		{ID: "publish.apply", Label: "Apply", Href: apply, Method: "POST", Primary: true, Enabled: true},
		{ID: "publish.cancel", Label: "Cancel", Href: s.route("/app/mail/compose"), Method: "GET", Enabled: true},
	}
	sheet.Form.Action, sheet.Form.Actions = apply, sheet.Actions
	s.writeSheet(w, r, sheet)
}

// postPublish makes the link and shows it once, or with op=revoke and id=
// takes one of the person's links away.
func (s *Server) postPublish(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	if r.FormValue("op") == "revoke" {
		err := s.users.RevokePublished(sess.AccountID(), r.FormValue("id"))
		if errors.Is(err, user.ErrNotFound) {
			s.notFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "could not revoke the link", http.StatusInternalServerError)
			return
		}
		s.redirect(w, r, "/app/files/published")
		return
	}
	if r.FormValue("k") == filesSpec.segment {
		s.postPublishFile(w, r, sess)
		return
	}
	c, uid, ok := s.publishTarget(r, sess)
	if !ok {
		s.notFound(w, r)
		return
	}
	expires := s.publishExpiry(r, livesOf(c.spec.kind, false))
	title, err := s.publishTitle(r.Context(), c, uid)
	if err != nil {
		s.notFound(w, r)
		return
	}
	key, _ := sess.DataKey()
	target := user.PublishTarget{
		AccountID: c.conn.ID, Kind: string(c.spec.kind), Col: c.col.Path, UID: uid, Title: title,
		Colour: string(catalogColour(c.col)),
	}
	if c.spec.kind == view.KindEvent && uid == "" && r.FormValue("how") == "range" {
		// The sheet of a whole calendar (s-publish-2).
		if from, _ := publishWindow(r.Form); from == "" {
			// A range was chosen and its days were not given: the sheet
			// again, with the days asked for, rather than a guess.
			// The words stand under the empty field (s-publish-2), the
			// first of them when both are.
			sheet := s.publishSheet(sess, c, uid, title, "range")
			empty := "from"
			if strings.TrimSpace(r.FormValue("from")) != "" {
				empty = "to"
			}
			for i := range sheet.Form.Sets {
				for j := range sheet.Form.Sets[i].Rows {
					row := &sheet.Form.Sets[i].Rows[j]
					if row.Name == "from" || row.Name == "to" {
						row.Value = text(r.FormValue(row.Name))
					}
					if row.Name == empty {
						row.Error = "Give both days, or choose The whole calendar."
					}
				}
			}
			s.writeSheet(w, r, sheet)
			return
		}
	}
	publishChoices(r.Form, c.spec.kind, uid, &target)
	var children []user.Child
	if c.spec.kind == view.KindNote && uid != "" && formCheck(r.Form, "attachments") == "on" {
		children = s.attachmentChildren(r.Context(), sess, c, uid)
	}
	token, err := s.users.PublishWith(sess.AccountID(), key, c.conn, target, expires, children)
	if err != nil {
		http.Error(w, "could not publish", http.StatusInternalServerError)
		return
	}
	s.writeSheet(w, r, s.madeSheet(r, sess, s.publishSheet(sess, c, uid, title, ""), token, expires))
}

// sheetDefaults puts into a link of a kind what the sheet of publishing it
// chooses before anyone touches it (s-publish-2, -3, -4, -19; Б39): a
// calendar's, an event's, a task's — the time and the title; a contact's and
// a book's — the name and the organisation, no phone, address, photo or
// note; and the file of none of them, as no sheet ticks it (the owner's
// decision of 23.10.2026). A link made without that sheet — by a letter
// that sends a record as a link (Пб38; the owner's decision Б38) — gives the
// guest no more than the sheet would have.
func sheetDefaults(kind view.Kind, target *user.PublishTarget) {
	switch {
	case isCalendarKind(kind):
		target.Shows = []string{calendarSeesTime}
		for _, f := range calendarVisitorFields {
			if f.on {
				target.Shows = append(target.Shows, f.name)
			}
		}
		target.NoDownload = true
	case kind == view.KindContact:
		target.Shows, target.HideOrganisation, target.NoDownload = nil, false, true
	case kind == view.KindNote:
		target.NoDownload = true
	}
}

// publishedTitle is what a link opens: a calendar given out for some days
// says which, «Личный · 10–31 August» (s-publish-1) — two links to one
// calendar are told apart by them.
// publishedKind is what a link is, in a word (s-publish-1: «calendar»): a
// collection by the kind of its collection, a record by its kind, a file
// and a folder by what they are, with their icons.
func publishedKind(t user.PublishTarget) (string, string) {
	switch {
	case t.Kind == string(view.KindFile) && t.Folder:
		return "folder", "i-file"
	case t.Kind == string(view.KindFile):
		return "file", "i-doc"
	case t.UID != "":
		return t.Kind, ""
	}
	if word, ok := map[string]string{"event": "calendar", "contact": "address book", "task": "task list", "note": "notebook"}[t.Kind]; ok {
		return word, ""
	}
	return t.Kind, ""
}

func publishedTitle(t user.PublishTarget) string {
	if t.From == "" || t.To == "" {
		return t.Title
	}
	return t.Title + " · " + dayRange(t.From, t.To)
}

// dayRange writes two days, as the sheet took them, the short way: «10–31
// August», «28 July – 3 August», and the years only when they differ.
func dayRange(from, to string) string {
	f, errFrom := time.Parse("2006-01-02", from)
	t, errTo := time.Parse("2006-01-02", to)
	switch {
	case errFrom != nil || errTo != nil:
		return from + " – " + to
	case f.Equal(t):
		return f.Format("2 January")
	case f.Year() != t.Year():
		return f.Format("2 January 2006") + " – " + t.Format("2 January 2006")
	case f.Month() != t.Month():
		return f.Format("2 January") + " – " + t.Format("2 January")
	}
	return f.Format("2") + "–" + t.Format("2 January")
}

// getFilesPublished lists the person's live links (s-publish-1): what each
// opens, when it was made and until when, when it was last opened — the
// days absolute, a link nobody opened says so in words — and on the row
// Copy, Send (with mail on, 7.18 of the mockup; without it — s-publish-17)
// and Revoke.
func (s *Server) getFilesPublished(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	// The frame stands it in the settings rail (s-publish-1): the list is
	// reached from the files as well, but it is the person's, not a folder's.
	page := view.PublishedLinksPage{Shell: s.withRail(s.buildShell(sess, "Published links"), s.settingsRail(sess, "/app/files/published"))}
	// The list is the instance's own: its refresh reads it again, and there
	// is no server's answer to date (s-publish-1).
	page.Top.Refresh = view.Refresh{Action: view.Action{ID: "published.refresh", Label: "Refresh", Icon: "i-refresh", Href: s.route("/app/files/published"), Method: "GET", Enabled: true}}
	// What a link is; on a local instance, that it does not leave the
	// network (s-publish-1).
	page.Subtitle = line("Read-only, one object or one collection per link")
	if s.instanceLocal() {
		page.Subtitle = append(page.Subtitle, plainRun(" · "), view.Run{Text: "this instance is local: the link opens only from your network", Mark: view.RunDanger})
	}
	blob := s.catalogOf(sess)
	tf := s.timeFor(sess)
	mail := s.canSendLetters(sess)
	for _, l := range s.users.PublishedLinks(sess.AccountID()) {
		host := l.Target.AccountID
		if conn, ok := blob.Connection(l.Target.AccountID); ok {
			host = connLabel(conn)
		}
		label := l.Target.Col
		var colour view.Colour
		if col, ok := blob.Collection(l.Target.AccountID, l.Target.Col); ok {
			label = collectionLabel(col)
			colour = catalogColour(col)
		}
		kind, icon := publishedKind(l.Target)
		if icon != "" {
			colour = ""
		}
		row := view.PublishedRow{
			Title: text(publishedTitle(l.Target)), Kind: text(kind), Stamp: stamp(host, label), Colour: colour, Icon: icon,
			Created: view.When(tf.numDate(l.CreatedAt)), Expires: "No expiry", LastSeen: "never opened", Unopened: true, Opens: l.Opens,
		}
		if l.ExpiresAt != nil {
			row.Expires = text(tf.numDate(*l.ExpiresAt))
		}
		if l.LastSeen != nil {
			row.LastSeen, row.Unopened = tf.numStamp(*l.LastSeen), false
		}
		copyLink := view.Action{ID: "publish.copy", Label: "Copy", Method: "GET", Enabled: true}
		send := view.Action{ID: "publish.mail", Label: "Send", Method: "GET", Enabled: true,
			Href: s.postQuery("/app/mail/link", url.Values{"id": {l.ID}}), Target: render.TargetSheet}
		if l.Gone {
			// What it opened is no longer there and Carrel could not follow
			// it (Б41; s-publish-21): no term, nothing to copy or send — the
			// row is there for Revoke.
			row.Gone, row.Expires = "Deleted, or moved where Carrel could not follow it", "—"
			for _, a := range []*view.Action{&copyLink, &send} {
				a.Enabled, a.Reason = false, "It is no longer there"
			}
		} else {
			row.Href = s.publishedHref(sess, l.Target) // Пб41
		}
		// Every link is kept whole with its token (since 19.09.2026; the
		// owner's decision of 09.10.2026: a link without one does not
		// exist) — the row shows it and offers it.
		link := s.route("/p/" + blob.Links[l.ID])
		row.Link, copyLink.Href = text(link), link
		// Copy takes the link whole (s-publish-1).
		row.Address = text(s.publicOrigin(r) + link)
		row.Actions = append(row.Actions, copyLink)
		if mail {
			row.Actions = append(row.Actions, send)
		}
		// Revoke sends the list's one form with the row's ID (s-publish-1,
		// Typecase's note 110).
		row.ID = l.ID
		row.Actions = append(row.Actions, view.Action{ID: "publish.revoke", Label: "Revoke", Method: "POST", Danger: true, Enabled: true,
			Href: s.postQuery("/app/publish", url.Values{"op": {"revoke"}})})
		page.Rows = append(page.Rows, row)
	}
	// «Send this link» went (s-mail-4): said where its sheet came from.
	if n, err := strconv.Atoi(r.URL.Query().Get("sent")); err == nil && n > 0 {
		page.Notices = append(page.Notices, view.Notice{Role: view.NoticeInfo, Title: text(fmt.Sprintf("The letter went to %s.", plural(n, "recipient")))})
	}
	if len(page.Rows) == 0 {
		// No link out (s-publish-18): where one is made.
		page.Empty = &view.Notice{Title: "No links are published.", Body: "A link is made where the thing lies: «Publish a link» in the ⋯ menu of a row, a card or a collection."}
	}
	s.writePage(w, r, http.StatusOK, page)
}

// publishedHref is where in Carrel a link's thing is (Пб41; s-publish-1:
// the title leads there): the record, the list — a calendar's agenda from
// the link's first day —, the file or the folder; "" when Carrel cannot say.
func (s *Server) publishedHref(sess *session.Session, t user.PublishTarget) string {
	if t.Kind == string(view.KindFile) {
		fc, ok := s.fileCollection(sess, t.AccountID, t.Col)
		if !ok {
			return ""
		}
		segs, ok := relSegs(t.Path)
		if !ok {
			return ""
		}
		return s.route(fc.href("", segs, t.Folder || len(segs) == 0))
	}
	spec, ok := recordKinds[segmentOf(view.Kind(t.Kind))]
	if !ok {
		return ""
	}
	c, ok := s.collectionOf(sess, spec, t.AccountID, t.Col)
	if !ok {
		return ""
	}
	switch {
	case t.UID != "":
		return s.route(c.recordPath(t.UID))
	case t.From != "":
		return s.postQuery(c.listPath(), url.Values{"from": {t.From}})
	}
	return s.route(c.listPath())
}

// openLink opens the link a public request names, or answers the one "no"
// and reports false. Guessing is what the address's bucket is for, so only a
// miss spends it; a matching secret is an identity, as a session is, and
// spends the link's own bucket of the same size (owner's decision,
// 18.09.2026) — a mounted drive would not live on 30 a minute.
func (s *Server) openLink(w http.ResponseWriter, r *http.Request) (user.OpenLink, bool) {
	start := time.Now()
	ip := s.clientIP(r)
	if !s.limit.anonRoom(ip, start) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return user.OpenLink{}, false
	}
	link, ok := s.users.OpenPublished(r.PathValue("token"))
	if !ok {
		s.limit.allowAnon(ip, start)
		s.tokenNotFound(w, r, start)
		return user.OpenLink{}, false
	}
	if !s.limit.allowLink(link.ID, start) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return user.OpenLink{}, false
	}
	w.Header().Set("Cache-Control", "private, no-store")
	return link, true
}

// openPublished opens a link to a collection or a record with its provider.
func (s *Server) openPublished(w http.ResponseWriter, r *http.Request, link user.OpenLink) (*collectionCtx, bool) {
	start := time.Now()
	spec, ok := recordKinds[segmentOf(view.Kind(link.Target.Kind))]
	if !ok {
		s.tokenNotFound(w, r, start)
		return nil, false
	}
	col := sealed.Collection{AccountID: link.Target.AccountID, Path: link.Target.Col, Name: link.Target.Title, Kinds: []view.Kind{spec.kind}}
	c, err := s.newCollectionCtx(nil, spec, link.Conn, col)
	if err != nil {
		s.tokenNotFound(w, r, start)
		return nil, false
	}
	return c, true
}

// publicTrouble is a source that did not answer, said to a reader who came
// from outside: no server, no address, no diagnostics.
func publicTrouble() view.Notice {
	return view.Notice{Role: view.NoticeAlert, Title: "The source did not answer.", Body: "Try again later."}
}

// publicGone is what is no longer there (s-publish-38): the page has no
// foot then — the link opens nothing any more.
func publicGone() view.Notice {
	return view.Notice{Role: view.NoticeInfo, Title: "No longer there.", Body: "It was deleted or moved."}
}

// getPublished is the page of a link: the thing, read now, read-only.
func (s *Server) getPublished(w http.ResponseWriter, r *http.Request) {
	link, ok := s.openLink(w, r)
	if !ok {
		return
	}
	if link.Target.Kind == string(view.KindFile) {
		s.publishedFilesPage(w, r, link)
		return
	}
	if link.Target.Attach > 0 {
		s.publishedAttachment(w, r, link, false)
		return
	}
	c, ok := s.openPublished(w, r, link)
	if !ok {
		return
	}
	token := r.PathValue("token")
	page := view.PublishedPage{
		Plain:    view.Plain{Title: text(link.Target.Title)},
		Kind:     c.spec.kind,
		ReadOnly: true,
		Foot:     publicFoot(c.tf, link),
		// The button names the file it gives (s-publish-6, -7, -8): a note
		// goes as Markdown, never as its raw iCalendar.
		Actions: []view.Action{{ID: "published.download", Label: text("Download " + exportFormats[c.spec.kind].ext), Icon: "i-download", Href: s.route("/p/" + token + "/download"), Method: "GET", Enabled: true}},
	}
	if link.Target.NoDownload {
		// The link was made without the file (the default): the page
		// stays, the file and the subscription are not offered and not
		// served (s-publish-24, -26…-29).
		page.Actions = nil
	} else if link.Target.UID == "" {
		page.Strip = s.publicStrip(r, c, link, token)
	}
	ctx := r.Context()
	if uid := link.Target.UID; uid != "" {
		obj, err := c.find(ctx, uid)
		switch {
		case notFoundErr(err):
			page.Notices, page.Actions, page.Foot = []view.Notice{publicGone()}, nil, nil
			_ = s.users.MarkLinkGone(link.ID) // the owner's list says so (Б41)
		case err != nil:
			page.Notices, page.Actions = []view.Notice{publicTrouble()}, nil
		default:
			if c.card != nil {
				obj = visitorCard(obj, link.Target)
			}
			if isCalendarKind(c.spec.kind) {
				obj = visitorRecord(obj, link.Target)
			}
			var files *model.Object
			if c.spec.kind == view.KindNote {
				// What of its attachments the page says is read from the
				// note itself; the guest's note has none (Пб3).
				files, obj = obj, visitorNote(obj)
			}
			if obj == nil {
				// Built of what the link allows it could not be: nothing
				// of it goes out.
				page.Notices, page.Actions = []view.Notice{publicTrouble()}, nil
				break
			}
			s.publicRecord(c, obj, &page)
			if files != nil {
				s.guestAttachments(r, link, token, files, &page)
			}
			if page.Note != nil {
				page.Width = s.publicWidth(r, "/p/"+token)
			}
		}
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	if uid := r.URL.Query().Get("uid"); uid != "" {
		// A row of the list opens its record in the same link (s-publish-6,
		// -7, -28, -29): the page of a link to that one record.
		s.publicListRecord(w, r, c, link, uid, page)
		return
	}
	if err := s.publicCollection(r, c, link, token, &page); err != nil {
		page.Notices, page.Actions, page.Strip = []view.Notice{publicTrouble()}, nil, nil
		page.Filter, page.Empty, page.Range = nil, "", nil
		if goneErr(err) {
			// A collection removed is gone, not a server that did not
			// answer (Б41): the guest is not told to try later forever.
			page.Notices, page.Foot = []view.Notice{publicGone()}, nil
			_ = s.users.MarkLinkGone(link.ID)
		}
	}
	guestRows(&page)
	s.writePage(w, r, http.StatusOK, page)
}

// guestRows takes the owner's own marks off the rows a guest sees (Пс23;
// note 122 p. 1): «linked ×2», «2 sources» and the like are how the owner
// keeps their records and tell the guest nothing but about the owner. What
// is part of the time stays — «repeats weekly», «tentative» —, and «3
// attendees», which is there only when the link lets the attendees out
// (visitorRecords).
func guestRows(page *view.PublishedPage) {
	for i := range page.Contacts {
		for j := range page.Contacts[i].Rows {
			page.Contacts[i].Rows[j].Badges = nil
		}
	}
	for i := range page.Days {
		for j := range page.Days[i].Rows {
			row := &page.Days[i].Rows[j]
			row.Badges = slices.DeleteFunc(row.Badges, func(b view.Badge) bool { return strings.HasPrefix(string(b.Label), "linked") })
		}
	}
	for i := range page.Tasks {
		for j := range page.Tasks[i].Rows {
			page.Tasks[i].Rows[j].Badges = nil
		}
	}
	for i := range page.Notes {
		for j := range page.Notes[i].Rows {
			page.Notes[i].Rows[j].Badges = nil
		}
	}
}

// publicStrip is the address strip of a published list that may be
// downloaded (s-publish-6, -7): a calendar to subscribe to, webcal:// and
// «Open in calendar app»; a book's address, which another Carrel takes as a
// feed. A task list and a notebook have their Download only.
func (s *Server) publicStrip(r *http.Request, c *collectionCtx, link user.OpenLink, token string) *view.PublishedStrip {
	file := "/p/" + token + "/" + collectionSlug(link.Target.Title) + exportFormats[c.spec.kind].ext
	switch c.spec.kind {
	case view.KindEvent:
		addr := "webcal://" + strings.TrimPrefix(strings.TrimPrefix(s.absURL(r, file), "https://"), "http://")
		return &view.PublishedStrip{Label: "Subscribe in your own client", Address: text(addr), Open: addr, Note: "read-only, updates on its own"}
	case view.KindContact:
		return &view.PublishedStrip{Label: "Address of this book", Address: text(s.absURL(r, file)), Note: "another Carrel can subscribe to it; other clients import it"}
	}
	return nil
}

// publicListRecord is a record of a published list opened from its row
// (?uid=): the page of a link to that one record, with what the list's
// link lets out, its Download the list's own (s-publish-16, -26, -27, -8).
func (s *Server) publicListRecord(w http.ResponseWriter, r *http.Request, c *collectionCtx, link user.OpenLink, uid string, page view.PublishedPage) {
	page.Strip = nil
	obj, err := c.find(r.Context(), uid)
	switch {
	case notFoundErr(err):
		page.Notices, page.Actions, page.Foot = []view.Notice{publicGone()}, nil, nil
	case err != nil:
		page.Notices, page.Actions = []view.Notice{publicTrouble()}, nil
	default:
		if c.card != nil {
			obj = visitorCard(obj, link.Target)
		}
		if isCalendarKind(c.spec.kind) {
			obj = visitorRecord(obj, link.Target)
		}
		if c.spec.kind == view.KindNote {
			obj = visitorNote(obj)
		}
		if obj == nil {
			page.Notices, page.Actions = []view.Notice{publicTrouble()}, nil
			break
		}
		s.publicRecord(c, obj, &page)
		if page.Note != nil {
			page.Width = s.publicWidth(r, s.postQuery("/p/"+r.PathValue("token"), url.Values{"uid": {uid}}))
		}
	}
	s.writePage(w, r, http.StatusOK, page)
}

// publicWidth is the width of a text read on a public page, by its address
// (s-publish-8, -10): the full width, or the reading width at
// ?width=reading — no script needed.
func (s *Server) publicWidth(r *http.Request, here string) []view.Action {
	reading := r.URL.Query().Get("width") == "reading"
	wide := s.route(here)
	sep := "?"
	if strings.Contains(here, "?") {
		sep = "&"
	}
	return []view.Action{
		{ID: "width.full", Label: "Full width", Href: wide, Method: "GET", Enabled: true, On: !reading},
		{ID: "width.reading", Label: "Reading width", Href: wide + sep + "width=reading", Method: "GET", Enabled: true, On: reading},
	}
}

// publicRecord fills the page with one record — its own fields, nothing that
// leads back into the owner's Carrel (note 122 p. 4). When and where are the
// line under the title, not fields (s-publish-16, -27); a contact's head is
// its initials, its name, its organisation and title (s-publish-26); people
// are names, without links or marks.
func (s *Server) publicRecord(c *collectionCtx, obj *model.Object, page *view.PublishedPage) {
	tf := c.tf
	page.Title, page.Lede = text(recordTitle(c.spec.kind, obj)), nil
	place := strings.TrimSpace(cal.Text(obj, "LOCATION"))
	lede := func(head ...string) {
		if place != "" {
			head = append(head, place)
		}
		page.Lede = line(strings.Join(head, " · "))
	}
	switch c.spec.kind {
	case view.KindContact:
		name := contactName(obj)
		page.Avatar = &view.Avatar{Initials: text(initials(name)), Colour: collectionColour(card.UID(obj))}
		var head []string
		if org := structured(firstValue(obj, "ORG")); org != "" {
			head = append(head, org)
		}
		if title := firstValue(obj, "TITLE"); title != "" {
			head = append(head, title)
		}
		page.Lede = line(strings.Join(head, " · "))
		for _, f := range contactFieldsOf(obj) {
			if f.prop == "ORG" || f.prop == "TITLE" || f.prop == "CATEGORIES" {
				continue
			}
			page.Fields = append(page.Fields, f.Field)
		}
	case view.KindEvent:
		var head []string
		tm, err := cal.TimingOf(obj, tf.tz)
		if err == nil && !tm.Start.IsZero() {
			head = append(head, tf.when(tm))
		}
		lede(head...)
		if err == nil && cal.HasRecurrence(obj) {
			words, end := tf.repeats(cal.RawRRULE(obj), tm.Start)
			page.Fields = append(page.Fields, view.Field{Label: "Repeats", Value: text(words), Badge: end})
		}
		if v := cal.Text(obj, "STATUS"); v != "" {
			page.Fields = append(page.Fields, view.Field{Label: "Status", Value: text(statusLabel(v))})
		}
		page.Body = text(cal.Description(obj))
		page.Attendees = recordPeople(obj, nil)
	case view.KindTask:
		var head []string
		if due, err := cal.DueOf(obj, tf.tz); err == nil && !due.Start.IsZero() {
			// «Due Friday, 7 August 2026», its time when it has one
			// (s-publish-27).
			day := due.Start.In(tf.tz).Format("Monday, 2 January 2006")
			if !due.AllDay {
				day += ", " + tf.clock(due.Start)
			}
			head = append(head, "Due "+day)
		}
		lede(head...)
		status := taskStatuses[0].label
		if v := strings.TrimSpace(firstValue(obj, "STATUS")); v != "" {
			status = statusLabel(v)
		}
		f := view.Field{Label: "Status", Runs: view.Runs{{Text: text(status)}}}
		if p := percentOf(obj); p != nil {
			f.Runs = append(f.Runs, view.Run{Text: " "}, view.Run{Text: text(strconv.Itoa(*p) + "%"), Mark: view.RunMono})
		}
		page.Fields = append(page.Fields, f)
		page.Body = text(cal.Description(obj))
	default:
		body := cal.Description(obj)
		page.Note = &view.NoteDoc{Mode: view.NoteReading, Title: page.Title, Body: text(body), Rendered: cal.RenderMarkdown(body), ReadOnly: true}
		// A note always says its day, monospaced (s-publish-8): a note
		// without one looks forged. Its time when it has one.
		if t := noteTime(obj, tf.tz); !t.IsZero() {
			day := tf.date(t)
			if lt := t.In(tf.tz); lt.Hour() != 0 || lt.Minute() != 0 {
				day += ", " + tf.clock(t)
			}
			page.Lede = runs(monoRun(day))
		}
	}
}

// publicCollection fills the page with the collection read now: a book, a
// calendar — its days, or a week of the whole of it —, a task list, a
// notebook (note 122). Each row opens its record in the same link (?uid=);
// «Filter by word» (?q=) sifts it as the owner's list does, and goes away
// with nothing to sift.
func (s *Server) publicCollection(r *http.Request, c *collectionCtx, link user.OpenLink, token string, page *view.PublishedPage) error {
	ctx := r.Context()
	tf := c.tf
	here := "/p/" + token
	open := func(uid string) string { return s.postQuery(here, url.Values{"uid": {uid}}) }
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	has := func(q string, hay ...string) bool {
		return q == "" || strings.Contains(strings.ToLower(strings.Join(hay, "\n")), strings.ToLower(q))
	}
	filter := func(n int) {
		if n > 0 || q != "" {
			page.Filter = s.publicFilter(here, "Filter by word", q, "")
		}
	}
	switch c.spec.kind {
	case view.KindContact:
		objs, err := c.contactsAll(ctx)
		if err != nil {
			return err
		}
		all := len(objs)
		sort.SliceStable(objs, func(i, j int) bool {
			return strings.ToLower(contactName(objs[i])) < strings.ToLower(contactName(objs[j]))
		})
		// By letter, as the book in Carrel (Пс10; s-publish-7), each row
		// marked with the book's colour (Пс12).
		for _, o := range objs {
			o = visitorCard(o, link.Target)
			name := contactName(o)
			row := view.ContactRow{
				UID: card.UID(o), Href: open(card.UID(o)), Avatar: view.Avatar{Initials: text(initials(name)), Colour: collectionColour(card.UID(o))},
				Name: text(name), Phone: text(firstValue(o, "TEL")), Email: text(firstValue(o, "EMAIL")),
				Org: text(structured(firstValue(o, "ORG"))), State: view.RecordOK, Colour: linkColour(link.Target),
			}
			if !has(q, string(row.Name), string(row.Phone), string(row.Email), string(row.Org)) {
				continue
			}
			rubric := rubricOf(name)
			if n := len(page.Contacts); n == 0 || string(page.Contacts[n-1].Rubric) != rubric {
				page.Contacts = append(page.Contacts, view.ContactGroup{Rubric: text(rubric)})
			}
			g := &page.Contacts[len(page.Contacts)-1]
			g.Rows, g.Count = append(g.Rows, row), g.Count+1
		}
		page.Lede = line(plural(all, "contact"))
		filter(all)
		if all == 0 {
			page.Empty = "This book is empty." // s-publish-35
		}
	case view.KindEvent:
		from, to := weekWindow(tf, r.URL.Query().Get("from"))
		fixed := publishedWindow(tf, link)
		if fixed != nil {
			from, to = fixed[0], fixed[1]
		}
		if q != "" {
			// With a word the days are all found in the calendar — within
			// the link's days when it has them —, each series once at its
			// next time (s-publish-39).
			return s.publicFound(r, c, link, token, q, fixed, page)
		}
		objs, _, err := c.calQuery(ctx, c.spec.component, &from, &to)
		if err != nil {
			return err
		}
		objs = visitorRecords(objs, link.Target)
		page.Days, _, _ = s.agendaDays(tf, kindedAll(c.listed(objs), view.KindEvent), from, to, c.spec.segment, false, nil)
		guestDays(page, link, open)
		// The days in one phrase (Пс5; s-publish-6, -24).
		page.Lede = line(tf.window(from, to))
		filter(1)
		if fixed == nil {
			// The whole calendar steps by weeks, as the owner's (s-publish-24):
			// ← Today →, the arrows saying where they go.
			week := func(t time.Time) string {
				return s.postQuery(here, url.Values{"from": {t.Format("2006-01-02")}})
			}
			page.Range = &view.Range{
				From: text(tf.date(from)), To: text(tf.date(to.AddDate(0, 0, -1))),
				Presets: []view.Action{
					{ID: "range.previous", Label: "Previous week", Icon: "i-back", Href: week(from.AddDate(0, 0, -7)), Method: "GET", Enabled: true},
					{ID: "range.current", Label: "Today", Href: s.route(here), Method: "GET", Enabled: true},
					{ID: "range.next", Label: "Next week", Icon: "i-next", Href: week(to), Method: "GET", Enabled: true},
				},
			}
			if len(page.Days) == 0 {
				page.Empty, page.Filter = "No events this week.", nil // s-publish-25
			}
		} else if len(page.Days) == 0 {
			page.Empty = "No events these days."
		}
	case view.KindTask:
		objs, _, err := c.calQuery(ctx, c.spec.component, nil, nil)
		if err != nil {
			return err
		}
		objs = visitorRecords(objs, link.Target)
		all := len(objs)
		// Open, then done, under their rubrics with the count (s-publish-28).
		groups := []view.TaskGroup{{Rubric: "Open"}, {Rubric: "Done"}}
		for _, o := range objs {
			title := recordTitle(view.KindTask, o)
			if !has(q, title, cal.Description(o)) {
				continue
			}
			row := view.TaskRow{UID: cal.UID(o), Href: open(cal.UID(o)), Done: taskDone(o), Summary: text(title), Percent: percentOf(o), State: view.RecordOK}
			if due, err := cal.DueOf(o, tf.tz); err == nil && !due.Start.IsZero() {
				row.Due = text(tf.shortDue(due))
				row.Overdue = overdueBy(tf, due, row.Done) != ""
			}
			g := &groups[0]
			if row.Done {
				g = &groups[1]
			}
			g.Rows, g.Count = append(g.Rows, row), g.Count+1
		}
		for _, g := range groups {
			if g.Count > 0 {
				page.Tasks = append(page.Tasks, g)
			}
		}
		page.Lede = line(plural(all, "task"))
		filter(all)
		if all == 0 {
			page.Empty = "This list is empty."
		}
	default:
		objs, _, err := c.calQuery(ctx, c.spec.component, nil, nil)
		if err != nil {
			return err
		}
		all := len(objs)
		// By month, the newest first, as the owner's notebook (s-publish-29).
		sort.SliceStable(objs, func(i, j int) bool { return noteTime(objs[i], tf.tz).After(noteTime(objs[j], tf.tz)) })
		for _, o := range objs {
			body := cal.Description(o)
			title := noteTitle(cal.Text(o, "SUMMARY"), body)
			if !has(q, title, body) {
				continue
			}
			row := view.NoteRow{UID: cal.UID(o), Href: open(cal.UID(o)), Title: text(title), Excerpt: text(noteExcerpt(body, title)), State: view.RecordOK}
			month := ""
			if at := noteTime(o, tf.tz); !at.IsZero() {
				row.Date = text(tf.shortDay(at))
				month = at.In(tf.tz).Format("January 2006")
			}
			if n := len(page.Notes); n == 0 || string(page.Notes[n-1].Rubric) != month {
				page.Notes = append(page.Notes, view.NoteGroup{Rubric: text(month)})
			}
			g := &page.Notes[len(page.Notes)-1]
			g.Rows, g.Count = append(g.Rows, row), g.Count+1
		}
		page.Lede = line(plural(all, "note"))
		filter(all)
		if all == 0 {
			page.Empty = "This notebook is empty."
		}
	}
	return nil
}

// publicFilter is «Filter by word» of a public list, «Filter this folder» of
// a folder: a GET to the page, the word under q, the folder open in a
// nested one under in (s-publish-7, -12, -30).
func (s *Server) publicFilter(here, label, q, in string) *view.Form {
	rows := []view.FormRow{{Name: "q", Label: text(label), Kind: view.InputText, Value: text(q)}}
	if in != "" {
		rows = append(rows, view.FormRow{Name: "in", Kind: view.InputHidden, Value: text(in)})
	}
	f := s.plainForm(here, "GET", rows...)
	return &f
}

// guestDays makes the rows of a published calendar the guest's: each opens
// its event in the same link (s-publish-6), in the calendar's colour (Пс7),
// with nothing to aim at but that page.
func guestDays(page *view.PublishedPage, link user.OpenLink, open func(string) string) {
	for i := range page.Days {
		for j := range page.Days[i].Rows {
			row := &page.Days[i].Rows[j]
			row.Href, row.Menu, row.Target = open(row.UID), nil, ""
			row.Colour = linkColour(link.Target)
		}
	}
}

// publicFound is a published calendar sifted by a word (s-publish-39): what
// it holds with the word — within the link's days when it has them —, by
// day; a series once, at its next time from today, with «repeats weekly»;
// what has passed muted. «N events with «word»» under the title, no ←
// Today →.
func (s *Server) publicFound(r *http.Request, c *collectionCtx, link user.OpenLink, token, q string, fixed *[2]time.Time, page *view.PublishedPage) error {
	var from, to *time.Time
	if fixed != nil {
		from, to = &fixed[0], &fixed[1]
	}
	objs, _, err := c.calQuery(r.Context(), c.spec.component, from, to)
	if err != nil {
		return err
	}
	objs = visitorRecords(objs, link.Target)
	page.Days = s.foundDays(c.tf, kindedAll(c.listed(objs), view.KindEvent), q, c.spec.segment, false, from, to)
	n := 0
	for _, d := range page.Days {
		n += len(d.Rows)
	}
	guestDays(page, link, func(uid string) string { return s.postQuery("/p/"+token, url.Values{"uid": {uid}}) })
	page.Lede = line(plural(n, "event") + " with «" + q + "»")
	page.Filter = s.publicFilter("/p/"+token, "Filter by word", q, "")
	return nil
}

// publicFoot is the foot of a public page, one phrase on every page (note
// 122 p. 3): until when the link works.
func publicFoot(tf timeFmt, link user.OpenLink) view.Runs {
	if link.Expires != nil {
		return line("This link works until " + tf.date(*link.Expires) + ".")
	}
	return line("This link does not expire.")
}

// linkColour is the colour of the published collection as it stood when
// the link was made (Пс7, Пс12); a link made before that wears the one its
// path gives.
func linkColour(t user.PublishTarget) view.Colour {
	if v := hexColour(t.Colour); v != "" {
		return view.Colour(v)
	}
	return collectionColour(t.Col)
}

// getPublishedDownload hands the thing over as a file, as it is now: the
// collection whole, or the one record.
// getPublishedFeed is the address of the strip (s-publish-6, -7): the
// calendar's or the book's file under its own name, «personal.ics»,
// «team.vcf», for a client that subscribes to it or imports it — the file of
// Download. Another name is no address.
func (s *Server) getPublishedFeed(w http.ResponseWriter, r *http.Request) {
	link, ok := s.openLink(w, r)
	if !ok {
		return
	}
	k := view.Kind(link.Target.Kind)
	spec, known := recordKinds[segmentOf(k)]
	if !known || (k != view.KindEvent && k != view.KindContact) || link.Target.UID != "" ||
		r.PathValue("name") != collectionSlug(link.Target.Title)+exportFormats[spec.kind].ext {
		s.tokenNotFound(w, r, time.Now())
		return
	}
	s.getPublishedDownload(w, r)
}

func (s *Server) getPublishedDownload(w http.ResponseWriter, r *http.Request) {
	link, ok := s.openLink(w, r)
	if !ok {
		return
	}
	if link.Target.NoDownload {
		s.notFound(w, r)
		return
	}
	if link.Target.Kind == string(view.KindFile) {
		s.publishedFilesDownload(w, r, link)
		return
	}
	if link.Target.Attach > 0 {
		s.publishedAttachment(w, r, link, true)
		return
	}
	c, ok := s.openPublished(w, r, link)
	if !ok {
		return
	}
	ctx := r.Context()
	var objs []*model.Object
	var err error
	if uid := link.Target.UID; uid != "" {
		var obj *model.Object
		obj, err = c.find(ctx, uid)
		objs = []*model.Object{obj}
	} else if w := publishedWindow(s.timeFor(nil), link); w != nil {
		objs, _, err = c.exportRecords(ctx, &w[0], &w[1])
	} else {
		objs, _, err = c.exportRecords(ctx, nil, nil)
	}
	if err != nil {
		page := view.PublishedPage{Plain: view.Plain{Title: text(link.Target.Title), Notices: []view.Notice{publicTrouble()}}, Kind: c.spec.kind, ReadOnly: true}
		if goneErr(err) {
			page.Notices = []view.Notice{publicGone()}
			_ = s.users.MarkLinkGone(link.ID)
		}
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	f := exportFormats[c.spec.kind]
	h := w.Header()
	h.Set("Content-Type", f.media)
	if v := mime.FormatMediaType("attachment", map[string]string{"filename": collectionSlug(link.Target.Title) + f.ext}); v != "" {
		h.Set("Content-Disposition", v)
	} else {
		h.Set("Content-Disposition", "attachment")
	}
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	w.WriteHeader(http.StatusOK)
	if isCalendarKind(c.spec.kind) {
		// The file obeys what the page does (Б39).
		objs = visitorRecords(objs, link.Target)
	}
	if c.spec.kind == view.KindNote {
		// The guest's .md is the note's title, text and days: not its
		// tags, not its attachments, not what another client wrote (Пб3).
		objs = visitorNotes(objs)
	}
	if c.card != nil {
		// The file obeys the ticks the page does, or it would go round them.
		for i, o := range objs {
			objs[i] = visitorCard(o, link.Target)
		}
	}
	// A record Carrel could not read stays out of a guest's file: nothing
	// checked what of it the link may show.
	writeExport(w, c, objs, nil)
}

// visitorFields are the fields of a contact a link shows only when asked to
// (S-25.1, s-publish-3), by the name of their tick: the label of the sheet
// and the properties of the card it stands for.
var visitorFields = []struct {
	name, label string
	props       []string
}{
	{"phones", "Phone numbers", []string{"TEL"}},
	{"emails", "Email addresses", []string{"EMAIL"}},
	{"photo", "Photo", []string{"PHOTO"}},
	{"note", "Note", []string{"NOTE"}},
}

// visitorAlways are the properties of a card every link of a contact gives:
// what the card is and whose. visitorOrganisation are the ones «Name and
// organisation» adds to them, unless it was taken off.
var (
	visitorAlways       = []string{"VERSION", "KIND", "UID", "REV", "FN", "N"}
	visitorOrganisation = []string{"ORG", "TITLE"}
)

// visitorCard is a card as a visitor of a link may have it: a new card of
// the name, the organisation — unless the link was made without it — and
// the fields the link was made to show. It is
// built of what is allowed, never cut down from the card: an address, a
// birthday, a property Carrel does not know or one it will learn of later is
// not in it without anyone having to think of taking it out. The page, the
// rows of a book and the file are all made from it.
func visitorCard(o *model.Object, target user.PublishTarget) *model.Object {
	if o == nil {
		return nil
	}
	allowed := map[string]bool{}
	for _, p := range visitorAlways {
		allowed[p] = true
	}
	if !target.HideOrganisation {
		for _, p := range visitorOrganisation {
			allowed[p] = true
		}
	}
	for _, f := range visitorFields {
		if slices.Contains(target.Shows, f.name) {
			for _, p := range f.props {
				allowed[p] = true
			}
		}
	}
	kept := map[string][]model.RawValue{}
	for name, values := range o.Props() {
		if !allowed[strings.ToUpper(name)] {
			continue
		}
		for _, v := range values {
			// The value alone: its line as it came may carry a group that
			// ties it to a label the visitor is not shown.
			kept[name] = append(kept[name], model.RawValue{Text: v.Text, Params: v.Params})
		}
	}
	return model.NewObject(o.Path, o.ETag, kept)
}

// publishedWindow is the days a published calendar shows, when the link was
// made with them: the first and the day after the last. nil — the whole
// calendar.
func publishedWindow(tf timeFmt, link user.OpenLink) *[2]time.Time {
	if link.Target.From == "" || link.Target.To == "" {
		return nil
	}
	from, errFrom := time.ParseInLocation("2006-01-02", link.Target.From, tf.tz)
	to, errTo := time.ParseInLocation("2006-01-02", link.Target.To, tf.tz)
	if errFrom != nil || errTo != nil || to.Before(from) {
		return nil
	}
	return &[2]time.Time{from, to.AddDate(0, 0, 1)}
}

// withoutDescriptions is the records with their text taken out: a link made
// without descriptions shares the times, not what was said at them.
func withoutDescriptions(objs []*model.Object) []*model.Object {
	out := make([]*model.Object, 0, len(objs))
	for _, o := range objs {
		if o == nil {
			continue
		}
		if len(o.Props()["DESCRIPTION"]) > 0 {
			// A record whose text could not be taken out is left out whole:
			// never given with what the link was made to keep back.
			patch, err := model.NewPatch(model.RemoveProp{Prop: "DESCRIPTION"})
			if err != nil || patch.Apply(o) != nil {
				continue
			}
		}
		out = append(out, o)
	}
	return out
}

// allowLabel is the label of a download tick, the format monospaced
// (s-publish-2, -3; Пб5).
func allowLabel(format, rest string) view.Runs {
	return join("Allow ", monoRun(format), rest)
}
