package view

import (
	"fmt"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/server/render"
)

// c5Row is a row of the tables of C-5: its routes, methods, answer and the
// cell of its view model.
type c5Row struct {
	routes  []string
	methods []string
	answer  string
	model   string
}

// c5Rows reads C-5 of the spec: every row of its tables.
func c5Rows(t *testing.T) []c5Row {
	t.Helper()
	src, err := os.ReadFile(specFile)
	if err != nil {
		t.Fatal(err)
	}
	spec := strings.ReplaceAll(string(src), "\r\n", "\n")
	i, j := strings.Index(spec, "\n## C-5. "), strings.Index(spec, "\n## C-6. ")
	if i < 0 || j < i {
		t.Fatal("no C-5 in the spec")
	}
	var rows []c5Row
	for _, l := range strings.Split(spec[i:j], "\n") {
		if !strings.HasPrefix(l, "| `/") {
			continue
		}
		cells := strings.Split(strings.Trim(l, "|"), " | ")
		if len(cells) < 5 {
			t.Fatalf("C-5 row of %d cells: %s", len(cells), l)
		}
		r := c5Row{answer: cells[3], model: strings.Join(cells[4:], " | ")}
		for _, route := range strings.Split(cells[0], ", ") {
			r.routes = append(r.routes, strings.Trim(strings.TrimSpace(route), "`"))
		}
		for _, m := range strings.Split(cells[1], ",") {
			r.methods = append(r.methods, strings.TrimSpace(m))
		}
		rows = append(rows, r)
	}
	if len(rows) < 50 {
		t.Fatalf("C-5: %d rows — the tables were not read", len(rows))
	}
	return rows
}

// c5Match: whether path is route of C-5, and how many of the route's
// segments are words — the more, the more particular the route ({kind} is
// one of the four kinds, {path…} the rest of the path).
func c5Match(route, path string) (int, bool) {
	rs := strings.Split(strings.TrimPrefix(route, "/"), "/")
	ps := strings.Split(strings.TrimPrefix(path, "/"), "/")
	words := 0
	for i, r := range rs {
		if strings.HasSuffix(r, "…}") {
			return words, i <= len(ps)
		}
		if i >= len(ps) {
			return 0, false
		}
		switch {
		case r == "{kind}":
			// The rail's form of files is a kind of /app/sources/ (C-5.3).
			if !slices.Contains([]string{"contacts", "calendar", "tasks", "notes"}, ps[i]) && !(ps[i] == "files" && strings.HasPrefix(route, "/app/sources/")) {
				return 0, false
			}
		case strings.HasPrefix(r, "{"):
			if ps[i] == "" {
				return 0, false
			}
		case r == ps[i]:
			words++
		default:
			return 0, false
		}
	}
	return words, len(rs) == len(ps)
}

// c5Of is the row of C-5 an address belongs to for a method, by the most
// particular route.
func c5Of(rows []c5Row, method, href string) (c5Row, string, bool) {
	u, err := url.Parse(href)
	if err != nil {
		return c5Row{}, "", false
	}
	best, found, route := -1, c5Row{}, ""
	for _, r := range rows {
		if !slices.Contains(r.methods, method) && !(method == "GET" && slices.Contains(r.methods, "DAV-методы")) {
			continue
		}
		for _, rt := range r.routes {
			if n, ok := c5Match(rt, u.Path); ok && n > best {
				best, found, route = n, r, rt
			}
		}
	}
	return found, route, best >= 0
}

var modelInCell = regexp.MustCompile("`([A-Z][A-Za-z]*)`")

// c5Target is the zone of C-7.2 C-5 gives the answer of a row for a
// method: a sheet is aimed at sheet, a fragment at the zone of its view
// model, anything else is a move. An answer that differs by method says so
// in parts: «лист (GET); страница, редирект (POST)».
func c5Target(r c5Row, method string) string {
	answer := r.answer
	for _, part := range strings.Split(r.answer, ";") {
		// A part that says the answer of one method, and nothing else.
		if part = strings.TrimSpace(part); strings.HasSuffix(part, "("+method+")") && !strings.Contains(part, "`") {
			answer = part
		}
	}
	switch {
	case strings.Contains(answer, "лист"):
		return render.TargetSheet
	case strings.Contains(answer, "фрагмент"):
		m := modelInCell.FindStringSubmatch(r.model)
		if m == nil {
			return "?"
		}
		return map[string]string{"Rail": render.TargetRail, "PollBar": render.TargetPoll, "TransferQueue": render.TargetQueue, "FilesPage": render.TargetMain}[m[1]]
	}
	return ""
}

// C-7.1, C-7.2 (stage 4 of the client's plan, С1): the client cannot ask
// for a fragment where the frame draws a move, and does not guess which is
// which — every action and form says the zone its answer replaces, and C-5
// is what says it. Every action and form of every fixture is held against
// C-5 read from the spec: its address is a row of C-5, its Target is a zone
// of C-7.2, and it is the zone the row's answer gives — sheet for a sheet,
// the zone of the view model for a fragment, empty for a page, a redirect,
// a file. A button of a form (no Href) has none: the form has it. A row of
// a list that opens in the panel is aimed at details and leads to a
// record's card (C-5.4), where the panel is cut from (C-7.2).
func TestFixtureTargetsAreC5(t *testing.T) {
	rows := c5Rows(t)
	zones := render.Targets()
	check := func(screen, at, method, href, target string) {
		if target != "" && !slices.Contains(zones, target) {
			t.Errorf("%s %s: target %q is no zone of C-7.2", screen, at, target)
		}
		if href == "" || strings.Contains(href, "://") || strings.HasPrefix(href, "mailto:") || strings.HasPrefix(href, "tel:") {
			if target != "" {
				t.Errorf("%s %s: %q aims at %q without an address of Carrel", screen, at, href, target)
			}
			return
		}
		row, route, ok := c5Of(rows, method, href)
		if !ok {
			t.Errorf("%s %s: %s %s is no address of C-5", screen, at, method, href)
			return
		}
		if want := c5Target(row, method); target != want {
			t.Errorf("%s %s: %s %s (C-5 %s, «%s») aims at %q, want %q", screen, at, method, href, route, row.answer, target, want)
		}
	}
	actions, forms, rowsSeen := 0, 0, 0
	eachFixtureAction(t, func(screen, at string, a Action) {
		if a.ID == "" {
			return
		}
		actions++
		check(screen, at, a.Method, a.Href, a.Target)
	})
	eachFixtureValue(t, reflect.TypeOf(Form{}), func(screen, at string, v reflect.Value) {
		f := v.Interface().(Form)
		if f.Action == "" {
			return
		}
		forms++
		check(screen, at+" (form)", strings.ToUpper(f.Method), f.Action, f.Target)
	})
	record := func(screen, at, href, target string) {
		rowsSeen++
		// A row of a public page opens its record in the same link, as a
		// page of its own (Typecase's note 122 p. 2): no panel there.
		if u, err := url.Parse(href); err == nil && u.Query().Get("uid") != "" {
			if _, route, ok := c5Of(rows, "GET", u.Path); ok && route == "/p/{token}" {
				if target != "" {
					t.Errorf("%s %s: a public row aims at %q, want the page", screen, at, target)
				}
				return
			}
		}
		if target != render.TargetDetails {
			t.Errorf("%s %s: a row of a list aims at %q, want details", screen, at, target)
		}
		// With «Keep the details panel open» a row leads to its own list
		// with the record in ?open= (Typecase's note 75, 186 p. 3): the
		// list is an address of C-5, and what it opens is a record's card.
		if u, err := url.Parse(href); err == nil && u.Query().Get("open") != "" {
			if _, _, ok := c5Of(rows, "GET", u.Path); !ok {
				t.Errorf("%s %s: a row opening in the panel leads to %s, no list of C-5", screen, at, u.Path)
			}
			href = u.Query().Get("open")
		}
		_, route, ok := c5Of(rows, "GET", href)
		if !ok || route != "/app/{kind}/{account}/{col}/{uid}" {
			t.Errorf("%s %s: a row opening in the panel leads to %s (%s), not to a record's card", screen, at, href, route)
		}
	}
	for _, typ := range []reflect.Type{reflect.TypeOf(ContactRow{}), reflect.TypeOf(AgendaRow{}), reflect.TypeOf(TaskRow{}), reflect.TypeOf(NoteRow{})} {
		eachFixtureValue(t, typ, func(screen, at string, v reflect.Value) {
			href, target := v.FieldByName("Href").String(), v.FieldByName("Target").String()
			if href == "" {
				if target != "" {
					t.Errorf("%s %s: a row without an address aims at %q", screen, at, target)
				}
				return
			}
			record(screen, at, href, target)
		})
	}
	// A row of a person's timeline opens its card in the panel
	// (s-contacts-7); a related record of a card or a panel is a move.
	eachFixtureValue(t, reflect.TypeOf(TimelineRow{}), func(screen, at string, v reflect.Value) {
		r := v.Interface().(TimelineRow)
		_, route, _ := c5Of(rows, "GET", r.Href)
		if screen == "app-contact-timeline" && strings.Contains(at, ".Rows[") && route == "/app/{kind}/{account}/{col}/{uid}" {
			record(screen, at, r.Href, r.Target)
		} else if r.Target != "" {
			t.Errorf("%s %s: a related record aims at %q: it is a move", screen, at, r.Target)
		}
	})
	if actions < 100 || forms < 10 || rowsSeen < 10 {
		t.Fatalf("%d actions, %d forms, %d rows: the fixtures were not walked", actions, forms, rowsSeen)
	}
	t.Log(fmt.Sprintf("%d actions, %d forms, %d rows held against C-5", actions, forms, rowsSeen))
}

// The owner's decision 4 of the client's plan (stage 4, С5): Print is the
// same address with ?print=1 — render.PrintParam and render.PrintValue —,
// the same route and view model, drawn on paper. Which addresses take it is
// C-5's to say: every Print of every fixture leads to its address with the
// parameter, and the row of C-5 that address belongs to names `?print=1`.
func TestFixturePrintsAreC5(t *testing.T) {
	rows := c5Rows(t)
	prints := 0
	eachFixtureAction(t, func(screen, at string, a Action) {
		if !strings.HasSuffix(a.ID, ".print") {
			return
		}
		prints++
		u, err := url.Parse(a.Href)
		if err != nil || u.Query().Get(render.PrintParam) != render.PrintValue || a.Method != "GET" {
			t.Errorf("%s %s: Print is %s %s, want GET with %s=%s", screen, at, a.Method, a.Href, render.PrintParam, render.PrintValue)
			return
		}
		row, route, ok := c5Of(rows, "GET", a.Href)
		if want := "`?" + render.PrintParam + "=" + render.PrintValue + "`"; !ok || !strings.Contains(row.model, want) {
			t.Errorf("%s %s: %s leads to %s, whose row of C-5 does not name %s", screen, at, a.Href, route, want)
		}
	})
	if prints < 3 {
		t.Fatalf("%d prints: the fixtures were not walked", prints)
	}
}
