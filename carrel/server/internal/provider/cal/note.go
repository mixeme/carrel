package cal

import (
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-ical"

	"carrel.dev/carrel/server/internal/model"
)

// A note as a file (S-19). Export writes Markdown with a header block, and
// what has no field of its own in that block goes into it all the same: an
// export does not lose. Import reads the same shape back, and a file written
// by hand — no block at all — is a note too.

// noteFence opens and closes the header block.
const noteFence = "---"

// noteFields are the properties the header block names in its own words, in
// the order it writes them. Everything else keeps the name it has on the
// wire, lower-cased.
var noteFields = []struct{ key, prop string }{
	{"title", ical.PropSummary},
	{"date", ical.PropDateTimeStart},
	{"categories", ical.PropCategories},
	{"uid", ical.PropUID},
}

func notePropOf(key string) string {
	for _, f := range noteFields {
		if strings.EqualFold(key, f.key) {
			return f.prop
		}
	}
	return strings.ToUpper(key)
}

func noteKeyOf(prop string) string {
	for _, f := range noteFields {
		if prop == f.prop {
			return f.key
		}
	}
	return strings.ToLower(prop)
}

// noteNamed reports a property the block writes in its own words, and which
// the rest of the block therefore must not write a second time.
func noteNamed(prop string) bool {
	for _, f := range noteFields {
		if prop == f.prop {
			return true
		}
	}
	return false
}

// NoteMarkdown writes one note: the header block, then DESCRIPTION as the
// body, unescaped — a line break is a line break. Values in the block are
// written as the record holds them, escapes and parameters and all, so
// reading the file back gives the record back.
func NoteMarkdown(w io.Writer, obj *model.Object) error {
	if obj == nil {
		return fmt.Errorf("cal: no note")
	}
	props := obj.Props()
	var rest []string
	for name := range props {
		if name == ical.PropDescription || noteNamed(name) {
			continue
		}
		rest = append(rest, name)
	}
	sort.Strings(rest)

	lines := []string{noteFence}
	write := func(prop string) {
		for _, v := range props[prop] {
			lines = append(lines, noteKeyOf(prop)+noteParams(v)+": "+v.Text)
		}
	}
	for _, f := range noteFields {
		write(f.prop)
	}
	for _, name := range rest {
		write(name)
	}
	lines = append(lines, noteFence, "")
	if _, err := io.WriteString(w, strings.Join(lines, "\n")+"\n"); err != nil {
		return err
	}
	body := Description(obj)
	if body == "" {
		return nil
	}
	_, err := io.WriteString(w, body+"\n")
	return err
}

// noteParams writes a value's parameters after its key, as the wire spells
// them: a date in another zone keeps its TZID.
func noteParams(v model.RawValue) string {
	if len(v.Params) == 0 {
		return ""
	}
	names := make([]string, 0, len(v.Params))
	for k := range v.Params {
		names = append(names, k)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, k := range names {
		b.WriteString(";" + k + "=" + v.Params[k])
	}
	return b.String()
}

// ParseNote reads a Markdown file as a note. A file with a header block gives
// back what the block names; a file without one is a note whose body is the
// whole file, and whose title the list takes from its first line — or, for an
// empty file, from its name.
func ParseNote(name string, doc []byte, now time.Time) (*model.Object, error) {
	header, body := cutNoteHeader(rawLines(doc))
	props := map[string][]string{}
	var order []string
	for _, line := range header {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		// The parameters keep the case they were written in: a TZID names a
		// zone, and zones are spelled the way tzdata spells them.
		name, params, _ := strings.Cut(strings.TrimSpace(key), ";")
		prop := notePropOf(name)
		if prop == ical.PropDescription {
			continue // the body is the description; a block cannot hold it too
		}
		if params != "" {
			prop += ";" + params
		}
		if _, seen := props[prop]; !seen {
			order = append(order, prop)
		}
		props[prop] = append(props[prop], strings.TrimSpace(value))
	}
	text := strings.Join(body, "\n")
	out := []string{"BEGIN:" + ical.CompCalendar, ical.PropVersion + ":2.0", ical.PropProductID + ":" + ProdID,
		"BEGIN:" + ical.CompJournal}
	for _, prop := range order {
		for _, v := range props[prop] {
			out = append(out, prop+":"+v)
		}
	}
	// An empty file still names itself: its title is the name it had.
	if strings.TrimSpace(text) == "" && !hasLineProp(out, ical.PropSummary) {
		out = append(out, ical.PropSummary+":"+EscapeText(strings.TrimSuffix(name, path.Ext(name))))
	}
	if !hasLineProp(out, ical.PropUID) {
		uid, err := model.NewUID()
		if err != nil {
			return nil, err
		}
		out = append(out, ical.PropUID+":"+uid)
	}
	if !hasLineProp(out, ical.PropDateTimeStamp) {
		out = append(out, ical.PropDateTimeStamp+":"+now.UTC().Format("20060102T150405Z"))
	}
	if strings.TrimSpace(text) != "" {
		out = append(out, ical.PropDescription+":"+EscapeText(strings.TrimRight(text, "\n")))
	}
	out = append(out, "END:"+ical.CompJournal, "END:"+ical.CompCalendar)
	return Parse("", "", strings.NewReader(strings.Join(out, "\r\n")+"\r\n"))
}

// cutNoteHeader takes the header block off the front of a file: it opens on
// the first line and closes on the next fence. A file that does not open with
// one is all body.
// HasNoteHeader reports whether a Markdown file begins with a header block
// (front matter) a note is read from.
func HasNoteHeader(doc []byte) bool {
	header, _ := cutNoteHeader(rawLines(doc))
	return len(header) > 0
}

func cutNoteHeader(lines []string) (header, body []string) {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != noteFence {
		return nil, lines
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != noteFence {
			continue
		}
		body = lines[i+1:]
		// The blank line after the block belongs to the block, not to the body.
		if len(body) > 0 && strings.TrimSpace(body[0]) == "" {
			body = body[1:]
		}
		return lines[1:i], body
	}
	return nil, lines
}
