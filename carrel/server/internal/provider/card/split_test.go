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
	objs, err := Split([]byte(twoCards))
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
	if _, err := Split([]byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n")); err == nil {
		t.Fatal("a calendar was taken for a vCard")
	}
}

// An import that meets the UID in the collection it writes to gives the card
// a new one and keeps everything else.
func TestSetUIDKeepsTheRest(t *testing.T) {
	objs, err := Split([]byte(twoCards))
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
