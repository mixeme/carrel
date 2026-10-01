package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// connectDialog reads the Connect account dialog of a frame: the labels of its
// fields in order, the check's label, whether the check is ticked and
// whether the password input is drawn disabled.
type connectDialog struct {
	labels           []string
	check            string
	ticked, disabled bool
}

func readConnectDialog(t *testing.T, id string) connectDialog {
	t.Helper()
	frame := mockupFrame(t, id)
	at := strings.Index(frame, `<div class="tc-form`)
	end := strings.Index(frame, `<div class="tc-formfoot`)
	if at < 0 || end < at {
		t.Fatalf("%s draws no form", id)
	}
	form := frame[at:end]
	var f connectDialog
	for _, m := range regexp.MustCompile(`<label>([^<]+)</label>`).FindAllStringSubmatch(form, -1) {
		f.labels = append(f.labels, m[1])
	}
	check := regexp.MustCompile(`<button class="tc-check" type="button"><span class="tc-tickrow__tick( is-off)?"></span>([^<]+)</button>`).FindStringSubmatch(form)
	if check == nil {
		t.Fatalf("%s draws no check in the form", id)
	}
	f.check, f.ticked = check[2], check[1] == ""
	pw := regexp.MustCompile(`<input[^>]*aria-label="Password"[^>]*>`).FindString(form)
	f.disabled = strings.Contains(pw, " disabled")
	return f
}

// visibleRows are the rows of a form a person sees, in order.
func visibleRows(f view.Form) []view.FormRow {
	var rows []view.FormRow
	for _, set := range f.Sets {
		for _, r := range set.Rows {
			if r.Kind != view.InputHidden {
				rows = append(rows, r)
			}
		}
	}
	return rows
}

// from-typecase note 16, s-profile-14: a server that asks no password is
// said by a check right after Password, and the check turns the password
// field off. The labels, their order and the check's words are the frame's;
// the relation «this check turns that field off» is data (C-6.8 Disables).
func TestConnectFormNoPasswordCheckIsTheFrames(t *testing.T) {
	frame := readConnectDialog(t, "s-profile-14")
	if !frame.ticked || !frame.disabled {
		t.Fatalf("frame: ticked %v, password disabled %v — the reading of the frame is off", frame.ticked, frame.disabled)
	}
	h, sess := davHarness(t)
	page := decode[view.ConnectionsPage](t, h.do(get("/app/settings/connections?op=add", sess)))
	rows := visibleRows(*page.Add)
	// The frame's fields come in its order; rows it does not draw may sit
	// between them.
	i := 0
	for _, r := range rows {
		if i < len(frame.labels) && string(r.Label) == frame.labels[i] {
			i++
		}
	}
	if i < len(frame.labels) {
		var got []string
		for _, r := range rows {
			got = append(got, string(r.Label))
		}
		t.Errorf("form rows %q, want the frame's %q in order", got, frame.labels)
	}
	pw := -1
	for j, r := range rows {
		if r.Name == "password" {
			pw = j
		}
	}
	if pw < 0 || pw+1 >= len(rows) {
		t.Fatalf("no row after Password: %+v", rows)
	}
	check := rows[pw+1]
	if string(check.Label) != frame.check || check.Kind != view.InputCheck || check.Disables != "password" {
		t.Errorf("row after Password = %+v, want the check %q turning off password", check, frame.check)
	}
	if check.Value != "" || rows[pw].ReadOnly {
		t.Errorf("a new form: check %q, password read-only %v — want unticked and on", check.Value, rows[pw].ReadOnly)
	}
}

// The Connect account dialog as s-profile-14 draws it: every field's
// placeholder and hint are the frame's words (Label says «optional» and has
// no hint, Server URL says how discovery goes), and the foot carries the
// frame's buttons in its order. The dialog is the content of main, not a
// sheet over the screen, so its Cancel is the server's (C-7.2).
func TestConnectDialogFieldsAndFootAreTheFrames(t *testing.T) {
	frame := mockupFrame(t, "s-profile-14")
	at := strings.Index(frame, `<div class="tc-form`)
	foot := strings.Index(frame, `<div class="tc-formfoot`)
	if at < 0 || foot < at {
		t.Fatal("s-profile-14 draws no form")
	}
	tags := regexp.MustCompile(`<[^>]+>`)
	type drawn struct{ placeholder, hint string }
	fields := map[string]drawn{}
	for _, part := range strings.Split(frame[at:foot], `<div class="tc-field`)[1:] {
		label := regexp.MustCompile(`<label>([^<]+)</label>`).FindStringSubmatch(part)
		if label == nil {
			continue
		}
		var d drawn
		if m := regexp.MustCompile(`placeholder="([^"]*)"`).FindStringSubmatch(part); m != nil {
			d.placeholder = m[1]
		}
		if m := regexp.MustCompile(`(?s)<span class="tc-field__hint">(.*?)</span>(</div>|$)`).FindStringSubmatch(part); m != nil {
			d.hint = tags.ReplaceAllString(m[1], "")
		}
		fields[label[1]] = d
	}
	var buttons []string
	for _, m := range regexp.MustCompile(`<button class="tc-btn[^"]*" type="button">([^<]+)</button>`).FindAllStringSubmatch(frame[foot:], -1) {
		buttons = append(buttons, m[1])
	}
	if fields["Server URL"].hint == "" || fields["Label"].placeholder == "" || len(buttons) == 0 {
		t.Fatalf("the reading of the frame is off: %+v %q", fields, buttons)
	}

	h, sess := davHarness(t)
	page := decode[view.ConnectionsPage](t, h.do(get("/app/settings/connections?op=add", sess)))
	for _, r := range visibleRows(*page.Add) {
		d, ok := fields[string(r.Label)]
		if !ok {
			continue
		}
		if string(r.Placeholder) != d.placeholder || string(r.Hint) != d.hint {
			t.Errorf("%s: placeholder %q hint %q, want the frame's %q and %q", r.Label, r.Placeholder, r.Hint, d.placeholder, d.hint)
		}
	}
	var got []string
	for _, a := range page.Add.Actions {
		got = append(got, string(a.Label))
	}
	if strings.Join(got, "|") != strings.Join(buttons, "|") {
		t.Errorf("foot %q, want the frame's %q", got, buttons)
	}
	if c := actionByID(page.Add.Actions, "connection.add.cancel"); c == nil || c.Method != "GET" || c.Href != "/app/settings/connections" {
		t.Errorf("Cancel = %+v, want a GET back to the connections", c)
	}
}

// noPasswordServer asks a login and an empty password, as Xandikos behind a
// proxy that lets the network in does.
func noPasswordServer(t *testing.T) *davtest.Server {
	t.Helper()
	srv := davtest.New(t)
	srv.Login, srv.Password = "mix", ""
	srv.AddCalendar(caldav.Calendar{Path: davtest.CalendarHome + "work/", Name: "Work"})
	return srv
}

func checkRow(t *testing.T, f view.Form) (check, password *view.FormRow) {
	t.Helper()
	password = formRow(f, "password")
	for _, set := range f.Sets {
		for j := range set.Rows {
			if set.Rows[j].Disables == "password" {
				check = &set.Rows[j]
			}
		}
	}
	if check == nil || password == nil {
		t.Fatalf("form without the no-password check: %+v", f)
	}
	return check, password
}

// The mark says «no password» where an empty field cannot: Connect with it
// ignores whatever the field holds, stores the connection as asking none,
// and the edit form comes pre-ticked with the password field off.
func TestConnectWithoutPassword(t *testing.T) {
	srv := noPasswordServer(t)
	h, sess := davHarness(t)
	rec := h.do(postForm("/app/settings/connections", sess, url.Values{
		"server": {srv.URL}, "login": {"mix"}, "password": {"typed-left-over-9"}, "noPassword": {"on"},
	}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("connect: %d %s", rec.Code, rec.Body.String())
	}
	key, _ := sess.DataKey()
	blob, _ := h.users.Catalog(sess.AccountID(), key)
	if len(blob.Connections) != 1 || blob.Connections[0].Password != "" || !blob.Connections[0].NoPassword {
		t.Fatalf("connections = %+v, want one asking no password", blob.Connections)
	}
	id := blob.Connections[0].ID

	page := decode[view.ConnectionsPage](t, h.do(get("/app/settings/connections?"+url.Values{"op": {"edit"}, "account": {id}}.Encode(), sess)))
	check, password := checkRow(t, *page.Add)
	if check.Value == "" || !password.ReadOnly {
		t.Errorf("edit form: check %q, password read-only %v — want it ticked and the field off", check.Value, password.ReadOnly)
	}

	// No password to show: the card has no Show password and says «no
	// password» instead of releasing nothing into the journal (s-profile-15).
	page = decode[view.ConnectionsPage](t, h.do(get("/app/settings/connections", sess)))
	card := cardOf(t, page, id)
	if card.Password != nil || card.Access == "" {
		t.Errorf("password = %+v, access %q; want no Show password and the line saying so", card.Password, card.Access)
	}

	// Test only reads no password under the mark either.
	trace := decode[traceFragment](t, h.do(fragment(postForm("/app/settings/connections/test", sess, url.Values{
		"server": {srv.URL}, "login": {"mix"}, "password": {"typed-left-over-9"}, "noPassword": {"on"}}), "main"))).Body.Trace
	if !trace.OK {
		t.Errorf("Test only with the mark = %+v", trace)
	}

	// A failed discovery gives the form back with the mark as it was sent.
	page = decode[view.ConnectionsPage](t, h.do(postForm("/app/settings/connections", sess, url.Values{
		"label": {"other"}, "server": {srv.URL + "/nowhere/"}, "login": {"nobody"}, "noPassword": {"on"},
	})))
	if noticeTitled(page.Shell.Notices, "Could not connect") == nil {
		t.Fatalf("notices = %+v, want the failure", page.Shell.Notices)
	}
	check, password = checkRow(t, *page.Add)
	if check.Value == "" || !password.ReadOnly {
		t.Errorf("failed connect: check %q, password read-only %v — want the mark echoed", check.Value, password.ReadOnly)
	}
}

// The open item «a password cannot be cleared through the connection form»:
// a stored password the server does not want is cleared by the mark on
// edit, and the links of the connection read with the empty one. Without
// the mark an empty field still keeps the stored password.
func TestEditClearsThePasswordWithTheMark(t *testing.T) {
	srv := noPasswordServer(t)
	srv.Password = "stored-secret-7"
	f := connectFixture(t, srv, davtest.CalendarHome+"work/", nil)
	key, _ := f.sess.DataKey()
	conn := catalogOf(t, f).Connections[0]
	link, err := f.h.users.Publish(f.sess.AccountID(), key, conn, user.PublishTarget{AccountID: f.id, Kind: "calendar", Col: davtest.CalendarHome + "work/", Title: "Work"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv.Password = "" // the proxy in front stopped asking

	save := "/app/settings/connections?" + url.Values{"op": {"edit"}, "account": {f.id}}.Encode()
	post := func(v url.Values) view.ConnectionsPage {
		t.Helper()
		return decode[view.ConnectionsPage](t, f.h.do(postForm(save, f.sess, v)))
	}

	// Unticked and empty: «unchanged» — the stored password goes on, and the
	// server refuses it.
	page := post(url.Values{"server": {conn.Server}, "login": {"mix"}})
	if noticeTitled(page.Shell.Notices, "Could not connect") == nil {
		t.Fatalf("an empty field did not keep the stored password: %+v", page.Shell.Notices)
	}
	if got := catalogOf(t, f).Connections[0]; got.Password != "stored-secret-7" || got.NoPassword {
		t.Fatalf("a failed edit changed the connection: %+v", got)
	}
	check, password := checkRow(t, *page.Add)
	if check.Value != "" || password.ReadOnly {
		t.Errorf("failed edit without the mark: check %q, password read-only %v", check.Value, password.ReadOnly)
	}

	page = post(url.Values{"server": {conn.Server}, "login": {"mix"}, "password": {"typed-left-over-9"}, "noPassword": {"true"}})
	if n := noticeTitled(page.Shell.Notices, "Could not connect"); n != nil {
		t.Fatalf("edit with the mark failed: %+v", n)
	}
	got := catalogOf(t, f).Connections[0]
	if got.ID != f.id || got.Password != "" || !got.NoPassword {
		t.Fatalf("after the edit: %+v, want the same ID with no password", got)
	}
	open, ok := f.h.users.OpenPublished(link)
	if !ok || open.Conn.Password != "" {
		t.Errorf("the link reads with %q, want the cleared password", open.Conn.Password)
	}
	for _, e := range f.h.st.Snapshot().Audit {
		if strings.Contains(e.Detail, "stored-secret-7") || strings.Contains(e.Detail, "typed-left-over-9") {
			t.Errorf("journal line %q carries password material", e.Detail)
		}
	}
	if auditCount(f.h, "connection.edit") != 1 {
		t.Errorf("connection.edit lines = %d, want 1", auditCount(f.h, "connection.edit"))
	}

	// Unticked with a password typed: that password, and the mark is gone.
	srv.Password = "new"
	post(url.Values{"server": {conn.Server}, "login": {"mix"}, "password": {"new"}})
	if got := catalogOf(t, f).Connections[0]; got.Password != "new" || got.NoPassword {
		t.Errorf("after typing a password: %+v", got)
	}
}
