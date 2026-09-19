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
