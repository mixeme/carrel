package web

import "testing"

// from-typecase note 18, s-notes-20: the line under the text of a note's
// edit names the first property Carrel does not know and says how many more
// there are — in words, as the frame does; the client counts nothing.
func TestForeignLine(t *testing.T) {
	for _, c := range []struct {
		names []string
		want  string
	}{
		{nil, ""},
		{[]string{"X-JTX-COLOR"}, "A property Carrel does not know — X-JTX-COLOR — is kept unchanged and not shown here."},
		{[]string{"X-JTX-COLOR", "X-OTHER"}, "Properties Carrel does not know — X-JTX-COLOR and one more — are kept unchanged and not shown here."},
		{[]string{"X-A", "X-B", "X-C"}, "Properties Carrel does not know — X-A and two more — are kept unchanged and not shown here."},
		{[]string{"X-1", "X-2", "X-3", "X-4", "X-5", "X-6", "X-7", "X-8", "X-9", "X-10", "X-11", "X-12"},
			"Properties Carrel does not know — X-1 and 11 more — are kept unchanged and not shown here."},
	} {
		if got := string(foreignLine(c.names)); got != c.want {
			t.Errorf("foreignLine(%q) = %q, want %q", c.names, got, c.want)
		}
	}
}
