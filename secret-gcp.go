package jsluice

import (
	"regexp"
	"strings"
)

func gcpKeyMatcher() SecretMatcher {
	gcpKey := regexp.MustCompile("^AIza[a-zA-Z0-9+_-]+$")

	return SecretMatcher{"(string) @matches", func(n *Node) *Secret {
		str := n.RawString()

		// Prefix check is nice and fast so we'll do that first
		// Remember that there are a *lot* of strings in JS files :D
		if !strings.HasPrefix(str, "AIza") {
			return nil
		}

		if !gcpKey.MatchString(str) {
			return nil
		}

		data := map[string]string{
			"key": str,
		}

		match := &Secret{
			Kind:     "gcpKey",
			Severity: SeverityLow,
			Data:     data,
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

		match.Context = grandparent.AsObject().AsMap()

		return match
	}}
}

func firebaseMatcher() SecretMatcher {
	// Firebase objects
	return SecretMatcher{"(object) @matches", func(n *Node) *Secret {
		o := n.AsObject()

		mustHave := map[string]bool{
			"apiKey":        true,
			"authDomain":    true,
			"projectId":     true,
			"storageBucket": true,
		}

		count := 0
		for _, k := range o.GetKeys() {
			if mustHave[k] {
				count++
			}
		}
		if count != len(mustHave) {
			return nil
		}

		if !strings.HasPrefix(o.GetStringI("apiKey", ""), "AIza") {
			return nil
		}

		return &Secret{
			Kind:     "firebase",
			Severity: SeverityHigh,
			Data:     o.AsMap(),
		}
	}}
}
