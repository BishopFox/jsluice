package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/BishopFox/jsluice"
)

func extractSecrets(opts options, filename string, source []byte, output chan string, errs chan error) {
	analyzer := jsluice.NewAnalyzer(source)

	// TODO: come up with a nice way to cache the patterns file and
	// only throw any open or parse errors once
	if opts.patternsFile != "" {
		f, err := os.Open(opts.patternsFile)
		if err != nil {
			errs <- err
			return
		}

		patterns, err := jsluice.ParseUserPatterns(f)
		if err != nil {
			errs <- err
			return
		}

		analyzer.AddSecretMatchers(patterns.SecretMatchers())
	}

	matches := analyzer.GetSecrets()
	for _, match := range matches {

		match.Filename = filename

		j, err := json.Marshal(match)
		if err != nil {
			continue
		}
		output <- fmt.Sprintf("%s", j)
	}
}
