package jsluice

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
)

// Analyzer could be considered the core type of jsluice. It wraps
// the parse tree for a JavaScript file and provides mechanisms to
// extract URLs, secrets etc
type Analyzer struct {
	source      []byte
	parser      *sitter.Parser
	urlMatchers []URLMatcher
	rootNode    *Node
}

// NewAnalyzer accepts a slice of bytes representing some JavaScript
// source code and returns a pointer to a new Analyzer
func NewAnalyzer(source []byte) *Analyzer {
	parser := sitter.NewParser()
	parser.SetLanguage(javascript.GetLanguage())
	tree := parser.Parse(nil, source)

	return &Analyzer{
		source:      source,
		parser:      parser,
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
