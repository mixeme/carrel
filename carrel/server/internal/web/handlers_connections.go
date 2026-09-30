package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/discover"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// connectionForm is what the Add form shows back. The password is never
// echoed: a failed attempt asks for it again. NoPassword is the mark «The
// server asks no password» (s-profile-14), echoed as it was sent.
type connectionForm struct {
	Label, Server, Login, CertFingerprint string
	ServerErr, LabelErr                   view.Text
	NoPassword                            bool
	// Editing is the connection the form edits; empty for a new one.
	Editing string
	// Open asks for the dialog without anything typed yet: Connect account
	// (s-profile-2). A form with anything in it is the dialog anyway.
	Open bool
}

// noPasswordMark reads the mark «The server asks no password». While it is
// on, the password field is off and its value is not read.
func noPasswordMark(r *http.Request) bool {
	v := r.FormValue("noPassword")
	return v == "on" || v == "true"
}

// getSettingsConnections shows the connections; ?reveal={account} fills that
// card's password when the unlock window is open (S-10.7).
func (s *Server) getSettingsConnections(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	f := connectionForm{Server: r.FormValue("server"), CertFingerprint: r.FormValue("certFingerprint")}
	f.Open = r.URL.Query().Get("op") == "add"
	if r.URL.Query().Get("op") == "edit" {
		key, live := sess.DataKey()
		blob, opened := s.users.Catalog(sess.AccountID(), key)
		conn, known := blob.Connection(r.URL.Query().Get("account"))
		if !live || !opened || !known || conn.Feed {
			s.notFound(w, r)
			return
		}
		// A connection stored with an empty password asks none, whether it was
		// said by the mark or before the mark existed.
		f = connectionForm{Label: conn.Label, Server: conn.Server, Login: conn.Login, CertFingerprint: conn.CertFingerprint,
			NoPassword: conn.NoPassword || conn.Password == "", Editing: conn.ID}
	}
	page := s.connectionsPage(sess, f, nil)
	if r.URL.Query().Get("op") == "feed" {
		page.Add = s.feedForm(sess, feedForm{Folder: r.URL.Query().Get("kind") == "folder"})
	}
	if id := r.URL.Query().Get("reveal"); id != "" && s.revealPassword(sess, &page, id) {
		w.Header().Set("Cache-Control", "no-store")
	}
	s.writePage(w, r, http.StatusOK, page)
}

// postSettingsConnections adds a connection. op=rediscover and op=apply run
// discovery again on one that exists (S-6.1).
func (s *Server) postSettingsConnections(w http.ResponseWriter, r *http.Request) {
	switch r.FormValue("op") {
	case "":
		s.addConnection(w, r)
	case "rediscover":
		s.rediscover(w, r, false)
	case "apply":
		s.rediscover(w, r, true)
	case "test":
		s.testConnection(w, r)
	case "edit":
		s.editConnection(w, r)
	case "feed.check", "feed.subscribe":
		s.postFeed(w, r)
	case "feed.recheck":
		s.recheckFeed(w, r)
	case "feed.unsubscribe":
		s.unsubscribe(w, r)
	default:
		if _, ok := s.requireLogin(w, r); ok {
			s.notFound(w, r)
		}
	}
}

// addConnection runs discovery (S-6) through the guard with the accepted
// fingerprint (F-13) and writes the connection with its collections into the
// catalog. A failed discovery saves nothing and says at which step it stopped,
// with the whole trace beside it (C-9.1) — screen state, not a 500.
func (s *Server) addConnection(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	key, live := sess.DataKey()
	if !live {
		s.notFound(w, r)
		return
	}
	f := connectionForm{
		Label:           connectionLabel(r.FormValue("label")),
		Server:          strings.TrimSpace(r.FormValue("server")),
		Login:           strings.TrimSpace(r.FormValue("login")),
		CertFingerprint: strings.TrimSpace(r.FormValue("certFingerprint")),
		NoPassword:      noPasswordMark(r),
	}
	if !validServerURL(f.Server) {
		f.ServerErr = text("Enter the full address, starting with https:// or http://.")
		s.writePage(w, r, http.StatusOK, s.connectionsPage(sess, f, nil))
		return
	}
	if f.Login == "" {
		// An address without a login is a link, not an account (owner's
		// definition, 18.09.2026): it is subscribed to, read-only, and the
		// same address is already in the subscribe form. The address is not
		// put in a URL — a link may carry its secret in it.
		page := s.connectionsPage(sess, connectionForm{}, &view.Notice{
			Role:  view.NoticeInfo,
			Title: "An address without a login is a link, not an account",
			Body: "An account is an address with its login and password: that is what Carrel writes to. A collection reached by a link — " +
				"a calendar's secret address, a shared folder, another Carrel's published link — is a subscription: read-only, and the link's own password, if it has one, is kept with it. The form below subscribes to this address.",
		})
		page.Add = s.feedForm(sess, feedForm{URL: f.Server, Folder: !looksLikeFeed(f.Server)})
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	blob := s.catalogOf(sess)
	if f.Label != "" && labelTaken(blob, f.Label, "") {
		f.LabelErr = labelTakenText(f.Label)
		s.writePage(w, r, http.StatusOK, s.connectionsPage(sess, f, nil))
		return
	}
	password := r.FormValue("password")
	if f.NoPassword {
		password = ""
	}
	pin := dav.NormalizeFingerprint(f.CertFingerprint)

	res, trace, err := discover.Run(r.Context(), s.guard, f.Server, f.Login, password, pin)
	if err != nil || res == nil || trace == nil || !trace.OK {
		s.writePage(w, r, http.StatusOK, s.connectionsPage(sess, f, discoveryFailure(trace, err, "Nothing was saved.")))
		return
	}

	conn := sealed.Connection{
		Label:           f.Label,
		Server:          res.BaseURL,
		Login:           f.Login,
		Password:        password,
		NoPassword:      f.NoPassword,
		CertFingerprint: pin,
		CalendarHome:    res.CalendarHome,
		AddressBookHome: res.AddressBookHome,
		Scheduling:      res.Scheduling,
		Addresses:       res.Addresses,
	}
	if res.FileOnly {
		conn.FileRoot = res.BaseURL
	}
	if labelTaken(blob, connLabel(conn), "") {
		// Without a label the server's host is the label, and a second
		// connection to the same server without one is refused the same way.
		f.LabelErr = labelTakenText(connLabel(conn))
		s.writePage(w, r, http.StatusOK, s.connectionsPage(sess, f, nil))
		return
	}
	if _, err := s.users.AddConnection(sess.AccountID(), key, conn, catalogEntries(res.Collections)); err != nil {
		http.Error(w, "could not save the connection", http.StatusInternalServerError)
		return
	}
	s.redirect(w, r, "/app/settings/connections")
}

// rediscover runs discovery again for one connection with what the catalog
// holds (S-6.1). Without apply it only offers the difference; with apply it
// writes what the person was shown and the server still confirms. A failed
// run changes nothing and marks nothing gone: a server that does not answer
// has not deleted anything (C-8.2).
func (s *Server) rediscover(w http.ResponseWriter, r *http.Request, apply bool) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	key, live := sess.DataKey()
	if !live {
		s.notFound(w, r)
		return
	}
	blob, opened := s.users.Catalog(sess.AccountID(), key)
	conn, known := blob.Connection(r.FormValue("account"))
	if !opened || !known {
		s.notFound(w, r)
		return
	}
	res, trace, err := discover.Run(r.Context(), s.guard, conn.Server, conn.Login, conn.Password, conn.CertFingerprint)
	if trace != nil {
		sess.NoteCheck(conn.ID, trace) // a re-discovery is a check too
	}
	if err != nil || res == nil || trace == nil || !trace.OK {
		s.writePage(w, r, http.StatusOK, s.connectionsPage(sess, connectionForm{}, discoveryFailure(trace, err, "Nothing was changed.")))
		return
	}
	if !apply {
		s.writePage(w, r, http.StatusOK, s.connectionsPage(sess, connectionForm{}, s.rediscoveryNotice(blob, conn, res.Collections)))
		return
	}
	proposal := user.Rediscovery{Found: catalogEntries(res.Collections), Add: r.Form["add"], Gone: r.Form["gone"],
		Scheduling: res.Scheduling, Addresses: res.Addresses}
	if err := s.users.ApplyRediscovery(sess.AccountID(), key, conn.ID, proposal); err != nil {
		http.Error(w, "could not save the catalog", http.StatusInternalServerError)
		return
	}
	s.redirect(w, r, "/app/settings/connections")
}

// editConnection saves a connection's new address, login, password or
// fingerprint under the same ID (C-4.1), and only after discovery answers
// with them: a failed run changes nothing and gives the form back. The mark
// «The server asks no password» clears the stored password (s-profile-14);
// without it an empty password is the stored one. The catalog is not
// replaced: what discovery finds is compared with it and offered, as
// re-discovery does (S-6.1), so a collection at the same path keeps its ticks
// and its links.
func (s *Server) editConnection(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	key, live := sess.DataKey()
	if !live {
		s.notFound(w, r)
		return
	}
	blob, opened := s.users.Catalog(sess.AccountID(), key)
	conn, known := blob.Connection(r.FormValue("account"))
	if !opened || !known || conn.Feed {
		s.notFound(w, r)
		return
	}
	f := connectionForm{
		Label:           connectionLabel(r.FormValue("label")),
		Server:          strings.TrimSpace(r.FormValue("server")),
		Login:           strings.TrimSpace(r.FormValue("login")),
		CertFingerprint: strings.TrimSpace(r.FormValue("certFingerprint")),
		NoPassword:      noPasswordMark(r),
		Editing:         conn.ID,
	}
	if !validServerURL(f.Server) {
		f.ServerErr = text("Enter the full address, starting with https:// or http://.")
		s.writePage(w, r, http.StatusOK, s.connectionsPage(sess, f, nil))
		return
	}
	if f.Label != "" && labelTaken(blob, f.Label, conn.ID) {
		f.LabelErr = labelTakenText(f.Label)
		s.writePage(w, r, http.StatusOK, s.connectionsPage(sess, f, nil))
		return
	}
	password := r.FormValue("password")
	switch {
	case f.NoPassword:
		password = ""
	case password == "":
		password = conn.Password
	}
	pin := dav.NormalizeFingerprint(f.CertFingerprint)
	res, trace, err := discover.Run(r.Context(), s.guard, f.Server, f.Login, password, pin)
	if err != nil || res == nil || trace == nil || !trace.OK {
		s.writePage(w, r, http.StatusOK, s.connectionsPage(sess, f, discoveryFailure(trace, err, "Nothing was changed.")))
		return
	}
	edited := sealed.Connection{
		ID:              conn.ID,
		Label:           f.Label,
		Server:          res.BaseURL,
		Login:           f.Login,
		Password:        password,
		NoPassword:      f.NoPassword,
		CertFingerprint: pin,
		CalendarHome:    res.CalendarHome,
		AddressBookHome: res.AddressBookHome,
		Scheduling:      res.Scheduling,
		Addresses:       res.Addresses,
	}
	if res.FileOnly {
		edited.FileRoot = res.BaseURL
	}
	if labelTaken(blob, connLabel(edited), conn.ID) {
		f.LabelErr = labelTakenText(connLabel(edited))
		s.writePage(w, r, http.StatusOK, s.connectionsPage(sess, f, nil))
		return
	}
	if err := s.users.UpdateConnection(sess.AccountID(), key, edited); err != nil {
		http.Error(w, "could not save the connection", http.StatusInternalServerError)
		return
	}
	sess.Cache().InvalidateConnection(conn.ID)
	sess.NoteCheck(conn.ID, trace)
	s.writePage(w, r, http.StatusOK, s.connectionsPage(sess, connectionForm{}, s.rediscoveryNotice(blob, edited, res.Collections)))
}

// testConnection is Test on a saved connection: discovery with the stored
// credentials through the guard, nothing written (C-6, C-9.1). The trace
// stays on the card for the session; a failed one turns the card
// unavailable and its button into Test again.
func (s *Server) testConnection(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	key, live := sess.DataKey()
	if !live {
		s.notFound(w, r)
		return
	}
	blob, opened := s.users.Catalog(sess.AccountID(), key)
	conn, known := blob.Connection(r.FormValue("account"))
	if !opened || !known || conn.Feed {
		s.notFound(w, r)
		return
	}
	_, trace, err := discover.Run(r.Context(), s.guard, conn.Server, conn.Login, conn.Password, conn.CertFingerprint)
	if trace == nil {
		trace = &view.DiagnosticTrace{}
	}
	if err != nil && len(trace.Steps) == 0 {
		trace.Steps = []view.TraceStep{{Name: "connect", State: view.CheckBad, Detail: text(err.Error())}}
	}
	sess.NoteCheck(conn.ID, trace)
	s.writePage(w, r, http.StatusOK, s.connectionsPage(sess, connectionForm{}, nil))
}

// rediscoveryNotice is the S-6.1 proposal: what appeared and what disappeared
// against the catalog, and Apply. Entries already marked gone stay out of the
// comparison, so one that came back is offered again.
func (s *Server) rediscoveryNotice(blob sealed.Blob, conn sealed.Connection, found []discover.Collection) *view.Notice {
	var stored []discover.Collection
	names := map[string]string{}
	for _, c := range blob.CollectionsOf(conn.ID) {
		if !c.Gone {
			stored = append(stored, discover.Collection{Path: c.Path, DisplayName: c.Name})
			names[c.Path] = collectionLabel(c)
		}
	}
	d := discover.CompareCollections(found, stored)
	host := connLabel(conn)
	if len(d.Added) == 0 && len(d.Removed) == 0 {
		return &view.Notice{
			Role:  view.NoticeInfo,
			Title: text("Nothing appeared or disappeared"),
			Body:  text(fmt.Sprintf("%s holds the same %s as the catalog.", host, plural(len(stored), "collection"))),
		}
	}
	apply := url.Values{"op": {"apply"}, "account": {conn.ID}}
	var parts []string
	details := &view.Diagnostic{Summary: text("Addresses")}
	if len(d.Added) > 0 {
		var added []string
		for _, c := range d.Added {
			added = append(added, collectionLabel(sealed.Collection{Path: c.Path, Name: c.DisplayName}))
			apply.Add("add", c.Path)
			details.Lines = append(details.Lines, text("appeared · "+c.Path))
		}
		parts = append(parts, "Appeared: "+strings.Join(added, ", ")+".")
	}
	if len(d.Removed) > 0 {
		var gone []string
		for _, p := range d.Removed {
			gone = append(gone, names[p])
			apply.Add("gone", p)
			details.Lines = append(details.Lines, text("gone · "+p))
		}
		parts = append(parts, "Gone: "+strings.Join(gone, ", ")+".")
	}
	return &view.Notice{
		Role:  view.NoticeAccent,
		Title: text("Collections changed on the server"),
		Body: text(fmt.Sprintf("%s holds %s; the catalog has %s. %s",
			host, plural(len(found), "collection"), plural(len(stored), "collection"), strings.Join(parts, " "))),
		Details: details,
		Actions: []view.Action{
			{ID: "connection.rediscover.apply", Label: "Apply", Href: s.postQuery("/app/settings/connections", apply), Method: "POST", Enabled: true},
			{ID: "connection.rediscover.keep", Label: "Keep the catalog", Href: s.route("/app/settings/connections"), Method: "GET", Enabled: true},
		},
	}
}

// connectionsPage is the list of connections (s-profile-1), or the dialog
// that adds or edits one when f holds anything (s-profile-2): the dialog is
// a state of its own, opened by Connect account, and the list carries no
// form.
func (s *Server) connectionsPage(sess *session.Session, f connectionForm, notice *view.Notice) view.ConnectionsPage {
	page := view.ConnectionsPage{
		Shell: s.withRail(s.buildShell(sess, "Connections"), s.settingsRail(sess, "/app/settings/connections")),
	}
	// The dialog stands over the page, and the page under it is as it is:
	// the dialog has a head of its own (s-profile-2, s-profile-16).
	if f.Open || f != (connectionForm{}) {
		add := s.connectionForm(sess, f)
		page.Add = &add
		page.DialogTitle, page.DialogSubtitle = "Connect account", "One connection gives calendars and address books alike."
		if f.Editing != "" {
			page.DialogTitle = "Edit connection"
		}
	}
	if key, live := sess.DataKey(); live {
		if blob, ok := s.users.Catalog(sess.AccountID(), key); ok {
			unlocked := sess.UnlockUntil(time.Now()) != nil
			tf := s.timeFor(sess)
			cols := 0
			for _, c := range blob.Connections {
				card := s.connectionCard(sess, tf, blob, c, unlocked)
				cols += len(card.Collections)
				page.Accounts = append(page.Accounts, card)
			}
			for _, f := range blob.Subscriptions {
				page.Subscriptions = append(page.Subscriptions, s.subscriptionCard(sess, tf, f))
			}
			// The DAV servers are the cards, the subscriptions apart; the
			// collections are the rows of those cards, unticked ones too, and
			// a server not answering now counts what it had at its last
			// answer (s-profile-1).
			page.Subtitle = text(fmt.Sprintf("Signed in as %s · %s · %s · %s", sess.Login(),
				plural(len(blob.Connections), "server"), plural(cols, "collection"), plural(len(blob.Subscriptions), "subscription")))
			page.Actions = append(page.Actions,
				view.Action{ID: "connection.add", Label: "Connect account", Icon: "i-plus", Href: s.connectHref(), Method: "GET", Enabled: true},
				view.Action{ID: "feed.subscribe", Label: "Subscribe to a feed", Icon: "i-link",
					Href: s.postQuery("/app/settings/connections", url.Values{"op": {"feed"}}), Method: "GET", Enabled: true})
			if n, ok := goneNotice(blob); ok {
				page.Shell.Notices = append(page.Shell.Notices, n)
			}
		}
	}
	if notice != nil {
		page.Shell.Notices = append(page.Shell.Notices, *notice)
	}
	page.Shell.Notices = append(page.Shell.Notices, connectionWarnings(f.Server, f.CertFingerprint)...)
	return page
}

// connectionForm is Add, or Edit of one connection: the same fields, the
// address and login filled, the password empty and kept when left so. The
// mark «The server asks no password» stands right after it and turns it off
// (s-profile-14): an empty active field means «unchanged», so «no password»
// cannot be said by emptiness.
func (s *Server) connectionForm(sess *session.Session, f connectionForm) view.Form {
	password := view.FormRow{Name: "password", Label: "Password", Kind: view.InputPassword, ReadOnly: f.NoPassword}
	action, id, label := "/app/settings/connections", "connection.add", "Connect"
	if f.Editing != "" {
		// Beside a field the mark turned off there is nothing kept to keep
		// (s-profile-16).
		if !f.NoPassword {
			password.Hint = "Leave it empty to keep the stored one."
		}
		action = s.postQuery("/app/settings/connections", url.Values{"op": {"edit"}, "account": {f.Editing}})
		id, label = "connection.edit.save", "Save"
	}
	form := s.plainForm(action, "POST",
		csrfField(sess),
		view.FormRow{Name: "label", Label: "Label", Kind: view.InputText, Value: text(f.Label), Error: f.LabelErr, Placeholder: "optional"},
		view.FormRow{Name: "server", Label: "Server URL", Kind: view.InputURL, Value: text(f.Server), Error: f.ServerErr, Required: true,
			Hint: ".well-known is tried first, then the URL exactly as typed."},
		view.FormRow{Name: "login", Label: "Username", Kind: view.InputText, Value: text(f.Login)},
		password,
		view.FormRow{Name: "noPassword", Label: "The server asks no password", Kind: view.InputCheck, Value: text(boolStr(f.NoPassword)),
			Disables: "password"},
		view.FormRow{Name: "certFingerprint", Label: "Certificate fingerprint", LabelAside: "SHA-256 · optional", Kind: view.InputText, Value: text(f.CertFingerprint),
			Placeholder: "only for a certificate the system does not trust"},
	)
	// The caret is in Server URL: Label is optional (s-profile-2).
	form = focused(form, "server")
	if f.Editing != "" {
		form.Action = action
		return withSubmit(form, id, label,
			view.Action{ID: "connection.edit.cancel", Label: "Cancel", Href: s.route("/app/settings/connections"), Method: "GET", Enabled: true})
	}
	// The dialog is the content of main, not a sheet over the screen: the
	// client has nothing to close it with, so its Cancel is here (C-7.2).
	return withSubmit(form, id, label,
		view.Action{ID: "connection.test", Label: "Test only", Href: s.route("/app/settings/connections/test"), Method: "POST", Enabled: true},
		view.Action{ID: "connection.add.cancel", Label: "Cancel", Href: s.route("/app/settings/connections"), Method: "GET", Enabled: true})
}

// connectionCard never carries the password. Outside the unlock window Show
// password opens the sheet; inside it asks for this card's password, and that
// request is what releases and journals it (S-10.7).
func (s *Server) connectionCard(sess *session.Session, tf timeFmt, blob sealed.Blob, c sealed.Connection, unlocked bool) view.ConnectionCard {
	card := view.ConnectionCard{
		AccountID: c.ID,
		Title:     text(connLabel(c)),
		Server:    text(c.Server),
		Login:     text(c.Login),
		State:     view.CollectionOK,
		Actions: []view.Action{{
			ID: "connection.test", Label: "Test",
			Href:   s.postQuery("/app/settings/connections", url.Values{"op": {"test"}, "account": {c.ID}}),
			Method: "POST", Enabled: true,
		}, {
			ID: "connection.edit", Label: "Edit",
			Href:   s.postQuery("/app/settings/connections", url.Values{"op": {"edit"}, "account": {c.ID}}),
			Method: "GET", Enabled: true,
		}, {
			ID: "connection.disconnect", Label: "Disconnect", Danger: true,
			Href:   s.postQuery("/app/settings/connections/disconnect", url.Values{"account": {c.ID}}),
			Method: "GET", Enabled: true,
		}, {
			ID: "connection.rediscover", Label: "Look for changes",
			Href:   s.postQuery("/app/settings/connections", url.Values{"op": {"rediscover"}, "account": {c.ID}}),
			Method: "POST", Enabled: true,
		}},
	}
	// Nothing to release from a server that asks no password: the address
	// line says «no password» and there is no Show password (s-profile-15).
	if c.NoPassword || c.Password == "" {
		card.Access = "no password"
	} else {
		reveal := view.Action{ID: "connection.password", Label: "Show password", Href: s.route("/app/settings/unlock"), Method: "GET", Enabled: true}
		if unlocked {
			reveal.Href = s.postQuery("/app/settings/connections", url.Values{"reveal": {c.ID}})
		}
		card.Password = &view.SecretField{Reveal: reveal}
	}
	if trace := sess.CheckOf(c.ID); trace != nil {
		card.Trace = trace
		if !trace.OK {
			card.State = view.CollectionUnavailable
			card.Actions[0].Label = "Test again"
		}
	}
	// Not hidden in advance and not greyed on read-only rows: the right to
	// create or rename is not guessed, a refusal is shown when it happens
	// (S-12.2).
	if c.CalendarHome != "" || c.AddressBookHome != "" {
		card.Actions = append(card.Actions, view.Action{
			ID: "collection.new", Label: "New collection", Icon: "i-plus",
			Href:   s.postQuery("/app/collections/new", url.Values{"a": {c.ID}}),
			Method: "GET", Enabled: true,
		})
	}
	// A plain file storage makes folders (s-profile-1): the same sheet, for
	// a folder beside its folders (S-12.1).
	if c.FileRoot != "" {
		card.Actions = append(card.Actions, view.Action{
			ID: "folder.new", Label: "New folder", Icon: "i-plus",
			Href:   s.postQuery("/app/collections/new", url.Values{"a": {c.ID}, "kind": {string(view.KindFile)}}),
			Method: "GET", Enabled: true,
		})
	}
	for _, col := range blob.CollectionsOf(c.ID) {
		if col.Gone {
			continue // named by goneNotice: a row has no state to carry it
		}
		row := view.CollectionRow{
			Label:    text(collectionLabel(col)),
			Path:     text(col.Path),
			Colour:   catalogColour(col),
			Kinds:    col.Kinds,
			ReadOnly: col.ReadOnly,
		}
		row.LastRead, row.Changed = readingOf(sess, tf, c.ID, col.Path)
		row.Menu = s.collectionRowMenu(blob, c, col)
		card.Collections = append(card.Collections, row)
	}
	return card
}

// collectionRowMenu is the «⋯» of a collection in its connection's card. A
// calendar or an address book (s-profile-1): Rename and colour, Move and Copy
// to another server, Export .ics or .vcf — the collection whole, as the
// delete sheet hands it over — Publish a link, and under a line Delete on
// the server…. A folder of a file storage (s-profile-12) is the same without
// the export: a folder is downloaded in Files.
func (s *Server) collectionRowMenu(blob sealed.Blob, c sealed.Connection, col sealed.Collection) []view.MenuItem {
	if col.Gone || len(col.Kinds) == 0 {
		return nil
	}
	rename := view.Action{ID: "collection.rename", Label: "Rename and colour", Icon: "i-pencil",
		Href: s.postQuery("/app/collections/rename", url.Values{"a": {c.ID}, "c": {col.Path}}), Method: "GET", Enabled: true}
	if col.Has(view.KindFile) {
		rename.Label = "Rename"
		if storageRoot(c, col) {
			rename.Enabled, rename.Href, rename.Reason = false, "", storageRootReason
		}
	}
	menu := []view.MenuItem{{Action: rename}}
	menu = append(menu, s.collectionMoveItems(blob, c, col)...)
	if col.Has(view.KindFile) {
		menu = append(menu, view.MenuItem{Action: view.Action{ID: "publish", Label: "Publish a link", Icon: "i-link", Method: "GET", Enabled: true,
			Href: s.postQuery("/app/publish", url.Values{"k": {filesSpec.segment}, "a": {c.ID}, "c": {col.Path}, "f": {""}, "dir": {"1"}})}})
	} else {
		export := s.collectionExport(c, col)
		menu = append(menu, view.MenuItem{Action: export},
			view.MenuItem{Action: s.publishActionAt(origin{conn: c, col: col}, segmentOf(col.Kinds[0]), "")})
	}
	if item, ok := s.collectionDeleteItem(c, col); ok {
		menu = append(menu, item)
	}
	return menu
}

// revealPassword fills one card's password: only inside this session's unlock
// window, only for this person's connection, and only once the release is in
// the journal.
func (s *Server) revealPassword(sess *session.Session, page *view.ConnectionsPage, id string) bool {
	key, live := sess.DataKey()
	if !live || sess.UnlockUntil(time.Now()) == nil {
		return false
	}
	for i := range page.Accounts {
		if page.Accounts[i].AccountID != id {
			continue
		}
		if page.Accounts[i].Password == nil || !page.Accounts[i].Password.Reveal.Enabled {
			return false // no password to release, and nothing to journal
		}
		password, err := s.users.RevealConnectionPassword(sess.AccountID(), key, id)
		if err != nil {
			return false
		}
		page.Accounts[i].Password = &view.SecretField{
			Shown:  true,
			Value:  text(password),
			Reveal: view.Action{ID: "connection.password", Label: "Hide password", Href: s.route("/app/settings/connections"), Method: "GET", Enabled: true},
		}
		return true
	}
	return false
}

// goneNotice names catalog entries the server no longer holds (S-6.1). They
// stay in the catalog, so a selection that ticks one says gone instead of
// losing it.
func goneNotice(blob sealed.Blob) (view.Notice, bool) {
	var names []string
	for _, c := range blob.Collections {
		if !c.Gone {
			continue
		}
		conn, _ := blob.Connection(c.AccountID)
		names = append(names, connLabel(conn)+" · "+collectionLabel(c))
	}
	if len(names) == 0 {
		return view.Notice{}, false
	}
	return view.Notice{
		Role:  view.NoticeInfo,
		Title: text("Gone from the server"),
		Body:  text("No longer on the server: " + strings.Join(names, ", ") + ". Where they were selected they stay, marked gone; Look for changes offers them again if they come back."),
	}, true
}

// catalogEntries turns what discovery found into catalog entries.
func catalogEntries(found []discover.Collection) []sealed.Collection {
	cols := make([]sealed.Collection, 0, len(found))
	for _, c := range found {
		cols = append(cols, sealed.Collection{
			Path:            c.Path,
			Name:            c.DisplayName,
			Kinds:           c.Kinds,
			Colour:          c.Colour,
			ReadOnly:        c.ReadOnly,
			Components:      c.Components,
			ChangeTag:       c.ChangeTag,
			MaxResourceSize: c.MaxResourceSize,
		})
	}
	return cols
}

// discoveryFailure names the last step that went wrong — steps run in order,
// and an earlier bad well-known probe is routine — and carries every step.
// nothing says what did not happen: a new connection is not saved, an
// existing one is not changed.
// connectHref opens the dialog Connect account (s-profile-2): the head of
// the connections and the foot of every rail go there.
func (s *Server) connectHref() string {
	return s.postQuery("/app/settings/connections", url.Values{"op": {"add"}})
}

func (s *Server) connectAction() view.Action {
	return view.Action{ID: "account.connect", Label: "Connect account", Icon: "i-server", Href: s.connectHref(), Method: "GET", Enabled: true}
}

func discoveryFailure(trace *view.DiagnosticTrace, err error, nothing string) *view.Notice {
	n := &view.Notice{
		Role:  view.NoticeAlert,
		Title: text("Could not connect"),
		Body:  text(nothing),
	}
	d := &view.Diagnostic{Summary: text("Discovery trace")}
	var failed view.Text
	if trace != nil {
		for _, st := range trace.Steps {
			d.Lines = append(d.Lines, text(fmt.Sprintf("%s · %s · %s", st.Name, st.State, st.Detail)))
			if st.State == view.CheckBad {
				failed = st.Name
			}
		}
	}
	if err != nil {
		d.Lines = append(d.Lines, text(err.Error()))
	}
	if failed != "" {
		n.Body = text(fmt.Sprintf("Discovery stopped at the %s step. %s", failed, nothing))
	}
	n.Details = d
	return n
}

func validServerURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Scheme, "https") || strings.EqualFold(u.Scheme, "http")
}

// connectionLabel is a typed label: one line, at most 60 characters.
func connectionLabel(v string) string {
	v = strings.Join(strings.Fields(v), " ")
	if r := []rune(v); len(r) > 60 {
		v = string(r[:60])
	}
	return v
}

// connLabel is what a connection is called on every screen: the person's
// label, else its host (C-4: the account is a label, not an identifier).
// labelTaken reports whether a connection other than except already goes by
// label — the label it acts under, as the stamps show it: its own, or the
// server's host when it has none (owner's decision 24.09.2026, s-profile-11).
// Letter case does not tell two sources apart on a screen, so it does not
// here either. A mailbox's label keeps the same rule in the same namespace
// (carrel-mailbox.md M-3): its stamps stand beside the servers' in search.
func labelTaken(blob sealed.Blob, label, except string) bool {
	for _, c := range blob.Connections {
		if c.ID != except && strings.EqualFold(connLabel(c), label) {
			return true
		}
	}
	return false
}

func labelTakenText(label string) view.Text {
	return text("There is already a connection called “" + label + "”. Choose another label: it is how your sources are told apart.")
}

func connLabel(c sealed.Connection) string {
	if c.Label != "" {
		return c.Label
	}
	return serverHost(c.Server)
}

func serverHost(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return raw
}

// collectionLabel is the name a collection goes by: the server's, else its
// address name. Two collections of one connection under one name — Nextcloud
// allows it — each carry their address name in brackets, «Carrel S-15
// (carrel-s-15-2)»: it is unique within the connection, and this is the
// label every stamp, rail row and column is made of (C-10, owner's decision
// 24.09.2026).
func collectionLabel(c sealed.Collection) string {
	if c.Name == "" {
		return addressName(c.Path)
	}
	if c.Twin {
		return c.Name + " (" + addressName(c.Path) + ")"
	}
	return c.Name
}

// addressName is the last segment of a collection's path, decoded.
func addressName(path string) string {
	p := strings.TrimSuffix(path, "/")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		p = p[i+1:]
	}
	if d, err := url.PathUnescape(p); err == nil {
		return d
	}
	return p
}

// catalogColour is C-10: the colour the server sent, else one of eight muted
// tones derived from the address. An alpha channel is dropped; anything that
// is not a hex colour counts as not sent.
func catalogColour(c sealed.Collection) view.Colour {
	if v := hexColour(c.Colour); v != "" {
		return view.Colour(v)
	}
	return collectionColour(c.Path)
}

// hexColour is a hex colour in one spelling: lower case, alpha dropped.
// Anything else is empty — not a colour the server sent.
func hexColour(v string) string {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "#") || !isHex(v[1:]) {
		return ""
	}
	switch len(v) {
	case 4, 7:
		return strings.ToLower(v)
	case 9:
		return strings.ToLower(v[:7])
	}
	return ""
}

func isHex(s string) bool {
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return false
		}
	}
	return s != ""
}
