package jsluice

import (
	"strings"

	"golang.org/x/exp/slices"
)

func matchJQuery() URLMatcher {

	return URLMatcher{"call_expression", func(n *Node, source []byte) *URL {
		callName := n.ChildByFieldName("function").Content()

		if !slices.Contains(
			[]string{
				"$.get", "$.post", "$.ajax",
				"jQuery.get", "jQuery.post", "jQuery.ajax",
			},
			callName,
		) {
			return nil
		}

		// The jQuery ajax calls have a few different call signatures
		// that we need to account for:
		//   jQuery.post( url [, data ] [, success ] [, dataType ] )
		//   jQuery.get( url [, data ] [, success ] [, dataType ] )
		//   jQuery.ajax( url [, settings ] )
		//   jQuery.post( [settings] )
		//   jQuery.get( [settings] )
		//   jQuery.ajax( [settings] )
		//
		// So we end up with three scenarios to deal with:
		//   1. The URL comes first, then a data object
		//   2. The URL comes first, then a settings object
		//   3. A settings object comes first.
		arguments := n.ChildByFieldName("arguments")
		if arguments == nil {
			return nil
		}

		firstArg := arguments.NamedChild(0)
		if firstArg == nil {
			return nil
		}
		secondArg := arguments.NamedChild(1)

		m := &URL{
			Type:   callName,
			Source: n.Content(),
		}

		// Infer the method for .post and .get calls
		if strings.HasSuffix(callName, ".post") {
			m.Method = "POST"
		} else if strings.HasSuffix(callName, ".get") {
			m.Method = "GET"
		}

		var settingsNode *Node

		if firstArg.IsStringy() {
			// first argument is the URL
			m.URL = firstArg.CollapsedString()

			// If the first arg is a URL, the second arg is a
			// settings object for $.ajax, or a data object for
			// $.get and $.post
			if strings.HasSuffix(callName, ".ajax") {
				settingsNode = secondArg
			} else {
				params := newObject(secondArg, source).getKeys()
				if m.Method == "GET" {
					m.QueryParams = params
				} else {
					m.BodyParams = params
				}
			}
		}

		if firstArg.Type() == "object" {
			// first argument is a settings object
			settingsNode = firstArg
		}

		if settingsNode == nil {
			// we didn't end up with a settings node,
			// so we can't infer anything else
			return m
		}

		settings := newObject(settingsNode, source)

		if m.URL == "" {
			m.URL = settings.getNode("url").CollapsedString()
		}

		m.Headers = settings.getObject("headers").asMap()

		if m.Method == "" {
			// method can be specified as either `method`, or
			// `type`, and defaults to GET
			m.Method = settings.getString(
				"method",
				settings.getString("type", "GET"),
			)
		}

		params := settings.getObject("data").getKeys()
		if m.Method == "GET" {
			m.QueryParams = params
		} else {
			m.BodyParams = params
		}

		return m
	}}
}
