package web_test

import (
	"context"
	"encoding/json"
	"html"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

// The pages outside the shell (U-6.14) against their frames: every text,
// field, button and link a frame of 7.20 and 7.21 draws is in the view
// model, in the frame's words (C-6 «кадр — правда», C-10). The frames are
// read here, not copied into the tests.

var (
	authTitle   = regexp.MustCompile(`(?s)<div class="tc-head__title[^"]*">(.*?)</div>`)
	authSub     = regexp.MustCompile(`(?s)<div class="tc-head__sub[^"]*">(.*?)</div>`)
	authProse   = regexp.MustCompile(`(?s)<div class="tc-prose[^"]*">(.*?)</div>`)
	authAlert   = regexp.MustCompile(`(?s)<div class="tc-msg is-alert[^"]*">(.*?)</div>`)
	authHint    = regexp.MustCompile(`(?s)<span class="tc-field__hint">(.*?)</span>`)
	authTagline = regexp.MustCompile(`(?s)<strong class="tc-auth__tagline-lead">(.*?)</strong>(.*?)</div>`)
	authField   = regexp.MustCompile(`(?s)<div class="tc-field[^"]*"><label>(.*?)</label><input class="tc-in[^"]*" type="text"(?: value="[^"]*")?(?: placeholder="([^"]*)")? aria-label="[^"]*"( disabled)?>`)
	authButton  = regexp.MustCompile(`(?s)<button class="tc-btn[^"]*" type="button">(.*?)</button>`)
	authLink    = regexp.MustCompile(`(?s)<span class="(?:tc-auth__link|tc-dim--accent[^"]*)">(.*?)</span>`)
	authCard    = regexp.MustCompile(`(?s)<div class="tc-card__term">(.*?)</div>\s*<div class="tc-prose[^"]*">(.*?)</div>\s*<button class="tc-btn[^"]*" type="button">(.*?)</button>`)
)

// authPage is what a page outside the shell draws: its texts, its fields —
// the label, its muted part, «[placeholder]», «off» for one that does not
// change —, its buttons and links in their order, and the cards of
// s-auth-6 as «term | text | button».
type authPage struct {
	Title, Sub, Prose, Alert, Hint, Lead, Tagline string
	Fields, Buttons, Links, Cards                 []string
}

// authFrame reads a frame of 7.20, 7.21 or of the password page, the
// caption left out; inside the shell only its main part.
func authFrame(t *testing.T, id string) authPage {
	t.Helper()
	frame, _, _ := strings.Cut(mockupFrame(t, id), "<figcaption")
	if _, main, ok := strings.Cut(frame, `<div class="tc-main">`); ok {
		frame = main
	}
	var p authPage
	for _, m := range authCard.FindAllStringSubmatch(frame, -1) {
		p.Cards = append(p.Cards, authText(m[1])+" | "+authText(m[2])+" | "+authText(m[3]))
	}
	frame = authCard.ReplaceAllString(frame, "")
	one := func(re *regexp.Regexp) string {
		if m := re.FindStringSubmatch(frame); m != nil {
			return authText(m[1])
		}
		return ""
	}
	p.Title, p.Sub, p.Prose, p.Alert, p.Hint = one(authTitle), one(authSub), one(authProse), one(authAlert), one(authHint)
	if m := authTagline.FindStringSubmatch(frame); m != nil {
		p.Lead, p.Tagline = frameText(m[1]), frameText(m[2])
	}
	for _, m := range authField.FindAllStringSubmatch(frame, -1) {
		f := frameText(m[1])
		if m[2] != "" {
			f += " [" + html.UnescapeString(m[2]) + "]"
		}
		if m[3] != "" {
			f += " off"
		}
		p.Fields = append(p.Fields, f)
	}
	for _, m := range authButton.FindAllStringSubmatch(frame, -1) {
		p.Buttons = append(p.Buttons, frameText(m[1]))
	}
	for _, m := range authLink.FindAllStringSubmatch(frame, -1) {
		p.Links = append(p.Links, frameText(m[1]))
	}
	return p
}

// authText is frameText without the space a dropped tag leaves before a
// stop: «to <span>v@example.org</span>.» is «to v@example.org.».
func authText(s string) string {
	return authStop.ReplaceAllString(frameText(s), "$1")
}

var authStop = regexp.MustCompile(` ([.,:;])`)

// said is a text of the server written the way frameText writes one:
// paragraphs and spaces as single spaces.
func said(t view.Text) string { return strings.Join(strings.Fields(string(t)), " ") }

// shownFields are the fields of a form written the way authFrame writes
// them.
func shownFields(f view.Form) []string {
	var out []string
	for _, set := range f.Sets {
		for _, r := range set.Rows {
			if r.Kind == view.InputHidden {
				continue
			}
			s := string(r.Label)
			if r.LabelAside != "" {
				s += " " + string(r.LabelAside)
			}
			if r.Placeholder != "" {
				s += " [" + string(r.Placeholder) + "]"
			}
			if r.ReadOnly {
				s += " off"
			}
			out = append(out, s)
		}
	}
	return out
}

func sameAuth(t *testing.T, what, id string, got authPage) {
	t.Helper()
	sameAuthAs(t, what, id, got, authFrame(t, id))
}

func sameAuthAs(t *testing.T, what, id string, got, want authPage) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s =\n%+v,\nwant (%s)\n%+v", what, got, id, want)
	}
}

func alertOf(p view.Plain) string {
	if len(p.Notices) != 1 || p.Notices[0].Role != view.NoticeAlert || p.Notices[0].Title != "" {
		return ""
	}
	return said(p.Notices[0].Body)
}

func openRegistration(t *testing.T, h *harness) {
	t.Helper()
	h.relayed()
	if err := h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		st.Settings.SelfRegEnabled = true
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
}

// s-auth-1: the sign, the tagline in two lines, Login and Password, Sign
// in, and under it «Create an account» — only while registration is open —
// and «Forgotten your password?». The title is not drawn — the sign stands
// in its place — and stays the document's title. Nothing the frame does not
// draw: no «About» (rest-3 is reached from the rail's foot).
func TestLoginPageIsTheFrame(t *testing.T) {
	h := newHarness(t)
	h.admin("mix", "password123")
	page := func(p view.LoginPage) authPage {
		links := []string{}
		if p.Register != nil {
			links = append(links, string(p.Register.Label))
		}
		return authPage{Lead: said(p.Lede), Tagline: said(p.Tagline), Fields: shownFields(p.Form),
			Buttons: labels(p.Form.Actions), Links: append(links, string(p.Forgot.Label))}
	}
	want := authFrame(t, "s-auth-1")
	closed := want
	closed.Links = want.Links[1:]

	shut := decode[view.LoginPage](t, h.do(get("/login", nil)))
	sameAuthAs(t, "sign-in, registration closed", "s-auth-1", page(shut), closed)
	if shut.Register != nil || shut.Title == "" || shut.Forgot.Href != "/forgot" || shut.Forgot.Method != http.MethodGet {
		t.Errorf("register %+v, title %q, forgot %+v", shut.Register, shut.Title, shut.Forgot)
	}
	raw := decode[map[string]any](t, h.do(get("/login", nil)))
	if _, ok := raw["about"]; ok {
		t.Error("the sign-in page carries «About», which s-auth-1 does not draw")
	}

	openRegistration(t, h)
	open := decode[view.LoginPage](t, h.do(get("/login", nil)))
	sameAuth(t, "sign-in, registration open", "s-auth-1", page(open))
	if open.Register == nil || open.Register.Href != "/register" || open.Register.Method != http.MethodGet || !open.Register.Enabled {
		t.Errorf("register = %+v", open.Register)
	}
	// The caret: in Login on the empty page. What stands in the fields of
	// s-auth-1 is typed, not sent, so only the focus is read from it.
	caret := func(in []authInput) []authInput {
		out := make([]authInput, len(in))
		for i, f := range in {
			out[i] = authInput{Label: f.Label, Focus: f.Focus}
		}
		return out
	}
	if got, want := caret(shownInputs(open.Form)), caret(authInputs(t, "s-auth-1")); !reflect.DeepEqual(got, want) {
		t.Errorf("sign-in fields = %+v, want (s-auth-1) %+v", got, want)
	}
}

// s-auth-13: a wrong login or a wrong password is the same page — the login
// stays typed, the password field is empty and in focus, and the reason is
// at it, one for both: which of the two was wrong the page does not tell.
// No notice over the fields.
func TestSignInRefusedIsTheFrame(t *testing.T) {
	h := newHarness(t)
	h.admin("mix", "password123")
	openRegistration(t, h)
	page := func(p view.LoginPage) authPage {
		links := []string{}
		if p.Register != nil {
			links = append(links, string(p.Register.Label))
		}
		return authPage{Lead: said(p.Lede), Tagline: said(p.Tagline), Alert: alertOf(p.Plain), Fields: shownFields(p.Form),
			Buttons: labels(p.Form.Actions), Links: append(links, string(p.Forgot.Label))}
	}
	wrongPassword := decode[view.LoginPage](t, h.do(h.anonPostForm("/login", "/login", url.Values{"login": {"mix"}, "password": {"wrong-one"}})))
	sameAuth(t, "a wrong password", "s-auth-13", page(wrongPassword))
	if len(wrongPassword.Notices) != 0 {
		t.Errorf("notices %+v: s-auth-13 draws none, the reason is at the field", wrongPassword.Notices)
	}
	want := authInputs(t, "s-auth-13")
	if got := shownInputs(wrongPassword.Form); !reflect.DeepEqual(got, want) {
		t.Errorf("fields of a wrong password = %+v,\nwant (s-auth-13) %+v", got, want)
	}

	// A login nobody has: the same page, the same reason, with what was
	// typed back in Login.
	nobody := decode[view.LoginPage](t, h.do(h.anonPostForm("/login", "/login", url.Values{"login": {"nobody"}, "password": {"wrong-one"}})))
	sameAuth(t, "an unknown login", "s-auth-13", page(nobody))
	want[0].Value = "nobody"
	if got := shownInputs(nobody.Form); !reflect.DeepEqual(got, want) {
		t.Errorf("fields of an unknown login = %+v,\nwant %+v", got, want)
	}
}

// authInput is a field as it arrives: its label, what stands in it, whether
// the caret is in it, and the reason at it.
type authInput struct {
	Label, Value, Error string
	Focus               bool
}

var authInputRe = regexp.MustCompile(`(?s)<div class="tc-field[^"]*"><label>(.*?)</label><input class="tc-in([^"]*)" type="text"(?: value="([^"]*)")?[^>]*>(?:<span class="tc-field__hint tc-dim--danger">(.*?)</span>)?`)

// authInputs reads the fields of a frame outside the shell: «is-focus» is
// the caret, the danger hint under a field is its reason.
func authInputs(t *testing.T, id string) []authInput {
	t.Helper()
	frame, _, _ := strings.Cut(mockupFrame(t, id), "<figcaption")
	var out []authInput
	for _, m := range authInputRe.FindAllStringSubmatch(frame, -1) {
		out = append(out, authInput{Label: frameText(m[1]), Value: html.UnescapeString(m[3]),
			Error: frameText(m[4]), Focus: strings.Contains(" "+m[2]+" ", " is-focus ")})
	}
	if len(out) == 0 {
		t.Fatalf("%s: no fields read", id)
	}
	return out
}

// shownInputs are the fields of a form written the way authInputs reads
// them.
func shownInputs(f view.Form) []authInput {
	var out []authInput
	for _, set := range f.Sets {
		for _, r := range set.Rows {
			if r.Kind == view.InputHidden {
				continue
			}
			out = append(out, authInput{Label: string(r.Label), Value: string(r.Value), Error: said(r.Error), Focus: r.Focus})
		}
	}
	return out
}

// s-auth-2 and s-auth-3: the form with the warning over the fields and
// Repeat password, and «Already have an account? Sign in»; after it went,
// «Check your email», the address the letter went to, and one door back.
func TestRegisterPagesAreTheFrames(t *testing.T) {
	h := newHarness(t)
	h.admin("mix", "password123")
	openRegistration(t, h)
	form := decode[view.RegisterPage](t, h.do(get("/register", nil)))
	sameAuth(t, "registration", "s-auth-2", authPage{Title: said(form.Title), Sub: said(form.Lede), Alert: alertOf(form.Plain),
		Fields: shownFields(form.Form), Buttons: labels(form.Form.Actions), Links: []string{string(form.SignIn.Label)}})
	if form.SignIn.Href != "/login" || form.SignIn.Method != http.MethodGet {
		t.Errorf("sign in = %+v", form.SignIn)
	}

	// The repeat is checked: a pair that differs registers nobody.
	bad := decode[view.RegisterPage](t, h.do(h.anonPostForm("/register", "/register",
		url.Values{"login": {"viktor"}, "email": {"v@example.org"}, "password": {"password123"}, "password2": {"password124"}})))
	if r := rowNamed(t, bad.Form, "password2"); r.Error == "" || bad.Sent || h.users.LoginExists("viktor") {
		t.Errorf("a pair that differs: %+v, sent %v", r, bad.Sent)
	}

	frame := authFrame(t, "s-auth-3")
	address := regexp.MustCompile(`[\w.]+@[\w.]+\w`).FindString(frame.Prose)
	if address == "" {
		t.Fatalf("s-auth-3 = %+v: read the frame again", frame)
	}
	sent := decode[view.RegisterPage](t, h.do(h.anonPostForm("/register", "/register",
		url.Values{"login": {"viktor"}, "email": {address}, "password": {"password123"}, "password2": {"password123"}})))
	sameAuth(t, "the letter went", "s-auth-3", authPage{Title: said(sent.Title), Prose: said(sent.Lede), Buttons: []string{string(sent.SignIn.Label)}})
	if !sent.Sent || sent.SignIn.Href != "/login" {
		t.Errorf("sent %v, back %+v", sent.Sent, sent.SignIn)
	}
}

// s-auth-4: «Set up Carrel», what the first account is, the warning, Login
// with its sample, Email (optional) with its dash, the two passwords.
func TestSetupPageIsTheFrame(t *testing.T) {
	h := newHarness(t)
	page := decode[view.SetupPage](t, h.do(get("/setup", nil)))
	got := authPage{Title: said(page.Title), Sub: said(page.Lede), Alert: alertOf(page.Plain), Fields: shownFields(page.Form), Buttons: labels(page.Form.Actions)}
	sameAuth(t, "setup", "s-auth-4", got)
	again := decode[view.SetupPage](t, h.do(h.anonPostForm("/setup", "/setup",
		url.Values{"login": {"mix"}, "password": {"password123"}, "password2": {"password124"}})))
	got = authPage{Title: said(again.Title), Sub: said(again.Lede), Alert: alertOf(again.Plain), Fields: shownFields(again.Form), Buttons: labels(again.Form.Actions)}
	sameAuth(t, "setup, the repeat differs", "s-auth-4", got)
	if r := rowNamed(t, again.Form, "password2"); r.Error == "" {
		t.Errorf("a pair that differs is not said: %+v", again.Form)
	}
}

// s-auth-5 to s-auth-9: the login or the address; then what can be done —
// the wipe by letter, and asking the administrator only for an account that
// deposited its key —, or that nothing can; the letter on its way; the
// administrator asked. Every state has one door back to sign-in.
func TestForgotPagesAreTheFrames(t *testing.T) {
	h := newHarness(t)
	const master = "sixteen-char-pass!"
	h.enableEscrow("alice", "password123", master)
	snap := h.st.Snapshot()
	adminID := snap.UserByLogin("alice").ID
	h.invite(adminID, "mix", "mixpass123")
	h.confirmAddress("mix", "mix@example.org")
	h.invite(adminID, "vera", "verapass123")
	h.confirmAddress("vera", "vera@example.org")
	if err := h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByLogin("vera")
		u.EscrowOptIn, u.EscrowDeposited = false, nil
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	relay := h.relayed()
	page := forgotShown

	start := decode[view.ForgotPage](t, h.do(get("/forgot", nil)))
	sameAuth(t, "forgot", "s-auth-5", page(start))
	if start.Back.Href != "/login" || start.Back.Method != http.MethodGet {
		t.Errorf("back = %+v", start.Back)
	}

	// Both cards in the frame's words: when the second is shown is the
	// caption's, not the card's (to-typecase task 40).
	options := authFrame(t, "s-auth-6")
	if len(options.Cards) != 2 {
		t.Fatalf("s-auth-6 cards = %q: read the frame again", options.Cards)
	}
	deposited := decode[view.ForgotPage](t, h.do(h.anonPostForm("/forgot", "/forgot", url.Values{"login": {"mix"}})))
	sameAuthAs(t, "what we can do", "s-auth-6", page(deposited), options)
	if !deposited.HasMail || deposited.Sent || deposited.Asked {
		t.Errorf("options: %+v", deposited)
	}
	notDeposited := decode[view.ForgotPage](t, h.do(h.anonPostForm("/forgot", "/forgot", url.Values{"login": {"vera"}})))
	onlyWipe := options
	onlyWipe.Cards = options.Cards[:1]
	onlyWipe.Prose = strings.Replace(options.Prose, "mix", "vera", 1)
	sameAuthAs(t, "what we can do, no key deposited", "s-auth-6", page(notDeposited), onlyWipe)
	if len(relay.Letters()) != 0 {
		t.Errorf("a letter went before it was asked for: %+v", relay.Letters())
	}

	for _, who := range []string{"nobody", "alice"} { // alice has no confirmed address
		none := decode[view.ForgotPage](t, h.do(h.anonPostForm("/forgot", "/forgot", url.Values{"login": {who}})))
		sameAuth(t, who+": nothing to do", "s-auth-7", page(none))
		if none.HasMail || len(none.Options) != 0 {
			t.Errorf("%s: %+v", who, none)
		}
	}

	post := func(o view.ForgotOption) view.ForgotPage {
		t.Helper()
		return decode[view.ForgotPage](t, h.do(h.anonPostForm("/forgot", o.Form.Action, formFields(o.Form))))
	}
	sent := post(deposited.Options[0])
	sameAuth(t, "the letter", "s-auth-8", page(sent))
	if letters := relay.Letters(); !sent.Sent || len(letters) != 1 || letters[0].To[0] != "mix@example.org" {
		t.Errorf("sent %v, letters %+v", sent.Sent, letters)
	}

	asked := post(deposited.Options[1])
	sameAuth(t, "the administrator asked", "s-auth-9", page(asked))
	recorded := false
	for _, e := range h.st.Snapshot().Audit {
		recorded = recorded || (e.What == "user.forgot.ask" && e.Whom == "mix")
	}
	if !asked.Asked || !recorded {
		t.Errorf("asked %v, recorded %v", asked.Asked, recorded)
	}
	// Asking for an account that deposited nothing records nothing.
	form := formFields(deposited.Options[1].Form)
	form.Set("login", "vera")
	if refused := decode[view.ForgotPage](t, h.do(h.anonPostForm("/forgot", "/forgot", form))); refused.Asked {
		t.Errorf("vera asked: %+v", refused)
	}
}

// forgotShown is a state of /forgot written the way authFrame reads one.
func forgotShown(p view.ForgotPage) authPage {
	var cards []string
	for _, o := range p.Options {
		button := ""
		if len(o.Form.Actions) == 1 {
			button = string(o.Form.Actions[0].Label)
		}
		cards = append(cards, said(o.Title)+" | "+said(o.Body)+" | "+button)
	}
	return authPage{Title: said(p.Title), Prose: said(p.Lede), Alert: alertOf(p.Plain), Fields: shownFields(p.Form),
		Buttons: append(labels(p.Form.Actions), string(p.Back.Label)), Cards: cards}
}

// from-typecase note 24: a letter that did not go is the frame the person
// was on, the server's line over it as an alert — the options of /forgot
// (s-auth-6), still there to try again or to ask; after /register the page
// of its letter (s-auth-3) with the line in place of the address, one door
// back. Nothing claims a letter went.
func TestLetterThatDidNotGoIsAnAlert(t *testing.T) {
	h := newHarness(t)
	const master = "sixteen-char-pass!"
	h.enableEscrow("alice", "password123", master)
	snap := h.st.Snapshot()
	h.invite(snap.UserByLogin("alice").ID, "mix", "mixpass123")
	h.confirmAddress("mix", "mix@example.org")
	relay := h.relayed()
	relay.Refuse = "mix@example.org"

	options := decode[view.ForgotPage](t, h.do(h.anonPostForm("/forgot", "/forgot", url.Values{"login": {"mix"}})))
	failed := decode[view.ForgotPage](t, h.do(h.anonPostForm("/forgot", options.Options[0].Form.Action, formFields(options.Options[0].Form))))
	want := authFrame(t, "s-auth-6")
	got := forgotShown(failed)
	if got.Alert == "" || failed.Sent || !failed.HasMail {
		t.Fatalf("the letter did not go: %+v", failed)
	}
	want.Alert = got.Alert
	sameAuthAs(t, "the letter did not go", "s-auth-6", got, want)

	openRegistration(t, h)
	h.relayed().Refuse = "v@example.org"
	reg := decode[view.RegisterPage](t, h.do(h.anonPostForm("/register", "/register",
		url.Values{"login": {"viktor"}, "email": {"v@example.org"}, "password": {"password123"}, "password2": {"password123"}})))
	sent := authFrame(t, "s-auth-3")
	if alertOf(reg.Plain) == "" || reg.Lede != "" || reg.Sent || len(reg.Form.Sets) != 0 ||
		reg.SignIn.Label != view.Text(sent.Buttons[0]) || reg.SignIn.Href != "/login" {
		t.Errorf("registration, the letter did not go: %+v", reg)
	}
}

// s-auth-10 and s-auth-11: one link, two pages. The erasure is named and
// its button says what it does — no word to type (the frame draws none) —
// and Cancel leads back; then the same form as an invitation's, the
// warning over it.
func TestForgotWipePagesAreTheFrames(t *testing.T) {
	h := newHarness(t)
	h.admin("mix", "password123")
	link, err := h.users.IssueConsoleWipeLink("mix")
	if err != nil {
		t.Fatal(err)
	}
	confirm := decode[view.ForgotWipePage](t, h.do(get(link, nil)))
	var cancel []string
	if confirm.Cancel != nil {
		cancel = []string{string(confirm.Cancel.Label)}
	}
	sameAuth(t, "erase", "s-auth-10", authPage{Title: said(confirm.Title), Prose: said(confirm.Lede),
		Fields: shownFields(confirm.Form), Buttons: append(labels(confirm.Form.Actions), cancel...)})
	if confirm.Cancel == nil || confirm.Cancel.Href != "/login" || len(confirm.Form.Actions) != 1 || !confirm.Form.Actions[0].Danger {
		t.Errorf("cancel %+v, actions %+v", confirm.Cancel, confirm.Form.Actions)
	}
	if _, _, _, ok := h.users.Authenticate(context.Background(), "mix", "password123"); !ok {
		t.Fatal("opening the link wiped")
	}

	next := decode[view.ForgotWipePage](t, h.do(h.anonPostForm(link, link, formFields(confirm.Form))))
	sameAuth(t, "a new password", "s-auth-11", authPage{Title: said(next.Title), Sub: said(next.Lede), Alert: alertOf(next.Plain),
		Fields: shownFields(next.Form), Buttons: labels(next.Form.Actions)})
	if next.Step != view.WipePassword || next.Cancel != nil {
		t.Errorf("step %q, cancel %+v", next.Step, next.Cancel)
	}
}

// rest-2: the address confirmed, one door to sign-in.
func TestConfirmEmailPageIsTheFrame(t *testing.T) {
	h := newHarness(t)
	h.admin("mix", "password123")
	openRegistration(t, h)
	token, err := h.users.Register(context.Background(), "viktor", "v@example.org", "password123")
	if err != nil {
		t.Fatal(err)
	}
	page := decode[view.ConfirmEmailPage](t, h.do(get("/confirm-email/"+token, nil)))
	sameAuth(t, "confirmed", "rest-2", authPage{Title: said(page.Title), Prose: said(page.Lede), Buttons: []string{string(page.Next.Label)}})
}

// rest-1 (from-typecase note 24): one page answers any dead secret link —
// an invitation, a wipe link, an address confirmation, a publication —
// «This link does not work», why it may be, one door to sign in. The
// answer is the same whatever the link was and whether it ever was: the
// status, the headers and the body (audit №3, C-11, S-9.5).
func TestDeadLinkPageIsTheFrame(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	used, err := h.users.CreateInvite(adminID, "bob", "")
	if err != nil {
		t.Fatal(err)
	}
	if rec := h.do(h.anonPostForm("/invite/"+used, "/invite/"+used, url.Values{"password": {"bobpass123"}, "password2": {"bobpass123"}})); rec.Code != http.StatusSeeOther {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body.String())
	}
	want := authFrame(t, "rest-1")
	never := strings.Repeat("A", 43)
	var first *http.Response
	var body string
	for _, req := range []*http.Request{
		get("/invite/"+never, nil), get("/invite/"+used, nil), get("/invite/not-a-token", nil),
		get("/forgot/"+never, nil), get("/confirm-email/"+never, nil), get("/p/"+never, nil),
		h.anonPostForm("/login", "/invite/"+never, url.Values{"password": {"x"}}),
		h.anonPostForm("/login", "/forgot/"+never, url.Values{}),
	} {
		rec := h.do(req)
		var page view.DeadLinkPage
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatalf("%s %s: %d %q", req.Method, req.URL.Path, rec.Code, rec.Body.String())
		}
		sameAuthAs(t, req.Method+" "+req.URL.Path, "rest-1", authPage{Title: said(page.Title), Prose: said(page.Lede),
			Buttons: []string{string(page.SignIn.Label)}}, want)
		if page.SignIn.Href != "/login" || page.SignIn.Method != http.MethodGet || !page.SignIn.Enabled {
			t.Errorf("%s: sign in = %+v", req.URL.Path, page.SignIn)
		}
		res := rec.Result()
		if first == nil {
			first, body = res, rec.Body.String()
			if res.StatusCode != http.StatusNotFound {
				t.Errorf("a dead link answers %d", res.StatusCode)
			}
			continue
		}
		if res.StatusCode != first.StatusCode || rec.Body.String() != body || !reflect.DeepEqual(res.Header, first.Header) {
			t.Errorf("%s %s differs: %d %v %q", req.Method, req.URL.Path, res.StatusCode, res.Header, rec.Body.String())
		}
	}
}

// rest-3: «Carrel», the version and the build, the tagline in two lines,
// the licence's line, Source code, Third-party licences and the way back —
// to sign-in without a session, from the shell back where one was.
func TestAboutPageIsTheFrame(t *testing.T) {
	h := newHarness(t)
	h.admin("mix", "password123")
	want := authFrame(t, "rest-3")
	if !regexp.MustCompile(`^\S+ · [0-9a-f]+$`).MatchString(want.Sub) {
		t.Fatalf("rest-3 build = %q: read the frame again", want.Sub)
	}
	page := func(p view.AboutPage) authPage {
		return authPage{Title: said(p.Title), Sub: string(p.Version) + " · " + string(p.Commit), Prose: said(p.Licence),
			Lead: said(p.Lede), Tagline: said(p.Tagline), Buttons: []string{string(p.Source.Label), string(p.Third.Label), string(p.Back.Label)}}
	}
	anon := decode[view.AboutPage](t, h.do(get("/about", nil)))
	want.Sub = string(anon.Version) + " · " + string(anon.Commit)
	sameAuthAs(t, "about", "rest-3", page(anon), want)
	if anon.Back.Href != "/login" {
		t.Errorf("back without a session = %+v", anon.Back)
	}

	mix := h.login("mix", "password123")
	from := func(referer string) view.Action {
		req := get("/about", mix)
		req.Header.Set("Referer", referer)
		return decode[view.AboutPage](t, h.do(req)).Back
	}
	for referer, href := range map[string]string{
		"http://example.com/app/settings/account": "/app/settings/account",
		"http://example.com/app/search?q=ann":     "/app/search?q=ann",
		"https://elsewhere.example/app/contacts":  "/app/contacts",
		"http://example.com/login":                "/app/contacts",
		"":                                        "/app/contacts",
	} {
		if back := from(referer); back.Href != href || back.Label == "Back to sign in" {
			t.Errorf("from %q: back = %+v, want %q", referer, back, href)
		}
	}
}

// account-3: one screen for two cases. From the account: the fields, the
// line under them and «Back to settings»; after a temporary password: the
// red line on top, and no way back — there is nowhere to go.
func TestPasswordPageIsTheFrame(t *testing.T) {
	h := newHarness(t)
	h.admin("mix", "password123")
	mix := h.login("mix", "password123")
	want := authFrame(t, "account-3")
	page := func(p view.PasswordPage) authPage {
		buttons := labels(p.Form.Actions)
		if p.Back != nil {
			buttons = append(buttons, string(p.Back.Label))
		}
		alert := ""
		if p.Warning.Role == view.NoticeAlert {
			alert = said(p.Warning.Body)
		}
		return authPage{Title: said(p.Title), Alert: alert, Hint: said(p.Note), Fields: shownFields(p.Form), Buttons: buttons}
	}
	usual := decode[view.PasswordPage](t, h.do(get("/app/password", mix)))
	plain := want
	plain.Alert = ""
	sameAuthAs(t, "change password", "account-3", page(usual), plain)
	if usual.Back == nil || usual.Back.Href != "/app/settings/account" {
		t.Errorf("back = %+v", usual.Back)
	}

	if err := h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		st.UserByLogin("mix").MustChangePassword = true
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	forced := decode[view.PasswordPage](t, h.do(get("/app/password", mix)))
	must := want
	must.Buttons = want.Buttons[:1]
	sameAuthAs(t, "a temporary password", "account-3", page(forced), must)
}
