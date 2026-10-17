package web

import (
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// panelLists are the lists whose rows open their record in the details panel
// (C-6.4): the book, the agenda, the tasks, the notes.
var panelLists = map[reflect.Type]bool{
	reflect.TypeFor[view.ContactsListPage](): true,
	reflect.TypeFor[view.AgendaPage]():       true,
	reflect.TypeFor[view.TasksPage]():        true,
	reflect.TypeFor[view.NotesPage]():        true,
}

// stampPanel points the rows of a list at the panel or at the record's page
// by this browser's «Keep the details panel open» (Look.Panel, Appearance;
// Typecase's note 75, task 186 p. 3): ticked, a row leads to the same list
// with ?open= — the record in the panel, without a script as with one, the
// panel being the list's zone —; not ticked, to the record's page. The list
// stays the same; only the rows' addresses change. Nothing else is touched:
// not the screen under a sheet, not the panel itself.
func stampPanel(r *http.Request, body any, look view.Look) any {
	if body == nil || !look.Panel || r == nil {
		return body
	}
	v := reflect.ValueOf(body)
	if !panelLists[v.Type()] {
		return body
	}
	out := reflect.New(v.Type()).Elem()
	out.Set(v)
	q := r.URL.Query()
	q.Del("open")
	q.Del("foreign")
	for _, name := range []string{"Groups", "Days", "Rows"} {
		if f := out.FieldByName(name); f.IsValid() {
			panelRows(f, r.URL.EscapedPath(), q)
		}
	}
	return out.Interface()
}

// panelRows rewrites, under v, the address of every row that opens in the
// details panel.
func panelRows(v reflect.Value, path string, q url.Values) {
	switch v.Kind() {
	case reflect.Slice:
		if v.IsNil() {
			return
		}
		fresh := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		reflect.Copy(fresh, v)
		v.Set(fresh)
		for i := 0; i < fresh.Len(); i++ {
			panelRows(fresh.Index(i), path, q)
		}
	case reflect.Struct:
		href, target := v.FieldByName("Href"), v.FieldByName("Target")
		if strings.HasSuffix(v.Type().Name(), "Row") && href.IsValid() && target.IsValid() &&
			href.Kind() == reflect.String && target.String() == render.TargetDetails && href.String() != "" {
			open := url.Values{}
			for k, vs := range q {
				open[k] = vs
			}
			open.Set("open", href.String())
			href.SetString(path + "?" + open.Encode())
			return
		}
		if rows := v.FieldByName("Rows"); rows.IsValid() && rows.Kind() == reflect.Slice {
			panelRows(rows, path, q)
		}
	}
}
