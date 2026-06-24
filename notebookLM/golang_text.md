# Domain Architecture: text

## Layout Topology
```text
text/
├── scanner
│   └── scanner.go
├── tabwriter
│   └── tabwriter.go
└── template
    ├── parse
    │   ├── lex.go
    │   ├── node.go
    │   └── parse.go
    ├── doc.go
    ├── exec.go
    ├── funcs.go
    ├── helper.go
    ├── option.go
    └── template.go
```

## Source Stream Aggregation

// === FILE: references/go/src/text/scanner/scanner.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package scanner provides a scanner and tokenizer for UTF-8-encoded text.
// It takes an io.Reader providing the source, which then can be tokenized
// through repeated calls to the Scan function. For compatibility with
// existing tools, the NUL character is not allowed. If the first character
// in the source is a UTF-8 encoded byte order mark (BOM), it is discarded.
//
// By default, a [Scanner] skips white space and Go comments and recognizes all
// literals as defined by the Go language specification. It may be
// customized to recognize only a subset of those literals and to recognize
// different identifier and white space characters.
package scanner

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"unicode"
	"unicode/utf8"
)

// Position is a value that represents a source position.
// A position is valid if Line > 0.
type Position struct {
	Filename string // filename, if any
	Offset   int    // byte offset, starting at 0
	Line     int    // line number, starting at 1
	Column   int    // column number, starting at 1 (character count per line)
}

// IsValid reports whether the position is valid.
func (pos *Position) IsValid() bool { return pos.Line > 0 }

func (pos Position) String() string {
	s := pos.Filename
	if s == "" {
		s = "<input>"
	}
	if pos.IsValid() {
		s += fmt.Sprintf(":%d:%d", pos.Line, pos.Column)
	}
	return s
}

// Predefined mode bits to control recognition of tokens. For instance,
// to configure a [Scanner] such that it only recognizes (Go) identifiers,
// integers, and skips comments, set the Scanner's Mode field to:
//
//	ScanIdents | ScanInts | ScanComments | SkipComments
//
// With the exceptions of comments, which are skipped if SkipComments is
// set, unrecognized tokens are not ignored. Instead, the scanner simply
// returns the respective individual characters (or possibly sub-tokens).
// For instance, if the mode is ScanIdents (not ScanStrings), the string
// "foo" is scanned as the token sequence '"' [Ident] '"'.
//
// Use GoTokens to configure the Scanner such that it accepts all Go
// literal tokens including Go identifiers. Comments will be skipped.
const (
	ScanIdents     = 1 << -Ident
	ScanInts       = 1 << -Int
	ScanFloats     = 1 << -Float // includes Ints and hexadecimal floats
	ScanChars      = 1 << -Char
	ScanStrings    = 1 << -String
	ScanRawStrings = 1 << -RawString
	ScanComments   = 1 << -Comment
	SkipComments   = 1 << -skipComment // if set with ScanComments, comments become white space
	GoTokens       = ScanIdents | ScanFloats | ScanChars | ScanStrings | ScanRawStrings | ScanComments | SkipComments
)

// The result of Scan is one of these tokens or a Unicode character.
const (
	EOF = -(iota + 1)
	Ident
	Int
	Float
	Char
	String
	RawString
	Comment

	// internal use only
	skipComment
)

var tokenString = map[rune]string{
	EOF:       "EOF",
	Ident:     "Ident",
	Int:       "Int",
	Float:     "Float",
	Char:      "Char",
	String:    "String",
	RawString: "RawString",
	Comment:   "Comment",
}

// TokenString returns a printable string for a token or Unicode character.
func TokenString(tok rune) string {
	if s, found := tokenString[tok]; found {
		return s
	}
	return fmt.Sprintf("%q", string(tok))
}

// GoWhitespace is the default value for the [Scanner]'s Whitespace field.
// Its value selects Go's white space characters.
const GoWhitespace = 1<<'\t' | 1<<'\n' | 1<<'\r' | 1<<' '

const bufLen = 1024 // at least utf8.UTFMax

// A Scanner implements reading of Unicode characters and tokens from an [io.Reader].
type Scanner struct {
	// Input
	src io.Reader

	// Source buffer
	srcBuf [bufLen + 1]byte // +1 for sentinel for common case of s.next()
	srcPos int              // reading position (srcBuf index)
	srcEnd int              // source end (srcBuf index)

	// Source position
	srcBufOffset int // byte offset of srcBuf[0] in source
	line         int // line count
	column       int // character count
	lastLineLen  int // length of last line in characters (for correct column reporting)
	lastCharLen  int // length of last character in bytes

	// Token text buffer
	// Typically, token text is stored completely in srcBuf, but in general
	// the token text's head may be buffered in tokBuf while the token text's
	// tail is stored in srcBuf.
	tokBuf bytes.Buffer // token text head that is not in srcBuf anymore
	tokPos int          // token text tail position (srcBuf index); valid if >= 0
	tokEnd int          // token text tail end (srcBuf index)

	// One character look-ahead
	ch rune // character before current srcPos

	// Error is called for each error encountered. If no Error
	// function is set, the error is reported to os.Stderr.
	Error func(s *Scanner, msg string)

	// ErrorCount is incremented by one for each error encountered.
	ErrorCount int

	// The Mode field controls which tokens are recognized. For instance,
	// to recognize Ints, set the ScanInts bit in Mode. The field may be
	// changed at any time.
	Mode uint

	// The Whitespace field controls which characters are recognized
	// as white space. To recognize a character ch <= ' ' as white space,
	// set the ch'th bit in Whitespace (the Scanner's behavior is undefined
	// for values ch > ' '). The field may be changed at any time.
	Whitespace uint64

	// IsIdentRune is a predicate controlling the characters accepted
	// as the ith rune in an identifier. The set of valid characters
	// must not intersect with the set of white space characters.
	// If no IsIdentRune function is set, regular Go identifiers are
	// accepted instead. The field may be changed at any time.
	IsIdentRune func(ch rune, i int) bool

	// Start position of most recently scanned token; set by Scan.
	// Calling Init or Next invalidates the position (Line == 0).
	// The Filename field is always left untouched by the Scanner.
	// If an error is reported (via Error) and Position is invalid,
	// the scanner is not inside a token. Call Pos to obtain an error
	// position in that case, or to obtain the position immediately
	// after the most recently scanned token.
	Position
}

// Init initializes a [Scanner] with a new source and returns s.
// [Scanner.Error] is set to nil, [Scanner.ErrorCount] is set to 0, [Scanner.Mode] is set to [GoTokens],
// and [Scanner.Whitespace] is set to [GoWhitespace].
func (s *Scanner) Init(src io.Reader) *Scanner {
	s.src = src

	// initialize source buffer
	// (the first call to next() will fill it by calling src.Read)
	s.srcBuf[0] = utf8.RuneSelf // sentinel
	s.srcPos = 0
	s.srcEnd = 0

	// initialize source position
	s.srcBufOffset = 0
	s.line = 1
	s.column = 0
	s.lastLineLen = 0
	s.lastCharLen = 0

	// initialize token text buffer
	// (required for first call to next()).
	s.tokPos = -1

	// initialize one character look-ahead
	s.ch = -2 // no char read yet, not EOF

	// initialize public fields
	s.Error = nil
	s.ErrorCount = 0
	s.Mode = GoTokens
	s.Whitespace = GoWhitespace
	s.Line = 0 // invalidate token position

	return s
}

// next reads and returns the next Unicode character. It is designed such
// that only a minimal amount of work needs to be done in the common ASCII
// case (one test to check for both ASCII and end-of-buffer, and one test
// to check for newlines).
func (s *Scanner) next() rune {
	ch, width := rune(s.srcBuf[s.srcPos]), 1

	if ch >= utf8.RuneSelf {
		// uncommon case: not ASCII or not enough bytes
		for s.srcPos+utf8.UTFMax > s.srcEnd && !utf8.FullRune(s.srcBuf[s.srcPos:s.srcEnd]) {
			// not enough bytes: read some more, but first
			// save away token text if any
			if s.tokPos >= 0 {
				s.tokBuf.Write(s.srcBuf[s.tokPos:s.srcPos])
				s.tokPos = 0
				// s.tokEnd is set by Scan()
			}
			// move unread bytes to beginning of buffer
			copy(s.srcBuf[0:], s.srcBuf[s.srcPos:s.srcEnd])
			s.srcBufOffset += s.srcPos
			// read more bytes
			// (an io.Reader must return io.EOF when it reaches
			// the end of what it is reading - simply returning
			// n == 0 will make this loop retry forever; but the
			// error is in the reader implementation in that case)
			i := s.srcEnd - s.srcPos
			n, err := s.src.Read(s.srcBuf[i:bufLen])
			s.srcPos = 0
			s.srcEnd = i + n
			s.srcBuf[s.srcEnd] = utf8.RuneSelf // sentinel
			if err != nil {
				if err != io.EOF {
					s.error(err.Error())
				}
				if s.srcEnd == 0 {
					if s.lastCharLen > 0 {
						// previous character was not EOF
						s.column++
					}
					s.lastCharLen = 0
					return EOF
				}
				// If err == EOF, we won't be getting more
				// bytes; break to avoid infinite loop. If
				// err is something else, we don't know if
				// we can get more bytes; thus also break.
				break
			}
		}
		// at least one byte
		ch = rune(s.srcBuf[s.srcPos])
		if ch >= utf8.RuneSelf {
			// uncommon case: not ASCII
			ch, width = utf8.DecodeRune(s.srcBuf[s.srcPos:s.srcEnd])
			if ch == utf8.RuneError && width == 1 {
				// advance for correct error position
				s.srcPos += width
				s.lastCharLen = width
				s.column++
				s.error("invalid UTF-8 encoding")
				return ch
			}
		}
	}

	// advance
	s.srcPos += width
	s.lastCharLen = width
	s.column++

	// special situations
	switch ch {
	case 0:
		// for compatibility with other tools
		s.error("invalid character NUL")
	case '\n':
		s.line++
		s.lastLineLen = s.column
		s.column = 0
	}

	return ch
}

// Next reads and returns the next Unicode character.
// It returns [EOF] at the end of the source. It reports
// a read error by calling s.Error, if not nil; otherwise
// it prints an error message to [os.Stderr]. Next does not
// update the [Scanner.Position] field; use [Scanner.Pos]() to
// get the current position.
func (s *Scanner) Next() rune {
	s.tokPos = -1 // don't collect token text
	s.Line = 0    // invalidate token position
	ch := s.Peek()
	if ch != EOF {
		s.ch = s.next()
	}
	return ch
}

// Peek returns the next Unicode character in the source without advancing
// the scanner. It returns [EOF] if the scanner's position is at the last
// character of the source.
func (s *Scanner) Peek() rune {
	if s.ch == -2 {
		// this code is only run for the very first character
		s.ch = s.next()
		if s.ch == '\uFEFF' {
			s.ch = s.next() // ignore BOM
		}
	}
	return s.ch
}

func (s *Scanner) error(msg string) {
	s.tokEnd = s.srcPos - s.lastCharLen // make sure token text is terminated
	s.ErrorCount++
	if s.Error != nil {
		s.Error(s, msg)
		return
	}
	pos := s.Position
	if !pos.IsValid() {
		pos = s.Pos()
	}
	fmt.Fprintf(os.Stderr, "%s: %s\n", pos, msg)
}

func (s *Scanner) errorf(format string, args ...any) {
	s.error(fmt.Sprintf(format, args...))
}

func (s *Scanner) isIdentRune(ch rune, i int) bool {
	if s.IsIdentRune != nil {
		return ch != EOF && s.IsIdentRune(ch, i)
	}
	return ch == '_' || unicode.IsLetter(ch) || unicode.IsDigit(ch) && i > 0
}

func (s *Scanner) scanIdentifier() rune {
	// we know the zero'th rune is OK; start scanning at the next one
	ch := s.next()
	for i := 1; s.isIdentRune(ch, i); i++ {
		ch = s.next()
	}
	return ch
}

func lower(ch rune) rune     { return ('a' - 'A') | ch } // returns lower-case ch iff ch is ASCII letter
func isDecimal(ch rune) bool { return '0' <= ch && ch <= '9' }
func isHex(ch rune) bool     { return '0' <= ch && ch <= '9' || 'a' <= lower(ch) && lower(ch) <= 'f' }

// digits accepts the sequence { digit | '_' } starting with ch0.
// If base <= 10, digits accepts any decimal digit but records
// the first invalid digit >= base in *invalid if *invalid == 0.
// digits returns the first rune that is not part of the sequence
// anymore, and a bitset describing whether the sequence contained
// digits (bit 0 is set), or separators '_' (bit 1 is set).
func (s *Scanner) digits(ch0 rune, base int, invalid *rune) (ch rune, digsep int) {
	ch = ch0
	if base <= 10 {
		max := rune('0' + base)
		for isDecimal(ch) || ch == '_' {
			ds := 1
			if ch == '_' {
				ds = 2
			} else if ch >= max && *invalid == 0 {
				*invalid = ch
			}
			digsep |= ds
			ch = s.next()
		}
	} else {
		for isHex(ch) || ch == '_' {
			ds := 1
			if ch == '_' {
				ds = 2
			}
			digsep |= ds
			ch = s.next()
		}
	}
	return
}

func (s *Scanner) scanNumber(ch rune, seenDot bool) (rune, rune) {
	base := 10         // number base
	prefix := rune(0)  // one of 0 (decimal), '0' (0-octal), 'x', 'o', or 'b'
	digsep := 0        // bit 0: digit present, bit 1: '_' present
	invalid := rune(0) // invalid digit in literal, or 0

	// integer part
	var tok rune
	var ds int
	if !seenDot {
		tok = Int
		if ch == '0' {
			ch = s.next()
			switch lower(ch) {
			case 'x':
				ch = s.next()
				base, prefix = 16, 'x'
			case 'o':
				ch = s.next()
				base, prefix = 8, 'o'
			case 'b':
				ch = s.next()
				base, prefix = 2, 'b'
			default:
				base, prefix = 8, '0'
				digsep = 1 // leading 0
			}
		}
		ch, ds = s.digits(ch, base, &invalid)
		digsep |= ds
		if ch == '.' && s.Mode&ScanFloats != 0 {
			ch = s.next()
			seenDot = true
		}
	}

	// fractional part
	if seenDot {
		tok = Float
		if prefix == 'o' || prefix == 'b' {
			s.error("invalid radix point in " + litname(prefix))
		}
		ch, ds = s.digits(ch, base, &invalid)
		digsep |= ds
	}

	if digsep&1 == 0 {
		s.error(litname(prefix) + " has no digits")
	}

	// exponent
	if e := lower(ch); (e == 'e' || e == 'p') && s.Mode&ScanFloats != 0 {
		switch {
		case e == 'e' && prefix != 0 && prefix != '0':
			s.errorf("%q exponent requires decimal mantissa", ch)
		case e == 'p' && prefix != 'x':
			s.errorf("%q exponent requires hexadecimal mantissa", ch)
		}
		ch = s.next()
		tok = Float
		if ch == '+' || ch == '-' {
			ch = s.next()
		}
		ch, ds = s.digits(ch, 10, nil)
		digsep |= ds
		if ds&1 == 0 {
			s.error("exponent has no digits")
		}
	} else if prefix == 'x' && tok == Float {
		s.error("hexadecimal mantissa requires a 'p' exponent")
	}

	if tok == Int && invalid != 0 {
		s.errorf("invalid digit %q in %s", invalid, litname(prefix))
	}

	if digsep&2 != 0 {
		s.tokEnd = s.srcPos - s.lastCharLen // make sure token text is terminated
		if i := invalidSep(s.TokenText()); i >= 0 {
			s.error("'_' must separate successive digits")
		}
	}

	return tok, ch
}

func litname(prefix rune) string {
	switch prefix {
	default:
		return "decimal literal"
	case 'x':
		return "hexadecimal literal"
	case 'o', '0':
		return "octal literal"
	case 'b':
		return "binary literal"
	}
}

// invalidSep returns the index of the first invalid separator in x, or -1.
func invalidSep(x string) int {
	x1 := ' ' // prefix char, we only care if it's 'x'
	d := '.'  // digit, one of '_', '0' (a digit), or '.' (anything else)
	i := 0

	// a prefix counts as a digit
	if len(x) >= 2 && x[0] == '0' {
		x1 = lower(rune(x[1]))
		if x1 == 'x' || x1 == 'o' || x1 == 'b' {
			d = '0'
			i = 2
		}
	}

	// mantissa and exponent
	for ; i < len(x); i++ {
		p := d // previous digit
		d = rune(x[i])
		switch {
		case d == '_':
			if p != '0' {
				return i
			}
		case isDecimal(d) || x1 == 'x' && isHex(d):
			d = '0'
		default:
			if p == '_' {
				return i - 1
			}
			d = '.'
		}
	}
	if d == '_' {
		return len(x) - 1
	}

	return -1
}

func digitVal(ch rune) int {
	switch {
	case '0' <= ch && ch <= '9':
		return int(ch - '0')
	case 'a' <= lower(ch) && lower(ch) <= 'f':
		return int(lower(ch) - 'a' + 10)
	}
	return 16 // larger than any legal digit val
}

func (s *Scanner) scanDigits(ch rune, base, n int) rune {
	for n > 0 && digitVal(ch) < base {
		ch = s.next()
		n--
	}
	if n > 0 {
		s.error("invalid char escape")
	}
	return ch
}

func (s *Scanner) scanEscape(quote rune) rune {
	ch := s.next() // read character after '/'
	switch ch {
	case 'a', 'b', 'f', 'n', 'r', 't', 'v', '\\', quote:
		// nothing to do
		ch = s.next()
	case '0', '1', '2', '3', '4', '5', '6', '7':
		ch = s.scanDigits(ch, 8, 3)
	case 'x':
		ch = s.scanDigits(s.next(), 16, 2)
	case 'u':
		ch = s.scanDigits(s.next(), 16, 4)
	case 'U':
		ch = s.scanDigits(s.next(), 16, 8)
	default:
		s.error("invalid char escape")
	}
	return ch
}

func (s *Scanner) scanString(quote rune) (n int) {
	ch := s.next() // read character after quote
	for ch != quote {
		if ch == '\n' || ch < 0 {
			s.error("literal not terminated")
			return
		}
		if ch == '\\' {
			ch = s.scanEscape(quote)
		} else {
			ch = s.next()
		}
		n++
	}
	return
}

func (s *Scanner) scanRawString() {
	ch := s.next() // read character after '`'
	for ch != '`' {
		if ch < 0 {
			s.error("literal not terminated")
			return
		}
		ch = s.next()
	}
}

func (s *Scanner) scanChar() {
	if s.scanString('\'') != 1 {
		s.error("invalid char literal")
	}
}

func (s *Scanner) scanComment(ch rune) rune {
	// ch == '/' || ch == '*'
	if ch == '/' {
		// line comment
		ch = s.next() // read character after "//"
		for ch != '\n' && ch >= 0 {
			ch = s.next()
		}
		return ch
	}

	// general comment
	ch = s.next() // read character after "/*"
	for {
		if ch < 0 {
			s.error("comment not terminated")
			break
		}
		ch0 := ch
		ch = s.next()
		if ch0 == '*' && ch == '/' {
			ch = s.next()
			break
		}
	}
	return ch
}

// Scan reads the next token or Unicode character from source and returns it.
// It only recognizes tokens t for which the respective [Scanner.Mode] bit (1<<-t) is set.
// It returns [EOF] at the end of the source. It reports scanner errors (read and
// token errors) by calling s.Error, if not nil; otherwise it prints an error
// message to [os.Stderr].
func (s *Scanner) Scan() rune {
	ch := s.Peek()

	// reset token text position
	s.tokPos = -1
	s.Line = 0

redo:
	// skip white space
	for s.Whitespace&(1<<uint(ch)) != 0 {
		ch = s.next()
	}

	// start collecting token text
	s.tokBuf.Reset()
	s.tokPos = s.srcPos - s.lastCharLen

	// set token position
	// (this is a slightly optimized version of the code in Pos())
	s.Offset = s.srcBufOffset + s.tokPos
	if s.column > 0 {
		// common case: last character was not a '\n'
		s.Line = s.line
		s.Column = s.column
	} else {
		// last character was a '\n'
		// (we cannot be at the beginning of the source
		// since we have called next() at least once)
		s.Line = s.line - 1
		s.Column = s.lastLineLen
	}

	// determine token value
	tok := ch
	switch {
	case s.isIdentRune(ch, 0):
		if s.Mode&ScanIdents != 0 {
			tok = Ident
			ch = s.scanIdentifier()
		} else {
			ch = s.next()
		}
	case isDecimal(ch):
		if s.Mode&(ScanInts|ScanFloats) != 0 {
			tok, ch = s.scanNumber(ch, false)
		} else {
			ch = s.next()
		}
	default:
		switch ch {
		case EOF:
			break
		case '"':
			if s.Mode&ScanStrings != 0 {
				s.scanString('"')
				tok = String
			}
			ch = s.next()
		case '\'':
			if s.Mode&ScanChars != 0 {
				s.scanChar()
				tok = Char
			}
			ch = s.next()
		case '.':
			ch = s.next()
			if isDecimal(ch) && s.Mode&ScanFloats != 0 {
				tok, ch = s.scanNumber(ch, true)
			}
		case '/':
			ch = s.next()
			if (ch == '/' || ch == '*') && s.Mode&ScanComments != 0 {
				if s.Mode&SkipComments != 0 {
					s.tokPos = -1 // don't collect token text
					ch = s.scanComment(ch)
					goto redo
				}
				ch = s.scanComment(ch)
				tok = Comment
			}
		case '`':
			if s.Mode&ScanRawStrings != 0 {
				s.scanRawString()
				tok = RawString
			}
			ch = s.next()
		default:
			ch = s.next()
		}
	}

	// end of token text
	s.tokEnd = s.srcPos - s.lastCharLen

	s.ch = ch
	return tok
}

// Pos returns the position of the character immediately after
// the character or token returned by the last call to [Scanner.Next] or [Scanner.Scan].
// Use the [Scanner.Position] field for the start position of the most
// recently scanned token.
func (s *Scanner) Pos() (pos Position) {
	pos.Filename = s.Filename
	pos.Offset = s.srcBufOffset + s.srcPos - s.lastCharLen
	switch {
	case s.column > 0:
		// common case: last character was not a '\n'
		pos.Line = s.line
		pos.Column = s.column
	case s.lastLineLen > 0:
		// last character was a '\n'
		pos.Line = s.line - 1
		pos.Column = s.lastLineLen
	default:
		// at the beginning of the source
		pos.Line = 1
		pos.Column = 1
	}
	return
}

// TokenText returns the string corresponding to the most recently scanned token.
// Valid after calling [Scanner.Scan] and in calls of [Scanner.Error].
func (s *Scanner) TokenText() string {
	if s.tokPos < 0 {
		// no token text
		return ""
	}

	if s.tokEnd < s.tokPos {
		// if EOF was reached, s.tokEnd is set to -1 (s.srcPos == 0)
		s.tokEnd = s.tokPos
	}
	// s.tokEnd >= s.tokPos

	if s.tokBuf.Len() == 0 {
		// common case: the entire token text is still in srcBuf
		return string(s.srcBuf[s.tokPos:s.tokEnd])
	}

	// part of the token text was saved in tokBuf: save the rest in
	// tokBuf as well and return its content
	s.tokBuf.Write(s.srcBuf[s.tokPos:s.tokEnd])
	s.tokPos = s.tokEnd // ensure idempotency of TokenText() call
	return s.tokBuf.String()
}

```

// === FILE: references/go/src/text/tabwriter/tabwriter.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tabwriter implements a write filter (tabwriter.Writer) that
// translates tabbed columns in input into properly aligned text.
//
// The package is using the Elastic Tabstops algorithm described at
// http://nickgravgaard.com/elastictabstops/index.html.
//
// The text/tabwriter package is frozen and is not accepting new features.
package tabwriter

import (
	"fmt"
	"io"
	"unicode/utf8"
)

// ----------------------------------------------------------------------------
// Filter implementation

// A cell represents a segment of text terminated by tabs or line breaks.
// The text itself is stored in a separate buffer; cell only describes the
// segment's size in bytes, its width in runes, and whether it's an htab
// ('\t') terminated cell.
type cell struct {
	size  int  // cell size in bytes
	width int  // cell width in runes
	htab  bool // true if the cell is terminated by an htab ('\t')
}

// A Writer is a filter that inserts padding around tab-delimited
// columns in its input to align them in the output.
//
// The Writer treats incoming bytes as UTF-8-encoded text consisting
// of cells terminated by horizontal ('\t') or vertical ('\v') tabs,
// and newline ('\n') or formfeed ('\f') characters; both newline and
// formfeed act as line breaks.
//
// Tab-terminated cells in contiguous lines constitute a column. The
// Writer inserts padding as needed to make all cells in a column have
// the same width, effectively aligning the columns. It assumes that
// all characters have the same width, except for tabs for which a
// tabwidth must be specified. Column cells must be tab-terminated, not
// tab-separated: non-tab terminated trailing text at the end of a line
// forms a cell but that cell is not part of an aligned column.
// For instance, in this example (where | stands for a horizontal tab):
//
//	aaaa|bbb|d
//	aa  |b  |dd
//	a   |
//	aa  |cccc|eee
//
// the b and c are in distinct columns (the b column is not contiguous
// all the way). The d and e are not in a column at all (there's no
// terminating tab, nor would the column be contiguous).
//
// The Writer assumes that all Unicode code points have the same width;
// this may not be true in some fonts or if the string contains combining
// characters.
//
// If [DiscardEmptyColumns] is set, empty columns that are terminated
// entirely by vertical (or "soft") tabs are discarded. Columns
// terminated by horizontal (or "hard") tabs are not affected by
// this flag.
//
// If a Writer is configured to filter HTML, HTML tags and entities
// are passed through. The widths of tags and entities are
// assumed to be zero (tags) and one (entities) for formatting purposes.
//
// A segment of text may be escaped by bracketing it with [Escape]
// characters. The tabwriter passes escaped text segments through
// unchanged. In particular, it does not interpret any tabs or line
// breaks within the segment. If the [StripEscape] flag is set, the
// Escape characters are stripped from the output; otherwise they
// are passed through as well. For the purpose of formatting, the
// width of the escaped text is always computed excluding the Escape
// characters.
//
// The formfeed character acts like a newline but it also terminates
// all columns in the current line (effectively calling [Writer.Flush]). Tab-
// terminated cells in the next line start new columns. Unless found
// inside an HTML tag or inside an escaped text segment, formfeed
// characters appear as newlines in the output.
//
// The Writer must buffer input internally, because proper spacing
// of one line may depend on the cells in future lines. Clients must
// call Flush when done calling [Writer.Write].
type Writer struct {
	// configuration
	output   io.Writer
	minwidth int
	tabwidth int
	padding  int
	padbytes [8]byte
	flags    uint

	// current state
	buf     []byte   // collected text excluding tabs or line breaks
	pos     int      // buffer position up to which cell.width of incomplete cell has been computed
	cell    cell     // current incomplete cell; cell.width is up to buf[pos] excluding ignored sections
	endChar byte     // terminating char of escaped sequence (Escape for escapes, '>', ';' for HTML tags/entities, or 0)
	lines   [][]cell // list of lines; each line is a list of cells
	widths  []int    // list of column widths in runes - re-used during formatting
}

// addLine adds a new line.
// flushed is a hint indicating whether the underlying writer was just flushed.
// If so, the previous line is not likely to be a good indicator of the new line's cells.
func (b *Writer) addLine(flushed bool) {
	// Grow slice instead of appending,
	// as that gives us an opportunity
	// to re-use an existing []cell.
	if n := len(b.lines) + 1; n <= cap(b.lines) {
		b.lines = b.lines[:n]
		b.lines[n-1] = b.lines[n-1][:0]
	} else {
		b.lines = append(b.lines, nil)
	}

	if !flushed {
		// The previous line is probably a good indicator
		// of how many cells the current line will have.
		// If the current line's capacity is smaller than that,
		// abandon it and make a new one.
		if n := len(b.lines); n >= 2 {
			if prev := len(b.lines[n-2]); prev > cap(b.lines[n-1]) {
				b.lines[n-1] = make([]cell, 0, prev)
			}
		}
	}
}

// Reset the current state.
func (b *Writer) reset() {
	b.buf = b.buf[:0]
	b.pos = 0
	b.cell = cell{}
	b.endChar = 0
	b.lines = b.lines[0:0]
	b.widths = b.widths[0:0]
	b.addLine(true)
}

// Internal representation (current state):
//
// - all text written is appended to buf; tabs and line breaks are stripped away
// - at any given time there is a (possibly empty) incomplete cell at the end
//   (the cell starts after a tab or line break)
// - cell.size is the number of bytes belonging to the cell so far
// - cell.width is text width in runes of that cell from the start of the cell to
//   position pos; html tags and entities are excluded from this width if html
//   filtering is enabled
// - the sizes and widths of processed text are kept in the lines list
//   which contains a list of cells for each line
// - the widths list is a temporary list with current widths used during
//   formatting; it is kept in Writer because it's re-used
//
//                    |<---------- size ---------->|
//                    |                            |
//                    |<- width ->|<- ignored ->|  |
//                    |           |             |  |
// [---processed---tab------------<tag>...</tag>...]
// ^                  ^                         ^
// |                  |                         |
// buf                start of incomplete cell  pos

// Formatting can be controlled with these flags.
const (
	// Ignore html tags and treat entities (starting with '&'
	// and ending in ';') as single characters (width = 1).
	FilterHTML uint = 1 << iota

	// Strip Escape characters bracketing escaped text segments
	// instead of passing them through unchanged with the text.
	StripEscape

	// Force right-alignment of cell content.
	// Default is left-alignment.
	AlignRight

	// Handle empty columns as if they were not present in
	// the input in the first place.
	DiscardEmptyColumns

	// Always use tabs for indentation columns (i.e., padding of
	// leading empty cells on the left) independent of padchar.
	TabIndent

	// Print a vertical bar ('|') between columns (after formatting).
	// Discarded columns appear as zero-width columns ("||").
	Debug
)

// A [Writer] must be initialized with a call to Init. The first parameter (output)
// specifies the filter output. The remaining parameters control the formatting:
//
//	minwidth	minimal cell width including any padding
//	tabwidth	width of tab characters (equivalent number of spaces)
//	padding		padding added to a cell before computing its width
//	padchar		ASCII char used for padding
//			if padchar == '\t', the Writer will assume that the
//			width of a '\t' in the formatted output is tabwidth,
//			and cells are left-aligned independent of align_left
//			(for correct-looking results, tabwidth must correspond
//			to the tab width in the viewer displaying the result)
//	flags		formatting control
func (b *Writer) Init(output io.Writer, minwidth, tabwidth, padding int, padchar byte, flags uint) *Writer {
	if minwidth < 0 || tabwidth < 0 || padding < 0 {
		panic("negative minwidth, tabwidth, or padding")
	}
	b.output = output
	b.minwidth = minwidth
	b.tabwidth = tabwidth
	b.padding = padding
	for i := range b.padbytes {
		b.padbytes[i] = padchar
	}
	if padchar == '\t' {
		// tab padding enforces left-alignment
		flags &^= AlignRight
	}
	b.flags = flags

	b.reset()

	return b
}

// debugging support (keep code around)
func (b *Writer) dump() {
	pos := 0
	for i, line := range b.lines {
		print("(", i, ") ")
		for _, c := range line {
			print("[", string(b.buf[pos:pos+c.size]), "]")
			pos += c.size
		}
		print("\n")
	}
	print("\n")
}

// local error wrapper so we can distinguish errors we want to return
// as errors from genuine panics (which we don't want to return as errors)
type osError struct {
	err error
}

func (b *Writer) write0(buf []byte) {
	n, err := b.output.Write(buf)
	if n != len(buf) && err == nil {
		err = io.ErrShortWrite
	}
	if err != nil {
		panic(osError{err})
	}
}

func (b *Writer) writeN(src []byte, n int) {
	for n > len(src) {
		b.write0(src)
		n -= len(src)
	}
	b.write0(src[0:n])
}

var (
	newline = []byte{'\n'}
	tabs    = []byte("\t\t\t\t\t\t\t\t")
)

func (b *Writer) writePadding(textw, cellw int, useTabs bool) {
	if b.padbytes[0] == '\t' || useTabs {
		// padding is done with tabs
		if b.tabwidth == 0 {
			return // tabs have no width - can't do any padding
		}
		// make cellw the smallest multiple of b.tabwidth
		cellw = (cellw + b.tabwidth - 1) / b.tabwidth * b.tabwidth
		n := cellw - textw // amount of padding
		if n < 0 {
			panic("internal error")
		}
		b.writeN(tabs, (n+b.tabwidth-1)/b.tabwidth)
		return
	}

	// padding is done with non-tab characters
	b.writeN(b.padbytes[0:], cellw-textw)
}

var vbar = []byte{'|'}

func (b *Writer) writeLines(pos0 int, line0, line1 int) (pos int) {
	pos = pos0
	for i := line0; i < line1; i++ {
		line := b.lines[i]

		// if TabIndent is set, use tabs to pad leading empty cells
		useTabs := b.flags&TabIndent != 0

		for j, c := range line {
			if j > 0 && b.flags&Debug != 0 {
				// indicate column break
				b.write0(vbar)
			}

			if c.size == 0 {
				// empty cell
				if j < len(b.widths) {
					b.writePadding(c.width, b.widths[j], useTabs)
				}
			} else {
				// non-empty cell
				useTabs = false
				if b.flags&AlignRight == 0 { // align left
					b.write0(b.buf[pos : pos+c.size])
					pos += c.size
					if j < len(b.widths) {
						b.writePadding(c.width, b.widths[j], false)
					}
				} else { // align right
					if j < len(b.widths) {
						b.writePadding(c.width, b.widths[j], false)
					}
					b.write0(b.buf[pos : pos+c.size])
					pos += c.size
				}
			}
		}

		if i+1 == len(b.lines) {
			// last buffered line - we don't have a newline, so just write
			// any outstanding buffered data
			b.write0(b.buf[pos : pos+b.cell.size])
			pos += b.cell.size
		} else {
			// not the last line - write newline
			b.write0(newline)
		}
	}
	return
}

// Format the text between line0 and line1 (excluding line1); pos
// is the buffer position corresponding to the beginning of line0.
// Returns the buffer position corresponding to the beginning of
// line1 and an error, if any.
func (b *Writer) format(pos0 int, line0, line1 int) (pos int) {
	pos = pos0
	column := len(b.widths)
	for this := line0; this < line1; this++ {
		line := b.lines[this]

		if column >= len(line)-1 {
			continue
		}
		// cell exists in this column => this line
		// has more cells than the previous line
		// (the last cell per line is ignored because cells are
		// tab-terminated; the last cell per line describes the
		// text before the newline/formfeed and does not belong
		// to a column)

		// print unprinted lines until beginning of block
		pos = b.writeLines(pos, line0, this)
		line0 = this

		// column block begin
		width := b.minwidth // minimal column width
		discardable := true // true if all cells in this column are empty and "soft"
		for ; this < line1; this++ {
			line = b.lines[this]
			if column >= len(line)-1 {
				break
			}
			// cell exists in this column
			c := line[column]
			// update width
			if w := c.width + b.padding; w > width {
				width = w
			}
			// update discardable
			if c.width > 0 || c.htab {
				discardable = false
			}
		}
		// column block end

		// discard empty columns if necessary
		if discardable && b.flags&DiscardEmptyColumns != 0 {
			width = 0
		}

		// format and print all columns to the right of this column
		// (we know the widths of this column and all columns to the left)
		b.widths = append(b.widths, width) // push width
		pos = b.format(pos, line0, this)
		b.widths = b.widths[0 : len(b.widths)-1] // pop width
		line0 = this
	}

	// print unprinted lines until end
	return b.writeLines(pos, line0, line1)
}

// Append text to current cell.
func (b *Writer) append(text []byte) {
	b.buf = append(b.buf, text...)
	b.cell.size += len(text)
}

// Update the cell width.
func (b *Writer) updateWidth() {
	b.cell.width += utf8.RuneCount(b.buf[b.pos:])
	b.pos = len(b.buf)
}

// To escape a text segment, bracket it with Escape characters.
// For instance, the tab in this string "Ignore this tab: \xff\t\xff"
// does not terminate a cell and constitutes a single character of
// width one for formatting purposes.
//
// The value 0xff was chosen because it cannot appear in a valid UTF-8 sequence.
const Escape = '\xff'

// Start escaped mode.
func (b *Writer) startEscape(ch byte) {
	switch ch {
	case Escape:
		b.endChar = Escape
	case '<':
		b.endChar = '>'
	case '&':
		b.endChar = ';'
	}
}

// Terminate escaped mode. If the escaped text was an HTML tag, its width
// is assumed to be zero for formatting purposes; if it was an HTML entity,
// its width is assumed to be one. In all other cases, the width is the
// unicode width of the text.
func (b *Writer) endEscape() {
	switch b.endChar {
	case Escape:
		b.updateWidth()
		if b.flags&StripEscape == 0 {
			b.cell.width -= 2 // don't count the Escape chars
		}
	case '>': // tag of zero width
	case ';':
		b.cell.width++ // entity, count as one rune
	}
	b.pos = len(b.buf)
	b.endChar = 0
}

// Terminate the current cell by adding it to the list of cells of the
// current line. Returns the number of cells in that line.
func (b *Writer) terminateCell(htab bool) int {
	b.cell.htab = htab
	line := &b.lines[len(b.lines)-1]
	*line = append(*line, b.cell)
	b.cell = cell{}
	return len(*line)
}

func (b *Writer) handlePanic(err *error, op string) {
	if e := recover(); e != nil {
		if op == "Flush" {
			// If Flush ran into a panic, we still need to reset.
			b.reset()
		}
		if nerr, ok := e.(osError); ok {
			*err = nerr.err
			return
		}
		panic(fmt.Sprintf("tabwriter: panic during %s (%v)", op, e))
	}
}

// Flush should be called after the last call to [Writer.Write] to ensure
// that any data buffered in the [Writer] is written to output. Any
// incomplete escape sequence at the end is considered
// complete for formatting purposes.
func (b *Writer) Flush() error {
	return b.flush()
}

// flush is the internal version of Flush, with a named return value which we
// don't want to expose.
func (b *Writer) flush() (err error) {
	defer b.handlePanic(&err, "Flush")
	b.flushNoDefers()
	return nil
}

// flushNoDefers is like flush, but without a deferred handlePanic call. This
// can be called from other methods which already have their own deferred
// handlePanic calls, such as Write, and avoid the extra defer work.
func (b *Writer) flushNoDefers() {
	// add current cell if not empty
	if b.cell.size > 0 {
		if b.endChar != 0 {
			// inside escape - terminate it even if incomplete
			b.endEscape()
		}
		b.terminateCell(false)
	}

	// format contents of buffer
	b.format(0, 0, len(b.lines))
	b.reset()
}

var hbar = []byte("---\n")

// Write writes buf to the writer b.
// The only errors returned are ones encountered
// while writing to the underlying output stream.
func (b *Writer) Write(buf []byte) (n int, err error) {
	defer b.handlePanic(&err, "Write")

	// split text into cells
	n = 0
	for i, ch := range buf {
		if b.endChar == 0 {
			// outside escape
			switch ch {
			case '\t', '\v', '\n', '\f':
				// end of cell
				b.append(buf[n:i])
				b.updateWidth()
				n = i + 1 // ch consumed
				ncells := b.terminateCell(ch == '\t')
				if ch == '\n' || ch == '\f' {
					// terminate line
					b.addLine(ch == '\f')
					if ch == '\f' || ncells == 1 {
						// A '\f' always forces a flush. Otherwise, if the previous
						// line has only one cell which does not have an impact on
						// the formatting of the following lines (the last cell per
						// line is ignored by format()), thus we can flush the
						// Writer contents.
						b.flushNoDefers()
						if ch == '\f' && b.flags&Debug != 0 {
							// indicate section break
							b.write0(hbar)
						}
					}
				}

			case Escape:
				// start of escaped sequence
				b.append(buf[n:i])
				b.updateWidth()
				n = i
				if b.flags&StripEscape != 0 {
					n++ // strip Escape
				}
				b.startEscape(Escape)

			case '<', '&':
				// possibly an html tag/entity
				if b.flags&FilterHTML != 0 {
					// begin of tag/entity
					b.append(buf[n:i])
					b.updateWidth()
					n = i
					b.startEscape(ch)
				}
			}

		} else {
			// inside escape
			if ch == b.endChar {
				// end of tag/entity
				j := i + 1
				if ch == Escape && b.flags&StripEscape != 0 {
					j = i // strip Escape
				}
				b.append(buf[n:j])
				n = i + 1 // ch consumed
				b.endEscape()
			}
		}
	}

	// append leftover text
	b.append(buf[n:])
	n = len(buf)
	return
}

// NewWriter allocates and initializes a new [Writer].
// The parameters are the same as for the Init function.
func NewWriter(output io.Writer, minwidth, tabwidth, padding int, padchar byte, flags uint) *Writer {
	return new(Writer).Init(output, minwidth, tabwidth, padding, padchar, flags)
}

```

// === FILE: references/go/src/text/template/doc.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package template implements data-driven templates for generating textual output.

To generate HTML output, see [html/template], which has the same interface
as this package but automatically secures HTML output against certain attacks.

Templates are executed by applying them to a data structure. Annotations in the
template refer to elements of the data structure (typically a field of a struct
or a key in a map) to control execution and derive values to be displayed.
Execution of the template walks the structure and sets the cursor, represented
by a period '.' and called "dot", to the value at the current location in the
structure as execution proceeds.

The security model used by this package assumes that template authors are
trusted. The package does not auto-escape output, so injecting code into
a template can lead to arbitrary code execution if the template is executed
by an untrusted source.

The input text for a template is UTF-8-encoded text in any format.
"Actions"--data evaluations or control structures--are delimited by
"{{" and "}}"; all text outside actions is copied to the output unchanged.

Once parsed, a template may be executed safely in parallel, although if parallel
executions share a Writer the output may be interleaved.

Here is a trivial example that prints "17 items are made of wool".

	type Inventory struct {
		Material string
		Count    uint
	}
	sweaters := Inventory{"wool", 17}
	tmpl, err := template.New("test").Parse("{{.Count}} items are made of {{.Material}}")
	if err != nil { panic(err) }
	err = tmpl.Execute(os.Stdout, sweaters)
	if err != nil { panic(err) }

More intricate examples appear below.

Text and spaces

By default, all text between actions is copied verbatim when the template is
executed. For example, the string " items are made of " in the example above
appears on standard output when the program is run.

However, to aid in formatting template source code, if an action's left
delimiter (by default "{{") is followed immediately by a minus sign and white
space, all trailing white space is trimmed from the immediately preceding text.
Similarly, if the right delimiter ("}}") is preceded by white space and a minus
sign, all leading white space is trimmed from the immediately following text.
In these trim markers, the white space must be present:
"{{- 3}}" is like "{{3}}" but trims the immediately preceding text, while
"{{-3}}" parses as an action containing the number -3.

For instance, when executing the template whose source is

	"{{23 -}} < {{- 45}}"

the generated output would be

	"23<45"

For this trimming, the definition of white space characters is the same as in Go:
space, horizontal tab, carriage return, and newline.

Actions

Here is the list of actions. "Arguments" and "pipelines" are evaluations of
data, defined in detail in the corresponding sections that follow.

*/
//	{{/* a comment */}}
//	{{- /* a comment with white space trimmed from preceding and following text */ -}}
//		A comment; discarded. May contain newlines.
//		Comments do not nest and must start and end at the
//		delimiters, as shown here.
/*

	{{pipeline}}
		The default textual representation (the same as would be
		printed by fmt.Print) of the value of the pipeline is copied
		to the output.

	{{if pipeline}} T1 {{end}}
		If the value of the pipeline is empty, no output is generated;
		otherwise, T1 is executed. The empty values are false, 0, any
		nil pointer or interface value, and any array, slice, map, or
		string of length zero.
		Dot is unaffected.

	{{if pipeline}} T1 {{else}} T0 {{end}}
		If the value of the pipeline is empty, T0 is executed;
		otherwise, T1 is executed. Dot is unaffected.

	{{if pipeline}} T1 {{else if pipeline}} T0 {{end}}
		To simplify the appearance of if-else chains, the else action
		of an if may include another if directly; the effect is exactly
		the same as writing
			{{if pipeline}} T1 {{else}}{{if pipeline}} T0 {{end}}{{end}}

	{{range pipeline}} T1 {{end}}
		The value of the pipeline must be an array, slice, map, iter.Seq,
		iter.Seq2, integer or channel.
		If the value of the pipeline has length zero, nothing is output;
		otherwise, dot is set to the successive elements of the array,
		slice, or map and T1 is executed. If the value is a map and the
		keys are of basic type with a defined order, the elements will be
		visited in sorted key order.

	{{range pipeline}} T1 {{else}} T0 {{end}}
		The value of the pipeline must be an array, slice, map, iter.Seq,
		iter.Seq2, integer or channel.
		If the value of the pipeline has length zero, dot is unaffected and
		T0 is executed; otherwise, dot is set to the successive elements
		of the array, slice, or map and T1 is executed.

	{{break}}
		The innermost {{range pipeline}} loop is ended early, stopping the
		current iteration and bypassing all remaining iterations.

	{{continue}}
		The current iteration of the innermost {{range pipeline}} loop is
		stopped, and the loop starts the next iteration.

	{{template "name"}}
		The template with the specified name is executed with nil data.

	{{template "name" pipeline}}
		The template with the specified name is executed with dot set
		to the value of the pipeline.

	{{block "name" pipeline}} T1 {{end}}
		A block is shorthand for defining a template
			{{define "name"}} T1 {{end}}
		and then executing it in place
			{{template "name" pipeline}}
		The typical use is to define a set of root templates that are
		then customized by redefining the block templates within.

	{{with pipeline}} T1 {{end}}
		If the value of the pipeline is empty, no output is generated;
		otherwise, dot is set to the value of the pipeline and T1 is
		executed.

	{{with pipeline}} T1 {{else}} T0 {{end}}
		If the value of the pipeline is empty, dot is unaffected and T0
		is executed; otherwise, dot is set to the value of the pipeline
		and T1 is executed.

	{{with pipeline}} T1 {{else with pipeline}} T0 {{end}}
		To simplify the appearance of with-else chains, the else action
		of a with may include another with directly; the effect is exactly
		the same as writing
			{{with pipeline}} T1 {{else}}{{with pipeline}} T0 {{end}}{{end}}


Arguments

An argument is a simple value, denoted by one of the following.

	- A boolean, string, character, integer, floating-point, imaginary
	  or complex constant in Go syntax. These behave like Go's untyped
	  constants. Note that, as in Go, whether a large integer constant
	  overflows when assigned or passed to a function can depend on whether
	  the host machine's ints are 32 or 64 bits.
	- The keyword nil, representing an untyped Go nil.
	- The character '.' (period):

		.

	  The result is the value of dot.
	- A variable name, which is a (possibly empty) alphanumeric string
	  preceded by a dollar sign, such as

		$piOver2

	  or

		$

	  The result is the value of the variable.
	  Variables are described below.
	- The name of a field of the data, which must be a struct, preceded
	  by a period, such as

		.Field

	  The result is the value of the field. Field invocations may be
	  chained:

	    .Field1.Field2

	  Fields can also be evaluated on variables, including chaining:

	    $x.Field1.Field2
	- The name of a key of the data, which must be a map, preceded
	  by a period, such as

		.Key

	  The result is the map element value indexed by the key.
	  Key invocations may be chained and combined with fields to any
	  depth:

	    .Field1.Key1.Field2.Key2

	  Although the key must be an alphanumeric identifier, unlike with
	  field names they do not need to start with an upper case letter.
	  Keys can also be evaluated on variables, including chaining:

	    $x.key1.key2
	- The name of a niladic method of the data, preceded by a period,
	  such as

		.Method

	  The result is the value of invoking the method with dot as the
	  receiver, dot.Method(). Such a method must have one return value (of
	  any type) or two return values, the second of which is an error.
	  If it has two and the returned error is non-nil, execution terminates
	  and an error is returned to the caller as the value of Execute.
	  Method invocations may be chained and combined with fields and keys
	  to any depth:

	    .Field1.Key1.Method1.Field2.Key2.Method2

	  Methods can also be evaluated on variables, including chaining:

	    $x.Method1.Field
	- The name of a niladic function, such as

		fun

	  The result is the value of invoking the function, fun(). The return
	  types and values behave as in methods. Functions and function
	  names are described below.
	- A parenthesized instance of one the above, for grouping. The result
	  may be accessed by a field or map key invocation.

		print (.F1 arg1) (.F2 arg2)
		(.StructValuedMethod "arg").Field

Arguments may evaluate to any type; if they are pointers the implementation
automatically indirects to the base type when required.
If an evaluation yields a function value, such as a function-valued
field of a struct, the function is not invoked automatically, but it
can be used as a truth value for an if action and the like. To invoke
it, use the call function, defined below.

Pipelines

A pipeline is a possibly chained sequence of "commands". A command is a simple
value (argument) or a function or method call, possibly with multiple arguments:

	Argument
		The result is the value of evaluating the argument.
	.Method [Argument...]
		The method can be alone or the last element of a chain but,
		unlike methods in the middle of a chain, it can take arguments.
		The result is the value of calling the method with the
		arguments:
			dot.Method(Argument1, etc.)
	functionName [Argument...]
		The result is the value of calling the function associated
		with the name:
			function(Argument1, etc.)
		Functions and function names are described below.

A pipeline may be "chained" by separating a sequence of commands with pipeline
characters '|'. In a chained pipeline, the result of each command is
passed as the last argument of the following command. The output of the final
command in the pipeline is the value of the pipeline.

The output of a command will be either one value or two values, the second of
which has type error. If that second value is present and evaluates to
non-nil, execution terminates and the error is returned to the caller of
Execute.

Variables

A pipeline inside an action may initialize a variable to capture the result.
The initialization has syntax

	$variable := pipeline

where $variable is the name of the variable. An action that declares a
variable produces no output.

Variables previously declared can also be assigned, using the syntax

	$variable = pipeline

If a "range" action initializes a variable, the variable is set to the
successive elements of the iteration. Also, a "range" may declare two
variables, separated by a comma:

	range $index, $element := pipeline

in which case $index and $element are set to the successive values of the
array/slice index or map key and element, respectively. Note that if there is
only one variable, it is assigned the element; this is opposite to the
convention in Go range clauses.

A variable's scope extends to the "end" action of the control structure ("if",
"with", or "range") in which it is declared, or to the end of the template if
there is no such control structure. A template invocation does not inherit
variables from the point of its invocation.

When execution begins, $ is set to the data argument passed to Execute, that is,
to the starting value of dot.

Examples

Here are some example one-line templates demonstrating pipelines and variables.
All produce the quoted word "output":

	{{"\"output\""}}
		A string constant.
	{{`"output"`}}
		A raw string constant.
	{{printf "%q" "output"}}
		A function call.
	{{"output" | printf "%q"}}
		A function call whose final argument comes from the previous
		command.
	{{printf "%q" (print "out" "put")}}
		A parenthesized argument.
	{{"put" | printf "%s%s" "out" | printf "%q"}}
		A more elaborate call.
	{{"output" | printf "%s" | printf "%q"}}
		A longer chain.
	{{with "output"}}{{printf "%q" .}}{{end}}
		A with action using dot.
	{{with $x := "output" | printf "%q"}}{{$x}}{{end}}
		A with action that creates and uses a variable.
	{{with $x := "output"}}{{printf "%q" $x}}{{end}}
		A with action that uses the variable in another action.
	{{with $x := "output"}}{{$x | printf "%q"}}{{end}}
		The same, but pipelined.

Functions

During execution functions are found in two function maps: first in the
template, then in the global function map. By default, no functions are defined
in the template but the Funcs method can be used to add them.

Predefined global functions are named as follows.

	and
		Returns the boolean AND of its arguments by returning the
		first empty argument or the last argument. That is,
		"and x y" behaves as "if x then y else x."
		Evaluation proceeds through the arguments left to right
		and returns when the result is determined.
	call
		Returns the result of calling the first argument, which
		must be a function, with the remaining arguments as parameters.
		Thus "call .X.Y 1 2" is, in Go notation, dot.X.Y(1, 2) where
		Y is a func-valued field, map entry, or the like.
		The first argument must be the result of an evaluation
		that yields a value of function type (as distinct from
		a predefined function such as print). The function must
		return either one or two result values, the second of which
		is of type error. If the arguments don't match the function
		or the returned error value is non-nil, execution stops.
	html
		Returns the escaped HTML equivalent of the textual
		representation of its arguments. This function is unavailable
		in html/template, with a few exceptions.
	index
		Returns the result of indexing its first argument by the
		following arguments. Thus "index x 1 2 3" is, in Go syntax,
		x[1][2][3]. Each indexed item must be a map, slice, or array.
	slice
		slice returns the result of slicing its first argument by the
		remaining arguments. Thus "slice x 1 2" is, in Go syntax, x[1:2],
		while "slice x" is x[:], "slice x 1" is x[1:], and "slice x 1 2 3"
		is x[1:2:3]. The first argument must be a string, slice, or array.
	js
		Returns the escaped JavaScript equivalent of the textual
		representation of its arguments.
	len
		Returns the integer length of its argument.
	not
		Returns the boolean negation of its single argument.
	or
		Returns the boolean OR of its arguments by returning the
		first non-empty argument or the last argument, that is,
		"or x y" behaves as "if x then x else y".
		Evaluation proceeds through the arguments left to right
		and returns when the result is determined.
	print
		An alias for fmt.Sprint
	printf
		An alias for fmt.Sprintf
	println
		An alias for fmt.Sprintln
	urlquery
		Returns the escaped value of the textual representation of
		its arguments in a form suitable for embedding in a URL query.
		This function is unavailable in html/template, with a few
		exceptions.

The boolean functions take any zero value to be false and a non-zero
value to be true.

There is also a set of binary comparison operators defined as
functions:

	eq
		Returns the boolean truth of arg1 == arg2
	ne
		Returns the boolean truth of arg1 != arg2
	lt
		Returns the boolean truth of arg1 < arg2
	le
		Returns the boolean truth of arg1 <= arg2
	gt
		Returns the boolean truth of arg1 > arg2
	ge
		Returns the boolean truth of arg1 >= arg2

For simpler multi-way equality tests, eq (only) accepts two or more
arguments and compares the second and subsequent to the first,
returning in effect

	arg1==arg2 || arg1==arg3 || arg1==arg4 ...

(Unlike with || in Go, however, eq is a function call and all the
arguments will be evaluated.)

The comparison functions work on any values whose type Go defines as
comparable. For basic types such as integers, the rules are relaxed:
size and exact type are ignored, so any integer value, signed or unsigned,
may be compared with any other integer value. (The arithmetic value is compared,
not the bit pattern, so all negative integers are less than all unsigned integers.)
However, as usual, one may not compare an int with a float32 and so on.

Associated templates

Each template is named by a string specified when it is created. Also, each
template is associated with zero or more other templates that it may invoke by
name; such associations are transitive and form a name space of templates.

A template may use a template invocation to instantiate another associated
template; see the explanation of the "template" action above. The name must be
that of a template associated with the template that contains the invocation.

Nested template definitions

When parsing a template, another template may be defined and associated with the
template being parsed. Template definitions must appear at the top level of the
template, much like global variables in a Go program.

The syntax of such definitions is to surround each template declaration with a
"define" and "end" action.

The define action names the template being created by providing a string
constant. Here is a simple example:

	{{define "T1"}}ONE{{end}}
	{{define "T2"}}TWO{{end}}
	{{define "T3"}}{{template "T1"}} {{template "T2"}}{{end}}
	{{template "T3"}}

This defines two templates, T1 and T2, and a third T3 that invokes the other two
when it is executed. Finally it invokes T3. If executed this template will
produce the text

	ONE TWO

By construction, a template may reside in only one association. If it's
necessary to have a template addressable from multiple associations, the
template definition must be parsed multiple times to create distinct *Template
values, or must be copied with [Template.Clone] or [Template.AddParseTree].

Parse may be called multiple times to assemble the various associated templates;
see [ParseFiles], [ParseGlob], [Template.ParseFiles] and [Template.ParseGlob]
for simple ways to parse related templates stored in files.

A template may be executed directly or through [Template.ExecuteTemplate], which executes
an associated template identified by name. To invoke our example above, we
might write,

	err := tmpl.Execute(os.Stdout, "no data needed")
	if err != nil {
		log.Fatalf("execution failed: %s", err)
	}

or to invoke a particular template explicitly by name,

	err := tmpl.ExecuteTemplate(os.Stdout, "T2", "no data needed")
	if err != nil {
		log.Fatalf("execution failed: %s", err)
	}

*/
package template

```

// === FILE: references/go/src/text/template/exec.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"errors"
	"fmt"
	"internal/fmtsort"
	"io"
	"reflect"
	"runtime"
	"strings"
	"text/template/parse"
)

// maxExecDepth specifies the maximum stack depth of templates within
// templates. This limit is only practically reached by accidentally
// recursive template invocations. This limit allows us to return
// an error instead of triggering a stack overflow.
var maxExecDepth = initMaxExecDepth()

func initMaxExecDepth() int {
	if runtime.GOARCH == "wasm" {
		return 1000
	}
	return 100000
}

// state represents the state of an execution. It's not part of the
// template so that multiple executions of the same template
// can execute in parallel.
type state struct {
	tmpl  *Template
	wr    io.Writer
	node  parse.Node // current node, for errors
	vars  []variable // push-down stack of variable values.
	depth int        // the height of the stack of executing templates.
}

// variable holds the dynamic value of a variable such as $, $x etc.
type variable struct {
	name  string
	value reflect.Value
}

// push pushes a new variable on the stack.
func (s *state) push(name string, value reflect.Value) {
	s.vars = append(s.vars, variable{name, value})
}

// mark returns the length of the variable stack.
func (s *state) mark() int {
	return len(s.vars)
}

// pop pops the variable stack up to the mark.
func (s *state) pop(mark int) {
	s.vars = s.vars[0:mark]
}

// setVar overwrites the last declared variable with the given name.
// Used by variable assignments.
func (s *state) setVar(name string, value reflect.Value) {
	for i := s.mark() - 1; i >= 0; i-- {
		if s.vars[i].name == name {
			s.vars[i].value = value
			return
		}
	}
	s.errorf("undefined variable: %s", name)
}

// setTopVar overwrites the top-nth variable on the stack. Used by range iterations.
func (s *state) setTopVar(n int, value reflect.Value) {
	s.vars[len(s.vars)-n].value = value
}

// varValue returns the value of the named variable.
func (s *state) varValue(name string) reflect.Value {
	for i := s.mark() - 1; i >= 0; i-- {
		if s.vars[i].name == name {
			return s.vars[i].value
		}
	}
	s.errorf("undefined variable: %s", name)
	return zero
}

var zero reflect.Value

type missingValType struct{}

var missingVal = reflect.ValueOf(missingValType{})

var missingValReflectType = reflect.TypeFor[missingValType]()

func isMissing(v reflect.Value) bool {
	return v.IsValid() && v.Type() == missingValReflectType
}

// at marks the state to be on node n, for error reporting.
func (s *state) at(node parse.Node) {
	s.node = node
}

// doublePercent returns the string with %'s replaced by %%, if necessary,
// so it can be used safely inside a Printf format string.
func doublePercent(str string) string {
	return strings.ReplaceAll(str, "%", "%%")
}

// TODO: It would be nice if ExecError was more broken down, but
// the way ErrorContext embeds the template name makes the
// processing too clumsy.

// ExecError is the custom error type returned when Execute has an
// error evaluating its template. (If a write error occurs, the actual
// error is returned; it will not be of type ExecError.)
type ExecError struct {
	Name string // Name of template.
	Err  error  // Pre-formatted error.
}

func (e ExecError) Error() string {
	return e.Err.Error()
}

func (e ExecError) Unwrap() error {
	return e.Err
}

// errorf records an ExecError and terminates processing.
func (s *state) errorf(format string, args ...any) {
	name := doublePercent(s.tmpl.Name())
	if s.node == nil {
		format = fmt.Sprintf("template: %s: %s", name, format)
	} else {
		location, context := s.tmpl.ErrorContext(s.node)
		format = fmt.Sprintf("template: %s: executing %q at <%s>: %s", location, name, doublePercent(context), format)
	}
	panic(ExecError{
		Name: s.tmpl.Name(),
		Err:  fmt.Errorf(format, args...),
	})
}

// writeError is the wrapper type used internally when Execute has an
// error writing to its output. We strip the wrapper in errRecover.
// Note that this is not an implementation of error, so it cannot escape
// from the package as an error value.
type writeError struct {
	Err error // Original error.
}

func (s *state) writeError(err error) {
	panic(writeError{
		Err: err,
	})
}

// errRecover is the handler that turns panics into returns from the top
// level of Parse.
func errRecover(errp *error) {
	e := recover()
	if e != nil {
		switch err := e.(type) {
		case runtime.Error:
			panic(e)
		case writeError:
			*errp = err.Err // Strip the wrapper.
		case ExecError:
			*errp = err // Keep the wrapper.
		default:
			panic(e)
		}
	}
}

// ExecuteTemplate applies the template associated with t that has the given name
// to the specified data object and writes the output to wr.
// If an error occurs executing the template or writing its output,
// execution stops, but partial results may already have been written to
// the output writer.
// A template may be executed safely in parallel, although if parallel
// executions share a Writer the output may be interleaved.
func (t *Template) ExecuteTemplate(wr io.Writer, name string, data any) error {
	tmpl := t.Lookup(name)
	if tmpl == nil {
		return fmt.Errorf("template: no template %q associated with template %q", name, t.name)
	}
	return tmpl.Execute(wr, data)
}

// Execute applies a parsed template to the specified data object,
// and writes the output to wr.
// If an error occurs executing the template or writing its output,
// execution stops, but partial results may already have been written to
// the output writer.
// A template may be executed safely in parallel, although if parallel
// executions share a Writer the output may be interleaved.
//
// If data is a [reflect.Value], the template applies to the concrete
// value that the reflect.Value holds, as in [fmt.Print].
func (t *Template) Execute(wr io.Writer, data any) error {
	return t.execute(wr, data)
}

func (t *Template) execute(wr io.Writer, data any) (err error) {
	defer errRecover(&err)
	value, ok := data.(reflect.Value)
	if !ok {
		value = reflect.ValueOf(data)
	}
	state := &state{
		tmpl: t,
		wr:   wr,
		vars: []variable{{"$", value}},
	}
	if t.Tree == nil || t.Root == nil {
		state.errorf("%q is an incomplete or empty template", t.Name())
	}
	state.walk(value, t.Root)
	return
}

// DefinedTemplates returns a string listing the defined templates,
// prefixed by the string "; defined templates are: ". If there are none,
// it returns the empty string. For generating an error message here
// and in [html/template].
func (t *Template) DefinedTemplates() string {
	if t.common == nil {
		return ""
	}
	var b strings.Builder
	t.muTmpl.RLock()
	defer t.muTmpl.RUnlock()
	for name, tmpl := range t.tmpl {
		if tmpl.Tree == nil || tmpl.Root == nil {
			continue
		}
		if b.Len() == 0 {
			b.WriteString("; defined templates are: ")
		} else {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%q", name)
	}
	return b.String()
}

// Sentinel errors for use with panic to signal early exits from range loops.
var (
	walkBreak    = errors.New("break")
	walkContinue = errors.New("continue")
)

// Walk functions step through the major pieces of the template structure,
// generating output as they go.
func (s *state) walk(dot reflect.Value, node parse.Node) {
	s.at(node)
	switch node := node.(type) {
	case *parse.ActionNode:
		// Do not pop variables so they persist until next end.
		// Also, if the action declares variables, don't print the result.
		val := s.evalPipeline(dot, node.Pipe)
		if len(node.Pipe.Decl) == 0 {
			s.printValue(node, val)
		}
	case *parse.BreakNode:
		panic(walkBreak)
	case *parse.CommentNode:
	case *parse.ContinueNode:
		panic(walkContinue)
	case *parse.IfNode:
		s.walkIfOrWith(parse.NodeIf, dot, node.Pipe, node.List, node.ElseList)
	case *parse.ListNode:
		for _, node := range node.Nodes {
			s.walk(dot, node)
		}
	case *parse.RangeNode:
		s.walkRange(dot, node)
	case *parse.TemplateNode:
		s.walkTemplate(dot, node)
	case *parse.TextNode:
		if _, err := s.wr.Write(node.Text); err != nil {
			s.writeError(err)
		}
	case *parse.WithNode:
		s.walkIfOrWith(parse.NodeWith, dot, node.Pipe, node.List, node.ElseList)
	default:
		s.errorf("unknown node: %s", node)
	}
}

// walkIfOrWith walks an 'if' or 'with' node. The two control structures
// are identical in behavior except that 'with' sets dot.
func (s *state) walkIfOrWith(typ parse.NodeType, dot reflect.Value, pipe *parse.PipeNode, list, elseList *parse.ListNode) {
	defer s.pop(s.mark())
	val := s.evalPipeline(dot, pipe)
	truth, ok := isTrue(indirectInterface(val))
	if !ok {
		s.errorf("if/with can't use %v", val)
	}
	if truth {
		if typ == parse.NodeWith {
			s.walk(val, list)
		} else {
			s.walk(dot, list)
		}
	} else if elseList != nil {
		s.walk(dot, elseList)
	}
}

// IsTrue reports whether the value is 'true', in the sense of not the zero of its type,
// and whether the value has a meaningful truth value. This is the definition of
// truth used by if and other such actions.
func IsTrue(val any) (truth, ok bool) {
	return isTrue(reflect.ValueOf(val))
}

func isTrue(val reflect.Value) (truth, ok bool) {
	if !val.IsValid() {
		// Something like var x interface{}, never set. It's a form of nil.
		return false, true
	}
	switch val.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		truth = val.Len() > 0
	case reflect.Bool:
		truth = val.Bool()
	case reflect.Complex64, reflect.Complex128:
		truth = val.Complex() != 0
	case reflect.Chan, reflect.Func, reflect.Pointer, reflect.UnsafePointer, reflect.Interface:
		truth = !val.IsNil()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		truth = val.Int() != 0
	case reflect.Float32, reflect.Float64:
		truth = val.Float() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		truth = val.Uint() != 0
	case reflect.Struct:
		truth = true // Struct values are always true.
	default:
		return
	}
	return truth, true
}

func (s *state) walkRange(dot reflect.Value, r *parse.RangeNode) {
	s.at(r)
	defer func() {
		if r := recover(); r != nil && r != walkBreak {
			panic(r)
		}
	}()
	defer s.pop(s.mark())
	val, _ := indirect(s.evalPipeline(dot, r.Pipe))
	// mark top of stack before any variables in the body are pushed.
	mark := s.mark()
	oneIteration := func(index, elem reflect.Value) {
		if len(r.Pipe.Decl) > 0 {
			if r.Pipe.IsAssign {
				// With two variables, index comes first.
				// With one, we use the element.
				if len(r.Pipe.Decl) > 1 {
					s.setVar(r.Pipe.Decl[0].Ident[0], index)
				} else {
					s.setVar(r.Pipe.Decl[0].Ident[0], elem)
				}
			} else {
				// Set top var (lexically the second if there
				// are two) to the element.
				s.setTopVar(1, elem)
			}
		}
		if len(r.Pipe.Decl) > 1 {
			if r.Pipe.IsAssign {
				s.setVar(r.Pipe.Decl[1].Ident[0], elem)
			} else {
				// Set next var (lexically the first if there
				// are two) to the index.
				s.setTopVar(2, index)
			}
		}
		defer s.pop(mark)
		defer func() {
			// Consume panic(walkContinue)
			if r := recover(); r != nil && r != walkContinue {
				panic(r)
			}
		}()
		s.walk(elem, r.List)
	}
	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if len(r.Pipe.Decl) > 1 {
			s.errorf("can't use %v to iterate over more than one variable", val)
			break
		}
		run := false
		for v := range val.Seq() {
			run = true
			// Pass element as second value, as we do for channels.
			oneIteration(reflect.Value{}, v)
		}
		if !run {
			break
		}
		return
	case reflect.Array, reflect.Slice:
		if val.Len() == 0 {
			break
		}
		for i := 0; i < val.Len(); i++ {
			oneIteration(reflect.ValueOf(i), val.Index(i))
		}
		return
	case reflect.Map:
		if val.Len() == 0 {
			break
		}
		om := fmtsort.Sort(val)
		for _, m := range om {
			oneIteration(m.Key, m.Value)
		}
		return
	case reflect.Chan:
		if val.IsNil() {
			break
		}
		if val.Type().ChanDir() == reflect.SendDir {
			s.errorf("range over send-only channel %v", val)
			break
		}
		i := 0
		for ; ; i++ {
			elem, ok := val.Recv()
			if !ok {
				break
			}
			oneIteration(reflect.ValueOf(i), elem)
		}
		if i == 0 {
			break
		}
		return
	case reflect.Invalid:
		break // An invalid value is likely a nil map, etc. and acts like an empty map.
	case reflect.Func:
		if val.Type().CanSeq() {
			if len(r.Pipe.Decl) > 1 {
				s.errorf("can't use %v iterate over more than one variable", val)
				break
			}
			run := false
			for v := range val.Seq() {
				run = true
				// Pass element as second value,
				// as we do for channels.
				oneIteration(reflect.Value{}, v)
			}
			if !run {
				break
			}
			return
		}
		if val.Type().CanSeq2() {
			run := false
			for i, v := range val.Seq2() {
				run = true
				if len(r.Pipe.Decl) > 1 {
					oneIteration(i, v)
				} else {
					// If there is only one range variable,
					// oneIteration will use the
					// second value.
					oneIteration(reflect.Value{}, i)
				}
			}
			if !run {
				break
			}
			return
		}
		fallthrough
	default:
		s.errorf("range can't iterate over %v", val)
	}
	if r.ElseList != nil {
		s.walk(dot, r.ElseList)
	}
}

func (s *state) walkTemplate(dot reflect.Value, t *parse.TemplateNode) {
	s.at(t)
	tmpl := s.tmpl.Lookup(t.Name)
	if tmpl == nil {
		s.errorf("template %q not defined", t.Name)
	}
	if s.depth == maxExecDepth {
		s.errorf("exceeded maximum template depth (%v)", maxExecDepth)
	}
	// Variables declared by the pipeline persist.
	dot = s.evalPipeline(dot, t.Pipe)
	newState := *s
	newState.depth++
	newState.tmpl = tmpl
	// No dynamic scoping: template invocations inherit no variables.
	newState.vars = []variable{{"$", dot}}
	newState.walk(dot, tmpl.Root)
}

// Eval functions evaluate pipelines, commands, and their elements and extract
// values from the data structure by examining fields, calling methods, and so on.
// The printing of those values happens only through walk functions.

// evalPipeline returns the value acquired by evaluating a pipeline. If the
// pipeline has a variable declaration, the variable will be pushed on the
// stack. Callers should therefore pop the stack after they are finished
// executing commands depending on the pipeline value.
func (s *state) evalPipeline(dot reflect.Value, pipe *parse.PipeNode) (value reflect.Value) {
	if pipe == nil {
		return
	}
	s.at(pipe)
	value = missingVal
	for _, cmd := range pipe.Cmds {
		value = s.evalCommand(dot, cmd, value) // previous value is this one's final arg.
		// If the object has type interface{}, dig down one level to the thing inside.
		if value.Kind() == reflect.Interface && value.Type().NumMethod() == 0 {
			value = value.Elem()
		}
	}
	for _, variable := range pipe.Decl {
		if pipe.IsAssign {
			s.setVar(variable.Ident[0], value)
		} else {
			s.push(variable.Ident[0], value)
		}
	}
	return value
}

func (s *state) notAFunction(args []parse.Node, final reflect.Value) {
	if len(args) > 1 || !isMissing(final) {
		s.errorf("can't give argument to non-function %s", args[0])
	}
}

func (s *state) evalCommand(dot reflect.Value, cmd *parse.CommandNode, final reflect.Value) reflect.Value {
	firstWord := cmd.Args[0]
	switch n := firstWord.(type) {
	case *parse.FieldNode:
		return s.evalFieldNode(dot, n, cmd.Args, final)
	case *parse.ChainNode:
		return s.evalChainNode(dot, n, cmd.Args, final)
	case *parse.IdentifierNode:
		// Must be a function.
		return s.evalFunction(dot, n, cmd, cmd.Args, final)
	case *parse.PipeNode:
		// Parenthesized pipeline. The arguments are all inside the pipeline; final must be absent.
		s.notAFunction(cmd.Args, final)
		return s.evalPipeline(dot, n)
	case *parse.VariableNode:
		return s.evalVariableNode(dot, n, cmd.Args, final)
	}
	s.at(firstWord)
	s.notAFunction(cmd.Args, final)
	switch word := firstWord.(type) {
	case *parse.BoolNode:
		return reflect.ValueOf(word.True)
	case *parse.DotNode:
		return dot
	case *parse.NilNode:
		s.errorf("nil is not a command")
	case *parse.NumberNode:
		return s.idealConstant(word)
	case *parse.StringNode:
		return reflect.ValueOf(word.Text)
	}
	s.errorf("can't evaluate command %q", firstWord)
	panic("not reached")
}

// idealConstant is called to return the value of a number in a context where
// we don't know the type. In that case, the syntax of the number tells us
// its type, and we use Go rules to resolve. Note there is no such thing as
// a uint ideal constant in this situation - the value must be of int type.
func (s *state) idealConstant(constant *parse.NumberNode) reflect.Value {
	// These are ideal constants but we don't know the type
	// and we have no context.  (If it was a method argument,
	// we'd know what we need.) The syntax guides us to some extent.
	s.at(constant)
	switch {
	case constant.IsComplex:
		return reflect.ValueOf(constant.Complex128) // incontrovertible.

	case constant.IsFloat &&
		!isHexInt(constant.Text) && !isRuneInt(constant.Text) &&
		strings.ContainsAny(constant.Text, ".eEpP"):
		return reflect.ValueOf(constant.Float64)

	case constant.IsInt:
		n := int(constant.Int64)
		if int64(n) != constant.Int64 {
			s.errorf("%s overflows int", constant.Text)
		}
		return reflect.ValueOf(n)

	case constant.IsUint:
		s.errorf("%s overflows int", constant.Text)
	}
	return zero
}

func isRuneInt(s string) bool {
	return len(s) > 0 && s[0] == '\''
}

func isHexInt(s string) bool {
	return len(s) > 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') && !strings.ContainsAny(s, "pP")
}

func (s *state) evalFieldNode(dot reflect.Value, field *parse.FieldNode, args []parse.Node, final reflect.Value) reflect.Value {
	s.at(field)
	return s.evalFieldChain(dot, dot, field, field.Ident, args, final)
}

func (s *state) evalChainNode(dot reflect.Value, chain *parse.ChainNode, args []parse.Node, final reflect.Value) reflect.Value {
	s.at(chain)
	if len(chain.Field) == 0 {
		s.errorf("internal error: no fields in evalChainNode")
	}
	if chain.Node.Type() == parse.NodeNil {
		s.errorf("indirection through explicit nil in %s", chain)
	}
	// (pipe).Field1.Field2 has pipe as .Node, fields as .Field. Eval the pipeline, then the fields.
	pipe := s.evalArg(dot, nil, chain.Node)
	return s.evalFieldChain(dot, pipe, chain, chain.Field, args, final)
}

func (s *state) evalVariableNode(dot reflect.Value, variable *parse.VariableNode, args []parse.Node, final reflect.Value) reflect.Value {
	// $x.Field has $x as the first ident, Field as the second. Eval the var, then the fields.
	s.at(variable)
	value := s.varValue(variable.Ident[0])
	if len(variable.Ident) == 1 {
		s.notAFunction(args, final)
		return value
	}
	return s.evalFieldChain(dot, value, variable, variable.Ident[1:], args, final)
}

// evalFieldChain evaluates .X.Y.Z possibly followed by arguments.
// dot is the environment in which to evaluate arguments, while
// receiver is the value being walked along the chain.
func (s *state) evalFieldChain(dot, receiver reflect.Value, node parse.Node, ident []string, args []parse.Node, final reflect.Value) reflect.Value {
	n := len(ident)
	for i := 0; i < n-1; i++ {
		receiver = s.evalField(dot, ident[i], node, nil, missingVal, receiver)
	}
	// Now if it's a method, it gets the arguments.
	return s.evalField(dot, ident[n-1], node, args, final, receiver)
}

func (s *state) evalFunction(dot reflect.Value, node *parse.IdentifierNode, cmd parse.Node, args []parse.Node, final reflect.Value) reflect.Value {
	s.at(node)
	name := node.Ident
	function, isBuiltin, ok := findFunction(name, s.tmpl)
	if !ok {
		s.errorf("%q is not a defined function", name)
	}
	return s.evalCall(dot, function, isBuiltin, cmd, name, args, final)
}

// evalField evaluates an expression like (.Field) or (.Field arg1 arg2).
// The 'final' argument represents the return value from the preceding
// value of the pipeline, if any.
func (s *state) evalField(dot reflect.Value, fieldName string, node parse.Node, args []parse.Node, final, receiver reflect.Value) reflect.Value {
	if !receiver.IsValid() {
		if s.tmpl.option.missingKey == mapError { // Treat invalid value as missing map key.
			s.errorf("nil data; no entry for key %q", fieldName)
		}
		return zero
	}
	typ := receiver.Type()
	receiver, isNil := indirect(receiver)
	if receiver.Kind() == reflect.Interface && isNil {
		// Calling a method on a nil interface can't work. The
		// MethodByName method call below would panic.
		s.errorf("nil pointer evaluating %s.%s", typ, fieldName)
		return zero
	}

	// Unless it's an interface, need to get to a value of type *T to guarantee
	// we see all methods of T and *T.
	ptr := receiver
	if ptr.Kind() != reflect.Interface && ptr.Kind() != reflect.Pointer && ptr.CanAddr() {
		ptr = ptr.Addr()
	}
	if method := ptr.MethodByName(fieldName); method.IsValid() {
		return s.evalCall(dot, method, false, node, fieldName, args, final)
	}
	hasArgs := len(args) > 1 || !isMissing(final)
	// It's not a method; must be a field of a struct or an element of a map.
	switch receiver.Kind() {
	case reflect.Struct:
		tField, ok := receiver.Type().FieldByName(fieldName)
		if ok {
			field, err := receiver.FieldByIndexErr(tField.Index)
			if !tField.IsExported() {
				s.errorf("%s is an unexported field of struct type %s", fieldName, typ)
			}
			if err != nil {
				s.errorf("%v", err)
			}
			// If it's a function, we must call it.
			if hasArgs {
				s.errorf("%s has arguments but cannot be invoked as function", fieldName)
			}
			return field
		}
	case reflect.Map:
		// If it's a map, attempt to use the field name as a key.
		nameVal := reflect.ValueOf(fieldName)
		if nameVal.Type().AssignableTo(receiver.Type().Key()) {
			if hasArgs {
				s.errorf("%s is not a method but has arguments", fieldName)
			}
			result := receiver.MapIndex(nameVal)
			if !result.IsValid() {
				switch s.tmpl.option.missingKey {
				case mapInvalid:
					// Just use the invalid value.
				case mapZeroValue:
					result = reflect.Zero(receiver.Type().Elem())
				case mapError:
					s.errorf("map has no entry for key %q", fieldName)
				}
			}
			return result
		}
	case reflect.Pointer:
		etyp := receiver.Type().Elem()
		if etyp.Kind() == reflect.Struct {
			if _, ok := etyp.FieldByName(fieldName); !ok {
				// If there's no such field, say "can't evaluate"
				// instead of "nil pointer evaluating".
				break
			}
		}
		if isNil {
			s.errorf("nil pointer evaluating %s.%s", typ, fieldName)
		}
	}
	s.errorf("can't evaluate field %s in type %s", fieldName, typ)
	panic("not reached")
}

var (
	errorType        = reflect.TypeFor[error]()
	fmtStringerType  = reflect.TypeFor[fmt.Stringer]()
	reflectValueType = reflect.TypeFor[reflect.Value]()
)

// evalCall executes a function or method call. If it's a method, fun already has the receiver bound, so
// it looks just like a function call. The arg list, if non-nil, includes (in the manner of the shell), arg[0]
// as the function itself.
func (s *state) evalCall(dot, fun reflect.Value, isBuiltin bool, node parse.Node, name string, args []parse.Node, final reflect.Value) reflect.Value {
	if args != nil {
		args = args[1:] // Zeroth arg is function name/node; not passed to function.
	}
	typ := fun.Type()
	numIn := len(args)
	if !isMissing(final) {
		numIn++
	}
	numFixed := len(args)
	if typ.IsVariadic() {
		numFixed = typ.NumIn() - 1 // last arg is the variadic one.
		if numIn < numFixed {
			s.errorf("wrong number of args for %s: want at least %d got %d", name, typ.NumIn()-1, len(args))
		}
	} else if numIn != typ.NumIn() {
		s.errorf("wrong number of args for %s: want %d got %d", name, typ.NumIn(), numIn)
	}
	if err := goodFunc(name, typ); err != nil {
		s.errorf("%v", err)
	}

	unwrap := func(v reflect.Value) reflect.Value {
		if v.Type() == reflectValueType {
			v = v.Interface().(reflect.Value)
		}
		return v
	}

	// Special case for builtin and/or, which short-circuit.
	if isBuiltin && (name == "and" || name == "or") {
		argType := typ.In(0)
		var v reflect.Value
		for _, arg := range args {
			v = s.evalArg(dot, argType, arg).Interface().(reflect.Value)
			if truth(v) == (name == "or") {
				// This value was already unwrapped
				// by the .Interface().(reflect.Value).
				return v
			}
		}
		if !final.Equal(missingVal) {
			// The last argument to and/or is coming from
			// the pipeline. We didn't short circuit on an earlier
			// argument, so we are going to return this one.
			// We don't have to evaluate final, but we do
			// have to check its type. Then, since we are
			// going to return it, we have to unwrap it.
			v = unwrap(s.validateType(final, argType))
		}
		return v
	}

	// Build the arg list.
	argv := make([]reflect.Value, numIn)
	// Args must be evaluated. Fixed args first.
	i := 0
	for ; i < numFixed && i < len(args); i++ {
		argv[i] = s.evalArg(dot, typ.In(i), args[i])
	}
	// Now the ... args.
	if typ.IsVariadic() {
		argType := typ.In(typ.NumIn() - 1).Elem() // Argument is a slice.
		for ; i < len(args); i++ {
			argv[i] = s.evalArg(dot, argType, args[i])
		}
	}
	// Add final value if necessary.
	if !isMissing(final) {
		t := typ.In(typ.NumIn() - 1)
		if typ.IsVariadic() {
			if numIn-1 < numFixed {
				// The added final argument corresponds to a fixed parameter of the function.
				// Validate against the type of the actual parameter.
				t = typ.In(numIn - 1)
			} else {
				// The added final argument corresponds to the variadic part.
				// Validate against the type of the elements of the variadic slice.
				t = t.Elem()
			}
		}
		argv[i] = s.validateType(final, t)
	}

	// Special case for the "call" builtin.
	// Insert the name of the callee function as the first argument.
	if isBuiltin && name == "call" {
		var calleeName string
		if len(args) == 0 {
			// final must be present or we would have errored out above.
			calleeName = final.String()
		} else {
			calleeName = args[0].String()
		}
		argv = append([]reflect.Value{reflect.ValueOf(calleeName)}, argv...)
		fun = reflect.ValueOf(call)
	}

	v, err := safeCall(fun, argv)
	// If we have an error that is not nil, stop execution and return that
	// error to the caller.
	if err != nil {
		s.at(node)
		s.errorf("error calling %s: %w", name, err)
	}
	return unwrap(v)
}

// canBeNil reports whether an untyped nil can be assigned to the type. See reflect.Zero.
func canBeNil(typ reflect.Type) bool {
	switch typ.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return true
	case reflect.Struct:
		return typ == reflectValueType
	}
	return false
}

// validateType guarantees that the value is valid and assignable to the type.
func (s *state) validateType(value reflect.Value, typ reflect.Type) reflect.Value {
	if !value.IsValid() {
		if typ == nil {
			// An untyped nil interface{}. Accept as a proper nil value.
			return reflect.ValueOf(nil)
		}
		if canBeNil(typ) {
			// Like above, but use the zero value of the non-nil type.
			return reflect.Zero(typ)
		}
		s.errorf("invalid value; expected %s", typ)
	}
	if typ == reflectValueType && value.Type() != typ {
		return reflect.ValueOf(value)
	}
	if typ != nil && !value.Type().AssignableTo(typ) {
		if value.Kind() == reflect.Interface && !value.IsNil() {
			value = value.Elem()
			if value.Type().AssignableTo(typ) {
				return value
			}
			// fallthrough
		}
		// Does one dereference or indirection work? We could do more, as we
		// do with method receivers, but that gets messy and method receivers
		// are much more constrained, so it makes more sense there than here.
		// Besides, one is almost always all you need.
		switch {
		case value.Kind() == reflect.Pointer && value.Type().Elem().AssignableTo(typ):
			value = value.Elem()
			if !value.IsValid() {
				s.errorf("dereference of nil pointer of type %s", typ)
			}
		case reflect.PointerTo(value.Type()).AssignableTo(typ) && value.CanAddr():
			value = value.Addr()
		default:
			s.errorf("wrong type for value; expected %s; got %s", typ, value.Type())
		}
	}
	return value
}

func (s *state) evalArg(dot reflect.Value, typ reflect.Type, n parse.Node) reflect.Value {
	s.at(n)
	switch arg := n.(type) {
	case *parse.DotNode:
		return s.validateType(dot, typ)
	case *parse.NilNode:
		if canBeNil(typ) {
			return reflect.Zero(typ)
		}
		s.errorf("cannot assign nil to %s", typ)
	case *parse.FieldNode:
		return s.validateType(s.evalFieldNode(dot, arg, []parse.Node{n}, missingVal), typ)
	case *parse.VariableNode:
		return s.validateType(s.evalVariableNode(dot, arg, nil, missingVal), typ)
	case *parse.PipeNode:
		return s.validateType(s.evalPipeline(dot, arg), typ)
	case *parse.IdentifierNode:
		return s.validateType(s.evalFunction(dot, arg, arg, nil, missingVal), typ)
	case *parse.ChainNode:
		return s.validateType(s.evalChainNode(dot, arg, nil, missingVal), typ)
	}
	switch typ.Kind() {
	case reflect.Bool:
		return s.evalBool(typ, n)
	case reflect.Complex64, reflect.Complex128:
		return s.evalComplex(typ, n)
	case reflect.Float32, reflect.Float64:
		return s.evalFloat(typ, n)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return s.evalInteger(typ, n)
	case reflect.Interface:
		if typ.NumMethod() == 0 {
			return s.evalEmptyInterface(dot, n)
		}
	case reflect.Struct:
		if typ == reflectValueType {
			return reflect.ValueOf(s.evalEmptyInterface(dot, n))
		}
	case reflect.String:
		return s.evalString(typ, n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return s.evalUnsignedInteger(typ, n)
	}
	s.errorf("can't handle %s for arg of type %s", n, typ)
	panic("not reached")
}

func (s *state) evalBool(typ reflect.Type, n parse.Node) reflect.Value {
	s.at(n)
	if n, ok := n.(*parse.BoolNode); ok {
		value := reflect.New(typ).Elem()
		value.SetBool(n.True)
		return value
	}
	s.errorf("expected bool; found %s", n)
	panic("not reached")
}

func (s *state) evalString(typ reflect.Type, n parse.Node) reflect.Value {
	s.at(n)
	if n, ok := n.(*parse.StringNode); ok {
		value := reflect.New(typ).Elem()
		value.SetString(n.Text)
		return value
	}
	s.errorf("expected string; found %s", n)
	panic("not reached")
}

func (s *state) evalInteger(typ reflect.Type, n parse.Node) reflect.Value {
	s.at(n)
	if n, ok := n.(*parse.NumberNode); ok && n.IsInt {
		value := reflect.New(typ).Elem()
		value.SetInt(n.Int64)
		return value
	}
	s.errorf("expected integer; found %s", n)
	panic("not reached")
}

func (s *state) evalUnsignedInteger(typ reflect.Type, n parse.Node) reflect.Value {
	s.at(n)
	if n, ok := n.(*parse.NumberNode); ok && n.IsUint {
		value := reflect.New(typ).Elem()
		value.SetUint(n.Uint64)
		return value
	}
	s.errorf("expected unsigned integer; found %s", n)
	panic("not reached")
}

func (s *state) evalFloat(typ reflect.Type, n parse.Node) reflect.Value {
	s.at(n)
	if n, ok := n.(*parse.NumberNode); ok && n.IsFloat {
		value := reflect.New(typ).Elem()
		value.SetFloat(n.Float64)
		return value
	}
	s.errorf("expected float; found %s", n)
	panic("not reached")
}

func (s *state) evalComplex(typ reflect.Type, n parse.Node) reflect.Value {
	if n, ok := n.(*parse.NumberNode); ok && n.IsComplex {
		value := reflect.New(typ).Elem()
		value.SetComplex(n.Complex128)
		return value
	}
	s.errorf("expected complex; found %s", n)
	panic("not reached")
}

func (s *state) evalEmptyInterface(dot reflect.Value, n parse.Node) reflect.Value {
	s.at(n)
	switch n := n.(type) {
	case *parse.BoolNode:
		return reflect.ValueOf(n.True)
	case *parse.DotNode:
		return dot
	case *parse.FieldNode:
		return s.evalFieldNode(dot, n, nil, missingVal)
	case *parse.IdentifierNode:
		return s.evalFunction(dot, n, n, nil, missingVal)
	case *parse.NilNode:
		// NilNode is handled in evalArg, the only place that calls here.
		s.errorf("evalEmptyInterface: nil (can't happen)")
	case *parse.NumberNode:
		return s.idealConstant(n)
	case *parse.StringNode:
		return reflect.ValueOf(n.Text)
	case *parse.VariableNode:
		return s.evalVariableNode(dot, n, nil, missingVal)
	case *parse.PipeNode:
		return s.evalPipeline(dot, n)
	}
	s.errorf("can't handle assignment of %s to empty interface argument", n)
	panic("not reached")
}

// indirect returns the item at the end of indirection, and a bool to indicate
// if it's nil. If the returned bool is true, the returned value's kind will be
// either a pointer or interface.
func indirect(v reflect.Value) (rv reflect.Value, isNil bool) {
	for ; v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface; v = v.Elem() {
		if v.IsNil() {
			return v, true
		}
	}
	return v, false
}

// indirectInterface returns the concrete value in an interface value,
// or else the zero reflect.Value.
// That is, if v represents the interface value x, the result is the same as reflect.ValueOf(x):
// the fact that x was an interface value is forgotten.
func indirectInterface(v reflect.Value) reflect.Value {
	if v.Kind() != reflect.Interface {
		return v
	}
	if v.IsNil() {
		return reflect.Value{}
	}
	return v.Elem()
}

// printValue writes the textual representation of the value to the output of
// the template.
func (s *state) printValue(n parse.Node, v reflect.Value) {
	s.at(n)
	iface, ok := printableValue(v)
	if !ok {
		s.errorf("can't print %s of type %s", n, v.Type())
	}
	_, err := fmt.Fprint(s.wr, iface)
	if err != nil {
		s.writeError(err)
	}
}

// printableValue returns the, possibly indirected, interface value inside v that
// is best for a call to formatted printer.
func printableValue(v reflect.Value) (any, bool) {
	if v.Kind() == reflect.Pointer {
		v, _ = indirect(v) // fmt.Fprint handles nil.
	}
	if !v.IsValid() {
		return "<no value>", true
	}

	if !v.Type().Implements(errorType) && !v.Type().Implements(fmtStringerType) {
		if v.CanAddr() && (reflect.PointerTo(v.Type()).Implements(errorType) || reflect.PointerTo(v.Type()).Implements(fmtStringerType)) {
			v = v.Addr()
		} else {
			switch v.Kind() {
			case reflect.Chan, reflect.Func:
				return nil, false
			}
		}
	}
	return v.Interface(), true
}

```

// === FILE: references/go/src/text/template/funcs.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// FuncMap is the type of the map defining the mapping from names to functions.
// Each function must have either a single return value, or two return values of
// which the second has type error. In that case, if the second (error)
// return value evaluates to non-nil during execution, execution terminates and
// Execute returns that error.
//
// Errors returned by Execute wrap the underlying error; call [errors.AsType] to
// unwrap them.
//
// When template execution invokes a function with an argument list, that list
// must be assignable to the function's parameter types. Functions meant to
// apply to arguments of arbitrary type can use parameters of type interface{} or
// of type [reflect.Value]. Similarly, functions meant to return a result of arbitrary
// type can return interface{} or [reflect.Value].
type FuncMap map[string]any

// builtins returns the FuncMap.
// It is not a global variable so the linker can dead code eliminate
// more when this isn't called. See golang.org/issue/36021.
// TODO: revert this back to a global map once golang.org/issue/2559 is fixed.
func builtins() FuncMap {
	return FuncMap{
		"and":      and,
		"call":     emptyCall,
		"html":     HTMLEscaper,
		"index":    index,
		"slice":    slice,
		"js":       JSEscaper,
		"len":      length,
		"not":      not,
		"or":       or,
		"print":    fmt.Sprint,
		"printf":   fmt.Sprintf,
		"println":  fmt.Sprintln,
		"urlquery": URLQueryEscaper,

		// Comparisons
		"eq": eq, // ==
		"ge": ge, // >=
		"gt": gt, // >
		"le": le, // <=
		"lt": lt, // <
		"ne": ne, // !=
	}
}

// builtinFuncs lazily computes & caches the builtinFuncs map.
var builtinFuncs = sync.OnceValue(func() map[string]reflect.Value {
	funcMap := builtins()
	m := make(map[string]reflect.Value, len(funcMap))
	addValueFuncs(m, funcMap)
	return m
})

// addValueFuncs adds to values the functions in funcs, converting them to reflect.Values.
func addValueFuncs(out map[string]reflect.Value, in FuncMap) {
	for name, fn := range in {
		if !goodName(name) {
			panic(fmt.Errorf("function name %q is not a valid identifier", name))
		}
		v := reflect.ValueOf(fn)
		if v.Kind() != reflect.Func {
			panic("value for " + name + " not a function")
		}
		if err := goodFunc(name, v.Type()); err != nil {
			panic(err)
		}
		out[name] = v
	}
}

// addFuncs adds to values the functions in funcs. It does no checking of the input -
// call addValueFuncs first.
func addFuncs(out, in FuncMap) {
	for name, fn := range in {
		out[name] = fn
	}
}

// goodFunc reports whether the function or method has the right result signature.
func goodFunc(name string, typ reflect.Type) error {
	// We allow functions with 1 result or 2 results where the second is an error.
	switch numOut := typ.NumOut(); {
	case numOut == 1:
		return nil
	case numOut == 2 && typ.Out(1) == errorType:
		return nil
	case numOut == 2:
		return fmt.Errorf("invalid function signature for %s: second return value should be error; is %s", name, typ.Out(1))
	default:
		return fmt.Errorf("function %s has %d return values; should be 1 or 2", name, typ.NumOut())
	}
}

// goodName reports whether the function name is a valid identifier.
func goodName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r == '_':
		case i == 0 && !unicode.IsLetter(r):
			return false
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			return false
		}
	}
	return true
}

// findFunction looks for a function in the template, and global map.
func findFunction(name string, tmpl *Template) (v reflect.Value, isBuiltin, ok bool) {
	if tmpl != nil && tmpl.common != nil {
		tmpl.muFuncs.RLock()
		defer tmpl.muFuncs.RUnlock()
		if fn := tmpl.execFuncs[name]; fn.IsValid() {
			return fn, false, true
		}
	}
	if fn := builtinFuncs()[name]; fn.IsValid() {
		return fn, true, true
	}
	return reflect.Value{}, false, false
}

// prepareArg checks if value can be used as an argument of type argType, and
// converts an invalid value to appropriate zero if possible.
func prepareArg(value reflect.Value, argType reflect.Type) (reflect.Value, error) {
	if !value.IsValid() {
		if !canBeNil(argType) {
			return reflect.Value{}, fmt.Errorf("value is nil; should be of type %s", argType)
		}
		value = reflect.Zero(argType)
	}
	if value.Type().AssignableTo(argType) {
		return value, nil
	}
	if intLike(value.Kind()) && intLike(argType.Kind()) && value.Type().ConvertibleTo(argType) {
		value = value.Convert(argType)
		return value, nil
	}
	return reflect.Value{}, fmt.Errorf("value has type %s; should be %s", value.Type(), argType)
}

func intLike(typ reflect.Kind) bool {
	switch typ {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return true
	}
	return false
}

// indexArg checks if a reflect.Value can be used as an index, and converts it to int if possible.
func indexArg(index reflect.Value, cap int) (int, error) {
	var x int64
	switch index.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		x = index.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		x = int64(index.Uint())
	case reflect.Invalid:
		return 0, fmt.Errorf("cannot index slice/array with nil")
	default:
		return 0, fmt.Errorf("cannot index slice/array with type %s", index.Type())
	}
	if x < 0 || int(x) < 0 || int(x) > cap {
		return 0, fmt.Errorf("index out of range: %d", x)
	}
	return int(x), nil
}

// Indexing.

// index returns the result of indexing its first argument by the following
// arguments. Thus "index x 1 2 3" is, in Go syntax, x[1][2][3]. Each
// indexed item must be a map, slice, or array.
func index(item reflect.Value, indexes ...reflect.Value) (reflect.Value, error) {
	item = indirectInterface(item)
	if !item.IsValid() {
		return reflect.Value{}, fmt.Errorf("index of untyped nil")
	}
	for _, index := range indexes {
		index = indirectInterface(index)
		var isNil bool
		if item, isNil = indirect(item); isNil {
			return reflect.Value{}, fmt.Errorf("index of nil pointer")
		}
		switch item.Kind() {
		case reflect.Array, reflect.Slice, reflect.String:
			x, err := indexArg(index, item.Len())
			if err != nil {
				return reflect.Value{}, err
			}
			item = item.Index(x)
		case reflect.Map:
			index, err := prepareArg(index, item.Type().Key())
			if err != nil {
				return reflect.Value{}, err
			}
			if x := item.MapIndex(index); x.IsValid() {
				item = x
			} else {
				item = reflect.Zero(item.Type().Elem())
			}
		case reflect.Invalid:
			// the loop holds invariant: item.IsValid()
			panic("unreachable")
		default:
			return reflect.Value{}, fmt.Errorf("can't index item of type %s", item.Type())
		}
	}
	return item, nil
}

// Slicing.

// slice returns the result of slicing its first argument by the remaining
// arguments. Thus "slice x 1 2" is, in Go syntax, x[1:2], while "slice x"
// is x[:], "slice x 1" is x[1:], and "slice x 1 2 3" is x[1:2:3]. The first
// argument must be a string, slice, or array.
func slice(item reflect.Value, indexes ...reflect.Value) (reflect.Value, error) {
	item = indirectInterface(item)
	if !item.IsValid() {
		return reflect.Value{}, fmt.Errorf("slice of untyped nil")
	}
	var isNil bool
	if item, isNil = indirect(item); isNil {
		return reflect.Value{}, fmt.Errorf("slice of nil pointer")
	}
	if len(indexes) > 3 {
		return reflect.Value{}, fmt.Errorf("too many slice indexes: %d", len(indexes))
	}
	var cap int
	switch item.Kind() {
	case reflect.String:
		if len(indexes) == 3 {
			return reflect.Value{}, fmt.Errorf("cannot 3-index slice a string")
		}
		cap = item.Len()
	case reflect.Array, reflect.Slice:
		cap = item.Cap()
	default:
		return reflect.Value{}, fmt.Errorf("can't slice item of type %s", item.Type())
	}
	// set default values for cases item[:], item[i:].
	idx := [3]int{0, item.Len()}
	for i, index := range indexes {
		x, err := indexArg(index, cap)
		if err != nil {
			return reflect.Value{}, err
		}
		idx[i] = x
	}
	// given item[i:j], make sure i <= j.
	if idx[0] > idx[1] {
		return reflect.Value{}, fmt.Errorf("invalid slice index: %d > %d", idx[0], idx[1])
	}
	if len(indexes) < 3 {
		return item.Slice(idx[0], idx[1]), nil
	}
	// given item[i:j:k], make sure i <= j <= k.
	if idx[1] > idx[2] {
		return reflect.Value{}, fmt.Errorf("invalid slice index: %d > %d", idx[1], idx[2])
	}
	return item.Slice3(idx[0], idx[1], idx[2]), nil
}

// Length

// length returns the length of the item, with an error if it has no defined length.
func length(item reflect.Value) (int, error) {
	item, isNil := indirect(item)
	if isNil {
		return 0, fmt.Errorf("len of nil pointer")
	}
	switch item.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
		return item.Len(), nil
	}
	return 0, fmt.Errorf("len of type %s", item.Type())
}

// Function invocation

func emptyCall(fn reflect.Value, args ...reflect.Value) reflect.Value {
	panic("unreachable") // implemented as a special case in evalCall
}

// call returns the result of evaluating the first argument as a function.
// The function must return 1 result, or 2 results, the second of which is an error.
func call(name string, fn reflect.Value, args ...reflect.Value) (reflect.Value, error) {
	fn = indirectInterface(fn)
	if !fn.IsValid() {
		return reflect.Value{}, fmt.Errorf("call of nil")
	}
	typ := fn.Type()
	if typ.Kind() != reflect.Func {
		return reflect.Value{}, fmt.Errorf("non-function %s of type %s", name, typ)
	}

	if err := goodFunc(name, typ); err != nil {
		return reflect.Value{}, err
	}
	numIn := typ.NumIn()
	var dddType reflect.Type
	if typ.IsVariadic() {
		if len(args) < numIn-1 {
			return reflect.Value{}, fmt.Errorf("wrong number of args for %s: got %d want at least %d", name, len(args), numIn-1)
		}
		dddType = typ.In(numIn - 1).Elem()
	} else {
		if len(args) != numIn {
			return reflect.Value{}, fmt.Errorf("wrong number of args for %s: got %d want %d", name, len(args), numIn)
		}
	}
	argv := make([]reflect.Value, len(args))
	for i, arg := range args {
		arg = indirectInterface(arg)
		// Compute the expected type. Clumsy because of variadics.
		argType := dddType
		if !typ.IsVariadic() || i < numIn-1 {
			argType = typ.In(i)
		}

		var err error
		if argv[i], err = prepareArg(arg, argType); err != nil {
			return reflect.Value{}, fmt.Errorf("arg %d: %w", i, err)
		}
	}
	return safeCall(fn, argv)
}

// safeCall runs fun.Call(args), and returns the resulting value and error, if
// any. If the call panics, the panic value is returned as an error.
func safeCall(fun reflect.Value, args []reflect.Value) (val reflect.Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("%v", r)
			}
		}
	}()
	ret := fun.Call(args)
	if len(ret) == 2 && !ret[1].IsNil() {
		return ret[0], ret[1].Interface().(error)
	}
	return ret[0], nil
}

// Boolean logic.

func truth(arg reflect.Value) bool {
	t, _ := isTrue(indirectInterface(arg))
	return t
}

// and computes the Boolean AND of its arguments, returning
// the first false argument it encounters, or the last argument.
func and(arg0 reflect.Value, args ...reflect.Value) reflect.Value {
	panic("unreachable") // implemented as a special case in evalCall
}

// or computes the Boolean OR of its arguments, returning
// the first true argument it encounters, or the last argument.
func or(arg0 reflect.Value, args ...reflect.Value) reflect.Value {
	panic("unreachable") // implemented as a special case in evalCall
}

// not returns the Boolean negation of its argument.
func not(arg reflect.Value) bool {
	return !truth(arg)
}

// Comparison.

// TODO: Perhaps allow comparison between signed and unsigned integers.

var (
	errBadComparisonType = errors.New("invalid type for comparison")
	errNoComparison      = errors.New("missing argument for comparison")
)

type kind int

const (
	invalidKind kind = iota
	boolKind
	complexKind
	intKind
	floatKind
	stringKind
	uintKind
)

func basicKind(v reflect.Value) (kind, error) {
	switch v.Kind() {
	case reflect.Bool:
		return boolKind, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intKind, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return uintKind, nil
	case reflect.Float32, reflect.Float64:
		return floatKind, nil
	case reflect.Complex64, reflect.Complex128:
		return complexKind, nil
	case reflect.String:
		return stringKind, nil
	}
	return invalidKind, errBadComparisonType
}

// isNil returns true if v is the zero reflect.Value, or nil of its type.
func isNil(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	}
	return false
}

// canCompare reports whether v1 and v2 are both the same kind, or one is nil.
// Called only when dealing with nillable types, or there's about to be an error.
func canCompare(v1, v2 reflect.Value) bool {
	k1 := v1.Kind()
	k2 := v2.Kind()
	if k1 == k2 {
		return true
	}
	// We know the type can be compared to nil.
	return k1 == reflect.Invalid || k2 == reflect.Invalid
}

// eq evaluates the comparison a == b || a == c || ...
func eq(arg1 reflect.Value, arg2 ...reflect.Value) (bool, error) {
	arg1 = indirectInterface(arg1)
	if len(arg2) == 0 {
		return false, errNoComparison
	}
	k1, _ := basicKind(arg1)
	for _, arg := range arg2 {
		arg = indirectInterface(arg)
		k2, _ := basicKind(arg)
		truth := false
		if k1 != k2 {
			// Special case: Can compare integer values regardless of type's sign.
			switch {
			case k1 == intKind && k2 == uintKind:
				truth = arg1.Int() >= 0 && uint64(arg1.Int()) == arg.Uint()
			case k1 == uintKind && k2 == intKind:
				truth = arg.Int() >= 0 && arg1.Uint() == uint64(arg.Int())
			default:
				if arg1.IsValid() && arg.IsValid() {
					return false, fmt.Errorf("incompatible types for comparison: %v and %v", arg1.Type(), arg.Type())
				}
			}
		} else {
			switch k1 {
			case boolKind:
				truth = arg1.Bool() == arg.Bool()
			case complexKind:
				truth = arg1.Complex() == arg.Complex()
			case floatKind:
				truth = arg1.Float() == arg.Float()
			case intKind:
				truth = arg1.Int() == arg.Int()
			case stringKind:
				truth = arg1.String() == arg.String()
			case uintKind:
				truth = arg1.Uint() == arg.Uint()
			default:
				if !canCompare(arg1, arg) {
					return false, fmt.Errorf("non-comparable types %s: %v, %s: %v", arg1, arg1.Type(), arg.Type(), arg)
				}
				if isNil(arg1) || isNil(arg) {
					truth = isNil(arg) == isNil(arg1)
				} else {
					if !arg.Type().Comparable() {
						return false, fmt.Errorf("non-comparable type %s: %v", arg, arg.Type())
					}
					truth = arg1.Interface() == arg.Interface()
				}
			}
		}
		if truth {
			return true, nil
		}
	}
	return false, nil
}

// ne evaluates the comparison a != b.
func ne(arg1, arg2 reflect.Value) (bool, error) {
	// != is the inverse of ==.
	equal, err := eq(arg1, arg2)
	return !equal, err
}

// lt evaluates the comparison a < b.
func lt(arg1, arg2 reflect.Value) (bool, error) {
	arg1 = indirectInterface(arg1)
	k1, err := basicKind(arg1)
	if err != nil {
		return false, err
	}
	arg2 = indirectInterface(arg2)
	k2, err := basicKind(arg2)
	if err != nil {
		return false, err
	}
	truth := false
	if k1 != k2 {
		// Special case: Can compare integer values regardless of type's sign.
		switch {
		case k1 == intKind && k2 == uintKind:
			truth = arg1.Int() < 0 || uint64(arg1.Int()) < arg2.Uint()
		case k1 == uintKind && k2 == intKind:
			truth = arg2.Int() >= 0 && arg1.Uint() < uint64(arg2.Int())
		default:
			return false, fmt.Errorf("incompatible types for comparison: %v and %v", arg1.Type(), arg2.Type())
		}
	} else {
		switch k1 {
		case boolKind, complexKind:
			return false, errBadComparisonType
		case floatKind:
			truth = arg1.Float() < arg2.Float()
		case intKind:
			truth = arg1.Int() < arg2.Int()
		case stringKind:
			truth = arg1.String() < arg2.String()
		case uintKind:
			truth = arg1.Uint() < arg2.Uint()
		default:
			panic("invalid kind")
		}
	}
	return truth, nil
}

// le evaluates the comparison <= b.
func le(arg1, arg2 reflect.Value) (bool, error) {
	// <= is < or ==.
	lessThan, err := lt(arg1, arg2)
	if lessThan || err != nil {
		return lessThan, err
	}
	return eq(arg1, arg2)
}

// gt evaluates the comparison a > b.
func gt(arg1, arg2 reflect.Value) (bool, error) {
	// > is the inverse of <=.
	lessOrEqual, err := le(arg1, arg2)
	if err != nil {
		return false, err
	}
	return !lessOrEqual, nil
}

// ge evaluates the comparison a >= b.
func ge(arg1, arg2 reflect.Value) (bool, error) {
	// >= is the inverse of <.
	lessThan, err := lt(arg1, arg2)
	if err != nil {
		return false, err
	}
	return !lessThan, nil
}

// HTML escaping.

var (
	htmlQuot = []byte("&#34;") // shorter than "&quot;"
	htmlApos = []byte("&#39;") // shorter than "&apos;" and apos was not in HTML until HTML5
	htmlAmp  = []byte("&amp;")
	htmlLt   = []byte("&lt;")
	htmlGt   = []byte("&gt;")
	htmlNull = []byte("\uFFFD")
)

// HTMLEscape writes to w the escaped HTML equivalent of the plain text data b.
func HTMLEscape(w io.Writer, b []byte) {
	last := 0
	for i, c := range b {
		var html []byte
		switch c {
		case '\000':
			html = htmlNull
		case '"':
			html = htmlQuot
		case '\'':
			html = htmlApos
		case '&':
			html = htmlAmp
		case '<':
			html = htmlLt
		case '>':
			html = htmlGt
		default:
			continue
		}
		w.Write(b[last:i])
		w.Write(html)
		last = i + 1
	}
	w.Write(b[last:])
}

// HTMLEscapeString returns the escaped HTML equivalent of the plain text data s.
func HTMLEscapeString(s string) string {
	// Avoid allocation if we can.
	if !strings.ContainsAny(s, "'\"&<>\000") {
		return s
	}
	var b strings.Builder
	HTMLEscape(&b, []byte(s))
	return b.String()
}

// HTMLEscaper returns the escaped HTML equivalent of the textual
// representation of its arguments.
func HTMLEscaper(args ...any) string {
	return HTMLEscapeString(evalArgs(args))
}

// JavaScript escaping.

var (
	jsLowUni = []byte(`\u00`)
	hex      = []byte("0123456789ABCDEF")

	jsBackslash = []byte(`\\`)
	jsApos      = []byte(`\'`)
	jsQuot      = []byte(`\"`)
	jsLt        = []byte(`\u003C`)
	jsGt        = []byte(`\u003E`)
	jsAmp       = []byte(`\u0026`)
	jsEq        = []byte(`\u003D`)
)

// JSEscape writes to w the escaped JavaScript equivalent of the plain text data b.
func JSEscape(w io.Writer, b []byte) {
	last := 0
	for i := 0; i < len(b); i++ {
		c := b[i]

		if !jsIsSpecial(rune(c)) {
			// fast path: nothing to do
			continue
		}
		w.Write(b[last:i])

		if c < utf8.RuneSelf {
			// Quotes, slashes and angle brackets get quoted.
			// Control characters get written as \u00XX.
			switch c {
			case '\\':
				w.Write(jsBackslash)
			case '\'':
				w.Write(jsApos)
			case '"':
				w.Write(jsQuot)
			case '<':
				w.Write(jsLt)
			case '>':
				w.Write(jsGt)
			case '&':
				w.Write(jsAmp)
			case '=':
				w.Write(jsEq)
			default:
				w.Write(jsLowUni)
				t, b := c>>4, c&0x0f
				w.Write(hex[t : t+1])
				w.Write(hex[b : b+1])
			}
		} else {
			// Unicode rune.
			r, size := utf8.DecodeRune(b[i:])
			if unicode.IsPrint(r) {
				w.Write(b[i : i+size])
			} else {
				fmt.Fprintf(w, "\\u%04X", r)
			}
			i += size - 1
		}
		last = i + 1
	}
	w.Write(b[last:])
}

// JSEscapeString returns the escaped JavaScript equivalent of the plain text data s.
func JSEscapeString(s string) string {
	// Avoid allocation if we can.
	if strings.IndexFunc(s, jsIsSpecial) < 0 {
		return s
	}
	var b strings.Builder
	JSEscape(&b, []byte(s))
	return b.String()
}

func jsIsSpecial(r rune) bool {
	switch r {
	case '\\', '\'', '"', '<', '>', '&', '=':
		return true
	}
	return r < ' ' || utf8.RuneSelf <= r
}

// JSEscaper returns the escaped JavaScript equivalent of the textual
// representation of its arguments.
func JSEscaper(args ...any) string {
	return JSEscapeString(evalArgs(args))
}

// URLQueryEscaper returns the escaped value of the textual representation of
// its arguments in a form suitable for embedding in a URL query.
func URLQueryEscaper(args ...any) string {
	return url.QueryEscape(evalArgs(args))
}

// evalArgs formats the list of arguments into a string. It is therefore equivalent to
//
//	fmt.Sprint(args...)
//
// except that each argument is indirected (if a pointer), as required,
// using the same rules as the default string evaluation during template
// execution.
func evalArgs(args []any) string {
	ok := false
	var s string
	// Fast path for simple common case.
	if len(args) == 1 {
		s, ok = args[0].(string)
	}
	if !ok {
		for i, arg := range args {
			a, ok := printableValue(reflect.ValueOf(arg))
			if ok {
				args[i] = a
			} // else let fmt do its thing
		}
		s = fmt.Sprint(args...)
	}
	return s
}

```

// === FILE: references/go/src/text/template/helper.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Helper functions to make constructing templates easier.

package template

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// Functions and methods to parse templates.

// Must is a helper that wraps a call to a function returning ([*Template], error)
// and panics if the error is non-nil. It is intended for use in variable
// initializations such as
//
//	var t = template.Must(template.New("name").Parse("text"))
func Must(t *Template, err error) *Template {
	if err != nil {
		panic(err)
	}
	return t
}

// ParseFiles creates a new [Template] and parses the template definitions from
// the named files. The returned template's name will have the base name and
// parsed contents of the first file. There must be at least one file.
// If an error occurs, parsing stops and the returned *Template is nil.
//
// When parsing multiple files with the same name in different directories,
// the last one mentioned will be the one that results.
// For instance, ParseFiles("a/foo", "b/foo") stores "b/foo" as the template
// named "foo", while "a/foo" is unavailable.
func ParseFiles(filenames ...string) (*Template, error) {
	return parseFiles(nil, readFileOS, filenames...)
}

// ParseFiles parses the named files and associates the resulting templates with
// t. If an error occurs, parsing stops and the returned template is nil;
// otherwise it is t. There must be at least one file.
// Since the templates created by ParseFiles are named by the base
// (see [filepath.Base]) names of the argument files, t should usually have the
// name of one of the (base) names of the files. If it does not, depending on
// t's contents before calling ParseFiles, t.Execute may fail. In that
// case use t.ExecuteTemplate to execute a valid template.
//
// When parsing multiple files with the same name in different directories,
// the last one mentioned will be the one that results.
func (t *Template) ParseFiles(filenames ...string) (*Template, error) {
	t.init()
	return parseFiles(t, readFileOS, filenames...)
}

// parseFiles is the helper for the method and function. If the argument
// template is nil, it is created from the first file.
func parseFiles(t *Template, readFile func(string) (string, []byte, error), filenames ...string) (*Template, error) {
	if len(filenames) == 0 {
		// Not really a problem, but be consistent.
		return nil, fmt.Errorf("template: no files named in call to ParseFiles")
	}
	for _, filename := range filenames {
		name, b, err := readFile(filename)
		if err != nil {
			return nil, err
		}
		s := string(b)
		// First template becomes return value if not already defined,
		// and we use that one for subsequent New calls to associate
		// all the templates together. Also, if this file has the same name
		// as t, this file becomes the contents of t, so
		//  t, err := New(name).Funcs(xxx).ParseFiles(name)
		// works. Otherwise we create a new template associated with t.
		var tmpl *Template
		if t == nil {
			t = New(name)
		}
		if name == t.Name() {
			tmpl = t
		} else {
			tmpl = t.New(name)
		}
		_, err = tmpl.Parse(s)
		if err != nil {
			return nil, err
		}
	}
	return t, nil
}

// ParseGlob creates a new [Template] and parses the template definitions from
// the files identified by the pattern. The files are matched according to the
// semantics of [filepath.Match], and the pattern must match at least one file.
// The returned template will have the [filepath.Base] name and (parsed)
// contents of the first file matched by the pattern. ParseGlob is equivalent to
// calling [ParseFiles] with the list of files matched by the pattern.
//
// When parsing multiple files with the same name in different directories,
// the last one mentioned will be the one that results.
func ParseGlob(pattern string) (*Template, error) {
	return parseGlob(nil, pattern)
}

// ParseGlob parses the template definitions in the files identified by the
// pattern and associates the resulting templates with t. The files are matched
// according to the semantics of [filepath.Match], and the pattern must match at
// least one file. ParseGlob is equivalent to calling [Template.ParseFiles] with
// the list of files matched by the pattern.
//
// When parsing multiple files with the same name in different directories,
// the last one mentioned will be the one that results.
func (t *Template) ParseGlob(pattern string) (*Template, error) {
	t.init()
	return parseGlob(t, pattern)
}

// parseGlob is the implementation of the function and method ParseGlob.
func parseGlob(t *Template, pattern string) (*Template, error) {
	filenames, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	if len(filenames) == 0 {
		return nil, fmt.Errorf("template: pattern matches no files: %#q", pattern)
	}
	return parseFiles(t, readFileOS, filenames...)
}

// ParseFS is like [Template.ParseFiles] or [Template.ParseGlob] but reads from the file system fsys
// instead of the host operating system's file system.
// It accepts a list of glob patterns (see [path.Match]).
// (Note that most file names serve as glob patterns matching only themselves.)
func ParseFS(fsys fs.FS, patterns ...string) (*Template, error) {
	return parseFS(nil, fsys, patterns)
}

// ParseFS is like [Template.ParseFiles] or [Template.ParseGlob] but reads from the file system fsys
// instead of the host operating system's file system.
// It accepts a list of glob patterns (see [path.Match]).
// (Note that most file names serve as glob patterns matching only themselves.)
func (t *Template) ParseFS(fsys fs.FS, patterns ...string) (*Template, error) {
	t.init()
	return parseFS(t, fsys, patterns)
}

func parseFS(t *Template, fsys fs.FS, patterns []string) (*Template, error) {
	var filenames []string
	for _, pattern := range patterns {
		list, err := fs.Glob(fsys, pattern)
		if err != nil {
			return nil, err
		}
		if len(list) == 0 {
			return nil, fmt.Errorf("template: pattern matches no files: %#q", pattern)
		}
		filenames = append(filenames, list...)
	}
	return parseFiles(t, readFileFS(fsys), filenames...)
}

func readFileOS(file string) (name string, b []byte, err error) {
	name = filepath.Base(file)
	b, err = os.ReadFile(file)
	return
}

func readFileFS(fsys fs.FS) func(string) (string, []byte, error) {
	return func(file string) (name string, b []byte, err error) {
		name = path.Base(file)
		b, err = fs.ReadFile(fsys, file)
		return
	}
}

```

// === FILE: references/go/src/text/template/option.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains the code to handle template options.

package template

import "strings"

// missingKeyAction defines how to respond to indexing a map with a key that is not present.
type missingKeyAction int

const (
	mapInvalid   missingKeyAction = iota // Return an invalid reflect.Value.
	mapZeroValue                         // Return the zero value for the map element.
	mapError                             // Error out
)

type option struct {
	missingKey missingKeyAction
}

// Option sets options for the template. Options are described by
// strings, either a simple string or "key=value". There can be at
// most one equals sign in an option string. If the option string
// is unrecognized or otherwise invalid, Option panics.
//
// Known options:
//
// missingkey: Control the behavior during execution if a map is
// indexed with a key that is not present in the map.
//
//	"missingkey=default" or "missingkey=invalid"
//		The default behavior: Do nothing and continue execution.
//		If printed, the result of the index operation is the string
//		"<no value>".
//	"missingkey=zero"
//		The operation returns the zero value for the map type's element.
//	"missingkey=error"
//		Execution stops immediately with an error.
func (t *Template) Option(opt ...string) *Template {
	t.init()
	for _, s := range opt {
		t.setOption(s)
	}
	return t
}

func (t *Template) setOption(opt string) {
	if opt == "" {
		panic("empty option string")
	}
	// key=value
	if key, value, ok := strings.Cut(opt, "="); ok {
		switch key {
		case "missingkey":
			switch value {
			case "invalid", "default":
				t.option.missingKey = mapInvalid
				return
			case "zero":
				t.option.missingKey = mapZeroValue
				return
			case "error":
				t.option.missingKey = mapError
				return
			}
		}
	}
	panic("unrecognized option: " + opt)
}

```

// === FILE: references/go/src/text/template/parse/lex.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// item represents a token or text string returned from the scanner.
type item struct {
	typ  itemType // The type of this item.
	pos  Pos      // The starting position, in bytes, of this item in the input string.
	val  string   // The value of this item.
	line int      // The line number at the start of this item.
}

func (i item) String() string {
	switch {
	case i.typ == itemEOF:
		return "EOF"
	case i.typ == itemError:
		return i.val
	case i.typ > itemKeyword:
		return fmt.Sprintf("<%s>", i.val)
	case len(i.val) > 10:
		return fmt.Sprintf("%.10q...", i.val)
	}
	return fmt.Sprintf("%q", i.val)
}

// itemType identifies the type of lex items.
type itemType int

const (
	itemError        itemType = iota // error occurred; value is text of error
	itemBool                         // boolean constant
	itemChar                         // printable ASCII character; grab bag for comma etc.
	itemCharConstant                 // character constant
	itemComment                      // comment text
	itemComplex                      // complex constant (1+2i); imaginary is just a number
	itemAssign                       // equals ('=') introducing an assignment
	itemDeclare                      // colon-equals (':=') introducing a declaration
	itemEOF
	itemField      // alphanumeric identifier starting with '.'
	itemIdentifier // alphanumeric identifier not starting with '.'
	itemLeftDelim  // left action delimiter
	itemLeftParen  // '(' inside action
	itemNumber     // simple number, including imaginary
	itemPipe       // pipe symbol
	itemRawString  // raw quoted string (includes quotes)
	itemRightDelim // right action delimiter
	itemRightParen // ')' inside action
	itemSpace      // run of spaces separating arguments
	itemString     // quoted string (includes quotes)
	itemText       // plain text
	itemVariable   // variable starting with '$', such as '$' or  '$1' or '$hello'
	// Keywords appear after all the rest.
	itemKeyword  // used only to delimit the keywords
	itemBlock    // block keyword
	itemBreak    // break keyword
	itemContinue // continue keyword
	itemDot      // the cursor, spelled '.'
	itemDefine   // define keyword
	itemElse     // else keyword
	itemEnd      // end keyword
	itemIf       // if keyword
	itemNil      // the untyped nil constant, easiest to treat as a keyword
	itemRange    // range keyword
	itemTemplate // template keyword
	itemWith     // with keyword
)

var key = map[string]itemType{
	".":        itemDot,
	"block":    itemBlock,
	"break":    itemBreak,
	"continue": itemContinue,
	"define":   itemDefine,
	"else":     itemElse,
	"end":      itemEnd,
	"if":       itemIf,
	"range":    itemRange,
	"nil":      itemNil,
	"template": itemTemplate,
	"with":     itemWith,
}

const eof = -1

// Trimming spaces.
// If the action begins "{{- " rather than "{{", then all space/tab/newlines
// preceding the action are trimmed; conversely if it ends " -}}" the
// leading spaces are trimmed. This is done entirely in the lexer; the
// parser never sees it happen. We require an ASCII space (' ', \t, \r, \n)
// to be present to avoid ambiguity with things like "{{-3}}". It reads
// better with the space present anyway. For simplicity, only ASCII
// does the job.
const (
	spaceChars    = " \t\r\n"  // These are the space characters defined by Go itself.
	trimMarker    = '-'        // Attached to left/right delimiter, trims trailing spaces from preceding/following text.
	trimMarkerLen = Pos(1 + 1) // marker plus space before or after
)

// stateFn represents the state of the scanner as a function that returns the next state.
type stateFn func(*lexer) stateFn

// lexer holds the state of the scanner.
type lexer struct {
	name         string // the name of the input; used only for error reports
	input        string // the string being scanned
	leftDelim    string // start of action marker
	rightDelim   string // end of action marker
	pos          Pos    // current position in the input
	start        Pos    // start position of this item
	atEOF        bool   // we have hit the end of input and returned eof
	parenDepth   int    // nesting depth of ( ) exprs
	line         int    // 1+number of newlines seen
	startLine    int    // start line of this item
	item         item   // item to return to parser
	insideAction bool   // are we inside an action?
	options      lexOptions
}

// lexOptions control behavior of the lexer. All default to false.
type lexOptions struct {
	emitComment bool // emit itemComment tokens.
	breakOK     bool // break keyword allowed
	continueOK  bool // continue keyword allowed
}

// next returns the next rune in the input.
func (l *lexer) next() rune {
	if int(l.pos) >= len(l.input) {
		l.atEOF = true
		return eof
	}
	r, w := utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += Pos(w)
	if r == '\n' {
		l.line++
	}
	return r
}

// peek returns but does not consume the next rune in the input.
func (l *lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

// backup steps back one rune.
func (l *lexer) backup() {
	if !l.atEOF && l.pos > 0 {
		r, w := utf8.DecodeLastRuneInString(l.input[:l.pos])
		l.pos -= Pos(w)
		// Correct newline count.
		if r == '\n' {
			l.line--
		}
	}
}

// thisItem returns the item at the current input point with the specified type
// and advances the input.
func (l *lexer) thisItem(t itemType) item {
	i := item{t, l.start, l.input[l.start:l.pos], l.startLine}
	l.start = l.pos
	l.startLine = l.line
	return i
}

// emit passes the trailing text as an item back to the parser.
func (l *lexer) emit(t itemType) stateFn {
	return l.emitItem(l.thisItem(t))
}

// emitItem passes the specified item to the parser.
func (l *lexer) emitItem(i item) stateFn {
	l.item = i
	return nil
}

// ignore skips over the pending input before this point.
// It tracks newlines in the ignored text, so use it only
// for text that is skipped without calling l.next.
func (l *lexer) ignore() {
	l.line += strings.Count(l.input[l.start:l.pos], "\n")
	l.start = l.pos
	l.startLine = l.line
}

// accept consumes the next rune if it's from the valid set.
func (l *lexer) accept(valid string) bool {
	if strings.ContainsRune(valid, l.next()) {
		return true
	}
	l.backup()
	return false
}

// acceptRun consumes a run of runes from the valid set.
func (l *lexer) acceptRun(valid string) {
	for strings.ContainsRune(valid, l.next()) {
	}
	l.backup()
}

// errorf returns an error token and terminates the scan by passing
// back a nil pointer that will be the next state, terminating l.nextItem.
func (l *lexer) errorf(format string, args ...any) stateFn {
	l.item = item{itemError, l.start, fmt.Sprintf(format, args...), l.startLine}
	l.start = 0
	l.pos = 0
	l.input = l.input[:0]
	return nil
}

// nextItem returns the next item from the input.
// Called by the parser, not in the lexing goroutine.
func (l *lexer) nextItem() item {
	l.item = item{itemEOF, l.pos, "EOF", l.startLine}
	state := lexText
	if l.insideAction {
		state = lexInsideAction
	}
	for {
		state = state(l)
		if state == nil {
			return l.item
		}
	}
}

// lex creates a new scanner for the input string.
func lex(name, input, left, right string) *lexer {
	if left == "" {
		left = defaultLeftDelim
	}
	if right == "" {
		right = defaultRightDelim
	}
	l := &lexer{
		name:         name,
		input:        input,
		leftDelim:    left,
		rightDelim:   right,
		line:         1,
		startLine:    1,
		insideAction: false,
	}
	return l
}

// state functions

const (
	defaultLeftDelim  = "{{"
	defaultRightDelim = "}}"
	leftComment       = "/*"
	rightComment      = "*/"
)

// lexText scans until an opening action delimiter, "{{".
func lexText(l *lexer) stateFn {
	if x := strings.Index(l.input[l.pos:], l.leftDelim); x >= 0 {
		if x > 0 {
			l.pos += Pos(x)
			// Do we trim any trailing space?
			trimLength := Pos(0)
			delimEnd := l.pos + Pos(len(l.leftDelim))
			if hasLeftTrimMarker(l.input[delimEnd:]) {
				trimLength = rightTrimLength(l.input[l.start:l.pos])
			}
			l.pos -= trimLength
			l.line += strings.Count(l.input[l.start:l.pos], "\n")
			i := l.thisItem(itemText)
			l.pos += trimLength
			l.ignore()
			if len(i.val) > 0 {
				return l.emitItem(i)
			}
		}
		return lexLeftDelim
	}
	l.pos = Pos(len(l.input))
	// Correctly reached EOF.
	if l.pos > l.start {
		l.line += strings.Count(l.input[l.start:l.pos], "\n")
		return l.emit(itemText)
	}
	return l.emit(itemEOF)
}

// rightTrimLength returns the length of the spaces at the end of the string.
func rightTrimLength(s string) Pos {
	return Pos(len(s) - len(strings.TrimRight(s, spaceChars)))
}

// atRightDelim reports whether the lexer is at a right delimiter, possibly preceded by a trim marker.
func (l *lexer) atRightDelim() (delim, trimSpaces bool) {
	if hasRightTrimMarker(l.input[l.pos:]) && strings.HasPrefix(l.input[l.pos+trimMarkerLen:], l.rightDelim) { // With trim marker.
		return true, true
	}
	if strings.HasPrefix(l.input[l.pos:], l.rightDelim) { // Without trim marker.
		return true, false
	}
	return false, false
}

// leftTrimLength returns the length of the spaces at the beginning of the string.
func leftTrimLength(s string) Pos {
	return Pos(len(s) - len(strings.TrimLeft(s, spaceChars)))
}

// lexLeftDelim scans the left delimiter, which is known to be present, possibly with a trim marker.
// (The text to be trimmed has already been emitted.)
func lexLeftDelim(l *lexer) stateFn {
	l.pos += Pos(len(l.leftDelim))
	trimSpace := hasLeftTrimMarker(l.input[l.pos:])
	afterMarker := Pos(0)
	if trimSpace {
		afterMarker = trimMarkerLen
	}
	if strings.HasPrefix(l.input[l.pos+afterMarker:], leftComment) {
		l.pos += afterMarker
		l.ignore()
		return lexComment
	}
	i := l.thisItem(itemLeftDelim)
	l.insideAction = true
	l.pos += afterMarker
	l.ignore()
	l.parenDepth = 0
	return l.emitItem(i)
}

// lexComment scans a comment. The left comment marker is known to be present.
func lexComment(l *lexer) stateFn {
	l.pos += Pos(len(leftComment))
	x := strings.Index(l.input[l.pos:], rightComment)
	if x < 0 {
		return l.errorf("unclosed comment")
	}
	l.pos += Pos(x + len(rightComment))
	delim, trimSpace := l.atRightDelim()
	if !delim {
		return l.errorf("comment ends before closing delimiter")
	}
	l.line += strings.Count(l.input[l.start:l.pos], "\n")
	i := l.thisItem(itemComment)
	if trimSpace {
		l.pos += trimMarkerLen
	}
	l.pos += Pos(len(l.rightDelim))
	if trimSpace {
		l.pos += leftTrimLength(l.input[l.pos:])
	}
	l.ignore()
	if l.options.emitComment {
		return l.emitItem(i)
	}
	return lexText
}

// lexRightDelim scans the right delimiter, which is known to be present, possibly with a trim marker.
func lexRightDelim(l *lexer) stateFn {
	_, trimSpace := l.atRightDelim()
	if trimSpace {
		l.pos += trimMarkerLen
		l.ignore()
	}
	l.pos += Pos(len(l.rightDelim))
	i := l.thisItem(itemRightDelim)
	if trimSpace {
		l.pos += leftTrimLength(l.input[l.pos:])
		l.ignore()
	}
	l.insideAction = false
	return l.emitItem(i)
}

// lexInsideAction scans the elements inside action delimiters.
func lexInsideAction(l *lexer) stateFn {
	// Either number, quoted string, or identifier.
	// Spaces separate arguments; runs of spaces turn into itemSpace.
	// Pipe symbols separate and are emitted.
	delim, _ := l.atRightDelim()
	if delim {
		if l.parenDepth == 0 {
			return lexRightDelim
		}
		return l.errorf("unclosed left paren")
	}
	switch r := l.next(); {
	case r == eof:
		return l.errorf("unclosed action")
	case isSpace(r):
		l.backup() // Put space back in case we have " -}}".
		return lexSpace
	case r == '=':
		return l.emit(itemAssign)
	case r == ':':
		if l.next() != '=' {
			return l.errorf("expected :=")
		}
		return l.emit(itemDeclare)
	case r == '|':
		return l.emit(itemPipe)
	case r == '"':
		return lexQuote
	case r == '`':
		return lexRawQuote
	case r == '$':
		return lexVariable
	case r == '\'':
		return lexChar
	case r == '.':
		// special look-ahead for ".field" so we don't break l.backup().
		if l.pos < Pos(len(l.input)) {
			r := l.input[l.pos]
			if r < '0' || '9' < r {
				return lexField
			}
		}
		fallthrough // '.' can start a number.
	case r == '+' || r == '-' || ('0' <= r && r <= '9'):
		l.backup()
		return lexNumber
	case isAlphaNumeric(r):
		l.backup()
		return lexIdentifier
	case r == '(':
		l.parenDepth++
		return l.emit(itemLeftParen)
	case r == ')':
		l.parenDepth--
		if l.parenDepth < 0 {
			return l.errorf("unexpected right paren")
		}
		return l.emit(itemRightParen)
	case r <= unicode.MaxASCII && unicode.IsPrint(r):
		return l.emit(itemChar)
	default:
		return l.errorf("unrecognized character in action: %#U", r)
	}
}

// lexSpace scans a run of space characters.
// We have not consumed the first space, which is known to be present.
// Take care if there is a trim-marked right delimiter, which starts with a space.
func lexSpace(l *lexer) stateFn {
	var r rune
	var numSpaces int
	for {
		r = l.peek()
		if !isSpace(r) {
			break
		}
		l.next()
		numSpaces++
	}
	// Be careful about a trim-marked closing delimiter, which has a minus
	// after a space. We know there is a space, so check for the '-' that might follow.
	if hasRightTrimMarker(l.input[l.pos-1:]) && strings.HasPrefix(l.input[l.pos-1+trimMarkerLen:], l.rightDelim) {
		l.backup() // Before the space.
		if numSpaces == 1 {
			return lexRightDelim // On the delim, so go right to that.
		}
	}
	return l.emit(itemSpace)
}

// lexIdentifier scans an alphanumeric.
func lexIdentifier(l *lexer) stateFn {
	for {
		switch r := l.next(); {
		case isAlphaNumeric(r):
			// absorb.
		default:
			l.backup()
			word := l.input[l.start:l.pos]
			if !l.atTerminator() {
				return l.errorf("bad character %#U", r)
			}
			switch {
			case key[word] > itemKeyword:
				item := key[word]
				if item == itemBreak && !l.options.breakOK || item == itemContinue && !l.options.continueOK {
					return l.emit(itemIdentifier)
				}
				return l.emit(item)
			case word[0] == '.':
				return l.emit(itemField)
			case word == "true", word == "false":
				return l.emit(itemBool)
			default:
				return l.emit(itemIdentifier)
			}
		}
	}
}

// lexField scans a field: .Alphanumeric.
// The . has been scanned.
func lexField(l *lexer) stateFn {
	return lexFieldOrVariable(l, itemField)
}

// lexVariable scans a Variable: $Alphanumeric.
// The $ has been scanned.
func lexVariable(l *lexer) stateFn {
	if l.atTerminator() { // Nothing interesting follows -> "$".
		return l.emit(itemVariable)
	}
	return lexFieldOrVariable(l, itemVariable)
}

// lexFieldOrVariable scans a field or variable: [.$]Alphanumeric.
// The . or $ has been scanned.
func lexFieldOrVariable(l *lexer, typ itemType) stateFn {
	if l.atTerminator() { // Nothing interesting follows -> "." or "$".
		if typ == itemVariable {
			return l.emit(itemVariable)
		}
		return l.emit(itemDot)
	}
	var r rune
	for {
		r = l.next()
		if !isAlphaNumeric(r) {
			l.backup()
			break
		}
	}
	if !l.atTerminator() {
		return l.errorf("bad character %#U", r)
	}
	return l.emit(typ)
}

// atTerminator reports whether the input is at valid termination character to
// appear after an identifier. Breaks .X.Y into two pieces. Also catches cases
// like "$x+2" not being acceptable without a space, in case we decide one
// day to implement arithmetic.
func (l *lexer) atTerminator() bool {
	r := l.peek()
	if isSpace(r) {
		return true
	}
	switch r {
	case eof, '.', ',', '|', ':', ')', '(':
		return true
	}
	return strings.HasPrefix(l.input[l.pos:], l.rightDelim)
}

// lexChar scans a character constant. The initial quote is already
// scanned. Syntax checking is done by the parser.
func lexChar(l *lexer) stateFn {
Loop:
	for {
		switch l.next() {
		case '\\':
			if r := l.next(); r != eof && r != '\n' {
				break
			}
			fallthrough
		case eof, '\n':
			return l.errorf("unterminated character constant")
		case '\'':
			break Loop
		}
	}
	return l.emit(itemCharConstant)
}

// lexNumber scans a number: decimal, octal, hex, float, or imaginary. This
// isn't a perfect number scanner - for instance it accepts "." and "0x0.2"
// and "089" - but when it's wrong the input is invalid and the parser (via
// strconv) will notice.
func lexNumber(l *lexer) stateFn {
	if !l.scanNumber() {
		return l.errorf("bad number syntax: %q", l.input[l.start:l.pos])
	}
	if sign := l.peek(); sign == '+' || sign == '-' {
		// Complex: 1+2i. No spaces, must end in 'i'.
		if !l.scanNumber() || l.input[l.pos-1] != 'i' {
			return l.errorf("bad number syntax: %q", l.input[l.start:l.pos])
		}
		return l.emit(itemComplex)
	}
	return l.emit(itemNumber)
}

func (l *lexer) scanNumber() bool {
	// Optional leading sign.
	l.accept("+-")
	// Is it hex?
	digits := "0123456789_"
	if l.accept("0") {
		// Note: Leading 0 does not mean octal in floats.
		if l.accept("xX") {
			digits = "0123456789abcdefABCDEF_"
		} else if l.accept("oO") {
			digits = "01234567_"
		} else if l.accept("bB") {
			digits = "01_"
		}
	}
	l.acceptRun(digits)
	if l.accept(".") {
		l.acceptRun(digits)
	}
	if len(digits) == 10+1 && l.accept("eE") {
		l.accept("+-")
		l.acceptRun("0123456789_")
	}
	if len(digits) == 16+6+1 && l.accept("pP") {
		l.accept("+-")
		l.acceptRun("0123456789_")
	}
	// Is it imaginary?
	l.accept("i")
	// Next thing mustn't be alphanumeric.
	if isAlphaNumeric(l.peek()) {
		l.next()
		return false
	}
	return true
}

// lexQuote scans a quoted string.
func lexQuote(l *lexer) stateFn {
Loop:
	for {
		switch l.next() {
		case '\\':
			if r := l.next(); r != eof && r != '\n' {
				break
			}
			fallthrough
		case eof, '\n':
			return l.errorf("unterminated quoted string")
		case '"':
			break Loop
		}
	}
	return l.emit(itemString)
}

// lexRawQuote scans a raw quoted string.
func lexRawQuote(l *lexer) stateFn {
Loop:
	for {
		switch l.next() {
		case eof:
			return l.errorf("unterminated raw quoted string")
		case '`':
			break Loop
		}
	}
	return l.emit(itemRawString)
}

// isSpace reports whether r is a space character.
func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\r' || r == '\n'
}

// isAlphaNumeric reports whether r is an alphabetic, digit, or underscore.
func isAlphaNumeric(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func hasLeftTrimMarker(s string) bool {
	return len(s) >= 2 && s[0] == trimMarker && isSpace(rune(s[1]))
}

func hasRightTrimMarker(s string) bool {
	return len(s) >= 2 && isSpace(rune(s[0])) && s[1] == trimMarker
}

```

// === FILE: references/go/src/text/template/parse/node.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Parse nodes.

package parse

import (
	"fmt"
	"strconv"
	"strings"
)

var textFormat = "%s" // Changed to "%q" in tests for better error messages.

// A Node is an element in the parse tree. The interface is trivial.
// The interface contains an unexported method so that only
// types local to this package can satisfy it.
type Node interface {
	Type() NodeType
	String() string
	// Copy does a deep copy of the Node and all its components.
	// To avoid type assertions, some XxxNodes also have specialized
	// CopyXxx methods that return *XxxNode.
	Copy() Node
	Position() Pos // byte position of start of node in full original input string
	// tree returns the containing *Tree.
	// It is unexported so all implementations of Node are in this package.
	tree() *Tree
	// writeTo writes the String output to the builder.
	writeTo(*strings.Builder)
}

// NodeType identifies the type of a parse tree node.
type NodeType int

// Pos represents a byte position in the original input text from which
// this template was parsed.
type Pos int

func (p Pos) Position() Pos {
	return p
}

// Type returns itself and provides an easy default implementation
// for embedding in a Node. Embedded in all non-trivial Nodes.
func (t NodeType) Type() NodeType {
	return t
}

const (
	NodeText       NodeType = iota // Plain text.
	NodeAction                     // A non-control action such as a field evaluation.
	NodeBool                       // A boolean constant.
	NodeChain                      // A sequence of field accesses.
	NodeCommand                    // An element of a pipeline.
	NodeDot                        // The cursor, dot.
	nodeElse                       // An else action. Not added to tree.
	nodeEnd                        // An end action. Not added to tree.
	NodeField                      // A field or method name.
	NodeIdentifier                 // An identifier; always a function name.
	NodeIf                         // An if action.
	NodeList                       // A list of Nodes.
	NodeNil                        // An untyped nil constant.
	NodeNumber                     // A numerical constant.
	NodePipe                       // A pipeline of commands.
	NodeRange                      // A range action.
	NodeString                     // A string constant.
	NodeTemplate                   // A template invocation action.
	NodeVariable                   // A $ variable.
	NodeWith                       // A with action.
	NodeComment                    // A comment.
	NodeBreak                      // A break action.
	NodeContinue                   // A continue action.
)

// Nodes.

// ListNode holds a sequence of nodes.
type ListNode struct {
	NodeType
	Pos
	tr    *Tree
	Nodes []Node // The element nodes in lexical order.
}

func (t *Tree) newList(pos Pos) *ListNode {
	return &ListNode{tr: t, NodeType: NodeList, Pos: pos}
}

func (l *ListNode) append(n Node) {
	l.Nodes = append(l.Nodes, n)
}

func (l *ListNode) tree() *Tree {
	return l.tr
}

func (l *ListNode) String() string {
	var sb strings.Builder
	l.writeTo(&sb)
	return sb.String()
}

func (l *ListNode) writeTo(sb *strings.Builder) {
	for _, n := range l.Nodes {
		n.writeTo(sb)
	}
}

func (l *ListNode) CopyList() *ListNode {
	if l == nil {
		return l
	}
	n := l.tr.newList(l.Pos)
	for _, elem := range l.Nodes {
		n.append(elem.Copy())
	}
	return n
}

func (l *ListNode) Copy() Node {
	return l.CopyList()
}

// TextNode holds plain text.
type TextNode struct {
	NodeType
	Pos
	tr   *Tree
	Text []byte // The text; may span newlines.
}

func (t *Tree) newText(pos Pos, text string) *TextNode {
	return &TextNode{tr: t, NodeType: NodeText, Pos: pos, Text: []byte(text)}
}

func (t *TextNode) String() string {
	return fmt.Sprintf(textFormat, t.Text)
}

func (t *TextNode) writeTo(sb *strings.Builder) {
	sb.WriteString(t.String())
}

func (t *TextNode) tree() *Tree {
	return t.tr
}

func (t *TextNode) Copy() Node {
	return &TextNode{tr: t.tr, NodeType: NodeText, Pos: t.Pos, Text: append([]byte{}, t.Text...)}
}

// CommentNode holds a comment.
type CommentNode struct {
	NodeType
	Pos
	tr   *Tree
	Text string // Comment text.
}

func (t *Tree) newComment(pos Pos, text string) *CommentNode {
	return &CommentNode{tr: t, NodeType: NodeComment, Pos: pos, Text: text}
}

func (c *CommentNode) String() string {
	var sb strings.Builder
	c.writeTo(&sb)
	return sb.String()
}

func (c *CommentNode) writeTo(sb *strings.Builder) {
	sb.WriteString(c.tr.leftDelim)
	sb.WriteString(c.Text)
	sb.WriteString(c.tr.rightDelim)
}

func (c *CommentNode) tree() *Tree {
	return c.tr
}

func (c *CommentNode) Copy() Node {
	return &CommentNode{tr: c.tr, NodeType: NodeComment, Pos: c.Pos, Text: c.Text}
}

// PipeNode holds a pipeline with optional declaration
type PipeNode struct {
	NodeType
	Pos
	tr       *Tree
	Line     int             // The line number in the input. Deprecated: Kept for compatibility.
	IsAssign bool            // The variables are being assigned, not declared.
	Decl     []*VariableNode // Variables in lexical order.
	Cmds     []*CommandNode  // The commands in lexical order.
}

func (t *Tree) newPipeline(pos Pos, line int, vars []*VariableNode) *PipeNode {
	return &PipeNode{tr: t, NodeType: NodePipe, Pos: pos, Line: line, Decl: vars}
}

func (p *PipeNode) append(command *CommandNode) {
	p.Cmds = append(p.Cmds, command)
}

func (p *PipeNode) String() string {
	var sb strings.Builder
	p.writeTo(&sb)
	return sb.String()
}

func (p *PipeNode) writeTo(sb *strings.Builder) {
	if len(p.Decl) > 0 {
		for i, v := range p.Decl {
			if i > 0 {
				sb.WriteString(", ")
			}
			v.writeTo(sb)
		}
		if p.IsAssign {
			sb.WriteString(" = ")
		} else {
			sb.WriteString(" := ")
		}
	}
	for i, c := range p.Cmds {
		if i > 0 {
			sb.WriteString(" | ")
		}
		c.writeTo(sb)
	}
}

func (p *PipeNode) tree() *Tree {
	return p.tr
}

func (p *PipeNode) CopyPipe() *PipeNode {
	if p == nil {
		return p
	}
	vars := make([]*VariableNode, len(p.Decl))
	for i, d := range p.Decl {
		vars[i] = d.Copy().(*VariableNode)
	}
	n := p.tr.newPipeline(p.Pos, p.Line, vars)
	n.IsAssign = p.IsAssign
	for _, c := range p.Cmds {
		n.append(c.Copy().(*CommandNode))
	}
	return n
}

func (p *PipeNode) Copy() Node {
	return p.CopyPipe()
}

// ActionNode holds an action (something bounded by delimiters).
// Control actions have their own nodes; ActionNode represents simple
// ones such as field evaluations and parenthesized pipelines.
type ActionNode struct {
	NodeType
	Pos
	tr   *Tree
	Line int       // The line number in the input. Deprecated: Kept for compatibility.
	Pipe *PipeNode // The pipeline in the action.
}

func (t *Tree) newAction(pos Pos, line int, pipe *PipeNode) *ActionNode {
	return &ActionNode{tr: t, NodeType: NodeAction, Pos: pos, Line: line, Pipe: pipe}
}

func (a *ActionNode) String() string {
	var sb strings.Builder
	a.writeTo(&sb)
	return sb.String()
}

func (a *ActionNode) writeTo(sb *strings.Builder) {
	sb.WriteString(a.tr.leftDelim)
	a.Pipe.writeTo(sb)
	sb.WriteString(a.tr.rightDelim)
}

func (a *ActionNode) tree() *Tree {
	return a.tr
}

func (a *ActionNode) Copy() Node {
	return a.tr.newAction(a.Pos, a.Line, a.Pipe.CopyPipe())
}

// CommandNode holds a command (a pipeline inside an evaluating action).
type CommandNode struct {
	NodeType
	Pos
	tr   *Tree
	Args []Node // Arguments in lexical order: Identifier, field, or constant.
}

func (t *Tree) newCommand(pos Pos) *CommandNode {
	return &CommandNode{tr: t, NodeType: NodeCommand, Pos: pos}
}

func (c *CommandNode) append(arg Node) {
	c.Args = append(c.Args, arg)
}

func (c *CommandNode) String() string {
	var sb strings.Builder
	c.writeTo(&sb)
	return sb.String()
}

func (c *CommandNode) writeTo(sb *strings.Builder) {
	for i, arg := range c.Args {
		if i > 0 {
			sb.WriteByte(' ')
		}
		if arg, ok := arg.(*PipeNode); ok {
			sb.WriteByte('(')
			arg.writeTo(sb)
			sb.WriteByte(')')
			continue
		}
		arg.writeTo(sb)
	}
}

func (c *CommandNode) tree() *Tree {
	return c.tr
}

func (c *CommandNode) Copy() Node {
	if c == nil {
		return c
	}
	n := c.tr.newCommand(c.Pos)
	for _, c := range c.Args {
		n.append(c.Copy())
	}
	return n
}

// IdentifierNode holds an identifier.
type IdentifierNode struct {
	NodeType
	Pos
	tr    *Tree
	Ident string // The identifier's name.
}

// NewIdentifier returns a new [IdentifierNode] with the given identifier name.
func NewIdentifier(ident string) *IdentifierNode {
	return &IdentifierNode{NodeType: NodeIdentifier, Ident: ident}
}

// SetPos sets the position. [NewIdentifier] is a public method so we can't modify its signature.
// Chained for convenience.
// TODO: fix one day?
func (i *IdentifierNode) SetPos(pos Pos) *IdentifierNode {
	i.Pos = pos
	return i
}

// SetTree sets the parent tree for the node. [NewIdentifier] is a public method so we can't modify its signature.
// Chained for convenience.
// TODO: fix one day?
func (i *IdentifierNode) SetTree(t *Tree) *IdentifierNode {
	i.tr = t
	return i
}

func (i *IdentifierNode) String() string {
	return i.Ident
}

func (i *IdentifierNode) writeTo(sb *strings.Builder) {
	sb.WriteString(i.String())
}

func (i *IdentifierNode) tree() *Tree {
	return i.tr
}

func (i *IdentifierNode) Copy() Node {
	return NewIdentifier(i.Ident).SetTree(i.tr).SetPos(i.Pos)
}

// VariableNode holds a list of variable names, possibly with chained field
// accesses. The dollar sign is part of the (first) name.
type VariableNode struct {
	NodeType
	Pos
	tr    *Tree
	Ident []string // Variable name and fields in lexical order.
}

func (t *Tree) newVariable(pos Pos, ident string) *VariableNode {
	return &VariableNode{tr: t, NodeType: NodeVariable, Pos: pos, Ident: strings.Split(ident, ".")}
}

func (v *VariableNode) String() string {
	var sb strings.Builder
	v.writeTo(&sb)
	return sb.String()
}

func (v *VariableNode) writeTo(sb *strings.Builder) {
	for i, id := range v.Ident {
		if i > 0 {
			sb.WriteByte('.')
		}
		sb.WriteString(id)
	}
}

func (v *VariableNode) tree() *Tree {
	return v.tr
}

func (v *VariableNode) Copy() Node {
	return &VariableNode{tr: v.tr, NodeType: NodeVariable, Pos: v.Pos, Ident: append([]string{}, v.Ident...)}
}

// DotNode holds the special identifier '.'.
type DotNode struct {
	NodeType
	Pos
	tr *Tree
}

func (t *Tree) newDot(pos Pos) *DotNode {
	return &DotNode{tr: t, NodeType: NodeDot, Pos: pos}
}

func (d *DotNode) Type() NodeType {
	// Override method on embedded NodeType for API compatibility.
	// TODO: Not really a problem; could change API without effect but
	// api tool complains.
	return NodeDot
}

func (d *DotNode) String() string {
	return "."
}

func (d *DotNode) writeTo(sb *strings.Builder) {
	sb.WriteString(d.String())
}

func (d *DotNode) tree() *Tree {
	return d.tr
}

func (d *DotNode) Copy() Node {
	return d.tr.newDot(d.Pos)
}

// NilNode holds the special identifier 'nil' representing an untyped nil constant.
type NilNode struct {
	NodeType
	Pos
	tr *Tree
}

func (t *Tree) newNil(pos Pos) *NilNode {
	return &NilNode{tr: t, NodeType: NodeNil, Pos: pos}
}

func (n *NilNode) Type() NodeType {
	// Override method on embedded NodeType for API compatibility.
	// TODO: Not really a problem; could change API without effect but
	// api tool complains.
	return NodeNil
}

func (n *NilNode) String() string {
	return "nil"
}

func (n *NilNode) writeTo(sb *strings.Builder) {
	sb.WriteString(n.String())
}

func (n *NilNode) tree() *Tree {
	return n.tr
}

func (n *NilNode) Copy() Node {
	return n.tr.newNil(n.Pos)
}

// FieldNode holds a field (identifier starting with '.').
// The names may be chained ('.x.y').
// The period is dropped from each ident.
type FieldNode struct {
	NodeType
	Pos
	tr    *Tree
	Ident []string // The identifiers in lexical order.
}

func (t *Tree) newField(pos Pos, ident string) *FieldNode {
	return &FieldNode{tr: t, NodeType: NodeField, Pos: pos, Ident: strings.Split(ident[1:], ".")} // [1:] to drop leading period
}

func (f *FieldNode) String() string {
	var sb strings.Builder
	f.writeTo(&sb)
	return sb.String()
}

func (f *FieldNode) writeTo(sb *strings.Builder) {
	for _, id := range f.Ident {
		sb.WriteByte('.')
		sb.WriteString(id)
	}
}

func (f *FieldNode) tree() *Tree {
	return f.tr
}

func (f *FieldNode) Copy() Node {
	return &FieldNode{tr: f.tr, NodeType: NodeField, Pos: f.Pos, Ident: append([]string{}, f.Ident...)}
}

// ChainNode holds a term followed by a chain of field accesses (identifier starting with '.').
// The names may be chained ('.x.y').
// The periods are dropped from each ident.
type ChainNode struct {
	NodeType
	Pos
	tr    *Tree
	Node  Node
	Field []string // The identifiers in lexical order.
}

func (t *Tree) newChain(pos Pos, node Node) *ChainNode {
	return &ChainNode{tr: t, NodeType: NodeChain, Pos: pos, Node: node}
}

// Add adds the named field (which should start with a period) to the end of the chain.
func (c *ChainNode) Add(field string) {
	if len(field) == 0 || field[0] != '.' {
		panic("no dot in field")
	}
	field = field[1:] // Remove leading dot.
	if field == "" {
		panic("empty field")
	}
	c.Field = append(c.Field, field)
}

func (c *ChainNode) String() string {
	var sb strings.Builder
	c.writeTo(&sb)
	return sb.String()
}

func (c *ChainNode) writeTo(sb *strings.Builder) {
	if _, ok := c.Node.(*PipeNode); ok {
		sb.WriteByte('(')
		c.Node.writeTo(sb)
		sb.WriteByte(')')
	} else {
		c.Node.writeTo(sb)
	}
	for _, field := range c.Field {
		sb.WriteByte('.')
		sb.WriteString(field)
	}
}

func (c *ChainNode) tree() *Tree {
	return c.tr
}

func (c *ChainNode) Copy() Node {
	return &ChainNode{tr: c.tr, NodeType: NodeChain, Pos: c.Pos, Node: c.Node, Field: append([]string{}, c.Field...)}
}

// BoolNode holds a boolean constant.
type BoolNode struct {
	NodeType
	Pos
	tr   *Tree
	True bool // The value of the boolean constant.
}

func (t *Tree) newBool(pos Pos, true bool) *BoolNode {
	return &BoolNode{tr: t, NodeType: NodeBool, Pos: pos, True: true}
}

func (b *BoolNode) String() string {
	if b.True {
		return "true"
	}
	return "false"
}

func (b *BoolNode) writeTo(sb *strings.Builder) {
	sb.WriteString(b.String())
}

func (b *BoolNode) tree() *Tree {
	return b.tr
}

func (b *BoolNode) Copy() Node {
	return b.tr.newBool(b.Pos, b.True)
}

// NumberNode holds a number: signed or unsigned integer, float, or complex.
// The value is parsed and stored under all the types that can represent the value.
// This simulates in a small amount of code the behavior of Go's ideal constants.
type NumberNode struct {
	NodeType
	Pos
	tr         *Tree
	IsInt      bool       // Number has an integral value.
	IsUint     bool       // Number has an unsigned integral value.
	IsFloat    bool       // Number has a floating-point value.
	IsComplex  bool       // Number is complex.
	Int64      int64      // The signed integer value.
	Uint64     uint64     // The unsigned integer value.
	Float64    float64    // The floating-point value.
	Complex128 complex128 // The complex value.
	Text       string     // The original textual representation from the input.
}

func (t *Tree) newNumber(pos Pos, text string, typ itemType) (*NumberNode, error) {
	n := &NumberNode{tr: t, NodeType: NodeNumber, Pos: pos, Text: text}
	switch typ {
	case itemCharConstant:
		rune, _, tail, err := strconv.UnquoteChar(text[1:], text[0])
		if err != nil {
			return nil, err
		}
		if tail != "'" {
			return nil, fmt.Errorf("malformed character constant: %s", text)
		}
		n.Int64 = int64(rune)
		n.IsInt = true
		n.Uint64 = uint64(rune)
		n.IsUint = true
		n.Float64 = float64(rune) // odd but those are the rules.
		n.IsFloat = true
		return n, nil
	case itemComplex:
		// fmt.Sscan can parse the pair, so let it do the work.
		if _, err := fmt.Sscan(text, &n.Complex128); err != nil {
			return nil, err
		}
		n.IsComplex = true
		n.simplifyComplex()
		return n, nil
	}
	// Imaginary constants can only be complex unless they are zero.
	if len(text) > 0 && text[len(text)-1] == 'i' {
		f, err := strconv.ParseFloat(text[:len(text)-1], 64)
		if err == nil {
			n.IsComplex = true
			n.Complex128 = complex(0, f)
			n.simplifyComplex()
			return n, nil
		}
	}
	// Do integer test first so we get 0x123 etc.
	u, err := strconv.ParseUint(text, 0, 64) // will fail for -0; fixed below.
	if err == nil {
		n.IsUint = true
		n.Uint64 = u
	}
	i, err := strconv.ParseInt(text, 0, 64)
	if err == nil {
		n.IsInt = true
		n.Int64 = i
		if i == 0 {
			n.IsUint = true // in case of -0.
			n.Uint64 = u
		}
	}
	// If an integer extraction succeeded, promote the float.
	if n.IsInt {
		n.IsFloat = true
		n.Float64 = float64(n.Int64)
	} else if n.IsUint {
		n.IsFloat = true
		n.Float64 = float64(n.Uint64)
	} else {
		f, err := strconv.ParseFloat(text, 64)
		if err == nil {
			// If we parsed it as a float but it looks like an integer,
			// it's a huge number too large to fit in an int. Reject it.
			if !strings.ContainsAny(text, ".eEpP") {
				return nil, fmt.Errorf("integer overflow: %q", text)
			}
			n.IsFloat = true
			n.Float64 = f
			// If a floating-point extraction succeeded, extract the int if needed.
			if !n.IsInt && float64(int64(f)) == f {
				n.IsInt = true
				n.Int64 = int64(f)
			}
			if !n.IsUint && float64(uint64(f)) == f {
				n.IsUint = true
				n.Uint64 = uint64(f)
			}
		}
	}
	if !n.IsInt && !n.IsUint && !n.IsFloat {
		return nil, fmt.Errorf("illegal number syntax: %q", text)
	}
	return n, nil
}

// simplifyComplex pulls out any other types that are represented by the complex number.
// These all require that the imaginary part be zero.
func (n *NumberNode) simplifyComplex() {
	n.IsFloat = imag(n.Complex128) == 0
	if n.IsFloat {
		n.Float64 = real(n.Complex128)
		n.IsInt = float64(int64(n.Float64)) == n.Float64
		if n.IsInt {
			n.Int64 = int64(n.Float64)
		}
		n.IsUint = float64(uint64(n.Float64)) == n.Float64
		if n.IsUint {
			n.Uint64 = uint64(n.Float64)
		}
	}
}

func (n *NumberNode) String() string {
	return n.Text
}

func (n *NumberNode) writeTo(sb *strings.Builder) {
	sb.WriteString(n.String())
}

func (n *NumberNode) tree() *Tree {
	return n.tr
}

func (n *NumberNode) Copy() Node {
	nn := new(NumberNode)
	*nn = *n // Easy, fast, correct.
	return nn
}

// StringNode holds a string constant. The value has been "unquoted".
type StringNode struct {
	NodeType
	Pos
	tr     *Tree
	Quoted string // The original text of the string, with quotes.
	Text   string // The string, after quote processing.
}

func (t *Tree) newString(pos Pos, orig, text string) *StringNode {
	return &StringNode{tr: t, NodeType: NodeString, Pos: pos, Quoted: orig, Text: text}
}

func (s *StringNode) String() string {
	return s.Quoted
}

func (s *StringNode) writeTo(sb *strings.Builder) {
	sb.WriteString(s.String())
}

func (s *StringNode) tree() *Tree {
	return s.tr
}

func (s *StringNode) Copy() Node {
	return s.tr.newString(s.Pos, s.Quoted, s.Text)
}

// endNode represents an {{end}} action.
// It does not appear in the final parse tree.
type endNode struct {
	NodeType
	Pos
	tr *Tree
}

func (t *Tree) newEnd(pos Pos) *endNode {
	return &endNode{tr: t, NodeType: nodeEnd, Pos: pos}
}

func (e *endNode) String() string {
	return e.tr.leftDelim + "end" + e.tr.rightDelim
}

func (e *endNode) writeTo(sb *strings.Builder) {
	sb.WriteString(e.String())
}

func (e *endNode) tree() *Tree {
	return e.tr
}

func (e *endNode) Copy() Node {
	return e.tr.newEnd(e.Pos)
}

// elseNode represents an {{else}} action. Does not appear in the final tree.
type elseNode struct {
	NodeType
	Pos
	tr   *Tree
	Line int // The line number in the input. Deprecated: Kept for compatibility.
}

func (t *Tree) newElse(pos Pos, line int) *elseNode {
	return &elseNode{tr: t, NodeType: nodeElse, Pos: pos, Line: line}
}

func (e *elseNode) Type() NodeType {
	return nodeElse
}

func (e *elseNode) String() string {
	return e.tr.leftDelim + "else" + e.tr.rightDelim
}

func (e *elseNode) writeTo(sb *strings.Builder) {
	sb.WriteString(e.String())
}

func (e *elseNode) tree() *Tree {
	return e.tr
}

func (e *elseNode) Copy() Node {
	return e.tr.newElse(e.Pos, e.Line)
}

// BranchNode is the common representation of if, range, and with.
type BranchNode struct {
	NodeType
	Pos
	tr       *Tree
	Line     int       // The line number in the input. Deprecated: Kept for compatibility.
	Pipe     *PipeNode // The pipeline to be evaluated.
	List     *ListNode // What to execute if the value is non-empty.
	ElseList *ListNode // What to execute if the value is empty (nil if absent).
}

func (b *BranchNode) String() string {
	var sb strings.Builder
	b.writeTo(&sb)
	return sb.String()
}

func (b *BranchNode) writeTo(sb *strings.Builder) {
	name := ""
	switch b.NodeType {
	case NodeIf:
		name = "if"
	case NodeRange:
		name = "range"
	case NodeWith:
		name = "with"
	default:
		panic("unknown branch type")
	}
	sb.WriteString(b.tr.leftDelim)
	sb.WriteString(name)
	sb.WriteByte(' ')
	b.Pipe.writeTo(sb)
	sb.WriteString(b.tr.rightDelim)
	b.List.writeTo(sb)
	if b.ElseList != nil {
		sb.WriteString(b.tr.leftDelim)
		sb.WriteString("else")
		sb.WriteString(b.tr.rightDelim)
		b.ElseList.writeTo(sb)
	}
	sb.WriteString(b.tr.leftDelim)
	sb.WriteString("end")
	sb.WriteString(b.tr.rightDelim)
}

func (b *BranchNode) tree() *Tree {
	return b.tr
}

func (b *BranchNode) Copy() Node {
	switch b.NodeType {
	case NodeIf:
		return b.tr.newIf(b.Pos, b.Line, b.Pipe, b.List, b.ElseList)
	case NodeRange:
		return b.tr.newRange(b.Pos, b.Line, b.Pipe, b.List, b.ElseList)
	case NodeWith:
		return b.tr.newWith(b.Pos, b.Line, b.Pipe, b.List, b.ElseList)
	default:
		panic("unknown branch type")
	}
}

// IfNode represents an {{if}} action and its commands.
type IfNode struct {
	BranchNode
}

func (t *Tree) newIf(pos Pos, line int, pipe *PipeNode, list, elseList *ListNode) *IfNode {
	return &IfNode{BranchNode{tr: t, NodeType: NodeIf, Pos: pos, Line: line, Pipe: pipe, List: list, ElseList: elseList}}
}

func (i *IfNode) Copy() Node {
	return i.tr.newIf(i.Pos, i.Line, i.Pipe.CopyPipe(), i.List.CopyList(), i.ElseList.CopyList())
}

// BreakNode represents a {{break}} action.
type BreakNode struct {
	tr *Tree
	NodeType
	Pos
	Line int
}

func (t *Tree) newBreak(pos Pos, line int) *BreakNode {
	return &BreakNode{tr: t, NodeType: NodeBreak, Pos: pos, Line: line}
}

func (b *BreakNode) Copy() Node                  { return b.tr.newBreak(b.Pos, b.Line) }
func (b *BreakNode) String() string              { return b.tr.leftDelim + "break" + b.tr.rightDelim }
func (b *BreakNode) tree() *Tree                 { return b.tr }
func (b *BreakNode) writeTo(sb *strings.Builder) { sb.WriteString(b.String()) }

// ContinueNode represents a {{continue}} action.
type ContinueNode struct {
	tr *Tree
	NodeType
	Pos
	Line int
}

func (t *Tree) newContinue(pos Pos, line int) *ContinueNode {
	return &ContinueNode{tr: t, NodeType: NodeContinue, Pos: pos, Line: line}
}

func (c *ContinueNode) Copy() Node                  { return c.tr.newContinue(c.Pos, c.Line) }
func (c *ContinueNode) String() string              { return c.tr.leftDelim + "continue" + c.tr.rightDelim }
func (c *ContinueNode) tree() *Tree                 { return c.tr }
func (c *ContinueNode) writeTo(sb *strings.Builder) { sb.WriteString(c.String()) }

// RangeNode represents a {{range}} action and its commands.
type RangeNode struct {
	BranchNode
}

func (t *Tree) newRange(pos Pos, line int, pipe *PipeNode, list, elseList *ListNode) *RangeNode {
	return &RangeNode{BranchNode{tr: t, NodeType: NodeRange, Pos: pos, Line: line, Pipe: pipe, List: list, ElseList: elseList}}
}

func (r *RangeNode) Copy() Node {
	return r.tr.newRange(r.Pos, r.Line, r.Pipe.CopyPipe(), r.List.CopyList(), r.ElseList.CopyList())
}

// WithNode represents a {{with}} action and its commands.
type WithNode struct {
	BranchNode
}

func (t *Tree) newWith(pos Pos, line int, pipe *PipeNode, list, elseList *ListNode) *WithNode {
	return &WithNode{BranchNode{tr: t, NodeType: NodeWith, Pos: pos, Line: line, Pipe: pipe, List: list, ElseList: elseList}}
}

func (w *WithNode) Copy() Node {
	return w.tr.newWith(w.Pos, w.Line, w.Pipe.CopyPipe(), w.List.CopyList(), w.ElseList.CopyList())
}

// TemplateNode represents a {{template}} action.
type TemplateNode struct {
	NodeType
	Pos
	tr   *Tree
	Line int       // The line number in the input. Deprecated: Kept for compatibility.
	Name string    // The name of the template (unquoted).
	Pipe *PipeNode // The command to evaluate as dot for the template.
}

func (t *Tree) newTemplate(pos Pos, line int, name string, pipe *PipeNode) *TemplateNode {
	return &TemplateNode{tr: t, NodeType: NodeTemplate, Pos: pos, Line: line, Name: name, Pipe: pipe}
}

func (t *TemplateNode) String() string {
	var sb strings.Builder
	t.writeTo(&sb)
	return sb.String()
}

func (t *TemplateNode) writeTo(sb *strings.Builder) {
	sb.WriteString(t.tr.leftDelim)
	sb.WriteString("template ")
	sb.WriteString(strconv.Quote(t.Name))
	if t.Pipe != nil {
		sb.WriteByte(' ')
		t.Pipe.writeTo(sb)
	}
	sb.WriteString(t.tr.rightDelim)
}

func (t *TemplateNode) tree() *Tree {
	return t.tr
}

func (t *TemplateNode) Copy() Node {
	return t.tr.newTemplate(t.Pos, t.Line, t.Name, t.Pipe.CopyPipe())
}

```

// === FILE: references/go/src/text/template/parse/parse.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package parse builds parse trees for templates as defined by text/template
// and html/template. Clients should use those packages to construct templates
// rather than this one, which provides shared internal data structures not
// intended for general use.
package parse

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

// Tree is the representation of a single parsed template.
type Tree struct {
	Name      string    // name of the template represented by the tree.
	ParseName string    // name of the top-level template during parsing, for error messages.
	Root      *ListNode // top-level root of the tree.
	Mode      Mode      // parsing mode.
	text      string    // text parsed to create the template (or its parent)
	// Parsing only; cleared after parse.
	funcs      []map[string]any
	lex        *lexer
	token      [3]item // three-token lookahead for parser.
	peekCount  int
	vars       []string // variables defined at the moment.
	treeSet    map[string]*Tree
	actionLine int // line of left delim starting action
	rangeDepth int
	stackDepth int // depth of nested parenthesized expressions

	leftDelim  string
	rightDelim string
}

// A Mode value is a set of flags (or 0). Modes control parser behavior.
type Mode uint

const (
	ParseComments Mode = 1 << iota // parse comments and add them to AST
	SkipFuncCheck                  // do not check that functions are defined
)

// maxStackDepth is the maximum depth permitted for nested
// parenthesized expressions.
var maxStackDepth = 10000

// init reduces maxStackDepth for WebAssembly due to its smaller stack size.
func init() {
	if runtime.GOARCH == "wasm" {
		maxStackDepth = 1000
	}
}

// Copy returns a copy of the [Tree]. Any parsing state is discarded.
func (t *Tree) Copy() *Tree {
	if t == nil {
		return nil
	}
	return &Tree{
		Name:       t.Name,
		ParseName:  t.ParseName,
		Root:       t.Root.CopyList(),
		text:       t.text,
		leftDelim:  t.leftDelim,
		rightDelim: t.rightDelim,
	}
}

// Parse returns a map from template name to [Tree], created by parsing the
// templates described in the argument string. The top-level template will be
// given the specified name. If an error is encountered, parsing stops and an
// empty map is returned with the error.
func Parse(name, text, leftDelim, rightDelim string, funcs ...map[string]any) (map[string]*Tree, error) {
	treeSet := make(map[string]*Tree)
	t := New(name)
	t.text = text
	_, err := t.Parse(text, leftDelim, rightDelim, treeSet, funcs...)
	return treeSet, err
}

// next returns the next token.
func (t *Tree) next() item {
	if t.peekCount > 0 {
		t.peekCount--
	} else {
		t.token[0] = t.lex.nextItem()
	}
	return t.token[t.peekCount]
}

// backup backs the input stream up one token.
func (t *Tree) backup() {
	t.peekCount++
}

// backup2 backs the input stream up two tokens.
// The zeroth token is already there.
func (t *Tree) backup2(t1 item) {
	t.token[1] = t1
	t.peekCount = 2
}

// backup3 backs the input stream up three tokens
// The zeroth token is already there.
func (t *Tree) backup3(t2, t1 item) { // Reverse order: we're pushing back.
	t.token[1] = t1
	t.token[2] = t2
	t.peekCount = 3
}

// peek returns but does not consume the next token.
func (t *Tree) peek() item {
	if t.peekCount > 0 {
		return t.token[t.peekCount-1]
	}
	t.peekCount = 1
	t.token[0] = t.lex.nextItem()
	return t.token[0]
}

// nextNonSpace returns the next non-space token.
func (t *Tree) nextNonSpace() (token item) {
	for {
		token = t.next()
		if token.typ != itemSpace {
			break
		}
	}
	return token
}

// peekNonSpace returns but does not consume the next non-space token.
func (t *Tree) peekNonSpace() item {
	token := t.nextNonSpace()
	t.backup()
	return token
}

// Parsing.

// New allocates a new parse tree with the given name.
func New(name string, funcs ...map[string]any) *Tree {
	return &Tree{
		Name:  name,
		funcs: funcs,
	}
}

// ErrorContext returns a textual representation of the location of the node in the input text.
// The receiver is only used when the node does not have a pointer to the tree inside,
// which can occur in old code.
func (t *Tree) ErrorContext(n Node) (location, context string) {
	pos := int(n.Position())
	tree := n.tree()
	if tree == nil {
		tree = t
	}
	text := tree.text[:pos]
	byteNum := strings.LastIndex(text, "\n")
	if byteNum == -1 {
		byteNum = pos // On first line.
	} else {
		byteNum++ // After the newline.
		byteNum = pos - byteNum
	}
	lineNum := 1 + strings.Count(text, "\n")
	context = n.String()
	return fmt.Sprintf("%s:%d:%d", tree.ParseName, lineNum, byteNum), context
}

// errorf formats the error and terminates processing.
func (t *Tree) errorf(format string, args ...any) {
	t.Root = nil
	format = fmt.Sprintf("template: %s:%d: %s", t.ParseName, t.token[0].line, format)
	panic(fmt.Errorf(format, args...))
}

// error terminates processing.
func (t *Tree) error(err error) {
	t.errorf("%s", err)
}

// expect consumes the next token and guarantees it has the required type.
func (t *Tree) expect(expected itemType, context string) item {
	token := t.nextNonSpace()
	if token.typ != expected {
		t.unexpected(token, context)
	}
	return token
}

// expectOneOf consumes the next token and guarantees it has one of the required types.
func (t *Tree) expectOneOf(expected1, expected2 itemType, context string) item {
	token := t.nextNonSpace()
	if token.typ != expected1 && token.typ != expected2 {
		t.unexpected(token, context)
	}
	return token
}

// unexpected complains about the token and terminates processing.
func (t *Tree) unexpected(token item, context string) {
	if token.typ == itemError {
		extra := ""
		if t.actionLine != 0 && t.actionLine != token.line {
			extra = fmt.Sprintf(" in action started at %s:%d", t.ParseName, t.actionLine)
			if strings.HasSuffix(token.val, " action") {
				extra = extra[len(" in action"):] // avoid "action in action"
			}
		}
		t.errorf("%s%s", token, extra)
	}
	t.errorf("unexpected %s in %s", token, context)
}

// recover is the handler that turns panics into returns from the top level of Parse.
func (t *Tree) recover(errp *error) {
	e := recover()
	if e != nil {
		if _, ok := e.(runtime.Error); ok {
			panic(e)
		}
		if t != nil {
			t.stopParse()
		}
		*errp = e.(error)
	}
}

// startParse initializes the parser, using the lexer.
func (t *Tree) startParse(funcs []map[string]any, lex *lexer, treeSet map[string]*Tree) {
	t.Root = nil
	t.lex = lex
	t.vars = []string{"$"}
	t.funcs = funcs
	t.treeSet = treeSet
	t.stackDepth = 0
	lex.options = lexOptions{
		emitComment: t.Mode&ParseComments != 0,
		breakOK:     !t.hasFunction("break"),
		continueOK:  !t.hasFunction("continue"),
	}
}

// stopParse terminates parsing.
func (t *Tree) stopParse() {
	t.lex = nil
	t.vars = nil
	t.funcs = nil
	t.treeSet = nil
}

// Parse parses the template definition string to construct a representation of
// the template for execution. If either action delimiter string is empty, the
// default ("{{" or "}}") is used. Embedded template definitions are added to
// the treeSet map.
func (t *Tree) Parse(text, leftDelim, rightDelim string, treeSet map[string]*Tree, funcs ...map[string]any) (tree *Tree, err error) {
	defer t.recover(&err)
	t.ParseName = t.Name
	t.leftDelim = leftDelim
	if t.leftDelim == "" {
		t.leftDelim = defaultLeftDelim
	}
	t.rightDelim = rightDelim
	if t.rightDelim == "" {
		t.rightDelim = defaultRightDelim
	}
	lexer := lex(t.Name, text, t.leftDelim, t.rightDelim)
	t.startParse(funcs, lexer, treeSet)
	t.text = text
	t.parse()
	t.add()
	t.stopParse()
	return t, nil
}

// add adds tree to t.treeSet.
func (t *Tree) add() {
	tree := t.treeSet[t.Name]
	if tree == nil || IsEmptyTree(tree.Root) {
		t.treeSet[t.Name] = t
		return
	}
	if !IsEmptyTree(t.Root) {
		t.errorf("template: multiple definition of template %q", t.Name)
	}
}

// IsEmptyTree reports whether this tree (node) is empty of everything but space or comments.
func IsEmptyTree(n Node) bool {
	switch n := n.(type) {
	case nil:
		return true
	case *ActionNode:
	case *CommentNode:
		return true
	case *IfNode:
	case *ListNode:
		for _, node := range n.Nodes {
			if !IsEmptyTree(node) {
				return false
			}
		}
		return true
	case *RangeNode:
	case *TemplateNode:
	case *TextNode:
		return len(bytes.TrimSpace(n.Text)) == 0
	case *WithNode:
	default:
		panic("unknown node: " + n.String())
	}
	return false
}

// parse is the top-level parser for a template, essentially the same
// as itemList except it also parses {{define}} actions.
// It runs to EOF.
func (t *Tree) parse() {
	t.Root = t.newList(t.peek().pos)
	for t.peek().typ != itemEOF {
		if t.peek().typ == itemLeftDelim {
			delim := t.next()
			if t.nextNonSpace().typ == itemDefine {
				newT := New("definition") // name will be updated once we know it.
				newT.text = t.text
				newT.Mode = t.Mode
				newT.leftDelim = t.leftDelim
				newT.rightDelim = t.rightDelim
				newT.ParseName = t.ParseName
				newT.startParse(t.funcs, t.lex, t.treeSet)
				newT.parseDefinition()
				continue
			}
			t.backup2(delim)
		}
		switch n := t.textOrAction(); n.Type() {
		case nodeEnd, nodeElse:
			t.errorf("unexpected %s", n)
		default:
			t.Root.append(n)
		}
	}
}

// parseDefinition parses a {{define}} ...  {{end}} template definition and
// installs the definition in t.treeSet. The "define" keyword has already
// been scanned.
func (t *Tree) parseDefinition() {
	const context = "define clause"
	name := t.expectOneOf(itemString, itemRawString, context)
	var err error
	t.Name, err = strconv.Unquote(name.val)
	if err != nil {
		t.error(err)
	}
	t.expect(itemRightDelim, context)
	var end Node
	t.Root, end = t.itemList()
	if end.Type() != nodeEnd {
		t.errorf("unexpected %s in %s", end, context)
	}
	t.add()
	t.stopParse()
}

// itemList:
//
//	textOrAction*
//
// Terminates at {{end}} or {{else}}, returned separately.
func (t *Tree) itemList() (list *ListNode, next Node) {
	list = t.newList(t.peekNonSpace().pos)
	for t.peekNonSpace().typ != itemEOF {
		n := t.textOrAction()
		switch n.Type() {
		case nodeEnd, nodeElse:
			return list, n
		}
		list.append(n)
	}
	t.errorf("unexpected EOF")
	return
}

// textOrAction:
//
//	text | comment | action
func (t *Tree) textOrAction() Node {
	switch token := t.nextNonSpace(); token.typ {
	case itemText:
		return t.newText(token.pos, token.val)
	case itemLeftDelim:
		t.actionLine = token.line
		defer t.clearActionLine()
		return t.action()
	case itemComment:
		return t.newComment(token.pos, token.val)
	default:
		t.unexpected(token, "input")
	}
	return nil
}

func (t *Tree) clearActionLine() {
	t.actionLine = 0
}

// Action:
//
//	control
//	command ("|" command)*
//
// Left delim is past. Now get actions.
// First word could be a keyword such as range.
func (t *Tree) action() (n Node) {
	switch token := t.nextNonSpace(); token.typ {
	case itemBlock:
		return t.blockControl()
	case itemBreak:
		return t.breakControl(token.pos, token.line)
	case itemContinue:
		return t.continueControl(token.pos, token.line)
	case itemElse:
		return t.elseControl()
	case itemEnd:
		return t.endControl()
	case itemIf:
		return t.ifControl()
	case itemRange:
		return t.rangeControl()
	case itemTemplate:
		return t.templateControl()
	case itemWith:
		return t.withControl()
	}
	t.backup()
	token := t.peek()
	// Do not pop variables; they persist until "end".
	return t.newAction(token.pos, token.line, t.pipeline("command", itemRightDelim))
}

// Break:
//
//	{{break}}
//
// Break keyword is past.
func (t *Tree) breakControl(pos Pos, line int) Node {
	if token := t.nextNonSpace(); token.typ != itemRightDelim {
		t.unexpected(token, "{{break}}")
	}
	if t.rangeDepth == 0 {
		t.errorf("{{break}} outside {{range}}")
	}
	return t.newBreak(pos, line)
}

// Continue:
//
//	{{continue}}
//
// Continue keyword is past.
func (t *Tree) continueControl(pos Pos, line int) Node {
	if token := t.nextNonSpace(); token.typ != itemRightDelim {
		t.unexpected(token, "{{continue}}")
	}
	if t.rangeDepth == 0 {
		t.errorf("{{continue}} outside {{range}}")
	}
	return t.newContinue(pos, line)
}

// Pipeline:
//
//	declarations? command ('|' command)*
func (t *Tree) pipeline(context string, end itemType) (pipe *PipeNode) {
	token := t.peekNonSpace()
	pipe = t.newPipeline(token.pos, token.line, nil)
	// Are there declarations or assignments?
decls:
	if v := t.peekNonSpace(); v.typ == itemVariable {
		t.next()
		// Since space is a token, we need 3-token look-ahead here in the worst case:
		// in "$x foo" we need to read "foo" (as opposed to ":=") to know that $x is an
		// argument variable rather than a declaration. So remember the token
		// adjacent to the variable so we can push it back if necessary.
		tokenAfterVariable := t.peek()
		next := t.peekNonSpace()
		switch {
		case next.typ == itemAssign, next.typ == itemDeclare:
			pipe.IsAssign = next.typ == itemAssign
			t.nextNonSpace()
			pipe.Decl = append(pipe.Decl, t.newVariable(v.pos, v.val))
			t.vars = append(t.vars, v.val)
		case next.typ == itemChar && next.val == ",":
			t.nextNonSpace()
			pipe.Decl = append(pipe.Decl, t.newVariable(v.pos, v.val))
			t.vars = append(t.vars, v.val)
			if context == "range" && len(pipe.Decl) < 2 {
				switch t.peekNonSpace().typ {
				case itemVariable, itemRightDelim, itemRightParen:
					// second initialized variable in a range pipeline
					goto decls
				default:
					t.errorf("range can only initialize variables")
				}
			}
			t.errorf("too many declarations in %s", context)
		case tokenAfterVariable.typ == itemSpace:
			t.backup3(v, tokenAfterVariable)
		default:
			t.backup2(v)
		}
	}
	for {
		switch token := t.nextNonSpace(); token.typ {
		case end:
			// At this point, the pipeline is complete
			t.checkPipeline(pipe, context)
			return
		case itemBool, itemCharConstant, itemComplex, itemDot, itemField, itemIdentifier,
			itemNumber, itemNil, itemRawString, itemString, itemVariable, itemLeftParen:
			t.backup()
			pipe.append(t.command())
		default:
			t.unexpected(token, context)
		}
	}
}

func (t *Tree) checkPipeline(pipe *PipeNode, context string) {
	// Reject empty pipelines
	if len(pipe.Cmds) == 0 {
		t.errorf("missing value for %s", context)
	}
	// Only the first command of a pipeline can start with a non executable operand
	for i, c := range pipe.Cmds[1:] {
		switch c.Args[0].Type() {
		case NodeBool, NodeDot, NodeNil, NodeNumber, NodeString:
			// With A|B|C, pipeline stage 2 is B
			t.errorf("non executable command in pipeline stage %d", i+2)
		}
	}
}

func (t *Tree) parseControl(context string) (pos Pos, line int, pipe *PipeNode, list, elseList *ListNode) {
	defer t.popVars(len(t.vars))
	pipe = t.pipeline(context, itemRightDelim)
	if context == "range" {
		t.rangeDepth++
	}
	var next Node
	list, next = t.itemList()
	if context == "range" {
		t.rangeDepth--
	}
	switch next.Type() {
	case nodeEnd: // done
	case nodeElse:
		// Special case for "else if" and "else with".
		// If the "else" is followed immediately by an "if" or "with",
		// the elseControl will have left the "if" or "with" token pending. Treat
		//	{{if a}}_{{else if b}}_{{end}}
		//  {{with a}}_{{else with b}}_{{end}}
		// as
		//	{{if a}}_{{else}}{{if b}}_{{end}}{{end}}
		//  {{with a}}_{{else}}{{with b}}_{{end}}{{end}}.
		// To do this, parse the "if" or "with" as usual and stop at it {{end}};
		// the subsequent{{end}} is assumed. This technique works even for long if-else-if chains.
		if context == "if" && t.peek().typ == itemIf {
			t.next() // Consume the "if" token.
			elseList = t.newList(next.Position())
			elseList.append(t.ifControl())
		} else if context == "with" && t.peek().typ == itemWith {
			t.next()
			elseList = t.newList(next.Position())
			elseList.append(t.withControl())
		} else {
			elseList, next = t.itemList()
			if next.Type() != nodeEnd {
				t.errorf("expected end; found %s", next)
			}
		}
	}
	return pipe.Position(), pipe.Line, pipe, list, elseList
}

// If:
//
//	{{if pipeline}} itemList {{end}}
//	{{if pipeline}} itemList {{else}} itemList {{end}}
//
// If keyword is past.
func (t *Tree) ifControl() Node {
	return t.newIf(t.parseControl("if"))
}

// Range:
//
//	{{range pipeline}} itemList {{end}}
//	{{range pipeline}} itemList {{else}} itemList {{end}}
//
// Range keyword is past.
func (t *Tree) rangeControl() Node {
	r := t.newRange(t.parseControl("range"))
	return r
}

// With:
//
//	{{with pipeline}} itemList {{end}}
//	{{with pipeline}} itemList {{else}} itemList {{end}}
//
// If keyword is past.
func (t *Tree) withControl() Node {
	return t.newWith(t.parseControl("with"))
}

// End:
//
//	{{end}}
//
// End keyword is past.
func (t *Tree) endControl() Node {
	return t.newEnd(t.expect(itemRightDelim, "end").pos)
}

// Else:
//
//	{{else}}
//
// Else keyword is past.
func (t *Tree) elseControl() Node {
	peek := t.peekNonSpace()
	// The "{{else if ... " and "{{else with ..." will be
	// treated as "{{else}}{{if ..." and "{{else}}{{with ...".
	// So return the else node here.
	if peek.typ == itemIf || peek.typ == itemWith {
		return t.newElse(peek.pos, peek.line)
	}
	token := t.expect(itemRightDelim, "else")
	return t.newElse(token.pos, token.line)
}

// Block:
//
//	{{block stringValue pipeline}}
//
// Block keyword is past.
// The name must be something that can evaluate to a string.
// The pipeline is mandatory.
func (t *Tree) blockControl() Node {
	const context = "block clause"

	token := t.nextNonSpace()
	name := t.parseTemplateName(token, context)
	pipe := t.pipeline(context, itemRightDelim)

	block := New(name) // name will be updated once we know it.
	block.text = t.text
	block.Mode = t.Mode
	block.leftDelim = t.leftDelim
	block.rightDelim = t.rightDelim
	block.ParseName = t.ParseName
	block.startParse(t.funcs, t.lex, t.treeSet)
	var end Node
	block.Root, end = block.itemList()
	if end.Type() != nodeEnd {
		t.errorf("unexpected %s in %s", end, context)
	}
	block.add()
	block.stopParse()

	return t.newTemplate(token.pos, token.line, name, pipe)
}

// Template:
//
//	{{template stringValue pipeline}}
//
// Template keyword is past. The name must be something that can evaluate
// to a string.
func (t *Tree) templateControl() Node {
	const context = "template clause"
	token := t.nextNonSpace()
	name := t.parseTemplateName(token, context)
	var pipe *PipeNode
	if t.nextNonSpace().typ != itemRightDelim {
		t.backup()
		// Do not pop variables; they persist until "end".
		pipe = t.pipeline(context, itemRightDelim)
	}
	return t.newTemplate(token.pos, token.line, name, pipe)
}

func (t *Tree) parseTemplateName(token item, context string) (name string) {
	switch token.typ {
	case itemString, itemRawString:
		s, err := strconv.Unquote(token.val)
		if err != nil {
			t.error(err)
		}
		name = s
	default:
		t.unexpected(token, context)
	}
	return
}

// command:
//
//	operand (space operand)*
//
// space-separated arguments up to a pipeline character or right delimiter.
// we consume the pipe character but leave the right delim to terminate the action.
func (t *Tree) command() *CommandNode {
	cmd := t.newCommand(t.peekNonSpace().pos)
	for {
		t.peekNonSpace() // skip leading spaces.
		operand := t.operand()
		if operand != nil {
			cmd.append(operand)
		}
		switch token := t.next(); token.typ {
		case itemSpace:
			continue
		case itemRightDelim, itemRightParen:
			t.backup()
		case itemPipe:
			// nothing here; break loop below
		default:
			t.unexpected(token, "operand")
		}
		break
	}
	if len(cmd.Args) == 0 {
		t.errorf("empty command")
	}
	return cmd
}

// operand:
//
//	term .Field*
//
// An operand is a space-separated component of a command,
// a term possibly followed by field accesses.
// A nil return means the next item is not an operand.
func (t *Tree) operand() Node {
	node := t.term()
	if node == nil {
		return nil
	}
	if t.peek().typ == itemField {
		chain := t.newChain(t.peek().pos, node)
		for t.peek().typ == itemField {
			chain.Add(t.next().val)
		}
		// Compatibility with original API: If the term is of type NodeField
		// or NodeVariable, just put more fields on the original.
		// Otherwise, keep the Chain node.
		// Obvious parsing errors involving literal values are detected here.
		// More complex error cases will have to be handled at execution time.
		switch node.Type() {
		case NodeField:
			node = t.newField(chain.Position(), chain.String())
		case NodeVariable:
			node = t.newVariable(chain.Position(), chain.String())
		case NodeBool, NodeString, NodeNumber, NodeNil, NodeDot:
			t.errorf("unexpected . after term %q", node.String())
		default:
			node = chain
		}
	}
	return node
}

// term:
//
//	literal (number, string, nil, boolean)
//	function (identifier)
//	.
//	.Field
//	$
//	'(' pipeline ')'
//
// A term is a simple "expression".
// A nil return means the next item is not a term.
func (t *Tree) term() Node {
	switch token := t.nextNonSpace(); token.typ {
	case itemIdentifier:
		checkFunc := t.Mode&SkipFuncCheck == 0
		if checkFunc && !t.hasFunction(token.val) {
			t.errorf("function %q not defined", token.val)
		}
		return NewIdentifier(token.val).SetTree(t).SetPos(token.pos)
	case itemDot:
		return t.newDot(token.pos)
	case itemNil:
		return t.newNil(token.pos)
	case itemVariable:
		return t.useVar(token.pos, token.val)
	case itemField:
		return t.newField(token.pos, token.val)
	case itemBool:
		return t.newBool(token.pos, token.val == "true")
	case itemCharConstant, itemComplex, itemNumber:
		number, err := t.newNumber(token.pos, token.val, token.typ)
		if err != nil {
			t.error(err)
		}
		return number
	case itemLeftParen:
		if t.stackDepth >= maxStackDepth {
			t.errorf("max expression depth exceeded")
		}
		t.stackDepth++
		defer func() { t.stackDepth-- }()
		return t.pipeline("parenthesized pipeline", itemRightParen)
	case itemString, itemRawString:
		s, err := strconv.Unquote(token.val)
		if err != nil {
			t.error(err)
		}
		return t.newString(token.pos, token.val, s)
	}
	t.backup()
	return nil
}

// hasFunction reports if a function name exists in the Tree's maps.
func (t *Tree) hasFunction(name string) bool {
	for _, funcMap := range t.funcs {
		if funcMap == nil {
			continue
		}
		if funcMap[name] != nil {
			return true
		}
	}
	return false
}

// popVars trims the variable list to the specified length
func (t *Tree) popVars(n int) {
	t.vars = t.vars[:n]
}

// useVar returns a node for a variable reference. It errors if the
// variable is not defined.
func (t *Tree) useVar(pos Pos, name string) Node {
	v := t.newVariable(pos, name)
	for _, varName := range t.vars {
		if varName == v.Ident[0] {
			return v
		}
	}
	t.errorf("undefined variable %q", v.Ident[0])
	return nil
}

```

// === FILE: references/go/src/text/template/template.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"maps"
	"reflect"
	"sync"
	"text/template/parse"
)

// common holds the information shared by related templates.
type common struct {
	tmpl   map[string]*Template // Map from name to defined templates.
	muTmpl sync.RWMutex         // protects tmpl
	option option
	// We use two maps, one for parsing and one for execution.
	// This separation makes the API cleaner since it doesn't
	// expose reflection to the client.
	muFuncs    sync.RWMutex // protects parseFuncs and execFuncs
	parseFuncs FuncMap
	execFuncs  map[string]reflect.Value
}

// Template is the representation of a parsed template. The *parse.Tree
// field is exported only for use by [html/template] and should be treated
// as unexported by all other clients.
type Template struct {
	name string
	*parse.Tree
	*common
	leftDelim  string
	rightDelim string
}

// New allocates a new, undefined template with the given name.
func New(name string) *Template {
	t := &Template{
		name: name,
	}
	t.init()
	return t
}

// Name returns the name of the template.
func (t *Template) Name() string {
	return t.name
}

// New allocates a new, undefined template associated with the given one and with the same
// delimiters. The association, which is transitive, allows one template to
// invoke another with a {{template}} action.
//
// Because associated templates share underlying data, template construction
// cannot be done safely in parallel. Once the templates are constructed, they
// can be executed in parallel.
func (t *Template) New(name string) *Template {
	t.init()
	nt := &Template{
		name:       name,
		common:     t.common,
		leftDelim:  t.leftDelim,
		rightDelim: t.rightDelim,
	}
	return nt
}

// init guarantees that t has a valid common structure.
func (t *Template) init() {
	if t.common == nil {
		c := new(common)
		c.tmpl = make(map[string]*Template)
		c.parseFuncs = make(FuncMap)
		c.execFuncs = make(map[string]reflect.Value)
		t.common = c
	}
}

// Clone returns a duplicate of the template, including all associated
// templates. The actual representation is not copied, but the name space of
// associated templates is, so further calls to [Template.Parse] in the copy will add
// templates to the copy but not to the original. Clone can be used to prepare
// common templates and use them with variant definitions for other templates
// by adding the variants after the clone is made.
func (t *Template) Clone() (*Template, error) {
	nt := t.copy(nil)
	nt.init()
	if t.common == nil {
		return nt, nil
	}
	nt.option = t.option
	t.muTmpl.RLock()
	defer t.muTmpl.RUnlock()
	for k, v := range t.tmpl {
		if k == t.name {
			nt.tmpl[t.name] = nt
			continue
		}
		// The associated templates share nt's common structure.
		tmpl := v.copy(nt.common)
		nt.tmpl[k] = tmpl
	}
	t.muFuncs.RLock()
	defer t.muFuncs.RUnlock()
	maps.Copy(nt.parseFuncs, t.parseFuncs)
	maps.Copy(nt.execFuncs, t.execFuncs)
	return nt, nil
}

// copy returns a shallow copy of t, with common set to the argument.
func (t *Template) copy(c *common) *Template {
	return &Template{
		name:       t.name,
		Tree:       t.Tree,
		common:     c,
		leftDelim:  t.leftDelim,
		rightDelim: t.rightDelim,
	}
}

// AddParseTree associates the argument parse tree with the template t, giving
// it the specified name. If the template has not been defined, this tree becomes
// its definition. If it has been defined and already has that name, the existing
// definition is replaced; otherwise a new template is created, defined, and returned.
func (t *Template) AddParseTree(name string, tree *parse.Tree) (*Template, error) {
	t.init()
	t.muTmpl.Lock()
	defer t.muTmpl.Unlock()
	nt := t
	if name != t.name {
		nt = t.New(name)
	}
	// Even if nt == t, we need to install it in the common.tmpl map.
	if t.associate(nt, tree) || nt.Tree == nil {
		nt.Tree = tree
	}
	return nt, nil
}

// Templates returns a slice of defined templates associated with t.
func (t *Template) Templates() []*Template {
	if t.common == nil {
		return nil
	}
	// Return a slice so we don't expose the map.
	t.muTmpl.RLock()
	defer t.muTmpl.RUnlock()
	m := make([]*Template, 0, len(t.tmpl))
	for _, v := range t.tmpl {
		m = append(m, v)
	}
	return m
}

// Delims sets the action delimiters to the specified strings, to be used in
// subsequent calls to [Template.Parse], [Template.ParseFiles], or [Template.ParseGlob]. Nested template
// definitions will inherit the settings. An empty delimiter stands for the
// corresponding default: {{ or }}.
// The return value is the template, so calls can be chained.
func (t *Template) Delims(left, right string) *Template {
	t.init()
	t.leftDelim = left
	t.rightDelim = right
	return t
}

// Funcs adds the elements of the argument map to the template's function map.
// It must be called before the template is parsed.
// It panics if a value in the map is not a function with appropriate return
// type or if the name cannot be used syntactically as a function in a template.
// It is legal to overwrite elements of the map. The return value is the template,
// so calls can be chained.
func (t *Template) Funcs(funcMap FuncMap) *Template {
	t.init()
	t.muFuncs.Lock()
	defer t.muFuncs.Unlock()
	addValueFuncs(t.execFuncs, funcMap)
	addFuncs(t.parseFuncs, funcMap)
	return t
}

// Lookup returns the template with the given name that is associated with t.
// It returns nil if there is no such template or the template has no definition.
func (t *Template) Lookup(name string) *Template {
	if t.common == nil {
		return nil
	}
	t.muTmpl.RLock()
	defer t.muTmpl.RUnlock()
	return t.tmpl[name]
}

// Parse parses text as a template body for t.
// Named template definitions ({{define ...}} or {{block ...}} statements) in text
// define additional templates associated with t and are removed from the
// definition of t itself.
//
// Templates can be redefined in successive calls to Parse.
// A template definition with a body containing only white space and comments
// is considered empty and will not replace an existing template's body.
// This allows using Parse to add new named template definitions without
// overwriting the main template body.
func (t *Template) Parse(text string) (*Template, error) {
	t.init()
	t.muFuncs.RLock()
	trees, err := parse.Parse(t.name, text, t.leftDelim, t.rightDelim, t.parseFuncs, builtins())
	t.muFuncs.RUnlock()
	if err != nil {
		return nil, err
	}
	// Add the newly parsed trees, including the one for t, into our common structure.
	for name, tree := range trees {
		if _, err := t.AddParseTree(name, tree); err != nil {
			return nil, err
		}
	}
	return t, nil
}

// associate installs the new template into the group of templates associated
// with t. The two are already known to share the common structure.
// The boolean return value reports whether to store this tree as t.Tree.
func (t *Template) associate(new *Template, tree *parse.Tree) bool {
	if new.common != t.common {
		panic("internal error: associate not common")
	}
	if old := t.tmpl[new.name]; old != nil && parse.IsEmptyTree(tree.Root) && old.Tree != nil {
		// If a template by that name exists,
		// don't replace it with an empty template.
		return false
	}
	t.tmpl[new.name] = new
	return true
}

```

