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

		// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_identifiers.html
		prefixes := []string{
			"ABIA", "ACCA", "AGPA", "AIDA",
			"AIPA", "AKIA", "ANPA", "ANVA",
			"APKA", "AROA", "ASCA", "ASIA",
		}

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

		if strings.Contains(str, "_") {
			return nil
		}

		// Check it matches the regex
		if !awsKey.MatchString(str) {
			return nil
		}

		data := map[string]string{
			"key": str,
		}

		match := &Secret{
			Kind:     "AWSAccessKey",
			Severity: SeverityLow,
			Data:     data,
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

		for _, k := range o.GetKeys() {
			k = strings.ToLower(k)
			if strings.Contains(k, "secret") {
				// TODO: check format of value
				// TODO: think of a way to handle multiple secrets in the same object?
				data["secret"] = DecodeString(o.GetStringI(k, ""))
				break
			}
		}

		sev := SeverityLow
		if data["secret"] != "" {
			sev = SeverityHigh
		}
		return &Secret{
			Kind:     "AWSAccessKey",
			Severity: sev,
			Data:     data,
			Context:  o.AsMap(),
		}

	}}
}
