package web

import "testing"

// A reference by UID is read the same way wherever it is read: urn:uuid: in
// any case is the form and is kept as written, the UID after it is as
// written, and anything else is a bare UID — the prefix alone too.
func TestSplitUIDRef(t *testing.T) {
	for _, c := range []struct{ in, prefix, uid string }{
		{"ada", "", "ada"},
		{"urn:uuid:ada", "urn:uuid:", "ada"},
		{"URN:UUID:7C1E2A40-0000-4000-8000-000000000001", "URN:UUID:", "7C1E2A40-0000-4000-8000-000000000001"},
		{"Urn:Uuid:x", "Urn:Uuid:", "x"},
		{"urn:uuid:", "", "urn:uuid:"},
		{"urn:uid:ada", "", "urn:uid:ada"},
		{" urn:uuid:ada", "", " urn:uuid:ada"},
		{"", "", ""},
	} {
		prefix, uid := splitUIDRef(c.in)
		if prefix != c.prefix || uid != c.uid {
			t.Errorf("splitUIDRef(%q) = %q, %q; want %q, %q", c.in, prefix, uid, c.prefix, c.uid)
		}
		if prefix+uid != c.in {
			t.Errorf("splitUIDRef(%q): the parts do not make the value again", c.in)
		}
	}
}
