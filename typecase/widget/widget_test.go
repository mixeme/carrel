package widget_test

import (
	"strings"
	"testing"

	"carrel.dev/typecase/catalog"
	"carrel.dev/typecase/css"
	"carrel.dev/typecase/token"
	"carrel.dev/typecase/widget"
)

func intPtr(n int) *int { return &n }

func TestRender_smoke(t *testing.T) {
	cases := []struct {
		name string
		html string
	}{
		{"btn", widget.Button{Label: "Go"}.Render()},
		{"input", widget.Input{Name: "x", Kind: widget.InputText}.Render()},
		{"check", widget.Check{Name: "c", Label: "Check"}.Render()},
		{"kbd", widget.Kbd{Keys: []string{"Esc"}}.Render()},
		{"close", widget.Close{For: "x", Label: "Close"}.Render()},
		{"brand", widget.Brand{Mark: "T", Word: "Test"}.Render()},
		{"mark", widget.Mark{Colour: "#000", Label: "A"}.Render()},
		{"num", widget.Num{Value: intPtr(1)}.Render()},
		{"badge", widget.Badge{Label: "New"}.Render()},
		{"avatar", widget.Avatar{Initials: "AB"}.Render()},
		{"tick", widget.Tick{Label: "Done"}.Render()},
		{"tag", widget.Tag{Label: "Tag"}.Render()},
		{"msg", widget.Msg{Title: "Hi", Text: "There"}.Render()},
		{"empty", widget.Empty{Title: "Empty", Text: "None", Action: widget.Action{Label: "Add"}}.Render()},
		{"skel", widget.Skel{Lines: 2}.Render()},
		{"meter", widget.Meter{Value: 1, Max: 10, Label: "1/10"}.Render()},
		{"form", widget.Form{Action: "/f", Sets: []widget.FieldSet{{Legend: "L", Rows: []widget.Field{{Name: "n", Label: "N", Kind: widget.InputText}}}}}.Render()},
		{"menu", widget.Menu{Label: "Menu", Items: []widget.MenuItem{{Label: "One", Href: "#"}}}.Render()},
		{"bar", widget.Bar{Left: []widget.Node{widget.Action{Label: "A"}}}.Render()},
		{"list", widget.List{Groups: []widget.ListGroup{{Rows: []widget.Row{{Cells: widget.RowCells{Title: "T"}}}}}}.Render()},
		{"table", widget.Table{Columns: []widget.Column{{Label: "C"}}, Rows: []widget.TableRow{{Cells: []string{"v"}}}}.Render()},
		{"shell", widget.Shell{Main: widget.Main{Label: "M", Body: []widget.Node{widget.Paragraph("b")}}}.Render()},
		{"doc", widget.Doc{Body: widget.Paragraph("x")}.Render()},
		{"search", widget.Search{Action: "/s"}.Render()},
		{"stat", widget.Stat{Label: "Users", Value: "1,234"}.Render()},
		{"stats", widget.Stats{Items: []widget.Stat{{Label: "Users", Value: "1,234"}}}.Render()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.html == "" {
				t.Fatal("empty render")
			}
			if strings.Contains(tc.html, "<script") {
				t.Fatal("inline script in output")
			}
		})
	}
}

func TestRowAvatar_fallbackNoImageSrc(t *testing.T) {
	html := widget.Row{
		Variant: widget.RowAvatar,
		Cells:   widget.RowCells{Title: "Alex", Subtitle: "alex@example.com"},
	}.Render()
	if strings.Contains(html, "<img") {
		t.Fatalf("avatar fallback must not emit img: %s", html)
	}
	if !strings.Contains(html, "tc-avatar__initials") {
		t.Fatalf("avatar fallback should use initials: %s", html)
	}
}

func TestCatalog_entries(t *testing.T) {
	entries := catalog.Entries()
	if len(entries) < 50 {
		t.Fatalf("expected comprehensive catalog, got %d entries", len(entries))
	}
	seen := make(map[string]struct{})
	for _, e := range entries {
		key := e.Widget + "/" + e.State
		if _, ok := seen[key]; ok {
			t.Fatalf("duplicate catalog entry %s", key)
		}
		seen[key] = struct{}{}
		if e.HTML == "" {
			t.Fatalf("empty HTML for %s", key)
		}
		if !strings.Contains(e.HTML, "tc-") {
			t.Fatalf("missing tc- class for %s", key)
		}
	}
}

func TestCSS_widgetTokenValidation(t *testing.T) {
	names := css.Order[2:]
	for _, name := range names {
		data, err := css.Files().Open(name)
		if err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, 1<<20)
		n, _ := data.Read(buf)
		_ = data.Close()
		if err := token.ValidateWidget(string(buf[:n])); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestEscape_html(t *testing.T) {
	html := widget.Button{Label: `<script>"&`}.Render()
	if strings.Contains(html, "<script>") {
		t.Fatal("unescaped HTML in label")
	}
}

// Пустой бейдж — ломаная фикстура ворот inv-badge-has-text. tc-usermenu отдавал
// её всякий раз, когда метка не задана (В-19).
func TestUserMenu_noEmptyBadge(t *testing.T) {
	html := widget.UserMenu{
		Name:  "Alex",
		Note:  "Signed in",
		Items: []widget.MenuItem{{Label: "Sign out", Href: "/logout"}},
	}.Render()
	if strings.Contains(html, `class="tc-badge`) {
		t.Fatalf("user menu without Mark must not render a badge: %s", html)
	}
	if strings.Contains(html, `<ul class="tc-usermenu__popover`) {
		t.Fatal("popover head must not sit inside the menu list")
	}
}

func TestBadge_emptyLabelRendersNothing(t *testing.T) {
	if got := (widget.Badge{}).Render(); got != "" {
		t.Fatalf("empty badge must render nothing, got %q", got)
	}
}

func TestRailSection_emptyTitleOmitsHeading(t *testing.T) {
	html := widget.RailSection{Rows: []widget.TickRow{{Label: "Combined notebook"}}}.Render()
	if strings.Contains(html, "tc-railsec__title") {
		t.Fatalf("empty title must omit heading: %s", html)
	}
	if !strings.Contains(html, "tc-railsec__rows") {
		t.Fatalf("rows must still render: %s", html)
	}
}

func TestRailSection_titledKeepsHeading(t *testing.T) {
	html := widget.RailSection{Title: "Views", Rows: []widget.TickRow{{Label: "All"}}}.Render()
	if !strings.Contains(html, `<h3 class="tc-railsec__title">Views</h3>`) {
		t.Fatalf("titled section must keep heading: %s", html)
	}
}

func TestRowExcerpt_dateAndSource(t *testing.T) {
	html := widget.Row{
		Variant: widget.RowExcerpt,
		Cells: widget.RowCells{
			Title:   "Note",
			Excerpt: "Body text",
			Date:    "10 Aug",
			Source:  "nextcloud · Notes",
		},
	}.Render()
	if !strings.Contains(html, `<span class="tc-row__date">10 Aug</span>`) {
		t.Fatalf("excerpt row must render date: %s", html)
	}
	if !strings.Contains(html, `<span class="tc-row__source">nextcloud · Notes</span>`) {
		t.Fatalf("excerpt row must render source: %s", html)
	}
}

func TestSideBlock_exactlyOne(t *testing.T) {
	sec := widget.Sec{Rubric: widget.Rubric{Text: "Info"}, Body: []widget.Node{widget.Paragraph("Details")}}
	field := widget.Field{Name: "title", Label: "Title", Kind: widget.InputText, Value: "Note"}
	fields := widget.Fields{Rows: []widget.FieldRow{{Label: "UID", Value: "42"}}}

	if got := (widget.SideBlock{Sec: &sec}).Render(); !strings.Contains(got, "tc-sec") {
		t.Fatalf("sec block: %q", got)
	}
	if got := (widget.SideBlock{Field: &field}).Render(); !strings.Contains(got, "tc-field") {
		t.Fatalf("field block: %q", got)
	}
	if got := (widget.SideBlock{Fields: &fields}).Render(); !strings.Contains(got, "tc-fields") {
		t.Fatalf("fields block: %q", got)
	}
	if got := (widget.SideBlock{}).Render(); got != "" {
		t.Fatalf("empty block must render nothing, got %q", got)
	}
	both := widget.SideBlock{Sec: &sec, Field: &field}
	if got := both.Render(); got != "" {
		t.Fatalf("two kinds must render nothing, got %q", got)
	}
}
