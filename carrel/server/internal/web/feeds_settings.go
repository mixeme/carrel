package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/provider/file"
	"carrel.dev/carrel/server/internal/publish"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// The subscribe form sits on the connections page, beside the servers: a
// feed is another source of records (S-25.2). Checking the address is the
// same kind of trace as discovery — what the server answered, how much, how
// fast, what was inside — and it saves nothing. Subscribing reads once more
// and keeps the address, the name and the kinds the feed carries; never its
// records.

type feedForm struct {
	URL, Name, Login string
	URLErr           view.Text
	// Folder is a WebDAV folder somebody shares rather than a feed file.
	Folder bool
	// Colour is the label's colour chosen; empty — the one derived from the
	// address (s-profile-10).
	Colour string
	// Asks is the tick «This feed asks for a password»: the login and the
	// password are read only with it (Нс27).
	Asks bool
	// Sections are what a check found the feed carries, «Calendar · 118»;
	// empty before a check.
	Sections []string
}

// feedHint is what the panel says of the address (s-profile-10).
const feedHint = "https://, webcal:// and a secret link from another Carrel are all understood."

// feedReach is the panel's warning under its form (s-profile-10): a feed is
// read by the server, so what the server cannot reach it cannot read.
const feedReach = "The feed is fetched by the Carrel server, not by your browser. An address on your local network works only if the server can reach it; a public Carrel cannot, and it says so instead of returning an empty list."

func (s *Server) feedForm(sess *session.Session, f feedForm) *view.Form {
	q := func(op string) string {
		v := url.Values{"op": {op}}
		if f.Folder {
			v.Set("kind", "folder")
		}
		return s.postQuery("/app/settings/connections", v)
	}
	address := view.FormRow{Name: "url", Label: "Feed address", Kind: view.InputURL, Value: text(f.URL), Error: f.URLErr, Required: true, Mono: true,
		Hint: feedHint}
	if f.Folder {
		address.Label = "Folder address"
		address.Hint = "A WebDAV folder somebody shares: a secret link from another Carrel, or any WebDAV address. It is read here and never written, whatever the server would allow."
	}
	action := "/app/settings/connections?op=feed.subscribe"
	if f.Folder {
		action += "&kind=folder"
	}
	// The panel of s-profile-10: the address, the name, the label's colour,
	// whether the feed asks for a password — the login and the password
	// shown while it is ticked (FormRow.Changed) —, and what a check found
	// it carries (Нс27).
	rows := []view.FormRow{csrfField(sess), address,
		{Name: "name", Label: "Name here", Kind: view.InputText, Value: text(f.Name),
			Hint: "Taken from the feed; change it if you like — other people’s feed names are often longer than the rail."}}
	if !f.Folder {
		derived := string(collectionColour(f.URL))
		chosen := f.Colour
		if chosen == "" {
			chosen = derived
		}
		colour := paletteRow(chosen, "", "A feed sends no colour of its own: Carrel derives one, and hatches the strip so a feed is recognisable among your own collections.")
		colour.Label = "Label colour"
		if chosen == derived {
			colour.ValueAside = "derived from the address"
		}
		rows = append(rows, colour)
	}
	rows = append(rows,
		view.FormRow{Name: "asks", Label: "This feed asks for a password", Kind: view.InputCheck, Value: text(boolStr(f.Asks)),
			Hint: "The login and password are kept the way DAV credentials are: encrypted under your key. A secret link needs no password — the secret is already in the address."},
		view.FormRow{Name: "login", Label: "Login of the link", Kind: view.InputText, Value: text(f.Login), Hidden: !f.Asks, Changed: []string{"asks"}},
		view.FormRow{Name: "password", Label: "Password of the link", Kind: view.InputPassword, Hidden: !f.Asks, Changed: []string{"asks"}},
	)
	if len(f.Sections) > 0 {
		sections := view.FormRow{Name: "sections", Label: "Sections", Kind: view.InputSelect, Multiple: true, ReadOnly: true,
			Hint: "Not a choice: a feed is one file, and all three kinds of record live in it. What it carries decides where it lands, and that is visible before subscribing."}
		for _, sc := range f.Sections {
			sections.Options = append(sections.Options, view.Option{Value: sc, Label: text(sc), Selected: true})
		}
		rows = append(rows, sections)
	}
	form := s.plainForm(action, "POST", rows...)
	form = focused(form, "url")
	form.Actions = []view.Action{
		{ID: "feed.check", Label: "Check the address", Href: q("feed.check"), Method: "POST", Enabled: true},
		{ID: "feed.subscribe", Label: "Subscribe", Href: q("feed.subscribe"), Method: "POST", Primary: true, Enabled: true},
		{ID: "feed.cancel", Label: "Cancel", Href: s.route("/app/settings/connections"), Method: "GET", Enabled: true},
	}
	return &form
}

// feedPanel opens the panel of a subscription over the page (s-profile-10):
// its form, the page's message about Check, and under the form what a feed
// read by the server cannot reach (Нс27).
func (s *Server) feedPanel(page *view.ConnectionsPage, sess *session.Session, f feedForm) {
	page.Add, page.Note = s.feedForm(sess, f), feedPanelNote
	if !f.Folder {
		page.AddNote = feedReach
	}
}

// probeFolder lists a shared folder once: how many entries, how fast.
func (s *Server) probeFolder(ctx context.Context, addr, login, password string) (int, time.Duration, error) {
	client, err := dav.NewClient(s.guard, addr, login, password, "")
	if err != nil {
		return 0, 0, err
	}
	files, err := file.New(client)
	if err != nil {
		return 0, 0, err
	}
	u, _ := url.Parse(addr)
	start := time.Now()
	entries, err := files.List(ctx, u.Path)
	return len(entries), time.Since(start), err
}

// folderTrace is what a folder check found.
func folderTrace(addr string, n int, took time.Duration, err error) view.Notice {
	if err != nil {
		n := view.Notice{Role: view.NoticeAlert, Title: "The folder could not be read", Body: "Nothing was saved.",
			Details: &view.Diagnostic{Summary: text(addr), Lines: []view.Text{text("[list] PROPFIND · " + err.Error())}}}
		if dav.IsStatus(err, http.StatusUnauthorized) {
			n.Body = "The folder asks for a login and password, and the ones given do not fit."
		}
		return n
	}
	return view.Notice{Role: view.NoticeInfo, Title: "The folder answers",
		Body:    text(fmt.Sprintf("%s in it. It lands in Files, read-only.", plural(n, "entry"))),
		Details: &view.Diagnostic{Summary: text(addr), Lines: []view.Text{text(fmt.Sprintf("[list] PROPFIND → %d entries · %d ms", n, took.Milliseconds()))}}}
}

// postFolder checks or subscribes to a shared folder: its address and name,
// never its files.
func (s *Server) postFolder(w http.ResponseWriter, r *http.Request, sess *session.Session, key [32]byte, f feedForm, password string) {
	page := s.connectionsPage(sess, connectionForm{}, nil)
	// A Nextcloud or ownCloud share link is a page for people; its WebDAV is
	// public.php/webdav with the token as the login and the link's password,
	// if it has one, as the password.
	if share, token, ok := publish.ShareLink(f.URL); ok && f.Login == "" {
		f.URL, f.Login = share, token
		if f.Name == "" {
			if u, err := url.Parse(share); err == nil {
				f.Name = "Shared folder · " + u.Host
			}
		}
	}
	addr, err := publish.FolderURL(f.URL)
	if err != nil {
		f.URLErr = "Give the address of a WebDAV folder: https://."
		s.feedPanel(&page, sess, f)
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	n, took, err := s.probeFolder(r.Context(), addr, f.Login, password)
	if err != nil || r.FormValue("op") == "feed.check" {
		s.feedPanel(&page, sess, f)
		page.Shell.Notices = append(page.Shell.Notices, folderTrace(addr, n, took, err))
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	name := f.Name
	if name == "" {
		name = feedNameOf(strings.TrimSuffix(addr, "/"))
	}
	if _, err := s.users.Subscribe(sess.AccountID(), key, sealed.Subscription{URL: addr, Name: name, Login: f.Login, Password: password, Kinds: []view.Kind{view.KindFile}}); err != nil {
		http.Error(w, "could not subscribe", http.StatusInternalServerError)
		return
	}
	s.redirect(w, r, "/app/settings/connections")
}

var sectionTitles = map[view.Kind]string{view.KindContact: "Contacts", view.KindEvent: "Calendar", view.KindTask: "Tasks", view.KindNote: "Notes"}

// feedTrace is what a read found, step by step.
func feedTrace(addr string, got publish.Feed, parsed publish.Parsed, err error) view.Notice {
	var lines []view.Text
	if got.Status != 0 {
		lines = append(lines, text(fmt.Sprintf("[fetch] GET %s → %d · %d ms · %s", addr, got.Status, got.Took.Milliseconds(), formatSize(got.Size))))
	}
	if got.Type != "" {
		lines = append(lines, text("[type] "+got.Type))
	}
	if err != nil {
		lines = append(lines, text("[error] "+err.Error()))
		n := view.Notice{Role: view.NoticeAlert, Title: "The feed could not be read", Body: "Nothing was saved.", Details: &view.Diagnostic{Summary: text(addr), Lines: lines}}
		switch {
		case got.Status == http.StatusUnauthorized:
			n.Body = "The feed asks for a password, and the one given does not fit."
		case errors.Is(err, publish.ErrNotAFeed):
			n.Body = "The address answered, but not with a calendar or an address book."
		case got.Status == 0:
			n.Body = "The Carrel server could not reach the address. An address on your local network works only if the server can reach it."
		}
		return n
	}
	var parts []string
	for _, k := range parsed.Kinds() {
		parts = append(parts, fmt.Sprintf("%s · %d", sectionTitles[k], len(parsed.Records[k])))
	}
	lines = append(lines, text("[parse] "+strings.Join(parts, ", ")))
	if parsed.Name != "" {
		lines = append(lines, text("[name] "+parsed.Name))
	}
	return view.Notice{
		Role: view.NoticeInfo, Title: "The feed answers",
		Body:    text("It lands in " + strings.Join(parts, ", ") + ": what the feed carries decides where it lands."),
		Details: &view.Diagnostic{Summary: text(addr), Lines: lines},
	}
}

// readFeed reads an address that is not a subscription yet: nothing of it
// is noted.
func (s *Server) readFeed(ctx context.Context, addr, login, password string) (publish.Parsed, publish.Feed, error) {
	return s.newFeedReader(nil, sealed.Connection{Server: addr, Login: login, Password: password}).read(ctx)
}

// postFeed checks the address (op=feed.check) or subscribes to it
// (op=feed.subscribe). A feed that does not answer with records is not
// subscribed to: it would look like a feed with nothing in it.
func (s *Server) postFeed(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	key, live := sess.DataKey()
	if !live {
		s.notFound(w, r)
		return
	}
	f := feedForm{URL: strings.TrimSpace(r.FormValue("url")), Name: strings.TrimSpace(r.FormValue("name")), Login: strings.TrimSpace(r.FormValue("login")),
		Folder: r.FormValue("kind") == "folder", Asks: checkbox(r, "asks") == "on"}
	if c := strings.ToLower(strings.TrimSpace(r.FormValue("colour"))); inPalette(c) {
		f.Colour = c
	}
	password := r.FormValue("password")
	switch {
	case r.Form.Has("asks") && !f.Asks:
		// The tick off: the login and the password are not read.
		f.Login, password = "", ""
	case !r.Form.Has("asks") && f.Login != "":
		// A post without the tick that names a login (an old client):
		// read as given.
		f.Asks = true
	}
	if f.Folder {
		s.postFolder(w, r, sess, key, f, password)
		return
	}
	page := s.connectionsPage(sess, connectionForm{}, nil)
	if _, err := publish.FeedURL(f.URL); err != nil {
		f.URLErr = "Give the address of a .ics or .vcf file: https:// or webcal://."
		s.feedPanel(&page, sess, f)
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	parsed, got, err := s.readFeed(r.Context(), f.URL, f.Login, password)
	if err == nil && len(parsed.Kinds()) == 0 {
		err = errors.New("the feed holds no records")
	}
	notice := feedTrace(f.URL, got, parsed, err)
	if err != nil || r.FormValue("op") == "feed.check" {
		if f.Name == "" {
			f.Name = parsed.Name
		}
		// What the feed carries, before subscribing; a feed that asked for
		// a password ticks the tick, so its fields are there.
		for _, k := range parsed.Kinds() {
			f.Sections = append(f.Sections, fmt.Sprintf("%s · %d", sectionTitles[k], len(parsed.Records[k])))
		}
		if got.Status == http.StatusUnauthorized {
			f.Asks = true
		}
		s.feedPanel(&page, sess, f)
		page.Shell.Notices = append(page.Shell.Notices, notice)
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	name := f.Name
	if name == "" {
		name = parsed.Name
	}
	if name == "" {
		name = feedNameOf(f.URL)
	}
	colour := f.Colour
	if colour == "" {
		colour = string(collectionColour(f.URL))
	}
	if _, err := s.users.Subscribe(sess.AccountID(), key, sealed.Subscription{URL: f.URL, Name: name, Colour: colour, Login: f.Login, Password: password, Kinds: parsed.Kinds()}); err != nil {
		http.Error(w, "could not subscribe", http.StatusInternalServerError)
		return
	}
	s.redirect(w, r, "/app/settings/connections")
}

// looksLikeFeed reports whether an address names a feed file rather than a
// folder: webcal://, or a .ics or .vcf at the end of the path.
func looksLikeFeed(addr string) bool {
	u, err := url.Parse(addr)
	if err != nil {
		return false
	}
	p := strings.ToLower(u.Path)
	return strings.HasPrefix(strings.ToLower(u.Scheme), "webcal") || strings.HasSuffix(p, ".ics") || strings.HasSuffix(p, ".vcf")
}

// feedNameOf is a name from the address when the feed gives none: the file
// name without its extension.
func feedNameOf(addr string) string {
	u, err := url.Parse(addr)
	if err != nil {
		return "Feed"
	}
	name := u.Path[strings.LastIndex(u.Path, "/")+1:]
	if i := strings.LastIndex(name, "."); i > 0 {
		name = name[:i]
	}
	if name == "" {
		return u.Host
	}
	return name
}

// recheckFeed reads a subscription again and says what it answered; the
// kinds it carries now are kept.
func (s *Server) recheckFeed(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	key, _ := sess.DataKey()
	f, ok := s.catalogOf(sess).Subscription(r.FormValue("feed"))
	if !ok {
		s.notFound(w, r)
		return
	}
	page := s.connectionsPage(sess, connectionForm{}, nil)
	if f.FeedCollection().Has(view.KindFile) {
		n, took, err := s.probeFolder(r.Context(), f.URL, f.Login, f.Password)
		page.Shell.Notices = append(page.Shell.Notices, folderTrace(f.URL, n, took, err))
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	parsed, got, err := s.newFeedReader(sess, sealed.Connection{ID: f.ID, Server: f.URL, Login: f.Login, Password: f.Password}).read(r.Context())
	if err == nil && len(parsed.Kinds()) > 0 {
		_ = s.users.EditFeed(sess.AccountID(), key, f.ID, "", "", parsed.Kinds())
	}
	page = s.connectionsPage(sess, connectionForm{}, nil)
	page.Shell.Notices = append(page.Shell.Notices, feedTrace(f.URL, got, parsed, err))
	s.writePage(w, r, http.StatusOK, page)
}

// unsubscribe removes the subscription, not anybody's data.
func (s *Server) unsubscribe(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	key, _ := sess.DataKey()
	err := s.users.Unsubscribe(sess.AccountID(), key, r.FormValue("feed"))
	if errors.Is(err, user.ErrNotFound) {
		s.notFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "could not unsubscribe", http.StatusInternalServerError)
		return
	}
	s.redirect(w, r, "/app/settings/connections")
}

func (s *Server) subscriptionCard(sess *session.Session, tf timeFmt, f sealed.Subscription) view.SubscriptionCard {
	col := f.FeedCollection()
	card := view.SubscriptionCard{ID: f.ID, Title: text(collectionLabel(col)), URL: text(f.URL), Colour: catalogColour(col), Kinds: f.Kinds,
		State: view.CollectionOK, Lands: []view.FeedKindRow{}}
	card.Checked, card.Changed = readingOf(sess, tf, f.ID, sealed.FeedPath)
	r, read := sess.ReadingOf(f.ID, sealed.FeedPath)
	if read && r.Fetched {
		card.Size, card.Took = text(roughSize(r.Size)), text(tookText(r.Took))
	}
	q := func(op string) string {
		return s.postQuery("/app/settings/connections", url.Values{"op": {op}, "feed": {f.ID}})
	}
	card.Actions = []view.Action{
		{ID: "feed.recheck", Label: "Check", Href: q("feed.recheck"), Method: "POST", Enabled: true},
		{ID: "feed.edit", Label: "Edit", Href: s.postQuery("/app/collections/rename", url.Values{"a": {f.ID}, "c": {sealed.FeedPath}}), Method: "GET", Enabled: len(f.Kinds) > 0},
		{ID: "feed.unsubscribe", Label: "Unsubscribe", Href: q("feed.unsubscribe"), Method: "POST", Danger: true, Enabled: true},
	}
	// The address line says whether a password is saved for the link, and
	// a saved one is shown behind the window a connection's is (Нс25).
	card.Access = "no password"
	if f.Password != "" {
		card.Access = "password saved"
		reveal := view.Action{ID: "connection.password", Label: "Show password", Href: s.route("/app/settings/unlock?" + url.Values{"for": {f.ID}}.Encode()), Method: "GET", Enabled: true}
		if sess.UnlockUntil(s.now()) != nil {
			reveal.Href = s.postQuery("/app/settings/connections", url.Values{"reveal": {f.ID}})
		}
		card.Password = &view.SecretField{Reveal: reveal}
	}
	// Lands in: a row a kind of the feed, in the order of the sections
	// (Нс26).
	blob := s.catalogOf(sess)
	for _, k := range []view.Kind{view.KindEvent, view.KindTask, view.KindNote, view.KindContact} {
		if !col.Has(k) {
			continue
		}
		seg := segmentOf(k)
		row := view.FeedKindRow{Kind: k, Label: text(kindTitle(seg)), Tick: view.TickOff,
			Objects: collectionCount(sess, f.ID, col, recordKinds[seg]),
			Toggle: view.Action{ID: "feed.tick", Label: text(kindTitle(seg)), Method: "POST", Enabled: true,
				Href: s.postQuery("/app/settings/connections", url.Values{"op": {"tick"}, "account": {f.ID}, "path": {sealed.FeedPath}, "kind": {string(k)}})}}
		if isTicked(blob, k, f.ID, sealed.FeedPath) {
			row.Tick = view.TickOn
		}
		card.Lands = append(card.Lands, row)
	}
	// The last fetch was refused (Нс25, s-profile-1): an alarm with what it
	// means, Check again; the time it was last read stays.
	if read && r.Refused.After(r.At) {
		card.State = view.CollectionUnavailable
		card.Actions[0].Label = "Check again"
		card.Note = text(r.Reason)
		if r.Unauthorized {
			card.Note = "The feed asks for a password, and none is saved for it."
			if f.Password != "" {
				card.Note = "The feed asks for a password and the saved one no longer fits. A feed that stopped answering must not look like a feed with nothing in it."
			}
		}
	}
	return card
}

// roughSize is a size as the line of a subscription card writes it
// (s-profile-1): bytes under a kilobyte, whole kilobytes under a megabyte,
// then megabytes with one decimal — «34 KB».
func roughSize(n int64) string {
	switch {
	case n < 1024:
		return strconv.FormatInt(n, 10) + " B"
	case n < 1024*1024:
		return strconv.FormatInt((n+512)/1024, 10) + " KB"
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
}

// tookText is how long a read took, as the same line writes it: tenths of a
// second — «0.4 s» —, milliseconds under a tenth.
func tookText(d time.Duration) string {
	if d < 100*time.Millisecond {
		return strconv.FormatInt(d.Milliseconds(), 10) + " ms"
	}
	return fmt.Sprintf("%.1f s", d.Seconds())
}

// feedPanelNote is the message of the page while the panel of a
// subscription is open (s-profile-10): a feed has nothing to synchronise,
// and Check — the card's button, in bold — asks whether it still replies.
var feedPanelNote = join("A feed is read again on every visit to the section, so there is nothing here to synchronise and nothing to refresh. ",
	strongRun("Check"), " asks whether the address still replies.")
