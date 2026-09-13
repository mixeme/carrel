package web

import (
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// cyrillic is the closed transliteration table of the collection slug
// (architecture.md). It runs after NFD, so й, ё, ї and ў arrive as a base
// letter and a combining mark, and the mark falls out with everything else
// outside [a-z0-9-].
var cyrillic = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ж': "zh", 'з': "z",
	'и': "i", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o", 'п': "p", 'р': "r",
	'с': "s", 'т': "t", 'у': "u", 'ф': "f", 'х': "kh", 'ц': "ts", 'ч': "ch", 'ш': "sh",
	'щ': "shch", 'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
	'і': "i", 'є': "ye", 'ґ': "g",
}

// slugFallback is the address of a name that leaves nothing behind.
const slugFallback = "col"

// collectionSlug derives the address segment of a new collection from its
// name (S-12.2): NFD, the table, lower case, a run of spaces or hyphens to one
// hyphen, then [a-z0-9-] only; empty becomes col.
func collectionSlug(name string) string {
	var b strings.Builder
	pending := false
	for _, r := range norm.NFD.String(name) {
		r = unicode.ToLower(r)
		if unicode.IsSpace(r) || r == '-' {
			pending = b.Len() > 0
			continue
		}
		out, ok := cyrillic[r]
		if !ok {
			out = string(r)
		}
		for _, c := range out {
			if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
				if pending {
					b.WriteByte('-')
					pending = false
				}
				b.WriteRune(c)
			}
		}
	}
	if b.Len() == 0 {
		return slugFallback
	}
	return b.String()
}

// uniqueSlug adds a numeric suffix while the segment is taken in the parent.
// Taken holds lower-case segments: a server on a case-insensitive store would
// refuse Work next to work.
func uniqueSlug(base string, taken map[string]bool) string {
	if !taken[strings.ToLower(base)] {
		return base
	}
	for n := 2; ; n++ {
		if c := base + "-" + strconv.Itoa(n); !taken[strings.ToLower(c)] {
			return c
		}
	}
}

// addressProblem checks an address segment typed by hand. Empty is no
// problem: the address then comes from the name. The segment is kept as typed,
// so it has to be one that every server and client spells the same way.
func addressProblem(seg string) string {
	switch {
	case seg == "":
		return ""
	case strings.Contains(seg, "/"):
		return "An address is one segment: no slash."
	case strings.HasPrefix(seg, "."):
		return "An address may not start with a dot."
	case strings.Contains(seg, ".."):
		return "An address may not contain two dots in a row."
	}
	for _, r := range seg {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-._~", r)) {
			return "Use Latin letters, digits, hyphens, dots, underscores and tildes."
		}
	}
	return ""
}
