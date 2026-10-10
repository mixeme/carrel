package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// S-25.1, s-publish-3: a phone and an address belong to a person who does
// not know of the link, so the sheet of a contact — and of a book — asks of
// each field apart what the visitor sees, and almost all of it is off until
// asked for: «Name and organisation» stands, «Phone numbers», «Email
// addresses», «Photo» and «Note» do not, nor does the .vcf. What the link
// was made with is what its page, the rows of a book and the file give — the
// file obeys the same ticks, or the download would go round them — and
// nothing the sheet does not name (an address, a birthday, a property Carrel
// does not know) is given at all.

const fullCard = "BEGIN:VCARD\nVERSION:4.0\nUID:grace\nFN:Grace Hopper\nN:Hopper;Grace;;;\nORG:Navy\nTITLE:Rear admiral\n" +
	"TEL;TYPE=cell:+1 555 0100\nEMAIL:grace@example.org\nADR:;;1 Main St;Arlington;VA;22201;USA\nBDAY:19061209\n" +
	"NOTE:Met at the conference.\nURL:https://example.org/grace\nX-SECRET-FLAG:keep-out\nPHOTO:data:image/png;base64,iVBORw0KGgo=\nEND:VCARD\n"

// contactSheet is the publish sheet of a contact or a book as the frame
// draws it, and what a post of the form with these ticks makes.
func publishWith(t *testing.T, f davFixture, href string, ticks ...string) string {
	t.Helper()
	sheet := decode[view.PublishSheet](t, f.h.do(get(href, f.sess)))
	form := url.Values{}
	for _, name := range ticks {
		form.Set(name, "on")
	}
	got := decode[view.PublishSheet](t, f.h.do(postForm(actionHref(t, sheet.Actions, "publish.create"), f.sess, form)))
	if !strings.HasPrefix(string(got.Link), "/p/") {
		t.Fatalf("no link: %+v", got)
	}
	return string(got.Link)
}

func TestPublishedContactShowsWhatWasTicked(t *testing.T) {
	// The frame's sheet: the rubric, the five ticks and which stand, the
	// tick of the file, how long the link lives.
	frame := mockupFrame(t, "s-publish-3")
	rubric := frameOne(t, regexp.MustCompile(`<div class="tc-field tc-field--w100"><label>([^<]*)</label>`), frame, "rubric of the ticks")[1]
	var drawn []string
	on := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?s)<button class="tc-check" type="button"><span class="tc-tickrow__tick( is-off)?"></span>(.*?)</button>`).FindAllStringSubmatch(frame, -1) {
		drawn = append(drawn, frameText(m[2]))
		on[frameText(m[2])] = m[1] == ""
	}
	lives := frameText(frameOne(t, regexp.MustCompile(`<label>Link lives for</label><button class="tc-in is-pick" type="button">([^<]*)</button>`), frame, "lifetime")[1])
	if len(drawn) != 6 || !on[drawn[0]] || on[drawn[1]] || on[drawn[2]] || on[drawn[3]] || on[drawn[4]] || on[drawn[5]] {
		t.Fatalf("s-publish-3 draws %q, standing %v: read the frame again", drawn, on)
	}

	f := publishFixture(t)
	f.srv.Seed(searchBook+"grace.vcf", fullCard)
	one, book := f.publishHref("contacts", searchBook, "grace"), f.publishHref("contacts", searchBook, "")
	for what, href := range map[string]string{"a contact": one, "a book": book} {
		sheet := decode[view.PublishSheet](t, f.h.do(get(href, f.sess)))
		var sees *view.FieldSet
		rows := map[string]view.FormRow{}
		var labels []string
		for i, set := range sheet.Form.Sets {
			if string(set.Legend) == rubric {
				sees = &sheet.Form.Sets[i]
			}
			for _, r := range set.Rows {
				rows[r.Name] = r
				if r.Kind == view.InputCheck {
					labels = append(labels, string(r.Label))
				}
			}
		}
		if sees == nil || len(sees.Rows) != 5 || strings.Join(labels, "|") != strings.Join(drawn, "|") {
			t.Fatalf("%s: the ticks are %q under %+v, want %q under %q", what, labels, sees, drawn, rubric)
		}
		for _, r := range rows {
			if r.Kind == view.InputCheck && (r.Value == "on") != on[string(r.Label)] {
				t.Errorf("%s: «%s» stands %v, the frame draws %v", what, r.Label, r.Value == "on", on[string(r.Label)])
			}
		}
		// The name is what the page is of: it cannot be taken off.
		if name := sees.Rows[0]; !name.ReadOnly || name.Value != "on" {
			t.Errorf("%s: «%s» = %+v, want on and not to be changed", what, name.Label, name)
		}
		chosen := ""
		for _, o := range rows["lives"].Options {
			if o.Selected {
				chosen = string(o.Label)
			}
		}
		if chosen != lives {
			t.Errorf("%s: the link lives for %q, the frame says %q", what, chosen, lives)
		}
	}

	fields := func(p view.PublishedPage) string {
		var out []string
		for _, fl := range p.Fields {
			out = append(out, string(fl.Label)+"="+string(fl.Value))
		}
		return strings.Join(out, "; ")
	}
	secrets := []string{"555 0100", "grace@example.org", "Main St", "1906", "Met at the conference", "example.org/grace", "keep-out", "iVBORw0KGgo"}
	has := func(body string, want ...string) (missing, leaked []string) {
		for _, s := range secrets {
			wanted := false
			for _, w := range want {
				wanted = wanted || w == s
			}
			switch found := strings.Contains(body, s); {
			case wanted && !found:
				missing = append(missing, s)
			case !wanted && found:
				leaked = append(leaked, s)
			}
		}
		return missing, leaked
	}

	// As the sheet stands: the name and the organisation, nothing else, no file.
	bare := publishWith(t, f, one)
	page := f.h.do(anon(bare))
	if _, leaked := has(page.Body.String()); len(leaked) > 0 {
		t.Errorf("a contact published as the sheet stands gives %q", leaked)
	}
	if p := publicPage(t, f, bare); string(p.Title) != "Grace Hopper" || !strings.Contains(fields(p), "Navy") || len(p.Actions) != 0 {
		t.Errorf("a contact published as the sheet stands: %q, %q, actions %+v", p.Title, fields(p), p.Actions)
	}
	if rec := f.h.do(anon(bare + "/download")); rec.Code != http.StatusNotFound {
		t.Errorf("the file of a link made without it: %d", rec.Code)
	}

	// Phones and the file ticked: the phone on the page and in the file, and
	// still nothing else.
	phones := publishWith(t, f, one, "phones", "download")
	if missing, leaked := has(f.h.do(anon(phones)).Body.String(), "555 0100"); len(missing)+len(leaked) > 0 {
		t.Errorf("with phones ticked the page lacks %q and gives %q", missing, leaked)
	}
	file := f.h.do(anon(phones + "/download"))
	if missing, leaked := has(file.Body.String(), "555 0100"); file.Code != http.StatusOK || len(missing)+len(leaked) > 0 {
		t.Errorf("with phones ticked the file (%d) lacks %q and gives %q", file.Code, missing, leaked)
	}
	for _, kept := range []string{"BEGIN:VCARD", "FN:Grace Hopper", "ORG:Navy", "UID:grace", "END:VCARD"} {
		if !strings.Contains(file.Body.String(), kept) {
			t.Errorf("the file has no %q:\n%s", kept, file.Body)
		}
	}

	// Everything ticked: the four, and never what the sheet does not name.
	all := publishWith(t, f, one, "phones", "emails", "photo", "note", "download")
	if missing, leaked := has(f.h.do(anon(all)).Body.String(), "555 0100", "grace@example.org", "Met at the conference"); len(missing)+len(leaked) > 0 {
		t.Errorf("with every tick the page lacks %q and gives %q", missing, leaked)
	}
	if missing, leaked := has(f.h.do(anon(all+"/download")).Body.String(), "555 0100", "grace@example.org", "Met at the conference", "iVBORw0KGgo"); len(missing)+len(leaked) > 0 {
		t.Errorf("with every tick the file lacks %q and gives %q", missing, leaked)
	}

	// A book: the ticks are for every card of it at once.
	shut := publishWith(t, f, book)
	if _, leaked := has(f.h.do(anon(shut)).Body.String()); len(leaked) > 0 || strings.Contains(f.h.do(anon(shut)).Body.String(), "ada@example.com") {
		t.Errorf("a book published as the sheet stands gives %q or Ada's address", leaked)
	}
	open := publishWith(t, f, book, "emails", "download")
	var grace view.ContactRow
	for _, r := range publicPage(t, f, open).Contacts {
		if r.Name == "Grace Hopper" {
			grace = r
		}
	}
	if grace.Email != "grace@example.org" || grace.Phone != "" || grace.Org != "Navy" {
		t.Errorf("a book with addresses ticked: Grace's row is %+v", grace)
	}
	whole := f.h.do(anon(open + "/download")).Body.String()
	if missing, leaked := has(whole, "grace@example.org"); len(missing)+len(leaked) > 0 || !strings.Contains(whole, "ada@example.com") || !strings.Contains(whole, "FN:Bob Builder") {
		t.Errorf("the file of a book with addresses ticked lacks %q, gives %q:\n%s", missing, leaked, whole)
	}
}
