package web_test

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/view"
)

// The rows of a list open in the details panel with the live layer, the
// page kept (v0.11.55); the row lit stayed the one opened before. The panel
// of a list asked by its row (?open=, target details) comes with the list
// out of band, its open row on, its rows still opening in the panel.
func TestThePanelsAnswerLightsItsRow(t *testing.T) {
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.Seed(searchBook+"ada.vcf", adaCard())
	srv.Seed(searchBook+"bob.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:bob\nFN:Bob Builder\nEND:VCARD\n")
	f := connectFixture(t, srv, searchBook, nil)
	tickAll(t, f.h, f.sess)
	list := combined(t, f.h, f.sess, "/app/contacts", contactsPoll)
	var bob string
	for _, g := range list.Groups {
		for _, r := range g.Rows {
			if string(r.Name) == "Bob Builder" {
				bob = r.Href
			}
		}
	}
	if !strings.Contains(bob, "open=") {
		t.Fatalf("Bob's row leads to %q, not the panel", bob)
	}
	req := get(bob, f.sess)
	req.Header.Set("Carrel-View", "fragment")
	req.Header.Set("Carrel-Target", "details")
	rec := f.h.do(req)
	var answer struct {
		Target string `json:"target"`
		Extra  []struct {
			Target string          `json:"target"`
			Body   json.RawMessage `json:"body"`
		} `json:"extra"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil || answer.Target != "details" {
		t.Fatalf("the panel's answer: %v %.300s", err, rec.Body.String())
	}
	for _, x := range answer.Extra {
		if x.Target != "main" {
			continue
		}
		var page view.ContactsListPage
		if err := json.Unmarshal(x.Body, &page); err != nil {
			t.Fatal(err)
		}
		for _, g := range page.Groups {
			for _, r := range g.Rows {
				if (string(r.Name) == "Bob Builder") != r.On {
					t.Errorf("%s on = %v", r.Name, r.On)
				}
				if u, _ := url.Parse(r.Href); u == nil || u.Query().Get("open") == "" {
					t.Errorf("%s leads to %s, not the panel", r.Name, r.Href)
				}
			}
		}
		return
	}
	t.Errorf("the panel's answer brings no list: %.300s", rec.Body.String())
}
