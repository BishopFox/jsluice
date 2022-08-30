package jsluice

import (
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
)

// nil-safe wrapper around calling node.Content(source)
func content(n *sitter.Node, source []byte) string {
	if n == nil {
		return ""
	}
	return n.Content(source)
}

func isStringy(n *sitter.Node, source []byte) bool {
	if n.Type() == "string" {
		return true
	}

	c := content(n, source)
	if len(c) == 0 {
		return false
	}

	switch c[0:0] {
	case `"`, "'", "`":
		return true
	default:
		return false
	}
}

func hasDescendantOfType(n *sitter.Node, t string) bool {
	if n == nil {
		return false
	}

	// node is provided type exactly
	if n.Type() == t {
		return true
	}

	hasType := false
	enter := func(n *sitter.Node) {
		if n.Type() == t {
			hasType = true
		}
	}

	walk(n, enter)
	return hasType
}

// cleanURL takes a node representing a URL and attempts to make it
// at least somewhat easily parseable. It's common to build URLs out
// of variables and function calls so we want to turn something like:
//
//  './upload.php?profile='+res.id+'&show='+$('.participate_modal_container').attr('data-val')
//
// Into something more like:
//
//  ./upload.php?profile=EXPR&show=EXPR
//
func cleanURL(n *sitter.Node, source []byte) string {
	if n == nil {
		return ""
	}
	switch n.Type() {
	case "binary_expression":
		return fmt.Sprintf(
			"%s%s",
			cleanURL(n.ChildByFieldName("left"), source),
			cleanURL(n.ChildByFieldName("right"), source),
		)
	case "string":
		return dequote(content(n, source))
	default:
		return "EXPR"
	}
}

func dequote(in string) string {
	return strings.Trim(in, "'\"`")
}

func query(n *sitter.Node, query string, enter func(*sitter.Node)) {
	q, err := sitter.NewQuery(
		[]byte(query),
		javascript.GetLanguage(),
	)
	if err != nil {
		return
	}

	qc := sitter.NewQueryCursor()
	defer qc.Close()

	qc.Exec(q, n)

	for {
		match, exists := qc.NextMatch()
		if !exists || match == nil {
			break
		}

		for _, capture := range match.Captures {
			enter(capture.Node)
		}
	}
}

func walk(n *sitter.Node, enter func(*sitter.Node)) {

	c := sitter.NewTreeCursor(n)
	defer c.Close()

	// walkies
	recurse := true
	for {
		// descend into the tree
		if recurse && c.GoToFirstChild() {
			recurse = true
			enter(c.CurrentNode())
			continue
		}

		// move sideways
		if c.GoToNextSibling() {
			recurse = true
			enter(c.CurrentNode())
			continue
		}

		// climb back up the tree, but make sure we don't descend right back to where we were
		if c.GoToParent() {
			recurse = false
			continue
		}
		break
	}

}

func PrintTree(source []byte) {
	parser := sitter.NewParser()
	parser.SetLanguage(javascript.GetLanguage())

	tree := parser.Parse(nil, source)
	root := tree.RootNode()

	PrettyPrint(root, source)
}

func PrettyPrint(n *sitter.Node, source []byte) {

	c := sitter.NewTreeCursor(n)
	defer c.Close()

	// walkies
	depth := 0
	recurse := true
	for {
		if recurse && c.CurrentNode().IsNamed() {
			fieldName := c.CurrentFieldName()
			if fieldName != "" {
				fieldName += ": "
			}

			contentStr := ""
			if c.CurrentNode().ChildCount() == 0 || c.CurrentNode().Type() == "string" {
				contentStr = fmt.Sprintf(" (%s)", content(c.CurrentNode(), source))
			}
			fmt.Printf("%s%s%s%s\n", strings.Repeat("  ", depth), fieldName, c.CurrentNode().Type(), contentStr)
		}

		// descend into the tree
		if recurse && c.GoToFirstChild() {
			recurse = true
			depth++
			continue
		}

		// move sideways
		if c.GoToNextSibling() {
			recurse = true
			continue
		}

		// climb back up the tree, but make sure we don't descend right back to where we were
		if c.GoToParent() {
			depth--
			recurse = false
			continue
		}
		break
	}

}
