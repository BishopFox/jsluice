package jsurls

import (
	"strconv"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
)

func TestCleanURL(t *testing.T) {
	cases := []struct {
		JS       []byte
		Expected string
	}{
		{[]byte(`"./login.php?redirect="+url`), "./login.php?redirect=EXPR"},
		{[]byte(`'/path/'+['one', 'two', 'three'].join('/')`), "/path/EXPR"},
		{[]byte(`someVar`), "EXPR"},
	}

	parser := sitter.NewParser()
	parser.SetLanguage(javascript.GetLanguage())

	for i, c := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			tree := parser.Parse(nil, c.JS)
			root := tree.RootNode()

			// Example tree:
			//   program
			//     expression_statement
			//       binary_expression
			//         left: string ("./login.php?redirect=")
			//         right: identifier (url)
			//
			// We want the binary_expression to pass to cleanURL, which is
			// the first Named Child of the first Named Child of the root node.
			actual := cleanURL(root.NamedChild(0).NamedChild(0), c.JS)

			if actual != c.Expected {
				t.Errorf("want %s for cleanURL(%s), have: %s", c.Expected, c.JS, actual)
			}
		})
	}
}
