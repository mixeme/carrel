package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/discover"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// connectionForm is what the Add form shows back. The password is never
// echoed: a failed attempt asks for it again.
type connectionForm struct {
	Server, Login, CertFingerprint string
	ServerErr                      view.Text
}

func (s *Server) getSettingsConnections(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	f := connectionForm{Server: r.FormValue("server"), CertFingerprint: r.FormValue("certFingerprint")}
	s.writePage(w, r, http.StatusOK, s.connectionsPage(sess, f, nil))
}

// postSettingsConnections runs discovery (S-6) through the guard with the
// accepted fingerprint (F-13) and writes the connection with its collections
// into the catalog. A failed discovery saves nothing and says at which step it
// stopped, with the whole trace beside it (C-9.1) — screen state, not a 500.
func (s *Server) postSettingsConnections(w http.ResponseWriter, r *http.Request) {
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
		Server:          strings.TrimSpace(r.FormValue("server")),
		Login:           strings.TrimSpace(r.FormValue("login")),
		CertFingerprint: strings.TrimSpace(r.FormValue("certFingerprint")),
	}
	if !validServerURL(f.Server) {
		f.ServerErr = text("Enter the full address, starting with https:// or http://.")
		s.writePage(w, r, http.StatusOK, s.connectionsPage(sess, f, nil))
		return
	}
	password := r.FormValue("password")
	pin := dav.NormalizeFingerprint(f.CertFingerprint)

	res, trace, err := discover.Run(r.Context(), s.guard, f.Server, f.Login, password, pin)
	if err != nil || res == nil || trace == nil || !trace.OK {
		s.writePage(w, r, http.StatusOK, s.connectionsPage(sess, f, discoveryFailure(trace, err)))
		return
	}

	conn := sealed.Connection{
		Server:          res.BaseURL,
		Login:           f.Login,
		Password:        password,
		CertFingerprint: pin,
		CalendarHome:    res.CalendarHome,
		AddressBookHome: res.AddressBookHome,
	}
	if res.FileOnly {
		conn.FileRoot = res.BaseURL
	}
	cols := make([]sealed.Collection, 0, len(res.Collections))
	for _, c := range res.Collections {
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
	if _, err := s.users.AddConnection(sess.AccountID(), key, conn, cols); err != nil {
		http.Error(w, "could not save the connection", http.StatusInternalServerError)
		return
	}
	s.redirect(w, r, "/app/settings/connections")
}

func (s *Server) connectionsPage(sess *session.Session, f connectionForm, failure *view.Notice) view.ConnectionsPage {
	page := view.ConnectionsPage{
		Shell: s.buildShell(sess, "Connections"),
		Add: s.plainForm("/app/settings/connections", "POST",
			csrfField(sess),
			view.FormRow{Name: "server", Label: "Server URL", Kind: view.InputURL, Value: text(f.Server), Error: f.ServerErr, Required: true},
			view.FormRow{Name: "login", Label: "Login", Kind: view.InputText, Value: text(f.Login)},
			view.FormRow{Name: "password", Label: "Password", Kind: view.InputPassword},
			view.FormRow{Name: "certFingerprint", Label: "Certificate fingerprint (SHA-256)", Kind: view.InputText, Value: text(f.CertFingerprint)},
		),
	}
	if key, live := sess.DataKey(); live {
		if blob, ok := s.users.Catalog(sess.AccountID(), key); ok {
			for _, c := range blob.Connections {
				page.Accounts = append(page.Accounts, s.connectionCard(blob, c))
			}
		}
	}
	if failure != nil {
		page.Shell.Notices = append(page.Shell.Notices, *failure)
	}
	page.Shell.Notices = append(page.Shell.Notices, connectionWarnings(f.Server, f.CertFingerprint)...)
	return page
}

func (s *Server) connectionCard(blob sealed.Blob, c sealed.Connection) view.ConnectionCard {
	card := view.ConnectionCard{
		AccountID: c.ID,
		Title:     text(serverHost(c.Server)),
		Server:    text(c.Server),
		Login:     text(c.Login),
		Password: view.SecretField{
			Reveal: view.Action{ID: "connection.password", Label: "Show password", Href: s.route("/app/settings/unlock"), Method: "GET", Enabled: true},
		},
		State: view.CollectionOK,
	}
	for _, col := range blob.CollectionsOf(c.ID) {
		card.Collections = append(card.Collections, view.CollectionRow{
			Label:    text(collectionLabel(col)),
			Path:     text(col.Path),
			Colour:   catalogColour(col),
			Kinds:    col.Kinds,
			ReadOnly: col.ReadOnly,
		})
	}
	return card
}

// discoveryFailure names the last step that went wrong — steps run in order,
// and an earlier bad well-known probe is routine — and carries every step.
func discoveryFailure(trace *view.DiagnosticTrace, err error) *view.Notice {
	n := &view.Notice{
		Role:  view.NoticeAlert,
		Title: text("Could not connect"),
		Body:  text("Nothing was saved."),
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
		n.Body = text(fmt.Sprintf("Discovery stopped at the %s step. Nothing was saved.", failed))
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

func serverHost(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return raw
}

func collectionLabel(c sealed.Collection) string {
	if c.Name != "" {
		return c.Name
	}
	p := strings.TrimSuffix(c.Path, "/")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

// catalogColour is C-10: the colour the server sent, else one of eight muted
// tones derived from the address. An alpha channel is dropped; anything that
// is not a hex colour counts as not sent.
func catalogColour(c sealed.Collection) view.Colour {
	v := strings.TrimSpace(c.Colour)
	if strings.HasPrefix(v, "#") && isHex(v[1:]) {
		switch len(v) {
		case 4, 7:
			return view.Colour(strings.ToLower(v))
		case 9:
			return view.Colour(strings.ToLower(v[:7]))
		}
	}
	return collectionColour(c.Path)
}

func isHex(s string) bool {
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return false
		}
	}
	return s != ""
}
