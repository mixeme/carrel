package user_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
)

// The block «Users’ own mailboxes» (carrel-mailbox.md M-2, s-mailbox-29):
// a form that does not carry it leaves what is stored; one that does stores
// it, and the journal names the row, as it names people's letters.
func TestSaveTheMailboxesSwitch(t *testing.T) {
	svc, st := testService(t)
	if err := svc.SetupFirstAdmin(context.Background(), "admin", "password123"); err != nil {
		t.Fatal(err)
	}
	admin := st.Snapshot().Users[0].ID
	on := true
	if refused, err := svc.SaveSettings(admin, user.InstanceUpdate{Mailboxes: &on}, store.SMTPSettings{}); err != nil || len(refused) > 0 {
		t.Fatalf("save: %v %v", refused, err)
	}
	if m := svc.Settings().Mailboxes; m == nil || !*m {
		t.Fatalf("stored = %v, want on", m)
	}
	if refused, err := svc.SaveSettings(admin, user.InstanceUpdate{}, store.SMTPSettings{}); err != nil || len(refused) > 0 {
		t.Fatalf("save without the block: %v %v", refused, err)
	}
	if m := svc.Settings().Mailboxes; m == nil || !*m {
		t.Fatalf("a form without the block changed it: %v", m)
	}
	journal := st.Snapshot().Audit
	if !slices.ContainsFunc(journal, func(e store.AuditEntry) bool {
		return e.What == "settings.instance" && strings.Contains(e.Detail, "mailboxes")
	}) {
		t.Errorf("the journal does not name the row: %+v", journal)
	}
}
