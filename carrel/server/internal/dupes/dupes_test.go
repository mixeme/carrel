package dupes

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

func vcard(t *testing.T, lines ...string) *model.Object {
	t.Helper()
	doc := "BEGIN:VCARD\r\nVERSION:4.0\r\n" + strings.Join(lines, "\r\n") + "\r\nEND:VCARD\r\n"
	obj, err := card.Parse("/book/x.vcf", `"1"`, strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	return obj
}

func ics(t *testing.T, comp string, lines ...string) *model.Object {
	t.Helper()
	doc := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//t//EN\r\nBEGIN:" + comp + "\r\n" +
		strings.Join(lines, "\r\n") + "\r\nEND:" + comp + "\r\nEND:VCALENDAR\r\n"
	obj, err := cal.Parse("/cal/x.ics", `"1"`, strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	return obj
}

// std are the instance's default weights (conf, TestDuplicatesWeights).
var std = Weights{UID: 80, Email: 60, Phone: 60, Name: 20, Birthday: 10, TitleDate: 70}

func rec(account, col, uid string, k view.Kind, obj *model.Object) Record {
	return Record{Ref: sealed.RecordRef{AccountID: account, Col: col, UID: uid}, Kind: k, Keys: KeysOf(k, obj, time.UTC)}
}

func TestFoldIsNotEqualFoldOnTheRawText(t *testing.T) {
	for _, c := range []struct{ a, b string }{
		{"Артём", "АРТЕМ"},
		{"José  Núñez", "jose nunez"},
		{"Straße", "STRASSE"},
	} {
		if Fold(c.a) != Fold(c.b) {
			t.Errorf("Fold(%q) = %q, Fold(%q) = %q", c.a, Fold(c.a), c.b, Fold(c.b))
		}
		if strings.EqualFold(c.a, c.b) {
			t.Errorf("%q and %q are EqualFold: the case does not show why NFD is needed", c.a, c.b)
		}
	}
	if FoldName("Белов Артём") != FoldName("Артем Белов") {
		t.Error("word order must not make two names")
	}
}

func TestNormalisation(t *testing.T) {
	if NormPhone("+7 903 118-22-04") != NormPhone("8 (903) 118 22 04") || NormPhone("+7 903 118-22-04") != "9031182204" {
		t.Errorf("phones: %q %q", NormPhone("+7 903 118-22-04"), NormPhone("8 (903) 118 22 04"))
	}
	if NormPhone("112") != "112" || NormPhone("ext. 204") != "204" || NormPhone("no digits") != "" {
		t.Error("a short number is a key as it is; whether it counts is the weight of the phone")
	}
	if NormEmail(" mailto:Ada@Example.COM ") != "ada@example.com" || NormEmail("not an address") != "" {
		t.Errorf("email: %q", NormEmail(" mailto:Ada@Example.COM "))
	}
	if NormBirthday("1990-01-02") != NormBirthday("19900102") {
		t.Error("birthday forms differ")
	}
}

// The default points (limits.md §4.4): one strong key passes 60 on its own,
// weak signals together never do.
func TestScoresAndThreshold(t *testing.T) {
	ada := vcard(t, "UID:a1", "FN:Ada Lovelace", "EMAIL:ada@example.com", "BDAY:1815-12-10")
	cases := []struct {
		name    string
		other   *model.Object
		uid     string
		score   int
		reasons []Reason
		group   bool
	}{
		{"email and name", vcard(t, "UID:b1", "FN:ADA LOVELACE", "EMAIL:ADA@example.com"), "b1", 80, []Reason{SameEmail, SameName}, true},
		{"email alone", vcard(t, "UID:b2", "FN:Office", "EMAIL:ada@example.com"), "b2", 60, []Reason{SameEmail}, true},
		{"name and birthday", vcard(t, "UID:b3", "FN:Lovelace Ada", "BDAY:18151210"), "b3", 30, []Reason{SameName, SameBirthday}, false},
		{"same UID", vcard(t, "UID:a1", "FN:Someone else"), "a1", 80, []Reason{SameUID}, true},
	}
	for _, c := range cases {
		a := rec("s1", "/book/", "a1", view.KindContact, ada)
		b := rec("s2", "/other/", c.uid, view.KindContact, c.other)
		score, reasons := Score(a, b, std)
		if score != c.score || !cmp.Equal(reasons, c.reasons) {
			t.Errorf("%s: %d %v, want %d %v", c.name, score, reasons, c.score, c.reasons)
		}
		groups := Find([]Record{a, b}, std, 60, nil)
		if (len(groups) == 1) != c.group {
			t.Errorf("%s: groups = %+v", c.name, groups)
		}
	}
}

// The mockup's four cases: a contact in two books, an event copied with a new
// UID, a task due the same day at another hour, a note copied into another
// notebook. Kinds never meet, even with one UID.
func TestFindEveryKind(t *testing.T) {
	recs := []Record{
		rec("s1", "/c1/", "p1", view.KindContact, vcard(t, "UID:p1", "FN:Артём Белов", "TEL:+7 903 118-22-04")),
		rec("s2", "/c2/", "p2", view.KindContact, vcard(t, "UID:p2", "FN:Белов Артем", "TEL:8 (903) 118 22 04")),
		rec("s1", "/work/", "e1", view.KindEvent, ics(t, "VEVENT", "UID:e1", "SUMMARY:Дежурство: первая линия", "DTSTART;VALUE=DATE:20260810")),
		rec("s2", "/duty/", "e2", view.KindEvent, ics(t, "VEVENT", "UID:e2", "SUMMARY:дежурство — первая линия", "DTSTART;VALUE=DATE:20260810")),
		rec("s2", "/duty/", "e3", view.KindEvent, ics(t, "VEVENT", "UID:e3", "SUMMARY:Дежурство: первая линия", "DTSTART;VALUE=DATE:20260817")),
		rec("s1", "/todo/", "t1", view.KindTask, ics(t, "VTODO", "UID:t1", "SUMMARY:Собрать чемодан", "DUE:20260812T180000Z")),
		rec("s2", "/mine/", "t2", view.KindTask, ics(t, "VTODO", "UID:t2", "SUMMARY:Собрать чемодан", "DUE;VALUE=DATE:20260812")),
		rec("s1", "/notes/", "n1", view.KindNote, ics(t, "VJOURNAL", "UID:n1", "SUMMARY:Что почитать про CRDT", "DTSTART;VALUE=DATE:20260808", "DESCRIPTION:one")),
		rec("s2", "/diary/", "n2", view.KindNote, ics(t, "VJOURNAL", "UID:n2", "SUMMARY:Что почитать про CRDT", "DTSTART;VALUE=DATE:20260808", "DESCRIPTION:another body")),
		rec("s2", "/diary/", "p1", view.KindNote, ics(t, "VJOURNAL", "UID:p1", "SUMMARY:x")),
	}
	groups := Find(recs, std, 60, nil)
	var got []string
	for _, g := range groups {
		var uids []string
		for _, m := range g.Members {
			uids = append(uids, m.Ref.UID)
		}
		got = append(got, fmt.Sprintf("%s %d %v %v", g.Kind, g.Score, uids, g.Reasons))
	}
	want := []string{
		"contact 80 [p1 p2] [phone name]",
		"event 70 [e1 e2] [title-date]",
		"task 70 [t1 t2] [title-date]",
		"note 70 [n1 n2] [title-date]",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("groups (-want +got):\n%s", diff)
	}
}

// A group is held by its weakest match, and a record listed twice is one.
func TestGroupsJoinAndCountOnce(t *testing.T) {
	a := rec("s", "/a/", "1", view.KindContact, vcard(t, "UID:1", "FN:Ada", "EMAIL:a@x.org"))
	b := rec("s", "/b/", "2", view.KindContact, vcard(t, "UID:2", "FN:Ada", "EMAIL:a@x.org", "TEL:+44 20 7946 0000"))
	c := rec("s", "/c/", "3", view.KindContact, vcard(t, "UID:3", "FN:Ada", "TEL:020 7946 0000"))
	groups := Find([]Record{a, b, c, b}, std, 60, nil)
	if len(groups) != 1 || len(groups[0].Members) != 3 || groups[0].Score != 80 {
		t.Fatalf("groups = %+v", groups)
	}
	if groups[0].ID != Signature(view.KindContact, []sealed.RecordRef{c.Ref, a.Ref, b.Ref}) {
		t.Error("the ID is not the signature of its members")
	}
}

// Only records sharing a strong key are compared: a large book of strangers
// is linear, not a square.
func TestFindDoesNotCompareEveryPair(t *testing.T) {
	var recs []Record
	for i := 0; i < 20000; i++ {
		recs = append(recs, Record{
			Ref:  sealed.RecordRef{AccountID: "s", Col: "/b/", UID: fmt.Sprint(i)},
			Kind: view.KindContact,
			Keys: Keys{Emails: []string{fmt.Sprintf("p%d@x.org", i)}, Name: "same name"},
		})
	}
	start := time.Now()
	if g := Find(recs, std, 60, nil); len(g) != 0 {
		t.Fatalf("strangers grouped: %d", len(g))
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("20000 records took %v", d)
	}
}

// The weights are the person's: a signal set to 0 is off, and a heavier name
// lets a name with a birthday pass.
func TestWeightsDecide(t *testing.T) {
	a := rec("s", "/a/", "1", view.KindContact, vcard(t, "UID:1", "FN:Ada Lovelace", "EMAIL:office@x.org", "BDAY:1815-12-10"))
	b := rec("s", "/b/", "2", view.KindContact, vcard(t, "UID:2", "FN:Lovelace Ada", "EMAIL:office@x.org", "BDAY:18151210"))
	if g := Find([]Record{a, b}, std, 60, nil); len(g) != 1 || g[0].Score != 90 {
		t.Fatalf("defaults: %+v", g)
	}
	noEmail := std
	SetWeight(&noEmail, SameEmail, 0)
	if g := Find([]Record{a, b}, noEmail, 60, nil); len(g) != 0 {
		t.Errorf("email off, name and birthday 30: %+v", g)
	}
	SetWeight(&noEmail, SameName, 50)
	g := Find([]Record{a, b}, noEmail, 60, nil)
	if len(g) != 1 || !cmp.Equal(g[0].Reasons, []Reason{SameName, SameBirthday}) || Weight(noEmail, SameName) != 50 {
		t.Errorf("heavier name: %+v", g)
	}
}

func TestPruneOnlyOnASuccessfulList(t *testing.T) {
	ref := func(col, uid string) sealed.RecordRef { return sealed.RecordRef{AccountID: "s", Col: col, UID: uid} }
	linked := []sealed.LinkedGroup{
		{ID: "g1", Kind: view.KindContact, Members: []sealed.RecordRef{ref("/a/", "1"), ref("/b/", "2"), ref("/c/", "3")}},
		{ID: "g2", Kind: view.KindContact, Members: []sealed.RecordRef{ref("/a/", "4"), ref("/down/", "5")}},
		{ID: "g3", Kind: view.KindTask, Members: []sealed.RecordRef{ref("/a/", "6"), ref("/b/", "7")}},
	}
	present := map[CollectionRef]map[string]bool{
		{AccountID: "s", Col: "/a/", Kind: view.KindContact}: {"1": true},
		{AccountID: "s", Col: "/b/", Kind: view.KindContact}: {"2": true},
		{AccountID: "s", Col: "/c/", Kind: view.KindContact}: {},
	}
	got, changed := Prune(linked, present)
	if !changed || len(got) != 2 || got[0].ID != "g1" || len(got[0].Members) != 2 || got[1].ID != "g3" {
		t.Fatalf("pruned = %+v", got)
	}
	// g2: "4" is missing from a list read whole, "5" is in a collection that
	// did not answer — one member left, so the group goes. g3's tasks were
	// never read: the contacts list of /a/ says nothing about them.
	if _, changed := Prune(got, present); changed {
		t.Error("pruning twice changes something")
	}
}

func TestLinkAbsorbsOverlappingGroups(t *testing.T) {
	r := func(uid string) sealed.RecordRef { return sealed.RecordRef{AccountID: "s", Col: "/a/", UID: uid} }
	linked := Link(nil, sealed.LinkedGroup{Kind: view.KindContact, Members: []sealed.RecordRef{r("1"), r("2")}})
	linked = Link(linked, sealed.LinkedGroup{Kind: view.KindContact, Members: []sealed.RecordRef{r("5"), r("6")}})
	linked = Link(linked, sealed.LinkedGroup{Kind: view.KindContact, Members: []sealed.RecordRef{r("2"), r("3")}})
	if len(linked) != 2 || len(linked[1].Members) != 3 {
		t.Fatalf("linked = %+v", linked)
	}
	if g, ok := LinkedOf(linked, r("1")); !ok || g.ID != Signature(view.KindContact, []sealed.RecordRef{r("1"), r("2"), r("3")}) {
		t.Errorf("group of 1 = %+v", g)
	}
}

func encodeCard(t *testing.T, obj *model.Object) string {
	t.Helper()
	var buf bytes.Buffer
	if err := card.Encode(&buf, obj); err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(buf.String(), "\r\n ", "")
}

// S-16 merge: the kept body is the base; sets are joined without exact
// repeats; every foreign property of every member stays; the kept record
// wins a collision unless the person chose otherwise; its identity is its
// own; the inputs are not changed.
func TestMergeCards(t *testing.T) {
	kept := vcard(t, "UID:keep", "FN:Артём Белов", "EMAIL;TYPE=home:a.belov@example.org", "TEL:+7 903 118-22-04",
		"X-KEPT:mine", "X-BOTH:kept")
	other := vcard(t, "UID:other", "FN:Белов Артём", "EMAIL;TYPE=home:a.belov@example.org", "EMAIL;TYPE=work:belov@team.example",
		"ORG:Тензор", "X-OTHER:theirs", "X-BOTH:other")
	before := encodeCard(t, other)

	merged, plan, err := Merge(kept, []*model.Object{other}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := encodeCard(t, merged)
	for _, want := range []string{"UID:keep", "FN:Артём Белов", "EMAIL;TYPE=work:belov@team.example", "ORG:Тензор",
		"X-KEPT:mine", "X-OTHER:theirs", "X-BOTH:kept", "X-BOTH:other"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	if strings.Count(got, "a.belov@example.org") != 1 || strings.Contains(got, "UID:other") {
		t.Errorf("repeat or foreign identity in\n%s", got)
	}
	if len(plan.Questions) != 1 || plan.Questions[0].Prop != "FN" || len(plan.Questions[0].Options) != 2 {
		t.Errorf("questions = %+v", plan.Questions)
	}
	if !cmp.Equal(plan.Union, []string{"EMAIL", "X-BOTH"}) {
		t.Errorf("union = %v", plan.Union)
	}
	if encodeCard(t, other) != before || merged.ETag != kept.ETag || merged.Path != kept.Path {
		t.Error("an input changed, or the merged record is not the kept one's version")
	}

	chosen, _, err := Merge(kept, []*model.Object{other}, map[string]int{"FN": 1})
	if err != nil {
		t.Fatal(err)
	}
	if got := encodeCard(t, chosen); !strings.Contains(got, "FN:Белов Артём") {
		t.Errorf("the choice was not taken:\n%s", got)
	}
}

// A calendar merge keeps the kept record's time whole, takes the others'
// reminders it lacks and says which rewritten occurrences go with a deleted
// series.
func TestMergeEvents(t *testing.T) {
	kept := ics(t, "VEVENT", "UID:k", "DTSTAMP:20260801T000000Z", "SUMMARY:Standup", "DTSTART;TZID=Europe/Moscow:20260810T100000",
		"BEGIN:VALARM", "ACTION:DISPLAY", "TRIGGER:-PT15M", "DESCRIPTION:r", "END:VALARM")
	doc := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//t//EN\r\n" +
		"BEGIN:VEVENT\r\nUID:o\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Standup\r\nDTSTART:20260810T070000Z\r\nRRULE:FREQ=DAILY\r\nLOCATION:Room 4\r\nX-OTHER:x\r\n" +
		"BEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT15M\r\nDESCRIPTION:r\r\nEND:VALARM\r\n" +
		"BEGIN:VALARM\r\nACTION:AUDIO\r\nTRIGGER:-PT1H\r\nEND:VALARM\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nUID:o\r\nDTSTAMP:20260801T000000Z\r\nRECURRENCE-ID:20260811T070000Z\r\nDTSTART:20260811T080000Z\r\nSUMMARY:Later\r\nEND:VEVENT\r\n" +
		"END:VCALENDAR\r\n"
	other, err := cal.Parse("/c/o.ics", `"2"`, strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	merged, plan, err := Merge(kept, []*model.Object{other}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Alarms != 1 || plan.Instances[1] != 1 {
		t.Errorf("plan = %+v", plan)
	}
	if got := merged.PropText("DTSTART"); len(got) != 1 || got[0] != "20260810T100000" || len(merged.PropText("RRULE")) != 0 {
		t.Errorf("time is not the kept record's: DTSTART %v RRULE %v", got, merged.PropText("RRULE"))
	}
	if merged.PropText("LOCATION")[0] != "Room 4" || merged.PropText("X-OTHER")[0] != "x" || cal.UID(merged) != "k" {
		t.Errorf("merged props = %v", merged.Props())
	}
	alarms, err := cal.Alarms(merged)
	if err != nil || len(alarms) != 2 {
		t.Errorf("alarms = %+v %v", alarms, err)
	}
}

// Дб7: a single value answered Leave is not in the merged record, and the
// plan says which value of the kept record went.
func TestMergeLeavesOutWhatWasChosenEmpty(t *testing.T) {
	kept := vcard(t, "UID:keep", "FN:Вера Лебедева", "ORG:Библиотека №4")
	other := vcard(t, "UID:other", "FN:Вера Лебедева")
	merged, plan, err := Merge(kept, []*model.Object{other}, map[string]int{"ORG": Leave})
	if err != nil {
		t.Fatal(err)
	}
	if got := encodeCard(t, merged); strings.Contains(got, "ORG") {
		t.Errorf("ORG left out, and still there:\n%s", got)
	}
	if len(plan.Left) != 1 || plan.Left[0].Prop != "ORG" || plan.Left[0].Value.Text != "Библиотека №4" {
		t.Errorf("left = %+v", plan.Left)
	}
	if kept.PropText("ORG")[0] != "Библиотека №4" {
		t.Error("the kept input changed")
	}
}

// A pair marked «not duplicates» does not join two matters (s-dup-1): Анна
// in three books stays her group, and Алексей Смирнов, who shares her work
// phone, is not drawn into it; Assess still scores the pair for its card.
func TestFindKeepsAnIgnoredPairApart(t *testing.T) {
	ref := func(col, uid string) sealed.RecordRef { return sealed.RecordRef{AccountID: "s", Col: col, UID: uid} }
	anna := func(col, uid string, phones ...string) Record {
		return Record{Ref: ref(col, uid), Kind: view.KindContact,
			Keys: Keys{Emails: []string{"a@x.org"}, Phones: phones, Name: "анна ковалёва"}}
	}
	recs := []Record{
		anna("/home/", "a1", "79124451088", "74951204500"),
		anna("/team/", "a2", "79124451088"),
		anna("/old/", "a3", "79124451088"),
		{Ref: ref("/home/", "s1"), Kind: view.KindContact, Keys: Keys{Phones: []string{"74951204500"}, Name: "алексей смирнов"}},
	}
	pair := []sealed.RecordRef{ref("/home/", "a1"), ref("/home/", "s1")}
	if g := Find(recs, std, 60, nil); len(g) != 1 || len(g[0].Members) != 4 {
		t.Fatalf("without the decision = %+v, want one group of four", g)
	}
	g := Find(recs, std, 60, [][]sealed.RecordRef{pair})
	if len(g) != 1 || len(g[0].Members) != 3 {
		t.Fatalf("with the pair apart = %+v, want Анна's three alone", g)
	}
	score, why := Assess([]Record{recs[0], recs[3]}, std)
	if score != std.Phone || !slices.Equal(why, []Reason{SamePhone}) {
		t.Errorf("the pair = %d %v, want the phone's points", score, why)
	}
}

// From-typecase note 117 p. 1: a value unticked on the group's card is not
// in the merged record — neither the kept one's nor another's —, a tag is
// taken out of its comma-separated value, and the plan names what was left.
func TestMergeKeepingLeavesTheUnticked(t *testing.T) {
	kept := vcard(t, "UID:a", "FN:Анна Ковалёва", "TEL;TYPE=WORK:+7 495 120-45-00", "TEL;TYPE=CELL:+7 912 445-10-88",
		"CATEGORIES:работа,издательство")
	other := vcard(t, "UID:a", "FN:Анна Ковалёва", "TEL;TYPE=WORK:+7 495 120-45-77", "EMAIL:old@example.org", "CATEGORIES:работа")
	merged, plan, err := MergeKeeping(kept, []*model.Object{other}, nil, map[string][]string{
		"TEL": {ValueKey("TEL", "+7 495 120-45-00")}, "EMAIL": {"old@example.org"}, "CATEGORIES": {"издательство"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := encodeCard(t, merged)
	for _, want := range []string{"TEL;TYPE=WORK:+7 495 120-45-77", "TEL;TYPE=CELL:+7 912 445-10-88", "CATEGORIES:работа"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	for _, gone := range []string{"120-45-00", "old@example.org", "издательство"} {
		if strings.Contains(got, gone) {
			t.Errorf("%q unticked, still in\n%s", gone, got)
		}
	}
	var left []string
	for _, l := range plan.Left {
		left = append(left, l.Prop+"="+l.Value.Text)
	}
	slices.Sort(left)
	if want := []string{"CATEGORIES=издательство", "EMAIL=old@example.org", "TEL=+7 495 120-45-00"}; !slices.Equal(left, want) {
		t.Errorf("left = %v, want %v", left, want)
	}
}
