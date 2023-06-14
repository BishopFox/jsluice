package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/BishopFox/jsluice"
)

func main() {
	analyzer := jsluice.NewAnalyzer([]byte(`
		var config = {
			apiKey: "AUTH_1a2b3c4d5e6f",
			apiURL: "https://api.example.com/v2/"
		}
	`))

	analyzer.AddSecretMatcher(
		// The first value in the jsluice.SecretMatcher struct is a
		// tree-sitter query to run on the JavaScript source.
		jsluice.SecretMatcher{"(pair) @match", func(n *jsluice.Node) *jsluice.Secret {
			key := n.ChildByFieldName("key").DecodedString()
			value := n.ChildByFieldName("value").DecodedString()

			if !strings.Contains(key, "api") {
				return nil
			}

			if !strings.HasPrefix(value, "AUTH_") {
				return nil
			}

			return &jsluice.Secret{
				Kind: "fakeApi",
				Data: map[string]string{
					"key":   key,
					"value": value,
				},
				Severity: jsluice.SeverityLow,
				Context:  n.Parent().AsMap(),
			}
		}},
	)

	for _, match := range analyzer.GetSecrets() {
		j, err := json.MarshalIndent(match, "", "  ")
		if err != nil {
			continue
		}

		fmt.Printf("%s\n", j)
	}
}
