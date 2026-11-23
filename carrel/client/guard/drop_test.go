package guard_test

import (
	"reflect"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"carrel.dev/carrel/client/document"
	"carrel.dev/carrel/client/guard"
	"carrel.dev/carrel/client/screens"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// A page's drop sheet (Typecase's notes 60, 136) has no frame on the
// screen: the frames draw it alone (s-io-1, -20…-23), and the document puts
// it, closed, among the page's own sheets. A page carries one where a file
// may be dropped on it — a list that names its import (ListBar.Drop), a
// record's page that names its drop (AttachDrop), an edit that picks a file
// from this computer — and none elsewhere; the
// sheet names a form of the page that has a file field. Opened, it is the
// sheet of its frame: the list's s-io-1, a record's page s-io-20, an edit's
// s-io-21, a card's photo s-io-22.
func TestDropSheets(t *testing.T) {
	l := load(t)
	took := map[string]int{}
	plain := 0
	for _, scr := range screens.All {
		for _, path := range l.fixtures(scr) {
			model, err := decode(scr, path)
			if err != nil {
				t.Fatalf("%s on %s: %v", label(scr), key(path), err)
			}
			at := label(scr) + " on " + key(path)
			doc := document.Of(scr, model, "")
			printed := doc.Root.Render()
			for _, f := range doc.Forms {
				printed += f.Render()
			}
			page := parseHTML(printed)
			sheets := findAll(page, func(n *html.Node) bool { return hasClass(n, "tc-scrim") && attrOf(n, "data-tc-drop") != "" })
			frame, want := dropFrame(model, page)
			if _, shell := doc.Root.(w.Shell); !shell {
				// A page outside the shell — the printed one — has no sheets.
				want = false
			}
			// A picture pasted into the text of an edit (s-io-21) becomes
			// one of its attachments, as a file dropped there (Typecase's
			// task 88, Area.Attach; the keys' sheet says «Paste an image as
			// an attachment»): each text area of the form the drop sheet
			// sends names that form. Nowhere else does a pasted picture land
			// from a text — a card's would become its photo. The page under
			// a sheet (inert) takes nothing.
			paste := ""
			if want && frame == "s-io-21" && len(sheets) == 1 {
				paste = attrOf(sheets[0], "data-tc-drop")
			}
			for _, a := range findAll(page, func(n *html.Node) bool { return n.Data == "textarea" }) {
				if inert(a) {
					continue
				}
				got := attrOf(a, "data-tc-paste")
				switch {
				case paste != "" && inForm(a, paste) && got != paste:
					t.Errorf("%s: the text area %q takes no pasted picture into %q, the form a dropped file goes to", at, attrOf(a, "name"), paste)
				case got != "" && (paste == "" || got != paste):
					t.Errorf("%s: the text area %q sends a pasted picture to %q, and no drop of an attachment goes there", at, attrOf(a, "name"), got)
				}
			}
			switch {
			case want && len(sheets) != 1:
				t.Errorf("%s: a file may be dropped here, and the page has %d drop sheets", at, len(sheets))
				continue
			case !want && len(sheets) != 0:
				t.Errorf("%s: a drop sheet where the page takes no file", at)
				continue
			case !want:
				plain++
				continue
			}
			id := attrOf(sheets[0], "data-tc-drop")
			if !takesFile(page, id) {
				t.Errorf("%s: the drop sheet names %q, and the page has no form of that ID with a file field", at, id)
			}
			if !hasAttr(sheets[0], "hidden") {
				t.Errorf("%s: the drop sheet stands open", at)
			}
			open := strings.Replace(outer(sheets[0]), " hidden", "", 1)
			if sheet, _, ok := scr.Drop(model); ok {
				none(t, at+": the drop sheet's meaning", guard.Meaning(w.Shell{Dialogs: []w.Scrim{sheet}}))
			}
			none(t, at+": the drop sheet's classes", classFindings(l, at, open))
			// The combined view's line names no collection — «where it goes
			// is asked next»: s-io-1's caption says so in words, its drawing
			// is of one collection, named in bold.
			if d := importDropOf(model); d != nil && !named(d.Line) {
				continue
			}
			if m, ok := model.(*view.ImportPage); ok && !named(m.Source.Drop.Line) {
				continue
			}
			took[frame]++
			parts := scrimOf(l.mock, frame)
			if fragment(l.mock, frame) {
				parts = sheetParts(l.mock, frame)
			}
			screen := parseHTML(open)
			if d := importDropOf(model); frame == "s-io-1" && (d == nil || len(d.Accept) != 1 || d.Accept[0] != ".vcf") {
				// s-io-1 draws the contacts' refusals (Typecase's note 209):
				// another kind refuses other files, as many as its own. Each
				// still names its case.
				for _, r := range findAll(screen, func(n *html.Node) bool { return hasClass(n, "tc-empty__refusal") }) {
					if attrOf(r, "data-tc-refusal") == "" {
						t.Errorf("%s: a refusal of the drop sheet names no case", at)
					}
				}
				screen, parts = withoutRefusals(screen), withoutRefusals(parts)
			}
			none(t, at+": the drop sheet against "+frame, partFindings(l, frame, at, parts, screen))
		}
	}
	for _, f := range []string{"s-io-1", "s-io-20", "s-io-21", "s-io-22", "s-io-23", "s-io-25"} {
		if took[f] == 0 {
			t.Errorf("no page's drop sheet is held against %s: the samples moved", f)
		}
	}
	if plain == 0 {
		t.Errorf("every page takes a drop: the samples moved")
	}
}

// dropFrame is the frame of the drop sheet a page of this model draws, and
// whether one is wanted: a list's import (s-io-1); the page of an event, a
// task or a note that names its drop (s-io-20); an edit of one that picks a
// file (s-io-21); a card's photo (s-io-22); the restore's archive (s-io-23);
// a folder's upload (s-io-25).
func dropFrame(model any, page []*html.Node) (string, bool) {
	if d := importDropOf(model); d != nil {
		return "s-io-1", true
	}
	picks := find(page, func(n *html.Node) bool { return hasClass(n, "tc-file") }) != nil
	switch m := model.(type) {
	case *view.EventEditPage, *view.TaskEditPage:
		return "s-io-21", picks
	case *view.NotePage:
		if m.Doc.Mode != view.NoteEditing {
			return "s-io-20", m.Drop != nil
		}
		return "s-io-21", picks
	case *view.EventPage:
		return "s-io-20", m.Drop != nil
	case *view.TaskPage:
		return "s-io-20", m.Drop != nil
	case *view.ContactEditPage:
		return "s-io-22", picks
	case *view.ImportPage:
		// The wizard's first step (Typecase's note 209): the sheet of a
		// list's import, its file into the dialog's field.
		return "s-io-1", m.Source.Drop != nil && m.Step != view.ImportStepPreview && m.Step != view.ImportStepReport && picks
	case *view.BackupPage:
		// The restore's first step, an archive from this computer.
		return "s-io-23", m.Restore.Open && m.Restore.Match == nil && m.Restore.Report == nil && picks
	case *view.FilesPage:
		// A folder that takes an upload, empty or not.
		for _, a := range m.Bar.Actions {
			if a.ID == "file.upload" && a.Enabled && a.Method == "POST" {
				return "s-io-25", !m.Print.Paper
			}
		}
	}
	return "", false
}

// withoutRefusals are nodes with the drop sheet's refusals taken out.
func withoutRefusals(nodes []*html.Node) []*html.Node {
	for _, n := range nodes {
		for _, r := range findAll([]*html.Node{n}, func(n *html.Node) bool { return hasClass(n, "tc-empty__refusal") }) {
			if r.Parent != nil {
				r.Parent.RemoveChild(r)
			}
		}
	}
	return nodes
}

// takesFile says whether the page has a form of the ID with a file field,
// in it or naming it (form=).
func takesFile(page []*html.Node, id string) bool {
	form := find(page, func(n *html.Node) bool { return n.Data == "form" && attrOf(n, "id") == id })
	if form == nil {
		return false
	}
	file := func(n *html.Node) bool { return n.Data == "input" && attrOf(n, "type") == "file" }
	return find([]*html.Node{form}, file) != nil ||
		find(page, func(n *html.Node) bool { return file(n) && attrOf(n, "form") == id }) != nil
}

// inForm says whether a field belongs to the form of the ID: inside it, or
// naming it (form=).
func inForm(field *html.Node, id string) bool {
	if attrOf(field, "form") != "" {
		return attrOf(field, "form") == id
	}
	for n := field.Parent; n != nil; n = n.Parent {
		if n.Type == html.ElementNode && n.Data == "form" {
			return attrOf(n, "id") == id
		}
	}
	return false
}

// inert says whether a node stands in a part of the page a sheet holds
// still (inert): the page under it.
func inert(n *html.Node) bool {
	for ; n != nil; n = n.Parent {
		if n.Type == html.ElementNode && hasAttr(n, "inert") {
			return true
		}
	}
	return false
}

// importDropOf is the import a list's bar names (ListBar.Drop), wherever the
// model holds its bar; nil when none does.
func importDropOf(model any) *view.ImportDrop {
	v := reflect.ValueOf(model)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	f := v.FieldByName("Bar")
	if !f.IsValid() {
		return nil
	}
	if b, ok := f.Interface().(view.ListBar); ok {
		return b.Drop
	}
	return nil
}

// named says whether a line names its collection in bold.
func named(r view.Runs) bool {
	for _, p := range r {
		if p.Mark == view.RunStrong {
			return true
		}
	}
	return false
}

// scrimOf is the scrim a frame draws over its page.
func scrimOf(mock, frame string) []*html.Node {
	if n := find(parseHTML(frameBlock(mock, frame)), func(n *html.Node) bool { return hasClass(n, "tc-scrim") }); n != nil {
		return []*html.Node{n}
	}
	return nil
}

func attrOf(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

func hasAttr(n *html.Node, name string) bool {
	for _, a := range n.Attr {
		if a.Key == name {
			return true
		}
	}
	return false
}
