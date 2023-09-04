package jsluice

import (
	"encoding/json"
	"errors"
	"io"
	"regexp"
)

// A UserPattern represents a pattern that was provided by a
// when using the command-line tool. When using the package
// directly, a SecretMatcher can be created directly instead
// of creating a UserPattern
type UserPattern struct {
	Name     string   `json:"name"`
	Key      string   `json:"key"`
	Value    string   `json:"value"`
	Severity Severity `json:"severity"`

	Object []*UserPattern `json:"object"`

	reKey   *regexp.Regexp
	reValue *regexp.Regexp
}

// ParseRegex parses all of the user-provided regular expressions
// for a pattern into Go *regexp.Regexp types
func (u *UserPattern) ParseRegex() error {
	if u.Value != "" {
		re, err := regexp.Compile(u.Value)
		if err != nil {
			return err
		}
		u.reValue = re
	}

	if u.Key != "" {
		re, err := regexp.Compile(u.Key)
		if err != nil {
			return err
		}
		u.reKey = re
	}

	if len(u.Object) > 0 {
		for _, m := range u.Object {
			m.ParseRegex()
		}
	}

	if u.Severity == "" {
		u.Severity = SeverityInfo
	}

	if u.reValue == nil && u.reKey == nil && len(u.Object) == 0 {
		return errors.New("'key', 'value', both, or 'object' must be supplied in user-defined matcher")
	}

	return nil
}

// MatchValue returns true if a pattern's value regex matches
// the supplied value, or if there is no value regex.
func (u *UserPattern) MatchValue(in string) bool {
	if u.reValue == nil {
		return true
	}
	return u.reValue.MatchString(in)
}

// MatchKey returns true if a pattern's key regex matches
// the supplied value, or if there is no key regex
func (u *UserPattern) MatchKey(in string) bool {
	if u.reKey == nil {
		return true
	}
	return u.reKey.MatchString(in)
}

// SecretMatcher returns a SecretMatcher based on the UserPattern,
// for use with (*Analyzer).AddSecretMatcher()
func (u *UserPattern) SecretMatcher() SecretMatcher {
	if len(u.Object) > 0 {
		return u.objectMatcher()
	}

	if u.reKey != nil {
		return u.pairMatcher()
	}

	return u.stringMatcher()
}

// objectMatcher returns a SecretMatcher for matching against objects
func (u *UserPattern) objectMatcher() SecretMatcher {
	return SecretMatcher{"(object) @matches", func(n *Node) *Secret {
		pairs := n.NamedChildren()

		matched := 0

		for _, pat := range u.Object {
			matcher := pat.pairMatcher()

			for _, pair := range pairs {
				if matcher.Fn(pair) != nil {
					matched++
					break
				}
			}
		}

		if matched != len(u.Object) {
			return nil
		}

		secret := &Secret{
			Kind:     u.Name,
			Data:     n.AsObject().AsMap(),
			Severity: u.Severity,
		}

		return secret
	}}
}

// pairMatcher returns a SecretMatcher for matching against key/value pairs
func (u *UserPattern) pairMatcher() SecretMatcher {
	return SecretMatcher{"(pair) @matches", func(n *Node) *Secret {

		key := n.ChildByFieldName("key")
		if key == nil || !u.MatchKey(key.RawString()) {
			return nil
		}

		value := n.ChildByFieldName("value")
		if value == nil || value.Type() != "string" {
			return nil
		}

		if !u.MatchValue(value.RawString()) {
			return nil
		}

		secret := &Secret{
			Kind: u.Name,
			Data: map[string]string{
				"key":   key.RawString(),
				"value": value.RawString(),
			},
			Severity: u.Severity,
		}

		parent := n.Parent()
		if parent == nil || parent.Type() != "object" {
			return secret
		}

		secret.Context = parent.AsObject().AsMap()

		return secret

	}}
}

// stringMatcher returns a SecretMatcher for matching against string literals
func (u *UserPattern) stringMatcher() SecretMatcher {
	return SecretMatcher{"(string) @matches", func(n *Node) *Secret {
		in := n.RawString()
		if !u.MatchValue(in) {
			return nil
		}

		secret := &Secret{
			Kind:     u.Name,
			Data:     map[string]string{"match": in},
			Severity: u.Severity,
		}

		parent := n.Parent()
		if parent == nil || parent.Type() != "pair" {
			return secret
		}

		grandParent := parent.Parent()
		if grandParent == nil || grandParent.Type() != "object" {
			return secret
		}

		secret.Context = grandParent.AsObject().AsMap()

		return secret
	}}
}

// UserPatterns is an alias for a slice of *UserPattern
type UserPatterns []*UserPattern

// SecretMatchers returns a slice of SecretMatcher for use with
// (*Analyzer).AddSecretMatchers()
func (u UserPatterns) SecretMatchers() []SecretMatcher {
	out := make([]SecretMatcher, 0)

	for _, p := range u {
		out = append(out, p.SecretMatcher())
	}
	return out
}

// ParseUserPatterns accepts an io.Reader pointing to a JSON user-pattern
// definition file, and returns a list of UserPatterns, and any error that
// occurred.
func ParseUserPatterns(r io.Reader) (UserPatterns, error) {
	out := make(UserPatterns, 0)

	dec := json.NewDecoder(r)
	err := dec.Decode(&out)
	if err != nil {
		return out, err
	}

	for _, p := range out {
		err = p.ParseRegex()
		if err != nil {
			return out, err
		}
	}

	return out, nil
}
