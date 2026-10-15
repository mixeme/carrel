package web_test

import (
	"net/url"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The owner's decisions of 12.10.2026 (А4, А5; Typecase's task 157,
// s-contacts-29): two addresses replaced in one edit are two choices — each
// offered the address typed in its own line, else «Do not rewrite» —, and
// each group of risk is a tick: a group not ticked keeps its records, and a
// record in two groups is rewritten only when both are ticked. Until then
// every old address went to the card's default one, and every record that
// could be written was.
func TestRewriteChoosesAddressesAndGroups(t *testing.T) {
	s := newRewriteStand(t)
	s.srv.Seed(peopleBook+"ada.vcf", strings.Replace(rewriteCard, "EMAIL;TYPE=work:ada@old.example", "EMAIL;TYPE=work:ada@old.example\r\nEMAIL;TYPE=home:ada@two.example", 1))
	s.srv.Seed(rewriteWork+"two.ics", strings.ReplaceAll(meeting("two", "alice@example.org", "20990110T100000Z"), "ADA@old.example", "ada@two.example"))
	resp := s.save(t, func(f url.Values) {
		f.Set("email.0", "ada@new.example")
		f.Set("email.1", "")
	})
	sheet := decode[view.RewriteSheet](t, s.h.do(get(resp.Header.Get("Location"), s.sess)))
	if sheet.Title != "Rewrite the addresses in their records?" || len(sheet.Pairs) != 2 {
		t.Fatalf("sheet = %+v", sheet)
	}
	// The sheet's button sends its form — the choices and the ticks
	// (Лс32; s-contacts-29): a post of its own would rewrite what was
	// offered, not what was chosen.
	if c := rewriteAction(t, sheet.Actions, "contact.rewrite.confirm"); !c.Submits {
		t.Errorf("Rewrite = %+v, want the button of the sheet's form", c)
	}
	old, two := sheet.Pairs[0], sheet.Pairs[1]
	if old.From != "ada@old.example" || old.To != "ada@new.example" || old.Choice.Name != "to.1" || old.Choice.Value != "ada@new.example" {
		t.Errorf("the replaced line = %+v", old)
	}
	if two.From != "ada@two.example" || two.To != "" || two.Choice.Name != "to.2" || len(two.Choice.Options) != 2 ||
		two.Choice.Options[1].Value != "" || two.Choice.Options[1].Label != "Do not rewrite" || !two.Choice.Options[1].Selected {
		t.Errorf("the removed line = %+v", two)
	}
	var values []string
	for _, g := range sheet.Groups {
		values = append(values, g.Value)
		if !g.On || g.Name != "group" || len(g.Records) == 0 || g.Records[0].Href == "" {
			t.Errorf("group = %+v", g)
		}
	}
	if !slices.Equal(values, []string{"scheduling", "others", "past"}) {
		t.Errorf("groups = %v", values)
	}
	if !strings.Contains(string(sheet.Both), "«old» stands in both «On servers that schedule» and «Past meetings»") {
		t.Errorf("both = %q", sheet.Both)
	}
	if !slices.Contains(sheet.Tally, "to.1 group=scheduling group=others") || !slices.Contains(sheet.Tally, "to.2 group=scheduling") {
		t.Errorf("tally = %q", sheet.Tally)
	}
	confirm := rewriteAction(t, sheet.Actions, "contact.rewrite.confirm")
	if confirm.Label != "Rewrite 3 records" || sheet.Count != "Rewrite {n} records" {
		t.Errorf("confirm = %+v, count %q", confirm, sheet.Count)
	}

	// The removed address goes to the new one too; the meetings organised
	// by others stay.
	form := formFields(sheet.Form)
	form.Set("to.1", "ada@new.example")
	form.Set("to.2", "ada@new.example")
	form["group"] = []string{"scheduling", "past"}
	s.h.do(postForm(confirm.Href, s.sess, form))
	for path, want := range map[string]bool{"mine.ics": true, "old.ics": true, "two.ics": true, "theirs.ics": false} {
		body, _, _ := s.srv.Object(rewriteWork + path)
		if got := strings.Contains(strings.ToLower(unfolded(body)), "mailto:ada@new.example"); got != want {
			t.Errorf("%s rewritten %v, want %v:\n%s", path, got, want, body)
		}
	}
}
