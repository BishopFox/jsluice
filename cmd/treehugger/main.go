package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/BishopFox/jsluice"
)

func main() {

	flag.Parse()
	queryStr := flag.Arg(0)

	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		source, err := ioutil.ReadFile(sc.Text())
		if err != nil {
			fmt.Fprintf(os.Stderr, "error opening file: %s\n", err)
			continue
		}

		analyzer := jsluice.NewAnalyzer(source)

		enter := func(n *jsluice.Node) {
			fmt.Println(n.Content())
		}

		analyzer.Query(queryStr, enter)
	}
}
