package jsluice

import (
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
)

type Node struct {
	node   *sitter.Node
	source []byte
}

func NewNode(n *sitter.Node, source []byte) *Node {
	return &Node{
		node:   n,
		source: source,
	}
}

func (n *Node) Content() string {
	if n.node == nil {
		return ""
	}
	return n.node.Content(n.source)
}

func (n *Node) Type() string {
	if n.node == nil {
		return ""
	}
	return n.node.Type()
}

func (n *Node) ChildByFieldName(name string) *Node {
	return NewNode(n.node.ChildByFieldName(name), n.source)
}

func (n *Node) NamedChild(index int) *Node {
	return NewNode(n.node.NamedChild(0), n.source)
}

func (n *Node) NamedChildCount() int {
	return int(n.node.NamedChildCount())
}

// CollapsedString takes a node representing a URL and attempts to make it
// at least somewhat easily parseable. It's common to build URLs out
// of variables and function calls so we want to turn something like:
//
//  './upload.php?profile='+res.id+'&show='+$('.participate_modal_container').attr('data-val')
//
// Into something more like:
//
//  ./upload.php?profile=EXPR&show=EXPR
//
func (n *Node) CollapsedString() string {
	if n.node == nil {
		return ""
	}
	switch n.Type() {
	case "binary_expression":
		return fmt.Sprintf(
			"%s%s",
			n.ChildByFieldName("left").CollapsedString(),
			n.ChildByFieldName("right").CollapsedString(),
		)
	case "string":
		return n.RawString()
	default:
		return "EXPR"
	}
}

func (n *Node) RawString() string {
	return dequote(n.Content())
}

func (n *Node) Parent() *Node {
	return NewNode(n.node.Parent(), n.source)
}

func (n *Node) Query(query string, fn func(*Node)) {
	q, err := sitter.NewQuery(
		[]byte(query),
		javascript.GetLanguage(),
	)
	if err != nil {
		return
	}

	qc := sitter.NewQueryCursor()
	defer qc.Close()

	qc.Exec(q, n.node)

	for {
		match, exists := qc.NextMatch()
		if !exists || match == nil {
			break
		}

		for _, capture := range match.Captures {
			fn(NewNode(capture.Node, n.source))
		}
	}
}

func (n *Node) IsStringy() bool {
	if n.Type() == "string" {
		return true
	}

	c := n.Content()
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

func dequote(in string) string {
	return strings.Trim(in, "'\"`")
}

func content(n *sitter.Node, source []byte) string {
	if n == nil {
		return ""
	}
	return n.Content(source)
}

func PrintTree(source []byte) {
	parser := sitter.NewParser()
	parser.SetLanguage(javascript.GetLanguage())

	tree := parser.Parse(nil, source)
	root := tree.RootNode()

	prettyPrint(root, source)
}

func prettyPrint(n *sitter.Node, source []byte) {

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
