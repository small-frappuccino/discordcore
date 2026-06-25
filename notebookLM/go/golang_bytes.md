# Domain Architecture: bytes

## Layout Topology
```text
bytes/
├── buffer.go
├── bytes.go
├── iter.go
└── reader.go
```

## Source Stream Aggregation

// === FILE: references!/go/src/bytes/buffer.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bytes

// Simple byte buffer for marshaling data.

import (
	"errors"
	"io"
	"unicode/utf8"
)

// smallBufferSize is an initial allocation minimal capacity.
const smallBufferSize = 64

// A Buffer is a variable-sized buffer of bytes with [Buffer.Read] and [Buffer.Write] methods.
// The zero value for Buffer is an empty buffer ready to use.
type Buffer struct {
	buf      []byte // contents are the bytes buf[off : len(buf)]
	off      int    // read at &buf[off], write at &buf[len(buf)]
	lastRead readOp // last read operation, so that Unread* can work correctly.

	// Copying and modifying a non-zero Buffer is prone to error,
	// but we cannot employ the noCopy trick used by WaitGroup and Mutex,
	// which causes vet's copylocks checker to report misuse, as vet
	// cannot reliably distinguish the zero and non-zero cases.
	// See #26462, #25907, #47276, #48398 for history.
}

// The readOp constants describe the last action performed on
// the buffer, so that UnreadRune and UnreadByte can check for
// invalid usage. opReadRuneX constants are chosen such that
// converted to int they correspond to the rune size that was read.
type readOp int8

// Don't use iota for these, as the values need to correspond with the
// names and comments, which is easier to see when being explicit.
const (
	opRead      readOp = -1 // Any other read operation.
	opInvalid   readOp = 0  // Non-read operation.
	opReadRune1 readOp = 1  // Read rune of size 1.
	opReadRune2 readOp = 2  // Read rune of size 2.
	opReadRune3 readOp = 3  // Read rune of size 3.
	opReadRune4 readOp = 4  // Read rune of size 4.
)

// ErrTooLarge is passed to panic if memory cannot be allocated to store data in a buffer.
var ErrTooLarge = errors.New("bytes.Buffer: too large")
var errNegativeRead = errors.New("bytes.Buffer: reader returned negative count from Read")

const maxInt = int(^uint(0) >> 1)

// Bytes returns a slice of length b.Len() holding the unread portion of the buffer.
// The slice is valid for use only until the next buffer modification (that is,
// only until the next call to a method like [Buffer.Read], [Buffer.Write], [Buffer.Reset], or [Buffer.Truncate]).
// The slice aliases the buffer content at least until the next buffer modification,
// so immediate changes to the slice will affect the result of future reads.
func (b *Buffer) Bytes() []byte { return b.buf[b.off:] }

// AvailableBuffer returns an empty buffer with b.Available() capacity.
// This buffer is intended to be appended to and
// passed to an immediately succeeding [Buffer.Write] call.
// The buffer is only valid until the next write operation on b.
func (b *Buffer) AvailableBuffer() []byte { return b.buf[len(b.buf):] }

// String returns the contents of the unread portion of the buffer
// as a string. If the [Buffer] is a nil pointer, it returns "<nil>".
//
// To build strings more efficiently, see the [strings.Builder] type.
func (b *Buffer) String() string {
	if b == nil {
		// Special case, useful in debugging.
		return "<nil>"
	}
	return string(b.buf[b.off:])
}

// Peek returns the next n bytes without advancing the buffer.
// If Peek returns fewer than n bytes, it also returns [io.EOF].
// The slice is only valid until the next call to a read or write method.
// The slice aliases the buffer content at least until the next buffer modification,
// so immediate changes to the slice will affect the result of future reads.
func (b *Buffer) Peek(n int) ([]byte, error) {
	if b.Len() < n {
		return b.buf[b.off:], io.EOF
	}
	return b.buf[b.off : b.off+n], nil
}

// empty reports whether the unread portion of the buffer is empty.
func (b *Buffer) empty() bool { return len(b.buf) <= b.off }

// Len returns the number of bytes of the unread portion of the buffer;
// b.Len() == len(b.Bytes()).
func (b *Buffer) Len() int { return len(b.buf) - b.off }

// Cap returns the capacity of the buffer's underlying byte slice, that is, the
// total space allocated for the buffer's data.
func (b *Buffer) Cap() int { return cap(b.buf) }

// Available returns how many bytes are unused in the buffer.
func (b *Buffer) Available() int { return cap(b.buf) - len(b.buf) }

// Truncate discards all but the first n unread bytes from the buffer
// but continues to use the same allocated storage.
// It panics if n is negative or greater than the length of the buffer.
func (b *Buffer) Truncate(n int) {
	if n == 0 {
		b.Reset()
		return
	}
	b.lastRead = opInvalid
	if n < 0 || n > b.Len() {
		panic("bytes.Buffer: truncation out of range")
	}
	b.buf = b.buf[:b.off+n]
}

// Reset resets the buffer to be empty,
// but it retains the underlying storage for use by future writes.
// Reset is the same as [Buffer.Truncate](0).
func (b *Buffer) Reset() {
	b.buf = b.buf[:0]
	b.off = 0
	b.lastRead = opInvalid
}

// tryGrowByReslice is an inlineable version of grow for the fast-case where the
// internal buffer only needs to be resliced.
// It returns the index where bytes should be written and whether it succeeded.
func (b *Buffer) tryGrowByReslice(n int) (int, bool) {
	if l := len(b.buf); n <= cap(b.buf)-l {
		b.buf = b.buf[:l+n]
		return l, true
	}
	return 0, false
}

// grow grows the buffer to guarantee space for n more bytes.
// It returns the index where bytes should be written.
// If the buffer can't grow it will panic with ErrTooLarge.
func (b *Buffer) grow(n int) int {
	m := b.Len()
	// If buffer is empty, reset to recover space.
	if m == 0 && b.off != 0 {
		b.Reset()
	}
	// Try to grow by means of a reslice.
	if i, ok := b.tryGrowByReslice(n); ok {
		return i
	}
	if b.buf == nil && n <= smallBufferSize {
		b.buf = make([]byte, n, smallBufferSize)
		return 0
	}
	c := cap(b.buf)
	if n <= c/2-m {
		// We can slide things down instead of allocating a new
		// slice. We only need m+n <= c to slide, but
		// we instead let capacity get twice as large so we
		// don't spend all our time copying.
		copy(b.buf, b.buf[b.off:])
	} else if c > maxInt-c-n {
		panic(ErrTooLarge)
	} else {
		// Add b.off to account for b.buf[:b.off] being sliced off the front.
		b.buf = growSlice(b.buf[b.off:], b.off+n)
	}
	// Restore b.off and len(b.buf).
	b.off = 0
	b.buf = b.buf[:m+n]
	return m
}

// Grow grows the buffer's capacity, if necessary, to guarantee space for
// another n bytes. After Grow(n), at least n bytes can be written to the
// buffer without another allocation.
// If n is negative, Grow will panic.
// If the buffer can't grow it will panic with [ErrTooLarge].
func (b *Buffer) Grow(n int) {
	if n < 0 {
		panic("bytes.Buffer.Grow: negative count")
	}
	m := b.grow(n)
	b.buf = b.buf[:m]
}

// Write appends the contents of p to the buffer, growing the buffer as
// needed. The return value n is the length of p; err is always nil. If the
// buffer becomes too large, Write will panic with [ErrTooLarge].
func (b *Buffer) Write(p []byte) (n int, err error) {
	b.lastRead = opInvalid
	m, ok := b.tryGrowByReslice(len(p))
	if !ok {
		m = b.grow(len(p))
	}
	return copy(b.buf[m:], p), nil
}

// WriteString appends the contents of s to the buffer, growing the buffer as
// needed. The return value n is the length of s; err is always nil. If the
// buffer becomes too large, WriteString will panic with [ErrTooLarge].
func (b *Buffer) WriteString(s string) (n int, err error) {
	b.lastRead = opInvalid
	m, ok := b.tryGrowByReslice(len(s))
	if !ok {
		m = b.grow(len(s))
	}
	return copy(b.buf[m:], s), nil
}

// MinRead is the minimum slice size passed to a [Buffer.Read] call by
// [Buffer.ReadFrom]. As long as the [Buffer] has at least MinRead bytes beyond
// what is required to hold the contents of r, [Buffer.ReadFrom] will not grow the
// underlying buffer.
const MinRead = 512

// ReadFrom reads data from r until EOF and appends it to the buffer, growing
// the buffer as needed. The return value n is the number of bytes read. Any
// error except io.EOF encountered during the read is also returned. If the
// buffer becomes too large, ReadFrom will panic with [ErrTooLarge].
func (b *Buffer) ReadFrom(r io.Reader) (n int64, err error) {
	b.lastRead = opInvalid
	for {
		i := b.grow(MinRead)
		b.buf = b.buf[:i]
		m, e := r.Read(b.buf[i:cap(b.buf)])
		if m < 0 {
			panic(errNegativeRead)
		}

		b.buf = b.buf[:i+m]
		n += int64(m)
		if e == io.EOF {
			return n, nil // e is EOF, so return nil explicitly
		}
		if e != nil {
			return n, e
		}
	}
}

// growSlice grows b by n, preserving the original content of b.
// If the allocation fails, it panics with ErrTooLarge.
func growSlice(b []byte, n int) []byte {
	defer func() {
		if recover() != nil {
			panic(ErrTooLarge)
		}
	}()
	// TODO(http://golang.org/issue/51462): We should rely on the append-make
	// pattern so that the compiler can call runtime.growslice. For example:
	//	return append(b, make([]byte, n)...)
	// This avoids unnecessary zero-ing of the first len(b) bytes of the
	// allocated slice, but this pattern causes b to escape onto the heap.
	//
	// Instead use the append-make pattern with a nil slice to ensure that
	// we allocate buffers rounded up to the closest size class.
	c := len(b) + n // ensure enough space for n elements
	if c < 2*cap(b) {
		// The growth rate has historically always been 2x. In the future,
		// we could rely purely on append to determine the growth rate.
		c = 2 * cap(b)
	}
	b2 := append([]byte(nil), make([]byte, c)...)
	i := copy(b2, b)
	return b2[:i]
}

// WriteTo writes data to w until the buffer is drained or an error occurs.
// The return value n is the number of bytes written; it always fits into an
// int, but it is int64 to match the [io.WriterTo] interface. Any error
// encountered during the write is also returned.
func (b *Buffer) WriteTo(w io.Writer) (n int64, err error) {
	b.lastRead = opInvalid
	if nBytes := b.Len(); nBytes > 0 {
		m, e := w.Write(b.buf[b.off:])
		if m > nBytes {
			panic("bytes.Buffer.WriteTo: invalid Write count")
		}
		b.off += m
		n = int64(m)
		if e != nil {
			return n, e
		}
		// all bytes should have been written, by definition of
		// Write method in io.Writer
		if m != nBytes {
			return n, io.ErrShortWrite
		}
	}
	// Buffer is now empty; reset.
	b.Reset()
	return n, nil
}

// WriteByte appends the byte c to the buffer, growing the buffer as needed.
// The returned error is always nil, but is included to match [bufio.Writer]'s
// WriteByte. If the buffer becomes too large, WriteByte will panic with
// [ErrTooLarge].
func (b *Buffer) WriteByte(c byte) error {
	b.lastRead = opInvalid
	m, ok := b.tryGrowByReslice(1)
	if !ok {
		m = b.grow(1)
	}
	b.buf[m] = c
	return nil
}

// WriteRune appends the UTF-8 encoding of Unicode code point r to the
// buffer, returning the number of bytes written and a nil error. The nil
// error is included to match [bufio.Writer]'s WriteRune. The buffer is grown
// as needed; if it becomes too large, WriteRune will panic with [ErrTooLarge].
func (b *Buffer) WriteRune(r rune) (n int, err error) {
	// Compare as uint32 to correctly handle negative runes.
	if uint32(r) < utf8.RuneSelf {
		b.WriteByte(byte(r))
		return 1, nil
	}
	b.lastRead = opInvalid
	m, ok := b.tryGrowByReslice(utf8.UTFMax)
	if !ok {
		m = b.grow(utf8.UTFMax)
	}
	b.buf = utf8.AppendRune(b.buf[:m], r)
	return len(b.buf) - m, nil
}

// Read reads the next len(p) bytes from the buffer or until the buffer
// is drained. The return value n is the number of bytes read. If the
// buffer has no data to return, err is [io.EOF] (unless len(p) is zero);
// otherwise it is nil.
func (b *Buffer) Read(p []byte) (n int, err error) {
	b.lastRead = opInvalid
	if b.empty() {
		// Buffer is empty, reset to recover space.
		b.Reset()
		if len(p) == 0 {
			return 0, nil
		}
		return 0, io.EOF
	}
	n = copy(p, b.buf[b.off:])
	b.off += n
	if n > 0 {
		b.lastRead = opRead
	}
	return n, nil
}

// Next returns a slice containing the next n bytes from the buffer,
// advancing the buffer as if the bytes had been returned by [Buffer.Read].
// If there are fewer than n bytes in the buffer, Next returns the entire buffer.
// The slice is only valid until the next call to a read or write method.
func (b *Buffer) Next(n int) []byte {
	b.lastRead = opInvalid
	m := b.Len()
	if n > m {
		n = m
	}
	data := b.buf[b.off : b.off+n]
	b.off += n
	if n > 0 {
		b.lastRead = opRead
	}
	return data
}

// ReadByte reads and returns the next byte from the buffer.
// If no byte is available, it returns error [io.EOF].
func (b *Buffer) ReadByte() (byte, error) {
	if b.empty() {
		// Buffer is empty, reset to recover space.
		b.Reset()
		return 0, io.EOF
	}
	c := b.buf[b.off]
	b.off++
	b.lastRead = opRead
	return c, nil
}

// ReadRune reads and returns the next UTF-8-encoded
// Unicode code point from the buffer.
// If no bytes are available, the error returned is io.EOF.
// If the bytes are an erroneous UTF-8 encoding, it
// consumes one byte and returns U+FFFD, 1.
func (b *Buffer) ReadRune() (r rune, size int, err error) {
	if b.empty() {
		// Buffer is empty, reset to recover space.
		b.Reset()
		return 0, 0, io.EOF
	}
	c := b.buf[b.off]
	if c < utf8.RuneSelf {
		b.off++
		b.lastRead = opReadRune1
		return rune(c), 1, nil
	}
	r, n := utf8.DecodeRune(b.buf[b.off:])
	b.off += n
	b.lastRead = readOp(n)
	return r, n, nil
}

// UnreadRune unreads the last rune returned by [Buffer.ReadRune].
// If the most recent read or write operation on the buffer was
// not a successful [Buffer.ReadRune], UnreadRune returns an error.  (In this regard
// it is stricter than [Buffer.UnreadByte], which will unread the last byte
// from any read operation.)
func (b *Buffer) UnreadRune() error {
	if b.lastRead <= opInvalid {
		return errors.New("bytes.Buffer: UnreadRune: previous operation was not a successful ReadRune")
	}
	if b.off >= int(b.lastRead) {
		b.off -= int(b.lastRead)
	}
	b.lastRead = opInvalid
	return nil
}

var errUnreadByte = errors.New("bytes.Buffer: UnreadByte: previous operation was not a successful read")

// UnreadByte unreads the last byte returned by the most recent successful
// read operation that read at least one byte. If a write has happened since
// the last read, if the last read returned an error, or if the read read zero
// bytes, UnreadByte returns an error.
func (b *Buffer) UnreadByte() error {
	if b.lastRead == opInvalid {
		return errUnreadByte
	}
	b.lastRead = opInvalid
	if b.off > 0 {
		b.off--
	}
	return nil
}

// ReadBytes reads until the first occurrence of delim in the input,
// returning a slice containing the data up to and including the delimiter.
// If ReadBytes encounters an error before finding a delimiter,
// it returns the data read before the error and the error itself (often [io.EOF]).
// ReadBytes returns err != nil if and only if the returned data does not end in
// delim.
func (b *Buffer) ReadBytes(delim byte) (line []byte, err error) {
	slice, err := b.readSlice(delim)
	// return a copy of slice. The buffer's backing array may
	// be overwritten by later calls.
	line = append(line, slice...)
	return line, err
}

// readSlice is like ReadBytes but returns a reference to internal buffer data.
func (b *Buffer) readSlice(delim byte) (line []byte, err error) {
	i := IndexByte(b.buf[b.off:], delim)
	end := b.off + i + 1
	if i < 0 {
		end = len(b.buf)
		err = io.EOF
	}
	line = b.buf[b.off:end]
	b.off = end
	b.lastRead = opRead
	return line, err
}

// ReadString reads until the first occurrence of delim in the input,
// returning a string containing the data up to and including the delimiter.
// If ReadString encounters an error before finding a delimiter,
// it returns the data read before the error and the error itself (often [io.EOF]).
// ReadString returns err != nil if and only if the returned data does not end
// in delim.
func (b *Buffer) ReadString(delim byte) (line string, err error) {
	slice, err := b.readSlice(delim)
	return string(slice), err
}

// NewBuffer creates and initializes a new [Buffer] using buf as its
// initial contents. The new [Buffer] takes ownership of buf, and the
// caller should not use buf after this call. NewBuffer is intended to
// prepare a [Buffer] to read existing data. It can also be used to set
// the initial size of the internal buffer for writing. To do that,
// buf should have the desired capacity but a length of zero.
//
// In most cases, new([Buffer]) (or just declaring a [Buffer] variable) is
// sufficient to initialize a [Buffer].
func NewBuffer(buf []byte) *Buffer { return &Buffer{buf: buf} }

// NewBufferString creates and initializes a new [Buffer] using string s as its
// initial contents. It is intended to prepare a buffer to read an existing
// string.
//
// In most cases, new([Buffer]) (or just declaring a [Buffer] variable) is
// sufficient to initialize a [Buffer].
func NewBufferString(s string) *Buffer {
	return &Buffer{buf: []byte(s)}
}

```

// === FILE: references!/go/src/bytes/bytes.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package bytes implements functions for the manipulation of byte slices.
// It is analogous to the facilities of the [strings] package.
package bytes

import (
	"internal/bytealg"
	"math/bits"
	"unicode"
	"unicode/utf8"
	_ "unsafe" // for linkname
)

// Equal reports whether a and b
// are the same length and contain the same bytes.
// A nil argument is equivalent to an empty slice.
func Equal(a, b []byte) bool {
	// Neither cmd/compile nor gccgo allocates for these string conversions.
	return string(a) == string(b)
}

// Compare returns an integer comparing two byte slices lexicographically.
// The result will be 0 if a == b, -1 if a < b, and +1 if a > b.
// A nil argument is equivalent to an empty slice.
func Compare(a, b []byte) int {
	return bytealg.Compare(a, b)
}

// explode splits s into a slice of UTF-8 sequences, one per Unicode code point (still slices of bytes),
// up to a maximum of n byte slices. Invalid UTF-8 sequences are chopped into individual bytes.
func explode(s []byte, n int) [][]byte {
	if n <= 0 || n > len(s) {
		n = len(s)
	}
	a := make([][]byte, n)
	var size int
	na := 0
	for len(s) > 0 {
		if na+1 >= n {
			a[na] = s
			na++
			break
		}
		_, size = utf8.DecodeRune(s)
		a[na] = s[0:size:size]
		s = s[size:]
		na++
	}
	return a[0:na]
}

// Count counts the number of non-overlapping instances of sep in s.
// If sep is an empty slice, Count returns 1 + the number of UTF-8-encoded code points in s.
func Count(s, sep []byte) int {
	// special case
	if len(sep) == 0 {
		return utf8.RuneCount(s) + 1
	}
	if len(sep) == 1 {
		return bytealg.Count(s, sep[0])
	}
	n := 0
	for {
		i := Index(s, sep)
		if i == -1 {
			return n
		}
		n++
		s = s[i+len(sep):]
	}
}

// Contains reports whether subslice is within b.
func Contains(b, subslice []byte) bool {
	return Index(b, subslice) != -1
}

// ContainsAny reports whether any of the UTF-8-encoded code points in chars are within b.
func ContainsAny(b []byte, chars string) bool {
	return IndexAny(b, chars) >= 0
}

// ContainsRune reports whether the rune is contained in the UTF-8-encoded byte slice b.
func ContainsRune(b []byte, r rune) bool {
	return IndexRune(b, r) >= 0
}

// ContainsFunc reports whether any of the UTF-8-encoded code points r within b satisfy f(r).
// It stops as soon as a call to f returns true.
func ContainsFunc(b []byte, f func(rune) bool) bool {
	return IndexFunc(b, f) >= 0
}

// IndexByte returns the index of the first instance of c in b, or -1 if c is not present in b.
func IndexByte(b []byte, c byte) int {
	return bytealg.IndexByte(b, c)
}

// LastIndex returns the index of the last instance of sep in s, or -1 if sep is not present in s.
func LastIndex(s, sep []byte) int {
	n := len(sep)
	switch {
	case n == 0:
		return len(s)
	case n == 1:
		return bytealg.LastIndexByte(s, sep[0])
	case n == len(s):
		if Equal(s, sep) {
			return 0
		}
		return -1
	case n > len(s):
		return -1
	}
	return bytealg.LastIndexRabinKarp(s, sep)
}

// LastIndexByte returns the index of the last instance of c in s, or -1 if c is not present in s.
func LastIndexByte(s []byte, c byte) int {
	return bytealg.LastIndexByte(s, c)
}

// IndexRune interprets s as a sequence of UTF-8-encoded code points.
// It returns the byte index of the first occurrence in s of the given rune.
// It returns -1 if rune is not present in s.
// If r is [utf8.RuneError], it returns the first instance of any
// invalid UTF-8 byte sequence.
func IndexRune(s []byte, r rune) int {
	const haveFastIndex = bytealg.MaxBruteForce > 0
	switch {
	case 0 <= r && r < utf8.RuneSelf:
		return IndexByte(s, byte(r))
	case r == utf8.RuneError:
		for i := 0; i < len(s); {
			r1, n := utf8.DecodeRune(s[i:])
			if r1 == utf8.RuneError {
				return i
			}
			i += n
		}
		return -1
	case !utf8.ValidRune(r):
		return -1
	default:
		// Search for rune r using the last byte of its UTF-8 encoded form.
		// The distribution of the last byte is more uniform compared to the
		// first byte which has a 78% chance of being [240, 243, 244].
		var b [utf8.UTFMax]byte
		n := utf8.EncodeRune(b[:], r)
		last := n - 1
		i := last
		fails := 0
		for i < len(s) {
			if s[i] != b[last] {
				o := IndexByte(s[i+1:], b[last])
				if o < 0 {
					return -1
				}
				i += o + 1
			}
			// Step backwards comparing bytes.
			for j := 1; j < n; j++ {
				if s[i-j] != b[last-j] {
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
		// Switch to bytealg.Index, if available, or a brute force search when
		// IndexByte returns too many false positives.
		if haveFastIndex {
			if j := bytealg.Index(s[i-last:], b[:n]); j >= 0 {
				return i + j - last
			}
		} else {
			// If bytealg.Index is not available a brute force search is
			// ~1.5-3x faster than Rabin-Karp since n is small.
			c0 := b[last]
			c1 := b[last-1] // There are at least 2 chars to match
		loop:
			for ; i < len(s); i++ {
				if s[i] == c0 && s[i-1] == c1 {
					for k := 2; k < n; k++ {
						if s[i-k] != b[last-k] {
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

// IndexAny interprets s as a sequence of UTF-8-encoded Unicode code points.
// It returns the byte index of the first occurrence in s of any of the Unicode
// code points in chars. It returns -1 if chars is empty or if there is no code
// point in common.
func IndexAny(s []byte, chars string) int {
	if chars == "" {
		// Avoid scanning all of s.
		return -1
	}
	if len(s) == 1 {
		r := rune(s[0])
		if r >= utf8.RuneSelf {
			// search utf8.RuneError.
			for _, r = range chars {
				if r == utf8.RuneError {
					return 0
				}
			}
			return -1
		}
		if bytealg.IndexByteString(chars, s[0]) >= 0 {
			return 0
		}
		return -1
	}
	if len(chars) == 1 {
		r := rune(chars[0])
		if r >= utf8.RuneSelf {
			r = utf8.RuneError
		}
		return IndexRune(s, r)
	}
	if shouldUseASCIISet(len(s)) {
		if as, isASCII := makeASCIISet(chars); isASCII {
			for i, c := range s {
				if as.contains(c) {
					return i
				}
			}
			return -1
		}
	}
	var width int
	for i := 0; i < len(s); i += width {
		r := rune(s[i])
		if r < utf8.RuneSelf {
			if bytealg.IndexByteString(chars, s[i]) >= 0 {
				return i
			}
			width = 1
			continue
		}
		r, width = utf8.DecodeRune(s[i:])
		if r != utf8.RuneError {
			// r is 2 to 4 bytes
			if len(chars) == width {
				if chars == string(r) {
					return i
				}
				continue
			}
			// Use bytealg.IndexString for performance if available.
			if bytealg.MaxLen >= width {
				if bytealg.IndexString(chars, string(r)) >= 0 {
					return i
				}
				continue
			}
		}
		for _, ch := range chars {
			if r == ch {
				return i
			}
		}
	}
	return -1
}

// LastIndexAny interprets s as a sequence of UTF-8-encoded Unicode code
// points. It returns the byte index of the last occurrence in s of any of
// the Unicode code points in chars. It returns -1 if chars is empty or if
// there is no code point in common.
func LastIndexAny(s []byte, chars string) int {
	if chars == "" {
		// Avoid scanning all of s.
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
	if len(s) == 1 {
		r := rune(s[0])
		if r >= utf8.RuneSelf {
			for _, r = range chars {
				if r == utf8.RuneError {
					return 0
				}
			}
			return -1
		}
		if bytealg.IndexByteString(chars, s[0]) >= 0 {
			return 0
		}
		return -1
	}
	if len(chars) == 1 {
		cr := rune(chars[0])
		if cr >= utf8.RuneSelf {
			cr = utf8.RuneError
		}
		for i := len(s); i > 0; {
			r, size := utf8.DecodeLastRune(s[:i])
			i -= size
			if r == cr {
				return i
			}
		}
		return -1
	}
	for i := len(s); i > 0; {
		r := rune(s[i-1])
		if r < utf8.RuneSelf {
			if bytealg.IndexByteString(chars, s[i-1]) >= 0 {
				return i - 1
			}
			i--
			continue
		}
		r, size := utf8.DecodeLastRune(s[:i])
		i -= size
		if r != utf8.RuneError {
			// r is 2 to 4 bytes
			if len(chars) == size {
				if chars == string(r) {
					return i
				}
				continue
			}
			// Use bytealg.IndexString for performance if available.
			if bytealg.MaxLen >= size {
				if bytealg.IndexString(chars, string(r)) >= 0 {
					return i
				}
				continue
			}
		}
		for _, ch := range chars {
			if r == ch {
				return i
			}
		}
	}
	return -1
}

// Generic split: splits after each instance of sep,
// including sepSave bytes of sep in the subslices.
func genSplit(s, sep []byte, sepSave, n int) [][]byte {
	if n == 0 {
		return nil
	}
	if len(sep) == 0 {
		return explode(s, n)
	}
	if n < 0 {
		n = Count(s, sep) + 1
	}
	n = min(n, len(s)+1)

	a := make([][]byte, n)
	n--
	i := 0
	for i < n {
		m := Index(s, sep)
		if m < 0 {
			break
		}
		a[i] = s[: m+sepSave : m+sepSave]
		s = s[m+len(sep):]
		i++
	}
	a[i] = s
	return a[:i+1]
}

// SplitN slices s into subslices separated by sep and returns a slice of
// the subslices between those separators.
// If sep is empty, SplitN splits after each UTF-8 sequence.
// The count determines the number of subslices to return:
//   - n > 0: at most n subslices; the last subslice will be the unsplit remainder;
//   - n == 0: the result is nil (zero subslices);
//   - n < 0: all subslices.
//
// To split around the first instance of a separator, see [Cut].
func SplitN(s, sep []byte, n int) [][]byte { return genSplit(s, sep, 0, n) }

// SplitAfterN slices s into subslices after each instance of sep and
// returns a slice of those subslices.
// If sep is empty, SplitAfterN splits after each UTF-8 sequence.
// The count determines the number of subslices to return:
//   - n > 0: at most n subslices; the last subslice will be the unsplit remainder;
//   - n == 0: the result is nil (zero subslices);
//   - n < 0: all subslices.
func SplitAfterN(s, sep []byte, n int) [][]byte {
	return genSplit(s, sep, len(sep), n)
}

// Split slices s into all subslices separated by sep and returns a slice of
// the subslices between those separators.
// If sep is empty, Split splits after each UTF-8 sequence.
// It is equivalent to SplitN with a count of -1.
//
// To split around the first instance of a separator, see [Cut].
func Split(s, sep []byte) [][]byte { return genSplit(s, sep, 0, -1) }

// SplitAfter slices s into all subslices after each instance of sep and
// returns a slice of those subslices.
// If sep is empty, SplitAfter splits after each UTF-8 sequence.
// It is equivalent to SplitAfterN with a count of -1.
func SplitAfter(s, sep []byte) [][]byte {
	return genSplit(s, sep, len(sep), -1)
}

var asciiSpace = [256]uint8{'\t': 1, '\n': 1, '\v': 1, '\f': 1, '\r': 1, ' ': 1}

// Fields interprets s as a sequence of UTF-8-encoded code points.
// It splits the slice s around each instance of one or more consecutive white space
// characters, as defined by [unicode.IsSpace], returning a slice of subslices of s or an
// empty slice if s contains only white space. Every element of the returned slice is
// non-empty. Unlike [Split], leading and trailing runs of white space characters
// are discarded.
func Fields(s []byte) [][]byte {
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
		// Some runes in the input slice are not ASCII.
		return FieldsFunc(s, unicode.IsSpace)
	}

	// ASCII fast path
	a := make([][]byte, n)
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
		a[na] = s[fieldStart:i:i]
		na++
		i++
		// Skip spaces in between fields.
		for i < len(s) && asciiSpace[s[i]] != 0 {
			i++
		}
		fieldStart = i
	}
	if fieldStart < len(s) { // Last field might end at EOF.
		a[na] = s[fieldStart:len(s):len(s)]
	}
	return a
}

// FieldsFunc interprets s as a sequence of UTF-8-encoded code points.
// It splits the slice s at each run of code points c satisfying f(c) and
// returns a slice of subslices of s. If all code points in s satisfy f(c), or
// len(s) == 0, an empty slice is returned. Every element of the returned slice is
// non-empty. Unlike [Split], leading and trailing runs of code points
// satisfying f(c) are discarded.
//
// FieldsFunc makes no guarantees about the order in which it calls f(c)
// and assumes that f always returns the same value for a given c.
func FieldsFunc(s []byte, f func(rune) bool) [][]byte {
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
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRune(s[i:])
		if f(r) {
			if start >= 0 {
				spans = append(spans, span{start, i})
				start = -1
			}
		} else {
			if start < 0 {
				start = i
			}
		}
		i += size
	}

	// Last field might end at EOF.
	if start >= 0 {
		spans = append(spans, span{start, len(s)})
	}

	// Create subslices from recorded field indices.
	a := make([][]byte, len(spans))
	for i, span := range spans {
		a[i] = s[span.start:span.end:span.end]
	}

	return a
}

// Join concatenates the elements of s to create a new byte slice. The separator
// sep is placed between elements in the resulting slice.
func Join(s [][]byte, sep []byte) []byte {
	if len(s) == 0 {
		return []byte{}
	}
	if len(s) == 1 {
		// Just return a copy.
		return append([]byte(nil), s[0]...)
	}

	var n int
	if len(sep) > 0 {
		if len(sep) >= maxInt/(len(s)-1) {
			panic("bytes: Join output length overflow")
		}
		n += len(sep) * (len(s) - 1)
	}
	for _, v := range s {
		if len(v) > maxInt-n {
			panic("bytes: Join output length overflow")
		}
		n += len(v)
	}

	b := bytealg.MakeNoZero(n)[:n:n]
	bp := copy(b, s[0])
	for _, v := range s[1:] {
		bp += copy(b[bp:], sep)
		bp += copy(b[bp:], v)
	}
	return b
}

// HasPrefix reports whether the byte slice s begins with prefix.
func HasPrefix(s, prefix []byte) bool {
	return len(s) >= len(prefix) && Equal(s[:len(prefix)], prefix)
}

// HasSuffix reports whether the byte slice s ends with suffix.
func HasSuffix(s, suffix []byte) bool {
	return len(s) >= len(suffix) && Equal(s[len(s)-len(suffix):], suffix)
}

// Map returns a copy of the byte slice s with all its characters modified
// according to the mapping function. If mapping returns a negative value, the character is
// dropped from the byte slice with no replacement. The characters in s and the
// output are interpreted as UTF-8-encoded code points.
func Map(mapping func(r rune) rune, s []byte) []byte {
	// In the worst case, the slice can grow when mapped, making
	// things unpleasant. But it's so rare we barge in assuming it's
	// fine. It could also shrink but that falls out naturally.
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		r, wid := utf8.DecodeRune(s[i:])
		r = mapping(r)
		if r >= 0 {
			b = utf8.AppendRune(b, r)
		}
		i += wid
	}
	return b
}

// Despite being an exported symbol,
// Repeat is linknamed by widely used packages.
// Notable members of the hall of shame include:
//   - gitee.com/quant1x/num
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
// Note that this comment is not part of the doc comment.
//
//go:linkname Repeat

// Repeat returns a new byte slice consisting of count copies of b.
//
// It panics if count is negative or if the result of (len(b) * count)
// overflows.
func Repeat(b []byte, count int) []byte {
	if count == 0 {
		return []byte{}
	}

	// Since we cannot return an error on overflow,
	// we should panic if the repeat will generate an overflow.
	// See golang.org/issue/16237.
	if count < 0 {
		panic("bytes: negative Repeat count")
	}
	hi, lo := bits.Mul(uint(len(b)), uint(count))
	if hi > 0 || lo > uint(maxInt) {
		panic("bytes: Repeat output length overflow")
	}
	n := int(lo) // lo = len(b) * count

	if len(b) == 0 {
		return []byte{}
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
	if chunkMax > chunkLimit {
		chunkMax = chunkLimit / len(b) * len(b)
		if chunkMax == 0 {
			chunkMax = len(b)
		}
	}
	nb := bytealg.MakeNoZero(n)[:n:n]
	bp := copy(nb, b)
	for bp < n {
		chunk := min(bp, chunkMax)
		bp += copy(nb[bp:], nb[:chunk])
	}
	return nb
}

// ToUpper returns a copy of the byte slice s with all Unicode letters mapped to
// their upper case.
func ToUpper(s []byte) []byte {
	isASCII, hasLower := true, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= utf8.RuneSelf {
			isASCII = false
			break
		}
		hasLower = hasLower || ('a' <= c && c <= 'z')
	}

	if isASCII { // optimize for ASCII-only byte slices.
		if !hasLower {
			// Just return a copy.
			return append([]byte(""), s...)
		}
		b := bytealg.MakeNoZero(len(s))[:len(s):len(s)]
		for i := 0; i < len(s); i++ {
			c := s[i]
			if 'a' <= c && c <= 'z' {
				c -= 'a' - 'A'
			}
			b[i] = c
		}
		return b
	}
	return Map(unicode.ToUpper, s)
}

// ToLower returns a copy of the byte slice s with all Unicode letters mapped to
// their lower case.
func ToLower(s []byte) []byte {
	isASCII, hasUpper := true, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= utf8.RuneSelf {
			isASCII = false
			break
		}
		hasUpper = hasUpper || ('A' <= c && c <= 'Z')
	}

	if isASCII { // optimize for ASCII-only byte slices.
		if !hasUpper {
			return append([]byte(""), s...)
		}
		b := bytealg.MakeNoZero(len(s))[:len(s):len(s)]
		for i := 0; i < len(s); i++ {
			c := s[i]
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			}
			b[i] = c
		}
		return b
	}
	return Map(unicode.ToLower, s)
}

// ToTitle treats s as UTF-8-encoded bytes and returns a copy with all the Unicode letters mapped to their title case.
func ToTitle(s []byte) []byte { return Map(unicode.ToTitle, s) }

// ToUpperSpecial treats s as UTF-8-encoded bytes and returns a copy with all the Unicode letters mapped to their
// upper case, giving priority to the special casing rules.
func ToUpperSpecial(c unicode.SpecialCase, s []byte) []byte {
	return Map(c.ToUpper, s)
}

// ToLowerSpecial treats s as UTF-8-encoded bytes and returns a copy with all the Unicode letters mapped to their
// lower case, giving priority to the special casing rules.
func ToLowerSpecial(c unicode.SpecialCase, s []byte) []byte {
	return Map(c.ToLower, s)
}

// ToTitleSpecial treats s as UTF-8-encoded bytes and returns a copy with all the Unicode letters mapped to their
// title case, giving priority to the special casing rules.
func ToTitleSpecial(c unicode.SpecialCase, s []byte) []byte {
	return Map(c.ToTitle, s)
}

// ToValidUTF8 treats s as UTF-8-encoded bytes and returns a copy with each run of bytes
// representing invalid UTF-8 replaced with the bytes in replacement, which may be empty.
func ToValidUTF8(s, replacement []byte) []byte {
	b := make([]byte, 0, len(s)+len(replacement))
	invalid := false // previous byte was from an invalid UTF-8 sequence
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			i++
			invalid = false
			b = append(b, c)
			continue
		}
		_, wid := utf8.DecodeRune(s[i:])
		if wid == 1 {
			i++
			if !invalid {
				invalid = true
				b = append(b, replacement...)
			}
			continue
		}
		invalid = false
		b = append(b, s[i:i+wid]...)
		i += wid
	}
	return b
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

// Title treats s as UTF-8-encoded bytes and returns a copy with all Unicode letters that begin
// words mapped to their title case.
//
// Deprecated: The rule Title uses for word boundaries does not handle Unicode
// punctuation properly. Use golang.org/x/text/cases instead.
func Title(s []byte) []byte {
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

// TrimLeftFunc treats s as UTF-8-encoded bytes and returns a subslice of s by slicing off
// all leading UTF-8-encoded code points c that satisfy f(c).
func TrimLeftFunc(s []byte, f func(r rune) bool) []byte {
	i := indexFunc(s, f, false)
	if i == -1 {
		return nil
	}
	return s[i:]
}

// TrimRightFunc returns a subslice of s by slicing off all trailing
// UTF-8-encoded code points c that satisfy f(c).
func TrimRightFunc(s []byte, f func(r rune) bool) []byte {
	i := lastIndexFunc(s, f, false)
	if i >= 0 && s[i] >= utf8.RuneSelf {
		_, wid := utf8.DecodeRune(s[i:])
		i += wid
	} else {
		i++
	}
	return s[0:i]
}

// TrimFunc returns a subslice of s by slicing off all leading and trailing
// UTF-8-encoded code points c that satisfy f(c).
func TrimFunc(s []byte, f func(r rune) bool) []byte {
	return TrimRightFunc(TrimLeftFunc(s, f), f)
}

// TrimPrefix returns s without the provided leading prefix string.
// If s doesn't start with prefix, s is returned unchanged.
func TrimPrefix(s, prefix []byte) []byte {
	if HasPrefix(s, prefix) {
		return s[len(prefix):]
	}
	return s
}

// TrimSuffix returns s without the provided trailing suffix string.
// If s doesn't end with suffix, s is returned unchanged.
func TrimSuffix(s, suffix []byte) []byte {
	if HasSuffix(s, suffix) {
		return s[:len(s)-len(suffix)]
	}
	return s
}

// IndexFunc interprets s as a sequence of UTF-8-encoded code points.
// It returns the byte index in s of the first Unicode
// code point satisfying f(c), or -1 if none do.
func IndexFunc(s []byte, f func(r rune) bool) int {
	return indexFunc(s, f, true)
}

// LastIndexFunc interprets s as a sequence of UTF-8-encoded code points.
// It returns the byte index in s of the last Unicode
// code point satisfying f(c), or -1 if none do.
func LastIndexFunc(s []byte, f func(r rune) bool) int {
	return lastIndexFunc(s, f, true)
}

// indexFunc is the same as IndexFunc except that if
// truth==false, the sense of the predicate function is
// inverted.
func indexFunc(s []byte, f func(r rune) bool, truth bool) int {
	start := 0
	for start < len(s) {
		r, wid := utf8.DecodeRune(s[start:])
		if f(r) == truth {
			return start
		}
		start += wid
	}
	return -1
}

// lastIndexFunc is the same as LastIndexFunc except that if
// truth==false, the sense of the predicate function is
// inverted.
func lastIndexFunc(s []byte, f func(r rune) bool, truth bool) int {
	for i := len(s); i > 0; {
		r, size := rune(s[i-1]), 1
		if r >= utf8.RuneSelf {
			r, size = utf8.DecodeLastRune(s[0:i])
		}
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

// containsRune is a simplified version of strings.ContainsRune
// to avoid importing the strings package.
// We avoid bytes.ContainsRune to avoid allocating a temporary copy of s.
func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}

// Trim returns a subslice of s by slicing off all leading and
// trailing UTF-8-encoded code points contained in cutset.
func Trim(s []byte, cutset string) []byte {
	if len(s) == 0 {
		// This is what we've historically done.
		return nil
	}
	if cutset == "" {
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

// TrimLeft returns a subslice of s by slicing off all leading
// UTF-8-encoded code points contained in cutset.
func TrimLeft(s []byte, cutset string) []byte {
	if len(s) == 0 {
		// This is what we've historically done.
		return nil
	}
	if cutset == "" {
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

func trimLeftByte(s []byte, c byte) []byte {
	for len(s) > 0 && s[0] == c {
		s = s[1:]
	}
	if len(s) == 0 {
		// This is what we've historically done.
		return nil
	}
	return s
}

func trimLeftASCII(s []byte, as *asciiSet) []byte {
	for len(s) > 0 {
		if !as.contains(s[0]) {
			break
		}
		s = s[1:]
	}
	if len(s) == 0 {
		// This is what we've historically done.
		return nil
	}
	return s
}

func trimLeftUnicode(s []byte, cutset string) []byte {
	for len(s) > 0 {
		r, n := utf8.DecodeRune(s)
		if !containsRune(cutset, r) {
			break
		}
		s = s[n:]
	}
	if len(s) == 0 {
		// This is what we've historically done.
		return nil
	}
	return s
}

// TrimRight returns a subslice of s by slicing off all trailing
// UTF-8-encoded code points that are contained in cutset.
func TrimRight(s []byte, cutset string) []byte {
	if len(s) == 0 || cutset == "" {
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

func trimRightByte(s []byte, c byte) []byte {
	for len(s) > 0 && s[len(s)-1] == c {
		s = s[:len(s)-1]
	}
	return s
}

func trimRightASCII(s []byte, as *asciiSet) []byte {
	for len(s) > 0 {
		if !as.contains(s[len(s)-1]) {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}

func trimRightUnicode(s []byte, cutset string) []byte {
	for len(s) > 0 {
		r, n := rune(s[len(s)-1]), 1
		if r >= utf8.RuneSelf {
			r, n = utf8.DecodeLastRune(s)
		}
		if !containsRune(cutset, r) {
			break
		}
		s = s[:len(s)-n]
	}
	return s
}

// TrimSpace returns a subslice of s by slicing off all leading and
// trailing white space, as defined by Unicode.
func TrimSpace(s []byte) []byte {
	// Fast path for ASCII: look for the first ASCII non-space byte.
	for lo, c := range s {
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
				return TrimFunc(s[:hi+1], unicode.IsSpace)
			}
			if asciiSpace[c] == 0 {
				// At this point, s[:hi+1] starts and ends with ASCII
				// non-space bytes, so we're done. Non-ASCII cases have
				// already been handled above.
				return s[:hi+1]
			}
		}
	}
	// Special case to preserve previous TrimLeftFunc behavior,
	// returning nil instead of empty slice if all spaces.
	return nil
}

// Runes interprets s as a sequence of UTF-8-encoded code points.
// It returns a slice of runes (Unicode code points) equivalent to s.
func Runes(s []byte) []rune {
	t := make([]rune, utf8.RuneCount(s))
	i := 0
	for len(s) > 0 {
		r, l := utf8.DecodeRune(s)
		t[i] = r
		i++
		s = s[l:]
	}
	return t
}

// Replace returns a copy of the slice s with the first n
// non-overlapping instances of old replaced by new.
// If old is empty, it matches at the beginning of the slice
// and after each UTF-8 sequence, yielding up to k+1 replacements
// for a k-rune slice.
// If n < 0, there is no limit on the number of replacements.
func Replace(s, old, new []byte, n int) []byte {
	m := 0
	if n != 0 {
		// Compute number of replacements.
		m = Count(s, old)
	}
	if m == 0 {
		// Just return a copy.
		return append([]byte(nil), s...)
	}
	if n < 0 || m < n {
		n = m
	}

	// Apply replacements to buffer.
	t := make([]byte, len(s)+n*(len(new)-len(old)))
	w := 0
	start := 0
	if len(old) > 0 {
		for range n {
			j := start + Index(s[start:], old)
			w += copy(t[w:], s[start:j])
			w += copy(t[w:], new)
			start = j + len(old)
		}
	} else { // len(old) == 0
		w += copy(t[w:], new)
		for range n - 1 {
			_, wid := utf8.DecodeRune(s[start:])
			j := start + wid
			w += copy(t[w:], s[start:j])
			w += copy(t[w:], new)
			start = j
		}
	}
	w += copy(t[w:], s[start:])
	return t[0:w]
}

// ReplaceAll returns a copy of the slice s with all
// non-overlapping instances of old replaced by new.
// If old is empty, it matches at the beginning of the slice
// and after each UTF-8 sequence, yielding up to k+1 replacements
// for a k-rune slice.
func ReplaceAll(s, old, new []byte) []byte {
	return Replace(s, old, new, -1)
}

// EqualFold reports whether s and t, interpreted as UTF-8 strings,
// are equal under simple Unicode case-folding, which is a more general
// form of case-insensitivity.
func EqualFold(s, t []byte) bool {
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
	for len(s) != 0 && len(t) != 0 {
		// Extract first rune from each.
		sr, size := utf8.DecodeRune(s)
		s = s[size:]
		tr, size := utf8.DecodeRune(t)
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

	// One string is empty. Are both?
	return len(s) == len(t)
}

// Index returns the index of the first instance of sep in s, or -1 if sep is not present in s.
func Index(s, sep []byte) int {
	n := len(sep)
	switch {
	case n == 0:
		return 0
	case n == 1:
		return IndexByte(s, sep[0])
	case n == len(s):
		if Equal(sep, s) {
			return 0
		}
		return -1
	case n > len(s):
		return -1
	case n <= bytealg.MaxLen:
		// Use brute force when s and sep both are small
		if len(s) <= bytealg.MaxBruteForce {
			return bytealg.Index(s, sep)
		}
		c0 := sep[0]
		c1 := sep[1]
		i := 0
		t := len(s) - n + 1
		fails := 0
		for i < t {
			if s[i] != c0 {
				// IndexByte is faster than bytealg.Index, so use it as long as
				// we're not getting lots of false positives.
				o := IndexByte(s[i+1:t], c0)
				if o < 0 {
					return -1
				}
				i += o + 1
			}
			if s[i+1] == c1 && Equal(s[i:i+n], sep) {
				return i
			}
			fails++
			i++
			// Switch to bytealg.Index when IndexByte produces too many false positives.
			if fails > bytealg.Cutover(i) {
				r := bytealg.Index(s[i:], sep)
				if r >= 0 {
					return r + i
				}
				return -1
			}
		}
		return -1
	}
	c0 := sep[0]
	c1 := sep[1]
	i := 0
	fails := 0
	t := len(s) - n + 1
	for i < t {
		if s[i] != c0 {
			o := IndexByte(s[i+1:t], c0)
			if o < 0 {
				break
			}
			i += o + 1
		}
		if s[i+1] == c1 && Equal(s[i:i+n], sep) {
			return i
		}
		i++
		fails++
		if fails >= 4+i>>4 && i < t {
			// Give up on IndexByte, it isn't skipping ahead
			// far enough to be better than Rabin-Karp.
			// Experiments (using IndexPeriodic) suggest
			// the cutover is about 16 byte skips.
			// TODO: if large prefixes of sep are matching
			// we should cutover at even larger average skips,
			// because Equal becomes that much more expensive.
			// This code does not take that effect into account.
			j := bytealg.IndexRabinKarp(s[i:], sep)
			if j < 0 {
				return -1
			}
			return i + j
		}
	}
	return -1
}

// Cut slices s around the first instance of sep,
// returning the text before and after sep.
// The found result reports whether sep appears in s.
// If sep does not appear in s, cut returns s, nil, false.
//
// Cut returns slices of the original slice s, not copies.
func Cut(s, sep []byte) (before, after []byte, found bool) {
	if i := Index(s, sep); i >= 0 {
		return s[:i], s[i+len(sep):], true
	}
	return s, nil, false
}

// Clone returns a copy of b[:len(b)].
// The result may have additional unused capacity.
// Clone(nil) returns nil.
func Clone(b []byte) []byte {
	if b == nil {
		return nil
	}
	return append([]byte{}, b...)
}

// CutPrefix returns s without the provided leading prefix byte slice
// and reports whether it found the prefix.
// If s doesn't start with prefix, CutPrefix returns s, false.
// If prefix is the empty byte slice, CutPrefix returns s, true.
//
// CutPrefix returns slices of the original slice s, not copies.
func CutPrefix(s, prefix []byte) (after []byte, found bool) {
	if !HasPrefix(s, prefix) {
		return s, false
	}
	return s[len(prefix):], true
}

// CutSuffix returns s without the provided ending suffix byte slice
// and reports whether it found the suffix.
// If s doesn't end with suffix, CutSuffix returns s, false.
// If suffix is the empty byte slice, CutSuffix returns s, true.
//
// CutSuffix returns slices of the original slice s, not copies.
func CutSuffix(s, suffix []byte) (before []byte, found bool) {
	if !HasSuffix(s, suffix) {
		return s, false
	}
	return s[:len(s)-len(suffix)], true
}

// CutLast slices s around the last instance of sep,
// returning the text before and after sep.
// The found result reports whether sep appears in s.
// If sep does not appear in s, CutLast returns s, nil, false.
//
// CutLast returns slices of the original slice s, not copies.
func CutLast(s, sep []byte) (before, after []byte, found bool) {
	if i := LastIndex(s, sep); i >= 0 {
		return s[:i], s[i+len(sep):], true
	}
	return s, nil, false
}

```

// === FILE: references!/go/src/bytes/iter.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bytes

import (
	"iter"
	"unicode"
	"unicode/utf8"
)

// Lines returns an iterator over the newline-terminated lines in the byte slice s.
// The lines yielded by the iterator include their terminating newlines.
// If s is empty, the iterator yields no lines at all.
// If s does not end in a newline, the final yielded line will not end in a newline.
// It returns a single-use iterator.
func Lines(s []byte) iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		for len(s) > 0 {
			var line []byte
			if i := IndexByte(s, '\n'); i >= 0 {
				line, s = s[:i+1], s[i+1:]
			} else {
				line, s = s, nil
			}
			if !yield(line[:len(line):len(line)]) {
				return
			}
		}
	}
}

// splitSeq is SplitSeq or SplitAfterSeq, configured by how many
// bytes of sep to include in the results (none or all).
func splitSeq(s, sep []byte, sepSave int) iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		if len(sep) == 0 {
			for len(s) > 0 {
				_, size := utf8.DecodeRune(s)
				if !yield(s[:size:size]) {
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
			if !yield(frag[:len(frag):len(frag)]) {
				return
			}
			s = s[i+len(sep):]
		}
		yield(s[:len(s):len(s)])
	}
}

// SplitSeq returns an iterator over all subslices of s separated by sep.
// The iterator yields the same subslices that would be returned by [Split](s, sep),
// but without constructing a new slice containing the subslices.
// It returns a single-use iterator.
func SplitSeq(s, sep []byte) iter.Seq[[]byte] {
	return splitSeq(s, sep, 0)
}

// SplitAfterSeq returns an iterator over subslices of s split after each instance of sep.
// The iterator yields the same subslices that would be returned by [SplitAfter](s, sep),
// but without constructing a new slice containing the subslices.
// It returns a single-use iterator.
func SplitAfterSeq(s, sep []byte) iter.Seq[[]byte] {
	return splitSeq(s, sep, len(sep))
}

// FieldsSeq returns an iterator over subslices of s split around runs of
// whitespace characters, as defined by [unicode.IsSpace].
// The iterator yields the same subslices that would be returned by [Fields](s),
// but without constructing a new slice containing the subslices.
func FieldsSeq(s []byte) iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		start := -1
		for i := 0; i < len(s); {
			size := 1
			r := rune(s[i])
			isSpace := asciiSpace[s[i]] != 0
			if r >= utf8.RuneSelf {
				r, size = utf8.DecodeRune(s[i:])
				isSpace = unicode.IsSpace(r)
			}
			if isSpace {
				if start >= 0 {
					if !yield(s[start:i:i]) {
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
			yield(s[start:len(s):len(s)])
		}
	}
}

// FieldsFuncSeq returns an iterator over subslices of s split around runs of
// Unicode code points satisfying f(c).
// The iterator yields the same subslices that would be returned by [FieldsFunc](s),
// but without constructing a new slice containing the subslices.
func FieldsFuncSeq(s []byte, f func(rune) bool) iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		start := -1
		for i := 0; i < len(s); {
			r, size := utf8.DecodeRune(s[i:])
			if f(r) {
				if start >= 0 {
					if !yield(s[start:i:i]) {
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
			yield(s[start:len(s):len(s)])
		}
	}
}

```

// === FILE: references!/go/src/bytes/reader.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bytes

import (
	"errors"
	"io"
	"unicode/utf8"
)

// A Reader implements the [io.Reader], [io.ReaderAt], [io.WriterTo], [io.Seeker],
// [io.ByteScanner], and [io.RuneScanner] interfaces by reading from
// a byte slice.
// Unlike a [Buffer], a Reader is read-only and supports seeking.
// The zero value for Reader operates like a Reader of an empty slice.
type Reader struct {
	s        []byte
	i        int64 // current reading index
	prevRune int   // index of previous rune; or < 0
}

// Len returns the number of bytes of the unread portion of the
// slice.
func (r *Reader) Len() int {
	if r.i >= int64(len(r.s)) {
		return 0
	}
	return int(int64(len(r.s)) - r.i)
}

// Size returns the original length of the underlying byte slice.
// Size is the number of bytes available for reading via [Reader.ReadAt].
// The result is unaffected by any method calls except [Reader.Reset].
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
		return 0, errors.New("bytes.Reader.ReadAt: negative offset")
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

// UnreadByte complements [Reader.ReadByte] in implementing the [io.ByteScanner] interface.
func (r *Reader) UnreadByte() error {
	if r.i <= 0 {
		return errors.New("bytes.Reader.UnreadByte: at beginning of slice")
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
	if c := r.s[r.i]; c < utf8.RuneSelf {
		r.i++
		return rune(c), 1, nil
	}
	ch, size = utf8.DecodeRune(r.s[r.i:])
	r.i += int64(size)
	return
}

// UnreadRune complements [Reader.ReadRune] in implementing the [io.RuneScanner] interface.
func (r *Reader) UnreadRune() error {
	if r.i <= 0 {
		return errors.New("bytes.Reader.UnreadRune: at beginning of slice")
	}
	if r.prevRune < 0 {
		return errors.New("bytes.Reader.UnreadRune: previous operation was not ReadRune")
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
		return 0, errors.New("bytes.Reader.Seek: invalid whence")
	}
	if abs < 0 {
		return 0, errors.New("bytes.Reader.Seek: negative position")
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
	b := r.s[r.i:]
	m, err := w.Write(b)
	if m > len(b) {
		panic("bytes.Reader.WriteTo: invalid Write count")
	}
	r.i += int64(m)
	n = int64(m)
	if m != len(b) && err == nil {
		err = io.ErrShortWrite
	}
	return
}

// Reset resets the [Reader] to be reading from b.
func (r *Reader) Reset(b []byte) { *r = Reader{b, 0, -1} }

// NewReader returns a new [Reader] reading from b.
func NewReader(b []byte) *Reader { return &Reader{b, 0, -1} }

```

