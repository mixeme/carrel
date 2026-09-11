package server_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestNoTypecaseImport enforces the plan's rule that carrel/server never
// imports carrel.dev/typecase: templates, CSS, and widget names have no
// place on this side of the seam.
func TestNoTypecaseImport(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "./...").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
	for _, dep := range strings.Fields(string(out)) {
		if strings.Contains(dep, "carrel.dev/typecase") {
			t.Fatalf("carrel/server module imports %s", dep)
		}
	}
}
