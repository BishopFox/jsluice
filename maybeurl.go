package jsluice

import (
	"net/url"
	"strings"
)

var fileExtensions set

func init() {
	fileExtensions = newSet([]string{
		"js", "css", "html", "htm", "xhtml", "xlsx",
		"xls", "docx", "doc", "pdf", "rss", "xml",
		"php", "phtml", "asp", "aspx", "asmx", "ashx",
		"cgi", "pl", "rb", "py", "do", "jsp",
		"jspa", "json", "jsonp", "txt",
	})
}

func MaybeURL(in string) bool {
	// This should eliminate a pretty big percentage of
	// string literals that we find, and avoid spending
	// the resources on parsing them as URLs
	if !strings.ContainsAny(in, "/?.") {
		return false
	}

	// We want to be fairly restrictive to cut out things
	// like regex strings, blocks of HTML etc. We will miss
	// a handful of URLs this way, but that's probably
	// better than spitting out a ton of false-positives
	if strings.ContainsAny(in, " ()!<>'\"`{}^$,") {
		return false
	}

	// This could be prone to false positives, but it
	// seems that in the wild most strings that start
	// with a slash are actually paths
	if strings.HasPrefix(in, "/") {
		return true
	}

	// Let's attempt to parse it as a URL, so we can
	// do some analysis on the individual parts
	u, err := url.Parse(in)
	if err != nil {
		return false
	}

	// Valid-scheme?
	if u.Scheme != "" {
		s := strings.ToLower(u.Scheme)
		if s != "http" && s != "https" {
			return false
		}
	}

	// Valid-looking hostname?
	if len(strings.Split(u.Hostname(), ".")) > 1 {
		return true
	}

	// Valid query string with at least one value?
	for _, vv := range u.Query() {
		if len(vv) > 0 && len(vv[0]) > 0 {
			return true
		}
	}

	// Known file extensions is the last thing we want to
	// check so if there's no dot then it's a no from us.
	if !strings.ContainsAny(u.Path, ".") {
		return false
	}

	parts := strings.Split(u.Path, ".")
	ext := parts[len(parts)-1]

	return fileExtensions.Contains(ext)

}
