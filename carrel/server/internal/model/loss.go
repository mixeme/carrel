package model

import (
	"maps"
	"sort"
	"strings"
	"time"
)

// LossAggregator records properties a server has already been seen dropping (S-7.1).
type LossAggregator interface {
	Has(serverID, prop string) bool
	Note(serverID, prop string)
}

// Loss is the outcome of comparing a sent object with a re-read copy.
type Loss struct {
	Properties []string
	Aggregated bool // true when every loss was already noted for this server
	// Days are the exceptions of a series the server did not keep, each by
	// its day (Typecase's Р-60, states-36): the report says them as days,
	// not as EXDATE and RECURRENCE-ID, which then are not in Properties.
	Days []LostDay
}

// LostDay is an exception of a series a server did not keep: the start the
// occurrence has in the series (At — an instant; a whole day's date in UTC
// when Day), and, for an occurrence changed apart (Moved), where it had been
// moved (To, ToDay; zero when it stood on its own start). A day left out
// that came back is not Moved.
type LostDay struct {
	At    time.Time
	Day   bool
	Moved bool
	To    time.Time
	ToDay bool
}

// CompareAfterPut compares sent with got after a successful PUT (S-7.1).
// Server-set stamps and parameter order are ignored; property presence and
// values are compared per value index.
func CompareAfterPut(serverID string, sent, got *Object, agg LossAggregator) *Loss {
	return compare(serverID, sent, got, agg, false)
}

// CompareAfterCreate is CompareAfterPut for a record just made under an
// identity the server chose itself: UID is left out, because a server may name
// the record and rewrite the UID it was sent (docs/servers.md, SV-25) and that
// UID was Carrel's to give, never the person's to lose (S-7.1).
func CompareAfterCreate(serverID string, sent, got *Object, agg LossAggregator) *Loss {
	return compare(serverID, sent, got, agg, true)
}

func compare(serverID string, sent, got *Object, agg LossAggregator, theirUID bool) *Loss {
	if sent == nil || got == nil {
		return nil
	}
	var lost []string
	allAggregated := true
	keys := sent.propKeys()
	for _, prop := range keys {
		if isServerStamp(prop) || theirUID && strings.EqualFold(prop, "UID") {
			continue
		}
		if !propsEqual(sent, got, prop) && !(strings.EqualFold(prop, "DTEND") && impliedEndLeftOut(sent, got)) {
			lost = append(lost, prop)
			if agg == nil || !agg.Has(serverID, prop) {
				allAggregated = false
				if agg != nil {
					agg.Note(serverID, prop)
				}
			}
		}
	}
	if len(lost) == 0 {
		return nil
	}
	sort.Strings(lost)
	return &Loss{Properties: lost, Aggregated: allAggregated && agg != nil}
}

func (o *Object) propKeys() []string {
	p := o.activePayload()
	if p == nil {
		return nil
	}
	out := make([]string, 0, len(p.props))
	for k := range p.props {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func propsEqual(a, b *Object, prop string) bool {
	av := a.propValues(prop)
	bv := b.propValues(prop)
	if len(av) != len(bv) {
		return false
	}
	structured := isStructured(prop)
	rule := isRule(prop)
	day := isCardDay(prop)
	for i := range av {
		if day && sameCardDay(av[i], bv[i]) {
			// One day in another form (vcard_day.go; SV-56).
			continue
		}
		switch {
		case av[i].text == bv[i].text:
		case structured && withoutTrailingEmpty(av[i].text) == withoutTrailingEmpty(bv[i].text):
		case rule && sameRuleParts(av[i].text, bv[i].text):
		default:
			return false
		}
		if !sameParams(av[i].params, withoutAddedPref(av[i].params, bv[i].params)) {
			return false
		}
	}
	return true
}

// withoutAddedPref is the parameters got came back with, less a PREF among
// its TYPE values that sent did not carry. Google marks the first phone,
// address and e-mail of a card PREF and keeps every label it was given
// (docs/servers.md, SV-56): the mark is the server's, and calling the value
// lost told a person of a loss that had not happened. A PREF sent and not
// kept is still compared, and still a loss.
func withoutAddedPref(sent, got map[string]string) map[string]string {
	for k, v := range sent {
		if strings.EqualFold(k, "TYPE") && hasPref(v) {
			return got
		}
	}
	for k, v := range got {
		if !strings.EqualFold(k, "TYPE") || !hasPref(v) {
			continue
		}
		var kept []string
		for _, t := range strings.Split(v, ",") {
			if !strings.EqualFold(strings.TrimSpace(t), "pref") {
				kept = append(kept, t)
			}
		}
		out := maps.Clone(got)
		if len(kept) == 0 {
			delete(out, k)
		} else {
			out[k] = strings.Join(kept, ",")
		}
		return out
	}
	return got
}

func hasPref(types string) bool {
	for _, t := range strings.Split(types, ",") {
		if strings.EqualFold(strings.TrimSpace(t), "pref") {
			return true
		}
	}
	return false
}

// impliedEndLeftOut reports a DTEND the server left out that its DTSTART
// implies anyway. RFC 5545 §3.6.1 ends an event without DTEND or DURATION at
// its start when the start is a time, and a day later when it is a date: the
// end Carrel writes for an event the person gave none (SV-53). Google keeps
// such an event without the DTEND (docs/servers.md, SV-56) — the same event,
// and calling its end lost told a person of a loss that had not happened. The
// start has to have come back as it was sent; an end with another zone or a
// longer span is the person's, and its loss is still reported.
func impliedEndLeftOut(sent, got *Object) bool {
	if got.propValues("DTEND") != nil || got.propValues("DURATION") != nil || !propsEqual(sent, got, "DTSTART") {
		return false
	}
	starts, ends := sent.propValues("DTSTART"), sent.propValues("DTEND")
	if len(starts) != 1 || len(ends) != 1 || !sameParams(starts[0].params, ends[0].params) {
		return false
	}
	start, end := strings.TrimSpace(starts[0].text), strings.TrimSpace(ends[0].text)
	if strings.Contains(start, "T") {
		return start == end
	}
	day, err := time.Parse("20060102", start)
	return err == nil && day.AddDate(0, 0, 1).Format("20060102") == end
}

// isRule names the properties whose value is a set of parts separated by
// semicolons, in no particular order: a recurrence rule and its exception.
func isRule(prop string) bool {
	switch strings.ToUpper(prop) {
	case "RRULE", "EXRULE":
		return true
	}
	return false
}

// sameRuleParts reads a rule as what it is — a set of parts — rather than as
// a line of text. SoGo returns FREQ, COUNT, BYDAY, WKST where Carrel sent
// FREQ, BYDAY, WKST, COUNT; the rule is the same rule, and calling it a
// dropped property would tell a person their series was lost when it was
// not (S-7.1).
func sameRuleParts(a, b string) bool {
	left, right := rulePartSet(a), rulePartSet(b)
	if len(left) != len(right) {
		return false
	}
	for k, v := range left {
		if right[k] != v {
			return false
		}
	}
	return true
}

func rulePartSet(v string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(v, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, val, _ := strings.Cut(part, "=")
		out[strings.ToUpper(strings.TrimSpace(name))] = strings.TrimSpace(val)
	}
	return out
}

func (o *Object) propValues(prop string) []propValue {
	p := o.activePayload()
	if p == nil {
		return nil
	}
	st, ok := p.props[strings.ToUpper(prop)]
	if !ok {
		return nil
	}
	return st.values
}

// Server stamps that may legitimately change on write.
// isStructured names the properties whose value is a list of components
// separated by ';'. A trailing empty component carries nothing: RFC 6350
// §6.2.2 reads a short N as the rest empty, so a server may hand back fewer
// components than it was sent and mean the same name (docs/servers.md, SV-27).
func isStructured(prop string) bool {
	switch strings.ToUpper(prop) {
	case "N", "ADR", "ORG", "GENDER":
		return true
	default:
		return false
	}
}

// withoutTrailingEmpty is a structured value with its trailing empty
// components dropped: "Lovelace;Ada;;;" is the name "Lovelace;Ada".
func withoutTrailingEmpty(v string) string {
	parts := strings.Split(v, ";")
	for len(parts) > 1 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return strings.Join(parts, ";")
}

func isServerStamp(prop string) bool {
	switch strings.ToUpper(prop) {
	case "DTSTAMP", "LAST-MODIFIED", "SEQUENCE", "CREATED", "REV", "VERSION":
		return true
	// The bookkeeping a server writes into a record of its own accord: when
	// an alarm was last acknowledged, which generation of it this is.
	// Mail.ru adds X-MOZ-LASTACK on a write and drops it on the next one,
	// and Carrel, which keeps every line it does not edit, would send it
	// back and report it lost. It is not the person's, and a loss report
	// full of things nobody wrote teaches people to close loss reports.
	case "X-MOZ-LASTACK", "X-MOZ-GENERATION", "X-MOZ-SNOOZE-TIME":
		return true
	default:
		return false
	}
}
