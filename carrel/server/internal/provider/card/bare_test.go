package card

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/model"
)

// oldPhoneCard is a vCard 2.1 as old phones export it: a parameter without
// a name is the type of the value (TEL;HOME;VOICE:…, EMAIL;INTERNET:…).
const oldPhoneCard = "BEGIN:VCARD\r\n" +
	"VERSION:2.1\r\n" +
	"UID:p1\r\n" +
	"N:Лебедева;Вера;;;\r\n" +
	"FN:Вера Лебедева\r\n" +
	"TEL;HOME;VOICE:+7 900 000-00-00\r\n" +
	"TEL;CELL:+7 912 445-10-88\r\n" +
	"EMAIL;INTERNET:vera@example.net\r\n" +
	"NOTE:keep\r\n" +
	"END:VCARD\r\n"

// go-vcard takes the value of a line whose last parameter has no name for
// that parameter's value and leaves the line empty: the address and the
// phones of such a card read as nothing (found 02.11.2026). Carrel reads a
// bare parameter as a TYPE, as model.ParseLine does, wherever a card is read
// — one card, a file of them, the answer of a query.
func TestABareParameterIsAType(t *testing.T) {
	check := func(where string, obj *model.Object) {
		t.Helper()
		props := obj.Props()
		if email := props["EMAIL"]; len(email) != 1 || email[0].Text != "vera@example.net" || !strings.EqualFold(email[0].Params["TYPE"], "INTERNET") {
			t.Errorf("%s: EMAIL = %+v", where, email)
		}
		tel := props["TEL"]
		if len(tel) != 2 || tel[0].Text != "+7 900 000-00-00" || tel[1].Text != "+7 912 445-10-88" {
			t.Errorf("%s: TEL = %+v", where, tel)
		}
	}
	obj, err := Parse("/c/p1.vcf", "e", strings.NewReader(oldPhoneCard))
	if err != nil {
		t.Fatal(err)
	}
	check("one card", obj)
	objs, broken, err := Split([]byte(oldPhoneCard))
	if err != nil || len(broken) != 0 || len(objs) != 1 {
		t.Fatalf("Split = %d cards, broken %+v, %v", len(objs), broken, err)
	}
	check("a file of cards", objs[0])
	c, err := readCard(oldPhoneCard)
	if err != nil {
		t.Fatal(err)
	}
	check("a query's answer", model.NewObject("/c/p1.vcf", "e", cardToProps(c, cardBlocks([]byte(oldPhoneCard))[0])))

	// A save of another field writes the lines it did not touch as they
	// came.
	out := encodeAfter(t, oldPhoneCard, model.SetText{Prop: "NOTE", Value: "new"})
	for _, line := range []string{"TEL;HOME;VOICE:+7 900 000-00-00", "TEL;CELL:+7 912 445-10-88", "EMAIL;INTERNET:vera@example.net"} {
		if !strings.Contains(out, line+"\r\n") {
			t.Errorf("after a save %q is gone:\n%s", line, out)
		}
	}
}
