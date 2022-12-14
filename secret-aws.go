package jsluice

import (
	"regexp"
	"strings"
)

func awsMatcher() SecretMatcher {
	awsKey := regexp.MustCompile("^\\w+$")

	return SecretMatcher{"(string) @matches", func(n *Node) *Secret {
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

	}}
}
