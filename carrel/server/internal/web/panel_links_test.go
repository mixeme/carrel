package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// recordHref is the record a row leads to: the row's address, or, when the
// row opens in the panel, the record its ?open= names.
func recordHref(href string) string {
	if _, query, ok := strings.Cut(href, "?"); ok {
		if q, err := url.ParseQuery(query); err == nil && q.Get("open") != "" {
			return q.Get("open")
		}
	}
	return href
}

// «Keep the details panel open» (Look.Panel; Typecase's note 75, task 186
// p. 3): ticked — as it stands without a cookie — a row of a list leads to
// the same list with ?open=, and that answer holds the record in the panel;
// not ticked, to the record's page. The list is otherwise the same.
func TestRowsOpenInThePanel(t *testing.T) {
	f := bookFixture(t, nil, "BEGIN:VCARD\nVERSION:3.0\nUID:ada\nFN:Ada Lovelace\nEND:VCARD\n")
	list := f.path("contacts")
	page := decode[view.ContactsListPage](t, f.h.do(get(list, f.sess)))
	row := page.Groups[0].Rows[0]
	if !strings.Contains(row.Href, "?open=") || recordHref(row.Href) != list+"/ada" || row.Target != "details" {
		t.Fatalf("a row with the panel kept open = %+v", row)
	}
	panel := decode[view.ContactsListPage](t, f.h.do(get(row.Href, f.sess)))
	if panel.Details == nil || panel.Details.Source.UID != "ada" {
		t.Errorf("the row's address holds no panel: %+v", panel.Details)
	}
	req := get(list, f.sess)
	req.AddCookie(&http.Cookie{Name: "carrel_look", Value: "panel=0"})
	page = decode[view.ContactsListPage](t, f.h.do(req))
	if got := page.Groups[0].Rows[0].Href; got != list+"/ada" {
		t.Errorf("a row with the panel not kept = %q", got)
	}
}
