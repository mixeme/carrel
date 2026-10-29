package web_test

import (
	"encoding/json"
	"net/url"
	"testing"

	"carrel.dev/carrel/server/view"
)

func TestNoteReadingControls(t *testing.T) {
	f := calendarFixture(t, nil, "n.ics", ics("BEGIN:VJOURNAL\nUID:n\nDTSTAMP:20260101T000000Z\nSUMMARY:Note\nDESCRIPTION:# Title\\n<script>literal</script>\nEND:VJOURNAL"))
	path := f.path("notes") + "/n"
	for _, query := range []string{"", "?view=source&width=full&side=0", "?view=reading&width=reading&side=1"} {
		p := decode[view.NotePage](t, f.h.do(get(path+query, f.sess)))
		data, _ := json.Marshal(p.Doc)
		var doc struct {
			Source   bool
			HideSide bool
			Controls []view.Action
		}
		_ = json.Unmarshal(data, &doc)
		if len(doc.Controls) != 4 {
			t.Fatalf("reading controls missing: %s", data)
		}
		if doc.Source != (query == "?view=source&width=full&side=0") || doc.HideSide != doc.Source || p.Look.Wide != doc.Source {
			t.Fatalf("query %q: %+v, wide=%v", query, doc, p.Look.Wide)
		}
		if p.Doc.Body != "# Title\n<script>literal</script>" {
			t.Fatalf("source changed: %q", p.Doc.Body)
		}
		for _, a := range doc.Controls {
			if a.Href == "" || a.Method != "GET" || !a.Enabled {
				t.Fatalf("inert control: %+v", a)
			}
			if doc.Source {
				u, err := url.Parse(a.Href)
				if err != nil {
					t.Fatal(err)
				}
				q := u.Query()
				if a.ID != "note.source" && q.Get("view") != "source" {
					t.Errorf("lost source: %s", a.Href)
				}
				if a.ID != "note.side" && q.Get("side") != "0" {
					t.Errorf("lost side choice: %s", a.Href)
				}
				if a.ID != "note.width.read" && q.Get("width") != "full" {
					t.Errorf("lost width: %s", a.Href)
				}
			}
		}
	}
	p := decode[view.NotePage](t, f.h.do(wideNotes(get(path, f.sess))))
	if !p.Look.Wide || !p.Doc.Controls[1].On {
		t.Fatal("browser's default width was lost")
	}
	p = decode[view.NotePage](t, f.h.do(wideNotes(get(path+"?width=read", f.sess))))
	if p.Look.Wide || !p.Doc.Controls[2].On {
		t.Fatal("explicit reading width did not override the browser's default")
	}
}
