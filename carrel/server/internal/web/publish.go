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

// publishAction opens the sheet for a collection or, with a UID, a record.
func (s *Server) publishAction(c *collectionCtx, uid string) view.Action {
	label := "Publish…"
	return view.Action{ID: "publish", Label: text(label), Icon: "i-link", Method: "GET", Enabled: true,
		Href: s.postQuery("/app/publish", publishQuery(c.spec.segment, c.conn.ID, c.col.Path, uid))}
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

func (s *Server) publishSheet(sess *session.Session, c *collectionCtx, uid, title string) view.PublishSheet {
	what := collectionNouns[c.spec.kind]
	if uid != "" {
		what = c.spec.noun
	}
	return s.sheetFor(sess, c.spec.kind, what, title, c.stamp(), publishQuery(c.spec.segment, c.conn.ID, c.col.Path, uid), c.listPath())
}

// sheetFor is the sheet of any target: what, where, for how long, and the
// two ways to make it — thirty days or, explicitly, no expiry.
func (s *Server) sheetFor(sess *session.Session, kind view.Kind, what, title string, where view.Stamp, q url.Values, cancel string) view.PublishSheet {
	tf := s.timeFor(sess)
	until := time.Now().Add(publish.DefaultTTL)
	forever := cloneValues(q)
	forever.Set("expires", "never")
	return view.PublishSheet{
		Kind:    kind,
		Target:  text(strings.ToUpper(what[:1]) + what[1:] + " «" + title + "» · " + string(where)),
		Expires: text("30 days, until " + tf.date(until)),
		Actions: []view.Action{
			{ID: "publish.create", Label: "Publish for 30 days", Href: s.postQuery("/app/publish", q), Method: "POST", Enabled: true},
			{ID: "publish.forever", Label: "Publish without an expiry", Href: s.postQuery("/app/publish", forever), Method: "POST", Enabled: true},
			{ID: "publish.cancel", Label: "Cancel", Href: s.route(cancel), Method: "GET", Enabled: true},
		},
	}
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
	if s.mailOn() {
		if id := user.LinkID(token); id != "" {
			sheet.Actions = append(sheet.Actions, view.Action{ID: "publish.mail", Label: "Send by letter", Method: "POST", Enabled: true,
				Href: s.postQuery("/app/mail/compose", url.Values{"op": {"link"}, "id": {id}})})
		}
	}
	return sheet
}

// publishExpiry is the lifetime the form chose.
func publishExpiry(r *http.Request) *time.Time {
	if r.FormValue("expires") == "never" {
		return nil
	}
	t := time.Now().Add(publish.DefaultTTL)
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
	s.writeSheet(w, r, s.publishSheet(sess, c, uid, title))
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
	expires := publishExpiry(r)
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
	token, err := s.users.Publish(sess.AccountID(), key, c.conn, user.PublishTarget{
		AccountID: c.conn.ID, Kind: string(c.spec.kind), Col: c.col.Path, UID: uid, Title: title,
	}, expires)
	if err != nil {
		http.Error(w, "could not publish", http.StatusInternalServerError)
		return
	}
	s.writeSheet(w, r, s.madeSheet(sess, s.publishSheet(sess, c, uid, title), token, expires))
}

// getFilesPublished lists the person's live links: what each opens, until
// when, when it was last opened, the link itself from the blob with Copy
// and, with mail on, Send this link — and Revoke. A link made before links
// were kept has no link to show.
func (s *Server) getFilesPublished(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	page := view.PublishedLinksPage{Shell: s.buildShell(sess, "Published links")}
	blob := s.catalogOf(sess)
	tf, now := s.timeFor(sess), time.Now()
	for _, l := range s.users.PublishedLinks(sess.AccountID()) {
		host := l.Target.AccountID
		if conn, ok := blob.Connection(l.Target.AccountID); ok {
			host = serverHost(conn.Server)
		}
		label := l.Target.Col
		if col, ok := blob.Collection(l.Target.AccountID, l.Target.Col); ok {
			label = collectionLabel(col)
		}
		row := view.PublishedRow{
			Title: text(l.Target.Title), Kind: view.Kind(l.Target.Kind), Stamp: stamp(host, label),
			Created: tf.formatWhen(l.CreatedAt, true, now), Expires: "No expiry",
			Actions: []view.Action{{ID: "publish.revoke", Label: "Revoke", Method: "POST", Danger: true, Enabled: true,
				Href: s.postQuery("/app/publish", url.Values{"op": {"revoke"}, "id": {l.ID}})}},
		}
		if l.ExpiresAt != nil {
			row.Expires = text(tf.date(*l.ExpiresAt))
		}
		if token := blob.Links[l.ID]; token != "" {
			link := s.route("/p/" + token)
			row.Link = text(link)
			more := []view.Action{{ID: "publish.copy", Label: "Copy", Href: link, Method: "GET", Enabled: true}}
			if s.mailOn() {
				more = append(more, view.Action{ID: "publish.mail", Label: "Send this link", Method: "POST", Enabled: true,
					Href: s.postQuery("/app/mail/compose", url.Values{"op": {"link"}, "id": {l.ID}})})
			}
			row.Actions = append(more, row.Actions...)
		}
		if l.LastSeen != nil {
			row.LastSeen = tf.formatWhen(*l.LastSeen, true, now)
		}
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
		Actions:  []view.Action{{ID: "published.download", Label: "Download", Icon: "i-down", Href: s.route("/p/" + token + "/download"), Method: "GET", Enabled: true}},
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
	if err := s.publicCollection(r, c, token, &page); err != nil {
		page.Notices, page.Actions = []view.Notice{publicTrouble()}, nil
	}
	s.writePage(w, r, http.StatusOK, page)
}

// publicRecord fills the page with one record — its own fields, nothing that
// leads back into the owner's Carrel.
func (s *Server) publicRecord(c *collectionCtx, obj *model.Object, page *view.PublishedPage) {
	tf := c.tf
	page.Title = text(recordTitle(c.spec.kind, obj))
	switch c.spec.kind {
	case view.KindContact:
		for _, f := range contactFieldsOf(obj) {
			page.Fields = append(page.Fields, f.Field)
		}
	case view.KindEvent:
		if tm, err := cal.TimingOf(obj, tf.tz); err == nil && !tm.Start.IsZero() {
			page.Fields = append(page.Fields, view.Field{Label: "When", Value: text(tf.when(tm))})
		}
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
func (s *Server) publicCollection(r *http.Request, c *collectionCtx, token string, page *view.PublishedPage) error {
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
		objs, _, err := c.calQuery(ctx, c.spec.component, &from, &to)
		if err != nil {
			return err
		}
		page.Days, _, _ = s.agendaDays(tf, c.listed(objs), from, to, c.spec.segment, false)
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
	case view.KindTask:
		objs, _, err := c.calQuery(ctx, c.spec.component, nil, nil)
		if err != nil {
			return err
		}
		for _, o := range objs {
			row := view.TaskRow{UID: cal.UID(o), Done: taskDone(o), Summary: text(recordTitle(view.KindTask, o)), Percent: percentOf(o), State: view.RecordOK}
			if due, err := cal.DueOf(o, tf.tz); err == nil && !due.Start.IsZero() {
				row.Due = text(tf.timeOrDate(due))
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
				row.Date = text(tf.date(at))
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
	writeExport(w, c, objs)
}
