package web

import "testing"

// Google marks a preferred phone and address TYPE=pref, and vCard 3 writes
// INTERNET on an address and VOICE on a phone (acceptance on our test
// servers, 18.11.2026: the card said «Phone · mobile, pref»). They are no
// kind of the value (notLabels): the card says what the form says, «Phone ·
// mobile» (s-contacts-6).
func TestATypedLabelLeavesOutWhatIsNoKind(t *testing.T) {
	for _, c := range []struct{ label, types, want string }{
		{"Phone", "CELL,pref", "Phone · mobile"},
		{"Email", "INTERNET,WORK,pref", "Email · work"},
		{"Phone", "VOICE", "Phone"},
		{"Phone", "work,fax", "Phone · work, fax"},
	} {
		if got := typedLabel(c.label, map[string]string{"TYPE": c.types}); got != c.want {
			t.Errorf("%s %s = %q, want %q", c.label, c.types, got, c.want)
		}
	}
}
