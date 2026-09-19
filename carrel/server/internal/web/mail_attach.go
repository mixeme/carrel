package web

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/fanout"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// The attach sheet (S-26.2): sources on the left, what they hold on the
// right, and below it the line the letter will get — file or link — before
// Attach is pressed. Only the person's own collections: somebody else's
// subscription is read here, not handed on.

const attachRowsMax = 200

// attachQuery is the sheet's address with one thing changed.
func attachQuery(r *http.Request, set url.Values) url.Values {
	q := url.Values{}
	for _, k := range []string{"what", "a", "c", "f", "q", "from", "to"} {
		if v := r.FormValue(k); v != "" {
			q.Set(k, v)
		}
	}
	for k, v := range set {
		if len(v) == 0 || v[0] == "" {
			q.Del(k)
			continue
		}
		q[k] = v
	}
	return q
}

func (s *Server) getMailAttach(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	k, ok := attachKindOf(r.FormValue("what"))
	if !ok || !s.mailOn() {
		s.notFound(w, r)
		return
	}
	s.writeMailSheet(w, r, s.attachSheet(r, sess, k))
}

// postMailAttach adds the chosen thing to the letter and goes back to it.
func (s *Server) postMailAttach(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	k, ok := attachKindOf(r.FormValue("what"))
	if !ok || !s.mailOn() {
		s.notFound(w, r)
		return
	}
	part, err := s.chosenPart(r, sess, k)
	if err != nil {
		sheet := s.attachSheet(r, sess, k)
		sheet.Actions = append([]view.Action{{ID: "mail.attach.error", Label: text(err.Error()), Method: "GET", Reason: text(err.Error())}}, sheet.Actions...)
		s.writeMailSheet(w, r, sheet)
		return
	}
	d := loadMailDraft(sess)
	d.Parts = s.reckonLinks(append(d.Parts, part))
	saveMailDraft(sess, d)
	s.redirect(w, r, "/app/mail/compose")
}

func (s *Server) attachSheet(r *http.Request, sess *session.Session, k attachKind) view.MailAttachSheet {
	blob := s.catalogOf(sess)
	account, colPath := r.FormValue("a"), r.FormValue("c")
	if account == "" {
		account, colPath = firstTicked(blob, k.Kind)
	}
	sheet := view.MailAttachSheet{Rail: s.attachRail(r, blob, k, account, colPath)}
	switch {
	case k.Kind == view.KindFile:
		sheet.Rows = s.attachFileRows(r, sess, k, account, colPath)
	case k.What != "book" && k.What != "range":
		sheet.Rows = s.attachRecordRows(r, sess, blob, k, account, colPath)
	}
	cancel := view.Action{ID: "mail.attach.cancel", Label: "Cancel", Href: s.route("/app/mail/compose"), Method: "GET", Enabled: true}
	part, err := s.chosenPart(r, sess, k)
	if err != nil {
		sheet.Actions = []view.Action{cancel}
		return sheet
	}
	d := loadMailDraft(sess)
	parts := s.reckonLinks(append(d.Parts, part))
	part = parts[len(parts)-1]
	sheet.Preview = []view.MailAttachRow{attachRow(part)}
	if k.What == "range" {
		// The days go by the address, so the preview — file or link, and
		// its size — follows them before Attach (owner's decision 19.09.2026).
		sheet.Range = &view.Form{
			Action: s.route("/app/mail/attach"), Method: "GET",
			Sets: []view.FieldSet{{Rows: []view.FormRow{
				{Name: "what", Kind: view.InputHidden, Value: "range"},
				{Name: "a", Kind: view.InputHidden, Value: text(part.Account)},
				{Name: "c", Kind: view.InputHidden, Value: text(part.Col)},
				{Name: "from", Label: "From", Kind: view.InputDate, Value: text(part.From)},
				{Name: "to", Label: "To", Kind: view.InputDate, Value: text(part.To), Hint: "Both days are in the file."},
			}}},
			Actions: []view.Action{{ID: "mail.attach.range", Label: "Show", Method: "GET", Enabled: true}},
		}
	}
	attach := func(label string, format string) view.Action {
		q := attachQuery(r, url.Values{"uid": {r.FormValue("uid")}, "file": {r.FormValue("file")}, "format": {format}})
		return view.Action{ID: "mail.attach.add", Label: text(label), Href: s.postQuery("/app/mail/attach", q), Method: "POST", Enabled: true}
	}
	if k.What == "note" {
		// Markdown is what a person opens and reads; the source .ics is the
		// faithful copy, for a client that reads VJOURNAL.
		sheet.Actions = []view.Action{attach("Attach as Markdown · .md", "md"), attach("Attach the source · .ics", "ics"), cancel}
		return sheet
	}
	sheet.Actions = []view.Action{attach("Attach", ""), cancel}
	return sheet
}

func attachRow(p mailPart) view.MailAttachRow {
	row := view.MailAttachRow{Kind: p.Kind, Title: text(p.Title), AsLink: p.AsLink}
	switch {
	case p.Folder:
		row.Format = "folder"
	case p.Name != "":
		row.Format = text(p.Name[strings.LastIndex(p.Name, ".")+1:])
		row.Size = formatSize(p.Size)
	}
	return row
}

// firstTicked is where the sheet opens: the first ticked collection of the
// kind, the person's own.
func firstTicked(blob sealed.Blob, k view.Kind) (string, string) {
	for _, pick := range blob.Selected {
		if pick.Kind != k {
			continue
		}
		conn, ok := blob.Connection(pick.AccountID)
		col, okCol := blob.Collection(pick.AccountID, pick.Path)
		if ok && okCol && !conn.Feed && !col.Gone {
			return pick.AccountID, pick.Path
		}
	}
	return "", ""
}

func (s *Server) attachRail(r *http.Request, blob sealed.Blob, k attachKind, account, colPath string) view.Rail {
	rail := view.Rail{About: s.aboutAction()}
	for _, conn := range blob.Connections {
		if conn.Feed {
			continue
		}
		sec := view.RailSection{Account: text(serverHost(conn.Server)), AccountID: conn.ID, State: view.CollectionOK}
		for _, col := range blob.CollectionsOf(conn.ID) {
			if !col.Has(k.Kind) || col.Gone {
				continue
			}
			row := view.SourceRow{Label: text(collectionLabel(col)), Colour: catalogColour(col), Tick: view.TickOff, State: view.CollectionOK,
				Href: s.route("/app/mail/attach?" + url.Values{"what": {k.What}, "a": {conn.ID}, "c": {col.Path}}.Encode())}
			if conn.ID == account && col.Path == colPath {
				row.Tick = view.TickOn
			}
			sec.Rows = append(sec.Rows, row)
		}
		if len(sec.Rows) > 0 {
			rail.Sections = append(rail.Sections, sec)
		}
	}
	return rail
}

// attachRecordRows are the records of the chosen collection, sifted by q.
func (s *Server) attachRecordRows(r *http.Request, sess *session.Session, blob sealed.Blob, k attachKind, account, colPath string) []view.FindRow {
	conn, ok := blob.Connection(account)
	col, okCol := blob.Collection(account, colPath)
	if !ok || !okCol || conn.Feed || col.Gone {
		return nil
	}
	src := s.openSource(conn.ID+":"+col.Path, conn, col)
	src.kinds = []view.Kind{k.Kind}
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.ProgressSourceTimeout)
	defer cancel()
	items, err := src.Run(ctx, fanout.Query{Text: strings.TrimSpace(r.FormValue("q"))})
	if err != nil && !errors.Is(err, fanout.ErrIncomplete) {
		return nil
	}
	tf := s.timeFor(sess)
	var rows []view.FindRow
	for _, it := range items {
		if len(rows) == attachRowsMax {
			break
		}
		row := s.findRow(tf, blob, it)
		row.Href = s.route("/app/mail/attach?" + attachQuery(r, url.Values{"a": {account}, "c": {colPath}, "uid": {it.UID}}).Encode())
		rows = append(rows, row)
	}
	return rows
}

// attachFileRows are the entries of the folder open: folders open further;
// a file is chosen. For a folder link only folders are shown, and the one
// open is what is chosen.
func (s *Server) attachFileRows(r *http.Request, sess *session.Session, k attachKind, account, colPath string) []view.FindRow {
	fc, ok := s.fileCollection(sess, account, colPath)
	if !ok || fc.col.Feed {
		return nil
	}
	segs, ok := relSegs(r.FormValue("f"))
	if !ok {
		return nil
	}
	dir, err := fc.path(segs, true)
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.ProgressSourceTimeout)
	defer cancel()
	entries, err := fc.files.List(ctx, dir)
	if err != nil {
		return nil
	}
	var rows []view.FindRow
	if len(segs) > 0 {
		up := strings.Join(segs[:len(segs)-1], "/")
		rows = append(rows, view.FindRow{Kind: view.KindFile, Title: "..", Detail: "folder",
			Href: s.route("/app/mail/attach?" + attachQuery(r, url.Values{"a": {account}, "c": {colPath}, "f": {up}, "file": nil}).Encode())})
	}
	for _, e := range entries {
		if len(rows) == attachRowsMax {
			break
		}
		rel := strings.Join(append(append([]string{}, segs...), e.Name), "/")
		row := view.FindRow{Kind: view.KindFile, Title: text(e.Name), Colour: catalogColour(fc.col), Stamp: fc.stamp()}
		switch {
		case e.Dir:
			row.Detail = "folder"
			row.Href = s.route("/app/mail/attach?" + attachQuery(r, url.Values{"a": {account}, "c": {colPath}, "f": {rel}, "file": nil}).Encode())
		case k.What == "folder":
			continue
		default:
			row.Detail = text(string(formatSize(e.Size)))
			row.Href = s.route("/app/mail/attach?" + attachQuery(r, url.Values{"a": {account}, "c": {colPath}, "file": {e.Name}}).Encode())
		}
		rows = append(rows, row)
	}
	return rows
}

// chosenPart is the thing the request names, with its size read now: a
// record or a collection encoded in its exchange format, a file stat'ed.
func (s *Server) chosenPart(r *http.Request, sess *session.Session, k attachKind) (mailPart, error) {
	ctx := r.Context()
	account, colPath := r.FormValue("a"), r.FormValue("c")
	blob := s.catalogOf(sess)
	if account == "" {
		account, colPath = firstTicked(blob, k.Kind)
	}
	if conn, ok := blob.Connection(account); !ok || conn.Feed {
		return mailPart{}, errors.New("Choose a collection.")
	}
	p := mailPart{What: k.What, Kind: k.Kind, Account: account, Col: colPath}
	if k.Kind == view.KindFile {
		fc, ok := s.fileCollection(sess, account, colPath)
		if !ok || fc.col.Feed {
			return mailPart{}, errors.New("Choose a collection.")
		}
		segs, ok := relSegs(r.FormValue("f"))
		if !ok {
			return mailPart{}, errors.New("No such folder.")
		}
		if k.What == "folder" {
			p.Folder, p.AsLink, p.Path = true, true, strings.Join(segs, "/")
			p.Title = fileTitle(fc, segs)
			return p, nil
		}
		name := r.FormValue("file")
		if name == "" {
			return mailPart{}, errors.New("Choose a file.")
		}
		segs = append(segs, name)
		if _, ok := relSegs(strings.Join(segs, "/")); !ok {
			return mailPart{}, errors.New("No such file.")
		}
		fp, err := fc.path(segs, false)
		if err != nil {
			return mailPart{}, err
		}
		e, err := fc.files.Stat(ctx, fp)
		if err != nil || e.Dir {
			return mailPart{}, errors.New("The file is not there any more.")
		}
		p.Path, p.Title, p.Name, p.Size = strings.Join(segs, "/"), name, name, e.Size
		p.Media = e.Type
		if p.Media == "" {
			p.Media = mime.TypeByExtension(path.Ext(name))
		}
		return p, nil
	}

	c, ok := s.collectionOf(sess, recordKinds[k.Segment], account, colPath)
	if !ok || c.col.Gone || c.feed != nil {
		return mailPart{}, errors.New("Choose a collection.")
	}
	label := collectionLabel(c.col)
	switch k.What {
	case "book":
		p.Title, p.Name, p.Media = "Address book «"+label+"»", collectionSlug(label)+".vcf", "text/vcard; charset=utf-8"
	case "range":
		// Thirty days from today unless the address names the days.
		tf := s.timeFor(sess)
		from, ok := parseDay(tf, r.FormValue("from"))
		if !ok {
			from = time.Now().In(tf.tz)
		}
		to, ok := parseDay(tf, r.FormValue("to"))
		if !ok || to.Before(from) {
			to = from.AddDate(0, 0, 29)
		}
		p.From, p.To = from.Format("2006-01-02"), to.Format("2006-01-02")
		p.Title = "Calendar «" + label + "» · " + tf.date(from) + " – " + tf.date(to)
		p.Name, p.Media = collectionSlug(label)+"-"+p.From+".ics", cal.MediaType
	default:
		p.UID = r.FormValue("uid")
		if p.UID == "" {
			return mailPart{}, errors.New("Choose one.")
		}
		obj, err := c.find(ctx, p.UID)
		if err != nil {
			return mailPart{}, errors.New("It is not there any more.")
		}
		p.Title = recordTitle(k.Kind, obj)
		p.Format = r.FormValue("format")
		switch {
		case k.What == "contact":
			p.Name, p.Media = collectionSlug(p.Title)+".vcf", "text/vcard; charset=utf-8"
		case k.What == "note" && p.Format != "ics":
			p.Format = "md"
			p.Name, p.Media = collectionSlug(p.Title)+".md", "text/markdown; charset=utf-8"
		default:
			p.Name, p.Media = collectionSlug(p.Title)+".ics", cal.MediaType
		}
	}
	data, err := s.partContent(ctx, sess, p)
	if err != nil {
		return mailPart{}, errors.New("It could not be read: " + sentence(err.Error()))
	}
	p.Size = int64(len(data))
	return p, nil
}
