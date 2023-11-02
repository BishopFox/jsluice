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

	enter := func(qr jsluice.QueryResult) {
		vals := make(map[string]any)

		for k, n := range qr {
			vals[k] = n.Content()

			switch {
			case opts.format:
				f, err := n.Format()
				if err == nil {
					vals[k] = f
				}
			case !opts.rawOutput:
				vals[k] = n.AsGoType()
			}
		}

		if len(vals) == 0 {
			return
		}

		if opts.includeFilename {
			vals["filename"] = filename
		}

		var out any
		out = vals
		if len(vals) == 1 {
			for _, val := range vals {
				out = val
				break
			}
		}

		b, err := json.Marshal(out)
		if err != nil {
			return
		}
		fmt.Fprintf(buf, "%s\n", b)
	}

	analyzer.QueryMulti(opts.query, enter)

	output <- strings.TrimSpace(buf.String())
}
