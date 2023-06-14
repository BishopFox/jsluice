package jsluice

import (
	"strings"
	"testing"
)

func TestParseUserPatterns(t *testing.T) {
	testData := strings.NewReader(`[
		{"name": "httpAuth", "value": "/[a-z0-9_/\\.:-]+@[a-z0-9-]+\\.[a-z0-9.-]+"},
		{"name": "base64", "value": "^(eyJ|YTo|Tzo|PD[89]|aHR0cHM6L|aHR0cDo|rO0)[%a-zA-Z0-9+/]+={0,2}"}
	]`)

	patterns, err := ParseUserPatterns(testData)

	if err != nil {
		t.Errorf("want nil error for ParseUserPatterns(testData); have %s", err)
	}

	if len(patterns) != 2 {
		t.Errorf("want 2 patterns from ParseUserPatterns(testData); have %d", len(patterns))
	}

	cases := []struct {
		i        int
		in       string
		expected bool
	}{
		{0, "//someuser:somepass@example.com", true},
		{0, "https://someuser:somepass@example.com", true},
		{0, "person@example.com", false},
		{1, "eyJmb28iOiAxMjN9Cg==", true},
		{1, "eyJ:-)b28iOiAxMjN9Cg==", false},
		{1, "foobareyJmb28iOiAxMjN9Cg==", false},
	}

	for _, c := range cases {
		if patterns[c.i].MatchValue(c.in) != c.expected {
			t.Errorf(
				"Want %t for (%s).MatchValue(%s); have %t",
				c.expected, patterns[c.i].reValue, c.in, !c.expected,
			)
		}
	}
}

func TestParseUserPatternsBadPattern(t *testing.T) {
	testData := strings.NewReader(`[
		{"name": "httpAuth", "pattern": "/[a-z0-9_/\\.:-]+@[a-z0-9-]+\\.[a-z0-9.-]+"},
		{"name": "base64", "pattern": "^(eyJ|YTo|Tzo|PD[89]|aHR0cHM6L|aHR0cDo|rO0[%a-zA-Z0-9+/]+={0,2}"}
	]`)

	_, err := ParseUserPatterns(testData)

	if err == nil {
		t.Error("want non-nil error for ParseUserPatterns(testData) with bad pattern; but have nil", err)
	}
}

func TestParseUserPatternsBadJSON(t *testing.T) {
	testData := strings.NewReader(`[
		{"name": "httpAuth", "pattern": "/[a-z0-9_/\\.:-]+@[a-z0-9-]+\\.[a-z0-9.-]+"},
		{"name": "base64", "pattern": "^(eyJ|YTo|Tzo|PD
	]`)

	_, err := ParseUserPatterns(testData)

	if err == nil {
		t.Error("want non-nil error for ParseUserPatterns(testData) with bad JSON; but have nil", err)
	}
}
