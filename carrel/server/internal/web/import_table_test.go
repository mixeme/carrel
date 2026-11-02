package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// previewRow is the row of the preview's one table that has what in a cell,
// read as a set's four — file, name, its date or address, result: the table
// of one file has no file column (Typecase's note 157), and its row comes
// back with that cell empty.
func previewRow(t *testing.T, tables []view.ImportTable, what string) []view.Text {
	t.Helper()
	row := previewCells(t, tables, what)
	if len(tables[0].Columns) == 3 {
		return append([]view.Text{""}, row...)
	}
	return row
}

// previewCells is that row as the table has it.
func previewCells(t *testing.T, tables []view.ImportTable, what string) []view.Text {
	t.Helper()
	if len(tables) != 1 {
		t.Fatalf("the preview is %d tables, the frames draw one", len(tables))
	}
	for _, line := range tables[0].Lines {
		// The component of a stray record is a badge at its title (Им8).
		if line.Component == view.Text(what) {
			return line.Cells
		}
		for _, cell := range line.Cells {
			if strings.Contains(string(cell), what) {
				return line.Cells
			}
		}
	}
	t.Fatalf("no row with %q in %+v", what, tables[0].Lines)
	return nil
}

func columnsSaid(cols []view.Text) string {
	var out []string
	for _, c := range cols {
		out = append(out, string(c))
	}
	return strings.Join(out, " · ")
}

// s-io-5…8: the preview is one table — its columns the kind's, the last
// Result —, and what the file holds and will not be written stands in it
// with the records, saying why and where it belongs: «Ready», «New UID will
// be assigned», «Skipped: a calendar takes events — import it into a list
// of tasks».
//
// An import of one file has no Source column: every row would name the file
// the head names; a contact is told by its first address, not its UID
// (Typecase's note 157, the owner's decision of 02.11.2026).
func TestImportPreviewIsOneTableWithAResult(t *testing.T) {
	f := importFixture(t)
	page := decode[view.ImportPage](t, f.h.do(get(f.path("contacts")+"/import", f.sess)))
	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "cards.vcf", []byte(importedCards))))
	page = asCopies(t, f, page)
	if len(page.Preview) != 1 || columnsSaid(page.Preview[0].Columns) != "Name · Email · Result" {
		t.Fatalf("contacts: %d tables, columns %v", len(page.Preview), page.Preview)
	}
	if row := previewCells(t, page.Preview, "Ада"); len(row) != 3 || row[2] != "New UID will be assigned" {
		t.Errorf("Ada against People = %q", row)
	}
	// Grace's card has no address: the cell is empty, the screen's «—».
	if row := previewCells(t, page.Preview, "Grace"); row[2] != "Ready" || row[1] != "" {
		t.Errorf("Grace = %q", row)
	}

	c := calendarFixture(t, nil)
	page = decode[view.ImportPage](t, c.h.do(get(c.path("calendar")+"/import", c.sess)))
	page = decode[view.ImportPage](t, c.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), c.sess, false, "cal.ics", []byte(importedCalendar))))
	if len(page.Preview) != 1 || columnsSaid(page.Preview[0].Columns) != "Summary · When · Result" {
		t.Fatalf("calendar: %+v", page.Preview)
	}
	if row := previewCells(t, page.Preview, "VTODO"); len(row) != 3 || string(row[2]) != "Skipped: a calendar takes events — import it into a list of tasks" {
		t.Errorf("the task = %q", row)
	}
	// A moment without an end is a moment (Typecase's note 113 p. 4).
	if row := previewRow(t, page.Preview, "Ковка"); row[3] != "Ready — a moment (no end)" || !strings.Contains(string(row[2]), "January") {
		t.Errorf("event = %q", row)
	}
	if n := page.Preview[0].Count; n != len(page.Preview[0].Lines) {
		t.Errorf("count %d of %d rows", n, len(page.Preview[0].Lines))
	}
}
