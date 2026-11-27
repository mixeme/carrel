package bind

import (
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// MailTrace is a mailbox's trace as one block (s-mailbox-25): a line a
// step, its name in brackets, what was asked and answered after it; a step
// of several lines — a line a command — goes on under its first, in the
// same column.
func MailTrace(steps []view.TraceStep) w.Diag {
	width := 0
	for _, s := range steps {
		width = max(width, len([]rune(string(s.Name)))+2)
	}
	pad := func(s string) string {
		for n := len([]rune(s)); n < width+2; n++ {
			s += " "
		}
		return s
	}
	text := ""
	for i, s := range steps {
		if i > 0 {
			text += "\n"
		}
		text += pad("[" + string(s.Name) + "]")
		for _, r := range string(s.Detail) {
			if r == '\n' {
				text += "\n" + pad("")
				continue
			}
			text += string(r)
		}
	}
	return w.Diag{Text: text}
}
