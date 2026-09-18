package web_test

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// dupFixture is S-16's ground: one person in two books on two servers — the
// same address and the same name, written differently, with UIDs of their
// own — and a stranger who shares nothing.
type dupFixture struct {
	moveFixture
}

const (
	adaHere  = "BEGIN:VCARD\nVERSION:4.0\nUID:ada-here\nFN:Ada Lovelace\nEMAIL;TYPE=home:ada@example.com\nTEL:+44 20 7946 0000\nX-HERE:kept\nEND:VCARD\n"
	adaThere = "BEGIN:VCARD\nVERSION:4.0\nUID:ada-there\nFN:LOVELACE Ada\nEMAIL:ADA@example.com\nEMAIL;TYPE=work:ada@engines.example\nORG:Analytical Engines\nX-THERE:theirs\nEND:VCARD\n"
)

func newDupFixture(t *testing.T) dupFixture {
	t.Helper()
	f := dupFixture{newMoveFixture(t)}
	f.srv.Seed(peopleBook+"ada-here.vcf", adaHere)
	f.srv.Seed(peopleBook+"bob.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:bob\nFN:Bob Builder\nEMAIL:bob@example.com\nEND:VCARD\n")
	f.other.Seed(workBook+"ada-there.vcf", adaThere)
	return f
}

func (f dupFixture) book(account, path string) string {
	return "/app/contacts/" + account + "/" + url.PathEscape(path)
}

// load opens both lists: what a person does before the duplicates exist.
func (f dupFixture) load(t *testing.T) {
	t.Helper()
	for _, p := range []string{f.book(f.id, peopleBook), f.book(f.otherID, workBook)} {
		if rec := f.h.do(get(p, f.sess)); rec.Code != http.StatusOK {
			t.Fatalf("%s: %d", p, rec.Code)
		}
	}
}

func (f dupFixture) requests() int { return len(f.srv.Requests()) + len(f.other.Requests()) }

func (f dupFixture) writes() int {
	n := 0
	for _, s := range []*davtest.Server{f.srv, f.other} {
		n += s.CountRequests(http.MethodPut, "") + s.CountRequests(http.MethodDelete, "")
	}
	return n
}

func (f dupFixture) page(t *testing.T) view.DuplicatesPage {
	t.Helper()
	return decode[view.DuplicatesPage](t, f.h.do(get("/app/duplicates", f.sess)))
}

func (f dupFixture) decisions(t *testing.T) sealed.Duplicates {
	t.Helper()
	key, _ := f.sess.DataKey()
	blob, _ := f.h.users.Catalog(f.sess.AccountID(), key)
	return blob.Duplicates
}

func (f dupFixture) decide(t *testing.T, a view.Action) {
	t.Helper()
	rec := f.h.do(postForm(a.Href, f.sess, url.Values{}))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/app/duplicates" {
		t.Fatalf("%s: %d %q %s", a.ID, rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
}

func onlyGroup(t *testing.T, p view.DuplicatesPage) view.DuplicateGroup {
	t.Helper()
	if len(p.Groups) != 1 {
		t.Fatalf("groups = %+v", p.Groups)
	}
	return p.Groups[0]
}

func actionOf(t *testing.T, g view.DuplicateGroup, id string) view.Action {
	t.Helper()
	for _, a := range g.Actions {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("no action %q in %+v", id, g.Actions)
	return view.Action{}
}

func navCount(t *testing.T, rail view.Rail) *int {
	t.Helper()
	for _, n := range rail.Nav {
		if n.ID == "nav.duplicates" {
			return n.Count
		}
	}
	t.Fatal("no Duplicates row in the rail")
	return nil
}

// Nothing loaded: an empty screen that says why, and not one request to any
// server — no fan-out hidden behind the page.
func TestDuplicatesNeedWhatIsLoaded(t *testing.T) {
	f := newDupFixture(t)
	before := f.requests()
	p := f.page(t)
	if len(p.Groups) != 0 || p.Bar.Loaded != 0 || !hasNotice(p.Notices, "Nothing is loaded yet") {
		t.Errorf("empty cache: %+v", p)
	}
	if n := f.requests() - before; n != 0 {
		t.Errorf("the duplicates screen asked the servers %d times", n)
	}
}

// Two lists read; the screen finds Ada among them with the reason beside the
// score, asks nobody, and counts her in the rail.
func TestDuplicatesFoundAmongLoadedLists(t *testing.T) {
	f := newDupFixture(t)
	f.load(t)
	before := f.requests()
	p := f.page(t)
	if n := f.requests() - before; n != 0 {
		t.Errorf("the duplicates screen asked the servers %d times", n)
	}
	g := onlyGroup(t, p)
	if g.Kind != view.KindContact || g.Score != 80 || len(g.Rows) != 2 || g.Decision != view.DecisionNone {
		t.Fatalf("group = %+v", g)
	}
	if strings.Join([]string{string(g.Reasons[0]), string(g.Reasons[1])}, ", ") != "same email, same name" {
		t.Errorf("reasons = %v", g.Reasons)
	}
	if p.Bar.Loaded != 4 || p.Threshold.Value != 60 || p.Threshold.Default != 60 {
		t.Errorf("bar %+v threshold %+v", p.Bar, p.Threshold)
	}
	if c := navCount(t, p.Rail); c == nil || *c != 1 {
		t.Errorf("rail count = %v", c)
	}
	if merge := actionOf(t, g, "duplicates.merge"); !merge.Enabled || !merge.Danger {
		t.Errorf("merge = %+v", merge)
	}
}

// Link writes the blob and nothing else; the combined book shows Ada once,
// with both sources; Unlink takes it back.
func TestLinkWritesNoServer(t *testing.T) {
	f := newDupFixture(t)
	f.load(t)
	g := onlyGroup(t, f.page(t))
	f.decide(t, actionOf(t, g, "duplicates.link"))
	if n := f.writes(); n != 0 {
		t.Fatalf("linking wrote to a server %d times", n)
	}
	d := f.decisions(t)
	if len(d.Linked) != 1 || len(d.Linked[0].Members) != 2 || d.Linked[0].ID != g.ID {
		t.Fatalf("blob = %+v", d)
	}
	linked := onlyGroup(t, f.page(t))
	if linked.Decision != view.DecisionLinked || linked.Score != 80 {
		t.Errorf("linked group = %+v", linked)
	}
	if c := navCount(t, f.page(t).Rail); c == nil || *c != 0 {
		t.Errorf("a linked group is still counted: %v", c)
	}

	tickRail(t, f.h, f.sess, "contacts", f.book(f.id, peopleBook), f.book(f.otherID, workBook))
	book := combined(t, f.h, f.sess, "/app/contacts", contactsPoll)
	var adas []view.ContactRow
	for _, grp := range book.Groups {
		for _, r := range grp.Rows {
			if strings.Contains(strings.ToLower(string(r.Name)), "lovelace") {
				adas = append(adas, r)
			}
		}
	}
	if len(adas) != 1 || adas[0].State != view.RecordLinked || len(adas[0].Sources) != 2 || len(adas[0].Badges) != 1 {
		t.Fatalf("combined rows for Ada = %+v", adas)
	}
	if adas[0].Phone == "" || adas[0].Org != "Analytical Engines" {
		t.Errorf("the linked row does not carry the group's fields: %+v", adas[0])
	}

	f.decide(t, actionOf(t, linked, "duplicates.unlink"))
	if d := f.decisions(t); len(d.Linked) != 0 {
		t.Errorf("unlink left %+v", d.Linked)
	}
	if again := onlyGroup(t, f.page(t)); again.Decision != view.DecisionNone {
		t.Errorf("after unlink = %+v", again)
	}
	if n := f.writes(); n != 0 {
		t.Errorf("unlinking wrote to a server %d times", n)
	}
}

// Not duplicates is kept in the blob and taken back where it was made; the
// fragment answer is the group as it now stands.
func TestNotDuplicatesIsRemembered(t *testing.T) {
	f := newDupFixture(t)
	f.load(t)
	g := onlyGroup(t, f.page(t))
	rec := f.h.do(fragment(postForm(actionOf(t, g, "duplicates.ignore").Href, f.sess, url.Values{}), "main"))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"decision":"ignored"`) {
		t.Fatalf("fragment: %d %s", rec.Code, rec.Body.String())
	}
	if d := f.decisions(t); len(d.Ignored) != 1 || d.Ignored[0] != g.ID {
		t.Fatalf("blob = %+v", d)
	}
	p := f.page(t)
	ignored := onlyGroup(t, p)
	if ignored.Decision != view.DecisionIgnored || *navCount(t, p.Rail) != 0 {
		t.Errorf("ignored = %+v", ignored)
	}
	f.decide(t, actionOf(t, ignored, "duplicates.again"))
	if again := onlyGroup(t, f.page(t)); again.Decision != view.DecisionNone {
		t.Errorf("offered again = %+v", again)
	}
}

// The person's own threshold lives in the blob; an empty value is the
// instance's again.
func TestThresholdIsThePersons(t *testing.T) {
	f := newDupFixture(t)
	f.load(t)
	set := func(v string) {
		rec := f.h.do(postForm("/app/duplicates/decide", f.sess, url.Values{"do": {"threshold"}, "value": {v}}))
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("threshold %q: %d", v, rec.Code)
		}
	}
	set("81")
	p := f.page(t)
	if len(p.Groups) != 0 || p.Threshold.Value != 81 || f.decisions(t).Threshold != 81 {
		t.Errorf("at 81: groups %d threshold %+v", len(p.Groups), p.Threshold)
	}
	set("")
	if p := f.page(t); len(p.Groups) != 1 || p.Threshold.Value != 60 {
		t.Errorf("back to the instance: %+v", p.Threshold)
	}
}

// The weights are the person's, set on the duplicates screen: a signal
// turned off takes its groups away, and an empty field is the instance's
// weight again.
func TestWeightsAreThePersons(t *testing.T) {
	f := newDupFixture(t)
	f.load(t)
	p := f.page(t)
	if len(p.Threshold.Weights) != 6 {
		t.Fatalf("weights = %+v", p.Threshold.Weights)
	}
	for _, w := range p.Threshold.Weights {
		if w.Own || w.Value != w.Default || w.Label == "" {
			t.Errorf("before any choice: %+v", w)
		}
	}
	set := func(v string) {
		rec := f.h.do(postForm("/app/duplicates/decide", f.sess, url.Values{"do": {"weights"}, "weight.email": {v}}))
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("weights: %d", rec.Code)
		}
	}
	set("0")
	p = f.page(t)
	if len(p.Groups) != 0 || f.decisions(t).Weights["email"] != 0 {
		t.Errorf("email off: groups %d, blob %+v", len(p.Groups), f.decisions(t))
	}
	for _, w := range p.Threshold.Weights {
		if w.Signal == "email" && (!w.Own || w.Value != 0 || w.Default != 60) {
			t.Errorf("email = %+v", w)
		}
	}
	set("-3")
	if f.decisions(t).Weights["email"] != 0 {
		t.Error("a weight out of range was saved")
	}
	set("")
	if p := f.page(t); len(p.Groups) != 1 || len(f.decisions(t).Weights) != 0 {
		t.Errorf("back to the instance: %d groups, %+v", len(p.Groups), f.decisions(t).Weights)
	}
}

// A linked member the server no longer holds leaves its group silently once
// a list read whole says so; a server that does not answer takes nobody out.
func TestLinkedMemberGoneOnlyWhenTheListSaysSo(t *testing.T) {
	f := newDupFixture(t)
	f.load(t)
	f.decide(t, actionOf(t, onlyGroup(t, f.page(t)), "duplicates.link"))

	f.other.Refuse("REPORT", http.StatusServiceUnavailable, "down")
	f.h.do(get(f.book(f.otherID, workBook), f.sess))
	f.page(t)
	if d := f.decisions(t); len(d.Linked) != 1 {
		t.Fatalf("an unavailable server dissolved the group: %+v", d)
	}

	f.srv.RemoveCollection(peopleBook)
	f.srv.AddAddressBook(carddav.AddressBook{Path: peopleBook, Name: "People"})
	f.srv.Seed(peopleBook+"bob.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:bob\nFN:Bob Builder\nEND:VCARD\n")
	f.h.do(get(f.book(f.id, peopleBook), f.sess))
	p := f.page(t)
	if d := f.decisions(t); len(d.Linked) != 0 {
		t.Errorf("a group of one is still linked: %+v", d)
	}
	for _, n := range p.Notices {
		if n.Role == view.NoticeAlert {
			t.Errorf("a gone member is reported: %+v", n)
		}
	}
}

func mergePage(t *testing.T, f dupFixture, href string) view.DuplicatesMergePage {
	t.Helper()
	return decode[view.DuplicatesMergePage](t, f.h.do(get(href, f.sess)))
}

func mergeAction(t *testing.T, p view.DuplicatesMergePage, id string) view.Action {
	t.Helper()
	for _, a := range p.Actions {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("no action %q in %+v", id, p.Actions)
	return view.Action{}
}

// S-16 merge, in its order: the page says what is written and what deleted;
// the kept record is written with the version shown, keeps every foreign
// property of both, and only then is the other deleted with its own version.
func TestMergeWritesThenDeletes(t *testing.T) {
	f := newDupFixture(t)
	f.load(t)
	g := onlyGroup(t, f.page(t))
	p := mergePage(t, f, actionOf(t, g, "duplicates.merge").Href)
	if strings.HasSuffix(string(p.Target.Stamp), "Work") {
		// Keep the card in People: the other one is chosen from the page.
		p = mergePage(t, f, mergeAction(t, p, "duplicates.merge.into").Href)
	}
	if !strings.HasSuffix(string(p.Target.Stamp), "People") || p.Warning.Details == nil {
		t.Fatalf("merge page = %+v", p)
	}
	lines := make([]string, len(p.Warning.Details.Lines))
	for i, l := range p.Warning.Details.Lines {
		lines[i] = string(l)
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"Kept and rewritten: ", "Deleted afterwards: ", "EMAIL;TYPE=work: ada@engines.example", "X-THERE: theirs"} {
		if !strings.Contains(joined, want) {
			t.Errorf("warning lacks %q:\n%s", want, joined)
		}
	}
	var fn *view.MergeRow
	for i := range p.Rows {
		if p.Rows[i].Property == "FN" {
			fn = &p.Rows[i]
		}
	}
	if fn == nil || fn.Union || len(fn.Options) != 2 || fn.Chosen != 0 {
		t.Fatalf("FN row = %+v in %+v", fn, p.Rows)
	}
	confirm := mergeAction(t, p, "duplicates.merge.confirm")
	if !confirm.Enabled || !confirm.Danger || confirm.Method != "POST" {
		t.Fatalf("confirm = %+v", confirm)
	}
	if n := f.writes(); n != 0 {
		t.Fatalf("the merge page wrote %d times", n)
	}

	rec := f.h.do(postForm(confirm.Href, f.sess, url.Values{}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("merge: %d %s", rec.Code, rec.Body.String())
	}
	keptSrv, goneSrv, keptPath, gonePath := f.srv, f.other, peopleBook+"ada-here.vcf", workBook+"ada-there.vcf"
	body, _, ok := keptSrv.Object(keptPath)
	if !ok {
		t.Fatal("the kept record is gone")
	}
	mustContain(t, unfolded(body), "X-HERE:kept", "X-THERE:theirs", "ada@engines.example", "ORG:Analytical Engines")
	if _, _, ok := goneSrv.Object(gonePath); ok {
		t.Error("the other record was not deleted")
	}
	if keptSrv.CountRequests(http.MethodPut, "") != 1 || goneSrv.CountRequests(http.MethodDelete, "") != 1 {
		t.Errorf("PUT %d, DELETE %d", keptSrv.CountRequests(http.MethodPut, ""), goneSrv.CountRequests(http.MethodDelete, ""))
	}
}

// A record that changed after the page read it is not merged over: nothing
// is written and nothing deleted.
func TestMergeRefusesWhatChangedSinceShown(t *testing.T) {
	f := newDupFixture(t)
	f.load(t)
	p := mergePage(t, f, actionOf(t, onlyGroup(t, f.page(t)), "duplicates.merge").Href)
	confirm := mergeAction(t, p, "duplicates.merge.confirm")
	f.other.Seed(workBook+"ada-there.vcf", strings.Replace(adaThere, "X-THERE:theirs", "X-THERE:edited elsewhere", 1))
	f.srv.Seed(peopleBook+"ada-here.vcf", strings.Replace(adaHere, "X-HERE:kept", "X-HERE:edited elsewhere", 1))
	rec := f.h.do(postForm(confirm.Href, f.sess, url.Values{}))
	got := decode[view.DuplicatesMergePage](t, rec)
	if !hasNotice(got.Notices, "A record changed since it was shown") {
		t.Errorf("notices = %+v", got.Notices)
	}
	if n := f.writes(); n != 0 {
		t.Errorf("wrote %d times over a changed record", n)
	}
}

// The delete refused: the merge stays written, nothing is rolled back, and
// the page says what was deleted and what is still there.
func TestMergeStopsAtTheFirstRefusedDelete(t *testing.T) {
	f := newDupFixture(t)
	f.load(t)
	p := mergePage(t, f, actionOf(t, onlyGroup(t, f.page(t)), "duplicates.merge").Href)
	f.srv.Refuse(http.MethodDelete, http.StatusForbidden, "no")
	f.other.Refuse(http.MethodDelete, http.StatusForbidden, "no")
	got := decode[view.DuplicatesMergePage](t, f.h.do(postForm(mergeAction(t, p, "duplicates.merge.confirm").Href, f.sess, url.Values{})))
	if got.Warning.Title != "Written, but not every other record was deleted" || got.Warning.Details == nil {
		t.Fatalf("result = %+v", got.Warning)
	}
	if !strings.Contains(string(got.Warning.Details.Lines[0]), "Still there: ") {
		t.Errorf("lines = %v", got.Warning.Details.Lines)
	}
	if f.srv.CountRequests(http.MethodPut, "")+f.other.CountRequests(http.MethodPut, "") != 1 {
		t.Error("the kept record was not written, or was written back")
	}
}

// A read-only member cannot be deleted, so the group cannot be merged: said
// on the screen, not tried.
func TestMergeUnavailableWithAReadOnlyMember(t *testing.T) {
	f := newDupFixture(t)
	f.srv.Seed(archiveBook+"ada-old.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:ada-old\nFN:Ada Lovelace\nEMAIL:ada@example.com\nEND:VCARD\n")
	f.load(t)
	f.h.do(get(f.book(f.id, archiveBook), f.sess))
	merge := actionOf(t, onlyGroup(t, f.page(t)), "duplicates.merge")
	if merge.Enabled || merge.Reason == "" {
		t.Errorf("merge with a read-only member = %+v", merge)
	}
}

func optionIndex(t *testing.T, row view.MergeRow, label string) int {
	t.Helper()
	for i, o := range row.Options {
		if string(o) == label {
			return i
		}
	}
	t.Fatalf("no option %q in %+v", label, row)
	return -1
}

// S-16: members that disagree on a single-valued field are asked about once,
// under the group, and the answer goes with Link. It is kept with the linked
// group: the combined row shows the chosen value, the group shows it chosen,
// and a later merge starts from it.
func TestLinkRemembersTheAnswer(t *testing.T) {
	f := newDupFixture(t)
	f.load(t)
	g := onlyGroup(t, f.page(t))
	if len(g.Choices) != 1 || g.Choices[0].Property != "FN" || len(g.Choices[0].Options) != 2 {
		t.Fatalf("choices = %+v", g.Choices)
	}
	pick := optionIndex(t, g.Choices[0], "LOVELACE Ada")
	link := actionOf(t, g, "duplicates.link")
	rec := f.h.do(postForm(link.Href, f.sess, url.Values{"choice.FN": {strconv.Itoa(pick)}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("link: %d %s", rec.Code, rec.Body.String())
	}
	d := f.decisions(t)
	if len(d.Linked) != 1 || d.Linked[0].Prefs["FN"].UID != "ada-there" {
		t.Fatalf("answer not kept: %+v", d.Linked)
	}
	if n := f.writes(); n != 0 {
		t.Fatalf("linking with an answer wrote %d times", n)
	}

	linked := onlyGroup(t, f.page(t))
	if linked.Choices[0].Chosen != optionIndex(t, linked.Choices[0], "LOVELACE Ada") {
		t.Errorf("the linked group does not show the answer: %+v", linked.Choices)
	}
	actionOf(t, linked, "duplicates.choose")

	tickRail(t, f.h, f.sess, "contacts", f.book(f.id, peopleBook), f.book(f.otherID, workBook))
	book := combined(t, f.h, f.sess, "/app/contacts", contactsPoll)
	found := false
	for _, grp := range book.Groups {
		for _, r := range grp.Rows {
			if r.State == view.RecordLinked {
				found = true
				if r.Name != "LOVELACE Ada" {
					t.Errorf("linked row name = %q, want the chosen one", r.Name)
				}
			}
		}
	}
	if !found {
		t.Fatal("no linked row")
	}

	p := mergePage(t, f, actionOf(t, linked, "duplicates.merge").Href)
	for _, row := range p.Rows {
		if row.Property == "FN" && string(row.Options[row.Chosen]) != "LOVELACE Ada" {
			t.Errorf("merge does not start from the answer: %+v", row)
		}
	}
}
