package web

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"sort"
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
	what := collectionNouns[c.spec.kind]
	if uid != "" {
		what = c.spec.noun
	}
	return s.sheetFor(sess, c.spec.kind, what, title, c.stamp(), publishQuery(c.spec.segment, c.conn.ID, c.col.Path, uid), c.listPath(), how)
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
func (s *Server) sheetFor(sess *session.Session, kind view.Kind, what, title string, where view.Stamp, q url.Values, cancel, how string) view.PublishSheet {
	tf := s.timeFor(sess)
	create := view.Action{ID: "publish.create", Label: "Create link", Href: s.postQuery("/app/publish", q), Method: "POST", Primary: true, Enabled: true}
	back := view.Action{ID: "publish.cancel", Label: "Cancel", Href: s.route(cancel), Method: "GET", Enabled: true}
	sheet := view.PublishSheet{
		Kind:    kind,
		Target:  text(strings.ToUpper(what[:1]) + what[1:] + " «" + title + "» · " + string(where)),
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
	if kind == view.KindContact {
		// The .vcf is off until asked for, like the fields it would carry.
		rows = append(rows, view.FormRow{Name: "download", Label: "Allow .vcf download and subscription", Kind: view.InputCheck})
	}
	rows = append(rows, view.FormRow{Name: "lives", Label: "Link lives for", Kind: view.InputSelect, Value: text(lives), Options: []view.Option{
		{Value: "7", Label: "7 days", Selected: lives == "7"},
		{Value: "14", Label: "14 days", Selected: lives == "14"},
		{Value: "30", Label: "30 days", Selected: lives == "30"},
		{Value: "never", Label: "No expiry", Separator: true}, // s-publish-14: only Revoke closes it
	}})
	if kind == view.KindEvent && q.Get("uid") == "" {
		rows = append(rows,
			view.FormRow{Name: "download", Label: "Allow .ics download and subscription", Kind: view.InputCheck, Value: "on"})
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
		sheet.Form.Sets = append([]view.FieldSet{calendarSees()}, sheet.Form.Sets...)
	}
	sheet.Form.Action = s.postQuery("/app/publish", q)
	sheet.Form.Actions = sheet.Actions
	return sheet
}

// publishWindow is the range a published calendar shows: both days or
// neither. A range that ends before it starts is no range at all — the whole
// calendar, as the sheet says by default.
func publishWindow(r *http.Request) (from, to string) {
	from, to = strings.TrimSpace(r.FormValue("from")), strings.TrimSpace(r.FormValue("to"))
	f, errFrom := time.Parse("2006-01-02", from)
	t, errTo := time.Parse("2006-01-02", to)
	if errFrom != nil || errTo != nil || t.Before(f) {
		return "", ""
	}
	return from, to
}

// madeSheet is the sheet once the link is made: the link, shown this once.
func (s *Server) madeSheet(sess *session.Session, sheet view.PublishSheet, token string, expires *time.Time) view.PublishSheet {
	link := s.route("/p/" + token)
	sheet.Link = text(link)
	sheet.Expires = "No expiry. Revoke it when it is no longer needed."
	if expires != nil {
		sheet.Expires = text("Until " + s.timeFor(sess).date(*expires))
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
	ttl := publish.DefaultTTL
	lives := r.FormValue("lives")
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
	if r.FormValue("expires") == "never" {
		return nil
	}
	t := s.now().Add(ttl)
	return &t
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
		s.writeSheet(w, r, s.fileSheet(sess, fc, segs, dir))
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
	s.writeSheet(w, r, s.publishSheet(sess, c, uid, title, r.FormValue("how")))
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
	}
	if c.spec.kind == view.KindEvent && uid == "" {
		// The sheet of a whole calendar (s-publish-2): «how» is always sent
		// by it. A post without it — an old client, a plain POST — publishes
		// as before: the whole calendar, downloadable, with its descriptions.
		switch r.FormValue("how") {
		case "whole":
		case "range":
			target.From, target.To = publishWindow(r)
			if target.From == "" {
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
		if r.FormValue("how") != "" {
			target.NoDownload = checkbox(r, "download") != "on"
		}
	}
	if isCalendarKind(c.spec.kind) {
		// What the visitor sees (Б39): the time always, and what is
		// ticked. A post without the sheet's form — a plain POST — gives
		// what the sheet ticks before anyone does: the time and the title.
		target.Shows = []string{calendarSeesTime}
		for _, f := range calendarVisitorFields {
			if r.Form.Has("lives") && checkbox(r, f.name) == "on" || !r.Form.Has("lives") && f.on {
				target.Shows = append(target.Shows, f.name)
			}
		}
	}
	if c.spec.kind == view.KindContact {
		// The sheet of a contact and of a book (s-publish-3): what is not
		// ticked is not shown, and the file is one of the things asked for.
		// A post without the ticks — a plain POST — publishes the name and
		// the organisation and no file. «Name and organisation» stands on
		// the sheet, so only a post of the sheet takes it off: one that says
		// how long the link lives, as the sheet's form always does.
		target.HideOrganisation = r.Form.Has("lives") && checkbox(r, "name") != "on"
		for _, f := range visitorFields {
			if checkbox(r, f.name) == "on" {
				target.Shows = append(target.Shows, f.name)
			}
		}
		target.NoDownload = checkbox(r, "download") != "on"
	}
	token, err := s.users.Publish(sess.AccountID(), key, c.conn, target, expires)
	if err != nil {
		http.Error(w, "could not publish", http.StatusInternalServerError)
		return
	}
	s.writeSheet(w, r, s.madeSheet(sess, s.publishSheet(sess, c, uid, title, ""), token, expires))
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
		Plain:    view.Plain{Title: text(link.Target.Title), Lede: line("Read-only. Published from Carrel.")},
		Kind:     c.spec.kind,
		ReadOnly: true,
		// The button names the file it gives (s-publish-6, -7, -8): a note
		// goes as Markdown, never as its raw iCalendar.
		Actions: []view.Action{{ID: "published.download", Label: text("Download " + exportFormats[c.spec.kind].ext), Icon: "i-down", Href: s.route("/p/" + token + "/download"), Method: "GET", Enabled: true}},
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
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	if err := s.publicCollection(r, c, link, token, &page); err != nil {
		page.Notices, page.Actions = []view.Notice{publicTrouble()}, nil
	}
	s.writePage(w, r, http.StatusOK, page)
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
		for _, o := range objs {
			o = visitorCard(o, link.Target)
			name := contactName(o)
			page.Contacts = append(page.Contacts, view.ContactRow{
				UID: card.UID(o), Avatar: view.Avatar{Initials: text(initials(name)), Colour: collectionColour(card.UID(o))},
				Name: text(name), Phone: text(firstValue(o, "TEL")), Email: text(firstValue(o, "EMAIL")),
				Org: text(structured(firstValue(o, "ORG"))), State: view.RecordOK,
			})
		}
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
				page.Days[i].Rows[j].Href, page.Days[i].Rows[j].Menu = "", nil
			}
		}
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
	}
	return nil
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
		if notFoundErr(err) {
			page.Notices = []view.Notice{publicGone()}
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
