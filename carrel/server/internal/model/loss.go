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
	for i := range av {
		if av[i].text != bv[i].text {
			return false
		}
		if !paramsEqual(av[i].params, bv[i].params) {
			return false
		}
	}
	return true
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

func paramsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[strings.ToUpper(k)] != v {
			return false
		}
	}
	return true
}

// Server stamps that may legitimately change on write.
func isServerStamp(prop string) bool {
	switch strings.ToUpper(prop) {
	case "DTSTAMP", "LAST-MODIFIED", "SEQUENCE", "CREATED", "REV", "VERSION":
		return true
	default:
		return false
	}
}
