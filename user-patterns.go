package jsluice

import (
	"encoding/json"
	"io"
	"regexp"
)

type UserPattern struct {
	Name    string `json:"name"`
	Pattern string `json:"pattern"`

	re *regexp.Regexp
}

func (u *UserPattern) Match(in string) bool {
	return u.re.MatchString(in)
}

type UserPatterns []*UserPattern

func (u UserPatterns) SecretMatcher() SecretMatcher {
	return SecretMatcher{"(string) @matches", func(n *Node) *Secret {
		for _, p := range u {
			in := n.RawString()
			if !p.Match(in) {
				continue
			}

			return &Secret{
				Kind: p.Name,
				Data: map[string]string{"match": in},
			}
		}
		return nil
	}}
}

func ParseUserPatterns(r io.Reader) (UserPatterns, error) {
	out := make(UserPatterns, 0)

	dec := json.NewDecoder(r)
	err := dec.Decode(&out)
	if err != nil {
		return out, err
	}

	for _, p := range out {
		re, err := regexp.Compile(p.Pattern)
		if err != nil {
			return out, err
		}

		p.re = re
	}

	return out, nil
}
