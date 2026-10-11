package file

import "testing"

// Two listings of a tree are the same tree when every member is in both, a
// folder a folder and a file of its size — and, of one tree at two moments,
// of its version (Б14 of the audit of 11.10.2026).
func TestTreeDiffers(t *testing.T) {
	base := []TreeFile{{Rel: "2020", Dir: true}, {Rel: "2020/b.txt", Size: 4, ETag: "b1"}, {Rel: "a.txt", Size: 5, ETag: "a1"}}
	with := func(edit func([]TreeFile) []TreeFile) []TreeFile {
		return edit(append([]TreeFile(nil), base...))
	}
	for _, tc := range []struct {
		name     string
		other    []TreeFile
		versions bool
		at       string
	}{
		{"the same", base, true, ""},
		{"a file added", with(func(l []TreeFile) []TreeFile { return append(l, TreeFile{Rel: "late.txt", Size: 1}) }), true, "late.txt"},
		{"a file gone", base[:2], true, "a.txt"},
		{"a file added in the middle", with(func(l []TreeFile) []TreeFile {
			return []TreeFile{l[0], {Rel: "2020/a0.txt", Size: 1}, l[1], l[2]}
		}), true, "2020/a0.txt"},
		{"another size", with(func(l []TreeFile) []TreeFile { l[2].Size = 6; return l }), false, "a.txt"},
		{"another version, of one tree", with(func(l []TreeFile) []TreeFile { l[1].ETag = "b2"; return l }), true, "2020/b.txt"},
		{"another version, of a copy", with(func(l []TreeFile) []TreeFile { l[1].ETag = "b2"; return l }), false, ""},
		{"no version given", with(func(l []TreeFile) []TreeFile { l[1].ETag = ""; return l }), true, ""},
		{"a file where a folder was", with(func(l []TreeFile) []TreeFile { l[0].Dir = false; return l }), false, "2020"},
	} {
		if got := TreeDiffers(base, tc.other, tc.versions); got != tc.at {
			t.Errorf("%s: differs at %q, want %q", tc.name, got, tc.at)
		}
	}
}
