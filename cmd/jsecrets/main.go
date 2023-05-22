package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/BishopFox/jsluice"
	"github.com/pkg/profile"
)

func main() {
	var profileMode bool
	flag.BoolVar(&profileMode, "profile", false, "Profile cpu usage and save a cpu.pprof file in the current dir")

	var patternFile string
	flag.StringVar(&patternFile, "patterns", "", "JSON file containing user-defined patterns")

	flag.Parse()

	if profileMode {
		defer profile.Start(profile.ProfilePath(".")).Stop()
	}

	filename := flag.Arg(0)

	source, err := ioutil.ReadFile(filename)
	if err != nil {
		// TODO: add better error output than log.Fatal()
		log.Fatal(err)
	}

	analyzer := jsluice.NewAnalyzer(source)

	if patternFile != "" {
		f, err := os.Open(patternFile)
		if err != nil {
			log.Fatal(err)
		}

		patterns, err := jsluice.ParseUserPatterns(f)
		if err != nil {
			log.Fatal(err)
		}

		analyzer.AddSecretMatcher(patterns.SecretMatcher())
	}

	matches := analyzer.GetSecrets()
	for _, match := range matches {

		match.Filename = filename

		j, err := json.Marshal(match)
		if err != nil {
			continue
		}
		fmt.Printf("%s\n", j)
	}
}
