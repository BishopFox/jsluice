package jsluice

import (
	"fmt"
	"strconv"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
)

// ExpressionPlaceholder is the string used to replace any
// expressions when string concatenations are collapsed. E.g:
//   "prefix" + someVar + "suffix"
// Would become:
//   prefixEXPRsuffix
var ExpressionPlaceholder = "EXPR"

// Node is a wrapper around a tree-sitter node. It serves as
// an attachment point for convenience methods, and also to
// store the raw JavaScript source that is a required argument
// for many tree-sitter functions.
type Node struct {
	node   *sitter.Node
	source []byte
}

// NewNode creates a new Node for the provided tree-sitter
// node and a byte-slice containing the JavaScript source.
// The source provided should be the complete source code
// and not just the source for the node in question.
func NewNode(n *sitter.Node, source []byte) *Node {
	return &Node{
		node:   n,
		source: source,
	}
}

// AsObject returns a Node as jsluice's internal object type,
// to allow the fetching of keys etc
func (n *Node) AsObject() Object {
	return NewObject(n, n.source)
}

// Content returns the source code for a particular node.
func (n *Node) Content() string {
	if n.node == nil {
		return ""
	}
	return n.node.Content(n.source)
}

// Type returns the tree-sitter type string for a Node.
// E.g. string, object, call_expression. If the node is
// nil then an empty string is returned.
func (n *Node) Type() string {
	if n.node == nil {
		return ""
	}
	return n.node.Type()
}

// Fetches a child Node from a named field. For example,
// the 'pair' node has two fields: key, and value.
func (n *Node) ChildByFieldName(name string) *Node {
	if !n.IsValid() {
		return nil
	}
	return NewNode(n.node.ChildByFieldName(name), n.source)
}

// NamedChild returns the 'named' child Node at the provided
// index. Tree-sitter considers a child to be named if it has
// a name in the syntax tree. Things like brackets are not named,
// but things like variables and function calls are named.
// See https://tree-sitter.github.io/tree-sitter/using-parsers#named-vs-anonymous-nodes
// for more details.
func (n *Node) NamedChild(index int) *Node {
	if !n.IsValid() {
		return nil
	}
	return NewNode(n.node.NamedChild(index), n.source)
}

// NamedChildCount returns the number of named children a Node has.
func (n *Node) NamedChildCount() int {
	if !n.IsValid() {
		return 0
	}
	return int(n.node.NamedChildCount())
}

// NamedChildren returns a slice of *Node containg all
// named children for a node.
func (n *Node) NamedChildren() []*Node {
	count := n.NamedChildCount()
	out := make([]*Node, 0, count)

	for i := 0; i < count; i++ {
		out = append(out, n.NamedChild(i))
	}

	return out
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
// The value of ExpressionPlaceholder is used as a placeholder, defaulting to 'EXPR'
func (n *Node) CollapsedString() string {
	if !n.IsValid() {
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
		return ExpressionPlaceholder
	}
}

// IsValid returns true if the *Node and the underlying
// tree-sitter node are both not nil.
func (n *Node) IsValid() bool {
	return n != nil && n.node != nil
}

// RawString returns the raw JavaScript representation
// of a string (i.e. escape sequences are left undecoded)
// but with the surrounding quotes removed.
func (n *Node) RawString() string {
	return dequote(n.Content())
}

// DecodedString returns a fully decoded version of a
// JavaScript string. It is just a convenience wrapper
// around the DecodeString function.
func (n *Node) DecodedString() string {
	return DecodeString(n.Content())
}

// AsGoType returns a representation of a Node as a native
// Go type, defaulting to a string containing the JavaScript
// source for the Node. Return types are:
//
//   string => string
//   number => int, float64
//   object => map[string]any
//   array  => []any
//   false  => false
//   true   => true
//   null   => nil
//   other  => string
//
func (n *Node) AsGoType() any {
	if n == nil {
		return nil
	}

	switch n.Type() {
	case "string":
		return n.DecodedString()
	case "number":
		return n.AsNumber()
	case "object":
		return n.AsMap()
	case "array":
		return n.AsArray()
	case "false":
		return false
	case "true":
		return true
	case "null":
		return nil
	default:
		return n.Content()
	}
}

// AsMap returns a representation of the Node as a map[string]any
func (n *Node) AsMap() map[string]any {
	if n.Type() != "object" {
		return map[string]any{}
	}

	pairs := n.NamedChildren()

	out := make(map[string]any, len(pairs))

	for _, pair := range pairs {
		if pair.Type() != "pair" {
			continue
		}

		key := DecodeString(pair.ChildByFieldName("key").RawString())
		value := pair.ChildByFieldName("value").AsGoType()

		out[key] = value
	}
	return out
}

// AsArray returns a representation of the Node as a []any
func (n *Node) AsArray() []any {
	if n.Type() != "array" {
		return []any{}
	}

	values := n.NamedChildren()

	out := make([]any, 0, len(values))

	for _, v := range values {
		out = append(out, v.AsGoType())
	}

	return out
}

// AsNumber returns a representation of the Node as an int or float64.
//
// Note: hex, octal etc number formats are currently unsupported
func (n *Node) AsNumber() any {
	if n.Type() != "number" {
		return 0
	}

	// TODO: handle hex, octal etc

	content := n.Content()
	if strings.Contains(content, ".") {
		// float
		f, err := strconv.ParseFloat(content, 64)
		if err != nil {
			return 0
		}
		return f
	}

	// int
	i, err := strconv.ParseInt(content, 10, 64)
	if err != nil {
		return 0
	}
	return i
}

// Parent returns the Parent Node for a Node
func (n *Node) Parent() *Node {
	if !n.IsValid() {
		return nil
	}
	return NewNode(n.node.Parent(), n.source)
}

// Query executes a tree-sitter query on a specific Node.
// Nodes matching the query are passed one at a time to the
// provided callback function.
//
// See https://tree-sitter.github.io/tree-sitter/using-parsers#pattern-matching-with-queries
// for query syntax documentation.
func (n *Node) Query(query string, fn func(*Node)) {
	if !n.IsValid() {
		return
	}
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

// IsStringy returns true if a Node is a string
// or is an expression starting with a string
// (e.g. a string concatenation expression).
func (n *Node) IsStringy() bool {
	if n.Type() == "string" {
		return true
	}

	c := n.Content()
	if len(c) == 0 {
		return false
	}

	switch c[0:1] {
	case `"`, "'", "`":
		return true
	default:
		return false
	}
}

// dequote removes surround quotes from the provided string
func dequote(in string) string {
	return strings.Trim(in, "'\"`")
}

// content returns the source for the provided tree-sitter
// node, checking if the node is nil first.
func content(n *sitter.Node, source []byte) string {
	if n == nil {
		return ""
	}
	return n.Content(source)
}

// PrintTree returns a string representation of the syntax tree
// for the provided JavaScript source
func PrintTree(source []byte) string {
	parser := sitter.NewParser()
	parser.SetLanguage(javascript.GetLanguage())

	tree := parser.Parse(nil, source)
	root := tree.RootNode()

	return getTree(root, source)
}

// getTree does the actual heavy lifting and recursion for PrintTree
// TODO: provide a way to print the tree as a JSON object?
func getTree(n *sitter.Node, source []byte) string {

	out := &strings.Builder{}

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
			fmt.Fprintf(out, "%s%s%s%s\n", strings.Repeat("  ", depth), fieldName, c.CurrentNode().Type(), contentStr)
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

	return strings.TrimSpace(out.String())
}
