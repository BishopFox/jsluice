package main

// Extract URLs and related stuff out of JavaScript files

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"sync"

	"github.com/BishopFox/jsluice"
	"github.com/pkg/profile"
	flag "github.com/spf13/pflag"
)

type options struct {
	// global
	profile     bool
	concurrency int
	placeholder string
	help        bool

	// urls
	includeSource bool
	ignoreStrings bool
	resolvePaths  string

	// secrets
	patternsFile string

	// query
	query     string
	rawOutput bool
}

const (
	modeURLs    = "urls"
	modeSecrets = "secrets"
	modeTree    = "tree"
	modeQuery   = "query"
)

type cmdFn func(options, string, []byte, chan string, chan error)

func init() {
	flag.Usage = func() {
		lines := []string{
			"jsluice - Extract URLs, paths, and secrets from JavaScript files",
			"",
			"Usage:",
			"  jsluice <mode> [options] [file...]",
			"",
			"Modes:",
			"  urls      Extract URLs and paths",
			"  secrets   Extract secrets and other interesting bits",
			"  tree      Print syntax trees for input files",
			"  query     Run tree-sitter a query against input files",
			"",
			"Global options:",
			"  -c, --concurrency int        Number of files to process concurrently (default 1)",
			"  -P, --placeholder string     Set the expression placeholder to a custom string (default 'EXPR')",
			"",
			"URLs mode:",
			"  -I, --ignore-strings         Ignore matches from string literals",
			"  -S, --include-source         Include the source code where the URL was found",
			"  -R, --resolve-paths <url>    Resolve relative paths using the absolute URL provided",
			"",
			"Secrets mode:",
			"  -p, --patterns <file>        JSON file containing user-defined secret patterns to look for",
			"",
			"Query mode:",
			"  -q, --query <query>          Tree sitter query to run; e.g. '(string) @matches'",
			"  -r, --raw-output             Do not JSON-encode query output",
			"",
			"Examples:",
			"  jsluice urls example.js",
			"  jsluice query -q '(object) @m' one.js two.js",
			"  find . -name *.js' | jsluice secrets -c 5 --patterns=apikeys.json",
		}
		fmt.Fprintf(os.Stderr, "%s\n", strings.Join(lines, "\n"))
	}
}

func main() {
	var opts options

	// global options
	flag.BoolVar(&opts.profile, "profile", false, "Profile CPU usage and save a cpu.pprof file in the current dir")
	flag.IntVarP(&opts.concurrency, "concurrency", "c", 1, "Number of files to process concurrently")
	flag.StringVarP(&opts.placeholder, "placeholder", "P", "EXPR", "Set the expression placeholder to a custom string")
	flag.BoolVarP(&opts.help, "help", "h", false, "")

	// url options
	flag.BoolVarP(&opts.includeSource, "include-source", "S", false, "Include the source code where the URL was found")
	flag.BoolVarP(&opts.ignoreStrings, "ignore-strings", "I", false, "Ignore matches from string literals")
	flag.StringVarP(&opts.resolvePaths, "resolve-paths", "R", "", "Resolve relative paths using the absolute URL provided")

	// secrets options
	flag.StringVarP(&opts.patternsFile, "patterns", "p", "", "JSON file containing user-defined secret patterns to look for")

	// query options
	flag.StringVarP(&opts.query, "query", "q", "", "Tree sitter query to run; e.g. '(string) @matches'")
	flag.BoolVarP(&opts.rawOutput, "raw-output", "r", false, "Do not JSON-encode query output")

	flag.Parse()

	if opts.help {
		flag.Usage()
		return
	}

	if opts.profile {
		defer profile.Start(profile.ProfilePath(".")).Stop()
	}

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: jsluice <mode> [...flags]")
		os.Exit(1)
	}

	jsluice.ExpressionPlaceholder = opts.placeholder

	mode := args[0]
	files := args[1:]

	// spin up an output worker
	output := make(chan string)
	errs := make(chan error)
	done := make(chan any)

	go func() {
		for {
			select {
			case out := <-output:
				if out == "" {
					continue
				}
				fmt.Println(out)
			case err := <-errs:
				fmt.Fprintf(os.Stderr, "error: %s\n", err)
			case <-done:
				break
			}
		}
	}()

	// now the process workers
	var modeFn cmdFn
	modes := map[string]cmdFn{
		modeURLs:    extractURLs,
		modeSecrets: extractSecrets,
		modeTree:    printTree,
		modeQuery:   runQuery,
	}

	if _, exists := modes[mode]; !exists {
		fmt.Fprintf(os.Stderr, "no such mode: %s\n", mode)
		os.Exit(2)
	}
	modeFn = modes[mode]

	jobs := make(chan string)

	var wg sync.WaitGroup
	for i := 0; i < opts.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filename := range jobs {

				source, err := ioutil.ReadFile(filename)
				if err != nil {
					errs <- err
					continue
				}

				modeFn(opts, filename, source, output, errs)
			}
		}()
	}

	// default to reading filenames from stdin, fall back
	// to treating the argument list as filenames
	var r io.Reader = os.Stdin
	if len(files) > 0 {
		r = strings.NewReader(strings.Join(files, "\n"))
	}
	input := bufio.NewScanner(r)

	for input.Scan() {
		jobs <- input.Text()
	}
	close(jobs)

	wg.Wait()
	done <- struct{}{}
	close(output)
	close(errs)

}
