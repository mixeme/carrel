package web

import (
	"strings"

	"carrel.dev/carrel/server/internal/sealed"

	"carrel.dev/carrel/server/view"
)

// A file dropped on a collection's list opens the wizard on its preview
// (s-io-1, Им3): where it was dropped is where it goes, so the first step
// has nothing to ask. The list says so before the drop — what will happen,
// where to, that nothing is written yet — and refuses a file of another
// kind with where it belongs.

// importDrop is the drop of one collection's list; nil when the collection
// cannot be written, which offers no wizard either.
func (s *Server) importDrop(c *collectionCtx) *view.ImportDrop {
	if c.col.ReadOnly {
		return nil
	}
	return dropOf(c.spec.kind, s.importHref(c, "", "", ""), " into ", strongRun(string(c.origin().stamp())))
}

// combinedDrop is the drop of a combined view (Им4; s-io-1): nothing is
// chosen there, so the file opens the first step with «Into» empty — where
// it goes is asked. nil — no collection of the kind takes records.
func (s *Server) combinedDrop(blob sealed.Blob, spec kindSpec) *view.ImportDrop {
	if _, ok := firstWritable(blob, spec.kind); !ok {
		return nil
	}
	return dropOf(spec.kind, s.route("/app/"+spec.segment+"/import"), " — where it goes is asked next", "")
}

// dropOf is the drop that sends the file to href: what the kind takes, and
// the overlay's line — the files, then where to (into, a run or words).
func dropOf(k view.Kind, href, how string, into any) *view.ImportDrop {
	accept := importFormats(k)
	d := &view.ImportDrop{
		Action: view.Action{ID: "import.drop", Label: "Import", Href: href, Method: "POST", File: "file", Enabled: true},
		Accept: accept,
		Many:   k == view.KindNote,
		Lead:   "Drop to import",
		Note:   "The preview opens first: nothing is written until it has been read",
	}
	tail := []any{how}
	if into != "" {
		tail = append(tail, into)
	}
	if d.Many {
		// Notes take several files at once, as a zip and a folder (s-io-1).
		d.Line = join(append([]any{monoRun(".md"), " files, a ", monoRun(".zip"), " or an ", monoRun(".ics"), " file"}, tail...)...)
	} else {
		d.Line = join(append([]any{"One ", monoRun(string(accept[0])), " file"}, tail...)...)
	}
	for _, other := range []struct {
		ext   string
		holds []view.Kind
	}{
		{".vcf", []view.Kind{view.KindContact}},
		{".ics", []view.Kind{view.KindEvent, view.KindTask, view.KindNote}},
		{".md", []view.Kind{view.KindNote}},
		{".zip", []view.Kind{view.KindNote}},
	} {
		if containsText(accept, other.ext) {
			continue
		}
		var where []string
		for _, k := range other.holds {
			where = append(where, dropHolder[k])
		}
		takes := "takes"
		if k == view.KindContact {
			takes = "take"
		}
		says := join(dropTaker[k]+" "+takes+" ", formatsRuns(accept), " — "+joinAnd(where)+" "+takesFor(where)+" this one")
		d.Elsewhere = append(d.Elsewhere, view.ImportElsewhere{Ext: text(other.ext), Says: says})
	}
	return d
}

// dropTaker names the kind's collection as the refusal begins: «Contacts
// take ‹.vcf› — …» (s-io-1).
var dropTaker = map[view.Kind]string{
	view.KindContact: "Contacts", view.KindEvent: "A calendar", view.KindTask: "A list of tasks", view.KindNote: "A notebook",
}

// dropHolder names where a file of another kind goes: «the calendar and the
// list of tasks take this one» (s-io-1).
var dropHolder = map[view.Kind]string{
	view.KindContact: "contacts", view.KindEvent: "the calendar", view.KindTask: "the list of tasks", view.KindNote: "a notebook",
}

// takesFor agrees the verb with where: one holder takes, two take.
func takesFor(where []string) string {
	if len(where) == 1 && where[0] != "contacts" {
		return "takes"
	}
	return "take"
}

// joinAnd lists words as a sentence does: «the calendar, the list of tasks
// and a notebook».
func joinAnd(words []string) string {
	switch len(words) {
	case 0:
		return ""
	case 1:
		return words[0]
	}
	return strings.Join(words[:len(words)-1], ", ") + " and " + words[len(words)-1]
}

// formatsRuns writes extensions in mono, as the overlay does: «‹.md›, ‹.zip›
// and ‹.ics›».
func formatsRuns(exts []view.Text) view.Runs {
	var out []any
	for i, e := range exts {
		switch {
		case i == 0:
		case i == len(exts)-1:
			out = append(out, " and ")
		default:
			out = append(out, ", ")
		}
		out = append(out, monoRun(string(e)))
	}
	return join(out...)
}

func containsText(list []view.Text, v string) bool {
	for _, t := range list {
		if string(t) == v {
			return true
		}
	}
	return false
}
