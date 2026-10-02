package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// From-typecase note 13, the small things, each read off its frame: the
// birthday typed as text (s-contacts-3, s-contacts-27), the buttons under the
// duplicate weights (s-dup-3), the download of a public page named by its
// file (s-publish-6, -7, -8), the Shortcuts of the files rail (s-files-1).

var (
	frameBirthday = regexp.MustCompile(`(?s)<label>Birthday</label><input ([^>]*)><span class="tc-field__hint">(.*?)</span>`)
	frameAttr     = regexp.MustCompile(`(\w+)="([^"]*)"`)
	frameMenuFoot = regexp.MustCompile(`(?s)<div class="tc-menu__foot">(.*?)</div>`)
	frameChanged  = regexp.MustCompile(`(?s)<div class="tc-menu__item">([^<]*?)\s*<span class="tc-badge">changed</span><span[^>]*>(\d+)</span><input[^>]*value="(\d+)"`)
	frameDownload = regexp.MustCompile(`</svg>(Download[^<]*)<`)
	frameShortcut = regexp.MustCompile(`(?s)<div class="(is-on )?tc-tree__row">(?:<svg><use href="#([^"]+)"/></svg>)?([^<]*)<span class="tc-tickrow__meta[^"]*">(.*?)</span></div>`)
)

type frameField struct{ kind, value, placeholder, hint string }

func readFrameBirthday(t *testing.T, id string) frameField {
	t.Helper()
	m := frameBirthday.FindStringSubmatch(mockupFrame(t, id))
	if m == nil {
		t.Fatalf("%s draws no Birthday: read it again", id)
	}
	attrs := map[string]string{}
	for _, a := range frameAttr.FindAllStringSubmatch(m[1], -1) {
		attrs[a[1]] = a[2]
	}
	return frameField{kind: attrs["type"], value: attrs["value"], placeholder: attrs["placeholder"], hint: frameInline(m[2])}
}

// Anna's birthday is filled in every edit (from-typecase note 32, task 81):
// the empty field is the new card's (s-contacts-27).
func TestBirthdayIsTypedAsTheFrameDrawsIt(t *testing.T) {
	filled, empty := readFrameBirthday(t, "s-contacts-3"), readFrameBirthday(t, "s-contacts-27")
	if again := readFrameBirthday(t, "s-contacts-4"); again != filled {
		t.Fatalf("s-contacts-4 draws Anna's birthday %+v, s-contacts-3 %+v: read the frames again", again, filled)
	}
	examples := regexp.MustCompile(`^Type it: (\S+), or (\S+) without the year\.$`).FindStringSubmatch(filled.hint)
	if examples == nil || filled.value != examples[1] || empty.placeholder == "" {
		t.Fatalf("hint %q, value %q, placeholder %q: the frames changed, read them again", filled.hint, filled.value, empty.placeholder)
	}
	f := contactsFixture(t, false,
		"BEGIN:VCARD\nVERSION:4.0\nUID:anna\nFN:Anna\nBDAY:19870314\nEND:VCARD\n",
		"BEGIN:VCARD\nVERSION:4.0\nUID:vera\nFN:Vera\nEND:VCARD\n")
	anna, vera := f.path("contacts")+"/anna", f.path("contacts")+"/vera"
	check := func(rec string, want frameField) {
		t.Helper()
		row := rowNamed(t, decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form, "bday")
		if string(row.Kind) != want.kind || string(row.Value) != want.value || string(row.Placeholder) != want.placeholder || string(row.Hint) != want.hint {
			t.Errorf("%s: birthday = kind %q value %q placeholder %q hint %q\nthe frame %+v", rec, row.Kind, row.Value, row.Placeholder, row.Hint, want)
		}
	}
	// The frame of the filled field draws no placeholder, and the field has
	// one all the same: it is the empty field's.
	check(anna, frameField{filled.kind, filled.value, empty.placeholder, filled.hint})
	check(vera, empty)

	save := func(rec, typed string) int {
		t.Helper()
		form := formFields(decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
		form.Set("bday", typed)
		return f.h.do(postForm(rec+"/edit", f.sess, form)).Code
	}
	for _, c := range []struct{ typed, line string }{
		{examples[2], "BDAY:--0314"}, // as the hint says: without the year
		{"15.03.1987", "BDAY:19870315"},
		{"1987-03-16", "BDAY:19870316"}, // the form of before still goes
		{"--03-17", "BDAY:--0317"},
	} {
		if code := save(anna, c.typed); code != http.StatusSeeOther {
			t.Fatalf("%s: %d", c.typed, code)
		}
		mustContain(t, body(t, f, "anna.vcf"), c.line)
	}
	puts := f.srv.CountRequests("PUT", f.col)
	form := formFields(decode[view.ContactEditPage](t, f.h.do(get(anna+"/edit", f.sess))).Form)
	form.Set("bday", "31.02.1987")
	page := decode[view.ContactEditPage](t, f.h.do(postForm(anna+"/edit", f.sess, form)))
	if len(page.Form.Errors) == 0 || page.Form.Errors[0].Name != "bday" || !strings.Contains(string(page.Form.Errors[0].Message), examples[1]) {
		t.Errorf("errors = %+v, want bday with the frame's example", page.Form.Errors)
	}
	if f.srv.CountRequests("PUT", f.col) != puts {
		t.Error("a day that does not exist was written")
	}
}

// s-dup-3: under the weights, Save and Back to defaults; a weight of the
// person's own is «changed».
func TestWeightButtonsAreTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-dup-3")
	foot := frameMenuFoot.FindStringSubmatch(frame)
	changed := frameChanged.FindStringSubmatch(frame)
	if foot == nil || changed == nil {
		t.Fatal("s-dup-3 draws no buttons or no changed weight: read it again")
	}
	want := frameAll(frameButton, foot[1], 1)
	label := frameInline(changed[1])
	def, _ := strconv.Atoi(changed[2])
	yours, _ := strconv.Atoi(changed[3])

	f := newDupFixture(t)
	f.load(t)
	p := f.page(t)
	var got []string
	for _, a := range p.Threshold.WeightActions {
		got = append(got, string(a.Label))
	}
	if !equalStrings(got, want) {
		t.Fatalf("buttons = %v, the frame %v", got, want)
	}
	sameActionMarks(t, "s-dup-3", "weights", p.Threshold.WeightActions, nil)
	save, reset := p.Threshold.WeightActions[0], p.Threshold.WeightActions[1]
	if !save.Enabled || save.Method != "POST" || reset.Enabled || reset.Reason == "" {
		t.Errorf("before any choice: save %+v, back to defaults %+v", save, reset)
	}
	var signal string
	for _, w := range p.Threshold.Weights {
		if string(w.Label) == label {
			signal = w.Signal
			if w.Default != def {
				t.Errorf("%s: default %d, the frame %d", label, w.Default, def)
			}
		}
	}
	if signal == "" {
		t.Fatalf("no weight %q", label)
	}
	if rec := f.h.do(postForm(save.Href, f.sess, url.Values{"weight." + signal: {strconv.Itoa(yours)}})); rec.Code != http.StatusSeeOther {
		t.Fatalf("save: %d", rec.Code)
	}
	p = f.page(t)
	for _, w := range p.Threshold.Weights {
		if (w.Signal == signal) != w.Own || w.Signal == signal && w.Value != yours {
			t.Errorf("after save: %+v", w)
		}
	}
	reset = p.Threshold.WeightActions[1]
	if !reset.Enabled || reset.Method != "POST" {
		t.Fatalf("back to defaults = %+v", reset)
	}
	if rec := f.h.do(postForm(reset.Href, f.sess, url.Values{})); rec.Code != http.StatusSeeOther {
		t.Fatalf("back to defaults: %d", rec.Code)
	}
	for _, w := range f.page(t).Threshold.Weights {
		if w.Own || w.Value != w.Default {
			t.Errorf("after back to defaults: %+v", w)
		}
	}
}

// s-publish-6, -7, -8: the download of a public page says which file it is.
func TestPublicDownloadNamesItsFile(t *testing.T) {
	f := publishFixture(t)
	for _, c := range []struct{ frame, segment, col, uid string }{
		{"s-publish-6", "calendar", searchWork, ""},
		{"s-publish-7", "contacts", searchBook, ""},
		{"s-publish-8", "notes", searchWork, "n1"},
	} {
		m := frameDownload.FindStringSubmatch(mockupFrame(t, c.frame))
		if m == nil {
			t.Fatalf("%s draws no download: read it again", c.frame)
		}
		page := publicPage(t, f, publishLink(t, f, f.publishHref(c.segment, c.col, c.uid), false))
		dl := actionByID(page.Actions, "published.download")
		if dl == nil || string(dl.Label) != m[1] {
			t.Errorf("%s: download = %+v, the frame %q", c.frame, dl, m[1])
		}
	}
}

// s-files-1, s-files-6: the files rail ends with Shortcuts — the attachment
// folder with where it lies, Published with the count of links — on every
// files screen, the open one marked.
func TestFilesRailShortcutsAreTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-files-1")
	at := strings.Index(frame, `<div class="tc-rubric">Shortcuts</div>`)
	if at < 0 {
		t.Fatal("s-files-1 draws no Shortcuts: read it again")
	}
	rows := frameShortcut.FindAllStringSubmatch(frame[at:], 2)
	if len(rows) != 2 || rows[0][1] == "" {
		t.Fatalf("Shortcuts = %v: the frame changed, read it again", rows)
	}
	attach, published := frameInline(rows[0][3]), frameInline(rows[1][3])
	// «Documents / carrel»: the storage and the folders the attachment folder
	// lies in, the folder itself being the row.
	place := strings.Split(frameInline(rows[0][4]), " / ")

	f := filesFixture(t, true)
	segs := append(place[1:], strings.ToLower(attach))
	folder := f.at("", append(append([]string{}, segs...), "")...)
	f.srv.SeedFolder(filesDocs + strings.Join(segs, "/") + "/")
	if rec := f.h.do(postForm("/app/settings/attachments", f.sess, attachAt(t, folder))); rec.Code != http.StatusSeeOther {
		t.Fatalf("attachment folder: %d %s", rec.Code, rec.Body)
	}
	publishLink(t, f, f.publishHref("contacts", searchBook, ""), false)

	check := func(where string, rail view.Rail, on bool) {
		t.Helper()
		if len(rail.Shortcuts) != 2 {
			t.Fatalf("%s: shortcuts = %+v", where, rail.Shortcuts)
		}
		a, p := rail.Shortcuts[0], rail.Shortcuts[1]
		if string(a.Label) != attach || a.Icon != rows[0][2] || a.Href != folder || string(a.Place) != "Docs / "+strings.Join(place[1:], " / ") || a.Count != nil || a.On != on {
			t.Errorf("%s: %s = %+v", where, attach, a)
		}
		if string(p.Label) != published || p.Icon != rows[1][2] || p.Href != "/app/files/published" || p.Count == nil || *p.Count != 1 || p.Place != "" {
			t.Errorf("%s: %s = %+v", where, published, p)
		}
	}
	check("root", decode[view.FilesRootPage](t, f.h.do(get("/app/files", f.sess))).Rail, false)
	check("attachment folder", decode[view.FilesPage](t, f.h.do(get(folder, f.sess))).Rail, true)
	check("storage", decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess))).Rail, false)
	if rail := decode[view.DuplicatesPage](t, f.h.do(get("/app/duplicates", f.sess))).Rail; len(rail.Shortcuts) != 0 {
		t.Errorf("duplicates have shortcuts: %+v", rail.Shortcuts)
	}
}

// s-files-1: the foot of the files rail is New folder, Connect account and
// Subscribe to a feed. Apply is the rail's own submit where there are no
// scripts (C-6.3) and is not drawn.
func TestFilesRailFootIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-files-1")
	foot := regexp.MustCompile(`(?s)<div class="tc-railfoot">(.*?)<a class="tc-railfoot__about"`).FindStringSubmatch(frame)
	if foot == nil {
		t.Fatal("s-files-1 draws no rail foot: read it again")
	}
	var want []string
	for _, m := range regexp.MustCompile(`title="([^"]*)"`).FindAllStringSubmatch(foot[1], -1) {
		want = append(want, m[1])
	}
	f := filesFixture(t, true)
	f.srv.SeedFolder(filesDocs + "carrel/")
	labels := func(rail view.Rail) (out []string, newFolder view.Action) {
		for _, a := range rail.Foot {
			if a.ID == "rail.apply" {
				continue
			}
			out = append(out, string(a.Label))
			if a.Label == "New folder" {
				newFolder = a
			}
		}
		return out, newFolder
	}
	got, nf := labels(decode[view.FilesPage](t, f.h.do(get(f.at("", "carrel", ""), f.sess))).Rail)
	if !equalStrings(got, want) {
		t.Fatalf("foot = %v, the frame %v", got, want)
	}
	if !nf.Enabled || nf.Method != "POST" || nf.Href != f.at("mkcol", "carrel", "") {
		t.Errorf("New folder in a folder = %+v", nf)
	}
	got, nf = labels(decode[view.FilesRootPage](t, f.h.do(get("/app/files", f.sess))).Rail)
	if !equalStrings(got, want) || nf.Enabled || nf.Reason == "" {
		t.Errorf("root: foot %v, New folder %+v", got, nf)
	}
}
