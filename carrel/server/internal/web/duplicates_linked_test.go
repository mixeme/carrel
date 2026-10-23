package web_test

import (
	"net/url"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/dupes"
	"carrel.dev/carrel/server/view"
)

// From-typecase note 117 pp. 1, 4, 5: the card asks with radios what is held
// once and with ticks what to keep of what is held many times; what was
// unticked is kept with the group, and the card of the linked record — its
// own address — shows what was chosen, each choice marked with where it came
// from, and where it is kept with what each copy holds otherwise.
func TestLinkedRecordShowsTheChoice(t *testing.T) {
	f := newDupFixture(t)
	f.load(t)
	g := onlyGroup(t, f.page(t))
	rows := map[string]view.MergeRow{}
	for _, c := range g.Choices {
		rows[string(c.Property)] = c
	}
	email, phone := rows["Email"], rows["Phone"]
	// The address both hold, whatever its case, is not asked about.
	if !email.Many || len(email.Options) != 1 || email.Options[0] != "ada@engines.example · Work" || len(email.Kept) != 1 {
		t.Fatalf("Email = %+v, want the one address only Work holds, ticked", email)
	}
	if !phone.Many || len(phone.Options) != 1 {
		t.Fatalf("Phone = %+v", phone)
	}
	form := url.Values{
		"choice.FN":  {choiceValue(t, rows["Display name"], "LOVELACE Ada")},
		"choice.ORG": {choiceValue(t, rows["Organisation"], "Analytical Engines")},
		"choice.TEL": {phone.Values[0]},
		// Email unticked: not sent.
	}
	f.h.do(decideRequest(t, f.sess, g, "duplicates.link", form))
	d := f.decisions(t)
	if len(d.Linked) != 1 || !slices.Equal(d.Linked[0].Dropped["EMAIL"], []string{dupes.ValueKey("EMAIL", "ada@engines.example")}) {
		t.Fatalf("linked = %+v, want the address unticked kept as dropped", d.Linked)
	}

	card := decode[view.ContactPage](t, f.h.do(get("/app/contacts/linked/ada-here", f.sess)))
	if card.Name != "LOVELACE Ada" || card.Subtitle != "All address books · in 2 books" {
		t.Errorf("head = %q · %q", card.Name, card.Subtitle)
	}
	var phoneFrom string
	for _, fl := range card.Fields {
		if strings.Contains(string(fl.Value), "engines") {
			t.Errorf("the address unticked is shown: %+v", fl)
		}
		if fl.Label == "Phone" && fl.From != nil {
			phoneFrom = string(fl.From.Name)
		}
		if fl.Label == "Email · home" && fl.From != nil {
			t.Errorf("the address both hold was no choice, marked %+v", fl.From)
		}
	}
	if phoneFrom != "People" {
		t.Errorf("the phone kept is marked %q, want its collection", phoneFrom)
	}
	got := map[string]string{}
	for _, b := range card.Books {
		_, col, _ := strings.Cut(string(b.Label), " · ")
		got[col] = string(b.Value)
	}
	if got["People"] != "Name, organisation differ" || got["Work"] != "Phone, email differ" {
		t.Errorf("where it is kept = %v", got)
	}
	if len(card.Actions) != 1 || !strings.HasSuffix(card.Actions[0].Href, "/app/contacts/linked/ada-here/copies") {
		t.Errorf("actions = %+v, want Edit to the copies", card.Actions)
	}
	var menu []string
	for _, m := range card.Menu {
		menu = append(menu, m.ID)
	}
	if !slices.Equal(menu, []string{"record.print", "duplicates.open", "duplicates.unlink"}) {
		t.Errorf("menu = %v", menu)
	}
	if rec := f.h.do(get("/app/contacts/linked/bob", f.sess)); rec.Code != 404 {
		t.Errorf("a record in no group: %d", rec.Code)
	}
	if n := f.writes(); n != 0 {
		t.Errorf("the linked record wrote %d times", n)
	}
}

// From-typecase note 117 p. 10 (s-dup-6): a decision none of whose records
// this session has read stands as «Decided earlier» — what was decided,
// under the name it was made, where its records lie and their UIDs — and
// is taken back from there without loading anything.
func TestDecidedEarlierWithNothingLoaded(t *testing.T) {
	f := newDupFixture(t)
	f.load(t)
	f.decide(t, onlyGroup(t, f.page(t)), "duplicates.link")

	f.sess = f.h.login("owner", "password123") // a new session: nothing read
	before := f.requests()
	p := f.page(t)
	if len(p.Groups) != 0 || len(p.Decided) != 1 {
		t.Fatalf("groups %+v, decided %+v", p.Groups, p.Decided)
	}
	d := p.Decided[0]
	if d.Title != "Ada Lovelace · LOVELACE Ada" || d.Badge.Label != "linked ×2" || d.Note != "Not loaded in this session" ||
		!strings.Contains(string(d.Where), "People") || !strings.Contains(string(d.Where), "Work") || d.UIDs != "UID ada-here, ada-there" {
		t.Errorf("decided = %+v", d)
	}
	if len(d.Actions) != 1 || d.Actions[0].ID != "duplicates.unlink" || d.Form == nil {
		t.Fatalf("decided actions %+v, form %+v", d.Actions, d.Form)
	}
	form := formFields(*d.Form)
	form.Set(d.Actions[0].Name, d.Actions[0].Value)
	if rec := f.h.do(postForm(d.Form.Action, f.sess, form)); rec.Code != 303 {
		t.Fatalf("unlink: %d %s", rec.Code, rec.Body.String())
	}
	if l := f.decisions(t).Linked; len(l) != 0 {
		t.Errorf("still linked: %+v", l)
	}
	if n := f.requests() - before; n != 0 {
		t.Errorf("the decision asked the servers %d times", n)
	}
}
