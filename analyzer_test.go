package jsluice

import "testing"

func TestAnalyzerBasicURLs(t *testing.T) {
	a := NewAnalyzer([]byte(`
		function foo(){
			document.location = "/logout"
		}
	`))

	urls := a.GetURLs()

	if len(urls) != 1 {
		t.Errorf("Expected exactly 1 URL; got %d", len(urls))
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
