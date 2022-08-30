package main

// Extract URLs and related stuff out of JavaScript files

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/bishopfoxmss/jsluice"
	"github.com/pkg/profile"
	flag "github.com/spf13/pflag"
)

func main() {
	var treeMode bool
	flag.BoolVarP(&treeMode, "tree", "t", false, "Just print the tree for the provided file")

	var includeSource bool
	flag.BoolVarP(&includeSource, "include-source", "i", false, "Include the source code where the URL was found")

	var ignoreStrings bool
	flag.BoolVar(&ignoreStrings, "ignore-strings", false, "Ignore matches from string literals")

	var includeFilename bool
	flag.BoolVar(&includeFilename, "include-filename", false, "Include the filename of the matched file in the output")

	var profileMode bool
	flag.BoolVar(&profileMode, "profile", false, "Profile cpu usage and save a cpu.pprof file in the current dir")

	var concurrency int
	flag.IntVarP(&concurrency, "concurrency", "c", 1, "Number of files to process concurrently")

	var resolve string
	flag.StringVarP(&resolve, "resolve", "r", "", "Resolve relative paths using the absolute URL provided")

	flag.Parse()

	if profileMode {
		defer profile.Start(profile.ProfilePath(".")).Stop()
	}

	var resolveURL *url.URL
	var err error
	if resolve != "" {
		resolveURL, err = url.Parse(resolve)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse resolve URL: %s\n", err)
			return
		}
	}

	var input io.Reader = os.Stdin
	if flag.Arg(0) != "" {
		input = strings.NewReader(
			strings.Join(flag.Args(), "\n"),
		)
	}

	wg := sync.WaitGroup{}
	jobs := make(chan string)
	matches := make(chan *jsluice.URL)

	for i := 0; i < concurrency; i++ {

		wg.Add(1)
		go func() {

			for filename := range jobs {

				source, err := ioutil.ReadFile(filename)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s", err)
					continue
				}

				// print just the tree and stop
				if treeMode {
					fmt.Printf("%s:\n", filename)
					jsluice.PrintTree(source)
					continue
				}

				analzyer := jsluice.NewAnalyzer(source)
				for _, m := range analzyer.GetURLs() {
					m.Filename = filename
					matches <- m
				}

			}

			wg.Done()
		}()

	}

	// read jobs from the input reader, send on jobs channel, close jobs channel
	go func() {
		sc := bufio.NewScanner(input)
		for sc.Scan() {
			filename := sc.Text()
			jobs <- filename
		}
		close(jobs)

		wg.Wait()
		close(matches)
	}()

	// read and filter the results
	for m := range matches {

		if ignoreStrings && m.Type == "stringLiteral" {
			continue
		}

		// remove filename if the user doesn't want it
		if !includeFilename {
			m.Filename = ""
		}

		// remove any souce if we don't want to display it
		if !includeSource {
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
			fmt.Fprintf(os.Stderr, "%s\n", err)
			continue
		}
		fmt.Printf("%s\n", j)
	}

}
