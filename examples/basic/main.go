package main

import (
	"encoding/json"
	"fmt"

	"github.com/BishopFox/jsluice"
)

func main() {
	analyzer := jsluice.NewAnalyzer([]byte(`
		const login = (redirect) => {
			document.location = "/login?redirect=" + redirect + "&method=oauth"
		}
	`))

	for _, url := range analyzer.GetURLs() {
		j, err := json.MarshalIndent(url, "", "  ")
		if err != nil {
			continue
		}

		fmt.Printf("%s\n", j)
	}
}
