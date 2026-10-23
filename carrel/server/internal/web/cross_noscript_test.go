package web_test

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// U-8: the conflict, the duplicates, the import and the collection sheets
// without scripts — the defects of the client's scan of these sections
// (client.md, «Раздел расхождения версий», «Раздел дубликатов», «Раздел
// импорта», «Раздел коллекций»: РБ2, ДбБ2–ДбБ4, ИБ4–ИБ6, КА1, КА2, КГ2).
// Each is reached through what the view model carries, as a browser without
// a script sends it: the hidden rows of a form, the value of each radio the
// page shows chosen, the address of a link.

// browserForm is what a browser sends of a form: its rows (formFields), and
// for each radio the page draws outside the form's rows, the one checked.
func browserForm(f view.Form, radios map[string]string) url.Values {
	out := formFields(f)
	for name, v := range radios {
		out.Set(name, v)
	}
	return out
}

// РБ2: «Write this combination» without scripts. The radios of the rows had
// no names and stood in no form, so the button wrote the person's edit over
// every row, whatever was chosen — the server version the person meant to
// keep was overwritten.
func TestConflictCombinationWithoutScripts(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEND:VCARD\n")
	rec := f.path("contacts") + "/ada"
	form := formFields(decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
	f.srv.Seed(f.col+"ada.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Byron\nEND:VCARD\n")
	form.Set("fn", "Ada King")
	form.Set("title", "Countess")
	if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != http.StatusSeeOther {
		t.Fatalf("POST over a moved version: %d", resp.Code)
	}
	page := decode[view.ConflictPage](t, f.h.do(get(rec+"/conflict", f.sess)))
	if page.Form == nil {
		t.Fatalf("the conflict has no form: its choices go nowhere without scripts")
	}
	if page.Form.Method != "POST" || page.Form.Action != rec+"/conflict" || formRow(*page.Form, "etag") == nil ||
		formRow(*page.Form, "op") == nil || formRow(*page.Form, "op").Value != "combine" || formRow(*page.Form, "_csrf") == nil {
		t.Fatalf("form = %+v", page.Form)
	}
	if a := actionByID(page.Form.Actions, "conflict.combine"); a == nil || !a.Primary || !a.Enabled || a.Href != "" {
		t.Errorf("Write this combination is not the form's button: %+v", page.Form.Actions)
	}
	if actionByID(page.Actions, "conflict.combine") != nil || actionByID(page.Actions, "conflict.mine") == nil ||
		actionByID(page.Actions, "conflict.theirs") == nil || actionByID(page.Actions, "conflict.cancel") == nil {
		t.Errorf("actions = %+v", page.Actions)
	}
	chosen := map[string]string{}
	for i, row := range page.Rows {
		if row.Name != "choice."+strconv.Itoa(i) || row.Choice == view.ChoiceUnset {
			t.Fatalf("row %d has no field or nothing chosen: %+v", i, row)
		}
		chosen[row.Name] = string(row.Choice)
		if row.Property == "FN" {
			chosen[row.Name] = string(view.ChoiceTheirs)
		}
	}
	resp := f.h.do(postForm(page.Form.Action, f.sess, browserForm(*page.Form, chosen)))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != rec {
		t.Fatalf("combine: %d %q %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	got := body(t, f, "ada.vcf")
	mustContain(t, got, "FN:Ada Byron", "TITLE:Countess")
	if strings.Contains(got, "Ada King") {
		t.Errorf("the edit was written over the server's name it was asked to keep:\n%s", got)
	}
}

// ДбБ2, ДбБ3: the threshold and the weights are forms of their own: the
// field of the threshold and a field per signal, by the names the server
// reads, and Back to defaults the same form sent to its own address.
func TestThresholdAndWeightsWithoutScripts(t *testing.T) {
	f := newDupFixture(t)
	f.load(t)
	p := f.page(t)
	tf := p.Threshold.Form
	value := formRow(tf, "value")
	if tf.Method != "POST" || tf.Action != "/app/duplicates/decide" || formRow(tf, "_csrf") == nil ||
		formRow(tf, "do") == nil || formRow(tf, "do").Value != "threshold" ||
		value == nil || value.Kind != view.InputNumber || value.Value != "60" || !value.Mono || value.Label != "Match threshold" {
		t.Fatalf("threshold form = %+v", tf)
	}
	fields := formFields(tf)
	fields.Set("value", "81")
	if rec := f.h.do(postForm(tf.Action, f.sess, fields)); rec.Code != http.StatusSeeOther || f.decisions(t).Threshold != 81 {
		t.Fatalf("threshold: %d, kept %d", rec.Code, f.decisions(t).Threshold)
	}

	wf := p.Threshold.WeightForm
	if wf.Method != "POST" || wf.Action != "/app/duplicates/decide?do=weights" || formRow(wf, "_csrf") == nil {
		t.Fatalf("weights form = %+v", wf)
	}
	sent := map[string]string{}
	for _, w := range p.Threshold.Weights {
		if w.Name != "weight."+w.Signal {
			t.Fatalf("weight without its field: %+v", w)
		}
		sent[w.Name] = strconv.Itoa(w.Value)
	}
	sent["weight.email"] = "0"
	save := actionByID(wf.Actions, "duplicates.weights.save")
	reset := actionByID(wf.Actions, "duplicates.weights.defaults")
	if save == nil || save.Href != "" || !save.Primary || reset == nil || reset.Enabled {
		t.Fatalf("weights buttons = %+v", wf.Actions)
	}
	if rec := f.h.do(postForm(wf.Action, f.sess, browserForm(wf, sent))); rec.Code != http.StatusSeeOther {
		t.Fatalf("save: %d", rec.Code)
	}
	if w, ok := f.decisions(t).Weights["email"]; !ok || w != 0 {
		t.Fatalf("email weight not kept: %+v", f.decisions(t).Weights)
	}
	reset = actionByID(f.page(t).Threshold.WeightForm.Actions, "duplicates.weights.defaults")
	if reset == nil || !reset.Enabled || !reset.Submits || reset.Method != "POST" {
		t.Fatalf("Back to defaults = %+v", reset)
	}
	// The form goes to Back to defaults' address with every field as it
	// stands: the address, not the fields, says what to do.
	if rec := f.h.do(postForm(reset.Href, f.sess, browserForm(wf, sent))); rec.Code != http.StatusSeeOther {
		t.Fatalf("defaults: %d", rec.Code)
	}
	if w := f.decisions(t).Weights; len(w) != 0 {
		t.Errorf("Back to defaults kept %+v", w)
	}
}

// ДбБ4: the answer to a question of a group goes with Link: the group is a
// form, its radios have names and its decisions are its buttons.
func TestGroupChoiceWithoutScripts(t *testing.T) {
	f := newDupFixture(t)
	f.load(t)
	g := onlyGroup(t, f.page(t))
	if g.Form == nil {
		t.Fatalf("the group has no form: its choice goes nowhere without scripts")
	}
	if g.Form.Method != "POST" || g.Form.Action != "/app/duplicates/decide" || formRow(*g.Form, "group") == nil ||
		formRow(*g.Form, "group").Value != view.Text(g.ID) || formRow(*g.Form, "_csrf") == nil {
		t.Fatalf("group form = %+v", g.Form)
	}
	// The organisation only one of them has is a question too (Дб7,
	// v0.2.283; until then the name was the only one).
	if len(g.Choices) != 2 || g.Choices[0].Name != "choice.FN" || g.Choices[0].Property != "Display name" {
		t.Fatalf("choices = %+v", g.Choices)
	}
	link := actionOf(t, g, "duplicates.link")
	if !link.Submits || actionOf(t, g, "duplicates.ignore").Submits != true || !actionOf(t, g, "duplicates.merge").Submits {
		t.Fatalf("the decisions are not the form's buttons: %+v", g.Actions)
	}
	pick := choiceValue(t, g.Choices[0], "LOVELACE Ada")
	rec := f.h.do(postForm(link.Href, f.sess, browserForm(*g.Form, map[string]string{g.Choices[0].Name: pick})))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("link: %d %s", rec.Code, rec.Body.String())
	}
	if d := f.decisions(t); len(d.Linked) != 1 || d.Linked[0].Prefs["FN"].UID != "ada-there" {
		t.Fatalf("the answer was not kept: %+v", d.Linked)
	}
}

// ИБ5: «Preview file» names its file field: without scripts the page sends
// the file as a form of its own, and the server reads the part «file».
func TestImportPreviewNamesItsFileField(t *testing.T) {
	f := importFixture(t)
	page := decode[view.ImportPage](t, f.h.do(get(f.path("contacts")+"/import", f.sess)))
	choose := actionByID(page.Actions, "import.choose")
	if choose == nil || choose.File != "file" || choose.Method != "POST" {
		t.Fatalf("Preview file = %+v", choose)
	}
	page = decode[view.ImportPage](t, f.h.do(importRequest(choose.Href, f.sess, false, "cards.vcf", []byte(importedCards))))
	if page.Step != view.ImportStepPreview {
		t.Fatalf("step = %q, notices %+v", page.Step, page.Notices)
	}
}

// ИБ4: «Into» is a menu of links — each writable collection is the wizard
// with it chosen; a read-only one is shown and leads nowhere. The way in
// from a combined view asks «Into» and does not pick one in silence.
func TestImportIntoIsLinks(t *testing.T) {
	f := importFixture(t)
	page := decode[view.ImportPage](t, f.h.do(get(f.path("contacts")+"/import", f.sess)))
	for _, o := range page.Into.Options {
		if o.ReadOnly != (o.Href == "") {
			t.Errorf("option %q: read-only %v, address %q", o.Label, o.ReadOnly, o.Href)
		}
	}
	friends := importOption(t, page, "Friends")
	if friends.Href == "" {
		t.Fatalf("Friends leads nowhere: %+v", friends)
	}
	again := decode[view.ImportPage](t, f.h.do(get(friends.Href, f.sess)))
	if again.Into.Chosen != friends.ID {
		t.Errorf("following Friends chose %q", again.Into.Chosen)
	}

	tickAll(t, f.h, f.sess)
	rec := f.h.do(get("/app/contacts/import", f.sess))
	if rec.Code != http.StatusOK {
		t.Fatalf("the way in from the combined view: %d %q — it chooses «Into» in silence", rec.Code, rec.Header().Get("Location"))
	}
	entry := decode[view.ImportPage](t, rec)
	choose := actionByID(entry.Actions, "import.choose")
	if entry.Into.Chosen != "" || choose == nil || choose.Enabled || choose.Reason == "" || len(entry.Into.Options) < 2 {
		t.Fatalf("the entry does not ask «Into»: chosen %q, Preview file %+v", entry.Into.Chosen, choose)
	}
	if o := importOption(t, entry, "People"); o.Href == "" {
		t.Errorf("People leads nowhere: %+v", o)
	}
}

// ИБ6: the folders of notes are chosen by links, as the trees of Files.
func TestImportFolderTreeIsLinks(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	srv.AddAddressBook(carddav.AddressBook{Path: peopleBook, Name: "People"})
	notebook := davtest.CalendarHome + "notes/"
	srv.AddCalendar(caldav.Calendar{Path: notebook, Name: "Notebook", SupportedComponentSet: []string{"VJOURNAL"}})
	srv.SeedFolder(davtest.FileRoot + "docs/")
	srv.SeedFolder(davtest.FileRoot + "docs/inner/")
	f := connectFixture(t, srv, notebook, nil)
	addFileCollection(t, f, davtest.FileRoot+"docs/", "Docs")
	page := decode[view.ImportPage](t, f.h.do(get(f.path("notes")+"/import", f.sess)))
	if page.Source.Folder == nil || len(page.Source.Folder.Nodes) != 1 || page.Source.Folder.Nodes[0].Href == "" {
		t.Fatalf("the storage leads nowhere: %+v", page.Source.Folder)
	}
	page = decode[view.ImportPage](t, f.h.do(get(page.Source.Folder.Nodes[0].Href, f.sess)))
	kids := page.Source.Folder.Nodes[0].Children
	if len(kids) != 1 || kids[0].Href == "" || kids[0].Href != kids[0].ID {
		t.Fatalf("the folder inside leads nowhere: %+v", kids)
	}
}

// КА1, КА2, КГ2: every collection sheet has its title and a way out, and
// the destinations of a move are links.
func TestCollectionSheetsHaveATitleAndAWayOut(t *testing.T) {
	f := newColMoveFixture(t)
	q := "?" + url.Values{"a": {f.id}, "c": {f.col}}.Encode()
	newSheet := decode[view.CollectionNewSheet](t, f.h.do(get("/app/collections/new?kind=event", f.sess)))
	rename := decode[view.CollectionRenameSheet](t, f.h.do(get("/app/collections/rename"+q, f.sess)))
	move := decode[view.CollectionMoveSheet](t, f.h.do(get("/app/collections/move"+q, f.sess)))
	copied := decode[view.CollectionMoveSheet](t, f.h.do(get("/app/collections/move"+q+"&copy=1", f.sess)))
	del := decode[view.CollectionDeleteSheet](t, f.h.do(get("/app/collections/delete"+q, f.sess)))
	for _, s := range []struct {
		title, want view.Text
		actions     []view.Action
	}{
		{newSheet.Title, "New calendar", newSheet.Actions},
		{rename.Title, "Rename and colour", rename.Actions},
		{move.Title, "Move calendar", move.Actions},
		{copied.Title, "Copy calendar", copied.Actions},
		{del.Title, "Delete on the server", del.Actions},
	} {
		if s.title != s.want {
			t.Errorf("title %q, want %q", s.title, s.want)
		}
		cancel := actionByID(s.actions, "collection.cancel")
		if cancel == nil || cancel.Method != "GET" || cancel.Href == "" || !cancel.Enabled {
			t.Errorf("%s: no way out of the sheet: %+v", s.want, s.actions)
		}
	}
	// From the rail the way out is the collection's list (the new one's:
	// the kind's combined view); from the settings, the settings.
	if c := actionByID(rename.Actions, "collection.cancel"); c == nil || c.Href != f.path("calendar") {
		t.Errorf("Cancel from the rail = %+v", c)
	}
	if c := actionByID(newSheet.Actions, "collection.cancel"); c == nil || c.Href != "/app/calendar" {
		t.Errorf("Cancel of a new calendar = %+v", c)
	}
	back := decode[view.CollectionRenameSheet](t, f.h.do(get("/app/collections/rename"+q+"&back=%2Fapp%2Fsettings%2Fconnections", f.sess)))
	if c := actionByID(back.Actions, "collection.cancel"); c == nil || c.Href != "/app/settings/connections" {
		t.Errorf("Cancel from the settings = %+v", c)
	}
	evil := decode[view.CollectionRenameSheet](t, f.h.do(get("/app/collections/rename"+q+"&back=https%3A%2F%2Fevil.example%2F", f.sess)))
	if c := actionByID(evil.Actions, "collection.cancel"); c == nil || strings.Contains(c.Href, "evil") {
		t.Errorf("Cancel leads off the instance: %+v", c)
	}
	settings := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	row := connectionRow(t, settings, f.id, f.col)
	for _, id := range []string{"collection.rename", "collection.move", "collection.delete"} {
		if h := menuHref(t, row.Menu, id); !strings.Contains(h, "back=%2Fapp%2Fsettings%2Fconnections") {
			t.Errorf("%s from the settings does not come back there: %q", id, h)
		}
	}
	for _, o := range move.Target.Options {
		if o.ReadOnly != (o.Href == "") || (o.Href != "" && o.Href != o.ID) {
			t.Errorf("destination %q: read-only %v, address %q", o.Label, o.ReadOnly, o.Href)
		}
	}
}
