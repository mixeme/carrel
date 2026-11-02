package card

import (
	"sort"
	"strings"

	"github.com/emersion/go-vcard"

	"carrel.dev/carrel/server/internal/model"
)

// A card is read twice: go-vcard gives the values their meaning (it unescapes
// them), and the content lines themselves give what go-vcard drops — groups,
// every value of a parameter, the escapes of a list — so an untouched value can
// go back exactly as it came (S-7).

// readCard reads one vCard with go-vcard, its bare parameters named first.
// vCard 2.1 writes a type without its name — TEL;HOME;VOICE:…,
// EMAIL;INTERNET:… —, and go-vcard v0.1.0 takes the value of such a line for
// the last parameter's and leaves the line empty (found 02.11.2026): a bare
// parameter is a TYPE, as model.ParseLine reads it, or an ENCODING when it
// names one. The lines the card keeps for writing (cardBlocks) are the ones
// it came in.
func readCard(text string) (vcard.Card, error) {
	return vcard.NewDecoder(strings.NewReader(namedParams(text))).Decode()
}

// hasBareParam reports a card with a parameter without a name.
func hasBareParam(text string) bool { return namedParams(text) != text }

// namedParams is text with every bare parameter named; text itself when it
// has none.
func namedParams(text string) string {
	lines := model.LogicalLines([]byte(text))
	changed := false
	for i, l := range lines {
		if named, ok := nameBare(l); ok {
			lines[i], changed = named, true
		}
	}
	if !changed {
		return text
	}
	return strings.Join(lines, "\r\n") + "\r\n"
}

// bareEncodings are the bare parameters of vCard 2.1 that name an encoding.
var bareEncodings = map[string]bool{"QUOTED-PRINTABLE": true, "BASE64": true, "8BIT": true, "7BIT": true}

// nameBare writes the bare parameters of one content line with their
// names; ok is false for a line that has none.
func nameBare(line string) (string, bool) {
	// The head ends at the first colon outside quotes.
	quoted, end := false, -1
	for i := 0; i < len(line) && end < 0; i++ {
		switch line[i] {
		case '"':
			quoted = !quoted
		case ':':
			if !quoted {
				end = i
			}
		}
	}
	if end < 0 {
		return line, false
	}
	var parts []string
	start := 0
	quoted = false
	for i := 0; i <= end; i++ {
		if i < end && line[i] == '"' {
			quoted = !quoted
		}
		if i == end || line[i] == ';' && !quoted {
			parts = append(parts, line[start:i])
			start = i + 1
		}
	}
	changed := false
	for i := 1; i < len(parts); i++ {
		p := parts[i]
		if p == "" || strings.ContainsRune(p, '=') {
			continue
		}
		if bareEncodings[strings.ToUpper(p)] {
			parts[i] = "ENCODING=" + p
		} else {
			parts[i] = "TYPE=" + p
		}
		changed = true
	}
	if !changed {
		return line, false
	}
	return strings.Join(parts, ";") + line[end:], true
}

// cardToProps turns a decoded card into raw values. lines are the logical
// lines of the same card; nil, or lines that do not match the card, leave the
// values without a wire line, and they are written fresh.
func cardToProps(card vcard.Card, lines []string) map[string][]model.RawValue {
	byName := linesByName(lines)
	out := make(map[string][]model.RawValue)
	for name, fields := range card {
		key := strings.ToUpper(name)
		wires := byName[key]
		if len(wires) != len(fields) {
			wires = nil
		}
		vals := make([]model.RawValue, len(fields))
		for i, f := range fields {
			if wires != nil {
				l := wires[i].line
				vals[i] = model.RawValue{Text: f.Value, Params: l.ParamMap(), Group: l.Group, Wire: wires[i].wire}
				continue
			}
			vals[i] = fieldToRaw(f)
		}
		out[key] = vals
	}
	return out
}

type wireLine struct {
	line model.Line
	wire string
}

// linesByName groups the content lines of one card by property name, in the
// order they stand — the order go-vcard keeps the fields of one name in. A
// line that does not parse spoils the match of the whole card: go-vcard read
// it as something, and guessing what is worse than writing fresh.
func linesByName(lines []string) map[string][]wireLine {
	if lines == nil {
		return nil
	}
	out := map[string][]wireLine{}
	for _, s := range lines {
		l, ok := model.ParseLine(s)
		if !ok {
			return nil
		}
		if l.Name == "BEGIN" || l.Name == "END" {
			continue
		}
		out[l.Name] = append(out[l.Name], wireLine{line: l, wire: s})
	}
	return out
}

func fieldToRaw(f *vcard.Field) model.RawValue {
	rv := model.RawValue{Text: f.Value, Group: f.Group}
	if len(f.Params) > 0 {
		rv.Params = make(map[string]string)
		for k, vs := range f.Params {
			if len(vs) > 0 {
				rv.Params[strings.ToUpper(k)] = strings.Join(vs, ",")
			}
		}
	}
	return rv
}

// cardBlocks cuts a document into the logical lines of each card in it,
// BEGIN:VCARD to END:VCARD, in order.
func cardBlocks(doc []byte) [][]string {
	var out [][]string
	var cur []string
	depth := 0
	for _, l := range model.LogicalLines(doc) {
		up := strings.ToUpper(strings.TrimSpace(l))
		switch {
		case up == "BEGIN:VCARD":
			depth++
			if depth == 1 {
				cur = nil
			}
		case up == "END:VCARD":
			depth--
			if depth == 0 {
				out = append(out, append(cur, l))
				cur = nil
				continue
			}
		}
		if depth > 0 {
			cur = append(cur, l)
		}
	}
	return out
}

// listProps are the text properties whose value is a comma-separated list
// (RFC 6350 §6.2.3, §6.7.1): their commas are separators, not text.
var listProps = map[string]bool{"CATEGORIES": true, "NICKNAME": true}

// uriProps hold a URI to binary content, and a data: URI keeps its comma
// (RFC 2397).
var uriProps = map[string]bool{"PHOTO": true, "LOGO": true, "SOUND": true, "KEY": true}

// formatValue escapes a value the way go-vcard reads it back: backslash,
// newline and comma — except the comma of a list and of a data: URI.
func formatValue(name, v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, "\r\n", `\n`)
	v = strings.ReplaceAll(v, "\n", `\n`)
	if listProps[name] || uriProps[name] && isDataURI(v) {
		return v
	}
	return strings.ReplaceAll(v, ",", `\,`)
}

// contentLine is a value as it goes on the wire: its own line when nobody
// touched it, otherwise written fresh.
func contentLine(name string, v model.RawValue) string {
	if v.Wire != "" {
		return v.Wire
	}
	var b strings.Builder
	if v.Group != "" {
		b.WriteString(v.Group + ".")
	}
	b.WriteString(name)
	b.WriteString(model.FormatParams(v.Params))
	b.WriteString(":")
	b.WriteString(formatValue(name, v.Text))
	return b.String()
}

// encodeCard writes the card: BEGIN, VERSION, the rest by name, END — the
// order go-vcard used, so a card written before and after this reads the
// same.
func encodeCard(props map[string][]model.RawValue) []byte {
	var b strings.Builder
	b.WriteString("BEGIN:VCARD\r\n")
	version := props[vcard.FieldVersion]
	if len(version) == 0 {
		version = []model.RawValue{{Text: "4.0"}}
	}
	b.WriteString(contentLine(vcard.FieldVersion, version[0]) + "\r\n")
	names := make([]string, 0, len(props))
	for n := range props {
		if n != vcard.FieldVersion && n != "BEGIN" && n != "END" {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		for _, v := range props[n] {
			b.WriteString(contentLine(n, v) + "\r\n")
		}
	}
	b.WriteString("END:VCARD\r\n")
	return []byte(b.String())
}
