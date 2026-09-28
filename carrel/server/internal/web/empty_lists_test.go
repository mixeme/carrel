package web_test

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// nullLists walks a value and its JSON side by side and names every list
// that came out as null. A pointer that is nil is not a finding: that is
// what null means (C-6.1).
func nullLists(t *testing.T, prefix string, typ reflect.Type, raw any) []string {
	t.Helper()
	var out []string
	switch typ.Kind() {
	case reflect.Pointer:
		if raw == nil {
			return nil
		}
		return nullLists(t, prefix, typ.Elem(), raw)
	case reflect.Slice:
		if raw == nil {
			return []string{prefix}
		}
		for i, e := range raw.([]any) {
			out = append(out, nullLists(t, prefix+"[]", typ.Elem(), e)...)
			if i > 2 {
				break
			}
		}
	case reflect.Struct:
		m, ok := raw.(map[string]any)
		if !ok {
			return nil
		}
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if !f.IsExported() {
				continue
			}
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if name == "-" {
				continue
			}
			if f.Anonymous && name == "" {
				out = append(out, nullLists(t, prefix, f.Type, raw)...)
				continue
			}
			if name == "" {
				name = f.Name
			}
			if e, has := m[name]; has {
				out = append(out, nullLists(t, prefix+"."+name, f.Type, e)...)
			}
		}
	}
	return out
}

// C-6.1: an empty list goes out as [], never as null — whatever the screen
// and whatever the handler did with it. The rule lives at the boundary, so
// it holds for a page that was never written with it in mind.
func TestEmptyListsGoOutAsLists(t *testing.T) {
	pages := []any{
		view.ContactsListPage{}, view.AgendaPage{}, view.TasksPage{}, view.NotesPage{},
		view.ContactPage{}, view.EventPage{}, view.TaskPage{}, view.NotePage{},
		view.ContactTimelinePage{}, view.SearchPage{}, view.ConnectionsPage{}, view.AdminAuditPage{},
		view.PublishSheet{}, view.ExportSheet{}, view.NewSheet{}, view.ImportPage{}, view.NoteDoc{},
	}
	for _, page := range pages {
		rec := httptest.NewRecorder()
		if err := (render.JSONRenderer{}).JSON(rec, 200, page); err != nil {
			t.Fatalf("%T: %v", page, err)
		}
		var raw any
		if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
			t.Fatalf("%T: %v", page, err)
		}
		if bad := nullLists(t, reflect.TypeOf(page).Name(), reflect.TypeOf(page), raw); len(bad) > 0 {
			t.Errorf("%T: these lists went out as null: %v", page, bad)
		}
	}
}

// The rule holds for a real answer too, not only for a zero value.
func TestAListScreenHasNoNullLists(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEND:VCARD\n")
	rec := f.h.do(get(f.path("contacts"), f.sess))
	var raw any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if bad := nullLists(t, "ContactsListPage", reflect.TypeOf(view.ContactsListPage{}), raw); len(bad) > 0 {
		t.Errorf("the contacts screen answers null lists: %v", bad)
	}
}
