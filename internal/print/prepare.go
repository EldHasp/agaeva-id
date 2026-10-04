package print

import (
	"strings"

	"golang.org/x/net/html"
)

const fsspBase = "https://is.fssp.gov.ru/"

// Prepare wraps the fragment the tab sent into a document that Chrome prints
// with the FSSP site's own stylesheets, screen colour, and the results block
// isolated on a landscape A4 page.
func Prepare(fragment string, stylesheetLinks []string) (string, error) {
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		return "", errEmpty
	}
	doc := parse(fragment)
	ensureSkeleton(doc)
	injectBaseAndStyles(doc, stylesheetLinks)
	if root := findResults(doc); root != nil && root.Data != "body" && root.Data != "html" {
		markRoot(root)
	}
	injectPrintCSS(doc, hasMarkedRoot(doc))
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n")
	if err := html.Render(&b, doc); err != nil {
		return "", err
	}
	return b.String(), nil
}

var errEmpty = errString("пустой фрагмент страницы")

type errString string

func (e errString) Error() string { return string(e) }

func parse(fragment string) *html.Node {
	root, err := html.Parse(strings.NewReader(fragment))
	if err != nil {
		root, _ = html.Parse(strings.NewReader("<html><head></head><body></body></html>"))
	}
	return root
}

func ensureSkeleton(doc *html.Node) {
	head := findTag(doc, "head")
	if head == nil {
		return
	}
	if !hasMetaCharset(head) {
		meta := &html.Node{
			Type: html.ElementNode,
			Data: "meta",
			Attr: []html.Attribute{{Key: "charset", Val: "utf-8"}},
		}
		head.InsertBefore(meta, head.FirstChild)
	}
}

func hasMetaCharset(head *html.Node) bool {
	for n := head.FirstChild; n != nil; n = n.NextSibling {
		if n.Type == html.ElementNode && n.Data == "meta" {
			for _, a := range n.Attr {
				if strings.EqualFold(a.Key, "charset") {
					return true
				}
			}
		}
	}
	return false
}

func injectBaseAndStyles(doc *html.Node, links []string) {
	head := findTag(doc, "head")
	if head == nil {
		return
	}
	if !hasBase(head) {
		base := &html.Node{
			Type: html.ElementNode,
			Data: "base",
			Attr: []html.Attribute{{Key: "href", Val: fsspBase}},
		}
		head.InsertBefore(base, head.FirstChild)
	}
	existing := map[string]bool{}
	for n := head.FirstChild; n != nil; n = n.NextSibling {
		if n.Type == html.ElementNode && n.Data == "link" && attr(n, "rel") == "stylesheet" {
			existing[attr(n, "href")] = true
		}
	}
	for _, href := range links {
		href = strings.TrimSpace(href)
		if href == "" || existing[href] {
			continue
		}
		existing[href] = true
		link := &html.Node{
			Type: html.ElementNode,
			Data: "link",
			Attr: []html.Attribute{
				{Key: "rel", Val: "stylesheet"},
				{Key: "href", Val: href},
			},
		}
		head.AppendChild(link)
	}
}

func hasBase(head *html.Node) bool {
	for n := head.FirstChild; n != nil; n = n.NextSibling {
		if n.Type == html.ElementNode && n.Data == "base" {
			return true
		}
	}
	return false
}

func injectPrintCSS(doc *html.Node, crop bool) {
	head := findTag(doc, "head")
	if head == nil {
		return
	}
	css := `@page { size: A4 landscape; margin: 8mm; }
html, body { background: #fff; }
* { -webkit-print-color-adjust: exact !important; print-color-adjust: exact !important; }
`
	if crop {
		css += `body * { visibility: hidden !important; }
#agaeva-print-root, #agaeva-print-root * { visibility: visible !important; }
#agaeva-print-root { position: absolute; left: 0; top: 0; width: 100%; }
`
	}
	style := &html.Node{Type: html.ElementNode, Data: "style"}
	style.AppendChild(&html.Node{Type: html.TextNode, Data: css})
	head.AppendChild(style)
}

func findResults(doc *html.Node) *html.Node {
	var best *html.Node
	bestScore := 0
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			score := scoreNode(n)
			if score > bestScore {
				bestScore = score
				best = n
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if bestScore < 8 {
		return nil
	}
	return best
}

func scoreNode(n *html.Node) int {
	id := strings.ToLower(attr(n, "id"))
	className := strings.ToLower(attr(n, "class"))
	blob := id + " " + className
	if strings.Contains(blob, "captcha") || strings.Contains(blob, "footer") || strings.Contains(blob, "header") || strings.Contains(blob, "menu") {
		return 0
	}
	score := 0
	if n.Data == "table" {
		score += 6
	}
	for _, hint := range []string{"results", "result", "search-result", "iss", "b-info", "list"} {
		if strings.Contains(blob, hint) {
			score += 12
		}
	}
	if id == "content" && containsTag(n, "table") {
		score += 10
	}
	if score > 0 && containsTag(n, "table") {
		score += 4
	}
	return score
}

func containsTag(n *html.Node, tag string) bool {
	if n.Type == html.ElementNode && n.Data == tag {
		return true
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if containsTag(c, tag) {
			return true
		}
	}
	return false
}

func markRoot(n *html.Node) {
	if attr(n, "id") == "" {
		n.Attr = append(n.Attr, html.Attribute{Key: "id", Val: "agaeva-print-root"})
		return
	}
	className := attr(n, "class")
	if !strings.Contains(className, "agaeva-print-root") {
		setAttr(n, "id", "agaeva-print-root")
	}
}

func hasMarkedRoot(doc *html.Node) bool {
	var found bool
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && attr(n, "id") == "agaeva-print-root" {
			found = true
			return
		}
		for c := n.FirstChild; c != nil && !found; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return found
}

func findTag(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findTag(c, tag); found != nil {
			return found
		}
	}
	return nil
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func setAttr(n *html.Node, key, val string) {
	for i, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			n.Attr[i].Val = val
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: val})
}

// HasStylesheet reports whether the prepared or original document already
// references a stylesheet or contains a style element.
func HasStylesheet(fragment string) bool {
	low := strings.ToLower(fragment)
	return strings.Contains(low, "rel=\"stylesheet\"") ||
		strings.Contains(low, "rel='stylesheet'") ||
		strings.Contains(low, "<style")
}
