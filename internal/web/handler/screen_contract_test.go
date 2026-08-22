// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/web"
)

// interface-rebuild.md §4. A template holds more than a look. Field names,
// hx-* and sse-* bindings, the data-* a script hooks, an action and a method,
// the id another node aims at, and the fields of the view the template reads —
// the empty-state branch among them. Half of that is written down nowhere, and
// a screen rewritten from its frame loses it silently: nothing looks wrong,
// and the loss surfaces half a year later on one screen out of fifty-five.
//
// So every name is frozen out of the template as it stands today, before a
// line of the rebuild is written, and the rebuilt screen is required to bring
// each one back. A name may still be dropped — but only out loud, with "- "
// in front of it and a reason after "#", which turns losing the behaviour
// into a decision visible in the diff instead of a side effect of retyping.
//
// This is the one place in the rebuild where the old template is allowed to
// influence the new one.

var updateContracts = flag.Bool("update", false,
	"rewrite the frozen contract sheets in testdata/contract from the templates as they stand")

const contractDir = "testdata/contract"

// templateActions matches one {{…}} span. Attribute scanning runs on a copy
// with every span replaced by "*", so a quote or a ">" inside an action
// cannot be read as the end of an attribute or of a tag.
var (
	templateAction = regexp.MustCompile(`\{\{[^}]*\}\}`)
	controlTag     = regexp.MustCompile(`(?is)<(input|select|textarea|button|form|a|img)\b([^>]*)>`)
	attrPair       = regexp.MustCompile(`([A-Za-z_:][-A-Za-z0-9_:.]*)\s*=\s*"([^"]*)"`)
	dottedPath     = regexp.MustCompile(`[$.][A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*`)
	starRun        = regexp.MustCompile(`\*+`)
)

// contractOf reads every name the rest of the system depends on out of one
// template. It is deliberately blunt: it does not judge whether a name is
// load-bearing, because the names that turned out to be load-bearing are
// exactly the ones nobody thought to write down.
func contractOf(body string) []string {
	seen := map[string]bool{}
	add := func(tok string) {
		if tok != "" {
			seen[tok] = true
		}
	}

	// Pass one: the view fields this template reads. A rewritten screen that
	// stops reading $d.Empty has lost the empty state, and nothing else in
	// this file would notice.
	for _, action := range templateAction.FindAllString(body, -1) {
		for _, path := range dottedPath.FindAllString(action, -1) {
			if p := normalizeReadPath(path); p != "" {
				add("read:" + p)
			}
		}
	}

	// Pass two: attributes, on a copy where actions cannot break the scan.
	flat := templateAction.ReplaceAllString(body, "*")
	for _, tag := range controlTag.FindAllStringSubmatch(flat, -1) {
		name := strings.ToLower(tag[1])
		attrs := map[string]string{}
		for _, a := range attrPair.FindAllStringSubmatch(tag[2], -1) {
			attrs[strings.ToLower(a[1])] = a[2]
		}
		for attr, value := range attrs {
			switch {
			case attr == "name" && name != "a" && name != "img":
				add("field:" + collapse(value))
			case attr == "action", attr == "method", attr == "enctype", attr == "accept":
				add("form:" + attr + "=" + collapse(value))
			case strings.HasPrefix(attr, "hx-"), strings.HasPrefix(attr, "sse-"):
				add("bind:" + attr + "=" + collapse(value))
			case strings.HasPrefix(attr, "data-"):
				add("data:" + attr + "=" + collapse(value))
			case attr == "id":
				if v := collapse(value); !strings.Contains(v, "*") {
					add("id:" + v)
				}
			case attr == "type" && name == "input":
				add("field:type=" + collapse(value))
			}
		}
	}

	// A bare data-* attribute carries no "=" and the pair scan above misses
	// it: `<div data-filter-scope>` is as much a hook as `data-col="name"`.
	for _, tag := range controlTag.FindAllStringSubmatch(flat, -1) {
		addBareData(tag[2], add)
	}
	for _, tag := range regexp.MustCompile(`(?is)<(div|span|section|ul|li|aside|nav|p|dl)\b([^>]*)>`).
		FindAllStringSubmatch(flat, -1) {
		addBareData(tag[2], add)
		for _, a := range attrPair.FindAllStringSubmatch(tag[2], -1) {
			attr := strings.ToLower(a[1])
			switch {
			case strings.HasPrefix(attr, "hx-"), strings.HasPrefix(attr, "sse-"):
				add("bind:" + attr + "=" + collapse(a[2]))
			case strings.HasPrefix(attr, "data-"):
				add("data:" + attr + "=" + collapse(a[2]))
			case attr == "id":
				if v := collapse(a[2]); !strings.Contains(v, "*") {
					add("id:" + v)
				}
			}
		}
	}

	// Pass three: attributes handed to a component through safeAttr, where the
	// quotes are escaped and pass two only ever sees a "*". The list template's
	// own id and its column root arrive that way, and both are hooks.
	for _, m := range escapedAttr.FindAllStringSubmatch(body, -1) {
		attr, value := strings.ToLower(m[1]), collapse(m[2])
		switch {
		case attr == "id":
			if !strings.Contains(value, "*") {
				add("id:" + value)
			}
		case strings.HasPrefix(attr, "hx-"), strings.HasPrefix(attr, "sse-"):
			add("bind:" + attr + "=" + value)
		default:
			add("data:" + attr + "=" + value)
		}
	}

	out := make([]string, 0, len(seen))
	for tok := range seen {
		out = append(out, tok)
	}
	sort.Strings(out)
	return out
}

// escapedAttr is the same attribute written inside a Go string inside a
// template action: id=\"contact-list\" data-columns-root=\"contacts\".
var escapedAttr = regexp.MustCompile(`\b((?:data|hx|sse)-[a-z0-9-]+|id)=\\"([^"\\]*)\\"`)

var bareAttr = regexp.MustCompile(`(?:^|\s)(data-[a-z0-9-]+)(?:\s|$)`)

func addBareData(attrs string, add func(string)) {
	// Strip attr="value" pairs first, so only the valueless ones are left.
	rest := attrPair.ReplaceAllString(attrs, " ")
	for _, m := range bareAttr.FindAllStringSubmatch(rest, -1) {
		add("data:" + m[1])
	}
}

// normalizeReadPath turns "$d.Collection.Color" and "$.Base" into the field
// path the handler has to keep supplying. A bare variable is not a contract:
// $row is this template's own name for something it just built.
func normalizeReadPath(path string) string {
	parts := strings.Split(path, ".")
	if parts[0] == "" || strings.HasPrefix(parts[0], "$") {
		parts = parts[1:]
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ".")
}

// collapse reduces a value with template actions in it to a stable shape:
// "/app/contacts/{{$d.AccountID}}/{{$d.ColEnc}}/new" becomes
// "/app/contacts/*/*/new". The route survives a rename of the variable that
// fills it, which is what the contract is about.
func collapse(v string) string {
	v = strings.TrimSpace(starRun.ReplaceAllString(v, "*"))
	return strings.Join(strings.Fields(v), " ")
}

// screenTemplateNames is every page template, the base frame included: the
// shell holds contracts of its own (the CSRF header htmx sends, the details
// target, the offline bar) and losing one of those breaks every screen at
// once.
func screenTemplateNames(t *testing.T) []string {
	t.Helper()
	templateFS, err := fs.Sub(web.TemplateFS, "template")
	if err != nil {
		t.Fatalf("template FS: %v", err)
	}
	names, err := fs.Glob(templateFS, "*.html")
	if err != nil {
		t.Fatalf("glob templates: %v", err)
	}
	if len(names) < 50 {
		t.Fatalf("only %d templates found; the glob is wrong, not the tree", len(names))
	}
	sort.Strings(names)
	return names
}

func readTemplate(t *testing.T, name string) string {
	t.Helper()
	templateFS, err := fs.Sub(web.TemplateFS, "template")
	if err != nil {
		t.Fatalf("template FS: %v", err)
	}
	b, err := fs.ReadFile(templateFS, name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

// contractSheet is one frozen file: the names still required, and the ones
// dropped on purpose with the reason they were.
type contractSheet struct {
	required []string
	dropped  map[string]string
}

func parseContractSheet(text string) (contractSheet, error) {
	sheet := contractSheet{dropped: map[string]string{}}
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			body := strings.TrimSpace(trimmed[2:])
			tok, reason, ok := strings.Cut(body, "#")
			if !ok || strings.TrimSpace(reason) == "" {
				return sheet, fmt.Errorf("line %d: a dropped name needs its reason after \"#\": %q", i+1, trimmed)
			}
			sheet.dropped[strings.TrimSpace(tok)] = strings.TrimSpace(reason)
			continue
		}
		sheet.required = append(sheet.required, trimmed)
	}
	return sheet, nil
}

const contractHeader = `# Contract of %s — frozen before the rebuild (docs/plans/interface-rebuild.md §4).
#
# Every line is a name something outside the markup depends on: a form field,
# an htmx or SSE binding, a data-* a script hooks, an id another node aims at,
# or a field of the view this template reads. The template rewritten from its
# frame has to bring each one back.
#
# A name may be dropped — but only out loud. Put "- " in front of the line and
# the reason after "#":
#
#     - field:sort_order  # the bar sorts by link now, not by a posted field
#
# Then the lost behaviour is a decision visible in the diff instead of a side
# effect of retyping the screen.
#
# Regenerate with: go test ./internal/web/handler -run TestScreenKeepsItsContract -update
`

func contractPath(name string) string {
	return filepath.Join(contractDir, strings.TrimSuffix(name, ".html")+".txt")
}

func TestScreenKeepsItsContract(t *testing.T) {
	names := screenTemplateNames(t)

	if *updateContracts {
		if err := os.MkdirAll(contractDir, 0o755); err != nil {
			t.Fatalf("make %s: %v", contractDir, err)
		}
		for _, name := range names {
			var b strings.Builder
			fmt.Fprintf(&b, contractHeader, name)
			existing := map[string]string{}
			if old, err := os.ReadFile(contractPath(name)); err == nil {
				sheet, err := parseContractSheet(string(old))
				if err != nil {
					t.Fatalf("%s: %v", contractPath(name), err)
				}
				existing = sheet.dropped
			}
			for _, tok := range contractOf(readTemplate(t, name)) {
				if reason, ok := existing[tok]; ok {
					fmt.Fprintf(&b, "- %s  # %s\n", tok, reason)
					continue
				}
				fmt.Fprintln(&b, tok)
			}
			// A name dropped earlier stays dropped even once the template no
			// longer produces it; the reason is the record.
			var stale []string
			for tok, reason := range existing {
				stale = append(stale, "- "+tok+"  # "+reason)
			}
			sort.Strings(stale)
			for _, line := range stale {
				if !strings.Contains(b.String(), strings.TrimPrefix(line, "- ")) {
					fmt.Fprintln(&b, line)
				}
			}
			if err := os.WriteFile(contractPath(name), []byte(b.String()), 0o644); err != nil {
				t.Fatalf("write %s: %v", contractPath(name), err)
			}
		}
		t.Logf("froze %d contract sheets in %s", len(names), contractDir)
		return
	}

	frozen := map[string]bool{}
	for _, name := range names {
		frozen[name] = true
		path := contractPath(name)
		b, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s has no frozen contract at %s; freeze it before the screen is rewritten "+
				"(go test ./internal/web/handler -run TestScreenKeepsItsContract -update)", name, path)
			continue
		}
		sheet, err := parseContractSheet(string(b))
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		have := map[string]bool{}
		for _, tok := range contractOf(readTemplate(t, name)) {
			have[tok] = true
		}
		for _, tok := range sheet.required {
			if have[tok] {
				continue
			}
			t.Errorf("%s no longer carries %s, which %s froze before the rebuild. "+
				"Put it back, or drop the line explicitly: prefix it with \"- \" and give the "+
				"reason after \"#\" in %s", name, tok, path, path)
		}
	}

	entries, err := os.ReadDir(contractDir)
	if err != nil {
		t.Fatalf("read %s: %v", contractDir, err)
	}
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".txt") + ".html"
		if !frozen[name] {
			t.Errorf("%s holds a contract for %s, which is not a template any more; "+
				"delete the sheet in the commit that deleted the screen", contractDir, name)
		}
	}
}

// The extractor is the whole of §4's protection, so it is held to a worked
// example rather than trusted: a contract nobody proved can read a template
// is a contract that freezes an empty set and passes for ever after.
func TestContractExtractorReadsWhatItPromises(t *testing.T) {
	const sample = `{{define "body"}}
{{$d := .Data}}
<form method="post" action="{{.Base}}/app/notes/{{$d.AccountID}}/{{$d.ColEnc}}/import" enctype="multipart/form-data" class="m-form">
    <input type="hidden" name="csrf_token" value="{{.CSRF}}">
    <input type="file" name="upload" accept=".md,text/markdown">
    <div id="find-panel" data-sse-panel hx-get="{{.Base}}/app/find/{{$d.TaskID}}/results" sse-swap="results"></div>
    {{if $d.Empty}}<p class="m-empty">Nothing here.</p>{{end}}
</form>
{{end}}`

	want := []string{
		"bind:hx-get=*/app/find/*/results",
		"bind:sse-swap=results",
		"data:data-sse-panel",
		"field:csrf_token",
		"field:type=file",
		"field:type=hidden",
		"field:upload",
		"form:accept=.md,text/markdown",
		"form:action=*/app/notes/*/*/import",
		"form:enctype=multipart/form-data",
		"form:method=post",
		"id:find-panel",
		"read:AccountID",
		"read:Base",
		"read:CSRF",
		"read:ColEnc",
		"read:Data",
		"read:Empty",
		"read:TaskID",
	}
	got := contractOf(sample)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("contractOf read\n  %s\nwanted\n  %s",
			strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}

	// And it has to notice a loss, or freezing was pointless. Removing the
	// empty-state branch and the upload field must remove exactly those names.
	stripped := strings.Replace(sample, `{{if $d.Empty}}<p class="m-empty">Nothing here.</p>{{end}}`, "", 1)
	stripped = strings.Replace(stripped, `<input type="file" name="upload" accept=".md,text/markdown">`, "", 1)
	after := map[string]bool{}
	for _, tok := range contractOf(stripped) {
		after[tok] = true
	}
	for _, gone := range []string{"read:Empty", "field:upload", "form:accept=.md,text/markdown", "field:type=file"} {
		if after[gone] {
			t.Errorf("%s survived a template that no longer has it; the extractor is reading something else", gone)
		}
	}
	for _, kept := range []string{"field:csrf_token", "id:find-panel", "bind:sse-swap=results"} {
		if !after[kept] {
			t.Errorf("%s went missing from a template that still has it; the extractor is over-eager", kept)
		}
	}
}
