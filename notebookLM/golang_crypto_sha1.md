# Domain Architecture: crypto/sha1

## Layout Topology
```text
crypto/sha1/
├── _asm
│   ├── go.mod
│   ├── go.sum
│   ├── sha1block_amd64_asm.go
│   └── sha1block_amd64_shani.go
├── sha1.go
├── sha1block.go
├── sha1block_386.s
├── sha1block_amd64.go
├── sha1block_amd64.s
├── sha1block_arm.s
├── sha1block_arm64.go
├── sha1block_arm64.s
├── sha1block_decl.go
├── sha1block_generic.go
├── sha1block_loong64.s
├── sha1block_riscv64.s
├── sha1block_s390x.go
└── sha1block_s390x.s
```

## Source Stream Aggregation

// === FILE: references/go/src/crypto/sha1/_asm/go.mod ===
```text
module crypto/sha1/_asm

go 1.24

require github.com/mmcloughlin/avo v0.6.0

require (
	golang.org/x/mod v0.20.0 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/tools v0.24.0 // indirect
)

```

// === FILE: references/go/src/crypto/sha1/_asm/go.sum ===
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

// === FILE: references/go/src/crypto/sha1/_asm/sha1block_amd64_asm.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	. "github.com/mmcloughlin/avo/build"
	. "github.com/mmcloughlin/avo/operand"
	. "github.com/mmcloughlin/avo/reg"
)

//go:generate go run . -out ../sha1block_amd64.s -pkg sha1

// AVX2 version by Intel, same algorithm as code in Linux kernel:
// https://github.com/torvalds/linux/blob/master/lib/crypto/x86/sha1-avx2-asm.S
// Authors:
// Ilya Albrekht <ilya.albrekht@intel.com>
// Maxim Locktyukhin <maxim.locktyukhin@intel.com>
// Ronen Zohar <ronen.zohar@intel.com>
// Chandramouli Narayanan <mouli@linux.intel.com>

func main() {
	Package("crypto/sha1")
	ConstraintExpr("!purego")
	blockAVX2()
	blockSHANI()
	Generate()
}

// This is the implementation using AVX2, BMI1 and BMI2. It is based on:
// "SHA-1 implementation with Intel(R) AVX2 instruction set extensions"
// From http://software.intel.com/en-us/articles
// (look for improving-the-performance-of-the-secure-hash-algorithm-1)
// This implementation is 2x unrolled, and interleaves vector instructions,
// used to precompute W, with scalar computation of current round
// for optimal scheduling.

// Trivial helper macros.

func UPDATE_HASH(A, TB, C, D, E GPPhysical) {
	ADDL(Mem{Base: R9}, A)
	MOVL(A, Mem{Base: R9})
	ADDL(Mem{Base: R9}.Offset(4), TB)
	MOVL(TB, Mem{Base: R9}.Offset(4))
	ADDL(Mem{Base: R9}.Offset(8), C)
	MOVL(C, Mem{Base: R9}.Offset(8))
	ADDL(Mem{Base: R9}.Offset(12), D)
	MOVL(D, Mem{Base: R9}.Offset(12))
	ADDL(Mem{Base: R9}.Offset(16), E)
	MOVL(E, Mem{Base: R9}.Offset(16))
}

// Helper macros for PRECALC, which does precomputations

func PRECALC_0(OFFSET int) {
	VMOVDQU(Mem{Base: R10}.Offset(OFFSET), X0)
}

func PRECALC_1(OFFSET int) {
	VINSERTI128(Imm(1), Mem{Base: R13}.Offset(OFFSET), Y0, Y0)
}

func PRECALC_2(YREG VecPhysical) {
	VPSHUFB(Y10, Y0, YREG)
}

func PRECALC_4(YREG VecPhysical, K_OFFSET int) {
	VPADDD(Mem{Base: R8}.Offset(K_OFFSET), YREG, Y0)
}

func PRECALC_7(OFFSET int) {
	VMOVDQU(Y0, Mem{Base: R14}.Offset(OFFSET*2))
}

// Message scheduling pre-compute for rounds 0-15
//
//   - R13 is a pointer to even 64-byte block
//   - R10 is a pointer to odd 64-byte block
//   - R14 is a pointer to temp buffer
//   - X0 is used as temp register
//   - YREG is clobbered as part of computation
//   - OFFSET chooses 16 byte chunk within a block
//   - R8 is a pointer to constants block
//   - K_OFFSET chooses K constants relevant to this round
//   - X10 holds swap mask
func PRECALC_00_15(OFFSET int, YREG VecPhysical) {
	PRECALC_0(OFFSET)
	PRECALC_1(OFFSET)
	PRECALC_2(YREG)
	PRECALC_4(YREG, 0x0)
	PRECALC_7(OFFSET)
}

// Helper macros for PRECALC_16_31

func PRECALC_16(REG_SUB_16, REG_SUB_12, REG_SUB_4, REG VecPhysical) {
	VPALIGNR(Imm(8), REG_SUB_16, REG_SUB_12, REG) // w[i-14]
	VPSRLDQ(Imm(4), REG_SUB_4, Y0)                // w[i-3]
}

func PRECALC_17(REG_SUB_16, REG_SUB_8, REG VecPhysical) {
	VPXOR(REG_SUB_8, REG, REG)
	VPXOR(REG_SUB_16, Y0, Y0)
}

func PRECALC_18(REG VecPhysical) {
	VPXOR(Y0, REG, REG)
	VPSLLDQ(Imm(12), REG, Y9)
}

func PRECALC_19(REG VecPhysical) {
	VPSLLD(Imm(1), REG, Y0)
	VPSRLD(Imm(31), REG, REG)
}

func PRECALC_20(REG VecPhysical) {
	VPOR(REG, Y0, Y0)
	VPSLLD(Imm(2), Y9, REG)
}

func PRECALC_21(REG VecPhysical) {
	VPSRLD(Imm(30), Y9, Y9)
	VPXOR(REG, Y0, Y0)
}

func PRECALC_23(REG VecPhysical, K_OFFSET, OFFSET int) {
	VPXOR(Y9, Y0, REG)
	VPADDD(Mem{Base: R8}.Offset(K_OFFSET), REG, Y0)
	VMOVDQU(Y0, Mem{Base: R14}.Offset(OFFSET))
}

// Message scheduling pre-compute for rounds 16-31
//   - calculating last 32 w[i] values in 8 XMM registers
//   - pre-calculate K+w[i] values and store to mem
//   - for later load by ALU add instruction.
//   - "brute force" vectorization for rounds 16-31 only
//   - due to w[i]->w[i-3] dependency.
//   - clobbers 5 input ymm registers REG_SUB*
//   - uses X0 and X9 as temp registers
//   - As always, R8 is a pointer to constants block
//   - and R14 is a pointer to temp buffer
func PRECALC_16_31(REG, REG_SUB_4, REG_SUB_8, REG_SUB_12, REG_SUB_16 VecPhysical, K_OFFSET, OFFSET int) {
	PRECALC_16(REG_SUB_16, REG_SUB_12, REG_SUB_4, REG)
	PRECALC_17(REG_SUB_16, REG_SUB_8, REG)
	PRECALC_18(REG)
	PRECALC_19(REG)
	PRECALC_20(REG)
	PRECALC_21(REG)
	PRECALC_23(REG, K_OFFSET, OFFSET)
}

// Helper macros for PRECALC_32_79

func PRECALC_32(REG_SUB_8, REG_SUB_4 VecPhysical) {
	VPALIGNR(Imm(8), REG_SUB_8, REG_SUB_4, Y0)
}

func PRECALC_33(REG_SUB_28, REG VecPhysical) {
	VPXOR(REG_SUB_28, REG, REG)
}

func PRECALC_34(REG_SUB_16 VecPhysical) {
	VPXOR(REG_SUB_16, Y0, Y0)
}

func PRECALC_35(REG VecPhysical) {
	VPXOR(Y0, REG, REG)
}

func PRECALC_36(REG VecPhysical) {
	VPSLLD(Imm(2), REG, Y0)
}

func PRECALC_37(REG VecPhysical) {
	VPSRLD(Imm(30), REG, REG)
	VPOR(REG, Y0, REG)
}

func PRECALC_39(REG VecPhysical, K_OFFSET, OFFSET int) {
	VPADDD(Mem{Base: R8}.Offset(K_OFFSET), REG, Y0)
	VMOVDQU(Y0, Mem{Base: R14}.Offset(OFFSET))
}

// Message scheduling pre-compute for rounds 32-79
// In SHA-1 specification we have:
// w[i] = (w[i-3] ^ w[i-8]  ^ w[i-14] ^ w[i-16]) rol 1
// Which is the same as:
// w[i] = (w[i-6] ^ w[i-16] ^ w[i-28] ^ w[i-32]) rol 2
// This allows for more efficient vectorization,
// since w[i]->w[i-3] dependency is broken

func PRECALC_32_79(REG, REG_SUB_4, REG_SUB_8, REG_SUB_16, REG_SUB_28 VecPhysical, K_OFFSET, OFFSET int) {
	PRECALC_32(REG_SUB_8, REG_SUB_4)
	PRECALC_33(REG_SUB_28, REG)
	PRECALC_34(REG_SUB_16)
	PRECALC_35(REG)
	PRECALC_36(REG)
	PRECALC_37(REG)
	PRECALC_39(REG, K_OFFSET, OFFSET)
}

func PRECALC() {
	PRECALC_00_15(0, Y15)
	PRECALC_00_15(0x10, Y14)
	PRECALC_00_15(0x20, Y13)
	PRECALC_00_15(0x30, Y12)
	PRECALC_16_31(Y8, Y12, Y13, Y14, Y15, 0, 0x80)
	PRECALC_16_31(Y7, Y8, Y12, Y13, Y14, 0x20, 0xa0)
	PRECALC_16_31(Y5, Y7, Y8, Y12, Y13, 0x20, 0xc0)
	PRECALC_16_31(Y3, Y5, Y7, Y8, Y12, 0x20, 0xe0)
	PRECALC_32_79(Y15, Y3, Y5, Y8, Y14, 0x20, 0x100)
	PRECALC_32_79(Y14, Y15, Y3, Y7, Y13, 0x20, 0x120)
	PRECALC_32_79(Y13, Y14, Y15, Y5, Y12, 0x40, 0x140)
	PRECALC_32_79(Y12, Y13, Y14, Y3, Y8, 0x40, 0x160)
	PRECALC_32_79(Y8, Y12, Y13, Y15, Y7, 0x40, 0x180)
	PRECALC_32_79(Y7, Y8, Y12, Y14, Y5, 0x40, 0x1a0)
	PRECALC_32_79(Y5, Y7, Y8, Y13, Y3, 0x40, 0x1c0)
	PRECALC_32_79(Y3, Y5, Y7, Y12, Y15, 0x60, 0x1e0)
	PRECALC_32_79(Y15, Y3, Y5, Y8, Y14, 0x60, 0x200)
	PRECALC_32_79(Y14, Y15, Y3, Y7, Y13, 0x60, 0x220)
	PRECALC_32_79(Y13, Y14, Y15, Y5, Y12, 0x60, 0x240)
	PRECALC_32_79(Y12, Y13, Y14, Y3, Y8, 0x60, 0x260)
}

// Macros calculating individual rounds have general form
// CALC_ROUND_PRE + PRECALC_ROUND + CALC_ROUND_POST
// CALC_ROUND_{PRE,POST} macros follow

func CALC_F1_PRE(OFFSET int, REG_A, REG_B, REG_C, REG_E GPPhysical) {
	ADDL(Mem{Base: R15}.Offset(OFFSET), REG_E)
	ANDNL(REG_C, REG_A, EBP)
	LEAL(Mem{Base: REG_E, Index: REG_B, Scale: 1}, REG_E) // Add F from the previous round
	RORXL(Imm(0x1b), REG_A, R12L)
	RORXL(Imm(2), REG_A, REG_B) //                           for next round
}

func CALC_F1_POST(REG_A, REG_B, REG_E GPPhysical) {
	ANDL(REG_B, REG_A)                                  // b&c
	XORL(EBP, REG_A)                                    // F1 = (b&c) ^ (~b&d)
	LEAL(Mem{Base: REG_E, Index: R12, Scale: 1}, REG_E) // E += A >>> 5
}

// Registers are cyclically rotated DX -> AX -> DI -> SI -> BX -> CX

func CALC_0() {
	MOVL(ESI, EBX) // Precalculating first round
	RORXL(Imm(2), ESI, ESI)
	ANDNL(EAX, EBX, EBP)
	ANDL(EDI, EBX)
	XORL(EBP, EBX)
	CALC_F1_PRE(0x0, ECX, EBX, EDI, EDX)
	PRECALC_0(0x80)
	CALC_F1_POST(ECX, ESI, EDX)
}

func CALC_1() {
	CALC_F1_PRE(0x4, EDX, ECX, ESI, EAX)
	PRECALC_1(0x80)
	CALC_F1_POST(EDX, EBX, EAX)
}

func CALC_2() {
	CALC_F1_PRE(0x8, EAX, EDX, EBX, EDI)
	PRECALC_2(Y15)
	CALC_F1_POST(EAX, ECX, EDI)
}

func CALC_3() {
	CALC_F1_PRE(0xc, EDI, EAX, ECX, ESI)
	CALC_F1_POST(EDI, EDX, ESI)
}

func CALC_4() {
	CALC_F1_PRE(0x20, ESI, EDI, EDX, EBX)
	PRECALC_4(Y15, 0x0)
	CALC_F1_POST(ESI, EAX, EBX)
}

func CALC_5() {
	CALC_F1_PRE(0x24, EBX, ESI, EAX, ECX)
	CALC_F1_POST(EBX, EDI, ECX)
}

func CALC_6() {
	CALC_F1_PRE(0x28, ECX, EBX, EDI, EDX)
	CALC_F1_POST(ECX, ESI, EDX)
}

func CALC_7() {
	CALC_F1_PRE(0x2c, EDX, ECX, ESI, EAX)
	PRECALC_7(0x0)
	CALC_F1_POST(EDX, EBX, EAX)
}

func CALC_8() {
	CALC_F1_PRE(0x40, EAX, EDX, EBX, EDI)
	PRECALC_0(0x90)
	CALC_F1_POST(EAX, ECX, EDI)
}

func CALC_9() {
	CALC_F1_PRE(0x44, EDI, EAX, ECX, ESI)
	PRECALC_1(0x90)
	CALC_F1_POST(EDI, EDX, ESI)
}

func CALC_10() {
	CALC_F1_PRE(0x48, ESI, EDI, EDX, EBX)
	PRECALC_2(Y14)
	CALC_F1_POST(ESI, EAX, EBX)
}

func CALC_11() {
	CALC_F1_PRE(0x4c, EBX, ESI, EAX, ECX)
	CALC_F1_POST(EBX, EDI, ECX)
}

func CALC_12() {
	CALC_F1_PRE(0x60, ECX, EBX, EDI, EDX)
	PRECALC_4(Y14, 0x0)
	CALC_F1_POST(ECX, ESI, EDX)
}

func CALC_13() {
	CALC_F1_PRE(0x64, EDX, ECX, ESI, EAX)
	CALC_F1_POST(EDX, EBX, EAX)
}

func CALC_14() {
	CALC_F1_PRE(0x68, EAX, EDX, EBX, EDI)
	CALC_F1_POST(EAX, ECX, EDI)
}

func CALC_15() {
	CALC_F1_PRE(0x6c, EDI, EAX, ECX, ESI)
	PRECALC_7(0x10)
	CALC_F1_POST(EDI, EDX, ESI)
}

func CALC_16() {
	CALC_F1_PRE(0x80, ESI, EDI, EDX, EBX)
	PRECALC_0(0xa0)
	CALC_F1_POST(ESI, EAX, EBX)
}

func CALC_17() {
	CALC_F1_PRE(0x84, EBX, ESI, EAX, ECX)
	PRECALC_1(0xa0)
	CALC_F1_POST(EBX, EDI, ECX)
}

func CALC_18() {
	CALC_F1_PRE(0x88, ECX, EBX, EDI, EDX)
	PRECALC_2(Y13)
	CALC_F1_POST(ECX, ESI, EDX)
}

func CALC_F2_PRE(OFFSET int, REG_A, REG_B, REG_E GPPhysical) {
	ADDL(Mem{Base: R15}.Offset(OFFSET), REG_E)
	LEAL(Mem{Base: REG_E, Index: REG_B, Scale: 1}, REG_E) // Add F from the previous round
	RORXL(Imm(0x1b), REG_A, R12L)
	RORXL(Imm(2), REG_A, REG_B) //                           for next round
}

func CALC_F2_POST(REG_A, REG_B, REG_C, REG_E GPPhysical) {
	XORL(REG_B, REG_A)
	ADDL(R12L, REG_E)
	XORL(REG_C, REG_A)
}

func CALC_19() {
	CALC_F2_PRE(0x8c, EDX, ECX, EAX)
	CALC_F2_POST(EDX, EBX, ESI, EAX)
}

func CALC_20() {
	CALC_F2_PRE(0xa0, EAX, EDX, EDI)
	PRECALC_4(Y13, 0x0)
	CALC_F2_POST(EAX, ECX, EBX, EDI)
}

func CALC_21() {
	CALC_F2_PRE(0xa4, EDI, EAX, ESI)
	CALC_F2_POST(EDI, EDX, ECX, ESI)
}

func CALC_22() {
	CALC_F2_PRE(0xa8, ESI, EDI, EBX)
	CALC_F2_POST(ESI, EAX, EDX, EBX)
}

func CALC_23() {
	CALC_F2_PRE(0xac, EBX, ESI, ECX)
	PRECALC_7(0x20)
	CALC_F2_POST(EBX, EDI, EAX, ECX)
}

func CALC_24() {
	CALC_F2_PRE(0xc0, ECX, EBX, EDX)
	PRECALC_0(0xb0)
	CALC_F2_POST(ECX, ESI, EDI, EDX)
}

func CALC_25() {
	CALC_F2_PRE(0xc4, EDX, ECX, EAX)
	PRECALC_1(0xb0)
	CALC_F2_POST(EDX, EBX, ESI, EAX)
}

func CALC_26() {
	CALC_F2_PRE(0xc8, EAX, EDX, EDI)
	PRECALC_2(Y12)
	CALC_F2_POST(EAX, ECX, EBX, EDI)
}

func CALC_27() {
	CALC_F2_PRE(0xcc, EDI, EAX, ESI)
	CALC_F2_POST(EDI, EDX, ECX, ESI)
}

func CALC_28() {
	CALC_F2_PRE(0xe0, ESI, EDI, EBX)
	PRECALC_4(Y12, 0x0)
	CALC_F2_POST(ESI, EAX, EDX, EBX)
}

func CALC_29() {
	CALC_F2_PRE(0xe4, EBX, ESI, ECX)
	CALC_F2_POST(EBX, EDI, EAX, ECX)
}

func CALC_30() {
	CALC_F2_PRE(0xe8, ECX, EBX, EDX)
	CALC_F2_POST(ECX, ESI, EDI, EDX)
}

func CALC_31() {
	CALC_F2_PRE(0xec, EDX, ECX, EAX)
	PRECALC_7(0x30)
	CALC_F2_POST(EDX, EBX, ESI, EAX)
}

func CALC_32() {
	CALC_F2_PRE(0x100, EAX, EDX, EDI)
	PRECALC_16(Y15, Y14, Y12, Y8)
	CALC_F2_POST(EAX, ECX, EBX, EDI)
}

func CALC_33() {
	CALC_F2_PRE(0x104, EDI, EAX, ESI)
	PRECALC_17(Y15, Y13, Y8)
	CALC_F2_POST(EDI, EDX, ECX, ESI)
}

func CALC_34() {
	CALC_F2_PRE(0x108, ESI, EDI, EBX)
	PRECALC_18(Y8)
	CALC_F2_POST(ESI, EAX, EDX, EBX)
}

func CALC_35() {
	CALC_F2_PRE(0x10c, EBX, ESI, ECX)
	PRECALC_19(Y8)
	CALC_F2_POST(EBX, EDI, EAX, ECX)
}

func CALC_36() {
	CALC_F2_PRE(0x120, ECX, EBX, EDX)
	PRECALC_20(Y8)
	CALC_F2_POST(ECX, ESI, EDI, EDX)
}

func CALC_37() {
	CALC_F2_PRE(0x124, EDX, ECX, EAX)
	PRECALC_21(Y8)
	CALC_F2_POST(EDX, EBX, ESI, EAX)
}

func CALC_38() {
	CALC_F2_PRE(0x128, EAX, EDX, EDI)
	CALC_F2_POST(EAX, ECX, EBX, EDI)
}

func CALC_F3_PRE(OFFSET int, REG_E GPPhysical) {
	ADDL(Mem{Base: R15}.Offset(OFFSET), REG_E)
}

func CALC_F3_POST(REG_A, REG_B, REG_C, REG_E, REG_TB GPPhysical) {
	LEAL(Mem{Base: REG_E, Index: REG_TB, Scale: 1}, REG_E) // Add F from the previous round
	MOVL(REG_B, EBP)
	ORL(REG_A, EBP)
	RORXL(Imm(0x1b), REG_A, R12L)
	RORXL(Imm(2), REG_A, REG_TB)
	ANDL(REG_C, EBP)
	ANDL(REG_B, REG_A)
	ORL(EBP, REG_A)
	ADDL(R12L, REG_E)
}

func CALC_39() {
	CALC_F3_PRE(0x12c, ESI)
	PRECALC_23(Y8, 0x0, 0x80)
	CALC_F3_POST(EDI, EDX, ECX, ESI, EAX)
}

func CALC_40() {
	CALC_F3_PRE(0x140, EBX)
	PRECALC_16(Y14, Y13, Y8, Y7)
	CALC_F3_POST(ESI, EAX, EDX, EBX, EDI)
}

func CALC_41() {
	CALC_F3_PRE(0x144, ECX)
	PRECALC_17(Y14, Y12, Y7)
	CALC_F3_POST(EBX, EDI, EAX, ECX, ESI)
}

func CALC_42() {
	CALC_F3_PRE(0x148, EDX)
	PRECALC_18(Y7)
	CALC_F3_POST(ECX, ESI, EDI, EDX, EBX)
}

func CALC_43() {
	CALC_F3_PRE(0x14c, EAX)
	PRECALC_19(Y7)
	CALC_F3_POST(EDX, EBX, ESI, EAX, ECX)
}

func CALC_44() {
	CALC_F3_PRE(0x160, EDI)
	PRECALC_20(Y7)
	CALC_F3_POST(EAX, ECX, EBX, EDI, EDX)
}

func CALC_45() {
	CALC_F3_PRE(0x164, ESI)
	PRECALC_21(Y7)
	CALC_F3_POST(EDI, EDX, ECX, ESI, EAX)
}

func CALC_46() {
	CALC_F3_PRE(0x168, EBX)
	CALC_F3_POST(ESI, EAX, EDX, EBX, EDI)
}

func CALC_47() {
	CALC_F3_PRE(0x16c, ECX)
	VPXOR(Y9, Y0, Y7)
	VPADDD(Mem{Base: R8}.Offset(0x20), Y7, Y0)
	VMOVDQU(Y0, Mem{Base: R14}.Offset(0xa0))
	CALC_F3_POST(EBX, EDI, EAX, ECX, ESI)
}

func CALC_48() {
	CALC_F3_PRE(0x180, EDX)
	PRECALC_16(Y13, Y12, Y7, Y5)
	CALC_F3_POST(ECX, ESI, EDI, EDX, EBX)
}

func CALC_49() {
	CALC_F3_PRE(0x184, EAX)
	PRECALC_17(Y13, Y8, Y5)
	CALC_F3_POST(EDX, EBX, ESI, EAX, ECX)
}

func CALC_50() {
	CALC_F3_PRE(0x188, EDI)
	PRECALC_18(Y5)
	CALC_F3_POST(EAX, ECX, EBX, EDI, EDX)
}

func CALC_51() {
	CALC_F3_PRE(0x18c, ESI)
	PRECALC_19(Y5)
	CALC_F3_POST(EDI, EDX, ECX, ESI, EAX)
}

func CALC_52() {
	CALC_F3_PRE(0x1a0, EBX)
	PRECALC_20(Y5)
	CALC_F3_POST(ESI, EAX, EDX, EBX, EDI)
}

func CALC_53() {
	CALC_F3_PRE(0x1a4, ECX)
	PRECALC_21(Y5)
	CALC_F3_POST(EBX, EDI, EAX, ECX, ESI)
}

func CALC_54() {
	CALC_F3_PRE(0x1a8, EDX)
	CALC_F3_POST(ECX, ESI, EDI, EDX, EBX)
}

func CALC_55() {
	CALC_F3_PRE(0x1ac, EAX)
	PRECALC_23(Y5, 0x20, 0xc0)
	CALC_F3_POST(EDX, EBX, ESI, EAX, ECX)
}

func CALC_56() {
	CALC_F3_PRE(0x1c0, EDI)
	PRECALC_16(Y12, Y8, Y5, Y3)
	CALC_F3_POST(EAX, ECX, EBX, EDI, EDX)
}

func CALC_57() {
	CALC_F3_PRE(0x1c4, ESI)
	PRECALC_17(Y12, Y7, Y3)
	CALC_F3_POST(EDI, EDX, ECX, ESI, EAX)
}

func CALC_58() {
	CALC_F3_PRE(0x1c8, EBX)
	PRECALC_18(Y3)
	CALC_F3_POST(ESI, EAX, EDX, EBX, EDI)
}

func CALC_59() {
	CALC_F2_PRE(0x1cc, EBX, ESI, ECX)
	PRECALC_19(Y3)
	CALC_F2_POST(EBX, EDI, EAX, ECX)
}

func CALC_60() {
	CALC_F2_PRE(0x1e0, ECX, EBX, EDX)
	PRECALC_20(Y3)
	CALC_F2_POST(ECX, ESI, EDI, EDX)
}

func CALC_61() {
	CALC_F2_PRE(0x1e4, EDX, ECX, EAX)
	PRECALC_21(Y3)
	CALC_F2_POST(EDX, EBX, ESI, EAX)
}

func CALC_62() {
	CALC_F2_PRE(0x1e8, EAX, EDX, EDI)
	CALC_F2_POST(EAX, ECX, EBX, EDI)
}

func CALC_63() {
	CALC_F2_PRE(0x1ec, EDI, EAX, ESI)
	PRECALC_23(Y3, 0x20, 0xe0)
	CALC_F2_POST(EDI, EDX, ECX, ESI)
}

func CALC_64() {
	CALC_F2_PRE(0x200, ESI, EDI, EBX)
	PRECALC_32(Y5, Y3)
	CALC_F2_POST(ESI, EAX, EDX, EBX)
}

func CALC_65() {
	CALC_F2_PRE(0x204, EBX, ESI, ECX)
	PRECALC_33(Y14, Y15)
	CALC_F2_POST(EBX, EDI, EAX, ECX)
}

func CALC_66() {
	CALC_F2_PRE(0x208, ECX, EBX, EDX)
	PRECALC_34(Y8)
	CALC_F2_POST(ECX, ESI, EDI, EDX)
}

func CALC_67() {
	CALC_F2_PRE(0x20c, EDX, ECX, EAX)
	PRECALC_35(Y15)
	CALC_F2_POST(EDX, EBX, ESI, EAX)
}

func CALC_68() {
	CALC_F2_PRE(0x220, EAX, EDX, EDI)
	PRECALC_36(Y15)
	CALC_F2_POST(EAX, ECX, EBX, EDI)
}

func CALC_69() {
	CALC_F2_PRE(0x224, EDI, EAX, ESI)
	PRECALC_37(Y15)
	CALC_F2_POST(EDI, EDX, ECX, ESI)
}

func CALC_70() {
	CALC_F2_PRE(0x228, ESI, EDI, EBX)
	CALC_F2_POST(ESI, EAX, EDX, EBX)
}

func CALC_71() {
	CALC_F2_PRE(0x22c, EBX, ESI, ECX)
	PRECALC_39(Y15, 0x20, 0x100)
	CALC_F2_POST(EBX, EDI, EAX, ECX)
}

func CALC_72() {
	CALC_F2_PRE(0x240, ECX, EBX, EDX)
	PRECALC_32(Y3, Y15)
	CALC_F2_POST(ECX, ESI, EDI, EDX)
}

func CALC_73() {
	CALC_F2_PRE(0x244, EDX, ECX, EAX)
	PRECALC_33(Y13, Y14)
	CALC_F2_POST(EDX, EBX, ESI, EAX)
}

func CALC_74() {
	CALC_F2_PRE(0x248, EAX, EDX, EDI)
	PRECALC_34(Y7)
	CALC_F2_POST(EAX, ECX, EBX, EDI)
}

func CALC_75() {
	CALC_F2_PRE(0x24c, EDI, EAX, ESI)
	PRECALC_35(Y14)
	CALC_F2_POST(EDI, EDX, ECX, ESI)
}

func CALC_76() {
	CALC_F2_PRE(0x260, ESI, EDI, EBX)
	PRECALC_36(Y14)
	CALC_F2_POST(ESI, EAX, EDX, EBX)
}

func CALC_77() {
	CALC_F2_PRE(0x264, EBX, ESI, ECX)
	PRECALC_37(Y14)
	CALC_F2_POST(EBX, EDI, EAX, ECX)
}

func CALC_78() {
	CALC_F2_PRE(0x268, ECX, EBX, EDX)
	CALC_F2_POST(ECX, ESI, EDI, EDX)
}

func CALC_79() {
	ADDL(Mem{Base: R15}.Offset(0x26c), EAX)
	LEAL(Mem{Base: AX, Index: CX, Scale: 1}, EAX)
	RORXL(Imm(0x1b), EDX, R12L)
	PRECALC_39(Y14, 0x20, 0x120)
	ADDL(R12L, EAX)
}

// Similar to CALC_0
func CALC_80() {
	MOVL(ECX, EDX)
	RORXL(Imm(2), ECX, ECX)
	ANDNL(ESI, EDX, EBP)
	ANDL(EBX, EDX)
	XORL(EBP, EDX)
	CALC_F1_PRE(0x10, EAX, EDX, EBX, EDI)
	PRECALC_32(Y15, Y14)
	CALC_F1_POST(EAX, ECX, EDI)
}

func CALC_81() {
	CALC_F1_PRE(0x14, EDI, EAX, ECX, ESI)
	PRECALC_33(Y12, Y13)
	CALC_F1_POST(EDI, EDX, ESI)
}

func CALC_82() {
	CALC_F1_PRE(0x18, ESI, EDI, EDX, EBX)
	PRECALC_34(Y5)
	CALC_F1_POST(ESI, EAX, EBX)
}

func CALC_83() {
	CALC_F1_PRE(0x1c, EBX, ESI, EAX, ECX)
	PRECALC_35(Y13)
	CALC_F1_POST(EBX, EDI, ECX)
}

func CALC_84() {
	CALC_F1_PRE(0x30, ECX, EBX, EDI, EDX)
	PRECALC_36(Y13)
	CALC_F1_POST(ECX, ESI, EDX)
}

func CALC_85() {
	CALC_F1_PRE(0x34, EDX, ECX, ESI, EAX)
	PRECALC_37(Y13)
	CALC_F1_POST(EDX, EBX, EAX)
}

func CALC_86() {
	CALC_F1_PRE(0x38, EAX, EDX, EBX, EDI)
	CALC_F1_POST(EAX, ECX, EDI)
}

func CALC_87() {
	CALC_F1_PRE(0x3c, EDI, EAX, ECX, ESI)
	PRECALC_39(Y13, 0x40, 0x140)
	CALC_F1_POST(EDI, EDX, ESI)
}

func CALC_88() {
	CALC_F1_PRE(0x50, ESI, EDI, EDX, EBX)
	PRECALC_32(Y14, Y13)
	CALC_F1_POST(ESI, EAX, EBX)
}

func CALC_89() {
	CALC_F1_PRE(0x54, EBX, ESI, EAX, ECX)
	PRECALC_33(Y8, Y12)
	CALC_F1_POST(EBX, EDI, ECX)
}

func CALC_90() {
	CALC_F1_PRE(0x58, ECX, EBX, EDI, EDX)
	PRECALC_34(Y3)
	CALC_F1_POST(ECX, ESI, EDX)
}

func CALC_91() {
	CALC_F1_PRE(0x5c, EDX, ECX, ESI, EAX)
	PRECALC_35(Y12)
	CALC_F1_POST(EDX, EBX, EAX)
}

func CALC_92() {
	CALC_F1_PRE(0x70, EAX, EDX, EBX, EDI)
	PRECALC_36(Y12)
	CALC_F1_POST(EAX, ECX, EDI)
}

func CALC_93() {
	CALC_F1_PRE(0x74, EDI, EAX, ECX, ESI)
	PRECALC_37(Y12)
	CALC_F1_POST(EDI, EDX, ESI)
}

func CALC_94() {
	CALC_F1_PRE(0x78, ESI, EDI, EDX, EBX)
	CALC_F1_POST(ESI, EAX, EBX)
}

func CALC_95() {
	CALC_F1_PRE(0x7c, EBX, ESI, EAX, ECX)
	PRECALC_39(Y12, 0x40, 0x160)
	CALC_F1_POST(EBX, EDI, ECX)
}

func CALC_96() {
	CALC_F1_PRE(0x90, ECX, EBX, EDI, EDX)
	PRECALC_32(Y13, Y12)
	CALC_F1_POST(ECX, ESI, EDX)
}

func CALC_97() {
	CALC_F1_PRE(0x94, EDX, ECX, ESI, EAX)
	PRECALC_33(Y7, Y8)
	CALC_F1_POST(EDX, EBX, EAX)
}

func CALC_98() {
	CALC_F1_PRE(0x98, EAX, EDX, EBX, EDI)
	PRECALC_34(Y15)
	CALC_F1_POST(EAX, ECX, EDI)
}

func CALC_99() {
	CALC_F2_PRE(0x9c, EDI, EAX, ESI)
	PRECALC_35(Y8)
	CALC_F2_POST(EDI, EDX, ECX, ESI)
}

func CALC_100() {
	CALC_F2_PRE(0xb0, ESI, EDI, EBX)
	PRECALC_36(Y8)
	CALC_F2_POST(ESI, EAX, EDX, EBX)
}

func CALC_101() {
	CALC_F2_PRE(0xb4, EBX, ESI, ECX)
	PRECALC_37(Y8)
	CALC_F2_POST(EBX, EDI, EAX, ECX)
}

func CALC_102() {
	CALC_F2_PRE(0xb8, ECX, EBX, EDX)
	CALC_F2_POST(ECX, ESI, EDI, EDX)
}

func CALC_103() {
	CALC_F2_PRE(0xbc, EDX, ECX, EAX)
	PRECALC_39(Y8, 0x40, 0x180)
	CALC_F2_POST(EDX, EBX, ESI, EAX)
}

func CALC_104() {
	CALC_F2_PRE(0xd0, EAX, EDX, EDI)
	PRECALC_32(Y12, Y8)
	CALC_F2_POST(EAX, ECX, EBX, EDI)
}

func CALC_105() {
	CALC_F2_PRE(0xd4, EDI, EAX, ESI)
	PRECALC_33(Y5, Y7)
	CALC_F2_POST(EDI, EDX, ECX, ESI)
}

func CALC_106() {
	CALC_F2_PRE(0xd8, ESI, EDI, EBX)
	PRECALC_34(Y14)
	CALC_F2_POST(ESI, EAX, EDX, EBX)
}

func CALC_107() {
	CALC_F2_PRE(0xdc, EBX, ESI, ECX)
	PRECALC_35(Y7)
	CALC_F2_POST(EBX, EDI, EAX, ECX)
}

func CALC_108() {
	CALC_F2_PRE(0xf0, ECX, EBX, EDX)
	PRECALC_36(Y7)
	CALC_F2_POST(ECX, ESI, EDI, EDX)
}

func CALC_109() {
	CALC_F2_PRE(0xf4, EDX, ECX, EAX)
	PRECALC_37(Y7)
	CALC_F2_POST(EDX, EBX, ESI, EAX)
}

func CALC_110() {
	CALC_F2_PRE(0xf8, EAX, EDX, EDI)
	CALC_F2_POST(EAX, ECX, EBX, EDI)
}

func CALC_111() {
	CALC_F2_PRE(0xfc, EDI, EAX, ESI)
	PRECALC_39(Y7, 0x40, 0x1a0)
	CALC_F2_POST(EDI, EDX, ECX, ESI)
}

func CALC_112() {
	CALC_F2_PRE(0x110, ESI, EDI, EBX)
	PRECALC_32(Y8, Y7)
	CALC_F2_POST(ESI, EAX, EDX, EBX)
}

func CALC_113() {
	CALC_F2_PRE(0x114, EBX, ESI, ECX)
	PRECALC_33(Y3, Y5)
	CALC_F2_POST(EBX, EDI, EAX, ECX)
}

func CALC_114() {
	CALC_F2_PRE(0x118, ECX, EBX, EDX)
	PRECALC_34(Y13)
	CALC_F2_POST(ECX, ESI, EDI, EDX)
}

func CALC_115() {
	CALC_F2_PRE(0x11c, EDX, ECX, EAX)
	PRECALC_35(Y5)
	CALC_F2_POST(EDX, EBX, ESI, EAX)
}

func CALC_116() {
	CALC_F2_PRE(0x130, EAX, EDX, EDI)
	PRECALC_36(Y5)
	CALC_F2_POST(EAX, ECX, EBX, EDI)
}

func CALC_117() {
	CALC_F2_PRE(0x134, EDI, EAX, ESI)
	PRECALC_37(Y5)
	CALC_F2_POST(EDI, EDX, ECX, ESI)
}

func CALC_118() {
	CALC_F2_PRE(0x138, ESI, EDI, EBX)
	CALC_F2_POST(ESI, EAX, EDX, EBX)
}

func CALC_119() {
	CALC_F3_PRE(0x13c, ECX)
	PRECALC_39(Y5, 0x40, 0x1c0)
	CALC_F3_POST(EBX, EDI, EAX, ECX, ESI)
}

func CALC_120() {
	CALC_F3_PRE(0x150, EDX)
	PRECALC_32(Y7, Y5)
	CALC_F3_POST(ECX, ESI, EDI, EDX, EBX)
}

func CALC_121() {
	CALC_F3_PRE(0x154, EAX)
	PRECALC_33(Y15, Y3)
	CALC_F3_POST(EDX, EBX, ESI, EAX, ECX)
}

func CALC_122() {
	CALC_F3_PRE(0x158, EDI)
	PRECALC_34(Y12)
	CALC_F3_POST(EAX, ECX, EBX, EDI, EDX)
}

func CALC_123() {
	CALC_F3_PRE(0x15c, ESI)
	PRECALC_35(Y3)
	CALC_F3_POST(EDI, EDX, ECX, ESI, EAX)
}

func CALC_124() {
	CALC_F3_PRE(0x170, EBX)
	PRECALC_36(Y3)
	CALC_F3_POST(ESI, EAX, EDX, EBX, EDI)
}

func CALC_125() {
	CALC_F3_PRE(0x174, ECX)
	PRECALC_37(Y3)
	CALC_F3_POST(EBX, EDI, EAX, ECX, ESI)
}

func CALC_126() {
	CALC_F3_PRE(0x178, EDX)
	CALC_F3_POST(ECX, ESI, EDI, EDX, EBX)
}

func CALC_127() {
	CALC_F3_PRE(0x17c, EAX)
	PRECALC_39(Y3, 0x60, 0x1e0)
	CALC_F3_POST(EDX, EBX, ESI, EAX, ECX)
}

func CALC_128() {
	CALC_F3_PRE(0x190, EDI)
	PRECALC_32(Y5, Y3)
	CALC_F3_POST(EAX, ECX, EBX, EDI, EDX)
}

func CALC_129() {
	CALC_F3_PRE(0x194, ESI)
	PRECALC_33(Y14, Y15)
	CALC_F3_POST(EDI, EDX, ECX, ESI, EAX)
}

func CALC_130() {
	CALC_F3_PRE(0x198, EBX)
	PRECALC_34(Y8)
	CALC_F3_POST(ESI, EAX, EDX, EBX, EDI)
}

func CALC_131() {
	CALC_F3_PRE(0x19c, ECX)
	PRECALC_35(Y15)
	CALC_F3_POST(EBX, EDI, EAX, ECX, ESI)
}

func CALC_132() {
	CALC_F3_PRE(0x1b0, EDX)
	PRECALC_36(Y15)
	CALC_F3_POST(ECX, ESI, EDI, EDX, EBX)
}

func CALC_133() {
	CALC_F3_PRE(0x1b4, EAX)
	PRECALC_37(Y15)
	CALC_F3_POST(EDX, EBX, ESI, EAX, ECX)
}

func CALC_134() {
	CALC_F3_PRE(0x1b8, EDI)
	CALC_F3_POST(EAX, ECX, EBX, EDI, EDX)
}

func CALC_135() {
	CALC_F3_PRE(0x1bc, ESI)
	PRECALC_39(Y15, 0x60, 0x200)
	CALC_F3_POST(EDI, EDX, ECX, ESI, EAX)
}

func CALC_136() {
	CALC_F3_PRE(0x1d0, EBX)
	PRECALC_32(Y3, Y15)
	CALC_F3_POST(ESI, EAX, EDX, EBX, EDI)
}

func CALC_137() {
	CALC_F3_PRE(0x1d4, ECX)
	PRECALC_33(Y13, Y14)
	CALC_F3_POST(EBX, EDI, EAX, ECX, ESI)
}

func CALC_138() {
	CALC_F3_PRE(0x1d8, EDX)
	PRECALC_34(Y7)
	CALC_F3_POST(ECX, ESI, EDI, EDX, EBX)
}

func CALC_139() {
	CALC_F2_PRE(0x1dc, EDX, ECX, EAX)
	PRECALC_35(Y14)
	CALC_F2_POST(EDX, EBX, ESI, EAX)
}

func CALC_140() {
	CALC_F2_PRE(0x1f0, EAX, EDX, EDI)
	PRECALC_36(Y14)
	CALC_F2_POST(EAX, ECX, EBX, EDI)
}

func CALC_141() {
	CALC_F2_PRE(0x1f4, EDI, EAX, ESI)
	PRECALC_37(Y14)
	CALC_F2_POST(EDI, EDX, ECX, ESI)
}

func CALC_142() {
	CALC_F2_PRE(0x1f8, ESI, EDI, EBX)
	CALC_F2_POST(ESI, EAX, EDX, EBX)
}

func CALC_143() {
	CALC_F2_PRE(0x1fc, EBX, ESI, ECX)
	PRECALC_39(Y14, 0x60, 0x220)
	CALC_F2_POST(EBX, EDI, EAX, ECX)
}

func CALC_144() {
	CALC_F2_PRE(0x210, ECX, EBX, EDX)
	PRECALC_32(Y15, Y14)
	CALC_F2_POST(ECX, ESI, EDI, EDX)
}

func CALC_145() {
	CALC_F2_PRE(0x214, EDX, ECX, EAX)
	PRECALC_33(Y12, Y13)
	CALC_F2_POST(EDX, EBX, ESI, EAX)
}

func CALC_146() {
	CALC_F2_PRE(0x218, EAX, EDX, EDI)
	PRECALC_34(Y5)
	CALC_F2_POST(EAX, ECX, EBX, EDI)
}

func CALC_147() {
	CALC_F2_PRE(0x21c, EDI, EAX, ESI)
	PRECALC_35(Y13)
	CALC_F2_POST(EDI, EDX, ECX, ESI)
}

func CALC_148() {
	CALC_F2_PRE(0x230, ESI, EDI, EBX)
	PRECALC_36(Y13)
	CALC_F2_POST(ESI, EAX, EDX, EBX)
}

func CALC_149() {
	CALC_F2_PRE(0x234, EBX, ESI, ECX)
	PRECALC_37(Y13)
	CALC_F2_POST(EBX, EDI, EAX, ECX)
}

func CALC_150() {
	CALC_F2_PRE(0x238, ECX, EBX, EDX)
	CALC_F2_POST(ECX, ESI, EDI, EDX)
}

func CALC_151() {
	CALC_F2_PRE(0x23c, EDX, ECX, EAX)
	PRECALC_39(Y13, 0x60, 0x240)
	CALC_F2_POST(EDX, EBX, ESI, EAX)
}

func CALC_152() {
	CALC_F2_PRE(0x250, EAX, EDX, EDI)
	PRECALC_32(Y14, Y13)
	CALC_F2_POST(EAX, ECX, EBX, EDI)
}

func CALC_153() {
	CALC_F2_PRE(0x254, EDI, EAX, ESI)
	PRECALC_33(Y8, Y12)
	CALC_F2_POST(EDI, EDX, ECX, ESI)
}

func CALC_154() {
	CALC_F2_PRE(0x258, ESI, EDI, EBX)
	PRECALC_34(Y3)
	CALC_F2_POST(ESI, EAX, EDX, EBX)
}

func CALC_155() {
	CALC_F2_PRE(0x25c, EBX, ESI, ECX)
	PRECALC_35(Y12)
	CALC_F2_POST(EBX, EDI, EAX, ECX)
}

func CALC_156() {
	CALC_F2_PRE(0x270, ECX, EBX, EDX)
	PRECALC_36(Y12)
	CALC_F2_POST(ECX, ESI, EDI, EDX)
}

func CALC_157() {
	CALC_F2_PRE(0x274, EDX, ECX, EAX)
	PRECALC_37(Y12)
	CALC_F2_POST(EDX, EBX, ESI, EAX)
}

func CALC_158() {
	CALC_F2_PRE(0x278, EAX, EDX, EDI)
	CALC_F2_POST(EAX, ECX, EBX, EDI)
}

func CALC_159() {
	ADDL(Mem{Base: R15}.Offset(0x27c), ESI)
	LEAL(Mem{Base: SI, Index: AX, Scale: 1}, ESI)
	RORXL(Imm(0x1b), EDI, R12L)
	PRECALC_39(Y12, 0x60, 0x260)
	ADDL(R12L, ESI)
}

func CALC() {
	MOVL(Mem{Base: R9}, ECX)
	MOVL(Mem{Base: R9}.Offset(4), ESI)
	MOVL(Mem{Base: R9}.Offset(8), EDI)
	MOVL(Mem{Base: R9}.Offset(12), EAX)
	MOVL(Mem{Base: R9}.Offset(16), EDX)
	MOVQ(RSP, R14)
	LEAQ(Mem{Base: SP}.Offset(2*4*80+32), R15)
	PRECALC() // Precalc WK for first 2 blocks
	XCHGQ(R15, R14)
	loop_avx2()
	begin()
}

// this loops is unrolled
func loop_avx2() {
	Label("loop")
	CMPQ(R10, R8) // we use R8 value (set below) as a signal of a last block
	JNE(LabelRef("begin"))
	VZEROUPPER()
	RET()
}

func begin() {
	Label("begin")
	CALC_0()
	CALC_1()
	CALC_2()
	CALC_3()
	CALC_4()
	CALC_5()
	CALC_6()
	CALC_7()
	CALC_8()
	CALC_9()
	CALC_10()
	CALC_11()
	CALC_12()
	CALC_13()
	CALC_14()
	CALC_15()
	CALC_16()
	CALC_17()
	CALC_18()
	CALC_19()
	CALC_20()
	CALC_21()
	CALC_22()
	CALC_23()
	CALC_24()
	CALC_25()
	CALC_26()
	CALC_27()
	CALC_28()
	CALC_29()
	CALC_30()
	CALC_31()
	CALC_32()
	CALC_33()
	CALC_34()
	CALC_35()
	CALC_36()
	CALC_37()
	CALC_38()
	CALC_39()
	CALC_40()
	CALC_41()
	CALC_42()
	CALC_43()
	CALC_44()
	CALC_45()
	CALC_46()
	CALC_47()
	CALC_48()
	CALC_49()
	CALC_50()
	CALC_51()
	CALC_52()
	CALC_53()
	CALC_54()
	CALC_55()
	CALC_56()
	CALC_57()
	CALC_58()
	CALC_59()
	ADDQ(Imm(128), R10) // move to next even-64-byte block
	CMPQ(R10, R11)      // is current block the last one?
	CMOVQCC(R8, R10)    // signal the last iteration smartly
	CALC_60()
	CALC_61()
	CALC_62()
	CALC_63()
	CALC_64()
	CALC_65()
	CALC_66()
	CALC_67()
	CALC_68()
	CALC_69()
	CALC_70()
	CALC_71()
	CALC_72()
	CALC_73()
	CALC_74()
	CALC_75()
	CALC_76()
	CALC_77()
	CALC_78()
	CALC_79()
	UPDATE_HASH(EAX, EDX, EBX, ESI, EDI)
	CMPQ(R10, R8) // is current block the last one?
	JE(LabelRef("loop"))
	MOVL(EDX, ECX)
	CALC_80()
	CALC_81()
	CALC_82()
	CALC_83()
	CALC_84()
	CALC_85()
	CALC_86()
	CALC_87()
	CALC_88()
	CALC_89()
	CALC_90()
	CALC_91()
	CALC_92()
	CALC_93()
	CALC_94()
	CALC_95()
	CALC_96()
	CALC_97()
	CALC_98()
	CALC_99()
	CALC_100()
	CALC_101()
	CALC_102()
	CALC_103()
	CALC_104()
	CALC_105()
	CALC_106()
	CALC_107()
	CALC_108()
	CALC_109()
	CALC_110()
	CALC_111()
	CALC_112()
	CALC_113()
	CALC_114()
	CALC_115()
	CALC_116()
	CALC_117()
	CALC_118()
	CALC_119()
	CALC_120()
	CALC_121()
	CALC_122()
	CALC_123()
	CALC_124()
	CALC_125()
	CALC_126()
	CALC_127()
	CALC_128()
	CALC_129()
	CALC_130()
	CALC_131()
	CALC_132()
	CALC_133()
	CALC_134()
	CALC_135()
	CALC_136()
	CALC_137()
	CALC_138()
	CALC_139()
	ADDQ(Imm(128), R13) //move to next even-64-byte block
	CMPQ(R13, R11)      //is current block the last one?
	CMOVQCC(R8, R10)
	CALC_140()
	CALC_141()
	CALC_142()
	CALC_143()
	CALC_144()
	CALC_145()
	CALC_146()
	CALC_147()
	CALC_148()
	CALC_149()
	CALC_150()
	CALC_151()
	CALC_152()
	CALC_153()
	CALC_154()
	CALC_155()
	CALC_156()
	CALC_157()
	CALC_158()
	CALC_159()
	UPDATE_HASH(ESI, EDI, EDX, ECX, EBX)
	MOVL(ESI, R12L)
	MOVL(EDI, ESI)
	MOVL(EDX, EDI)
	MOVL(EBX, EDX)
	MOVL(ECX, EAX)
	MOVL(R12L, ECX)
	XCHGQ(R15, R14)
	JMP(LabelRef("loop"))
}

func blockAVX2() {
	Implement("blockAVX2")
	AllocLocal(1408)

	Load(Param("dig"), RDI)
	Load(Param("p").Base(), RSI)
	Load(Param("p").Len(), RDX)
	SHRQ(Imm(6), RDX)
	SHLQ(Imm(6), RDX)

	K_XMM_AR := K_XMM_AR_DATA()
	LEAQ(K_XMM_AR, R8)

	MOVQ(RDI, R9)
	MOVQ(RSI, R10)
	LEAQ(Mem{Base: SI}.Offset(64), R13)

	ADDQ(RSI, RDX)
	ADDQ(Imm(64), RDX)
	MOVQ(RDX, R11)

	CMPQ(R13, R11)
	CMOVQCC(R8, R13)

	BSWAP_SHUFB_CTL := BSWAP_SHUFB_CTL_DATA()
	VMOVDQU(BSWAP_SHUFB_CTL, Y10)
	CALC()
}

// ##~~~~~~~~~~~~~~~~~~~~~~~~~~DATA SECTION~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~##

// Pointers for memoizing Data section symbols
var (
	K_XMM_AR_ptr, BSWAP_SHUFB_CTL_ptr *Mem
)

// To hold Round Constants for K_XMM_AR_DATA

var _K = []uint32{
	0x5A827999,
	0x6ED9EBA1,
	0x8F1BBCDC,
	0xCA62C1D6,
}

func K_XMM_AR_DATA() Mem {
	if K_XMM_AR_ptr != nil {
		return *K_XMM_AR_ptr
	}

	K_XMM_AR := GLOBL("K_XMM_AR", RODATA)
	K_XMM_AR_ptr = &K_XMM_AR

	offset_idx := 0
	for _, v := range _K {
		DATA((offset_idx+0)*4, U32(v))
		DATA((offset_idx+1)*4, U32(v))
		DATA((offset_idx+2)*4, U32(v))
		DATA((offset_idx+3)*4, U32(v))
		DATA((offset_idx+4)*4, U32(v))
		DATA((offset_idx+5)*4, U32(v))
		DATA((offset_idx+6)*4, U32(v))
		DATA((offset_idx+7)*4, U32(v))
		offset_idx += 8
	}
	return K_XMM_AR
}

var BSWAP_SHUFB_CTL_CONSTANTS = [8]uint32{
	0x00010203,
	0x04050607,
	0x08090a0b,
	0x0c0d0e0f,
	0x00010203,
	0x04050607,
	0x08090a0b,
	0x0c0d0e0f,
}

func BSWAP_SHUFB_CTL_DATA() Mem {
	if BSWAP_SHUFB_CTL_ptr != nil {
		return *BSWAP_SHUFB_CTL_ptr
	}

	BSWAP_SHUFB_CTL := GLOBL("BSWAP_SHUFB_CTL", RODATA)
	BSWAP_SHUFB_CTL_ptr = &BSWAP_SHUFB_CTL
	for i, v := range BSWAP_SHUFB_CTL_CONSTANTS {

		DATA(i*4, U32(v))
	}
	return BSWAP_SHUFB_CTL
}

```

// === FILE: references/go/src/crypto/sha1/_asm/sha1block_amd64_shani.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"

	. "github.com/mmcloughlin/avo/build"
	. "github.com/mmcloughlin/avo/operand"
	. "github.com/mmcloughlin/avo/reg"
)

// Implement the SHA-1 block function using the Intel(R) SHA extensions
// (SHA1RNDS4, SHA1NEXTE, SHA1MSG1, and SHA1MSG2). This implementation requires
// the AVX, SHA, SSE2, SSE4.1, and SSSE3 extensions.
//
// Reference:
// S. Gulley, et al, "New Instructions Supporting the Secure Hash
// Algorithm on Intel® Architecture Processors", July 2013
// https://www.intel.com/content/www/us/en/developer/articles/technical/intel-sha-extensions.html

func blockSHANI() {
	Implement("blockSHANI")

	digest := Load(Param("dig"), RDI)
	data := Load(Param("p").Base(), RSI)
	len := Load(Param("p").Len(), RDX)

	abcd := XMM()
	msg0, msg1, msg2, msg3 := XMM(), XMM(), XMM(), XMM()
	e0, e1 := XMM(), XMM()
	shufMask := XMM()

	CMPQ(len, Imm(0))
	JEQ(LabelRef("done"))
	ADDQ(data, len)

	stackPtr := GP64()
	{
		Comment("Allocate space on the stack for saving ABCD and E0, and align it to 16 bytes")
		local := AllocLocal(32 + 16)
		LEAQ(local.Offset(15), stackPtr)
		tmp := GP64()
		MOVQ(U64(15), tmp)
		NOTQ(tmp)
		ANDQ(tmp, stackPtr)
	}
	e0_save := Mem{Base: stackPtr}
	abcd_save := Mem{Base: stackPtr}.Offset(16)

	Comment("Load initial hash state")
	PINSRD(Imm(3), Mem{Base: digest}.Offset(16), e0)
	VMOVDQU(Mem{Base: digest}, abcd)
	PAND(upperMask(), e0)
	PSHUFD(Imm(0x1b), abcd, abcd)

	VMOVDQA(flipMask(), shufMask)

	Label("loop")

	Comment("Save ABCD and E working values")
	VMOVDQA(e0, e0_save)
	VMOVDQA(abcd, abcd_save)

	Comment("Rounds 0-3")
	VMOVDQU(Mem{Base: data}, msg0)
	PSHUFB(shufMask, msg0)
	PADDD(msg0, e0)
	VMOVDQA(abcd, e1)
	SHA1RNDS4(Imm(0), e0, abcd)

	Comment("Rounds 4-7")
	VMOVDQU(Mem{Base: data}.Offset(16), msg1)
	PSHUFB(shufMask, msg1)
	SHA1NEXTE(msg1, e1)
	VMOVDQA(abcd, e0)
	SHA1RNDS4(Imm(0), e1, abcd)
	SHA1MSG1(msg1, msg0)

	Comment("Rounds 8-11")
	VMOVDQU(Mem{Base: data}.Offset(16*2), msg2)
	PSHUFB(shufMask, msg2)
	SHA1NEXTE(msg2, e0)
	VMOVDQA(abcd, e1)
	SHA1RNDS4(Imm(0), e0, abcd)
	SHA1MSG1(msg2, msg1)
	PXOR(msg2, msg0)

	// Rounds 12 through 67 use the same repeated pattern, with e0 and e1 ping-ponging
	// back and forth, and each of the msg temporaries moving up one every four rounds.
	msgs := []VecVirtual{msg3, msg0, msg1, msg2}
	for i := range 14 {
		Comment(fmt.Sprintf("Rounds %d-%d", 12+(i*4), 12+(i*4)+3))
		a, b := e1, e0
		if i == 0 {
			VMOVDQU(Mem{Base: data}.Offset(16*3), msg3)
			PSHUFB(shufMask, msg3)
		}
		if i%2 == 1 {
			a, b = e0, e1
		}
		imm := uint64((12 + i*4) / 20)

		SHA1NEXTE(msgs[i%4], a)
		VMOVDQA(abcd, b)
		SHA1MSG2(msgs[i%4], msgs[(1+i)%4])
		SHA1RNDS4(Imm(imm), a, abcd)
		SHA1MSG1(msgs[i%4], msgs[(3+i)%4])
		PXOR(msgs[i%4], msgs[(2+i)%4])
	}

	Comment("Rounds 68-71")
	SHA1NEXTE(msg1, e1)
	VMOVDQA(abcd, e0)
	SHA1MSG2(msg1, msg2)
	SHA1RNDS4(Imm(3), e1, abcd)
	PXOR(msg1, msg3)

	Comment("Rounds 72-75")
	SHA1NEXTE(msg2, e0)
	VMOVDQA(abcd, e1)
	SHA1MSG2(msg2, msg3)
	SHA1RNDS4(Imm(3), e0, abcd)

	Comment("Rounds 76-79")
	SHA1NEXTE(msg3, e1)
	VMOVDQA(abcd, e0)
	SHA1RNDS4(Imm(3), e1, abcd)

	Comment("Add saved E and ABCD")
	SHA1NEXTE(e0_save, e0)
	PADDD(abcd_save, abcd)

	Comment("Check if we are done, if not return to the loop")
	ADDQ(Imm(64), data)
	CMPQ(data, len)
	JNE(LabelRef("loop"))

	Comment("Write the hash state back to digest")
	PSHUFD(Imm(0x1b), abcd, abcd)
	VMOVDQU(abcd, Mem{Base: digest})
	PEXTRD(Imm(3), e0, Mem{Base: digest}.Offset(16))

	Label("done")
	RET()
}

func flipMask() Mem {
	mask := GLOBL("shuffle_mask", RODATA)
	// 0x000102030405060708090a0b0c0d0e0f
	DATA(0x00, U64(0x08090a0b0c0d0e0f))
	DATA(0x08, U64(0x0001020304050607))
	return mask
}

func upperMask() Mem {
	mask := GLOBL("upper_mask", RODATA)
	// 0xFFFFFFFF000000000000000000000000
	DATA(0x00, U64(0x0000000000000000))
	DATA(0x08, U64(0xFFFFFFFF00000000))
	return mask
}

```

// === FILE: references/go/src/crypto/sha1/sha1.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sha1 implements the SHA-1 hash algorithm as defined in RFC 3174.
//
// SHA-1 is cryptographically broken and should not be used for secure
// applications.
package sha1

import (
	"crypto"
	"crypto/internal/boring"
	"crypto/internal/fips140only"
	"errors"
	"hash"
	"internal/byteorder"
)

func init() {
	crypto.RegisterHash(crypto.SHA1, New)
}

// The size of a SHA-1 checksum in bytes.
const Size = 20

// The blocksize of SHA-1 in bytes.
const BlockSize = 64

const (
	chunk = 64
	init0 = 0x67452301
	init1 = 0xEFCDAB89
	init2 = 0x98BADCFE
	init3 = 0x10325476
	init4 = 0xC3D2E1F0
)

// digest represents the partial evaluation of a checksum.
type digest struct {
	h   [5]uint32
	x   [chunk]byte
	nx  int
	len uint64
}

const (
	magic         = "sha\x01"
	marshaledSize = len(magic) + 5*4 + chunk + 8
)

func (d *digest) MarshalBinary() ([]byte, error) {
	return d.AppendBinary(make([]byte, 0, marshaledSize))
}

func (d *digest) AppendBinary(b []byte) ([]byte, error) {
	b = append(b, magic...)
	b = byteorder.BEAppendUint32(b, d.h[0])
	b = byteorder.BEAppendUint32(b, d.h[1])
	b = byteorder.BEAppendUint32(b, d.h[2])
	b = byteorder.BEAppendUint32(b, d.h[3])
	b = byteorder.BEAppendUint32(b, d.h[4])
	b = append(b, d.x[:d.nx]...)
	b = append(b, make([]byte, len(d.x)-d.nx)...)
	b = byteorder.BEAppendUint64(b, d.len)
	return b, nil
}

func (d *digest) UnmarshalBinary(b []byte) error {
	if len(b) < len(magic) || string(b[:len(magic)]) != magic {
		return errors.New("crypto/sha1: invalid hash state identifier")
	}
	if len(b) != marshaledSize {
		return errors.New("crypto/sha1: invalid hash state size")
	}
	b = b[len(magic):]
	b, d.h[0] = consumeUint32(b)
	b, d.h[1] = consumeUint32(b)
	b, d.h[2] = consumeUint32(b)
	b, d.h[3] = consumeUint32(b)
	b, d.h[4] = consumeUint32(b)
	b = b[copy(d.x[:], b):]
	b, d.len = consumeUint64(b)
	d.nx = int(d.len % chunk)
	return nil
}

func consumeUint64(b []byte) ([]byte, uint64) {
	return b[8:], byteorder.BEUint64(b)
}

func consumeUint32(b []byte) ([]byte, uint32) {
	return b[4:], byteorder.BEUint32(b)
}

func (d *digest) Clone() (hash.Cloner, error) {
	r := *d
	return &r, nil
}

func (d *digest) Reset() {
	d.h[0] = init0
	d.h[1] = init1
	d.h[2] = init2
	d.h[3] = init3
	d.h[4] = init4
	d.nx = 0
	d.len = 0
}

// New returns a new [hash.Hash] computing the SHA1 checksum. The Hash
// also implements [encoding.BinaryMarshaler], [encoding.BinaryAppender] and
// [encoding.BinaryUnmarshaler] to marshal and unmarshal the internal
// state of the hash.
func New() hash.Hash {
	if boring.Enabled {
		return boring.NewSHA1()
	}
	d := new(digest)
	d.Reset()
	return d
}

func (d *digest) Size() int { return Size }

func (d *digest) BlockSize() int { return BlockSize }

func (d *digest) Write(p []byte) (nn int, err error) {
	if fips140only.Enforced() {
		return 0, errors.New("crypto/sha1: use of SHA-1 is not allowed in FIPS 140-only mode")
	}
	boring.Unreachable()
	nn = len(p)
	d.len += uint64(nn)
	if d.nx > 0 {
		n := copy(d.x[d.nx:], p)
		d.nx += n
		if d.nx == chunk {
			block(d, d.x[:])
			d.nx = 0
		}
		p = p[n:]
	}
	if len(p) >= chunk {
		n := len(p) &^ (chunk - 1)
		block(d, p[:n])
		p = p[n:]
	}
	if len(p) > 0 {
		d.nx = copy(d.x[:], p)
	}
	return
}

func (d *digest) Sum(in []byte) []byte {
	boring.Unreachable()
	// Make a copy of d so that caller can keep writing and summing.
	d0 := *d
	hash := d0.checkSum()
	return append(in, hash[:]...)
}

func (d *digest) checkSum() [Size]byte {
	if fips140only.Enforced() {
		panic("crypto/sha1: use of SHA-1 is not allowed in FIPS 140-only mode")
	}

	len := d.len
	// Padding.  Add a 1 bit and 0 bits until 56 bytes mod 64.
	var tmp [64 + 8]byte // padding + length buffer
	tmp[0] = 0x80
	var t uint64
	if len%64 < 56 {
		t = 56 - len%64
	} else {
		t = 64 + 56 - len%64
	}

	// Length in bits.
	len <<= 3
	padlen := tmp[:t+8]
	byteorder.BEPutUint64(padlen[t:], len)
	d.Write(padlen)

	if d.nx != 0 {
		panic("d.nx != 0")
	}

	var digest [Size]byte

	byteorder.BEPutUint32(digest[0:], d.h[0])
	byteorder.BEPutUint32(digest[4:], d.h[1])
	byteorder.BEPutUint32(digest[8:], d.h[2])
	byteorder.BEPutUint32(digest[12:], d.h[3])
	byteorder.BEPutUint32(digest[16:], d.h[4])

	return digest
}

// ConstantTimeSum computes the same result of [Sum] but in constant time
func (d *digest) ConstantTimeSum(in []byte) []byte {
	d0 := *d
	hash := d0.constSum()
	return append(in, hash[:]...)
}

func (d *digest) constSum() [Size]byte {
	if fips140only.Enforced() {
		panic("crypto/sha1: use of SHA-1 is not allowed in FIPS 140-only mode")
	}

	var length [8]byte
	l := d.len << 3
	for i := uint(0); i < 8; i++ {
		length[i] = byte(l >> (56 - 8*i))
	}

	nx := byte(d.nx)
	t := nx - 56                 // if nx < 56 then the MSB of t is one
	mask1b := byte(int8(t) >> 7) // mask1b is 0xFF iff one block is enough

	separator := byte(0x80) // gets reset to 0x00 once used
	for i := byte(0); i < chunk; i++ {
		mask := byte(int8(i-nx) >> 7) // 0x00 after the end of data

		// if we reached the end of the data, replace with 0x80 or 0x00
		d.x[i] = (^mask & separator) | (mask & d.x[i])

		// zero the separator once used
		separator &= mask

		if i >= 56 {
			// we might have to write the length here if all fit in one block
			d.x[i] |= mask1b & length[i-56]
		}
	}

	// compress, and only keep the digest if all fit in one block
	block(d, d.x[:])

	var digest [Size]byte
	for i, s := range d.h {
		digest[i*4] = mask1b & byte(s>>24)
		digest[i*4+1] = mask1b & byte(s>>16)
		digest[i*4+2] = mask1b & byte(s>>8)
		digest[i*4+3] = mask1b & byte(s)
	}

	for i := byte(0); i < chunk; i++ {
		// second block, it's always past the end of data, might start with 0x80
		if i < 56 {
			d.x[i] = separator
			separator = 0
		} else {
			d.x[i] = length[i-56]
		}
	}

	// compress, and only keep the digest if we actually needed the second block
	block(d, d.x[:])

	for i, s := range d.h {
		digest[i*4] |= ^mask1b & byte(s>>24)
		digest[i*4+1] |= ^mask1b & byte(s>>16)
		digest[i*4+2] |= ^mask1b & byte(s>>8)
		digest[i*4+3] |= ^mask1b & byte(s)
	}

	return digest
}

// Sum returns the SHA-1 checksum of the data.
func Sum(data []byte) [Size]byte {
	if boring.Enabled {
		return boring.SHA1(data)
	}
	if fips140only.Enforced() {
		panic("crypto/sha1: use of SHA-1 is not allowed in FIPS 140-only mode")
	}
	var d digest
	d.Reset()
	d.Write(data)
	return d.checkSum()
}

```

// === FILE: references/go/src/crypto/sha1/sha1block.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sha1

import (
	"math/bits"
)

const (
	_K0 = 0x5A827999
	_K1 = 0x6ED9EBA1
	_K2 = 0x8F1BBCDC
	_K3 = 0xCA62C1D6
)

// blockGeneric is a portable, pure Go version of the SHA-1 block step.
// It's used by sha1block_generic.go and tests.
func blockGeneric(dig *digest, p []byte) {
	var w [16]uint32

	h0, h1, h2, h3, h4 := dig.h[0], dig.h[1], dig.h[2], dig.h[3], dig.h[4]
	for len(p) >= chunk {
		// Can interlace the computation of w with the
		// rounds below if needed for speed.
		for i := 0; i < 16; i++ {
			j := i * 4
			w[i] = uint32(p[j])<<24 | uint32(p[j+1])<<16 | uint32(p[j+2])<<8 | uint32(p[j+3])
		}

		a, b, c, d, e := h0, h1, h2, h3, h4

		// Each of the four 20-iteration rounds
		// differs only in the computation of f and
		// the choice of K (_K0, _K1, etc).
		i := 0
		for ; i < 16; i++ {
			f := b&c | (^b)&d
			t := bits.RotateLeft32(a, 5) + f + e + w[i&0xf] + _K0
			a, b, c, d, e = t, a, bits.RotateLeft32(b, 30), c, d
		}
		for ; i < 20; i++ {
			tmp := w[(i-3)&0xf] ^ w[(i-8)&0xf] ^ w[(i-14)&0xf] ^ w[(i)&0xf]
			w[i&0xf] = bits.RotateLeft32(tmp, 1)

			f := b&c | (^b)&d
			t := bits.RotateLeft32(a, 5) + f + e + w[i&0xf] + _K0
			a, b, c, d, e = t, a, bits.RotateLeft32(b, 30), c, d
		}
		for ; i < 40; i++ {
			tmp := w[(i-3)&0xf] ^ w[(i-8)&0xf] ^ w[(i-14)&0xf] ^ w[(i)&0xf]
			w[i&0xf] = bits.RotateLeft32(tmp, 1)
			f := b ^ c ^ d
			t := bits.RotateLeft32(a, 5) + f + e + w[i&0xf] + _K1
			a, b, c, d, e = t, a, bits.RotateLeft32(b, 30), c, d
		}
		for ; i < 60; i++ {
			tmp := w[(i-3)&0xf] ^ w[(i-8)&0xf] ^ w[(i-14)&0xf] ^ w[(i)&0xf]
			w[i&0xf] = bits.RotateLeft32(tmp, 1)
			f := ((b | c) & d) | (b & c)
			t := bits.RotateLeft32(a, 5) + f + e + w[i&0xf] + _K2
			a, b, c, d, e = t, a, bits.RotateLeft32(b, 30), c, d
		}
		for ; i < 80; i++ {
			tmp := w[(i-3)&0xf] ^ w[(i-8)&0xf] ^ w[(i-14)&0xf] ^ w[(i)&0xf]
			w[i&0xf] = bits.RotateLeft32(tmp, 1)
			f := b ^ c ^ d
			t := bits.RotateLeft32(a, 5) + f + e + w[i&0xf] + _K3
			a, b, c, d, e = t, a, bits.RotateLeft32(b, 30), c, d
		}

		h0 += a
		h1 += b
		h2 += c
		h3 += d
		h4 += e

		p = p[chunk:]
	}

	dig.h[0], dig.h[1], dig.h[2], dig.h[3], dig.h[4] = h0, h1, h2, h3, h4
}

```

// === FILE: references/go/src/crypto/sha1/sha1block_386.s ===
```text
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !purego

#include "textflag.h"

// SHA-1 block routine. See sha1block.go for Go equivalent.
//
// There are 80 rounds of 4 types:
//   - rounds 0-15 are type 1 and load data (ROUND1 macro).
//   - rounds 16-19 are type 1 and do not load data (ROUND1x macro).
//   - rounds 20-39 are type 2 and do not load data (ROUND2 macro).
//   - rounds 40-59 are type 3 and do not load data (ROUND3 macro).
//   - rounds 60-79 are type 4 and do not load data (ROUND4 macro).
//
// Each round loads or shuffles the data, then computes a per-round
// function of b, c, d, and then mixes the result into and rotates the
// five registers a, b, c, d, e holding the intermediate results.
//
// The register rotation is implemented by rotating the arguments to
// the round macros instead of by explicit move instructions.

// Like sha1block_amd64.s, but we keep the data and limit pointers on the stack.
// To free up the word pointer (R10 on amd64, DI here), we add it to e during
// LOAD/SHUFFLE instead of during MIX.
//
// The stack holds the intermediate word array - 16 uint32s - at 0(SP) up to 64(SP).
// The saved a, b, c, d, e (R11 through R15 on amd64) are at 64(SP) up to 84(SP).
// The saved limit pointer (DI on amd64) is at 84(SP).
// The saved data pointer (SI on amd64) is at 88(SP).

#define LOAD(index, e) \
	MOVL	88(SP), SI; \
	MOVL	(index*4)(SI), DI; \
	BSWAPL	DI; \
	MOVL	DI, (index*4)(SP); \
	ADDL	DI, e

#define SHUFFLE(index, e) \
	MOVL	(((index)&0xf)*4)(SP), DI; \
	XORL	(((index-3)&0xf)*4)(SP), DI; \
	XORL	(((index-8)&0xf)*4)(SP), DI; \
	XORL	(((index-14)&0xf)*4)(SP), DI; \
	ROLL	$1, DI; \
	MOVL	DI, (((index)&0xf)*4)(SP); \
	ADDL	DI, e

#define FUNC1(a, b, c, d, e) \
	MOVL	d, DI; \
	XORL	c, DI; \
	ANDL	b, DI; \
	XORL	d, DI

#define FUNC2(a, b, c, d, e) \
	MOVL	b, DI; \
	XORL	c, DI; \
	XORL	d, DI

#define FUNC3(a, b, c, d, e) \
	MOVL	b, SI; \
	ORL	c, SI; \
	ANDL	d, SI; \
	MOVL	b, DI; \
	ANDL	c, DI; \
	ORL	SI, DI

#define FUNC4 FUNC2

#define MIX(a, b, c, d, e, const) \
	ROLL	$30, b; \
	ADDL	DI, e; \
	MOVL	a, SI; \
	ROLL	$5, SI; \
	LEAL	const(e)(SI*1), e

#define ROUND1(a, b, c, d, e, index) \
	LOAD(index, e); \
	FUNC1(a, b, c, d, e); \
	MIX(a, b, c, d, e, 0x5A827999)

#define ROUND1x(a, b, c, d, e, index) \
	SHUFFLE(index, e); \
	FUNC1(a, b, c, d, e); \
	MIX(a, b, c, d, e, 0x5A827999)

#define ROUND2(a, b, c, d, e, index) \
	SHUFFLE(index, e); \
	FUNC2(a, b, c, d, e); \
	MIX(a, b, c, d, e, 0x6ED9EBA1)

#define ROUND3(a, b, c, d, e, index) \
	SHUFFLE(index, e); \
	FUNC3(a, b, c, d, e); \
	MIX(a, b, c, d, e, 0x8F1BBCDC)

#define ROUND4(a, b, c, d, e, index) \
	SHUFFLE(index, e); \
	FUNC4(a, b, c, d, e); \
	MIX(a, b, c, d, e, 0xCA62C1D6)

// func block(dig *digest, p []byte)
TEXT ·block(SB),NOSPLIT,$92-16
	MOVL	dig+0(FP),	BP
	MOVL	p+4(FP),	SI
	MOVL	p_len+8(FP),	DX
	SHRL	$6,		DX
	SHLL	$6,		DX

	LEAL	(SI)(DX*1),	DI
	MOVL	(0*4)(BP),	AX
	MOVL	(1*4)(BP),	BX
	MOVL	(2*4)(BP),	CX
	MOVL	(3*4)(BP),	DX
	MOVL	(4*4)(BP),	BP

	CMPL	SI,		DI
	JEQ	end

	MOVL	DI,	84(SP)

loop:
	MOVL	SI,	88(SP)

	MOVL	AX,	64(SP)
	MOVL	BX,	68(SP)
	MOVL	CX,	72(SP)
	MOVL	DX,	76(SP)
	MOVL	BP,	80(SP)

	ROUND1(AX, BX, CX, DX, BP, 0)
	ROUND1(BP, AX, BX, CX, DX, 1)
	ROUND1(DX, BP, AX, BX, CX, 2)
	ROUND1(CX, DX, BP, AX, BX, 3)
	ROUND1(BX, CX, DX, BP, AX, 4)
	ROUND1(AX, BX, CX, DX, BP, 5)
	ROUND1(BP, AX, BX, CX, DX, 6)
	ROUND1(DX, BP, AX, BX, CX, 7)
	ROUND1(CX, DX, BP, AX, BX, 8)
	ROUND1(BX, CX, DX, BP, AX, 9)
	ROUND1(AX, BX, CX, DX, BP, 10)
	ROUND1(BP, AX, BX, CX, DX, 11)
	ROUND1(DX, BP, AX, BX, CX, 12)
	ROUND1(CX, DX, BP, AX, BX, 13)
	ROUND1(BX, CX, DX, BP, AX, 14)
	ROUND1(AX, BX, CX, DX, BP, 15)

	ROUND1x(BP, AX, BX, CX, DX, 16)
	ROUND1x(DX, BP, AX, BX, CX, 17)
	ROUND1x(CX, DX, BP, AX, BX, 18)
	ROUND1x(BX, CX, DX, BP, AX, 19)

	ROUND2(AX, BX, CX, DX, BP, 20)
	ROUND2(BP, AX, BX, CX, DX, 21)
	ROUND2(DX, BP, AX, BX, CX, 22)
	ROUND2(CX, DX, BP, AX, BX, 23)
	ROUND2(BX, CX, DX, BP, AX, 24)
	ROUND2(AX, BX, CX, DX, BP, 25)
	ROUND2(BP, AX, BX, CX, DX, 26)
	ROUND2(DX, BP, AX, BX, CX, 27)
	ROUND2(CX, DX, BP, AX, BX, 28)
	ROUND2(BX, CX, DX, BP, AX, 29)
	ROUND2(AX, BX, CX, DX, BP, 30)
	ROUND2(BP, AX, BX, CX, DX, 31)
	ROUND2(DX, BP, AX, BX, CX, 32)
	ROUND2(CX, DX, BP, AX, BX, 33)
	ROUND2(BX, CX, DX, BP, AX, 34)
	ROUND2(AX, BX, CX, DX, BP, 35)
	ROUND2(BP, AX, BX, CX, DX, 36)
	ROUND2(DX, BP, AX, BX, CX, 37)
	ROUND2(CX, DX, BP, AX, BX, 38)
	ROUND2(BX, CX, DX, BP, AX, 39)

	ROUND3(AX, BX, CX, DX, BP, 40)
	ROUND3(BP, AX, BX, CX, DX, 41)
	ROUND3(DX, BP, AX, BX, CX, 42)
	ROUND3(CX, DX, BP, AX, BX, 43)
	ROUND3(BX, CX, DX, BP, AX, 44)
	ROUND3(AX, BX, CX, DX, BP, 45)
	ROUND3(BP, AX, BX, CX, DX, 46)
	ROUND3(DX, BP, AX, BX, CX, 47)
	ROUND3(CX, DX, BP, AX, BX, 48)
	ROUND3(BX, CX, DX, BP, AX, 49)
	ROUND3(AX, BX, CX, DX, BP, 50)
	ROUND3(BP, AX, BX, CX, DX, 51)
	ROUND3(DX, BP, AX, BX, CX, 52)
	ROUND3(CX, DX, BP, AX, BX, 53)
	ROUND3(BX, CX, DX, BP, AX, 54)
	ROUND3(AX, BX, CX, DX, BP, 55)
	ROUND3(BP, AX, BX, CX, DX, 56)
	ROUND3(DX, BP, AX, BX, CX, 57)
	ROUND3(CX, DX, BP, AX, BX, 58)
	ROUND3(BX, CX, DX, BP, AX, 59)

	ROUND4(AX, BX, CX, DX, BP, 60)
	ROUND4(BP, AX, BX, CX, DX, 61)
	ROUND4(DX, BP, AX, BX, CX, 62)
	ROUND4(CX, DX, BP, AX, BX, 63)
	ROUND4(BX, CX, DX, BP, AX, 64)
	ROUND4(AX, BX, CX, DX, BP, 65)
	ROUND4(BP, AX, BX, CX, DX, 66)
	ROUND4(DX, BP, AX, BX, CX, 67)
	ROUND4(CX, DX, BP, AX, BX, 68)
	ROUND4(BX, CX, DX, BP, AX, 69)
	ROUND4(AX, BX, CX, DX, BP, 70)
	ROUND4(BP, AX, BX, CX, DX, 71)
	ROUND4(DX, BP, AX, BX, CX, 72)
	ROUND4(CX, DX, BP, AX, BX, 73)
	ROUND4(BX, CX, DX, BP, AX, 74)
	ROUND4(AX, BX, CX, DX, BP, 75)
	ROUND4(BP, AX, BX, CX, DX, 76)
	ROUND4(DX, BP, AX, BX, CX, 77)
	ROUND4(CX, DX, BP, AX, BX, 78)
	ROUND4(BX, CX, DX, BP, AX, 79)

	ADDL	64(SP), AX
	ADDL	68(SP), BX
	ADDL	72(SP), CX
	ADDL	76(SP), DX
	ADDL	80(SP), BP

	MOVL	88(SP), SI
	ADDL	$64, SI
	CMPL	SI, 84(SP)
	JB	loop

end:
	MOVL	dig+0(FP), DI
	MOVL	AX, (0*4)(DI)
	MOVL	BX, (1*4)(DI)
	MOVL	CX, (2*4)(DI)
	MOVL	DX, (3*4)(DI)
	MOVL	BP, (4*4)(DI)
	RET

```

// === FILE: references/go/src/crypto/sha1/sha1block_amd64.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !purego

package sha1

import (
	"crypto/internal/impl"
	"internal/cpu"
)

//go:noescape
func blockAVX2(dig *digest, p []byte)

//go:noescape
func blockSHANI(dig *digest, p []byte)

var useAVX2 = cpu.X86.HasAVX && cpu.X86.HasAVX2 && cpu.X86.HasBMI1 && cpu.X86.HasBMI2
var useSHANI = cpu.X86.HasAVX && cpu.X86.HasSHA && cpu.X86.HasSSE41 && cpu.X86.HasSSSE3

func init() {
	impl.Register("sha1", "AVX2", &useAVX2)
	impl.Register("sha1", "SHA-NI", &useSHANI)
}

func block(dig *digest, p []byte) {
	if useSHANI {
		blockSHANI(dig, p)
	} else if useAVX2 && len(p) >= 256 {
		// blockAVX2 calculates sha1 for 2 block per iteration and also
		// interleaves precalculation for next block. So it may read up-to 192
		// bytes past end of p. We could add checks inside blockAVX2, but this
		// would just turn it into a copy of the old pre-AVX2 amd64 SHA1
		// assembly implementation, so just call blockGeneric instead.
		safeLen := len(p) - 128
		if safeLen%128 != 0 {
			safeLen -= 64
		}
		blockAVX2(dig, p[:safeLen])
		blockGeneric(dig, p[safeLen:])
	} else {
		blockGeneric(dig, p)
	}
}

```

// === FILE: references/go/src/crypto/sha1/sha1block_amd64.s ===
```text
// Code generated by command: go run sha1block_amd64_asm.go -out ../sha1block_amd64.s -pkg sha1. DO NOT EDIT.

//go:build !purego

#include "textflag.h"

// func blockAVX2(dig *digest, p []byte)
// Requires: AVX, AVX2, BMI, BMI2, CMOV
TEXT ·blockAVX2(SB), $1408-32
	MOVQ        dig+0(FP), DI
	MOVQ        p_base+8(FP), SI
	MOVQ        p_len+16(FP), DX
	SHRQ        $0x06, DX
	SHLQ        $0x06, DX
	LEAQ        K_XMM_AR<>+0(SB), R8
	MOVQ        DI, R9
	MOVQ        SI, R10
	LEAQ        64(SI), R13
	ADDQ        SI, DX
	ADDQ        $0x40, DX
	MOVQ        DX, R11
	CMPQ        R13, R11
	CMOVQCC     R8, R13
	VMOVDQU     BSWAP_SHUFB_CTL<>+0(SB), Y10
	MOVL        (R9), CX
	MOVL        4(R9), SI
	MOVL        8(R9), DI
	MOVL        12(R9), AX
	MOVL        16(R9), DX
	MOVQ        SP, R14
	LEAQ        672(SP), R15
	VMOVDQU     (R10), X0
	VINSERTI128 $0x01, (R13), Y0, Y0
	VPSHUFB     Y10, Y0, Y15
	VPADDD      (R8), Y15, Y0
	VMOVDQU     Y0, (R14)
	VMOVDQU     16(R10), X0
	VINSERTI128 $0x01, 16(R13), Y0, Y0
	VPSHUFB     Y10, Y0, Y14
	VPADDD      (R8), Y14, Y0
	VMOVDQU     Y0, 32(R14)
	VMOVDQU     32(R10), X0
	VINSERTI128 $0x01, 32(R13), Y0, Y0
	VPSHUFB     Y10, Y0, Y13
	VPADDD      (R8), Y13, Y0
	VMOVDQU     Y0, 64(R14)
	VMOVDQU     48(R10), X0
	VINSERTI128 $0x01, 48(R13), Y0, Y0
	VPSHUFB     Y10, Y0, Y12
	VPADDD      (R8), Y12, Y0
	VMOVDQU     Y0, 96(R14)
	VPALIGNR    $0x08, Y15, Y14, Y8
	VPSRLDQ     $0x04, Y12, Y0
	VPXOR       Y13, Y8, Y8
	VPXOR       Y15, Y0, Y0
	VPXOR       Y0, Y8, Y8
	VPSLLDQ     $0x0c, Y8, Y9
	VPSLLD      $0x01, Y8, Y0
	VPSRLD      $0x1f, Y8, Y8
	VPOR        Y8, Y0, Y0
	VPSLLD      $0x02, Y9, Y8
	VPSRLD      $0x1e, Y9, Y9
	VPXOR       Y8, Y0, Y0
	VPXOR       Y9, Y0, Y8
	VPADDD      (R8), Y8, Y0
	VMOVDQU     Y0, 128(R14)
	VPALIGNR    $0x08, Y14, Y13, Y7
	VPSRLDQ     $0x04, Y8, Y0
	VPXOR       Y12, Y7, Y7
	VPXOR       Y14, Y0, Y0
	VPXOR       Y0, Y7, Y7
	VPSLLDQ     $0x0c, Y7, Y9
	VPSLLD      $0x01, Y7, Y0
	VPSRLD      $0x1f, Y7, Y7
	VPOR        Y7, Y0, Y0
	VPSLLD      $0x02, Y9, Y7
	VPSRLD      $0x1e, Y9, Y9
	VPXOR       Y7, Y0, Y0
	VPXOR       Y9, Y0, Y7
	VPADDD      32(R8), Y7, Y0
	VMOVDQU     Y0, 160(R14)
	VPALIGNR    $0x08, Y13, Y12, Y5
	VPSRLDQ     $0x04, Y7, Y0
	VPXOR       Y8, Y5, Y5
	VPXOR       Y13, Y0, Y0
	VPXOR       Y0, Y5, Y5
	VPSLLDQ     $0x0c, Y5, Y9
	VPSLLD      $0x01, Y5, Y0
	VPSRLD      $0x1f, Y5, Y5
	VPOR        Y5, Y0, Y0
	VPSLLD      $0x02, Y9, Y5
	VPSRLD      $0x1e, Y9, Y9
	VPXOR       Y5, Y0, Y0
	VPXOR       Y9, Y0, Y5
	VPADDD      32(R8), Y5, Y0
	VMOVDQU     Y0, 192(R14)
	VPALIGNR    $0x08, Y12, Y8, Y3
	VPSRLDQ     $0x04, Y5, Y0
	VPXOR       Y7, Y3, Y3
	VPXOR       Y12, Y0, Y0
	VPXOR       Y0, Y3, Y3
	VPSLLDQ     $0x0c, Y3, Y9
	VPSLLD      $0x01, Y3, Y0
	VPSRLD      $0x1f, Y3, Y3
	VPOR        Y3, Y0, Y0
	VPSLLD      $0x02, Y9, Y3
	VPSRLD      $0x1e, Y9, Y9
	VPXOR       Y3, Y0, Y0
	VPXOR       Y9, Y0, Y3
	VPADDD      32(R8), Y3, Y0
	VMOVDQU     Y0, 224(R14)
	VPALIGNR    $0x08, Y5, Y3, Y0
	VPXOR       Y14, Y15, Y15
	VPXOR       Y8, Y0, Y0
	VPXOR       Y0, Y15, Y15
	VPSLLD      $0x02, Y15, Y0
	VPSRLD      $0x1e, Y15, Y15
	VPOR        Y15, Y0, Y15
	VPADDD      32(R8), Y15, Y0
	VMOVDQU     Y0, 256(R14)
	VPALIGNR    $0x08, Y3, Y15, Y0
	VPXOR       Y13, Y14, Y14
	VPXOR       Y7, Y0, Y0
	VPXOR       Y0, Y14, Y14
	VPSLLD      $0x02, Y14, Y0
	VPSRLD      $0x1e, Y14, Y14
	VPOR        Y14, Y0, Y14
	VPADDD      32(R8), Y14, Y0
	VMOVDQU     Y0, 288(R14)
	VPALIGNR    $0x08, Y15, Y14, Y0
	VPXOR       Y12, Y13, Y13
	VPXOR       Y5, Y0, Y0
	VPXOR       Y0, Y13, Y13
	VPSLLD      $0x02, Y13, Y0
	VPSRLD      $0x1e, Y13, Y13
	VPOR        Y13, Y0, Y13
	VPADDD      64(R8), Y13, Y0
	VMOVDQU     Y0, 320(R14)
	VPALIGNR    $0x08, Y14, Y13, Y0
	VPXOR       Y8, Y12, Y12
	VPXOR       Y3, Y0, Y0
	VPXOR       Y0, Y12, Y12
	VPSLLD      $0x02, Y12, Y0
	VPSRLD      $0x1e, Y12, Y12
	VPOR        Y12, Y0, Y12
	VPADDD      64(R8), Y12, Y0
	VMOVDQU     Y0, 352(R14)
	VPALIGNR    $0x08, Y13, Y12, Y0
	VPXOR       Y7, Y8, Y8
	VPXOR       Y15, Y0, Y0
	VPXOR       Y0, Y8, Y8
	VPSLLD      $0x02, Y8, Y0
	VPSRLD      $0x1e, Y8, Y8
	VPOR        Y8, Y0, Y8
	VPADDD      64(R8), Y8, Y0
	VMOVDQU     Y0, 384(R14)
	VPALIGNR    $0x08, Y12, Y8, Y0
	VPXOR       Y5, Y7, Y7
	VPXOR       Y14, Y0, Y0
	VPXOR       Y0, Y7, Y7
	VPSLLD      $0x02, Y7, Y0
	VPSRLD      $0x1e, Y7, Y7
	VPOR        Y7, Y0, Y7
	VPADDD      64(R8), Y7, Y0
	VMOVDQU     Y0, 416(R14)
	VPALIGNR    $0x08, Y8, Y7, Y0
	VPXOR       Y3, Y5, Y5
	VPXOR       Y13, Y0, Y0
	VPXOR       Y0, Y5, Y5
	VPSLLD      $0x02, Y5, Y0
	VPSRLD      $0x1e, Y5, Y5
	VPOR        Y5, Y0, Y5
	VPADDD      64(R8), Y5, Y0
	VMOVDQU     Y0, 448(R14)
	VPALIGNR    $0x08, Y7, Y5, Y0
	VPXOR       Y15, Y3, Y3
	VPXOR       Y12, Y0, Y0
	VPXOR       Y0, Y3, Y3
	VPSLLD      $0x02, Y3, Y0
	VPSRLD      $0x1e, Y3, Y3
	VPOR        Y3, Y0, Y3
	VPADDD      96(R8), Y3, Y0
	VMOVDQU     Y0, 480(R14)
	VPALIGNR    $0x08, Y5, Y3, Y0
	VPXOR       Y14, Y15, Y15
	VPXOR       Y8, Y0, Y0
	VPXOR       Y0, Y15, Y15
	VPSLLD      $0x02, Y15, Y0
	VPSRLD      $0x1e, Y15, Y15
	VPOR        Y15, Y0, Y15
	VPADDD      96(R8), Y15, Y0
	VMOVDQU     Y0, 512(R14)
	VPALIGNR    $0x08, Y3, Y15, Y0
	VPXOR       Y13, Y14, Y14
	VPXOR       Y7, Y0, Y0
	VPXOR       Y0, Y14, Y14
	VPSLLD      $0x02, Y14, Y0
	VPSRLD      $0x1e, Y14, Y14
	VPOR        Y14, Y0, Y14
	VPADDD      96(R8), Y14, Y0
	VMOVDQU     Y0, 544(R14)
	VPALIGNR    $0x08, Y15, Y14, Y0
	VPXOR       Y12, Y13, Y13
	VPXOR       Y5, Y0, Y0
	VPXOR       Y0, Y13, Y13
	VPSLLD      $0x02, Y13, Y0
	VPSRLD      $0x1e, Y13, Y13
	VPOR        Y13, Y0, Y13
	VPADDD      96(R8), Y13, Y0
	VMOVDQU     Y0, 576(R14)
	VPALIGNR    $0x08, Y14, Y13, Y0
	VPXOR       Y8, Y12, Y12
	VPXOR       Y3, Y0, Y0
	VPXOR       Y0, Y12, Y12
	VPSLLD      $0x02, Y12, Y0
	VPSRLD      $0x1e, Y12, Y12
	VPOR        Y12, Y0, Y12
	VPADDD      96(R8), Y12, Y0
	VMOVDQU     Y0, 608(R14)
	XCHGQ       R15, R14

loop:
	CMPQ R10, R8
	JNE  begin
	VZEROUPPER
	RET

begin:
	MOVL        SI, BX
	RORXL       $0x02, SI, SI
	ANDNL       AX, BX, BP
	ANDL        DI, BX
	XORL        BP, BX
	ADDL        (R15), DX
	ANDNL       DI, CX, BP
	LEAL        (DX)(BX*1), DX
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	VMOVDQU     128(R10), X0
	ANDL        SI, CX
	XORL        BP, CX
	LEAL        (DX)(R12*1), DX
	ADDL        4(R15), AX
	ANDNL       SI, DX, BP
	LEAL        (AX)(CX*1), AX
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	VINSERTI128 $0x01, 128(R13), Y0, Y0
	ANDL        BX, DX
	XORL        BP, DX
	LEAL        (AX)(R12*1), AX
	ADDL        8(R15), DI
	ANDNL       BX, AX, BP
	LEAL        (DI)(DX*1), DI
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	VPSHUFB     Y10, Y0, Y15
	ANDL        CX, AX
	XORL        BP, AX
	LEAL        (DI)(R12*1), DI
	ADDL        12(R15), SI
	ANDNL       CX, DI, BP
	LEAL        (SI)(AX*1), SI
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	ANDL        DX, DI
	XORL        BP, DI
	LEAL        (SI)(R12*1), SI
	ADDL        32(R15), BX
	ANDNL       DX, SI, BP
	LEAL        (BX)(DI*1), BX
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	VPADDD      (R8), Y15, Y0
	ANDL        AX, SI
	XORL        BP, SI
	LEAL        (BX)(R12*1), BX
	ADDL        36(R15), CX
	ANDNL       AX, BX, BP
	LEAL        (CX)(SI*1), CX
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	ANDL        DI, BX
	XORL        BP, BX
	LEAL        (CX)(R12*1), CX
	ADDL        40(R15), DX
	ANDNL       DI, CX, BP
	LEAL        (DX)(BX*1), DX
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	ANDL        SI, CX
	XORL        BP, CX
	LEAL        (DX)(R12*1), DX
	ADDL        44(R15), AX
	ANDNL       SI, DX, BP
	LEAL        (AX)(CX*1), AX
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	VMOVDQU     Y0, (R14)
	ANDL        BX, DX
	XORL        BP, DX
	LEAL        (AX)(R12*1), AX
	ADDL        64(R15), DI
	ANDNL       BX, AX, BP
	LEAL        (DI)(DX*1), DI
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	VMOVDQU     144(R10), X0
	ANDL        CX, AX
	XORL        BP, AX
	LEAL        (DI)(R12*1), DI
	ADDL        68(R15), SI
	ANDNL       CX, DI, BP
	LEAL        (SI)(AX*1), SI
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	VINSERTI128 $0x01, 144(R13), Y0, Y0
	ANDL        DX, DI
	XORL        BP, DI
	LEAL        (SI)(R12*1), SI
	ADDL        72(R15), BX
	ANDNL       DX, SI, BP
	LEAL        (BX)(DI*1), BX
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	VPSHUFB     Y10, Y0, Y14
	ANDL        AX, SI
	XORL        BP, SI
	LEAL        (BX)(R12*1), BX
	ADDL        76(R15), CX
	ANDNL       AX, BX, BP
	LEAL        (CX)(SI*1), CX
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	ANDL        DI, BX
	XORL        BP, BX
	LEAL        (CX)(R12*1), CX
	ADDL        96(R15), DX
	ANDNL       DI, CX, BP
	LEAL        (DX)(BX*1), DX
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	VPADDD      (R8), Y14, Y0
	ANDL        SI, CX
	XORL        BP, CX
	LEAL        (DX)(R12*1), DX
	ADDL        100(R15), AX
	ANDNL       SI, DX, BP
	LEAL        (AX)(CX*1), AX
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	ANDL        BX, DX
	XORL        BP, DX
	LEAL        (AX)(R12*1), AX
	ADDL        104(R15), DI
	ANDNL       BX, AX, BP
	LEAL        (DI)(DX*1), DI
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	ANDL        CX, AX
	XORL        BP, AX
	LEAL        (DI)(R12*1), DI
	ADDL        108(R15), SI
	ANDNL       CX, DI, BP
	LEAL        (SI)(AX*1), SI
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	VMOVDQU     Y0, 32(R14)
	ANDL        DX, DI
	XORL        BP, DI
	LEAL        (SI)(R12*1), SI
	ADDL        128(R15), BX
	ANDNL       DX, SI, BP
	LEAL        (BX)(DI*1), BX
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	VMOVDQU     160(R10), X0
	ANDL        AX, SI
	XORL        BP, SI
	LEAL        (BX)(R12*1), BX
	ADDL        132(R15), CX
	ANDNL       AX, BX, BP
	LEAL        (CX)(SI*1), CX
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	VINSERTI128 $0x01, 160(R13), Y0, Y0
	ANDL        DI, BX
	XORL        BP, BX
	LEAL        (CX)(R12*1), CX
	ADDL        136(R15), DX
	ANDNL       DI, CX, BP
	LEAL        (DX)(BX*1), DX
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	VPSHUFB     Y10, Y0, Y13
	ANDL        SI, CX
	XORL        BP, CX
	LEAL        (DX)(R12*1), DX
	ADDL        140(R15), AX
	LEAL        (AX)(CX*1), AX
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	XORL        BX, DX
	ADDL        R12, AX
	XORL        SI, DX
	ADDL        160(R15), DI
	LEAL        (DI)(DX*1), DI
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	VPADDD      (R8), Y13, Y0
	XORL        CX, AX
	ADDL        R12, DI
	XORL        BX, AX
	ADDL        164(R15), SI
	LEAL        (SI)(AX*1), SI
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	XORL        DX, DI
	ADDL        R12, SI
	XORL        CX, DI
	ADDL        168(R15), BX
	LEAL        (BX)(DI*1), BX
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	XORL        AX, SI
	ADDL        R12, BX
	XORL        DX, SI
	ADDL        172(R15), CX
	LEAL        (CX)(SI*1), CX
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	VMOVDQU     Y0, 64(R14)
	XORL        DI, BX
	ADDL        R12, CX
	XORL        AX, BX
	ADDL        192(R15), DX
	LEAL        (DX)(BX*1), DX
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	VMOVDQU     176(R10), X0
	XORL        SI, CX
	ADDL        R12, DX
	XORL        DI, CX
	ADDL        196(R15), AX
	LEAL        (AX)(CX*1), AX
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	VINSERTI128 $0x01, 176(R13), Y0, Y0
	XORL        BX, DX
	ADDL        R12, AX
	XORL        SI, DX
	ADDL        200(R15), DI
	LEAL        (DI)(DX*1), DI
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	VPSHUFB     Y10, Y0, Y12
	XORL        CX, AX
	ADDL        R12, DI
	XORL        BX, AX
	ADDL        204(R15), SI
	LEAL        (SI)(AX*1), SI
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	XORL        DX, DI
	ADDL        R12, SI
	XORL        CX, DI
	ADDL        224(R15), BX
	LEAL        (BX)(DI*1), BX
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	VPADDD      (R8), Y12, Y0
	XORL        AX, SI
	ADDL        R12, BX
	XORL        DX, SI
	ADDL        228(R15), CX
	LEAL        (CX)(SI*1), CX
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	XORL        DI, BX
	ADDL        R12, CX
	XORL        AX, BX
	ADDL        232(R15), DX
	LEAL        (DX)(BX*1), DX
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	XORL        SI, CX
	ADDL        R12, DX
	XORL        DI, CX
	ADDL        236(R15), AX
	LEAL        (AX)(CX*1), AX
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	VMOVDQU     Y0, 96(R14)
	XORL        BX, DX
	ADDL        R12, AX
	XORL        SI, DX
	ADDL        256(R15), DI
	LEAL        (DI)(DX*1), DI
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	VPALIGNR    $0x08, Y15, Y14, Y8
	VPSRLDQ     $0x04, Y12, Y0
	XORL        CX, AX
	ADDL        R12, DI
	XORL        BX, AX
	ADDL        260(R15), SI
	LEAL        (SI)(AX*1), SI
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	VPXOR       Y13, Y8, Y8
	VPXOR       Y15, Y0, Y0
	XORL        DX, DI
	ADDL        R12, SI
	XORL        CX, DI
	ADDL        264(R15), BX
	LEAL        (BX)(DI*1), BX
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	VPXOR       Y0, Y8, Y8
	VPSLLDQ     $0x0c, Y8, Y9
	XORL        AX, SI
	ADDL        R12, BX
	XORL        DX, SI
	ADDL        268(R15), CX
	LEAL        (CX)(SI*1), CX
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	VPSLLD      $0x01, Y8, Y0
	VPSRLD      $0x1f, Y8, Y8
	XORL        DI, BX
	ADDL        R12, CX
	XORL        AX, BX
	ADDL        288(R15), DX
	LEAL        (DX)(BX*1), DX
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	VPOR        Y8, Y0, Y0
	VPSLLD      $0x02, Y9, Y8
	XORL        SI, CX
	ADDL        R12, DX
	XORL        DI, CX
	ADDL        292(R15), AX
	LEAL        (AX)(CX*1), AX
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	VPSRLD      $0x1e, Y9, Y9
	VPXOR       Y8, Y0, Y0
	XORL        BX, DX
	ADDL        R12, AX
	XORL        SI, DX
	ADDL        296(R15), DI
	LEAL        (DI)(DX*1), DI
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	XORL        CX, AX
	ADDL        R12, DI
	XORL        BX, AX
	ADDL        300(R15), SI
	VPXOR       Y9, Y0, Y8
	VPADDD      (R8), Y8, Y0
	VMOVDQU     Y0, 128(R14)
	LEAL        (SI)(AX*1), SI
	MOVL        DX, BP
	ORL         DI, BP
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	ANDL        CX, BP
	ANDL        DX, DI
	ORL         BP, DI
	ADDL        R12, SI
	ADDL        320(R15), BX
	VPALIGNR    $0x08, Y14, Y13, Y7
	VPSRLDQ     $0x04, Y8, Y0
	LEAL        (BX)(DI*1), BX
	MOVL        AX, BP
	ORL         SI, BP
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	ANDL        DX, BP
	ANDL        AX, SI
	ORL         BP, SI
	ADDL        R12, BX
	ADDL        324(R15), CX
	VPXOR       Y12, Y7, Y7
	VPXOR       Y14, Y0, Y0
	LEAL        (CX)(SI*1), CX
	MOVL        DI, BP
	ORL         BX, BP
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	ANDL        AX, BP
	ANDL        DI, BX
	ORL         BP, BX
	ADDL        R12, CX
	ADDL        328(R15), DX
	VPXOR       Y0, Y7, Y7
	VPSLLDQ     $0x0c, Y7, Y9
	LEAL        (DX)(BX*1), DX
	MOVL        SI, BP
	ORL         CX, BP
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	ANDL        DI, BP
	ANDL        SI, CX
	ORL         BP, CX
	ADDL        R12, DX
	ADDL        332(R15), AX
	VPSLLD      $0x01, Y7, Y0
	VPSRLD      $0x1f, Y7, Y7
	LEAL        (AX)(CX*1), AX
	MOVL        BX, BP
	ORL         DX, BP
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	ANDL        SI, BP
	ANDL        BX, DX
	ORL         BP, DX
	ADDL        R12, AX
	ADDL        352(R15), DI
	VPOR        Y7, Y0, Y0
	VPSLLD      $0x02, Y9, Y7
	LEAL        (DI)(DX*1), DI
	MOVL        CX, BP
	ORL         AX, BP
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	ANDL        BX, BP
	ANDL        CX, AX
	ORL         BP, AX
	ADDL        R12, DI
	ADDL        356(R15), SI
	VPSRLD      $0x1e, Y9, Y9
	VPXOR       Y7, Y0, Y0
	LEAL        (SI)(AX*1), SI
	MOVL        DX, BP
	ORL         DI, BP
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	ANDL        CX, BP
	ANDL        DX, DI
	ORL         BP, DI
	ADDL        R12, SI
	ADDL        360(R15), BX
	LEAL        (BX)(DI*1), BX
	MOVL        AX, BP
	ORL         SI, BP
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	ANDL        DX, BP
	ANDL        AX, SI
	ORL         BP, SI
	ADDL        R12, BX
	ADDL        364(R15), CX
	VPXOR       Y9, Y0, Y7
	VPADDD      32(R8), Y7, Y0
	VMOVDQU     Y0, 160(R14)
	LEAL        (CX)(SI*1), CX
	MOVL        DI, BP
	ORL         BX, BP
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	ANDL        AX, BP
	ANDL        DI, BX
	ORL         BP, BX
	ADDL        R12, CX
	ADDL        384(R15), DX
	VPALIGNR    $0x08, Y13, Y12, Y5
	VPSRLDQ     $0x04, Y7, Y0
	LEAL        (DX)(BX*1), DX
	MOVL        SI, BP
	ORL         CX, BP
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	ANDL        DI, BP
	ANDL        SI, CX
	ORL         BP, CX
	ADDL        R12, DX
	ADDL        388(R15), AX
	VPXOR       Y8, Y5, Y5
	VPXOR       Y13, Y0, Y0
	LEAL        (AX)(CX*1), AX
	MOVL        BX, BP
	ORL         DX, BP
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	ANDL        SI, BP
	ANDL        BX, DX
	ORL         BP, DX
	ADDL        R12, AX
	ADDL        392(R15), DI
	VPXOR       Y0, Y5, Y5
	VPSLLDQ     $0x0c, Y5, Y9
	LEAL        (DI)(DX*1), DI
	MOVL        CX, BP
	ORL         AX, BP
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	ANDL        BX, BP
	ANDL        CX, AX
	ORL         BP, AX
	ADDL        R12, DI
	ADDL        396(R15), SI
	VPSLLD      $0x01, Y5, Y0
	VPSRLD      $0x1f, Y5, Y5
	LEAL        (SI)(AX*1), SI
	MOVL        DX, BP
	ORL         DI, BP
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	ANDL        CX, BP
	ANDL        DX, DI
	ORL         BP, DI
	ADDL        R12, SI
	ADDL        416(R15), BX
	VPOR        Y5, Y0, Y0
	VPSLLD      $0x02, Y9, Y5
	LEAL        (BX)(DI*1), BX
	MOVL        AX, BP
	ORL         SI, BP
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	ANDL        DX, BP
	ANDL        AX, SI
	ORL         BP, SI
	ADDL        R12, BX
	ADDL        420(R15), CX
	VPSRLD      $0x1e, Y9, Y9
	VPXOR       Y5, Y0, Y0
	LEAL        (CX)(SI*1), CX
	MOVL        DI, BP
	ORL         BX, BP
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	ANDL        AX, BP
	ANDL        DI, BX
	ORL         BP, BX
	ADDL        R12, CX
	ADDL        424(R15), DX
	LEAL        (DX)(BX*1), DX
	MOVL        SI, BP
	ORL         CX, BP
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	ANDL        DI, BP
	ANDL        SI, CX
	ORL         BP, CX
	ADDL        R12, DX
	ADDL        428(R15), AX
	VPXOR       Y9, Y0, Y5
	VPADDD      32(R8), Y5, Y0
	VMOVDQU     Y0, 192(R14)
	LEAL        (AX)(CX*1), AX
	MOVL        BX, BP
	ORL         DX, BP
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	ANDL        SI, BP
	ANDL        BX, DX
	ORL         BP, DX
	ADDL        R12, AX
	ADDL        448(R15), DI
	VPALIGNR    $0x08, Y12, Y8, Y3
	VPSRLDQ     $0x04, Y5, Y0
	LEAL        (DI)(DX*1), DI
	MOVL        CX, BP
	ORL         AX, BP
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	ANDL        BX, BP
	ANDL        CX, AX
	ORL         BP, AX
	ADDL        R12, DI
	ADDL        452(R15), SI
	VPXOR       Y7, Y3, Y3
	VPXOR       Y12, Y0, Y0
	LEAL        (SI)(AX*1), SI
	MOVL        DX, BP
	ORL         DI, BP
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	ANDL        CX, BP
	ANDL        DX, DI
	ORL         BP, DI
	ADDL        R12, SI
	ADDL        456(R15), BX
	VPXOR       Y0, Y3, Y3
	VPSLLDQ     $0x0c, Y3, Y9
	LEAL        (BX)(DI*1), BX
	MOVL        AX, BP
	ORL         SI, BP
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	ANDL        DX, BP
	ANDL        AX, SI
	ORL         BP, SI
	ADDL        R12, BX
	ADDL        460(R15), CX
	LEAL        (CX)(SI*1), CX
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	VPSLLD      $0x01, Y3, Y0
	VPSRLD      $0x1f, Y3, Y3
	XORL        DI, BX
	ADDL        R12, CX
	XORL        AX, BX
	ADDQ        $0x80, R10
	CMPQ        R10, R11
	CMOVQCC     R8, R10
	ADDL        480(R15), DX
	LEAL        (DX)(BX*1), DX
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	VPOR        Y3, Y0, Y0
	VPSLLD      $0x02, Y9, Y3
	XORL        SI, CX
	ADDL        R12, DX
	XORL        DI, CX
	ADDL        484(R15), AX
	LEAL        (AX)(CX*1), AX
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	VPSRLD      $0x1e, Y9, Y9
	VPXOR       Y3, Y0, Y0
	XORL        BX, DX
	ADDL        R12, AX
	XORL        SI, DX
	ADDL        488(R15), DI
	LEAL        (DI)(DX*1), DI
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	XORL        CX, AX
	ADDL        R12, DI
	XORL        BX, AX
	ADDL        492(R15), SI
	LEAL        (SI)(AX*1), SI
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	VPXOR       Y9, Y0, Y3
	VPADDD      32(R8), Y3, Y0
	VMOVDQU     Y0, 224(R14)
	XORL        DX, DI
	ADDL        R12, SI
	XORL        CX, DI
	ADDL        512(R15), BX
	LEAL        (BX)(DI*1), BX
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	VPALIGNR    $0x08, Y5, Y3, Y0
	XORL        AX, SI
	ADDL        R12, BX
	XORL        DX, SI
	ADDL        516(R15), CX
	LEAL        (CX)(SI*1), CX
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	VPXOR       Y14, Y15, Y15
	XORL        DI, BX
	ADDL        R12, CX
	XORL        AX, BX
	ADDL        520(R15), DX
	LEAL        (DX)(BX*1), DX
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	VPXOR       Y8, Y0, Y0
	XORL        SI, CX
	ADDL        R12, DX
	XORL        DI, CX
	ADDL        524(R15), AX
	LEAL        (AX)(CX*1), AX
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	VPXOR       Y0, Y15, Y15
	XORL        BX, DX
	ADDL        R12, AX
	XORL        SI, DX
	ADDL        544(R15), DI
	LEAL        (DI)(DX*1), DI
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	VPSLLD      $0x02, Y15, Y0
	XORL        CX, AX
	ADDL        R12, DI
	XORL        BX, AX
	ADDL        548(R15), SI
	LEAL        (SI)(AX*1), SI
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	VPSRLD      $0x1e, Y15, Y15
	VPOR        Y15, Y0, Y15
	XORL        DX, DI
	ADDL        R12, SI
	XORL        CX, DI
	ADDL        552(R15), BX
	LEAL        (BX)(DI*1), BX
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	XORL        AX, SI
	ADDL        R12, BX
	XORL        DX, SI
	ADDL        556(R15), CX
	LEAL        (CX)(SI*1), CX
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	VPADDD      32(R8), Y15, Y0
	VMOVDQU     Y0, 256(R14)
	XORL        DI, BX
	ADDL        R12, CX
	XORL        AX, BX
	ADDL        576(R15), DX
	LEAL        (DX)(BX*1), DX
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	VPALIGNR    $0x08, Y3, Y15, Y0
	XORL        SI, CX
	ADDL        R12, DX
	XORL        DI, CX
	ADDL        580(R15), AX
	LEAL        (AX)(CX*1), AX
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	VPXOR       Y13, Y14, Y14
	XORL        BX, DX
	ADDL        R12, AX
	XORL        SI, DX
	ADDL        584(R15), DI
	LEAL        (DI)(DX*1), DI
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	VPXOR       Y7, Y0, Y0
	XORL        CX, AX
	ADDL        R12, DI
	XORL        BX, AX
	ADDL        588(R15), SI
	LEAL        (SI)(AX*1), SI
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	VPXOR       Y0, Y14, Y14
	XORL        DX, DI
	ADDL        R12, SI
	XORL        CX, DI
	ADDL        608(R15), BX
	LEAL        (BX)(DI*1), BX
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	VPSLLD      $0x02, Y14, Y0
	XORL        AX, SI
	ADDL        R12, BX
	XORL        DX, SI
	ADDL        612(R15), CX
	LEAL        (CX)(SI*1), CX
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	VPSRLD      $0x1e, Y14, Y14
	VPOR        Y14, Y0, Y14
	XORL        DI, BX
	ADDL        R12, CX
	XORL        AX, BX
	ADDL        616(R15), DX
	LEAL        (DX)(BX*1), DX
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	XORL        SI, CX
	ADDL        R12, DX
	XORL        DI, CX
	ADDL        620(R15), AX
	LEAL        (AX)(CX*1), AX
	RORXL       $0x1b, DX, R12
	VPADDD      32(R8), Y14, Y0
	VMOVDQU     Y0, 288(R14)
	ADDL        R12, AX
	ADDL        (R9), AX
	MOVL        AX, (R9)
	ADDL        4(R9), DX
	MOVL        DX, 4(R9)
	ADDL        8(R9), BX
	MOVL        BX, 8(R9)
	ADDL        12(R9), SI
	MOVL        SI, 12(R9)
	ADDL        16(R9), DI
	MOVL        DI, 16(R9)
	CMPQ        R10, R8
	JE          loop
	MOVL        DX, CX
	MOVL        CX, DX
	RORXL       $0x02, CX, CX
	ANDNL       SI, DX, BP
	ANDL        BX, DX
	XORL        BP, DX
	ADDL        16(R15), DI
	ANDNL       BX, AX, BP
	LEAL        (DI)(DX*1), DI
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	VPALIGNR    $0x08, Y15, Y14, Y0
	ANDL        CX, AX
	XORL        BP, AX
	LEAL        (DI)(R12*1), DI
	ADDL        20(R15), SI
	ANDNL       CX, DI, BP
	LEAL        (SI)(AX*1), SI
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	VPXOR       Y12, Y13, Y13
	ANDL        DX, DI
	XORL        BP, DI
	LEAL        (SI)(R12*1), SI
	ADDL        24(R15), BX
	ANDNL       DX, SI, BP
	LEAL        (BX)(DI*1), BX
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	VPXOR       Y5, Y0, Y0
	ANDL        AX, SI
	XORL        BP, SI
	LEAL        (BX)(R12*1), BX
	ADDL        28(R15), CX
	ANDNL       AX, BX, BP
	LEAL        (CX)(SI*1), CX
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	VPXOR       Y0, Y13, Y13
	ANDL        DI, BX
	XORL        BP, BX
	LEAL        (CX)(R12*1), CX
	ADDL        48(R15), DX
	ANDNL       DI, CX, BP
	LEAL        (DX)(BX*1), DX
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	VPSLLD      $0x02, Y13, Y0
	ANDL        SI, CX
	XORL        BP, CX
	LEAL        (DX)(R12*1), DX
	ADDL        52(R15), AX
	ANDNL       SI, DX, BP
	LEAL        (AX)(CX*1), AX
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	VPSRLD      $0x1e, Y13, Y13
	VPOR        Y13, Y0, Y13
	ANDL        BX, DX
	XORL        BP, DX
	LEAL        (AX)(R12*1), AX
	ADDL        56(R15), DI
	ANDNL       BX, AX, BP
	LEAL        (DI)(DX*1), DI
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	ANDL        CX, AX
	XORL        BP, AX
	LEAL        (DI)(R12*1), DI
	ADDL        60(R15), SI
	ANDNL       CX, DI, BP
	LEAL        (SI)(AX*1), SI
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	VPADDD      64(R8), Y13, Y0
	VMOVDQU     Y0, 320(R14)
	ANDL        DX, DI
	XORL        BP, DI
	LEAL        (SI)(R12*1), SI
	ADDL        80(R15), BX
	ANDNL       DX, SI, BP
	LEAL        (BX)(DI*1), BX
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	VPALIGNR    $0x08, Y14, Y13, Y0
	ANDL        AX, SI
	XORL        BP, SI
	LEAL        (BX)(R12*1), BX
	ADDL        84(R15), CX
	ANDNL       AX, BX, BP
	LEAL        (CX)(SI*1), CX
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	VPXOR       Y8, Y12, Y12
	ANDL        DI, BX
	XORL        BP, BX
	LEAL        (CX)(R12*1), CX
	ADDL        88(R15), DX
	ANDNL       DI, CX, BP
	LEAL        (DX)(BX*1), DX
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	VPXOR       Y3, Y0, Y0
	ANDL        SI, CX
	XORL        BP, CX
	LEAL        (DX)(R12*1), DX
	ADDL        92(R15), AX
	ANDNL       SI, DX, BP
	LEAL        (AX)(CX*1), AX
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	VPXOR       Y0, Y12, Y12
	ANDL        BX, DX
	XORL        BP, DX
	LEAL        (AX)(R12*1), AX
	ADDL        112(R15), DI
	ANDNL       BX, AX, BP
	LEAL        (DI)(DX*1), DI
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	VPSLLD      $0x02, Y12, Y0
	ANDL        CX, AX
	XORL        BP, AX
	LEAL        (DI)(R12*1), DI
	ADDL        116(R15), SI
	ANDNL       CX, DI, BP
	LEAL        (SI)(AX*1), SI
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	VPSRLD      $0x1e, Y12, Y12
	VPOR        Y12, Y0, Y12
	ANDL        DX, DI
	XORL        BP, DI
	LEAL        (SI)(R12*1), SI
	ADDL        120(R15), BX
	ANDNL       DX, SI, BP
	LEAL        (BX)(DI*1), BX
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	ANDL        AX, SI
	XORL        BP, SI
	LEAL        (BX)(R12*1), BX
	ADDL        124(R15), CX
	ANDNL       AX, BX, BP
	LEAL        (CX)(SI*1), CX
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	VPADDD      64(R8), Y12, Y0
	VMOVDQU     Y0, 352(R14)
	ANDL        DI, BX
	XORL        BP, BX
	LEAL        (CX)(R12*1), CX
	ADDL        144(R15), DX
	ANDNL       DI, CX, BP
	LEAL        (DX)(BX*1), DX
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	VPALIGNR    $0x08, Y13, Y12, Y0
	ANDL        SI, CX
	XORL        BP, CX
	LEAL        (DX)(R12*1), DX
	ADDL        148(R15), AX
	ANDNL       SI, DX, BP
	LEAL        (AX)(CX*1), AX
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	VPXOR       Y7, Y8, Y8
	ANDL        BX, DX
	XORL        BP, DX
	LEAL        (AX)(R12*1), AX
	ADDL        152(R15), DI
	ANDNL       BX, AX, BP
	LEAL        (DI)(DX*1), DI
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	VPXOR       Y15, Y0, Y0
	ANDL        CX, AX
	XORL        BP, AX
	LEAL        (DI)(R12*1), DI
	ADDL        156(R15), SI
	LEAL        (SI)(AX*1), SI
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	VPXOR       Y0, Y8, Y8
	XORL        DX, DI
	ADDL        R12, SI
	XORL        CX, DI
	ADDL        176(R15), BX
	LEAL        (BX)(DI*1), BX
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	VPSLLD      $0x02, Y8, Y0
	XORL        AX, SI
	ADDL        R12, BX
	XORL        DX, SI
	ADDL        180(R15), CX
	LEAL        (CX)(SI*1), CX
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	VPSRLD      $0x1e, Y8, Y8
	VPOR        Y8, Y0, Y8
	XORL        DI, BX
	ADDL        R12, CX
	XORL        AX, BX
	ADDL        184(R15), DX
	LEAL        (DX)(BX*1), DX
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	XORL        SI, CX
	ADDL        R12, DX
	XORL        DI, CX
	ADDL        188(R15), AX
	LEAL        (AX)(CX*1), AX
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	VPADDD      64(R8), Y8, Y0
	VMOVDQU     Y0, 384(R14)
	XORL        BX, DX
	ADDL        R12, AX
	XORL        SI, DX
	ADDL        208(R15), DI
	LEAL        (DI)(DX*1), DI
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	VPALIGNR    $0x08, Y12, Y8, Y0
	XORL        CX, AX
	ADDL        R12, DI
	XORL        BX, AX
	ADDL        212(R15), SI
	LEAL        (SI)(AX*1), SI
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	VPXOR       Y5, Y7, Y7
	XORL        DX, DI
	ADDL        R12, SI
	XORL        CX, DI
	ADDL        216(R15), BX
	LEAL        (BX)(DI*1), BX
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	VPXOR       Y14, Y0, Y0
	XORL        AX, SI
	ADDL        R12, BX
	XORL        DX, SI
	ADDL        220(R15), CX
	LEAL        (CX)(SI*1), CX
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	VPXOR       Y0, Y7, Y7
	XORL        DI, BX
	ADDL        R12, CX
	XORL        AX, BX
	ADDL        240(R15), DX
	LEAL        (DX)(BX*1), DX
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	VPSLLD      $0x02, Y7, Y0
	XORL        SI, CX
	ADDL        R12, DX
	XORL        DI, CX
	ADDL        244(R15), AX
	LEAL        (AX)(CX*1), AX
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	VPSRLD      $0x1e, Y7, Y7
	VPOR        Y7, Y0, Y7
	XORL        BX, DX
	ADDL        R12, AX
	XORL        SI, DX
	ADDL        248(R15), DI
	LEAL        (DI)(DX*1), DI
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	XORL        CX, AX
	ADDL        R12, DI
	XORL        BX, AX
	ADDL        252(R15), SI
	LEAL        (SI)(AX*1), SI
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	VPADDD      64(R8), Y7, Y0
	VMOVDQU     Y0, 416(R14)
	XORL        DX, DI
	ADDL        R12, SI
	XORL        CX, DI
	ADDL        272(R15), BX
	LEAL        (BX)(DI*1), BX
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	VPALIGNR    $0x08, Y8, Y7, Y0
	XORL        AX, SI
	ADDL        R12, BX
	XORL        DX, SI
	ADDL        276(R15), CX
	LEAL        (CX)(SI*1), CX
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	VPXOR       Y3, Y5, Y5
	XORL        DI, BX
	ADDL        R12, CX
	XORL        AX, BX
	ADDL        280(R15), DX
	LEAL        (DX)(BX*1), DX
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	VPXOR       Y13, Y0, Y0
	XORL        SI, CX
	ADDL        R12, DX
	XORL        DI, CX
	ADDL        284(R15), AX
	LEAL        (AX)(CX*1), AX
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	VPXOR       Y0, Y5, Y5
	XORL        BX, DX
	ADDL        R12, AX
	XORL        SI, DX
	ADDL        304(R15), DI
	LEAL        (DI)(DX*1), DI
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	VPSLLD      $0x02, Y5, Y0
	XORL        CX, AX
	ADDL        R12, DI
	XORL        BX, AX
	ADDL        308(R15), SI
	LEAL        (SI)(AX*1), SI
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	VPSRLD      $0x1e, Y5, Y5
	VPOR        Y5, Y0, Y5
	XORL        DX, DI
	ADDL        R12, SI
	XORL        CX, DI
	ADDL        312(R15), BX
	LEAL        (BX)(DI*1), BX
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	XORL        AX, SI
	ADDL        R12, BX
	XORL        DX, SI
	ADDL        316(R15), CX
	VPADDD      64(R8), Y5, Y0
	VMOVDQU     Y0, 448(R14)
	LEAL        (CX)(SI*1), CX
	MOVL        DI, BP
	ORL         BX, BP
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	ANDL        AX, BP
	ANDL        DI, BX
	ORL         BP, BX
	ADDL        R12, CX
	ADDL        336(R15), DX
	VPALIGNR    $0x08, Y7, Y5, Y0
	LEAL        (DX)(BX*1), DX
	MOVL        SI, BP
	ORL         CX, BP
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	ANDL        DI, BP
	ANDL        SI, CX
	ORL         BP, CX
	ADDL        R12, DX
	ADDL        340(R15), AX
	VPXOR       Y15, Y3, Y3
	LEAL        (AX)(CX*1), AX
	MOVL        BX, BP
	ORL         DX, BP
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	ANDL        SI, BP
	ANDL        BX, DX
	ORL         BP, DX
	ADDL        R12, AX
	ADDL        344(R15), DI
	VPXOR       Y12, Y0, Y0
	LEAL        (DI)(DX*1), DI
	MOVL        CX, BP
	ORL         AX, BP
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	ANDL        BX, BP
	ANDL        CX, AX
	ORL         BP, AX
	ADDL        R12, DI
	ADDL        348(R15), SI
	VPXOR       Y0, Y3, Y3
	LEAL        (SI)(AX*1), SI
	MOVL        DX, BP
	ORL         DI, BP
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	ANDL        CX, BP
	ANDL        DX, DI
	ORL         BP, DI
	ADDL        R12, SI
	ADDL        368(R15), BX
	VPSLLD      $0x02, Y3, Y0
	LEAL        (BX)(DI*1), BX
	MOVL        AX, BP
	ORL         SI, BP
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	ANDL        DX, BP
	ANDL        AX, SI
	ORL         BP, SI
	ADDL        R12, BX
	ADDL        372(R15), CX
	VPSRLD      $0x1e, Y3, Y3
	VPOR        Y3, Y0, Y3
	LEAL        (CX)(SI*1), CX
	MOVL        DI, BP
	ORL         BX, BP
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	ANDL        AX, BP
	ANDL        DI, BX
	ORL         BP, BX
	ADDL        R12, CX
	ADDL        376(R15), DX
	LEAL        (DX)(BX*1), DX
	MOVL        SI, BP
	ORL         CX, BP
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	ANDL        DI, BP
	ANDL        SI, CX
	ORL         BP, CX
	ADDL        R12, DX
	ADDL        380(R15), AX
	VPADDD      96(R8), Y3, Y0
	VMOVDQU     Y0, 480(R14)
	LEAL        (AX)(CX*1), AX
	MOVL        BX, BP
	ORL         DX, BP
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	ANDL        SI, BP
	ANDL        BX, DX
	ORL         BP, DX
	ADDL        R12, AX
	ADDL        400(R15), DI
	VPALIGNR    $0x08, Y5, Y3, Y0
	LEAL        (DI)(DX*1), DI
	MOVL        CX, BP
	ORL         AX, BP
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	ANDL        BX, BP
	ANDL        CX, AX
	ORL         BP, AX
	ADDL        R12, DI
	ADDL        404(R15), SI
	VPXOR       Y14, Y15, Y15
	LEAL        (SI)(AX*1), SI
	MOVL        DX, BP
	ORL         DI, BP
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	ANDL        CX, BP
	ANDL        DX, DI
	ORL         BP, DI
	ADDL        R12, SI
	ADDL        408(R15), BX
	VPXOR       Y8, Y0, Y0
	LEAL        (BX)(DI*1), BX
	MOVL        AX, BP
	ORL         SI, BP
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	ANDL        DX, BP
	ANDL        AX, SI
	ORL         BP, SI
	ADDL        R12, BX
	ADDL        412(R15), CX
	VPXOR       Y0, Y15, Y15
	LEAL        (CX)(SI*1), CX
	MOVL        DI, BP
	ORL         BX, BP
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	ANDL        AX, BP
	ANDL        DI, BX
	ORL         BP, BX
	ADDL        R12, CX
	ADDL        432(R15), DX
	VPSLLD      $0x02, Y15, Y0
	LEAL        (DX)(BX*1), DX
	MOVL        SI, BP
	ORL         CX, BP
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	ANDL        DI, BP
	ANDL        SI, CX
	ORL         BP, CX
	ADDL        R12, DX
	ADDL        436(R15), AX
	VPSRLD      $0x1e, Y15, Y15
	VPOR        Y15, Y0, Y15
	LEAL        (AX)(CX*1), AX
	MOVL        BX, BP
	ORL         DX, BP
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	ANDL        SI, BP
	ANDL        BX, DX
	ORL         BP, DX
	ADDL        R12, AX
	ADDL        440(R15), DI
	LEAL        (DI)(DX*1), DI
	MOVL        CX, BP
	ORL         AX, BP
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	ANDL        BX, BP
	ANDL        CX, AX
	ORL         BP, AX
	ADDL        R12, DI
	ADDL        444(R15), SI
	VPADDD      96(R8), Y15, Y0
	VMOVDQU     Y0, 512(R14)
	LEAL        (SI)(AX*1), SI
	MOVL        DX, BP
	ORL         DI, BP
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	ANDL        CX, BP
	ANDL        DX, DI
	ORL         BP, DI
	ADDL        R12, SI
	ADDL        464(R15), BX
	VPALIGNR    $0x08, Y3, Y15, Y0
	LEAL        (BX)(DI*1), BX
	MOVL        AX, BP
	ORL         SI, BP
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	ANDL        DX, BP
	ANDL        AX, SI
	ORL         BP, SI
	ADDL        R12, BX
	ADDL        468(R15), CX
	VPXOR       Y13, Y14, Y14
	LEAL        (CX)(SI*1), CX
	MOVL        DI, BP
	ORL         BX, BP
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	ANDL        AX, BP
	ANDL        DI, BX
	ORL         BP, BX
	ADDL        R12, CX
	ADDL        472(R15), DX
	VPXOR       Y7, Y0, Y0
	LEAL        (DX)(BX*1), DX
	MOVL        SI, BP
	ORL         CX, BP
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	ANDL        DI, BP
	ANDL        SI, CX
	ORL         BP, CX
	ADDL        R12, DX
	ADDL        476(R15), AX
	LEAL        (AX)(CX*1), AX
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	VPXOR       Y0, Y14, Y14
	XORL        BX, DX
	ADDL        R12, AX
	XORL        SI, DX
	ADDQ        $0x80, R13
	CMPQ        R13, R11
	CMOVQCC     R8, R10
	ADDL        496(R15), DI
	LEAL        (DI)(DX*1), DI
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	VPSLLD      $0x02, Y14, Y0
	XORL        CX, AX
	ADDL        R12, DI
	XORL        BX, AX
	ADDL        500(R15), SI
	LEAL        (SI)(AX*1), SI
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	VPSRLD      $0x1e, Y14, Y14
	VPOR        Y14, Y0, Y14
	XORL        DX, DI
	ADDL        R12, SI
	XORL        CX, DI
	ADDL        504(R15), BX
	LEAL        (BX)(DI*1), BX
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	XORL        AX, SI
	ADDL        R12, BX
	XORL        DX, SI
	ADDL        508(R15), CX
	LEAL        (CX)(SI*1), CX
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	VPADDD      96(R8), Y14, Y0
	VMOVDQU     Y0, 544(R14)
	XORL        DI, BX
	ADDL        R12, CX
	XORL        AX, BX
	ADDL        528(R15), DX
	LEAL        (DX)(BX*1), DX
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	VPALIGNR    $0x08, Y15, Y14, Y0
	XORL        SI, CX
	ADDL        R12, DX
	XORL        DI, CX
	ADDL        532(R15), AX
	LEAL        (AX)(CX*1), AX
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	VPXOR       Y12, Y13, Y13
	XORL        BX, DX
	ADDL        R12, AX
	XORL        SI, DX
	ADDL        536(R15), DI
	LEAL        (DI)(DX*1), DI
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	VPXOR       Y5, Y0, Y0
	XORL        CX, AX
	ADDL        R12, DI
	XORL        BX, AX
	ADDL        540(R15), SI
	LEAL        (SI)(AX*1), SI
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	VPXOR       Y0, Y13, Y13
	XORL        DX, DI
	ADDL        R12, SI
	XORL        CX, DI
	ADDL        560(R15), BX
	LEAL        (BX)(DI*1), BX
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	VPSLLD      $0x02, Y13, Y0
	XORL        AX, SI
	ADDL        R12, BX
	XORL        DX, SI
	ADDL        564(R15), CX
	LEAL        (CX)(SI*1), CX
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	VPSRLD      $0x1e, Y13, Y13
	VPOR        Y13, Y0, Y13
	XORL        DI, BX
	ADDL        R12, CX
	XORL        AX, BX
	ADDL        568(R15), DX
	LEAL        (DX)(BX*1), DX
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	XORL        SI, CX
	ADDL        R12, DX
	XORL        DI, CX
	ADDL        572(R15), AX
	LEAL        (AX)(CX*1), AX
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	VPADDD      96(R8), Y13, Y0
	VMOVDQU     Y0, 576(R14)
	XORL        BX, DX
	ADDL        R12, AX
	XORL        SI, DX
	ADDL        592(R15), DI
	LEAL        (DI)(DX*1), DI
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	VPALIGNR    $0x08, Y14, Y13, Y0
	XORL        CX, AX
	ADDL        R12, DI
	XORL        BX, AX
	ADDL        596(R15), SI
	LEAL        (SI)(AX*1), SI
	RORXL       $0x1b, DI, R12
	RORXL       $0x02, DI, AX
	VPXOR       Y8, Y12, Y12
	XORL        DX, DI
	ADDL        R12, SI
	XORL        CX, DI
	ADDL        600(R15), BX
	LEAL        (BX)(DI*1), BX
	RORXL       $0x1b, SI, R12
	RORXL       $0x02, SI, DI
	VPXOR       Y3, Y0, Y0
	XORL        AX, SI
	ADDL        R12, BX
	XORL        DX, SI
	ADDL        604(R15), CX
	LEAL        (CX)(SI*1), CX
	RORXL       $0x1b, BX, R12
	RORXL       $0x02, BX, SI
	VPXOR       Y0, Y12, Y12
	XORL        DI, BX
	ADDL        R12, CX
	XORL        AX, BX
	ADDL        624(R15), DX
	LEAL        (DX)(BX*1), DX
	RORXL       $0x1b, CX, R12
	RORXL       $0x02, CX, BX
	VPSLLD      $0x02, Y12, Y0
	XORL        SI, CX
	ADDL        R12, DX
	XORL        DI, CX
	ADDL        628(R15), AX
	LEAL        (AX)(CX*1), AX
	RORXL       $0x1b, DX, R12
	RORXL       $0x02, DX, CX
	VPSRLD      $0x1e, Y12, Y12
	VPOR        Y12, Y0, Y12
	XORL        BX, DX
	ADDL        R12, AX
	XORL        SI, DX
	ADDL        632(R15), DI
	LEAL        (DI)(DX*1), DI
	RORXL       $0x1b, AX, R12
	RORXL       $0x02, AX, DX
	XORL        CX, AX
	ADDL        R12, DI
	XORL        BX, AX
	ADDL        636(R15), SI
	LEAL        (SI)(AX*1), SI
	RORXL       $0x1b, DI, R12
	VPADDD      96(R8), Y12, Y0
	VMOVDQU     Y0, 608(R14)
	ADDL        R12, SI
	ADDL        (R9), SI
	MOVL        SI, (R9)
	ADDL        4(R9), DI
	MOVL        DI, 4(R9)
	ADDL        8(R9), DX
	MOVL        DX, 8(R9)
	ADDL        12(R9), CX
	MOVL        CX, 12(R9)
	ADDL        16(R9), BX
	MOVL        BX, 16(R9)
	MOVL        SI, R12
	MOVL        DI, SI
	MOVL        DX, DI
	MOVL        BX, DX
	MOVL        CX, AX
	MOVL        R12, CX
	XCHGQ       R15, R14
	JMP         loop

DATA K_XMM_AR<>+0(SB)/4, $0x5a827999
DATA K_XMM_AR<>+4(SB)/4, $0x5a827999
DATA K_XMM_AR<>+8(SB)/4, $0x5a827999
DATA K_XMM_AR<>+12(SB)/4, $0x5a827999
DATA K_XMM_AR<>+16(SB)/4, $0x5a827999
DATA K_XMM_AR<>+20(SB)/4, $0x5a827999
DATA K_XMM_AR<>+24(SB)/4, $0x5a827999
DATA K_XMM_AR<>+28(SB)/4, $0x5a827999
DATA K_XMM_AR<>+32(SB)/4, $0x6ed9eba1
DATA K_XMM_AR<>+36(SB)/4, $0x6ed9eba1
DATA K_XMM_AR<>+40(SB)/4, $0x6ed9eba1
DATA K_XMM_AR<>+44(SB)/4, $0x6ed9eba1
DATA K_XMM_AR<>+48(SB)/4, $0x6ed9eba1
DATA K_XMM_AR<>+52(SB)/4, $0x6ed9eba1
DATA K_XMM_AR<>+56(SB)/4, $0x6ed9eba1
DATA K_XMM_AR<>+60(SB)/4, $0x6ed9eba1
DATA K_XMM_AR<>+64(SB)/4, $0x8f1bbcdc
DATA K_XMM_AR<>+68(SB)/4, $0x8f1bbcdc
DATA K_XMM_AR<>+72(SB)/4, $0x8f1bbcdc
DATA K_XMM_AR<>+76(SB)/4, $0x8f1bbcdc
DATA K_XMM_AR<>+80(SB)/4, $0x8f1bbcdc
DATA K_XMM_AR<>+84(SB)/4, $0x8f1bbcdc
DATA K_XMM_AR<>+88(SB)/4, $0x8f1bbcdc
DATA K_XMM_AR<>+92(SB)/4, $0x8f1bbcdc
DATA K_XMM_AR<>+96(SB)/4, $0xca62c1d6
DATA K_XMM_AR<>+100(SB)/4, $0xca62c1d6
DATA K_XMM_AR<>+104(SB)/4, $0xca62c1d6
DATA K_XMM_AR<>+108(SB)/4, $0xca62c1d6
DATA K_XMM_AR<>+112(SB)/4, $0xca62c1d6
DATA K_XMM_AR<>+116(SB)/4, $0xca62c1d6
DATA K_XMM_AR<>+120(SB)/4, $0xca62c1d6
DATA K_XMM_AR<>+124(SB)/4, $0xca62c1d6
GLOBL K_XMM_AR<>(SB), RODATA, $128

DATA BSWAP_SHUFB_CTL<>+0(SB)/4, $0x00010203
DATA BSWAP_SHUFB_CTL<>+4(SB)/4, $0x04050607
DATA BSWAP_SHUFB_CTL<>+8(SB)/4, $0x08090a0b
DATA BSWAP_SHUFB_CTL<>+12(SB)/4, $0x0c0d0e0f
DATA BSWAP_SHUFB_CTL<>+16(SB)/4, $0x00010203
DATA BSWAP_SHUFB_CTL<>+20(SB)/4, $0x04050607
DATA BSWAP_SHUFB_CTL<>+24(SB)/4, $0x08090a0b
DATA BSWAP_SHUFB_CTL<>+28(SB)/4, $0x0c0d0e0f
GLOBL BSWAP_SHUFB_CTL<>(SB), RODATA, $32

// func blockSHANI(dig *digest, p []byte)
// Requires: AVX, SHA, SSE2, SSE4.1, SSSE3
TEXT ·blockSHANI(SB), $48-32
	MOVQ dig+0(FP), DI
	MOVQ p_base+8(FP), SI
	MOVQ p_len+16(FP), DX
	CMPQ DX, $0x00
	JEQ  done
	ADDQ SI, DX

	// Allocate space on the stack for saving ABCD and E0, and align it to 16 bytes
	LEAQ 15(SP), AX
	MOVQ $0x000000000000000f, CX
	NOTQ CX
	ANDQ CX, AX

	// Load initial hash state
	PINSRD  $0x03, 16(DI), X5
	VMOVDQU (DI), X0
	PAND    upper_mask<>+0(SB), X5
	PSHUFD  $0x1b, X0, X0
	VMOVDQA shuffle_mask<>+0(SB), X7

loop:
	// Save ABCD and E working values
	VMOVDQA X5, (AX)
	VMOVDQA X0, 16(AX)

	// Rounds 0-3
	VMOVDQU   (SI), X1
	PSHUFB    X7, X1
	PADDD     X1, X5
	VMOVDQA   X0, X6
	SHA1RNDS4 $0x00, X5, X0

	// Rounds 4-7
	VMOVDQU   16(SI), X2
	PSHUFB    X7, X2
	SHA1NEXTE X2, X6
	VMOVDQA   X0, X5
	SHA1RNDS4 $0x00, X6, X0
	SHA1MSG1  X2, X1

	// Rounds 8-11
	VMOVDQU   32(SI), X3
	PSHUFB    X7, X3
	SHA1NEXTE X3, X5
	VMOVDQA   X0, X6
	SHA1RNDS4 $0x00, X5, X0
	SHA1MSG1  X3, X2
	PXOR      X3, X1

	// Rounds 12-15
	VMOVDQU   48(SI), X4
	PSHUFB    X7, X4
	SHA1NEXTE X4, X6
	VMOVDQA   X0, X5
	SHA1MSG2  X4, X1
	SHA1RNDS4 $0x00, X6, X0
	SHA1MSG1  X4, X3
	PXOR      X4, X2

	// Rounds 16-19
	SHA1NEXTE X1, X5
	VMOVDQA   X0, X6
	SHA1MSG2  X1, X2
	SHA1RNDS4 $0x00, X5, X0
	SHA1MSG1  X1, X4
	PXOR      X1, X3

	// Rounds 20-23
	SHA1NEXTE X2, X6
	VMOVDQA   X0, X5
	SHA1MSG2  X2, X3
	SHA1RNDS4 $0x01, X6, X0
	SHA1MSG1  X2, X1
	PXOR      X2, X4

	// Rounds 24-27
	SHA1NEXTE X3, X5
	VMOVDQA   X0, X6
	SHA1MSG2  X3, X4
	SHA1RNDS4 $0x01, X5, X0
	SHA1MSG1  X3, X2
	PXOR      X3, X1

	// Rounds 28-31
	SHA1NEXTE X4, X6
	VMOVDQA   X0, X5
	SHA1MSG2  X4, X1
	SHA1RNDS4 $0x01, X6, X0
	SHA1MSG1  X4, X3
	PXOR      X4, X2

	// Rounds 32-35
	SHA1NEXTE X1, X5
	VMOVDQA   X0, X6
	SHA1MSG2  X1, X2
	SHA1RNDS4 $0x01, X5, X0
	SHA1MSG1  X1, X4
	PXOR      X1, X3

	// Rounds 36-39
	SHA1NEXTE X2, X6
	VMOVDQA   X0, X5
	SHA1MSG2  X2, X3
	SHA1RNDS4 $0x01, X6, X0
	SHA1MSG1  X2, X1
	PXOR      X2, X4

	// Rounds 40-43
	SHA1NEXTE X3, X5
	VMOVDQA   X0, X6
	SHA1MSG2  X3, X4
	SHA1RNDS4 $0x02, X5, X0
	SHA1MSG1  X3, X2
	PXOR      X3, X1

	// Rounds 44-47
	SHA1NEXTE X4, X6
	VMOVDQA   X0, X5
	SHA1MSG2  X4, X1
	SHA1RNDS4 $0x02, X6, X0
	SHA1MSG1  X4, X3
	PXOR      X4, X2

	// Rounds 48-51
	SHA1NEXTE X1, X5
	VMOVDQA   X0, X6
	SHA1MSG2  X1, X2
	SHA1RNDS4 $0x02, X5, X0
	SHA1MSG1  X1, X4
	PXOR      X1, X3

	// Rounds 52-55
	SHA1NEXTE X2, X6
	VMOVDQA   X0, X5
	SHA1MSG2  X2, X3
	SHA1RNDS4 $0x02, X6, X0
	SHA1MSG1  X2, X1
	PXOR      X2, X4

	// Rounds 56-59
	SHA1NEXTE X3, X5
	VMOVDQA   X0, X6
	SHA1MSG2  X3, X4
	SHA1RNDS4 $0x02, X5, X0
	SHA1MSG1  X3, X2
	PXOR      X3, X1

	// Rounds 60-63
	SHA1NEXTE X4, X6
	VMOVDQA   X0, X5
	SHA1MSG2  X4, X1
	SHA1RNDS4 $0x03, X6, X0
	SHA1MSG1  X4, X3
	PXOR      X4, X2

	// Rounds 64-67
	SHA1NEXTE X1, X5
	VMOVDQA   X0, X6
	SHA1MSG2  X1, X2
	SHA1RNDS4 $0x03, X5, X0
	SHA1MSG1  X1, X4
	PXOR      X1, X3

	// Rounds 68-71
	SHA1NEXTE X2, X6
	VMOVDQA   X0, X5
	SHA1MSG2  X2, X3
	SHA1RNDS4 $0x03, X6, X0
	PXOR      X2, X4

	// Rounds 72-75
	SHA1NEXTE X3, X5
	VMOVDQA   X0, X6
	SHA1MSG2  X3, X4
	SHA1RNDS4 $0x03, X5, X0

	// Rounds 76-79
	SHA1NEXTE X4, X6
	VMOVDQA   X0, X5
	SHA1RNDS4 $0x03, X6, X0

	// Add saved E and ABCD
	SHA1NEXTE (AX), X5
	PADDD     16(AX), X0

	// Check if we are done, if not return to the loop
	ADDQ $0x40, SI
	CMPQ SI, DX
	JNE  loop

	// Write the hash state back to digest
	PSHUFD  $0x1b, X0, X0
	VMOVDQU X0, (DI)
	PEXTRD  $0x03, X5, 16(DI)

done:
	RET

DATA upper_mask<>+0(SB)/8, $0x0000000000000000
DATA upper_mask<>+8(SB)/8, $0xffffffff00000000
GLOBL upper_mask<>(SB), RODATA, $16

DATA shuffle_mask<>+0(SB)/8, $0x08090a0b0c0d0e0f
DATA shuffle_mask<>+8(SB)/8, $0x0001020304050607
GLOBL shuffle_mask<>(SB), RODATA, $16

```

// === FILE: references/go/src/crypto/sha1/sha1block_arm.s ===
```text
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// ARM version of md5block.go

//go:build !purego

#include "textflag.h"

// SHA-1 block routine. See sha1block.go for Go equivalent.
//
// There are 80 rounds of 4 types:
//   - rounds 0-15 are type 1 and load data (ROUND1 macro).
//   - rounds 16-19 are type 1 and do not load data (ROUND1x macro).
//   - rounds 20-39 are type 2 and do not load data (ROUND2 macro).
//   - rounds 40-59 are type 3 and do not load data (ROUND3 macro).
//   - rounds 60-79 are type 4 and do not load data (ROUND4 macro).
//
// Each round loads or shuffles the data, then computes a per-round
// function of b, c, d, and then mixes the result into and rotates the
// five registers a, b, c, d, e holding the intermediate results.
//
// The register rotation is implemented by rotating the arguments to
// the round macros instead of by explicit move instructions.

// Register definitions
#define Rdata	R0	// Pointer to incoming data
#define Rconst	R1	// Current constant for SHA round
#define Ra	R2		// SHA-1 accumulator
#define Rb	R3		// SHA-1 accumulator
#define Rc	R4		// SHA-1 accumulator
#define Rd	R5		// SHA-1 accumulator
#define Re	R6		// SHA-1 accumulator
#define Rt0	R7		// Temporary
#define Rt1	R8		// Temporary
// r9, r10 are forbidden
// r11 is OK provided you check the assembler that no synthetic instructions use it
#define Rt2	R11		// Temporary
#define Rctr	R12	// loop counter
#define Rw	R14		// point to w buffer

// func block(dig *digest, p []byte)
// 0(FP) is *digest
// 4(FP) is p.array (struct Slice)
// 8(FP) is p.len
//12(FP) is p.cap
//
// Stack frame
#define p_end	end-4(SP)		// pointer to the end of data
#define p_data	data-8(SP)	// current data pointer (unused?)
#define w_buf	buf-(8+4*80)(SP)	//80 words temporary buffer w uint32[80]
#define saved	abcde-(8+4*80+4*5)(SP)	// saved sha1 registers a,b,c,d,e - these must be last (unused?)
// Total size +4 for saved LR is 352

	// w[i] = p[j]<<24 | p[j+1]<<16 | p[j+2]<<8 | p[j+3]
	// e += w[i]
#define LOAD(Re) \
	MOVBU	2(Rdata), Rt0 ; \
	MOVBU	3(Rdata), Rt1 ; \
	MOVBU	1(Rdata), Rt2 ; \
	ORR	Rt0<<8, Rt1, Rt0	    ; \
	MOVBU.P	4(Rdata), Rt1 ; \
	ORR	Rt2<<16, Rt0, Rt0	    ; \
	ORR	Rt1<<24, Rt0, Rt0	    ; \
	MOVW.P	Rt0, 4(Rw)		    ; \
	ADD	Rt0, Re, Re

	// tmp := w[(i-3)&0xf] ^ w[(i-8)&0xf] ^ w[(i-14)&0xf] ^ w[(i)&0xf]
	// w[i&0xf] = tmp<<1 | tmp>>(32-1)
	// e += w[i&0xf]
#define SHUFFLE(Re) \
	MOVW	(-16*4)(Rw), Rt0 ; \
	MOVW	(-14*4)(Rw), Rt1 ; \
	MOVW	(-8*4)(Rw), Rt2  ; \
	EOR	Rt0, Rt1, Rt0  ; \
	MOVW	(-3*4)(Rw), Rt1  ; \
	EOR	Rt2, Rt0, Rt0  ; \
	EOR	Rt0, Rt1, Rt0  ; \
	MOVW	Rt0@>(32-1), Rt0  ; \
	MOVW.P	Rt0, 4(Rw)	  ; \
	ADD	Rt0, Re, Re

	// t1 = (b & c) | ((~b) & d)
#define FUNC1(Ra, Rb, Rc, Rd, Re) \
	MVN	Rb, Rt1	   ; \
	AND	Rb, Rc, Rt0  ; \
	AND	Rd, Rt1, Rt1 ; \
	ORR	Rt0, Rt1, Rt1

	// t1 = b ^ c ^ d
#define FUNC2(Ra, Rb, Rc, Rd, Re) \
	EOR	Rb, Rc, Rt1 ; \
	EOR	Rd, Rt1, Rt1

	// t1 = (b & c) | (b & d) | (c & d) =
	// t1 = (b & c) | ((b | c) & d)
#define FUNC3(Ra, Rb, Rc, Rd, Re) \
	ORR	Rb, Rc, Rt0  ; \
	AND	Rb, Rc, Rt1  ; \
	AND	Rd, Rt0, Rt0 ; \
	ORR	Rt0, Rt1, Rt1

#define FUNC4 FUNC2

	// a5 := a<<5 | a>>(32-5)
	// b = b<<30 | b>>(32-30)
	// e = a5 + t1 + e + const
#define MIX(Ra, Rb, Rc, Rd, Re) \
	ADD	Rt1, Re, Re	 ; \
	MOVW	Rb@>(32-30), Rb	 ; \
	ADD	Ra@>(32-5), Re, Re ; \
	ADD	Rconst, Re, Re

#define ROUND1(Ra, Rb, Rc, Rd, Re) \
	LOAD(Re)		; \
	FUNC1(Ra, Rb, Rc, Rd, Re)	; \
	MIX(Ra, Rb, Rc, Rd, Re)

#define ROUND1x(Ra, Rb, Rc, Rd, Re) \
	SHUFFLE(Re)	; \
	FUNC1(Ra, Rb, Rc, Rd, Re)	; \
	MIX(Ra, Rb, Rc, Rd, Re)

#define ROUND2(Ra, Rb, Rc, Rd, Re) \
	SHUFFLE(Re)	; \
	FUNC2(Ra, Rb, Rc, Rd, Re)	; \
	MIX(Ra, Rb, Rc, Rd, Re)

#define ROUND3(Ra, Rb, Rc, Rd, Re) \
	SHUFFLE(Re)	; \
	FUNC3(Ra, Rb, Rc, Rd, Re)	; \
	MIX(Ra, Rb, Rc, Rd, Re)

#define ROUND4(Ra, Rb, Rc, Rd, Re) \
	SHUFFLE(Re)	; \
	FUNC4(Ra, Rb, Rc, Rd, Re)	; \
	MIX(Ra, Rb, Rc, Rd, Re)


// func block(dig *digest, p []byte)
TEXT	·block(SB), 0, $352-16
	MOVW	p+4(FP), Rdata	// pointer to the data
	MOVW	p_len+8(FP), Rt0	// number of bytes
	ADD	Rdata, Rt0
	MOVW	Rt0, p_end	// pointer to end of data

	// Load up initial SHA-1 accumulator
	MOVW	dig+0(FP), Rt0
	MOVM.IA (Rt0), [Ra,Rb,Rc,Rd,Re]

loop:
	// Save registers at SP+4 onwards
	MOVM.IB [Ra,Rb,Rc,Rd,Re], (R13)

	MOVW	$w_buf, Rw
	MOVW	$0x5A827999, Rconst
	MOVW	$3, Rctr
loop1:	ROUND1(Ra, Rb, Rc, Rd, Re)
	ROUND1(Re, Ra, Rb, Rc, Rd)
	ROUND1(Rd, Re, Ra, Rb, Rc)
	ROUND1(Rc, Rd, Re, Ra, Rb)
	ROUND1(Rb, Rc, Rd, Re, Ra)
	SUB.S	$1, Rctr
	BNE	loop1

	ROUND1(Ra, Rb, Rc, Rd, Re)
	ROUND1x(Re, Ra, Rb, Rc, Rd)
	ROUND1x(Rd, Re, Ra, Rb, Rc)
	ROUND1x(Rc, Rd, Re, Ra, Rb)
	ROUND1x(Rb, Rc, Rd, Re, Ra)

	MOVW	$0x6ED9EBA1, Rconst
	MOVW	$4, Rctr
loop2:	ROUND2(Ra, Rb, Rc, Rd, Re)
	ROUND2(Re, Ra, Rb, Rc, Rd)
	ROUND2(Rd, Re, Ra, Rb, Rc)
	ROUND2(Rc, Rd, Re, Ra, Rb)
	ROUND2(Rb, Rc, Rd, Re, Ra)
	SUB.S	$1, Rctr
	BNE	loop2

	MOVW	$0x8F1BBCDC, Rconst
	MOVW	$4, Rctr
loop3:	ROUND3(Ra, Rb, Rc, Rd, Re)
	ROUND3(Re, Ra, Rb, Rc, Rd)
	ROUND3(Rd, Re, Ra, Rb, Rc)
	ROUND3(Rc, Rd, Re, Ra, Rb)
	ROUND3(Rb, Rc, Rd, Re, Ra)
	SUB.S	$1, Rctr
	BNE	loop3

	MOVW	$0xCA62C1D6, Rconst
	MOVW	$4, Rctr
loop4:	ROUND4(Ra, Rb, Rc, Rd, Re)
	ROUND4(Re, Ra, Rb, Rc, Rd)
	ROUND4(Rd, Re, Ra, Rb, Rc)
	ROUND4(Rc, Rd, Re, Ra, Rb)
	ROUND4(Rb, Rc, Rd, Re, Ra)
	SUB.S	$1, Rctr
	BNE	loop4

	// Accumulate - restoring registers from SP+4
	MOVM.IB (R13), [Rt0,Rt1,Rt2,Rctr,Rw]
	ADD	Rt0, Ra
	ADD	Rt1, Rb
	ADD	Rt2, Rc
	ADD	Rctr, Rd
	ADD	Rw, Re

	MOVW	p_end, Rt0
	CMP	Rt0, Rdata
	BLO	loop

	// Save final SHA-1 accumulator
	MOVW	dig+0(FP), Rt0
	MOVM.IA [Ra,Rb,Rc,Rd,Re], (Rt0)

	RET

```

// === FILE: references/go/src/crypto/sha1/sha1block_arm64.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !purego

package sha1

import (
	"crypto/internal/impl"
	"internal/cpu"
)

var useSHA1 = cpu.ARM64.HasSHA1

func init() {
	impl.Register("sha1", "Armv8.0", &useSHA1)
}

var k = []uint32{
	0x5A827999,
	0x6ED9EBA1,
	0x8F1BBCDC,
	0xCA62C1D6,
}

//go:noescape
func sha1block(h []uint32, p []byte, k []uint32)

func block(dig *digest, p []byte) {
	if useSHA1 {
		h := dig.h[:]
		sha1block(h, p, k)
	} else {
		blockGeneric(dig, p)
	}
}

```

// === FILE: references/go/src/crypto/sha1/sha1block_arm64.s ===
```text
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !purego

#include "textflag.h"

#define HASHUPDATECHOOSE \
	SHA1C	V16.S4, V1, V2 \
	SHA1H	V3, V1 \
	VMOV	V2.B16, V3.B16

#define HASHUPDATEPARITY \
	SHA1P	V16.S4, V1, V2 \
	SHA1H	V3, V1 \
	VMOV	V2.B16, V3.B16

#define HASHUPDATEMAJ \
	SHA1M	V16.S4, V1, V2 \
	SHA1H	V3, V1 \
	VMOV	V2.B16, V3.B16

// func sha1block(h []uint32, p []byte, k []uint32)
TEXT ·sha1block(SB),NOSPLIT,$0
	MOVD	h_base+0(FP), R0                             // hash value first address
	MOVD	p_base+24(FP), R1                            // message first address
	MOVD	k_base+48(FP), R2                            // k constants first address
	MOVD	p_len+32(FP), R3                             // message length
	VLD1.P	16(R0), [V0.S4]
	FMOVS	(R0), F20
	SUB	$16, R0, R0

blockloop:

	VLD1.P	16(R1), [V4.B16]                             // load message
	VLD1.P	16(R1), [V5.B16]
	VLD1.P	16(R1), [V6.B16]
	VLD1.P	16(R1), [V7.B16]
	VLD1	(R2), [V19.S4]                               // load constant k0-k79
	VMOV	V0.B16, V2.B16
	VMOV	V20.S[0], V1
	VMOV	V2.B16, V3.B16
	VDUP	V19.S[0], V17.S4
	VREV32	V4.B16, V4.B16                               // prepare for using message in Byte format
	VREV32	V5.B16, V5.B16
	VREV32	V6.B16, V6.B16
	VREV32	V7.B16, V7.B16


	VDUP	V19.S[1], V18.S4
	VADD	V17.S4, V4.S4, V16.S4
	SHA1SU0	V6.S4, V5.S4, V4.S4
	HASHUPDATECHOOSE
	SHA1SU1	V7.S4, V4.S4

	VADD	V17.S4, V5.S4, V16.S4
	SHA1SU0	V7.S4, V6.S4, V5.S4
	HASHUPDATECHOOSE
	SHA1SU1	V4.S4, V5.S4
	VADD	V17.S4, V6.S4, V16.S4
	SHA1SU0	V4.S4, V7.S4, V6.S4
	HASHUPDATECHOOSE
	SHA1SU1	V5.S4, V6.S4

	VADD	V17.S4, V7.S4, V16.S4
	SHA1SU0	V5.S4, V4.S4, V7.S4
	HASHUPDATECHOOSE
	SHA1SU1	V6.S4, V7.S4

	VADD	V17.S4, V4.S4, V16.S4
	SHA1SU0	V6.S4, V5.S4, V4.S4
	HASHUPDATECHOOSE
	SHA1SU1	V7.S4, V4.S4

	VDUP	V19.S[2], V17.S4
	VADD	V18.S4, V5.S4, V16.S4
	SHA1SU0	V7.S4, V6.S4, V5.S4
	HASHUPDATEPARITY
	SHA1SU1	V4.S4, V5.S4

	VADD	V18.S4, V6.S4, V16.S4
	SHA1SU0	V4.S4, V7.S4, V6.S4
	HASHUPDATEPARITY
	SHA1SU1	V5.S4, V6.S4

	VADD	V18.S4, V7.S4, V16.S4
	SHA1SU0	V5.S4, V4.S4, V7.S4
	HASHUPDATEPARITY
	SHA1SU1	V6.S4, V7.S4

	VADD	V18.S4, V4.S4, V16.S4
	SHA1SU0	V6.S4, V5.S4, V4.S4
	HASHUPDATEPARITY
	SHA1SU1	V7.S4, V4.S4

	VADD	V18.S4, V5.S4, V16.S4
	SHA1SU0	V7.S4, V6.S4, V5.S4
	HASHUPDATEPARITY
	SHA1SU1	V4.S4, V5.S4

	VDUP	V19.S[3], V18.S4
	VADD	V17.S4, V6.S4, V16.S4
	SHA1SU0	V4.S4, V7.S4, V6.S4
	HASHUPDATEMAJ
	SHA1SU1	V5.S4, V6.S4

	VADD	V17.S4, V7.S4, V16.S4
	SHA1SU0	V5.S4, V4.S4, V7.S4
	HASHUPDATEMAJ
	SHA1SU1	V6.S4, V7.S4

	VADD	V17.S4, V4.S4, V16.S4
	SHA1SU0	V6.S4, V5.S4, V4.S4
	HASHUPDATEMAJ
	SHA1SU1	V7.S4, V4.S4

	VADD	V17.S4, V5.S4, V16.S4
	SHA1SU0	V7.S4, V6.S4, V5.S4
	HASHUPDATEMAJ
	SHA1SU1	V4.S4, V5.S4

	VADD	V17.S4, V6.S4, V16.S4
	SHA1SU0	V4.S4, V7.S4, V6.S4
	HASHUPDATEMAJ
	SHA1SU1	V5.S4, V6.S4

	VADD	V18.S4, V7.S4, V16.S4
	SHA1SU0	V5.S4, V4.S4, V7.S4
	HASHUPDATEPARITY
	SHA1SU1	V6.S4, V7.S4

	VADD	V18.S4, V4.S4, V16.S4
	HASHUPDATEPARITY

	VADD	V18.S4, V5.S4, V16.S4
	HASHUPDATEPARITY

	VADD	V18.S4, V6.S4, V16.S4
	HASHUPDATEPARITY

	VADD	V18.S4, V7.S4, V16.S4
	HASHUPDATEPARITY

	SUB	$64, R3, R3                                  // message length - 64bytes, then compare with 64bytes
	VADD	V2.S4, V0.S4, V0.S4
	VADD	V1.S4, V20.S4, V20.S4
	CBNZ	R3, blockloop

sha1ret:

	VST1.P	[V0.S4], 16(R0)                               // store hash value H(dcba)
	FMOVS	F20, (R0)                                     // store hash value H(e)
	RET

```

// === FILE: references/go/src/crypto/sha1/sha1block_decl.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build (386 || arm || loong64 || riscv64) && !purego

package sha1

//go:noescape
func block(dig *digest, p []byte)

```

// === FILE: references/go/src/crypto/sha1/sha1block_generic.go ===
```go
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build (!386 && !amd64 && !arm && !arm64 && !loong64 && !riscv64 && !s390x) || purego

package sha1

func block(dig *digest, p []byte) {
	blockGeneric(dig, p)
}

```

// === FILE: references/go/src/crypto/sha1/sha1block_loong64.s ===
```text
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !purego

#include "textflag.h"

// SHA-1 block routine. See sha1block.go for Go equivalent.
//
// There are 80 rounds of 4 types:
//   - rounds 0-15 are type 1 and load data (ROUND1 macro).
//   - rounds 16-19 are type 1 and do not load data (ROUND1x macro).
//   - rounds 20-39 are type 2 and do not load data (ROUND2 macro).
//   - rounds 40-59 are type 3 and do not load data (ROUND3 macro).
//   - rounds 60-79 are type 4 and do not load data (ROUND4 macro).
//
// Each round loads or shuffles the data, then computes a per-round
// function of b, c, d, and then mixes the result into and rotates the
// five registers a, b, c, d, e holding the intermediate results.
//
// The register rotation is implemented by rotating the arguments to
// the round macros instead of by explicit move instructions.

#define REGTMP	R30
#define REGTMP1	R17
#define REGTMP2	R18
#define REGTMP3	R19
#define KEYREG1	R25
#define KEYREG2	R26
#define KEYREG3	R27
#define KEYREG4	R28

#define LOAD1(index) \
	MOVW	(index*4)(R5), REGTMP3; \
	REVB2W	REGTMP3, REGTMP3; \
	MOVW	REGTMP3, (index*4)(R3)

#define LOAD(index) \
	MOVW	(((index)&0xf)*4)(R3), REGTMP3; \
	MOVW	(((index-3)&0xf)*4)(R3), REGTMP; \
	MOVW	(((index-8)&0xf)*4)(R3), REGTMP1; \
	MOVW	(((index-14)&0xf)*4)(R3), REGTMP2; \
	XOR	REGTMP, REGTMP3; \
	XOR	REGTMP1, REGTMP3; \
	XOR	REGTMP2, REGTMP3; \
	ROTR	$31, REGTMP3; \
	MOVW	REGTMP3, (((index)&0xf)*4)(R3)

// f = d ^ (b & (c ^ d))
#define FUNC1(a, b, c, d, e) \
	XOR	c, d, REGTMP1; \
	AND	b, REGTMP1; \
	XOR	d, REGTMP1

// f = b ^ c ^ d
#define FUNC2(a, b, c, d, e) \
	XOR	b, c, REGTMP1; \
	XOR	d, REGTMP1

// f = (b & c) | ((b | c) & d)
#define FUNC3(a, b, c, d, e) \
	OR	b, c, REGTMP2; \
	AND	b, c, REGTMP; \
	AND	d, REGTMP2; \
	OR	REGTMP, REGTMP2, REGTMP1

#define FUNC4 FUNC2

#define MIX(a, b, c, d, e, key) \
	ROTR	$2, b; \	// b << 30
	ADD	REGTMP1, e; \	// e = e + f
	ROTR	$27, a, REGTMP2; \	// a << 5
	ADD	REGTMP3, e; \	// e = e + w[i]
	ADDV	key, e; \	// e = e + k
	ADD	REGTMP2, e	// e = e + a<<5

#define ROUND1(a, b, c, d, e, index) \
	LOAD1(index); \
	FUNC1(a, b, c, d, e); \
	MIX(a, b, c, d, e, KEYREG1)

#define ROUND1x(a, b, c, d, e, index) \
	LOAD(index); \
	FUNC1(a, b, c, d, e); \
	MIX(a, b, c, d, e, KEYREG1)

#define ROUND2(a, b, c, d, e, index) \
	LOAD(index); \
	FUNC2(a, b, c, d, e); \
	MIX(a, b, c, d, e, KEYREG2)

#define ROUND3(a, b, c, d, e, index) \
	LOAD(index); \
	FUNC3(a, b, c, d, e); \
	MIX(a, b, c, d, e, KEYREG3)

#define ROUND4(a, b, c, d, e, index) \
	LOAD(index); \
	FUNC4(a, b, c, d, e); \
	MIX(a, b, c, d, e, KEYREG4)

// A stack frame size of 64 bytes is required here, because
// the frame size used for data expansion is 64 bytes.
// See the definition of the macro LOAD above, and the definition
// of the local variable w in the general implementation (sha1block.go).
TEXT ·block(SB),NOSPLIT,$64-32
	MOVV	dig+0(FP),	R4
	MOVV	p_base+8(FP),	R5
	MOVV	p_len+16(FP),	R6
	AND	$~63, R6
	BEQ	R6, zero

	// p_len >= 64
	ADDV	R5, R6, R24
	MOVW	(0*4)(R4), R7
	MOVW	(1*4)(R4), R8
	MOVW	(2*4)(R4), R9
	MOVW	(3*4)(R4), R10
	MOVW	(4*4)(R4), R11

	MOVV	$·_K(SB), R21
	MOVW	(0*4)(R21), KEYREG1
	MOVW	(1*4)(R21), KEYREG2
	MOVW	(2*4)(R21), KEYREG3
	MOVW	(3*4)(R21), KEYREG4

loop:
	MOVW	R7,	R12
	MOVW	R8,	R13
	MOVW	R9,	R14
	MOVW	R10,	R15
	MOVW	R11,	R16

	ROUND1(R7,  R8,  R9,  R10, R11, 0)
	ROUND1(R11, R7,  R8,  R9,  R10, 1)
	ROUND1(R10, R11, R7,  R8,  R9,  2)
	ROUND1(R9,  R10, R11, R7,  R8,  3)
	ROUND1(R8,  R9,  R10, R11, R7,  4)
	ROUND1(R7,  R8,  R9,  R10, R11, 5)
	ROUND1(R11, R7,  R8,  R9,  R10, 6)
	ROUND1(R10, R11, R7,  R8,  R9,  7)
	ROUND1(R9,  R10, R11, R7,  R8,  8)
	ROUND1(R8,  R9,  R10, R11, R7,  9)
	ROUND1(R7,  R8,  R9,  R10, R11, 10)
	ROUND1(R11, R7,  R8,  R9,  R10, 11)
	ROUND1(R10, R11, R7,  R8,  R9,  12)
	ROUND1(R9,  R10, R11, R7,  R8,  13)
	ROUND1(R8,  R9,  R10, R11, R7,  14)
	ROUND1(R7,  R8,  R9,  R10, R11, 15)

	ROUND1x(R11, R7,  R8,  R9,  R10, 16)
	ROUND1x(R10, R11, R7,  R8,  R9,  17)
	ROUND1x(R9,  R10, R11, R7,  R8,  18)
	ROUND1x(R8,  R9,  R10, R11, R7,  19)

	ROUND2(R7,  R8,  R9,  R10, R11, 20)
	ROUND2(R11, R7,  R8,  R9,  R10, 21)
	ROUND2(R10, R11, R7,  R8,  R9,  22)
	ROUND2(R9,  R10, R11, R7,  R8,  23)
	ROUND2(R8,  R9,  R10, R11, R7,  24)
	ROUND2(R7,  R8,  R9,  R10, R11, 25)
	ROUND2(R11, R7,  R8,  R9,  R10, 26)
	ROUND2(R10, R11, R7,  R8,  R9,  27)
	ROUND2(R9,  R10, R11, R7,  R8,  28)
	ROUND2(R8,  R9,  R10, R11, R7,  29)
	ROUND2(R7,  R8,  R9,  R10, R11, 30)
	ROUND2(R11, R7,  R8,  R9,  R10, 31)
	ROUND2(R10, R11, R7,  R8,  R9,  32)
	ROUND2(R9,  R10, R11, R7,  R8,  33)
	ROUND2(R8,  R9,  R10, R11, R7,  34)
	ROUND2(R7,  R8,  R9,  R10, R11, 35)
	ROUND2(R11, R7,  R8,  R9,  R10, 36)
	ROUND2(R10, R11, R7,  R8,  R9,  37)
	ROUND2(R9,  R10, R11, R7,  R8,  38)
	ROUND2(R8,  R9,  R10, R11, R7,  39)

	ROUND3(R7,  R8,  R9,  R10, R11, 40)
	ROUND3(R11, R7,  R8,  R9,  R10, 41)
	ROUND3(R10, R11, R7,  R8,  R9,  42)
	ROUND3(R9,  R10, R11, R7,  R8,  43)
	ROUND3(R8,  R9,  R10, R11, R7,  44)
	ROUND3(R7,  R8,  R9,  R10, R11, 45)
	ROUND3(R11, R7,  R8,  R9,  R10, 46)
	ROUND3(R10, R11, R7,  R8,  R9,  47)
	ROUND3(R9,  R10, R11, R7,  R8,  48)
	ROUND3(R8,  R9,  R10, R11, R7,  49)
	ROUND3(R7,  R8,  R9,  R10, R11, 50)
	ROUND3(R11, R7,  R8,  R9,  R10, 51)
	ROUND3(R10, R11, R7,  R8,  R9,  52)
	ROUND3(R9,  R10, R11, R7,  R8,  53)
	ROUND3(R8,  R9,  R10, R11, R7,  54)
	ROUND3(R7,  R8,  R9,  R10, R11, 55)
	ROUND3(R11, R7,  R8,  R9,  R10, 56)
	ROUND3(R10, R11, R7,  R8,  R9,  57)
	ROUND3(R9,  R10, R11, R7,  R8,  58)
	ROUND3(R8,  R9,  R10, R11, R7,  59)

	ROUND4(R7,  R8,  R9,  R10, R11, 60)
	ROUND4(R11, R7,  R8,  R9,  R10, 61)
	ROUND4(R10, R11, R7,  R8,  R9,  62)
	ROUND4(R9,  R10, R11, R7,  R8,  63)
	ROUND4(R8,  R9,  R10, R11, R7,  64)
	ROUND4(R7,  R8,  R9,  R10, R11, 65)
	ROUND4(R11, R7,  R8,  R9,  R10, 66)
	ROUND4(R10, R11, R7,  R8,  R9,  67)
	ROUND4(R9,  R10, R11, R7,  R8,  68)
	ROUND4(R8,  R9,  R10, R11, R7,  69)
	ROUND4(R7,  R8,  R9,  R10, R11, 70)
	ROUND4(R11, R7,  R8,  R9,  R10, 71)
	ROUND4(R10, R11, R7,  R8,  R9,  72)
	ROUND4(R9,  R10, R11, R7,  R8,  73)
	ROUND4(R8,  R9,  R10, R11, R7,  74)
	ROUND4(R7,  R8,  R9,  R10, R11, 75)
	ROUND4(R11, R7,  R8,  R9,  R10, 76)
	ROUND4(R10, R11, R7,  R8,  R9,  77)
	ROUND4(R9,  R10, R11, R7,  R8,  78)
	ROUND4(R8,  R9,  R10, R11, R7,  79)

	ADD	R12, R7
	ADD	R13, R8
	ADD	R14, R9
	ADD	R15, R10
	ADD	R16, R11

	ADDV	$64, R5
	BNE	R5, R24, loop

end:
	MOVW	R7, (0*4)(R4)
	MOVW	R8, (1*4)(R4)
	MOVW	R9, (2*4)(R4)
	MOVW	R10, (3*4)(R4)
	MOVW	R11, (4*4)(R4)
zero:
	RET

GLOBL	·_K(SB),RODATA,$16
DATA	·_K+0(SB)/4, $0x5A827999
DATA	·_K+4(SB)/4, $0x6ED9EBA1
DATA	·_K+8(SB)/4, $0x8F1BBCDC
DATA	·_K+12(SB)/4, $0xCA62C1D6

```

// === FILE: references/go/src/crypto/sha1/sha1block_riscv64.s ===
```text
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !purego

#include "textflag.h"

#define LOAD(index) \
	MOVBU	((index*4)+0)(X29), X5; \
	MOVBU	((index*4)+1)(X29), X6; \
	MOVBU	((index*4)+2)(X29), X7; \
	MOVBU	((index*4)+3)(X29), X8; \
	SLL	$24, X5; \
	SLL	$16, X6; \
	OR	X5, X6, X5; \
	SLL	$8, X7; \
	OR	X5, X7, X5; \
	OR	X5, X8, X5; \
	MOVW	X5, (index*4)(X19)

#define SHUFFLE(index) \
	MOVWU	(((index)&0xf)*4)(X19), X5; \
	MOVWU	(((index-3)&0xf)*4)(X19), X6; \
	MOVWU	(((index-8)&0xf)*4)(X19), X7; \
	MOVWU	(((index-14)&0xf)*4)(X19), X8; \
	XOR	X6, X5; \
	XOR	X7, X5; \
	XOR	X8, X5; \
	RORW	$31, X5; \
	MOVW	X5, (((index)&0xf)*4)(X19)

// f = d ^ (b & (c ^ d))
#define FUNC1(a, b, c, d, e) \
	XOR	c, d, X7; \
	AND	b, X7; \
	XOR	d, X7

// f = b ^ c ^ d
#define FUNC2(a, b, c, d, e) \
	XOR	b, c, X7; \
	XOR	d, X7

// f = (b & c) | ((b | c) & d)
#define FUNC3(a, b, c, d, e) \
	OR	b, c, X8; \
	AND	b, c, X6; \
	AND	d, X8; \
	OR	X6, X8, X7

#define FUNC4 FUNC2

#define MIX(a, b, c, d, e, key) \
	RORW	$2, b; \
	ADD	X7, e; \
	RORW	$27, a, X8; \
	ADD	X5, e; \
	ADD	key, e; \
	ADD	X8, e

#define ROUND1(a, b, c, d, e, index) \
	LOAD(index); \
	FUNC1(a, b, c, d, e); \
	MIX(a, b, c, d, e, X15)

#define ROUND1x(a, b, c, d, e, index) \
	SHUFFLE(index); \
	FUNC1(a, b, c, d, e); \
	MIX(a, b, c, d, e, X15)

#define ROUND2(a, b, c, d, e, index) \
	SHUFFLE(index); \
	FUNC2(a, b, c, d, e); \
	MIX(a, b, c, d, e, X16)

#define ROUND3(a, b, c, d, e, index) \
	SHUFFLE(index); \
	FUNC3(a, b, c, d, e); \
	MIX(a, b, c, d, e, X17)

#define ROUND4(a, b, c, d, e, index) \
	SHUFFLE(index); \
	FUNC4(a, b, c, d, e); \
	MIX(a, b, c, d, e, X18)

// func block(dig *Digest, p []byte)
TEXT ·block(SB),NOSPLIT,$64-32
	MOV	p_base+8(FP), X29
	MOV	p_len+16(FP), X30
	SRL	$6, X30
	SLL	$6, X30

	ADD	X29, X30, X28
	BEQ	X28, X29, end

	ADD	$8, X2, X19	// message schedule buffer on stack

	MOV	dig+0(FP), X20
	MOVWU	(0*4)(X20), X10	// a = H0
	MOVWU	(1*4)(X20), X11	// b = H1
	MOVWU	(2*4)(X20), X12	// c = H2
	MOVWU	(3*4)(X20), X13	// d = H3
	MOVWU	(4*4)(X20), X14	// e = H4

	MOV	$·_K(SB), X21
	MOVW	(0*4)(X21), X15
	MOVW	(1*4)(X21), X16
	MOVW	(2*4)(X21), X17
	MOVW	(3*4)(X21), X18

loop:
	MOVW	X10, X22
	MOVW	X11, X23
	MOVW	X12, X24
	MOVW	X13, X25
	MOVW	X14, X26

	ROUND1(X10, X11, X12, X13, X14, 0)
	ROUND1(X14, X10, X11, X12, X13, 1)
	ROUND1(X13, X14, X10, X11, X12, 2)
	ROUND1(X12, X13, X14, X10, X11, 3)
	ROUND1(X11, X12, X13, X14, X10, 4)
	ROUND1(X10, X11, X12, X13, X14, 5)
	ROUND1(X14, X10, X11, X12, X13, 6)
	ROUND1(X13, X14, X10, X11, X12, 7)
	ROUND1(X12, X13, X14, X10, X11, 8)
	ROUND1(X11, X12, X13, X14, X10, 9)
	ROUND1(X10, X11, X12, X13, X14, 10)
	ROUND1(X14, X10, X11, X12, X13, 11)
	ROUND1(X13, X14, X10, X11, X12, 12)
	ROUND1(X12, X13, X14, X10, X11, 13)
	ROUND1(X11, X12, X13, X14, X10, 14)
	ROUND1(X10, X11, X12, X13, X14, 15)

	ROUND1x(X14, X10, X11, X12, X13, 16)
	ROUND1x(X13, X14, X10, X11, X12, 17)
	ROUND1x(X12, X13, X14, X10, X11, 18)
	ROUND1x(X11, X12, X13, X14, X10, 19)

	ROUND2(X10, X11, X12, X13, X14, 20)
	ROUND2(X14, X10, X11, X12, X13, 21)
	ROUND2(X13, X14, X10, X11, X12, 22)
	ROUND2(X12, X13, X14, X10, X11, 23)
	ROUND2(X11, X12, X13, X14, X10, 24)
	ROUND2(X10, X11, X12, X13, X14, 25)
	ROUND2(X14, X10, X11, X12, X13, 26)
	ROUND2(X13, X14, X10, X11, X12, 27)
	ROUND2(X12, X13, X14, X10, X11, 28)
	ROUND2(X11, X12, X13, X14, X10, 29)
	ROUND2(X10, X11, X12, X13, X14, 30)
	ROUND2(X14, X10, X11, X12, X13, 31)
	ROUND2(X13, X14, X10, X11, X12, 32)
	ROUND2(X12, X13, X14, X10, X11, 33)
	ROUND2(X11, X12, X13, X14, X10, 34)
	ROUND2(X10, X11, X12, X13, X14, 35)
	ROUND2(X14, X10, X11, X12, X13, 36)
	ROUND2(X13, X14, X10, X11, X12, 37)
	ROUND2(X12, X13, X14, X10, X11, 38)
	ROUND2(X11, X12, X13, X14, X10, 39)

	ROUND3(X10, X11, X12, X13, X14, 40)
	ROUND3(X14, X10, X11, X12, X13, 41)
	ROUND3(X13, X14, X10, X11, X12, 42)
	ROUND3(X12, X13, X14, X10, X11, 43)
	ROUND3(X11, X12, X13, X14, X10, 44)
	ROUND3(X10, X11, X12, X13, X14, 45)
	ROUND3(X14, X10, X11, X12, X13, 46)
	ROUND3(X13, X14, X10, X11, X12, 47)
	ROUND3(X12, X13, X14, X10, X11, 48)
	ROUND3(X11, X12, X13, X14, X10, 49)
	ROUND3(X10, X11, X12, X13, X14, 50)
	ROUND3(X14, X10, X11, X12, X13, 51)
	ROUND3(X13, X14, X10, X11, X12, 52)
	ROUND3(X12, X13, X14, X10, X11, 53)
	ROUND3(X11, X12, X13, X14, X10, 54)
	ROUND3(X10, X11, X12, X13, X14, 55)
	ROUND3(X14, X10, X11, X12, X13, 56)
	ROUND3(X13, X14, X10, X11, X12, 57)
	ROUND3(X12, X13, X14, X10, X11, 58)
	ROUND3(X11, X12, X13, X14, X10, 59)

	ROUND4(X10, X11, X12, X13, X14, 60)
	ROUND4(X14, X10, X11, X12, X13, 61)
	ROUND4(X13, X14, X10, X11, X12, 62)
	ROUND4(X12, X13, X14, X10, X11, 63)
	ROUND4(X11, X12, X13, X14, X10, 64)
	ROUND4(X10, X11, X12, X13, X14, 65)
	ROUND4(X14, X10, X11, X12, X13, 66)
	ROUND4(X13, X14, X10, X11, X12, 67)
	ROUND4(X12, X13, X14, X10, X11, 68)
	ROUND4(X11, X12, X13, X14, X10, 69)
	ROUND4(X10, X11, X12, X13, X14, 70)
	ROUND4(X14, X10, X11, X12, X13, 71)
	ROUND4(X13, X14, X10, X11, X12, 72)
	ROUND4(X12, X13, X14, X10, X11, 73)
	ROUND4(X11, X12, X13, X14, X10, 74)
	ROUND4(X10, X11, X12, X13, X14, 75)
	ROUND4(X14, X10, X11, X12, X13, 76)
	ROUND4(X13, X14, X10, X11, X12, 77)
	ROUND4(X12, X13, X14, X10, X11, 78)
	ROUND4(X11, X12, X13, X14, X10, 79)

	ADD	X22, X10
	ADD	X23, X11
	ADD	X24, X12
	ADD	X25, X13
	ADD	X26, X14

	ADD	$64, X29
	BNE	X28, X29, loop

end:
	MOVW	X10, (0*4)(X20)
	MOVW	X11, (1*4)(X20)
	MOVW	X12, (2*4)(X20)
	MOVW	X13, (3*4)(X20)
	MOVW	X14, (4*4)(X20)

	RET

GLOBL	·_K(SB),RODATA,$16
DATA	·_K+0(SB)/4, $0x5A827999
DATA	·_K+4(SB)/4, $0x6ED9EBA1
DATA	·_K+8(SB)/4, $0x8F1BBCDC
DATA	·_K+12(SB)/4, $0xCA62C1D6

```

// === FILE: references/go/src/crypto/sha1/sha1block_s390x.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !purego

package sha1

import (
	"crypto/internal/impl"
	"internal/cpu"
)

var useSHA1 = cpu.S390X.HasSHA1

func init() {
	// CP Assist for Cryptographic Functions (CPACF)
	// https://www.ibm.com/docs/en/zos/3.1.0?topic=icsf-cp-assist-cryptographic-functions-cpacf
	impl.Register("sha1", "CPACF", &useSHA1)
}

//go:noescape
func blockS390X(dig *digest, p []byte)

func block(dig *digest, p []byte) {
	if useSHA1 {
		blockS390X(dig, p)
	} else {
		blockGeneric(dig, p)
	}
}

```

// === FILE: references/go/src/crypto/sha1/sha1block_s390x.s ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !purego

#include "textflag.h"

// func blockS390X(dig *digest, p []byte)
TEXT ·blockS390X(SB), NOSPLIT|NOFRAME, $0-32
	LMG    dig+0(FP), R1, R3            // R2 = &p[0], R3 = len(p)
	MOVBZ  $1, R0                       // SHA-1 function code

loop:
	KIMD R0, R2      // compute intermediate message digest (KIMD)
	BVS  loop        // continue if interrupted
	RET

```

