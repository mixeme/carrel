package screens

import "carrel.dev/typecase/widget"

// Files assembles mockup s-files figure 1: folder view, table, tiles, queue,
// move dialog, filter chips, and the properties side panel.
func Files() string {
	return shellFrame(shellNav("/files"), filesRail(), filesMain(), filesSide(), filesOverlays())
}

func filesRail() widget.Rail {
	n2of3 := 2
	return widget.Rail{
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
			Actions: []widget.Action{{Label: "Connect account", Icon: "tc-i-server"}},
			Build:   demoBuild,
		},
	}
}

func filesMain() widget.Main {
	head := widget.Head{
		Title: "attachments",
		Crumbs: widget.Crumbs{Items: []widget.Crumb{
			{Label: "Documents", Href: "/files/documents"},
			{Label: "carrel", Href: "/files/documents/carrel"},
			{Label: "attachments"},
		}},
		Sub: "sftpgo · vps",
		Actions: []widget.Action{
			{Label: "Upload", Kind: widget.ButtonPrimary, Icon: "tc-i-up"},
		},
		Rank2: []widget.Action{
			{Label: "New folder", Icon: "tc-i-plus"},
			{Label: "Refresh", Icon: "tc-i-refresh"},
		},
	}

	selBar := widget.Bar{
		Separators: true,
		Left: []widget.Node{
			widget.Paragraph("1 selected · 412 KB"),
			widget.Action{Label: "Download", Icon: "tc-i-down"},
		},
		Rank2: []widget.Action{
			{Label: "Rename", Icon: "tc-i-pencil"},
			{Label: "Move", Icon: "tc-i-move"},
			{Label: "Copy", Icon: "tc-i-dup"},
			{Label: "Delete", Kind: widget.ButtonDanger, Icon: "tc-i-trash"},
		},
		Right: []widget.Node{widget.Action{Label: "Clear selection", Size: widget.ButtonSizeBar}},
	}

	filterBar := widget.Bar{
		Left: []widget.Node{
			widget.Input{Name: "filter", Kind: widget.InputText, Placeholder: "Filter this folder"},
			widget.Seg{
				Name:    "sort",
				Chosen:  "name",
				Options: []widget.SegOption{{Label: "Name", Value: "name"}, {Label: "Size", Value: "size"}, {Label: "Changed", Value: "changed"}},
			},
		},
		Right: []widget.Node{
			widget.Button{Label: "Columns", Icon: "tc-i-cols", Size: widget.ButtonSizeBar},
			widget.Num{Value: intPtr(14)},
		},
	}

	chips := []widget.Node{
		widget.Chip{Label: "images", On: true},
		widget.Chip{Label: "documents"},
		widget.Chip{Label: "folders"},
	}

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
				Actions:  []widget.Action{{Label: "Download", Icon: "tc-i-down"}, {Label: "More", Icon: "tc-i-more"}},
			},
			{
				Cells:   []string{"📄 release-checklist.md", "3 KB", "text/markdown", "2026-08-09 18:44"},
				Actions: []widget.Action{{Label: "Download", Icon: "tc-i-down"}, {Label: "More", Icon: "tc-i-more"}},
			},
			{
				Cells:   []string{"📄 смета-подрядчик.pdf", "1.2 MB", "application/pdf", "2026-08-05 14:20"},
				Actions: []widget.Action{{Label: "Download", Icon: "tc-i-down"}, {Label: "More", Icon: "tc-i-more"}},
			},
		},
	}

	metaList := widget.List{
		Groups: []widget.ListGroup{{
			Rows: []widget.Row{
				{
					Variant:  widget.RowMeta,
					Selected: true,
					Cells: widget.RowCells{
						Icon: "tc-i-img",
						Name: "2026-08-10-sync-board.png",
						Size: "412 KB",
						Time: "2026-08-10 11:07",
					},
				},
				{
					Variant: widget.RowMeta,
					Cells: widget.RowCells{
						Icon: "tc-i-doc",
						Name: "release-checklist.md",
						Size: "3 KB",
						Time: "2026-08-09 18:44",
					},
				},
			},
		}},
	}

	grid := widget.Grid{Tiles: []widget.Tile{
		{Name: "2026-08-10-sync-board.png", Selected: true, Thumb: widget.Thumb{Href: "/img/sample.svg", Alt: "Board"}},
		{Name: "2026-08-04-whiteboard.jpg", Thumb: widget.Thumb{Href: "/img/sample.svg", Alt: "Whiteboard"}},
		{Name: "release-checklist.md", Icon: "tc-i-doc"},
	}}

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
	}

	moveCard := widget.Sec{
		Rubric: widget.Rubric{Text: "Move 3 items"},
		Body: []widget.Node{
			widget.Paragraph("from Documents / carrel / attachments"),
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
			},
			widget.FormFoot{
				Primary: widget.Action{Label: "Move here", Kind: widget.ButtonPrimary},
				Rest:    []widget.Action{{Label: "Cancel"}},
			},
		},
	}

	return widget.Main{
		Label: "Files",
		Body: []widget.Node{
			head,
			selBar,
			filterBar,
			chips[0],
			chips[1],
			chips[2],
			table,
			metaList,
			grid,
			widget.Msg{Text: "Drop files onto the window, or onto a folder in the list. A name that is already taken is refused rather than overwritten, and the queue says so."},
			moveCard,
			queue,
		},
	}
}

func filesSide() widget.Side {
	fields := widget.Fields{Rows: []widget.FieldRow{
		{Label: "Size", Value: "412 KB"},
		{Label: "Type", Value: "image/png"},
		{Label: "Changed", Value: "2026-08-10 11:07"},
		{Label: "Path", Value: "/dav/Documents/carrel/attachments/2026-08-10-sync-board.png", Path: true},
	}}

	links := widget.Sec{
		Rubric: widget.Rubric{Text: "Links"},
		Body:   []widget.Node{widget.Paragraph("Carrel keeps no index of what points at this file. It may be attached to a note or an event — check before deleting.")},
	}

	danger := widget.Sec{
		Body: []widget.Node{widget.Danger{Title: "Danger zone", Rows: []widget.DangerRow{{
			Title:  "Delete",
			Text:   "Removes the file from the server.",
			Action: widget.Action{Label: "Delete", Kind: widget.ButtonDanger, Icon: "tc-i-trash"},
		}}}},
	}

	return widget.Side{
		Label: "File details",
		Title: "2026-08-10-sync-board.png",
		Open:  true,
		Blocks: []widget.SideBlock{
			{Sec: &widget.Sec{Body: []widget.Node{
				widget.Thumb{Href: "/img/sample.svg", Alt: "Preview", Size: widget.ThumbSizeTile},
				widget.Bar{Left: []widget.Node{
					widget.Action{Label: "Download", Kind: widget.ButtonPrimary, Icon: "tc-i-down"},
					widget.Action{Label: "Open"},
					widget.Action{Label: "Rename", Icon: "tc-i-pencil"},
					widget.Action{Label: "Move", Icon: "tc-i-move"},
				}},
			}}},
			{Fields: &fields},
			{Sec: &links},
			{Sec: &danger},
		},
	}
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
