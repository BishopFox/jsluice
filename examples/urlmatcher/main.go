package main

import (
	"fmt"
	"strings"

	"github.com/BishopFox/jsluice"
)

func main() {
	analyzer := jsluice.NewAnalyzer([]byte(`
		var fn = () => {
			var meta = {
				contact: "mailto:contact@example.com",
				home: "https://example.com"
			}
			return meta
		}
	`))

	analyzer.AddURLMatcher(
		// The first value in the jsluice.URLMatcher struct is the type of node to look for.
		// It can be one of "string", "assignment_expression", or "call_expression"
		jsluice.URLMatcher{"string", func(n *jsluice.Node) *jsluice.URL {
			val := n.DecodedString()
			if !strings.HasPrefix(val, "mailto:") {
				return nil
			}

			return &jsluice.URL{
				URL:  val,
				Type: "mailto",
			}
		}},
	)

	for _, match := range analyzer.GetURLs() {
		fmt.Println(match.URL)
	}
}
