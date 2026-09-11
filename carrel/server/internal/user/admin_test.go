package user_test

import (
	"context"
	"testing"

	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
)

func TestRevokeInviteRecordsActor(t *testing.T) {
	svc, st := testService(t)
	_ = svc.SetupFirstAdmin(context.Background(), "admin", "password123")
	adminID, _, _, _ := svc.Authenticate(context.Background(), "admin", "password123")
	secret, err := svc.CreateInvite(adminID, "bob", "bob@example.com")
	if err != nil {
		t.Fatal(err)
	}
	snap := st.Snapshot()
	var invID string
	for i := range snap.Invites {
		if snap.Invites[i].Login == "bob" {
			invID = snap.Invites[i].ID
			break
		}
	}
	if err := svc.RevokeInvite(adminID, invID); err != nil {
		t.Fatal(err)
	}
	if svc.InviteValid(context.Background(), secret) {
		t.Fatal("revoked invite still valid")
	}
	for _, e := range st.Snapshot().Audit {
		if e.What == "user.invite.revoke" && e.Who == "admin" {
			return
		}
	}
	t.Fatal("no user.invite.revoke audit entry")
}

func TestExtendInviteKeepsSecret(t *testing.T) {
	svc, st := testService(t)
	_ = svc.SetupFirstAdmin(context.Background(), "admin", "password123")
	adminID, _, _, _ := svc.Authenticate(context.Background(), "admin", "password123")
	secret, err := svc.CreateInvite(adminID, "bob", "")
	if err != nil {
		t.Fatal(err)
	}
	snap := st.Snapshot()
	invID := snap.Invites[len(snap.Invites)-1].ID
	if err := svc.ExtendInvite(adminID, invID); err != nil {
		t.Fatal(err)
	}
	if !svc.InviteValid(context.Background(), secret) {
		t.Fatal("extend broke the secret")
	}
}

func TestDeleteUserNeedsLoginConfirm(t *testing.T) {
	svc, st := testService(t)
	_ = svc.SetupFirstAdmin(context.Background(), "admin", "password123")
	adminID, _, _, _ := svc.Authenticate(context.Background(), "admin", "password123")
	secret, err := svc.CreateInvite(adminID, "bob", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.CompleteInvite(context.Background(), secret, "bobpass12"); err != nil {
		t.Fatal(err)
	}
	snap := st.Snapshot()
	bob := snap.UserByLogin("bob")
	if bob == nil {
		t.Fatal("no bob")
	}
	bobID := bob.ID
	if err := svc.DeleteUser(adminID, bobID, "wrong"); err != user.ErrLoginConfirm {
		t.Fatalf("DeleteUser: %v, want ErrLoginConfirm", err)
	}
	if err := svc.DeleteUser(adminID, bobID, "bob"); err != nil {
		t.Fatal(err)
	}
	snap = st.Snapshot()
	if snap.UserByLogin("bob") != nil {
		t.Fatal("bob still present")
	}
}

func TestSetUserRoleProtectsLastAdmin(t *testing.T) {
	svc, _ := testService(t)
	_ = svc.SetupFirstAdmin(context.Background(), "admin", "password123")
	adminID, _, _, _ := svc.Authenticate(context.Background(), "admin", "password123")
	if err := svc.SetUserRole(adminID, adminID, store.RoleUser); err != user.ErrLastAdmin {
		t.Fatalf("SetUserRole: %v, want ErrLastAdmin", err)
	}
}

func TestAdminResetPasswordUsesActor(t *testing.T) {
	svc, st := testService(t)
	_ = svc.SetupFirstAdmin(context.Background(), "admin", "password123")
	adminID, _, _, _ := svc.Authenticate(context.Background(), "admin", "password123")
	secret, err := svc.CreateInvite(adminID, "bob", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.CompleteInvite(context.Background(), secret, "bobpass12"); err != nil {
		t.Fatal(err)
	}
	snap := st.Snapshot()
	bob := snap.UserByLogin("bob")
	if bob == nil {
		t.Fatal("no bob")
	}
	bobID := bob.ID
	tok, _, esc, hand, err := svc.AdminResetPassword(context.Background(), adminID, bobID)
	if err != nil || esc || !hand || tok == "" {
		t.Fatalf("AdminResetPassword = tok=%q esc=%v hand=%v err=%v", tok, esc, hand, err)
	}
	found := false
	for _, e := range st.Snapshot().Audit {
		if e.What == "user.reset.request" && e.Who == "admin" && e.Whom == "bob" {
			found = true
		}
	}
	if !found {
		t.Fatal("reset not journaled with actor")
	}
}

func TestDeleteUserProtectsLastAdmin(t *testing.T) {
	svc, _ := testService(t)
	_ = svc.SetupFirstAdmin(context.Background(), "admin", "password123")
	adminID, _, _, _ := svc.Authenticate(context.Background(), "admin", "password123")
	if err := svc.DeleteUser(adminID, adminID, "admin"); err != user.ErrLastAdmin {
		t.Fatalf("DeleteUser: %v, want ErrLastAdmin", err)
	}
}
