package web_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

// settingsForm is the one form of the instance's settings as the page sends
// it, with what set says changed.
func settingsForm(t *testing.T, h *harness, sess *session.Session, set map[string]string) url.Values {
	t.Helper()
	form := formFields(decode[view.AdminSettingsPage](t, h.do(get("/admin/settings", sess))).Form)
	for k, v := range set {
		form.Set(k, v)
	}
	return form
}

// Owner's decision 19.09.2026: the name, the lifetimes, people's mail and
// its ceilings are settings of the screen; zero keeps the default, and a
// value out of range is refused on its field. Since the scan of 15.10.2026
// (Ад17, s-admin-3) they are one form with the relay: Save stores all of
// it or nothing, every refusal at its row at once, Discard is the page
// again, and the journal has one entry naming the rows that changed (Ад38).
func TestInstanceSettingsAreEdited(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	h.relayed()
	peopleOn(t, h)

	page := decode[view.AdminSettingsPage](t, h.do(get("/admin/settings", alice)))
	f := formFields(page.Form)
	if f.Get("session") != "30" || f.Get("inviteTTL") != "72" || f.Get("idle") != "12" || f.Get("name") != "Carrel" ||
		f.Get("attach") != "10" || f.Get("recipients") != "20" {
		t.Errorf("defaults = %v", f)
	}
	var legends []string
	for _, s := range page.Form.Sets {
		legends = append(legends, string(s.Legend))
	}
	if strings.Join(legends, ", ") != "Global, Mail, Letters from users, Test" || page.Subtitle != "Global settings and mail" ||
		page.Discard.Href != "/admin/settings" || page.Discard.Method != "GET" {
		t.Errorf("rubrics %q, subtitle %q, discard %+v", legends, page.Subtitle, page.Discard)
	}

	rec := h.do(postForm("/admin/settings", alice, settingsForm(t, h, alice, map[string]string{"name": "Контора", "inviteTTL": "24", "idle": "2", "session": "7"})))
	if rec.Code != 303 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body)
	}
	set := h.users.Settings()
	if set.Name != "Контора" || set.InviteHours != 24 || set.IdleHours != 2 || set.SessionDays != 7 {
		t.Errorf("stored = %+v", set)
	}
	snap := h.st.Snapshot()
	last := snap.Audit[len(snap.Audit)-1]
	if last.What != "settings.instance" || last.Detail != "inviteTTL, idle, session, name" {
		t.Errorf("journal = %+v, want the names of the rows changed", last)
	}
	if _, err := h.users.CreateInvite(adminID, "bob", ""); err != nil {
		t.Fatal(err)
	}
	snap = h.st.Snapshot()
	for _, inv := range snap.Invites {
		if inv.Login == "bob" && inv.ExpiresAt.Sub(inv.CreatedAt) != 24*time.Hour {
			t.Errorf("invitation lives %v", inv.ExpiresAt.Sub(inv.CreatedAt))
		}
	}
	// Idle applies to a session already open.
	if !alice.Expired(time.Now().Add(2*time.Hour + time.Minute)) {
		t.Error("the new idle timeout did not reach a live session")
	}
	login := decode[view.LoginPage](t, h.do(get("/login", nil)))
	if login.Title != "Sign in to Контора" {
		t.Errorf("sign-in title = %q", login.Title)
	}

	// Every refusal at its row at once, and nothing stored.
	page = decode[view.AdminSettingsPage](t, h.do(postForm("/admin/settings", alice,
		settingsForm(t, h, alice, map[string]string{"attach": "30", "letter": "20", "idle": "500", "name": "Другая"}))))
	refused := map[string]bool{}
	for _, s := range page.Form.Sets {
		for _, row := range s.Rows {
			if row.Error != "" {
				refused[row.Name] = true
			}
		}
	}
	if !refused["attach"] || !refused["idle"] || len(refused) != 2 {
		t.Errorf("refused = %v, want attach and idle at once", refused)
	}
	if h.users.Settings().Name != "Контора" {
		t.Error("a refused save stored a part of the form")
	}

	rec = h.do(postForm("/admin/settings", alice, settingsForm(t, h, alice, map[string]string{"people": "on", "attach": "1", "letter": "20", "recipients": "5", "perDay": "9"})))
	if rec.Code != 303 {
		t.Fatalf("limits: %d %s", rec.Code, rec.Body)
	}
	sheet := decode[view.MailSheet](t, h.do(get("/app/mail/compose", alice)))
	if !strings.HasPrefix(letterHint(sheet), "0 of 5 recipients.") {
		t.Errorf("the line under To = %q", letterHint(sheet))
	}
	if h.users.Settings().Name != "Контора" {
		t.Error("the letters' rubric changed the Global one")
	}
}

// Ад18–Ад22 of the scan of 15.10.2026 (s-admin-3): the rubrics say when the
// relay is not set up and people's letters are off; the relay's fields have
// the frame's samples; the sender of people's letters stands with them; the
// ranges are not said beside the fields; the test letter is off with the
// frame's words while there is no relay.
func TestInstanceSettingsAreTheFrame(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	page := decode[view.AdminSettingsPage](t, h.do(get("/admin/settings", alice)))
	sets := map[string]view.FieldSet{}
	for _, s := range page.Form.Sets {
		sets[string(s.Legend)] = s
	}
	if b := sets["Mail"].Badges; len(b) != 1 || b[0].Label != "not configured" || b[0].Role != view.BadgeAlert {
		t.Errorf("Mail badges = %+v", b)
	}
	if b := sets["Letters from users"].Badges; len(b) != 1 || b[0].Label != "off" {
		t.Errorf("letters badges = %+v", b)
	}
	samples := map[string]string{}
	for _, r := range sets["Mail"].Rows {
		samples[r.Name] = string(r.Placeholder)
	}
	if samples["host"] != "smtp.example.org" || samples["password"] != "leave blank to keep the current one" || samples["from"] != "carrel@example.org" || samples["fromName"] != "Carrel" {
		t.Errorf("samples = %v", samples)
	}
	// Without mail the tick is off with its reason, and nothing else of
	// people's letters is shown (Typecase's note 88).
	var letters []string
	for _, r := range sets["Letters from users"].Rows {
		letters = append(letters, r.Name)
	}
	if rows := sets["Letters from users"].Rows; strings.Join(letters, " ") != "people" || rows[0].Reason == "" {
		t.Errorf("letters without mail = %q %+v, want the tick alone, off with its reason", letters, rows)
	}
	h.relayed()
	peopleOn(t, h)
	page = decode[view.AdminSettingsPage](t, h.do(get("/admin/settings", alice)))
	letters = nil
	for _, s := range page.Form.Sets {
		if s.Legend == "Letters from users" {
			for _, r := range s.Rows {
				letters = append(letters, r.Name)
			}
			if len(s.Badges) != 1 || s.Badges[0].Label != "on" {
				t.Errorf("letters on: badges = %+v", s.Badges)
			}
		}
	}
	if strings.Join(letters, " ") != "people sender attach letter recipients perDay" {
		t.Errorf("letters = %q, want the sender after the box", letters)
	}
	for _, r := range sets["Global"].Rows {
		if r.Hint != "" && r.Name != "publicURL" {
			t.Errorf("%s says %q beside it", r.Name, r.Hint)
		}
	}
	var send *view.Action
	for i := range page.Form.Actions {
		if page.Form.Actions[i].ID == "admin.settings.mailtest" {
			send = &page.Form.Actions[i]
		}
	}
	// Never off (Typecase's note 82): it sends what is typed.
	if send == nil || !send.Enabled || send.Reason != "" || !send.Submits || send.Href != "/admin/settings/mail-test" {
		t.Errorf("Send test = %+v", send)
	}
}

// peopleOn turns people's letters on, as a Save of the tick does.
func peopleOn(t *testing.T, h *harness) {
	t.Helper()
	on := true
	if err := h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		st.Settings.PeopleMail = &on
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
}
