package jsurls

import "testing"

func TestAnalyzerBasic(t *testing.T) {
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
