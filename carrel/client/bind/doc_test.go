package bind_test

import (
	"testing"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// faces are a note's text as the frames draw it: the page's lead and the
// paragraph after a section (s-notes-2), the heading over tasks, a task's
// tick, inline code a chip (s-notes-25).
var faces = bind.TextFaces{Lead: w.ProseLead, After: w.ProseLeadAfter, Tasks: w.RubricBottom8, Tick: w.TickNudge, Code: w.MonoChip}

func para(text string) view.Block { return view.Block{Kind: view.BlockPara, Text: view.Text(text)} }

func TestSpans(t *testing.T) {
	b := view.Block{Kind: view.BlockPara, Children: []view.Block{
		para("Итоги в "), {Kind: view.BlockEmph, Text: "четверг"}, para(": гоняем с "),
		{Kind: view.BlockCode, Text: "--dry-run"}, para(", подробности — в "),
		{Kind: view.BlockLink, Href: "https://example.org/s", Children: []view.Block{para("заметке о "), {Kind: view.BlockStrong, Text: "схеме"}}},
		para("."),
	}}
	same(t, "a paragraph in its faces", bind.Spans(b, faces), []w.Inline{
		w.Text("Итоги в "), w.Em("четверг"), w.Text(": гоняем с "), w.Mono{Text: "--dry-run", Variant: w.MonoChip},
		w.Text(", подробности — в "), w.Link{Text: "заметке о схеме", Href: "https://example.org/s"}, w.Text("."),
	})
	same(t, "plain words", bind.Spans(para("Текст."), faces), []w.Inline{w.Text("Текст.")})
	same(t, "bold", bind.Spans(view.Block{Kind: view.BlockPara, Children: []view.Block{{Kind: view.BlockStrong, Text: "Виктор"}}}, faces), []w.Inline{w.Strong("Виктор")})
	same(t, "nothing", bind.Spans(view.Block{Kind: view.BlockPara}, faces), []w.Inline(nil))
}

func TestDocText(t *testing.T) {
	tasks := view.Block{Kind: view.BlockList, Children: []view.Block{
		{Kind: view.BlockTask, Text: "проверить changelog"},
		{Kind: view.BlockTask, Text: "попросить скриншоты", Checked: true},
	}}
	note := []view.Block{
		para("Договорились заморозить ветку."), para("Письмо пишет Анна."),
		{Kind: view.BlockHeading, Level: 2, Text: "Что делаем"}, tasks,
		{Kind: view.BlockHeading, Level: 2, Text: "Отложено"}, para("Перенос вложений."),
	}
	// The paragraphs before any section are the lead, one prose; a heading
	// over tasks is their rubric; a heading alone is a heading; the
	// paragraph after a section is the prose after it (s-notes-2).
	same(t, "the note of the set", bind.DocText(note, faces), []w.DocPart{
		w.Prose{Parts: []w.Inline{w.Text("Договорились заморозить ветку.\n\nПисьмо пишет Анна.")}, Variant: w.ProseLead},
		w.DocTasks{Title: w.Rubric{Text: "Что делаем", Variant: w.RubricBottom8}, Items: []w.DocTask{
			{Tick: w.Tick{Variant: w.TickNudge, Sign: true}, Text: "проверить changelog"},
			{Tick: w.Tick{Done: true, Variant: w.TickNudge, Sign: true}, Text: "попросить скриншоты"},
		}},
		w.DocHeading{Text: "Отложено"},
		w.Prose{Parts: []w.Inline{w.Text("Перенос вложений.")}, Variant: w.ProseLeadAfter},
	})
	// The parts of s-notes-25 (Typecase's task 102): a list nested in an
	// item, a quotation, a block of code; and a table with its head and the
	// alignment of its columns (s-notes-21).
	parts := []view.Block{
		{Kind: view.BlockHeading, Level: 2, Text: "Что держит"},
		{Kind: view.BlockList, Children: []view.Block{
			{Kind: view.BlockItem, Children: []view.Block{para("миграция — "), {Kind: view.BlockLink, Href: "https://e.org/t", Text: "задача"}}},
			{Kind: view.BlockItem, Children: []view.Block{para("тесты"), {Kind: view.BlockList, Children: []view.Block{
				{Kind: view.BlockItem, Children: []view.Block{{Kind: view.BlockCode, Text: "go test ./..."}, para(" — зелёный")}},
				{Kind: view.BlockItem, Text: "нагрузочный"},
			}}}},
		}},
		{Kind: view.BlockQuote, Children: []view.Block{para("Не выпускаем."), para("Пока не прогоним.")}},
		{Kind: view.BlockCode, Text: "make db-copy\n./carrel migrate"},
		{Kind: view.BlockTable, Children: []view.Block{
			{Kind: view.BlockRow, Header: true, Children: []view.Block{{Kind: view.BlockCell, Text: "Что"}, {Kind: view.BlockCell, Text: "Сколько", Align: view.AlignRight}, {Kind: view.BlockCell, Text: "Когда", Align: view.AlignCenter}}},
			{Kind: view.BlockRow, Children: []view.Block{{Kind: view.BlockCell, Text: "Чемодан"}, {Kind: view.BlockCell, Text: "4 500", Align: view.AlignRight}, {Kind: view.BlockCell, Align: view.AlignCenter}}},
		}},
	}
	same(t, "the parts of a text", bind.DocText(parts, faces), []w.DocPart{
		w.DocHeading{Text: "Что держит"},
		w.DocBullets{Items: [][]w.Inline{
			{w.Text("миграция — "), w.Link{Text: "задача", Href: "https://e.org/t"}},
			{w.Text("тесты"), w.DocBullets{Items: [][]w.Inline{
				{w.Mono{Text: "go test ./...", Variant: w.MonoChip}, w.Text(" — зелёный")},
				{w.Text("нагрузочный")},
			}}},
		}},
		w.DocQuote{Parts: []w.Inline{w.Text("Не выпускаем.\n\nПока не прогоним.")}},
		w.DocCode{Text: "make db-copy\n./carrel migrate"},
		w.DocTable{
			Head:   [][]w.Inline{{w.Text("Что")}, {w.Text("Сколько")}, {w.Text("Когда")}},
			Rows:   [][][]w.Inline{{{w.Text("Чемодан")}, {w.Text("4 500")}, nil}},
			Align:  []w.DocAlign{w.DocAlignStart, w.DocAlignEnd, w.DocAlignCenter},
			Switch: w.DocTableSwitch(false),
		},
	})
	same(t, "no text", bind.DocText(nil, faces), []w.DocPart(nil))
}

func TestSecText(t *testing.T) {
	// The panel (s-notes-1, -28): paragraphs as prose in the panel's face,
	// tasks as its list of tasks, and the parts its page has — a heading, a
	// bulleted list, a quotation, code, a table (Typecase's task 125,
	// 0a046d8).
	note := []view.Block{
		para("Договорились заморозить ветку."), para("Письмо пишет Анна."),
		{Kind: view.BlockHeading, Level: 2, Text: "Что делаем"},
		{Kind: view.BlockList, Children: []view.Block{{Kind: view.BlockTask, Text: "проверить changelog"}}},
		{Kind: view.BlockHeading, Level: 2, Text: "Отложено"}, para("Перенос вложений."),
		{Kind: view.BlockList, Children: []view.Block{{Kind: view.BlockItem, Text: "раз"}, {Kind: view.BlockItem, Text: "два"}}},
	}
	panel := faces
	panel.Lead = w.ProseRead
	same(t, "the panel's text", bind.SecText(note, panel), []w.SecPart{
		w.Prose{Parts: []w.Inline{w.Text("Договорились заморозить ветку.\n\nПисьмо пишет Анна.")}, Variant: w.ProseRead},
		w.DocHeading{Text: "Что делаем"},
		w.SecTasks{Items: []w.DocTask{{Tick: w.Tick{Variant: w.TickNudge, Sign: true}, Text: "проверить changelog"}}},
		w.DocHeading{Text: "Отложено"},
		w.Prose{Parts: []w.Inline{w.Text("Перенос вложений.")}, Variant: w.ProseRead},
		w.DocBullets{Items: [][]w.Inline{{w.Text("раз")}, {w.Text("два")}}},
	})
	same(t, "a quotation, code and a table", bind.SecText([]view.Block{
		{Kind: view.BlockQuote, Children: []view.Block{para("Не выпускаем.")}},
		{Kind: view.BlockCode, Text: "./carrel migrate"},
		{Kind: view.BlockTable, Children: []view.Block{
			{Kind: view.BlockRow, Header: true, Children: []view.Block{{Kind: view.BlockCell, Text: "Что"}}},
			{Kind: view.BlockRow, Children: []view.Block{{Kind: view.BlockCell, Text: "Чемодан"}}},
		}},
	}, panel), []w.SecPart{
		w.DocQuote{Parts: []w.Inline{w.Text("Не выпускаем.")}},
		w.DocCode{Text: "./carrel migrate"},
		w.DocTable{Head: [][]w.Inline{{w.Text("Что")}}, Rows: [][][]w.Inline{{{w.Text("Чемодан")}}}, Align: []w.DocAlign{w.DocAlignStart}, Switch: w.DocTableSwitch(false)},
	})
}

func TestDocForm(t *testing.T) {
	f := view.Form{Action: "/app/notes/a/z/5c81/edit", Method: "POST", Sets: []view.FieldSet{{Rows: []view.FormRow{
		{Kind: view.InputHidden, Name: "_csrf", Value: "c"}, {Kind: view.InputHidden, Name: "etag", Value: "e20"},
		{Kind: view.InputText, Name: "summary", Label: "Title", Value: "Заметка"},
	}}}}
	same(t, "the edit, a form", bind.DocForm(&f, "note-edit"), w.Doc{Action: "/app/notes/a/z/5c81/edit", Method: "POST", ID: "note-edit",
		Fields: []w.Hidden{{Name: "_csrf", Value: "c"}, {Name: "etag", Value: "e20"}}})
	same(t, "reading, no form", bind.DocForm(nil, "note-edit"), w.Doc{})
	save := view.Action{ID: "record.save", Label: "Save", Href: "/app/notes/a/z/5c81/edit?etag=e20", Method: "POST", Enabled: true, Primary: true, Submits: true}
	same(t, "Save of the edit bar submits the document by its ID", bind.SubmitForm(save, "note-edit"),
		w.Button{Label: "Save", Kind: w.ButtonPrimary, Form: "note-edit", To: "/app/notes/a/z/5c81/edit?etag=e20"})
	save.Enabled, save.Reason = false, "Read-only"
	same(t, "switched off", bind.SubmitForm(save, "note-edit"), w.Button{Label: "Save", Kind: w.ButtonPrimary, State: w.ButtonDisabled, Reason: "Read-only"})
}

func TestMarks(t *testing.T) {
	r := view.FormRow{Name: "notebook", Label: "Notebook", Kind: view.InputSelect, Value: "/z", Options: []view.Option{
		{Value: "/z", Label: "nextcloud · Записи", Colour: "#4a6b52", Selected: true},
		{Value: "/d", Label: "nextcloud · Дневник", Colour: "#6b4e7a"},
		{Value: "/s", Label: "nextcloud · Общие", Colour: "#3f6e8c", Disabled: true, Reason: "Read-only: this account cannot write here"},
	}}
	ro := w.Badge{Label: "ro", Place: w.Pushed}
	same(t, "the notebooks with their marks", bind.Marks(r, ro), w.Drop{
		Trigger: w.Pick{Value: "nextcloud · Записи", Mark: &w.Mark{Colour: "#4a6b52"}, Label: "Notebook"},
		Menu: w.Menu{Items: []w.MenuPart{
			w.MenuItem{Mark: &w.Mark{Colour: "#4a6b52"}, Label: "nextcloud · Записи", On: true, Name: "notebook", Value: "/z"},
			w.MenuItem{Mark: &w.Mark{Colour: "#6b4e7a"}, Label: "nextcloud · Дневник", Name: "notebook", Value: "/d"},
			w.MenuItem{Mark: &w.Mark{Colour: "#3f6e8c"}, Trail: ro, Label: "nextcloud · Общие", Reason: "Read-only: this account cannot write here", Name: "notebook", Value: "/s"},
		}},
	})
}

func TestTreeRow(t *testing.T) {
	same(t, "a neighbour", bind.TreeRow(view.Action{Label: "Что почитать", Icon: "i-note", Href: "/app/notes/a/z/6d92"}),
		w.TreeRow{Icon: "note", Label: "Что почитать", Href: "/app/notes/a/z/6d92"})
}
