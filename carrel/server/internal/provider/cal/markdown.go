package cal

import (
	"bytes"
	"html"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	gtext "github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"carrel.dev/carrel/server/view"
)

// RenderMarkdown parses note Markdown into view blocks (C-6.9). A task item
// («- [ ]», «- [x]») is a task with its tick (s-notes-1, s-notes-2), the way
// jtx Board and people write them; a GFM table is a table of rows and cells
// (s-notes-2 lists tables among what a note shows).
func RenderMarkdown(src string) []view.Block {
	if src == "" {
		return nil
	}
	srcBytes := []byte(src)
	md := goldmark.New(goldmark.WithExtensions(extension.TaskList, extension.Table))
	reader := gtext.NewReader(srcBytes)
	doc := md.Parser().Parse(reader)
	var out []view.Block
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		if n.Kind() == ast.KindThematicBreak {
			continue
		}
		out = append(out, blockFromNode(n, srcBytes))
	}
	return out
}

func blockFromNode(n ast.Node, src []byte) view.Block {
	switch n.Kind() {
	case ast.KindHeading:
		h := n.(*ast.Heading)
		b := withInlines(view.BlockHeading, n, src)
		b.Level = h.Level
		return b
	case ast.KindParagraph, ast.KindTextBlock:
		return withInlines(view.BlockPara, n, src)
	case ast.KindFencedCodeBlock, ast.KindCodeBlock:
		return view.Block{
			Kind: view.BlockCode,
			Text: view.Text(lineText(n, src)),
		}
	case ast.KindBlockquote:
		var kids []view.Block
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			kids = append(kids, blockFromNode(c, src))
		}
		return view.Block{Kind: view.BlockQuote, Children: kids}
	case ast.KindHTMLBlock:
		return view.Block{
			Kind: view.BlockPara,
			Text: view.Text(html.EscapeString(lineText(n, src))),
		}
	case ast.KindList:
		var items []view.Block
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			items = append(items, blockFromNode(c, src))
		}
		return view.Block{Kind: view.BlockList, Children: items}
	case ast.KindListItem:
		var kids []view.Block
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			kids = append(kids, blockFromNode(c, src))
		}
		kind := view.BlockItem
		checked, task := taskOf(n)
		if task {
			kind = view.BlockTask
		}
		if len(kids) == 1 && kids[0].Kind == view.BlockPara {
			item := kids[0]
			item.Kind, item.Checked = kind, checked
			return item
		}
		return view.Block{Kind: kind, Checked: checked, Children: kids}
	case extast.KindTable:
		var rows []view.Block
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			rows = append(rows, blockFromNode(c, src))
		}
		return view.Block{Kind: view.BlockTable, Children: rows}
	case extast.KindTableHeader, extast.KindTableRow:
		var cells []view.Block
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			cells = append(cells, blockFromNode(c, src))
		}
		return view.Block{Kind: view.BlockRow, Header: n.Kind() == extast.KindTableHeader, Children: cells}
	case extast.KindTableCell:
		b := withInlines(view.BlockCell, n, src)
		b.Align = cellAlign[n.(*extast.TableCell).Alignment]
		return b
	default:
		if s := textContent(n, src); s != "" {
			return view.Block{Kind: view.BlockPara, Text: view.Text(s)}
		}
		return view.Block{Kind: view.BlockPara}
	}
}

// cellAlign is a column's alignment from the table's delimiter row; a
// column without one has none.
var cellAlign = map[extast.Alignment]view.CellAlign{
	extast.AlignLeft:   view.AlignLeft,
	extast.AlignCenter: view.AlignCenter,
	extast.AlignRight:  view.AlignRight,
}

// taskOf reports a list item that opens with a tick, and whether it is
// ticked. The tick is the first inline of the item's first block; the text
// after it is the item's text.
func taskOf(item ast.Node) (checked, task bool) {
	first := item.FirstChild()
	if first == nil {
		return false, false
	}
	box, ok := first.FirstChild().(*extast.TaskCheckBox)
	if !ok {
		return false, false
	}
	return box.IsChecked, true
}

func withInlines(kind view.BlockKind, n ast.Node, src []byte) view.Block {
	kids := inlines(n, src)
	b := view.Block{Kind: kind}
	if len(kids) == 1 && kids[0].Kind == view.BlockPara && len(kids[0].Children) == 0 {
		b.Text = kids[0].Text
		return b
	}
	b.Children = kids
	return b
}

func inlines(n ast.Node, src []byte) []view.Block {
	var out []view.Block
	var text bytes.Buffer
	flush := func() {
		if text.Len() == 0 {
			return
		}
		out = append(out, view.Block{Kind: view.BlockPara, Text: view.Text(text.String())})
		text.Reset()
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch c.Kind() {
		case ast.KindText:
			t := c.(*ast.Text)
			text.Write(textValue(t, src))
			if t.HardLineBreak() || t.SoftLineBreak() {
				text.WriteByte('\n')
			}
		case ast.KindString:
			text.Write(c.(*ast.String).Value)
		case ast.KindEmphasis:
			flush()
			em := c.(*ast.Emphasis)
			kind := view.BlockEmph
			if em.Level >= 2 {
				kind = view.BlockStrong
			}
			out = append(out, inlineMark(kind, c, src, ""))
		case ast.KindLink:
			flush()
			link := c.(*ast.Link)
			out = append(out, inlineMark(view.BlockLink, c, src, string(link.Destination)))
		case ast.KindAutoLink:
			flush()
			al := c.(*ast.AutoLink)
			u := string(al.URL(src))
			out = append(out, view.Block{Kind: view.BlockLink, Href: u, Text: view.Text(u)})
		case ast.KindCodeSpan:
			flush()
			out = append(out, view.Block{Kind: view.BlockCode, Text: view.Text(textContent(c, src))})
		case ast.KindRawHTML:
			flush()
			raw := string(c.(*ast.RawHTML).Segments.Value(src))
			out = append(out, view.Block{Kind: view.BlockPara, Text: view.Text(html.EscapeString(raw))})
		default:
			if s := textContent(c, src); s != "" {
				text.WriteString(s)
			}
		}
	}
	flush()
	return out
}

func inlineMark(kind view.BlockKind, n ast.Node, src []byte, href string) view.Block {
	b := withInlines(kind, n, src)
	b.Href = href
	return b
}

func lineText(n ast.Node, src []byte) string {
	var buf bytes.Buffer
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		buf.Write(seg.Value(src))
	}
	return buf.String()
}

func textContent(n ast.Node, src []byte) string {
	var buf bytes.Buffer
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch c.Kind() {
		case ast.KindText:
			buf.Write(textValue(c.(*ast.Text), src))
		case ast.KindString:
			buf.Write(c.(*ast.String).Value)
		default:
			buf.WriteString(textContent(c, src))
		}
	}
	return buf.String()
}

// textValue is what a text shows: a backslash escape («\*», «\|» in a
// table cell) is the mark itself, as goldmark's own renderer writes it. The
// text of a code span is raw and keeps its backslashes as written.
func textValue(t *ast.Text, src []byte) []byte {
	v := t.Segment.Value(src)
	if t.IsRaw() {
		return v
	}
	return util.UnescapePunctuations(v)
}
