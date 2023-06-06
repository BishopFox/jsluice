package jsluice

import (
	"encoding/json"
	"errors"
	"io"
	"regexp"
)

type UserPattern struct {
	Name        string   `json:"name"`
	Pattern     string   `json:"pattern"`
	NamePattern string   `json:"namePattern"`
	Severity    Severity `json:"severity"`

	re     *regexp.Regexp
	reName *regexp.Regexp
}

func (u *UserPattern) ParseRegex() error {
	if u.Pattern != "" {
		re, err := regexp.Compile(u.Pattern)
		if err != nil {
			return err
		}
		u.re = re
	}

	if u.NamePattern != "" {
		re, err := regexp.Compile(u.NamePattern)
		if err != nil {
			return err
		}
		u.reName = re
	}

	if u.Severity == "" {
		u.Severity = SeverityInfo
	}

	if u.re == nil && u.reName == nil {
		return errors.New("pattern, namePattern, or both must be supplied in user-defined matcher")
	}

	return nil
}

func (u *UserPattern) Match(in string) bool {
	if u.re == nil {
		return true
	}
	return u.re.MatchString(in)
}

func (u *UserPattern) MatchName(in string) bool {
	if u.reName == nil {
		return true
	}
	return u.reName.MatchString(in)
}

func (u *UserPattern) SecretMatcher() SecretMatcher {
	if u.reName != nil {
		return u.pairMatcher()
	}

	return u.stringMatcher()
}

func (u *UserPattern) pairMatcher() SecretMatcher {
	return SecretMatcher{"(pair) @matches", func(n *Node) *Secret {

		key := n.ChildByFieldName("key")
		if key == nil || !u.MatchName(key.RawString()) {
			return nil
		}

		value := n.ChildByFieldName("value")
		if value == nil || value.Type() != "string" {
			return nil
		}

		if !u.Match(value.RawString()) {
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

		secret.Context = parent.AsObject().asMap()

		return secret

	}}
}

func (u *UserPattern) stringMatcher() SecretMatcher {
	return SecretMatcher{"(string) @matches", func(n *Node) *Secret {
		in := n.RawString()
		if !u.Match(in) {
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

		secret.Context = grandParent.AsObject().asMap()

		return secret
	}}
}

type UserPatterns []*UserPattern

func (u UserPatterns) SecretMatchers() []SecretMatcher {
	out := make([]SecretMatcher, 0)

	for _, p := range u {
		out = append(out, p.SecretMatcher())
	}
	return out
}

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
