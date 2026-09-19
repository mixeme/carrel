package card

import (
	"bytes"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/model"
)

// appleCard is a card the way Apple Contacts writes one: groups tying a label
// to a value, several TYPE values, a list of categories, a quoted parameter
// with a comma in it.
const appleCard = "BEGIN:VCARD\r\n" +
	"VERSION:3.0\r\n" +
	"UID:u1\r\n" +
	"FN:Ada Lovelace\r\n" +
	"N:Lovelace;Ada;;;\r\n" +
	"item1.EMAIL;type=INTERNET;type=HOME;type=pref:ada@example.org\r\n" +
	"item1.X-ABLabel:_$!<Home>!$_\r\n" +
	"item2.EMAIL;type=INTERNET:work@example.org\r\n" +
	"item2.X-ABLabel:Office\r\n" +
	"TEL;TYPE=CELL,VOICE:+1 555 0100\r\n" +
	"CATEGORIES:work,friends\r\n" +
	"ADR;TYPE=HOME;LABEL=\"Street 1, Town\":;;Street 1;Town;;;\r\n" +
	"NOTE:old\r\n" +
	"END:VCARD\r\n"

func encodeAfter(t *testing.T, doc string, ops ...model.PatchOp) string {
	t.Helper()
	obj, err := Parse("/c/a.vcf", "e", strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	patch, err := model.NewPatch(ops...)
	if err != nil {
		t.Fatal(err)
	}
	if err := patch.Apply(obj); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Encode(&buf, obj); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// An edit of one property leaves every other line as the server sent it
// (S-7, C-15): go-vcard alone dropped the groups, kept one TYPE of three,
// escaped the commas of CATEGORIES into one category and cut the quoted LABEL
// at its comma — on every save, and the loss check could not see it, because
// reading the card back collapsed it the same way.
func TestUntouchedLinesGoBackAsTheyCame(t *testing.T) {
	out := encodeAfter(t, appleCard, model.SetText{Prop: "NOTE", Value: "new"})
	for _, line := range strings.Split(appleCard, "\r\n") {
		if line == "" || strings.HasPrefix(line, "NOTE") {
			continue
		}
		if !strings.Contains(out, line+"\r\n") {
			t.Errorf("line %q changed:\n%s", line, out)
		}
	}
	if !strings.Contains(out, "NOTE:new\r\n") {
		t.Errorf("the edit is missing:\n%s", out)
	}
}

// Rewriting a list after one row changed rewrites that row only: the others
// keep their group, parameters and line; the changed one keeps its group and
// every TYPE value.
func TestListEditKeepsTheOtherRows(t *testing.T) {
	out := encodeAfter(t, appleCard,
		model.SetValues{Prop: "EMAIL", Values: []string{"ada@example.org", "office@example.org"}},
		model.SetParam{Prop: "EMAIL", Index: 1, Param: "TYPE", Value: "INTERNET"},
	)
	if !strings.Contains(out, "item1.EMAIL;type=INTERNET;type=HOME;type=pref:ada@example.org\r\n") {
		t.Errorf("the kept row changed:\n%s", out)
	}
	if !strings.Contains(out, "EMAIL;TYPE=INTERNET:office@example.org\r\n") {
		t.Errorf("the changed row:\n%s", out)
	}
	out = encodeAfter(t, appleCard, model.SetText{Prop: "TEL", Value: "+1 555 0199"},
		model.SetParam{Prop: "TEL", Index: 0, Param: "TYPE", Value: "CELL,VOICE"})
	if !strings.Contains(out, "TEL;TYPE=CELL,VOICE:+1 555 0199\r\n") {
		t.Errorf("an edited phone lost a TYPE:\n%s", out)
	}
}

// Categories written fresh stay a list.
func TestFreshCategoriesStayAList(t *testing.T) {
	out := encodeAfter(t, appleCard, model.SetText{Prop: "CATEGORIES", Value: "work,family"})
	if !strings.Contains(out, "CATEGORIES:work,family\r\n") {
		t.Errorf("categories:\n%s", out)
	}
}

// A server that writes the TYPE values back in another order or case has
// lost nothing; one that drops a value has.
func TestLossCheckReadsTypeAsASet(t *testing.T) {
	sent, err := Parse("/c/a.vcf", "e", strings.NewReader(appleCard))
	if err != nil {
		t.Fatal(err)
	}
	same := strings.Replace(appleCard, "type=INTERNET;type=HOME;type=pref", "TYPE=pref,home,internet", 1)
	got, err := Parse("/c/a.vcf", "e", strings.NewReader(same))
	if err != nil {
		t.Fatal(err)
	}
	if loss := model.CompareAfterPut("s", sent, got, nil); loss != nil {
		t.Errorf("reordered TYPE reported as lost: %v", loss.Properties)
	}
	dropped := strings.Replace(appleCard, "type=INTERNET;type=HOME;type=pref", "type=INTERNET", 1)
	got, err = Parse("/c/a.vcf", "e", strings.NewReader(dropped))
	if err != nil {
		t.Fatal(err)
	}
	if loss := model.CompareAfterPut("s", sent, got, nil); loss == nil {
		t.Error("a dropped TYPE value went unnoticed")
	}
}
