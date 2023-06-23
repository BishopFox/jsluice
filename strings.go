package jsluice

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// An item represents a token in a JavaScript string
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

// a stringLexer maintains the state needed to lex a string
type stringLexer struct {
	str   string // The input string
	start int    // The start position (in bytes) of the current token being lexed
	pos   int    // The current position (in bytes) of the rune being looked at
	items []item // A slice of tokens that have been emitted
	done  bool   // Flag that's set when we've consumed all of the input
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

// Next returns the next rune in the input string, moving the
// position pointer on by the size of the rune that was decoded.
// For ASCII text this wouldn't be required, but in JavaScript
// source we will encounter many runes that have a length of more
// than one byte
func (s *stringLexer) Next() rune {
	if s.pos >= len(s.str) {
		s.done = true
		return -1
	}

	r, l := utf8.DecodeRuneInString(s.str[s.pos:])
	s.pos += l
	return r
}

// Backup moves the position pointer back by the length of the
// previous rune in the input string.
func (s *stringLexer) Backup() {
	if s.done || s.pos <= 0 {
		return
	}
	_, l := utf8.DecodeLastRuneInString(s.str[:s.pos])
	s.pos -= l
}

// Peek returns the next rune in the input string without advancing
// the position pointer
func (s *stringLexer) Peek() rune {
	r := s.Next()
	s.Backup()
	return r
}

// Emit adds a token of the provided type to the stringLexer's
// internal list of tokens. The start pointer is advanced to the
// current position.
func (s *stringLexer) Emit(t itemType) {
	s.items = append(s.items, item{
		typ: t,
		val: s.str[s.start:s.pos],
	})
	s.start = s.pos
}

// Ignore moves the start position pointer to the current
// position without emitting a token; effectively ignoring
// the chunk of text between the last token we emitted and now.
func (s *stringLexer) Ignore() {
	s.start = s.pos
}

// Accept advances the position pointer only if the next rune
// is in the set of valid runes provided
func (s *stringLexer) Accept(valid string) bool {
	if strings.ContainsRune(valid, s.Next()) {
		return true
	}
	s.Backup()
	return false
}

// AcceptN advances the position pointer N times, only if
// the next N runes are in the set of valid runes provided
func (s *stringLexer) AcceptN(valid string, n int) bool {
	count := 0
	for i := 0; i < n; i++ {
		if s.Accept(valid) {
			count++
		}
	}
	return count == n
}

// AcceptUntil accepts any runes until the rune provided is
// encountered
func (s *stringLexer) AcceptUntil(r rune) {
	for s.Next() != r && !s.done {
	}
	s.Backup()
}

// AcceptRun accepts runes until encountering a rune not
// in the set of valid runes provided
func (s *stringLexer) AcceptRun(valid string) {
	for strings.ContainsRune(valid, s.Next()) {
	}
	s.Backup()
}

// String returns the unescaped representation of the input
// that has been lexed so far. Usually it would only be
// called after all the input has been processed.
func (s *stringLexer) String() string {
	out := &strings.Builder{}
	for _, i := range s.items {
		out.WriteString(i.String())
	}
	return out.String()
}

// DecodeString accepts a raw string as it might be found in some
// JavaScript source code, and converts any escape sequences. E.g:
//   foo\x3dbar -> foo=bar // Hex escapes
//   foo\u003Dbar -> foo=bar // Unicode escapes
//   foo\u{003D}bar -> foo=bar // Braced unicode escapes
//   foo\075bar -> foo=bar // Octal escape
//   foo\"bar -> foo"bar // Single character escapes
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
