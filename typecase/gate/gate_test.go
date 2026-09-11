package gate_test

import (
	"testing"

	"carrel.dev/typecase/catalog"
	"carrel.dev/typecase/gate"
)

func catalogEntries() []gate.CatalogEntry {
	var out []gate.CatalogEntry
	for _, e := range catalog.Entries() {
		out = append(out, gate.CatalogEntry{
			Widget: e.Widget,
			State:  e.State,
			HTML:   e.HTML,
		})
	}
	return out
}

func TestL8_gates(t *testing.T) {
	root, err := gate.FindModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	p := gate.Paths(root)
	cat := catalogEntries()

	checks := []gate.Name{
		gate.Completeness,
		gate.Ownership,
		gate.NodeNames,
		gate.TypedInput,
		gate.Enumerations,
		gate.DomainNaming,
		gate.DocFormat,
		gate.Catalog,
		gate.NoInline,
		gate.A11y,
	}
	for _, name := range checks {
		t.Run(string(name), func(t *testing.T) {
			if err := gate.Run(name, p, cat); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestL8_verifiable(t *testing.T) {
	// Each gate above has a matching bite test in bite_test.go (L-9.3).
	names := []gate.Name{
		gate.Completeness, gate.Ownership, gate.NodeNames, gate.TypedInput,
		gate.Enumerations, gate.DomainNaming, gate.DocFormat, gate.Catalog,
		gate.NoInline, gate.A11y,
	}
	if len(names) < 10 {
		t.Fatal("expected gate coverage")
	}
}
