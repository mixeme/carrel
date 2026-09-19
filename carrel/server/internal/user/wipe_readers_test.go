package user

import (
	"context"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

// S-10.4: a wipe destroys the connections. Three things read them without
// the person's password and outlived the wipe: a published link (its
// envelope holds the connection's credentials under server.key), an
// external backup trigger (its envelope holds the old data key) and a
// davloom device. A link made by whoever held the account kept reading
// after the owner wiped it to lock them out (audit №3, Ф3-6).
func TestWipeRevokesEveryReaderOfTheConnections(t *testing.T) {
	f := newFixture(t)
	id := f.setupAdmin(t)
	ctx := context.Background()
	_, key, _, _ := f.svc.Authenticate(ctx, "admin", "password123")
	conn := sealed.Connection{Server: "https://dav.example.org/", Login: "ann", Password: "pw"}
	account, err := f.svc.AddConnection(id, key, conn, []sealed.Collection{{Path: "/book/", Name: "Book", Kinds: []view.Kind{view.KindContact}}})
	if err != nil {
		t.Fatal(err)
	}
	conn.ID = account
	link, err := f.svc.Publish(id, key, conn, PublishTarget{AccountID: account, Kind: "contact", Col: "/book/", Title: "Book"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	jobID, err := f.svc.SaveBackupJob(id, key, sealed.BackupJob{Name: "j", Mode: sealed.BackupToDownload, Keep: 1})
	if err != nil {
		t.Fatal(err)
	}
	trigger, err := f.svc.EnableBackupTrigger(id, key, jobID)
	if err != nil {
		t.Fatal(err)
	}
	_, devicePW, err := f.svc.IssueDevice(ctx, id, key, "Phone", DeviceScope{Kinds: []view.Kind{view.KindContact}, Cols: []store.DeviceScope{{AccountID: account, Path: "/book/"}}})
	if err != nil {
		t.Fatal(err)
	}
	console, err := f.svc.IssueConsoleWipeLink("admin")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ForgotConfirmWipe(ctx, strings.TrimPrefix(console, "/forgot/")); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.svc.OpenPublished(link); ok {
		t.Error("a published link still opens after the wipe")
	}
	if n := len(f.svc.PublishedLinks(id)); n != 0 {
		t.Errorf("%d links listed after the wipe", n)
	}
	if _, _, _, ok := f.svc.OpenBackupTrigger(trigger); ok {
		t.Error("the external trigger still opens after the wipe")
	}
	if f.svc.BackupTriggerOn(id, jobID) {
		t.Error("the trigger is still on after the wipe")
	}
	if _, _, _, ok := f.svc.UnlockDevice(ctx, "admin", devicePW); ok {
		t.Error("a device still opens after the wipe")
	}
	for _, b := range f.st.Snapshot().Backups {
		if b.UserID == id && (len(b.TriggerHash) > 0 || len(b.KeyEnvelope) > 0) {
			t.Error("the state still holds the trigger's secret or the old key's envelope")
		}
	}
	for _, l := range f.st.Snapshot().Published {
		if l.UserID == id {
			t.Error("the state still holds a link's envelope")
		}
	}
}
