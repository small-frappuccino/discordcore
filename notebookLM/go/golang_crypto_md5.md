# Domain Architecture: crypto/md5

## Layout Topology
```text
crypto/md5/
├── _asm
│   ├── go.mod
│   ├── go.sum
│   └── md5block_amd64_asm.go
├── gen.go
├── md5.go
├── md5block.go
├── md5block_386.s
├── md5block_amd64.s
├── md5block_arm.s
├── md5block_arm64.s
├── md5block_decl.go
├── md5block_generic.go
├── md5block_loong64.s
├── md5block_ppc64x.s
├── md5block_riscv64.s
└── md5block_s390x.s
```

## Source Stream Aggregation

// === FILE: references!/go/src/crypto/md5/_asm/go.mod ===
```text
module crypto/md5/_asm

go 1.24

require github.com/mmcloughlin/avo v0.6.0

require (
	golang.org/x/mod v0.20.0 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/tools v0.24.0 // indirect
)

```

// === FILE: references!/go/src/crypto/md5/_asm/go.sum ===
```text
github.com/mmcloughlin/avo v0.6.0 h1:QH6FU8SKoTLaVs80GA8TJuLNkUYl4VokHKlPhVDg4YY=
github.com/mmcloughlin/avo v0.6.0/go.mod h1:8CoAGaCSYXtCPR+8y18Y9aB/kxb8JSS6FRI7mSkvD+8=
golang.org/x/mod v0.20.0 h1:utOm6MM3R3dnawAiJgn0y+xvuYRsm1RKM/4giyfDgV0=
golang.org/x/mod v0.20.0/go.mod h1:hTbmBsO62+eylJbnUtE2MGJUyE7QWk4xUqPFrRgJ+7c=
golang.org/x/sync v0.8.0 h1:3NFvSEYkUoMifnESzZl15y791HH1qU2xm6eCJU5ZPXQ=
golang.org/x/sync v0.8.0/go.mod h1:Czt+wKu1gCyEFDUtn0jG5QVvpJ6rzVqr5aXyt9drQfk=
golang.org/x/tools v0.24.0 h1:J1shsA93PJUEVaUSaay7UXAyE8aimq3GW0pjlolpa24=
golang.org/x/tools v0.24.0/go.mod h1:YhNqVBIfWHdzvTLs0d8LCuMhkKUgSUKldakyV7W/WDQ=

```

// === FILE: references!/go/src/crypto/md5/_asm/md5block_amd64_asm.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Original source:
//	http://www.zorinaq.com/papers/md5-amd64.html
//	http://www.zorinaq.com/papers/md5-amd64.tar.bz2
//
// Translated from Perl generating GNU assembly into
// #defines generating 6a assembly by the Go Authors.

package main

import (
	. "github.com/mmcloughlin/avo/build"
	. "github.com/mmcloughlin/avo/operand"
	. "github.com/mmcloughlin/avo/reg"
)

//go:generate go run . -out ../md5block_amd64.s -pkg md5

func main() {
	Package("crypto/md5")
	ConstraintExpr("!purego")
	block()
	Generate()
}

// MD5 optimized for AMD64.
//
// Author: Marc Bevand <bevand_m (at) epita.fr>
// Licence: I hereby disclaim the copyright on this code and place it
// in the public domain.
func block() {
	Implement("block")
	Attributes(NOSPLIT)
	AllocLocal(8)

	Load(Param("dig"), RBP)
	Load(Param("p").Base(), RSI)
	Load(Param("p").Len(), RDX)
	SHRQ(Imm(6), RDX)
	SHLQ(Imm(6), RDX)

	LEAQ(Mem{Base: SI, Index: DX, Scale: 1}, RDI)
	MOVL(Mem{Base: BP}.Offset(0*4), EAX)
	MOVL(Mem{Base: BP}.Offset(1*4), EBX)
	MOVL(Mem{Base: BP}.Offset(2*4), ECX)
	MOVL(Mem{Base: BP}.Offset(3*4), EDX)
	MOVL(Imm(0xffffffff), R11L)

	CMPQ(RSI, RDI)
	JEQ(LabelRef("end"))

	loop()
	end()
}

func loop() {
	Label("loop")
	MOVL(EAX, R12L)
	MOVL(EBX, R13L)
	MOVL(ECX, R14L)
	MOVL(EDX, R15L)

	MOVL(Mem{Base: SI}.Offset(0*4), R8L)
	MOVL(EDX, R9L)

	ROUND1(EAX, EBX, ECX, EDX, 1, 0xd76aa478, 7)
	ROUND1(EDX, EAX, EBX, ECX, 2, 0xe8c7b756, 12)
	ROUND1(ECX, EDX, EAX, EBX, 3, 0x242070db, 17)
	ROUND1(EBX, ECX, EDX, EAX, 4, 0xc1bdceee, 22)
	ROUND1(EAX, EBX, ECX, EDX, 5, 0xf57c0faf, 7)
	ROUND1(EDX, EAX, EBX, ECX, 6, 0x4787c62a, 12)
	ROUND1(ECX, EDX, EAX, EBX, 7, 0xa8304613, 17)
	ROUND1(EBX, ECX, EDX, EAX, 8, 0xfd469501, 22)
	ROUND1(EAX, EBX, ECX, EDX, 9, 0x698098d8, 7)
	ROUND1(EDX, EAX, EBX, ECX, 10, 0x8b44f7af, 12)
	ROUND1(ECX, EDX, EAX, EBX, 11, 0xffff5bb1, 17)
	ROUND1(EBX, ECX, EDX, EAX, 12, 0x895cd7be, 22)
	ROUND1(EAX, EBX, ECX, EDX, 13, 0x6b901122, 7)
	ROUND1(EDX, EAX, EBX, ECX, 14, 0xfd987193, 12)
	ROUND1(ECX, EDX, EAX, EBX, 15, 0xa679438e, 17)
	ROUND1(EBX, ECX, EDX, EAX, 1, 0x49b40821, 22)

	MOVL(EDX, R9L)
	MOVL(EDX, R10L)

	ROUND2(EAX, EBX, ECX, EDX, 6, 0xf61e2562, 5)
	ROUND2(EDX, EAX, EBX, ECX, 11, 0xc040b340, 9)
	ROUND2(ECX, EDX, EAX, EBX, 0, 0x265e5a51, 14)
	ROUND2(EBX, ECX, EDX, EAX, 5, 0xe9b6c7aa, 20)
	ROUND2(EAX, EBX, ECX, EDX, 10, 0xd62f105d, 5)
	ROUND2(EDX, EAX, EBX, ECX, 15, 0x2441453, 9)
	ROUND2(ECX, EDX, EAX, EBX, 4, 0xd8a1e681, 14)
	ROUND2(EBX, ECX, EDX, EAX, 9, 0xe7d3fbc8, 20)
	ROUND2(EAX, EBX, ECX, EDX, 14, 0x21e1cde6, 5)
	ROUND2(EDX, EAX, EBX, ECX, 3, 0xc33707d6, 9)
	ROUND2(ECX, EDX, EAX, EBX, 8, 0xf4d50d87, 14)
	ROUND2(EBX, ECX, EDX, EAX, 13, 0x455a14ed, 20)
	ROUND2(EAX, EBX, ECX, EDX, 2, 0xa9e3e905, 5)
	ROUND2(EDX, EAX, EBX, ECX, 7, 0xfcefa3f8, 9)
	ROUND2(ECX, EDX, EAX, EBX, 12, 0x676f02d9, 14)
	ROUND2(EBX, ECX, EDX, EAX, 5, 0x8d2a4c8a, 20)

	MOVL(ECX, R9L)

	ROUND3FIRST(EAX, EBX, ECX, EDX, 8, 0xfffa3942, 4)
	ROUND3(EDX, EAX, EBX, ECX, 11, 0x8771f681, 11)
	ROUND3(ECX, EDX, EAX, EBX, 14, 0x6d9d6122, 16)
	ROUND3(EBX, ECX, EDX, EAX, 1, 0xfde5380c, 23)
	ROUND3(EAX, EBX, ECX, EDX, 4, 0xa4beea44, 4)
	ROUND3(EDX, EAX, EBX, ECX, 7, 0x4bdecfa9, 11)
	ROUND3(ECX, EDX, EAX, EBX, 10, 0xf6bb4b60, 16)
	ROUND3(EBX, ECX, EDX, EAX, 13, 0xbebfbc70, 23)
	ROUND3(EAX, EBX, ECX, EDX, 0, 0x289b7ec6, 4)
	ROUND3(EDX, EAX, EBX, ECX, 3, 0xeaa127fa, 11)
	ROUND3(ECX, EDX, EAX, EBX, 6, 0xd4ef3085, 16)
	ROUND3(EBX, ECX, EDX, EAX, 9, 0x4881d05, 23)
	ROUND3(EAX, EBX, ECX, EDX, 12, 0xd9d4d039, 4)
	ROUND3(EDX, EAX, EBX, ECX, 15, 0xe6db99e5, 11)
	ROUND3(ECX, EDX, EAX, EBX, 2, 0x1fa27cf8, 16)
	ROUND3(EBX, ECX, EDX, EAX, 0, 0xc4ac5665, 23)

	MOVL(R11L, R9L)
	XORL(EDX, R9L)

	ROUND4(EAX, EBX, ECX, EDX, 7, 0xf4292244, 6)
	ROUND4(EDX, EAX, EBX, ECX, 14, 0x432aff97, 10)
	ROUND4(ECX, EDX, EAX, EBX, 5, 0xab9423a7, 15)
	ROUND4(EBX, ECX, EDX, EAX, 12, 0xfc93a039, 21)
	ROUND4(EAX, EBX, ECX, EDX, 3, 0x655b59c3, 6)
	ROUND4(EDX, EAX, EBX, ECX, 10, 0x8f0ccc92, 10)
	ROUND4(ECX, EDX, EAX, EBX, 1, 0xffeff47d, 15)
	ROUND4(EBX, ECX, EDX, EAX, 8, 0x85845dd1, 21)
	ROUND4(EAX, EBX, ECX, EDX, 15, 0x6fa87e4f, 6)
	ROUND4(EDX, EAX, EBX, ECX, 6, 0xfe2ce6e0, 10)
	ROUND4(ECX, EDX, EAX, EBX, 13, 0xa3014314, 15)
	ROUND4(EBX, ECX, EDX, EAX, 4, 0x4e0811a1, 21)
	ROUND4(EAX, EBX, ECX, EDX, 11, 0xf7537e82, 6)
	ROUND4(EDX, EAX, EBX, ECX, 2, 0xbd3af235, 10)
	ROUND4(ECX, EDX, EAX, EBX, 9, 0x2ad7d2bb, 15)
	ROUND4(EBX, ECX, EDX, EAX, 0, 0xeb86d391, 21)

	ADDL(R12L, EAX)
	ADDL(R13L, EBX)
	ADDL(R14L, ECX)
	ADDL(R15L, EDX)

	ADDQ(Imm(64), RSI)
	CMPQ(RSI, RDI)
	JB(LabelRef("loop"))
}

func end() {
	Label("end")
	MOVL(EAX, Mem{Base: BP}.Offset(0*4))
	MOVL(EBX, Mem{Base: BP}.Offset(1*4))
	MOVL(ECX, Mem{Base: BP}.Offset(2*4))
	MOVL(EDX, Mem{Base: BP}.Offset(3*4))
	RET()
}

func ROUND1(a, b, c, d GPPhysical, index int, konst, shift uint64) {
	XORL(c, R9L)
	ADDL(Imm(konst), a)
	ADDL(R8L, a)
	ANDL(b, R9L)
	XORL(d, R9L)
	MOVL(Mem{Base: SI}.Offset(index*4), R8L)
	ADDL(R9L, a)
	ROLL(Imm(shift), a)
	MOVL(c, R9L)
	ADDL(b, a)
}

// Uses https://github.com/animetosho/md5-optimisation#dependency-shortcut-in-g-function
func ROUND2(a, b, c, d GPPhysical, index int, konst, shift uint64) {
	XORL(R11L, R9L)
	ADDL(Imm(konst), a)
	ADDL(R8L, a)
	ANDL(b, R10L)
	ANDL(c, R9L)
	MOVL(Mem{Base: SI}.Offset(index*4), R8L)
	ADDL(R9L, a)
	ADDL(R10L, a)
	MOVL(c, R9L)
	MOVL(c, R10L)
	ROLL(Imm(shift), a)
	ADDL(b, a)
}

// Uses https://github.com/animetosho/md5-optimisation#h-function-re-use
func ROUND3FIRST(a, b, c, d GPPhysical, index int, konst, shift uint64) {
	MOVL(d, R9L)
	XORL(c, R9L)
	XORL(b, R9L)
	ADDL(Imm(konst), a)
	ADDL(R8L, a)
	MOVL(Mem{Base: SI}.Offset(index*4), R8L)
	ADDL(R9L, a)
	ROLL(Imm(shift), a)
	ADDL(b, a)
}

func ROUND3(a, b, c, d GPPhysical, index int, konst, shift uint64) {
	XORL(a, R9L)
	XORL(b, R9L)
	ADDL(Imm(konst), a)
	ADDL(R8L, a)
	MOVL(Mem{Base: SI}.Offset(index*4), R8L)
	ADDL(R9L, a)
	ROLL(Imm(shift), a)
	ADDL(b, a)
}

func ROUND4(a, b, c, d GPPhysical, index int, konst, shift uint64) {
	ADDL(Imm(konst), a)
	ADDL(R8L, a)
	ORL(b, R9L)
	XORL(c, R9L)
	ADDL(R9L, a)
	MOVL(Mem{Base: SI}.Offset(index*4), R8L)
	MOVL(Imm(0xffffffff), R9L)
	ROLL(Imm(shift), a)
	XORL(c, R9L)
	ADDL(b, a)
}

```

// === FILE: references!/go/src/crypto/md5/gen.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

// This program generates md5block.go
// Invoke as
//
//	go run gen.go -output md5block.go

package main

import (
	"bytes"
	"flag"
	"go/format"
	"log"
	"os"
	"strings"
	"text/template"
)

var filename = flag.String("output", "md5block.go", "output file name")

func main() {
	flag.Parse()

	var buf bytes.Buffer

	t := template.Must(template.New("main").Funcs(funcs).Parse(program))
	if err := t.Execute(&buf, data); err != nil {
		log.Fatal(err)
	}

	data, err := format.Source(buf.Bytes())
	if err != nil {
		log.Fatal(err)
	}
	err = os.WriteFile(*filename, data, 0644)
	if err != nil {
		log.Fatal(err)
	}
}

type Data struct {
	a, b, c, d string
	Shift1     []int
	Shift2     []int
	Shift3     []int
	Shift4     []int
	Table1     []uint32
	Table2     []uint32
	Table3     []uint32
	Table4     []uint32
}

var funcs = template.FuncMap{
	"dup":     dup,
	"relabel": relabel,
	"rotate":  rotate,
	"idx":     idx,
	"seq":     seq,
}

func dup(count int, x []int) []int {
	var out []int
	for i := 0; i < count; i++ {
		out = append(out, x...)
	}
	return out
}

func relabel(s string) string {
	return strings.NewReplacer("arg0", data.a, "arg1", data.b, "arg2", data.c, "arg3", data.d).Replace(s)
}

func rotate() string {
	data.a, data.b, data.c, data.d = data.d, data.a, data.b, data.c
	return "" // no output
}

func idx(round, index int) int {
	v := 0
	switch round {
	case 1:
		v = index
	case 2:
		v = (1 + 5*index) & 15
	case 3:
		v = (5 + 3*index) & 15
	case 4:
		v = (7 * index) & 15
	}
	return v
}

func seq(i int) []int {
	s := make([]int, i)
	for i := range s {
		s[i] = i
	}
	return s
}

var data = Data{
	a:      "a",
	b:      "b",
	c:      "c",
	d:      "d",
	Shift1: []int{7, 12, 17, 22},
	Shift2: []int{5, 9, 14, 20},
	Shift3: []int{4, 11, 16, 23},
	Shift4: []int{6, 10, 15, 21},

	// table[i] = int((1<<32) * abs(sin(i+1 radians))).
	Table1: []uint32{
		// round 1
		0xd76aa478,
		0xe8c7b756,
		0x242070db,
		0xc1bdceee,
		0xf57c0faf,
		0x4787c62a,
		0xa8304613,
		0xfd469501,
		0x698098d8,
		0x8b44f7af,
		0xffff5bb1,
		0x895cd7be,
		0x6b901122,
		0xfd987193,
		0xa679438e,
		0x49b40821,
	},
	Table2: []uint32{
		// round 2
		0xf61e2562,
		0xc040b340,
		0x265e5a51,
		0xe9b6c7aa,
		0xd62f105d,
		0x2441453,
		0xd8a1e681,
		0xe7d3fbc8,
		0x21e1cde6,
		0xc33707d6,
		0xf4d50d87,
		0x455a14ed,
		0xa9e3e905,
		0xfcefa3f8,
		0x676f02d9,
		0x8d2a4c8a,
	},
	Table3: []uint32{
		// round 3
		0xfffa3942,
		0x8771f681,
		0x6d9d6122,
		0xfde5380c,
		0xa4beea44,
		0x4bdecfa9,
		0xf6bb4b60,
		0xbebfbc70,
		0x289b7ec6,
		0xeaa127fa,
		0xd4ef3085,
		0x4881d05,
		0xd9d4d039,
		0xe6db99e5,
		0x1fa27cf8,
		0xc4ac5665,
	},
	Table4: []uint32{
		// round 4
		0xf4292244,
		0x432aff97,
		0xab9423a7,
		0xfc93a039,
		0x655b59c3,
		0x8f0ccc92,
		0xffeff47d,
		0x85845dd1,
		0x6fa87e4f,
		0xfe2ce6e0,
		0xa3014314,
		0x4e0811a1,
		0xf7537e82,
		0xbd3af235,
		0x2ad7d2bb,
		0xeb86d391,
	},
}

var program = `// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by go run gen.go -output md5block.go; DO NOT EDIT.

package md5

import (
	"internal/byteorder"
	"math/bits"
)

func blockGeneric(dig *digest, p []byte) {
	// load state
	a, b, c, d := dig.s[0], dig.s[1], dig.s[2], dig.s[3]

	for i := 0; i <= len(p)-BlockSize; i += BlockSize {
		// eliminate bounds checks on p
		q := p[i:]
		q = q[:BlockSize:BlockSize]

		// save current state
		aa, bb, cc, dd := a, b, c, d

		// load input block
		{{range $i := seq 16 -}}
			{{printf "x%x := byteorder.LEUint32(q[4*%#x:])" $i $i}}
		{{end}}

		// round 1
		{{range $i, $s := dup 4 .Shift1 -}}
			{{printf "arg0 = arg1 + bits.RotateLeft32((((arg2^arg3)&arg1)^arg3)+arg0+x%x+%#08x, %d)" (idx 1 $i) (index $.Table1 $i) $s | relabel}}
			{{rotate -}}
		{{end}}

		// round 2
		{{range $i, $s := dup 4 .Shift2 -}}
			{{printf "arg0 = arg1 + bits.RotateLeft32((((arg1^arg2)&arg3)^arg2)+arg0+x%x+%#08x, %d)" (idx 2 $i) (index $.Table2 $i) $s | relabel}}
			{{rotate -}}
		{{end}}

		// round 3
		{{range $i, $s := dup 4 .Shift3 -}}
			{{printf "arg0 = arg1 + bits.RotateLeft32((arg1^arg2^arg3)+arg0+x%x+%#08x, %d)" (idx 3 $i) (index $.Table3 $i) $s | relabel}}
			{{rotate -}}
		{{end}}

		// round 4
		{{range $i, $s := dup 4 .Shift4 -}}
			{{printf "arg0 = arg1 + bits.RotateLeft32((arg2^(arg1|^arg3))+arg0+x%x+%#08x, %d)" (idx 4 $i) (index $.Table4 $i) $s | relabel}}
			{{rotate -}}
		{{end}}

		// add saved state
		a += aa
		b += bb
		c += cc
		d += dd
	}

	// save state
	dig.s[0], dig.s[1], dig.s[2], dig.s[3] = a, b, c, d
}
`

```

// === FILE: references!/go/src/crypto/md5/md5.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run gen.go -output md5block.go

// Package md5 implements the MD5 hash algorithm as defined in RFC 1321.
//
// MD5 is cryptographically broken and should not be used for secure
// applications.
package md5

import (
	"crypto"
	"crypto/internal/fips140only"
	"errors"
	"hash"
	"internal/byteorder"
)

func init() {
	crypto.RegisterHash(crypto.MD5, New)
}

// The size of an MD5 checksum in bytes.
const Size = 16

// The blocksize of MD5 in bytes.
const BlockSize = 64

// The maximum number of bytes that can be passed to block(). The limit exists
// because implementations that rely on assembly routines are not preemptible.
const maxAsmIters = 1024
const maxAsmSize = BlockSize * maxAsmIters // 64KiB

const (
	init0 = 0x67452301
	init1 = 0xEFCDAB89
	init2 = 0x98BADCFE
	init3 = 0x10325476
)

// digest represents the partial evaluation of a checksum.
type digest struct {
	s   [4]uint32
	x   [BlockSize]byte
	nx  int
	len uint64
}

func (d *digest) Reset() {
	d.s[0] = init0
	d.s[1] = init1
	d.s[2] = init2
	d.s[3] = init3
	d.nx = 0
	d.len = 0
}

const (
	magic         = "md5\x01"
	marshaledSize = len(magic) + 4*4 + BlockSize + 8
)

func (d *digest) MarshalBinary() ([]byte, error) {
	return d.AppendBinary(make([]byte, 0, marshaledSize))
}

func (d *digest) AppendBinary(b []byte) ([]byte, error) {
	b = append(b, magic...)
	b = byteorder.BEAppendUint32(b, d.s[0])
	b = byteorder.BEAppendUint32(b, d.s[1])
	b = byteorder.BEAppendUint32(b, d.s[2])
	b = byteorder.BEAppendUint32(b, d.s[3])
	b = append(b, d.x[:d.nx]...)
	b = append(b, make([]byte, len(d.x)-d.nx)...)
	b = byteorder.BEAppendUint64(b, d.len)
	return b, nil
}

func (d *digest) UnmarshalBinary(b []byte) error {
	if len(b) < len(magic) || string(b[:len(magic)]) != magic {
		return errors.New("crypto/md5: invalid hash state identifier")
	}
	if len(b) != marshaledSize {
		return errors.New("crypto/md5: invalid hash state size")
	}
	b = b[len(magic):]
	b, d.s[0] = consumeUint32(b)
	b, d.s[1] = consumeUint32(b)
	b, d.s[2] = consumeUint32(b)
	b, d.s[3] = consumeUint32(b)
	b = b[copy(d.x[:], b):]
	b, d.len = consumeUint64(b)
	d.nx = int(d.len % BlockSize)
	return nil
}

func consumeUint64(b []byte) ([]byte, uint64) {
	return b[8:], byteorder.BEUint64(b[0:8])
}

func consumeUint32(b []byte) ([]byte, uint32) {
	return b[4:], byteorder.BEUint32(b[0:4])
}

func (d *digest) Clone() (hash.Cloner, error) {
	r := *d
	return &r, nil
}

// New returns a new [hash.Hash] computing the MD5 checksum. The Hash
// also implements [encoding.BinaryMarshaler], [encoding.BinaryAppender] and
// [encoding.BinaryUnmarshaler] to marshal and unmarshal the internal
// state of the hash.
func New() hash.Hash {
	d := new(digest)
	d.Reset()
	return d
}

func (d *digest) Size() int { return Size }

func (d *digest) BlockSize() int { return BlockSize }

func (d *digest) Write(p []byte) (nn int, err error) {
	if fips140only.Enforced() {
		return 0, errors.New("crypto/md5: use of MD5 is not allowed in FIPS 140-only mode")
	}
	// Note that we currently call block or blockGeneric
	// directly (guarded using haveAsm) because this allows
	// escape analysis to see that p and d don't escape.
	nn = len(p)
	d.len += uint64(nn)
	if d.nx > 0 {
		n := copy(d.x[d.nx:], p)
		d.nx += n
		if d.nx == BlockSize {
			if haveAsm {
				block(d, d.x[:])
			} else {
				blockGeneric(d, d.x[:])
			}
			d.nx = 0
		}
		p = p[n:]
	}
	if len(p) >= BlockSize {
		n := len(p) &^ (BlockSize - 1)
		if haveAsm {
			for n > maxAsmSize {
				block(d, p[:maxAsmSize])
				p = p[maxAsmSize:]
				n -= maxAsmSize
			}
			block(d, p[:n])
		} else {
			blockGeneric(d, p[:n])
		}
		p = p[n:]
	}
	if len(p) > 0 {
		d.nx = copy(d.x[:], p)
	}
	return
}

func (d *digest) Sum(in []byte) []byte {
	// Make a copy of d so that caller can keep writing and summing.
	d0 := *d
	hash := d0.checkSum()
	return append(in, hash[:]...)
}

func (d *digest) checkSum() [Size]byte {
	if fips140only.Enforced() {
		panic("crypto/md5: use of MD5 is not allowed in FIPS 140-only mode")
	}

	// Append 0x80 to the end of the message and then append zeros
	// until the length is a multiple of 56 bytes. Finally append
	// 8 bytes representing the message length in bits.
	//
	// 1 byte end marker :: 0-63 padding bytes :: 8 byte length
	tmp := [1 + 63 + 8]byte{0x80}
	pad := (55 - d.len) % 64                     // calculate number of padding bytes
	byteorder.LEPutUint64(tmp[1+pad:], d.len<<3) // append length in bits
	d.Write(tmp[:1+pad+8])

	// The previous write ensures that a whole number of
	// blocks (i.e. a multiple of 64 bytes) have been hashed.
	if d.nx != 0 {
		panic("d.nx != 0")
	}

	var digest [Size]byte
	byteorder.LEPutUint32(digest[0:], d.s[0])
	byteorder.LEPutUint32(digest[4:], d.s[1])
	byteorder.LEPutUint32(digest[8:], d.s[2])
	byteorder.LEPutUint32(digest[12:], d.s[3])
	return digest
}

// Sum returns the MD5 checksum of the data.
func Sum(data []byte) [Size]byte {
	var d digest
	d.Reset()
	d.Write(data)
	return d.checkSum()
}

```

// === FILE: references!/go/src/crypto/md5/md5block.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by go run gen.go -output md5block.go; DO NOT EDIT.

package md5

import (
	"internal/byteorder"
	"math/bits"
)

func blockGeneric(dig *digest, p []byte) {
	// load state
	a, b, c, d := dig.s[0], dig.s[1], dig.s[2], dig.s[3]

	for i := 0; i <= len(p)-BlockSize; i += BlockSize {
		// eliminate bounds checks on p
		q := p[i:]
		q = q[:BlockSize:BlockSize]

		// save current state
		aa, bb, cc, dd := a, b, c, d

		// load input block
		x0 := byteorder.LEUint32(q[4*0x0:])
		x1 := byteorder.LEUint32(q[4*0x1:])
		x2 := byteorder.LEUint32(q[4*0x2:])
		x3 := byteorder.LEUint32(q[4*0x3:])
		x4 := byteorder.LEUint32(q[4*0x4:])
		x5 := byteorder.LEUint32(q[4*0x5:])
		x6 := byteorder.LEUint32(q[4*0x6:])
		x7 := byteorder.LEUint32(q[4*0x7:])
		x8 := byteorder.LEUint32(q[4*0x8:])
		x9 := byteorder.LEUint32(q[4*0x9:])
		xa := byteorder.LEUint32(q[4*0xa:])
		xb := byteorder.LEUint32(q[4*0xb:])
		xc := byteorder.LEUint32(q[4*0xc:])
		xd := byteorder.LEUint32(q[4*0xd:])
		xe := byteorder.LEUint32(q[4*0xe:])
		xf := byteorder.LEUint32(q[4*0xf:])

		// round 1
		a = b + bits.RotateLeft32((((c^d)&b)^d)+a+x0+0xd76aa478, 7)
		d = a + bits.RotateLeft32((((b^c)&a)^c)+d+x1+0xe8c7b756, 12)
		c = d + bits.RotateLeft32((((a^b)&d)^b)+c+x2+0x242070db, 17)
		b = c + bits.RotateLeft32((((d^a)&c)^a)+b+x3+0xc1bdceee, 22)
		a = b + bits.RotateLeft32((((c^d)&b)^d)+a+x4+0xf57c0faf, 7)
		d = a + bits.RotateLeft32((((b^c)&a)^c)+d+x5+0x4787c62a, 12)
		c = d + bits.RotateLeft32((((a^b)&d)^b)+c+x6+0xa8304613, 17)
		b = c + bits.RotateLeft32((((d^a)&c)^a)+b+x7+0xfd469501, 22)
		a = b + bits.RotateLeft32((((c^d)&b)^d)+a+x8+0x698098d8, 7)
		d = a + bits.RotateLeft32((((b^c)&a)^c)+d+x9+0x8b44f7af, 12)
		c = d + bits.RotateLeft32((((a^b)&d)^b)+c+xa+0xffff5bb1, 17)
		b = c + bits.RotateLeft32((((d^a)&c)^a)+b+xb+0x895cd7be, 22)
		a = b + bits.RotateLeft32((((c^d)&b)^d)+a+xc+0x6b901122, 7)
		d = a + bits.RotateLeft32((((b^c)&a)^c)+d+xd+0xfd987193, 12)
		c = d + bits.RotateLeft32((((a^b)&d)^b)+c+xe+0xa679438e, 17)
		b = c + bits.RotateLeft32((((d^a)&c)^a)+b+xf+0x49b40821, 22)

		// round 2
		a = b + bits.RotateLeft32((((b^c)&d)^c)+a+x1+0xf61e2562, 5)
		d = a + bits.RotateLeft32((((a^b)&c)^b)+d+x6+0xc040b340, 9)
		c = d + bits.RotateLeft32((((d^a)&b)^a)+c+xb+0x265e5a51, 14)
		b = c + bits.RotateLeft32((((c^d)&a)^d)+b+x0+0xe9b6c7aa, 20)
		a = b + bits.RotateLeft32((((b^c)&d)^c)+a+x5+0xd62f105d, 5)
		d = a + bits.RotateLeft32((((a^b)&c)^b)+d+xa+0x02441453, 9)
		c = d + bits.RotateLeft32((((d^a)&b)^a)+c+xf+0xd8a1e681, 14)
		b = c + bits.RotateLeft32((((c^d)&a)^d)+b+x4+0xe7d3fbc8, 20)
		a = b + bits.RotateLeft32((((b^c)&d)^c)+a+x9+0x21e1cde6, 5)
		d = a + bits.RotateLeft32((((a^b)&c)^b)+d+xe+0xc33707d6, 9)
		c = d + bits.RotateLeft32((((d^a)&b)^a)+c+x3+0xf4d50d87, 14)
		b = c + bits.RotateLeft32((((c^d)&a)^d)+b+x8+0x455a14ed, 20)
		a = b + bits.RotateLeft32((((b^c)&d)^c)+a+xd+0xa9e3e905, 5)
		d = a + bits.RotateLeft32((((a^b)&c)^b)+d+x2+0xfcefa3f8, 9)
		c = d + bits.RotateLeft32((((d^a)&b)^a)+c+x7+0x676f02d9, 14)
		b = c + bits.RotateLeft32((((c^d)&a)^d)+b+xc+0x8d2a4c8a, 20)

		// round 3
		a = b + bits.RotateLeft32((b^c^d)+a+x5+0xfffa3942, 4)
		d = a + bits.RotateLeft32((a^b^c)+d+x8+0x8771f681, 11)
		c = d + bits.RotateLeft32((d^a^b)+c+xb+0x6d9d6122, 16)
		b = c + bits.RotateLeft32((c^d^a)+b+xe+0xfde5380c, 23)
		a = b + bits.RotateLeft32((b^c^d)+a+x1+0xa4beea44, 4)
		d = a + bits.RotateLeft32((a^b^c)+d+x4+0x4bdecfa9, 11)
		c = d + bits.RotateLeft32((d^a^b)+c+x7+0xf6bb4b60, 16)
		b = c + bits.RotateLeft32((c^d^a)+b+xa+0xbebfbc70, 23)
		a = b + bits.RotateLeft32((b^c^d)+a+xd+0x289b7ec6, 4)
		d = a + bits.RotateLeft32((a^b^c)+d+x0+0xeaa127fa, 11)
		c = d + bits.RotateLeft32((d^a^b)+c+x3+0xd4ef3085, 16)
		b = c + bits.RotateLeft32((c^d^a)+b+x6+0x04881d05, 23)
		a = b + bits.RotateLeft32((b^c^d)+a+x9+0xd9d4d039, 4)
		d = a + bits.RotateLeft32((a^b^c)+d+xc+0xe6db99e5, 11)
		c = d + bits.RotateLeft32((d^a^b)+c+xf+0x1fa27cf8, 16)
		b = c + bits.RotateLeft32((c^d^a)+b+x2+0xc4ac5665, 23)

		// round 4
		a = b + bits.RotateLeft32((c^(b|^d))+a+x0+0xf4292244, 6)
		d = a + bits.RotateLeft32((b^(a|^c))+d+x7+0x432aff97, 10)
		c = d + bits.RotateLeft32((a^(d|^b))+c+xe+0xab9423a7, 15)
		b = c + bits.RotateLeft32((d^(c|^a))+b+x5+0xfc93a039, 21)
		a = b + bits.RotateLeft32((c^(b|^d))+a+xc+0x655b59c3, 6)
		d = a + bits.RotateLeft32((b^(a|^c))+d+x3+0x8f0ccc92, 10)
		c = d + bits.RotateLeft32((a^(d|^b))+c+xa+0xffeff47d, 15)
		b = c + bits.RotateLeft32((d^(c|^a))+b+x1+0x85845dd1, 21)
		a = b + bits.RotateLeft32((c^(b|^d))+a+x8+0x6fa87e4f, 6)
		d = a + bits.RotateLeft32((b^(a|^c))+d+xf+0xfe2ce6e0, 10)
		c = d + bits.RotateLeft32((a^(d|^b))+c+x6+0xa3014314, 15)
		b = c + bits.RotateLeft32((d^(c|^a))+b+xd+0x4e0811a1, 21)
		a = b + bits.RotateLeft32((c^(b|^d))+a+x4+0xf7537e82, 6)
		d = a + bits.RotateLeft32((b^(a|^c))+d+xb+0xbd3af235, 10)
		c = d + bits.RotateLeft32((a^(d|^b))+c+x2+0x2ad7d2bb, 15)
		b = c + bits.RotateLeft32((d^(c|^a))+b+x9+0xeb86d391, 21)

		// add saved state
		a += aa
		b += bb
		c += cc
		d += dd
	}

	// save state
	dig.s[0], dig.s[1], dig.s[2], dig.s[3] = a, b, c, d
}

```

// === FILE: references!/go/src/crypto/md5/md5block_386.s ===
```text
// Original source:
//	http://www.zorinaq.com/papers/md5-amd64.html
//	http://www.zorinaq.com/papers/md5-amd64.tar.bz2
//
// Translated from Perl generating GNU assembly into
// #defines generating 8a assembly, and adjusted for 386,
// by the Go Authors.

//go:build !purego

#include "textflag.h"

// MD5 optimized for AMD64.
//
// Author: Marc Bevand <bevand_m (at) epita.fr>
// Licence: I hereby disclaim the copyright on this code and place it
// in the public domain.

#define ROUND1(a, b, c, d, index, const, shift) \
	XORL	c, BP; \
	LEAL	const(a)(DI*1), a; \
	ANDL	b, BP; \
	XORL d, BP; \
	MOVL (index*4)(SI), DI; \
	ADDL BP, a; \
	ROLL $shift, a; \
	MOVL c, BP; \
	ADDL b, a

#define ROUND2(a, b, c, d, index, const, shift) \
	LEAL	const(a)(DI*1),a; \
	MOVL	d,		DI; \
	ANDL	b,		DI; \
	MOVL	d,		BP; \
	NOTL	BP; \
	ANDL	c,		BP; \
	ORL	DI,		BP; \
	MOVL	(index*4)(SI),DI; \
	ADDL	BP,		a; \
	ROLL	$shift,	a; \
	ADDL	b,		a

#define ROUND3(a, b, c, d, index, const, shift) \
	LEAL	const(a)(DI*1),a; \
	MOVL	(index*4)(SI),DI; \
	XORL	d,		BP; \
	XORL	b,		BP; \
	ADDL	BP,		a; \
	ROLL	$shift,		a; \
	MOVL	b,		BP; \
	ADDL	b,		a

#define ROUND4(a, b, c, d, index, const, shift) \
	LEAL	const(a)(DI*1),a; \
	ORL	b,		BP; \
	XORL	c,		BP; \
	ADDL	BP,		a; \
	MOVL	(index*4)(SI),DI; \
	MOVL	$0xffffffff,	BP; \
	ROLL	$shift,		a; \
	XORL	c,		BP; \
	ADDL	b,		a

TEXT	·block(SB),NOSPLIT,$24-16
	MOVL	dig+0(FP),	BP
	MOVL	p+4(FP),	SI
	MOVL	p_len+8(FP), DX
	SHRL	$6,		DX
	SHLL	$6,		DX

	LEAL	(SI)(DX*1),	DI
	MOVL	(0*4)(BP),	AX
	MOVL	(1*4)(BP),	BX
	MOVL	(2*4)(BP),	CX
	MOVL	(3*4)(BP),	DX

	CMPL	SI,		DI
	JEQ	end

	MOVL	DI,		16(SP)

loop:
	MOVL	AX,		0(SP)
	MOVL	BX,		4(SP)
	MOVL	CX,		8(SP)
	MOVL	DX,		12(SP)

	MOVL	(0*4)(SI),	DI
	MOVL	DX,		BP

	ROUND1(AX,BX,CX,DX, 1,0xd76aa478, 7);
	ROUND1(DX,AX,BX,CX, 2,0xe8c7b756,12);
	ROUND1(CX,DX,AX,BX, 3,0x242070db,17);
	ROUND1(BX,CX,DX,AX, 4,0xc1bdceee,22);
	ROUND1(AX,BX,CX,DX, 5,0xf57c0faf, 7);
	ROUND1(DX,AX,BX,CX, 6,0x4787c62a,12);
	ROUND1(CX,DX,AX,BX, 7,0xa8304613,17);
	ROUND1(BX,CX,DX,AX, 8,0xfd469501,22);
	ROUND1(AX,BX,CX,DX, 9,0x698098d8, 7);
	ROUND1(DX,AX,BX,CX,10,0x8b44f7af,12);
	ROUND1(CX,DX,AX,BX,11,0xffff5bb1,17);
	ROUND1(BX,CX,DX,AX,12,0x895cd7be,22);
	ROUND1(AX,BX,CX,DX,13,0x6b901122, 7);
	ROUND1(DX,AX,BX,CX,14,0xfd987193,12);
	ROUND1(CX,DX,AX,BX,15,0xa679438e,17);
	ROUND1(BX,CX,DX,AX, 0,0x49b40821,22);

	MOVL	(1*4)(SI),	DI
	MOVL	DX,		BP

	ROUND2(AX,BX,CX,DX, 6,0xf61e2562, 5);
	ROUND2(DX,AX,BX,CX,11,0xc040b340, 9);
	ROUND2(CX,DX,AX,BX, 0,0x265e5a51,14);
	ROUND2(BX,CX,DX,AX, 5,0xe9b6c7aa,20);
	ROUND2(AX,BX,CX,DX,10,0xd62f105d, 5);
	ROUND2(DX,AX,BX,CX,15, 0x2441453, 9);
	ROUND2(CX,DX,AX,BX, 4,0xd8a1e681,14);
	ROUND2(BX,CX,DX,AX, 9,0xe7d3fbc8,20);
	ROUND2(AX,BX,CX,DX,14,0x21e1cde6, 5);
	ROUND2(DX,AX,BX,CX, 3,0xc33707d6, 9);
	ROUND2(CX,DX,AX,BX, 8,0xf4d50d87,14);
	ROUND2(BX,CX,DX,AX,13,0x455a14ed,20);
	ROUND2(AX,BX,CX,DX, 2,0xa9e3e905, 5);
	ROUND2(DX,AX,BX,CX, 7,0xfcefa3f8, 9);
	ROUND2(CX,DX,AX,BX,12,0x676f02d9,14);
	ROUND2(BX,CX,DX,AX, 0,0x8d2a4c8a,20);

	MOVL	(5*4)(SI),	DI
	MOVL	CX,		BP

	ROUND3(AX,BX,CX,DX, 8,0xfffa3942, 4);
	ROUND3(DX,AX,BX,CX,11,0x8771f681,11);
	ROUND3(CX,DX,AX,BX,14,0x6d9d6122,16);
	ROUND3(BX,CX,DX,AX, 1,0xfde5380c,23);
	ROUND3(AX,BX,CX,DX, 4,0xa4beea44, 4);
	ROUND3(DX,AX,BX,CX, 7,0x4bdecfa9,11);
	ROUND3(CX,DX,AX,BX,10,0xf6bb4b60,16);
	ROUND3(BX,CX,DX,AX,13,0xbebfbc70,23);
	ROUND3(AX,BX,CX,DX, 0,0x289b7ec6, 4);
	ROUND3(DX,AX,BX,CX, 3,0xeaa127fa,11);
	ROUND3(CX,DX,AX,BX, 6,0xd4ef3085,16);
	ROUND3(BX,CX,DX,AX, 9, 0x4881d05,23);
	ROUND3(AX,BX,CX,DX,12,0xd9d4d039, 4);
	ROUND3(DX,AX,BX,CX,15,0xe6db99e5,11);
	ROUND3(CX,DX,AX,BX, 2,0x1fa27cf8,16);
	ROUND3(BX,CX,DX,AX, 0,0xc4ac5665,23);

	MOVL	(0*4)(SI),	DI
	MOVL	$0xffffffff,	BP
	XORL	DX,		BP

	ROUND4(AX,BX,CX,DX, 7,0xf4292244, 6);
	ROUND4(DX,AX,BX,CX,14,0x432aff97,10);
	ROUND4(CX,DX,AX,BX, 5,0xab9423a7,15);
	ROUND4(BX,CX,DX,AX,12,0xfc93a039,21);
	ROUND4(AX,BX,CX,DX, 3,0x655b59c3, 6);
	ROUND4(DX,AX,BX,CX,10,0x8f0ccc92,10);
	ROUND4(CX,DX,AX,BX, 1,0xffeff47d,15);
	ROUND4(BX,CX,DX,AX, 8,0x85845dd1,21);
	ROUND4(AX,BX,CX,DX,15,0x6fa87e4f, 6);
	ROUND4(DX,AX,BX,CX, 6,0xfe2ce6e0,10);
	ROUND4(CX,DX,AX,BX,13,0xa3014314,15);
	ROUND4(BX,CX,DX,AX, 4,0x4e0811a1,21);
	ROUND4(AX,BX,CX,DX,11,0xf7537e82, 6);
	ROUND4(DX,AX,BX,CX, 2,0xbd3af235,10);
	ROUND4(CX,DX,AX,BX, 9,0x2ad7d2bb,15);
	ROUND4(BX,CX,DX,AX, 0,0xeb86d391,21);

	ADDL	0(SP),	AX
	ADDL	4(SP),	BX
	ADDL	8(SP),	CX
	ADDL	12(SP),	DX

	ADDL	$64,		SI
	CMPL	SI,		16(SP)
	JB	loop

end:
	MOVL	dig+0(FP),	BP
	MOVL	AX,		(0*4)(BP)
	MOVL	BX,		(1*4)(BP)
	MOVL	CX,		(2*4)(BP)
	MOVL	DX,		(3*4)(BP)
	RET

```

// === FILE: references!/go/src/crypto/md5/md5block_amd64.s ===
```text
// Code generated by command: go run md5block_amd64_asm.go -out ../md5block_amd64.s -pkg md5. DO NOT EDIT.

//go:build !purego

#include "textflag.h"

// func block(dig *digest, p []byte)
TEXT ·block(SB), NOSPLIT, $8-32
	MOVQ dig+0(FP), BP
	MOVQ p_base+8(FP), SI
	MOVQ p_len+16(FP), DX
	SHRQ $0x06, DX
	SHLQ $0x06, DX
	LEAQ (SI)(DX*1), DI
	MOVL (BP), AX
	MOVL 4(BP), BX
	MOVL 8(BP), CX
	MOVL 12(BP), DX
	MOVL $0xffffffff, R11
	CMPQ SI, DI
	JEQ  end

loop:
	MOVL AX, R12
	MOVL BX, R13
	MOVL CX, R14
	MOVL DX, R15
	MOVL (SI), R8
	MOVL DX, R9
	XORL CX, R9
	ADDL $0xd76aa478, AX
	ADDL R8, AX
	ANDL BX, R9
	XORL DX, R9
	MOVL 4(SI), R8
	ADDL R9, AX
	ROLL $0x07, AX
	MOVL CX, R9
	ADDL BX, AX
	XORL BX, R9
	ADDL $0xe8c7b756, DX
	ADDL R8, DX
	ANDL AX, R9
	XORL CX, R9
	MOVL 8(SI), R8
	ADDL R9, DX
	ROLL $0x0c, DX
	MOVL BX, R9
	ADDL AX, DX
	XORL AX, R9
	ADDL $0x242070db, CX
	ADDL R8, CX
	ANDL DX, R9
	XORL BX, R9
	MOVL 12(SI), R8
	ADDL R9, CX
	ROLL $0x11, CX
	MOVL AX, R9
	ADDL DX, CX
	XORL DX, R9
	ADDL $0xc1bdceee, BX
	ADDL R8, BX
	ANDL CX, R9
	XORL AX, R9
	MOVL 16(SI), R8
	ADDL R9, BX
	ROLL $0x16, BX
	MOVL DX, R9
	ADDL CX, BX
	XORL CX, R9
	ADDL $0xf57c0faf, AX
	ADDL R8, AX
	ANDL BX, R9
	XORL DX, R9
	MOVL 20(SI), R8
	ADDL R9, AX
	ROLL $0x07, AX
	MOVL CX, R9
	ADDL BX, AX
	XORL BX, R9
	ADDL $0x4787c62a, DX
	ADDL R8, DX
	ANDL AX, R9
	XORL CX, R9
	MOVL 24(SI), R8
	ADDL R9, DX
	ROLL $0x0c, DX
	MOVL BX, R9
	ADDL AX, DX
	XORL AX, R9
	ADDL $0xa8304613, CX
	ADDL R8, CX
	ANDL DX, R9
	XORL BX, R9
	MOVL 28(SI), R8
	ADDL R9, CX
	ROLL $0x11, CX
	MOVL AX, R9
	ADDL DX, CX
	XORL DX, R9
	ADDL $0xfd469501, BX
	ADDL R8, BX
	ANDL CX, R9
	XORL AX, R9
	MOVL 32(SI), R8
	ADDL R9, BX
	ROLL $0x16, BX
	MOVL DX, R9
	ADDL CX, BX
	XORL CX, R9
	ADDL $0x698098d8, AX
	ADDL R8, AX
	ANDL BX, R9
	XORL DX, R9
	MOVL 36(SI), R8
	ADDL R9, AX
	ROLL $0x07, AX
	MOVL CX, R9
	ADDL BX, AX
	XORL BX, R9
	ADDL $0x8b44f7af, DX
	ADDL R8, DX
	ANDL AX, R9
	XORL CX, R9
	MOVL 40(SI), R8
	ADDL R9, DX
	ROLL $0x0c, DX
	MOVL BX, R9
	ADDL AX, DX
	XORL AX, R9
	ADDL $0xffff5bb1, CX
	ADDL R8, CX
	ANDL DX, R9
	XORL BX, R9
	MOVL 44(SI), R8
	ADDL R9, CX
	ROLL $0x11, CX
	MOVL AX, R9
	ADDL DX, CX
	XORL DX, R9
	ADDL $0x895cd7be, BX
	ADDL R8, BX
	ANDL CX, R9
	XORL AX, R9
	MOVL 48(SI), R8
	ADDL R9, BX
	ROLL $0x16, BX
	MOVL DX, R9
	ADDL CX, BX
	XORL CX, R9
	ADDL $0x6b901122, AX
	ADDL R8, AX
	ANDL BX, R9
	XORL DX, R9
	MOVL 52(SI), R8
	ADDL R9, AX
	ROLL $0x07, AX
	MOVL CX, R9
	ADDL BX, AX
	XORL BX, R9
	ADDL $0xfd987193, DX
	ADDL R8, DX
	ANDL AX, R9
	XORL CX, R9
	MOVL 56(SI), R8
	ADDL R9, DX
	ROLL $0x0c, DX
	MOVL BX, R9
	ADDL AX, DX
	XORL AX, R9
	ADDL $0xa679438e, CX
	ADDL R8, CX
	ANDL DX, R9
	XORL BX, R9
	MOVL 60(SI), R8
	ADDL R9, CX
	ROLL $0x11, CX
	MOVL AX, R9
	ADDL DX, CX
	XORL DX, R9
	ADDL $0x49b40821, BX
	ADDL R8, BX
	ANDL CX, R9
	XORL AX, R9
	MOVL 4(SI), R8
	ADDL R9, BX
	ROLL $0x16, BX
	MOVL DX, R9
	ADDL CX, BX
	MOVL DX, R9
	MOVL DX, R10
	XORL R11, R9
	ADDL $0xf61e2562, AX
	ADDL R8, AX
	ANDL BX, R10
	ANDL CX, R9
	MOVL 24(SI), R8
	ADDL R9, AX
	ADDL R10, AX
	MOVL CX, R9
	MOVL CX, R10
	ROLL $0x05, AX
	ADDL BX, AX
	XORL R11, R9
	ADDL $0xc040b340, DX
	ADDL R8, DX
	ANDL AX, R10
	ANDL BX, R9
	MOVL 44(SI), R8
	ADDL R9, DX
	ADDL R10, DX
	MOVL BX, R9
	MOVL BX, R10
	ROLL $0x09, DX
	ADDL AX, DX
	XORL R11, R9
	ADDL $0x265e5a51, CX
	ADDL R8, CX
	ANDL DX, R10
	ANDL AX, R9
	MOVL (SI), R8
	ADDL R9, CX
	ADDL R10, CX
	MOVL AX, R9
	MOVL AX, R10
	ROLL $0x0e, CX
	ADDL DX, CX
	XORL R11, R9
	ADDL $0xe9b6c7aa, BX
	ADDL R8, BX
	ANDL CX, R10
	ANDL DX, R9
	MOVL 20(SI), R8
	ADDL R9, BX
	ADDL R10, BX
	MOVL DX, R9
	MOVL DX, R10
	ROLL $0x14, BX
	ADDL CX, BX
	XORL R11, R9
	ADDL $0xd62f105d, AX
	ADDL R8, AX
	ANDL BX, R10
	ANDL CX, R9
	MOVL 40(SI), R8
	ADDL R9, AX
	ADDL R10, AX
	MOVL CX, R9
	MOVL CX, R10
	ROLL $0x05, AX
	ADDL BX, AX
	XORL R11, R9
	ADDL $0x02441453, DX
	ADDL R8, DX
	ANDL AX, R10
	ANDL BX, R9
	MOVL 60(SI), R8
	ADDL R9, DX
	ADDL R10, DX
	MOVL BX, R9
	MOVL BX, R10
	ROLL $0x09, DX
	ADDL AX, DX
	XORL R11, R9
	ADDL $0xd8a1e681, CX
	ADDL R8, CX
	ANDL DX, R10
	ANDL AX, R9
	MOVL 16(SI), R8
	ADDL R9, CX
	ADDL R10, CX
	MOVL AX, R9
	MOVL AX, R10
	ROLL $0x0e, CX
	ADDL DX, CX
	XORL R11, R9
	ADDL $0xe7d3fbc8, BX
	ADDL R8, BX
	ANDL CX, R10
	ANDL DX, R9
	MOVL 36(SI), R8
	ADDL R9, BX
	ADDL R10, BX
	MOVL DX, R9
	MOVL DX, R10
	ROLL $0x14, BX
	ADDL CX, BX
	XORL R11, R9
	ADDL $0x21e1cde6, AX
	ADDL R8, AX
	ANDL BX, R10
	ANDL CX, R9
	MOVL 56(SI), R8
	ADDL R9, AX
	ADDL R10, AX
	MOVL CX, R9
	MOVL CX, R10
	ROLL $0x05, AX
	ADDL BX, AX
	XORL R11, R9
	ADDL $0xc33707d6, DX
	ADDL R8, DX
	ANDL AX, R10
	ANDL BX, R9
	MOVL 12(SI), R8
	ADDL R9, DX
	ADDL R10, DX
	MOVL BX, R9
	MOVL BX, R10
	ROLL $0x09, DX
	ADDL AX, DX
	XORL R11, R9
	ADDL $0xf4d50d87, CX
	ADDL R8, CX
	ANDL DX, R10
	ANDL AX, R9
	MOVL 32(SI), R8
	ADDL R9, CX
	ADDL R10, CX
	MOVL AX, R9
	MOVL AX, R10
	ROLL $0x0e, CX
	ADDL DX, CX
	XORL R11, R9
	ADDL $0x455a14ed, BX
	ADDL R8, BX
	ANDL CX, R10
	ANDL DX, R9
	MOVL 52(SI), R8
	ADDL R9, BX
	ADDL R10, BX
	MOVL DX, R9
	MOVL DX, R10
	ROLL $0x14, BX
	ADDL CX, BX
	XORL R11, R9
	ADDL $0xa9e3e905, AX
	ADDL R8, AX
	ANDL BX, R10
	ANDL CX, R9
	MOVL 8(SI), R8
	ADDL R9, AX
	ADDL R10, AX
	MOVL CX, R9
	MOVL CX, R10
	ROLL $0x05, AX
	ADDL BX, AX
	XORL R11, R9
	ADDL $0xfcefa3f8, DX
	ADDL R8, DX
	ANDL AX, R10
	ANDL BX, R9
	MOVL 28(SI), R8
	ADDL R9, DX
	ADDL R10, DX
	MOVL BX, R9
	MOVL BX, R10
	ROLL $0x09, DX
	ADDL AX, DX
	XORL R11, R9
	ADDL $0x676f02d9, CX
	ADDL R8, CX
	ANDL DX, R10
	ANDL AX, R9
	MOVL 48(SI), R8
	ADDL R9, CX
	ADDL R10, CX
	MOVL AX, R9
	MOVL AX, R10
	ROLL $0x0e, CX
	ADDL DX, CX
	XORL R11, R9
	ADDL $0x8d2a4c8a, BX
	ADDL R8, BX
	ANDL CX, R10
	ANDL DX, R9
	MOVL 20(SI), R8
	ADDL R9, BX
	ADDL R10, BX
	MOVL DX, R9
	MOVL DX, R10
	ROLL $0x14, BX
	ADDL CX, BX
	MOVL CX, R9
	MOVL DX, R9
	XORL CX, R9
	XORL BX, R9
	ADDL $0xfffa3942, AX
	ADDL R8, AX
	MOVL 32(SI), R8
	ADDL R9, AX
	ROLL $0x04, AX
	ADDL BX, AX
	XORL DX, R9
	XORL AX, R9
	ADDL $0x8771f681, DX
	ADDL R8, DX
	MOVL 44(SI), R8
	ADDL R9, DX
	ROLL $0x0b, DX
	ADDL AX, DX
	XORL CX, R9
	XORL DX, R9
	ADDL $0x6d9d6122, CX
	ADDL R8, CX
	MOVL 56(SI), R8
	ADDL R9, CX
	ROLL $0x10, CX
	ADDL DX, CX
	XORL BX, R9
	XORL CX, R9
	ADDL $0xfde5380c, BX
	ADDL R8, BX
	MOVL 4(SI), R8
	ADDL R9, BX
	ROLL $0x17, BX
	ADDL CX, BX
	XORL AX, R9
	XORL BX, R9
	ADDL $0xa4beea44, AX
	ADDL R8, AX
	MOVL 16(SI), R8
	ADDL R9, AX
	ROLL $0x04, AX
	ADDL BX, AX
	XORL DX, R9
	XORL AX, R9
	ADDL $0x4bdecfa9, DX
	ADDL R8, DX
	MOVL 28(SI), R8
	ADDL R9, DX
	ROLL $0x0b, DX
	ADDL AX, DX
	XORL CX, R9
	XORL DX, R9
	ADDL $0xf6bb4b60, CX
	ADDL R8, CX
	MOVL 40(SI), R8
	ADDL R9, CX
	ROLL $0x10, CX
	ADDL DX, CX
	XORL BX, R9
	XORL CX, R9
	ADDL $0xbebfbc70, BX
	ADDL R8, BX
	MOVL 52(SI), R8
	ADDL R9, BX
	ROLL $0x17, BX
	ADDL CX, BX
	XORL AX, R9
	XORL BX, R9
	ADDL $0x289b7ec6, AX
	ADDL R8, AX
	MOVL (SI), R8
	ADDL R9, AX
	ROLL $0x04, AX
	ADDL BX, AX
	XORL DX, R9
	XORL AX, R9
	ADDL $0xeaa127fa, DX
	ADDL R8, DX
	MOVL 12(SI), R8
	ADDL R9, DX
	ROLL $0x0b, DX
	ADDL AX, DX
	XORL CX, R9
	XORL DX, R9
	ADDL $0xd4ef3085, CX
	ADDL R8, CX
	MOVL 24(SI), R8
	ADDL R9, CX
	ROLL $0x10, CX
	ADDL DX, CX
	XORL BX, R9
	XORL CX, R9
	ADDL $0x04881d05, BX
	ADDL R8, BX
	MOVL 36(SI), R8
	ADDL R9, BX
	ROLL $0x17, BX
	ADDL CX, BX
	XORL AX, R9
	XORL BX, R9
	ADDL $0xd9d4d039, AX
	ADDL R8, AX
	MOVL 48(SI), R8
	ADDL R9, AX
	ROLL $0x04, AX
	ADDL BX, AX
	XORL DX, R9
	XORL AX, R9
	ADDL $0xe6db99e5, DX
	ADDL R8, DX
	MOVL 60(SI), R8
	ADDL R9, DX
	ROLL $0x0b, DX
	ADDL AX, DX
	XORL CX, R9
	XORL DX, R9
	ADDL $0x1fa27cf8, CX
	ADDL R8, CX
	MOVL 8(SI), R8
	ADDL R9, CX
	ROLL $0x10, CX
	ADDL DX, CX
	XORL BX, R9
	XORL CX, R9
	ADDL $0xc4ac5665, BX
	ADDL R8, BX
	MOVL (SI), R8
	ADDL R9, BX
	ROLL $0x17, BX
	ADDL CX, BX
	MOVL R11, R9
	XORL DX, R9
	ADDL $0xf4292244, AX
	ADDL R8, AX
	ORL  BX, R9
	XORL CX, R9
	ADDL R9, AX
	MOVL 28(SI), R8
	MOVL $0xffffffff, R9
	ROLL $0x06, AX
	XORL CX, R9
	ADDL BX, AX
	ADDL $0x432aff97, DX
	ADDL R8, DX
	ORL  AX, R9
	XORL BX, R9
	ADDL R9, DX
	MOVL 56(SI), R8
	MOVL $0xffffffff, R9
	ROLL $0x0a, DX
	XORL BX, R9
	ADDL AX, DX
	ADDL $0xab9423a7, CX
	ADDL R8, CX
	ORL  DX, R9
	XORL AX, R9
	ADDL R9, CX
	MOVL 20(SI), R8
	MOVL $0xffffffff, R9
	ROLL $0x0f, CX
	XORL AX, R9
	ADDL DX, CX
	ADDL $0xfc93a039, BX
	ADDL R8, BX
	ORL  CX, R9
	XORL DX, R9
	ADDL R9, BX
	MOVL 48(SI), R8
	MOVL $0xffffffff, R9
	ROLL $0x15, BX
	XORL DX, R9
	ADDL CX, BX
	ADDL $0x655b59c3, AX
	ADDL R8, AX
	ORL  BX, R9
	XORL CX, R9
	ADDL R9, AX
	MOVL 12(SI), R8
	MOVL $0xffffffff, R9
	ROLL $0x06, AX
	XORL CX, R9
	ADDL BX, AX
	ADDL $0x8f0ccc92, DX
	ADDL R8, DX
	ORL  AX, R9
	XORL BX, R9
	ADDL R9, DX
	MOVL 40(SI), R8
	MOVL $0xffffffff, R9
	ROLL $0x0a, DX
	XORL BX, R9
	ADDL AX, DX
	ADDL $0xffeff47d, CX
	ADDL R8, CX
	ORL  DX, R9
	XORL AX, R9
	ADDL R9, CX
	MOVL 4(SI), R8
	MOVL $0xffffffff, R9
	ROLL $0x0f, CX
	XORL AX, R9
	ADDL DX, CX
	ADDL $0x85845dd1, BX
	ADDL R8, BX
	ORL  CX, R9
	XORL DX, R9
	ADDL R9, BX
	MOVL 32(SI), R8
	MOVL $0xffffffff, R9
	ROLL $0x15, BX
	XORL DX, R9
	ADDL CX, BX
	ADDL $0x6fa87e4f, AX
	ADDL R8, AX
	ORL  BX, R9
	XORL CX, R9
	ADDL R9, AX
	MOVL 60(SI), R8
	MOVL $0xffffffff, R9
	ROLL $0x06, AX
	XORL CX, R9
	ADDL BX, AX
	ADDL $0xfe2ce6e0, DX
	ADDL R8, DX
	ORL  AX, R9
	XORL BX, R9
	ADDL R9, DX
	MOVL 24(SI), R8
	MOVL $0xffffffff, R9
	ROLL $0x0a, DX
	XORL BX, R9
	ADDL AX, DX
	ADDL $0xa3014314, CX
	ADDL R8, CX
	ORL  DX, R9
	XORL AX, R9
	ADDL R9, CX
	MOVL 52(SI), R8
	MOVL $0xffffffff, R9
	ROLL $0x0f, CX
	XORL AX, R9
	ADDL DX, CX
	ADDL $0x4e0811a1, BX
	ADDL R8, BX
	ORL  CX, R9
	XORL DX, R9
	ADDL R9, BX
	MOVL 16(SI), R8
	MOVL $0xffffffff, R9
	ROLL $0x15, BX
	XORL DX, R9
	ADDL CX, BX
	ADDL $0xf7537e82, AX
	ADDL R8, AX
	ORL  BX, R9
	XORL CX, R9
	ADDL R9, AX
	MOVL 44(SI), R8
	MOVL $0xffffffff, R9
	ROLL $0x06, AX
	XORL CX, R9
	ADDL BX, AX
	ADDL $0xbd3af235, DX
	ADDL R8, DX
	ORL  AX, R9
	XORL BX, R9
	ADDL R9, DX
	MOVL 8(SI), R8
	MOVL $0xffffffff, R9
	ROLL $0x0a, DX
	XORL BX, R9
	ADDL AX, DX
	ADDL $0x2ad7d2bb, CX
	ADDL R8, CX
	ORL  DX, R9
	XORL AX, R9
	ADDL R9, CX
	MOVL 36(SI), R8
	MOVL $0xffffffff, R9
	ROLL $0x0f, CX
	XORL AX, R9
	ADDL DX, CX
	ADDL $0xeb86d391, BX
	ADDL R8, BX
	ORL  CX, R9
	XORL DX, R9
	ADDL R9, BX
	MOVL (SI), R8
	MOVL $0xffffffff, R9
	ROLL $0x15, BX
	XORL DX, R9
	ADDL CX, BX
	ADDL R12, AX
	ADDL R13, BX
	ADDL R14, CX
	ADDL R15, DX
	ADDQ $0x40, SI
	CMPQ SI, DI
	JB   loop

end:
	MOVL AX, (BP)
	MOVL BX, 4(BP)
	MOVL CX, 8(BP)
	MOVL DX, 12(BP)
	RET

```

// === FILE: references!/go/src/crypto/md5/md5block_arm.s ===
```text
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// ARM version of md5block.go

//go:build !purego

#include "textflag.h"

// Register definitions
#define Rtable	R0	// Pointer to MD5 constants table
#define Rdata	R1	// Pointer to data to hash
#define Ra	R2	// MD5 accumulator
#define Rb	R3	// MD5 accumulator
#define Rc	R4	// MD5 accumulator
#define Rd	R5	// MD5 accumulator
#define Rc0	R6	// MD5 constant
#define Rc1	R7	// MD5 constant
#define Rc2	R8	// MD5 constant
// r9, r10 are forbidden
// r11 is OK provided you check the assembler that no synthetic instructions use it
#define Rc3	R11	// MD5 constant
#define Rt0	R12	// temporary
#define Rt1	R14	// temporary

// func block(dig *digest, p []byte)
// 0(FP) is *digest
// 4(FP) is p.array (struct Slice)
// 8(FP) is p.len
//12(FP) is p.cap
//
// Stack frame
#define p_end	end-4(SP)	// pointer to the end of data
#define p_data	data-8(SP)	// current data pointer
#define buf	buffer-(8+4*16)(SP)	//16 words temporary buffer
		// 3 words at 4..12(R13) for called routine parameters

TEXT	·block(SB), NOSPLIT, $84-16
	MOVW	p+4(FP), Rdata	// pointer to the data
	MOVW	p_len+8(FP), Rt0	// number of bytes
	ADD	Rdata, Rt0
	MOVW	Rt0, p_end	// pointer to end of data

loop:
	MOVW	Rdata, p_data	// Save Rdata
	AND.S	$3, Rdata, Rt0	// TST $3, Rdata not working see issue 5921
	BEQ	aligned			// aligned detected - skip copy

	// Copy the unaligned source data into the aligned temporary buffer
	// memmove(to=4(R13), from=8(R13), n=12(R13)) - Corrupts all registers
	MOVW	$buf, Rtable	// to
	MOVW	$64, Rc0		// n
	MOVM.IB	[Rtable,Rdata,Rc0], (R13)
	BL	runtime·memmove(SB)

	// Point to the local aligned copy of the data
	MOVW	$buf, Rdata

aligned:
	// Point to the table of constants
	// A PC relative add would be cheaper than this
	MOVW	$·table(SB), Rtable

	// Load up initial MD5 accumulator
	MOVW	dig+0(FP), Rc0
	MOVM.IA (Rc0), [Ra,Rb,Rc,Rd]

// a += (((c^d)&b)^d) + X[index] + const
// a = a<<shift | a>>(32-shift) + b
#define ROUND1(Ra, Rb, Rc, Rd, index, shift, Rconst) \
	EOR	Rc, Rd, Rt0		; \
	AND	Rb, Rt0			; \
	EOR	Rd, Rt0			; \
	MOVW	(index<<2)(Rdata), Rt1	; \
	ADD	Rt1, Rt0			; \
	ADD	Rconst, Rt0			; \
	ADD	Rt0, Ra			; \
	ADD	Ra@>(32-shift), Rb, Ra	;

	MOVM.IA.W (Rtable), [Rc0,Rc1,Rc2,Rc3]
	ROUND1(Ra, Rb, Rc, Rd,  0,	7, Rc0)
	ROUND1(Rd, Ra, Rb, Rc,  1, 12, Rc1)
	ROUND1(Rc, Rd, Ra, Rb,  2, 17, Rc2)
	ROUND1(Rb, Rc, Rd, Ra,  3, 22, Rc3)

	MOVM.IA.W (Rtable), [Rc0,Rc1,Rc2,Rc3]
	ROUND1(Ra, Rb, Rc, Rd,  4,	7, Rc0)
	ROUND1(Rd, Ra, Rb, Rc,  5, 12, Rc1)
	ROUND1(Rc, Rd, Ra, Rb,  6, 17, Rc2)
	ROUND1(Rb, Rc, Rd, Ra,  7, 22, Rc3)

	MOVM.IA.W (Rtable), [Rc0,Rc1,Rc2,Rc3]
	ROUND1(Ra, Rb, Rc, Rd,  8,	7, Rc0)
	ROUND1(Rd, Ra, Rb, Rc,  9, 12, Rc1)
	ROUND1(Rc, Rd, Ra, Rb, 10, 17, Rc2)
	ROUND1(Rb, Rc, Rd, Ra, 11, 22, Rc3)

	MOVM.IA.W (Rtable), [Rc0,Rc1,Rc2,Rc3]
	ROUND1(Ra, Rb, Rc, Rd, 12,	7, Rc0)
	ROUND1(Rd, Ra, Rb, Rc, 13, 12, Rc1)
	ROUND1(Rc, Rd, Ra, Rb, 14, 17, Rc2)
	ROUND1(Rb, Rc, Rd, Ra, 15, 22, Rc3)

// a += (((b^c)&d)^c) + X[index] + const
// a = a<<shift | a>>(32-shift) + b
#define ROUND2(Ra, Rb, Rc, Rd, index, shift, Rconst) \
	EOR	Rb, Rc, Rt0		; \
	AND	Rd, Rt0			; \
	EOR	Rc, Rt0			; \
	MOVW	(index<<2)(Rdata), Rt1	; \
	ADD	Rt1, Rt0			; \
	ADD	Rconst, Rt0			; \
	ADD	Rt0, Ra			; \
	ADD	Ra@>(32-shift), Rb, Ra	;

	MOVM.IA.W (Rtable), [Rc0,Rc1,Rc2,Rc3]
	ROUND2(Ra, Rb, Rc, Rd,  1,	5, Rc0)
	ROUND2(Rd, Ra, Rb, Rc,  6,	9, Rc1)
	ROUND2(Rc, Rd, Ra, Rb, 11, 14, Rc2)
	ROUND2(Rb, Rc, Rd, Ra,  0, 20, Rc3)

	MOVM.IA.W (Rtable), [Rc0,Rc1,Rc2,Rc3]
	ROUND2(Ra, Rb, Rc, Rd,  5,	5, Rc0)
	ROUND2(Rd, Ra, Rb, Rc, 10,	9, Rc1)
	ROUND2(Rc, Rd, Ra, Rb, 15, 14, Rc2)
	ROUND2(Rb, Rc, Rd, Ra,  4, 20, Rc3)

	MOVM.IA.W (Rtable), [Rc0,Rc1,Rc2,Rc3]
	ROUND2(Ra, Rb, Rc, Rd,  9,	5, Rc0)
	ROUND2(Rd, Ra, Rb, Rc, 14,	9, Rc1)
	ROUND2(Rc, Rd, Ra, Rb,  3, 14, Rc2)
	ROUND2(Rb, Rc, Rd, Ra,  8, 20, Rc3)

	MOVM.IA.W (Rtable), [Rc0,Rc1,Rc2,Rc3]
	ROUND2(Ra, Rb, Rc, Rd, 13,	5, Rc0)
	ROUND2(Rd, Ra, Rb, Rc,  2,	9, Rc1)
	ROUND2(Rc, Rd, Ra, Rb,  7, 14, Rc2)
	ROUND2(Rb, Rc, Rd, Ra, 12, 20, Rc3)

// a += (b^c^d) + X[index] + const
// a = a<<shift | a>>(32-shift) + b
#define ROUND3(Ra, Rb, Rc, Rd, index, shift, Rconst) \
	EOR	Rb, Rc, Rt0		; \
	EOR	Rd, Rt0			; \
	MOVW	(index<<2)(Rdata), Rt1	; \
	ADD	Rt1, Rt0			; \
	ADD	Rconst, Rt0			; \
	ADD	Rt0, Ra			; \
	ADD	Ra@>(32-shift), Rb, Ra	;

	MOVM.IA.W (Rtable), [Rc0,Rc1,Rc2,Rc3]
	ROUND3(Ra, Rb, Rc, Rd,  5,	4, Rc0)
	ROUND3(Rd, Ra, Rb, Rc,  8, 11, Rc1)
	ROUND3(Rc, Rd, Ra, Rb, 11, 16, Rc2)
	ROUND3(Rb, Rc, Rd, Ra, 14, 23, Rc3)

	MOVM.IA.W (Rtable), [Rc0,Rc1,Rc2,Rc3]
	ROUND3(Ra, Rb, Rc, Rd,  1,	4, Rc0)
	ROUND3(Rd, Ra, Rb, Rc,  4, 11, Rc1)
	ROUND3(Rc, Rd, Ra, Rb,  7, 16, Rc2)
	ROUND3(Rb, Rc, Rd, Ra, 10, 23, Rc3)

	MOVM.IA.W (Rtable), [Rc0,Rc1,Rc2,Rc3]
	ROUND3(Ra, Rb, Rc, Rd, 13,	4, Rc0)
	ROUND3(Rd, Ra, Rb, Rc,  0, 11, Rc1)
	ROUND3(Rc, Rd, Ra, Rb,  3, 16, Rc2)
	ROUND3(Rb, Rc, Rd, Ra,  6, 23, Rc3)

	MOVM.IA.W (Rtable), [Rc0,Rc1,Rc2,Rc3]
	ROUND3(Ra, Rb, Rc, Rd,  9,	4, Rc0)
	ROUND3(Rd, Ra, Rb, Rc, 12, 11, Rc1)
	ROUND3(Rc, Rd, Ra, Rb, 15, 16, Rc2)
	ROUND3(Rb, Rc, Rd, Ra,  2, 23, Rc3)

// a += (c^(b|^d)) + X[index] + const
// a = a<<shift | a>>(32-shift) + b
#define ROUND4(Ra, Rb, Rc, Rd, index, shift, Rconst) \
	MVN	Rd, Rt0			; \
	ORR	Rb, Rt0			; \
	EOR	Rc, Rt0			; \
	MOVW	(index<<2)(Rdata), Rt1	; \
	ADD	Rt1, Rt0			; \
	ADD	Rconst, Rt0			; \
	ADD	Rt0, Ra			; \
	ADD	Ra@>(32-shift), Rb, Ra	;

	MOVM.IA.W (Rtable), [Rc0,Rc1,Rc2,Rc3]
	ROUND4(Ra, Rb, Rc, Rd,  0,	6, Rc0)
	ROUND4(Rd, Ra, Rb, Rc,  7, 10, Rc1)
	ROUND4(Rc, Rd, Ra, Rb, 14, 15, Rc2)
	ROUND4(Rb, Rc, Rd, Ra,  5, 21, Rc3)

	MOVM.IA.W (Rtable), [Rc0,Rc1,Rc2,Rc3]
	ROUND4(Ra, Rb, Rc, Rd, 12,	6, Rc0)
	ROUND4(Rd, Ra, Rb, Rc,  3, 10, Rc1)
	ROUND4(Rc, Rd, Ra, Rb, 10, 15, Rc2)
	ROUND4(Rb, Rc, Rd, Ra,  1, 21, Rc3)

	MOVM.IA.W (Rtable), [Rc0,Rc1,Rc2,Rc3]
	ROUND4(Ra, Rb, Rc, Rd,  8,	6, Rc0)
	ROUND4(Rd, Ra, Rb, Rc, 15, 10, Rc1)
	ROUND4(Rc, Rd, Ra, Rb,  6, 15, Rc2)
	ROUND4(Rb, Rc, Rd, Ra, 13, 21, Rc3)

	MOVM.IA.W (Rtable), [Rc0,Rc1,Rc2,Rc3]
	ROUND4(Ra, Rb, Rc, Rd,  4,	6, Rc0)
	ROUND4(Rd, Ra, Rb, Rc, 11, 10, Rc1)
	ROUND4(Rc, Rd, Ra, Rb,  2, 15, Rc2)
	ROUND4(Rb, Rc, Rd, Ra,  9, 21, Rc3)

	MOVW	dig+0(FP), Rt0
	MOVM.IA (Rt0), [Rc0,Rc1,Rc2,Rc3]

	ADD	Rc0, Ra
	ADD	Rc1, Rb
	ADD	Rc2, Rc
	ADD	Rc3, Rd

	MOVM.IA [Ra,Rb,Rc,Rd], (Rt0)

	MOVW	p_data, Rdata
	MOVW	p_end, Rt0
	ADD	$64, Rdata
	CMP	Rt0, Rdata
	BLO	loop

	RET

// MD5 constants table

	// Round 1
	DATA	·table+0x00(SB)/4, $0xd76aa478
	DATA	·table+0x04(SB)/4, $0xe8c7b756
	DATA	·table+0x08(SB)/4, $0x242070db
	DATA	·table+0x0c(SB)/4, $0xc1bdceee
	DATA	·table+0x10(SB)/4, $0xf57c0faf
	DATA	·table+0x14(SB)/4, $0x4787c62a
	DATA	·table+0x18(SB)/4, $0xa8304613
	DATA	·table+0x1c(SB)/4, $0xfd469501
	DATA	·table+0x20(SB)/4, $0x698098d8
	DATA	·table+0x24(SB)/4, $0x8b44f7af
	DATA	·table+0x28(SB)/4, $0xffff5bb1
	DATA	·table+0x2c(SB)/4, $0x895cd7be
	DATA	·table+0x30(SB)/4, $0x6b901122
	DATA	·table+0x34(SB)/4, $0xfd987193
	DATA	·table+0x38(SB)/4, $0xa679438e
	DATA	·table+0x3c(SB)/4, $0x49b40821
	// Round 2
	DATA	·table+0x40(SB)/4, $0xf61e2562
	DATA	·table+0x44(SB)/4, $0xc040b340
	DATA	·table+0x48(SB)/4, $0x265e5a51
	DATA	·table+0x4c(SB)/4, $0xe9b6c7aa
	DATA	·table+0x50(SB)/4, $0xd62f105d
	DATA	·table+0x54(SB)/4, $0x02441453
	DATA	·table+0x58(SB)/4, $0xd8a1e681
	DATA	·table+0x5c(SB)/4, $0xe7d3fbc8
	DATA	·table+0x60(SB)/4, $0x21e1cde6
	DATA	·table+0x64(SB)/4, $0xc33707d6
	DATA	·table+0x68(SB)/4, $0xf4d50d87
	DATA	·table+0x6c(SB)/4, $0x455a14ed
	DATA	·table+0x70(SB)/4, $0xa9e3e905
	DATA	·table+0x74(SB)/4, $0xfcefa3f8
	DATA	·table+0x78(SB)/4, $0x676f02d9
	DATA	·table+0x7c(SB)/4, $0x8d2a4c8a
	// Round 3
	DATA	·table+0x80(SB)/4, $0xfffa3942
	DATA	·table+0x84(SB)/4, $0x8771f681
	DATA	·table+0x88(SB)/4, $0x6d9d6122
	DATA	·table+0x8c(SB)/4, $0xfde5380c
	DATA	·table+0x90(SB)/4, $0xa4beea44
	DATA	·table+0x94(SB)/4, $0x4bdecfa9
	DATA	·table+0x98(SB)/4, $0xf6bb4b60
	DATA	·table+0x9c(SB)/4, $0xbebfbc70
	DATA	·table+0xa0(SB)/4, $0x289b7ec6
	DATA	·table+0xa4(SB)/4, $0xeaa127fa
	DATA	·table+0xa8(SB)/4, $0xd4ef3085
	DATA	·table+0xac(SB)/4, $0x04881d05
	DATA	·table+0xb0(SB)/4, $0xd9d4d039
	DATA	·table+0xb4(SB)/4, $0xe6db99e5
	DATA	·table+0xb8(SB)/4, $0x1fa27cf8
	DATA	·table+0xbc(SB)/4, $0xc4ac5665
	// Round 4
	DATA	·table+0xc0(SB)/4, $0xf4292244
	DATA	·table+0xc4(SB)/4, $0x432aff97
	DATA	·table+0xc8(SB)/4, $0xab9423a7
	DATA	·table+0xcc(SB)/4, $0xfc93a039
	DATA	·table+0xd0(SB)/4, $0x655b59c3
	DATA	·table+0xd4(SB)/4, $0x8f0ccc92
	DATA	·table+0xd8(SB)/4, $0xffeff47d
	DATA	·table+0xdc(SB)/4, $0x85845dd1
	DATA	·table+0xe0(SB)/4, $0x6fa87e4f
	DATA	·table+0xe4(SB)/4, $0xfe2ce6e0
	DATA	·table+0xe8(SB)/4, $0xa3014314
	DATA	·table+0xec(SB)/4, $0x4e0811a1
	DATA	·table+0xf0(SB)/4, $0xf7537e82
	DATA	·table+0xf4(SB)/4, $0xbd3af235
	DATA	·table+0xf8(SB)/4, $0x2ad7d2bb
	DATA	·table+0xfc(SB)/4, $0xeb86d391
	// Global definition
	GLOBL	·table(SB),8,$256

```

// === FILE: references!/go/src/crypto/md5/md5block_arm64.s ===
```text
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// ARM64 version of md5block.go
// derived from crypto/md5/md5block_amd64.s

//go:build !purego

#include "textflag.h"

TEXT	·block(SB),NOSPLIT,$0-32
	MOVD	dig+0(FP), R0
	MOVD	p+8(FP), R1
	MOVD	p_len+16(FP), R2
	AND	$~63, R2
	CBZ	R2, zero

	ADD	R1, R2, R21
	LDPW	(0*8)(R0), (R4, R5)
	LDPW	(1*8)(R0), (R6, R7)

loop:
	MOVW	R4, R12
	MOVW	R5, R13
	MOVW	R6, R14
	MOVW	R7, R15

	MOVW	(0*4)(R1), R8
	MOVW	R7, R9

#define ROUND1(a, b, c, d, index, const, shift) \
	ADDW	$const, a; \
	ADDW	R8, a; \
	MOVW	(index*4)(R1), R8; \
	EORW	c, R9; \
	ANDW	b, R9; \
	EORW	d, R9; \
	ADDW	R9, a; \
	RORW	$(32-shift), a; \
	MOVW	c, R9; \
	ADDW	b, a

	ROUND1(R4,R5,R6,R7, 1,0xd76aa478, 7);
	ROUND1(R7,R4,R5,R6, 2,0xe8c7b756,12);
	ROUND1(R6,R7,R4,R5, 3,0x242070db,17);
	ROUND1(R5,R6,R7,R4, 4,0xc1bdceee,22);
	ROUND1(R4,R5,R6,R7, 5,0xf57c0faf, 7);
	ROUND1(R7,R4,R5,R6, 6,0x4787c62a,12);
	ROUND1(R6,R7,R4,R5, 7,0xa8304613,17);
	ROUND1(R5,R6,R7,R4, 8,0xfd469501,22);
	ROUND1(R4,R5,R6,R7, 9,0x698098d8, 7);
	ROUND1(R7,R4,R5,R6,10,0x8b44f7af,12);
	ROUND1(R6,R7,R4,R5,11,0xffff5bb1,17);
	ROUND1(R5,R6,R7,R4,12,0x895cd7be,22);
	ROUND1(R4,R5,R6,R7,13,0x6b901122, 7);
	ROUND1(R7,R4,R5,R6,14,0xfd987193,12);
	ROUND1(R6,R7,R4,R5,15,0xa679438e,17);
	ROUND1(R5,R6,R7,R4, 0,0x49b40821,22);

	MOVW	(1*4)(R1), R8
	MOVW	R7, R9
	MOVW	R7, R10

#define ROUND2(a, b, c, d, index, const, shift) \
	ADDW	$const, a; \
	ADDW	R8, a; \
	MOVW	(index*4)(R1), R8; \
	ANDW	b, R10; \
	BICW	R9, c, R9; \
	ORRW	R9, R10; \
	MOVW	c, R9; \
	ADDW	R10, a; \
	MOVW	c, R10; \
	RORW	$(32-shift), a; \
	ADDW	b, a

	ROUND2(R4,R5,R6,R7, 6,0xf61e2562, 5);
	ROUND2(R7,R4,R5,R6,11,0xc040b340, 9);
	ROUND2(R6,R7,R4,R5, 0,0x265e5a51,14);
	ROUND2(R5,R6,R7,R4, 5,0xe9b6c7aa,20);
	ROUND2(R4,R5,R6,R7,10,0xd62f105d, 5);
	ROUND2(R7,R4,R5,R6,15, 0x2441453, 9);
	ROUND2(R6,R7,R4,R5, 4,0xd8a1e681,14);
	ROUND2(R5,R6,R7,R4, 9,0xe7d3fbc8,20);
	ROUND2(R4,R5,R6,R7,14,0x21e1cde6, 5);
	ROUND2(R7,R4,R5,R6, 3,0xc33707d6, 9);
	ROUND2(R6,R7,R4,R5, 8,0xf4d50d87,14);
	ROUND2(R5,R6,R7,R4,13,0x455a14ed,20);
	ROUND2(R4,R5,R6,R7, 2,0xa9e3e905, 5);
	ROUND2(R7,R4,R5,R6, 7,0xfcefa3f8, 9);
	ROUND2(R6,R7,R4,R5,12,0x676f02d9,14);
	ROUND2(R5,R6,R7,R4, 0,0x8d2a4c8a,20);

	MOVW	(5*4)(R1), R8
	MOVW	R6, R9

#define ROUND3(a, b, c, d, index, const, shift) \
	ADDW	$const, a; \
	ADDW	R8, a; \
	MOVW	(index*4)(R1), R8; \
	EORW	d, R9; \
	EORW	b, R9; \
	ADDW	R9, a; \
	RORW	$(32-shift), a; \
	MOVW	b, R9; \
	ADDW	b, a

	ROUND3(R4,R5,R6,R7, 8,0xfffa3942, 4);
	ROUND3(R7,R4,R5,R6,11,0x8771f681,11);
	ROUND3(R6,R7,R4,R5,14,0x6d9d6122,16);
	ROUND3(R5,R6,R7,R4, 1,0xfde5380c,23);
	ROUND3(R4,R5,R6,R7, 4,0xa4beea44, 4);
	ROUND3(R7,R4,R5,R6, 7,0x4bdecfa9,11);
	ROUND3(R6,R7,R4,R5,10,0xf6bb4b60,16);
	ROUND3(R5,R6,R7,R4,13,0xbebfbc70,23);
	ROUND3(R4,R5,R6,R7, 0,0x289b7ec6, 4);
	ROUND3(R7,R4,R5,R6, 3,0xeaa127fa,11);
	ROUND3(R6,R7,R4,R5, 6,0xd4ef3085,16);
	ROUND3(R5,R6,R7,R4, 9, 0x4881d05,23);
	ROUND3(R4,R5,R6,R7,12,0xd9d4d039, 4);
	ROUND3(R7,R4,R5,R6,15,0xe6db99e5,11);
	ROUND3(R6,R7,R4,R5, 2,0x1fa27cf8,16);
	ROUND3(R5,R6,R7,R4, 0,0xc4ac5665,23);

	MOVW	(0*4)(R1), R8
	MVNW	R7, R9

#define ROUND4(a, b, c, d, index, const, shift) \
	ADDW	$const, a; \
	ADDW	R8, a; \
	MOVW	(index*4)(R1), R8; \
	ORRW	b, R9; \
	EORW	c, R9; \
	ADDW	R9, a; \
	RORW	$(32-shift), a; \
	MVNW	c, R9; \
	ADDW	b, a

	ROUND4(R4,R5,R6,R7, 7,0xf4292244, 6);
	ROUND4(R7,R4,R5,R6,14,0x432aff97,10);
	ROUND4(R6,R7,R4,R5, 5,0xab9423a7,15);
	ROUND4(R5,R6,R7,R4,12,0xfc93a039,21);
	ROUND4(R4,R5,R6,R7, 3,0x655b59c3, 6);
	ROUND4(R7,R4,R5,R6,10,0x8f0ccc92,10);
	ROUND4(R6,R7,R4,R5, 1,0xffeff47d,15);
	ROUND4(R5,R6,R7,R4, 8,0x85845dd1,21);
	ROUND4(R4,R5,R6,R7,15,0x6fa87e4f, 6);
	ROUND4(R7,R4,R5,R6, 6,0xfe2ce6e0,10);
	ROUND4(R6,R7,R4,R5,13,0xa3014314,15);
	ROUND4(R5,R6,R7,R4, 4,0x4e0811a1,21);
	ROUND4(R4,R5,R6,R7,11,0xf7537e82, 6);
	ROUND4(R7,R4,R5,R6, 2,0xbd3af235,10);
	ROUND4(R6,R7,R4,R5, 9,0x2ad7d2bb,15);
	ROUND4(R5,R6,R7,R4, 0,0xeb86d391,21);

	ADDW	R12, R4
	ADDW	R13, R5
	ADDW	R14, R6
	ADDW	R15, R7

	ADD	$64, R1
	CMP	R1, R21
	BNE	loop

	STPW	(R4, R5), (0*8)(R0)
	STPW	(R6, R7), (1*8)(R0)
zero:
	RET

```

// === FILE: references!/go/src/crypto/md5/md5block_decl.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build (386 || amd64 || arm || arm64 || loong64 || ppc64 || ppc64le || riscv64 || s390x) && !purego

package md5

const haveAsm = true

//go:noescape
func block(dig *digest, p []byte)

```

// === FILE: references!/go/src/crypto/md5/md5block_generic.go ===
```go
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build (!386 && !amd64 && !arm && !arm64 && !loong64 && !ppc64 && !ppc64le && !riscv64 && !s390x) || purego

package md5

const haveAsm = false

func block(dig *digest, p []byte) {
	blockGeneric(dig, p)
}

```

// === FILE: references!/go/src/crypto/md5/md5block_loong64.s ===
```text
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Loong64 version of md5block.go
// derived from crypto/md5/md5block_amd64.s

//go:build !purego

#define REGTMP	R30
#define REGTMP1 R12
#define REGTMP2 R18

#include "textflag.h"

// func block(dig *digest, p []byte)
TEXT	·block(SB),NOSPLIT,$0-32
	MOVV	dig+0(FP), R4
	MOVV	p+8(FP), R5
	MOVV	p_len+16(FP), R6
	AND	$~63, R6
	BEQ	R6, zero

	// p_len >= 64
	ADDV	R5, R6, R24
	MOVW	(0*4)(R4), R7
	MOVW	(1*4)(R4), R8
	MOVW	(2*4)(R4), R9
	MOVW	(3*4)(R4), R10

loop:
	MOVW	R7, R14
	MOVW	R8, R15
	MOVW	R9, R16
	MOVW	R10, R17

	MOVW	(0*4)(R5), R11
	MOVW	R10, REGTMP1

// F = ((c ^ d) & b) ^ d
#define ROUND1(a, b, c, d, index, const, shift) \
	ADDV	$const, a; \
	ADD	R11, a; \
	MOVW	(index*4)(R5), R11; \
	XOR	c, REGTMP1; \
	AND	b, REGTMP1; \
	XOR	d, REGTMP1; \
	ADD	REGTMP1, a; \
	ROTR	$(32-shift), a; \
	MOVW	c, REGTMP1; \
	ADD	b, a

	ROUND1(R7,  R8,  R9,  R10,  1, 0xd76aa478,  7);
	ROUND1(R10, R7,  R8,  R9,   2, 0xe8c7b756, 12);
	ROUND1(R9,  R10, R7,  R8,   3, 0x242070db, 17);
	ROUND1(R8,  R9,  R10, R7,   4, 0xc1bdceee, 22);
	ROUND1(R7,  R8,  R9,  R10,  5, 0xf57c0faf,  7);
	ROUND1(R10, R7,  R8,  R9,   6, 0x4787c62a, 12);
	ROUND1(R9,  R10, R7,  R8,   7, 0xa8304613, 17);
	ROUND1(R8,  R9,  R10, R7,   8, 0xfd469501, 22);
	ROUND1(R7,  R8,  R9,  R10,  9, 0x698098d8,  7);
	ROUND1(R10, R7,  R8,  R9,  10, 0x8b44f7af, 12);
	ROUND1(R9,  R10, R7,  R8,  11, 0xffff5bb1, 17);
	ROUND1(R8,  R9,  R10, R7,  12, 0x895cd7be, 22);
	ROUND1(R7,  R8,  R9,  R10, 13, 0x6b901122,  7);
	ROUND1(R10, R7,  R8,  R9,  14, 0xfd987193, 12);
	ROUND1(R9,  R10, R7,  R8,  15, 0xa679438e, 17);
	ROUND1(R8,  R9,  R10, R7,   1, 0x49b40821, 22);

	MOVW	(1*4)(R5), R11

// F = ((b ^ c) & d) ^ c
#define ROUND2(a, b, c, d, index, const, shift) \
	ADDV	$const, a; \
	ADD	R11, a; \
	MOVW	(index*4)(R5), R11; \
	XOR	b, c, REGTMP; \
	AND	REGTMP, d, REGTMP; \
	XOR	REGTMP, c, REGTMP; \
	ADD	REGTMP, a; \
	ROTR	$(32-shift), a; \
	ADD	b, a

	ROUND2(R7,  R8,  R9,  R10,  6, 0xf61e2562,  5);
	ROUND2(R10, R7,  R8,  R9,  11, 0xc040b340,  9);
	ROUND2(R9,  R10, R7,  R8,   0, 0x265e5a51, 14);
	ROUND2(R8,  R9,  R10, R7,   5, 0xe9b6c7aa, 20);
	ROUND2(R7,  R8,  R9,  R10, 10, 0xd62f105d,  5);
	ROUND2(R10, R7,  R8,  R9,  15,  0x2441453,  9);
	ROUND2(R9,  R10, R7,  R8,   4, 0xd8a1e681, 14);
	ROUND2(R8,  R9,  R10, R7,   9, 0xe7d3fbc8, 20);
	ROUND2(R7,  R8,  R9,  R10, 14, 0x21e1cde6,  5);
	ROUND2(R10, R7,  R8,  R9,   3, 0xc33707d6,  9);
	ROUND2(R9,  R10, R7,  R8,   8, 0xf4d50d87, 14);
	ROUND2(R8,  R9,  R10, R7,  13, 0x455a14ed, 20);
	ROUND2(R7,  R8,  R9,  R10,  2, 0xa9e3e905,  5);
	ROUND2(R10, R7,  R8,  R9,   7, 0xfcefa3f8,  9);
	ROUND2(R9,  R10, R7,  R8,  12, 0x676f02d9, 14);
	ROUND2(R8,  R9,  R10, R7,   5, 0x8d2a4c8a, 20);

	MOVW	(5*4)(R5), R11
	MOVW	R9, REGTMP1

// F = b ^ c ^ d
#define ROUND3(a, b, c, d, index, const, shift) \
	ADDV	$const, a; \
	ADD	R11, a; \
	MOVW	(index*4)(R5), R11; \
	XOR	d, REGTMP1; \
	XOR	b, REGTMP1; \
	ADD	REGTMP1, a; \
	ROTR	$(32-shift), a; \
	MOVW	b, REGTMP1; \
	ADD	b, a

	ROUND3(R7,  R8,  R9,  R10,  8, 0xfffa3942,  4);
	ROUND3(R10, R7,  R8,  R9,  11, 0x8771f681, 11);
	ROUND3(R9,  R10, R7,  R8,  14, 0x6d9d6122, 16);
	ROUND3(R8,  R9,  R10, R7,   1, 0xfde5380c, 23);
	ROUND3(R7,  R8,  R9,  R10,  4, 0xa4beea44,  4);
	ROUND3(R10, R7,  R8,  R9,   7, 0x4bdecfa9, 11);
	ROUND3(R9,  R10, R7,  R8,  10, 0xf6bb4b60, 16);
	ROUND3(R8,  R9,  R10, R7,  13, 0xbebfbc70, 23);
	ROUND3(R7,  R8,  R9,  R10,  0, 0x289b7ec6,  4);
	ROUND3(R10, R7,  R8,  R9,   3, 0xeaa127fa, 11);
	ROUND3(R9,  R10, R7,  R8,   6, 0xd4ef3085, 16);
	ROUND3(R8,  R9,  R10, R7,   9,  0x4881d05, 23);
	ROUND3(R7,  R8,  R9,  R10, 12, 0xd9d4d039,  4);
	ROUND3(R10, R7,  R8,  R9,  15, 0xe6db99e5, 11);
	ROUND3(R9,  R10, R7,  R8,   2, 0x1fa27cf8, 16);
	ROUND3(R8,  R9,  R10, R7,   0, 0xc4ac5665, 23);

	MOVW	(0*4)(R5), R11
	MOVV	$0xffffffff, REGTMP2
	XOR	R10, REGTMP2, REGTMP1	// REGTMP1 = ~d

// F = c ^ (b | (~d))
#define ROUND4(a, b, c, d, index, const, shift) \
	ADDV	$const, a; \
	ADD	R11, a; \
	MOVW	(index*4)(R5), R11; \
	OR	b, REGTMP1; \
	XOR	c, REGTMP1; \
	ADD	REGTMP1, a; \
	ROTR	$(32-shift), a; \
	MOVV	$0xffffffff, REGTMP2; \
	XOR	c, REGTMP2, REGTMP1; \
	ADD	b, a

	ROUND4(R7,  R8,  R9,  R10,  7, 0xf4292244,  6);
	ROUND4(R10, R7,  R8,  R9,  14, 0x432aff97, 10);
	ROUND4(R9,  R10, R7,  R8,   5, 0xab9423a7, 15);
	ROUND4(R8,  R9,  R10, R7,  12, 0xfc93a039, 21);
	ROUND4(R7,  R8,  R9,  R10,  3, 0x655b59c3,  6);
	ROUND4(R10, R7,  R8,  R9,  10, 0x8f0ccc92, 10);
	ROUND4(R9,  R10, R7,  R8,   1, 0xffeff47d, 15);
	ROUND4(R8,  R9,  R10, R7,   8, 0x85845dd1, 21);
	ROUND4(R7,  R8,  R9,  R10, 15, 0x6fa87e4f,  6);
	ROUND4(R10, R7,  R8,  R9,   6, 0xfe2ce6e0, 10);
	ROUND4(R9,  R10, R7,  R8,  13, 0xa3014314, 15);
	ROUND4(R8,  R9,  R10, R7,   4, 0x4e0811a1, 21);
	ROUND4(R7,  R8,  R9,  R10, 11, 0xf7537e82,  6);
	ROUND4(R10, R7,  R8,  R9,   2, 0xbd3af235, 10);
	ROUND4(R9,  R10, R7,  R8,   9, 0x2ad7d2bb, 15);
	ROUND4(R8,  R9,  R10, R7,   0, 0xeb86d391, 21);

	ADD	R14, R7
	ADD	R15, R8
	ADD	R16, R9
	ADD	R17, R10

	ADDV	$64, R5
	BNE	R5, R24, loop

	MOVW	R7, (0*4)(R4)
	MOVW	R8, (1*4)(R4)
	MOVW	R9, (2*4)(R4)
	MOVW	R10, (3*4)(R4)
zero:
	RET

```

// === FILE: references!/go/src/crypto/md5/md5block_ppc64x.s ===
```text
// Original source:
//	http://www.zorinaq.com/papers/md5-amd64.html
//	http://www.zorinaq.com/papers/md5-amd64.tar.bz2
//
// MD5 optimized for ppc64le using Go's assembler for
// ppc64le, based on md5block_amd64.s implementation by
// the Go authors.
//
// Author: Marc Bevand <bevand_m (at) epita.fr>
// Licence: I hereby disclaim the copyright on this code and place it
// in the public domain.

//go:build (ppc64 || ppc64le) && !purego

#include "textflag.h"

// ENDIAN_MOVE generates the appropriate
// 4 byte load for big or little endian.
// The 4 bytes at ptr+off is loaded into dst.
// The idx reg is only needed for big endian
// and is clobbered when used.
#ifdef GOARCH_ppc64le
#define ENDIAN_MOVE(off, ptr, dst, idx) \
	MOVWZ	off(ptr),dst
#else
#define ENDIAN_MOVE(off, ptr, dst, idx) \
	MOVD	$off,idx; \
	MOVWBR	(idx)(ptr), dst
#endif

#define M00 R18
#define M01 R19
#define M02 R20
#define M03 R24
#define M04 R25
#define M05 R26
#define M06 R27
#define M07 R28
#define M08 R29
#define M09 R21
#define M10 R11
#define M11 R8
#define M12 R7
#define M13 R12
#define M14 R23
#define M15 R10

#define ROUND1(a, b, c, d, index, const, shift) \
	ADD	$const, index, R9; \
	ADD	R9, a; \
	AND     b, c, R9; \
	ANDN    b, d, R31; \
	OR	R9, R31, R9; \
	ADD	R9, a; \
	ROTLW	$shift, a; \
	ADD	b, a;

#define ROUND2(a, b, c, d, index, const, shift) \
	ADD	$const, index, R9; \
	ADD	R9, a; \
	AND	b, d, R31; \
	ANDN	d, c, R9; \
	OR	R9, R31; \
	ADD	R31, a; \
	ROTLW	$shift, a; \
	ADD	b, a;

#define ROUND3(a, b, c, d, index, const, shift) \
	ADD	$const, index, R9; \
	ADD	R9, a; \
	XOR	d, c, R31; \
	XOR	b, R31; \
	ADD	R31, a; \
	ROTLW	$shift, a; \
	ADD	b, a;

#define ROUND4(a, b, c, d, index, const, shift) \
	ADD	$const, index, R9; \
	ADD	R9, a; \
	ORN     d, b, R31; \
	XOR	c, R31; \
	ADD	R31, a; \
	ROTLW	$shift, a; \
	ADD	b, a;


TEXT ·block(SB),NOSPLIT,$0-32
	MOVD	dig+0(FP), R10
	MOVD	p+8(FP), R6
	MOVD	p_len+16(FP), R5

	// We assume p_len >= 64
	SRD 	$6, R5
	MOVD	R5, CTR

	MOVWZ	0(R10), R22
	MOVWZ	4(R10), R3
	MOVWZ	8(R10), R4
	MOVWZ	12(R10), R5

loop:
	MOVD	R22, R14
	MOVD	R3, R15
	MOVD	R4, R16
	MOVD	R5, R17

	ENDIAN_MOVE( 0,R6,M00,M15)
	ENDIAN_MOVE( 4,R6,M01,M15)
	ENDIAN_MOVE( 8,R6,M02,M15)
	ENDIAN_MOVE(12,R6,M03,M15)

	ROUND1(R22,R3,R4,R5,M00,0xd76aa478, 7);
	ROUND1(R5,R22,R3,R4,M01,0xe8c7b756,12);
	ROUND1(R4,R5,R22,R3,M02,0x242070db,17);
	ROUND1(R3,R4,R5,R22,M03,0xc1bdceee,22);

	ENDIAN_MOVE(16,R6,M04,M15)
	ENDIAN_MOVE(20,R6,M05,M15)
	ENDIAN_MOVE(24,R6,M06,M15)
	ENDIAN_MOVE(28,R6,M07,M15)

	ROUND1(R22,R3,R4,R5,M04,0xf57c0faf, 7);
	ROUND1(R5,R22,R3,R4,M05,0x4787c62a,12);
	ROUND1(R4,R5,R22,R3,M06,0xa8304613,17);
	ROUND1(R3,R4,R5,R22,M07,0xfd469501,22);

	ENDIAN_MOVE(32,R6,M08,M15)
	ENDIAN_MOVE(36,R6,M09,M15)
	ENDIAN_MOVE(40,R6,M10,M15)
	ENDIAN_MOVE(44,R6,M11,M15)

	ROUND1(R22,R3,R4,R5,M08,0x698098d8, 7);
	ROUND1(R5,R22,R3,R4,M09,0x8b44f7af,12);
	ROUND1(R4,R5,R22,R3,M10,0xffff5bb1,17);
	ROUND1(R3,R4,R5,R22,M11,0x895cd7be,22);

	ENDIAN_MOVE(48,R6,M12,M15)
	ENDIAN_MOVE(52,R6,M13,M15)
	ENDIAN_MOVE(56,R6,M14,M15)
	ENDIAN_MOVE(60,R6,M15,M15)

	ROUND1(R22,R3,R4,R5,M12,0x6b901122, 7);
	ROUND1(R5,R22,R3,R4,M13,0xfd987193,12);
	ROUND1(R4,R5,R22,R3,M14,0xa679438e,17);
	ROUND1(R3,R4,R5,R22,M15,0x49b40821,22);

	ROUND2(R22,R3,R4,R5,M01,0xf61e2562, 5);
	ROUND2(R5,R22,R3,R4,M06,0xc040b340, 9);
	ROUND2(R4,R5,R22,R3,M11,0x265e5a51,14);
	ROUND2(R3,R4,R5,R22,M00,0xe9b6c7aa,20);
	ROUND2(R22,R3,R4,R5,M05,0xd62f105d, 5);
	ROUND2(R5,R22,R3,R4,M10, 0x2441453, 9);
	ROUND2(R4,R5,R22,R3,M15,0xd8a1e681,14);
	ROUND2(R3,R4,R5,R22,M04,0xe7d3fbc8,20);
	ROUND2(R22,R3,R4,R5,M09,0x21e1cde6, 5);
	ROUND2(R5,R22,R3,R4,M14,0xc33707d6, 9);
	ROUND2(R4,R5,R22,R3,M03,0xf4d50d87,14);
	ROUND2(R3,R4,R5,R22,M08,0x455a14ed,20);
	ROUND2(R22,R3,R4,R5,M13,0xa9e3e905, 5);
	ROUND2(R5,R22,R3,R4,M02,0xfcefa3f8, 9);
	ROUND2(R4,R5,R22,R3,M07,0x676f02d9,14);
	ROUND2(R3,R4,R5,R22,M12,0x8d2a4c8a,20);

	ROUND3(R22,R3,R4,R5,M05,0xfffa3942, 4);
	ROUND3(R5,R22,R3,R4,M08,0x8771f681,11);
	ROUND3(R4,R5,R22,R3,M11,0x6d9d6122,16);
	ROUND3(R3,R4,R5,R22,M14,0xfde5380c,23);
	ROUND3(R22,R3,R4,R5,M01,0xa4beea44, 4);
	ROUND3(R5,R22,R3,R4,M04,0x4bdecfa9,11);
	ROUND3(R4,R5,R22,R3,M07,0xf6bb4b60,16);
	ROUND3(R3,R4,R5,R22,M10,0xbebfbc70,23);
	ROUND3(R22,R3,R4,R5,M13,0x289b7ec6, 4);
	ROUND3(R5,R22,R3,R4,M00,0xeaa127fa,11);
	ROUND3(R4,R5,R22,R3,M03,0xd4ef3085,16);
	ROUND3(R3,R4,R5,R22,M06, 0x4881d05,23);
	ROUND3(R22,R3,R4,R5,M09,0xd9d4d039, 4);
	ROUND3(R5,R22,R3,R4,M12,0xe6db99e5,11);
	ROUND3(R4,R5,R22,R3,M15,0x1fa27cf8,16);
	ROUND3(R3,R4,R5,R22,M02,0xc4ac5665,23);

	ROUND4(R22,R3,R4,R5,M00,0xf4292244, 6);
	ROUND4(R5,R22,R3,R4,M07,0x432aff97,10);
	ROUND4(R4,R5,R22,R3,M14,0xab9423a7,15);
	ROUND4(R3,R4,R5,R22,M05,0xfc93a039,21);
	ROUND4(R22,R3,R4,R5,M12,0x655b59c3, 6);
	ROUND4(R5,R22,R3,R4,M03,0x8f0ccc92,10);
	ROUND4(R4,R5,R22,R3,M10,0xffeff47d,15);
	ROUND4(R3,R4,R5,R22,M01,0x85845dd1,21);
	ROUND4(R22,R3,R4,R5,M08,0x6fa87e4f, 6);
	ROUND4(R5,R22,R3,R4,M15,0xfe2ce6e0,10);
	ROUND4(R4,R5,R22,R3,M06,0xa3014314,15);
	ROUND4(R3,R4,R5,R22,M13,0x4e0811a1,21);
	ROUND4(R22,R3,R4,R5,M04,0xf7537e82, 6);
	ROUND4(R5,R22,R3,R4,M11,0xbd3af235,10);
	ROUND4(R4,R5,R22,R3,M02,0x2ad7d2bb,15);
	ROUND4(R3,R4,R5,R22,M09,0xeb86d391,21);

	ADD	R14, R22
	ADD	R15, R3
	ADD	R16, R4
	ADD	R17, R5
	ADD	$64, R6
	BDNZ	loop

end:
	MOVD	dig+0(FP), R10
	MOVWZ	R22, 0(R10)
	MOVWZ	R3, 4(R10)
	MOVWZ	R4, 8(R10)
	MOVWZ	R5, 12(R10)

	RET

```

// === FILE: references!/go/src/crypto/md5/md5block_riscv64.s ===
```text
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// RISCV64 version of md5block.go
// derived from crypto/md5/md5block_arm64.s and crypto/md5/md5block.go

//go:build !purego

#include "textflag.h"

#define LOAD32U(base, offset, tmp, dest) \
	MOVBU	(offset+0*1)(base), dest; \
	MOVBU	(offset+1*1)(base), tmp; \
	SLL	$8, tmp; \
	OR	tmp, dest; \
	MOVBU	(offset+2*1)(base), tmp; \
	SLL	$16, tmp; \
	OR	tmp, dest; \
	MOVBU	(offset+3*1)(base), tmp; \
	SLL	$24, tmp; \
	OR	tmp, dest

#define LOAD64U(base, offset, tmp1, tmp2, dst) \
	LOAD32U(base, offset, tmp1, dst); \
	LOAD32U(base, offset+4, tmp1, tmp2); \
	SLL	$32, tmp2; \
	OR	tmp2, dst

#define ROUND1EVN(a, b, c, d, x, const, shift) \
	MOV	$const, X23; \
	ADDW	x, a; \
	ADDW	X23, a; \
	XOR	c, d, X23; \
	AND	b, X23; \
	XOR	d, X23; \
	ADDW	X23, a; \
	RORIW	$(32-shift), a; \
	ADDW	b, a

#define ROUND1ODD(a, b, c, d, x, const, shift) \
	MOV	$const, X23; \
	ADDW	X23, a; \
	SRL	$32, x, X23; \
	ADDW	X23, a; \
	XOR	c, d, X23; \
	AND	b, X23; \
	XOR	d, X23; \
	ADDW	X23, a; \
	RORIW	$(32-shift), a; \
	ADDW	b, a

#define ROUND2EVN(a, b, c, d, x, const, shift) \
	MOV	$const, X23; \
	ADDW	x, a; \
	ADDW	X23, a; \
	XOR	b, c, X23; \
	AND	d, X23; \
	XOR	c, X23; \
	ADDW	X23, a; \
	RORIW	$(32-shift), a; \
	ADDW	b, a

#define ROUND2ODD(a, b, c, d, x, const, shift) \
	MOV	$const, X23; \
	ADDW	X23, a; \
	SRL	$32, x, X23; \
	ADDW	X23, a; \
	XOR	b, c, X23; \
	AND	d, X23; \
	XOR	c, X23; \
	ADDW	X23, a; \
	RORIW	$(32-shift), a; \
	ADDW	b, a

#define ROUND3EVN(a, b, c, d, x, const, shift) \
	MOV	$const, X23; \
	ADDW	x, a; \
	ADDW	X23, a; \
	XOR	c, d, X23; \
	XOR	b, X23; \
	ADDW	X23, a; \
	RORIW	$(32-shift), a; \
	ADDW	b, a

#define ROUND3ODD(a, b, c, d, x, const, shift) \
	MOV	$const, X23; \
	ADDW	X23, a; \
	SRL	$32, x, X23; \
	ADDW	X23, a; \
	XOR	c, d, X23; \
	XOR	b, X23; \
	ADDW	X23, a; \
	RORIW	$(32-shift), a; \
	ADDW	b, a

#define ROUND4EVN(a, b, c, d, x, const, shift) \
	MOV	$const, X23; \
	ADDW	x, a; \
	ADDW	X23, a; \
	ORN	d, b, X23; \
	XOR	c, X23; \
	ADDW	X23, a; \
	RORIW	$(32-shift), a; \
	ADDW	b, a

#define ROUND4ODD(a, b, c, d, x, const, shift) \
	MOV	$const, X23; \
	ADDW	X23, a; \
	SRL	$32, x, X23; \
	ADDW	X23, a; \
	ORN	d, b, X23; \
	XOR	c, X23; \
	ADDW	X23, a; \
	RORIW	$(32-shift), a; \
	ADDW	b, a

// Register use for the block function
//
// X5 - X12	: contain the 16 32 bit data items in the block we're
//		  processing.  Odd numbered values, e.g., x1, x3 are stored in
//		  the upper 32 bits of the register.
// X13 - X16	: a, b, c, d
// X17 - X20	: used to store the old values of a, b, c, d, i.e., aa, bb, cc,
//		  dd.  X17 and X18 are also used as temporary registers when
//		  loading unaligned data.
// X22		: pointer to dig.s
// X23		: temporary register
// X28		: pointer to the first byte beyond the end of p
// X29		: pointer to current 64 byte block of data, initially set to
//		  &p[0]
// X30		: temporary register

TEXT	·block(SB),NOSPLIT,$0-32
	MOV	p+8(FP), X29
	MOV	p_len+16(FP), X30
	SRL	$6, X30
	SLL	$6, X30
	BEQZ	X30, zero

	ADD	X29, X30, X28

	MOV	dig+0(FP), X22
	MOVWU	(0*4)(X22), X13	// a = s[0]
	MOVWU	(1*4)(X22), X14	// b = s[1]
	MOVWU	(2*4)(X22), X15	// c = s[2]
	MOVWU	(3*4)(X22), X16	// d = s[3]

loop:

	// Load the 64 bytes of data in x0-15 into 8 64 bit registers, X5-X12.
	// Different paths are taken to load the values depending on whether the
	// buffer is 8 byte aligned or not.  We load all the values up front
	// here at the start of the loop to avoid multiple alignment checks and
	// to reduce code size.  It takes 10 instructions to load an unaligned
	// 32 bit value and this value will be used 4 times in the main body
	// of the loop below.

	AND	$7, X29, X30
	BEQZ	X30, aligned

	LOAD64U(X29,0, X17, X18, X5)
	LOAD64U(X29,8, X17, X18, X6)
	LOAD64U(X29,16, X17, X18, X7)
	LOAD64U(X29,24, X17, X18, X8)
	LOAD64U(X29,32, X17, X18, X9)
	LOAD64U(X29,40, X17, X18, X10)
	LOAD64U(X29,48, X17, X18, X11)
	LOAD64U(X29,56, X17, X18, X12)
	JMP block_loaded

aligned:
	MOV	(0*8)(X29), X5
	MOV	(1*8)(X29), X6
	MOV	(2*8)(X29), X7
	MOV	(3*8)(X29), X8
	MOV	(4*8)(X29), X9
	MOV	(5*8)(X29), X10
	MOV	(6*8)(X29), X11
	MOV	(7*8)(X29), X12

block_loaded:
	MOV	X13, X17
	MOV	X14, X18
	MOV	X15, X19
	MOV	X16, X20

	// Some of the hex constants below are too large to fit into a
	// signed 32 bit value.  The assembler will handle these
	// constants in a special way to ensure that they are
	// zero extended.  Our algorithm is only interested in the
	// bottom 32 bits and doesn't care whether constants are
	// sign or zero extended when moved into 64 bit registers.
	// So we use signed constants instead of hex when bit 31 is
	// set so all constants can be loaded by lui+addi.

	ROUND1EVN(X13,X14,X15,X16,X5,  -680876936, 7); // 0xd76aa478
	ROUND1ODD(X16,X13,X14,X15,X5,  -389564586,12); // 0xe8c7b756
	ROUND1EVN(X15,X16,X13,X14,X6,  0x242070db,17); // 0x242070db
	ROUND1ODD(X14,X15,X16,X13,X6, -1044525330,22); // 0xc1bdceee
	ROUND1EVN(X13,X14,X15,X16,X7,  -176418897, 7); // 0xf57c0faf
	ROUND1ODD(X16,X13,X14,X15,X7,  0x4787c62a,12); // 0x4787c62a
	ROUND1EVN(X15,X16,X13,X14,X8, -1473231341,17); // 0xa8304613
	ROUND1ODD(X14,X15,X16,X13,X8,   -45705983,22); // 0xfd469501
	ROUND1EVN(X13,X14,X15,X16,X9,  0x698098d8, 7); // 0x698098d8
	ROUND1ODD(X16,X13,X14,X15,X9, -1958414417,12); // 0x8b44f7af
	ROUND1EVN(X15,X16,X13,X14,X10,     -42063,17); // 0xffff5bb1
	ROUND1ODD(X14,X15,X16,X13,X10,-1990404162,22); // 0x895cd7be
	ROUND1EVN(X13,X14,X15,X16,X11, 0x6b901122, 7); // 0x6b901122
	ROUND1ODD(X16,X13,X14,X15,X11,  -40341101,12); // 0xfd987193
	ROUND1EVN(X15,X16,X13,X14,X12,-1502002290,17); // 0xa679438e
	ROUND1ODD(X14,X15,X16,X13,X12, 0x49b40821,22); // 0x49b40821

	ROUND2ODD(X13,X14,X15,X16,X5,  -165796510, 5); // f61e2562
	ROUND2EVN(X16,X13,X14,X15,X8, -1069501632, 9); // c040b340
	ROUND2ODD(X15,X16,X13,X14,X10, 0x265e5a51,14); // 265e5a51
	ROUND2EVN(X14,X15,X16,X13,X5,  -373897302,20); // e9b6c7aa
	ROUND2ODD(X13,X14,X15,X16,X7,  -701558691, 5); // d62f105d
	ROUND2EVN(X16,X13,X14,X15,X10,  0x2441453, 9); // 2441453
	ROUND2ODD(X15,X16,X13,X14,X12, -660478335,14); // d8a1e681
	ROUND2EVN(X14,X15,X16,X13,X7,  -405537848,20); // e7d3fbc8
	ROUND2ODD(X13,X14,X15,X16,X9,  0x21e1cde6, 5); // 21e1cde6
	ROUND2EVN(X16,X13,X14,X15,X12,-1019803690, 9); // c33707d6
	ROUND2ODD(X15,X16,X13,X14,X6,  -187363961,14); // f4d50d87
	ROUND2EVN(X14,X15,X16,X13,X9,  0x455a14ed,20); // 455a14ed
	ROUND2ODD(X13,X14,X15,X16,X11,-1444681467, 5); // a9e3e905
	ROUND2EVN(X16,X13,X14,X15,X6,   -51403784, 9); // fcefa3f8
	ROUND2ODD(X15,X16,X13,X14,X8,  0x676f02d9,14); // 676f02d9
	ROUND2EVN(X14,X15,X16,X13,X11,-1926607734,20); // 8d2a4c8a

	ROUND3ODD(X13,X14,X15,X16,X7,     -378558, 4); // fffa3942
	ROUND3EVN(X16,X13,X14,X15,X9, -2022574463,11); // 8771f681
	ROUND3ODD(X15,X16,X13,X14,X10, 0x6d9d6122,16); // 6d9d6122
	ROUND3EVN(X14,X15,X16,X13,X12,  -35309556,23); // fde5380c
	ROUND3ODD(X13,X14,X15,X16,X5, -1530992060, 4); // a4beea44
	ROUND3EVN(X16,X13,X14,X15,X7,  0x4bdecfa9,11); // 4bdecfa9
	ROUND3ODD(X15,X16,X13,X14,X8,  -155497632,16); // f6bb4b60
	ROUND3EVN(X14,X15,X16,X13,X10,-1094730640,23); // bebfbc70
	ROUND3ODD(X13,X14,X15,X16,X11, 0x289b7ec6, 4); // 289b7ec6
	ROUND3EVN(X16,X13,X14,X15,X5,  -358537222,11); // eaa127fa
	ROUND3ODD(X15,X16,X13,X14,X6,  -722521979,16); // d4ef3085
	ROUND3EVN(X14,X15,X16,X13,X8,   0x4881d05,23); // 4881d05
	ROUND3ODD(X13,X14,X15,X16,X9,  -640364487, 4); // d9d4d039
	ROUND3EVN(X16,X13,X14,X15,X11, -421815835,11); // e6db99e5
	ROUND3ODD(X15,X16,X13,X14,X12, 0x1fa27cf8,16); // 1fa27cf8
	ROUND3EVN(X14,X15,X16,X13,X6,  -995338651,23); // c4ac5665

	ROUND4EVN(X13,X14,X15,X16,X5,  -198630844, 6); // f4292244
	ROUND4ODD(X16,X13,X14,X15,X8,  0x432aff97,10); // 432aff97
	ROUND4EVN(X15,X16,X13,X14,X12,-1416354905,15); // ab9423a7
	ROUND4ODD(X14,X15,X16,X13,X7,   -57434055,21); // fc93a039
	ROUND4EVN(X13,X14,X15,X16,X11, 0x655b59c3, 6); // 655b59c3
	ROUND4ODD(X16,X13,X14,X15,X6, -1894986606,10); // 8f0ccc92
	ROUND4EVN(X15,X16,X13,X14,X10   ,-1051523,15); // ffeff47d
	ROUND4ODD(X14,X15,X16,X13,X5, -2054922799,21); // 85845dd1
	ROUND4EVN(X13,X14,X15,X16,X9,  0x6fa87e4f, 6); // 6fa87e4f
	ROUND4ODD(X16,X13,X14,X15,X12,  -30611744,10); // fe2ce6e0
	ROUND4EVN(X15,X16,X13,X14,X8, -1560198380,15); // a3014314
	ROUND4ODD(X14,X15,X16,X13,X11, 0x4e0811a1,21); // 4e0811a1
	ROUND4EVN(X13,X14,X15,X16,X7,  -145523070, 6); // f7537e82
	ROUND4ODD(X16,X13,X14,X15,X10,-1120210379,10); // bd3af235
	ROUND4EVN(X15,X16,X13,X14,X6,  0x2ad7d2bb,15); // 2ad7d2bb
	ROUND4ODD(X14,X15,X16,X13,X9,  -343485551,21); // eb86d391

	ADDW	X17, X13
	ADDW	X18, X14
	ADDW	X19, X15
	ADDW	X20, X16

	ADD	$64, X29
	BNE	X28, X29, loop

	MOVW	X13, (0*4)(X22)
	MOVW	X14, (1*4)(X22)
	MOVW	X15, (2*4)(X22)
	MOVW	X16, (3*4)(X22)

zero:
	RET

```

// === FILE: references!/go/src/crypto/md5/md5block_s390x.s ===
```text
// Original source:
//	http://www.zorinaq.com/papers/md5-amd64.html
//	http://www.zorinaq.com/papers/md5-amd64.tar.bz2
//
// MD5 adapted for s390x using Go's assembler for
// s390x, based on md5block_amd64.s implementation by
// the Go authors.
//
// Author: Marc Bevand <bevand_m (at) epita.fr>
// Licence: I hereby disclaim the copyright on this code and place it
// in the public domain.

//go:build !purego

#include "textflag.h"

// func block(dig *digest, p []byte)
TEXT ·block(SB),NOSPLIT,$16-32
	MOVD	dig+0(FP), R1
	MOVD	p+8(FP), R6
	MOVD	p_len+16(FP), R5
	AND	$-64, R5
	LAY	(R6)(R5*1), R7

	LMY	0(R1), R2, R5
	CMPBEQ	R6, R7, end

loop:
	STMY	R2, R5, tmp-16(SP)

	MOVWBR	0(R6), R8
	MOVWZ	R5, R9

#define ROUND1(a, b, c, d, index, const, shift) \
	XOR	c, R9; \
	ADD	$const, a; \
	ADD	R8, a; \
	MOVWBR	(index*4)(R6), R8; \
	AND	b, R9; \
	XOR	d, R9; \
	ADD	R9, a; \
	RLL	$shift, a; \
	MOVWZ	c, R9; \
	ADD	b, a

	ROUND1(R2,R3,R4,R5, 1,0xd76aa478, 7);
	ROUND1(R5,R2,R3,R4, 2,0xe8c7b756,12);
	ROUND1(R4,R5,R2,R3, 3,0x242070db,17);
	ROUND1(R3,R4,R5,R2, 4,0xc1bdceee,22);
	ROUND1(R2,R3,R4,R5, 5,0xf57c0faf, 7);
	ROUND1(R5,R2,R3,R4, 6,0x4787c62a,12);
	ROUND1(R4,R5,R2,R3, 7,0xa8304613,17);
	ROUND1(R3,R4,R5,R2, 8,0xfd469501,22);
	ROUND1(R2,R3,R4,R5, 9,0x698098d8, 7);
	ROUND1(R5,R2,R3,R4,10,0x8b44f7af,12);
	ROUND1(R4,R5,R2,R3,11,0xffff5bb1,17);
	ROUND1(R3,R4,R5,R2,12,0x895cd7be,22);
	ROUND1(R2,R3,R4,R5,13,0x6b901122, 7);
	ROUND1(R5,R2,R3,R4,14,0xfd987193,12);
	ROUND1(R4,R5,R2,R3,15,0xa679438e,17);
	ROUND1(R3,R4,R5,R2, 0,0x49b40821,22);

	MOVWBR	(1*4)(R6), R8
	MOVWZ	R5, R9
	MOVWZ	R5, R1

#define ROUND2(a, b, c, d, index, const, shift) \
	XOR	$0xffffffff, R9; \ // NOTW R9
	ADD	$const, a; \
	ADD	R8, a; \
	MOVWBR	(index*4)(R6), R8; \
	AND	b, R1; \
	AND	c, R9; \
	OR	R9, R1; \
	MOVWZ	c, R9; \
	ADD	R1, a; \
	MOVWZ	c, R1; \
	RLL	$shift,	a; \
	ADD	b, a

	ROUND2(R2,R3,R4,R5, 6,0xf61e2562, 5);
	ROUND2(R5,R2,R3,R4,11,0xc040b340, 9);
	ROUND2(R4,R5,R2,R3, 0,0x265e5a51,14);
	ROUND2(R3,R4,R5,R2, 5,0xe9b6c7aa,20);
	ROUND2(R2,R3,R4,R5,10,0xd62f105d, 5);
	ROUND2(R5,R2,R3,R4,15, 0x2441453, 9);
	ROUND2(R4,R5,R2,R3, 4,0xd8a1e681,14);
	ROUND2(R3,R4,R5,R2, 9,0xe7d3fbc8,20);
	ROUND2(R2,R3,R4,R5,14,0x21e1cde6, 5);
	ROUND2(R5,R2,R3,R4, 3,0xc33707d6, 9);
	ROUND2(R4,R5,R2,R3, 8,0xf4d50d87,14);
	ROUND2(R3,R4,R5,R2,13,0x455a14ed,20);
	ROUND2(R2,R3,R4,R5, 2,0xa9e3e905, 5);
	ROUND2(R5,R2,R3,R4, 7,0xfcefa3f8, 9);
	ROUND2(R4,R5,R2,R3,12,0x676f02d9,14);
	ROUND2(R3,R4,R5,R2, 0,0x8d2a4c8a,20);

	MOVWBR	(5*4)(R6), R8
	MOVWZ	R4, R9

#define ROUND3(a, b, c, d, index, const, shift) \
	ADD	$const, a; \
	ADD	R8, a; \
	MOVWBR	(index*4)(R6), R8; \
	XOR	d, R9; \
	XOR	b, R9; \
	ADD	R9, a; \
	RLL	$shift, a; \
	MOVWZ	b, R9; \
	ADD	b, a

	ROUND3(R2,R3,R4,R5, 8,0xfffa3942, 4);
	ROUND3(R5,R2,R3,R4,11,0x8771f681,11);
	ROUND3(R4,R5,R2,R3,14,0x6d9d6122,16);
	ROUND3(R3,R4,R5,R2, 1,0xfde5380c,23);
	ROUND3(R2,R3,R4,R5, 4,0xa4beea44, 4);
	ROUND3(R5,R2,R3,R4, 7,0x4bdecfa9,11);
	ROUND3(R4,R5,R2,R3,10,0xf6bb4b60,16);
	ROUND3(R3,R4,R5,R2,13,0xbebfbc70,23);
	ROUND3(R2,R3,R4,R5, 0,0x289b7ec6, 4);
	ROUND3(R5,R2,R3,R4, 3,0xeaa127fa,11);
	ROUND3(R4,R5,R2,R3, 6,0xd4ef3085,16);
	ROUND3(R3,R4,R5,R2, 9, 0x4881d05,23);
	ROUND3(R2,R3,R4,R5,12,0xd9d4d039, 4);
	ROUND3(R5,R2,R3,R4,15,0xe6db99e5,11);
	ROUND3(R4,R5,R2,R3, 2,0x1fa27cf8,16);
	ROUND3(R3,R4,R5,R2, 0,0xc4ac5665,23);

	MOVWBR	(0*4)(R6), R8
	MOVWZ	$0xffffffff, R9
	XOR	R5, R9

#define ROUND4(a, b, c, d, index, const, shift) \
	ADD	$const, a; \
	ADD	R8, a; \
	MOVWBR	(index*4)(R6), R8; \
	OR	b, R9; \
	XOR	c, R9; \
	ADD	R9, a; \
	MOVWZ	$0xffffffff, R9; \
	RLL	$shift,	a; \
	XOR	c, R9; \
	ADD	b, a

	ROUND4(R2,R3,R4,R5, 7,0xf4292244, 6);
	ROUND4(R5,R2,R3,R4,14,0x432aff97,10);
	ROUND4(R4,R5,R2,R3, 5,0xab9423a7,15);
	ROUND4(R3,R4,R5,R2,12,0xfc93a039,21);
	ROUND4(R2,R3,R4,R5, 3,0x655b59c3, 6);
	ROUND4(R5,R2,R3,R4,10,0x8f0ccc92,10);
	ROUND4(R4,R5,R2,R3, 1,0xffeff47d,15);
	ROUND4(R3,R4,R5,R2, 8,0x85845dd1,21);
	ROUND4(R2,R3,R4,R5,15,0x6fa87e4f, 6);
	ROUND4(R5,R2,R3,R4, 6,0xfe2ce6e0,10);
	ROUND4(R4,R5,R2,R3,13,0xa3014314,15);
	ROUND4(R3,R4,R5,R2, 4,0x4e0811a1,21);
	ROUND4(R2,R3,R4,R5,11,0xf7537e82, 6);
	ROUND4(R5,R2,R3,R4, 2,0xbd3af235,10);
	ROUND4(R4,R5,R2,R3, 9,0x2ad7d2bb,15);
	ROUND4(R3,R4,R5,R2, 0,0xeb86d391,21);

	MOVWZ	tmp-16(SP), R1
	ADD	R1, R2
	MOVWZ	tmp-12(SP), R1
	ADD	R1, R3
	MOVWZ	tmp-8(SP), R1
	ADD	R1, R4
	MOVWZ	tmp-4(SP), R1
	ADD	R1, R5

	LA	64(R6), R6
	CMPBLT	R6, R7, loop

end:
	MOVD	dig+0(FP), R1
	STMY	R2, R5, 0(R1)
	RET

```

