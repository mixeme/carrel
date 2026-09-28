// A screen that writes markup by hand. Every constant below is a breach the
// guard must name.
package bad

const row = `<li class="tc-row">` // markup, tc-, class=

var (
	closing = "</span>"                // markup
	opening = "<" + "div"              // a tag split over a concatenation
	angle   = '<'                      // a tag built rune by rune
	shaded  = "STYLE = color: red"     // style=
	opener  = "data-tc-open"           // data-tc
	verb    = "<%s>"                   // markup with a verb
	comment = "<!-- specific -->"      // markup
	named   = "tc-sheet"               // tc-
	quoted  = "\x3cp\x3e"              // markup behind escapes
	joined  = "open " + "class=" + "x" // class=
)
