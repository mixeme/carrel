package web

import (
	"context"
	"errors"
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

func (s *Server) writeSheet(w http.ResponseWriter, r *http.Request, sheet view.PublishSheet) {
	if render.WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, sheet, nil)
		return
	}
	s.writePage(w, r, http.StatusOK, sheet)
}

func (s *Server) publishSheet(sess *session.Session, c *collectionCtx, uid, title, how string) view.PublishSheet {
	// The head names what is published and, at its end, which (Пб1;
	// s-publish-2, -3, -19): a collection by where it lies, a record by its
	// title.
	what, subject := collectionNouns[c.spec.kind], string(c.stamp())
	if uid != "" {
		what, subject = c.spec.noun, title
	}
	return s.sheetFor(sess, c.spec.kind, what, subject, publishQuery(c.spec.segment, c.conn.ID, c.col.Path, uid), c.listPath(), how)
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
	tf := s.timeFor(sess)
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
		rows = append(rows, view.FormRow{Name: "how", Label: "How much of it", Kind: view.InputRadio, Value: text(how), Options: []view.Option{
			{Value: "range", Label: "A date range", Selected: how == "range"},
			{Value: "whole", Label: "The whole calendar", Selected: how == "whole"},
		}})
		if how == "range" {
			today := dayOf(tf.now, tf.tz)
			rows = append(rows,
				view.FormRow{Name: "from", Label: "From", Kind: view.InputDate, Mono: true},
				view.FormRow{Name: "to", Label: "To", Kind: view.InputDate, Mono: true, Hint: text("Both days are in it. " + tf.date(today) + " is today.")})
		}
	}
	lives := livesOf(kind, what == "folder")
	var terms []view.FormRow
	switch {
	case kind == view.KindContact:
		// The .vcf is off until asked for, like the fields it would carry.
		terms = append(terms, view.FormRow{Name: "download", Label: "Allow .vcf download and subscription", Kind: view.InputCheck})
	case isCalendarKind(kind):
		// The .ics goes unless taken off — a calendar's and one event's
		// alike (s-publish-2, -19) —, after what the visitor sees: it obeys
		// the same ticks.
		terms = append(terms, view.FormRow{Name: "download", Label: "Allow .ics download and subscription", Kind: view.InputCheck, Value: "on"})
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
	sheet.Actions = []view.Action{
		{ID: "publish.open", Label: "Open the link", Href: link, Method: "GET", Enabled: true},
		{ID: "publish.list", Label: "All published links", Href: s.route("/app/files/published"), Method: "GET", Enabled: true},
	}
	// Send by letter leads, primary, where letters go out (s-publish-13).
	if s.canSendLetters(sess) {
		if id := user.LinkID(token); id != "" {
			sheet.Actions = append([]view.Action{{ID: "publish.mail", Label: "Send by letter", Method: "POST", Primary: true, Enabled: true,
				Href: s.postQuery("/app/mail/compose", url.Values{"op": {"link"}, "id": {id}})}}, sheet.Actions...)
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
	s.writeLetterSheet(w, r, sess, s.publishSheet(sess, c, uid, title, r.FormValue("how")))
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
	if err != nil || i < 0 || i >= len(d.Parts) || !d.Parts[i].AsLink || d.Parts[i].LinkID != "" {
		s.notFound(w, r)
		return
	}
	p := d.Parts[i]
	apply := s.postQuery("/app/mail/compose", url.Values{"op": {"terms"}, "i": {m}})
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
			sheet := s.publishSheet(sess, c, uid, title, "range")
			for i := range sheet.Form.Sets {
				for j := range sheet.Form.Sets[i].Rows {
					row := &sheet.Form.Sets[i].Rows[j]
					switch row.Name {
					case "from":
						row.Value, row.Error = text(r.FormValue("from")), "Give both days, or publish the whole calendar."
					case "to":
						row.Value = text(r.FormValue("to"))
					}
				}
			}
			s.writeSheet(w, r, sheet)
			return
		}
	}
	publishChoices(r.Form, c.spec.kind, uid, &target)
	token, err := s.users.Publish(sess.AccountID(), key, c.conn, target, expires)
	if err != nil {
		http.Error(w, "could not publish", http.StatusInternalServerError)
		return
	}
	s.writeSheet(w, r, s.madeSheet(r, sess, s.publishSheet(sess, c, uid, title, ""), token, expires))
}

// sheetDefaults puts into a link of a kind what the sheet of publishing it
// chooses before anyone touches it (s-publish-2, -3; Б39): a calendar's,
// an event's, a task's — the time and the title, the .ics allowed; a
// contact's and a book's — the name and the organisation, no phone, address,
// photo or note, and no .vcf. A link made without that sheet — by a letter
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
	case kind == view.KindContact:
		target.Shows, target.HideOrganisation, target.NoDownload = nil, false, true
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
		if l.Gone {
			// What it opened is no longer there and Carrel could not follow
			// it (Б41): the link lives on until it is revoked or expires.
			row.Gone = "No longer there: deleted, or moved where Carrel could not follow it"
		} else {
			row.Href = s.publishedHref(sess, l.Target) // Пб41
		}
		copyLink := view.Action{ID: "publish.copy", Label: "Copy", Method: "GET", Enabled: true}
		send := view.Action{ID: "publish.mail", Label: "Send", Method: "POST", Enabled: true,
			Href: s.postQuery("/app/mail/compose", url.Values{"op": {"link"}, "id": {l.ID}})}
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
		row.Actions = append(row.Actions, view.Action{ID: "publish.revoke", Label: "Revoke", Method: "POST", Danger: true, Enabled: true,
			Href: s.postQuery("/app/publish", url.Values{"op": {"revoke"}, "id": {l.ID}})})
		page.Rows = append(page.Rows, row)
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
	return view.Notice{Role: view.NoticeAlert, Title: "The source did not answer", Body: "What was published is kept on a server that did not answer just now. Try again later."}
}

func publicGone() view.Notice {
	return view.Notice{Role: view.NoticeInfo, Title: "No longer there", Body: "What was published has been deleted or moved where it was kept."}
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
	c, ok := s.openPublished(w, r, link)
	if !ok {
		return
	}
	token := r.PathValue("token")
	page := view.PublishedPage{
		Plain:    view.Plain{Title: text(link.Target.Title)},
		Kind:     c.spec.kind,
		ReadOnly: true,
		// A list says the owner can take it back (s-publish-6, -7); one
		// record says its day only (s-publish-8).
		Foot: publicFoot(c.tf, link, footRevoke),
		// The button names the file it gives (s-publish-6, -7, -8): a note
		// goes as Markdown, never as its raw iCalendar.
		Actions: []view.Action{{ID: "published.download", Label: text("Download " + exportFormats[c.spec.kind].ext), Icon: "i-download", Href: s.route("/p/" + token + "/download"), Method: "GET", Enabled: true}},
	}
	if link.Target.NoDownload {
		// The link was made without the file: the page stays, the .ics and
		// the subscription are not offered and not served (s-publish-2).
		page.Actions = nil
	}
	ctx := r.Context()
	if uid := link.Target.UID; uid != "" {
		obj, err := c.find(ctx, uid)
		switch {
		case notFoundErr(err):
			page.Notices, page.Actions = []view.Notice{publicGone()}, nil
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
			if obj == nil {
				// Built of what the link allows it could not be: nothing
				// of it goes out.
				page.Notices, page.Actions = []view.Notice{publicTrouble()}, nil
				break
			}
			s.publicRecord(c, obj, &page)
		}
		page.Foot = publicFoot(c.tf, link, "")
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	if err := s.publicCollection(r, c, link, token, &page); err != nil {
		page.Notices, page.Actions = []view.Notice{publicTrouble()}, nil
		if goneErr(err) {
			// A collection removed is gone, not a server that did not
			// answer (Б41): the guest is not told to try later forever.
			page.Notices = []view.Notice{publicGone()}
			_ = s.users.MarkLinkGone(link.ID)
		}
	}
	guestRows(&page)
	s.writePage(w, r, http.StatusOK, page)
}

// guestRows takes the owner's own marks off the rows a guest sees (Пс23;
// s-publish-6 draws the caption empty): «linked ×2», «2 sources» and the
// like are how the owner keeps their records, and tell the guest nothing
// but about the owner.
func guestRows(page *view.PublishedPage) {
	for i := range page.Contacts {
		for j := range page.Contacts[i].Rows {
			page.Contacts[i].Rows[j].Badges = nil
		}
	}
	for i := range page.Days {
		for j := range page.Days[i].Rows {
			page.Days[i].Rows[j].Badges = nil
		}
	}
	for i := range page.Tasks {
		page.Tasks[i].Badges = nil
	}
	for i := range page.Notes {
		page.Notes[i].Badges = nil
	}
}

// publicRecord fills the page with one record — its own fields, nothing that
// leads back into the owner's Carrel. The line under the title of an event
// is when and where (s-publish-16); no other lede — the badge «read-only»
// says what the page is.
func (s *Server) publicRecord(c *collectionCtx, obj *model.Object, page *view.PublishedPage) {
	tf := c.tf
	page.Title, page.Lede = text(recordTitle(c.spec.kind, obj)), nil
	switch c.spec.kind {
	case view.KindContact:
		for _, f := range contactFieldsOf(obj) {
			page.Fields = append(page.Fields, f.Field)
		}
	case view.KindEvent:
		var head []string
		if tm, err := cal.TimingOf(obj, tf.tz); err == nil && !tm.Start.IsZero() {
			page.Fields = append(page.Fields, view.Field{Label: "When", Value: text(tf.when(tm))})
			head = append(head, tf.when(tm))
		}
		if where := strings.TrimSpace(cal.Text(obj, "LOCATION")); where != "" {
			head = append(head, where)
		}
		page.Lede = line(strings.Join(head, " · "))
		page.Fields = append(page.Fields, calendarFields(obj)...)
		page.Body = text(cal.Description(obj))
	case view.KindTask:
		if due, err := cal.DueOf(obj, tf.tz); err == nil && !due.Start.IsZero() {
			page.Fields = append(page.Fields, view.Field{Label: "Due", Value: text(tf.timeOrDate(due))})
		}
		page.Fields = append(page.Fields, calendarFields(obj)...)
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

// publicCollection fills the page with the collection read now: a book, one
// week of a calendar with its range, a task list, a notebook.
func (s *Server) publicCollection(r *http.Request, c *collectionCtx, link user.OpenLink, token string, page *view.PublishedPage) error {
	ctx := r.Context()
	tf := c.tf
	switch c.spec.kind {
	case view.KindContact:
		objs, err := c.contactsAll(ctx)
		if err != nil {
			return err
		}
		sort.SliceStable(objs, func(i, j int) bool {
			return strings.ToLower(contactName(objs[i])) < strings.ToLower(contactName(objs[j]))
		})
		// By letter, as the book in Carrel (Пс10; s-publish-7), each row
		// marked with the book's colour (Пс12).
		for _, o := range objs {
			o = visitorCard(o, link.Target)
			name := contactName(o)
			row := view.ContactRow{
				UID: card.UID(o), Avatar: view.Avatar{Initials: text(initials(name)), Colour: collectionColour(card.UID(o))},
				Name: text(name), Phone: text(firstValue(o, "TEL")), Email: text(firstValue(o, "EMAIL")),
				Org: text(structured(firstValue(o, "ORG"))), State: view.RecordOK, Colour: linkColour(link.Target),
			}
			rubric := rubricOf(name)
			if n := len(page.Contacts); n == 0 || string(page.Contacts[n-1].Rubric) != rubric {
				page.Contacts = append(page.Contacts, view.ContactGroup{Rubric: text(rubric)})
			}
			g := &page.Contacts[len(page.Contacts)-1]
			g.Rows, g.Count = append(g.Rows, row), g.Count+1
		}
		page.Lede = line(plural(len(objs), "contact") + listReadNow)
	case view.KindEvent:
		from, to := weekWindow(tf, r.URL.Query().Get("from"))
		fixed := publishedWindow(tf, link)
		if fixed != nil {
			from, to = fixed[0], fixed[1]
		}
		objs, _, err := c.calQuery(ctx, c.spec.component, &from, &to)
		if err != nil {
			return err
		}
		objs = visitorRecords(objs, link.Target)
		page.Days, _, _ = s.agendaDays(tf, kindedAll(c.listed(objs), view.KindEvent), from, to, c.spec.segment, false, nil)
		for i := range page.Days {
			for j := range page.Days[i].Rows {
				// A row of the visitor's page opens nothing: no address,
				// and nothing for it to aim at (C-5).
				row := &page.Days[i].Rows[j]
				row.Href, row.Menu, row.Target = "", nil, ""
				row.Colour = linkColour(link.Target) // the calendar's (Пс7)
			}
		}
		// The days in one phrase (Пс5; s-publish-6).
		page.Lede = line(tf.window(from, to) + " · the page is read from the server on every visit")
		week := func(t time.Time) string {
			return s.postQuery("/p/"+token, url.Values{"from": {t.Format("2006-01-02")}})
		}
		page.Range = &view.Range{
			From: text(tf.date(from)), To: text(tf.date(to.AddDate(0, 0, -1))),
			Presets: []view.Action{
				{ID: "range.previous", Label: "Previous week", Href: week(from.AddDate(0, 0, -7)), Method: "GET", Enabled: true},
				{ID: "range.current", Label: "This week", Href: s.route("/p/" + token), Method: "GET", Enabled: true},
				{ID: "range.next", Label: "Next week", Href: week(to), Method: "GET", Enabled: true},
			},
		}
		if fixed != nil {
			// The days are the link's, not the reader's: there is nowhere
			// to step, and a preset that moves out of them would lie.
			page.Range.Presets = nil
		}
	case view.KindTask:
		objs, _, err := c.calQuery(ctx, c.spec.component, nil, nil)
		if err != nil {
			return err
		}
		objs = visitorRecords(objs, link.Target)
		for _, o := range objs {
			row := view.TaskRow{UID: cal.UID(o), Done: taskDone(o), Summary: text(recordTitle(view.KindTask, o)), Percent: percentOf(o), State: view.RecordOK}
			if due, err := cal.DueOf(o, tf.tz); err == nil && !due.Start.IsZero() {
				row.Due = text(tf.shortDue(due))
			}
			page.Tasks = append(page.Tasks, row)
		}
		sort.SliceStable(page.Tasks, func(i, j int) bool { return !page.Tasks[i].Done && page.Tasks[j].Done })
		page.Lede = line(plural(len(page.Tasks), "task") + listReadNow)
	default:
		objs, _, err := c.calQuery(ctx, c.spec.component, nil, nil)
		if err != nil {
			return err
		}
		sort.SliceStable(objs, func(i, j int) bool { return noteTime(objs[i], tf.tz).After(noteTime(objs[j], tf.tz)) })
		for _, o := range objs {
			body := cal.Description(o)
			title := noteTitle(cal.Text(o, "SUMMARY"), body)
			row := view.NoteRow{UID: cal.UID(o), Title: text(title), Excerpt: text(noteExcerpt(body, title)), State: view.RecordOK}
			if at := noteTime(o, tf.tz); !at.IsZero() {
				row.Date = text(tf.shortDay(at))
			}
			page.Notes = append(page.Notes, row)
		}
		page.Lede = line(plural(len(page.Notes), "note") + listReadNow)
	}
	return nil
}

// listReadNow follows the count under the title of a published list
// (s-publish-7): what the guest sees is not a copy.
const listReadNow = " · the list is read from the server on every visit"

// footRevoke is the foot of a published list after its day (s-publish-6,
// -7).
const footRevoke = "revoke"

// publicFoot is the foot of a public page (Пс2): until when the link works,
// then what the frames say after it — that the owner can take a list back
// (footRevoke), or how the page is read («the file is read from the server
// on every visit», s-publish-10); nothing more for one record or a file.
func publicFoot(tf timeFmt, link user.OpenLink, after string) view.Runs {
	until := "This link does not expire"
	if link.Expires != nil {
		until = "This link works until " + tf.date(*link.Expires)
	}
	switch after {
	case "":
		return line(until + ".")
	case footRevoke:
		return line(until + ". The owner can revoke it at any moment.")
	}
	return line(until + " · " + after)
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
