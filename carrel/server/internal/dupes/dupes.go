// Package dupes finds duplicate records among what a session has already
// loaded, and builds the merged record for a merge on the server (S-16).
//
// Nothing here asks a server. A record is folded once into the values it is
// compared by (Keys), and only records that share a strong key — a
// normalised email, the last ten digits of a phone, a UID, or for calendar
// kinds a title together with its date — are compared at all: a book of
// thousands of cards is never compared with itself pair by pair.
//
// The package does not import internal/web and fills no view page: it gives
// groups, reasons and a merge plan, and web turns them into screens.
package dupes

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// Points of each signal (docs/spec/limits.md §4.4, architecture.md). The
// threshold is points, not a probability: a number the person can change and
// check. One strong key with the name passes the default of 60; weak signals
// together never do.
const (
	ScoreUID      = 80
	ScoreEmail    = 50
	ScorePhone    = 50
	ScoreName     = 20
	ScoreBirthday = 10
	// ScoreTitleWhen is an event's title with its start, a task's with its
	// due day, a note's with its day: the date plays the part of a strong
	// key and the title the part of the name, so it weighs what those two
	// weigh together. Neither alone is a signal — a title alone is noise.
	ScoreTitleWhen = ScoreEmail + ScoreName
)

// Reason is why two records were matched; the screen puts it beside the
// score, because a reason can be checked and a number cannot.
type Reason string

const (
	SameUID       Reason = "uid"
	SameEmail     Reason = "email"
	SamePhone     Reason = "phone"
	SameName      Reason = "name"
	SameBirthday  Reason = "birthday"
	SameTitleWhen Reason = "title-when"
)

var reasonOrder = []Reason{SameUID, SameEmail, SamePhone, SameName, SameBirthday, SameTitleWhen}

// Keys are the values a record is compared by, already normalised. They are
// kept with the loaded list, so finding duplicates reads no record again.
type Keys struct {
	Emails   []string `json:"e,omitempty"`
	Phones   []string `json:"p,omitempty"`
	Name     string   `json:"n,omitempty"` // folded words, sorted: word order is not identity
	Birthday string   `json:"b,omitempty"` // digits of BDAY
	Title    string   `json:"t,omitempty"` // folded title of a calendar kind
	When     string   `json:"w,omitempty"` // start, due day or day of a calendar kind
}

// Record is one loaded record: where it is and what it is compared by.
type Record struct {
	Ref  sealed.RecordRef
	Kind view.Kind
	Keys Keys
}

// Group is a set of records found to be one entity. Score is the weakest
// match that holds the group together; Reasons are every reason among its
// matches.
type Group struct {
	ID      string
	Kind    view.Kind
	Members []Record
	Score   int
	Reasons []Reason
}

// Refs are the addresses of the group's members.
func (g Group) Refs() []sealed.RecordRef {
	out := make([]sealed.RecordRef, len(g.Members))
	for i, m := range g.Members {
		out[i] = m.Ref
	}
	return out
}

// Signature names a set of records whatever their order: the ID of a group
// and what "not duplicates" remembers. A group that gains a member has
// another signature and is offered again — that is new information.
func Signature(kind view.Kind, refs []sealed.RecordRef) string {
	lines := make([]string, len(refs))
	for i, r := range refs {
		lines[i] = r.AccountID + "\x00" + r.Col + "\x00" + r.UID
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(string(kind) + "\n" + strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:16])
}

// Find groups the records that match at threshold points or more. Records of
// different kinds are never compared; a record without a UID cannot be
// addressed and is left out, and one listed twice counts once.
func Find(records []Record, threshold int) []Group {
	recs := make([]Record, 0, len(records))
	seen := map[sealed.RecordRef]bool{}
	for _, r := range records {
		if r.Ref.UID == "" || seen[r.Ref] {
			continue
		}
		seen[r.Ref] = true
		recs = append(recs, r)
	}

	buckets := map[string][]int{}
	for i, r := range recs {
		for _, k := range bucketKeys(r) {
			buckets[k] = append(buckets[k], i)
		}
	}
	type pair struct{ a, b int }
	pairs := map[pair]bool{}
	for _, idx := range buckets {
		for x := 0; x < len(idx); x++ {
			for y := x + 1; y < len(idx); y++ {
				a, b := idx[x], idx[y]
				if a > b {
					a, b = b, a
				}
				pairs[pair{a, b}] = true
			}
		}
	}

	parent := make([]int, len(recs))
	for i := range parent {
		parent[i] = i
	}
	var root func(int) int
	root = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	type edge struct {
		a, b    int
		score   int
		reasons []Reason
	}
	var edges []edge
	for p := range pairs {
		score, reasons := Score(recs[p.a], recs[p.b])
		if score < threshold || score <= 0 {
			continue
		}
		edges = append(edges, edge{p.a, p.b, score, reasons})
		parent[root(p.a)] = root(p.b)
	}

	byRoot := map[int]*Group{}
	var order []int
	for _, e := range edges {
		r := root(e.a)
		g := byRoot[r]
		if g == nil {
			g = &Group{Kind: recs[e.a].Kind, Score: e.score}
			byRoot[r] = g
			order = append(order, r)
		}
		if e.score < g.Score {
			g.Score = e.score
		}
		g.Reasons = mergeReasons(g.Reasons, e.reasons)
	}
	for i := range recs {
		if g := byRoot[root(i)]; g != nil {
			g.Members = append(g.Members, recs[i])
		}
	}
	out := make([]Group, 0, len(order))
	for _, r := range order {
		g := byRoot[r]
		sort.SliceStable(g.Members, func(i, j int) bool { return refLess(g.Members[i].Ref, g.Members[j].Ref) })
		g.ID = Signature(g.Kind, g.Refs())
		out = append(out, *g)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if ki, kj := kindRank(out[i].Kind), kindRank(out[j].Kind); ki != kj {
			return ki < kj
		}
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Score is the points of one pair and why. Records of different kinds score
// nothing.
func Score(a, b Record) (int, []Reason) {
	if a.Kind != b.Kind {
		return 0, nil
	}
	var score int
	var reasons []Reason
	add := func(ok bool, n int, r Reason) {
		if ok {
			score += n
			reasons = append(reasons, r)
		}
	}
	add(a.Ref.UID != "" && a.Ref.UID == b.Ref.UID, ScoreUID, SameUID)
	if a.Kind == view.KindContact {
		add(shares(a.Keys.Emails, b.Keys.Emails), ScoreEmail, SameEmail)
		add(shares(a.Keys.Phones, b.Keys.Phones), ScorePhone, SamePhone)
		add(a.Keys.Name != "" && a.Keys.Name == b.Keys.Name, ScoreName, SameName)
		add(a.Keys.Birthday != "" && a.Keys.Birthday == b.Keys.Birthday, ScoreBirthday, SameBirthday)
		return score, reasons
	}
	add(a.Keys.Title != "" && a.Keys.When != "" && a.Keys.Title == b.Keys.Title && a.Keys.When == b.Keys.When,
		ScoreTitleWhen, SameTitleWhen)
	return score, reasons
}

// bucketKeys are the strong keys of a record, the only ones that put two
// records in front of each other. A name is weak and never a bucket.
func bucketKeys(r Record) []string {
	pre := string(r.Kind) + "\x00"
	out := []string{pre + "u\x00" + r.Ref.UID}
	if r.Kind == view.KindContact {
		for _, e := range r.Keys.Emails {
			out = append(out, pre+"e\x00"+e)
		}
		for _, p := range r.Keys.Phones {
			out = append(out, pre+"p\x00"+p)
		}
		return out
	}
	if r.Keys.Title != "" && r.Keys.When != "" {
		out = append(out, pre+"w\x00"+r.Keys.When+"\x00"+r.Keys.Title)
	}
	return out
}

func shares(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return true
			}
		}
	}
	return false
}

func mergeReasons(have, more []Reason) []Reason {
	set := map[Reason]bool{}
	for _, r := range have {
		set[r] = true
	}
	for _, r := range more {
		set[r] = true
	}
	var out []Reason
	for _, r := range reasonOrder {
		if set[r] {
			out = append(out, r)
		}
	}
	return out
}

func refLess(a, b sealed.RecordRef) bool {
	if a.AccountID != b.AccountID {
		return a.AccountID < b.AccountID
	}
	if a.Col != b.Col {
		return a.Col < b.Col
	}
	return a.UID < b.UID
}

func kindRank(k view.Kind) int {
	switch k {
	case view.KindContact:
		return 0
	case view.KindEvent:
		return 1
	case view.KindTask:
		return 2
	default:
		return 3
	}
}

var folder = cases.Fold()

// Fold is text as it is compared: NFD, combining marks dropped, case folded,
// anything that is not a letter or a digit a space, spaces collapsed. "Артём"
// and "АРТЕМ" are the same word; strings.EqualFold on the raw text would not
// say so.
func Fold(s string) string {
	d := norm.NFD.String(s)
	var b strings.Builder
	for _, r := range d {
		switch {
		case unicode.Is(unicode.Mn, r):
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(folder.String(b.String())), " ")
}

// FoldName is a folded name with its words sorted: "Белов Артём" and "Артём
// Белов" are one name.
func FoldName(s string) string {
	words := strings.Fields(Fold(s))
	sort.Strings(words)
	return strings.Join(words, " ")
}

// NormEmail is an address as it is compared: no mailto:, no spaces, lower
// case.
func NormEmail(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 7 && strings.EqualFold(s[:7], "mailto:") {
		s = s[7:]
	}
	s = strings.ToLower(strings.TrimSpace(s))
	if !strings.Contains(s, "@") {
		return ""
	}
	return s
}

// minPhoneDigits keeps short numbers out: a short code or an extension is
// shared by strangers, and a false match is worse than a missed one (S-16).
const minPhoneDigits = 7

// NormPhone is the last ten digits of a number, so "+7 903 118-22-04" and
// "8 (903) 118 22 04" meet; fewer than seven digits is not a key.
func NormPhone(s string) string {
	var d []byte
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			d = append(d, s[i])
		}
	}
	if len(d) < minPhoneDigits {
		return ""
	}
	if len(d) > 10 {
		d = d[len(d)-10:]
	}
	return string(d)
}

// NormBirthday is the digits of a BDAY: "1990-01-02" and "19900102" meet.
func NormBirthday(s string) string {
	var d []byte
	for i := 0; i < len(s) && s[i] != 'T'; i++ {
		if s[i] >= '0' && s[i] <= '9' {
			d = append(d, s[i])
		}
	}
	return string(d)
}
