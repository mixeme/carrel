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
		gate.Gofmt,
		gate.Vet,
		gate.Completeness,
		gate.Ownership,
		gate.NodeNames,
		gate.TypedInput,
		gate.Enumerations,
		gate.DomainNaming,
		gate.DocFormat,
		gate.Catalog,
		gate.SpecL4,
		gate.Verifiable,
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
	root, err := gate.FindModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	p := gate.Paths(root)
	if err := gate.Run(gate.Verifiable, p, catalogEntries()); err != nil {
		t.Fatal(err)
	}
	if len(gate.AllGates) != 14 {
		t.Fatalf("expected 14 gates, got %d", len(gate.AllGates))
	}
}
