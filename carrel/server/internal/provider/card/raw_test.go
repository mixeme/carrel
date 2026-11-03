package card

import "testing"

// RawCards hands a card over as its lines stand: a fold stays a fold, every
// line ends in CRLF, and a card without its END:VCARD is not one.
func TestRawCardsKeepTheLines(t *testing.T) {
	doc := "BEGIN:VCARD\nVERSION:4.0\nUID:a\nNOTE:long\n  folded\nX-ODD;;:kept\nEND:VCARD\r\n" +
		"BEGIN:VCARD\r\nUID:b\r\n"
	got := RawCards([]byte(doc))
	want := "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:a\r\nNOTE:long\r\n  folded\r\nX-ODD;;:kept\r\nEND:VCARD\r\n"
	if len(got) != 1 || string(got[0]) != want {
		t.Fatalf("cards = %q, want [%q]", got, want)
	}
}
