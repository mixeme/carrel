package web

import "testing"

// Two addresses replaced: counted and named, «them» for the records.
func TestReplacedAddressesWords(t *testing.T) {
	n := replacedNotice([]rewritePair{{From: "anna@oldfirm.example", To: "a.kovaleva@slovo.example"}, {From: "a.k@old.example"}}, "/sheet")
	if n.Title != "2 addresses replaced: anna@oldfirm.example, a.k@old.example." || n.Body != "Events and tasks that name them are unchanged." {
		t.Errorf("notice = %q / %q", n.Title, n.Body)
	}
}
