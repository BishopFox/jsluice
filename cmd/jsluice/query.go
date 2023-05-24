package main

import (
	"fmt"
	"strings"

	"github.com/BishopFox/jsluice"
)

func runQuery(opts options, filename string, source []byte, output chan string, errs chan error) {
	// TODO: add options to output nodes as trees and/or JSON blobs
	analyzer := jsluice.NewAnalyzer(source)

	buf := &strings.Builder{}
	enter := func(n *jsluice.Node) {
		fmt.Fprintln(buf, n.Content())
	}

	analyzer.Query(opts.query, enter)

	output <- strings.TrimSpace(buf.String())
}
