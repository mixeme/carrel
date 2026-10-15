package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// existingOption is an option of the question about records already there.
func existingOption(t *testing.T, page view.ImportPage, id string) view.Action {
	t.Helper()
	if page.Existing == nil {
		t.Fatalf("the preview asks nothing about the records already there: counts %+v", page.Counts)
	}
	for _, a := range page.Existing.Options {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("no option %q in %+v", id, page.Existing.Options)
	return view.Action{}
}

// asCopies answers the question of a preview about the records already
// there with «Import as copies» — what every import did before the question
// was asked. A preview that asks nothing stays as it is.
func asCopies(t *testing.T, f davFixture, page view.ImportPage) view.ImportPage {
	t.Helper()
	if page.Existing == nil {
		return page
	}
	return decode[view.ImportPage](t, f.h.do(get(existingOption(t, page, "import.existing.copy").Href, f.sess)))
}

// А7 of the audit of 11.10.2026 (the owner's decision of 12.10.2026): a file
// whose records the collection already holds — told by their UID — is asked
// about on the preview: skip them, which is what happens without an answer,
// replace them with the file's version, or import them as copies. The
// preview's rows and the button say what the press will do.
func TestImportAsksAboutRecordsAlreadyThere(t *testing.T) {
	preview := func(t *testing.T) (davFixture, view.ImportPage) {
		f := importFixture(t)
		page := decode[view.ImportPage](t, f.h.do(get(f.path("contacts")+"/import", f.sess)))
		return f, decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "cards.vcf", []byte(importedCards))))
	}

	// Without an answer: skipped.
	f, page := preview(t)
	// The question is the label of Into (from-typecase note 52, s-io-5).
	if page.Existing.Note != "1 record of this file is already in People" ||
		page.Existing.Hint != "It stays as it is here; the file's version is not written." {
		t.Errorf("the question = %q, %q", page.Existing.Note, page.Existing.Hint)
	}
	skip := existingOption(t, page, "import.existing.skip")
	if !skip.On || skip.Method != "GET" || existingOption(t, page, "import.existing.replace").On || existingOption(t, page, "import.existing.copy").On {
		t.Errorf("options = %+v, want Skip chosen", page.Existing.Options)
	}
	if row := previewRow(t, page.Preview, "Ада"); !strings.Contains(string(row[3]), "Already here") {
		t.Errorf("Ada, skipped = %q", row[3])
	}
	run := actionByID(page.Actions, "import.run")
	if run == nil || run.Label != "Import 1 contact" {
		t.Errorf("the button = %+v, want it to count what it writes", run)
	}
	report := decode[view.ImportPage](t, f.h.do(postForm(run.Href, f.sess, url.Values{}))).Report
	// «Already here, skipped» is a card of its own (s-io-9), not among
	// what the file held of another kind.
	if report == nil || report.Created != 1 || report.Renamed != 0 || report.Here != 1 || report.Skipped != 0 || report.Replaced != 0 ||
		len(report.Heres) != 1 || !strings.HasSuffix(string(report.Heres[0]), " · ada") {
		t.Fatalf("report = %+v", report)
	}
	if paths := f.srv.Paths(peopleBook); len(paths) != 2 {
		t.Errorf("People holds %v, want the card that was there and Grace", paths)
	}
	if body, _, _ := f.srv.Object(peopleBook + "ada.vcf"); !strings.Contains(body, "X-CUSTOM:keep") {
		t.Errorf("the card that was there changed: %q", body)
	}

	// Replace: the card there becomes the file's, under the same address.
	f, page = preview(t)
	page = decode[view.ImportPage](t, f.h.do(get(existingOption(t, page, "import.existing.replace").Href, f.sess)))
	if page.Step != view.ImportStepPreview || !existingOption(t, page, "import.existing.replace").On {
		t.Fatalf("after choosing Replace: step %q, options %+v", page.Step, page.Existing)
	}
	if row := previewRow(t, page.Preview, "Ада"); !strings.Contains(string(row[3]), "Replaces") {
		t.Errorf("Ada, replaced = %q", row[3])
	}
	run = actionByID(page.Actions, "import.run")
	if run.Label != "Import 2 contacts" {
		t.Errorf("the button = %q", run.Label)
	}
	report = decode[view.ImportPage](t, f.h.do(postForm(run.Href, f.sess, url.Values{}))).Report
	if report == nil || report.Created != 1 || report.Replaced != 1 || report.Renamed != 0 || report.Skipped != 0 || len(report.Replaces) != 1 || !strings.HasSuffix(string(report.Replaces[0]), " · ada") {
		t.Fatalf("report = %+v", report)
	}
	if strings.Contains(string(report.Note), "Nothing there was overwritten") || !strings.Contains(string(report.Note), "replaced") {
		t.Errorf("note = %q, want it to say a record was replaced", report.Note)
	}
	if paths := f.srv.Paths(peopleBook); len(paths) != 2 {
		t.Errorf("People holds %v", paths)
	}
	if body, _, _ := f.srv.Object(peopleBook + "ada.vcf"); !strings.Contains(body, "Ада") || !strings.Contains(body, "VERSION:3.0") {
		t.Errorf("the card there is not the file's: %q", body)
	}

	// Copies: as an import always did.
	f, page = preview(t)
	page = decode[view.ImportPage](t, f.h.do(get(existingOption(t, page, "import.existing.copy").Href, f.sess)))
	if row := previewRow(t, page.Preview, "Ада"); row[3] != "New UID will be assigned" {
		t.Errorf("Ada, as a copy = %q", row[3])
	}
	report = decode[view.ImportPage](t, f.h.do(postForm(actionHref(t, page.Actions, "import.run"), f.sess, url.Values{}))).Report
	if report == nil || report.Created != 2 || report.Renamed != 1 || report.Replaced != 0 {
		t.Fatalf("report = %+v", report)
	}
	if paths := f.srv.Paths(peopleBook); len(paths) != 3 {
		t.Errorf("People holds %v", paths)
	}
}

// A file with nothing the collection holds is asked nothing; a file that is
// all there already has nothing to write until the person says what to do.
func TestImportOfWhatIsAllThere(t *testing.T) {
	f := importFixture(t)
	page := decode[view.ImportPage](t, f.h.do(get(f.path("contacts")+"/import", f.sess)))
	choose := actionHref(t, page.Actions, "import.choose")
	fresh := "BEGIN:VCARD\nVERSION:4.0\nUID:brand-new\nFN:New Person\nEND:VCARD\n"
	if p := decode[view.ImportPage](t, f.h.do(importRequest(choose, f.sess, false, "new.vcf", []byte(fresh)))); p.Existing != nil {
		t.Errorf("a file of new records is asked %+v", p.Existing)
	}
	body, _, _ := f.srv.Object(peopleBook + "ada.vcf")
	p := decode[view.ImportPage](t, f.h.do(importRequest(choose, f.sess, false, "same.vcf", []byte(body))))
	run := actionByID(p.Actions, "import.run")
	if p.Existing == nil || run == nil || run.Enabled || run.Reason == "" {
		t.Errorf("a file that is all there: question %+v, button %+v — want the button off with why", p.Existing, run)
	}
}
