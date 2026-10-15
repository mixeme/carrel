package web_test

import (
	"net/http"
	"net/url"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Appearance (appearance-1; from-typecase note 50): the theme, the density
// and the lists' habits go with the one form and are kept in a cookie of
// this browser, not in the account; every page carries them (Shell.Look) —
// a browser without the cookie the frame's defaults. Reset forgets them and
// the columns each section remembered.
func TestAppearanceKeepsTheLookInThisBrowser(t *testing.T) {
	f := calendarFixture(t, nil)
	const settings = "/app/settings/appearance"
	page := decode[view.AppearancePage](t, f.h.do(get(settings, f.sess)))
	if page.Look != (view.Look{Theme: "auto", Panel: true, Source: true}) {
		t.Errorf("the default look = %+v", page.Look)
	}
	if theme := rowNamed(t, page.Form, "theme"); theme.Kind != view.InputRadio || theme.Value != "auto" || len(theme.Options) != 3 {
		t.Errorf("theme = %+v", theme)
	}
	rec := f.h.do(postForm(settings, f.sess, url.Values{"theme": {"dark"}, "density": {"compact"}, "source": {"on"}, "wide": {"on"},
		"weekStart": {"monday"}, "clock": {"24"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	var look *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "carrel_look" {
			look = c
		}
	}
	if look == nil {
		t.Fatal("no cookie keeps the look")
	}
	again := get(f.path("calendar"), f.sess)
	again.AddCookie(look)
	agenda := decode[view.AgendaPage](t, f.h.do(again))
	if agenda.Look != (view.Look{Theme: "dark", Compact: true, Panel: false, Source: true, Wide: true}) {
		t.Errorf("the agenda's look = %+v", agenda.Look)
	}
	// Another browser of the same person sees the defaults.
	if other := decode[view.AgendaPage](t, f.h.do(get(f.path("calendar"), f.sess))); other.Look.Theme != "auto" {
		t.Errorf("another browser's look = %+v", other.Look)
	}

	reset := postForm(page.Reset.Href, f.sess, url.Values{})
	reset.AddCookie(look)
	reset.AddCookie(&http.Cookie{Name: "carrel_cols_tasks", Value: "title,due"})
	rec = f.h.do(reset)
	gone := map[string]bool{}
	for _, c := range rec.Result().Cookies() {
		gone[c.Name] = c.MaxAge < 0
	}
	if !gone["carrel_look"] || !gone["carrel_cols_tasks"] {
		t.Errorf("reset forgot %v", gone)
	}
}
