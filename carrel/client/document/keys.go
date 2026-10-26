package document

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/client/screens"
	"carrel.dev/typecase/widget"
)

// Keys is the sheet of the keys (U-7, account-6): the table of U-7 over the
// screen it was opened from, which stays under it. It has no address — the
// account's item and «?» open it in place with the library's script, and
// its cross, Close and Esc shut it (Typecase's note 134). The keys are the
// client's, not a screen's: the same on every page of the shell.
var Keys = widget.Scrim{
	ID:     bind.KeysSheet,
	Closed: true,
	Content: widget.Sheet{
		Icon:  "search",
		Title: "Keyboard shortcuts",
		Head:  []widget.SheetHeadPart{widget.Close{Label: "Close"}},
		Body: []widget.SheetPart{
			widget.SheetKeys{Items: []widget.SheetKey{
				{Keys: "N", Does: "Quick note over this screen"},
				{Keys: "/", Does: "Focus the search field in the header"},
				{Keys: "Enter", Does: "Open the selected record full screen"},
				{Keys: "Esc", Does: "Close a sheet, the details panel or focus mode"},
				{Keys: "Ctrl+S", Does: "Save in the editor"},
				{Keys: "Ctrl+Enter", Does: "Save a quick note"},
				{Keys: "Ctrl+V", Does: "Paste an image as an attachment"},
			}},
			widget.SheetHint{Text: "Shortcuts do nothing while the cursor is in a field. None of them destroy anything."},
		},
		Foot:  []widget.SheetFootPart{widget.Button{Label: "Close", Closes: true}, widget.Kbd{Keys: "Esc"}, widget.Kbd{Keys: "?", Place: widget.Pushed}},
		Width: widget.SheetWidth480,
	},
}

// withKeys is a page of the shell with the sheet of the keys among its own
// closed sheets (Shell.Dialogs), where its account's menu has the item that
// opens it (bind.TopWords.Keys): the frames draw it in the zones' scheme
// only (shell-1).
func withKeys(root screens.Root) screens.Root {
	shell, ok := root.(widget.Shell)
	if !ok || !opensKeys(shell.Top) {
		return root
	}
	shell.Dialogs = append(append([]widget.Scrim(nil), shell.Dialogs...), Keys)
	return shell
}

// opensKeys says whether a top bar's menus have the item of the keys' sheet.
func opensKeys(top *widget.Top) bool {
	if top == nil {
		return false
	}
	for _, a := range top.Actions {
		drop, ok := a.(widget.TopDrop)
		if !ok {
			continue
		}
		for _, it := range drop.Menu.Items {
			if m, ok := it.(widget.MenuItem); ok && m.Href == "#"+bind.KeysSheet {
				return true
			}
		}
	}
	return false
}
