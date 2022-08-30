package jsurls

import (
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	"golang.org/x/exp/slices"
)

type nodeCache struct {
	sync.RWMutex
	data map[*sitter.Node][]*sitter.Node
}

func newNodeCache() *nodeCache {
	return &nodeCache{
		data: make(map[*sitter.Node][]*sitter.Node),
	}
}

func (c *nodeCache) set(k *sitter.Node, v []*sitter.Node) {
	c.Lock()
	c.data[k] = v
	c.Unlock()
}

func (c *nodeCache) get(k *sitter.Node) ([]*sitter.Node, bool) {
	c.RLock()
	v, exists := c.data[k]
	c.RUnlock()
	return v, exists
}

func matchXHR() urlMatcher {
	cache := newNodeCache()

	return urlMatcher{"call_expression", func(n *sitter.Node, source []byte) *URL {
		callName := content(n.ChildByFieldName("function"), source)

		// We don't know what the XMLHttpRequest object will be called,
		// so we have to focus on just the .open bit
		if !strings.HasSuffix(callName, ".open") {
			return nil
		}

		// There's a bunch of different stuff we might have matched,
		// including window.open, so we're going to try and guess
		// based on the first argument being a valid HTTP method.
		// This will miss cases where the method is a variable.
		arguments := n.ChildByFieldName("arguments")

		method := dequote(content(arguments.NamedChild(0), source))

		if !slices.Contains(
			[]string{"GET", "HEAD", "OPTIONS", "POST", "PUT", "PATCH", "DELETE"},
			method,
		) {
			return nil
		}

		urlArg := arguments.NamedChild(1)
		if !isStringy(urlArg, source) {
			return nil
		}

		match := &URL{
			URL:    cleanURL(urlArg, source),
			Method: method,
			Type:   "XMLHttpRequest.open",
			Source: content(n, source),
		}

		// to find headers we need to look for calls to setRequestHeader() on
		// the same object as the .open call. We'll stick to the same scope
		// (i.e. sibling expressions) because we have no way to know if we're
		// dealing with the same object or not otherwise.
		objectName := strings.TrimSuffix(callName, ".open")

		// We want to find the parent/ancestor node that defines the scope in which
		// we are calling XHR.open(). JavaScript has three types of scope: global,
		// function, and block. Block scope only comes into play if values
		// are defined using 'let', or 'const'. We don't know if the XHR object
		// was defined with let or const, so we're just going to ignore block scope.
		// That leaves us with global scope and function scope. To find those we
		// can ascend the tree until we hit a node with type "function_declaration",
		// or we hit a nil parent.
		parent := n.Parent()
		if parent == nil {
			return match
		}
		for {
			candidate := parent.Parent()
			if candidate == nil {
				break
			}
			parent = candidate
			if parent.Type() == "function_declaration" {
				break
			}
		}

		// Look for call_expressions under the same parent as our .open call.
		// It's common to end up querying the exact same parent over and over
		// again, so we cache the results on a per-parent node basis.
		nodes := make([]*sitter.Node, 0)
		if v, exists := cache.get(parent); exists {
			nodes = v
		} else {
			q := `
				(call_expression
					function: (member_expression
						object: (identifier)
						property: (property_identifier)
					)
					arguments: (arguments (string))
				) @matches
			`
			query(parent, q, func(sibling *sitter.Node) {
				nodes = append(nodes, sibling)
			})
			cache.set(parent, nodes)
		}

		headers := make(map[string]string, 0)
		// TODO: I think we can get more accuracy here by relying on the fact that
		// the .setRequestHeader calls we're interested in must come *after* the .open
		// call in order to be valid. In theory that means we can skip any nodes at
		// all that come before the .open call we're currently looking at. We could
		// also stop looking after we see a .send call on the same object, although
		// it's possible for the .send to be wrapped in a conditional so that might
		// cause us to miss some values.
		for _, sibling := range nodes {
			name := content(sibling.ChildByFieldName("function"), source)
			if !strings.HasSuffix(name, ".setRequestHeader") {
				continue
			}

			if !strings.HasPrefix(name, objectName) {
				continue
			}

			args := sibling.ChildByFieldName("arguments")
			headerNode := args.NamedChild(0)
			if headerNode == nil || headerNode.Type() != "string" {
				continue
			}

			header := dequote(content(headerNode, source))
			if _, exists := headers[header]; exists {
				continue
			}

			var value string
			valueNode := args.NamedChild(1)
			if valueNode != nil && valueNode.Type() == "string" {
				value = dequote(content(valueNode, source))
			}

			headers[header] = value
		}

		match.Headers = headers

		return match
	}}
}
