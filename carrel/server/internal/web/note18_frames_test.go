package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// From-typecase note 18 (Typecase 0.7.2 answered to-typecase tasks 22–25):
// the sheets of move and copy, From Files, a server that asks no password
// and the rewrite of an address, each string cut out of its frame.

var (
	frameDimLine   = regexp.MustCompile(`(?s)<span class="tc-dim--muted[^"]*">(.*?)</span>`)
	frameAlertMsg  = regexp.MustCompile(`(?s)<div class="tc-msg is-alert[^"]*">(.*?)</div>`)
	frameAnyMsg    = regexp.MustCompile(`(?s)<div class="tc-msg[^"]*">(?:<b>(.*?)</b>)?(.*?)</div>`)
	frameCardLine  = regexp.MustCompile(`(?s)<div class="tc-card__line">(.*?)</div>`)
	frameStackItem = regexp.MustCompile(`(?s)<span class="tc-stack__item"><b>(.*?)</b> <span class="tc-dim">— (.*?)</span></span>`)
)

// s-files-11, s-files-12: the copy sheet is the move sheet with its own
// message and «Copy here»; a move to another storage says so instead of the
// message of one server. One file has the same head: «Move 1 item».
func TestFileCopyAndOtherStorageAreTheFrames(t *testing.T) {
	cpFrame := mockupFrame(t, "s-files-11")
	cpTitle := frameOne(t, frameSheetT, cpFrame, "title")[1]
	cpMsg := frameOne(t, frameSheetMsg, cpFrame, "message")[1]
	cpConfirm := frameOne(t, frameSheetMain, cpFrame, "main button")[1]
	farMsg := frameOne(t, regexp.MustCompile(`(?s)<div class="tc-msg">(.*?)</div>`), mockupFrame(t, "s-files-12"), "message")[1]
	moveTitle := frameOne(t, frameSheetT, mockupFrame(t, "s-files-4"), "title")[1]
	n := regexp.MustCompile(`^Copy (\d+) items$`).FindStringSubmatch(cpTitle)
	if n == nil || !strings.HasPrefix(moveTitle, "Move "+n[1]+" items") {
		t.Fatalf("titles %q, %q: the frames changed, read them again", cpTitle, moveTitle)
	}

	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a.txt", "alpha")
	far := searchServer(t)
	far.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	far.SeedFolder(filesDocs)
	farID := addAccount(t, f, far)
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		b.Collections = append(b.Collections, sealed.Collection{AccountID: farID, Path: filesDocs, Name: "Far", Kinds: []view.Kind{view.KindFile}})
	})
	farRoot := "/app/files/" + farID + "/" + url.PathEscape(filesDocs) + "/"
	op := func(verb string) string {
		return f.at("op", "") + "?" + url.Values{"op": {verb}, "item": {"a.txt"}}.Encode()
	}

	cp := opSheet(t, f, op("copy"), url.Values{"to": {farRoot}})
	if want := strings.Replace(cpTitle, n[1]+" items", "1 item", 1); string(cp.Title) != want {
		t.Errorf("copy title = %q, want %q", cp.Title, want)
	}
	if plainText(cp.Warning) != cpMsg || cp.Actions[0].Label != view.Text(cpConfirm) {
		t.Errorf("copy: message %q, confirm %+v\nthe frame %q, %q", cp.Warning, cp.Actions[0], cpMsg, cpConfirm)
	}
	sameActionMarks(t, "s-files-11", "copy", cp.Actions, nil)
	mv := opSheet(t, f, op("move"), url.Values{"to": {farRoot}})
	if want := strings.Replace(moveTitle, n[1]+" items", "1 item", 1); string(mv.Title) != want {
		t.Errorf("move title = %q, want %q", mv.Title, want)
	}
	if plainText(mv.Warning) != farMsg {
		t.Errorf("move to another storage: %q\nthe frame %q", mv.Warning, farMsg)
	}
	sameActionMarks(t, "s-files-4", "move", mv.Actions, nil)
}

// s-files-13: New folder asks the name in a line over the tree — the field,
// «Create», Cancel — and a refusal stands under the line, in the frame's
// words, with the line open and the name typed.
func TestNewFolderLineIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-files-13")
	lineM := regexp.MustCompile(`(?s)<div class="tc-line tc-line--gap-8">(.*?)</div>`).FindStringSubmatch(frame)
	if lineM == nil {
		t.Fatal("s-files-13 draws no line: read it again")
	}
	line := lineM[1]
	field := frameOne(t, regexp.MustCompile(`<input [^>]*value="([^"]*)" aria-label="([^"]*)">`), line, "field")
	buttons := frameAll(frameButton, line, 1)
	refusal := frameOne(t, frameAlertMsg, frame, "refusal")[1]
	typed, label := field[1], field[2]
	in := regexp.MustCompile(`in (\S+) already`).FindStringSubmatch(refusal)
	if len(buttons) != 2 || in == nil || !strings.Contains(refusal, "«"+typed+"»") {
		t.Fatalf("buttons %q, refusal %q: the frame changed, read it again", buttons, refusal)
	}

	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a.txt", "alpha")
	f.srv.SeedFolder(filesDocs + in[1] + "/" + typed + "/")
	sheet := opSheet(t, f, f.at("op", "")+"?"+url.Values{"op": {"move"}, "item": {"a.txt"}}.Encode(), url.Values{"to": {f.at("", in[1], "")}})
	if sheet.Target == nil || sheet.Target.New == nil || !sheet.Target.New.Enabled || sheet.Target.Name != nil {
		t.Fatalf("tree = %+v, want New folder and its line closed", sheet.Target)
	}
	// New opens the line: the same sheet with new=1 (ФГ4).
	tree := decode[view.FileOpSheet](t, f.h.do(get(sheet.Target.New.Href, f.sess))).Target
	if tree == nil || tree.Name == nil {
		t.Fatalf("tree = %+v, want New folder with its line", tree)
	}
	sameCaret(t, "s-files-13", "new folder", *tree.Name)
	row := visibleRows(*tree.Name)
	if len(row) != 1 || row[0].Name != "name" || string(row[0].Label) != label || row[0].Value != "" {
		t.Errorf("the line's field = %+v, the frame %q", row, label)
	}
	var got []string
	for _, a := range tree.Name.Actions {
		got = append(got, string(a.Label))
	}
	if !equalStrings(got, buttons) || !strings.Contains(tree.Name.Action, "mkcol=1") || tree.Name.Method != "POST" {
		t.Errorf("the line = %+v, the frame's buttons %q", *tree.Name, buttons)
	}

	taken := opSheet(t, f, tree.Name.Action, url.Values{"name": {typed}})
	if taken.Failure == nil || taken.Failure.Title != "" || string(taken.Failure.Body) != refusal || taken.Failure.Role != view.NoticeAlert {
		t.Errorf("refusal = %+v\nthe frame %q", taken.Failure, refusal)
	}
	if taken.Target == nil || taken.Target.Name == nil || string(visibleRows(*taken.Target.Name)[0].Value) != typed {
		t.Errorf("after the refusal the line is %+v, want it open with %q", taken.Target, typed)
	} else {
		sameCaret(t, "s-files-13", "new folder refused", *taken.Target.Name)
	}
}

// s-files-6: on the root no folder is open, and New folder of the rail's
// foot is off with the frame's reason.
func TestRootNewFolderIsOffAsTheFrameDraws(t *testing.T) {
	foot := regexp.MustCompile(`(?s)<div class="tc-railfoot">(.*?)<a class="tc-railfoot__about"`).FindStringSubmatch(mockupFrame(t, "s-files-6"))
	if foot == nil {
		t.Fatal("s-files-6 draws no rail foot: read it again")
	}
	first := frameOne(t, regexp.MustCompile(`<button class="tc-btn" title="([^"]*)" type="button"( disabled)?>`), foot[1], "button")
	if first[2] == "" {
		t.Fatalf("s-files-6 draws New folder on: read it again")
	}
	f := filesFixture(t, true)
	for _, a := range decode[view.FilesRootPage](t, f.h.do(get("/app/files", f.sess))).Rail.Foot {
		if a.ID == "rail.folder.new" && (a.Enabled || string(a.Reason) != first[1]) {
			t.Errorf("New folder on the root = %+v, the frame off with %q", a, first[1])
		}
	}
}

// s-files-18: without an attachment folder the shortcut stays and says «not
// set»; it leads where the folder is chosen.
func TestAttachmentsShortcutWithoutAFolder(t *testing.T) {
	rows := frameShortcut.FindAllStringSubmatch(mockupFrame(t, "s-files-18"), -1)
	if len(rows) == 0 {
		t.Fatal("s-files-18 draws no shortcut: read it again")
	}
	notSet := frameInline(rows[0][4])
	f := filesFixture(t, true)
	a := decode[view.FilesRootPage](t, f.h.do(get("/app/files", f.sess))).Rail.Shortcuts[0]
	if string(a.Label) != frameInline(rows[0][3]) || string(a.Place) != notSet || a.Href != "/app/settings/attachments" {
		t.Errorf("Attachments without a folder = %+v, the frame %q", a, notSet)
	}
}

// s-dup-7: with no weight of one's own, Back to defaults is off with the
// frame's reason.
func TestBackToDefaultsIsOffAsTheFrameDraws(t *testing.T) {
	foot := frameMenuFoot.FindStringSubmatch(mockupFrame(t, "s-dup-7"))
	m := regexp.MustCompile(`<button class="tc-btn is-sm" title="([^"]*)" type="button" disabled>([^<]*)</button>`).FindStringSubmatch(foot[1])
	if m == nil {
		t.Fatal("s-dup-7 draws no disabled Back to defaults: read it again")
	}
	f := newDupFixture(t)
	f.load(t)
	reset := f.page(t).Threshold.WeightForm.Actions[1]
	if string(reset.Label) != m[2] || reset.Enabled || string(reset.Reason) != m[1] {
		t.Errorf("Back to defaults = %+v, the frame %q off with %q", reset, m[2], m[1])
	}
	sameActionMarks(t, "s-dup-7", "weights", f.page(t).Threshold.WeightForm.Actions, nil)
}

// s-contacts-18: a day that is not there is refused under the field in the
// frame's words.
func TestBirthdayErrorIsTheFrame(t *testing.T) {
	m := regexp.MustCompile(`(?s)<label>Birthday</label><input [^>]*value="([^"]*)"[^>]*><span class="tc-field__hint tc-dim--danger">(.*?)</span>`).FindStringSubmatch(mockupFrame(t, "s-contacts-18"))
	if m == nil {
		t.Fatal("s-contacts-18 draws no birthday error: read it again")
	}
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:anna\nFN:Anna\nEND:VCARD\n")
	anna := f.path("contacts") + "/anna"
	form := formFields(decode[view.ContactEditPage](t, f.h.do(get(anna+"/edit", f.sess))).Form)
	form.Set("bday", m[1])
	page := decode[view.ContactEditPage](t, f.h.do(postForm(anna+"/edit", f.sess, form)))
	if len(page.Form.Errors) == 0 || page.Form.Errors[0].Name != "bday" || string(page.Form.Errors[0].Message) != frameInline(m[2]) {
		t.Errorf("errors = %+v, the frame %q", page.Form.Errors, frameInline(m[2]))
	}
}

// s-contacts-17: copying a contact is the move sheet with «Copy contact»,
// the message of a copy and «Copy here».
func TestRecordCopySheetIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-contacts-17")
	title := frameOne(t, frameSheetT, frame, "title")[1]
	msg := frameOne(t, frameSheetMsg, frame, "message")[1]
	confirm := frameOne(t, frameSheetMain, frame, "main button")[1]
	f := newMoveFixture(t)
	sheet := decode[view.MoveSheet](t, f.h.do(get(f.record(f.id, peopleBook, "ada")+"/move", f.sess)))
	cp := decode[view.MoveSheet](t, f.h.do(get(moveOption(t, sheet, "Friends").ID+"&copy=1", f.sess)))
	if string(cp.Title) != title || plainText(cp.Warning) != msg || confirmOf(t, cp).Label != view.Text(confirm) {
		t.Errorf("copy = %q, %q, %+v\nthe frame %q, %q, %q", cp.Title, cp.Warning, confirmOf(t, cp), title, msg, confirm)
	}
	sameActionMarks(t, "s-contacts-17", "copy", cp.Actions, nil)
}

// s-publish-16: the public page of one event gives its own file.
func TestPublicRecordDownloadIsTheFrame(t *testing.T) {
	m := frameDownload.FindStringSubmatch(mockupFrame(t, "s-publish-16"))
	if m == nil {
		m = regexp.MustCompile(`<button class="tc-btn" type="button">(Download[^<]*)</button>`).FindStringSubmatch(mockupFrame(t, "s-publish-16"))
	}
	if m == nil {
		t.Fatal("s-publish-16 draws no download: read it again")
	}
	f := publishFixture(t)
	page := publicPage(t, f, publishLink(t, f, f.publishHref("calendar", searchWork, "e1"), false))
	if dl := actionByID(page.Actions, "published.download"); dl == nil || string(dl.Label) != m[1] {
		t.Errorf("download = %+v, the frame %q", dl, m[1])
	}
}

// s-files-16: From Files is a sheet with a head — «Attach from Files», to
// which record — and the files of a folder with their size and whether each
// goes inside or as a link, by the rule of an upload. Attach is off, with the
// frame's reason, until a file is chosen; the tree makes no folders.
func TestFromFilesSheetIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-files-16")
	title := frameOne(t, frameSheetT, frame, "title")[1]
	to := regexp.MustCompile(`^to «.*»$`)
	subject := frameOne(t, frameSheetSub, frame, "subject")[1]
	reason := frameOne(t, regexp.MustCompile(`(?s)<button class="tc-btn is-primary" type="button" disabled>(?:<svg.*?</svg>)?[^<]*</button><span class="tc-dim[^"]*">(.*?)</span>`), frame, "reason")[1]
	words := map[string]bool{}
	for _, m := range regexp.MustCompile(`<div class="tc-tree__row[^"]*">(?:<svg>.*?</svg>)?[^<]* <span class="tc-push tc-dim tc-dim--t11">([^<]*) · (\w+)</span></div>`).FindAllStringSubmatch(frame, -1) {
		words[m[2]] = true
	}
	if !to.MatchString(subject) || len(words) != 2 {
		t.Fatalf("subject %q, words %v: the frame changed, read it again", subject, words)
	}

	f := attachFixture(t, 1024, []string{"carrel"}, "e1.ics", attachEvent("e1", "Sync-up", ""))
	f.srv.SeedFile(attachFiles+"notes.txt", "small file")
	f.srv.SeedFile(attachFiles+"big.pdf", strings.Repeat("x", 2048))
	href := menuAction(actionByID(eventAt(t, f, "e1").Actions, "attach.add").Arrow, "attach.files").Href
	sheet := decode[view.FilePickerSheet](t, f.h.do(get(href+"&to="+url.QueryEscape("/app/files/"+f.id+"/"+url.PathEscape(attachFiles)+"/"), f.sess)))
	if string(sheet.Title) != title || string(sheet.Subject) != "to «Sync-up»" {
		t.Errorf("head = %q, %q; the frame %q, %q", sheet.Title, sheet.Subject, title, subject)
	}
	if a := actionByID(sheet.Actions, "files.picker.attach"); a == nil || a.Enabled || string(a.Reason) != reason {
		t.Errorf("Attach = %+v, the frame off with %q", a, reason)
	}
	sameActionMarks(t, "s-files-16", "from files", sheet.Actions, nil)
	if sheet.Tree.New != nil || sheet.Tree.Name != nil {
		t.Errorf("the tree makes folders: %+v", sheet.Tree.New)
	}
	lands := map[string]string{}
	size := regexp.MustCompile(`^\d+(\.\d)? (B|KB|MB)$`)
	var walk func([]view.FileTreeNode)
	walk = func(nodes []view.FileTreeNode) {
		for _, n := range nodes {
			if n.File {
				lands[n.Label] = string(n.Lands)
				if !size.MatchString(string(n.Size)) || !words[string(n.Lands)] {
					t.Errorf("%s: size %q, lands %q; the frame's words %v", n.Label, n.Size, n.Lands, words)
				}
			} else if n.Size != "" || n.Lands != "" {
				t.Errorf("folder %s has %q, %q", n.Label, n.Size, n.Lands)
			}
			walk(n.Children)
		}
	}
	walk(sheet.Tree.Nodes)
	if lands["notes.txt"] != "inside" || lands["big.pdf"] != "link" {
		t.Errorf("lands = %v, want notes.txt inside and big.pdf as a link", lands)
	}
}

// s-files-17: the two refusals of From Files are messages on the card, in
// the frame's words.
func TestFromFilesRefusalsAreTheFrame(t *testing.T) {
	msgs := frameAnyMsg.FindAllStringSubmatch(mockupFrame(t, "s-files-17"), -1)
	if len(msgs) != 2 {
		t.Fatalf("s-files-17 draws %d messages: read it again", len(msgs))
	}
	f := attachFixture(t, 0, nil, "e1.ics", attachEvent("e1", "Sync-up", ""))
	f.srv.SeedFile(attachFiles+"notes.txt", "small file")
	sheet := fromFiles(t, f, actionByID(eventAt(t, f, "e1").Actions, "attach.add").Arrow, "", "notes.txt")
	confirm := actionByID(sheet.Actions, "files.picker.attach").Href
	with := func(file string) view.EventPage {
		u, _ := url.Parse(confirm)
		q := u.Query()
		q.Set("file", file)
		u.RawQuery = q.Encode()
		return decode[view.EventPage](t, f.h.do(postForm(u.String(), f.sess, url.Values{})))
	}
	for i, page := range []view.EventPage{with(""), with("/app/files/" + f.id + "/" + url.PathEscape(attachFiles) + "/gone.txt")} {
		title, body := frameInline(msgs[i][1]), frameInline(msgs[i][2])
		if len(page.Notices) == 0 || string(page.Notices[0].Title) != title || string(page.Notices[0].Body) != body || page.Notices[0].Role != view.NoticeAlert {
			t.Errorf("notices = %+v\nthe frame %q %q", page.Notices, title, body)
		}
	}
}

// s-files-15: in a read-only folder Upload and New folder stand off with one
// reason, and the line under the title says it.
func TestReadOnlyFolderHeadIsTheFrame(t *testing.T) {
	sub := frameInline(frameOne(t, frameHeadSub, mockupFrame(t, "s-files-15"), "line under the title")[1])
	_, tail, ok := strings.Cut(sub, " · read-only")
	if !ok {
		t.Fatalf("line %q: the frame changed, read it again", sub)
	}
	tail = "read-only" + tail

	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a.txt", "alpha")
	title := string(decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess))).Accounts[0].Title)
	if page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess))); string(page.Subtitle) != title {
		t.Errorf("subtitle = %q, want %q", page.Subtitle, title)
	}
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		for i := range b.Collections {
			if b.Collections[i].Path == filesDocs {
				b.Collections[i].ReadOnly = true
			}
		}
	})
	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	if string(page.Subtitle) != title || string(page.Access) != tail {
		t.Errorf("subtitle = %q, access %q; want %q and %q", page.Subtitle, page.Access, title, tail)
	}
	up, nf := actionByID(page.Bar.Actions, "file.upload"), actionByID(page.Bar.Actions, "folder.new")
	if up == nil || nf == nil || up.Enabled || nf.Enabled || up.Reason == "" || up.Reason != nf.Reason {
		t.Fatalf("Upload %+v, New folder %+v: want both off with one reason", up, nf)
	}
	for _, m := range up.Arrow {
		if m.Enabled || m.Reason != up.Reason {
			t.Errorf("arrow item %+v", m)
		}
	}
}

// s-profile-15: a connection that asks no password has no Show password,
// and its address line says «no password».
func TestNoPasswordCardIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-profile-15")
	parts := strings.Split(frameInline(frameOne(t, frameCardLine, frame, "address line")[1]), " · ")
	buttons := frameAll(frameButton, frame, 1)
	if len(parts) < 3 || parts[2] != "no password" || len(buttons) == 0 {
		t.Fatalf("line %q, buttons %q: the frame changed, read it again", parts, buttons)
	}
	srv := noPasswordServer(t)
	h, sess := davHarness(t)
	if rec := h.do(postForm("/app/settings/connections", sess, url.Values{"server": {srv.URL}, "login": {"mix"}, "noPassword": {"on"}})); rec.Code != http.StatusSeeOther {
		t.Fatalf("connect: %d %s", rec.Code, rec.Body)
	}
	card := decode[view.ConnectionsPage](t, h.do(get("/app/settings/connections", sess))).Accounts[0]
	if card.Password != nil || string(card.Access) != parts[2] {
		t.Errorf("password %+v, access %q; the frame has no Show password and says %q", card.Password, card.Access, parts[2])
	}
	for _, a := range card.Actions {
		if a.Label == "Show password" {
			t.Errorf("the card has %+v", a)
		}
	}

	// A connection with a password keeps its button and says nothing.
	f := contactsFixture(t, false)
	card = decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess))).Accounts[0]
	if card.Password == nil || card.Password.Reveal.Label != "Show password" || card.Access != "" {
		t.Errorf("a card with a password = %+v, %q", card.Password, card.Access)
	}
}

// s-profile-16: the edit of such a connection has no hint at the password
// field; s-profile-2, -11, -14, -16: Certificate fingerprint is the optional
// field under the password, in the frame's words.
func TestNoPasswordEditAndFingerprintAreTheFrames(t *testing.T) {
	type drawn struct{ label, aside, placeholder string }
	fingerprint := func(id string) drawn {
		m := regexp.MustCompile(`(?s)<label>(Certificate fingerprint.*?)</label><input [^>]*placeholder="([^"]*)"`).FindStringSubmatch(mockupFrame(t, id))
		if m == nil {
			t.Fatalf("%s draws no Certificate fingerprint: read it again", id)
		}
		label, aside := splitLabel(m[1])
		return drawn{label, aside, m[2]}
	}
	want := fingerprint("s-profile-16")
	for _, id := range []string{"s-profile-2", "s-profile-11", "s-profile-14"} {
		if got := fingerprint(id); got != want {
			t.Fatalf("%s draws %+v, s-profile-16 %+v: read them again", id, got, want)
		}
	}
	edit, _, _ := strings.Cut(mockupFrame(t, "s-profile-16"), "<figcaption")
	if strings.Contains(edit, "Leave it empty") || !regexp.MustCompile(`aria-label="Password" disabled`).MatchString(edit) {
		t.Fatal("s-profile-16 changed: read it again")
	}

	srv := noPasswordServer(t)
	h, sess := davHarness(t)
	h.do(postForm("/app/settings/connections", sess, url.Values{"server": {srv.URL}, "login": {"mix"}, "noPassword": {"on"}}))
	id := decode[view.ConnectionsPage](t, h.do(get("/app/settings/connections", sess))).Accounts[0].AccountID
	for _, href := range []string{"/app/settings/connections?op=add", "/app/settings/connections?" + url.Values{"op": {"edit"}, "account": {id}}.Encode()} {
		rows := visibleRows(*decode[view.ConnectionsPage](t, h.do(get(href, sess))).Add)
		for i, r := range rows {
			if r.Name != "certFingerprint" {
				continue
			}
			if string(r.Label) != want.label || string(r.LabelAside) != want.aside || string(r.Placeholder) != want.placeholder || r.Required || i == 0 || rows[i-1].Disables != "password" {
				t.Errorf("%s: fingerprint = %+v after %+v, the frame %+v", href, r, rows[i-1], want)
			}
		}
	}
	password := formRow(*decode[view.ConnectionsPage](t, h.do(get("/app/settings/connections?"+url.Values{"op": {"edit"}, "account": {id}}.Encode(), sess))).Add, "password")
	if password.Hint != "" || !password.ReadOnly {
		t.Errorf("the edit's password = %+v, want it off and without a hint", password)
	}
}

// rewriteFrame is a rewrite sheet of a frame: the pairs' notes, the rows
// under «What rewriting can set off» and «What will not change», the line
// under them, the main button and the message over it.
type rewriteFrame struct {
	pairs, effects, kept [][2]string
	note, confirm, msg   string
}

func readRewriteFrame(t *testing.T, id string) rewriteFrame {
	t.Helper()
	frame := mockupFrame(t, id)
	var r rewriteFrame
	for _, block := range strings.Split(frame, `<div class="tc-sheet__block">`)[1:] {
		var rows [][2]string
		for _, m := range frameStackItem.FindAllStringSubmatch(block, -1) {
			rows = append(rows, [2]string{frameInline(m[1]), frameInline(m[2])})
		}
		switch {
		case strings.Contains(block, ">The address<"):
			r.pairs = rows
		case strings.Contains(block, ">What rewriting can set off<"):
			r.effects = rows
		case strings.Contains(block, ">What will not change<"):
			r.kept = rows
		}
	}
	if m := frameDimLine.FindStringSubmatch(frame); m != nil {
		r.note = frameInline(m[1])
	}
	if m := frameAlertMsg.FindStringSubmatch(frame); m != nil {
		r.msg = frameInline(m[1])
	}
	r.confirm = frameOne(t, regexp.MustCompile(`(?s)<div class="tc-sheet__foot"><button class="tc-btn is-primary"[^>]*>(.*?)</button>`), frame, "main button")[1]
	if len(r.pairs) != 1 {
		t.Fatalf("%s: pairs %q — read the frame again", id, r.pairs)
	}
	return r
}

// rewriteLone is Ada's card on a server that schedules, with calendars on it
// that name her old address as seeded, and a second account, ticked, whose
// server refuses the password.
func rewriteLone(t *testing.T, seed func(*davtest.Server)) (rewriteStand, string) {
	t.Helper()
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	srv.ScheduleOutbox = davtest.Principal + "outbox/"
	srv.UserAddresses = []string{"mailto:alice@example.org"}
	srv.AddAddressBook(carddav.AddressBook{Path: peopleBook, Name: "People"})
	srv.AddCalendar(caldav.Calendar{Path: rewriteWork, Name: "Work", SupportedComponentSet: []string{"VEVENT", "VTODO"}})
	srv.AddCalendar(caldav.Calendar{Path: rewriteShared, Name: "Shared", SupportedComponentSet: []string{"VEVENT"}, ReadOnly: true})
	srv.Seed(peopleBook+"ada.vcf", rewriteCard)
	seed(srv)
	f := connectFixture(t, srv, peopleBook, nil)
	far := davtest.New(t)
	far.Login, far.Password = "bob", "other"
	far.AddCalendar(caldav.Calendar{Path: rewriteWork, Name: "Common", SupportedComponentSet: []string{"VEVENT"}})
	farID := addAccount(t, f, far)
	far.Password = "changed" // the next read is refused: 401
	key, _ := f.sess.DataKey()
	sel := []sealed.Selection{{Kind: view.KindEvent, AccountID: f.id, Path: rewriteWork}, {Kind: view.KindEvent, AccountID: f.id, Path: rewriteShared},
		{Kind: view.KindEvent, AccountID: farID, Path: rewriteWork}}
	if err := f.h.users.SetSelection(f.sess.AccountID(), key, view.KindEvent, sel); err != nil {
		t.Fatal(err)
	}
	conns := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	return rewriteStand{davFixture: f, card: f.path("contacts") + "/ada"}, "Common · " + string(cardOf(t, conns, farID).Title)
}

// s-contacts-19: one record in the singular throughout; a ticked collection
// that did not answer stands under «What will not change» with the reason;
// the line under the blocks says the unticked ones were not looked in.
func TestRewriteOfOneRecordIsTheFrame(t *testing.T) {
	fr := readRewriteFrame(t, "s-contacts-19")
	if len(fr.effects) != 1 || len(fr.kept) != 1 || fr.note == "" {
		t.Fatalf("frame = %+v: read it again", fr)
	}
	s, unread := rewriteLone(t, func(srv *davtest.Server) {
		srv.Seed(rewriteWork+"mine.ics", meeting("mine", "alice@example.org", "20990105T100000Z"))
	})
	resp := s.save(t, func(f url.Values) { f.Set("email.0", "ada@new.example") })
	sheet := decode[view.RewriteSheet](t, s.h.do(get(resp.Header.Get("Location"), s.sess)))
	if len(sheet.Pairs) != 1 || string(sheet.Pairs[0].Note) != fr.pairs[0][1] {
		t.Errorf("pair = %+v, the frame %q", sheet.Pairs, fr.pairs[0][1])
	}
	if !sameFields(sheet.Effects, []view.Field{{Label: view.Text(fr.effects[0][0]), Value: view.Text(fr.effects[0][1])}}) {
		t.Errorf("effects = %+v, the frame %q", sheet.Effects, fr.effects)
	}
	if !sameFields(sheet.Kept, []view.Field{{Label: view.Text(unread), Value: view.Text(fr.kept[0][1])}}) {
		t.Errorf("kept = %+v, the frame %q", sheet.Kept, fr.kept)
	}
	if string(sheet.Note) != fr.note {
		t.Errorf("note = %q, the frame %q", sheet.Note, fr.note)
	}
	if c := rewriteAction(t, sheet.Actions, "contact.rewrite.confirm"); string(c.Label) != fr.confirm || !c.Enabled {
		t.Errorf("confirm = %+v, the frame %q", c, fr.confirm)
	}
	sameActionMarks(t, "s-contacts-19", "rewrite", sheet.Actions, nil)
}

// s-contacts-20: every record that names the old address is read-only or
// could not be read — nothing to rewrite, the pair's note without what
// rewriting mends, «Rewrite 0 records» off with the frame's message as its
// reason.
func TestNothingToRewriteIsTheFrame(t *testing.T) {
	fr := readRewriteFrame(t, "s-contacts-20")
	n := regexp.MustCompile(`^(\d+) events and tasks name the old one$`).FindStringSubmatch(fr.pairs[0][1])
	if n == nil || fr.msg == "" || len(fr.effects) != 0 {
		t.Fatalf("frame = %+v: read it again", fr)
	}
	s, _ := rewriteLone(t, func(srv *davtest.Server) {
		srv.Seed(rewriteShared+"a.ics", meeting("a", "boss@example.org", "20990108T100000Z"))
		srv.Seed(rewriteShared+"b.ics", meeting("b", "boss@example.org", "20990109T100000Z"))
	})
	resp := s.save(t, func(f url.Values) { f.Set("email.0", "ada@new.example") })
	sheet := decode[view.RewriteSheet](t, s.h.do(get(resp.Header.Get("Location"), s.sess)))
	// Only what was read is counted: the two records of Shared, not the
	// collection that did not answer (from-typecase note 19).
	if want := fr.pairs[0][1]; n[1] != "2" || len(sheet.Pairs) != 1 || string(sheet.Pairs[0].Note) != want {
		t.Errorf("pair = %+v, want %q", sheet.Pairs, want)
	}
	if len(sheet.Effects) != 0 || len(sheet.Kept) != 2 {
		t.Errorf("effects %+v, kept %+v", sheet.Effects, sheet.Kept)
	}
	if c := rewriteAction(t, sheet.Actions, "contact.rewrite.confirm"); string(c.Label) != fr.confirm || c.Enabled || string(c.Reason) != fr.msg {
		t.Errorf("confirm = %+v, the frame %q off with %q", c, fr.confirm, fr.msg)
	}
}

// s-contacts-21: an outcome without refusals is a plain message that counts
// what was rewritten.
func TestRewriteOutcomeWithoutRefusalsIsTheFrame(t *testing.T) {
	msgs := frameAnyMsg.FindAllStringSubmatch(mockupFrame(t, "s-contacts-21"), -1)
	if len(msgs) == 0 || !strings.Contains(msgs[0][0], `class="tc-msg"`) || frameInline(msgs[0][2]) != "" {
		t.Fatalf("s-contacts-21 = %q: read it again", msgs)
	}
	n := regexp.MustCompile(`^Rewritten in (\d+) records\.$`).FindStringSubmatch(frameInline(msgs[0][1]))
	if n == nil {
		t.Fatalf("title %q: read the frame again", msgs[0][1])
	}
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	srv.AddAddressBook(carddav.AddressBook{Path: peopleBook, Name: "People"})
	srv.AddCalendar(caldav.Calendar{Path: rewriteWork, Name: "Work", SupportedComponentSet: []string{"VEVENT"}})
	srv.Seed(peopleBook+"ada.vcf", rewriteCard)
	srv.Seed(rewriteWork+"mine.ics", meeting("mine", "alice@example.org", "20990105T100000Z"))
	srv.Seed(rewriteWork+"next.ics", meeting("next", "alice@example.org", "20990106T100000Z"))
	f := connectFixture(t, srv, peopleBook, nil)
	key, _ := f.sess.DataKey()
	if err := f.h.users.SetSelection(f.sess.AccountID(), key, view.KindEvent, []sealed.Selection{{Kind: view.KindEvent, AccountID: f.id, Path: rewriteWork}}); err != nil {
		t.Fatal(err)
	}
	s := rewriteStand{davFixture: f, card: f.path("contacts") + "/ada"}
	resp := s.save(t, func(v url.Values) { v.Set("email.0", "ada@new.example") })
	sheet := decode[view.RewriteSheet](t, s.h.do(get(resp.Header.Get("Location"), s.sess)))
	page := decode[view.ContactPage](t, s.h.do(postForm(rewriteAction(t, sheet.Actions, "contact.rewrite.confirm").Href, s.sess, url.Values{})))
	want := strings.Replace(frameInline(msgs[0][1]), n[1], "2", 1)
	if noticeTitled(page.Notices, want) == nil {
		t.Fatalf("notices = %+v, want %q", page.Notices, want)
	}
	if got := noticeTitled(page.Notices, want); got.Role != view.NoticeInfo || got.Body != "" || got.Details != nil {
		t.Errorf("outcome = %+v", *got)
	}
}
