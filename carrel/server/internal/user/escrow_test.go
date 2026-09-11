package user_test

import (
	"context"
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
	temp, err := svc.RecoverEscrow(ctx, adminID, "bob", master)
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
	if err := svc.CompleteInvite(ctx, secret, "bobpass1234"); err != nil {
		t.Fatal(err)
	}
	_, _, asked, _, err := svc.ForgotRequest(ctx, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if !asked {
		t.Fatal("deposited account should ask administrator on /forgot")
	}
}
