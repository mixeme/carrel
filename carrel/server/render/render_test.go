package render

import (
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// section is the text of one section of the common spec: from its heading
// to the next one of the same depth or higher.
func section(t *testing.T, spec, heading string) string {
	t.Helper()
	i := strings.Index(spec, "\n"+heading)
	if i < 0 {
		t.Fatalf("no %q in the spec", heading)
	}
	rest := spec[i+1+len(heading):]
	depth := strings.Index(heading, " ")
	if j := regexp.MustCompile(`\n#{1,` + strconv.Itoa(depth) + `} `).FindStringIndex(rest); j != nil {
		rest = rest[:j[0]]
	}
	return rest
}

// firstColumn is the first cell of every row of the tables of a section, in
// backquotes.
func firstColumn(text string) []string {
	var out []string
	for _, m := range regexp.MustCompile("(?m)^\\| `([^`]+)` \\|").FindAllStringSubmatch(text, -1) {
		out = append(out, m[1])
	}
	return out
}

// The names this package gives the client to hand the library as data
// (stage 4 of the client's plan) are the spec's, read from it, not from the
// constants: the headers and the value of C-7.1, the zones of C-7.2 in the
// order of their table, the events of C-7.4 — exactly those —, the field and
// the header of the token in C-11, and ?print=1 in C-5.
func TestTheNamesAreTheSpecs(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/spec/carrel-common.md")
	if err != nil {
		t.Fatal(err)
	}
	spec := strings.ReplaceAll(string(raw), "\r\n", "\n")
	c71 := section(t, spec, "### C-7.1 ")
	for _, want := range []string{"`" + HeaderView + ": " + ViewFragment + "`", "`" + HeaderTarget + ": <цель>`"} {
		if !strings.Contains(c71, want) {
			t.Errorf("C-7.1 does not say %s", want)
		}
	}
	zones := firstColumn(section(t, spec, "### C-7.2 "))
	if len(zones) < len(Targets()) || !slices.Equal(zones[:len(Targets())], Targets()) {
		t.Errorf("C-7.2 names the zones %v, the package %v", zones, Targets())
	}
	if events := firstColumn(section(t, spec, "### C-7.4 ")); !slices.Equal(events, Events()) {
		t.Errorf("C-7.4 names the events %v, the package %v", events, Events())
	}
	c11 := section(t, spec, "## C-11. ")
	for _, want := range []string{"`" + CSRFField + "`", "`" + CSRFHeader + "`"} {
		if !strings.Contains(c11, want) {
			t.Errorf("C-11 does not name %s", want)
		}
	}
	if c5 := section(t, spec, "## C-5. "); !strings.Contains(c5, "`?"+PrintParam+"="+PrintValue+"`") {
		t.Errorf("C-5 does not name ?%s=%s", PrintParam, PrintValue)
	}
}
