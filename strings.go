package jsluice

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type item struct {
	typ itemType
	val string
}

type itemType int

const (
	itemString itemType = iota
	itemSingleEscape
	itemHexEscape
	itemOctalEscape
	itemUnicodeEscape
	itemCodepointEscape
)

func (i item) String() string {
	switch i.typ {
	case itemString:
		return i.val
	case itemSingleEscape:
		escapes := map[string]string{
			"b": "\b",
			"f": "\f",
			"n": "\n",
			"r": "\r",
			"t": "\t",
			"v": "\v",
		}

		if out, exists := escapes[i.val]; exists {
			return out
		}
		return i.val
	case itemHexEscape, itemUnicodeEscape, itemCodepointEscape:
		num, err := strconv.ParseInt(i.val, 16, 0)
		if err != nil {
			return i.val
		}
		return string(rune(num))
	case itemOctalEscape:
		num, err := strconv.ParseInt(i.val, 8, 0)
		if err != nil {
			return i.val
		}
		return string(rune(num))
	default:
		return i.val
	}
}

type stringLexer struct {
	str   string
	start int
	pos   int
	items []item
	done  bool
}

func newStringLexer(in string) *stringLexer {
	return &stringLexer{
		str:   in,
		start: 0,
		pos:   0,
		items: make([]item, 0),
		done:  false,
	}
}

func (s *stringLexer) Next() rune {
	if s.pos >= len(s.str) {
		s.done = true
		return -1
	}

	r, l := utf8.DecodeRuneInString(s.str[s.pos:])
	s.pos += l
	return r
}

func (s *stringLexer) Backup() {
	if s.done || s.pos <= 0 {
		return
	}
	_, l := utf8.DecodeLastRuneInString(s.str[:s.pos])
	s.pos -= l
}

func (s *stringLexer) Peek() rune {
	r := s.Next()
	s.Backup()
	return r
}

func (s *stringLexer) Emit(t itemType) {
	s.items = append(s.items, item{
		typ: t,
		val: s.str[s.start:s.pos],
	})
	s.start = s.pos
}

func (s *stringLexer) Ignore() {
	s.start = s.pos
}

func (s *stringLexer) Accept(valid string) bool {
	if strings.ContainsRune(valid, s.Next()) {
		return true
	}
	s.Backup()
	return false
}

func (s *stringLexer) AcceptN(valid string, n int) bool {
	count := 0
	for i := 0; i < n; i++ {
		if s.Accept(valid) {
			count++
		}
	}
	return count == n
}

func (s *stringLexer) AcceptUntil(r rune) {
	for s.Next() != r && !s.done {
	}
	s.Backup()
}

func (s *stringLexer) AcceptRun(valid string) {
	for strings.ContainsRune(valid, s.Next()) {
	}
	s.Backup()
}

func (s *stringLexer) String() string {
	out := &strings.Builder{}
	for _, i := range s.items {
		out.WriteString(i.String())
	}
	return out.String()
}

func DecodeString(in string) string {
	in = dequote(in)
	l := newStringLexer(in)

	validHex := "0123456789abcdefABCDEF"

	for !l.done {
		l.AcceptUntil('\\')
		l.Emit(itemString)

		if l.done {
			break
		}

		// Ignore the backslash
		l.Next()
		l.Ignore()

		switch l.Next() {
		case 'b', 'f', 'n', 'r', 't', 'v', '\'', '"', '\\':
			l.Emit(itemSingleEscape)
		case '0':
			// It's a \0 (null)
			if !unicode.IsDigit(l.Peek()) {
				l.Emit(itemSingleEscape)
				continue
			}
			// It's an octal escape
			l.AcceptRun("01234567")
			l.Emit(itemOctalEscape)
		case 'x':
			// ignore the x
			l.Ignore()

			// Exactly 2 hex digits
			if l.AcceptN(validHex, 2) {
				l.Emit(itemHexEscape)
			}
		case 'u':
			// ignore the u
			l.Ignore()

			// e.g. \u{00003d}
			if l.Accept("{") {
				l.Ignore()
				l.AcceptRun(validHex)
				l.Emit(itemCodepointEscape)
				if l.Accept("}") {
					l.Ignore()
				}
			}

			// e.g. \u003d
			if l.AcceptN(validHex, 4) {
				l.Emit(itemUnicodeEscape)
			}

		}
	}

	return l.String()
}
