package zone_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/client/fixtures"
	"carrel.dev/carrel/client/zone"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// The spec and the fixtures, from carrel/client/zone up to the repository
// root.
const (
	commonSpec = "../../../docs/spec/carrel-common.md"
	fixtureDir = "../../../docs/spec/fixtures"
)

// routeKinds reads C-5: every view model a route answers as a page or a
// sheet, and which of them it is. A page of C-5.1 or C-5.2 is outside the
// shell. The spec is read, not the Go types, so that the kinds zone.KindOf
// gives are checked against something it did not derive them from.
func routeKinds(t *testing.T) map[string]zone.Kind {
	t.Helper()
	spec := readSpec(t)
	c5 := section(t, spec, "## C-5. ", "## C-6. ")
	ident := regexp.MustCompile("`([A-Z][A-Za-z]*)`")
	leading := regexp.MustCompile("^\\s*`[A-Z][A-Za-z]*`(,\\s*`[A-Z][A-Za-z]*`)*")
	kinds := map[string]zone.Kind{}
	var fragments []string
	sub := ""
	for _, line := range strings.Split(c5, "\n") {
		if strings.HasPrefix(line, "### ") {
			sub = strings.Fields(line)[1]
			continue
		}
		if !strings.HasPrefix(line, "| `/") {
			continue
		}
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		if len(cells) != 5 {
			t.Fatalf("C-5 row of %d cells: %s", len(cells), line)
		}
		answer, models := cells[3], ident.FindAllStringSubmatch(cells[4], -1)
		switch {
		case strings.Contains(answer, "лист") && len(models) > 0:
			// The sheets are the models the row opens with, one or a list
			// (/app/mail/send); a later one is what the POST answers with
			// (RewriteSheet → ContactPage).
			for _, m := range ident.FindAllStringSubmatch(leading.FindString(cells[4]), -1) {
				kinds[m[1]] = zone.Sheet
			}
		case strings.Contains(answer, "страница"):
			for _, m := range models {
				kind := zone.ShellPage
				if sub == "C-5.1" || sub == "C-5.2" {
					kind = zone.PlainPage
				}
				if prev, ok := kinds[m[1]]; ok && prev != kind {
					t.Fatalf("C-5 names %s both %v and %v", m[1], prev, kind)
				}
				kinds[m[1]] = kind
			}
		case (strings.Contains(answer, "фрагмент") || strings.Contains(answer, "меню")) && len(models) > 0:
			// A menu printed alone — the search line's suggestions
			// (SearchSuggest, Typecase's note 118) — is no page either.
			fragments = append(fragments, models[0][1])
		}
	}
	if len(kinds) == 0 {
		t.Fatal("C-5: no page or sheet rows read")
	}
	// A view model C-5 answers only as a fragment of its own — the
	// suggestions of a field (SuggestList, /app/contacts/suggest) — is no
	// page: it has no zones, the live layer puts it where the markup says.
	for _, f := range fragments {
		if _, ok := kinds[f]; !ok {
			kinds[f] = zone.NotPage
		}
	}
	return kinds
}

// zoneTable reads the table «Где зона во вью-модели» of C-7.2: for each
// target, the cell of a page in the shell, and the view models it names
// after «только у».
func zoneTable(t *testing.T) map[string][]string {
	t.Helper()
	c72 := section(t, readSpec(t), "### C-7.2 ", "### C-7.3 ")
	at := strings.Index(c72, "**Где зона во вью-модели.**")
	if at < 0 {
		t.Fatal("C-7.2 has no «Где зона во вью-модели»")
	}
	only := map[string][]string{}
	rows := 0
	ident := regexp.MustCompile("`([A-Z][A-Za-z]*)`")
	for _, line := range strings.Split(c72[at:], "\n") {
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		target := strings.Trim(strings.TrimSpace(cells[0]), "`")
		rows++
		cell := cells[1]
		if i := strings.Index(cell, "только у"); i >= 0 {
			for _, m := range ident.FindAllStringSubmatch(cell[i:], -1) {
				if m[1] != "Card" {
					only[target] = append(only[target], m[1])
				}
			}
		}
	}
	if rows != len(render.Targets()) {
		t.Fatalf("C-7.2 zone table: %d rows, C-7.2 has %d targets", rows, len(render.Targets()))
	}
	return only
}

func readSpec(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(commonSpec)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(raw), "\r\n", "\n")
}

func section(t *testing.T, text, from, to string) string {
	t.Helper()
	i := strings.Index(text, from)
	j := strings.Index(text, to)
	if i < 0 || j < i {
		t.Fatalf("no section %q … %q", from, to)
	}
	return text[i:j]
}

func typeName(model any) string {
	tp := reflect.TypeOf(model)
	if tp.Kind() == reflect.Pointer {
		tp = tp.Elem()
	}
	return tp.Name()
}

// The sheets zone knows are exactly the sheets of C-5.
func TestSheetsAreTheSheetsOfC5(t *testing.T) {
	var fromSpec, known []string
	for name, kind := range routeKinds(t) {
		if kind == zone.Sheet {
			fromSpec = append(fromSpec, name)
		}
	}
	for _, m := range allSheets() {
		if zone.KindOf(m) == zone.Sheet {
			known = append(known, typeName(m))
		}
	}
	slices.Sort(fromSpec)
	slices.Sort(known)
	if !slices.Equal(fromSpec, known) {
		t.Errorf("sheets of C-5: %v\nsheets zone knows: %v", fromSpec, known)
	}
}

// allSheets is every view model whose name says sheet, so the check above
// sees a sheet zone forgot as well as one it invented.
func allSheets() []any {
	return []any{
		view.PublishSheet{}, view.MailSheet{}, view.MailAttachSheet{}, view.MailLinkSheet{}, view.MailPreviewSheet{}, view.MailCloseSheet{}, view.MailSentSheet{}, view.MoveSheet{}, view.CopiesSheet{},
		view.AskSheet{}, view.RecordDeleteSheet{},
		view.RewriteSheet{}, view.ExportSheet{}, view.NewSheet{}, view.FileOpSheet{},
		view.FilePickerSheet{}, view.CollectionNewSheet{}, view.CollectionRenameSheet{},
		view.CollectionMoveSheet{}, view.CollectionDeleteSheet{}, view.UnlockSheet{},
		view.EscrowWithdrawSheet{}, view.ConnectionRemoveSheet{}, view.PasswordResetSheet{}, view.AccountDeleteSheet{},
		view.BackupTriggerSheet{},
		view.DavloomScopeSheet{},
		view.DavloomTreeSheet{},
	}
}

// shellKeys are the JSON keys the embedded Shell puts on a page.
func shellKeys(t *testing.T) []string {
	raw, err := json.Marshal(view.Shell{})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func asJSON(t *testing.T, v any) any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// Every fixture on disk, decoded as its view model, yields the zones C-7.2
// gives its kind — each equal to the part of the fixture it names — and an
// explicit ErrNoZone for every target its kind lacks.
func TestEveryFixtureHasItsZones(t *testing.T) {
	kinds := routeKinds(t)
	only := zoneTable(t)
	shell := shellKeys(t)
	screens, err := os.ReadDir(fixtureDir)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, screen := range screens {
		if !screen.IsDir() {
			continue
		}
		newModel, ok := fixtures.Models[screen.Name()]
		if !ok {
			t.Errorf("fixture screen %q has no view model in fixtures.Models", screen.Name())
			continue
		}
		states, _ := filepath.Glob(filepath.Join(fixtureDir, screen.Name(), "*.json"))
		for _, path := range states {
			seen++
			t.Run(screen.Name()+"/"+filepath.Base(path), func(t *testing.T) {
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				model := newModel()
				dec := json.NewDecoder(bytes.NewReader(raw))
				dec.DisallowUnknownFields()
				if err := dec.Decode(model); err != nil {
					t.Fatalf("decode into %T: %v", model, err)
				}
				var fixture map[string]any
				if err := json.Unmarshal(raw, &fixture); err != nil {
					t.Fatal(err)
				}
				name := typeName(model)
				kind, ok := kinds[name]
				if !ok {
					t.Fatalf("%s answers no route of C-5", name)
				}
				if got := zone.KindOf(model); got != kind {
					t.Fatalf("%s: zone says %v, C-5 says %v", name, got, kind)
				}
				want := map[string]any{} // target → the fixture's part
				switch kind {
				case zone.Sheet:
					want[render.TargetSheet] = fixture
				case zone.PlainPage:
					want[render.TargetMain] = fixture
				case zone.ShellPage:
					main := map[string]any{}
					for k, v := range fixture {
						if !slices.Contains(shell, k) {
							main[k] = v
						}
					}
					want[render.TargetMain] = main
					want[render.TargetRail] = fixture["rail"]
					want[render.TargetNotices] = fixture["notices"]
					want[render.TargetTopbar] = fixture["top"]
					want[render.TargetDetails] = fixture["details"]
					if slices.Contains(only[render.TargetDetails], name) {
						want[render.TargetDetails] = fixture["card"]
					}
					if slices.Contains(only[render.TargetPoll], name) {
						want[render.TargetPoll] = fixture["poll"]
					}
					if slices.Contains(only[render.TargetQueue], name) {
						want[render.TargetQueue] = fixture["queue"]
					}
				}
				for _, target := range render.Targets() {
					got, err := zone.Of(model, target)
					if kind == zone.NotPage {
						if !errors.Is(err, zone.ErrNotPage) {
							t.Errorf("%s answers only as a fragment, and target %s is %v, %v", name, target, got, err)
						}
						continue
					}
					w, has := want[target]
					if !has {
						if !errors.Is(err, zone.ErrNoZone) {
							t.Errorf("%s: target %s is not on this page, got %v, %v", name, target, got, err)
						}
						continue
					}
					if err != nil {
						t.Errorf("%s: target %s: %v", name, target, err)
						continue
					}
					g := asJSON(t, got)
					if target == render.TargetMain && kind == zone.ShellPage {
						if !reflect.ValueOf(got).Elem().FieldByName("Shell").IsZero() {
							t.Errorf("%s: main carries the shell", name)
						}
						for _, k := range shell {
							delete(g.(map[string]any), k)
						}
					}
					if !reflect.DeepEqual(g, w) {
						t.Errorf("%s: target %s is not the fixture's part:\n got %v\nwant %v", name, target, g, w)
					}
				}
			})
		}
	}
	if seen == 0 {
		t.Fatal("no fixtures under " + fixtureDir)
	}
}

// Main is a copy: cutting it leaves the page as it was, shell and all.
func TestMainLeavesThePage(t *testing.T) {
	page := &view.AgendaPage{Shell: view.Shell{Title: "Calendar"}}
	got, err := zone.Of(page, render.TargetMain)
	if err != nil {
		t.Fatal(err)
	}
	if got.(*view.AgendaPage) == page || page.Title != "Calendar" {
		t.Error("main changed the page it was cut from")
	}
	byValue, err := zone.Of(*page, render.TargetMain)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := byValue.(view.AgendaPage); !ok {
		t.Errorf("main of a value is %T, want view.AgendaPage", byValue)
	}
}

// No fixture shows a folder, so the queue is held here: a folder with
// transfers gives them, one without gives an empty queue, and no other page
// has a queue at all.
func TestQueueIsTheFolders(t *testing.T) {
	q := &view.TransferQueue{Note: "2 of 3"}
	got, err := zone.Of(&view.FilesPage{Queue: q}, render.TargetQueue)
	if err != nil || got.(*view.TransferQueue) != q {
		t.Errorf("folder with transfers: %v, %v", got, err)
	}
	got, err = zone.Of(&view.FilesPage{}, render.TargetQueue)
	if err != nil || got.(*view.TransferQueue) != nil {
		t.Errorf("folder without transfers: %v, %v", got, err)
	}
	if _, err := zone.Of(&view.FilesRootPage{}, render.TargetQueue); !errors.Is(err, zone.ErrNoZone) {
		t.Errorf("files root has a queue: %v", err)
	}
}

// A name outside C-7.2 is not a target, and a part already cut, or no view
// model at all, is not a page.
func TestWhatIsNotAZone(t *testing.T) {
	if _, err := zone.Of(&view.AgendaPage{}, "sidebar"); !errors.Is(err, zone.ErrUnknownTarget) {
		t.Errorf("unknown target: %v", err)
	}
	for _, m := range []any{view.Rail{}, &view.PollBar{}, []view.Notice{}, nil, (*view.AgendaPage)(nil), "main"} {
		if _, err := zone.Of(m, render.TargetMain); !errors.Is(err, zone.ErrNotPage) {
			t.Errorf("%T: %v", m, err)
		}
	}
}

func TestTargetsInOrder(t *testing.T) {
	got := zone.Targets(&view.SearchPage{})
	want := []string{"main", "rail", "details", "notices", "poll", "topbar"}
	if !slices.Equal(got, want) {
		t.Errorf("search page targets %v, want %v", got, want)
	}
	if got := zone.Targets(&view.LoginPage{}); !slices.Equal(got, []string{"main"}) {
		t.Errorf("login targets %v", got)
	}
	if got := zone.Targets(&view.MoveSheet{}); !slices.Equal(got, []string{"sheet"}) {
		t.Errorf("sheet targets %v", got)
	}
}

// A page keeps the place of its poll when none runs: the zone is carried
// whether the bar stands or not; a page without a poll does not carry it.
func TestCarries(t *testing.T) {
	if !zone.Carries(&view.AgendaPage{}, render.TargetPoll) {
		t.Error("an agenda without a running poll carries no poll")
	}
	if !zone.Carries(&view.ContactTimelinePage{}, render.TargetPoll) {
		t.Error("the timeline carries no poll")
	}
	if zone.Carries(&view.ContactPage{}, render.TargetPoll) {
		t.Error("a card carries a poll")
	}
	if zone.Carries(&view.LoginPage{}, render.TargetPoll) || !zone.Carries(&view.LoginPage{}, render.TargetMain) {
		t.Error("a page outside the shell: main only")
	}
}
