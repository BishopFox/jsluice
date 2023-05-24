package main

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/BishopFox/jsluice"
)

func extractURLs(opts options, source []byte, output chan string, errs chan error) {

	var resolveURL *url.URL
	var err error
	if opts.resolvePaths != "" {
		resolveURL, err = url.Parse(opts.resolvePaths)
		if err != nil {
			errs <- err
			return
		}
	}

	analzyer := jsluice.NewAnalyzer(source)
	for _, m := range analzyer.GetURLs() {
		if opts.ignoreStrings && m.Type == "stringLiteral" {
			continue
		}

		// remove filename if the user doesn't want it
		if !opts.includeFilename {
			m.Filename = ""
		}

		// remove any souce if we don't want to display it
		if !opts.includeSource {
			m.Source = ""
		}

		if resolveURL != nil {
			parsed, err := url.Parse(m.URL)
			if err == nil {
				m.URL = resolveURL.ResolveReference(parsed).String()
			}
		}

		j, err := json.Marshal(m)
		if err != nil {
			errs <- err
			continue
		}
		output <- fmt.Sprintf("%s", j)
	}
}
