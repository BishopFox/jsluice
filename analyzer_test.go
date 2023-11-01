package jsluice

import "testing"

func TestAnalyzerBasicURLs(t *testing.T) {
	a := NewAnalyzer([]byte(`
		function foo(){
			document.location = "/logout"
		}
	`))

	urls := a.GetURLs()

	if len(urls) < 1 {
		t.Errorf("Expected at least 1 URL; got %d", len(urls))
	}

	if urls[0].URL != "/logout" {
		t.Errorf("Expected first URL to be '/logout'; got %s", urls[0].URL)
	}
}

func TestAnalyzerBasicSecrets(t *testing.T) {
	a := NewAnalyzer([]byte(`
		function foo(){
			return {
				awsKey: "AKIAIOSFODNN7EXAMPLE",
				secret: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
			}
		}
	`))

	secrets := a.GetSecrets()

	if len(secrets) != 1 {
		t.Errorf("Expected exactly 1 secret; got %d", len(secrets))
	}

	if secrets[0].Kind != "AWSAccessKey" {
		t.Errorf("Expected first secret kind to be AWSAccessKey; got %s", secrets[0].Kind)
	}
}

func TestIsProbablyHTML(t *testing.T) {
	cases := []struct {
		in       []byte
		expected bool
	}{
		{[]byte("var foo = bar"), false},
		{[]byte(" \t\nvar foo = bar"), false},
		{[]byte("lol this isn't even JavaScript"), false},
		{[]byte("<!doctype html><html>"), true},
		{[]byte(" \t\n<div><p>"), true},
	}

	for _, c := range cases {
		actual := isProbablyHTML(c.in)

		if actual != c.expected {
			t.Errorf("want %t for isProbablyHTML(%q); have %t", c.expected, c.in, actual)
		}
	}
}
