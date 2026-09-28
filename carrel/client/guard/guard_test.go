package guard_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/client/guard"
)

// The client module, its screens and the client spec, from carrel/client/guard.
const (
	module     = ".."
	screens    = "../screens"
	clientSpec = "../../../docs/spec/carrel-client.md"
)

// own is what the guards leave out of the real module: this package, whose
// source names every forbidden thing in order to look for it, and its
// deliberately broken samples. Nothing imports this package; its samples are
// in testdata, which the go command never builds.
var own = []string{"guard"}

// ownSamples is what Files and CheckRegistries leave out: only the samples —
// this package's source is held to them like any other.
var ownSamples = []string{"guard/testdata"}

// report is findings one to a line, sorted, as «path:line: rule» — what a
// bite test compares, so that a guard red for the wrong reason is red here
// too.
func report(fs []guard.Finding) []string {
	fs = slices.Clone(fs)
	slices.SortFunc(fs, func(a, b guard.Finding) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		return a.Line - b.Line
	})
	var out []string
	for _, f := range fs {
		out = append(out, fmt.Sprintf("%s:%d: %s", f.Path, f.Line, f.Rule))
	}
	return out
}

// must turns a guard's answer into its findings, failing on an error.
func must(t *testing.T) func([]guard.Finding, error) []guard.Finding {
	return func(fs []guard.Finding, err error) []guard.Finding {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return fs
	}
}

func green(t *testing.T, what string, fs []guard.Finding) {
	t.Helper()
	for _, f := range fs {
		t.Errorf("%s: %s", what, f)
	}
}

func bite(t *testing.T, fs []guard.Finding, want []string) {
	t.Helper()
	if len(fs) == 0 {
		t.Fatal("the guard stayed green on a broken sample")
	}
	if got := report(fs); !slices.Equal(got, want) {
		var all []string
		for _, f := range fs {
			all = append(all, f.String())
		}
		t.Errorf("findings\n%s\nwant\n%s\nall:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"), strings.Join(all, "\n"))
	}
}

// --- The real module: every guard green. ---

func TestScreensImportOnlyWhatIsAllowed(t *testing.T) {
	green(t, "imports", must(t)(guard.Imports(screens)))
}

func TestNoMarkupInStrings(t *testing.T) {
	green(t, "strings", must(t)(guard.Strings(module, own...)))
}

func TestNoAssetsOfTheClientsOwn(t *testing.T) {
	green(t, "files", must(t)(guard.Files(module, ownSamples...)))
}

func TestRegistriesAndMarksAnswerEachOther(t *testing.T) {
	green(t, "registries", must(t)(guard.CheckRegistries(clientSpec, module, ownSamples...)))
}

// --- The bites: each guard red on its broken sample, green on its good one. ---

func TestImportsBite(t *testing.T) {
	bite(t, must(t)(guard.Imports("testdata/imports/bad/screens")), []string{
		"contacts/contacts.go:6: import",  // embed
		"contacts/contacts.go:7: import",  // fmt
		"contacts/contacts.go:8: import",  // html/template
		"contacts/contacts.go:9: import",  // strings
		"contacts/contacts.go:11: import", // another package of the client
	})
	green(t, "good imports", must(t)(guard.Imports("testdata/imports/good/screens")))
}

func TestStringsBite(t *testing.T) {
	bite(t, must(t)(guard.Strings("testdata/strings/bad")), []string{
		"screen.go:5: string",  // <li: markup
		"screen.go:5: string",  // class=
		"screen.go:5: string",  // tc-row
		"screen.go:8: string",  // "</span>"
		"screen.go:9: string",  // "<" + "div"
		"screen.go:10: string", // '<'
		"screen.go:11: string", // STYLE =
		"screen.go:12: string", // data-tc-open: data-tc
		"screen.go:12: string", // data-tc-open: tc-
		"screen.go:13: string", // "<%s>"
		"screen.go:14: string", // <!--
		"screen.go:15: string", // tc-sheet
		"screen.go:16: string", // "\x3cp\x3e"
		"screen.go:17: string", // "class="
	})
	green(t, "good strings", must(t)(guard.Strings("testdata/strings/good")))
}

func TestFilesBite(t *testing.T) {
	bite(t, must(t)(guard.Files("testdata/files/bad")), []string{
		"screens/icon.SVG:0: file",
		"screens/list.css:0: file",
		"screens/list.html:0: file",
		"screens/list.js:0: file",
		"screens/list.tmpl:0: file",
	})
	green(t, "good files", must(t)(guard.Files("testdata/files/good")))
}

func TestRegistriesBite(t *testing.T) {
	bite(t, must(t)(guard.CheckRegistries("testdata/registries/bad/spec.md", "testdata/registries/bad")), []string{
		"screens/screens.go:12: registry", // U-5.7 #9: a mark without a row
		"screens/screens.go:18: registry", // U-5.5 #2: a mark without a row
		"spec.md:8: registry",             // U-5.5 #1 twice
		"spec.md:21: registry",            // U-5.7 #2: no owner's decision
		"spec.md:22: registry",            // U-5.7 #3: «—» is no decision
		"spec.md:23: registry",            // U-5.7 #4: a row without a mark
		"spec.md:24: registry",            // U-5.7 x: not a row number
	})
	bite(t, must(t)(guard.CheckRegistries("testdata/registries/shapeless/spec.md", "testdata/registries/shapeless")), []string{
		"spec.md:0: registry", // U-5.5: no table
		"spec.md:0: registry", // U-5.7: a table of other columns
	})
	green(t, "good registries", must(t)(guard.CheckRegistries("testdata/registries/good/spec.md", "testdata/registries/good")))
	green(t, "empty registries", must(t)(guard.CheckRegistries("testdata/registries/empty/spec.md", "testdata/registries/empty")))
}
