package card

import (
	"strings"
	"testing"
)

// Typecase's note 113, s-io-5: a vCard 2.1 value written quoted-printable —
// broken over lines with «=» — is read as the text it stands for; one that
// cannot be decoded refuses its card alone, with the line it stands on.
func TestSplitReadsQuotedPrintable(t *testing.T) {
	doc := "BEGIN:VCARD\r\nVERSION:2.1\r\nUID:vera\r\n" +
		"N;CHARSET=UTF-8;ENCODING=QUOTED-PRINTABLE:=D0=9B=D0=B5=D0=B1=D0=B5=D0=B4=D0=B5=D0=B2=D0=B0;=D0=92=\r\n" +
		"=D0=B5=D1=80=D0=B0;;;\r\nEND:VCARD\r\n" +
		"BEGIN:VCARD\r\nVERSION:2.1\r\nUID:bad\r\nFN:Иван\r\n" +
		"NOTE;ENCODING=QUOTED-PRINTABLE:=ZZ broken\r\nEND:VCARD\r\n"
	objs, broken, err := Split([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 1 || !strings.HasPrefix(first(objs[0], "N"), "Лебедева;Вера") {
		t.Fatalf("cards = %d, N = %q", len(objs), first(objs[0], "N"))
	}
	if len(broken) != 1 || broken[0].Name != "Иван" || broken[0].Reason != "line 11: cannot decode quoted-printable" {
		t.Errorf("broken = %+v", broken)
	}
}
