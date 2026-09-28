package user_test

import (
	"context"
	"strings"
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

// s-admin-13, S-10.6: the reset opens the account with the administrator's
// temporary password, which the person changes at the next sign-in; the old
// password is gone and with it what it sealed. Every wipe link of the person
// dies with it (Ф2-4); the journal names who did it and never the password.
// A disabled account and one's own are not reset here.
func TestAdminResetPasswordUsesActor(t *testing.T) {
	svc, st := testService(t)
	ctx := context.Background()
	_ = svc.SetupFirstAdmin(ctx, "admin", "password123")
	adminID, _, _, _ := svc.Authenticate(ctx, "admin", "password123")
	secret, err := svc.CreateInvite(adminID, "bob", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.CompleteInvite(ctx, secret, "bobpass12"); err != nil {
		t.Fatal(err)
	}
	first := st.Snapshot()
	bobID := first.UserByLogin("bob").ID
	link, err := svc.IssueConsoleWipeLink("bob")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AdminResetPassword(ctx, adminID, bobID, "short"); err == nil {
		t.Fatal("a short temporary password was taken")
	}
	if err := svc.AdminResetPassword(ctx, adminID, adminID, "tide-oven-4c1-lamp"); err == nil {
		t.Fatal("the administrator reset their own password here")
	}
	if err := svc.AdminResetPassword(ctx, adminID, bobID, "tide-oven-4c1-lamp"); err != nil {
		t.Fatal(err)
	}
	if id, _, _, _ := svc.Authenticate(ctx, "bob", "bobpass12"); id != "" {
		t.Error("the old password still opens the account")
	}
	if id, _, _, _ := svc.Authenticate(ctx, "bob", "tide-oven-4c1-lamp"); id != bobID {
		t.Error("the temporary password does not open the account")
	}
	snap := st.Snapshot()
	if bob := snap.UserByID(bobID); !bob.MustChangePassword || bob.ConnCount != 0 {
		t.Errorf("bob after reset = must change %v, connections %d", bob.MustChangePassword, bob.ConnCount)
	}
	if _, _, valid := svc.ForgotWipeState(ctx, strings.TrimPrefix(link, "/forgot/")); valid {
		t.Error("a wipe link outlived the reset")
	}
	found := false
	for _, e := range snap.Audit {
		if strings.Contains(e.Detail, "tide-oven") {
			t.Errorf("the password is in the journal: %+v", e)
		}
		if e.What == "user.reset" && e.Who == "admin" && e.Whom == "bob" {
			found = true
		}
	}
	if !found {
		t.Fatal("reset not journaled with actor")
	}
	if err := svc.DisableUser(adminID, bobID); err != nil {
		t.Fatal(err)
	}
	if err := svc.AdminResetPassword(ctx, adminID, bobID, "tide-oven-4c1-lamp"); err == nil {
		t.Error("a disabled account was reset")
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
