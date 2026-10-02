package view

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The fields a page sends to the files section without scripts, and the
// ones a script sends in front of the files (S-17.1), are C-5's: a client
// knows them from the map, not from the server's code (the client's scan
// of the files section, ФБ8, ФБ9, ФБ13).
func TestC5NamesTheFieldsOfFiles(t *testing.T) {
	src, err := os.ReadFile(specFile)
	if err != nil {
		t.Fatal(err)
	}
	spec := strings.ReplaceAll(string(src), "\r\n", "\n")
	_, section, ok := strings.Cut(spec, "\n### C-5.5 ")
	if !ok {
		t.Fatal("no C-5.5 in the spec")
	}
	section, _, _ = strings.Cut(section, "\n### ")
	// A field is named in backticks, alone or as a parameter: `name`,
	// `?view=…`, `size.<имя>`.
	for _, field := range []string{"size.<имя>", "select", "toggle", "all", "view", "sort", "filter", "to", "name"} {
		if !regexp.MustCompile(`\x60\??` + regexp.QuoteMeta(field) + `[\x60=]`).MatchString(section) {
			t.Errorf("C-5.5 does not name %s", field)
		}
	}
}
