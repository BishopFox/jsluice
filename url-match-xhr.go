package jsluice

import (
	"strings"
	"sync"

	"golang.org/x/exp/slices"
)

type nodeCache struct {
	sync.RWMutex
	data map[*Node][]*Node
}

func newNodeCache() *nodeCache {
	return &nodeCache{
		data: make(map[*Node][]*Node),
	}
}

func (c *nodeCache) set(k *Node, v []*Node) {
	c.Lock()
	c.data[k] = v
	c.Unlock()
}

func (c *nodeCache) get(k *Node) ([]*Node, bool) {
	c.RLock()
	v, exists := c.data[k]
	c.RUnlock()
	return v, exists
}

func matchXHR() URLMatcher {
	cache := newNodeCache()

	return URLMatcher{"call_expression", func(n *Node) *URL {
		callName := n.ChildByFieldName("function").Content()

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

		method := arguments.NamedChild(0).RawString()

		if !slices.Contains(
			[]string{"GET", "HEAD", "OPTIONS", "POST", "PUT", "PATCH", "DELETE"},
			method,
		) {
			return nil
		}

		urlArg := arguments.NamedChild(1)
		if !urlArg.IsStringy() {
			return nil
		}

		match := &URL{
			URL:    urlArg.CollapsedString(),
			Method: method,
			Type:   "XMLHttpRequest.open",
			Source: n.Content(),
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
		if !parent.IsValid() {
			return match
		}
		for {
			candidate := parent.Parent()
			if candidate == nil {
				break
			}
			parent = candidate
			pt := parent.Type()
			if pt == "function_declaration" ||
				pt == "function" ||
				pt == "arrow_function" {
				break
			}
		}

		// Look for call_expressions under the same parent as our .open call.
		// It's common to end up querying the exact same parent over and over
		// again, so we cache the results on a per-parent node basis.
		nodes := make([]*Node, 0)
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
			parent.Query(q, func(sibling *Node) {
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
			name := sibling.ChildByFieldName("function").Content()
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

			header := headerNode.RawString()
			if _, exists := headers[header]; exists {
				continue
			}

			var value string
			valueNode := args.NamedChild(1)
			if valueNode != nil && valueNode.Type() == "string" {
				value = valueNode.RawString()
			}

			headers[header] = value
		}

		match.Headers = headers

		return match
	}}
}
