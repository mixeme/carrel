package gate

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	tableHeadRe = regexp.MustCompile(`<thead><tr>(.*?)</tr></thead>`)
	tableRowRe  = regexp.MustCompile(`(?is)<tr\b[^>]*>(.*?)</tr>`)
	thRe        = regexp.MustCompile(`<th\b`)
	tdRe        = regexp.MustCompile(`<td\b`)
)

// CheckTableColumns requires equal header and row cell counts in tc-table markup.
func CheckTableColumns(catalog []CatalogEntry) error {
	var bad []string
	for _, e := range catalog {
		if !strings.Contains(e.HTML, `class="tc-table"`) || strings.Contains(e.HTML, "tc-table--empty") {
			continue
		}
		head := tableHeadRe.FindStringSubmatch(e.HTML)
		if head == nil {
			bad = append(bad, e.Widget+"/"+e.State+": missing thead")
			continue
		}
		cols := len(thRe.FindAllString(head[1], -1))
		body := e.HTML[strings.Index(e.HTML, "<tbody>"):]
		rows := tableRowRe.FindAllStringSubmatch(body, -1)
		for i, row := range rows {
			cells := len(tdRe.FindAllString(row[1], -1))
			if cells != cols {
				bad = append(bad, fmt.Sprintf("%s/%s: row %d has %d cells, %d headers", e.Widget, e.State, i+1, cells, cols))
			}
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("table-columns: %s", strings.Join(bad, "; "))
	}
	return nil
}
