# Domain Architecture: hash

## Layout Topology
```text
hash/
├── adler32
│   └── adler32.go
├── crc32
│   ├── crc32.go
│   ├── crc32_amd64.go
│   ├── crc32_amd64.s
│   ├── crc32_arm64.go
│   ├── crc32_arm64.s
│   ├── crc32_generic.go
│   ├── crc32_loong64.go
│   ├── crc32_loong64.s
│   ├── crc32_otherarch.go
│   ├── crc32_ppc64le.go
│   ├── crc32_ppc64le.s
│   ├── crc32_s390x.go
│   ├── crc32_s390x.s
│   ├── crc32_table_ppc64le.s
│   ├── gen.go
│   └── gen_const_ppc64le.go
├── crc64
│   └── crc64.go
├── fnv
│   └── fnv.go
├── maphash
│   ├── hasher.go
│   └── maphash.go
├── hash.go
├── test_cases.txt
└── test_gen.awk
```

## Source Stream Aggregation

// === FILE: references/go/src/hash/adler32/adler32.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package adler32 implements the Adler-32 checksum.
//
// It is defined in RFC 1950:
//
//	Adler-32 is composed of two sums accumulated per byte: s1 is
//	the sum of all bytes, s2 is the sum of all s1 values. Both sums
//	are done modulo 65521. s1 is initialized to 1, s2 to zero.  The
//	Adler-32 checksum is stored as s2*65536 + s1 in most-
//	significant-byte first (network) order.
package adler32

import (
	"errors"
	"hash"
	"internal/byteorder"
)

const (
	// mod is the largest prime that is less than 65536.
	mod = 65521
	// nmax is the largest n such that
	// 255 * n * (n+1) / 2 + (n+1) * (mod-1) <= 2^32-1.
	// It is mentioned in RFC 1950 (search for "5552").
	nmax = 5552
)

// The size of an Adler-32 checksum in bytes.
const Size = 4

// digest represents the partial evaluation of a checksum.
// The low 16 bits are s1, the high 16 bits are s2.
type digest uint32

func (d *digest) Reset() { *d = 1 }

// New returns a new hash.Hash32 computing the Adler-32 checksum. Its
// Sum method will lay the value out in big-endian byte order. The
// returned Hash32 also implements [encoding.BinaryMarshaler] and
// [encoding.BinaryUnmarshaler] to marshal and unmarshal the internal
// state of the hash.
func New() hash.Hash32 {
	d := new(digest)
	d.Reset()
	return d
}

func (d *digest) Size() int { return Size }

func (d *digest) BlockSize() int { return 4 }

const (
	magic         = "adl\x01"
	marshaledSize = len(magic) + 4
)

func (d *digest) AppendBinary(b []byte) ([]byte, error) {
	b = append(b, magic...)
	b = byteorder.BEAppendUint32(b, uint32(*d))
	return b, nil
}

func (d *digest) MarshalBinary() ([]byte, error) {
	return d.AppendBinary(make([]byte, 0, marshaledSize))
}

func (d *digest) UnmarshalBinary(b []byte) error {
	if len(b) < len(magic) || string(b[:len(magic)]) != magic {
		return errors.New("hash/adler32: invalid hash state identifier")
	}
	if len(b) != marshaledSize {
		return errors.New("hash/adler32: invalid hash state size")
	}
	*d = digest(byteorder.BEUint32(b[len(magic):]))
	return nil
}

func (d *digest) Clone() (hash.Cloner, error) {
	r := *d
	return &r, nil
}

// Add p to the running checksum d.
func update(d digest, p []byte) digest {
	s1, s2 := uint32(d&0xffff), uint32(d>>16)
	for len(p) > 0 {
		var q []byte
		if len(p) > nmax {
			p, q = p[:nmax], p[nmax:]
		}
		for len(p) >= 4 {
			s1 += uint32(p[0])
			s2 += s1
			s1 += uint32(p[1])
			s2 += s1
			s1 += uint32(p[2])
			s2 += s1
			s1 += uint32(p[3])
			s2 += s1
			p = p[4:]
		}
		for _, x := range p {
			s1 += uint32(x)
			s2 += s1
		}
		s1 %= mod
		s2 %= mod
		p = q
	}
	return digest(s2<<16 | s1)
}

func (d *digest) Write(p []byte) (nn int, err error) {
	*d = update(*d, p)
	return len(p), nil
}

func (d *digest) Sum32() uint32 { return uint32(*d) }

func (d *digest) Sum(in []byte) []byte {
	s := uint32(*d)
	return append(in, byte(s>>24), byte(s>>16), byte(s>>8), byte(s))
}

// Checksum returns the Adler-32 checksum of data.
func Checksum(data []byte) uint32 { return uint32(update(1, data)) }

```

// === FILE: references/go/src/hash/crc32/crc32.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package crc32 implements the 32-bit cyclic redundancy check, or CRC-32,
// checksum. See https://en.wikipedia.org/wiki/Cyclic_redundancy_check for
// information.
//
// Polynomials are represented in LSB-first form also known as reversed representation.
//
// See https://en.wikipedia.org/wiki/Mathematics_of_cyclic_redundancy_checks#Reversed_representations_and_reciprocal_polynomials
// for information.
package crc32

import (
	"errors"
	"hash"
	"internal/byteorder"
	"sync"
	"sync/atomic"
)

// The size of a CRC-32 checksum in bytes.
const Size = 4

// Predefined polynomials.
const (
	// IEEE is by far and away the most common CRC-32 polynomial.
	// Used by ethernet (IEEE 802.3), v.42, fddi, gzip, zip, png, ...
	IEEE = 0xedb88320

	// Castagnoli's polynomial, used in iSCSI.
	// Has better error detection characteristics than IEEE.
	// https://dx.doi.org/10.1109/26.231911
	Castagnoli = 0x82f63b78

	// Koopman's polynomial.
	// Also has better error detection characteristics than IEEE.
	// https://dx.doi.org/10.1109/DSN.2002.1028931
	Koopman = 0xeb31d82e
)

// Table is a 256-word table representing the polynomial for efficient processing.
type Table [256]uint32

// This file makes use of functions implemented in architecture-specific files.
// The interface that they implement is as follows:
//
//    // archAvailableIEEE reports whether an architecture-specific CRC32-IEEE
//    // algorithm is available.
//    archAvailableIEEE() bool
//
//    // archInitIEEE initializes the architecture-specific CRC3-IEEE algorithm.
//    // It can only be called if archAvailableIEEE() returns true.
//    archInitIEEE()
//
//    // archUpdateIEEE updates the given CRC32-IEEE. It can only be called if
//    // archInitIEEE() was previously called.
//    archUpdateIEEE(crc uint32, p []byte) uint32
//
//    // archAvailableCastagnoli reports whether an architecture-specific
//    // CRC32-C algorithm is available.
//    archAvailableCastagnoli() bool
//
//    // archInitCastagnoli initializes the architecture-specific CRC32-C
//    // algorithm. It can only be called if archAvailableCastagnoli() returns
//    // true.
//    archInitCastagnoli()
//
//    // archUpdateCastagnoli updates the given CRC32-C. It can only be called
//    // if archInitCastagnoli() was previously called.
//    archUpdateCastagnoli(crc uint32, p []byte) uint32

// castagnoliTable points to a lazily initialized Table for the Castagnoli
// polynomial. MakeTable will always return this value when asked to make a
// Castagnoli table so we can compare against it to find when the caller is
// using this polynomial.
var castagnoliTable *Table
var castagnoliTable8 *slicing8Table
var updateCastagnoli func(crc uint32, p []byte) uint32
var haveCastagnoli atomic.Bool

var castagnoliInitOnce = sync.OnceFunc(func() {
	castagnoliTable = simpleMakeTable(Castagnoli)

	if archAvailableCastagnoli() {
		archInitCastagnoli()
		updateCastagnoli = archUpdateCastagnoli
	} else {
		// Initialize the slicing-by-8 table.
		castagnoliTable8 = slicingMakeTable(Castagnoli)
		updateCastagnoli = func(crc uint32, p []byte) uint32 {
			return slicingUpdate(crc, castagnoliTable8, p)
		}
	}

	haveCastagnoli.Store(true)
})

// IEEETable is the table for the [IEEE] polynomial.
var IEEETable = simpleMakeTable(IEEE)

// ieeeTable8 is the slicing8Table for IEEE
var ieeeTable8 *slicing8Table
var updateIEEE func(crc uint32, p []byte) uint32

var ieeeInitOnce = sync.OnceFunc(func() {
	if archAvailableIEEE() {
		archInitIEEE()
		updateIEEE = archUpdateIEEE
	} else {
		// Initialize the slicing-by-8 table.
		ieeeTable8 = slicingMakeTable(IEEE)
		updateIEEE = func(crc uint32, p []byte) uint32 {
			return slicingUpdate(crc, ieeeTable8, p)
		}
	}
})

// MakeTable returns a [Table] constructed from the specified polynomial.
// The contents of this [Table] must not be modified.
func MakeTable(poly uint32) *Table {
	switch poly {
	case IEEE:
		ieeeInitOnce()
		return IEEETable
	case Castagnoli:
		castagnoliInitOnce()
		return castagnoliTable
	default:
		return simpleMakeTable(poly)
	}
}

// digest represents the partial evaluation of a checksum.
type digest struct {
	crc uint32
	tab *Table
}

// New creates a new [hash.Hash32] computing the CRC-32 checksum using the
// polynomial represented by the [Table]. Its Sum method will lay the
// value out in big-endian byte order. The returned Hash32 also
// implements [encoding.BinaryMarshaler] and [encoding.BinaryUnmarshaler] to
// marshal and unmarshal the internal state of the hash.
func New(tab *Table) hash.Hash32 {
	if tab == IEEETable {
		ieeeInitOnce()
	}
	return &digest{0, tab}
}

// NewIEEE creates a new [hash.Hash32] computing the CRC-32 checksum using
// the [IEEE] polynomial. Its Sum method will lay the value out in
// big-endian byte order. The returned Hash32 also implements
// [encoding.BinaryMarshaler] and [encoding.BinaryUnmarshaler] to marshal
// and unmarshal the internal state of the hash.
func NewIEEE() hash.Hash32 { return New(IEEETable) }

func (d *digest) Size() int { return Size }

func (d *digest) BlockSize() int { return 1 }

func (d *digest) Reset() { d.crc = 0 }

const (
	magic         = "crc\x01"
	marshaledSize = len(magic) + 4 + 4
)

func (d *digest) AppendBinary(b []byte) ([]byte, error) {
	b = append(b, magic...)
	b = byteorder.BEAppendUint32(b, tableSum(d.tab))
	b = byteorder.BEAppendUint32(b, d.crc)
	return b, nil
}

func (d *digest) MarshalBinary() ([]byte, error) {
	return d.AppendBinary(make([]byte, 0, marshaledSize))

}

func (d *digest) UnmarshalBinary(b []byte) error {
	if len(b) < len(magic) || string(b[:len(magic)]) != magic {
		return errors.New("hash/crc32: invalid hash state identifier")
	}
	if len(b) != marshaledSize {
		return errors.New("hash/crc32: invalid hash state size")
	}
	if tableSum(d.tab) != byteorder.BEUint32(b[4:]) {
		return errors.New("hash/crc32: tables do not match")
	}
	d.crc = byteorder.BEUint32(b[8:])
	return nil
}

func (d *digest) Clone() (hash.Cloner, error) {
	r := *d
	return &r, nil
}

func update(crc uint32, tab *Table, p []byte, checkInitIEEE bool) uint32 {
	switch {
	case haveCastagnoli.Load() && tab == castagnoliTable:
		return updateCastagnoli(crc, p)
	case tab == IEEETable:
		if checkInitIEEE {
			ieeeInitOnce()
		}
		return updateIEEE(crc, p)
	default:
		return simpleUpdate(crc, tab, p)
	}
}

// Update returns the result of adding the bytes in p to the crc.
func Update(crc uint32, tab *Table, p []byte) uint32 {
	// Unfortunately, because IEEETable is exported, IEEE may be used without a
	// call to MakeTable. We have to make sure it gets initialized in that case.
	return update(crc, tab, p, true)
}

func (d *digest) Write(p []byte) (n int, err error) {
	// We only create digest objects through New() which takes care of
	// initialization in this case.
	d.crc = update(d.crc, d.tab, p, false)
	return len(p), nil
}

func (d *digest) Sum32() uint32 { return d.crc }

func (d *digest) Sum(in []byte) []byte {
	s := d.Sum32()
	return append(in, byte(s>>24), byte(s>>16), byte(s>>8), byte(s))
}

// Checksum returns the CRC-32 checksum of data
// using the polynomial represented by the [Table].
func Checksum(data []byte, tab *Table) uint32 { return Update(0, tab, data) }

// ChecksumIEEE returns the CRC-32 checksum of data
// using the [IEEE] polynomial.
func ChecksumIEEE(data []byte) uint32 {
	ieeeInitOnce()
	return updateIEEE(0, data)
}

// tableSum returns the IEEE checksum of table t.
func tableSum(t *Table) uint32 {
	var a [1024]byte
	b := a[:0]
	if t != nil {
		for _, x := range t {
			b = byteorder.BEAppendUint32(b, x)
		}
	}
	return ChecksumIEEE(b)
}

```

// === FILE: references/go/src/hash/crc32/crc32_amd64.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// AMD64-specific hardware-assisted CRC32 algorithms. See crc32.go for a
// description of the interface that each architecture-specific file
// implements.

package crc32

import (
	"internal/cpu"
	"unsafe"
)

// Offset into internal/cpu records for use in assembly.
const (
	offsetX86HasAVX512VPCLMULQDQL = unsafe.Offsetof(cpu.X86.HasAVX512VPCLMULQDQ)
)

// This file contains the code to call the SSE 4.2 version of the Castagnoli
// and IEEE CRC.

// castagnoliSSE42 is defined in crc32_amd64.s and uses the SSE 4.2 CRC32
// instruction.
//
//go:noescape
func castagnoliSSE42(crc uint32, p []byte) uint32

// castagnoliSSE42Triple is defined in crc32_amd64.s and uses the SSE 4.2 CRC32
// instruction.
//
//go:noescape
func castagnoliSSE42Triple(
	crcA, crcB, crcC uint32,
	a, b, c []byte,
	rounds uint32,
) (retA uint32, retB uint32, retC uint32)

// ieeeCLMUL is defined in crc_amd64.s and uses the PCLMULQDQ
// instruction as well as SSE 4.1.
//
//go:noescape
func ieeeCLMUL(crc uint32, p []byte) uint32

const castagnoliK1 = 168
const castagnoliK2 = 1344

type sse42Table [4]Table

var castagnoliSSE42TableK1 *sse42Table
var castagnoliSSE42TableK2 *sse42Table

func archAvailableCastagnoli() bool {
	return cpu.X86.HasSSE42
}

func archInitCastagnoli() {
	if !cpu.X86.HasSSE42 {
		panic("arch-specific Castagnoli not available")
	}
	castagnoliSSE42TableK1 = new(sse42Table)
	castagnoliSSE42TableK2 = new(sse42Table)
	// See description in updateCastagnoli.
	//    t[0][i] = CRC(i000, O)
	//    t[1][i] = CRC(0i00, O)
	//    t[2][i] = CRC(00i0, O)
	//    t[3][i] = CRC(000i, O)
	// where O is a sequence of K zeros.
	var tmp [castagnoliK2]byte
	for b := 0; b < 4; b++ {
		for i := 0; i < 256; i++ {
			val := uint32(i) << uint32(b*8)
			castagnoliSSE42TableK1[b][i] = castagnoliSSE42(val, tmp[:castagnoliK1])
			castagnoliSSE42TableK2[b][i] = castagnoliSSE42(val, tmp[:])
		}
	}
}

// castagnoliShift computes the CRC32-C of K1 or K2 zeroes (depending on the
// table given) with the given initial crc value. This corresponds to
// CRC(crc, O) in the description in updateCastagnoli.
func castagnoliShift(table *sse42Table, crc uint32) uint32 {
	return table[3][crc>>24] ^
		table[2][(crc>>16)&0xFF] ^
		table[1][(crc>>8)&0xFF] ^
		table[0][crc&0xFF]
}

func archUpdateCastagnoli(crc uint32, p []byte) uint32 {
	if !cpu.X86.HasSSE42 {
		panic("not available")
	}

	// This method is inspired from the algorithm in Intel's white paper:
	//    "Fast CRC Computation for iSCSI Polynomial Using CRC32 Instruction"
	// The same strategy of splitting the buffer in three is used but the
	// combining calculation is different; the complete derivation is explained
	// below.
	//
	// -- The basic idea --
	//
	// The CRC32 instruction (available in SSE4.2) can process 8 bytes at a
	// time. In recent Intel architectures the instruction takes 3 cycles;
	// however the processor can pipeline up to three instructions if they
	// don't depend on each other.
	//
	// Roughly this means that we can process three buffers in about the same
	// time we can process one buffer.
	//
	// The idea is then to split the buffer in three, CRC the three pieces
	// separately and then combine the results.
	//
	// Combining the results requires precomputed tables, so we must choose a
	// fixed buffer length to optimize. The longer the length, the faster; but
	// only buffers longer than this length will use the optimization. We choose
	// two cutoffs and compute tables for both:
	//  - one around 512: 168*3=504
	//  - one around 4KB: 1344*3=4032
	//
	// -- The nitty gritty --
	//
	// Let CRC(I, X) be the non-inverted CRC32-C of the sequence X (with
	// initial non-inverted CRC I). This function has the following properties:
	//   (a) CRC(I, AB) = CRC(CRC(I, A), B)
	//   (b) CRC(I, A xor B) = CRC(I, A) xor CRC(0, B)
	//
	// Say we want to compute CRC(I, ABC) where A, B, C are three sequences of
	// K bytes each, where K is a fixed constant. Let O be the sequence of K zero
	// bytes.
	//
	// CRC(I, ABC) = CRC(I, ABO xor C)
	//             = CRC(I, ABO) xor CRC(0, C)
	//             = CRC(CRC(I, AB), O) xor CRC(0, C)
	//             = CRC(CRC(I, AO xor B), O) xor CRC(0, C)
	//             = CRC(CRC(I, AO) xor CRC(0, B), O) xor CRC(0, C)
	//             = CRC(CRC(CRC(I, A), O) xor CRC(0, B), O) xor CRC(0, C)
	//
	// The castagnoliSSE42Triple function can compute CRC(I, A), CRC(0, B),
	// and CRC(0, C) efficiently.  We just need to find a way to quickly compute
	// CRC(uvwx, O) given a 4-byte initial value uvwx. We can precompute these
	// values; since we can't have a 32-bit table, we break it up into four
	// 8-bit tables:
	//
	//    CRC(uvwx, O) = CRC(u000, O) xor
	//                   CRC(0v00, O) xor
	//                   CRC(00w0, O) xor
	//                   CRC(000x, O)
	//
	// We can compute tables corresponding to the four terms for all 8-bit
	// values.

	crc = ^crc

	// If a buffer is long enough to use the optimization, process the first few
	// bytes to align the buffer to an 8 byte boundary (if necessary).
	if len(p) >= castagnoliK1*3 {
		delta := int(uintptr(unsafe.Pointer(&p[0])) & 7)
		if delta != 0 {
			delta = 8 - delta
			crc = castagnoliSSE42(crc, p[:delta])
			p = p[delta:]
		}
	}

	// Process 3*K2 at a time.
	for len(p) >= castagnoliK2*3 {
		// Compute CRC(I, A), CRC(0, B), and CRC(0, C).
		crcA, crcB, crcC := castagnoliSSE42Triple(
			crc, 0, 0,
			p, p[castagnoliK2:], p[castagnoliK2*2:],
			castagnoliK2/24)

		// CRC(I, AB) = CRC(CRC(I, A), O) xor CRC(0, B)
		crcAB := castagnoliShift(castagnoliSSE42TableK2, crcA) ^ crcB
		// CRC(I, ABC) = CRC(CRC(I, AB), O) xor CRC(0, C)
		crc = castagnoliShift(castagnoliSSE42TableK2, crcAB) ^ crcC
		p = p[castagnoliK2*3:]
	}

	// Process 3*K1 at a time.
	for len(p) >= castagnoliK1*3 {
		// Compute CRC(I, A), CRC(0, B), and CRC(0, C).
		crcA, crcB, crcC := castagnoliSSE42Triple(
			crc, 0, 0,
			p, p[castagnoliK1:], p[castagnoliK1*2:],
			castagnoliK1/24)

		// CRC(I, AB) = CRC(CRC(I, A), O) xor CRC(0, B)
		crcAB := castagnoliShift(castagnoliSSE42TableK1, crcA) ^ crcB
		// CRC(I, ABC) = CRC(CRC(I, AB), O) xor CRC(0, C)
		crc = castagnoliShift(castagnoliSSE42TableK1, crcAB) ^ crcC
		p = p[castagnoliK1*3:]
	}

	// Use the simple implementation for what's left.
	crc = castagnoliSSE42(crc, p)
	return ^crc
}

func archAvailableIEEE() bool {
	return cpu.X86.HasPCLMULQDQ && cpu.X86.HasSSE41
}

var archIeeeTable8 *slicing8Table

func archInitIEEE() {
	if !cpu.X86.HasPCLMULQDQ || !cpu.X86.HasSSE41 {
		panic("not available")
	}
	// We still use slicing-by-8 for small buffers.
	archIeeeTable8 = slicingMakeTable(IEEE)
}

func archUpdateIEEE(crc uint32, p []byte) uint32 {
	if !cpu.X86.HasPCLMULQDQ || !cpu.X86.HasSSE41 {
		panic("not available")
	}

	if len(p) >= 64 {
		left := len(p) & 15
		do := len(p) - left
		crc = ^ieeeCLMUL(^crc, p[:do])
		p = p[do:]
	}
	if len(p) == 0 {
		return crc
	}
	return slicingUpdate(crc, archIeeeTable8, p)
}

```

// === FILE: references/go/src/hash/crc32/crc32_amd64.s ===
```text
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"
#include "go_asm.h"

// castagnoliSSE42 updates the (non-inverted) crc with the given buffer.
//
// func castagnoliSSE42(crc uint32, p []byte) uint32
TEXT ·castagnoliSSE42(SB),NOSPLIT,$0
	MOVL crc+0(FP), AX  // CRC value
	MOVQ p+8(FP), SI  // data pointer
	MOVQ p_len+16(FP), CX  // len(p)

	// If there are fewer than 8 bytes to process, skip alignment.
	CMPQ CX, $8
	JL less_than_8

	MOVQ SI, BX
	ANDQ $7, BX
	JZ aligned

	// Process the first few bytes to 8-byte align the input.

	// BX = 8 - BX. We need to process this many bytes to align.
	SUBQ $1, BX
	XORQ $7, BX

	BTQ $0, BX
	JNC align_2

	CRC32B (SI), AX
	DECQ CX
	INCQ SI

align_2:
	BTQ $1, BX
	JNC align_4

	CRC32W (SI), AX

	SUBQ $2, CX
	ADDQ $2, SI

align_4:
	BTQ $2, BX
	JNC aligned

	CRC32L (SI), AX

	SUBQ $4, CX
	ADDQ $4, SI

aligned:
	// The input is now 8-byte aligned and we can process 8-byte chunks.
	CMPQ CX, $8
	JL less_than_8

	CRC32Q (SI), AX
	ADDQ $8, SI
	SUBQ $8, CX
	JMP aligned

less_than_8:
	// We may have some bytes left over; process 4 bytes, then 2, then 1.
	BTQ $2, CX
	JNC less_than_4

	CRC32L (SI), AX
	ADDQ $4, SI

less_than_4:
	BTQ $1, CX
	JNC less_than_2

	CRC32W (SI), AX
	ADDQ $2, SI

less_than_2:
	BTQ $0, CX
	JNC done

	CRC32B (SI), AX

done:
	MOVL AX, ret+32(FP)
	RET

// castagnoliSSE42Triple updates three (non-inverted) crcs with (24*rounds)
// bytes from each buffer.
//
// func castagnoliSSE42Triple(
//     crc1, crc2, crc3 uint32,
//     a, b, c []byte,
//     rounds uint32,
// ) (retA uint32, retB uint32, retC uint32)
TEXT ·castagnoliSSE42Triple(SB),NOSPLIT,$0
	MOVL crcA+0(FP), AX
	MOVL crcB+4(FP), CX
	MOVL crcC+8(FP), DX

	MOVQ a+16(FP), R8   // data pointer
	MOVQ b+40(FP), R9   // data pointer
	MOVQ c+64(FP), R10  // data pointer

	MOVL rounds+88(FP), R11

loop:
	CRC32Q (R8), AX
	CRC32Q (R9), CX
	CRC32Q (R10), DX

	CRC32Q 8(R8), AX
	CRC32Q 8(R9), CX
	CRC32Q 8(R10), DX

	CRC32Q 16(R8), AX
	CRC32Q 16(R9), CX
	CRC32Q 16(R10), DX

	ADDQ $24, R8
	ADDQ $24, R9
	ADDQ $24, R10

	DECQ R11
	JNZ loop

	MOVL AX, retA+96(FP)
	MOVL CX, retB+100(FP)
	MOVL DX, retC+104(FP)
	RET

// CRC32 polynomial data
//
// These constants are lifted from the
// Linux kernel, since they avoid the costly
// PSHUFB 16 byte reversal proposed in the
// original Intel paper.
// Splatted so it can be loaded with a single VMOVDQU64
DATA r2r1<>+0(SB)/8, $0x154442bd4
DATA r2r1<>+8(SB)/8, $0x1c6e41596
DATA r2r1<>+16(SB)/8, $0x154442bd4
DATA r2r1<>+24(SB)/8, $0x1c6e41596
DATA r2r1<>+32(SB)/8, $0x154442bd4
DATA r2r1<>+40(SB)/8, $0x1c6e41596
DATA r2r1<>+48(SB)/8, $0x154442bd4
DATA r2r1<>+56(SB)/8, $0x1c6e41596

DATA r4r3<>+0(SB)/8, $0x1751997d0
DATA r4r3<>+8(SB)/8, $0x0ccaa009e
DATA rupoly<>+0(SB)/8, $0x1db710641
DATA rupoly<>+8(SB)/8, $0x1f7011641
DATA r5<>+0(SB)/8, $0x163cd6124

GLOBL r2r1<>(SB), RODATA, $64
GLOBL r4r3<>(SB),RODATA,$16
GLOBL rupoly<>(SB),RODATA,$16
GLOBL r5<>(SB),RODATA,$8

// Based on https://www.intel.com/content/dam/www/public/us/en/documents/white-papers/fast-crc-computation-generic-polynomials-pclmulqdq-paper.pdf
// len(p) must be at least 64, and must be a multiple of 16.

// func ieeeCLMUL(crc uint32, p []byte) uint32
TEXT ·ieeeCLMUL(SB),NOSPLIT,$0
	MOVL   crc+0(FP), X0             // Initial CRC value
	MOVQ   p+8(FP), SI  	         // data pointer
	MOVQ   p_len+16(FP), CX          // len(p)

	// Check feature support and length to be >= 1024 bytes.
	CMPB internal∕cpu·X86+const_offsetX86HasAVX512VPCLMULQDQL(SB), $1
	JNE  useSSE42
	CMPQ CX, $1024
	JL   useSSE42

	// Use AVX512. Zero upper and Z10 and load initial CRC into lower part of Z10.
	VPXORQ    Z10, Z10, Z10
	VMOVAPS   X0, X10
	VMOVDQU64 (SI), Z1
	VPXORQ    Z10, Z1, Z1 // Merge initial CRC value into Z1
	ADDQ      $64, SI    // buf+=64
	SUBQ      $64, CX    // len-=64

	VMOVDQU64 r2r1<>+0(SB), Z0

loopback64Avx512:
	VMOVDQU64  (SI), Z11          // Load next
	VPCLMULQDQ $0x11, Z0, Z1, Z5
	VPCLMULQDQ $0, Z0, Z1, Z1
	VPTERNLOGD $0x96, Z11, Z5, Z1 // Combine results with xor into Z1

	ADDQ $0x40, DI
	ADDQ $64, SI    // buf+=64
	SUBQ $64, CX    // len-=64
	CMPQ CX, $64    // Less than 64 bytes left?
	JGE  loopback64Avx512

	// Unfold result into XMM1-XMM4 to match SSE4 code.
	VEXTRACTF32X4 $1, Z1, X2 // X2: Second 128-bit lane
	VEXTRACTF32X4 $2, Z1, X3 // X3: Third 128-bit lane
	VEXTRACTF32X4 $3, Z1, X4 // X4: Fourth 128-bit lane
	VZEROUPPER
	JMP remain64

	PCALIGN $16
useSSE42:
	MOVOU  (SI), X1
	MOVOU  16(SI), X2
	MOVOU  32(SI), X3
	MOVOU  48(SI), X4
	PXOR   X0, X1
	ADDQ   $64, SI                  // buf+=64
	SUBQ   $64, CX                  // len-=64
	CMPQ   CX, $64                  // Less than 64 bytes left
	JB     remain64

	MOVOA  r2r1<>+0(SB), X0
loopback64:
	MOVOA  X1, X5
	MOVOA  X2, X6
	MOVOA  X3, X7
	MOVOA  X4, X8

	PCLMULQDQ $0, X0, X1
	PCLMULQDQ $0, X0, X2
	PCLMULQDQ $0, X0, X3
	PCLMULQDQ $0, X0, X4

	/* Load next early */
	MOVOU    (SI), X11
	MOVOU    16(SI), X12
	MOVOU    32(SI), X13
	MOVOU    48(SI), X14

	PCLMULQDQ $0x11, X0, X5
	PCLMULQDQ $0x11, X0, X6
	PCLMULQDQ $0x11, X0, X7
	PCLMULQDQ $0x11, X0, X8

	PXOR     X5, X1
	PXOR     X6, X2
	PXOR     X7, X3
	PXOR     X8, X4

	PXOR     X11, X1
	PXOR     X12, X2
	PXOR     X13, X3
	PXOR     X14, X4

	ADDQ    $0x40, DI
	ADDQ    $64, SI      // buf+=64
	SUBQ    $64, CX      // len-=64
	CMPQ    CX, $64      // Less than 64 bytes left?
	JGE     loopback64

	PCALIGN $16
	/* Fold result into a single register (X1) */
remain64:
	MOVOA       r4r3<>+0(SB), X0

	MOVOA       X1, X5
	PCLMULQDQ   $0, X0, X1
	PCLMULQDQ   $0x11, X0, X5
	PXOR        X5, X1
	PXOR        X2, X1

	MOVOA       X1, X5
	PCLMULQDQ   $0, X0, X1
	PCLMULQDQ   $0x11, X0, X5
	PXOR        X5, X1
	PXOR        X3, X1

	MOVOA       X1, X5
	PCLMULQDQ   $0, X0, X1
	PCLMULQDQ   $0x11, X0, X5
	PXOR        X5, X1
	PXOR        X4, X1

	/* If there is less than 16 bytes left we are done */
	CMPQ        CX, $16
	JB          finish

	/* Encode 16 bytes */
remain16:
	MOVOU       (SI), X10
	MOVOA       X1, X5
	PCLMULQDQ   $0, X0, X1
	PCLMULQDQ   $0x11, X0, X5
	PXOR        X5, X1
	PXOR        X10, X1
	SUBQ        $16, CX
	ADDQ        $16, SI
	CMPQ        CX, $16
	JGE         remain16

finish:
	/* Fold final result into 32 bits and return it */
	PCMPEQB     X3, X3
	PCLMULQDQ   $1, X1, X0
	PSRLDQ      $8, X1
	PXOR        X0, X1

	MOVOA       X1, X2
	MOVQ        r5<>+0(SB), X0

	/* Creates 32 bit mask. Note that we don't care about upper half. */
	PSRLQ       $32, X3

	PSRLDQ      $4, X2
	PAND        X3, X1
	PCLMULQDQ   $0, X0, X1
	PXOR        X2, X1

	MOVOA       rupoly<>+0(SB), X0

	MOVOA       X1, X2
	PAND        X3, X1
	PCLMULQDQ   $0x10, X0, X1
	PAND        X3, X1
	PCLMULQDQ   $0, X0, X1
	PXOR        X2, X1

	PEXTRD	$1, X1, AX
	MOVL        AX, ret+32(FP)

	RET

```

// === FILE: references/go/src/hash/crc32/crc32_arm64.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// ARM64-specific hardware-assisted CRC32 algorithms. See crc32.go for a
// description of the interface that each architecture-specific file
// implements.

package crc32

import "internal/cpu"

func castagnoliUpdate(crc uint32, p []byte) uint32
func ieeeUpdate(crc uint32, p []byte) uint32

func archAvailableCastagnoli() bool {
	return cpu.ARM64.HasCRC32
}

func archInitCastagnoli() {
	if !cpu.ARM64.HasCRC32 {
		panic("arch-specific crc32 instruction for Castagnoli not available")
	}
}

func archUpdateCastagnoli(crc uint32, p []byte) uint32 {
	if !cpu.ARM64.HasCRC32 {
		panic("arch-specific crc32 instruction for Castagnoli not available")
	}

	return ^castagnoliUpdate(^crc, p)
}

func archAvailableIEEE() bool {
	return cpu.ARM64.HasCRC32
}

func archInitIEEE() {
	if !cpu.ARM64.HasCRC32 {
		panic("arch-specific crc32 instruction for IEEE not available")
	}
}

func archUpdateIEEE(crc uint32, p []byte) uint32 {
	if !cpu.ARM64.HasCRC32 {
		panic("arch-specific crc32 instruction for IEEE not available")
	}

	return ^ieeeUpdate(^crc, p)
}

```

// === FILE: references/go/src/hash/crc32/crc32_arm64.s ===
```text
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// castagnoliUpdate updates the non-inverted crc with the given data.

// func castagnoliUpdate(crc uint32, p []byte) uint32
TEXT ·castagnoliUpdate(SB),NOSPLIT,$0-36
	MOVWU	crc+0(FP), R9  // CRC value
	MOVD	p+8(FP), R13  // data pointer
	MOVD	p_len+16(FP), R11  // len(p)

update:
	CMP	$16, R11
	BLT	less_than_16
	LDP.P	16(R13), (R8, R10)
	CRC32CX	R8, R9
	CRC32CX	R10, R9
	SUB	$16, R11

	JMP	update

less_than_16:
	TBZ	$3, R11, less_than_8

	MOVD.P	8(R13), R10
	CRC32CX	R10, R9

less_than_8:
	TBZ	$2, R11, less_than_4

	MOVWU.P	4(R13), R10
	CRC32CW	R10, R9

less_than_4:
	TBZ	$1, R11, less_than_2

	MOVHU.P	2(R13), R10
	CRC32CH	R10, R9

less_than_2:
	TBZ	$0, R11, done

	MOVBU	(R13), R10
	CRC32CB	R10, R9

done:
	MOVWU	R9, ret+32(FP)
	RET

// ieeeUpdate updates the non-inverted crc with the given data.

// func ieeeUpdate(crc uint32, p []byte) uint32
TEXT ·ieeeUpdate(SB),NOSPLIT,$0-36
	MOVWU	crc+0(FP), R9  // CRC value
	MOVD	p+8(FP), R13  // data pointer
	MOVD	p_len+16(FP), R11  // len(p)

update:
	CMP	$16, R11
	BLT	less_than_16
	LDP.P	16(R13), (R8, R10)
	CRC32X	R8, R9
	CRC32X	R10, R9
	SUB	$16, R11

	JMP	update

less_than_16:
	TBZ $3, R11, less_than_8

	MOVD.P	8(R13), R10
	CRC32X	R10, R9

less_than_8:
	TBZ	$2, R11, less_than_4

	MOVWU.P	4(R13), R10
	CRC32W	R10, R9

less_than_4:
	TBZ	$1, R11, less_than_2

	MOVHU.P	2(R13), R10
	CRC32H	R10, R9

less_than_2:
	TBZ	$0, R11, done

	MOVBU	(R13), R10
	CRC32B	R10, R9

done:
	MOVWU	R9, ret+32(FP)
	RET

```

// === FILE: references/go/src/hash/crc32/crc32_generic.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains CRC32 algorithms that are not specific to any architecture
// and don't use hardware acceleration.
//
// The simple (and slow) CRC32 implementation only uses a 256*4 bytes table.
//
// The slicing-by-8 algorithm is a faster implementation that uses a bigger
// table (8*256*4 bytes).

package crc32

import "internal/byteorder"

// simpleMakeTable allocates and constructs a Table for the specified
// polynomial. The table is suitable for use with the simple algorithm
// (simpleUpdate).
func simpleMakeTable(poly uint32) *Table {
	t := new(Table)
	simplePopulateTable(poly, t)
	return t
}

// simplePopulateTable constructs a Table for the specified polynomial, suitable
// for use with simpleUpdate.
func simplePopulateTable(poly uint32, t *Table) {
	for i := 0; i < 256; i++ {
		crc := uint32(i)
		for j := 0; j < 8; j++ {
			if crc&1 == 1 {
				crc = (crc >> 1) ^ poly
			} else {
				crc >>= 1
			}
		}
		t[i] = crc
	}
}

// simpleUpdate uses the simple algorithm to update the CRC, given a table that
// was previously computed using simpleMakeTable.
func simpleUpdate(crc uint32, tab *Table, p []byte) uint32 {
	crc = ^crc
	for _, v := range p {
		crc = tab[byte(crc)^v] ^ (crc >> 8)
	}
	return ^crc
}

// Use slicing-by-8 when payload >= this value.
const slicing8Cutoff = 16

// slicing8Table is array of 8 Tables, used by the slicing-by-8 algorithm.
type slicing8Table [8]Table

// slicingMakeTable constructs a slicing8Table for the specified polynomial. The
// table is suitable for use with the slicing-by-8 algorithm (slicingUpdate).
func slicingMakeTable(poly uint32) *slicing8Table {
	t := new(slicing8Table)
	simplePopulateTable(poly, &t[0])
	for i := 0; i < 256; i++ {
		crc := t[0][i]
		for j := 1; j < 8; j++ {
			crc = t[0][crc&0xFF] ^ (crc >> 8)
			t[j][i] = crc
		}
	}
	return t
}

// slicingUpdate uses the slicing-by-8 algorithm to update the CRC, given a
// table that was previously computed using slicingMakeTable.
func slicingUpdate(crc uint32, tab *slicing8Table, p []byte) uint32 {
	if len(p) >= slicing8Cutoff {
		crc = ^crc
		for len(p) > 8 {
			crc ^= byteorder.LEUint32(p)
			crc = tab[0][p[7]] ^ tab[1][p[6]] ^ tab[2][p[5]] ^ tab[3][p[4]] ^
				tab[4][crc>>24] ^ tab[5][(crc>>16)&0xFF] ^
				tab[6][(crc>>8)&0xFF] ^ tab[7][crc&0xFF]
			p = p[8:]
		}
		crc = ^crc
	}
	if len(p) == 0 {
		return crc
	}
	return simpleUpdate(crc, &tab[0], p)
}

```

// === FILE: references/go/src/hash/crc32/crc32_loong64.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// LoongArch64-specific hardware-assisted CRC32 algorithms. See crc32.go for a
// description of the interface that each architecture-specific file
// implements.

package crc32

import "internal/cpu"

func castagnoliUpdate(crc uint32, p []byte) uint32
func ieeeUpdate(crc uint32, p []byte) uint32

func archAvailableCastagnoli() bool {
	return cpu.Loong64.HasCRC32
}

func archInitCastagnoli() {
	if !cpu.Loong64.HasCRC32 {
		panic("arch-specific crc32 instruction for Castagnoli not available")
	}
}

func archUpdateCastagnoli(crc uint32, p []byte) uint32 {
	if !cpu.Loong64.HasCRC32 {
		panic("arch-specific crc32 instruction for Castagnoli not available")
	}

	return ^castagnoliUpdate(^crc, p)
}

func archAvailableIEEE() bool {
	return cpu.Loong64.HasCRC32
}

func archInitIEEE() {
	if !cpu.Loong64.HasCRC32 {
		panic("arch-specific crc32 instruction for IEEE not available")
	}
}

func archUpdateIEEE(crc uint32, p []byte) uint32 {
	if !cpu.Loong64.HasCRC32 {
		panic("arch-specific crc32 instruction for IEEE not available")
	}

	return ^ieeeUpdate(^crc, p)
}

```

// === FILE: references/go/src/hash/crc32/crc32_loong64.s ===
```text
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// castagnoliUpdate updates the non-inverted crc with the given data.

// func castagnoliUpdate(crc uint32, p []byte) uint32
TEXT ·castagnoliUpdate(SB),NOSPLIT,$0-36
	MOVWU	crc+0(FP), R4		// a0 = CRC value
	MOVV	p+8(FP), R5		// a1 = data pointer
	MOVV	p_len+16(FP), R6	// a2 = len(p)

	SGT	$8, R6, R12
	BNE	R12, less_than_8
	AND	$7, R5, R12
	BEQ	R12, aligned

	// Process the first few bytes to 8-byte align the input.
	// t0 = 8 - t0. We need to process this many bytes to align.
	SUB	$1, R12
	XOR	$7, R12

	AND	$1, R12, R13
	BEQ	R13, align_2
	MOVB	(R5), R13
	CRCCWBW	R4, R13, R4
	ADDV	$1, R5
	ADDV	$-1, R6

align_2:
	AND	$2, R12, R13
	BEQ	R13, align_4
	MOVH	(R5), R13
	CRCCWHW	R4, R13, R4
	ADDV	$2, R5
	ADDV	$-2, R6

align_4:
	AND	$4, R12, R13
	BEQ	R13, aligned
	MOVW	(R5), R13
	CRCCWWW	R4, R13, R4
	ADDV	$4, R5
	ADDV	$-4, R6

aligned:
	// The input is now 8-byte aligned and we can process 8-byte chunks.
	SGT	$8, R6, R12
	BNE	R12, less_than_8
	MOVV	(R5), R13
	CRCCWVW	R4, R13, R4
	ADDV	$8, R5
	ADDV	$-8, R6
	JMP	aligned

less_than_8:
	// We may have some bytes left over; process 4 bytes, then 2, then 1.
	AND	$4, R6, R12
	BEQ	R12, less_than_4
	MOVW	(R5), R13
	CRCCWWW	R4, R13, R4
	ADDV	$4, R5
	ADDV	$-4, R6

less_than_4:
	AND	$2, R6, R12
	BEQ	R12, less_than_2
	MOVH	(R5), R13
	CRCCWHW	R4, R13, R4
	ADDV	$2, R5
	ADDV	$-2, R6

less_than_2:
	BEQ	R6, done
	MOVB	(R5), R13
	CRCCWBW	R4, R13, R4

done:
	MOVW	R4, ret+32(FP)
	RET

// ieeeUpdate updates the non-inverted crc with the given data.

// func ieeeUpdate(crc uint32, p []byte) uint32
TEXT ·ieeeUpdate(SB),NOSPLIT,$0-36
	MOVWU	crc+0(FP), R4		// a0 = CRC value
	MOVV	p+8(FP), R5		// a1 = data pointer
	MOVV	p_len+16(FP), R6	// a2 = len(p)

	SGT	$8, R6, R12
	BNE	R12, less_than_8
	AND	$7, R5, R12
	BEQ	R12, aligned

	// Process the first few bytes to 8-byte align the input.
	// t0 = 8 - t0. We need to process this many bytes to align.
	SUB	$1, R12
	XOR	$7, R12

	AND	$1, R12, R13
	BEQ	R13, align_2
	MOVB	(R5), R13
	CRCWBW	R4, R13, R4
	ADDV	$1, R5
	ADDV	$-1, R6

align_2:
	AND	$2, R12, R13
	BEQ	R13, align_4
	MOVH	(R5), R13
	CRCWHW	R4, R13, R4
	ADDV	$2, R5
	ADDV	$-2, R6

align_4:
	AND	$4, R12, R13
	BEQ	R13, aligned
	MOVW	(R5), R13
	CRCWWW	R4, R13, R4
	ADDV	$4, R5
	ADDV	$-4, R6

aligned:
	// The input is now 8-byte aligned and we can process 8-byte chunks.
	SGT	$8, R6, R12
	BNE	R12, less_than_8
	MOVV	(R5), R13
	CRCWVW	R4, R13, R4
	ADDV	$8, R5
	ADDV	$-8, R6
	JMP	aligned

less_than_8:
	// We may have some bytes left over; process 4 bytes, then 2, then 1.
	AND	$4, R6, R12
	BEQ	R12, less_than_4
	MOVW	(R5), R13
	CRCWWW	R4, R13, R4
	ADDV	$4, R5
	ADDV	$-4, R6

less_than_4:
	AND	$2, R6, R12
	BEQ	R12, less_than_2
	MOVH	(R5), R13
	CRCWHW	R4, R13, R4
	ADDV	$2, R5
	ADDV	$-2, R6

less_than_2:
	BEQ	R6, done
	MOVB	(R5), R13
	CRCWBW	R4, R13, R4

done:
	MOVW	R4, ret+32(FP)
	RET


```

// === FILE: references/go/src/hash/crc32/crc32_otherarch.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !amd64 && !s390x && !ppc64le && !arm64 && !loong64

package crc32

func archAvailableIEEE() bool                    { return false }
func archInitIEEE()                              { panic("not available") }
func archUpdateIEEE(crc uint32, p []byte) uint32 { panic("not available") }

func archAvailableCastagnoli() bool                    { return false }
func archInitCastagnoli()                              { panic("not available") }
func archUpdateCastagnoli(crc uint32, p []byte) uint32 { panic("not available") }

```

// === FILE: references/go/src/hash/crc32/crc32_ppc64le.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crc32

import (
	"unsafe"
)

const (
	vecMinLen    = 16
	vecAlignMask = 15 // align to 16 bytes
	crcIEEE      = 1
	crcCast      = 2
)

//go:noescape
func ppc64SlicingUpdateBy8(crc uint32, table8 *slicing8Table, p []byte) uint32

// this function requires the buffer to be 16 byte aligned and > 16 bytes long.
//
//go:noescape
func vectorCrc32(crc uint32, poly uint32, p []byte) uint32

var archCastagnoliTable8 *slicing8Table

func archInitCastagnoli() {
	archCastagnoliTable8 = slicingMakeTable(Castagnoli)
}

func archUpdateCastagnoli(crc uint32, p []byte) uint32 {
	if len(p) >= 4*vecMinLen {
		// If not aligned then process the initial unaligned bytes

		if uint64(uintptr(unsafe.Pointer(&p[0])))&uint64(vecAlignMask) != 0 {
			align := uint64(uintptr(unsafe.Pointer(&p[0]))) & uint64(vecAlignMask)
			newlen := vecMinLen - align
			crc = ppc64SlicingUpdateBy8(crc, archCastagnoliTable8, p[:newlen])
			p = p[newlen:]
		}
		// p should be aligned now
		aligned := len(p) & ^vecAlignMask
		crc = vectorCrc32(crc, crcCast, p[:aligned])
		p = p[aligned:]
	}
	if len(p) == 0 {
		return crc
	}
	return ppc64SlicingUpdateBy8(crc, archCastagnoliTable8, p)
}

func archAvailableIEEE() bool {
	return true
}
func archAvailableCastagnoli() bool {
	return true
}

var archIeeeTable8 *slicing8Table

func archInitIEEE() {
	// We still use slicing-by-8 for small buffers.
	archIeeeTable8 = slicingMakeTable(IEEE)
}

// archUpdateIEEE calculates the checksum of p using vectorizedIEEE.
func archUpdateIEEE(crc uint32, p []byte) uint32 {

	// Check if vector code should be used.  If not aligned, then handle those
	// first up to the aligned bytes.

	if len(p) >= 4*vecMinLen {
		if uint64(uintptr(unsafe.Pointer(&p[0])))&uint64(vecAlignMask) != 0 {
			align := uint64(uintptr(unsafe.Pointer(&p[0]))) & uint64(vecAlignMask)
			newlen := vecMinLen - align
			crc = ppc64SlicingUpdateBy8(crc, archIeeeTable8, p[:newlen])
			p = p[newlen:]
		}
		aligned := len(p) & ^vecAlignMask
		crc = vectorCrc32(crc, crcIEEE, p[:aligned])
		p = p[aligned:]
	}
	if len(p) == 0 {
		return crc
	}
	return ppc64SlicingUpdateBy8(crc, archIeeeTable8, p)
}

```

// === FILE: references/go/src/hash/crc32/crc32_ppc64le.s ===
```text
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The vectorized implementation found below is a derived work
// from code written by Anton Blanchard <anton@au.ibm.com> found
// at https://github.com/antonblanchard/crc32-vpmsum.  The original
// is dual licensed under GPL and Apache 2.  As the copyright holder
// for the work, IBM has contributed this new work under
// the golang license.

// Changes include porting to Go assembler with modifications for
// the Go ABI for ppc64le.

#include "textflag.h"

#define POWER8_OFFSET 132

#define off16	R16
#define off32	R17
#define off48	R18
#define off64	R19
#define off80	R20
#define off96	R21
#define	off112	R22

#define const1	V24
#define const2	V25

#define byteswap	V26
#define mask_32bit	V27
#define mask_64bit	V28
#define zeroes		V29

#define MAX_SIZE	32*1024
#define REFLECT

TEXT ·ppc64SlicingUpdateBy8(SB), NOSPLIT|NOFRAME, $0-44
	MOVWZ	crc+0(FP), R3   // incoming crc
	MOVD    table8+8(FP), R4   // *Table
	MOVD    p+16(FP), R5
	MOVD    p_len+24(FP), R6 // p len

	CMP     $0,R6           // len == 0?
	BNE     start
	MOVW    R3,ret+40(FP)   // return crc
	RET

start:
	NOR     R3,R3,R7        // ^crc
	MOVWZ	R7,R7		// 32 bits
	CMP	R6,$16
	MOVD	R6,CTR
	BLT	short
	SRAD    $3,R6,R8        // 8 byte chunks
	MOVD    R8,CTR

loop:
	MOVWZ	0(R5),R8	// 0-3 bytes of p ?Endian?
	MOVWZ	4(R5),R9	// 4-7 bytes of p
	MOVD	R4,R10		// &tab[0]
	XOR	R7,R8,R7	// crc ^= byte[0:3]
	RLDICL	$40,R9,$56,R17	// p[7]
	SLD	$2,R17,R17	// p[7]*4
	RLDICL	$40,R7,$56,R8	// crc>>24
	SLD	$2,R8,R8	// crc>>24*4
	RLDICL	$48,R9,$56,R18	// p[6]
	SLD	$2,R18,R18	// p[6]*4
	MOVWZ	(R10)(R17),R21	// tab[0][p[7]]
	ADD	$1024,R10,R10	// tab[1]
	RLDICL	$56,R9,$56,R19	// p[5]
	SLD	$2,R19,R19	// p[5]*4:1
	MOVWZ	(R10)(R18),R22	// tab[1][p[6]]
	ADD	$1024,R10,R10	// tab[2]
	XOR	R21,R22,R21	// xor done R22
	CLRLSLDI $56,R9,$2,R20
	MOVWZ	(R10)(R19),R23	// tab[2][p[5]]
	ADD	$1024,R10,R10	// &tab[3]
	XOR	R21,R23,R21	// xor done R23
	MOVWZ	(R10)(R20),R24	// tab[3][p[4]]
	ADD 	$1024,R10,R10   // &tab[4]
	XOR	R21,R24,R21	// xor done R24
	MOVWZ	(R10)(R8),R25	// tab[4][crc>>24]
	RLDICL	$48,R7,$56,R24	// crc>>16&0xFF
	XOR	R21,R25,R21	// xor done R25
	ADD	$1024,R10,R10	// &tab[5]
	SLD	$2,R24,R24	// crc>>16&0xFF*4
	MOVWZ	(R10)(R24),R26	// tab[5][crc>>16&0xFF]
	XOR	R21,R26,R21	// xor done R26
	RLDICL	$56,R7,$56,R25	// crc>>8
	ADD	$1024,R10,R10	// &tab[6]
	SLD	$2,R25,R25	// crc>>8&FF*2
	MOVBZ   R7,R26          // crc&0xFF
	MOVWZ	(R10)(R25),R27	// tab[6][crc>>8&0xFF]
	ADD 	$1024,R10,R10   // &tab[7]
	SLD	$2,R26,R26	// crc&0xFF*2
	XOR	R21,R27,R21	// xor done R27
	ADD     $8,R5           // p = p[8:]
	MOVWZ	(R10)(R26),R28	// tab[7][crc&0xFF]
	XOR	R21,R28,R21	// xor done R28
	MOVWZ	R21,R7		// crc for next round
	BDNZ 	loop
	ANDCC	$7,R6,R8	// any leftover bytes
	BEQ	done		// none --> done
	MOVD	R8,CTR		// byte count
	PCALIGN $16             // align short loop
short:
	MOVBZ 	0(R5),R8	// get v
	XOR 	R8,R7,R8	// byte(crc)^v -> R8
	RLDIC	$2,R8,$54,R8	// rldicl r8,r8,2,22
	SRD 	$8,R7,R14	// crc>>8
	MOVWZ	(R4)(R8),R10
	ADD	$1,R5
	XOR 	R10,R14,R7	// loop crc in R7
	BDNZ 	short
done:
	NOR     R7,R7,R7        // ^crc
	MOVW    R7,ret+40(FP)   // return crc
	RET

#ifdef BYTESWAP_DATA
DATA ·byteswapcons+0(SB)/8,$0x0706050403020100
DATA ·byteswapcons+8(SB)/8,$0x0f0e0d0c0b0a0908

GLOBL ·byteswapcons+0(SB),RODATA,$16
#endif

TEXT ·vectorCrc32(SB), NOSPLIT|NOFRAME, $0-36
	MOVWZ	crc+0(FP), R3   // incoming crc
	MOVWZ	ctab+4(FP), R14   // crc poly id
	MOVD    p+8(FP), R4
	MOVD    p_len+16(FP), R5 // p len

	// R3 = incoming crc
	// R14 = constant table identifier
	// R5 = address of bytes
	// R6 = length of bytes

	// defines for index loads

	MOVD	$16,off16
	MOVD	$32,off32
	MOVD	$48,off48
	MOVD	$64,off64
	MOVD	$80,off80
	MOVD	$96,off96
	MOVD	$112,off112
	MOVD	$0,R15

	MOVD	R3,R10	// save initial crc

	NOR	R3,R3,R3  // ^crc
	MOVWZ	R3,R3	// 32 bits
	VXOR	zeroes,zeroes,zeroes  // clear the V reg
	VSPLTISW $-1,V0
	VSLDOI	$4,V29,V0,mask_32bit
	VSLDOI	$8,V29,V0,mask_64bit

	VXOR	V8,V8,V8
	MTVSRD	R3,VS40	// crc initial value VS40 = V8

#ifdef REFLECT
	VSLDOI	$8,zeroes,V8,V8  // or: VSLDOI V29,V8,V27,4 for top 32 bits?
#else
	VSLDOI	$4,V8,zeroes,V8
#endif

#ifdef BYTESWAP_DATA
	MOVD    $·byteswapcons(SB),R3
	LVX	(R3),byteswap
#endif

	CMPU	R5,$256		// length of bytes
	BLT	short

	RLDICR	$0,R5,$56,R6 // chunk to process

	// First step for larger sizes
l1:	MOVD	$32768,R7
	MOVD	R7,R9
	CMP	R6,R7   // compare R6, R7 (MAX SIZE)
	BGT	top	// less than MAX, just do remainder
	MOVD	R6,R7
top:
	SUB	R7,R6,R6

	// mainloop does 128 bytes at a time
	SRD	$7,R7

	// determine the offset into the constants table to start with.
	// Each constant is 128 bytes, used against 16 bytes of data.
	SLD	$4,R7,R8
	SRD	$3,R9,R9
	SUB	R8,R9,R8

	// The last iteration is reduced in a separate step
	ADD	$-1,R7
	MOVD	R7,CTR

	// Determine which constant table (depends on poly)
	CMP	R14,$1
	BNE	castTable
	MOVD	$·IEEEConst(SB),R3
	BR	startConst
castTable:
	MOVD	$·CastConst(SB),R3

startConst:
	ADD	R3,R8,R3	// starting point in constants table

	VXOR	V0,V0,V0	// clear the V regs
	VXOR	V1,V1,V1
	VXOR	V2,V2,V2
	VXOR	V3,V3,V3
	VXOR	V4,V4,V4
	VXOR	V5,V5,V5
	VXOR	V6,V6,V6
	VXOR	V7,V7,V7

	LVX	(R3),const1	// loading constant values

	CMP	R15,$1		// Identify warm up pass
	BEQ	next

	// First warm up pass: load the bytes to process
	LVX	(R4),V16
	LVX	(R4+off16),V17
	LVX	(R4+off32),V18
	LVX	(R4+off48),V19
	LVX	(R4+off64),V20
	LVX	(R4+off80),V21
	LVX	(R4+off96),V22
	LVX	(R4+off112),V23
	ADD	$128,R4		// bump up to next 128 bytes in buffer

	VXOR	V16,V8,V16	// xor in initial CRC in V8

next:
	BC	18,0,first_warm_up_done

	ADD	$16,R3		// bump up to next constants
	LVX	(R3),const2	// table values

	VPMSUMD	V16,const1,V8 // second warm up pass
	LVX	(R4),V16	// load from buffer
	OR	$0,R2,R2

	VPMSUMD	V17,const1,V9	// vpmsumd with constants
	LVX	(R4+off16),V17	// load next from buffer
	OR	$0,R2,R2

	VPMSUMD	V18,const1,V10	// vpmsumd with constants
	LVX	(R4+off32),V18	// load next from buffer
	OR	$0,R2,R2

	VPMSUMD	V19,const1,V11	// vpmsumd with constants
	LVX	(R4+off48),V19	// load next from buffer
	OR	$0,R2,R2

	VPMSUMD	V20,const1,V12	// vpmsumd with constants
	LVX	(R4+off64),V20	// load next from buffer
	OR	$0,R2,R2

	VPMSUMD	V21,const1,V13	// vpmsumd with constants
	LVX	(R4+off80),V21	// load next from buffer
	OR	$0,R2,R2

	VPMSUMD	V22,const1,V14	// vpmsumd with constants
	LVX	(R4+off96),V22	// load next from buffer
	OR	$0,R2,R2

	VPMSUMD	V23,const1,V15	// vpmsumd with constants
	LVX	(R4+off112),V23	// load next from buffer

	ADD	$128,R4		// bump up to next 128 bytes in buffer

	BC	18,0,first_cool_down

cool_top:
	LVX	(R3),const1	// constants
	ADD	$16,R3		// inc to next constants
	OR	$0,R2,R2

	VXOR	V0,V8,V0	// xor in previous vpmsumd
	VPMSUMD	V16,const2,V8	// vpmsumd with constants
	LVX	(R4),V16	// buffer
	OR	$0,R2,R2

	VXOR	V1,V9,V1	// xor in previous
	VPMSUMD	V17,const2,V9	// vpmsumd with constants
	LVX	(R4+off16),V17	// next in buffer
	OR	$0,R2,R2

	VXOR	V2,V10,V2	// xor in previous
	VPMSUMD	V18,const2,V10	// vpmsumd with constants
	LVX	(R4+off32),V18	// next in buffer
	OR	$0,R2,R2

	VXOR	V3,V11,V3	// xor in previous
	VPMSUMD	V19,const2,V11	// vpmsumd with constants
	LVX	(R4+off48),V19	// next in buffer
	LVX	(R3),const2	// get next constant
	OR	$0,R2,R2

	VXOR	V4,V12,V4	// xor in previous
	VPMSUMD	V20,const1,V12	// vpmsumd with constants
	LVX	(R4+off64),V20	// next in buffer
	OR	$0,R2,R2

	VXOR	V5,V13,V5	// xor in previous
	VPMSUMD	V21,const1,V13	// vpmsumd with constants
	LVX	(R4+off80),V21	// next in buffer
	OR	$0,R2,R2

	VXOR	V6,V14,V6	// xor in previous
	VPMSUMD	V22,const1,V14	// vpmsumd with constants
	LVX	(R4+off96),V22	// next in buffer
	OR	$0,R2,R2

	VXOR	V7,V15,V7	// xor in previous
	VPMSUMD	V23,const1,V15	// vpmsumd with constants
	LVX	(R4+off112),V23	// next in buffer

	ADD	$128,R4		// bump up buffer pointer
	BDNZ	cool_top	// are we done?

first_cool_down:

	// load the constants
	// xor in the previous value
	// vpmsumd the result with constants

	LVX	(R3),const1
	ADD	$16,R3

	VXOR	V0,V8,V0
	VPMSUMD V16,const1,V8
	OR	$0,R2,R2

	VXOR	V1,V9,V1
	VPMSUMD	V17,const1,V9
	OR	$0,R2,R2

	VXOR	V2,V10,V2
	VPMSUMD	V18,const1,V10
	OR	$0,R2,R2

	VXOR	V3,V11,V3
	VPMSUMD	V19,const1,V11
	OR	$0,R2,R2

	VXOR	V4,V12,V4
	VPMSUMD	V20,const1,V12
	OR	$0,R2,R2

	VXOR	V5,V13,V5
	VPMSUMD	V21,const1,V13
	OR	$0,R2,R2

	VXOR	V6,V14,V6
	VPMSUMD	V22,const1,V14
	OR	$0,R2,R2

	VXOR	V7,V15,V7
	VPMSUMD	V23,const1,V15
	OR	$0,R2,R2

second_cool_down:

	VXOR    V0,V8,V0
	VXOR    V1,V9,V1
	VXOR    V2,V10,V2
	VXOR    V3,V11,V3
	VXOR    V4,V12,V4
	VXOR    V5,V13,V5
	VXOR    V6,V14,V6
	VXOR    V7,V15,V7

#ifdef REFLECT
	VSLDOI  $4,V0,zeroes,V0
	VSLDOI  $4,V1,zeroes,V1
	VSLDOI  $4,V2,zeroes,V2
	VSLDOI  $4,V3,zeroes,V3
	VSLDOI  $4,V4,zeroes,V4
	VSLDOI  $4,V5,zeroes,V5
	VSLDOI  $4,V6,zeroes,V6
	VSLDOI  $4,V7,zeroes,V7
#endif

	LVX	(R4),V8
	LVX	(R4+off16),V9
	LVX	(R4+off32),V10
	LVX	(R4+off48),V11
	LVX	(R4+off64),V12
	LVX	(R4+off80),V13
	LVX	(R4+off96),V14
	LVX	(R4+off112),V15

	ADD	$128,R4

	VXOR	V0,V8,V16
	VXOR	V1,V9,V17
	VXOR	V2,V10,V18
	VXOR	V3,V11,V19
	VXOR	V4,V12,V20
	VXOR	V5,V13,V21
	VXOR	V6,V14,V22
	VXOR	V7,V15,V23

	MOVD    $1,R15
	CMP     $0,R6
	ADD     $128,R6

	BNE	l1
	ANDCC   $127,R5
	SUBC	R5,$128,R6
	ADD	R3,R6,R3

	SRD	$4,R5,R7
	MOVD	R7,CTR
	LVX	(R3),V0
	LVX	(R3+off16),V1
	LVX	(R3+off32),V2
	LVX	(R3+off48),V3
	LVX	(R3+off64),V4
	LVX	(R3+off80),V5
	LVX	(R3+off96),V6
	LVX	(R3+off112),V7

	ADD	$128,R3

	VPMSUMW	V16,V0,V0
	VPMSUMW	V17,V1,V1
	VPMSUMW	V18,V2,V2
	VPMSUMW	V19,V3,V3
	VPMSUMW	V20,V4,V4
	VPMSUMW	V21,V5,V5
	VPMSUMW	V22,V6,V6
	VPMSUMW	V23,V7,V7

	// now reduce the tail

	CMP	$0,R7
	BEQ	next1

	LVX	(R4),V16
	LVX	(R3),V17
	VPMSUMW	V16,V17,V16
	VXOR	V0,V16,V0
	BC	18,0,next1

	LVX	(R4+off16),V16
	LVX	(R3+off16),V17
	VPMSUMW	V16,V17,V16
	VXOR	V0,V16,V0
	BC	18,0,next1

	LVX	(R4+off32),V16
	LVX	(R3+off32),V17
	VPMSUMW	V16,V17,V16
	VXOR	V0,V16,V0
	BC	18,0,next1

	LVX	(R4+off48),V16
	LVX	(R3+off48),V17
	VPMSUMW	V16,V17,V16
	VXOR	V0,V16,V0
	BC	18,0,next1

	LVX	(R4+off64),V16
	LVX	(R3+off64),V17
	VPMSUMW	V16,V17,V16
	VXOR	V0,V16,V0
	BC	18,0,next1

	LVX	(R4+off80),V16
	LVX	(R3+off80),V17
	VPMSUMW	V16,V17,V16
	VXOR	V0,V16,V0
	BC	18,0,next1

	LVX	(R4+off96),V16
	LVX	(R3+off96),V17
	VPMSUMW	V16,V17,V16
	VXOR	V0,V16,V0

next1:
	VXOR	V0,V1,V0
	VXOR	V2,V3,V2
	VXOR	V4,V5,V4
	VXOR	V6,V7,V6
	VXOR	V0,V2,V0
	VXOR	V4,V6,V4
	VXOR	V0,V4,V0

barrett_reduction:

	CMP	R14,$1
	BNE	barcstTable
	MOVD	$·IEEEBarConst(SB),R3
	BR	startbarConst
barcstTable:
	MOVD    $·CastBarConst(SB),R3

startbarConst:
	LVX	(R3),const1
	LVX	(R3+off16),const2

	VSLDOI	$8,V0,V0,V1
	VXOR	V0,V1,V0

#ifdef REFLECT
	VSPLTISB $1,V1
	VSL	V0,V1,V0
#endif

	VAND	V0,mask_64bit,V0

#ifndef	REFLECT

	VPMSUMD	V0,const1,V1
	VSLDOI	$8,zeroes,V1,V1
	VPMSUMD	V1,const2,V1
	VXOR	V0,V1,V0
	VSLDOI	$8,V0,zeroes,V0

#else

	VAND	V0,mask_32bit,V1
	VPMSUMD	V1,const1,V1
	VAND	V1,mask_32bit,V1
	VPMSUMD	V1,const2,V1
	VXOR	V0,V1,V0
	VSLDOI  $4,V0,zeroes,V0

#endif

	MFVSRD	VS32,R3 // VS32 = V0

	NOR	R3,R3,R3 // return ^crc
	MOVW	R3,ret+32(FP)
	RET

first_warm_up_done:

	LVX	(R3),const1
	ADD	$16,R3

	VPMSUMD	V16,const1,V8
	VPMSUMD	V17,const1,V9
	VPMSUMD	V18,const1,V10
	VPMSUMD	V19,const1,V11
	VPMSUMD	V20,const1,V12
	VPMSUMD	V21,const1,V13
	VPMSUMD	V22,const1,V14
	VPMSUMD	V23,const1,V15

	BR	second_cool_down

short:
	CMP	$0,R5
	BEQ	zero

	// compute short constants

	CMP     R14,$1
	BNE     castshTable
	MOVD    $·IEEEConst(SB),R3
	ADD	$4080,R3
	BR      startshConst
castshTable:
	MOVD    $·CastConst(SB),R3
	ADD	$4080,R3

startshConst:
	SUBC	R5,$256,R6	// sub from 256
	ADD	R3,R6,R3

	// calculate where to start

	SRD	$4,R5,R7
	MOVD	R7,CTR

	VXOR	V19,V19,V19
	VXOR	V20,V20,V20

	LVX	(R4),V0
	LVX	(R3),V16
	VXOR	V0,V8,V0
	VPMSUMW	V0,V16,V0
	BC	18,0,v0

	LVX	(R4+off16),V1
	LVX	(R3+off16),V17
	VPMSUMW	V1,V17,V1
	BC	18,0,v1

	LVX	(R4+off32),V2
	LVX	(R3+off32),V16
	VPMSUMW	V2,V16,V2
	BC	18,0,v2

	LVX	(R4+off48),V3
	LVX	(R3+off48),V17
	VPMSUMW	V3,V17,V3
	BC	18,0,v3

	LVX	(R4+off64),V4
	LVX	(R3+off64),V16
	VPMSUMW	V4,V16,V4
	BC	18,0,v4

	LVX	(R4+off80),V5
	LVX	(R3+off80),V17
	VPMSUMW	V5,V17,V5
	BC	18,0,v5

	LVX	(R4+off96),V6
	LVX	(R3+off96),V16
	VPMSUMW	V6,V16,V6
	BC	18,0,v6

	LVX	(R4+off112),V7
	LVX	(R3+off112),V17
	VPMSUMW	V7,V17,V7
	BC	18,0,v7

	ADD	$128,R3
	ADD	$128,R4

	LVX	(R4),V8
	LVX	(R3),V16
	VPMSUMW	V8,V16,V8
	BC	18,0,v8

	LVX	(R4+off16),V9
	LVX	(R3+off16),V17
	VPMSUMW	V9,V17,V9
	BC	18,0,v9

	LVX	(R4+off32),V10
	LVX	(R3+off32),V16
	VPMSUMW	V10,V16,V10
	BC	18,0,v10

	LVX	(R4+off48),V11
	LVX	(R3+off48),V17
	VPMSUMW	V11,V17,V11
	BC	18,0,v11

	LVX	(R4+off64),V12
	LVX	(R3+off64),V16
	VPMSUMW	V12,V16,V12
	BC	18,0,v12

	LVX	(R4+off80),V13
	LVX	(R3+off80),V17
	VPMSUMW	V13,V17,V13
	BC	18,0,v13

	LVX	(R4+off96),V14
	LVX	(R3+off96),V16
	VPMSUMW	V14,V16,V14
	BC	18,0,v14

	LVX	(R4+off112),V15
	LVX	(R3+off112),V17
	VPMSUMW	V15,V17,V15

	VXOR	V19,V15,V19
v14:	VXOR	V20,V14,V20
v13:	VXOR	V19,V13,V19
v12:	VXOR	V20,V12,V20
v11:	VXOR	V19,V11,V19
v10:	VXOR	V20,V10,V20
v9:	VXOR	V19,V9,V19
v8:	VXOR	V20,V8,V20
v7:	VXOR	V19,V7,V19
v6:	VXOR	V20,V6,V20
v5:	VXOR	V19,V5,V19
v4:	VXOR	V20,V4,V20
v3:	VXOR	V19,V3,V19
v2:	VXOR	V20,V2,V20
v1:	VXOR	V19,V1,V19
v0:	VXOR	V20,V0,V20

	VXOR	V19,V20,V0

	BR	barrett_reduction

zero:
	// This case is the original crc, so just return it
	MOVW    R10,ret+32(FP)
	RET

```

// === FILE: references/go/src/hash/crc32/crc32_s390x.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crc32

import "internal/cpu"

const (
	vxMinLen    = 64
	vxAlignMask = 15 // align to 16 bytes
)

// hasVX reports whether the machine has the z/Architecture
// vector facility installed and enabled.
var hasVX = cpu.S390X.HasVX

// vectorizedCastagnoli implements CRC32 using vector instructions.
// It is defined in crc32_s390x.s.
//
//go:noescape
func vectorizedCastagnoli(crc uint32, p []byte) uint32

// vectorizedIEEE implements CRC32 using vector instructions.
// It is defined in crc32_s390x.s.
//
//go:noescape
func vectorizedIEEE(crc uint32, p []byte) uint32

func archAvailableCastagnoli() bool {
	return hasVX
}

var archCastagnoliTable8 *slicing8Table

func archInitCastagnoli() {
	if !hasVX {
		panic("not available")
	}
	// We still use slicing-by-8 for small buffers.
	archCastagnoliTable8 = slicingMakeTable(Castagnoli)
}

// archUpdateCastagnoli calculates the checksum of p using
// vectorizedCastagnoli.
func archUpdateCastagnoli(crc uint32, p []byte) uint32 {
	if !hasVX {
		panic("not available")
	}
	// Use vectorized function if data length is above threshold.
	if len(p) >= vxMinLen {
		aligned := len(p) & ^vxAlignMask
		crc = vectorizedCastagnoli(crc, p[:aligned])
		p = p[aligned:]
	}
	if len(p) == 0 {
		return crc
	}
	return slicingUpdate(crc, archCastagnoliTable8, p)
}

func archAvailableIEEE() bool {
	return hasVX
}

var archIeeeTable8 *slicing8Table

func archInitIEEE() {
	if !hasVX {
		panic("not available")
	}
	// We still use slicing-by-8 for small buffers.
	archIeeeTable8 = slicingMakeTable(IEEE)
}

// archUpdateIEEE calculates the checksum of p using vectorizedIEEE.
func archUpdateIEEE(crc uint32, p []byte) uint32 {
	if !hasVX {
		panic("not available")
	}
	// Use vectorized function if data length is above threshold.
	if len(p) >= vxMinLen {
		aligned := len(p) & ^vxAlignMask
		crc = vectorizedIEEE(crc, p[:aligned])
		p = p[aligned:]
	}
	if len(p) == 0 {
		return crc
	}
	return slicingUpdate(crc, archIeeeTable8, p)
}

```

// === FILE: references/go/src/hash/crc32/crc32_s390x.s ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// Vector register range containing CRC-32 constants

#define CONST_PERM_LE2BE        V9
#define CONST_R2R1              V10
#define CONST_R4R3              V11
#define CONST_R5                V12
#define CONST_RU_POLY           V13
#define CONST_CRC_POLY          V14


// The CRC-32 constant block contains reduction constants to fold and
// process particular chunks of the input data stream in parallel.
//
// Note that the constant definitions below are extended in order to compute
// intermediate results with a single VECTOR GALOIS FIELD MULTIPLY instruction.
// The rightmost doubleword can be 0 to prevent contribution to the result or
// can be multiplied by 1 to perform an XOR without the need for a separate
// VECTOR EXCLUSIVE OR instruction.
//
// The polynomials used are bit-reflected:
//
//            IEEE: P'(x) = 0x0edb88320
//      Castagnoli: P'(x) = 0x082f63b78


// IEEE polynomial constants
DATA    ·crclecons+0(SB)/8,  $0x0F0E0D0C0B0A0908       // LE-to-BE mask
DATA    ·crclecons+8(SB)/8,  $0x0706050403020100
DATA    ·crclecons+16(SB)/8, $0x00000001c6e41596       // R2
DATA    ·crclecons+24(SB)/8, $0x0000000154442bd4       // R1
DATA    ·crclecons+32(SB)/8, $0x00000000ccaa009e       // R4
DATA    ·crclecons+40(SB)/8, $0x00000001751997d0       // R3
DATA    ·crclecons+48(SB)/8, $0x0000000000000000
DATA    ·crclecons+56(SB)/8, $0x0000000163cd6124       // R5
DATA    ·crclecons+64(SB)/8, $0x0000000000000000
DATA    ·crclecons+72(SB)/8, $0x00000001F7011641       // u'
DATA    ·crclecons+80(SB)/8, $0x0000000000000000
DATA    ·crclecons+88(SB)/8, $0x00000001DB710641       // P'(x) << 1

GLOBL    ·crclecons(SB),RODATA, $144

// Castagonli Polynomial constants
DATA    ·crcclecons+0(SB)/8,  $0x0F0E0D0C0B0A0908      // LE-to-BE mask
DATA    ·crcclecons+8(SB)/8,  $0x0706050403020100
DATA    ·crcclecons+16(SB)/8, $0x000000009e4addf8      // R2
DATA    ·crcclecons+24(SB)/8, $0x00000000740eef02      // R1
DATA    ·crcclecons+32(SB)/8, $0x000000014cd00bd6      // R4
DATA    ·crcclecons+40(SB)/8, $0x00000000f20c0dfe      // R3
DATA    ·crcclecons+48(SB)/8, $0x0000000000000000
DATA    ·crcclecons+56(SB)/8, $0x00000000dd45aab8      // R5
DATA    ·crcclecons+64(SB)/8, $0x0000000000000000
DATA    ·crcclecons+72(SB)/8, $0x00000000dea713f1      // u'
DATA    ·crcclecons+80(SB)/8, $0x0000000000000000
DATA    ·crcclecons+88(SB)/8, $0x0000000105ec76f0      // P'(x) << 1

GLOBL   ·crcclecons(SB),RODATA, $144

// The CRC-32 function(s) use these calling conventions:
//
// Parameters:
//
//      R2:    Initial CRC value, typically ~0; and final CRC (return) value.
//      R3:    Input buffer pointer, performance might be improved if the
//             buffer is on a doubleword boundary.
//      R4:    Length of the buffer, must be 64 bytes or greater.
//
// Register usage:
//
//      R5:     CRC-32 constant pool base pointer.
//      V0:     Initial CRC value and intermediate constants and results.
//      V1..V4: Data for CRC computation.
//      V5..V8: Next data chunks that are fetched from the input buffer.
//
//      V9..V14: CRC-32 constants.

// func vectorizedIEEE(crc uint32, p []byte) uint32
TEXT ·vectorizedIEEE(SB),NOSPLIT,$0
	MOVWZ   crc+0(FP), R2     // R2 stores the CRC value
	MOVD    p+8(FP), R3       // data pointer
	MOVD    p_len+16(FP), R4  // len(p)

	MOVD    $·crclecons(SB), R5
	BR      vectorizedBody<>(SB)

// func vectorizedCastagnoli(crc uint32, p []byte) uint32
TEXT ·vectorizedCastagnoli(SB),NOSPLIT,$0
	MOVWZ   crc+0(FP), R2     // R2 stores the CRC value
	MOVD    p+8(FP), R3       // data pointer
	MOVD    p_len+16(FP), R4  // len(p)

	// R5: crc-32 constant pool base pointer, constant is used to reduce crc
	MOVD    $·crcclecons(SB), R5
	BR      vectorizedBody<>(SB)

TEXT vectorizedBody<>(SB),NOSPLIT,$0
	XOR     $0xffffffff, R2 // NOTW R2
	VLM     0(R5), CONST_PERM_LE2BE, CONST_CRC_POLY

	// Load the initial CRC value into the rightmost word of V0
	VZERO   V0
	VLVGF   $3, R2, V0

	// Crash if the input size is less than 64-bytes.
	CMP     R4, $64
	BLT     crash

	// Load a 64-byte data chunk and XOR with CRC
	VLM     0(R3), V1, V4    // 64-bytes into V1..V4

	// Reflect the data if the CRC operation is in the bit-reflected domain
	VPERM   V1, V1, CONST_PERM_LE2BE, V1
	VPERM   V2, V2, CONST_PERM_LE2BE, V2
	VPERM   V3, V3, CONST_PERM_LE2BE, V3
	VPERM   V4, V4, CONST_PERM_LE2BE, V4

	VX      V0, V1, V1     // V1 ^= CRC
	ADD     $64, R3        // BUF = BUF + 64
	ADD     $(-64), R4

	// Check remaining buffer size and jump to proper folding method
	CMP     R4, $64
	BLT     less_than_64bytes

fold_64bytes_loop:
	// Load the next 64-byte data chunk into V5 to V8
	VLM     0(R3), V5, V8
	VPERM   V5, V5, CONST_PERM_LE2BE, V5
	VPERM   V6, V6, CONST_PERM_LE2BE, V6
	VPERM   V7, V7, CONST_PERM_LE2BE, V7
	VPERM   V8, V8, CONST_PERM_LE2BE, V8


	// Perform a GF(2) multiplication of the doublewords in V1 with
	// the reduction constants in V0.  The intermediate result is
	// then folded (accumulated) with the next data chunk in V5 and
	// stored in V1.  Repeat this step for the register contents
	// in V2, V3, and V4 respectively.

	VGFMAG  CONST_R2R1, V1, V5, V1
	VGFMAG  CONST_R2R1, V2, V6, V2
	VGFMAG  CONST_R2R1, V3, V7, V3
	VGFMAG  CONST_R2R1, V4, V8 ,V4

	// Adjust buffer pointer and length for next loop
	ADD     $64, R3                  // BUF = BUF + 64
	ADD     $(-64), R4               // LEN = LEN - 64

	CMP     R4, $64
	BGE     fold_64bytes_loop

less_than_64bytes:
	// Fold V1 to V4 into a single 128-bit value in V1
	VGFMAG  CONST_R4R3, V1, V2, V1
	VGFMAG  CONST_R4R3, V1, V3, V1
	VGFMAG  CONST_R4R3, V1, V4, V1

	// Check whether to continue with 64-bit folding
	CMP R4, $16
	BLT final_fold

fold_16bytes_loop:
	VL      0(R3), V2               // Load next data chunk
	VPERM   V2, V2, CONST_PERM_LE2BE, V2

	VGFMAG  CONST_R4R3, V1, V2, V1  // Fold next data chunk

	// Adjust buffer pointer and size for folding next data chunk
	ADD     $16, R3
	ADD     $-16, R4

	// Process remaining data chunks
	CMP     R4 ,$16
	BGE     fold_16bytes_loop

final_fold:
	VLEIB   $7, $0x40, V9
	VSRLB   V9, CONST_R4R3, V0
	VLEIG   $0, $1, V0

	VGFMG   V0, V1, V1

	VLEIB   $7, $0x20, V9         // Shift by words
	VSRLB   V9, V1, V2            // Store remaining bits in V2
	VUPLLF  V1, V1                // Split rightmost doubleword
	VGFMAG  CONST_R5, V1, V2, V1  // V1 = (V1 * R5) XOR V2


	// The input values to the Barret reduction are the degree-63 polynomial
	// in V1 (R(x)), degree-32 generator polynomial, and the reduction
	// constant u.  The Barret reduction result is the CRC value of R(x) mod
	// P(x).
	//
	// The Barret reduction algorithm is defined as:
	//
	//    1. T1(x) = floor( R(x) / x^32 ) GF2MUL u
	//    2. T2(x) = floor( T1(x) / x^32 ) GF2MUL P(x)
	//    3. C(x)  = R(x) XOR T2(x) mod x^32
	//
	// Note: To compensate the division by x^32, use the vector unpack
	// instruction to move the leftmost word into the leftmost doubleword
	// of the vector register.  The rightmost doubleword is multiplied
	// with zero to not contribute to the intermediate results.


	// T1(x) = floor( R(x) / x^32 ) GF2MUL u
	VUPLLF  V1, V2
	VGFMG   CONST_RU_POLY, V2, V2


	// Compute the GF(2) product of the CRC polynomial in VO with T1(x) in
	// V2 and XOR the intermediate result, T2(x),  with the value in V1.
	// The final result is in the rightmost word of V2.

	VUPLLF  V2, V2
	VGFMAG  CONST_CRC_POLY, V2, V1, V2

done:
	VLGVF   $2, V2, R2
	XOR     $0xffffffff, R2 // NOTW R2
	MOVWZ   R2, ret + 32(FP)
	RET

crash:
	MOVD    $0, (R0) // input size is less than 64-bytes

```

// === FILE: references/go/src/hash/crc32/crc32_table_ppc64le.s ===
```text
// Code generated by "go run gen_const_ppc64le.go"; DO NOT EDIT.

#include "textflag.h"

	/* Reduce 262144 kbits to 1024 bits */
	/* x^261184 mod p(x), x^261120 mod p(x) */
DATA ·IEEEConst+0(SB)/8,$0x0000000099ea94a8
DATA ·IEEEConst+8(SB)/8,$0x00000001651797d2

	/* x^260160 mod p(x), x^260096 mod p(x) */
DATA ·IEEEConst+16(SB)/8,$0x00000000945a8420
DATA ·IEEEConst+24(SB)/8,$0x0000000021e0d56c

	/* x^259136 mod p(x), x^259072 mod p(x) */
DATA ·IEEEConst+32(SB)/8,$0x0000000030762706
DATA ·IEEEConst+40(SB)/8,$0x000000000f95ecaa

	/* x^258112 mod p(x), x^258048 mod p(x) */
DATA ·IEEEConst+48(SB)/8,$0x00000001a52fc582
DATA ·IEEEConst+56(SB)/8,$0x00000001ebd224ac

	/* x^257088 mod p(x), x^257024 mod p(x) */
DATA ·IEEEConst+64(SB)/8,$0x00000001a4a7167a
DATA ·IEEEConst+72(SB)/8,$0x000000000ccb97ca

	/* x^256064 mod p(x), x^256000 mod p(x) */
DATA ·IEEEConst+80(SB)/8,$0x000000000c18249a
DATA ·IEEEConst+88(SB)/8,$0x00000001006ec8a8

	/* x^255040 mod p(x), x^254976 mod p(x) */
DATA ·IEEEConst+96(SB)/8,$0x00000000a924ae7c
DATA ·IEEEConst+104(SB)/8,$0x000000014f58f196

	/* x^254016 mod p(x), x^253952 mod p(x) */
DATA ·IEEEConst+112(SB)/8,$0x00000001e12ccc12
DATA ·IEEEConst+120(SB)/8,$0x00000001a7192ca6

	/* x^252992 mod p(x), x^252928 mod p(x) */
DATA ·IEEEConst+128(SB)/8,$0x00000000a0b9d4ac
DATA ·IEEEConst+136(SB)/8,$0x000000019a64bab2

	/* x^251968 mod p(x), x^251904 mod p(x) */
DATA ·IEEEConst+144(SB)/8,$0x0000000095e8ddfe
DATA ·IEEEConst+152(SB)/8,$0x0000000014f4ed2e

	/* x^250944 mod p(x), x^250880 mod p(x) */
DATA ·IEEEConst+160(SB)/8,$0x00000000233fddc4
DATA ·IEEEConst+168(SB)/8,$0x000000011092b6a2

	/* x^249920 mod p(x), x^249856 mod p(x) */
DATA ·IEEEConst+176(SB)/8,$0x00000001b4529b62
DATA ·IEEEConst+184(SB)/8,$0x00000000c8a1629c

	/* x^248896 mod p(x), x^248832 mod p(x) */
DATA ·IEEEConst+192(SB)/8,$0x00000001a7fa0e64
DATA ·IEEEConst+200(SB)/8,$0x000000017bf32e8e

	/* x^247872 mod p(x), x^247808 mod p(x) */
DATA ·IEEEConst+208(SB)/8,$0x00000001b5334592
DATA ·IEEEConst+216(SB)/8,$0x00000001f8cc6582

	/* x^246848 mod p(x), x^246784 mod p(x) */
DATA ·IEEEConst+224(SB)/8,$0x000000011f8ee1b4
DATA ·IEEEConst+232(SB)/8,$0x000000008631ddf0

	/* x^245824 mod p(x), x^245760 mod p(x) */
DATA ·IEEEConst+240(SB)/8,$0x000000006252e632
DATA ·IEEEConst+248(SB)/8,$0x000000007e5a76d0

	/* x^244800 mod p(x), x^244736 mod p(x) */
DATA ·IEEEConst+256(SB)/8,$0x00000000ab973e84
DATA ·IEEEConst+264(SB)/8,$0x000000002b09b31c

	/* x^243776 mod p(x), x^243712 mod p(x) */
DATA ·IEEEConst+272(SB)/8,$0x000000007734f5ec
DATA ·IEEEConst+280(SB)/8,$0x00000001b2df1f84

	/* x^242752 mod p(x), x^242688 mod p(x) */
DATA ·IEEEConst+288(SB)/8,$0x000000007c547798
DATA ·IEEEConst+296(SB)/8,$0x00000001d6f56afc

	/* x^241728 mod p(x), x^241664 mod p(x) */
DATA ·IEEEConst+304(SB)/8,$0x000000007ec40210
DATA ·IEEEConst+312(SB)/8,$0x00000001b9b5e70c

	/* x^240704 mod p(x), x^240640 mod p(x) */
DATA ·IEEEConst+320(SB)/8,$0x00000001ab1695a8
DATA ·IEEEConst+328(SB)/8,$0x0000000034b626d2

	/* x^239680 mod p(x), x^239616 mod p(x) */
DATA ·IEEEConst+336(SB)/8,$0x0000000090494bba
DATA ·IEEEConst+344(SB)/8,$0x000000014c53479a

	/* x^238656 mod p(x), x^238592 mod p(x) */
DATA ·IEEEConst+352(SB)/8,$0x00000001123fb816
DATA ·IEEEConst+360(SB)/8,$0x00000001a6d179a4

	/* x^237632 mod p(x), x^237568 mod p(x) */
DATA ·IEEEConst+368(SB)/8,$0x00000001e188c74c
DATA ·IEEEConst+376(SB)/8,$0x000000015abd16b4

	/* x^236608 mod p(x), x^236544 mod p(x) */
DATA ·IEEEConst+384(SB)/8,$0x00000001c2d3451c
DATA ·IEEEConst+392(SB)/8,$0x00000000018f9852

	/* x^235584 mod p(x), x^235520 mod p(x) */
DATA ·IEEEConst+400(SB)/8,$0x00000000f55cf1ca
DATA ·IEEEConst+408(SB)/8,$0x000000001fb3084a

	/* x^234560 mod p(x), x^234496 mod p(x) */
DATA ·IEEEConst+416(SB)/8,$0x00000001a0531540
DATA ·IEEEConst+424(SB)/8,$0x00000000c53dfb04

	/* x^233536 mod p(x), x^233472 mod p(x) */
DATA ·IEEEConst+432(SB)/8,$0x0000000132cd7ebc
DATA ·IEEEConst+440(SB)/8,$0x00000000e10c9ad6

	/* x^232512 mod p(x), x^232448 mod p(x) */
DATA ·IEEEConst+448(SB)/8,$0x0000000073ab7f36
DATA ·IEEEConst+456(SB)/8,$0x0000000025aa994a

	/* x^231488 mod p(x), x^231424 mod p(x) */
DATA ·IEEEConst+464(SB)/8,$0x0000000041aed1c2
DATA ·IEEEConst+472(SB)/8,$0x00000000fa3a74c4

	/* x^230464 mod p(x), x^230400 mod p(x) */
DATA ·IEEEConst+480(SB)/8,$0x0000000136c53800
DATA ·IEEEConst+488(SB)/8,$0x0000000033eb3f40

	/* x^229440 mod p(x), x^229376 mod p(x) */
DATA ·IEEEConst+496(SB)/8,$0x0000000126835a30
DATA ·IEEEConst+504(SB)/8,$0x000000017193f296

	/* x^228416 mod p(x), x^228352 mod p(x) */
DATA ·IEEEConst+512(SB)/8,$0x000000006241b502
DATA ·IEEEConst+520(SB)/8,$0x0000000043f6c86a

	/* x^227392 mod p(x), x^227328 mod p(x) */
DATA ·IEEEConst+528(SB)/8,$0x00000000d5196ad4
DATA ·IEEEConst+536(SB)/8,$0x000000016b513ec6

	/* x^226368 mod p(x), x^226304 mod p(x) */
DATA ·IEEEConst+544(SB)/8,$0x000000009cfa769a
DATA ·IEEEConst+552(SB)/8,$0x00000000c8f25b4e

	/* x^225344 mod p(x), x^225280 mod p(x) */
DATA ·IEEEConst+560(SB)/8,$0x00000000920e5df4
DATA ·IEEEConst+568(SB)/8,$0x00000001a45048ec

	/* x^224320 mod p(x), x^224256 mod p(x) */
DATA ·IEEEConst+576(SB)/8,$0x0000000169dc310e
DATA ·IEEEConst+584(SB)/8,$0x000000000c441004

	/* x^223296 mod p(x), x^223232 mod p(x) */
DATA ·IEEEConst+592(SB)/8,$0x0000000009fc331c
DATA ·IEEEConst+600(SB)/8,$0x000000000e17cad6

	/* x^222272 mod p(x), x^222208 mod p(x) */
DATA ·IEEEConst+608(SB)/8,$0x000000010d94a81e
DATA ·IEEEConst+616(SB)/8,$0x00000001253ae964

	/* x^221248 mod p(x), x^221184 mod p(x) */
DATA ·IEEEConst+624(SB)/8,$0x0000000027a20ab2
DATA ·IEEEConst+632(SB)/8,$0x00000001d7c88ebc

	/* x^220224 mod p(x), x^220160 mod p(x) */
DATA ·IEEEConst+640(SB)/8,$0x0000000114f87504
DATA ·IEEEConst+648(SB)/8,$0x00000001e7ca913a

	/* x^219200 mod p(x), x^219136 mod p(x) */
DATA ·IEEEConst+656(SB)/8,$0x000000004b076d96
DATA ·IEEEConst+664(SB)/8,$0x0000000033ed078a

	/* x^218176 mod p(x), x^218112 mod p(x) */
DATA ·IEEEConst+672(SB)/8,$0x00000000da4d1e74
DATA ·IEEEConst+680(SB)/8,$0x00000000e1839c78

	/* x^217152 mod p(x), x^217088 mod p(x) */
DATA ·IEEEConst+688(SB)/8,$0x000000001b81f672
DATA ·IEEEConst+696(SB)/8,$0x00000001322b267e

	/* x^216128 mod p(x), x^216064 mod p(x) */
DATA ·IEEEConst+704(SB)/8,$0x000000009367c988
DATA ·IEEEConst+712(SB)/8,$0x00000000638231b6

	/* x^215104 mod p(x), x^215040 mod p(x) */
DATA ·IEEEConst+720(SB)/8,$0x00000001717214ca
DATA ·IEEEConst+728(SB)/8,$0x00000001ee7f16f4

	/* x^214080 mod p(x), x^214016 mod p(x) */
DATA ·IEEEConst+736(SB)/8,$0x000000009f47d820
DATA ·IEEEConst+744(SB)/8,$0x0000000117d9924a

	/* x^213056 mod p(x), x^212992 mod p(x) */
DATA ·IEEEConst+752(SB)/8,$0x000000010d9a47d2
DATA ·IEEEConst+760(SB)/8,$0x00000000e1a9e0c4

	/* x^212032 mod p(x), x^211968 mod p(x) */
DATA ·IEEEConst+768(SB)/8,$0x00000000a696c58c
DATA ·IEEEConst+776(SB)/8,$0x00000001403731dc

	/* x^211008 mod p(x), x^210944 mod p(x) */
DATA ·IEEEConst+784(SB)/8,$0x000000002aa28ec6
DATA ·IEEEConst+792(SB)/8,$0x00000001a5ea9682

	/* x^209984 mod p(x), x^209920 mod p(x) */
DATA ·IEEEConst+800(SB)/8,$0x00000001fe18fd9a
DATA ·IEEEConst+808(SB)/8,$0x0000000101c5c578

	/* x^208960 mod p(x), x^208896 mod p(x) */
DATA ·IEEEConst+816(SB)/8,$0x000000019d4fc1ae
DATA ·IEEEConst+824(SB)/8,$0x00000000dddf6494

	/* x^207936 mod p(x), x^207872 mod p(x) */
DATA ·IEEEConst+832(SB)/8,$0x00000001ba0e3dea
DATA ·IEEEConst+840(SB)/8,$0x00000000f1c3db28

	/* x^206912 mod p(x), x^206848 mod p(x) */
DATA ·IEEEConst+848(SB)/8,$0x0000000074b59a5e
DATA ·IEEEConst+856(SB)/8,$0x000000013112fb9c

	/* x^205888 mod p(x), x^205824 mod p(x) */
DATA ·IEEEConst+864(SB)/8,$0x00000000f2b5ea98
DATA ·IEEEConst+872(SB)/8,$0x00000000b680b906

	/* x^204864 mod p(x), x^204800 mod p(x) */
DATA ·IEEEConst+880(SB)/8,$0x0000000187132676
DATA ·IEEEConst+888(SB)/8,$0x000000001a282932

	/* x^203840 mod p(x), x^203776 mod p(x) */
DATA ·IEEEConst+896(SB)/8,$0x000000010a8c6ad4
DATA ·IEEEConst+904(SB)/8,$0x0000000089406e7e

	/* x^202816 mod p(x), x^202752 mod p(x) */
DATA ·IEEEConst+912(SB)/8,$0x00000001e21dfe70
DATA ·IEEEConst+920(SB)/8,$0x00000001def6be8c

	/* x^201792 mod p(x), x^201728 mod p(x) */
DATA ·IEEEConst+928(SB)/8,$0x00000001da0050e4
DATA ·IEEEConst+936(SB)/8,$0x0000000075258728

	/* x^200768 mod p(x), x^200704 mod p(x) */
DATA ·IEEEConst+944(SB)/8,$0x00000000772172ae
DATA ·IEEEConst+952(SB)/8,$0x000000019536090a

	/* x^199744 mod p(x), x^199680 mod p(x) */
DATA ·IEEEConst+960(SB)/8,$0x00000000e47724aa
DATA ·IEEEConst+968(SB)/8,$0x00000000f2455bfc

	/* x^198720 mod p(x), x^198656 mod p(x) */
DATA ·IEEEConst+976(SB)/8,$0x000000003cd63ac4
DATA ·IEEEConst+984(SB)/8,$0x000000018c40baf4

	/* x^197696 mod p(x), x^197632 mod p(x) */
DATA ·IEEEConst+992(SB)/8,$0x00000001bf47d352
DATA ·IEEEConst+1000(SB)/8,$0x000000004cd390d4

	/* x^196672 mod p(x), x^196608 mod p(x) */
DATA ·IEEEConst+1008(SB)/8,$0x000000018dc1d708
DATA ·IEEEConst+1016(SB)/8,$0x00000001e4ece95a

	/* x^195648 mod p(x), x^195584 mod p(x) */
DATA ·IEEEConst+1024(SB)/8,$0x000000002d4620a4
DATA ·IEEEConst+1032(SB)/8,$0x000000001a3ee918

	/* x^194624 mod p(x), x^194560 mod p(x) */
DATA ·IEEEConst+1040(SB)/8,$0x0000000058fd1740
DATA ·IEEEConst+1048(SB)/8,$0x000000007c652fb8

	/* x^193600 mod p(x), x^193536 mod p(x) */
DATA ·IEEEConst+1056(SB)/8,$0x00000000dadd9bfc
DATA ·IEEEConst+1064(SB)/8,$0x000000011c67842c

	/* x^192576 mod p(x), x^192512 mod p(x) */
DATA ·IEEEConst+1072(SB)/8,$0x00000001ea2140be
DATA ·IEEEConst+1080(SB)/8,$0x00000000254f759c

	/* x^191552 mod p(x), x^191488 mod p(x) */
DATA ·IEEEConst+1088(SB)/8,$0x000000009de128ba
DATA ·IEEEConst+1096(SB)/8,$0x000000007ece94ca

	/* x^190528 mod p(x), x^190464 mod p(x) */
DATA ·IEEEConst+1104(SB)/8,$0x000000013ac3aa8e
DATA ·IEEEConst+1112(SB)/8,$0x0000000038f258c2

	/* x^189504 mod p(x), x^189440 mod p(x) */
DATA ·IEEEConst+1120(SB)/8,$0x0000000099980562
DATA ·IEEEConst+1128(SB)/8,$0x00000001cdf17b00

	/* x^188480 mod p(x), x^188416 mod p(x) */
DATA ·IEEEConst+1136(SB)/8,$0x00000001c1579c86
DATA ·IEEEConst+1144(SB)/8,$0x000000011f882c16

	/* x^187456 mod p(x), x^187392 mod p(x) */
DATA ·IEEEConst+1152(SB)/8,$0x0000000068dbbf94
DATA ·IEEEConst+1160(SB)/8,$0x0000000100093fc8

	/* x^186432 mod p(x), x^186368 mod p(x) */
DATA ·IEEEConst+1168(SB)/8,$0x000000004509fb04
DATA ·IEEEConst+1176(SB)/8,$0x00000001cd684f16

	/* x^185408 mod p(x), x^185344 mod p(x) */
DATA ·IEEEConst+1184(SB)/8,$0x00000001202f6398
DATA ·IEEEConst+1192(SB)/8,$0x000000004bc6a70a

	/* x^184384 mod p(x), x^184320 mod p(x) */
DATA ·IEEEConst+1200(SB)/8,$0x000000013aea243e
DATA ·IEEEConst+1208(SB)/8,$0x000000004fc7e8e4

	/* x^183360 mod p(x), x^183296 mod p(x) */
DATA ·IEEEConst+1216(SB)/8,$0x00000001b4052ae6
DATA ·IEEEConst+1224(SB)/8,$0x0000000130103f1c

	/* x^182336 mod p(x), x^182272 mod p(x) */
DATA ·IEEEConst+1232(SB)/8,$0x00000001cd2a0ae8
DATA ·IEEEConst+1240(SB)/8,$0x0000000111b0024c

	/* x^181312 mod p(x), x^181248 mod p(x) */
DATA ·IEEEConst+1248(SB)/8,$0x00000001fe4aa8b4
DATA ·IEEEConst+1256(SB)/8,$0x000000010b3079da

	/* x^180288 mod p(x), x^180224 mod p(x) */
DATA ·IEEEConst+1264(SB)/8,$0x00000001d1559a42
DATA ·IEEEConst+1272(SB)/8,$0x000000010192bcc2

	/* x^179264 mod p(x), x^179200 mod p(x) */
DATA ·IEEEConst+1280(SB)/8,$0x00000001f3e05ecc
DATA ·IEEEConst+1288(SB)/8,$0x0000000074838d50

	/* x^178240 mod p(x), x^178176 mod p(x) */
DATA ·IEEEConst+1296(SB)/8,$0x0000000104ddd2cc
DATA ·IEEEConst+1304(SB)/8,$0x000000001b20f520

	/* x^177216 mod p(x), x^177152 mod p(x) */
DATA ·IEEEConst+1312(SB)/8,$0x000000015393153c
DATA ·IEEEConst+1320(SB)/8,$0x0000000050c3590a

	/* x^176192 mod p(x), x^176128 mod p(x) */
DATA ·IEEEConst+1328(SB)/8,$0x0000000057e942c6
DATA ·IEEEConst+1336(SB)/8,$0x00000000b41cac8e

	/* x^175168 mod p(x), x^175104 mod p(x) */
DATA ·IEEEConst+1344(SB)/8,$0x000000012c633850
DATA ·IEEEConst+1352(SB)/8,$0x000000000c72cc78

	/* x^174144 mod p(x), x^174080 mod p(x) */
DATA ·IEEEConst+1360(SB)/8,$0x00000000ebcaae4c
DATA ·IEEEConst+1368(SB)/8,$0x0000000030cdb032

	/* x^173120 mod p(x), x^173056 mod p(x) */
DATA ·IEEEConst+1376(SB)/8,$0x000000013ee532a6
DATA ·IEEEConst+1384(SB)/8,$0x000000013e09fc32

	/* x^172096 mod p(x), x^172032 mod p(x) */
DATA ·IEEEConst+1392(SB)/8,$0x00000001bf0cbc7e
DATA ·IEEEConst+1400(SB)/8,$0x000000001ed624d2

	/* x^171072 mod p(x), x^171008 mod p(x) */
DATA ·IEEEConst+1408(SB)/8,$0x00000000d50b7a5a
DATA ·IEEEConst+1416(SB)/8,$0x00000000781aee1a

	/* x^170048 mod p(x), x^169984 mod p(x) */
DATA ·IEEEConst+1424(SB)/8,$0x0000000002fca6e8
DATA ·IEEEConst+1432(SB)/8,$0x00000001c4d8348c

	/* x^169024 mod p(x), x^168960 mod p(x) */
DATA ·IEEEConst+1440(SB)/8,$0x000000007af40044
DATA ·IEEEConst+1448(SB)/8,$0x0000000057a40336

	/* x^168000 mod p(x), x^167936 mod p(x) */
DATA ·IEEEConst+1456(SB)/8,$0x0000000016178744
DATA ·IEEEConst+1464(SB)/8,$0x0000000085544940

	/* x^166976 mod p(x), x^166912 mod p(x) */
DATA ·IEEEConst+1472(SB)/8,$0x000000014c177458
DATA ·IEEEConst+1480(SB)/8,$0x000000019cd21e80

	/* x^165952 mod p(x), x^165888 mod p(x) */
DATA ·IEEEConst+1488(SB)/8,$0x000000011b6ddf04
DATA ·IEEEConst+1496(SB)/8,$0x000000013eb95bc0

	/* x^164928 mod p(x), x^164864 mod p(x) */
DATA ·IEEEConst+1504(SB)/8,$0x00000001f3e29ccc
DATA ·IEEEConst+1512(SB)/8,$0x00000001dfc9fdfc

	/* x^163904 mod p(x), x^163840 mod p(x) */
DATA ·IEEEConst+1520(SB)/8,$0x0000000135ae7562
DATA ·IEEEConst+1528(SB)/8,$0x00000000cd028bc2

	/* x^162880 mod p(x), x^162816 mod p(x) */
DATA ·IEEEConst+1536(SB)/8,$0x0000000190ef812c
DATA ·IEEEConst+1544(SB)/8,$0x0000000090db8c44

	/* x^161856 mod p(x), x^161792 mod p(x) */
DATA ·IEEEConst+1552(SB)/8,$0x0000000067a2c786
DATA ·IEEEConst+1560(SB)/8,$0x000000010010a4ce

	/* x^160832 mod p(x), x^160768 mod p(x) */
DATA ·IEEEConst+1568(SB)/8,$0x0000000048b9496c
DATA ·IEEEConst+1576(SB)/8,$0x00000001c8f4c72c

	/* x^159808 mod p(x), x^159744 mod p(x) */
DATA ·IEEEConst+1584(SB)/8,$0x000000015a422de6
DATA ·IEEEConst+1592(SB)/8,$0x000000001c26170c

	/* x^158784 mod p(x), x^158720 mod p(x) */
DATA ·IEEEConst+1600(SB)/8,$0x00000001ef0e3640
DATA ·IEEEConst+1608(SB)/8,$0x00000000e3fccf68

	/* x^157760 mod p(x), x^157696 mod p(x) */
DATA ·IEEEConst+1616(SB)/8,$0x00000001006d2d26
DATA ·IEEEConst+1624(SB)/8,$0x00000000d513ed24

	/* x^156736 mod p(x), x^156672 mod p(x) */
DATA ·IEEEConst+1632(SB)/8,$0x00000001170d56d6
DATA ·IEEEConst+1640(SB)/8,$0x00000000141beada

	/* x^155712 mod p(x), x^155648 mod p(x) */
DATA ·IEEEConst+1648(SB)/8,$0x00000000a5fb613c
DATA ·IEEEConst+1656(SB)/8,$0x000000011071aea0

	/* x^154688 mod p(x), x^154624 mod p(x) */
DATA ·IEEEConst+1664(SB)/8,$0x0000000040bbf7fc
DATA ·IEEEConst+1672(SB)/8,$0x000000012e19080a

	/* x^153664 mod p(x), x^153600 mod p(x) */
DATA ·IEEEConst+1680(SB)/8,$0x000000016ac3a5b2
DATA ·IEEEConst+1688(SB)/8,$0x0000000100ecf826

	/* x^152640 mod p(x), x^152576 mod p(x) */
DATA ·IEEEConst+1696(SB)/8,$0x00000000abf16230
DATA ·IEEEConst+1704(SB)/8,$0x0000000069b09412

	/* x^151616 mod p(x), x^151552 mod p(x) */
DATA ·IEEEConst+1712(SB)/8,$0x00000001ebe23fac
DATA ·IEEEConst+1720(SB)/8,$0x0000000122297bac

	/* x^150592 mod p(x), x^150528 mod p(x) */
DATA ·IEEEConst+1728(SB)/8,$0x000000008b6a0894
DATA ·IEEEConst+1736(SB)/8,$0x00000000e9e4b068

	/* x^149568 mod p(x), x^149504 mod p(x) */
DATA ·IEEEConst+1744(SB)/8,$0x00000001288ea478
DATA ·IEEEConst+1752(SB)/8,$0x000000004b38651a

	/* x^148544 mod p(x), x^148480 mod p(x) */
DATA ·IEEEConst+1760(SB)/8,$0x000000016619c442
DATA ·IEEEConst+1768(SB)/8,$0x00000001468360e2

	/* x^147520 mod p(x), x^147456 mod p(x) */
DATA ·IEEEConst+1776(SB)/8,$0x0000000086230038
DATA ·IEEEConst+1784(SB)/8,$0x00000000121c2408

	/* x^146496 mod p(x), x^146432 mod p(x) */
DATA ·IEEEConst+1792(SB)/8,$0x000000017746a756
DATA ·IEEEConst+1800(SB)/8,$0x00000000da7e7d08

	/* x^145472 mod p(x), x^145408 mod p(x) */
DATA ·IEEEConst+1808(SB)/8,$0x0000000191b8f8f8
DATA ·IEEEConst+1816(SB)/8,$0x00000001058d7652

	/* x^144448 mod p(x), x^144384 mod p(x) */
DATA ·IEEEConst+1824(SB)/8,$0x000000008e167708
DATA ·IEEEConst+1832(SB)/8,$0x000000014a098a90

	/* x^143424 mod p(x), x^143360 mod p(x) */
DATA ·IEEEConst+1840(SB)/8,$0x0000000148b22d54
DATA ·IEEEConst+1848(SB)/8,$0x0000000020dbe72e

	/* x^142400 mod p(x), x^142336 mod p(x) */
DATA ·IEEEConst+1856(SB)/8,$0x0000000044ba2c3c
DATA ·IEEEConst+1864(SB)/8,$0x000000011e7323e8

	/* x^141376 mod p(x), x^141312 mod p(x) */
DATA ·IEEEConst+1872(SB)/8,$0x00000000b54d2b52
DATA ·IEEEConst+1880(SB)/8,$0x00000000d5d4bf94

	/* x^140352 mod p(x), x^140288 mod p(x) */
DATA ·IEEEConst+1888(SB)/8,$0x0000000005a4fd8a
DATA ·IEEEConst+1896(SB)/8,$0x0000000199d8746c

	/* x^139328 mod p(x), x^139264 mod p(x) */
DATA ·IEEEConst+1904(SB)/8,$0x0000000139f9fc46
DATA ·IEEEConst+1912(SB)/8,$0x00000000ce9ca8a0

	/* x^138304 mod p(x), x^138240 mod p(x) */
DATA ·IEEEConst+1920(SB)/8,$0x000000015a1fa824
DATA ·IEEEConst+1928(SB)/8,$0x00000000136edece

	/* x^137280 mod p(x), x^137216 mod p(x) */
DATA ·IEEEConst+1936(SB)/8,$0x000000000a61ae4c
DATA ·IEEEConst+1944(SB)/8,$0x000000019b92a068

	/* x^136256 mod p(x), x^136192 mod p(x) */
DATA ·IEEEConst+1952(SB)/8,$0x0000000145e9113e
DATA ·IEEEConst+1960(SB)/8,$0x0000000071d62206

	/* x^135232 mod p(x), x^135168 mod p(x) */
DATA ·IEEEConst+1968(SB)/8,$0x000000006a348448
DATA ·IEEEConst+1976(SB)/8,$0x00000000dfc50158

	/* x^134208 mod p(x), x^134144 mod p(x) */
DATA ·IEEEConst+1984(SB)/8,$0x000000004d80a08c
DATA ·IEEEConst+1992(SB)/8,$0x00000001517626bc

	/* x^133184 mod p(x), x^133120 mod p(x) */
DATA ·IEEEConst+2000(SB)/8,$0x000000014b6837a0
DATA ·IEEEConst+2008(SB)/8,$0x0000000148d1e4fa

	/* x^132160 mod p(x), x^132096 mod p(x) */
DATA ·IEEEConst+2016(SB)/8,$0x000000016896a7fc
DATA ·IEEEConst+2024(SB)/8,$0x0000000094d8266e

	/* x^131136 mod p(x), x^131072 mod p(x) */
DATA ·IEEEConst+2032(SB)/8,$0x000000014f187140
DATA ·IEEEConst+2040(SB)/8,$0x00000000606c5e34

	/* x^130112 mod p(x), x^130048 mod p(x) */
DATA ·IEEEConst+2048(SB)/8,$0x000000019581b9da
DATA ·IEEEConst+2056(SB)/8,$0x000000019766beaa

	/* x^129088 mod p(x), x^129024 mod p(x) */
DATA ·IEEEConst+2064(SB)/8,$0x00000001091bc984
DATA ·IEEEConst+2072(SB)/8,$0x00000001d80c506c

	/* x^128064 mod p(x), x^128000 mod p(x) */
DATA ·IEEEConst+2080(SB)/8,$0x000000001067223c
DATA ·IEEEConst+2088(SB)/8,$0x000000001e73837c

	/* x^127040 mod p(x), x^126976 mod p(x) */
DATA ·IEEEConst+2096(SB)/8,$0x00000001ab16ea02
DATA ·IEEEConst+2104(SB)/8,$0x0000000064d587de

	/* x^126016 mod p(x), x^125952 mod p(x) */
DATA ·IEEEConst+2112(SB)/8,$0x000000013c4598a8
DATA ·IEEEConst+2120(SB)/8,$0x00000000f4a507b0

	/* x^124992 mod p(x), x^124928 mod p(x) */
DATA ·IEEEConst+2128(SB)/8,$0x00000000b3735430
DATA ·IEEEConst+2136(SB)/8,$0x0000000040e342fc

	/* x^123968 mod p(x), x^123904 mod p(x) */
DATA ·IEEEConst+2144(SB)/8,$0x00000001bb3fc0c0
DATA ·IEEEConst+2152(SB)/8,$0x00000001d5ad9c3a

	/* x^122944 mod p(x), x^122880 mod p(x) */
DATA ·IEEEConst+2160(SB)/8,$0x00000001570ae19c
DATA ·IEEEConst+2168(SB)/8,$0x0000000094a691a4

	/* x^121920 mod p(x), x^121856 mod p(x) */
DATA ·IEEEConst+2176(SB)/8,$0x00000001ea910712
DATA ·IEEEConst+2184(SB)/8,$0x00000001271ecdfa

	/* x^120896 mod p(x), x^120832 mod p(x) */
DATA ·IEEEConst+2192(SB)/8,$0x0000000167127128
DATA ·IEEEConst+2200(SB)/8,$0x000000009e54475a

	/* x^119872 mod p(x), x^119808 mod p(x) */
DATA ·IEEEConst+2208(SB)/8,$0x0000000019e790a2
DATA ·IEEEConst+2216(SB)/8,$0x00000000c9c099ee

	/* x^118848 mod p(x), x^118784 mod p(x) */
DATA ·IEEEConst+2224(SB)/8,$0x000000003788f710
DATA ·IEEEConst+2232(SB)/8,$0x000000009a2f736c

	/* x^117824 mod p(x), x^117760 mod p(x) */
DATA ·IEEEConst+2240(SB)/8,$0x00000001682a160e
DATA ·IEEEConst+2248(SB)/8,$0x00000000bb9f4996

	/* x^116800 mod p(x), x^116736 mod p(x) */
DATA ·IEEEConst+2256(SB)/8,$0x000000007f0ebd2e
DATA ·IEEEConst+2264(SB)/8,$0x00000001db688050

	/* x^115776 mod p(x), x^115712 mod p(x) */
DATA ·IEEEConst+2272(SB)/8,$0x000000002b032080
DATA ·IEEEConst+2280(SB)/8,$0x00000000e9b10af4

	/* x^114752 mod p(x), x^114688 mod p(x) */
DATA ·IEEEConst+2288(SB)/8,$0x00000000cfd1664a
DATA ·IEEEConst+2296(SB)/8,$0x000000012d4545e4

	/* x^113728 mod p(x), x^113664 mod p(x) */
DATA ·IEEEConst+2304(SB)/8,$0x00000000aa1181c2
DATA ·IEEEConst+2312(SB)/8,$0x000000000361139c

	/* x^112704 mod p(x), x^112640 mod p(x) */
DATA ·IEEEConst+2320(SB)/8,$0x00000000ddd08002
DATA ·IEEEConst+2328(SB)/8,$0x00000001a5a1a3a8

	/* x^111680 mod p(x), x^111616 mod p(x) */
DATA ·IEEEConst+2336(SB)/8,$0x00000000e8dd0446
DATA ·IEEEConst+2344(SB)/8,$0x000000006844e0b0

	/* x^110656 mod p(x), x^110592 mod p(x) */
DATA ·IEEEConst+2352(SB)/8,$0x00000001bbd94a00
DATA ·IEEEConst+2360(SB)/8,$0x00000000c3762f28

	/* x^109632 mod p(x), x^109568 mod p(x) */
DATA ·IEEEConst+2368(SB)/8,$0x00000000ab6cd180
DATA ·IEEEConst+2376(SB)/8,$0x00000001d26287a2

	/* x^108608 mod p(x), x^108544 mod p(x) */
DATA ·IEEEConst+2384(SB)/8,$0x0000000031803ce2
DATA ·IEEEConst+2392(SB)/8,$0x00000001f6f0bba8

	/* x^107584 mod p(x), x^107520 mod p(x) */
DATA ·IEEEConst+2400(SB)/8,$0x0000000024f40b0c
DATA ·IEEEConst+2408(SB)/8,$0x000000002ffabd62

	/* x^106560 mod p(x), x^106496 mod p(x) */
DATA ·IEEEConst+2416(SB)/8,$0x00000001ba1d9834
DATA ·IEEEConst+2424(SB)/8,$0x00000000fb4516b8

	/* x^105536 mod p(x), x^105472 mod p(x) */
DATA ·IEEEConst+2432(SB)/8,$0x0000000104de61aa
DATA ·IEEEConst+2440(SB)/8,$0x000000018cfa961c

	/* x^104512 mod p(x), x^104448 mod p(x) */
DATA ·IEEEConst+2448(SB)/8,$0x0000000113e40d46
DATA ·IEEEConst+2456(SB)/8,$0x000000019e588d52

	/* x^103488 mod p(x), x^103424 mod p(x) */
DATA ·IEEEConst+2464(SB)/8,$0x00000001415598a0
DATA ·IEEEConst+2472(SB)/8,$0x00000001180f0bbc

	/* x^102464 mod p(x), x^102400 mod p(x) */
DATA ·IEEEConst+2480(SB)/8,$0x00000000bf6c8c90
DATA ·IEEEConst+2488(SB)/8,$0x00000000e1d9177a

	/* x^101440 mod p(x), x^101376 mod p(x) */
DATA ·IEEEConst+2496(SB)/8,$0x00000001788b0504
DATA ·IEEEConst+2504(SB)/8,$0x0000000105abc27c

	/* x^100416 mod p(x), x^100352 mod p(x) */
DATA ·IEEEConst+2512(SB)/8,$0x0000000038385d02
DATA ·IEEEConst+2520(SB)/8,$0x00000000972e4a58

	/* x^99392 mod p(x), x^99328 mod p(x) */
DATA ·IEEEConst+2528(SB)/8,$0x00000001b6c83844
DATA ·IEEEConst+2536(SB)/8,$0x0000000183499a5e

	/* x^98368 mod p(x), x^98304 mod p(x) */
DATA ·IEEEConst+2544(SB)/8,$0x0000000051061a8a
DATA ·IEEEConst+2552(SB)/8,$0x00000001c96a8cca

	/* x^97344 mod p(x), x^97280 mod p(x) */
DATA ·IEEEConst+2560(SB)/8,$0x000000017351388a
DATA ·IEEEConst+2568(SB)/8,$0x00000001a1a5b60c

	/* x^96320 mod p(x), x^96256 mod p(x) */
DATA ·IEEEConst+2576(SB)/8,$0x0000000132928f92
DATA ·IEEEConst+2584(SB)/8,$0x00000000e4b6ac9c

	/* x^95296 mod p(x), x^95232 mod p(x) */
DATA ·IEEEConst+2592(SB)/8,$0x00000000e6b4f48a
DATA ·IEEEConst+2600(SB)/8,$0x00000001807e7f5a

	/* x^94272 mod p(x), x^94208 mod p(x) */
DATA ·IEEEConst+2608(SB)/8,$0x0000000039d15e90
DATA ·IEEEConst+2616(SB)/8,$0x000000017a7e3bc8

	/* x^93248 mod p(x), x^93184 mod p(x) */
DATA ·IEEEConst+2624(SB)/8,$0x00000000312d6074
DATA ·IEEEConst+2632(SB)/8,$0x00000000d73975da

	/* x^92224 mod p(x), x^92160 mod p(x) */
DATA ·IEEEConst+2640(SB)/8,$0x000000017bbb2cc4
DATA ·IEEEConst+2648(SB)/8,$0x000000017375d038

	/* x^91200 mod p(x), x^91136 mod p(x) */
DATA ·IEEEConst+2656(SB)/8,$0x000000016ded3e18
DATA ·IEEEConst+2664(SB)/8,$0x00000000193680bc

	/* x^90176 mod p(x), x^90112 mod p(x) */
DATA ·IEEEConst+2672(SB)/8,$0x00000000f1638b16
DATA ·IEEEConst+2680(SB)/8,$0x00000000999b06f6

	/* x^89152 mod p(x), x^89088 mod p(x) */
DATA ·IEEEConst+2688(SB)/8,$0x00000001d38b9ecc
DATA ·IEEEConst+2696(SB)/8,$0x00000001f685d2b8

	/* x^88128 mod p(x), x^88064 mod p(x) */
DATA ·IEEEConst+2704(SB)/8,$0x000000018b8d09dc
DATA ·IEEEConst+2712(SB)/8,$0x00000001f4ecbed2

	/* x^87104 mod p(x), x^87040 mod p(x) */
DATA ·IEEEConst+2720(SB)/8,$0x00000000e7bc27d2
DATA ·IEEEConst+2728(SB)/8,$0x00000000ba16f1a0

	/* x^86080 mod p(x), x^86016 mod p(x) */
DATA ·IEEEConst+2736(SB)/8,$0x00000000275e1e96
DATA ·IEEEConst+2744(SB)/8,$0x0000000115aceac4

	/* x^85056 mod p(x), x^84992 mod p(x) */
DATA ·IEEEConst+2752(SB)/8,$0x00000000e2e3031e
DATA ·IEEEConst+2760(SB)/8,$0x00000001aeff6292

	/* x^84032 mod p(x), x^83968 mod p(x) */
DATA ·IEEEConst+2768(SB)/8,$0x00000001041c84d8
DATA ·IEEEConst+2776(SB)/8,$0x000000009640124c

	/* x^83008 mod p(x), x^82944 mod p(x) */
DATA ·IEEEConst+2784(SB)/8,$0x00000000706ce672
DATA ·IEEEConst+2792(SB)/8,$0x0000000114f41f02

	/* x^81984 mod p(x), x^81920 mod p(x) */
DATA ·IEEEConst+2800(SB)/8,$0x000000015d5070da
DATA ·IEEEConst+2808(SB)/8,$0x000000009c5f3586

	/* x^80960 mod p(x), x^80896 mod p(x) */
DATA ·IEEEConst+2816(SB)/8,$0x0000000038f9493a
DATA ·IEEEConst+2824(SB)/8,$0x00000001878275fa

	/* x^79936 mod p(x), x^79872 mod p(x) */
DATA ·IEEEConst+2832(SB)/8,$0x00000000a3348a76
DATA ·IEEEConst+2840(SB)/8,$0x00000000ddc42ce8

	/* x^78912 mod p(x), x^78848 mod p(x) */
DATA ·IEEEConst+2848(SB)/8,$0x00000001ad0aab92
DATA ·IEEEConst+2856(SB)/8,$0x0000000181d2c73a

	/* x^77888 mod p(x), x^77824 mod p(x) */
DATA ·IEEEConst+2864(SB)/8,$0x000000019e85f712
DATA ·IEEEConst+2872(SB)/8,$0x0000000141c9320a

	/* x^76864 mod p(x), x^76800 mod p(x) */
DATA ·IEEEConst+2880(SB)/8,$0x000000005a871e76
DATA ·IEEEConst+2888(SB)/8,$0x000000015235719a

	/* x^75840 mod p(x), x^75776 mod p(x) */
DATA ·IEEEConst+2896(SB)/8,$0x000000017249c662
DATA ·IEEEConst+2904(SB)/8,$0x00000000be27d804

	/* x^74816 mod p(x), x^74752 mod p(x) */
DATA ·IEEEConst+2912(SB)/8,$0x000000003a084712
DATA ·IEEEConst+2920(SB)/8,$0x000000006242d45a

	/* x^73792 mod p(x), x^73728 mod p(x) */
DATA ·IEEEConst+2928(SB)/8,$0x00000000ed438478
DATA ·IEEEConst+2936(SB)/8,$0x000000009a53638e

	/* x^72768 mod p(x), x^72704 mod p(x) */
DATA ·IEEEConst+2944(SB)/8,$0x00000000abac34cc
DATA ·IEEEConst+2952(SB)/8,$0x00000001001ecfb6

	/* x^71744 mod p(x), x^71680 mod p(x) */
DATA ·IEEEConst+2960(SB)/8,$0x000000005f35ef3e
DATA ·IEEEConst+2968(SB)/8,$0x000000016d7c2d64

	/* x^70720 mod p(x), x^70656 mod p(x) */
DATA ·IEEEConst+2976(SB)/8,$0x0000000047d6608c
DATA ·IEEEConst+2984(SB)/8,$0x00000001d0ce46c0

	/* x^69696 mod p(x), x^69632 mod p(x) */
DATA ·IEEEConst+2992(SB)/8,$0x000000002d01470e
DATA ·IEEEConst+3000(SB)/8,$0x0000000124c907b4

	/* x^68672 mod p(x), x^68608 mod p(x) */
DATA ·IEEEConst+3008(SB)/8,$0x0000000158bbc7b0
DATA ·IEEEConst+3016(SB)/8,$0x0000000018a555ca

	/* x^67648 mod p(x), x^67584 mod p(x) */
DATA ·IEEEConst+3024(SB)/8,$0x00000000c0a23e8e
DATA ·IEEEConst+3032(SB)/8,$0x000000006b0980bc

	/* x^66624 mod p(x), x^66560 mod p(x) */
DATA ·IEEEConst+3040(SB)/8,$0x00000001ebd85c88
DATA ·IEEEConst+3048(SB)/8,$0x000000008bbba964

	/* x^65600 mod p(x), x^65536 mod p(x) */
DATA ·IEEEConst+3056(SB)/8,$0x000000019ee20bb2
DATA ·IEEEConst+3064(SB)/8,$0x00000001070a5a1e

	/* x^64576 mod p(x), x^64512 mod p(x) */
DATA ·IEEEConst+3072(SB)/8,$0x00000001acabf2d6
DATA ·IEEEConst+3080(SB)/8,$0x000000002204322a

	/* x^63552 mod p(x), x^63488 mod p(x) */
DATA ·IEEEConst+3088(SB)/8,$0x00000001b7963d56
DATA ·IEEEConst+3096(SB)/8,$0x00000000a27524d0

	/* x^62528 mod p(x), x^62464 mod p(x) */
DATA ·IEEEConst+3104(SB)/8,$0x000000017bffa1fe
DATA ·IEEEConst+3112(SB)/8,$0x0000000020b1e4ba

	/* x^61504 mod p(x), x^61440 mod p(x) */
DATA ·IEEEConst+3120(SB)/8,$0x000000001f15333e
DATA ·IEEEConst+3128(SB)/8,$0x0000000032cc27fc

	/* x^60480 mod p(x), x^60416 mod p(x) */
DATA ·IEEEConst+3136(SB)/8,$0x000000018593129e
DATA ·IEEEConst+3144(SB)/8,$0x0000000044dd22b8

	/* x^59456 mod p(x), x^59392 mod p(x) */
DATA ·IEEEConst+3152(SB)/8,$0x000000019cb32602
DATA ·IEEEConst+3160(SB)/8,$0x00000000dffc9e0a

	/* x^58432 mod p(x), x^58368 mod p(x) */
DATA ·IEEEConst+3168(SB)/8,$0x0000000142b05cc8
DATA ·IEEEConst+3176(SB)/8,$0x00000001b7a0ed14

	/* x^57408 mod p(x), x^57344 mod p(x) */
DATA ·IEEEConst+3184(SB)/8,$0x00000001be49e7a4
DATA ·IEEEConst+3192(SB)/8,$0x00000000c7842488

	/* x^56384 mod p(x), x^56320 mod p(x) */
DATA ·IEEEConst+3200(SB)/8,$0x0000000108f69d6c
DATA ·IEEEConst+3208(SB)/8,$0x00000001c02a4fee

	/* x^55360 mod p(x), x^55296 mod p(x) */
DATA ·IEEEConst+3216(SB)/8,$0x000000006c0971f0
DATA ·IEEEConst+3224(SB)/8,$0x000000003c273778

	/* x^54336 mod p(x), x^54272 mod p(x) */
DATA ·IEEEConst+3232(SB)/8,$0x000000005b16467a
DATA ·IEEEConst+3240(SB)/8,$0x00000001d63f8894

	/* x^53312 mod p(x), x^53248 mod p(x) */
DATA ·IEEEConst+3248(SB)/8,$0x00000001551a628e
DATA ·IEEEConst+3256(SB)/8,$0x000000006be557d6

	/* x^52288 mod p(x), x^52224 mod p(x) */
DATA ·IEEEConst+3264(SB)/8,$0x000000019e42ea92
DATA ·IEEEConst+3272(SB)/8,$0x000000006a7806ea

	/* x^51264 mod p(x), x^51200 mod p(x) */
DATA ·IEEEConst+3280(SB)/8,$0x000000012fa83ff2
DATA ·IEEEConst+3288(SB)/8,$0x000000016155aa0c

	/* x^50240 mod p(x), x^50176 mod p(x) */
DATA ·IEEEConst+3296(SB)/8,$0x000000011ca9cde0
DATA ·IEEEConst+3304(SB)/8,$0x00000000908650ac

	/* x^49216 mod p(x), x^49152 mod p(x) */
DATA ·IEEEConst+3312(SB)/8,$0x00000000c8e5cd74
DATA ·IEEEConst+3320(SB)/8,$0x00000000aa5a8084

	/* x^48192 mod p(x), x^48128 mod p(x) */
DATA ·IEEEConst+3328(SB)/8,$0x0000000096c27f0c
DATA ·IEEEConst+3336(SB)/8,$0x0000000191bb500a

	/* x^47168 mod p(x), x^47104 mod p(x) */
DATA ·IEEEConst+3344(SB)/8,$0x000000002baed926
DATA ·IEEEConst+3352(SB)/8,$0x0000000064e9bed0

	/* x^46144 mod p(x), x^46080 mod p(x) */
DATA ·IEEEConst+3360(SB)/8,$0x000000017c8de8d2
DATA ·IEEEConst+3368(SB)/8,$0x000000009444f302

	/* x^45120 mod p(x), x^45056 mod p(x) */
DATA ·IEEEConst+3376(SB)/8,$0x00000000d43d6068
DATA ·IEEEConst+3384(SB)/8,$0x000000019db07d3c

	/* x^44096 mod p(x), x^44032 mod p(x) */
DATA ·IEEEConst+3392(SB)/8,$0x00000000cb2c4b26
DATA ·IEEEConst+3400(SB)/8,$0x00000001359e3e6e

	/* x^43072 mod p(x), x^43008 mod p(x) */
DATA ·IEEEConst+3408(SB)/8,$0x0000000145b8da26
DATA ·IEEEConst+3416(SB)/8,$0x00000001e4f10dd2

	/* x^42048 mod p(x), x^41984 mod p(x) */
DATA ·IEEEConst+3424(SB)/8,$0x000000018fff4b08
DATA ·IEEEConst+3432(SB)/8,$0x0000000124f5735e

	/* x^41024 mod p(x), x^40960 mod p(x) */
DATA ·IEEEConst+3440(SB)/8,$0x0000000150b58ed0
DATA ·IEEEConst+3448(SB)/8,$0x0000000124760a4c

	/* x^40000 mod p(x), x^39936 mod p(x) */
DATA ·IEEEConst+3456(SB)/8,$0x00000001549f39bc
DATA ·IEEEConst+3464(SB)/8,$0x000000000f1fc186

	/* x^38976 mod p(x), x^38912 mod p(x) */
DATA ·IEEEConst+3472(SB)/8,$0x00000000ef4d2f42
DATA ·IEEEConst+3480(SB)/8,$0x00000000150e4cc4

	/* x^37952 mod p(x), x^37888 mod p(x) */
DATA ·IEEEConst+3488(SB)/8,$0x00000001b1468572
DATA ·IEEEConst+3496(SB)/8,$0x000000002a6204e8

	/* x^36928 mod p(x), x^36864 mod p(x) */
DATA ·IEEEConst+3504(SB)/8,$0x000000013d7403b2
DATA ·IEEEConst+3512(SB)/8,$0x00000000beb1d432

	/* x^35904 mod p(x), x^35840 mod p(x) */
DATA ·IEEEConst+3520(SB)/8,$0x00000001a4681842
DATA ·IEEEConst+3528(SB)/8,$0x0000000135f3f1f0

	/* x^34880 mod p(x), x^34816 mod p(x) */
DATA ·IEEEConst+3536(SB)/8,$0x0000000167714492
DATA ·IEEEConst+3544(SB)/8,$0x0000000074fe2232

	/* x^33856 mod p(x), x^33792 mod p(x) */
DATA ·IEEEConst+3552(SB)/8,$0x00000001e599099a
DATA ·IEEEConst+3560(SB)/8,$0x000000001ac6e2ba

	/* x^32832 mod p(x), x^32768 mod p(x) */
DATA ·IEEEConst+3568(SB)/8,$0x00000000fe128194
DATA ·IEEEConst+3576(SB)/8,$0x0000000013fca91e

	/* x^31808 mod p(x), x^31744 mod p(x) */
DATA ·IEEEConst+3584(SB)/8,$0x0000000077e8b990
DATA ·IEEEConst+3592(SB)/8,$0x0000000183f4931e

	/* x^30784 mod p(x), x^30720 mod p(x) */
DATA ·IEEEConst+3600(SB)/8,$0x00000001a267f63a
DATA ·IEEEConst+3608(SB)/8,$0x00000000b6d9b4e4

	/* x^29760 mod p(x), x^29696 mod p(x) */
DATA ·IEEEConst+3616(SB)/8,$0x00000001945c245a
DATA ·IEEEConst+3624(SB)/8,$0x00000000b5188656

	/* x^28736 mod p(x), x^28672 mod p(x) */
DATA ·IEEEConst+3632(SB)/8,$0x0000000149002e76
DATA ·IEEEConst+3640(SB)/8,$0x0000000027a81a84

	/* x^27712 mod p(x), x^27648 mod p(x) */
DATA ·IEEEConst+3648(SB)/8,$0x00000001bb8310a4
DATA ·IEEEConst+3656(SB)/8,$0x0000000125699258

	/* x^26688 mod p(x), x^26624 mod p(x) */
DATA ·IEEEConst+3664(SB)/8,$0x000000019ec60bcc
DATA ·IEEEConst+3672(SB)/8,$0x00000001b23de796

	/* x^25664 mod p(x), x^25600 mod p(x) */
DATA ·IEEEConst+3680(SB)/8,$0x000000012d8590ae
DATA ·IEEEConst+3688(SB)/8,$0x00000000fe4365dc

	/* x^24640 mod p(x), x^24576 mod p(x) */
DATA ·IEEEConst+3696(SB)/8,$0x0000000065b00684
DATA ·IEEEConst+3704(SB)/8,$0x00000000c68f497a

	/* x^23616 mod p(x), x^23552 mod p(x) */
DATA ·IEEEConst+3712(SB)/8,$0x000000015e5aeadc
DATA ·IEEEConst+3720(SB)/8,$0x00000000fbf521ee

	/* x^22592 mod p(x), x^22528 mod p(x) */
DATA ·IEEEConst+3728(SB)/8,$0x00000000b77ff2b0
DATA ·IEEEConst+3736(SB)/8,$0x000000015eac3378

	/* x^21568 mod p(x), x^21504 mod p(x) */
DATA ·IEEEConst+3744(SB)/8,$0x0000000188da2ff6
DATA ·IEEEConst+3752(SB)/8,$0x0000000134914b90

	/* x^20544 mod p(x), x^20480 mod p(x) */
DATA ·IEEEConst+3760(SB)/8,$0x0000000063da929a
DATA ·IEEEConst+3768(SB)/8,$0x0000000016335cfe

	/* x^19520 mod p(x), x^19456 mod p(x) */
DATA ·IEEEConst+3776(SB)/8,$0x00000001389caa80
DATA ·IEEEConst+3784(SB)/8,$0x000000010372d10c

	/* x^18496 mod p(x), x^18432 mod p(x) */
DATA ·IEEEConst+3792(SB)/8,$0x000000013db599d2
DATA ·IEEEConst+3800(SB)/8,$0x000000015097b908

	/* x^17472 mod p(x), x^17408 mod p(x) */
DATA ·IEEEConst+3808(SB)/8,$0x0000000122505a86
DATA ·IEEEConst+3816(SB)/8,$0x00000001227a7572

	/* x^16448 mod p(x), x^16384 mod p(x) */
DATA ·IEEEConst+3824(SB)/8,$0x000000016bd72746
DATA ·IEEEConst+3832(SB)/8,$0x000000009a8f75c0

	/* x^15424 mod p(x), x^15360 mod p(x) */
DATA ·IEEEConst+3840(SB)/8,$0x00000001c3faf1d4
DATA ·IEEEConst+3848(SB)/8,$0x00000000682c77a2

	/* x^14400 mod p(x), x^14336 mod p(x) */
DATA ·IEEEConst+3856(SB)/8,$0x00000001111c826c
DATA ·IEEEConst+3864(SB)/8,$0x00000000231f091c

	/* x^13376 mod p(x), x^13312 mod p(x) */
DATA ·IEEEConst+3872(SB)/8,$0x00000000153e9fb2
DATA ·IEEEConst+3880(SB)/8,$0x000000007d4439f2

	/* x^12352 mod p(x), x^12288 mod p(x) */
DATA ·IEEEConst+3888(SB)/8,$0x000000002b1f7b60
DATA ·IEEEConst+3896(SB)/8,$0x000000017e221efc

	/* x^11328 mod p(x), x^11264 mod p(x) */
DATA ·IEEEConst+3904(SB)/8,$0x00000000b1dba570
DATA ·IEEEConst+3912(SB)/8,$0x0000000167457c38

	/* x^10304 mod p(x), x^10240 mod p(x) */
DATA ·IEEEConst+3920(SB)/8,$0x00000001f6397b76
DATA ·IEEEConst+3928(SB)/8,$0x00000000bdf081c4

	/* x^9280 mod p(x), x^9216 mod p(x) */
DATA ·IEEEConst+3936(SB)/8,$0x0000000156335214
DATA ·IEEEConst+3944(SB)/8,$0x000000016286d6b0

	/* x^8256 mod p(x), x^8192 mod p(x) */
DATA ·IEEEConst+3952(SB)/8,$0x00000001d70e3986
DATA ·IEEEConst+3960(SB)/8,$0x00000000c84f001c

	/* x^7232 mod p(x), x^7168 mod p(x) */
DATA ·IEEEConst+3968(SB)/8,$0x000000003701a774
DATA ·IEEEConst+3976(SB)/8,$0x0000000064efe7c0

	/* x^6208 mod p(x), x^6144 mod p(x) */
DATA ·IEEEConst+3984(SB)/8,$0x00000000ac81ef72
DATA ·IEEEConst+3992(SB)/8,$0x000000000ac2d904

	/* x^5184 mod p(x), x^5120 mod p(x) */
DATA ·IEEEConst+4000(SB)/8,$0x0000000133212464
DATA ·IEEEConst+4008(SB)/8,$0x00000000fd226d14

	/* x^4160 mod p(x), x^4096 mod p(x) */
DATA ·IEEEConst+4016(SB)/8,$0x00000000e4e45610
DATA ·IEEEConst+4024(SB)/8,$0x000000011cfd42e0

	/* x^3136 mod p(x), x^3072 mod p(x) */
DATA ·IEEEConst+4032(SB)/8,$0x000000000c1bd370
DATA ·IEEEConst+4040(SB)/8,$0x000000016e5a5678

	/* x^2112 mod p(x), x^2048 mod p(x) */
DATA ·IEEEConst+4048(SB)/8,$0x00000001a7b9e7a6
DATA ·IEEEConst+4056(SB)/8,$0x00000001d888fe22

	/* x^1088 mod p(x), x^1024 mod p(x) */
DATA ·IEEEConst+4064(SB)/8,$0x000000007d657a10
DATA ·IEEEConst+4072(SB)/8,$0x00000001af77fcd4

	/* x^2048 mod p(x), x^2016 mod p(x), x^1984 mod p(x), x^1952 mod p(x) */
DATA ·IEEEConst+4080(SB)/8,$0x99168a18ec447f11
DATA ·IEEEConst+4088(SB)/8,$0xed837b2613e8221e

	/* x^1920 mod p(x), x^1888 mod p(x), x^1856 mod p(x), x^1824 mod p(x) */
DATA ·IEEEConst+4096(SB)/8,$0xe23e954e8fd2cd3c
DATA ·IEEEConst+4104(SB)/8,$0xc8acdd8147b9ce5a

	/* x^1792 mod p(x), x^1760 mod p(x), x^1728 mod p(x), x^1696 mod p(x) */
DATA ·IEEEConst+4112(SB)/8,$0x92f8befe6b1d2b53
DATA ·IEEEConst+4120(SB)/8,$0xd9ad6d87d4277e25

	/* x^1664 mod p(x), x^1632 mod p(x), x^1600 mod p(x), x^1568 mod p(x) */
DATA ·IEEEConst+4128(SB)/8,$0xf38a3556291ea462
DATA ·IEEEConst+4136(SB)/8,$0xc10ec5e033fbca3b

	/* x^1536 mod p(x), x^1504 mod p(x), x^1472 mod p(x), x^1440 mod p(x) */
DATA ·IEEEConst+4144(SB)/8,$0x974ac56262b6ca4b
DATA ·IEEEConst+4152(SB)/8,$0xc0b55b0e82e02e2f

	/* x^1408 mod p(x), x^1376 mod p(x), x^1344 mod p(x), x^1312 mod p(x) */
DATA ·IEEEConst+4160(SB)/8,$0x855712b3784d2a56
DATA ·IEEEConst+4168(SB)/8,$0x71aa1df0e172334d

	/* x^1280 mod p(x), x^1248 mod p(x), x^1216 mod p(x), x^1184 mod p(x) */
DATA ·IEEEConst+4176(SB)/8,$0xa5abe9f80eaee722
DATA ·IEEEConst+4184(SB)/8,$0xfee3053e3969324d

	/* x^1152 mod p(x), x^1120 mod p(x), x^1088 mod p(x), x^1056 mod p(x) */
DATA ·IEEEConst+4192(SB)/8,$0x1fa0943ddb54814c
DATA ·IEEEConst+4200(SB)/8,$0xf44779b93eb2bd08

	/* x^1024 mod p(x), x^992 mod p(x), x^960 mod p(x), x^928 mod p(x) */
DATA ·IEEEConst+4208(SB)/8,$0xa53ff440d7bbfe6a
DATA ·IEEEConst+4216(SB)/8,$0xf5449b3f00cc3374

	/* x^896 mod p(x), x^864 mod p(x), x^832 mod p(x), x^800 mod p(x) */
DATA ·IEEEConst+4224(SB)/8,$0xebe7e3566325605c
DATA ·IEEEConst+4232(SB)/8,$0x6f8346e1d777606e

	/* x^768 mod p(x), x^736 mod p(x), x^704 mod p(x), x^672 mod p(x) */
DATA ·IEEEConst+4240(SB)/8,$0xc65a272ce5b592b8
DATA ·IEEEConst+4248(SB)/8,$0xe3ab4f2ac0b95347

	/* x^640 mod p(x), x^608 mod p(x), x^576 mod p(x), x^544 mod p(x) */
DATA ·IEEEConst+4256(SB)/8,$0x5705a9ca4721589f
DATA ·IEEEConst+4264(SB)/8,$0xaa2215ea329ecc11

	/* x^512 mod p(x), x^480 mod p(x), x^448 mod p(x), x^416 mod p(x) */
DATA ·IEEEConst+4272(SB)/8,$0xe3720acb88d14467
DATA ·IEEEConst+4280(SB)/8,$0x1ed8f66ed95efd26

	/* x^384 mod p(x), x^352 mod p(x), x^320 mod p(x), x^288 mod p(x) */
DATA ·IEEEConst+4288(SB)/8,$0xba1aca0315141c31
DATA ·IEEEConst+4296(SB)/8,$0x78ed02d5a700e96a

	/* x^256 mod p(x), x^224 mod p(x), x^192 mod p(x), x^160 mod p(x) */
DATA ·IEEEConst+4304(SB)/8,$0xad2a31b3ed627dae
DATA ·IEEEConst+4312(SB)/8,$0xba8ccbe832b39da3

	/* x^128 mod p(x), x^96 mod p(x), x^64 mod p(x), x^32 mod p(x) */
DATA ·IEEEConst+4320(SB)/8,$0x6655004fa06a2517
DATA ·IEEEConst+4328(SB)/8,$0xedb88320b1e6b092

GLOBL ·IEEEConst(SB),RODATA,$4336

	/* Barrett constant m - (4^32)/n */
DATA ·IEEEBarConst(SB)/8,$0x00000001f7011641
DATA ·IEEEBarConst+8(SB)/8,$0x0000000000000000
DATA ·IEEEBarConst+16(SB)/8,$0x00000001db710641
DATA ·IEEEBarConst+24(SB)/8,$0x0000000000000000
GLOBL ·IEEEBarConst(SB),RODATA,$32

	/* Reduce 262144 kbits to 1024 bits */
	/* x^261184 mod p(x), x^261120 mod p(x) */
DATA ·CastConst+0(SB)/8,$0x000000009c37c408
DATA ·CastConst+8(SB)/8,$0x00000000b6ca9e20

	/* x^260160 mod p(x), x^260096 mod p(x) */
DATA ·CastConst+16(SB)/8,$0x00000001b51df26c
DATA ·CastConst+24(SB)/8,$0x00000000350249a8

	/* x^259136 mod p(x), x^259072 mod p(x) */
DATA ·CastConst+32(SB)/8,$0x000000000724b9d0
DATA ·CastConst+40(SB)/8,$0x00000001862dac54

	/* x^258112 mod p(x), x^258048 mod p(x) */
DATA ·CastConst+48(SB)/8,$0x00000001c00532fe
DATA ·CastConst+56(SB)/8,$0x00000001d87fb48c

	/* x^257088 mod p(x), x^257024 mod p(x) */
DATA ·CastConst+64(SB)/8,$0x00000000f05a9362
DATA ·CastConst+72(SB)/8,$0x00000001f39b699e

	/* x^256064 mod p(x), x^256000 mod p(x) */
DATA ·CastConst+80(SB)/8,$0x00000001e1007970
DATA ·CastConst+88(SB)/8,$0x0000000101da11b4

	/* x^255040 mod p(x), x^254976 mod p(x) */
DATA ·CastConst+96(SB)/8,$0x00000000a57366ee
DATA ·CastConst+104(SB)/8,$0x00000001cab571e0

	/* x^254016 mod p(x), x^253952 mod p(x) */
DATA ·CastConst+112(SB)/8,$0x0000000192011284
DATA ·CastConst+120(SB)/8,$0x00000000c7020cfe

	/* x^252992 mod p(x), x^252928 mod p(x) */
DATA ·CastConst+128(SB)/8,$0x0000000162716d9a
DATA ·CastConst+136(SB)/8,$0x00000000cdaed1ae

	/* x^251968 mod p(x), x^251904 mod p(x) */
DATA ·CastConst+144(SB)/8,$0x00000000cd97ecde
DATA ·CastConst+152(SB)/8,$0x00000001e804effc

	/* x^250944 mod p(x), x^250880 mod p(x) */
DATA ·CastConst+160(SB)/8,$0x0000000058812bc0
DATA ·CastConst+168(SB)/8,$0x0000000077c3ea3a

	/* x^249920 mod p(x), x^249856 mod p(x) */
DATA ·CastConst+176(SB)/8,$0x0000000088b8c12e
DATA ·CastConst+184(SB)/8,$0x0000000068df31b4

	/* x^248896 mod p(x), x^248832 mod p(x) */
DATA ·CastConst+192(SB)/8,$0x00000001230b234c
DATA ·CastConst+200(SB)/8,$0x00000000b059b6c2

	/* x^247872 mod p(x), x^247808 mod p(x) */
DATA ·CastConst+208(SB)/8,$0x00000001120b416e
DATA ·CastConst+216(SB)/8,$0x0000000145fb8ed8

	/* x^246848 mod p(x), x^246784 mod p(x) */
DATA ·CastConst+224(SB)/8,$0x00000001974aecb0
DATA ·CastConst+232(SB)/8,$0x00000000cbc09168

	/* x^245824 mod p(x), x^245760 mod p(x) */
DATA ·CastConst+240(SB)/8,$0x000000008ee3f226
DATA ·CastConst+248(SB)/8,$0x000000005ceeedc2

	/* x^244800 mod p(x), x^244736 mod p(x) */
DATA ·CastConst+256(SB)/8,$0x00000001089aba9a
DATA ·CastConst+264(SB)/8,$0x0000000047d74e86

	/* x^243776 mod p(x), x^243712 mod p(x) */
DATA ·CastConst+272(SB)/8,$0x0000000065113872
DATA ·CastConst+280(SB)/8,$0x00000001407e9e22

	/* x^242752 mod p(x), x^242688 mod p(x) */
DATA ·CastConst+288(SB)/8,$0x000000005c07ec10
DATA ·CastConst+296(SB)/8,$0x00000001da967bda

	/* x^241728 mod p(x), x^241664 mod p(x) */
DATA ·CastConst+304(SB)/8,$0x0000000187590924
DATA ·CastConst+312(SB)/8,$0x000000006c898368

	/* x^240704 mod p(x), x^240640 mod p(x) */
DATA ·CastConst+320(SB)/8,$0x00000000e35da7c6
DATA ·CastConst+328(SB)/8,$0x00000000f2d14c98

	/* x^239680 mod p(x), x^239616 mod p(x) */
DATA ·CastConst+336(SB)/8,$0x000000000415855a
DATA ·CastConst+344(SB)/8,$0x00000001993c6ad4

	/* x^238656 mod p(x), x^238592 mod p(x) */
DATA ·CastConst+352(SB)/8,$0x0000000073617758
DATA ·CastConst+360(SB)/8,$0x000000014683d1ac

	/* x^237632 mod p(x), x^237568 mod p(x) */
DATA ·CastConst+368(SB)/8,$0x0000000176021d28
DATA ·CastConst+376(SB)/8,$0x00000001a7c93e6c

	/* x^236608 mod p(x), x^236544 mod p(x) */
DATA ·CastConst+384(SB)/8,$0x00000001c358fd0a
DATA ·CastConst+392(SB)/8,$0x000000010211e90a

	/* x^235584 mod p(x), x^235520 mod p(x) */
DATA ·CastConst+400(SB)/8,$0x00000001ff7a2c18
DATA ·CastConst+408(SB)/8,$0x000000001119403e

	/* x^234560 mod p(x), x^234496 mod p(x) */
DATA ·CastConst+416(SB)/8,$0x00000000f2d9f7e4
DATA ·CastConst+424(SB)/8,$0x000000001c3261aa

	/* x^233536 mod p(x), x^233472 mod p(x) */
DATA ·CastConst+432(SB)/8,$0x000000016cf1f9c8
DATA ·CastConst+440(SB)/8,$0x000000014e37a634

	/* x^232512 mod p(x), x^232448 mod p(x) */
DATA ·CastConst+448(SB)/8,$0x000000010af9279a
DATA ·CastConst+456(SB)/8,$0x0000000073786c0c

	/* x^231488 mod p(x), x^231424 mod p(x) */
DATA ·CastConst+464(SB)/8,$0x0000000004f101e8
DATA ·CastConst+472(SB)/8,$0x000000011dc037f8

	/* x^230464 mod p(x), x^230400 mod p(x) */
DATA ·CastConst+480(SB)/8,$0x0000000070bcf184
DATA ·CastConst+488(SB)/8,$0x0000000031433dfc

	/* x^229440 mod p(x), x^229376 mod p(x) */
DATA ·CastConst+496(SB)/8,$0x000000000a8de642
DATA ·CastConst+504(SB)/8,$0x000000009cde8348

	/* x^228416 mod p(x), x^228352 mod p(x) */
DATA ·CastConst+512(SB)/8,$0x0000000062ea130c
DATA ·CastConst+520(SB)/8,$0x0000000038d3c2a6

	/* x^227392 mod p(x), x^227328 mod p(x) */
DATA ·CastConst+528(SB)/8,$0x00000001eb31cbb2
DATA ·CastConst+536(SB)/8,$0x000000011b25f260

	/* x^226368 mod p(x), x^226304 mod p(x) */
DATA ·CastConst+544(SB)/8,$0x0000000170783448
DATA ·CastConst+552(SB)/8,$0x000000001629e6f0

	/* x^225344 mod p(x), x^225280 mod p(x) */
DATA ·CastConst+560(SB)/8,$0x00000001a684b4c6
DATA ·CastConst+568(SB)/8,$0x0000000160838b4c

	/* x^224320 mod p(x), x^224256 mod p(x) */
DATA ·CastConst+576(SB)/8,$0x00000000253ca5b4
DATA ·CastConst+584(SB)/8,$0x000000007a44011c

	/* x^223296 mod p(x), x^223232 mod p(x) */
DATA ·CastConst+592(SB)/8,$0x0000000057b4b1e2
DATA ·CastConst+600(SB)/8,$0x00000000226f417a

	/* x^222272 mod p(x), x^222208 mod p(x) */
DATA ·CastConst+608(SB)/8,$0x00000000b6bd084c
DATA ·CastConst+616(SB)/8,$0x0000000045eb2eb4

	/* x^221248 mod p(x), x^221184 mod p(x) */
DATA ·CastConst+624(SB)/8,$0x0000000123c2d592
DATA ·CastConst+632(SB)/8,$0x000000014459d70c

	/* x^220224 mod p(x), x^220160 mod p(x) */
DATA ·CastConst+640(SB)/8,$0x00000000159dafce
DATA ·CastConst+648(SB)/8,$0x00000001d406ed82

	/* x^219200 mod p(x), x^219136 mod p(x) */
DATA ·CastConst+656(SB)/8,$0x0000000127e1a64e
DATA ·CastConst+664(SB)/8,$0x0000000160c8e1a8

	/* x^218176 mod p(x), x^218112 mod p(x) */
DATA ·CastConst+672(SB)/8,$0x0000000056860754
DATA ·CastConst+680(SB)/8,$0x0000000027ba8098

	/* x^217152 mod p(x), x^217088 mod p(x) */
DATA ·CastConst+688(SB)/8,$0x00000001e661aae8
DATA ·CastConst+696(SB)/8,$0x000000006d92d018

	/* x^216128 mod p(x), x^216064 mod p(x) */
DATA ·CastConst+704(SB)/8,$0x00000000f82c6166
DATA ·CastConst+712(SB)/8,$0x000000012ed7e3f2

	/* x^215104 mod p(x), x^215040 mod p(x) */
DATA ·CastConst+720(SB)/8,$0x00000000c4f9c7ae
DATA ·CastConst+728(SB)/8,$0x000000002dc87788

	/* x^214080 mod p(x), x^214016 mod p(x) */
DATA ·CastConst+736(SB)/8,$0x0000000074203d20
DATA ·CastConst+744(SB)/8,$0x0000000018240bb8

	/* x^213056 mod p(x), x^212992 mod p(x) */
DATA ·CastConst+752(SB)/8,$0x0000000198173052
DATA ·CastConst+760(SB)/8,$0x000000001ad38158

	/* x^212032 mod p(x), x^211968 mod p(x) */
DATA ·CastConst+768(SB)/8,$0x00000001ce8aba54
DATA ·CastConst+776(SB)/8,$0x00000001396b78f2

	/* x^211008 mod p(x), x^210944 mod p(x) */
DATA ·CastConst+784(SB)/8,$0x00000001850d5d94
DATA ·CastConst+792(SB)/8,$0x000000011a681334

	/* x^209984 mod p(x), x^209920 mod p(x) */
DATA ·CastConst+800(SB)/8,$0x00000001d609239c
DATA ·CastConst+808(SB)/8,$0x000000012104732e

	/* x^208960 mod p(x), x^208896 mod p(x) */
DATA ·CastConst+816(SB)/8,$0x000000001595f048
DATA ·CastConst+824(SB)/8,$0x00000000a140d90c

	/* x^207936 mod p(x), x^207872 mod p(x) */
DATA ·CastConst+832(SB)/8,$0x0000000042ccee08
DATA ·CastConst+840(SB)/8,$0x00000001b7215eda

	/* x^206912 mod p(x), x^206848 mod p(x) */
DATA ·CastConst+848(SB)/8,$0x000000010a389d74
DATA ·CastConst+856(SB)/8,$0x00000001aaf1df3c

	/* x^205888 mod p(x), x^205824 mod p(x) */
DATA ·CastConst+864(SB)/8,$0x000000012a840da6
DATA ·CastConst+872(SB)/8,$0x0000000029d15b8a

	/* x^204864 mod p(x), x^204800 mod p(x) */
DATA ·CastConst+880(SB)/8,$0x000000001d181c0c
DATA ·CastConst+888(SB)/8,$0x00000000f1a96922

	/* x^203840 mod p(x), x^203776 mod p(x) */
DATA ·CastConst+896(SB)/8,$0x0000000068b7d1f6
DATA ·CastConst+904(SB)/8,$0x00000001ac80d03c

	/* x^202816 mod p(x), x^202752 mod p(x) */
DATA ·CastConst+912(SB)/8,$0x000000005b0f14fc
DATA ·CastConst+920(SB)/8,$0x000000000f11d56a

	/* x^201792 mod p(x), x^201728 mod p(x) */
DATA ·CastConst+928(SB)/8,$0x0000000179e9e730
DATA ·CastConst+936(SB)/8,$0x00000001f1c022a2

	/* x^200768 mod p(x), x^200704 mod p(x) */
DATA ·CastConst+944(SB)/8,$0x00000001ce1368d6
DATA ·CastConst+952(SB)/8,$0x0000000173d00ae2

	/* x^199744 mod p(x), x^199680 mod p(x) */
DATA ·CastConst+960(SB)/8,$0x0000000112c3a84c
DATA ·CastConst+968(SB)/8,$0x00000001d4ffe4ac

	/* x^198720 mod p(x), x^198656 mod p(x) */
DATA ·CastConst+976(SB)/8,$0x00000000de940fee
DATA ·CastConst+984(SB)/8,$0x000000016edc5ae4

	/* x^197696 mod p(x), x^197632 mod p(x) */
DATA ·CastConst+992(SB)/8,$0x00000000fe896b7e
DATA ·CastConst+1000(SB)/8,$0x00000001f1a02140

	/* x^196672 mod p(x), x^196608 mod p(x) */
DATA ·CastConst+1008(SB)/8,$0x00000001f797431c
DATA ·CastConst+1016(SB)/8,$0x00000000ca0b28a0

	/* x^195648 mod p(x), x^195584 mod p(x) */
DATA ·CastConst+1024(SB)/8,$0x0000000053e989ba
DATA ·CastConst+1032(SB)/8,$0x00000001928e30a2

	/* x^194624 mod p(x), x^194560 mod p(x) */
DATA ·CastConst+1040(SB)/8,$0x000000003920cd16
DATA ·CastConst+1048(SB)/8,$0x0000000097b1b002

	/* x^193600 mod p(x), x^193536 mod p(x) */
DATA ·CastConst+1056(SB)/8,$0x00000001e6f579b8
DATA ·CastConst+1064(SB)/8,$0x00000000b15bf906

	/* x^192576 mod p(x), x^192512 mod p(x) */
DATA ·CastConst+1072(SB)/8,$0x000000007493cb0a
DATA ·CastConst+1080(SB)/8,$0x00000000411c5d52

	/* x^191552 mod p(x), x^191488 mod p(x) */
DATA ·CastConst+1088(SB)/8,$0x00000001bdd376d8
DATA ·CastConst+1096(SB)/8,$0x00000001c36f3300

	/* x^190528 mod p(x), x^190464 mod p(x) */
DATA ·CastConst+1104(SB)/8,$0x000000016badfee6
DATA ·CastConst+1112(SB)/8,$0x00000001119227e0

	/* x^189504 mod p(x), x^189440 mod p(x) */
DATA ·CastConst+1120(SB)/8,$0x0000000071de5c58
DATA ·CastConst+1128(SB)/8,$0x00000000114d4702

	/* x^188480 mod p(x), x^188416 mod p(x) */
DATA ·CastConst+1136(SB)/8,$0x00000000453f317c
DATA ·CastConst+1144(SB)/8,$0x00000000458b5b98

	/* x^187456 mod p(x), x^187392 mod p(x) */
DATA ·CastConst+1152(SB)/8,$0x0000000121675cce
DATA ·CastConst+1160(SB)/8,$0x000000012e31fb8e

	/* x^186432 mod p(x), x^186368 mod p(x) */
DATA ·CastConst+1168(SB)/8,$0x00000001f409ee92
DATA ·CastConst+1176(SB)/8,$0x000000005cf619d8

	/* x^185408 mod p(x), x^185344 mod p(x) */
DATA ·CastConst+1184(SB)/8,$0x00000000f36b9c88
DATA ·CastConst+1192(SB)/8,$0x0000000063f4d8b2

	/* x^184384 mod p(x), x^184320 mod p(x) */
DATA ·CastConst+1200(SB)/8,$0x0000000036b398f4
DATA ·CastConst+1208(SB)/8,$0x000000004138dc8a

	/* x^183360 mod p(x), x^183296 mod p(x) */
DATA ·CastConst+1216(SB)/8,$0x00000001748f9adc
DATA ·CastConst+1224(SB)/8,$0x00000001d29ee8e0

	/* x^182336 mod p(x), x^182272 mod p(x) */
DATA ·CastConst+1232(SB)/8,$0x00000001be94ec00
DATA ·CastConst+1240(SB)/8,$0x000000006a08ace8

	/* x^181312 mod p(x), x^181248 mod p(x) */
DATA ·CastConst+1248(SB)/8,$0x00000000b74370d6
DATA ·CastConst+1256(SB)/8,$0x0000000127d42010

	/* x^180288 mod p(x), x^180224 mod p(x) */
DATA ·CastConst+1264(SB)/8,$0x00000001174d0b98
DATA ·CastConst+1272(SB)/8,$0x0000000019d76b62

	/* x^179264 mod p(x), x^179200 mod p(x) */
DATA ·CastConst+1280(SB)/8,$0x00000000befc06a4
DATA ·CastConst+1288(SB)/8,$0x00000001b1471f6e

	/* x^178240 mod p(x), x^178176 mod p(x) */
DATA ·CastConst+1296(SB)/8,$0x00000001ae125288
DATA ·CastConst+1304(SB)/8,$0x00000001f64c19cc

	/* x^177216 mod p(x), x^177152 mod p(x) */
DATA ·CastConst+1312(SB)/8,$0x0000000095c19b34
DATA ·CastConst+1320(SB)/8,$0x00000000003c0ea0

	/* x^176192 mod p(x), x^176128 mod p(x) */
DATA ·CastConst+1328(SB)/8,$0x00000001a78496f2
DATA ·CastConst+1336(SB)/8,$0x000000014d73abf6

	/* x^175168 mod p(x), x^175104 mod p(x) */
DATA ·CastConst+1344(SB)/8,$0x00000001ac5390a0
DATA ·CastConst+1352(SB)/8,$0x00000001620eb844

	/* x^174144 mod p(x), x^174080 mod p(x) */
DATA ·CastConst+1360(SB)/8,$0x000000002a80ed6e
DATA ·CastConst+1368(SB)/8,$0x0000000147655048

	/* x^173120 mod p(x), x^173056 mod p(x) */
DATA ·CastConst+1376(SB)/8,$0x00000001fa9b0128
DATA ·CastConst+1384(SB)/8,$0x0000000067b5077e

	/* x^172096 mod p(x), x^172032 mod p(x) */
DATA ·CastConst+1392(SB)/8,$0x00000001ea94929e
DATA ·CastConst+1400(SB)/8,$0x0000000010ffe206

	/* x^171072 mod p(x), x^171008 mod p(x) */
DATA ·CastConst+1408(SB)/8,$0x0000000125f4305c
DATA ·CastConst+1416(SB)/8,$0x000000000fee8f1e

	/* x^170048 mod p(x), x^169984 mod p(x) */
DATA ·CastConst+1424(SB)/8,$0x00000001471e2002
DATA ·CastConst+1432(SB)/8,$0x00000001da26fbae

	/* x^169024 mod p(x), x^168960 mod p(x) */
DATA ·CastConst+1440(SB)/8,$0x0000000132d2253a
DATA ·CastConst+1448(SB)/8,$0x00000001b3a8bd88

	/* x^168000 mod p(x), x^167936 mod p(x) */
DATA ·CastConst+1456(SB)/8,$0x00000000f26b3592
DATA ·CastConst+1464(SB)/8,$0x00000000e8f3898e

	/* x^166976 mod p(x), x^166912 mod p(x) */
DATA ·CastConst+1472(SB)/8,$0x00000000bc8b67b0
DATA ·CastConst+1480(SB)/8,$0x00000000b0d0d28c

	/* x^165952 mod p(x), x^165888 mod p(x) */
DATA ·CastConst+1488(SB)/8,$0x000000013a826ef2
DATA ·CastConst+1496(SB)/8,$0x0000000030f2a798

	/* x^164928 mod p(x), x^164864 mod p(x) */
DATA ·CastConst+1504(SB)/8,$0x0000000081482c84
DATA ·CastConst+1512(SB)/8,$0x000000000fba1002

	/* x^163904 mod p(x), x^163840 mod p(x) */
DATA ·CastConst+1520(SB)/8,$0x00000000e77307c2
DATA ·CastConst+1528(SB)/8,$0x00000000bdb9bd72

	/* x^162880 mod p(x), x^162816 mod p(x) */
DATA ·CastConst+1536(SB)/8,$0x00000000d4a07ec8
DATA ·CastConst+1544(SB)/8,$0x0000000075d3bf5a

	/* x^161856 mod p(x), x^161792 mod p(x) */
DATA ·CastConst+1552(SB)/8,$0x0000000017102100
DATA ·CastConst+1560(SB)/8,$0x00000000ef1f98a0

	/* x^160832 mod p(x), x^160768 mod p(x) */
DATA ·CastConst+1568(SB)/8,$0x00000000db406486
DATA ·CastConst+1576(SB)/8,$0x00000000689c7602

	/* x^159808 mod p(x), x^159744 mod p(x) */
DATA ·CastConst+1584(SB)/8,$0x0000000192db7f88
DATA ·CastConst+1592(SB)/8,$0x000000016d5fa5fe

	/* x^158784 mod p(x), x^158720 mod p(x) */
DATA ·CastConst+1600(SB)/8,$0x000000018bf67b1e
DATA ·CastConst+1608(SB)/8,$0x00000001d0d2b9ca

	/* x^157760 mod p(x), x^157696 mod p(x) */
DATA ·CastConst+1616(SB)/8,$0x000000007c09163e
DATA ·CastConst+1624(SB)/8,$0x0000000041e7b470

	/* x^156736 mod p(x), x^156672 mod p(x) */
DATA ·CastConst+1632(SB)/8,$0x000000000adac060
DATA ·CastConst+1640(SB)/8,$0x00000001cbb6495e

	/* x^155712 mod p(x), x^155648 mod p(x) */
DATA ·CastConst+1648(SB)/8,$0x00000000bd8316ae
DATA ·CastConst+1656(SB)/8,$0x000000010052a0b0

	/* x^154688 mod p(x), x^154624 mod p(x) */
DATA ·CastConst+1664(SB)/8,$0x000000019f09ab54
DATA ·CastConst+1672(SB)/8,$0x00000001d8effb5c

	/* x^153664 mod p(x), x^153600 mod p(x) */
DATA ·CastConst+1680(SB)/8,$0x0000000125155542
DATA ·CastConst+1688(SB)/8,$0x00000001d969853c

	/* x^152640 mod p(x), x^152576 mod p(x) */
DATA ·CastConst+1696(SB)/8,$0x000000018fdb5882
DATA ·CastConst+1704(SB)/8,$0x00000000523ccce2

	/* x^151616 mod p(x), x^151552 mod p(x) */
DATA ·CastConst+1712(SB)/8,$0x00000000e794b3f4
DATA ·CastConst+1720(SB)/8,$0x000000001e2436bc

	/* x^150592 mod p(x), x^150528 mod p(x) */
DATA ·CastConst+1728(SB)/8,$0x000000016f9bb022
DATA ·CastConst+1736(SB)/8,$0x00000000ddd1c3a2

	/* x^149568 mod p(x), x^149504 mod p(x) */
DATA ·CastConst+1744(SB)/8,$0x00000000290c9978
DATA ·CastConst+1752(SB)/8,$0x0000000019fcfe38

	/* x^148544 mod p(x), x^148480 mod p(x) */
DATA ·CastConst+1760(SB)/8,$0x0000000083c0f350
DATA ·CastConst+1768(SB)/8,$0x00000001ce95db64

	/* x^147520 mod p(x), x^147456 mod p(x) */
DATA ·CastConst+1776(SB)/8,$0x0000000173ea6628
DATA ·CastConst+1784(SB)/8,$0x00000000af582806

	/* x^146496 mod p(x), x^146432 mod p(x) */
DATA ·CastConst+1792(SB)/8,$0x00000001c8b4e00a
DATA ·CastConst+1800(SB)/8,$0x00000001006388f6

	/* x^145472 mod p(x), x^145408 mod p(x) */
DATA ·CastConst+1808(SB)/8,$0x00000000de95d6aa
DATA ·CastConst+1816(SB)/8,$0x0000000179eca00a

	/* x^144448 mod p(x), x^144384 mod p(x) */
DATA ·CastConst+1824(SB)/8,$0x000000010b7f7248
DATA ·CastConst+1832(SB)/8,$0x0000000122410a6a

	/* x^143424 mod p(x), x^143360 mod p(x) */
DATA ·CastConst+1840(SB)/8,$0x00000001326e3a06
DATA ·CastConst+1848(SB)/8,$0x000000004288e87c

	/* x^142400 mod p(x), x^142336 mod p(x) */
DATA ·CastConst+1856(SB)/8,$0x00000000bb62c2e6
DATA ·CastConst+1864(SB)/8,$0x000000016c5490da

	/* x^141376 mod p(x), x^141312 mod p(x) */
DATA ·CastConst+1872(SB)/8,$0x0000000156a4b2c2
DATA ·CastConst+1880(SB)/8,$0x00000000d1c71f6e

	/* x^140352 mod p(x), x^140288 mod p(x) */
DATA ·CastConst+1888(SB)/8,$0x000000011dfe763a
DATA ·CastConst+1896(SB)/8,$0x00000001b4ce08a6

	/* x^139328 mod p(x), x^139264 mod p(x) */
DATA ·CastConst+1904(SB)/8,$0x000000007bcca8e2
DATA ·CastConst+1912(SB)/8,$0x00000001466ba60c

	/* x^138304 mod p(x), x^138240 mod p(x) */
DATA ·CastConst+1920(SB)/8,$0x0000000186118faa
DATA ·CastConst+1928(SB)/8,$0x00000001f6c488a4

	/* x^137280 mod p(x), x^137216 mod p(x) */
DATA ·CastConst+1936(SB)/8,$0x0000000111a65a88
DATA ·CastConst+1944(SB)/8,$0x000000013bfb0682

	/* x^136256 mod p(x), x^136192 mod p(x) */
DATA ·CastConst+1952(SB)/8,$0x000000003565e1c4
DATA ·CastConst+1960(SB)/8,$0x00000000690e9e54

	/* x^135232 mod p(x), x^135168 mod p(x) */
DATA ·CastConst+1968(SB)/8,$0x000000012ed02a82
DATA ·CastConst+1976(SB)/8,$0x00000000281346b6

	/* x^134208 mod p(x), x^134144 mod p(x) */
DATA ·CastConst+1984(SB)/8,$0x00000000c486ecfc
DATA ·CastConst+1992(SB)/8,$0x0000000156464024

	/* x^133184 mod p(x), x^133120 mod p(x) */
DATA ·CastConst+2000(SB)/8,$0x0000000001b951b2
DATA ·CastConst+2008(SB)/8,$0x000000016063a8dc

	/* x^132160 mod p(x), x^132096 mod p(x) */
DATA ·CastConst+2016(SB)/8,$0x0000000048143916
DATA ·CastConst+2024(SB)/8,$0x0000000116a66362

	/* x^131136 mod p(x), x^131072 mod p(x) */
DATA ·CastConst+2032(SB)/8,$0x00000001dc2ae124
DATA ·CastConst+2040(SB)/8,$0x000000017e8aa4d2

	/* x^130112 mod p(x), x^130048 mod p(x) */
DATA ·CastConst+2048(SB)/8,$0x00000001416c58d6
DATA ·CastConst+2056(SB)/8,$0x00000001728eb10c

	/* x^129088 mod p(x), x^129024 mod p(x) */
DATA ·CastConst+2064(SB)/8,$0x00000000a479744a
DATA ·CastConst+2072(SB)/8,$0x00000001b08fd7fa

	/* x^128064 mod p(x), x^128000 mod p(x) */
DATA ·CastConst+2080(SB)/8,$0x0000000096ca3a26
DATA ·CastConst+2088(SB)/8,$0x00000001092a16e8

	/* x^127040 mod p(x), x^126976 mod p(x) */
DATA ·CastConst+2096(SB)/8,$0x00000000ff223d4e
DATA ·CastConst+2104(SB)/8,$0x00000000a505637c

	/* x^126016 mod p(x), x^125952 mod p(x) */
DATA ·CastConst+2112(SB)/8,$0x000000010e84da42
DATA ·CastConst+2120(SB)/8,$0x00000000d94869b2

	/* x^124992 mod p(x), x^124928 mod p(x) */
DATA ·CastConst+2128(SB)/8,$0x00000001b61ba3d0
DATA ·CastConst+2136(SB)/8,$0x00000001c8b203ae

	/* x^123968 mod p(x), x^123904 mod p(x) */
DATA ·CastConst+2144(SB)/8,$0x00000000680f2de8
DATA ·CastConst+2152(SB)/8,$0x000000005704aea0

	/* x^122944 mod p(x), x^122880 mod p(x) */
DATA ·CastConst+2160(SB)/8,$0x000000008772a9a8
DATA ·CastConst+2168(SB)/8,$0x000000012e295fa2

	/* x^121920 mod p(x), x^121856 mod p(x) */
DATA ·CastConst+2176(SB)/8,$0x0000000155f295bc
DATA ·CastConst+2184(SB)/8,$0x000000011d0908bc

	/* x^120896 mod p(x), x^120832 mod p(x) */
DATA ·CastConst+2192(SB)/8,$0x00000000595f9282
DATA ·CastConst+2200(SB)/8,$0x0000000193ed97ea

	/* x^119872 mod p(x), x^119808 mod p(x) */
DATA ·CastConst+2208(SB)/8,$0x0000000164b1c25a
DATA ·CastConst+2216(SB)/8,$0x000000013a0f1c52

	/* x^118848 mod p(x), x^118784 mod p(x) */
DATA ·CastConst+2224(SB)/8,$0x00000000fbd67c50
DATA ·CastConst+2232(SB)/8,$0x000000010c2c40c0

	/* x^117824 mod p(x), x^117760 mod p(x) */
DATA ·CastConst+2240(SB)/8,$0x0000000096076268
DATA ·CastConst+2248(SB)/8,$0x00000000ff6fac3e

	/* x^116800 mod p(x), x^116736 mod p(x) */
DATA ·CastConst+2256(SB)/8,$0x00000001d288e4cc
DATA ·CastConst+2264(SB)/8,$0x000000017b3609c0

	/* x^115776 mod p(x), x^115712 mod p(x) */
DATA ·CastConst+2272(SB)/8,$0x00000001eaac1bdc
DATA ·CastConst+2280(SB)/8,$0x0000000088c8c922

	/* x^114752 mod p(x), x^114688 mod p(x) */
DATA ·CastConst+2288(SB)/8,$0x00000001f1ea39e2
DATA ·CastConst+2296(SB)/8,$0x00000001751baae6

	/* x^113728 mod p(x), x^113664 mod p(x) */
DATA ·CastConst+2304(SB)/8,$0x00000001eb6506fc
DATA ·CastConst+2312(SB)/8,$0x0000000107952972

	/* x^112704 mod p(x), x^112640 mod p(x) */
DATA ·CastConst+2320(SB)/8,$0x000000010f806ffe
DATA ·CastConst+2328(SB)/8,$0x0000000162b00abe

	/* x^111680 mod p(x), x^111616 mod p(x) */
DATA ·CastConst+2336(SB)/8,$0x000000010408481e
DATA ·CastConst+2344(SB)/8,$0x000000000d7b404c

	/* x^110656 mod p(x), x^110592 mod p(x) */
DATA ·CastConst+2352(SB)/8,$0x0000000188260534
DATA ·CastConst+2360(SB)/8,$0x00000000763b13d4

	/* x^109632 mod p(x), x^109568 mod p(x) */
DATA ·CastConst+2368(SB)/8,$0x0000000058fc73e0
DATA ·CastConst+2376(SB)/8,$0x00000000f6dc22d8

	/* x^108608 mod p(x), x^108544 mod p(x) */
DATA ·CastConst+2384(SB)/8,$0x00000000391c59b8
DATA ·CastConst+2392(SB)/8,$0x000000007daae060

	/* x^107584 mod p(x), x^107520 mod p(x) */
DATA ·CastConst+2400(SB)/8,$0x000000018b638400
DATA ·CastConst+2408(SB)/8,$0x000000013359ab7c

	/* x^106560 mod p(x), x^106496 mod p(x) */
DATA ·CastConst+2416(SB)/8,$0x000000011738f5c4
DATA ·CastConst+2424(SB)/8,$0x000000008add438a

	/* x^105536 mod p(x), x^105472 mod p(x) */
DATA ·CastConst+2432(SB)/8,$0x000000008cf7c6da
DATA ·CastConst+2440(SB)/8,$0x00000001edbefdea

	/* x^104512 mod p(x), x^104448 mod p(x) */
DATA ·CastConst+2448(SB)/8,$0x00000001ef97fb16
DATA ·CastConst+2456(SB)/8,$0x000000004104e0f8

	/* x^103488 mod p(x), x^103424 mod p(x) */
DATA ·CastConst+2464(SB)/8,$0x0000000102130e20
DATA ·CastConst+2472(SB)/8,$0x00000000b48a8222

	/* x^102464 mod p(x), x^102400 mod p(x) */
DATA ·CastConst+2480(SB)/8,$0x00000000db968898
DATA ·CastConst+2488(SB)/8,$0x00000001bcb46844

	/* x^101440 mod p(x), x^101376 mod p(x) */
DATA ·CastConst+2496(SB)/8,$0x00000000b5047b5e
DATA ·CastConst+2504(SB)/8,$0x000000013293ce0a

	/* x^100416 mod p(x), x^100352 mod p(x) */
DATA ·CastConst+2512(SB)/8,$0x000000010b90fdb2
DATA ·CastConst+2520(SB)/8,$0x00000001710d0844

	/* x^99392 mod p(x), x^99328 mod p(x) */
DATA ·CastConst+2528(SB)/8,$0x000000004834a32e
DATA ·CastConst+2536(SB)/8,$0x0000000117907f6e

	/* x^98368 mod p(x), x^98304 mod p(x) */
DATA ·CastConst+2544(SB)/8,$0x0000000059c8f2b0
DATA ·CastConst+2552(SB)/8,$0x0000000087ddf93e

	/* x^97344 mod p(x), x^97280 mod p(x) */
DATA ·CastConst+2560(SB)/8,$0x0000000122cec508
DATA ·CastConst+2568(SB)/8,$0x000000005970e9b0

	/* x^96320 mod p(x), x^96256 mod p(x) */
DATA ·CastConst+2576(SB)/8,$0x000000000a330cda
DATA ·CastConst+2584(SB)/8,$0x0000000185b2b7d0

	/* x^95296 mod p(x), x^95232 mod p(x) */
DATA ·CastConst+2592(SB)/8,$0x000000014a47148c
DATA ·CastConst+2600(SB)/8,$0x00000001dcee0efc

	/* x^94272 mod p(x), x^94208 mod p(x) */
DATA ·CastConst+2608(SB)/8,$0x0000000042c61cb8
DATA ·CastConst+2616(SB)/8,$0x0000000030da2722

	/* x^93248 mod p(x), x^93184 mod p(x) */
DATA ·CastConst+2624(SB)/8,$0x0000000012fe6960
DATA ·CastConst+2632(SB)/8,$0x000000012f925a18

	/* x^92224 mod p(x), x^92160 mod p(x) */
DATA ·CastConst+2640(SB)/8,$0x00000000dbda2c20
DATA ·CastConst+2648(SB)/8,$0x00000000dd2e357c

	/* x^91200 mod p(x), x^91136 mod p(x) */
DATA ·CastConst+2656(SB)/8,$0x000000011122410c
DATA ·CastConst+2664(SB)/8,$0x00000000071c80de

	/* x^90176 mod p(x), x^90112 mod p(x) */
DATA ·CastConst+2672(SB)/8,$0x00000000977b2070
DATA ·CastConst+2680(SB)/8,$0x000000011513140a

	/* x^89152 mod p(x), x^89088 mod p(x) */
DATA ·CastConst+2688(SB)/8,$0x000000014050438e
DATA ·CastConst+2696(SB)/8,$0x00000001df876e8e

	/* x^88128 mod p(x), x^88064 mod p(x) */
DATA ·CastConst+2704(SB)/8,$0x0000000147c840e8
DATA ·CastConst+2712(SB)/8,$0x000000015f81d6ce

	/* x^87104 mod p(x), x^87040 mod p(x) */
DATA ·CastConst+2720(SB)/8,$0x00000001cc7c88ce
DATA ·CastConst+2728(SB)/8,$0x000000019dd94dbe

	/* x^86080 mod p(x), x^86016 mod p(x) */
DATA ·CastConst+2736(SB)/8,$0x00000001476b35a4
DATA ·CastConst+2744(SB)/8,$0x00000001373d206e

	/* x^85056 mod p(x), x^84992 mod p(x) */
DATA ·CastConst+2752(SB)/8,$0x000000013d52d508
DATA ·CastConst+2760(SB)/8,$0x00000000668ccade

	/* x^84032 mod p(x), x^83968 mod p(x) */
DATA ·CastConst+2768(SB)/8,$0x000000008e4be32e
DATA ·CastConst+2776(SB)/8,$0x00000001b192d268

	/* x^83008 mod p(x), x^82944 mod p(x) */
DATA ·CastConst+2784(SB)/8,$0x00000000024120fe
DATA ·CastConst+2792(SB)/8,$0x00000000e30f3a78

	/* x^81984 mod p(x), x^81920 mod p(x) */
DATA ·CastConst+2800(SB)/8,$0x00000000ddecddb4
DATA ·CastConst+2808(SB)/8,$0x000000010ef1f7bc

	/* x^80960 mod p(x), x^80896 mod p(x) */
DATA ·CastConst+2816(SB)/8,$0x00000000d4d403bc
DATA ·CastConst+2824(SB)/8,$0x00000001f5ac7380

	/* x^79936 mod p(x), x^79872 mod p(x) */
DATA ·CastConst+2832(SB)/8,$0x00000001734b89aa
DATA ·CastConst+2840(SB)/8,$0x000000011822ea70

	/* x^78912 mod p(x), x^78848 mod p(x) */
DATA ·CastConst+2848(SB)/8,$0x000000010e7a58d6
DATA ·CastConst+2856(SB)/8,$0x00000000c3a33848

	/* x^77888 mod p(x), x^77824 mod p(x) */
DATA ·CastConst+2864(SB)/8,$0x00000001f9f04e9c
DATA ·CastConst+2872(SB)/8,$0x00000001bd151c24

	/* x^76864 mod p(x), x^76800 mod p(x) */
DATA ·CastConst+2880(SB)/8,$0x00000000b692225e
DATA ·CastConst+2888(SB)/8,$0x0000000056002d76

	/* x^75840 mod p(x), x^75776 mod p(x) */
DATA ·CastConst+2896(SB)/8,$0x000000019b8d3f3e
DATA ·CastConst+2904(SB)/8,$0x000000014657c4f4

	/* x^74816 mod p(x), x^74752 mod p(x) */
DATA ·CastConst+2912(SB)/8,$0x00000001a874f11e
DATA ·CastConst+2920(SB)/8,$0x0000000113742d7c

	/* x^73792 mod p(x), x^73728 mod p(x) */
DATA ·CastConst+2928(SB)/8,$0x000000010d5a4254
DATA ·CastConst+2936(SB)/8,$0x000000019c5920ba

	/* x^72768 mod p(x), x^72704 mod p(x) */
DATA ·CastConst+2944(SB)/8,$0x00000000bbb2f5d6
DATA ·CastConst+2952(SB)/8,$0x000000005216d2d6

	/* x^71744 mod p(x), x^71680 mod p(x) */
DATA ·CastConst+2960(SB)/8,$0x0000000179cc0e36
DATA ·CastConst+2968(SB)/8,$0x0000000136f5ad8a

	/* x^70720 mod p(x), x^70656 mod p(x) */
DATA ·CastConst+2976(SB)/8,$0x00000001dca1da4a
DATA ·CastConst+2984(SB)/8,$0x000000018b07beb6

	/* x^69696 mod p(x), x^69632 mod p(x) */
DATA ·CastConst+2992(SB)/8,$0x00000000feb1a192
DATA ·CastConst+3000(SB)/8,$0x00000000db1e93b0

	/* x^68672 mod p(x), x^68608 mod p(x) */
DATA ·CastConst+3008(SB)/8,$0x00000000d1eeedd6
DATA ·CastConst+3016(SB)/8,$0x000000000b96fa3a

	/* x^67648 mod p(x), x^67584 mod p(x) */
DATA ·CastConst+3024(SB)/8,$0x000000008fad9bb4
DATA ·CastConst+3032(SB)/8,$0x00000001d9968af0

	/* x^66624 mod p(x), x^66560 mod p(x) */
DATA ·CastConst+3040(SB)/8,$0x00000001884938e4
DATA ·CastConst+3048(SB)/8,$0x000000000e4a77a2

	/* x^65600 mod p(x), x^65536 mod p(x) */
DATA ·CastConst+3056(SB)/8,$0x00000001bc2e9bc0
DATA ·CastConst+3064(SB)/8,$0x00000000508c2ac8

	/* x^64576 mod p(x), x^64512 mod p(x) */
DATA ·CastConst+3072(SB)/8,$0x00000001f9658a68
DATA ·CastConst+3080(SB)/8,$0x0000000021572a80

	/* x^63552 mod p(x), x^63488 mod p(x) */
DATA ·CastConst+3088(SB)/8,$0x000000001b9224fc
DATA ·CastConst+3096(SB)/8,$0x00000001b859daf2

	/* x^62528 mod p(x), x^62464 mod p(x) */
DATA ·CastConst+3104(SB)/8,$0x0000000055b2fb84
DATA ·CastConst+3112(SB)/8,$0x000000016f788474

	/* x^61504 mod p(x), x^61440 mod p(x) */
DATA ·CastConst+3120(SB)/8,$0x000000018b090348
DATA ·CastConst+3128(SB)/8,$0x00000001b438810e

	/* x^60480 mod p(x), x^60416 mod p(x) */
DATA ·CastConst+3136(SB)/8,$0x000000011ccbd5ea
DATA ·CastConst+3144(SB)/8,$0x0000000095ddc6f2

	/* x^59456 mod p(x), x^59392 mod p(x) */
DATA ·CastConst+3152(SB)/8,$0x0000000007ae47f8
DATA ·CastConst+3160(SB)/8,$0x00000001d977c20c

	/* x^58432 mod p(x), x^58368 mod p(x) */
DATA ·CastConst+3168(SB)/8,$0x0000000172acbec0
DATA ·CastConst+3176(SB)/8,$0x00000000ebedb99a

	/* x^57408 mod p(x), x^57344 mod p(x) */
DATA ·CastConst+3184(SB)/8,$0x00000001c6e3ff20
DATA ·CastConst+3192(SB)/8,$0x00000001df9e9e92

	/* x^56384 mod p(x), x^56320 mod p(x) */
DATA ·CastConst+3200(SB)/8,$0x00000000e1b38744
DATA ·CastConst+3208(SB)/8,$0x00000001a4a3f952

	/* x^55360 mod p(x), x^55296 mod p(x) */
DATA ·CastConst+3216(SB)/8,$0x00000000791585b2
DATA ·CastConst+3224(SB)/8,$0x00000000e2f51220

	/* x^54336 mod p(x), x^54272 mod p(x) */
DATA ·CastConst+3232(SB)/8,$0x00000000ac53b894
DATA ·CastConst+3240(SB)/8,$0x000000004aa01f3e

	/* x^53312 mod p(x), x^53248 mod p(x) */
DATA ·CastConst+3248(SB)/8,$0x00000001ed5f2cf4
DATA ·CastConst+3256(SB)/8,$0x00000000b3e90a58

	/* x^52288 mod p(x), x^52224 mod p(x) */
DATA ·CastConst+3264(SB)/8,$0x00000001df48b2e0
DATA ·CastConst+3272(SB)/8,$0x000000000c9ca2aa

	/* x^51264 mod p(x), x^51200 mod p(x) */
DATA ·CastConst+3280(SB)/8,$0x00000000049c1c62
DATA ·CastConst+3288(SB)/8,$0x0000000151682316

	/* x^50240 mod p(x), x^50176 mod p(x) */
DATA ·CastConst+3296(SB)/8,$0x000000017c460c12
DATA ·CastConst+3304(SB)/8,$0x0000000036fce78c

	/* x^49216 mod p(x), x^49152 mod p(x) */
DATA ·CastConst+3312(SB)/8,$0x000000015be4da7e
DATA ·CastConst+3320(SB)/8,$0x000000009037dc10

	/* x^48192 mod p(x), x^48128 mod p(x) */
DATA ·CastConst+3328(SB)/8,$0x000000010f38f668
DATA ·CastConst+3336(SB)/8,$0x00000000d3298582

	/* x^47168 mod p(x), x^47104 mod p(x) */
DATA ·CastConst+3344(SB)/8,$0x0000000039f40a00
DATA ·CastConst+3352(SB)/8,$0x00000001b42e8ad6

	/* x^46144 mod p(x), x^46080 mod p(x) */
DATA ·CastConst+3360(SB)/8,$0x00000000bd4c10c4
DATA ·CastConst+3368(SB)/8,$0x00000000142a9838

	/* x^45120 mod p(x), x^45056 mod p(x) */
DATA ·CastConst+3376(SB)/8,$0x0000000042db1d98
DATA ·CastConst+3384(SB)/8,$0x0000000109c7f190

	/* x^44096 mod p(x), x^44032 mod p(x) */
DATA ·CastConst+3392(SB)/8,$0x00000001c905bae6
DATA ·CastConst+3400(SB)/8,$0x0000000056ff9310

	/* x^43072 mod p(x), x^43008 mod p(x) */
DATA ·CastConst+3408(SB)/8,$0x00000000069d40ea
DATA ·CastConst+3416(SB)/8,$0x00000001594513aa

	/* x^42048 mod p(x), x^41984 mod p(x) */
DATA ·CastConst+3424(SB)/8,$0x000000008e4fbad0
DATA ·CastConst+3432(SB)/8,$0x00000001e3b5b1e8

	/* x^41024 mod p(x), x^40960 mod p(x) */
DATA ·CastConst+3440(SB)/8,$0x0000000047bedd46
DATA ·CastConst+3448(SB)/8,$0x000000011dd5fc08

	/* x^40000 mod p(x), x^39936 mod p(x) */
DATA ·CastConst+3456(SB)/8,$0x0000000026396bf8
DATA ·CastConst+3464(SB)/8,$0x00000001675f0cc2

	/* x^38976 mod p(x), x^38912 mod p(x) */
DATA ·CastConst+3472(SB)/8,$0x00000000379beb92
DATA ·CastConst+3480(SB)/8,$0x00000000d1c8dd44

	/* x^37952 mod p(x), x^37888 mod p(x) */
DATA ·CastConst+3488(SB)/8,$0x000000000abae54a
DATA ·CastConst+3496(SB)/8,$0x0000000115ebd3d8

	/* x^36928 mod p(x), x^36864 mod p(x) */
DATA ·CastConst+3504(SB)/8,$0x0000000007e6a128
DATA ·CastConst+3512(SB)/8,$0x00000001ecbd0dac

	/* x^35904 mod p(x), x^35840 mod p(x) */
DATA ·CastConst+3520(SB)/8,$0x000000000ade29d2
DATA ·CastConst+3528(SB)/8,$0x00000000cdf67af2

	/* x^34880 mod p(x), x^34816 mod p(x) */
DATA ·CastConst+3536(SB)/8,$0x00000000f974c45c
DATA ·CastConst+3544(SB)/8,$0x000000004c01ff4c

	/* x^33856 mod p(x), x^33792 mod p(x) */
DATA ·CastConst+3552(SB)/8,$0x00000000e77ac60a
DATA ·CastConst+3560(SB)/8,$0x00000000f2d8657e

	/* x^32832 mod p(x), x^32768 mod p(x) */
DATA ·CastConst+3568(SB)/8,$0x0000000145895816
DATA ·CastConst+3576(SB)/8,$0x000000006bae74c4

	/* x^31808 mod p(x), x^31744 mod p(x) */
DATA ·CastConst+3584(SB)/8,$0x0000000038e362be
DATA ·CastConst+3592(SB)/8,$0x0000000152af8aa0

	/* x^30784 mod p(x), x^30720 mod p(x) */
DATA ·CastConst+3600(SB)/8,$0x000000007f991a64
DATA ·CastConst+3608(SB)/8,$0x0000000004663802

	/* x^29760 mod p(x), x^29696 mod p(x) */
DATA ·CastConst+3616(SB)/8,$0x00000000fa366d3a
DATA ·CastConst+3624(SB)/8,$0x00000001ab2f5afc

	/* x^28736 mod p(x), x^28672 mod p(x) */
DATA ·CastConst+3632(SB)/8,$0x00000001a2bb34f0
DATA ·CastConst+3640(SB)/8,$0x0000000074a4ebd4

	/* x^27712 mod p(x), x^27648 mod p(x) */
DATA ·CastConst+3648(SB)/8,$0x0000000028a9981e
DATA ·CastConst+3656(SB)/8,$0x00000001d7ab3a4c

	/* x^26688 mod p(x), x^26624 mod p(x) */
DATA ·CastConst+3664(SB)/8,$0x00000001dbc672be
DATA ·CastConst+3672(SB)/8,$0x00000001a8da60c6

	/* x^25664 mod p(x), x^25600 mod p(x) */
DATA ·CastConst+3680(SB)/8,$0x00000000b04d77f6
DATA ·CastConst+3688(SB)/8,$0x000000013cf63820

	/* x^24640 mod p(x), x^24576 mod p(x) */
DATA ·CastConst+3696(SB)/8,$0x0000000124400d96
DATA ·CastConst+3704(SB)/8,$0x00000000bec12e1e

	/* x^23616 mod p(x), x^23552 mod p(x) */
DATA ·CastConst+3712(SB)/8,$0x000000014ca4b414
DATA ·CastConst+3720(SB)/8,$0x00000001c6368010

	/* x^22592 mod p(x), x^22528 mod p(x) */
DATA ·CastConst+3728(SB)/8,$0x000000012fe2c938
DATA ·CastConst+3736(SB)/8,$0x00000001e6e78758

	/* x^21568 mod p(x), x^21504 mod p(x) */
DATA ·CastConst+3744(SB)/8,$0x00000001faed01e6
DATA ·CastConst+3752(SB)/8,$0x000000008d7f2b3c

	/* x^20544 mod p(x), x^20480 mod p(x) */
DATA ·CastConst+3760(SB)/8,$0x000000007e80ecfe
DATA ·CastConst+3768(SB)/8,$0x000000016b4a156e

	/* x^19520 mod p(x), x^19456 mod p(x) */
DATA ·CastConst+3776(SB)/8,$0x0000000098daee94
DATA ·CastConst+3784(SB)/8,$0x00000001c63cfeb6

	/* x^18496 mod p(x), x^18432 mod p(x) */
DATA ·CastConst+3792(SB)/8,$0x000000010a04edea
DATA ·CastConst+3800(SB)/8,$0x000000015f902670

	/* x^17472 mod p(x), x^17408 mod p(x) */
DATA ·CastConst+3808(SB)/8,$0x00000001c00b4524
DATA ·CastConst+3816(SB)/8,$0x00000001cd5de11e

	/* x^16448 mod p(x), x^16384 mod p(x) */
DATA ·CastConst+3824(SB)/8,$0x0000000170296550
DATA ·CastConst+3832(SB)/8,$0x000000001acaec54

	/* x^15424 mod p(x), x^15360 mod p(x) */
DATA ·CastConst+3840(SB)/8,$0x0000000181afaa48
DATA ·CastConst+3848(SB)/8,$0x000000002bd0ca78

	/* x^14400 mod p(x), x^14336 mod p(x) */
DATA ·CastConst+3856(SB)/8,$0x0000000185a31ffa
DATA ·CastConst+3864(SB)/8,$0x0000000032d63d5c

	/* x^13376 mod p(x), x^13312 mod p(x) */
DATA ·CastConst+3872(SB)/8,$0x000000002469f608
DATA ·CastConst+3880(SB)/8,$0x000000001c6d4e4c

	/* x^12352 mod p(x), x^12288 mod p(x) */
DATA ·CastConst+3888(SB)/8,$0x000000006980102a
DATA ·CastConst+3896(SB)/8,$0x0000000106a60b92

	/* x^11328 mod p(x), x^11264 mod p(x) */
DATA ·CastConst+3904(SB)/8,$0x0000000111ea9ca8
DATA ·CastConst+3912(SB)/8,$0x00000000d3855e12

	/* x^10304 mod p(x), x^10240 mod p(x) */
DATA ·CastConst+3920(SB)/8,$0x00000001bd1d29ce
DATA ·CastConst+3928(SB)/8,$0x00000000e3125636

	/* x^9280 mod p(x), x^9216 mod p(x) */
DATA ·CastConst+3936(SB)/8,$0x00000001b34b9580
DATA ·CastConst+3944(SB)/8,$0x000000009e8f7ea4

	/* x^8256 mod p(x), x^8192 mod p(x) */
DATA ·CastConst+3952(SB)/8,$0x000000003076054e
DATA ·CastConst+3960(SB)/8,$0x00000001c82e562c

	/* x^7232 mod p(x), x^7168 mod p(x) */
DATA ·CastConst+3968(SB)/8,$0x000000012a608ea4
DATA ·CastConst+3976(SB)/8,$0x00000000ca9f09ce

	/* x^6208 mod p(x), x^6144 mod p(x) */
DATA ·CastConst+3984(SB)/8,$0x00000000784d05fe
DATA ·CastConst+3992(SB)/8,$0x00000000c63764e6

	/* x^5184 mod p(x), x^5120 mod p(x) */
DATA ·CastConst+4000(SB)/8,$0x000000016ef0d82a
DATA ·CastConst+4008(SB)/8,$0x0000000168d2e49e

	/* x^4160 mod p(x), x^4096 mod p(x) */
DATA ·CastConst+4016(SB)/8,$0x0000000075bda454
DATA ·CastConst+4024(SB)/8,$0x00000000e986c148

	/* x^3136 mod p(x), x^3072 mod p(x) */
DATA ·CastConst+4032(SB)/8,$0x000000003dc0a1c4
DATA ·CastConst+4040(SB)/8,$0x00000000cfb65894

	/* x^2112 mod p(x), x^2048 mod p(x) */
DATA ·CastConst+4048(SB)/8,$0x00000000e9a5d8be
DATA ·CastConst+4056(SB)/8,$0x0000000111cadee4

	/* x^1088 mod p(x), x^1024 mod p(x) */
DATA ·CastConst+4064(SB)/8,$0x00000001609bc4b4
DATA ·CastConst+4072(SB)/8,$0x0000000171fb63ce

	/* x^2048 mod p(x), x^2016 mod p(x), x^1984 mod p(x), x^1952 mod p(x) */
DATA ·CastConst+4080(SB)/8,$0x5cf015c388e56f72
DATA ·CastConst+4088(SB)/8,$0x7fec2963e5bf8048

	/* x^1920 mod p(x), x^1888 mod p(x), x^1856 mod p(x), x^1824 mod p(x) */
DATA ·CastConst+4096(SB)/8,$0x963a18920246e2e6
DATA ·CastConst+4104(SB)/8,$0x38e888d4844752a9

	/* x^1792 mod p(x), x^1760 mod p(x), x^1728 mod p(x), x^1696 mod p(x) */
DATA ·CastConst+4112(SB)/8,$0x419a441956993a31
DATA ·CastConst+4120(SB)/8,$0x42316c00730206ad

	/* x^1664 mod p(x), x^1632 mod p(x), x^1600 mod p(x), x^1568 mod p(x) */
DATA ·CastConst+4128(SB)/8,$0x924752ba2b830011
DATA ·CastConst+4136(SB)/8,$0x543d5c543e65ddf9

	/* x^1536 mod p(x), x^1504 mod p(x), x^1472 mod p(x), x^1440 mod p(x) */
DATA ·CastConst+4144(SB)/8,$0x55bd7f9518e4a304
DATA ·CastConst+4152(SB)/8,$0x78e87aaf56767c92

	/* x^1408 mod p(x), x^1376 mod p(x), x^1344 mod p(x), x^1312 mod p(x) */
DATA ·CastConst+4160(SB)/8,$0x6d76739fe0553f1e
DATA ·CastConst+4168(SB)/8,$0x8f68fcec1903da7f

	/* x^1280 mod p(x), x^1248 mod p(x), x^1216 mod p(x), x^1184 mod p(x) */
DATA ·CastConst+4176(SB)/8,$0xc133722b1fe0b5c3
DATA ·CastConst+4184(SB)/8,$0x3f4840246791d588

	/* x^1152 mod p(x), x^1120 mod p(x), x^1088 mod p(x), x^1056 mod p(x) */
DATA ·CastConst+4192(SB)/8,$0x64b67ee0e55ef1f3
DATA ·CastConst+4200(SB)/8,$0x34c96751b04de25a

	/* x^1024 mod p(x), x^992 mod p(x), x^960 mod p(x), x^928 mod p(x) */
DATA ·CastConst+4208(SB)/8,$0x069db049b8fdb1e7
DATA ·CastConst+4216(SB)/8,$0x156c8e180b4a395b

	/* x^896 mod p(x), x^864 mod p(x), x^832 mod p(x), x^800 mod p(x) */
DATA ·CastConst+4224(SB)/8,$0xa11bfaf3c9e90b9e
DATA ·CastConst+4232(SB)/8,$0xe0b99ccbe661f7be

	/* x^768 mod p(x), x^736 mod p(x), x^704 mod p(x), x^672 mod p(x) */
DATA ·CastConst+4240(SB)/8,$0x817cdc5119b29a35
DATA ·CastConst+4248(SB)/8,$0x041d37768cd75659

	/* x^640 mod p(x), x^608 mod p(x), x^576 mod p(x), x^544 mod p(x) */
DATA ·CastConst+4256(SB)/8,$0x1ce9d94b36c41f1c
DATA ·CastConst+4264(SB)/8,$0x3a0777818cfaa965

	/* x^512 mod p(x), x^480 mod p(x), x^448 mod p(x), x^416 mod p(x) */
DATA ·CastConst+4272(SB)/8,$0x4f256efcb82be955
DATA ·CastConst+4280(SB)/8,$0x0e148e8252377a55

	/* x^384 mod p(x), x^352 mod p(x), x^320 mod p(x), x^288 mod p(x) */
DATA ·CastConst+4288(SB)/8,$0xec1631edb2dea967
DATA ·CastConst+4296(SB)/8,$0x9c25531d19e65dde

	/* x^256 mod p(x), x^224 mod p(x), x^192 mod p(x), x^160 mod p(x) */
DATA ·CastConst+4304(SB)/8,$0x5d27e147510ac59a
DATA ·CastConst+4312(SB)/8,$0x790606ff9957c0a6

	/* x^128 mod p(x), x^96 mod p(x), x^64 mod p(x), x^32 mod p(x) */
DATA ·CastConst+4320(SB)/8,$0xa66805eb18b8ea18
DATA ·CastConst+4328(SB)/8,$0x82f63b786ea2d55c

GLOBL ·CastConst(SB),RODATA,$4336

	/* Barrett constant m - (4^32)/n */
DATA ·CastBarConst(SB)/8,$0x00000000dea713f1
DATA ·CastBarConst+8(SB)/8,$0x0000000000000000
DATA ·CastBarConst+16(SB)/8,$0x0000000105ec76f1
DATA ·CastBarConst+24(SB)/8,$0x0000000000000000
GLOBL ·CastBarConst(SB),RODATA,$32

	/* Reduce 262144 kbits to 1024 bits */
	/* x^261184 mod p(x), x^261120 mod p(x) */
DATA ·KoopConst+0(SB)/8,$0x00000000d72535b2
DATA ·KoopConst+8(SB)/8,$0x000000007fd74916

	/* x^260160 mod p(x), x^260096 mod p(x) */
DATA ·KoopConst+16(SB)/8,$0x0000000118a2a1b4
DATA ·KoopConst+24(SB)/8,$0x000000010e944b56

	/* x^259136 mod p(x), x^259072 mod p(x) */
DATA ·KoopConst+32(SB)/8,$0x0000000147b5c49c
DATA ·KoopConst+40(SB)/8,$0x00000000bfe71c20

	/* x^258112 mod p(x), x^258048 mod p(x) */
DATA ·KoopConst+48(SB)/8,$0x00000001ca76a040
DATA ·KoopConst+56(SB)/8,$0x0000000021324d9a

	/* x^257088 mod p(x), x^257024 mod p(x) */
DATA ·KoopConst+64(SB)/8,$0x00000001e3152efc
DATA ·KoopConst+72(SB)/8,$0x00000000d20972ce

	/* x^256064 mod p(x), x^256000 mod p(x) */
DATA ·KoopConst+80(SB)/8,$0x00000001b0349792
DATA ·KoopConst+88(SB)/8,$0x000000003475ea06

	/* x^255040 mod p(x), x^254976 mod p(x) */
DATA ·KoopConst+96(SB)/8,$0x0000000120a60fe0
DATA ·KoopConst+104(SB)/8,$0x00000001e40e36c4

	/* x^254016 mod p(x), x^253952 mod p(x) */
DATA ·KoopConst+112(SB)/8,$0x00000000b3c4b082
DATA ·KoopConst+120(SB)/8,$0x00000000b2490102

	/* x^252992 mod p(x), x^252928 mod p(x) */
DATA ·KoopConst+128(SB)/8,$0x000000017fe9f3d2
DATA ·KoopConst+136(SB)/8,$0x000000016b9e1332

	/* x^251968 mod p(x), x^251904 mod p(x) */
DATA ·KoopConst+144(SB)/8,$0x0000000145703cbe
DATA ·KoopConst+152(SB)/8,$0x00000001d6c378f4

	/* x^250944 mod p(x), x^250880 mod p(x) */
DATA ·KoopConst+160(SB)/8,$0x0000000107551c9c
DATA ·KoopConst+168(SB)/8,$0x0000000085796eac

	/* x^249920 mod p(x), x^249856 mod p(x) */
DATA ·KoopConst+176(SB)/8,$0x000000003865a702
DATA ·KoopConst+184(SB)/8,$0x000000019d2f3aaa

	/* x^248896 mod p(x), x^248832 mod p(x) */
DATA ·KoopConst+192(SB)/8,$0x000000005504f9b8
DATA ·KoopConst+200(SB)/8,$0x00000001554ddbd4

	/* x^247872 mod p(x), x^247808 mod p(x) */
DATA ·KoopConst+208(SB)/8,$0x00000000239bcdd4
DATA ·KoopConst+216(SB)/8,$0x00000000a76376b0

	/* x^246848 mod p(x), x^246784 mod p(x) */
DATA ·KoopConst+224(SB)/8,$0x00000000caead774
DATA ·KoopConst+232(SB)/8,$0x0000000139b7283c

	/* x^245824 mod p(x), x^245760 mod p(x) */
DATA ·KoopConst+240(SB)/8,$0x0000000022a3fa16
DATA ·KoopConst+248(SB)/8,$0x0000000111087030

	/* x^244800 mod p(x), x^244736 mod p(x) */
DATA ·KoopConst+256(SB)/8,$0x000000011f89160e
DATA ·KoopConst+264(SB)/8,$0x00000000ad786dc2

	/* x^243776 mod p(x), x^243712 mod p(x) */
DATA ·KoopConst+272(SB)/8,$0x00000001a976c248
DATA ·KoopConst+280(SB)/8,$0x00000000b7a1d068

	/* x^242752 mod p(x), x^242688 mod p(x) */
DATA ·KoopConst+288(SB)/8,$0x00000000c20d09c8
DATA ·KoopConst+296(SB)/8,$0x000000009c5c591c

	/* x^241728 mod p(x), x^241664 mod p(x) */
DATA ·KoopConst+304(SB)/8,$0x000000016264fe38
DATA ·KoopConst+312(SB)/8,$0x000000016482aa1a

	/* x^240704 mod p(x), x^240640 mod p(x) */
DATA ·KoopConst+320(SB)/8,$0x00000001b57aee6a
DATA ·KoopConst+328(SB)/8,$0x000000009a409ba8

	/* x^239680 mod p(x), x^239616 mod p(x) */
DATA ·KoopConst+336(SB)/8,$0x00000000e8f1be0a
DATA ·KoopConst+344(SB)/8,$0x00000001ad8eaed8

	/* x^238656 mod p(x), x^238592 mod p(x) */
DATA ·KoopConst+352(SB)/8,$0x0000000053fcd0fc
DATA ·KoopConst+360(SB)/8,$0x000000017558b57a

	/* x^237632 mod p(x), x^237568 mod p(x) */
DATA ·KoopConst+368(SB)/8,$0x000000012df9d496
DATA ·KoopConst+376(SB)/8,$0x00000000cbb749c8

	/* x^236608 mod p(x), x^236544 mod p(x) */
DATA ·KoopConst+384(SB)/8,$0x000000004cb0db26
DATA ·KoopConst+392(SB)/8,$0x000000008524fc5a

	/* x^235584 mod p(x), x^235520 mod p(x) */
DATA ·KoopConst+400(SB)/8,$0x00000001150c4584
DATA ·KoopConst+408(SB)/8,$0x0000000028ce6b76

	/* x^234560 mod p(x), x^234496 mod p(x) */
DATA ·KoopConst+416(SB)/8,$0x0000000104f52056
DATA ·KoopConst+424(SB)/8,$0x00000000e0c48bdc

	/* x^233536 mod p(x), x^233472 mod p(x) */
DATA ·KoopConst+432(SB)/8,$0x000000008ea11ac8
DATA ·KoopConst+440(SB)/8,$0x000000003dd3bf9a

	/* x^232512 mod p(x), x^232448 mod p(x) */
DATA ·KoopConst+448(SB)/8,$0x00000001cc0a3942
DATA ·KoopConst+456(SB)/8,$0x00000000cb71066c

	/* x^231488 mod p(x), x^231424 mod p(x) */
DATA ·KoopConst+464(SB)/8,$0x00000000d26231e6
DATA ·KoopConst+472(SB)/8,$0x00000001d4ee1540

	/* x^230464 mod p(x), x^230400 mod p(x) */
DATA ·KoopConst+480(SB)/8,$0x00000000c70d5730
DATA ·KoopConst+488(SB)/8,$0x00000001d82bed0a

	/* x^229440 mod p(x), x^229376 mod p(x) */
DATA ·KoopConst+496(SB)/8,$0x00000000e215dfc4
DATA ·KoopConst+504(SB)/8,$0x000000016e0c7d86

	/* x^228416 mod p(x), x^228352 mod p(x) */
DATA ·KoopConst+512(SB)/8,$0x000000013870d0dc
DATA ·KoopConst+520(SB)/8,$0x00000001437051b0

	/* x^227392 mod p(x), x^227328 mod p(x) */
DATA ·KoopConst+528(SB)/8,$0x0000000153e4cf3c
DATA ·KoopConst+536(SB)/8,$0x00000000f9a8d4be

	/* x^226368 mod p(x), x^226304 mod p(x) */
DATA ·KoopConst+544(SB)/8,$0x0000000125f6fdf0
DATA ·KoopConst+552(SB)/8,$0x000000016b09be1c

	/* x^225344 mod p(x), x^225280 mod p(x) */
DATA ·KoopConst+560(SB)/8,$0x0000000157ba3a82
DATA ·KoopConst+568(SB)/8,$0x0000000105f50ed6

	/* x^224320 mod p(x), x^224256 mod p(x) */
DATA ·KoopConst+576(SB)/8,$0x00000001cf711064
DATA ·KoopConst+584(SB)/8,$0x00000001ca7fe3cc

	/* x^223296 mod p(x), x^223232 mod p(x) */
DATA ·KoopConst+592(SB)/8,$0x00000001006353d2
DATA ·KoopConst+600(SB)/8,$0x0000000192372e78

	/* x^222272 mod p(x), x^222208 mod p(x) */
DATA ·KoopConst+608(SB)/8,$0x000000010cd9faec
DATA ·KoopConst+616(SB)/8,$0x000000008a47af7e

	/* x^221248 mod p(x), x^221184 mod p(x) */
DATA ·KoopConst+624(SB)/8,$0x000000012148b190
DATA ·KoopConst+632(SB)/8,$0x00000000a67473e8

	/* x^220224 mod p(x), x^220160 mod p(x) */
DATA ·KoopConst+640(SB)/8,$0x00000000776473d6
DATA ·KoopConst+648(SB)/8,$0x000000013689f2fa

	/* x^219200 mod p(x), x^219136 mod p(x) */
DATA ·KoopConst+656(SB)/8,$0x00000001ce765bd6
DATA ·KoopConst+664(SB)/8,$0x00000000e7231774

	/* x^218176 mod p(x), x^218112 mod p(x) */
DATA ·KoopConst+672(SB)/8,$0x00000000b29165e8
DATA ·KoopConst+680(SB)/8,$0x0000000011b5ae68

	/* x^217152 mod p(x), x^217088 mod p(x) */
DATA ·KoopConst+688(SB)/8,$0x0000000084ff5a68
DATA ·KoopConst+696(SB)/8,$0x000000004fd5c188

	/* x^216128 mod p(x), x^216064 mod p(x) */
DATA ·KoopConst+704(SB)/8,$0x00000001921e9076
DATA ·KoopConst+712(SB)/8,$0x000000012148fa22

	/* x^215104 mod p(x), x^215040 mod p(x) */
DATA ·KoopConst+720(SB)/8,$0x000000009a753a3c
DATA ·KoopConst+728(SB)/8,$0x000000010cff4f3e

	/* x^214080 mod p(x), x^214016 mod p(x) */
DATA ·KoopConst+736(SB)/8,$0x000000000251401e
DATA ·KoopConst+744(SB)/8,$0x00000001f9d991d4

	/* x^213056 mod p(x), x^212992 mod p(x) */
DATA ·KoopConst+752(SB)/8,$0x00000001f65541fa
DATA ·KoopConst+760(SB)/8,$0x00000001c31db214

	/* x^212032 mod p(x), x^211968 mod p(x) */
DATA ·KoopConst+768(SB)/8,$0x00000001d8c8117a
DATA ·KoopConst+776(SB)/8,$0x00000001849fba4a

	/* x^211008 mod p(x), x^210944 mod p(x) */
DATA ·KoopConst+784(SB)/8,$0x000000014f7a2200
DATA ·KoopConst+792(SB)/8,$0x00000001cb603184

	/* x^209984 mod p(x), x^209920 mod p(x) */
DATA ·KoopConst+800(SB)/8,$0x000000005154a9f4
DATA ·KoopConst+808(SB)/8,$0x0000000132db7116

	/* x^208960 mod p(x), x^208896 mod p(x) */
DATA ·KoopConst+816(SB)/8,$0x00000001dfc69196
DATA ·KoopConst+824(SB)/8,$0x0000000010694e22

	/* x^207936 mod p(x), x^207872 mod p(x) */
DATA ·KoopConst+832(SB)/8,$0x00000001c29f1aa0
DATA ·KoopConst+840(SB)/8,$0x0000000103b7b478

	/* x^206912 mod p(x), x^206848 mod p(x) */
DATA ·KoopConst+848(SB)/8,$0x000000013785f232
DATA ·KoopConst+856(SB)/8,$0x000000000ab44030

	/* x^205888 mod p(x), x^205824 mod p(x) */
DATA ·KoopConst+864(SB)/8,$0x000000010133536e
DATA ·KoopConst+872(SB)/8,$0x0000000131385b68

	/* x^204864 mod p(x), x^204800 mod p(x) */
DATA ·KoopConst+880(SB)/8,$0x00000001d45421dc
DATA ·KoopConst+888(SB)/8,$0x00000001761dab66

	/* x^203840 mod p(x), x^203776 mod p(x) */
DATA ·KoopConst+896(SB)/8,$0x000000000b59cc28
DATA ·KoopConst+904(SB)/8,$0x000000012cf0a2a6

	/* x^202816 mod p(x), x^202752 mod p(x) */
DATA ·KoopConst+912(SB)/8,$0x00000001f2f74aba
DATA ·KoopConst+920(SB)/8,$0x00000001f4ce25a2

	/* x^201792 mod p(x), x^201728 mod p(x) */
DATA ·KoopConst+928(SB)/8,$0x00000000fb308e7e
DATA ·KoopConst+936(SB)/8,$0x000000014c2aae20

	/* x^200768 mod p(x), x^200704 mod p(x) */
DATA ·KoopConst+944(SB)/8,$0x0000000167583fa6
DATA ·KoopConst+952(SB)/8,$0x00000001c162a55a

	/* x^199744 mod p(x), x^199680 mod p(x) */
DATA ·KoopConst+960(SB)/8,$0x000000017ebb13e0
DATA ·KoopConst+968(SB)/8,$0x0000000185681a40

	/* x^198720 mod p(x), x^198656 mod p(x) */
DATA ·KoopConst+976(SB)/8,$0x00000001ca653306
DATA ·KoopConst+984(SB)/8,$0x00000001f2642b48

	/* x^197696 mod p(x), x^197632 mod p(x) */
DATA ·KoopConst+992(SB)/8,$0x0000000093bb6946
DATA ·KoopConst+1000(SB)/8,$0x00000001d9cb5a78

	/* x^196672 mod p(x), x^196608 mod p(x) */
DATA ·KoopConst+1008(SB)/8,$0x00000000cbc1553e
DATA ·KoopConst+1016(SB)/8,$0x000000008059328c

	/* x^195648 mod p(x), x^195584 mod p(x) */
DATA ·KoopConst+1024(SB)/8,$0x00000001f9a86fec
DATA ·KoopConst+1032(SB)/8,$0x000000009373c360

	/* x^194624 mod p(x), x^194560 mod p(x) */
DATA ·KoopConst+1040(SB)/8,$0x0000000005c52d8a
DATA ·KoopConst+1048(SB)/8,$0x00000001a14061d6

	/* x^193600 mod p(x), x^193536 mod p(x) */
DATA ·KoopConst+1056(SB)/8,$0x000000010d8dc668
DATA ·KoopConst+1064(SB)/8,$0x00000000a9864d48

	/* x^192576 mod p(x), x^192512 mod p(x) */
DATA ·KoopConst+1072(SB)/8,$0x0000000158571310
DATA ·KoopConst+1080(SB)/8,$0x000000011df8c040

	/* x^191552 mod p(x), x^191488 mod p(x) */
DATA ·KoopConst+1088(SB)/8,$0x0000000166102348
DATA ·KoopConst+1096(SB)/8,$0x0000000023a3e6b6

	/* x^190528 mod p(x), x^190464 mod p(x) */
DATA ·KoopConst+1104(SB)/8,$0x0000000009513050
DATA ·KoopConst+1112(SB)/8,$0x00000001207db28a

	/* x^189504 mod p(x), x^189440 mod p(x) */
DATA ·KoopConst+1120(SB)/8,$0x00000000b0725c74
DATA ·KoopConst+1128(SB)/8,$0x00000000f94bc632

	/* x^188480 mod p(x), x^188416 mod p(x) */
DATA ·KoopConst+1136(SB)/8,$0x000000002985c7e2
DATA ·KoopConst+1144(SB)/8,$0x00000000ea32cbf6

	/* x^187456 mod p(x), x^187392 mod p(x) */
DATA ·KoopConst+1152(SB)/8,$0x00000000a7d4da9e
DATA ·KoopConst+1160(SB)/8,$0x0000000004eb981a

	/* x^186432 mod p(x), x^186368 mod p(x) */
DATA ·KoopConst+1168(SB)/8,$0x000000000a3f8792
DATA ·KoopConst+1176(SB)/8,$0x00000000ca8ce712

	/* x^185408 mod p(x), x^185344 mod p(x) */
DATA ·KoopConst+1184(SB)/8,$0x00000001ca2c1ce4
DATA ·KoopConst+1192(SB)/8,$0x0000000065ba801c

	/* x^184384 mod p(x), x^184320 mod p(x) */
DATA ·KoopConst+1200(SB)/8,$0x00000000e2900196
DATA ·KoopConst+1208(SB)/8,$0x0000000194aade7a

	/* x^183360 mod p(x), x^183296 mod p(x) */
DATA ·KoopConst+1216(SB)/8,$0x00000001fbadf0e4
DATA ·KoopConst+1224(SB)/8,$0x00000001e7939fb2

	/* x^182336 mod p(x), x^182272 mod p(x) */
DATA ·KoopConst+1232(SB)/8,$0x00000000d5d96c40
DATA ·KoopConst+1240(SB)/8,$0x0000000098e5fe22

	/* x^181312 mod p(x), x^181248 mod p(x) */
DATA ·KoopConst+1248(SB)/8,$0x000000015c11d3f2
DATA ·KoopConst+1256(SB)/8,$0x000000016bba0324

	/* x^180288 mod p(x), x^180224 mod p(x) */
DATA ·KoopConst+1264(SB)/8,$0x0000000111fb2648
DATA ·KoopConst+1272(SB)/8,$0x0000000104dce052

	/* x^179264 mod p(x), x^179200 mod p(x) */
DATA ·KoopConst+1280(SB)/8,$0x00000001d9f3a564
DATA ·KoopConst+1288(SB)/8,$0x00000001af31a42e

	/* x^178240 mod p(x), x^178176 mod p(x) */
DATA ·KoopConst+1296(SB)/8,$0x00000001b556cd1e
DATA ·KoopConst+1304(SB)/8,$0x00000001c56c57ba

	/* x^177216 mod p(x), x^177152 mod p(x) */
DATA ·KoopConst+1312(SB)/8,$0x0000000101994d2c
DATA ·KoopConst+1320(SB)/8,$0x00000000f6bb1a2e

	/* x^176192 mod p(x), x^176128 mod p(x) */
DATA ·KoopConst+1328(SB)/8,$0x00000001e8dbf09c
DATA ·KoopConst+1336(SB)/8,$0x00000001abdbf2b2

	/* x^175168 mod p(x), x^175104 mod p(x) */
DATA ·KoopConst+1344(SB)/8,$0x000000015580543a
DATA ·KoopConst+1352(SB)/8,$0x00000001a665a880

	/* x^174144 mod p(x), x^174080 mod p(x) */
DATA ·KoopConst+1360(SB)/8,$0x00000000c7074f24
DATA ·KoopConst+1368(SB)/8,$0x00000000c102c700

	/* x^173120 mod p(x), x^173056 mod p(x) */
DATA ·KoopConst+1376(SB)/8,$0x00000000fa4112b0
DATA ·KoopConst+1384(SB)/8,$0x00000000ee362a50

	/* x^172096 mod p(x), x^172032 mod p(x) */
DATA ·KoopConst+1392(SB)/8,$0x00000000e786c13e
DATA ·KoopConst+1400(SB)/8,$0x0000000045f29038

	/* x^171072 mod p(x), x^171008 mod p(x) */
DATA ·KoopConst+1408(SB)/8,$0x00000001e45e3694
DATA ·KoopConst+1416(SB)/8,$0x0000000117b9ab5c

	/* x^170048 mod p(x), x^169984 mod p(x) */
DATA ·KoopConst+1424(SB)/8,$0x000000005423dd8c
DATA ·KoopConst+1432(SB)/8,$0x00000001115dff5e

	/* x^169024 mod p(x), x^168960 mod p(x) */
DATA ·KoopConst+1440(SB)/8,$0x00000001a1e67766
DATA ·KoopConst+1448(SB)/8,$0x0000000117fad29c

	/* x^168000 mod p(x), x^167936 mod p(x) */
DATA ·KoopConst+1456(SB)/8,$0x0000000041a3f508
DATA ·KoopConst+1464(SB)/8,$0x000000017de134e6

	/* x^166976 mod p(x), x^166912 mod p(x) */
DATA ·KoopConst+1472(SB)/8,$0x000000003e792f7e
DATA ·KoopConst+1480(SB)/8,$0x00000000a2f5d19c

	/* x^165952 mod p(x), x^165888 mod p(x) */
DATA ·KoopConst+1488(SB)/8,$0x00000000c8948aaa
DATA ·KoopConst+1496(SB)/8,$0x00000000dee13658

	/* x^164928 mod p(x), x^164864 mod p(x) */
DATA ·KoopConst+1504(SB)/8,$0x000000005d4ccb36
DATA ·KoopConst+1512(SB)/8,$0x000000015355440c

	/* x^163904 mod p(x), x^163840 mod p(x) */
DATA ·KoopConst+1520(SB)/8,$0x00000000e92a78a2
DATA ·KoopConst+1528(SB)/8,$0x0000000197a21778

	/* x^162880 mod p(x), x^162816 mod p(x) */
DATA ·KoopConst+1536(SB)/8,$0x000000016ba67caa
DATA ·KoopConst+1544(SB)/8,$0x00000001a3835ec0

	/* x^161856 mod p(x), x^161792 mod p(x) */
DATA ·KoopConst+1552(SB)/8,$0x000000004838afc6
DATA ·KoopConst+1560(SB)/8,$0x0000000011f20912

	/* x^160832 mod p(x), x^160768 mod p(x) */
DATA ·KoopConst+1568(SB)/8,$0x000000016644e308
DATA ·KoopConst+1576(SB)/8,$0x00000001cce9d6cc

	/* x^159808 mod p(x), x^159744 mod p(x) */
DATA ·KoopConst+1584(SB)/8,$0x0000000037c22f42
DATA ·KoopConst+1592(SB)/8,$0x0000000084d1e71c

	/* x^158784 mod p(x), x^158720 mod p(x) */
DATA ·KoopConst+1600(SB)/8,$0x00000001dedba6ca
DATA ·KoopConst+1608(SB)/8,$0x0000000197c2ad54

	/* x^157760 mod p(x), x^157696 mod p(x) */
DATA ·KoopConst+1616(SB)/8,$0x0000000146a43500
DATA ·KoopConst+1624(SB)/8,$0x000000018609261e

	/* x^156736 mod p(x), x^156672 mod p(x) */
DATA ·KoopConst+1632(SB)/8,$0x000000001cf762de
DATA ·KoopConst+1640(SB)/8,$0x00000000b4b4c224

	/* x^155712 mod p(x), x^155648 mod p(x) */
DATA ·KoopConst+1648(SB)/8,$0x0000000022ff7eda
DATA ·KoopConst+1656(SB)/8,$0x0000000080817496

	/* x^154688 mod p(x), x^154624 mod p(x) */
DATA ·KoopConst+1664(SB)/8,$0x00000001b6df625e
DATA ·KoopConst+1672(SB)/8,$0x00000001aefb473c

	/* x^153664 mod p(x), x^153600 mod p(x) */
DATA ·KoopConst+1680(SB)/8,$0x00000001cc99ab58
DATA ·KoopConst+1688(SB)/8,$0x000000013f1aa474

	/* x^152640 mod p(x), x^152576 mod p(x) */
DATA ·KoopConst+1696(SB)/8,$0x00000001c53f5ce2
DATA ·KoopConst+1704(SB)/8,$0x000000010ca2c756

	/* x^151616 mod p(x), x^151552 mod p(x) */
DATA ·KoopConst+1712(SB)/8,$0x0000000082a9c60e
DATA ·KoopConst+1720(SB)/8,$0x000000002c63533a

	/* x^150592 mod p(x), x^150528 mod p(x) */
DATA ·KoopConst+1728(SB)/8,$0x00000000ec78b570
DATA ·KoopConst+1736(SB)/8,$0x00000001b7f2ad50

	/* x^149568 mod p(x), x^149504 mod p(x) */
DATA ·KoopConst+1744(SB)/8,$0x00000001d3fe1e8e
DATA ·KoopConst+1752(SB)/8,$0x00000000acdf4c20

	/* x^148544 mod p(x), x^148480 mod p(x) */
DATA ·KoopConst+1760(SB)/8,$0x000000007f9a7bde
DATA ·KoopConst+1768(SB)/8,$0x000000000bd29e8c

	/* x^147520 mod p(x), x^147456 mod p(x) */
DATA ·KoopConst+1776(SB)/8,$0x00000000e606f518
DATA ·KoopConst+1784(SB)/8,$0x00000001eef6992e

	/* x^146496 mod p(x), x^146432 mod p(x) */
DATA ·KoopConst+1792(SB)/8,$0x000000008538cb96
DATA ·KoopConst+1800(SB)/8,$0x00000000b01644e6

	/* x^145472 mod p(x), x^145408 mod p(x) */
DATA ·KoopConst+1808(SB)/8,$0x0000000131d030b2
DATA ·KoopConst+1816(SB)/8,$0x0000000059c51acc

	/* x^144448 mod p(x), x^144384 mod p(x) */
DATA ·KoopConst+1824(SB)/8,$0x00000000115a4d0e
DATA ·KoopConst+1832(SB)/8,$0x00000001a2849272

	/* x^143424 mod p(x), x^143360 mod p(x) */
DATA ·KoopConst+1840(SB)/8,$0x00000000e8a5356e
DATA ·KoopConst+1848(SB)/8,$0x00000001a4e0b610

	/* x^142400 mod p(x), x^142336 mod p(x) */
DATA ·KoopConst+1856(SB)/8,$0x0000000158d988be
DATA ·KoopConst+1864(SB)/8,$0x00000000084e81a6

	/* x^141376 mod p(x), x^141312 mod p(x) */
DATA ·KoopConst+1872(SB)/8,$0x00000001240db498
DATA ·KoopConst+1880(SB)/8,$0x00000001b71f1fd8

	/* x^140352 mod p(x), x^140288 mod p(x) */
DATA ·KoopConst+1888(SB)/8,$0x000000009ce87826
DATA ·KoopConst+1896(SB)/8,$0x000000017f7df380

	/* x^139328 mod p(x), x^139264 mod p(x) */
DATA ·KoopConst+1904(SB)/8,$0x0000000021944aae
DATA ·KoopConst+1912(SB)/8,$0x00000001f7f4e190

	/* x^138304 mod p(x), x^138240 mod p(x) */
DATA ·KoopConst+1920(SB)/8,$0x00000001cea3d67e
DATA ·KoopConst+1928(SB)/8,$0x0000000150220d86

	/* x^137280 mod p(x), x^137216 mod p(x) */
DATA ·KoopConst+1936(SB)/8,$0x000000004434e926
DATA ·KoopConst+1944(SB)/8,$0x00000001db7d2b2e

	/* x^136256 mod p(x), x^136192 mod p(x) */
DATA ·KoopConst+1952(SB)/8,$0x0000000011db8cbe
DATA ·KoopConst+1960(SB)/8,$0x00000000b6ba9668

	/* x^135232 mod p(x), x^135168 mod p(x) */
DATA ·KoopConst+1968(SB)/8,$0x00000001f6e0b8dc
DATA ·KoopConst+1976(SB)/8,$0x0000000103fdcecc

	/* x^134208 mod p(x), x^134144 mod p(x) */
DATA ·KoopConst+1984(SB)/8,$0x00000001f163f4a0
DATA ·KoopConst+1992(SB)/8,$0x0000000079816a22

	/* x^133184 mod p(x), x^133120 mod p(x) */
DATA ·KoopConst+2000(SB)/8,$0x000000007b6cc60e
DATA ·KoopConst+2008(SB)/8,$0x0000000173483482

	/* x^132160 mod p(x), x^132096 mod p(x) */
DATA ·KoopConst+2016(SB)/8,$0x000000000f26c82c
DATA ·KoopConst+2024(SB)/8,$0x00000000643ea4c0

	/* x^131136 mod p(x), x^131072 mod p(x) */
DATA ·KoopConst+2032(SB)/8,$0x00000000b0acad80
DATA ·KoopConst+2040(SB)/8,$0x00000000a64752d2

	/* x^130112 mod p(x), x^130048 mod p(x) */
DATA ·KoopConst+2048(SB)/8,$0x000000013687e91c
DATA ·KoopConst+2056(SB)/8,$0x00000000ca98eb3a

	/* x^129088 mod p(x), x^129024 mod p(x) */
DATA ·KoopConst+2064(SB)/8,$0x000000006bac3a96
DATA ·KoopConst+2072(SB)/8,$0x00000001ca6ac8f8

	/* x^128064 mod p(x), x^128000 mod p(x) */
DATA ·KoopConst+2080(SB)/8,$0x00000001bf197d5c
DATA ·KoopConst+2088(SB)/8,$0x00000001c48e2e68

	/* x^127040 mod p(x), x^126976 mod p(x) */
DATA ·KoopConst+2096(SB)/8,$0x00000000256e84f2
DATA ·KoopConst+2104(SB)/8,$0x0000000070086782

	/* x^126016 mod p(x), x^125952 mod p(x) */
DATA ·KoopConst+2112(SB)/8,$0x000000003eff0d16
DATA ·KoopConst+2120(SB)/8,$0x00000000f763621c

	/* x^124992 mod p(x), x^124928 mod p(x) */
DATA ·KoopConst+2128(SB)/8,$0x00000001748e9fd2
DATA ·KoopConst+2136(SB)/8,$0x00000000ba58646a

	/* x^123968 mod p(x), x^123904 mod p(x) */
DATA ·KoopConst+2144(SB)/8,$0x000000015bb85b42
DATA ·KoopConst+2152(SB)/8,$0x0000000138e157d8

	/* x^122944 mod p(x), x^122880 mod p(x) */
DATA ·KoopConst+2160(SB)/8,$0x0000000164d1a980
DATA ·KoopConst+2168(SB)/8,$0x00000001bf0a09dc

	/* x^121920 mod p(x), x^121856 mod p(x) */
DATA ·KoopConst+2176(SB)/8,$0x000000001415c9f0
DATA ·KoopConst+2184(SB)/8,$0x0000000098faf300

	/* x^120896 mod p(x), x^120832 mod p(x) */
DATA ·KoopConst+2192(SB)/8,$0x0000000195ae2f48
DATA ·KoopConst+2200(SB)/8,$0x00000001f872f2c6

	/* x^119872 mod p(x), x^119808 mod p(x) */
DATA ·KoopConst+2208(SB)/8,$0x0000000059d1d81a
DATA ·KoopConst+2216(SB)/8,$0x00000000f92577be

	/* x^118848 mod p(x), x^118784 mod p(x) */
DATA ·KoopConst+2224(SB)/8,$0x00000001bf80257a
DATA ·KoopConst+2232(SB)/8,$0x00000001a4d975f4

	/* x^117824 mod p(x), x^117760 mod p(x) */
DATA ·KoopConst+2240(SB)/8,$0x000000011e39bfce
DATA ·KoopConst+2248(SB)/8,$0x000000018b74eeca

	/* x^116800 mod p(x), x^116736 mod p(x) */
DATA ·KoopConst+2256(SB)/8,$0x00000001287a0456
DATA ·KoopConst+2264(SB)/8,$0x00000000e8980404

	/* x^115776 mod p(x), x^115712 mod p(x) */
DATA ·KoopConst+2272(SB)/8,$0x00000000a5eb589c
DATA ·KoopConst+2280(SB)/8,$0x0000000176ef2b74

	/* x^114752 mod p(x), x^114688 mod p(x) */
DATA ·KoopConst+2288(SB)/8,$0x000000017d71c452
DATA ·KoopConst+2296(SB)/8,$0x0000000063c85caa

	/* x^113728 mod p(x), x^113664 mod p(x) */
DATA ·KoopConst+2304(SB)/8,$0x00000000fa941f08
DATA ·KoopConst+2312(SB)/8,$0x00000001708012cc

	/* x^112704 mod p(x), x^112640 mod p(x) */
DATA ·KoopConst+2320(SB)/8,$0x0000000064ea030e
DATA ·KoopConst+2328(SB)/8,$0x00000000474d58f6

	/* x^111680 mod p(x), x^111616 mod p(x) */
DATA ·KoopConst+2336(SB)/8,$0x000000019b7cc7ba
DATA ·KoopConst+2344(SB)/8,$0x00000001c76085a6

	/* x^110656 mod p(x), x^110592 mod p(x) */
DATA ·KoopConst+2352(SB)/8,$0x00000000225cb7ba
DATA ·KoopConst+2360(SB)/8,$0x000000018fb0681a

	/* x^109632 mod p(x), x^109568 mod p(x) */
DATA ·KoopConst+2368(SB)/8,$0x000000010ab3e1da
DATA ·KoopConst+2376(SB)/8,$0x00000001fcee1f16

	/* x^108608 mod p(x), x^108544 mod p(x) */
DATA ·KoopConst+2384(SB)/8,$0x00000001ce5cc33e
DATA ·KoopConst+2392(SB)/8,$0x00000000cfbffb7c

	/* x^107584 mod p(x), x^107520 mod p(x) */
DATA ·KoopConst+2400(SB)/8,$0x000000005e980f6e
DATA ·KoopConst+2408(SB)/8,$0x000000017af8ee72

	/* x^106560 mod p(x), x^106496 mod p(x) */
DATA ·KoopConst+2416(SB)/8,$0x00000000d3bf3f46
DATA ·KoopConst+2424(SB)/8,$0x000000001c2ad3e2

	/* x^105536 mod p(x), x^105472 mod p(x) */
DATA ·KoopConst+2432(SB)/8,$0x000000018d554ae0
DATA ·KoopConst+2440(SB)/8,$0x00000000ee05450a

	/* x^104512 mod p(x), x^104448 mod p(x) */
DATA ·KoopConst+2448(SB)/8,$0x000000018e276eb0
DATA ·KoopConst+2456(SB)/8,$0x000000000f7d5bac

	/* x^103488 mod p(x), x^103424 mod p(x) */
DATA ·KoopConst+2464(SB)/8,$0x000000001c0319ce
DATA ·KoopConst+2472(SB)/8,$0x00000001cb26e004

	/* x^102464 mod p(x), x^102400 mod p(x) */
DATA ·KoopConst+2480(SB)/8,$0x00000001ca0c75ec
DATA ·KoopConst+2488(SB)/8,$0x00000001553314e2

	/* x^101440 mod p(x), x^101376 mod p(x) */
DATA ·KoopConst+2496(SB)/8,$0x00000001fb075330
DATA ·KoopConst+2504(SB)/8,$0x000000005729be2c

	/* x^100416 mod p(x), x^100352 mod p(x) */
DATA ·KoopConst+2512(SB)/8,$0x00000000677920e4
DATA ·KoopConst+2520(SB)/8,$0x0000000192c4479c

	/* x^99392 mod p(x), x^99328 mod p(x) */
DATA ·KoopConst+2528(SB)/8,$0x00000000332247c8
DATA ·KoopConst+2536(SB)/8,$0x0000000078d842b6

	/* x^98368 mod p(x), x^98304 mod p(x) */
DATA ·KoopConst+2544(SB)/8,$0x00000000ef84fc6c
DATA ·KoopConst+2552(SB)/8,$0x0000000145ffa282

	/* x^97344 mod p(x), x^97280 mod p(x) */
DATA ·KoopConst+2560(SB)/8,$0x0000000139ba7690
DATA ·KoopConst+2568(SB)/8,$0x000000019d679bf4

	/* x^96320 mod p(x), x^96256 mod p(x) */
DATA ·KoopConst+2576(SB)/8,$0x00000000029ef444
DATA ·KoopConst+2584(SB)/8,$0x000000019412f7a0

	/* x^95296 mod p(x), x^95232 mod p(x) */
DATA ·KoopConst+2592(SB)/8,$0x00000001d872048c
DATA ·KoopConst+2600(SB)/8,$0x00000000b28c5c96

	/* x^94272 mod p(x), x^94208 mod p(x) */
DATA ·KoopConst+2608(SB)/8,$0x000000016535d70a
DATA ·KoopConst+2616(SB)/8,$0x00000000554bfd44

	/* x^93248 mod p(x), x^93184 mod p(x) */
DATA ·KoopConst+2624(SB)/8,$0x00000000761dd222
DATA ·KoopConst+2632(SB)/8,$0x00000000ce9cfa48

	/* x^92224 mod p(x), x^92160 mod p(x) */
DATA ·KoopConst+2640(SB)/8,$0x00000001509a3a44
DATA ·KoopConst+2648(SB)/8,$0x00000000a4702ab2

	/* x^91200 mod p(x), x^91136 mod p(x) */
DATA ·KoopConst+2656(SB)/8,$0x000000007e7019f2
DATA ·KoopConst+2664(SB)/8,$0x00000001c967fbee

	/* x^90176 mod p(x), x^90112 mod p(x) */
DATA ·KoopConst+2672(SB)/8,$0x00000000fb4c56ea
DATA ·KoopConst+2680(SB)/8,$0x00000000fd514b3e

	/* x^89152 mod p(x), x^89088 mod p(x) */
DATA ·KoopConst+2688(SB)/8,$0x000000012022e0ee
DATA ·KoopConst+2696(SB)/8,$0x00000001c0b6f95e

	/* x^88128 mod p(x), x^88064 mod p(x) */
DATA ·KoopConst+2704(SB)/8,$0x0000000004bc6054
DATA ·KoopConst+2712(SB)/8,$0x0000000180e103ce

	/* x^87104 mod p(x), x^87040 mod p(x) */
DATA ·KoopConst+2720(SB)/8,$0x000000017a1a0030
DATA ·KoopConst+2728(SB)/8,$0x00000001a1630916

	/* x^86080 mod p(x), x^86016 mod p(x) */
DATA ·KoopConst+2736(SB)/8,$0x00000001c021a864
DATA ·KoopConst+2744(SB)/8,$0x000000009a727fb2

	/* x^85056 mod p(x), x^84992 mod p(x) */
DATA ·KoopConst+2752(SB)/8,$0x000000009c54421e
DATA ·KoopConst+2760(SB)/8,$0x00000000e83b081a

	/* x^84032 mod p(x), x^83968 mod p(x) */
DATA ·KoopConst+2768(SB)/8,$0x00000001b4e33e6a
DATA ·KoopConst+2776(SB)/8,$0x000000006b1a1f44

	/* x^83008 mod p(x), x^82944 mod p(x) */
DATA ·KoopConst+2784(SB)/8,$0x000000015d615af0
DATA ·KoopConst+2792(SB)/8,$0x00000000cf280394

	/* x^81984 mod p(x), x^81920 mod p(x) */
DATA ·KoopConst+2800(SB)/8,$0x00000001914a3ba8
DATA ·KoopConst+2808(SB)/8,$0x00000001154b8a9a

	/* x^80960 mod p(x), x^80896 mod p(x) */
DATA ·KoopConst+2816(SB)/8,$0x000000005f72ec44
DATA ·KoopConst+2824(SB)/8,$0x0000000149ec63e2

	/* x^79936 mod p(x), x^79872 mod p(x) */
DATA ·KoopConst+2832(SB)/8,$0x00000000a33746a8
DATA ·KoopConst+2840(SB)/8,$0x000000018ef902c4

	/* x^78912 mod p(x), x^78848 mod p(x) */
DATA ·KoopConst+2848(SB)/8,$0x00000001c91e90d4
DATA ·KoopConst+2856(SB)/8,$0x0000000069addb88

	/* x^77888 mod p(x), x^77824 mod p(x) */
DATA ·KoopConst+2864(SB)/8,$0x00000001052eb05e
DATA ·KoopConst+2872(SB)/8,$0x00000000e90a29ae

	/* x^76864 mod p(x), x^76800 mod p(x) */
DATA ·KoopConst+2880(SB)/8,$0x000000006a32f754
DATA ·KoopConst+2888(SB)/8,$0x00000000c53641ae

	/* x^75840 mod p(x), x^75776 mod p(x) */
DATA ·KoopConst+2896(SB)/8,$0x00000001ecbd6436
DATA ·KoopConst+2904(SB)/8,$0x00000000a17c3796

	/* x^74816 mod p(x), x^74752 mod p(x) */
DATA ·KoopConst+2912(SB)/8,$0x000000000fd3f93a
DATA ·KoopConst+2920(SB)/8,$0x000000015307a62c

	/* x^73792 mod p(x), x^73728 mod p(x) */
DATA ·KoopConst+2928(SB)/8,$0x00000001686a4c24
DATA ·KoopConst+2936(SB)/8,$0x000000002f94bbda

	/* x^72768 mod p(x), x^72704 mod p(x) */
DATA ·KoopConst+2944(SB)/8,$0x00000001e40afca0
DATA ·KoopConst+2952(SB)/8,$0x0000000072c8b5e6

	/* x^71744 mod p(x), x^71680 mod p(x) */
DATA ·KoopConst+2960(SB)/8,$0x000000012779a2b8
DATA ·KoopConst+2968(SB)/8,$0x00000000f09b7424

	/* x^70720 mod p(x), x^70656 mod p(x) */
DATA ·KoopConst+2976(SB)/8,$0x00000000dcdaeb9e
DATA ·KoopConst+2984(SB)/8,$0x00000001c57de3da

	/* x^69696 mod p(x), x^69632 mod p(x) */
DATA ·KoopConst+2992(SB)/8,$0x00000001674f7a2a
DATA ·KoopConst+3000(SB)/8,$0x000000013922b30e

	/* x^68672 mod p(x), x^68608 mod p(x) */
DATA ·KoopConst+3008(SB)/8,$0x00000000dcb9e846
DATA ·KoopConst+3016(SB)/8,$0x000000008759a6c2

	/* x^67648 mod p(x), x^67584 mod p(x) */
DATA ·KoopConst+3024(SB)/8,$0x00000000ea9a6af6
DATA ·KoopConst+3032(SB)/8,$0x00000000545ae424

	/* x^66624 mod p(x), x^66560 mod p(x) */
DATA ·KoopConst+3040(SB)/8,$0x000000006d1f7a74
DATA ·KoopConst+3048(SB)/8,$0x00000001e0cbafd2

	/* x^65600 mod p(x), x^65536 mod p(x) */
DATA ·KoopConst+3056(SB)/8,$0x000000006add215e
DATA ·KoopConst+3064(SB)/8,$0x0000000018360c04

	/* x^64576 mod p(x), x^64512 mod p(x) */
DATA ·KoopConst+3072(SB)/8,$0x000000010a9ee4b0
DATA ·KoopConst+3080(SB)/8,$0x00000000941dc432

	/* x^63552 mod p(x), x^63488 mod p(x) */
DATA ·KoopConst+3088(SB)/8,$0x00000000304c48d2
DATA ·KoopConst+3096(SB)/8,$0x0000000004d3566e

	/* x^62528 mod p(x), x^62464 mod p(x) */
DATA ·KoopConst+3104(SB)/8,$0x0000000163d0e672
DATA ·KoopConst+3112(SB)/8,$0x0000000096aed14e

	/* x^61504 mod p(x), x^61440 mod p(x) */
DATA ·KoopConst+3120(SB)/8,$0x0000000010049166
DATA ·KoopConst+3128(SB)/8,$0x0000000087c13618

	/* x^60480 mod p(x), x^60416 mod p(x) */
DATA ·KoopConst+3136(SB)/8,$0x00000001d3913e34
DATA ·KoopConst+3144(SB)/8,$0x00000001d52f7b0c

	/* x^59456 mod p(x), x^59392 mod p(x) */
DATA ·KoopConst+3152(SB)/8,$0x00000001e392d54a
DATA ·KoopConst+3160(SB)/8,$0x000000000182058e

	/* x^58432 mod p(x), x^58368 mod p(x) */
DATA ·KoopConst+3168(SB)/8,$0x0000000173f2704a
DATA ·KoopConst+3176(SB)/8,$0x00000001ed73aa02

	/* x^57408 mod p(x), x^57344 mod p(x) */
DATA ·KoopConst+3184(SB)/8,$0x000000019112b480
DATA ·KoopConst+3192(SB)/8,$0x000000002721a82e

	/* x^56384 mod p(x), x^56320 mod p(x) */
DATA ·KoopConst+3200(SB)/8,$0x0000000093d295d6
DATA ·KoopConst+3208(SB)/8,$0x000000012ca83da2

	/* x^55360 mod p(x), x^55296 mod p(x) */
DATA ·KoopConst+3216(SB)/8,$0x0000000114e37f44
DATA ·KoopConst+3224(SB)/8,$0x00000000da358698

	/* x^54336 mod p(x), x^54272 mod p(x) */
DATA ·KoopConst+3232(SB)/8,$0x00000000fcfebc86
DATA ·KoopConst+3240(SB)/8,$0x0000000011fad322

	/* x^53312 mod p(x), x^53248 mod p(x) */
DATA ·KoopConst+3248(SB)/8,$0x00000000834c48d6
DATA ·KoopConst+3256(SB)/8,$0x000000012b25025c

	/* x^52288 mod p(x), x^52224 mod p(x) */
DATA ·KoopConst+3264(SB)/8,$0x000000017b909372
DATA ·KoopConst+3272(SB)/8,$0x000000001290cd24

	/* x^51264 mod p(x), x^51200 mod p(x) */
DATA ·KoopConst+3280(SB)/8,$0x000000010156b9ac
DATA ·KoopConst+3288(SB)/8,$0x000000016edd0b06

	/* x^50240 mod p(x), x^50176 mod p(x) */
DATA ·KoopConst+3296(SB)/8,$0x0000000113a82fa8
DATA ·KoopConst+3304(SB)/8,$0x00000000c08e222a

	/* x^49216 mod p(x), x^49152 mod p(x) */
DATA ·KoopConst+3312(SB)/8,$0x0000000182dacb74
DATA ·KoopConst+3320(SB)/8,$0x00000000cfb4d10e

	/* x^48192 mod p(x), x^48128 mod p(x) */
DATA ·KoopConst+3328(SB)/8,$0x000000010210dc40
DATA ·KoopConst+3336(SB)/8,$0x000000013e156ece

	/* x^47168 mod p(x), x^47104 mod p(x) */
DATA ·KoopConst+3344(SB)/8,$0x000000008ab5ed20
DATA ·KoopConst+3352(SB)/8,$0x00000000f12d89f8

	/* x^46144 mod p(x), x^46080 mod p(x) */
DATA ·KoopConst+3360(SB)/8,$0x00000000810386fa
DATA ·KoopConst+3368(SB)/8,$0x00000001fce3337c

	/* x^45120 mod p(x), x^45056 mod p(x) */
DATA ·KoopConst+3376(SB)/8,$0x000000011dce2fe2
DATA ·KoopConst+3384(SB)/8,$0x00000001c4bf3514

	/* x^44096 mod p(x), x^44032 mod p(x) */
DATA ·KoopConst+3392(SB)/8,$0x000000004bb0a390
DATA ·KoopConst+3400(SB)/8,$0x00000001ae67c492

	/* x^43072 mod p(x), x^43008 mod p(x) */
DATA ·KoopConst+3408(SB)/8,$0x00000000028d486a
DATA ·KoopConst+3416(SB)/8,$0x00000000302af704

	/* x^42048 mod p(x), x^41984 mod p(x) */
DATA ·KoopConst+3424(SB)/8,$0x000000010e4d63fe
DATA ·KoopConst+3432(SB)/8,$0x00000001e375b250

	/* x^41024 mod p(x), x^40960 mod p(x) */
DATA ·KoopConst+3440(SB)/8,$0x000000014fd6f458
DATA ·KoopConst+3448(SB)/8,$0x00000001678b58c0

	/* x^40000 mod p(x), x^39936 mod p(x) */
DATA ·KoopConst+3456(SB)/8,$0x00000000db7a83a2
DATA ·KoopConst+3464(SB)/8,$0x0000000065103c1e

	/* x^38976 mod p(x), x^38912 mod p(x) */
DATA ·KoopConst+3472(SB)/8,$0x000000016cf9fa3c
DATA ·KoopConst+3480(SB)/8,$0x000000000ccd28ca

	/* x^37952 mod p(x), x^37888 mod p(x) */
DATA ·KoopConst+3488(SB)/8,$0x000000016bb33912
DATA ·KoopConst+3496(SB)/8,$0x0000000059c177d4

	/* x^36928 mod p(x), x^36864 mod p(x) */
DATA ·KoopConst+3504(SB)/8,$0x0000000135bda8bc
DATA ·KoopConst+3512(SB)/8,$0x00000001d162f83a

	/* x^35904 mod p(x), x^35840 mod p(x) */
DATA ·KoopConst+3520(SB)/8,$0x000000004e8c6b76
DATA ·KoopConst+3528(SB)/8,$0x00000001efc0230c

	/* x^34880 mod p(x), x^34816 mod p(x) */
DATA ·KoopConst+3536(SB)/8,$0x00000000e17cb750
DATA ·KoopConst+3544(SB)/8,$0x00000001a2a2e2d2

	/* x^33856 mod p(x), x^33792 mod p(x) */
DATA ·KoopConst+3552(SB)/8,$0x000000010e8bb9cc
DATA ·KoopConst+3560(SB)/8,$0x00000001145c9dc2

	/* x^32832 mod p(x), x^32768 mod p(x) */
DATA ·KoopConst+3568(SB)/8,$0x00000001859d1cae
DATA ·KoopConst+3576(SB)/8,$0x00000000949e4a48

	/* x^31808 mod p(x), x^31744 mod p(x) */
DATA ·KoopConst+3584(SB)/8,$0x0000000167802bbe
DATA ·KoopConst+3592(SB)/8,$0x0000000128beecbc

	/* x^30784 mod p(x), x^30720 mod p(x) */
DATA ·KoopConst+3600(SB)/8,$0x0000000086f5219c
DATA ·KoopConst+3608(SB)/8,$0x00000001ffc96ae4

	/* x^29760 mod p(x), x^29696 mod p(x) */
DATA ·KoopConst+3616(SB)/8,$0x00000001349a4faa
DATA ·KoopConst+3624(SB)/8,$0x00000001ba81e0aa

	/* x^28736 mod p(x), x^28672 mod p(x) */
DATA ·KoopConst+3632(SB)/8,$0x000000007da3353e
DATA ·KoopConst+3640(SB)/8,$0x0000000104d7df14

	/* x^27712 mod p(x), x^27648 mod p(x) */
DATA ·KoopConst+3648(SB)/8,$0x00000000440fba4e
DATA ·KoopConst+3656(SB)/8,$0x00000001c2ff8518

	/* x^26688 mod p(x), x^26624 mod p(x) */
DATA ·KoopConst+3664(SB)/8,$0x00000000507aba70
DATA ·KoopConst+3672(SB)/8,$0x00000000ba6d4708

	/* x^25664 mod p(x), x^25600 mod p(x) */
DATA ·KoopConst+3680(SB)/8,$0x0000000015b578b6
DATA ·KoopConst+3688(SB)/8,$0x00000001d49d4bba

	/* x^24640 mod p(x), x^24576 mod p(x) */
DATA ·KoopConst+3696(SB)/8,$0x0000000141633fb2
DATA ·KoopConst+3704(SB)/8,$0x00000000d21247e6

	/* x^23616 mod p(x), x^23552 mod p(x) */
DATA ·KoopConst+3712(SB)/8,$0x0000000178712680
DATA ·KoopConst+3720(SB)/8,$0x0000000063b4004a

	/* x^22592 mod p(x), x^22528 mod p(x) */
DATA ·KoopConst+3728(SB)/8,$0x000000001404c194
DATA ·KoopConst+3736(SB)/8,$0x0000000094f55d2c

	/* x^21568 mod p(x), x^21504 mod p(x) */
DATA ·KoopConst+3744(SB)/8,$0x00000000469dbe46
DATA ·KoopConst+3752(SB)/8,$0x00000001ca68fe74

	/* x^20544 mod p(x), x^20480 mod p(x) */
DATA ·KoopConst+3760(SB)/8,$0x00000000fb093fd8
DATA ·KoopConst+3768(SB)/8,$0x00000001fd7d1b4c

	/* x^19520 mod p(x), x^19456 mod p(x) */
DATA ·KoopConst+3776(SB)/8,$0x00000000767a2bfe
DATA ·KoopConst+3784(SB)/8,$0x0000000055982d0c

	/* x^18496 mod p(x), x^18432 mod p(x) */
DATA ·KoopConst+3792(SB)/8,$0x00000001344e22bc
DATA ·KoopConst+3800(SB)/8,$0x00000000221553a6

	/* x^17472 mod p(x), x^17408 mod p(x) */
DATA ·KoopConst+3808(SB)/8,$0x0000000161cd9978
DATA ·KoopConst+3816(SB)/8,$0x000000013d9a153a

	/* x^16448 mod p(x), x^16384 mod p(x) */
DATA ·KoopConst+3824(SB)/8,$0x00000001d702e906
DATA ·KoopConst+3832(SB)/8,$0x00000001cd108b3c

	/* x^15424 mod p(x), x^15360 mod p(x) */
DATA ·KoopConst+3840(SB)/8,$0x00000001c7db9908
DATA ·KoopConst+3848(SB)/8,$0x00000001d0af0f4a

	/* x^14400 mod p(x), x^14336 mod p(x) */
DATA ·KoopConst+3856(SB)/8,$0x00000001665d025c
DATA ·KoopConst+3864(SB)/8,$0x00000001196cf0ec

	/* x^13376 mod p(x), x^13312 mod p(x) */
DATA ·KoopConst+3872(SB)/8,$0x000000012df97c0e
DATA ·KoopConst+3880(SB)/8,$0x00000001c88c9704

	/* x^12352 mod p(x), x^12288 mod p(x) */
DATA ·KoopConst+3888(SB)/8,$0x000000006fed84da
DATA ·KoopConst+3896(SB)/8,$0x000000002013d300

	/* x^11328 mod p(x), x^11264 mod p(x) */
DATA ·KoopConst+3904(SB)/8,$0x00000000b094146e
DATA ·KoopConst+3912(SB)/8,$0x00000001c458501e

	/* x^10304 mod p(x), x^10240 mod p(x) */
DATA ·KoopConst+3920(SB)/8,$0x00000001ceb518a6
DATA ·KoopConst+3928(SB)/8,$0x000000003ce14802

	/* x^9280 mod p(x), x^9216 mod p(x) */
DATA ·KoopConst+3936(SB)/8,$0x000000011f16db0a
DATA ·KoopConst+3944(SB)/8,$0x00000000bb72bb98

	/* x^8256 mod p(x), x^8192 mod p(x) */
DATA ·KoopConst+3952(SB)/8,$0x00000001d4aa130e
DATA ·KoopConst+3960(SB)/8,$0x00000000fb9aeaba

	/* x^7232 mod p(x), x^7168 mod p(x) */
DATA ·KoopConst+3968(SB)/8,$0x00000001991f01d2
DATA ·KoopConst+3976(SB)/8,$0x000000000131f5e6

	/* x^6208 mod p(x), x^6144 mod p(x) */
DATA ·KoopConst+3984(SB)/8,$0x000000006bd58b4c
DATA ·KoopConst+3992(SB)/8,$0x0000000089d5799a

	/* x^5184 mod p(x), x^5120 mod p(x) */
DATA ·KoopConst+4000(SB)/8,$0x000000007272c166
DATA ·KoopConst+4008(SB)/8,$0x00000000474c43b0

	/* x^4160 mod p(x), x^4096 mod p(x) */
DATA ·KoopConst+4016(SB)/8,$0x000000013974e6f8
DATA ·KoopConst+4024(SB)/8,$0x00000001db991f34

	/* x^3136 mod p(x), x^3072 mod p(x) */
DATA ·KoopConst+4032(SB)/8,$0x000000000bd6e03c
DATA ·KoopConst+4040(SB)/8,$0x000000004b1bfd00

	/* x^2112 mod p(x), x^2048 mod p(x) */
DATA ·KoopConst+4048(SB)/8,$0x000000005988c652
DATA ·KoopConst+4056(SB)/8,$0x000000004036b796

	/* x^1088 mod p(x), x^1024 mod p(x) */
DATA ·KoopConst+4064(SB)/8,$0x00000000129ef036
DATA ·KoopConst+4072(SB)/8,$0x000000000c5ec3d4

	/* x^2048 mod p(x), x^2016 mod p(x), x^1984 mod p(x), x^1952 mod p(x) */
DATA ·KoopConst+4080(SB)/8,$0xd6f94847201b5bcb
DATA ·KoopConst+4088(SB)/8,$0x1efc02e79571e892

	/* x^1920 mod p(x), x^1888 mod p(x), x^1856 mod p(x), x^1824 mod p(x) */
DATA ·KoopConst+4096(SB)/8,$0xce08adcc294c1393
DATA ·KoopConst+4104(SB)/8,$0x0b269b5c5ab5f161

	/* x^1792 mod p(x), x^1760 mod p(x), x^1728 mod p(x), x^1696 mod p(x) */
DATA ·KoopConst+4112(SB)/8,$0x17315505e4201e72
DATA ·KoopConst+4120(SB)/8,$0x2e841f4784acf3e9

	/* x^1664 mod p(x), x^1632 mod p(x), x^1600 mod p(x), x^1568 mod p(x) */
DATA ·KoopConst+4128(SB)/8,$0x37cfc3a67cc667e3
DATA ·KoopConst+4136(SB)/8,$0x7020425856bc424b

	/* x^1536 mod p(x), x^1504 mod p(x), x^1472 mod p(x), x^1440 mod p(x) */
DATA ·KoopConst+4144(SB)/8,$0x8e2fa3369218d2c3
DATA ·KoopConst+4152(SB)/8,$0xdf81bf923f7c6ef1

	/* x^1408 mod p(x), x^1376 mod p(x), x^1344 mod p(x), x^1312 mod p(x) */
DATA ·KoopConst+4160(SB)/8,$0x5ce20d2d39ed1981
DATA ·KoopConst+4168(SB)/8,$0x9d0898a0af5ddc43

	/* x^1280 mod p(x), x^1248 mod p(x), x^1216 mod p(x), x^1184 mod p(x) */
DATA ·KoopConst+4176(SB)/8,$0x6f7f4546ca081e03
DATA ·KoopConst+4184(SB)/8,$0x4992836903fda047

	/* x^1152 mod p(x), x^1120 mod p(x), x^1088 mod p(x), x^1056 mod p(x) */
DATA ·KoopConst+4192(SB)/8,$0xfd4f413b9bf11d68
DATA ·KoopConst+4200(SB)/8,$0xf4ddf452094f781b

	/* x^1024 mod p(x), x^992 mod p(x), x^960 mod p(x), x^928 mod p(x) */
DATA ·KoopConst+4208(SB)/8,$0x11d84204062f61ea
DATA ·KoopConst+4216(SB)/8,$0x9487f1e51f3588cf

	/* x^896 mod p(x), x^864 mod p(x), x^832 mod p(x), x^800 mod p(x) */
DATA ·KoopConst+4224(SB)/8,$0xfaedf111abf58a1f
DATA ·KoopConst+4232(SB)/8,$0x31da2c22b1384ec9

	/* x^768 mod p(x), x^736 mod p(x), x^704 mod p(x), x^672 mod p(x) */
DATA ·KoopConst+4240(SB)/8,$0x0246b541e8f81b22
DATA ·KoopConst+4248(SB)/8,$0xc857ede58a42eb47

	/* x^640 mod p(x), x^608 mod p(x), x^576 mod p(x), x^544 mod p(x) */
DATA ·KoopConst+4256(SB)/8,$0xd4dbfa9b92b0372e
DATA ·KoopConst+4264(SB)/8,$0xe0354c0b2cd1c09a

	/* x^512 mod p(x), x^480 mod p(x), x^448 mod p(x), x^416 mod p(x) */
DATA ·KoopConst+4272(SB)/8,$0x5f36c79cfc4417ec
DATA ·KoopConst+4280(SB)/8,$0x4b92cf8d54b8f25b

	/* x^384 mod p(x), x^352 mod p(x), x^320 mod p(x), x^288 mod p(x) */
DATA ·KoopConst+4288(SB)/8,$0xdad234918345041e
DATA ·KoopConst+4296(SB)/8,$0x4e44c81828229301

	/* x^256 mod p(x), x^224 mod p(x), x^192 mod p(x), x^160 mod p(x) */
DATA ·KoopConst+4304(SB)/8,$0x56fd28cc8e02f1d0
DATA ·KoopConst+4312(SB)/8,$0x3da5e43c8ee9ee84

	/* x^128 mod p(x), x^96 mod p(x), x^64 mod p(x), x^32 mod p(x) */
DATA ·KoopConst+4320(SB)/8,$0xa583017cdfcb9f08
DATA ·KoopConst+4328(SB)/8,$0xeb31d82e0c62ab26

GLOBL ·KoopConst(SB),RODATA,$4336

	/* Barrett constant m - (4^32)/n */
DATA ·KoopBarConst(SB)/8,$0x0000000017d232cd
DATA ·KoopBarConst+8(SB)/8,$0x0000000000000000
DATA ·KoopBarConst+16(SB)/8,$0x00000001d663b05d
DATA ·KoopBarConst+24(SB)/8,$0x0000000000000000
GLOBL ·KoopBarConst(SB),RODATA,$32

```

// === FILE: references/go/src/hash/crc32/gen.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run gen_const_ppc64le.go

package crc32

```

// === FILE: references/go/src/hash/crc32/gen_const_ppc64le.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

// Generate the constant table associated with the poly used by the
// vpmsumd crc32 algorithm.
//
// go run gen_const_ppc64le.go
//
// generates crc32_table_ppc64le.s

// The following is derived from code written by Anton Blanchard
// <anton@au.ibm.com> found at https://github.com/antonblanchard/crc32-vpmsum.
// The original is dual licensed under GPL and Apache 2.  As the copyright holder
// for the work, IBM has contributed this new work under the golang license.

// This code was written in Go based on the original C implementation.

// This is a tool needed to generate the appropriate constants needed for
// the vpmsum algorithm.  It is included to generate new constant tables if
// new polynomial values are included in the future.

package main

import (
	"bytes"
	"fmt"
	"os"
)

var blocking = 32 * 1024

func reflect_bits(b uint64, nr uint) uint64 {
	var ref uint64

	for bit := uint64(0); bit < uint64(nr); bit++ {
		if (b & uint64(1)) == 1 {
			ref |= (1 << (uint64(nr-1) - bit))
		}
		b = (b >> 1)
	}
	return ref
}

func get_remainder(poly uint64, deg uint, n uint) uint64 {

	rem, _ := xnmodp(n, poly, deg)
	return rem
}

func get_quotient(poly uint64, bits, n uint) uint64 {

	_, div := xnmodp(n, poly, bits)
	return div
}

// xnmodp returns two values, p and div:
// p is the representation of the binary polynomial x**n mod (x ** deg + "poly")
// That is p is the binary representation of the modulus polynomial except for its highest-order term.
// div is the binary representation of the polynomial x**n / (x ** deg + "poly")
func xnmodp(n uint, poly uint64, deg uint) (uint64, uint64) {

	var mod, mask, high, div uint64

	if n < deg {
		div = 0
		return poly, div
	}
	mask = 1<<deg - 1
	poly &= mask
	mod = poly
	div = 1
	deg--
	n--
	for n > deg {
		high = (mod >> deg) & 1
		div = (div << 1) | high
		mod <<= 1
		if high != 0 {
			mod ^= poly
		}
		n--
	}
	return mod & mask, div
}

func main() {
	w := new(bytes.Buffer)

	// Standard: https://go.dev/s/generatedcode
	fmt.Fprintln(w, `// Code generated by "go run gen_const_ppc64le.go"; DO NOT EDIT.`)
	fmt.Fprintln(w)
	fmt.Fprintln(w, `#include "textflag.h"`)

	// These are the polynomials supported in vector now.
	// If adding others, include the polynomial and a name
	// to identify it.

	genCrc32ConstTable(w, 0xedb88320, "IEEE")
	genCrc32ConstTable(w, 0x82f63b78, "Cast")
	genCrc32ConstTable(w, 0xeb31d82e, "Koop")
	b := w.Bytes()

	err := os.WriteFile("crc32_table_ppc64le.s", b, 0666)
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't write output: %s\n", err)
	}
}

func genCrc32ConstTable(w *bytes.Buffer, poly uint32, polyid string) {

	ref_poly := reflect_bits(uint64(poly), 32)
	fmt.Fprintf(w, "\n\t/* Reduce %d kbits to 1024 bits */\n", blocking*8)
	j := 0
	for i := (blocking * 8) - 1024; i > 0; i -= 1024 {
		a := reflect_bits(get_remainder(ref_poly, 32, uint(i)), 32) << 1
		b := reflect_bits(get_remainder(ref_poly, 32, uint(i+64)), 32) << 1

		fmt.Fprintf(w, "\t/* x^%d mod p(x)%s, x^%d mod p(x)%s */\n", uint(i+64), "", uint(i), "")
		fmt.Fprintf(w, "DATA ·%sConst+%d(SB)/8,$0x%016x\n", polyid, j*8, b)
		fmt.Fprintf(w, "DATA ·%sConst+%d(SB)/8,$0x%016x\n", polyid, (j+1)*8, a)

		j += 2
		fmt.Fprintf(w, "\n")
	}

	for i := (1024 * 2) - 128; i >= 0; i -= 128 {
		a := reflect_bits(get_remainder(ref_poly, 32, uint(i+32)), 32)
		b := reflect_bits(get_remainder(ref_poly, 32, uint(i+64)), 32)
		c := reflect_bits(get_remainder(ref_poly, 32, uint(i+96)), 32)
		d := reflect_bits(get_remainder(ref_poly, 32, uint(i+128)), 32)

		fmt.Fprintf(w, "\t/* x^%d mod p(x)%s, x^%d mod p(x)%s, x^%d mod p(x)%s, x^%d mod p(x)%s */\n", i+128, "", i+96, "", i+64, "", i+32, "")
		fmt.Fprintf(w, "DATA ·%sConst+%d(SB)/8,$0x%08x%08x\n", polyid, j*8, c, d)
		fmt.Fprintf(w, "DATA ·%sConst+%d(SB)/8,$0x%08x%08x\n", polyid, (j+1)*8, a, b)

		j += 2
		fmt.Fprintf(w, "\n")
	}

	fmt.Fprintf(w, "GLOBL ·%sConst(SB),RODATA,$4336\n", polyid)
	fmt.Fprintf(w, "\n\t/* Barrett constant m - (4^32)/n */\n")
	fmt.Fprintf(w, "DATA ·%sBarConst(SB)/8,$0x%016x\n", polyid, reflect_bits(get_quotient(ref_poly, 32, 64), 33))
	fmt.Fprintf(w, "DATA ·%sBarConst+8(SB)/8,$0x0000000000000000\n", polyid)
	fmt.Fprintf(w, "DATA ·%sBarConst+16(SB)/8,$0x%016x\n", polyid, reflect_bits((uint64(1)<<32)|ref_poly, 33)) // reflected?
	fmt.Fprintf(w, "DATA ·%sBarConst+24(SB)/8,$0x0000000000000000\n", polyid)
	fmt.Fprintf(w, "GLOBL ·%sBarConst(SB),RODATA,$32\n", polyid)
}

```

// === FILE: references/go/src/hash/crc64/crc64.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package crc64 implements the 64-bit cyclic redundancy check, or CRC-64,
// checksum. See https://en.wikipedia.org/wiki/Cyclic_redundancy_check for
// information.
package crc64

import (
	"errors"
	"hash"
	"internal/byteorder"
	"sync"
)

// The size of a CRC-64 checksum in bytes.
const Size = 8

// Predefined polynomials.
const (
	// The ISO polynomial, defined in ISO 3309 and used in HDLC.
	ISO = 0xD800000000000000

	// The ECMA polynomial, defined in ECMA 182.
	ECMA = 0xC96C5795D7870F42
)

// Table is a 256-word table representing the polynomial for efficient processing.
type Table [256]uint64

var (
	slicing8TableISO  *[8]Table
	slicing8TableECMA *[8]Table
)

var buildSlicing8TablesOnce = sync.OnceFunc(buildSlicing8Tables)

func buildSlicing8Tables() {
	slicing8TableISO = makeSlicingBy8Table(makeTable(ISO))
	slicing8TableECMA = makeSlicingBy8Table(makeTable(ECMA))
}

// MakeTable returns a [Table] constructed from the specified polynomial.
// The contents of this [Table] must not be modified.
func MakeTable(poly uint64) *Table {
	buildSlicing8TablesOnce()
	switch poly {
	case ISO:
		return &slicing8TableISO[0]
	case ECMA:
		return &slicing8TableECMA[0]
	default:
		return makeTable(poly)
	}
}

func makeTable(poly uint64) *Table {
	t := new(Table)
	for i := 0; i < 256; i++ {
		crc := uint64(i)
		for j := 0; j < 8; j++ {
			if crc&1 == 1 {
				crc = (crc >> 1) ^ poly
			} else {
				crc >>= 1
			}
		}
		t[i] = crc
	}
	return t
}

func makeSlicingBy8Table(t *Table) *[8]Table {
	var helperTable [8]Table
	helperTable[0] = *t
	for i := 0; i < 256; i++ {
		crc := t[i]
		for j := 1; j < 8; j++ {
			crc = t[crc&0xff] ^ (crc >> 8)
			helperTable[j][i] = crc
		}
	}
	return &helperTable
}

// digest represents the partial evaluation of a checksum.
type digest struct {
	crc uint64
	tab *Table
}

// New creates a new hash.Hash64 computing the CRC-64 checksum using the
// polynomial represented by the [Table]. Its Sum method will lay the
// value out in big-endian byte order. The returned Hash64 also
// implements [encoding.BinaryMarshaler] and [encoding.BinaryUnmarshaler] to
// marshal and unmarshal the internal state of the hash.
func New(tab *Table) hash.Hash64 { return &digest{0, tab} }

func (d *digest) Size() int { return Size }

func (d *digest) BlockSize() int { return 1 }

func (d *digest) Reset() { d.crc = 0 }

const (
	magic         = "crc\x02"
	marshaledSize = len(magic) + 8 + 8
)

func (d *digest) AppendBinary(b []byte) ([]byte, error) {
	b = append(b, magic...)
	b = byteorder.BEAppendUint64(b, tableSum(d.tab))
	b = byteorder.BEAppendUint64(b, d.crc)
	return b, nil
}

func (d *digest) MarshalBinary() ([]byte, error) {
	return d.AppendBinary(make([]byte, 0, marshaledSize))
}

func (d *digest) UnmarshalBinary(b []byte) error {
	if len(b) < len(magic) || string(b[:len(magic)]) != magic {
		return errors.New("hash/crc64: invalid hash state identifier")
	}
	if len(b) != marshaledSize {
		return errors.New("hash/crc64: invalid hash state size")
	}
	if tableSum(d.tab) != byteorder.BEUint64(b[4:]) {
		return errors.New("hash/crc64: tables do not match")
	}
	d.crc = byteorder.BEUint64(b[12:])
	return nil
}

func (d *digest) Clone() (hash.Cloner, error) {
	r := *d
	return &r, nil
}

func update(crc uint64, tab *Table, p []byte) uint64 {
	buildSlicing8TablesOnce()
	crc = ^crc
	// Table comparison is somewhat expensive, so avoid it for small sizes
	for len(p) >= 64 {
		var helperTable *[8]Table
		if *tab == slicing8TableECMA[0] {
			helperTable = slicing8TableECMA
		} else if *tab == slicing8TableISO[0] {
			helperTable = slicing8TableISO
			// For smaller sizes creating extended table takes too much time
		} else if len(p) >= 2048 {
			// According to the tests between various x86 and arm CPUs, 2k is a reasonable
			// threshold for now. This may change in the future.
			helperTable = makeSlicingBy8Table(tab)
		} else {
			break
		}
		// Update using slicing-by-8
		for len(p) > 8 {
			crc ^= byteorder.LEUint64(p)
			crc = helperTable[7][crc&0xff] ^
				helperTable[6][(crc>>8)&0xff] ^
				helperTable[5][(crc>>16)&0xff] ^
				helperTable[4][(crc>>24)&0xff] ^
				helperTable[3][(crc>>32)&0xff] ^
				helperTable[2][(crc>>40)&0xff] ^
				helperTable[1][(crc>>48)&0xff] ^
				helperTable[0][crc>>56]
			p = p[8:]
		}
	}
	// For reminders or small sizes
	for _, v := range p {
		crc = tab[byte(crc)^v] ^ (crc >> 8)
	}
	return ^crc
}

// Update returns the result of adding the bytes in p to the crc.
func Update(crc uint64, tab *Table, p []byte) uint64 {
	return update(crc, tab, p)
}

func (d *digest) Write(p []byte) (n int, err error) {
	d.crc = update(d.crc, d.tab, p)
	return len(p), nil
}

func (d *digest) Sum64() uint64 { return d.crc }

func (d *digest) Sum(in []byte) []byte {
	s := d.Sum64()
	return append(in, byte(s>>56), byte(s>>48), byte(s>>40), byte(s>>32), byte(s>>24), byte(s>>16), byte(s>>8), byte(s))
}

// Checksum returns the CRC-64 checksum of data
// using the polynomial represented by the [Table].
func Checksum(data []byte, tab *Table) uint64 { return update(0, tab, data) }

// tableSum returns the ISO checksum of table t.
func tableSum(t *Table) uint64 {
	var a [2048]byte
	b := a[:0]
	if t != nil {
		for _, x := range t {
			b = byteorder.BEAppendUint64(b, x)
		}
	}
	return Checksum(b, MakeTable(ISO))
}

```

// === FILE: references/go/src/hash/fnv/fnv.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fnv implements FNV-1 and FNV-1a, non-cryptographic hash functions
// created by Glenn Fowler, Landon Curt Noll, and Phong Vo.
// See
// https://en.wikipedia.org/wiki/Fowler-Noll-Vo_hash_function.
//
// All the hash.Hash implementations returned by this package also
// implement encoding.BinaryMarshaler and encoding.BinaryUnmarshaler to
// marshal and unmarshal the internal state of the hash.
package fnv

import (
	"errors"
	"hash"
	"internal/byteorder"
	"math/bits"
)

type (
	sum32   uint32
	sum32a  uint32
	sum64   uint64
	sum64a  uint64
	sum128  [2]uint64
	sum128a [2]uint64
)

const (
	offset32        = 2166136261
	offset64        = 14695981039346656037
	offset128Lower  = 0x62b821756295c58d
	offset128Higher = 0x6c62272e07bb0142
	prime32         = 16777619
	prime64         = 1099511628211
	prime128Lower   = 0x13b
	prime128Shift   = 24
)

// New32 returns a new 32-bit FNV-1 [hash.Hash].
// Its Sum method will lay the value out in big-endian byte order.
func New32() hash.Hash32 {
	var s sum32 = offset32
	return &s
}

// New32a returns a new 32-bit FNV-1a [hash.Hash].
// Its Sum method will lay the value out in big-endian byte order.
func New32a() hash.Hash32 {
	var s sum32a = offset32
	return &s
}

// New64 returns a new 64-bit FNV-1 [hash.Hash].
// Its Sum method will lay the value out in big-endian byte order.
func New64() hash.Hash64 {
	var s sum64 = offset64
	return &s
}

// New64a returns a new 64-bit FNV-1a [hash.Hash].
// Its Sum method will lay the value out in big-endian byte order.
func New64a() hash.Hash64 {
	var s sum64a = offset64
	return &s
}

// New128 returns a new 128-bit FNV-1 [hash.Hash].
// Its Sum method will lay the value out in big-endian byte order.
func New128() hash.Hash {
	var s sum128
	s[0] = offset128Higher
	s[1] = offset128Lower
	return &s
}

// New128a returns a new 128-bit FNV-1a [hash.Hash].
// Its Sum method will lay the value out in big-endian byte order.
func New128a() hash.Hash {
	var s sum128a
	s[0] = offset128Higher
	s[1] = offset128Lower
	return &s
}

func (s *sum32) Reset()   { *s = offset32 }
func (s *sum32a) Reset()  { *s = offset32 }
func (s *sum64) Reset()   { *s = offset64 }
func (s *sum64a) Reset()  { *s = offset64 }
func (s *sum128) Reset()  { s[0] = offset128Higher; s[1] = offset128Lower }
func (s *sum128a) Reset() { s[0] = offset128Higher; s[1] = offset128Lower }

func (s *sum32) Sum32() uint32  { return uint32(*s) }
func (s *sum32a) Sum32() uint32 { return uint32(*s) }
func (s *sum64) Sum64() uint64  { return uint64(*s) }
func (s *sum64a) Sum64() uint64 { return uint64(*s) }

func (s *sum32) Write(data []byte) (int, error) {
	hash := *s
	for _, c := range data {
		hash *= prime32
		hash ^= sum32(c)
	}
	*s = hash
	return len(data), nil
}

func (s *sum32a) Write(data []byte) (int, error) {
	hash := *s
	for _, c := range data {
		hash ^= sum32a(c)
		hash *= prime32
	}
	*s = hash
	return len(data), nil
}

func (s *sum64) Write(data []byte) (int, error) {
	hash := *s
	for _, c := range data {
		hash *= prime64
		hash ^= sum64(c)
	}
	*s = hash
	return len(data), nil
}

func (s *sum64a) Write(data []byte) (int, error) {
	hash := *s
	for _, c := range data {
		hash ^= sum64a(c)
		hash *= prime64
	}
	*s = hash
	return len(data), nil
}

func (s *sum128) Write(data []byte) (int, error) {
	for _, c := range data {
		// Compute the multiplication
		s0, s1 := bits.Mul64(prime128Lower, s[1])
		s0 += s[1]<<prime128Shift + prime128Lower*s[0]
		// Update the values
		s[1] = s1
		s[0] = s0
		s[1] ^= uint64(c)
	}
	return len(data), nil
}

func (s *sum128a) Write(data []byte) (int, error) {
	for _, c := range data {
		s[1] ^= uint64(c)
		// Compute the multiplication
		s0, s1 := bits.Mul64(prime128Lower, s[1])
		s0 += s[1]<<prime128Shift + prime128Lower*s[0]
		// Update the values
		s[1] = s1
		s[0] = s0
	}
	return len(data), nil
}

func (s *sum32) Size() int   { return 4 }
func (s *sum32a) Size() int  { return 4 }
func (s *sum64) Size() int   { return 8 }
func (s *sum64a) Size() int  { return 8 }
func (s *sum128) Size() int  { return 16 }
func (s *sum128a) Size() int { return 16 }

func (s *sum32) BlockSize() int   { return 1 }
func (s *sum32a) BlockSize() int  { return 1 }
func (s *sum64) BlockSize() int   { return 1 }
func (s *sum64a) BlockSize() int  { return 1 }
func (s *sum128) BlockSize() int  { return 1 }
func (s *sum128a) BlockSize() int { return 1 }

func (s *sum32) Sum(in []byte) []byte {
	v := uint32(*s)
	return byteorder.BEAppendUint32(in, v)
}

func (s *sum32a) Sum(in []byte) []byte {
	v := uint32(*s)
	return byteorder.BEAppendUint32(in, v)
}

func (s *sum64) Sum(in []byte) []byte {
	v := uint64(*s)
	return byteorder.BEAppendUint64(in, v)
}

func (s *sum64a) Sum(in []byte) []byte {
	v := uint64(*s)
	return byteorder.BEAppendUint64(in, v)
}

func (s *sum128) Sum(in []byte) []byte {
	ret := byteorder.BEAppendUint64(in, s[0])
	return byteorder.BEAppendUint64(ret, s[1])
}

func (s *sum128a) Sum(in []byte) []byte {
	ret := byteorder.BEAppendUint64(in, s[0])
	return byteorder.BEAppendUint64(ret, s[1])
}

const (
	magic32          = "fnv\x01"
	magic32a         = "fnv\x02"
	magic64          = "fnv\x03"
	magic64a         = "fnv\x04"
	magic128         = "fnv\x05"
	magic128a        = "fnv\x06"
	marshaledSize32  = len(magic32) + 4
	marshaledSize64  = len(magic64) + 8
	marshaledSize128 = len(magic128) + 8*2
)

func (s *sum32) AppendBinary(b []byte) ([]byte, error) {
	b = append(b, magic32...)
	b = byteorder.BEAppendUint32(b, uint32(*s))
	return b, nil
}

func (s *sum32) MarshalBinary() ([]byte, error) {
	return s.AppendBinary(make([]byte, 0, marshaledSize32))
}

func (s *sum32a) AppendBinary(b []byte) ([]byte, error) {
	b = append(b, magic32a...)
	b = byteorder.BEAppendUint32(b, uint32(*s))
	return b, nil
}

func (s *sum32a) MarshalBinary() ([]byte, error) {
	return s.AppendBinary(make([]byte, 0, marshaledSize32))
}

func (s *sum64) AppendBinary(b []byte) ([]byte, error) {
	b = append(b, magic64...)
	b = byteorder.BEAppendUint64(b, uint64(*s))
	return b, nil
}

func (s *sum64) MarshalBinary() ([]byte, error) {
	return s.AppendBinary(make([]byte, 0, marshaledSize64))
}

func (s *sum64a) AppendBinary(b []byte) ([]byte, error) {
	b = append(b, magic64a...)
	b = byteorder.BEAppendUint64(b, uint64(*s))
	return b, nil
}

func (s *sum64a) MarshalBinary() ([]byte, error) {
	return s.AppendBinary(make([]byte, 0, marshaledSize64))
}

func (s *sum128) AppendBinary(b []byte) ([]byte, error) {
	b = append(b, magic128...)
	b = byteorder.BEAppendUint64(b, s[0])
	b = byteorder.BEAppendUint64(b, s[1])
	return b, nil
}

func (s *sum128) MarshalBinary() ([]byte, error) {
	return s.AppendBinary(make([]byte, 0, marshaledSize128))
}

func (s *sum128a) AppendBinary(b []byte) ([]byte, error) {
	b = append(b, magic128a...)
	b = byteorder.BEAppendUint64(b, s[0])
	b = byteorder.BEAppendUint64(b, s[1])
	return b, nil
}

func (s *sum128a) MarshalBinary() ([]byte, error) {
	return s.AppendBinary(make([]byte, 0, marshaledSize128))
}

func (s *sum32) UnmarshalBinary(b []byte) error {
	if len(b) < len(magic32) || string(b[:len(magic32)]) != magic32 {
		return errors.New("hash/fnv: invalid hash state identifier")
	}
	if len(b) != marshaledSize32 {
		return errors.New("hash/fnv: invalid hash state size")
	}
	*s = sum32(byteorder.BEUint32(b[4:]))
	return nil
}

func (s *sum32a) UnmarshalBinary(b []byte) error {
	if len(b) < len(magic32a) || string(b[:len(magic32a)]) != magic32a {
		return errors.New("hash/fnv: invalid hash state identifier")
	}
	if len(b) != marshaledSize32 {
		return errors.New("hash/fnv: invalid hash state size")
	}
	*s = sum32a(byteorder.BEUint32(b[4:]))
	return nil
}

func (s *sum64) UnmarshalBinary(b []byte) error {
	if len(b) < len(magic64) || string(b[:len(magic64)]) != magic64 {
		return errors.New("hash/fnv: invalid hash state identifier")
	}
	if len(b) != marshaledSize64 {
		return errors.New("hash/fnv: invalid hash state size")
	}
	*s = sum64(byteorder.BEUint64(b[4:]))
	return nil
}

func (s *sum64a) UnmarshalBinary(b []byte) error {
	if len(b) < len(magic64a) || string(b[:len(magic64a)]) != magic64a {
		return errors.New("hash/fnv: invalid hash state identifier")
	}
	if len(b) != marshaledSize64 {
		return errors.New("hash/fnv: invalid hash state size")
	}
	*s = sum64a(byteorder.BEUint64(b[4:]))
	return nil
}

func (s *sum128) UnmarshalBinary(b []byte) error {
	if len(b) < len(magic128) || string(b[:len(magic128)]) != magic128 {
		return errors.New("hash/fnv: invalid hash state identifier")
	}
	if len(b) != marshaledSize128 {
		return errors.New("hash/fnv: invalid hash state size")
	}
	s[0] = byteorder.BEUint64(b[4:])
	s[1] = byteorder.BEUint64(b[12:])
	return nil
}

func (s *sum128a) UnmarshalBinary(b []byte) error {
	if len(b) < len(magic128a) || string(b[:len(magic128a)]) != magic128a {
		return errors.New("hash/fnv: invalid hash state identifier")
	}
	if len(b) != marshaledSize128 {
		return errors.New("hash/fnv: invalid hash state size")
	}
	s[0] = byteorder.BEUint64(b[4:])
	s[1] = byteorder.BEUint64(b[12:])
	return nil
}

func (d *sum32) Clone() (hash.Cloner, error) {
	r := *d
	return &r, nil
}

func (d *sum32a) Clone() (hash.Cloner, error) {
	r := *d
	return &r, nil
}

func (d *sum64) Clone() (hash.Cloner, error) {
	r := *d
	return &r, nil
}

func (d *sum64a) Clone() (hash.Cloner, error) {
	r := *d
	return &r, nil
}

func (d *sum128) Clone() (hash.Cloner, error) {
	r := *d
	return &r, nil
}

func (d *sum128a) Clone() (hash.Cloner, error) {
	r := *d
	return &r, nil
}

```

// === FILE: references/go/src/hash/hash.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package hash provides interfaces for hash functions.
package hash

import "io"

// Hash is the common interface implemented by all hash functions.
//
// Hash implementations in the standard library (e.g. [hash/crc32] and
// [crypto/sha256]) implement the [encoding.BinaryMarshaler], [encoding.BinaryAppender],
// [encoding.BinaryUnmarshaler] and [Cloner] interfaces. Marshaling a hash implementation
// allows its internal state to be saved and used for additional processing
// later, without having to re-write the data previously written to the hash.
// The hash state may contain portions of the input in its original form,
// which users are expected to handle for any possible security implications.
//
// Compatibility: Any future changes to hash or crypto packages will endeavor
// to maintain compatibility with state encoded using previous versions.
// That is, any released versions of the packages should be able to
// decode data written with any previously released version,
// subject to issues such as security fixes.
// See the Go compatibility document for background: https://golang.org/doc/go1compat
type Hash interface {
	// Write (via the embedded io.Writer interface) adds more data to the running hash.
	// It never returns an error.
	io.Writer

	// Sum appends the current hash to b and returns the resulting slice.
	// It does not change the underlying hash state.
	Sum(b []byte) []byte

	// Reset resets the Hash to its initial state.
	Reset()

	// Size returns the number of bytes Sum will return.
	Size() int

	// BlockSize returns the hash's underlying block size.
	// The Write method must be able to accept any amount
	// of data, but it may operate more efficiently if all writes
	// are a multiple of the block size.
	BlockSize() int
}

// Hash32 is the common interface implemented by all 32-bit hash functions.
type Hash32 interface {
	Hash
	Sum32() uint32
}

// Hash64 is the common interface implemented by all 64-bit hash functions.
type Hash64 interface {
	Hash
	Sum64() uint64
}

// A Cloner is a hash function whose state can be cloned, returning a value with
// equivalent and independent state.
//
// All [Hash] implementations in the standard library implement this interface,
// unless GOFIPS140=v1.0.0 is set.
//
// If a hash can only determine at runtime if it can be cloned (e.g. if it wraps
// another hash), Clone may return an error wrapping [errors.ErrUnsupported].
// Otherwise, Clone must always return a nil error.
type Cloner interface {
	Hash
	Clone() (Cloner, error)
}

// XOF (extendable output function) is a hash function with arbitrary or unlimited output length.
type XOF interface {
	// Write absorbs more data into the XOF's state. It panics if called
	// after Read.
	io.Writer

	// Read reads more output from the XOF. It may return io.EOF if there
	// is a limit to the XOF output length.
	io.Reader

	// Reset resets the XOF to its initial state.
	Reset()

	// BlockSize returns the XOF's underlying block size.
	// The Write method must be able to accept any amount
	// of data, but it may operate more efficiently if all writes
	// are a multiple of the block size.
	BlockSize() int
}

```

// === FILE: references/go/src/hash/maphash/hasher.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package maphash

// A Hasher defines the interface between a hash-based container and its elements.
// It provides a hash function and an equivalence relation over values
// of type T, enabling those values to be inserted in hash tables
// and similar data structures.
//
// Of course, comparable types can already be used as keys of Go's
// built-in map type, but a Hasher enables non-comparable types to be
// used as keys of a suitable hash table too.
// Hashers may be useful even for comparable types, to define an
// equivalence relation that differs from the usual one (==), such as a
// field-based comparison for a pointer-to-struct type, or a
// case-insensitive comparison for strings, as in this example:
//
//	// CaseInsensitive is a Hasher[string] whose
//	// equivalence relation ignores letter case.
//	type CaseInsensitive struct{}
//
//	func (CaseInsensitive) Hash(h *Hash, s string) {
//		h.WriteString(strings.ToLower(s))
//	}
//
//	func (CaseInsensitive) Equal(x, y string) bool {
//		// (We avoid strings.EqualFold as it is not
//		// consistent with ToLower for all values.)
//		return strings.ToLower(x) == strings.ToLower(y)
//	}
//
// A Hasher also permits values to be used with other hash-based data
// structures such as a Bloom filter.
// The [ComparableHasher] type makes it convenient to enable comparable
// types to be used in such data structures under their usual (==)
// equivalence relation.
//
// # Hash invariants
//
// If two values are equal as defined by Equal(x, y), then they must
// have the same hash as defined by the effects of Hash(h, x) on h.
//
// Hashers must be logically stateless: the behavior of the Hash and
// Equal methods depends only on the arguments.
//
// # Writing a good function
//
// When defining a hash function and equivalence relation for a data
// type, it may help to first define a canonical encoding for values
// of that type as a sequence of elements, each being a number,
// string, boolean, or pointer.
// An encoding is canonical if two values that are logically equal
// have the same encoding, even if they are represented differently.
// For example, a canonical case-insensitive encoding of a string is
// [strings.ToLower].
//
// Once you have defined the encoding, the Hasher's Hash method should
// encode a value into the [Hash] using a sequence of calls to
// [Hash.Write] for byte slices, [Hash.WriteString] for strings,
// [Hash.WriteByte] for bytes, and [WriteComparable] for elements of
// other types. The Hasher's Equal method should compute the
// encodings of two values, then compare their corresponding
// elements, returning false at the first mismatch.
//
// A Hash method may discard information so long as it remains
// consistent with the Equal method as defined above.
// For example, valid implementations of CaseInsensitive.Hash might inspect
// only the first letter of the string, or even use a constant value.
// However, the lossier the hash function, the more frequent
// the hash collisions and the slower the hash table.
//
// Some data types, such as sets, are inherently unordered: the set
// {a, b, c} is equal to the set {c, b, a}.
// In some cases it is possible to define a canonical encoding for a
// set by sorting the elements into some order.
// In other cases this may inefficient, since it may require allocating
// memory, or infeasible, as when there is no convenient order.
// Another way to hash an unordered set is to compute the hash
// for each element separately, then combine all the element hashes
// using a commutative (order-independent) operator such as + or ^.
//
// The Hash method below, for a hypothetical Set type, illustrates
// this approach:
//
//	type Set[T comparable] struct{ ... }
//
//	type setHasher[T comparable] struct{}
//
//	func (setHasher[T]) Hash(hash *maphash.Hash, set *Set[T]) {
//		var accum uint64
//		for elem := range set.Elements() {
//			// Initialize a hasher for the element,
//			// using same seed as the outer hash.
//			var sub maphash.Hash
//			sub.SetSeed(hash.Seed())
//
//			// Hash the element.
//			maphash.WriteComparable(&sub, elem)
//
//			// Mix the element's hash into the set's hash.
//			accum ^= sub.Sum64()
//		}
//		maphash.WriteComparable(hash, accum)
//	}
//
// In many languages, a data type's hash operation simply returns an
// integer value.
// However, that makes it possible for an adversary to systematically
// construct a large number of values that all have the same hash,
// degrading the asymptotic performance of hash tables in a
// denial-of-service attack known as "hash flooding".
// By contrast, computing hashes as a sequence of values emitted into
// a [Hash] with an unpredictable [Seed] that varies from one hash
// table to another mitigates this attack.
//
// In effect, the Seed chooses one of 2⁶⁴ different hash functions.
// The code example above calls SetSeed on the element's sub-Hasher
// so that it uses the same hash function as for the Set itself, and
// not a random one.
type Hasher[T any] interface {
	Hash(*Hash, T)
	Equal(x, y T) bool
}

// ComparableHasher is an implementation of [Hasher] whose
// Equal(x, y) method is consistent with x == y.
//
// ComparableHasher is defined only for comparable types.
// The type system will not prevent you from instantiating a type
// such as ComparableHasher[any]; nonetheless you must not pass
// non-comparable argument values to its Hash or Equal methods.
type ComparableHasher[T comparable] struct {
	_ [0]func(T) // disallow comparison, and conversion between ComparableHasher[X] and ComparableHasher[Y]
}

func (ComparableHasher[T]) Hash(h *Hash, v T) { WriteComparable(h, v) }
func (ComparableHasher[T]) Equal(x, y T) bool { return x == y }

```

// === FILE: references/go/src/hash/maphash/maphash.go ===
```go
// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package maphash provides hash functions on byte sequences and comparable values.
// It also defines [Hasher], the interface between a hash function and a hash table.
//
// These hash functions are intended to be used to implement hash
// tables, Bloom filters, and other data structures that need to map
// arbitrary strings or byte sequences to a uniform distribution on
// unsigned 64-bit integers.
//
// Each different instance of a hash table or data structure should use its own [Seed].
//
// The hash functions are not cryptographically secure.
// (See crypto/sha256 and crypto/sha512 for cryptographic use.)
package maphash

import (
	"hash"
	"internal/abi"
	"internal/runtime/maps"
	"unsafe"
)

// A Seed is a random value that selects the specific hash function
// computed by a [Hash]. If two Hashes use the same Seeds, they
// will compute the same hash values for any given input.
// If two Hashes use different Seeds, they are very likely to compute
// distinct hash values for any given input.
//
// A Seed must be initialized by calling [MakeSeed].
// The zero seed is uninitialized and not valid for use with [Hash]'s SetSeed method.
//
// Each Seed value is local to a single process and cannot be serialized
// or otherwise recreated in a different process.
type Seed struct {
	s uint64
}

// Bytes returns the hash of b with the given seed.
//
// Bytes is equivalent to, but more convenient and efficient than:
//
//	var h Hash
//	h.SetSeed(seed)
//	h.Write(b)
//	return h.Sum64()
func Bytes(seed Seed, b []byte) uint64 {
	state := seed.s
	if state == 0 {
		panic("maphash: use of uninitialized Seed")
	}

	if len(b) > bufSize {
		b = b[:len(b):len(b)] // merge len and cap calculations when reslicing
		for len(b) > bufSize {
			state = rthash(b[:bufSize], state)
			b = b[bufSize:]
		}
	}
	return rthash(b, state)
}

// String returns the hash of s with the given seed.
//
// String is equivalent to, but more convenient and efficient than:
//
//	var h Hash
//	h.SetSeed(seed)
//	h.WriteString(s)
//	return h.Sum64()
func String(seed Seed, s string) uint64 {
	state := seed.s
	if state == 0 {
		panic("maphash: use of uninitialized Seed")
	}
	for len(s) > bufSize {
		state = rthashString(s[:bufSize], state)
		s = s[bufSize:]
	}
	return rthashString(s, state)
}

// A Hash computes a seeded hash of a byte sequence.
//
// The zero Hash is a valid Hash ready to use.
// A zero Hash chooses a random seed for itself during
// the first call to a Reset, Write, Seed, Clone, or Sum64 method.
// For control over the seed, use SetSeed.
//
// The computed hash values depend only on the initial seed and
// the sequence of bytes provided to the Hash object, not on the way
// in which the bytes are provided. For example, the three sequences
//
//	h.Write([]byte{'f','o','o'})
//	h.WriteByte('f'); h.WriteByte('o'); h.WriteByte('o')
//	h.WriteString("foo")
//
// all have the same effect.
//
// Hashes are intended to be collision-resistant, even for situations
// where an adversary controls the byte sequences being hashed.
//
// A Hash is not safe for concurrent use by multiple goroutines, but a Seed is.
// If multiple goroutines must compute the same seeded hash,
// each can declare its own Hash and call SetSeed with a common Seed.
type Hash struct {
	_     [0]func()     // not comparable
	seed  Seed          // initial seed used for this hash
	state Seed          // current hash of all flushed bytes
	buf   [bufSize]byte // unflushed byte buffer
	n     int           // number of unflushed bytes
}

// bufSize is the size of the Hash write buffer.
// The buffer ensures that writes depend only on the sequence of bytes,
// not the sequence of WriteByte/Write/WriteString calls,
// by always calling rthash with a full buffer (except for the tail).
const bufSize = 128

// initSeed seeds the hash if necessary.
// initSeed is called lazily before any operation that actually uses h.seed/h.state.
// Note that this does not include Write/WriteByte/WriteString in the case
// where they only add to h.buf. (If they write too much, they call h.flush,
// which does call h.initSeed.)
func (h *Hash) initSeed() {
	if h.seed.s == 0 {
		seed := MakeSeed()
		h.seed = seed
		h.state = seed
	}
}

// WriteByte adds b to the sequence of bytes hashed by h.
// It never fails; the error result is for implementing [io.ByteWriter].
func (h *Hash) WriteByte(b byte) error {
	if h.n == len(h.buf) {
		h.flush()
	}
	h.buf[h.n] = b
	h.n++
	return nil
}

// Write adds b to the sequence of bytes hashed by h.
// It always writes all of b and never fails; the count and error result are for implementing [io.Writer].
func (h *Hash) Write(b []byte) (int, error) {
	size := len(b)
	// Deal with bytes left over in h.buf.
	// h.n <= bufSize is always true.
	// Checking it is ~free and it lets the compiler eliminate a bounds check.
	if h.n > 0 && h.n <= bufSize {
		k := copy(h.buf[h.n:], b)
		h.n += k
		if h.n < bufSize {
			// Copied the entirety of b to h.buf.
			return size, nil
		}
		b = b[k:]
		h.flush()
		// No need to set h.n = 0 here; it happens just before exit.
	}
	// Process as many full buffers as possible, without copying, and calling initSeed only once.
	if len(b) > bufSize {
		h.initSeed()
		for len(b) > bufSize {
			h.state.s = rthash(b[:bufSize], h.state.s)
			b = b[bufSize:]
		}
	}
	// Copy the tail.
	copy(h.buf[:], b)
	h.n = len(b)
	return size, nil
}

// WriteString adds the bytes of s to the sequence of bytes hashed by h.
// It always writes all of s and never fails; the count and error result are for implementing [io.StringWriter].
func (h *Hash) WriteString(s string) (int, error) {
	// WriteString mirrors Write. See Write for comments.
	size := len(s)
	if h.n > 0 && h.n <= bufSize {
		k := copy(h.buf[h.n:], s)
		h.n += k
		if h.n < bufSize {
			return size, nil
		}
		s = s[k:]
		h.flush()
	}
	if len(s) > bufSize {
		h.initSeed()
		for len(s) > bufSize {
			h.state.s = rthashString(s[:bufSize], h.state.s)
			s = s[bufSize:]
		}
	}
	copy(h.buf[:], s)
	h.n = len(s)
	return size, nil
}

// Seed returns h's seed value.
func (h *Hash) Seed() Seed {
	h.initSeed()
	return h.seed
}

// SetSeed sets h to use seed, which must have been returned by [MakeSeed]
// or by another [Hash.Seed] method.
// Two [Hash] objects with the same seed behave identically.
// Two [Hash] objects with different seeds will very likely behave differently.
// Any bytes added to h before this call will be discarded.
func (h *Hash) SetSeed(seed Seed) {
	if seed.s == 0 {
		panic("maphash: use of uninitialized Seed")
	}
	h.seed = seed
	h.state = seed
	h.n = 0
}

// Reset discards all bytes added to h.
// (The seed remains the same.)
func (h *Hash) Reset() {
	h.initSeed()
	h.state = h.seed
	h.n = 0
}

// precondition: buffer is full.
func (h *Hash) flush() {
	if h.n != len(h.buf) {
		panic("maphash: flush of partially full buffer")
	}
	h.initSeed()
	h.state.s = rthash(h.buf[:h.n], h.state.s)
	h.n = 0
}

// Sum64 returns h's current 64-bit value, which depends on
// h's seed and the sequence of bytes added to h since the
// last call to [Hash.Reset] or [Hash.SetSeed].
//
// All bits of the Sum64 result are close to uniformly and
// independently distributed, so it can be safely reduced
// by using bit masking, shifting, or modular arithmetic.
func (h *Hash) Sum64() uint64 {
	h.initSeed()
	return rthash(h.buf[:h.n], h.state.s)
}

// MakeSeed returns a new random seed.
func MakeSeed() Seed {
	var s uint64
	for {
		s = randUint64()
		// We use seed 0 to indicate an uninitialized seed/hash,
		// so keep trying until we get a non-zero seed.
		if s != 0 {
			break
		}
	}
	return Seed{s: s}
}

// Sum appends the hash's current 64-bit value to b.
// It exists for implementing [hash.Hash].
// For direct calls, it is more efficient to use [Hash.Sum64].
func (h *Hash) Sum(b []byte) []byte {
	x := h.Sum64()
	return append(b,
		byte(x>>0),
		byte(x>>8),
		byte(x>>16),
		byte(x>>24),
		byte(x>>32),
		byte(x>>40),
		byte(x>>48),
		byte(x>>56))
}

// Size returns h's hash value size, 8 bytes.
func (h *Hash) Size() int { return 8 }

// BlockSize returns h's block size.
func (h *Hash) BlockSize() int { return len(h.buf) }

// Clone implements [hash.Cloner].
func (h *Hash) Clone() (hash.Cloner, error) {
	h.initSeed()
	r := *h
	return &r, nil
}

// Comparable returns the hash of comparable value v with the given seed
// such that Comparable(s, v1) == Comparable(s, v2) if v1 == v2.
// If v != v, then the resulting hash is randomly distributed.
func Comparable[T comparable](seed Seed, v T) uint64 {
	abi.EscapeNonString(v)
	return comparableHash(v, seed)
}

// WriteComparable adds x to the data hashed by h.
func WriteComparable[T comparable](h *Hash, x T) {
	abi.EscapeNonString(x)
	// writeComparable directly operates on h.state
	// without using h.buf. Mix in the buffer length so it won't
	// commute with a buffered write, which either changes h.n or changes
	// h.state.
	if h.n != 0 {
		writeComparable(h, h.n)
	}
	writeComparable(h, x)
}

//go:linkname runtime_rand runtime.rand
func runtime_rand() uint64

//go:linkname runtime_memhash runtime.memhash
//go:noescape
func runtime_memhash(p unsafe.Pointer, seed, s uintptr) uintptr

func rthash(buf []byte, seed uint64) uint64 {
	if len(buf) == 0 {
		return seed
	}
	len := len(buf)
	// The runtime hasher only works on uintptr. For 64-bit
	// architectures, we use the hasher directly. Otherwise,
	// we use two parallel hashers on the lower and upper 32 bits.
	if maps.Use64BitHash {
		return uint64(runtime_memhash(unsafe.Pointer(&buf[0]), uintptr(seed), uintptr(len)))
	}
	lo := runtime_memhash(unsafe.Pointer(&buf[0]), uintptr(uint32(seed)), uintptr(len))
	hi := runtime_memhash(unsafe.Pointer(&buf[0]), uintptr(seed>>32), uintptr(len))
	return uint64(hi)<<32 | uint64(lo)
}

func rthashString(s string, state uint64) uint64 {
	buf := unsafe.Slice(unsafe.StringData(s), len(s))
	return rthash(buf, state)
}

func randUint64() uint64 {
	return runtime_rand()
}

func comparableHash[T comparable](v T, seed Seed) uint64 {
	s := seed.s
	var m map[T]struct{}
	mTyp := abi.TypeOf(m)
	hasher := (*abi.MapType)(unsafe.Pointer(mTyp)).Hasher
	if maps.Use64BitHash {
		return uint64(hasher(abi.NoEscape(unsafe.Pointer(&v)), uintptr(s)))
	}
	lo := hasher(abi.NoEscape(unsafe.Pointer(&v)), uintptr(uint32(s)))
	hi := hasher(abi.NoEscape(unsafe.Pointer(&v)), uintptr(s>>32))
	return uint64(hi)<<32 | uint64(lo)
}

func writeComparable[T comparable](h *Hash, v T) {
	h.state.s = comparableHash(v, h.state)
}

```

// === FILE: references/go/src/hash/test_cases.txt ===
```text

a
ab
abc
abcd
abcde
abcdef
abcdefg
abcdefgh
abcdefghi
abcdefghij
Discard medicine more than two years old.
He who has a shady past knows that nice guys finish last.
I wouldn't marry him with a ten foot pole.
Free! Free!/A trip/to Mars/for 900/empty jars/Burma Shave
The days of the digital watch are numbered.  -Tom Stoppard
Nepal premier won't resign.
For every action there is an equal and opposite government program.
His money is twice tainted: 'taint yours and 'taint mine.
There is no reason for any individual to have a computer in their home. -Ken Olsen, 1977
It's a tiny change to the code and not completely disgusting. - Bob Manchek
size:  a.out:  bad magic
The major problem is with sendmail.  -Mark Horton
Give me a rock, paper and scissors and I will move the world.  CCFestoon
If the enemy is within range, then so are you.
It's well we cannot hear the screams/That we create in others' dreams.
You remind me of a TV show, but that's all right: I watch it anyway.
C is as portable as Stonehedge!!
Even if I could be Shakespeare, I think I should still choose to be Faraday. - A. Huxley
The fugacity of a constituent in a mixture of gases at a given temperature is proportional to its mole fraction.  Lewis-Randall Rule
How can you write a big system without C++?  -Paul Glick

```

// === FILE: references/go/src/hash/test_gen.awk ===
```text
# Copyright 2009 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# awk -f test_gen.awk test_cases.txt
# generates test case table.
# edit next line to set particular reference implementation and name.
BEGIN { cmd = "echo -n `9 sha1sum`"; name = "Sha1Test" }
{
	printf("\t%s{ \"", name);
	printf("%s", $0) |cmd;
	close(cmd);
	printf("\", \"%s\" },\n", $0);
}

```

