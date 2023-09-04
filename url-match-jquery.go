package jsluice

import (
	"strings"

	"golang.org/x/exp/slices"
)

func matchJQuery() URLMatcher {

	return URLMatcher{"call_expression", func(n *Node) *URL {
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
				params := secondArg.AsObject().GetKeys()
				if m.Method == "GET" {
					m.QueryParams = params
				} else {
					m.BodyParams = params
					m.ContentType = "application/x-www-form-urlencoded; charset=UTF-8"
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

		settings := settingsNode.AsObject()

		if m.URL == "" {
			m.URL = settings.GetNode("url").CollapsedString()
		}

		headers := settings.GetObject("headers")
		m.Headers = headers.AsMap()

		if m.Method == "" {
			// method can be specified as either `method`, or
			// `type`, and defaults to GET
			m.Method = settings.GetString(
				"method",
				settings.GetString("type", "GET"),
			)
		}

		params := settings.GetObject("data").GetKeys()
		if m.Method == "GET" {
			m.QueryParams = params
		} else {
			m.BodyParams = params
		}

		if m.Method != "GET" {
			ct := headers.GetStringI("content-type", "")
			if ct == "" {
				ct = settings.GetString(
					"contentType",
					"application/x-www-form-urlencoded; charset=UTF-8",
				)
			}
			m.ContentType = ct
		}

		return m
	}}
}
