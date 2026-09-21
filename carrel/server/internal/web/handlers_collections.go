package web

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

const (
	nsCalDAV  = "urn:ietf:params:xml:ns:caldav"
	nsCardDAV = "urn:ietf:params:xml:ns:carddav"
)

var (
	propDisplayName   = xml.Name{Space: "DAV:", Local: "displayname"}
	propResourceType  = xml.Name{Space: "DAV:", Local: "resourcetype"}
	propCalendarColor = xml.Name{Space: "http://apple.com/ns/ical/", Local: "calendar-color"}
	propComponentSet  = xml.Name{Space: nsCalDAV, Local: "supported-calendar-component-set"}
)

// calendarParts are what a new calendar can hold, in the order the catalog
// keeps kinds.
var calendarParts = []struct {
	component string
	kind      view.Kind
	label     string
}{
	{"VEVENT", view.KindEvent, "Events"},
	{"VTODO", view.KindTask, "Tasks"},
	{"VJOURNAL", view.KindNote, "Notes"},
}

// collectionForm is what the new-collection sheet shows back. KindChoice is
// set when the sheet was opened without ?kind — from a connection card, where
// it is not yet known whether a calendar or an address book is wanted.
type collectionForm struct {
	kind       view.Kind
	kindChoice bool
	account    string
	name       string
	colour     string
	components []string
	address    string
	errs       map[string]view.Text
}

func newCollectionKind(v string) (k view.Kind, choice, ok bool) {
	switch view.Kind(v) {
	case "":
		return view.KindEvent, true, true
	case view.KindContact, view.KindEvent, view.KindTask, view.KindNote:
		return view.Kind(v), false, true
	}
	return "", false, false
}

func newCollectionTitle(k view.Kind) string {
	switch k {
	case view.KindContact:
		return "address book"
	case view.KindTask:
		return "list"
	case view.KindNote:
		return "notebook"
	}
	return "calendar"
}

// homeFor is where a collection of this kind is made on the connection: the
// home-set discovery found. Empty when the server has none.
func homeFor(c sealed.Connection, k view.Kind) string {
	if k == view.KindContact {
		return c.AddressBookHome
	}
	return c.CalendarHome
}

func componentOf(k view.Kind) string {
	for _, p := range calendarParts {
		if p.kind == k {
			return p.component
		}
	}
	return ""
}

// openCatalog is the start of every collection sheet: a live session and the
// catalog it opens.
func (s *Server) openCatalog(w http.ResponseWriter, r *http.Request) (*session.Session, [32]byte, sealed.Blob, bool) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return nil, [32]byte{}, sealed.Blob{}, false
	}
	key, live := sess.DataKey()
	blob, opened := s.users.Catalog(sess.AccountID(), key)
	if !live || !opened {
		s.notFound(w, r)
		return nil, [32]byte{}, sealed.Blob{}, false
	}
	return sess, key, blob, true
}

// getCollectionNew opens the sheet from the rail (?kind) or from a connection
// card (?a). Nothing is asked of the server yet: the right to create is not
// guessed (S-12.2), and a refusal is shown when it happens.
func (s *Server) getCollectionNew(w http.ResponseWriter, r *http.Request) {
	sess, _, blob, ok := s.openCatalog(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	f := collectionForm{account: q.Get("a")}
	f.kind, f.kindChoice, ok = newCollectionKind(q.Get("kind"))
	if !ok {
		s.notFound(w, r)
		return
	}
	if f.kindChoice {
		conn, known := blob.Connection(f.account)
		if !known && len(blob.Connections) > 0 {
			conn = blob.Connections[0]
		}
		if conn.CalendarHome == "" && conn.AddressBookHome != "" {
			f.kind = view.KindContact
		}
	}
	if c := componentOf(f.kind); c != "" {
		f.components = []string{c}
	}
	s.writePage(w, r, http.StatusOK, s.collectionNewSheet(sess, blob, f, nil))
}

// postCollectionNew makes the collection (S-12.1): the address from the name
// unless one was typed, a numeric suffix where the server already holds that
// address, and one MKCALENDAR or MKCOL carrying name, colour and component
// set. A refusal creates nothing and keeps the form (S-12.2).
func (s *Server) postCollectionNew(w http.ResponseWriter, r *http.Request) {
	sess, key, blob, ok := s.openCatalog(w, r)
	if !ok {
		return
	}
	f := collectionForm{errs: map[string]view.Text{}}
	f.kind, f.kindChoice, ok = newCollectionKind(r.URL.Query().Get("kind"))
	if !ok {
		s.notFound(w, r)
		return
	}
	if f.kindChoice {
		if f.kind, _, ok = newCollectionKind(r.PostFormValue("kind")); !ok || f.kind == "" {
			f.kind = view.KindEvent
		}
	}
	f.account = r.PostFormValue("account")
	f.name = strings.TrimSpace(r.PostFormValue("name"))
	f.colour = strings.ToLower(strings.TrimSpace(r.PostFormValue("colour")))
	f.address = strings.TrimSpace(r.PostFormValue("address"))
	if f.kind != view.KindContact {
		for _, p := range calendarParts {
			for _, c := range r.PostForm["components"] {
				if c == p.component {
					f.components = append(f.components, c)
					break
				}
			}
		}
		if len(f.components) == 0 {
			f.errs["components"] = text("Choose at least one: events, tasks or notes.")
		}
		if f.colour != "" && !inPalette(f.colour) {
			f.errs["colour"] = text("Choose one of the colours shown.")
		}
	}
	conn, known := blob.Connection(f.account)
	home := ""
	if known {
		home = homeFor(conn, f.kind)
	}
	if home == "" {
		f.errs["account"] = text(fmt.Sprintf("Choose a server that holds %ss.", newCollectionTitle(f.kind)))
	}
	if f.name == "" {
		f.errs["name"] = text("Give it a name.")
	}
	if p := addressProblem(f.address); p != "" {
		f.errs["address"] = text(p)
	}
	if len(f.errs) > 0 {
		s.writePage(w, r, http.StatusOK, s.collectionNewSheet(sess, blob, f, nil))
		return
	}

	client, err := dav.NewClient(s.guard, conn.Server, conn.Login, conn.Password, conn.CertFingerprint)
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.collectionNewSheet(sess, blob, f, davRefusal(createRefusal, conn, "PROPFIND", home, err)))
		return
	}
	taken, err := childSegments(r.Context(), client, home)
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.collectionNewSheet(sess, blob, f, davRefusal(createRefusal, conn, "PROPFIND", home, err)))
		return
	}
	segment := f.address
	switch {
	case segment == "":
		segment = uniqueSlug(collectionSlug(f.name), taken)
	case taken[strings.ToLower(segment)]:
		f.errs["address"] = text("Something already lives at this address on the server.")
		s.writePage(w, r, http.StatusOK, s.collectionNewSheet(sess, blob, f, nil))
		return
	}
	path := strings.TrimSuffix(home, "/") + "/" + segment + "/"
	method, props, kinds := createRequest(f)
	// A refusal saying it is already there, on a request the guard had sent
	// once before, is this MKCOL arriving twice: the collection was made.
	// settlePath below reads the home back, so a wrong guess here ends as
	// «it is not in the list», never as a collection that is not there.
	if err := client.MkColProps(r.Context(), method, path, props); err != nil && !landedOnTheFirstTry(err) {
		s.writePage(w, r, http.StatusOK, s.collectionNewSheet(sess, blob, f, davRefusal(createRefusal, conn, method, path, err)))
		return
	}
	settled, err := settlePath(r.Context(), client, home, path, taken)
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.collectionNewSheet(sess, blob, f, lostCollection(conn, method, path, err,
			"It is not in the list here: run re-discovery on this connection to take in what the server holds.")))
		return
	}
	path = settled
	name := s.settleName(r.Context(), client, path, f.name)
	col := sealed.Collection{Path: path, Name: name, Kinds: kinds, Colour: f.colour, Components: f.components}
	if err := s.users.AddCollection(sess.AccountID(), key, conn.ID, col); err != nil {
		// The server holds it and the catalog does not: take it back, so a
		// failure leaves nothing behind (S-12.2).
		_ = client.Delete(context.WithoutCancel(r.Context()), path, "")
		http.Error(w, "could not save the collection", http.StatusInternalServerError)
		return
	}
	s.redirect(w, r, "/app/"+segmentOf(kinds[0])+"/"+conn.ID+"/"+url.PathEscape(path))
}

// settleName is the name the collection ends up with. A create carries it, but
// not every server takes it: SoGo names a new address book after its address
// and ignores the displayname of the MKCOL (docs/servers.md, SV-17). So the
// name is read back, sent once more on its own when it differs, and read again
// — and what the catalog keeps is what the server shows, because that is the
// name a person sees and types to delete it (S-12.4).
func (s *Server) settleName(ctx context.Context, client *dav.Client, path, want string) string {
	got := collectionDisplayName(ctx, client, path)
	if got == "" || got == want {
		return want
	}
	if err := client.PropPatch(ctx, path, []dav.Prop{{Name: propDisplayName, Value: dav.TextValue(want)}}, nil); err != nil {
		return got
	}
	if after := collectionDisplayName(ctx, client, path); after != "" {
		return after
	}
	return want
}

// settlePath is the address the collection ends up at. A create carries one,
// but not every server takes it: Yandex answers 201 to MKCALENDAR and puts the
// calendar at an address of its own (`events-<id>`), leaving nothing at the
// address that was asked for and sending no Location to say so
// (docs/servers.md, SV-19). So the address is read back, and when it holds
// nothing, the home is read again: the one child that was not there before the
// create is where the collection went. None, or more than one — someone else
// created one at the same moment — is not guessed at, because a catalog entry
// that points at nothing is worse than saying so.
func settlePath(ctx context.Context, client *dav.Client, home, path string, before map[string]bool) (string, error) {
	if _, err := client.PropFind(ctx, path, dav.DepthZero, []xml.Name{propResourceType}); err == nil {
		return path, nil
	}
	fresh, err := newChildren(ctx, client, home, before)
	if err != nil {
		return "", fmt.Errorf("nothing at %s, and %s could not be read again: %w", path, home, err)
	}
	if len(fresh) != 1 {
		return "", fmt.Errorf("nothing at %s, and %s holds %d new collections", path, home, len(fresh))
	}
	return strings.TrimSuffix(home, "/") + "/" + fresh[0] + "/", nil
}

// newChildren are the children of the home that before does not name. The
// segments come back as the server spells them: before is keyed in lower case
// for the comparison only, because an address is not the catalog's to change.
func newChildren(ctx context.Context, client *dav.Client, home string, before map[string]bool) ([]string, error) {
	ms, err := client.PropFind(ctx, home, dav.DepthOne, []xml.Name{propResourceType})
	if err != nil {
		return nil, err
	}
	self := strings.TrimSuffix(hrefPath(home), "/")
	var fresh []string
	for _, resp := range ms.Responses {
		p := strings.TrimSuffix(hrefPath(resp.Href), "/")
		if p == self || p == "" {
			continue
		}
		seg := p[strings.LastIndex(p, "/")+1:]
		if !before[strings.ToLower(seg)] {
			fresh = append(fresh, seg)
		}
	}
	return fresh, nil
}

// lostCollection is the answer when a server made the collection somewhere
// other than the address it was given and there is no telling where: the
// catalog is not filled in with a guess. Re-discovery on the connection offers
// what the server holds (S-12.1), which is how a collection made elsewhere
// enters the catalog anyway.
func lostCollection(conn sealed.Connection, method, path string, err error, tail string) *view.Notice {
	return &view.Notice{
		Role:  view.NoticeAlert,
		Title: text("Made, but not at the address it was given"),
		Body:  text(connLabel(conn) + " took the request and put the collection at an address of its own, and Carrel could not tell which. " + tail),
		Details: &view.Diagnostic{
			Summary: text(method + " " + path),
			Lines:   []view.Text{text(err.Error())},
		},
	}
}

func collectionDisplayName(ctx context.Context, client *dav.Client, path string) string {
	ms, err := client.PropFind(ctx, path, dav.DepthZero, []xml.Name{propDisplayName})
	if err != nil {
		return ""
	}
	for _, resp := range ms.Responses {
		if v := propText(resp.Props[propDisplayName]); v != "" {
			return v
		}
	}
	return ""
}

// createRequest is the one request that makes the collection: MKCALENDAR with
// its component set, or extended MKCOL typed as an address book.
func createRequest(f collectionForm) (string, []dav.Prop, []view.Kind) {
	if f.kind == view.KindContact {
		return "MKCOL", []dav.Prop{
			{Name: propResourceType, Value: []byte(`<collection xmlns="DAV:"/><addressbook xmlns="` + nsCardDAV + `"/>`)},
			{Name: propDisplayName, Value: dav.TextValue(f.name)},
		}, []view.Kind{view.KindContact}
	}
	var comps strings.Builder
	var kinds []view.Kind
	for _, p := range calendarParts {
		for _, c := range f.components {
			if c == p.component {
				comps.WriteString(`<comp xmlns="` + nsCalDAV + `" name="` + c + `"/>`)
				kinds = append(kinds, p.kind)
			}
		}
	}
	props := []dav.Prop{
		{Name: propDisplayName, Value: dav.TextValue(f.name)},
		{Name: propComponentSet, Value: []byte(comps.String())},
	}
	if f.colour != "" {
		props = append(props, dav.Prop{Name: propCalendarColor, Value: dav.TextValue(f.colour)})
	}
	return "MKCALENDAR", props, kinds
}

// childSegments lists the last segments of what the home already holds, in
// lower case, for the numeric suffix (S-12.2).
func childSegments(ctx context.Context, client *dav.Client, home string) (map[string]bool, error) {
	ms, err := client.PropFind(ctx, home, dav.DepthOne, []xml.Name{propResourceType})
	if err != nil {
		return nil, err
	}
	self := strings.TrimSuffix(hrefPath(home), "/")
	taken := map[string]bool{}
	for _, resp := range ms.Responses {
		p := strings.TrimSuffix(hrefPath(resp.Href), "/")
		if p == self || p == "" {
			continue
		}
		taken[strings.ToLower(p[strings.LastIndex(p, "/")+1:])] = true
	}
	return taken, nil
}

func hrefPath(href string) string {
	if u, err := url.Parse(href); err == nil {
		return u.Path
	}
	return href
}

func (s *Server) collectionNewSheet(sess *session.Session, blob sealed.Blob, f collectionForm, failure *view.Notice) view.CollectionNewSheet {
	noun := newCollectionTitle(f.kind)
	sheet := view.CollectionNewSheet{Kind: f.kind, Failure: failure}
	var eligible []sealed.Connection
	for _, c := range blob.Connections {
		opt := view.TargetOption{ID: c.ID, Label: text(connLabel(c))}
		switch {
		case homeFor(c, f.kind) != "":
			eligible = append(eligible, c)
		case c.FileRoot != "":
			opt.ReadOnly, opt.Reason = true, text("Files only")
		default:
			opt.ReadOnly, opt.Reason = true, text(fmt.Sprintf("No %ss on this server", noun))
		}
		sheet.Target.Options = append(sheet.Target.Options, opt)
	}
	var chosen sealed.Connection
	for _, c := range eligible {
		if c.ID == f.account || chosen.ID == "" {
			chosen = c
		}
		if c.ID == f.account {
			break
		}
	}
	sheet.Target.Chosen = chosen.ID
	switch {
	case len(eligible) == 0:
		sheet.Note = text(fmt.Sprintf("No connected server holds %ss. Connect one that does first.", noun))
	case len(eligible) < len(blob.Connections):
		sheet.Note = text(fmt.Sprintf("%d of %d servers are listed: only a server that holds %ss can take a new one.",
			len(eligible), len(blob.Connections), noun))
	}

	action := "/app/collections/new"
	if !f.kindChoice {
		action += "?kind=" + string(f.kind)
	}
	errs := f.errs
	rows := []view.FormRow{csrfField(sess)}
	if f.kindChoice {
		rows = append(rows, view.FormRow{
			Name: "kind", Label: "What it is", Kind: view.InputSelect, Value: text(string(f.kind)),
			Options: []view.Option{
				{Value: string(view.KindEvent), Label: "Calendar", Selected: f.kind != view.KindContact},
				{Value: string(view.KindContact), Label: "Address book", Selected: f.kind == view.KindContact},
			},
			Hint: text("Tasks and notes live in calendars: choose Calendar and say below what goes in it."),
		})
	}
	server := view.FormRow{Name: "account", Label: "On which server", Kind: view.InputSelect, Value: text(chosen.ID), Required: true, Error: errs["account"]}
	for _, c := range eligible {
		server.Options = append(server.Options, view.Option{Value: c.ID, Label: text(connLabel(c)), Selected: c.ID == chosen.ID})
	}
	rows = append(rows, server,
		view.FormRow{Name: "name", Label: "Name", Kind: view.InputText, Value: text(f.name), Required: true, Error: errs["name"]})
	if f.kind != view.KindContact {
		rows = append(rows, paletteRow(f.colour, errs["colour"],
			"Written to the server as calendar-color: other clients show the same one. Left unchosen, the colour follows from the address."))
		parts := view.FormRow{Name: "components", Label: "What goes in it", Kind: view.InputCheck, Error: errs["components"],
			Hint: text("Most servers settle this when the collection is made and do not change it afterwards. The collection appears under Calendar, Tasks or Notes by what it holds, not by where it was made.")}
		for _, p := range calendarParts {
			sel := false
			for _, c := range f.components {
				sel = sel || c == p.component
			}
			parts.Options = append(parts.Options, view.Option{Value: p.component, Label: text(p.label), Selected: sel})
		}
		rows = append(rows, parts)
	}
	address := view.FormRow{Name: "address", Label: "Address", Kind: view.InputText, Value: text(f.address), Error: errs["address"],
		Hint: text("Taken from the name when left empty; yours to change. Other clients and restore by path go by the address, and it does not change later; the name they only show.")}
	if home := homeFor(chosen, f.kind); home != "" {
		address.Hint = text(serverOrigin(chosen.Server) + strings.TrimSuffix(home, "/") + "/…/ — " + string(address.Hint))
	}
	rows = append(rows, address)
	sheet.Form = s.plainForm(action, "POST", rows...)

	create := view.Action{ID: "collection.create", Label: "Create", Href: s.route(action), Method: "POST", Enabled: chosen.ID != ""}
	if !create.Enabled {
		create.Reason = sheet.Note
	}
	sheet.Actions = []view.Action{create}
	return sheet
}

// openCollectionSheet finds the collection a rename sheet is about: ?a is the
// connection, ?c the collection address. Gone, foreign and file collections
// answer 404; folders are renamed by S-17, not here.
// openCollectionSheet opens the catalog entry a sheet names. A subscription
// opens only for renaming: moving and deleting are things of a server, and a
// feed is nobody's collection here (S-25.2).
func (s *Server) openCollectionSheet(w http.ResponseWriter, r *http.Request) (*session.Session, [32]byte, sealed.Connection, sealed.Collection, bool) {
	sess, key, blob, ok := s.openCatalog(w, r)
	if !ok {
		return nil, key, sealed.Connection{}, sealed.Collection{}, false
	}
	q := r.URL.Query()
	conn, hasConn := blob.Connection(q.Get("a"))
	col, hasCol := blob.Collection(q.Get("a"), q.Get("c"))
	if !hasConn || !hasCol || col.Gone || len(col.Kinds) == 0 || (col.Has(view.KindFile) && !col.Feed) ||
		(col.Feed && !strings.HasSuffix(r.URL.Path, "/rename")) {
		s.notFound(w, r)
		return nil, key, sealed.Connection{}, sealed.Collection{}, false
	}
	return sess, key, conn, col, true
}

func (s *Server) getCollectionRename(w http.ResponseWriter, r *http.Request) {
	sess, _, conn, col, ok := s.openCollectionSheet(w, r)
	if !ok {
		return
	}
	colour := string(catalogColour(col))
	if !inPalette(colour) {
		colour = ""
	}
	s.writePage(w, r, http.StatusOK, s.collectionRenameSheet(sess, conn, col, col.Name, colour, nil, nil))
}

// postCollectionRename writes the name, and the colour of a calendar, in one
// PROPPATCH (S-12.2). The address stays: no MOVE is sent. A refusal keeps the
// form and changes nothing in the catalog.
func (s *Server) postCollectionRename(w http.ResponseWriter, r *http.Request) {
	sess, key, conn, col, ok := s.openCollectionSheet(w, r)
	if !ok {
		return
	}
	name := strings.TrimSpace(r.PostFormValue("name"))
	colour := strings.ToLower(strings.TrimSpace(r.PostFormValue("colour")))
	errs := map[string]view.Text{}
	if name == "" {
		errs["name"] = text("Give it a name.")
	}
	if colour != "" && (col.Has(view.KindContact) || !inPalette(colour)) {
		errs["colour"] = text("Choose one of the colours shown.")
	}
	if len(errs) > 0 {
		s.writePage(w, r, http.StatusOK, s.collectionRenameSheet(sess, conn, col, name, colour, errs, nil))
		return
	}
	if conn.Feed {
		// A subscription's name and colour are its own here: nothing goes to
		// the feed's server (S-25.2).
		if err := s.users.EditFeed(sess.AccountID(), key, conn.ID, name, colour, nil); err != nil {
			http.Error(w, "could not save the subscription", http.StatusInternalServerError)
			return
		}
		s.redirect(w, r, "/app/settings/connections")
		return
	}
	props := []dav.Prop{{Name: propDisplayName, Value: dav.TextValue(name)}}
	if colour != "" {
		props = append(props, dav.Prop{Name: propCalendarColor, Value: dav.TextValue(colour)})
	}
	client, err := dav.NewClient(s.guard, conn.Server, conn.Login, conn.Password, conn.CertFingerprint)
	if err == nil {
		err = client.PropPatch(r.Context(), col.Path, props, nil)
	}
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.collectionRenameSheet(sess, conn, col, name, colour, nil,
			davRefusal(renameRefusal, conn, "PROPPATCH", col.Path, err)))
		return
	}
	if err := s.users.RenameCollection(sess.AccountID(), key, conn.ID, col.Path, name, colour); err != nil {
		http.Error(w, "could not save the collection", http.StatusInternalServerError)
		return
	}
	s.redirect(w, r, "/app/settings/connections")
}

func (s *Server) collectionRenameSheet(sess *session.Session, conn sealed.Connection, col sealed.Collection, name, colour string, errs map[string]view.Text, failure *view.Notice) view.CollectionRenameSheet {
	action := "/app/collections/rename?" + url.Values{"a": {conn.ID}, "c": {col.Path}}.Encode()
	rows := []view.FormRow{
		csrfField(sess),
		{Name: "name", Label: "Name", Kind: view.InputText, Value: text(name), Required: true, Error: errs["name"]},
	}
	if !col.Has(view.KindContact) {
		rows = append(rows, paletteRow(colour, errs["colour"], "Written to the server as calendar-color. Left unchosen, the colour stays as it is."))
	}
	if conn.Feed {
		if !col.Has(view.KindContact) {
			rows[len(rows)-1] = paletteRow(colour, errs["colour"], "Kept here: a feed sends no colour of its own.")
		}
		return view.CollectionRenameSheet{
			Kind:    col.Kinds[0],
			Form:    s.plainForm(action, "POST", rows...),
			Address: text(conn.Server),
			Note:    "The name is yours: it stays in Carrel and is never sent to the feed.",
			Actions: []view.Action{{ID: "collection.save", Label: "Save", Href: s.route(action), Method: "POST", Enabled: true}},
		}
	}
	return view.CollectionRenameSheet{
		Kind:    col.Kinds[0],
		Stamp:   view.Stamp(connLabel(conn)),
		Form:    s.plainForm(action, "POST", rows...),
		Address: text(serverOrigin(conn.Server) + col.Path),
		Note: text("Renaming is not moving: the address stays as it is, so devices, published links and backup jobs pointing here go on working. " +
			"The new name is written to the server, and other clients show it at their next sync."),
		Failure: failure,
		Actions: []view.Action{{ID: "collection.save", Label: "Save", Href: s.route(action), Method: "POST", Enabled: true}},
	}
}

// paletteRow offers the eight tones of C-10, not an arbitrary colour.
func paletteRow(chosen string, errText view.Text, hint string) view.FormRow {
	row := view.FormRow{Name: "colour", Label: "Colour", Kind: view.InputRadio, Value: text(chosen), Error: errText, Hint: text(hint)}
	for _, c := range collectionPalette {
		row.Options = append(row.Options, view.Option{Value: string(c), Label: text(string(c)), Selected: string(c) == chosen})
	}
	return row
}

func inPalette(c string) bool {
	for _, p := range collectionPalette {
		if strings.EqualFold(string(p), c) {
			return true
		}
	}
	return false
}

func serverOrigin(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	return ""
}

type refusalText struct{ title, forbidden, nothing string }

var (
	createRefusal = refusalText{
		title:     "The server would not make it",
		forbidden: "This account can read here but not create",
		nothing:   "Nothing was created, and the form is as you left it.",
	}
	renameRefusal = refusalText{
		title:     "The server refused the change",
		forbidden: "This account can read here but not change it",
		nothing:   "Nothing was renamed, and the form is as you left it.",
	}
)

// davRefusal is a server refusal as the content of the sheet (C-9.1): method,
// address, code and body. A 403 says what it means.
func davRefusal(rt refusalText, conn sealed.Connection, method, path string, err error) *view.Notice {
	n := &view.Notice{
		Role:  view.NoticeAlert,
		Title: text(rt.title),
		Body:  text(rt.nothing),
		Details: &view.Diagnostic{
			Summary: text(method + " " + path),
			Lines:   []view.Text{text(connLabel(conn) + path)},
		},
	}
	code := dav.StatusCode(err)
	switch {
	case code == http.StatusForbidden:
		n.Title = text(rt.forbidden)
		n.Body = text(rt.nothing + " The server does not give this account the right: either the user has no such right there, or the password was issued read-only.")
	case code == 0:
		n.Title = text("Could not reach the server")
	}
	if code != 0 {
		n.Details.Lines = append(n.Details.Lines, text(fmt.Sprintf("%s %s → %d", method, path, code)))
	}
	n.Details.Lines = append(n.Details.Lines, text(err.Error()))
	return n
}
