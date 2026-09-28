// Constants a screen may hold: none of them is markup or a name of the
// library's space, and the last is excused by a row of the registry of
// screen specifics.
package good

import "carrel.dev/typecase/widget"

const (
	title = "Contacts"
	count = "5 of 7"
	less  = "a < b"    // «<» before a space is not a tag
	etc   = "etc-less" // «tc-» inside a word is not the library's prefix
	class = "classic"
)

// специфика экрана: U-5.7 #1
var excused = "<wbr>"

var _ = widget.Text(title + count + less + etc + class + excused)
