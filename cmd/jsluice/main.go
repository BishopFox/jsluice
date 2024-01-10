package main

// Extract URLs and related stuff out of JavaScript files

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/BishopFox/jsluice"
	"github.com/pkg/profile"
	"github.com/slyrz/warc"
	flag "github.com/spf13/pflag"
)

type options struct {
	// global
	profile     bool
	cookie      string
	headers     []string
	concurrency int
	placeholder string
	help        bool
	warc        bool
	rawInput    bool
	certCheck   bool

	// urls
	includeSource bool
	ignoreStrings bool
	resolvePaths  string
	unique        bool

	// secrets
	patternsFile string

	// query
	query           string
	rawOutput       bool
	includeFilename bool
	format          bool
}

const (
	modeURLs    = "urls"
	modeSecrets = "secrets"
	modeTree    = "tree"
	modeQuery   = "query"
	modeFormat  = "format"
)

type stringSlice []string

func (ss *stringSlice) String() string {
	return strings.Join(*ss, ", ")
}

func (ss *stringSlice) Set(value string) error {
	*ss = append(*ss, value)
	return nil
}

func (ss *stringSlice) Type() string {
	return "string"
}

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
			"  format    Format JavaScript source using jsbeautifier-go",
			"",
			"Global options:",
			"  -c, --concurrency int        Number of files to process concurrently (default 1)",
			"  -C, --cookie string          Cookies to use when making requests to the specified HTTP based arguments",
			"  -H, --header string          Headers to use when making requests to the specified HTTP based arguments (can be specified multiple times)",
			"  -P, --placeholder string     Set the expression placeholder to a custom string (default 'EXPR')",
			"  -j, --raw-input              Read raw JavaScript source from stdin",
			"  -w, --warc                   Treat the input files as WARC (Web ARChive) files",
			"  -i, --no-check-certificate	Ignore validation of server certificates",
			"",
			"URLs mode:",
			"  -I, --ignore-strings         Ignore matches from string literals",
			"  -S, --include-source         Include the source code where the URL was found",
			"  -R, --resolve-paths <url>    Resolve relative paths using the absolute URL provided",
			"  -u, --unique                 Only output each URL once per input file",
			"",
			"Secrets mode:",
			"  -p, --patterns <file>        JSON file containing user-defined secret patterns to look for",
			"",
			"Query mode:",
			"  -q, --query <query>          Tree sitter query to run; e.g. '(string) @matches'",
			"  -r, --raw-output             Do not convert values to native types",
			"  -f, --include-filename       Include the filename in the output",
			"  -F, --format                 Format source code in the output",
			"",
			"Examples:",
			"  jsluice urls -C 'auth=true; user=admin;' -H 'Specific-Header-One: true' -H 'Specific-Header-Two: false' local_file.js https://remote.host/example.js",
			"  jsluice query -q '(object) @m' one.js two.js",
			"  find . -name '*.js' | jsluice secrets -c 5 --patterns=apikeys.json",
		}
		fmt.Fprintf(os.Stderr, "%s\n", strings.Join(lines, "\n"))
	}
}

func main() {
	var opts options
	var headers stringSlice

	// global options
	flag.BoolVar(&opts.profile, "profile", false, "Profile CPU usage and save a cpu.pprof file in the current dir")
	flag.IntVarP(&opts.concurrency, "concurrency", "c", 1, "Number of files to process concurrently")
	flag.StringVarP(&opts.cookie, "cookie", "C", "", "Cookie(s) to use when making HTTP requests")
	flag.VarP(&headers, "header", "H", "Headers to use when making HTTP requests")
	flag.BoolVarP(&opts.rawInput, "raw-input", "j", false, "Read raw JavaScript source from stdin")
	flag.StringVarP(&opts.placeholder, "placeholder", "P", "EXPR", "Set the expression placeholder to a custom string")
	flag.BoolVarP(&opts.help, "help", "h", false, "")
	flag.BoolVarP(&opts.warc, "warc", "w", false, "")
	flag.BoolVarP(&opts.certCheck, "no-check-certificate", "i", false, "Ignore validation of server certificates")

	// url options
	flag.BoolVarP(&opts.includeSource, "include-source", "S", false, "Include the source code where the URL was found")
	flag.BoolVarP(&opts.ignoreStrings, "ignore-strings", "I", false, "Ignore matches from string literals")
	flag.StringVarP(&opts.resolvePaths, "resolve-paths", "R", "", "Resolve relative paths using the absolute URL provided")
	flag.BoolVarP(&opts.unique, "unique", "u", false, "")

	// secrets options
	flag.StringVarP(&opts.patternsFile, "patterns", "p", "", "JSON file containing user-defined secret patterns to look for")

	// query options
	flag.StringVarP(&opts.query, "query", "q", "", "Tree sitter query to run; e.g. '(string) @matches'")
	flag.BoolVarP(&opts.rawOutput, "raw-output", "r", false, "Do not convert values to native types")
	flag.BoolVarP(&opts.includeFilename, "include-filename", "f", false, "Include the filename in the output")
	flag.BoolVarP(&opts.format, "format", "F", false, "Format source code in the output")

	flag.Parse()

	opts.headers = headers

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
				return
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
		modeFormat:  format,
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

				if opts.warc {
					responses, err := readWARCFile(filename)
					if err != nil {
						errs <- err
						continue
					}

					for _, response := range responses {
						modeFn(opts, response.url, response.source, output, errs)
					}
					continue
				}

				source, err := readFromFileOrURL(filename, opts.cookie, opts.headers, opts.certCheck)
				if err != nil {
					errs <- err
					continue
				}

				modeFn(opts, filename, source, output, errs)
			}
		}()
	}

	// If we're reading JS source straight from stdin, throw
	// it into a temp file that we'll clean up later
	if opts.rawInput {
		tmpfile, err := os.CreateTemp("", "jsluice-raw-input")

		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create temp file for raw input: %s\n", err)
			os.Exit(3)
		}
		defer os.Remove(tmpfile.Name())

		_, err = io.Copy(tmpfile, os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to write raw input to temp file: %s\n", err)
			os.Exit(3)
		}

		err = tmpfile.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to close temp file: %s\n", err)
			os.Exit(3)
		}

		// overwrite the files slice so we read only the raw input
		files = []string{tmpfile.Name()}
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

func readFromFileOrURL(path string, cookie string, headers []string, ignoreCert bool) ([]byte, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		client := &http.Client{}

		if ignoreCert {
			client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
		}

		req, err := http.NewRequest("GET", path, nil)
		if err != nil {
			return nil, err
		}

		// Add cookie to the request if specified
		if cookie != "" {
			req.Header.Set("Cookie", cookie)
		}

		// Add headers to the request if specified
		for _, header := range headers {
			parts := strings.SplitN(header, ":", 2)
			if len(parts) == 2 {
				req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		// Check if the request was successful
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("GET request failed with status code %d", resp.StatusCode)
		}

		return ioutil.ReadAll(resp.Body)
	}

	return ioutil.ReadFile(path)
}

type warcResponse struct {
	url    string
	source []byte
}

func readWARCFile(filename string) ([]warcResponse, error) {
	out := make([]warcResponse, 0)

	f, err := os.Open(filename)
	if err != nil {
		return out, err
	}
	defer f.Close()

	r, err := warc.NewReader(f)
	if err != nil {
		return out, err
	}
	defer r.Close()

	for {
		record, err := r.ReadRecord()
		if err != nil {
			break
		}

		if record.Header.Get("content-type") != "application/http; msgtype=response" {
			continue
		}

		buf := bufio.NewReader(record.Content)
		response, err := http.ReadResponse(buf, nil)
		if err != nil {
			return out, err
		}

		ct := strings.ToLower(response.Header.Get("content-type"))
		if !strings.Contains(ct, "javascript") && !strings.Contains(ct, "html") {
			continue
		}

		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return out, err
		}
		response.Body.Close()

		out = append(out, warcResponse{
			url:    record.Header.Get("WARC-Target-URI"),
			source: body,
		})
	}

	return out, nil
}
