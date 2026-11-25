package bind_test

import (
	"testing"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The files' rail (s-files-1, -6): the root is a row with an icon and the
// server's muted words; a storage says rw where it can be written and only
// ro where not; the shortcuts are a tree of links with what is said of each
// place; New folder of the foot posts its form.
func TestFilesRail(t *testing.T) {
	words := bind.RailWords{Form: "rail", Toggle: "toggle", None: "—", ReadOnly: "ro", Root: "file", Writable: "rw"}
	rail := view.Rail{
		Sections: []view.RailSection{
			{Rows: []view.SourceRow{{Href: "/app/files", Label: "Selected collections", Tick: view.TickPart, On: true, Meta: "2 of 3"}}},
			{Account: "sftpgo · vps", Rows: []view.SourceRow{
				{Href: "/d", Label: "Documents", Colour: "#a05a2c", Tick: view.TickOn},
				{Href: "/p", Label: "Public", Colour: "#a05a2c", Tick: view.TickOff, ReadOnly: true},
			}},
		},
		Ticks: &view.Form{Action: "/app/sources/files"},
	}
	same(t, "the files' rail", bind.Sources(rail, words), []w.RailPart{
		w.RailSec{Items: []w.RailSecPart{w.TickRow{Ticked: w.TickedPart, Kind: w.TickRowRoot, Icon: "file", Label: "Selected collections", Meta: "2 of 3", Current: true,
			Send: w.Send{Form: "rail", Name: "toggle", Value: "/app/files"}, Href: "/app/files"}}},
		w.RailSec{Items: []w.RailSecPart{
			w.Rubric{Text: "sftpgo · vps"},
			w.TickRow{Mark: w.Mark{Colour: "#a05a2c"}, Label: "Documents", Meta: "rw", Send: w.Send{Form: "rail", Name: "toggle", Value: "/d"}, Href: "/d"},
			w.TickRow{Ticked: w.TickedOff, Mark: w.Mark{Colour: "#a05a2c"}, Label: "Public", Hidden: true, Trail: w.Badge{Label: "ro"}, Send: w.Send{Form: "rail", Name: "toggle", Value: "/p"}, Href: "/p"},
		}},
	})

	cuts := []view.Shortcut{
		{Action: view.Action{ID: "files.attachments", Label: "Attachments", Icon: "i-file", Href: "/a", Method: "GET", Enabled: true, On: true}, Place: "Documents / carrel"},
		{Action: view.Action{ID: "files.published", Label: "Published", Icon: "i-link", Href: "/app/files/published", Method: "GET", Enabled: true}, Count: ptr(2)},
		{Action: view.Action{ID: "files.attachments", Label: "Attachments", Icon: "i-file", Href: "/s", Method: "GET", Enabled: true}, Place: "not set"},
	}
	same(t, "the shortcuts", bind.Shortcuts(cuts), w.Tree{Rows: []w.TreeRow{
		{Icon: "file", Label: "Attachments", Href: "/a", Trail: w.TreeNote{Text: "Documents / carrel"}},
		{Icon: "link", Label: "Published", Href: "/app/files/published", Trail: w.TreeNote{Text: "2"}},
		{Icon: "file", Label: "Attachments", Href: "/s", Trail: w.TreeNote{Text: "not set"}},
	}, On: 1, Variant: w.TreePadded})

	foot := view.Rail{Foot: []view.Action{
		{ID: "rail.folder.new", Label: "New folder", Icon: "i-plus", Href: "/mkcol/", Method: "POST", Enabled: true, CSRF: "c"},
		{ID: "account.connect", Label: "Connect account", Icon: "i-server", Href: "/connect", Method: "GET", Enabled: true},
		{ID: "rail.folder.off", Label: "New folder", Icon: "i-plus", Method: "POST", Reason: "Open a folder to make one in it."},
	}}
	same(t, "the foot", bind.Foot(foot, words).Items, []w.RailFootPart{
		w.Button{Icon: "plus", Title: "New folder", Form: "rail.folder.new"},
		w.Button{Icon: "server", Title: "Connect account", Href: "/connect"},
		w.Button{Icon: "plus", Title: "New folder", State: w.ButtonDisabled, Reason: "Open a folder to make one in it."},
	})
	same(t, "its forms", bind.FootForms(foot), []w.PostForm{{ID: "rail.folder.new", Action: "/mkcol/", Fields: []w.Hidden{{Name: "_csrf", Value: "c"}}}})
}

// The tree a sheet chooses in (s-files-4, s-notes-11, s-files-16): depth
// first, the indent by depth (three levels at most), a root's connection, a
// node read-only unavailable with its badge, the attachment folder and the
// one just made badged, a file's size and where it lands, the one chosen on.
func TestFileTree(t *testing.T) {
	tree := view.FileTree{Chosen: "/d/c/a/", Nodes: []view.FileTreeNode{
		{ID: "/d/", Label: "Documents", Stamp: "sftpgo · vps", Href: "?to=d", Expanded: true, Children: []view.FileTreeNode{
			{ID: "/d/c/", Label: "carrel", Href: "?to=c", Expanded: true, Children: []view.FileTreeNode{
				{ID: "/d/c/a/", Label: "attachments", Href: "?to=a", Attachments: true, Children: []view.FileTreeNode{
					{ID: "/d/c/a/x.md", Label: "x.md", File: true, Size: "3 KB", Lands: "inside", Href: "?to=x"},
				}},
				{ID: "/d/c/n/", Label: "2026-08", Href: "?to=n", Fresh: true},
			}},
		}},
		{ID: "/p/", Label: "Public", Stamp: "sftpgo · vps", ReadOnly: true, Href: "?to=p"},
	}}
	words := bind.TreeWords{Folder: "file", File: "doc", ReadOnly: "read-only", Attachments: "attachment folder", Fresh: "just created", Sep: " · "}
	same(t, "the tree", bind.FileTree(tree, words), w.Tree{Rows: []w.TreeRow{
		{Icon: "file", Label: "Documents", Href: "?to=d", Trail: w.Dim{Parts: []w.Inline{w.Text("sftpgo · vps")}, Size: w.Dim11}},
		{Icon: "file", Label: "carrel", Href: "?to=c", Indent: w.TreeLevel22},
		{Icon: "file", Label: "attachments", Href: "?to=a", Indent: w.TreeLevel40, Trail: w.Badge{Label: "attachment folder"}},
		{Icon: "doc", Label: "x.md", Href: "?to=x", Indent: w.TreeLevel58, Trail: w.Dim{Parts: []w.Inline{w.Text("3 KB · inside")}, Size: w.Dim11}},
		{Icon: "file", Label: "2026-08", Href: "?to=n", Indent: w.TreeLevel40, Trail: w.Badge{Label: "just created"}},
		{Icon: "file", Label: "Public", Unavailable: true, Trail: w.Badge{Label: "read-only"}},
	}, On: 3})
}

// The queue (s-files-2): its head, a row done, a row on its way, a name
// taken in red with its answers, each a button of its own form.
func TestQueue(t *testing.T) {
	both := view.Action{ID: "transfer.both", Label: "Keep both", Href: "/u?answer=both", Method: "POST", Enabled: true, CSRF: "c"}
	skip := view.Action{ID: "transfer.skip", Label: "Skip", Href: "/u?answer=skip", Method: "POST", Enabled: true, CSRF: "c"}
	q := view.TransferQueue{Title: "Uploading 2 of 3", Total: "12.4 MB", Rows: []view.TransferRow{
		{Name: "отчёт.pdf", Size: "4.1 MB", Percent: 100, State: view.OpDone},
		{Name: "плакат.png", Size: "8.3 MB", Percent: 62, State: view.OpRunning},
		{Name: "list.md", Size: "7 B", State: view.OpIdle, Question: &view.NameClash{Name: "list.md", Options: []view.Action{both, skip}}},
	}}
	words := bind.QueueWords{Done: "done", Taken: "name taken", Sep: " · ", Percent: "%"}
	same(t, "the queue", bind.Queue(q, words), w.Queue{
		Title: "Uploading 2 of 3",
		Total: w.Mono{Text: "12.4 MB", Variant: w.MonoDusty, Place: w.Pushed},
		Items: []w.QueueItem{
			{Name: "отчёт.pdf", Meta: "4.1 MB · done", Meter: &w.Meter{Percent: 100, Track: 100}},
			{Name: "плакат.png", Meta: "8.3 MB · 62%", Meter: &w.Meter{Percent: 62, Track: 100}},
			{Name: "list.md", Meta: "name taken", Failed: true, Acts: []w.Button{
				{Label: "Keep both", Small: true, Form: "transfer.both/u?answer=both"},
				{Label: "Skip", Small: true, Form: "transfer.skip/u?answer=skip"},
			}},
		},
	})
	same(t, "its forms", bind.QueueForms(q), []w.PostForm{
		{ID: "transfer.both/u?answer=both", Action: "/u?answer=both", Fields: []w.Hidden{{Name: "_csrf", Value: "c"}}},
		{ID: "transfer.skip/u?answer=skip", Action: "/u?answer=skip", Fields: []w.Hidden{{Name: "_csrf", Value: "c"}}},
	})
}

// The way into a folder: each folder above a link, the one open its name.
func TestCrumbs(t *testing.T) {
	same(t, "the crumbs", bind.Crumbs([]view.Crumb{{Label: "Documents", Href: "/d/"}, {Label: "carrel", Href: "/d/c/"}, {Label: "attachments"}}),
		&w.Crumbs{Path: []w.Crumb{{Label: "Documents", Href: "/d/"}, {Label: "carrel", Href: "/d/c/"}}, Current: "attachments"})
	same(t, "a storage's root", bind.Crumbs([]view.Crumb{{Label: "Documents"}}), &w.Crumbs{Current: "Documents"})
	same(t, "none", bind.Crumbs(nil), (*w.Crumbs)(nil))
}

// A menu of each row of a list: the items that post submit the form of
// their action's ID and address, one per row.
func TestMenuItemsAt(t *testing.T) {
	items := []view.MenuItem{
		{Action: view.Action{ID: "file.open", Label: "Open", Icon: "i-doc", Href: "/f", Method: "GET", Enabled: true}},
		{Action: view.Action{ID: "file.delete", Label: "Delete", Icon: "i-trash", Href: "/op?item=f&op=d", Method: "POST", Enabled: true, Danger: true, CSRF: "c"}, Separator: true},
	}
	same(t, "the items", bind.MenuItemsAt(items, "", bind.FormID), []w.MenuPart{
		w.MenuItem{Icon: "doc", Label: "Open", Href: "/f"},
		w.MenuSep{},
		w.MenuItem{Icon: "trash", Label: "Delete", Form: "file.delete/op?item=f&op=d", Tone: w.MenuToneDanger},
	})
	same(t, "their forms", bind.MenuFormsAt(items, bind.FormID), []w.PostForm{{ID: "file.delete/op?item=f&op=d", Action: "/op?item=f&op=d", Fields: []w.Hidden{{Name: "_csrf", Value: "c"}}}})
}

// The view of a folder (Typecase's task 115): an option of a switch of
// links keeps its icon.
func TestSegOptionIcon(t *testing.T) {
	same(t, "an option with its icon", bind.SegOption(view.Action{ID: "files.view.tiles", Label: "Tiles", Icon: "i-grid", Href: "?view=tiles", Method: "GET", Enabled: true}),
		w.SegOption{Label: "Tiles", Icon: "grid", Href: "?view=tiles"})
}

// The root's note (s-files-6): its pieces as a message's, the storage not
// ticked in bold.
func TestMsgParts(t *testing.T) {
	same(t, "the note", bind.MsgParts(view.Runs{{Text: "This is the root. "}, {Text: "Public", Mark: view.RunStrong}, {Text: " is not ticked."}}),
		[]w.MsgPart{w.Text("This is the root. "), w.Strong("Public"), w.Text(" is not ticked.")})
}
