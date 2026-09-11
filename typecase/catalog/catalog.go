package catalog

import (
	"fmt"
	"strings"

	"carrel.dev/typecase/widget"
)

// Entry is one widget demonstration in the catalog (L-9.1).
type Entry struct {
	Family string
	Widget string
	State  string
	HTML   string
}

// Entries returns every widget in each declared state for the live catalog.
func Entries() []Entry {
	var out []Entry
	add := func(family, name, state, html string) {
		out = append(out, Entry{Family: family, Widget: name, State: state, HTML: html})
	}

	// Primitives
	add("primitives", "tc-btn", "normal", widget.Button{Label: "Save"}.Render())
	add("primitives", "tc-btn", "primary", widget.Button{Label: "Create", Kind: widget.ButtonPrimary}.Render())
	add("primitives", "tc-btn", "danger", widget.Button{Label: "Delete permanently", Kind: widget.ButtonDanger}.Render())
	add("primitives", "tc-btn", "disabled", widget.Button{Label: "Unavailable", Reason: "Nothing selected"}.Render())
	add("primitives", "tc-in", "text", widget.Input{Name: "name", Kind: widget.InputText, Placeholder: "Enter name"}.Render())
	add("primitives", "tc-in", "select", widget.Input{Name: "role", Kind: widget.InputSelect, Options: []widget.SelectOption{{Label: "One", Value: "1"}, {Label: "Two", Value: "2", Selected: true}}}.Render())
	add("primitives", "tc-check", "checkbox", widget.Check{Name: "agree", Label: "I agree", Checked: true}.Render())
	add("primitives", "tc-kbd", "keys", widget.Kbd{Keys: []string{"Ctrl", "K"}}.Render())
	add("primitives", "tc-close", "default", widget.Close{For: "demo", Label: "Close"}.Render())
	add("primitives", "tc-brand", "full", widget.Brand{Mark: "◆", Word: "Typecase", Href: "/"}.Render())
	add("primitives", "tc-brand", "markOnly", widget.Brand{Mark: "◆", Word: "Typecase", MarkOnly: true}.Render())
	add("primitives", "tc-mark", "default", widget.Mark{Colour: "#4a6b52", Label: "Group A"}.Render())
	n := 3
	add("primitives", "tc-num", "known", widget.Num{Value: &n}.Render())
	add("primitives", "tc-num", "unknown", widget.Num{}.Render())
	add("primitives", "tc-badge", "neutral", widget.Badge{Label: "Draft", Role: widget.BadgeNeutral}.Render())
	add("primitives", "tc-avatar", "initials", widget.Avatar{Initials: "AB", Colour: "#4a6b52"}.Render())
	add("primitives", "tc-tick", "on", widget.Tick{Done: true, Label: "Mark done"}.Render())
	add("primitives", "tc-tag", "on", widget.Tag{Label: "Notes", On: true, Href: "/tags/notes"}.Render())
	add("primitives", "tc-msg", "info", widget.Msg{Role: widget.MsgInfo, Title: "Saved", Text: "Your changes were stored."}.Render())
	add("primitives", "tc-empty", "default", widget.Empty{Title: "Nothing here", Text: "Create the first item.", Action: widget.Action{Label: "Create", Kind: widget.ButtonPrimary}}.Render())
	add("primitives", "tc-skel", "line", widget.Skel{Shape: widget.SkelLine, Lines: 3}.Render())
	add("primitives", "tc-meter", "default", widget.Meter{Value: 42, Max: 100, Label: "42%"}.Render())

	// Forms
	add("forms", "tc-form", "normal", widget.Form{Action: "/save", Sets: []widget.FieldSet{{Legend: "Details", Rows: []widget.Field{{Name: "title", Label: "Title", Kind: widget.InputText}}}}, Foot: widget.FormFoot{Primary: widget.Action{Label: "Save", Kind: widget.ButtonPrimary}}}.Render())
	add("forms", "tc-fset", "default", widget.FieldSet{Legend: "Account", Rows: []widget.Field{{Name: "email", Label: "Email", Kind: widget.InputEmail}}}.Render())
	add("forms", "tc-field", "normal", widget.Field{Name: "phone", Label: "Phone", Kind: widget.InputTel, Hint: "Include country code"}.Render())
	add("forms", "tc-formfoot", "default", widget.FormFoot{Primary: widget.Action{Label: "Save"}, Rest: []widget.Action{{Label: "Cancel"}}}.Render())
	add("forms", "tc-danger", "default", widget.Danger{Title: "Danger zone", Rows: []widget.DangerRow{{Title: "Remove all data", Text: "This cannot be undone.", Action: widget.Action{Label: "Remove all data", Kind: widget.ButtonDanger}}}}.Render())

	// Overlays
	add("overlays", "tc-scrim", "on", widget.Scrim{For: "sheet", On: true}.Render())
	add("overlays", "tc-overlay", "sheet", widget.Overlay{Kind: widget.OverlaySheet, Open: true, ID: "demo-overlay", Body: widget.Paragraph("Overlay body")}.Render())
	add("overlays", "tc-sheet", "open", widget.Sheet{Title: "Edit", Target: "Settings", Body: widget.Paragraph("Sheet body"), Open: true, Actions: []widget.Action{{Label: "Apply", Kind: widget.ButtonPrimary}}}.Render())
	add("overlays", "tc-dialog", "open", widget.Dialog{Title: "Delete?", Text: "This removes the item.", Open: true, Danger: true, Confirm: widget.Action{Label: "Delete item", Kind: widget.ButtonDanger}, Cancel: widget.Action{Label: "Keep"}}.Render())
	add("overlays", "tc-menu", "closed", widget.Menu{Label: "Actions", Items: []widget.MenuItem{{Label: "Edit", Href: "/edit"}, {Separator: true}, {Label: "Remove", Href: "/remove"}}}.Render())
	add("overlays", "tc-menu", "open", widget.Menu{Label: "Actions", State: widget.MenuOpen, Items: []widget.MenuItem{{Label: "Edit", Href: "/edit"}}}.Render())
	add("overlays", "tc-usermenu", "closed", widget.UserMenu{Name: "Alex", Note: "Signed in", Items: []widget.MenuItem{{Label: "Sign out", Href: "/logout"}}}.Render())

	// Panels
	add("panels", "tc-bar", "full", widget.Bar{Left: []widget.Action{{Label: "New"}}, Right: []widget.Action{{Label: "Export"}}, Rank2: []widget.Action{{Label: "Archive"}}}.Render())
	add("panels", "tc-seg", "default", widget.Seg{Options: []widget.SegOption{{Label: "Day", Value: "d"}, {Label: "Week", Value: "w"}}, Chosen: "d", Name: "range"}.Render())
	add("panels", "tc-chip", "on", widget.Chip{Label: "Active", On: true, Count: &n}.Render())
	add("panels", "tc-range", "default", widget.Range{From: "2026-01-01", To: "2026-01-31", Presets: []widget.Action{{Label: "Last 7 days"}}}.Render())

	// Lists
	add("lists", "tc-list", "normal", widget.List{Groups: []widget.ListGroup{{Rubric: widget.Rubric{Text: "Today"}, Rows: []widget.Row{{Cells: widget.RowCells{Title: "Item one", Subtitle: "Detail"}}}}}}.Render())
	add("lists", "tc-list", "empty", widget.List{Empty: widget.Empty{Title: "No items", Text: "Add one.", Action: widget.Action{Label: "Add"}}}.Render())
	add("lists", "tc-row", "default", widget.Row{Cells: widget.RowCells{Title: "Row", Subtitle: "Subtitle"}}.Render())
	add("lists", "tc-row--avatar", "default", widget.Row{Variant: widget.RowAvatar, Cells: widget.RowCells{Title: "Alex", Subtitle: "alex@example.com"}}.Render())
	add("lists", "tc-row--lead", "default", widget.Row{Variant: widget.RowLead, Cells: widget.RowCells{Lead: "09:00", Title: "Standup", Subtitle: "Room A"}}.Render())
	add("lists", "tc-row--tick", "default", widget.Row{Variant: widget.RowTick, Cells: widget.RowCells{Title: "Task", Due: "Fri"}}.Render())
	add("lists", "tc-row--excerpt", "default", widget.Row{Variant: widget.RowExcerpt, Cells: widget.RowCells{Title: "Post", Excerpt: "First lines of content…"}}.Render())
	add("lists", "tc-row--meta", "default", widget.Row{Variant: widget.RowMeta, Cells: widget.RowCells{Icon: "📄", Name: "readme.txt", Size: "2 KB", Time: "10:30"}}.Render())
	add("lists", "tc-row--find", "default", widget.Row{Variant: widget.RowFind, Cells: widget.RowCells{Icon: "⌕", Title: "Query", Detail: "Jump to section", Caption: "Recent"}}.Render())
	add("lists", "tc-tickrow", "ok", widget.TickRow{Label: "Inbox", Tick: widget.TickOn, Count: &n}.Render())
	add("lists", "tc-rubric", "default", widget.Rubric{Text: "Section", Count: &n}.Render())

	// Tables
	add("tables", "tc-table", "normal", widget.Table{Columns: []widget.Column{{Label: "Name", Kind: widget.ColText}, {Label: "Count", Kind: widget.ColNumber}}, Rows: []widget.TableRow{{Cells: []string{"Alpha", "1"}, Actions: []widget.Action{{Label: "Open", Href: "/a"}}}}}.Render())
	add("tables", "tc-grid", "default", widget.Grid{Tiles: []widget.Tile{{Name: "Photo", Icon: "🖼"}}}.Render())
	add("tables", "tc-tile", "icon", widget.Tile{Name: "Document", Icon: "📄"}.Render())
	add("tables", "tc-thumb", "icon", widget.Thumb{Icon: "📄"}.Render())
	add("tables", "tc-tree", "default", widget.Tree{Nodes: []widget.TreeNode{{ID: "a", Label: "Root", Open: true, Children: []widget.TreeNode{{ID: "b", Label: "Child"}}}}, Chosen: "b"}.Render())
	p := 60
	add("tables", "tc-queue", "running", widget.Queue{Rows: []widget.QueueRow{{Label: "Import", State: widget.QueueRunning, Percent: &p}}}.Render())

	// Shell
	add("shell", "tc-shell", "wide", widget.Shell{Top: widget.Top{Brand: widget.Brand{Mark: "◆", Word: "App"}}, Main: widget.Main{Label: "Main", Body: widget.Paragraph("Content")}}.Render())
	add("shell", "tc-top", "normal", widget.Top{Brand: widget.Brand{Mark: "◆", Word: "App"}, Search: widget.Search{Action: "/search"}}.Render())
	add("shell", "tc-main", "default", widget.Main{Label: "Workspace", Body: widget.Paragraph("Main area")}.Render())
	add("shell", "tc-rail", "column", widget.Rail{Sections: []widget.RailSection{{Title: "Views", Rows: []widget.TickRow{{Label: "All", Tick: widget.TickOn}}}}}.Render())
	add("shell", "tc-railsec", "ok", widget.RailSection{Title: "Groups", State: widget.RailSecOK, Rows: []widget.TickRow{{Label: "One"}}}.Render())
	add("shell", "tc-railfoot", "default", widget.RailFoot{Actions: []widget.Action{{Label: "Apply"}}, Build: widget.Build{Name: "typecase", Version: "0.0.2", Commit: "abc123", Href: "/about"}}.Render())
	add("shell", "tc-side", "open", widget.Side{Label: "Details", Title: "Item", Open: true, Blocks: []widget.SideBlock{{Sec: widget.Sec{Rubric: widget.Rubric{Text: "Info"}, Body: widget.Paragraph("Details")}}}}.Render())
	add("shell", "tc-nav", "column", widget.Nav{Items: []widget.NavItem{{Label: "Home", Icon: "⌂", Href: "/"}, {Label: "Settings", Icon: "⚙", Href: "/settings"}}, Current: "/"}.Render())
	add("shell", "tc-head", "default", widget.Head{Crumbs: widget.Crumbs{Items: []widget.Crumb{{Label: "Home", Href: "/"}, {Label: "Page"}}}, Title: "Page", Sub: "Subtitle"}.Render())
	add("shell", "tc-crumbs", "default", widget.Crumbs{Items: []widget.Crumb{{Label: "Root", Href: "/"}, {Label: "Here"}}}.Render())

	// Document
	add("document", "tc-doc", "reading", widget.Doc{Mode: widget.DocReading, Body: widget.Paragraph("Document body.")}.Render())
	add("document", "tc-prose", "default", widget.Prose{Blocks: []widget.ProseBlock{{Kind: widget.ProseH2, Text: "Heading"}, {Kind: widget.ProseP, Text: "Paragraph."}}}.Render())
	add("document", "tc-docbar", "dirty", widget.DocBar{Dirty: true, Left: []widget.Action{{Label: "Save", Kind: widget.ButtonPrimary}}}.Render())
	add("document", "tc-docmeta", "default", widget.DocMeta{Sections: []widget.MetaSection{{Title: "Tags", Tags: []widget.Tag{{Label: "draft", On: true}}}}}.Render())

	// Utility
	add("utility", "tc-search", "idle", widget.Search{Action: "/search", Query: ""}.Render())
	add("utility", "tc-search", "suggesting", widget.Search{Action: "/search", Query: "type", Suggesting: true, History: []widget.SearchRow{{Query: "types", Href: "/q/types"}}}.Render())
	add("utility", "tc-runbar", "running", widget.RunBar{Summary: "Syncing", Running: true, Percent: &p}.Render())
	add("utility", "tc-diag", "collapsed", widget.Diag{Summary: "Technical details", Lines: []string{"ERR connection reset"}}.Render())
	add("utility", "tc-diff", "open", widget.Diff{Property: "Title", Options: []widget.DiffOption{{Label: "Version A", Value: "a"}, {Label: "Version B", Value: "b"}}, Chosen: "a"}.Render())
	add("utility", "tc-crop", "full", widget.Crop{Href: "/img/sample.jpg", Zoom: 100}.Render())
	add("utility", "tc-steps", "default", widget.Steps{Items: []string{"Start", "Details", "Done"}, Current: 1}.Render())
	add("utility", "tc-card", "default", widget.Card{Rubric: widget.Rubric{Text: "Card"}, Body: widget.Paragraph("Card body")}.Render())
	add("utility", "tc-sec", "default", widget.Sec{Rubric: widget.Rubric{Text: "Section"}, Body: widget.Paragraph("Section body")}.Render())
	add("utility", "tc-fields", "default", widget.Fields{Rows: []widget.FieldRow{{Label: "Name", Value: "Example"}, {Label: "Path", Value: "/very/long/path/to/item", Path: true}}}.Render())
	add("utility", "tc-stat", "default", widget.Stat{Label: "Users", Value: "1,234", Hint: "Last 30 days"}.Render())
	add("utility", "tc-stats", "default", widget.Stats{Items: []widget.Stat{{Label: "Users", Value: "1,234"}, {Label: "Sessions", Value: "89"}, {Label: "Errors", Value: "3"}}}.Render())

	// Additional states (L-9.1)
	add("primitives", "tc-check", "on", widget.Check{Name: "on", Label: "On", Checked: true}.Render())
	add("primitives", "tc-check", "off", widget.Check{Name: "off", Label: "Off"}.Render())
	add("primitives", "tc-check", "disabled", widget.Check{Name: "d", Label: "Disabled", Reason: "Not allowed"}.Render())
	add("primitives", "tc-badge", "linked", widget.Badge{Label: "Linked", Role: widget.BadgeLinked}.Render())
	add("primitives", "tc-badge", "alert", widget.Badge{Label: "Alert", Role: widget.BadgeAlert}.Render())
	add("primitives", "tc-badge", "local", widget.Badge{Label: "Local", Role: widget.BadgeLocal}.Render())
	add("primitives", "tc-badge", "plan", widget.Badge{Label: "Plan", Role: widget.BadgePlan}.Render())
	add("primitives", "tc-avatar", "image", widget.Avatar{Image: "/img/sample.jpg", Initials: "AB"}.Render())
	add("primitives", "tc-tick", "off", widget.Tick{Label: "Mark done"}.Render())
	add("primitives", "tc-tick", "disabled", widget.Tick{Label: "Locked", Reason: "Read only"}.Render())
	add("primitives", "tc-tag", "off", widget.Tag{Label: "Draft", Href: "/tags/draft"}.Render())
	add("primitives", "tc-msg", "accent", widget.Msg{Role: widget.MsgAccent, Title: "Notice", Text: "Accent message."}.Render())
	add("primitives", "tc-msg", "alert", widget.Msg{Role: widget.MsgAlert, Title: "Error", Text: "Something failed."}.Render())

	add("forms", "tc-form", "busy", widget.Form{Action: "/save", Busy: true, Sets: []widget.FieldSet{{Rows: []widget.Field{{Name: "title", Label: "Title", Kind: widget.InputText}}}}, Foot: widget.FormFoot{Primary: widget.Action{Label: "Save", Reason: "Busy"}}}.Render())
	add("forms", "tc-field", "error", widget.Field{Name: "email", Label: "Email", Kind: widget.InputEmail, Error: "Invalid address"}.Render())
	add("forms", "tc-field", "read-only", widget.Field{Name: "id", Label: "ID", Kind: widget.InputText, Value: "42", ReadOnly: true}.Render())
	add("forms", "tc-field", "disabled", widget.Field{Name: "role", Label: "Role", Kind: widget.InputText, Disabled: true}.Render())

	add("overlays", "tc-scrim", "off", widget.Scrim{For: "sheet", On: false}.Render())
	add("overlays", "tc-overlay", "dialog", widget.Overlay{Kind: widget.OverlayDialog, Open: true, ID: "dlg", Body: widget.Paragraph("Dialog body")}.Render())
	add("overlays", "tc-overlay", "popover", widget.Overlay{Kind: widget.OverlayPopover, Open: true, ID: "pop", Body: widget.Paragraph("Popover")}.Render())
	add("overlays", "tc-sheet", "closed", widget.Sheet{Title: "Edit", Target: "Settings", Body: widget.Paragraph("Closed"), Open: false}.Render())
	add("overlays", "tc-dialog", "closed", widget.Dialog{Title: "Delete?", Text: "Closed.", Open: false, Confirm: widget.Action{Label: "Delete", Kind: widget.ButtonDanger}, Cancel: widget.Action{Label: "Keep"}}.Render())
	add("overlays", "tc-menu", "disabled", widget.Menu{Label: "Actions", State: widget.MenuDisabled, Items: []widget.MenuItem{{Label: "Edit", Href: "/edit"}}}.Render())
	add("overlays", "tc-usermenu", "open", widget.UserMenu{Name: "Alex", Note: "Signed in", State: widget.MenuOpen, Items: []widget.MenuItem{{Label: "Sign out", Href: "/logout"}}}.Render())

	add("panels", "tc-bar", "collapsed", widget.Bar{Collapsed: true, Left: []widget.Action{{Label: "New"}}}.Render())
	add("panels", "tc-chip", "off", widget.Chip{Label: "Inactive", On: false}.Render())

	add("lists", "tc-row", "normal", widget.Row{Cells: widget.RowCells{Title: "Row", Subtitle: "Subtitle"}}.Render())
	add("lists", "tc-row", "selected", widget.Row{State: widget.RowSelected, Cells: widget.RowCells{Title: "Selected"}}.Render())
	add("lists", "tc-row", "muted", widget.Row{State: widget.RowMuted, Cells: widget.RowCells{Title: "Muted"}}.Render())
	add("lists", "tc-row", "alert", widget.Row{State: widget.RowAlert, Cells: widget.RowCells{Title: "Alert"}}.Render())
	add("lists", "tc-tickrow", "read-only", widget.TickRow{Label: "Archive", State: widget.TickRowReadOnly, Tick: widget.TickOn}.Render())
	add("lists", "tc-tickrow", "unavailable", widget.TickRow{Label: "Remote", State: widget.TickRowUnavailable}.Render())
	add("lists", "tc-tickrow", "gone", widget.TickRow{Label: "Removed", State: widget.TickRowGone}.Render())

	add("tables", "tc-table", "empty", widget.Table{Columns: []widget.Column{{Label: "Name"}}, Empty: widget.Empty{Title: "No rows", Text: "Add data.", Action: widget.Action{Label: "Add"}}}.Render())
	add("tables", "tc-tile", "image", widget.Tile{Name: "Photo", Thumb: widget.Thumb{Href: "/img/sample.jpg", Alt: "Sample"}}.Render())
	add("tables", "tc-thumb", "image", widget.Thumb{Href: "/img/sample.jpg", Alt: "Sample"}.Render())
	add("tables", "tc-thumb", "loading", widget.Thumb{Loading: true}.Render())
	add("tables", "tc-tree", "normal", widget.Tree{Nodes: []widget.TreeNode{{ID: "tree-root-a", Label: "Root"}}}.Render())
	add("tables", "tc-tree", "disabled", widget.Tree{Nodes: []widget.TreeNode{{ID: "tree-root-b", Label: "Root"}}, Disabled: []string{"tree-root-b"}}.Render())
	add("tables", "tc-queue", "idle", widget.Queue{Rows: []widget.QueueRow{{Label: "Waiting", State: widget.QueueIdle}}}.Render())
	add("tables", "tc-queue", "done", widget.Queue{Rows: []widget.QueueRow{{Label: "Done", State: widget.QueueDone}}}.Render())
	add("tables", "tc-queue", "failed", widget.Queue{Rows: []widget.QueueRow{{Label: "Failed", State: widget.QueueFailed}}}.Render())
	add("tables", "tc-queue", "partial", widget.Queue{Rows: []widget.QueueRow{{Label: "Partial", State: widget.QueuePartial}}}.Render())
	add("tables", "tc-queue", "cancelled", widget.Queue{Rows: []widget.QueueRow{{Label: "Cancelled", State: widget.QueueCancelled}}}.Render())

	add("shell", "tc-shell", "narrow", widget.Shell{Narrow: true, Main: widget.Main{Label: "Main", Body: widget.Paragraph("Narrow")}}.Render())
	add("shell", "tc-top", "narrow", widget.Top{Narrow: true, Brand: widget.Brand{Mark: "◆", Word: "App"}}.Render())
	add("shell", "tc-rail", "icons", widget.Rail{Layout: widget.RailLayoutIcons, Sections: []widget.RailSection{{Title: "Views", Rows: []widget.TickRow{{Label: "All"}}}}}.Render())
	add("shell", "tc-rail", "drawer", widget.Rail{Layout: widget.RailLayoutDrawer, Open: true, Sections: []widget.RailSection{{Title: "Views", Rows: []widget.TickRow{{Label: "All"}}}}}.Render())
	add("shell", "tc-railsec", "muted", widget.RailSection{Title: "Muted", State: widget.RailSecMuted, Rows: []widget.TickRow{{Label: "One"}}}.Render())
	add("shell", "tc-railsec", "alert", widget.RailSection{Title: "Alert", State: widget.RailSecAlert, Rows: []widget.TickRow{{Label: "One"}}}.Render())
	add("shell", "tc-side", "closed", widget.Side{Label: "Details", Title: "Item", Open: false, Blocks: []widget.SideBlock{{Sec: widget.Sec{Rubric: widget.Rubric{Text: "Info"}, Body: widget.Paragraph("Details")}}}}.Render())
	add("shell", "tc-nav", "icons", widget.Nav{Layout: widget.NavLayoutIcons, Items: []widget.NavItem{{Label: "Home", Icon: "⌂", Href: "/"}}, Current: "/"}.Render())
	add("shell", "tc-nav", "bottom", widget.Nav{Layout: widget.NavLayoutBottom, Items: []widget.NavItem{{Label: "Home", Icon: "⌂", Href: "/"}}, Current: "/"}.Render())

	add("document", "tc-doc", "editing", widget.Doc{Mode: widget.DocEditing, Body: widget.Paragraph("Editing.")}.Render())
	add("document", "tc-docbar", "full", widget.DocBar{Left: []widget.Action{{Label: "Save", Kind: widget.ButtonPrimary}}}.Render())
	add("document", "tc-docbar", "collapsed", widget.DocBar{Collapsed: true, Left: []widget.Action{{Label: "Save"}}}.Render())

	add("utility", "tc-runbar", "default", widget.RunBar{Summary: "Idle"}.Render())
	add("utility", "tc-diag", "expanded", widget.Diag{Summary: "Details", Expanded: true, Lines: []string{"line 1"}}.Render())
	add("utility", "tc-diff", "agreed", widget.Diff{Property: "Title", Agreed: true, Options: []widget.DiffOption{{Label: "A", Value: "a"}}, Chosen: "a"}.Render())
	add("utility", "tc-crop", "centre", widget.Crop{Href: "/img/sample.jpg", Narrow: true}.Render())

	return out
}

// PageHTML renders a full catalog page.
func PageHTML(title string, entries []Entry, withJS bool) string {
	var sections strings.Builder
	family := ""
	for _, e := range entries {
		if e.Family != family {
			if family != "" {
				sections.WriteString("</section>")
			}
			family = e.Family
			sections.WriteString(`<section class="catalog-family"><h2>` + family + `</h2>`)
		}
		sections.WriteString(`<article class="catalog-entry" id="` + slug(e.Widget) + `-` + slug(e.State) + `"><h3><code>` + e.Widget + `</code> — ` + e.State + `</h3><div class="catalog-demo">` + e.HTML + `</div></article>`)
	}
	if family != "" {
		sections.WriteString("</section>")
	}
	// The theme boot script runs on both pages: it must precede first paint
	// (L-5.3) and is not widget behavior. Only tc.js is withheld from the
	// second page.
	js := ""
	if withJS {
		js = `<script src="/tc.js"></script>`
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" data-tc-root>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title>
<script src="/theme.js"></script>
<link rel="stylesheet" href="/typecase.css">
<link rel="stylesheet" href="/catalog.css">
</head>
<body data-tc-root>
<header class="catalog-nav"><a href="/">With behavior</a><a href="/static">Without behavior</a></header>
<h1>%s</h1>
%s
%s
</body>
</html>`, title, title, sections.String(), js)
}

func slug(s string) string {
	return strings.NewReplacer(" ", "-", "/", "-", ".", "-").Replace(s)
}
