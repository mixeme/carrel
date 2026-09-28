package web

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"net/url"
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
	if WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, TargetSheet, sheet, nil)
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
// subscribed to, and whether the text of its events goes with the times,
// off unless ticked (s-publish-2).
func (s *Server) sheetFor(sess *session.Session, kind view.Kind, what, title string, where view.Stamp, q url.Values, cancel, how string) view.PublishSheet {
	tf := s.timeFor(sess)
	create := view.Action{ID: "publish.create", Label: "Create link", Href: s.postQuery("/app/publish", q), Method: "POST", Enabled: true}
	back := view.Action{ID: "publish.cancel", Label: "Cancel", Href: s.route(cancel), Method: "GET", Enabled: true}
	sheet := view.PublishSheet{
		Kind:    kind,
		Target:  text(strings.ToUpper(what[:1]) + what[1:] + " «" + title + "» · " + string(where)),
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
				view.FormRow{Name: "from", Label: "From", Kind: view.InputDate},
				view.FormRow{Name: "to", Label: "To", Kind: view.InputDate, Hint: text("Both days are in it. " + tf.date(today) + " is today.")})
		}
	}
	rows = append(rows, view.FormRow{Name: "lives", Label: "Link lives for", Kind: view.InputSelect, Value: "30", Options: []view.Option{
		{Value: "7", Label: "7 days"},
		{Value: "14", Label: "14 days"},
		{Value: "30", Label: "30 days", Selected: true},
		{Value: "never", Label: "No expiry", Separator: true}, // s-publish-14: only Revoke closes it
	}})
	if kind == view.KindEvent && q.Get("uid") == "" {
		rows = append(rows,
			view.FormRow{Name: "download", Label: "Allow .ics download and subscription", Kind: view.InputCheck, Value: "on"},
			view.FormRow{Name: "descriptions", Label: "Show event descriptions", Kind: view.InputCheck})
	}
	sheet.Form = s.plainForm("/app/publish", "POST", rows...)
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
	if s.canSendLetters(sess) {
		if id := user.LinkID(token); id != "" {
			sheet.Actions = append(sheet.Actions, view.Action{ID: "publish.mail", Label: "Send by letter", Method: "POST", Enabled: true,
				Href: s.postQuery("/app/mail/compose", url.Values{"op": {"link"}, "id": {id}})})
		}
	}
	return sheet
}

// publishExpiry is the lifetime the form chose: 7, 14 or 30 days, or none.
// A post that names none — a plain POST, an old client — gets the default.
// «expires=never» is how the sheet asked for no expiry before «Link lives
// for» (24.09.2026), and still means it.
func (s *Server) publishExpiry(r *http.Request) *time.Time {
	ttl := publish.DefaultTTL
	switch r.FormValue("lives") {
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
	expires := s.publishExpiry(r)
	if r.FormValue("k") == filesSpec.segment {
		s.postPublishFile(w, r, sess, expires)
		return
	}
	c, uid, ok := s.publishTarget(r, sess)
	if !ok {
		s.notFound(w, r)
		return
	}
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
			target.HideDescriptions = checkbox(r, "descriptions") != "on"
		}
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
// Copy, Send (with mail on, 7.18 of the mockup) and Revoke. A link made
// before links were kept has no link to show: Copy and Send are there,
// switched off, with the reason.
func (s *Server) getFilesPublished(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	// The frame stands it in the settings rail (s-publish-1): the list is
	// reached from the files as well, but it is the person's, not a folder's.
	page := view.PublishedLinksPage{Shell: s.withRail(s.buildShell(sess, "Published links"), s.settingsRail(sess, "/app/files/published"))}
	blob := s.catalogOf(sess)
	tf, now := s.timeFor(sess), s.now()
	mail := s.canSendLetters(sess)
	for _, l := range s.users.PublishedLinks(sess.AccountID()) {
		host := l.Target.AccountID
		if conn, ok := blob.Connection(l.Target.AccountID); ok {
			host = connLabel(conn)
		}
		label := l.Target.Col
		if col, ok := blob.Collection(l.Target.AccountID, l.Target.Col); ok {
			label = collectionLabel(col)
		}
		row := view.PublishedRow{
			Title: text(publishedTitle(l.Target)), Kind: view.Kind(l.Target.Kind), Stamp: stamp(host, label),
			Created: view.When(tf.date(l.CreatedAt)), Expires: "No expiry", LastSeen: "never opened", Opens: l.Opens,
		}
		if l.ExpiresAt != nil {
			row.Expires = text(tf.date(*l.ExpiresAt))
		}
		if l.LastSeen != nil {
			row.LastSeen = tf.formatWhen(*l.LastSeen, false, now)
		}
		copyLink := view.Action{ID: "publish.copy", Label: "Copy", Method: "GET", Enabled: true}
		send := view.Action{ID: "publish.mail", Label: "Send", Method: "POST", Enabled: true,
			Href: s.postQuery("/app/mail/compose", url.Values{"op": {"link"}, "id": {l.ID}})}
		if token := blob.Links[l.ID]; token != "" {
			link := s.route("/p/" + token)
			row.Link, copyLink.Href = text(link), link
		} else {
			const kept = "This link was made before links were kept: it cannot be shown again. Revoke it and publish anew."
			copyLink.Enabled, copyLink.Reason = false, kept
			send.Enabled, send.Href, send.Reason = false, "", kept
		}
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
		Plain:    view.Plain{Title: text(link.Target.Title), Lede: "Read-only. Published from Carrel."},
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
	page.Title, page.Lede = text(recordTitle(c.spec.kind, obj)), ""
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
		page.Lede = text(strings.Join(head, " · "))
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
		page.Days, _, _ = s.agendaDays(tf, kindedAll(c.listed(objs), view.KindEvent), from, to, c.spec.segment, false)
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
		objs, err = c.exportRecords(ctx, &w[0], &w[1])
	} else {
		objs, err = c.exportRecords(ctx, nil, nil)
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
	if link.Target.HideDescriptions {
		objs = withoutDescriptions(objs)
	}
	writeExport(w, c, objs)
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
			patch, err := model.NewPatch(model.RemoveProp{Prop: "DESCRIPTION"})
			if err == nil && patch.Apply(o) == nil {
				out = append(out, o)
				continue
			}
		}
		out = append(out, o)
	}
	return out
}
