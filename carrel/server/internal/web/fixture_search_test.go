package web_test

import (
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
)

// searchRecords are what s-find-1 finds for «ковал» besides Анна Ковалёва of
// the set (note 114): records drawn in the frames of the search alone, so
// the set does not hold them — the lists of the other frames do not draw
// them. Each is put where the frame names it, for the search and no longer.
var searchRecords = []struct {
	account, slug, uid, body string
}{
	{"radicale · vps", "komanda", "s1-kovalev", "BEGIN:VCARD\nVERSION:3.0\nUID:s1-kovalev\nFN:Пётр Ковалёв\nN:Ковалёв;Пётр;;;\nEMAIL;TYPE=work:p.kovalev@example.org\nEND:VCARD\n"},
	{"nextcloud", "lichnyj", "s1-zvonok", "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//Carrel set//EN\nBEGIN:VEVENT\nUID:s1-zvonok\nDTSTAMP:20260801T000000Z\nSUMMARY:Звонок с Ковалёвой\nDTSTART;TZID=Europe/Moscow:20260810T150000\nDTEND;TZID=Europe/Moscow:20260810T153000\nLOCATION:Zoom\nEND:VEVENT\nEND:VCALENDAR\n"},
	{"nextcloud", "rabota", "s1-smeta", "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//Carrel set//EN\nBEGIN:VTODO\nUID:s1-smeta\nDTSTAMP:20260801T000000Z\nSUMMARY:Ответить Ковалёвой по смете\nDUE;VALUE=DATE:20260812\nPERCENT-COMPLETE:40\nSTATUS:IN-PROCESS\nEND:VTODO\nEND:VCALENDAR\n"},
	{"nextcloud", "zapisi", "s1-tezisy", "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//Carrel set//EN\nBEGIN:VJOURNAL\nUID:s1-tezisy\nDTSTAMP:20260810T160000Z\nDTSTART;VALUE=DATE:20260810\nSUMMARY:Звонок с Ковалёвой — тезисы\nDESCRIPTION:Договорились заморозить ветку в среду; письмо пишет Анна…\nEND:VJOURNAL\nEND:VCALENDAR\n"},
}

// searchFiles are the two files of s-find-1 in Documents, with the sizes and
// times the frame gives them.
var searchFiles = []struct {
	name string
	size int64
	at   string
}{
	{"договор-ковалёва.pdf", 240 << 10, "2026-08-04 11:20"},
	{"Ковалёва — правки по обложке.md", 6 << 10, "2026-07-21 16:02"},
}

// seedSearch puts the records and files of the search's frames in place and
// returns what takes them away.
func seedSearch(t *testing.T, app *mockupApp) func() {
	t.Helper()
	var undo []func()
	for _, r := range searchRecords {
		srv, ext := app.servers[r.account], ".ics"
		if r.slug == "komanda" {
			ext = ".vcf"
		}
		p := app.realPath(t, r.slug) + r.uid + ext
		srv.Seed(p, r.body)
		undo = append(undo, func() { srv.Unseed(p) })
	}
	files := app.servers["sftpgo · vps"]
	for _, f := range searchFiles {
		p := davtest.FileRoot + "Documents/" + f.name
		files.SeedFile(p, "%PDF")
		files.Grow(p, f.size)
		files.SetModified(p, mockupDayAt(t, f.at))
		undo = append(undo, func() { files.RemoveFile(p) })
	}
	return func() {
		for _, u := range undo {
			u()
		}
	}
}
