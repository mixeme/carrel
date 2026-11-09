package settings

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of the external trigger's sheet (U-1.3): the cards' rubrics —
// by the tab open, what it hands over —, the mark of a password shown once,
// Copy and what it says after, the IDs of what is copied, the key of
// Cancel, the fields of GoSentry and of Gitea.
const (
	pwRubric      = "Password"
	lineRubric    = "The line to paste"
	scriptRubric  = "The script to paste"
	fieldsRubric  = "The fields to paste"
	giteaRubric   = "The workflow to paste"
	curlRubric    = "The request to send"
	onlyNow       = "visible only now"
	copyWord      = "Copy"
	copyLine      = "Copy the line"
	copyScript    = "Copy the script"
	copyWorkflow  = "Copy the workflow"
	saveAs        = "Save as "
	triggerPwID   = "trigger-pw"
	triggerLineID = "trigger-line"
	escKey        = "Esc"
	tabGoSentry   = "backup.trigger.tab.gosentry"
	tabGitea      = "backup.trigger.tab.gitea"
	tabCurl       = "backup.trigger.tab.curl"
	gsName        = "Name"
	gsFolder      = "Folder"
	gsSchedule    = "Schedule"
	gsCommand     = "Command"
	gsArguments   = "Arguments"
	giteaSecret   = "Secret"
	giteaValue    = "Its value"
)

// TriggerSheet is a job's external trigger over the list of jobs,
// /app/settings/backup/trigger (BackupTriggerSheet, s-backup-12…14): the
// job on the right of its title; off — what turning it on costs, in alarm,
// Turn on and Cancel (s-backup-14); on — the password as a row, how it was
// used, New password and Revoke, and the line to paste with the password
// hidden (s-backup-12); just issued — the password and the line whole, to
// copy, once (s-backup-13).
func TriggerSheet(p *view.BackupTriggerSheet) w.Sheet {
	s := w.Sheet{Title: string(p.Title), Width: w.SheetWidth680, Action: p.Form.Action, Method: p.Form.Method, Fields: bind.Hidden(p.Form)}
	if p.Subject != "" {
		s.Head = []w.SheetHeadPart{w.Dim{Parts: []w.Inline{w.Text(p.Subject)}, Size: w.Dim11_5, Variant: w.DimWrap, Place: w.Pushed}}
	}
	if p.Warning != "" {
		s.Body = append(s.Body, w.Msg{Parts: []w.MsgPart{w.Text(p.Warning)}, Tone: w.MsgAlert, Space: w.MsgFlush})
	}
	if pw := p.Password; pw != nil {
		s.Body = append(s.Body, passwordCard(pw))
	}
	if l := p.Line; l != nil {
		s.Body = append(s.Body, lineCard(l, p.Local))
	}
	for _, a := range p.Form.Actions {
		if a.Method == "GET" {
			s.Foot = append(s.Foot, w.Line{Items: []w.LinePart{bind.Button(a, ""), w.Kbd{Keys: escKey}}, Gap: w.LineGapNormal, Place: w.Pushed})
			continue
		}
		s.Foot = append(s.Foot, bind.Button(a, ""))
		if a.Primary {
			s.Foot = append(s.Foot, w.Kbd{Keys: enterKey})
		}
	}
	return s
}

// passwordCard is the trigger's password (s-backup-12, -13): issued just
// now, whole in a field to copy, marked as shown only now; else its hint as
// a session's row — when it was issued, its last run and where from, how
// many —, New password and Revoke at its end.
func passwordCard(pw *view.TriggerPassword) w.Card {
	if pw.Value != "" {
		in := w.Input{Value: string(pw.Value), Label: pwRubric, Mono: true, ReadOnly: true, Variant: w.InputKeySmFill, ID: triggerPwID}
		return w.Card{Variant: w.CardFlush, Items: []w.CardPart{
			w.Rubric{Text: pwRubric, Items: []w.RubricPart{w.Mono{Text: onlyNow, Variant: w.MonoTightDanger}}},
			w.Line{Items: []w.LinePart{in, w.Button{Label: copyWord, CopyFrom: triggerPwID, Copied: copiedWord}}, Gap: w.LineGapNormal, Block: true},
		}}
	}
	row := w.Line{Gap: w.LineGapNormal, Block: true, Items: []w.LinePart{w.Mono{Text: string(pw.Masked)}}}
	if len(pw.Use) > 0 {
		row.Items = append(row.Items, w.Dim{Parts: bind.Inlines(pw.Use), Size: w.Dim11_5})
	}
	acts := w.Line{Gap: w.LineGapNormal, Place: w.Pushed}
	for _, a := range pw.Actions {
		b := bind.Button(a, "")
		if a.Danger {
			b.Kind = w.ButtonDanger
		}
		acts.Items = append(acts.Items, b)
	}
	if len(acts.Items) > 0 {
		row.Items = append(row.Items, acts)
	}
	return w.Card{Variant: w.CardFlush, Items: []w.CardPart{w.Rubric{Text: pwRubric}, row}}
}

// lineCard is what the trigger hands over, by tab (s-backup-12, -13; the
// tabs' fragments s-backup-3, -4, -10, -11): the tabs as links; crontab —
// the line, or the script a job that downloads needs and why; GoSentry —
// the fields of its job dialog; Gitea — the workflow and its secret; curl
// — the request alone. Shown whole only when the password was just issued,
// with Copy then; the note under it.
func lineCard(l *view.TriggerBlock, local bool) w.Card {
	open := ""
	seg := w.Seg{Variant: w.SegStart}
	for _, t := range l.Tabs {
		seg.Options = append(seg.Options, bind.SegOption(t))
		if t.On {
			open = t.ID
		}
	}
	rubric := w.Rubric{Text: lineRubric}
	switch {
	case open == tabGoSentry:
		rubric.Text = fieldsRubric
	case open == tabGitea:
		rubric.Text = giteaRubric
	case open == tabCurl:
		rubric.Text = curlRubric
	case l.Script:
		rubric.Text = scriptRubric
	}
	if local {
		rubric.Items = append(rubric.Items, w.Badge{Label: localBadge, Tone: w.BadgeLocal, Title: localHint})
	}
	card := w.Card{Variant: w.CardFlush, Items: []w.CardPart{rubric}}
	if len(l.Destination) > 0 && open != tabGoSentry && open != tabGitea && open != tabCurl {
		card.Items = append(card.Items, w.CardText{Kind: w.CardAbove, Parts: cardParts(l.Destination)})
	}
	card.Items = append(card.Items, seg)
	switch open {
	case tabGoSentry:
		card.Items = append(card.Items, goSentryGrid(l.GoSentry, l.Shown))
	case tabGitea:
		card.Items = append(card.Items, w.Diag{Text: string(l.Gitea), Variant: w.DiagInk, ID: triggerLineID})
		if l.Shown {
			card.Items = append(card.Items, w.CardRow{Kind: w.CardTop8, Gap: w.LineGapNormal, Items: []w.CardRowPart{
				w.Button{Label: copyWorkflow, Icon: "dup", Kind: w.ButtonPrimary, CopyFrom: triggerLineID, Copied: copiedWord},
				w.Button{Label: saveAs + string(l.GiteaFile), Icon: "download", SaveAs: baseName(string(l.GiteaFile)), SaveFrom: triggerLineID},
			}})
		}
		card.Items = append(card.Items, w.CardGrid{Items: []w.CardGridPart{
			w.Dim{Parts: []w.Inline{w.Text(giteaSecret)}, Size: w.Dim11}, w.Input{Value: string(l.GiteaSecret), Label: giteaSecret, Mono: true, ReadOnly: true, Variant: w.Input11},
			copyButton(view.Action{Label: copyWord, Enabled: true}, string(l.GiteaSecret)),
		}})
	case tabCurl:
		card.Items = append(card.Items, w.Diag{Text: string(l.Curl), Variant: w.DiagInk, ID: triggerLineID})
		if l.Shown {
			card.Items = append(card.Items, copyRow(copyLine))
		}
	default:
		card.Items = append(card.Items, w.Diag{Text: string(l.Command), Variant: w.DiagInk, ID: triggerLineID})
		if l.Shown {
			label := copyLine
			if l.Script {
				label = copyScript
			}
			card.Items = append(card.Items, copyRow(label))
		}
	}
	if len(l.Note) > 0 {
		card.Items = append(card.Items, w.CardText{Kind: w.CardUsage, Parts: cardParts(l.Note)})
	}
	return card
}

// copyRow is the card's Copy of its block (the line, the script).
func copyRow(label string) w.CardRow {
	return w.CardRow{Kind: w.CardTop8, Gap: w.LineGapNormal, Items: []w.CardRowPart{
		w.Button{Label: label, Icon: "dup", Kind: w.ButtonPrimary, CopyFrom: triggerLineID, Copied: copiedWord},
	}}
}

// goSentryGrid is the fields of a GoSentry job (s-backup-4), each to copy:
// its name, folder, schedule, command, and the arguments one a line.
func goSentryGrid(g view.GoSentryJob, shown bool) w.CardGrid {
	grid := w.CardGrid{}
	field := func(label string, value string, mono bool) {
		grid.Items = append(grid.Items, w.Dim{Parts: []w.Inline{w.Text(label)}, Size: w.Dim11},
			w.Input{Value: value, Label: label, Mono: mono, ReadOnly: true, Variant: w.Input11},
			copyButton(view.Action{Label: copyWord, Enabled: shown}, value))
	}
	field(gsName, string(g.Name), false)
	field(gsFolder, string(g.Folder), false)
	field(gsSchedule, string(g.Schedule), true)
	field(gsCommand, string(g.Command), true)
	if len(g.Arguments) > 0 {
		args := ""
		for i, a := range g.Arguments {
			if i > 0 {
				args += "\n"
			}
			args += string(a)
		}
		grid.Items = append(grid.Items, w.Dim{Parts: []w.Inline{w.Text(gsArguments)}, Size: w.Dim11, Variant: w.DimTop},
			w.Area{Text: args, Label: gsArguments, Mono: true, Variant: w.AreaPaste},
			copyButton(view.Action{Label: copyWord, Enabled: shown}, args))
	}
	return grid
}

// baseName is the last part of a path: the name a file is saved as.
func baseName(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[i+1:]
		}
	}
	return p
}

// cardParts is a sentence of the model in its pieces as the text of a card.
func cardParts(r view.Runs) []w.CardTextPart {
	return as[w.CardTextPart](bind.Inlines(r))
}
