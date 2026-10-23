package web

import (
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
	// A file it does not take is refused on the overlay in one phrase
	// (Typecase's note 111 p. 4; s-io-1): «Only ‹.vcf› files are imported
	// here»; several files where one is taken — «One file at a time: only
	// notes take several».
	d.Refused = join("Only ", formatsRuns(accept, " or "), " files are imported here")
	if !d.Many {
		d.OneOnly = oneFileOnly
	}
	// A file another kind takes says where it goes (Typecase's note 113
	// p. 1): «Contacts take ‹.vcf› — the calendar and the list of tasks
	// take this one», «Contacts take this one».
	d.Elsewhere = []view.ImportElsewhere{}
	for _, e := range []string{".vcf", ".ics", ".md", ".zip"} {
		if containsText(accept, e) {
			continue
		}
		says := view.Runs{plainRun(upperFirst(dropHolders[e]) + " this one")}
		if k == view.KindContact {
			says = join("Contacts take ", monoRun(".vcf"), " — "+dropHolders[e]+" this one")
		}
		d.Elsewhere = append(d.Elsewhere, view.ImportElsewhere{Ext: text(e), Says: says})
	}
	return d
}

// dropHolders are what takes a file of each extension, as the overlay says
// it of a file dropped on another kind (s-io-1).
var dropHolders = map[string]string{
	".vcf": "contacts take",
	".ics": "the calendar and the list of tasks take",
	".md":  "a notebook takes",
	".zip": "a notebook takes",
}

// oneFileOnly is the refusal of several files where one is taken, on the
// overlay and of the first step's form (Typecase's note 111 p. 4).
const oneFileOnly = "One file at a time: only notes take several"

// formatsRuns writes extensions in mono, as the overlay does: «‹.md›, ‹.zip›
// and ‹.ics›»; last joins the last two.
func formatsRuns(exts []view.Text, last string) view.Runs {
	var out []any
	for i, e := range exts {
		switch {
		case i == 0:
		case i == len(exts)-1:
			out = append(out, last)
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
