package web

import "testing"

// What the fixtures do not show: an address under CARREL_BASE_PATH, a
// collection whose path is one escaped segment, a folder called «op», a
// method a row of C-5 does not hold, and an address outside Carrel.
func TestAnswerZoneReadsTheAddressAsTheMuxDoes(t *testing.T) {
	for _, c := range []struct{ base, method, href, want string }{
		{"", "GET", "/app/contacts/a-1/%2Falice%2Fcontacts%2Fpeople%2F/ada/move", "sheet"},
		{"/carrel", "GET", "/carrel/app/contacts/a-1/people/ada/move?copy=1", "sheet"},
		{"/carrel", "GET", "/app/contacts/a-1/people/ada/move", ""},
		{"", "POST", "/app/files/a-1/docs/op/notes/", "sheet"},
		{"", "GET", "/app/files/a-1/docs/op/notes/", ""},
		{"", "POST", "/app/files/a-1/docs/upload/", "queue"},
		{"", "POST", "/app/sources/files", "rail"},
		{"", "POST", "/app/sources/people", ""},
		{"", "GET", "/app/contacts/a-1/people/rewrite", ""},
		{"", "GET", "/app/contacts/a-1/people/ada/rewrite?plan=p", "sheet"},
		{"", "POST", "/app/contacts/a-1/people/ada/rewrite?plan=p", ""},
		{"", "GET", "https://example.org/app/publish", ""},
		{"", "GET", "/app/publish?k=contacts", "sheet"},
		{"", "GET", "/app/contacts/quick", "sheet"},
		{"", "GET", "/app/contacts/a-1/people", ""},
	} {
		if got := answerZone(c.base, c.method, c.href); got != c.want {
			t.Errorf("%s %s under %q: %q, want %q", c.method, c.href, c.base, got, c.want)
		}
	}
}
