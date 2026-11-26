package web_test

import (
	"encoding/json"
	"html"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var frameObjectsRow = regexp.MustCompile(`(?s)<span class="tc-table__filename">([^<]+)</span></span></td>\s*<td class="tc-mono tc-table__n3">.*?</td><td class="tc-mono tc-table__n4[^"]*">(\d+)</td>`)

// Typecase's note 178 p. 1: «Objects» is what each collection of the set
// holds, the trip of «Личный» and the copy of «Собрать чемодан» in «Работа»
// counted — Личный 5, Работа 6 (s-profile-1); the delete sheet of Личный
// says «5 events, one file, straight to you» (s-profile-9).
func TestObjectsAreWhatTheCollectionsHold(t *testing.T) {
	want := map[string]int{}
	for _, m := range frameObjectsRow.FindAllStringSubmatch(mockupFrame(t, "s-profile-1"), -1) {
		n, _ := strconv.Atoi(m[2])
		want[html.UnescapeString(m[1])] = n
	}
	// «Записи 3» since Typecase's notes 182, 193 (task 259).
	if want["Личный"] != 5 || want["Работа"] != 6 || want["Записи"] != 3 {
		t.Fatalf("s-profile-1 counts %v: read the frame again", want)
	}
	got := map[string]int{}
	var walk func(any)
	walk = func(o any) {
		switch v := o.(type) {
		case map[string]any:
			if n, ok := v["objects"].(float64); ok {
				if name, ok := v["label"].(string); ok {
					got[name] = int(n)
				}
			}
			for _, x := range v {
				walk(x)
			}
		case []any:
			for _, x := range v {
				walk(x)
			}
		}
	}
	walk(loadFixture(t, "app-settings-connections/normal.json"))
	// «Записи» holds the 16 notes of Neighbours since Typecase's note 234;
	// s-profile-1 still counts 3 — Typecase's task 295. When the frame says
	// 16, this line is red: hold Записи to the frame again.
	if want["Записи"] == 16 {
		t.Errorf("s-profile-1 counts Записи 16 now: task 295 is answered, drop the exception")
	}
	want["Записи"] = 16
	for name, n := range want {
		if g, ok := got[name]; ok && g != n {
			t.Errorf("%s: the fixture counts %d, the frame %d", name, g, n)
		}
	}

	line := regexp.MustCompile(`\d+ events, one file, straight to you`).FindString(mockupFrame(t, "s-profile-9"))
	if line == "" {
		t.Fatal("s-profile-9 says nothing of the file: read the frame again")
	}
	body, _ := json.Marshal(loadFixture(t, "app-collections-delete/normal.json"))
	if !strings.Contains(string(body), line) {
		t.Errorf("the delete sheet of Личный does not say %q", line)
	}
}
