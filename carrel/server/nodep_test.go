package server_test

import (
	"os/exec"
	"strings"
	"testing"
)

// wiring is the one package of the server that imports the client, and
// through it Typecase: the binary, which hands the server the client's
// renderer and mounts the library's files (docs/architecture.md, «Модули»).
const wiring = "carrel.dev/carrel/server/cmd/carrel"

// TestNoTypecaseImport enforces the plan's rule that carrel/server never
// imports carrel.dev/typecase or the client, carrel.dev/carrel/client:
// templates, CSS, and widget names have no place on this side of the seam.
// The binary alone wires them in; no other package of the module depends
// on either, directly or through another.
func TestNoTypecaseImport(t *testing.T) {
	out, err := exec.Command("go", "list", "-f", `{{.ImportPath}} {{join .Deps " "}}`, "./...").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
	wired := false
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pkg := fields[0]
		for _, dep := range fields[1:] {
			if !strings.HasPrefix(dep, "carrel.dev/typecase") && !strings.HasPrefix(dep, "carrel.dev/carrel/client") {
				continue
			}
			if pkg == wiring {
				wired = true
				continue
			}
			t.Errorf("%s imports %s", pkg, dep)
		}
	}
	if !wired {
		t.Errorf("%s does not wire the client in", wiring)
	}
}
