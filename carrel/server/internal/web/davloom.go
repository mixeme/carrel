package web

import (
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// /app/settings/davloom (S-27.2): the person's proxy devices. A device gets
// a password of its own, issued once after the unlock sheet (S-10.7) and
// shown once; what it sees is chosen when it is made and changed later
// without a new password; revoking deletes its key wrapper.

// deviceKinds are the kinds a device may see, in rail order, with what each
// means for a client — its name, and the muted words after it
// (s-davloom-2).
var deviceKinds = []struct {
	kind         view.Kind
	label, aside string
}{
	{view.KindEvent, "Calendars", ""},
	{view.KindContact, "Address books", ""},
	{view.KindTask, "Tasks", "· VTODO — not every client understands them"},
	{view.KindNote, "Notes", "· VJOURNAL — a different subset of clients"},
	{view.KindFile, "WebDAV folders", "· mount as a network drive"},
}

// davloomAddress is the address a client is given (plan S-27): the public
// origin's /dav/ when there is one, else the port on this host.
func (s *Server) davloomAddress() string {
	if o := strings.TrimSuffix(s.cfg.PublicURL, "/"); o != "" {
		return o + "/dav/"
	}
	return "http://127.0.0.1:" + strconv.Itoa(s.cfg.DAVloomPort) + "/"
}

// localAddress reports whether an address names this machine or a private
// network: loopback, a private or link-local IP, localhost, .local, .lan,
// .home.arpa.
func localAddress(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if ip, err := netip.ParseAddr(host); err == nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
	}
	return host == "localhost" || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".lan") ||
		strings.HasSuffix(host, ".home.arpa") || strings.HasSuffix(host, ".localhost")
}

type issuedDevice struct {
	Name, Password, ID string
}

func (s *Server) getSettingsDavloom(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	var issued *issuedDevice
	if data := sess.Draft(session.DraftIssuedDevice); data != nil {
		parts := strings.SplitN(string(data), "\x00", 3)
		for len(parts) < 3 {
			parts = append(parts, "")
		}
		issued = &issuedDevice{Name: parts[0], Password: parts[1], ID: parts[2]}
		sess.SetDraft(session.DraftIssuedDevice, nil) // shown once
	}
	s.writePage(w, r, http.StatusOK, s.davloomPage(r, sess, issued, nil))
}

// postSettingsDavloom issues (op=issue), makes a device read only or read
// and write (op=read, op=write &device=) or revokes it (op=revoke&device=).
// What a device sees is changed in its own sheet (davloomScope).
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
	id := r.FormValue("device")
	switch op := r.FormValue("op"); op {
	case "revoke":
		if err := s.users.RevokeDevice(sess.AccountID(), id); err != nil {
			s.notFound(w, r)
			return
		}
	case "issue":
		if sess.UnlockUntil(s.now()) == nil {
			// The person's own password first (S-10.7): the password's
			// sheet over the form, what was typed held (s-davloom-5,
			// Typecase's note 86); the sheet creates the password.
			_ = r.ParseForm()
			holdDevice(sess, r.PostForm)
			http.Redirect(w, r, s.unlockHref(unlockDavloom, "/app/settings/davloom?add=1&held=1"), http.StatusSeeOther)
			return
		}
		name := r.FormValue("name")
		id, password, err := s.users.IssueDevice(r.Context(), sess.AccountID(), key, name, scopeFromForm(r))
		if err != nil {
			s.writePage(w, r, http.StatusOK, s.davloomPage(r, sess, nil, err))
			return
		}
		sess.SetDraft(session.DraftIssuedDevice, []byte(strings.TrimSpace(name)+"\x00"+password+"\x00"+id))
	case "read", "write":
		// Read only or read and write, in the device's row and without a new
		// password (owner's decision 24.09.2026, s-davloom-1): buttons of the
		// page's one form (Typecase's note 102).
		if err := s.users.SetDeviceAccess(sess.AccountID(), id, op == "read"); err != nil {
			s.notFound(w, r)
			return
		}
	default:
		s.notFound(w, r)
		return
	}
	s.redirect(w, r, "/app/settings/davloom")
}

// accountDeleteNote is what deleting the account takes and what it leaves,
// beside Delete account and in its sheet (account-1, -15; the owner,
// 19.10.2026: «Carrel never touches it» was not true).
const accountDeleteNote = "Deletes the account and the saved connections. Data on the DAV servers stays."

// scopeFromForm reads the ticked kinds (kind.{kind}) and collections
// (col.{account}|{path}).
func scopeFromForm(r *http.Request) user.DeviceScope {
	_ = r.ParseForm()
	return deviceScopeOf(r.PostForm)
}

// deviceScopeOf is the scope of the values of Add device, as sent or as held.
func deviceScopeOf(form url.Values) user.DeviceScope {
	sc := user.DeviceScope{ReadOnly: form.Get("access") == "read"}
	on := func(v string) bool { return v == "on" || v == "true" }
	for _, k := range deviceKinds {
		if on(form.Get("kind." + string(k.kind))) {
			sc.Kinds = append(sc.Kinds, k.kind)
		}
	}
	for name, vals := range form {
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

func (s *Server) davloomPage(r *http.Request, sess *session.Session, issued *issuedDevice, formErr error) view.DavloomPage {
	page := view.DavloomPage{
		// The page is named as the rail names it (s-davloom-1, Нс46).
		Shell:    s.settingsShell(sess, "davloom", "/app/settings/davloom", "/app/settings/davloom"),
		Subtitle: line("One address for every device instead of an account for every server"),
		Enabled:  s.cfg.DAVloomPort != 0,
		Address:  text(s.davloomAddress()),
		// How the address goes into a client, and whose password (s-davloom-1).
		AddressHint: "In DAVx5 choose “Login with URL and user name”: the user name is your Carrel one, the password is the device’s, not yours.",
		// «not» in bold (s-davloom-1).
		Note: join("Changing your Carrel password does ", strongRun("not"), " disconnect devices: each has its own key wrapper. "+
			"To disconnect one, revoke it here — that is the only thing that stops it."),
		// The tree is a sheet of its own (s-davloom-9; owner's decision 19.10).
		Sees: view.Action{ID: "davloom.tree", Label: "What the client sees", Href: s.route("/app/settings/davloom/tree"), Method: "GET", Enabled: true, Target: render.TargetSheet},
	}
	// A local address does not reach a phone away from home, and the line
	// under the title says so where the address is given (s-davloom-1).
	if page.Enabled && localAddress(s.davloomAddress()) {
		page.Subtitle = join(page.Subtitle, " · ", view.Run{Text: "a phone on mobile data will not reach a local address", Mark: view.RunDanger})
	}
	// Add device in the head (s-davloom-1, Нс47): the dialog is a state of
	// the page of its own, not a form on every answer.
	add := view.Action{ID: "davloom.add", Label: "Add device", Icon: "i-plus", Method: "GET", Primary: true, Enabled: true,
		Href: s.postQuery("/app/settings/davloom", url.Values{"add": {"1"}})}
	if !page.Enabled {
		// Off (s-davloom-8): the badge says so, the devices stay and can be
		// revoked, none is added.
		page.Subtitle = line("No device connects until it is started.")
		add.Enabled, add.Href, add.Reason = false, "", "davloom is off on this instance."
	}
	page.Actions = []view.Action{add}
	page.Acts = s.plainForm("/app/settings/davloom", "POST", csrfField(sess))
	blob := s.catalogOf(sess)
	tf := s.timeFor(sess)
	devices := s.users.Devices(sess.AccountID())
	if len(devices) == 0 {
		page.Empty = view.Runs{strongRun("No devices yet.")} // s-davloom-6
	}
	for i := range devices {
		d := devices[i]
		row := view.ProxyDevice{Name: text(d.Name), Created: view.When(tf.numDate(d.Created)), ReadOnly: d.ReadOnly}
		if seen := s.users.DeviceLastSeen(d.ID); !seen.IsZero() {
			row.LastSeen = tf.numStamp(seen)
		}
		for _, k := range d.ScopeKinds {
			row.Scope = append(row.Scope, view.Kind(k))
		}
		// The marks of what it sees, each colour once, in the catalog's
		// order (s-davloom-1, Нс50).
		in := map[string]bool{}
		for _, c := range d.ScopeCols {
			in[c.AccountID+"|"+c.Path] = true
		}
		seen := map[view.Colour]bool{}
		for _, col := range blob.Collections {
			if colour := catalogColour(col); in[col.AccountID+"|"+col.Path] && !col.Gone && colour != "" && !seen[colour] {
				seen[colour] = true
				row.Marks = append(row.Marks, colour)
			}
		}
		row.Actions = []view.Action{
			// What it sees, its sheet (s-davloom-7).
			{ID: "davloom.scope", Label: text(plural(len(d.ScopeCols), "collection") + " · Change"), Method: "GET", Enabled: true,
				Href: s.route("/app/settings/davloom/scope?" + url.Values{"device": {d.ID}}.Encode()), Target: render.TargetSheet},
			// The device's right, a switch of two in its row (s-davloom-1).
			{ID: "davloom.access.read", Label: "read only", Method: "POST", Enabled: true, On: d.ReadOnly,
				Href: s.postQuery("/app/settings/davloom", url.Values{"op": {"read"}, "device": {d.ID}})},
			{ID: "davloom.access.write", Label: "read and write", Method: "POST", Enabled: true, On: !d.ReadOnly,
				Href: s.postQuery("/app/settings/davloom", url.Values{"op": {"write"}, "device": {d.ID}})},
			{ID: "davloom.revoke", Label: "Revoke", Method: "POST", Enabled: true, Danger: true,
				Href: s.postQuery("/app/settings/davloom", url.Values{"op": {"revoke"}, "device": {d.ID}})},
		}
		page.Devices = append(page.Devices, row)
	}
	adding := r.URL.Query().Get("add") != "" || r.Method == http.MethodPost && r.FormValue("op") == "issue" && formErr != nil
	switch {
	case issued != nil:
		page.Dialog = s.issuedDialog(sess, issued)
	case adding && page.Enabled:
		page.Dialog = s.deviceDialog(r, sess, blob, formErr)
	}
	return page
}

// clientTree is «What the client sees» (s-davloom-9, Нс51): the synthetic
// levels of davloom (S-27.3) — the root, the principal, the three home sets
// — and the person's collections under the sets that list their kinds, each
// by the address a client is given, its name and its mark.
func clientTree(blob sealed.Blob) []view.ProxyTreeRow {
	out := []view.ProxyTreeRow{{Path: "/dav/", Label: "root"}, {Path: "principal/", Depth: 1}}
	homes := []struct {
		path, label string
		kinds       []view.Kind
	}{
		{"cal/", "Calendars · events, tasks, notes", []view.Kind{view.KindEvent, view.KindTask, view.KindNote}},
		{"card/", "Address books", []view.Kind{view.KindContact}},
		{"files/", "Files · plain WebDAV", []view.Kind{view.KindFile}},
	}
	for _, h := range homes {
		out = append(out, view.ProxyTreeRow{Path: text(h.path), Label: text(h.label), Depth: 1})
		for _, conn := range blob.Connections {
			if conn.Feed {
				continue
			}
			for _, col := range blob.CollectionsOf(conn.ID) {
				if col.Gone || !slices.ContainsFunc(col.Kinds, func(k view.Kind) bool { return slices.Contains(h.kinds, k) }) {
					continue
				}
				out = append(out, view.ProxyTreeRow{Path: text("c/" + conn.ID + "/" + url.PathEscape(col.Path) + "/"),
					Label: text(collectionLabel(col)), Colour: catalogColour(col), Depth: 2})
			}
		}
	}
	return out
}

// issuedDialog is the password just issued (s-davloom-3, Нс49), shown this
// once: the device by its name, what it was given, the password with Copy
// and that it is not shown again, the address and the login to type, Done.
func (s *Server) issuedDialog(sess *session.Session, issued *issuedDevice) *view.DeviceDialog {
	sub := "Password created"
	for _, d := range s.users.Devices(sess.AccountID()) {
		if d.ID == issued.ID {
			access := "read and write"
			if d.ReadOnly {
				access = "read only"
			}
			sub += " · " + access + " · " + plural(len(d.ScopeCols), "collection")
		}
	}
	form := view.Form{Action: s.route("/app/settings/davloom"), Method: "GET", Sets: []view.FieldSet{{Rows: []view.FormRow{
		{Name: "address", Label: "Address", Kind: view.InputText, Value: text(s.davloomAddress()), ReadOnly: true, Mono: true},
		{Name: "login", Label: "User name", Kind: view.InputText, Value: text(sess.Login()), ReadOnly: true, Mono: true,
			Hint: "Your Carrel login; the password is the device's, not yours."},
	}}}}
	form.Actions = []view.Action{{ID: "davloom.done", Label: "Done", Href: s.route("/app/settings/davloom"), Method: "GET", Primary: true, Enabled: true}}
	return &view.DeviceDialog{
		Title: text(issued.Name), Subtitle: text(sub), Form: form,
		Password: &view.DevicePassword{Label: "Device password", Value: text(issued.Password),
			Warning: "Shown once and stored nowhere. Close this page and you will not see it again; lose it and you revoke this device and make another.",
			Copy:    view.Action{ID: "davloom.copy", Label: "Copy", Method: "GET", Enabled: true}},
	}
}

// deviceDialog is Add device (s-davloom-2, Нс48): the name and the right;
// «What the device will see» — the kinds, the muted words of each after its
// name —; «Collections»; the person's password asked where the window is
// closed (Нс73); Create password and Cancel; that the password is shown
// once.
func (s *Server) deviceDialog(r *http.Request, sess *session.Session, blob sealed.Blob, formErr error) *view.DeviceDialog {
	// What was typed: sent back with a refusal, or held while the
	// password's sheet was asked and brought back by its Cancel
	// (s-davloom-5).
	var typed url.Values
	if r.Method == http.MethodPost && formErr != nil {
		typed = r.PostForm
	} else if held, ok := heldDevice(sess); ok && r.URL.Query().Get("held") != "" {
		typed = held
	}
	posted := typed != nil
	on := func(name string, def bool) view.Text {
		if posted {
			v := typed.Get(name)
			return text(boolStr(v == "on" || v == "true"))
		}
		return text(boolStr(def))
	}
	head := []view.FormRow{csrfField(sess)}
	dialog := &view.DeviceDialog{Title: "Add device", Subtitle: "An app password for one device, with its own key wrapper"}
	dialog.Warning = "The password is shown once and stored nowhere. Lose it and you create a new one and revoke this — it cannot be recovered."
	name := ""
	if posted {
		name = typed.Get("name")
	}
	head = append(head, view.FormRow{Name: "op", Kind: view.InputHidden, Value: "issue"},
		view.FormRow{Name: "name", Label: "Name", Kind: view.InputText, Value: text(name), Required: true,
			Hint: "For you: this is how you will know what you are revoking later."})
	access := "write"
	if posted && typed.Get("access") == "read" {
		access = "read"
	}
	head = append(head, view.FormRow{Name: "access", Label: "What this device may do", Kind: view.InputRadio, Value: text(access),
		Options: []view.Option{
			{Value: "read", Label: "Read only", Selected: access == "read"},
			{Value: "write", Label: "Read and write", Selected: access == "write"},
		},
		Hint: "Read only: the device syncs, and whatever it sends back is refused. Changed later in the device’s row, without a new password."})
	// A new device sees calendars, address books and folders; tasks and
	// notes, which not every client understands, are ticked by the person
	// (s-davloom-2).
	var seen []view.FormRow
	for _, k := range deviceKinds {
		def := k.kind != view.KindTask && k.kind != view.KindNote
		seen = append(seen, view.FormRow{Name: "kind." + string(k.kind), Label: text(k.label), LabelAside: text(k.aside), Kind: view.InputCheck,
			Value: on("kind."+string(k.kind), def)})
	}
	var collections []view.FormRow
	for _, conn := range blob.Connections {
		if conn.Feed {
			continue
		}
		for _, col := range blob.CollectionsOf(conn.ID) {
			if col.Gone {
				continue
			}
			name := "col." + conn.ID + "|" + col.Path
			collections = append(collections, view.FormRow{Name: name, Label: view.Text(stamp(connLabel(conn), collectionLabel(col))), Kind: view.InputCheck,
				Value: on(name, true)})
		}
	}
	form := view.Form{Action: s.route("/app/settings/davloom"), Method: "POST"}
	switch {
	case formErr != nil && len(collections) > 0:
		collections[len(collections)-1].Error = text(formErr.Error())
		form.Errors = []view.FieldError{{Name: "scope", Message: text(formErr.Error())}}
	case formErr != nil:
		form.Errors = []view.FieldError{{Name: "scope", Message: text(formErr.Error())}}
	}
	form.Sets = []view.FieldSet{
		{Rows: head},
		{Legend: "What the device will see", LegendAside: "— changed later too, without re-issuing the password", Rows: seen},
		{Legend: "Collections", Rows: collections},
	}
	form = focused(form, "name")
	cancel := view.Action{ID: "davloom.cancel", Label: "Cancel", Href: s.route("/app/settings/davloom"), Method: "GET", Enabled: true}
	form.Actions = []view.Action{{ID: "davloom.issue", Label: "Create password", Method: "POST", Primary: true, Enabled: true}, cancel}
	dialog.Form = form
	return dialog
}
