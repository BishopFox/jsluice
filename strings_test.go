package jsluice

import (
	"testing"
)

func TestStringDecode(t *testing.T) {
	cases := []struct {
		in       string
		expected string
	}{
		// middle
		{`"foo bar"`, `foo bar`},
		{`"foo\\bar"`, `foo\bar`},
		{`"foo\"bar"`, `foo"bar`},
		{`"foo\'bar"`, `foo'bar`},
		{`"foo\075bar"`, `foo=bar`},
		{`"foo\tbar"`, "foo\tbar"},
		{`"foo\vbar"`, "foo\vbar"},
		{`"foo\u003dbar"`, "foo=bar"},
		{`"foo\u{00000000003d}bar"`, "foo=bar"},

		// end
		{`"foo\075"`, `foo=`},
		{`"foo\x3d"`, `foo=`},
		{`"foo\\"`, `foo\`},

		// start
		{`"\075foo"`, `=foo`},
		{`"\x3dfoo"`, `=foo`},
		{`"\\foo"`, `\foo`},

		// pairs
		{`"\075\x3d"`, `==`},
		{`"\u{00000003d}\x3d"`, `==`},

		// Invalid
		{`"\poo"`, `poo`},
		{`"\u{0003doops"`, `=oops`},

		// real-world
		{`"/help/doc/user_ed.jsp?loc\x3dhelp\x26target\x3d"`, "/help/doc/user_ed.jsp?loc=help&target="},
	}

	for _, c := range cases {
		actual := DecodeString(c.in)
		if c.expected != actual {
			t.Errorf("Want %s for DecodeString(%s); have %s", c.expected, c.in, actual)
		}
	}
}

func BenchmarkDecodeString(b *testing.B) {
	inputs := []string{
		`"foo bar"`,
		`"foo\\bar"`,
		`"foo\"bar"`,
		`"foo\'bar"`,
		`"foo\075bar"`,
		`"foo\tbar"`,
		`"foo\vbar"`,
		`"foo\u003dbar"`,
		`"foo\u{00000000003d}bar"`,
		`"foo\075"`,
		`"foo\x3d"`,
		`"foo\\"`,
		`"\075foo"`,
		`"\x3dfoo"`,
		`"\\foo"`,
		`"\075\x3d"`,
		`"\u{00000003d}\x3d"`,
		`"\poo"`,
		`"\u{0003doops"`,
		`"/help/doc/user_ed.jsp?loc\x3dhelp\x26target\x3d"`,
	}
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			_ = DecodeString(input)
		}
	}

}
