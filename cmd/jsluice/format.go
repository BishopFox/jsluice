package main

import (
	"github.com/BishopFox/jsluice"
)

func format(opts options, filename string, source []byte, output chan string, errs chan error) {

	analyzer := jsluice.NewAnalyzer(source)

	formatted, err := analyzer.RootNode().Format()
	if err != nil {
		errs <- err
		return
	}

	output <- formatted
}
