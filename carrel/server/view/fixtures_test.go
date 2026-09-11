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
var fixtureTypes = map[string]func() any{
	"app-contacts": func() any { return &ContactsListPage{} },
	"app-conflict": func() any { return &ConflictPage{} },
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
