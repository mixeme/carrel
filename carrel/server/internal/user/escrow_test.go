package user_test

import (
	"context"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/store"
)

func TestEscrowNotRetroactive(t *testing.T) {
	svc, st := testService(t)
	ctx := context.Background()
	if err := svc.SetupFirstAdmin(ctx, "admin", "password123"); err != nil {
		t.Fatal(err)
	}
	snap := st.Snapshot()
	adminID := snap.UserByLogin("admin").ID
	if err := svc.EnableEscrow(ctx, adminID, "sixteen-char-pass!", "sixteen-char-pass!"); err != nil {
		t.Fatal(err)
	}
	snap = st.Snapshot()
	admin := snap.UserByLogin("admin")
	if store.UserDeposited(admin) {
		t.Fatal("existing admin must not be deposited retroactively")
	}
	secret, err := svc.CreateInvite(adminID, "bob", "bob@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.CompleteInvite(ctx, secret, "bobpass1234"); err != nil {
		t.Fatal(err)
	}
	snap = st.Snapshot()
	bob := snap.UserByLogin("bob")
	if !store.UserDeposited(bob) {
		t.Fatal("new user after enable should be deposited on password set")
	}
}

func TestEscrowWithdrawRemovesCopy(t *testing.T) {
	svc, st := testService(t)
	ctx := context.Background()
	if err := svc.SetupFirstAdmin(ctx, "admin", "password123"); err != nil {
		t.Fatal(err)
	}
	snap := st.Snapshot()
	adminID := snap.UserByLogin("admin").ID
	if err := svc.EnableEscrow(ctx, adminID, "sixteen-char-pass!", "sixteen-char-pass!"); err != nil {
		t.Fatal(err)
	}
	secret, err := svc.CreateInvite(adminID, "bob", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.CompleteInvite(ctx, secret, "bobpass1234"); err != nil {
		t.Fatal(err)
	}
	_, key, _, ok := svc.Authenticate(ctx, "bob", "bobpass1234")
	if !ok {
		t.Fatal("auth")
	}
	snap = st.Snapshot()
	bobID := snap.UserByLogin("bob").ID
	if err := svc.WithdrawEscrow(bobID); err != nil {
		t.Fatal(err)
	}
	snap = st.Snapshot()
	bob := snap.UserByLogin("bob")
	if store.UserDeposited(bob) || bob.EscrowOptIn {
		t.Fatal("withdraw should clear deposit and opt-in")
	}
	if err := svc.DepositEscrow(bobID, key); err != nil {
		t.Fatal(err)
	}
	snap = st.Snapshot()
	if !store.UserDeposited(snap.UserByLogin("bob")) {
		t.Fatal("manual deposit should work")
	}
}

func TestEscrowRecoverRewrapsDataKey(t *testing.T) {
	svc, st := testService(t)
	ctx := context.Background()
	if err := svc.SetupFirstAdmin(ctx, "admin", "password123"); err != nil {
		t.Fatal(err)
	}
	snap := st.Snapshot()
	adminID := snap.UserByLogin("admin").ID
	master := "sixteen-char-pass!"
	if err := svc.EnableEscrow(ctx, adminID, master, master); err != nil {
		t.Fatal(err)
	}
	secret, err := svc.CreateInvite(adminID, "bob", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.CompleteInvite(ctx, secret, "bobpass1234"); err != nil {
		t.Fatal(err)
	}
	temp, err := svc.RecoverEscrow(ctx, adminID, "bob", master, "")
	if err != nil {
		t.Fatal(err)
	}
	_, key, _, ok := svc.Authenticate(ctx, "bob", temp)
	if !ok {
		t.Fatal("temp password should work")
	}
	if key == [32]byte{} {
		t.Fatal("data key should survive recovery")
	}
	snap = st.Snapshot()
	bob := snap.UserByLogin("bob")
	if !bob.MustChangePassword {
		t.Fatal("recovery should require password change")
	}
}

func TestForgotEscrowRequiresAdminWhenDeposited(t *testing.T) {
	svc, st := testService(t)
	ctx := context.Background()
	if err := svc.SetupFirstAdmin(ctx, "admin", "password123"); err != nil {
		t.Fatal(err)
	}
	snap := st.Snapshot()
	adminID := snap.UserByLogin("admin").ID
	if err := svc.EnableEscrow(ctx, adminID, "sixteen-char-pass!", "sixteen-char-pass!"); err != nil {
		t.Fatal(err)
	}
	secret, err := svc.CreateInvite(adminID, "bob", "bob@example.com")
	if err != nil {
		t.Fatal(err)
	}
	// The invitation went by letter, so it proved the address.
	if err := svc.MarkInviteMailed(secret, true, ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.CompleteInvite(ctx, secret, "bobpass1234"); err != nil {
		t.Fatal(err)
	}
	if _, hasMail, canAsk := svc.ForgotLookup("bob"); !hasMail || !canAsk {
		t.Fatalf("deposited account: hasMail=%v canAsk=%v, want the administrator offered on /forgot", hasMail, canAsk)
	}
	if asked, err := svc.ForgotAsk(ctx, "bob"); err != nil || !asked {
		t.Fatalf("ForgotAsk = %v, %v", asked, err)
	}
}

func TestEscrowRecoverBurnsWipeTokens(t *testing.T) {
	svc, st := testService(t)
	ctx := context.Background()
	if err := svc.SetupFirstAdmin(ctx, "admin", "password123"); err != nil {
		t.Fatal(err)
	}
	snap := st.Snapshot()
	adminID := snap.UserByLogin("admin").ID
	master := "sixteen-char-pass!"
	if err := svc.EnableEscrow(ctx, adminID, master, master); err != nil {
		t.Fatal(err)
	}
	secret, err := svc.CreateInvite(adminID, "bob", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.CompleteInvite(ctx, secret, "bobpass1234"); err != nil {
		t.Fatal(err)
	}
	link, err := svc.IssueConsoleWipeLink("bob")
	if err != nil {
		t.Fatal(err)
	}
	token := strings.TrimPrefix(link, "/forgot/")
	if _, err := svc.RecoverEscrow(ctx, adminID, "bob", master, ""); err != nil {
		t.Fatal(err)
	}
	if _, _, valid := svc.ForgotWipeState(ctx, token); valid {
		t.Fatal("wipe token still valid after escrow recovery")
	}
	snap = st.Snapshot()
	bob := snap.UserByLogin("bob")
	if bob == nil || len(bob.SealedBlob) == 0 {
		t.Fatal("blob should survive recovery")
	}
	if last := snap.Audit[len(snap.Audit)-1]; last.What != "escrow.recover" || last.Detail != "" {
		t.Fatalf("recover audit = %+v, want empty Detail until mail is sent", last)
	}
}
