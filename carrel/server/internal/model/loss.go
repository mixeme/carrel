package model

import (
	"sort"
	"strings"
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
		if !propsEqual(sent, got, prop) {
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
	for i := range av {
		switch {
		case av[i].text == bv[i].text:
		case structured && withoutTrailingEmpty(av[i].text) == withoutTrailingEmpty(bv[i].text):
		case rule && sameRuleParts(av[i].text, bv[i].text):
		default:
			return false
		}
		if !sameParams(av[i].params, bv[i].params) {
			return false
		}
	}
	return true
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
	default:
		return false
	}
}
