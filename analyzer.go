package jsluice

import (
	"bytes"
	"github.com/PuerkitoBio/goquery"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
)

// Analyzer could be considered the core type of jsluice. It wraps
// the parse tree for a JavaScript file and provides mechanisms to
// extract URLs, secrets etc
type Analyzer struct {
	urlMatchers        []URLMatcher
	rootNode           *Node
	userSecretMatchers []SecretMatcher
}

// NewAnalyzer accepts a slice of bytes representing some JavaScript
// source code and returns a pointer to a new Analyzer
func NewAnalyzer(source []byte) *Analyzer {
	// If the source is HTML, parse out the inline JavaScript
	parser := sitter.NewParser()

	parser.SetLanguage(javascript.GetLanguage())

	tree := parser.Parse(nil, source)
	// dump tree
	if tree.RootNode().HasError() {
		source = ParseInline(source)
		tree = parser.Parse(nil, source)
	}

	// TODO: Align how URLMatcher and SecretMatcher slices
	// are loaded. At the moment we load URLMatchers now,
	// and SecretMatchers only when GetSecrets is called.
	// This is mostly because URL matching was written first,
	// and then secret matching was added later.
	return &Analyzer{
		urlMatchers: AllURLMatchers(),
		rootNode:    NewNode(tree.RootNode(), source),
	}
}

// Query peforms a tree-sitter query on the JavaScript being analyzed.
// The provided function is called for every node that matches the query.
// See https://tree-sitter.github.io/tree-sitter/using-parsers#query-syntax
// for details on query syntax.
func (a *Analyzer) Query(q string, fn func(*Node)) {
	a.rootNode.Query(q, fn)
}

// ParseInline should be used to parse inline JavaScript in HTML pages.
// The provided function is called passing a byte[] source and should
// return a slice of bytes containing only the inline JavaScript.
// This is done through the use of the goquery library.
func ParseInline(source []byte) []byte {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(source))
	if err != nil {
		// Not a valid HTML document, so just return the source.
		return source
	}

	var inline []byte
	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		if s.Is("script") {
			inline = append(inline, []byte(s.Text()+"\n")...)
		}
	})
	if len(inline) == 0 {
		return source
	}
	return inline
}
