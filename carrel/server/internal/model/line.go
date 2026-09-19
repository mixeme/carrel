package model

import (
	"sort"
	"strings"
)

// The content line both formats share (RFC 5545 §3.1, RFC 6350 §3.3):
// [group "."] name *(";" param) ":" value. It is read here so that a value
// nobody touched can be written back exactly as the server sent it (S-7):
// go-vcard drops groups and every parameter value after the first one, and
// escapes the commas of a list, and a round trip through it changes a card
// that nobody edited.

// Param is one parameter of a content line with all of its values, unquoted.
type Param struct {
	Name   string
	Values []string
}

// Line is one unfolded content line, taken apart.
type Line struct {
	Group  string
	Name   string // upper case
	Params []Param
	Value  string // as it stands on the wire, escapes and all
}

// ParseLine takes an unfolded content line apart. A line that is not one —
// no colon outside quotes, no name, an unterminated quote — answers false: it
// is never a panic, whatever the far server sent.
func ParseLine(s string) (Line, bool) {
	var l Line
	i := 0
	for i < len(s) && s[i] != ';' && s[i] != ':' {
		i++
	}
	if i == len(s) || i == 0 {
		return Line{}, false
	}
	name := s[:i]
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
		l.Group, name = name[:dot], name[dot+1:]
	}
	if name == "" {
		return Line{}, false
	}
	l.Name = strings.ToUpper(name)
	for s[i] == ';' {
		i++
		end := i
		for end < len(s) && s[end] != '=' && s[end] != ';' && s[end] != ':' {
			end++
		}
		if end == len(s) || end == i {
			return Line{}, false
		}
		if s[end] != '=' {
			// A bare parameter (vCard 2.1: TEL;CELL:…) is a TYPE value.
			l.Params = append(l.Params, Param{Name: "TYPE", Values: []string{s[i:end]}})
			i = end
			continue
		}
		p := Param{Name: strings.ToUpper(s[i:end])}
		i = end + 1
		for {
			var v string
			if i < len(s) && s[i] == '"' {
				end := strings.IndexByte(s[i+1:], '"')
				if end < 0 {
					return Line{}, false
				}
				v = s[i+1 : i+1+end]
				i += end + 2
			} else {
				start := i
				for i < len(s) && s[i] != ',' && s[i] != ';' && s[i] != ':' {
					i++
				}
				v = s[start:i]
			}
			p.Values = append(p.Values, v)
			if i >= len(s) {
				return Line{}, false
			}
			if s[i] == ',' {
				i++
				continue
			}
			if s[i] != ';' && s[i] != ':' {
				return Line{}, false
			}
			break
		}
		l.Params = append(l.Params, p)
	}
	if s[i] != ':' {
		return Line{}, false
	}
	l.Value = s[i+1:]
	return l, true
}

// ParamMap is the parameters of a line as RawValue carries them: every value
// of a parameter, joined by commas, under its upper-case name.
func (l Line) ParamMap() map[string]string {
	if len(l.Params) == 0 {
		return nil
	}
	out := make(map[string]string, len(l.Params))
	for _, p := range l.Params {
		if prev, ok := out[p.Name]; ok && prev != "" {
			out[p.Name] = prev + "," + strings.Join(p.Values, ",")
			continue
		}
		out[p.Name] = strings.Join(p.Values, ",")
	}
	return out
}

// listParams hold a list of values by their definition (RFC 6350 §5.6, §5.5,
// §5.9; RFC 5545 §3.2.11, §3.2.4, §3.2.5): their commas are separators. Every
// other parameter holds one value, and a comma in it is part of the value.
var listParams = map[string]bool{
	"TYPE": true, "PID": true, "SORT-AS": true,
	"MEMBER": true, "DELEGATED-TO": true, "DELEGATED-FROM": true,
}

// ParamValues splits a joined parameter back into its values: on the commas
// of a list parameter, and not at all for any other.
func ParamValues(name, joined string) []string {
	if listParams[strings.ToUpper(name)] {
		return strings.Split(joined, ",")
	}
	return []string{joined}
}

// FormatParams writes parameters in the order of their names, each value
// quoted when it holds what would end it (RFC 5545 §3.1, RFC 6350 §3.3). A
// double quote cannot be written inside a quoted value and is dropped.
func FormatParams(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(";")
		b.WriteString(k)
		b.WriteString("=")
		for i, v := range ParamValues(k, params[k]) {
			if i > 0 {
				b.WriteString(",")
			}
			v = strings.ReplaceAll(v, `"`, "")
			if strings.ContainsAny(v, ",;:") {
				b.WriteString(`"` + v + `"`)
			} else {
				b.WriteString(v)
			}
		}
	}
	return b.String()
}

// LogicalLines are the unfolded lines of a document, without terminators.
func LogicalLines(doc []byte) []string {
	var out []string
	for rest := doc; len(rest) > 0; {
		line, tail, _ := cutLine(rest)
		rest = tail
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') && len(out) > 0 {
			out[len(out)-1] += string(line[1:])
			continue
		}
		if len(line) == 0 {
			continue
		}
		out = append(out, string(line))
	}
	return out
}

// sameParams compares parameters as the formats read them: names without
// case, and the values of a parameter as a set, without case. A server that
// writes TYPE=HOME,WORK back as TYPE=work,home has lost nothing.
func sameParams(a, b map[string]string) bool {
	norm := func(m map[string]string) map[string]string {
		out := make(map[string]string, len(m))
		for k, v := range m {
			vals := strings.Split(strings.ToLower(v), ",")
			sort.Strings(vals)
			out[strings.ToUpper(k)] = strings.Join(vals, ",")
		}
		return out
	}
	na, nb := norm(a), norm(b)
	if len(na) != len(nb) {
		return false
	}
	for k, v := range na {
		if w, ok := nb[k]; !ok || w != v {
			return false
		}
	}
	return true
}
