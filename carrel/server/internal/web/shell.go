package web

import (
	"net/http"
	"net/url"
	"time"

	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

func (s *Server) buildShell(sess *session.Session, title string) view.Shell {
	sh := view.Shell{
		Title: text(title),
		Top: view.TopBar{
			Search: view.SearchBox{
				Action:  s.route("/app/search"),
				History: nil,
				Jump:    nil,
				Whole:   nil,
			},
			NewMenu: s.newMenu(sess),
			User:    nil,
			Refresh: view.Refresh{},
		},
		Rail: view.Rail{
			About: s.aboutAction(),
		},
	}
	if sess != nil {
		now := s.now()
		if until := sess.UnlockUntil(now); until != nil {
			sh.Top.Unlocked = ptrWhen(s.timeFor(sess).formatWhen(*until, false, now))
		}
		if history := sess.History(); len(history) > 0 {
			tf := s.timeFor(sess)
			for _, e := range history {
				sh.Top.Search.History = append(sh.Top.Search.History, view.SearchHistoryRow{
					Query:   text(e.Query),
					Outcome: text(historyOutcome(e)),
					When:    tf.formatWhen(e.At, true, now),
					Href:    s.route(searchHref(e.Query)),
				})
			}
		}
		sh.Top.Signed, sh.Top.User = s.signedIn(sess), s.userMenu(sess)
		if s.users.ConsumeEscrowNotice(sess.AccountID()) {
			sh.Notices = []view.Notice{{
				Role:  view.NoticeAlert,
				Title: text("Key escrow applies to your account"),
				Body:  text("An administrator who holds the escrow master password can decrypt the DAV credentials you save here, and through them your contacts and calendars. This is not backup."),
			}}
		}
	}
	return sh
}

// aboutAction is the rail's last line: the name, and the build behind it when
// there is one (C-13). A run from source has no version and no commit, and
// then the line is the name alone — an empty version shown as one would be a
// lie about what is running.
// signedIn is the first line of the account menu: who is signed in and on
// which instance. Several instances of Carrel look alike on the screen —
// home, work, the one on this laptop — and only their names tell them apart.
// Whether the instance is local goes with its name: the badge «local» it
// puts beside the addresses it hands out (from-typecase note 22).
func (s *Server) signedIn(sess *session.Session) view.SignedIn {
	in := view.SignedIn{Instance: text(s.users.Settings().InstanceName()), Local: s.instanceLocal()}
	snap := s.store.Snapshot()
	if u := snap.UserByID(sess.AccountID()); u != nil {
		in.Login = text(u.Login)
	}
	return in
}

// userMenu is the account menu of the top bar (U-6): the settings, the
// administration for an administrator, and the way out past the line.
// Theme and the keyboard sheet stand there too, in the frame, and are not
// here: they have no route and nothing on the server behind them — the
// client draws and remembers them itself (U-11).
func (s *Server) userMenu(sess *session.Session) []view.MenuItem {
	out := []view.MenuItem{{Action: view.Action{
		ID: "user.settings", Label: "Settings", Icon: "i-gear",
		Href: s.route("/app/settings/account"), Method: "GET", Enabled: true,
	}}}
	if s.users.IsAdmin(sess.AccountID()) {
		out = append(out, view.MenuItem{Action: view.Action{
			ID: "user.admin", Label: "Administration", Icon: "i-adm",
			Href: s.route("/admin/"), Method: "GET", Enabled: true,
		}})
	}
	// The line before the way out: everything above it opens something,
	// and this one ends the session.
	return append(out, view.MenuItem{Separator: true, Action: view.Action{
		ID: "user.logout", Label: "Sign out", Href: s.route("/logout"), Method: "POST", Enabled: true,
	}})
}

func (s *Server) aboutAction() view.Action {
	label := "Carrel"
	switch {
	case s.version != "" && s.commit != "":
		label += " " + s.version + " · " + s.commit
	case s.version != "":
		label += " " + s.version
	case s.commit != "":
		label += " · " + s.commit
	}
	return view.Action{
		ID: "about", Label: text(label), Href: s.route("/about"), Method: "GET", Enabled: true,
	}
}

func (s *Server) tz() *time.Location {
	loc, err := time.LoadLocation(s.cfg.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

func ptrWhen(w view.When) *view.When { return &w }

func (s *Server) plainForm(action, method string, rows ...view.FormRow) view.Form {
	return view.Form{
		Action: s.route(action),
		Method: method,
		Sets: []view.FieldSet{{
			Rows: rows,
		}},
	}
}

func csrfField(sess *session.Session) view.FormRow {
	if sess == nil {
		return view.FormRow{Name: "_csrf", Kind: view.InputHidden, Value: ""}
	}
	return view.FormRow{Name: "_csrf", Kind: view.InputHidden, Value: text(sess.CSRFToken())}
}

func csrfFormRow(token string) view.FormRow {
	return view.FormRow{Name: "_csrf", Kind: view.InputHidden, Value: text(token)}
}

func (s *Server) anonPlainForm(w http.ResponseWriter, r *http.Request, action, method string, rows ...view.FormRow) view.Form {
	token := s.issueAnonymousCSRF(w, r)
	all := append([]view.FormRow{csrfFormRow(token)}, rows...)
	return s.plainForm(action, method, all...)
}

// newMenu is the create menu of the top bar: one item per kind, each opening
// the quick sheet from the shell (U-6.16). The sheet itself decides where the
// record goes — the collection the last one was created in, or a question when
// there is none — so the menu does not need to know about collections.
func (s *Server) newMenu(sess *session.Session) []view.MenuItem {
	if sess == nil {
		return nil
	}
	kinds := []struct {
		segment string
		label   string
	}{
		{"contacts", "Contact"},
		{"calendar", "Event"},
		{"tasks", "Task"},
		{"notes", "Note"},
	}
	out := make([]view.MenuItem, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, view.MenuItem{Action: view.Action{
			ID:      "new." + k.segment,
			Label:   text(k.label),
			Href:    s.route("/app/" + k.segment + "/quick"),
			Method:  "GET",
			Enabled: true,
		}})
	}
	// A file and a letter create nothing in a collection of records, so they
	// stand last, past the line (U-6.4). The upload asks where on its sheet;
	// without mail on the instance the letter is not offered.
	out = append(out, view.MenuItem{Separator: true, Action: view.Action{
		ID: "new.upload", Label: "Upload file", Icon: "i-up", Href: s.postQuery("/app/files/picker", url.Values{"for": {"upload"}}), Method: "GET", Enabled: true,
	}})
	if s.canSendLetters(sess) {
		out = append(out, view.MenuItem{Action: view.Action{
			ID: "new.letter", Label: "Letter", Icon: "i-mail", Href: s.route("/app/mail/compose"), Method: "GET", Enabled: true,
		}})
	}
	return out
}

// focused puts the caret (FormRow.Focus, one per form) where the frames
// draw it when the form is shown: in the first field that came back with a
// reason (s-auth-13, s-contacts-18, s-profile-11, s-admin-16, s-admin-20),
// else in the field named name — or, when that one does not change (a login
// the invitation names), in the first after it that does. A form without
// that field and without a reason has no caret.
func focused(f view.Form, name string) view.Form {
	type at struct{ set, row int }
	var reason, named, next *at
	for i := range f.Sets {
		for j := range f.Sets[i].Rows {
			r := &f.Sets[i].Rows[j]
			r.Focus = false
			if r.Kind == view.InputHidden {
				continue
			}
			switch {
			case reason == nil && r.Error != "":
				reason = &at{i, j}
			case named == nil && r.Name == name:
				named = &at{i, j}
			}
			if named != nil && next == nil && !r.ReadOnly && r.Kind != view.InputAdd && r.Kind != view.InputRemove {
				next = &at{i, j}
			}
		}
	}
	for _, p := range []*at{reason, next} {
		if p != nil {
			f.Sets[p.set].Rows[p.row].Focus = true
			return f
		}
	}
	return f
}

// withSubmit gives a form the button that sends it, and any others after it.
// The words on a button are the server's (C-3.1): a client that has to guess
// "Submit" is guessing the product.
func withSubmit(f view.Form, id, label string, more ...view.Action) view.Form {
	f.Actions = append([]view.Action{{ID: id, Label: text(label), Href: f.Action, Method: f.Method, Enabled: true}}, more...)
	return f
}

// withPrimary is withSubmit for a form whose frame draws its button as the
// primary one (is-primary, C-6.2 Action.Primary): Sign in, Create account,
// Save. A button drawn plain or destructive is withSubmit's: the order of
// the buttons does not make one primary.
func withPrimary(f view.Form, id, label string, more ...view.Action) view.Form {
	f = withSubmit(f, id, label, more...)
	f.Actions[0].Primary = true
	return f
}
