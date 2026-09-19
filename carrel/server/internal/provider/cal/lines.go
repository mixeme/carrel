package cal

import "strings"

// The line level of iCalendar (RFC 5545 §3.1). An import cuts a file into
// records and an export copies a record out; neither decodes the other
// program's bytes, so what it wrote stays as it wrote it — folding, order and
// all. Only the names of properties and components are read here.

// rawLines splits a document into physical lines without their terminators.
func rawLines(doc []byte) []string {
	s := strings.ReplaceAll(string(doc), "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// isFold reports a continuation of the line before it.
func isFold(line string) bool {
	return strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
}

// unfoldAt joins the line at i with its continuations.
func unfoldAt(lines []string, i int) string {
	var b strings.Builder
	b.WriteString(lines[i])
	for j := i + 1; j < len(lines) && isFold(lines[j]); j++ {
		b.WriteString(lines[j][1:])
	}
	return b.String()
}

// logicalLines are the unfolded lines of a block.
func logicalLines(lines []string) []string {
	var out []string
	for i := range lines {
		if !isFold(lines[i]) {
			out = append(out, unfoldAt(lines, i))
		}
	}
	return out
}

// propOf is the name and value of a content line; a BEGIN or an END is not
// one.
func propOf(line string) (name, value string, ok bool) {
	colon := strings.IndexByte(line, ':')
	if colon < 0 {
		return "", "", false
	}
	name = line[:colon]
	if semi := strings.IndexByte(name, ';'); semi >= 0 {
		name = name[:semi]
	}
	name = strings.ToUpper(strings.TrimSpace(name))
	if name == "" || name == "BEGIN" || name == "END" {
		return "", "", false
	}
	return name, line[colon+1:], true
}

func beginName(line string) (string, bool) {
	name, ok := strings.CutPrefix(strings.ToUpper(line), "BEGIN:")
	return strings.TrimSpace(name), ok
}

func isEnd(line string) bool {
	return strings.HasPrefix(strings.ToUpper(line), "END:")
}

// matchingEnd is the index of the END that closes the BEGIN at begin, or the
// length of the block when the file ends first.
func matchingEnd(lines []string, begin int) int {
	depth := 0
	for i := begin; i < len(lines); i++ {
		switch {
		case isFold(lines[i]):
		case strings.HasPrefix(strings.ToUpper(lines[i]), "BEGIN:"):
			depth++
		case isEnd(lines[i]):
			if depth--; depth == 0 {
				return i
			}
		}
	}
	return len(lines)
}

// ownLines are the lines of a component that belong to it and not to a
// component nested in it.
func ownLines(block []string) []string {
	if len(block) == 0 {
		return nil
	}
	var out []string
	for i := 1; i < len(block); i++ {
		if _, ok := beginName(block[i]); ok {
			i = matchingEnd(block, i)
			continue
		}
		if isEnd(block[i]) {
			continue
		}
		out = append(out, block[i])
	}
	return out
}

func hasProp(block []string, name string) bool {
	for _, line := range logicalLines(ownLines(block)) {
		if n, _, ok := propOf(line); ok && n == name {
			return true
		}
	}
	return false
}

// replaceProp writes one value over every occurrence of a property in the
// lines, continuations included, and says how many it replaced.
func replaceProp(lines []string, name, value string) ([]string, int) {
	out := make([]string, 0, len(lines))
	replaced := 0
	for i := 0; i < len(lines); i++ {
		if isFold(lines[i]) {
			out = append(out, lines[i])
			continue
		}
		if n, _, ok := propOf(unfoldAt(lines, i)); ok && n == name {
			out = append(out, name+":"+value)
			replaced++
			for i+1 < len(lines) && isFold(lines[i+1]) {
				i++
			}
			continue
		}
		out = append(out, lines[i])
	}
	return out, replaced
}

// mainLines are the own content lines, unfolded, of the component go-ical
// takes as the record: the first child of VCALENDAR named main that carries
// no RECURRENCE-ID (findMain).
func mainLines(doc []byte, main string) []string {
	lines := rawLines(doc)
	for i := 0; i < len(lines); i++ {
		name, ok := beginName(lines[i])
		if !ok {
			continue
		}
		if name == "VCALENDAR" {
			continue
		}
		end := matchingEnd(lines, i)
		if name == main && !hasProp(lines[i:end+1], "RECURRENCE-ID") {
			return logicalLines(ownLines(lines[i : end+1]))
		}
		i = end
	}
	return nil
}
