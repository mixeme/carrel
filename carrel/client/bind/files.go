package bind

import (
	"strconv"

	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The files section (U-6.5, C-6 FilesRootPage … PublishedLinksPage): the
// rail's shortcuts, its foot's forms, the root's note.

// Shortcuts are the rail's shortcuts (Rail.Shortcuts, s-files-1, -6) as a
// tree of links: each place's icon, its name, where it goes, and to its
// right what is said of it — where the attachment folder lies («not set»
// without one, s-files-18), how many links are out; the one open now on.
func Shortcuts(cuts []view.Shortcut) w.Tree {
	t := w.Tree{Variant: w.TreePadded}
	for i, s := range cuts {
		row := w.TreeRow{Icon: Icon(s.Icon), Label: string(s.Label), Href: s.Href}
		switch {
		case s.Place != "":
			row.Trail = w.TreeNote{Text: string(s.Place)}
		case s.Count != nil:
			row.Trail = w.TreeNote{Text: strconv.Itoa(*s.Count)}
		}
		t.Rows = append(t.Rows, row)
		if s.On {
			t.On = i + 1
		}
	}
	return t
}

// FootForms are the forms of the rail's foot actions that post (New folder,
// which asks the folder for its name, C-5.5 mkcol): each a PostForm under its
// action's ID, which its button in the foot submits (Foot).
func FootForms(r view.Rail) []w.PostForm {
	var out []w.PostForm
	for _, a := range r.Foot {
		if a.Method == "POST" && a.Enabled {
			out = append(out, ActionForm(a, a.ID))
		}
	}
	return out
}

// MsgParts are a sentence of the model in its pieces (Inlines) as the parts
// of a message: the root's note, a storage not ticked by name in bold
// (s-files-6); a break (RunBreak) starts a line of a list in it (s-backup-2).
func MsgParts(r view.Runs) []w.MsgPart {
	var out []w.MsgPart
	for _, run := range r {
		if run.Mark == view.RunBreak {
			out = append(out, w.Break{})
			continue
		}
		for _, in := range Inlines(view.Runs{run}) {
			if p, ok := in.(w.MsgPart); ok {
				out = append(out, p)
			}
		}
	}
	return out
}

// TreeWords are what the tree of folders takes from the screen: its icon of
// a folder and of a file, the badge of a node that cannot be chosen
// (read-only), of the attachment folder and of the one just made, and what
// joins a file's size and where it lands.
type TreeWords struct {
	Folder      w.Icon
	File        w.Icon
	ReadOnly    string
	Attachments string
	Fresh       string
	Sep         string
}

// FileTree is the tree a sheet chooses a folder or a file in (FileTree, C-6;
// s-files-4, s-notes-11, s-files-16) as the rows of a tree, depth first: each
// node its icon, its name, its depth as its indent (one level, two, three
// and more), a link to the same sheet
// with it chosen (FileTreeNode.Href) — a node that cannot be chosen is
// unavailable with the screen's badge —; to its right the storage's
// connection at a root (Stamp), a file's size and where it lands, the
// badge of the attachment folder or of the one just made. The node chosen
// (Tree.Chosen) is on.
func FileTree(t view.FileTree, words TreeWords) w.Tree {
	out := w.Tree{}
	var walk func(nodes []view.FileTreeNode, depth int)
	walk = func(nodes []view.FileTreeNode, depth int) {
		for _, n := range nodes {
			row := w.TreeRow{Icon: words.Folder, Label: n.Label, Href: n.Href}
			switch depth {
			case 0:
			case 1:
				row.Indent = w.TreeLevel22
			case 2:
				row.Indent = w.TreeLevel40
			default:
				row.Indent = w.TreeLevel58
			}
			if n.File {
				row.Icon = words.File
			}
			switch {
			case n.ReadOnly:
				row.Unavailable, row.Href = true, ""
				row.Trail = w.Badge{Label: words.ReadOnly}
			case n.Fresh:
				row.Trail = w.Badge{Label: words.Fresh}
			case n.Attachments:
				row.Trail = w.Badge{Label: words.Attachments}
			case n.File && n.Lands != "":
				row.Trail = w.Dim{Parts: []w.Inline{w.Text(string(n.Size) + words.Sep + string(n.Lands))}, Size: w.Dim11}
			case n.File && n.Size != "":
				row.Trail = w.Dim{Parts: []w.Inline{w.Text(string(n.Size))}, Size: w.Dim11}
			case depth == 0 && n.Stamp != "":
				row.Trail = w.Dim{Parts: []w.Inline{w.Text(string(n.Stamp))}, Size: w.Dim11}
			}
			out.Rows = append(out.Rows, row)
			if n.ID != "" && n.ID == t.Chosen {
				out.On = len(out.Rows)
			}
			walk(n.Children, depth+1)
		}
	}
	walk(t.Nodes, 0)
	return out
}

// QueueWords are what the queue takes from the screen: the words of a row
// done and of a name taken (C-8 OpState, TransferRow.Question), what joins
// a row's size and its state, the percent's sign.
type QueueWords struct {
	Done    string
	Taken   string
	Sep     string
	Percent string
	// Send — the button beside an answer that brings its file again
	// (Keep both, Replace of an upload, S-17.3), shown only without a
	// script: the file field sends nothing by itself
	Send string
	// Fold — what the queue says folded and unfolded with the library's
	// script (Queue.Fold, s-files-2): Hide, Show, and the total of a folded
	// one
	Fold *w.QueueFold
}

// Queue is the queue of transfers (TransferQueue, C-6.7; s-files-2) as the
// queue: its head — how far it went (Title) and the size of it all
// monospaced (Total) —, Hide or Show (Toggle), folded (Hidden), and a row for each
// transfer: its name, its size and its state — done, how far it went, the
// name taken —, its meter, the question of a name taken in red with its
// answers (Keep both, Replace, Skip) and the row's actions, each a button
// that submits its form (QueueForms).
func Queue(q view.TransferQueue, words QueueWords) w.Queue {
	out := w.Queue{
		Title: string(q.Title),
		Total: w.Mono{Text: string(q.Total), Variant: w.MonoDusty, Place: w.Pushed},
		Min:   q.Hidden,
		Fold:  words.Fold,
	}
	// Hide or Show: the same folder with the queue folded or unfolded, the
	// server's answer without a script (Queue.Toggle); a queue that is an
	// upload's answer has none.
	if q.Toggle != nil {
		out.Toggle = Button(*q.Toggle, "")
		out.Toggle.Small = true
	}
	for _, r := range q.Rows {
		it := w.QueueItem{Name: string(r.Name)}
		switch {
		case r.Question != nil && r.Question.Form != nil:
			// An upload's one form of the answer (s-files-2, Typecase's
			// note 100): Keep both and Replace pick the file in a field of
			// their own, one Send without a script, Skip sends it with
			// answer=skip.
			it.Meta, it.Failed = words.Taken, true
			id := TakenID(r.Question.Form.Action)
			sent := false
			for _, a := range r.Question.Options {
				b := FileButton(a, id)
				if a.File == "" && a.Method == "POST" && a.Enabled {
					if !sent {
						it.Acts = append(it.Acts, w.Button{Label: words.Send, Form: id, Small: true, NoScript: true})
						sent = true
					}
					b = w.Button{Label: string(a.Label), Submit: true, Form: id, To: a.Href}
				}
				b.Small, b.Kind = true, w.ButtonPlain
				it.Acts = append(it.Acts, b)
			}
		case r.Question != nil:
			it.Meta, it.Failed = words.Taken, true
			for _, a := range r.Question.Options {
				it.Acts = append(it.Acts, queueButton(a))
				// An upload kept nothing of its file: Keep both and Replace
				// pick it again (S-17.3), and without a script Send sends it.
				if send := Upload(a, FormID(a), words.Send); send != nil {
					send.Small = true
					it.Acts = append(it.Acts, *send)
				}
			}
		case r.State == view.OpDone:
			it.Meta = joined(string(r.Size), words.Done, words.Sep)
			it.Meter = &w.Meter{Percent: 100, Track: 100}
		case r.State == view.OpFailed:
			// Why it did not go, where the server says (s-files-24).
			it.Meta, it.Failed = string(r.Size), true
			if r.Reason != "" {
				it.Meta = string(r.Reason)
			}
		default:
			it.Meta = joined(string(r.Size), strconv.Itoa(r.Percent)+words.Percent, words.Sep)
			it.Meter = &w.Meter{Percent: r.Percent, Track: 100}
		}
		for _, a := range r.Actions {
			it.Acts = append(it.Acts, queueButton(a))
		}
		out.Items = append(out.Items, it)
	}
	return out
}

// QueueForms are the forms of the queue's buttons that post — the answers to
// a name taken, a row's actions —, one each by its action's ID and address.
func QueueForms(q view.TransferQueue) []w.PostForm {
	var out []w.PostForm
	add := func(a view.Action) {
		if a.Method == "POST" && a.Enabled {
			out = append(out, ActionForm(a, FormID(a)))
		}
	}
	for _, r := range q.Rows {
		switch f := r.Question; {
		case f != nil && f.Form != nil:
			// One form of the answer, multipart without a file field of its
			// own: its file fields stand in the row (TakenID; Typecase's
			// note 106).
			out = append(out, w.PostForm{ID: TakenID(f.Form.Action), Action: f.Form.Action, Fields: Hidden(*f.Form), Multipart: true})
		case f != nil:
			for _, a := range f.Options {
				add(a)
			}
		}
		for _, a := range r.Actions {
			add(a)
		}
	}
	return out
}

// queueButton is an answer or an action of a queue's row: a small button
// that submits its form (QueueForms), or goes; one that brings a file
// (Action.File: Keep both, Replace of an upload) picks it.
func queueButton(a view.Action) w.Button {
	b := FileButton(a, FormID(a))
	b.Small, b.Kind = true, w.ButtonPlain
	return b
}

// joined is two words with sep between them, either alone when the other is
// empty.
func joined(a, b, sep string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + sep + b
}

// TakenID is the ID of the one form of an upload's answer: its address,
// one per row.
func TakenID(action string) string {
	return action
}

// FormID is the ID of the PostForm of an action that posts with no form of
// its own where actions of one ID stand side by side (each row's Rename,
// each answer of the queue): its ID and its address, one per address.
func FormID(a view.Action) string {
	return a.ID + a.Href
}

// Crumbs are the way to a folder (FilesPage.Crumbs) as its crumbs: each
// folder above a link to it, the last — the folder open — its name only.
func Crumbs(cs []view.Crumb) *w.Crumbs {
	if len(cs) == 0 {
		return nil
	}
	out := &w.Crumbs{Current: string(cs[len(cs)-1].Label)}
	for _, c := range cs[:len(cs)-1] {
		out.Path = append(out.Path, w.Crumb{Label: string(c.Label), Href: c.Href})
	}
	return out
}

// MenuItemsAt are the items of a menu as MenuItems makes them, an item that
// posts submitting the form id names for its action (MenuFormsAt): a menu of
// each row of a list, whose items share their IDs from row to row.
func MenuItemsAt(items []view.MenuItem, ro string, id func(view.Action) string) []w.MenuPart {
	out := MenuItems(items, ro)
	i := 0
	for j, p := range out {
		m, ok := p.(w.MenuItem)
		if !ok {
			continue
		}
		it := items[i]
		i++
		switch {
		case m.Form != "":
			m.Form = id(it.Action)
		case m.File != nil && m.File.Form != "":
			m.File.Form = id(it.Action)
		}
		out[j] = m
	}
	return out
}

// MenuFormsAt are the forms of a menu's items that post (MenuItemsAt), each
// under the ID id names for its action.
func MenuFormsAt(items []view.MenuItem, id func(view.Action) string) []w.PostForm {
	var out []w.PostForm
	for _, it := range items {
		if it.Method == "POST" && it.Enabled && !it.Submits {
			out = append(out, ActionForm(it.Action, id(it.Action)))
		}
	}
	return out
}

// ServerDrop is the choice of a connection in a sheet's head (s-files-26,
// Typecase's note 83): the open one shown in the pick, its menu a link to
// the same sheet over each server — the open one on —, swapped in by the
// live layer as the sheet swap; one that did not answer is off with why.
func ServerDrop(servers []view.FileServer, label, swap string) w.Drop {
	d := w.Drop{Trigger: w.Pick{Label: label, Variant: w.PickT12W240}, Place: w.Pushed,
		Menu: w.Menu{Width: w.MenuWidth290, Pop: w.MenuUnderEnd}}
	for _, sv := range servers {
		item := w.MenuItem{Label: string(sv.Label), On: sv.On, Href: sv.Href, Swap: swap}
		if sv.Href == "" {
			item.Href, item.Swap, item.Reason = "#", "", string(sv.Reason)
		}
		if sv.On {
			d.Trigger = w.Pick{Label: label, Value: string(sv.Label), Variant: w.PickT12W240}
		}
		d.Menu.Items = append(d.Menu.Items, item)
	}
	return d
}
