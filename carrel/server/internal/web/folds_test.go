package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// What the frames draw as a button or a check that opens or switches
// something has somewhere to go without scripts (U-8; the client's stop of
// the contacts section, C12–C14): «Show 3» by the kept properties
// (s-contacts-1, -6), «Show N empty fields» under a form (s-contacts-3),
// «Compact rows» — a check of a form (s-contacts-1, components-38), and the
// name of the search box's field.

// «Show 3» is a link to the same card with the kept properties open: there
// the button is gone and the properties stay.
func TestForeignShowOpensThem(t *testing.T) {
	app := newMockupApp(t)
	card := app.listPath(t, "contacts", "kontakty") + "/4f2b-9c31-a0e7"
	page := decode[view.ContactPage](t, app.h.do(get(card, app.sess)))
	show := page.ForeignShow
	if show == nil || len(page.Foreign) == 0 || show.Method != "GET" || !show.Enabled || show.Href == "" || !strings.HasPrefix(string(show.Label), "Show ") {
		t.Fatalf("foreign show = %+v (%d kept)", show, len(page.Foreign))
	}
	open := decode[view.ContactPage](t, app.h.do(get(show.Href, app.sess)))
	if open.ForeignShow != nil || len(open.Foreign) != len(page.Foreign) || open.Name != page.Name {
		t.Errorf("opened: show %+v, %d kept of %d", open.ForeignShow, len(open.Foreign), len(page.Foreign))
	}

	// The panel beside the list: the list with the panel open and the
	// properties open; its other links do not keep the opening.
	list := app.listPath(t, "contacts", "kontakty")
	withPanel := decode[view.ContactsListPage](t, app.h.do(get(list+"?open="+url.QueryEscape(card), app.sess)))
	d := withPanel.Details
	if d == nil || d.ForeignShow == nil || d.ForeignShow.Method != "GET" {
		t.Fatalf("panel = %+v", d)
	}
	opened := decode[view.ContactsListPage](t, app.h.do(get(d.ForeignShow.Href, app.sess)))
	if opened.Details == nil || opened.Details.ForeignShow != nil || len(opened.Details.Foreign) != len(d.Foreign) {
		t.Fatalf("panel opened = %+v", opened.Details)
	}
	if strings.Contains(opened.Details.Close, "foreign") || (opened.Bar.Compact != nil && strings.Contains(opened.Bar.Compact.Action, "foreign")) {
		t.Errorf("the opening leaks into the list's links: close %q", opened.Details.Close)
	}
}

// «Show N empty fields» is a link to the same form with every field in its
// rubric; the form, posted from there, stays so.
func TestEmptyFieldsUnfold(t *testing.T) {
	app := newMockupApp(t)
	edit := app.listPath(t, "contacts", "kontakty") + "/4f2b-9c31-a0e7/edit"
	page := decode[view.ContactEditPage](t, app.h.do(get(edit, app.sess)))
	last := page.Form.Sets[len(page.Form.Sets)-1]
	if !last.Folded || last.Unfold == nil || last.Unfold.Method != "GET" || last.Unfold.Label != last.Legend || last.Unfold.Href == "" {
		t.Fatalf("the fold = %q folded %v, unfold %+v", last.Legend, last.Folded, last.Unfold)
	}
	folded := map[string]bool{}
	for _, r := range last.Rows {
		folded[r.Name] = true
	}
	open := decode[view.ContactEditPage](t, app.h.do(get(last.Unfold.Href, app.sess)))
	seen := map[string]bool{}
	for _, s := range open.Form.Sets {
		if s.Folded || s.Unfold != nil {
			t.Errorf("unfolded, yet %q folds", s.Legend)
		}
		for _, r := range s.Rows {
			seen[r.Name] = true
		}
	}
	for name := range folded {
		if !seen[name] {
			t.Errorf("unfolded, %q is not in the form", name)
		}
	}
	form := formFields(open.Form)
	form.Set("add", "tel")
	again := decode[view.ContactEditPage](t, app.h.do(postForm(open.Form.Action, app.sess, form)))
	for _, s := range again.Form.Sets {
		if s.Folded {
			t.Errorf("posted from the unfolded form, %q folds again", s.Legend)
		}
	}
}

// «Compact rows» is a check of a GET form on the list (s-contacts-1,
// components-38): ticked it sends compact=1, unticked the hidden 0 before
// it; the list's query goes along; the choice is this browser's.
func TestCompactRowsIsACheck(t *testing.T) {
	app := newMockupApp(t)
	list := app.listPath(t, "contacts", "kontakty")
	page := decode[view.ContactsListPage](t, app.h.do(get(list+"?sort=family", app.sess)))
	f := page.Bar.Compact
	if f == nil || f.Method != "GET" || f.Action != list {
		t.Fatalf("compact = %+v", f)
	}
	var check *view.FormRow
	sent := url.Values{}
	for _, s := range f.Sets {
		for i, r := range s.Rows {
			switch {
			case r.Kind == view.InputHidden:
				sent.Add(r.Name, string(r.Value))
			case r.Kind == view.InputCheck && r.Name == "compact":
				check = &s.Rows[i]
			default:
				t.Errorf("compact: a row %q of kind %s", r.Name, r.Kind)
			}
		}
	}
	if check == nil || len(check.Options) != 1 || check.Options[0].Label != "Compact rows" || check.Options[0].Value != "1" || check.Options[0].Selected {
		t.Fatalf("the check = %+v", check)
	}
	if sent.Get("sort") != "family" || sent.Get("compact") != "0" {
		t.Errorf("hidden = %v: the list's query and compact=0", sent)
	}
	ticked := url.Values{}
	for k, v := range sent {
		ticked[k] = append([]string(nil), v...)
	}
	ticked.Add("compact", "1")
	rec := app.h.do(get(list+"?"+ticked.Encode(), app.sess))
	if rec.Code != http.StatusSeeOther || strings.Contains(rec.Header().Get("Location"), "compact") || !strings.Contains(rec.Header().Get("Location"), "sort=family") {
		t.Fatalf("ticked: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	var cookie *http.Cookie
	for _, k := range rec.Result().Cookies() {
		if k.Name == "carrel_compact_contacts" {
			cookie = k
		}
	}
	if cookie == nil || cookie.Value != "1" {
		t.Fatalf("cookie = %+v", cookie)
	}
	req := get(list, app.sess)
	req.AddCookie(cookie)
	on := decode[view.ContactsListPage](t, app.h.do(req)).Bar.Compact
	if on == nil || !on.Sets[len(on.Sets)-1].Rows[len(on.Sets[len(on.Sets)-1].Rows)-1].Options[0].Selected {
		t.Errorf("after the choice = %+v", on)
	}
	rec = app.h.do(get(list+"?"+sent.Encode(), app.sess))
	for _, k := range rec.Result().Cookies() {
		if k.Name == "carrel_compact_contacts" && k.MaxAge >= 0 {
			t.Errorf("unticked: cookie %+v, want it dropped", k)
		}
	}
}

// The search box names its field: the client does not invent «q».
func TestSearchBoxNamesItsField(t *testing.T) {
	app := newMockupApp(t)
	page := decode[view.ContactsListPage](t, app.h.do(get(app.listPath(t, "contacts", "kontakty"), app.sess)))
	box := page.Top.Search
	if box.Name != "q" || box.Action != "/app/search" {
		t.Fatalf("search box = %+v", box)
	}
	found := decode[view.SearchPage](t, app.h.do(get(box.Action+"?"+box.Name+"="+url.QueryEscape("ковал"), app.sess)))
	if found.Top.Search.Query != "ковал" {
		t.Errorf("searched %q", found.Top.Search.Query)
	}
}
