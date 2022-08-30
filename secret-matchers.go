package jsluice

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

type Secret struct {
	Kind     string `json:"kind"`
	Data     any    `json:"data"`
	Filename string `json:"filename,omitempty"`
}

func (a *Analyzer) GetSecrets() []*Secret {
	out := make([]*Secret, 0)

	// we only want to run each query once so let's cache them
	nodeCache := make(map[string][]*sitter.Node)

	matchers := AllSecretMatchers()
	for _, m := range matchers {

		if _, exists := nodeCache[m.Query]; !exists {
			nodes := make([]*sitter.Node, 0)
			query(a.rootNode, m.Query, func(n *sitter.Node) {
				nodes = append(nodes, n)
			})
			nodeCache[m.Query] = nodes
		}

		nodes := nodeCache[m.Query]

		for _, n := range nodes {
			match := m.Fn(n, a.source)
			if match == nil {
				continue
			}

			out = append(out, match)
		}
	}
	return out
}

type SecretMatcher struct {
	Query string
	Fn    func(*sitter.Node, []byte) *Secret
}

func AllSecretMatchers() []SecretMatcher {
	return []SecretMatcher{
		// AWS Keys
		{"(string) @matches", func(n *sitter.Node, source []byte) *Secret {
			str := dequote(content(n, source))

			// https://docs.aws.amazon.com/STS/latest/APIReference/API_Credentials.html
			if len(str) < 16 || len(str) > 128 {
				return nil
			}
			prefixes := []string{"AKIA", "A3T", "AGPA", "AIDA", "AROA", "AIPA", "ANPA", "ANVA", "ASIA"}

			found := false
			for _, p := range prefixes {
				if strings.HasPrefix(str, p) {
					found = true
					break
				}
			}

			if !found {
				return nil
			}

			// TODO: check the rest of the chars in the string

			data := make(map[string]string)
			data["key"] = str

			match := &Secret{
				Kind: "AWSAccessKey",
				Data: data,
			}

			// We want to look in the same object for anything 'secret'.
			// If the parent type is "pair" and the granparent type is
			// "object" we can do that.
			parent := n.Parent()
			if parent == nil || parent.Type() != "pair" {
				return match
			}

			grandparent := parent.Parent()
			if grandparent == nil || grandparent.Type() != "object" {
				return match
			}

			o := newObject(grandparent, source)

			for _, k := range o.getKeys() {
				k = strings.ToLower(k)
				if strings.Contains(k, "secret") {
					// TODO: check format of value
					// TODO: think of a way to handle multiple secrets in the same object?
					data["secret"] = DecodeString(o.getStringI(k, ""))
					break
				}
			}

			return &Secret{
				Kind: "AWSAccessKey",
				Data: data,
			}

		}},

		// REACT_APP_... containing objects
		{"(object) @matches", func(n *sitter.Node, source []byte) *Secret {

			return nil
			o := newObject(n, source)

			hasReactAppKeys := false
			for _, k := range o.getKeys() {
				if strings.HasPrefix(k, "REACT_APP_") {
					hasReactAppKeys = true
					break
				}
			}

			if !hasReactAppKeys {
				return nil
			}

			return &Secret{
				Kind: "reactApp",
				Data: o.asMap(),
			}
		}},

		// Firebase objects
		{"(object) @matches", func(n *sitter.Node, source []byte) *Secret {
			o := newObject(n, source)

			mustHave := map[string]bool{
				"apiKey":        true,
				"appId":         true,
				"authDomain":    true,
				"projectId":     true,
				"storageBucket": true,
			}

			count := 0
			for _, k := range o.getKeys() {
				if mustHave[k] {
					count++
				}
			}
			if count != len(mustHave) {
				return nil
			}

			return &Secret{
				Kind: "firebase",
				Data: o.asMap(),
			}
		}},
	}
}
