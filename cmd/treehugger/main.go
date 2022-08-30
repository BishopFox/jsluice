package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
)

func main() {

	flag.Parse()
	queryStr := flag.Arg(0)

	parser := sitter.NewParser()
	parser.SetLanguage(javascript.GetLanguage())

	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		source, err := ioutil.ReadFile(sc.Text())
		if err != nil {
			fmt.Fprintf(os.Stderr, "error opening file: %s\n", err)
			continue
		}

		enter := func(n *sitter.Node) {
			content := n.Content(source)
			fmt.Println(content)
		}

		tree := parser.Parse(nil, source)
		root := tree.RootNode()

		query(root, queryStr, enter)
	}
}

func query(n *sitter.Node, queryStr string, enter func(*sitter.Node)) {

	q, err := sitter.NewQuery([]byte(queryStr), javascript.GetLanguage())
	if err != nil {
		log.Fatal(err)
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
