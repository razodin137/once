package htmltext

import (
	"fmt"
	"io"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// Render turns an HTML document into plain text, keeping headings, links and
// form controls while dropping the markup.
func Render(r io.Reader, base *url.URL) (string, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return "", fmt.Errorf("parsing html: %w", err)
	}

	renderer := &renderer{base: base}
	renderer.walkChildren(doc)

	return renderer.string(), nil
}

type renderer struct {
	base *url.URL

	buf        strings.Builder
	line       strings.Builder
	lineSet    bool
	lineIndent int
	indent     int
	spaced     bool
	preserve   bool
	cells      int
	lists      []listState
}

type listState struct {
	ordered bool
	index   int
}

// Private

func (r *renderer) string() string {
	r.flush()
	return strings.TrimLeft(r.buf.String(), "\n")
}

func (r *renderer) walk(n *html.Node) {
	switch n.Type {
	case html.TextNode:
		r.text(n.Data)
	case html.ElementNode:
		r.element(n)
	default:
		r.walkChildren(n)
	}
}

func (r *renderer) walkChildren(n *html.Node) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		r.walk(child)
	}
}

func (r *renderer) element(n *html.Node) {
	switch n.Data {
	case "head", "script", "style", "noscript", "template", "svg", "canvas", "iframe", "object", "audio", "video":
		return
	case "br":
		r.flush()
	case "hr":
		r.block()
	case "h1", "h2", "h3", "h4", "h5", "h6":
		r.block()
		r.walkChildren(n)
		r.block()
	case "p", "div", "section", "article", "header", "footer", "main", "nav", "aside",
		"figure", "figcaption", "details", "summary", "address", "dl", "dt", "dd":
		r.block()
		r.walkChildren(n)
		r.block()
	case "blockquote", "fieldset":
		r.block()
		r.indent++
		r.walkChildren(n)
		r.indent--
		r.block()
	case "pre":
		r.block()
		r.preserve = true
		r.walkChildren(n)
		r.flush()
		r.preserve = false
		r.block()
	case "ul", "ol":
		r.list(n)
	case "li":
		r.item(n)
	case "table":
		r.block()
		r.walkChildren(n)
		r.block()
	case "tr":
		r.cells = 0
		r.flush()
		r.walkChildren(n)
		r.flush()
	case "td", "th":
		if r.cells > 0 {
			r.write(" | ")
		}
		r.cells++
		r.walkChildren(n)
	case "a":
		r.link(n)
	case "img":
		r.image(n)
	case "form":
		r.form(n)
	case "label":
		r.flush()
		r.walkChildren(n)
		r.flush()
	case "input":
		r.control(n)
	case "textarea":
		r.write(named("textarea", attr(n, "name")))
		r.flush()
	case "select":
		r.selectControl(n)
	case "button":
		r.write(named("button", plainText(n)))
		r.flush()
	default:
		r.walkChildren(n)
	}
}

func (r *renderer) text(s string) {
	if !r.preserve {
		r.write(collapsed(s))
		return
	}

	for i, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		if i > 0 {
			r.flush()
		}
		r.write(line)
	}
}

func (r *renderer) list(n *html.Node) {
	r.flush()
	r.lists = append(r.lists, listState{ordered: n.Data == "ol"})
	r.walkChildren(n)
	r.lists = r.lists[:len(r.lists)-1]
	r.block()
}

func (r *renderer) item(n *html.Node) {
	r.flush()
	r.write(r.marker())
	r.indent++
	r.walkChildren(n)
	r.flush()
	r.indent--
}

func (r *renderer) marker() string {
	if len(r.lists) == 0 {
		return "- "
	}

	list := &r.lists[len(r.lists)-1]
	if !list.ordered {
		return "- "
	}

	list.index++
	return fmt.Sprintf("%d. ", list.index)
}

func (r *renderer) link(n *html.Node) {
	label := plainText(n)
	href := strings.TrimSpace(attr(n, "href"))

	if href == "" || strings.HasPrefix(href, "javascript:") {
		r.inline(label)
		return
	}

	target := r.resolve(href)
	switch label {
	case "":
		r.inline("<" + target + ">")
	case target:
		r.inline(target)
	default:
		r.inline(label + " <" + target + ">")
	}
}

func (r *renderer) image(n *html.Node) {
	if alt := plainTextAttr(n, "alt"); alt != "" {
		r.inline("[" + alt + "]")
	}
}

func (r *renderer) inline(s string) {
	if s == "" {
		return
	}

	if needsSpaceBefore(r.line.String()) {
		r.write(" ")
	}
	r.write(s)
}

func (r *renderer) form(n *html.Node) {
	r.block()
	r.write("form " + formMethod(n) + " " + r.formAction(n))
	r.flush()

	r.indent++
	r.walkChildren(n)
	r.flush()
	r.indent--

	r.block()
}

func (r *renderer) control(n *html.Node) {
	kind := strings.ToLower(attr(n, "type"))
	if kind == "" {
		kind = "text"
	}

	switch kind {
	case "hidden":
		return
	case "submit", "reset", "button", "image":
		label := attr(n, "value")
		if label == "" {
			label = strings.ToUpper(kind[:1]) + kind[1:]
		}
		r.write(named("button", label))
	case "checkbox", "radio":
		mark := "[ ]"
		if hasAttr(n, "checked") {
			mark = "[x]"
		}
		r.write(named(mark, attr(n, "name")))
	case "text":
		r.write(named("input", attr(n, "name")))
	default:
		r.write(named("input", attr(n, "name")) + " (" + kind + ")")
	}

	if !within(n, "label") {
		r.flush()
	}
}

func (r *renderer) selectControl(n *html.Node) {
	options := make([]string, 0, 4)
	collectOptions(n, &options)

	control := named("select", attr(n, "name"))
	if len(options) > 0 {
		control += " (" + strings.Join(options, ", ") + ")"
	}

	r.write(control)
	r.flush()
}

func (r *renderer) write(s string) {
	if !r.lineSet {
		r.lineSet = true
		r.lineIndent = r.indent
	}
	r.line.WriteString(s)
}

func (r *renderer) flush() {
	if !r.lineSet {
		return
	}

	var line string
	if r.preserve {
		line = strings.TrimRight(r.line.String(), " \t")
	} else {
		line = strings.Join(strings.Fields(r.line.String()), " ")
	}

	r.line.Reset()
	r.lineSet = false

	if line == "" {
		return
	}

	if r.spaced && r.buf.Len() > 0 {
		r.buf.WriteByte('\n')
	}
	r.spaced = false

	r.buf.WriteString(strings.Repeat("  ", r.lineIndent))
	r.buf.WriteString(line)
	r.buf.WriteByte('\n')
}

func (r *renderer) block() {
	r.flush()
	r.spaced = true
}

func (r *renderer) resolve(href string) string {
	if r.base == nil || strings.HasPrefix(href, "#") {
		return href
	}

	parsed, err := url.Parse(href)
	if err != nil {
		return href
	}

	return r.base.ResolveReference(parsed).String()
}

func (r *renderer) formAction(n *html.Node) string {
	action := strings.TrimSpace(attr(n, "action"))
	if action == "" {
		if r.base == nil {
			return ""
		}
		return r.base.String()
	}

	return r.resolve(action)
}

// Helpers

func needsSpaceBefore(line string) bool {
	if line == "" {
		return false
	}

	last, _ := utf8.DecodeLastRuneInString(line)
	if unicode.IsSpace(last) {
		return false
	}

	return unicode.IsLetter(last) || unicode.IsDigit(last) || strings.ContainsRune(".,;:!?)]}>\"'", last)
}

func formMethod(n *html.Node) string {
	method := strings.ToUpper(strings.TrimSpace(attr(n, "method")))
	if method == "" {
		return "GET"
	}
	return method
}

func collectOptions(n *html.Node, options *[]string) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}

		switch child.Data {
		case "option":
			if label := plainText(child); label != "" {
				*options = append(*options, label)
			}
		case "optgroup":
			collectOptions(child, options)
		}
	}
}

func collapsed(s string) string {
	body := strings.Join(strings.Fields(s), " ")
	if body == "" {
		if s != "" {
			return " "
		}
		return ""
	}

	if startsWithSpace(s) {
		body = " " + body
	}
	if endsWithSpace(s) {
		body += " "
	}

	return body
}

func startsWithSpace(s string) bool {
	return s != "" && strings.TrimLeft(s, " \t\r\n\f") != s
}

func endsWithSpace(s string) bool {
	return s != "" && strings.TrimRight(s, " \t\r\n\f") != s
}

func plainText(n *html.Node) string {
	var text strings.Builder
	collectText(n, &text)
	return strings.Join(strings.Fields(text.String()), " ")
}

func plainTextAttr(n *html.Node, name string) string {
	return strings.Join(strings.Fields(attr(n, name)), " ")
}

func collectText(n *html.Node, text *strings.Builder) {
	switch n.Type {
	case html.TextNode:
		text.WriteString(" " + n.Data)
		return
	case html.ElementNode:
		switch n.Data {
		case "script", "style":
			return
		case "img":
			text.WriteString(" " + attr(n, "alt"))
			return
		case "br":
			text.WriteString(" ")
			return
		}
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		collectText(child, text)
	}
}

func attr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

func hasAttr(n *html.Node, name string) bool {
	for _, a := range n.Attr {
		if a.Key == name {
			return true
		}
	}
	return false
}

func within(n *html.Node, name string) bool {
	for parent := n.Parent; parent != nil; parent = parent.Parent {
		if parent.Type == html.ElementNode && parent.Data == name {
			return true
		}
	}
	return false
}

func quoted(s string) string {
	return "\"" + s + "\""
}

func named(keyword, name string) string {
	if name == "" {
		return keyword
	}
	return keyword + " " + quoted(name)
}
