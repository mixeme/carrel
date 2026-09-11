package widget

// Paragraph returns a Prose block with one paragraph.
func Paragraph(text string) Prose {
	return Prose{Blocks: []ProseBlock{{Kind: ProseP, Text: text}}}
}
