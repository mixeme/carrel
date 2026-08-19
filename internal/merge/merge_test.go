// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package merge

import (
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/model"
)

func TestNormalizePhoneAndKey(t *testing.T) {
	cases := []struct {
		in         string
		normalized string
		key        string
	}{
		{"+7 495 123-45-67", "+74951234567", "951234567"},
		{"8 (495) 123 45 67", "84951234567", "951234567"},
		{"495 123-45-67", "4951234567", "951234567"},
		{"00 44 20 7123 4567", "+442071234567", "071234567"},
		{"tel:+44-20-7123-4567;ext=99", "+442071234567", "071234567"},
		{"112", "112", ""},
		{"", "", ""},
	}
	for _, c := range cases {
		if got := NormalizePhone(c.in); got != c.normalized {
			t.Errorf("NormalizePhone(%q) = %q, want %q", c.in, got, c.normalized)
		}
		if got := PhoneKey(c.in); got != c.key {
			t.Errorf("PhoneKey(%q) = %q, want %q", c.in, got, c.key)
		}
	}
}

func TestNormalizeNameSortsWords(t *testing.T) {
	if a, b := NormalizeName("Ivan Petrov"), NormalizeName("Petrov, Ivan"); a != b {
		t.Fatalf("name order matters: %q != %q", a, b)
	}
	if got := NormalizeName("  Dr. Ada  LOVELACE "); got != "ada dr lovelace" {
		t.Fatalf("NormalizeName = %q", got)
	}
	if got := NormalizeName("!!!"); got != "" {
		t.Fatalf("punctuation only should fold to nothing, got %q", got)
	}
}

func TestBirthdayMatchAcceptsPartialDates(t *testing.T) {
	full := BirthdayKey("1985-05-10")
	partial := BirthdayKey("--0510")
	if !birthdayMatch(full, partial) {
		t.Fatalf("%q and %q should be the same birthday", full, partial)
	}
	if birthdayMatch(full, BirthdayKey("1985-05-11")) {
		t.Fatal("different days matched")
	}
}

// TestScoreContacts is the table of §15: one strong signal reaches the default
// threshold on its own, and the weak ones together do not.
func TestScoreContacts(t *testing.T) {
	cases := []struct {
		name    string
		a, b    Fingerprint
		want    int
		signals []string
	}{
		{
			name:    "shared address",
			a:       Fingerprint{Kind: KindContact, Emails: []string{"ada@example.org"}},
			b:       Fingerprint{Kind: KindContact, Emails: []string{"ada@example.org"}},
			want:    WeightEmail,
			signals: []string{SignalEmail},
		},
		{
			name:    "shared number written differently",
			a:       Fingerprint{Kind: KindContact, Phones: []string{PhoneKey("+7 495 123-45-67")}},
			b:       Fingerprint{Kind: KindContact, Phones: []string{PhoneKey("8 495 1234567")}},
			want:    WeightPhone,
			signals: []string{SignalPhone},
		},
		{
			name:    "same name only is not enough",
			a:       Fingerprint{Kind: KindContact, Name: NormalizeName("Ada Lovelace")},
			b:       Fingerprint{Kind: KindContact, Name: NormalizeName("Lovelace, Ada")},
			want:    WeightName,
			signals: []string{SignalName},
		},
		{
			name: "name and birthday is still a near miss",
			a: Fingerprint{
				Kind: KindContact, Name: NormalizeName("Ada Lovelace"),
				Birthday: BirthdayKey("1815-12-10"),
			},
			b: Fingerprint{
				Kind: KindContact, Name: NormalizeName("Ada Lovelace"),
				Birthday: BirthdayKey("--1210"),
			},
			want:    WeightName + WeightBirthday,
			signals: []string{SignalName, SignalBirthday},
		},
		{
			name: "birthday alone makes nothing",
			a:    Fingerprint{Kind: KindContact, Birthday: BirthdayKey("1815-12-10"), Name: "ada"},
			b:    Fingerprint{Kind: KindContact, Birthday: BirthdayKey("1815-12-10"), Name: "grace"},
			want: 0,
		},
		{
			name:    "shared uid",
			a:       Fingerprint{Kind: KindContact, UID: "card-1"},
			b:       Fingerprint{Kind: KindContact, UID: "card-1"},
			want:    WeightUID,
			signals: []string{SignalUID},
		},
		{
			name: "different kinds never score",
			a:    Fingerprint{Kind: KindContact, UID: "same"},
			b:    Fingerprint{Kind: KindEvent, UID: "same"},
			want: 0,
		},
		{
			name: "empty fingerprints never score",
			a:    Fingerprint{Kind: KindContact},
			b:    Fingerprint{Kind: KindContact},
			want: 0,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			score, signals := Score(c.a, c.b)
			if score != c.want {
				t.Fatalf("score = %d, want %d (signals %v)", score, c.want, signals)
			}
			for _, want := range c.signals {
				if !contains(signals, want) {
					t.Fatalf("signals %v missing %q", signals, want)
				}
			}
			if c.want >= DefaultThreshold && len(c.signals) == 0 {
				t.Fatal("a match at the threshold must give a reason")
			}
		})
	}
}

func TestScoreEvents(t *testing.T) {
	start := time.Date(2026, 3, 4, 9, 0, 0, 0, time.UTC)
	same := Fingerprint{Kind: KindEvent, Name: NormalizeName("Planning"), Start: start}
	other := Fingerprint{Kind: KindEvent, Name: NormalizeName("Planning"), Start: start}
	if score, signals := Score(same, other); score != WeightStartSummary || !contains(signals, SignalStart) {
		t.Fatalf("start and title = %d %v", score, signals)
	}
	moved := Fingerprint{Kind: KindEvent, Name: NormalizeName("Planning"), Start: start.Add(time.Hour)}
	if score, _ := Score(same, moved); score != 0 {
		t.Fatalf("a different start should not match, got %d", score)
	}
	byUID := Fingerprint{Kind: KindEvent, UID: "evt-1", Start: start}
	if score, signals := Score(byUID, Fingerprint{Kind: KindEvent, UID: "evt-1"}); score != WeightUID || !contains(signals, SignalUID) {
		t.Fatalf("uid = %d %v", score, signals)
	}
}

func TestDetectContactsGroupsAndRespectsSkip(t *testing.T) {
	records := []Record{
		contactRecord(t, "a", "/books/one/", "one", "Ada Lovelace", "ada@example.org", "+7 495 123-45-67"),
		contactRecord(t, "b", "/books/two/", "two", "Lovelace Ada", "ada@example.org", ""),
		contactRecord(t, "c", "/books/two/", "three", "Grace Hopper", "grace@example.org", ""),
	}
	groups := DetectContacts(records, Options{})
	if len(groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(groups))
	}
	if len(groups[0].Members) != 2 {
		t.Fatalf("members = %d, want 2", len(groups[0].Members))
	}
	if groups[0].Kind != KindContact {
		t.Fatalf("kind = %q", groups[0].Kind)
	}
	if !contains(groups[0].Signals, SignalEmail) {
		t.Fatalf("signals = %v", groups[0].Signals)
	}

	// The "not duplicates" verdict of §15 arrives as Skip, and a rejected pair
	// is never offered again however well it scores.
	skipped := DetectContacts(records, Options{Skip: func(a, b int) bool { return true }})
	if len(skipped) != 0 {
		t.Fatalf("skipped pairs still grouped: %d", len(skipped))
	}
}

func TestDetectContactsChainsThroughSharedSignals(t *testing.T) {
	// One card shares an address with the second and a number with the third:
	// all three are one person, which is what the union of pairs is for.
	records := []Record{
		contactRecord(t, "a", "/books/one/", "one", "Ada Lovelace", "ada@example.org", "+7 495 123-45-67"),
		contactRecord(t, "b", "/books/two/", "two", "A. Lovelace", "ada@example.org", ""),
		contactRecord(t, "c", "/books/three/", "three", "Countess Lovelace", "", "8 495 123 45 67"),
	}
	groups := DetectContacts(records, Options{})
	if len(groups) != 1 || len(groups[0].Members) != 3 {
		t.Fatalf("groups = %+v", groups)
	}
}

func TestDetectContactsHonoursThreshold(t *testing.T) {
	records := []Record{
		contactRecord(t, "a", "/books/one/", "one", "Ada Lovelace", "", ""),
		contactRecord(t, "b", "/books/two/", "two", "Ada Lovelace", "", ""),
	}
	if groups := DetectContacts(records, Options{}); len(groups) != 0 {
		t.Fatalf("a shared name alone should stay below the default threshold, got %d", len(groups))
	}
	if groups := DetectContacts(records, Options{Threshold: WeightName}); len(groups) != 1 {
		t.Fatalf("lowering the threshold should offer the pair, got %d", len(groups))
	}
}

func TestDetectEvents(t *testing.T) {
	records := []Record{
		eventRecord(t, "a", "/cals/one/", "one", "Planning", "20260304T090000Z"),
		eventRecord(t, "b", "/cals/two/", "two", "Planning", "20260304T090000Z"),
		eventRecord(t, "c", "/cals/two/", "three", "Planning", "20260305T090000Z"),
	}
	groups := DetectEvents(records, time.UTC, Options{})
	if len(groups) != 1 || len(groups[0].Members) != 2 {
		t.Fatalf("groups = %+v", groups)
	}
	if groups[0].Kind != KindEvent {
		t.Fatalf("kind = %q", groups[0].Kind)
	}
}

func TestSetsJoinsPairs(t *testing.T) {
	sets := Sets(5, [][2]int{{0, 1}, {1, 4}})
	if len(sets) != 3 {
		t.Fatalf("sets = %v", sets)
	}
	if len(sets[0]) != 3 || sets[0][0] != 0 || sets[0][2] != 4 {
		t.Fatalf("first set = %v", sets[0])
	}
}

func TestMergeContactsUnionsAndAsksAboutConflicts(t *testing.T) {
	records := []Record{
		contactRecord(t, "a", "/books/one/", "one", "Ada Lovelace", "ada@example.org", "+7 495 123-45-67"),
		contactRecord(t, "b", "/books/two/", "two", "Ada King", "ada.king@example.org", "8 (495) 123 45 67"),
	}
	merged := MergeContacts(records, nil)

	name, ok := merged.Field("FN")
	if !ok || !name.Conflict {
		t.Fatalf("the two names should be a conflict: %+v", name)
	}
	if name.Chosen != "Ada Lovelace" || len(name.Options) != 2 {
		t.Fatalf("conflict = %+v", name)
	}
	if merged.Title != "Ada Lovelace" {
		t.Fatalf("title = %q", merged.Title)
	}
	if emails := merged.Values("EMAIL"); len(emails) != 2 {
		t.Fatalf("addresses should be a union: %v", emails)
	}
	// The same number written two ways is one number.
	if phones := merged.Values("TEL"); len(phones) != 1 {
		t.Fatalf("phones = %v", phones)
	}
	if len(merged.Conflicts()) != 1 {
		t.Fatalf("conflicts = %+v", merged.Conflicts())
	}

	// A remembered choice wins over the order the records happen to be in.
	remembered := MergeContacts(records, map[string]string{"FN": "Ada King"})
	chosen, _ := remembered.Field("FN")
	if chosen.Chosen != "Ada King" || !chosen.Remembered {
		t.Fatalf("preference ignored: %+v", chosen)
	}
	// A preference nothing offers any more falls back rather than inventing it.
	stale := MergeContacts(records, map[string]string{"FN": "Someone Else"})
	if field, _ := stale.Field("FN"); field.Chosen != "Ada Lovelace" || field.Remembered {
		t.Fatalf("stale preference used: %+v", field)
	}
}

func TestMergedPatchAddsWithoutOverwriting(t *testing.T) {
	target := vcard(t, "/books/one/one.vcf", strings.Join([]string{
		"BEGIN:VCARD", "VERSION:3.0", "UID:one", "FN:Ada Lovelace",
		"EMAIL:ada@example.org", "TEL:+7 495 123-45-67",
		"END:VCARD",
	}, "\r\n"))
	other := vcard(t, "/books/two/two.vcf", strings.Join([]string{
		"BEGIN:VCARD", "VERSION:3.0", "UID:two", "FN:Ada King",
		"EMAIL:ada.king@example.org", "TEL:8 (495) 123 45 67",
		"NOTE:Analytical engine", "X-CLIENT-FLAG:keep-me",
		"END:VCARD",
	}, "\r\n"))

	patch, added, err := MergedPatch(target, []*model.Object{other})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(added, "NOTE") || !contains(added, "EMAIL") || !contains(added, "X-CLIENT-FLAG") {
		t.Fatalf("added = %v", added)
	}
	// FN is a conflict, and §15 leaves the target's value alone.
	if contains(added, "FN") {
		t.Fatalf("the target's own name was replaced: %v", added)
	}
	// The same number in both cards is not added twice.
	if contains(added, "TEL") {
		t.Fatalf("a number already on the target was added again: %v", added)
	}

	merged := target.Clone()
	if err := merged.Apply(patch); err != nil {
		t.Fatal(err)
	}
	if got := len(merged.Property("EMAIL")); got != 2 {
		t.Fatalf("merged addresses = %d, want 2", got)
	}
	if got := len(merged.Property("TEL")); got != 1 {
		t.Fatalf("merged numbers = %d, want 1", got)
	}
	if got := merged.UID(); got != "one" {
		t.Fatalf("the target's UID changed to %q", got)
	}
	contact, err := merged.Contact()
	if err != nil {
		t.Fatal(err)
	}
	if contact.DisplayName() != "Ada Lovelace" {
		t.Fatalf("name = %q", contact.DisplayName())
	}
	if contact.Note != "Analytical engine" {
		t.Fatalf("note = %q", contact.Note)
	}
}

func TestMergedPatchRefusesOtherKinds(t *testing.T) {
	event := ical(t, "/cals/one/one.ics", strings.Join([]string{
		"BEGIN:VCALENDAR", "VERSION:2.0", "BEGIN:VEVENT", "UID:evt",
		"DTSTART:20260304T090000Z", "SUMMARY:Planning", "END:VEVENT", "END:VCALENDAR",
	}, "\r\n"))
	if _, _, err := MergedPatch(event, nil); err == nil {
		t.Fatal("an event should not be the target of a card merge")
	}
	card := vcard(t, "/books/one/one.vcf", "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:one\r\nFN:Ada\r\nEND:VCARD")
	if _, _, err := MergedPatch(card, []*model.Object{event}); err == nil {
		t.Fatal("an event should not be merged into a card")
	}
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func contactRecord(t *testing.T, accountID, collection, uid, name, email, phone string) Record {
	t.Helper()
	lines := []string{"BEGIN:VCARD", "VERSION:3.0", "UID:" + uid, "FN:" + name}
	if email != "" {
		lines = append(lines, "EMAIL:"+email)
	}
	if phone != "" {
		lines = append(lines, "TEL:"+phone)
	}
	lines = append(lines, "END:VCARD")
	return Record{
		AccountID: accountID, Collection: collection,
		Object: vcard(t, collection+uid+".vcf", strings.Join(lines, "\r\n")),
	}
}

func eventRecord(t *testing.T, accountID, collection, uid, summary, start string) Record {
	t.Helper()
	body := strings.Join([]string{
		"BEGIN:VCALENDAR", "VERSION:2.0", "BEGIN:VEVENT", "UID:" + uid,
		"DTSTART:" + start, "SUMMARY:" + summary, "END:VEVENT", "END:VCALENDAR",
	}, "\r\n")
	return Record{
		AccountID: accountID, Collection: collection,
		Object: ical(t, collection+uid+".ics", body),
	}
}

func TestDetectTodosGroupsOnUID(t *testing.T) {
	// The same task copied into a second list keeps its UID, which is the one
	// signal worth 60 on its own. A shared summary is a hint and stays below
	// the default threshold — two people can both have "Call the bank".
	records := []Record{
		todoRecord(t, "a", "/cals/one/", "task-1", "Renew the domain"),
		todoRecord(t, "b", "/cals/two/", "task-1", "Renew the domain"),
		todoRecord(t, "b", "/cals/two/", "task-2", "Renew the domain"),
		eventRecord(t, "b", "/cals/two/", "evt-1", "Renew the domain", "20260304T090000Z"),
	}
	groups := DetectTodos(records, time.UTC, Options{})
	if len(groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(groups))
	}
	if len(groups[0].Members) != 2 {
		t.Fatalf("members = %d, want 2 — the event and the third task are not tasks of this pair", len(groups[0].Members))
	}
	if groups[0].Kind != KindTodo {
		t.Fatalf("kind = %q", groups[0].Kind)
	}
	if !contains(groups[0].Signals, SignalUID) {
		t.Fatalf("signals = %v", groups[0].Signals)
	}
}

func TestDetectTodosNeedsMoreThanASummary(t *testing.T) {
	records := []Record{
		todoRecord(t, "a", "/cals/one/", "task-1", "Call the bank"),
		todoRecord(t, "b", "/cals/two/", "task-2", "Call the bank"),
	}
	if groups := DetectTodos(records, time.UTC, Options{}); len(groups) != 0 {
		t.Fatalf("a shared summary alone should stay below the threshold, got %d", len(groups))
	}
	if groups := DetectTodos(records, time.UTC, Options{Threshold: WeightName}); len(groups) != 1 {
		t.Fatalf("lowering the threshold should offer the pair, got %d", len(groups))
	}
}

func TestDetectNotesGroupsOnUIDAndRespectsSkip(t *testing.T) {
	records := []Record{
		noteRecord(t, "a", "/cals/one/", "note-1", "Minutes", "20260304"),
		noteRecord(t, "b", "/cals/two/", "note-1", "Minutes", "20260304"),
		noteRecord(t, "b", "/cals/two/", "note-2", "Shopping", ""),
	}
	groups := DetectNotes(records, time.UTC, Options{})
	if len(groups) != 1 || len(groups[0].Members) != 2 {
		t.Fatalf("groups = %+v", groups)
	}
	if groups[0].Kind != KindNote {
		t.Fatalf("kind = %q", groups[0].Kind)
	}
	skipped := DetectNotes(records, time.UTC, Options{Skip: func(a, b int) bool { return true }})
	if len(skipped) != 0 {
		t.Fatalf("skipped pairs still grouped: %d", len(skipped))
	}
}

func TestDetectNotesIgnoresOtherComponents(t *testing.T) {
	// The duplicates poll loads every component from a calendar (§1.15), so the
	// kind filter is what keeps a task out of the notes section.
	records := []Record{
		todoRecord(t, "a", "/cals/one/", "same-uid", "Minutes"),
		todoRecord(t, "b", "/cals/two/", "same-uid", "Minutes"),
	}
	if groups := DetectNotes(records, time.UTC, Options{}); len(groups) != 0 {
		t.Fatalf("tasks grouped as notes: %+v", groups)
	}
	if groups := DetectTodos(records, time.UTC, Options{}); len(groups) != 1 {
		t.Fatalf("the same pair should group as tasks, got %d", len(groups))
	}
}

func vcard(t *testing.T, path, body string) *model.Object {
	t.Helper()
	obj, err := model.ParseVCard(path, `"etag"`, []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	return obj
}

func ical(t *testing.T, path, body string) *model.Object {
	t.Helper()
	obj, err := model.ParseICal(path, `"etag"`, []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	return obj
}

func todoRecord(t *testing.T, accountID, collection, uid, summary string) Record {
	t.Helper()
	body := strings.Join([]string{
		"BEGIN:VCALENDAR", "VERSION:2.0", "BEGIN:VTODO", "UID:" + uid,
		"SUMMARY:" + summary, "END:VTODO", "END:VCALENDAR",
	}, "\r\n")
	return Record{
		AccountID: accountID, Collection: collection,
		Object: ical(t, collection+uid+".ics", body),
	}
}

func noteRecord(t *testing.T, accountID, collection, uid, summary, date string) Record {
	t.Helper()
	lines := []string{"BEGIN:VCALENDAR", "VERSION:2.0", "BEGIN:VJOURNAL", "UID:" + uid, "SUMMARY:" + summary}
	if date != "" {
		lines = append(lines, "DTSTART;VALUE=DATE:"+date)
	}
	lines = append(lines, "END:VJOURNAL", "END:VCALENDAR")
	return Record{
		AccountID: accountID, Collection: collection,
		Object: ical(t, collection+uid+".ics", strings.Join(lines, "\r\n")),
	}
}
