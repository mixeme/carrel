package web_test

import (
	"encoding/json"
	"testing"

	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/view"
)

// A collection's name in the rail opens it in main with the live layer
// (Typecase's task 292); the rail stayed as the page before drew it, the
// combined row lit (25.11.2026). A page asked into main comes with its rail
// out of band: the row of the collection opened is the one on.
func TestAPageAskedIntoMainBringsItsRail(t *testing.T) {
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.Seed(searchBook+"ada.vcf", adaCard())
	f := connectFixture(t, srv, searchBook, nil)
	tickAll(t, f.h, f.sess)
	list := combined(t, f.h, f.sess, "/app/contacts", contactsPoll)
	var people string
	for _, s := range list.Rail.Sections {
		for _, r := range s.Rows {
			if string(r.Label) == "People" {
				people = r.Href
			}
		}
	}
	if people == "" {
		t.Fatalf("no row «People» in the rail: %+v", list.Rail.Sections)
	}
	req := get(people, f.sess)
	req.Header.Set("Carrel-View", "fragment")
	req.Header.Set("Carrel-Target", "main")
	rec := f.h.do(req)
	var answer struct {
		Target string `json:"target"`
		Extra  []struct {
			Target string          `json:"target"`
			Body   json.RawMessage `json:"body"`
		} `json:"extra"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil || answer.Target != "main" {
		t.Fatalf("the book's answer: %v %.300s", err, rec.Body.String())
	}
	for _, x := range answer.Extra {
		if x.Target != "rail" {
			continue
		}
		var page struct {
			Rail view.Rail `json:"rail"`
		}
		if err := json.Unmarshal(x.Body, &page); err != nil {
			t.Fatal(err)
		}
		lit := 0
		for _, s := range page.Rail.Sections {
			for _, r := range s.Rows {
				if r.On {
					lit++
					if string(r.Label) != "People" {
						t.Errorf("the rail lights %q, not the book opened", r.Label)
					}
				}
			}
		}
		if lit != 1 {
			t.Errorf("rows lit = %d, want the book's alone", lit)
		}
		return
	}
	t.Errorf("the book's answer brings no rail: %.300s", rec.Body.String())
}
