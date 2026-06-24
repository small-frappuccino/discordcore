# Domain Architecture: mime

## Layout Topology
```text
mime/
├── multipart
│   ├── formdata.go
│   ├── multipart.go
│   ├── readmimeheader.go
│   └── writer.go
├── quotedprintable
│   ├── reader.go
│   └── writer.go
├── encodedword.go
├── grammar.go
├── mediatype.go
├── type.go
├── type_dragonfly.go
├── type_freebsd.go
├── type_openbsd.go
├── type_plan9.go
├── type_unix.go
└── type_windows.go
```

## Source Stream Aggregation

// === FILE: references/go/src/mime/encodedword.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mime

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

// A WordEncoder is an RFC 2047 encoded-word encoder.
type WordEncoder byte

const (
	// BEncoding represents Base64 encoding scheme as defined by RFC 2045.
	BEncoding = WordEncoder('b')
	// QEncoding represents the Q-encoding scheme as defined by RFC 2047.
	QEncoding = WordEncoder('q')
)

var (
	errInvalidWord = errors.New("mime: invalid RFC 2047 encoded-word")
)

// Encode returns the encoded-word form of s. If s is ASCII without special
// characters, it is returned unchanged. The provided charset is the IANA
// charset name of s. It is case insensitive.
func (e WordEncoder) Encode(charset, s string) string {
	if !needsEncoding(s) {
		return s
	}
	return e.encodeWord(charset, s)
}

func needsEncoding(s string) bool {
	for _, b := range s {
		if (b < ' ' || b > '~') && b != '\t' {
			return true
		}
	}
	return false
}

// encodeWord encodes a string into an encoded-word.
func (e WordEncoder) encodeWord(charset, s string) string {
	var buf strings.Builder
	// Could use a hint like len(s)*3, but that's not enough for cases
	// with word splits and too much for simpler inputs.
	// 48 is close to maxEncodedWordLen/2, but adjusted to allocator size class.
	buf.Grow(48)

	e.openWord(&buf, charset)
	if e == BEncoding {
		e.bEncode(&buf, charset, s)
	} else {
		e.qEncode(&buf, charset, s)
	}
	closeWord(&buf)

	return buf.String()
}

const (
	// The maximum length of an encoded-word is 75 characters.
	// See RFC 2047, section 2.
	maxEncodedWordLen = 75
	// maxContentLen is how much content can be encoded, ignoring the header and
	// 2-byte footer.
	maxContentLen = maxEncodedWordLen - len("=?UTF-8?q?") - len("?=")
)

var maxBase64Len = base64.StdEncoding.DecodedLen(maxContentLen)

// bEncode encodes s using base64 encoding and writes it to buf.
func (e WordEncoder) bEncode(buf *strings.Builder, charset, s string) {
	w := base64.NewEncoder(base64.StdEncoding, buf)
	// If the charset is not UTF-8 or if the content is short, do not bother
	// splitting the encoded-word.
	if !isUTF8(charset) || base64.StdEncoding.EncodedLen(len(s)) <= maxContentLen {
		io.WriteString(w, s)
		w.Close()
		return
	}

	var currentLen, last, runeLen int
	for i := 0; i < len(s); i += runeLen {
		// Multi-byte characters must not be split across encoded-words.
		// See RFC 2047, section 5.3.
		_, runeLen = utf8.DecodeRuneInString(s[i:])

		if currentLen+runeLen <= maxBase64Len {
			currentLen += runeLen
		} else {
			io.WriteString(w, s[last:i])
			w.Close()
			e.splitWord(buf, charset)
			last = i
			currentLen = runeLen
		}
	}
	io.WriteString(w, s[last:])
	w.Close()
}

// qEncode encodes s using Q encoding and writes it to buf. It splits the
// encoded-words when necessary.
func (e WordEncoder) qEncode(buf *strings.Builder, charset, s string) {
	// We only split encoded-words when the charset is UTF-8.
	if !isUTF8(charset) {
		writeQString(buf, s)
		return
	}

	var currentLen, runeLen int
	for i := 0; i < len(s); i += runeLen {
		b := s[i]
		// Multi-byte characters must not be split across encoded-words.
		// See RFC 2047, section 5.3.
		var encLen int
		if b >= ' ' && b <= '~' && b != '=' && b != '?' && b != '_' {
			runeLen, encLen = 1, 1
		} else {
			_, runeLen = utf8.DecodeRuneInString(s[i:])
			encLen = 3 * runeLen
		}

		if currentLen+encLen > maxContentLen {
			e.splitWord(buf, charset)
			currentLen = 0
		}
		writeQString(buf, s[i:i+runeLen])
		currentLen += encLen
	}
}

// writeQString encodes s using Q encoding and writes it to buf.
func writeQString(buf *strings.Builder, s string) {
	for i := 0; i < len(s); i++ {
		switch b := s[i]; {
		case b == ' ':
			buf.WriteByte('_')
		case b >= '!' && b <= '~' && b != '=' && b != '?' && b != '_':
			buf.WriteByte(b)
		default:
			buf.WriteByte('=')
			buf.WriteByte(upperhex[b>>4])
			buf.WriteByte(upperhex[b&0x0f])
		}
	}
}

// openWord writes the beginning of an encoded-word into buf.
func (e WordEncoder) openWord(buf *strings.Builder, charset string) {
	buf.WriteString("=?")
	buf.WriteString(charset)
	buf.WriteByte('?')
	buf.WriteByte(byte(e))
	buf.WriteByte('?')
}

// closeWord writes the end of an encoded-word into buf.
func closeWord(buf *strings.Builder) {
	buf.WriteString("?=")
}

// splitWord closes the current encoded-word and opens a new one.
func (e WordEncoder) splitWord(buf *strings.Builder, charset string) {
	closeWord(buf)
	buf.WriteByte(' ')
	e.openWord(buf, charset)
}

func isUTF8(charset string) bool {
	return strings.EqualFold(charset, "UTF-8")
}

const upperhex = "0123456789ABCDEF"

// A WordDecoder decodes MIME headers containing RFC 2047 encoded-words.
type WordDecoder struct {
	// CharsetReader, if non-nil, defines a function to generate
	// charset-conversion readers, converting from the provided
	// charset into UTF-8.
	// Charsets are always lower-case. utf-8, iso-8859-1 and us-ascii charsets
	// are handled by default.
	// One of the CharsetReader's result values must be non-nil.
	CharsetReader func(charset string, input io.Reader) (io.Reader, error)
}

// Decode decodes an RFC 2047 encoded-word.
func (d *WordDecoder) Decode(word string) (string, error) {
	// See https://tools.ietf.org/html/rfc2047#section-2 for details.
	// Our decoder is permissive, we accept empty encoded-text.
	if len(word) < 8 || !strings.HasPrefix(word, "=?") || !strings.HasSuffix(word, "?=") || strings.Count(word, "?") != 4 {
		return "", errInvalidWord
	}
	word = word[2 : len(word)-2]

	// split word "UTF-8?q?text" into "UTF-8", 'q', and "text"
	charset, text, _ := strings.Cut(word, "?")
	if charset == "" {
		return "", errInvalidWord
	}
	encoding, text, _ := strings.Cut(text, "?")
	if len(encoding) != 1 {
		return "", errInvalidWord
	}

	content, err := decode(encoding[0], text)
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	if err := d.convert(&buf, charset, content); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// DecodeHeader decodes all encoded-words of the given string. It returns an
// error if and only if [WordDecoder.CharsetReader] of d returns an error.
func (d *WordDecoder) DecodeHeader(header string) (string, error) {
	// If there is no encoded-word, returns before creating a buffer.
	i := strings.Index(header, "=?")
	if i == -1 {
		return header, nil
	}

	var buf strings.Builder

	buf.WriteString(header[:i])
	header = header[i:]

	betweenWords := false
	for {
		start := strings.Index(header, "=?")
		if start == -1 {
			break
		}
		cur := start + len("=?")

		i := strings.Index(header[cur:], "?")
		if i == -1 {
			break
		}
		charset := header[cur : cur+i]
		cur += i + len("?")

		if len(header) < cur+len("Q??=") {
			break
		}
		encoding := header[cur]
		cur++

		if header[cur] != '?' {
			break
		}
		cur++

		j := strings.Index(header[cur:], "?=")
		if j == -1 {
			break
		}
		text := header[cur : cur+j]
		end := cur + j + len("?=")

		content, err := decode(encoding, text)
		if err != nil {
			betweenWords = false
			buf.WriteString(header[:end])
			header = header[end:]
			continue
		}

		// Write characters before the encoded-word. White-space and newline
		// characters separating two encoded-words must be deleted.
		if start > 0 && (!betweenWords || hasNonWhitespace(header[:start])) {
			buf.WriteString(header[:start])
		}

		if err := d.convert(&buf, charset, content); err != nil {
			return "", err
		}

		header = header[end:]
		betweenWords = true
	}

	if len(header) > 0 {
		buf.WriteString(header)
	}

	return buf.String(), nil
}

func decode(encoding byte, text string) ([]byte, error) {
	switch encoding {
	case 'B', 'b':
		return base64.StdEncoding.DecodeString(text)
	case 'Q', 'q':
		return qDecode(text)
	default:
		return nil, errInvalidWord
	}
}

func (d *WordDecoder) convert(buf *strings.Builder, charset string, content []byte) error {
	switch {
	case strings.EqualFold("utf-8", charset):
		buf.Write(content)
	case strings.EqualFold("iso-8859-1", charset):
		for _, c := range content {
			buf.WriteRune(rune(c))
		}
	case strings.EqualFold("us-ascii", charset):
		for _, c := range content {
			if c >= utf8.RuneSelf {
				buf.WriteRune(unicode.ReplacementChar)
			} else {
				buf.WriteByte(c)
			}
		}
	default:
		if d.CharsetReader == nil {
			return fmt.Errorf("mime: unhandled charset %q", charset)
		}
		r, err := d.CharsetReader(strings.ToLower(charset), bytes.NewReader(content))
		if err != nil {
			return err
		}
		if _, err = io.Copy(buf, r); err != nil {
			return err
		}
	}
	return nil
}

// hasNonWhitespace reports whether s (assumed to be ASCII) contains at least
// one byte of non-whitespace.
func hasNonWhitespace(s string) bool {
	for _, b := range s {
		switch b {
		// Encoded-words can only be separated by linear white spaces which does
		// not include vertical tabs (\v).
		case ' ', '\t', '\n', '\r':
		default:
			return true
		}
	}
	return false
}

// qDecode decodes a Q encoded string.
func qDecode(s string) ([]byte, error) {
	dec := make([]byte, len(s))
	n := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '_':
			dec[n] = ' '
		case c == '=':
			if i+2 >= len(s) {
				return nil, errInvalidWord
			}
			b, err := readHexByte(s[i+1], s[i+2])
			if err != nil {
				return nil, err
			}
			dec[n] = b
			i += 2
		case (c <= '~' && c >= ' ') || c == '\n' || c == '\r' || c == '\t':
			dec[n] = c
		default:
			return nil, errInvalidWord
		}
		n++
	}

	return dec[:n], nil
}

// readHexByte returns the byte from its quoted-printable representation.
func readHexByte(a, b byte) (byte, error) {
	var hb, lb byte
	var err error
	if hb, err = fromHex(a); err != nil {
		return 0, err
	}
	if lb, err = fromHex(b); err != nil {
		return 0, err
	}
	return hb<<4 | lb, nil
}

func fromHex(b byte) (byte, error) {
	switch {
	case b >= '0' && b <= '9':
		return b - '0', nil
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10, nil
	// Accept badly encoded bytes.
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10, nil
	}
	return 0, fmt.Errorf("mime: invalid hex byte %#02x", b)
}

```

// === FILE: references/go/src/mime/grammar.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mime

// isTSpecial reports whether c is in 'tspecials' as defined by RFC
// 1521 and RFC 2045.
func isTSpecial(c byte) bool {
	// tspecials :=  "(" / ")" / "<" / ">" / "@" /
	//               "," / ";" / ":" / "\" / <">
	//               "/" / "[" / "]" / "?" / "="
	//
	// mask is a 128-bit bitmap with 1s for allowed bytes,
	// so that the byte c can be tested with a shift and an and.
	// If c >= 128, then 1<<c and 1<<(c-64) will both be zero,
	// and this function will return false.
	const mask = 0 |
		1<<'(' |
		1<<')' |
		1<<'<' |
		1<<'>' |
		1<<'@' |
		1<<',' |
		1<<';' |
		1<<':' |
		1<<'\\' |
		1<<'"' |
		1<<'/' |
		1<<'[' |
		1<<']' |
		1<<'?' |
		1<<'='
	return ((uint64(1)<<c)&(mask&(1<<64-1)) |
		(uint64(1)<<(c-64))&(mask>>64)) != 0
}

// isTokenChar reports whether c is in 'token' as defined by RFC
// 1521 and RFC 2045.
func isTokenChar(c byte) bool {
	// token := 1*<any (US-ASCII) CHAR except SPACE, CTLs,
	//             or tspecials>
	//
	// mask is a 128-bit bitmap with 1s for allowed bytes,
	// so that the byte c can be tested with a shift and an and.
	// If c >= 128, then 1<<c and 1<<(c-64) will both be zero,
	// and this function will return false.
	const mask = 0 |
		(1<<(10)-1)<<'0' |
		(1<<(26)-1)<<'a' |
		(1<<(26)-1)<<'A' |
		1<<'!' |
		1<<'#' |
		1<<'$' |
		1<<'%' |
		1<<'&' |
		1<<'\'' |
		1<<'*' |
		1<<'+' |
		1<<'-' |
		1<<'.' |
		1<<'^' |
		1<<'_' |
		1<<'`' |
		1<<'{' |
		1<<'|' |
		1<<'}' |
		1<<'~'
	return ((uint64(1)<<c)&(mask&(1<<64-1)) |
		(uint64(1)<<(c-64))&(mask>>64)) != 0
}

// isToken reports whether s is a 'token' as defined by RFC 1521
// and RFC 2045.
func isToken(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range []byte(s) {
		if !isTokenChar(c) {
			return false
		}
	}
	return true
}

```

// === FILE: references/go/src/mime/mediatype.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mime

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode"
)

// FormatMediaType serializes mediatype t and the parameters
// param as a media type conforming to RFC 2045 and RFC 2616.
// The type and parameter names are written in lower-case.
// When any of the arguments result in a standard violation then
// FormatMediaType returns the empty string.
func FormatMediaType(t string, param map[string]string) string {
	var b strings.Builder
	if major, sub, ok := strings.Cut(t, "/"); !ok {
		if !isToken(t) {
			return ""
		}
		b.WriteString(strings.ToLower(t))
	} else {
		if !isToken(major) || !isToken(sub) {
			return ""
		}
		b.WriteString(strings.ToLower(major))
		b.WriteByte('/')
		b.WriteString(strings.ToLower(sub))
	}

	for _, attribute := range slices.Sorted(maps.Keys(param)) {
		value := param[attribute]
		b.WriteByte(';')
		b.WriteByte(' ')
		if !isToken(attribute) {
			return ""
		}
		b.WriteString(strings.ToLower(attribute))

		needEnc := needsEncoding(value)
		if needEnc {
			// RFC 2231 section 4
			b.WriteByte('*')
		}
		b.WriteByte('=')

		if needEnc {
			b.WriteString("utf-8''")

			offset := 0
			for index := 0; index < len(value); index++ {
				ch := value[index]
				// {RFC 2231 section 7}
				// attribute-char := <any (US-ASCII) CHAR except SPACE, CTLs, "*", "'", "%", or tspecials>
				if ch <= ' ' || ch >= 0x7F ||
					ch == '*' || ch == '\'' || ch == '%' ||
					isTSpecial(ch) {

					b.WriteString(value[offset:index])
					offset = index + 1

					b.WriteByte('%')
					b.WriteByte(upperhex[ch>>4])
					b.WriteByte(upperhex[ch&0x0F])
				}
			}
			b.WriteString(value[offset:])
			continue
		}

		if isToken(value) {
			b.WriteString(value)
			continue
		}

		b.WriteByte('"')
		offset := 0
		for index := 0; index < len(value); index++ {
			character := value[index]
			if character == '"' || character == '\\' {
				b.WriteString(value[offset:index])
				offset = index
				b.WriteByte('\\')
			}
		}
		b.WriteString(value[offset:])
		b.WriteByte('"')
	}
	return b.String()
}

func checkMediaTypeDisposition(s string) error {
	typ, rest := consumeToken(s)
	if typ == "" {
		return errNoMediaType
	}
	if rest == "" {
		return nil
	}
	var ok bool
	if rest, ok = strings.CutPrefix(rest, "/"); !ok {
		return errNoSlashAfterFirstToken
	}
	subtype, rest := consumeToken(rest)
	if subtype == "" {
		return errNoTokenAfterSlash
	}
	if rest != "" {
		return errUnexpectedContentAfterMediaSubtype
	}
	return nil
}

var (
	errNoMediaType                        = errors.New("mime: no media type")
	errNoSlashAfterFirstToken             = errors.New("mime: expected slash after first token")
	errNoTokenAfterSlash                  = errors.New("mime: expected token after slash")
	errUnexpectedContentAfterMediaSubtype = errors.New("mime: unexpected content after media subtype")
)

// ErrInvalidMediaParameter is returned by [ParseMediaType] if
// the media type value was found but there was an error parsing
// the optional parameters
var ErrInvalidMediaParameter = errors.New("mime: invalid media parameter")

// ParseMediaType parses a media type value and any optional
// parameters, per RFC 1521.  Media types are the values in
// Content-Type and Content-Disposition headers (RFC 2183).
// On success, ParseMediaType returns the media type converted
// to lowercase and trimmed of white space and a non-nil map.
// If there is an error parsing the optional parameter,
// the media type will be returned along with the error
// [ErrInvalidMediaParameter].
// The returned map, params, maps from the lowercase
// attribute to the attribute value with its case preserved.
func ParseMediaType(v string) (mediatype string, params map[string]string, err error) {
	base, _, _ := strings.Cut(v, ";")
	mediatype = strings.TrimSpace(strings.ToLower(base))

	err = checkMediaTypeDisposition(mediatype)
	if err != nil {
		return "", nil, err
	}

	params = make(map[string]string)

	// Map of base parameter name -> parameter name -> value
	// for parameters containing a '*' character.
	// Lazily initialized.
	var continuation map[string]map[string]string

	v = v[len(base):]
	for len(v) > 0 {
		v = strings.TrimLeftFunc(v, unicode.IsSpace)
		if len(v) == 0 {
			break
		}
		key, value, rest := consumeMediaParam(v)
		if key == "" {
			if strings.TrimSpace(rest) == ";" {
				// Ignore trailing semicolons.
				// Not an error.
				break
			}
			// Parse error.
			return mediatype, nil, ErrInvalidMediaParameter
		}

		pmap := params
		if baseName, _, ok := strings.Cut(key, "*"); ok {
			if continuation == nil {
				continuation = make(map[string]map[string]string)
			}
			if pmap, ok = continuation[baseName]; !ok {
				continuation[baseName] = make(map[string]string)
				pmap = continuation[baseName]
			}
		}
		if v, exists := pmap[key]; exists && v != value {
			// Duplicate parameter names are incorrect, but we allow them if they are equal.
			return "", nil, errDuplicateParamName
		}
		pmap[key] = value
		v = rest
	}

	// Stitch together any continuations or things with stars
	// (i.e. RFC 2231 things with stars: "foo*0" or "foo*")
	var buf strings.Builder
	for key, pieceMap := range continuation {
		singlePartKey := key + "*"
		if v, ok := pieceMap[singlePartKey]; ok {
			if decv, ok := decode2231Enc(v); ok {
				params[key] = decv
			}
			continue
		}

		buf.Reset()
		valid := false
		for n := 0; ; n++ {
			simplePart := fmt.Sprintf("%s*%d", key, n)
			if v, ok := pieceMap[simplePart]; ok {
				valid = true
				buf.WriteString(v)
				continue
			}
			encodedPart := simplePart + "*"
			v, ok := pieceMap[encodedPart]
			if !ok {
				break
			}
			valid = true
			if n == 0 {
				if decv, ok := decode2231Enc(v); ok {
					buf.WriteString(decv)
				}
			} else {
				decv, _ := percentHexUnescape(v)
				buf.WriteString(decv)
			}
		}
		if valid {
			params[key] = buf.String()
		}
	}

	return
}

var errDuplicateParamName = errors.New("mime: duplicate parameter name")

func decode2231Enc(v string) (string, bool) {
	charset, v, ok := strings.Cut(v, "'")
	if !ok {
		return "", false
	}
	// TODO: ignoring the language part for now. If anybody needs it, we'll
	// need to decide how to expose it in the API. But I'm not sure
	// anybody uses it in practice.
	_, extOtherVals, ok := strings.Cut(v, "'")
	if !ok {
		return "", false
	}
	charset = strings.ToLower(charset)
	switch charset {
	case "us-ascii", "utf-8":
	default:
		// Empty or unsupported encoding.
		return "", false
	}
	return percentHexUnescape(extOtherVals)
}

// consumeToken consumes a token from the beginning of provided
// string, per RFC 2045 section 5.1 (referenced from 2183), and return
// the token consumed and the rest of the string. Returns ("", v) on
// failure to consume at least one character.
func consumeToken(v string) (token, rest string) {
	for i := range len(v) {
		if !isTokenChar(v[i]) {
			return v[:i], v[i:]
		}
	}
	return v, ""
}

// consumeValue consumes a "value" per RFC 2045, where a value is
// either a 'token' or a 'quoted-string'.  On success, consumeValue
// returns the value consumed (and de-quoted/escaped, if a
// quoted-string) and the rest of the string. On failure, returns
// ("", v).
func consumeValue(v string) (value, rest string) {
	if v == "" {
		return
	}
	if v[0] != '"' {
		return consumeToken(v)
	}

	// parse a quoted-string
	buffer := new(strings.Builder)
	for i := 1; i < len(v); i++ {
		r := v[i]
		if r == '"' {
			return buffer.String(), v[i+1:]
		}
		// When MSIE sends a full file path (in "intranet mode"), it does not
		// escape backslashes: "C:\dev\go\foo.txt", not "C:\\dev\\go\\foo.txt".
		//
		// No known MIME generators emit unnecessary backslash escapes
		// for simple token characters like numbers and letters.
		//
		// If we see an unnecessary backslash escape, assume it is from MSIE
		// and intended as a literal backslash. This makes Go servers deal better
		// with MSIE without affecting the way they handle conforming MIME
		// generators.
		if r == '\\' && i+1 < len(v) && isTSpecial(v[i+1]) {
			buffer.WriteByte(v[i+1])
			i++
			continue
		}
		if r == '\r' || r == '\n' {
			return "", v
		}
		buffer.WriteByte(v[i])
	}
	// Did not find end quote.
	return "", v
}

func consumeMediaParam(v string) (param, value, rest string) {
	rest = strings.TrimLeftFunc(v, unicode.IsSpace)
	var ok bool
	if rest, ok = strings.CutPrefix(rest, ";"); !ok {
		return "", "", v
	}

	rest = strings.TrimLeftFunc(rest, unicode.IsSpace)
	param, rest = consumeToken(rest)
	param = strings.ToLower(param)
	if param == "" {
		return "", "", v
	}

	rest = strings.TrimLeftFunc(rest, unicode.IsSpace)
	if rest, ok = strings.CutPrefix(rest, "="); !ok {
		return "", "", v
	}
	rest = strings.TrimLeftFunc(rest, unicode.IsSpace)
	value, rest2 := consumeValue(rest)
	if value == "" && rest2 == rest {
		return "", "", v
	}
	rest = rest2
	return param, value, rest
}

func percentHexUnescape(s string) (string, bool) {
	// Count %, check that they're well-formed.
	percents := 0
	for i := 0; i < len(s); {
		if s[i] != '%' {
			i++
			continue
		}
		percents++
		if i+2 >= len(s) || !ishex(s[i+1]) || !ishex(s[i+2]) {
			return "", false
		}
		i += 3
	}
	if percents == 0 {
		return s, true
	}

	t := make([]byte, len(s)-2*percents)
	j := 0
	for i := 0; i < len(s); {
		switch s[i] {
		case '%':
			t[j] = unhex(s[i+1])<<4 | unhex(s[i+2])
			j++
			i += 3
		default:
			t[j] = s[i]
			j++
			i++
		}
	}
	return string(t), true
}

func ishex(c byte) bool {
	switch {
	case '0' <= c && c <= '9':
		return true
	case 'a' <= c && c <= 'f':
		return true
	case 'A' <= c && c <= 'F':
		return true
	}
	return false
}

func unhex(c byte) byte {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}

```

// === FILE: references/go/src/mime/multipart/formdata.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package multipart

import (
	"bytes"
	"errors"
	"internal/godebug"
	"io"
	"math"
	"net/textproto"
	"os"
	"strconv"
)

// ErrMessageTooLarge is returned by ReadForm if the message form
// data is too large to be processed.
var ErrMessageTooLarge = errors.New("multipart: message too large")

// TODO(adg,bradfitz): find a way to unify the DoS-prevention strategy here
// with that of the http package's ParseForm.

// ReadForm parses an entire multipart message whose parts have
// a Content-Disposition of "form-data".
// It stores up to maxMemory bytes + 10MB (reserved for non-file parts)
// in memory. File parts which can't be stored in memory will be stored on
// disk in temporary files.
// It returns [ErrMessageTooLarge] if all non-file parts can't be stored in
// memory.
func (r *Reader) ReadForm(maxMemory int64) (*Form, error) {
	return r.readForm(maxMemory)
}

var (
	multipartfiles    = godebug.New("#multipartfiles") // TODO: document and remove #
	multipartmaxparts = godebug.New("multipartmaxparts")
)

func (r *Reader) readForm(maxMemory int64) (_ *Form, err error) {
	form := &Form{make(map[string][]string), make(map[string][]*FileHeader)}
	var (
		file    *os.File
		fileOff int64
	)
	numDiskFiles := 0
	combineFiles := true
	if multipartfiles.Value() == "distinct" {
		combineFiles = false
		// multipartfiles.IncNonDefault() // TODO: uncomment after documenting
	}
	maxParts := 1000
	if s := multipartmaxparts.Value(); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v >= 0 {
			maxParts = v
			multipartmaxparts.IncNonDefault()
		}
	}
	maxHeaders := maxMIMEHeaders()

	defer func() {
		if file != nil {
			if cerr := file.Close(); err == nil {
				err = cerr
			}
		}
		if combineFiles && numDiskFiles > 1 {
			for _, fhs := range form.File {
				for _, fh := range fhs {
					fh.tmpshared = true
				}
			}
		}
		if err != nil {
			form.RemoveAll()
			if file != nil {
				os.Remove(file.Name())
			}
		}
	}()

	// maxFileMemoryBytes is the maximum bytes of file data we will store in memory.
	// Data past this limit is written to disk.
	// This limit strictly applies to content, not metadata (filenames, MIME headers, etc.),
	// since metadata is always stored in memory, not disk.
	//
	// maxMemoryBytes is the maximum bytes we will store in memory, including file content,
	// non-file part values, metadata, and map entry overhead.
	//
	// We reserve an additional 10 MB in maxMemoryBytes for non-file data.
	//
	// The relationship between these parameters, as well as the overly-large and
	// unconfigurable 10 MB added on to maxMemory, is unfortunate but difficult to change
	// within the constraints of the API as documented.
	maxFileMemoryBytes := maxMemory
	if maxFileMemoryBytes == math.MaxInt64 {
		maxFileMemoryBytes--
	}
	maxMemoryBytes := maxMemory + int64(10<<20)
	if maxMemoryBytes <= 0 {
		if maxMemory < 0 {
			maxMemoryBytes = 0
		} else {
			maxMemoryBytes = math.MaxInt64
		}
	}
	var copyBuf []byte
	for {
		p, err := r.nextPart(false, maxMemoryBytes, maxHeaders)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if maxParts <= 0 {
			return nil, ErrMessageTooLarge
		}
		maxParts--

		name := p.FormName()
		if name == "" {
			continue
		}
		filename := p.FileName()

		// Multiple values for the same key (one map entry, longer slice) are cheaper
		// than the same number of values for different keys (many map entries), but
		// using a consistent per-value cost for overhead is simpler.
		const mapEntryOverhead = 200
		maxMemoryBytes -= int64(len(name))
		maxMemoryBytes -= mapEntryOverhead
		if maxMemoryBytes < 0 {
			// We can't actually take this path, since nextPart would already have
			// rejected the MIME headers for being too large. Check anyway.
			return nil, ErrMessageTooLarge
		}

		var b bytes.Buffer

		if filename == "" {
			// value, store as string in memory
			n, err := io.CopyN(&b, p, maxMemoryBytes+1)
			if err != nil && err != io.EOF {
				return nil, err
			}
			maxMemoryBytes -= n
			if maxMemoryBytes < 0 {
				return nil, ErrMessageTooLarge
			}
			form.Value[name] = append(form.Value[name], b.String())
			continue
		}

		// file, store in memory or on disk
		const fileHeaderSize = 100
		maxMemoryBytes -= mimeHeaderSize(p.Header)
		maxMemoryBytes -= mapEntryOverhead
		maxMemoryBytes -= fileHeaderSize
		if maxMemoryBytes < 0 {
			return nil, ErrMessageTooLarge
		}
		for _, v := range p.Header {
			maxHeaders -= int64(len(v))
		}
		fh := &FileHeader{
			Filename: filename,
			Header:   p.Header,
		}
		n, err := io.CopyN(&b, p, maxFileMemoryBytes+1)
		if err != nil && err != io.EOF {
			return nil, err
		}
		if n > maxFileMemoryBytes {
			if file == nil {
				file, err = os.CreateTemp(r.tempDir, "multipart-")
				if err != nil {
					return nil, err
				}
			}
			numDiskFiles++
			if _, err := file.Write(b.Bytes()); err != nil {
				return nil, err
			}
			if copyBuf == nil {
				copyBuf = make([]byte, 32*1024) // same buffer size as io.Copy uses
			}
			// os.File.ReadFrom will allocate its own copy buffer if we let io.Copy use it.
			type writerOnly struct{ io.Writer }
			remainingSize, err := io.CopyBuffer(writerOnly{file}, p, copyBuf)
			if err != nil {
				return nil, err
			}
			fh.tmpfile = file.Name()
			fh.Size = int64(b.Len()) + remainingSize
			fh.tmpoff = fileOff
			fileOff += fh.Size
			if !combineFiles {
				if err := file.Close(); err != nil {
					return nil, err
				}
				file = nil
			}
		} else {
			fh.content = b.Bytes()
			fh.Size = int64(len(fh.content))
			maxFileMemoryBytes -= n
			maxMemoryBytes -= n
		}
		form.File[name] = append(form.File[name], fh)
	}

	return form, nil
}

func mimeHeaderSize(h textproto.MIMEHeader) (size int64) {
	size = 400
	for k, vs := range h {
		size += int64(len(k))
		size += 200 // map entry overhead
		for _, v := range vs {
			size += int64(len(v))
		}
	}
	return size
}

// Form is a parsed multipart form.
// Its File parts are stored either in memory or on disk,
// and are accessible via the [*FileHeader]'s Open method.
// Its Value parts are stored as strings.
// Both are keyed by field name.
type Form struct {
	Value map[string][]string
	File  map[string][]*FileHeader
}

// RemoveAll removes any temporary files associated with a [Form].
func (f *Form) RemoveAll() error {
	var err error
	for _, fhs := range f.File {
		for _, fh := range fhs {
			if fh.tmpfile != "" {
				e := os.Remove(fh.tmpfile)
				if e != nil && !errors.Is(e, os.ErrNotExist) && err == nil {
					err = e
				}
			}
		}
	}
	return err
}

// A FileHeader describes a file part of a multipart request.
type FileHeader struct {
	Filename string
	Header   textproto.MIMEHeader
	Size     int64

	content   []byte
	tmpfile   string
	tmpoff    int64
	tmpshared bool
}

// Open opens and returns the [FileHeader]'s associated File.
func (fh *FileHeader) Open() (File, error) {
	if b := fh.content; b != nil {
		r := io.NewSectionReader(bytes.NewReader(b), 0, int64(len(b)))
		return sectionReadCloser{r, nil}, nil
	}
	if fh.tmpshared {
		f, err := os.Open(fh.tmpfile)
		if err != nil {
			return nil, err
		}
		r := io.NewSectionReader(f, fh.tmpoff, fh.Size)
		return sectionReadCloser{r, f}, nil
	}
	return os.Open(fh.tmpfile)
}

// File is an interface to access the file part of a multipart message.
// Its contents may be either stored in memory or on disk.
// If stored on disk, the File's underlying concrete type will be an *os.File.
type File interface {
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Closer
}

// helper types to turn a []byte into a File

type sectionReadCloser struct {
	*io.SectionReader
	io.Closer
}

func (rc sectionReadCloser) Close() error {
	if rc.Closer != nil {
		return rc.Closer.Close()
	}
	return nil
}

```

// === FILE: references/go/src/mime/multipart/multipart.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

/*
Package multipart implements MIME multipart parsing, as defined in RFC
2046.

The implementation is sufficient for HTTP (RFC 2388) and the multipart
bodies generated by popular browsers.

# Limits

To protect against malicious inputs, this package sets limits on the size
of the MIME data it processes.

[Reader.NextPart] and [Reader.NextRawPart] limit the number of headers in a
part to 10000 and [Reader.ReadForm] limits the total number of headers in all
FileHeaders to 10000.
These limits may be adjusted with the GODEBUG=multipartmaxheaders=<values>
setting.

Reader.ReadForm further limits the number of parts in a form to 1000.
This limit may be adjusted with the GODEBUG=multipartmaxparts=<value>
setting.
*/
package multipart

import (
	"bufio"
	"bytes"
	"fmt"
	"internal/godebug"
	"io"
	"mime"
	"mime/quotedprintable"
	"net/textproto"
	"path/filepath"
	"strconv"
	"strings"
)

var emptyParams = make(map[string]string)

// This constant needs to be at least 76 for this package to work correctly.
// This is because \r\n--separator_of_len_70- would fill the buffer and it
// wouldn't be safe to consume a single byte from it.
const peekBufferSize = 4096

// A Part represents a single part in a multipart body.
type Part struct {
	// The headers of the body, if any, with the keys canonicalized
	// in the same fashion that the Go http.Request headers are.
	// For example, "foo-bar" changes case to "Foo-Bar"
	Header textproto.MIMEHeader

	mr *Reader

	disposition       string
	dispositionParams map[string]string

	// r is either a reader directly reading from mr, or it's a
	// wrapper around such a reader, decoding the
	// Content-Transfer-Encoding
	r io.Reader

	n       int   // known data bytes waiting in mr.bufReader
	total   int64 // total data bytes read already
	err     error // error to return when n == 0
	readErr error // read error observed from mr.bufReader
}

// FormName returns the name parameter if p has a Content-Disposition
// of type "form-data".  Otherwise it returns the empty string.
func (p *Part) FormName() string {
	// See https://tools.ietf.org/html/rfc2183 section 2 for EBNF
	// of Content-Disposition value format.
	if p.dispositionParams == nil {
		p.parseContentDisposition()
	}
	if p.disposition != "form-data" {
		return ""
	}
	return p.dispositionParams["name"]
}

// FileName returns the filename parameter of the [Part]'s Content-Disposition
// header. If not empty, the filename is passed through filepath.Base (which is
// platform dependent) before being returned.
func (p *Part) FileName() string {
	if p.dispositionParams == nil {
		p.parseContentDisposition()
	}
	filename := p.dispositionParams["filename"]
	if filename == "" {
		return ""
	}
	// RFC 7578, Section 4.2 requires that if a filename is provided, the
	// directory path information must not be used.
	return filepath.Base(filename)
}

func (p *Part) parseContentDisposition() {
	v := p.Header.Get("Content-Disposition")
	var err error
	p.disposition, p.dispositionParams, err = mime.ParseMediaType(v)
	if err != nil {
		p.dispositionParams = emptyParams
	}
}

// NewReader creates a new multipart [Reader] reading from r using the
// given MIME boundary.
//
// The boundary is usually obtained from the "boundary" parameter of
// the message's "Content-Type" header. Use [mime.ParseMediaType] to
// parse such headers.
func NewReader(r io.Reader, boundary string) *Reader {
	b := []byte("\r\n--" + boundary + "--")
	return &Reader{
		bufReader:        bufio.NewReaderSize(&stickyErrorReader{r: r}, peekBufferSize),
		nl:               b[:2],
		nlDashBoundary:   b[:len(b)-2],
		dashBoundaryDash: b[2:],
		dashBoundary:     b[2 : len(b)-2],
	}
}

// stickyErrorReader is an io.Reader which never calls Read on its
// underlying Reader once an error has been seen. (the io.Reader
// interface's contract promises nothing about the return values of
// Read calls after an error, yet this package does do multiple Reads
// after error)
type stickyErrorReader struct {
	r   io.Reader
	err error
}

func (r *stickyErrorReader) Read(p []byte) (n int, _ error) {
	if r.err != nil {
		return 0, r.err
	}
	n, r.err = r.r.Read(p)
	return n, r.err
}

func newPart(mr *Reader, rawPart bool, maxMIMEHeaderSize, maxMIMEHeaders int64) (*Part, error) {
	bp := &Part{
		Header: make(map[string][]string),
		mr:     mr,
	}
	if err := bp.populateHeaders(maxMIMEHeaderSize, maxMIMEHeaders); err != nil {
		return nil, err
	}
	bp.r = partReader{bp}

	// rawPart is used to switch between Part.NextPart and Part.NextRawPart.
	if !rawPart {
		const cte = "Content-Transfer-Encoding"
		if strings.EqualFold(bp.Header.Get(cte), "quoted-printable") {
			bp.Header.Del(cte)
			bp.r = quotedprintable.NewReader(bp.r)
		}
	}
	return bp, nil
}

func (p *Part) populateHeaders(maxMIMEHeaderSize, maxMIMEHeaders int64) error {
	r := textproto.NewReader(p.mr.bufReader)
	header, err := readMIMEHeader(r, maxMIMEHeaderSize, maxMIMEHeaders)
	if err == nil {
		p.Header = header
	}
	// TODO: Add a distinguishable error to net/textproto.
	if err != nil && err.Error() == "message too large" {
		err = ErrMessageTooLarge
	}
	return err
}

// Read reads the body of a part, after its headers and before the
// next part (if any) begins.
func (p *Part) Read(d []byte) (n int, err error) {
	return p.r.Read(d)
}

// partReader implements io.Reader by reading raw bytes directly from the
// wrapped *Part, without doing any Transfer-Encoding decoding.
type partReader struct {
	p *Part
}

func (pr partReader) Read(d []byte) (int, error) {
	p := pr.p
	br := p.mr.bufReader

	// Read into buffer until we identify some data to return,
	// or we find a reason to stop (boundary or read error).
	for p.n == 0 && p.err == nil {
		peek, _ := br.Peek(br.Buffered())
		p.n, p.err = scanUntilBoundary(peek, p.mr.dashBoundary, p.mr.nlDashBoundary, p.total, p.readErr)
		if p.n == 0 && p.err == nil {
			// Force buffered I/O to read more into buffer.
			_, p.readErr = br.Peek(len(peek) + 1)
			if p.readErr == io.EOF {
				p.readErr = io.ErrUnexpectedEOF
			}
		}
	}

	// Read out from "data to return" part of buffer.
	if p.n == 0 {
		return 0, p.err
	}
	n := len(d)
	if n > p.n {
		n = p.n
	}
	n, _ = br.Read(d[:n])
	p.total += int64(n)
	p.n -= n
	if p.n == 0 {
		return n, p.err
	}
	return n, nil
}

// scanUntilBoundary scans buf to identify how much of it can be safely
// returned as part of the Part body.
// dashBoundary is "--boundary".
// nlDashBoundary is "\r\n--boundary" or "\n--boundary", depending on what mode we are in.
// The comments below (and the name) assume "\n--boundary", but either is accepted.
// total is the number of bytes read out so far. If total == 0, then a leading "--boundary" is recognized.
// readErr is the read error, if any, that followed reading the bytes in buf.
// scanUntilBoundary returns the number of data bytes from buf that can be
// returned as part of the Part body and also the error to return (if any)
// once those data bytes are done.
func scanUntilBoundary(buf, dashBoundary, nlDashBoundary []byte, total int64, readErr error) (int, error) {
	if total == 0 {
		// At beginning of body, allow dashBoundary.
		if bytes.HasPrefix(buf, dashBoundary) {
			switch matchAfterPrefix(buf, dashBoundary, readErr) {
			case -1:
				return len(dashBoundary), nil
			case 0:
				return 0, nil
			case +1:
				return 0, io.EOF
			}
		}
		if bytes.HasPrefix(dashBoundary, buf) {
			return 0, readErr
		}
	}

	// Search for "\n--boundary".
	if i := bytes.Index(buf, nlDashBoundary); i >= 0 {
		switch matchAfterPrefix(buf[i:], nlDashBoundary, readErr) {
		case -1:
			return i + len(nlDashBoundary), nil
		case 0:
			return i, nil
		case +1:
			return i, io.EOF
		}
	}
	if bytes.HasPrefix(nlDashBoundary, buf) {
		return 0, readErr
	}

	// Otherwise, anything up to the final \n is not part of the boundary
	// and so must be part of the body.
	// Also if the section from the final \n onward is not a prefix of the boundary,
	// it too must be part of the body.
	i := bytes.LastIndexByte(buf, nlDashBoundary[0])
	if i >= 0 && bytes.HasPrefix(nlDashBoundary, buf[i:]) {
		return i, nil
	}
	return len(buf), readErr
}

// matchAfterPrefix checks whether buf should be considered to match the boundary.
// The prefix is "--boundary" or "\r\n--boundary" or "\n--boundary",
// and the caller has verified already that bytes.HasPrefix(buf, prefix) is true.
//
// matchAfterPrefix returns +1 if the buffer does match the boundary,
// meaning the prefix is followed by a double dash, space, tab, cr, nl,
// or end of input.
// It returns -1 if the buffer definitely does NOT match the boundary,
// meaning the prefix is followed by some other character.
// For example, "--foobar" does not match "--foo".
// It returns 0 more input needs to be read to make the decision,
// meaning that len(buf) == len(prefix) and readErr == nil.
func matchAfterPrefix(buf, prefix []byte, readErr error) int {
	if len(buf) == len(prefix) {
		if readErr != nil {
			return +1
		}
		return 0
	}
	c := buf[len(prefix)]

	if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
		return +1
	}

	// Try to detect boundaryDash
	if c == '-' {
		if len(buf) == len(prefix)+1 {
			if readErr != nil {
				// Prefix + "-" does not match
				return -1
			}
			return 0
		}
		if buf[len(prefix)+1] == '-' {
			return +1
		}
	}

	return -1
}

func (p *Part) Close() error {
	io.Copy(io.Discard, p)
	return nil
}

// Reader is an iterator over parts in a MIME multipart body.
// Reader's underlying parser consumes its input as needed. Seeking
// isn't supported.
type Reader struct {
	bufReader *bufio.Reader
	tempDir   string // used in tests

	currentPart *Part
	partsRead   int

	nl               []byte // "\r\n" or "\n" (set after seeing first boundary line)
	nlDashBoundary   []byte // nl + "--boundary"
	dashBoundaryDash []byte // "--boundary--"
	dashBoundary     []byte // "--boundary"
}

// maxMIMEHeaderSize is the maximum size of a MIME header we will parse,
// including header keys, values, and map overhead.
const maxMIMEHeaderSize = 10 << 20

// multipartmaxheaders is the maximum number of header entries NextPart will return,
// as well as the maximum combined total of header entries Reader.ReadForm will return
// in FileHeaders.
var multipartmaxheaders = godebug.New("multipartmaxheaders")

func maxMIMEHeaders() int64 {
	if s := multipartmaxheaders.Value(); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil && v >= 0 {
			multipartmaxheaders.IncNonDefault()
			return v
		}
	}
	return 10000
}

// NextPart returns the next part in the multipart or an error.
// When there are no more parts, the error [io.EOF] is returned.
//
// As a special case, if the "Content-Transfer-Encoding" header
// has a value of "quoted-printable", that header is instead
// hidden and the body is transparently decoded during Read calls.
func (r *Reader) NextPart() (*Part, error) {
	return r.nextPart(false, maxMIMEHeaderSize, maxMIMEHeaders())
}

// NextRawPart returns the next part in the multipart or an error.
// When there are no more parts, the error [io.EOF] is returned.
//
// Unlike [Reader.NextPart], it does not have special handling for
// "Content-Transfer-Encoding: quoted-printable".
func (r *Reader) NextRawPart() (*Part, error) {
	return r.nextPart(true, maxMIMEHeaderSize, maxMIMEHeaders())
}

func (r *Reader) nextPart(rawPart bool, maxMIMEHeaderSize, maxMIMEHeaders int64) (*Part, error) {
	if r.currentPart != nil {
		r.currentPart.Close()
	}
	if string(r.dashBoundary) == "--" {
		return nil, fmt.Errorf("multipart: boundary is empty")
	}
	expectNewPart := false
	for {
		line, err := r.bufReader.ReadSlice('\n')

		if err == io.EOF && r.isFinalBoundary(line) {
			// If the buffer ends in "--boundary--" without the
			// trailing "\r\n", ReadSlice will return an error
			// (since it's missing the '\n'), but this is a valid
			// multipart EOF so we need to return io.EOF instead of
			// a fmt-wrapped one.
			return nil, io.EOF
		}
		if err != nil {
			return nil, fmt.Errorf("multipart: NextPart: %w", err)
		}

		if r.isBoundaryDelimiterLine(line) {
			r.partsRead++
			bp, err := newPart(r, rawPart, maxMIMEHeaderSize, maxMIMEHeaders)
			if err != nil {
				return nil, err
			}
			r.currentPart = bp
			return bp, nil
		}

		if r.isFinalBoundary(line) {
			// Expected EOF
			return nil, io.EOF
		}

		if expectNewPart {
			return nil, fmt.Errorf("multipart: expecting a new Part; got line %q", string(line))
		}

		if r.partsRead == 0 {
			// skip line
			continue
		}

		// Consume the "\n" or "\r\n" separator between the
		// body of the previous part and the boundary line we
		// now expect will follow. (either a new part or the
		// end boundary)
		if bytes.Equal(line, r.nl) {
			expectNewPart = true
			continue
		}

		return nil, fmt.Errorf("multipart: unexpected line in Next(): %q", line)
	}
}

// isFinalBoundary reports whether line is the final boundary line
// indicating that all parts are over.
// It matches `^--boundary--[ \t]*(\r\n)?$`
func (r *Reader) isFinalBoundary(line []byte) bool {
	if !bytes.HasPrefix(line, r.dashBoundaryDash) {
		return false
	}
	rest := line[len(r.dashBoundaryDash):]
	rest = skipLWSPChar(rest)
	return len(rest) == 0 || bytes.Equal(rest, r.nl)
}

func (r *Reader) isBoundaryDelimiterLine(line []byte) (ret bool) {
	// https://tools.ietf.org/html/rfc2046#section-5.1
	//   The boundary delimiter line is then defined as a line
	//   consisting entirely of two hyphen characters ("-",
	//   decimal value 45) followed by the boundary parameter
	//   value from the Content-Type header field, optional linear
	//   whitespace, and a terminating CRLF.
	if !bytes.HasPrefix(line, r.dashBoundary) {
		return false
	}
	rest := line[len(r.dashBoundary):]
	rest = skipLWSPChar(rest)

	// On the first part, see our lines are ending in \n instead of \r\n
	// and switch into that mode if so. This is a violation of the spec,
	// but occurs in practice.
	if r.partsRead == 0 && len(rest) == 1 && rest[0] == '\n' {
		r.nl = r.nl[1:]
		r.nlDashBoundary = r.nlDashBoundary[1:]
	}
	return bytes.Equal(rest, r.nl)
}

// skipLWSPChar returns b with leading spaces and tabs removed.
// RFC 822 defines:
//
//	LWSP-char = SPACE / HTAB
func skipLWSPChar(b []byte) []byte {
	for len(b) > 0 && (b[0] == ' ' || b[0] == '\t') {
		b = b[1:]
	}
	return b
}

```

// === FILE: references/go/src/mime/multipart/readmimeheader.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package multipart

import (
	"net/textproto"
	_ "unsafe" // for go:linkname
)

// readMIMEHeader is defined in package [net/textproto].
//
//go:linkname readMIMEHeader net/textproto.readMIMEHeader
func readMIMEHeader(r *textproto.Reader, maxMemory, maxHeaders int64) (textproto.MIMEHeader, error)

```

// === FILE: references/go/src/mime/multipart/writer.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package multipart

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/textproto"
	"slices"
	"strings"
)

// A Writer generates multipart messages.
type Writer struct {
	w        io.Writer
	boundary string
	lastpart *part
}

// NewWriter returns a new multipart [Writer] with a random boundary,
// writing to w.
func NewWriter(w io.Writer) *Writer {
	return &Writer{
		w:        w,
		boundary: randomBoundary(),
	}
}

// Boundary returns the [Writer]'s boundary.
func (w *Writer) Boundary() string {
	return w.boundary
}

// SetBoundary overrides the [Writer]'s default randomly-generated
// boundary separator with an explicit value.
//
// SetBoundary must be called before any parts are created, may only
// contain certain ASCII characters, and must be non-empty and
// at most 70 bytes long.
func (w *Writer) SetBoundary(boundary string) error {
	if w.lastpart != nil {
		return errors.New("mime: SetBoundary called after write")
	}
	// rfc2046#section-5.1.1
	if len(boundary) < 1 || len(boundary) > 70 {
		return errors.New("mime: invalid boundary length")
	}
	end := len(boundary) - 1
	for i, b := range boundary {
		if 'A' <= b && b <= 'Z' || 'a' <= b && b <= 'z' || '0' <= b && b <= '9' {
			continue
		}
		switch b {
		case '\'', '(', ')', '+', '_', ',', '-', '.', '/', ':', '=', '?':
			continue
		case ' ':
			if i != end {
				continue
			}
		}
		return errors.New("mime: invalid boundary character")
	}
	w.boundary = boundary
	return nil
}

// FormDataContentType returns the Content-Type for an HTTP
// multipart/form-data with this [Writer]'s Boundary.
func (w *Writer) FormDataContentType() string {
	b := w.boundary
	// We must quote the boundary if it contains any of the
	// tspecials characters defined by RFC 2045, or space.
	if strings.ContainsAny(b, `()<>@,;:\"/[]?= `) {
		b = `"` + b + `"`
	}
	return "multipart/form-data; boundary=" + b
}

func randomBoundary() string {
	var buf [30]byte
	_, err := io.ReadFull(rand.Reader, buf[:])
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", buf[:])
}

// CreatePart creates a new multipart section with the provided
// header. The body of the part should be written to the returned
// [Writer]. After calling CreatePart, any previous part may no longer
// be written to.
func (w *Writer) CreatePart(header textproto.MIMEHeader) (io.Writer, error) {
	if w.lastpart != nil {
		if err := w.lastpart.close(); err != nil {
			return nil, err
		}
	}
	var b bytes.Buffer
	if w.lastpart != nil {
		fmt.Fprintf(&b, "\r\n--%s\r\n", w.boundary)
	} else {
		fmt.Fprintf(&b, "--%s\r\n", w.boundary)
	}

	for _, k := range slices.Sorted(maps.Keys(header)) {
		for _, v := range header[k] {
			fmt.Fprintf(&b, "%s: %s\r\n", k, v)
		}
	}
	fmt.Fprintf(&b, "\r\n")
	_, err := io.Copy(w.w, &b)
	if err != nil {
		return nil, err
	}
	p := &part{
		mw: w,
	}
	w.lastpart = p
	return p, nil
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"", "\r", "%0D", "\n", "%0A")

// escapeQuotes escapes special characters in field parameter values.
//
// For historical reasons, this uses \ escaping for " and \ characters,
// and percent encoding for CR and LF.
//
// The WhatWG specification for form data encoding suggests that we should
// use percent encoding for " (%22), and should not escape \.
// https://html.spec.whatwg.org/multipage/form-control-infrastructure.html#multipart/form-data-encoding-algorithm
//
// Empirically, as of the time this comment was written, it is necessary
// to escape \ characters or else Chrome (and possibly other browsers) will
// interpet the unescaped \ as an escape.
func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

// CreateFormFile is a convenience wrapper around [Writer.CreatePart]. It creates
// a new form-data header with the provided field name and file name.
func (w *Writer) CreateFormFile(fieldname, filename string) (io.Writer, error) {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", FileContentDisposition(fieldname, filename))
	h.Set("Content-Type", "application/octet-stream")
	return w.CreatePart(h)
}

// CreateFormField calls [Writer.CreatePart] with a header using the
// given field name.
func (w *Writer) CreateFormField(fieldname string) (io.Writer, error) {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"`, escapeQuotes(fieldname)))
	return w.CreatePart(h)
}

// FileContentDisposition returns the value of a Content-Disposition header
// with the provided field name and file name.
func FileContentDisposition(fieldname, filename string) string {
	return fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
		escapeQuotes(fieldname), escapeQuotes(filename))
}

// WriteField calls [Writer.CreateFormField] and then writes the given value.
func (w *Writer) WriteField(fieldname, value string) error {
	p, err := w.CreateFormField(fieldname)
	if err != nil {
		return err
	}
	_, err = p.Write([]byte(value))
	return err
}

// Close finishes the multipart message and writes the trailing
// boundary end line to the output.
func (w *Writer) Close() error {
	if w.lastpart != nil {
		if err := w.lastpart.close(); err != nil {
			return err
		}
		w.lastpart = nil
	}
	_, err := fmt.Fprintf(w.w, "\r\n--%s--\r\n", w.boundary)
	return err
}

type part struct {
	mw     *Writer
	closed bool
	we     error // last error that occurred writing
}

func (p *part) close() error {
	p.closed = true
	return p.we
}

func (p *part) Write(d []byte) (n int, err error) {
	if p.closed {
		return 0, errors.New("multipart: can't write to finished part")
	}
	n, err = p.mw.w.Write(d)
	if err != nil {
		p.we = err
	}
	return
}

```

// === FILE: references/go/src/mime/quotedprintable/reader.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package quotedprintable implements quoted-printable encoding as specified by
// RFC 2045.
package quotedprintable

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

// Reader is a quoted-printable decoder.
type Reader struct {
	br   *bufio.Reader
	rerr error  // last read error
	line []byte // to be consumed before more of br
}

// NewReader returns a quoted-printable reader, decoding from r.
func NewReader(r io.Reader) *Reader {
	return &Reader{
		br: bufio.NewReader(r),
	}
}

func fromHex(b byte) (byte, error) {
	switch {
	case b >= '0' && b <= '9':
		return b - '0', nil
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10, nil
	// Accept badly encoded bytes.
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10, nil
	}
	return 0, fmt.Errorf("quotedprintable: invalid hex byte 0x%02x", b)
}

func readHexByte(v []byte) (b byte, err error) {
	if len(v) < 2 {
		return 0, io.ErrUnexpectedEOF
	}
	var hb, lb byte
	if hb, err = fromHex(v[0]); err != nil {
		return 0, err
	}
	if lb, err = fromHex(v[1]); err != nil {
		return 0, err
	}
	return hb<<4 | lb, nil
}

func isQPDiscardWhitespace(r rune) bool {
	switch r {
	case '\n', '\r', ' ', '\t':
		return true
	}
	return false
}

var (
	crlf       = []byte("\r\n")
	lf         = []byte("\n")
	softSuffix = []byte("=")
	lwspChar   = " \t"
)

// Read reads and decodes quoted-printable data from the underlying reader.
func (r *Reader) Read(p []byte) (n int, err error) {
	// Deviations from RFC 2045:
	// 1. in addition to "=\r\n", "=\n" is also treated as soft line break.
	// 2. it will pass through a '\r' or '\n' not preceded by '=', consistent
	//    with other broken QP encoders & decoders.
	// 3. it accepts soft line-break (=) at end of message (issue 15486); i.e.
	//    the final byte read from the underlying reader is allowed to be '=',
	//    and it will be silently ignored.
	// 4. it takes = as literal = if not followed by two hex digits
	//    but not at end of line (issue 13219).
	for len(p) > 0 {
		if len(r.line) == 0 {
			if r.rerr != nil {
				return n, r.rerr
			}
			r.line, r.rerr = r.br.ReadSlice('\n')

			// Does the line end in CRLF instead of just LF?
			hasLF := bytes.HasSuffix(r.line, lf)
			hasCR := bytes.HasSuffix(r.line, crlf)
			wholeLine := r.line
			r.line = bytes.TrimRightFunc(wholeLine, isQPDiscardWhitespace)
			if bytes.HasSuffix(r.line, softSuffix) {
				rightStripped := bytes.TrimLeft(wholeLine[len(r.line):], lwspChar)
				r.line = r.line[:len(r.line)-1]
				if !bytes.HasPrefix(rightStripped, lf) && !bytes.HasPrefix(rightStripped, crlf) &&
					!(len(rightStripped) == 0 && len(r.line) > 0 && r.rerr == io.EOF) {
					r.rerr = fmt.Errorf("quotedprintable: invalid bytes after =: %q", rightStripped)
				}
			} else if hasLF {
				if hasCR {
					r.line = append(r.line, '\r', '\n')
				} else {
					r.line = append(r.line, '\n')
				}
			}
			continue
		}
		b := r.line[0]

		switch {
		case b == '=':
			b, err = readHexByte(r.line[1:])
			if err != nil {
				if len(r.line) >= 2 && r.line[1] != '\r' && r.line[1] != '\n' {
					// Take the = as a literal =.
					b = '='
					break
				}
				return n, err
			}
			r.line = r.line[2:] // 2 of the 3; other 1 is done below
		case b == '\t' || b == '\r' || b == '\n':
			break
		case b >= 0x80:
			// As an extension to RFC 2045, we accept
			// values >= 0x80 without complaint. Issue 22597.
			break
		case b < ' ' || b > '~':
			return n, fmt.Errorf("quotedprintable: invalid unescaped byte 0x%02x in body", b)
		}
		p[0] = b
		p = p[1:]
		r.line = r.line[1:]
		n++
	}
	return n, nil
}

```

// === FILE: references/go/src/mime/quotedprintable/writer.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quotedprintable

import "io"

const lineMaxLen = 76

// A Writer is a quoted-printable writer that implements [io.WriteCloser].
type Writer struct {
	// Binary mode treats the writer's input as pure binary and processes end of
	// line bytes as binary data.
	Binary bool

	w    io.Writer
	i    int
	line [78]byte
	cr   bool
}

// NewWriter returns a new [Writer] that writes to w.
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

// Write encodes p using quoted-printable encoding and writes it to the
// underlying [io.Writer]. It limits line length to 76 characters. The encoded
// bytes are not necessarily flushed until the [Writer] is closed.
func (w *Writer) Write(p []byte) (n int, err error) {
	for i, b := range p {
		switch {
		// Simple writes are done in batch.
		case b >= '!' && b <= '~' && b != '=':
			continue
		case isWhitespace(b) || !w.Binary && (b == '\n' || b == '\r'):
			continue
		}

		if i > n {
			if err := w.write(p[n:i]); err != nil {
				return n, err
			}
			n = i
		}

		if err := w.encode(b); err != nil {
			return n, err
		}
		n++
	}

	if n == len(p) {
		return n, nil
	}

	if err := w.write(p[n:]); err != nil {
		return n, err
	}

	return len(p), nil
}

// Close closes the [Writer], flushing any unwritten data to the underlying
// [io.Writer], but does not close the underlying io.Writer.
func (w *Writer) Close() error {
	if err := w.checkLastByte(); err != nil {
		return err
	}

	return w.flush()
}

// write limits text encoded in quoted-printable to 76 characters per line.
func (w *Writer) write(p []byte) error {
	for _, b := range p {
		if b == '\n' || b == '\r' {
			// If the previous byte was \r, the CRLF has already been inserted.
			if w.cr && b == '\n' {
				w.cr = false
				continue
			}

			if b == '\r' {
				w.cr = true
			}

			if err := w.checkLastByte(); err != nil {
				return err
			}
			if err := w.insertCRLF(); err != nil {
				return err
			}
			continue
		}

		if w.i == lineMaxLen-1 {
			if err := w.insertSoftLineBreak(); err != nil {
				return err
			}
		}

		w.line[w.i] = b
		w.i++
		w.cr = false
	}

	return nil
}

func (w *Writer) encode(b byte) error {
	if lineMaxLen-1-w.i < 3 {
		if err := w.insertSoftLineBreak(); err != nil {
			return err
		}
	}

	w.line[w.i] = '='
	w.line[w.i+1] = upperhex[b>>4]
	w.line[w.i+2] = upperhex[b&0x0f]
	w.i += 3

	return nil
}

const upperhex = "0123456789ABCDEF"

// checkLastByte encodes the last buffered byte if it is a space or a tab.
func (w *Writer) checkLastByte() error {
	if w.i == 0 {
		return nil
	}

	b := w.line[w.i-1]
	if isWhitespace(b) {
		w.i--
		if err := w.encode(b); err != nil {
			return err
		}
	}

	return nil
}

func (w *Writer) insertSoftLineBreak() error {
	w.line[w.i] = '='
	w.i++

	return w.insertCRLF()
}

func (w *Writer) insertCRLF() error {
	w.line[w.i] = '\r'
	w.line[w.i+1] = '\n'
	w.i += 2

	return w.flush()
}

func (w *Writer) flush() error {
	if _, err := w.w.Write(w.line[:w.i]); err != nil {
		return err
	}

	w.i = 0
	return nil
}

func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t'
}

```

// === FILE: references/go/src/mime/type.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package mime implements parts of the MIME spec.
package mime

import (
	"fmt"
	"slices"
	"strings"
	"sync"
)

var (
	mimeTypes      sync.Map // map[string]string; ".Z" => "application/x-compress"
	mimeTypesLower sync.Map // map[string]string; ".z" => "application/x-compress"

	// extensions maps from MIME type to list of lowercase file
	// extensions: "image/jpeg" => [".jfif", ".jpg", ".jpeg", ".pjp", ".pjpeg"]
	extensionsMu sync.Mutex // Guards stores (but not loads) on extensions.
	extensions   sync.Map   // map[string][]string; slice values are append-only.
)

// setMimeTypes is used by initMime's non-test path, and by tests.
func setMimeTypes(lowerExt, mixExt map[string]string) {
	mimeTypes.Clear()
	mimeTypesLower.Clear()
	extensions.Clear()

	for k, v := range lowerExt {
		mimeTypesLower.Store(k, v)
	}
	for k, v := range mixExt {
		mimeTypes.Store(k, v)
	}

	extensionsMu.Lock()
	defer extensionsMu.Unlock()
	for k, v := range lowerExt {
		justType, _, err := ParseMediaType(v)
		if err != nil {
			panic(err)
		}
		var exts []string
		if ei, ok := extensions.Load(justType); ok {
			exts = ei.([]string)
		}
		extensions.Store(justType, append(exts, k))
	}
}

// A type is listed here if both Firefox and Chrome included them in their own
// lists.  In the case where they contradict they are deconflicted using IANA's
// listed media types https://www.iana.org/assignments/media-types/media-types.xhtml
//
// Chrome's MIME mappings to file extensions are defined at
// https://chromium.googlesource.com/chromium/src.git/+/refs/heads/main/net/base/mime_util.cc
//
// Firefox's MIME types can be found at
// https://github.com/mozilla-firefox/firefox/blob/main/netwerk/mime/nsMimeTypes.h
// and the mappings to file extensions at
// https://github.com/mozilla-firefox/firefox/blob/main/uriloader/exthandler/nsExternalHelperAppService.cpp
var builtinTypesLower = map[string]string{
	".ai":    "application/postscript",
	".apk":   "application/vnd.android.package-archive",
	".apng":  "image/apng",
	".avif":  "image/avif",
	".bin":   "application/octet-stream",
	".bmp":   "image/bmp",
	".com":   "application/octet-stream",
	".css":   "text/css; charset=utf-8",
	".csv":   "text/csv; charset=utf-8",
	".doc":   "application/msword",
	".docx":  "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".ehtml": "text/html; charset=utf-8",
	".eml":   "message/rfc822",
	".eps":   "application/postscript",
	".exe":   "application/octet-stream",
	".flac":  "audio/flac",
	".gif":   "image/gif",
	".gz":    "application/gzip",
	".htm":   "text/html; charset=utf-8",
	".html":  "text/html; charset=utf-8",
	".ico":   "image/vnd.microsoft.icon",
	".ics":   "text/calendar; charset=utf-8",
	".jfif":  "image/jpeg",
	".jpeg":  "image/jpeg",
	".jpg":   "image/jpeg",
	".js":    "text/javascript; charset=utf-8",
	".json":  "application/json",
	".m4a":   "audio/mp4",
	".mjs":   "text/javascript; charset=utf-8",
	".mp3":   "audio/mpeg",
	".mp4":   "video/mp4",
	".oga":   "audio/ogg",
	".ogg":   "audio/ogg",
	".ogv":   "video/ogg",
	".opus":  "audio/ogg",
	".pdf":   "application/pdf",
	".pjp":   "image/jpeg",
	".pjpeg": "image/jpeg",
	".png":   "image/png",
	".ppt":   "application/vnd.ms-powerpoint",
	".pptx":  "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	".ps":    "application/postscript",
	".rdf":   "application/rdf+xml",
	".rtf":   "application/rtf",
	".shtml": "text/html; charset=utf-8",
	".svg":   "image/svg+xml",
	".text":  "text/plain; charset=utf-8",
	".tif":   "image/tiff",
	".tiff":  "image/tiff",
	".txt":   "text/plain; charset=utf-8",
	".vtt":   "text/vtt; charset=utf-8",
	".wasm":  "application/wasm",
	".wav":   "audio/wav",
	".weba":  "audio/webm",
	".webm":  "video/webm",
	".webp":  "image/webp",
	".xbl":   "text/xml; charset=utf-8",
	".xbm":   "image/x-xbitmap",
	".xht":   "application/xhtml+xml",
	".xhtml": "application/xhtml+xml",
	".xls":   "application/vnd.ms-excel",
	".xlsx":  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".xml":   "text/xml; charset=utf-8",
	".xsl":   "text/xml; charset=utf-8",
	".zip":   "application/zip",
}

var once sync.Once // guards initMime

var testInitMime, osInitMime func()

func initMime() {
	if fn := testInitMime; fn != nil {
		fn()
	} else {
		setMimeTypes(builtinTypesLower, builtinTypesLower)
		osInitMime()
	}
}

// TypeByExtension returns the MIME type associated with the file extension ext.
// The extension ext should begin with a leading dot, as in ".html".
// When ext has no associated type, TypeByExtension returns "".
//
// Extensions are looked up first case-sensitively, then case-insensitively.
//
// The built-in table is small but on unix it is augmented by the local
// system's MIME-info database or mime.types file(s) if available under one or
// more of these names:
//
//	/usr/local/share/mime/globs2
//	/usr/share/mime/globs2
//	/etc/mime.types
//	/etc/apache2/mime.types
//	/etc/apache/mime.types
//	/etc/httpd/conf/mime.types
//
// On Windows, MIME types are extracted from the registry.
//
// Text types have the charset parameter set to "utf-8" by default.
func TypeByExtension(ext string) string {
	once.Do(initMime)

	// Case-sensitive lookup.
	if v, ok := mimeTypes.Load(ext); ok {
		return v.(string)
	}

	// Case-insensitive lookup.
	// Optimistically assume a short ASCII extension and be
	// allocation-free in that case.
	var buf [10]byte
	lower := buf[:0]
	const utf8RuneSelf = 0x80 // from utf8 package, but not importing it.
	for i := 0; i < len(ext); i++ {
		c := ext[i]
		if c >= utf8RuneSelf {
			// Slow path.
			si, _ := mimeTypesLower.Load(strings.ToLower(ext))
			s, _ := si.(string)
			return s
		}
		if 'A' <= c && c <= 'Z' {
			lower = append(lower, c+('a'-'A'))
		} else {
			lower = append(lower, c)
		}
	}
	si, _ := mimeTypesLower.Load(string(lower))
	s, _ := si.(string)
	return s
}

// ExtensionsByType returns the extensions known to be associated with the MIME
// type typ. The returned extensions will each begin with a leading dot, as in
// ".html". When typ has no associated extensions, ExtensionsByType returns an
// nil slice.
//
// The built-in table is small but on unix it is augmented by the local
// system's MIME-info database or mime.types file(s) if available under one or
// more of these names:
//
//	/usr/local/share/mime/globs2
//	/usr/share/mime/globs2
//	/etc/mime.types
//	/etc/apache2/mime.types
//	/etc/apache/mime.types
//	/etc/httpd/conf/mime.types
//
// On Windows, extensions are extracted from the registry.
func ExtensionsByType(typ string) ([]string, error) {
	justType, _, err := ParseMediaType(typ)
	if err != nil {
		return nil, err
	}

	once.Do(initMime)
	s, ok := extensions.Load(justType)
	if !ok {
		return nil, nil
	}
	ret := append([]string(nil), s.([]string)...)
	slices.Sort(ret)
	return ret, nil
}

// AddExtensionType sets the MIME type associated with
// the extension ext to typ. The extension should begin with
// a leading dot, as in ".html".
func AddExtensionType(ext, typ string) error {
	if !strings.HasPrefix(ext, ".") {
		return fmt.Errorf("mime: extension %q missing leading dot", ext)
	}
	once.Do(initMime)
	return setExtensionType(ext, typ)
}

func setExtensionType(extension, mimeType string) error {
	justType, param, err := ParseMediaType(mimeType)
	if err != nil {
		return err
	}
	if strings.HasPrefix(mimeType, "text/") && param["charset"] == "" {
		param["charset"] = "utf-8"
		mimeType = FormatMediaType(justType, param)
	}
	extLower := strings.ToLower(extension)

	mimeTypes.Store(extension, mimeType)
	mimeTypesLower.Store(extLower, mimeType)

	extensionsMu.Lock()
	defer extensionsMu.Unlock()
	var exts []string
	if ei, ok := extensions.Load(justType); ok {
		exts = ei.([]string)
	}
	for _, v := range exts {
		if v == extLower {
			return nil
		}
	}
	extensions.Store(justType, append(exts, extLower))
	return nil
}

```

// === FILE: references/go/src/mime/type_dragonfly.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mime

func init() {
	typeFiles = append(typeFiles, "/usr/local/etc/mime.types")
}

```

// === FILE: references/go/src/mime/type_freebsd.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mime

func init() {
	typeFiles = append(typeFiles, "/usr/local/etc/mime.types")
}

```

// === FILE: references/go/src/mime/type_openbsd.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mime

func init() {
	typeFiles = append(typeFiles, "/usr/share/misc/mime.types")
}

```

// === FILE: references/go/src/mime/type_plan9.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mime

import (
	"bufio"
	"os"
	"strings"
)

func init() {
	osInitMime = initMimePlan9
}

func initMimePlan9() {
	for _, filename := range typeFiles {
		loadMimeFile(filename)
	}
}

var typeFiles = []string{
	"/sys/lib/mimetype",
}

func initMimeForTests() map[string]string {
	typeFiles = []string{"testdata/test.types.plan9"}
	return map[string]string{
		".t1":  "application/test",
		".t2":  "text/test; charset=utf-8",
		".pNg": "image/png",
	}
}

func loadMimeFile(filename string) {
	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) <= 2 || fields[0][0] != '.' {
			continue
		}
		if fields[1] == "-" || fields[2] == "-" {
			continue
		}
		setExtensionType(fields[0], fields[1]+"/"+fields[2])
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
}

```

// === FILE: references/go/src/mime/type_unix.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix || (js && wasm) || wasip1

package mime

import (
	"bufio"
	"os"
	"strings"
)

func init() {
	osInitMime = initMimeUnix
}

// See https://specifications.freedesktop.org/shared-mime-info-spec/shared-mime-info-spec-0.21.html
// for the FreeDesktop Shared MIME-info Database specification.
var mimeGlobs = []string{
	"/usr/local/share/mime/globs2",
	"/usr/share/mime/globs2",
}

// Common locations for mime.types files on unix.
var typeFiles = []string{
	"/etc/mime.types",
	"/etc/apache2/mime.types",
	"/etc/apache/mime.types",
	"/etc/httpd/conf/mime.types",
}

func loadMimeGlobsFile(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		// Each line should be of format: weight:mimetype:glob[:morefields...]
		fields := strings.Split(scanner.Text(), ":")
		if len(fields) < 3 || len(fields[0]) < 1 || len(fields[2]) < 3 {
			continue
		} else if fields[0][0] == '#' || fields[2][0] != '*' || fields[2][1] != '.' {
			continue
		}

		extension := fields[2][1:]
		if strings.ContainsAny(extension, "?*[") {
			// Not a bare extension, but a glob. Ignore for now:
			// - we do not have an implementation for this glob
			//   syntax (translation to path/filepath.Match could
			//   be possible)
			// - support for globs with weight ordering would have
			//   performance impact to all lookups to support the
			//   rarely seen glob entries
			// - trying to match glob metacharacters literally is
			//   not useful
			continue
		}
		if _, ok := mimeTypes.Load(extension); ok {
			// We've already seen this extension.
			// The file is in weight order, so we keep
			// the first entry that we see.
			continue
		}

		setExtensionType(extension, fields[1])
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
	return nil
}

func loadMimeFile(filename string) {
	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) <= 1 || fields[0][0] == '#' {
			continue
		}
		mimeType := fields[0]
		for _, ext := range fields[1:] {
			if ext[0] == '#' {
				break
			}
			setExtensionType("."+ext, mimeType)
		}
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
}

func initMimeUnix() {
	for _, filename := range mimeGlobs {
		if err := loadMimeGlobsFile(filename); err == nil {
			return // Stop checking more files if mimetype database is found.
		}
	}

	// Fallback if no system-generated mimetype database exists.
	for _, filename := range typeFiles {
		loadMimeFile(filename)
	}
}

func initMimeForTests() map[string]string {
	mimeGlobs = []string{""}
	typeFiles = []string{"testdata/test.types"}
	return map[string]string{
		".T1":  "application/test",
		".t2":  "text/test; charset=utf-8",
		".png": "image/png",
	}
}

```

// === FILE: references/go/src/mime/type_windows.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mime

import (
	"internal/syscall/windows/registry"
)

func init() {
	osInitMime = initMimeWindows
}

func initMimeWindows() {
	names, err := registry.CLASSES_ROOT.ReadSubKeyNames()
	if err != nil {
		return
	}
	for _, name := range names {
		if len(name) < 2 || name[0] != '.' { // looking for extensions only
			continue
		}
		k, err := registry.OpenKey(registry.CLASSES_ROOT, name, registry.READ)
		if err != nil {
			continue
		}
		v, _, err := k.GetStringValue("Content Type")
		k.Close()
		if err != nil {
			continue
		}

		// There is a long-standing problem on Windows: the
		// registry sometimes records that the ".js" extension
		// should be "text/plain". See issue #32350. While
		// normally local configuration should override
		// defaults, this problem is common enough that we
		// handle it here by ignoring that registry setting.
		if name == ".js" && (v == "text/plain" || v == "text/plain; charset=utf-8") {
			continue
		}

		setExtensionType(name, v)
	}
}

func initMimeForTests() map[string]string {
	return map[string]string{
		".PnG": "image/png",
	}
}

```

