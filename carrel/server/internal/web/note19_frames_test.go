package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

// from-typecase note 19: the frames of the administration, the reset, the
// note, Related to, the account, the move and From Files, read as they are
// drawn now.

var frameLabelMuted = regexp.MustCompile(`(?s)^(.*?)\s*<span class="tc-dim">(.*?)</span>\s*$`)

// splitLabel is a field's label as the frame draws it: the label, and the
// muted part after it («Email» and «(optional)»), each as it reads.
func splitLabel(label string) (main, aside string) {
	if m := frameLabelMuted.FindStringSubmatch(label); m != nil {
		return frameText(m[1]), frameText(m[2])
	}
	return frameText(label), ""
}

// frameMutedLabel is the label of a frame that has a muted part and begins
// with main.
func frameMutedLabel(t *testing.T, id, main string) (string, string) {
	t.Helper()
	for _, m := range frameLabel.FindAllStringSubmatch(mockupFrame(t, id), -1) {
		if l, aside := splitLabel(m[1]); l == main && aside != "" {
			return l, aside
		}
	}
	t.Fatalf("%s draws no %q with a muted part: read the frame again", id, main)
	return "", ""
}

// from-typecase note 19, task 33: a label with a muted part is data — the
// part travels apart from the label (FormRow.LabelAside), wherever the
// frames draw one: «Email» «(optional)» of the temporary password
// (s-admin-1, s-admin-19) and of the first administrator (s-auth-4),
// «Certificate fingerprint» «SHA-256 · optional» of a connection
// (s-profile-2, -11, -14, -16).
func TestMutedPartOfALabelIsData(t *testing.T) {
	h := newHarness(t)
	setup := decode[view.SetupPage](t, h.do(get("/setup", nil)))
	label, aside := frameMutedLabel(t, "s-auth-4", "Email")
	if row := rowNamed(t, setup.Form, "email"); string(row.Label) != label || string(row.LabelAside) != aside {
		t.Errorf("setup email = %q + %q, want (s-auth-4) %q + %q", row.Label, row.LabelAside, label, aside)
	}

	h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	page := decode[view.AdminInvitesPage](t, h.do(get("/admin/invites", alice)))
	for _, id := range []string{"s-admin-1", "s-admin-19"} {
		label, aside := frameMutedLabel(t, id, "Email")
		if row := rowNamed(t, page.Temporary.Form, "email"); string(row.Label) != label || string(row.LabelAside) != aside {
			t.Errorf("temporary password email = %q + %q, want (%s) %q + %q", row.Label, row.LabelAside, id, label, aside)
		}
	}

	label, aside = frameMutedLabel(t, "s-profile-2", "Certificate fingerprint")
	add := decode[view.ConnectionsPage](t, h.do(get("/app/settings/connections?"+url.Values{"op": {"add"}}.Encode(), alice)))
	if add.Add == nil {
		t.Fatal("no Connect account form")
	}
	if row := rowNamed(t, *add.Add, "certFingerprint"); string(row.Label) != label || string(row.LabelAside) != aside {
		t.Errorf("fingerprint = %q + %q, want (s-profile-2) %q + %q", row.Label, row.LabelAside, label, aside)
	}
	for _, r := range add.Add.Sets {
		for _, row := range r.Rows {
			if strings.Contains(string(row.Label), "optional") {
				t.Errorf("%s: the label %q still carries its muted part", row.Name, row.Label)
			}
		}
	}
}

// from-typecase notes 19 and 32, tasks 30 and 79: /invite/{token} is
// s-auth-12 (a letter) and s-auth-14 (a link), not the registration of
// s-auth-2 — «Join» and the instance, who invited, the warning over the
// fields, then Login, Email, Password and Repeat password, Create account.
// The address is shown and can be changed (the owner's decision of 02.10):
// a letter's is put in, the one the invitation went to, a link's is empty;
// each says under it how it is confirmed. An address the letter did not go
// to waits for its own letter of confirmation; the one it went to is proven.
// An invitation that named its login shows it off. A refusal at Login is
// said in the words of s-admin-19.
func TestInvitePageIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-auth-12")
	title := frameText(regexp.MustCompile(`(?s)<div class="tc-head__title[^"]*">(.*?)</div>`).FindStringSubmatch(frame)[1])
	lede := frameText(regexp.MustCompile(`(?s)<div class="tc-head__sub[^"]*">(.*?)</div>`).FindStringSubmatch(frame)[1])
	warning := frameText(frameCardAlert.FindStringSubmatch(frame)[1])
	type field struct{ label, value, placeholder, hint string }
	fieldsOf := func(id string) []field {
		var out []field
		for _, m := range regexp.MustCompile(`(?s)<label>([^<]*)</label><input ([^>]*)>(?:<span class="tc-field__hint">(.*?)</span>)?`).FindAllStringSubmatch(mockupFrame(t, id), -1) {
			attrs := map[string]string{}
			for _, a := range frameAttr.FindAllStringSubmatch(m[2], -1) {
				attrs[a[1]] = a[2]
			}
			if strings.Contains(m[2], " disabled") {
				t.Fatalf("%s draws %q off: read the frame again", id, m[1])
			}
			out = append(out, field{m[1], attrs["value"], attrs["placeholder"], frameText(m[3])})
		}
		return out
	}
	letterFields, linkFields := fieldsOf("s-auth-12"), fieldsOf("s-auth-14")
	submit := frameText(regexp.MustCompile(`(?s)<div class="tc-formfoot[^"]*"><button class="tc-btn is-primary"[^>]*>(.*?)</button>`).FindStringSubmatch(frame)[1])
	if title != "Join carrel.example" || lede != "mix invited you. Choose a login and a password." || len(letterFields) != 4 || len(linkFields) != 4 ||
		letterFields[1].label != "Email" || letterFields[1].value != "anna@example.org" || letterFields[1].hint == "" ||
		linkFields[1].label != "Email" || linkFields[1].value != "" || linkFields[1].placeholder == "" || linkFields[1].hint == "" {
		t.Fatalf("s-auth-12, -14 = %q, %q, %q, %q: read the frames again", title, lede, letterFields, linkFields)
	}
	if linkLede := frameText(regexp.MustCompile(`(?s)<div class="tc-head__sub[^"]*">(.*?)</div>`).FindStringSubmatch(mockupFrame(t, "s-auth-14"))[1]); linkLede != lede {
		t.Fatalf("s-auth-14 says %q under the title, s-auth-12 %q: read the frames again", linkLede, lede)
	}

	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	if err := h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		st.Settings.Name = "carrel.example"
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	relay := h.relayed()
	alice := h.login("alice", "password123")
	secretOf := func(action, email string) string {
		t.Helper()
		before := len(h.st.Snapshot().Invites)
		if rec := h.do(postForm("/admin/invites?action="+action, alice, url.Values{"email": {email}})); rec.Code != http.StatusSeeOther {
			t.Fatalf("invite by %s: %d %s", action, rec.Code, rec.Body)
		}
		snap := h.st.Snapshot()
		if len(snap.Invites) != before+1 {
			t.Fatalf("invitations = %+v", snap.Invites)
		}
		return snap.Invites[before].Secret
	}
	type row struct {
		label, value, placeholder, hint string
		off, required                   bool
	}
	rows := func(p view.InvitePage) []row {
		var out []row
		for _, set := range p.Form.Sets {
			for _, r := range set.Rows {
				if r.Kind != view.InputHidden {
					out = append(out, row{string(r.Label), string(r.Value), string(r.Placeholder), string(r.Hint), r.ReadOnly, r.Required})
				}
			}
		}
		return out
	}
	check := func(what string, p view.InvitePage, wantLede string, want []row) {
		t.Helper()
		if string(p.Title) != title || saidRuns(p.Lede) != wantLede || len(p.Notices) != 1 ||
			p.Notices[0].Role != view.NoticeAlert || string(p.Notices[0].Body) != warning || p.Notices[0].Title != "" {
			t.Errorf("%s: head %q, %q, %+v; want %q, %q and the warning %q", what, p.Title, p.Lede, p.Notices, title, wantLede, warning)
		}
		if got := rows(p); len(got) != len(want) || !slices.Equal(got, want) {
			t.Errorf("%s: fields %+v, want %+v", what, got, want)
		}
		if got := labels(p.Form.Actions); !equalStrings(got, []string{submit}) {
			t.Errorf("%s: actions %q, want %q", what, got, submit)
		}
	}
	// The line under the title by the kind of invitation is the caption's
	// (from-typecase note 22): a login named before, no record of who
	// invited — without the first sentence.
	_, caption, _ := strings.Cut(frame, "<figcaption>")
	var kinds []string
	for _, m := range regexp.MustCompile(`«([^«»]*)»`).FindAllStringSubmatch(caption, -1) {
		kinds = append(kinds, frameText(m[1]))
	}
	if len(kinds) != 2 || kinds[0] != lede || !strings.HasPrefix(kinds[1], "mix invited you. ") ||
		!strings.Contains(caption, "без записи, кто пригласил, — без первой фразы") {
		t.Fatalf("s-auth-12 caption lines = %q: read the frame again", kinds)
	}
	byAlice := func(s string) string { return strings.Replace(s, "mix", "alice", 1) }
	lede = byAlice(lede)
	email := func(value, hint string) row {
		return row{"Email", value, linkFields[1].placeholder, hint, false, true}
	}
	pass := []row{{"Password", "", "", "", false, true}, {"Repeat password", "", "", "", false, true}}
	login := row{"Login", "", "", "", false, true}
	linkRows := append([]row{login, email("", linkFields[1].hint)}, pass...)

	letter := secretOf("send", "anna@example.org")
	check("a letter", decode[view.InvitePage](t, h.do(get("/invite/"+letter, nil))), lede,
		append([]row{login, email("anna@example.org", letterFields[1].hint)}, pass...))

	link := secretOf("link", "")
	check("a link", decode[view.InvitePage](t, h.do(get("/invite/"+link, nil))), lede, linkRows)

	named, err := h.users.CreateInvite(adminID, "vera", "")
	if err != nil {
		t.Fatal(err)
	}
	check("a login named", decode[view.InvitePage](t, h.do(get("/invite/"+named, nil))), byAlice(kinds[1]),
		append([]row{{"Login", "vera", "", "", true, false}, email("", linkFields[1].hint)}, pass...))

	// The login is refused at its field in the words of s-admin-19; an
	// address that is no address, at its own, with what was typed kept.
	taken := decode[view.InvitePage](t, h.do(h.anonPostForm("/invite/"+link, "/invite/"+link,
		url.Values{"login": {"alice"}, "email": {"vera@example.org"}, "password": {"verapass123"}, "password2": {"verapass123"}})))
	if r := rowNamed(t, taken.Form, "login"); r.Error != "Login is already taken." {
		t.Errorf("a login taken: %+v", r)
	}
	for _, typed := range []string{"", "vera at example"} {
		bad := decode[view.InvitePage](t, h.do(h.anonPostForm("/invite/"+link, "/invite/"+link,
			url.Values{"login": {"vera"}, "email": {typed}, "password": {"verapass123"}, "password2": {"verapass123"}})))
		if r := rowNamed(t, bad.Form, "email"); r.Error == "" || string(r.Value) != typed || string(rowNamed(t, bad.Form, "login").Value) != "vera" {
			t.Errorf("the address %q: %+v", typed, bad.Form)
		}
	}
	if snap := h.st.Snapshot(); snap.UserByLogin("vera") != nil && len(snap.UserByLogin("vera").LoginHash) > 0 {
		t.Fatal("a refused form made the account")
	}

	// Accepted: the address the letter went to is proven; another one, and
	// a link's, wait for a letter of their own.
	accept := func(secret, login, email string) store.User {
		t.Helper()
		sent := len(relay.Letters())
		if rec := h.do(h.anonPostForm("/invite/"+secret, "/invite/"+secret,
			url.Values{"login": {login}, "email": {email}, "password": {"verapass123"}, "password2": {"verapass123"}})); rec.Code != http.StatusSeeOther {
			t.Fatalf("%s: %d %s", login, rec.Code, rec.Body)
		}
		snap := h.st.Snapshot()
		u := snap.UserByLogin(login)
		if u == nil {
			t.Fatalf("%s: no account", login)
		}
		letters := relay.Letters()[sent:]
		switch {
		case u.EmailConfirmed && len(letters) != 0:
			t.Errorf("%s: the address is proven, and a letter went: %+v", login, letters)
		case !u.EmailConfirmed && (len(letters) != 1 || !slices.Equal(letters[0].To, []string{email}) || !strings.Contains(letters[0].Data, "/confirm-email/")):
			t.Errorf("%s: the letter of confirmation = %+v", login, letters)
		}
		return *u
	}
	if u := accept(letter, "anna", "anna@example.org"); u.Email != "anna@example.org" || !u.EmailConfirmed || u.PendingEmail != "" {
		t.Errorf("the address the invitation went to: %+v", u)
	}
	other := secretOf("send", "boris@example.org")
	if u := accept(other, "boris", "b.petrov@example.org"); u.Email != "" || u.EmailConfirmed || u.PendingEmail != "b.petrov@example.org" {
		t.Errorf("another address: %+v", u)
	}
	if u := accept(link, "vera2", "vera@example.org"); u.Email != "" || u.EmailConfirmed || u.PendingEmail != "vera@example.org" {
		t.Errorf("a link's address: %+v", u)
	}

	// An invitation without a record of who made it has no first sentence.
	last := secretOf("link", "")
	if err := h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		for i := range st.Invites {
			st.Invites[i].By = ""
		}
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	check("a link, nobody named", decode[view.InvitePage](t, h.do(get("/invite/"+last, nil))), strings.TrimPrefix(lede, "alice invited you. "), linkRows)
}

// from-typecase notes 19 and 22: «local» is the instance's, not a screen's —
// the addresses it hands out answer only from inside its own network. It
// stands beside «On a clock of your own» (s-backup-2), in the heads of
// davloom (s-davloom-1) and of the published links (s-publish-1), and on the
// card of a device's link (s-plan-1); the shell carries it for all of them.
// On when no outside address is declared (CARREL_PUBLIC_URL — the line is
// built from where the browser came) or the declared one is a name or an
// address of a private network; off for a public one.
func TestLocalInstanceIsTheShells(t *testing.T) {
	badge := regexp.MustCompile(`<span class="tc-badge is-local"[^>]*>local</span>`)
	tick := regexp.MustCompile(`(?s)<label class="tc-check">(?:<input[^>]*>)?(?:<span[^>]*></span>)?On a clock of your own(.*?)</label>`).FindStringSubmatch(mockupFrame(t, "s-backup-2"))
	if tick == nil || !badge.MatchString(tick[1]) {
		t.Fatalf("s-backup-2 draws no «local» at the tick: read the frame again")
	}
	routes := map[string]string{"s-davloom-1": "/app/settings/davloom", "s-publish-1": "/app/files/published",
		"s-plan-1": "/app/settings/devices", "s-backup-2": "/app/settings/backup"}
	for id := range routes {
		frame, _, _ := strings.Cut(mockupFrame(t, id), "<figcaption")
		if !badge.MatchString(frame) {
			t.Fatalf("%s draws no «local»: read the frame again", id)
		}
	}
	f := backupFixture(t)
	saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "8", "pw"))
	for _, c := range []struct {
		public string
		local  bool
	}{
		{"", true}, {"http://192.168.1.20:8080", true}, {"https://carrel.lan", true}, {"http://localhost:8080", true},
		{"https://[fd00::5]", true}, {"http://100.101.5.2", true}, {"https://carrel.example.org", false}, {"https://203.0.113.9", false},
	} {
		f.h.cfg.PublicURL = c.public
		for id, route := range routes {
			page := decode[struct {
				Top view.TopBar `json:"top"`
			}](t, f.h.do(get(route, f.sess)))
			if page.Top.Signed.Local != c.local {
				t.Errorf("%s (%s), CARREL_PUBLIC_URL %q: local = %v, want %v", route, id, c.public, page.Top.Signed.Local, c.local)
			}
		}
		if tb := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup", f.sess))).Jobs[0].Trigger; tb != nil {
			t.Errorf("the trigger is off, and its card is sent: %+v", tb)
		}
	}
	raw := decode[map[string]any](t, f.h.do(get("/app/settings/backup", f.sess)))
	if _, ok := raw["local"]; ok {
		t.Error("the backup page still carries «local» of its own")
	}
}

// from-typecase note 22: what turning the external trigger on costs is said
// under «On a clock of your own», ticked or not (s-backup-2, S-24.4) — and
// so not a second time in the card of the line.
func TestClockTickSaysItsPrice(t *testing.T) {
	m := regexp.MustCompile(`(?s)On a clock of your own.*?</label>\s*<span class="tc-field__hint">(.*?)</span>`).FindStringSubmatch(mockupFrame(t, "s-backup-2"))
	if m == nil {
		t.Fatal("s-backup-2 draws no hint under «On a clock of your own»: read the frame again")
	}
	hint := frameText(m[1])
	f := backupFixture(t)
	job := saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "8", "pw"))
	if got := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup", f.sess))).Jobs[0]; string(got.ClockHint) != hint || got.Clock.On {
		t.Errorf("unticked: hint = %q, want (s-backup-2) %q", got.ClockHint, hint)
	}
	_, tb := issueTrigger(t, f, job)
	if strings.Contains(string(tb.Note), "server's key") {
		t.Errorf("the card says the price again: %q", tb.Note)
	}
	if got := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup", f.sess))).Jobs[0]; string(got.ClockHint) != hint || !got.Clock.On {
		t.Errorf("ticked: hint = %q, want (s-backup-2) %q", got.ClockHint, hint)
	}
}

// from-typecase note 19, task 31: the head of the task list is the head of
// every kind — New task, Import, Export (s-tasks-1) — and Print, the list
// with print=1 since the owner's decision 4 of the client's plan (the
// client's scan of the tasks, ЗБ3).
func TestTaskListHeadIsTheFrame(t *testing.T) {
	var want []string
	for _, b := range headButtons(t, "s-tasks-1") {
		label, _, _ := strings.Cut(b, " [")
		want = append(want, label)
	}
	f := calendarFixture(t, nil)
	page := decode[struct {
		Bar view.ListBar `json:"bar"`
	}](t, f.h.do(get(f.path("tasks"), f.sess)))
	if got := labels(page.Bar.Actions); !equalStrings(got, want) {
		t.Errorf("head = %q, want (s-tasks-1) %q", got, want)
	}
}

// from-typecase note 19, task 32 (owner's decision): the categories are
// «Tags» everywhere — the full cards of an event, a contact and a task
// (s-agenda-2, s-contacts-6, s-tasks-2) as the panels and the forms.
func TestFullCardsLabelTheCategoriesTags(t *testing.T) {
	tagTerm := regexp.MustCompile(`<dt>([^<]*)</dt><dd><span class="tc-tag">`)
	term := ""
	for _, id := range []string{"s-agenda-2", "s-contacts-6", "s-tasks-2"} {
		m := tagTerm.FindStringSubmatch(mockupFrame(t, id))
		if m == nil || (term != "" && m[1] != term) {
			t.Fatalf("%s labels its tags %q, another %q: read the frames again", id, m, term)
		}
		term = m[1]
	}
	labelOf := func(fields []view.Field) []string {
		var out []string
		for _, f := range fields {
			if f.Label == view.Text(term) || f.Label == "Categories" {
				out = append(out, string(f.Label))
			}
		}
		return out
	}
	card := annaPage(t)
	if got := labelOf(card.Fields); !equalStrings(got, []string{term}) {
		t.Errorf("contact: %q, want (s-contacts-6) %q", got, term)
	}
	f := calendarFixture(t, nil,
		"e.ics", ics("BEGIN:VEVENT\nUID:e\nDTSTAMP:20260101T000000Z\nDTSTART:20260115T100000Z\nSUMMARY:Review\nCATEGORIES:work\nEND:VEVENT"),
		"t.ics", ics("BEGIN:VTODO\nUID:t\nDTSTAMP:20260101T000000Z\nSUMMARY:Pack\nCATEGORIES:home\nEND:VTODO"))
	event := decode[view.EventPage](t, f.h.do(get(f.path("calendar")+"/e", f.sess)))
	if got := labelOf(event.Fields); !equalStrings(got, []string{term}) {
		t.Errorf("event: %q, want (s-agenda-2) %q", got, term)
	}
	task := decode[view.TaskPage](t, f.h.do(get(f.path("tasks")+"/t", f.sess)))
	if got := labelOf(task.Fields); !equalStrings(got, []string{term}) {
		t.Errorf("task: %q, want (s-tasks-2) %q", got, term)
	}
}

// C-10 as the frames draw it (from-typecase note 19): a list says a day the
// short way — the columns of a row, «12 Aug» and «12 Aug 18:00» of a task
// (s-tasks-1), «10 Aug» of a note (s-notes-1) — with the year only when it
// is not this year's; the record itself says it in full.
func TestRowColumnsSayTheDayShort(t *testing.T) {
	short := regexp.MustCompile(`^\d{1,2} [A-Z][a-z]{2}( \d{2}:\d{2})?$`)
	for _, c := range []struct{ id, class string }{{"s-tasks-1", "tc-row__due"}, {"s-notes-1", "tc-row__date"}} {
		cells := regexp.MustCompile(`<span class="[^"]*`+c.class+`">([^<]*)</span>`).FindAllStringSubmatch(mockupFrame(t, c.id), -1)
		if len(cells) == 0 {
			t.Fatalf("%s has no %s: read the frame again", c.id, c.class)
		}
		for _, m := range cells {
			if m[1] != "—" && !short.MatchString(m[1]) {
				t.Fatalf("%s writes a day %q: read the frame again", c.id, m[1])
			}
		}
	}
	f := calendarFixture(t, nil,
		"d.ics", ics("BEGIN:VTODO\nUID:d\nDTSTAMP:20260101T000000Z\nSUMMARY:Day\nDUE;VALUE=DATE:20260812\nEND:VTODO"),
		"h.ics", ics("BEGIN:VTODO\nUID:h\nDTSTAMP:20260101T000000Z\nSUMMARY:Hour\nDUE:20260812T180000Z\nEND:VTODO"),
		"y.ics", ics("BEGIN:VTODO\nUID:y\nDTSTAMP:20260101T000000Z\nSUMMARY:Year\nDUE;VALUE=DATE:20250812\nEND:VTODO"),
		"n.ics", ics("BEGIN:VJOURNAL\nUID:n\nDTSTAMP:20260101T000000Z\nDTSTART:20260810T110500Z\nSUMMARY:Note\nEND:VJOURNAL"))
	f.h.srv.SetClock(time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC))
	tasks := decode[view.TasksPage](t, f.h.do(get(f.path("tasks")+"?show=all", f.sess)))
	due := map[string]string{}
	for _, g := range tasks.Groups {
		for _, r := range g.Rows {
			due[string(r.Summary)] = string(r.Due)
		}
	}
	if due["Day"] != "12 Aug" || due["Hour"] != "12 Aug 18:00" || due["Year"] != "12 Aug 2025" {
		t.Errorf("due = %q", due)
	}
	notes := decode[view.NotesPage](t, f.h.do(get(f.path("notes"), f.sess)))
	if len(notes.Groups) != 1 || len(notes.Groups[0].Rows) != 1 || notes.Groups[0].Rows[0].Date != "10 Aug" {
		t.Errorf("notes = %+v", notes.Groups)
	}
}

// from-typecase note 19, account-14: with the scheme off the page of Key
// escrow is its title and one line — no subtitle, no rubric, no button.
func TestEscrowPageWithTheSchemeOffIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "account-14")
	line := frameText(regexp.MustCompile(`(?s)<div class="tc-msg">(.*?)</div>`).FindStringSubmatch(frame)[1])
	if strings.Contains(frame, "tc-head__sub") || strings.Contains(frame, "tc-rubric") || strings.Contains(frame, "<button") || line == "" {
		t.Fatalf("account-14 draws more than a line: read the frame again")
	}
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	h.invite(adminID, "bob", "bobpass123")
	bob := h.login("bob", "bobpass123")
	page := decode[view.EscrowPage](t, h.do(get("/app/settings/escrow", bob)))
	if page.Title != "Key escrow" || page.Subtitle != "" || page.Rubric != "" || page.Confirm != nil ||
		string(page.Status.Note) != line || len(page.Status.Actions) != 0 || page.Status.Badge != nil {
		t.Errorf("escrow off = %+v, want (account-14) %q alone", page, line)
	}
}

// from-typecase note 19, task 33, s-files-16: in From Files a read-only
// storage is a source like any — not dimmed and without the badge a target
// of a move gets: nothing is written into it.
func TestFromFilesReadOnlyStorageIsASource(t *testing.T) {
	frame := mockupFrame(t, "s-files-16")
	// The storage's row carries no words since Typecase 88c2a63 (note 35,
	// Р-37: the server is the head's pick, not a mark of the row).
	row := regexp.MustCompile(`<div class="([^"]*)">(?:<svg>.*?</svg>)?Public(?: <span class="([^"]*)">([^<]*)</span>)?</div>`).FindStringSubmatch(frame)
	if row == nil || strings.Contains(row[1], "op-55") || strings.Contains(row[2], "tc-badge") {
		t.Fatalf("s-files-16 draws Public as %q: read the frame again", row)
	}
	f := attachFixture(t, 1024, []string{"carrel"}, "e1.ics", attachEvent("e1", "Sync-up", ""))
	f.srv.SeedFile(attachFiles+"notes.txt", "small file")
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		for i := range b.Collections {
			if b.Collections[i].Path == attachFiles {
				b.Collections[i].ReadOnly = true
			}
		}
	})
	href := menuAction(actionByID(eventAt(t, f, "e1").Actions, "attach.add").Arrow, "attach.files").Href
	sheet := decode[view.FilePickerSheet](t, f.h.do(get(href+"&to="+url.QueryEscape("/app/files/"+f.id+"/"+url.PathEscape(attachFiles)+"/"), f.sess)))
	if len(sheet.Tree.Nodes) == 0 {
		t.Fatal("no tree")
	}
	for _, n := range sheet.Tree.Nodes {
		if n.ReadOnly {
			t.Errorf("%s: a source marked read-only", n.Label)
		}
	}
}

// from-typecase note 19, task 33, s-files-19: in the upload sheet a refusal
// of New folder stands where it stands in the move sheet — under the line of
// the name, which stays open with what was typed — and the sheet's own line
// stays in its place: the words of s-notes-11.
func TestUploadSheetNewFolderRefusalIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-files-19")
	refusal := frameOne(t, frameAlertMsg, frame, "refusal")[1]
	// The sheet's own line stands in its body; the head has a line of its
	// own since Typecase 4b6e096 — the full path of the folder chosen, beside
	// the pick of the server (note 35, Р-37).
	ownLine := func(frame string) string {
		at := strings.Index(frame, `class="tc-sheet__body"`)
		if at < 0 {
			t.Fatal("the sheet has no body: read the frame again")
		}
		m := frameFootDim.FindStringSubmatch(frame[at:])
		if m == nil {
			t.Fatal("the sheet's body has no line of its own: read the frame again")
		}
		return frameText(m[1])
	}
	stays := ownLine(frame)
	purpose := ownLine(mockupFrame(t, "s-notes-11"))
	m := regexp.MustCompile(`«([^»]+)» in (\S+) already`).FindStringSubmatch(refusal)
	if m == nil || !strings.HasPrefix(purpose, stays) {
		t.Fatalf("s-files-19 refuses %q under %q, s-notes-11 says %q: read the frames again", refusal, stays, purpose)
	}
	typed, in := m[1], m[2]

	f := filesFixture(t, true)
	f.srv.SeedFolder(filesDocs + in + "/" + typed + "/")
	at := f.at("", in, "")
	sheet := decode[view.FilePickerSheet](t, f.h.do(get("/app/files/picker?"+url.Values{"for": {"upload"}, "to": {at}}.Encode(), f.sess)))
	if string(sheet.Purpose) != purpose || sheet.Failure != nil {
		t.Errorf("purpose %q, failure %+v; want the frame's %q and none", sheet.Purpose, sheet.Failure, purpose)
	}
	// The primary button of the sheet is the upload, in the frame's word —
	// «Upload», without a count since Typecase c801772 (note 42: the files
	// picked stand by name in the sheet, the button counts nothing) —;
	// Choose this folder, which the frame does not draw, is not (s-notes-11,
	// s-files-19).
	primary := regexp.MustCompile(`<button class="tc-btn is-primary" type="submit">([^<]+)</button>`)
	word := frameOne(t, primary, mockupFrame(t, "s-notes-11"), "primary upload")[1]
	if again := frameOne(t, primary, frame, "primary upload")[1]; again != word {
		t.Fatalf("s-notes-11 uploads with %q, s-files-19 with %q: read the frames again", word, again)
	}
	if up, ch := actionByID(sheet.Actions, "files.picker.upload"), actionByID(sheet.Actions, "files.picker.choose"); up == nil || !up.Primary || string(up.Label) != word || ch != nil {
		t.Errorf("upload %+v, choose %+v: want the upload primary and named %q, and no Choose this folder, as s-notes-11 draws it", up, ch, word)
	}
	// New opens the line (a GET with new=1, ФГ4); the line sends the name.
	line := decode[view.FilePickerSheet](t, f.h.do(get(sheet.Tree.New.Href, f.sess)))
	if line.Tree.Name == nil {
		t.Fatalf("New did not open the line: %+v", line.Tree)
	}
	taken := decode[view.FilePickerSheet](t, f.h.do(postForm(line.Tree.Name.Action, f.sess, url.Values{"name": {typed}})))
	if taken.Failure == nil || taken.Failure.Role != view.NoticeAlert || taken.Failure.Title != "" || string(taken.Failure.Body) != refusal {
		t.Errorf("refusal = %+v, want (s-files-19) %q", taken.Failure, refusal)
	}
	if taken.Tree.Name != nil {
		sameCaret(t, "s-files-19", "new folder refused", *taken.Tree.Name)
	} else {
		t.Errorf("the refusal closed the line of the name: %+v", taken.Tree)
	}
	if string(taken.Purpose) != purpose {
		t.Errorf("the sheet's line became %q", taken.Purpose)
	}
	if taken.Tree.Name == nil || string(visibleRows(*taken.Tree.Name)[0].Value) != typed {
		t.Errorf("after the refusal the line is %+v, want it open with %q", taken.Tree.Name, typed)
	}
	// The other refusals — a name that cannot be one — stand on the same
	// place in the server's words, the line open.
	bad := decode[view.FilePickerSheet](t, f.h.do(postForm(line.Tree.Name.Action, f.sess, url.Values{"name": {".."}})))
	if bad.Failure == nil || bad.Failure.Role != view.NoticeAlert || string(bad.Purpose) != purpose ||
		bad.Tree.Name == nil || string(visibleRows(*bad.Tree.Name)[0].Value) != ".." {
		t.Errorf("a name that is none: failure %+v, purpose %q, line %+v", bad.Failure, bad.Purpose, bad.Tree.Name)
	}
}

// from-typecase note 19, task 33, s-publish-16: under the title of one
// published record — when and where, «Monday, 10 August 2026, 10:00 – 11:00
// · Переговорная 2», the record's own full day; no lede besides, the badge
// «read-only» says it.
func TestPublicRecordHeadIsTheFrame(t *testing.T) {
	sub := frameText(frameOne(t, regexp.MustCompile(`(?s)<div class="tc-head__sub">(.*?)</div>`), mockupFrame(t, "s-publish-16"), "line under the title")[1])
	when, where, ok := strings.Cut(sub, " · ")
	if !ok || !regexp.MustCompile(`^[A-Z][a-z]+day, \d{1,2} [A-Z][a-z]+ \d{4}, \d{2}:\d{2} – \d{2}:\d{2}$`).MatchString(when) || where == "" {
		t.Fatalf("s-publish-16 = %q: read the frame again", sub)
	}
	f := publishFixture(t)
	// The frame's link shows the place: it was ticked (Б39, the owner's
	// decision of 12.10.2026 — the guest sees what is ticked).
	page := publicPage(t, f, publishWith(t, f, f.publishHref("calendar", searchWork, "e1"), "title", "location"))
	if saidRuns(page.Lede) != "Monday, 10 August 2026, 09:00 · Room 4" || !page.ReadOnly {
		t.Errorf("lede = %q, read-only %v; want «Monday, 10 August 2026, 09:00 · Room 4» as the frame writes it", page.Lede, page.ReadOnly)
	}
}

// from-typecase note 19 (owner's decision): the Time zone row stands on
// every event with a time — a new one too. The new-event form offers the
// list of the edit form with the instance zone chosen, and writes the
// chosen zone as the edit form does when a person changes it: TZID and its
// VTIMEZONE; Floating writes a floating time. A weekly event made at 10:00
// in a zone with summer time stays at 10:00 after the clocks change. The
// quick sheet draws no zone (s-notes-9) and writes in the instance zone.
func TestNewEventIsWrittenInItsZone(t *testing.T) {
	f := calendarFixture(t, func(h *harness) { h.cfg.Timezone = "Europe/Berlin" })
	list := f.path("calendar")
	page := decode[view.EventEditPage](t, f.h.do(get(list+"/new", f.sess)))
	zone := rowNamed(t, page.Form, "tz")
	if zone.Value != "Europe/Berlin" || zone.Kind != view.InputSelect || zone.Label != "Time zone" || zone.Placeholder == "" ||
		selectedOf(zone.Options)[0] != "Europe/Berlin" || zone.Options[len(zone.Options)-1].Value != "floating" {
		t.Fatalf("new event's zone = %+v", zone)
	}
	for _, o := range zone.Options {
		if o.Group != "" {
			t.Errorf("a new event came in no zone, and the list says one: %+v", o)
		}
	}
	create := func(set map[string]string) string {
		t.Helper()
		form := formFields(decode[view.EventEditPage](t, f.h.do(get(list+"/new", f.sess))).Form)
		for k, v := range set {
			form.Set(k, v)
		}
		rec := f.h.do(postForm(list+"/new", f.sess, form))
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("create %v: %d %s", set, rec.Code, rec.Body.String())
		}
		loc := rec.Header().Get("Location")
		return loc[strings.LastIndex(loc, "/")+1:]
	}
	weekly := create(map[string]string{"summary": "Standup", "start.date": "2026-03-16", "start.time": "10:00",
		"end.date": "2026-03-16", "end.time": "10:30", "repeat.freq": "WEEKLY", "repeat.interval": "1"})
	got := body(t, f, weekly+".ics")
	mustContain(t, got, "DTSTART;TZID=Europe/Berlin:20260316T100000", "DTEND;TZID=Europe/Berlin:20260316T103000",
		"BEGIN:VTIMEZONE", "TZID:Europe/Berlin", "RRULE:FREQ=WEEKLY")
	// After the clocks went forward on 29 March: still 10:00 in Berlin.
	agenda := decode[view.AgendaPage](t, f.h.do(get(list+"?from=2026-04-06", f.sess)))
	times := []string{}
	for _, d := range agenda.Days {
		for _, r := range d.Rows {
			if r.UID == weekly {
				times = append(times, string(r.Time))
			}
		}
	}
	if len(times) != 1 || times[0] != "10:00" {
		t.Errorf("the week after the change shows it at %q, want 10:00", times)
	}

	floating := create(map[string]string{"summary": "Anywhere", "start.date": "2026-03-16", "start.time": "10:00", "tz": "floating"})
	got = body(t, f, floating+".ics")
	mustContain(t, got, "DTSTART:20260316T100000")
	if strings.Contains(got, "TZID") || strings.Contains(got, "T100000Z") {
		t.Errorf("a floating event:\n%s", got)
	}
	// The clock typed is the clock in the zone chosen (s-agenda-3, -15).
	utc := create(map[string]string{"summary": "Zulu", "start.date": "2026-03-16", "start.time": "10:00", "tz": "UTC"})
	mustContain(t, body(t, f, utc+".ics"), "DTSTART:20260316T100000Z")

	// The quick sheet: no zone row, the instance zone written.
	quick := decode[view.NewSheet](t, f.h.do(get(list+"/new/quick", f.sess)))
	for _, set := range quick.Form.Sets {
		for _, r := range set.Rows {
			if r.Name == "tz" {
				t.Errorf("the quick sheet draws a zone: %+v", r)
			}
		}
	}
	qform := formFields(quick.Form)
	qform.Set("summary", "Quick")
	qform.Set("start.date", "2026-03-16")
	qform.Set("start.time", "10:00")
	rec := f.h.do(postForm(list+"/new/quick", f.sess, qform))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("quick: %d %s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	mustContain(t, body(t, f, loc[strings.LastIndex(loc, "/")+1:]+".ics"), "DTSTART;TZID=Europe/Berlin:20260316T100000", "BEGIN:VTIMEZONE")
}
