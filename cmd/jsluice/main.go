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

	"github.com/pkg/profile"
	flag "github.com/spf13/pflag"
)

type options struct {
	// global
	profile     bool
	concurrency int

	// urls
	includeSource   bool
	ignoreStrings   bool
	includeFilename bool
	resolvePaths    string
}

const (
	modeURLs    = "urls"
	modeSecrets = "secrets"
	modeTree    = "tree"
	modeQuery   = "query"
)

type cmdFn func(options, string, []byte, chan string, chan error)

func main() {
	var opts options

	// global options
	flag.BoolVar(&opts.profile, "profile", false, "Profile CPU usage and save a cpu.pprof file in the current dir")
	flag.IntVarP(&opts.concurrency, "concurrency", "c", 1, "Number of files to process concurrently")

	// url options
	flag.BoolVar(&opts.includeSource, "include-source", false, "Include the source code where the URL was found")
	flag.BoolVar(&opts.ignoreStrings, "ignore-strings", false, "Ignore matches from string literals")
	flag.BoolVar(&opts.includeFilename, "include-filename", false, "Include the filename of the matched file in the output")
	flag.StringVar(&opts.resolvePaths, "resolve-paths", "", "Resolve relative paths using the absolute URL provided")

	flag.Parse()

	if opts.profile {
		defer profile.Start(profile.ProfilePath(".")).Stop()
	}

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: jsluice <mode> [...flags]")
		os.Exit(1)
	}

	mode := args[0]
	files := args[1:]

	// spin up an output worker
	output := make(chan string)
	errors := make(chan error)
	done := make(chan any)

	go func() {
		for {
			select {
			case out := <-output:
				fmt.Println(out)
			case err := <-errors:
				fmt.Fprintf(os.Stderr, "error: %s\n", err)
			case <-done:
				break
			}
		}
	}()

	// now the process workers
	var modeFn cmdFn
	modes := map[string]cmdFn{
		modeURLs: extractURLs,
		modeTree: printTree,
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
					errors <- err
					continue
				}

				modeFn(opts, filename, source, output, errors)
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
	close(errors)

}
