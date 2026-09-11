package screens

import "carrel.dev/typecase/widget"

// ImportExport assembles mockup s-io step 2: wizard steps, preview tables for
// all four record kinds, and summary stats.
func ImportExport() string {
	return shellFrame(shellNav("/contacts"), ioRail(), ioMain(), widget.Side{}, "")
}

func ioRail() widget.Rail {
	n248, n51 := 248, 51
	return widget.Rail{
		Sections: []widget.RailSection{
			{
				Title: "nextcloud · home",
				Rows: []widget.TickRow{
					{Label: "Контакты", Mark: widget.Mark{Colour: "#4A6B52", Label: "Контакты"}, Count: intPtr(n248), Tick: widget.TickOn},
					{Label: "Архив", Mark: widget.Mark{Colour: "#A05A2C", Label: "Архив"}, Count: intPtr(n51)},
				},
			},
		},
	}
}

func ioMain() widget.Main {
	return widget.Main{
		Label: "Import",
		Body: []widget.Node{
			widget.Steps{Items: []string{"1 · Source", "2 · Preview", "3 · Report"}, Current: 1},
			widget.Head{
				Title: "Import contacts",
				Sub:   "Разное (mix).vcf · 84 KB · vCard 3.0",
				Actions: []widget.Action{
					{Label: "Cancel"},
					{Label: "Import 122 contacts", Kind: widget.ButtonPrimary},
				},
			},
			widget.Field{Name: "into", Label: "Into", Kind: widget.InputText, Value: "Контакты · nextcloud · home"},
			widget.Stats{Items: []widget.Stat{
				{Label: "ready", Value: "122"},
				{Label: "with errors", Value: "3"},
				{Label: "UID collisions", Value: "5"},
			}},
			ioPreviewSection("Contacts", []string{"Source", "Name", "UID", "Result"}, []widget.TableRow{
				{Cells: []string{"Разное (mix).vcf", "Анна Ковалёва", "4f2b-9c31-a0e7", "Ready"}},
				{Cells: []string{"Разное (mix).vcf", "Артём Белов", "1a77-40de-bb02", "New UID will be assigned"}},
				{Cells: []string{"Разное (mix).vcf", "— (no FN)", "—", "vCard 2.1 quoted-printable: cannot decode line 12"}},
			}),
			ioPreviewSection("Events", []string{"Source", "Summary", "When", "Result"}, []widget.TableRow{
				{Cells: []string{"rabota.ics", "Синк-ап по релизу", "10 Aug 10:00", "Ready"}},
				{Cells: []string{"rabota.ics", "Дежурство: первая линия", "all day", "Ready"}},
				{Cells: []string{"rabota.ics", "— (no SUMMARY)", "—", "Missing SUMMARY"}},
			}),
			ioPreviewSection("Tasks", []string{"Source", "Summary", "Due", "Result"}, []widget.TableRow{
				{Cells: []string{"tasks.ics", "Собрать чемодан", "12 Aug 18:00", "Ready"}},
				{Cells: []string{"tasks.ics", "Купить билеты", "15 Aug", "Ready"}},
				{Cells: []string{"rabota.ics", "Созвон с подрядчиком", "11 Aug 15:00", "Skipped — belongs in calendar"}},
			}),
			ioPreviewSection("Notes", []string{"Source", "Title", "Date", "Result"}, []widget.TableRow{
				{Cells: []string{"заметки-2026.zip", "Что почитать про CRDT", "8 Aug", "Ready"}},
				{Cells: []string{"заметки-2026.zip", "Про дорогу до дачи", "4 Aug", "Ready"}},
				{Cells: []string{"draft.md", "— (from filename)", "—", "No front matter — dated from file"}},
			}),
			widget.Stats{Items: []widget.Stat{
				{Label: "imported", Value: "118"},
				{Label: "skipped", Value: "4"},
				{Label: "errors", Value: "3"},
			}},
			widget.Msg{Text: "Import always creates. A UID that already exists gets a new one and is listed in the report."},
		},
	}
}

func ioPreviewSection(title string, cols []string, rows []widget.TableRow) widget.Sec {
	columns := make([]widget.Column, len(cols))
	for i, c := range cols {
		columns[i] = widget.Column{Label: c}
	}
	return widget.Sec{
		Rubric: widget.Rubric{Text: title},
		Body:   []widget.Node{widget.Table{Columns: columns, Rows: rows}},
	}
}
