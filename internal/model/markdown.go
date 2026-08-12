// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/emersion/go-ical"
)

// MarkdownSource records where an exported note came from, so a directory of
// files still says which account and collection each one was taken out of
// (§23.9).
type MarkdownSource struct {
	Account    string
	Collection string
}

// MarkdownNote is a note as a Markdown file: the front matter fields §23.9
// names, the properties that did not map to one of them, and the body.
type MarkdownNote struct {
	Title    string
	Date     time.Time
	DateOnly bool
	HasDate  bool
	Tags     []string
	UID      string
	Related  []Relation
	// Attachments are the `ATTACH` links of §23.10, read back as URIs.
	Attachments []string
	Source      MarkdownSource
	Body        string
	// Extra carries iCalendar properties with no front matter field of their
	// own, under their own names. Export must not lose what §8 goes to such
	// lengths to keep.
	Extra []Property
}

const markdownFence = "---"

// RenderMarkdown writes a note as YAML front matter followed by the body.
func RenderMarkdown(note Note, src MarkdownSource) []byte {
	var buf bytes.Buffer
	buf.WriteString(markdownFence + "\n")
	writeYAMLString(&buf, "title", note.DisplayTitle())
	if !note.Date.IsZero() {
		if note.DateOnly {
			writeYAMLString(&buf, "date", note.Date.Format("2006-01-02"))
		} else {
			writeYAMLString(&buf, "date", note.Date.Format(time.RFC3339))
		}
	}
	if len(note.Categories) > 0 {
		writeYAMLList(&buf, "tags", note.Categories)
	}
	writeYAMLString(&buf, "uid", note.UID)
	if len(note.Related) > 0 {
		writeYAMLList(&buf, "related", RelationUIDs(note.Related))
	}
	// Attachments travel as their links (§23.10). An attachment another client
	// embedded as base64 is not carried into Markdown: it is read-only anyway,
	// and a note file with a megabyte of encoded image in its front matter is
	// not a note file. What is skipped is said in the docs rather than left to
	// be discovered.
	if links := AttachmentLinks(note.Attachments); len(links) > 0 {
		writeYAMLList(&buf, "attachments", links)
	}
	if src.Account != "" {
		writeYAMLString(&buf, "account", src.Account)
	}
	if src.Collection != "" {
		writeYAMLString(&buf, "collection", src.Collection)
	}
	if len(note.Other) > 0 {
		buf.WriteString("carrel_properties:\n")
		for _, prop := range note.Other {
			for _, value := range prop.Values {
				buf.WriteString("  - ")
				writeYAMLInline(&buf, prop.Name+": "+value.Text)
				buf.WriteString("\n")
			}
		}
	}
	buf.WriteString(markdownFence + "\n\n")
	body := strings.TrimRight(note.Description, "\n")
	if body != "" {
		buf.WriteString(body)
		buf.WriteString("\n")
	}
	return buf.Bytes()
}

// MarkdownFilename builds the name a note is exported under: the date, the
// title transliterated and stripped of anything a file system objects to, and
// the UID appended when that is not yet unique. Nothing is ever silently
// overwritten (§23.9).
func MarkdownFilename(note Note, taken map[string]bool) string {
	stem := slug(note.DisplayTitle())
	if stem == "" {
		stem = "note"
	}
	prefix := ""
	if !note.Date.IsZero() {
		prefix = note.Date.Format("2006-01-02") + "-"
	}
	name := prefix + stem + ".md"
	if taken != nil && taken[strings.ToLower(name)] {
		suffix := slug(note.UID)
		if suffix == "" {
			suffix = "dup"
		}
		name = prefix + stem + "-" + suffix + ".md"
		for n := 2; taken[strings.ToLower(name)]; n++ {
			name = fmt.Sprintf("%s%s-%s-%d.md", prefix, stem, suffix, n)
		}
	}
	if taken != nil {
		taken[strings.ToLower(name)] = true
	}
	return name
}

// maxSlugRunes keeps a generated name well inside the shortest path limit any
// of the target file systems imposes.
const maxSlugRunes = 60

// Slug turns a title into a file-name stem: transliterated, lower case, and
// stripped of anything a file system objects to. §23.10 asks for attachment
// names built from the date and the entry's title, and they have to be the same
// kind of name a Markdown export produces or the folder reads as two schemes.
func Slug(s string) string { return slug(s) }

func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if repl, ok := translit[r]; ok {
			b.WriteString(repl)
			dash = false
			continue
		}
		switch {
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	runes := []rune(out)
	if len(runes) > maxSlugRunes {
		out = strings.Trim(string(runes[:maxSlugRunes]), "-")
	}
	return out
}

// translit maps the letters Carrel's own users type into ASCII. Anything not
// listed becomes a separator, which is why the UID suffix exists.
var translit = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "e",
	'ж': "zh", 'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m",
	'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u",
	'ф': "f", 'х': "h", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "sch",
	'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
	'ä': "ae", 'ö': "oe", 'ü': "ue", 'ß': "ss", 'é': "e", 'è': "e",
	'ê': "e", 'à': "a", 'â': "a", 'ç': "c", 'î': "i", 'ï': "i", 'ô': "o",
	'ù': "u", 'û': "u", 'ñ': "n", 'å': "a", 'ø': "o", 'æ': "ae",
}

// ParseMarkdown reads one Markdown file into note fields.
//
// Front matter is read when present and is not required: a folder of plain
// files from Obsidian or a dump from somewhere else has to import too, or the
// feature is useless for the migration it exists for (§23.9). Without it the
// title comes from the first heading or the file name, and the date from the
// file name or the modification time the caller passes in.
func ParseMarkdown(filename string, body []byte, mtime time.Time) (MarkdownNote, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return MarkdownNote{}, fmt.Errorf("markdown: %s is empty", filename)
	}
	front, rest := splitFrontMatter(body)
	note := MarkdownNote{Body: strings.TrimSpace(string(rest))}
	if front != nil {
		if err := note.readFrontMatter(front); err != nil {
			return MarkdownNote{}, fmt.Errorf("markdown: %s: %w", filename, err)
		}
	}
	if note.Title == "" {
		note.Title, note.Body = titleFromBody(note.Body)
	}
	base := baseName(filename)
	if !note.HasDate {
		if date, ok := dateFromName(base); ok {
			note.Date, note.DateOnly, note.HasDate = date, true, true
		} else if !mtime.IsZero() {
			note.Date, note.HasDate = mtime, true
		}
	}
	if note.Title == "" {
		note.Title = titleFromName(base)
	}
	if strings.TrimSpace(note.Title) == "" && strings.TrimSpace(note.Body) == "" {
		return MarkdownNote{}, fmt.Errorf("markdown: %s has neither a title nor a body", filename)
	}
	return note, nil
}

// splitFrontMatter returns the front matter block and the rest of the file, or
// nil and the whole file when there is none.
func splitFrontMatter(body []byte) ([]byte, []byte) {
	trimmed := bytes.TrimLeft(body, "\xef\xbb\xbf \t\r\n")
	if !bytes.HasPrefix(trimmed, []byte(markdownFence)) {
		return nil, body
	}
	rest := trimmed[len(markdownFence):]
	if i := bytes.IndexByte(rest, '\n'); i >= 0 {
		if strings.TrimSpace(string(rest[:i])) != "" {
			return nil, body
		}
		rest = rest[i+1:]
	} else {
		return nil, body
	}
	scanner := bufio.NewScanner(bytes.NewReader(rest))
	scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)
	var front bytes.Buffer
	consumed := 0
	for scanner.Scan() {
		line := scanner.Text()
		consumed += len(line) + 1
		if strings.TrimSpace(line) == markdownFence {
			if consumed > len(rest) {
				consumed = len(rest)
			}
			return front.Bytes(), rest[consumed:]
		}
		front.WriteString(line)
		front.WriteString("\n")
	}
	// An opening fence with no closing one is not front matter.
	return nil, body
}

func (n *MarkdownNote) readFrontMatter(front []byte) error {
	scanner := bufio.NewScanner(bytes.NewReader(front))
	scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)
	key := ""
	for scanner.Scan() {
		raw := scanner.Text()
		line := strings.TrimRight(raw, " \t")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if item, ok := listItem(line); ok {
			n.appendListValue(key, item)
			continue
		}
		name, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(name))
		n.setField(key, unquote(strings.TrimSpace(value)))
	}
	return scanner.Err()
}

func listItem(line string) (string, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(trimmed, "- ") && trimmed != "-" {
		return "", false
	}
	if len(line)-len(trimmed) == 0 && !strings.HasPrefix(line, "- ") {
		return "", false
	}
	return unquote(strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))), true
}

func (n *MarkdownNote) setField(key, value string) {
	switch key {
	case "title", "summary":
		n.Title = value
	case "date", "created":
		if value != "" {
			n.setDate(value)
		}
	case "uid":
		n.UID = value
	case "tags", "categories", "keywords":
		for _, tag := range splitList(value) {
			n.Tags = append(n.Tags, tag)
		}
	case "related", "related-to":
		for _, uid := range splitList(value) {
			n.Related = append(n.Related, Relation{UID: uid, RelType: RelTypeParent})
		}
	case "attachments", "attach":
		n.Attachments = append(n.Attachments, splitList(value)...)
	case "account":
		n.Source.Account = value
	case "collection":
		n.Source.Collection = value
	case "carrel_properties":
		if value != "" {
			n.appendExtra(value)
		}
	}
}

func (n *MarkdownNote) appendListValue(key, item string) {
	if item == "" {
		return
	}
	switch key {
	case "tags", "categories", "keywords":
		n.Tags = append(n.Tags, item)
	case "related", "related-to":
		n.Related = append(n.Related, Relation{UID: item, RelType: RelTypeParent})
	case "attachments", "attach":
		n.Attachments = append(n.Attachments, item)
	case "carrel_properties":
		n.appendExtra(item)
	}
}

func (n *MarkdownNote) appendExtra(item string) {
	name, value, found := strings.Cut(item, ":")
	if !found {
		return
	}
	canonical, err := canonicalName(name)
	if err != nil || protectedProperties[canonical] {
		return
	}
	for i := range n.Extra {
		if n.Extra[i].Name == canonical {
			n.Extra[i].Values = append(n.Extra[i].Values, Text(strings.TrimSpace(value)))
			return
		}
	}
	n.Extra = append(n.Extra, Property{Name: canonical, Values: []Value{Text(strings.TrimSpace(value))}})
}

var markdownDateLayouts = []string{
	time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05",
	"2006-01-02 15:04", "2006-01-02", "02.01.2006", "2006/01/02",
}

func (n *MarkdownNote) setDate(value string) {
	value = strings.TrimSpace(value)
	for _, layout := range markdownDateLayouts {
		t, err := time.Parse(layout, value)
		if err != nil {
			continue
		}
		n.Date, n.HasDate = t, true
		n.DateOnly = layout == "2006-01-02" || layout == "02.01.2006" || layout == "2006/01/02"
		return
	}
}

func splitList(value string) []string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "[")
	value = strings.TrimSuffix(value, "]")
	var out []string
	for _, part := range strings.Split(value, ",") {
		if item := unquote(strings.TrimSpace(part)); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func unquote(value string) string {
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			inner := value[1 : len(value)-1]
			if value[0] == '"' {
				inner = strings.ReplaceAll(inner, `\"`, `"`)
				inner = strings.ReplaceAll(inner, `\\`, `\`)
			}
			return inner
		}
	}
	return value
}

func titleFromBody(body string) (string, string) {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			title := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			rest := strings.TrimSpace(strings.Join(lines[i+1:], "\n"))
			return title, rest
		}
		return "", body
	}
	return "", body
}

func titleFromName(base string) string {
	stem := strings.TrimSuffix(base, ".md")
	stem = strings.TrimSuffix(stem, ".markdown")
	stem = strings.TrimSuffix(stem, ".txt")
	if date, ok := dateFromName(base); ok {
		stem = strings.TrimPrefix(stem, date.Format("2006-01-02"))
		stem = strings.TrimLeft(stem, " -_")
	}
	stem = strings.NewReplacer("_", " ", "-", " ").Replace(stem)
	return strings.TrimSpace(stem)
}

func dateFromName(base string) (time.Time, bool) {
	if len(base) >= 10 {
		if t, err := time.Parse("2006-01-02", base[:10]); err == nil {
			return t, true
		}
	}
	if len(base) >= 8 {
		if t, err := time.Parse("20060102", base[:8]); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func baseName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	return name
}

// IsMarkdownName reports whether a file in an import archive is one to read.
func IsMarkdownName(name string) bool {
	lower := strings.ToLower(baseName(name))
	if strings.HasPrefix(lower, ".") {
		return false
	}
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown") ||
		strings.HasSuffix(lower, ".txt")
}

// Patch turns a parsed file into the patch that fills a fresh VJOURNAL. Import
// only ever creates, so there is no existing object whose properties this could
// tread on (§23.9).
func (n MarkdownNote) Patch(loc *time.Location) *Patch {
	if loc == nil {
		loc = time.Local
	}
	p := &Patch{}
	if title := strings.TrimSpace(n.Title); title != "" {
		p.SetText(ical.PropSummary, title)
	}
	if body := strings.TrimSpace(n.Body); body != "" {
		p.SetText(ical.PropDescription, body)
	}
	if len(n.Tags) > 0 {
		p.SetText(ical.PropCategories, strings.Join(dedupeStrings(n.Tags), ","))
	}
	date := n.Date
	if !n.HasDate || date.IsZero() {
		date = time.Now()
	}
	if n.DateOnly || !n.HasDate {
		p.Set(ical.PropDateTimeStart, Value{
			Text:   date.In(loc).Format("20060102"),
			Params: map[string][]string{"VALUE": {"DATE"}},
		})
	} else {
		p.Set(ical.PropDateTimeStart, Value{
			Text:   date.In(loc).Format("20060102T150405"),
			Params: map[string][]string{"TZID": {loc.String()}},
		})
	}
	if values := RelationValues(n.Related); len(values) > 0 {
		p.Set(ical.PropRelatedTo, values...)
	}
	if values := AttachmentLinkValues(n.Attachments); len(values) > 0 {
		p.Set(ical.PropAttach, values...)
	}
	for _, prop := range n.Extra {
		if len(prop.Values) > 0 {
			p.Set(prop.Name, prop.Values...)
		}
	}
	return p
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = strings.TrimSpace(s); s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// SortedTags returns the tags in a stable order, for a report.
func (n MarkdownNote) SortedTags() []string {
	out := dedupeStrings(n.Tags)
	sort.Strings(out)
	return out
}

func writeYAMLString(w io.Writer, key, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	fmt.Fprintf(w, "%s: %s\n", key, yamlScalar(value))
}

func writeYAMLList(w io.Writer, key string, values []string) {
	items := dedupeStrings(values)
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(w, "%s:\n", key)
	for _, item := range items {
		fmt.Fprintf(w, "  - %s\n", yamlScalar(item))
	}
}

func writeYAMLInline(w io.Writer, value string) {
	fmt.Fprint(w, yamlScalar(value))
}

// yamlScalar quotes a value whenever leaving it bare could change its meaning
// to a reader: a leading indicator character, a colon, a quote, a newline, or
// surrounding space.
func yamlScalar(value string) string {
	value = strings.ReplaceAll(value, "\r\n", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	needsQuote := value == "" || value != strings.TrimSpace(value) ||
		strings.ContainsAny(value, ":#\"'{}[]&*!|>%@`,") ||
		strings.HasPrefix(value, "-") || strings.HasPrefix(value, "?")
	if !needsQuote {
		return value
	}
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}
