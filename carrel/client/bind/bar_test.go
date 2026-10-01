package bind_test

import (
	"testing"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

func TestFilter(t *testing.T) {
	bar := view.ListBar{FilterForm: &view.Form{Action: "/app/contacts", Method: "GET", Sets: []view.FieldSet{{Rows: []view.FormRow{
		{Kind: view.InputHidden, Name: "task", Value: "t"},
		{Kind: view.InputHidden, Name: "sort", Value: "changed"},
		{Kind: view.InputText, Name: "filter", Label: "Filter"},
	}}}}}
	same(t, "a filter that sends", bind.Filter(bar, "Filter the loaded cards"), w.Filter{
		Query:   w.Query{Placeholder: "Filter the loaded cards", Label: "Filter the loaded cards", Name: "filter"},
		Waiting: true,
		Action:  "/app/contacts",
		Hidden:  []w.Hidden{{Name: "task", Value: "t"}, {Name: "sort", Value: "changed"}},
	})
	bar.Filter = "анна"
	same(t, "what was typed", bind.Filter(bar, "Filter").Query.Value, "анна")
	same(t, "no form", bind.Filter(view.ListBar{}, "Filter"), w.Filter{Query: w.Query{Placeholder: "Filter", Label: "Filter"}, Waiting: true})
}

func TestCheckForm(t *testing.T) {
	compact := &view.Form{Action: "/app/contacts", Method: "GET", Sets: []view.FieldSet{{Rows: []view.FormRow{
		{Kind: view.InputHidden, Name: "task", Value: "t"},
		{Kind: view.InputHidden, Name: "compact", Value: "0"},
		{Kind: view.InputCheck, Name: "compact", Label: "Compact rows", Options: []view.Option{{Label: "Compact rows", Value: "1"}}},
	}}}}
	same(t, "«Compact rows» off", bind.CheckForm(*compact, "Apply"), w.CheckForm{
		Checks: []w.Check{{Name: "compact", Value: "1", Parts: []w.CheckPart{w.Text("Compact rows")}}},
		Action: "/app/contacts",
		Hidden: []w.Hidden{{Name: "task", Value: "t"}, {Name: "compact", Value: "0"}},
		Apply:  w.Button{Label: "Apply", Small: true, Submit: true},
	})
	same(t, "off", bind.Checked(compact), false)
	compact.Sets[0].Rows[2].Options[0].Selected = true
	same(t, "on", bind.CheckForm(*compact, "Apply").Checks[0].On, true)
	same(t, "on, checked", bind.Checked(compact), true)
	same(t, "none", bind.Checked(nil), false)
}

func TestColumns(t *testing.T) {
	c := view.Columns{
		Label: "Columns",
		Icon:  "i-cols",
		Form: view.Form{Action: "/app/contacts/columns", Method: "POST",
			Sets: []view.FieldSet{{Rows: []view.FormRow{{Kind: view.InputHidden, Name: "_csrf", Value: "c"}}}},
			Actions: []view.Action{
				{ID: "columns.apply", Label: "Apply", Href: "/app/contacts/columns", Method: "POST", Primary: true, Enabled: true},
				{ID: "columns.default", Label: "Reset to defaults", Href: "/app/contacts/columns", Method: "POST", Enabled: true},
			}},
		Items: []view.ColumnItem{
			{ID: "name", Label: "Name", On: true, Fixed: true, Aside: "always"},
			{ID: "phone", Label: "Phone", On: true},
			{ID: "tags", Label: "Tags"},
		},
	}
	same(t, "the button and its menu", bind.Columns(c, bind.ColumnWords{Name: "col", Reset: "default", Drag: "drag"}), w.Drop{
		Trigger: w.Button{Label: "Columns", Icon: "cols"},
		Menu: w.Menu{
			Items: []w.MenuPart{
				w.MenuHeading{Rubric: w.Rubric{Text: "Columns", Form: w.RubricItem}, Form: w.MenuHeadingRoomy},
				w.MenuItem{Icon: "drag", Faint: true, Tick: &w.TickBox{}, Label: "Name", Name: "col", Value: "name", Fixed: true,
					Trail: w.Dim{Parts: []w.Inline{w.Text("always")}, Size: w.Dim10_5, Place: w.Pushed}},
				w.MenuItem{Icon: "drag", Tick: &w.TickBox{}, Label: "Phone", Name: "col", Value: "phone"},
				w.MenuItem{Icon: "drag", Tick: &w.TickBox{Ticked: w.TickedOff}, Label: "Tags", Name: "col", Value: "tags"},
				w.MenuFoot{Items: []w.Button{
					{Label: "Apply", Kind: w.ButtonPrimary, Small: true, Submit: true},
					{Label: "Reset to defaults", Small: true, Submit: true, Name: "col", Value: "default"},
				}},
			},
			Action: "/app/contacts/columns",
			Hidden: []w.Hidden{{Name: "_csrf", Value: "c"}},
		},
	})
}
