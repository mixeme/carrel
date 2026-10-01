package view

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
// whose fixture is a fragment rather than a whole page would name the
// fragment; app-note-edit was one, a bare NoteDoc, until the edit of a note
// became its page (s-notes-20, from-typecase note 9).
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
	"app-contact-rewrite":      func() any { return &RewriteSheet{} },
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
	"app-note-edit":            func() any { return &NotePage{} },
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
	"confirm-email":            func() any { return &ConfirmEmailPage{} },
	"dead-link":                func() any { return &DeadLinkPage{} },
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

// C-6.2 (from-typecase notes 11 and 17): an action that cannot be taken is
// shown, switched off, with its reason; one that can carries no reason — an
// explanation beside a working button is a note the frame draws, not a
// Reason. The segment, tab or preset shown now is On and can be taken
// again: being shown is no reason to switch it off. Every fixture is walked,
// as its view model, for every Action in it.
func TestFixtureActionsSayWhyTheyAreOff(t *testing.T) {
	eachFixtureAction(t, func(screen, at string, a Action) {
		switch {
		case a.ID == "":
			// an empty place: no action at all
		case a.Enabled && a.Reason != "":
			t.Errorf("%s %s: %s «%s» can be taken and carries a reason %q", screen, at, a.ID, a.Label, a.Reason)
		case !a.Enabled && a.Reason == "":
			t.Errorf("%s %s: %s «%s» is off without a reason", screen, at, a.ID, a.Label)
		case a.Reason == "Shown now.":
			t.Errorf("%s %s: %s «%s» is switched off for being shown: it is On and enabled", screen, at, a.ID, a.Label)
		}
	})
}

// C-11, the client's stop C15: a POST is sent without scripts as a form
// of its own, and the server takes none without the session's token. Every
// POST action of a screen behind sign-in (app-*, admin-*) carries it — the
// buttons of a form too, so the client never asks which form a button is
// in. Pages outside the shell put theirs in their forms' _csrf.
func TestFixturePostActionsCarryTheToken(t *testing.T) {
	eachFixtureAction(t, func(screen, at string, a Action) {
		signed := strings.HasPrefix(screen, "app-") || strings.HasPrefix(screen, "admin-")
		if signed && a.Method == "POST" && a.CSRF == "" {
			t.Errorf("%s %s: %s «%s» is a POST without the session's token", screen, at, a.ID, a.Label)
		}
		if a.Method != "POST" && a.CSRF != "" {
			t.Errorf("%s %s: %s «%s» is a %s with a token", screen, at, a.ID, a.Label, a.Method)
		}
	})
}

// C-11, the client's scan of the calendar (Б6): a GET form puts its fields
// in the address, and the session's token in an address is a token in the
// history, the logs and the Referer. No GET form of any fixture carries
// _csrf, in any of its sets or in a template of a repeating row.
func TestFixtureGetFormsCarryNoToken(t *testing.T) {
	forms := 0
	eachFixtureValue(t, reflect.TypeOf(Form{}), func(screen, at string, v reflect.Value) {
		f := v.Interface().(Form)
		if !strings.EqualFold(f.Method, "GET") {
			return
		}
		forms++
		var rows func(rs []FormRow, at string)
		rows = func(rs []FormRow, at string) {
			for i, r := range rs {
				if r.Name == "_csrf" {
					t.Errorf("%s %s[%d]: a GET form to %s carries the session's token", screen, at, i, f.Action)
				}
				rows(r.Template, fmt.Sprintf("%s[%d].Template", at, i))
			}
		}
		for i, set := range f.Sets {
			rows(set.Rows, fmt.Sprintf("%s.Sets[%d].Rows", at, i))
		}
	})
	if forms == 0 {
		t.Fatal("no GET form in any fixture: the check looks at nothing")
	}
}

// eachFixtureValue calls fn for every value of type typ in every fixture on
// disk, each walked as its view model; at is the field path to it. A value
// of typ is not walked into.
func eachFixtureValue(t *testing.T, typ reflect.Type, fn func(screen, at string, v reflect.Value)) {
	t.Helper()
	screens, err := os.ReadDir(fixtureDir)
	if err != nil {
		t.Fatalf("read fixture dir: %v", err)
	}
	for _, screen := range screens {
		newModel, ok := fixtureTypes[screen.Name()]
		if !screen.IsDir() || !ok {
			continue
		}
		states, _ := filepath.Glob(filepath.Join(fixtureDir, screen.Name(), "*.json"))
		for _, path := range states {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			model := newModel()
			if err := json.Unmarshal(raw, model); err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			var walk func(v reflect.Value, at string)
			walk = func(v reflect.Value, at string) {
				switch v.Kind() {
				case reflect.Pointer, reflect.Interface:
					if !v.IsNil() {
						walk(v.Elem(), at)
					}
				case reflect.Slice, reflect.Array:
					for i := 0; i < v.Len(); i++ {
						walk(v.Index(i), fmt.Sprintf("%s[%d]", at, i))
					}
				case reflect.Struct:
					if v.Type() == typ {
						fn(screen.Name(), at, v)
						return
					}
					for i := 0; i < v.NumField(); i++ {
						walk(v.Field(i), at+"."+v.Type().Field(i).Name)
					}
				}
			}
			walk(reflect.ValueOf(model), screen.Name()+"/"+filepath.Base(path))
		}
	}
}

// eachFixtureAction calls fn for every Action in every fixture on disk,
// each walked as its view model; at is the field path to it.
func eachFixtureAction(t *testing.T, fn func(screen, at string, a Action)) {
	t.Helper()
	screens, err := os.ReadDir(fixtureDir)
	if err != nil {
		t.Fatalf("read fixture dir: %v", err)
	}
	actionType := reflect.TypeOf(Action{})
	for _, screen := range screens {
		newModel, ok := fixtureTypes[screen.Name()]
		if !screen.IsDir() || !ok {
			continue
		}
		states, _ := filepath.Glob(filepath.Join(fixtureDir, screen.Name(), "*.json"))
		for _, path := range states {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			model := newModel()
			if err := json.Unmarshal(raw, model); err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			var walk func(v reflect.Value, at string)
			walk = func(v reflect.Value, at string) {
				switch v.Kind() {
				case reflect.Pointer, reflect.Interface:
					if !v.IsNil() {
						walk(v.Elem(), at)
					}
				case reflect.Slice, reflect.Array:
					for i := 0; i < v.Len(); i++ {
						walk(v.Index(i), fmt.Sprintf("%s[%d]", at, i))
					}
				case reflect.Struct:
					if v.Type() == actionType {
						a := v.Interface().(Action)
						fn(screen.Name(), at, a)
						// The items of an arrow are actions too.
						walk(v.FieldByName("Arrow"), at+".Arrow")
						return
					}
					for i := 0; i < v.NumField(); i++ {
						walk(v.Field(i), at+"."+v.Type().Field(i).Name)
					}
				}
			}
			walk(reflect.ValueOf(model), filepath.Base(path))
		}
	}
}
