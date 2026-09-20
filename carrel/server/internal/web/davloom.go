package web

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// /app/settings/davloom (S-27.2): the person's proxy devices. A device gets
// a password of its own, issued once after the unlock sheet (S-10.7) and
// shown once; what it sees is chosen when it is made and changed later
// without a new password; revoking deletes its key wrapper.

// deviceKinds are the kinds a device may see, in rail order, with what each
// means for a client.
var deviceKinds = []struct {
	kind  view.Kind
	label string
}{
	{view.KindEvent, "Calendars"},
	{view.KindContact, "Address books"},
	{view.KindTask, "Tasks · VTODO — not every client understands them"},
	{view.KindNote, "Notes · VJOURNAL — a different subset of clients"},
	{view.KindFile, "WebDAV folders · mount as a network drive"},
}

// davloomAddress is the address a client is given (plan S-27): the public
// origin's /dav/ when there is one, else the port on this host.
func (s *Server) davloomAddress() string {
	if o := strings.TrimSuffix(s.cfg.PublicURL, "/"); o != "" {
		return o + "/dav/"
	}
	return "http://127.0.0.1:" + strconv.Itoa(s.cfg.DAVloomPort) + "/"
}

type issuedDevice struct {
	Name, Password string
}

func (s *Server) getSettingsDavloom(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	var issued *issuedDevice
	if data := sess.Draft(session.DraftIssuedDevice); data != nil {
		name, pw, _ := strings.Cut(string(data), "\x00")
		issued = &issuedDevice{Name: name, Password: pw}
		sess.SetDraft(session.DraftIssuedDevice, nil) // shown once
	}
	s.writePage(w, r, http.StatusOK, s.davloomPage(r, sess, issued, nil, ""))
}

// postSettingsDavloom issues (op=issue), changes a scope (op=scope&id=) or
// revokes (op=revoke&id=).
func (s *Server) postSettingsDavloom(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	key, live := sess.DataKey()
	if !live {
		s.notFound(w, r)
		return
	}
	id := r.FormValue("id")
	switch r.FormValue("op") {
	case "revoke":
		if err := s.users.RevokeDevice(sess.AccountID(), id); err != nil {
			s.notFound(w, r)
			return
		}
	case "scope":
		if err := s.users.SetDeviceScope(sess.AccountID(), key, id, scopeFromForm(r)); err != nil {
			s.writePage(w, r, http.StatusOK, s.davloomPage(r, sess, nil, err, id))
			return
		}
	case "issue":
		if sess.UnlockUntil(time.Now()) == nil {
			s.writePage(w, r, http.StatusOK, s.davloomPage(r, sess, nil, errors.New("Confirm your password first: Create password asks for it."), ""))
			return
		}
		name := r.FormValue("name")
		_, password, err := s.users.IssueDevice(r.Context(), sess.AccountID(), key, name, scopeFromForm(r))
		if err != nil {
			s.writePage(w, r, http.StatusOK, s.davloomPage(r, sess, nil, err, ""))
			return
		}
		sess.SetDraft(session.DraftIssuedDevice, []byte(strings.TrimSpace(name)+"\x00"+password))
	default:
		s.notFound(w, r)
		return
	}
	s.redirect(w, r, "/app/settings/davloom")
}

// scopeFromForm reads the ticked kinds (kind.{kind}) and collections
// (col.{account}|{path}).
func scopeFromForm(r *http.Request) user.DeviceScope {
	_ = r.ParseForm()
	var sc user.DeviceScope
	on := func(v string) bool { return v == "on" || v == "true" }
	for _, k := range deviceKinds {
		if on(r.PostForm.Get("kind." + string(k.kind))) {
			sc.Kinds = append(sc.Kinds, k.kind)
		}
	}
	for name, vals := range r.PostForm {
		rest, ok := strings.CutPrefix(name, "col.")
		if !ok || len(vals) == 0 || !on(vals[0]) {
			continue
		}
		if account, path, ok := strings.Cut(rest, "|"); ok {
			sc.Cols = append(sc.Cols, store.DeviceScope{AccountID: account, Path: path})
		}
	}
	return sc
}

func (s *Server) davloomPage(r *http.Request, sess *session.Session, issued *issuedDevice, formErr error, editing string) view.DavloomPage {
	page := view.DavloomPage{
		Shell:   s.withRail(s.buildShell(sess, "DAV proxy"), s.settingsRail(sess, "/app/settings/davloom")),
		Enabled: s.cfg.DAVloomPort != 0,
		Address: text(s.davloomAddress()),
		Note: "Changing your Carrel password does not disconnect devices: each has its own key wrapper. " +
			"To disconnect one, revoke it here — that is the only thing that stops it.",
	}
	if !page.Enabled {
		page.Note = "davloom is off on this instance: the administrator turns it on with CARREL_DAVLOOM_PORT. " + page.Note
	}
	blob := s.catalogOf(sess)
	tf, now := s.timeFor(sess), s.now()
	devices := s.users.Devices(sess.AccountID())
	var edited *store.ProxyDevice
	if editing == "" {
		editing = r.URL.Query().Get("edit")
	}
	for i := range devices {
		d := devices[i]
		row := view.ProxyDevice{Name: text(d.Name), Created: tf.formatWhen(d.Created, false, now)}
		if seen := s.users.DeviceLastSeen(d.ID); !seen.IsZero() {
			row.LastSeen = tf.formatWhen(seen, true, now)
		}
		for _, k := range d.ScopeKinds {
			row.Scope = append(row.Scope, view.Kind(k))
		}
		row.Actions = []view.Action{
			{ID: "davloom.scope", Label: text(plural(len(d.ScopeCols), "collection") + " · Change"), Method: "GET", Enabled: true,
				Href: s.postQuery("/app/settings/davloom", url.Values{"edit": {d.ID}})},
			{ID: "davloom.revoke", Label: "Revoke", Method: "POST", Enabled: true, Danger: true,
				Href: s.postQuery("/app/settings/davloom", url.Values{"op": {"revoke"}, "id": {d.ID}})},
		}
		page.Devices = append(page.Devices, row)
		if d.ID == editing {
			edited = &devices[i]
		}
	}
	page.Add = s.deviceForm(r, sess, blob, issued, edited, formErr)
	return page
}

// deviceForm is Add device, the scope of one device being changed, or —
// right after an issue — the password, shown this once.
func (s *Server) deviceForm(r *http.Request, sess *session.Session, blob sealed.Blob, issued *issuedDevice, edited *store.ProxyDevice, formErr error) view.Form {
	form := view.Form{Action: s.route("/app/settings/davloom"), Method: "POST"}
	if issued != nil {
		form.Sets = []view.FieldSet{{Legend: text(issued.Name + " · password created"), Rows: []view.FormRow{
			{Name: "password", Label: "Device password", Kind: view.InputText, Value: text(issued.Password), ReadOnly: true,
				Hint:  "Shown once and stored nowhere. Close this page and you will not see it again; lose it and you revoke this device and make another.",
				Extra: &view.Action{ID: "davloom.copy", Label: "Copy", Method: "GET", Enabled: true}},
			{Name: "address", Label: "Address", Kind: view.InputText, Value: text(s.davloomAddress()), ReadOnly: true},
			{Name: "login", Label: "User name", Kind: view.InputText, Value: text(sess.Login()), ReadOnly: true,
				Hint: "Your Carrel login; the password is the device's, not yours."},
		}}}
		form.Actions = []view.Action{{ID: "davloom.done", Label: "Done", Href: s.route("/app/settings/davloom"), Method: "GET", Enabled: true}}
		return form
	}

	kinds := map[view.Kind]bool{}
	cols := map[string]bool{}
	if edited != nil {
		for _, k := range edited.ScopeKinds {
			kinds[view.Kind(k)] = true
		}
		for _, c := range edited.ScopeCols {
			cols[c.AccountID+"|"+c.Path] = true
		}
	}
	posted := r.Method == http.MethodPost && formErr != nil
	on := func(name string, def bool) view.Text {
		if posted {
			v := r.PostForm.Get(name)
			return text(boolStr(v == "on" || v == "true"))
		}
		return text(boolStr(def))
	}
	rows := []view.FormRow{csrfField(sess)}
	legend := view.Text("Add device")
	if edited != nil {
		legend = text("What «" + edited.Name + "» sees — changed without a new password")
		rows = append(rows, view.FormRow{Name: "op", Kind: view.InputHidden, Value: "scope"}, view.FormRow{Name: "id", Kind: view.InputHidden, Value: text(edited.ID)})
	} else {
		name := ""
		if posted {
			name = r.PostForm.Get("name")
		}
		rows = append(rows, view.FormRow{Name: "op", Kind: view.InputHidden, Value: "issue"},
			view.FormRow{Name: "name", Label: "Name", Kind: view.InputText, Value: text(name), Required: true,
				Hint: "For you: this is how you will know what you are revoking later."})
	}
	for _, k := range deviceKinds {
		rows = append(rows, view.FormRow{Name: "kind." + string(k.kind), Label: text(k.label), Kind: view.InputCheck,
			Value: on("kind."+string(k.kind), edited == nil || kinds[k.kind])})
	}
	for _, conn := range blob.Connections {
		if conn.Feed {
			continue
		}
		for _, col := range blob.CollectionsOf(conn.ID) {
			if col.Gone {
				continue
			}
			name := "col." + conn.ID + "|" + col.Path
			rows = append(rows, view.FormRow{Name: name, Label: view.Text(stamp(connLabel(conn), collectionLabel(col))), Kind: view.InputCheck,
				Value: on(name, edited == nil || cols[conn.ID+"|"+col.Path])})
		}
	}
	if formErr != nil {
		rows[len(rows)-1].Error = text(formErr.Error())
		form.Errors = []view.FieldError{{Name: "scope", Message: text(formErr.Error())}}
	}
	form.Sets = []view.FieldSet{{Legend: legend, Rows: rows}}
	switch {
	case edited != nil:
		form.Actions = []view.Action{{ID: "davloom.scope.save", Label: "Save", Method: "POST", Enabled: true},
			{ID: "davloom.cancel", Label: "Cancel", Href: s.route("/app/settings/davloom"), Method: "GET", Enabled: true}}
	case sess.UnlockUntil(time.Now()) == nil:
		// The fourth place of the one unlock sheet (S-10.7): a password that
		// opens the data key is handed out only after the person's own.
		form.Actions = []view.Action{{ID: "davloom.issue", Label: "Create password…", Href: s.route("/app/settings/unlock"), Method: "GET", Enabled: true}}
	default:
		form.Actions = []view.Action{{ID: "davloom.issue", Label: "Create password", Method: "POST", Enabled: true}}
	}
	return form
}
