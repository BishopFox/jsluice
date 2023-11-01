package jsluice

import (
	"strings"
)

// A Secret represents any secret or otherwise interesting data
// found within a JavaScript file. E.g. an AWS access key.
type Secret struct {
	Kind     string   `json:"kind"`
	Data     any      `json:"data"`
	Filename string   `json:"filename,omitempty"`
	Severity Severity `json:"severity"`
	Context  any      `json:"context"`
}

// Severity indicates how serious a finding is
type Severity string

const (
	SeverityInfo   Severity = "info"
	SeverityLow    Severity = "low"
	SeverityMedium Severity = "medium"
	SeverityHigh   Severity = "high"
)

// AddSecretMatcher allows custom SecretMatchers to be added to the Analyzer
func (a *Analyzer) AddSecretMatcher(s SecretMatcher) {
	if a.userSecretMatchers == nil {
		a.userSecretMatchers = make([]SecretMatcher, 0)
	}

	a.userSecretMatchers = append(a.userSecretMatchers, s)
}

// AddSecretMatchers allows multiple custom SecretMatchers to be added to the Analyzer
func (a *Analyzer) AddSecretMatchers(ss []SecretMatcher) {
	if a.userSecretMatchers == nil {
		a.userSecretMatchers = make([]SecretMatcher, 0)
	}

	a.userSecretMatchers = append(a.userSecretMatchers, ss...)
}

// GetSecrets uses the parse tree and a set of Matchers (those provided
// by AllSecretMatchers()) to find secrets in JavaScript source code.
func (a *Analyzer) GetSecrets() []*Secret {
	out := make([]*Secret, 0)

	// we only want to run each query once so let's cache them
	nodeCache := make(map[string][]*Node)

	matchers := AllSecretMatchers()

	if a.userSecretMatchers != nil {
		matchers = append(matchers, a.userSecretMatchers...)
	}

	for _, m := range matchers {
		if _, exists := nodeCache[m.Query]; !exists {
			nodes := make([]*Node, 0)
			a.Query(m.Query, func(n *Node) {
				nodes = append(nodes, n)
			})
			nodeCache[m.Query] = nodes
		}

		nodes := nodeCache[m.Query]

		for _, n := range nodes {
			match := m.Fn(n)
			if match == nil {
				continue
			}

			out = append(out, match)
		}
	}
	return out
}

// A SecretMatcher is a tree-sitter query to find relevant nodes
// in the parse tree, and a function to inspect those nodes,
// returning any Secret that is found.
type SecretMatcher struct {
	Query string
	Fn    func(*Node) *Secret
}

// AllSecretMatchers returns the default list of SecretMatchers
func AllSecretMatchers() []SecretMatcher {

	return []SecretMatcher{
		awsMatcher(),
		gcpKeyMatcher(),
		firebaseMatcher(),
		githubKeyMatcher(),

		// REACT_APP_... containing objects
		{"(object) @matches", func(n *Node) *Secret {

			// disabled due to high false positive rate
			return nil

			o := n.AsObject()

			hasReactAppKeys := false
			for _, k := range o.GetKeys() {
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
				Data: o.AsMap(),
			}
		}},

		// generic secrets
		{"(pair) @matches", func(n *Node) *Secret {

			// disabled due to very high false positive rate
			// but left easy to enable for research purposes
			return nil

			key := n.ChildByFieldName("key")
			if key == nil {
				return nil
			}

			keyStr := strings.ToLower(key.RawString())
			if !strings.Contains(keyStr, "secret") {
				return nil
			}

			value := n.ChildByFieldName("value")
			if value == nil || value.Type() != "string" {
				return nil
			}

			data := map[string]string{
				"key": value.RawString(),
			}

			match := &Secret{
				Kind: "genericSecret",
				Data: data,
			}

			parent := n.Parent()
			if parent == nil || parent.Type() != "object" {
				return match
			}

			match.Context = parent.AsObject().AsMap()

			return match
		}},
	}
}
