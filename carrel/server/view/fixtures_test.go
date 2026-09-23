package view

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// fixtureDir is docs/spec/fixtures relative to this package, three levels
// up from carrel/server/view to the repository root.
const fixtureDir = "../../../docs/spec/fixtures"

// fixtureTypes maps a fixture screen directory (C-14.3) to the view model
// it decodes into. Only screens with fixtures on disk need an entry; add
// one here in the same commit as its fixtures.
//
// Two entries carried this check from v0.0.3 to 23.09.2026 while fixtures
// for sixty screens went in without one, so the test was red and nobody saw
// it: the Linux gates ran the live suites and the build, never this. They
// run the unit suite now, and the map holds every screen on disk. A screen
// whose fixture is a fragment rather than a whole page names the fragment:
// app-note-edit is a NoteDoc, the editor swapped in place (C-6.9).
var fixtureTypes = map[string]func() any{
	"about":                    func() any { return &AboutPage{} },
	"admin-audit":              func() any { return &AdminAuditPage{} },
	"admin-dav":                func() any { return &AdminDavPage{} },
	"admin-escrow":             func() any { return &AdminEscrowPage{} },
	"admin-install":            func() any { return &AdminInstallPage{} },
	"admin-invites":            func() any { return &AdminInvitesPage{} },
	"admin-settings":           func() any { return &AdminSettingsPage{} },
	"admin-users":              func() any { return &AdminUsersPage{} },
	"app-agenda":               func() any { return &AgendaPage{} },
	"app-agenda-all":           func() any { return &AgendaPage{} },
	"app-collections-delete":   func() any { return &CollectionDeleteSheet{} },
	"app-collections-move":     func() any { return &CollectionMoveSheet{} },
	"app-collections-new":      func() any { return &CollectionNewSheet{} },
	"app-collections-rename":   func() any { return &CollectionRenameSheet{} },
	"app-conflict":             func() any { return &ConflictPage{} },
	"app-conflict-event":       func() any { return &ConflictPage{} },
	"app-conflict-note":        func() any { return &ConflictPage{} },
	"app-conflict-task":        func() any { return &ConflictPage{} },
	"app-contact":              func() any { return &ContactPage{} },
	"app-contact-edit":         func() any { return &ContactEditPage{} },
	"app-contact-import":       func() any { return &ImportPage{} },
	"app-contact-move":         func() any { return &MoveSheet{} },
	"app-contact-new":          func() any { return &ContactEditPage{} },
	"app-contact-timeline":     func() any { return &ContactTimelinePage{} },
	"app-contacts":             func() any { return &ContactsListPage{} },
	"app-contacts-one":         func() any { return &ContactsListPage{} },
	"app-duplicates":           func() any { return &DuplicatesPage{} },
	"app-duplicates-merge":     func() any { return &DuplicatesMergePage{} },
	"app-event":                func() any { return &EventPage{} },
	"app-event-edit":           func() any { return &EventEditPage{} },
	"app-files":                func() any { return &FilesRootPage{} },
	"app-files-picker":         func() any { return &FilePickerSheet{} },
	"app-files-published":      func() any { return &PublishedLinksPage{} },
	"app-mail-compose":         func() any { return &MailSheet{} },
	"app-note":                 func() any { return &NotePage{} },
	"app-note-edit":            func() any { return &NoteDoc{} },
	"app-notes":                func() any { return &NotesPage{} },
	"app-notes-all":            func() any { return &NotesPage{} },
	"app-password":             func() any { return &PasswordPage{} },
	"app-photo-preview":        func() any { return &PhotoPreviewPage{} },
	"app-publish":              func() any { return &PublishSheet{} },
	"app-quick":                func() any { return &NewSheet{} },
	"app-search":               func() any { return &SearchPage{} },
	"app-settings-account":     func() any { return &AccountPage{} },
	"app-settings-appearance":  func() any { return &AppearancePage{} },
	"app-settings-attachments": func() any { return &AttachmentsPage{} },
	"app-settings-backup":      func() any { return &BackupPage{} },
	"app-settings-connections": func() any { return &ConnectionsPage{} },
	"app-settings-davloom":     func() any { return &DavloomPage{} },
	"app-settings-devices":     func() any { return &DevicesPage{} },
	"app-settings-escrow":      func() any { return &EscrowPage{} },
	"app-settings-unlock":      func() any { return &UnlockSheet{} },
	"app-task":                 func() any { return &TaskPage{} },
	"app-task-edit":            func() any { return &TaskEditPage{} },
	"app-tasks":                func() any { return &TasksPage{} },
	"app-tasks-all":            func() any { return &TasksPage{} },
	"forgot":                   func() any { return &ForgotPage{} },
	"forgot-token":             func() any { return &ForgotWipePage{} },
	"invite":                   func() any { return &InvitePage{} },
	"login":                    func() any { return &LoginPage{} },
	"register":                 func() any { return &RegisterPage{} },
	"setup":                    func() any { return &SetupPage{} },
}

// TestFixturesDecode confirms that every existing fixture unmarshals into
// its declared view model with no unknown fields, and that decoding then
// re-encoding loses or invents nothing (compared as decoded JSON trees,
// not as byte-for-byte strings — key order and spacing are not the
// contract).
func TestFixturesDecode(t *testing.T) {
	screens, err := os.ReadDir(fixtureDir)
	if err != nil {
		t.Fatalf("read fixture dir: %v", err)
	}
	found := 0
	for _, screen := range screens {
		if !screen.IsDir() {
			continue
		}
		newModel, ok := fixtureTypes[screen.Name()]
		if !ok {
			t.Errorf("fixture screen %q has no entry in fixtureTypes", screen.Name())
			continue
		}
		states, err := os.ReadDir(filepath.Join(fixtureDir, screen.Name()))
		if err != nil {
			t.Fatalf("read fixture screen %q: %v", screen.Name(), err)
		}
		for _, state := range states {
			if state.IsDir() || filepath.Ext(state.Name()) != ".json" {
				continue
			}
			found++
			path := filepath.Join(fixtureDir, screen.Name(), state.Name())
			t.Run(screen.Name()+"/"+state.Name(), func(t *testing.T) {
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read fixture: %v", err)
				}

				model := newModel()
				dec := json.NewDecoder(bytes.NewReader(raw))
				dec.DisallowUnknownFields()
				if err := dec.Decode(model); err != nil {
					t.Fatalf("decode into %T: %v", model, err)
				}

				var want, got any
				if err := json.Unmarshal(raw, &want); err != nil {
					t.Fatalf("decode fixture as generic JSON: %v", err)
				}
				roundTrip, err := json.Marshal(model)
				if err != nil {
					t.Fatalf("re-encode %T: %v", model, err)
				}
				if err := json.Unmarshal(roundTrip, &got); err != nil {
					t.Fatalf("decode round-trip as generic JSON: %v", err)
				}

				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("fixture round-trip mismatch (-fixture +decoded/re-encoded):\n%s", diff)
				}
			})
		}
	}
	if found == 0 {
		t.Fatal("no fixture files found under " + fixtureDir)
	}
}
