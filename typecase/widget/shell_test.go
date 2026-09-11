package widget_test

import (
	"strings"
	"testing"

	"carrel.dev/typecase/widget"
)

func TestShell_omitsEmptyColumns(t *testing.T) {
	html := widget.Shell{
		Top: widget.Top{Brand: widget.Brand{Mark: "◆", Word: "Carrel"}},
		Main: widget.Main{
			Label: "Edit",
			Body:  []widget.Node{widget.Paragraph("body")},
		},
	}.Render()
	if strings.Contains(html, "tc-shell__rail") {
		t.Fatal("empty shell must not emit rail column")
	}
	if strings.Contains(html, "tc-shell__side") {
		t.Fatal("empty shell must not emit side column")
	}
	if !strings.Contains(html, "tc-shell__main") {
		t.Fatal("shell must still emit main column")
	}
}

func TestShell_keepsPopulatedColumns(t *testing.T) {
	html := widget.Shell{
		Top:  widget.Top{Brand: widget.Brand{Mark: "◆", Word: "Carrel"}},
		Nav:  widget.Nav{Items: []widget.NavItem{{Label: "Notes", Href: "/notes"}}},
		Rail: widget.Rail{Sections: []widget.RailSection{{Rows: []widget.TickRow{{Label: "All"}}}}},
		Main: widget.Main{Label: "Notes", Body: []widget.Node{widget.Paragraph("list")}},
		Side: widget.Side{Label: "Details", Title: "Note", Open: true},
	}.Render()
	for _, zone := range []string{"tc-shell__rail", "tc-shell__main", "tc-shell__side"} {
		if !strings.Contains(html, zone) {
			t.Fatalf("populated shell missing %s", zone)
		}
	}
}
