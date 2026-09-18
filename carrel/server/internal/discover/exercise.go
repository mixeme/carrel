package discover

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/view"
)

// Exercise is the second, mutating stage of the DAV validator (U-6.13
// /admin/dav): in the first writable collection of each kind it creates a
// test object, reads it back and deletes it, and says what each step got.
// Only by explicit consent; nothing else on the server is touched, and an
// object the server would not delete is named with its path.
func Exercise(ctx context.Context, g *dav.Guard, res *Result, username, password, certPin string) view.ExerciseReport {
	report := view.ExerciseReport{OK: true}
	client, err := dav.NewClient(g, res.BaseURL, username, password, certPin)
	if err != nil {
		report.OK = false
		report.Rows = append(report.Rows, view.ExerciseRow{Operation: "Connect", Result: "failed", Detail: view.Text(formatErr(err))})
		return report
	}
	tag := make([]byte, 6)
	_, _ = rand.Read(tag)
	id := "carrel-check-" + hex.EncodeToString(tag)

	type probe struct {
		kind  view.Kind
		noun  string
		ext   string
		body  string
		found string
		media string
	}
	probes := []probe{
		{view.KindEvent, "an event", ".ics", "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Carrel//check//EN\r\nBEGIN:VEVENT\r\nUID:" + id +
			"\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20260101T100000Z\r\nDTEND:20260101T110000Z\r\nSUMMARY:Carrel check\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n", id, "text/calendar; charset=utf-8"},
		{view.KindTask, "a task", ".ics", "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Carrel//check//EN\r\nBEGIN:VTODO\r\nUID:" + id +
			"\r\nDTSTAMP:20260101T000000Z\r\nSUMMARY:Carrel check\r\nEND:VTODO\r\nEND:VCALENDAR\r\n", id, "text/calendar; charset=utf-8"},
		{view.KindContact, "a contact", ".vcf", "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:" + id + "\r\nFN:Carrel check\r\nN:check;Carrel;;;\r\nEND:VCARD\r\n", id, "text/vcard; charset=utf-8"},
		{view.KindFile, "a file", ".txt", "Carrel check " + id + "\n", id, "text/plain; charset=utf-8"},
	}
	for _, p := range probes {
		var col *Collection
		for i := range res.Collections {
			c := &res.Collections[i]
			if !c.ReadOnly && hasKind(c.Kinds, p.kind) {
				col = c
				break
			}
		}
		if col == nil {
			report.Rows = append(report.Rows, view.ExerciseRow{Operation: view.Text("Create " + p.noun), Result: "skipped",
				Detail: "No collection of this kind that can be written to."})
			continue
		}
		name := col.DisplayName
		if name == "" {
			name = col.Path
		}
		path := strings.TrimSuffix(col.Path, "/") + "/" + id + "-" + string(p.kind) + p.ext
		row := func(op, result, detail string) {
			if result != "ok" {
				report.OK = false
			}
			report.Rows = append(report.Rows, view.ExerciseRow{Operation: view.Text(op), Result: view.Text(result), Detail: view.Text(detail)})
		}
		_, err := client.PutCreate(ctx, path, dav.Typed(strings.NewReader(p.body), p.media))
		if err != nil {
			row(fmt.Sprintf("Create %s in «%s»", p.noun, name), "failed", formatErr(err))
			continue
		}
		row(fmt.Sprintf("Create %s in «%s»", p.noun, name), "ok", path)
		// The delete names the version read back, quoted as If-Match wants.
		etag := ""
		body, got, err := client.Get(ctx, path, nil)
		if err == nil && got != "" {
			etag = `"` + strings.Trim(got, `"`) + `"`
		}
		if err != nil {
			row("Read it back", "failed", formatErr(err))
		} else {
			data, _ := io.ReadAll(io.LimitReader(body, 64<<10))
			body.Close()
			if strings.Contains(string(data), p.found) {
				row("Read it back", "ok", fmt.Sprintf("%d bytes, as written", len(data)))
			} else {
				row("Read it back", "failed", "the server answered with something else")
			}
		}
		if err := client.Delete(ctx, path, etag); err != nil {
			row("Delete it", "failed", formatErr(err)+" — the test object stays at "+path)
		} else {
			row("Delete it", "ok", "")
		}
	}
	return report
}

func hasKind(kinds []view.Kind, k view.Kind) bool {
	for _, x := range kinds {
		if x == k {
			return true
		}
	}
	return false
}
