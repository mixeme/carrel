package web

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strings"

	"carrel.dev/carrel/server/internal/fanout"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/file"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// The attach sheet (S-26.2; s-mail-5, -6, -10, -12, -13): it stands in the
// letter's place, the letter waiting as a draft. On the left a tree — a
// server, its collections of the kind, «All …» above them where a record is
// searched for; a storage's folders for a file or a folder —, on the right
// what is found, and by Attach the line the letter will get — file or link —
// before it is pressed. Only the person's own collections: somebody else's
// subscription is read here, not handed on.

const attachRowsMax = 200

// attachNouns are what the sheet attaches, by what: its title says «Attach
// a note», Attach off says «Choose a note.».
var attachNouns = map[string]string{
	"contact": "contact", "book": "address book", "event": "event", "calendar": "calendar",
	"task": "task", "note": "note", "file": "file", "folder": "folder",
}

// attachAll is the first row of the tree of a kind whose records are
// searched for: every collection of it at once (С26).
var attachAll = map[string]string{
	"contact": "All address books", "event": "All calendars", "task": "All task lists", "note": "All notebooks",
}

// attachQuery is the sheet's address with one thing changed.
func attachQuery(r *http.Request, set url.Values) url.Values {
	q := url.Values{}
	for _, k := range []string{"what", "all", "a", "c", "dir", "q", "span", "from", "to", "format"} {
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
	if !ok || !s.canSendLetters(sess) {
		s.notFound(w, r)
		return
	}
	s.writeMailSheet(w, r, s.attachSheet(r, sess, k))
}

// postMailAttach adds the chosen thing to the letter and goes back to it;
// when it cannot, the sheet again with Attach off and why (С25).
func (s *Server) postMailAttach(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	k, ok := attachKindOf(r.FormValue("what"))
	if !ok || !s.canSendLetters(sess) {
		s.notFound(w, r)
		return
	}
	part, err := s.chosenPart(r, sess, k)
	if err != nil {
		s.writeMailSheet(w, r, s.attachSheet(r, sess, k))
		return
	}
	d := loadMailDraft(sess)
	d.Parts = s.reckonLinks(append(d.Parts, part))
	if p := s.sheetBack(r, r.FormValue("back"), false); p != "" {
		// Send of a record (С21): the letter stands over its page.
		d.Back = p
	}
	saveMailDraft(sess, d)
	s.redirect(w, r, "/app/mail/compose")
}

func (s *Server) attachSheet(r *http.Request, sess *session.Session, k attachKind) view.MailAttachSheet {
	blob := s.catalogOf(sess)
	account, colPath := r.FormValue("a"), r.FormValue("c")
	if account == "" && (k.Kind == view.KindFile || attachAll[k.What] == "") {
		// A calendar, a book, a storage: one is chosen, the first ticked
		// by default. A record is searched for in all of them.
		account, colPath = firstTicked(blob, k.Kind)
	}
	sheet := view.MailAttachSheet{What: k.What, Title: text("Attach " + article(attachNouns[k.What]))}
	switch {
	case k.Kind == view.KindFile:
		s.attachFiles(r, sess, k, account, colPath, &sheet)
	default:
		// A record chosen among all collections keeps «All …» on (all=1,
		// s-mail-12): its own collection is in the address to find it by.
		everywhere := account == "" || r.FormValue("all") == "1"
		on, onPath := account, colPath
		if everywhere {
			on, onPath = "", ""
		}
		sheet.Rail = s.attachRail(r, blob, k, on, onPath)
		if all := attachAll[k.What]; all != "" {
			sheet.All = &view.Action{ID: "mail.attach.all", Label: text(all), Method: "GET", Enabled: true, On: everywhere,
				Href: s.route("/app/mail/attach?" + attachQuery(r, url.Values{"all": nil, "a": nil, "c": nil, "uid": nil}).Encode())}
			// The search goes by a form of its own (Typecase's note 110 p. 1):
			// Enter in it asks again and presses no Attach.
			rows := []view.FormRow{{Name: "what", Kind: view.InputHidden, Value: text(k.What)}}
			if !everywhere {
				rows = append(rows, view.FormRow{Name: "a", Kind: view.InputHidden, Value: text(account)},
					view.FormRow{Name: "c", Kind: view.InputHidden, Value: text(colPath)})
			}
			rows = append(rows, view.FormRow{Name: "q", Label: "Search", Kind: view.InputText, Value: text(strings.TrimSpace(r.FormValue("q")))})
			sheet.Search = &view.Form{Action: s.route("/app/mail/attach"), Method: "GET", Sets: []view.FieldSet{{Rows: rows}}}
			sheet.Rows = s.attachRecordRows(r, sess, blob, k, on, onPath)
		}
	}
	cancel := view.Action{ID: "mail.attach.cancel", Label: "Cancel", Href: s.route("/app/mail/compose"), Method: "GET", Enabled: true}
	attach := view.Action{ID: "mail.attach.add", Label: "Attach", Method: "POST", Primary: true, Enabled: true, Submits: true}
	d := loadMailDraft(sess)
	sheet.Under = s.underOf(r, d.Back, "/app/contacts")

	part, err := s.chosenPart(r, sess, k)
	if err == nil {
		parts := s.reckonLinks(append(slices.Clone(d.Parts), part))
		part = parts[len(parts)-1]
		sheet.Preview = []view.MailAttachRow{attachRow(part)}
		sheet.Note = s.attachNote(part)
	}
	// The form of the sheet: what is chosen in its address, what to send it
	// as in its fields (s-mail-5, -10).
	ident := url.Values{"what": {k.What}, "a": {part.Account}, "c": {part.Col}}
	if err != nil {
		ident = attachQuery(r, url.Values{"q": nil, "span": nil, "from": nil, "to": nil, "format": nil})
	}
	switch {
	case part.UID != "":
		ident.Set("uid", part.UID)
	case k.What == "folder" && err == nil:
		ident.Set("dir", part.Path)
	case k.What == "file" && err == nil:
		ident.Set("path", part.Path)
	}
	var rows []view.FormRow
	switch k.What {
	case "calendar":
		rows = s.calendarSpan(sess, part)
	case "note":
		// Markdown is what a person opens and reads; the source .ics is the
		// faithful copy, for a client that reads VJOURNAL (owner's decision
		// 24.09.2026, s-mail-5).
		format := part.Format
		if format == "" {
			format = "md"
		}
		rows = append(rows, view.FormRow{Name: "format", Label: "Send it as", Kind: view.InputRadio, Value: text(format), Options: []view.Option{
			{Value: "md", Label: "Markdown · .md", Selected: format == "md"},
			{Value: "ics", Label: "Source · .ics", Selected: format == "ics"},
		}, HintRuns: join("Markdown is what a person opens and reads; it carries the text and links to attachments, not the foreign properties another client wrote. The source ",
			monoRun(".ics"), " is the faithful copy and needs a client that reads VJOURNAL — jtx Board, Evolution.")})
	}
	sheet.Form = s.plainForm("/app/mail/attach", "POST", append([]view.FormRow{csrfField(sess)}, rows...)...)
	sheet.Form.Action = s.postQuery("/app/mail/attach", ident)
	attach.Href = sheet.Form.Action
	if err != nil {
		// Attach stands always; nothing chosen, or what was could not be,
		// it is off and says why (С25; s-mail-10).
		attach.Enabled, attach.Reason = false, text(err.Error())
	}
	sheet.Actions = []view.Action{attach, cancel}
	return sheet
}

// calendarSpan are the fields of «Calendar» (s-mail-10): the whole of it or
// its days — Whole calendar turns the days off while it is chosen —, the
// days there always, from today to the end of the month unless chosen.
func (s *Server) calendarSpan(sess *session.Session, p mailPart) []view.FormRow {
	tf := s.timeFor(sess)
	today := tf.now.In(tf.tz)
	from, to := p.From, p.To
	if from == "" {
		from = today.Format("2006-01-02")
		to = today.AddDate(0, 1, -today.Day()).Format("2006-01-02")
	}
	span := spanOf(p)
	if span == "" {
		span = "whole"
	}
	whole := span == "whole"
	return []view.FormRow{
		{Name: "span", Label: "What goes in", Kind: view.InputRadio, Value: text(span), Options: []view.Option{
			{Value: "whole", Label: "Whole calendar", Selected: whole, Disables: "from to"},
			{Value: "dates", Label: "Dates", Selected: !whole},
		}},
		{Name: "from", Label: "From", Kind: view.InputDate, Mono: true, Value: text(from), ReadOnly: whole},
		{Name: "to", Label: "To", Kind: view.InputDate, Mono: true, Value: text(to), ReadOnly: whole, Hint: "Both days are in the file."},
	}
}

// attachNote is the message over Attach when what is chosen goes as a link
// (s-mail-6, -13): a folder always; anything over the ceiling of an
// attachment, or of the letter. Nil when it goes as a file.
func (s *Server) attachNote(p mailPart) view.Runs {
	tail := ". The link is listed in "
	end := ", works for 30 days, and revoking it there closes it for whoever got the letter."
	switch {
	case !p.AsLink:
		return nil
	case p.Folder:
		return join("A folder always goes as a link: the letter carries a secret link to it, with all that is in it"+tail, strongRun("Published links"), end)
	case p.Kind == view.KindFile:
		return join("This file is over the limit for an attachment, so the letter will carry a secret link to it instead"+tail, strongRun("Published links"), end)
	}
	return join("This is over the limit for an attachment, so the letter will carry a secret link to it instead"+tail, strongRun("Published links"), end)
}

func attachRow(p mailPart) view.MailAttachRow {
	title := p.Title
	if p.Detail != "" {
		title += " · " + p.Detail
	}
	row := view.MailAttachRow{Kind: p.Kind, Title: text(title), AsLink: p.AsLink}
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

// attachRail is the tree of collections (С26): a server, its collections of
// the kind under it, each a link to the sheet with it chosen — the search
// kept —, the chosen one on.
func (s *Server) attachRail(r *http.Request, blob sealed.Blob, k attachKind, account, colPath string) view.Rail {
	rail := view.Rail{About: s.aboutAction()}
	for _, conn := range blob.Connections {
		if conn.Feed {
			continue
		}
		sec := view.RailSection{Account: text(connLabel(conn)), AccountID: conn.ID, State: view.CollectionOK}
		for _, col := range blob.CollectionsOf(conn.ID) {
			if !col.Has(k.Kind) || col.Gone || col.Feed {
				continue
			}
			row := view.SourceRow{Label: text(collectionLabel(col)), Colour: catalogColour(col), Tick: view.TickOff, State: view.CollectionOK,
				Href: s.route("/app/mail/attach?" + attachQuery(r, url.Values{"all": nil, "a": {conn.ID}, "c": {col.Path}, "uid": nil}).Encode())}
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

// attachRecordRows are the records found (С26): in the chosen collection,
// or with «All …» in every one of the kind that answers — the same fanout
// as a search —, sifted by q; the one chosen on.
func (s *Server) attachRecordRows(r *http.Request, sess *session.Session, blob sealed.Blob, k attachKind, account, colPath string) []view.FindRow {
	var sources []fanout.Source
	for _, conn := range blob.Connections {
		if conn.Feed || (account != "" && conn.ID != account) {
			continue
		}
		for _, col := range blob.CollectionsOf(conn.ID) {
			if !col.Has(k.Kind) || col.Gone || col.Feed || (account != "" && col.Path != colPath) {
				continue
			}
			src := s.openSource(sess, conn.ID+":"+col.Path, conn, col)
			src.kinds = []view.Kind{k.Kind}
			sources = append(sources, src)
		}
	}
	items := s.fanoutItems(r.Context(), sess, sources, strings.TrimSpace(r.FormValue("q")))
	// The collections answer in their own time: the rows go in the
	// catalog's order of them, and by title within one.
	order := map[string]int{}
	for i, col := range blob.Collections {
		order[col.AccountID+"\x00"+col.Path] = i
	}
	tf := s.timeFor(sess)
	uid := r.FormValue("uid")
	var rows []view.FindRow
	var at []int
	for _, it := range items {
		if len(rows) == attachRowsMax {
			break
		}
		row := s.findRow(tf, blob, it)
		set := url.Values{"a": {it.AccountID}, "c": {it.Col}, "uid": {it.UID}}
		if account == "" {
			set.Set("all", "1")
		}
		row.Href = s.route("/app/mail/attach?" + attachQuery(r, set).Encode())
		row.On = it.UID == uid && it.AccountID == r.FormValue("a") && it.Col == r.FormValue("c")
		rows = append(rows, row)
		at = append(at, order[it.AccountID+"\x00"+it.Col])
	}
	idx := make([]int, len(rows))
	for i := range idx {
		idx[i] = i
	}
	slices.SortStableFunc(idx, func(i, j int) int {
		if at[i] != at[j] {
			return at[i] - at[j]
		}
		return strings.Compare(strings.ToLower(string(rows[i].Title)), strings.ToLower(string(rows[j].Title)))
	})
	sorted := make([]view.FindRow, len(rows))
	for i, k := range idx {
		sorted[i] = rows[k]
	}
	return sorted
}

// fanoutItems asks the sources for q, as a search does, and gives what they
// found once they have all answered or the request is gone.
func (s *Server) fanoutItems(ctx context.Context, sess *session.Session, sources []fanout.Source, q string) []fanout.Item {
	if len(sources) == 0 {
		return nil
	}
	opts := s.searchOptions(sess)
	opts.Finished = nil // a pick is not a search of the history
	id, err := user.NewTaskID()
	if err != nil {
		return nil
	}
	task, err := fanout.Start(id, fanout.Query{Text: q}, sources, opts)
	if err != nil {
		return nil
	}
	for task.Running() {
		changed := task.Changed()
		if !task.Running() {
			break
		}
		select {
		case <-changed:
		case <-ctx.Done():
			task.Cancel()
		}
	}
	return task.Snapshot().Items
}

// attachFiles fills the sheet of a file or a folder (С28; s-mail-6, -13):
// the tree of the storages with the open folder (dir) opened, the crumbs to
// it, and what is in it — a file a row with its size, when it changed and
// whether it goes as a file or a link, the chosen one (path) on; for a
// folder its folders, each choosing itself, and its files off.
func (s *Server) attachFiles(r *http.Request, sess *session.Session, k attachKind, account, colPath string, sheet *view.MailAttachSheet) {
	fc, ok := s.fileCollection(sess, account, colPath)
	if !ok || fc.col.Feed {
		return
	}
	dir, okDir := relSegs(r.FormValue("dir"))
	if !okDir {
		dir = nil
	}
	fc.segs = dir
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.ProgressSourceTimeout)
	defer cancel()
	sheet.Tree = s.attachTree(ctx, fc.blob, fc, k)
	at := func(segs []string) string {
		return s.route("/app/mail/attach?" + url.Values{"what": {k.What}, "a": {account}, "c": {colPath}, "dir": {strings.Join(segs, "/")}}.Encode())
	}
	sheet.Crumbs = []view.Crumb{{Label: text(collectionLabel(fc.col)), Href: at(nil)}}
	for i, seg := range dir {
		sheet.Crumbs = append(sheet.Crumbs, view.Crumb{Label: text(seg), Href: at(dir[:i+1])})
	}
	folder, err := fc.path(dir, true)
	if err != nil {
		return
	}
	entries, err := fc.files.List(ctx, folder)
	if err != nil {
		return
	}
	slices.SortFunc(entries, func(a, b file.Entry) int {
		if a.Dir != b.Dir {
			if a.Dir {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	d := loadMailDraft(sess)
	tf := s.timeFor(sess)
	chosen := r.FormValue("path")
	folders, files := 0, 0
	for _, e := range entries {
		if len(sheet.Rows) == attachRowsMax {
			break
		}
		rel := strings.Join(append(slices.Clone(dir), e.Name), "/")
		row := view.FindRow{Kind: view.KindFile, Title: text(e.Name), Colour: catalogColour(fc.col), Stamp: fc.stamp()}
		if !e.Modified.IsZero() {
			row.Changed = text(tf.shortDay(e.Modified))
		}
		switch {
		case e.Dir:
			folders++
			row.Detail, row.Icon = "folder", "i-file"
			row.Href = at(append(slices.Clone(dir), e.Name))
		case k.What == "folder":
			// A folder is chosen, not a file in it (s-mail-13).
			files++
			row.Size, row.Off = formatSize(e.Size), true
		default:
			files++
			row.Size = formatSize(e.Size)
			row.Href = s.route("/app/mail/attach?" + url.Values{"what": {k.What}, "a": {account}, "c": {colPath}, "dir": {strings.Join(dir, "/")}, "path": {rel}}.Encode())
			row.On = rel == chosen
			// Before the choice, whether it would go as a file or a link
			// (s-mail-6): the ceiling named where it is passed.
			parts := s.reckonLinks(append(slices.Clone(d.Parts), mailPart{Kind: view.KindFile, Size: e.Size}))
			switch {
			case e.Size > s.mailPolicy().Attach:
				row.Goes = text("over " + string(formatSize(s.mailPolicy().Attach)) + " · as a link")
			case parts[len(parts)-1].AsLink:
				row.Goes = text("over the letter's " + string(formatSize(s.mailPolicy().Letter)) + " · as a link")
			default:
				row.Goes = "as a file"
			}
		}
		sheet.Rows = append(sheet.Rows, row)
	}
	if k.What == "folder" {
		sheet.In = text("In it · " + folderCount(folders, files))
	}
}

// folderCount is what a folder holds in words: its folders and its files
// apart, none not said — «1 folder · 6 files» (С20).
func folderCount(folders, files int) string {
	var said []string
	if folders > 0 {
		said = append(said, plural(folders, "folder"))
	}
	if files > 0 || folders == 0 {
		said = append(said, plural(files, "file"))
	}
	return strings.Join(said, " · ")
}

// attachTree is the tree of the sheet of a file or a folder: the server of
// the storage open, its storages under it, the open folder's way opened; each
// storage and folder a link to the sheet with it open (s-mail-6, -13 — the
// tree of Move, not a third one).
func (s *Server) attachTree(ctx context.Context, blob sealed.Blob, open *fileCtx, k attachKind) *view.FileTree {
	out := &view.FileTree{}
	seen := map[string]bool{}
	for _, col := range blob.Collections {
		if !treeCollection(blob, col, true) || col.Feed || seen[col.AccountID] || col.AccountID != open.conn.ID {
			continue
		}
		seen[col.AccountID] = true
		conn, _ := blob.Connection(col.AccountID)
		server := view.FileTreeNode{ID: conn.ID, Label: connLabel(conn), Expanded: true}
		chosen := (*fileCtx)(nil)
		if open.conn.ID == conn.ID {
			chosen = open
		}
		t := s.tree(ctx, blob, chosen, "", false, 0, conn.ID)
		// Each node chooses by a link: the storage's root and the folders
		// under it, by their way from it.
		for i := range t.Nodes {
			c, _ := blob.Collection(conn.ID, collectionOfNode(blob, conn.ID, t.Nodes[i].Label))
			s.linkAttachNodes(&t.Nodes[i], k, conn.ID, c.Path, nil)
		}
		server.Children = t.Nodes
		out.Nodes = append(out.Nodes, server)
	}
	out.Chosen = s.route(open.href("", open.segs, true))
	return out
}

// collectionOfNode is the path of the storage of a server a root of the
// tree is labelled with.
func collectionOfNode(blob sealed.Blob, account, label string) string {
	for _, col := range blob.CollectionsOf(account) {
		if col.Has(view.KindFile) && collectionLabel(col) == label {
			return col.Path
		}
	}
	return ""
}

// linkAttachNodes gives a node of the tree and those under it the address of
// the sheet with it open (dir — its way from the storage's root).
func (s *Server) linkAttachNodes(n *view.FileTreeNode, k attachKind, account, colPath string, segs []string) {
	n.Href = s.route("/app/mail/attach?" + url.Values{"what": {k.What}, "a": {account}, "c": {colPath}, "dir": {strings.Join(segs, "/")}}.Encode())
	for i := range n.Children {
		s.linkAttachNodes(&n.Children[i], k, account, colPath, append(slices.Clone(segs), n.Children[i].Label))
	}
}

// chosenPart is the thing the request names, with its size read now: a
// record or a collection encoded in its exchange format, a file stat'ed. A
// file is path, a folder dir, under the collection's root.
func (s *Server) chosenPart(r *http.Request, sess *session.Session, k attachKind) (mailPart, error) {
	ctx := r.Context()
	account, colPath := r.FormValue("a"), r.FormValue("c")
	blob := s.catalogOf(sess)
	if account == "" && (k.Kind == view.KindFile || attachAll[k.What] == "") {
		account, colPath = firstTicked(blob, k.Kind)
	}
	choose := errors.New("Choose " + article(attachNouns[k.What]) + ".")
	if conn, ok := blob.Connection(account); !ok || conn.Feed {
		return mailPart{}, choose
	}
	p := mailPart{What: k.What, Kind: k.Kind, Account: account, Col: colPath}
	if k.Kind == view.KindFile {
		fc, ok := s.fileCollection(sess, account, colPath)
		if !ok || fc.col.Feed {
			return mailPart{}, choose
		}
		if k.What == "folder" {
			segs, ok := relSegs(r.FormValue("dir"))
			if !ok {
				return mailPart{}, errors.New("No such folder.")
			}
			p.Folder, p.AsLink, p.Path = true, true, strings.Join(segs, "/")
			p.Title = fileTitle(fc, segs)
			// How much is in it, after its name: its folders and its files
			// apart, none not said (Пб21, С20; s-mail-1: «Договор подряда ·
			// 1 folder · 6 files»); unread, its name alone.
			if dir, err := fc.path(segs, true); err == nil {
				if entries, err := fc.files.List(ctx, dir); err == nil {
					folders, files := 0, 0
					for _, e := range entries {
						if e.Dir {
							folders++
						} else {
							files++
						}
					}
					p.Detail = folderCount(folders, files)
				}
			}
			return p, nil
		}
		rel := r.FormValue("path")
		if rel == "" {
			return mailPart{}, choose
		}
		segs, ok := relSegs(rel)
		if !ok || len(segs) == 0 {
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
		name := segs[len(segs)-1]
		p.Path, p.Title, p.Name, p.Size = strings.Join(segs, "/"), name, name, e.Size
		p.Media = e.Type
		if p.Media == "" {
			p.Media = mime.TypeByExtension(path.Ext(name))
		}
		return p, nil
	}

	c, ok := s.collectionOf(sess, recordKinds[k.Segment], account, colPath)
	if !ok || c.col.Gone || c.feed != nil {
		return mailPart{}, choose
	}
	label := collectionLabel(c.col)
	switch k.What {
	case "book":
		p.Title, p.Name, p.Media = "Address book «"+label+"»", collectionSlug(label)+".vcf", "text/vcard; charset=utf-8"
	case "calendar":
		if r.FormValue("span") != "dates" {
			// The whole calendar is what the sheet opens on (s-mail-10).
			p.Title = "Calendar «" + label + "» · all events"
			p.Name, p.Media = collectionSlug(label)+".ics", cal.MediaType
			break
		}
		// From today to the end of the month unless the form names the
		// days (s-mail-10).
		tf := s.timeFor(sess)
		today := tf.now.In(tf.tz)
		from, ok := parseDay(tf, r.FormValue("from"))
		if !ok {
			from = today
		}
		to, ok := parseDay(tf, r.FormValue("to"))
		if !ok || to.Before(from) {
			to = from.AddDate(0, 1, -from.Day())
		}
		p.From, p.To = from.Format("2006-01-02"), to.Format("2006-01-02")
		p.Title = "Calendar «" + label + "» · " + tf.date(from) + " – " + tf.date(to)
		p.Name, p.Media = collectionSlug(label)+"-"+p.From+".ics", cal.MediaType
	default:
		p.UID = r.FormValue("uid")
		if p.UID == "" {
			return mailPart{}, choose
		}
		obj, err := c.find(ctx, p.UID)
		if err != nil {
			return mailPart{}, errors.New("It is not there any more.")
		}
		p.Title = recordTitle(k.Kind, obj)
		// The row says which one (Пб21; s-mail-1): an event its day and
		// hour, «Встреча на объекте · 18 August 10:00»; a contact where they
		// work, «Пётр Смирнов · подрядчик».
		p.Detail = partDetail(s.timeFor(sess), k.Kind, obj)
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

// partDetail is what tells one record attached from another after its
// title: an event's day and hour, a contact's organisation or title.
func partDetail(tf timeFmt, k view.Kind, obj *model.Object) string {
	switch k {
	case view.KindEvent:
		tm, err := cal.TimingOf(obj, tf.tz)
		if err != nil || tm.Start.IsZero() {
			return ""
		}
		if tm.AllDay {
			return tm.Start.In(tf.tz).Format("2 January")
		}
		return tm.Start.In(tf.tz).Format("2 January ") + tf.clock(tm.Start)
	case view.KindContact:
		if org := structured(firstValue(obj, "ORG")); org != "" {
			return org
		}
		return strings.TrimSpace(firstValue(obj, "TITLE"))
	}
	return ""
}

// spanOf is what of a calendar a part carries: "dates" when it names its
// days, "whole" otherwise; "" for any other part.
func spanOf(p mailPart) string {
	switch {
	case p.What != "calendar":
		return ""
	case p.From != "":
		return "dates"
	}
	return "whole"
}
