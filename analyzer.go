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
	rootNode    *sitter.Node
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
		rootNode:    tree.RootNode(),
	}
}
