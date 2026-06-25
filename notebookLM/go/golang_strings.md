# Domain Architecture: strings

## Layout Topology
```text
strings/
├── builder.go
├── clone.go
├── compare.go
├── iter.go
├── reader.go
├── replace.go
├── search.go
└── strings.go
```

## Source Stream Aggregation

// === FILE: references!/go/src/strings/builder.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package strings

import (
	"internal/abi"
	"internal/bytealg"
	"unicode/utf8"
	"unsafe"
)

// A Builder is used to efficiently build a string using [Builder.Write] methods.
// It minimizes memory copying. The zero value is ready to use.
// Do not copy a non-zero Builder.
type Builder struct {
	addr *Builder // of receiver, to detect copies by value

	// External users should never get direct access to this buffer, since
	// the slice at some point will be converted to a string using unsafe, also
	// data between len(buf) and cap(buf) might be uninitialized.
	buf []byte
}

// copyCheck implements a dynamic check to prevent modification after
// copying a non-zero Builder, which would be unsafe (see #25907, #47276).
//
// We cannot add a noCopy field to Builder, to cause vet's copylocks
// check to report copying, because copylocks cannot reliably
// discriminate the zero and nonzero cases.
func (b *Builder) copyCheck() {
	if b.addr == nil {
		// This hack works around a failing of Go's escape analysis
		// that was causing b to escape and be heap allocated.
		// See issue 23382.
		// TODO: once issue 7921 is fixed, this should be reverted to
		// just "b.addr = b".
		b.addr = (*Builder)(abi.NoEscape(unsafe.Pointer(b)))
	} else if b.addr != b {
		panic("strings: illegal use of non-zero Builder copied by value")
	}
}

// String returns the accumulated string.
func (b *Builder) String() string {
	return unsafe.String(unsafe.SliceData(b.buf), len(b.buf))
}

// Len returns the number of accumulated bytes; b.Len() == len(b.String()).
func (b *Builder) Len() int { return len(b.buf) }

// Cap returns the capacity of the builder's underlying byte slice. It is the
// total space allocated for the string being built and includes any bytes
// already written.
func (b *Builder) Cap() int { return cap(b.buf) }

// Reset resets the [Builder] to be empty.
func (b *Builder) Reset() {
	b.addr = nil
	b.buf = nil
}

// grow copies the buffer to a new, larger buffer so that there are at least n
// bytes of capacity beyond len(b.buf).
func (b *Builder) grow(n int) {
	buf := bytealg.MakeNoZero(2*cap(b.buf) + n)[:len(b.buf)]
	copy(buf, b.buf)
	b.buf = buf
}

// Grow grows b's capacity, if necessary, to guarantee space for
// another n bytes. After Grow(n), at least n bytes can be written to b
// without another allocation. If n is negative, Grow panics.
func (b *Builder) Grow(n int) {
	b.copyCheck()
	if n < 0 {
		panic("strings.Builder.Grow: negative count")
	}
	if cap(b.buf)-len(b.buf) < n {
		b.grow(n)
	}
}

// Write appends the contents of p to b's buffer.
// Write always returns len(p), nil.
func (b *Builder) Write(p []byte) (int, error) {
	b.copyCheck()
	b.buf = append(b.buf, p...)
	return len(p), nil
}

// WriteByte appends the byte c to b's buffer.
// The returned error is always nil.
func (b *Builder) WriteByte(c byte) error {
	b.copyCheck()
	b.buf = append(b.buf, c)
	return nil
}

// WriteRune appends the UTF-8 encoding of Unicode code point r to b's buffer.
// It returns the number of bytes written and a nil error.
func (b *Builder) WriteRune(r rune) (int, error) {
	b.copyCheck()
	n := len(b.buf)
	b.buf = utf8.AppendRune(b.buf, r)
	return len(b.buf) - n, nil
}

// WriteString appends the contents of s to b's buffer.
// It returns the length of s and a nil error.
func (b *Builder) WriteString(s string) (int, error) {
	b.copyCheck()
	b.buf = append(b.buf, s...)
	return len(s), nil
}

```

// === FILE: references!/go/src/strings/clone.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package strings

import (
	"internal/stringslite"
)

// Clone returns a fresh copy of s.
// It guarantees to make a copy of s into a new allocation,
// which can be important when retaining only a small substring
// of a much larger string. Using Clone can help such programs
// use less memory. Of course, since using Clone makes a copy,
// overuse of Clone can make programs use more memory.
// Clone should typically be used only rarely, and only when
// profiling indicates that it is needed.
// For strings of length zero the string "" will be returned
// and no allocation is made.
func Clone(s string) string {
	return stringslite.Clone(s)
}

```

// === FILE: references!/go/src/strings/compare.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package strings

import "internal/bytealg"

// Compare returns an integer comparing two strings lexicographically.
// The result will be 0 if a == b, -1 if a < b, and +1 if a > b.
//
// Use Compare when you need to perform a three-way comparison (with
// [slices.SortFunc], for example). It is usually clearer and always faster
// to use the built-in string comparison operators ==, <, >, and so on.
func Compare(a, b string) int {
	return bytealg.CompareString(a, b)
}

```

// === FILE: references!/go/src/strings/iter.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package strings

import (
	"iter"
	"unicode"
	"unicode/utf8"
)

// Lines returns an iterator over the newline-terminated lines in the string s.
// The lines yielded by the iterator include their terminating newlines.
// If s is empty, the iterator yields no lines at all.
// If s does not end in a newline, the final yielded line will not end in a newline.
// It returns a single-use iterator.
func Lines(s string) iter.Seq[string] {
	return func(yield func(string) bool) {
		for len(s) > 0 {
			var line string
			if i := IndexByte(s, '\n'); i >= 0 {
				line, s = s[:i+1], s[i+1:]
			} else {
				line, s = s, ""
			}
			if !yield(line) {
				return
			}
		}
	}
}

// splitSeq is SplitSeq or SplitAfterSeq, configured by how many
// bytes of sep to include in the results (none or all).
func splitSeq(s, sep string, sepSave int) iter.Seq[string] {
	return func(yield func(string) bool) {
		if len(sep) == 0 {
			for len(s) > 0 {
				_, size := utf8.DecodeRuneInString(s)
				if !yield(s[:size]) {
					return
				}
				s = s[size:]
			}
			return
		}
		for {
			i := Index(s, sep)
			if i < 0 {
				break
			}
			frag := s[:i+sepSave]
			if !yield(frag) {
				return
			}
			s = s[i+len(sep):]
		}
		yield(s)
	}
}

// SplitSeq returns an iterator over all substrings of s separated by sep.
// The iterator yields the same strings that would be returned by [Split](s, sep),
// but without constructing the slice.
// It returns a single-use iterator.
func SplitSeq(s, sep string) iter.Seq[string] {
	return splitSeq(s, sep, 0)
}

// SplitAfterSeq returns an iterator over substrings of s split after each instance of sep.
// The iterator yields the same strings that would be returned by [SplitAfter](s, sep),
// but without constructing the slice.
// It returns a single-use iterator.
func SplitAfterSeq(s, sep string) iter.Seq[string] {
	return splitSeq(s, sep, len(sep))
}

// FieldsSeq returns an iterator over substrings of s split around runs of
// whitespace characters, as defined by [unicode.IsSpace].
// The iterator yields the same strings that would be returned by [Fields](s),
// but without constructing the slice.
func FieldsSeq(s string) iter.Seq[string] {
	return func(yield func(string) bool) {
		start := -1
		for i := 0; i < len(s); {
			size := 1
			r := rune(s[i])
			isSpace := asciiSpace[s[i]] != 0
			if r >= utf8.RuneSelf {
				r, size = utf8.DecodeRuneInString(s[i:])
				isSpace = unicode.IsSpace(r)
			}
			if isSpace {
				if start >= 0 {
					if !yield(s[start:i]) {
						return
					}
					start = -1
				}
			} else if start < 0 {
				start = i
			}
			i += size
		}
		if start >= 0 {
			yield(s[start:])
		}
	}
}

// FieldsFuncSeq returns an iterator over substrings of s split around runs of
// Unicode code points satisfying f(c).
// The iterator yields the same strings that would be returned by [FieldsFunc](s),
// but without constructing the slice.
func FieldsFuncSeq(s string, f func(rune) bool) iter.Seq[string] {
	return func(yield func(string) bool) {
		start := -1
		for i := 0; i < len(s); {
			r, size := utf8.DecodeRuneInString(s[i:])
			if f(r) {
				if start >= 0 {
					if !yield(s[start:i]) {
						return
					}
					start = -1
				}
			} else if start < 0 {
				start = i
			}
			i += size
		}
		if start >= 0 {
			yield(s[start:])
		}
	}
}

```

// === FILE: references!/go/src/strings/reader.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package strings

import (
	"errors"
	"io"
	"unicode/utf8"
)

// A Reader implements the [io.Reader], [io.ReaderAt], [io.ByteReader], [io.ByteScanner],
// [io.RuneReader], [io.RuneScanner], [io.Seeker], and [io.WriterTo] interfaces by reading
// from a string.
// The zero value for Reader operates like a Reader of an empty string.
type Reader struct {
	s        string
	i        int64 // current reading index
	prevRune int   // index of previous rune; or < 0
}

// Len returns the number of bytes of the unread portion of the
// string.
func (r *Reader) Len() int {
	if r.i >= int64(len(r.s)) {
		return 0
	}
	return int(int64(len(r.s)) - r.i)
}

// Size returns the original length of the underlying string.
// Size is the number of bytes available for reading via [Reader.ReadAt].
// The returned value is always the same and is not affected by calls
// to any other method.
func (r *Reader) Size() int64 { return int64(len(r.s)) }

// Read implements the [io.Reader] interface.
func (r *Reader) Read(b []byte) (n int, err error) {
	if r.i >= int64(len(r.s)) {
		return 0, io.EOF
	}
	r.prevRune = -1
	n = copy(b, r.s[r.i:])
	r.i += int64(n)
	return
}

// ReadAt implements the [io.ReaderAt] interface.
func (r *Reader) ReadAt(b []byte, off int64) (n int, err error) {
	// cannot modify state - see io.ReaderAt
	if off < 0 {
		return 0, errors.New("strings.Reader.ReadAt: negative offset")
	}
	if off >= int64(len(r.s)) {
		return 0, io.EOF
	}
	n = copy(b, r.s[off:])
	if n < len(b) {
		err = io.EOF
	}
	return
}

// ReadByte implements the [io.ByteReader] interface.
func (r *Reader) ReadByte() (byte, error) {
	r.prevRune = -1
	if r.i >= int64(len(r.s)) {
		return 0, io.EOF
	}
	b := r.s[r.i]
	r.i++
	return b, nil
}

// UnreadByte implements the [io.ByteScanner] interface.
func (r *Reader) UnreadByte() error {
	if r.i <= 0 {
		return errors.New("strings.Reader.UnreadByte: at beginning of string")
	}
	r.prevRune = -1
	r.i--
	return nil
}

// ReadRune implements the [io.RuneReader] interface.
func (r *Reader) ReadRune() (ch rune, size int, err error) {
	if r.i >= int64(len(r.s)) {
		r.prevRune = -1
		return 0, 0, io.EOF
	}
	r.prevRune = int(r.i)
	ch, size = utf8.DecodeRuneInString(r.s[r.i:])
	r.i += int64(size)
	return
}

// UnreadRune implements the [io.RuneScanner] interface.
func (r *Reader) UnreadRune() error {
	if r.i <= 0 {
		return errors.New("strings.Reader.UnreadRune: at beginning of string")
	}
	if r.prevRune < 0 {
		return errors.New("strings.Reader.UnreadRune: previous operation was not ReadRune")
	}
	r.i = int64(r.prevRune)
	r.prevRune = -1
	return nil
}

// Seek implements the [io.Seeker] interface.
func (r *Reader) Seek(offset int64, whence int) (int64, error) {
	r.prevRune = -1
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = r.i + offset
	case io.SeekEnd:
		abs = int64(len(r.s)) + offset
	default:
		return 0, errors.New("strings.Reader.Seek: invalid whence")
	}
	if abs < 0 {
		return 0, errors.New("strings.Reader.Seek: negative position")
	}
	r.i = abs
	return abs, nil
}

// WriteTo implements the [io.WriterTo] interface.
func (r *Reader) WriteTo(w io.Writer) (n int64, err error) {
	r.prevRune = -1
	if r.i >= int64(len(r.s)) {
		return 0, nil
	}
	s := r.s[r.i:]
	m, err := io.WriteString(w, s)
	if m > len(s) {
		panic("strings.Reader.WriteTo: invalid WriteString count")
	}
	r.i += int64(m)
	n = int64(m)
	if m != len(s) && err == nil {
		err = io.ErrShortWrite
	}
	return
}

// Reset resets the [Reader] to be reading from s.
func (r *Reader) Reset(s string) { *r = Reader{s, 0, -1} }

// NewReader returns a new [Reader] reading from s.
// It is similar to [bytes.NewBufferString] but more efficient and non-writable.
func NewReader(s string) *Reader { return &Reader{s, 0, -1} }

```

// === FILE: references!/go/src/strings/replace.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package strings

import (
	"io"
	"sync"
)

// Replacer replaces a list of strings with replacements.
// It is safe for concurrent use by multiple goroutines.
type Replacer struct {
	once   sync.Once // guards buildOnce method
	r      replacer
	oldnew []string
}

// replacer is the interface that a replacement algorithm needs to implement.
type replacer interface {
	Replace(s string) string
	WriteString(w io.Writer, s string) (n int, err error)
}

// NewReplacer returns a new [Replacer] from a list of old, new string
// pairs. Replacements are performed in the order they appear in the
// target string, without overlapping matches. The old string
// comparisons are done in argument order.
//
// NewReplacer panics if given an odd number of arguments.
func NewReplacer(oldnew ...string) *Replacer {
	if len(oldnew)%2 == 1 {
		panic("strings.NewReplacer: odd argument count")
	}
	return &Replacer{oldnew: append([]string(nil), oldnew...)}
}

func (r *Replacer) buildOnce() {
	r.r = r.build()
	r.oldnew = nil
}

func (b *Replacer) build() replacer {
	oldnew := b.oldnew
	if len(oldnew) == 2 && len(oldnew[0]) > 1 {
		return makeSingleStringReplacer(oldnew[0], oldnew[1])
	}

	allNewBytes := true
	for i := 0; i < len(oldnew); i += 2 {
		if len(oldnew[i]) != 1 {
			return makeGenericReplacer(oldnew)
		}
		if len(oldnew[i+1]) != 1 {
			allNewBytes = false
		}
	}

	if allNewBytes {
		r := byteReplacer{}
		for i := range r {
			r[i] = byte(i)
		}
		// The first occurrence of old->new map takes precedence
		// over the others with the same old string.
		for i := len(oldnew) - 2; i >= 0; i -= 2 {
			o := oldnew[i][0]
			n := oldnew[i+1][0]
			r[o] = n
		}
		return &r
	}

	r := byteStringReplacer{toReplace: make([]string, 0, len(oldnew)/2)}
	// The first occurrence of old->new map takes precedence
	// over the others with the same old string.
	for i := len(oldnew) - 2; i >= 0; i -= 2 {
		o := oldnew[i][0]
		n := oldnew[i+1]
		// To avoid counting repetitions multiple times.
		if r.replacements[o] == nil {
			// We need to use string([]byte{o}) instead of string(o),
			// to avoid utf8 encoding of o.
			// E. g. byte(150) produces string of length 2.
			r.toReplace = append(r.toReplace, string([]byte{o}))
		}
		r.replacements[o] = []byte(n)

	}
	return &r
}

// Replace returns a copy of s with all replacements performed.
func (r *Replacer) Replace(s string) string {
	r.once.Do(r.buildOnce)
	return r.r.Replace(s)
}

// WriteString writes s to w with all replacements performed.
func (r *Replacer) WriteString(w io.Writer, s string) (n int, err error) {
	r.once.Do(r.buildOnce)
	return r.r.WriteString(w, s)
}

// trieNode is a node in a lookup trie for prioritized key/value pairs. Keys
// and values may be empty. For example, the trie containing keys "ax", "ay",
// "bcbc", "x" and "xy" could have eight nodes:
//
//	n0  -
//	n1  a-
//	n2  .x+
//	n3  .y+
//	n4  b-
//	n5  .cbc+
//	n6  x+
//	n7  .y+
//
// n0 is the root node, and its children are n1, n4 and n6; n1's children are
// n2 and n3; n4's child is n5; n6's child is n7. Nodes n0, n1 and n4 (marked
// with a trailing "-") are partial keys, and nodes n2, n3, n5, n6 and n7
// (marked with a trailing "+") are complete keys.
type trieNode struct {
	// value is the value of the trie node's key/value pair. It is empty if
	// this node is not a complete key.
	value string
	// priority is the priority (higher is more important) of the trie node's
	// key/value pair; keys are not necessarily matched shortest- or longest-
	// first. Priority is positive if this node is a complete key, and zero
	// otherwise. In the example above, positive/zero priorities are marked
	// with a trailing "+" or "-".
	priority int

	// A trie node may have zero, one or more child nodes:
	//  * if the remaining fields are zero, there are no children.
	//  * if prefix and next are non-zero, there is one child in next.
	//  * if table is non-zero, it defines all the children.
	//
	// Prefixes are preferred over tables when there is one child, but the
	// root node always uses a table for lookup efficiency.

	// prefix is the difference in keys between this trie node and the next.
	// In the example above, node n4 has prefix "cbc" and n4's next node is n5.
	// Node n5 has no children and so has zero prefix, next and table fields.
	prefix string
	next   *trieNode

	// table is a lookup table indexed by the next byte in the key, after
	// remapping that byte through genericReplacer.mapping to create a dense
	// index. In the example above, the keys only use 'a', 'b', 'c', 'x' and
	// 'y', which remap to 0, 1, 2, 3 and 4. All other bytes remap to 5, and
	// genericReplacer.tableSize will be 5. Node n0's table will be
	// []*trieNode{ 0:n1, 1:n4, 3:n6 }, where the 0, 1 and 3 are the remapped
	// 'a', 'b' and 'x'.
	table []*trieNode
}

func (t *trieNode) add(key, val string, priority int, r *genericReplacer) {
	if key == "" {
		if t.priority == 0 {
			t.value = val
			t.priority = priority
		}
		return
	}

	if t.prefix != "" {
		// Need to split the prefix among multiple nodes.
		var n int // length of the longest common prefix
		for ; n < len(t.prefix) && n < len(key); n++ {
			if t.prefix[n] != key[n] {
				break
			}
		}
		if n == len(t.prefix) {
			t.next.add(key[n:], val, priority, r)
		} else if n == 0 {
			// First byte differs, start a new lookup table here. Looking up
			// what is currently t.prefix[0] will lead to prefixNode, and
			// looking up key[0] will lead to keyNode.
			var prefixNode *trieNode
			if len(t.prefix) == 1 {
				prefixNode = t.next
			} else {
				prefixNode = &trieNode{
					prefix: t.prefix[1:],
					next:   t.next,
				}
			}
			keyNode := new(trieNode)
			t.table = make([]*trieNode, r.tableSize)
			t.table[r.mapping[t.prefix[0]]] = prefixNode
			t.table[r.mapping[key[0]]] = keyNode
			t.prefix = ""
			t.next = nil
			keyNode.add(key[1:], val, priority, r)
		} else {
			// Insert new node after the common section of the prefix.
			next := &trieNode{
				prefix: t.prefix[n:],
				next:   t.next,
			}
			t.prefix = t.prefix[:n]
			t.next = next
			next.add(key[n:], val, priority, r)
		}
	} else if t.table != nil {
		// Insert into existing table.
		m := r.mapping[key[0]]
		if t.table[m] == nil {
			t.table[m] = new(trieNode)
		}
		t.table[m].add(key[1:], val, priority, r)
	} else {
		t.prefix = key
		t.next = new(trieNode)
		t.next.add("", val, priority, r)
	}
}

func (r *genericReplacer) lookup(s string, ignoreRoot bool) (val string, keylen int, found bool) {
	// Iterate down the trie to the end, and grab the value and keylen with
	// the highest priority.
	bestPriority := 0
	node := &r.root
	n := 0
	for node != nil {
		if node.priority > bestPriority && !(ignoreRoot && node == &r.root) {
			bestPriority = node.priority
			val = node.value
			keylen = n
			found = true
		}

		if s == "" {
			break
		}
		if node.table != nil {
			index := r.mapping[s[0]]
			if int(index) == r.tableSize {
				break
			}
			node = node.table[index]
			s = s[1:]
			n++
		} else if node.prefix != "" && HasPrefix(s, node.prefix) {
			n += len(node.prefix)
			s = s[len(node.prefix):]
			node = node.next
		} else {
			break
		}
	}
	return
}

// genericReplacer is the fully generic algorithm.
// It's used as a fallback when nothing faster can be used.
type genericReplacer struct {
	root trieNode
	// tableSize is the size of a trie node's lookup table. It is the number
	// of unique key bytes.
	tableSize int
	// mapping maps from key bytes to a dense index for trieNode.table.
	mapping [256]byte
}

func makeGenericReplacer(oldnew []string) *genericReplacer {
	r := new(genericReplacer)
	// Find each byte used, then assign them each an index.
	for i := 0; i < len(oldnew); i += 2 {
		key := oldnew[i]
		for j := 0; j < len(key); j++ {
			r.mapping[key[j]] = 1
		}
	}

	for _, b := range r.mapping {
		r.tableSize += int(b)
	}

	var index byte
	for i, b := range r.mapping {
		if b == 0 {
			r.mapping[i] = byte(r.tableSize)
		} else {
			r.mapping[i] = index
			index++
		}
	}
	// Ensure root node uses a lookup table (for performance).
	r.root.table = make([]*trieNode, r.tableSize)

	for i := 0; i < len(oldnew); i += 2 {
		r.root.add(oldnew[i], oldnew[i+1], len(oldnew)-i, r)
	}
	return r
}

type appendSliceWriter []byte

// Write writes to the buffer to satisfy [io.Writer].
func (w *appendSliceWriter) Write(p []byte) (int, error) {
	*w = append(*w, p...)
	return len(p), nil
}

// WriteString writes to the buffer without string->[]byte->string allocations.
func (w *appendSliceWriter) WriteString(s string) (int, error) {
	*w = append(*w, s...)
	return len(s), nil
}

type stringWriter struct {
	w io.Writer
}

func (w stringWriter) WriteString(s string) (int, error) {
	return w.w.Write([]byte(s))
}

func getStringWriter(w io.Writer) io.StringWriter {
	sw, ok := w.(io.StringWriter)
	if !ok {
		sw = stringWriter{w}
	}
	return sw
}

func (r *genericReplacer) Replace(s string) string {
	buf := make(appendSliceWriter, 0, len(s))
	r.WriteString(&buf, s)
	return string(buf)
}

func (r *genericReplacer) WriteString(w io.Writer, s string) (n int, err error) {
	sw := getStringWriter(w)
	var last, wn int
	var prevMatchEmpty bool
	for i := 0; i <= len(s); {
		// Fast path: s[i] is not a prefix of any pattern.
		if i != len(s) && r.root.priority == 0 {
			index := int(r.mapping[s[i]])
			if index == r.tableSize || r.root.table[index] == nil {
				i++
				continue
			}
		}

		// Ignore the empty match iff the previous loop found the empty match.
		val, keylen, match := r.lookup(s[i:], prevMatchEmpty)
		prevMatchEmpty = match && keylen == 0
		if match {
			wn, err = sw.WriteString(s[last:i])
			n += wn
			if err != nil {
				return
			}
			wn, err = sw.WriteString(val)
			n += wn
			if err != nil {
				return
			}
			i += keylen
			last = i
			continue
		}
		i++
	}
	if last != len(s) {
		wn, err = sw.WriteString(s[last:])
		n += wn
	}
	return
}

// singleStringReplacer is the implementation that's used when there is only
// one string to replace (and that string has more than one byte).
type singleStringReplacer struct {
	finder *stringFinder
	// value is the new string that replaces that pattern when it's found.
	value string
}

func makeSingleStringReplacer(pattern string, value string) *singleStringReplacer {
	return &singleStringReplacer{finder: makeStringFinder(pattern), value: value}
}

func (r *singleStringReplacer) Replace(s string) string {
	var buf Builder
	i, matched := 0, false
	for {
		match := r.finder.next(s[i:])
		if match == -1 {
			break
		}
		matched = true
		buf.Grow(match + len(r.value))
		buf.WriteString(s[i : i+match])
		buf.WriteString(r.value)
		i += match + len(r.finder.pattern)
	}
	if !matched {
		return s
	}
	buf.WriteString(s[i:])
	return buf.String()
}

func (r *singleStringReplacer) WriteString(w io.Writer, s string) (n int, err error) {
	sw := getStringWriter(w)
	var i, wn int
	for {
		match := r.finder.next(s[i:])
		if match == -1 {
			break
		}
		wn, err = sw.WriteString(s[i : i+match])
		n += wn
		if err != nil {
			return
		}
		wn, err = sw.WriteString(r.value)
		n += wn
		if err != nil {
			return
		}
		i += match + len(r.finder.pattern)
	}
	wn, err = sw.WriteString(s[i:])
	n += wn
	return
}

// byteReplacer is the implementation that's used when all the "old"
// and "new" values are single ASCII bytes.
// The array contains replacement bytes indexed by old byte.
type byteReplacer [256]byte

func (r *byteReplacer) Replace(s string) string {
	var buf []byte // lazily allocated
	for i := 0; i < len(s); i++ {
		b := s[i]
		if r[b] != b {
			if buf == nil {
				buf = []byte(s)
			}
			buf[i] = r[b]
		}
	}
	if buf == nil {
		return s
	}
	return string(buf)
}

func (r *byteReplacer) WriteString(w io.Writer, s string) (n int, err error) {
	sw := getStringWriter(w)
	last := 0
	for i := 0; i < len(s); i++ {
		b := s[i]
		if r[b] == b {
			continue
		}
		if last != i {
			wn, err := sw.WriteString(s[last:i])
			n += wn
			if err != nil {
				return n, err
			}
		}
		last = i + 1
		nw, err := w.Write(r[b : int(b)+1])
		n += nw
		if err != nil {
			return n, err
		}
	}
	if last != len(s) {
		nw, err := sw.WriteString(s[last:])
		n += nw
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

// byteStringReplacer is the implementation that's used when all the
// "old" values are single ASCII bytes but the "new" values vary in size.
type byteStringReplacer struct {
	// replacements contains replacement byte slices indexed by old byte.
	// A nil []byte means that the old byte should not be replaced.
	replacements [256][]byte
	// toReplace keeps a list of bytes to replace. Depending on length of toReplace
	// and length of target string it may be faster to use Count, or a plain loop.
	// We store single byte as a string, because Count takes a string.
	toReplace []string
}

// countCutOff controls the ratio of a string length to a number of replacements
// at which (*byteStringReplacer).Replace switches algorithms.
// For strings with higher ration of length to replacements than that value,
// we call Count, for each replacement from toReplace.
// For strings, with a lower ratio we use simple loop, because of Count overhead.
// countCutOff is an empirically determined overhead multiplier.
// TODO(tocarip) revisit once we have register-based abi/mid-stack inlining.
const countCutOff = 8

func (r *byteStringReplacer) Replace(s string) string {
	newSize := len(s)
	anyChanges := false
	// Is it faster to use Count?
	if len(r.toReplace)*countCutOff <= len(s) {
		for _, x := range r.toReplace {
			if c := Count(s, x); c != 0 {
				// The -1 is because we are replacing 1 byte with len(replacements[b]) bytes.
				newSize += c * (len(r.replacements[x[0]]) - 1)
				anyChanges = true
			}

		}
	} else {
		for i := 0; i < len(s); i++ {
			b := s[i]
			if r.replacements[b] != nil {
				// See above for explanation of -1
				newSize += len(r.replacements[b]) - 1
				anyChanges = true
			}
		}
	}
	if !anyChanges {
		return s
	}
	buf := make([]byte, newSize)
	j := 0
	for i := 0; i < len(s); i++ {
		b := s[i]
		if r.replacements[b] != nil {
			j += copy(buf[j:], r.replacements[b])
		} else {
			buf[j] = b
			j++
		}
	}
	return string(buf)
}

func (r *byteStringReplacer) WriteString(w io.Writer, s string) (n int, err error) {
	sw := getStringWriter(w)
	last := 0
	for i := 0; i < len(s); i++ {
		b := s[i]
		if r.replacements[b] == nil {
			continue
		}
		if last != i {
			nw, err := sw.WriteString(s[last:i])
			n += nw
			if err != nil {
				return n, err
			}
		}
		last = i + 1
		nw, err := w.Write(r.replacements[b])
		n += nw
		if err != nil {
			return n, err
		}
	}
	if last != len(s) {
		var nw int
		nw, err = sw.WriteString(s[last:])
		n += nw
	}
	return
}

```

// === FILE: references!/go/src/strings/search.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package strings

// stringFinder efficiently finds strings in a source text. It's implemented
// using the Boyer-Moore string search algorithm:
// https://en.wikipedia.org/wiki/Boyer-Moore_string_search_algorithm
// https://www.cs.utexas.edu/~moore/publications/fstrpos.pdf (note: this aged
// document uses 1-based indexing)
type stringFinder struct {
	// pattern is the string that we are searching for in the text.
	pattern string

	// badCharSkip[b] contains the distance between the last byte of pattern
	// and the rightmost occurrence of b in pattern. If b is not in pattern,
	// badCharSkip[b] is len(pattern).
	//
	// Whenever a mismatch is found with byte b in the text, we can safely
	// shift the matching frame at least badCharSkip[b] until the next time
	// the matching char could be in alignment.
	badCharSkip [256]int

	// goodSuffixSkip[i] defines how far we can shift the matching frame given
	// that the suffix pattern[i+1:] matches, but the byte pattern[i] does
	// not. There are two cases to consider:
	//
	// 1. The matched suffix occurs elsewhere in pattern (with a different
	// byte preceding it that we might possibly match). In this case, we can
	// shift the matching frame to align with the next suffix chunk. For
	// example, the pattern "mississi" has the suffix "issi" next occurring
	// (in right-to-left order) at index 1, so goodSuffixSkip[3] ==
	// shift+len(suffix) == 3+4 == 7.
	//
	// 2. If the matched suffix does not occur elsewhere in pattern, then the
	// matching frame may share part of its prefix with the end of the
	// matching suffix. In this case, goodSuffixSkip[i] will contain how far
	// to shift the frame to align this portion of the prefix to the
	// suffix. For example, in the pattern "abcxxxabc", when the first
	// mismatch from the back is found to be in position 3, the matching
	// suffix "xxabc" is not found elsewhere in the pattern. However, its
	// rightmost "abc" (at position 6) is a prefix of the whole pattern, so
	// goodSuffixSkip[3] == shift+len(suffix) == 6+5 == 11.
	goodSuffixSkip []int
}

func makeStringFinder(pattern string) *stringFinder {
	f := &stringFinder{
		pattern:        pattern,
		goodSuffixSkip: make([]int, len(pattern)),
	}
	// last is the index of the last character in the pattern.
	last := len(pattern) - 1

	// Build bad character table.
	// Bytes not in the pattern can skip one pattern's length.
	for i := range f.badCharSkip {
		f.badCharSkip[i] = len(pattern)
	}
	// The loop condition is < instead of <= so that the last byte does not
	// have a zero distance to itself. Finding this byte out of place implies
	// that it is not in the last position.
	for i := 0; i < last; i++ {
		f.badCharSkip[pattern[i]] = last - i
	}

	// Build good suffix table.
	// First pass: set each value to the next index which starts a prefix of
	// pattern.
	lastPrefix := last
	for i := last; i >= 0; i-- {
		if HasPrefix(pattern, pattern[i+1:]) {
			lastPrefix = i + 1
		}
		// lastPrefix is the shift, and (last-i) is len(suffix).
		f.goodSuffixSkip[i] = lastPrefix + last - i
	}
	// Second pass: find repeats of pattern's suffix starting from the front.
	for i := 0; i < last; i++ {
		lenSuffix := longestCommonSuffix(pattern, pattern[1:i+1])
		if pattern[i-lenSuffix] != pattern[last-lenSuffix] {
			// (last-i) is the shift, and lenSuffix is len(suffix).
			f.goodSuffixSkip[last-lenSuffix] = lenSuffix + last - i
		}
	}

	return f
}

func longestCommonSuffix(a, b string) (i int) {
	for ; i < len(a) && i < len(b); i++ {
		if a[len(a)-1-i] != b[len(b)-1-i] {
			break
		}
	}
	return
}

// next returns the index in text of the first occurrence of the pattern. If
// the pattern is not found, it returns -1.
func (f *stringFinder) next(text string) int {
	i := len(f.pattern) - 1
	for i < len(text) {
		// Compare backwards from the end until the first unmatching character.
		j := len(f.pattern) - 1
		for j >= 0 && text[i] == f.pattern[j] {
			i--
			j--
		}
		if j < 0 {
			return i + 1 // match
		}
		i += max(f.badCharSkip[text[i]], f.goodSuffixSkip[j])
	}
	return -1
}

```

// === FILE: references!/go/src/strings/strings.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package strings implements simple functions to manipulate UTF-8 encoded strings.
//
// For information about UTF-8 strings in Go, see https://blog.golang.org/strings.
package strings

import (
	"internal/bytealg"
	"internal/stringslite"
	"math/bits"
	"unicode"
	"unicode/utf8"
)

const maxInt = int(^uint(0) >> 1)

// explode splits s into a slice of UTF-8 strings,
// one string per Unicode character up to a maximum of n (n < 0 means no limit).
// Invalid UTF-8 bytes are sliced individually.
func explode(s string, n int) []string {
	l := utf8.RuneCountInString(s)
	if n < 0 || n > l {
		n = l
	}
	a := make([]string, n)
	for i := 0; i < n-1; i++ {
		_, size := utf8.DecodeRuneInString(s)
		a[i] = s[:size]
		s = s[size:]
	}
	if n > 0 {
		a[n-1] = s
	}
	return a
}

// Count counts the number of non-overlapping instances of substr in s.
// If substr is an empty string, Count returns 1 + the number of Unicode code points in s.
func Count(s, substr string) int {
	// special case
	if len(substr) == 0 {
		return utf8.RuneCountInString(s) + 1
	}
	if len(substr) == 1 {
		return bytealg.CountString(s, substr[0])
	}
	n := 0
	for {
		i := Index(s, substr)
		if i == -1 {
			return n
		}
		n++
		s = s[i+len(substr):]
	}
}

// Contains reports whether substr is within s.
func Contains(s, substr string) bool {
	return Index(s, substr) >= 0
}

// ContainsAny reports whether any Unicode code points in chars are within s.
func ContainsAny(s, chars string) bool {
	return IndexAny(s, chars) >= 0
}

// ContainsRune reports whether the Unicode code point r is within s.
func ContainsRune(s string, r rune) bool {
	return IndexRune(s, r) >= 0
}

// ContainsFunc reports whether any Unicode code points r within s satisfy f(r).
// It stops as soon as a call to f returns true.
func ContainsFunc(s string, f func(rune) bool) bool {
	return IndexFunc(s, f) >= 0
}

// LastIndex returns the index of the last instance of substr in s, or -1 if substr is not present in s.
func LastIndex(s, substr string) int {
	n := len(substr)
	switch {
	case n == 0:
		return len(s)
	case n == 1:
		return bytealg.LastIndexByteString(s, substr[0])
	case n == len(s):
		if substr == s {
			return 0
		}
		return -1
	case n > len(s):
		return -1
	}
	return bytealg.LastIndexRabinKarp(s, substr)
}

// IndexByte returns the index of the first instance of c in s, or -1 if c is not present in s.
func IndexByte(s string, c byte) int {
	return stringslite.IndexByte(s, c)
}

// IndexRune returns the index of the first instance of the Unicode code point
// r, or -1 if rune is not present in s.
// If r is [utf8.RuneError], it returns the first instance of any
// invalid UTF-8 byte sequence.
func IndexRune(s string, r rune) int {
	const haveFastIndex = bytealg.MaxBruteForce > 0
	switch {
	case 0 <= r && r < utf8.RuneSelf:
		return IndexByte(s, byte(r))
	case r == utf8.RuneError:
		for i, r := range s {
			if r == utf8.RuneError {
				return i
			}
		}
		return -1
	case !utf8.ValidRune(r):
		return -1
	default:
		// Search for rune r using the last byte of its UTF-8 encoded form.
		// The distribution of the last byte is more uniform compared to the
		// first byte which has a 78% chance of being [240, 243, 244].
		rs := string(r)
		last := len(rs) - 1
		i := last
		fails := 0
		for i < len(s) {
			if s[i] != rs[last] {
				o := IndexByte(s[i+1:], rs[last])
				if o < 0 {
					return -1
				}
				i += o + 1
			}
			// Step backwards comparing bytes.
			for j := 1; j < len(rs); j++ {
				if s[i-j] != rs[last-j] {
					goto next
				}
			}
			return i - last
		next:
			fails++
			i++
			if (haveFastIndex && fails > bytealg.Cutover(i)) && i < len(s) ||
				(!haveFastIndex && fails >= 4+i>>4 && i < len(s)) {
				goto fallback
			}
		}
		return -1

	fallback:
		// see comment in ../bytes/bytes.go
		if haveFastIndex {
			if j := bytealg.IndexString(s[i-last:], string(r)); j >= 0 {
				return i + j - last
			}
		} else {
			c0 := rs[last]
			c1 := rs[last-1]
		loop:
			for ; i < len(s); i++ {
				if s[i] == c0 && s[i-1] == c1 {
					for k := 2; k < len(rs); k++ {
						if s[i-k] != rs[last-k] {
							continue loop
						}
					}
					return i - last
				}
			}
		}
		return -1
	}
}

// IndexAny returns the index of the first instance of any Unicode code point
// from chars in s, or -1 if no Unicode code point from chars is present in s.
func IndexAny(s, chars string) int {
	if chars == "" {
		// Avoid scanning all of s.
		return -1
	}
	if len(chars) == 1 {
		// Avoid scanning all of s.
		r := rune(chars[0])
		if r >= utf8.RuneSelf {
			r = utf8.RuneError
		}
		return IndexRune(s, r)
	}
	if shouldUseASCIISet(len(s)) {
		if as, isASCII := makeASCIISet(chars); isASCII {
			for i := 0; i < len(s); i++ {
				if as.contains(s[i]) {
					return i
				}
			}
			return -1
		}
	}
	for i, c := range s {
		if IndexRune(chars, c) >= 0 {
			return i
		}
	}
	return -1
}

// LastIndexAny returns the index of the last instance of any Unicode code
// point from chars in s, or -1 if no Unicode code point from chars is
// present in s.
func LastIndexAny(s, chars string) int {
	if chars == "" {
		// Avoid scanning all of s.
		return -1
	}
	if len(s) == 1 {
		rc := rune(s[0])
		if rc >= utf8.RuneSelf {
			rc = utf8.RuneError
		}
		if IndexRune(chars, rc) >= 0 {
			return 0
		}
		return -1
	}
	if shouldUseASCIISet(len(s)) {
		if as, isASCII := makeASCIISet(chars); isASCII {
			for i := len(s) - 1; i >= 0; i-- {
				if as.contains(s[i]) {
					return i
				}
			}
			return -1
		}
	}
	if len(chars) == 1 {
		rc := rune(chars[0])
		if rc >= utf8.RuneSelf {
			rc = utf8.RuneError
		}
		for i := len(s); i > 0; {
			r, size := utf8.DecodeLastRuneInString(s[:i])
			i -= size
			if rc == r {
				return i
			}
		}
		return -1
	}
	for i := len(s); i > 0; {
		r, size := utf8.DecodeLastRuneInString(s[:i])
		i -= size
		if IndexRune(chars, r) >= 0 {
			return i
		}
	}
	return -1
}

// LastIndexByte returns the index of the last instance of c in s, or -1 if c is not present in s.
func LastIndexByte(s string, c byte) int {
	return bytealg.LastIndexByteString(s, c)
}

// Generic split: splits after each instance of sep,
// including sepSave bytes of sep in the subarrays.
func genSplit(s, sep string, sepSave, n int) []string {
	if n == 0 {
		return nil
	}
	if sep == "" {
		return explode(s, n)
	}
	if n < 0 {
		n = Count(s, sep) + 1
	}

	n = min(n, len(s)+1)
	a := make([]string, n)
	n--
	i := 0
	for i < n {
		m := Index(s, sep)
		if m < 0 {
			break
		}
		a[i] = s[:m+sepSave]
		s = s[m+len(sep):]
		i++
	}
	a[i] = s
	return a[:i+1]
}

// SplitN slices s into substrings separated by sep and returns a slice of
// the substrings between those separators.
//
// The count determines the number of substrings to return:
//   - n > 0: at most n substrings; the last substring will be the unsplit remainder;
//   - n == 0: the result is nil (zero substrings);
//   - n < 0: all substrings.
//
// Edge cases for s and sep (for example, empty strings) are handled
// as described in the documentation for [Split].
//
// To split around the first instance of a separator, see [Cut].
func SplitN(s, sep string, n int) []string { return genSplit(s, sep, 0, n) }

// SplitAfterN slices s into substrings after each instance of sep and
// returns a slice of those substrings.
//
// The count determines the number of substrings to return:
//   - n > 0: at most n substrings; the last substring will be the unsplit remainder;
//   - n == 0: the result is nil (zero substrings);
//   - n < 0: all substrings.
//
// Edge cases for s and sep (for example, empty strings) are handled
// as described in the documentation for [SplitAfter].
func SplitAfterN(s, sep string, n int) []string {
	return genSplit(s, sep, len(sep), n)
}

// Split slices s into all substrings separated by sep and returns a slice of
// the substrings between those separators.
//
// If s does not contain sep and sep is not empty, Split returns a
// slice of length 1 whose only element is s.
//
// If sep is empty, Split splits after each UTF-8 sequence. If both s
// and sep are empty, Split returns an empty slice.
//
// It is equivalent to [SplitN] with a count of -1.
//
// To split around the first instance of a separator, see [Cut].
func Split(s, sep string) []string { return genSplit(s, sep, 0, -1) }

// SplitAfter slices s into all substrings after each instance of sep and
// returns a slice of those substrings.
//
// If s does not contain sep and sep is not empty, SplitAfter returns
// a slice of length 1 whose only element is s.
//
// If sep is empty, SplitAfter splits after each UTF-8 sequence. If
// both s and sep are empty, SplitAfter returns an empty slice.
//
// It is equivalent to [SplitAfterN] with a count of -1.
func SplitAfter(s, sep string) []string {
	return genSplit(s, sep, len(sep), -1)
}

var asciiSpace = [256]uint8{'\t': 1, '\n': 1, '\v': 1, '\f': 1, '\r': 1, ' ': 1}

// Fields splits the string s around each instance of one or more consecutive white space
// characters, as defined by [unicode.IsSpace], returning a slice of substrings of s or an
// empty slice if s contains only white space. Every element of the returned slice is
// non-empty. Unlike [Split], leading and trailing runs of white space characters
// are discarded.
func Fields(s string) []string {
	// First count the fields.
	// This is an exact count if s is ASCII, otherwise it is an approximation.
	n := 0
	wasSpace := 1
	// setBits is used to track which bits are set in the bytes of s.
	setBits := uint8(0)
	for i := 0; i < len(s); i++ {
		r := s[i]
		setBits |= r
		isSpace := int(asciiSpace[r])
		n += wasSpace & ^isSpace
		wasSpace = isSpace
	}

	if setBits >= utf8.RuneSelf {
		// Some runes in the input string are not ASCII.
		return FieldsFunc(s, unicode.IsSpace)
	}
	// ASCII fast path
	a := make([]string, n)
	na := 0
	fieldStart := 0
	i := 0
	// Skip spaces in the front of the input.
	for i < len(s) && asciiSpace[s[i]] != 0 {
		i++
	}
	fieldStart = i
	for i < len(s) {
		if asciiSpace[s[i]] == 0 {
			i++
			continue
		}
		a[na] = s[fieldStart:i]
		na++
		i++
		// Skip spaces in between fields.
		for i < len(s) && asciiSpace[s[i]] != 0 {
			i++
		}
		fieldStart = i
	}
	if fieldStart < len(s) { // Last field might end at EOF.
		a[na] = s[fieldStart:]
	}
	return a
}

// FieldsFunc splits the string s at each run of Unicode code points c satisfying f(c)
// and returns an array of slices of s. If all code points in s satisfy f(c) or the
// string is empty, an empty slice is returned. Every element of the returned slice is
// non-empty. Unlike [Split], leading and trailing runs of code points satisfying f(c)
// are discarded.
//
// FieldsFunc makes no guarantees about the order in which it calls f(c)
// and assumes that f always returns the same value for a given c.
func FieldsFunc(s string, f func(rune) bool) []string {
	// A span is used to record a slice of s of the form s[start:end].
	// The start index is inclusive and the end index is exclusive.
	type span struct {
		start int
		end   int
	}
	spans := make([]span, 0, 32)

	// Find the field start and end indices.
	// Doing this in a separate pass (rather than slicing the string s
	// and collecting the result substrings right away) is significantly
	// more efficient, possibly due to cache effects.
	start := -1 // valid span start if >= 0
	for end, rune := range s {
		if f(rune) {
			if start >= 0 {
				spans = append(spans, span{start, end})
				// Set start to a negative value.
				// Note: using -1 here consistently and reproducibly
				// slows down this code by a several percent on amd64.
				start = ^start
			}
		} else {
			if start < 0 {
				start = end
			}
		}
	}

	// Last field might end at EOF.
	if start >= 0 {
		spans = append(spans, span{start, len(s)})
	}

	// Create strings from recorded field indices.
	a := make([]string, len(spans))
	for i, span := range spans {
		a[i] = s[span.start:span.end]
	}

	return a
}

// Join concatenates the elements of its first argument to create a single string. The separator
// string sep is placed between elements in the resulting string.
func Join(elems []string, sep string) string {
	switch len(elems) {
	case 0:
		return ""
	case 1:
		return elems[0]
	}

	var n int
	if len(sep) > 0 {
		if len(sep) >= maxInt/(len(elems)-1) {
			panic("strings: Join output length overflow")
		}
		n += len(sep) * (len(elems) - 1)
	}
	for _, elem := range elems {
		if len(elem) > maxInt-n {
			panic("strings: Join output length overflow")
		}
		n += len(elem)
	}

	var b Builder
	b.Grow(n)
	b.WriteString(elems[0])
	for _, s := range elems[1:] {
		b.WriteString(sep)
		b.WriteString(s)
	}
	return b.String()
}

// HasPrefix reports whether the string s begins with prefix.
func HasPrefix(s, prefix string) bool {
	return stringslite.HasPrefix(s, prefix)
}

// HasSuffix reports whether the string s ends with suffix.
func HasSuffix(s, suffix string) bool {
	return stringslite.HasSuffix(s, suffix)
}

// Map returns a copy of the string s with all its characters modified
// according to the mapping function. If mapping returns a negative value, the character is
// dropped from the string with no replacement.
func Map(mapping func(rune) rune, s string) string {
	// In the worst case, the string can grow when mapped, making
	// things unpleasant. But it's so rare we barge in assuming it's
	// fine. It could also shrink but that falls out naturally.

	// The output buffer b is initialized on demand, the first
	// time a character differs.
	var b Builder

	for i, c := range s {
		r := mapping(c)
		if r == c && c != utf8.RuneError {
			continue
		}

		var width int
		if c == utf8.RuneError {
			c, width = utf8.DecodeRuneInString(s[i:])
			if width != 1 && r == c {
				continue
			}
		} else {
			width = utf8.RuneLen(c)
		}

		b.Grow(len(s) + utf8.UTFMax)
		b.WriteString(s[:i])
		if r >= 0 {
			b.WriteRune(r)
		}

		s = s[i+width:]
		break
	}

	// Fast path for unchanged input
	if b.Cap() == 0 { // didn't call b.Grow above
		return s
	}

	for _, c := range s {
		r := mapping(c)

		if r >= 0 {
			// common case
			// Due to inlining, it is more performant to determine if WriteByte should be
			// invoked rather than always call WriteRune
			if r < utf8.RuneSelf {
				b.WriteByte(byte(r))
			} else {
				// r is not an ASCII rune.
				b.WriteRune(r)
			}
		}
	}

	return b.String()
}

// According to static analysis, spaces, dashes, zeros, equals, and tabs
// are the most commonly repeated string literal,
// often used for display on fixed-width terminal windows.
// Pre-declare constants for these for O(1) repetition in the common-case.
const (
	repeatedSpaces = "" +
		"                                                                " +
		"                                                                "
	repeatedDashes = "" +
		"----------------------------------------------------------------" +
		"----------------------------------------------------------------"
	repeatedZeroes = "" +
		"0000000000000000000000000000000000000000000000000000000000000000"
	repeatedEquals = "" +
		"================================================================" +
		"================================================================"
	repeatedTabs = "" +
		"\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t" +
		"\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t"
)

// Repeat returns a new string consisting of count copies of the string s.
//
// It panics if count is negative or if the result of (len(s) * count)
// overflows.
func Repeat(s string, count int) string {
	switch count {
	case 0:
		return ""
	case 1:
		return s
	}

	// Since we cannot return an error on overflow,
	// we should panic if the repeat will generate an overflow.
	// See golang.org/issue/16237.
	if count < 0 {
		panic("strings: negative Repeat count")
	}
	hi, lo := bits.Mul(uint(len(s)), uint(count))
	if hi > 0 || lo > uint(maxInt) {
		panic("strings: Repeat output length overflow")
	}
	n := int(lo) // lo = len(s) * count

	if len(s) == 0 {
		return ""
	}

	// Optimize for commonly repeated strings of relatively short length.
	switch s[0] {
	case ' ', '-', '0', '=', '\t':
		switch {
		case n <= len(repeatedSpaces) && HasPrefix(repeatedSpaces, s):
			return repeatedSpaces[:n]
		case n <= len(repeatedDashes) && HasPrefix(repeatedDashes, s):
			return repeatedDashes[:n]
		case n <= len(repeatedZeroes) && HasPrefix(repeatedZeroes, s):
			return repeatedZeroes[:n]
		case n <= len(repeatedEquals) && HasPrefix(repeatedEquals, s):
			return repeatedEquals[:n]
		case n <= len(repeatedTabs) && HasPrefix(repeatedTabs, s):
			return repeatedTabs[:n]
		}
	}

	// Past a certain chunk size it is counterproductive to use
	// larger chunks as the source of the write, as when the source
	// is too large we are basically just thrashing the CPU D-cache.
	// So if the result length is larger than an empirically-found
	// limit (8KB), we stop growing the source string once the limit
	// is reached and keep reusing the same source string - that
	// should therefore be always resident in the L1 cache - until we
	// have completed the construction of the result.
	// This yields significant speedups (up to +100%) in cases where
	// the result length is large (roughly, over L2 cache size).
	const chunkLimit = 8 * 1024
	chunkMax := n
	if n > chunkLimit {
		chunkMax = chunkLimit / len(s) * len(s)
		if chunkMax == 0 {
			chunkMax = len(s)
		}
	}

	var b Builder
	b.Grow(n)
	b.WriteString(s)
	for b.Len() < n {
		chunk := min(n-b.Len(), b.Len(), chunkMax)
		b.WriteString(b.String()[:chunk])
	}
	return b.String()
}

// ToUpper returns s with all Unicode letters mapped to their upper case.
func ToUpper(s string) string {
	isASCII, hasLower := true, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= utf8.RuneSelf {
			isASCII = false
			break
		}
		hasLower = hasLower || ('a' <= c && c <= 'z')
	}

	if isASCII { // optimize for ASCII-only strings.
		if !hasLower {
			return s
		}
		var (
			b   Builder
			pos int
		)
		b.Grow(len(s))
		for i := 0; i < len(s); i++ {
			c := s[i]
			if 'a' <= c && c <= 'z' {
				c -= 'a' - 'A'
				if pos < i {
					b.WriteString(s[pos:i])
				}
				b.WriteByte(c)
				pos = i + 1
			}
		}
		if pos < len(s) {
			b.WriteString(s[pos:])
		}
		return b.String()
	}
	return Map(unicode.ToUpper, s)
}

// ToLower returns s with all Unicode letters mapped to their lower case.
func ToLower(s string) string {
	isASCII, hasUpper := true, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= utf8.RuneSelf {
			isASCII = false
			break
		}
		hasUpper = hasUpper || ('A' <= c && c <= 'Z')
	}

	if isASCII { // optimize for ASCII-only strings.
		if !hasUpper {
			return s
		}
		var (
			b   Builder
			pos int
		)
		b.Grow(len(s))
		for i := 0; i < len(s); i++ {
			c := s[i]
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
				if pos < i {
					b.WriteString(s[pos:i])
				}
				b.WriteByte(c)
				pos = i + 1
			}
		}
		if pos < len(s) {
			b.WriteString(s[pos:])
		}
		return b.String()
	}
	return Map(unicode.ToLower, s)
}

// ToTitle returns a copy of the string s with all Unicode letters mapped to
// their Unicode title case.
func ToTitle(s string) string { return Map(unicode.ToTitle, s) }

// ToUpperSpecial returns a copy of the string s with all Unicode letters mapped to their
// upper case using the case mapping specified by c.
func ToUpperSpecial(c unicode.SpecialCase, s string) string {
	return Map(c.ToUpper, s)
}

// ToLowerSpecial returns a copy of the string s with all Unicode letters mapped to their
// lower case using the case mapping specified by c.
func ToLowerSpecial(c unicode.SpecialCase, s string) string {
	return Map(c.ToLower, s)
}

// ToTitleSpecial returns a copy of the string s with all Unicode letters mapped to their
// Unicode title case, giving priority to the special casing rules.
func ToTitleSpecial(c unicode.SpecialCase, s string) string {
	return Map(c.ToTitle, s)
}

// ToValidUTF8 returns a copy of the string s with each run of invalid UTF-8 byte sequences
// replaced by the replacement string, which may be empty.
func ToValidUTF8(s, replacement string) string {
	var b Builder

	for i, c := range s {
		if c != utf8.RuneError {
			continue
		}

		_, wid := utf8.DecodeRuneInString(s[i:])
		if wid == 1 {
			b.Grow(len(s) + len(replacement))
			b.WriteString(s[:i])
			s = s[i:]
			break
		}
	}

	// Fast path for unchanged input
	if b.Cap() == 0 { // didn't call b.Grow above
		return s
	}

	invalid := false // previous byte was from an invalid UTF-8 sequence
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			i++
			invalid = false
			b.WriteByte(c)
			continue
		}
		_, wid := utf8.DecodeRuneInString(s[i:])
		if wid == 1 {
			i++
			if !invalid {
				invalid = true
				b.WriteString(replacement)
			}
			continue
		}
		invalid = false
		b.WriteString(s[i : i+wid])
		i += wid
	}

	return b.String()
}

// isSeparator reports whether the rune could mark a word boundary.
// TODO: update when package unicode captures more of the properties.
func isSeparator(r rune) bool {
	// ASCII alphanumerics and underscore are not separators
	if r <= 0x7F {
		switch {
		case '0' <= r && r <= '9':
			return false
		case 'a' <= r && r <= 'z':
			return false
		case 'A' <= r && r <= 'Z':
			return false
		case r == '_':
			return false
		}
		return true
	}
	// Letters and digits are not separators
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return false
	}
	// Otherwise, all we can do for now is treat spaces as separators.
	return unicode.IsSpace(r)
}

// Title returns a copy of the string s with all Unicode letters that begin words
// mapped to their Unicode title case.
//
// Deprecated: The rule Title uses for word boundaries does not handle Unicode
// punctuation properly. Use golang.org/x/text/cases instead.
func Title(s string) string {
	// Use a closure here to remember state.
	// Hackish but effective. Depends on Map scanning in order and calling
	// the closure once per rune.
	prev := ' '
	return Map(
		func(r rune) rune {
			if isSeparator(prev) {
				prev = r
				return unicode.ToTitle(r)
			}
			prev = r
			return r
		},
		s)
}

// TrimLeftFunc returns a slice of the string s with all leading
// Unicode code points c satisfying f(c) removed.
func TrimLeftFunc(s string, f func(rune) bool) string {
	i := indexFunc(s, f, false)
	if i == -1 {
		return ""
	}
	return s[i:]
}

// TrimRightFunc returns a slice of the string s with all trailing
// Unicode code points c satisfying f(c) removed.
func TrimRightFunc(s string, f func(rune) bool) string {
	i := lastIndexFunc(s, f, false)
	if i >= 0 {
		_, wid := utf8.DecodeRuneInString(s[i:])
		i += wid
	} else {
		i++
	}
	return s[0:i]
}

// TrimFunc returns a slice of the string s with all leading
// and trailing Unicode code points c satisfying f(c) removed.
func TrimFunc(s string, f func(rune) bool) string {
	return TrimRightFunc(TrimLeftFunc(s, f), f)
}

// IndexFunc returns the index into s of the first Unicode
// code point satisfying f(c), or -1 if none do.
func IndexFunc(s string, f func(rune) bool) int {
	return indexFunc(s, f, true)
}

// LastIndexFunc returns the index into s of the last
// Unicode code point satisfying f(c), or -1 if none do.
func LastIndexFunc(s string, f func(rune) bool) int {
	return lastIndexFunc(s, f, true)
}

// indexFunc is the same as IndexFunc except that if
// truth==false, the sense of the predicate function is
// inverted.
func indexFunc(s string, f func(rune) bool, truth bool) int {
	for i, r := range s {
		if f(r) == truth {
			return i
		}
	}
	return -1
}

// lastIndexFunc is the same as LastIndexFunc except that if
// truth==false, the sense of the predicate function is
// inverted.
func lastIndexFunc(s string, f func(rune) bool, truth bool) int {
	for i := len(s); i > 0; {
		r, size := utf8.DecodeLastRuneInString(s[0:i])
		i -= size
		if f(r) == truth {
			return i
		}
	}
	return -1
}

// asciiSet is a 256-byte lookup table for fast ASCII character membership testing.
// Each element corresponds to an ASCII character value, with true indicating the
// character is in the set. Using bool instead of byte allows the compiler to
// eliminate the comparison instruction, as bool values are guaranteed to be 0 or 1.
//
// The full 256-element table is used rather than a 128-element table to avoid
// additional operations in the lookup path. Alternative approaches were tested:
//   - [128]bool with explicit bounds check (if c >= 128): introduces branches
//     that cause pipeline stalls, resulting in ~70% slower performance
//   - [128]bool with masking (c&0x7f): eliminates bounds checks but the AND
//     operation still costs ~10% performance compared to direct indexing
//
// The 256-element array allows direct indexing with no bounds checks, no branches,
// and no masking operations, providing optimal performance. The additional 128 bytes
// of memory is a worthwhile tradeoff for the simpler, faster code.
type asciiSet [256]bool

// makeASCIISet creates a set of ASCII characters and reports whether all
// characters in chars are ASCII.
func makeASCIISet(chars string) (as asciiSet, ok bool) {
	for i := 0; i < len(chars); i++ {
		c := chars[i]
		if c >= utf8.RuneSelf {
			return as, false
		}
		as[c] = true
	}
	return as, true
}

// contains reports whether c is inside the set.
func (as *asciiSet) contains(c byte) bool {
	return as[c]
}

// shouldUseASCIISet returns whether to use the lookup table optimization.
// The threshold of 8 bytes balances initialization cost against per-byte
// search cost, performing well across all charset sizes.
//
// More complex heuristics (e.g., different thresholds per charset size)
// add branching overhead that eats away any theoretical improvements.
func shouldUseASCIISet(bufLen int) bool {
	return bufLen > 8
}

// Trim returns a slice of the string s with all leading and
// trailing Unicode code points contained in cutset removed.
func Trim(s, cutset string) string {
	if s == "" || cutset == "" {
		return s
	}
	if len(cutset) == 1 && cutset[0] < utf8.RuneSelf {
		return trimLeftByte(trimRightByte(s, cutset[0]), cutset[0])
	}
	if as, ok := makeASCIISet(cutset); ok {
		return trimLeftASCII(trimRightASCII(s, &as), &as)
	}
	return trimLeftUnicode(trimRightUnicode(s, cutset), cutset)
}

// TrimLeft returns a slice of the string s with all leading
// Unicode code points contained in cutset removed.
//
// To remove a prefix, use [TrimPrefix] instead.
func TrimLeft(s, cutset string) string {
	if s == "" || cutset == "" {
		return s
	}
	if len(cutset) == 1 && cutset[0] < utf8.RuneSelf {
		return trimLeftByte(s, cutset[0])
	}
	if as, ok := makeASCIISet(cutset); ok {
		return trimLeftASCII(s, &as)
	}
	return trimLeftUnicode(s, cutset)
}

func trimLeftByte(s string, c byte) string {
	for len(s) > 0 && s[0] == c {
		s = s[1:]
	}
	return s
}

func trimLeftASCII(s string, as *asciiSet) string {
	for len(s) > 0 {
		if !as.contains(s[0]) {
			break
		}
		s = s[1:]
	}
	return s
}

func trimLeftUnicode(s, cutset string) string {
	for len(s) > 0 {
		r, n := utf8.DecodeRuneInString(s)
		if !ContainsRune(cutset, r) {
			break
		}
		s = s[n:]
	}
	return s
}

// TrimRight returns a slice of the string s, with all trailing
// Unicode code points contained in cutset removed.
//
// To remove a suffix, use [TrimSuffix] instead.
func TrimRight(s, cutset string) string {
	if s == "" || cutset == "" {
		return s
	}
	if len(cutset) == 1 && cutset[0] < utf8.RuneSelf {
		return trimRightByte(s, cutset[0])
	}
	if as, ok := makeASCIISet(cutset); ok {
		return trimRightASCII(s, &as)
	}
	return trimRightUnicode(s, cutset)
}

func trimRightByte(s string, c byte) string {
	for len(s) > 0 && s[len(s)-1] == c {
		s = s[:len(s)-1]
	}
	return s
}

func trimRightASCII(s string, as *asciiSet) string {
	for len(s) > 0 {
		if !as.contains(s[len(s)-1]) {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}

func trimRightUnicode(s, cutset string) string {
	for len(s) > 0 {
		r, n := rune(s[len(s)-1]), 1
		if r >= utf8.RuneSelf {
			r, n = utf8.DecodeLastRuneInString(s)
		}
		if !ContainsRune(cutset, r) {
			break
		}
		s = s[:len(s)-n]
	}
	return s
}

// TrimSpace returns a slice (substring) of the string s,
// with all leading and trailing white space removed,
// as defined by Unicode.
func TrimSpace(s string) string {
	// Fast path for ASCII: look for the first ASCII non-space byte.
	for lo, c := range []byte(s) {
		if c >= utf8.RuneSelf {
			// If we run into a non-ASCII byte, fall back to the
			// slower unicode-aware method on the remaining bytes.
			return TrimFunc(s[lo:], unicode.IsSpace)
		}
		if asciiSpace[c] != 0 {
			continue
		}
		s = s[lo:]
		// Now look for the first ASCII non-space byte from the end.
		for hi := len(s) - 1; hi >= 0; hi-- {
			c := s[hi]
			if c >= utf8.RuneSelf {
				return TrimRightFunc(s[:hi+1], unicode.IsSpace)
			}
			if asciiSpace[c] == 0 {
				// At this point, s[:hi+1] starts and ends with ASCII
				// non-space bytes, so we're done. Non-ASCII cases have
				// already been handled above.
				return s[:hi+1]
			}
		}
	}
	return ""
}

// TrimPrefix returns s without the provided leading prefix string.
// If s doesn't start with prefix, s is returned unchanged.
func TrimPrefix(s, prefix string) string {
	return stringslite.TrimPrefix(s, prefix)
}

// TrimSuffix returns s without the provided trailing suffix string.
// If s doesn't end with suffix, s is returned unchanged.
func TrimSuffix(s, suffix string) string {
	return stringslite.TrimSuffix(s, suffix)
}

// Replace returns a copy of the string s with the first n
// non-overlapping instances of old replaced by new.
// If old is empty, it matches at the beginning of the string
// and after each UTF-8 sequence, yielding up to k+1 replacements
// for a k-rune string.
// If n < 0, there is no limit on the number of replacements.
func Replace(s, old, new string, n int) string {
	if old == new || n == 0 {
		return s // avoid allocation
	}

	// Compute number of replacements.
	if m := Count(s, old); m == 0 {
		return s // avoid allocation
	} else if n < 0 || m < n {
		n = m
	}

	// Apply replacements to buffer.
	var b Builder
	b.Grow(len(s) + n*(len(new)-len(old)))
	start := 0
	if len(old) > 0 {
		for range n {
			j := start + Index(s[start:], old)
			b.WriteString(s[start:j])
			b.WriteString(new)
			start = j + len(old)
		}
	} else { // len(old) == 0
		b.WriteString(new)
		for range n - 1 {
			_, wid := utf8.DecodeRuneInString(s[start:])
			j := start + wid
			b.WriteString(s[start:j])
			b.WriteString(new)
			start = j
		}
	}
	b.WriteString(s[start:])
	return b.String()
}

// ReplaceAll returns a copy of the string s with all
// non-overlapping instances of old replaced by new.
// If old is empty, it matches at the beginning of the string
// and after each UTF-8 sequence, yielding up to k+1 replacements
// for a k-rune string.
func ReplaceAll(s, old, new string) string {
	return Replace(s, old, new, -1)
}

// EqualFold reports whether s and t, interpreted as UTF-8 strings,
// are equal under simple Unicode case-folding, which is a more general
// form of case-insensitivity.
func EqualFold(s, t string) bool {
	// ASCII fast path
	i := 0
	for n := min(len(s), len(t)); i < n; i++ {
		sr := s[i]
		tr := t[i]
		if sr|tr >= utf8.RuneSelf {
			goto hasUnicode
		}

		// Easy case.
		if tr == sr {
			continue
		}

		// Make sr < tr to simplify what follows.
		if tr < sr {
			tr, sr = sr, tr
		}
		// ASCII only, sr/tr must be upper/lower case
		if 'A' <= sr && sr <= 'Z' && tr == sr+'a'-'A' {
			continue
		}
		return false
	}
	// Check if we've exhausted both strings.
	return len(s) == len(t)

hasUnicode:
	s = s[i:]
	t = t[i:]
	for _, sr := range s {
		// If t is exhausted the strings are not equal.
		if len(t) == 0 {
			return false
		}

		// Extract first rune from second string.
		tr, size := utf8.DecodeRuneInString(t)
		t = t[size:]

		// If they match, keep going; if not, return false.

		// Easy case.
		if tr == sr {
			continue
		}

		// Make sr < tr to simplify what follows.
		if tr < sr {
			tr, sr = sr, tr
		}
		// Fast check for ASCII.
		if tr < utf8.RuneSelf {
			// ASCII only, sr/tr must be upper/lower case
			if 'A' <= sr && sr <= 'Z' && tr == sr+'a'-'A' {
				continue
			}
			return false
		}

		// General case. SimpleFold(x) returns the next equivalent rune > x
		// or wraps around to smaller values.
		r := unicode.SimpleFold(sr)
		for r != sr && r < tr {
			r = unicode.SimpleFold(r)
		}
		if r == tr {
			continue
		}
		return false
	}

	// First string is empty, so check if the second one is also empty.
	return len(t) == 0
}

// Index returns the index of the first instance of substr in s, or -1 if substr is not present in s.
func Index(s, substr string) int {
	return stringslite.Index(s, substr)
}

// Cut slices s around the first instance of sep,
// returning the text before and after sep.
// The found result reports whether sep appears in s.
// If sep does not appear in s, cut returns s, "", false.
func Cut(s, sep string) (before, after string, found bool) {
	return stringslite.Cut(s, sep)
}

// CutPrefix returns s without the provided leading prefix string
// and reports whether it found the prefix.
// If s doesn't start with prefix, CutPrefix returns s, false.
// If prefix is the empty string, CutPrefix returns s, true.
func CutPrefix(s, prefix string) (after string, found bool) {
	return stringslite.CutPrefix(s, prefix)
}

// CutSuffix returns s without the provided ending suffix string
// and reports whether it found the suffix.
// If s doesn't end with suffix, CutSuffix returns s, false.
// If suffix is the empty string, CutSuffix returns s, true.
func CutSuffix(s, suffix string) (before string, found bool) {
	return stringslite.CutSuffix(s, suffix)
}

// CutLast slices s around the last instance of sep,
// returning the text before and after sep.
// The found result reports whether sep appears in s.
// If sep does not appear in s, CutLast returns s, "", false.
func CutLast(s, sep string) (before, after string, found bool) {
	if i := LastIndex(s, sep); i >= 0 {
		return s[:i], s[i+len(sep):], true
	}
	return s, "", false
}

```

