package web_test

import (
	"net/url"
	"testing"

	"carrel.dev/carrel/server/view"
)

// A recipient chosen from the suggestions is drawn with the name and the
// mark of the book they came from (Пб18; s-mail-1: «▍Анна Ковалёва ·
// a.kovaleva@example.org»); an address typed by hand stays its address.
// Without a script a name typed in «Name or address» (Enter: op=add) offers
// its addresses; an address typed joins the recipients (Typecase's note 110
// p. 3).
func TestChosenRecipientIsNamed(t *testing.T) {
	f, _ := letterFixture(t)
	sheet := decode[view.MailSheet](t, f.h.do(postForm("/app/mail/send?op=add", f.sess, url.Values{"to.add": {"ada"}})))
	if len(sheet.Suggest) == 0 || sheet.Suggest[0].Colour == "" || letterValueAdd(sheet) != "ada" {
		t.Fatalf("suggestions = %+v, the line %q: want the book's mark and the line kept", sheet.Suggest, letterValueAdd(sheet))
	}
	offered := sheet.Suggest[0]
	sheet = decode[view.MailSheet](t, f.h.do(postForm("/app/mail/send?op=add", f.sess, url.Values{
		"to": {string(offered.Address)}, "to.add": {"stranger@example.net"}, "subject": {"Hi"}, "text": {"x"},
	})))
	to := letterTo(sheet)
	if len(to) != 2 || letterValueAdd(sheet) != "" {
		t.Fatalf("to = %+v, the line %q", to, letterValueAdd(sheet))
	}
	if got := to[0]; got.Label != offered.Name || got.Meta != offered.Address || got.Colour != offered.Colour || got.Typed {
		t.Errorf("the chosen one = %+v, want %q %q", got, offered.Name, offered.Colour)
	}
	if got := to[1]; got.Label != "stranger@example.net" || got.Colour != "" || !got.Typed || got.Value != "stranger@example.net" {
		t.Errorf("the typed one = %+v", got)
	}
}

// letterValueAdd is what stands in «Name or address».
func letterValueAdd(sheet view.MailSheet) string {
	if r := formRow(sheet.Form, "to"); r != nil && r.Add != nil {
		return string(r.Add.Value)
	}
	return ""
}
