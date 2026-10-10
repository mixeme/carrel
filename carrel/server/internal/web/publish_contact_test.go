package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// S-25.1, s-publish-3: a phone and an address belong to a person who does
// not know of the link, so the sheet of a contact — and of a book — asks of
// each field apart what the visitor sees, and almost all of it is off until
// asked for: «Name and organisation» stands, «Phone numbers», «Email
// addresses», «Photo» and «Note» do not, nor does the .vcf. «Name and
// organisation» can be taken off (the owner's decision of 10.10.2026): the
// visitor is then given the name alone. What the link
// was made with is what its page, the rows of a book and the file give — the
// file obeys the same ticks, or the download would go round them — and
// nothing the sheet does not name (an address, a birthday, a property Carrel
// does not know) is given at all.

const fullCard = "BEGIN:VCARD\nVERSION:4.0\nUID:grace\nFN:Grace Hopper\nN:Hopper;Grace;;;\nORG:Navy\nTITLE:Rear admiral\n" +
	"TEL;TYPE=cell:+1 555 0100\nEMAIL:grace@example.org\nADR:;;1 Main St;Arlington;VA;22201;USA\nBDAY:19061209\n" +
	"NOTE:Met at the conference.\nURL:https://example.org/grace\nX-SECRET-FLAG:keep-out\nPHOTO:data:image/png;base64,iVBORw0KGgo=\nEND:VCARD\n"

// publishWith is the link a post of the sheet of a contact or a book makes
// with these ticks beside «Name and organisation», which stands as the sheet
// has it; "-name" takes that one off. The form is posted as a browser posts
// it: with how long the link lives.
func publishWith(t *testing.T, f davFixture, href string, ticks ...string) string {
	t.Helper()
	sheet := decode[view.PublishSheet](t, f.h.do(get(href, f.sess)))
	form := url.Values{"name": {"on"}}
	for _, o := range rowNamed(t, sheet.Form, "lives").Options {
		if o.Selected {
			form.Set("lives", o.Value)
		}
	}
	for _, name := range ticks {
		if name == "-name" {
			form.Del("name")
			continue
		}
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
		// It stands, and it is the owner's to take off.
		if name := sees.Rows[0]; name.ReadOnly || name.Value != "on" || name.Name != "name" {
			t.Errorf("%s: «%s» = %+v, want on and free to be taken off", what, name.Label, name)
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

	// A post that is not the sheet's — nothing in it — makes the link the
	// sheet stands on: the organisation is given.
	plainSheet := decode[view.PublishSheet](t, f.h.do(get(one, f.sess)))
	plain := decode[view.PublishSheet](t, f.h.do(postForm(actionHref(t, plainSheet.Actions, "publish.create"), f.sess, url.Values{})))
	if p := publicPage(t, f, string(plain.Link)); string(p.Title) != "Grace Hopper" || !strings.Contains(fields(p), "Navy") {
		t.Errorf("a contact published by a bare post: %q, %q", p.Title, fields(p))
	}

	// «Name and organisation» taken off: the name alone, on the page and in
	// the file — no organisation, no title.
	nameOnly := publishWith(t, f, one, "-name", "download")
	lone := f.h.do(anon(nameOnly)).Body.String()
	if p := publicPage(t, f, nameOnly); string(p.Title) != "Grace Hopper" || strings.Contains(lone, "Navy") || strings.Contains(lone, "Rear admiral") {
		t.Errorf("a contact published without its organisation: %q, %q", p.Title, fields(p))
	}
	if _, leaked := has(lone); len(leaked) > 0 {
		t.Errorf("a contact published without its organisation gives %q", leaked)
	}
	loneFile := f.h.do(anon(nameOnly + "/download")).Body.String()
	if !strings.Contains(loneFile, "FN:Grace Hopper") || !strings.Contains(loneFile, "N:Hopper;Grace") || strings.Contains(loneFile, "ORG") || strings.Contains(loneFile, "TITLE") {
		t.Errorf("the file of a contact published without its organisation:\n%s", loneFile)
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
	// Taken off for a book, no row of it names an organisation.
	for _, r := range publicPage(t, f, publishWith(t, f, book, "-name")).Contacts {
		if r.Name == "" || r.Org != "" {
			t.Errorf("a book published without organisations: row %+v", r)
		}
	}
	whole := f.h.do(anon(open + "/download")).Body.String()
	if missing, leaked := has(whole, "grace@example.org"); len(missing)+len(leaked) > 0 || !strings.Contains(whole, "ada@example.com") || !strings.Contains(whole, "FN:Bob Builder") {
		t.Errorf("the file of a book with addresses ticked lacks %q, gives %q:\n%s", missing, leaked, whole)
	}
}

// s-publish-5: the sheet of a folder says, in a message of its own, what the
// visitor gets — the folder whole, read-only, and the same address as a
// drive — before the link is made and after; its link lives 14 days unless
// changed. «Expires» is the day alone, never a sentence beginning with a
// full stop.
func TestFolderSheetSaysWhatTheVisitorGets(t *testing.T) {
	frame := mockupFrame(t, "s-publish-5")
	msg := frameText(frameOne(t, regexp.MustCompile(`(?s)<div class="tc-msg tc-msg--flush">(.*?)</div>`), frame, "message")[1])
	lives := frameText(frameOne(t, regexp.MustCompile(`<label>Link lives for</label><button class="tc-in is-pick" type="button">([^<]*)</button>`), frame, "lifetime")[1])
	if !strings.Contains(msg, "drive") || lives == "" {
		t.Fatalf("s-publish-5 says %q and %q: read the frame again", msg, lives)
	}

	f := pubDocs(t)
	href := "/app/publish?" + url.Values{"k": {"files"}, "a": {f.id}, "c": {filesDocs}, "f": {"share"}, "dir": {"1"}}.Encode()
	sheet := decode[view.PublishSheet](t, f.h.do(get(href, f.sess)))
	chosen := ""
	for _, o := range rowNamed(t, sheet.Form, "lives").Options {
		if o.Selected {
			chosen = string(o.Label)
		}
	}
	if string(sheet.Note) != msg || sheet.Expires != "" || chosen != lives {
		t.Errorf("the sheet of a folder: note %q, expires %q, lives %q; want %q, nothing, %q", sheet.Note, sheet.Expires, chosen, msg, lives)
	}
	made := decode[view.PublishSheet](t, f.h.do(postForm(actionHref(t, sheet.Actions, "publish.create"), f.sess, url.Values{})))
	if made.Link == "" || string(made.Note) != msg || !strings.HasPrefix(string(made.Expires), "Until ") || strings.Contains(string(made.Expires), "drive") {
		t.Errorf("the link made: note %q, expires %q", made.Note, made.Expires)
	}
	// Left as the sheet stands, the link lives what the sheet said.
	var days int
	for _, l := range f.h.users.PublishedLinks(f.sess.AccountID()) {
		if l.ExpiresAt != nil {
			days = int(l.ExpiresAt.Sub(l.CreatedAt).Hours()/24 + 0.5)
		}
	}
	if want := strings.TrimSuffix(lives, " days"); strconv.Itoa(days) != want {
		t.Errorf("the link lives %d days, the sheet said %q", days, lives)
	}
	// A file has no such message: there is nothing to mount.
	one := decode[view.PublishSheet](t, f.h.do(get("/app/publish?"+url.Values{"k": {"files"}, "a": {f.id}, "c": {filesDocs}, "f": {"secret.txt"}}.Encode(), f.sess)))
	if one.Note != "" || one.Expires != "" {
		t.Errorf("the sheet of a file: note %q, expires %q", one.Note, one.Expires)
	}
}
