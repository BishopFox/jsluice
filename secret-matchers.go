package jsluice

import (
	"regexp"
	"strings"
)

// A Secret represents any bit of secret or otherwise interesting
// data found within a JavaScript file. E.g. an AWS access key and
// secret.
type Secret struct {
	Kind       string `json:"kind"`
	Data       any    `json:"data"`
	Filename   string `json:"filename,omitempty"`
	LeadWorthy bool   `json:"leadWorthy"`
}

// GetSecrets uses the parse tree and a set of Matchers (those provided
// by AllSecretMatchers()) to find secrets in JavaScript source code.
func (a *Analyzer) GetSecrets() []*Secret {
	out := make([]*Secret, 0)

	// we only want to run each query once so let's cache them
	nodeCache := make(map[string][]*Node)

	matchers := AllSecretMatchers()
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
	awsKey := regexp.MustCompile("^\\w+$")
	gcpKey := regexp.MustCompile("^AIza[a-zA-Z0-9+_-]+$")

	return []SecretMatcher{
		// AWS Keys
		{"(string) @matches", func(n *Node) *Secret {
			str := n.RawString()

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

			// Check it matches the regex
			if !awsKey.MatchString(str) {
				return nil
			}

			data := struct {
				Key     string            `json:"key"`
				Secret  string            `json:"secret,omitempty"`
				Context map[string]string `json:"context,omitempty"`
			}{
				Key: str,
			}

			match := &Secret{
				Kind:       "AWSAccessKey",
				LeadWorthy: false,
				Data:       data,
			}

			// We want to look in the same object for anything 'secret'.
			// If the parent type is "pair" and the grandparent type is
			// "object" we can do that.
			parent := n.Parent()
			if parent == nil || parent.Type() != "pair" {
				return match
			}

			grandparent := parent.Parent()
			if grandparent == nil || grandparent.Type() != "object" {
				return match
			}

			o := grandparent.AsObject()
			data.Context = o.asMap()

			for _, k := range o.getKeys() {
				k = strings.ToLower(k)
				if strings.Contains(k, "secret") {
					// TODO: check format of value
					// TODO: think of a way to handle multiple secrets in the same object?
					data.Secret = DecodeString(o.getStringI(k, ""))
					break
				}
			}

			return &Secret{
				Kind:       "AWSAccessKey",
				LeadWorthy: data.Secret != "",
				Data:       data,
			}

		}},

		// REACT_APP_... containing objects
		{"(object) @matches", func(n *Node) *Secret {

			// disabled due to high false positive rate
			return nil

			o := n.AsObject()

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

		// GCP keys
		{"(string) @matches", func(n *Node) *Secret {
			str := n.RawString()

			// Prefix check is nice and fast so we'll do that first
			// Remember that there are a *lot* of strings in JS files :D
			if !strings.HasPrefix(str, "AIza") {
				return nil
			}

			if !gcpKey.MatchString(str) {
				return nil
			}

			data := struct {
				Key     string            `json:"key"`
				Context map[string]string `json:"context,omitempty"`
			}{
				Key: str,
			}

			match := &Secret{
				Kind:       "gcpKey",
				LeadWorthy: false,
				Data:       data,
			}

			// If the key is in an object we want to include that whole object as context
			parent := n.Parent()
			if parent == nil || parent.Type() != "pair" {
				return match
			}

			grandparent := parent.Parent()
			if grandparent == nil || grandparent.Type() != "object" {
				return match
			}

			data.Context = grandparent.AsObject().asMap()
			match.Data = data

			return match
		}},

		// Firebase objects
		{"(object) @matches", func(n *Node) *Secret {
			o := n.AsObject()

			mustHave := map[string]bool{
				"apiKey":        true,
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

			if !strings.HasPrefix(o.getStringI("apiKey", ""), "AIza") {
				return nil
			}

			return &Secret{
				Kind:       "firebase",
				LeadWorthy: true,
				Data:       o.asMap(),
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

			data := struct {
				Key     string            `json:"key"`
				Context map[string]string `json:"context,omitempty"`
			}{
				Key: value.RawString(),
			}

			match := &Secret{
				Kind: "genericSecret",
				Data: data,
			}

			parent := n.Parent()
			if parent == nil || parent.Type() != "object" {
				return match
			}

			data.Context = parent.AsObject().asMap()
			match.Data = data

			return match
		}},
	}
}
