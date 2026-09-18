package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

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
}

func (s *Server) feedForm(sess *session.Session, f feedForm) view.Form {
	form := s.plainForm("/app/settings/connections?op=feed.subscribe", "POST",
		csrfField(sess),
		view.FormRow{Name: "url", Label: "Feed address", Kind: view.InputURL, Value: text(f.URL), Error: f.URLErr, Required: true,
			Hint: "https://, webcal:// and a secret link from another Carrel are all understood. The feed is fetched by the Carrel server, not by your browser."},
		view.FormRow{Name: "name", Label: "Name here", Kind: view.InputText, Value: text(f.Name),
			Hint: "Taken from the feed when left empty. The name is yours and is never sent to the feed."},
		view.FormRow{Name: "login", Label: "Login, if the feed asks for one", Kind: view.InputText, Value: text(f.Login)},
		view.FormRow{Name: "password", Label: "Password", Kind: view.InputPassword},
	)
	form.Actions = []view.Action{
		{ID: "feed.check", Label: "Check the address", Href: s.postQuery("/app/settings/connections", url.Values{"op": {"feed.check"}}), Method: "POST", Enabled: true},
		{ID: "feed.subscribe", Label: "Subscribe", Href: s.postQuery("/app/settings/connections", url.Values{"op": {"feed.subscribe"}}), Method: "POST", Enabled: true},
	}
	return form
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

func (s *Server) readFeed(ctx context.Context, addr, login, password string) (publish.Parsed, publish.Feed, error) {
	return s.newFeedReader(addr, login, password).read(ctx)
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
	f := feedForm{URL: strings.TrimSpace(r.FormValue("url")), Name: strings.TrimSpace(r.FormValue("name")), Login: strings.TrimSpace(r.FormValue("login"))}
	password := r.FormValue("password")
	page := s.connectionsPage(sess, connectionForm{}, nil)
	if _, err := publish.FeedURL(f.URL); err != nil {
		f.URLErr = "Give the address of a .ics or .vcf file: https:// or webcal://."
		page.Add = s.feedForm(sess, f)
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
		page.Add = s.feedForm(sess, f)
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
	if _, err := s.users.Subscribe(sess.AccountID(), key, sealed.Subscription{URL: f.URL, Name: name, Login: f.Login, Password: password, Kinds: parsed.Kinds()}); err != nil {
		http.Error(w, "could not subscribe", http.StatusInternalServerError)
		return
	}
	s.redirect(w, r, "/app/settings/connections")
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
	parsed, got, err := s.readFeed(r.Context(), f.URL, f.Login, f.Password)
	if err == nil && len(parsed.Kinds()) > 0 {
		_ = s.users.EditFeed(sess.AccountID(), key, f.ID, "", "", parsed.Kinds())
	}
	page := s.connectionsPage(sess, connectionForm{}, nil)
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
	card := view.SubscriptionCard{Title: text(collectionLabel(col)), URL: text(f.URL), Colour: catalogColour(col), Kinds: f.Kinds}
	card.Checked, _ = readingOf(sess, tf, f.ID, sealed.FeedPath)
	q := func(op string) string {
		return s.postQuery("/app/settings/connections", url.Values{"op": {op}, "feed": {f.ID}})
	}
	card.Actions = []view.Action{
		{ID: "feed.recheck", Label: "Check", Href: q("feed.recheck"), Method: "POST", Enabled: true},
		{ID: "feed.edit", Label: "Edit", Href: s.postQuery("/app/collections/rename", url.Values{"a": {f.ID}, "c": {sealed.FeedPath}}), Method: "GET", Enabled: len(f.Kinds) > 0},
		{ID: "feed.unsubscribe", Label: "Unsubscribe", Href: q("feed.unsubscribe"), Method: "POST", Danger: true, Enabled: true},
	}
	return card
}
