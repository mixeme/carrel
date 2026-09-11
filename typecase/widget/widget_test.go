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
		{"bar", widget.Bar{Left: []widget.Action{{Label: "A"}}}.Render()},
		{"list", widget.List{Groups: []widget.ListGroup{{Rows: []widget.Row{{Cells: widget.RowCells{Title: "T"}}}}}}.Render()},
		{"table", widget.Table{Columns: []widget.Column{{Label: "C"}}, Rows: []widget.TableRow{{Cells: []string{"v"}}}}.Render()},
		{"shell", widget.Shell{Main: widget.Main{Label: "M", Body: widget.Paragraph("b")}}.Render()},
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
