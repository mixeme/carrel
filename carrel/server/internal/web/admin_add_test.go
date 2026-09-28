package web_test

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

// The owner's deferred round, part 5: «Add users» (s-admin-1) is three
// cards — an invitation the person accepts with a password of their own, a
// temporary password handed over in person, and self-registration — and
// the role (s-admin-8) is chosen on each way in. Key escrow (s-admin-6)
// names whom the master password opens, and a recovery takes the temporary
// password from the administrator. Every string compared is cut out of the
// frames.

var (
	frameCardStart = `<div class="tc-card">`
	frameCardPlain = regexp.MustCompile(`(?s)<span class="tc-plain">(.*?)</span>`)
	frameCardLede  = regexp.MustCompile(`(?s)<div class="tc-card__lede">(.*?)</div>`)
	frameCardAlert = regexp.MustCompile(`(?s)<div class="tc-msg is-alert[^"]*">(.*?)</div>`)
	frameCardBtn   = regexp.MustCompile(`(?s)<button class="tc-btn[^"]*"[^>]*>(.*?)</button>`)
	frameCardCheck = regexp.MustCompile(`(?s)<button class="tc-check"[^>]*>(.*?)</button>`)
	frameCardPick  = regexp.MustCompile(`(?s)<button class="tc-in is-pick"[^>]*>(.*?)</button>`)
	frameMsgAside  = regexp.MustCompile(`(?s)<span class="tc-msg__aside">(.*?)</span>`)
	frameRoleItem  = regexp.MustCompile(`(?s)<button class="[^"]*tc-menu__item[^"]*"[^>]*>(.*?)</button>`)
	frameAnyHint   = regexp.MustCompile(`(?s)<span class="tc-field__hint">(.*?)</span>`)
)

type frameCard struct {
	title, aside, badge, badgeRole, lede, warning, role string
	labels, checks, buttons                             []string
}

// readFrameCards are the cards of s-admin-1, in their order.
func readFrameCards(t *testing.T) []frameCard {
	t.Helper()
	frame := mockupFrame(t, "s-admin-1")
	if end := strings.Index(frame, `<div class="tc-list__group">`); end > 0 {
		frame = frame[:end]
	}
	parts := strings.Split(frame, frameCardStart)[1:]
	if len(parts) != 3 {
		t.Fatalf("s-admin-1 draws %d cards, want three: read the frame again", len(parts))
	}
	var out []frameCard
	for _, p := range parts {
		var c frameCard
		rubric := frameRubric.FindStringSubmatch(p)[1]
		c.title = frameText(rubric[:strings.Index(rubric+"<span", "<span")])
		if m := frameCardPlain.FindStringSubmatch(rubric); m != nil {
			c.aside = frameText(m[1])
		}
		if m := frameBadge.FindStringSubmatch(rubric); m != nil {
			c.badgeRole, c.badge = m[1], frameText(m[2])
		}
		if m := frameCardLede.FindStringSubmatch(p); m != nil {
			c.lede = frameText(m[1])
		}
		if m := frameCardAlert.FindStringSubmatch(p); m != nil {
			c.warning = frameText(m[1])
		}
		if m := frameCardPick.FindStringSubmatch(p); m != nil {
			c.role = frameText(m[1])
		}
		c.labels = frameAll(frameLabel, p, 1)
		c.checks = frameAll(frameCardCheck, p, 1)
		c.buttons = frameAll(frameCardBtn, p, 1)
		out = append(out, c)
	}
	return out
}

// checkCard compares a card the server sent with the frame's.
func checkCard(t *testing.T, got view.AddUserCard, want frameCard) {
	t.Helper()
	what := want.title
	if string(got.Title) != want.title || string(got.Aside) != want.aside || string(got.Lede) != want.lede {
		t.Errorf("%s: title, aside, lede = %q, %q, %q, want %q, %q, %q", what, got.Title, got.Aside, got.Lede, want.title, want.aside, want.lede)
	}
	switch {
	case want.badge == "" && got.Badge != nil:
		t.Errorf("%s: badge %+v, the frame draws none", what, got.Badge)
	case want.badge != "" && (got.Badge == nil || string(got.Badge.Label) != want.badge || got.Badge.Role != badgeRole(want.badgeRole)):
		t.Errorf("%s: badge %+v, want %q (%s)", what, got.Badge, want.badge, badgeRole(want.badgeRole))
	}
	switch {
	case want.warning == "" && got.Warning != nil:
		t.Errorf("%s: warning %+v, the frame draws none", what, got.Warning)
	case want.warning != "" && (got.Warning == nil || string(got.Warning.Body) != want.warning || got.Warning.Role != view.NoticeAlert):
		t.Errorf("%s: warning %+v, want %q", what, got.Warning, want.warning)
	}
	var fields, checks []string
	var role *view.FormRow
	for _, set := range got.Form.Sets {
		for i, row := range set.Rows {
			switch row.Kind {
			case view.InputHidden:
			case view.InputCheck:
				checks = append(checks, string(row.Label))
			default:
				fields = append(fields, string(row.Label))
			}
			if row.Name == "role" {
				role = &set.Rows[i]
			}
		}
	}
	if !equalStrings(fields, want.labels) || !equalStrings(checks, want.checks) {
		t.Errorf("%s: fields %q, checks %q, want %q, %q", what, fields, checks, want.labels, want.checks)
	}
	buttons := labels(got.Form.Actions)
	if got.Copy != nil {
		buttons = append(buttons, string(got.Copy.Label))
		if got.Link == "" || got.Copy.Href != string(got.Link) || got.Copy.Method != http.MethodGet {
			t.Errorf("%s: Copy %+v for the link %q", what, got.Copy, got.Link)
		}
	}
	wantButtons := slices.Clone(want.buttons)
	slices.Sort(buttons)
	slices.Sort(wantButtons)
	if !equalStrings(buttons, wantButtons) {
		t.Errorf("%s: buttons %q, want %q", what, buttons, wantButtons)
	}
	sayWhy(t, what, got.Form.Actions...)
	if want.role == "" {
		return
	}
	// The role is the list of s-admin-8, the frame's choice already made.
	var options []string
	chosen := ""
	for _, o := range role.Options {
		options = append(options, string(o.Label))
		if o.Selected {
			chosen = string(o.Label)
		}
	}
	// The chosen item is drawn is-on before its class, which frameMenu does
	// not read.
	roles := frameAll(frameRoleItem, mockupFrame(t, "s-admin-8"), 1)
	if role.Kind != view.InputSelect || !equalStrings(options, roles) || chosen != want.role {
		t.Errorf("%s: role %s %q chosen %q, want a list %q with %q", what, role.Kind, options, chosen, roles, want.role)
	}
}

func TestAddUsersIsTheFrame(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	// Mail works and registration is open: the frame draws the card «open»
	// with its address.
	h.relayed()
	if rec := h.do(postForm("/admin/invites", alice, url.Values{"action": {"selfreg"}, "selfReg": {"on"}})); rec.Code != http.StatusSeeOther {
		t.Fatalf("open registration: %d %s", rec.Code, rec.Body)
	}
	// The link just made stands in the card that made it, with Copy.
	rec := h.do(postForm("/admin/invites?action=link", alice, url.Values{"role": {"user"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create link: %d %s", rec.Code, rec.Body)
	}
	page := decode[view.AdminInvitesPage](t, h.do(get(rec.Header().Get("Location"), alice)))
	frame := mockupFrame(t, "s-admin-1")
	if sub := frameAll(frameHeadSub, frame, 1); len(sub) == 0 || string(page.Subtitle) != sub[0] {
		t.Errorf("subtitle = %q, want %q", page.Subtitle, sub)
	}
	if page.Title != "Add users" {
		t.Errorf("title = %q, want the rail's and the frame's «Add users»", page.Title)
	}
	cards := readFrameCards(t)
	checkCard(t, page.Invite, cards[0])
	checkCard(t, page.Temporary, cards[1])
	checkCard(t, page.SelfReg, cards[2])
	if !strings.Contains(string(page.Invite.Link), "/invite/") {
		t.Errorf("invitation link = %q", page.Invite.Link)
	}
	if !strings.HasSuffix(string(page.SelfReg.Link), "/register") {
		t.Errorf("registration link = %q", page.SelfReg.Link)
	}
	// The temporary password is offered, not left to invent.
	if pw := rowValue(page.Temporary.Form, "password"); len(pw) < 8 {
		t.Errorf("suggested temporary password = %q", pw)
	}
	// The Users page leads here with the frame's «Add user»; the
	// registration switch is no longer there.
	users := decode[view.AdminUsersPage](t, h.do(get("/admin/", alice)))
	if len(users.Add) != 1 || users.Add[0].Label != "Add user" || users.Add[0].Href != "/admin/invites" {
		t.Errorf("users head = %+v, want Add user to /admin/invites (s-admin-2)", users.Add)
	}
}

func rowValue(f view.Form, name string) string {
	for _, set := range f.Sets {
		for _, row := range set.Rows {
			if row.Name == name {
				return string(row.Value)
			}
		}
	}
	return ""
}

// Registration cannot be turned on while no letter can go (s-admin-1:
// «Cannot be turned on until mail works.»): Save says why.
func TestSelfRegistrationWaitsForMail(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	page := decode[view.AdminInvitesPage](t, h.do(get("/admin/invites", alice)))
	save := actionByID(page.SelfReg.Form.Actions, "admin.selfreg.save")
	if save == nil || save.Enabled || save.Reason == "" {
		t.Errorf("Save without mail = %+v, want off with the reason", save)
	}
	if page.SelfReg.Badge != nil || page.SelfReg.Link != "" {
		t.Errorf("closed registration: badge %+v, link %q", page.SelfReg.Badge, page.SelfReg.Link)
	}
	h.do(postForm("/admin/invites", alice, url.Values{"action": {"selfreg"}, "selfReg": {"on"}}))
	if snap := h.st.Snapshot(); snap.Settings.SelfRegEnabled {
		t.Error("registration opened without mail")
	}
	// Send invitation is off without mail, with the reason, and the server
	// does not take it either: no invitation is left behind unsent.
	send := actionByID(page.Invite.Form.Actions, "admin.invite.send")
	if send == nil || send.Enabled || send.Reason == "" {
		t.Errorf("Send invitation without mail = %+v", send)
	}
	sent := decode[view.AdminInvitesPage](t, h.do(postForm("/admin/invites?action=send", alice, url.Values{"email": {"anna@example.org"}})))
	if row := rowByName(sent.Invite.Form, "email"); row == nil || row.Error == "" {
		t.Errorf("send without mail: %+v", sent.Invite.Form)
	}
	if snap := h.st.Snapshot(); len(snap.Invites) != 0 {
		t.Errorf("invitations = %+v, want none", snap.Invites)
	}
}

// An invitation names no login: the person chooses it with the password
// (s-auth-2 serves /invite/{token} too), and gets the role it was made with.
func TestInvitationCarriesItsRole(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	rec := h.do(postForm("/admin/invites?action=link", alice, url.Values{"role": {"admin"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create link: %d %s", rec.Code, rec.Body)
	}
	snap := h.st.Snapshot()
	if len(snap.Invites) != 1 || snap.Invites[0].Role != store.RoleAdmin || snap.Invites[0].Login != "" {
		t.Fatalf("invites = %+v, want one for an administrator, no login", snap.Invites)
	}
	secret := snap.Invites[0].Secret
	inv := decode[view.InvitePage](t, h.do(get("/invite/"+secret, nil)))
	login := rowByName(inv.Form, "login")
	if login == nil || login.ReadOnly || !login.Required || login.Label != "Login" {
		t.Fatalf("invite form login row = %+v, want Login to type", login)
	}
	// A login already taken is said on its field; the invitation lives on.
	form := url.Values{"login": {"alice"}, "password": {"verapass123"}, "password2": {"verapass123"}}
	taken := decode[view.InvitePage](t, h.do(h.anonPostForm("/invite/"+secret, "/invite/"+secret, form)))
	if row := rowByName(taken.Form, "login"); row == nil || row.Error == "" {
		t.Fatalf("taken login: %+v", taken.Form)
	}
	form.Set("login", "Vera")
	if rec := h.do(h.anonPostForm("/invite/"+secret, "/invite/"+secret, form)); rec.Code != http.StatusSeeOther {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body)
	}
	snap = h.st.Snapshot()
	u := snap.UserByLogin("vera")
	if u == nil || u.Role != store.RoleAdmin || len(u.LoginHash) == 0 {
		t.Fatalf("vera = %+v, want an administrator with a password", u)
	}
	if _, _, mustChange, ok := h.users.Authenticate(context.Background(), "vera", "verapass123"); !ok || mustChange {
		t.Fatalf("vera signs in: ok %v, must change %v", ok, mustChange)
	}
}

func rowByName(f view.Form, name string) *view.FormRow {
	for _, set := range f.Sets {
		for i := range set.Rows {
			if set.Rows[i].Name == name {
				return &set.Rows[i]
			}
		}
	}
	return nil
}

// The temporary password (s-admin-1, S-10.2): the account is made at once
// with the role and the password the administrator hands over; the first
// sign-in asks for a new one. The password is shown once and goes nowhere
// else — not into the journal.
func TestTemporaryPasswordMakesTheAccount(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	const temp = "9f2-brick-lamp"
	form := url.Values{"role": {"admin"}, "login": {"Viktor"}, "email": {"v@example.org"}, "password": {temp}}
	rec := h.do(postForm("/admin/invites?action=user", alice, form))
	if rec.Code != http.StatusOK {
		t.Fatalf("create user: %d %s", rec.Code, rec.Body)
	}
	page := decode[view.AdminInvitesPage](t, rec)
	issued := page.Temporary.Issued
	if issued == nil || issued.Login != "viktor" || issued.Password != temp || issued.Copy.Label != "Copy" {
		t.Fatalf("issued = %+v, want viktor's password once", issued)
	}
	if next := rowValue(page.Temporary.Form, "password"); next == temp || next == "" {
		t.Errorf("the form offers %q again for the next person", next)
	}
	snap := h.st.Snapshot()
	u := snap.UserByLogin("viktor")
	if u == nil || u.Role != store.RoleAdmin || !u.MustChangePassword {
		t.Fatalf("viktor = %+v, want an administrator who must change the password", u)
	}
	// The address is the administrator's word, not proven: it waits.
	if u.Email != "" || u.PendingEmail != "v@example.org" {
		t.Errorf("address = %q, pending %q", u.Email, u.PendingEmail)
	}
	if _, _, mustChange, ok := h.users.Authenticate(context.Background(), "viktor", temp); !ok || !mustChange {
		t.Fatalf("viktor signs in: ok %v, must change %v", ok, mustChange)
	}
	made := false
	for _, e := range snap.Audit {
		if strings.Contains(e.Who+e.What+e.Whom+e.Detail, temp) {
			t.Errorf("journal line %+v carries the password", e)
		}
		if e.What == "user.create" && e.Whom == "viktor" && e.Who == "alice" {
			made = true
		}
	}
	if !made {
		t.Error("no user.create line in the journal")
	}
	// The login is taken now; the answer says so on the field.
	rec = h.do(postForm("/admin/invites?action=user", alice, form))
	again := decode[view.AdminInvitesPage](t, rec)
	if row := rowByName(again.Temporary.Form, "login"); row == nil || row.Error == "" || again.Temporary.Issued != nil {
		t.Errorf("second viktor: %+v", again.Temporary)
	}
	// Too short a password is refused on its field.
	form.Set("login", "irina")
	form.Set("password", "short")
	short := decode[view.AdminInvitesPage](t, h.do(postForm("/admin/invites?action=user", alice, form)))
	if row := rowByName(short.Temporary.Form, "password"); row == nil || row.Error == "" {
		t.Errorf("short password: %+v", short.Temporary.Form)
	}
	if snap := h.st.Snapshot(); snap.UserByLogin("irina") != nil {
		t.Error("irina made with a short password")
	}
}

// s-admin-6: whom the master password opens — every account, with the day
// of its copy and of its last recovery; the hints are the frame's; a
// recovery takes the temporary password from the administrator.
func TestAdminEscrowIsTheFrame(t *testing.T) {
	h := newHarness(t)
	const master = "sixteen-char-pass!"
	h.enableEscrow("alice", "password123", master)
	alice := h.login("alice", "password123")
	snap := h.st.Snapshot()
	h.invite(snap.UserByLogin("alice").ID, "bob", "bobpass123")
	frame := mockupFrame(t, "s-admin-6")
	cells := frameCells(t, "s-admin-6")
	day := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	moment := regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}$`)

	page := decode[view.AdminEscrowPage](t, h.do(get("/admin/escrow", alice)))
	if want := "1 of 2 accounts open with the master password"; string(page.Subtitle) != want {
		t.Errorf("subtitle = %q, want %q (s-admin-6: «%s»)", page.Subtitle, want, frameAll(frameHeadSub, frame, 1))
	}
	if aside := frameAll(frameMsgAside, frame, 1); len(aside) == 0 || string(page.Warning.Body) != aside[0] || page.Warning.Title != "" {
		t.Errorf("warning = %q / %q, want %q", page.Warning.Title, page.Warning.Body, aside)
	}
	rows := map[string]view.EscrowCoverRow{}
	for _, r := range page.Covered {
		rows[string(r.Login)] = r
	}
	if len(rows) != 2 || page.Opens != 1 {
		t.Fatalf("covered = %+v, opens %d; want both accounts, one open", page.Covered, page.Opens)
	}
	// mix of the frame: not deposited, a dash for the recovery.
	if a := rows["alice"]; a.Covered || string(a.Deposited) != cells[2][1] || string(a.LastRecovery) != cells[2][2] {
		t.Errorf("alice = %+v, want %q, %q", a, cells[2][1], cells[2][2])
	}
	// anna of the frame: the day of the copy, never recovered.
	if b := rows["bob"]; !b.Covered || !day.MatchString(string(b.Deposited)) || string(b.LastRecovery) != cells[0][2] {
		t.Errorf("bob = %+v, want a day and %q", b, cells[0][2])
	}
	// The rubrics and their hints, in the frame's order.
	hints := frameAll(frameAnyHint, frame, 1)
	for i, f := range []view.Form{page.Form, page.Master, page.Recover} {
		legend := []string{"Policy", "Master password", "Recover an account"}[i]
		if len(f.Sets) == 0 || string(f.Sets[0].Legend) != legend {
			t.Errorf("form %d legend = %+v, want %q", i, f.Sets, legend)
		}
		found := false
		for _, set := range f.Sets {
			for _, row := range set.Rows {
				found = found || (i < len(hints) && string(row.Hint) == hints[i])
			}
		}
		if !found {
			t.Errorf("%s: no hint %q", legend, hints[i])
		}
	}
	var recoverLabels []string
	for _, row := range page.Recover.Sets[0].Rows {
		if row.Kind != view.InputHidden {
			recoverLabels = append(recoverLabels, string(row.Label))
		}
	}
	if want := frameAll(frameLabel, frameBlock(t, frame, "Recover an account"), 1); !equalStrings(recoverLabels, want) {
		t.Errorf("recover fields = %q, want %q", recoverLabels, want)
	}
	if rec := actionByID(page.Recover.Actions, "escrow.recover"); rec == nil || !rec.Danger {
		t.Errorf("Recover = %+v, want it marked (is-danger)", rec)
	}
	if len(rowValue(page.Recover, "temp")) < 8 {
		t.Errorf("no temporary password offered: %q", rowValue(page.Recover, "temp"))
	}

	const temp = "9f2-brick-lamp"
	rec := h.do(postForm("/admin/escrow", alice, url.Values{"action": {"recover"}, "login": {"bob"}, "master": {master}, "temp": {temp}}))
	done := decode[view.AdminEscrowPage](t, rec)
	if done.Result == nil || done.Result.Password != temp {
		t.Fatalf("result = %+v, want the password typed", done.Result)
	}
	if _, _, mustChange, ok := h.users.Authenticate(context.Background(), "bob", temp); !ok || !mustChange {
		t.Fatalf("bob with the temporary password: ok %v, must change %v", ok, mustChange)
	}
	page = decode[view.AdminEscrowPage](t, h.do(get("/admin/escrow", alice)))
	for _, r := range page.Covered {
		if r.Login == "bob" && !moment.MatchString(string(r.LastRecovery)) {
			t.Errorf("bob's last recovery = %q, want the day and time as «%s»", r.LastRecovery, cells[1][2])
		}
	}
	for _, e := range h.st.Snapshot().Audit {
		if strings.Contains(e.Detail, temp) {
			t.Errorf("journal line %+v carries the password", e)
		}
	}
}
