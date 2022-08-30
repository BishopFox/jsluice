package jsurls

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

type object struct {
	node   *sitter.Node
	source []byte
}

func newObject(n *sitter.Node, source []byte) object {
	return object{
		node:   n,
		source: source,
	}
}

func (o object) asMap() map[string]string {
	out := make(map[string]string, 0)
	if !o.hasValidNode() {
		return out
	}

	for _, k := range o.getKeys() {
		out[k] = o.getString(k, "")
	}
	return out
}

func (o object) hasValidNode() bool {
	return o.node != nil && o.node.Type() == "object"
}

func (o object) getNodeFunc(fn func(key string) bool) *sitter.Node {
	if !o.hasValidNode() {
		return nil
	}

	count := int(o.node.NamedChildCount())

	for i := 0; i < count; i++ {
		pair := o.node.NamedChild(i)

		if pair.Type() != "pair" {
			continue
		}

		if !fn(dequote(content(pair.ChildByFieldName("key"), o.source))) {
			continue
		}

		return pair.ChildByFieldName("value")
	}
	return nil
}

func (o object) getNode(key string) *sitter.Node {
	return o.getNodeFunc(func(candidate string) bool {
		return key == candidate
	})
}

func (o object) getNodeI(key string) *sitter.Node {
	key = strings.ToLower(key)
	return o.getNodeFunc(func(candidate string) bool {
		return key == strings.ToLower(candidate)
	})
}

func (o object) getKeys() []string {
	out := make([]string, 0)
	if !o.hasValidNode() {
		return out
	}

	count := int(o.node.NamedChildCount())

	for i := 0; i < count; i++ {
		pair := o.node.NamedChild(i)

		if pair.Type() != "pair" {
			continue
		}

		key := dequote(content(pair.ChildByFieldName("key"), o.source))
		out = append(out, key)
	}
	return out
}

func (o object) getObject(key string) object {
	return newObject(o.getNode(key), o.source)
}

func (o object) getString(key, defaultVal string) string {
	value := o.getNode(key)
	if value == nil || value.Type() != "string" {
		return defaultVal
	}
	return dequote(content(value, o.source))
}

func (o object) getStringI(key, defaultVal string) string {
	value := o.getNodeI(key)
	if value == nil || value.Type() != "string" {
		return defaultVal
	}
	return dequote(content(value, o.source))
}
