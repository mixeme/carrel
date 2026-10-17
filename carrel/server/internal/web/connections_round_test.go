package web_test

import (
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// from-typecase note 19 (to-typecase 26): the connections as the frames now
// draw them, each read off its frame.

// s-profile-16: editing a connection is a dialog over the page — the page
// under it as it is, its title and its line, and the dialog with a head of
// its own. s-profile-2: Connect account is the same kind of dialog.
func TestEditConnectionIsADialogOverThePage(t *testing.T) {
	f := calendarFixture(t, nil)
	list := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	for _, one := range []struct{ frame, href string }{
		{"s-profile-16", actionHref(t, list.Accounts[0].Actions, "connection.edit")},
		{"s-profile-2", actionHref(t, list.Actions, "connection.add")},
	} {
		head := frameHeadText.FindStringSubmatch(mockupFrame(t, one.frame))
		if head == nil || !strings.Contains(mockupFrame(t, one.frame), `<div class="tc-dialog`) {
			t.Fatalf("%s draws no dialog head: read it again", one.frame)
		}
		page := decode[view.ConnectionsPage](t, f.h.do(get(one.href, f.sess)))
		if page.Add == nil || string(page.DialogTitle) != frameText(head[1]) || string(page.DialogSubtitle) != frameText(head[2]) {
			t.Errorf("%s: dialog %v, head %q · %q, the frame %q · %q", one.frame, page.Add != nil,
				page.DialogTitle, page.DialogSubtitle, frameText(head[1]), frameText(head[2]))
		}
		if page.Title != list.Title || saidRuns(page.Subtitle) != saidRuns(list.Subtitle) || len(page.Accounts) != len(list.Accounts) {
			t.Errorf("%s: the page under it = %q · %q with %d cards, want %q · %q with %d", one.frame,
				page.Title, page.Subtitle, len(page.Accounts), list.Title, list.Subtitle, len(list.Accounts))
		}
	}
	if list.DialogTitle != "" || list.DialogSubtitle != "" {
		t.Errorf("the list has a dialog head: %q · %q", list.DialogTitle, list.DialogSubtitle)
	}
}

var frameMenuReason = regexp.MustCompile(`(?s)<button class="tc-menu__item[^"]*"[^>]*role="menuitem"(?: title="([^"]*)")?( disabled)?>(.*?)</button>`)

// s-profile-17: the whole storage — the address the connection was made
// with — has Rename and Delete on the server… off with the frame's reason,
// and goes elsewhere only as a copy.
func TestWholeStorageMenuIsTheFrame(t *testing.T) {
	type item struct{ label, reason string }
	var want []item
	for _, m := range frameMenuReason.FindAllStringSubmatch(mockupFrame(t, "s-profile-17"), -1) {
		reason := ""
		if m[2] != "" {
			reason = html.UnescapeString(m[1])
		}
		want = append(want, item{frameText(m[3]), reason})
	}
	if len(want) != 4 || want[0].reason == "" {
		t.Fatalf("s-profile-17 = %+v: read it again", want)
	}
	f := calendarFixture(t, nil)
	fileStorage(t, f, "vps", "Docs")
	_, bID := fileStorage(t, f, "nas", "Archive")
	editBlob(t, f.h, f.sess, func(blob *sealed.Blob) {
		blob.Collections = append(blob.Collections, sealed.Collection{AccountID: bID, Path: "/", Name: "/", Kinds: []view.Kind{view.KindFile}})
	})
	page := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	menu := connectionRow(t, page, bID, "/").Menu
	sameMenu(t, "the whole storage", menu, "s-profile-17")
	var got []item
	for _, m := range menu {
		got = append(got, item{string(m.Label), string(m.Reason)})
		if m.Enabled == (m.Reason != "") {
			t.Errorf("%s: enabled %v with reason %q", m.ID, m.Enabled, m.Reason)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("menu = %+v, the frame %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d = %+v, the frame %+v", i, got[i], want[i])
		}
	}
}

var (
	frameSheetInput  = regexp.MustCompile(`(?s)<div class="tc-field[^"]*"><label>([^<]*)</label><input(?:[^>]*?placeholder="([^"]*)")?[^>]*>(?:<span class="tc-field__hint[^"]*">(.*?)</span>)?</div>`)
	frameSheetButton = regexp.MustCompile(`(?s)<div class="tc-sheet__foot"><button class="tc-btn is-(?:primary|danger-solid)"[^>]*>(.*?)</button>`)
	frameRefItem     = regexp.MustCompile(`(?s)<span class="tc-stack__item">(.*?)</span></span>`)
	// «Open in Files» is a link since note 73 (task 187): a button that only
	// goes somewhere is an <a>.
	frameOpenIn = regexp.MustCompile(`(?s)<div class="tc-line tc-line--gap-8">\s*<(?:button|a) class="tc-btn"[^>]*>(.*?)</(?:button|a)>\s*<span[^>]*>(.*?)</span>`)
)

// storageFolder is a plain file storage «vps» with a folder Docs of one file.
func storageFolder(t *testing.T) (davFixture, string, string) {
	t.Helper()
	f := calendarFixture(t, nil)
	_, id := fileStorage(t, f, "vps", "Docs")
	return f, id, davtest.FileRoot + "Docs/"
}

// s-profile-18: renaming a folder of a storage is the one field Name, the
// frame's line under it, and «Rename»; the head names the storage and the
// folder. A folder's address is its name, so no address is shown.
func TestFolderRenameSheetIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-profile-18")
	subject := frameOne(t, frameSheetSub, frame, "subject")[1]
	msg := frameOne(t, frameSheetMsg, frame, "message")[1]
	button := frameOne(t, frameSheetButton, frame, "main button")[1]
	var fields []string
	for _, m := range frameSheetInput.FindAllStringSubmatch(frame, -1) {
		fields = append(fields, m[1])
	}
	if !strings.HasSuffix(subject, " · Documents") || len(fields) != 1 {
		t.Fatalf("s-profile-18 subject %q, fields %q: read it again", subject, fields)
	}
	f, id, docs := storageFolder(t)
	page := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	sheet := decode[view.CollectionRenameSheet](t, f.h.do(get(menuHref(t, connectionRow(t, page, id, docs).Menu, "collection.rename"), f.sess)))
	sameCaret(t, "s-profile-18", "folder rename", sheet.Form)
	sameActionMarks(t, "s-profile-18", "folder rename", sheet.Actions, nil)
	if sheet.Stamp != "vps · Docs" {
		t.Errorf("head = %q, the frame %q (the storage · the folder)", sheet.Stamp, subject)
	}
	if string(sheet.Note) != msg || sheet.Address != "" {
		t.Errorf("note %q, address %q; the frame %q and no address", sheet.Note, sheet.Address, msg)
	}
	var got []string
	for _, r := range sheet.Form.Sets[0].Rows {
		if r.Kind != view.InputHidden {
			got = append(got, string(r.Label))
		}
	}
	if !equalStrings(got, fields) || len(sheet.Actions) != 2 || sheet.Actions[1].ID != "collection.cancel" || string(sheet.Actions[0].Label) != button {
		t.Errorf("fields %q, actions %+v; the frame %q and %q", got, sheet.Actions, fields, button)
	}
}

// s-profile-19: deleting a folder is the collection's sheet — what goes,
// what points at it, the typed name — with «Open in Files» and its line in
// place of the download; the head names the storage and the folder, the
// field shows the name as its sample, and the attachment folder in it is
// named among what points at it.
func TestFolderDeleteSheetIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-profile-19")
	subject := frameOne(t, frameSheetSub, frame, "subject")[1]
	warning := frameOne(t, frameSheetMsg, frame, "message")[1]
	button := frameOne(t, frameSheetButton, frame, "main button")[1]
	open := frameOne(t, frameOpenIn, frame, "Open in Files")
	field := frameSheetInput.FindStringSubmatch(frame)
	var refs []string
	for _, m := range frameRefItem.FindAllStringSubmatch(frame, -1) {
		refs = append(refs, frameText(m[1]))
	}
	count := regexp.MustCompile(`all (\d+) files`).FindStringSubmatch(warning)
	host := regexp.MustCompile(`deleted on ([^.]+)\.`).FindStringSubmatch(warning)
	if field == nil || count == nil || host == nil || len(refs) != 2 || !strings.HasPrefix(refs[0], "Attachments — ") {
		t.Fatalf("s-profile-19 field %q, warning %q, references %q: read it again", field, warning, refs)
	}
	job := regexp.MustCompile(`«([^»]+)»`).FindStringSubmatch(refs[1])
	// The archive line is the collection's (from-typecase note 20): the job
	// that copies the folder has made an archive of it.
	_, archive := pointers(t, "s-profile-19")
	day := frameArchiveDay.FindStringSubmatch(archive)
	if day == nil {
		t.Fatalf("s-profile-19 archive %q: read it again", archive)
	}
	f, id, docs := storageFolder(t)
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		b.Attachments = &sealed.AttachmentFolder{AccountID: id, Path: docs, Folder: []string{"carrel"}}
		b.BackupJobs = append(b.BackupJobs, sealed.BackupJob{ID: "j-1", Name: job[1],
			Sources: []sealed.Selection{{Kind: view.KindFile, AccountID: id, Path: docs}}})
	})
	lastArchived(t, f, "j-1", day[1])
	page := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	sheet := decode[view.CollectionDeleteSheet](t, f.h.do(get(menuHref(t, connectionRow(t, page, id, docs).Menu, "collection.delete"), f.sess)))
	sameCaret(t, "s-profile-19", "folder delete", sheet.Form)
	if sheet.Subject != "vps · Docs" {
		t.Errorf("head = %q, the frame %q (the storage · the folder)", sheet.Subject, subject)
	}
	// The server is named by its connection, as on the move sheet (s-profile-8).
	want := strings.Replace(strings.Replace(warning, "all "+count[1]+" files", "all 1 file", 1), "on "+host[1]+".", "on vps.", 1)
	if plainText(sheet.Warning) != want {
		t.Errorf("warning = %q\nwant      %q", sheet.Warning, want)
	}
	if !equalStrings(refLines(sheet.References), refs) {
		t.Errorf("references = %q, the frame %q", sheet.References, refs)
	}
	if plainText(sheet.Archive) != archive {
		t.Errorf("archive = %q,\nthe frame %q", sheet.Archive, archive)
	}
	if sheet.Export == nil || string(sheet.Export.Label) != open[1] || string(sheet.ExportNote) != open[2] {
		t.Errorf("export %+v %q, the frame %q %q", sheet.Export, sheet.ExportNote, open[1], open[2])
	}
	name := rowNamed(t, sheet.Form, "name")
	if string(name.Label) != field[1] || name.Placeholder != "Docs" || name.Hint != "" {
		t.Errorf("name = %+v, the frame %q with the name as its sample", name, field[1])
	}
	if len(sheet.Actions) != 2 || sheet.Actions[1].ID != "collection.cancel" || string(sheet.Actions[0].Label) != button {
		t.Errorf("actions = %+v, the frame %q", sheet.Actions, button)
	}
}

func texts(v []view.Text) []string {
	out := []string{}
	for _, t := range v {
		out = append(out, string(t))
	}
	return out
}

// refLines are the references as the frames read them: what, the screen's
// dash, what becomes of it (s-profile-9, Кл16).
func refLines(v []view.Reference) []string {
	out := []string{}
	for _, r := range v {
		out = append(out, string(r.What)+" — "+string(r.Then))
	}
	return out
}

// s-profile-20: New folder asks the server and the name, with the frame's
// line under the name, and nothing else.
func TestNewFolderSheetIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-profile-20")
	hint := frameSheetInput.FindStringSubmatch(frame)
	if hint == nil || hint[1] != "Name" || hint[3] == "" || !strings.Contains(frame, "<label>On which server</label>") {
		t.Fatalf("s-profile-20 Name = %q: read it again", hint)
	}
	f, id, _ := storageFolder(t)
	sheet := decode[view.CollectionNewSheet](t, f.h.do(get("/app/collections/new?"+url.Values{"a": {id}, "kind": {"file"}}.Encode(), f.sess)))
	var got []string
	for _, r := range sheet.Form.Sets[0].Rows {
		if r.Kind != view.InputHidden {
			got = append(got, string(r.Label))
		}
	}
	sameCaret(t, "s-profile-20", "new folder", sheet.Form)
	sameActionMarks(t, "s-profile-20", "new folder", sheet.Actions, nil)
	if !equalStrings(got, []string{"On which server", "Name"}) {
		t.Errorf("fields = %q", got)
	}
	if h := string(rowNamed(t, sheet.Form, "name").Hint); h != frameText(hint[3]) {
		t.Errorf("Name hint = %q, the frame %q", h, frameText(hint[3]))
	}
}

// s-profile-21: when no server of the kind answers, there is nothing to
// choose: no server field, the line as the sheet's message, and Create off
// with it.
func TestNewCollectionWhenNoServerAnswers(t *testing.T) {
	frame := mockupFrame(t, "s-profile-21")
	msg := frameOne(t, frameSheetMsg, frame, "message")[1]
	off := regexp.MustCompile(`<button class="tc-btn is-primary" type="submit" title="([^"]*)" disabled>Create</button>`).FindStringSubmatch(frame)
	if off == nil || html.UnescapeString(off[1]) != msg || !strings.Contains(msg, "baikal") || strings.Contains(frame, "<label>On which server</label>") {
		t.Fatalf("s-profile-21 message %q, Create %q: read it again", msg, off)
	}
	f := calendarFixture(t, nil)
	f.srv.Refuse("PROPFIND", http.StatusUnauthorized, "")
	f.h.do(postForm("/app/settings/connections?"+url.Values{"op": {"test"}, "account": {f.id}}.Encode(), f.sess, url.Values{}))
	sheet := decode[view.CollectionNewSheet](t, f.h.do(get("/app/collections/new?kind=event", f.sess)))
	want := strings.Replace(msg, "baikal", string(sheet.Target.Options[0].Label), 1)
	if string(sheet.Note) != want {
		t.Errorf("message = %q, want %q", sheet.Note, want)
	}
	if hasRow(sheet.Form, "account") {
		t.Errorf("a server field with nothing in it: %+v", rowNamed(t, sheet.Form, "account"))
	}
	if c := actionByID(sheet.Actions, "collection.create"); c == nil || c.Enabled || string(c.Reason) != want {
		t.Errorf("Create = %+v", c)
	}
	sameActionMarks(t, "s-profile-21", "no server", sheet.Actions, nil)
}

// attachments-4: Browse of the attachment folder — the frame's title and
// line; «Choose this folder» off, until a folder is chosen, with the reason
// the frame's caption gives.
func TestAttachmentsBrowseIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "attachments-4")
	title := frameOne(t, regexp.MustCompile(`<b class="tc-sheet__title">(.*?)</b>`), frame, "title")[1]
	purpose := frameOne(t, regexp.MustCompile(`(?s)<div class="tc-sheet__body">\s*<span[^>]*>(.*?)</span>`), frame, "line")[1]
	reason := frameOne(t, regexp.MustCompile(`с причиной «([^»]+)»`), frame, "reason")[1]
	f := calendarFixture(t, nil)
	fileStorage(t, f, "vps", "Docs")
	page := decode[view.AttachmentsPage](t, f.h.do(get("/app/settings/attachments", f.sess)))
	folder := rowNamed(t, page.Form, "folder")
	if folder.Extra == nil {
		t.Fatalf("Folder in it has no Browse: %+v", folder)
	}
	sheet := decode[view.FilePickerSheet](t, f.h.do(get(folder.Extra.Href, f.sess)))
	if string(sheet.Title) != title || string(sheet.Purpose) != purpose {
		t.Errorf("sheet %q · %q, the frame %q · %q", sheet.Title, sheet.Purpose, title, purpose)
	}
	if c := actionByID(sheet.Actions, "files.picker.choose"); c == nil || c.Enabled || string(c.Reason) != reason {
		t.Errorf("Choose this folder = %+v, want off with %q", c, reason)
	}
	sameActionMarks(t, "attachments-4", "browse", sheet.Actions, nil)
}

// s-profile-1: the line under the title counts the DAV servers — cards, the
// subscriptions apart — and the collections and folders in those cards,
// unticked ones too; a server that does not answer now counts as many as at
// its last answer.
func TestConnectionsSubtitleCountsTheCards(t *testing.T) {
	sub := frameText(frameOne(t, frameHeadText, mockupFrame(t, "s-profile-1"), "head")[2])
	if !regexp.MustCompile(`^Signed in as \S+ · \d+ servers · \d+ collections · \d+ subscriptions$`).MatchString(sub) {
		t.Fatalf("s-profile-1 line %q: read it again", sub)
	}
	f := calendarFixture(t, nil)
	fileStorage(t, f, "vps", "Docs", "Pics")
	silent, _ := fileStorage(t, f, "nas", "Archive")
	silent.Refuse("PROPFIND", http.StatusUnauthorized, "")
	for _, card := range decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess))).Accounts {
		f.h.do(postForm(actionHref(t, card.Actions, "connection.test"), f.sess, url.Values{}))
	}
	page := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	if page.Accounts[2].State != view.CollectionUnavailable {
		t.Fatalf("nas answers: %+v", page.Accounts[2])
	}
	cols := len(catalogOf(t, f).CollectionsOf(f.id))
	want := regexp.MustCompile(`\d+ servers · \d+ collections · \d+ subscriptions`).ReplaceAllString(
		regexp.MustCompile(`as \S+`).ReplaceAllString(sub, "as owner"), "3 servers · "+strconv.Itoa(cols+3)+" collections · 0 subscriptions")
	if plainText(page.Subtitle) != want {
		t.Errorf("line = %q, want %q", page.Subtitle, want)
	}
}
