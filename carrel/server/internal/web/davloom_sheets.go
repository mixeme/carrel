package web

import (
	"net/http"
	"net/url"
	"slices"
	"strings"

	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// The sheets over davloom (Typecase's note 96): what a device sees, changed
// without a new password (s-davloom-7), and what a client sees at the
// address (s-davloom-9; owner's decision 19.10.2026).

// davloomDevice is the device of ?device= among the person's, or a 404.
func (s *Server) davloomDevice(w http.ResponseWriter, r *http.Request) (*session.Session, store.ProxyDevice, bool) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return nil, store.ProxyDevice{}, false
	}
	id := r.URL.Query().Get("device")
	for _, d := range s.users.Devices(sess.AccountID()) {
		if d.ID == id {
			return sess, d, true
		}
	}
	s.notFound(w, r)
	return nil, store.ProxyDevice{}, false
}

func (s *Server) getDavloomScope(w http.ResponseWriter, r *http.Request) {
	sess, d, ok := s.davloomDevice(w, r)
	if !ok {
		return
	}
	s.writeDavloomScope(w, r, s.davloomScopeSheet(sess, d, nil, ""))
}

// postDavloomScope keeps the collections ticked and goes back to davloom;
// none ticked is refused in the sheet. A collection of a kind the device
// did not see adds its kinds: what is ticked is what it sees.
func (s *Server) postDavloomScope(w http.ResponseWriter, r *http.Request) {
	sess, d, ok := s.davloomDevice(w, r)
	if !ok {
		return
	}
	key, live := sess.DataKey()
	if !live {
		s.notFound(w, r)
		return
	}
	_ = readForm(r)
	blob := s.catalogOf(sess)
	sc := user.DeviceScope{ReadOnly: d.ReadOnly}
	kinds := map[view.Kind]bool{}
	for _, k := range d.ScopeKinds {
		kinds[view.Kind(k)] = true
		sc.Kinds = append(sc.Kinds, view.Kind(k))
	}
	ticked := map[string]bool{}
	for _, v := range r.PostForm["col"] {
		account, path, ok := strings.Cut(v, "|")
		col, found := blob.Collection(account, path)
		if !ok || !found {
			continue
		}
		ticked[v] = true
		sc.Cols = append(sc.Cols, store.DeviceScope{AccountID: account, Path: path})
		if !slices.ContainsFunc(col.Kinds, func(k view.Kind) bool { return kinds[k] }) {
			for _, k := range col.Kinds {
				kinds[k] = true
				sc.Kinds = append(sc.Kinds, k)
			}
		}
	}
	if err := s.users.SetDeviceScope(sess.AccountID(), key, d.ID, sc); err != nil {
		s.writeDavloomScope(w, r, s.davloomScopeSheet(sess, d, ticked, text(err.Error())))
		return
	}
	s.redirect(w, r, "/app/settings/davloom")
}

func (s *Server) writeDavloomScope(w http.ResponseWriter, r *http.Request, sheet view.DavloomScopeSheet) {
	if render.WantsFragment(r) && render.FragmentTarget(r) == render.TargetSheet {
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, sheet, nil)
		return
	}
	sheet.Under = s.underOf(r, "", "/app/settings/davloom") // Лс29
	s.writePage(w, r, http.StatusOK, sheet)
}

// davloomScopeSheet is «What it sees» (s-davloom-7): the device by its
// name; the collections by connection, a column each, as the sources of a
// copy job — a connection's first one names it, each its mark and what it
// is —, those under the home sets the device sees (a phone of calendars
// and address books is not offered folders); a connection whose password
// was refused is not offered; Save and Cancel. ticked is what was sent with
// a refusal; nil — the device's own.
func (s *Server) davloomScopeSheet(sess *session.Session, d store.ProxyDevice, ticked map[string]bool, refusal view.Text) view.DavloomScopeSheet {
	if ticked == nil {
		ticked = map[string]bool{}
		for _, c := range d.ScopeCols {
			ticked[selValue(c.AccountID, c.Path)] = true
		}
	}
	blob := s.catalogOf(sess)
	homes := deviceHomes(d)
	var opts []view.Option
	for _, conn := range blob.Connections {
		if conn.Feed || s.passwordRefusedOf(sess, blob, conn) {
			continue
		}
		first := true
		for _, col := range blob.CollectionsOf(conn.ID) {
			if col.Gone || !slices.ContainsFunc(col.Kinds, func(k view.Kind) bool { return homes[homeOf(k)] }) {
				continue
			}
			v := selValue(conn.ID, col.Path)
			o := view.Option{Value: v, Label: text(collectionLabel(col)), Selected: ticked[v], Colour: catalogColour(col), Meta: sourceKind(col)}
			if first {
				o.Group, first = text(connLabel(conn)), false
			}
			opts = append(opts, o)
		}
	}
	action := "/app/settings/davloom/scope?" + url.Values{"device": {d.ID}}.Encode()
	form := s.plainForm(action, "POST", csrfField(sess))
	form.Target = render.TargetSheet
	form.Sets = append(form.Sets, view.FieldSet{Rows: []view.FormRow{{Name: "col", Label: "What it sees", Kind: view.InputCheck, Multiple: true, Options: opts, Error: refusal}}})
	if refusal != "" {
		form.Errors = []view.FieldError{{Name: "col", Message: refusal}}
	}
	form.Actions = []view.Action{
		{ID: "davloom.scope.save", Label: "Save", Href: s.route(action), Method: "POST", Primary: true, Enabled: true, Target: render.TargetSheet},
		{ID: "davloom.scope.cancel", Label: "Cancel", Href: s.route("/app/settings/davloom"), Method: "GET", Enabled: true},
	}
	return view.DavloomScopeSheet{Subject: text(d.Name), Form: form}
}

// getDavloomTree is «What the client sees» (s-davloom-9): the address in
// the head, the tree, Close.
func (s *Server) getDavloomTree(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	sheet := view.DavloomTreeSheet{
		Address: text(s.davloomAddress()),
		Tree:    clientTree(s.catalogOf(sess)),
		Actions: []view.Action{{ID: "davloom.tree.close", Label: "Close", Href: s.route("/app/settings/davloom"), Method: "GET", Enabled: true}},
	}
	if render.WantsFragment(r) && render.FragmentTarget(r) == render.TargetSheet {
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, sheet, nil)
		return
	}
	sheet.Under = s.underOf(r, "", "/app/settings/davloom") // Лс29
	s.writePage(w, r, http.StatusOK, sheet)
}

// homeOf is the home set of davloom a kind lives under (S-27.3): events,
// tasks and notes in cal/, contacts in card/, files in files/.
func homeOf(k view.Kind) string {
	switch k {
	case view.KindContact:
		return "card/"
	case view.KindFile:
		return "files/"
	}
	return "cal/"
}

// deviceHomes are the home sets a device sees, by its kinds.
func deviceHomes(d store.ProxyDevice) map[string]bool {
	out := map[string]bool{}
	for _, k := range d.ScopeKinds {
		out[homeOf(view.Kind(k))] = true
	}
	return out
}
