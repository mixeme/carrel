package guard

import (
	"regexp"
	"strings"
)

// Drawn is the printed HTML of a screen without what it does not show: an
// element with the attribute hidden (Typecase's Field.Hidden — the numbers
// «Custom…» of a reminder opens, s-agenda-19). A frame draws what the page
// shows: the frame of the reminders opened draws them, the frames of the
// edits (s-agenda-3, s-tasks-3) do not, and the screen prints them on every
// row (from-typecase note 51). Typecase's shape drops a hidden field of a form
// (input type=hidden) but not an element hidden by the attribute — Carrel's
// task 161 to Typecase; until the library drops it, the comparison here does.
func Drawn(html string) string {
	var b strings.Builder
	for {
		loc := hiddenOpen.FindStringSubmatchIndex(html)
		if loc == nil {
			b.WriteString(html)
			return b.String()
		}
		b.WriteString(html[:loc[0]])
		tag := html[loc[2]:loc[3]]
		end := strings.Index(html[loc[0]:], ">") + loc[0] + 1
		if voids[tag] {
			html = html[end:]
			continue
		}
		html = html[closeOf(html, tag, end):]
	}
}

// hiddenOpen is the opening tag of a field of a form with the attribute
// hidden (tc-field). Other hidden elements the frames draw as they will be
// shown — the line of Save that sends files (Runbar.Progress, s-tasks-3) —
// and stay.
var hiddenOpen = regexp.MustCompile(`<([a-z][a-z0-9]*) class="tc-field\b[^"]*"[^>]*?\shidden(?:[\s>=/])`)

// voids are the elements that have no closing tag.
var voids = map[string]bool{"input": true, "br": true, "hr": true, "img": true, "meta": true, "link": true}

// closeOf is where the element tag opened before from ends: after its
// closing tag, counting the same tags opened inside it; the end of the text
// when it is not closed.
func closeOf(html, tag string, from int) int {
	depth := 1
	open, shut := "<"+tag, "</"+tag+">"
	for at := from; at < len(html); {
		o := strings.Index(html[at:], open)
		c := strings.Index(html[at:], shut)
		if c < 0 {
			return len(html)
		}
		if o >= 0 && o < c {
			if next := at + o + len(open); next < len(html) && (html[next] == ' ' || html[next] == '>') {
				depth++
			}
			at += o + len(open)
			continue
		}
		depth--
		at += c + len(shut)
		if depth == 0 {
			return at
		}
	}
	return len(html)
}
