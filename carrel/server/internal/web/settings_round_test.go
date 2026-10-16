package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// The owner's deferred round, part 1 (28.09.2026): the settings and the
// connections, each read off its frame.

var frameRailFoot = regexp.MustCompile(`(?s)<div class="tc-railfoot">(.*?)<a class="tc-railfoot__about"`)

// railFootOf is what the rail foot of a frame names, in its order: the
// buttons are drawn bare, and their titles say what they are.
func railFootOf(t *testing.T, id string) []string {
	t.Helper()
	foot := frameRailFoot.FindStringSubmatch(mockupFrame(t, id))
	if foot == nil {
		t.Fatalf("%s draws no rail foot: read it again", id)
	}
	var out []string
	for _, m := range regexp.MustCompile(`title="([^"]*)"`).FindAllStringSubmatch(foot[1], -1) {
		out = append(out, m[1])
	}
	return out
}

// s-contacts-1, s-agenda-1, s-tasks-1, s-notes-1: the rail foot of a kind
// names what it makes — «New address book», «New calendar», «New list»,
// «New notebook» — beside Connect account and Subscribe to a feed. Apply is
// the rail's own submit where there are no scripts (C-6.3) and is not drawn.
func TestKindRailFootsAreTheFrames(t *testing.T) {
	f := calendarFixture(t, nil)
	for _, one := range []struct{ frame, route string }{
		{"s-contacts-1", "/app/contacts"},
		{"s-agenda-1", "/app/calendar"},
		{"s-tasks-1", "/app/tasks"},
		{"s-notes-1", "/app/notes"},
	} {
		want := railFootOf(t, one.frame)
		page := decode[struct {
			Rail view.Rail `json:"rail"`
		}](t, f.h.do(get(one.route, f.sess)))
		var got []string
		for _, a := range page.Rail.Foot {
			if a.ID != "rail.apply" {
				got = append(got, string(a.Label))
			}
		}
		if !equalStrings(got, want) {
			t.Errorf("%s: foot = %q, the frame %s %q", one.route, got, one.frame, want)
		}
	}
}

var (
	frameHeadActs = regexp.MustCompile(`(?s)<div class="tc-head">.*?<div class="tc-acts">(.*?)</div>`)
	frameHeadText = regexp.MustCompile(`(?s)<div class="tc-head__title">(.*?)</div><div class="tc-head__sub">(.*?)</div>`)
)

// headButtons are the buttons of a frame's head, each «label [icon]».
func headButtons(t *testing.T, id string) []string {
	t.Helper()
	acts := frameHeadActs.FindStringSubmatch(mockupFrame(t, id))
	if acts == nil {
		t.Fatalf("%s draws no actions in its head: read it again", id)
	}
	var out []string
	for _, m := range frameButton.FindAllStringSubmatch(acts[1], -1) {
		icon := ""
		if i := frameIcon.FindStringSubmatch(m[1]); i != nil {
			icon = i[1]
		}
		out = append(out, frameText(m[1])+" ["+icon+"]")
	}
	return out
}

// s-profile-1, s-profile-2: «Connect account» stands in the head of the
// connections beside «Subscribe to a feed», and the list carries no form. The
// dialog is a state of its own (s-profile-2, -11, -14): the head's button
// opens it, under its own title and line, and so does Connect account at the
// foot of every rail.
func TestConnectAccountIsInTheHead(t *testing.T) {
	want := headButtons(t, "s-profile-1")
	head := frameHeadText.FindStringSubmatch(mockupFrame(t, "s-profile-2"))
	if head == nil || strings.Contains(mockupFrame(t, "s-profile-1"), `<div class="tc-form`) {
		t.Fatal("s-profile-1 draws a form, or s-profile-2 no head: read them again")
	}
	f := calendarFixture(t, nil)
	page := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	var got []string
	for _, a := range page.Actions {
		got = append(got, string(a.Label)+" ["+a.Icon+"]")
	}
	if !equalStrings(got, want) {
		t.Fatalf("head = %q, the frame %q", got, want)
	}
	if page.Add != nil {
		t.Errorf("the list carries a form: %+v", page.Add)
	}
	connect := page.Actions[0]
	if connect.ID != "connection.add" || connect.Method != "GET" || !connect.Enabled {
		t.Fatalf("Connect account = %+v", connect)
	}
	dialog := decode[view.ConnectionsPage](t, f.h.do(get(connect.Href, f.sess)))
	if dialog.Add == nil || actionByID(dialog.Add.Actions, "connection.add") == nil {
		t.Fatalf("%s opens no dialog: %+v", connect.Href, dialog.Add)
	}
	// The dialog stands over the page, with a head of its own (s-profile-16).
	if string(dialog.DialogTitle) != head[1] || string(dialog.DialogSubtitle) != head[2] {
		t.Errorf("dialog head = %q · %q, the frame %q · %q", dialog.DialogTitle, dialog.DialogSubtitle, head[1], head[2])
	}
	rail := decode[struct {
		Rail view.Rail `json:"rail"`
	}](t, f.h.do(get("/app/calendar", f.sess))).Rail
	if a := actionByID(rail.Foot, "account.connect"); a == nil || a.Href != connect.Href {
		t.Errorf("the rail's Connect account = %+v, want %s", a, connect.Href)
	}
}

var frameServerHint = regexp.MustCompile(`(?s)<label>On which server</label>.*?<span class="tc-field__hint[^"]*">(.*?)</span></div>`)

// s-profile-5, s-profile-13: «On which server» lists only the servers that
// answer now; one known not to answer is left out, and the hint under the
// field names it. What is known is what this session last heard: the last
// check of the connection, else the last poll of its collections — no
// request is sent to find out when the sheet opens.
func TestNewCollectionListsOnlyServersThatAnswer(t *testing.T) {
	hint := frameServerHint.FindStringSubmatch(mockupFrame(t, "s-profile-5"))
	if hint == nil || !strings.Contains(frameText(hint[1]), "Two of three are listed: baikal ") {
		t.Fatalf("s-profile-5 hint = %v: read it again", hint)
	}
	f := contactsFixture(t, false)
	office := davtest.New(t)
	office.Login, office.Password = "bob", "secret"
	team := davtest.AddressBookHome + "team/"
	office.AddAddressBook(carddav.AddressBook{Path: team, Name: "Team"})
	if rec := f.h.do(postForm("/app/settings/connections", f.sess, url.Values{
		"label": {"office"}, "server": {office.URL}, "login": {"bob"}, "password": {"secret"},
	})); rec.Code != http.StatusSeeOther {
		t.Fatalf("connect office: %d %s", rec.Code, rec.Body)
	}
	blob := catalogOf(t, f)
	officeID := blob.Connections[1].ID
	tickRail(t, f.h, f.sess, "contacts", f.path("contacts"), "/app/contacts/"+officeID+"/"+url.PathEscape(team))
	office.Refuse("REPORT", http.StatusUnauthorized, "")
	combined(t, f.h, f.sess, "/app/contacts", contactsPoll)

	servers := func() (view.FormRow, []string) {
		sheet := decode[view.CollectionNewSheet](t, f.h.do(get("/app/collections/new?kind=contact", f.sess)))
		row := rowNamed(t, sheet.Form, "account")
		var ids []string
		for _, o := range row.Options {
			ids = append(ids, o.Value)
		}
		return row, ids
	}
	row, ids := servers()
	want := strings.Replace(strings.Replace(frameText(hint[1]), "Two of three are", "One of two is", 1), "baikal", "office", 1)
	// In pieces, the server's name in mono (Кл3): read as the frame's words.
	said := ""
	for _, r := range row.HintRuns {
		said += string(r.Text)
	}
	if !equalStrings(ids, []string{f.id}) || said != want || len(row.HintRuns) != 3 || row.HintRuns[1].Mark != view.RunMono {
		t.Errorf("silent office: options %v, hint %+v\nwant %v, %q", ids, row.HintRuns, []string{f.id}, want)
	}

	office.Accept("REPORT")
	combined(t, f.h, f.sess, "/app/contacts", contactsPoll)
	if row, ids = servers(); len(ids) != 2 || row.Hint != "" || len(row.HintRuns) != 0 {
		t.Errorf("office answers again: options %v, hint %q %+v", ids, row.Hint, row.HintRuns)
	}
}

// s-profile-1: the «⋯» of a calendar in the connections is Rename and
// colour, Move and Copy to another server, Export .ics, Publish a link, and
// under a line Delete on the server… — icons as drawn. Export hands the
// calendar over whole, Publish opens the sheet of S-25.
func TestCollectionMenuIsTheFrame(t *testing.T) {
	f := newColMoveFixture(t)
	page := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	row := connectionRow(t, page, f.id, f.col)
	sameMenu(t, "calendar", row.Menu, "s-profile-1")
	rec := f.h.do(get(menuHref(t, row.Menu, "collection.export"), f.sess))
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/calendar") ||
		!strings.Contains(rec.Body.String(), "UID:report") {
		t.Errorf("Export .ics: %d %q, the task in it %v", rec.Code, rec.Header().Get("Content-Type"), strings.Contains(rec.Body.String(), "UID:report"))
	}
	sheet := decode[view.PublishSheet](t, f.h.do(get(menuHref(t, row.Menu, "publish"), f.sess)))
	if len(sheet.Actions) == 0 || sheet.Actions[0].ID != "publish.create" {
		t.Errorf("Publish a link opens %+v", sheet)
	}

	book := contactsFixture(t, false)
	page = decode[view.ConnectionsPage](t, book.h.do(get("/app/settings/connections", book.sess)))
	for _, m := range connectionRow(t, page, book.id, book.col).Menu {
		if m.ID == "collection.export" && m.Label != "Export .vcf" {
			t.Errorf("an address book's export = %q", m.Label)
		}
	}
}

// fileStorage connects a plain WebDAV storage the way discovery finds one
// without a principal (S-6): its address is the storage, and the folders
// under davtest.FileRoot are its collections, each seeded with one file.
func fileStorage(t *testing.T, f davFixture, label string, folders ...string) (*davtest.Server, string) {
	t.Helper()
	srv := davtest.New(t)
	srv.Login, srv.Password = "carol", "secret"
	var cols []sealed.Collection
	for _, name := range folders {
		srv.SeedFile(davtest.FileRoot+name+"/readme.txt", "in "+name)
		cols = append(cols, sealed.Collection{Path: davtest.FileRoot + name + "/", Name: name, Kinds: []view.Kind{view.KindFile}})
	}
	key, _ := f.sess.DataKey()
	id, err := f.h.users.AddConnection(f.sess.AccountID(), key, sealed.Connection{
		Label: label, Server: srv.URL, Login: srv.Login, Password: srv.Password, FileRoot: srv.URL,
	}, cols)
	if err != nil {
		t.Fatal(err)
	}
	return srv, id
}

// s-profile-12: a folder of a file storage has Rename, Move and Copy to
// another server, Publish a link and under a line Delete on the server…, and
// each does what S-12.1 says of a folder: renaming is MOVE, deleting is
// DELETE, and another server gets the folder as a tree, the way files go.
// The whole storage — the address the connection was made with — has no
// name of its own to change and is not deleted from here.
func TestFolderMenuIsTheFrame(t *testing.T) {
	f := calendarFixture(t, nil)
	a, aID := fileStorage(t, f, "vps", "Docs", "Pics")
	b, bID := fileStorage(t, f, "nas", "Archive")
	docs, pics := davtest.FileRoot+"Docs/", davtest.FileRoot+"Pics/"
	tickRail(t, f.h, f.sess, "files", "/app/files/"+aID+"/"+url.PathEscape(docs)+"/")
	editBlob(t, f.h, f.sess, func(blob *sealed.Blob) {
		blob.Attachments = &sealed.AttachmentFolder{AccountID: aID, Path: docs, Folder: []string{"carrel"}}
		blob.Collections = append(blob.Collections, sealed.Collection{AccountID: bID, Path: "/", Name: "/", Kinds: []view.Kind{view.KindFile}})
	})
	page := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	row := connectionRow(t, page, aID, docs)
	sameMenu(t, "folder", row.Menu, "s-profile-12")
	for _, m := range connectionRow(t, page, bID, "/").Menu {
		if (m.ID == "collection.rename" || m.ID == "collection.delete") && (m.Enabled || m.Reason == "") {
			t.Errorf("the whole storage: %s = %+v", m.ID, m.Action)
		}
	}
	if sheet := decode[view.PublishSheet](t, f.h.do(get(menuHref(t, row.Menu, "publish"), f.sess))); len(sheet.Actions) == 0 {
		t.Errorf("Publish a link opens %+v", sheet)
	}

	// Rename: MOVE beside it; the tick and the attachment folder follow.
	rename := menuHref(t, row.Menu, "collection.rename")
	if rec := f.h.do(postForm(rename, f.sess, url.Values{"name": {"Papers"}})); rec.Code != http.StatusSeeOther {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body)
	}
	papers := davtest.FileRoot + "Papers/"
	if got, ok := a.File(papers + "readme.txt"); !ok || got != "in Docs" || a.Names(docs) != nil {
		t.Errorf("after rename the server holds Papers %v, Docs %v", ok, a.Names(docs))
	}
	blob := catalogOf(t, f)
	if _, ok := blob.Collection(aID, papers); !ok || blob.Attachments == nil || blob.Attachments.Path != papers ||
		!strings.Contains(strings.Join(selected(t, f.h, f.sess, view.KindFile), " "), papers) {
		t.Errorf("after rename: catalog %+v, attachments %+v, ticks %v", blob.CollectionsOf(aID), blob.Attachments, selected(t, f.h, f.sess, view.KindFile))
	}

	// Copy to another server: the tree arrives, the original stays.
	cp := decode[view.CollectionMoveSheet](t, f.h.do(get("/app/collections/move?"+url.Values{"a": {aID}, "c": {pics}, "to": {bID}, "copy": {"1"}}.Encode(), f.sess)))
	if len(cp.Actions) != 2 || !cp.Actions[0].Enabled || len(cp.Warning) == 0 {
		t.Fatalf("copy sheet = %+v", cp)
	}
	if rec := f.h.do(postForm(cp.Actions[0].Href, f.sess, url.Values{})); rec.Code != http.StatusSeeOther {
		t.Fatalf("copy: %d %s", rec.Code, rec.Body)
	}
	if got, ok := b.File(pics + "readme.txt"); !ok || got != "in Pics" || a.Names(pics) == nil {
		t.Errorf("after copy: on nas %q %v, on vps %v", got, ok, a.Names(pics))
	}

	// Move to another server: the copy is whole, then the original goes.
	mv := decode[view.CollectionMoveSheet](t, f.h.do(get("/app/collections/move?"+url.Values{"a": {aID}, "c": {papers}, "to": {bID}}.Encode(), f.sess)))
	if rec := f.h.do(postForm(mv.Actions[0].Href, f.sess, url.Values{})); rec.Code != http.StatusSeeOther {
		t.Fatalf("move: %d %s", rec.Code, rec.Body)
	}
	if got, ok := b.File(papers + "readme.txt"); !ok || got != "in Docs" || a.Names(papers) != nil {
		t.Errorf("after move: on nas %q %v, on vps %v", got, ok, a.Names(papers))
	}
	blob = catalogOf(t, f)
	if _, ok := blob.Collection(aID, papers); ok || blob.Attachments == nil || blob.Attachments.AccountID != bID {
		t.Errorf("after move: vps still lists Papers %v, attachments %+v", ok, blob.Attachments)
	}

	// Delete on the server: the typed name, then DELETE of the folder.
	del := "/app/collections/delete?" + url.Values{"a": {bID}, "c": {davtest.FileRoot + "Archive/"}}.Encode()
	sheet := decode[view.CollectionDeleteSheet](t, f.h.do(get(del, f.sess)))
	if sheet.Export == nil || sheet.Export.Label != "Open in Files" {
		t.Errorf("a folder's delete sheet offers %+v", sheet.Export)
	}
	if rec := f.h.do(postForm(del, f.sess, url.Values{"name": {"Archive"}})); rec.Code != http.StatusSeeOther {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	if b.Names(davtest.FileRoot+"Archive/") != nil {
		t.Error("Archive is still on nas")
	}
	if _, ok := catalogOf(t, f).Collection(bID, davtest.FileRoot+"Archive/"); ok {
		t.Error("Archive is still in the catalog")
	}
}

var (
	frameAttachFolder = regexp.MustCompile(`(?s)<div class="tc-rubric">Folder</div>(.*?)<div class="tc-fset">`)
	frameFieldPart    = regexp.MustCompile(`(?s)<div class="tc-field[^"]*">(.*?)<span class="tc-field__hint">(.*?)</span></div>`)
)

// attachments-1, attachments-2: the folder for attachments is a Collection —
// a list of the file collections, a read-only one shown with its mark and
// not taken — then «Folder in it» with Browse, then «A subfolder per month»,
// each with the frame's hint. The month is a setting, and a large attachment
// goes into the subfolder of its month (S-20).
func TestAttachmentsFolderIsTheFrame(t *testing.T) {
	fset := frameAttachFolder.FindStringSubmatch(mockupFrame(t, "attachments-1"))
	if fset == nil {
		t.Fatal("attachments-1 draws no Folder rubric: read it again")
	}
	type drawn struct{ label, hint string }
	var want []drawn
	for _, m := range frameFieldPart.FindAllStringSubmatch(fset[1], -1) {
		label := regexp.MustCompile(`<label>([^<]+)</label>`).FindStringSubmatch(m[1])
		if label == nil {
			label = regexp.MustCompile(`class="tc-check">(?:<input[^>]*>)?<span[^>]*></span>([^<]+)</label>`).FindStringSubmatch(m[1])
		}
		if label == nil {
			t.Fatalf("a field of attachments-1 without a label: %s", m[1])
		}
		// A tag closed right before a comma leaves a space there in the text.
		want = append(want, drawn{label[1], strings.NewReplacer(" ,", ",", " .", ".").Replace(frameText(m[2]))})
	}
	browse := frameText(frameButton.FindStringSubmatch(want2(fset[1], "Folder in it"))[1])
	if len(want) != 3 || browse != "Browse" {
		t.Fatalf("attachments-1 Folder = %+v, button %q: read it again", want, browse)
	}

	f := attachFixture(t, 1024, []string{"carrel"}, "e1.ics", attachEvent("e1", "Sync-up", ""))
	f.h.srv.SetClock(time.Date(2026, 8, 16, 9, 41, 0, 0, time.UTC))
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		b.Collections = append(b.Collections, sealed.Collection{AccountID: f.id, Path: davtest.FileRoot + "public/", Name: "Public", Kinds: []view.Kind{view.KindFile}, ReadOnly: true})
	})
	page := decode[view.AttachmentsPage](t, f.h.do(get("/app/settings/attachments", f.sess)))
	var set *view.FieldSet
	for i := range page.Form.Sets {
		if page.Form.Sets[i].Legend == "Folder" {
			set = &page.Form.Sets[i]
		}
	}
	if set == nil {
		t.Fatalf("no set Folder: %+v", page.Form.Sets)
	}
	var got []drawn
	for _, r := range set.Rows {
		if r.Kind != view.InputHidden {
			got = append(got, drawn{string(r.Label), string(r.Hint)})
		}
	}
	// The frame's day is the clock's, so its months are the hint's.
	if len(got) != len(want) {
		t.Fatalf("Folder = %+v\nwant %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %+v, the frame %+v", i, got[i], want[i])
		}
	}
	collection, folder, monthly := set.Rows[1], set.Rows[2], set.Rows[3]
	if collection.Kind != view.InputSelect || len(collection.Options) != 2 {
		t.Fatalf("Collection = %+v", collection)
	}
	for _, o := range collection.Options {
		ro := strings.HasSuffix(string(o.Label), "Public")
		if o.Colour == "" || o.Disabled != ro || (o.Reason != "") != ro || o.Selected == ro {
			t.Errorf("option %+v", o)
		}
	}
	if folder.Kind != view.InputText || folder.Value != "/carrel" || folder.Extra == nil || folder.Extra.Label != view.Text(browse) || !folder.Extra.Enabled {
		t.Errorf("Folder in it = %+v, extra %+v", folder, folder.Extra)
	}
	if monthly.Kind != view.InputCheck || monthly.Value != "" {
		t.Errorf("A subfolder per month = %+v", monthly)
	}

	// Browse opens the one tree; choosing a folder there fills the form.
	sheet := decode[view.FilePickerSheet](t, f.h.do(get(folder.Extra.Href, f.sess)))
	choose := actionByID(sheet.Actions, "files.picker.choose")
	if choose == nil || !choose.Enabled || choose.Method != "GET" || !strings.Contains(sheet.Tree.Chosen, "/carrel/") {
		t.Fatalf("Browse = %+v, chosen %q", sheet.Actions, sheet.Tree.Chosen)
	}

	form := formFields(page.Form)
	form.Set("monthly", "true")
	if rec := f.h.do(postForm("/app/settings/attachments", f.sess, form)); rec.Code != http.StatusSeeOther {
		t.Fatalf("save: %d %s", rec.Code, rec.Body)
	}
	if af := catalogOf(t, f).Attachments; af == nil || !af.Monthly || strings.Join(af.Folder, "/") != "carrel" {
		t.Fatalf("stored %+v", af)
	}
	href := actionHref(t, eventAt(t, f, "e1").Actions, "attach.add")
	if resp := f.h.do(uploadRequest(href, f.sess, false, "big.pdf", make([]byte, 2048))); resp.Code != http.StatusSeeOther {
		t.Fatalf("attach: %d %s", resp.Code, resp.Body)
	}
	if _, ok := f.srv.File(attachFiles + "carrel/2026-08/2026-08-10 big.pdf"); !ok {
		t.Errorf("the month's folder holds %+v", folderRows(t, f, "carrel"))
	}

	ro := formFields(page.Form)
	ro.Set("collection", collection.Options[1].Value)
	if again := decode[view.AttachmentsPage](t, f.h.do(postForm("/app/settings/attachments", f.sess, ro))); len(again.Form.Errors) == 0 {
		t.Error("a read-only collection was taken")
	}
}

// attachAt is the attachments form choosing the folder a href of the files
// section names: its collection, and the folder in it.
func attachAt(t *testing.T, href string) url.Values {
	t.Helper()
	parts := strings.SplitN(strings.TrimPrefix(href, "/app/files/"), "/", 3)
	if len(parts) != 3 {
		t.Fatalf("no folder in %s", href)
	}
	col, err := url.PathUnescape(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var segs []string
	for _, s := range strings.Split(strings.Trim(parts[2], "/"), "/") {
		d, _ := url.PathUnescape(s)
		segs = append(segs, d)
	}
	return url.Values{"collection": {parts[0] + "|" + col}, "folder": {strings.Join(segs, "/")}}
}

// want2 is the part of a fieldset from the field labelled label on.
func want2(fset, label string) string {
	at := strings.Index(fset, "<label>"+label+"</label>")
	if at < 0 {
		return ""
	}
	return fset[at:]
}

var frameCardActs = regexp.MustCompile(`(?s)<div class="tc-card">\s*<div class="tc-rubric">([^<]+)<span class="tc-line tc-line--gap-6">(.*?)</span>\s*</div>`)

// s-profile-1: the card of a plain file storage carries «New folder» first in
// its head, where a DAV server's carries «New collection». It opens the same
// sheet as New collection, for a folder: the server, the name; the folder is
// made beside the storage's folders and joins the catalog (S-12.1).
func TestFileStorageCardHasNewFolder(t *testing.T) {
	var want string
	for _, m := range frameCardActs.FindAllStringSubmatch(mockupFrame(t, "s-profile-1"), -1) {
		first := frameButton.FindStringSubmatch(m[2])
		if first == nil {
			continue
		}
		if label := frameText(first[1]); strings.HasPrefix(label, "New folder") {
			icon := frameIcon.FindStringSubmatch(first[1])
			want = label + " [" + icon[1] + "]"
		}
	}
	if want == "" {
		t.Fatal("s-profile-1 draws no New folder in a card head: read it again")
	}
	f := calendarFixture(t, nil)
	srv, id := fileStorage(t, f, "vps", "Docs")
	page := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	var nf *view.Action
	for _, card := range page.Accounts {
		for i, a := range card.Actions {
			if a.Label == "New folder" {
				if card.AccountID != id {
					t.Errorf("New folder on %s, a DAV server", card.Title)
					continue
				}
				nf = &card.Actions[i]
			}
		}
	}
	if nf == nil || string(nf.Label)+" ["+nf.Icon+"]" != want || !nf.Enabled || nf.Method != "GET" {
		t.Fatalf("New folder = %+v, the frame %q", nf, want)
	}
	sheet := decode[view.CollectionNewSheet](t, f.h.do(get(nf.Href, f.sess)))
	if sheet.Kind != view.KindFile || rowNamed(t, sheet.Form, "account").Value != view.Text(id) {
		t.Fatalf("sheet = %+v", sheet)
	}
	form := formFields(sheet.Form)
	form.Set("name", "Scans")
	rec := f.h.do(postForm(sheet.Form.Action, f.sess, form))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	scans := davtest.FileRoot + "Scans/"
	if srv.Names(scans) == nil {
		t.Error("no Scans on the storage")
	}
	if col, ok := catalogOf(t, f).Collection(id, scans); !ok || !col.Has(view.KindFile) || col.Name != "Scans" {
		t.Errorf("catalog = %+v", catalogOf(t, f).CollectionsOf(id))
	}
	form.Set("name", "Docs")
	if again := decode[view.CollectionNewSheet](t, f.h.do(postForm(sheet.Form.Action, f.sess, form))); rowNamed(t, again.Form, "name").Error == "" {
		t.Error("a folder that is there already was taken")
	}
}

var (
	frameNowSet    = regexp.MustCompile(`(?s)<div class="tc-rubric">In this folder now</div>\s*<dl[^>]*>(.*?)</dl>\s*<div class="tc-field tc-grow"><span class="tc-field__hint">(.*?)</span></div>`)
	frameDT        = regexp.MustCompile(`<dt>([^<]+)</dt>`)
	frameFormMsg   = regexp.MustCompile(`(?s)<div class="tc-msg">(.*?)</div>`)
	frameWrittenAs = regexp.MustCompile(`(?s)<label>Written as</label>.*?<span class="tc-field__hint">(.*?)</span></div>`)
)

// attachments-1: under the file name, «In this folder now» — Files (count
// and size), Oldest, Path (the folder's whole address, to mount without
// Carrel) — and what Carrel does not do with them; the message under the
// form is the frame's; the hint of «Written as» is the frame's, example and all, and
// the queue does say it: a name taken in the folder is a row of the files
// queue naming what the file was written as.
func TestAttachmentsInThisFolderNowIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "attachments-1")
	now := frameNowSet.FindStringSubmatch(frame)
	msg := frameFormMsg.FindStringSubmatch(frame)
	written := frameWrittenAs.FindStringSubmatch(frame)
	if now == nil || msg == nil || written == nil {
		t.Fatal("attachments-1 changed: read it again")
	}
	var labels []string
	for _, m := range frameDT.FindAllStringSubmatch(now[1], -1) {
		labels = append(labels, m[1])
	}

	f := attachFixture(t, 1024, []string{"carrel"}, "e1.ics", attachEvent("e1", "Sync-up", ""))
	f.h.srv.SetClock(time.Date(2026, 8, 16, 9, 41, 0, 0, time.UTC))
	f.srv.SeedFile(attachFiles+"carrel/a.pdf", strings.Repeat("x", 1000))
	f.srv.SetModified(attachFiles+"carrel/a.pdf", time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC))
	f.srv.SeedFile(attachFiles+"carrel/2026-07/b.md", strings.Repeat("y", 48))
	f.srv.SetModified(attachFiles+"carrel/2026-07/b.md", time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC))

	page := decode[view.AttachmentsPage](t, f.h.do(get("/app/settings/attachments", f.sess)))
	var set *view.FieldSet
	for i := range page.Form.Sets {
		if page.Form.Sets[i].Legend == "In this folder now" {
			set = &page.Form.Sets[i]
		}
	}
	if set == nil || len(set.Rows) != len(labels) {
		t.Fatalf("In this folder now = %+v, the frame %q", set, labels)
	}
	for i, l := range labels {
		if r := set.Rows[i]; string(r.Label) != l || !r.ReadOnly {
			t.Errorf("row %d = %+v, the frame %q", i, r, l)
		}
	}
	if v := string(set.Rows[0].Value); !strings.HasPrefix(v, "2 · ") || !strings.HasSuffix(v, "KB") {
		t.Errorf("Files = %q, want two files and their size", v)
	}
	if set.Rows[1].Value != "2026-07-01" {
		t.Errorf("Oldest = %q", set.Rows[1].Value)
	}
	if want := f.srv.URL + attachFiles + "carrel/"; string(set.Rows[2].Value) != want {
		t.Errorf("Path = %q, want %q", set.Rows[2].Value, want)
	}
	if hint := string(set.Rows[len(set.Rows)-1].Hint); hint != frameText(now[2]) {
		t.Errorf("hint = %q, the frame %q", hint, frameText(now[2]))
	}
	if string(page.Note) != frameText(msg[1]) {
		t.Errorf("note = %q, the frame %q", page.Note, frameText(msg[1]))
	}
	// The example is set in mono inside the hint: its closing tag is not a
	// word break.
	if h, want := string(rowNamed(t, page.Form, "name").Hint), frameText(strings.ReplaceAll(written[1], "</span>", "")); h != want {
		t.Errorf("Written as hint = %q, the frame %q", h, want)
	}

	for range 2 {
		href := actionHref(t, eventAt(t, f, "e1").Actions, "attach.add")
		if resp := f.h.do(uploadRequest(href, f.sess, false, "big.pdf", make([]byte, 2048))); resp.Code != http.StatusSeeOther {
			t.Fatalf("attach: %d %s", resp.Code, resp.Body)
		}
	}
	folder := "/app/files/" + f.id + "/" + url.PathEscape(attachFiles) + "/carrel/"
	q := decode[view.FilesPage](t, f.h.do(get(folder, f.sess))).Queue
	if q == nil || !strings.Contains(string(q.Note), "2026-08-10 big-2.pdf") {
		t.Errorf("the queue says %+v", q)
	}
}
