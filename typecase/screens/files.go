package screens

import "carrel.dev/typecase/widget"

// Files assembles mockup s-files figure 1: folder view, table, tiles, queue,
// move dialog, filter chips, and the properties side panel.
func Files() string {
	return shellWrap(filesRail(), filesMain(), filesSide(), filesOverlays())
}

func filesRail() string {
	n2of3 := 2
	return shellNav("/files") +
		widget.Rail{
			Sections: []widget.RailSection{
				{
					Rows: []widget.TickRow{{
						Label: "Selected collections",
						Count: intPtr(n2of3),
						Tick:  widget.TickPartial,
					}},
				},
				{
					Title: "sftpgo · vps",
					Rows: []widget.TickRow{
						{Label: "Documents", Mark: widget.Mark{Colour: "#4A6B52", Label: "Documents"}, Tick: widget.TickOn},
						{Label: "Public", Mark: widget.Mark{Colour: "#9A9280", Label: "Public"}, Tick: widget.TickOff, State: widget.TickRowReadOnly},
					},
				},
				{
					Title: "nextcloud · home",
					Rows: []widget.TickRow{
						{Label: "Files", Mark: widget.Mark{Colour: "#3F6E8C", Label: "Files"}},
					},
				},
				{
					Title: "Shortcuts",
					Rows: []widget.TickRow{
						{Label: "Attachments"},
						{Label: "Published", Count: intPtr(2)},
					},
				},
			},
			Foot: widget.RailFoot{
				Actions: []widget.Action{{Label: "Connect account", Icon: "+"}},
				Build:   demoBuild,
			},
		}.Render()
}

func filesMain() string {
	head := widget.Head{
		Title: "attachments",
		Crumbs: widget.Crumbs{Items: []widget.Crumb{
			{Label: "Documents", Href: "/files/documents"},
			{Label: "carrel", Href: "/files/documents/carrel"},
			{Label: "attachments"},
		}},
		Sub: "sftpgo · vps",
		Actions: []widget.Action{
			{Label: "Upload", Kind: widget.ButtonPrimary, Icon: "↑"},
			{Label: "New folder", Icon: "+"},
			{Label: "Refresh", Icon: "↻"},
		},
	}.Render()

	selBar := `<div class="tc-bar"><div class="tc-bar__left"><strong>1 selected</strong> <span class="tc-row__size">412 KB</span></div>` +
		widget.Bar{
			Separators: true,
			Left: []widget.Node{
				widget.Action{Label: "Download", Icon: "↓"},
				widget.Action{Label: "Rename", Icon: "✎"},
				widget.Action{Label: "Move", Icon: "⇄"},
				widget.Action{Label: "Copy", Icon: "⧉"},
				widget.Action{Label: "Delete", Kind: widget.ButtonDanger, Icon: "🗑"},
			},
			Right: []widget.Node{widget.Action{Label: "Clear selection", Size: widget.ButtonSizeBar}},
		}.Render() + `</div>`

	chips := widget.Chip{Label: "images", On: true}.Render() +
		widget.Chip{Label: "documents"}.Render() +
		widget.Chip{Label: "folders"}.Render()

	filterBar := `<div class="tc-bar"><div class="tc-bar__left">` +
		widget.Input{Name: "filter", Kind: widget.InputText, Placeholder: "Filter this folder"}.Render() +
		widget.Seg{
			Name:    "sort",
			Chosen:  "name",
			Options: []widget.SegOption{{Label: "Name", Value: "name"}, {Label: "Size", Value: "size"}, {Label: "Changed", Value: "changed"}},
		}.Render() + chips + `</div><div class="tc-bar__right">` +
		widget.Button{Label: "Columns", Icon: "▦", Size: widget.ButtonSizeBar}.Render() +
		widget.Num{Value: intPtr(14)}.Render() + `</div></div>`

	table := widget.Table{
		Columns: []widget.Column{
			{Label: "Name"},
			{Label: "Size", Kind: widget.ColNumber},
			{Label: "Type"},
			{Label: "Changed"},
		},
		Rows: []widget.TableRow{
			{Cells: []string{"↑ up one folder", "", "", ""}},
			{Cells: []string{"📁 2026-07", "—", "folder · 34 items", "2026-07-01 09:12"}},
			{
				Selected: true,
				Cells:    []string{"🖼 2026-08-10-sync-board.png", "412 KB", "image/png", "2026-08-10 11:07"},
				Actions:  []widget.Action{{Label: "Download", Icon: "↓"}, {Label: "More", Icon: "⋯"}},
			},
			{
				Cells:   []string{"📄 release-checklist.md", "3 KB", "text/markdown", "2026-08-09 18:44"},
				Actions: []widget.Action{{Label: "Download", Icon: "↓"}, {Label: "More", Icon: "⋯"}},
			},
			{
				Cells:   []string{"📄 смета-подрядчик.pdf", "1.2 MB", "application/pdf", "2026-08-05 14:20"},
				Actions: []widget.Action{{Label: "Download", Icon: "↓"}, {Label: "More", Icon: "⋯"}},
			},
		},
	}.Render()

	metaList := widget.List{
		Groups: []widget.ListGroup{{
			Rows: []widget.Row{
				{
					Variant:  widget.RowMeta,
					Selected: true,
					Cells: widget.RowCells{
						Icon: "🖼",
						Name: "2026-08-10-sync-board.png",
						Size: "412 KB",
						Time: "2026-08-10 11:07",
					},
				},
				{
					Variant: widget.RowMeta,
					Cells: widget.RowCells{
						Icon: "📄",
						Name: "release-checklist.md",
						Size: "3 KB",
						Time: "2026-08-09 18:44",
					},
				},
			},
		}},
	}.Render()

	grid := widget.Grid{Tiles: []widget.Tile{
		{Name: "2026-08-10-sync-board.png", Selected: true, Thumb: widget.Thumb{Href: "/img/sample.svg", Alt: "Board"}},
		{Name: "2026-08-04-whiteboard.jpg", Thumb: widget.Thumb{Href: "/img/sample.svg", Alt: "Whiteboard"}},
		{Name: "release-checklist.md", Icon: "📄"},
	}}.Render()

	p100, p62 := 100, 62
	queue := widget.Queue{
		Rows: []widget.QueueRow{
			{Label: "отчёт-июль.pdf · 4.1 MB · done", State: widget.QueueDone, Percent: &p100},
			{Label: "плакат.png · 8.3 MB · 62%", State: widget.QueueRunning, Percent: &p62},
			{
				Label:    "release-checklist.md",
				State:    widget.QueueFailed,
				Question: "name taken",
				Choices:  []widget.Action{{Label: "Keep both"}, {Label: "Replace"}, {Label: "Skip"}},
			},
		},
		Note: "Uploading 2 of 3 · 12.4 MB",
	}.Render()

	moveCard := widget.Card{
		Rubric: widget.Rubric{Text: "Move 3 items"},
		Body:   []widget.Node{widget.Paragraph("from Documents / carrel / attachments")},
	}.Render() +
		widget.Tree{
			Nodes: []widget.TreeNode{
				{ID: "documents", Label: "Documents", Open: true, Children: []widget.TreeNode{
					{ID: "carrel", Label: "carrel", Open: true, Children: []widget.TreeNode{
						{ID: "2026-07", Label: "2026-07"},
						{ID: "attachments", Label: "attachments"},
					}},
				}},
				{ID: "files-nc", Label: "Files"},
			},
			Chosen: "2026-07",
		}.Render() +
		widget.FormFoot{
			Primary: widget.Action{Label: "Move here", Kind: widget.ButtonPrimary},
			Rest:    []widget.Action{{Label: "Cancel"}},
		}.Render()

	msg := widget.Msg{Text: "Drop files onto the window, or onto a folder in the list. A name that is already taken is refused rather than overwritten, and the queue says so."}.Render()

	return `<main class="tc-main" aria-label="Files"><div class="tc-main__body">` +
		head + selBar + filterBar + table + metaList + grid + msg + moveCard + queue +
		`</div></main>`
}

func filesSide() string {
	id := "files-side"
	fields := widget.Fields{Rows: []widget.FieldRow{
		{Label: "Size", Value: "412 KB"},
		{Label: "Type", Value: "image/png"},
		{Label: "Changed", Value: "2026-08-10 11:07"},
		{Label: "Path", Value: "/dav/Documents/carrel/attachments/2026-08-10-sync-board.png", Path: true},
	}}.Render()

	links := widget.Sec{
		Rubric: widget.Rubric{Text: "Links"},
		Body:   []widget.Node{widget.Paragraph("Carrel keeps no index of what points at this file. It may be attached to a note or an event — check before deleting.")},
	}.Render()

	return `<aside class="tc-side is-open" data-tc-side id="` + id + `" aria-label="File details">` +
		`<header class="tc-side__head"><h2 class="tc-side__title">2026-08-10-sync-board.png</h2>` +
		widget.Close{For: id, Label: "Close the panel"}.Render() +
		`</header><div class="tc-side__body">` +
		widget.Thumb{Href: "/img/sample.svg", Alt: "Preview", Size: widget.ThumbSizeTile}.Render() +
		widget.Bar{Left: []widget.Node{
			widget.Action{Label: "Download", Kind: widget.ButtonPrimary, Icon: "↓"},
			widget.Action{Label: "Open"},
			widget.Action{Label: "Rename", Icon: "✎"},
			widget.Action{Label: "Move", Icon: "⇄"},
		}}.Render() + fields + links +
		widget.Danger{Title: "Danger zone", Rows: []widget.DangerRow{{
			Title:  "Delete",
			Text:   "Removes the file from the server.",
			Action: widget.Action{Label: "Delete", Kind: widget.ButtonDanger, Icon: "🗑"},
		}}}.Render() +
		`</div></aside>`
}

func filesOverlays() string {
	return widget.Dialog{
		ID:      "move-dialog",
		Title:   "Move 3 items",
		Text:    "Choose a folder in the tree below.",
		Confirm: widget.Action{Label: "Move here", Kind: widget.ButtonPrimary},
		Cancel:  widget.Action{Label: "Cancel"},
	}.Render()
}
