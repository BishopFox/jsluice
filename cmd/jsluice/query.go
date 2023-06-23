package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/BishopFox/jsluice"
)

func runQuery(opts options, filename string, source []byte, output chan string, errs chan error) {
	// TODO: add options to output nodes as trees and/or JSON blobs
	analyzer := jsluice.NewAnalyzer(source)

	buf := &strings.Builder{}
	enter := func(n *jsluice.Node) {
		content := n.Content()

		if opts.rawOutput {
			fmt.Fprintln(buf, content)
			return
		}

		out := n.AsGoType()

		b, err := json.Marshal(out)
		if err != nil {
			return
		}
		fmt.Fprintf(buf, "%s\n", b)
	}

	analyzer.Query(opts.query, enter)

	output <- strings.TrimSpace(buf.String())
}
