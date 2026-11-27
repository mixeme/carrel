package bind

import (
	"testing"

	"carrel.dev/carrel/server/view"
)

// A mailbox's trace (s-mailbox-25): the names in one column, a step of
// several lines going on under its first.
func TestMailTrace(t *testing.T) {
	got := MailTrace([]view.TraceStep{
		{Name: "srv", Detail: "_imaps._tcp.office.example → mail.office.example:993"},
		{Name: "smtp", Detail: "mail.office.example:465  TLS · certificate matches\nAUTH PLAIN → 535 5.7.8 Error: authentication failed"},
	}).Text
	want := "[srv]   _imaps._tcp.office.example → mail.office.example:993\n" +
		"[smtp]  mail.office.example:465  TLS · certificate matches\n" +
		"        AUTH PLAIN → 535 5.7.8 Error: authentication failed"
	if got != want {
		t.Errorf("trace =\n%s\nwant\n%s", got, want)
	}
}
