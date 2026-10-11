package card

import (
	"bytes"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/model"
)

// first is one value of a property, for the checks below.
func first(obj *model.Object, prop string) string {
	if v := obj.PropText(prop); len(v) > 0 {
		return v[0]
	}
	return ""
}

const twoCards = "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:ada\r\nFN:Ада Лавлейс\r\nX-CUSTOM:keep\r\nEND:VCARD\r\n" +
	"BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Без идентификатора\r\nEND:VCARD\r\n"

// S-19: a .vcf holds one card or a thousand, and each becomes a record. A
// card without a UID is given one: a collection finds its records by UID.
func TestSplitReadsEveryCard(t *testing.T) {
	objs, _, err := Split([]byte(twoCards))
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 2 {
		t.Fatalf("cards = %d, want 2", len(objs))
	}
	if UID(objs[0]) != "ada" || first(objs[0], "X-CUSTOM") != "keep" {
		t.Errorf("first card = %+v", objs[0].Props())
	}
	if len(UID(objs[1])) != 32 {
		t.Errorf("no UID was given: %q", UID(objs[1]))
	}
	var buf bytes.Buffer
	if err := Encode(&buf, objs[1]); err != nil {
		t.Fatalf("the card does not encode: %v", err)
	}
	if !strings.Contains(buf.String(), "Без идентификатора") {
		t.Errorf("encoded = %q", buf.String())
	}
}

func TestSplitRefusesWhatIsNotAVCard(t *testing.T) {
	if _, _, err := Split([]byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n")); err == nil {
		t.Fatal("a calendar was taken for a vCard")
	}
}

// An import that meets the UID in the collection it writes to gives the card
// a new one and keeps everything else.
func TestSetUIDKeepsTheRest(t *testing.T) {
	objs, _, err := Split([]byte(twoCards))
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := SetUID(objs[0], "other")
	if err != nil {
		t.Fatal(err)
	}
	if UID(fresh) != "other" || first(fresh, "X-CUSTOM") != "keep" || first(fresh, "FN") != "Ада Лавлейс" {
		t.Fatalf("card = %+v", fresh.Props())
	}
	if UID(objs[0]) != "ada" {
		t.Errorf("the card it came from changed: %q", UID(objs[0]))
	}
}

// Б2 of the audit of 11.10.2026: a card that cannot be read does not take
// the cards after it with it. Split stopped at the first broken card — one
// cut short, without its END:VCARD — and answered with what came before, as
// though the file ended there.
func TestSplitReadsPastABrokenCard(t *testing.T) {
	doc := "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:a\r\nFN:Ada\r\nEND:VCARD\r\n" +
		"BEGIN:VCARD\r\nVERSION:3.0\r\nUID:b\r\nFN:Broken\r\nTEL:+1\r\n" +
		"BEGIN:VCARD\r\nVERSION:3.0\r\nUID:c\r\nFN:Charles\r\nX-CUSTOM:keep\r\nEND:VCARD\r\n"
	objs, broken, err := Split([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 2 || UID(objs[0]) != "a" || UID(objs[1]) != "c" || first(objs[1], "X-CUSTOM") != "keep" {
		t.Fatalf("cards read = %d", len(objs))
	}
	if len(broken) != 1 || broken[0].Name != "Broken" || broken[0].Reason == "" {
		t.Fatalf("broken = %+v", broken)
	}
	// The card after the broken one keeps the lines it came in.
	if objs[1].Props()["X-CUSTOM"][0].Wire != "X-CUSTOM:keep" {
		t.Errorf("the third card lost its lines: %+v", objs[1].Props()["X-CUSTOM"])
	}
	// The last card cut short is broken too, and the ones before it stand.
	objs, broken, err = Split([]byte(twoCards + "BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Cut\r\n"))
	if err != nil || len(objs) != 2 || len(broken) != 1 || broken[0].Name != "Cut" {
		t.Errorf("a file cut short: %d cards, broken %+v, %v", len(objs), broken, err)
	}
	// A file of one broken card holds no contact, and says why.
	if _, _, err := Split([]byte("BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Cut\r\n")); err == nil {
		t.Error("a file of one broken card was read as a file")
	}
}
