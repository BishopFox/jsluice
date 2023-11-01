package jsluice

import (
	"regexp"
)

func githubKeyMatcher() SecretMatcher {
	githubKey := regexp.MustCompile("([a-zA-Z0-9_-]{2,}:)?ghp_[a-zA-Z0-9]{30,}")

	return SecretMatcher{"(string) @matches", func(n *Node) *Secret {
		str := n.RawString()

		if !githubKey.MatchString(str) {
			return nil
		}

		data := map[string]string{
			"key": str,
		}

		match := &Secret{
			Kind:     "githubKey",
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
