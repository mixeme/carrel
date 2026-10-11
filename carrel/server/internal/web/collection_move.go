package web

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// S-12.3: a calendar or an address book moves to another connection. It is
// made there with the same name, colour and component set, every record goes
// as a single one does (S-18), and the original is deleted only when every
// record arrived whole. A copy stops before the delete. The sheet and its POST
// share /app/collections/move: a and c name the collection, to the connection
// it goes to, copy=1 the copy.

var propGetETag = xml.Name{Space: "DAV:", Local: "getetag"}

func collectionNoun(col sealed.Collection) string {
	switch {
	case col.Has(view.KindContact):
		return "address book"
	case col.Has(view.KindFile):
		return "folder"
	}
	return "calendar"
}

// collectionMoveQuery is the address of the move sheet: the collection, the
// destination chosen, copy, and back — the screen it was opened from, kept
// through every choice so that its Cancel still leads there.
func collectionMoveQuery(conn sealed.Connection, col sealed.Collection, to string, copying bool, back string) url.Values {
	q := url.Values{"a": {conn.ID}, "c": {col.Path}}
	if to != "" {
		q.Set("to", to)
	}
	if copying {
		q.Set("copy", "1")
	}
	if back != "" {
		q.Set("back", back)
	}
	return q
}

// collectionMoveTargets are every connection, the current one and those that
// cannot hold the kind shown and closed with the reason; eligible counts the
// others.
func (s *Server) collectionMoveTargets(blob sealed.Blob, conn sealed.Connection, col sealed.Collection, copying bool, back string) (opts []view.TargetOption, eligible int) {
	for _, c := range blob.Connections {
		// Beside a server the sheet says what it takes, or why not
		// (s-profile-8): «calendars», «already there», «files only».
		opt := view.TargetOption{
			ID:    s.postQuery("/app/collections/move", collectionMoveQuery(conn, col, c.ID, copying, back)),
			Label: text(connLabel(c)),
		}
		switch {
		case c.ID == conn.ID:
			opt.ReadOnly, opt.Reason = true, "already there"
		case col.Has(view.KindFile) && collectionHome(blob, c, col) == "":
			opt.ReadOnly, opt.Reason = true, "no file storage"
		case collectionHome(blob, c, col) == "" && c.FileRoot != "":
			opt.ReadOnly, opt.Reason = true, "files only"
		case collectionHome(blob, c, col) == "":
			opt.ReadOnly, opt.Reason = true, text("no "+collectionNoun(col)+"s")
		default:
			opt.Stamp = view.Stamp(collectionNoun(col) + "s")
			opt.Href = opt.ID // the sheet with it chosen (U-8)
			eligible++
		}
		opts = append(opts, opt)
	}
	return opts, eligible
}

// collectionMoveItems are the menu items of a collection row — Move and
// Copy to another server (s-profile-1, s-profile-12) — or none: with no other
// connection that holds the kind there is nowhere to go (S-12.3). A folder
// goes to another file storage as a tree (S-12.1).
func (s *Server) collectionMoveItems(blob sealed.Blob, conn sealed.Connection, col sealed.Collection, back string) []view.MenuItem {
	if col.Gone || col.Feed || len(col.Kinds) == 0 {
		return nil
	}
	if _, eligible := s.collectionMoveTargets(blob, conn, col, false, ""); eligible == 0 {
		return nil
	}
	copying := view.MenuItem{Action: view.Action{
		ID: "collection.copy", Label: "Copy to another server", Icon: "i-dup",
		Href:   s.postQuery("/app/collections/move", collectionMoveQuery(conn, col, "", true, back)),
		Method: "GET", Enabled: true,
	}}
	if col.Has(view.KindFile) && storageRoot(conn, col) {
		// The whole storage is not deleted from here, so it is not moved
		// either: it goes elsewhere only as a copy (s-profile-17).
		return []view.MenuItem{copying}
	}
	return []view.MenuItem{
		{Action: view.Action{
			ID: "collection.move", Label: "Move to another server", Icon: "i-move",
			Href:   s.postQuery("/app/collections/move", collectionMoveQuery(conn, col, "", false, back)),
			Method: "GET", Enabled: true,
		}},
		copying,
	}
}

// collectionMoveBlock is why the move or copy to dest cannot start, or "".
func collectionMoveBlock(blob sealed.Blob, conn sealed.Connection, col sealed.Collection, dest sealed.Connection, known, copying bool) string {
	noun := collectionNoun(col)
	switch {
	case !known:
		return "Choose a server."
	case dest.ID == conn.ID:
		return "The " + noun + " is already on this server."
	case col.Has(view.KindFile) && collectionHome(blob, dest, col) == "":
		return "That server is no file storage."
	case collectionHome(blob, dest, col) == "":
		return "That server holds no " + noun + "s."
	case !copying && col.ReadOnly:
		return "This " + noun + " is read-only here, so it could not be deleted after the move. It can be copied."
	case !copying && col.Has(view.KindFile) && storageRoot(conn, col):
		return storageRootReason + " It can be copied, not moved."
	}
	return ""
}

// collectionMoveSheet says what will happen before it starts (S-12.3): where
// it goes, that the address changes, and when the original is deleted.
// count is the records the collection holds as the session last learnt it,
// nil when it does not know.
func (s *Server) collectionMoveSheet(blob sealed.Blob, conn sealed.Connection, col sealed.Collection, copying bool, to string, count *int, back string) view.CollectionMoveSheet {
	noun, label, here := collectionNoun(col), collectionLabel(col), connLabel(conn)
	verb := "Move "
	if copying {
		verb = "Copy "
	}
	sheet := view.CollectionMoveSheet{Kind: col.Kinds[0], Copy: copying, Title: text(verb + noun), Subject: text(here + " · " + label)}
	sheet.Target.Options, _ = s.collectionMoveTargets(blob, conn, col, copying, back)
	dest, known := blob.Connection(to)
	confirm := view.Action{ID: "collection.move.confirm", Label: "Move", Method: "POST", Primary: true, Enabled: true}
	if copying {
		confirm.ID, confirm.Label = "collection.copy.confirm", "Copy"
	}
	if reason := collectionMoveBlock(blob, conn, col, dest, known, copying); reason != "" {
		confirm.Enabled, confirm.Reason = false, text(reason)
	} else {
		chosen := s.postQuery("/app/collections/move", collectionMoveQuery(conn, col, dest.ID, copying, back))
		sheet.Target.Chosen, confirm.Href = chosen, chosen
		// The words of s-profile-8: what is made, how much goes, and what
		// becomes of this one.
		// What the collection is for names what it holds: a calendar that
		// takes tasks too still copies «events» (its count is of events).
		unit := recordNoun(col)
		if spec, ok := recordKinds[segmentOf(col.Kinds[0])]; ok {
			unit = spec.noun
		}
		records := []view.Run{plainRun("all its " + unit + "s")}
		if count != nil && *count > 0 {
			records = allOf(*count, unit)
		}
		switch {
		case col.Has(view.KindFile) && copying:
			// S-17.1: between storages every file travels through Carrel.
			sheet.Warning = line(fmt.Sprintf("Carrel creates a folder there and copies everything in it, each file downloaded and uploaded again; this one on %s stays as it is.", here))
		case col.Has(view.KindFile):
			sheet.Warning = line(fmt.Sprintf("Carrel creates a folder there, copies everything in it — each file downloaded and uploaded again — then deletes this one on %s. "+
				"Devices, published links and backup jobs pointing at the old address will see it vanish — same as Delete, except the files live on.", here))
		case copying:
			sheet.Warning = join("Carrel creates a "+noun+" there and copies ", records, "; this one on "+here+" stays as it is.")
		default:
			sheet.Warning = join("Carrel creates a "+noun+" there, copies ", records, ", then deletes this one on "+here+". "+
				"Devices, published links and backup jobs pointing at the old address will see it vanish — same as Delete, except the records live on. This is not renaming the path.")
		}
	}
	sheet.Actions = []view.Action{confirm, s.sheetCancel(back, collectionList(conn, col))}
	return sheet
}

func (s *Server) getCollectionMove(w http.ResponseWriter, r *http.Request) {
	sess, _, conn, col, ok := s.openCollectionSheet(w, r)
	if !ok {
		return
	}
	blob := s.catalogOf(sess)
	if s.collectionMoveItems(blob, conn, col, "") == nil {
		s.notFound(w, r)
		return
	}
	q := r.URL.Query()
	var count *int
	if spec, ok := recordKinds[segmentOf(col.Kinds[0])]; ok {
		count = collectionCount(sess, conn.ID, col, spec)
	}
	s.writePage(w, r, http.StatusOK, s.collectionMoveSheet(blob, conn, col, q.Get("copy") != "", q.Get("to"), count, sheetBack(q.Get("back"))))
}

// postCollectionMove runs a confirmed move or copy. What happens is on the
// connections page afterwards: the new collection opens when all went well;
// otherwise a notice says what stayed where.
func (s *Server) postCollectionMove(w http.ResponseWriter, r *http.Request) {
	sess, key, conn, col, ok := s.openCollectionSheet(w, r)
	if !ok {
		return
	}
	blob := s.catalogOf(sess)
	if s.collectionMoveItems(blob, conn, col, "") == nil {
		s.notFound(w, r)
		return
	}
	copying, to := r.FormValue("copy") != "", r.FormValue("to")
	dest, known := blob.Connection(to)
	if collectionMoveBlock(blob, conn, col, dest, known, copying) != "" {
		var count *int
		if spec, ok := recordKinds[segmentOf(col.Kinds[0])]; ok {
			count = collectionCount(sess, conn.ID, col, spec)
		}
		s.writePage(w, r, http.StatusOK, s.collectionMoveSheet(blob, conn, col, copying, to, count, sheetBack(r.URL.Query().Get("back"))))
		return
	}
	m := &collectionMove{s: s, sess: sess, key: key, conn: conn, col: col, dest: dest, copying: copying, home: collectionHome(blob, dest, col)}
	var done string
	var notice *view.Notice
	if col.Has(view.KindFile) {
		done, notice = m.runFiles(r.Context())
	} else {
		done, notice = m.run(r.Context())
	}
	if notice == nil {
		s.redirect(w, r, done)
		return
	}
	s.writePage(w, r, http.StatusOK, s.connectionsPage(sess, connectionForm{}, notice))
}

// collectionMove is one run of S-12.3.
type collectionMove struct {
	s       *Server
	sess    *session.Session
	key     [32]byte
	conn    sealed.Connection
	col     sealed.Collection
	dest    sealed.Connection
	copying bool
	home    string // where the new collection is made on dest

	src, dst *collectionCtx
	placed   sealed.Collection // the new collection as the catalog will hold it
}

// transferred is one resource of the original and what became of it.
type transferred struct {
	name string        // file name at the new address
	sent *model.Object // a record; nil for a resource copied as bytes
}

func (m *collectionMove) title() string {
	if m.copying {
		return "Nothing was copied"
	}
	return "Nothing was moved"
}

func (m *collectionMove) newAddress() string {
	if m.placed.Has(view.KindFile) {
		return fileHref(m.dest.ID, m.placed.Path, "", nil, true)
	}
	return "/app/" + segmentOf(m.placed.Kinds[0]) + "/" + m.dest.ID + "/" + url.PathEscape(m.placed.Path)
}

func (m *collectionMove) run(ctx context.Context) (string, *view.Notice) {
	noun, label := collectionNoun(m.col), collectionLabel(m.col)
	spec := recordKinds[segmentOf(m.col.Kinds[0])]
	var err error
	if m.src, err = m.s.newCollectionCtx(m.sess, spec, m.conn, m.col); err != nil {
		return "", davRefusal(moveCollectionRefusal(m.title(), label), m.conn, "PROPFIND", m.col.Path, err)
	}

	// What the original holds, by name and version, and its records read whole.
	before, err := members(ctx, m.src.t, m.col.Path)
	if err != nil {
		return "", davRefusal(moveCollectionRefusal(m.title(), label), m.conn, "PROPFIND", m.col.Path, err)
	}
	// Every record, a stray task in a calendar of events too.
	records, err := m.src.allRecords(ctx)
	if err != nil {
		return "", davRefusal(moveCollectionRefusal(m.title(), label), m.conn, "REPORT", m.col.Path, err)
	}

	// Name and colour as the server holds them now: another client may have
	// changed them since discovery. The catalog is what is left when it says
	// nothing.
	colour := m.col.Colour
	if name, c, err := liveLabel(ctx, m.src.t, m.col.Path); err == nil {
		if name != "" {
			label = name
		}
		if c != "" {
			colour = c
		}
	}

	// The new collection: the same last segment when it is free there.
	home := m.home
	client, err := dav.NewClient(m.s.guard, m.dest.Server, m.dest.Login, m.dest.Password, m.dest.CertFingerprint)
	var taken map[string]bool
	if err == nil {
		taken, err = childSegments(ctx, client, home)
	}
	if err != nil {
		return "", davRefusal(moveCollectionRefusal(m.title(), label), m.dest, "PROPFIND", home, err)
	}
	segment := path.Base(strings.TrimSuffix(m.col.Path, "/"))
	if addressProblem(segment) != "" || segment == "" {
		segment = collectionSlug(label)
	}
	segment = uniqueSlug(segment, taken)
	f := collectionForm{kind: m.col.Kinds[0], name: label, colour: colour, components: m.col.Components}
	if f.kind != view.KindContact && len(f.components) == 0 {
		for _, k := range m.col.Kinds {
			if c := componentOf(k); c != "" {
				f.components = append(f.components, c)
			}
		}
	}
	newPath := strings.TrimSuffix(home, "/") + "/" + segment + "/"
	method, props, kinds := createRequest(f)
	if err := client.MkColProps(ctx, method, newPath, props); err != nil {
		return "", davRefusal(moveCollectionRefusal(m.title(), label), m.dest, method, newPath, err)
	}
	// Where it actually landed, before anything is copied into it: a server
	// that picks its own address (SV-19) would otherwise take every record at
	// an address that holds nothing.
	settled, err := settlePath(ctx, client, home, newPath, taken)
	if err != nil {
		return "", lostCollection(m.dest, method, newPath, err,
			label+" here is as it was: run re-discovery on that connection to see what the server made.")
	}
	newPath = settled
	m.placed = sealed.Collection{Path: newPath, Name: label, Kinds: kinds, Colour: colour, Components: f.components}
	if m.dst, err = m.s.newCollectionCtx(m.sess, spec, m.dest, m.placed); err != nil {
		m.takeBack(ctx)
		return "", davRefusal(moveCollectionRefusal(m.title(), label), m.dest, method, newPath, err)
	}

	// Every resource of the original, one at a time. A refusal takes the new
	// collection back: a half copy is not left behind (S-12.2).
	sent, refused := m.copyAll(ctx, before, records)
	if len(refused) > 0 {
		m.takeBack(ctx)
		n := &view.Notice{
			Role:  view.NoticeAlert,
			Title: text(m.title()),
			Body: text(fmt.Sprintf("%s refused %s of %d, so the new %s there was taken back. %s is as it was.",
				connLabel(m.dest), plural(len(refused), "record"), len(before), noun, label)),
			Details: &view.Diagnostic{Summary: text("PUT " + newPath), Lines: refused},
		}
		return "", n
	}

	// Read back and compared, as a single write is.
	lost, err := m.readBack(ctx, sent)
	if err != nil {
		lost = append(lost, text("The new "+noun+" could not be read back: "+err.Error()))
	}
	if m.copying {
		if err := m.place(false); err != nil {
			m.takeBack(ctx)
			return "", m.catalogFailure()
		}
		if len(lost) > 0 {
			return "", m.kept("Copied, but the server did not keep everything",
				fmt.Sprintf("%s did not keep all of what was copied; %s here has it all.", connLabel(m.dest), label), lost)
		}
		return m.newAddress(), nil
	}
	if len(lost) > 0 {
		if err := m.place(false); err != nil {
			m.takeBack(ctx)
			return "", m.catalogFailure()
		}
		return "", m.kept("Copied, and the original kept",
			fmt.Sprintf("%s did not keep all of what was copied, so %s here was not deleted. Both are there now.", connLabel(m.dest), label), lost)
	}

	// The original is deleted only as it was read: a record added or changed
	// by another client meanwhile keeps it.
	after, err := members(ctx, m.src.t, m.col.Path)
	if err != nil || !sameMembers(before, after) {
		if err := m.place(false); err != nil {
			m.takeBack(ctx)
			return "", m.catalogFailure()
		}
		return "", m.kept("Copied, but the original changed, so it stays",
			fmt.Sprintf("%s changed on %s while it was copied. The copy holds it as it was read; both are there now.", label, connLabel(m.conn)), nil)
	}
	// The catalog holds both before anything is deleted: whatever happens next,
	// it does not point at nothing.
	if err := m.place(false); err != nil {
		m.takeBack(ctx)
		return "", m.catalogFailure()
	}
	if err := m.src.t.Delete(ctx, m.col.Path, ""); err != nil {
		n := m.kept("Copied, but the original could not be deleted",
			fmt.Sprintf("Both are there now. Delete %s on %s yourself once the server lets you.", label, connLabel(m.conn)), nil)
		n.Details = davRefusal(moveCollectionRefusal("", label), m.conn, "DELETE", m.col.Path, err).Details
		return "", n
	}
	if err := m.place(true); err != nil {
		return "", m.kept("Moved, but the list of collections still names the original",
			"The "+noun+" was moved and the original deleted; run re-discovery on its connection to take it out.", nil)
	}
	return m.newAddress(), nil
}

// copyAll writes each resource of the original into the new collection. A
// record goes as S-18 sends it; what is not a readable record — a broken one,
// another component — goes as its bytes. A name taken there becomes {uid}-2.
func (m *collectionMove) copyAll(ctx context.Context, before map[string]string, records []*model.Object) ([]transferred, []view.Text) {
	var out []transferred
	var refused []view.Text
	byName := map[string]*model.Object{}
	for _, obj := range records {
		byName[memberName(obj.Path)] = obj
	}
	for name := range before {
		obj := byName[name]
		dst := m.placed.Path + name
		if obj == nil {
			if err := m.copyBytes(ctx, m.col.Path+name, dst); err != nil {
				refused = append(refused, text(name+" · "+err.Error()))
				continue
			}
			out = append(out, transferred{name: name})
			continue
		}
		doc, err := m.src.body(obj)
		var sent *model.Object
		if err == nil {
			sent, err = m.src.parse(obj.Path, "", doc)
		}
		if err == nil {
			sent.Path = dst
			err = m.dst.create(ctx, dst, sent)
			if dav.IsStatus(err, http.StatusPreconditionFailed) {
				name = fileStem(uidOf(m.src, obj)) + "-2" + m.src.spec.ext
				dst = m.placed.Path + name
				sent.Path = dst
				err = m.dst.create(ctx, dst, sent)
			}
		}
		if err != nil {
			refused = append(refused, text(name+" · "+err.Error()))
			continue
		}
		out = append(out, transferred{name: name, sent: sent})
	}
	return out, refused
}

func (m *collectionMove) copyBytes(ctx context.Context, from, to string) error {
	rc, _, err := m.src.t.Get(ctx, from, nil)
	if err != nil {
		return err
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return err
	}
	creator, ok := m.dst.t.(dav.Creator)
	if !ok {
		return fmt.Errorf("the connection cannot create")
	}
	media := cal.MediaType
	if m.src.card != nil {
		media = card.MediaType
	}
	_, err = creator.PutCreate(ctx, to, dav.Typed(bytes.NewReader(data), media))
	return err
}

// readBack reads the new collection once and compares every record with what
// was sent; a resource copied as bytes only has to be there.
func (m *collectionMove) readBack(ctx context.Context, sent []transferred) ([]view.Text, error) {
	got, err := m.dst.allRecords(ctx)
	if err != nil {
		return nil, err
	}
	there, err := members(ctx, m.dst.t, m.placed.Path)
	if err != nil {
		return nil, err
	}
	byName := map[string]*model.Object{}
	for _, obj := range got {
		byName[memberName(obj.Path)] = obj
	}
	var lost []view.Text
	for _, t := range sent {
		if _, ok := there[t.name]; !ok {
			lost = append(lost, text(t.name+" · not there after it was written"))
			continue
		}
		if t.sent == nil {
			continue
		}
		back := byName[t.name]
		if back == nil {
			lost = append(lost, text(t.name+" · not readable after it was written"))
			continue
		}
		if loss := writeLoss(m.dest.ID, t.sent, back, m.sess.Losses(), false); loss != nil {
			lost = append(lost, text(t.name+" · "+strings.Join(loss.Properties, ", ")))
		}
	}
	return lost, nil
}

func (m *collectionMove) place(move bool) error {
	return m.s.users.PlaceMovedCollection(m.sess.AccountID(), m.key, m.conn.ID, m.col.Path, m.dest.ID, m.placed, move)
}

// takeBack deletes the new collection after a failure, whatever the request's
// own context says.
func (m *collectionMove) takeBack(ctx context.Context) {
	if m.dst != nil {
		_ = m.dst.t.Delete(context.WithoutCancel(ctx), m.placed.Path, "")
		return
	}
	if client, err := dav.NewClient(m.s.guard, m.dest.Server, m.dest.Login, m.dest.Password, m.dest.CertFingerprint); err == nil {
		_ = client.Delete(context.WithoutCancel(ctx), m.placed.Path, "")
	}
}

func (m *collectionMove) catalogFailure() *view.Notice {
	return &view.Notice{Role: view.NoticeAlert, Title: text(m.title()),
		Body: text("The list of collections could not be saved, so the new " + collectionNoun(m.col) + " was taken back.")}
}

// kept is a move or copy that left both collections, with a link to the new one.
func (m *collectionMove) kept(title, body string, lines []view.Text) *view.Notice {
	n := &view.Notice{
		Role: view.NoticeAlert, Title: text(title), Body: text(body),
		Actions: []view.Action{{ID: "collection.open", Label: "Open the copy", Href: m.s.route(m.newAddress()), Method: "GET", Enabled: true}},
	}
	if len(lines) > 0 {
		n.Details = &view.Diagnostic{Summary: text("What " + connLabel(m.dest) + " did not keep"), Lines: lines}
	}
	return n
}

func moveCollectionRefusal(title, label string) refusalText {
	return refusalText{
		title:     title,
		forbidden: "This account can read here but not write",
		nothing:   label + " is as it was.",
	}
}

// members are the resources a collection holds, by file name, with their
// versions: one PROPFIND, depth 1.
func members(ctx context.Context, t dav.Transport, collection string) (map[string]string, error) {
	ms, err := t.PropFind(ctx, collection, dav.DepthOne, []xml.Name{propGetETag})
	if err != nil {
		return nil, err
	}
	self := memberName(collection)
	out := map[string]string{}
	for _, resp := range ms.Responses {
		href := hrefPath(resp.Href)
		if strings.HasSuffix(href, "/") {
			continue // the collection itself, or a collection inside it
		}
		name := memberName(href)
		if name == "" || name == self {
			continue
		}
		out[name] = strings.Trim(string(resp.Props[propGetETag]), `" `)
	}
	return out, nil
}

// liveLabel is a collection's displayname and calendar-color as the server
// holds them, one PROPFIND at depth 0; empty where it sent none.
func liveLabel(ctx context.Context, t dav.Transport, collection string) (name, colour string, err error) {
	ms, err := t.PropFind(ctx, collection, dav.DepthZero, []xml.Name{propDisplayName, propCalendarColor})
	if err != nil {
		return "", "", err
	}
	for _, resp := range ms.Responses {
		if v := propText(resp.Props[propDisplayName]); v != "" {
			name = v
		}
		if v := propText(resp.Props[propCalendarColor]); v != "" {
			colour = v
		}
	}
	return name, colour, nil
}

// propText is the text of a property element's content: "&amp;" is "&".
func propText(v []byte) string {
	var txt struct {
		Text string `xml:",chardata"`
	}
	if len(bytes.TrimSpace(v)) == 0 || bytes.Contains(v, []byte("<")) {
		return ""
	}
	if err := xml.Unmarshal(append(append([]byte("<v>"), v...), "</v>"...), &txt); err != nil {
		return ""
	}
	return strings.TrimSpace(txt.Text)
}

func memberName(p string) string {
	p = hrefPath(p)
	if u, err := url.PathUnescape(p); err == nil {
		p = u
	}
	return path.Base(strings.TrimSuffix(p, "/"))
}

func sameMembers(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for name, etag := range a {
		if other, ok := b[name]; !ok || other != etag {
			return false
		}
	}
	return true
}

func uidOf(c *collectionCtx, obj *model.Object) string {
	if c.card != nil {
		return card.UID(obj)
	}
	return cal.UID(obj)
}
