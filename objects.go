package jsluice

import (
	"strings"
)

// Object is a wrapper about a N ode that contains a JS Object
// It has convenience methods to find properties of the object,
// convert it to other types etc.
type Object struct {
	node   *Node
	source []byte
}

// NewObject returns a jsluice Object for the given Node
func NewObject(n *Node, source []byte) Object {
	return Object{
		node:   n,
		source: source,
	}
}

// AsMap returns a Go map version of the object
func (o Object) AsMap() map[string]string {
	out := make(map[string]string, 0)
	if !o.HasValidNode() {
		return out
	}

	for _, k := range o.GetKeys() {
		out[k] = o.GetString(k, "")
	}
	return out
}

// HasValidNode returns true if the underlying node is
// a valid JavaScript object
func (o Object) HasValidNode() bool {
	return o.node.IsValid() && o.node.Type() == "object"
}

// GetNodeFunc is a general-purpose method for finding object
// properties by their key. The provided function is called
// with each key in turn. The first time that function returns
// true the corresponding *Node for that key is returned.
func (o Object) GetNodeFunc(fn func(key string) bool) *Node {
	if !o.HasValidNode() {
		return &Node{}
	}

	count := int(o.node.NamedChildCount())

	for i := 0; i < count; i++ {
		pair := o.node.NamedChild(i)

		if pair.Type() != "pair" {
			continue
		}

		if !fn(pair.ChildByFieldName("key").RawString()) {
			continue
		}

		return pair.ChildByFieldName("value")
	}
	return nil
}

// GetNode returns the matching *Node for a given key
func (o Object) GetNode(key string) *Node {
	return o.GetNodeFunc(func(candidate string) bool {
		return key == candidate
	})
}

// GetNodeI is like GetNode, but case-insensitive
func (o Object) GetNodeI(key string) *Node {
	key = strings.ToLower(key)
	return o.GetNodeFunc(func(candidate string) bool {
		return key == strings.ToLower(candidate)
	})
}

// GetKeys returns a slice of all keys in an object
func (o Object) GetKeys() []string {
	out := make([]string, 0)
	if !o.HasValidNode() {
		return out
	}

	count := int(o.node.NamedChildCount())

	for i := 0; i < count; i++ {
		pair := o.node.NamedChild(i)

		if pair.Type() != "pair" {
			continue
		}

		key := pair.ChildByFieldName("key").RawString()
		out = append(out, key)
	}
	return out
}

// GetObject returns the property corresponding to the
// provided key as an Object
func (o Object) GetObject(key string) Object {
	return NewObject(o.GetNode(key), o.source)
}

// GetString returns the property corresponding to the
// provided key as a string, or the defaultVal if the
// key is not found.
func (o Object) GetString(key, defaultVal string) string {
	value := o.GetNode(key)
	if value == nil || value.Type() != "string" {
		return defaultVal
	}
	return value.RawString()
}

// GetStringI is like GetString, but the key is case-insensitive
func (o Object) GetStringI(key, defaultVal string) string {
	value := o.GetNodeI(key)
	if value == nil || value.Type() != "string" {
		return defaultVal
	}
	return value.RawString()
}
