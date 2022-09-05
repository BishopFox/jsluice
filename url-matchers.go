package jsluice

import (
	"net/url"
	"regexp"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// A URL is any URL found in the source code with accompanying details
type URL struct {
	URL         string            `json:"url"`
	QueryParams []string          `json:"queryParams"`
	BodyParams  []string          `json:"bodyParams"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers,omitempty"`
	ContentType string            `json:"contentType,omitempty"`

	// some description like locationAssignment, fetch, $.post or something like that
	Type string `json:"type"`

	// full source/content of the node; is optional
	Source string `json:"source,omitempty"`

	// the filename in which the match was found
	Filename string `json:"filename,omitempty"`
}

// GetURLs searches the JavaScript source code for absolute and relative URLs and returns
// a slice of results.
func (a *Analyzer) GetURLs() []*URL {

	matches := make([]*URL, 0)

	re := regexp.MustCompile("[^A-Z-a-z]")

	// function to run on entry to each node in the tree
	enter := func(n *sitter.Node) {

		for _, matcher := range a.urlMatchers {
			if matcher.Type != n.Type() {
				continue
			}

			match := matcher.Fn(n, a.source)
			if match == nil {
				continue
			}

			// decode any escapes in the URL
			match.URL = DecodeString(match.URL)

			// an empty slice is easier to deal with than null, e.g when using jq
			if match.QueryParams == nil {
				match.QueryParams = []string{}
			}
			if match.BodyParams == nil {
				match.BodyParams = []string{}
			}

			// Filter out data: and tel: schemes
			lower := strings.ToLower(match.URL)
			if strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "tel:") {
				continue
			}

			// Look for URLs that are entirely made up of EXPR replacements
			// and skip them. Maybe this should be optional? Maybe it should
			// remove things like EXPR#EXPR etc too
			letters := re.ReplaceAllString(match.URL, "")
			if strings.ReplaceAll(letters, "EXPR", "") == "" {
				continue
			}

			// Parse any query params out of the URL and add them. Some, but not
			// all of the matchers will add query params, so we want to do it here
			// and then remove duplicates
			u, err := url.Parse(match.URL)
			if err == nil {
				// manually disallow www.w3.org just because it shows up so damn often
				if u.Hostname() == "www.w3.org" {
					continue
				}

				for p, _ := range u.Query() {
					// Ignore params that were expressions
					if p == "EXPR" {
						continue
					}
					match.QueryParams = append(match.QueryParams, p)
				}
			}
			match.QueryParams = unique(match.QueryParams)

			matches = append(matches, match)
		}
	}

	// find the nodes we need in the the tree and run the enter function for every node
	query(a.rootNode, "[(assignment_expression) (call_expression) (string)] @matches", enter)

	return matches
}

func unique[T comparable](items []T) []T {
	set := make(map[T]any)
	for _, item := range items {
		set[item] = struct{}{}
	}
	out := make([]T, len(set))
	i := 0
	for item, _ := range set {
		out[i] = item
		i++
	}
	return out
}

// A URLMatcher has a type of thing it matches against (e.g. assignment_expression),
// and a function to actually do the matching and producing of the *URL
type URLMatcher struct {
	Type string
	Fn   func(*sitter.Node, []byte) *URL
}

// AllURLMatchers returns the detault list of URLMatchers
func AllURLMatchers() []URLMatcher {

	assignmentNames := newSet([]string{
		"location",
		"this.url",
		"this._url",
		"this.baseUrl",
	})

	isInterestingAssignment := func(name string) bool {
		if assignmentNames.Contains(name) {
			return true
		}

		if strings.HasSuffix(name, ".href") {
			return true
		}

		if strings.HasSuffix(name, ".location") {
			return true
		}

		return false
	}

	matchers := []URLMatcher{
		// XMLHttpRequest.open(method, url)
		matchXHR(),

		// $.post, $.get, and $.ajax
		matchJQuery(),

		// location assignment
		{"assignment_expression", func(n *sitter.Node, source []byte) *URL {
			left := n.ChildByFieldName("left")
			right := n.ChildByFieldName("right")

			if !isInterestingAssignment(content(left, source)) {
				return nil
			}

			// We want to find values that at least *start* with a string of some kind.
			// This might be kind of useful to the crawler:
			//
			//   location.href = "/somePath/" + someVar;
			//
			// Where as this tends to end up being kind of useless:
			//
			//   location.href = someVar + "/somePath/";
			//
			// So while we might miss out on some things this way, they probably wouldn't
			// have been super useful to anything automated anyway.
			rightContent := content(right, source)
			if len(rightContent) < 2 {
				return nil
			}
			p := rightContent[0:1]
			if p != `"` && p != "'" && p != "`" {
				return nil
			}

			return &URL{
				URL:    cleanURL(right, source),
				Method: "GET",
				Type:   "locationAssignment",
				Source: content(n, source),
			}
		}},

		// location replacement
		{"call_expression", func(n *sitter.Node, source []byte) *URL {
			callName := content(n.ChildByFieldName("function"), source)

			if !strings.HasSuffix(callName, "location.replace") {
				return nil
			}

			arguments := n.ChildByFieldName("arguments")

			// check the argument contains at least one string literal
			if !hasDescendantOfType(arguments.NamedChild(0), "string") {
				return nil
			}

			return &URL{
				URL:    cleanURL(arguments.NamedChild(0), source),
				Method: "GET",
				Type:   "locationReplacement",
				Source: content(n, source),
			}
		}},

		// window.open(url)
		{"call_expression", func(n *sitter.Node, source []byte) *URL {
			callName := content(n.ChildByFieldName("function"), source)
			if callName != "window.open" && callName != "open" {
				return nil
			}
			arguments := n.ChildByFieldName("arguments")

			// check the argument contains at least one string literal
			if !hasDescendantOfType(arguments.NamedChild(0), "string") {
				return nil
			}

			return &URL{
				URL:    cleanURL(arguments.NamedChild(0), source),
				Method: "GET",
				Type:   "window.open",
				Source: content(n, source),
			}
			return nil
		}},

		// fetch(url, [init])
		{"call_expression", func(n *sitter.Node, source []byte) *URL {
			callName := content(n.ChildByFieldName("function"), source)
			if callName != "fetch" {
				return nil
			}
			arguments := n.ChildByFieldName("arguments")

			// check the argument contains at least one string literal
			if !hasDescendantOfType(arguments.NamedChild(0), "string") {
				return nil
			}

			init := newObject(arguments.NamedChild(1), source)

			return &URL{
				URL:         cleanURL(arguments.NamedChild(0), source),
				Method:      init.getString("method", "GET"),
				Headers:     init.getObject("headers").asMap(),
				ContentType: init.getObject("headers").getStringI("content-type", ""),
				Type:        "fetch",
				Source:      content(n, source),
			}
			return nil
		}},

		// string literals
		{"string", func(n *sitter.Node, source []byte) *URL {
			trimmed := dequote(content(n, source))

			if !MaybeURL(trimmed) {
				return nil
			}

			return &URL{
				URL:    trimmed,
				Type:   "stringLiteral",
				Source: content(n, source),
			}
		}},
	}

	return matchers
}
