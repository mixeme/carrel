package model

import (
	"bytes"
	"unicode/utf8"
)

// foldAt is the line length both formats ask for: 75 octets, not counting the
// CRLF (RFC 5545 §3.1, RFC 6350 §3.2).
const foldAt = 75

// FoldLines folds the lines of a written record the way both formats ask:
// nothing longer than 75 octets, a fold being a CRLF and one space, which
// unfolding removes. It is the same rule in iCalendar (RFC 5545 §3.1) and in
// vCard (RFC 6350 §3.2), so it is written once and used by both providers.
//
// Neither go-ical nor go-vcard folds, and neither will: upstream refused the
// feature and a pull request for it twice over, because folding is a SHOULD
// (docs/servers.md, SV-31). So an inline attachment or a photo went out as one
// line of hundreds of kilobytes, and a parser with a line limit had every
// right to refuse it.
//
// A fold never falls inside a character: RFC 6350 §3.2 recommends it, and a
// split rune would be a broken record rather than a long line. Folding an
// already folded document changes nothing, every line being short by then.
func FoldLines(doc []byte) []byte {
	if !needsFolding(doc) {
		return doc
	}
	var out bytes.Buffer
	out.Grow(len(doc) + len(doc)/foldAt*3)
	for rest := doc; len(rest) > 0; {
		line, tail, more := cutLine(rest)
		rest = tail
		writeFolded(&out, line)
		if more {
			out.WriteString("\r\n")
		}
	}
	return out.Bytes()
}

// needsFolding answers whether any line is too long, so a document that is
// already short is handed back as it came, without a copy.
func needsFolding(doc []byte) bool {
	for rest := doc; len(rest) > 0; {
		line, tail, _ := cutLine(rest)
		if len(line) > foldAt {
			return true
		}
		rest = tail
	}
	return false
}

// cutLine takes the next line off doc, whatever it ends with: the encoders
// write CRLF, and a document from elsewhere may hold LF alone. more is false
// for the last line, so a document that ended without a break is written back
// the same way.
func cutLine(doc []byte) (line, rest []byte, more bool) {
	i := bytes.IndexByte(doc, '\n')
	if i < 0 {
		return doc, nil, false
	}
	line = doc[:i]
	if n := len(line); n > 0 && line[n-1] == '\r' {
		line = line[:n-1]
	}
	return line, doc[i+1:], true
}

// writeFolded writes one line, broken into pieces of at most 75 octets. The
// space that opens a continuation counts toward the length, and the break
// moves back to a character boundary rather than through a rune.
func writeFolded(out *bytes.Buffer, line []byte) {
	if len(line) <= foldAt {
		out.Write(line)
		return
	}
	out.Write(line[:boundary(line, foldAt)])
	line = line[boundary(line, foldAt):]
	for len(line) > 0 {
		n := boundary(line, foldAt-1)
		out.WriteString("\r\n ")
		out.Write(line[:n])
		line = line[n:]
	}
}

// boundary is the largest cut of at most n octets that does not fall inside a
// character. A single rune longer than n is not a case any format produces,
// and the whole rune goes rather than half of it.
func boundary(line []byte, n int) int {
	if len(line) <= n {
		return len(line)
	}
	for i := n; i > 0; i-- {
		if utf8.RuneStart(line[i]) {
			return i
		}
	}
	_, size := utf8.DecodeRune(line)
	return size
}
