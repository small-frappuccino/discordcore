# Domain Architecture: compress

## Layout Topology
```text
compress/
├── bzip2
│   ├── bit_reader.go
│   ├── bzip2.go
│   ├── huffman.go
│   └── move_to_front.go
├── flate
│   ├── deflate.go
│   ├── deflatefast.go
│   ├── dict_decoder.go
│   ├── huffman_bit_writer.go
│   ├── huffman_code.go
│   ├── inflate.go
│   ├── level1.go
│   ├── level2.go
│   ├── level3.go
│   ├── level4.go
│   ├── level5.go
│   ├── level6.go
│   ├── load_store.go
│   ├── regmask_amd64.go
│   ├── regmask_other.go
│   └── token.go
├── gzip
│   ├── gunzip.go
│   └── gzip.go
├── lzw
│   ├── reader.go
│   └── writer.go
└── zlib
    ├── reader.go
    └── writer.go
```

## Source Stream Aggregation

// === FILE: references/go/src/compress/bzip2/bit_reader.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bzip2

import (
	"bufio"
	"io"
)

// bitReader wraps an io.Reader and provides the ability to read values,
// bit-by-bit, from it. Its Read* methods don't return the usual error
// because the error handling was verbose. Instead, any error is kept and can
// be checked afterwards.
type bitReader struct {
	r    io.ByteReader
	n    uint64
	bits uint
	err  error
}

// newBitReader returns a new bitReader reading from r. If r is not
// already an io.ByteReader, it will be converted via a bufio.Reader.
func newBitReader(r io.Reader) bitReader {
	byter, ok := r.(io.ByteReader)
	if !ok {
		byter = bufio.NewReader(r)
	}
	return bitReader{r: byter}
}

// ReadBits64 reads the given number of bits and returns them in the
// least-significant part of a uint64. In the event of an error, it returns 0
// and the error can be obtained by calling bitReader.Err().
func (br *bitReader) ReadBits64(bits uint) (n uint64) {
	for bits > br.bits {
		b, err := br.r.ReadByte()
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		if err != nil {
			br.err = err
			return 0
		}
		br.n <<= 8
		br.n |= uint64(b)
		br.bits += 8
	}

	// br.n looks like this (assuming that br.bits = 14 and bits = 6):
	// Bit: 111111
	//      5432109876543210
	//
	//         (6 bits, the desired output)
	//        |-----|
	//        V     V
	//      0101101101001110
	//        ^            ^
	//        |------------|
	//           br.bits (num valid bits)
	//
	// The next line right shifts the desired bits into the
	// least-significant places and masks off anything above.
	n = (br.n >> (br.bits - bits)) & ((1 << bits) - 1)
	br.bits -= bits
	return
}

func (br *bitReader) ReadBits(bits uint) (n int) {
	n64 := br.ReadBits64(bits)
	return int(n64)
}

func (br *bitReader) ReadBit() bool {
	n := br.ReadBits(1)
	return n != 0
}

func (br *bitReader) Err() error {
	return br.err
}

```

// === FILE: references/go/src/compress/bzip2/bzip2.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package bzip2 implements bzip2 decompression.
package bzip2

import "io"

// There's no RFC for bzip2. I used the Wikipedia page for reference and a lot
// of guessing: https://en.wikipedia.org/wiki/Bzip2
// The source code to pyflate was useful for debugging:
// http://www.paul.sladen.org/projects/pyflate

// A StructuralError is returned when the bzip2 data is found to be
// syntactically invalid.
type StructuralError string

func (s StructuralError) Error() string {
	return "bzip2 data invalid: " + string(s)
}

// A reader decompresses bzip2 compressed data.
type reader struct {
	br           bitReader
	fileCRC      uint32
	blockCRC     uint32
	wantBlockCRC uint32
	setupDone    bool // true if we have parsed the bzip2 header.
	eof          bool
	blockSize    int       // blockSize in bytes, i.e. 900 * 1000.
	c            [256]uint // the ``C'' array for the inverse BWT.
	tt           []uint32  // mirrors the ``tt'' array in the bzip2 source and contains the P array in the upper 24 bits.
	tPos         uint32    // Index of the next output byte in tt.

	preRLE      []uint32 // contains the RLE data still to be processed.
	preRLEUsed  int      // number of entries of preRLE used.
	lastByte    int      // the last byte value seen.
	byteRepeats uint     // the number of repeats of lastByte seen.
	repeats     uint     // the number of copies of lastByte to output.
}

// NewReader returns an [io.Reader] which decompresses bzip2 data from r.
// If r does not also implement [io.ByteReader],
// the decompressor may read more data than necessary from r.
func NewReader(r io.Reader) io.Reader {
	bz2 := new(reader)
	bz2.br = newBitReader(r)
	return bz2
}

const bzip2FileMagic = 0x425a // "BZ"
const bzip2BlockMagic = 0x314159265359
const bzip2FinalMagic = 0x177245385090

// setup parses the bzip2 header.
func (bz2 *reader) setup(needMagic bool) error {
	br := &bz2.br

	if needMagic {
		magic := br.ReadBits(16)
		if magic != bzip2FileMagic {
			return StructuralError("bad magic value")
		}
	}

	t := br.ReadBits(8)
	if t != 'h' {
		return StructuralError("non-Huffman entropy encoding")
	}

	level := br.ReadBits(8)
	if level < '1' || level > '9' {
		return StructuralError("invalid compression level")
	}

	bz2.fileCRC = 0
	bz2.blockSize = 100 * 1000 * (level - '0')
	if bz2.blockSize > len(bz2.tt) {
		bz2.tt = make([]uint32, bz2.blockSize)
	}
	return nil
}

func (bz2 *reader) Read(buf []byte) (n int, err error) {
	if bz2.eof {
		return 0, io.EOF
	}

	if !bz2.setupDone {
		err = bz2.setup(true)
		brErr := bz2.br.Err()
		if brErr != nil {
			err = brErr
		}
		if err != nil {
			return 0, err
		}
		bz2.setupDone = true
	}

	n, err = bz2.read(buf)
	brErr := bz2.br.Err()
	if brErr != nil {
		err = brErr
	}
	return
}

func (bz2 *reader) readFromBlock(buf []byte) int {
	// bzip2 is a block based compressor, except that it has a run-length
	// preprocessing step. The block based nature means that we can
	// preallocate fixed-size buffers and reuse them. However, the RLE
	// preprocessing would require allocating huge buffers to store the
	// maximum expansion. Thus we process blocks all at once, except for
	// the RLE which we decompress as required.
	n := 0
	for (bz2.repeats > 0 || bz2.preRLEUsed < len(bz2.preRLE)) && n < len(buf) {
		// We have RLE data pending.

		// The run-length encoding works like this:
		// Any sequence of four equal bytes is followed by a length
		// byte which contains the number of repeats of that byte to
		// include. (The number of repeats can be zero.) Because we are
		// decompressing on-demand our state is kept in the reader
		// object.

		if bz2.repeats > 0 {
			buf[n] = byte(bz2.lastByte)
			n++
			bz2.repeats--
			if bz2.repeats == 0 {
				bz2.lastByte = -1
			}
			continue
		}

		bz2.tPos = bz2.preRLE[bz2.tPos]
		b := byte(bz2.tPos)
		bz2.tPos >>= 8
		bz2.preRLEUsed++

		if bz2.byteRepeats == 3 {
			bz2.repeats = uint(b)
			bz2.byteRepeats = 0
			continue
		}

		if bz2.lastByte == int(b) {
			bz2.byteRepeats++
		} else {
			bz2.byteRepeats = 0
		}
		bz2.lastByte = int(b)

		buf[n] = b
		n++
	}

	return n
}

func (bz2 *reader) read(buf []byte) (int, error) {
	for {
		n := bz2.readFromBlock(buf)
		if n > 0 || len(buf) == 0 {
			bz2.blockCRC = updateCRC(bz2.blockCRC, buf[:n])
			return n, nil
		}

		// End of block. Check CRC.
		if bz2.blockCRC != bz2.wantBlockCRC {
			bz2.br.err = StructuralError("block checksum mismatch")
			return 0, bz2.br.err
		}

		// Find next block.
		br := &bz2.br
		switch br.ReadBits64(48) {
		default:
			return 0, StructuralError("bad magic value found")

		case bzip2BlockMagic:
			// Start of block.
			err := bz2.readBlock()
			if err != nil {
				return 0, err
			}

		case bzip2FinalMagic:
			// Check end-of-file CRC.
			wantFileCRC := uint32(br.ReadBits64(32))
			if br.err != nil {
				return 0, br.err
			}
			if bz2.fileCRC != wantFileCRC {
				br.err = StructuralError("file checksum mismatch")
				return 0, br.err
			}

			// Skip ahead to byte boundary.
			// Is there a file concatenated to this one?
			// It would start with BZ.
			if br.bits%8 != 0 {
				br.ReadBits(br.bits % 8)
			}
			b, err := br.r.ReadByte()
			if err == io.EOF {
				br.err = io.EOF
				bz2.eof = true
				return 0, io.EOF
			}
			if err != nil {
				br.err = err
				return 0, err
			}
			z, err := br.r.ReadByte()
			if err != nil {
				if err == io.EOF {
					err = io.ErrUnexpectedEOF
				}
				br.err = err
				return 0, err
			}
			if b != 'B' || z != 'Z' {
				return 0, StructuralError("bad magic value in continuation file")
			}
			if err := bz2.setup(false); err != nil {
				return 0, err
			}
		}
	}
}

// readBlock reads a bzip2 block. The magic number should already have been consumed.
func (bz2 *reader) readBlock() (err error) {
	br := &bz2.br
	bz2.wantBlockCRC = uint32(br.ReadBits64(32)) // skip checksum. TODO: check it if we can figure out what it is.
	bz2.blockCRC = 0
	bz2.fileCRC = (bz2.fileCRC<<1 | bz2.fileCRC>>31) ^ bz2.wantBlockCRC
	randomized := br.ReadBits(1)
	if randomized != 0 {
		return StructuralError("deprecated randomized files")
	}
	origPtr := uint(br.ReadBits(24))

	// If not every byte value is used in the block (i.e., it's text) then
	// the symbol set is reduced. The symbols used are stored as a
	// two-level, 16x16 bitmap.
	symbolRangeUsedBitmap := br.ReadBits(16)
	symbolPresent := make([]bool, 256)
	numSymbols := 0
	for symRange := uint(0); symRange < 16; symRange++ {
		if symbolRangeUsedBitmap&(1<<(15-symRange)) != 0 {
			bits := br.ReadBits(16)
			for symbol := uint(0); symbol < 16; symbol++ {
				if bits&(1<<(15-symbol)) != 0 {
					symbolPresent[16*symRange+symbol] = true
					numSymbols++
				}
			}
		}
	}

	if numSymbols == 0 {
		// There must be an EOF symbol.
		return StructuralError("no symbols in input")
	}

	// A block uses between two and six different Huffman trees.
	numHuffmanTrees := br.ReadBits(3)
	if numHuffmanTrees < 2 || numHuffmanTrees > 6 {
		return StructuralError("invalid number of Huffman trees")
	}

	// The Huffman tree can switch every 50 symbols so there's a list of
	// tree indexes telling us which tree to use for each 50 symbol block.
	numSelectors := br.ReadBits(15)
	treeIndexes := make([]uint8, numSelectors)

	// The tree indexes are move-to-front transformed and stored as unary
	// numbers.
	mtfTreeDecoder := newMTFDecoderWithRange(numHuffmanTrees)
	for i := range treeIndexes {
		c := 0
		for {
			inc := br.ReadBits(1)
			if inc == 0 {
				break
			}
			c++
		}
		if c >= numHuffmanTrees {
			return StructuralError("tree index too large")
		}
		treeIndexes[i] = mtfTreeDecoder.Decode(c)
	}

	// The list of symbols for the move-to-front transform is taken from
	// the previously decoded symbol bitmap.
	symbols := make([]byte, numSymbols)
	nextSymbol := 0
	for i := 0; i < 256; i++ {
		if symbolPresent[i] {
			symbols[nextSymbol] = byte(i)
			nextSymbol++
		}
	}
	mtf := newMTFDecoder(symbols)

	numSymbols += 2 // to account for RUNA and RUNB symbols
	huffmanTrees := make([]huffmanTree, numHuffmanTrees)

	// Now we decode the arrays of code-lengths for each tree.
	lengths := make([]uint8, numSymbols)
	for i := range huffmanTrees {
		// The code lengths are delta encoded from a 5-bit base value.
		length := br.ReadBits(5)
		for j := range lengths {
			for {
				if length < 1 || length > 20 {
					return StructuralError("Huffman length out of range")
				}
				if !br.ReadBit() {
					break
				}
				if br.ReadBit() {
					length--
				} else {
					length++
				}
			}
			lengths[j] = uint8(length)
		}
		huffmanTrees[i], err = newHuffmanTree(lengths)
		if err != nil {
			return err
		}
	}

	selectorIndex := 1 // the next tree index to use
	if len(treeIndexes) == 0 {
		return StructuralError("no tree selectors given")
	}
	if int(treeIndexes[0]) >= len(huffmanTrees) {
		return StructuralError("tree selector out of range")
	}
	currentHuffmanTree := huffmanTrees[treeIndexes[0]]
	bufIndex := 0 // indexes bz2.buf, the output buffer.
	// The output of the move-to-front transform is run-length encoded and
	// we merge the decoding into the Huffman parsing loop. These two
	// variables accumulate the repeat count. See the Wikipedia page for
	// details.
	repeat := 0
	repeatPower := 0

	// The `C' array (used by the inverse BWT) needs to be zero initialized.
	clear(bz2.c[:])

	decoded := 0 // counts the number of symbols decoded by the current tree.
	for {
		if decoded == 50 {
			if selectorIndex >= numSelectors {
				return StructuralError("insufficient selector indices for number of symbols")
			}
			if int(treeIndexes[selectorIndex]) >= len(huffmanTrees) {
				return StructuralError("tree selector out of range")
			}
			currentHuffmanTree = huffmanTrees[treeIndexes[selectorIndex]]
			selectorIndex++
			decoded = 0
		}

		v := currentHuffmanTree.Decode(br)
		decoded++

		if v < 2 {
			// This is either the RUNA or RUNB symbol.
			if repeat == 0 {
				repeatPower = 1
			}
			repeat += repeatPower << v
			repeatPower <<= 1

			// This limit of 2 million comes from the bzip2 source
			// code. It prevents repeat from overflowing.
			if repeat > 2*1024*1024 {
				return StructuralError("repeat count too large")
			}
			continue
		}

		if repeat > 0 {
			// We have decoded a complete run-length so we need to
			// replicate the last output symbol.
			if repeat > bz2.blockSize-bufIndex {
				return StructuralError("repeats past end of block")
			}
			for i := 0; i < repeat; i++ {
				b := mtf.First()
				bz2.tt[bufIndex] = uint32(b)
				bz2.c[b]++
				bufIndex++
			}
			repeat = 0
		}

		if int(v) == numSymbols-1 {
			// This is the EOF symbol. Because it's always at the
			// end of the move-to-front list, and never gets moved
			// to the front, it has this unique value.
			break
		}

		// Since two metasymbols (RUNA and RUNB) have values 0 and 1,
		// one would expect |v-2| to be passed to the MTF decoder.
		// However, the front of the MTF list is never referenced as 0,
		// it's always referenced with a run-length of 1. Thus 0
		// doesn't need to be encoded and we have |v-1| in the next
		// line.
		b := mtf.Decode(int(v - 1))
		if bufIndex >= bz2.blockSize {
			return StructuralError("data exceeds block size")
		}
		bz2.tt[bufIndex] = uint32(b)
		bz2.c[b]++
		bufIndex++
	}

	if origPtr >= uint(bufIndex) {
		return StructuralError("origPtr out of bounds")
	}

	// We have completed the entropy decoding. Now we can perform the
	// inverse BWT and setup the RLE buffer.
	bz2.preRLE = bz2.tt[:bufIndex]
	bz2.preRLEUsed = 0
	bz2.tPos = inverseBWT(bz2.preRLE, origPtr, bz2.c[:])
	bz2.lastByte = -1
	bz2.byteRepeats = 0
	bz2.repeats = 0

	return nil
}

// inverseBWT implements the inverse Burrows-Wheeler transform as described in
// http://www.hpl.hp.com/techreports/Compaq-DEC/SRC-RR-124.pdf, section 4.2.
// In that document, origPtr is called “I” and c is the “C” array after the
// first pass over the data. It's an argument here because we merge the first
// pass with the Huffman decoding.
//
// This also implements the “single array” method from the bzip2 source code
// which leaves the output, still shuffled, in the bottom 8 bits of tt with the
// index of the next byte in the top 24-bits. The index of the first byte is
// returned.
func inverseBWT(tt []uint32, origPtr uint, c []uint) uint32 {
	sum := uint(0)
	for i := 0; i < 256; i++ {
		sum += c[i]
		c[i] = sum - c[i]
	}

	for i := range tt {
		b := tt[i] & 0xff
		tt[c[b]] |= uint32(i) << 8
		c[b]++
	}

	return tt[origPtr] >> 8
}

// This is a standard CRC32 like in hash/crc32 except that all the shifts are reversed,
// causing the bits in the input to be processed in the reverse of the usual order.

var crctab [256]uint32

func init() {
	const poly = 0x04C11DB7
	for i := range crctab {
		crc := uint32(i) << 24
		for j := 0; j < 8; j++ {
			if crc&0x80000000 != 0 {
				crc = (crc << 1) ^ poly
			} else {
				crc <<= 1
			}
		}
		crctab[i] = crc
	}
}

// updateCRC updates the crc value to incorporate the data in b.
// The initial value is 0.
func updateCRC(val uint32, b []byte) uint32 {
	crc := ^val
	for _, v := range b {
		crc = crctab[byte(crc>>24)^v] ^ (crc << 8)
	}
	return ^crc
}

```

// === FILE: references/go/src/compress/bzip2/huffman.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bzip2

import (
	"cmp"
	"slices"
)

// A huffmanTree is a binary tree which is navigated, bit-by-bit to reach a
// symbol.
type huffmanTree struct {
	// nodes contains all the non-leaf nodes in the tree. nodes[0] is the
	// root of the tree and nextNode contains the index of the next element
	// of nodes to use when the tree is being constructed.
	nodes    []huffmanNode
	nextNode int
}

// A huffmanNode is a node in the tree. left and right contain indexes into the
// nodes slice of the tree. If left or right is invalidNodeValue then the child
// is a left node and its value is in leftValue/rightValue.
//
// The symbols are uint16s because bzip2 encodes not only MTF indexes in the
// tree, but also two magic values for run-length encoding and an EOF symbol.
// Thus there are more than 256 possible symbols.
type huffmanNode struct {
	left, right           uint16
	leftValue, rightValue uint16
}

// invalidNodeValue is an invalid index which marks a leaf node in the tree.
const invalidNodeValue = 0xffff

// Decode reads bits from the given bitReader and navigates the tree until a
// symbol is found.
func (t *huffmanTree) Decode(br *bitReader) (v uint16) {
	nodeIndex := uint16(0) // node 0 is the root of the tree.

	for {
		node := &t.nodes[nodeIndex]

		var bit uint16
		if br.bits > 0 {
			// Get next bit - fast path.
			br.bits--
			bit = uint16(br.n>>(br.bits&63)) & 1
		} else {
			// Get next bit - slow path.
			// Use ReadBits to retrieve a single bit
			// from the underling io.ByteReader.
			bit = uint16(br.ReadBits(1))
		}

		// Trick a compiler into generating conditional move instead of branch,
		// by making both loads unconditional.
		l, r := node.left, node.right

		if bit == 1 {
			nodeIndex = l
		} else {
			nodeIndex = r
		}

		if nodeIndex == invalidNodeValue {
			// We found a leaf. Use the value of bit to decide
			// whether is a left or a right value.
			l, r := node.leftValue, node.rightValue
			if bit == 1 {
				v = l
			} else {
				v = r
			}
			return
		}
	}
}

// newHuffmanTree builds a Huffman tree from a slice containing the code
// lengths of each symbol. The maximum code length is 32 bits.
func newHuffmanTree(lengths []uint8) (huffmanTree, error) {
	// There are many possible trees that assign the same code length to
	// each symbol (consider reflecting a tree down the middle, for
	// example). Since the code length assignments determine the
	// efficiency of the tree, each of these trees is equally good. In
	// order to minimize the amount of information needed to build a tree
	// bzip2 uses a canonical tree so that it can be reconstructed given
	// only the code length assignments.

	if len(lengths) < 2 {
		panic("newHuffmanTree: too few symbols")
	}

	var t huffmanTree

	// First we sort the code length assignments by ascending code length,
	// using the symbol value to break ties.
	pairs := make([]huffmanSymbolLengthPair, len(lengths))
	for i, length := range lengths {
		pairs[i].value = uint16(i)
		pairs[i].length = length
	}

	slices.SortFunc(pairs, func(a, b huffmanSymbolLengthPair) int {
		if c := cmp.Compare(a.length, b.length); c != 0 {
			return c
		}
		return cmp.Compare(a.value, b.value)
	})

	// Now we assign codes to the symbols, starting with the longest code.
	// We keep the codes packed into a uint32, at the most-significant end.
	// So branches are taken from the MSB downwards. This makes it easy to
	// sort them later.
	code := uint32(0)
	length := uint8(32)

	codes := make([]huffmanCode, len(lengths))
	for i := len(pairs) - 1; i >= 0; i-- {
		if length > pairs[i].length {
			length = pairs[i].length
		}
		codes[i].code = code
		codes[i].codeLen = length
		codes[i].value = pairs[i].value
		// We need to 'increment' the code, which means treating |code|
		// like a |length| bit number.
		code += 1 << (32 - length)
	}

	// Now we can sort by the code so that the left half of each branch are
	// grouped together, recursively.
	slices.SortFunc(codes, func(a, b huffmanCode) int {
		return cmp.Compare(a.code, b.code)
	})

	t.nodes = make([]huffmanNode, len(codes))
	_, err := buildHuffmanNode(&t, codes, 0)
	return t, err
}

// huffmanSymbolLengthPair contains a symbol and its code length.
type huffmanSymbolLengthPair struct {
	value  uint16
	length uint8
}

// huffmanCode contains a symbol, its code and code length.
type huffmanCode struct {
	code    uint32
	codeLen uint8
	value   uint16
}

// buildHuffmanNode takes a slice of sorted huffmanCodes and builds a node in
// the Huffman tree at the given level. It returns the index of the newly
// constructed node.
func buildHuffmanNode(t *huffmanTree, codes []huffmanCode, level uint32) (nodeIndex uint16, err error) {
	test := uint32(1) << (31 - level)

	// We have to search the list of codes to find the divide between the left and right sides.
	firstRightIndex := len(codes)
	for i, code := range codes {
		if code.code&test != 0 {
			firstRightIndex = i
			break
		}
	}

	left := codes[:firstRightIndex]
	right := codes[firstRightIndex:]

	if len(left) == 0 || len(right) == 0 {
		// There is a superfluous level in the Huffman tree indicating
		// a bug in the encoder. However, this bug has been observed in
		// the wild so we handle it.

		// If this function was called recursively then we know that
		// len(codes) >= 2 because, otherwise, we would have hit the
		// "leaf node" case, below, and not recurred.
		//
		// However, for the initial call it's possible that len(codes)
		// is zero or one. Both cases are invalid because a zero length
		// tree cannot encode anything and a length-1 tree can only
		// encode EOF and so is superfluous. We reject both.
		if len(codes) < 2 {
			return 0, StructuralError("empty Huffman tree")
		}

		// In this case the recursion doesn't always reduce the length
		// of codes so we need to ensure termination via another
		// mechanism.
		if level == 31 {
			// Since len(codes) >= 2 the only way that the values
			// can match at all 32 bits is if they are equal, which
			// is invalid. This ensures that we never enter
			// infinite recursion.
			return 0, StructuralError("equal symbols in Huffman tree")
		}

		if len(left) == 0 {
			return buildHuffmanNode(t, right, level+1)
		}
		return buildHuffmanNode(t, left, level+1)
	}

	nodeIndex = uint16(t.nextNode)
	node := &t.nodes[t.nextNode]
	t.nextNode++

	if len(left) == 1 {
		// leaf node
		node.left = invalidNodeValue
		node.leftValue = left[0].value
	} else {
		node.left, err = buildHuffmanNode(t, left, level+1)
	}

	if err != nil {
		return
	}

	if len(right) == 1 {
		// leaf node
		node.right = invalidNodeValue
		node.rightValue = right[0].value
	} else {
		node.right, err = buildHuffmanNode(t, right, level+1)
	}

	return
}

```

// === FILE: references/go/src/compress/bzip2/move_to_front.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bzip2

// moveToFrontDecoder implements a move-to-front list. Such a list is an
// efficient way to transform a string with repeating elements into one with
// many small valued numbers, which is suitable for entropy encoding. It works
// by starting with an initial list of symbols and references symbols by their
// index into that list. When a symbol is referenced, it's moved to the front
// of the list. Thus, a repeated symbol ends up being encoded with many zeros,
// as the symbol will be at the front of the list after the first access.
type moveToFrontDecoder []byte

// newMTFDecoder creates a move-to-front decoder with an explicit initial list
// of symbols.
func newMTFDecoder(symbols []byte) moveToFrontDecoder {
	if len(symbols) > 256 {
		panic("too many symbols")
	}
	return moveToFrontDecoder(symbols)
}

// newMTFDecoderWithRange creates a move-to-front decoder with an initial
// symbol list of 0...n-1.
func newMTFDecoderWithRange(n int) moveToFrontDecoder {
	if n > 256 {
		panic("newMTFDecoderWithRange: cannot have > 256 symbols")
	}

	m := make([]byte, n)
	for i := 0; i < n; i++ {
		m[i] = byte(i)
	}
	return moveToFrontDecoder(m)
}

func (m moveToFrontDecoder) Decode(n int) (b byte) {
	// Implement move-to-front with a simple copy. This approach
	// beats more sophisticated approaches in benchmarking, probably
	// because it has high locality of reference inside of a
	// single cache line (most move-to-front operations have n < 64).
	b = m[n]
	copy(m[1:], m[:n])
	m[0] = b
	return
}

// First returns the symbol at the front of the list.
func (m moveToFrontDecoder) First() byte {
	return m[0]
}

```

// === FILE: references/go/src/compress/flate/deflate.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flate

import (
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
)

const (
	NoCompression      = 0
	BestSpeed          = 1
	BestCompression    = 9
	DefaultCompression = -1

	// HuffmanOnly disables Lempel-Ziv match searching and only performs Huffman
	// entropy encoding. This mode is useful in compressing data that has
	// already been compressed with an LZ style algorithm (e.g. Snappy or LZ4)
	// that lacks an entropy encoder. Compression gains are achieved when
	// certain bytes in the input stream occur more frequently than others.
	//
	// Note that HuffmanOnly produces a compressed output that is
	// RFC 1951 compliant. That is, any valid DEFLATE decompressor will
	// continue to be able to decompress this output.
	HuffmanOnly = -2
)

const (
	logWindowSize  = 15
	windowSize     = 1 << logWindowSize
	windowMask     = windowSize - 1
	minMatchLength = 4   // The smallest match that the compressor looks for
	maxMatchLength = 258 // The longest match for the compressor
	minOffsetSize  = 1   // The shortest offset that makes any sense

	// The maximum number of tokens we will encode at the time.
	// Smaller sizes usually creates less optimal blocks.
	// Bigger can make context switching slow.
	// We use this for levels 7-9, so we make it big.
	maxFlateBlockTokens = 1 << 15
	maxStoreBlockSize   = 65535
	hashBits            = 17 // After 17 performance degrades
	hashSize            = 1 << hashBits
	hashMask            = (1 << hashBits) - 1
	maxHashOffset       = 1 << 28

	skipNever = math.MaxInt32
)

// compressionLevel holds the parameters for levels 7-9.
type compressionLevel struct {
	good  int32 // "good enough" match length
	lazy  int32 // don't try to find a later, better match above this length
	nice  int32 // stop looking for a better match above this length
	chain int32 // maximum number of hash chain entries to search
	level int
}

var levels = []compressionLevel{
	{}, // 0
	// Level 1-6 uses specialized algorithm - values not used
	{0, 0, 0, 0, 1},
	{0, 0, 0, 0, 2},
	{0, 0, 0, 0, 3},
	{0, 0, 0, 0, 4},
	{0, 0, 0, 0, 5},
	{0, 0, 0, 0, 6},
	// Levels 7-9 use increasingly more lazy matching
	// and increasingly stringent conditions for "good enough".
	{8, 12, 16, 24, 7},
	{16, 30, 40, 64, 8},
	{32, 258, 258, 1024, 9},
}

// advancedState contains state for levels 7-9, with bigger hash tables, etc.
type advancedState struct {
	// deflate state
	length         int32
	offset         int32
	maxInsertIndex int32
	chainHead      int32
	hashOffset     int32

	literalCounter uint16 // consecutive literal count; overflows to reset after 64KB.

	// input window: unprocessed data is window[index:windowEnd]
	index     int32
	hashMatch [maxMatchLength + minMatchLength]uint32

	// Input hash chains
	// hashHead[hashValue] contains the largest inputIndex with the specified hash value
	// If hashHead[hashValue] is within the current window, then
	// hashPrev[hashHead[hashValue] & windowMask] contains the previous index
	// with the same hash value.
	hashHead [hashSize]int32
	hashPrev [windowSize]int32
}

type compressor struct {
	compressionLevel

	h *huffmanEncoder   // huffman encoder, with state
	w *huffmanBitWriter // writer for blocks

	// compression algorithm
	fill func(*compressor, []byte) int // copy data to window
	step func(*compressor)             // process window

	window     []byte // current window - size depends on encoder level
	windowEnd  int32  // filled bytes in window
	blockStart int32  // window index where current tokens start
	err        error  // stateful error

	// queued output tokens
	tokens tokens         // tokens store for each block
	fast   fastEnc        // encoder to use for blocks
	state  *advancedState // chained encoder for level 7-9

	sync          bool // requesting flush
	byteAvailable bool // if true, still need to process window[index-1].
}

// fillDeflate will add b to the current window for levels 7-9.
func (d *compressor) fillDeflate(b []byte) int {
	s := d.state
	if s.index >= 2*windowSize-(minMatchLength+maxMatchLength) {
		// shift the window by windowSize
		copy(d.window[:], d.window[windowSize:2*windowSize])
		s.index -= windowSize
		d.windowEnd -= windowSize
		if d.blockStart >= windowSize {
			d.blockStart -= windowSize
		} else {
			d.blockStart = math.MaxInt32
		}
		s.hashOffset += windowSize
		if s.hashOffset > maxHashOffset {
			delta := s.hashOffset - 1
			s.hashOffset -= delta
			s.chainHead -= delta
			// Note: range over &array to avoid copy (see go.dev/issue/18625).
			for i, v := range &s.hashPrev {
				s.hashPrev[i] = max(v-delta, 0)
			}
			for i, v := range &s.hashHead {
				s.hashHead[i] = max(v-delta, 0)
			}
		}
	}
	n := copy(d.window[d.windowEnd:], b)
	d.windowEnd += int32(n)
	return n
}

// writeBlock will write tokens to output.
// The provided index is where the block starts in d.window.
func (d *compressor) writeBlock(tok *tokens, index int32, eof bool) error {
	if index > 0 || eof {
		var window []byte
		if d.blockStart <= index {
			window = d.window[d.blockStart:index]
		}
		d.blockStart = index
		d.w.writeBlockDynamic(tok, eof, window, d.sync)
		return d.w.err
	}
	return nil
}

// writeBlockSkip writes the current block and uses the number of tokens
// to determine if the block should be stored when there are no matches, or
// only Huffman encoded.
func (d *compressor) writeBlockSkip(tok *tokens, index int32, eof bool) error {
	if index > 0 || eof {
		if d.blockStart <= index {
			window := d.window[d.blockStart:index]
			// If we removed less than a 64th of all literals
			// we huffman compress the block.
			if int(tok.n) > len(window)-(len(window)>>6) {
				d.w.writeBlockHuff(eof, window, d.sync)
			} else {
				// Write a dynamic huffman block.
				d.w.writeBlockDynamic(tok, eof, window, d.sync)
			}
		} else {
			d.w.writeBlock(tok, eof, nil)
		}
		d.blockStart = index
		return d.w.err
	}
	return nil
}

// fillWindow will fill the current window with the supplied
// dictionary and calculate all hashes.
// This is much faster than doing a full encode.
// Should only be used after a start/reset.
func (d *compressor) fillWindow(b []byte) {
	// Do not fill window if we are in store-only or huffman mode.
	if d.level <= 0 {
		return
	}
	if d.fast != nil {
		// encode the last data, but discard the result
		if len(b) > maxMatchOffset {
			b = b[len(b)-maxMatchOffset:]
		}
		d.fast.encode(&d.tokens, b)
		d.tokens.Reset()
		return
	}
	s := d.state
	// If we are given too much, cut it.
	if len(b) > windowSize {
		b = b[len(b)-windowSize:]
	}
	// Add all to window.
	n := int32(copy(d.window[d.windowEnd:], b))

	// Calculate 256 hashes at the time (more L1 cache hits)
	loops := (n + 256 - minMatchLength) / 256
	for j := range loops {
		startindex := j * 256
		end := min(startindex+256+minMatchLength-1, n)
		tocheck := d.window[startindex:end]
		dstSize := len(tocheck) - minMatchLength + 1

		if dstSize <= 0 {
			continue
		}

		dst := s.hashMatch[:dstSize]
		bulkHash4(tocheck, dst)
		var newH uint32
		for i, val := range dst {
			di := int32(i) + startindex
			newH = val & hashMask
			// Get previous value with the same hash.
			// Our chain should point to the previous value.
			s.hashPrev[di&windowMask] = s.hashHead[newH]
			// Set the head of the hash chain to us.
			s.hashHead[newH] = di + s.hashOffset
		}
	}
	// Update window information.
	d.windowEnd += n
	s.index = n
}

// findMatch finds the longest match starting at pos in the hash chain starting
// at prevHead. It searches up to d.chain entries in the chain.
func (d *compressor) findMatch(pos int32, prevHead int32, lookahead int32) (length, offset int32, ok bool) {
	minMatchLook := min(lookahead, maxMatchLength)

	win := d.window[0 : pos+minMatchLook]

	// We quit when we get a match that's at least nice long
	nice := min(d.nice, int32(len(win))-pos)

	// If we've got a match that's good enough, only look in 1/4 the chain.
	tries := d.chain
	length = minMatchLength - 1

	wEnd := win[pos+length]
	wPos := win[pos:]
	minIndex := max(pos-windowSize, 0)
	offset = 0

	// Minimum gain to accept a match.
	cGain := 4

	// Some like it higher (CSV), some like it lower (JSON)
	const baseCost = 3
	// Base is 4 bytes at with an additional cost.
	// Matches must be better than this.

	for i := prevHead; tries > 0; tries-- {
		if wEnd == win[i+length] {
			n := int32(matchLen(win[i:i+minMatchLook], wPos))
			if n > length {
				if d.chain >= 100 {
					// Calculate gain. Estimates the gains of the new match compared to emitting as literals.
					newGain := d.h.bitLengthRaw(wPos[:n]) - int(offsetExtraBits[offsetCode(uint32(pos-i))]) - baseCost - int(lengthExtraBits[lengthCodes[(n-3)&255]])
					if newGain <= cGain {
						goto next
					}
					cGain = newGain
				}
				length = n
				offset = pos - i
				ok = true
				if n >= nice {
					// The match is good enough that we don't try to find a better one.
					break
				}
				wEnd = win[pos+n]
			}
		}
	next:
		if i <= minIndex {
			// hashPrev[i & windowMask] has already been overwritten, so stop now.
			break
		}
		i = d.state.hashPrev[i&windowMask] - d.state.hashOffset
		if i < minIndex {
			break
		}
	}
	return
}

// writeStoredBlock writes an uncompressed block to the stream.
func (d *compressor) writeStoredBlock(buf []byte) error {
	if d.w.writeStoredHeader(len(buf), false); d.w.err != nil {
		return d.w.err
	}
	d.w.writeBytes(buf)
	return d.w.err
}

// hash4 returns a hash representation of the first 4 bytes
// of the supplied slice.
// The caller must ensure that len(b) >= 4.
func hash4(b []byte) uint32 {
	return hash4u(loadLE32(b, 0), hashBits)
}

// hash4 returns the hash of u to fit in a hash table with h bits.
// Preferably h should be a constant and should always be <32.
func hash4u(u uint32, h uint8) uint32 {
	return (u * prime4bytes) >> (32 - h)
}

// bulkHash4 sets dst[i] = hash4(b[i:i+4]) for all i <= len(b)-4.
func bulkHash4(b []byte, dst []uint32) {
	if len(b) < 4 {
		return
	}
	hb := loadLE32(b, 0)

	dst[0] = hash4u(hb, hashBits)
	end := len(b) - 4 + 1
	for i := 1; i < end; i++ {
		hb = (hb >> 8) | uint32(b[i+3])<<24
		dst[i] = hash4u(hb, hashBits)
	}
}

// initDeflate initializes d for levels 7-9.
func (d *compressor) initDeflate() {
	d.window = make([]byte, 2*windowSize)
	d.byteAvailable = false
	d.err = nil
	if d.state == nil {
		return
	}
	s := d.state
	s.index = 0
	s.hashOffset = 1
	s.length = minMatchLength - 1
	s.offset = 0
	s.chainHead = -1
}

// tryBetterMatchAtEnd checks whether a better match exists at the end of the
// previous match and, if so, emits the skipped literals and adjusts the match.
// Returns the (possibly updated) prevLength and prevOffset.
func (d *compressor) tryBetterMatchAtEnd(prevLength, prevOffset, lookahead int32) (newLen, newOff int32) {
	// We start checking at checkOff from the current match position.
	// This allows up to two additional literals, but that could be
	// compensated by a higher quality match.
	// If the match looks better, we extend backwards.
	const checkOff = 2
	s := d.state

	if prevLength >= maxMatchLength-checkOff {
		return prevLength, prevOffset
	}
	prevIndex := s.index - 1
	if prevIndex+prevLength >= s.maxInsertIndex {
		return prevLength, prevOffset
	}

	end := min(lookahead, maxMatchLength+checkOff) + prevIndex
	minIndex := max(s.index-windowSize, 0)

	h := hash4(d.window[prevIndex+prevLength:])
	ch2 := s.hashHead[h] - s.hashOffset - prevLength
	if prevIndex-ch2 == prevOffset || ch2 <= minIndex+checkOff {
		return prevLength, prevOffset
	}

	length := int32(matchLen(d.window[prevIndex+checkOff:end], d.window[ch2+checkOff:]))
	if length <= prevLength {
		return prevLength, prevOffset
	}

	prevLength = length
	prevOffset = prevIndex - ch2

	for i := int32(checkOff - 1); i >= 0; i-- {
		if prevLength >= maxMatchLength || d.window[prevIndex+i] != d.window[ch2+i] {
			for j := range i + 1 {
				d.tokens.AddLiteral(d.window[prevIndex+j])
				if d.tokens.n == maxFlateBlockTokens {
					if d.err = d.writeBlock(&d.tokens, s.index, false); d.err != nil {
						return prevLength, prevOffset
					}
					d.tokens.Reset()
				}
				s.index++
				if s.index < s.maxInsertIndex {
					h := hash4(d.window[s.index:])
					ch := s.hashHead[h]
					s.chainHead = ch
					s.hashPrev[s.index&windowMask] = ch
					s.hashHead[h] = s.index + s.hashOffset
				}
			}
			break
		}
		prevLength++
	}
	return prevLength, prevOffset
}

// skipLiterals emits extra literal bytes during long runs of incompressible data,
// skipping ahead to avoid futile match searches. Returns false on write error.
func (d *compressor) skipLiterals() bool {
	s := d.state
	n := int32(s.literalCounter) - d.chain
	if n <= 0 {
		return true
	}
	n = 1 + n>>6
	for range n {
		if s.index >= d.windowEnd-1 {
			break
		}
		d.tokens.AddLiteral(d.window[s.index-1])
		if d.tokens.n == maxFlateBlockTokens {
			if d.err = d.writeBlock(&d.tokens, s.index, false); d.err != nil {
				return false
			}
			d.tokens.Reset()
		}
		if s.index < s.maxInsertIndex {
			h := hash4(d.window[s.index:])
			ch := s.hashHead[h]
			s.chainHead = ch
			s.hashPrev[s.index&windowMask] = ch
			s.hashHead[h] = s.index + s.hashOffset
		}
		s.index++
	}
	d.tokens.AddLiteral(d.window[s.index-1])
	d.byteAvailable = false
	if d.tokens.n == maxFlateBlockTokens {
		if d.err = d.writeBlock(&d.tokens, s.index, false); d.err != nil {
			return false
		}
		d.tokens.Reset()
	}
	return true
}

// deflateLazy encodes the current window using lazy matching.
// Lazy matching defers emitting a match to see if the next position yields a better one.
// Unique to levels 7-9 is that more than 2 matches are potentially checked
// until a good/nice one is found.
func (d *compressor) deflateLazy() {
	s := d.state

	if d.windowEnd-s.index < minMatchLength+maxMatchLength && !d.sync {
		return
	}
	if d.windowEnd != s.index && d.chain > 100 {
		// Get literal huffman coder.
		// This is used to estimate the cost of emitting a literal.
		if d.h == nil {
			d.h = newHuffmanEncoder(maxFlateBlockTokens)
		}
		var tmp [256]uint16
		toIndex := d.window[s.index:d.windowEnd]
		toIndex = toIndex[:min(len(toIndex), maxFlateBlockTokens)]
		for _, v := range toIndex {
			tmp[v]++
		}
		d.h.generate(tmp[:], 15)
	}

	s.maxInsertIndex = d.windowEnd - (minMatchLength - 1)

	for {
		lookahead := d.windowEnd - s.index
		if lookahead < minMatchLength+maxMatchLength {
			if !d.sync {
				return
			}
			if lookahead == 0 {
				// Flush current output block if any.
				if d.byteAvailable {
					// There is still one pending token that needs to be flushed
					d.tokens.AddLiteral(d.window[s.index-1])
					d.byteAvailable = false
				}
				if d.tokens.n > 0 {
					if d.err = d.writeBlock(&d.tokens, s.index, false); d.err != nil {
						return
					}
					d.tokens.Reset()
				}
				return
			}
		}
		if s.index < s.maxInsertIndex {
			h := hash4(d.window[s.index:])
			ch := s.hashHead[h]
			s.chainHead = ch
			s.hashPrev[s.index&windowMask] = ch
			s.hashHead[h] = s.index + s.hashOffset
		}
		prevLength := s.length
		prevOffset := s.offset
		s.length = minMatchLength - 1
		s.offset = 0
		minIndex := max(s.index-windowSize, 0)

		if s.chainHead-s.hashOffset >= minIndex && lookahead > prevLength && prevLength < d.lazy {
			if newLength, newOffset, ok := d.findMatch(s.index, s.chainHead-s.hashOffset, lookahead); ok {
				s.length = newLength
				s.offset = newOffset
			}
		}

		if prevLength >= minMatchLength && s.length <= prevLength {
			prevLength, prevOffset = d.tryBetterMatchAtEnd(prevLength, prevOffset, lookahead)
			if d.err != nil {
				return
			}

			// There was a match at the previous step, and the current match is
			// not better. Output the previous match.
			d.tokens.AddMatch(uint32(prevLength-3), uint32(prevOffset-minOffsetSize))

			// Insert in the hash table all strings up to the end of the match.
			// index and index-1 are already inserted. If there is not enough
			// lookahead, the last two strings are not inserted into the hash
			// table.
			newIndex := s.index + prevLength - 1
			end := min(newIndex, s.maxInsertIndex)
			end += minMatchLength - 1
			startindex := min(s.index+1, s.maxInsertIndex)
			tocheck := d.window[startindex:end]
			dstSize := len(tocheck) - minMatchLength + 1
			if dstSize > 0 {
				dst := s.hashMatch[:dstSize]
				bulkHash4(tocheck, dst)
				var newH uint32
				for i, val := range dst {
					di := int32(i) + startindex
					newH = val & hashMask
					s.hashPrev[di&windowMask] = s.hashHead[newH]
					s.hashHead[newH] = di + s.hashOffset
				}
			}

			s.index = newIndex
			d.byteAvailable = false
			s.length = minMatchLength - 1
			if d.tokens.n == maxFlateBlockTokens {
				if d.err = d.writeBlock(&d.tokens, s.index, false); d.err != nil {
					return
				}
				d.tokens.Reset()
			}
			s.literalCounter = 0
			continue
		}
		if s.length >= minMatchLength {
			s.literalCounter = 0
		}
		if d.byteAvailable {
			s.literalCounter++
			d.tokens.AddLiteral(d.window[s.index-1])
			if d.tokens.n == maxFlateBlockTokens {
				if d.err = d.writeBlock(&d.tokens, s.index, false); d.err != nil {
					return
				}
				d.tokens.Reset()
			}
			s.index++
			if !d.skipLiterals() {
				return
			}
		} else {
			s.index++
			d.byteAvailable = true
		}
	}
}

// store will store the current window if it has filled or if we are in sync.
func (d *compressor) store() {
	if d.windowEnd > 0 && (d.windowEnd == maxStoreBlockSize || d.sync) {
		d.err = d.writeStoredBlock(d.window[:d.windowEnd])
		d.windowEnd = 0
	}
}

// fillBlock appends b to d.window, returning the number of bytes copied.
// If n < len(b), the window is filled.
func (d *compressor) fillBlock(b []byte) int {
	n := copy(d.window[d.windowEnd:], b)
	d.windowEnd += int32(n)
	return n
}

// deflateHuff compresses and stores the current window
// (if it has filled or if we are in sync or flush).
// It uses Huffman-only encoding.
func (d *compressor) deflateHuff() {
	if int(d.windowEnd) < len(d.window) && !d.sync || d.windowEnd == 0 {
		return
	}
	d.w.writeBlockHuff(false, d.window[:d.windowEnd], d.sync)
	d.err = d.w.err
	d.windowEnd = 0
}

// deflateFast encodes the current window
// if it has filled or if we are doing sync/flush.
// It uses the level 1-6 fast encoding.
func (d *compressor) deflateFast() {
	// We only compress if we have maxStoreBlockSize.
	if int(d.windowEnd) < len(d.window) {
		if !d.sync {
			return
		}
		// Handle extremely small sizes.
		if d.windowEnd < 128 {
			if d.windowEnd == 0 {
				return
			}
			if d.windowEnd <= 32 {
				d.err = d.writeStoredBlock(d.window[:d.windowEnd])
			} else {
				d.w.writeBlockHuff(false, d.window[:d.windowEnd], true)
				d.err = d.w.err
			}
			d.tokens.Reset()
			d.windowEnd = 0
			d.fast.reset()
			return
		}
	}

	d.fast.encode(&d.tokens, d.window[:d.windowEnd])
	// If we made zero matches, store the block as is.
	if d.tokens.n == 0 {
		d.err = d.writeStoredBlock(d.window[:d.windowEnd])
		// If we removed less than 1/16th, huffman compress the block.
	} else if int32(d.tokens.n) > d.windowEnd-(d.windowEnd>>4) {
		d.w.writeBlockHuff(false, d.window[:d.windowEnd], d.sync)
		d.err = d.w.err
	} else {
		d.w.writeBlockDynamic(&d.tokens, false, d.window[:d.windowEnd], d.sync)
		d.err = d.w.err
	}
	d.tokens.Reset()
	d.windowEnd = 0
}

// write adds b to the compressor.
// It can only return a short length if an error occurs.
func (d *compressor) write(b []byte) (n int, err error) {
	if d.err != nil {
		return 0, d.err
	}
	n = len(b)
	for len(b) > 0 {
		if int(d.windowEnd) == len(d.window) || d.sync {
			d.step(d)
		}
		b = b[d.fill(d, b):]
		if d.err != nil {
			return 0, d.err
		}
	}
	return n, d.err
}

// syncFlush will flush the compressor by writing
// any remaining window and writing a stored block
// to byte-align the output.
func (d *compressor) syncFlush() error {
	if d.err != nil {
		return d.err
	}
	d.sync = true
	d.step(d)
	if d.err == nil {
		d.w.writeStoredHeader(0, false)
		d.w.flush()
		d.err = d.w.err
	}
	d.sync = false
	return d.err
}

// init a new encode with new writer and compression level.
func (d *compressor) init(w io.Writer, level int) (err error) {
	d.w = newHuffmanBitWriter(w)

	switch {
	case level == NoCompression:
		d.window = make([]byte, maxStoreBlockSize)
		d.fill = (*compressor).fillBlock
		d.step = (*compressor).store
	case level == HuffmanOnly:
		d.w.logNewTablePenalty = 10
		d.window = make([]byte, 32<<10)
		d.fill = (*compressor).fillBlock
		d.step = (*compressor).deflateHuff
	case level == DefaultCompression:
		level = 6
		fallthrough
	case 1 <= level && level <= 6:
		d.w.logNewTablePenalty = 7
		d.fast = newFastEnc(level)
		d.window = make([]byte, maxStoreBlockSize)
		d.fill = (*compressor).fillBlock
		d.step = (*compressor).deflateFast
	case 7 <= level && level <= 9:
		d.w.logNewTablePenalty = 8
		d.state = &advancedState{}
		d.compressionLevel = levels[level]
		d.initDeflate()
		d.fill = (*compressor).fillDeflate
		d.step = (*compressor).deflateLazy
	default:
		return fmt.Errorf("flate: invalid compression level %d: want value in range [-2, 9]", level)
	}
	d.level = level
	return nil
}

// reset resets the compressor with a new output writer.
func (d *compressor) reset(w io.Writer) {
	d.w.reset(w)
	d.sync = false
	d.err = nil
	d.windowEnd = 0
	// We only need to reset a few things for fast encoders.
	if d.fast != nil {
		d.fast.reset()
		d.tokens.Reset()
		return
	}
	if d.compressionLevel.chain == 0 {
		return
	}
	s := d.state
	s.chainHead = -1
	clear(s.hashHead[:])
	clear(s.hashPrev[:])
	s.hashOffset = 1
	s.index = 0
	d.blockStart, d.byteAvailable = 0, false
	d.tokens.Reset()
	s.length = minMatchLength - 1
	s.offset = 0
	s.literalCounter = 0
	s.maxInsertIndex = 0
}

var errWriterClosed = errors.New("flate: closed writer")

// close flushes any uncompressed data and writes an EOF block.
func (d *compressor) close() error {
	if d.err == errWriterClosed {
		return nil
	}
	if d.err != nil {
		return d.err
	}
	d.sync = true
	d.step(d)
	if d.err != nil {
		return d.err
	}
	if d.w.writeStoredHeader(0, true); d.w.err != nil {
		return d.w.err
	}
	d.w.flush()
	if d.w.err != nil {
		return d.w.err
	}
	d.err = errWriterClosed
	d.w.reset(nil)
	return nil
}

// NewWriter returns a new [Writer] compressing data at the given level.
// Following zlib, levels range from 1 ([BestSpeed]) to 9 ([BestCompression]);
// higher levels typically run slower but compress more. Level 0
// ([NoCompression]) does not attempt any compression; it only adds the
// necessary DEFLATE framing.
// Level -1 ([DefaultCompression]) uses the default compression level.
// Level -2 ([HuffmanOnly]) will use Huffman compression only, giving
// a very fast compression for all types of input, but sacrificing considerable
// compression efficiency.
//
// If level is in the range [-2, 9] then the error returned will be nil.
// Otherwise the error returned will be non-nil.
//
// Note that the exact bytes written to w are not covered by the Go 1
// compatibility promise. Callers, including tests, should not depend on the
// exact written bytes.
func NewWriter(w io.Writer, level int) (*Writer, error) {
	var dw Writer
	if err := dw.d.init(w, level); err != nil {
		return nil, err
	}
	return &dw, nil
}

// NewWriterDict is like [NewWriter] but initializes the new
// [Writer] with a preset dictionary. The returned [Writer] behaves
// as if the dictionary had been written to it without producing
// any compressed output. The compressed data written to w
// can only be decompressed by a reader initialized with the
// same dictionary (see [NewReaderDict]).
//
// Note that the exact bytes written to w are not covered by the Go 1
// compatibility promise. Callers, including tests, should not depend on the
// exact written bytes.
func NewWriterDict(w io.Writer, level int, dict []byte) (*Writer, error) {
	zw, err := NewWriter(w, level)
	if err != nil {
		return nil, err
	}
	zw.d.fillWindow(dict)
	// Clone dict so we can Reset without changing the provided slice.
	zw.dict = slices.Clone(dict)
	return zw, err
}

// A Writer takes data written to it and writes the compressed
// form of that data to an underlying writer (see [NewWriter]).
type Writer struct {
	d    compressor
	dict []byte
}

// Write writes data to w, which will eventually write the
// compressed form of data to its underlying writer.
func (w *Writer) Write(data []byte) (n int, err error) {
	return w.d.write(data)
}

// Flush flushes any pending data to the underlying writer.
// It is useful mainly in compressed network protocols, to ensure that
// a remote reader has enough data to reconstruct a packet.
// Flush does not return until the data has been written.
// Calling Flush when there is no pending data still causes the [Writer]
// to emit a sync marker of at least 4 bytes.
// If the underlying writer returns an error, Flush returns that error.
//
// In the terminology of the zlib library, Flush is equivalent to Z_SYNC_FLUSH.
func (w *Writer) Flush() error {
	// For more about flushing:
	// https://www.bolet.org/~pornin/deflate-flush.html
	return w.d.syncFlush()
}

// Close flushes and closes the writer.
func (w *Writer) Close() error {
	return w.d.close()
}

// Reset discards the writer's state and makes it equivalent to
// the result of NewWriter or NewWriterDict called with dst
// and w's level and dictionary.
func (w *Writer) Reset(dst io.Writer) {
	w.d.reset(dst)
	w.d.fillWindow(w.dict)
}

```

// === FILE: references/go/src/compress/flate/deflatefast.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flate

import (
	"math/bits"
)

const (
	// tableBits is the number of bits used in the hash table.
	tableBits = 15

	// tableSize is the size of the hash table.
	tableSize = 1 << tableBits

	// hashLongBytes is the number of bytes used for long table hashes.
	hashLongBytes = 7

	// baseMatchOffset is the smallest match offset.
	baseMatchOffset = 1

	// baseMatchLength is the smallest match length per RFC section 3.2.5.
	baseMatchLength = 3

	// maxMatchOffset is the largest match offset.
	maxMatchOffset = 1 << 15

	// allocHistory is the size to preallocate for history.
	allocHistory = maxStoreBlockSize * 5

	// bufferReset is the buffer offset at which the history is reset.
	bufferReset = (1 << 31) - allocHistory - maxStoreBlockSize - 1
)

// fastEncL1 to fastEncL6 provides specialized encoders for levels 1-6
// that each provide a different speed/size/memory strategies.
//
// Level 1: Single small table, 5 byte hashes, sparse indexing.
// Level 2: Single big table, 5 byte hashes, indexing ~ every 2 bytes.
// Level 3: Single medium table, 5 byte hashes, 2 candidates per table entry.
// Level 4: Two tables, 4/7 byte hashes, 1 candidate per table entry.
// Level 5: Two tables, 4/7 byte hashes, 2 candidates per 7-byte table entry.
// Level 6: Two tables, 4/7 byte hashes, full indexing, checks for repeats.
//
// Skipping on contiguous non-matches also decreases as levels go up.

// fastEnc is the interface implemented by the level 1-6 fast encoders.
type fastEnc interface {
	// encode src into dst.
	encode(dst *tokens, src []byte)
	// reset the encoder so matches are not made with previous data.
	reset()
}

// newFastEnc returns a fastEnc encoder for the given compression level (1-6).
func newFastEnc(level int) fastEnc {
	switch level {
	case 1:
		return &fastEncL1{fastGen: fastGen{cur: maxStoreBlockSize}}
	case 2:
		return &fastEncL2{fastGen: fastGen{cur: maxStoreBlockSize}}
	case 3:
		return &fastEncL3{fastGen: fastGen{cur: maxStoreBlockSize}}
	case 4:
		return &fastEncL4{fastGen: fastGen{cur: maxStoreBlockSize}}
	case 5:
		return &fastEncL5{fastGen: fastGen{cur: maxStoreBlockSize}}
	case 6:
		return &fastEncL6{fastGen: fastGen{cur: maxStoreBlockSize}}
	default:
		panic("invalid level specified")
	}
}

// fastGen maintains the table for matches,
// and the previous byte block for level 1 and up.
// This is the generic implementation.
type fastGen struct {
	hist []byte
	cur  int32
}

// addBlock appends src to the history and returns the offset where src starts in e.hist.
func (e *fastGen) addBlock(src []byte) int32 {
	// check if we have space already
	if len(e.hist)+len(src) > cap(e.hist) {
		if cap(e.hist) == 0 {
			e.hist = make([]byte, 0, allocHistory)
		} else {
			if cap(e.hist) < maxMatchOffset*2 {
				panic("unexpected buffer size")
			}
			// Move down
			offset := int32(len(e.hist)) - maxMatchOffset
			copy(e.hist[0:maxMatchOffset], e.hist[offset:offset+maxMatchOffset])
			e.cur += offset
			e.hist = e.hist[:maxMatchOffset]
		}
	}
	s := int32(len(e.hist))
	e.hist = append(e.hist, src...)
	return s
}

// matchLenLimited returns the match length between offsets s and t in src.
// The maximum length returned is maxMatchLength - 4.
// It is assumed that s > t, that t >= 0 and s < len(src).
func (e *fastGen) matchLenLimited(s, t int, src []byte) int32 {
	a := src[s:min(s+maxMatchLength-4, len(src))]
	b := src[t:]
	return int32(matchLen(a, b))
}

// matchLenLong returns the match length between offsets s and t in src.
// It is assumed that s > t, that t >= 0 and s < len(src).
func (e *fastGen) matchLenLong(s, t int, src []byte) int32 {
	return int32(matchLen(src[s:], src[t:]))
}

// reset resets the encoding table to prepare for a new compression stream.
func (e *fastGen) reset() {
	if cap(e.hist) < allocHistory {
		e.hist = make([]byte, 0, allocHistory)
	}
	// We offset current position so everything will be out of reach.
	// If we are above the buffer reset it will be cleared anyway since len(hist) == 0.
	if e.cur <= bufferReset {
		e.cur += maxMatchOffset + int32(len(e.hist))
	}
	e.hist = e.hist[:0]
}

func (f *fastGen) getFastGen() *fastGen { return f }

// tableEntry stores the offset of a hash match in the input history.
type tableEntry struct {
	offset int32
}

// tableEntryPrev stores the current and previous offsets for a hash entry.
type tableEntryPrev struct {
	cur  tableEntry
	prev tableEntry
}

const (
	prime3bytes = 506832829
	prime4bytes = 2654435761
	prime5bytes = 889523592379
	prime6bytes = 227718039650203
	prime7bytes = 58295818150454627
	prime8bytes = 0xcf1bbcdcb7a56463
)

// hashLen returns a hash of the first n bytes of u, using b output bits.
// It expects 3 <= n <= 8; other values are treated as n == 4.
// The bit length b must be <= 32.
// b and n should be constants in speed-critical use.
func hashLen(u uint64, b, n uint8) uint32 {
	switch n {
	case 3:
		return (uint32(u<<8) * prime3bytes) >> (32 - b)
	case 5:
		return uint32(((u << (64 - 40)) * prime5bytes) >> (64 - b))
	case 6:
		return uint32(((u << (64 - 48)) * prime6bytes) >> (64 - b))
	case 7:
		return uint32(((u << (64 - 56)) * prime7bytes) >> (64 - b))
	case 8:
		return uint32((u * prime8bytes) >> (64 - b))
	default:
		return (uint32(u) * prime4bytes) >> (32 - b)
	}
}

// matchLen returns the maximum common prefix length of a and b.
// a must be the shortest of the two.
func matchLen(a, b []byte) (n int) {
	left := len(a)
	for left >= 8 {
		diff := loadLE64(a, n) ^ loadLE64(b, n)
		if diff != 0 {
			return n + bits.TrailingZeros64(diff)>>3
		}
		n += 8
		left -= 8
	}

	a = a[n:]
	b = b[n:]
	b = b[:len(a)]
	for i := range a {
		if a[i] != b[i] {
			break
		}
		n++
	}
	return n
}

```

// === FILE: references/go/src/compress/flate/dict_decoder.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flate

// dictDecoder implements the LZ77 sliding dictionary as used in decompression.
// LZ77 decompresses data through sequences of two forms of commands:
//
//   - Literal insertions: Runs of one or more symbols are inserted into the data
//     stream as is. This is accomplished through the writeByte method for a
//     single symbol, or combinations of writeSlice/writeMark for multiple symbols.
//     Any valid stream must start with a literal insertion if no preset dictionary
//     is used.
//
//   - Backward copies: Runs of one or more symbols are copied from previously
//     emitted data. Backward copies come as the tuple (dist, length) where dist
//     determines how far back in the stream to copy from and length determines how
//     many bytes to copy. Note that it is valid for the length to be greater than
//     the distance. Since LZ77 uses forward copies, that situation is used to
//     perform a form of run-length encoding on repeated runs of symbols.
//     The writeCopy and tryWriteCopy are used to implement this command.
//
// For performance reasons, this implementation performs little to no sanity
// checks about the arguments. As such, the invariants documented for each
// method call must be respected.
type dictDecoder struct {
	hist []byte // Sliding window history

	// Invariant: 0 <= rdPos <= wrPos <= len(hist)
	wrPos int  // Current output position in buffer
	rdPos int  // Have emitted hist[:rdPos] already
	full  bool // Has a full window length been written yet?
}

// init initializes dictDecoder to have a sliding window dictionary of the given
// size. If a preset dict is provided, it will initialize the dictionary with
// the contents of dict.
func (dd *dictDecoder) init(size int, dict []byte) {
	*dd = dictDecoder{hist: dd.hist}

	if cap(dd.hist) < size {
		dd.hist = make([]byte, size)
	}
	dd.hist = dd.hist[:size]

	if len(dict) > len(dd.hist) {
		dict = dict[len(dict)-len(dd.hist):]
	}
	dd.wrPos = copy(dd.hist, dict)
	if dd.wrPos == len(dd.hist) {
		dd.wrPos = 0
		dd.full = true
	}
	dd.rdPos = dd.wrPos
}

// histSize reports the total amount of historical data in the dictionary.
func (dd *dictDecoder) histSize() int {
	if dd.full {
		return len(dd.hist)
	}
	return dd.wrPos
}

// availRead reports the number of bytes that can be flushed by readFlush.
func (dd *dictDecoder) availRead() int {
	return dd.wrPos - dd.rdPos
}

// availWrite reports the available amount of output buffer space.
func (dd *dictDecoder) availWrite() int {
	return len(dd.hist) - dd.wrPos
}

// writeSlice returns a slice of the available buffer to write data to.
//
// This invariant will be kept: len(s) <= availWrite()
func (dd *dictDecoder) writeSlice() []byte {
	return dd.hist[dd.wrPos:]
}

// writeMark advances the writer pointer by cnt.
//
// This invariant must be kept: 0 <= cnt <= availWrite()
func (dd *dictDecoder) writeMark(cnt int) {
	dd.wrPos += cnt
}

// writeByte writes a single byte to the dictionary.
//
// This invariant must be kept: 0 < availWrite()
func (dd *dictDecoder) writeByte(c byte) {
	dd.hist[dd.wrPos] = c
	dd.wrPos++
}

// writeCopy copies a string at a given (dist, length) to the output.
// This returns the number of bytes copied and may be less than the requested
// length if the available space in the output buffer is too small.
//
// This invariant must be kept: 0 < dist <= histSize()
func (dd *dictDecoder) writeCopy(dist, length int) int {
	dstBase := dd.wrPos
	dstPos := dstBase
	srcPos := dstPos - dist
	endPos := min(dstPos+length, len(dd.hist))

	// Copy non-overlapping section after destination position.
	//
	// This section is non-overlapping in that the copy length for this section
	// is always less than or equal to the backwards distance. This can occur
	// if a distance refers to data that wraps-around in the buffer.
	// Thus, a backwards copy is performed here; that is, the exact bytes in
	// the source prior to the copy is placed in the destination.
	if srcPos < 0 {
		srcPos += len(dd.hist)
		dstPos += copy(dd.hist[dstPos:endPos], dd.hist[srcPos:])
		srcPos = 0
	}

	// Copy possibly overlapping section before destination position.
	//
	// This section can overlap if the copy length for this section is larger
	// than the backwards distance. This is allowed by LZ77 so that repeated
	// strings can be succinctly represented using (dist, length) pairs.
	// Thus, a forwards copy is performed here; that is, the bytes copied is
	// possibly dependent on the resulting bytes in the destination as the copy
	// progresses along. This is functionally equivalent to the following:
	//
	//	for i := 0; i < endPos-dstPos; i++ {
	//		dd.hist[dstPos+i] = dd.hist[srcPos+i]
	//	}
	//	dstPos = endPos
	//
	for dstPos < endPos {
		dstPos += copy(dd.hist[dstPos:endPos], dd.hist[srcPos:dstPos])
	}

	dd.wrPos = dstPos
	return dstPos - dstBase
}

// tryWriteCopy tries to copy a string at a given (distance, length) to the
// output. This specialized version is optimized for short distances.
//
// This method is designed to be inlined for performance reasons.
//
// This invariant must be kept: 0 < dist <= histSize()
func (dd *dictDecoder) tryWriteCopy(dist, length int) int {
	dstPos := dd.wrPos
	endPos := dstPos + length
	if dstPos < dist || endPos > len(dd.hist) {
		return 0
	}
	dstBase := dstPos
	srcPos := dstPos - dist

	// Copy possibly overlapping section before destination position.
	for dstPos < endPos {
		dstPos += copy(dd.hist[dstPos:endPos], dd.hist[srcPos:dstPos])
	}

	dd.wrPos = dstPos
	return dstPos - dstBase
}

// readFlush returns a slice of the historical buffer that is ready to be
// emitted to the user. The data returned by readFlush must be fully consumed
// before calling any other dictDecoder methods.
func (dd *dictDecoder) readFlush() []byte {
	toRead := dd.hist[dd.rdPos:dd.wrPos]
	dd.rdPos = dd.wrPos
	if dd.wrPos == len(dd.hist) {
		dd.wrPos, dd.rdPos = 0, 0
		dd.full = true
	}
	return toRead
}

```

// === FILE: references/go/src/compress/flate/huffman_bit_writer.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flate

import (
	"io"
	"math"
	"sync"
)

const (
	// The largest offset code.
	offsetCodeCount = 30

	// The special code used to mark the end of a block.
	endBlockMarker = 256

	// The first length code.
	lengthCodesStart = 257

	// The number of codegen codes.
	codegenCodeCount = 19
	badCode          = 255

	// maxPredefinedTokens is the maximum number of tokens
	// where we check if fixed size is smaller.
	maxPredefinedTokens = 250

	// bufferFlushSize indicates the buffer size
	// after which bytes are flushed to the writer.
	// Should preferably be a multiple of 6, since
	// we accumulate 6 bytes between writes to the buffer.
	bufferFlushSize = 246
)

// lengthExtraBitsMinCode is the minimum length code that emits extra bits.
const lengthExtraBitsMinCode = 8

// lengthExtraBits[i] is the number of extra bits needed by
// length code i + lengthCodesStart.
var lengthExtraBits = [32]uint8{
	/* 257 */ 0, 0, 0,
	/* 260 */ 0, 0, 0, 0, 0, 1, 1, 1, 1, 2,
	/* 270 */ 2, 2, 2, 3, 3, 3, 3, 4, 4, 4,
	/* 280 */ 4, 5, 5, 5, 5, 0,
}

// lengthBase[i] is the length indicated by length code i + lengthCodesStart.
var lengthBase = [32]uint8{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 10,
	12, 14, 16, 20, 24, 28, 32, 40, 48, 56,
	64, 80, 96, 112, 128, 160, 192, 224, 255,
}

// offsetExtraBitsMinCode is the minimum offset code that emits extra bits.
const offsetExtraBitsMinCode = 4

// offsetExtraBits[i] is the number of extra bits for offset code i.
var offsetExtraBits = [32]int8{
	0, 0, 0, 0, 1, 1, 2, 2, 3, 3,
	4, 4, 5, 5, 6, 6, 7, 7, 8, 8,
	9, 9, 10, 10, 11, 11, 12, 12, 13, 13,
	/* extended window */
	14, 14,
}

// offsetCombined combines offset lookup of extra bits and offset code in a single table.
var offsetCombined = [32]uint32{
	0x0, 0x0, 0x0, 0x0, 0x401, 0x601, 0x802, 0xc02,
	0x1003, 0x1803, 0x2004, 0x3004, 0x4005, 0x6005,
	0x8006, 0xc006, 0x10007, 0x18007, 0x20008, 0x30008,
	0x40009, 0x60009, 0x8000a, 0xc000a, 0x10000b, 0x18000b,
	0x20000c, 0x30000c, 0x40000d, 0x60000d, 0x0, 0x0}

/*
Generated with:

func genOffsetCombined() {
	var offsetBase = [32]uint32{
		0x000000, 0x000001, 0x000002, 0x000003, 0x000004,
		0x000006, 0x000008, 0x00000c, 0x000010, 0x000018,
		0x000020, 0x000030, 0x000040, 0x000060, 0x000080,
		0x0000c0, 0x000100, 0x000180, 0x000200, 0x000300,
		0x000400, 0x000600, 0x000800, 0x000c00, 0x001000,
		0x001800, 0x002000, 0x003000, 0x004000, 0x006000,

		0x008000, 0x00c000,
	}

	for i := range offsetCombined[:] {
		// Don't use extended window values...
		if offsetExtraBits[i] == 0 || offsetBase[i] > 0x006000 {
			continue
		}
		offsetCombined[i] = uint32(offsetExtraBits[i]) | (offsetBase[i] << 8)
	}
	fmt.Printf("offsetCombined = %#v\n", offsetCombined)
}
*/

// codegenOrder is the order in which codegen code sizes are written.
var codegenOrder = []uint32{16, 17, 18, 0, 8, 7, 9, 6, 10, 5, 11, 4, 12, 3, 13, 2, 14, 1, 15}

// huffmanBitWriter encodes tokens and values to a stream.
// The huffmanBitWriter supports reusing huffman tables and will combine
// blocks, if compression is less than creating a new table.
//
// An incoming block estimates the output size of a new table using a
// 'fresh' by calculating the optimal size and adding a penalty.
// A Huffman table is not optimal, which is why we add a penalty,
// and generating a new table is slower for both compression and decompression.
type huffmanBitWriter struct {
	// writer is the underlying writer.
	// Do not use it directly; use the write method, which ensures
	// that Write errors are sticky.
	writer io.Writer

	// Data waiting to be written is bytes[0:nbytes]
	// and then the low nbits of bits.
	bits   uint64
	nbits  uint8
	nbytes uint8

	// If wroteHuffman is set, a table for outputting only literals
	// has been generated and offsets are invalid.
	wroteHuffman    bool
	literalEncoding *huffmanEncoder
	tmpLitEncoding  *huffmanEncoder
	offsetEncoding  *huffmanEncoder
	codegenEncoding *huffmanEncoder
	err             error

	// If prevHeader is non-zero the Huffman table can be reused.
	// It also indicates that an EOB has not yet been emitted, so if a new table
	// is generated, an EOB with the previous table must be written.
	prevHeader int

	// logNewTablePenalty is a log2 penalty reduction for creating new tables.
	// The initial penalty is 100%.
	// Adding 1 will cut the penalty in half.
	logNewTablePenalty uint
	bytes              [256 + 8]byte
	literalFreq        [lengthCodesStart + 32]uint16
	offsetFreq         [32]uint16
	codegenFreq        [codegenCodeCount]uint16

	// codegen must have an extra space for the final symbol.
	codegen [literalCount + offsetCodeCount + 1]uint8
}

// newHuffmanBitWriter creates a new huffmanBitWriter that will write to w.
func newHuffmanBitWriter(w io.Writer) *huffmanBitWriter {
	return &huffmanBitWriter{
		writer:          w,
		literalEncoding: newHuffmanEncoder(literalCount),
		tmpLitEncoding:  newHuffmanEncoder(literalCount),
		codegenEncoding: newHuffmanEncoder(codegenCodeCount),
		offsetEncoding:  newHuffmanEncoder(offsetCodeCount),
	}
}

// reset the huffmanBitWriter state and replace the output.
func (w *huffmanBitWriter) reset(writer io.Writer) {
	w.writer = writer
	w.bits, w.nbits, w.nbytes, w.err = 0, 0, 0, nil
	w.prevHeader = 0
	w.wroteHuffman = false
}

// canReuse checks if the current generated tables can be
// reused for the provided tokens.
func (w *huffmanBitWriter) canReuse(t *tokens) (ok bool) {
	a := t.offHist[:offsetCodeCount]
	b := w.offsetEncoding.codes
	b = b[:len(a)]
	for i, v := range a {
		if v != 0 && b[i].zero() {
			return false
		}
	}

	a = t.extraHist[:literalCount-256]
	b = w.literalEncoding.codes[256:literalCount]
	b = b[:len(a)]
	for i, v := range a {
		if v != 0 && b[i].zero() {
			return false
		}
	}

	a = t.litHist[:256]
	b = w.literalEncoding.codes[:len(a)]
	for i, v := range a {
		if v != 0 && b[i].zero() {
			return false
		}
	}
	return true
}

// flush flushes the currently encoded data.
// An EOB will be written if the current block hasn't been ended.
func (w *huffmanBitWriter) flush() {
	if w.err != nil {
		w.nbits = 0
		return
	}
	if w.prevHeader > 0 {
		// We owe an EOB
		w.writeCode(w.literalEncoding.codes[endBlockMarker])
		w.prevHeader = 0
	}
	n := w.nbytes
	for w.nbits != 0 {
		w.bytes[n] = byte(w.bits)
		w.bits >>= 8
		if w.nbits > 8 { // Avoid underflow
			w.nbits -= 8
		} else {
			w.nbits = 0
		}
		n++
	}
	w.bits = 0
	if n > 0 {
		w.write(w.bytes[:n])
	}
	w.nbytes = 0
}

// write writes the provided bytes directly to the output,
// ignoring all queued bytes.
func (w *huffmanBitWriter) write(b []byte) {
	if w.err != nil {
		return
	}
	_, w.err = w.writer.Write(b)
}

// writeBits writes nb bits from b to the stream.
func (w *huffmanBitWriter) writeBits(b int32, nb uint8) {
	w.bits |= uint64(b) << (w.nbits & 63)
	w.nbits += nb
	if w.nbits >= 48 {
		w.flushBits()
	}
}

// writeBytes writes the provided bytes to the stream.
func (w *huffmanBitWriter) writeBytes(bytes []byte) {
	if w.err != nil {
		return
	}
	n := w.nbytes
	if w.nbits&7 != 0 {
		w.err = InternalError("writeBytes with unfinished bits")
		return
	}
	for w.nbits != 0 {
		w.bytes[n] = byte(w.bits)
		w.bits >>= 8
		w.nbits -= 8
		n++
	}
	if n != 0 {
		w.write(w.bytes[:n])
	}
	w.nbytes = 0
	w.write(bytes)
}

// RFC 1951 3.2.7 specifies a special run-length encoding for specifying
// the literal and offset lengths arrays (which are concatenated into a single
// array).  This method generates that run-length encoding.
//
// The result is written into the codegen array, and the frequencies
// of each code is written into the codegenFreq array.
// Codes 0-15 are single byte codes. Codes 16-18 are followed by additional
// information. Code badCode is an end marker
//
//	numLiterals      The number of literals in literalEncoding
//	numOffsets       The number of offsets in offsetEncoding
//	litenc, offenc   The literal and offset encoder to use
func (w *huffmanBitWriter) generateCodegen(numLiterals int, numOffsets int, litEnc, offEnc *huffmanEncoder) {
	clear(w.codegenFreq[:])
	// Note that we are using codegen both as a temporary variable for holding
	// a copy of the frequencies, and as the place where we put the result.
	// This is fine because the output is always shorter than the input used
	// so far.
	codegen := w.codegen[:] // cache
	// Copy the concatenated code sizes to codegen. Put a marker at the end.
	cgnl := codegen[:numLiterals]
	for i := range cgnl {
		cgnl[i] = litEnc.codes[i].len()
	}

	cgnl = codegen[numLiterals : numLiterals+numOffsets]
	for i := range cgnl {
		cgnl[i] = offEnc.codes[i].len()
	}
	codegen[numLiterals+numOffsets] = badCode

	size := codegen[0]
	count := 1
	outIndex := 0
	for inIndex := 1; size != badCode; inIndex++ {
		// INVARIANT: We have seen "count" copies of size that have not yet
		// had output generated for them.
		nextSize := codegen[inIndex]
		if nextSize == size {
			count++
			continue
		}
		// We need to generate codegen indicating "count" of size.
		if size != 0 {
			codegen[outIndex] = size
			outIndex++
			w.codegenFreq[size]++
			count--
			for count >= 3 {
				n := min(6, count)
				codegen[outIndex] = 16
				outIndex++
				codegen[outIndex] = uint8(n - 3)
				outIndex++
				w.codegenFreq[16]++
				count -= n
			}
		} else {
			for count >= 11 {
				n := min(138, count)
				codegen[outIndex] = 18
				outIndex++
				codegen[outIndex] = uint8(n - 11)
				outIndex++
				w.codegenFreq[18]++
				count -= n
			}
			if count >= 3 {
				// count >= 3 && count <= 10
				codegen[outIndex] = 17
				outIndex++
				codegen[outIndex] = uint8(count - 3)
				outIndex++
				w.codegenFreq[17]++
				count = 0
			}
		}
		count--
		for ; count >= 0; count-- {
			codegen[outIndex] = size
			outIndex++
			w.codegenFreq[size]++
		}
		// Set up invariant for next time through the loop.
		size = nextSize
		count = 1
	}
	// Marker indicating the end of the codegen.
	codegen[outIndex] = badCode
}

// codegens returns current number of non-zero codegens.
func (w *huffmanBitWriter) codegens() int {
	numCodegens := len(w.codegenFreq)
	for numCodegens > 4 && w.codegenFreq[codegenOrder[numCodegens-1]] == 0 {
		numCodegens--
	}
	return numCodegens
}

// headerSize returns the size of the header with the current encodings.
func (w *huffmanBitWriter) headerSize() (size, numCodegens int) {
	numCodegens = len(w.codegenFreq)
	for numCodegens > 4 && w.codegenFreq[codegenOrder[numCodegens-1]] == 0 {
		numCodegens--
	}
	return 3 + 5 + 5 + 4 + (3 * numCodegens) +
		w.codegenEncoding.bitLength(w.codegenFreq[:]) +
		int(w.codegenFreq[16])*2 +
		int(w.codegenFreq[17])*3 +
		int(w.codegenFreq[18])*7, numCodegens
}

// dynamicSize returns the size of dynamically encoded data in bits.
func (w *huffmanBitWriter) dynamicReuseSize(litEnc, offEnc *huffmanEncoder) (size int) {
	size = litEnc.bitLength(w.literalFreq[:]) +
		offEnc.bitLength(w.offsetFreq[:])
	return size
}

// dynamicSize returns the size of dynamically encoded data in bits.
func (w *huffmanBitWriter) dynamicSize(litEnc, offEnc *huffmanEncoder, extraBits int) (size, numCodegens int) {
	header, numCodegens := w.headerSize()
	size = header +
		litEnc.bitLength(w.literalFreq[:]) +
		offEnc.bitLength(w.offsetFreq[:]) +
		extraBits
	return size, numCodegens
}

// extraBitSize returns the number of bits that will be written
// as "extra" bits on matches.
func (w *huffmanBitWriter) extraBitSize() int {
	total := 0
	for i, n := range w.literalFreq[257:literalCount] {
		total += int(n) * int(lengthExtraBits[i&31])
	}
	for i, n := range w.offsetFreq[:offsetCodeCount] {
		total += int(n) * int(offsetExtraBits[i&31])
	}
	return total
}

// fixedSize returns the size of dynamically encoded data in bits.
func (w *huffmanBitWriter) fixedSize(extraBits int) int {
	return 3 +
		fixedLiteralEncoding().bitLength(w.literalFreq[:]) +
		fixedOffsetEncoding().bitLength(w.offsetFreq[:]) +
		extraBits
}

// storedSize calculates the stored size, including header.
// The function returns the size in bits and whether the block
// fits inside a single block.
func (w *huffmanBitWriter) storedSize(in []byte) (int, bool) {
	if in == nil {
		return 0, false
	}
	if len(in) <= maxStoreBlockSize {
		return (len(in) + 5) * 8, true
	}
	return 0, false
}

// writeCode writes 'c' to the stream.
func (w *huffmanBitWriter) writeCode(c hcode) {
	w.bits |= c.code64() << (w.nbits & reg8SizeMask64)
	w.nbits += c.len()
	if w.nbits >= 48 {
		w.flushBits()
	}
}

// flushBits writes accumulated bits to the byte buffer.
func (w *huffmanBitWriter) flushBits() {
	bits := w.bits
	w.bits >>= 48
	w.nbits -= 48
	n := w.nbytes

	// We overwrite, but faster...
	storeLE64(w.bytes[n:], bits)
	n += 6

	if n >= bufferFlushSize {
		if w.err != nil {
			n = 0
			return
		}
		w.write(w.bytes[:n])
		n = 0
	}

	w.nbytes = n
}

// writeDynamicHeader writes the header of a dynamic Huffman block to the output stream.
//
// numLiterals is the number of literals specified in codegen.
// numOffsets is the number of offsets specified in codegen.
// numCodegens is the number of codegens used in codegen.
func (w *huffmanBitWriter) writeDynamicHeader(numLiterals int, numOffsets int, numCodegens int, isEof bool) {
	if w.err != nil {
		return
	}
	var firstBits int32 = 4
	if isEof {
		firstBits = 5
	}
	w.writeBits(firstBits, 3)
	w.writeBits(int32(numLiterals-257), 5)
	w.writeBits(int32(numOffsets-1), 5)
	w.writeBits(int32(numCodegens-4), 4)

	for i := range numCodegens {
		value := uint(w.codegenEncoding.codes[codegenOrder[i]].len())
		w.writeBits(int32(value), 3)
	}

	i := 0
	for {
		var codeWord = uint32(w.codegen[i])
		i++
		if codeWord == badCode {
			break
		}
		w.writeCode(w.codegenEncoding.codes[codeWord])

		switch codeWord {
		case 16:
			w.writeBits(int32(w.codegen[i]), 2)
			i++
		case 17:
			w.writeBits(int32(w.codegen[i]), 3)
			i++
		case 18:
			w.writeBits(int32(w.codegen[i]), 7)
			i++
		}
	}
}

// writeStoredHeader writes a stored header.
// If the stored block is only used for EOF,
// it is replaced with a fixed huffman block.
func (w *huffmanBitWriter) writeStoredHeader(length int, isEof bool) {
	if w.err != nil {
		return
	}
	if w.prevHeader > 0 {
		// We owe an EOB
		w.writeCode(w.literalEncoding.codes[endBlockMarker])
		w.prevHeader = 0
	}

	// To write EOF, use a fixed encoding block. 10 bits instead of 5 bytes.
	if length == 0 && isEof {
		w.writeFixedHeader(isEof)
		// EOB: 7 bits, value: 0
		w.writeBits(0, 7)
		w.flush()
		return
	}

	var flag int32
	if isEof {
		flag = 1
	}
	w.writeBits(flag, 3)
	w.flush()
	w.writeBits(int32(length), 16)
	w.writeBits(int32(^uint16(length)), 16)
}

// writeFixedHeader writes a fixed encoding header to the output stream.
func (w *huffmanBitWriter) writeFixedHeader(isEof bool) {
	if w.err != nil {
		return
	}
	if w.prevHeader > 0 {
		// We owe an EOB
		w.writeCode(w.literalEncoding.codes[endBlockMarker])
		w.prevHeader = 0
	}

	// Indicate that we are a fixed Huffman block
	var value int32 = 2
	if isEof {
		value = 3
	}
	w.writeBits(value, 3)
}

// writeBlock writes a block of tokens using the smallest encoding.
// The original input can be supplied, and if the Huffman-encoded data
// is larger than the original bytes, the data will be written as a
// stored block.
// If the input is nil, the tokens will always be Huffman encoded.
func (w *huffmanBitWriter) writeBlock(tokens *tokens, eof bool, input []byte) {
	if w.err != nil {
		return
	}

	tokens.AddEOB()
	if w.prevHeader > 0 {
		// We owe an EOB
		w.writeCode(w.literalEncoding.codes[endBlockMarker])
		w.prevHeader = 0
	}
	numLiterals, numOffsets := w.indexTokens(tokens)
	w.generate()
	var extraBits int
	storedSize, storable := w.storedSize(input)
	if storable {
		extraBits = w.extraBitSize()
	}

	// Figure out smallest code.
	// Fixed Huffman baseline.
	var literalEncoding = fixedLiteralEncoding()
	var offsetEncoding = fixedOffsetEncoding()
	var size = math.MaxInt32
	if tokens.n < maxPredefinedTokens {
		size = w.fixedSize(extraBits)
	}

	// Dynamic Huffman?
	var numCodegens int

	// Generate codegen and codegenFrequencies, which indicates how to encode
	// the literalEncoding and the offsetEncoding.
	w.generateCodegen(numLiterals, numOffsets, w.literalEncoding, w.offsetEncoding)
	w.codegenEncoding.generate(w.codegenFreq[:], 7)
	dynamicSize, numCodegens := w.dynamicSize(w.literalEncoding, w.offsetEncoding, extraBits)

	if dynamicSize < size {
		size = dynamicSize
		literalEncoding = w.literalEncoding
		offsetEncoding = w.offsetEncoding
	}

	// Stored bytes?
	if storable && storedSize <= size {
		w.writeStoredHeader(len(input), eof)
		w.writeBytes(input)
		return
	}

	// Huffman.
	if literalEncoding == fixedLiteralEncoding() {
		w.writeFixedHeader(eof)
	} else {
		w.writeDynamicHeader(numLiterals, numOffsets, numCodegens, eof)
	}

	// Write the tokens.
	w.writeTokens(tokens.Slice(), literalEncoding.codes, offsetEncoding.codes)
}

// writeBlockDynamic encodes a block using a dynamic Huffman table.
// This should be used if the symbols used have a disproportionate
// histogram distribution.
func (w *huffmanBitWriter) writeBlockDynamic(tokens *tokens, eof bool, input []byte, sync bool) {
	if w.err != nil {
		return
	}

	sync = sync || eof
	if sync {
		tokens.AddEOB()
	} else {
		// Ensure we can always write EOB.
		tokens.extraHist[0] = 1
	}

	// We cannot reuse pure Huffman table, and must mark as EOF.
	if (w.wroteHuffman || eof) && w.prevHeader > 0 {
		// We will not try to reuse.
		w.writeCode(w.literalEncoding.codes[endBlockMarker])
		w.prevHeader = 0
		w.wroteHuffman = false
	}

	if w.prevHeader > 0 && !w.canReuse(tokens) {
		w.writeCode(w.literalEncoding.codes[endBlockMarker])
		w.prevHeader = 0
	}

	numLiterals, numOffsets := w.indexTokens(tokens)
	extraBits := 0
	ssize, storable := w.storedSize(input)

	if storable || w.prevHeader > 0 {
		extraBits = w.extraBitSize()
	}

	var size int

	// Check whether we should reuse the previous Huffman table.
	if w.prevHeader > 0 {
		// Estimate size for using a new table.
		// Use the previous header size as the best estimate.
		newSize := w.prevHeader + tokens.EstimatedBits()

		// The estimated size is calculated as an optimal table.
		// We add a penalty to make it more realistic and re-use a bit more.
		newSize += int(w.literalEncoding.codes[endBlockMarker].len()) + newSize>>w.logNewTablePenalty

		// Calculate the size for reusing the current table.
		reuseSize := w.dynamicReuseSize(w.literalEncoding, w.offsetEncoding) + extraBits

		// Check if a new table is better.
		if newSize < reuseSize {
			// Write the EOB we owe.
			w.writeCode(w.literalEncoding.codes[endBlockMarker])
			size = newSize
			w.prevHeader = 0
		} else {
			size = reuseSize
		}

		// Small blocks can be more efficient with fixed encoding.
		if tokens.n < maxPredefinedTokens {
			if preSize := w.fixedSize(extraBits) + 7; preSize < size {
				// Check if we get a reasonable size decrease.
				if storable && ssize <= size {
					w.writeStoredHeader(len(input), eof)
					w.writeBytes(input)
					return
				}
				w.writeFixedHeader(eof)
				if !sync {
					tokens.AddEOB()
				}
				w.writeTokens(tokens.Slice(), fixedLiteralEncoding().codes, fixedOffsetEncoding().codes)
				return
			}
		}

		// Check if we get a reasonable size decrease.
		if storable && ssize <= size {
			w.writeStoredHeader(len(input), eof)
			w.writeBytes(input)
			return
		}
	}

	// We want a new block/table
	if w.prevHeader == 0 {
		w.literalFreq[endBlockMarker] = 1

		w.generate()
		// Generate codegen and codegenFrequencies, which indicates how to encode
		// the literalEncoding and the offsetEncoding.
		w.generateCodegen(numLiterals, numOffsets, w.literalEncoding, w.offsetEncoding)
		w.codegenEncoding.generate(w.codegenFreq[:], 7)

		var numCodegens int
		size, numCodegens = w.dynamicSize(w.literalEncoding, w.offsetEncoding, extraBits)

		// Store predefined or raw, if we don't get a reasonable improvement.
		if tokens.n < maxPredefinedTokens {
			if preSize := w.fixedSize(extraBits); preSize <= size {
				// Store bytes, if we don't get an improvement.
				if storable && ssize <= preSize {
					w.writeStoredHeader(len(input), eof)
					w.writeBytes(input)
					return
				}
				w.writeFixedHeader(eof)
				if !sync {
					tokens.AddEOB()
				}
				w.writeTokens(tokens.Slice(), fixedLiteralEncoding().codes, fixedOffsetEncoding().codes)
				return
			}
		}

		if storable && ssize <= size {
			// Store bytes, if we don't get an improvement.
			w.writeStoredHeader(len(input), eof)
			w.writeBytes(input)
			return
		}

		// Write Huffman table.
		w.writeDynamicHeader(numLiterals, numOffsets, numCodegens, eof)
		if !sync {
			w.prevHeader, _ = w.headerSize()
		}
		w.wroteHuffman = false
	}

	if sync {
		w.prevHeader = 0
	}
	// Write the tokens.
	w.writeTokens(tokens.Slice(), w.literalEncoding.codes, w.offsetEncoding.codes)
}

// indexTokens indexes a slice of tokens, updates literalFreq and offsetFreq,
// and generates literalEncoding and offsetEncoding.
// It returns the number of literal and offset tokens.
func (w *huffmanBitWriter) indexTokens(t *tokens) (numLiterals, numOffsets int) {
	*(*[256]uint16)(w.literalFreq[:]) = t.litHist
	*(*[32]uint16)(w.literalFreq[256:]) = t.extraHist
	w.offsetFreq = t.offHist

	if t.n == 0 {
		return
	}
	// get the number of literals
	numLiterals = len(w.literalFreq)
	for w.literalFreq[numLiterals-1] == 0 {
		numLiterals--
	}
	// get the number of offsets
	numOffsets = len(w.offsetFreq)
	for numOffsets > 0 && w.offsetFreq[numOffsets-1] == 0 {
		numOffsets--
	}
	if numOffsets == 0 {
		// We haven't found a single match. If we want to go with the dynamic encoding,
		// we should count at least one offset to be sure that the offset huffman tree could be encoded.
		w.offsetFreq[0] = 1
		numOffsets = 1
	}
	return
}

// generate literalEncoding and offsetEncoding based on respective histograms.
func (w *huffmanBitWriter) generate() {
	w.literalEncoding.generate(w.literalFreq[:literalCount], 15)
	w.offsetEncoding.generate(w.offsetFreq[:offsetCodeCount], 15)
}

// writeTokens writes a slice of tokens to the output.
// Codes for literal and offset encoding must be supplied.
func (w *huffmanBitWriter) writeTokens(tokens []token, lenCodes, offCodes []hcode) {
	if w.err != nil {
		return
	}
	if len(tokens) == 0 {
		return
	}

	// Only last token should be endBlockMarker.
	var deferEOB bool
	if tokens[len(tokens)-1] == endBlockMarker {
		tokens = tokens[:len(tokens)-1]
		deferEOB = true
	}

	// Create slices up to the next power of two to avoid bounds checks.
	lits := lenCodes[:256]
	offs := offCodes[:32]
	lengths := lenCodes[lengthCodesStart:]
	lengths = lengths[:32]

	// Go 1.16 LOVES having these on stack.
	bits, nbits, nbytes := w.bits, w.nbits, w.nbytes

	for _, t := range tokens {
		if t < 256 {
			c := lits[t]
			bits |= c.code64() << (nbits & 63)
			nbits += c.len()
			if nbits >= 48 {
				storeLE64(w.bytes[nbytes:], bits)
				bits >>= 48
				nbits -= 48
				nbytes += 6
				if nbytes >= bufferFlushSize {
					if w.err != nil {
						nbytes = 0
						return
					}
					_, w.err = w.writer.Write(w.bytes[:nbytes])
					nbytes = 0
				}
			}
			continue
		}

		// Write the length
		length := t.length()
		lenCode := lengthCode(length) & 31
		// inlined 'w.writeCode(lengths[lengthCode])'
		c := lengths[lenCode]
		bits |= c.code64() << (nbits & 63)
		nbits += c.len()
		if nbits >= 48 {
			storeLE64(w.bytes[nbytes:], bits)
			bits >>= 48
			nbits -= 48
			nbytes += 6
			if nbytes >= bufferFlushSize {
				if w.err != nil {
					nbytes = 0
					return
				}
				_, w.err = w.writer.Write(w.bytes[:nbytes])
				nbytes = 0
			}
		}

		if lenCode >= lengthExtraBitsMinCode {
			extraLengthBits := lengthExtraBits[lenCode]
			//w.writeBits(extraLength, extraLengthBits)
			extraLength := int32(length - lengthBase[lenCode])
			bits |= uint64(extraLength) << (nbits & 63)
			nbits += extraLengthBits
			if nbits >= 48 {
				storeLE64(w.bytes[nbytes:], bits)
				bits >>= 48
				nbits -= 48
				nbytes += 6
				if nbytes >= bufferFlushSize {
					if w.err != nil {
						nbytes = 0
						return
					}
					_, w.err = w.writer.Write(w.bytes[:nbytes])
					nbytes = 0
				}
			}
		}
		// Write the offset
		offset := t.offset()
		offCode := (offset >> 16) & 31
		// inlined 'w.writeCode(offs[offCode])'
		c = offs[offCode]
		bits |= c.code64() << (nbits & 63)
		nbits += c.len()
		if nbits >= 48 {
			storeLE64(w.bytes[nbytes:], bits)
			bits >>= 48
			nbits -= 48
			nbytes += 6
			if nbytes >= bufferFlushSize {
				if w.err != nil {
					nbytes = 0
					return
				}
				_, w.err = w.writer.Write(w.bytes[:nbytes])
				nbytes = 0
			}
		}

		if offCode >= offsetExtraBitsMinCode {
			offsetComb := offsetCombined[offCode]
			bits |= uint64((offset-(offsetComb>>8))&matchOffsetOnlyMask) << (nbits & 63)
			nbits += uint8(offsetComb)
			if nbits >= 48 {
				storeLE64(w.bytes[nbytes:], bits)
				bits >>= 48
				nbits -= 48
				nbytes += 6
				if nbytes >= bufferFlushSize {
					if w.err != nil {
						nbytes = 0
						return
					}
					_, w.err = w.writer.Write(w.bytes[:nbytes])
					nbytes = 0
				}
			}
		}
	}
	// Restore...
	w.bits, w.nbits, w.nbytes = bits, nbits, nbytes

	if deferEOB {
		w.writeCode(lenCodes[endBlockMarker])
	}
}

// huffOffset is a static offset encoder used for Huffman-only encoding.
// It can be reused since we will not be encoding offset values.
var huffOffset = sync.OnceValue(func() *huffmanEncoder {
	w := newHuffmanBitWriter(nil)
	w.offsetFreq[0] = 1
	h := newHuffmanEncoder(offsetCodeCount)
	h.generate(w.offsetFreq[:offsetCodeCount], 15)
	return h
})

// writeBlockHuff encodes a block of bytes as either
// Huffman-encoded literals or uncompressed bytes if the
// results gain very little from compression.
func (w *huffmanBitWriter) writeBlockHuff(eof bool, input []byte, sync bool) {
	if w.err != nil {
		return
	}

	// Clear histogram
	clear(w.literalFreq[:])
	if !w.wroteHuffman {
		clear(w.offsetFreq[:])
	}

	const numLiterals = endBlockMarker + 1
	const numOffsets = 1

	// Estimate size of literal encoding.
	const guessHeaderSizeBits = 70 * 8 // 70 bytes; see https://stackoverflow.com/a/25454430
	histogram(input, w.literalFreq[:numLiterals])
	ssize, storable := w.storedSize(input)
	if storable && len(input) > 1024 {
		// Quick check for incompressible content.
		// The following checks if all frequencies lie
		// close to the average frequency.
		// If so, we quickly store the data uncompressed.
		// This will typically only trigger on random data.
		// Most other data will typically exit after only a few iterations.
		abs := float64(0)
		avg := float64(len(input)) / 256
		max := float64(len(input) * 2)
		for _, v := range w.literalFreq[:256] {
			diff := float64(v) - avg
			abs += diff * diff
			if abs >= max {
				break
			}
		}
		if abs < max {
			// No chance we can compress this...
			w.writeStoredHeader(len(input), eof)
			w.writeBytes(input)
			return
		}
	}
	w.literalFreq[endBlockMarker] = 1
	w.tmpLitEncoding.generate(w.literalFreq[:numLiterals], 15)
	estBits := w.tmpLitEncoding.canEncodeLen(w.literalFreq[:numLiterals])
	if estBits < math.MaxInt32 {
		estBits += w.prevHeader
		if w.prevHeader == 0 {
			estBits += guessHeaderSizeBits
		}
		estBits += estBits >> w.logNewTablePenalty
	}

	// Store bytes, if we don't get a reasonable improvement.
	if storable && ssize <= estBits {
		w.writeStoredHeader(len(input), eof)
		w.writeBytes(input)
		return
	}

	if w.prevHeader > 0 {
		reuseSize := w.literalEncoding.canEncodeLen(w.literalFreq[:256])
		if estBits < reuseSize {
			// We owe an EOB
			w.writeCode(w.literalEncoding.codes[endBlockMarker])
			w.prevHeader = 0
		}
	}

	if w.prevHeader == 0 {
		// Use the temp encoding, so swap.
		w.literalEncoding, w.tmpLitEncoding = w.tmpLitEncoding, w.literalEncoding
		// Generate codegen and codegenFrequencies, which indicates how to encode
		// the literalEncoding and the offsetEncoding.
		w.generateCodegen(numLiterals, numOffsets, w.literalEncoding, huffOffset())
		w.codegenEncoding.generate(w.codegenFreq[:], 7)
		numCodegens := w.codegens()

		// Huffman.
		w.writeDynamicHeader(numLiterals, numOffsets, numCodegens, eof)
		w.wroteHuffman = true
		w.prevHeader, _ = w.headerSize()
	}

	encoding := w.literalEncoding.codes[:256]
	// Go 1.16 LOVES having these on stack. At least 1.5x the speed.
	bits, nbits, nbytes := w.bits, w.nbits, w.nbytes

	// Unroll, write 3 codes/loop.
	// Fastest number of unrolls.
	for len(input) > 3 {
		// We must have at least 48 bits free.
		if nbits >= 8 {
			n := nbits >> 3
			storeLE64(w.bytes[nbytes:], bits)
			bits >>= (n * 8) & 63
			nbits -= n * 8
			nbytes += n
		}
		if nbytes >= bufferFlushSize {
			if w.err != nil {
				nbytes = 0
				return
			}
			_, w.err = w.writer.Write(w.bytes[:nbytes])
			nbytes = 0
		}
		a, b := encoding[input[0]], encoding[input[1]]
		bits |= a.code64() << (nbits & 63)
		bits |= b.code64() << ((nbits + a.len()) & 63)
		c := encoding[input[2]]
		nbits += b.len() + a.len()
		bits |= c.code64() << (nbits & 63)
		nbits += c.len()
		input = input[3:]
	}

	// Remaining...
	for _, t := range input {
		if nbits >= 48 {
			storeLE64(w.bytes[nbytes:], bits)
			bits >>= 48
			nbits -= 48
			nbytes += 6
			if nbytes >= bufferFlushSize {
				if w.err != nil {
					nbytes = 0
					return
				}
				_, w.err = w.writer.Write(w.bytes[:nbytes])
				nbytes = 0
			}
		}
		// Bitwriting inlined, ~30% speedup
		c := encoding[t]
		bits |= c.code64() << (nbits & 63)

		nbits += c.len()
	}
	// Restore...
	w.bits, w.nbits, w.nbytes = bits, nbits, nbytes

	// Flush if needed to have space.
	if w.nbits >= 48 {
		w.flushBits()
	}

	if eof || sync {
		w.writeCode(w.literalEncoding.codes[endBlockMarker])
		w.prevHeader = 0
		w.wroteHuffman = false
	}
}

```

// === FILE: references/go/src/compress/flate/huffman_code.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flate

import (
	"math"
	"math/bits"
	"slices"
	"sync"
)

const (
	maxBitsLimit = 16
	// number of valid literals
	literalCount = 286
)

// hcode is a huffman code with a bit code and bit length.
type hcode uint32

// len returns the length of the code in bits.
func (h hcode) len() uint8 {
	return uint8(h)
}

// code64 returns the code as a uint64.
func (h hcode) code64() uint64 {
	return uint64(h >> 8)
}

// zero returns true if the code is unset.
func (h hcode) zero() bool {
	return h == 0
}

// set sets the code and length of an hcode.
func (h *hcode) set(code uint16, length uint8) {
	*h = newhcode(code, length)
}

// newhcode combines a code and length into an hcode.
func newhcode(code uint16, length uint8) hcode {
	return hcode(length) | (hcode(code) << 8)
}

// huffmanEncoder provides a fast way to generate Huffman codes for a given
// frequency table.  It is based on the algorithm described in RFC 1951,
// section 3.2.2.
type huffmanEncoder struct {
	codes    []hcode
	bitCount [17]int32

	// freqcache is a reusable buffer with the longest possible frequency table.
	// Possible lengths are codegenCodeCount, offsetCodeCount and literalCount.
	// The largest of these is literalCount, so we allocate for that case.
	freqcache [literalCount + 1]literalNode
}

// newHuffmanEncoder returns a new huffmanEncoder with the given size.
func newHuffmanEncoder(size int) *huffmanEncoder {
	// Make capacity to next power of two.
	c := uint(bits.Len32(uint32(size - 1)))
	return &huffmanEncoder{codes: make([]hcode, size, 1<<c)}
}

// literalNode represents a literal node in the huffman tree.
type literalNode struct {
	literal uint16
	freq    uint16
}

// maxNode returns a literalNode with the maximum possible literal and frequency.
func maxNode() literalNode { return literalNode{math.MaxUint16, math.MaxUint16} }

// A levelInfo describes the state of the constructed tree for a given depth.
type levelInfo struct {
	// Our level.  for better printing
	level int32

	// The frequency of the last node at this level
	lastFreq int32

	// The frequency of the next character to add to this level
	nextCharFreq int32

	// The frequency of the next pair (from level below) to add to this level.
	// Only valid if the "needed" value of the next lower level is 0.
	nextPairFreq int32

	// The number of chains remaining to generate for this level before moving
	// up to the next level
	needed int32
}

// reverseBits returns the b-bit reversal of x.
// It shifts x into the top b bits, reverses all 16, leaving the result in the low b bits.
func reverseBits(x uint16, b byte) uint16 {
	return bits.Reverse16(x << ((16 - b) & 15))
}

// generateFixedLiteralEncoding returns the encoder for the fixed literal table.
func generateFixedLiteralEncoding() *huffmanEncoder {
	h := newHuffmanEncoder(literalCount)
	codes := h.codes
	var ch uint16
	for ch = range uint16(literalCount) {
		var bits uint16
		var size uint8
		switch {
		case ch < 144:
			// size 8, 000110000  .. 10111111
			bits = ch + 48
			size = 8
		case ch < 256:
			// size 9, 110010000 .. 111111111
			bits = ch + 400 - 144
			size = 9
		case ch < 280:
			// size 7, 0000000 .. 0010111
			bits = ch - 256
			size = 7
		default:
			// size 8, 11000000 .. 11000111
			bits = ch + 192 - 280
			size = 8
		}
		codes[ch] = newhcode(reverseBits(bits, size), size)
	}
	return h
}

func generateFixedOffsetEncoding() *huffmanEncoder {
	h := newHuffmanEncoder(30)
	codes := h.codes
	for ch := range codes {
		codes[ch] = newhcode(reverseBits(uint16(ch), 5), 5)
	}
	return h
}

var (
	fixedLiteralEncoding = sync.OnceValue(generateFixedLiteralEncoding)
	fixedOffsetEncoding  = sync.OnceValue(generateFixedOffsetEncoding)
)

// bitLength returns the number of bits needed to encode freq.
func (h *huffmanEncoder) bitLength(freq []uint16) int {
	var total int
	for i, f := range freq {
		if f != 0 {
			total += int(f) * int(h.codes[i].len())
		}
	}
	return total
}

// bitLengthRaw will return the number of bits needed to encode b.
// For unset codes 1 bit/entry will be added.
func (h *huffmanEncoder) bitLengthRaw(b []byte) int {
	var total int
	for _, f := range b {
		total += max(1, int(h.codes[f].len()))
	}
	return total
}

// canEncodeLen returns the number of bits to encode freq.
// It returns math.MaxInt32 if freq cannot be encoded.
func (h *huffmanEncoder) canEncodeLen(freq []uint16) int {
	var total int
	for i, f := range freq {
		if f != 0 {
			code := h.codes[i]
			if code.zero() {
				return math.MaxInt32
			}
			total += int(f) * int(code.len())
		}
	}
	return total
}

// bitCounts returns an integer slice in which slice[i] is the number
// of literals that should be encoded using i bits.
//
// This method is only called when len(list) >= 3.
// The cases of 0, 1, and 2 literals are handled by special case code.
//
// list is an array of the literals with non-zero frequencies
// and their associated frequencies. The array is in order of increasing
// frequency and has as its last element a special element with frequency
// MaxInt32.
//
// maxBits is the maximum number of bits that should be used to encode any literal.
// It must be less than 16.
func (h *huffmanEncoder) bitCounts(list []literalNode, maxBits int32) []int32 {
	if maxBits >= maxBitsLimit {
		panic("flate: maxBits too large")
	}
	n := int32(len(list))
	list = list[0 : n+1]
	list[n] = maxNode()

	// The tree can't have greater depth than n - 1, no matter what. This
	// saves a little bit of work in some small cases
	if maxBits > n-1 {
		maxBits = n - 1
	}

	// Create information about each of the levels.
	// A bogus "Level 0" whose sole purpose is so that
	// level1.prev.needed==0.  This makes level1.nextPairFreq
	// be a legitimate value that never gets chosen.
	var levels [maxBitsLimit]levelInfo
	// leafCounts[i] counts the number of literals at the left
	// of ancestors of the rightmost node at level i.
	// leafCounts[i][j] is the number of literals at the left
	// of the level j ancestor.
	var leafCounts [maxBitsLimit][maxBitsLimit]int32

	_ = list[2] // check bounds here instead of in loop
	for level := int32(1); level <= maxBits; level++ {
		// For every level, the first two items are the first two characters.
		// We initialize the levels as if we had already figured this out.
		levels[level] = levelInfo{
			level:        level,
			lastFreq:     int32(list[1].freq),
			nextCharFreq: int32(list[2].freq),
			nextPairFreq: int32(list[0].freq) + int32(list[1].freq),
		}
		leafCounts[level][level] = 2
		if level == 1 {
			levels[level].nextPairFreq = math.MaxInt32
		}
	}

	// We need a total of 2*n - 2 items at top level and have already generated 2.
	levels[maxBits].needed = 2*n - 4

	level := uint32(maxBits)
	for level < 16 {
		l := &levels[level]
		if l.nextPairFreq == math.MaxInt32 && l.nextCharFreq == math.MaxInt32 {
			// We've run out of both leafs and pairs.
			// End all calculations for this level.
			// To make sure we never come back to this level or any lower level,
			// set nextPairFreq impossibly large.
			l.needed = 0
			levels[level+1].nextPairFreq = math.MaxInt32
			level++
			continue
		}

		prevFreq := l.lastFreq
		if l.nextCharFreq < l.nextPairFreq {
			// The next item on this row is a leaf node.
			n := leafCounts[level][level] + 1
			l.lastFreq = l.nextCharFreq
			// Lower leafCounts are the same of the previous node.
			leafCounts[level][level] = n
			e := list[n]
			if e.literal < math.MaxUint16 {
				l.nextCharFreq = int32(e.freq)
			} else {
				l.nextCharFreq = math.MaxInt32
			}
		} else {
			// The next item on this row is a pair from the previous row.
			// nextPairFreq isn't valid until we generate two
			// more values in the level below
			l.lastFreq = l.nextPairFreq
			// Take leaf counts from the lower level, except counts[level] remains the same.
			save := leafCounts[level][level]
			leafCounts[level] = leafCounts[level-1]
			leafCounts[level][level] = save
			levels[l.level-1].needed = 2
		}

		if l.needed--; l.needed == 0 {
			// We've done everything we need to do for this level.
			// Continue calculating one level up. Fill in nextPairFreq
			// of that level with the sum of the two nodes we've just calculated on
			// this level.
			if l.level == maxBits {
				// All done!
				break
			}
			levels[l.level+1].nextPairFreq = prevFreq + l.lastFreq
			level++
		} else {
			// If we stole from below, move down temporarily to replenish it.
			for levels[level-1].needed > 0 {
				level--
			}
		}
	}

	// Somethings is wrong if at the end, the top level is null or hasn't used
	// all of the leaves.
	if leafCounts[maxBits][maxBits] != n {
		panic("leafCounts[maxBits][maxBits] != n")
	}

	bitCount := h.bitCount[:maxBits+1]
	bits := 1
	counts := &leafCounts[maxBits]
	for level := maxBits; level > 0; level-- {
		// chain.leafCount gives the number of literals requiring at least "bits"
		// bits to encode.
		bitCount[bits] = counts[level] - counts[level-1]
		bits++
	}
	return bitCount
}

// assignEncodingAndSize assigns bit counts and encodings to the leaves
// as specified in RFC 1951 3.2.2.
func (h *huffmanEncoder) assignEncodingAndSize(bitCount []int32, list []literalNode) {
	code := uint16(0)
	for n, bits := range bitCount {
		code <<= 1
		if n == 0 || bits == 0 {
			continue
		}
		// The literals list[len(list)-bits] .. list[len(list)-bits]
		// are encoded using "bits" bits, and get the values
		// code, code + 1, ....  The code values are
		// assigned in literal order (not frequency order).
		chunk := list[len(list)-int(bits):]

		slices.SortFunc(chunk, func(a, b literalNode) int {
			return int(a.literal) - int(b.literal)
		})
		for _, node := range chunk {
			h.codes[node.literal] = newhcode(reverseBits(code, uint8(n)), uint8(n))
			code++
		}
		list = list[0 : len(list)-int(bits)]
	}
}

// generate rewrites h to be the Huffman code for the given frequency count.
// freq[i] is the frequency of literal i, and maxBits is the maximum number
// of bits to use for any literal.
func (h *huffmanEncoder) generate(freq []uint16, maxBits int32) {
	list := h.freqcache[:len(freq)+1]
	codes := h.codes[:len(freq)]
	// Number of non-zero literals
	count := 0
	// Set list to be the set of all non-zero literals and their frequencies
	for i, f := range freq {
		if f != 0 {
			list[count] = literalNode{uint16(i), f}
			count++
		} else {
			codes[i] = 0
		}
	}
	list[count] = literalNode{}

	list = list[:count]
	if count <= 2 {
		// Handle the small cases here, because they are awkward for the general case code. With
		// two or fewer literals, everything has bit length 1.
		for i, node := range list {
			// "list" is in order of increasing literal value.
			h.codes[node.literal].set(uint16(i), 1)
		}
		return
	}
	slices.SortFunc(list, func(a, b literalNode) int {
		// Literals can be contained in 9 bits, so we shift freq to be branchless.
		return (int(a.freq)<<10 + int(a.literal)) - (int(b.freq)<<10 + int(b.literal))
	})

	// Get the number of literals for each bit count
	bitCount := h.bitCounts(list, maxBits)
	// And do the assignment
	h.assignEncodingAndSize(bitCount, list)
}

func histogram(b []byte, h []uint16) {
	if len(b) >= 8<<10 {
		histogramSplit(b, h)
		return
	}
	h = h[:256]
	for _, t := range b {
		h[t]++
	}
}

func histogramSplit(b []byte, h []uint16) {
	// Walk four quarters in parallel.
	// Tested to be faster than walking halves.
	h = h[:256]
	// Make size divisible by 4
	for len(b)&3 != 0 {
		h[b[0]]++
		b = b[1:]
	}
	n := len(b) / 4
	x, y, z, w := b[:n], b[n:], b[n+n:], b[n+n+n:]
	y, z, w = y[:len(x)], z[:len(x)], w[:len(x)]
	for i, t := range x {
		v0 := &h[t]
		v1 := &h[y[i]]
		v2 := &h[z[i]]
		v3 := &h[w[i]]
		*v0++
		*v1++
		*v2++
		*v3++
	}
}

```

// === FILE: references/go/src/compress/flate/inflate.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package flate implements the DEFLATE compressed data format, described in
// RFC 1951.  The [compress/gzip] and [compress/zlib] packages implement access
// to DEFLATE-based file formats.
package flate

import (
	"bufio"
	"io"
	"math/bits"
	"strconv"
	"sync"
)

const (
	maxCodeLen = 16 // max length of Huffman code
	// The next three numbers come from the RFC section 3.2.7, with the
	// additional proviso in section 3.2.5 which implies that distance codes
	// 30 and 31 should never occur in compressed data.
	maxNumLit  = 286
	maxNumDist = 30
	numCodes   = 19 // number of codes in Huffman meta-code
)

// Initialize the fixedHuffmanDecoder only once upon first use.
var fixedOnce sync.Once
var fixedHuffmanDecoder huffmanDecoder

// A CorruptInputError reports the presence of corrupt input at a given offset.
type CorruptInputError int64

func (e CorruptInputError) Error() string {
	return "flate: corrupt input before offset " + strconv.FormatInt(int64(e), 10)
}

// An InternalError reports an error in the flate code itself.
type InternalError string

func (e InternalError) Error() string { return "flate: internal error: " + string(e) }

// A ReadError reports an error encountered while reading input.
//
// Deprecated: No longer returned.
type ReadError struct {
	Offset int64 // byte offset where error occurred
	Err    error // error returned by underlying Read
}

func (e *ReadError) Error() string {
	return "flate: read error at offset " + strconv.FormatInt(e.Offset, 10) + ": " + e.Err.Error()
}

// A WriteError reports an error encountered while writing output.
//
// Deprecated: No longer returned.
type WriteError struct {
	Offset int64 // byte offset where error occurred
	Err    error // error returned by underlying Write
}

func (e *WriteError) Error() string {
	return "flate: write error at offset " + strconv.FormatInt(e.Offset, 10) + ": " + e.Err.Error()
}

// Resetter resets a ReadCloser returned by [NewReader] or [NewReaderDict]
// to switch to a new underlying [Reader]. This permits reusing a ReadCloser
// instead of allocating a new one.
type Resetter interface {
	// Reset discards any buffered data and resets the Resetter as if it was
	// newly initialized with the given reader.
	Reset(r io.Reader, dict []byte) error
}

// The data structure for decoding Huffman tables is based on that of
// zlib. There is a lookup table of a fixed bit width (huffmanChunkBits),
// For codes smaller than the table width, there are multiple entries
// (each combination of trailing bits has the same value). For codes
// larger than the table width, the table contains a link to an overflow
// table. The width of each entry in the link table is the maximum code
// size minus the chunk width.
//
// Note that you can do a lookup in the table even without all bits
// filled. Since the extra bits are zero, and the DEFLATE Huffman codes
// have the property that shorter codes come before longer ones, the
// bit length estimate in the result is a lower bound on the actual
// number of bits.
//
// See the following:
//	https://github.com/madler/zlib/raw/master/doc/algorithm.txt

// chunk & 15 is number of bits
// chunk >> 4 is value, including table link

const (
	huffmanChunkBits  = 9
	huffmanNumChunks  = 1 << huffmanChunkBits
	huffmanCountMask  = 15
	huffmanValueShift = 4
)

type huffmanDecoder struct {
	min      int                      // the minimum code length
	chunks   [huffmanNumChunks]uint32 // chunks as described above
	links    [][]uint32               // overflow links
	linkMask uint32                   // mask the width of the link table
}

// Initialize Huffman decoding tables from array of code lengths.
// Following this function, h is guaranteed to be initialized into a complete
// tree (i.e., neither over-subscribed nor under-subscribed). The exception is a
// degenerate case where the tree has only a single symbol with length 1. Empty
// trees are permitted.
func (h *huffmanDecoder) init(lengths []int) bool {
	// Sanity enables additional runtime tests during Huffman
	// table construction. It's intended to be used during
	// development to supplement the currently ad-hoc unit tests.
	const sanity = false

	if h.min != 0 {
		*h = huffmanDecoder{}
	}

	// Count number of codes of each length,
	// compute min and max length.
	var count [maxCodeLen]int
	var min, max int
	for _, n := range lengths {
		if n == 0 {
			continue
		}
		if min == 0 || n < min {
			min = n
		}
		if n > max {
			max = n
		}
		count[n]++
	}

	// Empty tree. The decompressor.huffSym function will fail later if the tree
	// is used. Technically, an empty tree is only valid for the HDIST tree and
	// not the HCLEN and HLIT tree. However, a stream with an empty HCLEN tree
	// is guaranteed to fail since it will attempt to use the tree to decode the
	// codes for the HLIT and HDIST trees. Similarly, an empty HLIT tree is
	// guaranteed to fail later since the compressed data section must be
	// composed of at least one symbol (the end-of-block marker).
	if max == 0 {
		return true
	}

	code := 0
	var nextcode [maxCodeLen]int
	for i := min; i <= max; i++ {
		code <<= 1
		nextcode[i] = code
		code += count[i]
	}

	// Check that the coding is complete (i.e., that we've
	// assigned all 2-to-the-max possible bit sequences).
	// Exception: To be compatible with zlib, we also need to
	// accept degenerate single-code codings. See also
	// TestDegenerateHuffmanCoding.
	if code != 1<<uint(max) && !(code == 1 && max == 1) {
		return false
	}

	h.min = min
	if max > huffmanChunkBits {
		numLinks := 1 << (uint(max) - huffmanChunkBits)
		h.linkMask = uint32(numLinks - 1)

		// create link tables
		link := nextcode[huffmanChunkBits+1] >> 1
		h.links = make([][]uint32, huffmanNumChunks-link)
		for j := uint(link); j < huffmanNumChunks; j++ {
			reverse := int(bits.Reverse16(uint16(j)))
			reverse >>= uint(16 - huffmanChunkBits)
			off := j - uint(link)
			if sanity && h.chunks[reverse] != 0 {
				panic("impossible: overwriting existing chunk")
			}
			h.chunks[reverse] = uint32(off<<huffmanValueShift | (huffmanChunkBits + 1))
			h.links[off] = make([]uint32, numLinks)
		}
	}

	for i, n := range lengths {
		if n == 0 {
			continue
		}
		code := nextcode[n]
		nextcode[n]++
		chunk := uint32(i<<huffmanValueShift | n)
		reverse := int(bits.Reverse16(uint16(code)))
		reverse >>= uint(16 - n)
		if n <= huffmanChunkBits {
			for off := reverse; off < len(h.chunks); off += 1 << uint(n) {
				// We should never need to overwrite
				// an existing chunk. Also, 0 is
				// never a valid chunk, because the
				// lower 4 "count" bits should be
				// between 1 and 15.
				if sanity && h.chunks[off] != 0 {
					panic("impossible: overwriting existing chunk")
				}
				h.chunks[off] = chunk
			}
		} else {
			j := reverse & (huffmanNumChunks - 1)
			if sanity && h.chunks[j]&huffmanCountMask != huffmanChunkBits+1 {
				// Longer codes should have been
				// associated with a link table above.
				panic("impossible: not an indirect chunk")
			}
			value := h.chunks[j] >> huffmanValueShift
			linktab := h.links[value]
			reverse >>= huffmanChunkBits
			for off := reverse; off < len(linktab); off += 1 << uint(n-huffmanChunkBits) {
				if sanity && linktab[off] != 0 {
					panic("impossible: overwriting existing chunk")
				}
				linktab[off] = chunk
			}
		}
	}

	if sanity {
		// Above we've sanity checked that we never overwrote
		// an existing entry. Here we additionally check that
		// we filled the tables completely.
		for i, chunk := range h.chunks {
			if chunk == 0 {
				// As an exception, in the degenerate
				// single-code case, we allow odd
				// chunks to be missing.
				if code == 1 && i%2 == 1 {
					continue
				}
				panic("impossible: missing chunk")
			}
		}
		for _, linktab := range h.links {
			for _, chunk := range linktab {
				if chunk == 0 {
					panic("impossible: missing chunk")
				}
			}
		}
	}

	return true
}

// The actual read interface needed by [NewReader].
// If the passed in [io.Reader] does not also have ReadByte,
// the [NewReader] will introduce its own buffering.
type Reader interface {
	io.Reader
	io.ByteReader
}

// Decompress state.
type decompressor struct {
	// Input source.
	r       Reader
	rBuf    *bufio.Reader // created if provided io.Reader does not implement io.ByteReader
	roffset int64

	// Input bits, in top of b.
	b  uint32
	nb uint

	// Huffman decoders for literal/length, distance.
	h1, h2 huffmanDecoder

	// Length arrays used to define Huffman codes.
	bits     *[maxNumLit + maxNumDist]int
	codebits *[numCodes]int

	// Output history, buffer.
	dict dictDecoder

	// Temporary buffer (avoids repeated allocation).
	buf [4]byte

	// Next step in the decompression,
	// and decompression state.
	step      func(*decompressor)
	stepState int
	final     bool
	err       error
	toRead    []byte
	hl, hd    *huffmanDecoder
	copyLen   int
	copyDist  int
}

func (f *decompressor) nextBlock() {
	for f.nb < 1+2 {
		if f.err = f.moreBits(); f.err != nil {
			return
		}
	}
	f.final = f.b&1 == 1
	f.b >>= 1
	typ := f.b & 3
	f.b >>= 2
	f.nb -= 1 + 2
	switch typ {
	case 0:
		f.dataBlock()
	case 1:
		// compressed, fixed Huffman tables
		f.hl = &fixedHuffmanDecoder
		f.hd = nil
		f.huffmanBlock()
	case 2:
		// compressed, dynamic Huffman tables
		if f.err = f.readHuffman(); f.err != nil {
			break
		}
		f.hl = &f.h1
		f.hd = &f.h2
		f.huffmanBlock()
	default:
		// 3 is reserved.
		f.err = CorruptInputError(f.roffset)
	}
}

func (f *decompressor) Read(b []byte) (int, error) {
	for {
		if len(f.toRead) > 0 {
			n := copy(b, f.toRead)
			f.toRead = f.toRead[n:]
			if len(f.toRead) == 0 {
				return n, f.err
			}
			return n, nil
		}
		if f.err != nil {
			return 0, f.err
		}
		f.step(f)
		if f.err != nil && len(f.toRead) == 0 {
			f.toRead = f.dict.readFlush() // Flush what's left in case of error
		}
	}
}

func (f *decompressor) Close() error {
	if f.err == io.EOF {
		return nil
	}
	return f.err
}

// RFC 1951 section 3.2.7.
// Compression with dynamic Huffman codes

var codeOrder = [...]int{16, 17, 18, 0, 8, 7, 9, 6, 10, 5, 11, 4, 12, 3, 13, 2, 14, 1, 15}

func (f *decompressor) readHuffman() error {
	// HLIT[5], HDIST[5], HCLEN[4].
	for f.nb < 5+5+4 {
		if err := f.moreBits(); err != nil {
			return err
		}
	}
	nlit := int(f.b&0x1F) + 257
	if nlit > maxNumLit {
		return CorruptInputError(f.roffset)
	}
	f.b >>= 5
	ndist := int(f.b&0x1F) + 1
	if ndist > maxNumDist {
		return CorruptInputError(f.roffset)
	}
	f.b >>= 5
	nclen := int(f.b&0xF) + 4
	// numCodes is 19, so nclen is always valid.
	f.b >>= 4
	f.nb -= 5 + 5 + 4

	// (HCLEN+4)*3 bits: code lengths in the magic codeOrder order.
	for i := 0; i < nclen; i++ {
		for f.nb < 3 {
			if err := f.moreBits(); err != nil {
				return err
			}
		}
		f.codebits[codeOrder[i]] = int(f.b & 0x7)
		f.b >>= 3
		f.nb -= 3
	}
	for i := nclen; i < len(codeOrder); i++ {
		f.codebits[codeOrder[i]] = 0
	}
	if !f.h1.init(f.codebits[0:]) {
		return CorruptInputError(f.roffset)
	}

	// HLIT + 257 code lengths, HDIST + 1 code lengths,
	// using the code length Huffman code.
	for i, n := 0, nlit+ndist; i < n; {
		x, err := f.huffSym(&f.h1)
		if err != nil {
			return err
		}
		if x < 16 {
			// Actual length.
			f.bits[i] = x
			i++
			continue
		}
		// Repeat previous length or zero.
		var rep int
		var nb uint
		var b int
		switch x {
		default:
			return InternalError("unexpected length code")
		case 16:
			rep = 3
			nb = 2
			if i == 0 {
				return CorruptInputError(f.roffset)
			}
			b = f.bits[i-1]
		case 17:
			rep = 3
			nb = 3
			b = 0
		case 18:
			rep = 11
			nb = 7
			b = 0
		}
		for f.nb < nb {
			if err := f.moreBits(); err != nil {
				return err
			}
		}
		rep += int(f.b & uint32(1<<nb-1))
		f.b >>= nb
		f.nb -= nb
		if i+rep > n {
			return CorruptInputError(f.roffset)
		}
		for j := 0; j < rep; j++ {
			f.bits[i] = b
			i++
		}
	}

	if !f.h1.init(f.bits[0:nlit]) || !f.h2.init(f.bits[nlit:nlit+ndist]) {
		return CorruptInputError(f.roffset)
	}

	// As an optimization, we can initialize the min bits to read at a time
	// for the HLIT tree to the length of the EOB marker since we know that
	// every block must terminate with one. This preserves the property that
	// we never read any extra bytes after the end of the DEFLATE stream.
	if f.h1.min < f.bits[endBlockMarker] {
		f.h1.min = f.bits[endBlockMarker]
	}

	return nil
}

// Decode a single Huffman block from f.
// hl and hd are the Huffman states for the lit/length values
// and the distance values, respectively. If hd == nil, using the
// fixed distance encoding associated with fixed Huffman blocks.
func (f *decompressor) huffmanBlock() {
	const (
		stateInit = iota // Zero value must be stateInit
		stateDict
	)

	switch f.stepState {
	case stateInit:
		goto readLiteral
	case stateDict:
		goto copyHistory
	}

readLiteral:
	// Read literal and/or (length, distance) according to RFC section 3.2.3.
	{
		v, err := f.huffSym(f.hl)
		if err != nil {
			f.err = err
			return
		}
		var n uint // number of bits extra
		var length int
		switch {
		case v < 256:
			f.dict.writeByte(byte(v))
			if f.dict.availWrite() == 0 {
				f.toRead = f.dict.readFlush()
				f.step = (*decompressor).huffmanBlock
				f.stepState = stateInit
				return
			}
			goto readLiteral
		case v == 256:
			f.finishBlock()
			return
		// otherwise, reference to older data
		case v < 265:
			length = v - (257 - 3)
			n = 0
		case v < 269:
			length = v*2 - (265*2 - 11)
			n = 1
		case v < 273:
			length = v*4 - (269*4 - 19)
			n = 2
		case v < 277:
			length = v*8 - (273*8 - 35)
			n = 3
		case v < 281:
			length = v*16 - (277*16 - 67)
			n = 4
		case v < 285:
			length = v*32 - (281*32 - 131)
			n = 5
		case v < maxNumLit:
			length = 258
			n = 0
		default:
			f.err = CorruptInputError(f.roffset)
			return
		}
		if n > 0 {
			for f.nb < n {
				if err = f.moreBits(); err != nil {
					f.err = err
					return
				}
			}
			length += int(f.b & uint32(1<<n-1))
			f.b >>= n
			f.nb -= n
		}

		var dist int
		if f.hd == nil {
			for f.nb < 5 {
				if err = f.moreBits(); err != nil {
					f.err = err
					return
				}
			}
			dist = int(bits.Reverse8(uint8(f.b & 0x1F << 3)))
			f.b >>= 5
			f.nb -= 5
		} else {
			if dist, err = f.huffSym(f.hd); err != nil {
				f.err = err
				return
			}
		}

		switch {
		case dist < 4:
			dist++
		case dist < maxNumDist:
			nb := uint(dist-2) >> 1
			// have 1 bit in bottom of dist, need nb more.
			extra := (dist & 1) << nb
			for f.nb < nb {
				if err = f.moreBits(); err != nil {
					f.err = err
					return
				}
			}
			extra |= int(f.b & uint32(1<<nb-1))
			f.b >>= nb
			f.nb -= nb
			dist = 1<<(nb+1) + 1 + extra
		default:
			f.err = CorruptInputError(f.roffset)
			return
		}

		// No check on length; encoding can be prescient.
		if dist > f.dict.histSize() {
			f.err = CorruptInputError(f.roffset)
			return
		}

		f.copyLen, f.copyDist = length, dist
		goto copyHistory
	}

copyHistory:
	// Perform a backwards copy according to RFC section 3.2.3.
	{
		cnt := f.dict.tryWriteCopy(f.copyDist, f.copyLen)
		if cnt == 0 {
			cnt = f.dict.writeCopy(f.copyDist, f.copyLen)
		}
		f.copyLen -= cnt

		if f.dict.availWrite() == 0 || f.copyLen > 0 {
			f.toRead = f.dict.readFlush()
			f.step = (*decompressor).huffmanBlock // We need to continue this work
			f.stepState = stateDict
			return
		}
		goto readLiteral
	}
}

// Copy a single uncompressed data block from input to output.
func (f *decompressor) dataBlock() {
	// Uncompressed.
	// Discard current half-byte.
	f.nb = 0
	f.b = 0

	// Length then ones-complement of length.
	nr, err := io.ReadFull(f.r, f.buf[0:4])
	f.roffset += int64(nr)
	if err != nil {
		f.err = noEOF(err)
		return
	}
	n := int(f.buf[0]) | int(f.buf[1])<<8
	nn := int(f.buf[2]) | int(f.buf[3])<<8
	if uint16(nn) != uint16(^n) {
		f.err = CorruptInputError(f.roffset)
		return
	}

	if n == 0 {
		f.toRead = f.dict.readFlush()
		f.finishBlock()
		return
	}

	f.copyLen = n
	f.copyData()
}

// copyData copies f.copyLen bytes from the underlying reader into f.hist.
// It pauses for reads when f.hist is full.
func (f *decompressor) copyData() {
	buf := f.dict.writeSlice()
	if len(buf) > f.copyLen {
		buf = buf[:f.copyLen]
	}

	cnt, err := io.ReadFull(f.r, buf)
	f.roffset += int64(cnt)
	f.copyLen -= cnt
	f.dict.writeMark(cnt)
	if err != nil {
		f.err = noEOF(err)
		return
	}

	if f.dict.availWrite() == 0 || f.copyLen > 0 {
		f.toRead = f.dict.readFlush()
		f.step = (*decompressor).copyData
		return
	}
	f.finishBlock()
}

func (f *decompressor) finishBlock() {
	if f.final {
		if f.dict.availRead() > 0 {
			f.toRead = f.dict.readFlush()
		}
		f.err = io.EOF
	}
	f.step = (*decompressor).nextBlock
}

// noEOF returns err, unless err == io.EOF, in which case it returns io.ErrUnexpectedEOF.
func noEOF(e error) error {
	if e == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return e
}

func (f *decompressor) moreBits() error {
	c, err := f.r.ReadByte()
	if err != nil {
		return noEOF(err)
	}
	f.roffset++
	f.b |= uint32(c) << f.nb
	f.nb += 8
	return nil
}

// Read the next Huffman-encoded symbol from f according to h.
func (f *decompressor) huffSym(h *huffmanDecoder) (int, error) {
	// Since a huffmanDecoder can be empty or be composed of a degenerate tree
	// with single element, huffSym must error on these two edge cases. In both
	// cases, the chunks slice will be 0 for the invalid sequence, leading it
	// satisfy the n == 0 check below.
	n := uint(h.min)
	// Optimization. Compiler isn't smart enough to keep f.b,f.nb in registers,
	// but is smart enough to keep local variables in registers, so use nb and b,
	// inline call to moreBits and reassign b,nb back to f on return.
	nb, b := f.nb, f.b
	for {
		for nb < n {
			c, err := f.r.ReadByte()
			if err != nil {
				f.b = b
				f.nb = nb
				return 0, noEOF(err)
			}
			f.roffset++
			b |= uint32(c) << (nb & 31)
			nb += 8
		}
		chunk := h.chunks[b&(huffmanNumChunks-1)]
		n = uint(chunk & huffmanCountMask)
		if n > huffmanChunkBits {
			chunk = h.links[chunk>>huffmanValueShift][(b>>huffmanChunkBits)&h.linkMask]
			n = uint(chunk & huffmanCountMask)
		}
		if n <= nb {
			if n == 0 {
				f.b = b
				f.nb = nb
				f.err = CorruptInputError(f.roffset)
				return 0, f.err
			}
			f.b = b >> (n & 31)
			f.nb = nb - n
			return int(chunk >> huffmanValueShift), nil
		}
	}
}

func (f *decompressor) makeReader(r io.Reader) {
	if rr, ok := r.(Reader); ok {
		f.rBuf = nil
		f.r = rr
		return
	}
	// Reuse rBuf if possible. Invariant: rBuf is always created (and owned) by decompressor.
	if f.rBuf != nil {
		f.rBuf.Reset(r)
	} else {
		// bufio.NewReader will not return r, as r does not implement flate.Reader, so it is not bufio.Reader.
		f.rBuf = bufio.NewReader(r)
	}
	f.r = f.rBuf
}

func fixedHuffmanDecoderInit() {
	fixedOnce.Do(func() {
		// These come from the RFC section 3.2.6.
		var bits [288]int
		for i := 0; i < 144; i++ {
			bits[i] = 8
		}
		for i := 144; i < 256; i++ {
			bits[i] = 9
		}
		for i := 256; i < 280; i++ {
			bits[i] = 7
		}
		for i := 280; i < 288; i++ {
			bits[i] = 8
		}
		fixedHuffmanDecoder.init(bits[:])
	})
}

func (f *decompressor) Reset(r io.Reader, dict []byte) error {
	*f = decompressor{
		rBuf:     f.rBuf,
		bits:     f.bits,
		codebits: f.codebits,
		dict:     f.dict,
		step:     (*decompressor).nextBlock,
	}
	f.makeReader(r)
	f.dict.init(maxMatchOffset, dict)
	return nil
}

// NewReader returns a new ReadCloser that can be used
// to read the uncompressed version of r.
// If r does not also implement [io.ByteReader],
// the decompressor may read more data than necessary from r.
// The reader returns [io.EOF] after the final block in the DEFLATE stream has
// been encountered. Any trailing data after the final block is ignored.
//
// The [io.ReadCloser] returned by NewReader also implements [Resetter].
func NewReader(r io.Reader) io.ReadCloser {
	fixedHuffmanDecoderInit()

	var f decompressor
	f.makeReader(r)
	f.bits = new([maxNumLit + maxNumDist]int)
	f.codebits = new([numCodes]int)
	f.step = (*decompressor).nextBlock
	f.dict.init(maxMatchOffset, nil)
	return &f
}

// NewReaderDict is like [NewReader] but initializes the reader
// with a preset dictionary. The returned reader behaves as if
// the uncompressed data stream started with the given dictionary,
// which has already been read. NewReaderDict is typically used
// to read data compressed by [NewWriterDict].
//
// The ReadCloser returned by NewReaderDict also implements [Resetter].
func NewReaderDict(r io.Reader, dict []byte) io.ReadCloser {
	fixedHuffmanDecoderInit()

	var f decompressor
	f.makeReader(r)
	f.bits = new([maxNumLit + maxNumDist]int)
	f.codebits = new([numCodes]int)
	f.step = (*decompressor).nextBlock
	f.dict.init(maxMatchOffset, dict)
	return &f
}

```

// === FILE: references/go/src/compress/flate/level1.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flate

// Level 1 uses a single small table with 5 byte hashes.
type fastEncL1 struct {
	fastGen
	table [tableSize]tableEntry
}

func (e *fastEncL1) encode(dst *tokens, src []byte) {
	const (
		inputMargin            = 12 - 1
		minNonLiteralBlockSize = 1 + 1 + inputMargin
		hashBytes              = 5
	)

	// Protect against e.cur wraparound.
	for e.cur >= bufferReset {
		if len(e.hist) == 0 {
			clear(e.table[:])
			e.cur = maxMatchOffset
			break
		}
		// Shift down everything in the table that isn't already too far away.
		minOff := e.cur + int32(len(e.hist)) - maxMatchOffset
		for i := range e.table[:] {
			v := e.table[i].offset
			if v <= minOff {
				v = 0
			} else {
				v = v - e.cur + maxMatchOffset
			}
			e.table[i].offset = v
		}
		e.cur = maxMatchOffset
	}

	s := e.addBlock(src)

	if len(src) < minNonLiteralBlockSize {
		// We do not fill the token table.
		// This will be picked up by caller.
		dst.n = uint16(len(src))
		return
	}

	// Override src
	src = e.hist

	// nextEmit is where in src the next emitLiterals should start from.
	nextEmit := s

	// sLimit is when to stop looking for offset/length copies. The inputMargin
	// lets us use a fast path for emitLiterals in the main loop, while we are
	// looking for copies.
	sLimit := int32(len(src) - inputMargin)

	cv := loadLE64(src, s)

	for {
		const skipLog = 5
		const doEvery = 2

		nextS := s
		var candidate tableEntry
		var t int32
		for {
			nextHash := hashLen(cv, tableBits, hashBytes)
			candidate = e.table[nextHash]
			nextS = s + doEvery + (s-nextEmit)>>skipLog
			if nextS > sLimit {
				goto emitRemainder
			}

			now := loadLE64(src, nextS)
			e.table[nextHash] = tableEntry{offset: s + e.cur}
			nextHash = hashLen(now, tableBits, hashBytes)
			t = candidate.offset - e.cur
			if s-t < maxMatchOffset && uint32(cv) == loadLE32(src, t) {
				e.table[nextHash] = tableEntry{offset: nextS + e.cur}
				break
			}

			// Do one right away...
			cv = now
			s = nextS
			nextS++
			candidate = e.table[nextHash]
			now >>= 8
			e.table[nextHash] = tableEntry{offset: s + e.cur}

			t = candidate.offset - e.cur
			if s-t < maxMatchOffset && uint32(cv) == loadLE32(src, t) {
				e.table[nextHash] = tableEntry{offset: nextS + e.cur}
				break
			}
			cv = now
			s = nextS
		}

		// A 4-byte match has been found. We'll later see if more than 4 bytes
		// match. But, prior to the match, src[nextEmit:s] are unmatched. Emit
		// them as literal bytes.
		for {
			// Invariant: we have a 4-byte match at s, and no need to emit any
			// literal bytes prior to s.

			// Extend the 4-byte match as long as possible.
			l := e.matchLenLong(int(s+4), int(t+4), src) + 4

			// Extend backwards
			for t > 0 && s > nextEmit && loadLE8(src, t-1) == loadLE8(src, s-1) {
				s--
				t--
				l++
			}
			if nextEmit < s {
				for _, v := range src[nextEmit:s] {
					dst.tokens[dst.n] = token(v)
					dst.litHist[v]++
					dst.n++
				}
			}

			// Save the match found. Same as 'dst.AddMatchLong(l, uint32(s-t-baseMatchOffset))'
			xOffset := uint32(s - t - baseMatchOffset)
			xLength := l
			oc := offsetCode(xOffset)
			xOffset |= oc << 16
			for xLength > 0 {
				xl := xLength
				if xl > 258 {
					if xl > 258+baseMatchLength {
						xl = 258
					} else {
						xl = 258 - baseMatchLength
					}
				}
				xLength -= xl
				xl -= baseMatchLength
				dst.extraHist[lengthCodes1[uint8(xl)]]++
				dst.offHist[oc]++
				dst.tokens[dst.n] = token(matchType | uint32(xl)<<lengthShift | xOffset)
				dst.n++
			}
			s += l
			nextEmit = s
			if nextS >= s {
				s = nextS + 1
			}
			if s >= sLimit {
				// Index first pair after match end.
				if int(s+l+8) < len(src) {
					cv := loadLE64(src, s)
					e.table[hashLen(cv, tableBits, hashBytes)] = tableEntry{offset: s + e.cur}
				}
				goto emitRemainder
			}

			// We could immediately start working at s now, but to improve
			// compression we first update the hash table at s-2 and at s. If
			// another emitCopy is not our next move, also calculate nextHash
			// at s+1. At least on GOARCH=amd64, these three hash calculations
			// are faster as one load64 call (with some shifts) instead of
			// three load32 calls.
			x := loadLE64(src, s-2)
			o := e.cur + s - 2
			prevHash := hashLen(x, tableBits, hashBytes)
			e.table[prevHash] = tableEntry{offset: o}
			x >>= 16
			currHash := hashLen(x, tableBits, hashBytes)
			candidate = e.table[currHash]
			e.table[currHash] = tableEntry{offset: o + 2}

			t = candidate.offset - e.cur
			if s-t > maxMatchOffset || uint32(x) != loadLE32(src, t) {
				cv = x >> 8
				s++
				break
			}
		}
	}

emitRemainder:
	if int(nextEmit) < len(src) {
		// If nothing was added, don't encode literals.
		if dst.n == 0 {
			return
		}
		emitLiterals(dst, src[nextEmit:])
	}
}

```

// === FILE: references/go/src/compress/flate/level2.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flate

const (
	l2TableBits = 17               // Bits used in level 2 table
	l2TableSize = 1 << l2TableBits // Size of the level 2 table
)

// Level 2 uses a similar algorithm to level 1, but with a larger table.
type fastEncL2 struct {
	fastGen
	table [l2TableSize]tableEntry
}

func (e *fastEncL2) encode(dst *tokens, src []byte) {
	const (
		inputMargin            = 12 - 1
		minNonLiteralBlockSize = 1 + 1 + inputMargin
		hashBytes              = 5
	)

	// Protect against e.cur wraparound.
	for e.cur >= bufferReset {
		if len(e.hist) == 0 {
			clear(e.table[:])
			e.cur = maxMatchOffset
			break
		}
		// Shift down everything in the table that isn't already too far away.
		minOff := e.cur + int32(len(e.hist)) - maxMatchOffset
		for i := range e.table[:] {
			v := e.table[i].offset
			if v <= minOff {
				v = 0
			} else {
				v = v - e.cur + maxMatchOffset
			}
			e.table[i].offset = v
		}
		e.cur = maxMatchOffset
	}

	s := e.addBlock(src)

	if len(src) < minNonLiteralBlockSize {
		// We do not fill the token table.
		// This will be picked up by caller.
		dst.n = uint16(len(src))
		return
	}

	// Override src
	src = e.hist

	// nextEmit is where in src the next emitLiterals should start from.
	nextEmit := s

	// sLimit is when to stop looking for offset/length copies. The inputMargin
	// lets us use a fast path for emitLiterals in the main loop, while we are
	// looking for copies.
	sLimit := int32(len(src) - inputMargin)

	cv := loadLE64(src, s)
	for {
		// When should we start skipping if we haven't found matches in a long while.
		const skipLog = 5
		const doEvery = 2

		nextS := s
		var candidate tableEntry
		for {
			nextHash := hashLen(cv, l2TableBits, hashBytes)
			s = nextS
			nextS = s + doEvery + (s-nextEmit)>>skipLog
			if nextS > sLimit {
				goto emitRemainder
			}
			candidate = e.table[nextHash]
			now := loadLE64(src, nextS)
			e.table[nextHash] = tableEntry{offset: s + e.cur}
			nextHash = hashLen(now, l2TableBits, hashBytes)

			offset := s - (candidate.offset - e.cur)
			if offset < maxMatchOffset && uint32(cv) == loadLE32(src, candidate.offset-e.cur) {
				e.table[nextHash] = tableEntry{offset: nextS + e.cur}
				break
			}

			// Do one right away...
			cv = now
			s = nextS
			nextS++
			candidate = e.table[nextHash]
			now >>= 8
			e.table[nextHash] = tableEntry{offset: s + e.cur}

			offset = s - (candidate.offset - e.cur)
			if offset < maxMatchOffset && uint32(cv) == loadLE32(src, candidate.offset-e.cur) {
				break
			}
			cv = now
		}

		// A 4-byte match has been found. We'll later see if more than 4 bytes match.
		for {
			// Extend the 4-byte match as long as possible.
			t := candidate.offset - e.cur
			l := e.matchLenLong(int(s+4), int(t+4), src) + 4

			// Extend backwards
			for t > 0 && s > nextEmit && src[t-1] == src[s-1] {
				s--
				t--
				l++
			}
			if nextEmit < s {
				for _, v := range src[nextEmit:s] {
					dst.tokens[dst.n] = token(v)
					dst.litHist[v]++
					dst.n++
				}
			}

			dst.AddMatchLong(l, uint32(s-t-baseMatchOffset))
			s += l
			nextEmit = s
			if nextS >= s {
				s = nextS + 1
			}

			if s >= sLimit {
				// Index first pair after match end.
				if int(s+l+8) < len(src) {
					cv := loadLE64(src, s)
					e.table[hashLen(cv, l2TableBits, hashBytes)] = tableEntry{offset: s + e.cur}
				}
				goto emitRemainder
			}

			// Store every second hash in-between, but offset by 1.
			for i := s - l + 2; i < s-5; i += 7 {
				x := loadLE64(src, i)
				nextHash := hashLen(x, l2TableBits, hashBytes)
				e.table[nextHash] = tableEntry{offset: e.cur + i}
				// Skip one
				x >>= 16
				nextHash = hashLen(x, l2TableBits, hashBytes)
				e.table[nextHash] = tableEntry{offset: e.cur + i + 2}
				// Skip one
				x >>= 16
				nextHash = hashLen(x, l2TableBits, hashBytes)
				e.table[nextHash] = tableEntry{offset: e.cur + i + 4}
			}

			// We could immediately start working at s now, but to improve
			// compression we first update the hash table at s-2 to s. If
			// another emitCopy is not our next move, also calculate nextHash
			// at s+1.
			x := loadLE64(src, s-2)
			o := e.cur + s - 2
			prevHash := hashLen(x, l2TableBits, hashBytes)
			prevHash2 := hashLen(x>>8, l2TableBits, hashBytes)
			e.table[prevHash] = tableEntry{offset: o}
			e.table[prevHash2] = tableEntry{offset: o + 1}
			currHash := hashLen(x>>16, l2TableBits, hashBytes)
			candidate = e.table[currHash]
			e.table[currHash] = tableEntry{offset: o + 2}

			offset := s - (candidate.offset - e.cur)
			if offset > maxMatchOffset || uint32(x>>16) != loadLE32(src, candidate.offset-e.cur) {
				cv = x >> 24
				s++
				break
			}
		}
	}

emitRemainder:
	if int(nextEmit) < len(src) {
		// If nothing was added, don't encode literals.
		if dst.n == 0 {
			return
		}

		emitLiterals(dst, src[nextEmit:])
	}
}

```

// === FILE: references/go/src/compress/flate/level3.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flate

const (
	l3TableBits = 16               // Bits used in level 3 table
	l3TableSize = 1 << l3TableBits // Size of the level 3 table
)

// Level 3 uses a similar algorithm to level 2, with a smaller table,
// but will check up two candidates for each iteration with more
// entries added to the table.
type fastEncL3 struct {
	fastGen
	table [l3TableSize]tableEntryPrev
}

func (e *fastEncL3) encode(dst *tokens, src []byte) {
	const (
		inputMargin            = 12 - 1
		minNonLiteralBlockSize = 1 + 1 + inputMargin
		hashBytes              = 5
	)

	// Protect against e.cur wraparound.
	for e.cur >= bufferReset {
		if len(e.hist) == 0 {
			clear(e.table[:])
			e.cur = maxMatchOffset
			break
		}
		// Shift down everything in the table that isn't already too far away.
		minOff := e.cur + int32(len(e.hist)) - maxMatchOffset
		for i := range e.table[:] {
			v := e.table[i]
			if v.cur.offset <= minOff {
				v.cur.offset = 0
			} else {
				v.cur.offset = v.cur.offset - e.cur + maxMatchOffset
			}
			if v.prev.offset <= minOff {
				v.prev.offset = 0
			} else {
				v.prev.offset = v.prev.offset - e.cur + maxMatchOffset
			}
			e.table[i] = v
		}
		e.cur = maxMatchOffset
	}

	s := e.addBlock(src)

	// Skip if too small.
	if len(src) < minNonLiteralBlockSize {
		// We do not fill the token table.
		// This will be picked up by caller.
		dst.n = uint16(len(src))
		return
	}

	// Override src
	src = e.hist
	nextEmit := s

	// sLimit is when to stop looking for offset/length copies. The inputMargin
	// lets us use a fast path for emitLiterals in the main loop, while we are
	// looking for copies.
	sLimit := int32(len(src) - inputMargin)

	// nextEmit is where in src the next emitLiterals should start from.
	cv := loadLE64(src, s)
	for {
		const skipLog = 7
		nextS := s
		var candidate tableEntry
		for {
			nextHash := hashLen(cv, l3TableBits, hashBytes)
			s = nextS
			nextS = s + 1 + (s-nextEmit)>>skipLog
			if nextS > sLimit {
				goto emitRemainder
			}
			candidates := e.table[nextHash]
			now := loadLE64(src, nextS)

			// Safe offset distance until s + 4...
			minOffset := e.cur + s - (maxMatchOffset - 4)
			e.table[nextHash] = tableEntryPrev{prev: candidates.cur, cur: tableEntry{offset: s + e.cur}}

			// Check both candidates
			candidate = candidates.cur
			if candidate.offset < minOffset {
				cv = now
				// Previous will also be invalid, we have nothing.
				continue
			}

			if uint32(cv) == loadLE32(src, candidate.offset-e.cur) {
				if candidates.prev.offset < minOffset || uint32(cv) != loadLE32(src, candidates.prev.offset-e.cur) {
					break
				}
				// Both match and are valid, pick longest.
				offset := s - (candidate.offset - e.cur)
				o2 := s - (candidates.prev.offset - e.cur)
				l1, l2 := matchLen(src[s+4:], src[s-offset+4:]), matchLen(src[s+4:], src[s-o2+4:])
				if l2 > l1 {
					candidate = candidates.prev
				}
				break
			} else {
				// We only check if value mismatches.
				// Offset will always be invalid in other cases.
				candidate = candidates.prev
				if candidate.offset > minOffset && uint32(cv) == loadLE32(src, candidate.offset-e.cur) {
					break
				}
			}
			cv = now
		}

		for {
			// Extend the 4-byte match as long as possible.
			//
			t := candidate.offset - e.cur
			l := e.matchLenLong(int(s+4), int(t+4), src) + 4

			// Extend backwards
			for t > 0 && s > nextEmit && src[t-1] == src[s-1] {
				s--
				t--
				l++
			}
			// Emit literals.
			if nextEmit < s {
				for _, v := range src[nextEmit:s] {
					dst.tokens[dst.n] = token(v)
					dst.litHist[v]++
					dst.n++
				}
			}

			// Emit match.
			dst.AddMatchLong(l, uint32(s-t-baseMatchOffset))
			s += l
			nextEmit = s
			if nextS >= s {
				s = nextS + 1
			}

			if s >= sLimit {
				t += l
				// Index first pair after match end.
				if int(t+8) < len(src) && t > 0 {
					cv = loadLE64(src, t)
					nextHash := hashLen(cv, l3TableBits, hashBytes)
					e.table[nextHash] = tableEntryPrev{
						prev: e.table[nextHash].cur,
						cur:  tableEntry{offset: e.cur + t},
					}
				}
				goto emitRemainder
			}

			// Store every 5th hash in-between.
			for i := s - l + 2; i < s-5; i += 6 {
				nextHash := hashLen(loadLE64(src, i), l3TableBits, hashBytes)
				e.table[nextHash] = tableEntryPrev{
					prev: e.table[nextHash].cur,
					cur:  tableEntry{offset: e.cur + i}}
			}
			// We could immediately start working at s now, but to improve
			// compression we first update the hash table at s-2 to s.
			x := loadLE64(src, s-2)
			prevHash := hashLen(x, l3TableBits, hashBytes)

			e.table[prevHash] = tableEntryPrev{
				prev: e.table[prevHash].cur,
				cur:  tableEntry{offset: e.cur + s - 2},
			}
			x >>= 8
			prevHash = hashLen(x, l3TableBits, hashBytes)

			e.table[prevHash] = tableEntryPrev{
				prev: e.table[prevHash].cur,
				cur:  tableEntry{offset: e.cur + s - 1},
			}
			x >>= 8
			currHash := hashLen(x, l3TableBits, hashBytes)
			candidates := e.table[currHash]
			cv = x
			e.table[currHash] = tableEntryPrev{
				prev: candidates.cur,
				cur:  tableEntry{offset: s + e.cur},
			}

			// Check both candidates
			candidate = candidates.cur
			minOffset := e.cur + s - (maxMatchOffset - 4)

			if candidate.offset > minOffset {
				if uint32(cv) == loadLE32(src, candidate.offset-e.cur) {
					// Found a match...
					continue
				}
				candidate = candidates.prev
				if candidate.offset > minOffset && uint32(cv) == loadLE32(src, candidate.offset-e.cur) {
					// Match at prev...
					continue
				}
			}
			cv = x >> 8
			s++
			break
		}
	}

emitRemainder:
	if int(nextEmit) < len(src) {
		// If nothing was added, don't encode literals.
		if dst.n == 0 {
			return
		}

		emitLiterals(dst, src[nextEmit:])
	}
}

```

// === FILE: references/go/src/compress/flate/level4.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flate

// Level 4 uses two tables, one for short (4 bytes) and one for long (7 bytes) matches.
type fastEncL4 struct {
	fastGen
	table  [tableSize]tableEntry
	bTable [tableSize]tableEntry
}

func (e *fastEncL4) encode(dst *tokens, src []byte) {
	const (
		inputMargin            = 12 - 1
		minNonLiteralBlockSize = 1 + 1 + inputMargin
		hashShortBytes         = 4
	)
	// Protect against e.cur wraparound.
	for e.cur >= bufferReset {
		if len(e.hist) == 0 {
			clear(e.table[:])
			clear(e.bTable[:])
			e.cur = maxMatchOffset
			break
		}
		// Shift down everything in the table that isn't already too far away.
		minOff := e.cur + int32(len(e.hist)) - maxMatchOffset
		for i := range e.table[:] {
			v := e.table[i].offset
			if v <= minOff {
				v = 0
			} else {
				v = v - e.cur + maxMatchOffset
			}
			e.table[i].offset = v
		}
		for i := range e.bTable[:] {
			v := e.bTable[i].offset
			if v <= minOff {
				v = 0
			} else {
				v = v - e.cur + maxMatchOffset
			}
			e.bTable[i].offset = v
		}
		e.cur = maxMatchOffset
	}

	s := e.addBlock(src)

	// This check isn't in the Snappy implementation, but there, the caller
	// instead of the callee handles this case.
	if len(src) < minNonLiteralBlockSize {
		// We do not fill the token table.
		// This will be picked up by caller.
		dst.n = uint16(len(src))
		return
	}

	// Override src
	src = e.hist
	nextEmit := s

	// sLimit is when to stop looking for offset/length copies. The inputMargin
	// lets us use a fast path for emitLiterals in the main loop, while we are
	// looking for copies.
	sLimit := int32(len(src) - inputMargin)

	// nextEmit is where in src the next emitLiterals should start from.
	cv := loadLE64(src, s)
	for {
		const skipLog = 6
		const doEvery = 1

		nextS := s
		var t int32
		for {
			nextHashS := hashLen(cv, tableBits, hashShortBytes)
			nextHashL := hashLen(cv, tableBits, hashLongBytes)

			s = nextS
			nextS = s + doEvery + (s-nextEmit)>>skipLog
			if nextS > sLimit {
				goto emitRemainder
			}
			// Fetch a short+long candidate
			sCandidate := e.table[nextHashS]
			lCandidate := e.bTable[nextHashL]
			next := loadLE64(src, nextS)
			entry := tableEntry{offset: s + e.cur}
			e.table[nextHashS] = entry
			e.bTable[nextHashL] = entry

			t = lCandidate.offset - e.cur
			if s-t < maxMatchOffset && uint32(cv) == loadLE32(src, t) {
				// We got a long match. Use that.
				break
			}

			t = sCandidate.offset - e.cur
			if s-t < maxMatchOffset && uint32(cv) == loadLE32(src, t) {
				// Found a 4 match...
				lCandidate = e.bTable[hashLen(next, tableBits, hashLongBytes)]

				// If the next long is a candidate, check if we should use that instead...
				lOff := lCandidate.offset - e.cur
				if nextS-lOff < maxMatchOffset && loadLE32(src, lOff) == uint32(next) {
					l1, l2 := matchLen(src[s+4:], src[t+4:]), matchLen(src[nextS+4:], src[nextS-lOff+4:])
					if l2 > l1 {
						s = nextS
						t = lCandidate.offset - e.cur
					}
				}
				break
			}
			cv = next
		}

		// A 4-byte match has been found. We'll later see if more than 4 bytes
		// match. But, prior to the match, src[nextEmit:s] are unmatched. Emit
		// them as literal bytes.

		// Extend the 4-byte match as long as possible.
		l := e.matchLenLong(int(s+4), int(t+4), src) + 4

		// Extend backwards
		for t > 0 && s > nextEmit && src[t-1] == src[s-1] {
			s--
			t--
			l++
		}
		if nextEmit < s {
			for _, v := range src[nextEmit:s] {
				dst.tokens[dst.n] = token(v)
				dst.litHist[v]++
				dst.n++
			}
		}

		dst.AddMatchLong(l, uint32(s-t-baseMatchOffset))
		s += l
		nextEmit = s
		if nextS >= s {
			s = nextS + 1
		}

		if s >= sLimit {
			// Index first pair after match end.
			if int(s+8) < len(src) {
				cv := loadLE64(src, s)
				e.table[hashLen(cv, tableBits, hashShortBytes)] = tableEntry{offset: s + e.cur}
				e.bTable[hashLen(cv, tableBits, hashLongBytes)] = tableEntry{offset: s + e.cur}
			}
			goto emitRemainder
		}

		// Store every 3rd hash in-between
		i := nextS
		if i < s-1 {
			cv := loadLE64(src, i)
			t := tableEntry{offset: i + e.cur}
			t2 := tableEntry{offset: t.offset + 1}
			e.bTable[hashLen(cv, tableBits, hashLongBytes)] = t
			e.bTable[hashLen(cv>>8, tableBits, hashLongBytes)] = t2
			e.table[hashLen(cv>>8, tableBits, hashShortBytes)] = t2

			i += 3
			for ; i < s-1; i += 3 {
				cv := loadLE64(src, i)
				t := tableEntry{offset: i + e.cur}
				t2 := tableEntry{offset: t.offset + 1}
				e.bTable[hashLen(cv, tableBits, hashLongBytes)] = t
				e.bTable[hashLen(cv>>8, tableBits, hashLongBytes)] = t2
				e.table[hashLen(cv>>8, tableBits, hashShortBytes)] = t2
			}
		}

		// We could immediately start working at s now, but to improve
		// compression we first update the hash table at s-1 and at s.
		x := loadLE64(src, s-1)
		o := e.cur + s - 1
		prevHashS := hashLen(x, tableBits, hashShortBytes)
		prevHashL := hashLen(x, tableBits, hashLongBytes)
		e.table[prevHashS] = tableEntry{offset: o}
		e.bTable[prevHashL] = tableEntry{offset: o}
		cv = x >> 8
	}

emitRemainder:
	if int(nextEmit) < len(src) {
		// If nothing was added, don't encode literals.
		if dst.n == 0 {
			return
		}

		emitLiterals(dst, src[nextEmit:])
	}
}

```

// === FILE: references/go/src/compress/flate/level5.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flate

// Level 5 is similar to level 4, but for long matches two candidates are tested.
// Once a match is found, when it stops it will attempt to find a match that extends further.
type fastEncL5 struct {
	fastGen
	table  [tableSize]tableEntry
	bTable [tableSize]tableEntryPrev
}

func (e *fastEncL5) encode(dst *tokens, src []byte) {
	const (
		inputMargin            = 12 - 1
		minNonLiteralBlockSize = 1 + 1 + inputMargin
		hashShortBytes         = 4
	)

	// Protect against e.cur wraparound.
	for e.cur >= bufferReset {
		if len(e.hist) == 0 {
			clear(e.table[:])
			clear(e.bTable[:])
			e.cur = maxMatchOffset
			break
		}
		// Shift down everything in the table that isn't already too far away.
		minOff := e.cur + int32(len(e.hist)) - maxMatchOffset
		for i := range e.table[:] {
			v := e.table[i].offset
			if v <= minOff {
				v = 0
			} else {
				v = v - e.cur + maxMatchOffset
			}
			e.table[i].offset = v
		}
		for i := range e.bTable[:] {
			v := e.bTable[i]
			if v.cur.offset <= minOff {
				v.cur.offset = 0
				v.prev.offset = 0
			} else {
				v.cur.offset = v.cur.offset - e.cur + maxMatchOffset
				if v.prev.offset <= minOff {
					v.prev.offset = 0
				} else {
					v.prev.offset = v.prev.offset - e.cur + maxMatchOffset
				}
			}
			e.bTable[i] = v
		}
		e.cur = maxMatchOffset
	}

	s := e.addBlock(src)

	// This check isn't in the Snappy implementation, but there, the caller
	// instead of the callee handles this case.
	if len(src) < minNonLiteralBlockSize {
		// We do not fill the token table.
		// This will be picked up by caller.
		dst.n = uint16(len(src))
		return
	}

	// Override src
	src = e.hist

	// nextEmit is where in src the next emitLiterals should start from.
	nextEmit := s

	// sLimit is when to stop looking for offset/length copies. The inputMargin
	// lets us use a fast path for emitLiterals in the main loop, while we are
	// looking for copies.
	sLimit := int32(len(src) - inputMargin)

	cv := loadLE64(src, s)
	for {
		const skipLog = 6
		const doEvery = 1

		nextS := s
		var l int32
		var t int32
		for {
			nextHashS := hashLen(cv, tableBits, hashShortBytes)
			nextHashL := hashLen(cv, tableBits, hashLongBytes)

			s = nextS
			nextS = s + doEvery + (s-nextEmit)>>skipLog
			if nextS > sLimit {
				goto emitRemainder
			}
			// Fetch a short+long candidate
			sCandidate := e.table[nextHashS]
			lCandidate := e.bTable[nextHashL]
			next := loadLE64(src, nextS)
			entry := tableEntry{offset: s + e.cur}
			e.table[nextHashS] = entry
			eLong := &e.bTable[nextHashL]
			eLong.cur, eLong.prev = entry, eLong.cur

			nextHashS = hashLen(next, tableBits, hashShortBytes)
			nextHashL = hashLen(next, tableBits, hashLongBytes)

			t = lCandidate.cur.offset - e.cur
			if s-t < maxMatchOffset {
				if uint32(cv) == loadLE32(src, t) {
					// Store the next match
					e.table[nextHashS] = tableEntry{offset: nextS + e.cur}
					eLong := &e.bTable[nextHashL]
					eLong.cur, eLong.prev = tableEntry{offset: nextS + e.cur}, eLong.cur

					t2 := lCandidate.prev.offset - e.cur
					if s-t2 < maxMatchOffset && uint32(cv) == loadLE32(src, t2) {
						l = e.matchLenLimited(int(s+4), int(t+4), src) + 4
						ml1 := e.matchLenLimited(int(s+4), int(t2+4), src) + 4
						if ml1 > l {
							t = t2
							l = ml1
							break
						}
					}
					break
				}
				t = lCandidate.prev.offset - e.cur
				if s-t < maxMatchOffset && uint32(cv) == loadLE32(src, t) {
					// Store the next match
					e.table[nextHashS] = tableEntry{offset: nextS + e.cur}
					eLong := &e.bTable[nextHashL]
					eLong.cur, eLong.prev = tableEntry{offset: nextS + e.cur}, eLong.cur
					break
				}
			}

			t = sCandidate.offset - e.cur
			if s-t < maxMatchOffset && uint32(cv) == loadLE32(src, t) {
				// Found a 4 match...
				l = e.matchLenLimited(int(s+4), int(t+4), src) + 4
				lCandidate = e.bTable[nextHashL]
				// Store the next match

				e.table[nextHashS] = tableEntry{offset: nextS + e.cur}
				eLong := &e.bTable[nextHashL]
				eLong.cur, eLong.prev = tableEntry{offset: nextS + e.cur}, eLong.cur

				// If the next long is a candidate, use that...
				t2 := lCandidate.cur.offset - e.cur
				if nextS-t2 < maxMatchOffset {
					if loadLE32(src, t2) == uint32(next) {
						ml := e.matchLenLimited(int(nextS+4), int(t2+4), src) + 4
						if ml > l {
							t = t2
							s = nextS
							l = ml
							break
						}
					}
					// If the previous long is a candidate, use that...
					t2 = lCandidate.prev.offset - e.cur
					if nextS-t2 < maxMatchOffset && loadLE32(src, t2) == uint32(next) {
						ml := e.matchLenLimited(int(nextS+4), int(t2+4), src) + 4
						if ml > l {
							t = t2
							s = nextS
							l = ml
							break
						}
					}
				}
				break
			}
			cv = next
		}

		if l == 0 {
			// Extend the 4-byte match as long as possible.
			l = e.matchLenLong(int(s+4), int(t+4), src) + 4
		} else if l == maxMatchLength {
			l += e.matchLenLong(int(s+l), int(t+l), src)
		}

		// Try to locate a better match by checking the end of best match...
		if sAt := s + l; l < 30 && sAt < sLimit {
			// Allow some bytes at the beginning to mismatch.
			// Sweet spot is 2/3 bytes depending on input.
			// 3 is only a little better when it is but sometimes a lot worse.
			// The skipped bytes are tested in Extend backwards,
			// and still picked up as part of the match if they do.
			const skipBeginning = 2
			eLong := e.bTable[hashLen(loadLE64(src, sAt), tableBits, hashLongBytes)].cur.offset
			t2 := eLong - e.cur - l + skipBeginning
			s2 := s + skipBeginning
			off := s2 - t2
			if t2 >= 0 && off < maxMatchOffset && off > 0 {
				if l2 := e.matchLenLong(int(s2), int(t2), src); l2 > l {
					t = t2
					l = l2
					s = s2
				}
			}
		}

		// Extend backwards
		for t > 0 && s > nextEmit && src[t-1] == src[s-1] {
			s--
			t--
			l++
		}
		if nextEmit < s {
			for _, v := range src[nextEmit:s] {
				dst.tokens[dst.n] = token(v)
				dst.litHist[v]++
				dst.n++
			}
		}

		dst.AddMatchLong(l, uint32(s-t-baseMatchOffset))
		s += l
		nextEmit = s
		if nextS >= s {
			s = nextS + 1
		}

		if s >= sLimit {
			goto emitRemainder
		}

		// Store every 3rd hash in-between.
		const hashEvery = 3
		i := s - l + 1
		if i < s-1 {
			cv := loadLE64(src, i)
			t := tableEntry{offset: i + e.cur}
			e.table[hashLen(cv, tableBits, hashShortBytes)] = t
			eLong := &e.bTable[hashLen(cv, tableBits, hashLongBytes)]
			eLong.cur, eLong.prev = t, eLong.cur

			// Do an long at i+1
			cv >>= 8
			t = tableEntry{offset: t.offset + 1}
			eLong = &e.bTable[hashLen(cv, tableBits, hashLongBytes)]
			eLong.cur, eLong.prev = t, eLong.cur

			// We only have enough bits for a short entry at i+2
			cv >>= 8
			t = tableEntry{offset: t.offset + 1}
			e.table[hashLen(cv, tableBits, hashShortBytes)] = t

			// Skip one - otherwise we risk hitting 's'
			i += 4
			for ; i < s-1; i += hashEvery {
				cv := loadLE64(src, i)
				t := tableEntry{offset: i + e.cur}
				t2 := tableEntry{offset: t.offset + 1}
				eLong := &e.bTable[hashLen(cv, tableBits, hashLongBytes)]
				eLong.cur, eLong.prev = t, eLong.cur
				e.table[hashLen(cv>>8, tableBits, hashShortBytes)] = t2
			}
		}

		// We could immediately start working at s now, but to improve
		// compression we first update the hash table at s-1 and at s.
		x := loadLE64(src, s-1)
		o := e.cur + s - 1
		prevHashS := hashLen(x, tableBits, hashShortBytes)
		prevHashL := hashLen(x, tableBits, hashLongBytes)
		e.table[prevHashS] = tableEntry{offset: o}
		eLong := &e.bTable[prevHashL]
		eLong.cur, eLong.prev = tableEntry{offset: o}, eLong.cur
		cv = x >> 8
	}

emitRemainder:
	if int(nextEmit) < len(src) {
		// If nothing was added, don't encode literals.
		if dst.n == 0 {
			return
		}

		emitLiterals(dst, src[nextEmit:])
	}
}

```

// === FILE: references/go/src/compress/flate/level6.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flate

// Level 6 extends level 5, but does "repeat offset" check,
// as well as adding more hash entries to the tables.
type fastEncL6 struct {
	fastGen
	table  [tableSize]tableEntry
	bTable [tableSize]tableEntryPrev
}

func (e *fastEncL6) encode(dst *tokens, src []byte) {
	const (
		inputMargin            = 12 - 1
		minNonLiteralBlockSize = 1 + 1 + inputMargin
		hashShortBytes         = 4
	)

	// Protect against e.cur wraparound.
	for e.cur >= bufferReset {
		if len(e.hist) == 0 {
			clear(e.table[:])
			clear(e.bTable[:])
			e.cur = maxMatchOffset
			break
		}
		// Shift down everything in the table that isn't already too far away.
		minOff := e.cur + int32(len(e.hist)) - maxMatchOffset
		for i := range e.table[:] {
			v := e.table[i].offset
			if v <= minOff {
				v = 0
			} else {
				v = v - e.cur + maxMatchOffset
			}
			e.table[i].offset = v
		}
		for i := range e.bTable[:] {
			v := e.bTable[i]
			if v.cur.offset <= minOff {
				v.cur.offset = 0
				v.prev.offset = 0
			} else {
				v.cur.offset = v.cur.offset - e.cur + maxMatchOffset
				if v.prev.offset <= minOff {
					v.prev.offset = 0
				} else {
					v.prev.offset = v.prev.offset - e.cur + maxMatchOffset
				}
			}
			e.bTable[i] = v
		}
		e.cur = maxMatchOffset
	}

	s := e.addBlock(src)

	if len(src) < minNonLiteralBlockSize {
		// We do not fill the token table.
		// This will be picked up by caller.
		dst.n = uint16(len(src))
		return
	}

	// Override src
	src = e.hist

	// nextEmit is where in src the next emitLiterals should start from.
	nextEmit := s

	// sLimit is when to stop looking for offset/length copies. The inputMargin
	// lets us use a fast path for emitLiterals in the main loop, while we are
	// looking for copies.
	sLimit := int32(len(src) - inputMargin)

	cv := loadLE64(src, s)
	// Repeat MUST be > 1 and within range
	repeat := int32(1)
	for {
		const skipLog = 7
		const doEvery = 1

		nextS := s
		var l int32
		var t int32
		for {
			nextHashS := hashLen(cv, tableBits, hashShortBytes)
			nextHashL := hashLen(cv, tableBits, hashLongBytes)
			s = nextS
			nextS = s + doEvery + (s-nextEmit)>>skipLog
			if nextS > sLimit {
				goto emitRemainder
			}
			// Fetch a short+long candidate
			sCandidate := e.table[nextHashS]
			lCandidate := e.bTable[nextHashL]
			next := loadLE64(src, nextS)
			entry := tableEntry{offset: s + e.cur}
			e.table[nextHashS] = entry
			eLong := &e.bTable[nextHashL]
			eLong.cur, eLong.prev = entry, eLong.cur

			// Calculate hashes of 'next'
			nextHashS = hashLen(next, tableBits, hashShortBytes)
			nextHashL = hashLen(next, tableBits, hashLongBytes)

			t = lCandidate.cur.offset - e.cur
			if s-t < maxMatchOffset {
				if uint32(cv) == loadLE32(src, t) {
					// Long candidate matches at least 4 bytes.

					// Store the next match
					e.table[nextHashS] = tableEntry{offset: nextS + e.cur}
					eLong := &e.bTable[nextHashL]
					eLong.cur, eLong.prev = tableEntry{offset: nextS + e.cur}, eLong.cur

					// Check the previous long candidate as well.
					t2 := lCandidate.prev.offset - e.cur
					if s-t2 < maxMatchOffset && uint32(cv) == loadLE32(src, t2) {
						l = e.matchLenLimited(int(s+4), int(t+4), src) + 4
						ml1 := e.matchLenLimited(int(s+4), int(t2+4), src) + 4
						if ml1 > l {
							t = t2
							l = ml1
							break
						}
					}
					break
				}
				// Current value did not match, but check if previous long value does.
				t = lCandidate.prev.offset - e.cur
				if s-t < maxMatchOffset && uint32(cv) == loadLE32(src, t) {
					// Store the next match
					e.table[nextHashS] = tableEntry{offset: nextS + e.cur}
					eLong := &e.bTable[nextHashL]
					eLong.cur, eLong.prev = tableEntry{offset: nextS + e.cur}, eLong.cur
					break
				}
			}

			t = sCandidate.offset - e.cur
			if s-t < maxMatchOffset && uint32(cv) == loadLE32(src, t) {
				// Found a 4 match...
				l = e.matchLenLimited(int(s+4), int(t+4), src) + 4

				// Look up next long candidate (at nextS)
				lCandidate = e.bTable[nextHashL]

				// Store the next match
				e.table[nextHashS] = tableEntry{offset: nextS + e.cur}
				eLong := &e.bTable[nextHashL]
				eLong.cur, eLong.prev = tableEntry{offset: nextS + e.cur}, eLong.cur

				// Check repeat at s + repOff
				const repOff = 1
				t2 := s - repeat + repOff
				if loadLE32(src, t2) == uint32(cv>>(8*repOff)) {
					ml := e.matchLenLimited(int(s+4+repOff), int(t2+4), src) + 4
					if ml > l {
						t = t2
						l = ml
						s += repOff
						// Not worth checking more.
						break
					}
				}

				// If the next long is a candidate, use that...
				t2 = lCandidate.cur.offset - e.cur
				if nextS-t2 < maxMatchOffset {
					if loadLE32(src, t2) == uint32(next) {
						ml := e.matchLenLimited(int(nextS+4), int(t2+4), src) + 4
						if ml > l {
							t = t2
							s = nextS
							l = ml
							// This is ok, but check previous as well.
						}
					}
					// If the previous long is a candidate, use that...
					t2 = lCandidate.prev.offset - e.cur
					if nextS-t2 < maxMatchOffset && loadLE32(src, t2) == uint32(next) {
						ml := e.matchLenLimited(int(nextS+4), int(t2+4), src) + 4
						if ml > l {
							t = t2
							s = nextS
							l = ml
							break
						}
					}
				}
				break
			}
			cv = next
		}

		// Extend the 4-byte match as long as possible.
		if l == 0 {
			l = e.matchLenLong(int(s+4), int(t+4), src) + 4
		} else if l == maxMatchLength {
			l += e.matchLenLong(int(s+l), int(t+l), src)
		}

		// Try to locate a better match by checking the end-of-match...
		if sAt := s + l; sAt < sLimit {
			// Allow some bytes at the beginning to mismatch.
			// Sweet spot is 2/3 bytes depending on input.
			// 3 is only a little better when it is but sometimes a lot worse.
			// The skipped bytes are tested in extend backwards,
			// and still picked up as part of the match if they do.
			const skipBeginning = 2
			eLong := &e.bTable[hashLen(loadLE64(src, sAt), tableBits, hashLongBytes)]
			// Test current
			t2 := eLong.cur.offset - e.cur - l + skipBeginning
			s2 := s + skipBeginning
			off := s2 - t2
			if off < maxMatchOffset {
				if off > 0 && t2 >= 0 {
					if l2 := e.matchLenLong(int(s2), int(t2), src); l2 > l {
						t = t2
						l = l2
						s = s2
					}
				}
				// Test previous entry:
				t2 = eLong.prev.offset - e.cur - l + skipBeginning
				off := s2 - t2
				if off > 0 && off < maxMatchOffset && t2 >= 0 {
					if l2 := e.matchLenLong(int(s2), int(t2), src); l2 > l {
						t = t2
						l = l2
						s = s2
					}
				}
			}
		}

		// Extend backwards
		for t > 0 && s > nextEmit && src[t-1] == src[s-1] {
			s--
			t--
			l++
		}
		if nextEmit < s {
			for _, v := range src[nextEmit:s] {
				dst.tokens[dst.n] = token(v)
				dst.litHist[v]++
				dst.n++
			}
		}

		dst.AddMatchLong(l, uint32(s-t-baseMatchOffset))
		repeat = s - t
		s += l
		nextEmit = s
		if nextS >= s {
			s = nextS + 1
		}

		if s >= sLimit {
			// Index after match end.
			for i := nextS + 1; i < int32(len(src))-8; i += 2 {
				cv := loadLE64(src, i)
				e.table[hashLen(cv, tableBits, hashShortBytes)] = tableEntry{offset: i + e.cur}
				eLong := &e.bTable[hashLen(cv, tableBits, hashLongBytes)]
				eLong.cur, eLong.prev = tableEntry{offset: i + e.cur}, eLong.cur
			}
			goto emitRemainder
		}

		// Store every long hash in-between and every second short.
		for i := nextS + 1; i < s-1; i += 2 {
			cv := loadLE64(src, i)
			t := tableEntry{offset: i + e.cur}
			t2 := tableEntry{offset: t.offset + 1}
			eLong := &e.bTable[hashLen(cv, tableBits, hashLongBytes)]
			eLong2 := &e.bTable[hashLen(cv>>8, tableBits, hashLongBytes)]
			e.table[hashLen(cv, tableBits, hashShortBytes)] = t
			eLong.cur, eLong.prev = t, eLong.cur
			eLong2.cur, eLong2.prev = t2, eLong2.cur
		}
		cv = loadLE64(src, s)
	}

emitRemainder:
	if int(nextEmit) < len(src) {
		// If nothing was added, don't encode literals.
		if dst.n == 0 {
			return
		}

		emitLiterals(dst, src[nextEmit:])
	}
}

```

// === FILE: references/go/src/compress/flate/load_store.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flate

// This file contains functions for loading and storing integers in little endian format.
// These can be replaced with unsafe versions if deemed necessary.

type indexer interface {
	int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64
}

// loadLE8 will load from b at index i.
func loadLE8[I indexer](b []byte, i I) byte {
	return b[i]
}

// loadLE32 will load from b at index i.
func loadLE32[I indexer](b []byte, i I) uint32 {
	b = b[i : i+4]
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

// loadLE64 will load from b at index i.
func loadLE64[I indexer](b []byte, i I) uint64 {
	b = b[i : i+8]
	return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
}

// storeLE64 will store v at start of b.
func storeLE64(b []byte, v uint64) {
	_ = b[7] // early bounds check to guarantee safety of writes below
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
	b[4] = byte(v >> 32)
	b[5] = byte(v >> 40)
	b[6] = byte(v >> 48)
	b[7] = byte(v >> 56)
}

```

// === FILE: references/go/src/compress/flate/regmask_amd64.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flate

// shiftMask is a no-op shift mask for x86-64.
// Using it lets the compiler omit the check for shift size >= 64.
const reg8SizeMask64 = 63

```

// === FILE: references/go/src/compress/flate/regmask_other.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !amd64

package flate

// shiftMask is a no-op shift mask for non-x86-64.
// The compiler will optimize it away.
const reg8SizeMask64 = 0xff

```

// === FILE: references/go/src/compress/flate/token.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flate

import (
	"math"
)

const (
	// Token is a compound value:
	// bits 0-16  xoffset = offset - MIN_OFFSET_SIZE, or literal - 16 bits
	// bits 16-22 offset code - 5 bits
	// bits 22-30 xlength = length - MIN_MATCH_LENGTH - 8 bits
	// bits 30-32 type, 0 = literal  1=EOF  2=Match   3=Unused - 2 bits
	lengthShift         = 22
	offsetMask          = 1<<lengthShift - 1
	typeMask            = 3 << 30
	matchType           = 1 << 30
	matchOffsetOnlyMask = 0xffff
)

// The length code for length X (MIN_MATCH_LENGTH <= X <= MAX_MATCH_LENGTH)
// is lengthCodes[length - MIN_MATCH_LENGTH]
var lengthCodes = [256]uint8{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 8,
	9, 9, 10, 10, 11, 11, 12, 12, 12, 12,
	13, 13, 13, 13, 14, 14, 14, 14, 15, 15,
	15, 15, 16, 16, 16, 16, 16, 16, 16, 16,
	17, 17, 17, 17, 17, 17, 17, 17, 18, 18,
	18, 18, 18, 18, 18, 18, 19, 19, 19, 19,
	19, 19, 19, 19, 20, 20, 20, 20, 20, 20,
	20, 20, 20, 20, 20, 20, 20, 20, 20, 20,
	21, 21, 21, 21, 21, 21, 21, 21, 21, 21,
	21, 21, 21, 21, 21, 21, 22, 22, 22, 22,
	22, 22, 22, 22, 22, 22, 22, 22, 22, 22,
	22, 22, 23, 23, 23, 23, 23, 23, 23, 23,
	23, 23, 23, 23, 23, 23, 23, 23, 24, 24,
	24, 24, 24, 24, 24, 24, 24, 24, 24, 24,
	24, 24, 24, 24, 24, 24, 24, 24, 24, 24,
	24, 24, 24, 24, 24, 24, 24, 24, 24, 24,
	25, 25, 25, 25, 25, 25, 25, 25, 25, 25,
	25, 25, 25, 25, 25, 25, 25, 25, 25, 25,
	25, 25, 25, 25, 25, 25, 25, 25, 25, 25,
	25, 25, 26, 26, 26, 26, 26, 26, 26, 26,
	26, 26, 26, 26, 26, 26, 26, 26, 26, 26,
	26, 26, 26, 26, 26, 26, 26, 26, 26, 26,
	26, 26, 26, 26, 27, 27, 27, 27, 27, 27,
	27, 27, 27, 27, 27, 27, 27, 27, 27, 27,
	27, 27, 27, 27, 27, 27, 27, 27, 27, 27,
	27, 27, 27, 27, 27, 28,
}

// lengthCodes1 is length codes, but starting at 1.
var lengthCodes1 = [256]uint8{
	1, 2, 3, 4, 5, 6, 7, 8, 9, 9,
	10, 10, 11, 11, 12, 12, 13, 13, 13, 13,
	14, 14, 14, 14, 15, 15, 15, 15, 16, 16,
	16, 16, 17, 17, 17, 17, 17, 17, 17, 17,
	18, 18, 18, 18, 18, 18, 18, 18, 19, 19,
	19, 19, 19, 19, 19, 19, 20, 20, 20, 20,
	20, 20, 20, 20, 21, 21, 21, 21, 21, 21,
	21, 21, 21, 21, 21, 21, 21, 21, 21, 21,
	22, 22, 22, 22, 22, 22, 22, 22, 22, 22,
	22, 22, 22, 22, 22, 22, 23, 23, 23, 23,
	23, 23, 23, 23, 23, 23, 23, 23, 23, 23,
	23, 23, 24, 24, 24, 24, 24, 24, 24, 24,
	24, 24, 24, 24, 24, 24, 24, 24, 25, 25,
	25, 25, 25, 25, 25, 25, 25, 25, 25, 25,
	25, 25, 25, 25, 25, 25, 25, 25, 25, 25,
	25, 25, 25, 25, 25, 25, 25, 25, 25, 25,
	26, 26, 26, 26, 26, 26, 26, 26, 26, 26,
	26, 26, 26, 26, 26, 26, 26, 26, 26, 26,
	26, 26, 26, 26, 26, 26, 26, 26, 26, 26,
	26, 26, 27, 27, 27, 27, 27, 27, 27, 27,
	27, 27, 27, 27, 27, 27, 27, 27, 27, 27,
	27, 27, 27, 27, 27, 27, 27, 27, 27, 27,
	27, 27, 27, 27, 28, 28, 28, 28, 28, 28,
	28, 28, 28, 28, 28, 28, 28, 28, 28, 28,
	28, 28, 28, 28, 28, 28, 28, 28, 28, 28,
	28, 28, 28, 28, 28, 29,
}

var offsetCodes = [256]uint32{
	0, 1, 2, 3, 4, 4, 5, 5, 6, 6, 6, 6, 7, 7, 7, 7,
	8, 8, 8, 8, 8, 8, 8, 8, 9, 9, 9, 9, 9, 9, 9, 9,
	10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10,
	11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11,
	12, 12, 12, 12, 12, 12, 12, 12, 12, 12, 12, 12, 12, 12, 12, 12,
	12, 12, 12, 12, 12, 12, 12, 12, 12, 12, 12, 12, 12, 12, 12, 12,
	13, 13, 13, 13, 13, 13, 13, 13, 13, 13, 13, 13, 13, 13, 13, 13,
	13, 13, 13, 13, 13, 13, 13, 13, 13, 13, 13, 13, 13, 13, 13, 13,
	14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14,
	14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14,
	14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14,
	14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14,
	15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15,
	15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15,
	15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15,
	15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15,
}

// offsetCodes14 are offsetCodes, but with 14 added.
var offsetCodes14 = [256]uint32{
	14, 15, 16, 17, 18, 18, 19, 19, 20, 20, 20, 20, 21, 21, 21, 21,
	22, 22, 22, 22, 22, 22, 22, 22, 23, 23, 23, 23, 23, 23, 23, 23,
	24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24,
	25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25,
	26, 26, 26, 26, 26, 26, 26, 26, 26, 26, 26, 26, 26, 26, 26, 26,
	26, 26, 26, 26, 26, 26, 26, 26, 26, 26, 26, 26, 26, 26, 26, 26,
	27, 27, 27, 27, 27, 27, 27, 27, 27, 27, 27, 27, 27, 27, 27, 27,
	27, 27, 27, 27, 27, 27, 27, 27, 27, 27, 27, 27, 27, 27, 27, 27,
	28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28,
	28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28,
	28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28,
	28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28,
	29, 29, 29, 29, 29, 29, 29, 29, 29, 29, 29, 29, 29, 29, 29, 29,
	29, 29, 29, 29, 29, 29, 29, 29, 29, 29, 29, 29, 29, 29, 29, 29,
	29, 29, 29, 29, 29, 29, 29, 29, 29, 29, 29, 29, 29, 29, 29, 29,
	29, 29, 29, 29, 29, 29, 29, 29, 29, 29, 29, 29, 29, 29, 29, 29,
}

// A token is a token that will be written to output stream.
// It is either a literal or a match with offset and length.
type token uint32

// tokens are compound values as described above.
// Histograms are created as tokens are added.
// A full block is allocated.
type tokens struct {
	extraHist [32]uint16  // codes 256->maxnumlit
	offHist   [32]uint16  // offset codes
	litHist   [256]uint16 // codes 0->255
	nFilled   int
	n         uint16 // Must be able to contain maxStoreBlockSize
	tokens    [65536]token
}

// Reset resets the tokens and histograms.
func (t *tokens) Reset() {
	if t.n == 0 {
		return
	}
	t.n = 0
	t.nFilled = 0
	clear(t.litHist[:])
	clear(t.extraHist[:])
	clear(t.offHist[:])
}

// indexTokens creates tokens from a slice of unindexed tokens.
func indexTokens(in []token) tokens {
	var t tokens
	t.indexTokens(in)
	return t
}

// indexTokens clears and sets t from a slice of unindexed tokens.
func (t *tokens) indexTokens(in []token) {
	t.Reset()
	for _, tok := range in {
		if tok < matchType {
			t.AddLiteral(tok.literal())
			continue
		}
		t.AddMatch(uint32(tok.length()), tok.offset()&matchOffsetOnlyMask)
	}
}

// emitLiterals writes a literal chunk and returns the number of bytes written.
func emitLiterals(dst *tokens, lit []byte) {
	for _, v := range lit {
		dst.tokens[dst.n] = token(v)
		dst.litHist[v]++
		dst.n++
	}
}

// AddLiteral adds a single literal to the tokens.
func (t *tokens) AddLiteral(lit byte) {
	t.tokens[t.n] = token(lit)
	t.litHist[lit]++
	t.n++
}

// mFastLog2 returns a fast approximation of log2(val).
// From https://stackoverflow.com/a/28730362.
func mFastLog2(val float32) float32 {
	ux := int32(math.Float32bits(val))
	log2 := (float32)(((ux >> 23) & 255) - 128)
	ux &= -0x7f800001
	ux += 127 << 23
	uval := math.Float32frombits(uint32(ux))
	log2 += ((-0.34484843)*uval+2.02466578)*uval - 0.67487759
	return log2
}

// EstimatedBits returns an estimated minimum size for the
// optimal compression of t.
// Minimum 1 bit is assigned per symbol.
// Maximum 15 bits are assigned per symbol.
func (t *tokens) EstimatedBits() int {
	shannon := float32(0)
	bits := int(0)
	nMatches := 0
	total := int(t.n) + t.nFilled
	if total > 0 {
		invTotal := 1.0 / float32(total)
		for _, v := range t.litHist[:] {
			if v > 0 {
				n := float32(v)
				shannon += min(15, max(1, -mFastLog2(n*invTotal))) * n
			}
		}
		// Just add 15 for EOB
		shannon += 15
		for i, v := range t.extraHist[1 : literalCount-256] {
			if v > 0 {
				n := float32(v)
				shannon += min(15, max(1, -mFastLog2(n*invTotal))) * n
				bits += int(lengthExtraBits[i&31]) * int(v)
				nMatches += int(v)
			}
		}
	}
	if nMatches > 0 {
		invTotal := 1.0 / float32(nMatches)
		for i, v := range t.offHist[:offsetCodeCount] {
			if v > 0 {
				n := float32(v)
				shannon += min(15, max(1, -mFastLog2(n*invTotal))) * n
				bits += int(offsetExtraBits[i&31]) * int(v)
			}
		}
	}
	return int(shannon) + bits
}

// AddMatch adds a match to the tokens.
// This function is very sensitive to inlining and right on the border.
func (t *tokens) AddMatch(xlength uint32, xoffset uint32) {
	oCode := offsetCode(xoffset)
	xoffset |= oCode << 16

	t.extraHist[lengthCodes1[uint8(xlength)]]++
	t.offHist[oCode&31]++
	t.tokens[t.n] = token(matchType | xlength<<lengthShift | xoffset)
	t.n++
}

// AddMatchLong adds a match to the tokens, potentially longer than max match length.
// Length should NOT have the base subtracted, only offset should.
func (t *tokens) AddMatchLong(xlength int32, xoffset uint32) {
	oc := offsetCode(xoffset)
	xoffset |= oc << 16
	for xlength > 0 {
		xl := xlength
		if xl > 258 {
			// We need to have at least baseMatchLength left over for next loop.
			if xl > 258+baseMatchLength {
				xl = 258
			} else {
				xl = 258 - baseMatchLength
			}
		}
		xlength -= xl
		xl -= baseMatchLength
		t.extraHist[lengthCodes1[uint8(xl)]]++
		t.offHist[oc&31]++
		t.tokens[t.n] = token(matchType | uint32(xl)<<lengthShift | xoffset)
		t.n++
	}
}

// AddEOB adds an end of block marker to the tokens.
func (t *tokens) AddEOB() {
	t.tokens[t.n] = token(endBlockMarker)
	t.extraHist[0]++
	t.n++
}

// Slice returns a slice of the tokens that references the tokens in t.
func (t *tokens) Slice() []token {
	return t.tokens[:t.n]
}

// typ returns the type of a token.
func (t token) typ() uint32 { return uint32(t) & typeMask }

// literal returns the literal value of t.
func (t token) literal() uint8 { return uint8(t) }

// offset returns the offset of a match token.
func (t token) offset() uint32 { return uint32(t) & offsetMask }

// length returns the length of a match token.
func (t token) length() uint8 { return uint8(t >> lengthShift) }

// lengthCode converts a match length to its code.
func lengthCode(len uint8) uint8 { return lengthCodes[len] }

// offsetCode returns the offset code corresponding to a specific offset.
func offsetCode(off uint32) uint32 {
	if off < uint32(len(offsetCodes)) {
		return offsetCodes[uint8(off)]
	}
	return offsetCodes14[uint8(off>>7)]
}

```

// === FILE: references/go/src/compress/gzip/gunzip.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gzip implements reading and writing of gzip format compressed files,
// as specified in RFC 1952.
package gzip

import (
	"bufio"
	"compress/flate"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"time"
)

const (
	gzipID1     = 0x1f
	gzipID2     = 0x8b
	gzipDeflate = 8
	flagText    = 1 << 0
	flagHdrCrc  = 1 << 1
	flagExtra   = 1 << 2
	flagName    = 1 << 3
	flagComment = 1 << 4
)

var (
	// ErrChecksum is returned when reading GZIP data that has an invalid checksum.
	ErrChecksum = errors.New("gzip: invalid checksum")
	// ErrHeader is returned when reading GZIP data that has an invalid header.
	ErrHeader = errors.New("gzip: invalid header")
)

var le = binary.LittleEndian

// noEOF converts io.EOF to io.ErrUnexpectedEOF.
func noEOF(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}

// The gzip file stores a header giving metadata about the compressed file.
// That header is exposed as the fields of the [Writer] and [Reader] structs.
//
// Strings must be UTF-8 encoded and may only contain Unicode code points
// U+0001 through U+00FF, due to limitations of the GZIP file format.
type Header struct {
	Comment string    // comment
	Extra   []byte    // "extra data"
	ModTime time.Time // modification time
	Name    string    // file name
	OS      byte      // operating system type
}

// A Reader is an [io.Reader] that can be read to retrieve
// uncompressed data from a gzip-format compressed file.
//
// In general, a gzip file can be a concatenation of gzip files,
// each with its own header. Reads from the Reader
// return the concatenation of the uncompressed data of each.
// Only the first header is recorded in the Reader fields.
//
// Gzip files store a length and checksum of the uncompressed data.
// The Reader will return an [ErrChecksum] when [Reader.Read]
// reaches the end of the uncompressed data if it does not
// have the expected length or checksum. Clients should treat data
// returned by [Reader.Read] as tentative until they receive the [io.EOF]
// marking the end of the data.
type Reader struct {
	Header       // valid after NewReader or Reader.Reset
	r            flate.Reader
	decompressor io.ReadCloser
	digest       uint32 // CRC-32, IEEE polynomial (section 8)
	size         uint32 // Uncompressed size (section 2.3.1)
	buf          [512]byte
	err          error
	multistream  bool
}

// NewReader creates a new [Reader] reading the given reader.
// If r does not also implement [io.ByteReader],
// the decompressor may read more data than necessary from r.
//
// It is the caller's responsibility to call [Reader.Close] when done.
//
// The Reader.[Header] fields will be valid in the [Reader] returned.
func NewReader(r io.Reader) (*Reader, error) {
	z := new(Reader)
	if err := z.Reset(r); err != nil {
		return nil, err
	}
	return z, nil
}

// Reset discards the [Reader] z's state and makes it equivalent to the
// result of its original state from [NewReader], but reading from r instead.
// This permits reusing a [Reader] rather than allocating a new one.
func (z *Reader) Reset(r io.Reader) error {
	*z = Reader{
		decompressor: z.decompressor,
		multistream:  true,
	}
	if rr, ok := r.(flate.Reader); ok {
		z.r = rr
	} else {
		z.r = bufio.NewReader(r)
	}
	z.Header, z.err = z.readHeader()
	return z.err
}

// Multistream controls whether the reader supports multistream files.
//
// If enabled (the default), the [Reader] expects the input to be a sequence
// of individually gzipped data streams, each with its own header and
// trailer, ending at EOF. The effect is that the concatenation of a sequence
// of gzipped files is treated as equivalent to the gzip of the concatenation
// of the sequence. This is standard behavior for gzip readers.
//
// Calling Multistream(false) disables this behavior; disabling the behavior
// can be useful when reading file formats that distinguish individual gzip
// data streams or mix gzip data streams with other data streams.
// In this mode, when the [Reader] reaches the end of the data stream,
// [Reader.Read] returns [io.EOF]. The underlying reader must implement [io.ByteReader]
// in order to be left positioned just after the gzip stream.
// To start the next stream, call z.Reset(r) followed by z.Multistream(false).
// If there is no next stream, z.Reset(r) will return [io.EOF].
func (z *Reader) Multistream(ok bool) {
	z.multistream = ok
}

// readString reads a NUL-terminated string from z.r.
// It treats the bytes read as being encoded as ISO 8859-1 (Latin-1) and
// will output a string encoded using UTF-8.
// This method always updates z.digest with the data read.
func (z *Reader) readString() (string, error) {
	var err error
	needConv := false
	for i := 0; ; i++ {
		if i >= len(z.buf) {
			return "", ErrHeader
		}
		z.buf[i], err = z.r.ReadByte()
		if err != nil {
			return "", err
		}
		if z.buf[i] > 0x7f {
			needConv = true
		}
		if z.buf[i] == 0 {
			// Digest covers the NUL terminator.
			z.digest = crc32.Update(z.digest, crc32.IEEETable, z.buf[:i+1])

			// Strings are ISO 8859-1, Latin-1 (RFC 1952, section 2.3.1).
			if needConv {
				s := make([]rune, 0, i)
				for _, v := range z.buf[:i] {
					s = append(s, rune(v))
				}
				return string(s), nil
			}
			return string(z.buf[:i]), nil
		}
	}
}

// readHeader reads the GZIP header according to section 2.3.1.
// This method does not set z.err.
func (z *Reader) readHeader() (hdr Header, err error) {
	if _, err = io.ReadFull(z.r, z.buf[:10]); err != nil {
		// RFC 1952, section 2.2, says the following:
		//	A gzip file consists of a series of "members" (compressed data sets).
		//
		// Other than this, the specification does not clarify whether a
		// "series" is defined as "one or more" or "zero or more". To err on the
		// side of caution, Go interprets this to mean "zero or more".
		// Thus, it is okay to return io.EOF here.
		return hdr, err
	}
	if z.buf[0] != gzipID1 || z.buf[1] != gzipID2 || z.buf[2] != gzipDeflate {
		return hdr, ErrHeader
	}
	flg := z.buf[3]
	if t := int64(le.Uint32(z.buf[4:8])); t > 0 {
		// Section 2.3.1, the zero value for MTIME means that the
		// modified time is not set.
		hdr.ModTime = time.Unix(t, 0)
	}
	// z.buf[8] is XFL and is currently ignored.
	hdr.OS = z.buf[9]
	z.digest = crc32.ChecksumIEEE(z.buf[:10])

	if flg&flagExtra != 0 {
		if _, err = io.ReadFull(z.r, z.buf[:2]); err != nil {
			return hdr, noEOF(err)
		}
		z.digest = crc32.Update(z.digest, crc32.IEEETable, z.buf[:2])
		data := make([]byte, le.Uint16(z.buf[:2]))
		if _, err = io.ReadFull(z.r, data); err != nil {
			return hdr, noEOF(err)
		}
		z.digest = crc32.Update(z.digest, crc32.IEEETable, data)
		hdr.Extra = data
	}

	var s string
	if flg&flagName != 0 {
		if s, err = z.readString(); err != nil {
			return hdr, noEOF(err)
		}
		hdr.Name = s
	}

	if flg&flagComment != 0 {
		if s, err = z.readString(); err != nil {
			return hdr, noEOF(err)
		}
		hdr.Comment = s
	}

	if flg&flagHdrCrc != 0 {
		if _, err = io.ReadFull(z.r, z.buf[:2]); err != nil {
			return hdr, noEOF(err)
		}
		digest := le.Uint16(z.buf[:2])
		if digest != uint16(z.digest) {
			return hdr, ErrHeader
		}
	}

	z.digest = 0
	if z.decompressor == nil {
		z.decompressor = flate.NewReader(z.r)
	} else {
		z.decompressor.(flate.Resetter).Reset(z.r, nil)
	}
	return hdr, nil
}

// Read implements [io.Reader], reading uncompressed bytes from its underlying reader.
func (z *Reader) Read(p []byte) (n int, err error) {
	if z.err != nil {
		return 0, z.err
	}

	for n == 0 {
		n, z.err = z.decompressor.Read(p)
		z.digest = crc32.Update(z.digest, crc32.IEEETable, p[:n])
		z.size += uint32(n)
		if z.err != io.EOF {
			// In the normal case we return here.
			return n, z.err
		}

		// Finished file; check checksum and size.
		if _, err := io.ReadFull(z.r, z.buf[:8]); err != nil {
			z.err = noEOF(err)
			return n, z.err
		}
		digest := le.Uint32(z.buf[:4])
		size := le.Uint32(z.buf[4:8])
		if digest != z.digest || size != z.size {
			z.err = ErrChecksum
			return n, z.err
		}
		z.digest, z.size = 0, 0

		// File is ok; check if there is another.
		if !z.multistream {
			return n, io.EOF
		}
		z.err = nil // Remove io.EOF

		if _, z.err = z.readHeader(); z.err != nil {
			return n, z.err
		}
	}

	return n, nil
}

// Close closes the [Reader]. It does not close the underlying reader.
// In order for the GZIP checksum to be verified, the reader must be
// fully consumed until the [io.EOF].
func (z *Reader) Close() error { return z.decompressor.Close() }

```

// === FILE: references/go/src/compress/gzip/gzip.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gzip

import (
	"compress/flate"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"time"
)

// These constants are copied from the [flate] package, so that code that imports
// [compress/gzip] does not also have to import [compress/flate].
const (
	NoCompression      = flate.NoCompression
	BestSpeed          = flate.BestSpeed
	BestCompression    = flate.BestCompression
	DefaultCompression = flate.DefaultCompression
	HuffmanOnly        = flate.HuffmanOnly
)

// A Writer is an [io.WriteCloser].
// Writes to a Writer are compressed and written to w.
type Writer struct {
	Header      // written at first call to Write, Flush, or Close
	w           io.Writer
	level       int
	wroteHeader bool
	closed      bool
	buf         [10]byte
	compressor  *flate.Writer
	digest      uint32 // CRC-32, IEEE polynomial (section 8)
	size        uint32 // Uncompressed size (section 2.3.1)
	err         error
}

// NewWriter returns a new [Writer].
// Writes to the returned writer are compressed and written to w.
//
// It is the caller's responsibility to call Close on the [Writer] when done.
// Writes may be buffered and not flushed until Close.
//
// Callers that wish to set the fields in Writer.[Header] must do so before
// the first call to Write, Flush, or Close.
//
// Note that the exact bytes written to w are not covered by the Go 1
// compatibility promise. Callers, including tests, should not depend on the
// exact written bytes.
func NewWriter(w io.Writer) *Writer {
	z, _ := NewWriterLevel(w, DefaultCompression)
	return z
}

// NewWriterLevel is like [NewWriter] but specifies the compression level instead
// of assuming [DefaultCompression].
//
// The compression level can be [DefaultCompression], [NoCompression], [HuffmanOnly]
// or any integer value between [BestSpeed] and [BestCompression] inclusive.
// The error returned will be nil if the level is valid.
//
// Note that the exact bytes written to w are not covered by the Go 1
// compatibility promise. Callers, including tests, should not depend on the
// exact written bytes.
func NewWriterLevel(w io.Writer, level int) (*Writer, error) {
	if level < HuffmanOnly || level > BestCompression {
		return nil, fmt.Errorf("gzip: invalid compression level: %d", level)
	}
	z := new(Writer)
	z.init(w, level)
	return z, nil
}

func (z *Writer) init(w io.Writer, level int) {
	compressor := z.compressor
	if compressor != nil {
		compressor.Reset(w)
	}
	*z = Writer{
		Header: Header{
			OS: 255, // unknown
		},
		w:          w,
		level:      level,
		compressor: compressor,
	}
}

// Reset discards the [Writer] z's state and makes it equivalent to the
// result of its original state from [NewWriter] or [NewWriterLevel], but
// writing to w instead. This permits reusing a [Writer] rather than
// allocating a new one.
func (z *Writer) Reset(w io.Writer) {
	z.init(w, z.level)
}

// writeBytes writes a length-prefixed byte slice to z.w.
func (z *Writer) writeBytes(b []byte) error {
	if len(b) > 0xffff {
		return errors.New("gzip.Write: Extra data is too large")
	}
	le.PutUint16(z.buf[:2], uint16(len(b)))
	_, err := z.w.Write(z.buf[:2])
	if err != nil {
		return err
	}
	_, err = z.w.Write(b)
	return err
}

// writeString writes a UTF-8 string s in GZIP's format to z.w.
// GZIP (RFC 1952) specifies that strings are NUL-terminated ISO 8859-1 (Latin-1).
func (z *Writer) writeString(s string) (err error) {
	// GZIP stores Latin-1 strings; error if non-Latin-1; convert if non-ASCII.
	needconv := false
	for _, v := range s {
		if v == 0 || v > 0xff {
			return errors.New("gzip.Write: non-Latin-1 header string")
		}
		if v > 0x7f {
			needconv = true
		}
	}
	if needconv {
		b := make([]byte, 0, len(s))
		for _, v := range s {
			b = append(b, byte(v))
		}
		_, err = z.w.Write(b)
	} else {
		_, err = io.WriteString(z.w, s)
	}
	if err != nil {
		return err
	}
	// GZIP strings are NUL-terminated.
	z.buf[0] = 0
	_, err = z.w.Write(z.buf[:1])
	return err
}

// Write writes a compressed form of p to the underlying [io.Writer]. The
// compressed bytes are not necessarily flushed until the [Writer] is closed.
func (z *Writer) Write(p []byte) (int, error) {
	if z.err != nil {
		return 0, z.err
	}
	var n int
	// Write the GZIP header lazily.
	if !z.wroteHeader {
		z.wroteHeader = true
		z.buf = [10]byte{0: gzipID1, 1: gzipID2, 2: gzipDeflate}
		if z.Extra != nil {
			z.buf[3] |= 0x04
		}
		if z.Name != "" {
			z.buf[3] |= 0x08
		}
		if z.Comment != "" {
			z.buf[3] |= 0x10
		}
		if z.ModTime.After(time.Unix(0, 0)) {
			// Section 2.3.1, the zero value for MTIME means that the
			// modified time is not set.
			le.PutUint32(z.buf[4:8], uint32(z.ModTime.Unix()))
		}
		if z.level == BestCompression {
			z.buf[8] = 2
		} else if z.level == BestSpeed {
			z.buf[8] = 4
		}
		z.buf[9] = z.OS
		_, z.err = z.w.Write(z.buf[:10])
		if z.err != nil {
			return 0, z.err
		}
		if z.Extra != nil {
			z.err = z.writeBytes(z.Extra)
			if z.err != nil {
				return 0, z.err
			}
		}
		if z.Name != "" {
			z.err = z.writeString(z.Name)
			if z.err != nil {
				return 0, z.err
			}
		}
		if z.Comment != "" {
			z.err = z.writeString(z.Comment)
			if z.err != nil {
				return 0, z.err
			}
		}
		if z.compressor == nil {
			z.compressor, _ = flate.NewWriter(z.w, z.level)
		}
	}
	z.size += uint32(len(p))
	z.digest = crc32.Update(z.digest, crc32.IEEETable, p)
	n, z.err = z.compressor.Write(p)
	return n, z.err
}

// Flush flushes any pending compressed data to the underlying writer.
//
// It is useful mainly in compressed network protocols, to ensure that
// a remote reader has enough data to reconstruct a packet. Flush does
// not return until the data has been written. If the underlying
// writer returns an error, Flush returns that error.
//
// In the terminology of the zlib library, Flush is equivalent to Z_SYNC_FLUSH.
func (z *Writer) Flush() error {
	if z.err != nil {
		return z.err
	}
	if z.closed {
		return nil
	}
	if !z.wroteHeader {
		z.Write(nil)
		if z.err != nil {
			return z.err
		}
	}
	z.err = z.compressor.Flush()
	return z.err
}

// Close closes the [Writer] by flushing any unwritten data to the underlying
// [io.Writer] and writing the GZIP footer.
// It does not close the underlying [io.Writer].
func (z *Writer) Close() error {
	if z.err != nil {
		return z.err
	}
	if z.closed {
		return nil
	}
	z.closed = true
	if !z.wroteHeader {
		z.Write(nil)
		if z.err != nil {
			return z.err
		}
	}
	z.err = z.compressor.Close()
	if z.err != nil {
		return z.err
	}
	le.PutUint32(z.buf[:4], z.digest)
	le.PutUint32(z.buf[4:8], z.size)
	_, z.err = z.w.Write(z.buf[:8])
	return z.err
}

```

// === FILE: references/go/src/compress/lzw/reader.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package lzw implements the Lempel-Ziv-Welch compressed data format,
// described in T. A. Welch, “A Technique for High-Performance Data
// Compression”, Computer, 17(6) (June 1984), pp 8-19.
//
// In particular, it implements LZW as used by the GIF and PDF file
// formats, which means variable-width codes up to 12 bits and the first
// two non-literal codes are a clear code and an EOF code.
//
// The TIFF file format uses a similar but incompatible version of the LZW
// algorithm. See the [golang.org/x/image/tiff/lzw] package for an
// implementation.
package lzw

// TODO(nigeltao): check that PDF uses LZW in the same way as GIF,
// modulo LSB/MSB packing order.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

// Order specifies the bit ordering in an LZW data stream.
type Order int

const (
	// LSB means Least Significant Bits first, as used in the GIF file format.
	LSB Order = iota
	// MSB means Most Significant Bits first, as used in the TIFF and PDF
	// file formats.
	MSB
)

const (
	maxWidth           = 12
	decoderInvalidCode = 0xffff
	flushBuffer        = 1 << maxWidth
)

// Reader is an [io.Reader] which can be used to read compressed data in the
// LZW format.
type Reader struct {
	r        io.ByteReader
	bits     uint32
	nBits    uint
	width    uint
	read     func(*Reader) (uint16, error) // readLSB or readMSB
	litWidth int                           // width in bits of literal codes
	err      error

	// The first 1<<litWidth codes are literal codes.
	// The next two codes mean clear and EOF.
	// Other valid codes are in the range [lo, hi] where lo := clear + 2,
	// with the upper bound incrementing on each code seen.
	//
	// overflow is the code at which hi overflows the code width. It always
	// equals 1 << width.
	//
	// last is the most recently seen code, or decoderInvalidCode.
	//
	// An invariant is that hi < overflow.
	clear, eof, hi, overflow, last uint16

	// Each code c in [lo, hi] expands to two or more bytes. For c != hi:
	//   suffix[c] is the last of these bytes.
	//   prefix[c] is the code for all but the last byte.
	//   This code can either be a literal code or another code in [lo, c).
	// The c == hi case is a special case.
	suffix [1 << maxWidth]uint8
	prefix [1 << maxWidth]uint16

	// output is the temporary output buffer.
	// Literal codes are accumulated from the start of the buffer.
	// Non-literal codes decode to a sequence of suffixes that are first
	// written right-to-left from the end of the buffer before being copied
	// to the start of the buffer.
	// It is flushed when it contains >= 1<<maxWidth bytes,
	// so that there is always room to decode an entire code.
	output [2 * 1 << maxWidth]byte
	o      int    // write index into output
	toRead []byte // bytes to return from Read
}

// readLSB returns the next code for "Least Significant Bits first" data.
func (r *Reader) readLSB() (uint16, error) {
	for r.nBits < r.width {
		x, err := r.r.ReadByte()
		if err != nil {
			return 0, err
		}
		r.bits |= uint32(x) << r.nBits
		r.nBits += 8
	}
	code := uint16(r.bits & (1<<r.width - 1))
	r.bits >>= r.width
	r.nBits -= r.width
	return code, nil
}

// readMSB returns the next code for "Most Significant Bits first" data.
func (r *Reader) readMSB() (uint16, error) {
	for r.nBits < r.width {
		x, err := r.r.ReadByte()
		if err != nil {
			return 0, err
		}
		r.bits |= uint32(x) << (24 - r.nBits)
		r.nBits += 8
	}
	code := uint16(r.bits >> (32 - r.width))
	r.bits <<= r.width
	r.nBits -= r.width
	return code, nil
}

// Read implements io.Reader, reading uncompressed bytes from its underlying reader.
func (r *Reader) Read(b []byte) (int, error) {
	for {
		if len(r.toRead) > 0 {
			n := copy(b, r.toRead)
			r.toRead = r.toRead[n:]
			return n, nil
		}
		if r.err != nil {
			return 0, r.err
		}
		r.decode()
	}
}

// decode decompresses bytes from r and leaves them in d.toRead.
// read specifies how to decode bytes into codes.
// litWidth is the width in bits of literal codes.
func (r *Reader) decode() {
	// Loop over the code stream, converting codes into decompressed bytes.
loop:
	for {
		code, err := r.read(r)
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			r.err = err
			break
		}
		switch {
		case code < r.clear:
			// We have a literal code.
			r.output[r.o] = uint8(code)
			r.o++
			if r.last != decoderInvalidCode {
				// Save what the hi code expands to.
				r.suffix[r.hi] = uint8(code)
				r.prefix[r.hi] = r.last
			}
		case code == r.clear:
			r.width = 1 + uint(r.litWidth)
			r.hi = r.eof
			r.overflow = 1 << r.width
			r.last = decoderInvalidCode
			continue
		case code == r.eof:
			r.err = io.EOF
			break loop
		case code <= r.hi:
			c, i := code, len(r.output)-1
			if code == r.hi && r.last != decoderInvalidCode {
				// code == hi is a special case which expands to the last expansion
				// followed by the head of the last expansion. To find the head, we walk
				// the prefix chain until we find a literal code.
				c = r.last
				for c >= r.clear {
					c = r.prefix[c]
				}
				r.output[i] = uint8(c)
				i--
				c = r.last
			}
			// Copy the suffix chain into output and then write that to w.
			for c >= r.clear {
				r.output[i] = r.suffix[c]
				i--
				c = r.prefix[c]
			}
			r.output[i] = uint8(c)
			r.o += copy(r.output[r.o:], r.output[i:])
			if r.last != decoderInvalidCode {
				// Save what the hi code expands to.
				r.suffix[r.hi] = uint8(c)
				r.prefix[r.hi] = r.last
			}
		default:
			r.err = errors.New("lzw: invalid code")
			break loop
		}
		r.last, r.hi = code, r.hi+1
		if r.hi >= r.overflow {
			if r.hi > r.overflow {
				panic("unreachable")
			}
			if r.width == maxWidth {
				r.last = decoderInvalidCode
				// Undo the d.hi++ a few lines above, so that (1) we maintain
				// the invariant that d.hi < d.overflow, and (2) d.hi does not
				// eventually overflow a uint16.
				r.hi--
			} else {
				r.width++
				r.overflow = 1 << r.width
			}
		}
		if r.o >= flushBuffer {
			break
		}
	}
	// Flush pending output.
	r.toRead = r.output[:r.o]
	r.o = 0
}

var errClosed = errors.New("lzw: reader/writer is closed")

// Close closes the [Reader] and returns an error for any future read operation.
// It does not close the underlying [io.Reader].
func (r *Reader) Close() error {
	r.err = errClosed // in case any Reads come along
	return nil
}

// Reset clears the [Reader]'s state and allows it to be reused again
// as a new [Reader].
func (r *Reader) Reset(src io.Reader, order Order, litWidth int) {
	*r = Reader{}
	r.init(src, order, litWidth)
}

// NewReader creates a new [io.ReadCloser].
// Reads from the returned [io.ReadCloser] read and decompress data from r.
// If r does not also implement [io.ByteReader],
// the decompressor may read more data than necessary from r.
// It is the caller's responsibility to call Close on the ReadCloser when
// finished reading.
// The number of bits to use for literal codes, litWidth, must be in the
// range [2,8] and is typically 8. It must equal the litWidth
// used during compression.
//
// It is guaranteed that the underlying type of the returned [io.ReadCloser]
// is a *[Reader].
func NewReader(r io.Reader, order Order, litWidth int) io.ReadCloser {
	return newReader(r, order, litWidth)
}

func newReader(src io.Reader, order Order, litWidth int) *Reader {
	r := new(Reader)
	r.init(src, order, litWidth)
	return r
}

func (r *Reader) init(src io.Reader, order Order, litWidth int) {
	switch order {
	case LSB:
		r.read = (*Reader).readLSB
	case MSB:
		r.read = (*Reader).readMSB
	default:
		r.err = errors.New("lzw: unknown order")
		return
	}
	if litWidth < 2 || 8 < litWidth {
		r.err = fmt.Errorf("lzw: litWidth %d out of range", litWidth)
		return
	}

	br, ok := src.(io.ByteReader)
	if !ok && src != nil {
		br = bufio.NewReader(src)
	}
	r.r = br
	r.litWidth = litWidth
	r.width = 1 + uint(litWidth)
	r.clear = uint16(1) << uint(litWidth)
	r.eof, r.hi = r.clear+1, r.clear+1
	r.overflow = uint16(1) << r.width
	r.last = decoderInvalidCode
}

```

// === FILE: references/go/src/compress/lzw/writer.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lzw

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

// A writer is a buffered, flushable writer.
type writer interface {
	io.ByteWriter
	Flush() error
}

const (
	// A code is a 12 bit value, stored as a uint32 when encoding to avoid
	// type conversions when shifting bits.
	maxCode     = 1<<12 - 1
	invalidCode = 1<<32 - 1
	// There are 1<<12 possible codes, which is an upper bound on the number of
	// valid hash table entries at any given point in time. tableSize is 4x that.
	tableSize = 4 * 1 << 12
	tableMask = tableSize - 1
	// A hash table entry is a uint32. Zero is an invalid entry since the
	// lower 12 bits of a valid entry must be a non-literal code.
	invalidEntry = 0
)

// Writer is an LZW compressor. It writes the compressed form of the data
// to an underlying writer (see [NewWriter]).
type Writer struct {
	// w is the writer that compressed bytes are written to.
	w writer
	// litWidth is the width in bits of literal codes.
	litWidth uint
	// order, write, bits, nBits and width are the state for
	// converting a code stream into a byte stream.
	order Order
	write func(*Writer, uint32) error
	nBits uint
	width uint
	bits  uint32
	// hi is the code implied by the next code emission.
	// overflow is the code at which hi overflows the code width.
	hi, overflow uint32
	// savedCode is the accumulated code at the end of the most recent Write
	// call. It is equal to invalidCode if there was no such call.
	savedCode uint32
	// err is the first error encountered during writing. Closing the writer
	// will make any future Write calls return errClosed
	err error
	// table is the hash table from 20-bit keys to 12-bit values. Each table
	// entry contains key<<12|val and collisions resolve by linear probing.
	// The keys consist of a 12-bit code prefix and an 8-bit byte suffix.
	// The values are a 12-bit code.
	table [tableSize]uint32
}

// writeLSB writes the code c for "Least Significant Bits first" data.
func (w *Writer) writeLSB(c uint32) error {
	w.bits |= c << w.nBits
	w.nBits += w.width
	for w.nBits >= 8 {
		if err := w.w.WriteByte(uint8(w.bits)); err != nil {
			return err
		}
		w.bits >>= 8
		w.nBits -= 8
	}
	return nil
}

// writeMSB writes the code c for "Most Significant Bits first" data.
func (w *Writer) writeMSB(c uint32) error {
	w.bits |= c << (32 - w.width - w.nBits)
	w.nBits += w.width
	for w.nBits >= 8 {
		if err := w.w.WriteByte(uint8(w.bits >> 24)); err != nil {
			return err
		}
		w.bits <<= 8
		w.nBits -= 8
	}
	return nil
}

// errOutOfCodes is an internal error that means that the writer has run out
// of unused codes and a clear code needs to be sent next.
var errOutOfCodes = errors.New("lzw: out of codes")

// incHi increments e.hi and checks for both overflow and running out of
// unused codes. In the latter case, incHi sends a clear code, resets the
// writer state and returns errOutOfCodes.
func (w *Writer) incHi() error {
	w.hi++
	if w.hi == w.overflow {
		w.width++
		w.overflow <<= 1
	}
	if w.hi == maxCode {
		clear := uint32(1) << w.litWidth
		if err := w.write(w, clear); err != nil {
			return err
		}
		w.width = w.litWidth + 1
		w.hi = clear + 1
		w.overflow = clear << 1
		for i := range w.table {
			w.table[i] = invalidEntry
		}
		return errOutOfCodes
	}
	return nil
}

// Write writes a compressed representation of p to w's underlying writer.
func (w *Writer) Write(p []byte) (n int, err error) {
	if w.err != nil {
		return 0, w.err
	}
	if len(p) == 0 {
		return 0, nil
	}
	if maxLit := uint8(1<<w.litWidth - 1); maxLit != 0xff {
		for _, x := range p {
			if x > maxLit {
				w.err = errors.New("lzw: input byte too large for the litWidth")
				return 0, w.err
			}
		}
	}
	n = len(p)
	code := w.savedCode
	if code == invalidCode {
		// This is the first write; send a clear code.
		// https://www.w3.org/Graphics/GIF/spec-gif89a.txt Appendix F
		// "Variable-Length-Code LZW Compression" says that "Encoders should
		// output a Clear code as the first code of each image data stream".
		//
		// LZW compression isn't only used by GIF, but it's cheap to follow
		// that directive unconditionally.
		clear := uint32(1) << w.litWidth
		if err := w.write(w, clear); err != nil {
			return 0, err
		}
		// After the starting clear code, the next code sent (for non-empty
		// input) is always a literal code.
		code, p = uint32(p[0]), p[1:]
	}
loop:
	for _, x := range p {
		literal := uint32(x)
		key := code<<8 | literal
		// If there is a hash table hit for this key then we continue the loop
		// and do not emit a code yet.
		hash := (key>>12 ^ key) & tableMask
		for h, t := hash, w.table[hash]; t != invalidEntry; {
			if key == t>>12 {
				code = t & maxCode
				continue loop
			}
			h = (h + 1) & tableMask
			t = w.table[h]
		}
		// Otherwise, write the current code, and literal becomes the start of
		// the next emitted code.
		if w.err = w.write(w, code); w.err != nil {
			return 0, w.err
		}
		code = literal
		// Increment e.hi, the next implied code. If we run out of codes, reset
		// the writer state (including clearing the hash table) and continue.
		if err1 := w.incHi(); err1 != nil {
			if err1 == errOutOfCodes {
				continue
			}
			w.err = err1
			return 0, w.err
		}
		// Otherwise, insert key -> e.hi into the map that e.table represents.
		for {
			if w.table[hash] == invalidEntry {
				w.table[hash] = (key << 12) | w.hi
				break
			}
			hash = (hash + 1) & tableMask
		}
	}
	w.savedCode = code
	return n, nil
}

// Close closes the [Writer], flushing any pending output. It does not close
// w's underlying writer.
func (w *Writer) Close() error {
	if w.err != nil {
		if w.err == errClosed {
			return nil
		}
		return w.err
	}
	// Make any future calls to Write return errClosed.
	w.err = errClosed
	// Write the savedCode if valid.
	if w.savedCode != invalidCode {
		if err := w.write(w, w.savedCode); err != nil {
			return err
		}
		if err := w.incHi(); err != nil && err != errOutOfCodes {
			return err
		}
	} else {
		// Write the starting clear code, as w.Write did not.
		clear := uint32(1) << w.litWidth
		if err := w.write(w, clear); err != nil {
			return err
		}
	}
	// Write the eof code.
	eof := uint32(1)<<w.litWidth + 1
	if err := w.write(w, eof); err != nil {
		return err
	}
	// Write the final bits.
	if w.nBits > 0 {
		if w.order == MSB {
			w.bits >>= 24
		}
		if err := w.w.WriteByte(uint8(w.bits)); err != nil {
			return err
		}
	}
	return w.w.Flush()
}

// Reset clears the [Writer]'s state and allows it to be reused again
// as a new [Writer].
func (w *Writer) Reset(dst io.Writer, order Order, litWidth int) {
	*w = Writer{}
	w.init(dst, order, litWidth)
}

// NewWriter creates a new [io.WriteCloser].
// Writes to the returned [io.WriteCloser] are compressed and written to w.
// It is the caller's responsibility to call Close on the WriteCloser when
// finished writing.
// The number of bits to use for literal codes, litWidth, must be in the
// range [2,8] and is typically 8. Input bytes must be less than 1<<litWidth.
//
// It is guaranteed that the underlying type of the returned [io.WriteCloser]
// is a *[Writer].
func NewWriter(w io.Writer, order Order, litWidth int) io.WriteCloser {
	return newWriter(w, order, litWidth)
}

func newWriter(dst io.Writer, order Order, litWidth int) *Writer {
	w := new(Writer)
	w.init(dst, order, litWidth)
	return w
}

func (w *Writer) init(dst io.Writer, order Order, litWidth int) {
	switch order {
	case LSB:
		w.write = (*Writer).writeLSB
	case MSB:
		w.write = (*Writer).writeMSB
	default:
		w.err = errors.New("lzw: unknown order")
		return
	}
	if litWidth < 2 || 8 < litWidth {
		w.err = fmt.Errorf("lzw: litWidth %d out of range", litWidth)
		return
	}
	bw, ok := dst.(writer)
	if !ok && dst != nil {
		bw = bufio.NewWriter(dst)
	}
	w.w = bw
	lw := uint(litWidth)
	w.order = order
	w.width = 1 + lw
	w.litWidth = lw
	w.hi = 1<<lw + 1
	w.overflow = 1 << (lw + 1)
	w.savedCode = invalidCode
}

```

// === FILE: references/go/src/compress/zlib/reader.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package zlib implements reading and writing of zlib format compressed data,
as specified in RFC 1950.

The implementation provides filters that uncompress during reading
and compress during writing.  For example, to write compressed data
to a buffer:

	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	w.Write([]byte("hello, world\n"))
	w.Close()

and to read that data back:

	r, err := zlib.NewReader(&b)
	io.Copy(os.Stdout, r)
	r.Close()
*/
package zlib

import (
	"bufio"
	"compress/flate"
	"encoding/binary"
	"errors"
	"hash"
	"hash/adler32"
	"io"
)

const (
	zlibDeflate   = 8
	zlibMaxWindow = 7
)

var (
	// ErrChecksum is returned when reading ZLIB data that has an invalid checksum.
	ErrChecksum = errors.New("zlib: invalid checksum")
	// ErrDictionary is returned when reading ZLIB data that has an invalid dictionary.
	ErrDictionary = errors.New("zlib: invalid dictionary")
	// ErrHeader is returned when reading ZLIB data that has an invalid header.
	ErrHeader = errors.New("zlib: invalid header")
)

type reader struct {
	r            flate.Reader
	decompressor io.ReadCloser
	digest       hash.Hash32
	err          error
	scratch      [4]byte
}

// Resetter resets a ReadCloser returned by [NewReader] or [NewReaderDict]
// to switch to a new underlying Reader. This permits reusing a ReadCloser
// instead of allocating a new one.
type Resetter interface {
	// Reset discards any buffered data and resets the Resetter as if it was
	// newly initialized with the given reader.
	Reset(r io.Reader, dict []byte) error
}

// NewReader creates a new ReadCloser.
// Reads from the returned ReadCloser read and decompress data from r.
// If r does not implement [io.ByteReader], the decompressor may read more
// data than necessary from r.
// It is the caller's responsibility to call Close on the ReadCloser when done.
//
// The [io.ReadCloser] returned by NewReader also implements [Resetter].
func NewReader(r io.Reader) (io.ReadCloser, error) {
	return NewReaderDict(r, nil)
}

// NewReaderDict is like [NewReader] but uses a preset dictionary.
// NewReaderDict ignores the dictionary if the compressed data does not refer to it.
// If the compressed data refers to a different dictionary, NewReaderDict returns [ErrDictionary].
//
// The ReadCloser returned by NewReaderDict also implements [Resetter].
func NewReaderDict(r io.Reader, dict []byte) (io.ReadCloser, error) {
	z := new(reader)
	err := z.Reset(r, dict)
	if err != nil {
		return nil, err
	}
	return z, nil
}

func (z *reader) Read(p []byte) (int, error) {
	if z.err != nil {
		return 0, z.err
	}

	var n int
	n, z.err = z.decompressor.Read(p)
	z.digest.Write(p[0:n])
	if z.err != io.EOF {
		// In the normal case we return here.
		return n, z.err
	}

	// Finished file; check checksum.
	if _, err := io.ReadFull(z.r, z.scratch[0:4]); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		z.err = err
		return n, z.err
	}
	// ZLIB (RFC 1950) is big-endian, unlike GZIP (RFC 1952).
	checksum := binary.BigEndian.Uint32(z.scratch[:4])
	if checksum != z.digest.Sum32() {
		z.err = ErrChecksum
		return n, z.err
	}
	return n, io.EOF
}

// Calling Close does not close the wrapped [io.Reader] originally passed to [NewReader].
// In order for the ZLIB checksum to be verified, the reader must be
// fully consumed until the [io.EOF].
func (z *reader) Close() error {
	if z.err != nil && z.err != io.EOF {
		return z.err
	}
	z.err = z.decompressor.Close()
	return z.err
}

func (z *reader) Reset(r io.Reader, dict []byte) error {
	*z = reader{decompressor: z.decompressor}
	if fr, ok := r.(flate.Reader); ok {
		z.r = fr
	} else {
		z.r = bufio.NewReader(r)
	}

	// Read the header (RFC 1950 section 2.2.).
	_, z.err = io.ReadFull(z.r, z.scratch[0:2])
	if z.err != nil {
		if z.err == io.EOF {
			z.err = io.ErrUnexpectedEOF
		}
		return z.err
	}
	h := binary.BigEndian.Uint16(z.scratch[:2])
	if (z.scratch[0]&0x0f != zlibDeflate) || (z.scratch[0]>>4 > zlibMaxWindow) || (h%31 != 0) {
		z.err = ErrHeader
		return z.err
	}
	haveDict := z.scratch[1]&0x20 != 0
	if haveDict {
		_, z.err = io.ReadFull(z.r, z.scratch[0:4])
		if z.err != nil {
			if z.err == io.EOF {
				z.err = io.ErrUnexpectedEOF
			}
			return z.err
		}
		checksum := binary.BigEndian.Uint32(z.scratch[:4])
		if checksum != adler32.Checksum(dict) {
			z.err = ErrDictionary
			return z.err
		}
	}

	if z.decompressor == nil {
		if haveDict {
			z.decompressor = flate.NewReaderDict(z.r, dict)
		} else {
			z.decompressor = flate.NewReader(z.r)
		}
	} else {
		z.decompressor.(flate.Resetter).Reset(z.r, dict)
	}
	z.digest = adler32.New()
	return nil
}

```

// === FILE: references/go/src/compress/zlib/writer.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package zlib

import (
	"compress/flate"
	"encoding/binary"
	"fmt"
	"hash"
	"hash/adler32"
	"io"
)

// These constants are copied from the [flate] package, so that code that imports
// [compress/zlib] does not also have to import [compress/flate].
const (
	NoCompression      = flate.NoCompression
	BestSpeed          = flate.BestSpeed
	BestCompression    = flate.BestCompression
	DefaultCompression = flate.DefaultCompression
	HuffmanOnly        = flate.HuffmanOnly
)

// A Writer takes data written to it and writes the compressed
// form of that data to an underlying writer (see [NewWriter]).
type Writer struct {
	w           io.Writer
	level       int
	dict        []byte
	compressor  *flate.Writer
	digest      hash.Hash32
	err         error
	scratch     [4]byte
	wroteHeader bool
}

// NewWriter creates a new [Writer].
// Writes to the returned Writer are compressed and written to w.
//
// It is the caller's responsibility to call Close on the Writer when done.
// Writes may be buffered and not flushed until Close.
//
// Note that the exact bytes written to w are not covered by the Go 1
// compatibility promise. Callers, including tests, should not depend on the
// exact written bytes.
func NewWriter(w io.Writer) *Writer {
	z, _ := NewWriterLevelDict(w, DefaultCompression, nil)
	return z
}

// NewWriterLevel is like [NewWriter] but specifies the compression level instead
// of assuming [DefaultCompression].
//
// The compression level can be [DefaultCompression], [NoCompression], [HuffmanOnly]
// or any integer value between [BestSpeed] and [BestCompression] inclusive.
// The error returned will be nil if the level is valid.
//
// Note that the exact bytes written to w are not covered by the Go 1
// compatibility promise. Callers, including tests, should not depend on the
// exact written bytes.
func NewWriterLevel(w io.Writer, level int) (*Writer, error) {
	return NewWriterLevelDict(w, level, nil)
}

// NewWriterLevelDict is like [NewWriterLevel] but specifies a dictionary to
// compress with.
//
// The dictionary may be nil. If not, its contents should not be modified until
// the Writer is closed.
//
// Note that the exact bytes written to w are not covered by the Go 1
// compatibility promise. Callers, including tests, should not depend on the
// exact written bytes.
func NewWriterLevelDict(w io.Writer, level int, dict []byte) (*Writer, error) {
	if level < HuffmanOnly || level > BestCompression {
		return nil, fmt.Errorf("zlib: invalid compression level: %d", level)
	}
	return &Writer{
		w:     w,
		level: level,
		dict:  dict,
	}, nil
}

// Reset clears the state of the [Writer] z such that it is equivalent to its
// initial state from [NewWriterLevel] or [NewWriterLevelDict], but instead writing
// to w.
func (z *Writer) Reset(w io.Writer) {
	z.w = w
	// z.level and z.dict left unchanged.
	if z.compressor != nil {
		z.compressor.Reset(w)
	}
	if z.digest != nil {
		z.digest.Reset()
	}
	z.err = nil
	z.scratch = [4]byte{}
	z.wroteHeader = false
}

// writeHeader writes the ZLIB header.
func (z *Writer) writeHeader() (err error) {
	z.wroteHeader = true
	// ZLIB has a two-byte header (as documented in RFC 1950).
	// The first four bits is the CINFO (compression info), which is 7 for the default deflate window size.
	// The next four bits is the CM (compression method), which is 8 for deflate.
	z.scratch[0] = 0x78
	// The next two bits is the FLEVEL (compression level). The four values are:
	// 0=fastest, 1=fast, 2=default, 3=best.
	// The next bit, FDICT, is set if a dictionary is given.
	// The final five FCHECK bits form a mod-31 checksum.
	switch z.level {
	case -2, 0, 1:
		z.scratch[1] = 0 << 6
	case 2, 3, 4, 5:
		z.scratch[1] = 1 << 6
	case 6, -1:
		z.scratch[1] = 2 << 6
	case 7, 8, 9:
		z.scratch[1] = 3 << 6
	default:
		panic("unreachable")
	}
	if z.dict != nil {
		z.scratch[1] |= 1 << 5
	}
	z.scratch[1] += uint8(31 - binary.BigEndian.Uint16(z.scratch[:2])%31)
	if _, err = z.w.Write(z.scratch[0:2]); err != nil {
		return err
	}
	if z.dict != nil {
		// The next four bytes are the Adler-32 checksum of the dictionary.
		binary.BigEndian.PutUint32(z.scratch[:], adler32.Checksum(z.dict))
		if _, err = z.w.Write(z.scratch[0:4]); err != nil {
			return err
		}
	}
	if z.compressor == nil {
		// Initialize deflater unless the Writer is being reused
		// after a Reset call.
		z.compressor, err = flate.NewWriterDict(z.w, z.level, z.dict)
		if err != nil {
			return err
		}
		z.digest = adler32.New()
	}
	return nil
}

// Write writes a compressed form of p to the underlying [io.Writer]. The
// compressed bytes are not necessarily flushed until the [Writer] is closed or
// explicitly flushed.
func (z *Writer) Write(p []byte) (n int, err error) {
	if !z.wroteHeader {
		z.err = z.writeHeader()
	}
	if z.err != nil {
		return 0, z.err
	}
	if len(p) == 0 {
		return 0, nil
	}
	n, err = z.compressor.Write(p)
	if err != nil {
		z.err = err
		return
	}
	z.digest.Write(p)
	return
}

// Flush flushes the Writer to its underlying [io.Writer].
func (z *Writer) Flush() error {
	if !z.wroteHeader {
		z.err = z.writeHeader()
	}
	if z.err != nil {
		return z.err
	}
	z.err = z.compressor.Flush()
	return z.err
}

// Close closes the Writer, flushing any unwritten data to the underlying
// [io.Writer], but does not close the underlying io.Writer.
func (z *Writer) Close() error {
	if !z.wroteHeader {
		z.err = z.writeHeader()
	}
	if z.err != nil {
		return z.err
	}
	z.err = z.compressor.Close()
	if z.err != nil {
		return z.err
	}
	checksum := z.digest.Sum32()
	// ZLIB (RFC 1950) is big-endian, unlike GZIP (RFC 1952).
	binary.BigEndian.PutUint32(z.scratch[:], checksum)
	_, z.err = z.w.Write(z.scratch[0:4])
	return z.err
}

```

