package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"regexp"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
)

func main() {

	reWhitespace := regexp.MustCompile(`\s{2,}`)
	reJSName := regexp.MustCompile(`^[a-zA-Z0-9_$.-]+$`)

	flag.Parse()
	source, err := ioutil.ReadFile(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}

	parser := sitter.NewParser()
	parser.SetLanguage(javascript.GetLanguage())

	tree := parser.Parse(nil, source)
	root := tree.RootNode()

	enter := func(n *sitter.Node) {
		switch n.Type() {
		case "assignment_expression":
			left := n.ChildByFieldName("left")
			right := n.ChildByFieldName("right")
			if left == nil || right == nil {
				return
			}

			rightContent := right.Content(source)
			if !startsWithString(rightContent) {
				return
			}

			rightContent = reWhitespace.ReplaceAllString(rightContent, " ")
			rightStr := dequote(right.Content(source))

			if couldBePath(rightStr) {
				fmt.Printf("%s (assignment)\n", left.Content(source))
			}

		case "call_expression":
			callName := n.ChildByFieldName("function").Content(source)
			// It's common to find things like immediately called anonymous functions
			// in JS source, and we don't care about those because we could never match
			// on them
			if !reJSName.MatchString(callName) {
				return
			}

			arguments := n.ChildByFieldName("arguments")
			if arguments == nil {
				return
			}

			// we want to iterate over the arguments and find
			// any that look like a url
			c := sitter.NewTreeCursor(arguments)
			defer c.Close()

			// no args
			if !c.GoToFirstChild() {
				return
			}

			foundPath := false
			position := 0
			for {
				arg := c.CurrentNode()
				if arg == nil {
					break
				}

				// named args only (i.e. don't count commas etc)
				if arg.IsNamed() {

					argContent := arg.Content(source)
					if startsWithString(argContent) && couldBePath(dequote(argContent)) {
						foundPath = true
						break
					}
					position++
				}

				if !c.GoToNextSibling() {
					break
				}
			}

			if foundPath {
				fmt.Printf("%s (arg %d)\n", callName, position)
			}
		}
	}
	queryNodes(root, enter)
}

func startsWithString(in string) bool {
	if len(in) < 2 {
		return false
	}

	p := in[0:1]
	if p == `"` || p == "'" || p == "`" {
		return true
	}

	return false
}

func couldBePath(in string) bool {

	if (strings.HasPrefix(in, "http:") && len(in) > 7) ||
		(strings.HasPrefix(in, "https:") && len(in) > 8) ||
		(strings.HasPrefix(in, "/") && len(in) > 3) ||
		(strings.HasPrefix(in, "./") && len(in) > 4) {
		return true
	}

	return false
}

func queryNodes(n *sitter.Node, enter func(*sitter.Node)) {

	query, err := sitter.NewQuery(
		[]byte("[(assignment_expression) (call_expression)] @matches"),
		javascript.GetLanguage(),
	)
	if err != nil {
		log.Fatal(err)
	}
	qc := sitter.NewQueryCursor()
	defer qc.Close()

	qc.Exec(query, n)

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

func dequote(in string) string {
	return strings.Trim(in, "'\"`")
}
