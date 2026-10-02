package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// previewRow is the row of the preview's one table that has what in a cell.
func previewRow(t *testing.T, tables []view.ImportTable, what string) []view.Text {
	t.Helper()
	if len(tables) != 1 {
		t.Fatalf("the preview is %d tables, the frames draw one", len(tables))
	}
	for _, row := range tables[0].Rows {
		for _, cell := range row {
			if strings.Contains(string(cell), what) {
				return row
			}
		}
	}
	t.Fatalf("no row with %q in %+v", what, tables[0].Rows)
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
func TestImportPreviewIsOneTableWithAResult(t *testing.T) {
	f := importFixture(t)
	page := decode[view.ImportPage](t, f.h.do(get(f.path("contacts")+"/import", f.sess)))
	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "cards.vcf", []byte(importedCards))))
	if len(page.Preview) != 1 || columnsSaid(page.Preview[0].Columns) != "Source · Name · UID · Result" {
		t.Fatalf("contacts: %d tables, columns %v", len(page.Preview), page.Preview)
	}
	if row := previewRow(t, page.Preview, "Ада"); row[0] != "cards.vcf" || row[3] != "New UID will be assigned" {
		t.Errorf("Ada against People = %q", row)
	}
	if row := previewRow(t, page.Preview, "Grace"); row[3] != "Ready" || row[2] == "" {
		t.Errorf("Grace = %q", row)
	}

	c := calendarFixture(t, nil)
	page = decode[view.ImportPage](t, c.h.do(get(c.path("calendar")+"/import", c.sess)))
	page = decode[view.ImportPage](t, c.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), c.sess, false, "cal.ics", []byte(importedCalendar))))
	if len(page.Preview) != 1 || columnsSaid(page.Preview[0].Columns) != "Source · Summary · When · Result" {
		t.Fatalf("calendar: %+v", page.Preview)
	}
	if row := previewRow(t, page.Preview, "VTODO"); row[0] != "cal.ics" || string(row[3]) != "Skipped: a calendar takes events — import it into a list of tasks" {
		t.Errorf("the task = %q", row)
	}
	if row := previewRow(t, page.Preview, "Ковка"); row[3] != "Ready" || !strings.Contains(string(row[2]), "January") {
		t.Errorf("event = %q", row)
	}
	if n := page.Preview[0].Count; n != len(page.Preview[0].Rows) {
		t.Errorf("count %d of %d rows", n, len(page.Preview[0].Rows))
	}
}
