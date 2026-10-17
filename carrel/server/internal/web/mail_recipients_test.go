package web_test

import (
	"net/url"
	"testing"

	"carrel.dev/carrel/server/view"
)

// A recipient chosen from the suggestions is drawn with the name and the
// mark of the book they came from (Пб18; s-mail-1: «▍Анна Ковалёва ·
// a.kovaleva@example.org»); an address typed by hand stays its address.
func TestChosenRecipientIsNamed(t *testing.T) {
	f, _ := letterFixture(t)
	sheet := decode[view.MailSheet](t, f.h.do(get("/app/mail/compose?q=ada", f.sess)))
	if len(sheet.Suggest) == 0 || sheet.Suggest[0].Colour == "" {
		t.Fatalf("suggestions = %+v, want the book's mark", sheet.Suggest)
	}
	offered := sheet.Suggest[0]
	sheet = decode[view.MailSheet](t, f.h.do(postForm("/app/mail/compose", f.sess, url.Values{
		"to": {string(offered.Address) + ", stranger@example.net"}, "subject": {"Hi"}, "body": {"x"},
	})))
	if len(sheet.To) != 2 {
		t.Fatalf("to = %+v", sheet.To)
	}
	if got := sheet.To[0]; got.Name != offered.Name || got.Colour != offered.Colour {
		t.Errorf("the chosen one = %+v, want %q %q", got, offered.Name, offered.Colour)
	}
	if got := sheet.To[1]; got.Name != "" || got.Colour != "" || got.Address != "stranger@example.net" {
		t.Errorf("the typed one = %+v", got)
	}
}
