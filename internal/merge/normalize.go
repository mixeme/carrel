// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package merge

import (
	"sort"
	"strings"
	"unicode"
)

// phoneKeyDigits is how many trailing digits identify a number.
//
// §15 asks for E.164, which cannot be reached from a card alone: a number
// written without a country code has no country in it, and guessing one from
// the instance's locale would be wrong for every person who has ever travelled.
// The comparable form is therefore the significant tail — enough digits that a
// collision is improbable, few enough that +7 495 123-45-67, 8 495 123-45-67 and
// 495 123-45-67 are one number, which is how the same person ends up written in
// two address books.
const phoneKeyDigits = 9

// phoneMinDigits is the shortest number that identifies anybody. Below it lies
// the short codes — 112, an internal extension — which say nothing about who a
// card is about.
const phoneMinDigits = 5

// NormalizePhone folds a telephone number towards E.164: separators and letters
// dropped, an international prefix written as +, the rest left as the card wrote
// it. It is what a merged card shows; comparison uses PhoneKey.
func NormalizePhone(value string) string {
	digits, plus := phoneDigits(value)
	if digits == "" {
		return ""
	}
	if plus {
		return "+" + digits
	}
	return digits
}

// PhoneKey returns the comparable tail of a number, or "" when there is nothing
// comparable in it.
func PhoneKey(value string) string {
	digits, _ := phoneDigits(value)
	if len(digits) < phoneMinDigits {
		return ""
	}
	if len(digits) <= phoneKeyDigits {
		return digits
	}
	return digits[len(digits)-phoneKeyDigits:]
}

// phoneDigits strips a written number to its digits, reporting whether it was
// written in international form. A leading 00 is the same prefix as +, and an
// extension after a separator is kept: dropping it would merge two desks.
func phoneDigits(value string) (string, bool) {
	s := strings.TrimSpace(value)
	if s == "" {
		return "", false
	}
	if i := strings.Index(s, ":"); i >= 0 {
		// tel: URIs, which is how a vCard 4.0 card writes a number.
		if scheme := strings.ToLower(s[:i]); scheme == "tel" {
			s = s[i+1:]
		}
	}
	// A tel: URI may carry parameters; they are not part of the number.
	if i := strings.IndexByte(s, ';'); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)

	var b strings.Builder
	plus := strings.HasPrefix(s, "+")
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	if !plus && strings.HasPrefix(digits, "00") && len(digits) > 2 {
		plus = true
		digits = digits[2:]
	}
	return digits, plus
}

// NormalizeName folds a display name to the form two records are compared under:
// case dropped, punctuation dropped, whitespace collapsed and the words sorted.
// Sorting is what makes "Ivan Petrov" and "Petrov Ivan" the same name, which is
// the difference between two clients rather than between two people (§15).
func NormalizeName(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteByte(' ')
		}
	}
	words := strings.Fields(b.String())
	if len(words) == 0 {
		return ""
	}
	sort.Strings(words)
	return strings.Join(words, " ")
}

// BirthdayKey reduces a BDAY to its digits, so 1985-05-10 and 19850510 are the
// same date. A partial date keeps its own length and is compared as a suffix.
func BirthdayKey(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
		if b.Len() == 8 {
			break
		}
	}
	return b.String()
}

// birthdayMatch compares two BDAY keys. A vCard may carry a date with no year
// (--0510), and a card that gives the day and the month agrees with one that
// also gives the year: it is the same birthday, written by a client that had
// less to write.
func birthdayMatch(a, b string) bool {
	if len(a) < 4 || len(b) < 4 {
		return false
	}
	if a == b {
		return true
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	return strings.HasSuffix(b, a)
}

// similarity is the Levenshtein distance of two normalised names as a ratio
// between 0 and 100, where 100 is an exact match.
func similarity(a, b string) int {
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 100
	}
	distance := levenshtein([]rune(a), []rune(b))
	longest := len([]rune(a))
	if n := len([]rune(b)); n > longest {
		longest = n
	}
	if longest == 0 {
		return 0
	}
	return 100 - (distance * 100 / longest)
}

// levenshtein is the edit distance between two rune slices, computed over two
// rows rather than a full matrix.
func levenshtein(a, b []rune) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	previous := make([]int, len(b)+1)
	current := make([]int, len(b)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := 1; i <= len(a); i++ {
		current[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			current[j] = min3(current[j-1]+1, previous[j]+1, previous[j-1]+cost)
		}
		previous, current = current, previous
	}
	return previous[len(b)]
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}
