package model

import (
	"maps"
	"strings"
)

// The days of a card — BDAY, ANNIVERSARY, Apple's X-ABDATE — are one day in
// several forms: 1990-04-13 and 19900413; a day without a year as vCard 4.0
// writes it, --0413 or --04-13, and as Apple writes it in vCard 3.0, which
// has none (RFC 2426): 1604-04-13 with X-APPLE-OMIT-YEAR=1604 naming the
// placeholder year. Google keeps that marker inside VALUE —
// BDAY;VALUE=DATE,X-APPLE-OMIT-YEAR=1604:1604-04-13 (docs/servers.md, SV-56;
// found live 04.11.2026) — and VALUE=DATE is what a day is anyway.

// appleOmitYear is the parameter Apple names a placeholder year by.
const appleOmitYear = "X-APPLE-OMIT-YEAR"

// AppleOmitYear is the year X-APPLE-OMIT-YEAR names among params, as Apple
// writes it or — folded, true — inside VALUE, as Google keeps it; "" when
// the value carries no such marker.
func AppleOmitYear(params map[string]string) (year string, folded bool) {
	for k, v := range params {
		if strings.EqualFold(k, appleOmitYear) {
			return strings.TrimSpace(v), false
		}
	}
	for k, v := range params {
		if !strings.EqualFold(k, "VALUE") {
			continue
		}
		for _, part := range strings.Split(v, ",") {
			if name, year, ok := strings.Cut(strings.TrimSpace(part), "="); ok && strings.EqualFold(name, appleOmitYear) {
				return strings.TrimSpace(year), true
			}
		}
	}
	return "", false
}

// isCardDay reports a property whose value is a day of the card.
func isCardDay(prop string) bool {
	switch strings.ToUpper(prop[strings.LastIndex(prop, ".")+1:]) {
	case "BDAY", "ANNIVERSARY", "X-ABDATE":
		return true
	}
	return false
}

// cardDay is a day of a card as one form: YYYYMMDD, or --MMDD without a
// year; and the parameters that are not the day's form — VALUE=DATE and the
// marker of the placeholder year taken out. ok is false for what is not a
// day: a time, a text; such a value is compared as it is.
func cardDay(text string, params map[string]string) (day string, rest map[string]string, ok bool) {
	v := strings.TrimSpace(text)
	year, _ := AppleOmitYear(params)
	yearless := strings.HasPrefix(v, "--")
	if year != "" && strings.HasPrefix(v, year) {
		v, yearless = strings.TrimPrefix(v, year), true
	}
	digits := strings.ReplaceAll(strings.TrimPrefix(v, "--"), "-", "")
	switch {
	case yearless && len(digits) == 4 && allDigits(digits):
		day = "--" + digits
	case !yearless && len(digits) == 8 && allDigits(digits):
		day = digits
	default:
		return "", nil, false
	}
	rest = maps.Clone(params)
	for k, v := range rest {
		switch {
		case strings.EqualFold(k, appleOmitYear):
			delete(rest, k)
		case strings.EqualFold(k, "VALUE"):
			var kept []string
			for _, part := range strings.Split(v, ",") {
				part = strings.TrimSpace(part)
				name, _, _ := strings.Cut(part, "=")
				if !strings.EqualFold(part, "DATE") && !strings.EqualFold(name, appleOmitYear) {
					kept = append(kept, part)
				}
			}
			if len(kept) == 0 {
				delete(rest, k)
			} else {
				rest[k] = strings.Join(kept, ",")
			}
		}
	}
	return day, rest, true
}

// sameCardDay reports two values of a card's day that are one day in two
// forms, with the same other parameters.
func sameCardDay(a, b propValue) bool {
	da, ra, oka := cardDay(a.text, a.params)
	db, rb, okb := cardDay(b.text, b.params)
	return oka && okb && da == db && sameParams(ra, rb)
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
