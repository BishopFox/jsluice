# jsluice

[![Go Reference](https://pkg.go.dev/badge/github.com/BishopFox/jsluice.svg)](https://pkg.go.dev/github.com/BishopFox/jsluice)

`jsluice` is a Go package and [command-line tool](/cmd/jsluice/) for extracting URLs, paths, secrets,
and other interesting data from JavaScript source code.

If you want to do those things right away: look at the [command-line tool](/cmd/jsluice/).

If you want to integrate `jsluice`'s capabilities with your own project: look at the [examples](/examples/),
and read the [package documentation](https://pkg.go.dev/github.com/BishopFox/jsluice).

## Install

To install the command-line tool, run:

```
▶ go install github.com/BishopFox/jsluice/cmd/jsluice@latest
```

To add the package to your project, run:

```
▶ go get github.com/BishopFox/jsluice
```

## Extracting URLs

Rather than using regular expressions alone, `jsluice` uses `go-tree-sitter` to look for places that URLs are known to be used,
such as being assigned to `document.location`, passed to `window.open()`, or passed to `fetch()` etc.

A simple example program is provided [here](/examples/basic/main.go):

```go
analyzer := jsluice.NewAnalyzer([]byte(`
    const login = (redirect) => {
        document.location = "/login?redirect=" + redirect + "&method=oauth"
    }
`))

for _, url := range analyzer.GetURLs() {
    j, err := json.MarshalIndent(url, "", "  ")
    if err != nil {
        continue
    }

    fmt.Printf("%s\n", j)
}
```

Running the example:
```
▶ go run examples/basic/main.go
{
  "url": "/login?redirect=EXPR\u0026method=oauth",
  "queryParams": [
    "method",
    "redirect"
  ],
  "bodyParams": [],
  "method": "GET",
  "type": "locationAssignment",
  "source": "document.location = \"/login?redirect=\" + redirect + \"\u0026method=oauth\""
}
```

Note that the value of the `redirect` query string parameter is `EXPR`.
Code like this is common in JavaScript:

```javascript
document.location = "/login?redirect=" + redirect + "&method=oauth"
```

`jsluice` understands string concatenation, and replaces any expressions it cannot know the value
of with `EXPR`. Although not a foolproof solution, this approach results in a valid URL or path
more often than not, and means that it's possible to discover things that aren't easily found using
other approaches. In this case, a naive regular expression may well miss the `method` query string
parameter:

```
▶ JS='document.location = "/login?redirect=" + redirect + "&method=oauth"'
▶ echo $JS | grep -oE 'document\.location = "[^"]+"'
document.location = "/login?redirect="
```

### Custom URL Matchers

`jsluice` comes with some built-in URL matchers for common scenarios, but you can add more
with the `AddURLMatcher` function:

```go
analyzer := jsluice.NewAnalyzer([]byte(`
    var fn = () => {
        var meta = {
            contact: "mailto:contact@example.com",
            home: "https://example.com"
        }
        return meta
    }
`))

analyzer.AddURLMatcher(
    // The first value in the jsluice.URLMatcher struct is the type of node to look for.
    // It can be one of "string", "assignment_expression", or "call_expression"
    jsluice.URLMatcher{"string", func(n *jsluice.Node) *jsluice.URL {
        val := n.DecodedString()
        if !strings.HasPrefix(val, "mailto:") {
            return nil
        }

        return &jsluice.URL{
            URL:  val,
            Type: "mailto",
        }
    }},
)

for _, match := range analyzer.GetURLs() {
    fmt.Println(match.URL)
}
```

There's a copy of this example [here](/examples/urlmatcher/main.go). You can run it like this:

```
▶ go run examples/urlmatcher/main.go
mailto:contact@example.com
https://example.com
```

`jsluice` doesn't match `mailto:` URIs by default, it was found by the custom `URLMatcher`.


## Extracting Secrets

As well as URLs, `jsluice` can extract secrets. As with URL extraction, custom matchers can
be supplied to supplement the default matchers. There's a short example program [here](/examples/secrets/main.go)
that does just that:

```go
analyzer := jsluice.NewAnalyzer([]byte(`
    var config = {
        apiKey: "AUTH_1a2b3c4d5e6f",
        apiURL: "https://api.example.com/v2/"
    }
`))

analyzer.AddSecretMatcher(
    // The first value in the jsluice.SecretMatcher struct is a
    // tree-sitter query to run on the JavaScript source.
    jsluice.SecretMatcher{"(pair) @match", func(n *jsluice.Node) *jsluice.Secret {
        key := n.ChildByFieldName("key").DecodedString()
        value := n.ChildByFieldName("value").DecodedString()

        if !strings.Contains(key, "api") {
            return nil
        }

        if !strings.HasPrefix(value, "AUTH_") {
            return nil
        }

        return &jsluice.Secret{
            Kind: "fakeApi",
            Data: map[string]string{
                "key":   key,
                "value": value,
            },
            Severity: jsluice.SeverityLow,
            Context:  n.Parent().AsMap(),
        }
    }},
)

for _, match := range analyzer.GetSecrets() {
    j, err := json.MarshalIndent(match, "", "  ")
    if err != nil {
        continue
    }

    fmt.Printf("%s\n", j)
}
```

Running the example:

```
▶ go run examples/secrets/main.go
[2023-06-14T13:04:16+0100]
{
  "kind": "fakeApi",
  "data": {
    "key": "apiKey",
    "value": "AUTH_1a2b3c4d5e6f"
  },
  "severity": "low",
  "context": {
    "apiKey": "AUTH_1a2b3c4d5e6f",
    "apiURL": "https://api.example.com/v2/"
  }
}
```

Because we have a syntax tree available for the entire JavaScript source,
it was possible to inspect both the `key` and `value`, and also to easily
provide the parent object as context for the match.
