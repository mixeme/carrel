package web_test

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

// The account page read off its frames: account-1 (the page), account-8
// (confirmed), s-mail-9 (not confirmed, no letter), account-9 (the letter
// went), account-7 and account-2 (the key deposited, the deposit sheet),
// account-4 (the escrow page). Every string compared below is cut out of the
// frame, not written beside it.

var (
	frameHeadSub  = regexp.MustCompile(`(?s)<div class="tc-head__sub">(.*?)</div>`)
	frameRubric   = regexp.MustCompile(`(?s)<div class="tc-rubric">(.*?)</div>`)
	frameHint     = regexp.MustCompile(`(?s)<span class="tc-field__hint">(.*?)</span>\s*(?:</div>|<span class="tc-line">|<div class="tc-msg)`)
	frameLabel    = regexp.MustCompile(`(?s)<label>(.*?)</label>`)
	frameMsg      = regexp.MustCompile(`(?s)<div class="tc-msg is-(\w+)[^"]*">(.*?)(?:<button[^>]*>(.*?)</button>)?</div>`)
	frameBadge    = regexp.MustCompile(`(?s)<span class="tc-badge(?: is-(\w+))?">(.*?)</span>`)
	frameChange   = regexp.MustCompile(`(?s)<input class="tc-in tc-in--fill" type="text" value="([^"]*)"><button class="tc-btn" type="button">(.*?)</button>`)
	frameNameSave = regexp.MustCompile(`(?s)<input class="tc-in tc-in--fill" type="text" value="[^"]*" aria-label="Display name"><button class="tc-btn" type="button">(.*?)</button>`)
	frameFieldBt  = regexp.MustCompile(`(?s)<div class="tc-field"><button class="tc-btn[^"]*" type="button">(.*?)</button></div>`)
	framePair     = regexp.MustCompile(`(?s)<dt>(.*?)</dt><dd[^>]*>(.*?)</dd>`)
	frameFootDim  = regexp.MustCompile(`(?s)<span class="tc-dim[^"]*tc-dim--wrap">(.*?)</span>`)
	frameSheetT   = regexp.MustCompile(`(?s)<b class="tc-sheet__title">(.*?)</b>`)
	frameSheetF   = regexp.MustCompile(`(?s)<div class="tc-sheet__foot">(.*?)</div>`)
)

func frameAll(re *regexp.Regexp, s string, group int) []string {
	var out []string
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		out = append(out, frameText(m[group]))
	}
	return out
}

// frameBlock is the part of a frame from one rubric to the next.
func frameBlock(t *testing.T, frame, rubric string) string {
	t.Helper()
	at := strings.Index(frame, `<div class="tc-rubric">`+rubric)
	if at < 0 {
		t.Fatalf("the frame has no rubric %q", rubric)
	}
	rest := frame[at+1:]
	if end := strings.Index(rest, `<div class="tc-fset`); end >= 0 {
		rest = rest[:end]
	}
	return rest
}

type frameEmail struct {
	label, value, change, hint string
	badge, badgeRole           string
	msg, msgRole, button       string
}

func readFrameEmail(t *testing.T, id string) frameEmail {
	t.Helper()
	block := frameBlock(t, mockupFrame(t, id), "Email")
	var f frameEmail
	if m := frameLabel.FindStringSubmatch(block); m != nil {
		f.label = frameText(m[1])
	}
	if m := frameChange.FindStringSubmatch(block); m != nil {
		f.value, f.change = m[1], frameText(m[2])
	}
	if m := frameHint.FindStringSubmatch(block); m != nil {
		f.hint = frameText(m[1])
	}
	if m := frameBadge.FindStringSubmatch(block); m != nil {
		f.badgeRole, f.badge = m[1], frameText(m[2])
	}
	if m := frameMsg.FindStringSubmatch(block); m != nil {
		// An address is set in mono inside the sentence: its closing tag is
		// not a word break («new@example.org: until…», account-10).
		f.msgRole, f.msg, f.button = m[1], frameText(strings.ReplaceAll(m[2], "</span>", "")), frameText(m[3])
	}
	if f.label == "" || f.value == "" || f.hint == "" {
		t.Fatalf("%s: the Email block did not read: %+v", id, f)
	}
	return f
}

func badgeRole(frameRole string) view.BadgeRole {
	if frameRole == "" {
		return view.BadgeNeutral
	}
	return view.BadgeRole(frameRole)
}

// checkEmailBlock compares the Email block of the page with a frame's.
func checkEmailBlock(t *testing.T, id string, page view.AccountPage, state view.EmailState) {
	t.Helper()
	want := readFrameEmail(t, id)
	row := profileRow(page.Profile, "email")
	if string(row.Label) != want.label || string(row.Value) != want.value || string(row.Hint) != want.hint {
		t.Errorf("%s: Address row = %q %q %q, the frame draws %q %q %q", id, row.Label, row.Value, row.Hint, want.label, want.value, want.hint)
	}
	if row.Extra == nil || string(row.Extra.Label) != want.change {
		t.Errorf("%s: the field's button = %+v, the frame draws %q", id, row.Extra, want.change)
	}
	got := page.Email
	if got.State != state {
		t.Errorf("%s: state = %q, want %q", id, got.State, state)
	}
	switch {
	case want.badge == "" && got.Badge != nil:
		t.Errorf("%s: a badge %+v the frame does not draw", id, *got.Badge)
	case want.badge != "" && (got.Badge == nil || string(got.Badge.Label) != want.badge || got.Badge.Role != badgeRole(want.badgeRole)):
		t.Errorf("%s: badge = %+v, the frame draws %q (%s)", id, got.Badge, want.badge, want.badgeRole)
	}
	switch {
	case want.msg == "" && got.Message != nil:
		t.Errorf("%s: a message %+v the frame does not draw", id, *got.Message)
	case want.msg != "":
		if got.Message == nil {
			t.Errorf("%s: no message, the frame draws %q", id, want.msg)
			break
		}
		if string(got.Message.Body) != want.msg || got.Message.Role != view.NoticeRole(want.msgRole) || got.Message.Title != "" {
			t.Errorf("%s: message = %q (%s, title %q), the frame draws %q (%s)", id, got.Message.Body, got.Message.Role, got.Message.Title, want.msg, want.msgRole)
		}
		if len(got.Message.Actions) != 1 || string(got.Message.Actions[0].Label) != want.button {
			t.Errorf("%s: message buttons = %v, the frame draws %q", id, labels(got.Message.Actions), want.button)
		}
	}
}

// accountHarness makes «mix» on the day account-1 says it was created, with
// the relay on and the screens in UTC.
func accountHarness(t *testing.T) (*harness, *session.Session) {
	t.Helper()
	h := newHarness(t)
	h.cfg.Timezone = "UTC"
	h.rebuild()
	at := func(day string) time.Time {
		v, err := time.Parse("2006-01-02 15:04", day)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	h.users.SetClock(at("2026-02-11 09:00"))
	h.srv.SetClock(at("2026-02-11 09:00"))
	h.admin("alice", "password123")
	snap := h.st.Snapshot()
	secret, err := h.users.CreateInvite(snap.UserByLogin("alice").ID, "mix", "mix@example.org")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.users.CompleteInvite(t.Context(), secret, "mixpass123"); err != nil {
		t.Fatal(err)
	}
	return h, h.login("mix", "mixpass123")
}

func setDay(t *testing.T, h *harness, day string) {
	t.Helper()
	v, err := time.Parse("2006-01-02 15:04", day)
	if err != nil {
		t.Fatal(err)
	}
	h.users.SetClock(v)
	h.srv.SetClock(v)
}

// from-typecase note 10: the Email block in its four states, each the
// frame's, and a letter that did go is what makes «sent» — for as long as
// its link lives.
func TestAccountEmailBlockIsTheFrames(t *testing.T) {
	h, mix := accountHarness(t)
	// The first relay refuses the address: a letter that did not go.
	refusing := h.relayed()
	refusing.Refuse = "mix@example.org"

	// account-1: the head, the rubrics of the form, no Login row, no
	// «Letters you send».
	frame := mockupFrame(t, "account-1")
	raw := h.do(get("/app/settings/account", mix))
	page := decode[view.AccountPage](t, raw)
	if m := frameHeadSub.FindStringSubmatch(frame); m == nil || string(page.Subtitle) != frameText(m[1]) {
		t.Errorf("subtitle = %q, the frame draws %v", page.Subtitle, m)
	}
	rubrics := frameAll(frameRubric, frame[strings.Index(frame, `class="tc-main"`):], 1)
	var legends []string
	for _, set := range page.Profile.Sets {
		legends = append(legends, string(set.Legend))
	}
	if strings.Join(legends, "|") != strings.Join(rubrics[:2], "|") {
		t.Errorf("form rubrics = %v, the frame draws %v first", legends, rubrics)
	}
	if row := profileRow(page.Profile, "login"); row.Name != "" {
		t.Errorf("a Login row the frame does not draw: %+v", row)
	}
	if name := profileRow(page.Profile, "name"); string(name.Label) != frameText(frameLabel.FindStringSubmatch(frameBlock(t, frame, "Name"))[1]) {
		t.Errorf("name label = %q", name.Label)
	}
	// Save stands beside Display name, as Change beside the address; the
	// form has no button of its own.
	nameSave := frameNameSave.FindStringSubmatch(frameBlock(t, frame, "Name"))
	if nameSave == nil {
		t.Fatal("account-1 draws no button beside Display name: read the frame again")
	}
	if name := profileRow(page.Profile, "name"); name.Extra == nil || string(name.Extra.Label) != frameText(nameSave[1]) || name.Extra.Method != "POST" {
		t.Errorf("the name's button = %+v, the frame draws %q", name.Extra, frameText(nameSave[1]))
	}
	if len(page.Profile.Actions) != 0 {
		t.Errorf("form buttons %v the frame does not draw", labels(page.Profile.Actions))
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw.Body.Bytes(), &keys); err != nil {
		t.Fatal(err)
	}
	if _, ok := keys["mail"]; ok {
		t.Error("the page still carries «Letters you send»: the frame draws it on the letter's sheet only")
	}
	if hint := frameAll(frameHint, frameBlock(t, frame, "Password"), 1); len(hint) != 1 || string(page.PasswordNote) != hint[0] {
		t.Errorf("password note = %q, the frame draws %v", page.PasswordNote, hint)
	}

	// s-mail-9: invited by a link, the address is unproven and nothing went.
	checkEmailBlock(t, "s-mail-9", page, view.EmailUnconfirmed)
	if page.Email.Message == nil {
		t.FailNow()
	}

	// A letter that the relay refused is not a letter that went.
	setDay(t, h, "2026-02-12 10:00")
	h.do(postForm(actionHref(t, page.Email.Message.Actions, "account.confirm"), mix, url.Values{}))
	checkEmailBlock(t, "s-mail-9", decode[view.AccountPage](t, h.do(get("/app/settings/account", mix))), view.EmailUnconfirmed)
	h.relayed()

	// account-9: the letter went on 12 February at 11:42 and the page is
	// read the day after, while the link lives — to where, when, Send
	// again. Sent the same day, the block says the clock alone.
	setDay(t, h, "2026-02-12 11:42")
	h.do(postForm(actionHref(t, page.Email.Message.Actions, "account.confirm"), mix, url.Values{}))
	if today := decode[view.AccountPage](t, h.do(get("/app/settings/account", mix))); today.Email.SentAt != "11:42" ||
		today.Email.Message == nil || !strings.HasSuffix(string(today.Email.Message.Body), " at 11:42.") {
		t.Errorf("sent today = %q, %+v", today.Email.SentAt, today.Email.Message)
	}
	setDay(t, h, "2026-02-13 09:00")
	page = decode[view.AccountPage](t, h.do(get("/app/settings/account", mix)))
	checkEmailBlock(t, "account-9", page, view.EmailSent)
	if page.Email.SentTo != "mix@example.org" || page.Email.SentAt != "12 February, 11:42" {
		t.Errorf("sent = %q at %q", page.Email.SentTo, page.Email.SentAt)
	}

	// The link burns after its time: nothing waits to be opened, and the
	// block is s-mail-9 again.
	setDay(t, h, "2026-02-16 12:00")
	checkEmailBlock(t, "s-mail-9", decode[view.AccountPage](t, h.do(get("/app/settings/account", mix))), view.EmailUnconfirmed)

	// account-8: sent once more on the 14th, opened, confirmed that day.
	setDay(t, h, "2026-02-14 11:42")
	relay := h.relayed()
	h.do(postForm(actionHref(t, page.Email.Message.Actions, "account.confirm"), mix, url.Values{}))
	letters := relay.Letters()
	if len(letters) != 1 {
		t.Fatalf("letters = %d", len(letters))
	}
	_, body := letterBody(t, letters[0])
	link := regexp.MustCompile(`http://example\.com(/confirm-email/[A-Za-z0-9_-]+)`).FindStringSubmatch(body)
	if link == nil {
		t.Fatalf("no link in:\n%s", body)
	}
	setDay(t, h, "2026-02-14 12:10")
	if rec := h.do(get(link[1], nil)); rec.Code != 200 {
		t.Fatalf("confirm: %d", rec.Code)
	}
	page = decode[view.AccountPage](t, h.do(get("/app/settings/account", mix)))
	checkEmailBlock(t, "account-8", page, view.EmailConfirmed)
	if page.Email.ConfirmedAt != "14 February" {
		t.Errorf("confirmed at = %q", page.Email.ConfirmedAt)
	}

	// account-10: a change whose letter the relay refused — nothing to
	// open, the old address stays in use, and Send again.
	refusing = h.relayed()
	refusing.Refuse = "new@example.org"
	h.do(postForm(actionHref(t, []view.Action{*profileRow(page.Profile, "email").Extra}, "account.email.change"), mix,
		url.Values{"name": {""}, "email": {"new@example.org"}}))
	page = decode[view.AccountPage](t, h.do(get("/app/settings/account", mix)))
	checkEmailBlock(t, "account-10", page, view.EmailPending)
	if page.Email.Message == nil {
		t.FailNow()
	}

	// account-1: a new address waits, its letter gone; the confirmed one
	// stays in use.
	h.relayed()
	h.do(postForm(actionHref(t, page.Email.Message.Actions, "account.confirm"), mix, url.Values{}))
	page = decode[view.AccountPage](t, h.do(get("/app/settings/account", mix)))
	checkEmailBlock(t, "account-1", page, view.EmailPending)
	if page.Email.Pending != "new@example.org" {
		t.Errorf("pending = %q", page.Email.Pending)
	}

	// An address confirmed before the day was recorded says «confirmed».
	if err := h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByLogin("mix")
		u.PendingEmail, u.EmailConfirmedAt = "", nil
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	page = decode[view.AccountPage](t, h.do(get("/app/settings/account", mix)))
	if page.Email.State != view.EmailConfirmed || page.Email.Badge == nil || page.Email.Badge.Label != "confirmed" || page.Email.ConfirmedAt != "" {
		t.Errorf("confirmed without a day = %+v", page.Email)
	}
}

type frameEscrow struct {
	badge, badgeRole, hint, button string
}

func readFrameEscrow(t *testing.T, id string) frameEscrow {
	t.Helper()
	frame := mockupFrame(t, id)
	at := strings.Index(frame, `<div class="tc-rubric">Key escrow`)
	if at < 0 {
		t.Fatalf("%s: no Key escrow rubric", id)
	}
	block := frame[at:]
	var f frameEscrow
	if m := frameBadge.FindStringSubmatch(block); m != nil && strings.Index(block, m[0]) < strings.Index(block, "</div>") {
		f.badgeRole, f.badge = m[1], frameText(m[2])
	}
	if m := frameAll(frameHint, block, 1); len(m) > 0 {
		f.hint = m[0]
	}
	if m := frameFieldBt.FindStringSubmatch(block); m != nil {
		f.button = frameText(m[1])
	}
	return f
}

// from-typecase note 10: Key escrow on the account page (account-1,
// account-7), the deposit sheet over it (account-2), the escrow page
// (account-4); withdrawal goes through a sheet of its own.
func TestAccountEscrowIsTheFrames(t *testing.T) {
	h := newHarness(t)
	h.cfg.Timezone = "UTC"
	h.rebuild()
	h.enableEscrow("alice", "password123", "sixteen-char-pass!")
	snap := h.st.Snapshot()
	h.invite(snap.UserByLogin("alice").ID, "mix", "mixpass123")
	mix := h.login("mix", "mixpass123")
	if err := h.users.WithdrawEscrow(mix.AccountID()); err != nil {
		t.Fatal(err)
	}

	// account-1: not deposited — the frame's words and Deposit my key.
	want := readFrameEscrow(t, "account-1")
	page := decode[view.AccountPage](t, h.do(get("/app/settings/account", mix)))
	if string(page.Escrow.Note) != want.hint || page.Escrow.Badge != nil {
		t.Errorf("escrow note = %q badge %+v, the frame draws %q", page.Escrow.Note, page.Escrow.Badge, want.hint)
	}
	if len(page.Escrow.Actions) != 1 || string(page.Escrow.Actions[0].Label) != want.button || page.Escrow.Actions[0].Method != "GET" {
		t.Fatalf("escrow actions = %+v, the frame draws %q", page.Escrow.Actions, want.button)
	}
	for _, a := range page.Actions {
		if a.ID == "account.escrow" {
			t.Error("a Key escrow button the frame does not draw")
		}
	}

	// account-12: the escrow page without a copy — what a deposit does,
	// before the button.
	none := mockupFrame(t, "account-12")
	np := decode[view.EscrowPage](t, h.do(get("/app/settings/escrow", mix)))
	if m := frameHeadSub.FindStringSubmatch(none); m == nil || string(np.Subtitle) != frameText(m[1]) {
		t.Errorf("escrow subtitle without a copy = %q, the frame draws %v", np.Subtitle, m)
	}
	if m := frameRubric.FindStringSubmatch(none); m == nil || string(np.Rubric) != frameText(m[1]) {
		t.Errorf("escrow rubric without a copy = %q, the frame draws %v", np.Rubric, m)
	}
	if m := frameAll(frameHint, none, 1); len(m) != 1 || string(np.Status.Note) != m[0] {
		t.Errorf("escrow note without a copy = %q, the frame draws %v", np.Status.Note, m)
	}
	if m := frameFieldBt.FindStringSubmatch(none); m == nil || len(np.Status.Actions) != 1 ||
		string(np.Status.Actions[0].Label) != frameText(m[1]) || np.Status.Actions[0].Href != page.Escrow.Actions[0].Href {
		t.Errorf("escrow page actions without a copy = %+v, the frame draws %v", np.Status.Actions, m)
	}

	// account-2: the sheet over Account.
	sheetFrame := mockupFrame(t, "account-2")
	sheetAt := strings.Index(sheetFrame, `<div class="tc-sheet `)
	sheetPart := sheetFrame[sheetAt:]
	sheet := decode[view.UnlockSheet](t, h.do(get(page.Escrow.Actions[0].Href, mix)))
	if m := frameSheetT.FindStringSubmatch(sheetPart); m == nil || string(sheet.Purpose) != frameText(m[1]) {
		t.Errorf("sheet purpose = %q, the frame's title %v", sheet.Purpose, m)
	}
	if m := frameMsg.FindStringSubmatch(sheetPart); m == nil || string(sheet.Warning) != frameText(m[2]) {
		t.Errorf("sheet warning = %q", sheet.Warning)
	}
	pw := profileRow(sheet.Form, "password")
	if m := frameLabel.FindStringSubmatch(sheetPart); m == nil || string(pw.Label) != frameText(m[1]) || pw.Kind != view.InputPassword {
		t.Errorf("password row = %+v", pw)
	}
	if hints := frameAll(regexp.MustCompile(`(?s)<span class="tc-field__hint">(.*?)</span>`), sheetPart, 1); len(hints) != 1 || string(pw.Hint) != hints[0] {
		t.Errorf("password hint = %q, the frame draws %v", pw.Hint, hints)
	}
	var foot []string
	for _, b := range frameButton.FindAllStringSubmatch(frameSheetF.FindStringSubmatch(sheetPart)[1], -1) {
		foot = append(foot, frameText(b[1]))
	}
	if strings.Join(labels(sheet.Form.Actions), "|") != strings.Join(foot, "|") {
		t.Errorf("sheet buttons = %v, the frame draws %v", labels(sheet.Form.Actions), foot)
	}

	// A wrong password deposits nothing.
	fields := formFields(sheet.Form)
	fields.Set("password", "wrong-password")
	h.do(fragment(postForm(sheet.Form.Action, mix, fields), "sheet"))
	if snap := h.st.Snapshot(); store.UserDeposited(snap.UserByLogin("mix")) {
		t.Fatal("deposited on a wrong password")
	}
	setDay(t, h, "2026-03-02 10:00")
	fields.Set("password", "mixpass123")
	rec := h.do(postForm(sheet.Form.Action, mix, fields))
	if rec.Code != 303 || !strings.HasSuffix(rec.Header().Get("Location"), "/app/settings/escrow") {
		t.Fatalf("deposit: %d %s", rec.Code, rec.Header().Get("Location"))
	}

	// account-7: deposited — the badge, the words, Withdraw.
	want = readFrameEscrow(t, "account-7")
	page = decode[view.AccountPage](t, h.do(get("/app/settings/account", mix)))
	if b := page.Escrow.Badge; b == nil || string(b.Label) != want.badge || b.Role != badgeRole(want.badgeRole) {
		t.Errorf("escrow badge = %+v, the frame draws %q (%s)", b, want.badge, want.badgeRole)
	}
	if string(page.Escrow.Note) != want.hint {
		t.Errorf("escrow note = %q, the frame draws %q", page.Escrow.Note, want.hint)
	}
	if len(page.Escrow.Actions) != 1 || string(page.Escrow.Actions[0].Label) != want.button || page.Escrow.Actions[0].Method != "GET" || !page.Escrow.Actions[0].Danger {
		t.Fatalf("escrow actions = %+v, the frame draws %q", page.Escrow.Actions, want.button)
	}

	// account-4: the escrow page — the words, the pairs, the line by the
	// button.
	esc := mockupFrame(t, "account-4")
	ep := decode[view.EscrowPage](t, h.do(get("/app/settings/escrow", mix)))
	if m := frameHeadSub.FindStringSubmatch(esc); m == nil || string(ep.Subtitle) != frameText(m[1]) {
		t.Errorf("escrow subtitle = %q", ep.Subtitle)
	}
	if m := frameMsg.FindStringSubmatch(esc); m == nil || string(ep.Status.Note) != frameText(m[2]) {
		t.Errorf("escrow page note = %q", ep.Status.Note)
	}
	pairs := framePair.FindAllStringSubmatch(esc, -1)
	if len(pairs) != 2 || frameText(pairs[0][1]) != "Deposited" || frameText(pairs[1][1]) != "Last recovery" {
		t.Fatalf("the frame's pairs changed: %v", pairs)
	}
	if string(ep.Status.Deposited) != frameText(pairs[0][2]) || string(ep.Status.LastRecovery) != frameText(pairs[1][2]) {
		t.Errorf("pairs = %q / %q, the frame draws %q / %q", ep.Status.Deposited, ep.Status.LastRecovery, frameText(pairs[0][2]), frameText(pairs[1][2]))
	}
	if m := frameFootDim.FindStringSubmatch(esc); m == nil || string(ep.Status.ActionNote) != frameText(m[1]) {
		t.Errorf("action note = %q", ep.Status.ActionNote)
	}
	if len(ep.Status.Actions) != 1 || ep.Status.Actions[0].Method != "GET" {
		t.Fatalf("escrow page actions = %+v", ep.Status.Actions)
	}
	if ep.Rubric != "" {
		t.Errorf("a rubric %q the frame does not draw over the deposited copy", ep.Rubric)
	}

	// account-11: Withdraw opens a sheet; only its button takes the copy
	// away.
	wFrame := mockupFrame(t, "account-11")
	wPart := wFrame[strings.Index(wFrame, `<div class="tc-sheet `):]
	ws := decode[view.EscrowWithdrawSheet](t, h.do(get(ep.Status.Actions[0].Href, mix)))
	if snap := h.st.Snapshot(); !store.UserDeposited(snap.UserByLogin("mix")) {
		t.Fatal("opening the sheet withdrew the key")
	}
	if m := frameSheetT.FindStringSubmatch(wPart); m == nil || string(ws.Title) != frameText(m[1]) {
		t.Errorf("withdraw sheet title = %q, the frame draws %v", ws.Title, m)
	}
	if m := frameMsg.FindStringSubmatch(wPart); m == nil || string(ws.Warning) != frameText(m[2]) || m[1] != "alert" {
		t.Errorf("withdraw sheet warning = %q, the frame draws %v", ws.Warning, m)
	}
	var wFoot []string
	for _, b := range frameButton.FindAllStringSubmatch(frameSheetF.FindStringSubmatch(wPart)[1], -1) {
		wFoot = append(wFoot, frameText(b[1]))
	}
	if strings.Join(labels(ws.Actions), "|") != strings.Join(wFoot, "|") || !ws.Actions[0].Danger {
		t.Errorf("withdraw sheet buttons = %v, the frame draws %v", labels(ws.Actions), wFoot)
	}
	rec = h.do(postForm(actionHref(t, ws.Actions, "escrow.withdraw"), mix, formFields(ws.Form)))
	if rec.Code != 303 {
		t.Fatalf("withdraw: %d", rec.Code)
	}
	if snap := h.st.Snapshot(); store.UserDeposited(snap.UserByLogin("mix")) {
		t.Fatal("still deposited after the sheet's button")
	}
}

// account-13: the scheme is off — the block stays and says so, without a
// badge and without a button.
func TestAccountEscrowOffIsTheFrame(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	h.invite(adminID, "bob", "bobpass123")
	bob := h.login("bob", "bobpass123")
	want := readFrameEscrow(t, "account-13")
	if want.hint == "" || want.badge != "" || want.button != "" {
		t.Fatalf("account-13 reads otherwise: %+v", want)
	}
	page := decode[view.AccountPage](t, h.do(get("/app/settings/account", bob)))
	if e := page.Escrow; e.Active || string(e.Note) != want.hint || e.Badge != nil || len(e.Actions) != 0 {
		t.Errorf("escrow off = %+v, the frame draws %q alone", e, want.hint)
	}
}

func fixtureAs[T any](t *testing.T, name string) T {
	t.Helper()
	raw, err := json.Marshal(loadFixture(t, name))
	if err != nil {
		t.Fatal(err)
	}
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// The fixture set is the frames' instance: escrow on (account-1 offers
// Deposit my key, s-admin-6 is the scheme switched on), and people's letters
// going from their login on the instance's domain (s-mail-1).
func TestFixturesAreTheAccountFrames(t *testing.T) {
	account := fixtureAs[view.AccountPage](t, "app-settings-account/normal.json")
	want := readFrameEscrow(t, "account-1")
	if string(account.Escrow.Note) != want.hint || len(account.Escrow.Actions) != 1 || string(account.Escrow.Actions[0].Label) != want.button {
		t.Errorf("account fixture escrow = %q %v, the frame draws %q and %q", account.Escrow.Note, labels(account.Escrow.Actions), want.hint, want.button)
	}
	escrow := fixtureAs[view.EscrowPage](t, "app-settings-escrow/normal.json")
	if !escrow.Status.Active || len(escrow.Status.Actions) != 1 || string(escrow.Status.Actions[0].Label) != want.button {
		t.Errorf("escrow fixture = active %v, actions %v", escrow.Status.Active, labels(escrow.Status.Actions))
	}
	admin := fixtureAs[view.AdminEscrowPage](t, "admin-escrow/normal.json")
	policy := regexp.MustCompile(`<span class="tc-tickrow__tick"></span>(.*?)</button>`).FindStringSubmatch(mockupFrame(t, "s-admin-6"))
	if !admin.Enabled || policy == nil || string(profileRow(admin.Form, "coverNew").Label) != frameText(policy[1]) {
		t.Errorf("admin escrow fixture: enabled %v, policy row %+v, the frame draws %v", admin.Enabled, profileRow(admin.Form, "coverNew"), policy)
	}

	mail := mockupFrame(t, "s-mail-1")
	from := regexp.MustCompile(`>from (\S+)</span>`).FindStringSubmatch(mail)
	signed := regexp.MustCompile(`Signed <b>(.*?)</b> · replies go to (\S+)</span>`).FindStringSubmatch(mail)
	if from == nil || signed == nil {
		t.Fatal("s-mail-1 changed: no from tag or Signed line")
	}
	sheet := fixtureAs[view.MailSheet](t, "app-mail-compose/normal.json")
	if string(sheet.From) != signed[1]+" <"+from[1]+">" || string(sheet.ReplyTo) != signed[2] {
		t.Errorf("compose fixture from %q reply %q, the frame draws %q as %q, replies to %q", sheet.From, sheet.ReplyTo, from[1], signed[1], signed[2])
	}
}

// A change of address whose letter did not go can be sent again: without
// it the person is stuck with a change they cannot finish. The frame draws
// the pending badge alone (account-1) — to-typecase task 21.
func TestPendingChangeCanBeSentAgain(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	h.invite(adminID, "bob", "bobpass123")
	h.confirmAddress("bob", "bob@example.org")
	refusing := h.relayed()
	refusing.Refuse = "new@example.org"
	bob := h.login("bob", "bobpass123")
	h.do(postForm("/app/settings/account", bob, url.Values{"name": {""}, "email": {"new@example.org"}}))

	page := decode[view.AccountPage](t, h.do(get("/app/settings/account", bob)))
	if page.Email.State != view.EmailPending || page.Email.Message == nil || len(page.Email.Message.Actions) != 1 ||
		page.Email.Message.Actions[0].Label != "Send again" {
		t.Fatalf("pending without a letter = %+v", page.Email)
	}
	relay := h.relayed()
	h.do(postForm(actionHref(t, page.Email.Message.Actions, "account.confirm"), bob, url.Values{}))
	if letters := relay.Letters(); len(letters) != 1 || letters[0].To[0] != "new@example.org" {
		t.Fatalf("letters = %+v", letters)
	}
	page = decode[view.AccountPage](t, h.do(get("/app/settings/account", bob)))
	if page.Email.State != view.EmailPending || page.Email.Message != nil || page.Email.SentTo != "new@example.org" {
		t.Errorf("pending with a letter gone = %+v", page.Email)
	}
}
