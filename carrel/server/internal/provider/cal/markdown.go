package cal

import (
	"bytes"
	"html"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	gtext "github.com/yuin/goldmark/text"

	"carrel.dev/carrel/server/view"
)

// RenderMarkdown parses note Markdown into view blocks (C-6.9).
func RenderMarkdown(src string) []view.Block {
	if src == "" {
		return nil
	}
	srcBytes := []byte(src)
	md := goldmark.New()
	reader := gtext.NewReader(srcBytes)
	doc := md.Parser().Parse(reader)
	var out []view.Block
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		out = append(out, blockFromNode(n, srcBytes))
	}
	return out
}

func blockFromNode(n ast.Node, src []byte) view.Block {
	switch n.Kind() {
	case ast.KindHeading:
		h := n.(*ast.Heading)
		return view.Block{
			Kind:  view.BlockHeading,
			Level: h.Level,
			Text:  view.Text(textContent(n, src)),
		}
	case ast.KindParagraph:
		return view.Block{
			Kind: view.BlockPara,
			Text: view.Text(inlineText(n, src)),
		}
	case ast.KindFencedCodeBlock, ast.KindCodeBlock:
		return view.Block{
			Kind: view.BlockCode,
			Text: view.Text(textContent(n, src)),
		}
	case ast.KindBlockquote:
		var kids []view.Block
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			kids = append(kids, blockFromNode(c, src))
		}
		return view.Block{Kind: view.BlockQuote, Children: kids}
	case ast.KindHTMLBlock:
		hb := n.(*ast.HTMLBlock)
		var raw bytes.Buffer
		for i := 0; i < hb.Lines().Len(); i++ {
			seg := hb.Lines().At(i)
			raw.Write(seg.Value(src))
		}
		return view.Block{
			Kind: view.BlockPara,
			Text: view.Text(html.EscapeString(raw.String())),
		}
	case ast.KindList:
		var items []view.Block
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			items = append(items, blockFromNode(c, src))
		}
		return view.Block{Kind: view.BlockList, Children: items}
	case ast.KindListItem:
		return view.Block{
			Kind: view.BlockItem,
			Text: view.Text(inlineText(n, src)),
		}
	default:
		if s := textContent(n, src); s != "" {
			return view.Block{Kind: view.BlockPara, Text: view.Text(s)}
		}
		return view.Block{Kind: view.BlockPara}
	}
}

func textContent(n ast.Node, src []byte) string {
	var buf bytes.Buffer
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		buf.WriteString(segment(c, src))
	}
	return buf.String()
}

func inlineText(n ast.Node, src []byte) string {
	var buf bytes.Buffer
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		buf.WriteString(inlineSegment(c, src))
	}
	return buf.String()
}

func inlineSegment(n ast.Node, src []byte) string {
	switch n.Kind() {
	case ast.KindText:
		return string(n.(*ast.Text).Segment.Value(src))
	case ast.KindString:
		return string(n.(*ast.String).Value)
	case ast.KindEmphasis:
		return inlineText(n, src)
	case ast.KindLink:
		return inlineText(n, src)
	case ast.KindRawHTML:
		return html.EscapeString(string(n.(*ast.RawHTML).Segments.Value(src)))
	case ast.KindHTMLBlock:
		return html.EscapeString(segment(n, src))
	default:
		return segment(n, src)
	}
}

func segment(n ast.Node, src []byte) string {
	if t, ok := n.(*ast.Text); ok {
		return string(t.Segment.Value(src))
	}
	return inlineText(n, src)
}
