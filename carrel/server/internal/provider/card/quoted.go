package card

import (
	"bytes"
	"fmt"
	"io"
	"mime/quotedprintable"
	"strings"
)

// The line level of a .vcf as an import reads it: lines unfolded, each with
// the line of the file it begins on, so that a card refused says where
// (Typecase's notes 111 p. 6, 113; s-io-5: «line 12: cannot decode
// quoted-printable»). vCard 2.1 writes a value quoted-printable and breaks
// it with a «=» at the end of a line, not with a fold.

// numberedLines are the logical lines of a file and the line each begins on.
func numberedLines(doc []byte) (lines []string, at []int) {
	raw := strings.Split(strings.ReplaceAll(string(doc), "\r\n", "\n"), "\n")
	quoted := false // the line before ended a quoted-printable value with «=»
	for i, l := range raw {
		l = strings.TrimSuffix(l, "\r")
		switch {
		case len(lines) > 0 && quoted:
			lines[len(lines)-1] = strings.TrimSuffix(lines[len(lines)-1], "=") + l
		case len(lines) > 0 && (strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t")):
			lines[len(lines)-1] += l[1:]
		case strings.TrimSpace(l) == "":
			continue
		default:
			lines, at = append(lines, l), append(at, i+1)
		}
		last := lines[len(lines)-1]
		quoted = isQuoted(last) && strings.HasSuffix(last, "=")
	}
	return lines, at
}

// isQuoted says whether a content line's value is quoted-printable: an
// ENCODING=QUOTED-PRINTABLE parameter, or the bare QUOTED-PRINTABLE of 2.1.
func isQuoted(line string) bool {
	colon := strings.IndexByte(line, ':')
	if colon < 0 {
		return false
	}
	for _, p := range strings.Split(line[:colon], ";")[1:] {
		p = strings.ToUpper(strings.TrimSpace(p))
		if p == "QUOTED-PRINTABLE" || p == "ENCODING=QUOTED-PRINTABLE" {
			return true
		}
	}
	return false
}

// decodeQuoted decodes the quoted-printable values of a card's lines and
// takes the encoding off them; a value that cannot be decoded is an error
// that names its line.
func decodeQuoted(lines []string, at []int) ([]string, error) {
	out := make([]string, len(lines))
	for i, l := range lines {
		if !isQuoted(l) {
			out[i] = l
			continue
		}
		colon := strings.IndexByte(l, ':')
		head, value := l[:colon], l[colon+1:]
		b, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(value)))
		if err != nil || !strictQuoted(value) || bytes.ContainsRune(b, 0) {
			return nil, fmt.Errorf("line %d: cannot decode quoted-printable", at[i])
		}
		var params []string
		for _, p := range strings.Split(head, ";")[1:] {
			up := strings.ToUpper(strings.TrimSpace(p))
			if up == "QUOTED-PRINTABLE" || up == "ENCODING=QUOTED-PRINTABLE" || strings.HasPrefix(up, "CHARSET=") {
				continue
			}
			params = append(params, p)
		}
		name := strings.Split(head, ";")[0]
		text := strings.NewReplacer("\r\n", "\n", "\n", "\n").Replace(string(b))
		out[i] = strings.Join(append([]string{name}, params...), ";") + ":" + text
	}
	return out, nil
}

// strictQuoted says whether every «=» of a value begins an escape of two
// hex digits: the decoder passes a broken one through as it stands, and a
// value read so is not what the file meant.
func strictQuoted(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] != '=' {
			continue
		}
		if i+2 >= len(value) {
			return false
		}
		if !isHex(value[i+1]) || !isHex(value[i+2]) {
			return false
		}
		i += 2
	}
	return true
}

func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'A' <= c && c <= 'F' || 'a' <= c && c <= 'f'
}
