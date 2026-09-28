package user

// Tests written for Fable audit №2 (accounts and secrets). Each one was red
// on the code as of 07.09.2026 before the matching fix landed (C-16).

import (
	"context"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
)

type fixture struct {
	svc *Service
	st  *store.Store
	mgr *session.Manager
	cfg *conf.Config
	now time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dir := t.TempDir()
	key, err := crypt.CreateServerKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dir, key)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DataDir = dir
	mgr := session.NewManager(cfg)
	f := &fixture{st: st, mgr: mgr, cfg: cfg, now: time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)}
	f.svc = New(st, mgr, cfg)
	f.svc.now = func() time.Time { return f.now }
	return f
}

func (f *fixture) advance(d time.Duration) { f.now = f.now.Add(d) }

func (f *fixture) setupAdmin(t *testing.T) string {
	t.Helper()
	if err := f.svc.SetupFirstAdmin(context.Background(), "admin", "password123"); err != nil {
		t.Fatal(err)
	}
	snap := f.st.Snapshot()
	return snap.UserByLogin("admin").ID
}

func (f *fixture) idOf(t *testing.T, login string) string {
	t.Helper()
	snap := f.st.Snapshot()
	u := snap.UserByLogin(login)
	if u == nil {
		t.Fatalf("no user %q", login)
	}
	return u.ID
}

func (f *fixture) openRegistration(t *testing.T) {
	t.Helper()
	err := f.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		st.Settings.SelfRegEnabled = true
		st.Settings.SMTP = store.SMTPSettings{Host: "127.0.0.1", Port: 25, From: "noreply@carrel.example"}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) confirmEmailOf(t *testing.T, login, email string) {
	t.Helper()
	err := f.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByLogin(login)
		if u == nil {
			t.Fatalf("no user %q", login)
		}
		u.Email = email
		u.EmailConfirmed = true
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) canLogin(login, password string) bool {
	_, _, _, ok := f.svc.Authenticate(context.Background(), login, password)
	return ok
}

// Ф2-1. Open registration must not reuse a record that already belongs to
// someone: the first administrator has no email, so before the fix
// Register("admin", …) wiped the administrator's credentials.
func TestRegisterCannotWipeExistingAccount(t *testing.T) {
	f := newFixture(t)
	f.setupAdmin(t)
	f.openRegistration(t)

	_, err := f.svc.Register(context.Background(), "admin", "evil@example.com", "hijack123")
	if err != ErrLoginTaken {
		t.Fatalf("Register on existing login: err = %v, want ErrLoginTaken", err)
	}
	if !f.canLogin("admin", "password123") {
		t.Fatal("administrator lost their password to open registration")
	}
}

// Ф2-1 (invited login). A pending invitation reserves its login: registering
// with it must fail and the invitee must still be able to complete the invite.
func TestRegisterCannotSquatPendingInvite(t *testing.T) {
	f := newFixture(t)
	adminID := f.setupAdmin(t)
	f.openRegistration(t)
	secret, err := f.svc.CreateInvite(adminID, "bob", "bob@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Register(context.Background(), "bob", "evil@example.com", "hijack123"); err != ErrLoginTaken {
		t.Fatalf("Register on invited login: err = %v, want ErrLoginTaken", err)
	}
	if err := f.svc.CompleteInvite(context.Background(), secret, "bobpass12"); err != nil {
		t.Fatalf("invitee cannot complete: %v", err)
	}
	if !f.canLogin("bob", "bobpass12") {
		t.Fatal("invitee cannot log in")
	}
}

// S-10.2: resubmitting the same login and address before confirmation
// replaces the password and the secret. The first confirmation link dies.
func TestRegisterResubmitReplacesSecret(t *testing.T) {
	f := newFixture(t)
	f.setupAdmin(t)
	f.openRegistration(t)
	tok1, err := f.svc.Register(context.Background(), "carol", "carol@example.com", "firstpass1")
	if err != nil {
		t.Fatal(err)
	}
	tok2, err := f.svc.Register(context.Background(), "carol", "carol@example.com", "secondpass")
	if err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	if f.svc.ConfirmEmail(context.Background(), tok1) {
		t.Fatal("first confirmation link should be dead after resubmit")
	}
	if !f.svc.ConfirmEmail(context.Background(), tok2) {
		t.Fatal("second confirmation link should work")
	}
	if f.canLogin("carol", "firstpass1") || !f.canLogin("carol", "secondpass") {
		t.Fatal("password was not replaced by the resubmission")
	}
	// A different address for an unconfirmed login is somebody else.
	if _, err := f.svc.Register(context.Background(), "dave", "dave@example.com", "davepass1"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Register(context.Background(), "dave", "other@example.com", "otherpass"); err != ErrLoginTaken {
		t.Fatalf("resubmit with another address: err = %v, want ErrLoginTaken", err)
	}
}

// Ф2-2. Resending an invitation replaces the secret (S-10.3): the previous
// link must stop working.
func TestInviteResendKillsOldSecret(t *testing.T) {
	f := newFixture(t)
	adminID := f.setupAdmin(t)
	first, err := f.svc.CreateInvite(adminID, "bob", "bob@example.com")
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.svc.CreateInvite(adminID, "bob", "bob@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if f.svc.InviteValid(context.Background(), first) {
		t.Fatal("first invite secret still valid after resend")
	}
	if !f.svc.InviteValid(context.Background(), second) {
		t.Fatal("second invite secret should be valid")
	}
	if err := f.svc.CompleteInvite(context.Background(), first, "bobpass12"); err != ErrNotFound {
		t.Fatalf("CompleteInvite with the old secret: err = %v, want ErrNotFound", err)
	}
}

func TestInviteExpiresAfterTTL(t *testing.T) {
	f := newFixture(t)
	adminID := f.setupAdmin(t)
	secret, err := f.svc.CreateInvite(adminID, "bob", "")
	if err != nil {
		t.Fatal(err)
	}
	f.advance(conf.InviteTTL + time.Second)
	if f.svc.InviteValid(context.Background(), secret) {
		t.Fatal("invite valid after TTL")
	}
}

// S-10.6: the journal names who issued the invitation, not a constant.
func TestCreateInviteRecordsActor(t *testing.T) {
	f := newFixture(t)
	adminID := f.setupAdmin(t)
	if _, err := f.svc.CreateInvite(adminID, "bob", ""); err != nil {
		t.Fatal(err)
	}
	for _, e := range f.st.Snapshot().Audit {
		if e.What == "user.invite.create" {
			if e.Who != "admin" || e.Whom != "bob" {
				t.Fatalf("audit who=%q whom=%q", e.Who, e.Whom)
			}
			return
		}
	}
	t.Fatal("no user.invite.create audit entry")
}

// architecture.md: repeating the console command issues a new secret and
// kills the previous one.
func TestConsoleLinkRepeatKillsOld(t *testing.T) {
	f := newFixture(t)
	f.setupAdmin(t)
	l1, err := f.svc.IssueConsoleWipeLink("admin")
	if err != nil {
		t.Fatal(err)
	}
	l2, err := f.svc.IssueConsoleWipeLink("admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := f.svc.ForgotWipeState(context.Background(), tokenOf(l1)); ok {
		t.Fatal("first console link still valid after repeat")
	}
	if _, _, ok := f.svc.ForgotWipeState(context.Background(), tokenOf(l2)); !ok {
		t.Fatal("second console link should be valid")
	}
	f.advance(conf.ConsoleWipeLinkTTL + time.Second)
	if _, _, ok := f.svc.ForgotWipeState(context.Background(), tokenOf(l2)); ok {
		t.Fatal("console link valid after 15 minutes")
	}
}

func tokenOf(link string) string { return link[len("/forgot/"):] }

// The full two-step wipe (S-10.4): confirm destroys credentials and ends
// sessions, the same token then sets a password, and afterwards the token is
// dead.
func TestForgotWipeTwoSteps(t *testing.T) {
	f := newFixture(t)
	id := f.setupAdmin(t)
	f.confirmEmailOf(t, "admin", "admin@example.com")
	sess, err := f.mgr.Create(id, "admin", [32]byte{1})
	if err != nil {
		t.Fatal(err)
	}

	tok, sent, asked, hasMail, err := f.svc.ForgotRequest(context.Background(), "ADMIN@example.com")
	if err != nil || !sent || asked || !hasMail {
		t.Fatalf("ForgotRequest = sent=%v asked=%v hasMail=%v err=%v", sent, asked, hasMail, err)
	}
	if login, step, ok := f.svc.ForgotWipeState(context.Background(), tok); !ok || login != "admin" || step != "confirm" {
		t.Fatalf("state before wipe = %q %q %v", login, step, ok)
	}
	// Step one: the password cannot be set before the wipe was confirmed.
	if err := f.svc.ForgotSetPassword(context.Background(), tok, "newpass123"); err != ErrNotFound {
		t.Fatalf("set password before confirm: err = %v, want ErrNotFound", err)
	}
	if err := f.svc.ForgotConfirmWipe(context.Background(), tok); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.mgr.Get(sess.ID()); ok {
		t.Fatal("session survived the wipe")
	}
	if f.canLogin("admin", "password123") {
		t.Fatal("old password still works after wipe")
	}
	if _, step, ok := f.svc.ForgotWipeState(context.Background(), tok); !ok || step != "password" {
		t.Fatalf("state after wipe = %q %v", step, ok)
	}
	// Step two.
	if err := f.svc.ForgotSetPassword(context.Background(), tok, "newpass123"); err != nil {
		t.Fatal(err)
	}
	if !f.canLogin("admin", "newpass123") {
		t.Fatal("new password does not work")
	}
	if _, _, ok := f.svc.ForgotWipeState(context.Background(), tok); ok {
		t.Fatal("token alive after the password was set")
	}
	snap := f.st.Snapshot()
	u := snap.UserByLogin("admin")
	if u.Role != store.RoleAdmin || !u.EmailConfirmed || u.Email != "admin@example.com" {
		t.Fatalf("identity changed by wipe: %+v", u)
	}
}

func TestForgotLinkExpires(t *testing.T) {
	f := newFixture(t)
	f.setupAdmin(t)
	f.confirmEmailOf(t, "admin", "admin@example.com")
	tok, _, _, _, err := f.svc.ForgotRequest(context.Background(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	f.advance(conf.ForgotLinkTTL + time.Second)
	if _, _, ok := f.svc.ForgotWipeState(context.Background(), tok); ok {
		t.Fatal("forgot link valid after 24 h")
	}
	if err := f.svc.ForgotConfirmWipe(context.Background(), tok); err != ErrNotFound {
		t.Fatalf("expired token wiped: err = %v", err)
	}
}

// Ф2-3. Setting a password burns every outstanding wipe token of that
// person, not only the one used: an older link must not wipe the account a
// second time.
func TestSetPasswordBurnsOtherWipeTokens(t *testing.T) {
	f := newFixture(t)
	f.setupAdmin(t)
	f.confirmEmailOf(t, "admin", "admin@example.com")
	console, err := f.svc.IssueConsoleWipeLink("admin")
	if err != nil {
		t.Fatal(err)
	}
	mail, _, _, _, err := f.svc.ForgotRequest(context.Background(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ForgotConfirmWipe(context.Background(), mail); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ForgotSetPassword(context.Background(), mail, "newpass123"); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := f.svc.ForgotWipeState(context.Background(), tokenOf(console)); ok {
		t.Fatal("console link alive after a password was set")
	}
	if err := f.svc.ForgotConfirmWipe(context.Background(), tokenOf(console)); err != ErrNotFound {
		t.Fatalf("old link wiped again: err = %v", err)
	}
	if !f.canLogin("admin", "newpass123") {
		t.Fatal("password destroyed by the old link")
	}
}

// Ф2-3 (invite). Completing an invitation is also setting a password.
func TestCompleteInviteBurnsWipeTokens(t *testing.T) {
	f := newFixture(t)
	adminID := f.setupAdmin(t)
	secret, err := f.svc.CreateInvite(adminID, "bob", "")
	if err != nil {
		t.Fatal(err)
	}
	console, err := f.svc.IssueConsoleWipeLink("bob")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.CompleteInvite(context.Background(), secret, "bobpass12"); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := f.svc.ForgotWipeState(context.Background(), tokenOf(console)); ok {
		t.Fatal("console link alive after the invitee set a password")
	}
}

// Ф2-4. Disabled accounts get the same /forgot answer as accounts without a
// confirmed address: no token is issued (S-10.6 — data stays intact).
func TestForgotRefusesDisabledAccount(t *testing.T) {
	f := newFixture(t)
	adminID := f.setupAdmin(t)
	if _, err := f.svc.CreateInvite(adminID, "bob", ""); err != nil {
		t.Fatal(err)
	}
	f.confirmEmailOf(t, "bob", "bob@example.com")
	bobID := f.idOf(t, "bob")
	if err := f.svc.DisableUser(adminID, bobID); err != nil {
		t.Fatal(err)
	}
	tok, sent, asked, hasMail, err := f.svc.ForgotRequest(context.Background(), "bob")
	if err != nil || tok != "" || sent || asked || hasMail {
		t.Fatalf("disabled account got a wipe token: sent=%v asked=%v hasMail=%v", sent, asked, hasMail)
	}
	if len(f.st.Snapshot().Tokens) != 0 {
		t.Fatal("token stored for a disabled account")
	}
}

func TestForgotUnknownAndUnconfirmedLookAlike(t *testing.T) {
	f := newFixture(t)
	f.setupAdmin(t) // no email at all
	for _, who := range []string{"admin", "nobody", "nobody@example.com"} {
		tok, sent, asked, hasMail, err := f.svc.ForgotRequest(context.Background(), who)
		if err != nil || tok != "" || sent || asked || hasMail {
			t.Fatalf("%q: sent=%v asked=%v hasMail=%v err=%v", who, sent, asked, hasMail, err)
		}
	}
}

// Ф2-5. Used and expired tokens are dropped, otherwise every /forgot POST
// grows the state file forever and every Mutate re-encodes it.
func TestUsedAndExpiredTokensArePruned(t *testing.T) {
	f := newFixture(t)
	f.setupAdmin(t)
	f.confirmEmailOf(t, "admin", "admin@example.com")
	if _, _, _, _, err := f.svc.ForgotRequest(context.Background(), "admin"); err != nil {
		t.Fatal(err)
	}
	f.advance(conf.ForgotLinkTTL + time.Second)
	live, _, _, _, err := f.svc.ForgotRequest(context.Background(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(f.st.Snapshot().Tokens); n != 1 {
		t.Fatalf("tokens after expiry = %d, want 1", n)
	}
	if _, _, ok := f.svc.ForgotWipeState(context.Background(), live); !ok {
		t.Fatal("live token pruned")
	}
	if err := f.svc.ForgotConfirmWipe(context.Background(), live); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ForgotSetPassword(context.Background(), live, "newpass123"); err != nil {
		t.Fatal(err)
	}
	if n := len(f.st.Snapshot().Tokens); n != 0 {
		t.Fatalf("tokens after use = %d, want 0", n)
	}
}

// productionLoginKDF is the sign-in KDF as it ships, read before TestMain
// makes it cheap (package variables are set before TestMain runs). With the
// cheap values the KDF is lost in the rest of a sign-in, and under a full
// run's load the test below failed on noise (24.09.2026).
var productionLoginKDF = crypt.LoginKDF

// Ф2-6. An unknown login must cost the same as a wrong password: without a
// dummy derivation the miss path skipped Argon2 and answered in microseconds.
func TestAuthenticateUnknownLoginCostsAKDF(t *testing.T) {
	cheap := crypt.LoginKDF
	crypt.LoginKDF = productionLoginKDF // what is measured is the KDF (main_test.go)
	t.Cleanup(func() { crypt.LoginKDF = cheap })
	f := newFixture(t)
	f.setupAdmin(t)
	f.canLogin("nobody", "x") // warm the dummy record

	measure := func(login string) time.Duration {
		var best time.Duration
		for i := 0; i < 3; i++ {
			st := time.Now()
			f.canLogin(login, "wrongpassword")
			if d := time.Since(st); best == 0 || d < best {
				best = d
			}
		}
		return best
	}
	known := measure("admin")
	unknown := measure("nobody")
	if unknown*3 < known {
		t.Fatalf("unknown login answered in %v, known login in %v: enumeration by timing", unknown, known)
	}
}

func TestAuthenticateRefusesDisabledAndUnconfirmed(t *testing.T) {
	f := newFixture(t)
	adminID := f.setupAdmin(t)
	f.openRegistration(t)
	if _, err := f.svc.Register(context.Background(), "carol", "carol@example.com", "carolpass1"); err != nil {
		t.Fatal(err)
	}
	if f.canLogin("carol", "carolpass1") {
		t.Fatal("unconfirmed self-registration can log in")
	}
	secret, err := f.svc.CreateInvite(adminID, "bob", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.CompleteInvite(context.Background(), secret, "bobpass12"); err != nil {
		t.Fatal(err)
	}
	bobID := f.idOf(t, "bob")
	if err := f.svc.DisableUser(adminID, bobID); err != nil {
		t.Fatal(err)
	}
	if f.canLogin("bob", "bobpass12") {
		t.Fatal("disabled account can log in")
	}
}

// Unlock (S-10.7) verifies the password of the session's own account and
// writes the journal line.
func TestUnlockVerifiesPasswordAndJournals(t *testing.T) {
	f := newFixture(t)
	id := f.setupAdmin(t)
	if f.svc.Unlock(context.Background(), id, "wrongpassword") {
		t.Fatal("unlock with a wrong password")
	}
	if f.svc.Unlock(context.Background(), "a-nobody0000", "password123") {
		t.Fatal("unlock for an unknown account")
	}
	if !f.svc.Unlock(context.Background(), id, "password123") {
		t.Fatal("unlock with the right password refused")
	}
	found := false
	for _, e := range f.st.Snapshot().Audit {
		if e.What == "secret.unlock" && e.Who == "admin" {
			found = true
		}
	}
	if !found {
		t.Fatal("unlock not journaled")
	}
}

// Ф2-7. {account} is a connection (C-4), owned when it sits in the viewer's
// sealed blob; the viewer's own user ID is not a connection.
func TestOwnsAccountByConnectionInBlob(t *testing.T) {
	f := newFixture(t)
	adminID := f.setupAdmin(t)
	secret, err := f.svc.CreateInvite(adminID, "bob", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.CompleteInvite(context.Background(), secret, "bobpass12"); err != nil {
		t.Fatal(err)
	}
	adminKey := putConnection(t, f, "admin", "password123", "a-conn0admin")
	bobKey := putConnection(t, f, "bob", "bobpass12", "a-conn000bob")
	bobID := f.idOf(t, "bob")

	if !f.svc.OwnsAccount(adminID, adminKey, "a-conn0admin") {
		t.Fatal("own connection not owned")
	}
	if f.svc.OwnsAccount(adminID, adminKey, "a-conn000bob") {
		t.Fatal("foreign connection owned")
	}
	if f.svc.OwnsAccount(adminID, adminKey, adminID) {
		t.Fatal("user ID accepted as a connection")
	}
	if f.svc.OwnsAccount(bobID, adminKey, "a-conn000bob") {
		t.Fatal("foreign key opened the blob")
	}
	if !f.svc.OwnsAccount(bobID, bobKey, "a-conn000bob") {
		t.Fatal("bob does not own his connection")
	}
}

func putConnection(t *testing.T, f *fixture, login, password, connID string) [32]byte {
	t.Helper()
	_, key, _, ok := f.svc.Authenticate(context.Background(), login, password)
	if !ok {
		t.Fatalf("login %q failed", login)
	}
	blob, err := sealed.Seal(key, sealed.Blob{Connections: []sealed.Connection{{ID: connID, Server: "https://dav.example.com"}}}, login)
	if err != nil {
		t.Fatal(err)
	}
	err = f.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByLogin(login)
		u.SealedBlob = blob
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// S-10.6, s-admin-12: a disabled account is enabled again from its row; the
// person signs in with the password they had, and the journal says who let
// them back.
func TestEnableUserLetsThemBackIn(t *testing.T) {
	f := newFixture(t)
	adminID := f.setupAdmin(t)
	secret, err := f.svc.CreateInvite(adminID, "bob", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.CompleteInvite(context.Background(), secret, "bobpass123"); err != nil {
		t.Fatal(err)
	}
	bobID := f.idOf(t, "bob")
	if err := f.svc.DisableUser(adminID, bobID); err != nil {
		t.Fatal(err)
	}
	if _, _, _, ok := f.svc.Authenticate(context.Background(), "bob", "bobpass123"); ok {
		t.Fatal("a disabled account signed in")
	}
	if err := f.svc.EnableUser(adminID, bobID); err != nil {
		t.Fatal(err)
	}
	if _, _, _, ok := f.svc.Authenticate(context.Background(), "bob", "bobpass123"); !ok {
		t.Fatal("enabled, bob cannot sign in")
	}
	snap := f.st.Snapshot()
	var last store.AuditEntry
	for _, e := range snap.Audit {
		if e.What == "user.enable" {
			last = e
		}
	}
	if last.Who != "admin" || last.Whom != "bob" {
		t.Errorf("journal = %+v", snap.Audit)
	}
	if err := f.svc.EnableUser(adminID, "u-nobody"); err != ErrNotFound {
		t.Errorf("unknown account: %v", err)
	}
}
