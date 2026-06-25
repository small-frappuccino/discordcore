# Domain Architecture: math

## Layout Topology
```text
math/
├── big
│   ├── internal
│   │   └── asmgen
│   │       ├── 386.go
│   │       ├── add.go
│   │       ├── amd64.go
│   │       ├── arch.go
│   │       ├── arm.go
│   │       ├── arm64.go
│   │       ├── asm.go
│   │       ├── cheat.go
│   │       ├── func.go
│   │       ├── loong64.go
│   │       ├── main.go
│   │       ├── mips.go
│   │       ├── mips64.go
│   │       ├── mul.go
│   │       ├── pipe.go
│   │       ├── ppc64.go
│   │       ├── riscv64.go
│   │       ├── s390x.go
│   │       └── shift.go
│   ├── accuracy_string.go
│   ├── arith.go
│   ├── arith_386.s
│   ├── arith_amd64.go
│   ├── arith_amd64.s
│   ├── arith_arm.s
│   ├── arith_arm64.s
│   ├── arith_decl.go
│   ├── arith_decl_pure.go
│   ├── arith_loong64.s
│   ├── arith_mips64x.s
│   ├── arith_mipsx.s
│   ├── arith_ppc64x.s
│   ├── arith_riscv64.s
│   ├── arith_s390x.s
│   ├── arith_wasm.s
│   ├── arithvec_s390x.go
│   ├── arithvec_s390x.s
│   ├── calibrate.md
│   ├── calibrate_graph.go
│   ├── decimal.go
│   ├── doc.go
│   ├── float.go
│   ├── floatconv.go
│   ├── floatmarsh.go
│   ├── ftoa.go
│   ├── int.go
│   ├── intconv.go
│   ├── intmarsh.go
│   ├── nat.go
│   ├── natconv.go
│   ├── natdiv.go
│   ├── natmul.go
│   ├── prime.go
│   ├── rat.go
│   ├── ratconv.go
│   ├── ratmarsh.go
│   ├── roundingmode_string.go
│   └── sqrt.go
├── bits
│   ├── bits.go
│   ├── bits_errors.go
│   ├── bits_errors_bootstrap.go
│   ├── bits_tables.go
│   ├── make_examples.go
│   └── make_tables.go
├── cmplx
│   ├── abs.go
│   ├── asin.go
│   ├── conj.go
│   ├── exp.go
│   ├── isinf.go
│   ├── isnan.go
│   ├── log.go
│   ├── phase.go
│   ├── polar.go
│   ├── pow.go
│   ├── rect.go
│   ├── sin.go
│   ├── sqrt.go
│   └── tan.go
├── rand
│   ├── v2
│   │   ├── chacha8.go
│   │   ├── exp.go
│   │   ├── normal.go
│   │   ├── pcg.go
│   │   ├── rand.go
│   │   └── zipf.go
│   ├── exp.go
│   ├── gen_cooked.go
│   ├── normal.go
│   ├── rand.go
│   ├── rng.go
│   └── zipf.go
├── abs.go
├── acos_s390x.s
├── acosh.go
├── acosh_s390x.s
├── arith_s390x.go
├── asin.go
├── asin_s390x.s
├── asinh.go
├── asinh_s390x.s
├── atan.go
├── atan2.go
├── atan2_s390x.s
├── atan_s390x.s
├── atanh.go
├── atanh_s390x.s
├── bits.go
├── cbrt.go
├── cbrt_s390x.s
├── const.go
├── copysign.go
├── cosh_s390x.s
├── dim.go
├── dim_amd64.s
├── dim_arm64.s
├── dim_asm.go
├── dim_loong64.s
├── dim_noasm.go
├── dim_riscv64.s
├── dim_s390x.s
├── erf.go
├── erf_s390x.s
├── erfc_s390x.s
├── erfinv.go
├── exp.go
├── exp2_asm.go
├── exp2_noasm.go
├── exp_amd64.go
├── exp_amd64.s
├── exp_arm64.s
├── exp_asm.go
├── exp_loong64.s
├── exp_noasm.go
├── exp_riscv64.s
├── exp_s390x.s
├── expm1.go
├── expm1_s390x.s
├── floor.go
├── floor_386.s
├── floor_amd64.s
├── floor_arm64.s
├── floor_asm.go
├── floor_loong64.s
├── floor_noasm.go
├── floor_ppc64x.s
├── floor_riscv64.s
├── floor_s390x.s
├── floor_wasm.s
├── fma.go
├── frexp.go
├── gamma.go
├── hypot.go
├── hypot_386.s
├── hypot_amd64.s
├── hypot_asm.go
├── hypot_noasm.go
├── j0.go
├── j1.go
├── jn.go
├── ldexp.go
├── lgamma.go
├── log.go
├── log10.go
├── log10_s390x.s
├── log1p.go
├── log1p_s390x.s
├── log_amd64.s
├── log_asm.go
├── log_s390x.s
├── log_stub.go
├── logb.go
├── mod.go
├── modf.go
├── nextafter.go
├── pow.go
├── pow10.go
├── pow_s390x.s
├── remainder.go
├── signbit.go
├── sin.go
├── sin_s390x.s
├── sincos.go
├── sinh.go
├── sinh_s390x.s
├── sqrt.go
├── stubs.go
├── stubs_s390x.s
├── tan.go
├── tan_s390x.s
├── tanh.go
├── tanh_s390x.s
├── trig_reduce.go
└── unsafe.go
```

## Source Stream Aggregation

// === FILE: references!/go/src/math/abs.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// Abs returns the absolute value of x.
//
// Special cases are:
//
//	Abs(±Inf) = +Inf
//	Abs(NaN) = NaN
func Abs(x float64) float64 {
	return Float64frombits(Float64bits(x) &^ signMask)
}

```

// === FILE: references!/go/src/math/acos_s390x.s ===
```text
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// Minimax polynomial coefficients and other constants
DATA ·acosrodataL13<> + 0(SB)/8, $0.314159265358979323E+01   //pi
DATA ·acosrodataL13<> + 8(SB)/8, $-0.0
DATA ·acosrodataL13<> + 16(SB)/8, $0x7ff8000000000000    //Nan
DATA ·acosrodataL13<> + 24(SB)/8, $-1.0
DATA ·acosrodataL13<> + 32(SB)/8, $1.0
DATA ·acosrodataL13<> + 40(SB)/8, $0.166666666666651626E+00
DATA ·acosrodataL13<> + 48(SB)/8, $0.750000000042621169E-01
DATA ·acosrodataL13<> + 56(SB)/8, $0.446428567178116477E-01
DATA ·acosrodataL13<> + 64(SB)/8, $0.303819660378071894E-01
DATA ·acosrodataL13<> + 72(SB)/8, $0.223715011892010405E-01
DATA ·acosrodataL13<> + 80(SB)/8, $0.173659424522364952E-01
DATA ·acosrodataL13<> + 88(SB)/8, $0.137810186504372266E-01
DATA ·acosrodataL13<> + 96(SB)/8, $0.134066870961173521E-01
DATA ·acosrodataL13<> + 104(SB)/8, $-.412335502831898721E-02
DATA ·acosrodataL13<> + 112(SB)/8, $0.867383739532082719E-01
DATA ·acosrodataL13<> + 120(SB)/8, $-.328765950607171649E+00
DATA ·acosrodataL13<> + 128(SB)/8, $0.110401073869414626E+01
DATA ·acosrodataL13<> + 136(SB)/8, $-.270694366992537307E+01
DATA ·acosrodataL13<> + 144(SB)/8, $0.500196500770928669E+01
DATA ·acosrodataL13<> + 152(SB)/8, $-.665866959108585165E+01
DATA ·acosrodataL13<> + 160(SB)/8, $-.344895269334086578E+01
DATA ·acosrodataL13<> + 168(SB)/8, $0.927437952918301659E+00
DATA ·acosrodataL13<> + 176(SB)/8, $0.610487478874645653E+01
DATA ·acosrodataL13<> + 184(SB)/8, $0.157079632679489656e+01
DATA ·acosrodataL13<> + 192(SB)/8, $0.0
GLOBL ·acosrodataL13<> + 0(SB), RODATA, $200

// Acos returns the arccosine, in radians, of the argument.
//
// Special case is:
//      Acos(x) = NaN if x < -1 or x > 1
// The algorithm used is minimax polynomial approximation
// with coefficients determined with a Remez exchange algorithm.

TEXT	·acosAsm(SB), NOSPLIT, $0-16
	FMOVD	x+0(FP), F0
	MOVD	$·acosrodataL13<>+0(SB), R9
	LGDR	F0, R12
	FMOVD	F0, F10
	SRAD	$32, R12
	WORD	$0xC0293FE6	//iilf	%r2,1072079005
	BYTE	$0xA0
	BYTE	$0x9D
	WORD	$0xB917001C	//llgtr	%r1,%r12
	CMPW	R1,R2
	BGT	L2
	FMOVD	192(R9), F8
	FMADD	F0, F0, F8
	FMOVD	184(R9), F1
L3:
	WFMDB	V8, V8, V2
	FMOVD	176(R9), F6
	FMOVD	168(R9), F0
	FMOVD	160(R9), F4
	WFMADB	V2, V0, V6, V0
	FMOVD	152(R9), F6
	WFMADB	V2, V4, V6, V4
	FMOVD	144(R9), F6
	WFMADB	V2, V0, V6, V0
	FMOVD	136(R9), F6
	WFMADB	V2, V4, V6, V4
	FMOVD	128(R9), F6
	WFMADB	V2, V0, V6, V0
	FMOVD	120(R9), F6
	WFMADB	V2, V4, V6, V4
	FMOVD	112(R9), F6
	WFMADB	V2, V0, V6, V0
	FMOVD	104(R9), F6
	WFMADB	V2, V4, V6, V4
	FMOVD	96(R9), F6
	WFMADB	V2, V0, V6, V0
	FMOVD	88(R9), F6
	WFMADB	V2, V4, V6, V4
	FMOVD	80(R9), F6
	WFMADB	V2, V0, V6, V0
	FMOVD	72(R9), F6
	WFMADB	V2, V4, V6, V4
	FMOVD	64(R9), F6
	WFMADB	V2, V0, V6, V0
	FMOVD	56(R9), F6
	WFMADB	V2, V4, V6, V4
	FMOVD	48(R9), F6
	WFMADB	V2, V0, V6, V0
	FMOVD	40(R9), F6
	WFMADB	V2, V4, V6, V2
	FMOVD	192(R9), F4
	WFMADB	V8, V0, V2, V0
	WFMADB	V10, V8, V4, V8
	FMADD	F0, F8, F10
	WFSDB	V10, V1, V10
L1:
	FMOVD	F10, ret+8(FP)
	RET

L2:
	WORD	$0xC0293FEF	//iilf	%r2,1072693247
	BYTE	$0xFF
	BYTE	$0xFF
	CMPW	R1, R2
	BLE	L12
L4:
	WORD	$0xED009020	//cdb	%f0,.L34-.L13(%r9)
	BYTE	$0x00
	BYTE	$0x19
	BEQ	L8
	WORD	$0xED009018	//cdb	%f0,.L35-.L13(%r9)
	BYTE	$0x00
	BYTE	$0x19
	BEQ	L9
	WFCEDBS	V10, V10, V0
	BVS	L1
	FMOVD	16(R9), F10
	BR	L1
L12:
	FMOVD	24(R9), F0
	FMADD	F10, F10, F0
	LCDBR	F0, F8
	WORD	$0xED009008	//cdb	%f0,.L37-.L13(%r9)
	BYTE	$0x00
	BYTE	$0x19
	FSQRT	F8, F10
L5:
	MOVW	R12, R4
	CMPBLE	R4, $0, L7
	LCDBR	F10, F10
	FMOVD	$0, F1
	BR	L3
L9:
	FMOVD	0(R9), F10
	BR	L1
L8:
	FMOVD	$0, F0
	FMOVD	F0, ret+8(FP)
	RET
L7:
	FMOVD	0(R9), F1
	BR	L3

```

// === FILE: references!/go/src/math/acosh.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// The original C code, the long comment, and the constants
// below are from FreeBSD's /usr/src/lib/msun/src/e_acosh.c
// and came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
//
// __ieee754_acosh(x)
// Method :
//	Based on
//	        acosh(x) = log [ x + sqrt(x*x-1) ]
//	we have
//	        acosh(x) := log(x)+ln2,	if x is large; else
//	        acosh(x) := log(2x-1/(sqrt(x*x-1)+x)) if x>2; else
//	        acosh(x) := log1p(t+sqrt(2.0*t+t*t)); where t=x-1.
//
// Special cases:
//	acosh(x) is NaN with signal if x<1.
//	acosh(NaN) is NaN without signal.
//

// Acosh returns the inverse hyperbolic cosine of x.
//
// Special cases are:
//
//	Acosh(+Inf) = +Inf
//	Acosh(x) = NaN if x < 1
//	Acosh(NaN) = NaN
func Acosh(x float64) float64 {
	if haveArchAcosh {
		return archAcosh(x)
	}
	return acosh(x)
}

func acosh(x float64) float64 {
	const Large = 1 << 28 // 2**28
	// first case is special case
	switch {
	case x < 1 || IsNaN(x):
		return NaN()
	case x == 1:
		return 0
	case x >= Large:
		return Log(x) + Ln2 // x > 2**28
	case x > 2:
		return Log(2*x - 1/(x+Sqrt(x*x-1))) // 2**28 > x > 2
	}
	t := x - 1
	return Log1p(t + Sqrt(2*t+t*t)) // 2 >= x > 1
}

```

// === FILE: references!/go/src/math/acosh_s390x.s ===
```text
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// Minimax polynomial coefficients and other constants
DATA ·acoshrodataL11<> + 0(SB)/8, $-1.0
DATA ·acoshrodataL11<> + 8(SB)/8, $.41375273347623353626
DATA ·acoshrodataL11<> + 16(SB)/8, $.51487302528619766235E+04
DATA ·acoshrodataL11<> + 24(SB)/8, $-1.67526912689208984375
DATA ·acoshrodataL11<> + 32(SB)/8, $0.181818181818181826E+00
DATA ·acoshrodataL11<> + 40(SB)/8, $-.165289256198351540E-01
DATA ·acoshrodataL11<> + 48(SB)/8, $0.200350613573012186E-02
DATA ·acoshrodataL11<> + 56(SB)/8, $-.273205381970859341E-03
DATA ·acoshrodataL11<> + 64(SB)/8, $0.397389654305194527E-04
DATA ·acoshrodataL11<> + 72(SB)/8, $0.938370938292558173E-06
DATA ·acoshrodataL11<> + 80(SB)/8, $-.602107458843052029E-05
DATA ·acoshrodataL11<> + 88(SB)/8, $0.212881813645679599E-07
DATA ·acoshrodataL11<> + 96(SB)/8, $-.148682720127920854E-06
DATA ·acoshrodataL11<> + 104(SB)/8, $-5.5
DATA ·acoshrodataL11<> + 112(SB)/8, $0x7ff8000000000000      //Nan
GLOBL ·acoshrodataL11<> + 0(SB), RODATA, $120

// Table of log correction terms
DATA ·acoshtab2068<> + 0(SB)/8, $0.585235384085551248E-01
DATA ·acoshtab2068<> + 8(SB)/8, $0.412206153771168640E-01
DATA ·acoshtab2068<> + 16(SB)/8, $0.273839003221648339E-01
DATA ·acoshtab2068<> + 24(SB)/8, $0.166383778368856480E-01
DATA ·acoshtab2068<> + 32(SB)/8, $0.866678223433169637E-02
DATA ·acoshtab2068<> + 40(SB)/8, $0.319831684989627514E-02
DATA ·acoshtab2068<> + 48(SB)/8, $0.0
DATA ·acoshtab2068<> + 56(SB)/8, $-.113006378583725549E-02
DATA ·acoshtab2068<> + 64(SB)/8, $-.367979419636602491E-03
DATA ·acoshtab2068<> + 72(SB)/8, $0.213172484510484979E-02
DATA ·acoshtab2068<> + 80(SB)/8, $0.623271047682013536E-02
DATA ·acoshtab2068<> + 88(SB)/8, $0.118140812789696885E-01
DATA ·acoshtab2068<> + 96(SB)/8, $0.187681358930914206E-01
DATA ·acoshtab2068<> + 104(SB)/8, $0.269985148668178992E-01
DATA ·acoshtab2068<> + 112(SB)/8, $0.364186619761331328E-01
DATA ·acoshtab2068<> + 120(SB)/8, $0.469505379381388441E-01
GLOBL ·acoshtab2068<> + 0(SB), RODATA, $128

// Acosh returns the inverse hyperbolic cosine of the argument.
//
// Special cases are:
//      Acosh(+Inf) = +Inf
//      Acosh(x) = NaN if x < 1
//      Acosh(NaN) = NaN
// The algorithm used is minimax polynomial approximation
// with coefficients determined with a Remez exchange algorithm.

TEXT	·acoshAsm(SB), NOSPLIT, $0-16
	FMOVD	x+0(FP), F0
	MOVD	$·acoshrodataL11<>+0(SB), R9
	LGDR	F0, R1
	WORD	$0xC0295FEF	//iilf	%r2,1609564159
	BYTE	$0xFF
	BYTE	$0xFF
	SRAD	$32, R1
	CMPW	R1, R2
	BGT	L2
	WORD	$0xC0293FEF	//iilf	%r2,1072693247
	BYTE	$0xFF
	BYTE	$0xFF
	CMPW	R1, R2
	BGT	L10
L3:
	WFCEDBS	V0, V0, V2
	BVS	L1
	FMOVD	112(R9), F0
L1:
	FMOVD	F0, ret+8(FP)
	RET
L2:
	WORD	$0xC0297FEF	//iilf	%r2,2146435071
	BYTE	$0xFF
	BYTE	$0xFF
	MOVW	R1, R6
	MOVW	R2, R7
	CMPBGT	R6, R7, L1
	FMOVD	F0, F8
	FMOVD	$0, F0
	WFADB	V0, V8, V0
	WORD	$0xC0398006	//iilf	%r3,2147909631
	BYTE	$0x7F
	BYTE	$0xFF
	LGDR	F0, R5
	SRAD	$32, R5
	MOVH	$0x0, R1
	SUBW	R5, R3
	FMOVD	$0, F10
	RISBGZ	$32, $47, $0, R3, R4
	RISBGZ	$57, $60, $51, R3, R3
	BYTE	$0x18	//lr	%r2,%r4
	BYTE	$0x24
	RISBGN	$0, $31, $32, R4, R1
	SUBW	$0x100000, R2
	SRAW	$8, R2, R2
	ORW	$0x45000000, R2
L5:
	LDGR	R1, F0
	FMOVD	104(R9), F2
	FMADD	F8, F0, F2
	FMOVD	96(R9), F4
	WFMADB	V10, V0, V2, V0
	FMOVD	88(R9), F6
	FMOVD	80(R9), F2
	WFMADB	V0, V6, V4, V6
	FMOVD	72(R9), F1
	WFMDB	V0, V0, V4
	WFMADB	V0, V1, V2, V1
	FMOVD	64(R9), F2
	WFMADB	V6, V4, V1, V6
	FMOVD	56(R9), F1
	RISBGZ	$57, $60, $0, R3, R3
	WFMADB	V0, V2, V1, V2
	FMOVD	48(R9), F1
	WFMADB	V4, V6, V2, V6
	FMOVD	40(R9), F2
	WFMADB	V0, V1, V2, V1
	VLVGF	$0, R2, V2
	WFMADB	V4, V6, V1, V4
	LDEBR	F2, F2
	FMOVD	32(R9), F6
	WFMADB	V0, V4, V6, V4
	FMOVD	24(R9), F1
	FMOVD	16(R9), F6
	MOVD	$·acoshtab2068<>+0(SB), R1
	WFMADB	V2, V1, V6, V2
	FMOVD	0(R3)(R1*1), F3
	WFMADB	V0, V4, V3, V0
	FMOVD	8(R9), F4
	FMADD	F4, F2, F0
	FMOVD	F0, ret+8(FP)
	RET
L10:
	FMOVD	F0, F8
	FMOVD	0(R9), F0
	FMADD	F8, F8, F0
	LTDBR	F0, F0
	FSQRT	F0, F10
L4:
	WFADB	V10, V8, V0
	WORD	$0xC0398006	//iilf	%r3,2147909631
	BYTE	$0x7F
	BYTE	$0xFF
	LGDR	F0, R5
	SRAD	$32, R5
	MOVH	$0x0, R1
	SUBW	R5, R3
	SRAW	$8, R3, R2
	RISBGZ	$32, $47, $0, R3, R4
	ANDW	$0xFFFFFF00, R2
	RISBGZ	$57, $60, $51, R3, R3
	ORW	$0x45000000, R2
	RISBGN	$0, $31, $32, R4, R1
	BR	L5

```

// === FILE: references!/go/src/math/arith_s390x.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

import "internal/cpu"

func expTrampolineSetup(x float64) float64
func expAsm(x float64) float64

func logTrampolineSetup(x float64) float64
func logAsm(x float64) float64

// Below here all functions are grouped in stubs.go for other
// architectures.

const haveArchLog10 = true

func archLog10(x float64) float64
func log10TrampolineSetup(x float64) float64
func log10Asm(x float64) float64

const haveArchCos = true

func archCos(x float64) float64
func cosTrampolineSetup(x float64) float64
func cosAsm(x float64) float64

const haveArchCosh = true

func archCosh(x float64) float64
func coshTrampolineSetup(x float64) float64
func coshAsm(x float64) float64

const haveArchSin = true

func archSin(x float64) float64
func sinTrampolineSetup(x float64) float64
func sinAsm(x float64) float64

const haveArchSinh = true

func archSinh(x float64) float64
func sinhTrampolineSetup(x float64) float64
func sinhAsm(x float64) float64

const haveArchTanh = true

func archTanh(x float64) float64
func tanhTrampolineSetup(x float64) float64
func tanhAsm(x float64) float64

const haveArchLog1p = true

func archLog1p(x float64) float64
func log1pTrampolineSetup(x float64) float64
func log1pAsm(x float64) float64

const haveArchAtanh = true

func archAtanh(x float64) float64
func atanhTrampolineSetup(x float64) float64
func atanhAsm(x float64) float64

const haveArchAcos = true

func archAcos(x float64) float64
func acosTrampolineSetup(x float64) float64
func acosAsm(x float64) float64

const haveArchAcosh = true

func archAcosh(x float64) float64
func acoshTrampolineSetup(x float64) float64
func acoshAsm(x float64) float64

const haveArchAsin = true

func archAsin(x float64) float64
func asinTrampolineSetup(x float64) float64
func asinAsm(x float64) float64

const haveArchAsinh = true

func archAsinh(x float64) float64
func asinhTrampolineSetup(x float64) float64
func asinhAsm(x float64) float64

const haveArchErf = true

func archErf(x float64) float64
func erfTrampolineSetup(x float64) float64
func erfAsm(x float64) float64

const haveArchErfc = true

func archErfc(x float64) float64
func erfcTrampolineSetup(x float64) float64
func erfcAsm(x float64) float64

const haveArchAtan = true

func archAtan(x float64) float64
func atanTrampolineSetup(x float64) float64
func atanAsm(x float64) float64

const haveArchAtan2 = true

func archAtan2(y, x float64) float64
func atan2TrampolineSetup(x, y float64) float64
func atan2Asm(x, y float64) float64

const haveArchCbrt = true

func archCbrt(x float64) float64
func cbrtTrampolineSetup(x float64) float64
func cbrtAsm(x float64) float64

const haveArchTan = true

func archTan(x float64) float64
func tanTrampolineSetup(x float64) float64
func tanAsm(x float64) float64

const haveArchExpm1 = true

func archExpm1(x float64) float64
func expm1TrampolineSetup(x float64) float64
func expm1Asm(x float64) float64

const haveArchPow = false

func archPow(x, y float64) float64
func powTrampolineSetup(x, y float64) float64
func powAsm(x, y float64) float64

const haveArchFrexp = false

func archFrexp(x float64) (float64, int) {
	panic("not implemented")
}

const haveArchLdexp = false

func archLdexp(frac float64, exp int) float64 {
	panic("not implemented")
}

const haveArchLog2 = false

func archLog2(x float64) float64 {
	panic("not implemented")
}

const haveArchMod = false

func archMod(x, y float64) float64 {
	panic("not implemented")
}

const haveArchRemainder = false

func archRemainder(x, y float64) float64 {
	panic("not implemented")
}

// hasVX reports whether the machine has the z/Architecture
// vector facility installed and enabled.
var hasVX = cpu.S390X.HasVX

```

// === FILE: references!/go/src/math/asin.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

/*
	Floating-point arcsine and arccosine.

	They are implemented by computing the arctangent
	after appropriate range reduction.
*/

// Asin returns the arcsine, in radians, of x.
//
// Special cases are:
//
//	Asin(±0) = ±0
//	Asin(x) = NaN if x < -1 or x > 1
func Asin(x float64) float64 {
	if haveArchAsin {
		return archAsin(x)
	}
	return asin(x)
}

func asin(x float64) float64 {
	if x == 0 {
		return x // special case
	}
	sign := false
	if x < 0 {
		x = -x
		sign = true
	}
	if x > 1 {
		return NaN() // special case
	}

	temp := Sqrt(1 - x*x)
	if x > 0.7 {
		temp = Pi/2 - satan(temp/x)
	} else {
		temp = satan(x / temp)
	}

	if sign {
		temp = -temp
	}
	return temp
}

// Acos returns the arccosine, in radians, of x.
//
// Special case is:
//
//	Acos(x) = NaN if x < -1 or x > 1
func Acos(x float64) float64 {
	if haveArchAcos {
		return archAcos(x)
	}
	return acos(x)
}

func acos(x float64) float64 {
	return Pi/2 - Asin(x)
}

```

// === FILE: references!/go/src/math/asin_s390x.s ===
```text
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// Minimax polynomial coefficients and other constants
DATA ·asinrodataL15<> + 0(SB)/8, $-1.309611320495605469
DATA ·asinrodataL15<> + 8(SB)/8, $0x3ff921fb54442d18
DATA ·asinrodataL15<> + 16(SB)/8, $0xbff921fb54442d18
DATA ·asinrodataL15<> + 24(SB)/8, $1.309611320495605469
DATA ·asinrodataL15<> + 32(SB)/8, $-0.0
DATA ·asinrodataL15<> + 40(SB)/8, $1.199437040755305217
DATA ·asinrodataL15<> + 48(SB)/8, $0.166666666666651626E+00
DATA ·asinrodataL15<> + 56(SB)/8, $0.750000000042621169E-01
DATA ·asinrodataL15<> + 64(SB)/8, $0.446428567178116477E-01
DATA ·asinrodataL15<> + 72(SB)/8, $0.303819660378071894E-01
DATA ·asinrodataL15<> + 80(SB)/8, $0.223715011892010405E-01
DATA ·asinrodataL15<> + 88(SB)/8, $0.173659424522364952E-01
DATA ·asinrodataL15<> + 96(SB)/8, $0.137810186504372266E-01
DATA ·asinrodataL15<> + 104(SB)/8, $0.134066870961173521E-01
DATA ·asinrodataL15<> + 112(SB)/8, $-.412335502831898721E-02
DATA ·asinrodataL15<> + 120(SB)/8, $0.867383739532082719E-01
DATA ·asinrodataL15<> + 128(SB)/8, $-.328765950607171649E+00
DATA ·asinrodataL15<> + 136(SB)/8, $0.110401073869414626E+01
DATA ·asinrodataL15<> + 144(SB)/8, $-.270694366992537307E+01
DATA ·asinrodataL15<> + 152(SB)/8, $0.500196500770928669E+01
DATA ·asinrodataL15<> + 160(SB)/8, $-.665866959108585165E+01
DATA ·asinrodataL15<> + 168(SB)/8, $-.344895269334086578E+01
DATA ·asinrodataL15<> + 176(SB)/8, $0.927437952918301659E+00
DATA ·asinrodataL15<> + 184(SB)/8, $0.610487478874645653E+01
DATA ·asinrodataL15<> + 192(SB)/8, $0x7ff8000000000000			//+Inf
DATA ·asinrodataL15<> + 200(SB)/8, $-1.0
DATA ·asinrodataL15<> + 208(SB)/8, $1.0
DATA ·asinrodataL15<> + 216(SB)/8, $1.00000000000000000e-20
GLOBL ·asinrodataL15<> + 0(SB), RODATA, $224

// Asin returns the arcsine, in radians, of the argument.
//
// Special cases are:
//      Asin(±0) = ±0=
//      Asin(x) = NaN if x < -1 or x > 1
// The algorithm used is minimax polynomial approximation
// with coefficients determined with a Remez exchange algorithm.

TEXT	·asinAsm(SB), NOSPLIT, $0-16
	FMOVD	x+0(FP), F0
	MOVD	$·asinrodataL15<>+0(SB), R9
	LGDR	F0, R7
	FMOVD	F0, F8
	SRAD	$32, R7
	WORD	$0xC0193FE6 //iilf  %r1,1072079005
	BYTE	$0xA0
	BYTE	$0x9D
	WORD	$0xB91700C7 //llgtr %r12,%r7
	MOVW	R12, R8
	MOVW	R1, R6
	CMPBGT	R8, R6, L2
	WORD	$0xC0193BFF //iilf  %r1,1006632959
	BYTE	$0xFF
	BYTE	$0xFF
	MOVW	R1, R6
	CMPBGT	R8, R6, L13
L3:
	FMOVD	216(R9), F0
	FMADD	F0, F8, F8
L1:
	FMOVD	F8, ret+8(FP)
	RET
L2:
	WORD	$0xC0193FEF	//iilf	%r1,1072693247
	BYTE	$0xFF
	BYTE	$0xFF
	CMPW	R12, R1
	BLE	L14
L5:
	WORD	$0xED0090D0	//cdb	%f0,.L17-.L15(%r9)
	BYTE	$0x00
	BYTE	$0x19
	BEQ		L9
	WORD	$0xED0090C8	//cdb	%f0,.L18-.L15(%r9)
	BYTE	$0x00
	BYTE	$0x19
	BEQ	L10
	WFCEDBS	V8, V8, V0
	BVS	L1
	FMOVD	192(R9), F8
	BR	L1
L13:
	WFMDB	V0, V0, V10
L4:
	WFMDB	V10, V10, V0
	FMOVD	184(R9), F6
	FMOVD	176(R9), F2
	FMOVD	168(R9), F4
	WFMADB	V0, V2, V6, V2
	FMOVD	160(R9), F6
	WFMADB	V0, V4, V6, V4
	FMOVD	152(R9), F6
	WFMADB	V0, V2, V6, V2
	FMOVD	144(R9), F6
	WFMADB	V0, V4, V6, V4
	FMOVD	136(R9), F6
	WFMADB	V0, V2, V6, V2
	WORD	$0xC0193FE6	//iilf	%r1,1072079005
	BYTE	$0xA0
	BYTE	$0x9D
	FMOVD	128(R9), F6
	WFMADB	V0, V4, V6, V4
	FMOVD	120(R9), F6
	WFMADB	V0, V2, V6, V2
	FMOVD	112(R9), F6
	WFMADB	V0, V4, V6, V4
	FMOVD	104(R9), F6
	WFMADB	V0, V2, V6, V2
	FMOVD	96(R9), F6
	WFMADB	V0, V4, V6, V4
	FMOVD	88(R9), F6
	WFMADB	V0, V2, V6, V2
	FMOVD	80(R9), F6
	WFMADB	V0, V4, V6, V4
	FMOVD	72(R9), F6
	WFMADB	V0, V2, V6, V2
	FMOVD	64(R9), F6
	WFMADB	V0, V4, V6, V4
	FMOVD	56(R9), F6
	WFMADB	V0, V2, V6, V2
	FMOVD	48(R9), F6
	WFMADB	V0, V4, V6, V0
	WFMDB	V8, V10, V4
	FMADD	F2, F10, F0
	FMADD	F0, F4, F8
	CMPW	R12, R1
	BLE	L1
	FMOVD	40(R9), F0
	FMADD	F0, F1, F8
	FMOVD	F8, ret+8(FP)
	RET
L14:
	FMOVD	200(R9), F0
	FMADD	F8, F8, F0
	LCDBR	F0, F10
	WORD	$0xED009020	//cdb	%f0,.L39-.L15(%r9)
	BYTE	$0x00
	BYTE	$0x19
	FSQRT	F10, F8
L6:
	MOVW	R7, R6
	CMPBLE	R6, $0, L8
	LCDBR	F8, F8
	FMOVD	24(R9), F1
	BR	L4
L10:
	FMOVD	16(R9), F8
	BR	L1
L9:
	FMOVD	8(R9), F8
	FMOVD	F8, ret+8(FP)
	RET
L8:
	FMOVD	0(R9), F1
	BR	L4

```

// === FILE: references!/go/src/math/asinh.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// The original C code, the long comment, and the constants
// below are from FreeBSD's /usr/src/lib/msun/src/s_asinh.c
// and came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
//
// asinh(x)
// Method :
//	Based on
//	        asinh(x) = sign(x) * log [ |x| + sqrt(x*x+1) ]
//	we have
//	asinh(x) := x  if  1+x*x=1,
//	         := sign(x)*(log(x)+ln2) for large |x|, else
//	         := sign(x)*log(2|x|+1/(|x|+sqrt(x*x+1))) if|x|>2, else
//	         := sign(x)*log1p(|x| + x**2/(1 + sqrt(1+x**2)))
//

// Asinh returns the inverse hyperbolic sine of x.
//
// Special cases are:
//
//	Asinh(±0) = ±0
//	Asinh(±Inf) = ±Inf
//	Asinh(NaN) = NaN
func Asinh(x float64) float64 {
	if haveArchAsinh {
		return archAsinh(x)
	}
	return asinh(x)
}

func asinh(x float64) float64 {
	const (
		Ln2      = 6.93147180559945286227e-01 // 0x3FE62E42FEFA39EF
		NearZero = 1.0 / (1 << 28)            // 2**-28
		Large    = 1 << 28                    // 2**28
	)
	// special cases
	if IsNaN(x) || IsInf(x, 0) {
		return x
	}
	sign := false
	if x < 0 {
		x = -x
		sign = true
	}
	var temp float64
	switch {
	case x > Large:
		temp = Log(x) + Ln2 // |x| > 2**28
	case x > 2:
		temp = Log(2*x + 1/(Sqrt(x*x+1)+x)) // 2**28 > |x| > 2.0
	case x < NearZero:
		temp = x // |x| < 2**-28
	default:
		temp = Log1p(x + x*x/(1+Sqrt(1+x*x))) // 2.0 > |x| > 2**-28
	}
	if sign {
		temp = -temp
	}
	return temp
}

```

// === FILE: references!/go/src/math/asinh_s390x.s ===
```text
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// Minimax polynomial coefficients and other constants
DATA ·asinhrodataL18<> + 0(SB)/8, $0.749999999977387502E-01
DATA ·asinhrodataL18<> + 8(SB)/8, $-.166666666666657082E+00
DATA ·asinhrodataL18<> + 16(SB)/8, $0.303819368237360639E-01
DATA ·asinhrodataL18<> + 24(SB)/8, $-.446428569571752982E-01
DATA ·asinhrodataL18<> + 32(SB)/8, $0.173500047922695924E-01
DATA ·asinhrodataL18<> + 40(SB)/8, $-.223719767210027185E-01
DATA ·asinhrodataL18<> + 48(SB)/8, $0.113655037946822130E-01
DATA ·asinhrodataL18<> + 56(SB)/8, $0.579747490622448943E-02
DATA ·asinhrodataL18<> + 64(SB)/8, $-.139372433914359122E-01
DATA ·asinhrodataL18<> + 72(SB)/8, $-.218674325255800840E-02
DATA ·asinhrodataL18<> + 80(SB)/8, $-.891074277756961157E-02
DATA ·asinhrodataL18<> + 88(SB)/8, $.41375273347623353626
DATA ·asinhrodataL18<> + 96(SB)/8, $.51487302528619766235E+04
DATA ·asinhrodataL18<> + 104(SB)/8, $-1.67526912689208984375
DATA ·asinhrodataL18<> + 112(SB)/8, $0.181818181818181826E+00
DATA ·asinhrodataL18<> + 120(SB)/8, $-.165289256198351540E-01
DATA ·asinhrodataL18<> + 128(SB)/8, $0.200350613573012186E-02
DATA ·asinhrodataL18<> + 136(SB)/8, $-.273205381970859341E-03
DATA ·asinhrodataL18<> + 144(SB)/8, $0.397389654305194527E-04
DATA ·asinhrodataL18<> + 152(SB)/8, $0.938370938292558173E-06
DATA ·asinhrodataL18<> + 160(SB)/8, $0.212881813645679599E-07
DATA ·asinhrodataL18<> + 168(SB)/8, $-.602107458843052029E-05
DATA ·asinhrodataL18<> + 176(SB)/8, $-.148682720127920854E-06
DATA ·asinhrodataL18<> + 184(SB)/8, $-5.5
DATA ·asinhrodataL18<> + 192(SB)/8, $1.0
DATA ·asinhrodataL18<> + 200(SB)/8, $1.0E-20
GLOBL ·asinhrodataL18<> + 0(SB), RODATA, $208

// Table of log correction terms
DATA ·asinhtab2080<> + 0(SB)/8, $0.585235384085551248E-01
DATA ·asinhtab2080<> + 8(SB)/8, $0.412206153771168640E-01
DATA ·asinhtab2080<> + 16(SB)/8, $0.273839003221648339E-01
DATA ·asinhtab2080<> + 24(SB)/8, $0.166383778368856480E-01
DATA ·asinhtab2080<> + 32(SB)/8, $0.866678223433169637E-02
DATA ·asinhtab2080<> + 40(SB)/8, $0.319831684989627514E-02
DATA ·asinhtab2080<> + 48(SB)/8, $0.0
DATA ·asinhtab2080<> + 56(SB)/8, $-.113006378583725549E-02
DATA ·asinhtab2080<> + 64(SB)/8, $-.367979419636602491E-03
DATA ·asinhtab2080<> + 72(SB)/8, $0.213172484510484979E-02
DATA ·asinhtab2080<> + 80(SB)/8, $0.623271047682013536E-02
DATA ·asinhtab2080<> + 88(SB)/8, $0.118140812789696885E-01
DATA ·asinhtab2080<> + 96(SB)/8, $0.187681358930914206E-01
DATA ·asinhtab2080<> + 104(SB)/8, $0.269985148668178992E-01
DATA ·asinhtab2080<> + 112(SB)/8, $0.364186619761331328E-01
DATA ·asinhtab2080<> + 120(SB)/8, $0.469505379381388441E-01
GLOBL ·asinhtab2080<> + 0(SB), RODATA, $128

// Asinh returns the inverse hyperbolic sine of the argument.
//
// Special cases are:
//      Asinh(±0) = ±0
//      Asinh(±Inf) = ±Inf
//      Asinh(NaN) = NaN
// The algorithm used is minimax polynomial approximation
// with coefficients determined with a Remez exchange algorithm.

TEXT	·asinhAsm(SB), NOSPLIT, $0-16
	FMOVD	x+0(FP), F0
	MOVD	$·asinhrodataL18<>+0(SB), R9
	LGDR	F0, R12
	WORD	$0xC0293FDF	//iilf	%r2,1071644671
	BYTE	$0xFF
	BYTE	$0xFF
	SRAD	$32, R12
	WORD	$0xB917001C	//llgtr	%r1,%r12
	MOVW	R1, R6
	MOVW	R2, R7
	CMPBLE	R6, R7, L2
	WORD	$0xC0295FEF	//iilf	%r2,1609564159
	BYTE	$0xFF
	BYTE	$0xFF
	MOVW	R2, R7
	CMPBLE	R6, R7, L14
L3:
	WORD	$0xC0297FEF	//iilf	%r2,2146435071
	BYTE	$0xFF
	BYTE	$0xFF
	CMPW	R1, R2
	BGT	L1
	LTDBR	F0, F0
	FMOVD	F0, F10
	BLTU	L15
L9:
	FMOVD	$0, F0
	WFADB	V0, V10, V0
	WORD	$0xC0398006	//iilf	%r3,2147909631
	BYTE	$0x7F
	BYTE	$0xFF
	LGDR	F0, R5
	SRAD	$32, R5
	MOVH	$0x0, R2
	SUBW	R5, R3
	FMOVD	$0, F8
	RISBGZ	$32, $47, $0, R3, R4
	BYTE	$0x18	//lr	%r1,%r4
	BYTE	$0x14
	RISBGN	$0, $31, $32, R4, R2
	SUBW	$0x100000, R1
	SRAW	$8, R1, R1
	ORW	$0x45000000, R1
	BR	L6
L2:
	MOVD	$0x30000000, R2
	CMPW	R1, R2
	BGT	L16
	FMOVD	200(R9), F2
	FMADD	F2, F0, F0
L1:
	FMOVD	F0, ret+8(FP)
	RET
L14:
	LTDBR	F0, F0
	BLTU	L17
	FMOVD	F0, F10
L4:
	FMOVD	192(R9), F2
	WFMADB	V0, V0, V2, V0
	LTDBR	F0, F0
	FSQRT	F0, F8
L5:
	WFADB	V8, V10, V0
	WORD	$0xC0398006	//iilf	%r3,2147909631
	BYTE	$0x7F
	BYTE	$0xFF
	LGDR	F0, R5
	SRAD	$32, R5
	MOVH	$0x0, R2
	SUBW	R5, R3
	RISBGZ	$32, $47, $0, R3, R4
	SRAW	$8, R4, R1
	RISBGN	$0, $31, $32, R4, R2
	ORW	$0x45000000, R1
L6:
	LDGR	R2, F2
	FMOVD	184(R9), F0
	WFMADB	V8, V2, V0, V8
	FMOVD	176(R9), F4
	WFMADB	V10, V2, V8, V2
	FMOVD	168(R9), F0
	FMOVD	160(R9), F6
	FMOVD	152(R9), F1
	WFMADB	V2, V6, V4, V6
	WFMADB	V2, V1, V0, V1
	WFMDB	V2, V2, V4
	FMOVD	144(R9), F0
	WFMADB	V6, V4, V1, V6
	FMOVD	136(R9), F1
	RISBGZ	$57, $60, $51, R3, R3
	WFMADB	V2, V0, V1, V0
	FMOVD	128(R9), F1
	WFMADB	V4, V6, V0, V6
	FMOVD	120(R9), F0
	WFMADB	V2, V1, V0, V1
	VLVGF	$0, R1, V0
	WFMADB	V4, V6, V1, V4
	LDEBR	F0, F0
	FMOVD	112(R9), F6
	WFMADB	V2, V4, V6, V4
	MOVD	$·asinhtab2080<>+0(SB), R1
	FMOVD	104(R9), F1
	WORD	$0x68331000	//ld	%f3,0(%r3,%r1)
	FMOVD	96(R9), F6
	WFMADB	V2, V4, V3, V2
	WFMADB	V0, V1, V6, V0
	FMOVD	88(R9), F4
	WFMADB	V0, V4, V2, V0
	MOVD	R12, R6
	CMPBGT	R6, $0, L1

	LCDBR	F0, F0
	FMOVD	F0, ret+8(FP)
	RET
L16:
	WFMDB	V0, V0, V1
	FMOVD	80(R9), F6
	WFMDB	V1, V1, V4
	FMOVD	72(R9), F2
	WFMADB	V4, V2, V6, V2
	FMOVD	64(R9), F3
	FMOVD	56(R9), F6
	WFMADB	V4, V2, V3, V2
	FMOVD	48(R9), F3
	WFMADB	V4, V6, V3, V6
	FMOVD	40(R9), F5
	FMOVD	32(R9), F3
	WFMADB	V4, V2, V5, V2
	WFMADB	V4, V6, V3, V6
	FMOVD	24(R9), F5
	FMOVD	16(R9), F3
	WFMADB	V4, V2, V5, V2
	WFMADB	V4, V6, V3, V6
	FMOVD	8(R9), F5
	FMOVD	0(R9), F3
	WFMADB	V4, V2, V5, V2
	WFMADB	V4, V6, V3, V4
	WFMDB	V0, V1, V6
	WFMADB	V1, V4, V2, V4
	FMADD	F4, F6, F0
	FMOVD	F0, ret+8(FP)
	RET
L17:
	LCDBR	F0, F10
	BR	L4
L15:
	LCDBR	F0, F10
	BR	L9

```

// === FILE: references!/go/src/math/atan.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

/*
	Floating-point arctangent.
*/

// The original C code, the long comment, and the constants below were
// from http://netlib.sandia.gov/cephes/cmath/atan.c, available from
// http://www.netlib.org/cephes/cmath.tgz.
// The go code is a version of the original C.
//
// atan.c
// Inverse circular tangent (arctangent)
//
// SYNOPSIS:
// double x, y, atan();
// y = atan( x );
//
// DESCRIPTION:
// Returns radian angle between -pi/2 and +pi/2 whose tangent is x.
//
// Range reduction is from three intervals into the interval from zero to 0.66.
// The approximant uses a rational function of degree 4/5 of the form
// x + x**3 P(x)/Q(x).
//
// ACCURACY:
//                      Relative error:
// arithmetic   domain    # trials  peak     rms
//    DEC       -10, 10   50000     2.4e-17  8.3e-18
//    IEEE      -10, 10   10^6      1.8e-16  5.0e-17
//
// Cephes Math Library Release 2.8:  June, 2000
// Copyright 1984, 1987, 1989, 1992, 2000 by Stephen L. Moshier
//
// The readme file at http://netlib.sandia.gov/cephes/ says:
//    Some software in this archive may be from the book _Methods and
// Programs for Mathematical Functions_ (Prentice-Hall or Simon & Schuster
// International, 1989) or from the Cephes Mathematical Library, a
// commercial product. In either event, it is copyrighted by the author.
// What you see here may be used freely but it comes with no support or
// guarantee.
//
//   The two known misprints in the book are repaired here in the
// source listings for the gamma function and the incomplete beta
// integral.
//
//   Stephen L. Moshier
//   moshier@na-net.ornl.gov

// xatan evaluates a series valid in the range [0, 0.66].
func xatan(x float64) float64 {
	const (
		P0 = -8.750608600031904122785e-01
		P1 = -1.615753718733365076637e+01
		P2 = -7.500855792314704667340e+01
		P3 = -1.228866684490136173410e+02
		P4 = -6.485021904942025371773e+01
		Q0 = +2.485846490142306297962e+01
		Q1 = +1.650270098316988542046e+02
		Q2 = +4.328810604912902668951e+02
		Q3 = +4.853903996359136964868e+02
		Q4 = +1.945506571482613964425e+02
	)
	z := x * x
	z = z * ((((P0*z+P1)*z+P2)*z+P3)*z + P4) / (((((z+Q0)*z+Q1)*z+Q2)*z+Q3)*z + Q4)
	z = x*z + x
	return z
}

// satan reduces its argument (known to be positive)
// to the range [0, 0.66] and calls xatan.
func satan(x float64) float64 {
	const (
		Morebits = 6.123233995736765886130e-17 // pi/2 = PIO2 + Morebits
		Tan3pio8 = 2.41421356237309504880      // tan(3*pi/8)
	)
	if x <= 0.66 {
		return xatan(x)
	}
	if x > Tan3pio8 {
		return Pi/2 - xatan(1/x) + Morebits
	}
	return Pi/4 + xatan((x-1)/(x+1)) + 0.5*Morebits
}

// Atan returns the arctangent, in radians, of x.
//
// Special cases are:
//
//	Atan(±0) = ±0
//	Atan(±Inf) = ±Pi/2
func Atan(x float64) float64 {
	if haveArchAtan {
		return archAtan(x)
	}
	return atan(x)
}

func atan(x float64) float64 {
	if x == 0 {
		return x
	}
	if x > 0 {
		return satan(x)
	}
	return -satan(-x)
}

```

// === FILE: references!/go/src/math/atan2.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// Atan2 returns the arc tangent of y/x, using
// the signs of the two to determine the quadrant
// of the return value.
//
// Special cases are (in order):
//
//	Atan2(y, NaN) = NaN
//	Atan2(NaN, x) = NaN
//	Atan2(+0, x>=0) = +0
//	Atan2(-0, x>=0) = -0
//	Atan2(+0, x<=-0) = +Pi
//	Atan2(-0, x<=-0) = -Pi
//	Atan2(y>0, 0) = +Pi/2
//	Atan2(y<0, 0) = -Pi/2
//	Atan2(+Inf, +Inf) = +Pi/4
//	Atan2(-Inf, +Inf) = -Pi/4
//	Atan2(+Inf, -Inf) = 3Pi/4
//	Atan2(-Inf, -Inf) = -3Pi/4
//	Atan2(y, +Inf) = 0
//	Atan2(y>0, -Inf) = +Pi
//	Atan2(y<0, -Inf) = -Pi
//	Atan2(+Inf, x) = +Pi/2
//	Atan2(-Inf, x) = -Pi/2
func Atan2(y, x float64) float64 {
	if haveArchAtan2 {
		return archAtan2(y, x)
	}
	return atan2(y, x)
}

func atan2(y, x float64) float64 {
	// special cases
	switch {
	case IsNaN(y) || IsNaN(x):
		return NaN()
	case y == 0:
		if x >= 0 && !Signbit(x) {
			return Copysign(0, y)
		}
		return Copysign(Pi, y)
	case x == 0:
		return Copysign(Pi/2, y)
	case IsInf(x, 0):
		if IsInf(x, 1) {
			switch {
			case IsInf(y, 0):
				return Copysign(Pi/4, y)
			default:
				return Copysign(0, y)
			}
		}
		switch {
		case IsInf(y, 0):
			return Copysign(3*Pi/4, y)
		default:
			return Copysign(Pi, y)
		}
	case IsInf(y, 0):
		return Copysign(Pi/2, y)
	}

	// Call atan and determine the quadrant.
	q := Atan(y / x)
	if x < 0 {
		if q <= 0 {
			return q + Pi
		}
		return q - Pi
	}
	return q
}

```

// === FILE: references!/go/src/math/atan2_s390x.s ===
```text
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

#define PosInf		0x7FF0000000000000
#define NegInf		0xFFF0000000000000
#define NegZero		0x8000000000000000
#define Pi		0x400921FB54442D18
#define NegPi		0xC00921FB54442D18
#define Pi3Div4		0x4002D97C7F3321D2	// 3Pi/4
#define NegPi3Div4	0xC002D97C7F3321D2	// -3Pi/4
#define PiDiv4		0x3FE921FB54442D18	// Pi/4
#define NegPiDiv4	0xBFE921FB54442D18	// -Pi/4

// Minimax polynomial coefficients and other constants
DATA ·atan2rodataL25<> + 0(SB)/8, $0.199999999999554423E+00
DATA ·atan2rodataL25<> + 8(SB)/8, $-.333333333333330928E+00
DATA ·atan2rodataL25<> + 16(SB)/8, $0.111111110136634272E+00
DATA ·atan2rodataL25<> + 24(SB)/8, $-.142857142828026806E+00
DATA ·atan2rodataL25<> + 32(SB)/8, $0.769228118888682505E-01
DATA ·atan2rodataL25<> + 40(SB)/8, $0.588059263575587687E-01
DATA ·atan2rodataL25<> + 48(SB)/8, $-.909090711945939878E-01
DATA ·atan2rodataL25<> + 56(SB)/8, $-.666641501287528609E-01
DATA ·atan2rodataL25<> + 64(SB)/8, $0.472329433805024762E-01
DATA ·atan2rodataL25<> + 72(SB)/8, $-.525380587584426406E-01
DATA ·atan2rodataL25<> + 80(SB)/8, $-.422172007412067035E-01
DATA ·atan2rodataL25<> + 88(SB)/8, $0.366935664549587481E-01
DATA ·atan2rodataL25<> + 96(SB)/8, $0.220852012160300086E-01
DATA ·atan2rodataL25<> + 104(SB)/8, $-.299856214685512712E-01
DATA ·atan2rodataL25<> + 112(SB)/8, $0.726338160757602439E-02
DATA ·atan2rodataL25<> + 120(SB)/8, $0.134893651284712515E-04
DATA ·atan2rodataL25<> + 128(SB)/8, $-.291935324869629616E-02
DATA ·atan2rodataL25<> + 136(SB)/8, $-.154797890856877418E-03
DATA ·atan2rodataL25<> + 144(SB)/8, $0.843488472994227321E-03
DATA ·atan2rodataL25<> + 152(SB)/8, $-.139950258898989925E-01
GLOBL ·atan2rodataL25<> + 0(SB), RODATA, $160

DATA ·atan2xpi2h<> + 0(SB)/8, $0x3ff330e4e4fa7b1b
DATA ·atan2xpi2h<> + 8(SB)/8, $0xbff330e4e4fa7b1b
DATA ·atan2xpi2h<> + 16(SB)/8, $0x400330e4e4fa7b1b
DATA ·atan2xpi2h<> + 24(SB)/8, $0xc00330e4e4fa7b1b
GLOBL ·atan2xpi2h<> + 0(SB), RODATA, $32
DATA ·atan2xpim<> + 0(SB)/8, $0x3ff4f42b00000000
GLOBL ·atan2xpim<> + 0(SB), RODATA, $8

// Atan2 returns the arc tangent of y/x, using
// the signs of the two to determine the quadrant
// of the return value.
//
// Special cases are (in order):
//      Atan2(y, NaN) = NaN
//      Atan2(NaN, x) = NaN
//      Atan2(+0, x>=0) = +0
//      Atan2(-0, x>=0) = -0
//      Atan2(+0, x<=-0) = +Pi
//      Atan2(-0, x<=-0) = -Pi
//      Atan2(y>0, 0) = +Pi/2
//      Atan2(y<0, 0) = -Pi/2
//      Atan2(+Inf, +Inf) = +Pi/4
//      Atan2(-Inf, +Inf) = -Pi/4
//      Atan2(+Inf, -Inf) = 3Pi/4
//      Atan2(-Inf, -Inf) = -3Pi/4
//      Atan2(y, +Inf) = 0
//      Atan2(y>0, -Inf) = +Pi
//      Atan2(y<0, -Inf) = -Pi
//      Atan2(+Inf, x) = +Pi/2
//      Atan2(-Inf, x) = -Pi/2
// The algorithm used is minimax polynomial approximation
// with coefficients determined with a Remez exchange algorithm.

TEXT	·atan2Asm(SB), NOSPLIT, $0-24
	// special case
	MOVD	x+0(FP), R1
	MOVD	y+8(FP), R2

	// special case Atan2(NaN, y) = NaN
	MOVD	$~(1<<63), R5
	AND	R1, R5		// x = |x|
	MOVD	$PosInf, R3
	CMPUBLT	R3, R5, returnX

	// special case Atan2(x, NaN) = NaN
	MOVD	$~(1<<63), R5
	AND	R2, R5
	CMPUBLT R3, R5, returnY

	MOVD	$NegZero, R3
	CMPUBEQ	R3, R1, xIsNegZero

	MOVD	$0, R3
	CMPUBEQ	R3, R1, xIsPosZero

	MOVD	$PosInf, R4
	CMPUBEQ	R4, R2, yIsPosInf

	MOVD	$NegInf, R4
	CMPUBEQ	R4, R2, yIsNegInf
	BR	Normal
xIsNegZero:
	// special case Atan(-0, y>=0) = -0
	MOVD	$0, R4
	CMPBLE	R4, R2, returnX

	//special case Atan2(-0, y<=-0) = -Pi
	MOVD	$NegZero, R4
	CMPBGE	R4, R2, returnNegPi
	BR	Normal
xIsPosZero:
	//special case Atan2(0, 0) = 0
	MOVD	$0, R4
	CMPUBEQ	R4, R2, returnX

	//special case Atan2(0, y<=-0) = Pi
	MOVD	$NegZero, R4
	CMPBGE	R4, R2, returnPi
	BR Normal
yIsNegInf:
	//special case Atan2(+Inf, -Inf) = 3Pi/4
	MOVD	$PosInf, R3
	CMPUBEQ	R3, R1, posInfNegInf

	//special case Atan2(-Inf, -Inf) = -3Pi/4
	MOVD	$NegInf, R3
	CMPUBEQ	R3, R1, negInfNegInf
	BR Normal
yIsPosInf:
	//special case Atan2(+Inf, +Inf) = Pi/4
	MOVD	$PosInf, R3
	CMPUBEQ	R3, R1, posInfPosInf

	//special case Atan2(-Inf, +Inf) = -Pi/4
	MOVD	$NegInf, R3
	CMPUBEQ	R3, R1, negInfPosInf

	//special case Atan2(x, +Inf) = Copysign(0, x)
	CMPBLT	R1, $0, returnNegZero
	BR returnPosZero

Normal:
	FMOVD	x+0(FP), F0
	FMOVD	y+8(FP), F2
	MOVD	$·atan2rodataL25<>+0(SB), R9
	LGDR	F0, R2
	LGDR	F2, R1
	RISBGNZ	$32, $63, $32, R2, R2
	RISBGNZ	$32, $63, $32, R1, R1
	WORD	$0xB9170032	//llgtr	%r3,%r2
	RISBGZ	$63, $63, $33, R2, R5
	WORD	$0xB9170041	//llgtr	%r4,%r1
	WFLCDB	V0, V20
	MOVW	R4, R6
	MOVW	R3, R7
	CMPUBLT	R6, R7, L17
	WFDDB	V2, V0, V3
	ADDW	$2, R5, R2
	MOVW	R4, R6
	MOVW	R3, R7
	CMPUBLE	R6, R7, L20
L3:
	WFMDB	V3, V3, V4
	VLEG	$0, 152(R9), V18
	VLEG	$0, 144(R9), V16
	FMOVD	136(R9), F1
	FMOVD	128(R9), F5
	FMOVD	120(R9), F6
	WFMADB	V4, V16, V5, V16
	WFMADB	V4, V6, V1, V6
	FMOVD	112(R9), F7
	WFMDB	V4, V4, V1
	WFMADB	V4, V7, V18, V7
	VLEG	$0, 104(R9), V18
	WFMADB	V1, V6, V16, V6
	CMPWU	R4, R3
	FMOVD	96(R9), F5
	VLEG	$0, 88(R9), V16
	WFMADB	V4, V5, V18, V5
	VLEG	$0, 80(R9), V18
	VLEG	$0, 72(R9), V22
	WFMADB	V4, V16, V18, V16
	VLEG	$0, 64(R9), V18
	WFMADB	V1, V7, V5, V7
	WFMADB	V4, V18, V22, V18
	WFMDB	V1, V1, V5
	WFMADB	V1, V16, V18, V16
	VLEG	$0, 56(R9), V18
	WFMADB	V5, V6, V7, V6
	VLEG	$0, 48(R9), V22
	FMOVD	40(R9), F7
	WFMADB	V4, V7, V18, V7
	VLEG	$0, 32(R9), V18
	WFMADB	V5, V6, V16, V6
	WFMADB	V4, V18, V22, V18
	VLEG	$0, 24(R9), V16
	WFMADB	V1, V7, V18, V7
	VLEG	$0, 16(R9), V18
	VLEG	$0, 8(R9), V22
	WFMADB	V4, V18, V16, V18
	VLEG	$0, 0(R9), V16
	WFMADB	V5, V6, V7, V6
	WFMADB	V4, V16, V22, V16
	FMUL	F3, F4
	WFMADB	V1, V18, V16, V1
	FMADD	F6, F5, F1
	WFMADB	V4, V1, V3, V4
	BLT	L18
	BGT	L7
	LTDBR	F2, F2
	BLTU	L21
L8:
	LTDBR	F0, F0
	BLTU	L22
L9:
	WFCHDBS	V2, V0, V0
	BNE	L18
L7:
	MOVW	R1, R6
	CMPBGE	R6, $0, L1
L18:
	RISBGZ	$58, $60, $3, R2, R2
	MOVD	$·atan2xpi2h<>+0(SB), R1
	MOVD	·atan2xpim<>+0(SB), R3
	LDGR	R3, F0
	WORD	$0xED021000	//madb	%f4,%f0,0(%r2,%r1)
	BYTE	$0x40
	BYTE	$0x1E
L1:
	FMOVD	F4, ret+16(FP)
	RET

L20:
	LTDBR	F2, F2
	BLTU	L23
	FMOVD	F2, F6
L4:
	LTDBR	F0, F0
	BLTU	L24
	FMOVD	F0, F4
L5:
	WFCHDBS	V6, V4, V4
	BEQ	L3
L17:
	WFDDB	V0, V2, V4
	BYTE	$0x18	//lr	%r2,%r5
	BYTE	$0x25
	LCDBR	F4, F3
	BR	L3
L23:
	LCDBR   F2, F6
	BR	L4
L22:
	VLR	V20, V0
	BR	L9
L21:
	LCDBR   F2, F2
	BR	L8
L24:
	VLR	V20, V4
	BR	L5
returnX:	//the result is same as the first argument
	MOVD	R1, ret+16(FP)
	RET
returnY:	//the result is same as the second argument
	MOVD	R2, ret+16(FP)
	RET
returnPi:
	MOVD	$Pi, R1
	MOVD	R1, ret+16(FP)
	RET
returnNegPi:
	MOVD	$NegPi, R1
	MOVD	R1, ret+16(FP)
	RET
posInfNegInf:
	MOVD	$Pi3Div4, R1
	MOVD	R1, ret+16(FP)
	RET
negInfNegInf:
	MOVD	$NegPi3Div4, R1
	MOVD	R1, ret+16(FP)
	RET
posInfPosInf:
	MOVD	$PiDiv4, R1
	MOVD	R1, ret+16(FP)
	RET
negInfPosInf:
	MOVD	$NegPiDiv4, R1
	MOVD	R1, ret+16(FP)
	RET
returnNegZero:
	MOVD	$NegZero, R1
	MOVD	R1, ret+16(FP)
	RET
returnPosZero:
	MOVD	$0, ret+16(FP)
	RET

```

// === FILE: references!/go/src/math/atan_s390x.s ===
```text
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// Minimax polynomial coefficients and other constants
DATA ·atanrodataL8<> + 0(SB)/8, $0.199999999999554423E+00
DATA ·atanrodataL8<> + 8(SB)/8, $0.111111110136634272E+00
DATA ·atanrodataL8<> + 16(SB)/8, $-.142857142828026806E+00
DATA ·atanrodataL8<> + 24(SB)/8, $-.333333333333330928E+00
DATA ·atanrodataL8<> + 32(SB)/8, $0.769228118888682505E-01
DATA ·atanrodataL8<> + 40(SB)/8, $0.588059263575587687E-01
DATA ·atanrodataL8<> + 48(SB)/8, $-.666641501287528609E-01
DATA ·atanrodataL8<> + 56(SB)/8, $-.909090711945939878E-01
DATA ·atanrodataL8<> + 64(SB)/8, $0.472329433805024762E-01
DATA ·atanrodataL8<> + 72(SB)/8, $0.366935664549587481E-01
DATA ·atanrodataL8<> + 80(SB)/8, $-.422172007412067035E-01
DATA ·atanrodataL8<> + 88(SB)/8, $-.299856214685512712E-01
DATA ·atanrodataL8<> + 96(SB)/8, $0.220852012160300086E-01
DATA ·atanrodataL8<> + 104(SB)/8, $0.726338160757602439E-02
DATA ·atanrodataL8<> + 112(SB)/8, $0.843488472994227321E-03
DATA ·atanrodataL8<> + 120(SB)/8, $0.134893651284712515E-04
DATA ·atanrodataL8<> + 128(SB)/8, $-.525380587584426406E-01
DATA ·atanrodataL8<> + 136(SB)/8, $-.139950258898989925E-01
DATA ·atanrodataL8<> + 144(SB)/8, $-.291935324869629616E-02
DATA ·atanrodataL8<> + 152(SB)/8, $-.154797890856877418E-03
GLOBL ·atanrodataL8<> + 0(SB), RODATA, $160

DATA ·atanxpi2h<> + 0(SB)/8, $0x3ff330e4e4fa7b1b
DATA ·atanxpi2h<> + 8(SB)/8, $0xbff330e4e4fa7b1b
DATA ·atanxpi2h<> + 16(SB)/8, $0x400330e4e4fa7b1b
DATA ·atanxpi2h<> + 24(SB)/4, $0xc00330e4e4fa7b1b
GLOBL ·atanxpi2h<> + 0(SB), RODATA, $32
DATA ·atanxpim<> + 0(SB)/8, $0x3ff4f42b00000000
GLOBL ·atanxpim<> + 0(SB), RODATA, $8
DATA ·atanxmone<> + 0(SB)/8, $-1.0
GLOBL ·atanxmone<> + 0(SB), RODATA, $8

// Atan returns the arctangent, in radians, of the argument.
//
// Special cases are:
//      Atan(±0) = ±0
//      Atan(±Inf) = ±Pi/2Pi
// The algorithm used is minimax polynomial approximation
// with coefficients determined with a Remez exchange algorithm.

TEXT	·atanAsm(SB), NOSPLIT, $0-16
	FMOVD	x+0(FP), F0
	//special case Atan(±0) = ±0
	FMOVD   $(0.0), F1
	FCMPU   F0, F1
	BEQ     atanIsZero

	MOVD	$·atanrodataL8<>+0(SB), R5
	MOVH	$0x3FE0, R3
	LGDR	F0, R1
	RISBGNZ	$32, $63, $32, R1, R1
	RLL	$16, R1, R2
	ANDW	$0x7FF0, R2
	MOVW	R2, R6
	MOVW	R3, R7
	CMPUBLE	R6, R7, L6
	MOVD	$·atanxmone<>+0(SB), R3
	FMOVD	0(R3), F2
	WFDDB	V0, V2, V0
	RISBGZ	$63, $63, $33, R1, R1
	MOVD	$·atanxpi2h<>+0(SB), R3
	MOVWZ	R1, R1
	SLD	$3, R1, R1
	WORD	$0x68813000	//ld	%f8,0(%r1,%r3)
L6:
	WFMDB	V0, V0, V2
	FMOVD	152(R5), F6
	FMOVD	144(R5), F1
	FMOVD	136(R5), F7
	VLEG	$0, 128(R5), V16
	FMOVD	120(R5), F4
	FMOVD	112(R5), F5
	WFMADB	V2, V4, V6, V4
	WFMADB	V2, V5, V1, V5
	WFMDB	V2, V2, V6
	FMOVD	104(R5), F3
	FMOVD	96(R5), F1
	WFMADB	V2, V3, V7, V3
	MOVH	$0x3FE0, R1
	FMOVD	88(R5), F7
	WFMADB	V2, V1, V7, V1
	FMOVD	80(R5), F7
	WFMADB	V6, V3, V1, V3
	WFMADB	V6, V4, V5, V4
	WFMDB	V6, V6, V1
	FMOVD	72(R5), F5
	WFMADB	V2, V5, V7, V5
	FMOVD	64(R5), F7
	WFMADB	V2, V7, V16, V7
	VLEG	$0, 56(R5), V16
	WFMADB	V6, V5, V7, V5
	WFMADB	V1, V4, V3, V4
	FMOVD	48(R5), F7
	FMOVD	40(R5), F3
	WFMADB	V2, V3, V7, V3
	FMOVD	32(R5), F7
	WFMADB	V2, V7, V16, V7
	VLEG	$0, 24(R5), V16
	WFMADB	V1, V4, V5, V4
	FMOVD	16(R5), F5
	WFMADB	V6, V3, V7, V3
	FMOVD	8(R5), F7
	WFMADB	V2, V7, V5, V7
	FMOVD	0(R5), F5
	WFMADB	V2, V5, V16, V5
	WFMADB	V1, V4, V3, V4
	WFMADB	V6, V7, V5, V6
	FMUL	F0, F2
	FMADD	F4, F1, F6
	FMADD	F6, F2, F0
	MOVW	R2, R6
	MOVW	R1, R7
	CMPUBLE	R6, R7, L1
	MOVD	$·atanxpim<>+0(SB), R1
	WORD	$0xED801000	//madb	%f0,%f8,0(%r1)
	BYTE	$0x00
	BYTE	$0x1E
L1:
atanIsZero:
	FMOVD	F0, ret+8(FP)
	RET

```

// === FILE: references!/go/src/math/atanh.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// The original C code, the long comment, and the constants
// below are from FreeBSD's /usr/src/lib/msun/src/e_atanh.c
// and came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
//
// __ieee754_atanh(x)
// Method :
//	1. Reduce x to positive by atanh(-x) = -atanh(x)
//	2. For x>=0.5
//	            1              2x                          x
//	atanh(x) = --- * log(1 + -------) = 0.5 * log1p(2 * --------)
//	            2             1 - x                      1 - x
//
//	For x<0.5
//	atanh(x) = 0.5*log1p(2x+2x*x/(1-x))
//
// Special cases:
//	atanh(x) is NaN if |x| > 1 with signal;
//	atanh(NaN) is that NaN with no signal;
//	atanh(+-1) is +-INF with signal.
//

// Atanh returns the inverse hyperbolic tangent of x.
//
// Special cases are:
//
//	Atanh(1) = +Inf
//	Atanh(±0) = ±0
//	Atanh(-1) = -Inf
//	Atanh(x) = NaN if x < -1 or x > 1
//	Atanh(NaN) = NaN
func Atanh(x float64) float64 {
	if haveArchAtanh {
		return archAtanh(x)
	}
	return atanh(x)
}

func atanh(x float64) float64 {
	const NearZero = 1.0 / (1 << 28) // 2**-28
	// special cases
	switch {
	case x < -1 || x > 1 || IsNaN(x):
		return NaN()
	case x == 1:
		return Inf(1)
	case x == -1:
		return Inf(-1)
	}
	sign := false
	if x < 0 {
		x = -x
		sign = true
	}
	var temp float64
	switch {
	case x < NearZero:
		temp = x
	case x < 0.5:
		temp = x + x
		temp = 0.5 * Log1p(temp+temp*x/(1-x))
	default:
		temp = 0.5 * Log1p((x+x)/(1-x))
	}
	if sign {
		temp = -temp
	}
	return temp
}

```

// === FILE: references!/go/src/math/atanh_s390x.s ===
```text
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// Minimax polynomial coefficients and other constants
DATA ·atanhrodataL10<> + 0(SB)/8, $.41375273347623353626
DATA ·atanhrodataL10<> + 8(SB)/8, $.51487302528619766235E+04
DATA ·atanhrodataL10<> + 16(SB)/8, $-1.67526912689208984375
DATA ·atanhrodataL10<> + 24(SB)/8, $0.181818181818181826E+00
DATA ·atanhrodataL10<> + 32(SB)/8, $-.165289256198351540E-01
DATA ·atanhrodataL10<> + 40(SB)/8, $0.200350613573012186E-02
DATA ·atanhrodataL10<> + 48(SB)/8, $0.397389654305194527E-04
DATA ·atanhrodataL10<> + 56(SB)/8, $-.273205381970859341E-03
DATA ·atanhrodataL10<> + 64(SB)/8, $0.938370938292558173E-06
DATA ·atanhrodataL10<> + 72(SB)/8, $-.148682720127920854E-06
DATA ·atanhrodataL10<> + 80(SB)/8, $ 0.212881813645679599E-07
DATA ·atanhrodataL10<> + 88(SB)/8, $-.602107458843052029E-05
DATA ·atanhrodataL10<> + 96(SB)/8, $-5.5
DATA ·atanhrodataL10<> + 104(SB)/8, $-0.5
DATA ·atanhrodataL10<> + 112(SB)/8, $0.0
DATA ·atanhrodataL10<> + 120(SB)/8, $0x7ff8000000000000      //Nan
DATA ·atanhrodataL10<> + 128(SB)/8, $-1.0
DATA ·atanhrodataL10<> + 136(SB)/8, $1.0
DATA ·atanhrodataL10<> + 144(SB)/8, $1.0E-20
GLOBL ·atanhrodataL10<> + 0(SB), RODATA, $152

// Table of log correction terms
DATA ·atanhtab2076<> + 0(SB)/8, $0.585235384085551248E-01
DATA ·atanhtab2076<> + 8(SB)/8, $0.412206153771168640E-01
DATA ·atanhtab2076<> + 16(SB)/8, $0.273839003221648339E-01
DATA ·atanhtab2076<> + 24(SB)/8, $0.166383778368856480E-01
DATA ·atanhtab2076<> + 32(SB)/8, $0.866678223433169637E-02
DATA ·atanhtab2076<> + 40(SB)/8, $0.319831684989627514E-02
DATA ·atanhtab2076<> + 48(SB)/8, $0.000000000000000000E+00
DATA ·atanhtab2076<> + 56(SB)/8, $-.113006378583725549E-02
DATA ·atanhtab2076<> + 64(SB)/8, $-.367979419636602491E-03
DATA ·atanhtab2076<> + 72(SB)/8, $0.213172484510484979E-02
DATA ·atanhtab2076<> + 80(SB)/8, $0.623271047682013536E-02
DATA ·atanhtab2076<> + 88(SB)/8, $0.118140812789696885E-01
DATA ·atanhtab2076<> + 96(SB)/8, $0.187681358930914206E-01
DATA ·atanhtab2076<> + 104(SB)/8, $0.269985148668178992E-01
DATA ·atanhtab2076<> + 112(SB)/8, $0.364186619761331328E-01
DATA ·atanhtab2076<> + 120(SB)/8, $0.469505379381388441E-01
GLOBL ·atanhtab2076<> + 0(SB), RODATA, $128

// Table of +/- .5
DATA ·atanhtabh2075<> + 0(SB)/8, $0.5
DATA ·atanhtabh2075<> + 8(SB)/8, $-.5
GLOBL ·atanhtabh2075<> + 0(SB), RODATA, $16

// Atanh returns the inverse hyperbolic tangent of the argument.
//
// Special cases are:
//      Atanh(1) = +Inf
//      Atanh(±0) = ±0
//      Atanh(-1) = -Inf
//      Atanh(x) = NaN if x < -1 or x > 1
//      Atanh(NaN) = NaN
// The algorithm used is minimax polynomial approximation
// with coefficients determined with a Remez exchange algorithm.

TEXT    ·atanhAsm(SB), NOSPLIT, $0-16
	FMOVD   x+0(FP), F0
	MOVD    $·atanhrodataL10<>+0(SB), R5
	LGDR    F0, R1
	WORD    $0xC0393FEF //iilf  %r3,1072693247
	BYTE    $0xFF
	BYTE    $0xFF
	SRAD    $32, R1
	WORD    $0xB9170021 //llgtr %r2,%r1
	MOVW    R2, R6
	MOVW    R3, R7
	CMPBGT  R6, R7, L2
	WORD    $0xC0392FFF //iilf  %r3,805306367
	BYTE    $0xFF
	BYTE    $0xFF
	MOVW    R2, R6
	MOVW    R3, R7
	CMPBGT  R6, R7, L9
L3:
	FMOVD   144(R5), F2
	FMADD   F2, F0, F0
L1:
	FMOVD   F0, ret+8(FP)
	RET

L2:
	WORD    $0xED005088 //cdb   %f0,.L12-.L10(%r5)
	BYTE    $0x00
	BYTE    $0x19
	BEQ L5
	WORD    $0xED005080 //cdb   %f0,.L13-.L10(%r5)
	BYTE    $0x00
	BYTE    $0x19
	BEQ L5
	WFCEDBS V0, V0, V2
	BVS L1
	FMOVD   120(R5), F0
	BR  L1
L5:
	WORD    $0xED005070 //ddb   %f0,.L15-.L10(%r5)
	BYTE    $0x00
	BYTE    $0x1D
	FMOVD   F0, ret+8(FP)
	RET

L9:
	FMOVD   F0, F2
	MOVD    $·atanhtabh2075<>+0(SB), R2
	SRW $31, R1, R1
	FMOVD   104(R5), F4
	MOVW    R1, R1
	SLD $3, R1, R1
	WORD    $0x68012000 //ld    %f0,0(%r1,%r2)
	WFMADB  V2, V4, V0, V4
	VLEG    $0, 96(R5), V16
	FDIV    F4, F2
	WORD    $0xC0298006 //iilf  %r2,2147909631
	BYTE    $0x7F
	BYTE    $0xFF
	FMOVD   88(R5), F6
	FMOVD   80(R5), F1
	FMOVD   72(R5), F7
	FMOVD   64(R5), F5
	FMOVD   F2, F4
	WORD    $0xED405088 //adb   %f4,.L12-.L10(%r5)
	BYTE    $0x00
	BYTE    $0x1A
	LGDR    F4, R4
	SRAD    $32, R4
	FMOVD   F4, F3
	WORD    $0xED305088 //sdb   %f3,.L12-.L10(%r5)
	BYTE    $0x00
	BYTE    $0x1B
	SUBW    R4, R2
	WFSDB   V3, V2, V3
	RISBGZ  $32, $47, $0, R2, R1
	SLD $32, R1, R1
	LDGR    R1, F2
	WFMADB  V4, V2, V16, V4
	SRAW    $8, R2, R1
	WFMADB  V4, V5, V6, V5
	WFMDB   V4, V4, V6
	WFMADB  V4, V1, V7, V1
	WFMADB  V2, V3, V4, V2
	WFMADB  V1, V6, V5, V1
	FMOVD   56(R5), F3
	FMOVD   48(R5), F5
	WFMADB  V4, V5, V3, V4
	FMOVD   40(R5), F3
	FMADD   F1, F6, F4
	FMOVD   32(R5), F1
	FMADD   F3, F2, F1
	ANDW    $0xFFFFFF00, R1
	WFMADB  V6, V4, V1, V6
	FMOVD   24(R5), F3
	ORW $0x45000000, R1
	WFMADB  V2, V6, V3, V6
	VLVGF   $0, R1, V4
	LDEBR   F4, F4
	RISBGZ  $57, $60, $51, R2, R2
	MOVD    $·atanhtab2076<>+0(SB), R1
	FMOVD   16(R5), F3
	WORD    $0x68521000 //ld    %f5,0(%r2,%r1)
	FMOVD   8(R5), F1
	WFMADB  V2, V6, V5, V2
	WFMADB  V4, V3, V1, V4
	FMOVD   0(R5), F6
	FMADD   F6, F4, F2
	FMUL    F2, F0
	FMOVD   F0, ret+8(FP)
	RET

```

// === FILE: references!/go/src/math/big/accuracy_string.go ===
```go
// Code generated by "stringer -type=Accuracy"; DO NOT EDIT.

package big

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[Below - -1]
	_ = x[Exact-0]
	_ = x[Above-1]
}

const _Accuracy_name = "BelowExactAbove"

var _Accuracy_index = [...]uint8{0, 5, 10, 15}

func (i Accuracy) String() string {
	idx := int(i) - -1
	if i < -1 || idx >= len(_Accuracy_index)-1 {
		return "Accuracy(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _Accuracy_name[_Accuracy_index[idx]:_Accuracy_index[idx+1]]
}

```

// === FILE: references!/go/src/math/big/arith.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file provides Go implementations of elementary multi-precision
// arithmetic operations on word vectors. These have the suffix _g.
// These are needed for platforms without assembly implementations of these routines.
// This file also contains elementary operations that can be implemented
// sufficiently efficiently in Go.

package big

import (
	"math/bits"
	_ "unsafe" // for go:linkname
)

// A Word represents a single digit of a multi-precision unsigned integer.
type Word uint

const (
	_S = _W / 8 // word size in bytes

	_W = bits.UintSize // word size in bits
	_B = 1 << _W       // digit base
	_M = _B - 1        // digit mask
)

// In these routines, it is the caller's responsibility to arrange for
// x, y, and z to all have the same length. We check this and panic.
// The assembly versions of these routines do not include that check.
//
// The check+panic also has the effect of teaching the compiler that
// “i in range for z” implies “i in range for x and y”, eliminating all
// bounds checks in loops from 0 to len(z) and vice versa.

// ----------------------------------------------------------------------------
// Elementary operations on words
//
// These operations are used by the vector operations below.

// z1<<_W + z0 = x*y
func mulWW(x, y Word) (z1, z0 Word) {
	hi, lo := bits.Mul(uint(x), uint(y))
	return Word(hi), Word(lo)
}

// z1<<_W + z0 = x*y + c
func mulAddWWW_g(x, y, c Word) (z1, z0 Word) {
	hi, lo := bits.Mul(uint(x), uint(y))
	var cc uint
	lo, cc = bits.Add(lo, uint(c), 0)
	return Word(hi + cc), Word(lo)
}

// nlz returns the number of leading zeros in x.
// Wraps bits.LeadingZeros call for convenience.
func nlz(x Word) uint {
	return uint(bits.LeadingZeros(uint(x)))
}

// The resulting carry c is either 0 or 1.
func addVV_g(z, x, y []Word) (c Word) {
	if len(x) != len(z) || len(y) != len(z) {
		panic("addVV len")
	}

	for i := range z {
		zi, cc := bits.Add(uint(x[i]), uint(y[i]), uint(c))
		z[i] = Word(zi)
		c = Word(cc)
	}
	return
}

// The resulting carry c is either 0 or 1.
func subVV_g(z, x, y []Word) (c Word) {
	if len(x) != len(z) || len(y) != len(z) {
		panic("subVV len")
	}

	for i := range z {
		zi, cc := bits.Sub(uint(x[i]), uint(y[i]), uint(c))
		z[i] = Word(zi)
		c = Word(cc)
	}
	return
}

// addVW sets z = x + y, returning the final carry c.
// The behavior is undefined if len(x) != len(z).
// If len(z) == 0, c = y; otherwise, c is 0 or 1.
//
// addVW should be an internal detail,
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/remyoudompheng/bigfft
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
//go:linkname addVW
func addVW(z, x []Word, y Word) (c Word) {
	if len(x) != len(z) {
		panic("addVW len")
	}

	if len(z) == 0 {
		return y
	}
	zi, cc := bits.Add(uint(x[0]), uint(y), 0)
	z[0] = Word(zi)
	if cc == 0 {
		if &z[0] != &x[0] {
			copy(z[1:], x[1:])
		}
		return 0
	}
	for i := 1; i < len(z); i++ {
		xi := x[i]
		if xi != ^Word(0) {
			z[i] = xi + 1
			if &z[0] != &x[0] {
				copy(z[i+1:], x[i+1:])
			}
			return 0
		}
		z[i] = 0
	}
	return 1
}

// addVW_ref is the reference implementation for addVW, used only for testing.
func addVW_ref(z, x []Word, y Word) (c Word) {
	c = y
	for i := range z {
		zi, cc := bits.Add(uint(x[i]), uint(c), 0)
		z[i] = Word(zi)
		c = Word(cc)
	}
	return
}

// subVW sets z = x - y, returning the final carry c.
// The behavior is undefined if len(x) != len(z).
// If len(z) == 0, c = y; otherwise, c is 0 or 1.
//
// subVW should be an internal detail,
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/remyoudompheng/bigfft
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
//go:linkname subVW
func subVW(z, x []Word, y Word) (c Word) {
	if len(x) != len(z) {
		panic("subVW len")
	}

	if len(z) == 0 {
		return y
	}
	zi, cc := bits.Sub(uint(x[0]), uint(y), 0)
	z[0] = Word(zi)
	if cc == 0 {
		if &z[0] != &x[0] {
			copy(z[1:], x[1:])
		}
		return 0
	}
	for i := 1; i < len(z); i++ {
		xi := x[i]
		if xi != 0 {
			z[i] = xi - 1
			if &z[0] != &x[0] {
				copy(z[i+1:], x[i+1:])
			}
			return 0
		}
		z[i] = ^Word(0)
	}
	return 1
}

// subVW_ref is the reference implementation for subVW, used only for testing.
func subVW_ref(z, x []Word, y Word) (c Word) {
	c = y
	for i := range z {
		zi, cc := bits.Sub(uint(x[i]), uint(c), 0)
		z[i] = Word(zi)
		c = Word(cc)
	}
	return c
}

func lshVU_g(z, x []Word, s uint) (c Word) {
	if len(x) != len(z) {
		panic("lshVU len")
	}

	if s == 0 {
		copy(z, x)
		return
	}
	if len(z) == 0 {
		return
	}
	s &= _W - 1 // hint to the compiler that shifts by s don't need guard code
	ŝ := _W - s
	ŝ &= _W - 1 // ditto
	c = x[len(z)-1] >> ŝ
	for i := len(z) - 1; i > 0; i-- {
		z[i] = x[i]<<s | x[i-1]>>ŝ
	}
	z[0] = x[0] << s
	return
}

func rshVU_g(z, x []Word, s uint) (c Word) {
	if len(x) != len(z) {
		panic("rshVU len")
	}

	if s == 0 {
		copy(z, x)
		return
	}
	if len(z) == 0 {
		return
	}
	s &= _W - 1 // hint to the compiler that shifts by s don't need guard code
	ŝ := _W - s
	ŝ &= _W - 1 // ditto
	c = x[0] << ŝ
	for i := 1; i < len(z); i++ {
		z[i-1] = x[i-1]>>s | x[i]<<ŝ
	}
	z[len(z)-1] = x[len(z)-1] >> s
	return
}

func mulAddVWW_g(z, x []Word, y, r Word) (c Word) {
	if len(x) != len(z) {
		panic("mulAddVWW len")
	}
	c = r
	for i := range z {
		c, z[i] = mulAddWWW_g(x[i], y, c)
	}
	return
}

func addMulVVWW_g(z, x, y []Word, m, a Word) (c Word) {
	if len(x) != len(z) || len(y) != len(z) {
		panic("addMulVVWW len")
	}

	c = a
	for i := range z {
		z1, z0 := mulAddWWW_g(y[i], m, x[i])
		lo, cc := bits.Add(uint(z0), uint(c), 0)
		c, z[i] = Word(cc), Word(lo)
		c += z1
	}
	return
}

// q = ( x1 << _W + x0 - r)/y. m = floor(( _B^2 - 1 ) / d - _B). Requiring x1<y.
// An approximate reciprocal with a reference to "Improved Division by Invariant Integers
// (IEEE Transactions on Computers, 11 Jun. 2010)"
func divWW(x1, x0, y, m Word) (q, r Word) {
	s := nlz(y)
	if s != 0 {
		x1 = x1<<s | x0>>(_W-s)
		x0 <<= s
		y <<= s
	}
	d := uint(y)
	// We know that
	//   m = ⎣(B^2-1)/d⎦-B
	//   ⎣(B^2-1)/d⎦ = m+B
	//   (B^2-1)/d = m+B+delta1    0 <= delta1 <= (d-1)/d
	//   B^2/d = m+B+delta2        0 <= delta2 <= 1
	// The quotient we're trying to compute is
	//   quotient = ⎣(x1*B+x0)/d⎦
	//            = ⎣(x1*B*(B^2/d)+x0*(B^2/d))/B^2⎦
	//            = ⎣(x1*B*(m+B+delta2)+x0*(m+B+delta2))/B^2⎦
	//            = ⎣(x1*m+x1*B+x0)/B + x0*m/B^2 + delta2*(x1*B+x0)/B^2⎦
	// The latter two terms of this three-term sum are between 0 and 1.
	// So we can compute just the first term, and we will be low by at most 2.
	t1, t0 := bits.Mul(uint(m), uint(x1))
	_, c := bits.Add(t0, uint(x0), 0)
	t1, _ = bits.Add(t1, uint(x1), c)
	// The quotient is either t1, t1+1, or t1+2.
	// We'll try t1 and adjust if needed.
	qq := t1
	// compute remainder r=x-d*q.
	dq1, dq0 := bits.Mul(d, qq)
	r0, b := bits.Sub(uint(x0), dq0, 0)
	r1, _ := bits.Sub(uint(x1), dq1, b)
	// The remainder we just computed is bounded above by B+d:
	// r = x1*B + x0 - d*q.
	//   = x1*B + x0 - d*⎣(x1*m+x1*B+x0)/B⎦
	//   = x1*B + x0 - d*((x1*m+x1*B+x0)/B-alpha)                                   0 <= alpha < 1
	//   = x1*B + x0 - x1*d/B*m                         - x1*d - x0*d/B + d*alpha
	//   = x1*B + x0 - x1*d/B*⎣(B^2-1)/d-B⎦             - x1*d - x0*d/B + d*alpha
	//   = x1*B + x0 - x1*d/B*⎣(B^2-1)/d-B⎦             - x1*d - x0*d/B + d*alpha
	//   = x1*B + x0 - x1*d/B*((B^2-1)/d-B-beta)        - x1*d - x0*d/B + d*alpha   0 <= beta < 1
	//   = x1*B + x0 - x1*B + x1/B + x1*d + x1*d/B*beta - x1*d - x0*d/B + d*alpha
	//   =        x0        + x1/B        + x1*d/B*beta        - x0*d/B + d*alpha
	//   = x0*(1-d/B) + x1*(1+d*beta)/B + d*alpha
	//   <  B*(1-d/B) +  d*B/B          + d          because x0<B (and 1-d/B>0), x1<d, 1+d*beta<=B, alpha<1
	//   =  B - d     +  d              + d
	//   = B+d
	// So r1 can only be 0 or 1. If r1 is 1, then we know q was too small.
	// Add 1 to q and subtract d from r. That guarantees that r is <B, so
	// we no longer need to keep track of r1.
	if r1 != 0 {
		qq++
		r0 -= d
	}
	// If the remainder is still too large, increment q one more time.
	if r0 >= d {
		qq++
		r0 -= d
	}
	return Word(qq), Word(r0 >> s)
}

// reciprocalWord return the reciprocal of the divisor. rec = floor(( _B^2 - 1 ) / u - _B). u = d1 << nlz(d1).
func reciprocalWord(d1 Word) Word {
	u := uint(d1 << nlz(d1))
	x1 := ^u
	x0 := uint(_M)
	rec, _ := bits.Div(x1, x0, u) // (_B^2-1)/U-_B = (_B*(_M-C)+_M)/U
	return Word(rec)
}

```

// === FILE: references!/go/src/math/big/arith_386.s ===
```text
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by 'go generate' (with ./internal/asmgen). DO NOT EDIT.

//go:build !math_big_pure_go

#include "textflag.h"

// func addVV(z, x, y []Word) (c Word)
TEXT ·addVV(SB), NOSPLIT, $0
	MOVL z_len+4(FP), BX
	MOVL x_base+12(FP), SI
	MOVL y_base+24(FP), DI
	MOVL z_base+0(FP), BP
	// compute unrolled loop lengths
	MOVL BX, CX
	ANDL $3, CX
	SHRL $2, BX
	MOVL $0, DX	// clear saved carry
loop1:
	TESTL CX, CX; JZ loop1done
loop1cont:
	// unroll 1X in batches of 1
	ADDL DX, DX	// restore carry
	MOVL 0(SI), DX
	ADCL 0(DI), DX
	MOVL DX, 0(BP)
	SBBL DX, DX	// save carry
	LEAL 4(SI), SI	// ADD $4, SI
	LEAL 4(DI), DI	// ADD $4, DI
	LEAL 4(BP), BP	// ADD $4, BP
	SUBL $1, CX; JNZ loop1cont
loop1done:
loop4:
	TESTL BX, BX; JZ loop4done
loop4cont:
	// unroll 4X in batches of 1
	ADDL DX, DX	// restore carry
	MOVL 0(SI), CX
	ADCL 0(DI), CX
	MOVL CX, 0(BP)
	MOVL 4(SI), CX
	ADCL 4(DI), CX
	MOVL CX, 4(BP)
	MOVL 8(SI), CX
	ADCL 8(DI), CX
	MOVL CX, 8(BP)
	MOVL 12(SI), CX
	ADCL 12(DI), CX
	MOVL CX, 12(BP)
	SBBL DX, DX	// save carry
	LEAL 16(SI), SI	// ADD $16, SI
	LEAL 16(DI), DI	// ADD $16, DI
	LEAL 16(BP), BP	// ADD $16, BP
	SUBL $1, BX; JNZ loop4cont
loop4done:
	NEGL DX	// convert add carry
	MOVL DX, c+36(FP)
	RET

// func subVV(z, x, y []Word) (c Word)
TEXT ·subVV(SB), NOSPLIT, $0
	MOVL z_len+4(FP), BX
	MOVL x_base+12(FP), SI
	MOVL y_base+24(FP), DI
	MOVL z_base+0(FP), BP
	// compute unrolled loop lengths
	MOVL BX, CX
	ANDL $3, CX
	SHRL $2, BX
	MOVL $0, DX	// clear saved carry
loop1:
	TESTL CX, CX; JZ loop1done
loop1cont:
	// unroll 1X in batches of 1
	ADDL DX, DX	// restore carry
	MOVL 0(SI), DX
	SBBL 0(DI), DX
	MOVL DX, 0(BP)
	SBBL DX, DX	// save carry
	LEAL 4(SI), SI	// ADD $4, SI
	LEAL 4(DI), DI	// ADD $4, DI
	LEAL 4(BP), BP	// ADD $4, BP
	SUBL $1, CX; JNZ loop1cont
loop1done:
loop4:
	TESTL BX, BX; JZ loop4done
loop4cont:
	// unroll 4X in batches of 1
	ADDL DX, DX	// restore carry
	MOVL 0(SI), CX
	SBBL 0(DI), CX
	MOVL CX, 0(BP)
	MOVL 4(SI), CX
	SBBL 4(DI), CX
	MOVL CX, 4(BP)
	MOVL 8(SI), CX
	SBBL 8(DI), CX
	MOVL CX, 8(BP)
	MOVL 12(SI), CX
	SBBL 12(DI), CX
	MOVL CX, 12(BP)
	SBBL DX, DX	// save carry
	LEAL 16(SI), SI	// ADD $16, SI
	LEAL 16(DI), DI	// ADD $16, DI
	LEAL 16(BP), BP	// ADD $16, BP
	SUBL $1, BX; JNZ loop4cont
loop4done:
	NEGL DX	// convert sub carry
	MOVL DX, c+36(FP)
	RET

// func lshVU(z, x []Word, s uint) (c Word)
TEXT ·lshVU(SB), NOSPLIT, $0
	MOVL z_len+4(FP), BX
	TESTL BX, BX; JZ ret0
	MOVL s+24(FP), CX
	MOVL x_base+12(FP), SI
	MOVL z_base+0(FP), DI
	// run loop backward, using counter as positive index
	// shift first word into carry
	MOVL -4(SI)(BX*4), BP
	MOVL $0, DX
	SHLL CX, BP, DX
	MOVL DX, c+28(FP)
	// shift remaining words
	SUBL $1, BX
loop1:
	TESTL BX, BX; JZ loop1done
loop1cont:
	// unroll 1X in batches of 1
	MOVL -4(SI)(BX*4), DX
	SHLL CX, DX, BP
	MOVL BP, 0(DI)(BX*4)
	MOVL DX, BP
	SUBL $1, BX; JNZ loop1cont
loop1done:
	// store final shifted bits
	SHLL CX, BP
	MOVL BP, 0(DI)(BX*4)
	RET
ret0:
	MOVL $0, c+28(FP)
	RET

// func rshVU(z, x []Word, s uint) (c Word)
TEXT ·rshVU(SB), NOSPLIT, $0
	MOVL z_len+4(FP), BX
	TESTL BX, BX; JZ ret0
	MOVL s+24(FP), CX
	MOVL x_base+12(FP), SI
	MOVL z_base+0(FP), DI
	// use counter as negative index
	LEAL (SI)(BX*4), SI
	LEAL (DI)(BX*4), DI
	NEGL BX
	// shift first word into carry
	MOVL 0(SI)(BX*4), BP
	MOVL $0, DX
	SHRL CX, BP, DX
	MOVL DX, c+28(FP)
	// shift remaining words
	ADDL $1, BX
loop1:
	TESTL BX, BX; JZ loop1done
loop1cont:
	// unroll 1X in batches of 1
	MOVL 0(SI)(BX*4), DX
	SHRL CX, DX, BP
	MOVL BP, -4(DI)(BX*4)
	MOVL DX, BP
	ADDL $1, BX; JNZ loop1cont
loop1done:
	// store final shifted bits
	SHRL CX, BP
	MOVL BP, -4(DI)(BX*4)
	RET
ret0:
	MOVL $0, c+28(FP)
	RET

// func mulAddVWW(z, x []Word, m, a Word) (c Word)
TEXT ·mulAddVWW(SB), NOSPLIT, $0
	MOVL m+24(FP), BX
	MOVL a+28(FP), SI
	MOVL z_len+4(FP), DI
	MOVL x_base+12(FP), BP
	MOVL z_base+0(FP), CX
	// use counter as negative index
	LEAL (BP)(DI*4), BP
	LEAL (CX)(DI*4), CX
	NEGL DI
loop1:
	TESTL DI, DI; JZ loop1done
loop1cont:
	// unroll 1X in batches of 1
	MOVL 0(BP)(DI*4), AX
	// multiply
	MULL BX
	ADDL SI, AX
	MOVL DX, SI
	ADCL $0, SI
	MOVL AX, 0(CX)(DI*4)
	ADDL $1, DI; JNZ loop1cont
loop1done:
	MOVL SI, c+32(FP)
	RET

// func addMulVVWW(z, x, y []Word, m, a Word) (c Word)
TEXT ·addMulVVWW(SB), NOSPLIT, $0
	MOVL a+40(FP), BX
	MOVL z_len+4(FP), SI
	MOVL x_base+12(FP), DI
	MOVL y_base+24(FP), BP
	MOVL z_base+0(FP), CX
	// use counter as negative index
	LEAL (DI)(SI*4), DI
	LEAL (BP)(SI*4), BP
	LEAL (CX)(SI*4), CX
	NEGL SI
loop1:
	TESTL SI, SI; JZ loop1done
loop1cont:
	// unroll 1X in batches of 1
	MOVL 0(BP)(SI*4), AX
	// multiply
	MULL m+36(FP)
	ADDL BX, AX
	MOVL DX, BX
	ADCL $0, BX
	// add
	ADDL 0(DI)(SI*4), AX
	ADCL $0, BX
	MOVL AX, 0(CX)(SI*4)
	ADDL $1, SI; JNZ loop1cont
loop1done:
	MOVL BX, c+44(FP)
	RET

```

// === FILE: references!/go/src/math/big/arith_amd64.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !math_big_pure_go

package big

import "internal/cpu"

var hasADX = cpu.X86.HasADX && cpu.X86.HasBMI2

```

// === FILE: references!/go/src/math/big/arith_amd64.s ===
```text
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by 'go generate' (with ./internal/asmgen). DO NOT EDIT.

//go:build !math_big_pure_go

#include "textflag.h"

// func addVV(z, x, y []Word) (c Word)
TEXT ·addVV(SB), NOSPLIT, $0
	MOVQ z_len+8(FP), BX
	MOVQ x_base+24(FP), SI
	MOVQ y_base+48(FP), DI
	MOVQ z_base+0(FP), R8
	// compute unrolled loop lengths
	MOVQ BX, R9
	ANDQ $3, R9
	SHRQ $2, BX
	MOVQ $0, R10	// clear saved carry
loop1:
	TESTQ R9, R9; JZ loop1done
loop1cont:
	// unroll 1X
	ADDQ R10, R10	// restore carry
	MOVQ 0(SI), R10
	ADCQ 0(DI), R10
	MOVQ R10, 0(R8)
	SBBQ R10, R10	// save carry
	LEAQ 8(SI), SI	// ADD $8, SI
	LEAQ 8(DI), DI	// ADD $8, DI
	LEAQ 8(R8), R8	// ADD $8, R8
	SUBQ $1, R9; JNZ loop1cont
loop1done:
loop4:
	TESTQ BX, BX; JZ loop4done
loop4cont:
	// unroll 4X
	ADDQ R10, R10	// restore carry
	MOVQ 0(SI), R9
	MOVQ 8(SI), R10
	MOVQ 16(SI), R11
	MOVQ 24(SI), R12
	ADCQ 0(DI), R9
	ADCQ 8(DI), R10
	ADCQ 16(DI), R11
	ADCQ 24(DI), R12
	MOVQ R9, 0(R8)
	MOVQ R10, 8(R8)
	MOVQ R11, 16(R8)
	MOVQ R12, 24(R8)
	SBBQ R10, R10	// save carry
	LEAQ 32(SI), SI	// ADD $32, SI
	LEAQ 32(DI), DI	// ADD $32, DI
	LEAQ 32(R8), R8	// ADD $32, R8
	SUBQ $1, BX; JNZ loop4cont
loop4done:
	NEGQ R10	// convert add carry
	MOVQ R10, c+72(FP)
	RET

// func subVV(z, x, y []Word) (c Word)
TEXT ·subVV(SB), NOSPLIT, $0
	MOVQ z_len+8(FP), BX
	MOVQ x_base+24(FP), SI
	MOVQ y_base+48(FP), DI
	MOVQ z_base+0(FP), R8
	// compute unrolled loop lengths
	MOVQ BX, R9
	ANDQ $3, R9
	SHRQ $2, BX
	MOVQ $0, R10	// clear saved carry
loop1:
	TESTQ R9, R9; JZ loop1done
loop1cont:
	// unroll 1X
	ADDQ R10, R10	// restore carry
	MOVQ 0(SI), R10
	SBBQ 0(DI), R10
	MOVQ R10, 0(R8)
	SBBQ R10, R10	// save carry
	LEAQ 8(SI), SI	// ADD $8, SI
	LEAQ 8(DI), DI	// ADD $8, DI
	LEAQ 8(R8), R8	// ADD $8, R8
	SUBQ $1, R9; JNZ loop1cont
loop1done:
loop4:
	TESTQ BX, BX; JZ loop4done
loop4cont:
	// unroll 4X
	ADDQ R10, R10	// restore carry
	MOVQ 0(SI), R9
	MOVQ 8(SI), R10
	MOVQ 16(SI), R11
	MOVQ 24(SI), R12
	SBBQ 0(DI), R9
	SBBQ 8(DI), R10
	SBBQ 16(DI), R11
	SBBQ 24(DI), R12
	MOVQ R9, 0(R8)
	MOVQ R10, 8(R8)
	MOVQ R11, 16(R8)
	MOVQ R12, 24(R8)
	SBBQ R10, R10	// save carry
	LEAQ 32(SI), SI	// ADD $32, SI
	LEAQ 32(DI), DI	// ADD $32, DI
	LEAQ 32(R8), R8	// ADD $32, R8
	SUBQ $1, BX; JNZ loop4cont
loop4done:
	NEGQ R10	// convert sub carry
	MOVQ R10, c+72(FP)
	RET

// func lshVU(z, x []Word, s uint) (c Word)
TEXT ·lshVU(SB), NOSPLIT, $0
	MOVQ z_len+8(FP), BX
	TESTQ BX, BX; JZ ret0
	MOVQ s+48(FP), CX
	MOVQ x_base+24(FP), SI
	MOVQ z_base+0(FP), DI
	// run loop backward
	LEAQ (SI)(BX*8), SI
	LEAQ (DI)(BX*8), DI
	// shift first word into carry
	MOVQ -8(SI), R8
	MOVQ $0, R9
	SHLQ CX, R8, R9
	MOVQ R9, c+56(FP)
	// shift remaining words
	SUBQ $1, BX
	// compute unrolled loop lengths
	MOVQ BX, R9
	ANDQ $3, R9
	SHRQ $2, BX
loop1:
	TESTQ R9, R9; JZ loop1done
loop1cont:
	// unroll 1X
	MOVQ -16(SI), R10
	SHLQ CX, R10, R8
	MOVQ R8, -8(DI)
	MOVQ R10, R8
	LEAQ -8(SI), SI	// ADD $-8, SI
	LEAQ -8(DI), DI	// ADD $-8, DI
	SUBQ $1, R9; JNZ loop1cont
loop1done:
loop4:
	TESTQ BX, BX; JZ loop4done
loop4cont:
	// unroll 4X
	MOVQ -16(SI), R9
	MOVQ -24(SI), R10
	MOVQ -32(SI), R11
	MOVQ -40(SI), R12
	SHLQ CX, R9, R8
	SHLQ CX, R10, R9
	SHLQ CX, R11, R10
	SHLQ CX, R12, R11
	MOVQ R8, -8(DI)
	MOVQ R9, -16(DI)
	MOVQ R10, -24(DI)
	MOVQ R11, -32(DI)
	MOVQ R12, R8
	LEAQ -32(SI), SI	// ADD $-32, SI
	LEAQ -32(DI), DI	// ADD $-32, DI
	SUBQ $1, BX; JNZ loop4cont
loop4done:
	// store final shifted bits
	SHLQ CX, R8
	MOVQ R8, -8(DI)
	RET
ret0:
	MOVQ $0, c+56(FP)
	RET

// func rshVU(z, x []Word, s uint) (c Word)
TEXT ·rshVU(SB), NOSPLIT, $0
	MOVQ z_len+8(FP), BX
	TESTQ BX, BX; JZ ret0
	MOVQ s+48(FP), CX
	MOVQ x_base+24(FP), SI
	MOVQ z_base+0(FP), DI
	// shift first word into carry
	MOVQ 0(SI), R8
	MOVQ $0, R9
	SHRQ CX, R8, R9
	MOVQ R9, c+56(FP)
	// shift remaining words
	SUBQ $1, BX
	// compute unrolled loop lengths
	MOVQ BX, R9
	ANDQ $3, R9
	SHRQ $2, BX
loop1:
	TESTQ R9, R9; JZ loop1done
loop1cont:
	// unroll 1X
	MOVQ 8(SI), R10
	SHRQ CX, R10, R8
	MOVQ R8, 0(DI)
	MOVQ R10, R8
	LEAQ 8(SI), SI	// ADD $8, SI
	LEAQ 8(DI), DI	// ADD $8, DI
	SUBQ $1, R9; JNZ loop1cont
loop1done:
loop4:
	TESTQ BX, BX; JZ loop4done
loop4cont:
	// unroll 4X
	MOVQ 8(SI), R9
	MOVQ 16(SI), R10
	MOVQ 24(SI), R11
	MOVQ 32(SI), R12
	SHRQ CX, R9, R8
	SHRQ CX, R10, R9
	SHRQ CX, R11, R10
	SHRQ CX, R12, R11
	MOVQ R8, 0(DI)
	MOVQ R9, 8(DI)
	MOVQ R10, 16(DI)
	MOVQ R11, 24(DI)
	MOVQ R12, R8
	LEAQ 32(SI), SI	// ADD $32, SI
	LEAQ 32(DI), DI	// ADD $32, DI
	SUBQ $1, BX; JNZ loop4cont
loop4done:
	// store final shifted bits
	SHRQ CX, R8
	MOVQ R8, 0(DI)
	RET
ret0:
	MOVQ $0, c+56(FP)
	RET

// func mulAddVWW(z, x []Word, m, a Word) (c Word)
TEXT ·mulAddVWW(SB), NOSPLIT, $0
	MOVQ m+48(FP), BX
	MOVQ a+56(FP), SI
	MOVQ z_len+8(FP), DI
	MOVQ x_base+24(FP), R8
	MOVQ z_base+0(FP), R9
	// compute unrolled loop lengths
	MOVQ DI, R10
	ANDQ $3, R10
	SHRQ $2, DI
loop1:
	TESTQ R10, R10; JZ loop1done
loop1cont:
	// unroll 1X in batches of 1
	MOVQ 0(R8), AX
	// multiply
	MULQ BX
	ADDQ SI, AX
	MOVQ DX, SI
	ADCQ $0, SI
	MOVQ AX, 0(R9)
	LEAQ 8(R8), R8	// ADD $8, R8
	LEAQ 8(R9), R9	// ADD $8, R9
	SUBQ $1, R10; JNZ loop1cont
loop1done:
loop4:
	TESTQ DI, DI; JZ loop4done
loop4cont:
	// unroll 4X in batches of 1
	MOVQ 0(R8), AX
	// multiply
	MULQ BX
	ADDQ SI, AX
	MOVQ DX, SI
	ADCQ $0, SI
	MOVQ AX, 0(R9)
	MOVQ 8(R8), AX
	// multiply
	MULQ BX
	ADDQ SI, AX
	MOVQ DX, SI
	ADCQ $0, SI
	MOVQ AX, 8(R9)
	MOVQ 16(R8), AX
	// multiply
	MULQ BX
	ADDQ SI, AX
	MOVQ DX, SI
	ADCQ $0, SI
	MOVQ AX, 16(R9)
	MOVQ 24(R8), AX
	// multiply
	MULQ BX
	ADDQ SI, AX
	MOVQ DX, SI
	ADCQ $0, SI
	MOVQ AX, 24(R9)
	LEAQ 32(R8), R8	// ADD $32, R8
	LEAQ 32(R9), R9	// ADD $32, R9
	SUBQ $1, DI; JNZ loop4cont
loop4done:
	MOVQ SI, c+64(FP)
	RET

// func addMulVVWW(z, x, y []Word, m, a Word) (c Word)
TEXT ·addMulVVWW(SB), NOSPLIT, $0
	CMPB ·hasADX(SB), $0; JNZ altcarry
	MOVQ m+72(FP), BX
	MOVQ a+80(FP), SI
	MOVQ z_len+8(FP), DI
	MOVQ x_base+24(FP), R8
	MOVQ y_base+48(FP), R9
	MOVQ z_base+0(FP), R10
	// compute unrolled loop lengths
	MOVQ DI, R11
	ANDQ $3, R11
	SHRQ $2, DI
loop1:
	TESTQ R11, R11; JZ loop1done
loop1cont:
	// unroll 1X in batches of 1
	MOVQ 0(R9), AX
	// multiply
	MULQ BX
	ADDQ SI, AX
	MOVQ DX, SI
	ADCQ $0, SI
	// add
	ADDQ 0(R8), AX
	ADCQ $0, SI
	MOVQ AX, 0(R10)
	LEAQ 8(R8), R8	// ADD $8, R8
	LEAQ 8(R9), R9	// ADD $8, R9
	LEAQ 8(R10), R10	// ADD $8, R10
	SUBQ $1, R11; JNZ loop1cont
loop1done:
loop4:
	TESTQ DI, DI; JZ loop4done
loop4cont:
	// unroll 4X in batches of 1
	MOVQ 0(R9), AX
	// multiply
	MULQ BX
	ADDQ SI, AX
	MOVQ DX, SI
	ADCQ $0, SI
	// add
	ADDQ 0(R8), AX
	ADCQ $0, SI
	MOVQ AX, 0(R10)
	MOVQ 8(R9), AX
	// multiply
	MULQ BX
	ADDQ SI, AX
	MOVQ DX, SI
	ADCQ $0, SI
	// add
	ADDQ 8(R8), AX
	ADCQ $0, SI
	MOVQ AX, 8(R10)
	MOVQ 16(R9), AX
	// multiply
	MULQ BX
	ADDQ SI, AX
	MOVQ DX, SI
	ADCQ $0, SI
	// add
	ADDQ 16(R8), AX
	ADCQ $0, SI
	MOVQ AX, 16(R10)
	MOVQ 24(R9), AX
	// multiply
	MULQ BX
	ADDQ SI, AX
	MOVQ DX, SI
	ADCQ $0, SI
	// add
	ADDQ 24(R8), AX
	ADCQ $0, SI
	MOVQ AX, 24(R10)
	LEAQ 32(R8), R8	// ADD $32, R8
	LEAQ 32(R9), R9	// ADD $32, R9
	LEAQ 32(R10), R10	// ADD $32, R10
	SUBQ $1, DI; JNZ loop4cont
loop4done:
	MOVQ SI, c+88(FP)
	RET
altcarry:
	MOVQ m+72(FP), DX
	MOVQ a+80(FP), BX
	MOVQ z_len+8(FP), SI
	MOVQ $0, DI
	MOVQ x_base+24(FP), R8
	MOVQ y_base+48(FP), R9
	MOVQ z_base+0(FP), R10
	// compute unrolled loop lengths
	MOVQ SI, R11
	ANDQ $7, R11
	SHRQ $3, SI
alt1:
	TESTQ R11, R11; JZ alt1done
alt1cont:
	// unroll 1X
	// multiply and add
	TESTQ AX, AX	// clear carry
	TESTQ AX, AX	// clear carry
	MULXQ 0(R9), R13, R12
	ADCXQ BX, R13
	ADOXQ 0(R8), R13
	MOVQ R13, 0(R10)
	MOVQ R12, BX
	ADCXQ DI, BX
	ADOXQ DI, BX
	LEAQ 8(R8), R8	// ADD $8, R8
	LEAQ 8(R9), R9	// ADD $8, R9
	LEAQ 8(R10), R10	// ADD $8, R10
	SUBQ $1, R11; JNZ alt1cont
alt1done:
alt8:
	TESTQ SI, SI; JZ alt8done
alt8cont:
	// unroll 8X in batches of 2
	// multiply and add
	TESTQ AX, AX	// clear carry
	TESTQ AX, AX	// clear carry
	MULXQ 0(R9), R13, R11
	ADCXQ BX, R13
	ADOXQ 0(R8), R13
	MULXQ 8(R9), R14, BX
	ADCXQ R11, R14
	ADOXQ 8(R8), R14
	MOVQ R13, 0(R10)
	MOVQ R14, 8(R10)
	MULXQ 16(R9), R13, R11
	ADCXQ BX, R13
	ADOXQ 16(R8), R13
	MULXQ 24(R9), R14, BX
	ADCXQ R11, R14
	ADOXQ 24(R8), R14
	MOVQ R13, 16(R10)
	MOVQ R14, 24(R10)
	MULXQ 32(R9), R13, R11
	ADCXQ BX, R13
	ADOXQ 32(R8), R13
	MULXQ 40(R9), R14, BX
	ADCXQ R11, R14
	ADOXQ 40(R8), R14
	MOVQ R13, 32(R10)
	MOVQ R14, 40(R10)
	MULXQ 48(R9), R13, R11
	ADCXQ BX, R13
	ADOXQ 48(R8), R13
	MULXQ 56(R9), R14, BX
	ADCXQ R11, R14
	ADOXQ 56(R8), R14
	MOVQ R13, 48(R10)
	MOVQ R14, 56(R10)
	ADCXQ DI, BX
	ADOXQ DI, BX
	LEAQ 64(R8), R8	// ADD $64, R8
	LEAQ 64(R9), R9	// ADD $64, R9
	LEAQ 64(R10), R10	// ADD $64, R10
	SUBQ $1, SI; JNZ alt8cont
alt8done:
	MOVQ BX, c+88(FP)
	RET

```

// === FILE: references!/go/src/math/big/arith_arm.s ===
```text
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by 'go generate' (with ./internal/asmgen). DO NOT EDIT.

//go:build !math_big_pure_go

#include "textflag.h"

// func addVV(z, x, y []Word) (c Word)
TEXT ·addVV(SB), NOSPLIT, $0
	MOVW z_len+4(FP), R0
	MOVW x_base+12(FP), R1
	MOVW y_base+24(FP), R2
	MOVW z_base+0(FP), R3
	// compute unrolled loop lengths
	AND $3, R0, R4
	MOVW R0>>2, R0
	ADD.S $0, R0	// clear carry
loop1:
	TEQ $0, R4; BEQ loop1done
loop1cont:
	// unroll 1X
	MOVW.P 4(R1), R5
	MOVW.P 4(R2), R6
	ADC.S R6, R5
	MOVW.P R5, 4(R3)
	SUB $1, R4
	TEQ $0, R4; BNE loop1cont
loop1done:
loop4:
	TEQ $0, R0; BEQ loop4done
loop4cont:
	// unroll 4X
	MOVW.P 4(R1), R4
	MOVW.P 4(R1), R5
	MOVW.P 4(R1), R6
	MOVW.P 4(R1), R7
	MOVW.P 4(R2), R8
	MOVW.P 4(R2), R9
	MOVW.P 4(R2), R11
	MOVW.P 4(R2), R12
	ADC.S R8, R4
	ADC.S R9, R5
	ADC.S R11, R6
	ADC.S R12, R7
	MOVW.P R4, 4(R3)
	MOVW.P R5, 4(R3)
	MOVW.P R6, 4(R3)
	MOVW.P R7, 4(R3)
	SUB $1, R0
	TEQ $0, R0; BNE loop4cont
loop4done:
	SBC R1, R1	// save carry
	ADD $1, R1	// convert add carry
	MOVW R1, c+36(FP)
	RET

// func subVV(z, x, y []Word) (c Word)
TEXT ·subVV(SB), NOSPLIT, $0
	MOVW z_len+4(FP), R0
	MOVW x_base+12(FP), R1
	MOVW y_base+24(FP), R2
	MOVW z_base+0(FP), R3
	// compute unrolled loop lengths
	AND $3, R0, R4
	MOVW R0>>2, R0
	SUB.S $0, R0	// clear carry
loop1:
	TEQ $0, R4; BEQ loop1done
loop1cont:
	// unroll 1X
	MOVW.P 4(R1), R5
	MOVW.P 4(R2), R6
	SBC.S R6, R5
	MOVW.P R5, 4(R3)
	SUB $1, R4
	TEQ $0, R4; BNE loop1cont
loop1done:
loop4:
	TEQ $0, R0; BEQ loop4done
loop4cont:
	// unroll 4X
	MOVW.P 4(R1), R4
	MOVW.P 4(R1), R5
	MOVW.P 4(R1), R6
	MOVW.P 4(R1), R7
	MOVW.P 4(R2), R8
	MOVW.P 4(R2), R9
	MOVW.P 4(R2), R11
	MOVW.P 4(R2), R12
	SBC.S R8, R4
	SBC.S R9, R5
	SBC.S R11, R6
	SBC.S R12, R7
	MOVW.P R4, 4(R3)
	MOVW.P R5, 4(R3)
	MOVW.P R6, 4(R3)
	MOVW.P R7, 4(R3)
	SUB $1, R0
	TEQ $0, R0; BNE loop4cont
loop4done:
	SBC R1, R1	// save carry
	RSB $0, R1, R1	// convert sub carry
	MOVW R1, c+36(FP)
	RET

// func lshVU(z, x []Word, s uint) (c Word)
TEXT ·lshVU(SB), NOSPLIT, $0
	MOVW z_len+4(FP), R0
	TEQ $0, R0; BEQ ret0
	MOVW s+24(FP), R1
	MOVW x_base+12(FP), R2
	MOVW z_base+0(FP), R3
	// run loop backward
	ADD R0<<2, R2, R2
	ADD R0<<2, R3, R3
	// shift first word into carry
	MOVW.W -4(R2), R4
	MOVW $32, R5
	SUB R1, R5
	MOVW R4>>R5, R6
	MOVW R4<<R1, R4
	MOVW R6, c+28(FP)
	// shift remaining words
	SUB $1, R0
	// compute unrolled loop lengths
	AND $3, R0, R6
	MOVW R0>>2, R0
loop1:
	TEQ $0, R6; BEQ loop1done
loop1cont:
	// unroll 1X
	MOVW.W -4(R2), R7
	ORR R7>>R5, R4
	MOVW.W R4, -4(R3)
	MOVW R7<<R1, R4
	SUB $1, R6
	TEQ $0, R6; BNE loop1cont
loop1done:
loop4:
	TEQ $0, R0; BEQ loop4done
loop4cont:
	// unroll 4X
	MOVW.W -4(R2), R6
	MOVW.W -4(R2), R7
	MOVW.W -4(R2), R8
	MOVW.W -4(R2), R9
	ORR R6>>R5, R4
	MOVW.W R4, -4(R3)
	MOVW R6<<R1, R4
	ORR R7>>R5, R4
	MOVW.W R4, -4(R3)
	MOVW R7<<R1, R4
	ORR R8>>R5, R4
	MOVW.W R4, -4(R3)
	MOVW R8<<R1, R4
	ORR R9>>R5, R4
	MOVW.W R4, -4(R3)
	MOVW R9<<R1, R4
	SUB $1, R0
	TEQ $0, R0; BNE loop4cont
loop4done:
	// store final shifted bits
	MOVW.W R4, -4(R3)
	RET
ret0:
	MOVW $0, R1
	MOVW R1, c+28(FP)
	RET

// func rshVU(z, x []Word, s uint) (c Word)
TEXT ·rshVU(SB), NOSPLIT, $0
	MOVW z_len+4(FP), R0
	TEQ $0, R0; BEQ ret0
	MOVW s+24(FP), R1
	MOVW x_base+12(FP), R2
	MOVW z_base+0(FP), R3
	// shift first word into carry
	MOVW.P 4(R2), R4
	MOVW $32, R5
	SUB R1, R5
	MOVW R4<<R5, R6
	MOVW R4>>R1, R4
	MOVW R6, c+28(FP)
	// shift remaining words
	SUB $1, R0
	// compute unrolled loop lengths
	AND $3, R0, R6
	MOVW R0>>2, R0
loop1:
	TEQ $0, R6; BEQ loop1done
loop1cont:
	// unroll 1X
	MOVW.P 4(R2), R7
	ORR R7<<R5, R4
	MOVW.P R4, 4(R3)
	MOVW R7>>R1, R4
	SUB $1, R6
	TEQ $0, R6; BNE loop1cont
loop1done:
loop4:
	TEQ $0, R0; BEQ loop4done
loop4cont:
	// unroll 4X
	MOVW.P 4(R2), R6
	MOVW.P 4(R2), R7
	MOVW.P 4(R2), R8
	MOVW.P 4(R2), R9
	ORR R6<<R5, R4
	MOVW.P R4, 4(R3)
	MOVW R6>>R1, R4
	ORR R7<<R5, R4
	MOVW.P R4, 4(R3)
	MOVW R7>>R1, R4
	ORR R8<<R5, R4
	MOVW.P R4, 4(R3)
	MOVW R8>>R1, R4
	ORR R9<<R5, R4
	MOVW.P R4, 4(R3)
	MOVW R9>>R1, R4
	SUB $1, R0
	TEQ $0, R0; BNE loop4cont
loop4done:
	// store final shifted bits
	MOVW.P R4, 4(R3)
	RET
ret0:
	MOVW $0, R1
	MOVW R1, c+28(FP)
	RET

// func mulAddVWW(z, x []Word, m, a Word) (c Word)
TEXT ·mulAddVWW(SB), NOSPLIT, $0
	MOVW m+24(FP), R0
	MOVW a+28(FP), R1
	MOVW z_len+4(FP), R2
	MOVW x_base+12(FP), R3
	MOVW z_base+0(FP), R4
	// compute unrolled loop lengths
	AND $3, R2, R5
	MOVW R2>>2, R2
loop1:
	TEQ $0, R5; BEQ loop1done
loop1cont:
	// unroll 1X
	MOVW.P 4(R3), R6
	// multiply
	MULLU R0, R6, (R7, R6)
	ADD.S R1, R6
	ADC $0, R7, R1
	MOVW.P R6, 4(R4)
	SUB $1, R5
	TEQ $0, R5; BNE loop1cont
loop1done:
loop4:
	TEQ $0, R2; BEQ loop4done
loop4cont:
	// unroll 4X in batches of 2
	MOVW.P 4(R3), R5
	MOVW.P 4(R3), R6
	// multiply
	MULLU R0, R5, (R7, R5)
	ADD.S R1, R5
	MULLU R0, R6, (R8, R6)
	ADC.S R7, R6
	ADC $0, R8, R1
	MOVW.P R5, 4(R4)
	MOVW.P R6, 4(R4)
	MOVW.P 4(R3), R5
	MOVW.P 4(R3), R6
	// multiply
	MULLU R0, R5, (R7, R5)
	ADD.S R1, R5
	MULLU R0, R6, (R8, R6)
	ADC.S R7, R6
	ADC $0, R8, R1
	MOVW.P R5, 4(R4)
	MOVW.P R6, 4(R4)
	SUB $1, R2
	TEQ $0, R2; BNE loop4cont
loop4done:
	MOVW R1, c+32(FP)
	RET

// func addMulVVWW(z, x, y []Word, m, a Word) (c Word)
TEXT ·addMulVVWW(SB), NOSPLIT, $0
	MOVW m+36(FP), R0
	MOVW a+40(FP), R1
	MOVW z_len+4(FP), R2
	MOVW x_base+12(FP), R3
	MOVW y_base+24(FP), R4
	MOVW z_base+0(FP), R5
	// compute unrolled loop lengths
	AND $3, R2, R6
	MOVW R2>>2, R2
loop1:
	TEQ $0, R6; BEQ loop1done
loop1cont:
	// unroll 1X
	MOVW.P 4(R3), R7
	MOVW.P 4(R4), R8
	// multiply
	MULLU R0, R8, (R9, R8)
	ADD.S R1, R8
	ADC $0, R9, R1
	// add
	ADD.S R7, R8
	ADC $0, R1
	MOVW.P R8, 4(R5)
	SUB $1, R6
	TEQ $0, R6; BNE loop1cont
loop1done:
loop4:
	TEQ $0, R2; BEQ loop4done
loop4cont:
	// unroll 4X in batches of 2
	MOVW.P 4(R3), R6
	MOVW.P 4(R3), R7
	MOVW.P 4(R4), R8
	MOVW.P 4(R4), R9
	// multiply
	MULLU R0, R8, (R11, R8)
	ADD.S R1, R8
	MULLU R0, R9, (R12, R9)
	ADC.S R11, R9
	ADC $0, R12, R1
	// add
	ADD.S R6, R8
	ADC.S R7, R9
	ADC $0, R1
	MOVW.P R8, 4(R5)
	MOVW.P R9, 4(R5)
	MOVW.P 4(R3), R6
	MOVW.P 4(R3), R7
	MOVW.P 4(R4), R8
	MOVW.P 4(R4), R9
	// multiply
	MULLU R0, R8, (R11, R8)
	ADD.S R1, R8
	MULLU R0, R9, (R12, R9)
	ADC.S R11, R9
	ADC $0, R12, R1
	// add
	ADD.S R6, R8
	ADC.S R7, R9
	ADC $0, R1
	MOVW.P R8, 4(R5)
	MOVW.P R9, 4(R5)
	SUB $1, R2
	TEQ $0, R2; BNE loop4cont
loop4done:
	MOVW R1, c+44(FP)
	RET

```

// === FILE: references!/go/src/math/big/arith_arm64.s ===
```text
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by 'go generate' (with ./internal/asmgen). DO NOT EDIT.

//go:build !math_big_pure_go

#include "textflag.h"

// func addVV(z, x, y []Word) (c Word)
TEXT ·addVV(SB), NOSPLIT, $0
	MOVD z_len+8(FP), R0
	MOVD x_base+24(FP), R1
	MOVD y_base+48(FP), R2
	MOVD z_base+0(FP), R3
	// compute unrolled loop lengths
	AND $3, R0, R4
	LSR $2, R0
	ADDS ZR, R0	// clear carry
loop1:
	CBZ R4, loop1done
loop1cont:
	// unroll 1X
	MOVD.P 8(R1), R5
	MOVD.P 8(R2), R6
	ADCS R6, R5
	MOVD.P R5, 8(R3)
	SUB $1, R4
	CBNZ R4, loop1cont
loop1done:
loop4:
	CBZ R0, loop4done
loop4cont:
	// unroll 4X
	LDP.P 32(R1), (R4, R5)
	LDP -16(R1), (R6, R7)
	LDP.P 32(R2), (R8, R9)
	LDP -16(R2), (R10, R11)
	ADCS R8, R4
	ADCS R9, R5
	ADCS R10, R6
	ADCS R11, R7
	STP.P (R4, R5), 32(R3)
	STP (R6, R7), -16(R3)
	SUB $1, R0
	CBNZ R0, loop4cont
loop4done:
	ADC ZR, ZR, R1	// save & convert add carry
	MOVD R1, c+72(FP)
	RET

// func subVV(z, x, y []Word) (c Word)
TEXT ·subVV(SB), NOSPLIT, $0
	MOVD z_len+8(FP), R0
	MOVD x_base+24(FP), R1
	MOVD y_base+48(FP), R2
	MOVD z_base+0(FP), R3
	// compute unrolled loop lengths
	AND $3, R0, R4
	LSR $2, R0
	SUBS ZR, R0	// clear carry
loop1:
	CBZ R4, loop1done
loop1cont:
	// unroll 1X
	MOVD.P 8(R1), R5
	MOVD.P 8(R2), R6
	SBCS R6, R5
	MOVD.P R5, 8(R3)
	SUB $1, R4
	CBNZ R4, loop1cont
loop1done:
loop4:
	CBZ R0, loop4done
loop4cont:
	// unroll 4X
	LDP.P 32(R1), (R4, R5)
	LDP -16(R1), (R6, R7)
	LDP.P 32(R2), (R8, R9)
	LDP -16(R2), (R10, R11)
	SBCS R8, R4
	SBCS R9, R5
	SBCS R10, R6
	SBCS R11, R7
	STP.P (R4, R5), 32(R3)
	STP (R6, R7), -16(R3)
	SUB $1, R0
	CBNZ R0, loop4cont
loop4done:
	SBC R1, R1	// save carry
	SUB R1, ZR, R1	// convert sub carry
	MOVD R1, c+72(FP)
	RET

// func lshVU(z, x []Word, s uint) (c Word)
TEXT ·lshVU(SB), NOSPLIT, $0
	MOVD z_len+8(FP), R0
	CBZ R0, ret0
	MOVD s+48(FP), R1
	MOVD x_base+24(FP), R2
	MOVD z_base+0(FP), R3
	// run loop backward
	ADD R0<<3, R2, R2
	ADD R0<<3, R3, R3
	// shift first word into carry
	MOVD.W -8(R2), R4
	MOVD $64, R5
	SUB R1, R5
	LSR R5, R4, R6
	LSL R1, R4
	MOVD R6, c+56(FP)
	// shift remaining words
	SUB $1, R0
	// compute unrolled loop lengths
	AND $3, R0, R6
	LSR $2, R0
loop1:
	CBZ R6, loop1done
loop1cont:
	// unroll 1X
	MOVD.W -8(R2), R7
	LSR R5, R7, R8
	ORR R4, R8
	LSL R1, R7, R4
	MOVD.W R8, -8(R3)
	SUB $1, R6
	CBNZ R6, loop1cont
loop1done:
loop4:
	CBZ R0, loop4done
loop4cont:
	// unroll 4X
	LDP.W -32(R2), (R9, R8)
	LDP 16(R2), (R7, R6)
	LSR R5, R6, R10
	ORR R4, R10
	LSL R1, R6, R4
	LSR R5, R7, R6
	ORR R4, R6
	LSL R1, R7, R4
	LSR R5, R8, R7
	ORR R4, R7
	LSL R1, R8, R4
	LSR R5, R9, R8
	ORR R4, R8
	LSL R1, R9, R4
	STP.W (R8, R7), -32(R3)
	STP (R6, R10), 16(R3)
	SUB $1, R0
	CBNZ R0, loop4cont
loop4done:
	// store final shifted bits
	MOVD.W R4, -8(R3)
	RET
ret0:
	MOVD ZR, c+56(FP)
	RET

// func rshVU(z, x []Word, s uint) (c Word)
TEXT ·rshVU(SB), NOSPLIT, $0
	MOVD z_len+8(FP), R0
	CBZ R0, ret0
	MOVD s+48(FP), R1
	MOVD x_base+24(FP), R2
	MOVD z_base+0(FP), R3
	// shift first word into carry
	MOVD.P 8(R2), R4
	MOVD $64, R5
	SUB R1, R5
	LSL R5, R4, R6
	LSR R1, R4
	MOVD R6, c+56(FP)
	// shift remaining words
	SUB $1, R0
	// compute unrolled loop lengths
	AND $3, R0, R6
	LSR $2, R0
loop1:
	CBZ R6, loop1done
loop1cont:
	// unroll 1X
	MOVD.P 8(R2), R7
	LSL R5, R7, R8
	ORR R4, R8
	LSR R1, R7, R4
	MOVD.P R8, 8(R3)
	SUB $1, R6
	CBNZ R6, loop1cont
loop1done:
loop4:
	CBZ R0, loop4done
loop4cont:
	// unroll 4X
	LDP.P 32(R2), (R6, R7)
	LDP -16(R2), (R8, R9)
	LSL R5, R6, R10
	ORR R4, R10
	LSR R1, R6, R4
	LSL R5, R7, R6
	ORR R4, R6
	LSR R1, R7, R4
	LSL R5, R8, R7
	ORR R4, R7
	LSR R1, R8, R4
	LSL R5, R9, R8
	ORR R4, R8
	LSR R1, R9, R4
	STP.P (R10, R6), 32(R3)
	STP (R7, R8), -16(R3)
	SUB $1, R0
	CBNZ R0, loop4cont
loop4done:
	// store final shifted bits
	MOVD.P R4, 8(R3)
	RET
ret0:
	MOVD ZR, c+56(FP)
	RET

// func mulAddVWW(z, x []Word, m, a Word) (c Word)
TEXT ·mulAddVWW(SB), NOSPLIT, $0
	MOVD m+48(FP), R0
	MOVD a+56(FP), R1
	MOVD z_len+8(FP), R2
	MOVD x_base+24(FP), R3
	MOVD z_base+0(FP), R4
	// compute unrolled loop lengths
	AND $7, R2, R5
	LSR $3, R2
loop1:
	CBZ R5, loop1done
loop1cont:
	// unroll 1X
	MOVD.P 8(R3), R6
	// multiply
	UMULH R0, R6, R7
	MUL R0, R6
	ADDS R1, R6
	ADC ZR, R7, R1
	MOVD.P R6, 8(R4)
	SUB $1, R5
	CBNZ R5, loop1cont
loop1done:
loop8:
	CBZ R2, loop8done
loop8cont:
	// unroll 8X
	LDP.P 64(R3), (R5, R6)
	LDP -48(R3), (R7, R8)
	LDP -32(R3), (R9, R10)
	LDP -16(R3), (R11, R12)
	// multiply
	UMULH R0, R5, R13
	MUL R0, R5
	ADDS R1, R5
	UMULH R0, R6, R14
	MUL R0, R6
	ADCS R13, R6
	UMULH R0, R7, R13
	MUL R0, R7
	ADCS R14, R7
	UMULH R0, R8, R14
	MUL R0, R8
	ADCS R13, R8
	UMULH R0, R9, R13
	MUL R0, R9
	ADCS R14, R9
	UMULH R0, R10, R14
	MUL R0, R10
	ADCS R13, R10
	UMULH R0, R11, R13
	MUL R0, R11
	ADCS R14, R11
	UMULH R0, R12, R14
	MUL R0, R12
	ADCS R13, R12
	ADC ZR, R14, R1
	STP.P (R5, R6), 64(R4)
	STP (R7, R8), -48(R4)
	STP (R9, R10), -32(R4)
	STP (R11, R12), -16(R4)
	SUB $1, R2
	CBNZ R2, loop8cont
loop8done:
	MOVD R1, c+64(FP)
	RET

// func addMulVVWW(z, x, y []Word, m, a Word) (c Word)
TEXT ·addMulVVWW(SB), NOSPLIT, $0
	MOVD m+72(FP), R0
	MOVD a+80(FP), R1
	MOVD z_len+8(FP), R2
	MOVD x_base+24(FP), R3
	MOVD y_base+48(FP), R4
	MOVD z_base+0(FP), R5
	// compute unrolled loop lengths
	AND $7, R2, R6
	LSR $3, R2
loop1:
	CBZ R6, loop1done
loop1cont:
	// unroll 1X
	MOVD.P 8(R3), R7
	MOVD.P 8(R4), R8
	// multiply
	UMULH R0, R8, R9
	MUL R0, R8
	ADDS R1, R8
	ADC ZR, R9, R1
	// add
	ADDS R7, R8
	ADC ZR, R1
	MOVD.P R8, 8(R5)
	SUB $1, R6
	CBNZ R6, loop1cont
loop1done:
loop8:
	CBZ R2, loop8done
loop8cont:
	// unroll 8X
	LDP.P 64(R3), (R6, R7)
	LDP -48(R3), (R8, R9)
	LDP -32(R3), (R10, R11)
	LDP -16(R3), (R12, R13)
	LDP.P 64(R4), (R14, R15)
	LDP -48(R4), (R16, R17)
	LDP -32(R4), (R19, R20)
	LDP -16(R4), (R21, R22)
	// multiply
	UMULH R0, R14, R23
	MUL R0, R14
	ADDS R1, R14
	UMULH R0, R15, R24
	MUL R0, R15
	ADCS R23, R15
	UMULH R0, R16, R23
	MUL R0, R16
	ADCS R24, R16
	UMULH R0, R17, R24
	MUL R0, R17
	ADCS R23, R17
	UMULH R0, R19, R23
	MUL R0, R19
	ADCS R24, R19
	UMULH R0, R20, R24
	MUL R0, R20
	ADCS R23, R20
	UMULH R0, R21, R23
	MUL R0, R21
	ADCS R24, R21
	UMULH R0, R22, R24
	MUL R0, R22
	ADCS R23, R22
	ADC ZR, R24, R1
	// add
	ADDS R6, R14
	ADCS R7, R15
	ADCS R8, R16
	ADCS R9, R17
	ADCS R10, R19
	ADCS R11, R20
	ADCS R12, R21
	ADCS R13, R22
	ADC ZR, R1
	STP.P (R14, R15), 64(R5)
	STP (R16, R17), -48(R5)
	STP (R19, R20), -32(R5)
	STP (R21, R22), -16(R5)
	SUB $1, R2
	CBNZ R2, loop8cont
loop8done:
	MOVD R1, c+88(FP)
	RET

```

// === FILE: references!/go/src/math/big/arith_decl.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !math_big_pure_go

//go:generate go test ./internal/asmgen -generate

package big

import _ "unsafe" // for linkname

// implemented in arith_$GOARCH.s

// addVV should be an internal detail,
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/remyoudompheng/bigfft
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
//go:linkname addVV
//go:noescape
func addVV(z, x, y []Word) (c Word)

// subVV should be an internal detail,
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/remyoudompheng/bigfft
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
//go:linkname subVV
//go:noescape
func subVV(z, x, y []Word) (c Word)

// shlVU should be an internal detail (and a stale one at that),
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/remyoudompheng/bigfft
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
//go:linkname shlVU
func shlVU(z, x []Word, s uint) (c Word) {
	if s == 0 {
		copy(z, x)
		return 0
	}
	return lshVU(z, x, s)
}

// lshVU sets z = x<<s, returning the high bits c. 1 ≤ s ≤ _B-1.
//
//go:noescape
func lshVU(z, x []Word, s uint) (c Word)

// rshVU sets z = x>>s, returning the low bits c. 1 ≤ s ≤ _B-1.
//
//go:noescape
func rshVU(z, x []Word, s uint) (c Word)

// mulAddVWW should be an internal detail,
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/remyoudompheng/bigfft
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
//go:linkname mulAddVWW
//go:noescape
func mulAddVWW(z, x []Word, m, a Word) (c Word)

// addMulVVW should be an internal detail (and a stale one at that),
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/remyoudompheng/bigfft
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
//go:linkname addMulVVW
func addMulVVW(z, x []Word, y Word) (c Word) {
	return addMulVVWW(z, z, x, y, 0)
}

// addMulVVWW sets z = x+y*m+a.
//
//go:noescape
func addMulVVWW(z, x, y []Word, m, a Word) (c Word)

```

// === FILE: references!/go/src/math/big/arith_decl_pure.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build math_big_pure_go

package big

func addVV(z, x, y []Word) (c Word) {
	return addVV_g(z, x, y)
}

func subVV(z, x, y []Word) (c Word) {
	return subVV_g(z, x, y)
}

func lshVU(z, x []Word, s uint) (c Word) {
	return lshVU_g(z, x, s)
}

func rshVU(z, x []Word, s uint) (c Word) {
	return rshVU_g(z, x, s)
}

func mulAddVWW(z, x []Word, y, r Word) (c Word) {
	return mulAddVWW_g(z, x, y, r)
}

func addMulVVWW(z, x, y []Word, m, a Word) (c Word) {
	return addMulVVWW_g(z, x, y, m, a)
}

```

// === FILE: references!/go/src/math/big/arith_loong64.s ===
```text
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by 'go generate' (with ./internal/asmgen). DO NOT EDIT.

//go:build !math_big_pure_go

#include "textflag.h"

// func addVV(z, x, y []Word) (c Word)
TEXT ·addVV(SB), NOSPLIT, $0
	MOVV z_len+8(FP), R4
	MOVV x_base+24(FP), R5
	MOVV y_base+48(FP), R6
	MOVV z_base+0(FP), R7
	// compute unrolled loop lengths
	AND $3, R4, R8
	SRLV $2, R4
	XOR R28, R28	// clear carry
loop1:
	BEQ R8, loop1done
loop1cont:
	// unroll 1X
	MOVV 0(R5), R9
	MOVV 0(R6), R10
	ADDVU R10, R9	// ADCS R10, R9, R9 (cr=R28)
	SGTU R10, R9, R30	// ...
	ADDVU R28, R9	// ...
	SGTU R28, R9, R28	// ...
	ADDVU R30, R28	// ...
	MOVV R9, 0(R7)
	ADDVU $8, R5
	ADDVU $8, R6
	ADDVU $8, R7
	SUBVU $1, R8
	BNE R8, loop1cont
loop1done:
loop4:
	BEQ R4, loop4done
loop4cont:
	// unroll 4X
	MOVV 0(R5), R8
	MOVV 8(R5), R9
	MOVV 16(R5), R10
	MOVV 24(R5), R11
	MOVV 0(R6), R12
	MOVV 8(R6), R13
	MOVV 16(R6), R14
	MOVV 24(R6), R15
	ADDVU R12, R8	// ADCS R12, R8, R8 (cr=R28)
	SGTU R12, R8, R30	// ...
	ADDVU R28, R8	// ...
	SGTU R28, R8, R28	// ...
	ADDVU R30, R28	// ...
	ADDVU R13, R9	// ADCS R13, R9, R9 (cr=R28)
	SGTU R13, R9, R30	// ...
	ADDVU R28, R9	// ...
	SGTU R28, R9, R28	// ...
	ADDVU R30, R28	// ...
	ADDVU R14, R10	// ADCS R14, R10, R10 (cr=R28)
	SGTU R14, R10, R30	// ...
	ADDVU R28, R10	// ...
	SGTU R28, R10, R28	// ...
	ADDVU R30, R28	// ...
	ADDVU R15, R11	// ADCS R15, R11, R11 (cr=R28)
	SGTU R15, R11, R30	// ...
	ADDVU R28, R11	// ...
	SGTU R28, R11, R28	// ...
	ADDVU R30, R28	// ...
	MOVV R8, 0(R7)
	MOVV R9, 8(R7)
	MOVV R10, 16(R7)
	MOVV R11, 24(R7)
	ADDVU $32, R5
	ADDVU $32, R6
	ADDVU $32, R7
	SUBVU $1, R4
	BNE R4, loop4cont
loop4done:
	MOVV R28, c+72(FP)
	RET

// func subVV(z, x, y []Word) (c Word)
TEXT ·subVV(SB), NOSPLIT, $0
	MOVV z_len+8(FP), R4
	MOVV x_base+24(FP), R5
	MOVV y_base+48(FP), R6
	MOVV z_base+0(FP), R7
	// compute unrolled loop lengths
	AND $3, R4, R8
	SRLV $2, R4
	XOR R28, R28	// clear carry
loop1:
	BEQ R8, loop1done
loop1cont:
	// unroll 1X
	MOVV 0(R5), R9
	MOVV 0(R6), R10
	SGTU R28, R9, R30	// SBCS R10, R9, R9
	SUBVU R28, R9	// ...
	SGTU R10, R9, R28	// ...
	SUBVU R10, R9	// ...
	ADDVU R30, R28	// ...
	MOVV R9, 0(R7)
	ADDVU $8, R5
	ADDVU $8, R6
	ADDVU $8, R7
	SUBVU $1, R8
	BNE R8, loop1cont
loop1done:
loop4:
	BEQ R4, loop4done
loop4cont:
	// unroll 4X
	MOVV 0(R5), R8
	MOVV 8(R5), R9
	MOVV 16(R5), R10
	MOVV 24(R5), R11
	MOVV 0(R6), R12
	MOVV 8(R6), R13
	MOVV 16(R6), R14
	MOVV 24(R6), R15
	SGTU R28, R8, R30	// SBCS R12, R8, R8
	SUBVU R28, R8	// ...
	SGTU R12, R8, R28	// ...
	SUBVU R12, R8	// ...
	ADDVU R30, R28	// ...
	SGTU R28, R9, R30	// SBCS R13, R9, R9
	SUBVU R28, R9	// ...
	SGTU R13, R9, R28	// ...
	SUBVU R13, R9	// ...
	ADDVU R30, R28	// ...
	SGTU R28, R10, R30	// SBCS R14, R10, R10
	SUBVU R28, R10	// ...
	SGTU R14, R10, R28	// ...
	SUBVU R14, R10	// ...
	ADDVU R30, R28	// ...
	SGTU R28, R11, R30	// SBCS R15, R11, R11
	SUBVU R28, R11	// ...
	SGTU R15, R11, R28	// ...
	SUBVU R15, R11	// ...
	ADDVU R30, R28	// ...
	MOVV R8, 0(R7)
	MOVV R9, 8(R7)
	MOVV R10, 16(R7)
	MOVV R11, 24(R7)
	ADDVU $32, R5
	ADDVU $32, R6
	ADDVU $32, R7
	SUBVU $1, R4
	BNE R4, loop4cont
loop4done:
	MOVV R28, c+72(FP)
	RET

// func lshVU(z, x []Word, s uint) (c Word)
TEXT ·lshVU(SB), NOSPLIT, $0
	MOVV z_len+8(FP), R4
	BEQ R4, ret0
	MOVV s+48(FP), R5
	MOVV x_base+24(FP), R6
	MOVV z_base+0(FP), R7
	// run loop backward
	ALSLV $3, R4, R6, R6
	ALSLV $3, R4, R7, R7
	// shift first word into carry
	MOVV -8(R6), R8
	MOVV $64, R9
	SUBVU R5, R9
	SRLV R9, R8, R10
	SLLV R5, R8
	MOVV R10, c+56(FP)
	// shift remaining words
	SUBVU $1, R4
	// compute unrolled loop lengths
	AND $3, R4, R10
	SRLV $2, R4
loop1:
	BEQ R10, loop1done
loop1cont:
	// unroll 1X
	MOVV -16(R6), R11
	SRLV R9, R11, R12
	OR R8, R12
	SLLV R5, R11, R8
	MOVV R12, -8(R7)
	ADDVU $-8, R6
	ADDVU $-8, R7
	SUBVU $1, R10
	BNE R10, loop1cont
loop1done:
loop4:
	BEQ R4, loop4done
loop4cont:
	// unroll 4X
	MOVV -16(R6), R10
	MOVV -24(R6), R11
	MOVV -32(R6), R12
	MOVV -40(R6), R13
	SRLV R9, R10, R14
	OR R8, R14
	SLLV R5, R10, R8
	SRLV R9, R11, R10
	OR R8, R10
	SLLV R5, R11, R8
	SRLV R9, R12, R11
	OR R8, R11
	SLLV R5, R12, R8
	SRLV R9, R13, R12
	OR R8, R12
	SLLV R5, R13, R8
	MOVV R14, -8(R7)
	MOVV R10, -16(R7)
	MOVV R11, -24(R7)
	MOVV R12, -32(R7)
	ADDVU $-32, R6
	ADDVU $-32, R7
	SUBVU $1, R4
	BNE R4, loop4cont
loop4done:
	// store final shifted bits
	MOVV R8, -8(R7)
	RET
ret0:
	MOVV R0, c+56(FP)
	RET

// func rshVU(z, x []Word, s uint) (c Word)
TEXT ·rshVU(SB), NOSPLIT, $0
	MOVV z_len+8(FP), R4
	BEQ R4, ret0
	MOVV s+48(FP), R5
	MOVV x_base+24(FP), R6
	MOVV z_base+0(FP), R7
	// shift first word into carry
	MOVV 0(R6), R8
	MOVV $64, R9
	SUBVU R5, R9
	SLLV R9, R8, R10
	SRLV R5, R8
	MOVV R10, c+56(FP)
	// shift remaining words
	SUBVU $1, R4
	// compute unrolled loop lengths
	AND $3, R4, R10
	SRLV $2, R4
loop1:
	BEQ R10, loop1done
loop1cont:
	// unroll 1X
	MOVV 8(R6), R11
	SLLV R9, R11, R12
	OR R8, R12
	SRLV R5, R11, R8
	MOVV R12, 0(R7)
	ADDVU $8, R6
	ADDVU $8, R7
	SUBVU $1, R10
	BNE R10, loop1cont
loop1done:
loop4:
	BEQ R4, loop4done
loop4cont:
	// unroll 4X
	MOVV 8(R6), R10
	MOVV 16(R6), R11
	MOVV 24(R6), R12
	MOVV 32(R6), R13
	SLLV R9, R10, R14
	OR R8, R14
	SRLV R5, R10, R8
	SLLV R9, R11, R10
	OR R8, R10
	SRLV R5, R11, R8
	SLLV R9, R12, R11
	OR R8, R11
	SRLV R5, R12, R8
	SLLV R9, R13, R12
	OR R8, R12
	SRLV R5, R13, R8
	MOVV R14, 0(R7)
	MOVV R10, 8(R7)
	MOVV R11, 16(R7)
	MOVV R12, 24(R7)
	ADDVU $32, R6
	ADDVU $32, R7
	SUBVU $1, R4
	BNE R4, loop4cont
loop4done:
	// store final shifted bits
	MOVV R8, 0(R7)
	RET
ret0:
	MOVV R0, c+56(FP)
	RET

// func mulAddVWW(z, x []Word, m, a Word) (c Word)
TEXT ·mulAddVWW(SB), NOSPLIT, $0
	MOVV m+48(FP), R4
	MOVV a+56(FP), R5
	MOVV z_len+8(FP), R6
	MOVV x_base+24(FP), R7
	MOVV z_base+0(FP), R8
	// compute unrolled loop lengths
	AND $3, R6, R9
	SRLV $2, R6
loop1:
	BEQ R9, loop1done
loop1cont:
	// unroll 1X
	MOVV 0(R7), R10
	// synthetic carry, one column at a time
	MULV R4, R10, R11
	MULHVU R4, R10, R12
	ADDVU R5, R11, R10	// ADDS R5, R11, R10 (cr=R28)
	SGTU R5, R10, R28	// ...
	ADDVU R28, R12, R5	// ADC $0, R12, R5
	MOVV R10, 0(R8)
	ADDVU $8, R7
	ADDVU $8, R8
	SUBVU $1, R9
	BNE R9, loop1cont
loop1done:
loop4:
	BEQ R6, loop4done
loop4cont:
	// unroll 4X
	MOVV 0(R7), R9
	MOVV 8(R7), R10
	MOVV 16(R7), R11
	MOVV 24(R7), R12
	// synthetic carry, one column at a time
	MULV R4, R9, R13
	MULHVU R4, R9, R14
	ADDVU R5, R13, R9	// ADDS R5, R13, R9 (cr=R28)
	SGTU R5, R9, R28	// ...
	ADDVU R28, R14, R5	// ADC $0, R14, R5
	MULV R4, R10, R13
	MULHVU R4, R10, R14
	ADDVU R5, R13, R10	// ADDS R5, R13, R10 (cr=R28)
	SGTU R5, R10, R28	// ...
	ADDVU R28, R14, R5	// ADC $0, R14, R5
	MULV R4, R11, R13
	MULHVU R4, R11, R14
	ADDVU R5, R13, R11	// ADDS R5, R13, R11 (cr=R28)
	SGTU R5, R11, R28	// ...
	ADDVU R28, R14, R5	// ADC $0, R14, R5
	MULV R4, R12, R13
	MULHVU R4, R12, R14
	ADDVU R5, R13, R12	// ADDS R5, R13, R12 (cr=R28)
	SGTU R5, R12, R28	// ...
	ADDVU R28, R14, R5	// ADC $0, R14, R5
	MOVV R9, 0(R8)
	MOVV R10, 8(R8)
	MOVV R11, 16(R8)
	MOVV R12, 24(R8)
	ADDVU $32, R7
	ADDVU $32, R8
	SUBVU $1, R6
	BNE R6, loop4cont
loop4done:
	MOVV R5, c+64(FP)
	RET

// func addMulVVWW(z, x, y []Word, m, a Word) (c Word)
TEXT ·addMulVVWW(SB), NOSPLIT, $0
	MOVV m+72(FP), R4
	MOVV a+80(FP), R5
	MOVV z_len+8(FP), R6
	MOVV x_base+24(FP), R7
	MOVV y_base+48(FP), R8
	MOVV z_base+0(FP), R9
	// compute unrolled loop lengths
	AND $3, R6, R10
	SRLV $2, R6
loop1:
	BEQ R10, loop1done
loop1cont:
	// unroll 1X
	MOVV 0(R7), R11
	MOVV 0(R8), R12
	// synthetic carry, one column at a time
	MULV R4, R12, R13
	MULHVU R4, R12, R14
	ADDVU R11, R13	// ADDS R11, R13, R13 (cr=R28)
	SGTU R11, R13, R28	// ...
	ADDVU R28, R14	// ADC $0, R14, R14
	ADDVU R5, R13, R12	// ADDS R5, R13, R12 (cr=R28)
	SGTU R5, R12, R28	// ...
	ADDVU R28, R14, R5	// ADC $0, R14, R5
	MOVV R12, 0(R9)
	ADDVU $8, R7
	ADDVU $8, R8
	ADDVU $8, R9
	SUBVU $1, R10
	BNE R10, loop1cont
loop1done:
loop4:
	BEQ R6, loop4done
loop4cont:
	// unroll 4X
	MOVV 0(R7), R10
	MOVV 8(R7), R11
	MOVV 16(R7), R12
	MOVV 24(R7), R13
	MOVV 0(R8), R14
	MOVV 8(R8), R15
	MOVV 16(R8), R16
	MOVV 24(R8), R17
	// synthetic carry, one column at a time
	MULV R4, R14, R18
	MULHVU R4, R14, R19
	ADDVU R10, R18	// ADDS R10, R18, R18 (cr=R28)
	SGTU R10, R18, R28	// ...
	ADDVU R28, R19	// ADC $0, R19, R19
	ADDVU R5, R18, R14	// ADDS R5, R18, R14 (cr=R28)
	SGTU R5, R14, R28	// ...
	ADDVU R28, R19, R5	// ADC $0, R19, R5
	MULV R4, R15, R18
	MULHVU R4, R15, R19
	ADDVU R11, R18	// ADDS R11, R18, R18 (cr=R28)
	SGTU R11, R18, R28	// ...
	ADDVU R28, R19	// ADC $0, R19, R19
	ADDVU R5, R18, R15	// ADDS R5, R18, R15 (cr=R28)
	SGTU R5, R15, R28	// ...
	ADDVU R28, R19, R5	// ADC $0, R19, R5
	MULV R4, R16, R18
	MULHVU R4, R16, R19
	ADDVU R12, R18	// ADDS R12, R18, R18 (cr=R28)
	SGTU R12, R18, R28	// ...
	ADDVU R28, R19	// ADC $0, R19, R19
	ADDVU R5, R18, R16	// ADDS R5, R18, R16 (cr=R28)
	SGTU R5, R16, R28	// ...
	ADDVU R28, R19, R5	// ADC $0, R19, R5
	MULV R4, R17, R18
	MULHVU R4, R17, R19
	ADDVU R13, R18	// ADDS R13, R18, R18 (cr=R28)
	SGTU R13, R18, R28	// ...
	ADDVU R28, R19	// ADC $0, R19, R19
	ADDVU R5, R18, R17	// ADDS R5, R18, R17 (cr=R28)
	SGTU R5, R17, R28	// ...
	ADDVU R28, R19, R5	// ADC $0, R19, R5
	MOVV R14, 0(R9)
	MOVV R15, 8(R9)
	MOVV R16, 16(R9)
	MOVV R17, 24(R9)
	ADDVU $32, R7
	ADDVU $32, R8
	ADDVU $32, R9
	SUBVU $1, R6
	BNE R6, loop4cont
loop4done:
	MOVV R5, c+88(FP)
	RET

```

// === FILE: references!/go/src/math/big/arith_mips64x.s ===
```text
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by 'go generate' (with ./internal/asmgen). DO NOT EDIT.

//go:build !math_big_pure_go && (mips64 || mips64le)

#include "textflag.h"

// func addVV(z, x, y []Word) (c Word)
TEXT ·addVV(SB), NOSPLIT, $0
	MOVV z_len+8(FP), R1
	MOVV x_base+24(FP), R2
	MOVV y_base+48(FP), R3
	MOVV z_base+0(FP), R4
	// compute unrolled loop lengths
	AND $3, R1, R5
	SRLV $2, R1
	XOR R24, R24	// clear carry
loop1:
	BEQ R5, loop1done
loop1cont:
	// unroll 1X
	MOVV 0(R2), R6
	MOVV 0(R3), R7
	ADDVU R7, R6	// ADCS R7, R6, R6 (cr=R24)
	SGTU R7, R6, R23	// ...
	ADDVU R24, R6	// ...
	SGTU R24, R6, R24	// ...
	ADDVU R23, R24	// ...
	MOVV R6, 0(R4)
	ADDVU $8, R2
	ADDVU $8, R3
	ADDVU $8, R4
	SUBVU $1, R5
	BNE R5, loop1cont
loop1done:
loop4:
	BEQ R1, loop4done
loop4cont:
	// unroll 4X
	MOVV 0(R2), R5
	MOVV 8(R2), R6
	MOVV 16(R2), R7
	MOVV 24(R2), R8
	MOVV 0(R3), R9
	MOVV 8(R3), R10
	MOVV 16(R3), R11
	MOVV 24(R3), R12
	ADDVU R9, R5	// ADCS R9, R5, R5 (cr=R24)
	SGTU R9, R5, R23	// ...
	ADDVU R24, R5	// ...
	SGTU R24, R5, R24	// ...
	ADDVU R23, R24	// ...
	ADDVU R10, R6	// ADCS R10, R6, R6 (cr=R24)
	SGTU R10, R6, R23	// ...
	ADDVU R24, R6	// ...
	SGTU R24, R6, R24	// ...
	ADDVU R23, R24	// ...
	ADDVU R11, R7	// ADCS R11, R7, R7 (cr=R24)
	SGTU R11, R7, R23	// ...
	ADDVU R24, R7	// ...
	SGTU R24, R7, R24	// ...
	ADDVU R23, R24	// ...
	ADDVU R12, R8	// ADCS R12, R8, R8 (cr=R24)
	SGTU R12, R8, R23	// ...
	ADDVU R24, R8	// ...
	SGTU R24, R8, R24	// ...
	ADDVU R23, R24	// ...
	MOVV R5, 0(R4)
	MOVV R6, 8(R4)
	MOVV R7, 16(R4)
	MOVV R8, 24(R4)
	ADDVU $32, R2
	ADDVU $32, R3
	ADDVU $32, R4
	SUBVU $1, R1
	BNE R1, loop4cont
loop4done:
	MOVV R24, c+72(FP)
	RET

// func subVV(z, x, y []Word) (c Word)
TEXT ·subVV(SB), NOSPLIT, $0
	MOVV z_len+8(FP), R1
	MOVV x_base+24(FP), R2
	MOVV y_base+48(FP), R3
	MOVV z_base+0(FP), R4
	// compute unrolled loop lengths
	AND $3, R1, R5
	SRLV $2, R1
	XOR R24, R24	// clear carry
loop1:
	BEQ R5, loop1done
loop1cont:
	// unroll 1X
	MOVV 0(R2), R6
	MOVV 0(R3), R7
	SGTU R24, R6, R23	// SBCS R7, R6, R6
	SUBVU R24, R6	// ...
	SGTU R7, R6, R24	// ...
	SUBVU R7, R6	// ...
	ADDVU R23, R24	// ...
	MOVV R6, 0(R4)
	ADDVU $8, R2
	ADDVU $8, R3
	ADDVU $8, R4
	SUBVU $1, R5
	BNE R5, loop1cont
loop1done:
loop4:
	BEQ R1, loop4done
loop4cont:
	// unroll 4X
	MOVV 0(R2), R5
	MOVV 8(R2), R6
	MOVV 16(R2), R7
	MOVV 24(R2), R8
	MOVV 0(R3), R9
	MOVV 8(R3), R10
	MOVV 16(R3), R11
	MOVV 24(R3), R12
	SGTU R24, R5, R23	// SBCS R9, R5, R5
	SUBVU R24, R5	// ...
	SGTU R9, R5, R24	// ...
	SUBVU R9, R5	// ...
	ADDVU R23, R24	// ...
	SGTU R24, R6, R23	// SBCS R10, R6, R6
	SUBVU R24, R6	// ...
	SGTU R10, R6, R24	// ...
	SUBVU R10, R6	// ...
	ADDVU R23, R24	// ...
	SGTU R24, R7, R23	// SBCS R11, R7, R7
	SUBVU R24, R7	// ...
	SGTU R11, R7, R24	// ...
	SUBVU R11, R7	// ...
	ADDVU R23, R24	// ...
	SGTU R24, R8, R23	// SBCS R12, R8, R8
	SUBVU R24, R8	// ...
	SGTU R12, R8, R24	// ...
	SUBVU R12, R8	// ...
	ADDVU R23, R24	// ...
	MOVV R5, 0(R4)
	MOVV R6, 8(R4)
	MOVV R7, 16(R4)
	MOVV R8, 24(R4)
	ADDVU $32, R2
	ADDVU $32, R3
	ADDVU $32, R4
	SUBVU $1, R1
	BNE R1, loop4cont
loop4done:
	MOVV R24, c+72(FP)
	RET

// func lshVU(z, x []Word, s uint) (c Word)
TEXT ·lshVU(SB), NOSPLIT, $0
	MOVV z_len+8(FP), R1
	BEQ R1, ret0
	MOVV s+48(FP), R2
	MOVV x_base+24(FP), R3
	MOVV z_base+0(FP), R4
	// run loop backward
	SLLV $3, R1, R5
	ADDVU R5, R3
	SLLV $3, R1, R5
	ADDVU R5, R4
	// shift first word into carry
	MOVV -8(R3), R5
	MOVV $64, R6
	SUBVU R2, R6
	SRLV R6, R5, R7
	SLLV R2, R5
	MOVV R7, c+56(FP)
	// shift remaining words
	SUBVU $1, R1
	// compute unrolled loop lengths
	AND $3, R1, R7
	SRLV $2, R1
loop1:
	BEQ R7, loop1done
loop1cont:
	// unroll 1X
	MOVV -16(R3), R8
	SRLV R6, R8, R9
	OR R5, R9
	SLLV R2, R8, R5
	MOVV R9, -8(R4)
	ADDVU $-8, R3
	ADDVU $-8, R4
	SUBVU $1, R7
	BNE R7, loop1cont
loop1done:
loop4:
	BEQ R1, loop4done
loop4cont:
	// unroll 4X
	MOVV -16(R3), R7
	MOVV -24(R3), R8
	MOVV -32(R3), R9
	MOVV -40(R3), R10
	SRLV R6, R7, R11
	OR R5, R11
	SLLV R2, R7, R5
	SRLV R6, R8, R7
	OR R5, R7
	SLLV R2, R8, R5
	SRLV R6, R9, R8
	OR R5, R8
	SLLV R2, R9, R5
	SRLV R6, R10, R9
	OR R5, R9
	SLLV R2, R10, R5
	MOVV R11, -8(R4)
	MOVV R7, -16(R4)
	MOVV R8, -24(R4)
	MOVV R9, -32(R4)
	ADDVU $-32, R3
	ADDVU $-32, R4
	SUBVU $1, R1
	BNE R1, loop4cont
loop4done:
	// store final shifted bits
	MOVV R5, -8(R4)
	RET
ret0:
	MOVV R0, c+56(FP)
	RET

// func rshVU(z, x []Word, s uint) (c Word)
TEXT ·rshVU(SB), NOSPLIT, $0
	MOVV z_len+8(FP), R1
	BEQ R1, ret0
	MOVV s+48(FP), R2
	MOVV x_base+24(FP), R3
	MOVV z_base+0(FP), R4
	// shift first word into carry
	MOVV 0(R3), R5
	MOVV $64, R6
	SUBVU R2, R6
	SLLV R6, R5, R7
	SRLV R2, R5
	MOVV R7, c+56(FP)
	// shift remaining words
	SUBVU $1, R1
	// compute unrolled loop lengths
	AND $3, R1, R7
	SRLV $2, R1
loop1:
	BEQ R7, loop1done
loop1cont:
	// unroll 1X
	MOVV 8(R3), R8
	SLLV R6, R8, R9
	OR R5, R9
	SRLV R2, R8, R5
	MOVV R9, 0(R4)
	ADDVU $8, R3
	ADDVU $8, R4
	SUBVU $1, R7
	BNE R7, loop1cont
loop1done:
loop4:
	BEQ R1, loop4done
loop4cont:
	// unroll 4X
	MOVV 8(R3), R7
	MOVV 16(R3), R8
	MOVV 24(R3), R9
	MOVV 32(R3), R10
	SLLV R6, R7, R11
	OR R5, R11
	SRLV R2, R7, R5
	SLLV R6, R8, R7
	OR R5, R7
	SRLV R2, R8, R5
	SLLV R6, R9, R8
	OR R5, R8
	SRLV R2, R9, R5
	SLLV R6, R10, R9
	OR R5, R9
	SRLV R2, R10, R5
	MOVV R11, 0(R4)
	MOVV R7, 8(R4)
	MOVV R8, 16(R4)
	MOVV R9, 24(R4)
	ADDVU $32, R3
	ADDVU $32, R4
	SUBVU $1, R1
	BNE R1, loop4cont
loop4done:
	// store final shifted bits
	MOVV R5, 0(R4)
	RET
ret0:
	MOVV R0, c+56(FP)
	RET

// func mulAddVWW(z, x []Word, m, a Word) (c Word)
TEXT ·mulAddVWW(SB), NOSPLIT, $0
	MOVV m+48(FP), R1
	MOVV a+56(FP), R2
	MOVV z_len+8(FP), R3
	MOVV x_base+24(FP), R4
	MOVV z_base+0(FP), R5
	// compute unrolled loop lengths
	AND $3, R3, R6
	SRLV $2, R3
loop1:
	BEQ R6, loop1done
loop1cont:
	// unroll 1X
	MOVV 0(R4), R7
	// synthetic carry, one column at a time
	MULVU R1, R7
	MOVV LO, R8
	MOVV HI, R9
	ADDVU R2, R8, R7	// ADDS R2, R8, R7 (cr=R24)
	SGTU R2, R7, R24	// ...
	ADDVU R24, R9, R2	// ADC $0, R9, R2
	MOVV R7, 0(R5)
	ADDVU $8, R4
	ADDVU $8, R5
	SUBVU $1, R6
	BNE R6, loop1cont
loop1done:
loop4:
	BEQ R3, loop4done
loop4cont:
	// unroll 4X
	MOVV 0(R4), R6
	MOVV 8(R4), R7
	MOVV 16(R4), R8
	MOVV 24(R4), R9
	// synthetic carry, one column at a time
	MULVU R1, R6
	MOVV LO, R10
	MOVV HI, R11
	ADDVU R2, R10, R6	// ADDS R2, R10, R6 (cr=R24)
	SGTU R2, R6, R24	// ...
	ADDVU R24, R11, R2	// ADC $0, R11, R2
	MULVU R1, R7
	MOVV LO, R10
	MOVV HI, R11
	ADDVU R2, R10, R7	// ADDS R2, R10, R7 (cr=R24)
	SGTU R2, R7, R24	// ...
	ADDVU R24, R11, R2	// ADC $0, R11, R2
	MULVU R1, R8
	MOVV LO, R10
	MOVV HI, R11
	ADDVU R2, R10, R8	// ADDS R2, R10, R8 (cr=R24)
	SGTU R2, R8, R24	// ...
	ADDVU R24, R11, R2	// ADC $0, R11, R2
	MULVU R1, R9
	MOVV LO, R10
	MOVV HI, R11
	ADDVU R2, R10, R9	// ADDS R2, R10, R9 (cr=R24)
	SGTU R2, R9, R24	// ...
	ADDVU R24, R11, R2	// ADC $0, R11, R2
	MOVV R6, 0(R5)
	MOVV R7, 8(R5)
	MOVV R8, 16(R5)
	MOVV R9, 24(R5)
	ADDVU $32, R4
	ADDVU $32, R5
	SUBVU $1, R3
	BNE R3, loop4cont
loop4done:
	MOVV R2, c+64(FP)
	RET

// func addMulVVWW(z, x, y []Word, m, a Word) (c Word)
TEXT ·addMulVVWW(SB), NOSPLIT, $0
	MOVV m+72(FP), R1
	MOVV a+80(FP), R2
	MOVV z_len+8(FP), R3
	MOVV x_base+24(FP), R4
	MOVV y_base+48(FP), R5
	MOVV z_base+0(FP), R6
	// compute unrolled loop lengths
	AND $3, R3, R7
	SRLV $2, R3
loop1:
	BEQ R7, loop1done
loop1cont:
	// unroll 1X
	MOVV 0(R4), R8
	MOVV 0(R5), R9
	// synthetic carry, one column at a time
	MULVU R1, R9
	MOVV LO, R10
	MOVV HI, R11
	ADDVU R8, R10	// ADDS R8, R10, R10 (cr=R24)
	SGTU R8, R10, R24	// ...
	ADDVU R24, R11	// ADC $0, R11, R11
	ADDVU R2, R10, R9	// ADDS R2, R10, R9 (cr=R24)
	SGTU R2, R9, R24	// ...
	ADDVU R24, R11, R2	// ADC $0, R11, R2
	MOVV R9, 0(R6)
	ADDVU $8, R4
	ADDVU $8, R5
	ADDVU $8, R6
	SUBVU $1, R7
	BNE R7, loop1cont
loop1done:
loop4:
	BEQ R3, loop4done
loop4cont:
	// unroll 4X
	MOVV 0(R4), R7
	MOVV 8(R4), R8
	MOVV 16(R4), R9
	MOVV 24(R4), R10
	MOVV 0(R5), R11
	MOVV 8(R5), R12
	MOVV 16(R5), R13
	MOVV 24(R5), R14
	// synthetic carry, one column at a time
	MULVU R1, R11
	MOVV LO, R15
	MOVV HI, R16
	ADDVU R7, R15	// ADDS R7, R15, R15 (cr=R24)
	SGTU R7, R15, R24	// ...
	ADDVU R24, R16	// ADC $0, R16, R16
	ADDVU R2, R15, R11	// ADDS R2, R15, R11 (cr=R24)
	SGTU R2, R11, R24	// ...
	ADDVU R24, R16, R2	// ADC $0, R16, R2
	MULVU R1, R12
	MOVV LO, R15
	MOVV HI, R16
	ADDVU R8, R15	// ADDS R8, R15, R15 (cr=R24)
	SGTU R8, R15, R24	// ...
	ADDVU R24, R16	// ADC $0, R16, R16
	ADDVU R2, R15, R12	// ADDS R2, R15, R12 (cr=R24)
	SGTU R2, R12, R24	// ...
	ADDVU R24, R16, R2	// ADC $0, R16, R2
	MULVU R1, R13
	MOVV LO, R15
	MOVV HI, R16
	ADDVU R9, R15	// ADDS R9, R15, R15 (cr=R24)
	SGTU R9, R15, R24	// ...
	ADDVU R24, R16	// ADC $0, R16, R16
	ADDVU R2, R15, R13	// ADDS R2, R15, R13 (cr=R24)
	SGTU R2, R13, R24	// ...
	ADDVU R24, R16, R2	// ADC $0, R16, R2
	MULVU R1, R14
	MOVV LO, R15
	MOVV HI, R16
	ADDVU R10, R15	// ADDS R10, R15, R15 (cr=R24)
	SGTU R10, R15, R24	// ...
	ADDVU R24, R16	// ADC $0, R16, R16
	ADDVU R2, R15, R14	// ADDS R2, R15, R14 (cr=R24)
	SGTU R2, R14, R24	// ...
	ADDVU R24, R16, R2	// ADC $0, R16, R2
	MOVV R11, 0(R6)
	MOVV R12, 8(R6)
	MOVV R13, 16(R6)
	MOVV R14, 24(R6)
	ADDVU $32, R4
	ADDVU $32, R5
	ADDVU $32, R6
	SUBVU $1, R3
	BNE R3, loop4cont
loop4done:
	MOVV R2, c+88(FP)
	RET

```

// === FILE: references!/go/src/math/big/arith_mipsx.s ===
```text
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by 'go generate' (with ./internal/asmgen). DO NOT EDIT.

//go:build !math_big_pure_go && (mips || mipsle)

#include "textflag.h"

// func addVV(z, x, y []Word) (c Word)
TEXT ·addVV(SB), NOSPLIT, $0
	MOVW z_len+4(FP), R1
	MOVW x_base+12(FP), R2
	MOVW y_base+24(FP), R3
	MOVW z_base+0(FP), R4
	// compute unrolled loop lengths
	AND $3, R1, R5
	SRL $2, R1
	XOR R24, R24	// clear carry
loop1:
	BEQ R5, loop1done
loop1cont:
	// unroll 1X
	MOVW 0(R2), R6
	MOVW 0(R3), R7
	ADDU R7, R6	// ADCS R7, R6, R6 (cr=R24)
	SGTU R7, R6, R23	// ...
	ADDU R24, R6	// ...
	SGTU R24, R6, R24	// ...
	ADDU R23, R24	// ...
	MOVW R6, 0(R4)
	ADDU $4, R2
	ADDU $4, R3
	ADDU $4, R4
	SUBU $1, R5
	BNE R5, loop1cont
loop1done:
loop4:
	BEQ R1, loop4done
loop4cont:
	// unroll 4X
	MOVW 0(R2), R5
	MOVW 4(R2), R6
	MOVW 8(R2), R7
	MOVW 12(R2), R8
	MOVW 0(R3), R9
	MOVW 4(R3), R10
	MOVW 8(R3), R11
	MOVW 12(R3), R12
	ADDU R9, R5	// ADCS R9, R5, R5 (cr=R24)
	SGTU R9, R5, R23	// ...
	ADDU R24, R5	// ...
	SGTU R24, R5, R24	// ...
	ADDU R23, R24	// ...
	ADDU R10, R6	// ADCS R10, R6, R6 (cr=R24)
	SGTU R10, R6, R23	// ...
	ADDU R24, R6	// ...
	SGTU R24, R6, R24	// ...
	ADDU R23, R24	// ...
	ADDU R11, R7	// ADCS R11, R7, R7 (cr=R24)
	SGTU R11, R7, R23	// ...
	ADDU R24, R7	// ...
	SGTU R24, R7, R24	// ...
	ADDU R23, R24	// ...
	ADDU R12, R8	// ADCS R12, R8, R8 (cr=R24)
	SGTU R12, R8, R23	// ...
	ADDU R24, R8	// ...
	SGTU R24, R8, R24	// ...
	ADDU R23, R24	// ...
	MOVW R5, 0(R4)
	MOVW R6, 4(R4)
	MOVW R7, 8(R4)
	MOVW R8, 12(R4)
	ADDU $16, R2
	ADDU $16, R3
	ADDU $16, R4
	SUBU $1, R1
	BNE R1, loop4cont
loop4done:
	MOVW R24, c+36(FP)
	RET

// func subVV(z, x, y []Word) (c Word)
TEXT ·subVV(SB), NOSPLIT, $0
	MOVW z_len+4(FP), R1
	MOVW x_base+12(FP), R2
	MOVW y_base+24(FP), R3
	MOVW z_base+0(FP), R4
	// compute unrolled loop lengths
	AND $3, R1, R5
	SRL $2, R1
	XOR R24, R24	// clear carry
loop1:
	BEQ R5, loop1done
loop1cont:
	// unroll 1X
	MOVW 0(R2), R6
	MOVW 0(R3), R7
	SGTU R24, R6, R23	// SBCS R7, R6, R6
	SUBU R24, R6	// ...
	SGTU R7, R6, R24	// ...
	SUBU R7, R6	// ...
	ADDU R23, R24	// ...
	MOVW R6, 0(R4)
	ADDU $4, R2
	ADDU $4, R3
	ADDU $4, R4
	SUBU $1, R5
	BNE R5, loop1cont
loop1done:
loop4:
	BEQ R1, loop4done
loop4cont:
	// unroll 4X
	MOVW 0(R2), R5
	MOVW 4(R2), R6
	MOVW 8(R2), R7
	MOVW 12(R2), R8
	MOVW 0(R3), R9
	MOVW 4(R3), R10
	MOVW 8(R3), R11
	MOVW 12(R3), R12
	SGTU R24, R5, R23	// SBCS R9, R5, R5
	SUBU R24, R5	// ...
	SGTU R9, R5, R24	// ...
	SUBU R9, R5	// ...
	ADDU R23, R24	// ...
	SGTU R24, R6, R23	// SBCS R10, R6, R6
	SUBU R24, R6	// ...
	SGTU R10, R6, R24	// ...
	SUBU R10, R6	// ...
	ADDU R23, R24	// ...
	SGTU R24, R7, R23	// SBCS R11, R7, R7
	SUBU R24, R7	// ...
	SGTU R11, R7, R24	// ...
	SUBU R11, R7	// ...
	ADDU R23, R24	// ...
	SGTU R24, R8, R23	// SBCS R12, R8, R8
	SUBU R24, R8	// ...
	SGTU R12, R8, R24	// ...
	SUBU R12, R8	// ...
	ADDU R23, R24	// ...
	MOVW R5, 0(R4)
	MOVW R6, 4(R4)
	MOVW R7, 8(R4)
	MOVW R8, 12(R4)
	ADDU $16, R2
	ADDU $16, R3
	ADDU $16, R4
	SUBU $1, R1
	BNE R1, loop4cont
loop4done:
	MOVW R24, c+36(FP)
	RET

// func lshVU(z, x []Word, s uint) (c Word)
TEXT ·lshVU(SB), NOSPLIT, $0
	MOVW z_len+4(FP), R1
	BEQ R1, ret0
	MOVW s+24(FP), R2
	MOVW x_base+12(FP), R3
	MOVW z_base+0(FP), R4
	// run loop backward
	SLL $2, R1, R5
	ADDU R5, R3
	SLL $2, R1, R5
	ADDU R5, R4
	// shift first word into carry
	MOVW -4(R3), R5
	MOVW $32, R6
	SUBU R2, R6
	SRL R6, R5, R7
	SLL R2, R5
	MOVW R7, c+28(FP)
	// shift remaining words
	SUBU $1, R1
	// compute unrolled loop lengths
	AND $3, R1, R7
	SRL $2, R1
loop1:
	BEQ R7, loop1done
loop1cont:
	// unroll 1X
	MOVW -8(R3), R8
	SRL R6, R8, R9
	OR R5, R9
	SLL R2, R8, R5
	MOVW R9, -4(R4)
	ADDU $-4, R3
	ADDU $-4, R4
	SUBU $1, R7
	BNE R7, loop1cont
loop1done:
loop4:
	BEQ R1, loop4done
loop4cont:
	// unroll 4X
	MOVW -8(R3), R7
	MOVW -12(R3), R8
	MOVW -16(R3), R9
	MOVW -20(R3), R10
	SRL R6, R7, R11
	OR R5, R11
	SLL R2, R7, R5
	SRL R6, R8, R7
	OR R5, R7
	SLL R2, R8, R5
	SRL R6, R9, R8
	OR R5, R8
	SLL R2, R9, R5
	SRL R6, R10, R9
	OR R5, R9
	SLL R2, R10, R5
	MOVW R11, -4(R4)
	MOVW R7, -8(R4)
	MOVW R8, -12(R4)
	MOVW R9, -16(R4)
	ADDU $-16, R3
	ADDU $-16, R4
	SUBU $1, R1
	BNE R1, loop4cont
loop4done:
	// store final shifted bits
	MOVW R5, -4(R4)
	RET
ret0:
	MOVW R0, c+28(FP)
	RET

// func rshVU(z, x []Word, s uint) (c Word)
TEXT ·rshVU(SB), NOSPLIT, $0
	MOVW z_len+4(FP), R1
	BEQ R1, ret0
	MOVW s+24(FP), R2
	MOVW x_base+12(FP), R3
	MOVW z_base+0(FP), R4
	// shift first word into carry
	MOVW 0(R3), R5
	MOVW $32, R6
	SUBU R2, R6
	SLL R6, R5, R7
	SRL R2, R5
	MOVW R7, c+28(FP)
	// shift remaining words
	SUBU $1, R1
	// compute unrolled loop lengths
	AND $3, R1, R7
	SRL $2, R1
loop1:
	BEQ R7, loop1done
loop1cont:
	// unroll 1X
	MOVW 4(R3), R8
	SLL R6, R8, R9
	OR R5, R9
	SRL R2, R8, R5
	MOVW R9, 0(R4)
	ADDU $4, R3
	ADDU $4, R4
	SUBU $1, R7
	BNE R7, loop1cont
loop1done:
loop4:
	BEQ R1, loop4done
loop4cont:
	// unroll 4X
	MOVW 4(R3), R7
	MOVW 8(R3), R8
	MOVW 12(R3), R9
	MOVW 16(R3), R10
	SLL R6, R7, R11
	OR R5, R11
	SRL R2, R7, R5
	SLL R6, R8, R7
	OR R5, R7
	SRL R2, R8, R5
	SLL R6, R9, R8
	OR R5, R8
	SRL R2, R9, R5
	SLL R6, R10, R9
	OR R5, R9
	SRL R2, R10, R5
	MOVW R11, 0(R4)
	MOVW R7, 4(R4)
	MOVW R8, 8(R4)
	MOVW R9, 12(R4)
	ADDU $16, R3
	ADDU $16, R4
	SUBU $1, R1
	BNE R1, loop4cont
loop4done:
	// store final shifted bits
	MOVW R5, 0(R4)
	RET
ret0:
	MOVW R0, c+28(FP)
	RET

// func mulAddVWW(z, x []Word, m, a Word) (c Word)
TEXT ·mulAddVWW(SB), NOSPLIT, $0
	MOVW m+24(FP), R1
	MOVW a+28(FP), R2
	MOVW z_len+4(FP), R3
	MOVW x_base+12(FP), R4
	MOVW z_base+0(FP), R5
	// compute unrolled loop lengths
	AND $3, R3, R6
	SRL $2, R3
loop1:
	BEQ R6, loop1done
loop1cont:
	// unroll 1X
	MOVW 0(R4), R7
	// synthetic carry, one column at a time
	MULU R1, R7
	MOVW LO, R8
	MOVW HI, R9
	ADDU R2, R8, R7	// ADDS R2, R8, R7 (cr=R24)
	SGTU R2, R7, R24	// ...
	ADDU R24, R9, R2	// ADC $0, R9, R2
	MOVW R7, 0(R5)
	ADDU $4, R4
	ADDU $4, R5
	SUBU $1, R6
	BNE R6, loop1cont
loop1done:
loop4:
	BEQ R3, loop4done
loop4cont:
	// unroll 4X
	MOVW 0(R4), R6
	MOVW 4(R4), R7
	MOVW 8(R4), R8
	MOVW 12(R4), R9
	// synthetic carry, one column at a time
	MULU R1, R6
	MOVW LO, R10
	MOVW HI, R11
	ADDU R2, R10, R6	// ADDS R2, R10, R6 (cr=R24)
	SGTU R2, R6, R24	// ...
	ADDU R24, R11, R2	// ADC $0, R11, R2
	MULU R1, R7
	MOVW LO, R10
	MOVW HI, R11
	ADDU R2, R10, R7	// ADDS R2, R10, R7 (cr=R24)
	SGTU R2, R7, R24	// ...
	ADDU R24, R11, R2	// ADC $0, R11, R2
	MULU R1, R8
	MOVW LO, R10
	MOVW HI, R11
	ADDU R2, R10, R8	// ADDS R2, R10, R8 (cr=R24)
	SGTU R2, R8, R24	// ...
	ADDU R24, R11, R2	// ADC $0, R11, R2
	MULU R1, R9
	MOVW LO, R10
	MOVW HI, R11
	ADDU R2, R10, R9	// ADDS R2, R10, R9 (cr=R24)
	SGTU R2, R9, R24	// ...
	ADDU R24, R11, R2	// ADC $0, R11, R2
	MOVW R6, 0(R5)
	MOVW R7, 4(R5)
	MOVW R8, 8(R5)
	MOVW R9, 12(R5)
	ADDU $16, R4
	ADDU $16, R5
	SUBU $1, R3
	BNE R3, loop4cont
loop4done:
	MOVW R2, c+32(FP)
	RET

// func addMulVVWW(z, x, y []Word, m, a Word) (c Word)
TEXT ·addMulVVWW(SB), NOSPLIT, $0
	MOVW m+36(FP), R1
	MOVW a+40(FP), R2
	MOVW z_len+4(FP), R3
	MOVW x_base+12(FP), R4
	MOVW y_base+24(FP), R5
	MOVW z_base+0(FP), R6
	// compute unrolled loop lengths
	AND $3, R3, R7
	SRL $2, R3
loop1:
	BEQ R7, loop1done
loop1cont:
	// unroll 1X
	MOVW 0(R4), R8
	MOVW 0(R5), R9
	// synthetic carry, one column at a time
	MULU R1, R9
	MOVW LO, R10
	MOVW HI, R11
	ADDU R8, R10	// ADDS R8, R10, R10 (cr=R24)
	SGTU R8, R10, R24	// ...
	ADDU R24, R11	// ADC $0, R11, R11
	ADDU R2, R10, R9	// ADDS R2, R10, R9 (cr=R24)
	SGTU R2, R9, R24	// ...
	ADDU R24, R11, R2	// ADC $0, R11, R2
	MOVW R9, 0(R6)
	ADDU $4, R4
	ADDU $4, R5
	ADDU $4, R6
	SUBU $1, R7
	BNE R7, loop1cont
loop1done:
loop4:
	BEQ R3, loop4done
loop4cont:
	// unroll 4X
	MOVW 0(R4), R7
	MOVW 4(R4), R8
	MOVW 8(R4), R9
	MOVW 12(R4), R10
	MOVW 0(R5), R11
	MOVW 4(R5), R12
	MOVW 8(R5), R13
	MOVW 12(R5), R14
	// synthetic carry, one column at a time
	MULU R1, R11
	MOVW LO, R15
	MOVW HI, R16
	ADDU R7, R15	// ADDS R7, R15, R15 (cr=R24)
	SGTU R7, R15, R24	// ...
	ADDU R24, R16	// ADC $0, R16, R16
	ADDU R2, R15, R11	// ADDS R2, R15, R11 (cr=R24)
	SGTU R2, R11, R24	// ...
	ADDU R24, R16, R2	// ADC $0, R16, R2
	MULU R1, R12
	MOVW LO, R15
	MOVW HI, R16
	ADDU R8, R15	// ADDS R8, R15, R15 (cr=R24)
	SGTU R8, R15, R24	// ...
	ADDU R24, R16	// ADC $0, R16, R16
	ADDU R2, R15, R12	// ADDS R2, R15, R12 (cr=R24)
	SGTU R2, R12, R24	// ...
	ADDU R24, R16, R2	// ADC $0, R16, R2
	MULU R1, R13
	MOVW LO, R15
	MOVW HI, R16
	ADDU R9, R15	// ADDS R9, R15, R15 (cr=R24)
	SGTU R9, R15, R24	// ...
	ADDU R24, R16	// ADC $0, R16, R16
	ADDU R2, R15, R13	// ADDS R2, R15, R13 (cr=R24)
	SGTU R2, R13, R24	// ...
	ADDU R24, R16, R2	// ADC $0, R16, R2
	MULU R1, R14
	MOVW LO, R15
	MOVW HI, R16
	ADDU R10, R15	// ADDS R10, R15, R15 (cr=R24)
	SGTU R10, R15, R24	// ...
	ADDU R24, R16	// ADC $0, R16, R16
	ADDU R2, R15, R14	// ADDS R2, R15, R14 (cr=R24)
	SGTU R2, R14, R24	// ...
	ADDU R24, R16, R2	// ADC $0, R16, R2
	MOVW R11, 0(R6)
	MOVW R12, 4(R6)
	MOVW R13, 8(R6)
	MOVW R14, 12(R6)
	ADDU $16, R4
	ADDU $16, R5
	ADDU $16, R6
	SUBU $1, R3
	BNE R3, loop4cont
loop4done:
	MOVW R2, c+44(FP)
	RET

```

// === FILE: references!/go/src/math/big/arith_ppc64x.s ===
```text
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by 'go generate' (with ./internal/asmgen). DO NOT EDIT.

//go:build !math_big_pure_go && (ppc64 || ppc64le)

#include "textflag.h"

// func addVV(z, x, y []Word) (c Word)
TEXT ·addVV(SB), NOSPLIT, $0
	MOVD z_len+8(FP), R3
	MOVD x_base+24(FP), R4
	MOVD y_base+48(FP), R5
	MOVD z_base+0(FP), R6
	// compute unrolled loop lengths
	ANDCC $3, R3, R7
	SRD $2, R3
	ADDC R0, R3	// clear carry
loop1:
	CMP R7, $0; BEQ loop1done; MOVD R7, CTR
loop1cont:
	// unroll 1X
	MOVD 0(R4), R8
	MOVD 0(R5), R9
	ADDE R9, R8
	MOVD R8, 0(R6)
	ADD $8, R4
	ADD $8, R5
	ADD $8, R6
	BDNZ loop1cont
loop1done:
loop4:
	CMP R3, $0; BEQ loop4done; MOVD R3, CTR
loop4cont:
	// unroll 4X
	MOVD 0(R4), R7
	MOVD 8(R4), R8
	MOVD 16(R4), R9
	MOVD 24(R4), R10
	MOVD 0(R5), R11
	MOVD 8(R5), R12
	MOVD 16(R5), R14
	MOVD 24(R5), R15
	ADDE R11, R7
	ADDE R12, R8
	ADDE R14, R9
	ADDE R15, R10
	MOVD R7, 0(R6)
	MOVD R8, 8(R6)
	MOVD R9, 16(R6)
	MOVD R10, 24(R6)
	ADD $32, R4
	ADD $32, R5
	ADD $32, R6
	BDNZ loop4cont
loop4done:
	ADDE R0, R0, R4	// save & convert add carry
	MOVD R4, c+72(FP)
	RET

// func subVV(z, x, y []Word) (c Word)
TEXT ·subVV(SB), NOSPLIT, $0
	MOVD z_len+8(FP), R3
	MOVD x_base+24(FP), R4
	MOVD y_base+48(FP), R5
	MOVD z_base+0(FP), R6
	// compute unrolled loop lengths
	ANDCC $3, R3, R7
	SRD $2, R3
	SUBC R0, R3	// clear carry
loop1:
	CMP R7, $0; BEQ loop1done; MOVD R7, CTR
loop1cont:
	// unroll 1X
	MOVD 0(R4), R8
	MOVD 0(R5), R9
	SUBE R9, R8
	MOVD R8, 0(R6)
	ADD $8, R4
	ADD $8, R5
	ADD $8, R6
	BDNZ loop1cont
loop1done:
loop4:
	CMP R3, $0; BEQ loop4done; MOVD R3, CTR
loop4cont:
	// unroll 4X
	MOVD 0(R4), R7
	MOVD 8(R4), R8
	MOVD 16(R4), R9
	MOVD 24(R4), R10
	MOVD 0(R5), R11
	MOVD 8(R5), R12
	MOVD 16(R5), R14
	MOVD 24(R5), R15
	SUBE R11, R7
	SUBE R12, R8
	SUBE R14, R9
	SUBE R15, R10
	MOVD R7, 0(R6)
	MOVD R8, 8(R6)
	MOVD R9, 16(R6)
	MOVD R10, 24(R6)
	ADD $32, R4
	ADD $32, R5
	ADD $32, R6
	BDNZ loop4cont
loop4done:
	SUBE R4, R4	// save carry
	SUB R4, R0, R4	// convert sub carry
	MOVD R4, c+72(FP)
	RET

// func lshVU(z, x []Word, s uint) (c Word)
TEXT ·lshVU(SB), NOSPLIT, $0
	MOVD z_len+8(FP), R3
	CMP R3, $0; BEQ ret0
	MOVD s+48(FP), R4
	MOVD x_base+24(FP), R5
	MOVD z_base+0(FP), R6
	// run loop backward
	SLD $3, R3, R7
	ADD R7, R5
	SLD $3, R3, R7
	ADD R7, R6
	// shift first word into carry
	MOVD -8(R5), R7
	MOVD $64, R8
	SUB R4, R8
	SRD R8, R7, R9
	SLD R4, R7
	MOVD R9, c+56(FP)
	// shift remaining words
	SUB $1, R3
	// compute unrolled loop lengths
	ANDCC $3, R3, R9
	SRD $2, R3
loop1:
	CMP R9, $0; BEQ loop1done; MOVD R9, CTR
loop1cont:
	// unroll 1X
	MOVD -16(R5), R10
	SRD R8, R10, R11
	OR R7, R11
	SLD R4, R10, R7
	MOVD R11, -8(R6)
	ADD $-8, R5
	ADD $-8, R6
	BDNZ loop1cont
loop1done:
loop4:
	CMP R3, $0; BEQ loop4done; MOVD R3, CTR
loop4cont:
	// unroll 4X
	MOVD -16(R5), R9
	MOVD -24(R5), R10
	MOVD -32(R5), R11
	MOVD -40(R5), R12
	SRD R8, R9, R14
	OR R7, R14
	SLD R4, R9, R7
	SRD R8, R10, R9
	OR R7, R9
	SLD R4, R10, R7
	SRD R8, R11, R10
	OR R7, R10
	SLD R4, R11, R7
	SRD R8, R12, R11
	OR R7, R11
	SLD R4, R12, R7
	MOVD R14, -8(R6)
	MOVD R9, -16(R6)
	MOVD R10, -24(R6)
	MOVD R11, -32(R6)
	ADD $-32, R5
	ADD $-32, R6
	BDNZ loop4cont
loop4done:
	// store final shifted bits
	MOVD R7, -8(R6)
	RET
ret0:
	MOVD R0, c+56(FP)
	RET

// func rshVU(z, x []Word, s uint) (c Word)
TEXT ·rshVU(SB), NOSPLIT, $0
	MOVD z_len+8(FP), R3
	CMP R3, $0; BEQ ret0
	MOVD s+48(FP), R4
	MOVD x_base+24(FP), R5
	MOVD z_base+0(FP), R6
	// shift first word into carry
	MOVD 0(R5), R7
	MOVD $64, R8
	SUB R4, R8
	SLD R8, R7, R9
	SRD R4, R7
	MOVD R9, c+56(FP)
	// shift remaining words
	SUB $1, R3
	// compute unrolled loop lengths
	ANDCC $3, R3, R9
	SRD $2, R3
loop1:
	CMP R9, $0; BEQ loop1done; MOVD R9, CTR
loop1cont:
	// unroll 1X
	MOVD 8(R5), R10
	SLD R8, R10, R11
	OR R7, R11
	SRD R4, R10, R7
	MOVD R11, 0(R6)
	ADD $8, R5
	ADD $8, R6
	BDNZ loop1cont
loop1done:
loop4:
	CMP R3, $0; BEQ loop4done; MOVD R3, CTR
loop4cont:
	// unroll 4X
	MOVD 8(R5), R9
	MOVD 16(R5), R10
	MOVD 24(R5), R11
	MOVD 32(R5), R12
	SLD R8, R9, R14
	OR R7, R14
	SRD R4, R9, R7
	SLD R8, R10, R9
	OR R7, R9
	SRD R4, R10, R7
	SLD R8, R11, R10
	OR R7, R10
	SRD R4, R11, R7
	SLD R8, R12, R11
	OR R7, R11
	SRD R4, R12, R7
	MOVD R14, 0(R6)
	MOVD R9, 8(R6)
	MOVD R10, 16(R6)
	MOVD R11, 24(R6)
	ADD $32, R5
	ADD $32, R6
	BDNZ loop4cont
loop4done:
	// store final shifted bits
	MOVD R7, 0(R6)
	RET
ret0:
	MOVD R0, c+56(FP)
	RET

// func mulAddVWW(z, x []Word, m, a Word) (c Word)
TEXT ·mulAddVWW(SB), NOSPLIT, $0
	MOVD m+48(FP), R3
	MOVD a+56(FP), R4
	MOVD z_len+8(FP), R5
	MOVD x_base+24(FP), R6
	MOVD z_base+0(FP), R7
	// compute unrolled loop lengths
	ANDCC $3, R5, R8
	SRD $2, R5
loop1:
	CMP R8, $0; BEQ loop1done; MOVD R8, CTR
loop1cont:
	// unroll 1X
	MOVD 0(R6), R9
	// multiply
	MULHDU R3, R9, R10
	MULLD R3, R9
	ADDC R4, R9
	ADDE R0, R10, R4
	MOVD R9, 0(R7)
	ADD $8, R6
	ADD $8, R7
	BDNZ loop1cont
loop1done:
loop4:
	CMP R5, $0; BEQ loop4done; MOVD R5, CTR
loop4cont:
	// unroll 4X
	MOVD 0(R6), R8
	MOVD 8(R6), R9
	MOVD 16(R6), R10
	MOVD 24(R6), R11
	// multiply
	MULHDU R3, R8, R12
	MULLD R3, R8
	ADDC R4, R8
	MULHDU R3, R9, R14
	MULLD R3, R9
	ADDE R12, R9
	MULHDU R3, R10, R12
	MULLD R3, R10
	ADDE R14, R10
	MULHDU R3, R11, R14
	MULLD R3, R11
	ADDE R12, R11
	ADDE R0, R14, R4
	MOVD R8, 0(R7)
	MOVD R9, 8(R7)
	MOVD R10, 16(R7)
	MOVD R11, 24(R7)
	ADD $32, R6
	ADD $32, R7
	BDNZ loop4cont
loop4done:
	MOVD R4, c+64(FP)
	RET

// func addMulVVWW(z, x, y []Word, m, a Word) (c Word)
TEXT ·addMulVVWW(SB), NOSPLIT, $0
	MOVD m+72(FP), R3
	MOVD a+80(FP), R4
	MOVD z_len+8(FP), R5
	MOVD x_base+24(FP), R6
	MOVD y_base+48(FP), R7
	MOVD z_base+0(FP), R8
	// compute unrolled loop lengths
	ANDCC $3, R5, R9
	SRD $2, R5
loop1:
	CMP R9, $0; BEQ loop1done; MOVD R9, CTR
loop1cont:
	// unroll 1X
	MOVD 0(R6), R10
	MOVD 0(R7), R11
	// multiply
	MULHDU R3, R11, R12
	MULLD R3, R11
	ADDC R4, R11
	ADDE R0, R12, R4
	// add
	ADDC R10, R11
	ADDE R0, R4
	MOVD R11, 0(R8)
	ADD $8, R6
	ADD $8, R7
	ADD $8, R8
	BDNZ loop1cont
loop1done:
loop4:
	CMP R5, $0; BEQ loop4done; MOVD R5, CTR
loop4cont:
	// unroll 4X
	MOVD 0(R6), R9
	MOVD 8(R6), R10
	MOVD 16(R6), R11
	MOVD 24(R6), R12
	MOVD 0(R7), R14
	MOVD 8(R7), R15
	MOVD 16(R7), R16
	MOVD 24(R7), R17
	// multiply
	MULHDU R3, R14, R18
	MULLD R3, R14
	ADDC R4, R14
	MULHDU R3, R15, R19
	MULLD R3, R15
	ADDE R18, R15
	MULHDU R3, R16, R18
	MULLD R3, R16
	ADDE R19, R16
	MULHDU R3, R17, R19
	MULLD R3, R17
	ADDE R18, R17
	ADDE R0, R19, R4
	// add
	ADDC R9, R14
	ADDE R10, R15
	ADDE R11, R16
	ADDE R12, R17
	ADDE R0, R4
	MOVD R14, 0(R8)
	MOVD R15, 8(R8)
	MOVD R16, 16(R8)
	MOVD R17, 24(R8)
	ADD $32, R6
	ADD $32, R7
	ADD $32, R8
	BDNZ loop4cont
loop4done:
	MOVD R4, c+88(FP)
	RET

```

// === FILE: references!/go/src/math/big/arith_riscv64.s ===
```text
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by 'go generate' (with ./internal/asmgen). DO NOT EDIT.

//go:build !math_big_pure_go

#include "textflag.h"

// func addVV(z, x, y []Word) (c Word)
TEXT ·addVV(SB), NOSPLIT, $0
	MOV z_len+8(FP), X5
	MOV x_base+24(FP), X6
	MOV y_base+48(FP), X7
	MOV z_base+0(FP), X8
	// compute unrolled loop lengths
	AND $3, X5, X9
	SRL $2, X5
	XOR X28, X28	// clear carry
loop1:
	BEQZ X9, loop1done
loop1cont:
	// unroll 1X
	MOV 0(X6), X10
	MOV 0(X7), X11
	ADD X11, X10	// ADCS X11, X10, X10 (cr=X28)
	SLTU X11, X10, X31	// ...
	ADD X28, X10	// ...
	SLTU X28, X10, X28	// ...
	ADD X31, X28	// ...
	MOV X10, 0(X8)
	ADD $8, X6
	ADD $8, X7
	ADD $8, X8
	SUB $1, X9
	BNEZ X9, loop1cont
loop1done:
loop4:
	BEQZ X5, loop4done
loop4cont:
	// unroll 4X
	MOV 0(X6), X9
	MOV 8(X6), X10
	MOV 16(X6), X11
	MOV 24(X6), X12
	MOV 0(X7), X13
	MOV 8(X7), X14
	MOV 16(X7), X15
	MOV 24(X7), X16
	ADD X13, X9	// ADCS X13, X9, X9 (cr=X28)
	SLTU X13, X9, X31	// ...
	ADD X28, X9	// ...
	SLTU X28, X9, X28	// ...
	ADD X31, X28	// ...
	ADD X14, X10	// ADCS X14, X10, X10 (cr=X28)
	SLTU X14, X10, X31	// ...
	ADD X28, X10	// ...
	SLTU X28, X10, X28	// ...
	ADD X31, X28	// ...
	ADD X15, X11	// ADCS X15, X11, X11 (cr=X28)
	SLTU X15, X11, X31	// ...
	ADD X28, X11	// ...
	SLTU X28, X11, X28	// ...
	ADD X31, X28	// ...
	ADD X16, X12	// ADCS X16, X12, X12 (cr=X28)
	SLTU X16, X12, X31	// ...
	ADD X28, X12	// ...
	SLTU X28, X12, X28	// ...
	ADD X31, X28	// ...
	MOV X9, 0(X8)
	MOV X10, 8(X8)
	MOV X11, 16(X8)
	MOV X12, 24(X8)
	ADD $32, X6
	ADD $32, X7
	ADD $32, X8
	SUB $1, X5
	BNEZ X5, loop4cont
loop4done:
	MOV X28, c+72(FP)
	RET

// func subVV(z, x, y []Word) (c Word)
TEXT ·subVV(SB), NOSPLIT, $0
	MOV z_len+8(FP), X5
	MOV x_base+24(FP), X6
	MOV y_base+48(FP), X7
	MOV z_base+0(FP), X8
	// compute unrolled loop lengths
	AND $3, X5, X9
	SRL $2, X5
	XOR X28, X28	// clear carry
loop1:
	BEQZ X9, loop1done
loop1cont:
	// unroll 1X
	MOV 0(X6), X10
	MOV 0(X7), X11
	SLTU X28, X10, X31	// SBCS X11, X10, X10
	SUB X28, X10	// ...
	SLTU X11, X10, X28	// ...
	SUB X11, X10	// ...
	ADD X31, X28	// ...
	MOV X10, 0(X8)
	ADD $8, X6
	ADD $8, X7
	ADD $8, X8
	SUB $1, X9
	BNEZ X9, loop1cont
loop1done:
loop4:
	BEQZ X5, loop4done
loop4cont:
	// unroll 4X
	MOV 0(X6), X9
	MOV 8(X6), X10
	MOV 16(X6), X11
	MOV 24(X6), X12
	MOV 0(X7), X13
	MOV 8(X7), X14
	MOV 16(X7), X15
	MOV 24(X7), X16
	SLTU X28, X9, X31	// SBCS X13, X9, X9
	SUB X28, X9	// ...
	SLTU X13, X9, X28	// ...
	SUB X13, X9	// ...
	ADD X31, X28	// ...
	SLTU X28, X10, X31	// SBCS X14, X10, X10
	SUB X28, X10	// ...
	SLTU X14, X10, X28	// ...
	SUB X14, X10	// ...
	ADD X31, X28	// ...
	SLTU X28, X11, X31	// SBCS X15, X11, X11
	SUB X28, X11	// ...
	SLTU X15, X11, X28	// ...
	SUB X15, X11	// ...
	ADD X31, X28	// ...
	SLTU X28, X12, X31	// SBCS X16, X12, X12
	SUB X28, X12	// ...
	SLTU X16, X12, X28	// ...
	SUB X16, X12	// ...
	ADD X31, X28	// ...
	MOV X9, 0(X8)
	MOV X10, 8(X8)
	MOV X11, 16(X8)
	MOV X12, 24(X8)
	ADD $32, X6
	ADD $32, X7
	ADD $32, X8
	SUB $1, X5
	BNEZ X5, loop4cont
loop4done:
	MOV X28, c+72(FP)
	RET

// func lshVU(z, x []Word, s uint) (c Word)
TEXT ·lshVU(SB), NOSPLIT, $0
	MOV z_len+8(FP), X5
	BEQZ X5, ret0
	MOV s+48(FP), X6
	MOV x_base+24(FP), X7
	MOV z_base+0(FP), X8
	// run loop backward
	SLL $3, X5, X9
	ADD X9, X7
	SLL $3, X5, X9
	ADD X9, X8
	// shift first word into carry
	MOV -8(X7), X9
	MOV $64, X10
	SUB X6, X10
	SRL X10, X9, X11
	SLL X6, X9
	MOV X11, c+56(FP)
	// shift remaining words
	SUB $1, X5
	// compute unrolled loop lengths
	AND $3, X5, X11
	SRL $2, X5
loop1:
	BEQZ X11, loop1done
loop1cont:
	// unroll 1X
	MOV -16(X7), X12
	SRL X10, X12, X13
	OR X9, X13
	SLL X6, X12, X9
	MOV X13, -8(X8)
	ADD $-8, X7
	ADD $-8, X8
	SUB $1, X11
	BNEZ X11, loop1cont
loop1done:
loop4:
	BEQZ X5, loop4done
loop4cont:
	// unroll 4X
	MOV -16(X7), X11
	MOV -24(X7), X12
	MOV -32(X7), X13
	MOV -40(X7), X14
	SRL X10, X11, X15
	OR X9, X15
	SLL X6, X11, X9
	SRL X10, X12, X11
	OR X9, X11
	SLL X6, X12, X9
	SRL X10, X13, X12
	OR X9, X12
	SLL X6, X13, X9
	SRL X10, X14, X13
	OR X9, X13
	SLL X6, X14, X9
	MOV X15, -8(X8)
	MOV X11, -16(X8)
	MOV X12, -24(X8)
	MOV X13, -32(X8)
	ADD $-32, X7
	ADD $-32, X8
	SUB $1, X5
	BNEZ X5, loop4cont
loop4done:
	// store final shifted bits
	MOV X9, -8(X8)
	RET
ret0:
	MOV X0, c+56(FP)
	RET

// func rshVU(z, x []Word, s uint) (c Word)
TEXT ·rshVU(SB), NOSPLIT, $0
	MOV z_len+8(FP), X5
	BEQZ X5, ret0
	MOV s+48(FP), X6
	MOV x_base+24(FP), X7
	MOV z_base+0(FP), X8
	// shift first word into carry
	MOV 0(X7), X9
	MOV $64, X10
	SUB X6, X10
	SLL X10, X9, X11
	SRL X6, X9
	MOV X11, c+56(FP)
	// shift remaining words
	SUB $1, X5
	// compute unrolled loop lengths
	AND $3, X5, X11
	SRL $2, X5
loop1:
	BEQZ X11, loop1done
loop1cont:
	// unroll 1X
	MOV 8(X7), X12
	SLL X10, X12, X13
	OR X9, X13
	SRL X6, X12, X9
	MOV X13, 0(X8)
	ADD $8, X7
	ADD $8, X8
	SUB $1, X11
	BNEZ X11, loop1cont
loop1done:
loop4:
	BEQZ X5, loop4done
loop4cont:
	// unroll 4X
	MOV 8(X7), X11
	MOV 16(X7), X12
	MOV 24(X7), X13
	MOV 32(X7), X14
	SLL X10, X11, X15
	OR X9, X15
	SRL X6, X11, X9
	SLL X10, X12, X11
	OR X9, X11
	SRL X6, X12, X9
	SLL X10, X13, X12
	OR X9, X12
	SRL X6, X13, X9
	SLL X10, X14, X13
	OR X9, X13
	SRL X6, X14, X9
	MOV X15, 0(X8)
	MOV X11, 8(X8)
	MOV X12, 16(X8)
	MOV X13, 24(X8)
	ADD $32, X7
	ADD $32, X8
	SUB $1, X5
	BNEZ X5, loop4cont
loop4done:
	// store final shifted bits
	MOV X9, 0(X8)
	RET
ret0:
	MOV X0, c+56(FP)
	RET

// func mulAddVWW(z, x []Word, m, a Word) (c Word)
TEXT ·mulAddVWW(SB), NOSPLIT, $0
	MOV m+48(FP), X5
	MOV a+56(FP), X6
	MOV z_len+8(FP), X7
	MOV x_base+24(FP), X8
	MOV z_base+0(FP), X9
	// compute unrolled loop lengths
	AND $3, X7, X10
	SRL $2, X7
loop1:
	BEQZ X10, loop1done
loop1cont:
	// unroll 1X
	MOV 0(X8), X11
	// synthetic carry, one column at a time
	MUL X5, X11, X12
	MULHU X5, X11, X13
	ADD X6, X12, X11	// ADDS X6, X12, X11 (cr=X28)
	SLTU X6, X11, X28	// ...
	ADD X28, X13, X6	// ADC $0, X13, X6
	MOV X11, 0(X9)
	ADD $8, X8
	ADD $8, X9
	SUB $1, X10
	BNEZ X10, loop1cont
loop1done:
loop4:
	BEQZ X7, loop4done
loop4cont:
	// unroll 4X
	MOV 0(X8), X10
	MOV 8(X8), X11
	MOV 16(X8), X12
	MOV 24(X8), X13
	// synthetic carry, one column at a time
	MUL X5, X10, X14
	MULHU X5, X10, X15
	ADD X6, X14, X10	// ADDS X6, X14, X10 (cr=X28)
	SLTU X6, X10, X28	// ...
	ADD X28, X15, X6	// ADC $0, X15, X6
	MUL X5, X11, X14
	MULHU X5, X11, X15
	ADD X6, X14, X11	// ADDS X6, X14, X11 (cr=X28)
	SLTU X6, X11, X28	// ...
	ADD X28, X15, X6	// ADC $0, X15, X6
	MUL X5, X12, X14
	MULHU X5, X12, X15
	ADD X6, X14, X12	// ADDS X6, X14, X12 (cr=X28)
	SLTU X6, X12, X28	// ...
	ADD X28, X15, X6	// ADC $0, X15, X6
	MUL X5, X13, X14
	MULHU X5, X13, X15
	ADD X6, X14, X13	// ADDS X6, X14, X13 (cr=X28)
	SLTU X6, X13, X28	// ...
	ADD X28, X15, X6	// ADC $0, X15, X6
	MOV X10, 0(X9)
	MOV X11, 8(X9)
	MOV X12, 16(X9)
	MOV X13, 24(X9)
	ADD $32, X8
	ADD $32, X9
	SUB $1, X7
	BNEZ X7, loop4cont
loop4done:
	MOV X6, c+64(FP)
	RET

// func addMulVVWW(z, x, y []Word, m, a Word) (c Word)
TEXT ·addMulVVWW(SB), NOSPLIT, $0
	MOV m+72(FP), X5
	MOV a+80(FP), X6
	MOV z_len+8(FP), X7
	MOV x_base+24(FP), X8
	MOV y_base+48(FP), X9
	MOV z_base+0(FP), X10
	// compute unrolled loop lengths
	AND $3, X7, X11
	SRL $2, X7
loop1:
	BEQZ X11, loop1done
loop1cont:
	// unroll 1X
	MOV 0(X8), X12
	MOV 0(X9), X13
	// synthetic carry, one column at a time
	MUL X5, X13, X14
	MULHU X5, X13, X15
	ADD X12, X14	// ADDS X12, X14, X14 (cr=X28)
	SLTU X12, X14, X28	// ...
	ADD X28, X15	// ADC $0, X15, X15
	ADD X6, X14, X13	// ADDS X6, X14, X13 (cr=X28)
	SLTU X6, X13, X28	// ...
	ADD X28, X15, X6	// ADC $0, X15, X6
	MOV X13, 0(X10)
	ADD $8, X8
	ADD $8, X9
	ADD $8, X10
	SUB $1, X11
	BNEZ X11, loop1cont
loop1done:
loop4:
	BEQZ X7, loop4done
loop4cont:
	// unroll 4X
	MOV 0(X8), X11
	MOV 8(X8), X12
	MOV 16(X8), X13
	MOV 24(X8), X14
	MOV 0(X9), X15
	MOV 8(X9), X16
	MOV 16(X9), X17
	MOV 24(X9), X18
	// synthetic carry, one column at a time
	MUL X5, X15, X19
	MULHU X5, X15, X20
	ADD X11, X19	// ADDS X11, X19, X19 (cr=X28)
	SLTU X11, X19, X28	// ...
	ADD X28, X20	// ADC $0, X20, X20
	ADD X6, X19, X15	// ADDS X6, X19, X15 (cr=X28)
	SLTU X6, X15, X28	// ...
	ADD X28, X20, X6	// ADC $0, X20, X6
	MUL X5, X16, X19
	MULHU X5, X16, X20
	ADD X12, X19	// ADDS X12, X19, X19 (cr=X28)
	SLTU X12, X19, X28	// ...
	ADD X28, X20	// ADC $0, X20, X20
	ADD X6, X19, X16	// ADDS X6, X19, X16 (cr=X28)
	SLTU X6, X16, X28	// ...
	ADD X28, X20, X6	// ADC $0, X20, X6
	MUL X5, X17, X19
	MULHU X5, X17, X20
	ADD X13, X19	// ADDS X13, X19, X19 (cr=X28)
	SLTU X13, X19, X28	// ...
	ADD X28, X20	// ADC $0, X20, X20
	ADD X6, X19, X17	// ADDS X6, X19, X17 (cr=X28)
	SLTU X6, X17, X28	// ...
	ADD X28, X20, X6	// ADC $0, X20, X6
	MUL X5, X18, X19
	MULHU X5, X18, X20
	ADD X14, X19	// ADDS X14, X19, X19 (cr=X28)
	SLTU X14, X19, X28	// ...
	ADD X28, X20	// ADC $0, X20, X20
	ADD X6, X19, X18	// ADDS X6, X19, X18 (cr=X28)
	SLTU X6, X18, X28	// ...
	ADD X28, X20, X6	// ADC $0, X20, X6
	MOV X15, 0(X10)
	MOV X16, 8(X10)
	MOV X17, 16(X10)
	MOV X18, 24(X10)
	ADD $32, X8
	ADD $32, X9
	ADD $32, X10
	SUB $1, X7
	BNEZ X7, loop4cont
loop4done:
	MOV X6, c+88(FP)
	RET

```

// === FILE: references!/go/src/math/big/arith_s390x.s ===
```text
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by 'go generate' (with ./internal/asmgen). DO NOT EDIT.

//go:build !math_big_pure_go

#include "textflag.h"

// func addVV(z, x, y []Word) (c Word)
TEXT ·addVV(SB), NOSPLIT, $0
	MOVB ·hasVX(SB), R1
	CMPBEQ R1, $0, novec
	JMP ·addVVvec(SB)
novec:
	MOVD $0, R0
	MOVD z_len+8(FP), R1
	MOVD x_base+24(FP), R2
	MOVD y_base+48(FP), R3
	MOVD z_base+0(FP), R4
	// compute unrolled loop lengths
	MOVD R1, R5
	AND $3, R5
	SRD $2, R1
	ADDC R0, R1	// clear carry
loop1:
	CMPBEQ R5, $0, loop1done
loop1cont:
	// unroll 1X
	MOVD 0(R2), R6
	MOVD 0(R3), R7
	ADDE R7, R6
	MOVD R6, 0(R4)
	LAY 8(R2), R2	// ADD $8, R2
	LAY 8(R3), R3	// ADD $8, R3
	LAY 8(R4), R4	// ADD $8, R4
	LAY -1(R5), R5	// ADD $-1, R5
	CMPBNE R5, $0, loop1cont
loop1done:
loop4:
	CMPBEQ R1, $0, loop4done
loop4cont:
	// unroll 4X in batches of 2
	MOVD 0(R2), R5
	MOVD 8(R2), R6
	MOVD 0(R3), R7
	MOVD 8(R3), R8
	ADDE R7, R5
	ADDE R8, R6
	MOVD R5, 0(R4)
	MOVD R6, 8(R4)
	MOVD 16(R2), R5
	MOVD 24(R2), R6
	MOVD 16(R3), R7
	MOVD 24(R3), R8
	ADDE R7, R5
	ADDE R8, R6
	MOVD R5, 16(R4)
	MOVD R6, 24(R4)
	LAY 32(R2), R2	// ADD $32, R2
	LAY 32(R3), R3	// ADD $32, R3
	LAY 32(R4), R4	// ADD $32, R4
	LAY -1(R1), R1	// ADD $-1, R1
	CMPBNE R1, $0, loop4cont
loop4done:
	ADDE R0, R0, R2	// save & convert add carry
	MOVD R2, c+72(FP)
	RET

// func subVV(z, x, y []Word) (c Word)
TEXT ·subVV(SB), NOSPLIT, $0
	MOVB ·hasVX(SB), R1
	CMPBEQ R1, $0, novec
	JMP ·subVVvec(SB)
novec:
	MOVD $0, R0
	MOVD z_len+8(FP), R1
	MOVD x_base+24(FP), R2
	MOVD y_base+48(FP), R3
	MOVD z_base+0(FP), R4
	// compute unrolled loop lengths
	MOVD R1, R5
	AND $3, R5
	SRD $2, R1
	SUBC R0, R1	// clear carry
loop1:
	CMPBEQ R5, $0, loop1done
loop1cont:
	// unroll 1X
	MOVD 0(R2), R6
	MOVD 0(R3), R7
	SUBE R7, R6
	MOVD R6, 0(R4)
	LAY 8(R2), R2	// ADD $8, R2
	LAY 8(R3), R3	// ADD $8, R3
	LAY 8(R4), R4	// ADD $8, R4
	LAY -1(R5), R5	// ADD $-1, R5
	CMPBNE R5, $0, loop1cont
loop1done:
loop4:
	CMPBEQ R1, $0, loop4done
loop4cont:
	// unroll 4X in batches of 2
	MOVD 0(R2), R5
	MOVD 8(R2), R6
	MOVD 0(R3), R7
	MOVD 8(R3), R8
	SUBE R7, R5
	SUBE R8, R6
	MOVD R5, 0(R4)
	MOVD R6, 8(R4)
	MOVD 16(R2), R5
	MOVD 24(R2), R6
	MOVD 16(R3), R7
	MOVD 24(R3), R8
	SUBE R7, R5
	SUBE R8, R6
	MOVD R5, 16(R4)
	MOVD R6, 24(R4)
	LAY 32(R2), R2	// ADD $32, R2
	LAY 32(R3), R3	// ADD $32, R3
	LAY 32(R4), R4	// ADD $32, R4
	LAY -1(R1), R1	// ADD $-1, R1
	CMPBNE R1, $0, loop4cont
loop4done:
	SUBE R2, R2	// save carry
	NEG R2	// convert sub carry
	MOVD R2, c+72(FP)
	RET

// func lshVU(z, x []Word, s uint) (c Word)
TEXT ·lshVU(SB), NOSPLIT, $0
	MOVD $0, R0
	MOVD z_len+8(FP), R1
	CMPBEQ R1, $0, ret0
	MOVD s+48(FP), R2
	MOVD x_base+24(FP), R3
	MOVD z_base+0(FP), R4
	// run loop backward
	SLD $3, R1, R5
	LAY (R5)(R3), R3	// ADD R5, R3
	SLD $3, R1, R5
	LAY (R5)(R4), R4	// ADD R5, R4
	// shift first word into carry
	MOVD -8(R3), R5
	MOVD $64, R6
	SUBC R2, R6
	SRD R6, R5, R7
	SLD R2, R5
	MOVD R7, c+56(FP)
	// shift remaining words
	SUBC $1, R1
	// compute unrolled loop lengths
	MOVD R1, R7
	AND $3, R7
	SRD $2, R1
loop1:
	CMPBEQ R7, $0, loop1done
loop1cont:
	// unroll 1X
	MOVD -16(R3), R8
	SRD R6, R8, R9
	OR R5, R9
	SLD R2, R8, R5
	MOVD R9, -8(R4)
	LAY -8(R3), R3	// ADD $-8, R3
	LAY -8(R4), R4	// ADD $-8, R4
	LAY -1(R7), R7	// ADD $-1, R7
	CMPBNE R7, $0, loop1cont
loop1done:
loop4:
	CMPBEQ R1, $0, loop4done
loop4cont:
	// unroll 4X in batches of 2
	MOVD -16(R3), R7
	MOVD -24(R3), R8
	SRD R6, R7, R9
	OR R5, R9
	SLD R2, R7, R5
	SRD R6, R8, R7
	OR R5, R7
	SLD R2, R8, R5
	MOVD R9, -8(R4)
	MOVD R7, -16(R4)
	MOVD -32(R3), R7
	MOVD -40(R3), R8
	SRD R6, R7, R9
	OR R5, R9
	SLD R2, R7, R5
	SRD R6, R8, R7
	OR R5, R7
	SLD R2, R8, R5
	MOVD R9, -24(R4)
	MOVD R7, -32(R4)
	LAY -32(R3), R3	// ADD $-32, R3
	LAY -32(R4), R4	// ADD $-32, R4
	LAY -1(R1), R1	// ADD $-1, R1
	CMPBNE R1, $0, loop4cont
loop4done:
	// store final shifted bits
	MOVD R5, -8(R4)
	RET
ret0:
	MOVD R0, c+56(FP)
	RET

// func rshVU(z, x []Word, s uint) (c Word)
TEXT ·rshVU(SB), NOSPLIT, $0
	MOVD $0, R0
	MOVD z_len+8(FP), R1
	CMPBEQ R1, $0, ret0
	MOVD s+48(FP), R2
	MOVD x_base+24(FP), R3
	MOVD z_base+0(FP), R4
	// shift first word into carry
	MOVD 0(R3), R5
	MOVD $64, R6
	SUBC R2, R6
	SLD R6, R5, R7
	SRD R2, R5
	MOVD R7, c+56(FP)
	// shift remaining words
	SUBC $1, R1
	// compute unrolled loop lengths
	MOVD R1, R7
	AND $3, R7
	SRD $2, R1
loop1:
	CMPBEQ R7, $0, loop1done
loop1cont:
	// unroll 1X
	MOVD 8(R3), R8
	SLD R6, R8, R9
	OR R5, R9
	SRD R2, R8, R5
	MOVD R9, 0(R4)
	LAY 8(R3), R3	// ADD $8, R3
	LAY 8(R4), R4	// ADD $8, R4
	LAY -1(R7), R7	// ADD $-1, R7
	CMPBNE R7, $0, loop1cont
loop1done:
loop4:
	CMPBEQ R1, $0, loop4done
loop4cont:
	// unroll 4X in batches of 2
	MOVD 8(R3), R7
	MOVD 16(R3), R8
	SLD R6, R7, R9
	OR R5, R9
	SRD R2, R7, R5
	SLD R6, R8, R7
	OR R5, R7
	SRD R2, R8, R5
	MOVD R9, 0(R4)
	MOVD R7, 8(R4)
	MOVD 24(R3), R7
	MOVD 32(R3), R8
	SLD R6, R7, R9
	OR R5, R9
	SRD R2, R7, R5
	SLD R6, R8, R7
	OR R5, R7
	SRD R2, R8, R5
	MOVD R9, 16(R4)
	MOVD R7, 24(R4)
	LAY 32(R3), R3	// ADD $32, R3
	LAY 32(R4), R4	// ADD $32, R4
	LAY -1(R1), R1	// ADD $-1, R1
	CMPBNE R1, $0, loop4cont
loop4done:
	// store final shifted bits
	MOVD R5, 0(R4)
	RET
ret0:
	MOVD R0, c+56(FP)
	RET

// func mulAddVWW(z, x []Word, m, a Word) (c Word)
TEXT ·mulAddVWW(SB), NOSPLIT, $0
	MOVD $0, R0
	MOVD m+48(FP), R1
	MOVD a+56(FP), R2
	MOVD z_len+8(FP), R3
	MOVD x_base+24(FP), R4
	MOVD z_base+0(FP), R5
	// compute unrolled loop lengths
	MOVD R3, R6
	AND $3, R6
	SRD $2, R3
loop1:
	CMPBEQ R6, $0, loop1done
loop1cont:
	// unroll 1X in batches of 1
	MOVD 0(R4), R11
	// multiply
	MLGR R1, R10
	ADDC R2, R11
	ADDE R0, R10, R2
	MOVD R11, 0(R5)
	LAY 8(R4), R4	// ADD $8, R4
	LAY 8(R5), R5	// ADD $8, R5
	LAY -1(R6), R6	// ADD $-1, R6
	CMPBNE R6, $0, loop1cont
loop1done:
loop4:
	CMPBEQ R3, $0, loop4done
loop4cont:
	// unroll 4X in batches of 1
	MOVD 0(R4), R11
	// multiply
	MLGR R1, R10
	ADDC R2, R11
	ADDE R0, R10, R2
	MOVD R11, 0(R5)
	MOVD 8(R4), R11
	// multiply
	MLGR R1, R10
	ADDC R2, R11
	ADDE R0, R10, R2
	MOVD R11, 8(R5)
	MOVD 16(R4), R11
	// multiply
	MLGR R1, R10
	ADDC R2, R11
	ADDE R0, R10, R2
	MOVD R11, 16(R5)
	MOVD 24(R4), R11
	// multiply
	MLGR R1, R10
	ADDC R2, R11
	ADDE R0, R10, R2
	MOVD R11, 24(R5)
	LAY 32(R4), R4	// ADD $32, R4
	LAY 32(R5), R5	// ADD $32, R5
	LAY -1(R3), R3	// ADD $-1, R3
	CMPBNE R3, $0, loop4cont
loop4done:
	MOVD R2, c+64(FP)
	RET

// func addMulVVWW(z, x, y []Word, m, a Word) (c Word)
TEXT ·addMulVVWW(SB), NOSPLIT, $0
	MOVD $0, R0
	MOVD m+72(FP), R1
	MOVD a+80(FP), R2
	MOVD z_len+8(FP), R3
	MOVD x_base+24(FP), R4
	MOVD y_base+48(FP), R5
	MOVD z_base+0(FP), R6
	// compute unrolled loop lengths
	MOVD R3, R7
	AND $3, R7
	SRD $2, R3
loop1:
	CMPBEQ R7, $0, loop1done
loop1cont:
	// unroll 1X in batches of 1
	MOVD 0(R4), R8
	MOVD 0(R5), R11
	// multiply
	MLGR R1, R10
	ADDC R2, R11
	ADDE R0, R10, R2
	// add
	ADDC R8, R11
	ADDE R0, R2
	MOVD R11, 0(R6)
	LAY 8(R4), R4	// ADD $8, R4
	LAY 8(R5), R5	// ADD $8, R5
	LAY 8(R6), R6	// ADD $8, R6
	LAY -1(R7), R7	// ADD $-1, R7
	CMPBNE R7, $0, loop1cont
loop1done:
loop4:
	CMPBEQ R3, $0, loop4done
loop4cont:
	// unroll 4X in batches of 1
	MOVD 0(R4), R7
	MOVD 0(R5), R11
	// multiply
	MLGR R1, R10
	ADDC R2, R11
	ADDE R0, R10, R2
	// add
	ADDC R7, R11
	ADDE R0, R2
	MOVD R11, 0(R6)
	MOVD 8(R4), R7
	MOVD 8(R5), R11
	// multiply
	MLGR R1, R10
	ADDC R2, R11
	ADDE R0, R10, R2
	// add
	ADDC R7, R11
	ADDE R0, R2
	MOVD R11, 8(R6)
	MOVD 16(R4), R7
	MOVD 16(R5), R11
	// multiply
	MLGR R1, R10
	ADDC R2, R11
	ADDE R0, R10, R2
	// add
	ADDC R7, R11
	ADDE R0, R2
	MOVD R11, 16(R6)
	MOVD 24(R4), R7
	MOVD 24(R5), R11
	// multiply
	MLGR R1, R10
	ADDC R2, R11
	ADDE R0, R10, R2
	// add
	ADDC R7, R11
	ADDE R0, R2
	MOVD R11, 24(R6)
	LAY 32(R4), R4	// ADD $32, R4
	LAY 32(R5), R5	// ADD $32, R5
	LAY 32(R6), R6	// ADD $32, R6
	LAY -1(R3), R3	// ADD $-1, R3
	CMPBNE R3, $0, loop4cont
loop4done:
	MOVD R2, c+88(FP)
	RET

```

// === FILE: references!/go/src/math/big/arith_wasm.s ===
```text
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !math_big_pure_go

#include "textflag.h"

TEXT ·addVV(SB),NOSPLIT,$0
	JMP ·addVV_g(SB)

TEXT ·subVV(SB),NOSPLIT,$0
	JMP ·subVV_g(SB)

TEXT ·lshVU(SB),NOSPLIT,$0
	JMP ·lshVU_g(SB)

TEXT ·rshVU(SB),NOSPLIT,$0
	JMP ·rshVU_g(SB)

TEXT ·mulAddVWW(SB),NOSPLIT,$0
	JMP ·mulAddVWW_g(SB)

TEXT ·addMulVVWW(SB),NOSPLIT,$0
	JMP ·addMulVVWW_g(SB)


```

// === FILE: references!/go/src/math/big/arithvec_s390x.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !math_big_pure_go

package big

import "internal/cpu"

var hasVX = cpu.S390X.HasVX

func addVVvec(z, x, y []Word) (c Word)
func subVVvec(z, x, y []Word) (c Word)

```

// === FILE: references!/go/src/math/big/arithvec_s390x.s ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !math_big_pure_go

#include "textflag.h"

TEXT ·addVVvec(SB), NOSPLIT, $0
	MOVD z_len+8(FP), R3
	MOVD x+24(FP), R8
	MOVD y+48(FP), R9
	MOVD z+0(FP), R2

	MOVD $0, R4  // c = 0
	MOVD $0, R0  // make sure it's zero
	MOVD $0, R10 // i = 0

	// s/JL/JMP/ below to disable the unrolled loop
	SUB $4, R3
	BLT v1
	SUB $12, R3 // n -= 16
	BLT A1      // if n < 0 goto A1

	MOVD R8, R5
	MOVD R9, R6
	MOVD R2, R7

	// n >= 0
	// regular loop body unrolled 16x
	VZERO V0 // c = 0

UU1:
	VLM  0(R5), V1, V4    // 64-bytes into V1..V8
	ADD  $64, R5
	VPDI $0x4, V1, V1, V1 // flip the doublewords to big-endian order
	VPDI $0x4, V2, V2, V2 // flip the doublewords to big-endian order

	VLM  0(R6), V9, V12      // 64-bytes into V9..V16
	ADD  $64, R6
	VPDI $0x4, V9, V9, V9    // flip the doublewords to big-endian order
	VPDI $0x4, V10, V10, V10 // flip the doublewords to big-endian order

	VACCCQ V1, V9, V0, V25
	VACQ   V1, V9, V0, V17
	VACCCQ V2, V10, V25, V26
	VACQ   V2, V10, V25, V18

	VLM 0(R5), V5, V6   // 32-bytes into V1..V8
	VLM 0(R6), V13, V14 // 32-bytes into V9..V16
	ADD $32, R5
	ADD $32, R6

	VPDI $0x4, V3, V3, V3    // flip the doublewords to big-endian order
	VPDI $0x4, V4, V4, V4    // flip the doublewords to big-endian order
	VPDI $0x4, V11, V11, V11 // flip the doublewords to big-endian order
	VPDI $0x4, V12, V12, V12 // flip the doublewords to big-endian order

	VACCCQ V3, V11, V26, V27
	VACQ   V3, V11, V26, V19
	VACCCQ V4, V12, V27, V28
	VACQ   V4, V12, V27, V20

	VLM 0(R5), V7, V8   // 32-bytes into V1..V8
	VLM 0(R6), V15, V16 // 32-bytes into V9..V16
	ADD $32, R5
	ADD $32, R6

	VPDI $0x4, V5, V5, V5    // flip the doublewords to big-endian order
	VPDI $0x4, V6, V6, V6    // flip the doublewords to big-endian order
	VPDI $0x4, V13, V13, V13 // flip the doublewords to big-endian order
	VPDI $0x4, V14, V14, V14 // flip the doublewords to big-endian order

	VACCCQ V5, V13, V28, V29
	VACQ   V5, V13, V28, V21
	VACCCQ V6, V14, V29, V30
	VACQ   V6, V14, V29, V22

	VPDI $0x4, V7, V7, V7    // flip the doublewords to big-endian order
	VPDI $0x4, V8, V8, V8    // flip the doublewords to big-endian order
	VPDI $0x4, V15, V15, V15 // flip the doublewords to big-endian order
	VPDI $0x4, V16, V16, V16 // flip the doublewords to big-endian order

	VACCCQ V7, V15, V30, V31
	VACQ   V7, V15, V30, V23
	VACCCQ V8, V16, V31, V0  // V0 has carry-over
	VACQ   V8, V16, V31, V24

	VPDI  $0x4, V17, V17, V17 // flip the doublewords to big-endian order
	VPDI  $0x4, V18, V18, V18 // flip the doublewords to big-endian order
	VPDI  $0x4, V19, V19, V19 // flip the doublewords to big-endian order
	VPDI  $0x4, V20, V20, V20 // flip the doublewords to big-endian order
	VPDI  $0x4, V21, V21, V21 // flip the doublewords to big-endian order
	VPDI  $0x4, V22, V22, V22 // flip the doublewords to big-endian order
	VPDI  $0x4, V23, V23, V23 // flip the doublewords to big-endian order
	VPDI  $0x4, V24, V24, V24 // flip the doublewords to big-endian order
	VSTM  V17, V24, 0(R7)     // 128-bytes into z
	ADD   $128, R7
	ADD   $128, R10           // i += 16
	SUB   $16, R3             // n -= 16
	BGE   UU1                 // if n >= 0 goto U1
	VLGVG $1, V0, R4          // put cf into R4
	NEG   R4, R4              // save cf

A1:
	ADD $12, R3 // n += 16

	// s/JL/JMP/ below to disable the unrolled loop
	BLT v1 // if n < 0 goto v1

U1:  // n >= 0
	// regular loop body unrolled 4x
	MOVD 0(R8)(R10*1), R5
	MOVD 8(R8)(R10*1), R6
	MOVD 16(R8)(R10*1), R7
	MOVD 24(R8)(R10*1), R1
	ADDC R4, R4             // restore CF
	MOVD 0(R9)(R10*1), R11
	ADDE R11, R5
	MOVD 8(R9)(R10*1), R11
	ADDE R11, R6
	MOVD 16(R9)(R10*1), R11
	ADDE R11, R7
	MOVD 24(R9)(R10*1), R11
	ADDE R11, R1
	MOVD R0, R4
	ADDE R4, R4             // save CF
	NEG  R4, R4
	MOVD R5, 0(R2)(R10*1)
	MOVD R6, 8(R2)(R10*1)
	MOVD R7, 16(R2)(R10*1)
	MOVD R1, 24(R2)(R10*1)

	ADD $32, R10 // i += 4
	SUB $4, R3   // n -= 4
	BGE U1       // if n >= 0 goto U1

v1:
	ADD $4, R3 // n += 4
	BLE E1     // if n <= 0 goto E1

L1:  // n > 0
	ADDC R4, R4            // restore CF
	MOVD 0(R8)(R10*1), R5
	MOVD 0(R9)(R10*1), R11
	ADDE R11, R5
	MOVD R5, 0(R2)(R10*1)
	MOVD R0, R4
	ADDE R4, R4            // save CF
	NEG  R4, R4

	ADD $8, R10 // i++
	SUB $1, R3  // n--
	BGT L1      // if n > 0 goto L1

E1:
	NEG  R4, R4
	MOVD R4, c+72(FP) // return c
	RET

TEXT ·subVVvec(SB), NOSPLIT, $0
	MOVD z_len+8(FP), R3
	MOVD x+24(FP), R8
	MOVD y+48(FP), R9
	MOVD z+0(FP), R2
	MOVD $0, R4          // c = 0
	MOVD $0, R0          // make sure it's zero
	MOVD $0, R10         // i = 0

	// s/JL/JMP/ below to disable the unrolled loop
	SUB $4, R3  // n -= 4
	BLT v1      // if n < 0 goto v1
	SUB $12, R3 // n -= 16
	BLT A1      // if n < 0 goto A1

	MOVD R8, R5
	MOVD R9, R6
	MOVD R2, R7

	// n >= 0
	// regular loop body unrolled 16x
	VZERO V0         // cf = 0
	MOVD  $1, R4     // for 390 subtraction cf starts as 1 (no borrow)
	VLVGG $1, R4, V0 // put carry into V0

UU1:
	VLM  0(R5), V1, V4    // 64-bytes into V1..V8
	ADD  $64, R5
	VPDI $0x4, V1, V1, V1 // flip the doublewords to big-endian order
	VPDI $0x4, V2, V2, V2 // flip the doublewords to big-endian order

	VLM  0(R6), V9, V12      // 64-bytes into V9..V16
	ADD  $64, R6
	VPDI $0x4, V9, V9, V9    // flip the doublewords to big-endian order
	VPDI $0x4, V10, V10, V10 // flip the doublewords to big-endian order

	VSBCBIQ V1, V9, V0, V25
	VSBIQ   V1, V9, V0, V17
	VSBCBIQ V2, V10, V25, V26
	VSBIQ   V2, V10, V25, V18

	VLM 0(R5), V5, V6   // 32-bytes into V1..V8
	VLM 0(R6), V13, V14 // 32-bytes into V9..V16
	ADD $32, R5
	ADD $32, R6

	VPDI $0x4, V3, V3, V3    // flip the doublewords to big-endian order
	VPDI $0x4, V4, V4, V4    // flip the doublewords to big-endian order
	VPDI $0x4, V11, V11, V11 // flip the doublewords to big-endian order
	VPDI $0x4, V12, V12, V12 // flip the doublewords to big-endian order

	VSBCBIQ V3, V11, V26, V27
	VSBIQ   V3, V11, V26, V19
	VSBCBIQ V4, V12, V27, V28
	VSBIQ   V4, V12, V27, V20

	VLM 0(R5), V7, V8   // 32-bytes into V1..V8
	VLM 0(R6), V15, V16 // 32-bytes into V9..V16
	ADD $32, R5
	ADD $32, R6

	VPDI $0x4, V5, V5, V5    // flip the doublewords to big-endian order
	VPDI $0x4, V6, V6, V6    // flip the doublewords to big-endian order
	VPDI $0x4, V13, V13, V13 // flip the doublewords to big-endian order
	VPDI $0x4, V14, V14, V14 // flip the doublewords to big-endian order

	VSBCBIQ V5, V13, V28, V29
	VSBIQ   V5, V13, V28, V21
	VSBCBIQ V6, V14, V29, V30
	VSBIQ   V6, V14, V29, V22

	VPDI $0x4, V7, V7, V7    // flip the doublewords to big-endian order
	VPDI $0x4, V8, V8, V8    // flip the doublewords to big-endian order
	VPDI $0x4, V15, V15, V15 // flip the doublewords to big-endian order
	VPDI $0x4, V16, V16, V16 // flip the doublewords to big-endian order

	VSBCBIQ V7, V15, V30, V31
	VSBIQ   V7, V15, V30, V23
	VSBCBIQ V8, V16, V31, V0  // V0 has carry-over
	VSBIQ   V8, V16, V31, V24

	VPDI  $0x4, V17, V17, V17 // flip the doublewords to big-endian order
	VPDI  $0x4, V18, V18, V18 // flip the doublewords to big-endian order
	VPDI  $0x4, V19, V19, V19 // flip the doublewords to big-endian order
	VPDI  $0x4, V20, V20, V20 // flip the doublewords to big-endian order
	VPDI  $0x4, V21, V21, V21 // flip the doublewords to big-endian order
	VPDI  $0x4, V22, V22, V22 // flip the doublewords to big-endian order
	VPDI  $0x4, V23, V23, V23 // flip the doublewords to big-endian order
	VPDI  $0x4, V24, V24, V24 // flip the doublewords to big-endian order
	VSTM  V17, V24, 0(R7)     // 128-bytes into z
	ADD   $128, R7
	ADD   $128, R10           // i += 16
	SUB   $16, R3             // n -= 16
	BGE   UU1                 // if n >= 0 goto U1
	VLGVG $1, V0, R4          // put cf into R4
	SUB   $1, R4              // save cf

A1:
	ADD $12, R3 // n += 16
	BLT v1      // if n < 0 goto v1

U1:  // n >= 0
	// regular loop body unrolled 4x
	MOVD 0(R8)(R10*1), R5
	MOVD 8(R8)(R10*1), R6
	MOVD 16(R8)(R10*1), R7
	MOVD 24(R8)(R10*1), R1
	MOVD R0, R11
	SUBC R4, R11            // restore CF
	MOVD 0(R9)(R10*1), R11
	SUBE R11, R5
	MOVD 8(R9)(R10*1), R11
	SUBE R11, R6
	MOVD 16(R9)(R10*1), R11
	SUBE R11, R7
	MOVD 24(R9)(R10*1), R11
	SUBE R11, R1
	MOVD R0, R4
	SUBE R4, R4             // save CF
	MOVD R5, 0(R2)(R10*1)
	MOVD R6, 8(R2)(R10*1)
	MOVD R7, 16(R2)(R10*1)
	MOVD R1, 24(R2)(R10*1)

	ADD $32, R10 // i += 4
	SUB $4, R3   // n -= 4
	BGE U1       // if n >= 0 goto U1n

v1:
	ADD $4, R3 // n += 4
	BLE E1     // if n <= 0 goto E1

L1:  // n > 0
	MOVD R0, R11
	SUBC R4, R11           // restore CF
	MOVD 0(R8)(R10*1), R5
	MOVD 0(R9)(R10*1), R11
	SUBE R11, R5
	MOVD R5, 0(R2)(R10*1)
	MOVD R0, R4
	SUBE R4, R4            // save CF

	ADD $8, R10 // i++
	SUB $1, R3  // n--
	BGT L1      // if n > 0 goto L1n

E1:
	NEG  R4, R4
	MOVD R4, c+72(FP) // return c
	RET

```

// === FILE: references!/go/src/math/big/calibrate.md ===
```markdown
# Calibration of Algorithm Thresholds

This document describes the approach to calibration of algorithmic thresholds in
`math/big`, implemented in [calibrate_test.go](calibrate_test.go).

Basic operations like multiplication and division have many possible implementations.
Most algorithms that are better asymptotically have overheads that make them
run slower for small inputs. When presented with an operation to run, `math/big`
must decide which algorithm to use.

For example, for small inputs, multiplication using the “grade school algorithm” is fastest.
Given multi-digit x, y and a target z: clear z, and then for each digit y[i], z[i:] += x\*y[i].
That last operation, adding a vector times a digit to another vector (including carrying up
the vector during the multiplication and addition), can be implemented in a tight assembly loop.
The overall speed is O(N\*\*2) where N is the number of digits in x and y (assume they match),
but the tight inner loop performs well for small inputs.

[Karatsuba's algorithm](https://en.wikipedia.org/wiki/Karatsuba_algorithm)
multiplies two N-digit numbers by splitting them in half, computing
three N/2-digit products, and then reconstructing the final product using a few more
additions and subtractions. It runs in O(N\*\*log₂ 3) = O(N\*\*1.58) time.
The grade school loop runs faster for small inputs,
but eventually Karatsuba's smaller asymptotic run time wins.

The multiplication implementation must decide which to use.
Under the assumption that once Karatsuba is faster for some N,
it will be larger for all larger N as well,
the rule is to use Karatsuba's algorithm when the input length N ≥ karatsubaThreshold.

Calibration is the process of determining what karatsubaThreshold should be set to.
It doesn't sound like it should be that hard, but it is:
- Theoretical analysis does not help: the answer depends on the actual machines
and the actual constant factors in the two implementations.
- We are picking a single karatsubaThreshold for all systems,
despite them having different relative execution speeds for the operations
in the two algorithms.
(We could in theory pick different thresholds for different architectures,
but there can still be significant variation within a given architecture.)
- The assumption that there is a single N where
an asymptotically better algorithm becomes faster and stays faster
is not true in general.
- Recursive algorithms like Karatsuba's may have  different optimal
thresholds for different large input sizes.
- Thresholds can interfere. For example, changing the karatsubaThreshold makes
multiplication faster or slower, which in turn affects the best divRecursiveThreshold
(because divisions use multiplication).

The best we can do is measure the performance of the overall multiplication
algorithm across a variety of inputs and thresholds and look for a threshold
that balances all these concerns reasonably well,
setting thresholds in dependency order (for example, multiplication before division).

The code in `calibrate_test.go` does this measurement of a variety of input sizes
and threshold values and prints the timing results as a CSV file.
The code in `calibrate_graph.go` reads the CSV and writes out an SVG file plotting the data.
For example:

	go test -run=Calibrate/KaratsubaMul -timeout=1h -calibrate >kmul.csv
	go run calibrate_graph.go kmul.csv >kmul.svg

Any particular input is sensitive to only a few transitions in threshold.
For example, an input of size 320 recurses on inputs of size 160,
which recurses on inputs of size 80,
which recurses on inputs of size 40,
and so on, until falling below the Karatsuba threshold.
Here is what the timing looks like for an input of size 320,
normalized so that 1.0 is the fastest timing observed:

![KaratsubaThreshold on an Apple M3 Pro, N=320 only](https://swtch.com/math/big/_calibrate/KaratsubaMul/cal.mac320.svg)

For this input, all thresholds from 21 to 40 perform optimally and identically: they all mean “recurse at N=40 but not at N=20”.
From the single input of size N=320, we cannot decide which of these 20 thresholds is best.

Other inputs exercise other decision points. For example, here is the timing for N=240:

![KaratsubaThreshold on an Apple M3 Pro, N=240 only](https://swtch.com/math/big/_calibrate/KaratsubaMul/cal.mac240.svg)

In this case, all the thresholds from 31 to 60 perform optimally and identically, recursing at N=60 but not N=30.

If we combine these two into a single graph and then plot the geometric mean of the two lines in blue,
the optimal range becomes a little clearer:

![KaratsubaThreshold on an Apple M3 Pro](https://swtch.com/math/big/_calibrate/KaratsubaMul/cal.mac240+320.svg)

The actual calibration runs all possible inputs from size N=200 to N=400, in increments of 8,
plotting all 26 lines in a faded gray (note the changed y-axis scale, zooming in near 1.0).

![KaratsubaThreshold on an Apple M3 Pro](https://swtch.com/math/big/_calibrate/KaratsubaMul/cal.mac.svg)

Now the optimal value is clear: the best threshold on this chip, with these algorithmic implementations, is 40.

Unfortunately, other chips are different. Here is an Intel Xeon server chip:

![KaratsubaThreshold on an Apple M3 Pro](https://swtch.com/math/big/_calibrate/KaratsubaMul/cal.c2s16.svg)

On this chip, the best threshold is closer to 60. Luckily, 40 is not a terrible choice either: it is only about 2% slower on average.

The rest of this document presents the timings measured for the `math/big` thresholds on a variety of machines
and justifies the final thresholds. The timings used these machines:

- The `gotip-linux-amd64_c3h88-perf_vs_release` gomote, a Google Cloud c3-high-88 machine using an Intel Xeon Platinum 8481C CPU (Emerald Rapids).
- The `gotip-linux-amd64_c2s16-perf_vs_release` gomote, a Google Cloud c2-standard-16 machine using an Intel Xeon Gold 6253CL CPU (Cascade Lake).
- A home server built with an AMD Ryzen 9 7950X CPU.
- The `gotip-linux-arm64_c4as16-perf_vs_release` gomote, a Google Cloud c4a-standard-16 machine using Google's Axiom Arm CPU.
- An Apple MacBook Pro with an Apple M3 Pro CPU.

In general, we break ties in favor of the newer c3h88 x86 perf gomote, then the c4as16 arm64 perf gomote, and then the others.

## Karatsuba Multiplication

Here are the full results for the Karatsuba multiplication threshold.

![KaratsubaThreshold on an Intel Xeon Platium 8481C](https://swtch.com/math/big/_calibrate/KaratsubaMul/cal.c3h88.svg)
![KaratsubaThreshold on an Intel Xeon Gold 6253CL](https://swtch.com/math/big/_calibrate/KaratsubaMul/cal.c2s16.svg)
![KaratsubaThreshold on an AMD Ryzen 9 7950X](https://swtch.com/math/big/_calibrate/KaratsubaMul/cal.s7.svg)
![KaratsubaThreshold on an Axiom Arm](https://swtch.com/math/big/_calibrate/KaratsubaMul/cal.c4as16.svg)
![KaratsubaThreshold on an Apple M3 Pro](https://swtch.com/math/big/_calibrate/KaratsubaMul/cal.mac.svg)

The majority of systems have optimum thresholds near 40, so we chose karatsubaThreshold = 40.

## Basic Squaring

For squaring a number (`z.Mul(x, x)`), math/big uses grade school multiplication
up to basicSqrThreshold, where it switches to a customized algorithm that is
still quadratic but avoids half the word-by-word multiplies
since the two arguments are identical.
That algorithm's inner loops are not as tight as the grade school multiplication,
so it is slower for small inputs. How small?

Here are the timings:

![BasicSqrThreshold on an Intel Xeon Platium 8481C](https://swtch.com/math/big/_calibrate/BasicSqr/cal.c3h88.svg)
![BasicSqrThreshold on an Intel Xeon Gold 6253CL](https://swtch.com/math/big/_calibrate/BasicSqr/cal.c2s16.svg)
![BasicSqrThreshold on an AMD Ryzen 9 7950X](https://swtch.com/math/big/_calibrate/BasicSqr/cal.s7.svg)
![BasicSqrThreshold on an Axiom Arm](https://swtch.com/math/big/_calibrate/BasicSqr/cal.c4as16.svg)
![BasicSqrThreshold on an Apple M3 Pro](https://swtch.com/math/big/_calibrate/BasicSqr/cal.mac.svg)

These inputs are so small that the calibration times batches of 100 instead of individual operations.
There is no one best threshold, even on a single system, because some of the sizes seem to run
the grade school algorithm faster than others.
For example, on the AMD CPU,
for N=14, basic squaring is 4% faster than basic multiplication,
suggesting the threshold has been crossed,
but for N=16, basic multiplication is 9% faster than basic squaring,
probably because the tight assembly can use larger chunks.

It is unclear why the Axiom Arm timings are so incredibly noisy.

We chose basicSqrThreshold = 12.

## Karatsuba Squaring

Beyond the basic squaring threshold, at some point a customized Karatsuba can take over.
It uses three half-sized squarings instead of three half-sized multiplies.
Here are the timings:

![KaratsubaSqrThreshold on an Intel Xeon Platium 8481C](https://swtch.com/math/big/_calibrate/KaratsubaSqr/cal.c3h88.svg)
![KaratsubaSqrThreshold on an Intel Xeon Gold 6253CL](https://swtch.com/math/big/_calibrate/KaratsubaSqr/cal.c2s16.svg)
![KaratsubaSqrThreshold on an AMD Ryzen 9 7950X](https://swtch.com/math/big/_calibrate/KaratsubaSqr/cal.s7.svg)
![KaratsubaSqrThreshold on an Axiom Arm](https://swtch.com/math/big/_calibrate/KaratsubaSqr/cal.c4as16.svg)
![KaratsubaSqrThreshold on an Apple M3 Pro](https://swtch.com/math/big/_calibrate/KaratsubaSqr/cal.mac.svg)

The majority of chips preferred a lower threshold, around 60-70,
but the older Intel Xeon and the AMD prefer a threshold around 100-120.

We chose karatsubaSqrThreshold = 80, which is within 2% of optimal on all the chips.

## Recursive Division

Division uses a recursive divide-and-conquer algorithm for large inputs,
eventually falling back to a more traditional grade-school whole-input trial-and-error division.
Here are the timings for the threshold between the two:

![DivRecursiveThreshold on an Intel Xeon Platium 8481C](https://swtch.com/math/big/_calibrate/DivRecursive/cal.c3h88.svg)
![DivRecursiveThreshold on an Intel Xeon Gold 6253CL](https://swtch.com/math/big/_calibrate/DivRecursive/cal.c2s16.svg)
![DivRecursiveThreshold on an AMD Ryzen 9 7950X](https://swtch.com/math/big/_calibrate/DivRecursive/cal.s7.svg)
![DivRecursiveThreshold on an Axiom Arm](https://swtch.com/math/big/_calibrate/DivRecursive/cal.c4as16.svg)
![DivRecursiveThreshold on an Apple M3 Pro](https://swtch.com/math/big/_calibrate/DivRecursive/cal.mac.svg)

We chose divRecursiveThreshold = 40.

```

// === FILE: references!/go/src/math/big/calibrate_graph.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

// This program converts CSV calibration data printed by
//
//	go test -run=Calibrate/Name -calibrate >file.csv
//
// into an SVG file. Invoke as:
//
//	go run calibrate_graph.go file.csv >file.svg
//
// See calibrate.md for more details.

package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: go run calibrate_graph.go file.csv >file.svg\n")
	os.Exit(2)
}

// A Point is an X, Y coordinate in the data being plotted.
type Point struct {
	X, Y float64
}

// A Graph is a graph to draw as SVG.
type Graph struct {
	Title   string    // title above graph
	Geomean []Point   // geomean line
	Lines   [][]Point // normalized data lines
	XAxis   string    // x-axis label
	YAxis   string    // y-axis label
	Min     Point     // min point of data display
	Max     Point     // max point of data display
}

var yMax = flag.Float64("ymax", 1.2, "maximum y axis value")
var alphaNorm = flag.Float64("alphanorm", 0.1, "alpha for a single norm line")

func main() {
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() != 1 {
		usage()
	}

	// Read CSV. It may be enclosed in
	//	-- name.csv --
	//	...
	//	-- eof --
	// framing, in which case remove the framing.
	fdata, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	if _, after, ok := bytes.Cut(fdata, []byte(".csv --\n")); ok {
		fdata = after
	}
	if before, _, ok := bytes.Cut(fdata, []byte("-- eof --\n")); ok {
		fdata = before
	}
	rd := csv.NewReader(bytes.NewReader(fdata))
	rd.FieldsPerRecord = -1
	records, err := rd.ReadAll()
	if err != nil {
		log.Fatal(err)
	}

	// Construct graph from loaded CSV.
	// CSV starts with metadata lines like
	//	goos,darwin
	// and then has two tables of timings.
	// Each table looks like
	//	size \ threshold,10,20,30,40
	//	100,1,2,3,4
	//	200,2,3,4,5
	//	300,3,4,5,6
	//	400,4,5,6,7
	//	500,5,6,7,8
	// The header line gives the threshold values and then each row
	// gives an input size and the timings for each threshold.
	// Omitted timings are empty strings and turn into infinities when parsing.
	// The first table gives raw nanosecond timings.
	// The second table gives timings normalized relative to the fastest
	// possible threshold for a given input size.
	// We only want the second table.
	// The tables are followed by a list of geomeans of all the normalized
	// timings for each threshold:
	//	geomean,1.2,1.1,1.0,1.4
	// We turn each normalized timing row into a line in the graph,
	// and we turn the geomean into an overlaid thick line.
	// The metadata is used for preparing the titles.
	g := &Graph{
		YAxis: "Relative Slowdown",
		Min:   Point{0, 1},
		Max:   Point{1, 1.2},
	}
	meta := make(map[string]string)
	table := 0 // number of table headers seen
	var thresholds []float64
	maxNorm := 0.0
	for _, rec := range records {
		if len(rec) == 0 {
			continue
		}
		if len(rec) == 2 {
			meta[rec[0]] = rec[1]
			continue
		}
		if rec[0] == `size \ threshold` {
			table++
			if table == 2 {
				thresholds = parseFloats(rec)
				g.Min.X = thresholds[0]
				g.Max.X = thresholds[len(thresholds)-1]
			}
			continue
		}
		if rec[0] == "geomean" {
			table = 3 // end of norms table
			geomeans := parseFloats(rec)
			g.Geomean = floatsToLine(thresholds, geomeans)
			continue
		}
		if table == 2 {
			if _, err := strconv.Atoi(rec[0]); err != nil { // size
				log.Fatalf("invalid table line: %q", rec)
			}
			norms := parseFloats(rec)
			if len(norms) > len(thresholds) {
				log.Fatalf("too many timings (%d > %d): %q", len(norms), len(thresholds), rec)
			}
			g.Lines = append(g.Lines, floatsToLine(thresholds, norms))
			for _, y := range norms {
				maxNorm = max(maxNorm, y)
			}
			continue
		}
	}

	g.Max.Y = min(*yMax, math.Ceil(maxNorm*100)/100)
	g.XAxis = meta["calibrate"] + "Threshold"
	g.Title = meta["goos"] + "/" + meta["goarch"] + " " + meta["cpu"]

	os.Stdout.Write(g.SVG())
}

// parseFloats parses rec[1:] as floating point values.
// If a field is the empty string, it is represented as +Inf.
func parseFloats(rec []string) []float64 {
	floats := make([]float64, 0, len(rec)-1)
	for _, v := range rec[1:] {
		if v == "" {
			floats = append(floats, math.Inf(+1))
			continue
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			log.Fatalf("invalid record: %q (%v)", rec, err)
		}
		floats = append(floats, f)
	}
	return floats
}

// floatsToLine converts a sequence of floats into a line, ignoring missing (infinite) values.
func floatsToLine(x, y []float64) []Point {
	var line []Point
	for i, yi := range y {
		if !math.IsInf(yi, 0) {
			line = append(line, Point{x[i], yi})
		}
	}
	return line
}

const svgHeader = `<svg width="%d" height="%d" version="1.1" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <style type="text/css"><![CDATA[
      text { stroke-width: 0; white-space: pre; }
      text.hjc { text-anchor: middle; }
      text.hjl { text-anchor: start; }
      text.hjr { text-anchor: end; }
      .def { stroke-linecap: round; stroke-linejoin: round; fill: none; stroke: #000000; stroke-width: 1px; }
      .tick { stroke: #000000; fill: #000000; font: %dpx Times; }
      .title { stroke: #000000; fill: #000000; font: %dpx Times; font-weight: bold; }
      .axis { stroke-width: 2px; }
      .norm { stroke: rgba(0,0,0,%f); }
      .geomean { stroke: #6666ff; stroke-width: 2px; }
    ]]></style>
  </defs>
  <g class="def">
`

// Layout constants for drawing graph
const (
	DX   = 600          // width of graphed data
	DY   = 150          // height of graphed data
	ML   = 80           // margin left
	MT   = 30           // margin top
	MR   = 10           // margin right
	MB   = 50           // margin bottom
	PS   = 14           // point size of text
	W    = ML + DX + MR // width of overall graph
	H    = MT + DY + MB // height of overall graph
	Tick = 5            // axis tick length
)

// An SVGPoint is a point in the SVG image, in pixel units,
// with Y increasing down the page.
type SVGPoint struct {
	X, Y int
}

func (p SVGPoint) String() string {
	return fmt.Sprintf("%d,%d", p.X, p.Y)
}

// pt converts an x, y data value (such as from a Point) to an SVGPoint.
func (g *Graph) pt(x, y float64) SVGPoint {
	return SVGPoint{
		X: ML + int((x-g.Min.X)/(g.Max.X-g.Min.X)*DX),
		Y: H - MB - int((y-g.Min.Y)/(g.Max.Y-g.Min.Y)*DY),
	}
}

// SVG returns the SVG text for the graph.
func (g *Graph) SVG() []byte {

	var svg bytes.Buffer
	fmt.Fprintf(&svg, svgHeader, W, H, PS, PS, *alphaNorm)

	// Draw data, clipped.
	fmt.Fprintf(&svg, "<clipPath id=\"cp\"><path d=\"M %v L %v L %v L %v Z\" /></clipPath>\n",
		g.pt(g.Min.X, g.Min.Y), g.pt(g.Max.X, g.Min.Y), g.pt(g.Max.X, g.Max.Y), g.pt(g.Min.X, g.Max.Y))
	fmt.Fprintf(&svg, "<g clip-path=\"url(#cp)\">\n")
	for _, line := range g.Lines {
		if len(line) == 0 {
			continue
		}
		fmt.Fprintf(&svg, "<path class=\"norm\" d=\"M %v", g.pt(line[0].X, line[0].Y))
		for _, v := range line[1:] {
			fmt.Fprintf(&svg, " L %v", g.pt(v.X, v.Y))
		}
		fmt.Fprintf(&svg, "\"/>\n")
	}
	// Draw geomean.
	if len(g.Geomean) > 0 {
		line := g.Geomean
		fmt.Fprintf(&svg, "<path class=\"geomean\" d=\"M %v", g.pt(line[0].X, line[0].Y))
		for _, v := range line[1:] {
			fmt.Fprintf(&svg, " L %v", g.pt(v.X, v.Y))
		}
		fmt.Fprintf(&svg, "\"/>\n")
	}
	fmt.Fprintf(&svg, "</g>\n")

	// Draw axes and major and minor tick marks.
	fmt.Fprintf(&svg, "<path class=\"axis\" d=\"")
	fmt.Fprintf(&svg, " M %v L %v", g.pt(g.Min.X, g.Min.Y), g.pt(g.Max.X, g.Min.Y)) // x axis
	fmt.Fprintf(&svg, " M %v L %v", g.pt(g.Min.X, g.Min.Y), g.pt(g.Min.X, g.Max.Y)) // y axis
	xscale := 10.0
	if g.Max.X-g.Min.X < 100 {
		xscale = 1.0
	}
	for x := int(math.Ceil(g.Min.X / xscale)); float64(x)*xscale <= g.Max.X; x++ {
		if x%5 != 0 {
			fmt.Fprintf(&svg, " M %v l 0,%d", g.pt(float64(x)*xscale, g.Min.Y), Tick)
		} else {
			fmt.Fprintf(&svg, " M %v l 0,%d", g.pt(float64(x)*xscale, g.Min.Y), 2*Tick)
		}
	}
	yscale := 100.0
	if g.Max.Y-g.Min.Y > 0.5 {
		yscale = 10
	}
	for y := int(math.Ceil(g.Min.Y * yscale)); float64(y) <= g.Max.Y*yscale; y++ {
		if y%5 != 0 {
			fmt.Fprintf(&svg, " M %v l -%d,0", g.pt(g.Min.X, float64(y)/yscale), Tick)
		} else {
			fmt.Fprintf(&svg, " M %v l -%d,0", g.pt(g.Min.X, float64(y)/yscale), 2*Tick)
		}
	}
	fmt.Fprintf(&svg, "\"/>\n")

	// Draw tick labels on major marks.
	for x := int(math.Ceil(g.Min.X / xscale)); float64(x)*xscale <= g.Max.X; x++ {
		if x%5 == 0 {
			p := g.pt(float64(x)*xscale, g.Min.Y)
			fmt.Fprintf(&svg, "<text x=\"%d\" y=\"%d\" class=\"tick hjc\">%d</text>\n", p.X, p.Y+2*Tick+PS, x*int(xscale))
		}
	}
	for y := int(math.Ceil(g.Min.Y * yscale)); float64(y) <= g.Max.Y*yscale; y++ {
		if y%5 == 0 {
			p := g.pt(g.Min.X, float64(y)/yscale)
			fmt.Fprintf(&svg, "<text x=\"%d\" y=\"%d\" class=\"tick hjr\">%.2f</text>\n", p.X-2*Tick-Tick, p.Y+PS/3, float64(y)/yscale)
		}
	}

	// Draw graph title and axis titles.
	fmt.Fprintf(&svg, "<text x=\"%d\" y=\"%d\" class=\"title hjc\">%s</text>\n", ML+DX/2, MT-PS/3, g.Title)
	fmt.Fprintf(&svg, "<text x=\"%d\" y=\"%d\" class=\"title hjc\">%s</text>\n", ML+DX/2, MT+DY+2*Tick+2*PS+PS/2, g.XAxis)
	fmt.Fprintf(&svg, "<g transform=\"translate(%d,%d) rotate(-90)\"><text x=\"0\" y=\"0\" class=\"title hjc\">%s</text></g>\n", ML-Tick-Tick-3*PS, MT+DY/2, g.YAxis)

	fmt.Fprintf(&svg, "</g></svg>\n")
	return svg.Bytes()
}

```

// === FILE: references!/go/src/math/big/decimal.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements multi-precision decimal numbers.
// The implementation is for float to decimal conversion only;
// not general purpose use.
// The only operations are precise conversion from binary to
// decimal and rounding.
//
// The key observation and some code (shr) is borrowed from
// strconv/decimal.go: conversion of binary fractional values can be done
// precisely in multi-precision decimal because 2 divides 10 (required for
// >> of mantissa); but conversion of decimal floating-point values cannot
// be done precisely in binary representation.
//
// In contrast to strconv/decimal.go, only right shift is implemented in
// decimal format - left shift can be done precisely in binary format.

package big

// A decimal represents an unsigned floating-point number in decimal representation.
// The value of a non-zero decimal d is d.mant * 10**d.exp with 0.1 <= d.mant < 1,
// with the most-significant mantissa digit at index 0. For the zero decimal, the
// mantissa length and exponent are 0.
// The zero value for decimal represents a ready-to-use 0.0.
type decimal struct {
	mant []byte // mantissa ASCII digits, big-endian
	exp  int    // exponent
}

// at returns the i'th mantissa digit, starting with the most significant digit at 0.
func (d *decimal) at(i int) byte {
	if 0 <= i && i < len(d.mant) {
		return d.mant[i]
	}
	return '0'
}

// Maximum shift amount that can be done in one pass without overflow.
// A Word has _W bits and (1<<maxShift - 1)*10 + 9 must fit into Word.
const maxShift = _W - 4

// TODO(gri) Since we know the desired decimal precision when converting
// a floating-point number, we may be able to limit the number of decimal
// digits that need to be computed by init by providing an additional
// precision argument and keeping track of when a number was truncated early
// (equivalent of "sticky bit" in binary rounding).

// TODO(gri) Along the same lines, enforce some limit to shift magnitudes
// to avoid "infinitely" long running conversions (until we run out of space).

// Init initializes x to the decimal representation of m << shift (for
// shift >= 0), or m >> -shift (for shift < 0).
func (x *decimal) init(m nat, shift int) {
	// special case 0
	if len(m) == 0 {
		x.mant = x.mant[:0]
		x.exp = 0
		return
	}

	// Optimization: If we need to shift right, first remove any trailing
	// zero bits from m to reduce shift amount that needs to be done in
	// decimal format (since that is likely slower).
	if shift < 0 {
		ntz := m.trailingZeroBits()
		s := uint(-shift)
		if s >= ntz {
			s = ntz // shift at most ntz bits
		}
		m = nat(nil).rsh(m, s)
		shift += int(s)
	}

	// Do any shift left in binary representation.
	if shift > 0 {
		m = nat(nil).lsh(m, uint(shift))
		shift = 0
	}

	// Convert mantissa into decimal representation.
	s := m.utoa(10)
	n := len(s)
	x.exp = n
	// Trim trailing zeros; instead the exponent is tracking
	// the decimal point independent of the number of digits.
	for n > 0 && s[n-1] == '0' {
		n--
	}
	x.mant = append(x.mant[:0], s[:n]...)

	// Do any (remaining) shift right in decimal representation.
	if shift < 0 {
		for shift < -maxShift {
			rsh(x, maxShift)
			shift += maxShift
		}
		rsh(x, uint(-shift))
	}
}

// rsh implements x >> s, for s <= maxShift.
func rsh(x *decimal, s uint) {
	// Division by 1<<s using shift-and-subtract algorithm.

	// pick up enough leading digits to cover first shift
	r := 0 // read index
	var n Word
	for n>>s == 0 && r < len(x.mant) {
		ch := Word(x.mant[r])
		r++
		n = n*10 + ch - '0'
	}
	if n == 0 {
		// x == 0; shouldn't get here, but handle anyway
		x.mant = x.mant[:0]
		return
	}
	for n>>s == 0 {
		r++
		n *= 10
	}
	x.exp += 1 - r

	// read a digit, write a digit
	w := 0 // write index
	mask := Word(1)<<s - 1
	for r < len(x.mant) {
		ch := Word(x.mant[r])
		r++
		d := n >> s
		n &= mask // n -= d << s
		x.mant[w] = byte(d + '0')
		w++
		n = n*10 + ch - '0'
	}

	// write extra digits that still fit
	for n > 0 && w < len(x.mant) {
		d := n >> s
		n &= mask
		x.mant[w] = byte(d + '0')
		w++
		n = n * 10
	}
	x.mant = x.mant[:w] // the number may be shorter (e.g. 1024 >> 10)

	// append additional digits that didn't fit
	for n > 0 {
		d := n >> s
		n &= mask
		x.mant = append(x.mant, byte(d+'0'))
		n = n * 10
	}

	trim(x)
}

func (x *decimal) String() string {
	if len(x.mant) == 0 {
		return "0"
	}

	var buf []byte
	switch {
	case x.exp <= 0:
		// 0.00ddd
		buf = make([]byte, 0, 2+(-x.exp)+len(x.mant))
		buf = append(buf, "0."...)
		buf = appendZeros(buf, -x.exp)
		buf = append(buf, x.mant...)

	case /* 0 < */ x.exp < len(x.mant):
		// dd.ddd
		buf = make([]byte, 0, 1+len(x.mant))
		buf = append(buf, x.mant[:x.exp]...)
		buf = append(buf, '.')
		buf = append(buf, x.mant[x.exp:]...)

	default: // len(x.mant) <= x.exp
		// ddd00
		buf = make([]byte, 0, x.exp)
		buf = append(buf, x.mant...)
		buf = appendZeros(buf, x.exp-len(x.mant))
	}

	return string(buf)
}

// appendZeros appends n 0 digits to buf and returns buf.
func appendZeros(buf []byte, n int) []byte {
	for ; n > 0; n-- {
		buf = append(buf, '0')
	}
	return buf
}

// shouldRoundUp reports if x should be rounded up
// if shortened to n digits. n must be a valid index
// for x.mant.
func shouldRoundUp(x *decimal, n int) bool {
	if x.mant[n] == '5' && n+1 == len(x.mant) {
		// exactly halfway - round to even
		return n > 0 && (x.mant[n-1]-'0')&1 != 0
	}
	// not halfway - digit tells all (x.mant has no trailing zeros)
	return x.mant[n] >= '5'
}

// round sets x to (at most) n mantissa digits by rounding it
// to the nearest even value with n (or fever) mantissa digits.
// If n < 0, x remains unchanged.
func (x *decimal) round(n int) {
	if n < 0 || n >= len(x.mant) {
		return // nothing to do
	}

	if shouldRoundUp(x, n) {
		x.roundUp(n)
	} else {
		x.roundDown(n)
	}
}

func (x *decimal) roundUp(n int) {
	if n < 0 || n >= len(x.mant) {
		return // nothing to do
	}
	// 0 <= n < len(x.mant)

	// find first digit < '9'
	for n > 0 && x.mant[n-1] >= '9' {
		n--
	}

	if n == 0 {
		// all digits are '9's => round up to '1' and update exponent
		x.mant[0] = '1' // ok since len(x.mant) > n
		x.mant = x.mant[:1]
		x.exp++
		return
	}

	// n > 0 && x.mant[n-1] < '9'
	x.mant[n-1]++
	x.mant = x.mant[:n]
	// x already trimmed
}

func (x *decimal) roundDown(n int) {
	if n < 0 || n >= len(x.mant) {
		return // nothing to do
	}
	x.mant = x.mant[:n]
	trim(x)
}

// trim cuts off any trailing zeros from x's mantissa;
// they are meaningless for the value of x.
func trim(x *decimal) {
	i := len(x.mant)
	for i > 0 && x.mant[i-1] == '0' {
		i--
	}
	x.mant = x.mant[:i]
	if i == 0 {
		x.exp = 0
	}
}

```

// === FILE: references!/go/src/math/big/doc.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package big implements arbitrary-precision arithmetic (big numbers).
The following numeric types are supported:

	Int    signed integers
	Rat    rational numbers
	Float  floating-point numbers

The zero value for an [Int], [Rat], or [Float] correspond to 0. Thus, new
values can be declared in the usual ways and denote 0 without further
initialization:

	var x Int        // &x is an *Int of value 0
	var r = &Rat{}   // r is a *Rat of value 0
	y := new(Float)  // y is a *Float of value 0

Alternatively, new values can be allocated and initialized with factory
functions of the form:

	func NewT(v V) *T

For instance, [NewInt](x) returns an *[Int] set to the value of the int64
argument x, [NewRat](a, b) returns a *[Rat] set to the fraction a/b where
a and b are int64 values, and [NewFloat](f) returns a *[Float] initialized
to the float64 argument f. More flexibility is provided with explicit
setters, for instance:

	var z1 Int
	z1.SetUint64(123)                 // z1 := 123
	z2 := new(Rat).SetFloat64(1.25)   // z2 := 5/4
	z3 := new(Float).SetInt(z1)       // z3 := 123.0

Setters, numeric operations and predicates are represented as methods of
the form:

	func (z *T) SetV(v V) *T          // z = v
	func (z *T) Unary(x *T) *T        // z = unary x
	func (z *T) Binary(x, y *T) *T    // z = x binary y
	func (x *T) Pred() P              // p = pred(x)

with T one of [Int], [Rat], or [Float]. For unary and binary operations, the
result is the receiver (usually named z in that case; see below); if it
is one of the operands x or y it may be safely overwritten (and its memory
reused).

Arithmetic expressions are typically written as a sequence of individual
method calls, with each call corresponding to an operation. The receiver
denotes the result and the method arguments are the operation's operands.
For instance, given three *Int values a, b and c, the invocation

	c.Add(a, b)

computes the sum a + b and stores the result in c, overwriting whatever
value was held in c before. Unless specified otherwise, operations permit
aliasing of parameters, so it is perfectly ok to write

	sum.Add(sum, x)

to accumulate values x in a sum.

(By always passing in a result value via the receiver, memory use can be
much better controlled. Instead of having to allocate new memory for each
result, an operation can reuse the space allocated for the result value,
and overwrite that value with the new result in the process.)

Notational convention: Incoming method parameters (including the receiver)
are named consistently in the API to clarify their use. Incoming operands
are usually named x, y, a, b, and so on, but never z. A parameter specifying
the result is named z (typically the receiver).

For instance, the arguments for (*Int).Add are named x and y, and because
the receiver specifies the result destination, it is called z:

	func (z *Int) Add(x, y *Int) *Int

Methods of this form typically return the incoming receiver as well, to
enable simple call chaining.

Methods which don't require a result value to be passed in (for instance,
[Int.Sign]), simply return the result. In this case, the receiver is typically
the first operand, named x:

	func (x *Int) Sign() int

Various methods support conversions between strings and corresponding
numeric values, and vice versa: *[Int], *[Rat], and *[Float] values implement
the Stringer interface for a (default) string representation of the value,
but also provide SetString methods to initialize a value from a string in
a variety of supported formats (see the respective SetString documentation).

Finally, *[Int], *[Rat], and *[Float] satisfy [fmt.Scanner] for scanning
and (except for *[Rat]) the Formatter interface for formatted printing.
*/
package big

```

// === FILE: references!/go/src/math/big/float.go ===
```go
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements multi-precision floating-point numbers.
// Like in the GNU MPFR library (https://www.mpfr.org/), operands
// can be of mixed precision. Unlike MPFR, the rounding mode is
// not specified with each operation, but with each operand. The
// rounding mode of the result operand determines the rounding
// mode of an operation. This is a from-scratch implementation.

package big

import (
	"fmt"
	"math"
	"math/bits"
)

const debugFloat = false // enable for debugging

// A nonzero finite Float represents a multi-precision floating point number
//
//	sign × mantissa × 2**exponent
//
// with 0.5 <= mantissa < 1.0, and MinExp <= exponent <= MaxExp.
// A Float may also be zero (+0, -0) or infinite (+Inf, -Inf).
// All Floats are ordered, and the ordering of two Floats x and y
// is defined by x.Cmp(y).
//
// Each Float value also has a precision, rounding mode, and accuracy.
// The precision is the maximum number of mantissa bits available to
// represent the value. The rounding mode specifies how a result should
// be rounded to fit into the mantissa bits, and accuracy describes the
// rounding error with respect to the exact result.
//
// Unless specified otherwise, all operations (including setters) that
// specify a *Float variable for the result (usually via the receiver
// with the exception of [Float.MantExp]), round the numeric result according
// to the precision and rounding mode of the result variable.
//
// If the provided result precision is 0 (see below), it is set to the
// precision of the argument with the largest precision value before any
// rounding takes place, and the rounding mode remains unchanged. Thus,
// uninitialized Floats provided as result arguments will have their
// precision set to a reasonable value determined by the operands, and
// their mode is the zero value for RoundingMode (ToNearestEven).
//
// By setting the desired precision to 24 or 53 and using matching rounding
// mode (typically [ToNearestEven]), Float operations produce the same results
// as the corresponding float32 or float64 IEEE 754 arithmetic for operands
// that correspond to normal (i.e., not denormal) float32 or float64 numbers.
// Exponent underflow and overflow lead to a 0 or an Infinity for different
// values than IEEE 754 because Float exponents have a much larger range.
//
// The zero (uninitialized) value for a Float is ready to use and represents
// the number +0.0 exactly, with precision 0 and rounding mode [ToNearestEven].
//
// Operations always take pointer arguments (*Float) rather
// than Float values, and each unique Float value requires
// its own unique *Float pointer. To "copy" a Float value,
// an existing (or newly allocated) Float must be set to
// a new value using the [Float.Set] method; shallow copies
// of Floats are not supported and may lead to errors.
type Float struct {
	prec uint32
	mode RoundingMode
	acc  Accuracy
	form form
	neg  bool
	mant nat
	exp  int32
}

// An ErrNaN panic is raised by a [Float] operation that would lead to
// a NaN under IEEE 754 rules. An ErrNaN implements the error interface.
type ErrNaN struct {
	msg string
}

func (err ErrNaN) Error() string {
	return err.msg
}

// NewFloat allocates and returns a new [Float] set to x,
// with precision 53 and rounding mode [ToNearestEven].
// NewFloat panics with [ErrNaN] if x is a NaN.
func NewFloat(x float64) *Float {
	if math.IsNaN(x) {
		panic(ErrNaN{"NewFloat(NaN)"})
	}
	return new(Float).SetFloat64(x)
}

// Exponent and precision limits.
const (
	MaxExp  = math.MaxInt32  // largest supported exponent
	MinExp  = math.MinInt32  // smallest supported exponent
	MaxPrec = math.MaxUint32 // largest (theoretically) supported precision; likely memory-limited
)

// Internal representation: The mantissa bits x.mant of a nonzero finite
// Float x are stored in a nat slice long enough to hold up to x.prec bits;
// the slice may (but doesn't have to) be shorter if the mantissa contains
// trailing 0 bits. x.mant is normalized if the msb of x.mant == 1 (i.e.,
// the msb is shifted all the way "to the left"). Thus, if the mantissa has
// trailing 0 bits or x.prec is not a multiple of the Word size _W,
// x.mant[0] has trailing zero bits. The msb of the mantissa corresponds
// to the value 0.5; the exponent x.exp shifts the binary point as needed.
//
// A zero or non-finite Float x ignores x.mant and x.exp.
//
// x                 form      neg      mant         exp
// ----------------------------------------------------------
// ±0                zero      sign     -            -
// 0 < |x| < +Inf    finite    sign     mantissa     exponent
// ±Inf              inf       sign     -            -

// A form value describes the internal representation.
type form byte

// The form value order is relevant - do not change!
const (
	zero form = iota
	finite
	inf
)

// RoundingMode determines how a [Float] value is rounded to the
// desired precision. Rounding may change the [Float] value; the
// rounding error is described by the [Float]'s [Accuracy].
type RoundingMode byte

// These constants define supported rounding modes.
const (
	ToNearestEven RoundingMode = iota // == IEEE 754-2008 roundTiesToEven
	ToNearestAway                     // == IEEE 754-2008 roundTiesToAway
	ToZero                            // == IEEE 754-2008 roundTowardZero
	AwayFromZero                      // no IEEE 754-2008 equivalent
	ToNegativeInf                     // == IEEE 754-2008 roundTowardNegative
	ToPositiveInf                     // == IEEE 754-2008 roundTowardPositive
)

//go:generate stringer -type=RoundingMode

// Accuracy describes the rounding error produced by the most recent
// operation that generated a [Float] value, relative to the exact value.
type Accuracy int8

// Constants describing the [Accuracy] of a [Float].
const (
	Below Accuracy = -1
	Exact Accuracy = 0
	Above Accuracy = +1
)

//go:generate stringer -type=Accuracy

// SetPrec sets z's precision to prec and returns the (possibly) rounded
// value of z. Rounding occurs according to z's rounding mode if the mantissa
// cannot be represented in prec bits without loss of precision.
// SetPrec(0) maps all finite values to ±0; infinite values remain unchanged.
// If prec > [MaxPrec], it is set to [MaxPrec].
func (z *Float) SetPrec(prec uint) *Float {
	z.acc = Exact // optimistically assume no rounding is needed

	// special case
	if prec == 0 {
		z.prec = 0
		if z.form == finite {
			// truncate z to 0
			z.acc = makeAcc(z.neg)
			z.form = zero
		}
		return z
	}

	// general case
	if prec > MaxPrec {
		prec = MaxPrec
	}
	old := z.prec
	z.prec = uint32(prec)
	if z.prec < old {
		z.round(0)
	}
	return z
}

func makeAcc(above bool) Accuracy {
	if above {
		return Above
	}
	return Below
}

// SetMode sets z's rounding mode to mode and returns an exact z.
// z remains unchanged otherwise.
// z.SetMode(z.Mode()) is a cheap way to set z's accuracy to [Exact].
func (z *Float) SetMode(mode RoundingMode) *Float {
	z.mode = mode
	z.acc = Exact
	return z
}

// Prec returns the mantissa precision of x in bits.
// The result may be 0 for |x| == 0 and |x| == Inf.
func (x *Float) Prec() uint {
	return uint(x.prec)
}

// MinPrec returns the minimum precision required to represent x exactly
// (i.e., the smallest prec before x.SetPrec(prec) would start rounding x).
// The result is 0 for |x| == 0 and |x| == Inf.
func (x *Float) MinPrec() uint {
	if x.form != finite {
		return 0
	}
	return uint(len(x.mant))*_W - x.mant.trailingZeroBits()
}

// Mode returns the rounding mode of x.
func (x *Float) Mode() RoundingMode {
	return x.mode
}

// Acc returns the accuracy of x produced by the most recent
// operation, unless explicitly documented otherwise by that
// operation.
func (x *Float) Acc() Accuracy {
	return x.acc
}

// Sign returns:
//   - -1 if x < 0;
//   - 0 if x is ±0;
//   - +1 if x > 0.
func (x *Float) Sign() int {
	if debugFloat {
		x.validate()
	}
	if x.form == zero {
		return 0
	}
	if x.neg {
		return -1
	}
	return 1
}

// MantExp breaks x into its mantissa and exponent components
// and returns the exponent. If a non-nil mant argument is
// provided its value is set to the mantissa of x, with the
// same precision and rounding mode as x. The components
// satisfy x == mant × 2**exp, with 0.5 <= |mant| < 1.0.
// Calling MantExp with a nil argument is an efficient way to
// get the exponent of the receiver.
//
// Special cases are:
//
//	(  ±0).MantExp(mant) = 0, with mant set to   ±0
//	(±Inf).MantExp(mant) = 0, with mant set to ±Inf
//
// x and mant may be the same in which case x is set to its
// mantissa value.
func (x *Float) MantExp(mant *Float) (exp int) {
	if debugFloat {
		x.validate()
	}
	if x.form == finite {
		exp = int(x.exp)
	}
	if mant != nil {
		mant.Copy(x)
		if mant.form == finite {
			mant.exp = 0
		}
	}
	return
}

func (z *Float) setExpAndRound(exp int64, sbit uint) {
	if exp < MinExp {
		// underflow
		z.acc = makeAcc(z.neg)
		z.form = zero
		return
	}

	if exp > MaxExp {
		// overflow
		z.acc = makeAcc(!z.neg)
		z.form = inf
		return
	}

	z.form = finite
	z.exp = int32(exp)
	z.round(sbit)
}

// SetMantExp sets z to mant × 2**exp and returns z.
// The result z has the same precision and rounding mode
// as mant. SetMantExp is an inverse of [Float.MantExp] but does
// not require 0.5 <= |mant| < 1.0. Specifically, for a
// given x of type *[Float], SetMantExp relates to [Float.MantExp]
// as follows:
//
//	mant := new(Float)
//	new(Float).SetMantExp(mant, x.MantExp(mant)).Cmp(x) == 0
//
// Special cases are:
//
//	z.SetMantExp(  ±0, exp) =   ±0
//	z.SetMantExp(±Inf, exp) = ±Inf
//
// z and mant may be the same in which case z's exponent
// is set to exp.
func (z *Float) SetMantExp(mant *Float, exp int) *Float {
	if debugFloat {
		z.validate()
		mant.validate()
	}
	z.Copy(mant)

	if z.form == finite {
		// 0 < |mant| < +Inf
		z.setExpAndRound(int64(z.exp)+int64(exp), 0)
	}
	return z
}

// Signbit reports whether x is negative or negative zero.
func (x *Float) Signbit() bool {
	return x.neg
}

// IsInf reports whether x is +Inf or -Inf.
func (x *Float) IsInf() bool {
	return x.form == inf
}

// IsInt reports whether x is an integer.
// ±Inf values are not integers.
func (x *Float) IsInt() bool {
	if debugFloat {
		x.validate()
	}
	// special cases
	if x.form != finite {
		return x.form == zero
	}
	// x.form == finite
	if x.exp <= 0 {
		return false
	}
	// x.exp > 0
	return x.prec <= uint32(x.exp) || x.MinPrec() <= uint(x.exp) // not enough bits for fractional mantissa
}

// debugging support
func (x *Float) validate() {
	if !debugFloat {
		// avoid performance bugs
		panic("validate called but debugFloat is not set")
	}
	if msg := x.validate0(); msg != "" {
		panic(msg)
	}
}

func (x *Float) validate0() string {
	if x.form != finite {
		return ""
	}
	m := len(x.mant)
	if m == 0 {
		return "nonzero finite number with empty mantissa"
	}
	const msb = 1 << (_W - 1)
	if x.mant[m-1]&msb == 0 {
		return fmt.Sprintf("msb not set in last word %#x of %s", x.mant[m-1], x.Text('p', 0))
	}
	if x.prec == 0 {
		return "zero precision finite number"
	}
	return ""
}

// round rounds z according to z.mode to z.prec bits and sets z.acc accordingly.
// sbit must be 0 or 1 and summarizes any "sticky bit" information one might
// have before calling round. z's mantissa must be normalized (with the msb set)
// or empty.
//
// CAUTION: The rounding modes [ToNegativeInf], [ToPositiveInf] are affected by the
// sign of z. For correct rounding, the sign of z must be set correctly before
// calling round.
func (z *Float) round(sbit uint) {
	if debugFloat {
		z.validate()
	}

	z.acc = Exact
	if z.form != finite {
		// ±0 or ±Inf => nothing left to do
		return
	}
	// z.form == finite && len(z.mant) > 0
	// m > 0 implies z.prec > 0 (checked by validate)

	m := uint32(len(z.mant)) // present mantissa length in words
	bits := m * _W           // present mantissa bits; bits > 0
	if bits <= z.prec {
		// mantissa fits => nothing to do
		return
	}
	// bits > z.prec

	// Rounding is based on two bits: the rounding bit (rbit) and the
	// sticky bit (sbit). The rbit is the bit immediately before the
	// z.prec leading mantissa bits (the "0.5"). The sbit is set if any
	// of the bits before the rbit are set (the "0.25", "0.125", etc.):
	//
	//   rbit  sbit  => "fractional part"
	//
	//   0     0        == 0
	//   0     1        >  0  , < 0.5
	//   1     0        == 0.5
	//   1     1        >  0.5, < 1.0

	// bits > z.prec: mantissa too large => round
	r := uint(bits - z.prec - 1) // rounding bit position; r >= 0
	rbit := z.mant.bit(r) & 1    // rounding bit; be safe and ensure it's a single bit
	// The sticky bit is only needed for rounding ToNearestEven
	// or when the rounding bit is zero. Avoid computation otherwise.
	if sbit == 0 && (rbit == 0 || z.mode == ToNearestEven) {
		sbit = z.mant.sticky(r)
	}
	sbit &= 1 // be safe and ensure it's a single bit

	// cut off extra words
	n := (z.prec + (_W - 1)) / _W // mantissa length in words for desired precision
	if m > n {
		copy(z.mant, z.mant[m-n:]) // move n last words to front
		z.mant = z.mant[:n]
	}

	// determine number of trailing zero bits (ntz) and compute lsb mask of mantissa's least-significant word
	ntz := n*_W - z.prec // 0 <= ntz < _W
	lsb := Word(1) << ntz

	// round if result is inexact
	if rbit|sbit != 0 {
		// Make rounding decision: The result mantissa is truncated ("rounded down")
		// by default. Decide if we need to increment, or "round up", the (unsigned)
		// mantissa.
		inc := false
		switch z.mode {
		case ToNegativeInf:
			inc = z.neg
		case ToZero:
			// nothing to do
		case ToNearestEven:
			inc = rbit != 0 && (sbit != 0 || z.mant[0]&lsb != 0)
		case ToNearestAway:
			inc = rbit != 0
		case AwayFromZero:
			inc = true
		case ToPositiveInf:
			inc = !z.neg
		default:
			panic("unreachable")
		}

		// A positive result (!z.neg) is Above the exact result if we increment,
		// and it's Below if we truncate (Exact results require no rounding).
		// For a negative result (z.neg) it is exactly the opposite.
		z.acc = makeAcc(inc != z.neg)

		if inc {
			// add 1 to mantissa
			if addVW(z.mant, z.mant, lsb) != 0 {
				// mantissa overflow => adjust exponent
				if z.exp >= MaxExp {
					// exponent overflow
					z.form = inf
					return
				}
				z.exp++
				// adjust mantissa: divide by 2 to compensate for exponent adjustment
				rshVU(z.mant, z.mant, 1)
				// set msb == carry == 1 from the mantissa overflow above
				const msb = 1 << (_W - 1)
				z.mant[n-1] |= msb
			}
		}
	}

	// zero out trailing bits in least-significant word
	z.mant[0] &^= lsb - 1

	if debugFloat {
		z.validate()
	}
}

func (z *Float) setBits64(neg bool, x uint64) *Float {
	if z.prec == 0 {
		z.prec = 64
	}
	z.acc = Exact
	z.neg = neg
	if x == 0 {
		z.form = zero
		return z
	}
	// x != 0
	z.form = finite
	s := bits.LeadingZeros64(x)
	z.mant = z.mant.setUint64(x << uint(s))
	z.exp = int32(64 - s) // always fits
	if z.prec < 64 {
		z.round(0)
	}
	return z
}

// SetUint64 sets z to the (possibly rounded) value of x and returns z.
// If z's precision is 0, it is changed to 64 (and rounding will have
// no effect).
func (z *Float) SetUint64(x uint64) *Float {
	return z.setBits64(false, x)
}

// SetInt64 sets z to the (possibly rounded) value of x and returns z.
// If z's precision is 0, it is changed to 64 (and rounding will have
// no effect).
func (z *Float) SetInt64(x int64) *Float {
	u := x
	if u < 0 {
		u = -u
	}
	// We cannot simply call z.SetUint64(uint64(u)) and change
	// the sign afterwards because the sign affects rounding.
	return z.setBits64(x < 0, uint64(u))
}

// SetFloat64 sets z to the (possibly rounded) value of x and returns z.
// If z's precision is 0, it is changed to 53 (and rounding will have
// no effect). SetFloat64 panics with [ErrNaN] if x is a NaN.
func (z *Float) SetFloat64(x float64) *Float {
	if z.prec == 0 {
		z.prec = 53
	}
	if math.IsNaN(x) {
		panic(ErrNaN{"Float.SetFloat64(NaN)"})
	}
	z.acc = Exact
	z.neg = math.Signbit(x) // handle -0, -Inf correctly
	if x == 0 {
		z.form = zero
		return z
	}
	if math.IsInf(x, 0) {
		z.form = inf
		return z
	}
	// normalized x != 0
	z.form = finite
	fmant, exp := math.Frexp(x) // get normalized mantissa
	z.mant = z.mant.setUint64(1<<63 | math.Float64bits(fmant)<<11)
	z.exp = int32(exp) // always fits
	if z.prec < 53 {
		z.round(0)
	}
	return z
}

// fnorm normalizes mantissa m by shifting it to the left
// such that the msb of the most-significant word (msw) is 1.
// It returns the shift amount. It assumes that len(m) != 0.
func fnorm(m nat) int64 {
	if debugFloat && (len(m) == 0 || m[len(m)-1] == 0) {
		panic("msw of mantissa is 0")
	}
	s := nlz(m[len(m)-1])
	if s > 0 {
		c := lshVU(m, m, s)
		if debugFloat && c != 0 {
			panic("nlz or lshVU incorrect")
		}
	}
	return int64(s)
}

// SetInt sets z to the (possibly rounded) value of x and returns z.
// If z's precision is 0, it is changed to the larger of x.BitLen()
// or 64 (and rounding will have no effect).
func (z *Float) SetInt(x *Int) *Float {
	// TODO(gri) can be more efficient if z.prec > 0
	// but small compared to the size of x, or if there
	// are many trailing 0's.
	bits := uint32(x.BitLen())
	if z.prec == 0 {
		z.prec = max(bits, 64)
	}
	z.acc = Exact
	z.neg = x.neg
	if len(x.abs) == 0 {
		z.form = zero
		return z
	}
	// x != 0
	z.mant = z.mant.set(x.abs)
	fnorm(z.mant)
	z.setExpAndRound(int64(bits), 0)
	return z
}

// SetRat sets z to the (possibly rounded) value of x and returns z.
// If z's precision is 0, it is changed to the largest of a.BitLen(),
// b.BitLen(), or 64; with x = a/b.
func (z *Float) SetRat(x *Rat) *Float {
	if x.IsInt() {
		return z.SetInt(x.Num())
	}
	var a, b Float
	a.SetInt(x.Num())
	b.SetInt(x.Denom())
	if z.prec == 0 {
		z.prec = max(a.prec, b.prec)
	}
	return z.Quo(&a, &b)
}

// SetInf sets z to the infinite Float -Inf if signbit is
// set, or +Inf if signbit is not set, and returns z. The
// precision of z is unchanged and the result is always
// [Exact].
func (z *Float) SetInf(signbit bool) *Float {
	z.acc = Exact
	z.form = inf
	z.neg = signbit
	return z
}

// Set sets z to the (possibly rounded) value of x and returns z.
// If z's precision is 0, it is changed to the precision of x
// before setting z (and rounding will have no effect).
// Rounding is performed according to z's precision and rounding
// mode; and z's accuracy reports the result error relative to the
// exact (not rounded) result.
func (z *Float) Set(x *Float) *Float {
	if debugFloat {
		x.validate()
	}
	z.acc = Exact
	if z != x {
		z.form = x.form
		z.neg = x.neg
		if x.form == finite {
			z.exp = x.exp
			z.mant = z.mant.set(x.mant)
		}
		if z.prec == 0 {
			z.prec = x.prec
		} else if z.prec < x.prec {
			z.round(0)
		}
	}
	return z
}

// Copy sets z to x, with the same precision, rounding mode, and accuracy as x.
// Copy returns z. If x and z are identical, Copy is a no-op.
func (z *Float) Copy(x *Float) *Float {
	if debugFloat {
		x.validate()
	}
	if z != x {
		z.prec = x.prec
		z.mode = x.mode
		z.acc = x.acc
		z.form = x.form
		z.neg = x.neg
		if z.form == finite {
			z.mant = z.mant.set(x.mant)
			z.exp = x.exp
		}
	}
	return z
}

// msb32 returns the 32 most significant bits of x.
func msb32(x nat) uint32 {
	i := len(x) - 1
	if i < 0 {
		return 0
	}
	if debugFloat && x[i]&(1<<(_W-1)) == 0 {
		panic("x not normalized")
	}
	switch _W {
	case 32:
		return uint32(x[i])
	case 64:
		return uint32(x[i] >> 32)
	}
	panic("unreachable")
}

// msb64 returns the 64 most significant bits of x.
func msb64(x nat) uint64 {
	i := len(x) - 1
	if i < 0 {
		return 0
	}
	if debugFloat && x[i]&(1<<(_W-1)) == 0 {
		panic("x not normalized")
	}
	switch _W {
	case 32:
		v := uint64(x[i]) << 32
		if i > 0 {
			v |= uint64(x[i-1])
		}
		return v
	case 64:
		return uint64(x[i])
	}
	panic("unreachable")
}

// Uint64 returns the unsigned integer resulting from truncating x
// towards zero. If 0 <= x <= [math.MaxUint64], the result is [Exact]
// if x is an integer and [Below] otherwise.
// The result is (0, [Above]) for x < 0, and ([math.MaxUint64], [Below])
// for x > [math.MaxUint64].
func (x *Float) Uint64() (uint64, Accuracy) {
	if debugFloat {
		x.validate()
	}

	switch x.form {
	case finite:
		if x.neg {
			return 0, Above
		}
		// 0 < x < +Inf
		if x.exp <= 0 {
			// 0 < x < 1
			return 0, Below
		}
		// 1 <= x < Inf
		if x.exp <= 64 {
			// u = trunc(x) fits into a uint64
			u := msb64(x.mant) >> (64 - uint32(x.exp))
			if x.MinPrec() <= 64 {
				return u, Exact
			}
			return u, Below // x truncated
		}
		// x too large
		return math.MaxUint64, Below

	case zero:
		return 0, Exact

	case inf:
		if x.neg {
			return 0, Above
		}
		return math.MaxUint64, Below
	}

	panic("unreachable")
}

// Int64 returns the integer resulting from truncating x towards zero.
// If [math.MinInt64] <= x <= [math.MaxInt64], the result is [Exact] if x is
// an integer, and [Above] (x < 0) or [Below] (x > 0) otherwise.
// The result is ([math.MinInt64], [Above]) for x < [math.MinInt64],
// and ([math.MaxInt64], [Below]) for x > [math.MaxInt64].
func (x *Float) Int64() (int64, Accuracy) {
	if debugFloat {
		x.validate()
	}

	switch x.form {
	case finite:
		// 0 < |x| < +Inf
		acc := makeAcc(x.neg)
		if x.exp <= 0 {
			// 0 < |x| < 1
			return 0, acc
		}
		// x.exp > 0

		// 1 <= |x| < +Inf
		if x.exp <= 63 {
			// i = trunc(x) fits into an int64 (excluding math.MinInt64)
			i := int64(msb64(x.mant) >> (64 - uint32(x.exp)))
			if x.neg {
				i = -i
			}
			if x.MinPrec() <= uint(x.exp) {
				return i, Exact
			}
			return i, acc // x truncated
		}
		if x.neg {
			// check for special case x == math.MinInt64 (i.e., x == -(0.5 << 64))
			if x.exp == 64 && x.MinPrec() == 1 {
				acc = Exact
			}
			return math.MinInt64, acc
		}
		// x too large
		return math.MaxInt64, Below

	case zero:
		return 0, Exact

	case inf:
		if x.neg {
			return math.MinInt64, Above
		}
		return math.MaxInt64, Below
	}

	panic("unreachable")
}

// Float32 returns the float32 value nearest to x. If x is too small to be
// represented by a float32 (|x| < [math.SmallestNonzeroFloat32]), the result
// is (0, [Below]) or (-0, [Above]), respectively, depending on the sign of x.
// If x is too large to be represented by a float32 (|x| > [math.MaxFloat32]),
// the result is (+Inf, [Above]) or (-Inf, [Below]), depending on the sign of x.
func (x *Float) Float32() (float32, Accuracy) {
	if debugFloat {
		x.validate()
	}

	switch x.form {
	case finite:
		// 0 < |x| < +Inf

		const (
			fbits = 32                //        float size
			mbits = 23                //        mantissa size (excluding implicit msb)
			ebits = fbits - mbits - 1 //     8  exponent size
			bias  = 1<<(ebits-1) - 1  //   127  exponent bias
			dmin  = 1 - bias - mbits  //  -149  smallest unbiased exponent (denormal)
			emin  = 1 - bias          //  -126  smallest unbiased exponent (normal)
			emax  = bias              //   127  largest unbiased exponent (normal)
		)

		// Float mantissa m is 0.5 <= m < 1.0; compute exponent e for float32 mantissa.
		e := x.exp - 1 // exponent for normal mantissa m with 1.0 <= m < 2.0

		// Compute precision p for float32 mantissa.
		// If the exponent is too small, we have a denormal number before
		// rounding and fewer than p mantissa bits of precision available
		// (the exponent remains fixed but the mantissa gets shifted right).
		p := mbits + 1 // precision of normal float
		if e < emin {
			// recompute precision
			p = mbits + 1 - emin + int(e)
			// If p == 0, the mantissa of x is shifted so much to the right
			// that its msb falls immediately to the right of the float32
			// mantissa space. In other words, if the smallest denormal is
			// considered "1.0", for p == 0, the mantissa value m is >= 0.5.
			// If m > 0.5, it is rounded up to 1.0; i.e., the smallest denormal.
			// If m == 0.5, it is rounded down to even, i.e., 0.0.
			// If p < 0, the mantissa value m is <= "0.25" which is never rounded up.
			if p < 0 /* m <= 0.25 */ || p == 0 && x.mant.sticky(uint(len(x.mant))*_W-1) == 0 /* m == 0.5 */ {
				// underflow to ±0
				if x.neg {
					var z float32
					return -z, Above
				}
				return 0.0, Below
			}
			// otherwise, round up
			// We handle p == 0 explicitly because it's easy and because
			// Float.round doesn't support rounding to 0 bits of precision.
			if p == 0 {
				if x.neg {
					return -math.SmallestNonzeroFloat32, Below
				}
				return math.SmallestNonzeroFloat32, Above
			}
		}
		// p > 0

		// round
		var r Float
		r.prec = uint32(p)
		r.Set(x)
		e = r.exp - 1

		// Rounding may have caused r to overflow to ±Inf
		// (rounding never causes underflows to 0).
		// If the exponent is too large, also overflow to ±Inf.
		if r.form == inf || e > emax {
			// overflow
			if x.neg {
				return float32(math.Inf(-1)), Below
			}
			return float32(math.Inf(+1)), Above
		}
		// e <= emax

		// Determine sign, biased exponent, and mantissa.
		var sign, bexp, mant uint32
		if x.neg {
			sign = 1 << (fbits - 1)
		}

		// Rounding may have caused a denormal number to
		// become normal. Check again.
		if e < emin {
			// denormal number: recompute precision
			// Since rounding may have at best increased precision
			// and we have eliminated p <= 0 early, we know p > 0.
			// bexp == 0 for denormals
			p = mbits + 1 - emin + int(e)
			mant = msb32(r.mant) >> uint(fbits-p)
		} else {
			// normal number: emin <= e <= emax
			bexp = uint32(e+bias) << mbits
			mant = msb32(r.mant) >> ebits & (1<<mbits - 1) // cut off msb (implicit 1 bit)
		}

		return math.Float32frombits(sign | bexp | mant), r.acc

	case zero:
		if x.neg {
			var z float32
			return -z, Exact
		}
		return 0.0, Exact

	case inf:
		if x.neg {
			return float32(math.Inf(-1)), Exact
		}
		return float32(math.Inf(+1)), Exact
	}

	panic("unreachable")
}

// Float64 returns the float64 value nearest to x. If x is too small to be
// represented by a float64 (|x| < [math.SmallestNonzeroFloat64]), the result
// is (0, [Below]) or (-0, [Above]), respectively, depending on the sign of x.
// If x is too large to be represented by a float64 (|x| > [math.MaxFloat64]),
// the result is (+Inf, [Above]) or (-Inf, [Below]), depending on the sign of x.
func (x *Float) Float64() (float64, Accuracy) {
	if debugFloat {
		x.validate()
	}

	switch x.form {
	case finite:
		// 0 < |x| < +Inf

		const (
			fbits = 64                //        float size
			mbits = 52                //        mantissa size (excluding implicit msb)
			ebits = fbits - mbits - 1 //    11  exponent size
			bias  = 1<<(ebits-1) - 1  //  1023  exponent bias
			dmin  = 1 - bias - mbits  // -1074  smallest unbiased exponent (denormal)
			emin  = 1 - bias          // -1022  smallest unbiased exponent (normal)
			emax  = bias              //  1023  largest unbiased exponent (normal)
		)

		// Float mantissa m is 0.5 <= m < 1.0; compute exponent e for float64 mantissa.
		e := x.exp - 1 // exponent for normal mantissa m with 1.0 <= m < 2.0

		// Compute precision p for float64 mantissa.
		// If the exponent is too small, we have a denormal number before
		// rounding and fewer than p mantissa bits of precision available
		// (the exponent remains fixed but the mantissa gets shifted right).
		p := mbits + 1 // precision of normal float
		if e < emin {
			// recompute precision
			p = mbits + 1 - emin + int(e)
			// If p == 0, the mantissa of x is shifted so much to the right
			// that its msb falls immediately to the right of the float64
			// mantissa space. In other words, if the smallest denormal is
			// considered "1.0", for p == 0, the mantissa value m is >= 0.5.
			// If m > 0.5, it is rounded up to 1.0; i.e., the smallest denormal.
			// If m == 0.5, it is rounded down to even, i.e., 0.0.
			// If p < 0, the mantissa value m is <= "0.25" which is never rounded up.
			if p < 0 /* m <= 0.25 */ || p == 0 && x.mant.sticky(uint(len(x.mant))*_W-1) == 0 /* m == 0.5 */ {
				// underflow to ±0
				if x.neg {
					var z float64
					return -z, Above
				}
				return 0.0, Below
			}
			// otherwise, round up
			// We handle p == 0 explicitly because it's easy and because
			// Float.round doesn't support rounding to 0 bits of precision.
			if p == 0 {
				if x.neg {
					return -math.SmallestNonzeroFloat64, Below
				}
				return math.SmallestNonzeroFloat64, Above
			}
		}
		// p > 0

		// round
		var r Float
		r.prec = uint32(p)
		r.Set(x)
		e = r.exp - 1

		// Rounding may have caused r to overflow to ±Inf
		// (rounding never causes underflows to 0).
		// If the exponent is too large, also overflow to ±Inf.
		if r.form == inf || e > emax {
			// overflow
			if x.neg {
				return math.Inf(-1), Below
			}
			return math.Inf(+1), Above
		}
		// e <= emax

		// Determine sign, biased exponent, and mantissa.
		var sign, bexp, mant uint64
		if x.neg {
			sign = 1 << (fbits - 1)
		}

		// Rounding may have caused a denormal number to
		// become normal. Check again.
		if e < emin {
			// denormal number: recompute precision
			// Since rounding may have at best increased precision
			// and we have eliminated p <= 0 early, we know p > 0.
			// bexp == 0 for denormals
			p = mbits + 1 - emin + int(e)
			mant = msb64(r.mant) >> uint(fbits-p)
		} else {
			// normal number: emin <= e <= emax
			bexp = uint64(e+bias) << mbits
			mant = msb64(r.mant) >> ebits & (1<<mbits - 1) // cut off msb (implicit 1 bit)
		}

		return math.Float64frombits(sign | bexp | mant), r.acc

	case zero:
		if x.neg {
			var z float64
			return -z, Exact
		}
		return 0.0, Exact

	case inf:
		if x.neg {
			return math.Inf(-1), Exact
		}
		return math.Inf(+1), Exact
	}

	panic("unreachable")
}

// Int returns the result of truncating x towards zero;
// or nil if x is an infinity.
// The result is [Exact] if x.IsInt(); otherwise it is [Below]
// for x > 0, and [Above] for x < 0.
// If a non-nil *[Int] argument z is provided, [Int] stores
// the result in z instead of allocating a new [Int].
func (x *Float) Int(z *Int) (*Int, Accuracy) {
	if debugFloat {
		x.validate()
	}

	if z == nil && x.form <= finite {
		z = new(Int)
	}

	switch x.form {
	case finite:
		// 0 < |x| < +Inf
		acc := makeAcc(x.neg)
		if x.exp <= 0 {
			// 0 < |x| < 1
			return z.SetInt64(0), acc
		}
		// x.exp > 0

		// 1 <= |x| < +Inf
		// determine minimum required precision for x
		allBits := uint(len(x.mant)) * _W
		exp := uint(x.exp)
		if x.MinPrec() <= exp {
			acc = Exact
		}
		// shift mantissa as needed
		if z == nil {
			z = new(Int)
		}
		z.neg = x.neg
		switch {
		case exp > allBits:
			z.abs = z.abs.lsh(x.mant, exp-allBits)
		default:
			z.abs = z.abs.set(x.mant)
		case exp < allBits:
			z.abs = z.abs.rsh(x.mant, allBits-exp)
		}
		return z, acc

	case zero:
		return z.SetInt64(0), Exact

	case inf:
		return nil, makeAcc(x.neg)
	}

	panic("unreachable")
}

// Rat returns the rational number corresponding to x;
// or nil if x is an infinity.
// The result is [Exact] if x is not an Inf.
// If a non-nil *[Rat] argument z is provided, [Rat] stores
// the result in z instead of allocating a new [Rat].
func (x *Float) Rat(z *Rat) (*Rat, Accuracy) {
	if debugFloat {
		x.validate()
	}

	if z == nil && x.form <= finite {
		z = new(Rat)
	}

	switch x.form {
	case finite:
		// 0 < |x| < +Inf
		allBits := int32(len(x.mant)) * _W
		// build up numerator and denominator
		z.a.neg = x.neg
		switch {
		case x.exp > allBits:
			z.a.abs = z.a.abs.lsh(x.mant, uint(x.exp-allBits))
			z.b.abs = z.b.abs[:0] // == 1 (see Rat)
			// z already in normal form
		default:
			z.a.abs = z.a.abs.set(x.mant)
			z.b.abs = z.b.abs[:0] // == 1 (see Rat)
			// z already in normal form
		case x.exp < allBits:
			z.a.abs = z.a.abs.set(x.mant)
			t := z.b.abs.setUint64(1)
			z.b.abs = t.lsh(t, uint(allBits-x.exp))
			z.norm()
		}
		return z, Exact

	case zero:
		return z.SetInt64(0), Exact

	case inf:
		return nil, makeAcc(x.neg)
	}

	panic("unreachable")
}

// Abs sets z to the (possibly rounded) value |x| (the absolute value of x)
// and returns z.
func (z *Float) Abs(x *Float) *Float {
	z.Set(x)
	z.neg = false
	return z
}

// Neg sets z to the (possibly rounded) value of x with its sign negated,
// and returns z.
func (z *Float) Neg(x *Float) *Float {
	z.Set(x)
	z.neg = !z.neg
	return z
}

func validateBinaryOperands(x, y *Float) {
	if !debugFloat {
		// avoid performance bugs
		panic("validateBinaryOperands called but debugFloat is not set")
	}
	if len(x.mant) == 0 {
		panic("empty mantissa for x")
	}
	if len(y.mant) == 0 {
		panic("empty mantissa for y")
	}
}

// z = x + y, ignoring signs of x and y for the addition
// but using the sign of z for rounding the result.
// x and y must have a non-empty mantissa and valid exponent.
func (z *Float) uadd(x, y *Float) {
	// Note: This implementation requires 2 shifts most of the
	// time. It is also inefficient if exponents or precisions
	// differ by wide margins. The following article describes
	// an efficient (but much more complicated) implementation
	// compatible with the internal representation used here:
	//
	// Vincent Lefèvre: "The Generic Multiple-Precision Floating-
	// Point Addition With Exact Rounding (as in the MPFR Library)"
	// http://www.vinc17.net/research/papers/rnc6.pdf

	if debugFloat {
		validateBinaryOperands(x, y)
	}

	// compute exponents ex, ey for mantissa with "binary point"
	// on the right (mantissa.0) - use int64 to avoid overflow
	ex := int64(x.exp) - int64(len(x.mant))*_W
	ey := int64(y.exp) - int64(len(y.mant))*_W

	al := alias(z.mant, x.mant) || alias(z.mant, y.mant)

	// TODO(gri) having a combined add-and-shift primitive
	//           could make this code significantly faster
	switch {
	case ex < ey:
		if al {
			t := nat(nil).lsh(y.mant, uint(ey-ex))
			z.mant = z.mant.add(x.mant, t)
		} else {
			z.mant = z.mant.lsh(y.mant, uint(ey-ex))
			z.mant = z.mant.add(x.mant, z.mant)
		}
	default:
		// ex == ey, no shift needed
		z.mant = z.mant.add(x.mant, y.mant)
	case ex > ey:
		if al {
			t := nat(nil).lsh(x.mant, uint(ex-ey))
			z.mant = z.mant.add(t, y.mant)
		} else {
			z.mant = z.mant.lsh(x.mant, uint(ex-ey))
			z.mant = z.mant.add(z.mant, y.mant)
		}
		ex = ey
	}
	// len(z.mant) > 0

	z.setExpAndRound(ex+int64(len(z.mant))*_W-fnorm(z.mant), 0)
}

// z = x - y for |x| > |y|, ignoring signs of x and y for the subtraction
// but using the sign of z for rounding the result.
// x and y must have a non-empty mantissa and valid exponent.
func (z *Float) usub(x, y *Float) {
	// This code is symmetric to uadd.
	// We have not factored the common code out because
	// eventually uadd (and usub) should be optimized
	// by special-casing, and the code will diverge.

	if debugFloat {
		validateBinaryOperands(x, y)
	}

	ex := int64(x.exp) - int64(len(x.mant))*_W
	ey := int64(y.exp) - int64(len(y.mant))*_W

	al := alias(z.mant, x.mant) || alias(z.mant, y.mant)

	switch {
	case ex < ey:
		if al {
			t := nat(nil).lsh(y.mant, uint(ey-ex))
			z.mant = t.sub(x.mant, t)
		} else {
			z.mant = z.mant.lsh(y.mant, uint(ey-ex))
			z.mant = z.mant.sub(x.mant, z.mant)
		}
	default:
		// ex == ey, no shift needed
		z.mant = z.mant.sub(x.mant, y.mant)
	case ex > ey:
		if al {
			t := nat(nil).lsh(x.mant, uint(ex-ey))
			z.mant = t.sub(t, y.mant)
		} else {
			z.mant = z.mant.lsh(x.mant, uint(ex-ey))
			z.mant = z.mant.sub(z.mant, y.mant)
		}
		ex = ey
	}

	// operands may have canceled each other out
	if len(z.mant) == 0 {
		z.acc = Exact
		z.form = zero
		z.neg = false
		return
	}
	// len(z.mant) > 0

	z.setExpAndRound(ex+int64(len(z.mant))*_W-fnorm(z.mant), 0)
}

// z = x * y, ignoring signs of x and y for the multiplication
// but using the sign of z for rounding the result.
// x and y must have a non-empty mantissa and valid exponent.
func (z *Float) umul(x, y *Float) {
	if debugFloat {
		validateBinaryOperands(x, y)
	}

	// Note: This is doing too much work if the precision
	// of z is less than the sum of the precisions of x
	// and y which is often the case (e.g., if all floats
	// have the same precision).
	// TODO(gri) Optimize this for the common case.

	e := int64(x.exp) + int64(y.exp)
	if x == y {
		z.mant = z.mant.sqr(nil, x.mant)
	} else {
		z.mant = z.mant.mul(nil, x.mant, y.mant)
	}
	z.setExpAndRound(e-fnorm(z.mant), 0)
}

// z = x / y, ignoring signs of x and y for the division
// but using the sign of z for rounding the result.
// x and y must have a non-empty mantissa and valid exponent.
func (z *Float) uquo(x, y *Float) {
	if debugFloat {
		validateBinaryOperands(x, y)
	}

	// mantissa length in words for desired result precision + 1
	// (at least one extra bit so we get the rounding bit after
	// the division)
	n := int(z.prec/_W) + 1

	// compute adjusted x.mant such that we get enough result precision
	xadj := x.mant
	if d := n - len(x.mant) + len(y.mant); d > 0 {
		// d extra words needed => add d "0 digits" to x
		xadj = make(nat, len(x.mant)+d)
		copy(xadj[d:], x.mant)
	}
	// TODO(gri): If we have too many digits (d < 0), we should be able
	// to shorten x for faster division. But we must be extra careful
	// with rounding in that case.

	// Compute d before division since there may be aliasing of x.mant
	// (via xadj) or y.mant with z.mant.
	d := len(xadj) - len(y.mant)

	// divide
	stk := getStack()
	defer stk.free()
	var r nat
	z.mant, r = z.mant.div(stk, nil, xadj, y.mant)
	e := int64(x.exp) - int64(y.exp) - int64(d-len(z.mant))*_W

	// The result is long enough to include (at least) the rounding bit.
	// If there's a non-zero remainder, the corresponding fractional part
	// (if it were computed), would have a non-zero sticky bit (if it were
	// zero, it couldn't have a non-zero remainder).
	var sbit uint
	if len(r) > 0 {
		sbit = 1
	}

	z.setExpAndRound(e-fnorm(z.mant), sbit)
}

// ucmp returns -1, 0, or +1, depending on whether
// |x| < |y|, |x| == |y|, or |x| > |y|.
// x and y must have a non-empty mantissa and valid exponent.
func (x *Float) ucmp(y *Float) int {
	if debugFloat {
		validateBinaryOperands(x, y)
	}

	switch {
	case x.exp < y.exp:
		return -1
	case x.exp > y.exp:
		return +1
	}
	// x.exp == y.exp

	// compare mantissas
	i := len(x.mant)
	j := len(y.mant)
	for i > 0 || j > 0 {
		var xm, ym Word
		if i > 0 {
			i--
			xm = x.mant[i]
		}
		if j > 0 {
			j--
			ym = y.mant[j]
		}
		switch {
		case xm < ym:
			return -1
		case xm > ym:
			return +1
		}
	}

	return 0
}

// Handling of sign bit as defined by IEEE 754-2008, section 6.3:
//
// When neither the inputs nor result are NaN, the sign of a product or
// quotient is the exclusive OR of the operands’ signs; the sign of a sum,
// or of a difference x−y regarded as a sum x+(−y), differs from at most
// one of the addends’ signs; and the sign of the result of conversions,
// the quantize operation, the roundToIntegral operations, and the
// roundToIntegralExact (see 5.3.1) is the sign of the first or only operand.
// These rules shall apply even when operands or results are zero or infinite.
//
// When the sum of two operands with opposite signs (or the difference of
// two operands with like signs) is exactly zero, the sign of that sum (or
// difference) shall be +0 in all rounding-direction attributes except
// roundTowardNegative; under that attribute, the sign of an exact zero
// sum (or difference) shall be −0. However, x+x = x−(−x) retains the same
// sign as x even when x is zero.
//
// See also: https://play.golang.org/p/RtH3UCt5IH

// Add sets z to the rounded sum x+y and returns z. If z's precision is 0,
// it is changed to the larger of x's or y's precision before the operation.
// Rounding is performed according to z's precision and rounding mode; and
// z's accuracy reports the result error relative to the exact (not rounded)
// result. Add panics with [ErrNaN] if x and y are infinities with opposite
// signs. The value of z is undefined in that case.
func (z *Float) Add(x, y *Float) *Float {
	if debugFloat {
		x.validate()
		y.validate()
	}

	if z.prec == 0 {
		z.prec = max(x.prec, y.prec)
	}

	if x.form == finite && y.form == finite {
		// x + y (common case)

		// Below we set z.neg = x.neg, and when z aliases y this will
		// change the y operand's sign. This is fine, because if an
		// operand aliases the receiver it'll be overwritten, but we still
		// want the original x.neg and y.neg values when we evaluate
		// x.neg != y.neg, so we need to save y.neg before setting z.neg.
		yneg := y.neg

		z.neg = x.neg
		if x.neg == yneg {
			// x + y == x + y
			// (-x) + (-y) == -(x + y)
			z.uadd(x, y)
		} else {
			// x + (-y) == x - y == -(y - x)
			// (-x) + y == y - x == -(x - y)
			if x.ucmp(y) > 0 {
				z.usub(x, y)
			} else {
				z.neg = !z.neg
				z.usub(y, x)
			}
		}
		if z.form == zero && z.mode == ToNegativeInf && z.acc == Exact {
			z.neg = true
		}
		return z
	}

	if x.form == inf && y.form == inf && x.neg != y.neg {
		// +Inf + -Inf
		// -Inf + +Inf
		// value of z is undefined but make sure it's valid
		z.acc = Exact
		z.form = zero
		z.neg = false
		panic(ErrNaN{"addition of infinities with opposite signs"})
	}

	if x.form == zero && y.form == zero {
		// ±0 + ±0
		z.acc = Exact
		z.form = zero
		z.neg = x.neg && y.neg // -0 + -0 == -0
		return z
	}

	if x.form == inf || y.form == zero {
		// ±Inf + y
		// x + ±0
		return z.Set(x)
	}

	// ±0 + y
	// x + ±Inf
	return z.Set(y)
}

// Sub sets z to the rounded difference x-y and returns z.
// Precision, rounding, and accuracy reporting are as for [Float.Add].
// Sub panics with [ErrNaN] if x and y are infinities with equal
// signs. The value of z is undefined in that case.
func (z *Float) Sub(x, y *Float) *Float {
	if debugFloat {
		x.validate()
		y.validate()
	}

	if z.prec == 0 {
		z.prec = max(x.prec, y.prec)
	}

	if x.form == finite && y.form == finite {
		// x - y (common case)
		yneg := y.neg
		z.neg = x.neg
		if x.neg != yneg {
			// x - (-y) == x + y
			// (-x) - y == -(x + y)
			z.uadd(x, y)
		} else {
			// x - y == x - y == -(y - x)
			// (-x) - (-y) == y - x == -(x - y)
			if x.ucmp(y) > 0 {
				z.usub(x, y)
			} else {
				z.neg = !z.neg
				z.usub(y, x)
			}
		}
		if z.form == zero && z.mode == ToNegativeInf && z.acc == Exact {
			z.neg = true
		}
		return z
	}

	if x.form == inf && y.form == inf && x.neg == y.neg {
		// +Inf - +Inf
		// -Inf - -Inf
		// value of z is undefined but make sure it's valid
		z.acc = Exact
		z.form = zero
		z.neg = false
		panic(ErrNaN{"subtraction of infinities with equal signs"})
	}

	if x.form == zero && y.form == zero {
		// ±0 - ±0
		z.acc = Exact
		z.form = zero
		z.neg = x.neg && !y.neg // -0 - +0 == -0
		return z
	}

	if x.form == inf || y.form == zero {
		// ±Inf - y
		// x - ±0
		return z.Set(x)
	}

	// ±0 - y
	// x - ±Inf
	return z.Neg(y)
}

// Mul sets z to the rounded product x*y and returns z.
// Precision, rounding, and accuracy reporting are as for [Float.Add].
// Mul panics with [ErrNaN] if one operand is zero and the other
// operand an infinity. The value of z is undefined in that case.
func (z *Float) Mul(x, y *Float) *Float {
	if debugFloat {
		x.validate()
		y.validate()
	}

	if z.prec == 0 {
		z.prec = max(x.prec, y.prec)
	}

	z.neg = x.neg != y.neg

	if x.form == finite && y.form == finite {
		// x * y (common case)
		z.umul(x, y)
		return z
	}

	z.acc = Exact
	if x.form == zero && y.form == inf || x.form == inf && y.form == zero {
		// ±0 * ±Inf
		// ±Inf * ±0
		// value of z is undefined but make sure it's valid
		z.form = zero
		z.neg = false
		panic(ErrNaN{"multiplication of zero with infinity"})
	}

	if x.form == inf || y.form == inf {
		// ±Inf * y
		// x * ±Inf
		z.form = inf
		return z
	}

	// ±0 * y
	// x * ±0
	z.form = zero
	return z
}

// Quo sets z to the rounded quotient x/y and returns z.
// Precision, rounding, and accuracy reporting are as for [Float.Add].
// Quo panics with [ErrNaN] if both operands are zero or infinities.
// The value of z is undefined in that case.
func (z *Float) Quo(x, y *Float) *Float {
	if debugFloat {
		x.validate()
		y.validate()
	}

	if z.prec == 0 {
		z.prec = max(x.prec, y.prec)
	}

	z.neg = x.neg != y.neg

	if x.form == finite && y.form == finite {
		// x / y (common case)
		z.uquo(x, y)
		return z
	}

	z.acc = Exact
	if x.form == zero && y.form == zero || x.form == inf && y.form == inf {
		// ±0 / ±0
		// ±Inf / ±Inf
		// value of z is undefined but make sure it's valid
		z.form = zero
		z.neg = false
		panic(ErrNaN{"division of zero by zero or infinity by infinity"})
	}

	if x.form == zero || y.form == inf {
		// ±0 / y
		// x / ±Inf
		z.form = zero
		return z
	}

	// x / ±0
	// ±Inf / y
	z.form = inf
	return z
}

// Cmp compares x and y and returns:
//   - -1 if x < y;
//   - 0 if x == y (incl. -0 == 0, -Inf == -Inf, and +Inf == +Inf);
//   - +1 if x > y.
func (x *Float) Cmp(y *Float) int {
	if debugFloat {
		x.validate()
		y.validate()
	}

	mx := x.ord()
	my := y.ord()
	switch {
	case mx < my:
		return -1
	case mx > my:
		return +1
	}
	// mx == my

	// only if |mx| == 1 we have to compare the mantissae
	switch mx {
	case -1:
		return y.ucmp(x)
	case +1:
		return x.ucmp(y)
	}

	return 0
}

// ord classifies x and returns:
//
//	-2 if -Inf == x
//	-1 if -Inf < x < 0
//	 0 if x == 0 (signed or unsigned)
//	+1 if 0 < x < +Inf
//	+2 if x == +Inf
func (x *Float) ord() int {
	var m int
	switch x.form {
	case finite:
		m = 1
	case zero:
		return 0
	case inf:
		m = 2
	}
	if x.neg {
		m = -m
	}
	return m
}

```

// === FILE: references!/go/src/math/big/floatconv.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements string-to-Float conversion functions.

package big

import (
	"fmt"
	"io"
	"strings"
)

var floatZero Float

// SetString sets z to the value of s and returns z and a boolean indicating
// success. s must be a floating-point number of the same format as accepted
// by [Float.Parse], with base argument 0. The entire string (not just a prefix) must
// be valid for success. If the operation failed, the value of z is undefined
// but the returned value is nil.
func (z *Float) SetString(s string) (*Float, bool) {
	if f, _, err := z.Parse(s, 0); err == nil {
		return f, true
	}
	return nil, false
}

// scan is like Parse but reads the longest possible prefix representing a valid
// floating point number from an io.ByteScanner rather than a string. It serves
// as the implementation of Parse. It does not recognize ±Inf and does not expect
// EOF at the end.
func (z *Float) scan(r io.ByteScanner, base int) (f *Float, b int, err error) {
	prec := z.prec
	if prec == 0 {
		prec = 64
	}

	// A reasonable value in case of an error.
	z.form = zero

	// sign
	z.neg, err = scanSign(r)
	if err != nil {
		return
	}

	// mantissa
	var fcount int // fractional digit count; valid if <= 0
	z.mant, b, fcount, err = z.mant.scan(r, base, true)
	if err != nil {
		return
	}

	// exponent
	var exp int64
	var ebase int
	exp, ebase, err = scanExponent(r, true, base == 0)
	if err != nil {
		return
	}

	// special-case 0
	if len(z.mant) == 0 {
		z.prec = prec
		z.acc = Exact
		z.form = zero
		f = z
		return
	}
	// len(z.mant) > 0

	// The mantissa may have a radix point (fcount <= 0) and there
	// may be a nonzero exponent exp. The radix point amounts to a
	// division by b**(-fcount). An exponent means multiplication by
	// ebase**exp. Finally, mantissa normalization (shift left) requires
	// a correcting multiplication by 2**(-shiftcount). Multiplications
	// are commutative, so we can apply them in any order as long as there
	// is no loss of precision. We only have powers of 2 and 10, and
	// we split powers of 10 into the product of the same powers of
	// 2 and 5. This reduces the size of the multiplication factor
	// needed for base-10 exponents.

	// normalize mantissa and determine initial exponent contributions
	exp2 := int64(len(z.mant))*_W - fnorm(z.mant)
	exp5 := int64(0)

	// determine binary or decimal exponent contribution of radix point
	if fcount < 0 {
		// The mantissa has a radix point ddd.dddd; and
		// -fcount is the number of digits to the right
		// of '.'. Adjust relevant exponent accordingly.
		d := int64(fcount)
		switch b {
		case 10:
			exp5 = d
			fallthrough // 10**e == 5**e * 2**e
		case 2:
			exp2 += d
		case 8:
			exp2 += d * 3 // octal digits are 3 bits each
		case 16:
			exp2 += d * 4 // hexadecimal digits are 4 bits each
		default:
			panic("unexpected mantissa base")
		}
		// fcount consumed - not needed anymore
	}

	// take actual exponent into account
	switch ebase {
	case 10:
		exp5 += exp
		fallthrough // see fallthrough above
	case 2:
		exp2 += exp
	default:
		panic("unexpected exponent base")
	}
	// exp consumed - not needed anymore

	// apply 2**exp2
	if MinExp <= exp2 && exp2 <= MaxExp {
		z.prec = prec
		z.form = finite
		z.exp = int32(exp2)
		f = z
	} else {
		err = fmt.Errorf("exponent overflow")
		return
	}

	if exp5 == 0 {
		// no decimal exponent contribution
		z.round(0)
		return
	}
	// exp5 != 0

	// apply 5**exp5
	p := new(Float).SetPrec(z.Prec() + 64) // use more bits for p -- TODO(gri) what is the right number?
	if exp5 < 0 {
		z.Quo(z, p.pow5(uint64(-exp5)))
	} else {
		z.Mul(z, p.pow5(uint64(exp5)))
	}

	return
}

// These powers of 5 fit into a uint64.
//
//	for p, q := uint64(0), uint64(1); p < q; p, q = q, q*5 {
//		fmt.Println(q)
//	}
var pow5tab = [...]uint64{
	1,
	5,
	25,
	125,
	625,
	3125,
	15625,
	78125,
	390625,
	1953125,
	9765625,
	48828125,
	244140625,
	1220703125,
	6103515625,
	30517578125,
	152587890625,
	762939453125,
	3814697265625,
	19073486328125,
	95367431640625,
	476837158203125,
	2384185791015625,
	11920928955078125,
	59604644775390625,
	298023223876953125,
	1490116119384765625,
	7450580596923828125,
}

// pow5 sets z to 5**n and returns z.
// n must not be negative.
func (z *Float) pow5(n uint64) *Float {
	const m = uint64(len(pow5tab) - 1)
	if n <= m {
		return z.SetUint64(pow5tab[n])
	}
	// n > m

	z.SetUint64(pow5tab[m])
	n -= m

	// use more bits for f than for z
	// TODO(gri) what is the right number?
	f := new(Float).SetPrec(z.Prec() + 64).SetUint64(5)

	for n > 0 {
		if n&1 != 0 {
			z.Mul(z, f)
		}
		f.Mul(f, f)
		n >>= 1
	}

	return z
}

// Parse parses s which must contain a text representation of a floating-
// point number with a mantissa in the given conversion base (the exponent
// is always a decimal number), or a string representing an infinite value.
//
// For base 0, an underscore character “_” may appear between a base
// prefix and an adjacent digit, and between successive digits; such
// underscores do not change the value of the number, or the returned
// digit count. Incorrect placement of underscores is reported as an
// error if there are no other errors. If base != 0, underscores are
// not recognized and thus terminate scanning like any other character
// that is not a valid radix point or digit.
//
// It sets z to the (possibly rounded) value of the corresponding floating-
// point value, and returns z, the actual base b, and an error err, if any.
// The entire string (not just a prefix) must be consumed for success.
// If z's precision is 0, it is changed to 64 before rounding takes effect.
// The number must be of the form:
//
//	number    = [ sign ] ( float | "inf" | "Inf" ) .
//	sign      = "+" | "-" .
//	float     = ( mantissa | prefix pmantissa ) [ exponent ] .
//	prefix    = "0" [ "b" | "B" | "o" | "O" | "x" | "X" ] .
//	mantissa  = digits "." [ digits ] | digits | "." digits .
//	pmantissa = [ "_" ] digits "." [ digits ] | [ "_" ] digits | "." digits .
//	exponent  = ( "e" | "E" | "p" | "P" ) [ sign ] digits .
//	digits    = digit { [ "_" ] digit } .
//	digit     = "0" ... "9" | "a" ... "z" | "A" ... "Z" .
//
// The base argument must be 0, 2, 8, 10, or 16. Providing an invalid base
// argument will lead to a run-time panic.
//
// For base 0, the number prefix determines the actual base: A prefix of
// “0b” or “0B” selects base 2, “0o” or “0O” selects base 8, and
// “0x” or “0X” selects base 16. Otherwise, the actual base is 10 and
// no prefix is accepted. The octal prefix "0" is not supported (a leading
// "0" is simply considered a "0").
//
// A "p" or "P" exponent indicates a base 2 (rather than base 10) exponent;
// for instance, "0x1.fffffffffffffp1023" (using base 0) represents the
// maximum float64 value. For hexadecimal mantissae, the exponent character
// must be one of 'p' or 'P', if present (an "e" or "E" exponent indicator
// cannot be distinguished from a mantissa digit).
//
// The returned *Float f is nil and the value of z is valid but not
// defined if an error is reported.
func (z *Float) Parse(s string, base int) (f *Float, b int, err error) {
	// scan doesn't handle ±Inf
	if len(s) == 3 && (s == "Inf" || s == "inf") {
		f = z.SetInf(false)
		return
	}
	if len(s) == 4 && (s[0] == '+' || s[0] == '-') && (s[1:] == "Inf" || s[1:] == "inf") {
		f = z.SetInf(s[0] == '-')
		return
	}

	r := strings.NewReader(s)
	if f, b, err = z.scan(r, base); err != nil {
		return
	}

	// entire string must have been consumed
	if ch, err2 := r.ReadByte(); err2 == nil {
		err = fmt.Errorf("expected end of string, found %q", ch)
	} else if err2 != io.EOF {
		err = err2
	}

	return
}

// ParseFloat is like f.Parse(s, base) with f set to the given precision
// and rounding mode.
func ParseFloat(s string, base int, prec uint, mode RoundingMode) (f *Float, b int, err error) {
	return new(Float).SetPrec(prec).SetMode(mode).Parse(s, base)
}

var _ fmt.Scanner = (*Float)(nil) // *Float must implement fmt.Scanner

// Scan is a support routine for [fmt.Scanner]; it sets z to the value of
// the scanned number. It accepts formats whose verbs are supported by
// [fmt.Scan] for floating point values, which are:
// 'b' (binary), 'e', 'E', 'f', 'F', 'g' and 'G'.
// Scan doesn't handle ±Inf.
func (z *Float) Scan(s fmt.ScanState, ch rune) error {
	s.SkipSpace()
	_, _, err := z.scan(byteReader{s}, 0)
	return err
}

```

// === FILE: references!/go/src/math/big/floatmarsh.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements encoding/decoding of Floats.

package big

import (
	"errors"
	"fmt"
	"internal/byteorder"
)

// Gob codec version. Permits backward-compatible changes to the encoding.
const floatGobVersion byte = 1

// GobEncode implements the [encoding/gob.GobEncoder] interface.
// The [Float] value and all its attributes (precision,
// rounding mode, accuracy) are marshaled.
func (x *Float) GobEncode() ([]byte, error) {
	if x == nil {
		return nil, nil
	}

	// determine max. space (bytes) required for encoding
	sz := 1 + 1 + 4 // version + mode|acc|form|neg (3+2+2+1bit) + prec
	n := 0          // number of mantissa words
	if x.form == finite {
		// add space for mantissa and exponent
		n = int((x.prec + (_W - 1)) / _W) // required mantissa length in words for given precision
		// actual mantissa slice could be shorter (trailing 0's) or longer (unused bits):
		// - if shorter, only encode the words present
		// - if longer, cut off unused words when encoding in bytes
		//   (in practice, this should never happen since rounding
		//   takes care of it, but be safe and do it always)
		if len(x.mant) < n {
			n = len(x.mant)
		}
		// len(x.mant) >= n
		sz += 4 + n*_S // exp + mant
	}
	buf := make([]byte, sz)

	buf[0] = floatGobVersion
	b := byte(x.mode&7)<<5 | byte((x.acc+1)&3)<<3 | byte(x.form&3)<<1
	if x.neg {
		b |= 1
	}
	buf[1] = b
	byteorder.BEPutUint32(buf[2:], x.prec)

	if x.form == finite {
		byteorder.BEPutUint32(buf[6:], uint32(x.exp))
		x.mant[len(x.mant)-n:].bytes(buf[10:]) // cut off unused trailing words
	}

	return buf, nil
}

// GobDecode implements the [encoding/gob.GobDecoder] interface.
// The result is rounded per the precision and rounding mode of
// z unless z's precision is 0, in which case z is set exactly
// to the decoded value.
func (z *Float) GobDecode(buf []byte) error {
	if len(buf) == 0 {
		// Other side sent a nil or default value.
		*z = Float{}
		return nil
	}
	if len(buf) < 6 {
		return errors.New("Float.GobDecode: buffer too small")
	}

	if buf[0] != floatGobVersion {
		return fmt.Errorf("Float.GobDecode: encoding version %d not supported", buf[0])
	}

	oldPrec := z.prec
	oldMode := z.mode

	b := buf[1]
	z.mode = RoundingMode((b >> 5) & 7)
	z.acc = Accuracy((b>>3)&3) - 1
	z.form = form((b >> 1) & 3)
	z.neg = b&1 != 0
	z.prec = byteorder.BEUint32(buf[2:])

	if z.form == finite {
		if len(buf) < 10 {
			return errors.New("Float.GobDecode: buffer too small for finite form float")
		}
		z.exp = int32(byteorder.BEUint32(buf[6:]))
		z.mant = z.mant.setBytes(buf[10:])
	}

	if oldPrec != 0 {
		z.mode = oldMode
		z.SetPrec(uint(oldPrec))
	}

	if msg := z.validate0(); msg != "" {
		return errors.New("Float.GobDecode: " + msg)
	}

	return nil
}

// AppendText implements the [encoding.TextAppender] interface.
// Only the [Float] value is marshaled (in full precision), other
// attributes such as precision or accuracy are ignored.
func (x *Float) AppendText(b []byte) ([]byte, error) {
	if x == nil {
		return append(b, "<nil>"...), nil
	}
	return x.Append(b, 'g', -1), nil
}

// MarshalText implements the [encoding.TextMarshaler] interface.
// Only the [Float] value is marshaled (in full precision), other
// attributes such as precision or accuracy are ignored.
func (x *Float) MarshalText() (text []byte, err error) {
	return x.AppendText(nil)
}

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
// The result is rounded per the precision and rounding mode of z.
// If z's precision is 0, it is changed to 64 before rounding takes
// effect.
func (z *Float) UnmarshalText(text []byte) error {
	// TODO(gri): get rid of the []byte/string conversion
	_, _, err := z.Parse(string(text), 0)
	if err != nil {
		err = fmt.Errorf("math/big: cannot unmarshal %q into a *big.Float (%v)", text, err)
	}
	return err
}

```

// === FILE: references!/go/src/math/big/ftoa.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements Float-to-string conversion functions.
// It is closely following the corresponding implementation
// in strconv/ftoa.go, but modified and simplified for Float.

package big

import (
	"bytes"
	"fmt"
	"strconv"
)

// Text converts the floating-point number x to a string according
// to the given format and precision prec. The format is one of:
//
//	'e'	-d.dddde±dd, decimal exponent, at least two (possibly 0) exponent digits
//	'E'	-d.ddddE±dd, decimal exponent, at least two (possibly 0) exponent digits
//	'f'	-ddddd.dddd, no exponent
//	'g'	like 'e' for large exponents, like 'f' otherwise
//	'G'	like 'E' for large exponents, like 'f' otherwise
//	'x'	-0xd.dddddp±dd, hexadecimal mantissa, decimal power of two exponent
//	'p'	-0x.dddp±dd, hexadecimal mantissa, decimal power of two exponent (non-standard)
//	'b'	-ddddddp±dd, decimal mantissa, decimal power of two exponent (non-standard)
//
// For the power-of-two exponent formats, the mantissa is printed in normalized form:
//
//	'x'	hexadecimal mantissa in [1, 2), or 0
//	'p'	hexadecimal mantissa in [½, 1), or 0
//	'b'	decimal integer mantissa using x.Prec() bits, or 0
//
// Note that the 'x' form is the one used by most other languages and libraries.
//
// If format is a different character, Text returns a "%" followed by the
// unrecognized format character.
//
// The precision prec controls the number of digits (excluding the exponent)
// printed by the 'e', 'E', 'f', 'g', 'G', and 'x' formats.
// For 'e', 'E', 'f', and 'x', it is the number of digits after the decimal point.
// For 'g' and 'G' it is the total number of digits. A negative precision selects
// the smallest number of decimal digits necessary to identify the value x uniquely
// using x.Prec() mantissa bits.
// The prec value is ignored for the 'b' and 'p' formats.
//
// Note that Text may return a different result than strconv.FormatFloat for
// corresponding arguments if the matching float32 or float64 number provided
// to strconv.FormatFloat is a denormalized number.
func (x *Float) Text(format byte, prec int) string {
	cap := 10 // TODO(gri) determine a good/better value here
	if prec > 0 {
		cap += prec
	}
	return string(x.Append(make([]byte, 0, cap), format, prec))
}

// String formats x like x.Text('g', 10).
// (String must be called explicitly, [Float.Format] does not support %s verb.)
func (x *Float) String() string {
	return x.Text('g', 10)
}

// Append appends to buf the string form of the floating-point number x,
// as generated by x.Text, and returns the extended buffer.
func (x *Float) Append(buf []byte, fmt byte, prec int) []byte {
	// sign
	if x.neg {
		buf = append(buf, '-')
	}

	// Inf
	if x.form == inf {
		if !x.neg {
			buf = append(buf, '+')
		}
		return append(buf, "Inf"...)
	}

	// pick off easy formats
	switch fmt {
	case 'b':
		return x.fmtB(buf)
	case 'p':
		return x.fmtP(buf)
	case 'x':
		return x.fmtX(buf, prec)
	}

	// Algorithm:
	//   1) convert Float to multiprecision decimal
	//   2) round to desired precision
	//   3) read digits out and format

	// 1) convert Float to multiprecision decimal
	var d decimal // == 0.0
	if x.form == finite {
		// x != 0
		d.init(x.mant, int(x.exp)-x.mant.bitLen())
	}

	// 2) round to desired precision
	shortest := false
	if prec < 0 {
		shortest = true
		roundShortest(&d, x)
		// Precision for shortest representation mode.
		switch fmt {
		case 'e', 'E':
			prec = len(d.mant) - 1
		case 'f':
			prec = max(len(d.mant)-d.exp, 0)
		case 'g', 'G':
			prec = len(d.mant)
		}
	} else {
		// round appropriately
		switch fmt {
		case 'e', 'E':
			// one digit before and number of digits after decimal point
			d.round(1 + prec)
		case 'f':
			// number of digits before and after decimal point
			d.round(d.exp + prec)
		case 'g', 'G':
			if prec == 0 {
				prec = 1
			}
			d.round(prec)
		}
	}

	// 3) read digits out and format
	switch fmt {
	case 'e', 'E':
		return fmtE(buf, fmt, prec, d)
	case 'f':
		return fmtF(buf, prec, d)
	case 'g', 'G':
		// trim trailing fractional zeros in %e format
		eprec := prec
		if eprec > len(d.mant) && len(d.mant) >= d.exp {
			eprec = len(d.mant)
		}
		// %e is used if the exponent from the conversion
		// is less than -4 or greater than or equal to the precision.
		// If precision was the shortest possible, use eprec = 6 for
		// this decision.
		if shortest {
			eprec = 6
		}
		exp := d.exp - 1
		if exp < -4 || exp >= eprec {
			if prec > len(d.mant) {
				prec = len(d.mant)
			}
			return fmtE(buf, fmt+'e'-'g', prec-1, d)
		}
		if prec > d.exp {
			prec = len(d.mant)
		}
		return fmtF(buf, max(prec-d.exp, 0), d)
	}

	// unknown format
	if x.neg {
		buf = buf[:len(buf)-1] // sign was added prematurely - remove it again
	}
	return append(buf, '%', fmt)
}

func roundShortest(d *decimal, x *Float) {
	// if the mantissa is zero, the number is zero - stop now
	if len(d.mant) == 0 {
		return
	}

	// Approach: All numbers in the interval [x - 1/2ulp, x + 1/2ulp]
	// (possibly exclusive) round to x for the given precision of x.
	// Compute the lower and upper bound in decimal form and find the
	// shortest decimal number d such that lower <= d <= upper.

	// TODO(gri) strconv/ftoa.do describes a shortcut in some cases.
	// See if we can use it (in adjusted form) here as well.

	// 1) Compute normalized mantissa mant and exponent exp for x such
	// that the lsb of mant corresponds to 1/2 ulp for the precision of
	// x (i.e., for mant we want x.prec + 1 bits).
	mant := nat(nil).set(x.mant)
	exp := int(x.exp) - mant.bitLen()
	s := mant.bitLen() - int(x.prec+1)
	switch {
	case s < 0:
		mant = mant.lsh(mant, uint(-s))
	case s > 0:
		mant = mant.rsh(mant, uint(+s))
	}
	exp += s
	// x = mant * 2**exp with lsb(mant) == 1/2 ulp of x.prec

	// 2) Compute lower bound by subtracting 1/2 ulp.
	var lower decimal
	var tmp nat
	lower.init(tmp.sub(mant, natOne), exp)

	// 3) Compute upper bound by adding 1/2 ulp.
	var upper decimal
	upper.init(tmp.add(mant, natOne), exp)

	// The upper and lower bounds are possible outputs only if
	// the original mantissa is even, so that ToNearestEven rounding
	// would round to the original mantissa and not the neighbors.
	inclusive := mant[0]&2 == 0 // test bit 1 since original mantissa was shifted by 1

	// Now we can figure out the minimum number of digits required.
	// Walk along until d has distinguished itself from upper and lower.
	for i, m := range d.mant {
		l := lower.at(i)
		u := upper.at(i)

		// Okay to round down (truncate) if lower has a different digit
		// or if lower is inclusive and is exactly the result of rounding
		// down (i.e., and we have reached the final digit of lower).
		okdown := l != m || inclusive && i+1 == len(lower.mant)

		// Okay to round up if upper has a different digit and either upper
		// is inclusive or upper is bigger than the result of rounding up.
		okup := m != u && (inclusive || m+1 < u || i+1 < len(upper.mant))

		// If it's okay to do either, then round to the nearest one.
		// If it's okay to do only one, do it.
		switch {
		case okdown && okup:
			d.round(i + 1)
			return
		case okdown:
			d.roundDown(i + 1)
			return
		case okup:
			d.roundUp(i + 1)
			return
		}
	}
}

// %e: d.ddddde±dd
func fmtE(buf []byte, fmt byte, prec int, d decimal) []byte {
	// first digit
	ch := byte('0')
	if len(d.mant) > 0 {
		ch = d.mant[0]
	}
	buf = append(buf, ch)

	// .moredigits
	if prec > 0 {
		buf = append(buf, '.')
		i := 1
		m := min(len(d.mant), prec+1)
		if i < m {
			buf = append(buf, d.mant[i:m]...)
			i = m
		}
		for ; i <= prec; i++ {
			buf = append(buf, '0')
		}
	}

	// e±
	buf = append(buf, fmt)
	var exp int64
	if len(d.mant) > 0 {
		exp = int64(d.exp) - 1 // -1 because first digit was printed before '.'
	}
	if exp < 0 {
		ch = '-'
		exp = -exp
	} else {
		ch = '+'
	}
	buf = append(buf, ch)

	// dd...d
	if exp < 10 {
		buf = append(buf, '0') // at least 2 exponent digits
	}
	return strconv.AppendInt(buf, exp, 10)
}

// %f: ddddddd.ddddd
func fmtF(buf []byte, prec int, d decimal) []byte {
	// integer, padded with zeros as needed
	if d.exp > 0 {
		m := min(len(d.mant), d.exp)
		buf = append(buf, d.mant[:m]...)
		for ; m < d.exp; m++ {
			buf = append(buf, '0')
		}
	} else {
		buf = append(buf, '0')
	}

	// fraction
	if prec > 0 {
		buf = append(buf, '.')
		for i := 0; i < prec; i++ {
			buf = append(buf, d.at(d.exp+i))
		}
	}

	return buf
}

// fmtB appends the string of x in the format mantissa "p" exponent
// with a decimal mantissa and a binary exponent, or "0" if x is zero,
// and returns the extended buffer.
// The mantissa is normalized such that is uses x.Prec() bits in binary
// representation.
// The sign of x is ignored, and x must not be an Inf.
// (The caller handles Inf before invoking fmtB.)
func (x *Float) fmtB(buf []byte) []byte {
	if x.form == zero {
		return append(buf, '0')
	}

	if debugFloat && x.form != finite {
		panic("non-finite float")
	}
	// x != 0

	// adjust mantissa to use exactly x.prec bits
	m := x.mant
	switch w := uint32(len(x.mant)) * _W; {
	case w < x.prec:
		m = nat(nil).lsh(m, uint(x.prec-w))
	case w > x.prec:
		m = nat(nil).rsh(m, uint(w-x.prec))
	}

	buf = append(buf, m.utoa(10)...)
	buf = append(buf, 'p')
	e := int64(x.exp) - int64(x.prec)
	if e >= 0 {
		buf = append(buf, '+')
	}
	return strconv.AppendInt(buf, e, 10)
}

// fmtX appends the string of x in the format "0x1." mantissa "p" exponent
// with a hexadecimal mantissa and a binary exponent, or "0x0p0" if x is zero,
// and returns the extended buffer.
// A non-zero mantissa is normalized such that 1.0 <= mantissa < 2.0.
// The sign of x is ignored, and x must not be an Inf.
// (The caller handles Inf before invoking fmtX.)
func (x *Float) fmtX(buf []byte, prec int) []byte {
	if x.form == zero {
		buf = append(buf, "0x0"...)
		if prec > 0 {
			buf = append(buf, '.')
			for i := 0; i < prec; i++ {
				buf = append(buf, '0')
			}
		}
		buf = append(buf, "p+00"...)
		return buf
	}

	if debugFloat && x.form != finite {
		panic("non-finite float")
	}

	// round mantissa to n bits
	var n uint
	if prec < 0 {
		n = 1 + (x.MinPrec()-1+3)/4*4 // round MinPrec up to 1 mod 4
	} else {
		n = 1 + 4*uint(prec)
	}
	// n%4 == 1
	x = new(Float).SetPrec(n).SetMode(x.mode).Set(x)

	// adjust mantissa to use exactly n bits
	m := x.mant
	switch w := uint(len(x.mant)) * _W; {
	case w < n:
		m = nat(nil).lsh(m, n-w)
	case w > n:
		m = nat(nil).rsh(m, w-n)
	}
	exp64 := int64(x.exp) - 1 // avoid wrap-around

	hm := m.utoa(16)
	if debugFloat && hm[0] != '1' {
		panic("incorrect mantissa: " + string(hm))
	}
	buf = append(buf, "0x1"...)
	if len(hm) > 1 {
		buf = append(buf, '.')
		buf = append(buf, hm[1:]...)
	}

	buf = append(buf, 'p')
	if exp64 >= 0 {
		buf = append(buf, '+')
	} else {
		exp64 = -exp64
		buf = append(buf, '-')
	}
	// Force at least two exponent digits, to match fmt.
	if exp64 < 10 {
		buf = append(buf, '0')
	}
	return strconv.AppendInt(buf, exp64, 10)
}

// fmtP appends the string of x in the format "0x." mantissa "p" exponent
// with a hexadecimal mantissa and a binary exponent, or "0" if x is zero,
// and returns the extended buffer.
// The mantissa is normalized such that 0.5 <= 0.mantissa < 1.0.
// The sign of x is ignored, and x must not be an Inf.
// (The caller handles Inf before invoking fmtP.)
func (x *Float) fmtP(buf []byte) []byte {
	if x.form == zero {
		return append(buf, '0')
	}

	if debugFloat && x.form != finite {
		panic("non-finite float")
	}
	// x != 0

	// remove trailing 0 words early
	// (no need to convert to hex 0's and trim later)
	m := x.mant
	i := 0
	for i < len(m) && m[i] == 0 {
		i++
	}
	m = m[i:]

	buf = append(buf, "0x."...)
	buf = append(buf, bytes.TrimRight(m.utoa(16), "0")...)
	buf = append(buf, 'p')
	if x.exp >= 0 {
		buf = append(buf, '+')
	}
	return strconv.AppendInt(buf, int64(x.exp), 10)
}

var _ fmt.Formatter = &floatZero // *Float must implement fmt.Formatter

// Format implements [fmt.Formatter]. It accepts all the regular
// formats for floating-point numbers ('b', 'e', 'E', 'f', 'F',
// 'g', 'G', 'x') as well as 'p' and 'v'. See (*Float).Text for the
// interpretation of 'p'. The 'v' format is handled like 'g'.
// Format also supports specification of the minimum precision
// in digits, the output field width, as well as the format flags
// '+' and ' ' for sign control, '0' for space or zero padding,
// and '-' for left or right justification. See the fmt package
// for details.
func (x *Float) Format(s fmt.State, format rune) {
	prec, hasPrec := s.Precision()
	if !hasPrec {
		prec = 6 // default precision for 'e', 'f'
	}

	switch format {
	case 'e', 'E', 'f', 'b', 'p', 'x':
		// nothing to do
	case 'F':
		// (*Float).Text doesn't support 'F'; handle like 'f'
		format = 'f'
	case 'v':
		// handle like 'g'
		format = 'g'
		fallthrough
	case 'g', 'G':
		if !hasPrec {
			prec = -1 // default precision for 'g', 'G'
		}
	default:
		fmt.Fprintf(s, "%%!%c(*big.Float=%s)", format, x.String())
		return
	}
	var buf []byte
	buf = x.Append(buf, byte(format), prec)
	if len(buf) == 0 {
		buf = []byte("?") // should never happen, but don't crash
	}
	// len(buf) > 0

	var sign string
	switch {
	case buf[0] == '-':
		sign = "-"
		buf = buf[1:]
	case buf[0] == '+':
		// +Inf
		sign = "+"
		if s.Flag(' ') {
			sign = " "
		}
		buf = buf[1:]
	case s.Flag('+'):
		sign = "+"
	case s.Flag(' '):
		sign = " "
	}

	var padding int
	if width, hasWidth := s.Width(); hasWidth && width > len(sign)+len(buf) {
		padding = width - len(sign) - len(buf)
	}

	switch {
	case s.Flag('0') && !x.IsInf():
		// 0-padding on left
		writeMultiple(s, sign, 1)
		writeMultiple(s, "0", padding)
		s.Write(buf)
	case s.Flag('-'):
		// padding on right
		writeMultiple(s, sign, 1)
		s.Write(buf)
		writeMultiple(s, " ", padding)
	default:
		// padding on left
		writeMultiple(s, " ", padding)
		writeMultiple(s, sign, 1)
		s.Write(buf)
	}
}

```

// === FILE: references!/go/src/math/big/int.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements signed multi-precision integers.

package big

import (
	"fmt"
	"io"
	"math/rand"
	"strings"
)

// An Int represents a signed multi-precision integer.
// The zero value for an Int represents the value 0.
//
// Operations always take pointer arguments (*Int) rather
// than Int values, and each unique Int value requires
// its own unique *Int pointer. To "copy" an Int value,
// an existing (or newly allocated) Int must be set to
// a new value using the [Int.Set] method; shallow copies
// of Ints are not supported and may lead to errors.
//
// Note that methods may leak the Int's value through timing side-channels.
// Because of this and because of the scope and complexity of the
// implementation, Int is not well-suited to implement cryptographic operations.
// The standard library avoids exposing non-trivial Int methods to
// attacker-controlled inputs and the determination of whether a bug in math/big
// is considered a security vulnerability might depend on the impact on the
// standard library.
type Int struct {
	neg bool // sign
	abs nat  // absolute value of the integer
}

var intOne = &Int{false, natOne}

// Sign returns:
//   - -1 if x < 0;
//   - 0 if x == 0;
//   - +1 if x > 0.
func (x *Int) Sign() int {
	// This function is used in cryptographic operations. It must not leak
	// anything but the Int's sign and bit size through side-channels. Any
	// changes must be reviewed by a security expert.
	if len(x.abs) == 0 {
		return 0
	}
	if x.neg {
		return -1
	}
	return 1
}

// SetInt64 sets z to x and returns z.
func (z *Int) SetInt64(x int64) *Int {
	neg := false
	if x < 0 {
		neg = true
		x = -x
	}
	z.abs = z.abs.setUint64(uint64(x))
	z.neg = neg
	return z
}

// SetUint64 sets z to x and returns z.
func (z *Int) SetUint64(x uint64) *Int {
	z.abs = z.abs.setUint64(x)
	z.neg = false
	return z
}

// NewInt allocates and returns a new [Int] set to x.
func NewInt(x int64) *Int {
	// This code is arranged to be inlineable and produce
	// zero allocations when inlined. See issue 29951.
	u := uint64(x)
	if x < 0 {
		u = -u
	}
	var abs []Word
	if x == 0 {
	} else if _W == 32 && u>>32 != 0 {
		abs = []Word{Word(u), Word(u >> 32)}
	} else {
		abs = []Word{Word(u)}
	}
	return &Int{neg: x < 0, abs: abs}
}

// Set sets z to x and returns z.
func (z *Int) Set(x *Int) *Int {
	if z != x {
		z.abs = z.abs.set(x.abs)
		z.neg = x.neg
	}
	return z
}

// Bits provides raw (unchecked but fast) access to x by returning its
// absolute value as a little-endian [Word] slice. The result and x share
// the same underlying array.
// Bits is intended to support implementation of missing low-level [Int]
// functionality outside this package; it should be avoided otherwise.
func (x *Int) Bits() []Word {
	// This function is used in cryptographic operations. It must not leak
	// anything but the Int's sign and bit size through side-channels. Any
	// changes must be reviewed by a security expert.
	return x.abs
}

// SetBits provides raw (unchecked but fast) access to z by setting its
// value to abs, interpreted as a little-endian [Word] slice, and returning
// z. The result and abs share the same underlying array.
// SetBits is intended to support implementation of missing low-level [Int]
// functionality outside this package; it should be avoided otherwise.
func (z *Int) SetBits(abs []Word) *Int {
	z.abs = nat(abs).norm()
	z.neg = false
	return z
}

// Abs sets z to |x| (the absolute value of x) and returns z.
func (z *Int) Abs(x *Int) *Int {
	z.Set(x)
	z.neg = false
	return z
}

// Neg sets z to -x and returns z.
func (z *Int) Neg(x *Int) *Int {
	z.Set(x)
	z.neg = len(z.abs) > 0 && !z.neg // 0 has no sign
	return z
}

// Add sets z to the sum x+y and returns z.
func (z *Int) Add(x, y *Int) *Int {
	neg := x.neg
	if x.neg == y.neg {
		// x + y == x + y
		// (-x) + (-y) == -(x + y)
		z.abs = z.abs.add(x.abs, y.abs)
	} else {
		// x + (-y) == x - y == -(y - x)
		// (-x) + y == y - x == -(x - y)
		if x.abs.cmp(y.abs) >= 0 {
			z.abs = z.abs.sub(x.abs, y.abs)
		} else {
			neg = !neg
			z.abs = z.abs.sub(y.abs, x.abs)
		}
	}
	z.neg = len(z.abs) > 0 && neg // 0 has no sign
	return z
}

// Sub sets z to the difference x-y and returns z.
func (z *Int) Sub(x, y *Int) *Int {
	neg := x.neg
	if x.neg != y.neg {
		// x - (-y) == x + y
		// (-x) - y == -(x + y)
		z.abs = z.abs.add(x.abs, y.abs)
	} else {
		// x - y == x - y == -(y - x)
		// (-x) - (-y) == y - x == -(x - y)
		if x.abs.cmp(y.abs) >= 0 {
			z.abs = z.abs.sub(x.abs, y.abs)
		} else {
			neg = !neg
			z.abs = z.abs.sub(y.abs, x.abs)
		}
	}
	z.neg = len(z.abs) > 0 && neg // 0 has no sign
	return z
}

// Mul sets z to the product x*y and returns z.
func (z *Int) Mul(x, y *Int) *Int {
	z.mul(nil, x, y)
	return z
}

// mul is like Mul but takes an explicit stack to use, for internal use.
// It does not return a *Int because doing so makes the stack-allocated Ints
// used in natmul.go escape to the heap (even though the result is unused).
func (z *Int) mul(stk *stack, x, y *Int) {
	// x * y == x * y
	// x * (-y) == -(x * y)
	// (-x) * y == -(x * y)
	// (-x) * (-y) == x * y
	if x == y {
		z.abs = z.abs.sqr(stk, x.abs)
		z.neg = false
		return
	}
	z.abs = z.abs.mul(stk, x.abs, y.abs)
	z.neg = len(z.abs) > 0 && x.neg != y.neg // 0 has no sign
}

// MulRange sets z to the product of all integers
// in the range [a, b] inclusively and returns z.
// If a > b (empty range), the result is 1.
func (z *Int) MulRange(a, b int64) *Int {
	switch {
	case a > b:
		return z.SetInt64(1) // empty range
	case a <= 0 && b >= 0:
		return z.SetInt64(0) // range includes 0
	}
	// a <= b && (b < 0 || a > 0)

	neg := false
	if a < 0 {
		neg = (b-a)&1 == 0
		a, b = -b, -a
	}

	z.abs = z.abs.mulRange(nil, uint64(a), uint64(b))
	z.neg = neg
	return z
}

// Binomial sets z to the binomial coefficient C(n, k) and returns z.
func (z *Int) Binomial(n, k int64) *Int {
	if k > n || k < 0 {
		return z.SetInt64(0)
	}
	// reduce the number of multiplications by reducing k
	if k > n-k {
		k = n - k // C(n, k) == C(n, n-k)
	}
	// C(n, k) == n * (n-1) * ... * (n-k+1) / k * (k-1) * ... * 1
	//         == n * (n-1) * ... * (n-k+1) / 1 * (1+1) * ... * k
	//
	// Using the multiplicative formula produces smaller values
	// at each step, requiring fewer allocations and computations:
	//
	// z = 1
	// for i := 0; i < k; i = i+1 {
	//     z *= n-i
	//     z /= i+1
	// }
	//
	// finally to avoid computing i+1 twice per loop:
	//
	// z = 1
	// i := 0
	// for i < k {
	//     z *= n-i
	//     i++
	//     z /= i
	// }
	var N, K, i, t Int
	N.SetInt64(n)
	K.SetInt64(k)
	z.Set(intOne)
	for i.Cmp(&K) < 0 {
		z.Mul(z, t.Sub(&N, &i))
		i.Add(&i, intOne)
		z.Quo(z, &i)
	}
	return z
}

// Quo sets z to the quotient x/y for y != 0 and returns z.
// If y == 0, a division-by-zero run-time panic occurs.
// Quo implements truncated division (like Go); see [Int.QuoRem] for more details.
func (z *Int) Quo(x, y *Int) *Int {
	z.abs, _ = z.abs.div(nil, nil, x.abs, y.abs)
	z.neg = len(z.abs) > 0 && x.neg != y.neg // 0 has no sign
	return z
}

// Rem sets z to the remainder x%y for y != 0 and returns z.
// If y == 0, a division-by-zero run-time panic occurs.
// Rem implements truncated modulus (like Go); see [Int.QuoRem] for more details.
func (z *Int) Rem(x, y *Int) *Int {
	_, z.abs = nat(nil).div(nil, z.abs, x.abs, y.abs)
	z.neg = len(z.abs) > 0 && x.neg // 0 has no sign
	return z
}

// QuoRem sets z to the quotient x/y and r to the remainder x%y
// and returns the pair (z, r) for y != 0.
// If y == 0, a division-by-zero run-time panic occurs.
//
// QuoRem implements T-division and modulus (like Go):
//
//	q = x/y      with the result truncated to zero
//	r = x - y*q
//
// (See Daan Leijen, “Division and Modulus for Computer Scientists”.)
// See [Int.DivMod] for Euclidean division and modulus (unlike Go).
func (z *Int) QuoRem(x, y, r *Int) (*Int, *Int) {
	z.abs, r.abs = z.abs.div(nil, r.abs, x.abs, y.abs)
	z.neg, r.neg = len(z.abs) > 0 && x.neg != y.neg, len(r.abs) > 0 && x.neg // 0 has no sign
	return z, r
}

// Div sets z to the quotient x/y for y != 0 and returns z.
// If y == 0, a division-by-zero run-time panic occurs.
// Div implements Euclidean division (unlike Go); see [Int.DivMod] for more details.
func (z *Int) Div(x, y *Int) *Int {
	y_neg := y.neg // z may be an alias for y
	var r Int
	z.QuoRem(x, y, &r)
	if r.neg {
		if y_neg {
			z.Add(z, intOne)
		} else {
			z.Sub(z, intOne)
		}
	}
	return z
}

// Mod sets z to the modulus x%y for y != 0 and returns z.
// If y == 0, a division-by-zero run-time panic occurs.
// Mod implements Euclidean modulus (unlike Go); see [Int.DivMod] for more details.
func (z *Int) Mod(x, y *Int) *Int {
	y0 := y // save y
	if z == y || alias(z.abs, y.abs) {
		y0 = new(Int).Set(y)
	}
	var q Int
	q.QuoRem(x, y, z)
	if z.neg {
		if y0.neg {
			z.Sub(z, y0)
		} else {
			z.Add(z, y0)
		}
	}
	return z
}

// DivMod sets z to the quotient x div y and m to the modulus x mod y
// and returns the pair (z, m) for y != 0.
// If y == 0, a division-by-zero run-time panic occurs.
//
// DivMod implements Euclidean division and modulus (unlike Go):
//
//	q = x div y  such that
//	m = x - y*q  with 0 <= m < |y|
//
// (See Raymond T. Boute, “The Euclidean definition of the functions
// div and mod”. ACM Transactions on Programming Languages and
// Systems (TOPLAS), 14(2):127-144, New York, NY, USA, 4/1992.
// ACM press.)
// See [Int.QuoRem] for T-division and modulus (like Go).
func (z *Int) DivMod(x, y, m *Int) (*Int, *Int) {
	y0 := y // save y
	if z == y || alias(z.abs, y.abs) {
		y0 = new(Int).Set(y)
	}
	z.QuoRem(x, y, m)
	if m.neg {
		if y0.neg {
			z.Add(z, intOne)
			m.Sub(m, y0)
		} else {
			z.Sub(z, intOne)
			m.Add(m, y0)
		}
	}
	return z, m
}

// Rounding modes that determine how the integer quotient is adjusted in an integer division.
// See Daan Leijen, “Division and Modulus for Computer Scientists”, for details.
const (
	Trunc = ToZero        // T-division (same as Go division)
	Floor = ToNegativeInf // F-division
	Round = ToNearestEven // R-division
	Ceil  = ToPositiveInf // C-division
)

// Divide computes the integer quotient q and remainder r such that
//
//	q = f(x/y)
//	r = x - y*q
//
// where f is described by the rounding mode,
// which must be one of [Trunc], [Floor], [Round] or [Ceil].
// Divide sets z to q if z != nil, updates r if r != nil,
// and returns the pair (z, r) if y != 0.
// If y == 0, a division-by-zero run-time panic occurs.
func (z *Int) Divide(x, y, r *Int, mode RoundingMode) (*Int, *Int) {
	// TODO: optimize the code where z or r is nil
	var z_abs nat
	if z != nil {
		z_abs = z.abs
	}
	var r_neg bool
	var r_abs nat
	if r != nil {
		r_abs = r.abs
	}
	y_abs := y.abs // save y
	if z == y || alias(z_abs, y.abs) {
		y_abs = nat(nil).set(y.abs)
	}
	neg := x.neg != y.neg
	z_abs, r_abs = z_abs.div(nil, r_abs, x.abs, y.abs)
	if len(r_abs) > 0 {
		switch mode {
		case Trunc:
			r_neg = x.neg
		case Floor:
			r_neg = y.neg
			if neg {
				z_abs = z_abs.add(z_abs, natOne)
				r_abs = r_abs.sub(y_abs, r_abs)
			}
		case Ceil:
			r_neg = !y.neg
			if !neg {
				z_abs = z_abs.add(z_abs, natOne)
				r_abs = r_abs.sub(y_abs, r_abs)
			}
		case Round:
			switch nat(nil).mul(nil, r_abs, natTwo).cmp(y_abs) {
			case -1:
				r_neg = x.neg
			case 0:
				even := len(z_abs) == 0 || z_abs[0]&1 == 0
				if even {
					r_neg = x.neg
					break
				}
				fallthrough
			case 1:
				r_neg = !x.neg
				z_abs = z_abs.add(z_abs, natOne)
				r_abs = r_abs.sub(y_abs, r_abs)
			}
		default:
			panic("unsupported rounding mode")
		}
	}
	if z != nil {
		z.abs = z_abs
		z.neg = neg && len(z_abs) > 0 // 0 has no sign
	}
	if r != nil {
		r.abs = r_abs
		r.neg = r_neg
	}
	return z, r
}

// Cmp compares x and y and returns:
//   - -1 if x < y;
//   - 0 if x == y;
//   - +1 if x > y.
func (x *Int) Cmp(y *Int) (r int) {
	// x cmp y == x cmp y
	// x cmp (-y) == x
	// (-x) cmp y == y
	// (-x) cmp (-y) == -(x cmp y)
	switch {
	case x == y:
		// nothing to do
	case x.neg == y.neg:
		r = x.abs.cmp(y.abs)
		if x.neg {
			r = -r
		}
	case x.neg:
		r = -1
	default:
		r = 1
	}
	return
}

// CmpAbs compares the absolute values of x and y and returns:
//   - -1 if |x| < |y|;
//   - 0 if |x| == |y|;
//   - +1 if |x| > |y|.
func (x *Int) CmpAbs(y *Int) int {
	return x.abs.cmp(y.abs)
}

// low32 returns the least significant 32 bits of x.
func low32(x nat) uint32 {
	if len(x) == 0 {
		return 0
	}
	return uint32(x[0])
}

// low64 returns the least significant 64 bits of x.
func low64(x nat) uint64 {
	if len(x) == 0 {
		return 0
	}
	v := uint64(x[0])
	if _W == 32 && len(x) > 1 {
		return uint64(x[1])<<32 | v
	}
	return v
}

// Int64 returns the int64 representation of x.
// If x cannot be represented in an int64, the result is undefined.
func (x *Int) Int64() int64 {
	v := int64(low64(x.abs))
	if x.neg {
		v = -v
	}
	return v
}

// Uint64 returns the uint64 representation of x.
// If x cannot be represented in a uint64, the result is undefined.
func (x *Int) Uint64() uint64 {
	return low64(x.abs)
}

// IsInt64 reports whether x can be represented as an int64.
func (x *Int) IsInt64() bool {
	if len(x.abs) <= 64/_W {
		w := int64(low64(x.abs))
		return w >= 0 || x.neg && w == -w
	}
	return false
}

// IsUint64 reports whether x can be represented as a uint64.
func (x *Int) IsUint64() bool {
	return !x.neg && len(x.abs) <= 64/_W
}

// Float64 returns the float64 value nearest x,
// and an indication of any rounding that occurred.
func (x *Int) Float64() (float64, Accuracy) {
	n := x.abs.bitLen() // NB: still uses slow crypto impl!
	if n == 0 {
		return 0.0, Exact
	}

	// Fast path: no more than 53 significant bits.
	if n <= 53 || n < 64 && n-int(x.abs.trailingZeroBits()) <= 53 {
		f := float64(low64(x.abs))
		if x.neg {
			f = -f
		}
		return f, Exact
	}

	return new(Float).SetInt(x).Float64()
}

// SetString sets z to the value of s, interpreted in the given base,
// and returns z and a boolean indicating success. The entire string
// (not just a prefix) must be valid for success. If SetString fails,
// the value of z is undefined but the returned value is nil.
//
// The base argument must be 0 or a value between 2 and [MaxBase].
// For base 0, the number prefix determines the actual base: A prefix of
// “0b” or “0B” selects base 2, “0”, “0o” or “0O” selects base 8,
// and “0x” or “0X” selects base 16. Otherwise, the selected base is 10
// and no prefix is accepted.
//
// For bases <= 36, lower and upper case letters are considered the same:
// The letters 'a' to 'z' and 'A' to 'Z' represent digit values 10 to 35.
// For bases > 36, the upper case letters 'A' to 'Z' represent the digit
// values 36 to 61.
//
// For base 0, an underscore character “_” may appear between a base
// prefix and an adjacent digit, and between successive digits; such
// underscores do not change the value of the number.
// Incorrect placement of underscores is reported as an error if there
// are no other errors. If base != 0, underscores are not recognized
// and act like any other character that is not a valid digit.
func (z *Int) SetString(s string, base int) (*Int, bool) {
	return z.setFromScanner(strings.NewReader(s), base)
}

// setFromScanner implements SetString given an io.ByteScanner.
// For documentation see comments of SetString.
func (z *Int) setFromScanner(r io.ByteScanner, base int) (*Int, bool) {
	if _, _, err := z.scan(r, base); err != nil {
		return nil, false
	}
	// entire content must have been consumed
	if _, err := r.ReadByte(); err != io.EOF {
		return nil, false
	}
	return z, true // err == io.EOF => scan consumed all content of r
}

// SetBytes interprets buf as the bytes of a big-endian unsigned
// integer, sets z to that value, and returns z.
func (z *Int) SetBytes(buf []byte) *Int {
	z.abs = z.abs.setBytes(buf)
	z.neg = false
	return z
}

// Bytes returns the absolute value of x as a big-endian byte slice.
//
// To use a fixed length slice, or a preallocated one, use [Int.FillBytes].
func (x *Int) Bytes() []byte {
	// This function is used in cryptographic operations. It must not leak
	// anything but the Int's sign and bit size through side-channels. Any
	// changes must be reviewed by a security expert.
	buf := make([]byte, len(x.abs)*_S)
	return buf[x.abs.bytes(buf):]
}

// FillBytes sets buf to the absolute value of x, storing it as a zero-extended
// big-endian byte slice, and returns buf.
//
// If the absolute value of x doesn't fit in buf, FillBytes will panic.
func (x *Int) FillBytes(buf []byte) []byte {
	// Clear whole buffer.
	clear(buf)
	x.abs.bytes(buf)
	return buf
}

// BitLen returns the length of the absolute value of x in bits.
// The bit length of 0 is 0.
func (x *Int) BitLen() int {
	// This function is used in cryptographic operations. It must not leak
	// anything but the Int's sign and bit size through side-channels. Any
	// changes must be reviewed by a security expert.
	return x.abs.bitLen()
}

// TrailingZeroBits returns the number of consecutive least significant zero
// bits of |x|.
func (x *Int) TrailingZeroBits() uint {
	return x.abs.trailingZeroBits()
}

// Exp sets z = x**y mod |m| (i.e. the sign of m is ignored), and returns z.
// If m == nil or m == 0, z = x**y unless y <= 0 then z = 1. If m != 0, y < 0,
// and x and m are not relatively prime, z is unchanged and nil is returned.
//
// Modular exponentiation of inputs of a particular size is not a
// cryptographically constant-time operation.
func (z *Int) Exp(x, y, m *Int) *Int {
	return z.exp(x, y, m, false)
}

func (z *Int) expSlow(x, y, m *Int) *Int {
	return z.exp(x, y, m, true)
}

func (z *Int) exp(x, y, m *Int, slow bool) *Int {
	// See Knuth, volume 2, section 4.6.3.
	xWords := x.abs
	if y.neg {
		if m == nil || len(m.abs) == 0 {
			return z.SetInt64(1)
		}
		// for y < 0: x**y mod m == (x**(-1))**|y| mod m
		inverse := new(Int).ModInverse(x, m)
		if inverse == nil {
			return nil
		}
		xWords = inverse.abs
	}
	yWords := y.abs

	var mWords nat
	if m != nil {
		if z == m || alias(z.abs, m.abs) {
			m = new(Int).Set(m)
		}
		mWords = m.abs // m.abs may be nil for m == 0
	}

	z.abs = z.abs.expNN(nil, xWords, yWords, mWords, slow)
	z.neg = len(z.abs) > 0 && x.neg && len(yWords) > 0 && yWords[0]&1 == 1 // 0 has no sign
	if z.neg && len(mWords) > 0 {
		// make modulus result positive
		z.abs = z.abs.sub(mWords, z.abs) // z == x**y mod |m| && 0 <= z < |m|
		z.neg = false
	}

	return z
}

// GCD sets z to the greatest common divisor of a and b and returns z.
// If x or y are not nil, GCD sets their value such that z = a*x + b*y.
//
// a and b may be positive, zero or negative. (Before Go 1.14 both had
// to be > 0.) Regardless of the signs of a and b, z is always >= 0.
//
// If a == b == 0, GCD sets z = x = y = 0.
//
// If a == 0 and b != 0, GCD sets z = |b|, x = 0, y = sign(b) * 1.
//
// If a != 0 and b == 0, GCD sets z = |a|, x = sign(a) * 1, y = 0.
func (z *Int) GCD(x, y, a, b *Int) *Int {
	if len(a.abs) == 0 || len(b.abs) == 0 {
		lenA, lenB, negA, negB := len(a.abs), len(b.abs), a.neg, b.neg
		if lenA == 0 {
			z.Set(b)
		} else {
			z.Set(a)
		}
		z.neg = false
		if x != nil {
			if lenA == 0 {
				x.SetUint64(0)
			} else {
				x.SetUint64(1)
				x.neg = negA
			}
		}
		if y != nil {
			if lenB == 0 {
				y.SetUint64(0)
			} else {
				y.SetUint64(1)
				y.neg = negB
			}
		}
		return z
	}

	return z.lehmerGCD(x, y, a, b)
}

// lehmerSimulate attempts to simulate several Euclidean update steps
// using the leading digits of A and B.  It returns u0, u1, v0, v1
// such that A and B can be updated as:
//
//	A = u0*A + v0*B
//	B = u1*A + v1*B
//
// Requirements: A >= B and len(B.abs) >= 2
// Since we are calculating with full words to avoid overflow,
// we use 'even' to track the sign of the cosequences.
// For even iterations: u0, v1 >= 0 && u1, v0 <= 0
// For odd  iterations: u0, v1 <= 0 && u1, v0 >= 0
func lehmerSimulate(A, B *Int) (u0, u1, v0, v1 Word, even bool) {
	// initialize the digits
	var a1, a2, u2, v2 Word

	m := len(B.abs) // m >= 2
	n := len(A.abs) // n >= m >= 2

	// extract the top Word of bits from A and B
	h := nlz(A.abs[n-1])
	a1 = A.abs[n-1]<<h | A.abs[n-2]>>(_W-h)
	// B may have implicit zero words in the high bits if the lengths differ
	switch {
	case n == m:
		a2 = B.abs[n-1]<<h | B.abs[n-2]>>(_W-h)
	case n == m+1:
		a2 = B.abs[n-2] >> (_W - h)
	default:
		a2 = 0
	}

	// Since we are calculating with full words to avoid overflow,
	// we use 'even' to track the sign of the cosequences.
	// For even iterations: u0, v1 >= 0 && u1, v0 <= 0
	// For odd  iterations: u0, v1 <= 0 && u1, v0 >= 0
	// The first iteration starts with k=1 (odd).
	even = false
	// variables to track the cosequences
	u0, u1, u2 = 0, 1, 0
	v0, v1, v2 = 0, 0, 1

	// Calculate the quotient and cosequences using Collins' stopping condition.
	// Note that overflow of a Word is not possible when computing the remainder
	// sequence and cosequences since the cosequence size is bounded by the input size.
	// See section 4.2 of Jebelean for details.
	for a2 >= v2 && a1-a2 >= v1+v2 {
		q, r := a1/a2, a1%a2
		a1, a2 = a2, r
		u0, u1, u2 = u1, u2, u1+q*u2
		v0, v1, v2 = v1, v2, v1+q*v2
		even = !even
	}
	return
}

// lehmerUpdate updates the inputs A and B such that:
//
//	A = u0*A + v0*B
//	B = u1*A + v1*B
//
// where the signs of u0, u1, v0, v1 are given by even
// For even == true: u0, v1 >= 0 && u1, v0 <= 0
// For even == false: u0, v1 <= 0 && u1, v0 >= 0
// q, r, s, t are temporary variables to avoid allocations in the multiplication.
func lehmerUpdate(A, B, q, r *Int, u0, u1, v0, v1 Word, even bool) {
	mulW(q, B, even, v0)
	mulW(r, A, even, u1)
	mulW(A, A, !even, u0)
	mulW(B, B, !even, v1)
	A.Add(A, q)
	B.Add(B, r)
}

// mulW sets z = x * (-?)w
// where the minus sign is present when neg is true.
func mulW(z, x *Int, neg bool, w Word) {
	z.abs = z.abs.mulAddWW(x.abs, w, 0)
	z.neg = x.neg != neg
}

// euclidUpdate performs a single step of the Euclidean GCD algorithm
// if extended is true, it also updates the cosequence Ua, Ub.
// q and r are used as temporaries; the initial values are ignored.
func euclidUpdate(A, B, Ua, Ub, q, r *Int, extended bool) (nA, nB, nr, nUa, nUb *Int) {
	q.QuoRem(A, B, r)

	if extended {
		// Ua, Ub = Ub, Ua-q*Ub
		q.Mul(q, Ub)
		Ua, Ub = Ub, Ua
		Ub.Sub(Ub, q)
	}

	return B, r, A, Ua, Ub
}

// lehmerGCD sets z to the greatest common divisor of a and b,
// which both must be != 0, and returns z.
// If x or y are not nil, their values are set such that z = a*x + b*y.
// See Knuth, The Art of Computer Programming, Vol. 2, Section 4.5.2, Algorithm L.
// This implementation uses the improved condition by Collins requiring only one
// quotient and avoiding the possibility of single Word overflow.
// See Jebelean, "Improving the multiprecision Euclidean algorithm",
// Design and Implementation of Symbolic Computation Systems, pp 45-58.
// The cosequences are updated according to Algorithm 10.45 from
// Cohen et al. "Handbook of Elliptic and Hyperelliptic Curve Cryptography" pp 192.
func (z *Int) lehmerGCD(x, y, a, b *Int) *Int {
	var A, B, Ua, Ub *Int

	A = new(Int).Abs(a)
	B = new(Int).Abs(b)

	extended := x != nil || y != nil

	if extended {
		// Ua (Ub) tracks how many times input a has been accumulated into A (B).
		Ua = new(Int).SetInt64(1)
		Ub = new(Int)
	}

	// temp variables for multiprecision update
	q := new(Int)
	r := new(Int)

	// ensure A >= B
	if A.abs.cmp(B.abs) < 0 {
		A, B = B, A
		Ub, Ua = Ua, Ub
	}

	// loop invariant A >= B
	for len(B.abs) > 1 {
		// Attempt to calculate in single-precision using leading words of A and B.
		u0, u1, v0, v1, even := lehmerSimulate(A, B)

		// multiprecision Step
		if v0 != 0 {
			// Simulate the effect of the single-precision steps using the cosequences.
			// A = u0*A + v0*B
			// B = u1*A + v1*B
			lehmerUpdate(A, B, q, r, u0, u1, v0, v1, even)

			if extended {
				// Ua = u0*Ua + v0*Ub
				// Ub = u1*Ua + v1*Ub
				lehmerUpdate(Ua, Ub, q, r, u0, u1, v0, v1, even)
			}

		} else {
			// Single-digit calculations failed to simulate any quotients.
			// Do a standard Euclidean step.
			A, B, r, Ua, Ub = euclidUpdate(A, B, Ua, Ub, q, r, extended)
		}
	}

	if len(B.abs) > 0 {
		// extended Euclidean algorithm base case if B is a single Word
		if len(A.abs) > 1 {
			// A is longer than a single Word, so one update is needed.
			A, B, r, Ua, Ub = euclidUpdate(A, B, Ua, Ub, q, r, extended)
		}
		if len(B.abs) > 0 {
			// A and B are both a single Word.
			aWord, bWord := A.abs[0], B.abs[0]
			if extended {
				var ua, ub, va, vb Word
				ua, ub = 1, 0
				va, vb = 0, 1
				even := true
				for bWord != 0 {
					q, r := aWord/bWord, aWord%bWord
					aWord, bWord = bWord, r
					ua, ub = ub, ua+q*ub
					va, vb = vb, va+q*vb
					even = !even
				}

				mulW(Ua, Ua, !even, ua)
				mulW(Ub, Ub, even, va)
				Ua.Add(Ua, Ub)
			} else {
				for bWord != 0 {
					aWord, bWord = bWord, aWord%bWord
				}
			}
			A.abs[0] = aWord
		}
	}
	negA := a.neg
	if y != nil {
		// avoid aliasing b needed in the division below
		if y == b {
			B.Set(b)
		} else {
			B = b
		}
		// y = (z - a*x)/b
		y.Mul(a, Ua) // y can safely alias a
		if negA {
			y.neg = !y.neg
		}
		y.Sub(A, y)
		y.Div(y, B)
	}

	if x != nil {
		x.Set(Ua)
		if negA {
			x.neg = !x.neg
		}
	}

	z.Set(A)

	return z
}

// Rand sets z to a pseudo-random number in [0, n) and returns z.
//
// As this uses the [math/rand] package, it must not be used for
// security-sensitive work. Use [crypto/rand.Int] instead.
func (z *Int) Rand(rnd *rand.Rand, n *Int) *Int {
	// z.neg is not modified before the if check, because z and n might alias.
	if n.neg || len(n.abs) == 0 {
		z.neg = false
		z.abs = nil
		return z
	}
	z.neg = false
	z.abs = z.abs.random(rnd, n.abs, n.abs.bitLen())
	return z
}

// ModInverse sets z to the multiplicative inverse of g in the ring ℤ/nℤ
// and returns z. If g and n are not relatively prime, g has no multiplicative
// inverse in the ring ℤ/nℤ.  In this case, z is unchanged and the return value
// is nil. If n == 0, a division-by-zero run-time panic occurs.
func (z *Int) ModInverse(g, n *Int) *Int {
	// GCD expects parameters a and b to be > 0.
	if n.neg {
		var n2 Int
		n = n2.Neg(n)
	}
	if g.neg {
		var g2 Int
		g = g2.Mod(g, n)
	}
	var d, x Int
	d.GCD(&x, nil, g, n)

	// if and only if d==1, g and n are relatively prime
	if d.Cmp(intOne) != 0 {
		return nil
	}

	// x and y are such that g*x + n*y = 1, therefore x is the inverse element,
	// but it may be negative, so convert to the range 0 <= z < |n|
	if x.neg {
		z.Add(&x, n)
	} else {
		z.Set(&x)
	}
	return z
}

func (z nat) modInverse(g, n nat) nat {
	// TODO(rsc): ModInverse should be implemented in terms of this function.
	return (&Int{abs: z}).ModInverse(&Int{abs: g}, &Int{abs: n}).abs
}

// Jacobi returns the Jacobi symbol (x/y), either +1, -1, or 0.
// The y argument must be an odd integer.
func Jacobi(x, y *Int) int {
	if len(y.abs) == 0 || y.abs[0]&1 == 0 {
		panic(fmt.Sprintf("big: invalid 2nd argument to Int.Jacobi: need odd integer but got %s", y.String()))
	}

	// We use the formulation described in chapter 2, section 2.4,
	// "The Yacas Book of Algorithms":
	// http://yacas.sourceforge.net/Algo.book.pdf

	var a, b, c Int
	a.Set(x)
	b.Set(y)
	j := 1

	if b.neg {
		if a.neg {
			j = -1
		}
		b.neg = false
	}

	for {
		if b.Cmp(intOne) == 0 {
			return j
		}
		if len(a.abs) == 0 {
			return 0
		}
		a.Mod(&a, &b)
		if len(a.abs) == 0 {
			return 0
		}
		// a > 0

		// handle factors of 2 in 'a'
		s := a.abs.trailingZeroBits()
		if s&1 != 0 {
			bmod8 := b.abs[0] & 7
			if bmod8 == 3 || bmod8 == 5 {
				j = -j
			}
		}
		c.Rsh(&a, s) // a = 2^s*c

		// swap numerator and denominator
		if b.abs[0]&3 == 3 && c.abs[0]&3 == 3 {
			j = -j
		}
		a.Set(&b)
		b.Set(&c)
	}
}

// modSqrt3Mod4 uses the identity
//
//	   (a^((p+1)/4))^2  mod p
//	== u^(p+1)          mod p
//	== u^2              mod p
//
// to calculate the square root of any quadratic residue mod p quickly for 3
// mod 4 primes.
func (z *Int) modSqrt3Mod4Prime(x, p *Int) *Int {
	e := new(Int).Add(p, intOne) // e = p + 1
	e.Rsh(e, 2)                  // e = (p + 1) / 4
	z.Exp(x, e, p)               // z = x^e mod p
	return z
}

// modSqrt5Mod8Prime uses Atkin's observation that 2 is not a square mod p
//
//	alpha ==  (2*a)^((p-5)/8)    mod p
//	beta  ==  2*a*alpha^2        mod p  is a square root of -1
//	b     ==  a*alpha*(beta-1)   mod p  is a square root of a
//
// to calculate the square root of any quadratic residue mod p quickly for 5
// mod 8 primes.
func (z *Int) modSqrt5Mod8Prime(x, p *Int) *Int {
	// p == 5 mod 8 implies p = e*8 + 5
	// e is the quotient and 5 the remainder on division by 8
	e := new(Int).Rsh(p, 3)  // e = (p - 5) / 8
	tx := new(Int).Lsh(x, 1) // tx = 2*x
	alpha := new(Int).Exp(tx, e, p)
	beta := new(Int).Mul(alpha, alpha)
	beta.Mod(beta, p)
	beta.Mul(beta, tx)
	beta.Mod(beta, p)
	beta.Sub(beta, intOne)
	beta.Mul(beta, x)
	beta.Mod(beta, p)
	beta.Mul(beta, alpha)
	z.Mod(beta, p)
	return z
}

// modSqrtTonelliShanks uses the Tonelli-Shanks algorithm to find the square
// root of a quadratic residue modulo any prime.
func (z *Int) modSqrtTonelliShanks(x, p *Int) *Int {
	// Break p-1 into s*2^e such that s is odd.
	var s Int
	s.Sub(p, intOne)
	e := s.abs.trailingZeroBits()
	s.Rsh(&s, e)

	// find some non-square n
	var n Int
	n.SetInt64(2)
	for Jacobi(&n, p) != -1 {
		n.Add(&n, intOne)
	}

	// Core of the Tonelli-Shanks algorithm. Follows the description in
	// section 6 of "Square roots from 1; 24, 51, 10 to Dan Shanks" by Ezra
	// Brown:
	// https://www.maa.org/sites/default/files/pdf/upload_library/22/Polya/07468342.di020786.02p0470a.pdf
	var y, b, g, t Int
	y.Add(&s, intOne)
	y.Rsh(&y, 1)
	y.Exp(x, &y, p)  // y = x^((s+1)/2)
	b.Exp(x, &s, p)  // b = x^s
	g.Exp(&n, &s, p) // g = n^s
	r := e
	for {
		// find the least m such that ord_p(b) = 2^m
		var m uint
		t.Set(&b)
		for t.Cmp(intOne) != 0 {
			t.Mul(&t, &t).Mod(&t, p)
			m++
		}

		if m == 0 {
			return z.Set(&y)
		}

		t.SetInt64(0).SetBit(&t, int(r-m-1), 1).Exp(&g, &t, p)
		// t = g^(2^(r-m-1)) mod p
		g.Mul(&t, &t).Mod(&g, p) // g = g^(2^(r-m)) mod p
		y.Mul(&y, &t).Mod(&y, p)
		b.Mul(&b, &g).Mod(&b, p)
		r = m
	}
}

// ModSqrt sets z to a square root of x mod p if such a square root exists, and
// returns z. The modulus p must be an odd prime. If x is not a square mod p,
// ModSqrt leaves z unchanged and returns nil. This function panics if p is
// not an odd integer, its behavior is undefined if p is odd but not prime.
func (z *Int) ModSqrt(x, p *Int) *Int {
	switch Jacobi(x, p) {
	case -1:
		return nil // x is not a square mod p
	case 0:
		return z.SetInt64(0) // sqrt(0) mod p = 0
	case 1:
		break
	}
	if x.neg || x.Cmp(p) >= 0 { // ensure 0 <= x < p
		x = new(Int).Mod(x, p)
	}

	switch {
	case p.abs[0]%4 == 3:
		// Check whether p is 3 mod 4, and if so, use the faster algorithm.
		return z.modSqrt3Mod4Prime(x, p)
	case p.abs[0]%8 == 5:
		// Check whether p is 5 mod 8, use Atkin's algorithm.
		return z.modSqrt5Mod8Prime(x, p)
	default:
		// Otherwise, use Tonelli-Shanks.
		return z.modSqrtTonelliShanks(x, p)
	}
}

// Lsh sets z = x << n and returns z.
func (z *Int) Lsh(x *Int, n uint) *Int {
	z.abs = z.abs.lsh(x.abs, n)
	z.neg = x.neg
	return z
}

// Rsh sets z = x >> n and returns z.
func (z *Int) Rsh(x *Int, n uint) *Int {
	if x.neg {
		// (-x) >> s == ^(x-1) >> s == ^((x-1) >> s) == -(((x-1) >> s) + 1)
		t := z.abs.sub(x.abs, natOne) // no underflow because |x| > 0
		t = t.rsh(t, n)
		z.abs = t.add(t, natOne)
		z.neg = true // z cannot be zero if x is negative
		return z
	}

	z.abs = z.abs.rsh(x.abs, n)
	z.neg = false
	return z
}

// Bit returns the value of the i'th bit of x. That is, it
// returns (x>>i)&1. The bit index i must be >= 0.
func (x *Int) Bit(i int) uint {
	if i == 0 {
		// optimization for common case: odd/even test of x
		if len(x.abs) > 0 {
			return uint(x.abs[0] & 1) // bit 0 is same for -x
		}
		return 0
	}
	if i < 0 {
		panic("negative bit index")
	}
	if x.neg {
		t := nat(nil).sub(x.abs, natOne)
		return t.bit(uint(i)) ^ 1
	}

	return x.abs.bit(uint(i))
}

// SetBit sets z to x, with x's i'th bit set to b (0 or 1).
// That is,
//   - if b is 1, SetBit sets z = x | (1 << i);
//   - if b is 0, SetBit sets z = x &^ (1 << i);
//   - if b is not 0 or 1, SetBit will panic.
func (z *Int) SetBit(x *Int, i int, b uint) *Int {
	if i < 0 {
		panic("negative bit index")
	}
	if x.neg {
		t := z.abs.sub(x.abs, natOne)
		t = t.setBit(t, uint(i), b^1)
		z.abs = t.add(t, natOne)
		z.neg = len(z.abs) > 0
		return z
	}
	z.abs = z.abs.setBit(x.abs, uint(i), b)
	z.neg = false
	return z
}

// And sets z = x & y and returns z.
func (z *Int) And(x, y *Int) *Int {
	if x.neg == y.neg {
		if x.neg {
			// (-x) & (-y) == ^(x-1) & ^(y-1) == ^((x-1) | (y-1)) == -(((x-1) | (y-1)) + 1)
			x1 := nat(nil).sub(x.abs, natOne)
			y1 := nat(nil).sub(y.abs, natOne)
			z.abs = z.abs.add(z.abs.or(x1, y1), natOne)
			z.neg = true // z cannot be zero if x and y are negative
			return z
		}

		// x & y == x & y
		z.abs = z.abs.and(x.abs, y.abs)
		z.neg = false
		return z
	}

	// x.neg != y.neg
	if x.neg {
		x, y = y, x // & is symmetric
	}

	// x & (-y) == x & ^(y-1) == x &^ (y-1)
	y1 := nat(nil).sub(y.abs, natOne)
	z.abs = z.abs.andNot(x.abs, y1)
	z.neg = false
	return z
}

// AndNot sets z = x &^ y and returns z.
func (z *Int) AndNot(x, y *Int) *Int {
	if x.neg == y.neg {
		if x.neg {
			// (-x) &^ (-y) == ^(x-1) &^ ^(y-1) == ^(x-1) & (y-1) == (y-1) &^ (x-1)
			x1 := nat(nil).sub(x.abs, natOne)
			y1 := nat(nil).sub(y.abs, natOne)
			z.abs = z.abs.andNot(y1, x1)
			z.neg = false
			return z
		}

		// x &^ y == x &^ y
		z.abs = z.abs.andNot(x.abs, y.abs)
		z.neg = false
		return z
	}

	if x.neg {
		// (-x) &^ y == ^(x-1) &^ y == ^(x-1) & ^y == ^((x-1) | y) == -(((x-1) | y) + 1)
		x1 := nat(nil).sub(x.abs, natOne)
		z.abs = z.abs.add(z.abs.or(x1, y.abs), natOne)
		z.neg = true // z cannot be zero if x is negative and y is positive
		return z
	}

	// x &^ (-y) == x &^ ^(y-1) == x & (y-1)
	y1 := nat(nil).sub(y.abs, natOne)
	z.abs = z.abs.and(x.abs, y1)
	z.neg = false
	return z
}

// Or sets z = x | y and returns z.
func (z *Int) Or(x, y *Int) *Int {
	if x.neg == y.neg {
		if x.neg {
			// (-x) | (-y) == ^(x-1) | ^(y-1) == ^((x-1) & (y-1)) == -(((x-1) & (y-1)) + 1)
			x1 := nat(nil).sub(x.abs, natOne)
			y1 := nat(nil).sub(y.abs, natOne)
			z.abs = z.abs.add(z.abs.and(x1, y1), natOne)
			z.neg = true // z cannot be zero if x and y are negative
			return z
		}

		// x | y == x | y
		z.abs = z.abs.or(x.abs, y.abs)
		z.neg = false
		return z
	}

	// x.neg != y.neg
	if x.neg {
		x, y = y, x // | is symmetric
	}

	// x | (-y) == x | ^(y-1) == ^((y-1) &^ x) == -(^((y-1) &^ x) + 1)
	y1 := nat(nil).sub(y.abs, natOne)
	z.abs = z.abs.add(z.abs.andNot(y1, x.abs), natOne)
	z.neg = true // z cannot be zero if one of x or y is negative
	return z
}

// Xor sets z = x ^ y and returns z.
func (z *Int) Xor(x, y *Int) *Int {
	if x.neg == y.neg {
		if x.neg {
			// (-x) ^ (-y) == ^(x-1) ^ ^(y-1) == (x-1) ^ (y-1)
			x1 := nat(nil).sub(x.abs, natOne)
			y1 := nat(nil).sub(y.abs, natOne)
			z.abs = z.abs.xor(x1, y1)
			z.neg = false
			return z
		}

		// x ^ y == x ^ y
		z.abs = z.abs.xor(x.abs, y.abs)
		z.neg = false
		return z
	}

	// x.neg != y.neg
	if x.neg {
		x, y = y, x // ^ is symmetric
	}

	// x ^ (-y) == x ^ ^(y-1) == ^(x ^ (y-1)) == -((x ^ (y-1)) + 1)
	y1 := nat(nil).sub(y.abs, natOne)
	z.abs = z.abs.add(z.abs.xor(x.abs, y1), natOne)
	z.neg = true // z cannot be zero if only one of x or y is negative
	return z
}

// Not sets z = ^x and returns z.
func (z *Int) Not(x *Int) *Int {
	if x.neg {
		// ^(-x) == ^(^(x-1)) == x-1
		z.abs = z.abs.sub(x.abs, natOne)
		z.neg = false
		return z
	}

	// ^x == -x-1 == -(x+1)
	z.abs = z.abs.add(x.abs, natOne)
	z.neg = true // z cannot be zero if x is positive
	return z
}

// Sqrt sets z to ⌊√x⌋, the largest integer such that z² ≤ x, and returns z.
// It panics if x is negative.
func (z *Int) Sqrt(x *Int) *Int {
	if x.neg {
		panic("square root of negative number")
	}
	z.neg = false
	z.abs = z.abs.sqrt(nil, x.abs)
	return z
}

```

// === FILE: references!/go/src/math/big/intconv.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements int-to-string conversion functions.

package big

import (
	"errors"
	"fmt"
	"io"
)

// Text returns the string representation of x in the given base.
// Base must be between 2 and 62, inclusive. The result uses the
// lower-case letters 'a' to 'z' for digit values 10 to 35, and
// the upper-case letters 'A' to 'Z' for digit values 36 to 61.
// No prefix (such as "0x") is added to the string. If x is a nil
// pointer it returns "<nil>".
func (x *Int) Text(base int) string {
	if x == nil {
		return "<nil>"
	}
	return string(x.abs.itoa(x.neg, base))
}

// Append appends the string representation of x, as generated by
// x.Text(base), to buf and returns the extended buffer.
func (x *Int) Append(buf []byte, base int) []byte {
	if x == nil {
		return append(buf, "<nil>"...)
	}
	return append(buf, x.abs.itoa(x.neg, base)...)
}

// String returns the decimal representation of x as generated by
// x.Text(10).
func (x *Int) String() string {
	return x.Text(10)
}

// write count copies of text to s.
func writeMultiple(s fmt.State, text string, count int) {
	if len(text) <= 0 || count <= 0 {
		return
	}
	if len(text) == 1 {
		if bw, ok := s.(io.ByteWriter); ok {
			for range count {
				bw.WriteByte(text[0])
			}
			return
		}
	}
	if sw, ok := s.(io.StringWriter); ok {
		for range count {
			sw.WriteString(text)
		}
		return
	}
	b := []byte(text)
	for range count {
		s.Write(b)
	}
}

var _ fmt.Formatter = intOne // *Int must implement fmt.Formatter

// Format implements [fmt.Formatter]. It accepts the formats
// 'b' (binary), 'o' (octal with 0 prefix), 'O' (octal with 0o prefix),
// 'd' (decimal), 'x' (lowercase hexadecimal), and
// 'X' (uppercase hexadecimal).
// Also supported are the full suite of package fmt's format
// flags for integral types, including '+' and ' ' for sign
// control, '#' for leading zero in octal and for hexadecimal,
// a leading "0x" or "0X" for "%#x" and "%#X" respectively,
// specification of minimum digits precision, output field
// width, space or zero padding, and '-' for left or right
// justification.
func (x *Int) Format(s fmt.State, ch rune) {
	// determine base
	var base int
	switch ch {
	case 'b':
		base = 2
	case 'o', 'O':
		base = 8
	case 'd', 's', 'v':
		base = 10
	case 'x', 'X':
		base = 16
	default:
		// unknown format
		fmt.Fprintf(s, "%%!%c(big.Int=%s)", ch, x.String())
		return
	}

	if x == nil {
		fmt.Fprint(s, "<nil>")
		return
	}

	// determine sign character
	sign := ""
	switch {
	case x.neg:
		sign = "-"
	case s.Flag('+'): // supersedes ' ' when both specified
		sign = "+"
	case s.Flag(' '):
		sign = " "
	}

	// determine prefix characters for indicating output base
	prefix := ""
	if s.Flag('#') {
		switch ch {
		case 'b': // binary
			prefix = "0b"
		case 'o': // octal
			prefix = "0"
		case 'x': // hexadecimal
			prefix = "0x"
		case 'X':
			prefix = "0X"
		}
	}
	if ch == 'O' {
		prefix = "0o"
	}

	digits := x.abs.utoa(base)
	if ch == 'X' {
		// faster than bytes.ToUpper
		for i, d := range digits {
			if 'a' <= d && d <= 'z' {
				digits[i] = 'A' + (d - 'a')
			}
		}
	}

	// number of characters for the three classes of number padding
	var left int  // space characters to left of digits for right justification ("%8d")
	var zeros int // zero characters (actually cs[0]) as left-most digits ("%.8d")
	var right int // space characters to right of digits for left justification ("%-8d")

	// determine number padding from precision: the least number of digits to output
	precision, precisionSet := s.Precision()
	if precisionSet {
		switch {
		case len(digits) < precision:
			zeros = precision - len(digits) // count of zero padding
		case len(digits) == 1 && digits[0] == '0' && precision == 0:
			return // print nothing if zero value (x == 0) and zero precision ("." or ".0")
		}
	}

	// determine field pad from width: the least number of characters to output
	length := len(sign) + len(prefix) + zeros + len(digits)
	if width, widthSet := s.Width(); widthSet && length < width { // pad as specified
		switch d := width - length; {
		case s.Flag('-'):
			// pad on the right with spaces; supersedes '0' when both specified
			right = d
		case s.Flag('0') && !precisionSet:
			// pad with zeros unless precision also specified
			zeros = d
		default:
			// pad on the left with spaces
			left = d
		}
	}

	// print number as [left pad][sign][prefix][zero pad][digits][right pad]
	writeMultiple(s, " ", left)
	writeMultiple(s, sign, 1)
	writeMultiple(s, prefix, 1)
	writeMultiple(s, "0", zeros)
	s.Write(digits)
	writeMultiple(s, " ", right)
}

// scan sets z to the integer value corresponding to the longest possible prefix
// read from r representing a signed integer number in a given conversion base.
// It returns z, the actual conversion base used, and an error, if any. In the
// error case, the value of z is undefined but the returned value is nil. The
// syntax follows the syntax of integer literals in Go.
//
// The base argument must be 0 or a value from 2 through MaxBase. If the base
// is 0, the string prefix determines the actual conversion base. A prefix of
// “0b” or “0B” selects base 2; a “0”, “0o”, or “0O” prefix selects
// base 8, and a “0x” or “0X” prefix selects base 16. Otherwise the selected
// base is 10.
func (z *Int) scan(r io.ByteScanner, base int) (*Int, int, error) {
	// determine sign
	neg, err := scanSign(r)
	if err != nil {
		return nil, 0, err
	}

	// determine mantissa
	z.abs, base, _, err = z.abs.scan(r, base, false)
	if err != nil {
		return nil, base, err
	}
	z.neg = len(z.abs) > 0 && neg // 0 has no sign

	return z, base, nil
}

func scanSign(r io.ByteScanner) (neg bool, err error) {
	var ch byte
	if ch, err = r.ReadByte(); err != nil {
		return false, err
	}
	switch ch {
	case '-':
		neg = true
	case '+':
		// nothing to do
	default:
		r.UnreadByte()
	}
	return
}

// byteReader is a local wrapper around fmt.ScanState;
// it implements the ByteReader interface.
type byteReader struct {
	fmt.ScanState
}

func (r byteReader) ReadByte() (byte, error) {
	ch, size, err := r.ReadRune()
	if size != 1 && err == nil {
		err = fmt.Errorf("invalid rune %#U", ch)
	}
	return byte(ch), err
}

func (r byteReader) UnreadByte() error {
	return r.UnreadRune()
}

var _ fmt.Scanner = intOne // *Int must implement fmt.Scanner

// Scan is a support routine for [fmt.Scanner]; it sets z to the value of
// the scanned number. It accepts the formats 'b' (binary), 'o' (octal),
// 'd' (decimal), 'x' (lowercase hexadecimal), and 'X' (uppercase hexadecimal).
func (z *Int) Scan(s fmt.ScanState, ch rune) error {
	s.SkipSpace() // skip leading space characters
	base := 0
	switch ch {
	case 'b':
		base = 2
	case 'o':
		base = 8
	case 'd':
		base = 10
	case 'x', 'X':
		base = 16
	case 's', 'v':
		// let scan determine the base
	default:
		return errors.New("Int.Scan: invalid verb")
	}
	_, _, err := z.scan(byteReader{s}, base)
	return err
}

```

// === FILE: references!/go/src/math/big/internal/asmgen/386.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package asmgen

import "fmt"

var Arch386 = &Arch{
	Name:      "386",
	WordBits:  32,
	WordBytes: 4,

	regs: []string{
		"BX", "SI", "DI", "BP",
		"CX", "DX", "AX", // last, to leave available for hinted allocation
	},
	op3:              x86Op3,
	hint:             x86Hint,
	memOK:            true,
	subCarryIsBorrow: true,
	maxColumns:       1, // not enough registers for more

	// Note: It would be nice to not set memIndex and then
	// delete all the code in pipe.go that supports it.
	// But a few routines, notably lshVU and mulAddVWW,
	// benefit dramatically from the use of index registers.
	// Perhaps some day we will decide 386 performance
	// does not matter enough to keep this code.
	memIndex: _386MemIndex,

	mov:      "MOVL",
	adds:     "ADDL",
	adcs:     "ADCL",
	subs:     "SUBL",
	sbcs:     "SBBL",
	lsh:      "SHLL",
	lshd:     "SHLL",
	rsh:      "SHRL",
	rshd:     "SHRL",
	and:      "ANDL",
	or:       "ORL",
	xor:      "XORL",
	neg:      "NEGL",
	lea:      "LEAL",
	mulWideF: x86MulWide,

	addWords: "LEAL (%[2]s)(%[1]s*4), %[3]s",

	jmpZero:       "TESTL %[1]s, %[1]s; JZ %[2]s",
	jmpNonZero:    "TESTL %[1]s, %[1]s; JNZ %[2]s",
	loopBottom:    "SUBL $1, %[1]s; JNZ %[2]s",
	loopBottomNeg: "ADDL $1, %[1]s; JNZ %[2]s",
}

func _386MemIndex(a *Asm, off int, ix Reg, p RegPtr) Reg {
	return Reg{fmt.Sprintf("%d(%s)(%s*%d)", off, p, ix, a.Arch.WordBytes)}
}

```

// === FILE: references!/go/src/math/big/internal/asmgen/add.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package asmgen

// addOrSubVV generates addVV or subVV,
// which do z, c = x ± y.
// The caller guarantees that len(z) == len(x) == len(y).
func addOrSubVV(a *Asm, name string) {
	f := a.Func("func " + name + "(z, x, y []Word) (c Word)")

	add := a.Add
	which := AddCarry
	if name == "subVV" {
		add = a.Sub
		which = SubCarry
	}

	n := f.Arg("z_len")
	p := f.Pipe()
	p.SetHint("y", HintMemOK) // allow y to be used from memory on x86
	p.Start(n, 1, 4)
	var c Reg
	if !a.Arch.CarrySafeLoop {
		// Carry smashed by loop tests; allocate and save in register
		// around unrolled blocks.
		c = a.Reg()
		a.Mov(a.Imm(0), c)
		a.EOL("clear saved carry")
		p.AtUnrollStart(func() { a.RestoreCarry(c); a.Free(c) })
		p.AtUnrollEnd(func() { a.Unfree(c); a.SaveCarry(c) })
	} else {
		// Carry preserved by loop; clear now, ahead of loop
		// (but after Start, which may have modified it).
		a.ClearCarry(which)
	}
	p.Loop(func(in, out [][]Reg) {
		for i, x := range in[0] {
			y := in[1][i]
			add(y, x, x, SetCarry|UseCarry)
		}
		p.StoreN(in[:1])
	})
	p.Done()

	// Copy carry to output.
	if c.Valid() {
		a.ConvertCarry(which, c)
	} else {
		c = a.RegHint(HintCarry)
		a.SaveConvertCarry(which, c)
	}
	f.StoreArg(c, "c")
	a.Free(c)
	a.Ret()
}

```

// === FILE: references!/go/src/math/big/internal/asmgen/amd64.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package asmgen

var ArchAMD64 = &Arch{
	Name:      "amd64",
	WordBits:  64,
	WordBytes: 8,

	regs: []string{
		"BX", "SI", "DI",
		"R8", "R9", "R10", "R11", "R12", "R13", "R14", "R15",
		"AX", "DX", "CX", // last to leave available for hinted allocation
	},
	op3:              x86Op3,
	hint:             x86Hint,
	memOK:            true,
	subCarryIsBorrow: true,

	// Note: Not setting memIndex, because code generally runs faster
	// if we avoid the use of scaled-index memory references,
	// particularly in ADX instructions.

	options: map[Option]func(*Asm, string){
		OptionAltCarry: amd64JmpADX,
	},

	mov:      "MOVQ",
	adds:     "ADDQ",
	adcs:     "ADCQ",
	subs:     "SUBQ",
	sbcs:     "SBBQ",
	lsh:      "SHLQ",
	lshd:     "SHLQ",
	rsh:      "SHRQ",
	rshd:     "SHRQ",
	and:      "ANDQ",
	or:       "ORQ",
	xor:      "XORQ",
	neg:      "NEGQ",
	lea:      "LEAQ",
	addF:     amd64Add,
	mulWideF: x86MulWide,

	addWords: "LEAQ (%[2]s)(%[1]s*8), %[3]s",

	jmpZero:       "TESTQ %[1]s, %[1]s; JZ %[2]s",
	jmpNonZero:    "TESTQ %[1]s, %[1]s; JNZ %[2]s",
	loopBottom:    "SUBQ $1, %[1]s; JNZ %[2]s",
	loopBottomNeg: "ADDQ $1, %[1]s; JNZ %[2]s",
}

func amd64JmpADX(a *Asm, label string) {
	a.Printf("\tCMPB ·hasADX(SB), $0; JNZ %s\n", label)
}

func amd64Add(a *Asm, src1, src2 Reg, dst Reg, carry Carry) bool {
	if a.Enabled(OptionAltCarry) {
		// If OptionAltCarry is enabled, the generator is emitting ADD instructions
		// both with and without the AltCarry flag set; the AltCarry flag means to
		// use ADOX. Otherwise we have to use ADCX.
		// Using regular ADD/ADC would smash both carry flags,
		// so we reject anything we can't handled with ADCX/ADOX.
		if carry&UseCarry != 0 && carry&(SetCarry|SmashCarry) != 0 {
			if carry&AltCarry != 0 {
				a.op3("ADOXQ", src1, src2, dst)
			} else {
				a.op3("ADCXQ", src1, src2, dst)
			}
			return true
		}
		if carry&(SetCarry|UseCarry) == SetCarry && a.IsZero(src1) && src2 == dst {
			// Clearing carry flag. Caller will add EOL comment.
			a.Printf("\tTESTQ AX, AX\n")
			return true
		}
		if carry != KeepCarry {
			a.Fatalf("unsupported carry")
		}
	}
	return false
}

// The x86-prefixed functions are shared with Arch386 in 386.go.

func x86Op3(name string) bool {
	// As far as a.op3 is concerned, there are no 3-op instructions.
	// (We print instructions like MULX ourselves.)
	return false
}

func x86Hint(a *Asm, h Hint) string {
	switch h {
	case HintShiftCount:
		return "CX"
	case HintMulSrc:
		if a.Enabled(OptionAltCarry) { // using MULX
			return "DX"
		}
		return "AX"
	case HintMulHi:
		if a.Enabled(OptionAltCarry) { // using MULX
			return ""
		}
		return "DX"
	}
	return ""
}

func x86Suffix(a *Asm) string {
	// Note: Not using a.Arch == Arch386 to avoid init cycle.
	if a.Arch.Name == "386" {
		return "L"
	}
	return "Q"
}

func x86MulWide(a *Asm, src1, src2, dstlo, dsthi Reg) {
	if a.Enabled(OptionAltCarry) {
		// Using ADCX/ADOX; use MULX to avoid clearing carry flag.
		if src1.name != "DX" {
			if src2.name != "DX" {
				a.Fatalf("mul src1 or src2 must be DX")
			}
			src2 = src1
		}
		a.Printf("\tMULXQ %s, %s, %s\n", src2, dstlo, dsthi)
		return
	}

	if src1.name != "AX" {
		if src2.name != "AX" {
			a.Fatalf("mulwide src1 or src2 must be AX")
		}
		src2 = src1
	}
	if dstlo.name != "AX" {
		a.Fatalf("mulwide dstlo must be AX")
	}
	if dsthi.name != "DX" {
		a.Fatalf("mulwide dsthi must be DX")
	}
	a.Printf("\tMUL%s %s\n", x86Suffix(a), src2)
}

```

// === FILE: references!/go/src/math/big/internal/asmgen/arch.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package asmgen

import (
	"fmt"
	"strings"
)

// Note: Exported fields and methods are expected to be used
// by function generators (like the ones in add.go and so on).
// Unexported fields and methods should not be.

// An Arch defines how to generate assembly for a specific architecture.
type Arch struct {
	Name          string // name of architecture
	Build         string // build tag
	WordBits      int    // length of word in bits (32 or 64)
	WordBytes     int    // length of word in bytes (4 or 8)
	CarrySafeLoop bool   // whether loops preserve carry flag across iterations

	// Registers.
	regs        []string // usable general registers, in allocation order
	reg0        string   // dedicated zero register
	regCarry    string   // dedicated carry register, for systems with no hardware carry bits
	regAltCarry string   // dedicated secondary carry register, for systems with no hardware carry bits
	regTmp      string   // dedicated temporary register

	// regShift indicates that the architecture supports
	// using REG1>>REG2 and REG1<<REG2 as the first source
	// operand in an arithmetic instruction. (32-bit ARM does this.)
	regShift bool

	// setup is called to emit any per-architecture function prologue,
	// immediately after the TEXT line has been emitted.
	// If setup is nil, it is taken to be a no-op.
	setup func(*Func)

	// hint returns the register to use for a given hint.
	// Returning an empty string indicates no preference.
	// If hint is nil, it is considered to return an empty string.
	hint func(*Asm, Hint) string

	// op3 reports whether the named opcode accepts 3 operands
	// (true on most instructions on most systems, but not true of x86 instructions).
	// The assembler unconditionally turns op x,z,z into op x,z.
	// If op3 returns false, then the assembler will turn op x,y,z into mov y,z; op x,z.
	// If op3 is nil, then all opcodes are assumed to accept 3 operands.
	op3 func(name string) bool

	// memOK indicates that arithmetic instructions can use memory references (like on x86)
	memOK bool

	// maxColumns is the default maximum number of vector columns
	// to process in a single [Pipe.Loop] block.
	// 0 means unlimited.
	// [Pipe.SetMaxColumns] overrides this.
	maxColumns int

	// Instruction names.
	mov   string // move (word-sized)
	add   string // add with no carry involvement
	adds  string // add, setting but not using carry
	adc   string // add, using but not setting carry
	adcs  string // add, setting and using carry
	sub   string // sub with no carry involvement
	subs  string // sub, setting but not using carry
	sbc   string // sub, using but not setting carry
	sbcs  string // sub, setting and using carry
	mul   string // multiply
	mulhi string // multiply producing high bits
	lsh   string // left shift
	lshd  string // double-width left shift
	rsh   string // right shift
	rshd  string // double-width right shift
	and   string // bitwise and
	or    string // bitwise or
	xor   string // bitwise xor
	neg   string // negate
	rsb   string // reverse subtract
	sltu  string // set less-than unsigned (dst = src2 < src1), for carry-less systems
	sgtu  string // set greater-than unsigned (dst = src2 > src1), for carry-less systems
	lea   string // load effective address

	// addF and subF implement a.Add and a.Sub
	// on systems where the situation is more complicated than
	// the six basic instructions (add, adds, adcs, sub, subs, sbcs).
	// They return a boolean indicating whether the operation was handled.
	addF func(a *Asm, src1, src2, dst Reg, carry Carry) bool
	subF func(a *Asm, src1, src2, dst Reg, carry Carry) bool

	// mulF and mulWideF implement Mul and MulWide.
	// They call Fatalf if the operation is unsupported.
	// An architecture can set the mul field instead of mulF.
	// mulWide is optional, but otherwise mulhi should be set.
	mulWideF func(a *Asm, src1, src2, dstlo, dsthi Reg)

	// addWords is a printf format taking src1, src2, dst
	// and sets dst = WordBytes*src1+src2.
	// It may modify the carry flag.
	addWords string

	// subCarryIsBorrow is true when the actual processor carry bit used in subtraction
	// is really a “borrow” bit, meaning 1 means borrow and 0 means no borrow.
	// In contrast, most systems (except x86) use a carry bit with the opposite
	// meaning: 0 means a borrow happened, and 1 means it didn't.
	subCarryIsBorrow bool

	// Jump instruction printf formats.
	// jmpZero and jmpNonZero are printf formats taking src, label
	// and jump to label if src is zero / non-zero.
	jmpZero    string
	jmpNonZero string

	// loopTop is a printf format taking src, label that should
	// jump to label if src is zero, or else set up for a loop.
	// If loopTop is not set, jmpZero is used.
	loopTop string

	// loopBottom is a printf format taking dst, label that should
	// decrement dst and then jump to label if src is non-zero.
	// If loopBottom is not set, a subtraction is used followed by
	// use of jmpNonZero.
	loopBottom string

	// loopBottomNeg is like loopBottom but used in negative-index
	// loops, which only happen memIndex is also set (only on 386).
	// It increments dst instead of decrementing it.
	loopBottomNeg string

	// Indexed memory access.
	// If set, memIndex returns a memory reference for a mov instruction
	// addressing off(ptr)(ix*WordBytes).
	// Using memIndex costs an extra register but allows the end-of-loop
	// to do a single increment/decrement instead of advancing two or three pointers.
	// This is particularly important on 386.
	memIndex func(a *Asm, off int, ix Reg, ptr RegPtr) Reg

	// Incrementing/decrementing memory access.
	// loadIncN loads memory at ptr into regs, incrementing ptr by WordBytes after each reg.
	// loadDecN loads memory at ptr into regs, decrementing ptr by WordBytes before each reg.
	// storeIncN and storeDecN are the same, but storing from regs instead of loading into regs.
	// If missing, the assembler accesses memory and advances pointers using separate instructions.
	loadIncN  func(a *Asm, ptr RegPtr, regs []Reg)
	loadDecN  func(a *Asm, ptr RegPtr, regs []Reg)
	storeIncN func(a *Asm, ptr RegPtr, regs []Reg)
	storeDecN func(a *Asm, ptr RegPtr, regs []Reg)

	// options is a map from optional CPU features to functions that test for them.
	// The test function should jump to label if the feature is available.
	options map[Option]func(a *Asm, label string)
}

// HasShiftWide reports whether the Arch has working LshWide/RshWide instructions.
// If not, calling them will panic.
func (a *Arch) HasShiftWide() bool {
	return a.lshd != ""
}

// A Hint is a hint about what a register will be used for,
// so that an appropriate one can be selected.
type Hint uint

const (
	HintNone       Hint = iota
	HintShiftCount      // shift count (CX on x86)
	HintMulSrc          // mul source operand (AX on x86)
	HintMulHi           // wide mul high output (DX on x86)
	HintMemOK           // a memory reference is okay
	HintCarry           // carry flag
	HintAltCarry        // secondary carry flag
)

// A Reg is an allocated register or other assembly operand.
// (For example, a constant might have name "$123"
// and a memory reference might have name "0(R8)".)
type Reg struct{ name string }

// IsImm reports whether r is an immediate value.
func (r Reg) IsImm() bool { return strings.HasPrefix(r.name, "$") }

// IsMem reports whether r is a memory value.
func (r Reg) IsMem() bool { return strings.HasSuffix(r.name, ")") }

// String returns the assembly syntax for r.
func (r Reg) String() string { return r.name }

// Valid reports whether is valid, meaning r is not the zero value of Reg (a register with no name).
func (r Reg) Valid() bool { return r.name != "" }

// A RegPtr is like a Reg but expected to hold a pointer.
// The separate Go type helps keeps pointers and scalars separate and avoid mistakes;
// it is okay to convert to Reg as needed to use specific routines.
type RegPtr struct{ name string }

// String returns the assembly syntax for r.
func (r RegPtr) String() string { return r.name }

// Valid reports whether is valid, meaning r is not the zero value of RegPtr (a register with no name).
func (r RegPtr) Valid() bool { return r.name != "" }

// mem returns a memory reference to off bytes from the pointer r.
func (r *RegPtr) mem(off int) Reg { return Reg{fmt.Sprintf("%d(%s)", off, r)} }

// A Carry is a flag field explaining how an instruction sets and uses the carry flags.
// Different operations expect different sets of bits.
// Add and Sub expect: UseCarry or 0, SetCarry, KeepCarry, or SmashCarry; and AltCarry or 0.
// ClearCarry, SaveCarry, and ConvertCarry expect: AddCarry or SubCarry; and AltCarry or 0.
type Carry uint

const (
	SetCarry   Carry = 1 << iota // sets carry
	UseCarry                     // uses carry
	KeepCarry                    // must preserve carry
	SmashCarry                   // can modify carry or not, whatever is easiest

	AltCarry // use the secondary carry flag
	AddCarry // use add carry flag semantics (for ClearCarry, ConvertCarry)
	SubCarry // use sub carry flag semantics (for ClearCarry, ConvertCarry)
)

// An Option denotes an optional CPU feature that can be tested at runtime.
type Option int

const (
	_ Option = iota

	// OptionAltCarry checks whether there is an add instruction
	// that uses a secondary carry flag, so that two different sums
	// can be accumulated in parallel with independent carry flags.
	// Some architectures (MIPS, Loong64, RISC-V) provide this
	// functionality natively, indicated by asm.Carry().Valid() being true.
	OptionAltCarry
)

```

// === FILE: references!/go/src/math/big/internal/asmgen/arm.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package asmgen

var ArchARM = &Arch{
	Name:          "arm",
	WordBits:      32,
	WordBytes:     4,
	CarrySafeLoop: true,

	regs: []string{
		// R10 is g.
		// R11 is the assembler/linker temporary (but we use it as a regular register).
		// R13 is SP.
		// R14 is LR.
		// R15 is PC.
		"R0", "R1", "R2", "R3", "R4", "R5", "R6", "R7", "R8", "R9", "R11", "R12",
	},
	regShift: true,

	mov:  "MOVW",
	add:  "ADD",
	adds: "ADD.S",
	adc:  "ADC",
	adcs: "ADC.S",
	sub:  "SUB",
	subs: "SUB.S",
	sbc:  "SBC",
	sbcs: "SBC.S",
	rsb:  "RSB",
	and:  "AND",
	or:   "ORR",
	xor:  "EOR",

	mulWideF: armMulWide,

	addWords: "ADD %s<<2, %s, %s",

	jmpZero:    "TEQ $0, %s; BEQ %s",
	jmpNonZero: "TEQ $0, %s; BNE %s",

	loadIncN:  armLoadIncN,
	loadDecN:  armLoadDecN,
	storeIncN: armStoreIncN,
	storeDecN: armStoreDecN,
}

func armMulWide(a *Asm, src1, src2, dstlo, dsthi Reg) {
	a.Printf("\tMULLU %s, %s, (%s, %s)\n", src1, src2, dsthi, dstlo)
}

func armLoadIncN(a *Asm, p RegPtr, regs []Reg) {
	for _, r := range regs {
		a.Printf("\tMOVW.P %d(%s), %s\n", a.Arch.WordBytes, p, r)
	}
}

func armLoadDecN(a *Asm, p RegPtr, regs []Reg) {
	for _, r := range regs {
		a.Printf("\tMOVW.W %d(%s), %s\n", -a.Arch.WordBytes, p, r)
	}
}

func armStoreIncN(a *Asm, p RegPtr, regs []Reg) {
	for _, r := range regs {
		a.Printf("\tMOVW.P %s, %d(%s)\n", r, a.Arch.WordBytes, p)
	}
}

func armStoreDecN(a *Asm, p RegPtr, regs []Reg) {
	for _, r := range regs {
		a.Printf("\tMOVW.W %s, %d(%s)\n", r, -a.Arch.WordBytes, p)
	}
}

```

// === FILE: references!/go/src/math/big/internal/asmgen/arm64.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package asmgen

var ArchARM64 = &Arch{
	Name:          "arm64",
	WordBits:      64,
	WordBytes:     8,
	CarrySafeLoop: true,

	regs: []string{
		// R18 is the platform register.
		// R27 is the assembler/linker temporary (which we could potentially use but don't).
		// R28 is g.
		// R29 is FP.
		// R30 is LR.
		"R0", "R1", "R2", "R3", "R4", "R5", "R6", "R7", "R8", "R9",
		"R10", "R11", "R12", "R13", "R14", "R15", "R16", "R17", "R19",
		"R20", "R21", "R22", "R23", "R24", "R25", "R26",
	},
	reg0: "ZR",

	mov:   "MOVD",
	add:   "ADD",
	adds:  "ADDS",
	adc:   "ADC",
	adcs:  "ADCS",
	sub:   "SUB",
	subs:  "SUBS",
	sbc:   "SBC",
	sbcs:  "SBCS",
	mul:   "MUL",
	mulhi: "UMULH",
	lsh:   "LSL",
	rsh:   "LSR",
	and:   "AND",
	or:    "ORR",
	xor:   "EOR",

	addWords: "ADD %[1]s<<3, %[2]s, %[3]s",

	jmpZero:    "CBZ %s, %s",
	jmpNonZero: "CBNZ %s, %s",

	loadIncN:  arm64LoadIncN,
	loadDecN:  arm64LoadDecN,
	storeIncN: arm64StoreIncN,
	storeDecN: arm64StoreDecN,
}

func arm64LoadIncN(a *Asm, p RegPtr, regs []Reg) {
	if len(regs) == 1 {
		a.Printf("\tMOVD.P %d(%s), %s\n", a.Arch.WordBytes, p, regs[0])
		return
	}
	a.Printf("\tLDP.P %d(%s), (%s, %s)\n", len(regs)*a.Arch.WordBytes, p, regs[0], regs[1])
	var i int
	for i = 2; i+2 <= len(regs); i += 2 {
		a.Printf("\tLDP %d(%s), (%s, %s)\n", (i-len(regs))*a.Arch.WordBytes, p, regs[i], regs[i+1])
	}
	if i < len(regs) {
		a.Printf("\tMOVD %d(%s), %s\n", -1*a.Arch.WordBytes, p, regs[i])
	}
}

func arm64LoadDecN(a *Asm, p RegPtr, regs []Reg) {
	if len(regs) == 1 {
		a.Printf("\tMOVD.W -%d(%s), %s\n", a.Arch.WordBytes, p, regs[0])
		return
	}
	a.Printf("\tLDP.W %d(%s), (%s, %s)\n", -len(regs)*a.Arch.WordBytes, p, regs[len(regs)-1], regs[len(regs)-2])
	var i int
	for i = 2; i+2 <= len(regs); i += 2 {
		a.Printf("\tLDP %d(%s), (%s, %s)\n", i*a.Arch.WordBytes, p, regs[len(regs)-1-i], regs[len(regs)-2-i])
	}
	if i < len(regs) {
		a.Printf("\tMOVD %d(%s), %s\n", i*a.Arch.WordBytes, p, regs[0])
	}
}

func arm64StoreIncN(a *Asm, p RegPtr, regs []Reg) {
	if len(regs) == 1 {
		a.Printf("\tMOVD.P %s, %d(%s)\n", regs[0], a.Arch.WordBytes, p)
		return
	}
	a.Printf("\tSTP.P (%s, %s), %d(%s)\n", regs[0], regs[1], len(regs)*a.Arch.WordBytes, p)
	var i int
	for i = 2; i+2 <= len(regs); i += 2 {
		a.Printf("\tSTP (%s, %s), %d(%s)\n", regs[i], regs[i+1], (i-len(regs))*a.Arch.WordBytes, p)
	}
	if i < len(regs) {
		a.Printf("\tMOVD %s, %d(%s)\n", regs[i], -1*a.Arch.WordBytes, p)
	}
}

func arm64StoreDecN(a *Asm, p RegPtr, regs []Reg) {
	if len(regs) == 1 {
		a.Printf("\tMOVD.W %s, -%d(%s)\n", regs[0], a.Arch.WordBytes, p)
		return
	}
	a.Printf("\tSTP.W (%s, %s), %d(%s)\n", regs[len(regs)-1], regs[len(regs)-2], -len(regs)*a.Arch.WordBytes, p)
	var i int
	for i = 2; i+2 <= len(regs); i += 2 {
		a.Printf("\tSTP (%s, %s), %d(%s)\n", regs[len(regs)-1-i], regs[len(regs)-2-i], i*a.Arch.WordBytes, p)
	}
	if i < len(regs) {
		a.Printf("\tMOVD %s, %d(%s)\n", regs[0], i*a.Arch.WordBytes, p)
	}
}

```

// === FILE: references!/go/src/math/big/internal/asmgen/asm.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package asmgen

import (
	"bytes"
	"cmp"
	"fmt"
	"math/bits"
	"slices"
	"strings"
)

// Note: Exported fields and methods are expected to be used
// by function generators (like the ones in add.go and so on).
// Unexported fields and methods should not be.

// An Asm is an assembly file being written.
type Asm struct {
	Arch     *Arch           // architecture
	out      bytes.Buffer    // output buffer
	regavail uint64          // bitmap of available registers
	enabled  map[Option]bool // enabled optional CPU features
}

// NewAsm returns a new Asm preparing assembly
// for the given architecture to be written to file.
func NewAsm(arch *Arch) *Asm {
	a := &Asm{Arch: arch, enabled: make(map[Option]bool)}
	buildTag := ""
	if arch.Build != "" {
		buildTag = " && (" + arch.Build + ")"
	}
	a.Printf(asmHeader, buildTag)
	return a
}

// Note: Using Copyright 2025, not the current year, to avoid test failures
// on January 1 and spurious diffs when regenerating assembly.
// The generator was written in 2025; that's good enough.
// (As a matter of policy the Go project does not update copyright
// notices every year, since copyright terms are so long anyway.)

var asmHeader = `// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by 'go generate' (with ./internal/asmgen). DO NOT EDIT.

//go:build !math_big_pure_go%s

#include "textflag.h"
`

// Fatalf reports a fatal error by panicking.
// Panicking is appropriate because there is a bug in the generator,
// and panicking will show the exact source lines leading to that bug.
func (a *Asm) Fatalf(format string, args ...any) {
	text := a.out.String()
	i := strings.LastIndex(text, "\nTEXT")
	text = text[i+1:]
	panic("[" + a.Arch.Name + "] asmgen internal error: " + fmt.Sprintf(format, args...) + "\n" + text)
}

// hint returns the register name for the given hint.
func (a *Asm) hint(h Hint) string {
	if h == HintCarry && a.Arch.regCarry != "" {
		return a.Arch.regCarry
	}
	if h == HintAltCarry && a.Arch.regAltCarry != "" {
		return a.Arch.regAltCarry
	}
	if h == HintNone || a.Arch.hint == nil {
		return ""
	}
	return a.Arch.hint(a, h)
}

// ZR returns the zero register (the specific register guaranteed to hold the integer 0),
// or else the zero Reg (Reg{}, which has r.Valid() == false).
func (a *Asm) ZR() Reg {
	return Reg{a.Arch.reg0}
}

// tmp returns the temporary register, or else the zero Reg.
// The temporary register is one available for use implementing logical instructions
// that compile into multiple actual instructions on a given system.
// The assembler sometimes uses it for that purpose, as do we.
// Of course, if we are using it, we'd better not emit an instruction that
// will cause the assembler to smash it while we want it to be holding
// a live value. In general it is the architecture implementation's responsibility
// not to suggest the use of any such pseudo-instructions in situations
// where they would cause problems.
func (a *Asm) tmp() Reg {
	return Reg{a.Arch.regTmp}
}

// Carry returns the carry register, or else the zero Reg.
func (a *Asm) Carry() Reg {
	return Reg{a.Arch.regCarry}
}

// AltCarry returns the secondary carry register, or else the zero Reg.
func (a *Asm) AltCarry() Reg {
	return Reg{a.Arch.regAltCarry}
}

// Imm returns a Reg representing an immediate (constant) value.
func (a *Asm) Imm(x int) Reg {
	if x == 0 && a.Arch.reg0 != "" {
		return Reg{a.Arch.reg0}
	}
	return Reg{fmt.Sprintf("$%d", x)}
}

// IsZero reports whether r is a zero immediate or the zero register.
func (a *Asm) IsZero(r Reg) bool {
	return r.name == "$0" || a.Arch.reg0 != "" && r.name == a.Arch.reg0
}

// Reg allocates a new register.
func (a *Asm) Reg() Reg {
	i := bits.TrailingZeros64(a.regavail)
	if i == 64 {
		a.Fatalf("out of registers")
	}
	a.regavail ^= 1 << i
	return Reg{a.Arch.regs[i]}
}

// RegHint allocates a new register, with a hint as to its purpose.
func (a *Asm) RegHint(hint Hint) Reg {
	if name := a.hint(hint); name != "" {
		i := slices.Index(a.Arch.regs, name)
		if i < 0 {
			return Reg{name}
		}
		if a.regavail&(1<<i) == 0 {
			a.Fatalf("hint for already allocated register %s", name)
		}
		a.regavail &^= 1 << i
		return Reg{name}
	}
	return a.Reg()
}

// Free frees a previously allocated register.
// If r is not a register (if it's an immediate or a memory reference), Free is a no-op.
func (a *Asm) Free(r Reg) {
	i := slices.Index(a.Arch.regs, r.name)
	if i < 0 {
		return
	}
	if a.regavail&(1<<i) != 0 {
		a.Fatalf("register %s already freed", r.name)
	}
	a.regavail |= 1 << i
}

// Unfree reallocates a previously freed register r.
// If r is not a register (if it's an immediate or a memory reference), Unfree is a no-op.
// If r is not free for allocation, Unfree panics.
// A Free paired with Unfree can release a register for use temporarily
// but then reclaim it, such as at the end of a loop body when it must be restored.
func (a *Asm) Unfree(r Reg) {
	i := slices.Index(a.Arch.regs, r.name)
	if i < 0 {
		return
	}
	if a.regavail&(1<<i) == 0 {
		a.Fatalf("register %s not free", r.name)
	}
	a.regavail &^= 1 << i
}

// A RegsUsed is a snapshot of which registers are allocated.
type RegsUsed struct {
	avail uint64
}

// RegsUsed returns a snapshot of which registers are currently allocated,
// which can be passed to a future call to [Asm.SetRegsUsed].
func (a *Asm) RegsUsed() RegsUsed {
	return RegsUsed{a.regavail}
}

// SetRegsUsed sets which registers are currently allocated.
// The argument should have been returned from a previous
// call to [Asm.RegsUsed].
func (a *Asm) SetRegsUsed(used RegsUsed) {
	a.regavail = used.avail
}

// FreeAll frees all known registers.
func (a *Asm) FreeAll() {
	a.regavail = 1<<len(a.Arch.regs) - 1
}

// Printf emits to the assembly output.
func (a *Asm) Printf(format string, args ...any) {
	text := fmt.Sprintf(format, args...)
	if strings.Contains(text, "%!") {
		a.Fatalf("printf error: %s", text)
	}
	a.out.WriteString(text)
}

// Comment emits a line comment to the assembly output.
func (a *Asm) Comment(format string, args ...any) {
	fmt.Fprintf(&a.out, "\t// %s\n", fmt.Sprintf(format, args...))
}

// EOL appends an end-of-line comment to the previous line.
func (a *Asm) EOL(format string, args ...any) {
	bytes := a.out.Bytes()
	if len(bytes) > 0 && bytes[len(bytes)-1] == '\n' {
		a.out.Truncate(a.out.Len() - 1)
	}
	a.Comment(format, args...)
}

// JmpEnable emits a test for the optional CPU feature that jumps to label if the feature is present.
// If JmpEnable returns false, the feature is not available on this architecture and no code was emitted.
func (a *Asm) JmpEnable(option Option, label string) bool {
	jmpEnable := a.Arch.options[option]
	if jmpEnable == nil {
		return false
	}
	jmpEnable(a, label)
	return true
}

// Enabled reports whether the optional CPU feature is considered
// to be enabled at this point in the assembly output.
func (a *Asm) Enabled(option Option) bool {
	return a.enabled[option]
}

// SetOption changes whether the optional CPU feature should be
// considered to be enabled.
func (a *Asm) SetOption(option Option, on bool) {
	a.enabled[option] = on
}

// op3 emits a 3-operand instruction op src1, src2, dst,
// taking care to handle 2-operand machines and also
// to simplify the printout when src2==dst.
func (a *Asm) op3(op string, src1, src2, dst Reg) {
	if op == "" {
		a.Fatalf("missing instruction")
	}
	if src2 == dst {
		// src2 and dst are same; print as 2-op form.
		a.Printf("\t%s %s, %s\n", op, src1, dst)
	} else if a.Arch.op3 != nil && !a.Arch.op3(op) {
		// Machine does not have 3-op form for op; convert to 2-op.
		if src1 == dst {
			a.Fatalf("implicit mov %s, %s would smash src1", src2, dst)
		}
		a.Mov(src2, dst)
		a.Printf("\t%s %s, %s\n", op, src1, dst)
	} else {
		// Full 3-op form.
		a.Printf("\t%s %s, %s, %s\n", op, src1, src2, dst)
	}
}

// Mov emits dst = src.
func (a *Asm) Mov(src, dst Reg) {
	if src != dst {
		a.Printf("\t%s %s, %s\n", a.Arch.mov, src, dst)
	}
}

// AddWords emits dst = src1*WordBytes + src2.
// It does not set or use the carry flag.
func (a *Asm) AddWords(src1 Reg, src2, dst RegPtr) {
	if a.Arch.addWords == "" {
		// Note: Assuming that Lsh does not clobber the carry flag.
		// Architectures where this is not true (x86) need to provide Arch.addWords.
		t := a.Reg()
		a.Lsh(a.Imm(bits.TrailingZeros(uint(a.Arch.WordBytes))), src1, t)
		a.Add(t, Reg(src2), Reg(dst), KeepCarry)
		a.Free(t)
		return
	}
	a.Printf("\t"+a.Arch.addWords+"\n", src1, src2, dst)
}

// And emits dst = src1 & src2
// It may modify the carry flag.
func (a *Asm) And(src1, src2, dst Reg) {
	a.op3(a.Arch.and, src1, src2, dst)
}

// Or emits dst = src1 | src2
// It may modify the carry flag.
func (a *Asm) Or(src1, src2, dst Reg) {
	a.op3(a.Arch.or, src1, src2, dst)
}

// Xor emits dst = src1 ^ src2
// It may modify the carry flag.
func (a *Asm) Xor(src1, src2, dst Reg) {
	a.op3(a.Arch.xor, src1, src2, dst)
}

// Neg emits dst = -src.
// It may modify the carry flag.
func (a *Asm) Neg(src, dst Reg) {
	if a.Arch.neg == "" {
		if a.Arch.rsb != "" {
			a.Printf("\t%s $0, %s, %s\n", a.Arch.rsb, src, dst)
			return
		}
		if a.Arch.sub != "" && a.Arch.reg0 != "" {
			a.Printf("\t%s %s, %s, %s\n", a.Arch.sub, src, a.Arch.reg0, dst)
			return
		}
		a.Fatalf("missing neg")
	}
	if src == dst {
		a.Printf("\t%s %s\n", a.Arch.neg, dst)
	} else {
		a.Printf("\t%s %s, %s\n", a.Arch.neg, src, dst)
	}
}

// HasRegShift reports whether the architecture can use shift expressions as operands.
func (a *Asm) HasRegShift() bool {
	return a.Arch.regShift
}

// LshReg returns a shift-expression operand src<<shift.
// If a.HasRegShift() == false, LshReg panics.
func (a *Asm) LshReg(shift, src Reg) Reg {
	if !a.HasRegShift() {
		a.Fatalf("no reg shift")
	}
	return Reg{fmt.Sprintf("%s<<%s", src, strings.TrimPrefix(shift.name, "$"))}
}

// Lsh emits dst = src << shift.
// It may modify the carry flag.
func (a *Asm) Lsh(shift, src, dst Reg) {
	if need := a.hint(HintShiftCount); need != "" && shift.name != need && !shift.IsImm() {
		a.Fatalf("shift count not in %s", need)
	}
	if a.HasRegShift() {
		a.Mov(a.LshReg(shift, src), dst)
		return
	}
	a.op3(a.Arch.lsh, shift, src, dst)
}

// LshWide emits dst = src << shift with low bits shifted from adj.
// It may modify the carry flag.
func (a *Asm) LshWide(shift, adj, src, dst Reg) {
	if a.Arch.lshd == "" {
		a.Fatalf("no lshwide on %s", a.Arch.Name)
	}
	if need := a.hint(HintShiftCount); need != "" && shift.name != need && !shift.IsImm() {
		a.Fatalf("shift count not in %s", need)
	}
	a.op3(fmt.Sprintf("%s %s,", a.Arch.lshd, shift), adj, src, dst)
}

// RshReg returns a shift-expression operand src>>shift.
// If a.HasRegShift() == false, RshReg panics.
func (a *Asm) RshReg(shift, src Reg) Reg {
	if !a.HasRegShift() {
		a.Fatalf("no reg shift")
	}
	return Reg{fmt.Sprintf("%s>>%s", src, strings.TrimPrefix(shift.name, "$"))}
}

// Rsh emits dst = src >> shift.
// It may modify the carry flag.
func (a *Asm) Rsh(shift, src, dst Reg) {
	if need := a.hint(HintShiftCount); need != "" && shift.name != need && !shift.IsImm() {
		a.Fatalf("shift count not in %s", need)
	}
	if a.HasRegShift() {
		a.Mov(a.RshReg(shift, src), dst)
		return
	}
	a.op3(a.Arch.rsh, shift, src, dst)
}

// RshWide emits dst = src >> shift with high bits shifted from adj.
// It may modify the carry flag.
func (a *Asm) RshWide(shift, adj, src, dst Reg) {
	if a.Arch.lshd == "" {
		a.Fatalf("no rshwide on %s", a.Arch.Name)
	}
	if need := a.hint(HintShiftCount); need != "" && shift.name != need && !shift.IsImm() {
		a.Fatalf("shift count not in %s", need)
	}
	a.op3(fmt.Sprintf("%s %s,", a.Arch.rshd, shift), adj, src, dst)
}

// SLTU emits dst = src2 < src1 (0 or 1), using an unsigned comparison.
func (a *Asm) SLTU(src1, src2, dst Reg) {
	switch {
	default:
		a.Fatalf("arch has no sltu/sgtu")
	case a.Arch.sltu != "":
		a.Printf("\t%s %s, %s, %s\n", a.Arch.sltu, src1, src2, dst)
	case a.Arch.sgtu != "":
		a.Printf("\t%s %s, %s, %s\n", a.Arch.sgtu, src2, src1, dst)
	}
}

// Add emits dst = src1+src2, with the specified carry behavior.
func (a *Asm) Add(src1, src2, dst Reg, carry Carry) {
	switch {
	default:
		a.Fatalf("unsupported carry behavior")
	case a.Arch.addF != nil && a.Arch.addF(a, src1, src2, dst, carry):
		// handled
	case a.Arch.add != "" && (carry == KeepCarry || carry == SmashCarry):
		a.op3(a.Arch.add, src1, src2, dst)
	case a.Arch.adds != "" && (carry == SetCarry || carry == SmashCarry):
		a.op3(a.Arch.adds, src1, src2, dst)
	case a.Arch.adc != "" && (carry == UseCarry || carry == UseCarry|SmashCarry):
		a.op3(a.Arch.adc, src1, src2, dst)
	case a.Arch.adcs != "" && (carry == UseCarry|SetCarry || carry == UseCarry|SmashCarry):
		a.op3(a.Arch.adcs, src1, src2, dst)
	case a.Arch.lea != "" && (carry == KeepCarry || carry == SmashCarry):
		if src1.IsImm() {
			a.Printf("\t%s %s(%s), %s\n", a.Arch.lea, src1.name[1:], src2, dst) // name[1:] removes $
		} else {
			a.Printf("\t%s (%s)(%s), %s\n", a.Arch.lea, src1, src2, dst)
		}
		if src2 == dst {
			a.EOL("ADD %s, %s", src1, dst)
		} else {
			a.EOL("ADD %s, %s, %s", src1, src2, dst)
		}

	case a.Arch.add != "" && a.Arch.regCarry != "":
		// Machine has no carry flag; instead we've dedicated a register
		// and use SLTU/SGTU (set less-than/greater-than unsigned)
		// to compute the carry flags as needed.
		// For ADD x, y, z, SLTU x/y, z, c computes the carry (borrow) bit.
		// Either of x or y can be used as the second argument, provided
		// it is not aliased to z.
		// To make the output less of a wall of instructions,
		// we comment the “higher-level” operation, with ... marking
		// continued instructions implementing the operation.
		cr := a.Carry()
		if carry&AltCarry != 0 {
			cr = a.AltCarry()
			if !cr.Valid() {
				a.Fatalf("alt carry not supported")
			}
			carry &^= AltCarry
		}
		tmp := a.tmp()
		if !tmp.Valid() {
			a.Fatalf("cannot simulate sub carry without regTmp")
		}
		switch carry {
		default:
			a.Fatalf("unsupported carry behavior")
		case UseCarry, UseCarry | SmashCarry:
			// Easy case, just add the carry afterward.
			if a.IsZero(src1) {
				// Only here to use the carry.
				a.Add(cr, src2, dst, KeepCarry)
				a.EOL("ADC $0, %s, %s", src2, dst)
				break
			}
			a.Add(src1, src2, dst, KeepCarry)
			a.EOL("ADC %s, %s, %s (cr=%s)", src1, src2, dst, cr)
			a.Add(cr, dst, dst, KeepCarry)
			a.EOL("...")

		case SetCarry:
			if a.IsZero(src1) && src2 == dst {
				// Only here to clear the carry flag. (Caller will comment.)
				a.Xor(cr, cr, cr)
				break
			}
			var old Reg // old is a src distinct from dst
			switch {
			case dst != src1:
				old = src1
			case dst != src2:
				old = src2
			default:
				// src1 == src2 == dst.
				// Overflows if and only if the high bit is set, so copy high bit to carry.
				a.Rsh(a.Imm(a.Arch.WordBits-1), src1, cr)
				a.EOL("ADDS %s, %s, %s (cr=%s)", src1, src2, dst, cr)
				a.Add(src1, src2, dst, KeepCarry)
				a.EOL("...")
				return
			}
			a.Add(src1, src2, dst, KeepCarry)
			a.EOL("ADDS %s, %s, %s (cr=%s)", src1, src2, dst, cr)
			a.SLTU(old, dst, cr) // dst < old (one of the src) implies carry
			a.EOL("...")

		case UseCarry | SetCarry:
			if a.IsZero(src1) {
				// Only here to use and then set the carry.
				// Easy since carry is not aliased to dst.
				a.Add(cr, src2, dst, KeepCarry)
				a.EOL("ADCS $0, %s, %s (cr=%s)", src2, dst, cr)
				a.SLTU(cr, dst, cr) // dst < cr implies carry
				a.EOL("...")
				break
			}
			// General case. Need to do two different adds (src1 + src2 + cr),
			// computing carry bits for both, and add'ing them together.
			// Start with src1+src2.
			var old Reg // old is a src distinct from dst
			switch {
			case dst != src1:
				old = src1
			case dst != src2:
				old = src2
			}
			if old.Valid() {
				a.Add(src1, src2, dst, KeepCarry)
				a.EOL("ADCS %s, %s, %s (cr=%s)", src1, src2, dst, cr)
				a.SLTU(old, dst, tmp) // // dst < old (one of the src) implies carry
				a.EOL("...")
			} else {
				// src1 == src2 == dst, like above. Sign bit is carry bit,
				// but we copy it into tmp, not cr.
				a.Rsh(a.Imm(a.Arch.WordBits-1), src1, tmp)
				a.EOL("ADCS %s, %s, %s (cr=%s)", src1, src2, dst, cr)
				a.Add(src1, src2, dst, KeepCarry)
				a.EOL("...")
			}
			// Add cr to dst.
			a.Add(cr, dst, dst, KeepCarry)
			a.EOL("...")
			a.SLTU(cr, dst, cr) // sum < cr implies carry
			a.EOL("...")
			// Add the two carry bits (at most one can be set, because (2⁶⁴-1)+(2⁶⁴-1)+1 < 2·2⁶⁴).
			a.Add(tmp, cr, cr, KeepCarry)
			a.EOL("...")
		}
	}
}

// Sub emits dst = src2-src1, with the specified carry behavior.
func (a *Asm) Sub(src1, src2, dst Reg, carry Carry) {
	switch {
	default:
		a.Fatalf("unsupported carry behavior")
	case a.Arch.subF != nil && a.Arch.subF(a, src1, src2, dst, carry):
		// handled
	case a.Arch.sub != "" && (carry == KeepCarry || carry == SmashCarry):
		a.op3(a.Arch.sub, src1, src2, dst)
	case a.Arch.subs != "" && (carry == SetCarry || carry == SmashCarry):
		a.op3(a.Arch.subs, src1, src2, dst)
	case a.Arch.sbc != "" && (carry == UseCarry || carry == UseCarry|SmashCarry):
		a.op3(a.Arch.sbc, src1, src2, dst)
	case a.Arch.sbcs != "" && (carry == UseCarry|SetCarry || carry == UseCarry|SmashCarry):
		a.op3(a.Arch.sbcs, src1, src2, dst)
	case strings.HasPrefix(src1.name, "$") && (carry == KeepCarry || carry == SmashCarry):
		// Running out of options; if this is an immediate
		// and we don't need to worry about carry semantics,
		// try adding the negation.
		if strings.HasPrefix(src1.name, "$-") {
			src1.name = "$" + src1.name[2:]
		} else {
			src1.name = "$-" + src1.name[1:]
		}
		a.Add(src1, src2, dst, carry)

	case a.Arch.sub != "" && a.Arch.regCarry != "":
		// Machine has no carry flag; instead we've dedicated a register
		// and use SLTU/SGTU (set less-than/greater-than unsigned)
		// to compute the carry bits as needed.
		// For SUB x, y, z, SLTU x, y, c computes the carry (borrow) bit.
		// To make the output less of a wall of instructions,
		// we comment the “higher-level” operation, with ... marking
		// continued instructions implementing the operation.
		// Be careful! Subtract and add have different overflow behaviors,
		// so the details here are NOT the same as in Add above.
		cr := a.Carry()
		if carry&AltCarry != 0 {
			a.Fatalf("alt carry not supported")
		}
		tmp := a.tmp()
		if !tmp.Valid() {
			a.Fatalf("cannot simulate carry without regTmp")
		}
		switch carry {
		default:
			a.Fatalf("unsupported carry behavior")
		case UseCarry, UseCarry | SmashCarry:
			// Easy case, just subtract the carry afterward.
			if a.IsZero(src1) {
				// Only here to use the carry.
				a.Sub(cr, src2, dst, KeepCarry)
				a.EOL("SBC $0, %s, %s", src2, dst)
				break
			}
			a.Sub(src1, src2, dst, KeepCarry)
			a.EOL("SBC %s, %s, %s", src1, src2, dst)
			a.Sub(cr, dst, dst, KeepCarry)
			a.EOL("...")

		case SetCarry:
			if a.IsZero(src1) && src2 == dst {
				// Only here to clear the carry flag.
				a.Xor(cr, cr, cr)
				break
			}
			// Compute the new carry first, in case dst is src1 or src2.
			a.SLTU(src1, src2, cr)
			a.EOL("SUBS %s, %s, %s", src1, src2, dst)
			a.Sub(src1, src2, dst, KeepCarry)
			a.EOL("...")

		case UseCarry | SetCarry:
			if a.IsZero(src1) {
				// Only here to use and then set the carry.
				if src2 == dst {
					// Unfortunate case. Using src2==dst is common (think x -= y)
					// and also more efficient on two-operand machines (like x86),
					// but here subtracting from dst will smash src2, making it
					// impossible to recover the carry information after the SUB.
					// But we want to use the carry, so we can't compute it before
					// the SUB either. Compute into a temporary and MOV.
					a.SLTU(cr, src2, tmp)
					a.EOL("SBCS $0, %s, %s", src2, dst)
					a.Sub(cr, src2, dst, KeepCarry)
					a.EOL("...")
					a.Mov(tmp, cr)
					a.EOL("...")
					break
				}
				a.Sub(cr, src2, dst, KeepCarry) // src2 not dst, so src2 preserved
				a.SLTU(cr, src2, cr)
				break
			}
			// General case. Need to do two different subtracts (src2 - cr - src1),
			// computing carry bits for both, and add'ing them together.
			// Doing src2 - cr first frees up cr to store the carry from the sub of src1.
			a.SLTU(cr, src2, tmp)
			a.EOL("SBCS %s, %s, %s", src1, src2, dst)
			a.Sub(cr, src2, dst, KeepCarry)
			a.EOL("...")
			a.SLTU(src1, dst, cr)
			a.EOL("...")
			a.Sub(src1, dst, dst, KeepCarry)
			a.EOL("...")
			a.Add(tmp, cr, cr, KeepCarry)
			a.EOL("...")
		}
	}
}

// ClearCarry clears the carry flag.
// The ‘which’ parameter must be AddCarry or SubCarry to specify how the flag will be used.
// (On some systems, the sub carry's actual processor bit is inverted from its usual value.)
func (a *Asm) ClearCarry(which Carry) {
	dst := Reg{a.Arch.regs[0]} // not actually modified
	switch which & (AddCarry | SubCarry) {
	default:
		a.Fatalf("bad carry")
	case AddCarry:
		a.Add(a.Imm(0), dst, dst, SetCarry|which&AltCarry)
	case SubCarry:
		a.Sub(a.Imm(0), dst, dst, SetCarry|which&AltCarry)
	}
	a.EOL("clear carry")
}

// SaveCarry saves the carry flag into dst.
// The meaning of the bits in dst is architecture-dependent.
// The carry flag is left in an undefined state.
func (a *Asm) SaveCarry(dst Reg) {
	// Note: As implemented here, the carry flag is actually left unmodified,
	// but we say it is in an undefined state in case that changes in the future.
	// (The SmashCarry could be changed to SetCarry if so.)
	if cr := a.Carry(); cr.Valid() {
		if cr == dst {
			return // avoid EOL
		}
		a.Mov(cr, dst)
	} else {
		a.Sub(dst, dst, dst, UseCarry|SmashCarry)
	}
	a.EOL("save carry")
}

// RestoreCarry restores the carry flag from src.
// src is left in an undefined state.
func (a *Asm) RestoreCarry(src Reg) {
	if cr := a.Carry(); cr.Valid() {
		if cr == src {
			return // avoid EOL
		}
		a.Mov(src, cr)
	} else if a.Arch.subCarryIsBorrow {
		a.Add(src, src, src, SetCarry)
	} else {
		// SaveCarry saved the sub carry flag with an encoding of 0, 1 -> 0, ^0.
		// Restore it by subtracting from a value less than ^0, which will carry if src != 0.
		// If there is no zero register, the SP register is guaranteed to be less than ^0.
		// (This may seem too clever, but on GOARCH=arm we have no other good options.)
		a.Sub(src, cmp.Or(a.ZR(), Reg{"SP"}), src, SetCarry)
	}
	a.EOL("restore carry")
}

// ConvertCarry converts the carry flag in dst from the internal format to a 0 or 1.
// The carry flag is left in an undefined state.
func (a *Asm) ConvertCarry(which Carry, dst Reg) {
	if a.Carry().Valid() { // already 0 or 1
		return
	}
	switch which {
	case AddCarry:
		if a.Arch.subCarryIsBorrow {
			a.Neg(dst, dst)
		} else {
			a.Add(a.Imm(1), dst, dst, SmashCarry)
		}
		a.EOL("convert add carry")
	case SubCarry:
		a.Neg(dst, dst)
		a.EOL("convert sub carry")
	}
}

// SaveConvertCarry saves and converts the carry flag into dst: 0 unset, 1 set.
// The carry flag is left in an undefined state.
func (a *Asm) SaveConvertCarry(which Carry, dst Reg) {
	switch which {
	default:
		a.Fatalf("bad carry")
	case AddCarry:
		if (a.Arch.adc != "" || a.Arch.adcs != "") && a.ZR().Valid() {
			a.Add(a.ZR(), a.ZR(), dst, UseCarry|SmashCarry)
			a.EOL("save & convert add carry")
			return
		}
	case SubCarry:
		// no special cases
	}
	a.SaveCarry(dst)
	a.ConvertCarry(which, dst)
}

// MulWide emits dstlo = src1 * src2 and dsthi = (src1 * src2) >> WordBits.
// The carry flag is left in an undefined state.
// If dstlo or dsthi is the zero Reg, then those outputs are discarded.
func (a *Asm) MulWide(src1, src2, dstlo, dsthi Reg) {
	switch {
	default:
		a.Fatalf("mulwide not available")
	case a.Arch.mulWideF != nil:
		a.Arch.mulWideF(a, src1, src2, dstlo, dsthi)
	case a.Arch.mul != "" && !dsthi.Valid():
		a.op3(a.Arch.mul, src1, src2, dstlo)
	case a.Arch.mulhi != "" && !dstlo.Valid():
		a.op3(a.Arch.mulhi, src1, src2, dsthi)
	case a.Arch.mul != "" && a.Arch.mulhi != "" && dstlo != src1 && dstlo != src2:
		a.op3(a.Arch.mul, src1, src2, dstlo)
		a.op3(a.Arch.mulhi, src1, src2, dsthi)
	case a.Arch.mul != "" && a.Arch.mulhi != "" && dsthi != src1 && dsthi != src2:
		a.op3(a.Arch.mulhi, src1, src2, dsthi)
		a.op3(a.Arch.mul, src1, src2, dstlo)
	}
}

// Jmp jumps to the label.
func (a *Asm) Jmp(label string) {
	// Note: Some systems prefer the spelling B or BR, but all accept JMP.
	a.Printf("\tJMP %s\n", label)
}

// JmpZero jumps to the label if src is zero.
// It may modify the carry flag unless a.Arch.CarrySafeLoop is true.
func (a *Asm) JmpZero(src Reg, label string) {
	a.Printf("\t"+a.Arch.jmpZero+"\n", src, label)
}

// JmpNonZero jumps to the label if src is non-zero.
// It may modify the carry flag unless a.Arch,CarrySafeLoop is true.
func (a *Asm) JmpNonZero(src Reg, label string) {
	a.Printf("\t"+a.Arch.jmpNonZero+"\n", src, label)
}

// Label emits a label with the given name.
func (a *Asm) Label(name string) {
	a.Printf("%s:\n", name)
}

// Ret returns.
func (a *Asm) Ret() {
	a.Printf("\tRET\n")
}

```

// === FILE: references!/go/src/math/big/internal/asmgen/cheat.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

// This program can be compiled with -S to produce a “cheat sheet”
// for filling out a new Arch: the compiler will show you how to implement
// the various operations.
//
// Usage (replace TARGET with your target architecture):
//
//	GOOS=linux GOARCH=TARGET go build -gcflags='-p=cheat -S' cheat.go

package p

import "math/bits"

func mov(x, y uint) uint             { return y }
func zero() uint                     { return 0 }
func add(x, y uint) uint             { return x + y }
func adds(x, y, c uint) (uint, uint) { return bits.Add(x, y, 0) }
func adcs(x, y, c uint) (uint, uint) { return bits.Add(x, y, c) }
func sub(x, y uint) uint             { return x + y }
func subs(x, y uint) (uint, uint)    { return bits.Sub(x, y, 0) }
func sbcs(x, y, c uint) (uint, uint) { return bits.Sub(x, y, c) }
func mul(x, y uint) uint             { return x * y }
func mulWide(x, y uint) (uint, uint) { return bits.Mul(x, y) }
func lsh(x, s uint) uint             { return x << s }
func rsh(x, s uint) uint             { return x >> s }
func and(x, y uint) uint             { return x & y }
func or(x, y uint) uint              { return x | y }
func xor(x, y uint) uint             { return x ^ y }
func neg(x uint) uint                { return -x }
func loop(x int) int {
	s := 0
	for i := 1; i < x; i++ {
		s += i
		if s == 98 { // useful for jmpEqual
			return 99
		}
		if s == 99 {
			return 100
		}
		if s == 0 { // useful for jmpZero
			return 101
		}
		if s != 0 { // useful for jmpNonZero
			s *= 3
		}
		s += 2 // keep last condition from being inverted
	}
	return s
}
func mem(x *[10]struct{ a, b uint }, i int) uint { return x[i].b }

```

// === FILE: references!/go/src/math/big/internal/asmgen/func.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package asmgen

import (
	"fmt"
	"slices"
	"strings"
)

// Note: Exported fields and methods are expected to be used
// by function generators (like the ones in add.go and so on).
// Unexported fields and methods should not be.

// A Func represents a single assembly function.
type Func struct {
	Name    string
	Asm     *Asm
	inputs  []string       // name of input slices (not beginning with z)
	outputs []string       // names of output slices (beginning with z)
	args    map[string]int // offsets of args, results on stack
}

// Func starts a new function in the assembly output.
func (a *Asm) Func(decl string) *Func {
	d, ok := strings.CutPrefix(decl, "func ")
	if !ok {
		a.Fatalf("func decl does not begin with 'func '")
	}
	name, d, ok := strings.Cut(d, "(")
	if !ok {
		a.Fatalf("func decl does not have func arg list")
	}
	f := &Func{
		Name: name,
		Asm:  a,
		args: make(map[string]int),
	}
	a.FreeAll()

	// Parse argument names and types. Quick and dirty.
	// Convert (args) (results) into args, results.
	d = strings.ReplaceAll(d, ") (", ", ")
	d = strings.TrimSuffix(d, ")")
	args := strings.Split(d, ",")

	// Assign implicit types to all arguments (x, y int -> x int, y int).
	typ := ""
	for i, arg := range slices.Backward(args) {
		arg = strings.TrimSpace(arg)
		if !strings.Contains(arg, " ") {
			if typ == "" {
				a.Fatalf("missing argument type")
			}
			arg += " " + typ
		} else {
			_, typ, _ = strings.Cut(arg, " ")
		}
		args[i] = arg
	}

	// Record mapping from names to offsets.
	off := 0
	for _, arg := range args {
		name, typ, _ := strings.Cut(arg, " ")
		switch typ {
		default:
			a.Fatalf("unknown type %s", typ)
		case "Word", "uint", "int":
			f.args[name] = off
			off += a.Arch.WordBytes
		case "[]Word":
			if strings.HasPrefix(name, "z") {
				f.outputs = append(f.outputs, name)
			} else {
				f.inputs = append(f.inputs, name)
			}
			f.args[name+"_base"] = off
			f.args[name+"_len"] = off + a.Arch.WordBytes
			f.args[name+"_cap"] = off + 2*a.Arch.WordBytes
			off += 3 * a.Arch.WordBytes
		}
	}

	a.Printf("\n")
	a.Printf("// %s\n", decl)
	a.Printf("TEXT ·%s(SB), NOSPLIT, $0\n", name)
	if a.Arch.setup != nil {
		a.Arch.setup(f)
	}
	return f
}

// Arg allocates a new register, copies the named argument (or result) into it,
// and returns that register.
func (f *Func) Arg(name string) Reg {
	return f.ArgHint(name, HintNone)
}

// ArgHint is like Arg but uses a register allocation hint.
func (f *Func) ArgHint(name string, hint Hint) Reg {
	off, ok := f.args[name]
	if !ok {
		f.Asm.Fatalf("unknown argument %s", name)
	}
	mem := Reg{fmt.Sprintf("%s+%d(FP)", name, off)}
	if hint == HintMemOK && f.Asm.Arch.memOK {
		return mem
	}
	r := f.Asm.RegHint(hint)
	f.Asm.Mov(mem, r)
	return r
}

// ArgPtr is like Arg but returns a RegPtr.
func (f *Func) ArgPtr(name string) RegPtr {
	return RegPtr(f.Arg(name))
}

// StoreArg stores src into the named argument (or result).
func (f *Func) StoreArg(src Reg, name string) {
	off, ok := f.args[name]
	if !ok {
		f.Asm.Fatalf("unknown argument %s", name)
	}
	a := f.Asm
	mem := Reg{fmt.Sprintf("%s+%d(FP)", name, off)}
	if src.IsImm() && !a.Arch.memOK {
		r := a.Reg()
		a.Mov(src, r)
		a.Mov(r, mem)
		a.Free(r)
		return
	}
	a.Mov(src, mem)
}

```

// === FILE: references!/go/src/math/big/internal/asmgen/loong64.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package asmgen

var ArchLoong64 = &Arch{
	Name:          "loong64",
	WordBits:      64,
	WordBytes:     8,
	CarrySafeLoop: true,

	regs: []string{
		// R0 is set to 0.
		// R1 is LR.
		// R2 is ???
		// R3 is SP.
		// R22 is g.
		// R28 and R29 are our virtual carry flags.
		// R30 is the linker/assembler temp, which we use too.
		"R4", "R5", "R6", "R7", "R8", "R9",
		"R10", "R11", "R12", "R13", "R14", "R15", "R16", "R17", "R18", "R19",
		"R20", "R21", "R23", "R24", "R25", "R26", "R27",
		"R31",
	},
	reg0:        "R0",
	regCarry:    "R28",
	regAltCarry: "R29",
	regTmp:      "R30",

	mov:   "MOVV",
	add:   "ADDVU",
	sub:   "SUBVU",
	sltu:  "SGTU",
	mul:   "MULV",
	mulhi: "MULHVU",
	lsh:   "SLLV",
	rsh:   "SRLV",
	and:   "AND",
	or:    "OR",
	xor:   "XOR",

	addWords: "ALSLV $3, %[1]s, %[2]s, %[3]s",

	jmpZero:    "BEQ %s, %s",
	jmpNonZero: "BNE %s, %s",
}

```

// === FILE: references!/go/src/math/big/internal/asmgen/main.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Asmgen generates math/big assembly.
//
// Usage:
//
//	cd go/src/math/big
//	go test ./internal/asmgen -generate
//
// Or:
//
//	go generate math/big
package asmgen

var arches = []*Arch{
	Arch386,
	ArchAMD64,
	ArchARM,
	ArchARM64,
	ArchLoong64,
	ArchMIPS,
	ArchMIPS64x,
	ArchPPC64x,
	ArchRISCV64,
	ArchS390X,
}

// generate returns the file name and content of the generated assembly for the given architecture.
func generate(arch *Arch) (file string, data []byte) {
	file = "arith_" + arch.Name + ".s"
	a := NewAsm(arch)
	addOrSubVV(a, "addVV")
	addOrSubVV(a, "subVV")
	shiftVU(a, "lshVU")
	shiftVU(a, "rshVU")
	mulAddVWW(a)
	addMulVVWW(a)
	return file, a.out.Bytes()
}

```

// === FILE: references!/go/src/math/big/internal/asmgen/mips.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package asmgen

var ArchMIPS = &Arch{
	Name:          "mipsx",
	Build:         "mips || mipsle",
	WordBits:      32,
	WordBytes:     4,
	CarrySafeLoop: true,

	regs: []string{
		// R0 is 0
		// R23 is the assembler/linker temporary (which we use too).
		// R24 and R25 are our virtual carry flags.
		// R28 is SB.
		// R29 is SP.
		// R30 is g.
		// R31 is LR.
		"R1", "R2", "R3", "R4", "R5", "R6", "R7", "R8", "R9",
		"R10", "R11", "R12", "R13", "R14", "R15", "R16", "R17", "R18", "R19",
		"R20", "R21", "R22", "R24", "R25",
	},
	reg0:        "R0",
	regTmp:      "R23",
	regCarry:    "R24",
	regAltCarry: "R25",

	mov:      "MOVW",
	add:      "ADDU",
	sltu:     "SGTU", // SGTU args are swapped, so it's really SLTU
	sub:      "SUBU",
	mulWideF: mipsMulWide,
	lsh:      "SLL",
	rsh:      "SRL",
	and:      "AND",
	or:       "OR",
	xor:      "XOR",

	jmpZero:    "BEQ %s, %s",
	jmpNonZero: "BNE %s, %s",
}

func mipsMulWide(a *Asm, src1, src2, dstlo, dsthi Reg) {
	a.Printf("\tMULU %s, %s\n\tMOVW LO, %s\n\tMOVW HI, %s\n", src1, src2, dstlo, dsthi)
}

```

// === FILE: references!/go/src/math/big/internal/asmgen/mips64.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package asmgen

var ArchMIPS64x = &Arch{
	Name:          "mips64x",
	Build:         "mips64 || mips64le",
	WordBits:      64,
	WordBytes:     8,
	CarrySafeLoop: true,

	regs: []string{
		// R0 is 0
		// R23 is the assembler/linker temporary (which we use too).
		// R24 and R25 are our virtual carry flags.
		// R28 is SB.
		// R29 is SP.
		// R30 is g.
		// R31 is LR.
		"R1", "R2", "R3", "R4", "R5", "R6", "R7", "R8", "R9",
		"R10", "R11", "R12", "R13", "R14", "R15", "R16", "R17", "R18", "R19",
		"R20", "R21", "R22", "R24", "R25",
	},
	reg0:        "R0",
	regTmp:      "R23",
	regCarry:    "R24",
	regAltCarry: "R25",

	mov:      "MOVV",
	add:      "ADDVU",
	sltu:     "SGTU", // SGTU args are swapped, so it's really SLTU
	sub:      "SUBVU",
	mulWideF: mips64MulWide,
	lsh:      "SLLV",
	rsh:      "SRLV",
	and:      "AND",
	or:       "OR",
	xor:      "XOR",

	jmpZero:    "BEQ %s, %s",
	jmpNonZero: "BNE %s, %s",
}

func mips64MulWide(a *Asm, src1, src2, dstlo, dsthi Reg) {
	a.Printf("\tMULVU %s, %s\n\tMOVV LO, %s\n\tMOVV HI, %s\n", src1, src2, dstlo, dsthi)
}

```

// === FILE: references!/go/src/math/big/internal/asmgen/mul.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package asmgen

// mulAddVWW generates mulAddVWW, which does z, c = x*m + a.
func mulAddVWW(a *Asm) {
	f := a.Func("func mulAddVWW(z, x []Word, m, a Word) (c Word)")

	if a.AltCarry().Valid() {
		addMulVirtualCarry(f, 0)
		return
	}
	addMul(f, "", "x", 0)
}

// addMulVVWW generates addMulVVWW which does z, c = x + y*m + a.
// (A more pedantic name would be addMulAddVVWW.)
func addMulVVWW(a *Asm) {
	f := a.Func("func addMulVVWW(z, x, y []Word, m, a Word) (c Word)")

	// If the architecture has virtual carries, emit that version unconditionally.
	if a.AltCarry().Valid() {
		addMulVirtualCarry(f, 1)
		return
	}

	// If the architecture optionally has two carries, test and emit both versions.
	if a.JmpEnable(OptionAltCarry, "altcarry") {
		regs := a.RegsUsed()
		addMul(f, "x", "y", 1)
		a.Label("altcarry")
		a.SetOption(OptionAltCarry, true)
		a.SetRegsUsed(regs)
		addMulAlt(f)
		a.SetOption(OptionAltCarry, false)
		return
	}

	// Otherwise emit the one-carry form.
	addMul(f, "x", "y", 1)
}

// Computing z = addsrc + m*mulsrc + a, we need:
//
//	for i := range z {
//		lo, hi := m * mulsrc[i]
//		lo, carry = bits.Add(lo, a, 0)
//		lo, carryAlt = bits.Add(lo, addsrc[i], 0)
//		z[i] = lo
//		a = hi + carry + carryAlt  // cannot overflow
//	}
//
// The final addition cannot overflow because after processing N words,
// the maximum possible value is (for a 64-bit system):
//
//	  (2**64N - 1) + (2**64 - 1)*(2**64N - 1) + (2**64 - 1)
//	= (2**64)*(2**64N - 1) + (2**64 - 1)
//	= 2**64(N+1) - 1,
//
// which fits in N+1 words (the high order one being the new value of a).
//
// (For example, with 3 decimal words, 999 + 9*999 + 9 = 999*10 + 9 = 9999.)
//
// If we unroll the loop a bit, then we can chain the carries in two passes.
// Consider:
//
//	lo0, hi0 := m * mulsrc[i]
//	lo0, carry = bits.Add(lo0, a, 0)
//	lo0, carryAlt = bits.Add(lo0, addsrc[i], 0)
//	z[i] = lo0
//	a = hi + carry + carryAlt // cannot overflow
//
//	lo1, hi1 := m * mulsrc[i]
//	lo1, carry = bits.Add(lo1, a, 0)
//	lo1, carryAlt = bits.Add(lo1, addsrc[i], 0)
//	z[i] = lo1
//	a = hi + carry + carryAlt // cannot overflow
//
//	lo2, hi2 := m * mulsrc[i]
//	lo2, carry = bits.Add(lo2, a, 0)
//	lo2, carryAlt = bits.Add(lo2, addsrc[i], 0)
//	z[i] = lo2
//	a = hi + carry + carryAlt // cannot overflow
//
//	lo3, hi3 := m * mulsrc[i]
//	lo3, carry = bits.Add(lo3, a, 0)
//	lo3, carryAlt = bits.Add(lo3, addsrc[i], 0)
//	z[i] = lo3
//	a = hi + carry + carryAlt // cannot overflow
//
// There are three ways we can optimize this sequence.
//
// (1) Reordering, we can chain carries so that we can use one hardware carry flag
// but amortize the cost of saving and restoring it across multiple instructions:
//
//	// multiply
//	lo0, hi0 := m * mulsrc[i]
//	lo1, hi1 := m * mulsrc[i+1]
//	lo2, hi2 := m * mulsrc[i+2]
//	lo3, hi3 := m * mulsrc[i+3]
//
//	lo0, carry = bits.Add(lo0, a, 0)
//	lo1, carry = bits.Add(lo1, hi0, carry)
//	lo2, carry = bits.Add(lo2, hi1, carry)
//	lo3, carry = bits.Add(lo3, hi2, carry)
//	a = hi3 + carry // cannot overflow
//
//	// add
//	lo0, carryAlt = bits.Add(lo0, addsrc[i], 0)
//	lo1, carryAlt = bits.Add(lo1, addsrc[i+1], carryAlt)
//	lo2, carryAlt = bits.Add(lo2, addsrc[i+2], carryAlt)
//	lo3, carryAlt = bits.Add(lo3, addrsc[i+3], carryAlt)
//	a = a + carryAlt // cannot overflow
//
//	z[i] = lo0
//	z[i+1] = lo1
//	z[i+2] = lo2
//	z[i+3] = lo3
//
// addMul takes this approach, using the hardware carry flag
// first for carry and then for carryAlt.
//
// (2) addMulAlt assumes there are two hardware carry flags available.
// It dedicates one each to carry and carryAlt, so that a multi-block
// unrolling can keep the flags in hardware across all the blocks.
// So even if the block size is 1, the code can do:
//
//	// multiply and add
//	lo0, hi0 := m * mulsrc[i]
//	lo0, carry = bits.Add(lo0, a, 0)
//	lo0, carryAlt = bits.Add(lo0, addsrc[i], 0)
//	z[i] = lo0
//
//	lo1, hi1 := m * mulsrc[i+1]
//	lo1, carry = bits.Add(lo1, hi0, carry)
//	lo1, carryAlt = bits.Add(lo1, addsrc[i+1], carryAlt)
//	z[i+1] = lo1
//
//	lo2, hi2 := m * mulsrc[i+2]
//	lo2, carry = bits.Add(lo2, hi1, carry)
//	lo2, carryAlt = bits.Add(lo2, addsrc[i+2], carryAlt)
//	z[i+2] = lo2
//
//	lo3, hi3 := m * mulsrc[i+3]
//	lo3, carry = bits.Add(lo3, hi2, carry)
//	lo3, carryAlt = bits.Add(lo3, addrsc[i+3], carryAlt)
//	z[i+3] = lo2
//
//	a = hi3 + carry + carryAlt // cannot overflow
//
// (3) addMulVirtualCarry optimizes for systems with explicitly computed carry bits
// (loong64, mips, riscv64), cutting the number of actual instructions almost by half.
// Look again at the original word-at-a-time version:
//
//	lo1, hi1 := m * mulsrc[i]
//	lo1, carry = bits.Add(lo1, a, 0)
//	lo1, carryAlt = bits.Add(lo1, addsrc[i], 0)
//	z[i] = lo1
//	a = hi + carry + carryAlt // cannot overflow
//
// Although it uses four adds per word, those are cheap adds: the two bits.Add adds
// use two instructions each (ADD+SLTU) and the final + adds only use one ADD each,
// for a total of 6 instructions per word. In contrast, the middle stanzas in (2) use
// only two “adds” per word, but these are SetCarry|UseCarry adds, which compile to
// five instruction each, for a total of 10 instructions per word. So the word-at-a-time
// loop is actually better. And we can reorder things slightly to use only a single carry bit:
//
//	lo1, hi1 := m * mulsrc[i]
//	lo1, carry = bits.Add(lo1, a, 0)
//	a = hi + carry
//	lo1, carry = bits.Add(lo1, addsrc[i], 0)
//	a = a + carry
//	z[i] = lo1
func addMul(f *Func, addsrc, mulsrc string, mulIndex int) {
	a := f.Asm
	mh := HintNone
	if a.Arch == Arch386 && addsrc != "" {
		mh = HintMemOK // too few registers otherwise
	}
	m := f.ArgHint("m", mh)
	c := f.Arg("a")
	n := f.Arg("z_len")

	p := f.Pipe()
	if addsrc != "" {
		p.SetHint(addsrc, HintMemOK)
	}
	p.SetHint(mulsrc, HintMulSrc)
	unroll := []int{1, 4}
	switch a.Arch {
	case Arch386:
		unroll = []int{1} // too few registers
	case ArchARM:
		p.SetMaxColumns(2) // too few registers (but more than 386)
	case ArchARM64:
		unroll = []int{1, 8} // 5% speedup on c4as16
	}

	// See the large comment above for an explanation of the code being generated.
	// This is optimization strategy 1.
	p.Start(n, unroll...)
	p.Loop(func(in, out [][]Reg) {
		a.Comment("multiply")
		prev := c
		flag := SetCarry
		for i, x := range in[mulIndex] {
			hi := a.RegHint(HintMulHi)
			a.MulWide(m, x, x, hi)
			a.Add(prev, x, x, flag)
			flag = UseCarry | SetCarry
			if prev != c {
				a.Free(prev)
			}
			out[0][i] = x
			prev = hi
		}
		a.Add(a.Imm(0), prev, c, UseCarry|SmashCarry)
		if addsrc != "" {
			a.Comment("add")
			flag := SetCarry
			for i, x := range in[0] {
				a.Add(x, out[0][i], out[0][i], flag)
				flag = UseCarry | SetCarry
			}
			a.Add(a.Imm(0), c, c, UseCarry|SmashCarry)
		}
		p.StoreN(out)
	})

	f.StoreArg(c, "c")
	a.Ret()
}

func addMulAlt(f *Func) {
	a := f.Asm
	m := f.ArgHint("m", HintMulSrc)
	c := f.Arg("a")
	n := f.Arg("z_len")

	// On amd64, we need a non-immediate for the AtUnrollEnd adds.
	r0 := a.ZR()
	if !r0.Valid() {
		r0 = a.Reg()
		a.Mov(a.Imm(0), r0)
	}

	p := f.Pipe()
	p.SetLabel("alt")
	p.SetHint("x", HintMemOK)
	p.SetHint("y", HintMemOK)
	if a.Arch == ArchAMD64 {
		p.SetMaxColumns(2)
	}

	// See the large comment above for an explanation of the code being generated.
	// This is optimization strategy (2).
	var hi Reg
	prev := c
	p.Start(n, 1, 8)
	p.AtUnrollStart(func() {
		a.Comment("multiply and add")
		a.ClearCarry(AddCarry | AltCarry)
		a.ClearCarry(AddCarry)
		hi = a.Reg()
	})
	p.AtUnrollEnd(func() {
		a.Add(r0, prev, c, UseCarry|SmashCarry)
		a.Add(r0, c, c, UseCarry|SmashCarry|AltCarry)
		prev = c
	})
	p.Loop(func(in, out [][]Reg) {
		for i, y := range in[1] {
			x := in[0][i]
			lo := y
			if lo.IsMem() {
				lo = a.Reg()
			}
			a.MulWide(m, y, lo, hi)
			a.Add(prev, lo, lo, UseCarry|SetCarry)
			a.Add(x, lo, lo, UseCarry|SetCarry|AltCarry)
			out[0][i] = lo
			prev, hi = hi, prev
		}
		p.StoreN(out)
	})

	f.StoreArg(c, "c")
	a.Ret()
}

func addMulVirtualCarry(f *Func, mulIndex int) {
	a := f.Asm
	m := f.Arg("m")
	c := f.Arg("a")
	n := f.Arg("z_len")

	// See the large comment above for an explanation of the code being generated.
	// This is optimization strategy (3).
	p := f.Pipe()
	p.Start(n, 1, 4)
	p.Loop(func(in, out [][]Reg) {
		a.Comment("synthetic carry, one column at a time")
		lo, hi := a.Reg(), a.Reg()
		for i, x := range in[mulIndex] {
			a.MulWide(m, x, lo, hi)
			if mulIndex == 1 {
				a.Add(in[0][i], lo, lo, SetCarry)
				a.Add(a.Imm(0), hi, hi, UseCarry|SmashCarry)
			}
			a.Add(c, lo, x, SetCarry)
			a.Add(a.Imm(0), hi, c, UseCarry|SmashCarry)
			out[0][i] = x
		}
		p.StoreN(out)
	})
	f.StoreArg(c, "c")
	a.Ret()
}

```

// === FILE: references!/go/src/math/big/internal/asmgen/pipe.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package asmgen

import (
	"fmt"
	"math/bits"
	"slices"
)

// Note: Exported fields and methods are expected to be used
// by function generators (like the ones in add.go and so on).
// Unexported fields and methods should not be.

// A Pipe manages the input and output data pipelines for a function's
// memory operations.
//
// The input is one or more equal-length slices of words, so collectively
// it can be viewed as a matrix, in which each slice is a row and each column
// is a set of corresponding words from the different slices.
// The output can be viewed the same way, although it is often just one row.
type Pipe struct {
	f               *Func    // function being generated
	label           string   // prefix for loop labels (default "loop")
	backward        bool     // processing columns in reverse
	started         bool     // Start has been called
	loaded          bool     // LoadPtrs has been called
	inPtr           []RegPtr // input slice pointers
	hints           []Hint   // for each inPtr, a register hint to use for its data
	outPtr          []RegPtr // output slice pointers
	index           Reg      // index register, if in use
	useIndexCounter bool     // index counter requested
	indexCounter    int      // index is also counter (386); 0 no, -1 negative counter, +1 positive counter
	readOff         int      // read offset not yet added to index
	writeOff        int      // write offset not yet added to index
	factors         []int    // unrolling factors
	counts          []Reg    // iterations for each factor
	needWrite       bool     // need a write call during Loop1/LoopN
	maxColumns      int      // maximum columns during unrolled loop
	unrollStart     func()   // emit code at start of unrolled body
	unrollEnd       func()   // emit code end of unrolled body
}

// Pipe creates and returns a new pipe for use in the function f.
func (f *Func) Pipe() *Pipe {
	a := f.Asm
	p := &Pipe{
		f:          f,
		label:      "loop",
		maxColumns: 10000000,
	}
	if m := a.Arch.maxColumns; m != 0 {
		p.maxColumns = m
	}
	return p
}

// SetBackward sets the pipe to process the input and output columns in reverse order.
// This is needed for left shifts, which might otherwise overwrite data they will read later.
func (p *Pipe) SetBackward() {
	if p.loaded {
		p.f.Asm.Fatalf("SetBackward after Start/LoadPtrs")
	}
	p.backward = true
}

// SetUseIndexCounter sets the pipe to use an index counter if possible,
// meaning the loop counter is also used as an index for accessing the slice data.
// This clever trick is slower on modern processors, but it is still necessary on 386.
// On non-386 systems, SetUseIndexCounter is a no-op.
func (p *Pipe) SetUseIndexCounter() {
	if p.f.Asm.Arch.memIndex == nil { // need memIndex (only 386 provides it)
		return
	}
	p.useIndexCounter = true
}

// SetLabel sets the label prefix for the loops emitted by the pipe.
// The default prefix is "loop".
func (p *Pipe) SetLabel(label string) {
	p.label = label
}

// SetMaxColumns sets the maximum number of
// columns processed in a single loop body call.
func (p *Pipe) SetMaxColumns(m int) {
	p.maxColumns = m
}

// SetHint records that the inputs from the named vector
// should be allocated with the given register hint.
//
// If the hint indicates a single register on the target architecture,
// then SetHint calls SetMaxColumns(1), since the hinted register
// can only be used for one value at a time.
func (p *Pipe) SetHint(name string, hint Hint) {
	if hint == HintMemOK && !p.f.Asm.Arch.memOK {
		return
	}
	i := slices.Index(p.f.inputs, name)
	if i < 0 {
		p.f.Asm.Fatalf("unknown input name %s", name)
	}
	if p.f.Asm.hint(hint) != "" {
		p.SetMaxColumns(1)
	}
	for len(p.hints) <= i {
		p.hints = append(p.hints, HintNone)
	}
	p.hints[i] = hint
}

// LoadPtrs loads the slice pointer arguments into registers,
// assuming that the slice length n has already been loaded
// into the register n.
//
// Start will call LoadPtrs if it has not been called already.
// LoadPtrs only needs to be called explicitly when code needs
// to use LoadN before Start, like when the shift.go generators
// read an initial word before the loop.
func (p *Pipe) LoadPtrs(n Reg) {
	a := p.f.Asm
	if p.loaded {
		a.Fatalf("pointers already loaded")
	}

	// Load the actual pointers.
	p.loaded = true
	for _, name := range p.f.inputs {
		p.inPtr = append(p.inPtr, RegPtr(p.f.Arg(name+"_base")))
	}
	for _, name := range p.f.outputs {
		p.outPtr = append(p.outPtr, RegPtr(p.f.Arg(name+"_base")))
	}

	// Decide the memory access strategy for LoadN and StoreN.
	switch {
	case p.backward && p.useIndexCounter:
		// Generator wants an index counter, meaning when the iteration counter
		// is AX, we will access the slice with pointer BX using (BX)(AX*WordBytes).
		// The loop is moving backward through the slice, but the counter
		// is also moving backward, so not much to do.
		a.Comment("run loop backward, using counter as positive index")
		p.indexCounter = +1
		p.index = n

	case !p.backward && p.useIndexCounter:
		// Generator wants an index counter, but the loop is moving forward.
		// To make the counter move in the direction of data access,
		// we negate the counter, counting up from -len(z) to -1.
		// To make the index access the right words, we add len(z)*WordBytes
		// to each of the pointers.
		// See comment below about the garbage collector (non-)implications
		// of pointing beyond the slice bounds.
		a.Comment("use counter as negative index")
		p.indexCounter = -1
		p.index = n
		for _, ptr := range p.inPtr {
			a.AddWords(n, ptr, ptr)
		}
		for _, ptr := range p.outPtr {
			a.AddWords(n, ptr, ptr)
		}
		a.Neg(n, n)

	case p.backward:
		// Generator wants to run the loop backward.
		// We'll decrement the pointers before using them,
		// so position them at the very end of the slices.
		// If we had precise pointer information for assembly,
		// these pointers would cause problems with the garbage collector,
		// since they no longer point into the allocated slice,
		// but the garbage collector ignores unexpected values in assembly stacks,
		// and the actual slice pointers are still in the argument stack slots,
		// so the slices won't be collected early.
		// If we switched to the register ABI, we might have to rethink this.
		// (The same thing happens by the end of forward loops,
		// but it's less important since once the pointers go off the slice
		// in a forward loop, the loop is over and the slice won't be accessed anymore.)
		a.Comment("run loop backward")
		for _, ptr := range p.inPtr {
			a.AddWords(n, ptr, ptr)
		}
		for _, ptr := range p.outPtr {
			a.AddWords(n, ptr, ptr)
		}

	case !p.backward:
		// Nothing to do!
	}
}

// LoadN returns the next n columns of input words as a slice of rows.
// Regs for inputs that have been marked using p.SetMemOK will be direct memory references.
// Regs for other inputs will be newly allocated registers and must be freed.
func (p *Pipe) LoadN(n int) [][]Reg {
	a := p.f.Asm
	regs := make([][]Reg, len(p.inPtr))
	for i, ptr := range p.inPtr {
		regs[i] = make([]Reg, n)
		switch {
		case a.Arch.loadIncN != nil:
			// Load from memory and advance pointers at the same time.
			for j := range regs[i] {
				regs[i][j] = p.f.Asm.Reg()
			}
			if p.backward {
				a.Arch.loadDecN(a, ptr, regs[i])
			} else {
				a.Arch.loadIncN(a, ptr, regs[i])
			}

		default:
			// Load from memory using offsets.
			// We'll advance the pointers or the index counter later.
			for j := range n {
				off := p.readOff + j
				if p.backward {
					off = -(off + 1)
				}
				var mem Reg
				if p.indexCounter != 0 {
					mem = a.Arch.memIndex(a, off*a.Arch.WordBytes, p.index, ptr)
				} else {
					mem = ptr.mem(off * a.Arch.WordBytes)
				}
				h := HintNone
				if i < len(p.hints) {
					h = p.hints[i]
				}
				if h == HintMemOK {
					regs[i][j] = mem
				} else {
					r := p.f.Asm.RegHint(h)
					a.Mov(mem, r)
					regs[i][j] = r
				}
			}
		}
	}
	p.readOff += n
	return regs
}

// StoreN writes regs (a slice of rows) to the next n columns of output, where n = len(regs[0]).
func (p *Pipe) StoreN(regs [][]Reg) {
	p.needWrite = false
	a := p.f.Asm
	if len(regs) != len(p.outPtr) {
		p.f.Asm.Fatalf("wrong number of output rows")
	}
	n := len(regs[0])
	for i, ptr := range p.outPtr {
		switch {
		case a.Arch.storeIncN != nil:
			// Store to memory and advance pointers at the same time.
			if p.backward {
				a.Arch.storeDecN(a, ptr, regs[i])
			} else {
				a.Arch.storeIncN(a, ptr, regs[i])
			}

		default:
			// Store to memory using offsets.
			// We'll advance the pointers or the index counter later.
			for j, r := range regs[i] {
				off := p.writeOff + j
				if p.backward {
					off = -(off + 1)
				}
				var mem Reg
				if p.indexCounter != 0 {
					mem = a.Arch.memIndex(a, off*a.Arch.WordBytes, p.index, ptr)
				} else {
					mem = ptr.mem(off * a.Arch.WordBytes)
				}
				a.Mov(r, mem)
			}
		}
	}
	p.writeOff += n
}

// advancePtrs advances the pointers by step
// or handles bookkeeping for an imminent index advance by step
// that the caller will do.
func (p *Pipe) advancePtrs(step int) {
	a := p.f.Asm
	switch {
	case a.Arch.loadIncN != nil:
		// nothing to do

	default:
		// Adjust read/write offsets for pointer advance (or imminent index advance).
		p.readOff -= step
		p.writeOff -= step

		if p.indexCounter == 0 {
			// Advance pointers.
			if p.backward {
				step = -step
			}
			for _, ptr := range p.inPtr {
				a.Add(a.Imm(step*a.Arch.WordBytes), Reg(ptr), Reg(ptr), KeepCarry)
			}
			for _, ptr := range p.outPtr {
				a.Add(a.Imm(step*a.Arch.WordBytes), Reg(ptr), Reg(ptr), KeepCarry)
			}
		}
	}
}

// DropInput deletes the named input from the pipe,
// usually because it has been exhausted.
// (This is not used yet but will be used in a future generator.)
func (p *Pipe) DropInput(name string) {
	i := slices.Index(p.f.inputs, name)
	if i < 0 {
		p.f.Asm.Fatalf("unknown input %s", name)
	}
	ptr := p.inPtr[i]
	p.f.Asm.Free(Reg(ptr))
	p.inPtr = slices.Delete(p.inPtr, i, i+1)
	p.f.inputs = slices.Delete(p.f.inputs, i, i+1)
	if len(p.hints) > i {
		p.hints = slices.Delete(p.hints, i, i+1)
	}
}

// Start prepares to loop over n columns.
// The factors give a sequence of unrolling factors to use,
// which must be either strictly increasing or strictly decreasing
// and must include 1.
// For example, 4, 1 means to process 4 elements at a time
// and then 1 at a time for the final 0-3; specifying 1,4 instead
// handles 0-3 elements first and then 4 at a time.
// Similarly, 32, 4, 1 means to process 32 at a time,
// then 4 at a time, then 1 at a time.
//
// One benefit of using 1, 4 instead of 4, 1 is that the body
// processing 4 at a time needs more registers, and if it is
// the final body, the register holding the fragment count (0-3)
// has been freed and is available for use.
//
// Start may modify the carry flag.
//
// Start must be followed by a call to Loop1 or LoopN,
// but it is permitted to emit other instructions first,
// for example to set an initial carry flag.
func (p *Pipe) Start(n Reg, factors ...int) {
	a := p.f.Asm
	if p.started {
		a.Fatalf("loop already started")
	}
	if p.useIndexCounter && len(factors) > 1 {
		a.Fatalf("cannot call SetUseIndexCounter and then use Start with factors != [1]; have factors = %v", factors)
	}
	p.started = true
	if !p.loaded {
		if len(factors) == 1 {
			p.SetUseIndexCounter()
		}
		p.LoadPtrs(n)
	}

	// If there were calls to LoadN between LoadPtrs and Start,
	// adjust the loop not to scan those columns, assuming that
	// either the code already called an equivalent StoreN or else
	// that it will do so after the loop.
	if off := p.readOff; off != 0 {
		if p.indexCounter < 0 {
			// Index is negated, so add off instead of subtracting.
			a.Add(a.Imm(off), n, n, SmashCarry)
		} else {
			a.Sub(a.Imm(off), n, n, SmashCarry)
		}
		if p.indexCounter != 0 {
			// n is also the index we are using, so adjust readOff and writeOff
			// to continue to point at the same positions as before we changed n.
			p.readOff -= off
			p.writeOff -= off
		}
	}

	p.Restart(n, factors...)
}

// Restart prepares to loop over an additional n columns,
// beyond a previous loop run by p.Start/p.Loop.
func (p *Pipe) Restart(n Reg, factors ...int) {
	a := p.f.Asm
	if !p.started {
		a.Fatalf("pipe not started")
	}
	p.factors = factors
	p.counts = make([]Reg, len(factors))
	if len(factors) == 0 {
		factors = []int{1}
	}

	// Compute the loop lengths for each unrolled section into separate registers.
	// We compute them all ahead of time in case the computation would smash
	// a carry flag that the loop bodies need preserved.
	if len(factors) > 1 {
		a.Comment("compute unrolled loop lengths")
	}
	switch {
	default:
		a.Fatalf("invalid factors %v", factors)

	case factors[0] == 1:
		// increasing loop factors
		div := 1
		for i, f := range factors[1:] {
			if f <= factors[i] {
				a.Fatalf("non-increasing factors %v", factors)
			}
			if f&(f-1) != 0 {
				a.Fatalf("non-power-of-two factors %v", factors)
			}
			t := p.f.Asm.Reg()
			f /= div
			a.And(a.Imm(f-1), n, t)
			a.Rsh(a.Imm(bits.TrailingZeros(uint(f))), n, n)
			div *= f
			p.counts[i] = t
		}
		p.counts[len(p.counts)-1] = n

	case factors[len(factors)-1] == 1:
		// decreasing loop factors
		for i, f := range factors[:len(factors)-1] {
			if f <= factors[i+1] {
				a.Fatalf("non-decreasing factors %v", factors)
			}
			if f&(f-1) != 0 {
				a.Fatalf("non-power-of-two factors %v", factors)
			}
			t := p.f.Asm.Reg()
			a.Rsh(a.Imm(bits.TrailingZeros(uint(f))), n, t)
			a.And(a.Imm(f-1), n, n)
			p.counts[i] = t
		}
		p.counts[len(p.counts)-1] = n
	}
}

// Done frees all the registers allocated by the pipe.
func (p *Pipe) Done() {
	for _, ptr := range p.inPtr {
		p.f.Asm.Free(Reg(ptr))
	}
	p.inPtr = nil
	for _, ptr := range p.outPtr {
		p.f.Asm.Free(Reg(ptr))
	}
	p.outPtr = nil
	p.index = Reg{}
}

// Loop emits code for the loop, calling block repeatedly to emit code that
// handles a block of N input columns (for arbitrary N = len(in[0]) chosen by p).
// block must call p.StoreN(out) to write N output columns.
// The out slice is a pre-allocated matrix of uninitialized Reg values.
// block is expected to set each entry to the Reg that should be written
// before calling p.StoreN(out).
//
// For example, if the loop is to be unrolled 4x in blocks of 2 columns each,
// the sequence of calls to emit the unrolled loop body is:
//
//	start()  // set by pAtUnrollStart
//	... reads for 2 columns ...
//	block()
//	... writes for 2 columns ...
//	... reads for 2 columns ...
//	block()
//	... writes for 2 columns ...
//	end()  // set by p.AtUnrollEnd
//
// Any registers allocated during block are freed automatically when block returns.
func (p *Pipe) Loop(block func(in, out [][]Reg)) {
	if p.factors == nil {
		p.f.Asm.Fatalf("Pipe.Start not called")
	}
	for i, factor := range p.factors {
		n := p.counts[i]
		p.unroll(n, factor, block)
		if i < len(p.factors)-1 {
			p.f.Asm.Free(n)
		}
	}
	p.factors = nil
}

// AtUnrollStart sets a function to call at the start of an unrolled sequence.
// See [Pipe.Loop] for details.
func (p *Pipe) AtUnrollStart(start func()) {
	p.unrollStart = start
}

// AtUnrollEnd sets a function to call at the end of an unrolled sequence.
// See [Pipe.Loop] for details.
func (p *Pipe) AtUnrollEnd(end func()) {
	p.unrollEnd = end
}

// unroll emits a single unrolled loop for the given factor, iterating n times.
func (p *Pipe) unroll(n Reg, factor int, block func(in, out [][]Reg)) {
	a := p.f.Asm
	label := fmt.Sprintf("%s%d", p.label, factor)

	// Top of loop control flow.
	a.Label(label)
	if a.Arch.loopTop != "" {
		a.Printf("\t"+a.Arch.loopTop+"\n", n, label+"done")
	} else {
		a.JmpZero(n, label+"done")
	}
	a.Label(label + "cont")

	// Unrolled loop body.
	if factor < p.maxColumns {
		a.Comment("unroll %dX", factor)
	} else {
		a.Comment("unroll %dX in batches of %d", factor, p.maxColumns)
	}
	if p.unrollStart != nil {
		p.unrollStart()
	}
	for done := 0; done < factor; {
		batch := min(factor-done, p.maxColumns)
		regs := a.RegsUsed()
		out := make([][]Reg, len(p.outPtr))
		for i := range out {
			out[i] = make([]Reg, batch)
		}
		in := p.LoadN(batch)
		p.needWrite = true
		block(in, out)
		if p.needWrite && len(p.outPtr) > 0 {
			a.Fatalf("missing p.Write1 or p.StoreN")
		}
		a.SetRegsUsed(regs) // free anything block allocated
		done += batch
	}
	if p.unrollEnd != nil {
		p.unrollEnd()
	}
	p.advancePtrs(factor)

	// Bottom of loop control flow.
	switch {
	case p.indexCounter >= 0 && a.Arch.loopBottom != "":
		a.Printf("\t"+a.Arch.loopBottom+"\n", n, label+"cont")

	case p.indexCounter >= 0:
		a.Sub(a.Imm(1), n, n, KeepCarry)
		a.JmpNonZero(n, label+"cont")

	case p.indexCounter < 0 && a.Arch.loopBottomNeg != "":
		a.Printf("\t"+a.Arch.loopBottomNeg+"\n", n, label+"cont")

	case p.indexCounter < 0:
		a.Add(a.Imm(1), n, n, KeepCarry)
	}
	a.Label(label + "done")
}

```

// === FILE: references!/go/src/math/big/internal/asmgen/ppc64.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package asmgen

var ArchPPC64x = &Arch{
	Name:          "ppc64x",
	Build:         "ppc64 || ppc64le",
	WordBits:      64,
	WordBytes:     8,
	CarrySafeLoop: true,

	// Note: The old, hand-written ppc64x assembly used MOVDU
	// to avoid explicit pointer updates in a few routines, but the new
	// generated code runs just as fast, so we haven't bothered to try
	// to add that back. (It's not trivial; you'd have to keep the pointers
	// shifted one word in order to make the semantics work.)
	//
	// The old assembly also used some complex vector instructions
	// to implement lshVU and rshVU, but the generated code that uses
	// ordinary integer instructions is much faster than the vector code was,
	// at least on the power10 gomote.

	regs: []string{
		// R0 is 0 by convention.
		// R1 is SP.
		// R2 is TOC.
		// R30 is g.
		// R31 is the assembler/linker temporary (which we use too).
		"R3", "R4", "R5", "R6", "R7", "R8", "R9",
		"R10", "R11", "R12" /*R13 is TLS*/, "R14", "R15", "R16", "R17", "R18", "R19",
		"R20", "R21", "R22", "R23", "R24", "R25", "R26", "R27", "R28", "R29",
	},
	reg0:   "R0",
	regTmp: "R31",

	// Note: Could write an addF and subF to use ADDZE and SUBZE,
	// but we have R0 so it doesn't seem to matter much.

	mov:   "MOVD",
	add:   "ADD",
	adds:  "ADDC",
	adcs:  "ADDE",
	sub:   "SUB",
	subs:  "SUBC",
	sbcs:  "SUBE",
	mul:   "MULLD",
	mulhi: "MULHDU",
	lsh:   "SLD",
	rsh:   "SRD",
	and:   "ANDCC", // regular AND does not accept immediates
	or:    "OR",
	xor:   "XOR",

	jmpZero:    "CMP %[1]s, $0; BEQ %[2]s",
	jmpNonZero: "CMP %s, $0; BNE %s",

	// Note: Using CTR means that we could free the count register
	// during the loop body, but the portable logic doesn't know that,
	// and we're not hurting for registers.
	loopTop:    "CMP %[1]s, $0; BEQ %[2]s; MOVD %[1]s, CTR",
	loopBottom: "BDNZ %[2]s",
}

```

// === FILE: references!/go/src/math/big/internal/asmgen/riscv64.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package asmgen

var ArchRISCV64 = &Arch{
	Name:          "riscv64",
	WordBits:      64,
	WordBytes:     8,
	CarrySafeLoop: true,

	regs: []string{
		// X0 is zero.
		// X1 is LR.
		// X2 is SP.
		// X3 is SB.
		// X4 is TP.
		// X27 is g.
		// X28 and X29 are our virtual carry flags.
		// X31 is the assembler/linker temporary (which we use too).
		"X5", "X6", "X7", "X8", "X9",
		"X10", "X11", "X12", "X13", "X14", "X15", "X16", "X17", "X18", "X19",
		"X20", "X21", "X22", "X23", "X24", "X25", "X26",
		"X30",
	},

	reg0:        "X0",
	regCarry:    "X28",
	regAltCarry: "X29",
	regTmp:      "X31",

	mov:   "MOV",
	add:   "ADD",
	sub:   "SUB",
	mul:   "MUL",
	mulhi: "MULHU",
	lsh:   "SLL",
	rsh:   "SRL",
	and:   "AND",
	or:    "OR",
	xor:   "XOR",
	sltu:  "SLTU",

	jmpZero:    "BEQZ %s, %s",
	jmpNonZero: "BNEZ %s, %s",
}

```

// === FILE: references!/go/src/math/big/internal/asmgen/s390x.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package asmgen

var ArchS390X = &Arch{
	Name:          "s390x",
	WordBits:      64,
	WordBytes:     8,
	CarrySafeLoop: true,

	regs: []string{
		// R0 is 0 by convention in this code (see setup).
		// R10 is the assembler/linker temporary.
		// R11 is a second assembler/linker temporary, for wide multiply.
		// We allow allocating R10 and R11 so that we can use them as
		// direct multiplication targets while tracking whether they're in use.
		// R13 is g.
		// R14 is LR.
		// R15 is SP.
		"R1", "R2", "R3", "R4", "R5", "R6", "R7", "R8", "R9",
		"R10", "R11", "R12",
	},
	reg0:       "R0",
	regTmp:     "R10",
	setup:      s390xSetup,
	maxColumns: 2,
	op3:        s390xOp3,
	hint:       s390xHint,

	// Instruction reference: chapter 7 of
	// https://www.ibm.com/docs/en/SSQ2R2_15.0.0/com.ibm.tpf.toolkit.hlasm.doc/dz9zr006.pdf

	mov:      "MOVD",
	adds:     "ADDC", // ADD is an alias for ADDC, sets carry
	adcs:     "ADDE",
	subs:     "SUBC", // SUB is an alias for SUBC, sets carry
	sbcs:     "SUBE",
	mulWideF: s390MulWide,
	lsh:      "SLD",
	rsh:      "SRD",
	and:      "AND",
	or:       "OR",
	xor:      "XOR",
	neg:      "NEG",
	lea:      "LAY", // LAY because LA only accepts positive offsets

	jmpZero:    "CMPBEQ %s, $0, %s",
	jmpNonZero: "CMPBNE %s, $0, %s",
}

func s390xSetup(f *Func) {
	a := f.Asm
	if f.Name == "addVV" || f.Name == "subVV" {
		// S390x, unlike every other system, has vector instructions
		// that can propagate carry bits during parallel adds (VACC).
		// Instead of trying to generate that for this one system,
		// jump to the hand-written code in arithvec_s390x.s.
		a.Printf("\tMOVB ·hasVX(SB), R1\n")
		a.Printf("\tCMPBEQ R1, $0, novec\n")
		a.Printf("\tJMP ·%svec(SB)\n", f.Name)
		a.Printf("novec:\n")
	}
	a.Printf("\tMOVD $0, R0\n")
}

func s390xOp3(name string) bool {
	if name == "AND" { // AND with immediate only takes imm, reg; not imm, reg, reg.
		return false
	}
	return true
}

func s390xHint(_ *Asm, h Hint) string {
	switch h {
	case HintMulSrc:
		return "R11"
	case HintMulHi:
		return "R10"
	}
	return ""
}

func s390MulWide(a *Asm, src1, src2, dstlo, dsthi Reg) {
	if src1.name != "R11" && src2.name != "R11" {
		a.Fatalf("mulWide src1 or src2 must be R11")
	}
	if dstlo.name != "R11" {
		a.Fatalf("mulWide dstlo must be R11")
	}
	if dsthi.name != "R10" {
		a.Fatalf("mulWide dsthi must be R10")
	}
	src := src1
	if src.name == "R11" {
		src = src2
	}
	a.Printf("\tMLGR %s, R10\n", src)
}

```

// === FILE: references!/go/src/math/big/internal/asmgen/shift.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package asmgen

// shiftVU generates lshVU and rshVU, which do
// z, c = x << s and z, c = x >> s, for 0 < s < _W.
func shiftVU(a *Asm, name string) {
	// Because these routines can be called for z.Lsh(z, N) and z.Rsh(z, N),
	// the input and output slices may be aliased at different offsets.
	// For example (on 64-bit systems), during z.Lsh(z, 65), &z[0] == &x[1],
	// and during z.Rsh(z, 65), &z[1] == &x[0].
	// For left shift, we must process the slices from len(z)-1 down to 0,
	// so that we don't overwrite a word before we need to read it.
	// For right shift, we must process the slices from 0 up to len(z)-1.
	// The different traversals at least make the two cases more consistent,
	// since we're always delaying the output by one word compared
	// to the input.

	f := a.Func("func " + name + "(z, x []Word, s uint) (c Word)")

	// Check for no input early, since we need to start by reading 1 word.
	n := f.Arg("z_len")
	a.JmpZero(n, "ret0")

	// Start loop by reading first input word.
	s := f.ArgHint("s", HintShiftCount)
	p := f.Pipe()
	if name == "lshVU" {
		p.SetBackward()
	}
	unroll := []int{1, 4}
	if a.Arch == Arch386 {
		unroll = []int{1} // too few registers for more
		p.SetUseIndexCounter()
	}
	p.LoadPtrs(n)
	a.Comment("shift first word into carry")
	prev := p.LoadN(1)[0][0]

	// Decide how to shift. On systems with a wide shift (x86), use that.
	// Otherwise, we need shift by s and negative (reverse) shift by 64-s or 32-s.
	shift := a.Lsh
	shiftWide := a.LshWide
	negShift := a.Rsh
	negShiftReg := a.RshReg
	if name == "rshVU" {
		shift = a.Rsh
		shiftWide = a.RshWide
		negShift = a.Lsh
		negShiftReg = a.LshReg
	}
	if a.Arch.HasShiftWide() {
		// Use wide shift to avoid needing negative shifts.
		// The invariant is that prev holds the previous word (not shifted at all),
		// to be used as input into the wide shift.
		// After the loop finishes, prev holds the final output word to be written.
		c := a.Reg()
		shiftWide(s, prev, a.Imm(0), c)
		f.StoreArg(c, "c")
		a.Free(c)
		a.Comment("shift remaining words")
		p.Start(n, unroll...)
		p.Loop(func(in [][]Reg, out [][]Reg) {
			// We reuse the input registers as output, delayed one cycle; prev is the first output.
			// After writing the outputs to memory, we can copy the final x value into prev
			// for the next iteration.
			old := prev
			for i, x := range in[0] {
				shiftWide(s, x, old, old)
				out[0][i] = old
				old = x
			}
			p.StoreN(out)
			a.Mov(old, prev)
		})
		a.Comment("store final shifted bits")
		shift(s, prev, prev)
	} else {
		// Construct values from x << s and x >> (64-s).
		// After the first word has been processed, the invariant is that
		// prev holds x << s, to be used as the high bits of the next output word,
		// once we find the low bits after reading the next input word.
		// After the loop finishes, prev holds the final output word to be written.
		sNeg := a.Reg()
		a.Mov(a.Imm(a.Arch.WordBits), sNeg)
		a.Sub(s, sNeg, sNeg, SmashCarry)
		c := a.Reg()
		negShift(sNeg, prev, c)
		shift(s, prev, prev)
		f.StoreArg(c, "c")
		a.Free(c)
		a.Comment("shift remaining words")
		p.Start(n, unroll...)
		p.Loop(func(in, out [][]Reg) {
			if a.HasRegShift() {
				// ARM (32-bit) allows shifts in most arithmetic expressions,
				// including OR, letting us combine the negShift and a.Or.
				// The simplest way to manage the registers is to do StoreN for
				// one output at a time, and since we don't use multi-register
				// stores on ARM, that doesn't hurt us.
				out[0] = out[0][:1]
				for _, x := range in[0] {
					a.Or(negShiftReg(sNeg, x), prev, prev)
					out[0][0] = prev
					p.StoreN(out)
					shift(s, x, prev)
				}
				return
			}
			// We reuse the input registers as output, delayed one cycle; z0 is the first output.
			z0 := a.Reg()
			z := z0
			for i, x := range in[0] {
				negShift(sNeg, x, z)
				a.Or(prev, z, z)
				shift(s, x, prev)
				out[0][i] = z
				z = x
			}
			p.StoreN(out)
		})
		a.Comment("store final shifted bits")
	}
	p.StoreN([][]Reg{{prev}})
	p.Done()
	a.Free(s)
	a.Ret()

	// Return 0, used from above.
	a.Label("ret0")
	f.StoreArg(a.Imm(0), "c")
	a.Ret()
}

```

// === FILE: references!/go/src/math/big/intmarsh.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements encoding/decoding of Ints.

package big

import (
	"bytes"
	"fmt"
)

// Gob codec version. Permits backward-compatible changes to the encoding.
const intGobVersion byte = 1

// GobEncode implements the [encoding/gob.GobEncoder] interface.
func (x *Int) GobEncode() ([]byte, error) {
	if x == nil {
		return nil, nil
	}
	buf := make([]byte, 1+len(x.abs)*_S) // extra byte for version and sign bit
	i := x.abs.bytes(buf) - 1            // i >= 0
	b := intGobVersion << 1              // make space for sign bit
	if x.neg {
		b |= 1
	}
	buf[i] = b
	return buf[i:], nil
}

// GobDecode implements the [encoding/gob.GobDecoder] interface.
func (z *Int) GobDecode(buf []byte) error {
	if len(buf) == 0 {
		// Other side sent a nil or default value.
		*z = Int{}
		return nil
	}
	b := buf[0]
	if b>>1 != intGobVersion {
		return fmt.Errorf("Int.GobDecode: encoding version %d not supported", b>>1)
	}
	z.neg = b&1 != 0
	z.abs = z.abs.setBytes(buf[1:])
	return nil
}

// AppendText implements the [encoding.TextAppender] interface.
func (x *Int) AppendText(b []byte) (text []byte, err error) {
	return x.Append(b, 10), nil
}

// MarshalText implements the [encoding.TextMarshaler] interface.
func (x *Int) MarshalText() (text []byte, err error) {
	return x.AppendText(nil)
}

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (z *Int) UnmarshalText(text []byte) error {
	if _, ok := z.setFromScanner(bytes.NewReader(text), 0); !ok {
		return fmt.Errorf("math/big: cannot unmarshal %q into a *big.Int", text)
	}
	return nil
}

// The JSON marshalers are only here for API backward compatibility
// (programs that explicitly look for these two methods). JSON works
// fine with the TextMarshaler only.

// MarshalJSON implements the [encoding/json.Marshaler] interface.
func (x *Int) MarshalJSON() ([]byte, error) {
	if x == nil {
		return []byte("null"), nil
	}
	return x.abs.itoa(x.neg, 10), nil
}

// UnmarshalJSON implements the [encoding/json.Unmarshaler] interface.
func (z *Int) UnmarshalJSON(text []byte) error {
	// Ignore null, like in the main JSON package.
	if string(text) == "null" {
		return nil
	}
	return z.UnmarshalText(text)
}

```

// === FILE: references!/go/src/math/big/nat.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements unsigned multi-precision integers (natural
// numbers). They are the building blocks for the implementation
// of signed integers, rationals, and floating-point numbers.
//
// Caution: This implementation relies on the function "alias"
//          which assumes that (nat) slice capacities are never
//          changed (no 3-operand slice expressions). If that
//          changes, alias needs to be updated for correctness.

package big

import (
	"internal/byteorder"
	"math/bits"
	"math/rand"
	"slices"
	"sync"
)

// An unsigned integer x of the form
//
//	x = x[n-1]*_B^(n-1) + x[n-2]*_B^(n-2) + ... + x[1]*_B + x[0]
//
// with 0 <= x[i] < _B and 0 <= i < n is stored in a slice of length n,
// with the digits x[i] as the slice elements.
//
// A number is normalized if the slice contains no leading 0 digits.
// During arithmetic operations, denormalized values may occur but are
// always normalized before returning the final result. The normalized
// representation of 0 is the empty or nil slice (length = 0).
type nat []Word

var (
	natOne  = nat{1}
	natTwo  = nat{2}
	natFive = nat{5}
	natTen  = nat{10}
)

func (z nat) String() string {
	return "0x" + string(z.itoa(false, 16))
}

func (z nat) norm() nat {
	i := len(z)
	for i > 0 && z[i-1] == 0 {
		i--
	}
	return z[0:i]
}

func (z nat) make(n int) nat {
	if n <= cap(z) {
		return z[:n] // reuse z
	}
	if n == 1 {
		// Most nats start small and stay that way; don't over-allocate.
		return make(nat, 1)
	}
	// Choosing a good value for e has significant performance impact
	// because it increases the chance that a value can be reused.
	const e = 4 // extra capacity
	return make(nat, n, n+e)
}

func (z nat) setWord(x Word) nat {
	if x == 0 {
		return z[:0]
	}
	z = z.make(1)
	z[0] = x
	return z
}

func (z nat) setUint64(x uint64) nat {
	// single-word value
	if w := Word(x); uint64(w) == x {
		return z.setWord(w)
	}
	// 2-word value
	z = z.make(2)
	z[1] = Word(x >> 32)
	z[0] = Word(x)
	return z
}

func (z nat) set(x nat) nat {
	z = z.make(len(x))
	copy(z, x)
	return z
}

func (z nat) add(x, y nat) nat {
	m := len(x)
	n := len(y)

	switch {
	case m < n:
		return z.add(y, x)
	case m == 0:
		// n == 0 because m >= n; result is 0
		return z[:0]
	case n == 0:
		// result is x
		return z.set(x)
	}
	// m > 0

	z = z.make(m + 1)
	c := addVV(z[:n], x[:n], y[:n])
	if m > n {
		c = addVW(z[n:m], x[n:], c)
	}
	z[m] = c

	return z.norm()
}

func (z nat) sub(x, y nat) nat {
	m := len(x)
	n := len(y)

	switch {
	case m < n:
		panic("underflow")
	case m == 0:
		// n == 0 because m >= n; result is 0
		return z[:0]
	case n == 0:
		// result is x
		return z.set(x)
	}
	// m > 0

	z = z.make(m)
	c := subVV(z[:n], x[:n], y[:n])
	if m > n {
		c = subVW(z[n:], x[n:], c)
	}
	if c != 0 {
		panic("underflow")
	}

	return z.norm()
}

func (x nat) cmp(y nat) (r int) {
	m := len(x)
	n := len(y)
	if m != n || m == 0 {
		switch {
		case m < n:
			r = -1
		case m > n:
			r = 1
		}
		return
	}

	i := m - 1
	for i > 0 && x[i] == y[i] {
		i--
	}

	switch {
	case x[i] < y[i]:
		r = -1
	case x[i] > y[i]:
		r = 1
	}
	return
}

// montgomery computes z mod m = x*y*2**(-n*_W) mod m,
// assuming k = -1/m mod 2**_W.
// z is used for storing the result which is returned;
// z must not alias x, y or m.
// See Gueron, "Efficient Software Implementations of Modular Exponentiation".
// https://eprint.iacr.org/2011/239.pdf
// In the terminology of that paper, this is an "Almost Montgomery Multiplication":
// x and y are required to satisfy 0 <= z < 2**(n*_W) and then the result
// z is guaranteed to satisfy 0 <= z < 2**(n*_W), but it may not be < m.
func (z nat) montgomery(x, y, m nat, k Word, n int) nat {
	// This code assumes x, y, m are all the same length, n.
	// (required by addMulVVW and the for loop).
	// It also assumes that x, y are already reduced mod m,
	// or else the result will not be properly reduced.
	if len(x) != n || len(y) != n || len(m) != n {
		panic("math/big: mismatched montgomery number lengths")
	}
	z = z.make(n * 2)
	clear(z)
	var c Word
	for i := 0; i < n; i++ {
		d := y[i]
		c2 := addMulVVWW(z[i:n+i], z[i:n+i], x, d, 0)
		t := z[i] * k
		c3 := addMulVVWW(z[i:n+i], z[i:n+i], m, t, 0)
		cx := c + c2
		cy := cx + c3
		z[n+i] = cy
		if cx < c2 || cy < c3 {
			c = 1
		} else {
			c = 0
		}
	}
	if c != 0 {
		subVV(z[:n], z[n:], m)
	} else {
		copy(z[:n], z[n:])
	}
	return z[:n]
}

// alias reports whether x and y share the same base array.
//
// Note: alias assumes that the capacity of underlying arrays
// is never changed for nat values; i.e. that there are
// no 3-operand slice expressions in this code (or worse,
// reflect-based operations to the same effect).
func alias(x, y nat) bool {
	return cap(x) > 0 && cap(y) > 0 && &x[0:cap(x)][cap(x)-1] == &y[0:cap(y)][cap(y)-1]
}

// addTo implements z += x; z must be long enough.
// (we don't use nat.add because we need z to stay the same
// slice, and we don't need to normalize z after each addition)
func addTo(z, x nat) {
	if n := len(x); n > 0 {
		if c := addVV(z[:n], z[:n], x[:n]); c != 0 {
			if n < len(z) {
				addVW(z[n:], z[n:], c)
			}
		}
	}
}

// mulRange computes the product of all the unsigned integers in the
// range [a, b] inclusively. If a > b (empty range), the result is 1.
// The caller may pass stk == nil to request that mulRange obtain and release one itself.
func (z nat) mulRange(stk *stack, a, b uint64) nat {
	switch {
	case a == 0:
		// cut long ranges short (optimization)
		return z.setUint64(0)
	case a > b:
		return z.setUint64(1)
	case a == b:
		return z.setUint64(a)
	case a+1 == b:
		return z.mul(stk, nat(nil).setUint64(a), nat(nil).setUint64(b))
	}

	if stk == nil {
		stk = getStack()
		defer stk.free()
	}

	m := a + (b-a)/2 // avoid overflow
	return z.mul(stk, nat(nil).mulRange(stk, a, m), nat(nil).mulRange(stk, m+1, b))
}

// A stackInner provides temporary storage for complex calculations
// such as multiplication and division.
// It should only be used by [stack], below.
type stackInner struct {
	w []Word
}

var stackPool sync.Pool // pool of *stackInner

// getStack returns a temporary stack.
// The caller must call [stack.free] to give up use of the stack when finished.
func getStackInner() *stackInner {
	s, _ := stackPool.Get().(*stackInner)
	if s == nil {
		s = new(stackInner)
	}
	return s
}

// free returns the stack for use by another calculation.
func (s *stackInner) free() {
	s.w = s.w[:0]
	stackPool.Put(s)
}

// save returns the current stack pointer.
// A future call to restore with the same value
// frees any temporaries allocated on the stack after the call to save.
func (s *stackInner) save() int {
	return len(s.w)
}

// restore restores the stack pointer to n.
// It is almost always invoked as
//
//	defer stk.restore(stk.save())
//
// which makes sure to pop any temporaries allocated in the current function
// from the stack before returning.
func (s *stackInner) restore(n int) {
	s.w = s.w[:n]
}

// nat returns a nat of n words, allocated on the stack.
func (s *stackInner) nat(n int) nat {
	nr := (n + 3) &^ 3 // round up to multiple of 4
	off := len(s.w)
	s.w = slices.Grow(s.w, nr)
	s.w = s.w[:off+nr]
	x := s.w[off : off+n : off+n]
	if n > 0 {
		x[0] = 0xfedcb
	}
	return x
}

// A stack provides temporary storage for complex calculations
// such as multiplication and division.
// In general, if a function takes a *stack, it expects a non-nil *stack.
// However, certain functions may allow passing a nil *stack instead,
// so that they can handle trivial stack-free cases without forcing the
// caller to obtain and free a stack that will be unused. These functions
// document that they accept a nil *stack in their doc comments.
type stack struct {
	si *stackInner
}

func getStack() *stack {
	return &stack{}
}
func (s *stack) free() {
	si := s.si
	if si != nil {
		si.free()
	}
}
func (s *stack) save() int {
	si := s.si
	if si == nil {
		return 0
	}
	return si.save()
}
func (s *stack) restore(n int) {
	si := s.si
	if si == nil {
		return
	}
	si.restore(n)
}
func (s *stack) nat(n int) nat {
	si := s.si
	if si == nil {
		if n <= 4 {
			// For small allocations, just ask the allocator.
			// It isn't worth pooling these allocations.
			// See issue 73999.
			r := slices.Grow(nat(nil), n)
			r = r[:n]
			if n > 0 {
				r[0] = 0xabcdef
			}
			return r
		}
		si, _ = stackPool.Get().(*stackInner)
		if si == nil {
			si = new(stackInner)
		}
		s.si = si
	}
	return si.nat(n)
}

// bitLen returns the length of x in bits.
// Unlike most methods, it works even if x is not normalized.
func (x nat) bitLen() int {
	// This function is used in cryptographic operations. It must not leak
	// anything but the Int's sign and bit size through side-channels. Any
	// changes must be reviewed by a security expert.
	if i := len(x) - 1; i >= 0 {
		// bits.Len uses a lookup table for the low-order bits on some
		// architectures. Neutralize any input-dependent behavior by setting all
		// bits after the first one bit.
		top := uint(x[i])
		top |= top >> 1
		top |= top >> 2
		top |= top >> 4
		top |= top >> 8
		top |= top >> 16
		top |= top >> 16 >> 16 // ">> 32" doesn't compile on 32-bit architectures
		return i*_W + bits.Len(top)
	}
	return 0
}

// trailingZeroBits returns the number of consecutive least significant zero
// bits of x.
func (x nat) trailingZeroBits() uint {
	if len(x) == 0 {
		return 0
	}
	var i uint
	for x[i] == 0 {
		i++
	}
	// x[i] != 0
	return i*_W + uint(bits.TrailingZeros(uint(x[i])))
}

// isPow2 returns i, true when x == 2**i and 0, false otherwise.
func (x nat) isPow2() (uint, bool) {
	var i uint
	for x[i] == 0 {
		i++
	}
	if i == uint(len(x))-1 && x[i]&(x[i]-1) == 0 {
		return i*_W + uint(bits.TrailingZeros(uint(x[i]))), true
	}
	return 0, false
}

func same(x, y nat) bool {
	return len(x) == len(y) && len(x) > 0 && &x[0] == &y[0]
}

// z = x << s
func (z nat) lsh(x nat, s uint) nat {
	if s == 0 {
		if same(z, x) {
			return z
		}
		if !alias(z, x) {
			return z.set(x)
		}
	}

	m := len(x)
	if m == 0 {
		return z[:0]
	}
	// m > 0

	n := m + int(s/_W)
	z = z.make(n + 1)
	if s %= _W; s == 0 {
		copy(z[n-m:n], x)
		z[n] = 0
	} else {
		z[n] = lshVU(z[n-m:n], x, s)
	}
	clear(z[0 : n-m])

	return z.norm()
}

// z = x >> s
func (z nat) rsh(x nat, s uint) nat {
	if s == 0 {
		if same(z, x) {
			return z
		}
		if !alias(z, x) {
			return z.set(x)
		}
	}

	m := len(x)
	n := m - int(s/_W)
	if n <= 0 {
		return z[:0]
	}
	// n > 0

	z = z.make(n)
	if s %= _W; s == 0 {
		copy(z, x[m-n:])
	} else {
		rshVU(z, x[m-n:], s)
	}

	return z.norm()
}

func (z nat) setBit(x nat, i uint, b uint) nat {
	j := int(i / _W)
	m := Word(1) << (i % _W)
	n := len(x)
	switch b {
	case 0:
		z = z.make(n)
		copy(z, x)
		if j >= n {
			// no need to grow
			return z
		}
		z[j] &^= m
		return z.norm()
	case 1:
		if j >= n {
			z = z.make(j + 1)
			clear(z[n:])
		} else {
			z = z.make(n)
		}
		copy(z, x)
		z[j] |= m
		// no need to normalize
		return z
	}
	panic("set bit is not 0 or 1")
}

// bit returns the value of the i'th bit, with lsb == bit 0.
func (x nat) bit(i uint) uint {
	j := i / _W
	if j >= uint(len(x)) {
		return 0
	}
	// 0 <= j < len(x)
	return uint(x[j] >> (i % _W) & 1)
}

// sticky returns 1 if there's a 1 bit within the
// i least significant bits, otherwise it returns 0.
func (x nat) sticky(i uint) uint {
	j := i / _W
	if j >= uint(len(x)) {
		if len(x) == 0 {
			return 0
		}
		return 1
	}
	// 0 <= j < len(x)
	for _, x := range x[:j] {
		if x != 0 {
			return 1
		}
	}
	if x[j]<<(_W-i%_W) != 0 {
		return 1
	}
	return 0
}

func (z nat) and(x, y nat) nat {
	m := len(x)
	n := len(y)
	if m > n {
		m = n
	}
	// m <= n

	z = z.make(m)
	for i := 0; i < m; i++ {
		z[i] = x[i] & y[i]
	}

	return z.norm()
}

// trunc returns z = x mod 2ⁿ.
func (z nat) trunc(x nat, n uint) nat {
	w := (n + _W - 1) / _W
	if uint(len(x)) < w {
		return z.set(x)
	}
	z = z.make(int(w))
	copy(z, x)
	if n%_W != 0 {
		z[len(z)-1] &= 1<<(n%_W) - 1
	}
	return z.norm()
}

func (z nat) andNot(x, y nat) nat {
	m := len(x)
	n := len(y)
	if n > m {
		n = m
	}
	// m >= n

	z = z.make(m)
	for i := 0; i < n; i++ {
		z[i] = x[i] &^ y[i]
	}
	copy(z[n:m], x[n:m])

	return z.norm()
}

func (z nat) or(x, y nat) nat {
	m := len(x)
	n := len(y)
	s := x
	if m < n {
		n, m = m, n
		s = y
	}
	// m >= n

	z = z.make(m)
	for i := 0; i < n; i++ {
		z[i] = x[i] | y[i]
	}
	copy(z[n:m], s[n:m])

	return z.norm()
}

func (z nat) xor(x, y nat) nat {
	m := len(x)
	n := len(y)
	s := x
	if m < n {
		n, m = m, n
		s = y
	}
	// m >= n

	z = z.make(m)
	for i := 0; i < n; i++ {
		z[i] = x[i] ^ y[i]
	}
	copy(z[n:m], s[n:m])

	return z.norm()
}

// random creates a random integer in [0..limit), using the space in z if
// possible. n is the bit length of limit.
func (z nat) random(rand *rand.Rand, limit nat, n int) nat {
	if alias(z, limit) {
		z = nil // z is an alias for limit - cannot reuse
	}
	z = z.make(len(limit))

	bitLengthOfMSW := uint(n % _W)
	if bitLengthOfMSW == 0 {
		bitLengthOfMSW = _W
	}
	mask := Word((1 << bitLengthOfMSW) - 1)

	for {
		switch _W {
		case 32:
			for i := range z {
				z[i] = Word(rand.Uint32())
			}
		case 64:
			for i := range z {
				z[i] = Word(rand.Uint32()) | Word(rand.Uint32())<<32
			}
		default:
			panic("unknown word size")
		}
		z[len(limit)-1] &= mask
		if z.cmp(limit) < 0 {
			break
		}
	}

	return z.norm()
}

// If m != 0 (i.e., len(m) != 0), expNN sets z to x**y mod m;
// otherwise it sets z to x**y. The result is the value of z.
// The caller may pass stk == nil to request that expNN obtain and release one itself.
func (z nat) expNN(stk *stack, x, y, m nat, slow bool) nat {
	if alias(z, x) || alias(z, y) {
		// We cannot allow in-place modification of x or y.
		z = nil
	}

	// x**y mod 1 == 0
	if len(m) == 1 && m[0] == 1 {
		return z.setWord(0)
	}
	// m == 0 || m > 1

	// x**0 == 1
	if len(y) == 0 {
		return z.setWord(1)
	}
	// y > 0

	// 0**y = 0
	if len(x) == 0 {
		return z.setWord(0)
	}
	// x > 0

	// 1**y = 1
	if len(x) == 1 && x[0] == 1 {
		return z.setWord(1)
	}
	// x > 1

	// x**1 == x
	if len(y) == 1 && y[0] == 1 && len(m) == 0 {
		return z.set(x)
	}
	if stk == nil {
		stk = getStack()
		defer stk.free()
	}
	if len(y) == 1 && y[0] == 1 { // len(m) > 0
		return z.rem(stk, x, m)
	}

	// y > 1

	if len(m) != 0 {
		// We likely end up being as long as the modulus.
		z = z.make(len(m))

		// If the exponent is large, we use the Montgomery method for odd values,
		// and a 4-bit, windowed exponentiation for powers of two,
		// and a CRT-decomposed Montgomery method for the remaining values
		// (even values times non-trivial odd values, which decompose into one
		// instance of each of the first two cases).
		if len(y) > 1 && !slow {
			if m[0]&1 == 1 {
				return z.expNNMontgomery(stk, x, y, m)
			}
			if logM, ok := m.isPow2(); ok {
				return z.expNNWindowed(stk, x, y, logM)
			}
			return z.expNNMontgomeryEven(stk, x, y, m)
		}
	}

	z = z.set(x)
	v := y[len(y)-1] // v > 0 because y is normalized and y > 0
	shift := nlz(v) + 1
	v <<= shift
	var q nat

	const mask = 1 << (_W - 1)

	// We walk through the bits of the exponent one by one. Each time we
	// see a bit, we square, thus doubling the power. If the bit is a one,
	// we also multiply by x, thus adding one to the power.

	w := _W - int(shift)
	// zz and r are used to avoid allocating in mul and div as
	// otherwise the arguments would alias.
	var zz, r nat
	for j := 0; j < w; j++ {
		zz = zz.sqr(stk, z)
		zz, z = z, zz

		if v&mask != 0 {
			zz = zz.mul(stk, z, x)
			zz, z = z, zz
		}

		if len(m) != 0 {
			zz, r = zz.div(stk, r, z, m)
			zz, r, q, z = q, z, zz, r
		}

		v <<= 1
	}

	for i := len(y) - 2; i >= 0; i-- {
		v = y[i]

		for j := 0; j < _W; j++ {
			zz = zz.sqr(stk, z)
			zz, z = z, zz

			if v&mask != 0 {
				zz = zz.mul(stk, z, x)
				zz, z = z, zz
			}

			if len(m) != 0 {
				zz, r = zz.div(stk, r, z, m)
				zz, r, q, z = q, z, zz, r
			}

			v <<= 1
		}
	}

	return z.norm()
}

// expNNMontgomeryEven calculates x**y mod m where m = m1 × m2 for m1 = 2ⁿ and m2 odd.
// It uses two recursive calls to expNN for x**y mod m1 and x**y mod m2
// and then uses the Chinese Remainder Theorem to combine the results.
// The recursive call using m1 will use expNNWindowed,
// while the recursive call using m2 will use expNNMontgomery.
// For more details, see Ç. K. Koç, “Montgomery Reduction with Even Modulus”,
// IEE Proceedings: Computers and Digital Techniques, 141(5) 314-316, September 1994.
// http://www.people.vcu.edu/~jwang3/CMSC691/j34monex.pdf
func (z nat) expNNMontgomeryEven(stk *stack, x, y, m nat) nat {
	// Split m = m₁ × m₂ where m₁ = 2ⁿ
	n := m.trailingZeroBits()
	m1 := nat(nil).lsh(natOne, n)
	m2 := nat(nil).rsh(m, n)

	// We want z = x**y mod m.
	// z₁ = x**y mod m1 = (x**y mod m) mod m1 = z mod m1
	// z₂ = x**y mod m2 = (x**y mod m) mod m2 = z mod m2
	// (We are using the math/big convention for names here,
	// where the computation is z = x**y mod m, so its parts are z1 and z2.
	// The paper is computing x = a**e mod n; it refers to these as x2 and z1.)
	z1 := nat(nil).expNN(stk, x, y, m1, false)
	z2 := nat(nil).expNN(stk, x, y, m2, false)

	// Reconstruct z from z₁, z₂ using CRT, using algorithm from paper,
	// which uses only a single modInverse (and an easy one at that).
	//	p = (z₁ - z₂) × m₂⁻¹ (mod m₁)
	//	z = z₂ + p × m₂
	// The final addition is in range because:
	//	z = z₂ + p × m₂
	//	  ≤ z₂ + (m₁-1) × m₂
	//	  < m₂ + (m₁-1) × m₂
	//	  = m₁ × m₂
	//	  = m.
	z = z.set(z2)

	// Compute (z₁ - z₂) mod m1 [m1 == 2**n] into z1.
	z1 = z1.subMod2N(z1, z2, n)

	// Reuse z2 for p = (z₁ - z₂) [in z1] * m2⁻¹ (mod m₁ [= 2ⁿ]).
	m2inv := nat(nil).modInverse(m2, m1)
	z2 = z2.mul(stk, z1, m2inv)
	z2 = z2.trunc(z2, n)

	// Reuse z1 for p * m2.
	z = z.add(z, z1.mul(stk, z2, m2))

	return z
}

// expNNWindowed calculates x**y mod m using a fixed, 4-bit window,
// where m = 2**logM.
func (z nat) expNNWindowed(stk *stack, x, y nat, logM uint) nat {
	if len(y) <= 1 {
		panic("big: misuse of expNNWindowed")
	}
	if x[0]&1 == 0 {
		// len(y) > 1, so y  > logM.
		// x is even, so x**y is a multiple of 2**y which is a multiple of 2**logM.
		return z.setWord(0)
	}
	if logM == 1 {
		return z.setWord(1)
	}

	// zz is used to avoid allocating in mul as otherwise
	// the arguments would alias.
	defer stk.restore(stk.save())
	w := int((logM + _W - 1) / _W)
	zz := stk.nat(w)

	const n = 4
	// powers[i] contains x^i.
	var powers [1 << n]nat
	for i := range powers {
		powers[i] = stk.nat(w)
	}
	powers[0] = powers[0].set(natOne)
	powers[1] = powers[1].trunc(x, logM)
	for i := 2; i < 1<<n; i += 2 {
		p2, p, p1 := &powers[i/2], &powers[i], &powers[i+1]
		*p = p.sqr(stk, *p2)
		*p = p.trunc(*p, logM)
		*p1 = p1.mul(stk, *p, x)
		*p1 = p1.trunc(*p1, logM)
	}

	// Because phi(2**logM) = 2**(logM-1), x**(2**(logM-1)) = 1,
	// so we can compute x**(y mod 2**(logM-1)) instead of x**y.
	// That is, we can throw away all but the bottom logM-1 bits of y.
	// Instead of allocating a new y, we start reading y at the right word
	// and truncate it appropriately at the start of the loop.
	i := len(y) - 1
	mtop := int((logM - 2) / _W) // -2 because the top word of N bits is the (N-1)/W'th word.
	mmask := ^Word(0)
	if mbits := (logM - 1) & (_W - 1); mbits != 0 {
		mmask = (1 << mbits) - 1
	}
	if i > mtop {
		i = mtop
	}
	advance := false
	z = z.setWord(1)
	for ; i >= 0; i-- {
		yi := y[i]
		if i == mtop {
			yi &= mmask
		}
		for j := 0; j < _W; j += n {
			if advance {
				// Account for use of 4 bits in previous iteration.
				// Unrolled loop for significant performance
				// gain. Use go test -bench=".*" in crypto/rsa
				// to check performance before making changes.
				zz = zz.sqr(stk, z)
				zz, z = z, zz
				z = z.trunc(z, logM)

				zz = zz.sqr(stk, z)
				zz, z = z, zz
				z = z.trunc(z, logM)

				zz = zz.sqr(stk, z)
				zz, z = z, zz
				z = z.trunc(z, logM)

				zz = zz.sqr(stk, z)
				zz, z = z, zz
				z = z.trunc(z, logM)
			}

			zz = zz.mul(stk, z, powers[yi>>(_W-n)])
			zz, z = z, zz
			z = z.trunc(z, logM)

			yi <<= n
			advance = true
		}
	}

	return z.norm()
}

// expNNMontgomery calculates x**y mod m using a fixed, 4-bit window.
// Uses Montgomery representation.
func (z nat) expNNMontgomery(stk *stack, x, y, m nat) nat {
	numWords := len(m)

	// We want the lengths of x and m to be equal.
	// It is OK if x >= m as long as len(x) == len(m).
	if len(x) > numWords {
		_, x = nat(nil).div(stk, nil, x, m)
		// Note: now len(x) <= numWords, not guaranteed ==.
	}
	if len(x) < numWords {
		rr := make(nat, numWords)
		copy(rr, x)
		x = rr
	}

	// Ideally the precomputations would be performed outside, and reused
	// k0 = -m**-1 mod 2**_W. Algorithm from: Dumas, J.G. "On Newton–Raphson
	// Iteration for Multiplicative Inverses Modulo Prime Powers".
	k0 := 2 - m[0]
	t := m[0] - 1
	for i := 1; i < _W; i <<= 1 {
		t *= t
		k0 *= (t + 1)
	}
	k0 = -k0

	// RR = 2**(2*_W*len(m)) mod m
	RR := nat(nil).setWord(1)
	zz := nat(nil).lsh(RR, uint(2*numWords*_W))
	_, RR = nat(nil).div(stk, RR, zz, m)
	if len(RR) < numWords {
		zz = zz.make(numWords)
		copy(zz, RR)
		RR = zz
	}
	// one = 1, with equal length to that of m
	one := make(nat, numWords)
	one[0] = 1

	const n = 4
	// powers[i] contains x^i
	var powers [1 << n]nat
	powers[0] = powers[0].montgomery(one, RR, m, k0, numWords)
	powers[1] = powers[1].montgomery(x, RR, m, k0, numWords)
	for i := 2; i < 1<<n; i++ {
		powers[i] = powers[i].montgomery(powers[i-1], powers[1], m, k0, numWords)
	}

	// initialize z = 1 (Montgomery 1)
	z = z.make(numWords)
	copy(z, powers[0])

	zz = zz.make(numWords)

	// same windowed exponent, but with Montgomery multiplications
	for i := len(y) - 1; i >= 0; i-- {
		yi := y[i]
		for j := 0; j < _W; j += n {
			if i != len(y)-1 || j != 0 {
				zz = zz.montgomery(z, z, m, k0, numWords)
				z = z.montgomery(zz, zz, m, k0, numWords)
				zz = zz.montgomery(z, z, m, k0, numWords)
				z = z.montgomery(zz, zz, m, k0, numWords)
			}
			zz = zz.montgomery(z, powers[yi>>(_W-n)], m, k0, numWords)
			z, zz = zz, z
			yi <<= n
		}
	}
	// convert to regular number
	zz = zz.montgomery(z, one, m, k0, numWords)

	// One last reduction, just in case.
	// See golang.org/issue/13907.
	if zz.cmp(m) >= 0 {
		// Common case is m has high bit set; in that case,
		// since zz is the same length as m, there can be just
		// one multiple of m to remove. Just subtract.
		// We think that the subtract should be sufficient in general,
		// so do that unconditionally, but double-check,
		// in case our beliefs are wrong.
		// The div is not expected to be reached.
		zz = zz.sub(zz, m)
		if zz.cmp(m) >= 0 {
			_, zz = nat(nil).div(stk, nil, zz, m)
		}
	}

	return zz.norm()
}

// bytes writes the value of z into buf using big-endian encoding.
// The value of z is encoded in the slice buf[i:]. If the value of z
// cannot be represented in buf, bytes panics. The number i of unused
// bytes at the beginning of buf is returned as result.
func (z nat) bytes(buf []byte) (i int) {
	// This function is used in cryptographic operations. It must not leak
	// anything but the Int's sign and bit size through side-channels. Any
	// changes must be reviewed by a security expert.
	i = len(buf)
	for _, d := range z {
		for j := 0; j < _S; j++ {
			i--
			if i >= 0 {
				buf[i] = byte(d)
			} else if byte(d) != 0 {
				panic("math/big: buffer too small to fit value")
			}
			d >>= 8
		}
	}

	if i < 0 {
		i = 0
	}
	for i < len(buf) && buf[i] == 0 {
		i++
	}

	return
}

// bigEndianWord returns the contents of buf interpreted as a big-endian encoded Word value.
func bigEndianWord(buf []byte) Word {
	if _W == 64 {
		return Word(byteorder.BEUint64(buf))
	}
	return Word(byteorder.BEUint32(buf))
}

// setBytes interprets buf as the bytes of a big-endian unsigned
// integer, sets z to that value, and returns z.
func (z nat) setBytes(buf []byte) nat {
	z = z.make((len(buf) + _S - 1) / _S)

	i := len(buf)
	for k := 0; i >= _S; k++ {
		z[k] = bigEndianWord(buf[i-_S : i])
		i -= _S
	}
	if i > 0 {
		var d Word
		for s := uint(0); i > 0; s += 8 {
			d |= Word(buf[i-1]) << s
			i--
		}
		z[len(z)-1] = d
	}

	return z.norm()
}

// sqrt sets z = ⌊√x⌋
// The caller may pass stk == nil to request that sqrt obtain and release one itself.
func (z nat) sqrt(stk *stack, x nat) nat {
	if x.cmp(natOne) <= 0 {
		return z.set(x)
	}
	if alias(z, x) {
		z = nil
	}

	if stk == nil {
		stk = getStack()
		defer stk.free()
	}

	// Start with value known to be too large and repeat "z = ⌊(z + ⌊x/z⌋)/2⌋" until it stops getting smaller.
	// See Brent and Zimmermann, Modern Computer Arithmetic, Algorithm 1.13 (SqrtInt).
	// https://members.loria.fr/PZimmermann/mca/pub226.html
	// If x is one less than a perfect square, the sequence oscillates between the correct z and z+1;
	// otherwise it converges to the correct z and stays there.
	var z1, z2 nat
	z1 = z
	z1 = z1.setUint64(1)
	z1 = z1.lsh(z1, uint(x.bitLen()+1)/2) // must be ≥ √x
	for n := 0; ; n++ {
		z2, _ = z2.div(stk, nil, x, z1)
		z2 = z2.add(z2, z1)
		z2 = z2.rsh(z2, 1)
		if z2.cmp(z1) >= 0 {
			// z1 is answer.
			// Figure out whether z1 or z2 is currently aliased to z by looking at loop count.
			if n&1 == 0 {
				return z1
			}
			return z.set(z1)
		}
		z1, z2 = z2, z1
	}
}

// subMod2N returns z = (x - y) mod 2ⁿ.
func (z nat) subMod2N(x, y nat, n uint) nat {
	if uint(x.bitLen()) > n {
		if alias(z, x) {
			// ok to overwrite x in place
			x = x.trunc(x, n)
		} else {
			x = nat(nil).trunc(x, n)
		}
	}
	if uint(y.bitLen()) > n {
		if alias(z, y) {
			// ok to overwrite y in place
			y = y.trunc(y, n)
		} else {
			y = nat(nil).trunc(y, n)
		}
	}
	if x.cmp(y) >= 0 {
		return z.sub(x, y)
	}
	// x - y < 0; x - y mod 2ⁿ = x - y + 2ⁿ = 2ⁿ - (y - x) = 1 + 2ⁿ-1 - (y - x) = 1 + ^(y - x).
	z = z.sub(y, x)
	for uint(len(z))*_W < n {
		z = append(z, 0)
	}
	for i := range z {
		z[i] = ^z[i]
	}
	z = z.trunc(z, n)
	return z.add(z, natOne)
}

```

// === FILE: references!/go/src/math/big/natconv.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements nat-to-string conversion functions.

package big

import (
	"errors"
	"fmt"
	"io"
	"math"
	"math/bits"
	"slices"
	"sync"
)

const digits = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// Note: MaxBase = len(digits), but it must remain an untyped rune constant
//       for API compatibility.

// MaxBase is the largest number base accepted for string conversions.
const MaxBase = 10 + ('z' - 'a' + 1) + ('Z' - 'A' + 1)
const maxBaseSmall = 10 + ('z' - 'a' + 1)

// maxPow returns (b**n, n) such that b**n is the largest power b**n <= _M.
// For instance maxPow(10) == (1e19, 19) for 19 decimal digits in a 64bit Word.
// In other words, at most n digits in base b fit into a Word.
// TODO(gri) replace this with a table, generated at build time.
func maxPow(b Word) (p Word, n int) {
	p, n = b, 1 // assuming b <= _M
	for max := _M / b; p <= max; {
		// p == b**n && p <= max
		p *= b
		n++
	}
	// p == b**n && p <= _M
	return
}

// pow returns x**n for n > 0, and 1 otherwise.
func pow(x Word, n int) (p Word) {
	// n == sum of bi * 2**i, for 0 <= i < imax, and bi is 0 or 1
	// thus x**n == product of x**(2**i) for all i where bi == 1
	// (Russian Peasant Method for exponentiation)
	p = 1
	for n > 0 {
		if n&1 != 0 {
			p *= x
		}
		x *= x
		n >>= 1
	}
	return
}

// scan errors
var (
	errNoDigits = errors.New("number has no digits")
	errInvalSep = errors.New("'_' must separate successive digits")
)

// scan scans the number corresponding to the longest possible prefix
// from r representing an unsigned number in a given conversion base.
// scan returns the corresponding natural number res, the actual base b,
// a digit count, and a read or syntax error err, if any.
//
// For base 0, an underscore character “_” may appear between a base
// prefix and an adjacent digit, and between successive digits; such
// underscores do not change the value of the number, or the returned
// digit count. Incorrect placement of underscores is reported as an
// error if there are no other errors. If base != 0, underscores are
// not recognized and thus terminate scanning like any other character
// that is not a valid radix point or digit.
//
//	number    = mantissa | prefix pmantissa .
//	prefix    = "0" [ "b" | "B" | "o" | "O" | "x" | "X" ] .
//	mantissa  = digits "." [ digits ] | digits | "." digits .
//	pmantissa = [ "_" ] digits "." [ digits ] | [ "_" ] digits | "." digits .
//	digits    = digit { [ "_" ] digit } .
//	digit     = "0" ... "9" | "a" ... "z" | "A" ... "Z" .
//
// Unless fracOk is set, the base argument must be 0 or a value between
// 2 and MaxBase. If fracOk is set, the base argument must be one of
// 0, 2, 8, 10, or 16. Providing an invalid base argument leads to a run-
// time panic.
//
// For base 0, the number prefix determines the actual base: A prefix of
// “0b” or “0B” selects base 2, “0o” or “0O” selects base 8, and
// “0x” or “0X” selects base 16. If fracOk is false, a “0” prefix
// (immediately followed by digits) selects base 8 as well. Otherwise,
// the selected base is 10 and no prefix is accepted.
//
// If fracOk is set, a period followed by a fractional part is permitted.
// The result value is computed as if there were no period present; and
// the count value is used to determine the fractional part.
//
// For bases <= 36, lower and upper case letters are considered the same:
// The letters 'a' to 'z' and 'A' to 'Z' represent digit values 10 to 35.
// For bases > 36, the upper case letters 'A' to 'Z' represent the digit
// values 36 to 61.
//
// A result digit count > 0 corresponds to the number of (non-prefix) digits
// parsed. A digit count <= 0 indicates the presence of a period (if fracOk
// is set, only), and -count is the number of fractional digits found.
// In this case, the actual value of the scanned number is res * b**count.
func (z nat) scan(r io.ByteScanner, base int, fracOk bool) (res nat, b, count int, err error) {
	// Reject invalid bases.
	baseOk := base == 0 ||
		!fracOk && 2 <= base && base <= MaxBase ||
		fracOk && (base == 2 || base == 8 || base == 10 || base == 16)
	if !baseOk {
		panic(fmt.Sprintf("invalid number base %d", base))
	}

	// prev encodes the previously seen char: it is one
	// of '_', '0' (a digit), or '.' (anything else). A
	// valid separator '_' may only occur after a digit
	// and if base == 0.
	prev := '.'
	invalSep := false

	// one char look-ahead
	ch, err := r.ReadByte()

	// Determine actual base.
	b, prefix := base, 0
	if base == 0 {
		// Actual base is 10 unless there's a base prefix.
		b = 10
		if err == nil && ch == '0' {
			prev = '0'
			count = 1
			ch, err = r.ReadByte()
			if err == nil {
				// possibly one of 0b, 0B, 0o, 0O, 0x, 0X
				switch ch {
				case 'b', 'B':
					b, prefix = 2, 'b'
				case 'o', 'O':
					b, prefix = 8, 'o'
				case 'x', 'X':
					b, prefix = 16, 'x'
				default:
					if !fracOk {
						b, prefix = 8, '0'
					}
				}
				if prefix != 0 {
					count = 0 // prefix is not counted
					if prefix != '0' {
						ch, err = r.ReadByte()
					}
				}
			}
		}
	}

	// Convert string.
	// Algorithm: Collect digits in groups of at most n digits in di.
	// For bases that pack exactly into words (2, 4, 16), append di's
	// directly to the int representation and then reverse at the end (bn==0 marks this case).
	// For other bases, use mulAddWW for every such group to shift
	// z up one group and add di to the result.
	// With more cleverness we could also handle binary bases like 8 and 32
	// (corresponding to 3-bit and 5-bit chunks) that don't pack nicely into
	// words, but those are not too important.
	z = z[:0]
	b1 := Word(b)
	var bn Word // b1**n (or 0 for the special bit-packing cases b=2,4,16)
	var n int   // max digits that fit into Word
	switch b {
	case 2: // 1 bit per digit
		n = _W
	case 4: // 2 bits per digit
		n = _W / 2
	case 16: // 4 bits per digit
		n = _W / 4
	default:
		bn, n = maxPow(b1)
	}
	di := Word(0) // 0 <= di < b1**i < bn
	i := 0        // 0 <= i < n
	dp := -1      // position of decimal point
	for err == nil {
		if ch == '.' && fracOk {
			fracOk = false
			if prev == '_' {
				invalSep = true
			}
			prev = '.'
			dp = count
		} else if ch == '_' && base == 0 {
			if prev != '0' {
				invalSep = true
			}
			prev = '_'
		} else {
			// convert rune into digit value d1
			var d1 Word
			switch {
			case '0' <= ch && ch <= '9':
				d1 = Word(ch - '0')
			case 'a' <= ch && ch <= 'z':
				d1 = Word(ch - 'a' + 10)
			case 'A' <= ch && ch <= 'Z':
				if b <= maxBaseSmall {
					d1 = Word(ch - 'A' + 10)
				} else {
					d1 = Word(ch - 'A' + maxBaseSmall)
				}
			default:
				d1 = MaxBase + 1
			}
			if d1 >= b1 {
				r.UnreadByte() // ch does not belong to number anymore
				break
			}
			prev = '0'
			count++

			// collect d1 in di
			di = di*b1 + d1
			i++

			// if di is "full", add it to the result
			if i == n {
				if bn == 0 {
					z = append(z, di)
				} else {
					z = z.mulAddWW(z, bn, di)
				}
				di = 0
				i = 0
			}
		}

		ch, err = r.ReadByte()
	}

	if err == io.EOF {
		err = nil
	}

	// other errors take precedence over invalid separators
	if err == nil && (invalSep || prev == '_') {
		err = errInvalSep
	}

	if count == 0 {
		// no digits found
		if prefix == '0' {
			// there was only the octal prefix 0 (possibly followed by separators and digits > 7);
			// interpret as decimal 0
			return z[:0], 10, 1, err
		}
		err = errNoDigits // fall through; result will be 0
	}

	if bn == 0 {
		if i > 0 {
			// Add remaining digit chunk to result.
			// Left-justify group's digits; will shift back down after reverse.
			z = append(z, di*pow(b1, n-i))
		}
		slices.Reverse(z)
		z = z.norm()
		if i > 0 {
			z = z.rsh(z, uint(n-i)*uint(_W/n))
		}
	} else {
		if i > 0 {
			// Add remaining digit chunk to result.
			z = z.mulAddWW(z, pow(b1, i), di)
		}
	}
	res = z

	// adjust count for fraction, if any
	if dp >= 0 {
		// 0 <= dp <= count
		count = dp - count
	}

	return
}

// utoa converts x to an ASCII representation in the given base;
// base must be between 2 and MaxBase, inclusive.
func (x nat) utoa(base int) []byte {
	return x.itoa(false, base)
}

// itoa is like utoa but it prepends a '-' if neg && x != 0.
func (x nat) itoa(neg bool, base int) []byte {
	if base < 2 || base > MaxBase {
		panic("invalid base")
	}

	// x == 0
	if len(x) == 0 {
		return []byte("0")
	}
	// len(x) > 0

	// allocate buffer for conversion
	i := int(float64(x.bitLen())/math.Log2(float64(base))) + 1 // off by 1 at most
	if neg {
		i++
	}
	s := make([]byte, i)

	// convert power of two and non power of two bases separately
	if b := Word(base); b == b&-b {
		// shift is base b digit size in bits
		shift := uint(bits.TrailingZeros(uint(b))) // shift > 0 because b >= 2
		mask := Word(1<<shift - 1)
		w := x[0]         // current word
		nbits := uint(_W) // number of unprocessed bits in w

		// convert less-significant words (include leading zeros)
		for k := 1; k < len(x); k++ {
			// convert full digits
			for nbits >= shift {
				i--
				s[i] = digits[w&mask]
				w >>= shift
				nbits -= shift
			}

			// convert any partial leading digit and advance to next word
			if nbits == 0 {
				// no partial digit remaining, just advance
				w = x[k]
				nbits = _W
			} else {
				// partial digit in current word w (== x[k-1]) and next word x[k]
				w |= x[k] << nbits
				i--
				s[i] = digits[w&mask]

				// advance
				w = x[k] >> (shift - nbits)
				nbits = _W - (shift - nbits)
			}
		}

		// convert digits of most-significant word w (omit leading zeros)
		for w != 0 {
			i--
			s[i] = digits[w&mask]
			w >>= shift
		}

	} else {
		stk := getStack()
		defer stk.free()

		bb, ndigits := maxPow(b)

		// construct table of successive squares of bb*leafSize to use in subdivisions
		// result (table != nil) <=> (len(x) > leafSize > 0)
		table := divisors(stk, len(x), b, ndigits, bb)

		// preserve x, create local copy for use by convertWords
		q := nat(nil).set(x)

		// convert q to string s in base b
		q.convertWords(stk, s, b, ndigits, bb, table)

		// strip leading zeros
		// (x != 0; thus s must contain at least one non-zero digit
		// and the loop will terminate)
		i = 0
		for s[i] == '0' {
			i++
		}
	}

	if neg {
		i--
		s[i] = '-'
	}

	return s[i:]
}

// Convert words of q to base b digits in s. If q is large, it is recursively "split in half"
// by nat/nat division using tabulated divisors. Otherwise, it is converted iteratively using
// repeated nat/Word division.
//
// The iterative method processes n Words by n divW() calls, each of which visits every Word in the
// incrementally shortened q for a total of n + (n-1) + (n-2) ... + 2 + 1, or n(n+1)/2 divW()'s.
// Recursive conversion divides q by its approximate square root, yielding two parts, each half
// the size of q. Using the iterative method on both halves means 2 * (n/2)(n/2 + 1)/2 divW()'s
// plus the expensive long div(). Asymptotically, the ratio is favorable at 1/2 the divW()'s, and
// is made better by splitting the subblocks recursively. Best is to split blocks until one more
// split would take longer (because of the nat/nat div()) than the twice as many divW()'s of the
// iterative approach. This threshold is represented by leafSize. Benchmarking of leafSize in the
// range 2..64 shows that values of 8 and 16 work well, with a 4x speedup at medium lengths and
// ~30x for 20000 digits. Use nat_test.go's BenchmarkLeafSize tests to optimize leafSize for
// specific hardware.
func (q nat) convertWords(stk *stack, s []byte, b Word, ndigits int, bb Word, table []divisor) {
	// split larger blocks recursively
	if table != nil {
		// len(q) > leafSize > 0
		var r nat
		index := len(table) - 1
		for len(q) > leafSize {
			// find divisor close to sqrt(q) if possible, but in any case < q
			maxLength := q.bitLen()     // ~= log2 q, or at of least largest possible q of this bit length
			minLength := maxLength >> 1 // ~= log2 sqrt(q)
			for index > 0 && table[index-1].nbits > minLength {
				index-- // desired
			}
			if table[index].nbits >= maxLength && table[index].bbb.cmp(q) >= 0 {
				index--
				if index < 0 {
					panic("internal inconsistency")
				}
			}

			// split q into the two digit number (q'*bbb + r) to form independent subblocks
			q, r = q.div(stk, r, q, table[index].bbb)

			// convert subblocks and collect results in s[:h] and s[h:]
			h := len(s) - table[index].ndigits
			r.convertWords(stk, s[h:], b, ndigits, bb, table[0:index])
			s = s[:h] // == q.convertWords(stk, s, b, ndigits, bb, table[0:index+1])
		}
	}

	// having split any large blocks now process the remaining (small) block iteratively
	i := len(s)
	var r Word
	if b == 10 {
		// hard-coding for 10 here speeds this up by 1.25x (allows for / and % by constants)
		for len(q) > 0 {
			// extract least significant, base bb "digit"
			q, r = q.divW(q, bb)
			for j := 0; j < ndigits && i > 0; j++ {
				i--
				// avoid % computation since r%10 == r - int(r/10)*10;
				// this appears to be faster for BenchmarkString10000Base10
				// and smaller strings (but a bit slower for larger ones)
				t := r / 10
				s[i] = '0' + byte(r-t*10)
				r = t
			}
		}
	} else {
		for len(q) > 0 {
			// extract least significant, base bb "digit"
			q, r = q.divW(q, bb)
			for j := 0; j < ndigits && i > 0; j++ {
				i--
				s[i] = digits[r%b]
				r /= b
			}
		}
	}

	// prepend high-order zeros
	for i > 0 { // while need more leading zeros
		i--
		s[i] = '0'
	}
}

// Split blocks greater than leafSize Words (or set to 0 to disable recursive conversion)
// Benchmark and configure leafSize using: go test -bench="Leaf"
//
//	8 and 16 effective on 3.0 GHz Xeon "Clovertown" CPU (128 byte cache lines)
//	8 and 16 effective on 2.66 GHz Core 2 Duo "Penryn" CPU
var leafSize int = 8 // number of Word-size binary values treat as a monolithic block

type divisor struct {
	bbb     nat // divisor
	nbits   int // bit length of divisor (discounting leading zeros) ~= log2(bbb)
	ndigits int // digit length of divisor in terms of output base digits
}

var cacheBase10 struct {
	sync.Mutex
	table [64]divisor // cached divisors for base 10
}

// expWW computes x**y
func (z nat) expWW(stk *stack, x, y Word) nat {
	return z.expNN(stk, nat(nil).setWord(x), nat(nil).setWord(y), nil, false)
}

// construct table of powers of bb*leafSize to use in subdivisions.
func divisors(stk *stack, m int, b Word, ndigits int, bb Word) []divisor {
	// only compute table when recursive conversion is enabled and x is large
	if leafSize == 0 || m <= leafSize {
		return nil
	}

	// determine k where (bb**leafSize)**(2**k) >= sqrt(x)
	k := 1
	for words := leafSize; words < m>>1 && k < len(cacheBase10.table); words <<= 1 {
		k++
	}

	// reuse and extend existing table of divisors or create new table as appropriate
	var table []divisor // for b == 10, table overlaps with cacheBase10.table
	if b == 10 {
		cacheBase10.Lock()
		table = cacheBase10.table[0:k] // reuse old table for this conversion
	} else {
		table = make([]divisor, k) // create new table for this conversion
	}

	// extend table
	if table[k-1].ndigits == 0 {
		// add new entries as needed
		var larger nat
		for i := 0; i < k; i++ {
			if table[i].ndigits == 0 {
				if i == 0 {
					table[0].bbb = nat(nil).expWW(stk, bb, Word(leafSize))
					table[0].ndigits = ndigits * leafSize
				} else {
					table[i].bbb = nat(nil).sqr(stk, table[i-1].bbb)
					table[i].ndigits = 2 * table[i-1].ndigits
				}

				// optimization: exploit aggregated extra bits in macro blocks
				larger = nat(nil).set(table[i].bbb)
				for mulAddVWW(larger, larger, b, 0) == 0 {
					table[i].bbb = table[i].bbb.set(larger)
					table[i].ndigits++
				}

				table[i].nbits = table[i].bbb.bitLen()
			}
		}
	}

	if b == 10 {
		cacheBase10.Unlock()
	}

	return table
}

```

// === FILE: references!/go/src/math/big/natdiv.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*

Multi-precision division. Here be dragons.

Given u and v, where u is n+m digits, and v is n digits (with no leading zeros),
the goal is to return quo, rem such that u = quo*v + rem, where 0 ≤ rem < v.
That is, quo = ⌊u/v⌋ where ⌊x⌋ denotes the floor (truncation to integer) of x,
and rem = u - quo·v.


Long Division

Division in a computer proceeds the same as long division in elementary school,
but computers are not as good as schoolchildren at following vague directions,
so we have to be much more precise about the actual steps and what can happen.

We work from most to least significant digit of the quotient, doing:

 • Guess a digit q, the number of v to subtract from the current
   section of u to zero out the topmost digit.
 • Check the guess by multiplying q·v and comparing it against
   the current section of u, adjusting the guess as needed.
 • Subtract q·v from the current section of u.
 • Add q to the corresponding section of the result quo.

When all digits have been processed, the final remainder is left in u
and returned as rem.

For example, here is a sketch of dividing 5 digits by 3 digits (n=3, m=2).

	                 q₂ q₁ q₀
	         _________________
	v₂ v₁ v₀ ) u₄ u₃ u₂ u₁ u₀
	           ↓  ↓  ↓  |  |
	          [u₄ u₃ u₂]|  |
	        - [  q₂·v  ]|  |
	        ----------- ↓  |
	          [  rem  | u₁]|
	        - [    q₁·v   ]|
	           ----------- ↓
	             [  rem  | u₀]
	           - [    q₀·v   ]
	              ------------
	                [  rem   ]

Instead of creating new storage for the remainders and copying digits from u
as indicated by the arrows, we use u's storage directly as both the source
and destination of the subtractions, so that the remainders overwrite
successive overlapping sections of u as the division proceeds, using a slice
of u to identify the current section. This avoids all the copying as well as
shifting of remainders.

Division of u with n+m digits by v with n digits (in base B) can in general
produce at most m+1 digits, because:

  • u < B^(n+m)               [B^(n+m) has n+m+1 digits]
  • v ≥ B^(n-1)               [B^(n-1) is the smallest n-digit number]
  • u/v < B^(n+m) / B^(n-1)   [divide bounds for u, v]
  • u/v < B^(m+1)             [simplify]

The first step is special: it takes the top n digits of u and divides them by
the n digits of v, producing the first quotient digit and an n-digit remainder.
In the example, q₂ = ⌊u₄u₃u₂ / v⌋.

The first step divides n digits by n digits to ensure that it produces only a
single digit.

Each subsequent step appends the next digit from u to the remainder and divides
those n+1 digits by the n digits of v, producing another quotient digit and a
new n-digit remainder.

Subsequent steps divide n+1 digits by n digits, an operation that in general
might produce two digits. However, as used in the algorithm, that division is
guaranteed to produce only a single digit. The dividend is of the form
rem·B + d, where rem is a remainder from the previous step and d is a single
digit, so:

 • rem ≤ v - 1                 [rem is a remainder from dividing by v]
 • rem·B ≤ v·B - B             [multiply by B]
 • d ≤ B - 1                   [d is a single digit]
 • rem·B + d ≤ v·B - 1         [add]
 • rem·B + d < v·B             [change ≤ to <]
 • (rem·B + d)/v < B           [divide by v]


Guess and Check

At each step we need to divide n+1 digits by n digits, but this is for the
implementation of division by n digits, so we can't just invoke a division
routine: we _are_ the division routine. Instead, we guess at the answer and
then check it using multiplication. If the guess is wrong, we correct it.

How can this guessing possibly be efficient? It turns out that the following
statement (let's call it the Good Guess Guarantee) is true.

If

 • q = ⌊u/v⌋ where u is n+1 digits and v is n digits,
 • q < B, and
 • the topmost digit of v = vₙ₋₁ ≥ B/2,

then q̂ = ⌊uₙuₙ₋₁ / vₙ₋₁⌋ satisfies q ≤ q̂ ≤ q+2. (Proof below.)

That is, if we know the answer has only a single digit and we guess an answer
by ignoring the bottom n-1 digits of u and v, using a 2-by-1-digit division,
then that guess is at least as large as the correct answer. It is also not
too much larger: it is off by at most two from the correct answer.

Note that in the first step of the overall division, which is an n-by-n-digit
division, the 2-by-1 guess uses an implicit uₙ = 0.

Note that using a 2-by-1-digit division here does not mean calling ourselves
recursively. Instead, we use an efficient direct hardware implementation of
that operation.

Note that because q is u/v rounded down, q·v must not exceed u: u ≥ q·v.
If a guess q̂ is too big, it will not satisfy this test. Viewed a different way,
the remainder r̂ for a given q̂ is u - q̂·v, which must be positive. If it is
negative, then the guess q̂ is too big.

This gives us a way to compute q. First compute q̂ with 2-by-1-digit division.
Then, while u < q̂·v, decrement q̂; this loop executes at most twice, because
q̂ ≤ q+2.


Scaling Inputs

The Good Guess Guarantee requires that the top digit of v (vₙ₋₁) be at least B/2.
For example in base 10, ⌊172/19⌋ = 9, but ⌊18/1⌋ = 18: the guess is wildly off
because the first digit 1 is smaller than B/2 = 5.

We can ensure that v has a large top digit by multiplying both u and v by the
right amount. Continuing the example, if we multiply both 172 and 19 by 3, we
now have ⌊516/57⌋, the leading digit of v is now ≥ 5, and sure enough
⌊51/5⌋ = 10 is much closer to the correct answer 9. It would be easier here
to multiply by 4, because that can be done with a shift. Specifically, we can
always count the number of leading zeros i in the first digit of v and then
shift both u and v left by i bits.

Having scaled u and v, the value ⌊u/v⌋ is unchanged, but the remainder will
be scaled: 172 mod 19 is 1, but 516 mod 57 is 3. We have to divide the remainder
by the scaling factor (shifting right i bits) when we finish.

Note that these shifts happen before and after the entire division algorithm,
not at each step in the per-digit iteration.

Note the effect of scaling inputs on the size of the possible quotient.
In the scaled u/v, u can gain a digit from scaling; v never does, because we
pick the scaling factor to make v's top digit larger but without overflowing.
If u and v have n+m and n digits after scaling, then:

  • u < B^(n+m)               [B^(n+m) has n+m+1 digits]
  • v ≥ B^n / 2               [vₙ₋₁ ≥ B/2, so vₙ₋₁·B^(n-1) ≥ B^n/2]
  • u/v < B^(n+m) / (B^n / 2) [divide bounds for u, v]
  • u/v < 2 B^m               [simplify]

The quotient can still have m+1 significant digits, but if so the top digit
must be a 1. This provides a different way to handle the first digit of the
result: compare the top n digits of u against v and fill in either a 0 or a 1.


Refining Guesses

Before we check whether u < q̂·v, we can adjust our guess to change it from
q̂ = ⌊uₙuₙ₋₁ / vₙ₋₁⌋ into the refined guess ⌊uₙuₙ₋₁uₙ₋₂ / vₙ₋₁vₙ₋₂⌋.
Although not mentioned above, the Good Guess Guarantee also promises that this
3-by-2-digit division guess is more precise and at most one away from the real
answer q. The improvement from the 2-by-1 to the 3-by-2 guess can also be done
without n-digit math.

If we have a guess q̂ = ⌊uₙuₙ₋₁ / vₙ₋₁⌋ and we want to see if it also equal to
⌊uₙuₙ₋₁uₙ₋₂ / vₙ₋₁vₙ₋₂⌋, we can use the same check we would for the full division:
if uₙuₙ₋₁uₙ₋₂ < q̂·vₙ₋₁vₙ₋₂, then the guess is too large and should be reduced.

Checking uₙuₙ₋₁uₙ₋₂ < q̂·vₙ₋₁vₙ₋₂ is the same as uₙuₙ₋₁uₙ₋₂ - q̂·vₙ₋₁vₙ₋₂ < 0,
and

	uₙuₙ₋₁uₙ₋₂ - q̂·vₙ₋₁vₙ₋₂ = (uₙuₙ₋₁·B + uₙ₋₂) - q̂·(vₙ₋₁·B + vₙ₋₂)
	                          [splitting off the bottom digit]
	                      = (uₙuₙ₋₁ - q̂·vₙ₋₁)·B + uₙ₋₂ - q̂·vₙ₋₂
	                          [regrouping]

The expression (uₙuₙ₋₁ - q̂·vₙ₋₁) is the remainder of uₙuₙ₋₁ / vₙ₋₁.
If the initial guess returns both q̂ and its remainder r̂, then checking
whether uₙuₙ₋₁uₙ₋₂ < q̂·vₙ₋₁vₙ₋₂ is the same as checking r̂·B + uₙ₋₂ < q̂·vₙ₋₂.

If we find that r̂·B + uₙ₋₂ < q̂·vₙ₋₂, then we can adjust the guess by
decrementing q̂ and adding vₙ₋₁ to r̂. We repeat until r̂·B + uₙ₋₂ ≥ q̂·vₙ₋₂.
(As before, this fixup is only needed at most twice.)

Now that q̂ = ⌊uₙuₙ₋₁uₙ₋₂ / vₙ₋₁vₙ₋₂⌋, as mentioned above it is at most one
away from the correct q, and we've avoided doing any n-digit math.
(If we need the new remainder, it can be computed as r̂·B + uₙ₋₂ - q̂·vₙ₋₂.)

The final check u < q̂·v and the possible fixup must be done at full precision.
For random inputs, a fixup at this step is exceedingly rare: the 3-by-2 guess
is not often wrong at all. But still we must do the check. Note that since the
3-by-2 guess is off by at most 1, it can be convenient to perform the final
u < q̂·v as part of the computation of the remainder r = u - q̂·v. If the
subtraction underflows, decremeting q̂ and adding one v back to r is enough to
arrive at the final q, r.

That's the entirety of long division: scale the inputs, and then loop over
each output position, guessing, checking, and correcting the next output digit.

For a 2n-digit number divided by an n-digit number (the worst size-n case for
division complexity), this algorithm uses n+1 iterations, each of which must do
at least the 1-by-n-digit multiplication q̂·v. That's O(n) iterations of
O(n) time each, so O(n²) time overall.


Recursive Division

For very large inputs, it is possible to improve on the O(n²) algorithm.
Let's call a group of n/2 real digits a (very) “wide digit”. We can run the
standard long division algorithm explained above over the wide digits instead of
the actual digits. This will result in many fewer steps, but the math involved in
each step is more work.

Where basic long division uses a 2-by-1-digit division to guess the initial q̂,
the new algorithm must use a 2-by-1-wide-digit division, which is of course
really an n-by-n/2-digit division. That's OK: if we implement n-digit division
in terms of n/2-digit division, the recursion will terminate when the divisor
becomes small enough to handle with standard long division or even with the
2-by-1 hardware instruction.

For example, here is a sketch of dividing 10 digits by 4, proceeding with
wide digits corresponding to two regular digits. The first step, still special,
must leave off a (regular) digit, dividing 5 by 4 and producing a 4-digit
remainder less than v. The middle steps divide 6 digits by 4, guaranteed to
produce two output digits each (one wide digit) with 4-digit remainders.
The final step must use what it has: the 4-digit remainder plus one more,
5 digits to divide by 4.

	                       q₆ q₅ q₄ q₃ q₂ q₁ q₀
	            _______________________________
	v₃ v₂ v₁ v₀ ) u₉ u₈ u₇ u₆ u₅ u₄ u₃ u₂ u₁ u₀
	              ↓  ↓  ↓  ↓  ↓  |  |  |  |  |
	             [u₉ u₈ u₇ u₆ u₅]|  |  |  |  |
	           - [    q₆q₅·v    ]|  |  |  |  |
	           ----------------- ↓  ↓  |  |  |
	                [    rem    |u₄ u₃]|  |  |
	              - [     q₄q₃·v      ]|  |  |
	              -------------------- ↓  ↓  |
	                      [    rem    |u₂ u₁]|
	                    - [     q₂q₁·v      ]|
	                    -------------------- ↓
	                            [    rem    |u₀]
	                          - [     q₀·v     ]
	                          ------------------
	                               [    rem    ]

An alternative would be to look ahead to how well n/2 divides into n+m and
adjust the first step to use fewer digits as needed, making the first step
more special to make the last step not special at all. For example, using the
same input, we could choose to use only 4 digits in the first step, leaving
a full wide digit for the last step:

	                       q₆ q₅ q₄ q₃ q₂ q₁ q₀
	            _______________________________
	v₃ v₂ v₁ v₀ ) u₉ u₈ u₇ u₆ u₅ u₄ u₃ u₂ u₁ u₀
	              ↓  ↓  ↓  ↓  |  |  |  |  |  |
	             [u₉ u₈ u₇ u₆]|  |  |  |  |  |
	           - [    q₆·v   ]|  |  |  |  |  |
	           -------------- ↓  ↓  |  |  |  |
	             [    rem    |u₅ u₄]|  |  |  |
	           - [     q₅q₄·v      ]|  |  |  |
	           -------------------- ↓  ↓  |  |
	                   [    rem    |u₃ u₂]|  |
	                 - [     q₃q₂·v      ]|  |
	                 -------------------- ↓  ↓
	                         [    rem    |u₁ u₀]
	                       - [     q₁q₀·v      ]
	                       ---------------------
	                               [    rem    ]

Today, the code in divRecursiveStep works like the first example. Perhaps in
the future we will make it work like the alternative, to avoid a special case
in the final iteration.

Either way, each step is a 3-by-2-wide-digit division approximated first by
a 2-by-1-wide-digit division, just as we did for regular digits in long division.
Because the actual answer we want is a 3-by-2-wide-digit division, instead of
multiplying q̂·v directly during the fixup, we can use the quick refinement
from long division (an n/2-by-n/2 multiply) to correct q to its actual value
and also compute the remainder (as mentioned above), and then stop after that,
never doing a full n-by-n multiply.

Instead of using an n-by-n/2-digit division to produce n/2 digits, we can add
(not discard) one more real digit, doing an (n+1)-by-(n/2+1)-digit division that
produces n/2+1 digits. That single extra digit tightens the Good Guess Guarantee
to q ≤ q̂ ≤ q+1 and lets us drop long division's special treatment of the first
digit. These benefits are discussed more after the Good Guess Guarantee proof
below.


How Fast is Recursive Division?

For a 2n-by-n-digit division, this algorithm runs a 4-by-2 long division over
wide digits, producing two wide digits plus a possible leading regular digit 1,
which can be handled without a recursive call. That is, the algorithm uses two
full iterations, each using an n-by-n/2-digit division and an n/2-by-n/2-digit
multiplication, along with a few n-digit additions and subtractions. The standard
n-by-n-digit multiplication algorithm requires O(n²) time, making the overall
algorithm require time T(n) where

	T(n) = 2T(n/2) + O(n) + O(n²)

which, by the Bentley-Haken-Saxe theorem, ends up reducing to T(n) = O(n²).
This is not an improvement over regular long division.

When the number of digits n becomes large enough, Karatsuba's algorithm for
multiplication can be used instead, which takes O(n^log₂3) = O(n^1.6) time.
(Karatsuba multiplication is implemented in func karatsuba in nat.go.)
That makes the overall recursive division algorithm take O(n^1.6) time as well,
which is an improvement, but again only for large enough numbers.

It is not critical to make sure that every recursion does only two recursive
calls. While in general the number of recursive calls can change the time
analysis, in this case doing three calls does not change the analysis:

	T(n) = 3T(n/2) + O(n) + O(n^log₂3)

ends up being T(n) = O(n^log₂3). Because the Karatsuba multiplication taking
time O(n^log₂3) is itself doing 3 half-sized recursions, doing three for the
division does not hurt the asymptotic performance. Of course, it is likely
still faster in practice to do two.


Proof of the Good Guess Guarantee

Given numbers x, y, let us break them into the quotients and remainders when
divided by some scaling factor S, with the added constraints that the quotient
x/y and the high part of y are both less than some limit T, and that the high
part of y is at least half as big as T.

	x₁ = ⌊x/S⌋        y₁ = ⌊y/S⌋
	x₀ = x mod S      y₀ = y mod S

	x  = x₁·S + x₀    0 ≤ x₀ < S    x/y < T
	y  = y₁·S + y₀    0 ≤ y₀ < S    T/2 ≤ y₁ < T

And consider the two truncated quotients:

	q = ⌊x/y⌋
	q̂ = ⌊x₁/y₁⌋

We will prove that q ≤ q̂ ≤ q+2.

The guarantee makes no real demands on the scaling factor S: it is simply the
magnitude of the digits cut from both x and y to produce x₁ and y₁.
The guarantee makes only limited demands on T: it must be large enough to hold
the quotient x/y, and y₁ must have roughly the same size.

To apply to the earlier discussion of 2-by-1 guesses in long division,
we would choose:

	S  = Bⁿ⁻¹
	T  = B
	x  = u
	x₁ = uₙuₙ₋₁
	x₀ = uₙ₋₂...u₀
	y  = v
	y₁ = vₙ₋₁
	y₀ = vₙ₋₂...u₀

These simpler variables avoid repeating those longer expressions in the proof.

Note also that, by definition, truncating division ⌊x/y⌋ satisfies

	x/y - 1 < ⌊x/y⌋ ≤ x/y.

This fact will be used a few times in the proofs.

Proof that q ≤ q̂:

	q̂·y₁ = ⌊x₁/y₁⌋·y₁                      [by definition, q̂ = ⌊x₁/y₁⌋]
	     > (x₁/y₁ - 1)·y₁                  [x₁/y₁ - 1 < ⌊x₁/y₁⌋]
	     = x₁ - y₁                         [distribute y₁]

	So q̂·y₁ > x₁ - y₁.
	Since q̂·y₁ is an integer, q̂·y₁ ≥ x₁ - y₁ + 1.

	q̂ - q = q̂ - ⌊x/y⌋                      [by definition, q = ⌊x/y⌋]
	      ≥ q̂ - x/y                        [⌊x/y⌋ < x/y]
	      = (1/y)·(q̂·y - x)                [factor out 1/y]
	      ≥ (1/y)·(q̂·y₁·S - x)             [y = y₁·S + y₀ ≥ y₁·S]
	      ≥ (1/y)·((x₁ - y₁ + 1)·S - x)    [above: q̂·y₁ ≥ x₁ - y₁ + 1]
	      = (1/y)·(x₁·S - y₁·S + S - x)    [distribute S]
	      = (1/y)·(S - x₀ - y₁·S)          [-x = -x₁·S - x₀]
	      > -y₁·S / y                      [x₀ < S, so S - x₀ > 0; drop it]
	      ≥ -1                             [y₁·S ≤ y]

	So q̂ - q > -1.
	Since q̂ - q is an integer, q̂ - q ≥ 0, or equivalently q ≤ q̂.

Proof that q̂ ≤ q+2:

	x₁/y₁ - x/y = x₁·S/y₁·S - x/y          [multiply left term by S/S]
	            ≤ x/y₁·S - x/y             [x₁S ≤ x]
	            = (x/y)·(y/y₁·S - 1)       [factor out x/y]
	            = (x/y)·((y - y₁·S)/y₁·S)  [move -1 into y/y₁·S fraction]
	            = (x/y)·(y₀/y₁·S)          [y - y₁·S = y₀]
	            = (x/y)·(1/y₁)·(y₀/S)      [factor out 1/y₁]
	            < (x/y)·(1/y₁)             [y₀ < S, so y₀/S < 1]
	            ≤ (x/y)·(2/T)              [y₁ ≥ T/2, so 1/y₁ ≤ 2/T]
	            < T·(2/T)                  [x/y < T]
	            = 2                        [T·(2/T) = 2]

	So x₁/y₁ - x/y < 2.

	q̂ - q = ⌊x₁/y₁⌋ - q                    [by definition, q̂ = ⌊x₁/y₁⌋]
	      = ⌊x₁/y₁⌋ - ⌊x/y⌋                [by definition, q = ⌊x/y⌋]
	      ≤ x₁/y₁ - ⌊x/y⌋                  [⌊x₁/y₁⌋ ≤ x₁/y₁]
	      < x₁/y₁ - (x/y - 1)              [⌊x/y⌋ > x/y - 1]
	      = (x₁/y₁ - x/y) + 1              [regrouping]
	      < 2 + 1                          [above: x₁/y₁ - x/y < 2]
	      = 3

	So q̂ - q < 3.
	Since q̂ - q is an integer, q̂ - q ≤ 2.

Note that when x/y < T/2, the bounds tighten to x₁/y₁ - x/y < 1 and therefore
q̂ - q ≤ 1.

Note also that in the general case 2n-by-n division where we don't know that
x/y < T, we do know that x/y < 2T, yielding the bound q̂ - q ≤ 4. So we could
remove the special case first step of long division as long as we allow the
first fixup loop to run up to four times. (Using a simple comparison to decide
whether the first digit is 0 or 1 is still more efficient, though.)

Finally, note that when dividing three leading base-B digits by two (scaled),
we have T = B² and x/y < B = T/B, a much tighter bound than x/y < T.
This in turn yields the much tighter bound x₁/y₁ - x/y < 2/B. This means that
⌊x₁/y₁⌋ and ⌊x/y⌋ can only differ when x/y is less than 2/B greater than an
integer. For random x and y, the chance of this is 2/B, or, for large B,
approximately zero. This means that after we produce the 3-by-2 guess in the
long division algorithm, the fixup loop essentially never runs.

In the recursive algorithm, the extra digit in (2·⌊n/2⌋+1)-by-(⌊n/2⌋+1)-digit
division has exactly the same effect: the probability of needing a fixup is the
same 2/B. Even better, we can allow the general case x/y < 2T and the fixup
probability only grows to 4/B, still essentially zero.


References

There are no great references for implementing long division; thus this comment.
Here are some notes about what to expect from the obvious references.

Knuth Volume 2 (Seminumerical Algorithms) section 4.3.1 is the usual canonical
reference for long division, but that entire series is highly compressed, never
repeating a necessary fact and leaving important insights to the exercises.
For example, no rationale whatsoever is given for the calculation that extends
q̂ from a 2-by-1 to a 3-by-2 guess, nor why it reduces the error bound.
The proof that the calculation even has the desired effect is left to exercises.
The solutions to those exercises provided at the back of the book are entirely
calculations, still with no explanation as to what is going on or how you would
arrive at the idea of doing those exact calculations. Nowhere is it mentioned
that this test extends the 2-by-1 guess into a 3-by-2 guess. The proof of the
Good Guess Guarantee is only for the 2-by-1 guess and argues by contradiction,
making it difficult to understand how modifications like adding another digit
or adjusting the quotient range affects the overall bound.

All that said, Knuth remains the canonical reference. It is dense but packed
full of information and references, and the proofs are simpler than many other
presentations. The proofs above are reworkings of Knuth's to remove the
arguments by contradiction and add explanations or steps that Knuth omitted.
But beware of errors in older printings. Take the published errata with you.

Brinch Hansen's “Multiple-length Division Revisited: a Tour of the Minefield”
starts with a blunt critique of Knuth's presentation (among others) and then
presents a more detailed and easier to follow treatment of long division,
including an implementation in Pascal. But the algorithm and implementation
work entirely in terms of 3-by-2 division, which is much less useful on modern
hardware than an algorithm using 2-by-1 division. The proofs are a bit too
focused on digit counting and seem needlessly complex, especially compared to
the ones given above.

Burnikel and Ziegler's “Fast Recursive Division” introduced the key insight of
implementing division by an n-digit divisor using recursive calls to division
by an n/2-digit divisor, relying on Karatsuba multiplication to yield a
sub-quadratic run time. However, the presentation decisions are made almost
entirely for the purpose of simplifying the run-time analysis, rather than
simplifying the presentation. Instead of a single algorithm that loops over
quotient digits, the paper presents two mutually-recursive algorithms, for
2n-by-n and 3n-by-2n. The paper also does not present any general (n+m)-by-n
algorithm.

The proofs in the paper are remarkably complex, especially considering that
the algorithm is at its core just long division on wide digits, so that the
usual long division proofs apply essentially unaltered.
*/

package big

import "math/bits"

// rem returns r such that r = u%v.
// It uses z as the storage for r.
func (z nat) rem(stk *stack, u, v nat) (r nat) {
	if alias(z, u) {
		z = nil
	}
	defer stk.restore(stk.save())
	q := stk.nat(max(1, len(u)-(len(v)-1)))
	_, r = q.div(stk, z, u, v)
	return r
}

// div returns q, r such that q = ⌊u/v⌋ and r = u%v = u - q·v.
// It uses z and z2 as the storage for q and r.
// The caller may pass stk == nil to request that div obtain and release one itself.
func (z nat) div(stk *stack, z2, u, v nat) (q, r nat) {
	if len(v) == 0 {
		panic("division by zero")
	}

	if len(v) == 1 {
		// Short division: long optimized for a single-word divisor.
		// In that case, the 2-by-1 guess is all we need at each step.
		var r2 Word
		q, r2 = z.divW(u, v[0])
		r = z2.setWord(r2)
		return
	}

	if u.cmp(v) < 0 {
		q = z[:0]
		r = z2.set(u)
		return
	}

	if stk == nil {
		stk = getStack()
		defer stk.free()
	}

	q, r = z.divLarge(stk, z2, u, v)
	return
}

// divW returns q, r such that q = ⌊x/y⌋ and r = x%y = x - q·y.
// It uses z as the storage for q.
// Note that y is a single digit (Word), not a big number.
func (z nat) divW(x nat, y Word) (q nat, r Word) {
	m := len(x)
	switch {
	case y == 0:
		panic("division by zero")
	case y == 1:
		q = z.set(x) // result is x
		return
	case m == 0:
		q = z[:0] // result is 0
		return
	}
	// m > 0
	z = z.make(m)
	r = divWVW(z, 0, x, y)
	q = z.norm()
	return
}

// modW returns x % d.
func (x nat) modW(d Word) (r Word) {
	// TODO(agl): we don't actually need to store the q value.
	var q nat
	q = q.make(len(x))
	return divWVW(q, 0, x, d)
}

// divWVW overwrites z with ⌊x/y⌋, returning the remainder r.
// The caller must ensure that len(z) = len(x).
func divWVW(z []Word, xn Word, x []Word, y Word) (r Word) {
	r = xn
	if len(x) == 1 {
		qq, rr := bits.Div(uint(r), uint(x[0]), uint(y))
		z[0] = Word(qq)
		return Word(rr)
	}
	rec := reciprocalWord(y)
	for i := len(z) - 1; i >= 0; i-- {
		z[i], r = divWW(r, x[i], y, rec)
	}
	return r
}

// div returns q, r such that q = ⌊uIn/vIn⌋ and r = uIn%vIn = uIn - q·vIn.
// It uses z and u as the storage for q and r.
// The caller must ensure that len(vIn) ≥ 2 (use divW otherwise)
// and that len(uIn) ≥ len(vIn) (the answer is 0, uIn otherwise).
func (z nat) divLarge(stk *stack, u, uIn, vIn nat) (q, r nat) {
	n := len(vIn)
	m := len(uIn) - n

	// Scale the inputs so vIn's top bit is 1 (see “Scaling Inputs” above).
	// vIn is treated as a read-only input (it may be in use by another
	// goroutine), so we must make a copy.
	// uIn is copied to u.
	defer stk.restore(stk.save())
	shift := nlz(vIn[n-1])
	v := stk.nat(n)
	u = u.make(len(uIn) + 1)
	if shift == 0 {
		copy(v, vIn)
		copy(u[:len(uIn)], uIn)
		u[len(uIn)] = 0
	} else {
		lshVU(v, vIn, shift)
		u[len(uIn)] = lshVU(u[:len(uIn)], uIn, shift)
	}

	// The caller should not pass aliased z and u, since those are
	// the two different outputs, but correct just in case.
	if alias(z, u) {
		z = nil
	}
	q = z.make(m + 1)

	// Use basic or recursive long division depending on size.
	if n < divRecursiveThreshold {
		q.divBasic(stk, u, v)
	} else {
		q.divRecursive(stk, u, v)
	}

	q = q.norm()

	// Undo scaling of remainder.
	if shift != 0 {
		rshVU(u, u, shift)
	}
	r = u.norm()

	return q, r
}

// divBasic implements long division as described above.
// It overwrites q with ⌊u/v⌋ and overwrites u with the remainder r.
// q must be large enough to hold ⌊u/v⌋.
func (q nat) divBasic(stk *stack, u, v nat) {
	n := len(v)
	m := len(u) - n

	defer stk.restore(stk.save())
	qhatv := stk.nat(n + 1)

	// Set up for divWW below, precomputing reciprocal argument.
	vn1 := v[n-1]
	rec := reciprocalWord(vn1)

	// Invent a leading 0 for u, for the first iteration.
	// Invariant: ujn == u[j+n] in each iteration.
	ujn := Word(0)

	// Compute each digit of quotient.
	for j := m; j >= 0; j-- {
		// Compute the 2-by-1 guess q̂.
		qhat := Word(_M)

		// ujn ≤ vn1, or else q̂ would be more than one digit.
		// For ujn == vn1, we set q̂ to the max digit M above.
		// Otherwise, we compute the 2-by-1 guess.
		if ujn != vn1 {
			var rhat Word
			qhat, rhat = divWW(ujn, u[j+n-1], vn1, rec)

			// Refine q̂ to a 3-by-2 guess. See “Refining Guesses” above.
			vn2 := v[n-2]
			x1, x2 := mulWW(qhat, vn2)
			ujn2 := u[j+n-2]
			for greaterThan(x1, x2, rhat, ujn2) { // x1x2 > r̂ u[j+n-2]
				qhat--
				rhat += vn1
				// If r̂  overflows, then
				// r̂ u[j+n-2]v[n-1] is now definitely > x1 x2.
				if rhat < vn1 {
					break
				}

				// Maintain (x1, x2) = qhat * vn2.
				// Since we did qhat-- we need to do (x1, x2) -= vn2.
				if vn2 > x2 {
					x1--
				}
				x2 -= vn2
			}
		}

		// Compute q̂·v.
		qhatv[n] = mulAddVWW(qhatv[0:n], v, qhat, 0)
		qhl := len(qhatv)
		if j+qhl > len(u) && qhatv[n] == 0 {
			qhl--
		}

		// Subtract q̂·v from the current section of u.
		// If it underflows, q̂·v > u, which we fix up
		// by decrementing q̂ and adding v back.
		c := subVV(u[j:j+qhl], u[j:j+qhl], qhatv[:qhl])
		if c != 0 {
			c := addVV(u[j:j+n], u[j:j+n], v)
			// If n == qhl, the carry from subVV and the carry from addVV
			// cancel out and don't affect u[j+n].
			if n < qhl {
				u[j+n] += c
			}
			qhat--
		}

		ujn = u[j+n-1]

		// Save quotient digit.
		// Caller may know the top digit is zero and not leave room for it.
		if j == m && m == len(q) && qhat == 0 {
			continue
		}
		q[j] = qhat
	}
}

// greaterThan reports whether the two digit numbers x1 x2 > y1 y2.
// TODO(rsc): In contradiction to most of this file, x1 is the high
// digit and x2 is the low digit. This should be fixed.
func greaterThan(x1, x2, y1, y2 Word) bool {
	return x1 > y1 || x1 == y1 && x2 > y2
}

// divRecursiveThreshold is the number of divisor digits
// at which point divRecursive is faster than divBasic.
var divRecursiveThreshold = 40 // see calibrate_test.go

// divRecursive implements recursive division as described above.
// It overwrites z with ⌊u/v⌋ and overwrites u with the remainder r.
// z must be large enough to hold ⌊u/v⌋.
// This function is just for allocating and freeing temporaries
// around divRecursiveStep, the real implementation.
func (z nat) divRecursive(stk *stack, u, v nat) {
	clear(z)
	z.divRecursiveStep(stk, u, v, 0)
}

// divRecursiveStep is the actual implementation of recursive division.
// It adds ⌊u/v⌋ to z and overwrites u with the remainder r.
// z must be large enough to hold ⌊u/v⌋.
// It uses temps[depth] (allocating if needed) as a temporary live across
// the recursive call. It also uses tmp, but not live across the recursion.
func (z nat) divRecursiveStep(stk *stack, u, v nat, depth int) {
	// u is a subsection of the original and may have leading zeros.
	// TODO(rsc): The v = v.norm() is useless and should be removed.
	// We know (and require) that v's top digit is ≥ B/2.
	u = u.norm()
	v = v.norm()
	if len(u) == 0 {
		clear(z)
		return
	}

	// Fall back to basic division if the problem is now small enough.
	n := len(v)
	if n < divRecursiveThreshold {
		z.divBasic(stk, u, v)
		return
	}

	// Nothing to do if u is shorter than v (implies u < v).
	m := len(u) - n
	if m < 0 {
		return
	}

	// We consider B digits in a row as a single wide digit.
	// (See “Recursive Division” above.)
	//
	// TODO(rsc): rename B to Wide, to avoid confusion with _B,
	// which is something entirely different.
	// TODO(rsc): Look into whether using ⌈n/2⌉ is better than ⌊n/2⌋.
	B := n / 2

	// Allocate a nat for qhat below.
	defer stk.restore(stk.save())
	qhat0 := stk.nat(B + 1)

	// Compute each wide digit of the quotient.
	//
	// TODO(rsc): Change the loop to be
	//	for j := (m+B-1)/B*B; j > 0; j -= B {
	// which will make the final step a regular step, letting us
	// delete what amounts to an extra copy of the loop body below.
	j := m
	for j > B {
		// Divide u[j-B:j+n] (3 wide digits) by v (2 wide digits).
		// First make the 2-by-1-wide-digit guess using a recursive call.
		// Then extend the guess to the full 3-by-2 (see “Refining Guesses”).
		//
		// For the 2-by-1-wide-digit guess, instead of doing 2B-by-B-digit,
		// we use a (2B+1)-by-(B+1) digit, which handles the possibility that
		// the result has an extra leading 1 digit as well as guaranteeing
		// that the computed q̂ will be off by at most 1 instead of 2.

		// s is the number of digits to drop from the 3B- and 2B-digit chunks.
		// We drop B-1 to be left with 2B+1 and B+1.
		s := (B - 1)

		// uu is the up-to-3B-digit section of u we are working on.
		uu := u[j-B:]

		// Compute the 2-by-1 guess q̂, leaving r̂ in uu[s:B+n].
		qhat := qhat0
		clear(qhat)
		qhat.divRecursiveStep(stk, uu[s:B+n], v[s:], depth+1)
		qhat = qhat.norm()

		// Extend to a 3-by-2 quotient and remainder.
		// Because divRecursiveStep overwrote the top part of uu with
		// the remainder r̂, the full uu already contains the equivalent
		// of r̂·B + uₙ₋₂ from the “Refining Guesses” discussion.
		// Subtracting q̂·vₙ₋₂ from it will compute the full-length remainder.
		// If that subtraction underflows, q̂·v > u, which we fix up
		// by decrementing q̂ and adding v back, same as in long division.

		// TODO(rsc): Instead of subtract and fix-up, this code is computing
		// q̂·vₙ₋₂ and decrementing q̂ until that product is ≤ u.
		// But we can do the subtraction directly, as in the comment above
		// and in long division, because we know that q̂ is wrong by at most one.
		mark := stk.save()
		qhatv := stk.nat(3 * n)
		clear(qhatv)
		qhatv = qhatv.mul(stk, qhat, v[:s])
		for i := 0; i < 2; i++ {
			e := qhatv.cmp(uu.norm())
			if e <= 0 {
				break
			}
			subVW(qhat, qhat, 1)
			c := subVV(qhatv[:s], qhatv[:s], v[:s])
			if len(qhatv) > s {
				subVW(qhatv[s:], qhatv[s:], c)
			}
			addTo(uu[s:], v[s:])
		}
		if qhatv.cmp(uu.norm()) > 0 {
			panic("impossible")
		}
		c := subVV(uu[:len(qhatv)], uu[:len(qhatv)], qhatv)
		if c > 0 {
			subVW(uu[len(qhatv):], uu[len(qhatv):], c)
		}
		addTo(z[j-B:], qhat)
		j -= B
		stk.restore(mark)
	}

	// TODO(rsc): Rewrite loop as described above and delete all this code.

	// Now u < (v<<B), compute lower bits in the same way.
	// Choose shift = B-1 again.
	s := B - 1
	qhat := qhat0
	clear(qhat)
	qhat.divRecursiveStep(stk, u[s:].norm(), v[s:], depth+1)
	qhat = qhat.norm()
	qhatv := stk.nat(3 * n)
	clear(qhatv)
	qhatv = qhatv.mul(stk, qhat, v[:s])
	// Set the correct remainder as before.
	for i := 0; i < 2; i++ {
		if e := qhatv.cmp(u.norm()); e > 0 {
			subVW(qhat, qhat, 1)
			c := subVV(qhatv[:s], qhatv[:s], v[:s])
			if len(qhatv) > s {
				subVW(qhatv[s:], qhatv[s:], c)
			}
			addTo(u[s:], v[s:])
		}
	}
	if qhatv.cmp(u.norm()) > 0 {
		panic("impossible")
	}
	c := subVV(u[:len(qhatv)], u[:len(qhatv)], qhatv)
	if c > 0 {
		c = subVW(u[len(qhatv):], u[len(qhatv):], c)
	}
	if c > 0 {
		panic("impossible")
	}

	// Done!
	addTo(z, qhat.norm())
}

```

// === FILE: references!/go/src/math/big/natmul.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Multiplication.

package big

// Operands that are shorter than karatsubaThreshold are multiplied using
// "grade school" multiplication; for longer operands the Karatsuba algorithm
// is used.
var karatsubaThreshold = 40 // see calibrate_test.go

// mul sets z = x*y, using stk for temporary storage.
// The caller may pass stk == nil to request that mul obtain and release one itself.
func (z nat) mul(stk *stack, x, y nat) nat {
	m := len(x)
	n := len(y)

	switch {
	case m < n:
		return z.mul(stk, y, x)
	case m == 0 || n == 0:
		return z[:0]
	case n == 1:
		return z.mulAddWW(x, y[0], 0)
	}
	// m >= n > 1

	// determine if z can be reused
	if alias(z, x) || alias(z, y) {
		z = nil // z is an alias for x or y - cannot reuse
	}
	z = z.make(m + n)

	// use basic multiplication if the numbers are small
	if n < karatsubaThreshold {
		basicMul(z, x, y)
		return z.norm()
	}

	if stk == nil {
		stk = getStack()
		defer stk.free()
	}

	// Let x = x1:x0 where x0 is the same length as y.
	// Compute z = x0*y and then add in x1*y in sections
	// if needed.
	karatsuba(stk, z[:2*n], x[:n], y)

	if n < m {
		clear(z[2*n:])
		defer stk.restore(stk.save())
		t := stk.nat(2 * n)
		for i := n; i < m; i += n {
			t = t.mul(stk, x[i:min(i+n, len(x))], y)
			addTo(z[i:], t)
		}
	}

	return z.norm()
}

// Operands that are shorter than basicSqrThreshold are squared using
// "grade school" multiplication; for operands longer than karatsubaSqrThreshold
// we use the Karatsuba algorithm optimized for x == y.
var basicSqrThreshold = 12     // see calibrate_test.go
var karatsubaSqrThreshold = 80 // see calibrate_test.go

// sqr sets z = x*x, using stk for temporary storage.
// The caller may pass stk == nil to request that sqr obtain and release one itself.
func (z nat) sqr(stk *stack, x nat) nat {
	n := len(x)
	switch {
	case n == 0:
		return z[:0]
	case n == 1:
		d := x[0]
		z = z.make(2)
		z[1], z[0] = mulWW(d, d)
		return z.norm()
	}

	if alias(z, x) {
		z = nil // z is an alias for x - cannot reuse
	}
	z = z.make(2 * n)

	if n < basicSqrThreshold && n < karatsubaSqrThreshold {
		basicMul(z, x, x)
		return z.norm()
	}

	if stk == nil {
		stk = getStack()
		defer stk.free()
	}

	if n < karatsubaSqrThreshold {
		basicSqr(stk, z, x)
		return z.norm()
	}

	karatsubaSqr(stk, z, x)
	return z.norm()
}

// basicSqr sets z = x*x and is asymptotically faster than basicMul
// by about a factor of 2, but slower for small arguments due to overhead.
// Requirements: len(x) > 0, len(z) == 2*len(x)
// The (non-normalized) result is placed in z.
func basicSqr(stk *stack, z, x nat) {
	n := len(x)
	if n < basicSqrThreshold {
		basicMul(z, x, x)
		return
	}

	defer stk.restore(stk.save())
	t := stk.nat(2 * n)
	clear(t)
	z[1], z[0] = mulWW(x[0], x[0]) // the initial square
	for i := 1; i < n; i++ {
		d := x[i]
		// z collects the squares x[i] * x[i]
		z[2*i+1], z[2*i] = mulWW(d, d)
		// t collects the products x[i] * x[j] where j < i
		t[2*i] = addMulVVWW(t[i:2*i], t[i:2*i], x[0:i], d, 0)
	}
	t[2*n-1] = lshVU(t[1:2*n-1], t[1:2*n-1], 1) // double the j < i products
	addVV(z, z, t)                              // combine the result
}

// mulAddWW returns z = x*y + r.
func (z nat) mulAddWW(x nat, y, r Word) nat {
	m := len(x)
	if m == 0 || y == 0 {
		return z.setWord(r) // result is r
	}
	// m > 0

	z = z.make(m + 1)
	z[m] = mulAddVWW(z[0:m], x, y, r)

	return z.norm()
}

// basicMul multiplies x and y and leaves the result in z.
// The (non-normalized) result is placed in z[0 : len(x) + len(y)].
func basicMul(z, x, y nat) {
	clear(z[0 : len(x)+len(y)]) // initialize z
	for i, d := range y {
		if d != 0 {
			z[len(x)+i] = addMulVVWW(z[i:i+len(x)], z[i:i+len(x)], x, d, 0)
		}
	}
}

// karatsuba multiplies x and y,
// writing the (non-normalized) result to z.
// x and y must have the same length n,
// and z must have length twice that.
func karatsuba(stk *stack, z, x, y nat) {
	n := len(y)
	if len(x) != n || len(z) != 2*n {
		panic("bad karatsuba length")
	}

	// Fall back to basic algorithm if small enough.
	if n < karatsubaThreshold || n < 2 {
		basicMul(z, x, y)
		return
	}

	// Let the notation x1:x0 denote the nat (x1<<N)+x0 for some N,
	// and similarly z2:z1:z0 = (z2<<2N)+(z1<<N)+z0.
	//
	// (Note that z0, z1, z2 might be ≥ 2**N, in which case the high
	// bits of, say, z0 are being added to the low bits of z1 in this notation.)
	//
	// Karatsuba multiplication is based on the observation that
	//
	//	x1:x0 * y1:y0 = x1*y1:(x0*y1+y0*x1):x0*y0
	//	              = x1*y1:((x0-x1)*(y1-y0)+x1*y1+x0*y0):x0*y0
	//
	// The second form uses only three half-width multiplications
	// instead of the four that the straightforward first form does.
	//
	// We call the three pieces z0, z1, z2:
	//
	//	z0 = x0*y0
	//	z2 = x1*y1
	//	z1 = (x0-x1)*(y1-y0) + z0 + z2

	n2 := (n + 1) / 2
	x0, x1 := &Int{abs: x[:n2].norm()}, &Int{abs: x[n2:].norm()}
	y0, y1 := &Int{abs: y[:n2].norm()}, &Int{abs: y[n2:].norm()}
	z0 := &Int{abs: z[0 : 2*n2]}
	z2 := &Int{abs: z[2*n2:]}

	// Allocate temporary storage for z1; repurpose z0 to hold tx and ty.
	defer stk.restore(stk.save())
	z1 := &Int{abs: stk.nat(2*n2 + 1)}
	tx := &Int{abs: z[0:n2]}
	ty := &Int{abs: z[n2 : 2*n2]}

	tx.Sub(x0, x1)
	ty.Sub(y1, y0)
	z1.mul(stk, tx, ty)

	clear(z)
	z0.mul(stk, x0, y0)
	z2.mul(stk, x1, y1)
	z1.Add(z1, z0)
	z1.Add(z1, z2)
	addTo(z[n2:], z1.abs)

	// Debug mode: double-check answer and print trace on failure.
	const debug = false
	if debug {
		zz := make(nat, len(z))
		basicMul(zz, x, y)
		if z.cmp(zz) != 0 {
			// All the temps were aliased to z and gone. Recompute.
			z0 = new(Int)
			z0.mul(stk, x0, y0)
			tx = new(Int).Sub(x1, x0)
			ty = new(Int).Sub(y0, y1)
			z2 = new(Int)
			z2.mul(stk, x1, y1)
			print("karatsuba wrong\n")
			trace("x ", &Int{abs: x})
			trace("y ", &Int{abs: y})
			trace("z ", &Int{abs: z})
			trace("zz", &Int{abs: zz})
			trace("x0", x0)
			trace("x1", x1)
			trace("y0", y0)
			trace("y1", y1)
			trace("tx", tx)
			trace("ty", ty)
			trace("z0", z0)
			trace("z1", z1)
			trace("z2", z2)
			panic("karatsuba")
		}
	}

}

// karatsubaSqr squares x,
// writing the (non-normalized) result to z.
// z must have length 2*len(x).
// It is analogous to [karatsuba] but can run faster
// knowing both multiplicands are the same value.
func karatsubaSqr(stk *stack, z, x nat) {
	n := len(x)
	if len(z) != 2*n {
		panic("bad karatsubaSqr length")
	}

	if n < karatsubaSqrThreshold || n < 2 {
		basicSqr(stk, z, x)
		return
	}

	// Recall that for karatsuba we want to compute:
	//
	//	x1:x0 * y1:y0 = x1y1:(x0y1+y0x1):x0y0
	//                = x1y1:((x0-x1)*(y1-y0)+x1y1+x0y0):x0y0
	//	              = z2:z1:z0
	// where:
	//
	//	z0 = x0y0
	//	z2 = x1y1
	//	z1 = (x0-x1)*(y1-y0) + z0 + z2
	//
	// When x = y, these simplify to:
	//
	//	z0 = x0²
	//	z2 = x1²
	//	z1 = z0 + z2 - (x0-x1)²

	n2 := (n + 1) / 2
	x0, x1 := &Int{abs: x[:n2].norm()}, &Int{abs: x[n2:].norm()}
	z0 := &Int{abs: z[0 : 2*n2]}
	z2 := &Int{abs: z[2*n2:]}

	// Allocate temporary storage for z1; repurpose z0 to hold tx.
	defer stk.restore(stk.save())
	z1 := &Int{abs: stk.nat(2*n2 + 1)}
	tx := &Int{abs: z[0:n2]}

	tx.Sub(x0, x1)
	z1.abs = z1.abs.sqr(stk, tx.abs)
	z1.neg = true

	clear(z)
	z0.abs = z0.abs.sqr(stk, x0.abs)
	z2.abs = z2.abs.sqr(stk, x1.abs)
	z1.Add(z1, z0)
	z1.Add(z1, z2)
	addTo(z[n2:], z1.abs)

	// Debug mode: double-check answer and print trace on failure.
	const debug = false
	if debug {
		zz := make(nat, len(z))
		basicSqr(stk, zz, x)
		if z.cmp(zz) != 0 {
			// All the temps were aliased to z and gone. Recompute.
			tx = new(Int).Sub(x0, x1)
			z0 = new(Int).Mul(x0, x0)
			z2 = new(Int).Mul(x1, x1)
			z1 = new(Int).Mul(tx, tx)
			z1.Neg(z1)
			z1.Add(z1, z0)
			z1.Add(z1, z2)
			print("karatsubaSqr wrong\n")
			trace("x ", &Int{abs: x})
			trace("z ", &Int{abs: z})
			trace("zz", &Int{abs: zz})
			trace("x0", x0)
			trace("x1", x1)
			trace("z0", z0)
			trace("z1", z1)
			trace("z2", z2)
			panic("karatsubaSqr")
		}
	}
}

// ifmt returns the debug formatting of the Int x: 0xHEX.
func ifmt(x *Int) string {
	neg, s, t := "", x.Text(16), ""
	if s == "" { // happens for denormalized zero
		s = "0x0"
	}
	if s[0] == '-' {
		neg, s = "-", s[1:]
	}

	// Add _ between words.
	const D = _W / 4 // digits per chunk
	for len(s) > D {
		s, t = s[:len(s)-D], s[len(s)-D:]+"_"+t
	}
	return neg + s + t
}

// trace prints a single debug value.
func trace(name string, x *Int) {
	print(name, "=", ifmt(x), "\n")
}

```

// === FILE: references!/go/src/math/big/prime.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package big

import "math/rand"

// ProbablyPrime reports whether x is probably prime,
// applying the Miller-Rabin test with n pseudorandomly chosen bases
// as well as a Baillie-PSW test.
//
// If x is prime, ProbablyPrime returns true.
// If x is chosen randomly and not prime, ProbablyPrime probably returns false.
// The probability of returning true for a randomly chosen non-prime is at most ¼ⁿ.
//
// ProbablyPrime is 100% accurate for inputs less than 2⁶⁴.
// See Menezes et al., Handbook of Applied Cryptography, 1997, pp. 145-149,
// and FIPS 186-4 Appendix F for further discussion of the error probabilities.
//
// ProbablyPrime is not suitable for judging primes that an adversary may
// have crafted to fool the test.
//
// As of Go 1.8, ProbablyPrime(0) is allowed and applies only a Baillie-PSW test.
// Before Go 1.8, ProbablyPrime applied only the Miller-Rabin tests, and ProbablyPrime(0) panicked.
func (x *Int) ProbablyPrime(n int) bool {
	// Note regarding the doc comment above:
	// It would be more precise to say that the Baillie-PSW test uses the
	// extra strong Lucas test as its Lucas test, but since no one knows
	// how to tell any of the Lucas tests apart inside a Baillie-PSW test
	// (they all work equally well empirically), that detail need not be
	// documented or implicitly guaranteed.
	// The comment does avoid saying "the" Baillie-PSW test
	// because of this general ambiguity.

	if n < 0 {
		panic("negative n for ProbablyPrime")
	}
	if x.neg || len(x.abs) == 0 {
		return false
	}

	// primeBitMask records the primes < 64.
	const primeBitMask uint64 = 1<<2 | 1<<3 | 1<<5 | 1<<7 |
		1<<11 | 1<<13 | 1<<17 | 1<<19 | 1<<23 | 1<<29 | 1<<31 |
		1<<37 | 1<<41 | 1<<43 | 1<<47 | 1<<53 | 1<<59 | 1<<61

	w := x.abs[0]
	if len(x.abs) == 1 && w < 64 {
		return primeBitMask&(1<<w) != 0
	}

	if w&1 == 0 {
		return false // x is even
	}

	const primesA = 3 * 5 * 7 * 11 * 13 * 17 * 19 * 23 * 37
	const primesB = 29 * 31 * 41 * 43 * 47 * 53

	var rA, rB uint32
	switch _W {
	case 32:
		rA = uint32(x.abs.modW(primesA))
		rB = uint32(x.abs.modW(primesB))
	case 64:
		r := x.abs.modW((primesA * primesB) & _M)
		rA = uint32(r % primesA)
		rB = uint32(r % primesB)
	default:
		panic("math/big: invalid word size")
	}

	if rA%3 == 0 || rA%5 == 0 || rA%7 == 0 || rA%11 == 0 || rA%13 == 0 || rA%17 == 0 || rA%19 == 0 || rA%23 == 0 || rA%37 == 0 ||
		rB%29 == 0 || rB%31 == 0 || rB%41 == 0 || rB%43 == 0 || rB%47 == 0 || rB%53 == 0 {
		return false
	}

	stk := getStack()
	defer stk.free()
	return x.abs.probablyPrimeMillerRabin(stk, n+1, true) && x.abs.probablyPrimeLucas(stk)
}

// probablyPrimeMillerRabin reports whether n passes reps rounds of the
// Miller-Rabin primality test, using pseudo-randomly chosen bases.
// If force2 is true, one of the rounds is forced to use base 2.
// See Handbook of Applied Cryptography, p. 139, Algorithm 4.24.
// The number n is known to be non-zero.
func (n nat) probablyPrimeMillerRabin(stk *stack, reps int, force2 bool) bool {
	nm1 := nat(nil).sub(n, natOne)
	// determine q, k such that nm1 = q << k
	k := nm1.trailingZeroBits()
	q := nat(nil).rsh(nm1, k)

	nm3 := nat(nil).sub(nm1, natTwo)
	rand := rand.New(rand.NewSource(int64(n[0])))

	var x, y, quotient nat
	nm3Len := nm3.bitLen()

NextRandom:
	for i := 0; i < reps; i++ {
		if i == reps-1 && force2 {
			x = x.set(natTwo)
		} else {
			x = x.random(rand, nm3, nm3Len)
			x = x.add(x, natTwo)
		}
		y = y.expNN(stk, x, q, n, false)
		if y.cmp(natOne) == 0 || y.cmp(nm1) == 0 {
			continue
		}
		for j := uint(1); j < k; j++ {
			y = y.sqr(stk, y)
			quotient, y = quotient.div(stk, y, y, n)
			if y.cmp(nm1) == 0 {
				continue NextRandom
			}
			if y.cmp(natOne) == 0 {
				return false
			}
		}
		return false
	}

	return true
}

// probablyPrimeLucas reports whether n passes the "almost extra strong" Lucas probable prime test,
// using Baillie-OEIS parameter selection. This corresponds to "AESLPSP" on Jacobsen's tables (link below).
// The combination of this test and a Miller-Rabin/Fermat test with base 2 gives a Baillie-PSW test.
//
// References:
//
// Baillie and Wagstaff, "Lucas Pseudoprimes", Mathematics of Computation 35(152),
// October 1980, pp. 1391-1417, especially page 1401.
// https://www.ams.org/journals/mcom/1980-35-152/S0025-5718-1980-0583518-6/S0025-5718-1980-0583518-6.pdf
//
// Grantham, "Frobenius Pseudoprimes", Mathematics of Computation 70(234),
// March 2000, pp. 873-891.
// https://www.ams.org/journals/mcom/2001-70-234/S0025-5718-00-01197-2/S0025-5718-00-01197-2.pdf
//
// Baillie, "Extra strong Lucas pseudoprimes", OEIS A217719, https://oeis.org/A217719.
//
// Jacobsen, "Pseudoprime Statistics, Tables, and Data", http://ntheory.org/pseudoprimes.html.
//
// Nicely, "The Baillie-PSW Primality Test", https://web.archive.org/web/20191121062007/http://www.trnicely.net/misc/bpsw.html.
// (Note that Nicely's definition of the "extra strong" test gives the wrong Jacobi condition,
// as pointed out by Jacobsen.)
//
// Crandall and Pomerance, Prime Numbers: A Computational Perspective, 2nd ed.
// Springer, 2005.
func (n nat) probablyPrimeLucas(stk *stack) bool {
	// Discard 0, 1.
	if len(n) == 0 || n.cmp(natOne) == 0 {
		return false
	}
	// Two is the only even prime.
	// Already checked by caller, but here to allow testing in isolation.
	if n[0]&1 == 0 {
		return n.cmp(natTwo) == 0
	}

	// Baillie-OEIS "method C" for choosing D, P, Q,
	// as in https://oeis.org/A217719/a217719.txt:
	// try increasing P ≥ 3 such that D = P² - 4 (so Q = 1)
	// until Jacobi(D, n) = -1.
	// The search is expected to succeed for non-square n after just a few trials.
	// After more than expected failures, check whether n is square
	// (which would cause Jacobi(D, n) = 1 for all D not dividing n).
	p := Word(3)
	d := nat{1}
	t1 := nat(nil) // temp
	intD := &Int{abs: d}
	intN := &Int{abs: n}
	for ; ; p++ {
		if p > 10000 {
			// This is widely believed to be impossible.
			// If we get a report, we'll want the exact number n.
			panic("math/big: internal error: cannot find (D/n) = -1 for " + intN.String())
		}
		d[0] = p*p - 4
		j := Jacobi(intD, intN)
		if j == -1 {
			break
		}
		if j == 0 {
			// d = p²-4 = (p-2)(p+2).
			// If (d/n) == 0 then d shares a prime factor with n.
			// Since the loop proceeds in increasing p and starts with p-2==1,
			// the shared prime factor must be p+2.
			// If p+2 == n, then n is prime; otherwise p+2 is a proper factor of n.
			return len(n) == 1 && n[0] == p+2
		}
		if p == 40 {
			// We'll never find (d/n) = -1 if n is a square.
			// If n is a non-square we expect to find a d in just a few attempts on average.
			// After 40 attempts, take a moment to check if n is indeed a square.
			t1 = t1.sqrt(stk, n)
			t1 = t1.sqr(stk, t1)
			if t1.cmp(n) == 0 {
				return false
			}
		}
	}

	// Grantham definition of "extra strong Lucas pseudoprime", after Thm 2.3 on p. 876
	// (D, P, Q above have become Δ, b, 1):
	//
	// Let U_n = U_n(b, 1), V_n = V_n(b, 1), and Δ = b²-4.
	// An extra strong Lucas pseudoprime to base b is a composite n = 2^r s + Jacobi(Δ, n),
	// where s is odd and gcd(n, 2*Δ) = 1, such that either (i) U_s ≡ 0 mod n and V_s ≡ ±2 mod n,
	// or (ii) V_{2^t s} ≡ 0 mod n for some 0 ≤ t < r-1.
	//
	// We know gcd(n, Δ) = 1 or else we'd have found Jacobi(d, n) == 0 above.
	// We know gcd(n, 2) = 1 because n is odd.
	//
	// Arrange s = (n - Jacobi(Δ, n)) / 2^r = (n+1) / 2^r.
	s := nat(nil).add(n, natOne)
	r := int(s.trailingZeroBits())
	s = s.rsh(s, uint(r))
	nm2 := nat(nil).sub(n, natTwo) // n-2

	// We apply the "almost extra strong" test, which checks the above conditions
	// except for U_s ≡ 0 mod n, which allows us to avoid computing any U_k values.
	// Jacobsen points out that maybe we should just do the full extra strong test:
	// "It is also possible to recover U_n using Crandall and Pomerance equation 3.13:
	// U_n = D^-1 (2V_{n+1} - PV_n) allowing us to run the full extra-strong test
	// at the cost of a single modular inversion. This computation is easy and fast in GMP,
	// so we can get the full extra-strong test at essentially the same performance as the
	// almost extra strong test."

	// Compute Lucas sequence V_s(b, 1), where:
	//
	//	V(0) = 2
	//	V(1) = P
	//	V(k) = P V(k-1) - Q V(k-2).
	//
	// (Remember that due to method C above, P = b, Q = 1.)
	//
	// In general V(k) = α^k + β^k, where α and β are roots of x² - Px + Q.
	// Crandall and Pomerance (p.147) observe that for 0 ≤ j ≤ k,
	//
	//	V(j+k) = V(j)V(k) - V(k-j).
	//
	// So in particular, to quickly double the subscript:
	//
	//	V(2k) = V(k)² - 2
	//	V(2k+1) = V(k) V(k+1) - P
	//
	// We can therefore start with k=0 and build up to k=s in log₂(s) steps.
	natP := nat(nil).setWord(p)
	vk := nat(nil).setWord(2)
	vk1 := nat(nil).setWord(p)
	t2 := nat(nil) // temp
	for i := int(s.bitLen()); i >= 0; i-- {
		if s.bit(uint(i)) != 0 {
			// k' = 2k+1
			// V(k') = V(2k+1) = V(k) V(k+1) - P.
			t1 = t1.mul(stk, vk, vk1)
			t1 = t1.add(t1, n)
			t1 = t1.sub(t1, natP)
			t2, vk = t2.div(stk, vk, t1, n)
			// V(k'+1) = V(2k+2) = V(k+1)² - 2.
			t1 = t1.sqr(stk, vk1)
			t1 = t1.add(t1, nm2)
			t2, vk1 = t2.div(stk, vk1, t1, n)
		} else {
			// k' = 2k
			// V(k'+1) = V(2k+1) = V(k) V(k+1) - P.
			t1 = t1.mul(stk, vk, vk1)
			t1 = t1.add(t1, n)
			t1 = t1.sub(t1, natP)
			t2, vk1 = t2.div(stk, vk1, t1, n)
			// V(k') = V(2k) = V(k)² - 2
			t1 = t1.sqr(stk, vk)
			t1 = t1.add(t1, nm2)
			t2, vk = t2.div(stk, vk, t1, n)
		}
	}

	// Now k=s, so vk = V(s). Check V(s) ≡ ±2 (mod n).
	if vk.cmp(natTwo) == 0 || vk.cmp(nm2) == 0 {
		// Check U(s) ≡ 0.
		// As suggested by Jacobsen, apply Crandall and Pomerance equation 3.13:
		//
		//	U(k) = D⁻¹ (2 V(k+1) - P V(k))
		//
		// Since we are checking for U(k) == 0 it suffices to check 2 V(k+1) == P V(k) mod n,
		// or P V(k) - 2 V(k+1) == 0 mod n.
		t1 := t1.mul(stk, vk, natP)
		t2 := t2.lsh(vk1, 1)
		if t1.cmp(t2) < 0 {
			t1, t2 = t2, t1
		}
		t1 = t1.sub(t1, t2)
		t3 := vk1 // steal vk1, no longer needed below
		vk1 = nil
		_ = vk1
		t2, t3 = t2.div(stk, t3, t1, n)
		if len(t3) == 0 {
			return true
		}
	}

	// Check V(2^t s) ≡ 0 mod n for some 0 ≤ t < r-1.
	for t := 0; t < r-1; t++ {
		if len(vk) == 0 { // vk == 0
			return true
		}
		// Optimization: V(k) = 2 is a fixed point for V(k') = V(k)² - 2,
		// so if V(k) = 2, we can stop: we will never find a future V(k) == 0.
		if len(vk) == 1 && vk[0] == 2 { // vk == 2
			return false
		}
		// k' = 2k
		// V(k') = V(2k) = V(k)² - 2
		t1 = t1.sqr(stk, vk)
		t1 = t1.sub(t1, natTwo)
		t2, vk = t2.div(stk, vk, t1, n)
	}
	return false
}

```

// === FILE: references!/go/src/math/big/rat.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements multi-precision rational numbers.

package big

import (
	"fmt"
	"math"
)

// A Rat represents a quotient a/b of arbitrary precision.
// The zero value for a Rat represents the value 0.
//
// Operations always take pointer arguments (*Rat) rather
// than Rat values, and each unique Rat value requires
// its own unique *Rat pointer. To "copy" a Rat value,
// an existing (or newly allocated) Rat must be set to
// a new value using the [Rat.Set] method; shallow copies
// of Rats are not supported and may lead to errors.
type Rat struct {
	// To make zero values for Rat work w/o initialization,
	// a zero value of b (len(b) == 0) acts like b == 1. At
	// the earliest opportunity (when an assignment to the Rat
	// is made), such uninitialized denominators are set to 1.
	// a.neg determines the sign of the Rat, b.neg is ignored.
	a, b Int
}

// NewRat creates a new [Rat] with numerator a and denominator b.
func NewRat(a, b int64) *Rat {
	return new(Rat).SetFrac64(a, b)
}

// SetFloat64 sets z to exactly f and returns z.
// If f is not finite, SetFloat returns nil.
func (z *Rat) SetFloat64(f float64) *Rat {
	const expMask = 1<<11 - 1
	bits := math.Float64bits(f)
	mantissa := bits & (1<<52 - 1)
	exp := int((bits >> 52) & expMask)
	switch exp {
	case expMask: // non-finite
		return nil
	case 0: // denormal
		exp -= 1022
	default: // normal
		mantissa |= 1 << 52
		exp -= 1023
	}

	shift := 52 - exp

	// Optimization (?): partially pre-normalise.
	for mantissa&1 == 0 && shift > 0 {
		mantissa >>= 1
		shift--
	}

	z.a.SetUint64(mantissa)
	z.a.neg = f < 0
	z.b.Set(intOne)
	if shift > 0 {
		z.b.Lsh(&z.b, uint(shift))
	} else {
		z.a.Lsh(&z.a, uint(-shift))
	}
	return z.norm()
}

// quotToFloat32 returns the non-negative float32 value
// nearest to the quotient a/b, using round-to-even in
// halfway cases. It does not mutate its arguments.
// Preconditions: b is non-zero; a and b have no common factors.
func quotToFloat32(stk *stack, a, b nat) (f float32, exact bool) {
	const (
		// float size in bits
		Fsize = 32

		// mantissa
		Msize  = 23
		Msize1 = Msize + 1 // incl. implicit 1
		Msize2 = Msize1 + 1

		// exponent
		Esize = Fsize - Msize1
		Ebias = 1<<(Esize-1) - 1
		Emin  = 1 - Ebias
		Emax  = Ebias
	)

	// TODO(adonovan): specialize common degenerate cases: 1.0, integers.
	alen := a.bitLen()
	if alen == 0 {
		return 0, true
	}
	blen := b.bitLen()
	if blen == 0 {
		panic("division by zero")
	}

	// 1. Left-shift A or B such that quotient A/B is in [1<<Msize1, 1<<(Msize2+1)
	// (Msize2 bits if A < B when they are left-aligned, Msize2+1 bits if A >= B).
	// This is 2 or 3 more than the float32 mantissa field width of Msize:
	// - the optional extra bit is shifted away in step 3 below.
	// - the high-order 1 is omitted in "normal" representation;
	// - the low-order 1 will be used during rounding then discarded.
	exp := alen - blen
	var a2, b2 nat
	a2 = a2.set(a)
	b2 = b2.set(b)
	if shift := Msize2 - exp; shift > 0 {
		a2 = a2.lsh(a2, uint(shift))
	} else if shift < 0 {
		b2 = b2.lsh(b2, uint(-shift))
	}

	// 2. Compute quotient and remainder (q, r).  NB: due to the
	// extra shift, the low-order bit of q is logically the
	// high-order bit of r.
	var q nat
	q, r := q.div(stk, a2, a2, b2) // (recycle a2)
	mantissa := low32(q)
	haveRem := len(r) > 0 // mantissa&1 && !haveRem => remainder is exactly half

	// 3. If quotient didn't fit in Msize2 bits, redo division by b2<<1
	// (in effect---we accomplish this incrementally).
	if mantissa>>Msize2 == 1 {
		if mantissa&1 == 1 {
			haveRem = true
		}
		mantissa >>= 1
		exp++
	}
	if mantissa>>Msize1 != 1 {
		panic(fmt.Sprintf("expected exactly %d bits of result", Msize2))
	}

	// 4. Rounding.
	if Emin-Msize <= exp && exp <= Emin {
		// Denormal case; lose 'shift' bits of precision.
		shift := uint(Emin - (exp - 1)) // [1..Esize1)
		lostbits := mantissa & (1<<shift - 1)
		haveRem = haveRem || lostbits != 0
		mantissa >>= shift
		exp = 2 - Ebias // == exp + shift
	}
	// Round q using round-half-to-even.
	exact = !haveRem
	if mantissa&1 != 0 {
		exact = false
		if haveRem || mantissa&2 != 0 {
			if mantissa++; mantissa >= 1<<Msize2 {
				// Complete rollover 11...1 => 100...0, so shift is safe
				mantissa >>= 1
				exp++
			}
		}
	}
	mantissa >>= 1 // discard rounding bit.  Mantissa now scaled by 1<<Msize1.

	f = float32(math.Ldexp(float64(mantissa), exp-Msize1))
	if math.IsInf(float64(f), 0) {
		exact = false
	}
	return
}

// quotToFloat64 returns the non-negative float64 value
// nearest to the quotient a/b, using round-to-even in
// halfway cases. It does not mutate its arguments.
// Preconditions: b is non-zero; a and b have no common factors.
func quotToFloat64(stk *stack, a, b nat) (f float64, exact bool) {
	const (
		// float size in bits
		Fsize = 64

		// mantissa
		Msize  = 52
		Msize1 = Msize + 1 // incl. implicit 1
		Msize2 = Msize1 + 1

		// exponent
		Esize = Fsize - Msize1
		Ebias = 1<<(Esize-1) - 1
		Emin  = 1 - Ebias
		Emax  = Ebias
	)

	// TODO(adonovan): specialize common degenerate cases: 1.0, integers.
	alen := a.bitLen()
	if alen == 0 {
		return 0, true
	}
	blen := b.bitLen()
	if blen == 0 {
		panic("division by zero")
	}

	// 1. Left-shift A or B such that quotient A/B is in [1<<Msize1, 1<<(Msize2+1)
	// (Msize2 bits if A < B when they are left-aligned, Msize2+1 bits if A >= B).
	// This is 2 or 3 more than the float64 mantissa field width of Msize:
	// - the optional extra bit is shifted away in step 3 below.
	// - the high-order 1 is omitted in "normal" representation;
	// - the low-order 1 will be used during rounding then discarded.
	exp := alen - blen
	var a2, b2 nat
	a2 = a2.set(a)
	b2 = b2.set(b)
	if shift := Msize2 - exp; shift > 0 {
		a2 = a2.lsh(a2, uint(shift))
	} else if shift < 0 {
		b2 = b2.lsh(b2, uint(-shift))
	}

	// 2. Compute quotient and remainder (q, r).  NB: due to the
	// extra shift, the low-order bit of q is logically the
	// high-order bit of r.
	var q nat
	q, r := q.div(stk, a2, a2, b2) // (recycle a2)
	mantissa := low64(q)
	haveRem := len(r) > 0 // mantissa&1 && !haveRem => remainder is exactly half

	// 3. If quotient didn't fit in Msize2 bits, redo division by b2<<1
	// (in effect---we accomplish this incrementally).
	if mantissa>>Msize2 == 1 {
		if mantissa&1 == 1 {
			haveRem = true
		}
		mantissa >>= 1
		exp++
	}
	if mantissa>>Msize1 != 1 {
		panic(fmt.Sprintf("expected exactly %d bits of result", Msize2))
	}

	// 4. Rounding.
	if Emin-Msize <= exp && exp <= Emin {
		// Denormal case; lose 'shift' bits of precision.
		shift := uint(Emin - (exp - 1)) // [1..Esize1)
		lostbits := mantissa & (1<<shift - 1)
		haveRem = haveRem || lostbits != 0
		mantissa >>= shift
		exp = 2 - Ebias // == exp + shift
	}
	// Round q using round-half-to-even.
	exact = !haveRem
	if mantissa&1 != 0 {
		exact = false
		if haveRem || mantissa&2 != 0 {
			if mantissa++; mantissa >= 1<<Msize2 {
				// Complete rollover 11...1 => 100...0, so shift is safe
				mantissa >>= 1
				exp++
			}
		}
	}
	mantissa >>= 1 // discard rounding bit.  Mantissa now scaled by 1<<Msize1.

	f = math.Ldexp(float64(mantissa), exp-Msize1)
	if math.IsInf(f, 0) {
		exact = false
	}
	return
}

// Float32 returns the nearest float32 value for x and a bool indicating
// whether f represents x exactly. If the magnitude of x is too large to
// be represented by a float32, f is an infinity and exact is false.
// The sign of f always matches the sign of x, even if f == 0.
func (x *Rat) Float32() (f float32, exact bool) {
	b := x.b.abs
	if len(b) == 0 {
		b = natOne
	}
	stk := getStack()
	defer stk.free()
	f, exact = quotToFloat32(stk, x.a.abs, b)
	if x.a.neg {
		f = -f
	}
	return
}

// Float64 returns the nearest float64 value for x and a bool indicating
// whether f represents x exactly. If the magnitude of x is too large to
// be represented by a float64, f is an infinity and exact is false.
// The sign of f always matches the sign of x, even if f == 0.
func (x *Rat) Float64() (f float64, exact bool) {
	b := x.b.abs
	if len(b) == 0 {
		b = natOne
	}
	stk := getStack()
	defer stk.free()
	f, exact = quotToFloat64(stk, x.a.abs, b)
	if x.a.neg {
		f = -f
	}
	return
}

// SetFrac sets z to a/b and returns z.
// If b == 0, SetFrac panics.
func (z *Rat) SetFrac(a, b *Int) *Rat {
	z.a.neg = a.neg != b.neg
	babs := b.abs
	if len(babs) == 0 {
		panic("division by zero")
	}
	if &z.a == b || alias(z.a.abs, babs) {
		babs = nat(nil).set(babs) // make a copy
	}
	z.a.abs = z.a.abs.set(a.abs)
	z.b.abs = z.b.abs.set(babs)
	return z.norm()
}

// SetFrac64 sets z to a/b and returns z.
// If b == 0, SetFrac64 panics.
func (z *Rat) SetFrac64(a, b int64) *Rat {
	if b == 0 {
		panic("division by zero")
	}
	z.a.SetInt64(a)
	if b < 0 {
		b = -b
		z.a.neg = !z.a.neg
	}
	z.b.abs = z.b.abs.setUint64(uint64(b))
	return z.norm()
}

// SetInt sets z to x (by making a copy of x) and returns z.
func (z *Rat) SetInt(x *Int) *Rat {
	z.a.Set(x)
	z.b.abs = z.b.abs.setWord(1)
	return z
}

// SetInt64 sets z to x and returns z.
func (z *Rat) SetInt64(x int64) *Rat {
	z.a.SetInt64(x)
	z.b.abs = z.b.abs.setWord(1)
	return z
}

// SetUint64 sets z to x and returns z.
func (z *Rat) SetUint64(x uint64) *Rat {
	z.a.SetUint64(x)
	z.b.abs = z.b.abs.setWord(1)
	return z
}

// Set sets z to x (by making a copy of x) and returns z.
func (z *Rat) Set(x *Rat) *Rat {
	if z != x {
		z.a.Set(&x.a)
		z.b.Set(&x.b)
	}
	if len(z.b.abs) == 0 {
		z.b.abs = z.b.abs.setWord(1)
	}
	return z
}

// Abs sets z to |x| (the absolute value of x) and returns z.
func (z *Rat) Abs(x *Rat) *Rat {
	z.Set(x)
	z.a.neg = false
	return z
}

// Neg sets z to -x and returns z.
func (z *Rat) Neg(x *Rat) *Rat {
	z.Set(x)
	z.a.neg = len(z.a.abs) > 0 && !z.a.neg // 0 has no sign
	return z
}

// Inv sets z to 1/x and returns z.
// If x == 0, Inv panics.
func (z *Rat) Inv(x *Rat) *Rat {
	if len(x.a.abs) == 0 {
		panic("division by zero")
	}
	z.Set(x)
	z.a.abs, z.b.abs = z.b.abs, z.a.abs
	return z
}

// Sign returns:
//   - -1 if x < 0;
//   - 0 if x == 0;
//   - +1 if x > 0.
func (x *Rat) Sign() int {
	return x.a.Sign()
}

// IsInt reports whether the denominator of x is 1.
func (x *Rat) IsInt() bool {
	return len(x.b.abs) == 0 || x.b.abs.cmp(natOne) == 0
}

// Num returns the numerator of x; it may be <= 0.
// The result is a reference to x's numerator; it
// may change if a new value is assigned to x, and vice versa.
// The sign of the numerator corresponds to the sign of x.
func (x *Rat) Num() *Int {
	return &x.a
}

// Denom returns the denominator of x; it is always > 0.
// The result is a reference to x's denominator, unless
// x is an uninitialized (zero value) [Rat], in which case
// the result is a new [Int] of value 1. (To initialize x,
// any operation that sets x will do, including x.Set(x).)
// If the result is a reference to x's denominator it
// may change if a new value is assigned to x, and vice versa.
func (x *Rat) Denom() *Int {
	// Note that x.b.neg is guaranteed false.
	if len(x.b.abs) == 0 {
		// Note: If this proves problematic, we could
		//       panic instead and require the Rat to
		//       be explicitly initialized.
		return &Int{abs: nat{1}}
	}
	return &x.b
}

func (z *Rat) norm() *Rat {
	switch {
	case len(z.a.abs) == 0:
		// z == 0; normalize sign and denominator
		z.a.neg = false
		fallthrough
	case len(z.b.abs) == 0:
		// z is integer; normalize denominator
		z.b.abs = z.b.abs.setWord(1)
	default:
		// z is fraction; normalize numerator and denominator
		stk := getStack()
		defer stk.free()
		neg := z.a.neg
		z.a.neg = false
		z.b.neg = false
		if f := NewInt(0).lehmerGCD(nil, nil, &z.a, &z.b); f.Cmp(intOne) != 0 {
			z.a.abs, _ = z.a.abs.div(stk, nil, z.a.abs, f.abs)
			z.b.abs, _ = z.b.abs.div(stk, nil, z.b.abs, f.abs)
		}
		z.a.neg = neg
	}
	return z
}

// mulDenom sets z to the denominator product x*y (by taking into
// account that 0 values for x or y must be interpreted as 1) and
// returns z.
func mulDenom(stk *stack, z, x, y nat) nat {
	switch {
	case len(x) == 0 && len(y) == 0:
		return z.setWord(1)
	case len(x) == 0:
		return z.set(y)
	case len(y) == 0:
		return z.set(x)
	}
	return z.mul(stk, x, y)
}

// scaleDenom sets z to the product x*f.
// If f == 0 (zero value of denominator), z is set to (a copy of) x.
func (z *Int) scaleDenom(stk *stack, x *Int, f nat) {
	if len(f) == 0 {
		z.Set(x)
		return
	}
	z.abs = z.abs.mul(stk, x.abs, f)
	z.neg = x.neg
}

// Cmp compares x and y and returns:
//   - -1 if x < y;
//   - 0 if x == y;
//   - +1 if x > y.
func (x *Rat) Cmp(y *Rat) int {
	var a, b Int
	stk := getStack()
	defer stk.free()
	a.scaleDenom(stk, &x.a, y.b.abs)
	b.scaleDenom(stk, &y.a, x.b.abs)
	return a.Cmp(&b)
}

// Add sets z to the sum x+y and returns z.
func (z *Rat) Add(x, y *Rat) *Rat {
	stk := getStack()
	defer stk.free()

	var a1, a2 Int
	a1.scaleDenom(stk, &x.a, y.b.abs)
	a2.scaleDenom(stk, &y.a, x.b.abs)
	z.a.Add(&a1, &a2)
	z.b.abs = mulDenom(stk, z.b.abs, x.b.abs, y.b.abs)
	return z.norm()
}

// Sub sets z to the difference x-y and returns z.
func (z *Rat) Sub(x, y *Rat) *Rat {
	stk := getStack()
	defer stk.free()

	var a1, a2 Int
	a1.scaleDenom(stk, &x.a, y.b.abs)
	a2.scaleDenom(stk, &y.a, x.b.abs)
	z.a.Sub(&a1, &a2)
	z.b.abs = mulDenom(stk, z.b.abs, x.b.abs, y.b.abs)
	return z.norm()
}

// Mul sets z to the product x*y and returns z.
func (z *Rat) Mul(x, y *Rat) *Rat {
	stk := getStack()
	defer stk.free()

	if x == y {
		// a squared Rat is positive and can't be reduced (no need to call norm())
		z.a.neg = false
		z.a.abs = z.a.abs.sqr(stk, x.a.abs)
		if len(x.b.abs) == 0 {
			z.b.abs = z.b.abs.setWord(1)
		} else {
			z.b.abs = z.b.abs.sqr(stk, x.b.abs)
		}
		return z
	}

	z.a.mul(stk, &x.a, &y.a)
	z.b.abs = mulDenom(stk, z.b.abs, x.b.abs, y.b.abs)
	return z.norm()
}

// Quo sets z to the quotient x/y and returns z.
// If y == 0, Quo panics.
func (z *Rat) Quo(x, y *Rat) *Rat {
	stk := getStack()
	defer stk.free()

	if len(y.a.abs) == 0 {
		panic("division by zero")
	}
	var a, b Int
	a.scaleDenom(stk, &x.a, y.b.abs)
	b.scaleDenom(stk, &y.a, x.b.abs)
	z.a.abs = a.abs
	z.b.abs = b.abs
	z.a.neg = a.neg != b.neg
	return z.norm()
}

```

// === FILE: references!/go/src/math/big/ratconv.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements rat-to-string conversion functions.

package big

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func ratTok(ch rune) bool {
	return strings.ContainsRune("+-/0123456789.eE", ch)
}

var ratZero Rat
var _ fmt.Scanner = &ratZero // *Rat must implement fmt.Scanner

// Scan is a support routine for fmt.Scanner. It accepts the formats
// 'e', 'E', 'f', 'F', 'g', 'G', and 'v'. All formats are equivalent.
func (z *Rat) Scan(s fmt.ScanState, ch rune) error {
	tok, err := s.Token(true, ratTok)
	if err != nil {
		return err
	}
	if !strings.ContainsRune("efgEFGv", ch) {
		return errors.New("Rat.Scan: invalid verb")
	}
	if _, ok := z.SetString(string(tok)); !ok {
		return errors.New("Rat.Scan: invalid syntax")
	}
	return nil
}

// SetString sets z to the value of s and returns z and a boolean indicating
// success. s can be given as a (possibly signed) fraction "a/b", or as a
// floating-point number optionally followed by an exponent.
// If a fraction is provided, both the dividend and the divisor may be a
// decimal integer or independently use a prefix of “0b”, “0” or “0o”,
// or “0x” (or their upper-case variants) to denote a binary, octal, or
// hexadecimal integer, respectively. The divisor may not be signed.
// If a floating-point number is provided, it may be in decimal form or
// use any of the same prefixes as above but for “0” to denote a non-decimal
// mantissa. A leading “0” is considered a decimal leading 0; it does not
// indicate octal representation in this case.
// An optional base-10 “e” or base-2 “p” (or their upper-case variants)
// exponent may be provided as well, except for hexadecimal floats which
// only accept an (optional) “p” exponent (because an “e” or “E” cannot
// be distinguished from a mantissa digit). If the exponent's absolute value
// is too large, the operation may fail.
// The entire string, not just a prefix, must be valid for success. If the
// operation failed, the value of z is undefined but the returned value is nil.
func (z *Rat) SetString(s string) (*Rat, bool) {
	if len(s) == 0 {
		return nil, false
	}
	// len(s) > 0

	// parse fraction a/b, if any
	if sep := strings.Index(s, "/"); sep >= 0 {
		if _, ok := z.a.SetString(s[:sep], 0); !ok {
			return nil, false
		}
		r := strings.NewReader(s[sep+1:])
		var err error
		if z.b.abs, _, _, err = z.b.abs.scan(r, 0, false); err != nil {
			return nil, false
		}
		// entire string must have been consumed
		if _, err = r.ReadByte(); err != io.EOF {
			return nil, false
		}
		if len(z.b.abs) == 0 {
			return nil, false
		}
		return z.norm(), true
	}

	// parse floating-point number
	r := strings.NewReader(s)

	// sign
	neg, err := scanSign(r)
	if err != nil {
		return nil, false
	}

	// mantissa
	var base int
	var fcount int // fractional digit count; valid if <= 0
	z.a.abs, base, fcount, err = z.a.abs.scan(r, 0, true)
	if err != nil {
		return nil, false
	}

	// exponent
	var exp int64
	var ebase int
	exp, ebase, err = scanExponent(r, true, true)
	if err != nil {
		return nil, false
	}

	// there should be no unread characters left
	if _, err = r.ReadByte(); err != io.EOF {
		return nil, false
	}

	// special-case 0 (see also issue #16176)
	if len(z.a.abs) == 0 {
		return z.norm(), true
	}
	// len(z.a.abs) > 0

	// The mantissa may have a radix point (fcount <= 0) and there
	// may be a nonzero exponent exp. The radix point amounts to a
	// division by base**(-fcount), which equals a multiplication by
	// base**fcount. An exponent means multiplication by ebase**exp.
	// Multiplications are commutative, so we can apply them in any
	// order. We only have powers of 2 and 10, and we split powers
	// of 10 into the product of the same powers of 2 and 5. This
	// may reduce the size of shift/multiplication factors or
	// divisors required to create the final fraction, depending
	// on the actual floating-point value.

	// determine binary or decimal exponent contribution of radix point
	var exp2, exp5 int64
	if fcount < 0 {
		// The mantissa has a radix point ddd.dddd; and
		// -fcount is the number of digits to the right
		// of '.'. Adjust relevant exponent accordingly.
		d := int64(fcount)
		switch base {
		case 10:
			exp5 = d
			fallthrough // 10**e == 5**e * 2**e
		case 2:
			exp2 = d
		case 8:
			exp2 = d * 3 // octal digits are 3 bits each
		case 16:
			exp2 = d * 4 // hexadecimal digits are 4 bits each
		default:
			panic("unexpected mantissa base")
		}
		// fcount consumed - not needed anymore
	}

	// take actual exponent into account
	switch ebase {
	case 10:
		exp5 += exp
		fallthrough // see fallthrough above
	case 2:
		exp2 += exp
	default:
		panic("unexpected exponent base")
	}
	// exp consumed - not needed anymore

	stk := getStack()
	defer stk.free()

	// apply exp5 contributions
	// (start with exp5 so the numbers to multiply are smaller)
	if exp5 != 0 {
		n := exp5
		if n < 0 {
			n = -n
			if n < 0 {
				// This can occur if -n overflows. -(-1 << 63) would become
				// -1 << 63, which is still negative.
				return nil, false
			}
		}
		if n > 1e6 {
			return nil, false // avoid excessively large exponents
		}
		pow5 := z.b.abs.expNN(stk, natFive, nat(nil).setWord(Word(n)), nil, false) // use underlying array of z.b.abs
		if exp5 > 0 {
			z.a.abs = z.a.abs.mul(stk, z.a.abs, pow5)
			z.b.abs = z.b.abs.setWord(1)
		} else {
			z.b.abs = pow5
		}
	} else {
		z.b.abs = z.b.abs.setWord(1)
	}

	// apply exp2 contributions
	if exp2 < -1e7 || exp2 > 1e7 {
		return nil, false // avoid excessively large exponents
	}
	if exp2 > 0 {
		z.a.abs = z.a.abs.lsh(z.a.abs, uint(exp2))
	} else if exp2 < 0 {
		z.b.abs = z.b.abs.lsh(z.b.abs, uint(-exp2))
	}

	z.a.neg = neg && len(z.a.abs) > 0 // 0 has no sign

	return z.norm(), true
}

// scanExponent scans the longest possible prefix of r representing a base 10
// (“e”, “E”) or a base 2 (“p”, “P”) exponent, if any. It returns the
// exponent, the exponent base (10 or 2), or a read or syntax error, if any.
//
// If sepOk is set, an underscore character “_” may appear between successive
// exponent digits; such underscores do not change the value of the exponent.
// Incorrect placement of underscores is reported as an error if there are no
// other errors. If sepOk is not set, underscores are not recognized and thus
// terminate scanning like any other character that is not a valid digit.
//
//	exponent = ( "e" | "E" | "p" | "P" ) [ sign ] digits .
//	sign     = "+" | "-" .
//	digits   = digit { [ '_' ] digit } .
//	digit    = "0" ... "9" .
//
// A base 2 exponent is only permitted if base2ok is set.
func scanExponent(r io.ByteScanner, base2ok, sepOk bool) (exp int64, base int, err error) {
	// one char look-ahead
	ch, err := r.ReadByte()
	if err != nil {
		if err == io.EOF {
			err = nil
		}
		return 0, 10, err
	}

	// exponent char
	switch ch {
	case 'e', 'E':
		base = 10
	case 'p', 'P':
		if base2ok {
			base = 2
			break // ok
		}
		fallthrough // binary exponent not permitted
	default:
		r.UnreadByte() // ch does not belong to exponent anymore
		return 0, 10, nil
	}

	// sign
	var digits []byte
	ch, err = r.ReadByte()
	if err == nil && (ch == '+' || ch == '-') {
		if ch == '-' {
			digits = append(digits, '-')
		}
		ch, err = r.ReadByte()
	}

	// prev encodes the previously seen char: it is one
	// of '_', '0' (a digit), or '.' (anything else). A
	// valid separator '_' may only occur after a digit.
	prev := '.'
	invalSep := false

	// exponent value
	hasDigits := false
	for err == nil {
		if '0' <= ch && ch <= '9' {
			digits = append(digits, ch)
			prev = '0'
			hasDigits = true
		} else if ch == '_' && sepOk {
			if prev != '0' {
				invalSep = true
			}
			prev = '_'
		} else {
			r.UnreadByte() // ch does not belong to number anymore
			break
		}
		ch, err = r.ReadByte()
	}

	if err == io.EOF {
		err = nil
	}
	if err == nil && !hasDigits {
		err = errNoDigits
	}
	if err == nil {
		exp, err = strconv.ParseInt(string(digits), 10, 64)
	}
	// other errors take precedence over invalid separators
	if err == nil && (invalSep || prev == '_') {
		err = errInvalSep
	}

	return
}

// String returns a string representation of x in the form "a/b" (even if b == 1).
func (x *Rat) String() string {
	return string(x.marshal(nil))
}

// marshal implements [Rat.String] returning a slice of bytes.
// It appends the string representation of x in the form "a/b" (even if b == 1) to buf,
// and returns the extended buffer.
func (x *Rat) marshal(buf []byte) []byte {
	buf = x.a.Append(buf, 10)
	buf = append(buf, '/')
	if len(x.b.abs) != 0 {
		buf = x.b.Append(buf, 10)
	} else {
		buf = append(buf, '1')
	}
	return buf
}

// RatString returns a string representation of x in the form "a/b" if b != 1,
// and in the form "a" if b == 1.
func (x *Rat) RatString() string {
	if x.IsInt() {
		return x.a.String()
	}
	return x.String()
}

// FloatString returns a string representation of x in decimal form with prec
// digits of precision after the radix point. The last digit is rounded to
// nearest, with halves rounded away from zero.
func (x *Rat) FloatString(prec int) string {
	var buf []byte

	if x.IsInt() {
		buf = x.a.Append(buf, 10)
		if prec > 0 {
			buf = append(buf, '.')
			for i := prec; i > 0; i-- {
				buf = append(buf, '0')
			}
		}
		return string(buf)
	}
	// x.b.abs != 0

	stk := getStack()
	defer stk.free()
	q, r := nat(nil).div(stk, nat(nil), x.a.abs, x.b.abs)

	p := natOne
	if prec > 0 {
		p = nat(nil).expNN(stk, natTen, nat(nil).setUint64(uint64(prec)), nil, false)
	}

	r = r.mul(stk, r, p)
	r, r2 := r.div(stk, nat(nil), r, x.b.abs)

	// see if we need to round up
	r2 = r2.add(r2, r2)
	if x.b.abs.cmp(r2) <= 0 {
		r = r.add(r, natOne)
		if r.cmp(p) >= 0 {
			q = nat(nil).add(q, natOne)
			r = nat(nil).sub(r, p)
		}
	}

	if x.a.neg {
		buf = append(buf, '-')
	}
	buf = append(buf, q.utoa(10)...) // itoa ignores sign if q == 0

	if prec > 0 {
		buf = append(buf, '.')
		rs := r.utoa(10)
		for i := prec - len(rs); i > 0; i-- {
			buf = append(buf, '0')
		}
		buf = append(buf, rs...)
	}

	return string(buf)
}

// Note: FloatPrec (below) is in this file rather than rat.go because
//       its results are relevant for decimal representation/printing.

// FloatPrec returns the number n of non-repeating digits immediately
// following the decimal point of the decimal representation of x.
// The boolean result indicates whether a decimal representation of x
// with that many fractional digits is exact or rounded.
//
// Examples:
//
//	x      n    exact    decimal representation n fractional digits
//	0      0    true     0
//	1      0    true     1
//	1/2    1    true     0.5
//	1/3    0    false    0       (0.333... rounded)
//	1/4    2    true     0.25
//	1/6    1    false    0.2     (0.166... rounded)
func (x *Rat) FloatPrec() (n int, exact bool) {
	stk := getStack()
	defer stk.free()

	// Determine q and largest p2, p5 such that d = q·2^p2·5^p5.
	// The results n, exact are:
	//
	//     n = max(p2, p5)
	//     exact = q == 1
	//
	// For details see:
	// https://en.wikipedia.org/wiki/Repeating_decimal#Reciprocals_of_integers_not_coprime_to_10
	d := x.Denom().abs // d >= 1

	// Determine p2 by counting factors of 2.
	// p2 corresponds to the trailing zero bits in d.
	// Do this first to reduce q as much as possible.
	var q nat
	p2 := d.trailingZeroBits()
	q = q.rsh(d, p2)

	// Determine p5 by counting factors of 5.
	// Build a table starting with an initial power of 5,
	// and use repeated squaring until the factor doesn't
	// divide q anymore. Then use the table to determine
	// the power of 5 in q.
	const fp = 13        // f == 5^fp
	var tab []nat        // tab[i] == (5^fp)^(2^i) == 5^(fp·2^i)
	f := nat{1220703125} // == 5^fp (must fit into a uint32 Word)
	var t, r nat         // temporaries
	for {
		if _, r = t.div(stk, r, q, f); len(r) != 0 {
			break // f doesn't divide q evenly
		}
		tab = append(tab, f)
		f = nat(nil).sqr(stk, f) // nat(nil) to ensure a new f for each table entry
	}

	// Factor q using the table entries, if any.
	// We start with the largest factor f = tab[len(tab)-1]
	// that evenly divides q. It does so at most once because
	// otherwise f·f would also divide q. That can't be true
	// because f·f is the next higher table entry, contradicting
	// how f was chosen in the first place.
	// The same reasoning applies to the subsequent factors.
	var p5 uint
	for i := len(tab) - 1; i >= 0; i-- {
		if t, r = t.div(stk, r, q, tab[i]); len(r) == 0 {
			p5 += fp * (1 << i) // tab[i] == 5^(fp·2^i)
			q = q.set(t)
		}
	}

	// If fp != 1, we may still have multiples of 5 left.
	for {
		if t, r = t.div(stk, r, q, natFive); len(r) != 0 {
			break
		}
		p5++
		q = q.set(t)
	}

	return int(max(p2, p5)), q.cmp(natOne) == 0
}

```

// === FILE: references!/go/src/math/big/ratmarsh.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements encoding/decoding of Rats.

package big

import (
	"errors"
	"fmt"
	"internal/byteorder"
	"math"
)

// Gob codec version. Permits backward-compatible changes to the encoding.
const ratGobVersion byte = 1

// GobEncode implements the [encoding/gob.GobEncoder] interface.
func (x *Rat) GobEncode() ([]byte, error) {
	if x == nil {
		return nil, nil
	}
	buf := make([]byte, 1+4+(len(x.a.abs)+len(x.b.abs))*_S) // extra bytes for version and sign bit (1), and numerator length (4)
	i := x.b.abs.bytes(buf)
	j := x.a.abs.bytes(buf[:i])
	n := i - j
	if int(uint32(n)) != n {
		// this should never happen
		return nil, errors.New("Rat.GobEncode: numerator too large")
	}
	byteorder.BEPutUint32(buf[j-4:j], uint32(n))
	j -= 1 + 4
	b := ratGobVersion << 1 // make space for sign bit
	if x.a.neg {
		b |= 1
	}
	buf[j] = b
	return buf[j:], nil
}

// GobDecode implements the [encoding/gob.GobDecoder] interface.
func (z *Rat) GobDecode(buf []byte) error {
	if len(buf) == 0 {
		// Other side sent a nil or default value.
		*z = Rat{}
		return nil
	}
	if len(buf) < 5 {
		return errors.New("Rat.GobDecode: buffer too small")
	}
	b := buf[0]
	if b>>1 != ratGobVersion {
		return fmt.Errorf("Rat.GobDecode: encoding version %d not supported", b>>1)
	}
	const j = 1 + 4
	ln := byteorder.BEUint32(buf[j-4 : j])
	if uint64(ln) > math.MaxInt-j {
		return errors.New("Rat.GobDecode: invalid length")
	}
	i := j + int(ln)
	if len(buf) < i {
		return errors.New("Rat.GobDecode: buffer too small")
	}
	z.a.neg = b&1 != 0
	z.a.abs = z.a.abs.setBytes(buf[j:i])
	z.b.abs = z.b.abs.setBytes(buf[i:])
	return nil
}

// AppendText implements the [encoding.TextAppender] interface.
func (x *Rat) AppendText(b []byte) ([]byte, error) {
	if x.IsInt() {
		return x.a.AppendText(b)
	}
	return x.marshal(b), nil
}

// MarshalText implements the [encoding.TextMarshaler] interface.
func (x *Rat) MarshalText() (text []byte, err error) {
	return x.AppendText(nil)
}

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (z *Rat) UnmarshalText(text []byte) error {
	// TODO(gri): get rid of the []byte/string conversion
	if _, ok := z.SetString(string(text)); !ok {
		return fmt.Errorf("math/big: cannot unmarshal %q into a *big.Rat", text)
	}
	return nil
}

```

// === FILE: references!/go/src/math/big/roundingmode_string.go ===
```go
// Code generated by "stringer -type=RoundingMode"; DO NOT EDIT.

package big

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[ToNearestEven-0]
	_ = x[ToNearestAway-1]
	_ = x[ToZero-2]
	_ = x[AwayFromZero-3]
	_ = x[ToNegativeInf-4]
	_ = x[ToPositiveInf-5]
}

const _RoundingMode_name = "ToNearestEvenToNearestAwayToZeroAwayFromZeroToNegativeInfToPositiveInf"

var _RoundingMode_index = [...]uint8{0, 13, 26, 32, 44, 57, 70}

func (i RoundingMode) String() string {
	idx := int(i) - 0
	if i < 0 || idx >= len(_RoundingMode_index)-1 {
		return "RoundingMode(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _RoundingMode_name[_RoundingMode_index[idx]:_RoundingMode_index[idx+1]]
}

```

// === FILE: references!/go/src/math/big/sqrt.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package big

import (
	"math"
	"sync"
)

var threeOnce struct {
	sync.Once
	v *Float
}

func three() *Float {
	threeOnce.Do(func() {
		threeOnce.v = NewFloat(3.0)
	})
	return threeOnce.v
}

// Sqrt sets z to the rounded square root of x, and returns it.
//
// If z's precision is 0, it is changed to x's precision before the
// operation. Rounding is performed according to z's precision and
// rounding mode, but z's accuracy is not computed. Specifically, the
// result of z.Acc() is undefined.
//
// The function panics if z < 0. The value of z is undefined in that
// case.
func (z *Float) Sqrt(x *Float) *Float {
	if debugFloat {
		x.validate()
	}

	if z.prec == 0 {
		z.prec = x.prec
	}

	if x.Sign() == -1 {
		// following IEEE754-2008 (section 7.2)
		panic(ErrNaN{"square root of negative operand"})
	}

	// handle ±0 and +∞
	if x.form != finite {
		z.acc = Exact
		z.form = x.form
		z.neg = x.neg // IEEE754-2008 requires √±0 = ±0
		return z
	}

	// MantExp sets the argument's precision to the receiver's, and
	// when z.prec > x.prec this will lower z.prec. Restore it after
	// the MantExp call.
	prec := z.prec
	b := x.MantExp(z)
	z.prec = prec

	// Compute √(z·2**b) as
	//   √( z)·2**(½b)     if b is even
	//   √(2z)·2**(⌊½b⌋)   if b > 0 is odd
	//   √(½z)·2**(⌈½b⌉)   if b < 0 is odd
	switch b % 2 {
	case 0:
		// nothing to do
	case 1:
		z.exp++
	case -1:
		z.exp--
	}
	// 0.25 <= z < 2.0

	// Solving 1/x² - z = 0 avoids Quo calls and is faster, especially
	// for high precisions.
	z.sqrtInverse(z)

	// re-attach halved exponent
	return z.SetMantExp(z, b/2)
}

// Compute √x (to z.prec precision) by solving
//
//	1/t² - x = 0
//
// for t (using Newton's method), and then inverting.
func (z *Float) sqrtInverse(x *Float) {
	// let
	//   f(t) = 1/t² - x
	// then
	//   g(t) = f(t)/f'(t) = -½t(1 - xt²)
	// and the next guess is given by
	//   t2 = t - g(t) = ½t(3 - xt²)
	u := newFloat(z.prec)
	v := newFloat(z.prec)
	three := three()
	ng := func(t *Float) *Float {
		u.prec = t.prec
		v.prec = t.prec
		u.Mul(t, t)     // u = t²
		u.Mul(x, u)     //   = xt²
		v.Sub(three, u) // v = 3 - xt²
		u.Mul(t, v)     // u = t(3 - xt²)
		u.exp--         //   = ½t(3 - xt²)
		return t.Set(u)
	}

	xf, _ := x.Float64()
	sqi := newFloat(z.prec)
	sqi.SetFloat64(1 / math.Sqrt(xf))
	for prec := z.prec + 32; sqi.prec < prec; {
		sqi.prec *= 2
		sqi = ng(sqi)
	}
	// sqi = 1/√x

	// x/√x = √x
	z.Mul(x, sqi)
}

// newFloat returns a new *Float with space for twice the given
// precision.
func newFloat(prec2 uint32) *Float {
	z := new(Float)
	// nat.make ensures the slice length is > 0
	z.mant = z.mant.make(int(prec2/_W) * 2)
	return z
}

```

// === FILE: references!/go/src/math/bits/bits.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run make_tables.go

// Package bits implements bit counting and manipulation
// functions for the predeclared unsigned integer types.
//
// Functions in this package may be implemented directly by
// the compiler, for better performance. For those functions
// the code in this package will not be used. Which
// functions are implemented by the compiler depends on the
// architecture and the Go release.
package bits

const uintSize = 32 << (^uint(0) >> 63) // 32 or 64

// UintSize is the size of a uint in bits.
const UintSize = uintSize

// --- LeadingZeros ---

// LeadingZeros returns the number of leading zero bits in x; the result is [UintSize] for x == 0.
func LeadingZeros(x uint) int { return UintSize - Len(x) }

// LeadingZeros8 returns the number of leading zero bits in x; the result is 8 for x == 0.
func LeadingZeros8(x uint8) int { return 8 - Len8(x) }

// LeadingZeros16 returns the number of leading zero bits in x; the result is 16 for x == 0.
func LeadingZeros16(x uint16) int { return 16 - Len16(x) }

// LeadingZeros32 returns the number of leading zero bits in x; the result is 32 for x == 0.
func LeadingZeros32(x uint32) int { return 32 - Len32(x) }

// LeadingZeros64 returns the number of leading zero bits in x; the result is 64 for x == 0.
func LeadingZeros64(x uint64) int { return 64 - Len64(x) }

// --- TrailingZeros ---

// See http://keithandkatie.com/keith/papers/debruijn.html
const deBruijn32 = 0x077CB531

var deBruijn32tab = [32]byte{
	0, 1, 28, 2, 29, 14, 24, 3, 30, 22, 20, 15, 25, 17, 4, 8,
	31, 27, 13, 23, 21, 19, 16, 7, 26, 12, 18, 6, 11, 5, 10, 9,
}

const deBruijn64 = 0x03f79d71b4ca8b09

var deBruijn64tab = [64]byte{
	0, 1, 56, 2, 57, 49, 28, 3, 61, 58, 42, 50, 38, 29, 17, 4,
	62, 47, 59, 36, 45, 43, 51, 22, 53, 39, 33, 30, 24, 18, 12, 5,
	63, 55, 48, 27, 60, 41, 37, 16, 46, 35, 44, 21, 52, 32, 23, 11,
	54, 26, 40, 15, 34, 20, 31, 10, 25, 14, 19, 9, 13, 8, 7, 6,
}

// TrailingZeros returns the number of trailing zero bits in x; the result is [UintSize] for x == 0.
func TrailingZeros(x uint) int {
	if UintSize == 32 {
		return TrailingZeros32(uint32(x))
	}
	return TrailingZeros64(uint64(x))
}

// TrailingZeros8 returns the number of trailing zero bits in x; the result is 8 for x == 0.
func TrailingZeros8(x uint8) int {
	return int(ntz8tab[x])
}

// TrailingZeros16 returns the number of trailing zero bits in x; the result is 16 for x == 0.
func TrailingZeros16(x uint16) int {
	if x == 0 {
		return 16
	}
	// see comment in TrailingZeros64
	return int(deBruijn32tab[uint32(x&-x)*deBruijn32>>(32-5)])
}

// TrailingZeros32 returns the number of trailing zero bits in x; the result is 32 for x == 0.
func TrailingZeros32(x uint32) int {
	if x == 0 {
		return 32
	}
	// see comment in TrailingZeros64
	return int(deBruijn32tab[(x&-x)*deBruijn32>>(32-5)])
}

// TrailingZeros64 returns the number of trailing zero bits in x; the result is 64 for x == 0.
func TrailingZeros64(x uint64) int {
	if x == 0 {
		return 64
	}
	// If popcount is fast, replace code below with return popcount(^x & (x - 1)).
	//
	// x & -x leaves only the right-most bit set in the word. Let k be the
	// index of that bit. Since only a single bit is set, the value is two
	// to the power of k. Multiplying by a power of two is equivalent to
	// left shifting, in this case by k bits. The de Bruijn (64 bit) constant
	// is such that all six bit, consecutive substrings are distinct.
	// Therefore, if we have a left shifted version of this constant we can
	// find by how many bits it was shifted by looking at which six bit
	// substring ended up at the top of the word.
	// (Knuth, volume 4, section 7.3.1)
	return int(deBruijn64tab[(x&-x)*deBruijn64>>(64-6)])
}

// --- OnesCount ---

const m0 = 0x5555555555555555 // 01010101 ...
const m1 = 0x3333333333333333 // 00110011 ...
const m2 = 0x0f0f0f0f0f0f0f0f // 00001111 ...
const m3 = 0x00ff00ff00ff00ff // etc.
const m4 = 0x0000ffff0000ffff

// OnesCount returns the number of one bits ("population count") in x.
func OnesCount(x uint) int {
	if UintSize == 32 {
		return OnesCount32(uint32(x))
	}
	return OnesCount64(uint64(x))
}

// OnesCount8 returns the number of one bits ("population count") in x.
func OnesCount8(x uint8) int {
	return int(pop8tab[x])
}

// OnesCount16 returns the number of one bits ("population count") in x.
func OnesCount16(x uint16) int {
	return int(pop8tab[x>>8] + pop8tab[x&0xff])
}

// OnesCount32 returns the number of one bits ("population count") in x.
func OnesCount32(x uint32) int {
	return int(pop8tab[x>>24] + pop8tab[x>>16&0xff] + pop8tab[x>>8&0xff] + pop8tab[x&0xff])
}

// OnesCount64 returns the number of one bits ("population count") in x.
func OnesCount64(x uint64) int {
	// Implementation: Parallel summing of adjacent bits.
	// See "Hacker's Delight", Chap. 5: Counting Bits.
	// The following pattern shows the general approach:
	//
	//   x = x>>1&(m0&m) + x&(m0&m)
	//   x = x>>2&(m1&m) + x&(m1&m)
	//   x = x>>4&(m2&m) + x&(m2&m)
	//   x = x>>8&(m3&m) + x&(m3&m)
	//   x = x>>16&(m4&m) + x&(m4&m)
	//   x = x>>32&(m5&m) + x&(m5&m)
	//   return int(x)
	//
	// Masking (& operations) can be left away when there's no
	// danger that a field's sum will carry over into the next
	// field: Since the result cannot be > 64, 8 bits is enough
	// and we can ignore the masks for the shifts by 8 and up.
	// Per "Hacker's Delight", the first line can be simplified
	// more, but it saves at best one instruction, so we leave
	// it alone for clarity.
	const m = 1<<64 - 1
	x = x>>1&(m0&m) + x&(m0&m)
	x = x>>2&(m1&m) + x&(m1&m)
	x = (x>>4 + x) & (m2 & m)
	x += x >> 8
	x += x >> 16
	x += x >> 32
	return int(x) & (1<<7 - 1)
}

// --- RotateLeft ---

// RotateLeft returns the value of x rotated left by (k mod [UintSize]) bits.
// To rotate x right by k bits, call RotateLeft(x, -k).
//
// This function's execution time does not depend on the inputs.
func RotateLeft(x uint, k int) uint {
	if UintSize == 32 {
		return uint(RotateLeft32(uint32(x), k))
	}
	return uint(RotateLeft64(uint64(x), k))
}

// RotateLeft8 returns the value of x rotated left by (k mod 8) bits.
// To rotate x right by k bits, call RotateLeft8(x, -k).
//
// This function's execution time does not depend on the inputs.
func RotateLeft8(x uint8, k int) uint8 {
	const n = 8
	s := uint(k) & (n - 1)
	return x<<s | x>>(n-s)
}

// RotateLeft16 returns the value of x rotated left by (k mod 16) bits.
// To rotate x right by k bits, call RotateLeft16(x, -k).
//
// This function's execution time does not depend on the inputs.
func RotateLeft16(x uint16, k int) uint16 {
	const n = 16
	s := uint(k) & (n - 1)
	return x<<s | x>>(n-s)
}

// RotateLeft32 returns the value of x rotated left by (k mod 32) bits.
// To rotate x right by k bits, call RotateLeft32(x, -k).
//
// This function's execution time does not depend on the inputs.
func RotateLeft32(x uint32, k int) uint32 {
	const n = 32
	s := uint(k) & (n - 1)
	return x<<s | x>>(n-s)
}

// RotateLeft64 returns the value of x rotated left by (k mod 64) bits.
// To rotate x right by k bits, call RotateLeft64(x, -k).
//
// This function's execution time does not depend on the inputs.
func RotateLeft64(x uint64, k int) uint64 {
	const n = 64
	s := uint(k) & (n - 1)
	return x<<s | x>>(n-s)
}

// --- Reverse ---

// Reverse returns the value of x with its bits in reversed order.
func Reverse(x uint) uint {
	if UintSize == 32 {
		return uint(Reverse32(uint32(x)))
	}
	return uint(Reverse64(uint64(x)))
}

// Reverse8 returns the value of x with its bits in reversed order.
func Reverse8(x uint8) uint8 {
	return rev8tab[x]
}

// Reverse16 returns the value of x with its bits in reversed order.
func Reverse16(x uint16) uint16 {
	return uint16(rev8tab[x>>8]) | uint16(rev8tab[x&0xff])<<8
}

// Reverse32 returns the value of x with its bits in reversed order.
func Reverse32(x uint32) uint32 {
	const m = 1<<32 - 1
	x = x>>1&(m0&m) | x&(m0&m)<<1
	x = x>>2&(m1&m) | x&(m1&m)<<2
	x = x>>4&(m2&m) | x&(m2&m)<<4
	return ReverseBytes32(x)
}

// Reverse64 returns the value of x with its bits in reversed order.
func Reverse64(x uint64) uint64 {
	const m = 1<<64 - 1
	x = x>>1&(m0&m) | x&(m0&m)<<1
	x = x>>2&(m1&m) | x&(m1&m)<<2
	x = x>>4&(m2&m) | x&(m2&m)<<4
	return ReverseBytes64(x)
}

// --- ReverseBytes ---

// ReverseBytes returns the value of x with its bytes in reversed order.
//
// This function's execution time does not depend on the inputs.
func ReverseBytes(x uint) uint {
	if UintSize == 32 {
		return uint(ReverseBytes32(uint32(x)))
	}
	return uint(ReverseBytes64(uint64(x)))
}

// ReverseBytes16 returns the value of x with its bytes in reversed order.
//
// This function's execution time does not depend on the inputs.
func ReverseBytes16(x uint16) uint16 {
	return x>>8 | x<<8
}

// ReverseBytes32 returns the value of x with its bytes in reversed order.
//
// This function's execution time does not depend on the inputs.
func ReverseBytes32(x uint32) uint32 {
	const m = 1<<32 - 1
	x = x>>8&(m3&m) | x&(m3&m)<<8
	return x>>16 | x<<16
}

// ReverseBytes64 returns the value of x with its bytes in reversed order.
//
// This function's execution time does not depend on the inputs.
func ReverseBytes64(x uint64) uint64 {
	const m = 1<<64 - 1
	x = x>>8&(m3&m) | x&(m3&m)<<8
	x = x>>16&(m4&m) | x&(m4&m)<<16
	return x>>32 | x<<32
}

// --- Len ---

// Len returns the minimum number of bits required to represent x; the result is 0 for x == 0.
func Len(x uint) int {
	if UintSize == 32 {
		return Len32(uint32(x))
	}
	return Len64(uint64(x))
}

// Len8 returns the minimum number of bits required to represent x; the result is 0 for x == 0.
func Len8(x uint8) int {
	return int(len8tab[x])
}

// Len16 returns the minimum number of bits required to represent x; the result is 0 for x == 0.
func Len16(x uint16) (n int) {
	if x >= 1<<8 {
		x >>= 8
		n = 8
	}
	return n + int(len8tab[uint8(x)])
}

// Len32 returns the minimum number of bits required to represent x; the result is 0 for x == 0.
func Len32(x uint32) (n int) {
	if x >= 1<<16 {
		x >>= 16
		n = 16
	}
	if x >= 1<<8 {
		x >>= 8
		n += 8
	}
	return n + int(len8tab[uint8(x)])
}

// Len64 returns the minimum number of bits required to represent x; the result is 0 for x == 0.
func Len64(x uint64) (n int) {
	if x >= 1<<32 {
		x >>= 32
		n = 32
	}
	if x >= 1<<16 {
		x >>= 16
		n += 16
	}
	if x >= 1<<8 {
		x >>= 8
		n += 8
	}
	return n + int(len8tab[uint8(x)])
}

// --- Add with carry ---

// Add returns the sum with carry of x, y and carry: sum = x + y + carry.
// The carry input must be 0 or 1; otherwise the behavior is undefined.
// The carryOut output is guaranteed to be 0 or 1.
//
// This function's execution time does not depend on the inputs.
func Add(x, y, carry uint) (sum, carryOut uint) {
	if UintSize == 32 {
		s32, c32 := Add32(uint32(x), uint32(y), uint32(carry))
		return uint(s32), uint(c32)
	}
	s64, c64 := Add64(uint64(x), uint64(y), uint64(carry))
	return uint(s64), uint(c64)
}

// Add32 returns the sum with carry of x, y and carry: sum = x + y + carry.
// The carry input must be 0 or 1; otherwise the behavior is undefined.
// The carryOut output is guaranteed to be 0 or 1.
//
// This function's execution time does not depend on the inputs.
func Add32(x, y, carry uint32) (sum, carryOut uint32) {
	sum64 := uint64(x) + uint64(y) + uint64(carry)
	sum = uint32(sum64)
	carryOut = uint32(sum64 >> 32)
	return
}

// Add64 returns the sum with carry of x, y and carry: sum = x + y + carry.
// The carry input must be 0 or 1; otherwise the behavior is undefined.
// The carryOut output is guaranteed to be 0 or 1.
//
// This function's execution time does not depend on the inputs.
func Add64(x, y, carry uint64) (sum, carryOut uint64) {
	sum = x + y + carry
	// The sum will overflow if both top bits are set (x & y) or if one of them
	// is (x | y), and a carry from the lower place happened. If such a carry
	// happens, the top bit will be 1 + 0 + 1 = 0 (&^ sum).
	carryOut = ((x & y) | ((x | y) &^ sum)) >> 63
	return
}

// --- Subtract with borrow ---

// Sub returns the difference of x, y and borrow: diff = x - y - borrow.
// The borrow input must be 0 or 1; otherwise the behavior is undefined.
// The borrowOut output is guaranteed to be 0 or 1.
//
// This function's execution time does not depend on the inputs.
func Sub(x, y, borrow uint) (diff, borrowOut uint) {
	if UintSize == 32 {
		d32, b32 := Sub32(uint32(x), uint32(y), uint32(borrow))
		return uint(d32), uint(b32)
	}
	d64, b64 := Sub64(uint64(x), uint64(y), uint64(borrow))
	return uint(d64), uint(b64)
}

// Sub32 returns the difference of x, y and borrow, diff = x - y - borrow.
// The borrow input must be 0 or 1; otherwise the behavior is undefined.
// The borrowOut output is guaranteed to be 0 or 1.
//
// This function's execution time does not depend on the inputs.
func Sub32(x, y, borrow uint32) (diff, borrowOut uint32) {
	diff = x - y - borrow
	// The difference will underflow if the top bit of x is not set and the top
	// bit of y is set (^x & y) or if they are the same (^(x ^ y)) and a borrow
	// from the lower place happens. If that borrow happens, the result will be
	// 1 - 1 - 1 = 0 - 0 - 1 = 1 (& diff).
	borrowOut = ((^x & y) | (^(x ^ y) & diff)) >> 31
	return
}

// Sub64 returns the difference of x, y and borrow: diff = x - y - borrow.
// The borrow input must be 0 or 1; otherwise the behavior is undefined.
// The borrowOut output is guaranteed to be 0 or 1.
//
// This function's execution time does not depend on the inputs.
func Sub64(x, y, borrow uint64) (diff, borrowOut uint64) {
	diff = x - y - borrow
	// See Sub32 for the bit logic.
	borrowOut = ((^x & y) | (^(x ^ y) & diff)) >> 63
	return
}

// --- Full-width multiply ---

// Mul returns the full-width product of x and y: (hi, lo) = x * y
// with the product bits' upper half returned in hi and the lower
// half returned in lo.
//
// This function's execution time does not depend on the inputs.
func Mul(x, y uint) (hi, lo uint) {
	if UintSize == 32 {
		h, l := Mul32(uint32(x), uint32(y))
		return uint(h), uint(l)
	}
	h, l := Mul64(uint64(x), uint64(y))
	return uint(h), uint(l)
}

// Mul32 returns the 64-bit product of x and y: (hi, lo) = x * y
// with the product bits' upper half returned in hi and the lower
// half returned in lo.
//
// This function's execution time does not depend on the inputs.
func Mul32(x, y uint32) (hi, lo uint32) {
	tmp := uint64(x) * uint64(y)
	hi, lo = uint32(tmp>>32), uint32(tmp)
	return
}

// Mul64 returns the 128-bit product of x and y: (hi, lo) = x * y
// with the product bits' upper half returned in hi and the lower
// half returned in lo.
//
// This function's execution time does not depend on the inputs.
func Mul64(x, y uint64) (hi, lo uint64) {
	const mask32 = 1<<32 - 1
	x0 := x & mask32
	x1 := x >> 32
	y0 := y & mask32
	y1 := y >> 32
	w0 := x0 * y0
	t := x1*y0 + w0>>32
	w1 := t & mask32
	w2 := t >> 32
	w1 += x0 * y1
	hi = x1*y1 + w2 + w1>>32
	lo = x * y
	return
}

// --- Full-width divide ---

// Div returns the quotient and remainder of (hi, lo) divided by y:
// quo = (hi, lo)/y, rem = (hi, lo)%y with the dividend bits' upper
// half in parameter hi and the lower half in parameter lo.
// Div panics for y == 0 (division by zero) or y <= hi (quotient overflow).
func Div(hi, lo, y uint) (quo, rem uint) {
	if UintSize == 32 {
		q, r := Div32(uint32(hi), uint32(lo), uint32(y))
		return uint(q), uint(r)
	}
	q, r := Div64(uint64(hi), uint64(lo), uint64(y))
	return uint(q), uint(r)
}

// Div32 returns the quotient and remainder of (hi, lo) divided by y:
// quo = (hi, lo)/y, rem = (hi, lo)%y with the dividend bits' upper
// half in parameter hi and the lower half in parameter lo.
// Div32 panics for y == 0 (division by zero) or y <= hi (quotient overflow).
func Div32(hi, lo, y uint32) (quo, rem uint32) {
	if y != 0 && y <= hi {
		panic(overflowError)
	}
	z := uint64(hi)<<32 | uint64(lo)
	quo, rem = uint32(z/uint64(y)), uint32(z%uint64(y))
	return
}

// Div64 returns the quotient and remainder of (hi, lo) divided by y:
// quo = (hi, lo)/y, rem = (hi, lo)%y with the dividend bits' upper
// half in parameter hi and the lower half in parameter lo.
// Div64 panics for y == 0 (division by zero) or y <= hi (quotient overflow).
func Div64(hi, lo, y uint64) (quo, rem uint64) {
	if y == 0 {
		panic(divideError)
	}
	if y <= hi {
		panic(overflowError)
	}

	// If high part is zero, we can directly return the results.
	if hi == 0 {
		return lo / y, lo % y
	}

	s := uint(LeadingZeros64(y))
	y <<= s

	const (
		two32  = 1 << 32
		mask32 = two32 - 1
	)
	yn1 := y >> 32
	yn0 := y & mask32
	un32 := hi<<s | lo>>(64-s)
	un10 := lo << s
	un1 := un10 >> 32
	un0 := un10 & mask32
	q1 := un32 / yn1
	rhat := un32 - q1*yn1

	for q1 >= two32 || q1*yn0 > two32*rhat+un1 {
		q1--
		rhat += yn1
		if rhat >= two32 {
			break
		}
	}

	un21 := un32*two32 + un1 - q1*y
	q0 := un21 / yn1
	rhat = un21 - q0*yn1

	for q0 >= two32 || q0*yn0 > two32*rhat+un0 {
		q0--
		rhat += yn1
		if rhat >= two32 {
			break
		}
	}

	return q1*two32 + q0, (un21*two32 + un0 - q0*y) >> s
}

// Rem returns the remainder of (hi, lo) divided by y. Rem panics for
// y == 0 (division by zero) but, unlike Div, it doesn't panic on a
// quotient overflow.
func Rem(hi, lo, y uint) uint {
	if UintSize == 32 {
		return uint(Rem32(uint32(hi), uint32(lo), uint32(y)))
	}
	return uint(Rem64(uint64(hi), uint64(lo), uint64(y)))
}

// Rem32 returns the remainder of (hi, lo) divided by y. Rem32 panics
// for y == 0 (division by zero) but, unlike [Div32], it doesn't panic
// on a quotient overflow.
func Rem32(hi, lo, y uint32) uint32 {
	return uint32((uint64(hi)<<32 | uint64(lo)) % uint64(y))
}

// Rem64 returns the remainder of (hi, lo) divided by y. Rem64 panics
// for y == 0 (division by zero) but, unlike [Div64], it doesn't panic
// on a quotient overflow.
func Rem64(hi, lo, y uint64) uint64 {
	// We scale down hi so that hi < y, then use Div64 to compute the
	// rem with the guarantee that it won't panic on quotient overflow.
	// Given that
	//   hi ≡ hi%y    (mod y)
	// we have
	//   hi<<64 + lo ≡ (hi%y)<<64 + lo    (mod y)
	_, rem := Div64(hi%y, lo, y)
	return rem
}

```

// === FILE: references!/go/src/math/bits/bits_errors.go ===
```go
// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package bits

import _ "unsafe"

//go:linkname overflowError runtime.overflowError
var overflowError error

//go:linkname divideError runtime.divideError
var divideError error

```

// === FILE: references!/go/src/math/bits/bits_errors_bootstrap.go ===
```go
// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build compiler_bootstrap

// This version used only for bootstrap (on this path we want
// to avoid use of go:linkname as applied to variables).

package bits

type errorString string

func (e errorString) RuntimeError() {}

func (e errorString) Error() string {
	return "runtime error: " + string(e)
}

var overflowError = error(errorString("integer overflow"))

var divideError = error(errorString("integer divide by zero"))

```

// === FILE: references!/go/src/math/bits/bits_tables.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by go run make_tables.go. DO NOT EDIT.

package bits

const ntz8tab = "" +
	"\x08\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00" +
	"\x04\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00" +
	"\x05\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00" +
	"\x04\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00" +
	"\x06\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00" +
	"\x04\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00" +
	"\x05\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00" +
	"\x04\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00" +
	"\x07\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00" +
	"\x04\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00" +
	"\x05\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00" +
	"\x04\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00" +
	"\x06\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00" +
	"\x04\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00" +
	"\x05\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00" +
	"\x04\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00"

const pop8tab = "" +
	"\x00\x01\x01\x02\x01\x02\x02\x03\x01\x02\x02\x03\x02\x03\x03\x04" +
	"\x01\x02\x02\x03\x02\x03\x03\x04\x02\x03\x03\x04\x03\x04\x04\x05" +
	"\x01\x02\x02\x03\x02\x03\x03\x04\x02\x03\x03\x04\x03\x04\x04\x05" +
	"\x02\x03\x03\x04\x03\x04\x04\x05\x03\x04\x04\x05\x04\x05\x05\x06" +
	"\x01\x02\x02\x03\x02\x03\x03\x04\x02\x03\x03\x04\x03\x04\x04\x05" +
	"\x02\x03\x03\x04\x03\x04\x04\x05\x03\x04\x04\x05\x04\x05\x05\x06" +
	"\x02\x03\x03\x04\x03\x04\x04\x05\x03\x04\x04\x05\x04\x05\x05\x06" +
	"\x03\x04\x04\x05\x04\x05\x05\x06\x04\x05\x05\x06\x05\x06\x06\x07" +
	"\x01\x02\x02\x03\x02\x03\x03\x04\x02\x03\x03\x04\x03\x04\x04\x05" +
	"\x02\x03\x03\x04\x03\x04\x04\x05\x03\x04\x04\x05\x04\x05\x05\x06" +
	"\x02\x03\x03\x04\x03\x04\x04\x05\x03\x04\x04\x05\x04\x05\x05\x06" +
	"\x03\x04\x04\x05\x04\x05\x05\x06\x04\x05\x05\x06\x05\x06\x06\x07" +
	"\x02\x03\x03\x04\x03\x04\x04\x05\x03\x04\x04\x05\x04\x05\x05\x06" +
	"\x03\x04\x04\x05\x04\x05\x05\x06\x04\x05\x05\x06\x05\x06\x06\x07" +
	"\x03\x04\x04\x05\x04\x05\x05\x06\x04\x05\x05\x06\x05\x06\x06\x07" +
	"\x04\x05\x05\x06\x05\x06\x06\x07\x05\x06\x06\x07\x06\x07\x07\x08"

const rev8tab = "" +
	"\x00\x80\x40\xc0\x20\xa0\x60\xe0\x10\x90\x50\xd0\x30\xb0\x70\xf0" +
	"\x08\x88\x48\xc8\x28\xa8\x68\xe8\x18\x98\x58\xd8\x38\xb8\x78\xf8" +
	"\x04\x84\x44\xc4\x24\xa4\x64\xe4\x14\x94\x54\xd4\x34\xb4\x74\xf4" +
	"\x0c\x8c\x4c\xcc\x2c\xac\x6c\xec\x1c\x9c\x5c\xdc\x3c\xbc\x7c\xfc" +
	"\x02\x82\x42\xc2\x22\xa2\x62\xe2\x12\x92\x52\xd2\x32\xb2\x72\xf2" +
	"\x0a\x8a\x4a\xca\x2a\xaa\x6a\xea\x1a\x9a\x5a\xda\x3a\xba\x7a\xfa" +
	"\x06\x86\x46\xc6\x26\xa6\x66\xe6\x16\x96\x56\xd6\x36\xb6\x76\xf6" +
	"\x0e\x8e\x4e\xce\x2e\xae\x6e\xee\x1e\x9e\x5e\xde\x3e\xbe\x7e\xfe" +
	"\x01\x81\x41\xc1\x21\xa1\x61\xe1\x11\x91\x51\xd1\x31\xb1\x71\xf1" +
	"\x09\x89\x49\xc9\x29\xa9\x69\xe9\x19\x99\x59\xd9\x39\xb9\x79\xf9" +
	"\x05\x85\x45\xc5\x25\xa5\x65\xe5\x15\x95\x55\xd5\x35\xb5\x75\xf5" +
	"\x0d\x8d\x4d\xcd\x2d\xad\x6d\xed\x1d\x9d\x5d\xdd\x3d\xbd\x7d\xfd" +
	"\x03\x83\x43\xc3\x23\xa3\x63\xe3\x13\x93\x53\xd3\x33\xb3\x73\xf3" +
	"\x0b\x8b\x4b\xcb\x2b\xab\x6b\xeb\x1b\x9b\x5b\xdb\x3b\xbb\x7b\xfb" +
	"\x07\x87\x47\xc7\x27\xa7\x67\xe7\x17\x97\x57\xd7\x37\xb7\x77\xf7" +
	"\x0f\x8f\x4f\xcf\x2f\xaf\x6f\xef\x1f\x9f\x5f\xdf\x3f\xbf\x7f\xff"

const len8tab = "" +
	"\x00\x01\x02\x02\x03\x03\x03\x03\x04\x04\x04\x04\x04\x04\x04\x04" +
	"\x05\x05\x05\x05\x05\x05\x05\x05\x05\x05\x05\x05\x05\x05\x05\x05" +
	"\x06\x06\x06\x06\x06\x06\x06\x06\x06\x06\x06\x06\x06\x06\x06\x06" +
	"\x06\x06\x06\x06\x06\x06\x06\x06\x06\x06\x06\x06\x06\x06\x06\x06" +
	"\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07" +
	"\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07" +
	"\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07" +
	"\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07\x07" +
	"\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08" +
	"\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08" +
	"\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08" +
	"\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08" +
	"\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08" +
	"\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08" +
	"\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08" +
	"\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08\x08"

```

// === FILE: references!/go/src/math/bits/make_examples.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

// This program generates example_test.go.

package main

import (
	"bytes"
	"fmt"
	"log"
	"math/bits"
	"os"
)

const header = `// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by go run make_examples.go. DO NOT EDIT.

package bits_test

import (
	"fmt"
	"math/bits"
)
`

func main() {
	w := bytes.NewBuffer([]byte(header))

	for _, e := range []struct {
		name string
		in   int
		out  [4]any
		out2 [4]any
	}{
		{
			name: "LeadingZeros",
			in:   1,
			out:  [4]any{bits.LeadingZeros8(1), bits.LeadingZeros16(1), bits.LeadingZeros32(1), bits.LeadingZeros64(1)},
		},
		{
			name: "TrailingZeros",
			in:   14,
			out:  [4]any{bits.TrailingZeros8(14), bits.TrailingZeros16(14), bits.TrailingZeros32(14), bits.TrailingZeros64(14)},
		},
		{
			name: "OnesCount",
			in:   14,
			out:  [4]any{bits.OnesCount8(14), bits.OnesCount16(14), bits.OnesCount32(14), bits.OnesCount64(14)},
		},
		{
			name: "RotateLeft",
			in:   15,
			out:  [4]any{bits.RotateLeft8(15, 2), bits.RotateLeft16(15, 2), bits.RotateLeft32(15, 2), bits.RotateLeft64(15, 2)},
			out2: [4]any{bits.RotateLeft8(15, -2), bits.RotateLeft16(15, -2), bits.RotateLeft32(15, -2), bits.RotateLeft64(15, -2)},
		},
		{
			name: "Reverse",
			in:   19,
			out:  [4]any{bits.Reverse8(19), bits.Reverse16(19), bits.Reverse32(19), bits.Reverse64(19)},
		},
		{
			name: "ReverseBytes",
			in:   15,
			out:  [4]any{nil, bits.ReverseBytes16(15), bits.ReverseBytes32(15), bits.ReverseBytes64(15)},
		},
		{
			name: "Len",
			in:   8,
			out:  [4]any{bits.Len8(8), bits.Len16(8), bits.Len32(8), bits.Len64(8)},
		},
	} {
		for i, size := range []int{8, 16, 32, 64} {
			if e.out[i] == nil {
				continue // function doesn't exist
			}
			f := fmt.Sprintf("%s%d", e.name, size)
			fmt.Fprintf(w, "\nfunc Example%s() {\n", f)
			switch e.name {
			case "RotateLeft", "Reverse", "ReverseBytes":
				fmt.Fprintf(w, "\tfmt.Printf(\"%%0%db\\n\", %d)\n", size, e.in)
				if e.name == "RotateLeft" {
					fmt.Fprintf(w, "\tfmt.Printf(\"%%0%db\\n\", bits.%s(%d, 2))\n", size, f, e.in)
					fmt.Fprintf(w, "\tfmt.Printf(\"%%0%db\\n\", bits.%s(%d, -2))\n", size, f, e.in)
				} else {
					fmt.Fprintf(w, "\tfmt.Printf(\"%%0%db\\n\", bits.%s(%d))\n", size, f, e.in)
				}
				fmt.Fprintf(w, "\t// Output:\n")
				fmt.Fprintf(w, "\t// %0*b\n", size, e.in)
				fmt.Fprintf(w, "\t// %0*b\n", size, e.out[i])
				if e.name == "RotateLeft" && e.out2[i] != nil {
					fmt.Fprintf(w, "\t// %0*b\n", size, e.out2[i])
				}
			default:
				fmt.Fprintf(w, "\tfmt.Printf(\"%s(%%0%db) = %%d\\n\", %d, bits.%s(%d))\n", f, size, e.in, f, e.in)
				fmt.Fprintf(w, "\t// Output:\n")
				fmt.Fprintf(w, "\t// %s(%0*b) = %d\n", f, size, e.in, e.out[i])
			}
			fmt.Fprintf(w, "}\n")
		}
	}

	if err := os.WriteFile("example_test.go", w.Bytes(), 0666); err != nil {
		log.Fatal(err)
	}
}

```

// === FILE: references!/go/src/math/bits/make_tables.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

// This program generates bits_tables.go.

package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io"
	"log"
	"os"
)

var header = []byte(`// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by go run make_tables.go. DO NOT EDIT.

package bits

`)

func main() {
	buf := bytes.NewBuffer(header)

	gen(buf, "ntz8tab", ntz8)
	gen(buf, "pop8tab", pop8)
	gen(buf, "rev8tab", rev8)
	gen(buf, "len8tab", len8)

	out, err := format.Source(buf.Bytes())
	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile("bits_tables.go", out, 0666)
	if err != nil {
		log.Fatal(err)
	}
}

func gen(w io.Writer, name string, f func(uint8) uint8) {
	// Use a const string to allow the compiler to constant-evaluate lookups at constant index.
	fmt.Fprintf(w, "const %s = \"\"+\n\"", name)
	for i := 0; i < 256; i++ {
		fmt.Fprintf(w, "\\x%02x", f(uint8(i)))
		if i%16 == 15 && i != 255 {
			fmt.Fprint(w, "\"+\n\"")
		}
	}
	fmt.Fprint(w, "\"\n\n")
}

func ntz8(x uint8) (n uint8) {
	for x&1 == 0 && n < 8 {
		x >>= 1
		n++
	}
	return
}

func pop8(x uint8) (n uint8) {
	for x != 0 {
		x &= x - 1
		n++
	}
	return
}

func rev8(x uint8) (r uint8) {
	for i := 8; i > 0; i-- {
		r = r<<1 | x&1
		x >>= 1
	}
	return
}

func len8(x uint8) (n uint8) {
	for x != 0 {
		x >>= 1
		n++
	}
	return
}

```

// === FILE: references!/go/src/math/bits.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

const (
	uvnan    = 0x7FF8000000000001
	uvinf    = 0x7FF0000000000000
	uvneginf = 0xFFF0000000000000
	uvone    = 0x3FF0000000000000
	mask     = 0x7FF
	shift    = 64 - 11 - 1
	bias     = 1023
	signMask = 1 << 63
	fracMask = 1<<shift - 1
)

// Inf returns positive infinity if sign >= 0, negative infinity if sign < 0.
func Inf(sign int) float64 {
	var v uint64
	if sign >= 0 {
		v = uvinf
	} else {
		v = uvneginf
	}
	return Float64frombits(v)
}

// NaN returns an IEEE 754 “not-a-number” value.
func NaN() float64 { return Float64frombits(uvnan) }

// IsNaN reports whether f is an IEEE 754 “not-a-number” value.
func IsNaN(f float64) (is bool) {
	// IEEE 754 says that only NaNs satisfy f != f.
	// To avoid the floating-point hardware, could use:
	//	x := Float64bits(f);
	//	return uint32(x>>shift)&mask == mask && x != uvinf && x != uvneginf
	return f != f
}

// IsInf reports whether f is an infinity, according to sign.
// If sign > 0, IsInf reports whether f is positive infinity.
// If sign < 0, IsInf reports whether f is negative infinity.
// If sign == 0, IsInf reports whether f is either infinity.
func IsInf(f float64, sign int) bool {
	// Test for infinity by comparing against maximum float.
	// To avoid the floating-point hardware, could use:
	//	x := Float64bits(f);
	//	return sign >= 0 && x == uvinf || sign <= 0 && x == uvneginf;
	if sign == 0 {
		f = Abs(f)
	} else if sign < 0 {
		f = -f
	}
	return f > MaxFloat64
}

// normalize returns a normal number y and exponent exp
// satisfying x == y × 2**exp. It assumes x is finite and non-zero.
func normalize(x float64) (y float64, exp int) {
	const SmallestNormal = 2.2250738585072014e-308 // 2**-1022
	if Abs(x) < SmallestNormal {
		return x * (1 << 52), -52
	}
	return x, 0
}

```

// === FILE: references!/go/src/math/cbrt.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// The go code is a modified version of the original C code from
// http://www.netlib.org/fdlibm/s_cbrt.c and came with this notice.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunSoft, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================

// Cbrt returns the cube root of x.
//
// Special cases are:
//
//	Cbrt(±0) = ±0
//	Cbrt(±Inf) = ±Inf
//	Cbrt(NaN) = NaN
func Cbrt(x float64) float64 {
	if haveArchCbrt {
		return archCbrt(x)
	}
	return cbrt(x)
}

func cbrt(x float64) float64 {
	const (
		B1             = 715094163                   // (682-0.03306235651)*2**20
		B2             = 696219795                   // (664-0.03306235651)*2**20
		C              = 5.42857142857142815906e-01  // 19/35     = 0x3FE15F15F15F15F1
		D              = -7.05306122448979611050e-01 // -864/1225 = 0xBFE691DE2532C834
		E              = 1.41428571428571436819e+00  // 99/70     = 0x3FF6A0EA0EA0EA0F
		F              = 1.60714285714285720630e+00  // 45/28     = 0x3FF9B6DB6DB6DB6E
		G              = 3.57142857142857150787e-01  // 5/14      = 0x3FD6DB6DB6DB6DB7
		SmallestNormal = 2.22507385850720138309e-308 // 2**-1022  = 0x0010000000000000
	)
	// special cases
	switch {
	case x == 0 || IsNaN(x) || IsInf(x, 0):
		return x
	}

	sign := false
	if x < 0 {
		x = -x
		sign = true
	}

	// rough cbrt to 5 bits
	t := Float64frombits(Float64bits(x)/3 + B1<<32)
	if x < SmallestNormal {
		// subnormal number
		t = float64(1 << 54) // set t= 2**54
		t *= x
		t = Float64frombits(Float64bits(t)/3 + B2<<32)
	}

	// new cbrt to 23 bits
	r := t * t / x
	s := C + r*t
	t *= G + F/(s+E+D/s)

	// chop to 22 bits, make larger than cbrt(x)
	t = Float64frombits(Float64bits(t)&(0xFFFFFFFFC<<28) + 1<<30)

	// one step newton iteration to 53 bits with error less than 0.667ulps
	s = t * t // t*t is exact
	r = x / s
	w := t + t
	r = (r - t) / (w + r) // r-s is exact
	t = t + t*r

	// restore the sign bit
	if sign {
		t = -t
	}
	return t
}

```

// === FILE: references!/go/src/math/cbrt_s390x.s ===
```text
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// Minimax polynomial coefficients and other constants
DATA ·cbrtrodataL9<> + 0(SB)/8, $-.00016272731015974436E+00
DATA ·cbrtrodataL9<> + 8(SB)/8, $0.66639548758285293179E+00
DATA ·cbrtrodataL9<> + 16(SB)/8, $0.55519402697349815993E+00
DATA ·cbrtrodataL9<> + 24(SB)/8, $0.49338566048766782004E+00
DATA ·cbrtrodataL9<> + 32(SB)/8, $0.45208160036325611486E+00
DATA ·cbrtrodataL9<> + 40(SB)/8, $0.43099892837778637816E+00
DATA ·cbrtrodataL9<> + 48(SB)/8, $1.000244140625
DATA ·cbrtrodataL9<> + 56(SB)/8, $0.33333333333333333333E+00
DATA ·cbrtrodataL9<> + 64(SB)/8, $79228162514264337593543950336.
GLOBL ·cbrtrodataL9<> + 0(SB), RODATA, $72

// Index tables
DATA ·cbrttab32069<> + 0(SB)/8, $0x404030303020202
DATA ·cbrttab32069<> + 8(SB)/8, $0x101010101000000
DATA ·cbrttab32069<> + 16(SB)/8, $0x808070706060605
DATA ·cbrttab32069<> + 24(SB)/8, $0x505040404040303
DATA ·cbrttab32069<> + 32(SB)/8, $0xe0d0c0c0b0b0b0a
DATA ·cbrttab32069<> + 40(SB)/8, $0xa09090908080808
DATA ·cbrttab32069<> + 48(SB)/8, $0x11111010100f0f0f
DATA ·cbrttab32069<> + 56(SB)/8, $0xe0e0e0e0e0d0d0d
DATA ·cbrttab32069<> + 64(SB)/8, $0x1515141413131312
DATA ·cbrttab32069<> + 72(SB)/8, $0x1212111111111010
GLOBL ·cbrttab32069<> + 0(SB), RODATA, $80

DATA ·cbrttab22068<> + 0(SB)/8, $0x151015001420141
DATA ·cbrttab22068<> + 8(SB)/8, $0x140013201310130
DATA ·cbrttab22068<> + 16(SB)/8, $0x122012101200112
DATA ·cbrttab22068<> + 24(SB)/8, $0x111011001020101
DATA ·cbrttab22068<> + 32(SB)/8, $0x10000f200f100f0
DATA ·cbrttab22068<> + 40(SB)/8, $0xe200e100e000d2
DATA ·cbrttab22068<> + 48(SB)/8, $0xd100d000c200c1
DATA ·cbrttab22068<> + 56(SB)/8, $0xc000b200b100b0
DATA ·cbrttab22068<> + 64(SB)/8, $0xa200a100a00092
DATA ·cbrttab22068<> + 72(SB)/8, $0x91009000820081
DATA ·cbrttab22068<> + 80(SB)/8, $0x80007200710070
DATA ·cbrttab22068<> + 88(SB)/8, $0x62006100600052
DATA ·cbrttab22068<> + 96(SB)/8, $0x51005000420041
DATA ·cbrttab22068<> + 104(SB)/8, $0x40003200310030
DATA ·cbrttab22068<> + 112(SB)/8, $0x22002100200012
DATA ·cbrttab22068<> + 120(SB)/8, $0x11001000020001
GLOBL ·cbrttab22068<> + 0(SB), RODATA, $128

DATA ·cbrttab12067<> + 0(SB)/8, $0x53e1529051324fe1
DATA ·cbrttab12067<> + 8(SB)/8, $0x4e904d324be14a90
DATA ·cbrttab12067<> + 16(SB)/8, $0x493247e146904532
DATA ·cbrttab12067<> + 24(SB)/8, $0x43e1429041323fe1
DATA ·cbrttab12067<> + 32(SB)/8, $0x3e903d323be13a90
DATA ·cbrttab12067<> + 40(SB)/8, $0x393237e136903532
DATA ·cbrttab12067<> + 48(SB)/8, $0x33e1329031322fe1
DATA ·cbrttab12067<> + 56(SB)/8, $0x2e902d322be12a90
DATA ·cbrttab12067<> + 64(SB)/8, $0xd3e1d290d132cfe1
DATA ·cbrttab12067<> + 72(SB)/8, $0xce90cd32cbe1ca90
DATA ·cbrttab12067<> + 80(SB)/8, $0xc932c7e1c690c532
DATA ·cbrttab12067<> + 88(SB)/8, $0xc3e1c290c132bfe1
DATA ·cbrttab12067<> + 96(SB)/8, $0xbe90bd32bbe1ba90
DATA ·cbrttab12067<> + 104(SB)/8, $0xb932b7e1b690b532
DATA ·cbrttab12067<> + 112(SB)/8, $0xb3e1b290b132afe1
DATA ·cbrttab12067<> + 120(SB)/8, $0xae90ad32abe1aa90
GLOBL ·cbrttab12067<> + 0(SB), RODATA, $128

// Cbrt returns the cube root of the argument.
//
// Special cases are:
//      Cbrt(±0) = ±0
//      Cbrt(±Inf) = ±Inf
//      Cbrt(NaN) = NaN
// The algorithm used is minimax polynomial approximation
// with coefficients determined with a Remez exchange algorithm.

TEXT	·cbrtAsm(SB), NOSPLIT, $0-16
	FMOVD	x+0(FP), F0
	MOVD	$·cbrtrodataL9<>+0(SB), R9
	LGDR	F0, R2
	WORD	$0xC039000F	//iilf	%r3,1048575
	BYTE	$0xFF
	BYTE	$0xFF
	SRAD	$32, R2
	WORD	$0xB9170012	//llgtr	%r1,%r2
	MOVW	R1, R6
	MOVW	R3, R7
	CMPBLE	R6, R7, L2
	WORD	$0xC0397FEF	//iilf	%r3,2146435071
	BYTE	$0xFF
	BYTE	$0xFF
	MOVW	R3, R7
	CMPBLE	R6, R7, L8
L1:
	FMOVD	F0, ret+8(FP)
	RET
L3:
L2:
	LTDBR	F0, F0
	BEQ	L1
	FMOVD	F0, F2
	WORD	$0xED209040	//mdb	%f2,.L10-.L9(%r9)
	BYTE	$0x00
	BYTE	$0x1C
	MOVH	$0x200, R4
	LGDR	F2, R2
	SRAD	$32, R2
L4:
	RISBGZ	$57, $62, $39, R2, R3
	MOVD	$·cbrttab12067<>+0(SB), R1
	WORD	$0x48131000	//lh	%r1,0(%r3,%r1)
	RISBGZ	$57, $62, $45, R2, R3
	MOVD	$·cbrttab22068<>+0(SB), R5
	RISBGNZ	$60, $63, $48, R2, R2
	WORD	$0x4A135000	//ah	%r1,0(%r3,%r5)
	BYTE	$0x18	//lr	%r3,%r1
	BYTE	$0x31
	MOVD	$·cbrttab32069<>+0(SB), R1
	FMOVD	56(R9), F1
	FMOVD	48(R9), F5
	WORD	$0xEC23393B	//rosbg	%r2,%r3,57,59,4
	BYTE	$0x04
	BYTE	$0x56
	WORD	$0xE3121000	//llc	%r1,0(%r2,%r1)
	BYTE	$0x00
	BYTE	$0x94
	ADDW	R3, R1
	ADDW	R4, R1
	SLW	$16, R1, R1
	SLD	$32, R1, R1
	LDGR	R1, F2
	WFMDB	V2, V2, V4
	WFMDB	V4, V0, V6
	WFMSDB	V4, V6, V2, V4
	FMOVD	40(R9), F6
	FMSUB	F1, F4, F2
	FMOVD	32(R9), F4
	WFMDB	V2, V2, V3
	FMOVD	24(R9), F1
	FMUL	F3, F0
	FMOVD	16(R9), F3
	WFMADB	V2, V0, V5, V2
	FMOVD	8(R9), F5
	FMADD	F6, F2, F4
	WFMADB	V2, V1, V3, V1
	WFMDB	V2, V2, V6
	FMOVD	0(R9), F3
	WFMADB	V4, V6, V1, V4
	WFMADB	V2, V5, V3, V2
	FMADD	F4, F6, F2
	FMADD	F2, F0, F0
	FMOVD	F0, ret+8(FP)
	RET
L8:
	MOVH	$0x0, R4
	BR	L4

```

// === FILE: references!/go/src/math/cmplx/abs.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package cmplx provides basic constants and mathematical functions for
// complex numbers. Special case handling conforms to the C99 standard
// Annex G IEC 60559-compatible complex arithmetic.
package cmplx

import "math"

// Abs returns the absolute value (also called the modulus) of x.
func Abs(x complex128) float64 { return math.Hypot(real(x), imag(x)) }

```

// === FILE: references!/go/src/math/cmplx/asin.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmplx

import "math"

// The original C code, the long comment, and the constants
// below are from http://netlib.sandia.gov/cephes/c9x-complex/clog.c.
// The go code is a simplified version of the original C.
//
// Cephes Math Library Release 2.8:  June, 2000
// Copyright 1984, 1987, 1989, 1992, 2000 by Stephen L. Moshier
//
// The readme file at http://netlib.sandia.gov/cephes/ says:
//    Some software in this archive may be from the book _Methods and
// Programs for Mathematical Functions_ (Prentice-Hall or Simon & Schuster
// International, 1989) or from the Cephes Mathematical Library, a
// commercial product. In either event, it is copyrighted by the author.
// What you see here may be used freely but it comes with no support or
// guarantee.
//
//   The two known misprints in the book are repaired here in the
// source listings for the gamma function and the incomplete beta
// integral.
//
//   Stephen L. Moshier
//   moshier@na-net.ornl.gov

// Complex circular arc sine
//
// DESCRIPTION:
//
// Inverse complex sine:
//                               2
// w = -i clog( iz + csqrt( 1 - z ) ).
//
// casin(z) = -i casinh(iz)
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    DEC       -10,+10     10100       2.1e-15     3.4e-16
//    IEEE      -10,+10     30000       2.2e-14     2.7e-15
// Larger relative error can be observed for z near zero.
// Also tested by csin(casin(z)) = z.

// Asin returns the inverse sine of x.
func Asin(x complex128) complex128 {
	switch re, im := real(x), imag(x); {
	case im == 0 && math.Abs(re) <= 1:
		return complex(math.Asin(re), im)
	case re == 0 && math.Abs(im) <= 1:
		return complex(re, math.Asinh(im))
	case math.IsNaN(im):
		switch {
		case re == 0:
			return complex(re, math.NaN())
		case math.IsInf(re, 0):
			return complex(math.NaN(), re)
		default:
			return NaN()
		}
	case math.IsInf(im, 0):
		switch {
		case math.IsNaN(re):
			return x
		case math.IsInf(re, 0):
			return complex(math.Copysign(math.Pi/4, re), im)
		default:
			return complex(math.Copysign(0, re), im)
		}
	case math.IsInf(re, 0):
		return complex(math.Copysign(math.Pi/2, re), math.Copysign(re, im))
	}
	ct := complex(-imag(x), real(x)) // i * x
	xx := x * x
	x1 := complex(1-real(xx), -imag(xx)) // 1 - x*x
	x2 := Sqrt(x1)                       // x2 = sqrt(1 - x*x)
	w := Log(ct + x2)
	return complex(imag(w), -real(w)) // -i * w
}

// Asinh returns the inverse hyperbolic sine of x.
func Asinh(x complex128) complex128 {
	switch re, im := real(x), imag(x); {
	case im == 0 && math.Abs(re) <= 1:
		return complex(math.Asinh(re), im)
	case re == 0 && math.Abs(im) <= 1:
		return complex(re, math.Asin(im))
	case math.IsInf(re, 0):
		switch {
		case math.IsInf(im, 0):
			return complex(re, math.Copysign(math.Pi/4, im))
		case math.IsNaN(im):
			return x
		default:
			return complex(re, math.Copysign(0.0, im))
		}
	case math.IsNaN(re):
		switch {
		case im == 0:
			return x
		case math.IsInf(im, 0):
			return complex(im, re)
		default:
			return NaN()
		}
	case math.IsInf(im, 0):
		return complex(math.Copysign(im, re), math.Copysign(math.Pi/2, im))
	}
	xx := x * x
	x1 := complex(1+real(xx), imag(xx)) // 1 + x*x
	return Log(x + Sqrt(x1))            // log(x + sqrt(1 + x*x))
}

// Complex circular arc cosine
//
// DESCRIPTION:
//
// w = arccos z  =  PI/2 - arcsin z.
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    DEC       -10,+10      5200      1.6e-15      2.8e-16
//    IEEE      -10,+10     30000      1.8e-14      2.2e-15

// Acos returns the inverse cosine of x.
func Acos(x complex128) complex128 {
	w := Asin(x)
	return complex(math.Pi/2-real(w), -imag(w))
}

// Acosh returns the inverse hyperbolic cosine of x.
func Acosh(x complex128) complex128 {
	if x == 0 {
		return complex(0, math.Copysign(math.Pi/2, imag(x)))
	}
	w := Acos(x)
	if imag(w) <= 0 {
		return complex(-imag(w), real(w)) // i * w
	}
	return complex(imag(w), -real(w)) // -i * w
}

// Complex circular arc tangent
//
// DESCRIPTION:
//
// If
//     z = x + iy,
//
// then
//          1       (    2x     )
// Re w  =  - arctan(-----------)  +  k PI
//          2       (     2    2)
//                  (1 - x  - y )
//
//               ( 2         2)
//          1    (x  +  (y+1) )
// Im w  =  - log(------------)
//          4    ( 2         2)
//               (x  +  (y-1) )
//
// Where k is an arbitrary integer.
//
// catan(z) = -i catanh(iz).
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    DEC       -10,+10      5900       1.3e-16     7.8e-18
//    IEEE      -10,+10     30000       2.3e-15     8.5e-17
// The check catan( ctan(z) )  =  z, with |x| and |y| < PI/2,
// had peak relative error 1.5e-16, rms relative error
// 2.9e-17.  See also clog().

// Atan returns the inverse tangent of x.
func Atan(x complex128) complex128 {
	switch re, im := real(x), imag(x); {
	case im == 0:
		return complex(math.Atan(re), im)
	case re == 0 && math.Abs(im) <= 1:
		return complex(re, math.Atanh(im))
	case math.IsInf(im, 0) || math.IsInf(re, 0):
		if math.IsNaN(re) {
			return complex(math.NaN(), math.Copysign(0, im))
		}
		return complex(math.Copysign(math.Pi/2, re), math.Copysign(0, im))
	case math.IsNaN(re) || math.IsNaN(im):
		return NaN()
	}
	x2 := real(x) * real(x)
	a := 1 - x2 - imag(x)*imag(x)
	if a == 0 {
		return NaN()
	}
	t := 0.5 * math.Atan2(2*real(x), a)
	w := reducePi(t)

	t = imag(x) - 1
	b := x2 + t*t
	if b == 0 {
		return NaN()
	}
	t = imag(x) + 1
	c := (x2 + t*t) / b
	return complex(w, 0.25*math.Log(c))
}

// Atanh returns the inverse hyperbolic tangent of x.
func Atanh(x complex128) complex128 {
	z := complex(-imag(x), real(x)) // z = i * x
	z = Atan(z)
	return complex(imag(z), -real(z)) // z = -i * z
}

```

// === FILE: references!/go/src/math/cmplx/conj.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmplx

// Conj returns the complex conjugate of x.
func Conj(x complex128) complex128 { return complex(real(x), -imag(x)) }

```

// === FILE: references!/go/src/math/cmplx/exp.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmplx

import "math"

// The original C code, the long comment, and the constants
// below are from http://netlib.sandia.gov/cephes/c9x-complex/clog.c.
// The go code is a simplified version of the original C.
//
// Cephes Math Library Release 2.8:  June, 2000
// Copyright 1984, 1987, 1989, 1992, 2000 by Stephen L. Moshier
//
// The readme file at http://netlib.sandia.gov/cephes/ says:
//    Some software in this archive may be from the book _Methods and
// Programs for Mathematical Functions_ (Prentice-Hall or Simon & Schuster
// International, 1989) or from the Cephes Mathematical Library, a
// commercial product. In either event, it is copyrighted by the author.
// What you see here may be used freely but it comes with no support or
// guarantee.
//
//   The two known misprints in the book are repaired here in the
// source listings for the gamma function and the incomplete beta
// integral.
//
//   Stephen L. Moshier
//   moshier@na-net.ornl.gov

// Complex exponential function
//
// DESCRIPTION:
//
// Returns the complex exponential of the complex argument z.
//
// If
//     z = x + iy,
//     r = exp(x),
// then
//     w = r cos y + i r sin y.
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    DEC       -10,+10      8700       3.7e-17     1.1e-17
//    IEEE      -10,+10     30000       3.0e-16     8.7e-17

// Exp returns e**x, the base-e exponential of x.
func Exp(x complex128) complex128 {
	switch re, im := real(x), imag(x); {
	case math.IsInf(re, 0):
		switch {
		case re > 0 && im == 0:
			return x
		case math.IsInf(im, 0) || math.IsNaN(im):
			if re < 0 {
				return complex(0, math.Copysign(0, im))
			} else {
				return complex(math.Inf(1.0), math.NaN())
			}
		}
	case math.IsNaN(re):
		if im == 0 {
			return complex(math.NaN(), im)
		}
	}
	r := math.Exp(real(x))
	s, c := math.Sincos(imag(x))
	return complex(r*c, r*s)
}

```

// === FILE: references!/go/src/math/cmplx/isinf.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmplx

import "math"

// IsInf reports whether either real(x) or imag(x) is an infinity.
func IsInf(x complex128) bool {
	if math.IsInf(real(x), 0) || math.IsInf(imag(x), 0) {
		return true
	}
	return false
}

// Inf returns a complex infinity, complex(+Inf, +Inf).
func Inf() complex128 {
	inf := math.Inf(1)
	return complex(inf, inf)
}

```

// === FILE: references!/go/src/math/cmplx/isnan.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmplx

import "math"

// IsNaN reports whether either real(x) or imag(x) is NaN
// and neither is an infinity.
func IsNaN(x complex128) bool {
	switch {
	case math.IsInf(real(x), 0) || math.IsInf(imag(x), 0):
		return false
	case math.IsNaN(real(x)) || math.IsNaN(imag(x)):
		return true
	}
	return false
}

// NaN returns a complex “not-a-number” value.
func NaN() complex128 {
	nan := math.NaN()
	return complex(nan, nan)
}

```

// === FILE: references!/go/src/math/cmplx/log.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmplx

import "math"

// The original C code, the long comment, and the constants
// below are from http://netlib.sandia.gov/cephes/c9x-complex/clog.c.
// The go code is a simplified version of the original C.
//
// Cephes Math Library Release 2.8:  June, 2000
// Copyright 1984, 1987, 1989, 1992, 2000 by Stephen L. Moshier
//
// The readme file at http://netlib.sandia.gov/cephes/ says:
//    Some software in this archive may be from the book _Methods and
// Programs for Mathematical Functions_ (Prentice-Hall or Simon & Schuster
// International, 1989) or from the Cephes Mathematical Library, a
// commercial product. In either event, it is copyrighted by the author.
// What you see here may be used freely but it comes with no support or
// guarantee.
//
//   The two known misprints in the book are repaired here in the
// source listings for the gamma function and the incomplete beta
// integral.
//
//   Stephen L. Moshier
//   moshier@na-net.ornl.gov

// Complex natural logarithm
//
// DESCRIPTION:
//
// Returns complex logarithm to the base e (2.718...) of
// the complex argument z.
//
// If
//       z = x + iy, r = sqrt( x**2 + y**2 ),
// then
//       w = log(r) + i arctan(y/x).
//
// The arctangent ranges from -PI to +PI.
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    DEC       -10,+10      7000       8.5e-17     1.9e-17
//    IEEE      -10,+10     30000       5.0e-15     1.1e-16
//
// Larger relative error can be observed for z near 1 +i0.
// In IEEE arithmetic the peak absolute error is 5.2e-16, rms
// absolute error 1.0e-16.

// Log returns the natural logarithm of x.
func Log(x complex128) complex128 {
	return complex(math.Log(Abs(x)), Phase(x))
}

// Log10 returns the decimal logarithm of x.
func Log10(x complex128) complex128 {
	z := Log(x)
	return complex(math.Log10E*real(z), math.Log10E*imag(z))
}

```

// === FILE: references!/go/src/math/cmplx/phase.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmplx

import "math"

// Phase returns the phase (also called the argument) of x.
// The returned value is in the range [-Pi, Pi].
func Phase(x complex128) float64 { return math.Atan2(imag(x), real(x)) }

```

// === FILE: references!/go/src/math/cmplx/polar.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmplx

// Polar returns the absolute value r and phase θ of x,
// such that x = r * e**θi.
// The phase is in the range [-Pi, Pi].
func Polar(x complex128) (r, θ float64) {
	return Abs(x), Phase(x)
}

```

// === FILE: references!/go/src/math/cmplx/pow.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmplx

import "math"

// The original C code, the long comment, and the constants
// below are from http://netlib.sandia.gov/cephes/c9x-complex/clog.c.
// The go code is a simplified version of the original C.
//
// Cephes Math Library Release 2.8:  June, 2000
// Copyright 1984, 1987, 1989, 1992, 2000 by Stephen L. Moshier
//
// The readme file at http://netlib.sandia.gov/cephes/ says:
//    Some software in this archive may be from the book _Methods and
// Programs for Mathematical Functions_ (Prentice-Hall or Simon & Schuster
// International, 1989) or from the Cephes Mathematical Library, a
// commercial product. In either event, it is copyrighted by the author.
// What you see here may be used freely but it comes with no support or
// guarantee.
//
//   The two known misprints in the book are repaired here in the
// source listings for the gamma function and the incomplete beta
// integral.
//
//   Stephen L. Moshier
//   moshier@na-net.ornl.gov

// Complex power function
//
// DESCRIPTION:
//
// Raises complex A to the complex Zth power.
// Definition is per AMS55 # 4.2.8,
// analytically equivalent to cpow(a,z) = cexp(z clog(a)).
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    IEEE      -10,+10     30000       9.4e-15     1.5e-15

// Pow returns x**y, the base-x exponential of y.
// For generalized compatibility with [math.Pow]:
//
//	Pow(0, ±0) returns 1+0i
//	Pow(0, c) for real(c)<0 returns Inf+0i if imag(c) is zero, otherwise Inf+Inf i.
func Pow(x, y complex128) complex128 {
	if x == 0 { // Guaranteed also true for x == -0.
		if IsNaN(y) {
			return NaN()
		}
		r, i := real(y), imag(y)
		switch {
		case r == 0:
			return 1
		case r < 0:
			if i == 0 {
				return complex(math.Inf(1), 0)
			}
			return Inf()
		case r > 0:
			return 0
		}
		panic("not reached")
	}
	modulus := Abs(x)
	if modulus == 0 {
		return complex(0, 0)
	}
	r := math.Pow(modulus, real(y))
	arg := Phase(x)
	theta := real(y) * arg
	if imag(y) != 0 {
		r *= math.Exp(-imag(y) * arg)
		theta += imag(y) * math.Log(modulus)
	}
	s, c := math.Sincos(theta)
	return complex(r*c, r*s)
}

```

// === FILE: references!/go/src/math/cmplx/rect.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmplx

import "math"

// Rect returns the complex number x with polar coordinates r, θ.
func Rect(r, θ float64) complex128 {
	s, c := math.Sincos(θ)
	return complex(r*c, r*s)
}

```

// === FILE: references!/go/src/math/cmplx/sin.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmplx

import "math"

// The original C code, the long comment, and the constants
// below are from http://netlib.sandia.gov/cephes/c9x-complex/clog.c.
// The go code is a simplified version of the original C.
//
// Cephes Math Library Release 2.8:  June, 2000
// Copyright 1984, 1987, 1989, 1992, 2000 by Stephen L. Moshier
//
// The readme file at http://netlib.sandia.gov/cephes/ says:
//    Some software in this archive may be from the book _Methods and
// Programs for Mathematical Functions_ (Prentice-Hall or Simon & Schuster
// International, 1989) or from the Cephes Mathematical Library, a
// commercial product. In either event, it is copyrighted by the author.
// What you see here may be used freely but it comes with no support or
// guarantee.
//
//   The two known misprints in the book are repaired here in the
// source listings for the gamma function and the incomplete beta
// integral.
//
//   Stephen L. Moshier
//   moshier@na-net.ornl.gov

// Complex circular sine
//
// DESCRIPTION:
//
// If
//     z = x + iy,
//
// then
//
//     w = sin x  cosh y  +  i cos x sinh y.
//
// csin(z) = -i csinh(iz).
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    DEC       -10,+10      8400       5.3e-17     1.3e-17
//    IEEE      -10,+10     30000       3.8e-16     1.0e-16
// Also tested by csin(casin(z)) = z.

// Sin returns the sine of x.
func Sin(x complex128) complex128 {
	switch re, im := real(x), imag(x); {
	case im == 0 && (math.IsInf(re, 0) || math.IsNaN(re)):
		return complex(math.NaN(), im)
	case math.IsInf(im, 0):
		switch {
		case re == 0:
			return x
		case math.IsInf(re, 0) || math.IsNaN(re):
			return complex(math.NaN(), im)
		}
	case re == 0 && math.IsNaN(im):
		return x
	}
	s, c := math.Sincos(real(x))
	sh, ch := sinhcosh(imag(x))
	return complex(s*ch, c*sh)
}

// Complex hyperbolic sine
//
// DESCRIPTION:
//
// csinh z = (cexp(z) - cexp(-z))/2
//         = sinh x * cos y  +  i cosh x * sin y .
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    IEEE      -10,+10     30000       3.1e-16     8.2e-17

// Sinh returns the hyperbolic sine of x.
func Sinh(x complex128) complex128 {
	switch re, im := real(x), imag(x); {
	case re == 0 && (math.IsInf(im, 0) || math.IsNaN(im)):
		return complex(re, math.NaN())
	case math.IsInf(re, 0):
		switch {
		case im == 0:
			return complex(re, im)
		case math.IsInf(im, 0) || math.IsNaN(im):
			return complex(re, math.NaN())
		}
	case im == 0 && math.IsNaN(re):
		return complex(math.NaN(), im)
	}
	s, c := math.Sincos(imag(x))
	sh, ch := sinhcosh(real(x))
	return complex(c*sh, s*ch)
}

// Complex circular cosine
//
// DESCRIPTION:
//
// If
//     z = x + iy,
//
// then
//
//     w = cos x  cosh y  -  i sin x sinh y.
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    DEC       -10,+10      8400       4.5e-17     1.3e-17
//    IEEE      -10,+10     30000       3.8e-16     1.0e-16

// Cos returns the cosine of x.
func Cos(x complex128) complex128 {
	switch re, im := real(x), imag(x); {
	case im == 0 && (math.IsInf(re, 0) || math.IsNaN(re)):
		return complex(math.NaN(), -im*math.Copysign(0, re))
	case math.IsInf(im, 0):
		switch {
		case re == 0:
			return complex(math.Inf(1), -re*math.Copysign(0, im))
		case math.IsInf(re, 0) || math.IsNaN(re):
			return complex(math.Inf(1), math.NaN())
		}
	case re == 0 && math.IsNaN(im):
		return complex(math.NaN(), 0)
	}
	s, c := math.Sincos(real(x))
	sh, ch := sinhcosh(imag(x))
	return complex(c*ch, -s*sh)
}

// Complex hyperbolic cosine
//
// DESCRIPTION:
//
// ccosh(z) = cosh x  cos y + i sinh x sin y .
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    IEEE      -10,+10     30000       2.9e-16     8.1e-17

// Cosh returns the hyperbolic cosine of x.
func Cosh(x complex128) complex128 {
	switch re, im := real(x), imag(x); {
	case re == 0 && (math.IsInf(im, 0) || math.IsNaN(im)):
		return complex(math.NaN(), re*math.Copysign(0, im))
	case math.IsInf(re, 0):
		switch {
		case im == 0:
			return complex(math.Inf(1), im*math.Copysign(0, re))
		case math.IsInf(im, 0) || math.IsNaN(im):
			return complex(math.Inf(1), math.NaN())
		}
	case im == 0 && math.IsNaN(re):
		return complex(math.NaN(), im)
	}
	s, c := math.Sincos(imag(x))
	sh, ch := sinhcosh(real(x))
	return complex(c*ch, s*sh)
}

// calculate sinh and cosh.
func sinhcosh(x float64) (sh, ch float64) {
	if math.Abs(x) <= 0.5 {
		return math.Sinh(x), math.Cosh(x)
	}
	e := math.Exp(x)
	ei := 0.5 / e
	e *= 0.5
	return e - ei, e + ei
}

```

// === FILE: references!/go/src/math/cmplx/sqrt.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmplx

import "math"

// The original C code, the long comment, and the constants
// below are from http://netlib.sandia.gov/cephes/c9x-complex/clog.c.
// The go code is a simplified version of the original C.
//
// Cephes Math Library Release 2.8:  June, 2000
// Copyright 1984, 1987, 1989, 1992, 2000 by Stephen L. Moshier
//
// The readme file at http://netlib.sandia.gov/cephes/ says:
//    Some software in this archive may be from the book _Methods and
// Programs for Mathematical Functions_ (Prentice-Hall or Simon & Schuster
// International, 1989) or from the Cephes Mathematical Library, a
// commercial product. In either event, it is copyrighted by the author.
// What you see here may be used freely but it comes with no support or
// guarantee.
//
//   The two known misprints in the book are repaired here in the
// source listings for the gamma function and the incomplete beta
// integral.
//
//   Stephen L. Moshier
//   moshier@na-net.ornl.gov

// Complex square root
//
// DESCRIPTION:
//
// If z = x + iy,  r = |z|, then
//
//                       1/2
// Re w  =  [ (r + x)/2 ]   ,
//
//                       1/2
// Im w  =  [ (r - x)/2 ]   .
//
// Cancellation error in r-x or r+x is avoided by using the
// identity  2 Re w Im w  =  y.
//
// Note that -w is also a square root of z. The root chosen
// is always in the right half plane and Im w has the same sign as y.
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    DEC       -10,+10     25000       3.2e-17     9.6e-18
//    IEEE      -10,+10   1,000,000     2.9e-16     6.1e-17

// Sqrt returns the square root of x.
// The result r is chosen so that real(r) ≥ 0 and imag(r) has the same sign as imag(x).
func Sqrt(x complex128) complex128 {
	if imag(x) == 0 {
		// Ensure that imag(r) has the same sign as imag(x) for imag(x) == signed zero.
		if real(x) == 0 {
			return complex(0, imag(x))
		}
		if real(x) < 0 {
			return complex(0, math.Copysign(math.Sqrt(-real(x)), imag(x)))
		}
		return complex(math.Sqrt(real(x)), imag(x))
	} else if math.IsInf(imag(x), 0) {
		return complex(math.Inf(1.0), imag(x))
	}
	if real(x) == 0 {
		if imag(x) < 0 {
			r := math.Sqrt(-0.5 * imag(x))
			return complex(r, -r)
		}
		r := math.Sqrt(0.5 * imag(x))
		return complex(r, r)
	}
	a := real(x)
	b := imag(x)
	var scale float64
	// Rescale to avoid internal overflow or underflow.
	if math.Abs(a) > 4 || math.Abs(b) > 4 {
		a *= 0.25
		b *= 0.25
		scale = 2
	} else {
		a *= 1.8014398509481984e16 // 2**54
		b *= 1.8014398509481984e16
		scale = 7.450580596923828125e-9 // 2**-27
	}
	r := math.Hypot(a, b)
	var t float64
	if a > 0 {
		t = math.Sqrt(0.5*r + 0.5*a)
		r = scale * math.Abs((0.5*b)/t)
		t *= scale
	} else {
		r = math.Sqrt(0.5*r - 0.5*a)
		t = scale * math.Abs((0.5*b)/r)
		r *= scale
	}
	if b < 0 {
		return complex(t, -r)
	}
	return complex(t, r)
}

```

// === FILE: references!/go/src/math/cmplx/tan.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmplx

import (
	"math"
	"math/bits"
)

// The original C code, the long comment, and the constants
// below are from http://netlib.sandia.gov/cephes/c9x-complex/clog.c.
// The go code is a simplified version of the original C.
//
// Cephes Math Library Release 2.8:  June, 2000
// Copyright 1984, 1987, 1989, 1992, 2000 by Stephen L. Moshier
//
// The readme file at http://netlib.sandia.gov/cephes/ says:
//    Some software in this archive may be from the book _Methods and
// Programs for Mathematical Functions_ (Prentice-Hall or Simon & Schuster
// International, 1989) or from the Cephes Mathematical Library, a
// commercial product. In either event, it is copyrighted by the author.
// What you see here may be used freely but it comes with no support or
// guarantee.
//
//   The two known misprints in the book are repaired here in the
// source listings for the gamma function and the incomplete beta
// integral.
//
//   Stephen L. Moshier
//   moshier@na-net.ornl.gov

// Complex circular tangent
//
// DESCRIPTION:
//
// If
//     z = x + iy,
//
// then
//
//           sin 2x  +  i sinh 2y
//     w  =  --------------------.
//            cos 2x  +  cosh 2y
//
// On the real axis the denominator is zero at odd multiples
// of PI/2. The denominator is evaluated by its Taylor
// series near these points.
//
// ctan(z) = -i ctanh(iz).
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    DEC       -10,+10      5200       7.1e-17     1.6e-17
//    IEEE      -10,+10     30000       7.2e-16     1.2e-16
// Also tested by ctan * ccot = 1 and catan(ctan(z))  =  z.

// Tan returns the tangent of x.
func Tan(x complex128) complex128 {
	switch re, im := real(x), imag(x); {
	case math.IsInf(im, 0):
		switch {
		case math.IsInf(re, 0) || math.IsNaN(re):
			return complex(math.Copysign(0, re), math.Copysign(1, im))
		}
		return complex(math.Copysign(0, math.Sin(2*re)), math.Copysign(1, im))
	case re == 0 && math.IsNaN(im):
		return x
	}
	d := math.Cos(2*real(x)) + math.Cosh(2*imag(x))
	if math.Abs(d) < 0.25 {
		d = tanSeries(x)
	}
	if d == 0 {
		return Inf()
	}
	return complex(math.Sin(2*real(x))/d, math.Sinh(2*imag(x))/d)
}

// Complex hyperbolic tangent
//
// DESCRIPTION:
//
// tanh z = (sinh 2x  +  i sin 2y) / (cosh 2x + cos 2y) .
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    IEEE      -10,+10     30000       1.7e-14     2.4e-16

// Tanh returns the hyperbolic tangent of x.
func Tanh(x complex128) complex128 {
	switch re, im := real(x), imag(x); {
	case math.IsInf(re, 0):
		switch {
		case math.IsInf(im, 0) || math.IsNaN(im):
			return complex(math.Copysign(1, re), math.Copysign(0, im))
		}
		return complex(math.Copysign(1, re), math.Copysign(0, math.Sin(2*im)))
	case im == 0 && math.IsNaN(re):
		return x
	}
	d := math.Cosh(2*real(x)) + math.Cos(2*imag(x))
	if d == 0 {
		return Inf()
	}
	return complex(math.Sinh(2*real(x))/d, math.Sin(2*imag(x))/d)
}

// reducePi reduces the input argument x to the range (-Pi/2, Pi/2].
// x must be greater than or equal to 0. For small arguments it
// uses Cody-Waite reduction in 3 float64 parts based on:
// "Elementary Function Evaluation:  Algorithms and Implementation"
// Jean-Michel Muller, 1997.
// For very large arguments it uses Payne-Hanek range reduction based on:
// "ARGUMENT REDUCTION FOR HUGE ARGUMENTS: Good to the Last Bit"
// K. C. Ng et al, March 24, 1992.
func reducePi(x float64) float64 {
	// reduceThreshold is the maximum value of x where the reduction using
	// Cody-Waite reduction still gives accurate results. This threshold
	// is set by t*PIn being representable as a float64 without error
	// where t is given by t = floor(x * (1 / Pi)) and PIn are the leading partial
	// terms of Pi. Since the leading terms, PI1 and PI2 below, have 30 and 32
	// trailing zero bits respectively, t should have less than 30 significant bits.
	//	t < 1<<30  -> floor(x*(1/Pi)+0.5) < 1<<30 -> x < (1<<30-1) * Pi - 0.5
	// So, conservatively we can take x < 1<<30.
	const reduceThreshold float64 = 1 << 30
	if math.Abs(x) < reduceThreshold {
		// Use Cody-Waite reduction in three parts.
		const (
			// PI1, PI2 and PI3 comprise an extended precision value of PI
			// such that PI ~= PI1 + PI2 + PI3. The parts are chosen so
			// that PI1 and PI2 have an approximately equal number of trailing
			// zero bits. This ensures that t*PI1 and t*PI2 are exact for
			// large integer values of t. The full precision PI3 ensures the
			// approximation of PI is accurate to 102 bits to handle cancellation
			// during subtraction.
			PI1 = 3.141592502593994      // 0x400921fb40000000
			PI2 = 1.5099578831723193e-07 // 0x3e84442d00000000
			PI3 = 1.0780605716316238e-14 // 0x3d08469898cc5170
		)
		t := x / math.Pi
		t += 0.5
		t = float64(int64(t)) // int64(t) = the multiple
		return ((x - t*PI1) - t*PI2) - t*PI3
	}
	// Must apply Payne-Hanek range reduction
	const (
		mask     = 0x7FF
		shift    = 64 - 11 - 1
		bias     = 1023
		fracMask = 1<<shift - 1
	)
	// Extract out the integer and exponent such that,
	// x = ix * 2 ** exp.
	ix := math.Float64bits(x)
	exp := int(ix>>shift&mask) - bias - shift
	ix &= fracMask
	ix |= 1 << shift

	// mPi is the binary digits of 1/Pi as a uint64 array,
	// that is, 1/Pi = Sum mPi[i]*2^(-64*i).
	// 19 64-bit digits give 1216 bits of precision
	// to handle the largest possible float64 exponent.
	var mPi = [...]uint64{
		0x0000000000000000,
		0x517cc1b727220a94,
		0xfe13abe8fa9a6ee0,
		0x6db14acc9e21c820,
		0xff28b1d5ef5de2b0,
		0xdb92371d2126e970,
		0x0324977504e8c90e,
		0x7f0ef58e5894d39f,
		0x74411afa975da242,
		0x74ce38135a2fbf20,
		0x9cc8eb1cc1a99cfa,
		0x4e422fc5defc941d,
		0x8ffc4bffef02cc07,
		0xf79788c5ad05368f,
		0xb69b3f6793e584db,
		0xa7a31fb34f2ff516,
		0xba93dd63f5f2f8bd,
		0x9e839cfbc5294975,
		0x35fdafd88fc6ae84,
		0x2b0198237e3db5d5,
	}
	// Use the exponent to extract the 3 appropriate uint64 digits from mPi,
	// B ~ (z0, z1, z2), such that the product leading digit has the exponent -64.
	// Note, exp >= 50 since x >= reduceThreshold and exp < 971 for maximum float64.
	digit, bitshift := uint(exp+64)/64, uint(exp+64)%64
	z0 := (mPi[digit] << bitshift) | (mPi[digit+1] >> (64 - bitshift))
	z1 := (mPi[digit+1] << bitshift) | (mPi[digit+2] >> (64 - bitshift))
	z2 := (mPi[digit+2] << bitshift) | (mPi[digit+3] >> (64 - bitshift))
	// Multiply mantissa by the digits and extract the upper two digits (hi, lo).
	z2hi, _ := bits.Mul64(z2, ix)
	z1hi, z1lo := bits.Mul64(z1, ix)
	z0lo := z0 * ix
	lo, c := bits.Add64(z1lo, z2hi, 0)
	hi, _ := bits.Add64(z0lo, z1hi, c)
	// Find the magnitude of the fraction.
	lz := uint(bits.LeadingZeros64(hi))
	e := uint64(bias - (lz + 1))
	// Clear implicit mantissa bit and shift into place.
	hi = (hi << (lz + 1)) | (lo >> (64 - (lz + 1)))
	hi >>= 64 - shift
	// Include the exponent and convert to a float.
	hi |= e << shift
	x = math.Float64frombits(hi)
	// map to (-Pi/2, Pi/2]
	if x > 0.5 {
		x--
	}
	return math.Pi * x
}

// Taylor series expansion for cosh(2y) - cos(2x)
func tanSeries(z complex128) float64 {
	const MACHEP = 1.0 / (1 << 53)
	x := math.Abs(2 * real(z))
	y := math.Abs(2 * imag(z))
	x = reducePi(x)
	x = x * x
	y = y * y
	x2 := 1.0
	y2 := 1.0
	f := 1.0
	rn := 0.0
	d := 0.0
	for {
		rn++
		f *= rn
		rn++
		f *= rn
		x2 *= x
		y2 *= y
		t := y2 + x2
		t /= f
		d += t

		rn++
		f *= rn
		rn++
		f *= rn
		x2 *= x
		y2 *= y
		t = y2 - x2
		t /= f
		d += t
		if !(math.Abs(t/d) > MACHEP) {
			// Caution: Use ! and > instead of <= for correct behavior if t/d is NaN.
			// See issue 17577.
			break
		}
	}
	return d
}

// Complex circular cotangent
//
// DESCRIPTION:
//
// If
//     z = x + iy,
//
// then
//
//           sin 2x  -  i sinh 2y
//     w  =  --------------------.
//            cosh 2y  -  cos 2x
//
// On the real axis, the denominator has zeros at even
// multiples of PI/2.  Near these points it is evaluated
// by a Taylor series.
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    DEC       -10,+10      3000       6.5e-17     1.6e-17
//    IEEE      -10,+10     30000       9.2e-16     1.2e-16
// Also tested by ctan * ccot = 1 + i0.

// Cot returns the cotangent of x.
func Cot(x complex128) complex128 {
	d := math.Cosh(2*imag(x)) - math.Cos(2*real(x))
	if math.Abs(d) < 0.25 {
		d = tanSeries(x)
	}
	if d == 0 {
		return Inf()
	}
	return complex(math.Sin(2*real(x))/d, -math.Sinh(2*imag(x))/d)
}

```

// === FILE: references!/go/src/math/const.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package math provides basic constants and mathematical functions.
//
// This package does not guarantee bit-identical results across architectures.
package math

// Mathematical constants.
const (
	E   = 2.71828182845904523536028747135266249775724709369995957496696763 // https://oeis.org/A001113
	Pi  = 3.14159265358979323846264338327950288419716939937510582097494459 // https://oeis.org/A000796
	Phi = 1.61803398874989484820458683436563811772030917980576286213544862 // https://oeis.org/A001622

	Sqrt2   = 1.41421356237309504880168872420969807856967187537694807317667974 // https://oeis.org/A002193
	SqrtE   = 1.64872127070012814684865078781416357165377610071014801157507931 // https://oeis.org/A019774
	SqrtPi  = 1.77245385090551602729816748334114518279754945612238712821380779 // https://oeis.org/A002161
	SqrtPhi = 1.27201964951406896425242246173749149171560804184009624861664038 // https://oeis.org/A139339

	Ln2    = 0.693147180559945309417232121458176568075500134360255254120680009 // https://oeis.org/A002162
	Log2E  = 1 / Ln2
	Ln10   = 2.30258509299404568401799145468436420760110148862877297603332790 // https://oeis.org/A002392
	Log10E = 1 / Ln10
)

// Floating-point limit values.
// Max is the largest finite value representable by the type.
// SmallestNonzero is the smallest positive, non-zero value representable by the type.
const (
	MaxFloat32             = 0x1p127 * (1 + (1 - 0x1p-23)) // 3.40282346638528859811704183484516925440e+38
	SmallestNonzeroFloat32 = 0x1p-126 * 0x1p-23            // 1.401298464324817070923729583289916131280e-45

	MaxFloat64             = 0x1p1023 * (1 + (1 - 0x1p-52)) // 1.79769313486231570814527423731704356798070e+308
	SmallestNonzeroFloat64 = 0x1p-1022 * 0x1p-52            // 4.9406564584124654417656879286822137236505980e-324
)

// Integer limit values.
const (
	intSize = 32 << (^uint(0) >> 63) // 32 or 64

	MaxInt    = 1<<(intSize-1) - 1  // MaxInt32 or MaxInt64 depending on intSize.
	MinInt    = -1 << (intSize - 1) // MinInt32 or MinInt64 depending on intSize.
	MaxInt8   = 1<<7 - 1            // 127
	MinInt8   = -1 << 7             // -128
	MaxInt16  = 1<<15 - 1           // 32767
	MinInt16  = -1 << 15            // -32768
	MaxInt32  = 1<<31 - 1           // 2147483647
	MinInt32  = -1 << 31            // -2147483648
	MaxInt64  = 1<<63 - 1           // 9223372036854775807
	MinInt64  = -1 << 63            // -9223372036854775808
	MaxUint   = 1<<intSize - 1      // MaxUint32 or MaxUint64 depending on intSize.
	MaxUint8  = 1<<8 - 1            // 255
	MaxUint16 = 1<<16 - 1           // 65535
	MaxUint32 = 1<<32 - 1           // 4294967295
	MaxUint64 = 1<<64 - 1           // 18446744073709551615
)

```

// === FILE: references!/go/src/math/copysign.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// Copysign returns a value with the magnitude of f
// and the sign of sign.
func Copysign(f, sign float64) float64 {
	return Float64frombits(Float64bits(f)&^signMask | Float64bits(sign)&signMask)
}

```

// === FILE: references!/go/src/math/cosh_s390x.s ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// Constants
DATA coshrodataL23<>+0(SB)/8, $0.231904681384629956E-16
DATA coshrodataL23<>+8(SB)/8, $0.693147180559945286E+00
DATA coshrodataL23<>+16(SB)/8, $0.144269504088896339E+01
DATA coshrodataL23<>+24(SB)/8, $704.E0
GLOBL coshrodataL23<>+0(SB), RODATA, $32
DATA coshxinf<>+0(SB)/8, $0x7FF0000000000000
GLOBL coshxinf<>+0(SB), RODATA, $8
DATA coshxlim1<>+0(SB)/8, $800.E0
GLOBL coshxlim1<>+0(SB), RODATA, $8
DATA coshxaddhy<>+0(SB)/8, $0xc2f0000100003fdf
GLOBL coshxaddhy<>+0(SB), RODATA, $8
DATA coshx4ff<>+0(SB)/8, $0x4ff0000000000000
GLOBL coshx4ff<>+0(SB), RODATA, $8
DATA coshe1<>+0(SB)/8, $0x3ff000000000000a
GLOBL coshe1<>+0(SB), RODATA, $8

// Log multiplier table
DATA coshtab<>+0(SB)/8, $0.442737824274138381E-01
DATA coshtab<>+8(SB)/8, $0.263602189790660309E-01
DATA coshtab<>+16(SB)/8, $0.122565642281703586E-01
DATA coshtab<>+24(SB)/8, $0.143757052860721398E-02
DATA coshtab<>+32(SB)/8, $-.651375034121276075E-02
DATA coshtab<>+40(SB)/8, $-.119317678849450159E-01
DATA coshtab<>+48(SB)/8, $-.150868749549871069E-01
DATA coshtab<>+56(SB)/8, $-.161992609578469234E-01
DATA coshtab<>+64(SB)/8, $-.154492360403337917E-01
DATA coshtab<>+72(SB)/8, $-.129850717389178721E-01
DATA coshtab<>+80(SB)/8, $-.892902649276657891E-02
DATA coshtab<>+88(SB)/8, $-.338202636596794887E-02
DATA coshtab<>+96(SB)/8, $0.357266307045684762E-02
DATA coshtab<>+104(SB)/8, $0.118665304327406698E-01
DATA coshtab<>+112(SB)/8, $0.214434994118118914E-01
DATA coshtab<>+120(SB)/8, $0.322580645161290314E-01
GLOBL coshtab<>+0(SB), RODATA, $128

// Minimax polynomial approximations
DATA coshe2<>+0(SB)/8, $0.500000000000004237e+00
GLOBL coshe2<>+0(SB), RODATA, $8
DATA coshe3<>+0(SB)/8, $0.166666666630345592e+00
GLOBL coshe3<>+0(SB), RODATA, $8
DATA coshe4<>+0(SB)/8, $0.416666664838056960e-01
GLOBL coshe4<>+0(SB), RODATA, $8
DATA coshe5<>+0(SB)/8, $0.833349307718286047e-02
GLOBL coshe5<>+0(SB), RODATA, $8
DATA coshe6<>+0(SB)/8, $0.138926439368309441e-02
GLOBL coshe6<>+0(SB), RODATA, $8

// Cosh returns the hyperbolic cosine of x.
//
// Special cases are:
//      Cosh(±0) = 1
//      Cosh(±Inf) = +Inf
//      Cosh(NaN) = NaN
// The algorithm used is minimax polynomial approximation
// with coefficients determined with a Remez exchange algorithm.

TEXT ·coshAsm(SB),NOSPLIT,$0-16
	FMOVD   x+0(FP), F0
	MOVD    $coshrodataL23<>+0(SB), R9
	LTDBR	F0, F0
	MOVD    $0x4086000000000000, R2
	MOVD    $0x4086000000000000, R3
	BLTU    L19
	FMOVD   F0, F4
L2:
	WORD    $0xED409018     //cdb %f4,.L24-.L23(%r9)
	BYTE    $0x00
	BYTE    $0x19
	BGE     L14     //jnl   .L14
	BVS     L14
	WFCEDBS V4, V4, V2
	BEQ     L20
L1:
	FMOVD   F0, ret+8(FP)
	RET

L14:
	WFCEDBS V4, V4, V2
	BVS     L1
	MOVD    $coshxlim1<>+0(SB), R1
	FMOVD   0(R1), F2
	WFCHEDBS        V4, V2, V2
	BEQ     L21
	MOVD    $coshxaddhy<>+0(SB), R1
	FMOVD   coshrodataL23<>+16(SB), F5
	FMOVD   0(R1), F2
	WFMSDB  V0, V5, V2, V5
	FMOVD   coshrodataL23<>+8(SB), F3
	FADD    F5, F2
	MOVD    $coshe6<>+0(SB), R1
	WFMSDB  V2, V3, V0, V3
	FMOVD   0(R1), F6
	WFMDB   V3, V3, V1
	MOVD    $coshe4<>+0(SB), R1
	FMOVD   coshrodataL23<>+0(SB), F7
	WFMADB  V2, V7, V3, V2
	FMOVD   0(R1), F3
	MOVD    $coshe5<>+0(SB), R1
	WFMADB  V1, V6, V3, V6
	FMOVD   0(R1), F7
	MOVD    $coshe3<>+0(SB), R1
	FMOVD   0(R1), F3
	WFMADB  V1, V7, V3, V7
	FNEG    F2, F3
	LGDR    F5, R1
	MOVD    $coshe2<>+0(SB), R3
	WFCEDBS V4, V0, V0
	FMOVD   0(R3), F5
	MOVD    $coshe1<>+0(SB), R3
	WFMADB  V1, V6, V5, V6
	FMOVD   0(R3), F5
	RISBGN	$0, $15, $48, R1, R2
	WFMADB  V1, V7, V5, V1
	BVS     L22
	RISBGZ	$57, $60, $3, R1, R4
	MOVD    $coshtab<>+0(SB), R3
	WFMADB  V3, V6, V1, V6
	WORD    $0x68043000     //ld    %f0,0(%r4,%r3)
	FMSUB   F0, F3, F2
	WORD    $0xA71AF000     //ahi   %r1,-4096
	WFMADB  V2, V6, V0, V6
L17:
	RISBGN	$0, $15, $48, R1, R2
	LDGR    R2, F2
	FMADD   F2, F6, F2
	MOVD    $coshx4ff<>+0(SB), R1
	FMOVD   0(R1), F0
	FMUL    F2, F0
	FMOVD   F0, ret+8(FP)
	RET

L19:
	FNEG    F0, F4
	BR      L2
L20:
	MOVD    $coshxaddhy<>+0(SB), R1
	FMOVD   coshrodataL23<>+16(SB), F3
	FMOVD   0(R1), F2
	WFMSDB  V0, V3, V2, V3
	FMOVD   coshrodataL23<>+8(SB), F4
	FADD    F3, F2
	MOVD    $coshe6<>+0(SB), R1
	FMSUB   F4, F2, F0
	FMOVD   0(R1), F6
	WFMDB   V0, V0, V1
	MOVD    $coshe4<>+0(SB), R1
	FMOVD   0(R1), F4
	MOVD    $coshe5<>+0(SB), R1
	FMOVD   coshrodataL23<>+0(SB), F5
	WFMADB  V1, V6, V4, V6
	FMADD   F5, F2, F0
	FMOVD   0(R1), F2
	MOVD    $coshe3<>+0(SB), R1
	FMOVD   0(R1), F4
	WFMADB  V1, V2, V4, V2
	MOVD    $coshe2<>+0(SB), R1
	FMOVD   0(R1), F5
	FNEG    F0, F4
	WFMADB  V1, V6, V5, V6
	MOVD    $coshe1<>+0(SB), R1
	FMOVD   0(R1), F5
	WFMADB  V1, V2, V5, V1
	LGDR    F3, R1
	MOVD    $coshtab<>+0(SB), R5
	WFMADB  V4, V6, V1, V3
	RISBGZ	$57, $60, $3, R1, R4
	WFMSDB  V4, V6, V1, V6
	WORD    $0x68145000     //ld %f1,0(%r4,%r5)
	WFMSDB  V4, V1, V0, V2
	WORD    $0xA7487FBE     //lhi %r4,32702
	FMADD   F3, F2, F1
	SUBW    R1, R4
	RISBGZ	$57, $60, $3, R4, R12
	WORD    $0x682C5000     //ld %f2,0(%r12,%r5)
	FMSUB   F2, F4, F0
	RISBGN	$0, $15, $48, R1, R2
	WFMADB  V0, V6, V2, V6
	RISBGN	$0, $15, $48, R4, R3
	LDGR    R2, F2
	LDGR    R3, F0
	FMADD   F2, F1, F2
	FMADD   F0, F6, F0
	FADD    F2, F0
	FMOVD   F0, ret+8(FP)
	RET

L22:
	WORD    $0xA7387FBE     //lhi %r3,32702
	MOVD    $coshtab<>+0(SB), R4
	SUBW    R1, R3
	WFMSDB  V3, V6, V1, V6
	RISBGZ	$57, $60, $3, R3, R3
	WORD    $0x68034000     //ld %f0,0(%r3,%r4)
	FMSUB   F0, F3, F2
	WORD    $0xA7386FBE     //lhi %r3,28606
	WFMADB  V2, V6, V0, V6
	SUBW    R1, R3, R1
	BR      L17
L21:
	MOVD    $coshxinf<>+0(SB), R1
	FMOVD   0(R1), F0
	FMOVD   F0, ret+8(FP)
	RET


```

// === FILE: references!/go/src/math/dim.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// Dim returns the maximum of x-y or 0.
//
// Special cases are:
//
//	Dim(+Inf, +Inf) = NaN
//	Dim(-Inf, -Inf) = NaN
//	Dim(x, NaN) = Dim(NaN, x) = NaN
func Dim(x, y float64) float64 {
	// The special cases result in NaN after the subtraction:
	//      +Inf - +Inf = NaN
	//      -Inf - -Inf = NaN
	//       NaN - y    = NaN
	//         x - NaN  = NaN
	v := x - y
	if v <= 0 {
		// v is negative or 0
		return 0
	}
	// v is positive or NaN
	return v
}

// Max returns the larger of x or y.
//
// Special cases are:
//
//	Max(x, +Inf) = Max(+Inf, x) = +Inf
//	Max(x, NaN) = Max(NaN, x) = NaN
//	Max(+0, ±0) = Max(±0, +0) = +0
//	Max(-0, -0) = -0
//
// Note that this differs from the built-in function max when called
// with NaN and +Inf.
func Max(x, y float64) float64 {
	if haveArchMax {
		return archMax(x, y)
	}
	return max(x, y)
}

func max(x, y float64) float64 {
	// special cases
	switch {
	case IsInf(x, 1) || IsInf(y, 1):
		return Inf(1)
	case IsNaN(x) || IsNaN(y):
		return NaN()
	case x == 0 && x == y:
		if Signbit(x) {
			return y
		}
		return x
	}
	if x > y {
		return x
	}
	return y
}

// Min returns the smaller of x or y.
//
// Special cases are:
//
//	Min(x, -Inf) = Min(-Inf, x) = -Inf
//	Min(x, NaN) = Min(NaN, x) = NaN
//	Min(-0, ±0) = Min(±0, -0) = -0
//
// Note that this differs from the built-in function min when called
// with NaN and -Inf.
func Min(x, y float64) float64 {
	if haveArchMin {
		return archMin(x, y)
	}
	return min(x, y)
}

func min(x, y float64) float64 {
	// special cases
	switch {
	case IsInf(x, -1) || IsInf(y, -1):
		return Inf(-1)
	case IsNaN(x) || IsNaN(y):
		return NaN()
	case x == 0 && x == y:
		if Signbit(x) {
			return x
		}
		return y
	}
	if x < y {
		return x
	}
	return y
}

```

// === FILE: references!/go/src/math/dim_amd64.s ===
```text
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

#define PosInf 0x7FF0000000000000
#define NaN    0x7FF8000000000001
#define NegInf 0xFFF0000000000000

// func ·archMax(x, y float64) float64
TEXT ·archMax(SB),NOSPLIT,$0
	// +Inf special cases
	MOVQ    $PosInf, AX
	MOVQ    x+0(FP), R8
	CMPQ    AX, R8
	JEQ     isPosInf
	MOVQ    y+8(FP), R9
	CMPQ    AX, R9
	JEQ     isPosInf
	// NaN special cases
	MOVQ    $~(1<<63), DX // bit mask
	MOVQ    $PosInf, AX
	MOVQ    R8, BX
	ANDQ    DX, BX // x = |x|
	CMPQ    AX, BX
	JLT     isMaxNaN
	MOVQ    R9, CX
	ANDQ    DX, CX // y = |y|
	CMPQ    AX, CX
	JLT     isMaxNaN
	// ±0 special cases
	ORQ     CX, BX
	JEQ     isMaxZero

	MOVQ    R8, X0
	MOVQ    R9, X1
	MAXSD   X1, X0
	MOVSD   X0, ret+16(FP)
	RET
isMaxNaN: // return NaN
	MOVQ	$NaN, AX
isPosInf: // return +Inf
	MOVQ    AX, ret+16(FP)
	RET
isMaxZero:
	MOVQ    $(1<<63), AX // -0.0
	CMPQ    AX, R8
	JEQ     +3(PC)
	MOVQ    R8, ret+16(FP) // return 0
	RET
	MOVQ    R9, ret+16(FP) // return other 0
	RET

// func archMin(x, y float64) float64
TEXT ·archMin(SB),NOSPLIT,$0
	// -Inf special cases
	MOVQ    $NegInf, AX
	MOVQ    x+0(FP), R8
	CMPQ    AX, R8
	JEQ     isNegInf
	MOVQ    y+8(FP), R9
	CMPQ    AX, R9
	JEQ     isNegInf
	// NaN special cases
	MOVQ    $~(1<<63), DX
	MOVQ    $PosInf, AX
	MOVQ    R8, BX
	ANDQ    DX, BX // x = |x|
	CMPQ    AX, BX
	JLT     isMinNaN
	MOVQ    R9, CX
	ANDQ    DX, CX // y = |y|
	CMPQ    AX, CX
	JLT     isMinNaN
	// ±0 special cases
	ORQ     CX, BX
	JEQ     isMinZero

	MOVQ    R8, X0
	MOVQ    R9, X1
	MINSD   X1, X0
	MOVSD X0, ret+16(FP)
	RET
isMinNaN: // return NaN
	MOVQ	$NaN, AX
isNegInf: // return -Inf
	MOVQ    AX, ret+16(FP)
	RET
isMinZero:
	MOVQ    $(1<<63), AX // -0.0
	CMPQ    AX, R8
	JEQ     +3(PC)
	MOVQ    R9, ret+16(FP) // return other 0
	RET
	MOVQ    R8, ret+16(FP) // return -0
	RET


```

// === FILE: references!/go/src/math/dim_arm64.s ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

#define PosInf 0x7FF0000000000000
#define NaN    0x7FF8000000000001
#define NegInf 0xFFF0000000000000

// func ·archMax(x, y float64) float64
TEXT ·archMax(SB),NOSPLIT,$0
	// +Inf special cases
	MOVD	$PosInf, R0
	MOVD	x+0(FP), R1
	CMP	R0, R1
	BEQ	isPosInf
	MOVD	y+8(FP), R2
	CMP	R0, R2
	BEQ	isPosInf
	// normal case
	FMOVD	R1, F0
	FMOVD	R2, F1
	FMAXD	F0, F1, F0
	FMOVD	F0, ret+16(FP)
	RET
isPosInf: // return +Inf
	MOVD	R0, ret+16(FP)
	RET

// func archMin(x, y float64) float64
TEXT ·archMin(SB),NOSPLIT,$0
	// -Inf special cases
	MOVD	$NegInf, R0
	MOVD	x+0(FP), R1
	CMP	R0, R1
	BEQ	isNegInf
	MOVD	y+8(FP), R2
	CMP	R0, R2
	BEQ	isNegInf
	// normal case
	FMOVD	R1, F0
	FMOVD	R2, F1
	FMIND	F0, F1, F0
	FMOVD	F0, ret+16(FP)
	RET
isNegInf: // return -Inf
	MOVD	R0, ret+16(FP)
	RET

```

// === FILE: references!/go/src/math/dim_asm.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build amd64 || arm64 || loong64 || riscv64 || s390x

package math

const haveArchMax = true

func archMax(x, y float64) float64

const haveArchMin = true

func archMin(x, y float64) float64

```

// === FILE: references!/go/src/math/dim_loong64.s ===
```text
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

#define PosInf 0x7FF0000000000000
#define NaN    0x7FF8000000000001
#define NegInf 0xFFF0000000000000

TEXT ·archMax(SB),NOSPLIT,$0
	MOVD	x+0(FP), F0
	MOVD	y+8(FP), F1
	FCLASSD	F0, F2
	FCLASSD	F1, F3

	// combine x and y categories together to judge
	MOVV	F2, R4
	MOVV	F3, R5
	OR	R5, R4

	// +Inf special cases
	AND	$64, R4, R5
	BNE	R5, isPosInf

	// NaN special cases
	AND	$2, R4, R5
	BNE	R5, isMaxNaN

	// normal case
	FMAXD	F0, F1, F0
	MOVD	F0, ret+16(FP)
	RET

isMaxNaN:
	MOVV	$NaN, R6
	MOVV	R6, ret+16(FP)
	RET

isPosInf:
	MOVV	$PosInf, R6
	MOVV	R6, ret+16(FP)
	RET

TEXT ·archMin(SB),NOSPLIT,$0
	MOVD	x+0(FP), F0
	MOVD	y+8(FP), F1
	FCLASSD	F0, F2
	FCLASSD	F1, F3

	// combine x and y categories together to judge
	MOVV	F2, R4
	MOVV	F3, R5
	OR	R5, R4

	// -Inf special cases
	AND	$4, R4, R5
	BNE	R5, isNegInf

	// NaN special cases
	AND	$2, R4, R5
	BNE	R5, isMinNaN

	// normal case
	FMIND	F0, F1, F0
	MOVD	F0, ret+16(FP)
	RET

isMinNaN:
	MOVV	$NaN, R6
	MOVV	R6, ret+16(FP)
	RET

isNegInf:
	MOVV	$NegInf, R6
	MOVV	R6, ret+16(FP)
	RET

```

// === FILE: references!/go/src/math/dim_noasm.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !amd64 && !arm64 && !loong64 && !riscv64 && !s390x

package math

const haveArchMax = false

func archMax(x, y float64) float64 {
	panic("not implemented")
}

const haveArchMin = false

func archMin(x, y float64) float64 {
	panic("not implemented")
}

```

// === FILE: references!/go/src/math/dim_riscv64.s ===
```text
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// Values returned from an FCLASS instruction.
#define	NegInf	0x001
#define	PosInf	0x080
#define	NaN	0x200

// func archMax(x, y float64) float64
TEXT ·archMax(SB),NOSPLIT,$0
	MOVD	x+0(FP), F0
	MOVD	y+8(FP), F1
	FCLASSD	F0, X5
	FCLASSD	F1, X6

	// +Inf special cases
	MOV	$PosInf, X7
	BEQ	X7, X5, isMaxX
	BEQ	X7, X6, isMaxY

	// NaN special cases
	MOV	$NaN, X7
	BEQ	X7, X5, isMaxX
	BEQ	X7, X6, isMaxY

	// normal case
	FMAXD	F0, F1, F0
	MOVD	F0, ret+16(FP)
	RET

isMaxX: // return x
	MOVD	F0, ret+16(FP)
	RET

isMaxY: // return y
	MOVD	F1, ret+16(FP)
	RET

// func archMin(x, y float64) float64
TEXT ·archMin(SB),NOSPLIT,$0
	MOVD	x+0(FP), F0
	MOVD	y+8(FP), F1
	FCLASSD	F0, X5
	FCLASSD	F1, X6

	// -Inf special cases
	MOV	$NegInf, X7
	BEQ	X7, X5, isMinX
	BEQ	X7, X6, isMinY

	// NaN special cases
	MOV	$NaN, X7
	BEQ	X7, X5, isMinX
	BEQ	X7, X6, isMinY

	// normal case
	FMIND	F0, F1, F0
	MOVD	F0, ret+16(FP)
	RET

isMinX: // return x
	MOVD	F0, ret+16(FP)
	RET

isMinY: // return y
	MOVD	F1, ret+16(FP)
	RET

```

// === FILE: references!/go/src/math/dim_s390x.s ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Based on dim_amd64.s

#include "textflag.h"

#define PosInf 0x7FF0000000000000
#define NaN    0x7FF8000000000001
#define NegInf 0xFFF0000000000000

// func ·Max(x, y float64) float64
TEXT ·archMax(SB),NOSPLIT,$0
	// +Inf special cases
	MOVD    $PosInf, R4
	MOVD    x+0(FP), R8
	CMPUBEQ R4, R8, isPosInf
	MOVD    y+8(FP), R9
	CMPUBEQ R4, R9, isPosInf
	// NaN special cases
	MOVD    $~(1<<63), R5 // bit mask
	MOVD    $PosInf, R4
	MOVD    R8, R2
	AND     R5, R2 // x = |x|
	CMPUBLT R4, R2, isMaxNaN
	MOVD    R9, R3
	AND     R5, R3 // y = |y|
	CMPUBLT R4, R3, isMaxNaN
	// ±0 special cases
	OR      R3, R2
	BEQ     isMaxZero

	FMOVD   x+0(FP), F1
	FMOVD   y+8(FP), F2
	FCMPU   F2, F1
	BGT     +3(PC)
	FMOVD   F1, ret+16(FP)
	RET
	FMOVD   F2, ret+16(FP)
	RET
isMaxNaN: // return NaN
	MOVD	$NaN, R4
isPosInf: // return +Inf
	MOVD    R4, ret+16(FP)
	RET
isMaxZero:
	MOVD    $(1<<63), R4 // -0.0
	CMPUBEQ R4, R8, +3(PC)
	MOVD    R8, ret+16(FP) // return 0
	RET
	MOVD    R9, ret+16(FP) // return other 0
	RET

// func archMin(x, y float64) float64
TEXT ·archMin(SB),NOSPLIT,$0
	// -Inf special cases
	MOVD    $NegInf, R4
	MOVD    x+0(FP), R8
	CMPUBEQ R4, R8, isNegInf
	MOVD    y+8(FP), R9
	CMPUBEQ R4, R9, isNegInf
	// NaN special cases
	MOVD    $~(1<<63), R5
	MOVD    $PosInf, R4
	MOVD    R8, R2
	AND     R5, R2 // x = |x|
	CMPUBLT R4, R2, isMinNaN
	MOVD    R9, R3
	AND     R5, R3 // y = |y|
	CMPUBLT R4, R3, isMinNaN
	// ±0 special cases
	OR      R3, R2
	BEQ     isMinZero

	FMOVD   x+0(FP), F1
	FMOVD   y+8(FP), F2
	FCMPU   F2, F1
	BLT     +3(PC)
	FMOVD   F1, ret+16(FP)
	RET
	FMOVD   F2, ret+16(FP)
	RET
isMinNaN: // return NaN
	MOVD	$NaN, R4
isNegInf: // return -Inf
	MOVD    R4, ret+16(FP)
	RET
isMinZero:
	MOVD    $(1<<63), R4 // -0.0
	CMPUBEQ R4, R8, +3(PC)
	MOVD    R9, ret+16(FP) // return other 0
	RET
	MOVD    R8, ret+16(FP) // return -0
	RET


```

// === FILE: references!/go/src/math/erf.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

/*
	Floating-point error function and complementary error function.
*/

// The original C code and the long comment below are
// from FreeBSD's /usr/src/lib/msun/src/s_erf.c and
// came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
//
// double erf(double x)
// double erfc(double x)
//                           x
//                    2      |\
//     erf(x)  =  ---------  | exp(-t*t)dt
//                 sqrt(pi) \|
//                           0
//
//     erfc(x) =  1-erf(x)
//  Note that
//              erf(-x) = -erf(x)
//              erfc(-x) = 2 - erfc(x)
//
// Method:
//      1. For |x| in [0, 0.84375]
//          erf(x)  = x + x*R(x**2)
//          erfc(x) = 1 - erf(x)           if x in [-.84375,0.25]
//                  = 0.5 + ((0.5-x)-x*R)  if x in [0.25,0.84375]
//         where R = P/Q where P is an odd poly of degree 8 and
//         Q is an odd poly of degree 10.
//                                               -57.90
//                      | R - (erf(x)-x)/x | <= 2
//
//
//         Remark. The formula is derived by noting
//          erf(x) = (2/sqrt(pi))*(x - x**3/3 + x**5/10 - x**7/42 + ....)
//         and that
//          2/sqrt(pi) = 1.128379167095512573896158903121545171688
//         is close to one. The interval is chosen because the fix
//         point of erf(x) is near 0.6174 (i.e., erf(x)=x when x is
//         near 0.6174), and by some experiment, 0.84375 is chosen to
//         guarantee the error is less than one ulp for erf.
//
//      2. For |x| in [0.84375,1.25], let s = |x| - 1, and
//         c = 0.84506291151 rounded to single (24 bits)
//              erf(x)  = sign(x) * (c  + P1(s)/Q1(s))
//              erfc(x) = (1-c)  - P1(s)/Q1(s) if x > 0
//                        1+(c+P1(s)/Q1(s))    if x < 0
//              |P1/Q1 - (erf(|x|)-c)| <= 2**-59.06
//         Remark: here we use the taylor series expansion at x=1.
//              erf(1+s) = erf(1) + s*Poly(s)
//                       = 0.845.. + P1(s)/Q1(s)
//         That is, we use rational approximation to approximate
//                      erf(1+s) - (c = (single)0.84506291151)
//         Note that |P1/Q1|< 0.078 for x in [0.84375,1.25]
//         where
//              P1(s) = degree 6 poly in s
//              Q1(s) = degree 6 poly in s
//
//      3. For x in [1.25,1/0.35(~2.857143)],
//              erfc(x) = (1/x)*exp(-x*x-0.5625+R1/S1)
//              erf(x)  = 1 - erfc(x)
//         where
//              R1(z) = degree 7 poly in z, (z=1/x**2)
//              S1(z) = degree 8 poly in z
//
//      4. For x in [1/0.35,28]
//              erfc(x) = (1/x)*exp(-x*x-0.5625+R2/S2) if x > 0
//                      = 2.0 - (1/x)*exp(-x*x-0.5625+R2/S2) if -6<x<0
//                      = 2.0 - tiny            (if x <= -6)
//              erf(x)  = sign(x)*(1.0 - erfc(x)) if x < 6, else
//              erf(x)  = sign(x)*(1.0 - tiny)
//         where
//              R2(z) = degree 6 poly in z, (z=1/x**2)
//              S2(z) = degree 7 poly in z
//
//      Note1:
//         To compute exp(-x*x-0.5625+R/S), let s be a single
//         precision number and s := x; then
//              -x*x = -s*s + (s-x)*(s+x)
//              exp(-x*x-0.5626+R/S) =
//                      exp(-s*s-0.5625)*exp((s-x)*(s+x)+R/S);
//      Note2:
//         Here 4 and 5 make use of the asymptotic series
//                        exp(-x*x)
//              erfc(x) ~ ---------- * ( 1 + Poly(1/x**2) )
//                        x*sqrt(pi)
//         We use rational approximation to approximate
//              g(s)=f(1/x**2) = log(erfc(x)*x) - x*x + 0.5625
//         Here is the error bound for R1/S1 and R2/S2
//              |R1/S1 - f(x)|  < 2**(-62.57)
//              |R2/S2 - f(x)|  < 2**(-61.52)
//
//      5. For inf > x >= 28
//              erf(x)  = sign(x) *(1 - tiny)  (raise inexact)
//              erfc(x) = tiny*tiny (raise underflow) if x > 0
//                      = 2 - tiny if x<0
//
//      7. Special case:
//              erf(0)  = 0, erf(inf)  = 1, erf(-inf) = -1,
//              erfc(0) = 1, erfc(inf) = 0, erfc(-inf) = 2,
//              erfc/erf(NaN) is NaN

const (
	erx = 8.45062911510467529297e-01 // 0x3FEB0AC160000000
	// Coefficients for approximation to  erf in [0, 0.84375]
	efx  = 1.28379167095512586316e-01  // 0x3FC06EBA8214DB69
	efx8 = 1.02703333676410069053e+00  // 0x3FF06EBA8214DB69
	pp0  = 1.28379167095512558561e-01  // 0x3FC06EBA8214DB68
	pp1  = -3.25042107247001499370e-01 // 0xBFD4CD7D691CB913
	pp2  = -2.84817495755985104766e-02 // 0xBF9D2A51DBD7194F
	pp3  = -5.77027029648944159157e-03 // 0xBF77A291236668E4
	pp4  = -2.37630166566501626084e-05 // 0xBEF8EAD6120016AC
	qq1  = 3.97917223959155352819e-01  // 0x3FD97779CDDADC09
	qq2  = 6.50222499887672944485e-02  // 0x3FB0A54C5536CEBA
	qq3  = 5.08130628187576562776e-03  // 0x3F74D022C4D36B0F
	qq4  = 1.32494738004321644526e-04  // 0x3F215DC9221C1A10
	qq5  = -3.96022827877536812320e-06 // 0xBED09C4342A26120
	// Coefficients for approximation to  erf  in [0.84375, 1.25]
	pa0 = -2.36211856075265944077e-03 // 0xBF6359B8BEF77538
	pa1 = 4.14856118683748331666e-01  // 0x3FDA8D00AD92B34D
	pa2 = -3.72207876035701323847e-01 // 0xBFD7D240FBB8C3F1
	pa3 = 3.18346619901161753674e-01  // 0x3FD45FCA805120E4
	pa4 = -1.10894694282396677476e-01 // 0xBFBC63983D3E28EC
	pa5 = 3.54783043256182359371e-02  // 0x3FA22A36599795EB
	pa6 = -2.16637559486879084300e-03 // 0xBF61BF380A96073F
	qa1 = 1.06420880400844228286e-01  // 0x3FBB3E6618EEE323
	qa2 = 5.40397917702171048937e-01  // 0x3FE14AF092EB6F33
	qa3 = 7.18286544141962662868e-02  // 0x3FB2635CD99FE9A7
	qa4 = 1.26171219808761642112e-01  // 0x3FC02660E763351F
	qa5 = 1.36370839120290507362e-02  // 0x3F8BEDC26B51DD1C
	qa6 = 1.19844998467991074170e-02  // 0x3F888B545735151D
	// Coefficients for approximation to  erfc in [1.25, 1/0.35]
	ra0 = -9.86494403484714822705e-03 // 0xBF843412600D6435
	ra1 = -6.93858572707181764372e-01 // 0xBFE63416E4BA7360
	ra2 = -1.05586262253232909814e+01 // 0xC0251E0441B0E726
	ra3 = -6.23753324503260060396e+01 // 0xC04F300AE4CBA38D
	ra4 = -1.62396669462573470355e+02 // 0xC0644CB184282266
	ra5 = -1.84605092906711035994e+02 // 0xC067135CEBCCABB2
	ra6 = -8.12874355063065934246e+01 // 0xC054526557E4D2F2
	ra7 = -9.81432934416914548592e+00 // 0xC023A0EFC69AC25C
	sa1 = 1.96512716674392571292e+01  // 0x4033A6B9BD707687
	sa2 = 1.37657754143519042600e+02  // 0x4061350C526AE721
	sa3 = 4.34565877475229228821e+02  // 0x407B290DD58A1A71
	sa4 = 6.45387271733267880336e+02  // 0x40842B1921EC2868
	sa5 = 4.29008140027567833386e+02  // 0x407AD02157700314
	sa6 = 1.08635005541779435134e+02  // 0x405B28A3EE48AE2C
	sa7 = 6.57024977031928170135e+00  // 0x401A47EF8E484A93
	sa8 = -6.04244152148580987438e-02 // 0xBFAEEFF2EE749A62
	// Coefficients for approximation to  erfc in [1/.35, 28]
	rb0 = -9.86494292470009928597e-03 // 0xBF84341239E86F4A
	rb1 = -7.99283237680523006574e-01 // 0xBFE993BA70C285DE
	rb2 = -1.77579549177547519889e+01 // 0xC031C209555F995A
	rb3 = -1.60636384855821916062e+02 // 0xC064145D43C5ED98
	rb4 = -6.37566443368389627722e+02 // 0xC083EC881375F228
	rb5 = -1.02509513161107724954e+03 // 0xC09004616A2E5992
	rb6 = -4.83519191608651397019e+02 // 0xC07E384E9BDC383F
	sb1 = 3.03380607434824582924e+01  // 0x403E568B261D5190
	sb2 = 3.25792512996573918826e+02  // 0x40745CAE221B9F0A
	sb3 = 1.53672958608443695994e+03  // 0x409802EB189D5118
	sb4 = 3.19985821950859553908e+03  // 0x40A8FFB7688C246A
	sb5 = 2.55305040643316442583e+03  // 0x40A3F219CEDF3BE6
	sb6 = 4.74528541206955367215e+02  // 0x407DA874E79FE763
	sb7 = -2.24409524465858183362e+01 // 0xC03670E242712D62
)

// Erf returns the error function of x.
//
// Special cases are:
//
//	Erf(+Inf) = 1
//	Erf(-Inf) = -1
//	Erf(NaN) = NaN
func Erf(x float64) float64 {
	if haveArchErf {
		return archErf(x)
	}
	return erf(x)
}

func erf(x float64) float64 {
	const (
		VeryTiny = 2.848094538889218e-306 // 0x0080000000000000
		Small    = 1.0 / (1 << 28)        // 2**-28
	)
	// special cases
	switch {
	case IsNaN(x):
		return NaN()
	case IsInf(x, 1):
		return 1
	case IsInf(x, -1):
		return -1
	}
	sign := false
	if x < 0 {
		x = -x
		sign = true
	}
	if x < 0.84375 { // |x| < 0.84375
		var temp float64
		if x < Small { // |x| < 2**-28
			if x < VeryTiny {
				temp = 0.125 * (8.0*x + efx8*x) // avoid underflow
			} else {
				temp = x + efx*x
			}
		} else {
			z := x * x
			r := pp0 + z*(pp1+z*(pp2+z*(pp3+z*pp4)))
			s := 1 + z*(qq1+z*(qq2+z*(qq3+z*(qq4+z*qq5))))
			y := r / s
			temp = x + x*y
		}
		if sign {
			return -temp
		}
		return temp
	}
	if x < 1.25 { // 0.84375 <= |x| < 1.25
		s := x - 1
		P := pa0 + s*(pa1+s*(pa2+s*(pa3+s*(pa4+s*(pa5+s*pa6)))))
		Q := 1 + s*(qa1+s*(qa2+s*(qa3+s*(qa4+s*(qa5+s*qa6)))))
		if sign {
			return -erx - P/Q
		}
		return erx + P/Q
	}
	if x >= 6 { // inf > |x| >= 6
		if sign {
			return -1
		}
		return 1
	}
	s := 1 / (x * x)
	var R, S float64
	if x < 1/0.35 { // |x| < 1 / 0.35  ~ 2.857143
		R = ra0 + s*(ra1+s*(ra2+s*(ra3+s*(ra4+s*(ra5+s*(ra6+s*ra7))))))
		S = 1 + s*(sa1+s*(sa2+s*(sa3+s*(sa4+s*(sa5+s*(sa6+s*(sa7+s*sa8)))))))
	} else { // |x| >= 1 / 0.35  ~ 2.857143
		R = rb0 + s*(rb1+s*(rb2+s*(rb3+s*(rb4+s*(rb5+s*rb6)))))
		S = 1 + s*(sb1+s*(sb2+s*(sb3+s*(sb4+s*(sb5+s*(sb6+s*sb7))))))
	}
	z := Float64frombits(Float64bits(x) & 0xffffffff00000000) // pseudo-single (20-bit) precision x
	r := Exp(-z*z-0.5625) * Exp((z-x)*(z+x)+R/S)
	if sign {
		return r/x - 1
	}
	return 1 - r/x
}

// Erfc returns the complementary error function of x.
//
// Special cases are:
//
//	Erfc(+Inf) = 0
//	Erfc(-Inf) = 2
//	Erfc(NaN) = NaN
func Erfc(x float64) float64 {
	if haveArchErfc {
		return archErfc(x)
	}
	return erfc(x)
}

func erfc(x float64) float64 {
	const Tiny = 1.0 / (1 << 56) // 2**-56
	// special cases
	switch {
	case IsNaN(x):
		return NaN()
	case IsInf(x, 1):
		return 0
	case IsInf(x, -1):
		return 2
	}
	sign := false
	if x < 0 {
		x = -x
		sign = true
	}
	if x < 0.84375 { // |x| < 0.84375
		var temp float64
		if x < Tiny { // |x| < 2**-56
			temp = x
		} else {
			z := x * x
			r := pp0 + z*(pp1+z*(pp2+z*(pp3+z*pp4)))
			s := 1 + z*(qq1+z*(qq2+z*(qq3+z*(qq4+z*qq5))))
			y := r / s
			if x < 0.25 { // |x| < 1/4
				temp = x + x*y
			} else {
				temp = 0.5 + (x*y + (x - 0.5))
			}
		}
		if sign {
			return 1 + temp
		}
		return 1 - temp
	}
	if x < 1.25 { // 0.84375 <= |x| < 1.25
		s := x - 1
		P := pa0 + s*(pa1+s*(pa2+s*(pa3+s*(pa4+s*(pa5+s*pa6)))))
		Q := 1 + s*(qa1+s*(qa2+s*(qa3+s*(qa4+s*(qa5+s*qa6)))))
		if sign {
			return 1 + erx + P/Q
		}
		return 1 - erx - P/Q

	}
	if x < 28 { // |x| < 28
		s := 1 / (x * x)
		var R, S float64
		if x < 1/0.35 { // |x| < 1 / 0.35 ~ 2.857143
			R = ra0 + s*(ra1+s*(ra2+s*(ra3+s*(ra4+s*(ra5+s*(ra6+s*ra7))))))
			S = 1 + s*(sa1+s*(sa2+s*(sa3+s*(sa4+s*(sa5+s*(sa6+s*(sa7+s*sa8)))))))
		} else { // |x| >= 1 / 0.35 ~ 2.857143
			if sign && x > 6 {
				return 2 // x < -6
			}
			R = rb0 + s*(rb1+s*(rb2+s*(rb3+s*(rb4+s*(rb5+s*rb6)))))
			S = 1 + s*(sb1+s*(sb2+s*(sb3+s*(sb4+s*(sb5+s*(sb6+s*sb7))))))
		}
		z := Float64frombits(Float64bits(x) & 0xffffffff00000000) // pseudo-single (20-bit) precision x
		r := Exp(-z*z-0.5625) * Exp((z-x)*(z+x)+R/S)
		if sign {
			return 2 - r/x
		}
		return r / x
	}
	if sign {
		return 2
	}
	return 0
}

```

// === FILE: references!/go/src/math/erf_s390x.s ===
```text
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// Minimax polynomial coefficients and other constants
DATA ·erfrodataL13<> + 0(SB)/8, $0.243673229298474689E+01
DATA ·erfrodataL13<> + 8(SB)/8, $-.654905018503145600E+00
DATA ·erfrodataL13<> + 16(SB)/8, $0.404669310217538718E+01
DATA ·erfrodataL13<> + 24(SB)/8, $-.564189219162765367E+00
DATA ·erfrodataL13<> + 32(SB)/8, $-.200104300906596851E+01
DATA ·erfrodataL13<> + 40(SB)/8, $0.5
DATA ·erfrodataL13<> + 48(SB)/8, $0.144070097650207154E+00
DATA ·erfrodataL13<> + 56(SB)/8, $-.116697735205906191E+00
DATA ·erfrodataL13<> + 64(SB)/8, $0.256847684882319665E-01
DATA ·erfrodataL13<> + 72(SB)/8, $-.510805169106229148E-02
DATA ·erfrodataL13<> + 80(SB)/8, $0.885258164825590267E-03
DATA ·erfrodataL13<> + 88(SB)/8, $-.133861989591931411E-03
DATA ·erfrodataL13<> + 96(SB)/8, $0.178294867340272534E-04
DATA ·erfrodataL13<> + 104(SB)/8, $-.211436095674019218E-05
DATA ·erfrodataL13<> + 112(SB)/8, $0.225503753499344434E-06
DATA ·erfrodataL13<> + 120(SB)/8, $-.218247939190783624E-07
DATA ·erfrodataL13<> + 128(SB)/8, $0.193179206264594029E-08
DATA ·erfrodataL13<> + 136(SB)/8, $-.157440643541715319E-09
DATA ·erfrodataL13<> + 144(SB)/8, $0.118878583237342616E-10
DATA ·erfrodataL13<> + 152(SB)/8, $0.554289288424588473E-13
DATA ·erfrodataL13<> + 160(SB)/8, $-.277649758489502214E-14
DATA ·erfrodataL13<> + 168(SB)/8, $-.839318416990049443E-12
DATA ·erfrodataL13<> + 176(SB)/8, $-2.25
DATA ·erfrodataL13<> + 184(SB)/8, $.12837916709551258632
DATA ·erfrodataL13<> + 192(SB)/8, $1.0
DATA ·erfrodataL13<> + 200(SB)/8, $0.500000000000004237e+00
DATA ·erfrodataL13<> + 208(SB)/8, $1.0
DATA ·erfrodataL13<> + 216(SB)/8, $0.416666664838056960e-01
DATA ·erfrodataL13<> + 224(SB)/8, $0.166666666630345592e+00
DATA ·erfrodataL13<> + 232(SB)/8, $0.138926439368309441e-02
DATA ·erfrodataL13<> + 240(SB)/8, $0.833349307718286047e-02
DATA ·erfrodataL13<> + 248(SB)/8, $-.693147180559945286e+00
DATA ·erfrodataL13<> + 256(SB)/8, $-.144269504088896339e+01
DATA ·erfrodataL13<> + 264(SB)/8, $281475245147134.9375
DATA ·erfrodataL13<> + 272(SB)/8, $0.358256136398192529E+01
DATA ·erfrodataL13<> + 280(SB)/8, $-.554084396500738270E+00
DATA ·erfrodataL13<> + 288(SB)/8, $0.203630123025312046E+02
DATA ·erfrodataL13<> + 296(SB)/8, $-.735750304705934424E+01
DATA ·erfrodataL13<> + 304(SB)/8, $0.250491598091071797E+02
DATA ·erfrodataL13<> + 312(SB)/8, $-.118955882760959931E+02
DATA ·erfrodataL13<> + 320(SB)/8, $0.942903335085524187E+01
DATA ·erfrodataL13<> + 328(SB)/8, $-.564189522219085689E+00
DATA ·erfrodataL13<> + 336(SB)/8, $-.503767199403555540E+01
DATA ·erfrodataL13<> + 344(SB)/8, $0xbbc79ca10c924223
DATA ·erfrodataL13<> + 352(SB)/8, $0.004099975562609307E+01
DATA ·erfrodataL13<> + 360(SB)/8, $-.324434353381296556E+00
DATA ·erfrodataL13<> + 368(SB)/8, $0.945204812084476250E-01
DATA ·erfrodataL13<> + 376(SB)/8, $-.221407443830058214E-01
DATA ·erfrodataL13<> + 384(SB)/8, $0.426072376238804349E-02
DATA ·erfrodataL13<> + 392(SB)/8, $-.692229229127016977E-03
DATA ·erfrodataL13<> + 400(SB)/8, $0.971111253652087188E-04
DATA ·erfrodataL13<> + 408(SB)/8, $-.119752226272050504E-04
DATA ·erfrodataL13<> + 416(SB)/8, $0.131662993588532278E-05
DATA ·erfrodataL13<> + 424(SB)/8, $0.115776482315851236E-07
DATA ·erfrodataL13<> + 432(SB)/8, $-.780118522218151687E-09
DATA ·erfrodataL13<> + 440(SB)/8, $-.130465975877241088E-06
DATA ·erfrodataL13<> + 448(SB)/8, $-0.25
GLOBL ·erfrodataL13<> + 0(SB), RODATA, $456

// Table of log correction terms
DATA ·erftab2066<> + 0(SB)/8, $0.442737824274138381e-01
DATA ·erftab2066<> + 8(SB)/8, $0.263602189790660309e-01
DATA ·erftab2066<> + 16(SB)/8, $0.122565642281703586e-01
DATA ·erftab2066<> + 24(SB)/8, $0.143757052860721398e-02
DATA ·erftab2066<> + 32(SB)/8, $-.651375034121276075e-02
DATA ·erftab2066<> + 40(SB)/8, $-.119317678849450159e-01
DATA ·erftab2066<> + 48(SB)/8, $-.150868749549871069e-01
DATA ·erftab2066<> + 56(SB)/8, $-.161992609578469234e-01
DATA ·erftab2066<> + 64(SB)/8, $-.154492360403337917e-01
DATA ·erftab2066<> + 72(SB)/8, $-.129850717389178721e-01
DATA ·erftab2066<> + 80(SB)/8, $-.892902649276657891e-02
DATA ·erftab2066<> + 88(SB)/8, $-.338202636596794887e-02
DATA ·erftab2066<> + 96(SB)/8, $0.357266307045684762e-02
DATA ·erftab2066<> + 104(SB)/8, $0.118665304327406698e-01
DATA ·erftab2066<> + 112(SB)/8, $0.214434994118118914e-01
DATA ·erftab2066<> + 120(SB)/8, $0.322580645161290314e-01
GLOBL ·erftab2066<> + 0(SB), RODATA, $128

// Table of +/- 1.0
DATA ·erftab12067<> + 0(SB)/8, $1.0
DATA ·erftab12067<> + 8(SB)/8, $-1.0
GLOBL ·erftab12067<> + 0(SB), RODATA, $16

// Erf returns the error function of the argument.
//
// Special cases are:
//      Erf(+Inf) = 1
//      Erf(-Inf) = -1
//      Erf(NaN) = NaN
// The algorithm used is minimax polynomial approximation
// with coefficients determined with a Remez exchange algorithm.

TEXT	·erfAsm(SB), NOSPLIT, $0-16
	FMOVD	x+0(FP), F0
	MOVD	$·erfrodataL13<>+0(SB), R5
	LGDR	F0, R1
	FMOVD	F0, F6
	SRAD	$48, R1
	MOVH	$16383, R3
	RISBGZ	$49, $63, $0, R1, R2
	MOVW	R2, R6
	MOVW	R3, R7
	CMPBGT	R6, R7, L2
	MOVH	$12287, R1
	MOVW	R1, R7
	CMPBLE	R6, R7 ,L12
	MOVH	$16367, R1
	MOVW	R1, R7
	CMPBGT	R6, R7, L5
	FMOVD	448(R5), F4
	FMADD	F0, F0, F4
	FMOVD	440(R5), F3
	WFMDB	V4, V4, V2
	FMOVD	432(R5), F0
	FMOVD	424(R5), F1
	WFMADB	V2, V0, V3, V0
	FMOVD	416(R5), F3
	WFMADB	V2, V1, V3, V1
	FMOVD	408(R5), F5
	FMOVD	400(R5), F3
	WFMADB	V2, V0, V5, V0
	WFMADB	V2, V1, V3, V1
	FMOVD	392(R5), F5
	FMOVD	384(R5), F3
	WFMADB	V2, V0, V5, V0
	WFMADB	V2, V1, V3, V1
	FMOVD	376(R5), F5
	FMOVD	368(R5), F3
	WFMADB	V2, V0, V5, V0
	WFMADB	V2, V1, V3, V1
	FMOVD	360(R5), F5
	FMOVD	352(R5), F3
	WFMADB	V2, V0, V5, V0
	WFMADB	V2, V1, V3, V2
	WFMADB	V4, V0, V2, V0
	WFMADB	V6, V0, V6, V0
L1:
	FMOVD	F0, ret+8(FP)
	RET
L2:
	MOVH	R1, R1
	MOVH	$16407, R3
	SRW	$31, R1, R1
	MOVW	R2, R6
	MOVW	R3, R7
	CMPBLE	R6, R7, L6
	MOVW	R1, R1
	SLD	$3, R1, R1
	MOVD	$·erftab12067<>+0(SB), R3
	WORD    $0x68013000     //ld %f0,0(%r1,%r3)
	MOVH	$32751, R1
	MOVW	R1, R7
	CMPBGT	R6, R7, L7
	FMOVD	344(R5), F2
	FMADD	F2, F0, F0
L7:
	WFCEDBS	V6, V6, V2
	BEQ	L1
	FMOVD	F6, F0
	FMOVD	F0, ret+8(FP)
	RET

L6:
	MOVW	R1, R1
	SLD	$3, R1, R1
	MOVD	$·erftab12067<>+0(SB), R4
	WFMDB	V0, V0, V1
	MOVH	$0x0, R3
	WORD    $0x68014000     //ld %f0,0(%r1,%r4)
	MOVH	$16399, R1
	MOVW	R2, R6
	MOVW	R1, R7
	CMPBGT	R6, R7, L8
	FMOVD	336(R5), F3
	FMOVD	328(R5), F2
	FMOVD	F1, F4
	WFMADB	V1, V2, V3, V2
	WORD	$0xED405140	//adb %f4,.L30-.L13(%r5)
	BYTE	$0x00
	BYTE	$0x1A
	FMOVD	312(R5), F3
	WFMADB	V1, V2, V3, V2
	FMOVD	304(R5), F3
	WFMADB	V1, V4, V3, V4
	FMOVD	296(R5), F3
	WFMADB	V1, V2, V3, V2
	FMOVD	288(R5), F3
	WFMADB	V1, V4, V3, V4
	FMOVD	280(R5), F3
	WFMADB	V1, V2, V3, V2
	FMOVD	272(R5), F3
	WFMADB	V1, V4, V3, V4
L9:
	FMOVD	264(R5), F3
	FMUL	F4, F6
	FMOVD	256(R5), F4
	WFMADB	V1, V4, V3, V4
	FDIV	F6, F2
	LGDR	F4, R1
	FSUB	F3, F4
	FMOVD	248(R5), F6
	WFMSDB	V4, V6, V1, V4
	FMOVD	240(R5), F1
	FMOVD	232(R5), F6
	WFMADB	V4, V6, V1, V6
	FMOVD	224(R5), F1
	FMOVD	216(R5), F3
	WFMADB	V4, V3, V1, V3
	WFMDB	V4, V4, V1
	FMOVD	208(R5), F5
	WFMADB	V6, V1, V3, V6
	FMOVD	200(R5), F3
	MOVH	R1,R1
	WFMADB	V4, V3, V5, V3
	RISBGZ	$57, $60, $3, R1, R2
	WFMADB	V1, V6, V3, V6
	RISBGN	$0, $15, $48, R1, R3
	MOVD	$·erftab2066<>+0(SB), R1
	FMOVD	192(R5), F1
	LDGR	R3, F3
	WORD	$0xED221000	//madb %f2,%f2,0(%r2,%r1)
	BYTE	$0x20
	BYTE	$0x1E
	WFMADB	V4, V6, V1, V4
	FMUL	F3, F2
	FMADD	F4, F2, F0
	FMOVD	F0, ret+8(FP)
	RET
L12:
	FMOVD	184(R5), F0
	WFMADB	V6, V0, V6, V0
	FMOVD	F0, ret+8(FP)
	RET
L5:
	FMOVD	176(R5), F1
	FMADD	F0, F0, F1
	FMOVD	168(R5), F3
	WFMDB	V1, V1, V2
	FMOVD	160(R5), F0
	FMOVD	152(R5), F4
	WFMADB	V2, V0, V3, V0
	FMOVD	144(R5), F3
	WFMADB	V2, V4, V3, V4
	FMOVD	136(R5), F5
	FMOVD	128(R5), F3
	WFMADB	V2, V0, V5, V0
	WFMADB	V2, V4, V3, V4
	FMOVD	120(R5), F5
	FMOVD	112(R5), F3
	WFMADB	V2, V0, V5, V0
	WFMADB	V2, V4, V3, V4
	FMOVD	104(R5), F5
	FMOVD	96(R5), F3
	WFMADB	V2, V0, V5, V0
	WFMADB	V2, V4, V3, V4
	FMOVD	88(R5), F5
	FMOVD	80(R5), F3
	WFMADB	V2, V0, V5, V0
	WFMADB	V2, V4, V3, V4
	FMOVD	72(R5), F5
	FMOVD	64(R5), F3
	WFMADB	V2, V0, V5, V0
	WFMADB	V2, V4, V3, V4
	FMOVD	56(R5), F5
	FMOVD	48(R5), F3
	WFMADB	V2, V0, V5, V0
	WFMADB	V2, V4, V3, V2
	FMOVD	40(R5), F4
	WFMADB	V1, V0, V2, V0
	FMUL	F6, F0
	FMADD	F4, F6, F0
	FMOVD	F0, ret+8(FP)
	RET
L8:
	FMOVD	32(R5), F3
	FMOVD	24(R5), F2
	FMOVD	F1, F4
	WFMADB	V1, V2, V3, V2
	WORD	$0xED405010	//adb %f4,.L68-.L13(%r5)
	BYTE	$0x00
	BYTE	$0x1A
	FMOVD	8(R5), F3
	WFMADB	V1, V2, V3, V2
	FMOVD	·erfrodataL13<>+0(SB), F3
	WFMADB	V1, V4, V3, V4
	BR	L9

```

// === FILE: references!/go/src/math/erfc_s390x.s ===
```text
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

#define Neg2p11 0xC000E147AE147AE1
#define Pos15   0x402E

// Minimax polynomial coefficients and other constants
DATA ·erfcrodataL38<> + 0(SB)/8, $.234875460637085087E-01
DATA ·erfcrodataL38<> + 8(SB)/8, $.234469449299256284E-01
DATA ·erfcrodataL38<> + 16(SB)/8, $-.606918710392844955E-04
DATA ·erfcrodataL38<> + 24(SB)/8, $-.198827088077636213E-04
DATA ·erfcrodataL38<> + 32(SB)/8, $.257805645845475331E-06
DATA ·erfcrodataL38<> + 40(SB)/8, $-.184427218110620284E-09
DATA ·erfcrodataL38<> + 48(SB)/8, $.122408098288933181E-10
DATA ·erfcrodataL38<> + 56(SB)/8, $.484691106751495392E-07
DATA ·erfcrodataL38<> + 64(SB)/8, $-.150147637632890281E-08
DATA ·erfcrodataL38<> + 72(SB)/8, $23.999999999973521625
DATA ·erfcrodataL38<> + 80(SB)/8, $27.226017111108365754
DATA ·erfcrodataL38<> + 88(SB)/8, $-2.0
DATA ·erfcrodataL38<> + 96(SB)/8, $0.100108802034478228E+00
DATA ·erfcrodataL38<> + 104(SB)/8, $0.244588413746558125E+00
DATA ·erfcrodataL38<> + 112(SB)/8, $-.669188879646637174E-01
DATA ·erfcrodataL38<> + 120(SB)/8, $0.151311447000953551E-01
DATA ·erfcrodataL38<> + 128(SB)/8, $-.284720833493302061E-02
DATA ·erfcrodataL38<> + 136(SB)/8, $0.455491239358743212E-03
DATA ·erfcrodataL38<> + 144(SB)/8, $-.631850539280720949E-04
DATA ·erfcrodataL38<> + 152(SB)/8, $0.772532660726086679E-05
DATA ·erfcrodataL38<> + 160(SB)/8, $-.843706007150936940E-06
DATA ·erfcrodataL38<> + 168(SB)/8, $-.735330214904227472E-08
DATA ·erfcrodataL38<> + 176(SB)/8, $0.753002008837084967E-09
DATA ·erfcrodataL38<> + 184(SB)/8, $0.832482036660624637E-07
DATA ·erfcrodataL38<> + 192(SB)/8, $-0.75
DATA ·erfcrodataL38<> + 200(SB)/8, $.927765678007128609E-01
DATA ·erfcrodataL38<> + 208(SB)/8, $.903621209344751506E-01
DATA ·erfcrodataL38<> + 216(SB)/8, $-.344203375025257265E-02
DATA ·erfcrodataL38<> + 224(SB)/8, $-.869243428221791329E-03
DATA ·erfcrodataL38<> + 232(SB)/8, $.174699813107105603E-03
DATA ·erfcrodataL38<> + 240(SB)/8, $.649481036316130000E-05
DATA ·erfcrodataL38<> + 248(SB)/8, $-.895265844897118382E-05
DATA ·erfcrodataL38<> + 256(SB)/8, $.135970046909529513E-05
DATA ·erfcrodataL38<> + 264(SB)/8, $.277617717014748015E-06
DATA ·erfcrodataL38<> + 272(SB)/8, $.810628018408232910E-08
DATA ·erfcrodataL38<> + 280(SB)/8, $.210430084693497985E-07
DATA ·erfcrodataL38<> + 288(SB)/8, $-.342138077525615091E-08
DATA ·erfcrodataL38<> + 296(SB)/8, $-.165467946798610800E-06
DATA ·erfcrodataL38<> + 304(SB)/8, $5.999999999988412824
DATA ·erfcrodataL38<> + 312(SB)/8, $.468542210149072159E-01
DATA ·erfcrodataL38<> + 320(SB)/8, $.465343528567604256E-01
DATA ·erfcrodataL38<> + 328(SB)/8, $-.473338083650201733E-03
DATA ·erfcrodataL38<> + 336(SB)/8, $-.147220659069079156E-03
DATA ·erfcrodataL38<> + 344(SB)/8, $.755284723554388339E-05
DATA ·erfcrodataL38<> + 352(SB)/8, $.116158570631428789E-05
DATA ·erfcrodataL38<> + 360(SB)/8, $-.155445501551602389E-06
DATA ·erfcrodataL38<> + 368(SB)/8, $-.616940119847805046E-10
DATA ·erfcrodataL38<> + 376(SB)/8, $-.728705590727563158E-10
DATA ·erfcrodataL38<> + 384(SB)/8, $-.983452460354586779E-08
DATA ·erfcrodataL38<> + 392(SB)/8, $.365156164194346316E-08
DATA ·erfcrodataL38<> + 400(SB)/8, $11.999999999996530775
DATA ·erfcrodataL38<> + 408(SB)/8, $0.467773498104726584E-02
DATA ·erfcrodataL38<> + 416(SB)/8, $0.206669853540920535E-01
DATA ·erfcrodataL38<> + 424(SB)/8, $0.413339707081841473E-01
DATA ·erfcrodataL38<> + 432(SB)/8, $0.482229658262131320E-01
DATA ·erfcrodataL38<> + 440(SB)/8, $0.344449755901841897E-01
DATA ·erfcrodataL38<> + 448(SB)/8, $0.130890907240765465E-01
DATA ·erfcrodataL38<> + 456(SB)/8, $-.459266344100642687E-03
DATA ·erfcrodataL38<> + 464(SB)/8, $-.337888800856913728E-02
DATA ·erfcrodataL38<> + 472(SB)/8, $-.159103061687062373E-02
DATA ·erfcrodataL38<> + 480(SB)/8, $-.501128905515922644E-04
DATA ·erfcrodataL38<> + 488(SB)/8, $0.262775855852903132E-03
DATA ·erfcrodataL38<> + 496(SB)/8, $0.103860982197462436E-03
DATA ·erfcrodataL38<> + 504(SB)/8, $-.548835785414200775E-05
DATA ·erfcrodataL38<> + 512(SB)/8, $-.157075054646618214E-04
DATA ·erfcrodataL38<> + 520(SB)/8, $-.480056366276045110E-05
DATA ·erfcrodataL38<> + 528(SB)/8, $0.198263013759701555E-05
DATA ·erfcrodataL38<> + 536(SB)/8, $-.224394262958888780E-06
DATA ·erfcrodataL38<> + 544(SB)/8, $-.321853693146683428E-06
DATA ·erfcrodataL38<> + 552(SB)/8, $0.445073894984683537E-07
DATA ·erfcrodataL38<> + 560(SB)/8, $0.660425940000555729E-06
DATA ·erfcrodataL38<> + 568(SB)/8, $2.0
DATA ·erfcrodataL38<> + 576(SB)/8, $8.63616855509444462538e-78
DATA ·erfcrodataL38<> + 584(SB)/8, $1.00000000000000222044
DATA ·erfcrodataL38<> + 592(SB)/8, $0.500000000000004237e+00
DATA ·erfcrodataL38<> + 600(SB)/8, $0.416666664838056960e-01
DATA ·erfcrodataL38<> + 608(SB)/8, $0.166666666630345592e+00
DATA ·erfcrodataL38<> + 616(SB)/8, $0.138926439368309441e-02
DATA ·erfcrodataL38<> + 624(SB)/8, $0.833349307718286047e-02
DATA ·erfcrodataL38<> + 632(SB)/8, $-.693147180558298714e+00
DATA ·erfcrodataL38<> + 640(SB)/8, $-.164659495826017651e-11
DATA ·erfcrodataL38<> + 648(SB)/8, $.179001151181866548E+00
DATA ·erfcrodataL38<> + 656(SB)/8, $-.144269504088896339e+01
DATA ·erfcrodataL38<> + 664(SB)/8, $+281475245147134.9375
DATA ·erfcrodataL38<> + 672(SB)/8, $.163116780021877404E+00
DATA ·erfcrodataL38<> + 680(SB)/8, $-.201574395828120710E-01
DATA ·erfcrodataL38<> + 688(SB)/8, $-.185726336009394125E-02
DATA ·erfcrodataL38<> + 696(SB)/8, $.199349204957273749E-02
DATA ·erfcrodataL38<> + 704(SB)/8, $-.554902415532606242E-03
DATA ·erfcrodataL38<> + 712(SB)/8, $-.638914789660242846E-05
DATA ·erfcrodataL38<> + 720(SB)/8, $-.424441522653742898E-04
DATA ·erfcrodataL38<> + 728(SB)/8, $.827967511921486190E-04
DATA ·erfcrodataL38<> + 736(SB)/8, $.913965446284062654E-05
DATA ·erfcrodataL38<> + 744(SB)/8, $.277344791076320853E-05
DATA ·erfcrodataL38<> + 752(SB)/8, $-.467239678927239526E-06
DATA ·erfcrodataL38<> + 760(SB)/8, $.344814065920419986E-07
DATA ·erfcrodataL38<> + 768(SB)/8, $-.366013491552527132E-05
DATA ·erfcrodataL38<> + 776(SB)/8, $.181242810023783439E-05
DATA ·erfcrodataL38<> + 784(SB)/8, $2.999999999991234567
DATA ·erfcrodataL38<> + 792(SB)/8, $1.0
GLOBL ·erfcrodataL38<> + 0(SB), RODATA, $800

// Table of log correction terms
DATA ·erfctab2069<> + 0(SB)/8, $0.442737824274138381e-01
DATA ·erfctab2069<> + 8(SB)/8, $0.263602189790660309e-01
DATA ·erfctab2069<> + 16(SB)/8, $0.122565642281703586e-01
DATA ·erfctab2069<> + 24(SB)/8, $0.143757052860721398e-02
DATA ·erfctab2069<> + 32(SB)/8, $-.651375034121276075e-02
DATA ·erfctab2069<> + 40(SB)/8, $-.119317678849450159e-01
DATA ·erfctab2069<> + 48(SB)/8, $-.150868749549871069e-01
DATA ·erfctab2069<> + 56(SB)/8, $-.161992609578469234e-01
DATA ·erfctab2069<> + 64(SB)/8, $-.154492360403337917e-01
DATA ·erfctab2069<> + 72(SB)/8, $-.129850717389178721e-01
DATA ·erfctab2069<> + 80(SB)/8, $-.892902649276657891e-02
DATA ·erfctab2069<> + 88(SB)/8, $-.338202636596794887e-02
DATA ·erfctab2069<> + 96(SB)/8, $0.357266307045684762e-02
DATA ·erfctab2069<> + 104(SB)/8, $0.118665304327406698e-01
DATA ·erfctab2069<> + 112(SB)/8, $0.214434994118118914e-01
DATA ·erfctab2069<> + 120(SB)/8, $0.322580645161290314e-01
GLOBL ·erfctab2069<> + 0(SB), RODATA, $128

// Erfc returns the complementary error function of the argument.
//
// Special cases are:
//      Erfc(+Inf) = 0
//      Erfc(-Inf) = 2
//      Erfc(NaN) = NaN
// The algorithm used is minimax polynomial approximation
// with coefficients determined with a Remez exchange algorithm.
// This assembly implementation handles inputs in the range [-2.11, +15].
// For all other inputs we call the generic Go implementation.

TEXT	·erfcAsm(SB), NOSPLIT|NOFRAME, $0-16
	MOVD	x+0(FP), R1
	MOVD	$Neg2p11, R2
	CMPUBGT	R1, R2, usego

	FMOVD	x+0(FP), F0
	MOVD	$·erfcrodataL38<>+0(SB), R9
	FMOVD	F0, F2
	SRAD	$48, R1
	MOVH	R1, R2
	ANDW	$0x7FFF, R1
	MOVH	$Pos15, R3
	CMPW	R1, R3
	BGT	usego
	MOVH	$0x3FFF, R3
	MOVW	R1, R6
	MOVW	R3, R7
	CMPBGT	R6, R7, L2
	MOVH	$0x3FEF, R3
	MOVW	R3, R7
	CMPBGT	R6, R7, L3
	MOVH	$0x2FFF, R2
	MOVW	R2, R7
	CMPBGT	R6, R7, L4
	FMOVD	792(R9), F0
	WFSDB	V2, V0, V2
	FMOVD	F2, ret+8(FP)
	RET

L2:
	LTDBR	F0, F0
	MOVH	$0x0, R4
	BLTU	L3
	FMOVD	F0, F1
L9:
	MOVH	$0x400F, R3
	MOVW	R1, R6
	MOVW	R3, R7
	CMPBGT	R6, R7, L10
	FMOVD	784(R9), F3
	FSUB	F1, F3
	VLEG	$0, 776(R9), V20
	WFDDB	V1, V3, V6
	VLEG	$0, 768(R9), V18
	FMOVD	760(R9), F7
	FMOVD	752(R9), F5
	VLEG	$0, 744(R9), V16
	FMOVD	736(R9), F3
	FMOVD	728(R9), F2
	FMOVD	720(R9), F4
	WFMDB	V6, V6, V1
	FMUL	F0, F0
	MOVH	$0x0, R3
	WFMADB	V1, V7, V20, V7
	WFMADB	V1, V5, V18, V5
	WFMADB	V1, V7, V16, V7
	WFMADB	V1, V5, V3, V5
	WFMADB	V1, V7, V4, V7
	WFMADB	V1, V5, V2, V5
	FMOVD	712(R9), F2
	WFMADB	V1, V7, V2, V7
	FMOVD	704(R9), F2
	WFMADB	V1, V5, V2, V5
	FMOVD	696(R9), F2
	WFMADB	V1, V7, V2, V7
	FMOVD	688(R9), F2
	MOVH	$0x0, R1
	WFMADB	V1, V5, V2, V5
	FMOVD	680(R9), F2
	WFMADB	V1, V7, V2, V7
	FMOVD	672(R9), F2
	WFMADB	V1, V5, V2, V1
	FMOVD	664(R9), F3
	WFMADB	V6, V7, V1, V7
	FMOVD	656(R9), F5
	FMOVD	648(R9), F2
	WFMADB	V0, V5, V3, V5
	WFMADB	V6, V7, V2, V7
L11:
	LGDR	F5, R6
	WFSDB	V0, V0, V2
	WORD	$0xED509298	//sdb	%f5,.L55-.L38(%r9)
	BYTE	$0x00
	BYTE	$0x1B
	FMOVD	640(R9), F6
	FMOVD	632(R9), F4
	WFMSDB	V5, V6, V2, V6
	WFMSDB	V5, V4, V0, V4
	FMOVD	624(R9), F2
	FADD	F6, F4
	FMOVD	616(R9), F0
	FMOVD	608(R9), F6
	WFMADB	V4, V0, V2, V0
	FMOVD	600(R9), F3
	WFMDB	V4, V4, V2
	MOVH	R6,R6
	ADD	R6, R3
	WFMADB	V4, V3, V6, V3
	FMOVD	592(R9), F6
	WFMADB	V0, V2, V3, V0
	FMOVD	584(R9), F3
	WFMADB	V4, V6, V3, V6
	RISBGZ	$57, $60, $3, R3, R12
	WFMADB	V2, V0, V6, V0
	MOVD	$·erfctab2069<>+0(SB), R5
	WORD	$0x682C5000	//ld	%f2,0(%r12,%r5)
	FMADD	F2, F4, F4
	RISBGN	$0, $15, $48, R3, R4
	WFMADB	V4, V0, V2, V4
	LDGR	R4, F2
	FMADD	F4, F2, F2
	MOVW	R2, R6
	CMPBLE	R6, $0, L20
	MOVW	R1, R6
	CMPBEQ	R6, $0, L21
	WORD	$0xED709240	//mdb	%f7,.L66-.L38(%r9)
	BYTE	$0x00
	BYTE	$0x1C
L21:
	FMUL	F7, F2
L1:
	FMOVD	F2, ret+8(FP)
	RET
L3:
	LTDBR	F0, F0
	BLTU	L30
	FMOVD	568(R9), F2
	WFSDB	V0, V2, V0
L8:
	WFMDB	V0, V0, V4
	FMOVD	560(R9), F2
	FMOVD	552(R9), F6
	FMOVD	544(R9), F1
	WFMADB	V4, V6, V2, V6
	FMOVD	536(R9), F2
	WFMADB	V4, V1, V2, V1
	FMOVD	528(R9), F3
	FMOVD	520(R9), F2
	WFMADB	V4, V6, V3, V6
	WFMADB	V4, V1, V2, V1
	FMOVD	512(R9), F3
	FMOVD	504(R9), F2
	WFMADB	V4, V6, V3, V6
	WFMADB	V4, V1, V2, V1
	FMOVD	496(R9), F3
	FMOVD	488(R9), F2
	WFMADB	V4, V6, V3, V6
	WFMADB	V4, V1, V2, V1
	FMOVD	480(R9), F3
	FMOVD	472(R9), F2
	WFMADB	V4, V6, V3, V6
	WFMADB	V4, V1, V2, V1
	FMOVD	464(R9), F3
	FMOVD	456(R9), F2
	WFMADB	V4, V6, V3, V6
	WFMADB	V4, V1, V2, V1
	FMOVD	448(R9), F3
	FMOVD	440(R9), F2
	WFMADB	V4, V6, V3, V6
	WFMADB	V4, V1, V2, V1
	FMOVD	432(R9), F3
	FMOVD	424(R9), F2
	WFMADB	V4, V6, V3, V6
	WFMADB	V4, V1, V2, V1
	FMOVD	416(R9), F3
	FMOVD	408(R9), F2
	WFMADB	V4, V6, V3, V6
	FMADD	F1, F4, F2
	FMADD	F6, F0, F2
	MOVW	R2, R6
	CMPBGE	R6, $0, L1
	FMOVD	568(R9), F0
	WFSDB	V2, V0, V2
	BR	L1
L10:
	MOVH	$0x401F, R3
	MOVW	R1, R6
	MOVW	R3, R7
	CMPBLE	R6, R7, L36
	MOVH	$0x402F, R3
	MOVW	R3, R7
	CMPBGT	R6, R7, L13
	FMOVD	400(R9), F3
	FSUB	F1, F3
	VLEG	$0, 392(R9), V20
	WFDDB	V1, V3, V6
	VLEG	$0, 384(R9), V18
	FMOVD	376(R9), F2
	FMOVD	368(R9), F4
	VLEG	$0, 360(R9), V16
	FMOVD	352(R9), F7
	FMOVD	344(R9), F3
	FMUL	F0, F0
	WFMDB	V6, V6, V1
	FMOVD	656(R9), F5
	MOVH	$0x0, R3
	WFMADB	V1, V2, V20, V2
	WFMADB	V1, V4, V18, V4
	WFMADB	V1, V2, V16, V2
	WFMADB	V1, V4, V7, V4
	WFMADB	V1, V2, V3, V2
	FMOVD	336(R9), F3
	WFMADB	V1, V4, V3, V4
	FMOVD	328(R9), F3
	WFMADB	V1, V2, V3, V2
	FMOVD	320(R9), F3
	WFMADB	V1, V4, V3, V1
	FMOVD	312(R9), F7
	WFMADB	V6, V2, V1, V2
	MOVH	$0x0, R1
	FMOVD	664(R9), F3
	FMADD	F2, F6, F7
	WFMADB	V0, V5, V3, V5
	BR	L11
L35:
	LCDBR	F0, F1
	BR	L9
L36:
	FMOVD	304(R9), F3
	FSUB	F1, F3
	VLEG	$0, 296(R9), V20
	WFDDB	V1, V3, V6
	FMOVD	288(R9), F5
	FMOVD	280(R9), F1
	FMOVD	272(R9), F2
	VLEG	$0, 264(R9), V18
	VLEG	$0, 256(R9), V16
	FMOVD	248(R9), F3
	FMOVD	240(R9), F4
	WFMDB	V6, V6, V7
	FMUL	F0, F0
	MOVH	$0x0, R3
	FMADD	F5, F7, F1
	WFMADB	V7, V2, V20, V2
	WFMADB	V7, V1, V18, V1
	WFMADB	V7, V2, V16, V2
	WFMADB	V7, V1, V3, V1
	WFMADB	V7, V2, V4, V2
	FMOVD	232(R9), F4
	WFMADB	V7, V1, V4, V1
	FMOVD	224(R9), F4
	WFMADB	V7, V2, V4, V2
	FMOVD	216(R9), F4
	WFMADB	V7, V1, V4, V1
	FMOVD	208(R9), F4
	MOVH	$0x0, R1
	WFMADB	V7, V2, V4, V7
	FMOVD	656(R9), F5
	WFMADB	V6, V1, V7, V1
	FMOVD	664(R9), F3
	FMOVD	200(R9), F7
	WFMADB	V0, V5, V3, V5
	FMADD	F1, F6, F7
	BR	L11
L4:
	FMOVD	192(R9), F1
	FMADD	F0, F0, F1
	FMOVD	184(R9), F3
	WFMDB	V1, V1, V0
	FMOVD	176(R9), F4
	FMOVD	168(R9), F6
	WFMADB	V0, V4, V3, V4
	FMOVD	160(R9), F3
	WFMADB	V0, V6, V3, V6
	FMOVD	152(R9), F5
	FMOVD	144(R9), F3
	WFMADB	V0, V4, V5, V4
	WFMADB	V0, V6, V3, V6
	FMOVD	136(R9), F5
	FMOVD	128(R9), F3
	WFMADB	V0, V4, V5, V4
	WFMADB	V0, V6, V3, V6
	FMOVD	120(R9), F5
	FMOVD	112(R9), F3
	WFMADB	V0, V4, V5, V4
	WFMADB	V0, V6, V3, V6
	FMOVD	104(R9), F5
	FMOVD	96(R9), F3
	WFMADB	V0, V4, V5, V4
	WFMADB	V0, V6, V3, V0
	FMOVD	F2, F6
	FMADD	F4, F1, F0
	WORD	$0xED609318	//sdb	%f6,.L39-.L38(%r9)
	BYTE	$0x00
	BYTE	$0x1B
	WFMSDB	V2, V0, V6, V2
	FMOVD	F2, ret+8(FP)
	RET
L30:
	WORD	$0xED009238	//adb	%f0,.L67-.L38(%r9)
	BYTE	$0x00
	BYTE	$0x1A
	BR	L8
L20:
	FMOVD	88(R9), F0
	WFMADB	V7, V2, V0, V2
	LCDBR	F2, F2
	FMOVD	F2, ret+8(FP)
	RET
L13:
	MOVH	$0x403A, R3
	MOVW	R1, R6
	MOVW	R3, R7
	CMPBLE	R6, R7, L4
	WORD	$0xED109050	//cdb	%f1,.L128-.L38(%r9)
	BYTE	$0x00
	BYTE	$0x19
	BGE	L37
	BVS	L37
	FMOVD	72(R9), F6
	FSUB	F1, F6
	MOVH	$0x1000, R3
	FDIV	F1, F6
	MOVH	$0x1000, R1
L17:
	WFMDB	V6, V6, V1
	FMOVD	64(R9), F2
	FMOVD	56(R9), F4
	FMOVD	48(R9), F3
	WFMADB	V1, V3, V2, V3
	FMOVD	40(R9), F2
	WFMADB	V1, V2, V4, V2
	FMOVD	32(R9), F4
	WFMADB	V1, V3, V4, V3
	FMOVD	24(R9), F4
	WFMADB	V1, V2, V4, V2
	FMOVD	16(R9), F4
	WFMADB	V1, V3, V4, V3
	FMOVD	8(R9), F4
	WFMADB	V1, V2, V4, V1
	FMUL	F0, F0
	WFMADB	V3, V6, V1, V3
	FMOVD	656(R9), F5
	FMOVD	664(R9), F4
	FMOVD	0(R9), F7
	WFMADB	V0, V5, V4, V5
	FMADD	F6, F3, F7
	BR	L11
L14:
	FMOVD	72(R9), F6
	FSUB	F1, F6
	MOVH	$0x403A, R3
	FDIV	F1, F6
	MOVW	R1, R6
	MOVW	R3, R7
	CMPBEQ	R6, R7, L23
	MOVH	$0x0, R3
	MOVH	$0x0, R1
	BR	L17
L37:
	WFCEDBS	V0, V0, V0
	BVS	L1
	MOVW	R2, R6
	CMPBLE	R6, $0, L18
	MOVH	$0x7FEF, R2
	MOVW	R1, R6
	MOVW	R2, R7
	CMPBGT	R6, R7, L24

	WORD	$0xA5400010	//iihh	%r4,16
	LDGR	R4, F2
	FMUL	F2, F2
	BR	L1
L23:
	MOVH	$0x1000, R3
	MOVH	$0x1000, R1
	BR	L17
L24:
	FMOVD	$0, F2
	BR	L1
L18:
	MOVH	$0x7FEF, R2
	MOVW	R1, R6
	MOVW	R2, R7
	CMPBGT	R6, R7, L25
	WORD	$0xA5408010	//iihh	%r4,32784
	FMOVD	568(R9), F2
	LDGR	R4, F0
	FMADD	F2, F0, F2
	BR	L1
L25:
	FMOVD	568(R9), F2
	BR	L1
usego:
	BR	·erfc(SB)

```

// === FILE: references!/go/src/math/erfinv.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

/*
	Inverse of the floating-point error function.
*/

// This implementation is based on the rational approximation
// of percentage points of normal distribution available from
// https://www.jstor.org/stable/2347330.

const (
	// Coefficients for approximation to erf in |x| <= 0.85
	a0 = 1.1975323115670912564578e0
	a1 = 4.7072688112383978012285e1
	a2 = 6.9706266534389598238465e2
	a3 = 4.8548868893843886794648e3
	a4 = 1.6235862515167575384252e4
	a5 = 2.3782041382114385731252e4
	a6 = 1.1819493347062294404278e4
	a7 = 8.8709406962545514830200e2
	b0 = 1.0000000000000000000e0
	b1 = 4.2313330701600911252e1
	b2 = 6.8718700749205790830e2
	b3 = 5.3941960214247511077e3
	b4 = 2.1213794301586595867e4
	b5 = 3.9307895800092710610e4
	b6 = 2.8729085735721942674e4
	b7 = 5.2264952788528545610e3
	// Coefficients for approximation to erf in 0.85 < |x| <= 1-2*exp(-25)
	c0 = 1.42343711074968357734e0
	c1 = 4.63033784615654529590e0
	c2 = 5.76949722146069140550e0
	c3 = 3.64784832476320460504e0
	c4 = 1.27045825245236838258e0
	c5 = 2.41780725177450611770e-1
	c6 = 2.27238449892691845833e-2
	c7 = 7.74545014278341407640e-4
	d0 = 1.4142135623730950488016887e0
	d1 = 2.9036514445419946173133295e0
	d2 = 2.3707661626024532365971225e0
	d3 = 9.7547832001787427186894837e-1
	d4 = 2.0945065210512749128288442e-1
	d5 = 2.1494160384252876777097297e-2
	d6 = 7.7441459065157709165577218e-4
	d7 = 1.4859850019840355905497876e-9
	// Coefficients for approximation to erf in 1-2*exp(-25) < |x| < 1
	e0 = 6.65790464350110377720e0
	e1 = 5.46378491116411436990e0
	e2 = 1.78482653991729133580e0
	e3 = 2.96560571828504891230e-1
	e4 = 2.65321895265761230930e-2
	e5 = 1.24266094738807843860e-3
	e6 = 2.71155556874348757815e-5
	e7 = 2.01033439929228813265e-7
	f0 = 1.414213562373095048801689e0
	f1 = 8.482908416595164588112026e-1
	f2 = 1.936480946950659106176712e-1
	f3 = 2.103693768272068968719679e-2
	f4 = 1.112800997078859844711555e-3
	f5 = 2.611088405080593625138020e-5
	f6 = 2.010321207683943062279931e-7
	f7 = 2.891024605872965461538222e-15
)

// Erfinv returns the inverse error function of x.
//
// Special cases are:
//
//	Erfinv(1) = +Inf
//	Erfinv(-1) = -Inf
//	Erfinv(x) = NaN if x < -1 or x > 1
//	Erfinv(NaN) = NaN
func Erfinv(x float64) float64 {
	// special cases
	if IsNaN(x) || x <= -1 || x >= 1 {
		if x == -1 || x == 1 {
			return Inf(int(x))
		}
		return NaN()
	}

	sign := false
	if x < 0 {
		x = -x
		sign = true
	}

	var ans float64
	if x <= 0.85 { // |x| <= 0.85
		r := 0.180625 - 0.25*x*x
		z1 := ((((((a7*r+a6)*r+a5)*r+a4)*r+a3)*r+a2)*r+a1)*r + a0
		z2 := ((((((b7*r+b6)*r+b5)*r+b4)*r+b3)*r+b2)*r+b1)*r + b0
		ans = (x * z1) / z2
	} else {
		var z1, z2 float64
		r := Sqrt(Ln2 - Log(1.0-x))
		if r <= 5.0 {
			r -= 1.6
			z1 = ((((((c7*r+c6)*r+c5)*r+c4)*r+c3)*r+c2)*r+c1)*r + c0
			z2 = ((((((d7*r+d6)*r+d5)*r+d4)*r+d3)*r+d2)*r+d1)*r + d0
		} else {
			r -= 5.0
			z1 = ((((((e7*r+e6)*r+e5)*r+e4)*r+e3)*r+e2)*r+e1)*r + e0
			z2 = ((((((f7*r+f6)*r+f5)*r+f4)*r+f3)*r+f2)*r+f1)*r + f0
		}
		ans = z1 / z2
	}

	if sign {
		return -ans
	}
	return ans
}

// Erfcinv returns the inverse of [Erfc](x).
//
// Special cases are:
//
//	Erfcinv(0) = +Inf
//	Erfcinv(2) = -Inf
//	Erfcinv(x) = NaN if x < 0 or x > 2
//	Erfcinv(NaN) = NaN
func Erfcinv(x float64) float64 {
	return Erfinv(1 - x)
}

```

// === FILE: references!/go/src/math/exp.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// Exp returns e**x, the base-e exponential of x.
//
// Special cases are:
//
//	Exp(+Inf) = +Inf
//	Exp(NaN) = NaN
//
// Very large values overflow to 0 or +Inf.
// Very small values underflow to 1.
func Exp(x float64) float64 {
	if haveArchExp {
		return archExp(x)
	}
	return exp(x)
}

// The original C code, the long comment, and the constants
// below are from FreeBSD's /usr/src/lib/msun/src/e_exp.c
// and came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 2004 by Sun Microsystems, Inc. All rights reserved.
//
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
//
// exp(x)
// Returns the exponential of x.
//
// Method
//   1. Argument reduction:
//      Reduce x to an r so that |r| <= 0.5*ln2 ~ 0.34658.
//      Given x, find r and integer k such that
//
//               x = k*ln2 + r,  |r| <= 0.5*ln2.
//
//      Here r will be represented as r = hi-lo for better
//      accuracy.
//
//   2. Approximation of exp(r) by a special rational function on
//      the interval [0,0.34658]:
//      Write
//          R(r**2) = r*(exp(r)+1)/(exp(r)-1) = 2 + r*r/6 - r**4/360 + ...
//      We use a special Remez algorithm on [0,0.34658] to generate
//      a polynomial of degree 5 to approximate R. The maximum error
//      of this polynomial approximation is bounded by 2**-59. In
//      other words,
//          R(z) ~ 2.0 + P1*z + P2*z**2 + P3*z**3 + P4*z**4 + P5*z**5
//      (where z=r*r, and the values of P1 to P5 are listed below)
//      and
//          |                  5          |     -59
//          | 2.0+P1*z+...+P5*z   -  R(z) | <= 2
//          |                             |
//      The computation of exp(r) thus becomes
//                             2*r
//              exp(r) = 1 + -------
//                            R - r
//                                 r*R1(r)
//                     = 1 + r + ----------- (for better accuracy)
//                                2 - R1(r)
//      where
//                               2       4             10
//              R1(r) = r - (P1*r  + P2*r  + ... + P5*r   ).
//
//   3. Scale back to obtain exp(x):
//      From step 1, we have
//         exp(x) = 2**k * exp(r)
//
// Special cases:
//      exp(INF) is INF, exp(NaN) is NaN;
//      exp(-INF) is 0, and
//      for finite argument, only exp(0)=1 is exact.
//
// Accuracy:
//      according to an error analysis, the error is always less than
//      1 ulp (unit in the last place).
//
// Misc. info.
//      For IEEE double
//          if x >  7.09782712893383973096e+02 then exp(x) overflow
//          if x < -7.45133219101941108420e+02 then exp(x) underflow
//
// Constants:
// The hexadecimal values are the intended ones for the following
// constants. The decimal values may be used, provided that the
// compiler will convert from decimal to binary accurately enough
// to produce the hexadecimal values shown.

func exp(x float64) float64 {
	const (
		Ln2Hi = 6.93147180369123816490e-01
		Ln2Lo = 1.90821492927058770002e-10
		Log2e = 1.44269504088896338700e+00

		Overflow  = 7.09782712893383973096e+02
		Underflow = -7.45133219101941108420e+02
		NearZero  = 1.0 / (1 << 28) // 2**-28
	)

	// special cases
	switch {
	case IsNaN(x):
		return x
	case x > Overflow: // handles case where x is +∞
		return Inf(1)
	case x < Underflow: // handles case where x is -∞
		return 0
	case -NearZero < x && x < NearZero:
		return 1 + x
	}

	// reduce; computed as r = hi - lo for extra precision.
	var k int
	switch {
	case x < 0:
		k = int(Log2e*x - 0.5)
	case x > 0:
		k = int(Log2e*x + 0.5)
	}
	hi := x - float64(k)*Ln2Hi
	lo := float64(k) * Ln2Lo

	// compute
	return expmulti(hi, lo, k)
}

// Exp2 returns 2**x, the base-2 exponential of x.
//
// Special cases are the same as [Exp].
func Exp2(x float64) float64 {
	if haveArchExp2 {
		return archExp2(x)
	}
	return exp2(x)
}

func exp2(x float64) float64 {
	const (
		Ln2Hi = 6.93147180369123816490e-01
		Ln2Lo = 1.90821492927058770002e-10

		Overflow  = 1.0239999999999999e+03
		Underflow = -1.0740e+03
	)

	// special cases
	switch {
	case IsNaN(x):
		return x
	case x > Overflow: // handles case where x is +∞
		return Inf(1)
	case x < Underflow: // handles case where x is -∞
		return 0
	}

	// argument reduction; x = r×lg(e) + k with |r| ≤ ln(2)/2.
	// computed as r = hi - lo for extra precision.
	var k int
	switch {
	case x > 0:
		k = int(x + 0.5)
	case x < 0:
		k = int(x - 0.5)
	}
	t := x - float64(k)
	hi := t * Ln2Hi
	lo := -t * Ln2Lo

	// compute
	return expmulti(hi, lo, k)
}

// exp1 returns e**r × 2**k where r = hi - lo and |r| ≤ ln(2)/2.
func expmulti(hi, lo float64, k int) float64 {
	const (
		P1 = 1.66666666666666657415e-01  /* 0x3FC55555; 0x55555555 */
		P2 = -2.77777777770155933842e-03 /* 0xBF66C16C; 0x16BEBD93 */
		P3 = 6.61375632143793436117e-05  /* 0x3F11566A; 0xAF25DE2C */
		P4 = -1.65339022054652515390e-06 /* 0xBEBBBD41; 0xC5D26BF1 */
		P5 = 4.13813679705723846039e-08  /* 0x3E663769; 0x72BEA4D0 */
	)

	r := hi - lo
	t := r * r
	c := r - t*(P1+t*(P2+t*(P3+t*(P4+t*P5))))
	y := 1 - ((lo - (r*c)/(2-c)) - hi)
	// TODO(rsc): make sure Ldexp can handle boundary k
	return Ldexp(y, k)
}

```

// === FILE: references!/go/src/math/exp2_asm.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build arm64 || loong64 || riscv64

package math

const haveArchExp2 = true

func archExp2(x float64) float64

```

// === FILE: references!/go/src/math/exp2_noasm.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !arm64 && !loong64 && !riscv64

package math

const haveArchExp2 = false

func archExp2(x float64) float64 {
	panic("not implemented")
}

```

// === FILE: references!/go/src/math/exp_amd64.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build amd64

package math

import "internal/cpu"

var useFMA = cpu.X86.HasAVX && cpu.X86.HasFMA

```

// === FILE: references!/go/src/math/exp_amd64.s ===
```text
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// The method is based on a paper by Naoki Shibata: "Efficient evaluation
// methods of elementary functions suitable for SIMD computation", Proc.
// of International Supercomputing Conference 2010 (ISC'10), pp. 25 -- 32
// (May 2010). The paper is available at
// https://link.springer.com/article/10.1007/s00450-010-0108-2
//
// The original code and the constants below are from the author's
// implementation available at http://freshmeat.net/projects/sleef.
// The README file says, "The software is in public domain.
// You can use the software without any obligation."
//
// This code is a simplified version of the original.

#define LN2 0.6931471805599453094172321214581766 // log_e(2)
#define LOG2E 1.4426950408889634073599246810018920 // 1/LN2
#define LN2U 0.69314718055966295651160180568695068359375 // upper half LN2
#define LN2L 0.28235290563031577122588448175013436025525412068e-12 // lower half LN2
#define PosInf 0x7FF0000000000000
#define NegInf 0xFFF0000000000000
#define Overflow 7.09782712893384e+02

DATA exprodata<>+0(SB)/8, $0.5
DATA exprodata<>+8(SB)/8, $1.0
DATA exprodata<>+16(SB)/8, $2.0
DATA exprodata<>+24(SB)/8, $1.6666666666666666667e-1
DATA exprodata<>+32(SB)/8, $4.1666666666666666667e-2
DATA exprodata<>+40(SB)/8, $8.3333333333333333333e-3
DATA exprodata<>+48(SB)/8, $1.3888888888888888889e-3
DATA exprodata<>+56(SB)/8, $1.9841269841269841270e-4
DATA exprodata<>+64(SB)/8, $2.4801587301587301587e-5
GLOBL exprodata<>+0(SB), RODATA, $72

// func Exp(x float64) float64
TEXT ·archExp(SB),NOSPLIT,$0
	// test bits for not-finite
	MOVQ    x+0(FP), BX
	MOVQ    $~(1<<63), AX // sign bit mask
	MOVQ    BX, DX
	ANDQ    AX, DX
	MOVQ    $PosInf, AX
	CMPQ    AX, DX
	JLE     notFinite
	// check if argument will overflow
	MOVQ    BX, X0
	MOVSD   $Overflow, X1
	COMISD  X1, X0
	JA      overflow
	MOVSD   $LOG2E, X1
	MULSD   X0, X1
	CVTSD2SL X1, BX // BX = exponent
	CVTSL2SD BX, X1
	CMPB ·useFMA(SB), $1
	JE   avxfma
	MOVSD   $LN2U, X2
	MULSD   X1, X2
	SUBSD   X2, X0
	MOVSD   $LN2L, X2
	MULSD   X1, X2
	SUBSD   X2, X0
	// reduce argument
	MULSD   $0.0625, X0
	// Taylor series evaluation
	MOVSD   exprodata<>+64(SB), X1
	MULSD   X0, X1
	ADDSD   exprodata<>+56(SB), X1
	MULSD   X0, X1
	ADDSD   exprodata<>+48(SB), X1
	MULSD   X0, X1
	ADDSD   exprodata<>+40(SB), X1
	MULSD   X0, X1
	ADDSD   exprodata<>+32(SB), X1
	MULSD   X0, X1
	ADDSD   exprodata<>+24(SB), X1
	MULSD   X0, X1
	ADDSD   exprodata<>+0(SB), X1
	MULSD   X0, X1
	ADDSD   exprodata<>+8(SB), X1
	MULSD   X1, X0
	MOVSD   exprodata<>+16(SB), X1
	ADDSD   X0, X1
	MULSD   X1, X0
	MOVSD   exprodata<>+16(SB), X1
	ADDSD   X0, X1
	MULSD   X1, X0
	MOVSD   exprodata<>+16(SB), X1
	ADDSD   X0, X1
	MULSD   X1, X0
	MOVSD   exprodata<>+16(SB), X1
	ADDSD   X0, X1
	MULSD   X1, X0
	ADDSD exprodata<>+8(SB), X0
	// return fr * 2**exponent
ldexp:
	ADDL    $0x3FF, BX // add bias
	JLE     denormal
	CMPL    BX, $0x7FF
	JGE     overflow
lastStep:
	SHLQ    $52, BX
	MOVQ    BX, X1
	MULSD   X1, X0
	MOVSD   X0, ret+8(FP)
	RET
notFinite:
	// test bits for -Inf
	MOVQ    $NegInf, AX
	CMPQ    AX, BX
	JNE     notNegInf
	// -Inf, return 0
underflow: // return 0
	MOVQ    $0, ret+8(FP)
	RET
overflow: // return +Inf
	MOVQ    $PosInf, BX
notNegInf: // NaN or +Inf, return x
	MOVQ    BX, ret+8(FP)
	RET
denormal:
	CMPL    BX, $-52
	JL      underflow
	ADDL    $0x3FE, BX // add bias - 1
	SHLQ    $52, BX
	MOVQ    BX, X1
	MULSD   X1, X0
	MOVQ    $1, BX
	JMP     lastStep

avxfma:
	MOVSD   $LN2U, X2
	VFNMADD231SD X2, X1, X0
	MOVSD   $LN2L, X2
	VFNMADD231SD X2, X1, X0
	// reduce argument
	MULSD   $0.0625, X0
	// Taylor series evaluation
	MOVSD   exprodata<>+64(SB), X1
	VFMADD213SD exprodata<>+56(SB), X0, X1
	VFMADD213SD exprodata<>+48(SB), X0, X1
	VFMADD213SD exprodata<>+40(SB), X0, X1
	VFMADD213SD exprodata<>+32(SB), X0, X1
	VFMADD213SD exprodata<>+24(SB), X0, X1
	VFMADD213SD exprodata<>+0(SB), X0, X1
	VFMADD213SD exprodata<>+8(SB), X0, X1
	MULSD   X1, X0
	VADDSD exprodata<>+16(SB), X0, X1
	MULSD   X1, X0
	VADDSD exprodata<>+16(SB), X0, X1
	MULSD   X1, X0
	VADDSD exprodata<>+16(SB), X0, X1
	MULSD   X1, X0
	VADDSD exprodata<>+16(SB), X0, X1
	VFMADD213SD   exprodata<>+8(SB), X1, X0
	JMP ldexp

```

// === FILE: references!/go/src/math/exp_arm64.s ===
```text
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#define	Ln2Hi	6.93147180369123816490e-01
#define	Ln2Lo	1.90821492927058770002e-10
#define	Log2e	1.44269504088896338700e+00
#define	Overflow	7.09782712893383973096e+02
#define	Underflow	-7.45133219101941108420e+02
#define	Overflow2	1.0239999999999999e+03
#define	Underflow2	-1.0740e+03
#define	NearZero	0x3e30000000000000	// 2**-28
#define	PosInf	0x7ff0000000000000
#define	FracMask	0x000fffffffffffff
#define	C1	0x3cb0000000000000	// 2**-52
#define	P1	1.66666666666666657415e-01	// 0x3FC55555; 0x55555555
#define	P2	-2.77777777770155933842e-03	// 0xBF66C16C; 0x16BEBD93
#define	P3	6.61375632143793436117e-05	// 0x3F11566A; 0xAF25DE2C
#define	P4	-1.65339022054652515390e-06	// 0xBEBBBD41; 0xC5D26BF1
#define	P5	4.13813679705723846039e-08	// 0x3E663769; 0x72BEA4D0

// Exp returns e**x, the base-e exponential of x.
// This is an assembly implementation of the method used for function Exp in file exp.go.
//
// func Exp(x float64) float64
TEXT ·archExp(SB),$0-16
	FMOVD	x+0(FP), F0	// F0 = x
	FCMPD	F0, F0
	BNE	isNaN		// x = NaN, return NaN
	FMOVD	$Overflow, F1
	FCMPD	F1, F0
	BGT	overflow	// x > Overflow, return PosInf
	FMOVD	$Underflow, F1
	FCMPD	F1, F0
	BLT	underflow	// x < Underflow, return 0
	MOVD	$NearZero, R0
	FMOVD	R0, F2
	FABSD	F0, F3
	FMOVD	$1.0, F1	// F1 = 1.0
	FCMPD	F2, F3
	BLT	nearzero	// fabs(x) < NearZero, return 1 + x
	// argument reduction, x = k*ln2 + r,  |r| <= 0.5*ln2
	// computed as r = hi - lo for extra precision.
	FMOVD	$Log2e, F2
	FMOVD	$0.5, F3
	FNMSUBD	F0, F3, F2, F4	// Log2e*x - 0.5
	FMADDD	F0, F3, F2, F3	// Log2e*x + 0.5
	FCMPD	$0.0, F0
	FCSELD	LT, F4, F3, F3	// F3 = k
	FCVTZSD	F3, R1		// R1 = int(k)
	SCVTFD	R1, F3		// F3 = float64(int(k))
	FMOVD	$Ln2Hi, F4	// F4 = Ln2Hi
	FMOVD	$Ln2Lo, F5	// F5 = Ln2Lo
	FMSUBD	F3, F0, F4, F4	// F4 = hi = x - float64(int(k))*Ln2Hi
	FMULD	F3, F5		// F5 = lo = float64(int(k)) * Ln2Lo
	FSUBD	F5, F4, F6	// F6 = r = hi - lo
	FMULD	F6, F6, F7	// F7 = t = r * r
	// compute y
	FMOVD	$P5, F8		// F8 = P5
	FMOVD	$P4, F9		// F9 = P4
	FMADDD	F7, F9, F8, F13	// P4+t*P5
	FMOVD	$P3, F10	// F10 = P3
	FMADDD	F7, F10, F13, F13	// P3+t*(P4+t*P5)
	FMOVD	$P2, F11	// F11 = P2
	FMADDD	F7, F11, F13, F13	// P2+t*(P3+t*(P4+t*P5))
	FMOVD	$P1, F12	// F12 = P1
	FMADDD	F7, F12, F13, F13	// P1+t*(P2+t*(P3+t*(P4+t*P5)))
	FMSUBD	F7, F6, F13, F13	// F13 = c = r - t*(P1+t*(P2+t*(P3+t*(P4+t*P5))))
	FMOVD	$2.0, F14
	FSUBD	F13, F14
	FMULD	F6, F13, F15
	FDIVD	F14, F15	// F15 = (r*c)/(2-c)
	FSUBD	F15, F5, F15	// lo-(r*c)/(2-c)
	FSUBD	F4, F15, F15	// (lo-(r*c)/(2-c))-hi
	FSUBD	F15, F1, F16	// F16 = y = 1-((lo-(r*c)/(2-c))-hi)
	// inline Ldexp(y, k), benefit:
	// 1, no parameter pass overhead.
	// 2, skip unnecessary checks for Inf/NaN/Zero
	FMOVD	F16, R0
	AND	$FracMask, R0, R2	// fraction
	LSR	$52, R0, R5	// exponent
	ADD	R1, R5		// R1 = int(k)
	CMP	$1, R5
	BGE	normal
	ADD	$52, R5		// denormal
	MOVD	$C1, R8
	FMOVD	R8, F1		// m = 2**-52
normal:
	ORR	R5<<52, R2, R0
	FMOVD	R0, F0
	FMULD	F1, F0		// return m * x
	FMOVD	F0, ret+8(FP)
	RET
nearzero:
	FADDD	F1, F0
isNaN:
	FMOVD	F0, ret+8(FP)
	RET
underflow:
	MOVD	ZR, ret+8(FP)
	RET
overflow:
	MOVD	$PosInf, R0
	MOVD	R0, ret+8(FP)
	RET


// Exp2 returns 2**x, the base-2 exponential of x.
// This is an assembly implementation of the method used for function Exp2 in file exp.go.
//
// func Exp2(x float64) float64
TEXT ·archExp2(SB),$0-16
	FMOVD	x+0(FP), F0	// F0 = x
	FCMPD	F0, F0
	BNE	isNaN		// x = NaN, return NaN
	FMOVD	$Overflow2, F1
	FCMPD	F1, F0
	BGT	overflow	// x > Overflow, return PosInf
	FMOVD	$Underflow2, F1
	FCMPD	F1, F0
	BLT	underflow	// x < Underflow, return 0
	// argument reduction; x = r*lg(e) + k with |r| <= ln(2)/2
	// computed as r = hi - lo for extra precision.
	FMOVD	$0.5, F2
	FSUBD	F2, F0, F3	// x + 0.5
	FADDD	F2, F0, F4	// x - 0.5
	FCMPD	$0.0, F0
	FCSELD	LT, F3, F4, F3	// F3 = k
	FCVTZSD	F3, R1		// R1 = int(k)
	SCVTFD	R1, F3		// F3 = float64(int(k))
	FSUBD	F3, F0, F3	// t = x - float64(int(k))
	FMOVD	$Ln2Hi, F4	// F4 = Ln2Hi
	FMOVD	$Ln2Lo, F5	// F5 = Ln2Lo
	FMULD	F3, F4		// F4 = hi = t * Ln2Hi
	FNMULD	F3, F5		// F5 = lo = -t * Ln2Lo
	FSUBD	F5, F4, F6	// F6 = r = hi - lo
	FMULD	F6, F6, F7	// F7 = t = r * r
	// compute y
	FMOVD	$P5, F8		// F8 = P5
	FMOVD	$P4, F9		// F9 = P4
	FMADDD	F7, F9, F8, F13	// P4+t*P5
	FMOVD	$P3, F10	// F10 = P3
	FMADDD	F7, F10, F13, F13	// P3+t*(P4+t*P5)
	FMOVD	$P2, F11	// F11 = P2
	FMADDD	F7, F11, F13, F13	// P2+t*(P3+t*(P4+t*P5))
	FMOVD	$P1, F12	// F12 = P1
	FMADDD	F7, F12, F13, F13	// P1+t*(P2+t*(P3+t*(P4+t*P5)))
	FMSUBD	F7, F6, F13, F13	// F13 = c = r - t*(P1+t*(P2+t*(P3+t*(P4+t*P5))))
	FMOVD	$2.0, F14
	FSUBD	F13, F14
	FMULD	F6, F13, F15
	FDIVD	F14, F15	// F15 = (r*c)/(2-c)
	FMOVD	$1.0, F1	// F1 = 1.0
	FSUBD	F15, F5, F15	// lo-(r*c)/(2-c)
	FSUBD	F4, F15, F15	// (lo-(r*c)/(2-c))-hi
	FSUBD	F15, F1, F16	// F16 = y = 1-((lo-(r*c)/(2-c))-hi)
	// inline Ldexp(y, k), benefit:
	// 1, no parameter pass overhead.
	// 2, skip unnecessary checks for Inf/NaN/Zero
	FMOVD	F16, R0
	AND	$FracMask, R0, R2	// fraction
	LSR	$52, R0, R5	// exponent
	ADD	R1, R5		// R1 = int(k)
	CMP	$1, R5
	BGE	normal
	ADD	$52, R5		// denormal
	MOVD	$C1, R8
	FMOVD	R8, F1		// m = 2**-52
normal:
	ORR	R5<<52, R2, R0
	FMOVD	R0, F0
	FMULD	F1, F0		// return m * x
isNaN:
	FMOVD	F0, ret+8(FP)
	RET
underflow:
	MOVD	ZR, ret+8(FP)
	RET
overflow:
	MOVD	$PosInf, R0
	MOVD	R0, ret+8(FP)
	RET

```

// === FILE: references!/go/src/math/exp_asm.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build amd64 || arm64 || loong64 || riscv64 || s390x

package math

const haveArchExp = true

func archExp(x float64) float64

```

// === FILE: references!/go/src/math/exp_loong64.s ===
```text
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

#define NearZero	0x3e30000000000000	// 2**-28
#define PosInf		0x7ff0000000000000
#define FracMask	0x000fffffffffffff
#define C1		0x3cb0000000000000	// 2**-52

DATA exprodata<>+0(SB)/8, $0.0
DATA exprodata<>+8(SB)/8, $0.5
DATA exprodata<>+16(SB)/8, $1.0
DATA exprodata<>+24(SB)/8, $2.0
DATA exprodata<>+32(SB)/8, $6.93147180369123816490e-01	// Ln2Hi
DATA exprodata<>+40(SB)/8, $1.90821492927058770002e-10	// Ln2Lo
DATA exprodata<>+48(SB)/8, $1.44269504088896338700e+00	// Log2e
DATA exprodata<>+56(SB)/8, $7.09782712893383973096e+02	// Overflow
DATA exprodata<>+64(SB)/8, $-7.45133219101941108420e+02	// Underflow
DATA exprodata<>+72(SB)/8, $1.0239999999999999e+03	// Overflow2
DATA exprodata<>+80(SB)/8, $-1.0740e+03			// Underflow2
DATA exprodata<>+88(SB)/8, $3.7252902984619141e-09	// NearZero
GLOBL exprodata<>+0(SB), NOPTR|RODATA, $96

DATA expmultirodata<>+0(SB)/8, $1.66666666666666657415e-01	// P1
DATA expmultirodata<>+8(SB)/8, $-2.77777777770155933842e-03	// P2
DATA expmultirodata<>+16(SB)/8, $6.61375632143793436117e-05	// P3
DATA expmultirodata<>+24(SB)/8, $-1.65339022054652515390e-06	// P4
DATA expmultirodata<>+32(SB)/8, $4.13813679705723846039e-08	// P5
GLOBL expmultirodata<>+0(SB), NOPTR|RODATA, $40

// Exp returns e**x, the base-e exponential of x.
// This is an assembly implementation of the method used for function Exp in file exp.go.
//
// func Exp(x float64) float64
TEXT ·archExp(SB),$0-16
	MOVD	x+0(FP), F0	// F0 = x

	MOVV	$exprodata<>+0(SB), R10
	MOVD	56(R10), F1	// Overflow
	MOVD	64(R10), F2	// Underflow
	MOVD	88(R10), F3	// NearZero
	MOVD	16(R10), F17	// 1.0

	CMPEQD	F0, F0, FCC0
	BFPF	isNaN		// x = NaN, return NaN

	CMPGTD	F0, F1, FCC0
	BFPT	overflow	// x > Overflow, return PosInf

	CMPGTD	F2, F0, FCC0
	BFPT	underflow	// x < Underflow, return 0

	ABSD	F0, F5
	CMPGTD	F3, F5, FCC0
	BFPT	nearzero	// fabs(x) < NearZero, return 1 + x

	// argument reduction, x = k*ln2 + r,  |r| <= 0.5*ln2
	// computed as r = hi - lo for extra precision.
	MOVD	0(R10), F5
	MOVD	8(R10), F3
	MOVD	48(R10), F2
	CMPGTD	F0, F5, FCC0
	FMSUBD	F3, F2, F0, F4	// Log2e*x - 0.5
	FMADDD	F3, F2, F0, F3	// Log2e*x + 0.5
	FSEL	FCC0, F3, F4, F3
	FTINTRZVD F3, F4	// float64 -> int64
	MOVV	F4, R5		// R5 = int(k)
	FFINTDV	F4, F3		// int64 -> float64

	MOVD	32(R10), F4
	MOVD	40(R10), F5
	FNMSUBD	F0, F3, F4, F4
	MULD	F3, F5, F5
	SUBD	F5, F4, F6
	MULD	F6, F6, F7

	// compute c
	MOVV	$expmultirodata<>+0(SB), R11
	MOVD	32(R11), F8
	MOVD	24(R11), F9
	FMADDD	F9, F8, F7, F13
	MOVD	16(R11), F10
	FMADDD	F10, F13, F7, F13
	MOVD	8(R11), F11
	FMADDD	F11, F13, F7, F13
	MOVD	0(R11), F12
	FMADDD	F12, F13, F7, F13
	FNMSUBD	F6, F13, F7, F13

	// compute y
	MOVD	24(R10), F14
	SUBD	F13, F14, F14
	MULD	F6, F13, F15
	DIVD	F14, F15, F15
	SUBD	F15, F5, F15
	SUBD	F4, F15, F15
	SUBD	F15, F17, F16

	// inline Ldexp(y, k), benefit:
	// 1, no parameter pass overhead.
	// 2, skip unnecessary checks for Inf/NaN/Zero
	MOVV	F16, R4
	MOVV	$FracMask, R9
	AND	R9, R4, R6	// fraction
	SRLV	$52, R4, R7	// exponent
	ADDV	R5, R7
	MOVV	$1, R12
	BGE	R7, R12, normal
	ADDV	$52, R7		// denormal
	MOVV	$C1, R8
	MOVV	R8, F17
normal:
	SLLV	$52, R7
	OR	R7, R6, R4
	MOVV	R4, F0
	MULD	F17, F0		// return m * x
	MOVD	F0, ret+8(FP)
	RET
nearzero:
	ADDD	F17, F0, F0
isNaN:
	MOVD	F0, ret+8(FP)
	RET
underflow:
	MOVV	R0, ret+8(FP)
	RET
overflow:
	MOVV	$PosInf, R4
	MOVV	R4, ret+8(FP)
	RET


// Exp2 returns 2**x, the base-2 exponential of x.
// This is an assembly implementation of the method used for function Exp2 in file exp.go.
//
// func Exp2(x float64) float64
TEXT ·archExp2(SB),$0-16
	MOVD	x+0(FP), F0	// F0 = x

	MOVV	$exprodata<>+0(SB), R10
	MOVD	72(R10), F1	// Overflow2
	MOVD	80(R10), F2	// Underflow2
	MOVD	88(R10), F3	// NearZero

	CMPEQD	F0, F0, FCC0
	BFPF	isNaN		// x = NaN, return NaN

	CMPGTD	F0, F1, FCC0
	BFPT	overflow	// x > Overflow, return PosInf

	CMPGTD	F2, F0, FCC0
	BFPT	underflow	// x < Underflow, return 0

	// argument reduction; x = r*lg(e) + k with |r| <= ln(2)/2
	// computed as r = hi - lo for extra precision.
	MOVD	0(R10), F10
	MOVD	8(R10), F2
	CMPGTD	F0, F10, FCC0
	SUBD	F2, F0, F4	// x - 0.5
	ADDD	F2, F0, F3	// x + 0.5
	FSEL	FCC0, F3, F4, F3
	FTINTRZVD F3, F4
	MOVV	F4, R5
	FFINTDV	F4, F3

	MOVD	32(R10), F4
	MOVD	40(R10), F5
	SUBD	F3, F0, F3
	MULD	F3, F4
	FNMSUBD	F10, F3, F5, F5
	SUBD	F5, F4, F6
	MULD	F6, F6, F7

	// compute c
	MOVV	$expmultirodata<>+0(SB), R11
	MOVD	32(R11), F8
	MOVD	24(R11), F9
	FMADDD	F9, F8, F7, F13
	MOVD	16(R11), F10
	FMADDD	F10, F13, F7, F13
	MOVD	8(R11), F11
	FMADDD	F11, F13, F7, F13
	MOVD	0(R11), F12
	FMADDD	F12, F13, F7, F13
	FNMSUBD	F6, F13, F7, F13

	// compute y
	MOVD	24(R10), F14
	SUBD	F13, F14, F14
	MULD	F6, F13, F15
	DIVD	F14, F15

	MOVD	16(R10), F17
	SUBD	F15, F5, F15
	SUBD	F4, F15, F15
	SUBD	F15, F17, F16

	// inline Ldexp(y, k), benefit:
	// 1, no parameter pass overhead.
	// 2, skip unnecessary checks for Inf/NaN/Zero
	MOVV	F16, R4
	MOVV	$FracMask, R9
	SRLV	$52, R4, R7	// exponent
	AND	R9, R4, R6	// fraction
	ADDV	R5, R7
	MOVV	$1, R12
	BGE	R7, R12, normal

	ADDV	$52, R7		// denormal
	MOVV	$C1, R8
	MOVV	R8, F17
normal:
	SLLV	$52, R7
	OR	R7, R6, R4
	MOVV	R4, F0
	MULD	F17, F0
isNaN:
	MOVD	F0, ret+8(FP)
	RET
underflow:
	MOVV	R0, ret+8(FP)
	RET
overflow:
	MOVV	$PosInf, R4
	MOVV	R4, ret+8(FP)
	RET

```

// === FILE: references!/go/src/math/exp_noasm.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !amd64 && !arm64 && !loong64 && !riscv64 && !s390x

package math

const haveArchExp = false

func archExp(x float64) float64 {
	panic("not implemented")
}

```

// === FILE: references!/go/src/math/exp_riscv64.s ===
```text
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

#define NearZero	0x3e30000000000000	// 2**-28
#define PosInf		0x7ff0000000000000
#define FracMask	0x000fffffffffffff
#define C1		0x3cb0000000000000	// 2**-52

DATA exprodata<>+0(SB)/8, $0.0
DATA exprodata<>+8(SB)/8, $0.5
DATA exprodata<>+16(SB)/8, $1.0
DATA exprodata<>+24(SB)/8, $2.0
DATA exprodata<>+32(SB)/8, $6.93147180369123816490e-01	// Ln2Hi
DATA exprodata<>+40(SB)/8, $1.90821492927058770002e-10	// Ln2Lo
DATA exprodata<>+48(SB)/8, $1.44269504088896338700e+00	// Log2e
DATA exprodata<>+56(SB)/8, $7.09782712893383973096e+02	// Overflow
DATA exprodata<>+64(SB)/8, $-7.45133219101941108420e+02	// Underflow
DATA exprodata<>+72(SB)/8, $1.0239999999999999e+03	// Overflow2
DATA exprodata<>+80(SB)/8, $-1.0740e+03			// Underflow2
DATA exprodata<>+88(SB)/8, $3.7252902984619141e-09	// NearZero
GLOBL exprodata<>+0(SB), NOPTR|RODATA, $96

DATA expmultirodata<>+0(SB)/8, $1.66666666666666657415e-01	// P1
DATA expmultirodata<>+8(SB)/8, $-2.77777777770155933842e-03	// P2
DATA expmultirodata<>+16(SB)/8, $6.61375632143793436117e-05	// P3
DATA expmultirodata<>+24(SB)/8, $-1.65339022054652515390e-06	// P4
DATA expmultirodata<>+32(SB)/8, $4.13813679705723846039e-08	// P5
GLOBL expmultirodata<>+0(SB), NOPTR|RODATA, $40

// Exp returns e**x, the base-e exponential of x.
// This is an assembly implementation of the method used for function Exp in file exp.go.
//
// func Exp(x float64) float64
TEXT ·archExp(SB),$0-16
	MOVD	x+0(FP), F0	// F0 = x

	MOV	$exprodata<>+0(SB), X5
	MOVD	56(X5), F1	// Overflow
	MOVD	64(X5), F2	// Underflow
	MOVD	88(X5), F3	// NearZero
	MOVD	16(X5), F17	// 1.0

	FEQD	F0, F0, X7
	BEQ	X0, X7, isNaN		// x = NaN, return NaN

	FLTD	F0, F1, X7
	BNE	X0, X7, overflow	// x > Overflow, return PosInf

	FLTD	F2, F0, X7
	BNE	X0, X7, underflow	// x < Underflow, return 0

	FABSD	F0, F5
	FLTD	F3, F5, X7
	BNE	X0, X7, nearzero	// fabs(x) < NearZero, return 1 + x

	// argument reduction, x = k*ln2 + r,  |r| <= 0.5*ln2
	// computed as r = hi - lo for extra precision.
	MOVD	0(X5), F5
	MOVD	8(X5), F3
	MOVD	48(X5), F2
	FLTD	F0, F5, X7
	BNE	X0, X7, add		// x > 0
sub:
	FMSUBD	F0, F2, F3, F3	// Log2e*x - 0.5
	JMP	2(PC)
add:
	FMADDD	F0, F2, F3, F3	// Log2e*x + 0.5

	FCVTLD.RTZ	F3, X16	// float64 -> int64
	FCVTDL	X16, F3		// int64 -> float64

	MOVD	32(X5), F4
	MOVD	40(X5), F5
	FNMSUBD	F3, F4, F0, F4
	FMULD	F3, F5, F5
	FSUBD	F5, F4, F6
	FMULD	F6, F6, F7

	// compute c
	// r=(FMA x y z) -> FMADDD z, y, x, r
	// r=(FMA x y z) -> FMADDD x, y, z, r
	MOV	$expmultirodata<>+0(SB), X6
	MOVD	32(X6), F8
	MOVD	24(X6), F9
	FMADDD	F7, F8, F9, F13
	MOVD	16(X6), F10
	FMADDD	F7, F13, F10, F13
	MOVD	8(X6), F11
	FMADDD	F7, F13, F11, F13
	MOVD	0(X6), F12
	FMADDD	F7, F13, F12, F13
	FNMSUBD	F7, F13, F6, F13

	// compute y
	MOVD	24(X5), F14
	FSUBD	F13, F14, F14
	FMULD	F6, F13, F15
	FDIVD	F14, F15, F15
	FSUBD	F15, F5, F15
	FSUBD	F4, F15, F15
	FSUBD	F15, F17, F16

	// inline Ldexp(y, k), benefit:
	// 1, no parameter pass overhead.
	// 2, skip unnecessary checks for Inf/NaN/Zero
	MOVD	F16, X15
	MOV	$FracMask, X20
	AND	X20, X15, X17	// fraction
	SRL	$52, X15, X18	// exponent
	ADD	X16, X18
	MOV	$1, X21
	BGE	X18, X21, normal
	ADD	$52, X18		// denormal
	MOV	$C1, X19
	MOVD	X19, F17
normal:
	SLL	$52, X18
	OR	X18, X17, X15
	MOVD	X15, F0
	FMULD	F17, F0, F0		// return m * x
	MOVD	F0, ret+8(FP)
	RET
nearzero:
	FADDD	F17, F0, F0
isNaN:
	MOVD	F0, ret+8(FP)
	RET
underflow:
	MOV	X0, ret+8(FP)
	RET
overflow:
	MOV	$PosInf, X15
	MOV	X15, ret+8(FP)
	RET


// Exp2 returns 2**x, the base-2 exponential of x.
// This is an assembly implementation of the method used for function Exp2 in file exp.go.
//
// func Exp2(x float64) float64
TEXT ·archExp2(SB),$0-16
	MOVD	x+0(FP), F0	// F0 = x

	MOV	$exprodata<>+0(SB), X5
	MOVD	72(X5), F1	// Overflow2
	MOVD	80(X5), F2	// Underflow2
	MOVD	88(X5), F3	// NearZero

	FEQD	F0, F0, X7
	BEQ	X0, X7, isNaN		// x = NaN, return NaN

	FLTD	F0, F1, X7
	BNE	X0, X7, overflow	// x > Overflow, return PosInf

	FLTD	F2, F0, X7
	BNE	X0, X7, underflow	// x < Underflow, return 0

	// argument reduction; x = r*lg(e) + k with |r| <= ln(2)/2
	// computed as r = hi - lo for extra precision.
	MOVD	0(X5), F10
	MOVD	8(X5), F2
	FLTD	F0, F10, X7
	BNE	X0, X7, add
sub:
	FSUBD	F2, F0, F3	// x - 0.5
	JMP	2(PC)
add:
	FADDD	F2, F0, F3	// x + 0.5

	FCVTLD.RTZ	F3, X16
	FCVTDL	X16, F3

	MOVD	32(X5), F4
	MOVD	40(X5), F5
	FSUBD	F3, F0, F3
	FMULD	F3, F4, F4
	FNMSUBD	F5, F3, F10, F5
	FSUBD	F5, F4, F6
	FMULD	F6, F6, F7

	// compute c
	MOV	$expmultirodata<>+0(SB), X6
	MOVD	32(X6), F8
	MOVD	24(X6), F9
	FMADDD	F7, F8, F9, F13
	MOVD	16(X6), F10
	FMADDD	F7, F13, F10, F13
	MOVD	8(X6), F11
	FMADDD	F7, F13, F11, F13
	MOVD	0(X6), F12
	FMADDD	F7, F13, F12, F13
	FNMSUBD	F7, F13, F6, F13

	// compute y
	MOVD	24(X5), F14
	FSUBD	F13, F14, F14
	FMULD	F6, F13, F15
	FDIVD	F14, F15, F15

	MOVD	16(X5), F17
	FSUBD	F15, F5, F15
	FSUBD	F4, F15, F15
	FSUBD	F15, F17, F16

	// inline Ldexp(y, k), benefit:
	// 1, no parameter pass overhead.
	// 2, skip unnecessary checks for Inf/NaN/Zero
	MOVD	F16, X15
	MOV	$FracMask, X20
	SRL	$52, X15, X18	// exponent
	AND	X20, X15, X17	// fraction
	ADD	X16, X18
	MOV	$1, X21
	BGE	X18, X21, normal

	ADD	$52, X18		// denormal
	MOV	$C1, X19
	MOVD	X19, F17
normal:
	SLL	$52, X18
	OR	X18, X17, X15
	MOVD	X15, F0
	FMULD	F17, F0, F0
isNaN:
	MOVD	F0, ret+8(FP)
	RET
underflow:
	MOV	X0, ret+8(FP)
	RET
overflow:
	MOV	$PosInf, X15
	MOV	X15, ret+8(FP)
	RET

```

// === FILE: references!/go/src/math/exp_s390x.s ===
```text
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// Minimax polynomial approximation and other constants
DATA ·exprodataL22<> + 0(SB)/8, $800.0E+00
DATA ·exprodataL22<> + 8(SB)/8, $1.0000000000000022e+00
DATA ·exprodataL22<> + 16(SB)/8, $0.500000000000004237e+00
DATA ·exprodataL22<> + 24(SB)/8, $0.166666666630345592e+00
DATA ·exprodataL22<> + 32(SB)/8, $0.138926439368309441e-02
DATA ·exprodataL22<> + 40(SB)/8, $0.833349307718286047e-02
DATA ·exprodataL22<> + 48(SB)/8, $0.416666664838056960e-01
DATA ·exprodataL22<> + 56(SB)/8, $-.231904681384629956E-16
DATA ·exprodataL22<> + 64(SB)/8, $-.693147180559945286E+00
DATA ·exprodataL22<> + 72(SB)/8, $0.144269504088896339E+01
DATA ·exprodataL22<> + 80(SB)/8, $704.0E+00
GLOBL ·exprodataL22<> + 0(SB), RODATA, $88

DATA ·expxinf<> + 0(SB)/8, $0x7ff0000000000000
GLOBL ·expxinf<> + 0(SB), RODATA, $8
DATA ·expx4ff<> + 0(SB)/8, $0x4ff0000000000000
GLOBL ·expx4ff<> + 0(SB), RODATA, $8
DATA ·expx2ff<> + 0(SB)/8, $0x2ff0000000000000
GLOBL ·expx2ff<> + 0(SB), RODATA, $8
DATA ·expxaddexp<> + 0(SB)/8, $0xc2f0000100003fef
GLOBL ·expxaddexp<> + 0(SB), RODATA, $8

// Log multipliers table
DATA ·exptexp<> + 0(SB)/8, $0.442737824274138381E-01
DATA ·exptexp<> + 8(SB)/8, $0.263602189790660309E-01
DATA ·exptexp<> + 16(SB)/8, $0.122565642281703586E-01
DATA ·exptexp<> + 24(SB)/8, $0.143757052860721398E-02
DATA ·exptexp<> + 32(SB)/8, $-.651375034121276075E-02
DATA ·exptexp<> + 40(SB)/8, $-.119317678849450159E-01
DATA ·exptexp<> + 48(SB)/8, $-.150868749549871069E-01
DATA ·exptexp<> + 56(SB)/8, $-.161992609578469234E-01
DATA ·exptexp<> + 64(SB)/8, $-.154492360403337917E-01
DATA ·exptexp<> + 72(SB)/8, $-.129850717389178721E-01
DATA ·exptexp<> + 80(SB)/8, $-.892902649276657891E-02
DATA ·exptexp<> + 88(SB)/8, $-.338202636596794887E-02
DATA ·exptexp<> + 96(SB)/8, $0.357266307045684762E-02
DATA ·exptexp<> + 104(SB)/8, $0.118665304327406698E-01
DATA ·exptexp<> + 112(SB)/8, $0.214434994118118914E-01
DATA ·exptexp<> + 120(SB)/8, $0.322580645161290314E-01
GLOBL ·exptexp<> + 0(SB), RODATA, $128

// Exp returns e**x, the base-e exponential of x.
//
// Special cases are:
//      Exp(+Inf) = +Inf
//      Exp(NaN) = NaN
// Very large values overflow to 0 or +Inf.
// Very small values underflow to 1.
// The algorithm used is minimax polynomial approximation using a table of
// polynomial coefficients determined with a Remez exchange algorithm.

TEXT	·expAsm(SB), NOSPLIT, $0-16
	FMOVD	x+0(FP), F0
	MOVD	$·exprodataL22<>+0(SB), R5
	LTDBR	F0, F0
	BLTU	L20
	FMOVD	F0, F2
L2:
	WORD	$0xED205050	//cdb	%f2,.L23-.L22(%r5)
	BYTE	$0x00
	BYTE	$0x19
	BGE	L16
	BVS	L16
	WFCEDBS	V2, V2, V2
	BVS	LEXITTAGexp
	MOVD	$·expxaddexp<>+0(SB), R1
	FMOVD	72(R5), F6
	FMOVD	0(R1), F2
	WFMSDB	V0, V6, V2, V6
	FMOVD	64(R5), F4
	FADD	F6, F2
	FMOVD	56(R5), F1
	FMADD	F4, F2, F0
	FMOVD	48(R5), F3
	WFMADB	V2, V1, V0, V2
	FMOVD	40(R5), F1
	FMOVD	32(R5), F4
	FMUL	F0, F0
	WFMADB	V2, V4, V1, V4
	LGDR	F6, R1
	FMOVD	24(R5), F1
	WFMADB	V2, V3, V1, V3
	FMOVD	16(R5), F1
	WFMADB	V0, V4, V3, V4
	FMOVD	8(R5), F3
	WFMADB	V2, V1, V3, V1
	RISBGZ	$57, $60, $3, R1, R3
	WFMADB	V0, V4, V1, V0
	MOVD	$·exptexp<>+0(SB), R2
	WORD	$0x68432000	//ld	%f4,0(%r3,%r2)
	FMADD	F4, F2, F2
	SLD	$48, R1, R2
	WFMADB	V2, V0, V4, V2
	LDGR	R2, F0
	FMADD	F0, F2, F0
	FMOVD	F0, ret+8(FP)
	RET
L16:
	WFCEDBS	V2, V2, V4
	BVS	LEXITTAGexp
	WORD	$0xED205000	//cdb	%f2,.L33-.L22(%r5)
	BYTE	$0x00
	BYTE	$0x19
	BLT	L6
	WFCEDBS	V2, V0, V0
	BVS	L13
	MOVD	$·expxinf<>+0(SB), R1
	FMOVD	0(R1), F0
	FMOVD	F0, ret+8(FP)
	RET
L20:
	LCDBR	F0, F2
	BR	L2
L6:
	MOVD	$·expxaddexp<>+0(SB), R1
	FMOVD	72(R5), F3
	FMOVD	0(R1), F4
	WFMSDB	V0, V3, V4, V3
	FMOVD	64(R5), F6
	FADD	F3, F4
	FMOVD	56(R5), F5
	WFMADB	V4, V6, V0, V6
	FMOVD	32(R5), F1
	WFMADB	V4, V5, V6, V4
	FMOVD	40(R5), F5
	FMUL	F6, F6
	WFMADB	V4, V1, V5, V1
	FMOVD	48(R5), F7
	LGDR	F3, R1
	FMOVD	24(R5), F5
	WFMADB	V4, V7, V5, V7
	FMOVD	16(R5), F5
	WFMADB	V6, V1, V7, V1
	FMOVD	8(R5), F7
	WFMADB	V4, V5, V7, V5
	RISBGZ	$57, $60, $3, R1, R3
	WFMADB	V6, V1, V5, V6
	MOVD	$·exptexp<>+0(SB), R2
	WFCHDBS	V2, V0, V0
	WORD	$0x68132000	//ld	%f1,0(%r3,%r2)
	FMADD	F1, F4, F4
	MOVD	$0x4086000000000000, R2
	WFMADB	V4, V6, V1, V4
	BEQ	L21
	ADDW	$0xF000, R1
	RISBGN	$0, $15, $48, R1, R2
	LDGR	R2, F0
	FMADD	F0, F4, F0
	MOVD	$·expx4ff<>+0(SB), R3
	FMOVD	0(R3), F2
	FMUL	F2, F0
	FMOVD	F0, ret+8(FP)
	RET
L13:
	FMOVD	$0, F0
	FMOVD	F0, ret+8(FP)
	RET
L21:
	ADDW	$0x1000, R1
	RISBGN	$0, $15, $48, R1, R2
	LDGR	R2, F0
	FMADD	F0, F4, F0
	MOVD	$·expx2ff<>+0(SB), R3
	FMOVD	0(R3), F2
	FMUL	F2, F0
	FMOVD	F0, ret+8(FP)
	RET
LEXITTAGexp:
	FMOVD	F0, ret+8(FP)
	RET

```

// === FILE: references!/go/src/math/expm1.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// The original C code, the long comment, and the constants
// below are from FreeBSD's /usr/src/lib/msun/src/s_expm1.c
// and came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
// expm1(x)
// Returns exp(x)-1, the exponential of x minus 1.
//
// Method
//   1. Argument reduction:
//      Given x, find r and integer k such that
//
//               x = k*ln2 + r,  |r| <= 0.5*ln2 ~ 0.34658
//
//      Here a correction term c will be computed to compensate
//      the error in r when rounded to a floating-point number.
//
//   2. Approximating expm1(r) by a special rational function on
//      the interval [0,0.34658]:
//      Since
//          r*(exp(r)+1)/(exp(r)-1) = 2+ r**2/6 - r**4/360 + ...
//      we define R1(r*r) by
//          r*(exp(r)+1)/(exp(r)-1) = 2+ r**2/6 * R1(r*r)
//      That is,
//          R1(r**2) = 6/r *((exp(r)+1)/(exp(r)-1) - 2/r)
//                   = 6/r * ( 1 + 2.0*(1/(exp(r)-1) - 1/r))
//                   = 1 - r**2/60 + r**4/2520 - r**6/100800 + ...
//      We use a special Reme algorithm on [0,0.347] to generate
//      a polynomial of degree 5 in r*r to approximate R1. The
//      maximum error of this polynomial approximation is bounded
//      by 2**-61. In other words,
//          R1(z) ~ 1.0 + Q1*z + Q2*z**2 + Q3*z**3 + Q4*z**4 + Q5*z**5
//      where   Q1  =  -1.6666666666666567384E-2,
//              Q2  =   3.9682539681370365873E-4,
//              Q3  =  -9.9206344733435987357E-6,
//              Q4  =   2.5051361420808517002E-7,
//              Q5  =  -6.2843505682382617102E-9;
//      (where z=r*r, and the values of Q1 to Q5 are listed below)
//      with error bounded by
//          |                  5           |     -61
//          | 1.0+Q1*z+...+Q5*z   -  R1(z) | <= 2
//          |                              |
//
//      expm1(r) = exp(r)-1 is then computed by the following
//      specific way which minimize the accumulation rounding error:
//                             2     3
//                            r     r    [ 3 - (R1 + R1*r/2)  ]
//            expm1(r) = r + --- + --- * [--------------------]
//                            2     2    [ 6 - r*(3 - R1*r/2) ]
//
//      To compensate the error in the argument reduction, we use
//              expm1(r+c) = expm1(r) + c + expm1(r)*c
//                         ~ expm1(r) + c + r*c
//      Thus c+r*c will be added in as the correction terms for
//      expm1(r+c). Now rearrange the term to avoid optimization
//      screw up:
//                      (      2                                    2 )
//                      ({  ( r    [ R1 -  (3 - R1*r/2) ]  )  }    r  )
//       expm1(r+c)~r - ({r*(--- * [--------------------]-c)-c} - --- )
//                      ({  ( 2    [ 6 - r*(3 - R1*r/2) ]  )  }    2  )
//                      (                                             )
//
//                 = r - E
//   3. Scale back to obtain expm1(x):
//      From step 1, we have
//         expm1(x) = either 2**k*[expm1(r)+1] - 1
//                  = or     2**k*[expm1(r) + (1-2**-k)]
//   4. Implementation notes:
//      (A). To save one multiplication, we scale the coefficient Qi
//           to Qi*2**i, and replace z by (x**2)/2.
//      (B). To achieve maximum accuracy, we compute expm1(x) by
//        (i)   if x < -56*ln2, return -1.0, (raise inexact if x!=inf)
//        (ii)  if k=0, return r-E
//        (iii) if k=-1, return 0.5*(r-E)-0.5
//        (iv)  if k=1 if r < -0.25, return 2*((r+0.5)- E)
//                     else          return  1.0+2.0*(r-E);
//        (v)   if (k<-2||k>56) return 2**k(1-(E-r)) - 1 (or exp(x)-1)
//        (vi)  if k <= 20, return 2**k((1-2**-k)-(E-r)), else
//        (vii) return 2**k(1-((E+2**-k)-r))
//
// Special cases:
//      expm1(INF) is INF, expm1(NaN) is NaN;
//      expm1(-INF) is -1, and
//      for finite argument, only expm1(0)=0 is exact.
//
// Accuracy:
//      according to an error analysis, the error is always less than
//      1 ulp (unit in the last place).
//
// Misc. info.
//      For IEEE double
//          if x >  7.09782712893383973096e+02 then expm1(x) overflow
//
// Constants:
// The hexadecimal values are the intended ones for the following
// constants. The decimal values may be used, provided that the
// compiler will convert from decimal to binary accurately enough
// to produce the hexadecimal values shown.
//

// Expm1 returns e**x - 1, the base-e exponential of x minus 1.
// It is more accurate than [Exp](x) - 1 when x is near zero.
//
// Special cases are:
//
//	Expm1(+Inf) = +Inf
//	Expm1(-Inf) = -1
//	Expm1(NaN) = NaN
//
// Very large values overflow to -1 or +Inf.
func Expm1(x float64) float64 {
	if haveArchExpm1 {
		return archExpm1(x)
	}
	return expm1(x)
}

func expm1(x float64) float64 {
	const (
		Othreshold = 7.09782712893383973096e+02 // 0x40862E42FEFA39EF
		Ln2X56     = 3.88162421113569373274e+01 // 0x4043687a9f1af2b1
		Ln2HalfX3  = 1.03972077083991796413e+00 // 0x3ff0a2b23f3bab73
		Ln2Half    = 3.46573590279972654709e-01 // 0x3fd62e42fefa39ef
		Ln2Hi      = 6.93147180369123816490e-01 // 0x3fe62e42fee00000
		Ln2Lo      = 1.90821492927058770002e-10 // 0x3dea39ef35793c76
		InvLn2     = 1.44269504088896338700e+00 // 0x3ff71547652b82fe
		Tiny       = 1.0 / (1 << 54)            // 2**-54 = 0x3c90000000000000
		// scaled coefficients related to expm1
		Q1 = -3.33333333333331316428e-02 // 0xBFA11111111110F4
		Q2 = 1.58730158725481460165e-03  // 0x3F5A01A019FE5585
		Q3 = -7.93650757867487942473e-05 // 0xBF14CE199EAADBB7
		Q4 = 4.00821782732936239552e-06  // 0x3ED0CFCA86E65239
		Q5 = -2.01099218183624371326e-07 // 0xBE8AFDB76E09C32D
	)

	// special cases
	switch {
	case IsInf(x, 1) || IsNaN(x):
		return x
	case IsInf(x, -1):
		return -1
	}

	absx := x
	sign := false
	if x < 0 {
		absx = -absx
		sign = true
	}

	// filter out huge argument
	if absx >= Ln2X56 { // if |x| >= 56 * ln2
		if sign {
			return -1 // x < -56*ln2, return -1
		}
		if absx >= Othreshold { // if |x| >= 709.78...
			return Inf(1)
		}
	}

	// argument reduction
	var c float64
	var k int
	if absx > Ln2Half { // if  |x| > 0.5 * ln2
		var hi, lo float64
		if absx < Ln2HalfX3 { // and |x| < 1.5 * ln2
			if !sign {
				hi = x - Ln2Hi
				lo = Ln2Lo
				k = 1
			} else {
				hi = x + Ln2Hi
				lo = -Ln2Lo
				k = -1
			}
		} else {
			if !sign {
				k = int(InvLn2*x + 0.5)
			} else {
				k = int(InvLn2*x - 0.5)
			}
			t := float64(k)
			hi = x - t*Ln2Hi // t * Ln2Hi is exact here
			lo = t * Ln2Lo
		}
		x = hi - lo
		c = (hi - x) - lo
	} else if absx < Tiny { // when |x| < 2**-54, return x
		return x
	} else {
		k = 0
	}

	// x is now in primary range
	hfx := 0.5 * x
	hxs := x * hfx
	r1 := 1 + hxs*(Q1+hxs*(Q2+hxs*(Q3+hxs*(Q4+hxs*Q5))))
	t := 3 - r1*hfx
	e := hxs * ((r1 - t) / (6.0 - x*t))
	if k == 0 {
		return x - (x*e - hxs) // c is 0
	}
	e = (x*(e-c) - c)
	e -= hxs
	switch {
	case k == -1:
		return 0.5*(x-e) - 0.5
	case k == 1:
		if x < -0.25 {
			return -2 * (e - (x + 0.5))
		}
		return 1 + 2*(x-e)
	case k <= -2 || k > 56: // suffice to return exp(x)-1
		y := 1 - (e - x)
		y = Float64frombits(Float64bits(y) + uint64(k)<<52) // add k to y's exponent
		return y - 1
	}
	if k < 20 {
		t := Float64frombits(0x3ff0000000000000 - (0x20000000000000 >> uint(k))) // t=1-2**-k
		y := t - (e - x)
		y = Float64frombits(Float64bits(y) + uint64(k)<<52) // add k to y's exponent
		return y
	}
	t = Float64frombits(uint64(0x3ff-k) << 52) // 2**-k
	y := x - (e + t)
	y++
	y = Float64frombits(Float64bits(y) + uint64(k)<<52) // add k to y's exponent
	return y
}

```

// === FILE: references!/go/src/math/expm1_s390x.s ===
```text
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// Minimax polynomial approximation and other constants
DATA ·expm1rodataL22<> + 0(SB)/8, $-1.0
DATA ·expm1rodataL22<> + 8(SB)/8, $800.0E+00
DATA ·expm1rodataL22<> + 16(SB)/8, $1.0
DATA ·expm1rodataL22<> + 24(SB)/8, $-.231904681384629956E-16
DATA ·expm1rodataL22<> + 32(SB)/8, $0.50000000000000029671E+00
DATA ·expm1rodataL22<> + 40(SB)/8, $0.16666666666666676570E+00
DATA ·expm1rodataL22<> + 48(SB)/8, $0.83333333323590973444E-02
DATA ·expm1rodataL22<> + 56(SB)/8, $0.13889096526400683566E-02
DATA ·expm1rodataL22<> + 64(SB)/8, $0.41666666661701152924E-01
DATA ·expm1rodataL22<> + 72(SB)/8, $0.19841562053987360264E-03
DATA ·expm1rodataL22<> + 80(SB)/8, $-.693147180559945286E+00
DATA ·expm1rodataL22<> + 88(SB)/8, $0.144269504088896339E+01
DATA ·expm1rodataL22<> + 96(SB)/8, $704.0E+00
GLOBL ·expm1rodataL22<> + 0(SB), RODATA, $104

DATA ·expm1xmone<> + 0(SB)/8, $0xbff0000000000000
GLOBL ·expm1xmone<> + 0(SB), RODATA, $8
DATA ·expm1xinf<> + 0(SB)/8, $0x7ff0000000000000
GLOBL ·expm1xinf<> + 0(SB), RODATA, $8
DATA ·expm1x4ff<> + 0(SB)/8, $0x4ff0000000000000
GLOBL ·expm1x4ff<> + 0(SB), RODATA, $8
DATA ·expm1x2ff<> + 0(SB)/8, $0x2ff0000000000000
GLOBL ·expm1x2ff<> + 0(SB), RODATA, $8
DATA ·expm1xaddexp<> + 0(SB)/8, $0xc2f0000100003ff0
GLOBL ·expm1xaddexp<> + 0(SB), RODATA, $8

// Log multipliers table
DATA ·expm1tab<> + 0(SB)/8, $0.0
DATA ·expm1tab<> + 8(SB)/8, $-.171540871271399150E-01
DATA ·expm1tab<> + 16(SB)/8, $-.306597931864376363E-01
DATA ·expm1tab<> + 24(SB)/8, $-.410200970469965021E-01
DATA ·expm1tab<> + 32(SB)/8, $-.486343079978231466E-01
DATA ·expm1tab<> + 40(SB)/8, $-.538226193725835820E-01
DATA ·expm1tab<> + 48(SB)/8, $-.568439602538111520E-01
DATA ·expm1tab<> + 56(SB)/8, $-.579091847395528847E-01
DATA ·expm1tab<> + 64(SB)/8, $-.571909584179366341E-01
DATA ·expm1tab<> + 72(SB)/8, $-.548312665987204407E-01
DATA ·expm1tab<> + 80(SB)/8, $-.509471843643441085E-01
DATA ·expm1tab<> + 88(SB)/8, $-.456353588448863359E-01
DATA ·expm1tab<> + 96(SB)/8, $-.389755254243262365E-01
DATA ·expm1tab<> + 104(SB)/8, $-.310332908285244231E-01
DATA ·expm1tab<> + 112(SB)/8, $-.218623539150173528E-01
DATA ·expm1tab<> + 120(SB)/8, $-.115062908917949451E-01
GLOBL ·expm1tab<> + 0(SB), RODATA, $128

// Expm1 returns e**x - 1, the base-e exponential of x minus 1.
// It is more accurate than Exp(x) - 1 when x is near zero.
//
// Special cases are:
//      Expm1(+Inf) = +Inf
//      Expm1(-Inf) = -1
//      Expm1(NaN) = NaN
// Very large values overflow to -1 or +Inf.
// The algorithm used is minimax polynomial approximation using a table of
// polynomial coefficients determined with a Remez exchange algorithm.

TEXT	·expm1Asm(SB), NOSPLIT, $0-16
	FMOVD	x+0(FP), F0
	MOVD	$·expm1rodataL22<>+0(SB), R5
	LTDBR	F0, F0
	BLTU	L20
	FMOVD	F0, F2
L2:
	WORD	$0xED205060	//cdb	%f2,.L23-.L22(%r5)
	BYTE	$0x00
	BYTE	$0x19
	BGE	L16
	BVS	L16
	WFCEDBS	V2, V2, V2
	BVS	LEXITTAGexpm1
	MOVD	$·expm1xaddexp<>+0(SB), R1
	FMOVD	88(R5), F1
	FMOVD	0(R1), F2
	WFMSDB	V0, V1, V2, V1
	FMOVD	80(R5), F6
	WFADB	V1, V2, V4
	FMOVD	72(R5), F2
	FMADD	F6, F4, F0
	FMOVD	64(R5), F3
	FMOVD	56(R5), F6
	FMOVD	48(R5), F5
	FMADD	F2, F0, F6
	WFMADB	V0, V5, V3, V5
	WFMDB	V0, V0, V2
	LGDR	F1, R1
	WFMADB	V6, V2, V5, V6
	FMOVD	40(R5), F3
	FMOVD	32(R5), F5
	WFMADB	V0, V3, V5, V3
	FMOVD	24(R5), F5
	WFMADB	V2, V6, V3, V2
	FMADD	F5, F4, F0
	FMOVD	16(R5), F6
	WFMADB	V0, V2, V6, V2
	RISBGZ	$57, $60, $3, R1, R3
	LCDBR	F2, F2
	MOVD	$·expm1tab<>+0(SB), R2
	WORD	$0x68432000	//ld	%f4,0(%r3,%r2)
	FMADD	F4, F0, F0
	SLD	$48, R1, R2
	WFMSDB	V2, V0, V4, V0
	LDGR	R2, F4
	LCDBR   F0, F0
	FSUB	F4, F6
	WFMSDB	V0, V4, V6, V0
	FMOVD	F0, ret+8(FP)
	RET
L16:
	WFCEDBS	V2, V2, V4
	BVS	LEXITTAGexpm1
	WORD	$0xED205008	//cdb	%f2,.L34-.L22(%r5)
	BYTE	$0x00
	BYTE	$0x19
	BLT	L6
	WFCEDBS	V2, V0, V0
	BVS	L7
	MOVD	$·expm1xinf<>+0(SB), R1
	FMOVD	0(R1), F0
	FMOVD	F0, ret+8(FP)
	RET
L20:
	LCDBR   F0, F2
	BR	L2
L6:
	MOVD	$·expm1xaddexp<>+0(SB), R1
	FMOVD	88(R5), F5
	FMOVD	0(R1), F4
	WFMSDB	V0, V5, V4, V5
	FMOVD	80(R5), F3
	WFADB	V5, V4, V1
	VLEG	$0, 48(R5), V16
	WFMADB	V1, V3, V0, V3
	FMOVD	56(R5), F4
	FMOVD	64(R5), F7
	FMOVD	72(R5), F6
	WFMADB	V3, V16, V7, V16
	WFMADB	V3, V6, V4, V6
	WFMDB	V3, V3, V4
	MOVD	$·expm1tab<>+0(SB), R2
	WFMADB	V6, V4, V16, V6
	VLEG	$0, 32(R5), V16
	FMOVD	40(R5), F7
	WFMADB	V3, V7, V16, V7
	VLEG	$0, 24(R5), V16
	WFMADB	V4, V6, V7, V4
	WFMADB	V1, V16, V3, V1
	FMOVD	16(R5), F6
	FMADD	F4, F1, F6
	LGDR	F5, R1
	LCDBR   F6, F6
	RISBGZ	$57, $60, $3, R1, R3
	WORD	$0x68432000	//ld	%f4,0(%r3,%r2)
	FMADD	F4, F1, F1
	MOVD	$0x4086000000000000, R2
	FMSUB	F1, F6, F4
	LCDBR   F4, F4
	WFCHDBS	V2, V0, V0
	BEQ	L21
	ADDW	$0xF000, R1
	RISBGN	$0, $15, $48, R1, R2
	LDGR	R2, F0
	FMADD	F0, F4, F0
	MOVD	$·expm1x4ff<>+0(SB), R3
	FMOVD	0(R5), F4
	FMOVD	0(R3), F2
	WFMADB	V2, V0, V4, V0
	FMOVD	F0, ret+8(FP)
	RET
L7:
	MOVD	$·expm1xmone<>+0(SB), R1
	FMOVD	0(R1), F0
	FMOVD	F0, ret+8(FP)
	RET
L21:
	ADDW	$0x1000, R1
	RISBGN	$0, $15, $48, R1, R2
	LDGR	R2, F0
	FMADD	F0, F4, F0
	MOVD	$·expm1x2ff<>+0(SB), R3
	FMOVD	0(R5), F4
	FMOVD	0(R3), F2
	WFMADB	V2, V0, V4, V0
	FMOVD	F0, ret+8(FP)
	RET
LEXITTAGexpm1:
	FMOVD	F0, ret+8(FP)
	RET

```

// === FILE: references!/go/src/math/floor.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// Floor returns the greatest integer value less than or equal to x.
//
// Special cases are:
//
//	Floor(±0) = ±0
//	Floor(±Inf) = ±Inf
//	Floor(NaN) = NaN
func Floor(x float64) float64 {
	if haveArchFloor {
		return archFloor(x)
	}
	return floor(x)
}

func floor(x float64) float64 {
	if x == 0 || IsNaN(x) || IsInf(x, 0) {
		return x
	}
	if x < 0 {
		d, fract := Modf(-x)
		if fract != 0.0 {
			d = d + 1
		}
		return -d
	}
	d, _ := Modf(x)
	return d
}

// Ceil returns the least integer value greater than or equal to x.
//
// Special cases are:
//
//	Ceil(±0) = ±0
//	Ceil(±Inf) = ±Inf
//	Ceil(NaN) = NaN
func Ceil(x float64) float64 {
	if haveArchCeil {
		return archCeil(x)
	}
	return ceil(x)
}

func ceil(x float64) float64 {
	return -Floor(-x)
}

// Trunc returns the integer value of x.
//
// Special cases are:
//
//	Trunc(±0) = ±0
//	Trunc(±Inf) = ±Inf
//	Trunc(NaN) = NaN
func Trunc(x float64) float64 {
	if haveArchTrunc {
		return archTrunc(x)
	}
	return trunc(x)
}

func trunc(x float64) float64 {
	if Abs(x) < 1 {
		return Copysign(0, x)
	}

	b := Float64bits(x)
	e := uint(b>>shift)&mask - bias

	// Keep the top 12+e bits, the integer part; clear the rest.
	if e < 64-12 {
		b &^= 1<<(64-12-e) - 1
	}
	return Float64frombits(b)
}

// Round returns the nearest integer, rounding half away from zero.
//
// Special cases are:
//
//	Round(±0) = ±0
//	Round(±Inf) = ±Inf
//	Round(NaN) = NaN
func Round(x float64) float64 {
	// Round is a faster implementation of:
	//
	// func Round(x float64) float64 {
	//   t := Trunc(x)
	//   if Abs(x-t) >= 0.5 {
	//     return t + Copysign(1, x)
	//   }
	//   return t
	// }
	bits := Float64bits(x)
	e := uint(bits>>shift) & mask
	if e < bias {
		// Round abs(x) < 1 including denormals.
		bits &= signMask // +-0
		if e == bias-1 {
			bits |= uvone // +-1
		}
	} else if e < bias+shift {
		// Round any abs(x) >= 1 containing a fractional component [0,1).
		//
		// Numbers with larger exponents are returned unchanged since they
		// must be either an integer, infinity, or NaN.
		const half = 1 << (shift - 1)
		e -= bias
		bits += half >> e
		bits &^= fracMask >> e
	}
	return Float64frombits(bits)
}

// RoundToEven returns the nearest integer, rounding ties to even.
//
// Special cases are:
//
//	RoundToEven(±0) = ±0
//	RoundToEven(±Inf) = ±Inf
//	RoundToEven(NaN) = NaN
func RoundToEven(x float64) float64 {
	// RoundToEven is a faster implementation of:
	//
	// func RoundToEven(x float64) float64 {
	//   t := math.Trunc(x)
	//   odd := math.Remainder(t, 2) != 0
	//   if d := math.Abs(x - t); d > 0.5 || (d == 0.5 && odd) {
	//     return t + math.Copysign(1, x)
	//   }
	//   return t
	// }
	bits := Float64bits(x)
	e := uint(bits>>shift) & mask
	if e >= bias {
		// Round abs(x) >= 1.
		// - Large numbers without fractional components, infinity, and NaN are unchanged.
		// - Add 0.499.. or 0.5 before truncating depending on whether the truncated
		//   number is even or odd (respectively).
		const halfMinusULP = (1 << (shift - 1)) - 1
		e -= bias
		bits += (halfMinusULP + (bits>>(shift-e))&1) >> e
		bits &^= fracMask >> e
	} else if e == bias-1 && bits&fracMask != 0 {
		// Round 0.5 < abs(x) < 1.
		bits = bits&signMask | uvone // +-1
	} else {
		// Round abs(x) <= 0.5 including denormals.
		bits &= signMask // +-0
	}
	return Float64frombits(bits)
}

```

// === FILE: references!/go/src/math/floor_386.s ===
```text
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// func archCeil(x float64) float64
TEXT ·archCeil(SB),NOSPLIT,$0
	FMOVD   x+0(FP), F0  // F0=x
	FSTCW   -2(SP)       // save old Control Word
	MOVW    -2(SP), AX
	ANDW    $0xf3ff, AX
	ORW     $0x0800, AX  // Rounding Control set to +Inf
	MOVW    AX, -4(SP)   // store new Control Word
	FLDCW   -4(SP)       // load new Control Word
	FRNDINT              // F0=Ceil(x)
	FLDCW   -2(SP)       // load old Control Word
	FMOVDP  F0, ret+8(FP)
	RET

// func archFloor(x float64) float64
TEXT ·archFloor(SB),NOSPLIT,$0
	FMOVD   x+0(FP), F0  // F0=x
	FSTCW   -2(SP)       // save old Control Word
	MOVW    -2(SP), AX
	ANDW    $0xf3ff, AX
	ORW     $0x0400, AX  // Rounding Control set to -Inf
	MOVW    AX, -4(SP)   // store new Control Word
	FLDCW   -4(SP)       // load new Control Word
	FRNDINT              // F0=Floor(x)
	FLDCW   -2(SP)       // load old Control Word
	FMOVDP  F0, ret+8(FP)
	RET

// func archTrunc(x float64) float64
TEXT ·archTrunc(SB),NOSPLIT,$0
	FMOVD   x+0(FP), F0  // F0=x
	FSTCW   -2(SP)       // save old Control Word
	MOVW    -2(SP), AX
	ORW     $0x0c00, AX  // Rounding Control set to truncate
	MOVW    AX, -4(SP)   // store new Control Word
	FLDCW   -4(SP)       // load new Control Word
	FRNDINT              // F0=Trunc(x)
	FLDCW   -2(SP)       // load old Control Word
	FMOVDP  F0, ret+8(FP)
	RET

```

// === FILE: references!/go/src/math/floor_amd64.s ===
```text
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

#define Big		0x4330000000000000 // 2**52

// func archFloor(x float64) float64
TEXT ·archFloor(SB),NOSPLIT,$0
	MOVQ	x+0(FP), AX
	MOVQ	$~(1<<63), DX // sign bit mask
	ANDQ	AX,DX // DX = |x|
	SUBQ	$1,DX
	MOVQ    $(Big - 1), CX // if |x| >= 2**52-1 or IsNaN(x) or |x| == 0, return x
	CMPQ	DX,CX
	JAE     isBig_floor
	MOVQ	AX, X0 // X0 = x
	CVTTSD2SQ	X0, AX
	CVTSQ2SD	AX, X1 // X1 = float(int(x))
	CMPSD	X1, X0, 1 // compare LT; X0 = 0xffffffffffffffff or 0
	MOVSD	$(-1.0), X2
	ANDPD	X2, X0 // if x < float(int(x)) {X0 = -1} else {X0 = 0}
	ADDSD	X1, X0
	MOVSD	X0, ret+8(FP)
	RET
isBig_floor:
	MOVQ    AX, ret+8(FP) // return x
	RET

// func archCeil(x float64) float64
TEXT ·archCeil(SB),NOSPLIT,$0
	MOVQ	x+0(FP), AX
	MOVQ	$~(1<<63), DX // sign bit mask
	MOVQ	AX, BX // BX = copy of x
	ANDQ    DX, BX // BX = |x|
	MOVQ    $Big, CX // if |x| >= 2**52 or IsNaN(x), return x
	CMPQ    BX, CX
	JAE     isBig_ceil
	MOVQ	AX, X0 // X0 = x
	MOVQ	DX, X2 // X2 = sign bit mask
	CVTTSD2SQ	X0, AX
	ANDNPD	X0, X2 // X2 = sign
	CVTSQ2SD	AX, X1	// X1 = float(int(x))
	CMPSD	X1, X0, 2 // compare LE; X0 = 0xffffffffffffffff or 0
	ORPD	X2, X1 // if X1 = 0.0, incorporate sign
	MOVSD	$1.0, X3
	ANDNPD	X3, X0
	ORPD	X2, X0 // if float(int(x)) <= x {X0 = 1} else {X0 = -0}
	ADDSD	X1, X0
	MOVSD	X0, ret+8(FP)
	RET
isBig_ceil:
	MOVQ	AX, ret+8(FP)
	RET

// func archTrunc(x float64) float64
TEXT ·archTrunc(SB),NOSPLIT,$0
	MOVQ	x+0(FP), AX
	MOVQ	$~(1<<63), DX // sign bit mask
	MOVQ	AX, BX // BX = copy of x
	ANDQ    DX, BX // BX = |x|
	MOVQ    $Big, CX // if |x| >= 2**52 or IsNaN(x), return x
	CMPQ    BX, CX
	JAE     isBig_trunc
	MOVQ	AX, X0
	MOVQ	DX, X2 // X2 = sign bit mask
	CVTTSD2SQ	X0, AX
	ANDNPD	X0, X2 // X2 = sign
	CVTSQ2SD	AX, X0 // X0 = float(int(x))
	ORPD	X2, X0 // if X0 = 0.0, incorporate sign
	MOVSD	X0, ret+8(FP)
	RET
isBig_trunc:
	MOVQ    AX, ret+8(FP) // return x
	RET

```

// === FILE: references!/go/src/math/floor_arm64.s ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// func archFloor(x float64) float64
TEXT ·archFloor(SB),NOSPLIT,$0
	FMOVD	x+0(FP), F0
	FRINTMD	F0, F0
	FMOVD	F0, ret+8(FP)
	RET

// func archCeil(x float64) float64
TEXT ·archCeil(SB),NOSPLIT,$0
	FMOVD	x+0(FP), F0
	FRINTPD	F0, F0
	FMOVD	F0, ret+8(FP)
	RET

// func archTrunc(x float64) float64
TEXT ·archTrunc(SB),NOSPLIT,$0
	FMOVD	x+0(FP), F0
	FRINTZD	F0, F0
	FMOVD	F0, ret+8(FP)
	RET

```

// === FILE: references!/go/src/math/floor_asm.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build 386 || amd64 || arm64 || loong64 || ppc64 || ppc64le || riscv64 || s390x || wasm

package math

const haveArchFloor = true

func archFloor(x float64) float64

const haveArchCeil = true

func archCeil(x float64) float64

const haveArchTrunc = true

func archTrunc(x float64) float64

```

// === FILE: references!/go/src/math/floor_loong64.s ===
```text
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// derived from math/floor_riscv64.s

#include "textflag.h"

#define ROUNDFN(NAME, FUNC)	\
TEXT NAME(SB),NOSPLIT,$0;	\
	MOVD	x+0(FP), F0;	\
	MOVV	F0, R11;	\
	/* 1023: bias of exponent, [-2^53, 2^53]: exactly integer represent range */;	\
	MOVV	$1023+53, R12;	\
	/* Drop all fraction bits */;	\
	SRLV	$52, R11, R11;	\
	/* Remove sign bit */;	\
	AND	$0x7FF, R11, R11;	\
	BLTU	R12, R11, isExtremum;	\
normal:;	\
	FUNC	F0, F2;	\
	MOVV	F2, R10;	\
	BEQ	R10, R0, is0;	\
	FFINTDV	F2, F0;	\
/* Return either input is +-Inf, NaN(0x7FF) or out of precision limitation */;	\
isExtremum:;	\
	MOVD	F0, ret+8(FP);	\
	RET;	\
is0:;	\
	FCOPYSGD	F0, F2, F2;	\
	MOVD	F2, ret+8(FP);	\
	RET

// func archFloor(x float64) float64
ROUNDFN(·archFloor, FTINTRMVD)

// func archCeil(x float64) float64
ROUNDFN(·archCeil, FTINTRPVD)

// func archTrunc(x float64) float64
ROUNDFN(·archTrunc, FTINTRZVD)

```

// === FILE: references!/go/src/math/floor_noasm.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !386 && !amd64 && !arm64 && !loong64 && !ppc64 && !ppc64le && !riscv64 && !s390x && !wasm

package math

const haveArchFloor = false

func archFloor(x float64) float64 {
	panic("not implemented")
}

const haveArchCeil = false

func archCeil(x float64) float64 {
	panic("not implemented")
}

const haveArchTrunc = false

func archTrunc(x float64) float64 {
	panic("not implemented")
}

```

// === FILE: references!/go/src/math/floor_ppc64x.s ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ppc64 || ppc64le

#include "textflag.h"

TEXT ·archFloor(SB),NOSPLIT,$0
	FMOVD   x+0(FP), F0
	FRIM	F0, F0
	FMOVD   F0, ret+8(FP)
	RET

TEXT ·archCeil(SB),NOSPLIT,$0
	FMOVD   x+0(FP), F0
	FRIP    F0, F0
	FMOVD	F0, ret+8(FP)
	RET

TEXT ·archTrunc(SB),NOSPLIT,$0
	FMOVD   x+0(FP), F0
	FRIZ    F0, F0
	FMOVD   F0, ret+8(FP)
	RET

```

// === FILE: references!/go/src/math/floor_riscv64.s ===
```text
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// RISC-V offered floating-point (FP) rounding by FP conversion instructions (FCVT)
// with rounding mode field.
// As Go spec expects FP rounding result in FP, we have to use FCVT integer
// back to FP (fp -> int -> fp).
// RISC-V only set Inexact flag during invalid FP-integer conversion without changing any data,
// on the other hand, RISC-V sets out of integer represent range yet valid FP into NaN.
// When it comes to integer-FP conversion, invalid FP like NaN, +-Inf will be
// converted into the closest valid FP, for example:
//
// `Floor(-Inf) -> int64(0x7fffffffffffffff) -> float64(9.22e+18)`
// `Floor(18446744073709549568.0) -> int64(0x7fffffffffffffff) -> float64(9.22e+18)`
//
// This ISA conversion limitation requires we skip all invalid or out of range FP
// before any normal rounding operations.

#define ROUNDFN(NAME, MODE) 	\
TEXT NAME(SB),NOSPLIT,$0; 	\
	MOVD	x+0(FP), F10; 	\
	FMVXD	F10, X10;	\
	/* Drop all fraction bits */;\
	SRL	$52, X10, X12;	\
	/* Remove sign bit */;	\
	AND	$0x7FF, X12, X12;\
	/* Return either input is +-Inf, NaN(0x7FF) or out of precision limitation */;\
	/* 1023: bias of exponent, [-2^53, 2^53]: exactly integer represent range */;\
	MOV	$1023+53, X11;	\
	BLTU	X11, X12, 4(PC);\
	FCVTLD.MODE F10, X11;	\
	FCVTDL	X11, F11;	\
	/* RISC-V rounds negative values to +0, restore original sign */;\
	FSGNJD	F10, F11, F10;	\
	MOVD	F10, ret+8(FP); \
	RET

// func archFloor(x float64) float64
ROUNDFN(·archFloor, RDN)

// func archCeil(x float64) float64
ROUNDFN(·archCeil, RUP)

// func archTrunc(x float64) float64
ROUNDFN(·archTrunc, RTZ)

```

// === FILE: references!/go/src/math/floor_s390x.s ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// func archFloor(x float64) float64
TEXT ·archFloor(SB),NOSPLIT,$0
	FMOVD	x+0(FP), F0
	FIDBR	$7, F0, F0
	FMOVD	F0, ret+8(FP)
	RET

// func archCeil(x float64) float64
TEXT ·archCeil(SB),NOSPLIT,$0
	FMOVD	x+0(FP), F0
	FIDBR	$6, F0, F0
	FMOVD	F0, ret+8(FP)
	RET

// func archTrunc(x float64) float64
TEXT ·archTrunc(SB),NOSPLIT,$0
	FMOVD	x+0(FP), F0
	FIDBR	$5, F0, F0
	FMOVD	F0, ret+8(FP)
	RET

```

// === FILE: references!/go/src/math/floor_wasm.s ===
```text
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

TEXT ·archFloor(SB),NOSPLIT,$0
	Get SP
	F64Load x+0(FP)
	F64Floor
	F64Store ret+8(FP)
	RET

TEXT ·archCeil(SB),NOSPLIT,$0
	Get SP
	F64Load x+0(FP)
	F64Ceil
	F64Store ret+8(FP)
	RET

TEXT ·archTrunc(SB),NOSPLIT,$0
	Get SP
	F64Load x+0(FP)
	F64Trunc
	F64Store ret+8(FP)
	RET

```

// === FILE: references!/go/src/math/fma.go ===
```go
// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

import "math/bits"

func zero(x uint64) uint64 {
	if x == 0 {
		return 1
	}
	return 0
	// branchless:
	// return ((x>>1 | x&1) - 1) >> 63
}

func nonzero(x uint64) uint64 {
	if x != 0 {
		return 1
	}
	return 0
	// branchless:
	// return 1 - ((x>>1|x&1)-1)>>63
}

func shl(u1, u2 uint64, n uint) (r1, r2 uint64) {
	r1 = u1<<n | u2>>(64-n) | u2<<(n-64)
	r2 = u2 << n
	return
}

func shr(u1, u2 uint64, n uint) (r1, r2 uint64) {
	r2 = u2>>n | u1<<(64-n) | u1>>(n-64)
	r1 = u1 >> n
	return
}

// shrcompress compresses the bottom n+1 bits of the two-word
// value into a single bit. the result is equal to the value
// shifted to the right by n, except the result's 0th bit is
// set to the bitwise OR of the bottom n+1 bits.
func shrcompress(u1, u2 uint64, n uint) (r1, r2 uint64) {
	// TODO: Performance here is really sensitive to the
	// order/placement of these branches. n == 0 is common
	// enough to be in the fast path. Perhaps more measurement
	// needs to be done to find the optimal order/placement?
	switch {
	case n == 0:
		return u1, u2
	case n == 64:
		return 0, u1 | nonzero(u2)
	case n >= 128:
		return 0, nonzero(u1 | u2)
	case n < 64:
		r1, r2 = shr(u1, u2, n)
		r2 |= nonzero(u2 & (1<<n - 1))
	case n < 128:
		r1, r2 = shr(u1, u2, n)
		r2 |= nonzero(u1&(1<<(n-64)-1) | u2)
	}
	return
}

func lz(u1, u2 uint64) (l int32) {
	l = int32(bits.LeadingZeros64(u1))
	if l == 64 {
		l += int32(bits.LeadingZeros64(u2))
	}
	return l
}

// split splits b into sign, biased exponent, and mantissa.
// It adds the implicit 1 bit to the mantissa for normal values,
// and normalizes subnormal values.
func split(b uint64) (sign uint32, exp int32, mantissa uint64) {
	sign = uint32(b >> 63)
	exp = int32(b>>52) & mask
	mantissa = b & fracMask

	if exp == 0 {
		// Normalize value if subnormal.
		shift := uint(bits.LeadingZeros64(mantissa) - 11)
		mantissa <<= shift
		exp = 1 - int32(shift)
	} else {
		// Add implicit 1 bit
		mantissa |= 1 << 52
	}
	return
}

// FMA returns x * y + z, computed with only one rounding.
// (That is, FMA returns the fused multiply-add of x, y, and z.)
func FMA(x, y, z float64) float64 {
	bx, by, bz := Float64bits(x), Float64bits(y), Float64bits(z)

	// Inf or NaN or zero involved. At most one rounding will occur.
	if x == 0.0 || y == 0.0 || bx&uvinf == uvinf || by&uvinf == uvinf {
		return x*y + z
	}
	// Handle z == 0.0 separately.
	// Adding zero usually does not change the original value.
	// However, there is an exception with negative zero. (e.g. (-0) + (+0) = (+0))
	// This applies when x * y is negative and underflows.
	if z == 0.0 {
		return x * y
	}
	// Handle non-finite z separately. Evaluating x*y+z where
	// x and y are finite, but z is infinite, should always result in z.
	if bz&uvinf == uvinf {
		return z
	}

	// Inputs are (sub)normal.
	// Split x, y, z into sign, exponent, mantissa.
	xs, xe, xm := split(bx)
	ys, ye, ym := split(by)
	zs, ze, zm := split(bz)

	// Compute product p = x*y as sign, exponent, two-word mantissa.
	// Start with exponent. "is normal" bit isn't subtracted yet.
	pe := xe + ye - bias + 1

	// pm1:pm2 is the double-word mantissa for the product p.
	// Shift left to leave top bit in product. Effectively
	// shifts the 106-bit product to the left by 21.
	pm1, pm2 := bits.Mul64(xm<<10, ym<<11)
	zm1, zm2 := zm<<10, uint64(0)
	ps := xs ^ ys // product sign

	// normalize to 62nd bit
	is62zero := uint((^pm1 >> 62) & 1)
	pm1, pm2 = shl(pm1, pm2, is62zero)
	pe -= int32(is62zero)

	// Swap addition operands so |p| >= |z|
	if pe < ze || pe == ze && pm1 < zm1 {
		ps, pe, pm1, pm2, zs, ze, zm1, zm2 = zs, ze, zm1, zm2, ps, pe, pm1, pm2
	}

	// Special case: if p == -z the result is always +0 since neither operand is zero.
	if ps != zs && pe == ze && pm1 == zm1 && pm2 == zm2 {
		return 0
	}

	// Align significands
	zm1, zm2 = shrcompress(zm1, zm2, uint(pe-ze))

	// Compute resulting significands, normalizing if necessary.
	var m, c uint64
	if ps == zs {
		// Adding (pm1:pm2) + (zm1:zm2)
		pm2, c = bits.Add64(pm2, zm2, 0)
		pm1, _ = bits.Add64(pm1, zm1, c)
		pe -= int32(^pm1 >> 63)
		pm1, m = shrcompress(pm1, pm2, uint(64+pm1>>63))
	} else {
		// Subtracting (pm1:pm2) - (zm1:zm2)
		// TODO: should we special-case cancellation?
		pm2, c = bits.Sub64(pm2, zm2, 0)
		pm1, _ = bits.Sub64(pm1, zm1, c)
		nz := lz(pm1, pm2)
		pe -= nz
		m, pm2 = shl(pm1, pm2, uint(nz-1))
		m |= nonzero(pm2)
	}

	// Round and break ties to even
	if pe > 1022+bias || pe == 1022+bias && (m+1<<9)>>63 == 1 {
		// rounded value overflows exponent range
		return Float64frombits(uint64(ps)<<63 | uvinf)
	}
	if pe < 0 {
		n := uint(-pe)
		m = m>>n | nonzero(m&(1<<n-1))
		pe = 0
	}
	m = ((m + 1<<9) >> 10) & ^zero((m&(1<<10-1))^1<<9)
	pe &= -int32(nonzero(m))
	return Float64frombits(uint64(ps)<<63 + uint64(pe)<<52 + m)
}

```

// === FILE: references!/go/src/math/frexp.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// Frexp breaks f into a normalized fraction
// and an integral power of two.
// It returns frac and exp satisfying f == frac × 2**exp,
// with the absolute value of frac in the interval [½, 1).
//
// Special cases are:
//
//	Frexp(±0) = ±0, 0
//	Frexp(±Inf) = ±Inf, 0
//	Frexp(NaN) = NaN, 0
func Frexp(f float64) (frac float64, exp int) {
	if haveArchFrexp {
		return archFrexp(f)
	}
	return frexp(f)
}

func frexp(f float64) (frac float64, exp int) {
	// special cases
	switch {
	case f == 0:
		return f, 0 // correctly return -0
	case IsInf(f, 0) || IsNaN(f):
		return f, 0
	}
	f, exp = normalize(f)
	x := Float64bits(f)
	exp += int((x>>shift)&mask) - bias + 1
	x &^= mask << shift
	x |= (-1 + bias) << shift
	frac = Float64frombits(x)
	return
}

```

// === FILE: references!/go/src/math/gamma.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// The original C code, the long comment, and the constants
// below are from http://netlib.sandia.gov/cephes/cprob/gamma.c.
// The go code is a simplified version of the original C.
//
//      tgamma.c
//
//      Gamma function
//
// SYNOPSIS:
//
// double x, y, tgamma();
// extern int signgam;
//
// y = tgamma( x );
//
// DESCRIPTION:
//
// Returns gamma function of the argument. The result is
// correctly signed, and the sign (+1 or -1) is also
// returned in a global (extern) variable named signgam.
// This variable is also filled in by the logarithmic gamma
// function lgamma().
//
// Arguments |x| <= 34 are reduced by recurrence and the function
// approximated by a rational function of degree 6/7 in the
// interval (2,3).  Large arguments are handled by Stirling's
// formula. Large negative arguments are made positive using
// a reflection formula.
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    DEC      -34, 34      10000       1.3e-16     2.5e-17
//    IEEE    -170,-33      20000       2.3e-15     3.3e-16
//    IEEE     -33,  33     20000       9.4e-16     2.2e-16
//    IEEE      33, 171.6   20000       2.3e-15     3.2e-16
//
// Error for arguments outside the test range will be larger
// owing to error amplification by the exponential function.
//
// Cephes Math Library Release 2.8:  June, 2000
// Copyright 1984, 1987, 1989, 1992, 2000 by Stephen L. Moshier
//
// The readme file at http://netlib.sandia.gov/cephes/ says:
//    Some software in this archive may be from the book _Methods and
// Programs for Mathematical Functions_ (Prentice-Hall or Simon & Schuster
// International, 1989) or from the Cephes Mathematical Library, a
// commercial product. In either event, it is copyrighted by the author.
// What you see here may be used freely but it comes with no support or
// guarantee.
//
//   The two known misprints in the book are repaired here in the
// source listings for the gamma function and the incomplete beta
// integral.
//
//   Stephen L. Moshier
//   moshier@na-net.ornl.gov

var _gamP = [...]float64{
	1.60119522476751861407e-04,
	1.19135147006586384913e-03,
	1.04213797561761569935e-02,
	4.76367800457137231464e-02,
	2.07448227648435975150e-01,
	4.94214826801497100753e-01,
	9.99999999999999996796e-01,
}
var _gamQ = [...]float64{
	-2.31581873324120129819e-05,
	5.39605580493303397842e-04,
	-4.45641913851797240494e-03,
	1.18139785222060435552e-02,
	3.58236398605498653373e-02,
	-2.34591795718243348568e-01,
	7.14304917030273074085e-02,
	1.00000000000000000320e+00,
}
var _gamS = [...]float64{
	7.87311395793093628397e-04,
	-2.29549961613378126380e-04,
	-2.68132617805781232825e-03,
	3.47222221605458667310e-03,
	8.33333333333482257126e-02,
}

// Gamma function computed by Stirling's formula.
// The pair of results must be multiplied together to get the actual answer.
// The multiplication is left to the caller so that, if careful, the caller can avoid
// infinity for 172 <= x <= 180.
// The polynomial is valid for 33 <= x <= 172; larger values are only used
// in reciprocal and produce denormalized floats. The lower precision there
// masks any imprecision in the polynomial.
func stirling(x float64) (float64, float64) {
	if x > 200 {
		return Inf(1), 1
	}
	const (
		SqrtTwoPi   = 2.506628274631000502417
		MaxStirling = 143.01608
	)
	w := 1 / x
	w = 1 + w*((((_gamS[0]*w+_gamS[1])*w+_gamS[2])*w+_gamS[3])*w+_gamS[4])
	y1 := Exp(x)
	y2 := 1.0
	if x > MaxStirling { // avoid Pow() overflow
		v := Pow(x, 0.5*x-0.25)
		y1, y2 = v, v/y1
	} else {
		y1 = Pow(x, x-0.5) / y1
	}
	return y1, SqrtTwoPi * w * y2
}

// Gamma returns the Gamma function of x.
//
// Special cases are:
//
//	Gamma(+Inf) = +Inf
//	Gamma(+0) = +Inf
//	Gamma(-0) = -Inf
//	Gamma(x) = NaN for integer x < 0
//	Gamma(-Inf) = NaN
//	Gamma(NaN) = NaN
func Gamma(x float64) float64 {
	const Euler = 0.57721566490153286060651209008240243104215933593992 // A001620
	// special cases
	switch {
	case isNegInt(x) || IsInf(x, -1) || IsNaN(x):
		return NaN()
	case IsInf(x, 1):
		return Inf(1)
	case x == 0:
		if Signbit(x) {
			return Inf(-1)
		}
		return Inf(1)
	}
	q := Abs(x)
	p := Floor(q)
	if q > 33 {
		if x >= 0 {
			y1, y2 := stirling(x)
			return y1 * y2
		}
		// Note: x is negative but (checked above) not a negative integer,
		// so x must be small enough to be in range for conversion to int64.
		// If |x| were >= 2⁶³ it would have to be an integer.
		signgam := 1
		if ip := int64(p); ip&1 == 0 {
			signgam = -1
		}
		z := q - p
		if z > 0.5 {
			p = p + 1
			z = q - p
		}
		z = q * Sin(Pi*z)
		if z == 0 {
			return Inf(signgam)
		}
		sq1, sq2 := stirling(q)
		absz := Abs(z)
		d := absz * sq1 * sq2
		if IsInf(d, 0) {
			z = Pi / absz / sq1 / sq2
		} else {
			z = Pi / d
		}
		return float64(signgam) * z
	}

	// Reduce argument
	z := 1.0
	for x >= 3 {
		x = x - 1
		z = z * x
	}
	for x < 0 {
		if x > -1e-09 {
			goto small
		}
		z = z / x
		x = x + 1
	}
	for x < 2 {
		if x < 1e-09 {
			goto small
		}
		z = z / x
		x = x + 1
	}

	if x == 2 {
		return z
	}

	x = x - 2
	p = (((((x*_gamP[0]+_gamP[1])*x+_gamP[2])*x+_gamP[3])*x+_gamP[4])*x+_gamP[5])*x + _gamP[6]
	q = ((((((x*_gamQ[0]+_gamQ[1])*x+_gamQ[2])*x+_gamQ[3])*x+_gamQ[4])*x+_gamQ[5])*x+_gamQ[6])*x + _gamQ[7]
	return z * p / q

small:
	if x == 0 {
		return Inf(1)
	}
	return z / ((1 + Euler*x) * x)
}

func isNegInt(x float64) bool {
	if x < 0 {
		_, xf := Modf(x)
		return xf == 0
	}
	return false
}

```

// === FILE: references!/go/src/math/hypot.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

/*
	Hypot -- sqrt(p*p + q*q), but overflows only if the result does.
*/

// Hypot returns [Sqrt](p*p + q*q), taking care to avoid
// unnecessary overflow and underflow.
//
// Special cases are:
//
//	Hypot(±Inf, q) = +Inf
//	Hypot(p, ±Inf) = +Inf
//	Hypot(NaN, q) = NaN
//	Hypot(p, NaN) = NaN
func Hypot(p, q float64) float64 {
	if haveArchHypot {
		return archHypot(p, q)
	}
	return hypot(p, q)
}

func hypot(p, q float64) float64 {
	p, q = Abs(p), Abs(q)
	// special cases
	switch {
	case IsInf(p, 1) || IsInf(q, 1):
		return Inf(1)
	case IsNaN(p) || IsNaN(q):
		return NaN()
	}
	if p < q {
		p, q = q, p
	}
	if p == 0 {
		return 0
	}
	q = q / p
	return p * Sqrt(1+q*q)
}

```

// === FILE: references!/go/src/math/hypot_386.s ===
```text
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// func archHypot(p, q float64) float64
TEXT ·archHypot(SB),NOSPLIT,$0
// test bits for not-finite
	MOVL    p_hi+4(FP), AX   // high word p
	ANDL    $0x7ff00000, AX
	CMPL    AX, $0x7ff00000
	JEQ     not_finite
	MOVL    q_hi+12(FP), AX   // high word q
	ANDL    $0x7ff00000, AX
	CMPL    AX, $0x7ff00000
	JEQ     not_finite
	FMOVD   p+0(FP), F0  // F0=p
	FABS                 // F0=|p|
	FMOVD   q+8(FP), F0  // F0=q, F1=|p|
	FABS                 // F0=|q|, F1=|p|
	FUCOMI  F0, F1       // compare F0 to F1
	JCC     2(PC)        // jump if F0 >= F1
	FXCHD   F0, F1       // F0=|p| (larger), F1=|q| (smaller)
	FTST                 // compare F0 to 0
	FSTSW	AX
	ANDW    $0x4000, AX
	JNE     10(PC)       // jump if F0 = 0
	FXCHD   F0, F1       // F0=q (smaller), F1=p (larger)
	FDIVD   F1, F0       // F0=q(=q/p), F1=p
	FMULD   F0, F0       // F0=q*q, F1=p
	FLD1                 // F0=1, F1=q*q, F2=p
	FADDDP  F0, F1       // F0=1+q*q, F1=p
	FSQRT                // F0=sqrt(1+q*q), F1=p
	FMULDP  F0, F1       // F0=p*sqrt(1+q*q)
	FMOVDP  F0, ret+16(FP)
	RET
	FMOVDP  F0, F1       // F0=0
	FMOVDP  F0, ret+16(FP)
	RET
not_finite:
// test bits for -Inf or +Inf
	MOVL    p_hi+4(FP), AX  // high word p
	ORL     p_lo+0(FP), AX  // low word p
	ANDL    $0x7fffffff, AX
	CMPL    AX, $0x7ff00000
	JEQ     is_inf
	MOVL    q_hi+12(FP), AX  // high word q
	ORL     q_lo+8(FP), AX   // low word q
	ANDL    $0x7fffffff, AX
	CMPL    AX, $0x7ff00000
	JEQ     is_inf
	MOVL    $0x7ff80000, ret_hi+20(FP)  // return NaN = 0x7FF8000000000001
	MOVL    $0x00000001, ret_lo+16(FP)
	RET
is_inf:
	MOVL    AX, ret_hi+20(FP)  // return +Inf = 0x7FF0000000000000
	MOVL    $0x00000000, ret_lo+16(FP)
	RET

```

// === FILE: references!/go/src/math/hypot_amd64.s ===
```text
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

#define PosInf 0x7FF0000000000000
#define NaN 0x7FF8000000000001

// func archHypot(p, q float64) float64
TEXT ·archHypot(SB),NOSPLIT,$0
	// test bits for special cases
	MOVQ    p+0(FP), BX
	MOVQ    $~(1<<63), AX
	ANDQ    AX, BX // p = |p|
	MOVQ    q+8(FP), CX
	ANDQ    AX, CX // q = |q|
	MOVQ    $PosInf, AX
	CMPQ    AX, BX
	JLE     isInfOrNaN
	CMPQ    AX, CX
	JLE     isInfOrNaN
	// hypot = max * sqrt(1 + (min/max)**2)
	MOVQ    BX, X0
	MOVQ    CX, X1
	ORQ     CX, BX
	JEQ     isZero
	MOVAPD  X0, X2
	MAXSD   X1, X0
	MINSD   X2, X1
	DIVSD   X0, X1
	MULSD   X1, X1
	ADDSD   $1.0, X1
	SQRTSD  X1, X1
	MULSD   X1, X0
	MOVSD   X0, ret+16(FP)
	RET
isInfOrNaN:
	CMPQ    AX, BX
	JEQ     isInf
	CMPQ    AX, CX
	JEQ     isInf
	MOVQ    $NaN, AX
	MOVQ    AX, ret+16(FP) // return NaN
	RET
isInf:
	MOVQ    AX, ret+16(FP) // return +Inf
	RET
isZero:
	MOVQ    $0, AX
	MOVQ    AX, ret+16(FP) // return 0
	RET

```

// === FILE: references!/go/src/math/hypot_asm.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build 386 || amd64

package math

const haveArchHypot = true

func archHypot(p, q float64) float64

```

// === FILE: references!/go/src/math/hypot_noasm.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !386 && !amd64

package math

const haveArchHypot = false

func archHypot(p, q float64) float64 {
	panic("not implemented")
}

```

// === FILE: references!/go/src/math/j0.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

/*
	Bessel function of the first and second kinds of order zero.
*/

// The original C code and the long comment below are
// from FreeBSD's /usr/src/lib/msun/src/e_j0.c and
// came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
// __ieee754_j0(x), __ieee754_y0(x)
// Bessel function of the first and second kinds of order zero.
// Method -- j0(x):
//      1. For tiny x, we use j0(x) = 1 - x**2/4 + x**4/64 - ...
//      2. Reduce x to |x| since j0(x)=j0(-x),  and
//         for x in (0,2)
//              j0(x) = 1-z/4+ z**2*R0/S0,  where z = x*x;
//         (precision:  |j0-1+z/4-z**2R0/S0 |<2**-63.67 )
//         for x in (2,inf)
//              j0(x) = sqrt(2/(pi*x))*(p0(x)*cos(x0)-q0(x)*sin(x0))
//         where x0 = x-pi/4. It is better to compute sin(x0),cos(x0)
//         as follow:
//              cos(x0) = cos(x)cos(pi/4)+sin(x)sin(pi/4)
//                      = 1/sqrt(2) * (cos(x) + sin(x))
//              sin(x0) = sin(x)cos(pi/4)-cos(x)sin(pi/4)
//                      = 1/sqrt(2) * (sin(x) - cos(x))
//         (To avoid cancellation, use
//              sin(x) +- cos(x) = -cos(2x)/(sin(x) -+ cos(x))
//         to compute the worse one.)
//
//      3 Special cases
//              j0(nan)= nan
//              j0(0) = 1
//              j0(inf) = 0
//
// Method -- y0(x):
//      1. For x<2.
//         Since
//              y0(x) = 2/pi*(j0(x)*(ln(x/2)+Euler) + x**2/4 - ...)
//         therefore y0(x)-2/pi*j0(x)*ln(x) is an even function.
//         We use the following function to approximate y0,
//              y0(x) = U(z)/V(z) + (2/pi)*(j0(x)*ln(x)), z= x**2
//         where
//              U(z) = u00 + u01*z + ... + u06*z**6
//              V(z) = 1  + v01*z + ... + v04*z**4
//         with absolute approximation error bounded by 2**-72.
//         Note: For tiny x, U/V = u0 and j0(x)~1, hence
//              y0(tiny) = u0 + (2/pi)*ln(tiny), (choose tiny<2**-27)
//      2. For x>=2.
//              y0(x) = sqrt(2/(pi*x))*(p0(x)*cos(x0)+q0(x)*sin(x0))
//         where x0 = x-pi/4. It is better to compute sin(x0),cos(x0)
//         by the method mentioned above.
//      3. Special cases: y0(0)=-inf, y0(x<0)=NaN, y0(inf)=0.
//

// J0 returns the order-zero Bessel function of the first kind.
//
// Special cases are:
//
//	J0(±Inf) = 0
//	J0(0) = 1
//	J0(NaN) = NaN
func J0(x float64) float64 {
	const (
		Huge   = 1e300
		TwoM27 = 1.0 / (1 << 27) // 2**-27 0x3e40000000000000
		TwoM13 = 1.0 / (1 << 13) // 2**-13 0x3f20000000000000
		Two129 = 1 << 129        // 2**129 0x4800000000000000
		// R0/S0 on [0, 2]
		R02 = 1.56249999999999947958e-02  // 0x3F8FFFFFFFFFFFFD
		R03 = -1.89979294238854721751e-04 // 0xBF28E6A5B61AC6E9
		R04 = 1.82954049532700665670e-06  // 0x3EBEB1D10C503919
		R05 = -4.61832688532103189199e-09 // 0xBE33D5E773D63FCE
		S01 = 1.56191029464890010492e-02  // 0x3F8FFCE882C8C2A4
		S02 = 1.16926784663337450260e-04  // 0x3F1EA6D2DD57DBF4
		S03 = 5.13546550207318111446e-07  // 0x3EA13B54CE84D5A9
		S04 = 1.16614003333790000205e-09  // 0x3E1408BCF4745D8F
	)
	// special cases
	switch {
	case IsNaN(x):
		return x
	case IsInf(x, 0):
		return 0
	case x == 0:
		return 1
	}

	x = Abs(x)
	if x >= 2 {
		s, c := Sincos(x)
		ss := s - c
		cc := s + c

		// make sure x+x does not overflow
		if x < MaxFloat64/2 {
			z := -Cos(x + x)
			if s*c < 0 {
				cc = z / ss
			} else {
				ss = z / cc
			}
		}

		// j0(x) = 1/sqrt(pi) * (P(0,x)*cc - Q(0,x)*ss) / sqrt(x)
		// y0(x) = 1/sqrt(pi) * (P(0,x)*ss + Q(0,x)*cc) / sqrt(x)

		var z float64
		if x > Two129 { // |x| > ~6.8056e+38
			z = (1 / SqrtPi) * cc / Sqrt(x)
		} else {
			u := pzero(x)
			v := qzero(x)
			z = (1 / SqrtPi) * (u*cc - v*ss) / Sqrt(x)
		}
		return z // |x| >= 2.0
	}
	if x < TwoM13 { // |x| < ~1.2207e-4
		if x < TwoM27 {
			return 1 // |x| < ~7.4506e-9
		}
		return 1 - 0.25*x*x // ~7.4506e-9 < |x| < ~1.2207e-4
	}
	z := x * x
	r := z * (R02 + z*(R03+z*(R04+z*R05)))
	s := 1 + z*(S01+z*(S02+z*(S03+z*S04)))
	if x < 1 {
		return 1 + z*(-0.25+(r/s)) // |x| < 1.00
	}
	u := 0.5 * x
	return (1+u)*(1-u) + z*(r/s) // 1.0 < |x| < 2.0
}

// Y0 returns the order-zero Bessel function of the second kind.
//
// Special cases are:
//
//	Y0(+Inf) = 0
//	Y0(0) = -Inf
//	Y0(x < 0) = NaN
//	Y0(NaN) = NaN
func Y0(x float64) float64 {
	const (
		TwoM27 = 1.0 / (1 << 27)             // 2**-27 0x3e40000000000000
		Two129 = 1 << 129                    // 2**129 0x4800000000000000
		U00    = -7.38042951086872317523e-02 // 0xBFB2E4D699CBD01F
		U01    = 1.76666452509181115538e-01  // 0x3FC69D019DE9E3FC
		U02    = -1.38185671945596898896e-02 // 0xBF8C4CE8B16CFA97
		U03    = 3.47453432093683650238e-04  // 0x3F36C54D20B29B6B
		U04    = -3.81407053724364161125e-06 // 0xBECFFEA773D25CAD
		U05    = 1.95590137035022920206e-08  // 0x3E5500573B4EABD4
		U06    = -3.98205194132103398453e-11 // 0xBDC5E43D693FB3C8
		V01    = 1.27304834834123699328e-02  // 0x3F8A127091C9C71A
		V02    = 7.60068627350353253702e-05  // 0x3F13ECBBF578C6C1
		V03    = 2.59150851840457805467e-07  // 0x3E91642D7FF202FD
		V04    = 4.41110311332675467403e-10  // 0x3DFE50183BD6D9EF
	)
	// special cases
	switch {
	case x < 0 || IsNaN(x):
		return NaN()
	case IsInf(x, 1):
		return 0
	case x == 0:
		return Inf(-1)
	}

	if x >= 2 { // |x| >= 2.0

		// y0(x) = sqrt(2/(pi*x))*(p0(x)*sin(x0)+q0(x)*cos(x0))
		//     where x0 = x-pi/4
		// Better formula:
		//     cos(x0) = cos(x)cos(pi/4)+sin(x)sin(pi/4)
		//             =  1/sqrt(2) * (sin(x) + cos(x))
		//     sin(x0) = sin(x)cos(3pi/4)-cos(x)sin(3pi/4)
		//             =  1/sqrt(2) * (sin(x) - cos(x))
		// To avoid cancellation, use
		//     sin(x) +- cos(x) = -cos(2x)/(sin(x) -+ cos(x))
		// to compute the worse one.

		s, c := Sincos(x)
		ss := s - c
		cc := s + c

		// j0(x) = 1/sqrt(pi) * (P(0,x)*cc - Q(0,x)*ss) / sqrt(x)
		// y0(x) = 1/sqrt(pi) * (P(0,x)*ss + Q(0,x)*cc) / sqrt(x)

		// make sure x+x does not overflow
		if x < MaxFloat64/2 {
			z := -Cos(x + x)
			if s*c < 0 {
				cc = z / ss
			} else {
				ss = z / cc
			}
		}
		var z float64
		if x > Two129 { // |x| > ~6.8056e+38
			z = (1 / SqrtPi) * ss / Sqrt(x)
		} else {
			u := pzero(x)
			v := qzero(x)
			z = (1 / SqrtPi) * (u*ss + v*cc) / Sqrt(x)
		}
		return z // |x| >= 2.0
	}
	if x <= TwoM27 {
		return U00 + (2/Pi)*Log(x) // |x| < ~7.4506e-9
	}
	z := x * x
	u := U00 + z*(U01+z*(U02+z*(U03+z*(U04+z*(U05+z*U06)))))
	v := 1 + z*(V01+z*(V02+z*(V03+z*V04)))
	return u/v + (2/Pi)*J0(x)*Log(x) // ~7.4506e-9 < |x| < 2.0
}

// The asymptotic expansions of pzero is
//      1 - 9/128 s**2 + 11025/98304 s**4 - ..., where s = 1/x.
// For x >= 2, We approximate pzero by
// 	pzero(x) = 1 + (R/S)
// where  R = pR0 + pR1*s**2 + pR2*s**4 + ... + pR5*s**10
// 	  S = 1 + pS0*s**2 + ... + pS4*s**10
// and
//      | pzero(x)-1-R/S | <= 2  ** ( -60.26)

// for x in [inf, 8]=1/[0,0.125]
var p0R8 = [6]float64{
	0.00000000000000000000e+00,  // 0x0000000000000000
	-7.03124999999900357484e-02, // 0xBFB1FFFFFFFFFD32
	-8.08167041275349795626e+00, // 0xC02029D0B44FA779
	-2.57063105679704847262e+02, // 0xC07011027B19E863
	-2.48521641009428822144e+03, // 0xC0A36A6ECD4DCAFC
	-5.25304380490729545272e+03, // 0xC0B4850B36CC643D
}
var p0S8 = [5]float64{
	1.16534364619668181717e+02, // 0x405D223307A96751
	3.83374475364121826715e+03, // 0x40ADF37D50596938
	4.05978572648472545552e+04, // 0x40E3D2BB6EB6B05F
	1.16752972564375915681e+05, // 0x40FC810F8F9FA9BD
	4.76277284146730962675e+04, // 0x40E741774F2C49DC
}

// for x in [8,4.5454]=1/[0.125,0.22001]
var p0R5 = [6]float64{
	-1.14125464691894502584e-11, // 0xBDA918B147E495CC
	-7.03124940873599280078e-02, // 0xBFB1FFFFE69AFBC6
	-4.15961064470587782438e+00, // 0xC010A370F90C6BBF
	-6.76747652265167261021e+01, // 0xC050EB2F5A7D1783
	-3.31231299649172967747e+02, // 0xC074B3B36742CC63
	-3.46433388365604912451e+02, // 0xC075A6EF28A38BD7
}
var p0S5 = [5]float64{
	6.07539382692300335975e+01, // 0x404E60810C98C5DE
	1.05125230595704579173e+03, // 0x40906D025C7E2864
	5.97897094333855784498e+03, // 0x40B75AF88FBE1D60
	9.62544514357774460223e+03, // 0x40C2CCB8FA76FA38
	2.40605815922939109441e+03, // 0x40A2CC1DC70BE864
}

// for x in [4.547,2.8571]=1/[0.2199,0.35001]
var p0R3 = [6]float64{
	-2.54704601771951915620e-09, // 0xBE25E1036FE1AA86
	-7.03119616381481654654e-02, // 0xBFB1FFF6F7C0E24B
	-2.40903221549529611423e+00, // 0xC00345B2AEA48074
	-2.19659774734883086467e+01, // 0xC035F74A4CB94E14
	-5.80791704701737572236e+01, // 0xC04D0A22420A1A45
	-3.14479470594888503854e+01, // 0xC03F72ACA892D80F
}
var p0S3 = [5]float64{
	3.58560338055209726349e+01, // 0x4041ED9284077DD3
	3.61513983050303863820e+02, // 0x40769839464A7C0E
	1.19360783792111533330e+03, // 0x4092A66E6D1061D6
	1.12799679856907414432e+03, // 0x40919FFCB8C39B7E
	1.73580930813335754692e+02, // 0x4065B296FC379081
}

// for x in [2.8570,2]=1/[0.3499,0.5]
var p0R2 = [6]float64{
	-8.87534333032526411254e-08, // 0xBE77D316E927026D
	-7.03030995483624743247e-02, // 0xBFB1FF62495E1E42
	-1.45073846780952986357e+00, // 0xBFF736398A24A843
	-7.63569613823527770791e+00, // 0xC01E8AF3EDAFA7F3
	-1.11931668860356747786e+01, // 0xC02662E6C5246303
	-3.23364579351335335033e+00, // 0xC009DE81AF8FE70F
}
var p0S2 = [5]float64{
	2.22202997532088808441e+01, // 0x40363865908B5959
	1.36206794218215208048e+02, // 0x4061069E0EE8878F
	2.70470278658083486789e+02, // 0x4070E78642EA079B
	1.53875394208320329881e+02, // 0x40633C033AB6FAFF
	1.46576176948256193810e+01, // 0x402D50B344391809
}

func pzero(x float64) float64 {
	var p *[6]float64
	var q *[5]float64
	if x >= 8 {
		p = &p0R8
		q = &p0S8
	} else if x >= 4.5454 {
		p = &p0R5
		q = &p0S5
	} else if x >= 2.8571 {
		p = &p0R3
		q = &p0S3
	} else if x >= 2 {
		p = &p0R2
		q = &p0S2
	}
	z := 1 / (x * x)
	r := p[0] + z*(p[1]+z*(p[2]+z*(p[3]+z*(p[4]+z*p[5]))))
	s := 1 + z*(q[0]+z*(q[1]+z*(q[2]+z*(q[3]+z*q[4]))))
	return 1 + r/s
}

// For x >= 8, the asymptotic expansions of qzero is
//      -1/8 s + 75/1024 s**3 - ..., where s = 1/x.
// We approximate pzero by
//      qzero(x) = s*(-1.25 + (R/S))
// where R = qR0 + qR1*s**2 + qR2*s**4 + ... + qR5*s**10
//       S = 1 + qS0*s**2 + ... + qS5*s**12
// and
//      | qzero(x)/s +1.25-R/S | <= 2**(-61.22)

// for x in [inf, 8]=1/[0,0.125]
var q0R8 = [6]float64{
	0.00000000000000000000e+00, // 0x0000000000000000
	7.32421874999935051953e-02, // 0x3FB2BFFFFFFFFE2C
	1.17682064682252693899e+01, // 0x402789525BB334D6
	5.57673380256401856059e+02, // 0x40816D6315301825
	8.85919720756468632317e+03, // 0x40C14D993E18F46D
	3.70146267776887834771e+04, // 0x40E212D40E901566
}
var q0S8 = [6]float64{
	1.63776026895689824414e+02,  // 0x406478D5365B39BC
	8.09834494656449805916e+03,  // 0x40BFA2584E6B0563
	1.42538291419120476348e+05,  // 0x4101665254D38C3F
	8.03309257119514397345e+05,  // 0x412883DA83A52B43
	8.40501579819060512818e+05,  // 0x4129A66B28DE0B3D
	-3.43899293537866615225e+05, // 0xC114FD6D2C9530C5
}

// for x in [8,4.5454]=1/[0.125,0.22001]
var q0R5 = [6]float64{
	1.84085963594515531381e-11, // 0x3DB43D8F29CC8CD9
	7.32421766612684765896e-02, // 0x3FB2BFFFD172B04C
	5.83563508962056953777e+00, // 0x401757B0B9953DD3
	1.35111577286449829671e+02, // 0x4060E3920A8788E9
	1.02724376596164097464e+03, // 0x40900CF99DC8C481
	1.98997785864605384631e+03, // 0x409F17E953C6E3A6
}
var q0S5 = [6]float64{
	8.27766102236537761883e+01,  // 0x4054B1B3FB5E1543
	2.07781416421392987104e+03,  // 0x40A03BA0DA21C0CE
	1.88472887785718085070e+04,  // 0x40D267D27B591E6D
	5.67511122894947329769e+04,  // 0x40EBB5E397E02372
	3.59767538425114471465e+04,  // 0x40E191181F7A54A0
	-5.35434275601944773371e+03, // 0xC0B4EA57BEDBC609
}

// for x in [4.547,2.8571]=1/[0.2199,0.35001]
var q0R3 = [6]float64{
	4.37741014089738620906e-09, // 0x3E32CD036ADECB82
	7.32411180042911447163e-02, // 0x3FB2BFEE0E8D0842
	3.34423137516170720929e+00, // 0x400AC0FC61149CF5
	4.26218440745412650017e+01, // 0x40454F98962DAEDD
	1.70808091340565596283e+02, // 0x406559DBE25EFD1F
	1.66733948696651168575e+02, // 0x4064D77C81FA21E0
}
var q0S3 = [6]float64{
	4.87588729724587182091e+01,  // 0x40486122BFE343A6
	7.09689221056606015736e+02,  // 0x40862D8386544EB3
	3.70414822620111362994e+03,  // 0x40ACF04BE44DFC63
	6.46042516752568917582e+03,  // 0x40B93C6CD7C76A28
	2.51633368920368957333e+03,  // 0x40A3A8AAD94FB1C0
	-1.49247451836156386662e+02, // 0xC062A7EB201CF40F
}

// for x in [2.8570,2]=1/[0.3499,0.5]
var q0R2 = [6]float64{
	1.50444444886983272379e-07, // 0x3E84313B54F76BDB
	7.32234265963079278272e-02, // 0x3FB2BEC53E883E34
	1.99819174093815998816e+00, // 0x3FFFF897E727779C
	1.44956029347885735348e+01, // 0x402CFDBFAAF96FE5
	3.16662317504781540833e+01, // 0x403FAA8E29FBDC4A
	1.62527075710929267416e+01, // 0x403040B171814BB4
}
var q0S2 = [6]float64{
	3.03655848355219184498e+01,  // 0x403E5D96F7C07AED
	2.69348118608049844624e+02,  // 0x4070D591E4D14B40
	8.44783757595320139444e+02,  // 0x408A664522B3BF22
	8.82935845112488550512e+02,  // 0x408B977C9C5CC214
	2.12666388511798828631e+02,  // 0x406A95530E001365
	-5.31095493882666946917e+00, // 0xC0153E6AF8B32931
}

func qzero(x float64) float64 {
	var p, q *[6]float64
	if x >= 8 {
		p = &q0R8
		q = &q0S8
	} else if x >= 4.5454 {
		p = &q0R5
		q = &q0S5
	} else if x >= 2.8571 {
		p = &q0R3
		q = &q0S3
	} else if x >= 2 {
		p = &q0R2
		q = &q0S2
	}
	z := 1 / (x * x)
	r := p[0] + z*(p[1]+z*(p[2]+z*(p[3]+z*(p[4]+z*p[5]))))
	s := 1 + z*(q[0]+z*(q[1]+z*(q[2]+z*(q[3]+z*(q[4]+z*q[5])))))
	return (-0.125 + r/s) / x
}

```

// === FILE: references!/go/src/math/j1.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

/*
	Bessel function of the first and second kinds of order one.
*/

// The original C code and the long comment below are
// from FreeBSD's /usr/src/lib/msun/src/e_j1.c and
// came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
// __ieee754_j1(x), __ieee754_y1(x)
// Bessel function of the first and second kinds of order one.
// Method -- j1(x):
//      1. For tiny x, we use j1(x) = x/2 - x**3/16 + x**5/384 - ...
//      2. Reduce x to |x| since j1(x)=-j1(-x),  and
//         for x in (0,2)
//              j1(x) = x/2 + x*z*R0/S0,  where z = x*x;
//         (precision:  |j1/x - 1/2 - R0/S0 |<2**-61.51 )
//         for x in (2,inf)
//              j1(x) = sqrt(2/(pi*x))*(p1(x)*cos(x1)-q1(x)*sin(x1))
//              y1(x) = sqrt(2/(pi*x))*(p1(x)*sin(x1)+q1(x)*cos(x1))
//         where x1 = x-3*pi/4. It is better to compute sin(x1),cos(x1)
//         as follow:
//              cos(x1) =  cos(x)cos(3pi/4)+sin(x)sin(3pi/4)
//                      =  1/sqrt(2) * (sin(x) - cos(x))
//              sin(x1) =  sin(x)cos(3pi/4)-cos(x)sin(3pi/4)
//                      = -1/sqrt(2) * (sin(x) + cos(x))
//         (To avoid cancellation, use
//              sin(x) +- cos(x) = -cos(2x)/(sin(x) -+ cos(x))
//         to compute the worse one.)
//
//      3 Special cases
//              j1(nan)= nan
//              j1(0) = 0
//              j1(inf) = 0
//
// Method -- y1(x):
//      1. screen out x<=0 cases: y1(0)=-inf, y1(x<0)=NaN
//      2. For x<2.
//         Since
//              y1(x) = 2/pi*(j1(x)*(ln(x/2)+Euler)-1/x-x/2+5/64*x**3-...)
//         therefore y1(x)-2/pi*j1(x)*ln(x)-1/x is an odd function.
//         We use the following function to approximate y1,
//              y1(x) = x*U(z)/V(z) + (2/pi)*(j1(x)*ln(x)-1/x), z= x**2
//         where for x in [0,2] (abs err less than 2**-65.89)
//              U(z) = U0[0] + U0[1]*z + ... + U0[4]*z**4
//              V(z) = 1  + v0[0]*z + ... + v0[4]*z**5
//         Note: For tiny x, 1/x dominate y1 and hence
//              y1(tiny) = -2/pi/tiny, (choose tiny<2**-54)
//      3. For x>=2.
//               y1(x) = sqrt(2/(pi*x))*(p1(x)*sin(x1)+q1(x)*cos(x1))
//         where x1 = x-3*pi/4. It is better to compute sin(x1),cos(x1)
//         by method mentioned above.

// J1 returns the order-one Bessel function of the first kind.
//
// Special cases are:
//
//	J1(±Inf) = 0
//	J1(NaN) = NaN
func J1(x float64) float64 {
	const (
		TwoM27 = 1.0 / (1 << 27) // 2**-27 0x3e40000000000000
		Two129 = 1 << 129        // 2**129 0x4800000000000000
		// R0/S0 on [0, 2]
		R00 = -6.25000000000000000000e-02 // 0xBFB0000000000000
		R01 = 1.40705666955189706048e-03  // 0x3F570D9F98472C61
		R02 = -1.59955631084035597520e-05 // 0xBEF0C5C6BA169668
		R03 = 4.96727999609584448412e-08  // 0x3E6AAAFA46CA0BD9
		S01 = 1.91537599538363460805e-02  // 0x3F939D0B12637E53
		S02 = 1.85946785588630915560e-04  // 0x3F285F56B9CDF664
		S03 = 1.17718464042623683263e-06  // 0x3EB3BFF8333F8498
		S04 = 5.04636257076217042715e-09  // 0x3E35AC88C97DFF2C
		S05 = 1.23542274426137913908e-11  // 0x3DAB2ACFCFB97ED8
	)
	// special cases
	switch {
	case IsNaN(x):
		return x
	case IsInf(x, 0) || x == 0:
		return 0
	}

	sign := false
	if x < 0 {
		x = -x
		sign = true
	}
	if x >= 2 {
		s, c := Sincos(x)
		ss := -s - c
		cc := s - c

		// make sure x+x does not overflow
		if x < MaxFloat64/2 {
			z := Cos(x + x)
			if s*c > 0 {
				cc = z / ss
			} else {
				ss = z / cc
			}
		}

		// j1(x) = 1/sqrt(pi) * (P(1,x)*cc - Q(1,x)*ss) / sqrt(x)
		// y1(x) = 1/sqrt(pi) * (P(1,x)*ss + Q(1,x)*cc) / sqrt(x)

		var z float64
		if x > Two129 {
			z = (1 / SqrtPi) * cc / Sqrt(x)
		} else {
			u := pone(x)
			v := qone(x)
			z = (1 / SqrtPi) * (u*cc - v*ss) / Sqrt(x)
		}
		if sign {
			return -z
		}
		return z
	}
	if x < TwoM27 { // |x|<2**-27
		return 0.5 * x // inexact if x!=0 necessary
	}
	z := x * x
	r := z * (R00 + z*(R01+z*(R02+z*R03)))
	s := 1.0 + z*(S01+z*(S02+z*(S03+z*(S04+z*S05))))
	r *= x
	z = 0.5*x + r/s
	if sign {
		return -z
	}
	return z
}

// Y1 returns the order-one Bessel function of the second kind.
//
// Special cases are:
//
//	Y1(+Inf) = 0
//	Y1(0) = -Inf
//	Y1(x < 0) = NaN
//	Y1(NaN) = NaN
func Y1(x float64) float64 {
	const (
		TwoM54 = 1.0 / (1 << 54)             // 2**-54 0x3c90000000000000
		Two129 = 1 << 129                    // 2**129 0x4800000000000000
		U00    = -1.96057090646238940668e-01 // 0xBFC91866143CBC8A
		U01    = 5.04438716639811282616e-02  // 0x3FA9D3C776292CD1
		U02    = -1.91256895875763547298e-03 // 0xBF5F55E54844F50F
		U03    = 2.35252600561610495928e-05  // 0x3EF8AB038FA6B88E
		U04    = -9.19099158039878874504e-08 // 0xBE78AC00569105B8
		V00    = 1.99167318236649903973e-02  // 0x3F94650D3F4DA9F0
		V01    = 2.02552581025135171496e-04  // 0x3F2A8C896C257764
		V02    = 1.35608801097516229404e-06  // 0x3EB6C05A894E8CA6
		V03    = 6.22741452364621501295e-09  // 0x3E3ABF1D5BA69A86
		V04    = 1.66559246207992079114e-11  // 0x3DB25039DACA772A
	)
	// special cases
	switch {
	case x < 0 || IsNaN(x):
		return NaN()
	case IsInf(x, 1):
		return 0
	case x == 0:
		return Inf(-1)
	}

	if x >= 2 {
		s, c := Sincos(x)
		ss := -s - c
		cc := s - c

		// make sure x+x does not overflow
		if x < MaxFloat64/2 {
			z := Cos(x + x)
			if s*c > 0 {
				cc = z / ss
			} else {
				ss = z / cc
			}
		}
		// y1(x) = sqrt(2/(pi*x))*(p1(x)*sin(x0)+q1(x)*cos(x0))
		// where x0 = x-3pi/4
		//     Better formula:
		//         cos(x0) = cos(x)cos(3pi/4)+sin(x)sin(3pi/4)
		//                 =  1/sqrt(2) * (sin(x) - cos(x))
		//         sin(x0) = sin(x)cos(3pi/4)-cos(x)sin(3pi/4)
		//                 = -1/sqrt(2) * (cos(x) + sin(x))
		// To avoid cancellation, use
		//     sin(x) +- cos(x) = -cos(2x)/(sin(x) -+ cos(x))
		// to compute the worse one.

		var z float64
		if x > Two129 {
			z = (1 / SqrtPi) * ss / Sqrt(x)
		} else {
			u := pone(x)
			v := qone(x)
			z = (1 / SqrtPi) * (u*ss + v*cc) / Sqrt(x)
		}
		return z
	}
	if x <= TwoM54 { // x < 2**-54
		return -(2 / Pi) / x
	}
	z := x * x
	u := U00 + z*(U01+z*(U02+z*(U03+z*U04)))
	v := 1 + z*(V00+z*(V01+z*(V02+z*(V03+z*V04))))
	return x*(u/v) + (2/Pi)*(J1(x)*Log(x)-1/x)
}

// For x >= 8, the asymptotic expansions of pone is
//      1 + 15/128 s**2 - 4725/2**15 s**4 - ..., where s = 1/x.
// We approximate pone by
//      pone(x) = 1 + (R/S)
// where R = pr0 + pr1*s**2 + pr2*s**4 + ... + pr5*s**10
//       S = 1 + ps0*s**2 + ... + ps4*s**10
// and
//      | pone(x)-1-R/S | <= 2**(-60.06)

// for x in [inf, 8]=1/[0,0.125]
var p1R8 = [6]float64{
	0.00000000000000000000e+00, // 0x0000000000000000
	1.17187499999988647970e-01, // 0x3FBDFFFFFFFFFCCE
	1.32394806593073575129e+01, // 0x402A7A9D357F7FCE
	4.12051854307378562225e+02, // 0x4079C0D4652EA590
	3.87474538913960532227e+03, // 0x40AE457DA3A532CC
	7.91447954031891731574e+03, // 0x40BEEA7AC32782DD
}
var p1S8 = [5]float64{
	1.14207370375678408436e+02, // 0x405C8D458E656CAC
	3.65093083420853463394e+03, // 0x40AC85DC964D274F
	3.69562060269033463555e+04, // 0x40E20B8697C5BB7F
	9.76027935934950801311e+04, // 0x40F7D42CB28F17BB
	3.08042720627888811578e+04, // 0x40DE1511697A0B2D
}

// for x in [8,4.5454] = 1/[0.125,0.22001]
var p1R5 = [6]float64{
	1.31990519556243522749e-11, // 0x3DAD0667DAE1CA7D
	1.17187493190614097638e-01, // 0x3FBDFFFFE2C10043
	6.80275127868432871736e+00, // 0x401B36046E6315E3
	1.08308182990189109773e+02, // 0x405B13B9452602ED
	5.17636139533199752805e+02, // 0x40802D16D052D649
	5.28715201363337541807e+02, // 0x408085B8BB7E0CB7
}
var p1S5 = [5]float64{
	5.92805987221131331921e+01, // 0x404DA3EAA8AF633D
	9.91401418733614377743e+02, // 0x408EFB361B066701
	5.35326695291487976647e+03, // 0x40B4E9445706B6FB
	7.84469031749551231769e+03, // 0x40BEA4B0B8A5BB15
	1.50404688810361062679e+03, // 0x40978030036F5E51
}

// for x in[4.5453,2.8571] = 1/[0.2199,0.35001]
var p1R3 = [6]float64{
	3.02503916137373618024e-09, // 0x3E29FC21A7AD9EDD
	1.17186865567253592491e-01, // 0x3FBDFFF55B21D17B
	3.93297750033315640650e+00, // 0x400F76BCE85EAD8A
	3.51194035591636932736e+01, // 0x40418F489DA6D129
	9.10550110750781271918e+01, // 0x4056C3854D2C1837
	4.85590685197364919645e+01, // 0x4048478F8EA83EE5
}
var p1S3 = [5]float64{
	3.47913095001251519989e+01, // 0x40416549A134069C
	3.36762458747825746741e+02, // 0x40750C3307F1A75F
	1.04687139975775130551e+03, // 0x40905B7C5037D523
	8.90811346398256432622e+02, // 0x408BD67DA32E31E9
	1.03787932439639277504e+02, // 0x4059F26D7C2EED53
}

// for x in [2.8570,2] = 1/[0.3499,0.5]
var p1R2 = [6]float64{
	1.07710830106873743082e-07, // 0x3E7CE9D4F65544F4
	1.17176219462683348094e-01, // 0x3FBDFF42BE760D83
	2.36851496667608785174e+00, // 0x4002F2B7F98FAEC0
	1.22426109148261232917e+01, // 0x40287C377F71A964
	1.76939711271687727390e+01, // 0x4031B1A8177F8EE2
	5.07352312588818499250e+00, // 0x40144B49A574C1FE
}
var p1S2 = [5]float64{
	2.14364859363821409488e+01, // 0x40356FBD8AD5ECDC
	1.25290227168402751090e+02, // 0x405F529314F92CD5
	2.32276469057162813669e+02, // 0x406D08D8D5A2DBD9
	1.17679373287147100768e+02, // 0x405D6B7ADA1884A9
	8.36463893371618283368e+00, // 0x4020BAB1F44E5192
}

func pone(x float64) float64 {
	var p *[6]float64
	var q *[5]float64
	if x >= 8 {
		p = &p1R8
		q = &p1S8
	} else if x >= 4.5454 {
		p = &p1R5
		q = &p1S5
	} else if x >= 2.8571 {
		p = &p1R3
		q = &p1S3
	} else if x >= 2 {
		p = &p1R2
		q = &p1S2
	}
	z := 1 / (x * x)
	r := p[0] + z*(p[1]+z*(p[2]+z*(p[3]+z*(p[4]+z*p[5]))))
	s := 1.0 + z*(q[0]+z*(q[1]+z*(q[2]+z*(q[3]+z*q[4]))))
	return 1 + r/s
}

// For x >= 8, the asymptotic expansions of qone is
//      3/8 s - 105/1024 s**3 - ..., where s = 1/x.
// We approximate qone by
//      qone(x) = s*(0.375 + (R/S))
// where R = qr1*s**2 + qr2*s**4 + ... + qr5*s**10
//       S = 1 + qs1*s**2 + ... + qs6*s**12
// and
//      | qone(x)/s -0.375-R/S | <= 2**(-61.13)

// for x in [inf, 8] = 1/[0,0.125]
var q1R8 = [6]float64{
	0.00000000000000000000e+00,  // 0x0000000000000000
	-1.02539062499992714161e-01, // 0xBFBA3FFFFFFFFDF3
	-1.62717534544589987888e+01, // 0xC0304591A26779F7
	-7.59601722513950107896e+02, // 0xC087BCD053E4B576
	-1.18498066702429587167e+04, // 0xC0C724E740F87415
	-4.84385124285750353010e+04, // 0xC0E7A6D065D09C6A
}
var q1S8 = [6]float64{
	1.61395369700722909556e+02,  // 0x40642CA6DE5BCDE5
	7.82538599923348465381e+03,  // 0x40BE9162D0D88419
	1.33875336287249578163e+05,  // 0x4100579AB0B75E98
	7.19657723683240939863e+05,  // 0x4125F65372869C19
	6.66601232617776375264e+05,  // 0x412457D27719AD5C
	-2.94490264303834643215e+05, // 0xC111F9690EA5AA18
}

// for x in [8,4.5454] = 1/[0.125,0.22001]
var q1R5 = [6]float64{
	-2.08979931141764104297e-11, // 0xBDB6FA431AA1A098
	-1.02539050241375426231e-01, // 0xBFBA3FFFCB597FEF
	-8.05644828123936029840e+00, // 0xC0201CE6CA03AD4B
	-1.83669607474888380239e+02, // 0xC066F56D6CA7B9B0
	-1.37319376065508163265e+03, // 0xC09574C66931734F
	-2.61244440453215656817e+03, // 0xC0A468E388FDA79D
}
var q1S5 = [6]float64{
	8.12765501384335777857e+01,  // 0x405451B2FF5A11B2
	1.99179873460485964642e+03,  // 0x409F1F31E77BF839
	1.74684851924908907677e+04,  // 0x40D10F1F0D64CE29
	4.98514270910352279316e+04,  // 0x40E8576DAABAD197
	2.79480751638918118260e+04,  // 0x40DB4B04CF7C364B
	-4.71918354795128470869e+03, // 0xC0B26F2EFCFFA004
}

// for x in [4.5454,2.8571] = 1/[0.2199,0.35001] ???
var q1R3 = [6]float64{
	-5.07831226461766561369e-09, // 0xBE35CFA9D38FC84F
	-1.02537829820837089745e-01, // 0xBFBA3FEB51AEED54
	-4.61011581139473403113e+00, // 0xC01270C23302D9FF
	-5.78472216562783643212e+01, // 0xC04CEC71C25D16DA
	-2.28244540737631695038e+02, // 0xC06C87D34718D55F
	-2.19210128478909325622e+02, // 0xC06B66B95F5C1BF6
}
var q1S3 = [6]float64{
	4.76651550323729509273e+01,  // 0x4047D523CCD367E4
	6.73865112676699709482e+02,  // 0x40850EEBC031EE3E
	3.38015286679526343505e+03,  // 0x40AA684E448E7C9A
	5.54772909720722782367e+03,  // 0x40B5ABBAA61D54A6
	1.90311919338810798763e+03,  // 0x409DBC7A0DD4DF4B
	-1.35201191444307340817e+02, // 0xC060E670290A311F
}

// for x in [2.8570,2] = 1/[0.3499,0.5]
var q1R2 = [6]float64{
	-1.78381727510958865572e-07, // 0xBE87F12644C626D2
	-1.02517042607985553460e-01, // 0xBFBA3E8E9148B010
	-2.75220568278187460720e+00, // 0xC006048469BB4EDA
	-1.96636162643703720221e+01, // 0xC033A9E2C168907F
	-4.23253133372830490089e+01, // 0xC04529A3DE104AAA
	-2.13719211703704061733e+01, // 0xC0355F3639CF6E52
}
var q1S2 = [6]float64{
	2.95333629060523854548e+01,  // 0x403D888A78AE64FF
	2.52981549982190529136e+02,  // 0x406F9F68DB821CBA
	7.57502834868645436472e+02,  // 0x4087AC05CE49A0F7
	7.39393205320467245656e+02,  // 0x40871B2548D4C029
	1.55949003336666123687e+02,  // 0x40637E5E3C3ED8D4
	-4.95949898822628210127e+00, // 0xC013D686E71BE86B
}

func qone(x float64) float64 {
	var p, q *[6]float64
	if x >= 8 {
		p = &q1R8
		q = &q1S8
	} else if x >= 4.5454 {
		p = &q1R5
		q = &q1S5
	} else if x >= 2.8571 {
		p = &q1R3
		q = &q1S3
	} else if x >= 2 {
		p = &q1R2
		q = &q1S2
	}
	z := 1 / (x * x)
	r := p[0] + z*(p[1]+z*(p[2]+z*(p[3]+z*(p[4]+z*p[5]))))
	s := 1 + z*(q[0]+z*(q[1]+z*(q[2]+z*(q[3]+z*(q[4]+z*q[5])))))
	return (0.375 + r/s) / x
}

```

// === FILE: references!/go/src/math/jn.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

/*
	Bessel function of the first and second kinds of order n.
*/

// The original C code and the long comment below are
// from FreeBSD's /usr/src/lib/msun/src/e_jn.c and
// came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
// __ieee754_jn(n, x), __ieee754_yn(n, x)
// floating point Bessel's function of the 1st and 2nd kind
// of order n
//
// Special cases:
//      y0(0)=y1(0)=yn(n,0) = -inf with division by zero signal;
//      y0(-ve)=y1(-ve)=yn(n,-ve) are NaN with invalid signal.
// Note 2. About jn(n,x), yn(n,x)
//      For n=0, j0(x) is called,
//      for n=1, j1(x) is called,
//      for n<x, forward recursion is used starting
//      from values of j0(x) and j1(x).
//      for n>x, a continued fraction approximation to
//      j(n,x)/j(n-1,x) is evaluated and then backward
//      recursion is used starting from a supposed value
//      for j(n,x). The resulting value of j(0,x) is
//      compared with the actual value to correct the
//      supposed value of j(n,x).
//
//      yn(n,x) is similar in all respects, except
//      that forward recursion is used for all
//      values of n>1.

// Jn returns the order-n Bessel function of the first kind.
//
// Special cases are:
//
//	Jn(n, ±Inf) = 0
//	Jn(n, NaN) = NaN
func Jn(n int, x float64) float64 {
	const (
		TwoM29 = 1.0 / (1 << 29) // 2**-29 0x3e10000000000000
		Two302 = 1 << 302        // 2**302 0x52D0000000000000
	)
	// special cases
	switch {
	case IsNaN(x):
		return x
	case IsInf(x, 0):
		return 0
	}
	// J(-n, x) = (-1)**n * J(n, x), J(n, -x) = (-1)**n * J(n, x)
	// Thus, J(-n, x) = J(n, -x)

	if n == 0 {
		return J0(x)
	}
	if x == 0 {
		return 0
	}
	if n < 0 {
		n, x = -n, -x
	}
	if n == 1 {
		return J1(x)
	}
	sign := false
	if x < 0 {
		x = -x
		if n&1 == 1 {
			sign = true // odd n and negative x
		}
	}
	var b float64
	if float64(n) <= x {
		// Safe to use J(n+1,x)=2n/x *J(n,x)-J(n-1,x)
		if x >= Two302 { // x > 2**302

			// (x >> n**2)
			//          Jn(x) = cos(x-(2n+1)*pi/4)*sqrt(2/x*pi)
			//          Yn(x) = sin(x-(2n+1)*pi/4)*sqrt(2/x*pi)
			//          Let s=sin(x), c=cos(x),
			//              xn=x-(2n+1)*pi/4, sqt2 = sqrt(2),then
			//
			//                 n    sin(xn)*sqt2    cos(xn)*sqt2
			//              ----------------------------------
			//                 0     s-c             c+s
			//                 1    -s-c            -c+s
			//                 2    -s+c            -c-s
			//                 3     s+c             c-s

			var temp float64
			switch s, c := Sincos(x); n & 3 {
			case 0:
				temp = c + s
			case 1:
				temp = -c + s
			case 2:
				temp = -c - s
			case 3:
				temp = c - s
			}
			b = (1 / SqrtPi) * temp / Sqrt(x)
		} else {
			b = J1(x)
			for i, a := 1, J0(x); i < n; i++ {
				a, b = b, b*(float64(i+i)/x)-a // avoid underflow
			}
		}
	} else {
		if x < TwoM29 { // x < 2**-29
			// x is tiny, return the first Taylor expansion of J(n,x)
			// J(n,x) = 1/n!*(x/2)**n  - ...

			if n > 33 { // underflow
				b = 0
			} else {
				temp := x * 0.5
				b = temp
				a := 1.0
				for i := 2; i <= n; i++ {
					a *= float64(i) // a = n!
					b *= temp       // b = (x/2)**n
				}
				b /= a
			}
		} else {
			// use backward recurrence
			//                      x      x**2      x**2
			//  J(n,x)/J(n-1,x) =  ----   ------   ------   .....
			//                      2n  - 2(n+1) - 2(n+2)
			//
			//                      1      1        1
			//  (for large x)   =  ----  ------   ------   .....
			//                      2n   2(n+1)   2(n+2)
			//                      -- - ------ - ------ -
			//                       x     x         x
			//
			// Let w = 2n/x and h=2/x, then the above quotient
			// is equal to the continued fraction:
			//                  1
			//      = -----------------------
			//                     1
			//         w - -----------------
			//                        1
			//              w+h - ---------
			//                     w+2h - ...
			//
			// To determine how many terms needed, let
			// Q(0) = w, Q(1) = w(w+h) - 1,
			// Q(k) = (w+k*h)*Q(k-1) - Q(k-2),
			// When Q(k) > 1e4	good for single
			// When Q(k) > 1e9	good for double
			// When Q(k) > 1e17	good for quadruple

			// determine k
			w := float64(n+n) / x
			h := 2 / x
			q0 := w
			z := w + h
			q1 := w*z - 1
			k := 1
			for q1 < 1e9 {
				k++
				z += h
				q0, q1 = q1, z*q1-q0
			}
			m := n + n
			t := 0.0
			for i := 2 * (n + k); i >= m; i -= 2 {
				t = 1 / (float64(i)/x - t)
			}
			a := t
			b = 1
			//  estimate log((2/x)**n*n!) = n*log(2/x)+n*ln(n)
			//  Hence, if n*(log(2n/x)) > ...
			//  single 8.8722839355e+01
			//  double 7.09782712893383973096e+02
			//  long double 1.1356523406294143949491931077970765006170e+04
			//  then recurrent value may overflow and the result is
			//  likely underflow to zero

			tmp := float64(n)
			v := 2 / x
			tmp = tmp * Log(Abs(v*tmp))
			if tmp < 7.09782712893383973096e+02 {
				for i := n - 1; i > 0; i-- {
					di := float64(i + i)
					a, b = b, b*di/x-a
				}
			} else {
				for i := n - 1; i > 0; i-- {
					di := float64(i + i)
					a, b = b, b*di/x-a
					// scale b to avoid spurious overflow
					if b > 1e100 {
						a /= b
						t /= b
						b = 1
					}
				}
			}
			b = t * J0(x) / b
		}
	}
	if sign {
		return -b
	}
	return b
}

// Yn returns the order-n Bessel function of the second kind.
//
// Special cases are:
//
//	Yn(n, +Inf) = 0
//	Yn(n ≥ 0, 0) = -Inf
//	Yn(n < 0, 0) = +Inf if n is odd, -Inf if n is even
//	Yn(n, x < 0) = NaN
//	Yn(n, NaN) = NaN
func Yn(n int, x float64) float64 {
	const Two302 = 1 << 302 // 2**302 0x52D0000000000000
	// special cases
	switch {
	case x < 0 || IsNaN(x):
		return NaN()
	case IsInf(x, 1):
		return 0
	}

	if n == 0 {
		return Y0(x)
	}
	if x == 0 {
		if n < 0 && n&1 == 1 {
			return Inf(1)
		}
		return Inf(-1)
	}
	sign := false
	if n < 0 {
		n = -n
		if n&1 == 1 {
			sign = true // sign true if n < 0 && |n| odd
		}
	}
	if n == 1 {
		if sign {
			return -Y1(x)
		}
		return Y1(x)
	}
	var b float64
	if x >= Two302 { // x > 2**302
		// (x >> n**2)
		//	    Jn(x) = cos(x-(2n+1)*pi/4)*sqrt(2/x*pi)
		//	    Yn(x) = sin(x-(2n+1)*pi/4)*sqrt(2/x*pi)
		//	    Let s=sin(x), c=cos(x),
		//		xn=x-(2n+1)*pi/4, sqt2 = sqrt(2),then
		//
		//		   n	sin(xn)*sqt2	cos(xn)*sqt2
		//		----------------------------------
		//		   0	 s-c		 c+s
		//		   1	-s-c 		-c+s
		//		   2	-s+c		-c-s
		//		   3	 s+c		 c-s

		var temp float64
		switch s, c := Sincos(x); n & 3 {
		case 0:
			temp = s - c
		case 1:
			temp = -s - c
		case 2:
			temp = -s + c
		case 3:
			temp = s + c
		}
		b = (1 / SqrtPi) * temp / Sqrt(x)
	} else {
		a := Y0(x)
		b = Y1(x)
		// quit if b is -inf
		for i := 1; i < n && !IsInf(b, -1); i++ {
			a, b = b, (float64(i+i)/x)*b-a
		}
	}
	if sign {
		return -b
	}
	return b
}

```

// === FILE: references!/go/src/math/ldexp.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// Ldexp is the inverse of [Frexp].
// It returns frac × 2**exp.
//
// Special cases are:
//
//	Ldexp(±0, exp) = ±0
//	Ldexp(±Inf, exp) = ±Inf
//	Ldexp(NaN, exp) = NaN
func Ldexp(frac float64, exp int) float64 {
	if haveArchLdexp {
		return archLdexp(frac, exp)
	}
	return ldexp(frac, exp)
}

func ldexp(frac float64, exp int) float64 {
	// special cases
	switch {
	case frac == 0:
		return frac // correctly return -0
	case IsInf(frac, 0) || IsNaN(frac):
		return frac
	}
	frac, e := normalize(frac)
	exp += e
	x := Float64bits(frac)
	exp += int(x>>shift)&mask - bias
	if exp < -1075 {
		return Copysign(0, frac) // underflow
	}
	if exp > 1023 { // overflow
		if frac < 0 {
			return Inf(-1)
		}
		return Inf(1)
	}
	var m float64 = 1
	if exp < -1022 { // denormal
		exp += 53
		m = 1.0 / (1 << 53) // 2**-53
	}
	x &^= mask << shift
	x |= uint64(exp+bias) << shift
	return m * Float64frombits(x)
}

```

// === FILE: references!/go/src/math/lgamma.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

/*
	Floating-point logarithm of the Gamma function.
*/

// The original C code and the long comment below are
// from FreeBSD's /usr/src/lib/msun/src/e_lgamma_r.c and
// came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
// __ieee754_lgamma_r(x, signgamp)
// Reentrant version of the logarithm of the Gamma function
// with user provided pointer for the sign of Gamma(x).
//
// Method:
//   1. Argument Reduction for 0 < x <= 8
//      Since gamma(1+s)=s*gamma(s), for x in [0,8], we may
//      reduce x to a number in [1.5,2.5] by
//              lgamma(1+s) = log(s) + lgamma(s)
//      for example,
//              lgamma(7.3) = log(6.3) + lgamma(6.3)
//                          = log(6.3*5.3) + lgamma(5.3)
//                          = log(6.3*5.3*4.3*3.3*2.3) + lgamma(2.3)
//   2. Polynomial approximation of lgamma around its
//      minimum (ymin=1.461632144968362245) to maintain monotonicity.
//      On [ymin-0.23, ymin+0.27] (i.e., [1.23164,1.73163]), use
//              Let z = x-ymin;
//              lgamma(x) = -1.214862905358496078218 + z**2*poly(z)
//              poly(z) is a 14 degree polynomial.
//   2. Rational approximation in the primary interval [2,3]
//      We use the following approximation:
//              s = x-2.0;
//              lgamma(x) = 0.5*s + s*P(s)/Q(s)
//      with accuracy
//              |P/Q - (lgamma(x)-0.5s)| < 2**-61.71
//      Our algorithms are based on the following observation
//
//                             zeta(2)-1    2    zeta(3)-1    3
// lgamma(2+s) = s*(1-Euler) + --------- * s  -  --------- * s  + ...
//                                 2                 3
//
//      where Euler = 0.5772156649... is the Euler constant, which
//      is very close to 0.5.
//
//   3. For x>=8, we have
//      lgamma(x)~(x-0.5)log(x)-x+0.5*log(2pi)+1/(12x)-1/(360x**3)+....
//      (better formula:
//         lgamma(x)~(x-0.5)*(log(x)-1)-.5*(log(2pi)-1) + ...)
//      Let z = 1/x, then we approximation
//              f(z) = lgamma(x) - (x-0.5)(log(x)-1)
//      by
//                                  3       5             11
//              w = w0 + w1*z + w2*z  + w3*z  + ... + w6*z
//      where
//              |w - f(z)| < 2**-58.74
//
//   4. For negative x, since (G is gamma function)
//              -x*G(-x)*G(x) = pi/sin(pi*x),
//      we have
//              G(x) = pi/(sin(pi*x)*(-x)*G(-x))
//      since G(-x) is positive, sign(G(x)) = sign(sin(pi*x)) for x<0
//      Hence, for x<0, signgam = sign(sin(pi*x)) and
//              lgamma(x) = log(|Gamma(x)|)
//                        = log(pi/(|x*sin(pi*x)|)) - lgamma(-x);
//      Note: one should avoid computing pi*(-x) directly in the
//            computation of sin(pi*(-x)).
//
//   5. Special Cases
//              lgamma(2+s) ~ s*(1-Euler) for tiny s
//              lgamma(1)=lgamma(2)=0
//              lgamma(x) ~ -log(x) for tiny x
//              lgamma(0) = lgamma(inf) = inf
//              lgamma(-integer) = +-inf
//
//

var _lgamA = [...]float64{
	7.72156649015328655494e-02, // 0x3FB3C467E37DB0C8
	3.22467033424113591611e-01, // 0x3FD4A34CC4A60FAD
	6.73523010531292681824e-02, // 0x3FB13E001A5562A7
	2.05808084325167332806e-02, // 0x3F951322AC92547B
	7.38555086081402883957e-03, // 0x3F7E404FB68FEFE8
	2.89051383673415629091e-03, // 0x3F67ADD8CCB7926B
	1.19270763183362067845e-03, // 0x3F538A94116F3F5D
	5.10069792153511336608e-04, // 0x3F40B6C689B99C00
	2.20862790713908385557e-04, // 0x3F2CF2ECED10E54D
	1.08011567247583939954e-04, // 0x3F1C5088987DFB07
	2.52144565451257326939e-05, // 0x3EFA7074428CFA52
	4.48640949618915160150e-05, // 0x3F07858E90A45837
}
var _lgamR = [...]float64{
	1.0,                        // placeholder
	1.39200533467621045958e+00, // 0x3FF645A762C4AB74
	7.21935547567138069525e-01, // 0x3FE71A1893D3DCDC
	1.71933865632803078993e-01, // 0x3FC601EDCCFBDF27
	1.86459191715652901344e-02, // 0x3F9317EA742ED475
	7.77942496381893596434e-04, // 0x3F497DDACA41A95B
	7.32668430744625636189e-06, // 0x3EDEBAF7A5B38140
}
var _lgamS = [...]float64{
	-7.72156649015328655494e-02, // 0xBFB3C467E37DB0C8
	2.14982415960608852501e-01,  // 0x3FCB848B36E20878
	3.25778796408930981787e-01,  // 0x3FD4D98F4F139F59
	1.46350472652464452805e-01,  // 0x3FC2BB9CBEE5F2F7
	2.66422703033638609560e-02,  // 0x3F9B481C7E939961
	1.84028451407337715652e-03,  // 0x3F5E26B67368F239
	3.19475326584100867617e-05,  // 0x3F00BFECDD17E945
}
var _lgamT = [...]float64{
	4.83836122723810047042e-01,  // 0x3FDEF72BC8EE38A2
	-1.47587722994593911752e-01, // 0xBFC2E4278DC6C509
	6.46249402391333854778e-02,  // 0x3FB08B4294D5419B
	-3.27885410759859649565e-02, // 0xBFA0C9A8DF35B713
	1.79706750811820387126e-02,  // 0x3F9266E7970AF9EC
	-1.03142241298341437450e-02, // 0xBF851F9FBA91EC6A
	6.10053870246291332635e-03,  // 0x3F78FCE0E370E344
	-3.68452016781138256760e-03, // 0xBF6E2EFFB3E914D7
	2.25964780900612472250e-03,  // 0x3F6282D32E15C915
	-1.40346469989232843813e-03, // 0xBF56FE8EBF2D1AF1
	8.81081882437654011382e-04,  // 0x3F4CDF0CEF61A8E9
	-5.38595305356740546715e-04, // 0xBF41A6109C73E0EC
	3.15632070903625950361e-04,  // 0x3F34AF6D6C0EBBF7
	-3.12754168375120860518e-04, // 0xBF347F24ECC38C38
	3.35529192635519073543e-04,  // 0x3F35FD3EE8C2D3F4
}
var _lgamU = [...]float64{
	-7.72156649015328655494e-02, // 0xBFB3C467E37DB0C8
	6.32827064025093366517e-01,  // 0x3FE4401E8B005DFF
	1.45492250137234768737e+00,  // 0x3FF7475CD119BD6F
	9.77717527963372745603e-01,  // 0x3FEF497644EA8450
	2.28963728064692451092e-01,  // 0x3FCD4EAEF6010924
	1.33810918536787660377e-02,  // 0x3F8B678BBF2BAB09
}
var _lgamV = [...]float64{
	1.0,
	2.45597793713041134822e+00, // 0x4003A5D7C2BD619C
	2.12848976379893395361e+00, // 0x40010725A42B18F5
	7.69285150456672783825e-01, // 0x3FE89DFBE45050AF
	1.04222645593369134254e-01, // 0x3FBAAE55D6537C88
	3.21709242282423911810e-03, // 0x3F6A5ABB57D0CF61
}
var _lgamW = [...]float64{
	4.18938533204672725052e-01,  // 0x3FDACFE390C97D69
	8.33333333333329678849e-02,  // 0x3FB555555555553B
	-2.77777777728775536470e-03, // 0xBF66C16C16B02E5C
	7.93650558643019558500e-04,  // 0x3F4A019F98CF38B6
	-5.95187557450339963135e-04, // 0xBF4380CB8C0FE741
	8.36339918996282139126e-04,  // 0x3F4B67BA4CDAD5D1
	-1.63092934096575273989e-03, // 0xBF5AB89D0B9E43E4
}

// Lgamma returns the natural logarithm and sign (-1 or +1) of [Gamma](x).
//
// Special cases are:
//
//	Lgamma(+Inf) = +Inf
//	Lgamma(0) = +Inf
//	Lgamma(-integer) = +Inf
//	Lgamma(-Inf) = -Inf
//	Lgamma(NaN) = NaN
func Lgamma(x float64) (lgamma float64, sign int) {
	const (
		Ymin  = 1.461632144968362245
		Two52 = 1 << 52                     // 0x4330000000000000 ~4.5036e+15
		Two53 = 1 << 53                     // 0x4340000000000000 ~9.0072e+15
		Two58 = 1 << 58                     // 0x4390000000000000 ~2.8823e+17
		Tiny  = 1.0 / (1 << 70)             // 0x3b90000000000000 ~8.47033e-22
		Tc    = 1.46163214496836224576e+00  // 0x3FF762D86356BE3F
		Tf    = -1.21486290535849611461e-01 // 0xBFBF19B9BCC38A42
		// Tt = -(tail of Tf)
		Tt = -3.63867699703950536541e-18 // 0xBC50C7CAA48A971F
	)
	// special cases
	sign = 1
	switch {
	case IsNaN(x):
		lgamma = x
		return
	case IsInf(x, 0):
		lgamma = x
		return
	case x == 0:
		lgamma = Inf(1)
		return
	}

	neg := false
	if x < 0 {
		x = -x
		neg = true
	}

	if x < Tiny { // if |x| < 2**-70, return -log(|x|)
		if neg {
			sign = -1
		}
		lgamma = -Log(x)
		return
	}
	var nadj float64
	if neg {
		if x >= Two52 { // |x| >= 2**52, must be -integer
			lgamma = Inf(1)
			return
		}
		t := sinPi(x)
		if t == 0 {
			lgamma = Inf(1) // -integer
			return
		}
		nadj = Log(Pi / Abs(t*x))
		if t < 0 {
			sign = -1
		}
	}

	switch {
	case x == 1 || x == 2: // purge off 1 and 2
		lgamma = 0
		return
	case x < 2: // use lgamma(x) = lgamma(x+1) - log(x)
		var y float64
		var i int
		if x <= 0.9 {
			lgamma = -Log(x)
			switch {
			case x >= (Ymin - 1 + 0.27): // 0.7316 <= x <=  0.9
				y = 1 - x
				i = 0
			case x >= (Ymin - 1 - 0.27): // 0.2316 <= x < 0.7316
				y = x - (Tc - 1)
				i = 1
			default: // 0 < x < 0.2316
				y = x
				i = 2
			}
		} else {
			lgamma = 0
			switch {
			case x >= (Ymin + 0.27): // 1.7316 <= x < 2
				y = 2 - x
				i = 0
			case x >= (Ymin - 0.27): // 1.2316 <= x < 1.7316
				y = x - Tc
				i = 1
			default: // 0.9 < x < 1.2316
				y = x - 1
				i = 2
			}
		}
		switch i {
		case 0:
			z := y * y
			p1 := _lgamA[0] + z*(_lgamA[2]+z*(_lgamA[4]+z*(_lgamA[6]+z*(_lgamA[8]+z*_lgamA[10]))))
			p2 := z * (_lgamA[1] + z*(+_lgamA[3]+z*(_lgamA[5]+z*(_lgamA[7]+z*(_lgamA[9]+z*_lgamA[11])))))
			p := y*p1 + p2
			lgamma += (p - 0.5*y)
		case 1:
			z := y * y
			w := z * y
			p1 := _lgamT[0] + w*(_lgamT[3]+w*(_lgamT[6]+w*(_lgamT[9]+w*_lgamT[12]))) // parallel comp
			p2 := _lgamT[1] + w*(_lgamT[4]+w*(_lgamT[7]+w*(_lgamT[10]+w*_lgamT[13])))
			p3 := _lgamT[2] + w*(_lgamT[5]+w*(_lgamT[8]+w*(_lgamT[11]+w*_lgamT[14])))
			p := z*p1 - (Tt - w*(p2+y*p3))
			lgamma += (Tf + p)
		case 2:
			p1 := y * (_lgamU[0] + y*(_lgamU[1]+y*(_lgamU[2]+y*(_lgamU[3]+y*(_lgamU[4]+y*_lgamU[5])))))
			p2 := 1 + y*(_lgamV[1]+y*(_lgamV[2]+y*(_lgamV[3]+y*(_lgamV[4]+y*_lgamV[5]))))
			lgamma += (-0.5*y + p1/p2)
		}
	case x < 8: // 2 <= x < 8
		i := int(x)
		y := x - float64(i)
		p := y * (_lgamS[0] + y*(_lgamS[1]+y*(_lgamS[2]+y*(_lgamS[3]+y*(_lgamS[4]+y*(_lgamS[5]+y*_lgamS[6]))))))
		q := 1 + y*(_lgamR[1]+y*(_lgamR[2]+y*(_lgamR[3]+y*(_lgamR[4]+y*(_lgamR[5]+y*_lgamR[6])))))
		lgamma = 0.5*y + p/q
		z := 1.0 // Lgamma(1+s) = Log(s) + Lgamma(s)
		switch i {
		case 7:
			z *= (y + 6)
			fallthrough
		case 6:
			z *= (y + 5)
			fallthrough
		case 5:
			z *= (y + 4)
			fallthrough
		case 4:
			z *= (y + 3)
			fallthrough
		case 3:
			z *= (y + 2)
			lgamma += Log(z)
		}
	case x < Two58: // 8 <= x < 2**58
		t := Log(x)
		z := 1 / x
		y := z * z
		w := _lgamW[0] + z*(_lgamW[1]+y*(_lgamW[2]+y*(_lgamW[3]+y*(_lgamW[4]+y*(_lgamW[5]+y*_lgamW[6])))))
		lgamma = (x-0.5)*(t-1) + w
	default: // 2**58 <= x <= Inf
		lgamma = x * (Log(x) - 1)
	}
	if neg {
		lgamma = nadj - lgamma
	}
	return
}

// sinPi(x) is a helper function for negative x
func sinPi(x float64) float64 {
	const (
		Two52 = 1 << 52 // 0x4330000000000000 ~4.5036e+15
		Two53 = 1 << 53 // 0x4340000000000000 ~9.0072e+15
	)
	if x < 0.25 {
		return -Sin(Pi * x)
	}

	// argument reduction
	z := Floor(x)
	var n int
	if z != x { // inexact
		x = Mod(x, 2)
		n = int(x * 4)
	} else {
		if x >= Two53 { // x must be even
			x = 0
			n = 0
		} else {
			if x < Two52 {
				z = x + Two52 // exact
			}
			n = int(1 & Float64bits(z))
			x = float64(n)
			n <<= 2
		}
	}
	switch n {
	case 0:
		x = Sin(Pi * x)
	case 1, 2:
		x = Cos(Pi * (0.5 - x))
	case 3, 4:
		x = Sin(Pi * (1 - x))
	case 5, 6:
		x = -Cos(Pi * (x - 1.5))
	default:
		x = Sin(Pi * (x - 2))
	}
	return -x
}

```

// === FILE: references!/go/src/math/log.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

/*
	Floating-point logarithm.
*/

// The original C code, the long comment, and the constants
// below are from FreeBSD's /usr/src/lib/msun/src/e_log.c
// and came with this notice. The go code is a simpler
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
// __ieee754_log(x)
// Return the logarithm of x
//
// Method :
//   1. Argument Reduction: find k and f such that
//			x = 2**k * (1+f),
//	   where  sqrt(2)/2 < 1+f < sqrt(2) .
//
//   2. Approximation of log(1+f).
//	Let s = f/(2+f) ; based on log(1+f) = log(1+s) - log(1-s)
//		 = 2s + 2/3 s**3 + 2/5 s**5 + .....,
//	     	 = 2s + s*R
//      We use a special Reme algorithm on [0,0.1716] to generate
//	a polynomial of degree 14 to approximate R.  The maximum error
//	of this polynomial approximation is bounded by 2**-58.45. In
//	other words,
//		        2      4      6      8      10      12      14
//	    R(z) ~ L1*s +L2*s +L3*s +L4*s +L5*s  +L6*s  +L7*s
//	(the values of L1 to L7 are listed in the program) and
//	    |      2          14          |     -58.45
//	    | L1*s +...+L7*s    -  R(z) | <= 2
//	    |                             |
//	Note that 2s = f - s*f = f - hfsq + s*hfsq, where hfsq = f*f/2.
//	In order to guarantee error in log below 1ulp, we compute log by
//		log(1+f) = f - s*(f - R)		(if f is not too large)
//		log(1+f) = f - (hfsq - s*(hfsq+R)).	(better accuracy)
//
//	3. Finally,  log(x) = k*Ln2 + log(1+f).
//			    = k*Ln2_hi+(f-(hfsq-(s*(hfsq+R)+k*Ln2_lo)))
//	   Here Ln2 is split into two floating point number:
//			Ln2_hi + Ln2_lo,
//	   where n*Ln2_hi is always exact for |n| < 2000.
//
// Special cases:
//	log(x) is NaN with signal if x < 0 (including -INF) ;
//	log(+INF) is +INF; log(0) is -INF with signal;
//	log(NaN) is that NaN with no signal.
//
// Accuracy:
//	according to an error analysis, the error is always less than
//	1 ulp (unit in the last place).
//
// Constants:
// The hexadecimal values are the intended ones for the following
// constants. The decimal values may be used, provided that the
// compiler will convert from decimal to binary accurately enough
// to produce the hexadecimal values shown.

// Log returns the natural logarithm of x.
//
// Special cases are:
//
//	Log(+Inf) = +Inf
//	Log(0) = -Inf
//	Log(x < 0) = NaN
//	Log(NaN) = NaN
func Log(x float64) float64 {
	if haveArchLog {
		return archLog(x)
	}
	return log(x)
}

func log(x float64) float64 {
	const (
		Ln2Hi = 6.93147180369123816490e-01 /* 3fe62e42 fee00000 */
		Ln2Lo = 1.90821492927058770002e-10 /* 3dea39ef 35793c76 */
		L1    = 6.666666666666735130e-01   /* 3FE55555 55555593 */
		L2    = 3.999999999940941908e-01   /* 3FD99999 9997FA04 */
		L3    = 2.857142874366239149e-01   /* 3FD24924 94229359 */
		L4    = 2.222219843214978396e-01   /* 3FCC71C5 1D8E78AF */
		L5    = 1.818357216161805012e-01   /* 3FC74664 96CB03DE */
		L6    = 1.531383769920937332e-01   /* 3FC39A09 D078C69F */
		L7    = 1.479819860511658591e-01   /* 3FC2F112 DF3E5244 */
	)

	// special cases
	switch {
	case IsNaN(x) || IsInf(x, 1):
		return x
	case x < 0:
		return NaN()
	case x == 0:
		return Inf(-1)
	}

	// reduce
	f1, ki := Frexp(x)
	if f1 < Sqrt2/2 {
		f1 *= 2
		ki--
	}
	f := f1 - 1
	k := float64(ki)

	// compute
	s := f / (2 + f)
	s2 := s * s
	s4 := s2 * s2
	t1 := s2 * (L1 + s4*(L3+s4*(L5+s4*L7)))
	t2 := s4 * (L2 + s4*(L4+s4*L6))
	R := t1 + t2
	hfsq := 0.5 * f * f
	return k*Ln2Hi - ((hfsq - (s*(hfsq+R) + k*Ln2Lo)) - f)
}

```

// === FILE: references!/go/src/math/log10.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// Log10 returns the decimal logarithm of x.
// The special cases are the same as for [Log].
func Log10(x float64) float64 {
	if haveArchLog10 {
		return archLog10(x)
	}
	return log10(x)
}

func log10(x float64) float64 {
	return Log(x) * (1 / Ln10)
}

// Log2 returns the binary logarithm of x.
// The special cases are the same as for [Log].
func Log2(x float64) float64 {
	if haveArchLog2 {
		return archLog2(x)
	}
	return log2(x)
}

func log2(x float64) float64 {
	frac, exp := Frexp(x)
	// Make sure exact powers of two give an exact answer.
	// Don't depend on Log(0.5)*(1/Ln2)+exp being exactly exp-1.
	if frac == 0.5 {
		return float64(exp - 1)
	}
	return Log(frac)*(1/Ln2) + float64(exp)
}

```

// === FILE: references!/go/src/math/log10_s390x.s ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// Minimax polynomial coefficients and other constants
DATA log10rodataL19<>+0(SB)/8, $0.000000000000000000E+00
DATA log10rodataL19<>+8(SB)/8, $-1.0
DATA log10rodataL19<>+16(SB)/8, $0x7FF8000000000000   //+NanN
DATA log10rodataL19<>+24(SB)/8, $.15375570329280596749
DATA log10rodataL19<>+32(SB)/8, $.60171950900703668594E+04
DATA log10rodataL19<>+40(SB)/8, $-1.9578460454940795898
DATA log10rodataL19<>+48(SB)/8, $0.78962633073318517310E-01
DATA log10rodataL19<>+56(SB)/8, $-.71784211884836937993E-02
DATA log10rodataL19<>+64(SB)/8, $0.87011165920689940661E-03
DATA log10rodataL19<>+72(SB)/8, $-.11865158981621437541E-03
DATA log10rodataL19<>+80(SB)/8, $0.17258413403018680410E-04
DATA log10rodataL19<>+88(SB)/8, $0.40752932047883484315E-06
DATA log10rodataL19<>+96(SB)/8, $-.26149194688832680410E-05
DATA log10rodataL19<>+104(SB)/8, $0.92453396963875026759E-08
DATA log10rodataL19<>+112(SB)/8, $-.64572084905921579630E-07
DATA log10rodataL19<>+120(SB)/8, $-5.5
DATA log10rodataL19<>+128(SB)/8, $18446744073709551616.
GLOBL log10rodataL19<>+0(SB), RODATA, $136

// Table of log10 correction terms
DATA log10tab2074<>+0(SB)/8, $0.254164497922885069E-01
DATA log10tab2074<>+8(SB)/8, $0.179018857989381839E-01
DATA log10tab2074<>+16(SB)/8, $0.118926768029048674E-01
DATA log10tab2074<>+24(SB)/8, $0.722595568238080033E-02
DATA log10tab2074<>+32(SB)/8, $0.376393570022739135E-02
DATA log10tab2074<>+40(SB)/8, $0.138901135928814326E-02
DATA log10tab2074<>+48(SB)/8, $0
DATA log10tab2074<>+56(SB)/8, $-0.490780466387818203E-03
DATA log10tab2074<>+64(SB)/8, $-0.159811431402137571E-03
DATA log10tab2074<>+72(SB)/8, $0.925796337165100494E-03
DATA log10tab2074<>+80(SB)/8, $0.270683176738357035E-02
DATA log10tab2074<>+88(SB)/8, $0.513079030821304758E-02
DATA log10tab2074<>+96(SB)/8, $0.815089785397996303E-02
DATA log10tab2074<>+104(SB)/8, $0.117253060262419215E-01
DATA log10tab2074<>+112(SB)/8, $0.158164239345343963E-01
DATA log10tab2074<>+120(SB)/8, $0.203903595489229786E-01
GLOBL log10tab2074<>+0(SB), RODATA, $128

// Log10 returns the decimal logarithm of the argument.
//
// Special cases are:
//      Log(+Inf) = +Inf
//      Log(0) = -Inf
//      Log(x < 0) = NaN
//      Log(NaN) = NaN
// The algorithm used is minimax polynomial approximation
// with coefficients determined with a Remez exchange algorithm.

TEXT ·log10Asm(SB),NOSPLIT,$8-16
	FMOVD   x+0(FP), F0
	MOVD    $log10rodataL19<>+0(SB), R9
	FMOVD   F0, x-8(SP)
	WORD    $0xC0298006     //iilf %r2,2147909631
	BYTE    $0x7F
	BYTE    $0xFF
	WORD    $0x5840F008     //l %r4, 8(%r15)
	SUBW    R4, R2, R3
	RISBGZ	$32, $47, $0, R3, R5
	MOVH    $0x0, R1
	RISBGN	$0, $31, $32, R5, R1
	WORD    $0xC0590016     //iilf %r5,1507327
	BYTE    $0xFF
	BYTE    $0xFF
	MOVW    R4, R10
	MOVW    R5, R11
	CMPBLE  R10, R11, L2
	WORD    $0xC0297FEF     //iilf %r2,2146435071
	BYTE    $0xFF
	BYTE    $0xFF
	MOVW    R4, R10
	MOVW    R2, R11
	CMPBLE  R10, R11, L16
L3:
L1:
	FMOVD   F0, ret+8(FP)
	RET

L2:
	LTDBR	F0, F0
	BLEU    L13
	WORD    $0xED009080     //mdb %f0,.L20-.L19(%r9)
	BYTE    $0x00
	BYTE    $0x1C
	FMOVD   F0, x-8(SP)
	WORD    $0x5B20F008     //s %r2, 8(%r15)
	RISBGZ	$57, $60, $51, R2, R3
	ANDW    $0xFFFF0000, R2
	RISBGN	$0, $31, $32, R2, R1
	ADDW    $0x4000000, R2
	BLEU    L17
L8:
	SRW     $8, R2, R2
	ORW     $0x45000000, R2
L4:
	FMOVD   log10rodataL19<>+120(SB), F2
	LDGR    R1, F4
	WFMADB  V4, V0, V2, V0
	FMOVD   log10rodataL19<>+112(SB), F4
	FMOVD   log10rodataL19<>+104(SB), F6
	WFMADB  V0, V6, V4, V6
	FMOVD   log10rodataL19<>+96(SB), F4
	FMOVD   log10rodataL19<>+88(SB), F1
	WFMADB  V0, V1, V4, V1
	WFMDB   V0, V0, V4
	FMOVD   log10rodataL19<>+80(SB), F2
	WFMADB  V6, V4, V1, V6
	FMOVD   log10rodataL19<>+72(SB), F1
	WFMADB  V0, V2, V1, V2
	FMOVD   log10rodataL19<>+64(SB), F1
	RISBGZ	$57, $60, $0, R3, R3
	WFMADB  V4, V6, V2, V6
	FMOVD   log10rodataL19<>+56(SB), F2
	WFMADB  V0, V1, V2, V1
	VLVGF   $0, R2, V2
	WFMADB  V4, V6, V1, V4
	LDEBR   F2, F2
	FMOVD   log10rodataL19<>+48(SB), F6
	WFMADB  V0, V4, V6, V4
	FMOVD   log10rodataL19<>+40(SB), F1
	FMOVD   log10rodataL19<>+32(SB), F6
	MOVD    $log10tab2074<>+0(SB), R1
	WFMADB  V2, V1, V6, V2
	WORD    $0x68331000     //ld %f3,0(%r3,%r1)
	WFMADB  V0, V4, V3, V0
	FMOVD   log10rodataL19<>+24(SB), F4
	FMADD   F4, F2, F0
	FMOVD   F0, ret+8(FP)
	RET

L16:
	RISBGZ	$40, $55, $56, R3, R2
	RISBGZ	$57, $60, $51, R3, R3
	ORW     $0x45000000, R2
	BR      L4
L13:
	BGE     L18     //jnl .L18
	BVS     L18
	FMOVD   log10rodataL19<>+16(SB), F0
	BR      L1
L17:
	SRAW    $1, R2, R2
	SUBW    $0x40000000, R2
	BR      L8
L18:
	FMOVD   log10rodataL19<>+8(SB), F0
	WORD    $0xED009000     //ddb %f0,.L36-.L19(%r9)
	BYTE    $0x00
	BYTE    $0x1D
	BR      L1

```

// === FILE: references!/go/src/math/log1p.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// The original C code, the long comment, and the constants
// below are from FreeBSD's /usr/src/lib/msun/src/s_log1p.c
// and came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
//
// double log1p(double x)
//
// Method :
//   1. Argument Reduction: find k and f such that
//                      1+x = 2**k * (1+f),
//         where  sqrt(2)/2 < 1+f < sqrt(2) .
//
//      Note. If k=0, then f=x is exact. However, if k!=0, then f
//      may not be representable exactly. In that case, a correction
//      term is need. Let u=1+x rounded. Let c = (1+x)-u, then
//      log(1+x) - log(u) ~ c/u. Thus, we proceed to compute log(u),
//      and add back the correction term c/u.
//      (Note: when x > 2**53, one can simply return log(x))
//
//   2. Approximation of log1p(f).
//      Let s = f/(2+f) ; based on log(1+f) = log(1+s) - log(1-s)
//               = 2s + 2/3 s**3 + 2/5 s**5 + .....,
//               = 2s + s*R
//      We use a special Reme algorithm on [0,0.1716] to generate
//      a polynomial of degree 14 to approximate R The maximum error
//      of this polynomial approximation is bounded by 2**-58.45. In
//      other words,
//                      2      4      6      8      10      12      14
//          R(z) ~ Lp1*s +Lp2*s +Lp3*s +Lp4*s +Lp5*s  +Lp6*s  +Lp7*s
//      (the values of Lp1 to Lp7 are listed in the program)
//      and
//          |      2          14          |     -58.45
//          | Lp1*s +...+Lp7*s    -  R(z) | <= 2
//          |                             |
//      Note that 2s = f - s*f = f - hfsq + s*hfsq, where hfsq = f*f/2.
//      In order to guarantee error in log below 1ulp, we compute log
//      by
//              log1p(f) = f - (hfsq - s*(hfsq+R)).
//
//   3. Finally, log1p(x) = k*ln2 + log1p(f).
//                        = k*ln2_hi+(f-(hfsq-(s*(hfsq+R)+k*ln2_lo)))
//      Here ln2 is split into two floating point number:
//                   ln2_hi + ln2_lo,
//      where n*ln2_hi is always exact for |n| < 2000.
//
// Special cases:
//      log1p(x) is NaN with signal if x < -1 (including -INF) ;
//      log1p(+INF) is +INF; log1p(-1) is -INF with signal;
//      log1p(NaN) is that NaN with no signal.
//
// Accuracy:
//      according to an error analysis, the error is always less than
//      1 ulp (unit in the last place).
//
// Constants:
// The hexadecimal values are the intended ones for the following
// constants. The decimal values may be used, provided that the
// compiler will convert from decimal to binary accurately enough
// to produce the hexadecimal values shown.
//
// Note: Assuming log() return accurate answer, the following
//       algorithm can be used to compute log1p(x) to within a few ULP:
//
//              u = 1+x;
//              if(u==1.0) return x ; else
//                         return log(u)*(x/(u-1.0));
//
//       See HP-15C Advanced Functions Handbook, p.193.

// Log1p returns the natural logarithm of 1 plus its argument x.
// It is more accurate than [Log](1 + x) when x is near zero.
//
// Special cases are:
//
//	Log1p(+Inf) = +Inf
//	Log1p(±0) = ±0
//	Log1p(-1) = -Inf
//	Log1p(x < -1) = NaN
//	Log1p(NaN) = NaN
func Log1p(x float64) float64 {
	if haveArchLog1p {
		return archLog1p(x)
	}
	return log1p(x)
}

func log1p(x float64) float64 {
	const (
		Sqrt2M1     = 4.142135623730950488017e-01  // Sqrt(2)-1 = 0x3fda827999fcef34
		Sqrt2HalfM1 = -2.928932188134524755992e-01 // Sqrt(2)/2-1 = 0xbfd2bec333018866
		Small       = 1.0 / (1 << 29)              // 2**-29 = 0x3e20000000000000
		Tiny        = 1.0 / (1 << 54)              // 2**-54
		Two53       = 1 << 53                      // 2**53
		Ln2Hi       = 6.93147180369123816490e-01   // 3fe62e42fee00000
		Ln2Lo       = 1.90821492927058770002e-10   // 3dea39ef35793c76
		Lp1         = 6.666666666666735130e-01     // 3FE5555555555593
		Lp2         = 3.999999999940941908e-01     // 3FD999999997FA04
		Lp3         = 2.857142874366239149e-01     // 3FD2492494229359
		Lp4         = 2.222219843214978396e-01     // 3FCC71C51D8E78AF
		Lp5         = 1.818357216161805012e-01     // 3FC7466496CB03DE
		Lp6         = 1.531383769920937332e-01     // 3FC39A09D078C69F
		Lp7         = 1.479819860511658591e-01     // 3FC2F112DF3E5244
	)

	// special cases
	switch {
	case x < -1 || IsNaN(x): // includes -Inf
		return NaN()
	case x == -1:
		return Inf(-1)
	case IsInf(x, 1):
		return Inf(1)
	}

	absx := Abs(x)

	var f float64
	var iu uint64
	k := 1
	if absx < Sqrt2M1 { //  |x| < Sqrt(2)-1
		if absx < Small { // |x| < 2**-29
			if absx < Tiny { // |x| < 2**-54
				return x
			}
			return x - x*x*0.5
		}
		if x > Sqrt2HalfM1 { // Sqrt(2)/2-1 < x
			// (Sqrt(2)/2-1) < x < (Sqrt(2)-1)
			k = 0
			f = x
			iu = 1
		}
	}
	var c float64
	if k != 0 {
		var u float64
		if absx < Two53 { // 1<<53
			u = 1.0 + x
			iu = Float64bits(u)
			k = int((iu >> 52) - 1023)
			// correction term
			if k > 0 {
				c = 1.0 - (u - x)
			} else {
				c = x - (u - 1.0)
			}
			c /= u
		} else {
			u = x
			iu = Float64bits(u)
			k = int((iu >> 52) - 1023)
			c = 0
		}
		iu &= 0x000fffffffffffff
		if iu < 0x0006a09e667f3bcd { // mantissa of Sqrt(2)
			u = Float64frombits(iu | 0x3ff0000000000000) // normalize u
		} else {
			k++
			u = Float64frombits(iu | 0x3fe0000000000000) // normalize u/2
			iu = (0x0010000000000000 - iu) >> 2
		}
		f = u - 1.0 // Sqrt(2)/2 < u < Sqrt(2)
	}
	hfsq := 0.5 * f * f
	var s, R, z float64
	if iu == 0 { // |f| < 2**-20
		if f == 0 {
			if k == 0 {
				return 0
			}
			c += float64(k) * Ln2Lo
			return float64(k)*Ln2Hi + c
		}
		R = hfsq * (1.0 - 0.66666666666666666*f) // avoid division
		if k == 0 {
			return f - R
		}
		return float64(k)*Ln2Hi - ((R - (float64(k)*Ln2Lo + c)) - f)
	}
	s = f / (2.0 + f)
	z = s * s
	R = z * (Lp1 + z*(Lp2+z*(Lp3+z*(Lp4+z*(Lp5+z*(Lp6+z*Lp7))))))
	if k == 0 {
		return f - (hfsq - s*(hfsq+R))
	}
	return float64(k)*Ln2Hi - ((hfsq - (s*(hfsq+R) + (float64(k)*Ln2Lo + c))) - f)
}

```

// === FILE: references!/go/src/math/log1p_s390x.s ===
```text
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// Constants
DATA ·log1pxlim<> + 0(SB)/4, $0xfff00000
GLOBL ·log1pxlim<> + 0(SB), RODATA, $4
DATA ·log1pxzero<> + 0(SB)/8, $0.0
GLOBL ·log1pxzero<> + 0(SB), RODATA, $8
DATA ·log1pxminf<> + 0(SB)/8, $0xfff0000000000000
GLOBL ·log1pxminf<> + 0(SB), RODATA, $8
DATA ·log1pxnan<> + 0(SB)/8, $0x7ff8000000000000
GLOBL ·log1pxnan<> + 0(SB), RODATA, $8
DATA ·log1pyout<> + 0(SB)/8, $0x40fce621e71da000
GLOBL ·log1pyout<> + 0(SB), RODATA, $8
DATA ·log1pxout<> + 0(SB)/8, $0x40f1000000000000
GLOBL ·log1pxout<> + 0(SB), RODATA, $8
DATA ·log1pxl2<> + 0(SB)/8, $0xbfda7aecbeba4e46
GLOBL ·log1pxl2<> + 0(SB), RODATA, $8
DATA ·log1pxl1<> + 0(SB)/8, $0x3ffacde700000000
GLOBL ·log1pxl1<> + 0(SB), RODATA, $8
DATA ·log1pxa<> + 0(SB)/8, $5.5
GLOBL ·log1pxa<> + 0(SB), RODATA, $8
DATA ·log1pxmone<> + 0(SB)/8, $-1.0
GLOBL ·log1pxmone<> + 0(SB), RODATA, $8

// Minimax polynomial approximations
DATA ·log1pc8<> + 0(SB)/8, $0.212881813645679599E-07
GLOBL ·log1pc8<> + 0(SB), RODATA, $8
DATA ·log1pc7<> + 0(SB)/8, $-.148682720127920854E-06
GLOBL ·log1pc7<> + 0(SB), RODATA, $8
DATA ·log1pc6<> + 0(SB)/8, $0.938370938292558173E-06
GLOBL ·log1pc6<> + 0(SB), RODATA, $8
DATA ·log1pc5<> + 0(SB)/8, $-.602107458843052029E-05
GLOBL ·log1pc5<> + 0(SB), RODATA, $8
DATA ·log1pc4<> + 0(SB)/8, $0.397389654305194527E-04
GLOBL ·log1pc4<> + 0(SB), RODATA, $8
DATA ·log1pc3<> + 0(SB)/8, $-.273205381970859341E-03
GLOBL ·log1pc3<> + 0(SB), RODATA, $8
DATA ·log1pc2<> + 0(SB)/8, $0.200350613573012186E-02
GLOBL ·log1pc2<> + 0(SB), RODATA, $8
DATA ·log1pc1<> + 0(SB)/8, $-.165289256198351540E-01
GLOBL ·log1pc1<> + 0(SB), RODATA, $8
DATA ·log1pc0<> + 0(SB)/8, $0.181818181818181826E+00
GLOBL ·log1pc0<> + 0(SB), RODATA, $8


// Table of log10 correction terms
DATA ·log1ptab<> + 0(SB)/8, $0.585235384085551248E-01
DATA ·log1ptab<> + 8(SB)/8, $0.412206153771168640E-01
DATA ·log1ptab<> + 16(SB)/8, $0.273839003221648339E-01
DATA ·log1ptab<> + 24(SB)/8, $0.166383778368856480E-01
DATA ·log1ptab<> + 32(SB)/8, $0.866678223433169637E-02
DATA ·log1ptab<> + 40(SB)/8, $0.319831684989627514E-02
DATA ·log1ptab<> + 48(SB)/8, $-.000000000000000000E+00
DATA ·log1ptab<> + 56(SB)/8, $-.113006378583725549E-02
DATA ·log1ptab<> + 64(SB)/8, $-.367979419636602491E-03
DATA ·log1ptab<> + 72(SB)/8, $0.213172484510484979E-02
DATA ·log1ptab<> + 80(SB)/8, $0.623271047682013536E-02
DATA ·log1ptab<> + 88(SB)/8, $0.118140812789696885E-01
DATA ·log1ptab<> + 96(SB)/8, $0.187681358930914206E-01
DATA ·log1ptab<> + 104(SB)/8, $0.269985148668178992E-01
DATA ·log1ptab<> + 112(SB)/8, $0.364186619761331328E-01
DATA ·log1ptab<> + 120(SB)/8, $0.469505379381388441E-01
GLOBL ·log1ptab<> + 0(SB), RODATA, $128

// Log1p returns the natural logarithm of 1 plus its argument x.
// It is more accurate than Log(1 + x) when x is near zero.
//
// Special cases are:
//      Log1p(+Inf) = +Inf
//      Log1p(±0) = ±0
//      Log1p(-1) = -Inf
//      Log1p(x < -1) = NaN
//      Log1p(NaN) = NaN
// The algorithm used is minimax polynomial approximation
// with coefficients determined with a Remez exchange algorithm.

TEXT	·log1pAsm(SB), NOSPLIT, $0-16
	FMOVD	x+0(FP), F0
	MOVD	$·log1pxmone<>+0(SB), R1
	MOVD	·log1pxout<>+0(SB), R2
	FMOVD	0(R1), F3
	MOVD	$·log1pxa<>+0(SB), R1
	MOVWZ	·log1pxlim<>+0(SB), R0
	FMOVD	0(R1), F1
	MOVD	$·log1pc8<>+0(SB), R1
	FMOVD	0(R1), F5
	MOVD	$·log1pc7<>+0(SB), R1
	VLEG	$0, 0(R1), V20
	MOVD	$·log1pc6<>+0(SB), R1
	WFSDB	V0, V3, V4
	VLEG	$0, 0(R1), V18
	MOVD	$·log1pc5<>+0(SB), R1
	VLEG	$0, 0(R1), V16
	MOVD	R2, R5
	LGDR	F4, R3
	WORD	$0xC0190006	//iilf	%r1,425983
	BYTE	$0x7F
	BYTE	$0xFF
	SRAD	$32, R3, R3
	SUBW	R3, R1
	SRW	$16, R1, R1
	BYTE	$0x18	//lr	%r4,%r1
	BYTE	$0x41
	RISBGN	$0, $15, $48, R4, R2
	RISBGN	$16, $31, $32, R4, R5
	MOVW	R0, R6
	MOVW	R3, R7
	CMPBGT	R6, R7, L8
	WFCEDBS	V4, V4, V6
	MOVD	$·log1pxzero<>+0(SB), R1
	FMOVD	0(R1), F2
	BVS	LEXITTAGlog1p
	LCDBR	F4, F4
	WFCEDBS	V2, V4, V6
	BEQ	L9
	WFCHDBS	V4, V2, V2
	BEQ	LEXITTAGlog1p
	MOVD	$·log1pxnan<>+0(SB), R1
	FMOVD	0(R1), F0
	FMOVD	F0, ret+8(FP)
	RET

L8:
	LDGR	R2, F2
	FSUB	F4, F3
	FMADD	F2, F4, F1
	MOVD	$·log1pc4<>+0(SB), R2
	LCDBR	F1, F4
	FMOVD	0(R2), F7
	FSUB	F3, F0
	MOVD	$·log1pc3<>+0(SB), R2
	FMOVD	0(R2), F3
	MOVD	$·log1pc2<>+0(SB), R2
	WFMDB	V1, V1, V6
	FMADD	F7, F4, F3
	WFMSDB	V0, V2, V1, V0
	FMOVD	0(R2), F7
	WFMADB	V4, V5, V20, V5
	MOVD	$·log1pc1<>+0(SB), R2
	FMOVD	0(R2), F2
	FMADD	F7, F4, F2
	WFMADB	V4, V18, V16, V4
	FMADD	F3, F6, F2
	WFMADB	V5, V6, V4, V5
	FMUL	F6, F6
	MOVD	$·log1pc0<>+0(SB), R2
	WFMADB	V6, V5, V2, V6
	FMOVD	0(R2), F4
	WFMADB	V0, V6, V4, V6
	RISBGZ	$57, $60, $3, R1, R1
	MOVD	$·log1ptab<>+0(SB), R2
	MOVD	$·log1pxl1<>+0(SB), R3
	WORD	$0x68112000	//ld	%f1,0(%r1,%r2)
	FMOVD	0(R3), F2
	WFMADB	V0, V6, V1, V0
	MOVD	$·log1pyout<>+0(SB), R1
	LDGR	R5, F6
	FMOVD	0(R1), F4
	WFMSDB	V2, V6, V4, V2
	MOVD	$·log1pxl2<>+0(SB), R1
	FMOVD	0(R1), F4
	FMADD	F4, F2, F0
	FMOVD	F0, ret+8(FP)
	RET

L9:
	MOVD	$·log1pxminf<>+0(SB), R1
	FMOVD	0(R1), F0
	FMOVD	F0, ret+8(FP)
	RET


LEXITTAGlog1p:
	FMOVD	F0, ret+8(FP)
	RET


```

// === FILE: references!/go/src/math/log_amd64.s ===
```text
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

#define HSqrt2 7.07106781186547524401e-01 // sqrt(2)/2
#define Ln2Hi  6.93147180369123816490e-01 // 0x3fe62e42fee00000
#define Ln2Lo  1.90821492927058770002e-10 // 0x3dea39ef35793c76
#define L1     6.666666666666735130e-01   // 0x3FE5555555555593
#define L2     3.999999999940941908e-01   // 0x3FD999999997FA04
#define L3     2.857142874366239149e-01   // 0x3FD2492494229359
#define L4     2.222219843214978396e-01   // 0x3FCC71C51D8E78AF
#define L5     1.818357216161805012e-01   // 0x3FC7466496CB03DE
#define L6     1.531383769920937332e-01   // 0x3FC39A09D078C69F
#define L7     1.479819860511658591e-01   // 0x3FC2F112DF3E5244
#define NaN    0x7FF8000000000001
#define NegInf 0xFFF0000000000000
#define PosInf 0x7FF0000000000000

// func Log(x float64) float64
TEXT ·archLog(SB),NOSPLIT,$0
	// test bits for special cases
	MOVQ    x+0(FP), BX
	MOVQ    $~(1<<63), AX // sign bit mask
	ANDQ    BX, AX
	JEQ     isZero
	MOVQ    $0, AX
	CMPQ    AX, BX
	JGT     isNegative
	MOVQ    $PosInf, AX
	CMPQ    AX, BX
	JLE     isInfOrNaN
	// f1, ki := math.Frexp(x); k := float64(ki)
	MOVQ    BX, X0
	MOVQ    $0x000FFFFFFFFFFFFF, AX
	MOVQ    AX, X2
	ANDPD   X0, X2
	MOVSD   $0.5, X0 // 0x3FE0000000000000
	ORPD    X0, X2 // X2= f1
	SHRQ    $52, BX
	ANDL    $0x7FF, BX
	SUBL    $0x3FE, BX
	XORPS   X1, X1 // break dependency for CVTSL2SD
	CVTSL2SD BX, X1 // x1= k, x2= f1
	// if f1 < math.Sqrt2/2 { k -= 1; f1 *= 2 }
	MOVSD   $HSqrt2, X0 // x0= 0.7071, x1= k, x2= f1
	CMPSD   X2, X0, 5 // cmpnlt; x0= 0 or ^0, x1= k, x2 = f1
	MOVSD   $1.0, X3 // x0= 0 or ^0, x1= k, x2 = f1, x3= 1
	ANDPD   X0, X3 // x0= 0 or ^0, x1= k, x2 = f1, x3= 0 or 1
	SUBSD   X3, X1 // x0= 0 or ^0, x1= k, x2 = f1, x3= 0 or 1
	MOVSD   $1.0, X0 // x0= 1, x1= k, x2= f1, x3= 0 or 1
	ADDSD   X0, X3 // x0= 1, x1= k, x2= f1, x3= 1 or 2
	MULSD   X3, X2 // x0= 1, x1= k, x2= f1
	// f := f1 - 1
	SUBSD   X0, X2 // x1= k, x2= f
	// s := f / (2 + f)
	MOVSD   $2.0, X0
	ADDSD   X2, X0
	MOVAPD  X2, X3
	DIVSD   X0, X3 // x1=k, x2= f, x3= s
	// s2 := s * s
	MOVAPD  X3, X4 // x1= k, x2= f, x3= s
	MULSD   X4, X4 // x1= k, x2= f, x3= s, x4= s2
	// s4 := s2 * s2
	MOVAPD  X4, X5 // x1= k, x2= f, x3= s, x4= s2
	MULSD   X5, X5 // x1= k, x2= f, x3= s, x4= s2, x5= s4
	// t1 := s2 * (L1 + s4*(L3+s4*(L5+s4*L7)))
	MOVSD   $L7, X6
	MULSD   X5, X6
	ADDSD   $L5, X6
	MULSD   X5, X6
	ADDSD   $L3, X6
	MULSD   X5, X6
	ADDSD   $L1, X6
	MULSD   X6, X4 // x1= k, x2= f, x3= s, x4= t1, x5= s4
	// t2 := s4 * (L2 + s4*(L4+s4*L6))
	MOVSD   $L6, X6
	MULSD   X5, X6
	ADDSD   $L4, X6
	MULSD   X5, X6
	ADDSD   $L2, X6
	MULSD   X6, X5 // x1= k, x2= f, x3= s, x4= t1, x5= t2
	// R := t1 + t2
	ADDSD   X5, X4 // x1= k, x2= f, x3= s, x4= R
	// hfsq := 0.5 * f * f
	MOVSD   $0.5, X0
	MULSD   X2, X0
	MULSD   X2, X0 // x0= hfsq, x1= k, x2= f, x3= s, x4= R
	// return k*Ln2Hi - ((hfsq - (s*(hfsq+R) + k*Ln2Lo)) - f)
	ADDSD   X0, X4 // x0= hfsq, x1= k, x2= f, x3= s, x4= hfsq+R
	MULSD   X4, X3 // x0= hfsq, x1= k, x2= f, x3= s*(hfsq+R)
	MOVSD   $Ln2Lo, X4
	MULSD   X1, X4 // x4= k*Ln2Lo
	ADDSD   X4, X3 // x0= hfsq, x1= k, x2= f, x3= s*(hfsq+R)+k*Ln2Lo
	SUBSD   X3, X0 // x0= hfsq-(s*(hfsq+R)+k*Ln2Lo), x1= k, x2= f
	SUBSD   X2, X0 // x0= (hfsq-(s*(hfsq+R)+k*Ln2Lo))-f, x1= k
	MULSD   $Ln2Hi, X1 // x0= (hfsq-(s*(hfsq+R)+k*Ln2Lo))-f, x1= k*Ln2Hi
	SUBSD   X0, X1 // x1= k*Ln2Hi-((hfsq-(s*(hfsq+R)+k*Ln2Lo))-f)
	MOVSD   X1, ret+8(FP)
	RET
isInfOrNaN:
	MOVQ    BX, ret+8(FP) // +Inf or NaN, return x
	RET
isNegative:
	MOVQ    $NaN, AX
	MOVQ    AX, ret+8(FP) // return NaN
	RET
isZero:
	MOVQ    $NegInf, AX
	MOVQ    AX, ret+8(FP) // return -Inf
	RET

```

// === FILE: references!/go/src/math/log_asm.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build amd64 || s390x

package math

const haveArchLog = true

func archLog(x float64) float64

```

// === FILE: references!/go/src/math/log_s390x.s ===
```text
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// Minimax polynomial approximations
DATA ·logrodataL21<> + 0(SB)/8, $-.499999999999999778E+00
DATA ·logrodataL21<> + 8(SB)/8, $0.333333333333343751E+00
DATA ·logrodataL21<> + 16(SB)/8, $-.250000000001606881E+00
DATA ·logrodataL21<> + 24(SB)/8, $0.199999999971603032E+00
DATA ·logrodataL21<> + 32(SB)/8, $-.166666663114122038E+00
DATA ·logrodataL21<> + 40(SB)/8, $-.125002923782692399E+00
DATA ·logrodataL21<> + 48(SB)/8, $0.111142014580396256E+00
DATA ·logrodataL21<> + 56(SB)/8, $0.759438932618934220E-01
DATA ·logrodataL21<> + 64(SB)/8, $0.142857144267212549E+00
DATA ·logrodataL21<> + 72(SB)/8, $-.993038938793590759E-01
DATA ·logrodataL21<> + 80(SB)/8, $-1.0
GLOBL ·logrodataL21<> + 0(SB), RODATA, $88

// Constants
DATA ·logxminf<> + 0(SB)/8, $0xfff0000000000000
GLOBL ·logxminf<> + 0(SB), RODATA, $8
DATA ·logxnan<> + 0(SB)/8, $0x7ff8000000000000
GLOBL ·logxnan<> + 0(SB), RODATA, $8
DATA ·logx43f<> + 0(SB)/8, $0x43f0000000000000
GLOBL ·logx43f<> + 0(SB), RODATA, $8
DATA ·logxl2<> + 0(SB)/8, $0x3fda7aecbeba4e46
GLOBL ·logxl2<> + 0(SB), RODATA, $8
DATA ·logxl1<> + 0(SB)/8, $0x3ffacde700000000
GLOBL ·logxl1<> + 0(SB), RODATA, $8

/* Input transform scale and add constants */
DATA ·logxm<> + 0(SB)/8, $0x3fc77604e63c84b1
DATA ·logxm<> + 8(SB)/8, $0x40fb39456ab53250
DATA ·logxm<> + 16(SB)/8, $0x3fc9ee358b945f3f
DATA ·logxm<> + 24(SB)/8, $0x40fb39418bf3b137
DATA ·logxm<> + 32(SB)/8, $0x3fccfb2e1304f4b6
DATA ·logxm<> + 40(SB)/8, $0x40fb393d3eda3022
DATA ·logxm<> + 48(SB)/8, $0x3fd0000000000000
DATA ·logxm<> + 56(SB)/8, $0x40fb393969e70000
DATA ·logxm<> + 64(SB)/8, $0x3fd11117aafbfe04
DATA ·logxm<> + 72(SB)/8, $0x40fb3936eaefafcf
DATA ·logxm<> + 80(SB)/8, $0x3fd2492af5e658b2
DATA ·logxm<> + 88(SB)/8, $0x40fb39343ff01715
DATA ·logxm<> + 96(SB)/8, $0x3fd3b50c622a43dd
DATA ·logxm<> + 104(SB)/8, $0x40fb39315adae2f3
DATA ·logxm<> + 112(SB)/8, $0x3fd56bbeea918777
DATA ·logxm<> + 120(SB)/8, $0x40fb392e21698552
GLOBL ·logxm<> + 0(SB), RODATA, $128

// Log returns the natural logarithm of the argument.
//
// Special cases are:
//      Log(+Inf) = +Inf
//      Log(0) = -Inf
//      Log(x < 0) = NaN
//      Log(NaN) = NaN
// The algorithm used is minimax polynomial approximation using a table of
// polynomial coefficients determined with a Remez exchange algorithm.

TEXT	·logAsm(SB), NOSPLIT, $0-16
	FMOVD	x+0(FP), F0
	MOVD	$·logrodataL21<>+0(SB), R9
	MOVH	$0x8006, R4
	LGDR	F0, R1
	MOVD	$0x3FF0000000000000, R6
	SRAD	$48, R1, R1
	MOVD	$0x40F03E8000000000, R8
	SUBW	R1, R4
	RISBGZ	$32, $59, $0, R4, R2
	RISBGN	$0, $15, $48, R2, R6
	RISBGN	$16, $31, $32, R2, R8
	MOVW	R1, R7
	CMPBGT	R7, $22, L17
	LTDBR	F0, F0
	MOVD	$·logx43f<>+0(SB), R1
	FMOVD	0(R1), F2
	BLEU	L3
	MOVH	$0x8005, R12
	MOVH	$0x8405, R0
	BR	L15
L7:
	LTDBR	F0, F0
	BLEU	L3
L15:
	FMUL	F2, F0
	LGDR	F0, R1
	SRAD	$48, R1, R1
	SUBW	R1, R0, R2
	SUBW	R1, R12, R3
	BYTE	$0x18	//lr	%r4,%r2
	BYTE	$0x42
	ANDW	$0xFFFFFFF0, R3
	ANDW	$0xFFFFFFF0, R2
	BYTE	$0x18	//lr	%r5,%r1
	BYTE	$0x51
	MOVW	R1, R7
	CMPBLE	R7, $22, L7
	RISBGN	$0, $15, $48, R3, R6
	RISBGN	$16, $31, $32, R2, R8
L2:
	MOVH	R5, R5
	MOVH	$0x7FEF, R1
	CMPW	R5, R1
	BGT	L1
	LDGR	R6, F2
	FMUL	F2, F0
	RISBGZ	$57, $59, $3, R4, R4
	FMOVD	80(R9), F2
	MOVD	$·logxm<>+0(SB), R7
	ADD	R7, R4
	FMOVD	72(R9), F4
	WORD	$0xED004000	//madb	%f2,%f0,0(%r4)
	BYTE	$0x20
	BYTE	$0x1E
	FMOVD	64(R9), F1
	FMOVD	F2, F0
	FMOVD	56(R9), F2
	WFMADB	V0, V2, V4, V2
	WFMDB	V0, V0, V6
	FMOVD	48(R9), F4
	WFMADB	V0, V2, V4, V2
	FMOVD	40(R9), F4
	WFMADB	V2, V6, V1, V2
	FMOVD	32(R9), F1
	WFMADB	V6, V4, V1, V4
	FMOVD	24(R9), F1
	WFMADB	V6, V2, V1, V2
	FMOVD	16(R9), F1
	WFMADB	V6, V4, V1, V4
	MOVD	$·logxl1<>+0(SB), R1
	FMOVD	8(R9), F1
	WFMADB	V6, V2, V1, V2
	FMOVD	0(R9), F1
	WFMADB	V6, V4, V1, V4
	FMOVD	8(R4), F1
	WFMADB	V0, V2, V4, V2
	LDGR	R8, F4
	WFMADB	V6, V2, V0, V2
	WORD	$0xED401000	//msdb	%f1,%f4,0(%r1)
	BYTE	$0x10
	BYTE	$0x1F
	MOVD	·logxl2<>+0(SB), R1
	LCDBR	F1, F0
	LDGR	R1, F4
	WFMADB	V0, V4, V2, V0
L1:
	FMOVD	F0, ret+8(FP)
	RET
L3:
	LTDBR	F0, F0
	BEQ	L20
	BGE	L1
	BVS	L1

	MOVD	$·logxnan<>+0(SB), R1
	FMOVD	0(R1), F0
	BR	L1
L20:
	MOVD	$·logxminf<>+0(SB), R1
	FMOVD	0(R1), F0
	FMOVD	F0, ret+8(FP)
	RET
L17:
	BYTE	$0x18	//lr	%r5,%r1
	BYTE	$0x51
	BR	L2

```

// === FILE: references!/go/src/math/log_stub.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !amd64 && !s390x

package math

const haveArchLog = false

func archLog(x float64) float64 {
	panic("not implemented")
}

```

// === FILE: references!/go/src/math/logb.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// Logb returns the binary exponent of x.
//
// Special cases are:
//
//	Logb(±Inf) = +Inf
//	Logb(0) = -Inf
//	Logb(NaN) = NaN
func Logb(x float64) float64 {
	// special cases
	switch {
	case x == 0:
		return Inf(-1)
	case IsInf(x, 0):
		return Inf(1)
	case IsNaN(x):
		return x
	}
	return float64(ilogb(x))
}

// Ilogb returns the binary exponent of x as an integer.
//
// Special cases are:
//
//	Ilogb(±Inf) = MaxInt32
//	Ilogb(0) = MinInt32
//	Ilogb(NaN) = MaxInt32
func Ilogb(x float64) int {
	// special cases
	switch {
	case x == 0:
		return MinInt32
	case IsNaN(x):
		return MaxInt32
	case IsInf(x, 0):
		return MaxInt32
	}
	return ilogb(x)
}

// ilogb returns the binary exponent of x. It assumes x is finite and
// non-zero.
func ilogb(x float64) int {
	x, exp := normalize(x)
	return int((Float64bits(x)>>shift)&mask) - bias + exp
}

```

// === FILE: references!/go/src/math/mod.go ===
```go
// Copyright 2009-2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

/*
	Floating-point mod function.
*/

// Mod returns the floating-point remainder of x/y.
// The magnitude of the result is less than y and its
// sign agrees with that of x.
//
// Special cases are:
//
//	Mod(±Inf, y) = NaN
//	Mod(NaN, y) = NaN
//	Mod(x, 0) = NaN
//	Mod(x, ±Inf) = x
//	Mod(x, NaN) = NaN
func Mod(x, y float64) float64 {
	if haveArchMod {
		return archMod(x, y)
	}
	return mod(x, y)
}

func mod(x, y float64) float64 {
	if y == 0 || IsInf(x, 0) || IsNaN(x) || IsNaN(y) {
		return NaN()
	}
	y = Abs(y)

	yfr, yexp := Frexp(y)
	r := x
	if x < 0 {
		r = -x
	}

	for r >= y {
		rfr, rexp := Frexp(r)
		if rfr < yfr {
			rexp = rexp - 1
		}
		r = r - Ldexp(y, rexp-yexp)
	}
	if x < 0 {
		r = -r
	}
	return r
}

```

// === FILE: references!/go/src/math/modf.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// Modf returns integer and fractional floating-point numbers
// that sum to f. Both values have the same sign as f.
//
// Special cases are:
//
//	Modf(±Inf) = ±Inf, NaN
//	Modf(NaN) = NaN, NaN
func Modf(f float64) (integer float64, fractional float64) {
	integer = Trunc(f)
	fractional = Copysign(f-integer, f)
	return
}

```

// === FILE: references!/go/src/math/nextafter.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// Nextafter32 returns the next representable float32 value after x towards y.
//
// Special cases are:
//
//	Nextafter32(x, x)   = x
//	Nextafter32(NaN, y) = NaN
//	Nextafter32(x, NaN) = NaN
func Nextafter32(x, y float32) (r float32) {
	switch {
	case IsNaN(float64(x)) || IsNaN(float64(y)): // special case
		r = float32(NaN())
	case x == y:
		r = x
	case x == 0:
		r = float32(Copysign(float64(Float32frombits(1)), float64(y)))
	case (y > x) == (x > 0):
		r = Float32frombits(Float32bits(x) + 1)
	default:
		r = Float32frombits(Float32bits(x) - 1)
	}
	return
}

// Nextafter returns the next representable float64 value after x towards y.
//
// Special cases are:
//
//	Nextafter(x, x)   = x
//	Nextafter(NaN, y) = NaN
//	Nextafter(x, NaN) = NaN
func Nextafter(x, y float64) (r float64) {
	switch {
	case IsNaN(x) || IsNaN(y): // special case
		r = NaN()
	case x == y:
		r = x
	case x == 0:
		r = Copysign(Float64frombits(1), y)
	case (y > x) == (x > 0):
		r = Float64frombits(Float64bits(x) + 1)
	default:
		r = Float64frombits(Float64bits(x) - 1)
	}
	return
}

```

// === FILE: references!/go/src/math/pow.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

func isOddInt(x float64) bool {
	if Abs(x) >= (1 << 53) {
		// 1 << 53 is the largest exact integer in the float64 format.
		// Any number outside this range will be truncated before the decimal point and therefore will always be
		// an even integer.
		// Without this check and if x overflows int64 the int64(xi) conversion below may produce incorrect results
		// on some architectures (and does so on arm64). See issue #57465.
		return false
	}

	xi, xf := Modf(x)
	return xf == 0 && int64(xi)&1 == 1
}

// Special cases taken from FreeBSD's /usr/src/lib/msun/src/e_pow.c
// updated by IEEE Std. 754-2008 "Section 9.2.1 Special values".

// Pow returns x**y, the base-x exponential of y.
//
// Special cases are (in order):
//
//	Pow(x, ±0) = 1 for any x
//	Pow(1, y) = 1 for any y
//	Pow(x, 1) = x for any x
//	Pow(NaN, y) = NaN
//	Pow(x, NaN) = NaN
//	Pow(±0, y) = ±Inf for y an odd integer < 0
//	Pow(±0, -Inf) = +Inf
//	Pow(±0, +Inf) = +0
//	Pow(±0, y) = +Inf for finite y < 0 and not an odd integer
//	Pow(±0, y) = ±0 for y an odd integer > 0
//	Pow(±0, y) = +0 for finite y > 0 and not an odd integer
//	Pow(-1, ±Inf) = 1
//	Pow(x, +Inf) = +Inf for |x| > 1
//	Pow(x, -Inf) = +0 for |x| > 1
//	Pow(x, +Inf) = +0 for |x| < 1
//	Pow(x, -Inf) = +Inf for |x| < 1
//	Pow(+Inf, y) = +Inf for y > 0
//	Pow(+Inf, y) = +0 for y < 0
//	Pow(-Inf, y) = Pow(-0, -y)
//	Pow(x, y) = NaN for finite x < 0 and finite non-integer y
func Pow(x, y float64) float64 {
	if haveArchPow {
		return archPow(x, y)
	}
	return pow(x, y)
}

func pow(x, y float64) float64 {
	switch {
	case y == 0 || x == 1:
		return 1
	case y == 1:
		return x
	case IsNaN(x) || IsNaN(y):
		return NaN()
	case x == 0:
		switch {
		case y < 0:
			if Signbit(x) && isOddInt(y) {
				return Inf(-1)
			}
			return Inf(1)
		case y > 0:
			if Signbit(x) && isOddInt(y) {
				return x
			}
			return 0
		}
	case IsInf(y, 0):
		switch {
		case x == -1:
			return 1
		case (Abs(x) < 1) == IsInf(y, 1):
			return 0
		default:
			return Inf(1)
		}
	case IsInf(x, 0):
		if IsInf(x, -1) {
			return Pow(1/x, -y) // Pow(-0, -y)
		}
		switch {
		case y < 0:
			return 0
		case y > 0:
			return Inf(1)
		}
	case y == 0.5:
		return Sqrt(x)
	case y == -0.5:
		return 1 / Sqrt(x)
	}

	yi, yf := Modf(Abs(y))
	if yf != 0 && x < 0 {
		return NaN()
	}
	if yi >= 1<<63 {
		// yi is a large even int that will lead to overflow (or underflow to 0)
		// for all x except -1 (x == 1 was handled earlier)
		switch {
		case x == -1:
			return 1
		case (Abs(x) < 1) == (y > 0):
			return 0
		default:
			return Inf(1)
		}
	}

	// ans = a1 * 2**ae (= 1 for now).
	a1 := 1.0
	ae := 0

	// ans *= x**yf
	if yf != 0 {
		if yf > 0.5 {
			yf--
			yi++
		}
		a1 = Exp(yf * Log(x))
	}

	// ans *= x**yi
	// by multiplying in successive squarings
	// of x according to bits of yi.
	// accumulate powers of two into exp.
	x1, xe := Frexp(x)
	for i := int64(yi); i != 0; i >>= 1 {
		if xe < -1<<12 || 1<<12 < xe {
			// catch xe before it overflows the left shift below
			// Since i !=0 it has at least one bit still set, so ae will accumulate xe
			// on at least one more iteration, ae += xe is a lower bound on ae
			// the lower bound on ae exceeds the size of a float64 exp
			// so the final call to Ldexp will produce under/overflow (0/Inf)
			ae += xe
			break
		}
		if i&1 == 1 {
			a1 *= x1
			ae += xe
		}
		x1 *= x1
		xe <<= 1
		if x1 < .5 {
			x1 += x1
			xe--
		}
	}

	// ans = a1*2**ae
	// if y < 0 { ans = 1 / ans }
	// but in the opposite order
	if y < 0 {
		a1 = 1 / a1
		ae = -ae
	}
	return Ldexp(a1, ae)
}

```

// === FILE: references!/go/src/math/pow10.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// pow10tab stores the pre-computed values 10**i for i < 32.
var pow10tab = [...]float64{
	1e00, 1e01, 1e02, 1e03, 1e04, 1e05, 1e06, 1e07, 1e08, 1e09,
	1e10, 1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18, 1e19,
	1e20, 1e21, 1e22, 1e23, 1e24, 1e25, 1e26, 1e27, 1e28, 1e29,
	1e30, 1e31,
}

// pow10postab32 stores the pre-computed value for 10**(i*32) at index i.
var pow10postab32 = [...]float64{
	1e00, 1e32, 1e64, 1e96, 1e128, 1e160, 1e192, 1e224, 1e256, 1e288,
}

// pow10negtab32 stores the pre-computed value for 10**(-i*32) at index i.
var pow10negtab32 = [...]float64{
	1e-00, 1e-32, 1e-64, 1e-96, 1e-128, 1e-160, 1e-192, 1e-224, 1e-256, 1e-288, 1e-320,
}

// Pow10 returns 10**n, the base-10 exponential of n.
//
// Special cases are:
//
//	Pow10(n) =    0 for n < -323
//	Pow10(n) = +Inf for n > 308
func Pow10(n int) float64 {
	if 0 <= n && n <= 308 {
		return pow10postab32[uint(n)/32] * pow10tab[uint(n)%32]
	}

	if -323 <= n && n < 0 {
		return pow10negtab32[uint(-n)/32] / pow10tab[uint(-n)%32]
	}

	// n < -323 || 308 < n
	if n > 0 {
		return Inf(1)
	}

	// n < -323
	return 0
}

```

// === FILE: references!/go/src/math/pow_s390x.s ===
```text
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

#define PosInf   0x7FF0000000000000
#define NaN      0x7FF8000000000001
#define NegInf   0xFFF0000000000000
#define PosOne   0x3FF0000000000000
#define NegOne   0xBFF0000000000000
#define NegZero  0x8000000000000000

// Minimax polynomial approximation
DATA ·powrodataL51<> + 0(SB)/8, $-1.0
DATA ·powrodataL51<> + 8(SB)/8, $1.0
DATA ·powrodataL51<> + 16(SB)/8, $0.24022650695910110361E+00
DATA ·powrodataL51<> + 24(SB)/8, $0.69314718055994686185E+00
DATA ·powrodataL51<> + 32(SB)/8, $0.96181291057109484809E-02
DATA ·powrodataL51<> + 40(SB)/8, $0.15403814778342868389E-03
DATA ·powrodataL51<> + 48(SB)/8, $0.55504108652095235601E-01
DATA ·powrodataL51<> + 56(SB)/8, $0.13333818813168698658E-02
DATA ·powrodataL51<> + 64(SB)/8, $0.68205322933914439200E-12
DATA ·powrodataL51<> + 72(SB)/8, $-.18466496523378731640E-01
DATA ·powrodataL51<> + 80(SB)/8, $0.19697596291603973706E-02
DATA ·powrodataL51<> + 88(SB)/8, $0.23083120654155209200E+00
DATA ·powrodataL51<> + 96(SB)/8, $0.55324356012093416771E-06
DATA ·powrodataL51<> + 104(SB)/8, $-.40340677224649339048E-05
DATA ·powrodataL51<> + 112(SB)/8, $0.30255507904062541562E-04
DATA ·powrodataL51<> + 120(SB)/8, $-.77453979912413008787E-07
DATA ·powrodataL51<> + 128(SB)/8, $-.23637115549923464737E-03
DATA ·powrodataL51<> + 136(SB)/8, $0.11016119077267717198E-07
DATA ·powrodataL51<> + 144(SB)/8, $0.22608272174486123035E-09
DATA ·powrodataL51<> + 152(SB)/8, $-.15895808101370190382E-08
DATA ·powrodataL51<> + 160(SB)/8, $0x4540190000000000
GLOBL ·powrodataL51<> + 0(SB), RODATA, $168

// Constants
DATA ·pow_x001a<> + 0(SB)/8, $0x1a000000000000
GLOBL ·pow_x001a<> + 0(SB), RODATA, $8
DATA ·pow_xinf<> + 0(SB)/8, $0x7ff0000000000000      //+Inf
GLOBL ·pow_xinf<> + 0(SB), RODATA, $8
DATA ·pow_xnan<> + 0(SB)/8, $0x7ff8000000000000      //NaN
GLOBL ·pow_xnan<> + 0(SB), RODATA, $8
DATA ·pow_x434<> + 0(SB)/8, $0x4340000000000000
GLOBL ·pow_x434<> + 0(SB), RODATA, $8
DATA ·pow_x433<> + 0(SB)/8, $0x4330000000000000
GLOBL ·pow_x433<> + 0(SB), RODATA, $8
DATA ·pow_x43f<> + 0(SB)/8, $0x43f0000000000000
GLOBL ·pow_x43f<> + 0(SB), RODATA, $8
DATA ·pow_xadd<> + 0(SB)/8, $0xc2f0000100003fef
GLOBL ·pow_xadd<> + 0(SB), RODATA, $8
DATA ·pow_xa<> + 0(SB)/8, $0x4019000000000000
GLOBL ·pow_xa<> + 0(SB), RODATA, $8

// Scale correction tables
DATA powiadd<> + 0(SB)/8, $0xf000000000000000
DATA powiadd<> + 8(SB)/8, $0x1000000000000000
GLOBL powiadd<> + 0(SB), RODATA, $16
DATA powxscale<> + 0(SB)/8, $0x4ff0000000000000
DATA powxscale<> + 8(SB)/8, $0x2ff0000000000000
GLOBL powxscale<> + 0(SB), RODATA, $16

// Fractional powers of 2 table
DATA ·powtexp<> + 0(SB)/8, $0.442737824274138381E-01
DATA ·powtexp<> + 8(SB)/8, $0.263602189790660309E-01
DATA ·powtexp<> + 16(SB)/8, $0.122565642281703586E-01
DATA ·powtexp<> + 24(SB)/8, $0.143757052860721398E-02
DATA ·powtexp<> + 32(SB)/8, $-.651375034121276075E-02
DATA ·powtexp<> + 40(SB)/8, $-.119317678849450159E-01
DATA ·powtexp<> + 48(SB)/8, $-.150868749549871069E-01
DATA ·powtexp<> + 56(SB)/8, $-.161992609578469234E-01
DATA ·powtexp<> + 64(SB)/8, $-.154492360403337917E-01
DATA ·powtexp<> + 72(SB)/8, $-.129850717389178721E-01
DATA ·powtexp<> + 80(SB)/8, $-.892902649276657891E-02
DATA ·powtexp<> + 88(SB)/8, $-.338202636596794887E-02
DATA ·powtexp<> + 96(SB)/8, $0.357266307045684762E-02
DATA ·powtexp<> + 104(SB)/8, $0.118665304327406698E-01
DATA ·powtexp<> + 112(SB)/8, $0.214434994118118914E-01
DATA ·powtexp<> + 120(SB)/8, $0.322580645161290314E-01
GLOBL ·powtexp<> + 0(SB), RODATA, $128

// Log multiplier tables
DATA ·powtl<> + 0(SB)/8, $0xbdf9723a80db6a05
DATA ·powtl<> + 8(SB)/8, $0x3e0cfe4a0babe862
DATA ·powtl<> + 16(SB)/8, $0xbe163b42dd33dada
DATA ·powtl<> + 24(SB)/8, $0xbe0cdf9de2a8429c
DATA ·powtl<> + 32(SB)/8, $0xbde9723a80db6a05
DATA ·powtl<> + 40(SB)/8, $0xbdb37fcae081745e
DATA ·powtl<> + 48(SB)/8, $0xbdd8b2f901ac662c
DATA ·powtl<> + 56(SB)/8, $0xbde867dc68c36cc9
DATA ·powtl<> + 64(SB)/8, $0xbdd23e36b47256b7
DATA ·powtl<> + 72(SB)/8, $0xbde4c9b89fcc7933
DATA ·powtl<> + 80(SB)/8, $0xbdd16905cad7cf66
DATA ·powtl<> + 88(SB)/8, $0x3ddb417414aa5529
DATA ·powtl<> + 96(SB)/8, $0xbdce046f2889983c
DATA ·powtl<> + 104(SB)/8, $0x3dc2c3865d072897
DATA ·powtl<> + 112(SB)/8, $0x8000000000000000
DATA ·powtl<> + 120(SB)/8, $0x3dc1ca48817f8afe
DATA ·powtl<> + 128(SB)/8, $0xbdd703518a88bfb7
DATA ·powtl<> + 136(SB)/8, $0x3dc64afcc46942ce
DATA ·powtl<> + 144(SB)/8, $0xbd9d79191389891a
DATA ·powtl<> + 152(SB)/8, $0x3ddd563044da4fa0
DATA ·powtl<> + 160(SB)/8, $0x3e0f42b5e5f8f4b6
DATA ·powtl<> + 168(SB)/8, $0x3e0dfa2c2cbf6ead
DATA ·powtl<> + 176(SB)/8, $0x3e14e25e91661293
DATA ·powtl<> + 184(SB)/8, $0x3e0aac461509e20c
GLOBL ·powtl<> + 0(SB), RODATA, $192

DATA ·powtm<> + 0(SB)/8, $0x3da69e13
DATA ·powtm<> + 8(SB)/8, $0x100003d66fcb6
DATA ·powtm<> + 16(SB)/8, $0x200003d1538df
DATA ·powtm<> + 24(SB)/8, $0x300003cab729e
DATA ·powtm<> + 32(SB)/8, $0x400003c1a784c
DATA ·powtm<> + 40(SB)/8, $0x500003ac9b074
DATA ·powtm<> + 48(SB)/8, $0x60000bb498d22
DATA ·powtm<> + 56(SB)/8, $0x68000bb8b29a2
DATA ·powtm<> + 64(SB)/8, $0x70000bb9a32d4
DATA ·powtm<> + 72(SB)/8, $0x74000bb9946bb
DATA ·powtm<> + 80(SB)/8, $0x78000bb92e34b
DATA ·powtm<> + 88(SB)/8, $0x80000bb6c57dc
DATA ·powtm<> + 96(SB)/8, $0x84000bb4020f7
DATA ·powtm<> + 104(SB)/8, $0x8c000ba93832d
DATA ·powtm<> + 112(SB)/8, $0x9000080000000
DATA ·powtm<> + 120(SB)/8, $0x940003aa66c4c
DATA ·powtm<> + 128(SB)/8, $0x980003b2fb12a
DATA ·powtm<> + 136(SB)/8, $0xa00003bc1def6
DATA ·powtm<> + 144(SB)/8, $0xa80003c1eb0eb
DATA ·powtm<> + 152(SB)/8, $0xb00003c64dcec
DATA ·powtm<> + 160(SB)/8, $0xc00003cc49e4e
DATA ·powtm<> + 168(SB)/8, $0xd00003d12f1de
DATA ·powtm<> + 176(SB)/8, $0xe00003d4a9c6f
DATA ·powtm<> + 184(SB)/8, $0xf00003d846c66
GLOBL ·powtm<> + 0(SB), RODATA, $192

// Table of indices into multiplier tables
// Adjusted from asm to remove offset and convert
DATA ·powtabi<> + 0(SB)/8, $0x1010101
DATA ·powtabi<> + 8(SB)/8, $0x101020202020203
DATA ·powtabi<> + 16(SB)/8, $0x303030404040405
DATA ·powtabi<> + 24(SB)/8, $0x505050606060708
DATA ·powtabi<> + 32(SB)/8, $0x90a0b0c0d0e0f10
DATA ·powtabi<> + 40(SB)/8, $0x1011111212121313
DATA ·powtabi<> + 48(SB)/8, $0x1314141414151515
DATA ·powtabi<> + 56(SB)/8, $0x1516161617171717
GLOBL ·powtabi<> + 0(SB), RODATA, $64

// Pow returns x**y, the base-x exponential of y.
//
// Special cases are (in order):
//      Pow(x, ±0) = 1 for any x
//      Pow(1, y) = 1 for any y
//      Pow(x, 1) = x for any x
//      Pow(NaN, y) = NaN
//      Pow(x, NaN) = NaN
//      Pow(±0, y) = ±Inf for y an odd integer < 0
//      Pow(±0, -Inf) = +Inf
//      Pow(±0, +Inf) = +0
//      Pow(±0, y) = +Inf for finite y < 0 and not an odd integer
//      Pow(±0, y) = ±0 for y an odd integer > 0
//      Pow(±0, y) = +0 for finite y > 0 and not an odd integer
//      Pow(-1, ±Inf) = 1
//      Pow(x, +Inf) = +Inf for |x| > 1
//      Pow(x, -Inf) = +0 for |x| > 1
//      Pow(x, +Inf) = +0 for |x| < 1
//      Pow(x, -Inf) = +Inf for |x| < 1
//      Pow(+Inf, y) = +Inf for y > 0
//      Pow(+Inf, y) = +0 for y < 0
//      Pow(-Inf, y) = Pow(-0, -y)
//      Pow(x, y) = NaN for finite x < 0 and finite non-integer y

TEXT	·powAsm(SB), NOSPLIT, $0-24
	// special case
	MOVD	x+0(FP), R1
	MOVD	y+8(FP), R2

	// special case Pow(1, y) = 1 for any y
	MOVD	$PosOne, R3
	CMPUBEQ	R1, R3, xIsOne

	// special case Pow(x, 1) = x for any x
	MOVD	$PosOne, R4
	CMPUBEQ	R2, R4, yIsOne

	// special case Pow(x, NaN) = NaN for any x
	MOVD	$~(1<<63), R5
	AND	R2, R5    // y = |y|
	MOVD	$PosInf, R4
	CMPUBLT R4, R5, yIsNan

	MOVD	$NegInf, R3
	CMPUBEQ	R1, R3, xIsNegInf

	MOVD	$NegOne, R3
	CMPUBEQ	R1, R3, xIsNegOne

	MOVD	$PosInf, R3
	CMPUBEQ	R1, R3, xIsPosInf

	MOVD	$NegZero, R3
	CMPUBEQ	R1, R3, xIsNegZero

	MOVD	$PosInf, R4
	CMPUBEQ	R2, R4, yIsPosInf

	MOVD	$0x0, R3
	CMPUBEQ	R1, R3, xIsPosZero
	CMPBLT	R1, R3, xLtZero
	BR	Normal
xIsPosInf:
	// special case Pow(+Inf, y) = +Inf for y > 0
	MOVD	$0x0, R4
	CMPBGT	R2, R4, posInfGeZero
	BR	Normal
xIsNegInf:
	//Pow(-Inf, y) = Pow(-0, -y)
	FMOVD y+8(FP), F2
	FNEG F2, F2			// y = -y
	BR negZeroNegY		// call Pow(-0, -y)
xIsNegOne:
	// special case Pow(-1, ±Inf) = 1
	MOVD	$PosInf, R4
	CMPUBEQ	R2, R4, negOnePosInf
	MOVD	$NegInf, R4
	CMPUBEQ	R2, R4, negOneNegInf
	BR	Normal
xIsPosZero:
	// special case Pow(+0, -Inf) = +Inf
	MOVD	$NegInf, R4
	CMPUBEQ	R2, R4, zeroNegInf

	// special case Pow(+0, y < 0) = +Inf
	FMOVD	y+8(FP), F2
	FMOVD	$(0.0), F4
	FCMPU	F2, F4
	BLT	posZeroLtZero				//y < 0.0
	BR	Normal
xIsNegZero:
	// special case Pow(-0, -Inf) = +Inf
	MOVD	$NegInf, R4
	CMPUBEQ	R2, R4, zeroNegInf
	FMOVD	y+8(FP), F2
negZeroNegY:
	// special case Pow(x, ±0) = 1 for any x
	FMOVD	$(0.0), F4
	FCMPU	F4, F2
	BLT	negZeroGtZero		// y > 0.0
	BEQ yIsZero				// y = 0.0

	FMOVD $(-0.0), F4
	FCMPU F4, F2
	BLT negZeroGtZero				// y > -0.0
	BEQ yIsZero				// y = -0.0

	// special case Pow(-0, y) = -Inf for y an odd integer < 0
	// special case Pow(-0, y) = +Inf for finite y < 0 and not an odd integer
	FIDBR	$5, F2, F4		//F2 translate to integer F4
	FCMPU	F2, F4
	BNE	zeroNotOdd			// y is not an (odd) integer and y < 0
	FMOVD	$(2.0), F4
	FDIV	F4, F2			// F2 = F2 / 2.0
	FIDBR	$5, F2, F4		//F2 translate to integer F4
	FCMPU	F2, F4
	BNE	negZeroOddInt		// y is an odd integer and y < 0
	BR	zeroNotOdd			// y is not an (odd) integer and y < 0

negZeroGtZero:
	// special case Pow(-0, y) = -0 for y an odd integer > 0
	// special case Pow(±0, y) = +0 for finite y > 0 and not an odd integer
	FIDBR	$5, F2, F4      //F2 translate to integer F4
	FCMPU	F2, F4
	BNE	zeroNotOddGtZero    // y is not an (odd) integer and y > 0
	FMOVD	$(2.0), F4
	FDIV	F4, F2          // F2 = F2 / 2.0
	FIDBR	$5, F2, F4      //F2 translate to integer F4
	FCMPU	F2, F4
	BNE	negZeroOddIntGtZero       // y is an odd integer and y > 0
	BR	zeroNotOddGtZero          // y is not an (odd) integer

xLtZero:
	// special case Pow(x, y) = NaN for finite x < 0 and finite non-integer y
	FMOVD	y+8(FP), F2
	FIDBR	$5, F2, F4
	FCMPU	F2, F4
	BNE	ltZeroInt
	BR	Normal
yIsPosInf:
	// special case Pow(x, +Inf) = +Inf for |x| > 1
	FMOVD	x+0(FP), F1
	FMOVD	$(1.0), F3
	FCMPU	F1, F3
	BGT	gtOnePosInf
	FMOVD	$(-1.0), F3
	FCMPU	F1, F3
	BLT	ltNegOnePosInf
Normal:
	FMOVD	x+0(FP), F0
	FMOVD	y+8(FP), F2
	MOVD	$·powrodataL51<>+0(SB), R9
	LGDR	F0, R3
	WORD	$0xC0298009	//iilf	%r2,2148095317
	BYTE	$0x55
	BYTE	$0x55
	RISBGNZ	$32, $63, $32, R3, R1
	SUBW	R1, R2
	RISBGNZ	$58, $63, $50, R2, R3
	BYTE	$0x18	//lr	%r5,%r1
	BYTE	$0x51
	MOVD	$·powtabi<>+0(SB), R12
	WORD	$0xE303C000	//llgc	%r0,0(%r3,%r12)
	BYTE	$0x00
	BYTE	$0x90
	SUBW	$0x1A0000, R5
	SLD	$3, R0, R3
	MOVD	$·powtm<>+0(SB), R4
	MOVH	$0x0, R8
	ANDW	$0x7FF00000, R2
	ORW	R5, R1
	WORD	$0x5A234000	//a	%r2,0(%r3,%r4)
	MOVD	$0x3FF0000000000000, R5
	RISBGZ	$40, $63, $56, R2, R3
	RISBGN	$0, $31, $32, R2, R8
	ORW	$0x45000000, R3
	MOVW	R1, R6
	CMPBLT	R6, $0, L42
	FMOVD	F0, F4
L2:
	VLVGF	$0, R3, V1
	MOVD	$·pow_xa<>+0(SB), R2
	WORD	$0xED3090A0	//lde	%f3,.L52-.L51(%r9)
	BYTE	$0x00
	BYTE	$0x24
	FMOVD	0(R2), F6
	FSUBS	F1, F3
	LDGR	R8, F1
	WFMSDB	V4, V1, V6, V4
	FMOVD	152(R9), F6
	WFMDB	V4, V4, V7
	FMOVD	144(R9), F1
	FMOVD	136(R9), F5
	WFMADB	V4, V1, V6, V1
	VLEG	$0, 128(R9), V16
	FMOVD	120(R9), F6
	WFMADB	V4, V5, V6, V5
	FMOVD	112(R9), F6
	WFMADB	V1, V7, V5, V1
	WFMADB	V4, V6, V16, V16
	SLD	$3, R0, R2
	FMOVD	104(R9), F5
	WORD	$0xED824004	//ldeb	%f8,4(%r2,%r4)
	BYTE	$0x00
	BYTE	$0x04
	LDEBR	F3, F3
	FMOVD	96(R9), F6
	WFMADB	V4, V6, V5, V6
	FADD	F8, F3
	WFMADB	V7, V6, V16, V6
	FMUL	F7, F7
	FMOVD	88(R9), F5
	FMADD	F7, F1, F6
	WFMADB	V4, V5, V3, V16
	FMOVD	80(R9), F1
	WFSDB	V16, V3, V3
	MOVD	$·powtl<>+0(SB), R3
	WFMADB	V4, V6, V1, V6
	FMADD	F5, F4, F3
	FMOVD	72(R9), F1
	WFMADB	V4, V6, V1, V6
	WORD	$0xED323000	//adb	%f3,0(%r2,%r3)
	BYTE	$0x00
	BYTE	$0x1A
	FMOVD	64(R9), F1
	WFMADB	V4, V6, V1, V6
	MOVD	$·pow_xadd<>+0(SB), R2
	WFMADB	V4, V6, V3, V4
	FMOVD	0(R2), F5
	WFADB	V4, V16, V3
	VLEG	$0, 56(R9), V20
	WFMSDB	V2, V3, V5, V3
	VLEG	$0, 48(R9), V18
	WFADB	V3, V5, V6
	LGDR	F3, R2
	WFMSDB	V2, V16, V6, V16
	FMOVD	40(R9), F1
	WFMADB	V2, V4, V16, V4
	FMOVD	32(R9), F7
	WFMDB	V4, V4, V3
	WFMADB	V4, V1, V20, V1
	WFMADB	V4, V7, V18, V7
	VLEG	$0, 24(R9), V16
	WFMADB	V1, V3, V7, V1
	FMOVD	16(R9), F5
	WFMADB	V4, V5, V16, V5
	RISBGZ	$57, $60, $3, R2, R4
	WFMADB	V3, V1, V5, V1
	MOVD	$·powtexp<>+0(SB), R3
	WORD	$0x68343000	//ld	%f3,0(%r4,%r3)
	FMADD	F3, F4, F4
	RISBGN	$0, $15, $48, R2, R5
	WFMADB	V4, V1, V3, V4
	LGDR	F6, R2
	LDGR	R5, F1
	SRAD	$48, R2, R2
	FMADD	F1, F4, F1
	RLL	$16, R2, R2
	ANDW	$0x7FFF0000, R2
	WORD	$0xC22B3F71	//alfi	%r2,1064370176
	BYTE	$0x00
	BYTE	$0x00
	ORW	R2, R1, R3
	MOVW	R3, R6
	CMPBLT	R6, $0, L43
L1:
	FMOVD	F1, ret+16(FP)
	RET
L43:
	LTDBR	F0, F0
	BLTU	L44
	FMOVD	F0, F3
L7:
	MOVD	$·pow_xinf<>+0(SB), R3
	FMOVD	0(R3), F5
	WFCEDBS	V3, V5, V7
	BVS	L8
	WFMDB	V3, V2, V6
L8:
	WFCEDBS	V2, V2, V3
	BVS	L9
	LTDBR	F2, F2
	BEQ	L26
	MOVW	R1, R6
	CMPBLT	R6, $0, L45
L11:
	WORD	$0xC0190003	//iilf	%r1,262143
	BYTE	$0xFF
	BYTE	$0xFF
	MOVW	R2, R7
	MOVW	R1, R6
	CMPBLE	R7, R6, L34
	RISBGNZ	$32, $63, $32, R5, R1
	LGDR	F6, R2
	MOVD	$powiadd<>+0(SB), R3
	RISBGZ	$60, $60, $4, R2, R2
	WORD	$0x5A123000	//a	%r1,0(%r2,%r3)
	RISBGN	$0, $31, $32, R1, R5
	LDGR	R5, F1
	FMADD	F1, F4, F1
	MOVD	$powxscale<>+0(SB), R1
	WORD	$0xED121000	//mdb	%f1,0(%r2,%r1)
	BYTE	$0x00
	BYTE	$0x1C
	BR	L1
L42:
	LTDBR	F0, F0
	BLTU	L46
	FMOVD	F0, F4
L3:
	MOVD	$·pow_x001a<>+0(SB), R2
	WORD	$0xED402000	//cdb	%f4,0(%r2)
	BYTE	$0x00
	BYTE	$0x19
	BGE	L2
	BVS	L2
	MOVD	$·pow_x43f<>+0(SB), R2
	WORD	$0xED402000	//mdb	%f4,0(%r2)
	BYTE	$0x00
	BYTE	$0x1C
	WORD	$0xC0298009	//iilf	%r2,2148095317
	BYTE	$0x55
	BYTE	$0x55
	LGDR	F4, R3
	RISBGNZ	$32, $63, $32, R3, R3
	SUBW	R3, R2, R3
	RISBGZ	$33, $43, $0, R3, R2
	RISBGNZ	$58, $63, $50, R3, R3
	WORD	$0xE303C000	//llgc	%r0,0(%r3,%r12)
	BYTE	$0x00
	BYTE	$0x90
	SLD	$3, R0, R3
	WORD	$0x5A234000	//a	%r2,0(%r3,%r4)
	BYTE	$0x18	//lr	%r3,%r2
	BYTE	$0x32
	RISBGN	$0, $31, $32, R3, R8
	ADDW	$0x4000000, R3
	BLEU	L5
	RISBGZ	$40, $63, $56, R3, R3
	ORW	$0x45000000, R3
	BR	L2
L9:
	WFCEDBS	V0, V0, V4
	BVS	L35
	FMOVD	F2, F1
	BR	L1
L46:
	LCDBR	F0, F4
	BR	L3
L44:
	LCDBR   F0, F3
	BR	L7
L35:
	FMOVD	F0, F1
	BR	L1
L26:
	FMOVD	8(R9), F1
	BR	L1
L34:
	FMOVD	8(R9), F4
L19:
	LTDBR	F6, F6
	BLEU	L47
L18:
	WFMDB	V4, V5, V1
	BR	L1
L5:
	RISBGZ	$33, $50, $63, R3, R3
	WORD	$0xC23B4000	//alfi	%r3,1073741824
	BYTE	$0x00
	BYTE	$0x00
	RLL	$24, R3, R3
	ORW	$0x45000000, R3
	BR	L2
L45:
	WFCEDBS	V0, V0, V4
	BVS	L35
	LTDBR	F0, F0
	BLEU	L48
	FMOVD	8(R9), F4
L12:
	MOVW	R2, R6
	CMPBLT	R6, $0, L19
	FMUL	F4, F1
	BR	L1
L47:
	BLT	L40
	WFCEDBS	V0, V0, V2
	BVS	L49
L16:
	MOVD	·pow_xnan<>+0(SB), R1
	LDGR	R1, F0
	WFMDB	V4, V0, V1
	BR	L1
L48:
	LGDR	F0, R3
	RISBGNZ	$32, $63, $32, R3, R1
	MOVW	R1, R6
	CMPBEQ	R6, $0, L29
	LTDBR	F2, F2
	BLTU	L50
	FMOVD	F2, F4
L14:
	MOVD	$·pow_x433<>+0(SB), R1
	FMOVD	0(R1), F7
	WFCHDBS	V4, V7, V3
	BEQ	L15
	WFADB	V7, V4, V3
	FSUB	F7, F3
	WFCEDBS	V4, V3, V3
	BEQ	L15
	LTDBR	F0, F0
	FMOVD	8(R9), F4
	BNE	L16
L13:
	LTDBR	F2, F2
	BLT	L18
L40:
	FMOVD	$0, F0
	WFMDB	V4, V0, V1
	BR	L1
L49:
	WFMDB	V0, V4, V1
	BR	L1
L29:
	FMOVD	8(R9), F4
	BR	L13
L15:
	MOVD	$·pow_x434<>+0(SB), R1
	FMOVD	0(R1), F7
	WFCHDBS	V4, V7, V3
	BEQ	L32
	WFADB	V7, V4, V3
	FSUB	F7, F3
	WFCEDBS	V4, V3, V4
	BEQ	L32
	FMOVD	0(R9), F4
L17:
	LTDBR	F0, F0
	BNE	L12
	BR	L13
L32:
	FMOVD	8(R9), F4
	BR	L17
L50:
	LCDBR   F2, F4
	BR	L14
xIsOne:			// Pow(1, y) = 1 for any y
yIsOne:			// Pow(x, 1) = x for any x
posInfGeZero:	// Pow(+Inf, y) = +Inf for y > 0
	MOVD	R1, ret+16(FP)
	RET
yIsNan:			//  Pow(NaN, y) = NaN
ltZeroInt:		// Pow(x, y) = NaN for finite x < 0 and finite non-integer y
	MOVD	$NaN, R2
	MOVD	R2, ret+16(FP)
	RET
negOnePosInf:	// Pow(-1, ±Inf) = 1
negOneNegInf:
	MOVD	$PosOne, R3
	MOVD	R3, ret+16(FP)
	RET
negZeroOddInt:
	MOVD	$NegInf, R3
	MOVD	R3, ret+16(FP)
	RET
zeroNotOdd:		// Pow(±0, y) = +Inf for finite y < 0 and not an odd integer
posZeroLtZero:	// special case Pow(+0, y < 0) = +Inf
zeroNegInf:		// Pow(±0, -Inf) = +Inf
	MOVD	$PosInf, R3
	MOVD	R3, ret+16(FP)
	RET
gtOnePosInf:	//Pow(x, +Inf) = +Inf for |x| > 1
ltNegOnePosInf:
	MOVD	R2, ret+16(FP)
	RET
yIsZero:		//Pow(x, ±0) = 1 for any x
	MOVD	$PosOne, R4
	MOVD	R4, ret+16(FP)
	RET
negZeroOddIntGtZero:        // Pow(-0, y) = -0 for y an odd integer > 0
	MOVD	$NegZero, R3
	MOVD	R3, ret+16(FP)
	RET
zeroNotOddGtZero:        // Pow(±0, y) = +0 for finite y > 0 and not an odd integer
	MOVD	$0, ret+16(FP)
	RET

```

// === FILE: references!/go/src/math/rand/exp.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rand

import (
	"math"
)

/*
 * Exponential distribution
 *
 * See "The Ziggurat Method for Generating Random Variables"
 * (Marsaglia & Tsang, 2000)
 * https://www.jstatsoft.org/v05/i08/paper [pdf]
 */

const (
	re = 7.69711747013104972
)

// ExpFloat64 returns an exponentially distributed float64 in the range
// (0, +[math.MaxFloat64]] with an exponential distribution whose rate parameter
// (lambda) is 1 and whose mean is 1/lambda (1).
// To produce a distribution with a different rate parameter,
// callers can adjust the output using:
//
//	sample = ExpFloat64() / desiredRateParameter
func (r *Rand) ExpFloat64() float64 {
	for {
		j := r.Uint32()
		i := j & 0xFF
		x := float64(j) * float64(we[i])
		if j < ke[i] {
			return x
		}
		if i == 0 {
			return re - math.Log(r.Float64())
		}
		if fe[i]+float32(r.Float64())*(fe[i-1]-fe[i]) < float32(math.Exp(-x)) {
			return x
		}
	}
}

var ke = [256]uint32{
	0xe290a139, 0x0, 0x9beadebc, 0xc377ac71, 0xd4ddb990,
	0xde893fb8, 0xe4a8e87c, 0xe8dff16a, 0xebf2deab, 0xee49a6e8,
	0xf0204efd, 0xf19bdb8e, 0xf2d458bb, 0xf3da104b, 0xf4b86d78,
	0xf577ad8a, 0xf61de83d, 0xf6afb784, 0xf730a573, 0xf7a37651,
	0xf80a5bb6, 0xf867189d, 0xf8bb1b4f, 0xf9079062, 0xf94d70ca,
	0xf98d8c7d, 0xf9c8928a, 0xf9ff175b, 0xfa319996, 0xfa6085f8,
	0xfa8c3a62, 0xfab5084e, 0xfadb36c8, 0xfaff0410, 0xfb20a6ea,
	0xfb404fb4, 0xfb5e2951, 0xfb7a59e9, 0xfb95038c, 0xfbae44ba,
	0xfbc638d8, 0xfbdcf892, 0xfbf29a30, 0xfc0731df, 0xfc1ad1ed,
	0xfc2d8b02, 0xfc3f6c4d, 0xfc5083ac, 0xfc60ddd1, 0xfc708662,
	0xfc7f8810, 0xfc8decb4, 0xfc9bbd62, 0xfca9027c, 0xfcb5c3c3,
	0xfcc20864, 0xfccdd70a, 0xfcd935e3, 0xfce42ab0, 0xfceebace,
	0xfcf8eb3b, 0xfd02c0a0, 0xfd0c3f59, 0xfd156b7b, 0xfd1e48d6,
	0xfd26daff, 0xfd2f2552, 0xfd372af7, 0xfd3eeee5, 0xfd4673e7,
	0xfd4dbc9e, 0xfd54cb85, 0xfd5ba2f2, 0xfd62451b, 0xfd68b415,
	0xfd6ef1da, 0xfd750047, 0xfd7ae120, 0xfd809612, 0xfd8620b4,
	0xfd8b8285, 0xfd90bcf5, 0xfd95d15e, 0xfd9ac10b, 0xfd9f8d36,
	0xfda43708, 0xfda8bf9e, 0xfdad2806, 0xfdb17141, 0xfdb59c46,
	0xfdb9a9fd, 0xfdbd9b46, 0xfdc170f6, 0xfdc52bd8, 0xfdc8ccac,
	0xfdcc542d, 0xfdcfc30b, 0xfdd319ef, 0xfdd6597a, 0xfdd98245,
	0xfddc94e5, 0xfddf91e6, 0xfde279ce, 0xfde54d1f, 0xfde80c52,
	0xfdeab7de, 0xfded5034, 0xfdefd5be, 0xfdf248e3, 0xfdf4aa06,
	0xfdf6f984, 0xfdf937b6, 0xfdfb64f4, 0xfdfd818d, 0xfdff8dd0,
	0xfe018a08, 0xfe03767a, 0xfe05536c, 0xfe07211c, 0xfe08dfc9,
	0xfe0a8fab, 0xfe0c30fb, 0xfe0dc3ec, 0xfe0f48b1, 0xfe10bf76,
	0xfe122869, 0xfe1383b4, 0xfe14d17c, 0xfe1611e7, 0xfe174516,
	0xfe186b2a, 0xfe19843e, 0xfe1a9070, 0xfe1b8fd6, 0xfe1c8289,
	0xfe1d689b, 0xfe1e4220, 0xfe1f0f26, 0xfe1fcfbc, 0xfe2083ed,
	0xfe212bc3, 0xfe21c745, 0xfe225678, 0xfe22d95f, 0xfe234ffb,
	0xfe23ba4a, 0xfe241849, 0xfe2469f2, 0xfe24af3c, 0xfe24e81e,
	0xfe25148b, 0xfe253474, 0xfe2547c7, 0xfe254e70, 0xfe25485a,
	0xfe25356a, 0xfe251586, 0xfe24e88f, 0xfe24ae64, 0xfe2466e1,
	0xfe2411df, 0xfe23af34, 0xfe233eb4, 0xfe22c02c, 0xfe22336b,
	0xfe219838, 0xfe20ee58, 0xfe20358c, 0xfe1f6d92, 0xfe1e9621,
	0xfe1daef0, 0xfe1cb7ac, 0xfe1bb002, 0xfe1a9798, 0xfe196e0d,
	0xfe1832fd, 0xfe16e5fe, 0xfe15869d, 0xfe141464, 0xfe128ed3,
	0xfe10f565, 0xfe0f478c, 0xfe0d84b1, 0xfe0bac36, 0xfe09bd73,
	0xfe07b7b5, 0xfe059a40, 0xfe03644c, 0xfe011504, 0xfdfeab88,
	0xfdfc26e9, 0xfdf98629, 0xfdf6c83b, 0xfdf3ec01, 0xfdf0f04a,
	0xfdedd3d1, 0xfdea953d, 0xfde7331e, 0xfde3abe9, 0xfddffdfb,
	0xfddc2791, 0xfdd826cd, 0xfdd3f9a8, 0xfdcf9dfc, 0xfdcb1176,
	0xfdc65198, 0xfdc15bb3, 0xfdbc2ce2, 0xfdb6c206, 0xfdb117be,
	0xfdab2a63, 0xfda4f5fd, 0xfd9e7640, 0xfd97a67a, 0xfd908192,
	0xfd8901f2, 0xfd812182, 0xfd78d98e, 0xfd7022bb, 0xfd66f4ed,
	0xfd5d4732, 0xfd530f9c, 0xfd48432b, 0xfd3cd59a, 0xfd30b936,
	0xfd23dea4, 0xfd16349e, 0xfd07a7a3, 0xfcf8219b, 0xfce7895b,
	0xfcd5c220, 0xfcc2aadb, 0xfcae1d5e, 0xfc97ed4e, 0xfc7fe6d4,
	0xfc65ccf3, 0xfc495762, 0xfc2a2fc8, 0xfc07ee19, 0xfbe213c1,
	0xfbb8051a, 0xfb890078, 0xfb5411a5, 0xfb180005, 0xfad33482,
	0xfa839276, 0xfa263b32, 0xf9b72d1c, 0xf930a1a2, 0xf889f023,
	0xf7b577d2, 0xf69c650c, 0xf51530f0, 0xf2cb0e3c, 0xeeefb15d,
	0xe6da6ecf,
}
var we = [256]float32{
	2.0249555e-09, 1.486674e-11, 2.4409617e-11, 3.1968806e-11,
	3.844677e-11, 4.4228204e-11, 4.9516443e-11, 5.443359e-11,
	5.905944e-11, 6.344942e-11, 6.7643814e-11, 7.1672945e-11,
	7.556032e-11, 7.932458e-11, 8.298079e-11, 8.654132e-11,
	9.0016515e-11, 9.3415074e-11, 9.674443e-11, 1.0001099e-10,
	1.03220314e-10, 1.06377254e-10, 1.09486115e-10, 1.1255068e-10,
	1.1557435e-10, 1.1856015e-10, 1.2151083e-10, 1.2442886e-10,
	1.2731648e-10, 1.3017575e-10, 1.3300853e-10, 1.3581657e-10,
	1.3860142e-10, 1.4136457e-10, 1.4410738e-10, 1.4683108e-10,
	1.4953687e-10, 1.5222583e-10, 1.54899e-10, 1.5755733e-10,
	1.6020171e-10, 1.6283301e-10, 1.6545203e-10, 1.6805951e-10,
	1.7065617e-10, 1.732427e-10, 1.7581973e-10, 1.7838787e-10,
	1.8094774e-10, 1.8349985e-10, 1.8604476e-10, 1.8858298e-10,
	1.9111498e-10, 1.9364126e-10, 1.9616223e-10, 1.9867835e-10,
	2.0119004e-10, 2.0369768e-10, 2.0620168e-10, 2.087024e-10,
	2.1120022e-10, 2.136955e-10, 2.1618855e-10, 2.1867974e-10,
	2.2116936e-10, 2.2365775e-10, 2.261452e-10, 2.2863202e-10,
	2.311185e-10, 2.3360494e-10, 2.360916e-10, 2.3857874e-10,
	2.4106667e-10, 2.4355562e-10, 2.4604588e-10, 2.485377e-10,
	2.5103128e-10, 2.5352695e-10, 2.560249e-10, 2.585254e-10,
	2.6102867e-10, 2.6353494e-10, 2.6604446e-10, 2.6855745e-10,
	2.7107416e-10, 2.7359479e-10, 2.761196e-10, 2.7864877e-10,
	2.8118255e-10, 2.8372119e-10, 2.8626485e-10, 2.888138e-10,
	2.9136826e-10, 2.939284e-10, 2.9649452e-10, 2.9906677e-10,
	3.016454e-10, 3.0423064e-10, 3.0682268e-10, 3.0942177e-10,
	3.1202813e-10, 3.1464195e-10, 3.1726352e-10, 3.19893e-10,
	3.2253064e-10, 3.251767e-10, 3.2783135e-10, 3.3049485e-10,
	3.3316744e-10, 3.3584938e-10, 3.3854083e-10, 3.4124212e-10,
	3.4395342e-10, 3.46675e-10, 3.4940711e-10, 3.5215003e-10,
	3.5490397e-10, 3.5766917e-10, 3.6044595e-10, 3.6323455e-10,
	3.660352e-10, 3.6884823e-10, 3.7167386e-10, 3.745124e-10,
	3.773641e-10, 3.802293e-10, 3.8310827e-10, 3.860013e-10,
	3.8890866e-10, 3.918307e-10, 3.9476775e-10, 3.9772008e-10,
	4.0068804e-10, 4.0367196e-10, 4.0667217e-10, 4.09689e-10,
	4.1272286e-10, 4.1577405e-10, 4.1884296e-10, 4.2192994e-10,
	4.250354e-10, 4.281597e-10, 4.313033e-10, 4.3446652e-10,
	4.3764986e-10, 4.408537e-10, 4.4407847e-10, 4.4732465e-10,
	4.5059267e-10, 4.5388301e-10, 4.571962e-10, 4.6053267e-10,
	4.6389292e-10, 4.6727755e-10, 4.70687e-10, 4.741219e-10,
	4.7758275e-10, 4.810702e-10, 4.845848e-10, 4.8812715e-10,
	4.9169796e-10, 4.9529775e-10, 4.989273e-10, 5.0258725e-10,
	5.0627835e-10, 5.100013e-10, 5.1375687e-10, 5.1754584e-10,
	5.21369e-10, 5.2522725e-10, 5.2912136e-10, 5.330522e-10,
	5.370208e-10, 5.4102806e-10, 5.45075e-10, 5.491625e-10,
	5.532918e-10, 5.5746385e-10, 5.616799e-10, 5.6594107e-10,
	5.7024857e-10, 5.746037e-10, 5.7900773e-10, 5.834621e-10,
	5.8796823e-10, 5.925276e-10, 5.971417e-10, 6.018122e-10,
	6.065408e-10, 6.113292e-10, 6.1617933e-10, 6.2109295e-10,
	6.260722e-10, 6.3111916e-10, 6.3623595e-10, 6.4142497e-10,
	6.4668854e-10, 6.5202926e-10, 6.5744976e-10, 6.6295286e-10,
	6.6854156e-10, 6.742188e-10, 6.79988e-10, 6.858526e-10,
	6.9181616e-10, 6.978826e-10, 7.04056e-10, 7.103407e-10,
	7.167412e-10, 7.2326256e-10, 7.2990985e-10, 7.366886e-10,
	7.4360473e-10, 7.5066453e-10, 7.5787476e-10, 7.6524265e-10,
	7.7277595e-10, 7.80483e-10, 7.883728e-10, 7.9645507e-10,
	8.047402e-10, 8.1323964e-10, 8.219657e-10, 8.309319e-10,
	8.401528e-10, 8.496445e-10, 8.594247e-10, 8.6951274e-10,
	8.799301e-10, 8.9070046e-10, 9.018503e-10, 9.134092e-10,
	9.254101e-10, 9.378904e-10, 9.508923e-10, 9.644638e-10,
	9.786603e-10, 9.935448e-10, 1.0091913e-09, 1.025686e-09,
	1.0431306e-09, 1.0616465e-09, 1.08138e-09, 1.1025096e-09,
	1.1252564e-09, 1.1498986e-09, 1.1767932e-09, 1.206409e-09,
	1.2393786e-09, 1.276585e-09, 1.3193139e-09, 1.3695435e-09,
	1.4305498e-09, 1.508365e-09, 1.6160854e-09, 1.7921248e-09,
}
var fe = [256]float32{
	1, 0.9381437, 0.90046996, 0.87170434, 0.8477855, 0.8269933,
	0.8084217, 0.7915276, 0.77595687, 0.7614634, 0.7478686,
	0.7350381, 0.72286767, 0.71127474, 0.70019263, 0.6895665,
	0.67935055, 0.6695063, 0.66000086, 0.65080583, 0.6418967,
	0.63325197, 0.6248527, 0.6166822, 0.60872537, 0.60096896,
	0.5934009, 0.58601034, 0.5787874, 0.57172304, 0.5648092,
	0.5580383, 0.5514034, 0.5448982, 0.5385169, 0.53225386,
	0.5261042, 0.52006316, 0.5141264, 0.50828975, 0.5025495,
	0.496902, 0.49134386, 0.485872, 0.48048335, 0.4751752,
	0.46994483, 0.46478975, 0.45970762, 0.45469615, 0.44975325,
	0.44487688, 0.44006512, 0.43531612, 0.43062815, 0.42599955,
	0.42142874, 0.4169142, 0.41245446, 0.40804818, 0.403694,
	0.3993907, 0.39513698, 0.39093173, 0.38677382, 0.38266218,
	0.37859577, 0.37457356, 0.37059465, 0.3666581, 0.362763,
	0.35890847, 0.35509375, 0.351318, 0.3475805, 0.34388044,
	0.34021714, 0.3365899, 0.33299807, 0.32944095, 0.32591796,
	0.3224285, 0.3189719, 0.31554767, 0.31215525, 0.30879408,
	0.3054636, 0.3021634, 0.29889292, 0.2956517, 0.29243928,
	0.28925523, 0.28609908, 0.28297043, 0.27986884, 0.27679393,
	0.2737453, 0.2707226, 0.2677254, 0.26475343, 0.26180625,
	0.25888354, 0.25598502, 0.2531103, 0.25025907, 0.24743107,
	0.24462597, 0.24184346, 0.23908329, 0.23634516, 0.23362878,
	0.23093392, 0.2282603, 0.22560766, 0.22297576, 0.22036438,
	0.21777324, 0.21520215, 0.21265087, 0.21011916, 0.20760682,
	0.20511365, 0.20263945, 0.20018397, 0.19774707, 0.19532852,
	0.19292815, 0.19054577, 0.1881812, 0.18583426, 0.18350479,
	0.1811926, 0.17889754, 0.17661946, 0.17435817, 0.17211354,
	0.1698854, 0.16767362, 0.16547804, 0.16329853, 0.16113494,
	0.15898713, 0.15685499, 0.15473837, 0.15263714, 0.15055119,
	0.14848037, 0.14642459, 0.14438373, 0.14235765, 0.14034624,
	0.13834943, 0.13636707, 0.13439907, 0.13244532, 0.13050574,
	0.1285802, 0.12666863, 0.12477092, 0.12288698, 0.12101672,
	0.119160056, 0.1173169, 0.115487166, 0.11367077, 0.11186763,
	0.11007768, 0.10830083, 0.10653701, 0.10478614, 0.10304816,
	0.101323, 0.09961058, 0.09791085, 0.09622374, 0.09454919,
	0.09288713, 0.091237515, 0.08960028, 0.087975375, 0.08636274,
	0.08476233, 0.083174095, 0.081597984, 0.08003395, 0.07848195,
	0.076941945, 0.07541389, 0.07389775, 0.072393484, 0.07090106,
	0.069420435, 0.06795159, 0.066494495, 0.06504912, 0.063615434,
	0.062193416, 0.060783047, 0.059384305, 0.057997175,
	0.05662164, 0.05525769, 0.053905312, 0.052564494, 0.051235236,
	0.049917534, 0.048611384, 0.047316793, 0.046033762, 0.0447623,
	0.043502413, 0.042254124, 0.041017443, 0.039792392,
	0.038578995, 0.037377283, 0.036187284, 0.035009038,
	0.033842582, 0.032687962, 0.031545233, 0.030414443, 0.02929566,
	0.02818895, 0.027094385, 0.026012046, 0.024942026, 0.023884421,
	0.022839336, 0.021806888, 0.020787204, 0.019780423, 0.0187867,
	0.0178062, 0.016839107, 0.015885621, 0.014945968, 0.014020392,
	0.013109165, 0.012212592, 0.011331013, 0.01046481, 0.009614414,
	0.008780315, 0.007963077, 0.0071633533, 0.006381906,
	0.0056196423, 0.0048776558, 0.004157295, 0.0034602648,
	0.0027887989, 0.0021459677, 0.0015362998, 0.0009672693,
	0.00045413437,
}

```

// === FILE: references!/go/src/math/rand/gen_cooked.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

// This program computes the value of rngCooked in rng.go,
// which is used for seeding all instances of rand.Source.
// a 64bit and a 63bit version of the array is printed to
// the standard output.

package main

import "fmt"

const (
	length = 607
	tap    = 273
	mask   = (1 << 63) - 1
	a      = 48271
	m      = (1 << 31) - 1
	q      = 44488
	r      = 3399
)

var (
	rngVec          [length]int64
	rngTap, rngFeed int
)

func seedrand(x int32) int32 {
	hi := x / q
	lo := x % q
	x = a*lo - r*hi
	if x < 0 {
		x += m
	}
	return x
}

func srand(seed int32) {
	rngTap = 0
	rngFeed = length - tap
	seed %= m
	if seed < 0 {
		seed += m
	} else if seed == 0 {
		seed = 89482311
	}
	x := seed
	for i := -20; i < length; i++ {
		x = seedrand(x)
		if i >= 0 {
			var u int64
			u = int64(x) << 20
			x = seedrand(x)
			u ^= int64(x) << 10
			x = seedrand(x)
			u ^= int64(x)
			rngVec[i] = u
		}
	}
}

func vrand() int64 {
	rngTap--
	if rngTap < 0 {
		rngTap += length
	}
	rngFeed--
	if rngFeed < 0 {
		rngFeed += length
	}
	x := (rngVec[rngFeed] + rngVec[rngTap])
	rngVec[rngFeed] = x
	return x
}

func main() {
	srand(1)
	for i := uint64(0); i < 7.8e12; i++ {
		vrand()
	}
	fmt.Printf("rngVec after 7.8e12 calls to vrand:\n%#v\n", rngVec)
	for i := range rngVec {
		rngVec[i] &= mask
	}
	fmt.Printf("lower 63bit of rngVec after 7.8e12 calls to vrand:\n%#v\n", rngVec)
}

```

// === FILE: references!/go/src/math/rand/normal.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rand

import (
	"math"
)

/*
 * Normal distribution
 *
 * See "The Ziggurat Method for Generating Random Variables"
 * (Marsaglia & Tsang, 2000)
 * http://www.jstatsoft.org/v05/i08/paper [pdf]
 */

const (
	rn = 3.442619855899
)

func absInt32(i int32) uint32 {
	if i < 0 {
		return uint32(-i)
	}
	return uint32(i)
}

// NormFloat64 returns a normally distributed float64 in
// the range -[math.MaxFloat64] through +[math.MaxFloat64] inclusive,
// with standard normal distribution (mean = 0, stddev = 1).
// To produce a different normal distribution, callers can
// adjust the output using:
//
//	sample = NormFloat64() * desiredStdDev + desiredMean
func (r *Rand) NormFloat64() float64 {
	for {
		j := int32(r.Uint32()) // Possibly negative
		i := j & 0x7F
		x := float64(j) * float64(wn[i])
		if absInt32(j) < kn[i] {
			// This case should be hit better than 99% of the time.
			return x
		}

		if i == 0 {
			// This extra work is only required for the base strip.
			for {
				x = -math.Log(r.Float64()) * (1.0 / rn)
				y := -math.Log(r.Float64())
				if y+y >= x*x {
					break
				}
			}
			if j > 0 {
				return rn + x
			}
			return -rn - x
		}
		if fn[i]+float32(r.Float64())*(fn[i-1]-fn[i]) < float32(math.Exp(-.5*x*x)) {
			return x
		}
	}
}

var kn = [128]uint32{
	0x76ad2212, 0x0, 0x600f1b53, 0x6ce447a6, 0x725b46a2,
	0x7560051d, 0x774921eb, 0x789a25bd, 0x799045c3, 0x7a4bce5d,
	0x7adf629f, 0x7b5682a6, 0x7bb8a8c6, 0x7c0ae722, 0x7c50cce7,
	0x7c8cec5b, 0x7cc12cd6, 0x7ceefed2, 0x7d177e0b, 0x7d3b8883,
	0x7d5bce6c, 0x7d78dd64, 0x7d932886, 0x7dab0e57, 0x7dc0dd30,
	0x7dd4d688, 0x7de73185, 0x7df81cea, 0x7e07c0a3, 0x7e163efa,
	0x7e23b587, 0x7e303dfd, 0x7e3beec2, 0x7e46db77, 0x7e51155d,
	0x7e5aabb3, 0x7e63abf7, 0x7e6c222c, 0x7e741906, 0x7e7b9a18,
	0x7e82adfa, 0x7e895c63, 0x7e8fac4b, 0x7e95a3fb, 0x7e9b4924,
	0x7ea0a0ef, 0x7ea5b00d, 0x7eaa7ac3, 0x7eaf04f3, 0x7eb3522a,
	0x7eb765a5, 0x7ebb4259, 0x7ebeeafd, 0x7ec2620a, 0x7ec5a9c4,
	0x7ec8c441, 0x7ecbb365, 0x7ece78ed, 0x7ed11671, 0x7ed38d62,
	0x7ed5df12, 0x7ed80cb4, 0x7eda175c, 0x7edc0005, 0x7eddc78e,
	0x7edf6ebf, 0x7ee0f647, 0x7ee25ebe, 0x7ee3a8a9, 0x7ee4d473,
	0x7ee5e276, 0x7ee6d2f5, 0x7ee7a620, 0x7ee85c10, 0x7ee8f4cd,
	0x7ee97047, 0x7ee9ce59, 0x7eea0eca, 0x7eea3147, 0x7eea3568,
	0x7eea1aab, 0x7ee9e071, 0x7ee98602, 0x7ee90a88, 0x7ee86d08,
	0x7ee7ac6a, 0x7ee6c769, 0x7ee5bc9c, 0x7ee48a67, 0x7ee32efc,
	0x7ee1a857, 0x7edff42f, 0x7ede0ffa, 0x7edbf8d9, 0x7ed9ab94,
	0x7ed7248d, 0x7ed45fae, 0x7ed1585c, 0x7ece095f, 0x7eca6ccb,
	0x7ec67be2, 0x7ec22eee, 0x7ebd7d1a, 0x7eb85c35, 0x7eb2c075,
	0x7eac9c20, 0x7ea5df27, 0x7e9e769f, 0x7e964c16, 0x7e8d44ba,
	0x7e834033, 0x7e781728, 0x7e6b9933, 0x7e5d8a1a, 0x7e4d9ded,
	0x7e3b737a, 0x7e268c2f, 0x7e0e3ff5, 0x7df1aa5d, 0x7dcf8c72,
	0x7da61a1e, 0x7d72a0fb, 0x7d30e097, 0x7cd9b4ab, 0x7c600f1a,
	0x7ba90bdc, 0x7a722176, 0x77d664e5,
}
var wn = [128]float32{
	1.7290405e-09, 1.2680929e-10, 1.6897518e-10, 1.9862688e-10,
	2.2232431e-10, 2.4244937e-10, 2.601613e-10, 2.7611988e-10,
	2.9073963e-10, 3.042997e-10, 3.1699796e-10, 3.289802e-10,
	3.4035738e-10, 3.5121603e-10, 3.616251e-10, 3.7164058e-10,
	3.8130857e-10, 3.9066758e-10, 3.9975012e-10, 4.08584e-10,
	4.1719309e-10, 4.2559822e-10, 4.338176e-10, 4.418672e-10,
	4.497613e-10, 4.5751258e-10, 4.651324e-10, 4.7263105e-10,
	4.8001775e-10, 4.87301e-10, 4.944885e-10, 5.015873e-10,
	5.0860405e-10, 5.155446e-10, 5.2241467e-10, 5.2921934e-10,
	5.359635e-10, 5.426517e-10, 5.4928817e-10, 5.5587696e-10,
	5.624219e-10, 5.6892646e-10, 5.753941e-10, 5.818282e-10,
	5.882317e-10, 5.946077e-10, 6.00959e-10, 6.072884e-10,
	6.135985e-10, 6.19892e-10, 6.2617134e-10, 6.3243905e-10,
	6.386974e-10, 6.449488e-10, 6.511956e-10, 6.5744005e-10,
	6.6368433e-10, 6.699307e-10, 6.7618144e-10, 6.824387e-10,
	6.8870465e-10, 6.949815e-10, 7.012715e-10, 7.075768e-10,
	7.1389966e-10, 7.202424e-10, 7.266073e-10, 7.329966e-10,
	7.394128e-10, 7.4585826e-10, 7.5233547e-10, 7.58847e-10,
	7.653954e-10, 7.719835e-10, 7.7861395e-10, 7.852897e-10,
	7.920138e-10, 7.987892e-10, 8.0561924e-10, 8.125073e-10,
	8.194569e-10, 8.2647167e-10, 8.3355556e-10, 8.407127e-10,
	8.479473e-10, 8.55264e-10, 8.6266755e-10, 8.7016316e-10,
	8.777562e-10, 8.8545243e-10, 8.932582e-10, 9.0117996e-10,
	9.09225e-10, 9.174008e-10, 9.2571584e-10, 9.341788e-10,
	9.427997e-10, 9.515889e-10, 9.605579e-10, 9.697193e-10,
	9.790869e-10, 9.88676e-10, 9.985036e-10, 1.0085882e-09,
	1.0189509e-09, 1.0296151e-09, 1.0406069e-09, 1.0519566e-09,
	1.063698e-09, 1.0758702e-09, 1.0885183e-09, 1.1016947e-09,
	1.1154611e-09, 1.1298902e-09, 1.1450696e-09, 1.1611052e-09,
	1.1781276e-09, 1.1962995e-09, 1.2158287e-09, 1.2369856e-09,
	1.2601323e-09, 1.2857697e-09, 1.3146202e-09, 1.347784e-09,
	1.3870636e-09, 1.4357403e-09, 1.5008659e-09, 1.6030948e-09,
}
var fn = [128]float32{
	1, 0.9635997, 0.9362827, 0.9130436, 0.89228165, 0.87324303,
	0.8555006, 0.8387836, 0.8229072, 0.8077383, 0.793177,
	0.7791461, 0.7655842, 0.7524416, 0.73967725, 0.7272569,
	0.7151515, 0.7033361, 0.69178915, 0.68049186, 0.6694277,
	0.658582, 0.6479418, 0.63749546, 0.6272325, 0.6171434,
	0.6072195, 0.5974532, 0.58783704, 0.5783647, 0.56903,
	0.5598274, 0.5507518, 0.54179835, 0.5329627, 0.52424055,
	0.5156282, 0.50712204, 0.49871865, 0.49041483, 0.48220766,
	0.4740943, 0.46607214, 0.4581387, 0.45029163, 0.44252872,
	0.43484783, 0.427247, 0.41972435, 0.41227803, 0.40490642,
	0.39760786, 0.3903808, 0.3832238, 0.37613547, 0.36911446,
	0.3621595, 0.35526937, 0.34844297, 0.34167916, 0.33497685,
	0.3283351, 0.3217529, 0.3152294, 0.30876362, 0.30235484,
	0.29600215, 0.28970486, 0.2834622, 0.2772735, 0.27113807,
	0.2650553, 0.25902456, 0.2530453, 0.24711695, 0.241239,
	0.23541094, 0.22963232, 0.2239027, 0.21822165, 0.21258877,
	0.20700371, 0.20146611, 0.19597565, 0.19053204, 0.18513499,
	0.17978427, 0.17447963, 0.1692209, 0.16400786, 0.15884037,
	0.15371831, 0.14864157, 0.14361008, 0.13862377, 0.13368265,
	0.12878671, 0.12393598, 0.119130544, 0.11437051, 0.10965602,
	0.104987256, 0.10036444, 0.095787846, 0.0912578, 0.08677467,
	0.0823389, 0.077950984, 0.073611505, 0.06932112, 0.06508058,
	0.06089077, 0.056752663, 0.0526674, 0.048636295, 0.044660863,
	0.040742867, 0.03688439, 0.033087887, 0.029356318,
	0.025693292, 0.022103304, 0.018592102, 0.015167298,
	0.011839478, 0.008624485, 0.005548995, 0.0026696292,
}

```

// === FILE: references!/go/src/math/rand/rand.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package rand implements pseudo-random number generators suitable for tasks
// such as simulation, but it should not be used for security-sensitive work.
//
// Random numbers are generated by a [Source], usually wrapped in a [Rand].
// Both types should be used by a single goroutine at a time: sharing among
// multiple goroutines requires some kind of synchronization.
//
// Top-level functions, such as [Float64] and [Int],
// are safe for concurrent use by multiple goroutines.
//
// This package's outputs might be easily predictable regardless of how it's
// seeded. For random numbers suitable for security-sensitive work, see the
// crypto/rand package.
package rand

import (
	"internal/godebug"
	"sync"
	"sync/atomic"
	_ "unsafe" // for go:linkname
)

// A Source represents a source of uniformly-distributed
// pseudo-random int64 values in the range [0, 1<<63).
//
// A Source is not safe for concurrent use by multiple goroutines.
type Source interface {
	Int63() int64
	Seed(seed int64)
}

// A Source64 is a [Source] that can also generate
// uniformly-distributed pseudo-random uint64 values in
// the range [0, 1<<64) directly.
// If a [Rand] r's underlying [Source] s implements Source64,
// then r.Uint64 returns the result of one call to s.Uint64
// instead of making two calls to s.Int63.
type Source64 interface {
	Source
	Uint64() uint64
}

// NewSource returns a new pseudo-random [Source] seeded with the given value.
// Unlike the default [Source] used by top-level functions, this source is not
// safe for concurrent use by multiple goroutines.
// The returned [Source] implements [Source64].
func NewSource(seed int64) Source {
	return newSource(seed)
}

func newSource(seed int64) *rngSource {
	var rng rngSource
	rng.Seed(seed)
	return &rng
}

// A Rand is a source of random numbers.
type Rand struct {
	src Source
	s64 Source64 // non-nil if src is source64

	// readVal contains remainder of 63-bit integer used for bytes
	// generation during most recent Read call.
	// It is saved so next Read call can start where the previous
	// one finished.
	readVal int64
	// readPos indicates the number of low-order bytes of readVal
	// that are still valid.
	readPos int8
}

// New returns a new [Rand] that uses random values from src
// to generate other random values.
func New(src Source) *Rand {
	s64, _ := src.(Source64)
	return &Rand{src: src, s64: s64}
}

// Seed uses the provided seed value to initialize the generator to a deterministic state.
// Seed should not be called concurrently with any other [Rand] method.
func (r *Rand) Seed(seed int64) {
	if lk, ok := r.src.(*lockedSource); ok {
		lk.seedPos(seed, &r.readPos)
		return
	}

	r.src.Seed(seed)
	r.readPos = 0
}

// Int63 returns a non-negative pseudo-random 63-bit integer as an int64.
func (r *Rand) Int63() int64 { return r.src.Int63() }

// Uint32 returns a pseudo-random 32-bit value as a uint32.
func (r *Rand) Uint32() uint32 { return uint32(r.Int63() >> 31) }

// Uint64 returns a pseudo-random 64-bit value as a uint64.
func (r *Rand) Uint64() uint64 {
	if r.s64 != nil {
		return r.s64.Uint64()
	}
	return uint64(r.Int63())>>31 | uint64(r.Int63())<<32
}

// Int31 returns a non-negative pseudo-random 31-bit integer as an int32.
func (r *Rand) Int31() int32 { return int32(r.Int63() >> 32) }

// Int returns a non-negative pseudo-random int.
func (r *Rand) Int() int {
	u := uint(r.Int63())
	return int(u << 1 >> 1) // clear sign bit if int == int32
}

// Int63n returns, as an int64, a non-negative pseudo-random number in the half-open interval [0,n).
// It panics if n <= 0.
func (r *Rand) Int63n(n int64) int64 {
	if n <= 0 {
		panic("invalid argument to Int63n")
	}
	if n&(n-1) == 0 { // n is power of two, can mask
		return r.Int63() & (n - 1)
	}
	max := int64((1 << 63) - 1 - (1<<63)%uint64(n))
	v := r.Int63()
	for v > max {
		v = r.Int63()
	}
	return v % n
}

// Int31n returns, as an int32, a non-negative pseudo-random number in the half-open interval [0,n).
// It panics if n <= 0.
func (r *Rand) Int31n(n int32) int32 {
	if n <= 0 {
		panic("invalid argument to Int31n")
	}
	if n&(n-1) == 0 { // n is power of two, can mask
		return r.Int31() & (n - 1)
	}
	max := int32((1 << 31) - 1 - (1<<31)%uint32(n))
	v := r.Int31()
	for v > max {
		v = r.Int31()
	}
	return v % n
}

// int31n returns, as an int32, a non-negative pseudo-random number in the half-open interval [0,n).
// n must be > 0, but int31n does not check this; the caller must ensure it.
// int31n exists because Int31n is inefficient, but Go 1 compatibility
// requires that the stream of values produced by math/rand remain unchanged.
// int31n can thus only be used internally, by newly introduced APIs.
//
// For implementation details, see:
// https://lemire.me/blog/2016/06/27/a-fast-alternative-to-the-modulo-reduction
// https://lemire.me/blog/2016/06/30/fast-random-shuffling
func (r *Rand) int31n(n int32) int32 {
	v := r.Uint32()
	prod := uint64(v) * uint64(n)
	low := uint32(prod)
	if low < uint32(n) {
		thresh := uint32(-n) % uint32(n)
		for low < thresh {
			v = r.Uint32()
			prod = uint64(v) * uint64(n)
			low = uint32(prod)
		}
	}
	return int32(prod >> 32)
}

// Intn returns, as an int, a non-negative pseudo-random number in the half-open interval [0,n).
// It panics if n <= 0.
func (r *Rand) Intn(n int) int {
	if n <= 0 {
		panic("invalid argument to Intn")
	}
	if n <= 1<<31-1 {
		return int(r.Int31n(int32(n)))
	}
	return int(r.Int63n(int64(n)))
}

// Float64 returns, as a float64, a pseudo-random number in the half-open interval [0.0,1.0).
func (r *Rand) Float64() float64 {
	// A clearer, simpler implementation would be:
	//	return float64(r.Int63n(1<<53)) / (1<<53)
	// However, Go 1 shipped with
	//	return float64(r.Int63()) / (1 << 63)
	// and we want to preserve that value stream.
	//
	// There is one bug in the value stream: r.Int63() may be so close
	// to 1<<63 that the division rounds up to 1.0, and we've guaranteed
	// that the result is always less than 1.0.
	//
	// We tried to fix this by mapping 1.0 back to 0.0, but since float64
	// values near 0 are much denser than near 1, mapping 1 to 0 caused
	// a theoretically significant overshoot in the probability of returning 0.
	// Instead of that, if we round up to 1, just try again.
	// Getting 1 only happens 1/2⁵³ of the time, so most clients
	// will not observe it anyway.
again:
	f := float64(r.Int63()) / (1 << 63)
	if f == 1 {
		goto again // resample; this branch is taken O(never)
	}
	return f
}

// Float32 returns, as a float32, a pseudo-random number in the half-open interval [0.0,1.0).
func (r *Rand) Float32() float32 {
	// Same rationale as in Float64: we want to preserve the Go 1 value
	// stream except we want to fix it not to return 1.0
	// This only happens 1/2²⁴ of the time (plus the 1/2⁵³ of the time in Float64).
again:
	f := float32(r.Float64())
	if f == 1 {
		goto again // resample; this branch is taken O(very rarely)
	}
	return f
}

// Perm returns, as a slice of n ints, a pseudo-random permutation of the integers
// in the half-open interval [0,n).
func (r *Rand) Perm(n int) []int {
	m := make([]int, n)
	// In the following loop, the iteration when i=0 always swaps m[0] with m[0].
	// A change to remove this useless iteration is to assign 1 to i in the init
	// statement. But Perm also effects r. Making this change will affect
	// the final state of r. So this change can't be made for compatibility
	// reasons for Go 1.
	for i := 0; i < n; i++ {
		j := r.Intn(i + 1)
		m[i] = m[j]
		m[j] = i
	}
	return m
}

// Shuffle pseudo-randomizes the order of elements.
// n is the number of elements. Shuffle panics if n < 0.
// swap swaps the elements with indexes i and j.
func (r *Rand) Shuffle(n int, swap func(i, j int)) {
	if n < 0 {
		panic("invalid argument to Shuffle")
	}

	// Fisher-Yates shuffle: https://en.wikipedia.org/wiki/Fisher%E2%80%93Yates_shuffle
	// Shuffle really ought not be called with n that doesn't fit in 32 bits.
	// Not only will it take a very long time, but with 2³¹! possible permutations,
	// there's no way that any PRNG can have a big enough internal state to
	// generate even a minuscule percentage of the possible permutations.
	// Nevertheless, the right API signature accepts an int n, so handle it as best we can.
	i := n - 1
	for ; i > 1<<31-1-1; i-- {
		j := int(r.Int63n(int64(i + 1)))
		swap(i, j)
	}
	for ; i > 0; i-- {
		j := int(r.int31n(int32(i + 1)))
		swap(i, j)
	}
}

// Read generates len(p) random bytes and writes them into p. It
// always returns len(p) and a nil error.
// Read should not be called concurrently with any other Rand method.
func (r *Rand) Read(p []byte) (n int, err error) {
	switch src := r.src.(type) {
	case *lockedSource:
		return src.read(p, &r.readVal, &r.readPos)
	case *runtimeSource:
		return src.read(p, &r.readVal, &r.readPos)
	}
	return read(p, r.src, &r.readVal, &r.readPos)
}

func read(p []byte, src Source, readVal *int64, readPos *int8) (n int, err error) {
	pos := *readPos
	val := *readVal
	rng, _ := src.(*rngSource)
	for n = 0; n < len(p); n++ {
		if pos == 0 {
			if rng != nil {
				val = rng.Int63()
			} else {
				val = src.Int63()
			}
			pos = 7
		}
		p[n] = byte(val)
		val >>= 8
		pos--
	}
	*readPos = pos
	*readVal = val
	return
}

/*
 * Top-level convenience functions
 */

// globalRandGenerator is the source of random numbers for the top-level
// convenience functions. When possible it uses the runtime fastrand64
// function to avoid locking. This is not possible if the user called Seed,
// either explicitly or implicitly via GODEBUG=randautoseed=0.
var globalRandGenerator atomic.Pointer[Rand]

var randautoseed = godebug.New("randautoseed")

// randseednop controls whether the global Seed is a no-op.
var randseednop = godebug.New("randseednop")

// globalRand returns the generator to use for the top-level convenience
// functions.
func globalRand() *Rand {
	if r := globalRandGenerator.Load(); r != nil {
		return r
	}

	// This is the first call. Initialize based on GODEBUG.
	var r *Rand
	if randautoseed.Value() == "0" {
		randautoseed.IncNonDefault()
		r = New(new(lockedSource))
		r.Seed(1)
	} else {
		r = &Rand{
			src: &runtimeSource{},
			s64: &runtimeSource{},
		}
	}

	if !globalRandGenerator.CompareAndSwap(nil, r) {
		// Two different goroutines called some top-level
		// function at the same time. While the results in
		// that case are unpredictable, if we just use r here,
		// and we are using a seed, we will most likely return
		// the same value for both calls. That doesn't seem ideal.
		// Just use the first one to get in.
		return globalRandGenerator.Load()
	}

	return r
}

//go:linkname runtime_rand runtime.rand
func runtime_rand() uint64

// runtimeSource is an implementation of Source64 that uses the runtime
// fastrand functions.
type runtimeSource struct {
	// The mutex is used to avoid race conditions in Read.
	mu sync.Mutex
}

func (*runtimeSource) Int63() int64 {
	return int64(runtime_rand() & rngMask)
}

func (*runtimeSource) Seed(int64) {
	panic("internal error: call to runtimeSource.Seed")
}

func (*runtimeSource) Uint64() uint64 {
	return runtime_rand()
}

func (fs *runtimeSource) read(p []byte, readVal *int64, readPos *int8) (n int, err error) {
	fs.mu.Lock()
	n, err = read(p, fs, readVal, readPos)
	fs.mu.Unlock()
	return
}

// Seed uses the provided seed value to initialize the default Source to a
// deterministic state. Seed values that have the same remainder when
// divided by 2³¹-1 generate the same pseudo-random sequence.
// Seed, unlike the [Rand.Seed] method, is safe for concurrent use.
//
// If Seed is not called, the generator is seeded randomly at program startup.
//
// Prior to Go 1.20, the generator was seeded like Seed(1) at program startup.
// To force the old behavior, call Seed(1) at program startup.
// Alternately, set GODEBUG=randautoseed=0 in the environment
// before making any calls to functions in this package.
//
// Deprecated: As of Go 1.20 there is no reason to call Seed with
// a random value. Programs that call Seed with a known value to get
// a specific sequence of results should use New(NewSource(seed)) to
// obtain a local random generator.
//
// As of Go 1.24 [Seed] is a no-op. To restore the previous behavior set
// GODEBUG=randseednop=0.
func Seed(seed int64) {
	if randseednop.Value() != "0" {
		return
	}
	randseednop.IncNonDefault()

	orig := globalRandGenerator.Load()

	// If we are already using a lockedSource, we can just re-seed it.
	if orig != nil {
		if _, ok := orig.src.(*lockedSource); ok {
			orig.Seed(seed)
			return
		}
	}

	// Otherwise either
	// 1) orig == nil, which is the normal case when Seed is the first
	// top-level function to be called, or
	// 2) orig is already a runtimeSource, in which case we need to change
	// to a lockedSource.
	// Either way we do the same thing.

	r := New(new(lockedSource))
	r.Seed(seed)

	if !globalRandGenerator.CompareAndSwap(orig, r) {
		// Something changed underfoot. Retry to be safe.
		Seed(seed)
	}
}

// Int63 returns a non-negative pseudo-random 63-bit integer as an int64
// from the default [Source].
func Int63() int64 { return globalRand().Int63() }

// Uint32 returns a pseudo-random 32-bit value as a uint32
// from the default [Source].
func Uint32() uint32 { return globalRand().Uint32() }

// Uint64 returns a pseudo-random 64-bit value as a uint64
// from the default [Source].
func Uint64() uint64 { return globalRand().Uint64() }

// Int31 returns a non-negative pseudo-random 31-bit integer as an int32
// from the default [Source].
func Int31() int32 { return globalRand().Int31() }

// Int returns a non-negative pseudo-random int from the default [Source].
func Int() int { return globalRand().Int() }

// Int63n returns, as an int64, a non-negative pseudo-random number in the half-open interval [0,n)
// from the default [Source].
// It panics if n <= 0.
func Int63n(n int64) int64 { return globalRand().Int63n(n) }

// Int31n returns, as an int32, a non-negative pseudo-random number in the half-open interval [0,n)
// from the default [Source].
// It panics if n <= 0.
func Int31n(n int32) int32 { return globalRand().Int31n(n) }

// Intn returns, as an int, a non-negative pseudo-random number in the half-open interval [0,n)
// from the default [Source].
// It panics if n <= 0.
func Intn(n int) int { return globalRand().Intn(n) }

// Float64 returns, as a float64, a pseudo-random number in the half-open interval [0.0,1.0)
// from the default [Source].
func Float64() float64 { return globalRand().Float64() }

// Float32 returns, as a float32, a pseudo-random number in the half-open interval [0.0,1.0)
// from the default [Source].
func Float32() float32 { return globalRand().Float32() }

// Perm returns, as a slice of n ints, a pseudo-random permutation of the integers
// in the half-open interval [0,n) from the default [Source].
func Perm(n int) []int { return globalRand().Perm(n) }

// Shuffle pseudo-randomizes the order of elements using the default [Source].
// n is the number of elements. Shuffle panics if n < 0.
// swap swaps the elements with indexes i and j.
func Shuffle(n int, swap func(i, j int)) { globalRand().Shuffle(n, swap) }

// Read generates len(p) random bytes from the default [Source] and
// writes them into p. It always returns len(p) and a nil error.
// Read, unlike the [Rand.Read] method, is safe for concurrent use.
//
// Deprecated: For almost all use cases, [crypto/rand.Read] is more appropriate.
// If a deterministic source is required, use [math/rand/v2.ChaCha8.Read].
func Read(p []byte) (n int, err error) { return globalRand().Read(p) }

// NormFloat64 returns a normally distributed float64 in the range
// [-[math.MaxFloat64], +[math.MaxFloat64]] with
// standard normal distribution (mean = 0, stddev = 1)
// from the default [Source].
// To produce a different normal distribution, callers can
// adjust the output using:
//
//	sample = NormFloat64() * desiredStdDev + desiredMean
func NormFloat64() float64 { return globalRand().NormFloat64() }

// ExpFloat64 returns an exponentially distributed float64 in the range
// (0, +[math.MaxFloat64]] with an exponential distribution whose rate parameter
// (lambda) is 1 and whose mean is 1/lambda (1) from the default [Source].
// To produce a distribution with a different rate parameter,
// callers can adjust the output using:
//
//	sample = ExpFloat64() / desiredRateParameter
func ExpFloat64() float64 { return globalRand().ExpFloat64() }

type lockedSource struct {
	lk sync.Mutex
	s  *rngSource
}

func (r *lockedSource) Int63() (n int64) {
	r.lk.Lock()
	n = r.s.Int63()
	r.lk.Unlock()
	return
}

func (r *lockedSource) Uint64() (n uint64) {
	r.lk.Lock()
	n = r.s.Uint64()
	r.lk.Unlock()
	return
}

func (r *lockedSource) Seed(seed int64) {
	r.lk.Lock()
	r.seed(seed)
	r.lk.Unlock()
}

// seedPos implements Seed for a lockedSource without a race condition.
func (r *lockedSource) seedPos(seed int64, readPos *int8) {
	r.lk.Lock()
	r.seed(seed)
	*readPos = 0
	r.lk.Unlock()
}

// seed seeds the underlying source.
// The caller must have locked r.lk.
func (r *lockedSource) seed(seed int64) {
	if r.s == nil {
		r.s = newSource(seed)
	} else {
		r.s.Seed(seed)
	}
}

// read implements Read for a lockedSource without a race condition.
func (r *lockedSource) read(p []byte, readVal *int64, readPos *int8) (n int, err error) {
	r.lk.Lock()
	n, err = read(p, r.s, readVal, readPos)
	r.lk.Unlock()
	return
}

```

// === FILE: references!/go/src/math/rand/rng.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rand

/*
 * Uniform distribution
 *
 * algorithm by
 * DP Mitchell and JA Reeds
 */

const (
	rngLen   = 607
	rngTap   = 273
	rngMax   = 1 << 63
	rngMask  = rngMax - 1
	int32max = (1 << 31) - 1
)

var (
	// rngCooked used for seeding. See gen_cooked.go for details.
	rngCooked [rngLen]int64 = [...]int64{
		-4181792142133755926, -4576982950128230565, 1395769623340756751, 5333664234075297259,
		-6347679516498800754, 9033628115061424579, 7143218595135194537, 4812947590706362721,
		7937252194349799378, 5307299880338848416, 8209348851763925077, -7107630437535961764,
		4593015457530856296, 8140875735541888011, -5903942795589686782, -603556388664454774,
		-7496297993371156308, 113108499721038619, 4569519971459345583, -4160538177779461077,
		-6835753265595711384, -6507240692498089696, 6559392774825876886, 7650093201692370310,
		7684323884043752161, -8965504200858744418, -2629915517445760644, 271327514973697897,
		-6433985589514657524, 1065192797246149621, 3344507881999356393, -4763574095074709175,
		7465081662728599889, 1014950805555097187, -4773931307508785033, -5742262670416273165,
		2418672789110888383, 5796562887576294778, 4484266064449540171, 3738982361971787048,
		-4699774852342421385, 10530508058128498, -589538253572429690, -6598062107225984180,
		8660405965245884302, 10162832508971942, -2682657355892958417, 7031802312784620857,
		6240911277345944669, 831864355460801054, -1218937899312622917, 2116287251661052151,
		2202309800992166967, 9161020366945053561, 4069299552407763864, 4936383537992622449,
		457351505131524928, -8881176990926596454, -6375600354038175299, -7155351920868399290,
		4368649989588021065, 887231587095185257, -3659780529968199312, -2407146836602825512,
		5616972787034086048, -751562733459939242, 1686575021641186857, -5177887698780513806,
		-4979215821652996885, -1375154703071198421, 5632136521049761902, -8390088894796940536,
		-193645528485698615, -5979788902190688516, -4907000935050298721, -285522056888777828,
		-2776431630044341707, 1679342092332374735, 6050638460742422078, -2229851317345194226,
		-1582494184340482199, 5881353426285907985, 812786550756860885, 4541845584483343330,
		-6497901820577766722, 4980675660146853729, -4012602956251539747, -329088717864244987,
		-2896929232104691526, 1495812843684243920, -2153620458055647789, 7370257291860230865,
		-2466442761497833547, 4706794511633873654, -1398851569026877145, 8549875090542453214,
		-9189721207376179652, -7894453601103453165, 7297902601803624459, 1011190183918857495,
		-6985347000036920864, 5147159997473910359, -8326859945294252826, 2659470849286379941,
		6097729358393448602, -7491646050550022124, -5117116194870963097, -896216826133240300,
		-745860416168701406, 5803876044675762232, -787954255994554146, -3234519180203704564,
		-4507534739750823898, -1657200065590290694, 505808562678895611, -4153273856159712438,
		-8381261370078904295, 572156825025677802, 1791881013492340891, 3393267094866038768,
		-5444650186382539299, 2352769483186201278, -7930912453007408350, -325464993179687389,
		-3441562999710612272, -6489413242825283295, 5092019688680754699, -227247482082248967,
		4234737173186232084, 5027558287275472836, 4635198586344772304, -536033143587636457,
		5907508150730407386, -8438615781380831356, 972392927514829904, -3801314342046600696,
		-4064951393885491917, -174840358296132583, 2407211146698877100, -1640089820333676239,
		3940796514530962282, -5882197405809569433, 3095313889586102949, -1818050141166537098,
		5832080132947175283, 7890064875145919662, 8184139210799583195, -8073512175445549678,
		-7758774793014564506, -4581724029666783935, 3516491885471466898, -8267083515063118116,
		6657089965014657519, 5220884358887979358, 1796677326474620641, 5340761970648932916,
		1147977171614181568, 5066037465548252321, 2574765911837859848, 1085848279845204775,
		-5873264506986385449, 6116438694366558490, 2107701075971293812, -7420077970933506541,
		2469478054175558874, -1855128755834809824, -5431463669011098282, -9038325065738319171,
		-6966276280341336160, 7217693971077460129, -8314322083775271549, 7196649268545224266,
		-3585711691453906209, -5267827091426810625, 8057528650917418961, -5084103596553648165,
		-2601445448341207749, -7850010900052094367, 6527366231383600011, 3507654575162700890,
		9202058512774729859, 1954818376891585542, -2582991129724600103, 8299563319178235687,
		-5321504681635821435, 7046310742295574065, -2376176645520785576, -7650733936335907755,
		8850422670118399721, 3631909142291992901, 5158881091950831288, -6340413719511654215,
		4763258931815816403, 6280052734341785344, -4979582628649810958, 2043464728020827976,
		-2678071570832690343, 4562580375758598164, 5495451168795427352, -7485059175264624713,
		553004618757816492, 6895160632757959823, -989748114590090637, 7139506338801360852,
		-672480814466784139, 5535668688139305547, 2430933853350256242, -3821430778991574732,
		-1063731997747047009, -3065878205254005442, 7632066283658143750, 6308328381617103346,
		3681878764086140361, 3289686137190109749, 6587997200611086848, 244714774258135476,
		-5143583659437639708, 8090302575944624335, 2945117363431356361, -8359047641006034763,
		3009039260312620700, -793344576772241777, 401084700045993341, -1968749590416080887,
		4707864159563588614, -3583123505891281857, -3240864324164777915, -5908273794572565703,
		-3719524458082857382, -5281400669679581926, 8118566580304798074, 3839261274019871296,
		7062410411742090847, -8481991033874568140, 6027994129690250817, -6725542042704711878,
		-2971981702428546974, -7854441788951256975, 8809096399316380241, 6492004350391900708,
		2462145737463489636, -8818543617934476634, -5070345602623085213, -8961586321599299868,
		-3758656652254704451, -8630661632476012791, 6764129236657751224, -709716318315418359,
		-3403028373052861600, -8838073512170985897, -3999237033416576341, -2920240395515973663,
		-2073249475545404416, 368107899140673753, -6108185202296464250, -6307735683270494757,
		4782583894627718279, 6718292300699989587, 8387085186914375220, 3387513132024756289,
		4654329375432538231, -292704475491394206, -3848998599978456535, 7623042350483453954,
		7725442901813263321, 9186225467561587250, -5132344747257272453, -6865740430362196008,
		2530936820058611833, 1636551876240043639, -3658707362519810009, 1452244145334316253,
		-7161729655835084979, -7943791770359481772, 9108481583171221009, -3200093350120725999,
		5007630032676973346, 2153168792952589781, 6720334534964750538, -3181825545719981703,
		3433922409283786309, 2285479922797300912, 3110614940896576130, -2856812446131932915,
		-3804580617188639299, 7163298419643543757, 4891138053923696990, 580618510277907015,
		1684034065251686769, 4429514767357295841, -8893025458299325803, -8103734041042601133,
		7177515271653460134, 4589042248470800257, -1530083407795771245, 143607045258444228,
		246994305896273627, -8356954712051676521, 6473547110565816071, 3092379936208876896,
		2058427839513754051, -4089587328327907870, 8785882556301281247, -3074039370013608197,
		-637529855400303673, 6137678347805511274, -7152924852417805802, 5708223427705576541,
		-3223714144396531304, 4358391411789012426, 325123008708389849, 6837621693887290924,
		4843721905315627004, -3212720814705499393, -3825019837890901156, 4602025990114250980,
		1044646352569048800, 9106614159853161675, -8394115921626182539, -4304087667751778808,
		2681532557646850893, 3681559472488511871, -3915372517896561773, -2889241648411946534,
		-6564663803938238204, -8060058171802589521, 581945337509520675, 3648778920718647903,
		-4799698790548231394, -7602572252857820065, 220828013409515943, -1072987336855386047,
		4287360518296753003, -4633371852008891965, 5513660857261085186, -2258542936462001533,
		-8744380348503999773, 8746140185685648781, 228500091334420247, 1356187007457302238,
		3019253992034194581, 3152601605678500003, -8793219284148773595, 5559581553696971176,
		4916432985369275664, -8559797105120221417, -5802598197927043732, 2868348622579915573,
		-7224052902810357288, -5894682518218493085, 2587672709781371173, -7706116723325376475,
		3092343956317362483, -5561119517847711700, 972445599196498113, -1558506600978816441,
		1708913533482282562, -2305554874185907314, -6005743014309462908, -6653329009633068701,
		-483583197311151195, 2488075924621352812, -4529369641467339140, -4663743555056261452,
		2997203966153298104, 1282559373026354493, 240113143146674385, 8665713329246516443,
		628141331766346752, -4651421219668005332, -7750560848702540400, 7596648026010355826,
		-3132152619100351065, 7834161864828164065, 7103445518877254909, 4390861237357459201,
		-4780718172614204074, -319889632007444440, 622261699494173647, -3186110786557562560,
		-8718967088789066690, -1948156510637662747, -8212195255998774408, -7028621931231314745,
		2623071828615234808, -4066058308780939700, -5484966924888173764, -6683604512778046238,
		-6756087640505506466, 5256026990536851868, 7841086888628396109, 6640857538655893162,
		-8021284697816458310, -7109857044414059830, -1689021141511844405, -4298087301956291063,
		-4077748265377282003, -998231156719803476, 2719520354384050532, 9132346697815513771,
		4332154495710163773, -2085582442760428892, 6994721091344268833, -2556143461985726874,
		-8567931991128098309, 59934747298466858, -3098398008776739403, -265597256199410390,
		2332206071942466437, -7522315324568406181, 3154897383618636503, -7585605855467168281,
		-6762850759087199275, 197309393502684135, -8579694182469508493, 2543179307861934850,
		4350769010207485119, -4468719947444108136, -7207776534213261296, -1224312577878317200,
		4287946071480840813, 8362686366770308971, 6486469209321732151, -5605644191012979782,
		-1669018511020473564, 4450022655153542367, -7618176296641240059, -3896357471549267421,
		-4596796223304447488, -6531150016257070659, -8982326463137525940, -4125325062227681798,
		-1306489741394045544, -8338554946557245229, 5329160409530630596, 7790979528857726136,
		4955070238059373407, -4304834761432101506, -6215295852904371179, 3007769226071157901,
		-6753025801236972788, 8928702772696731736, 7856187920214445904, -4748497451462800923,
		7900176660600710914, -7082800908938549136, -6797926979589575837, -6737316883512927978,
		4186670094382025798, 1883939007446035042, -414705992779907823, 3734134241178479257,
		4065968871360089196, 6953124200385847784, -7917685222115876751, -7585632937840318161,
		-5567246375906782599, -5256612402221608788, 3106378204088556331, -2894472214076325998,
		4565385105440252958, 1979884289539493806, -6891578849933910383, 3783206694208922581,
		8464961209802336085, 2843963751609577687, 3030678195484896323, -4429654462759003204,
		4459239494808162889, 402587895800087237, 8057891408711167515, 4541888170938985079,
		1042662272908816815, -3666068979732206850, 2647678726283249984, 2144477441549833761,
		-3417019821499388721, -2105601033380872185, 5916597177708541638, -8760774321402454447,
		8833658097025758785, 5970273481425315300, 563813119381731307, -6455022486202078793,
		1598828206250873866, -4016978389451217698, -2988328551145513985, -6071154634840136312,
		8469693267274066490, 125672920241807416, -3912292412830714870, -2559617104544284221,
		-486523741806024092, -4735332261862713930, 5923302823487327109, -9082480245771672572,
		-1808429243461201518, 7990420780896957397, 4317817392807076702, 3625184369705367340,
		-6482649271566653105, -3480272027152017464, -3225473396345736649, -368878695502291645,
		-3981164001421868007, -8522033136963788610, 7609280429197514109, 3020985755112334161,
		-2572049329799262942, 2635195723621160615, 5144520864246028816, -8188285521126945980,
		1567242097116389047, 8172389260191636581, -2885551685425483535, -7060359469858316883,
		-6480181133964513127, -7317004403633452381, 6011544915663598137, 5932255307352610768,
		2241128460406315459, -8327867140638080220, 3094483003111372717, 4583857460292963101,
		9079887171656594975, -384082854924064405, -3460631649611717935, 4225072055348026230,
		-7385151438465742745, 3801620336801580414, -399845416774701952, -7446754431269675473,
		7899055018877642622, 5421679761463003041, 5521102963086275121, -4975092593295409910,
		8735487530905098534, -7462844945281082830, -2080886987197029914, -1000715163927557685,
		-4253840471931071485, -5828896094657903328, 6424174453260338141, 359248545074932887,
		-5949720754023045210, -2426265837057637212, 3030918217665093212, -9077771202237461772,
		-3186796180789149575, 740416251634527158, -2142944401404840226, 6951781370868335478,
		399922722363687927, -8928469722407522623, -1378421100515597285, -8343051178220066766,
		-3030716356046100229, -8811767350470065420, 9026808440365124461, 6440783557497587732,
		4615674634722404292, 539897290441580544, 2096238225866883852, 8751955639408182687,
		-7316147128802486205, 7381039757301768559, 6157238513393239656, -1473377804940618233,
		8629571604380892756, 5280433031239081479, 7101611890139813254, 2479018537985767835,
		7169176924412769570, -1281305539061572506, -7865612307799218120, 2278447439451174845,
		3625338785743880657, 6477479539006708521, 8976185375579272206, -3712000482142939688,
		1326024180520890843, 7537449876596048829, 5464680203499696154, 3189671183162196045,
		6346751753565857109, -8982212049534145501, -6127578587196093755, -245039190118465649,
		-6320577374581628592, 7208698530190629697, 7276901792339343736, -7490986807540332668,
		4133292154170828382, 2918308698224194548, -7703910638917631350, -3929437324238184044,
		-4300543082831323144, -6344160503358350167, 5896236396443472108, -758328221503023383,
		-1894351639983151068, -307900319840287220, -6278469401177312761, -2171292963361310674,
		8382142935188824023, 9103922860780351547, 4152330101494654406,
	}
)

type rngSource struct {
	tap  int           // index into vec
	feed int           // index into vec
	vec  [rngLen]int64 // current feedback register
}

// seed rng x[n+1] = 48271 * x[n] mod (2**31 - 1)
func seedrand(x int32) int32 {
	const (
		A = 48271
		Q = 44488
		R = 3399
	)

	hi := x / Q
	lo := x % Q
	x = A*lo - R*hi
	if x < 0 {
		x += int32max
	}
	return x
}

// Seed uses the provided seed value to initialize the generator to a deterministic state.
func (rng *rngSource) Seed(seed int64) {
	rng.tap = 0
	rng.feed = rngLen - rngTap

	seed = seed % int32max
	if seed < 0 {
		seed += int32max
	}
	if seed == 0 {
		seed = 89482311
	}

	x := int32(seed)
	for i := -20; i < rngLen; i++ {
		x = seedrand(x)
		if i >= 0 {
			var u int64
			u = int64(x) << 40
			x = seedrand(x)
			u ^= int64(x) << 20
			x = seedrand(x)
			u ^= int64(x)
			u ^= rngCooked[i]
			rng.vec[i] = u
		}
	}
}

// Int63 returns a non-negative pseudo-random 63-bit integer as an int64.
func (rng *rngSource) Int63() int64 {
	return int64(rng.Uint64() & rngMask)
}

// Uint64 returns a non-negative pseudo-random 64-bit integer as a uint64.
func (rng *rngSource) Uint64() uint64 {
	rng.tap--
	if rng.tap < 0 {
		rng.tap += rngLen
	}

	rng.feed--
	if rng.feed < 0 {
		rng.feed += rngLen
	}

	x := rng.vec[rng.feed] + rng.vec[rng.tap]
	rng.vec[rng.feed] = x
	return uint64(x)
}

```

// === FILE: references!/go/src/math/rand/v2/chacha8.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rand

import (
	"errors"
	"internal/byteorder"
	"internal/chacha8rand"
)

// A ChaCha8 is a ChaCha8-based cryptographically strong
// random number generator.
type ChaCha8 struct {
	state chacha8rand.State

	// The last readLen bytes of readBuf are still to be consumed by Read.
	readBuf [8]byte
	readLen int // 0 <= readLen <= 8
}

// NewChaCha8 returns a new ChaCha8 seeded with the given seed.
func NewChaCha8(seed [32]byte) *ChaCha8 {
	c := new(ChaCha8)
	c.state.Init(seed)
	return c
}

// Seed resets the ChaCha8 to behave the same way as NewChaCha8(seed).
func (c *ChaCha8) Seed(seed [32]byte) {
	c.state.Init(seed)
	c.readLen = 0
	c.readBuf = [8]byte{}
}

// Uint64 returns a uniformly distributed random uint64 value.
func (c *ChaCha8) Uint64() uint64 {
	for {
		x, ok := c.state.Next()
		if ok {
			return x
		}
		c.state.Refill()
	}
}

// Read reads exactly len(p) bytes into p.
// It always returns len(p) and a nil error.
//
// If calls to Read and Uint64 are interleaved, the order in which bits are
// returned by the two is undefined, and Read may return bits generated before
// the last call to Uint64.
func (c *ChaCha8) Read(p []byte) (n int, err error) {
	if c.readLen > 0 {
		n = copy(p, c.readBuf[len(c.readBuf)-c.readLen:])
		c.readLen -= n
		p = p[n:]
	}
	for len(p) >= 8 {
		byteorder.LEPutUint64(p, c.Uint64())
		p = p[8:]
		n += 8
	}
	if len(p) > 0 {
		byteorder.LEPutUint64(c.readBuf[:], c.Uint64())
		n += copy(p, c.readBuf[:])
		c.readLen = 8 - len(p)
	}
	return
}

// UnmarshalBinary implements the [encoding.BinaryUnmarshaler] interface.
func (c *ChaCha8) UnmarshalBinary(data []byte) error {
	data, ok := cutPrefix(data, []byte("readbuf:"))
	if ok {
		var buf []byte
		buf, data, ok = readUint8LengthPrefixed(data)
		if !ok {
			return errors.New("invalid ChaCha8 Read buffer encoding")
		}
		c.readLen = copy(c.readBuf[len(c.readBuf)-len(buf):], buf)
	}
	return chacha8rand.Unmarshal(&c.state, data)
}

func cutPrefix(s, prefix []byte) (after []byte, found bool) {
	if len(s) < len(prefix) || string(s[:len(prefix)]) != string(prefix) {
		return s, false
	}
	return s[len(prefix):], true
}

func readUint8LengthPrefixed(b []byte) (buf, rest []byte, ok bool) {
	if len(b) == 0 || len(b) < int(1+b[0]) {
		return nil, nil, false
	}
	return b[1 : 1+b[0]], b[1+b[0]:], true
}

// AppendBinary implements the [encoding.BinaryAppender] interface.
func (c *ChaCha8) AppendBinary(b []byte) ([]byte, error) {
	if c.readLen > 0 {
		b = append(b, "readbuf:"...)
		b = append(b, uint8(c.readLen))
		b = append(b, c.readBuf[len(c.readBuf)-c.readLen:]...)
	}
	return append(b, chacha8rand.Marshal(&c.state)...), nil
}

// MarshalBinary implements the [encoding.BinaryMarshaler] interface.
func (c *ChaCha8) MarshalBinary() ([]byte, error) {
	// the maximum length of (chacha8rand.Marshal + c.readBuf + "readbuf:") is 64
	return c.AppendBinary(make([]byte, 0, 64))
}

```

// === FILE: references!/go/src/math/rand/v2/exp.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rand

import (
	"math"
)

/*
 * Exponential distribution
 *
 * See "The Ziggurat Method for Generating Random Variables"
 * (Marsaglia & Tsang, 2000)
 * https://www.jstatsoft.org/v05/i08/paper [pdf]
 */

const (
	re = 7.69711747013104972
)

// ExpFloat64 returns an exponentially distributed float64 in the range
// (0, +math.MaxFloat64] with an exponential distribution whose rate parameter
// (lambda) is 1 and whose mean is 1/lambda (1).
// To produce a distribution with a different rate parameter,
// callers can adjust the output using:
//
//	sample = ExpFloat64() / desiredRateParameter
func (r *Rand) ExpFloat64() float64 {
	for {
		u := r.Uint64()
		j := uint32(u)
		i := uint8(u >> 32)
		x := float64(j) * float64(we[i])
		if j < ke[i] {
			return x
		}
		if i == 0 {
			return re - math.Log(r.Float64())
		}
		if fe[i]+float32(r.Float64())*(fe[i-1]-fe[i]) < float32(math.Exp(-x)) {
			return x
		}
	}
}

var ke = [256]uint32{
	0xe290a139, 0x0, 0x9beadebc, 0xc377ac71, 0xd4ddb990,
	0xde893fb8, 0xe4a8e87c, 0xe8dff16a, 0xebf2deab, 0xee49a6e8,
	0xf0204efd, 0xf19bdb8e, 0xf2d458bb, 0xf3da104b, 0xf4b86d78,
	0xf577ad8a, 0xf61de83d, 0xf6afb784, 0xf730a573, 0xf7a37651,
	0xf80a5bb6, 0xf867189d, 0xf8bb1b4f, 0xf9079062, 0xf94d70ca,
	0xf98d8c7d, 0xf9c8928a, 0xf9ff175b, 0xfa319996, 0xfa6085f8,
	0xfa8c3a62, 0xfab5084e, 0xfadb36c8, 0xfaff0410, 0xfb20a6ea,
	0xfb404fb4, 0xfb5e2951, 0xfb7a59e9, 0xfb95038c, 0xfbae44ba,
	0xfbc638d8, 0xfbdcf892, 0xfbf29a30, 0xfc0731df, 0xfc1ad1ed,
	0xfc2d8b02, 0xfc3f6c4d, 0xfc5083ac, 0xfc60ddd1, 0xfc708662,
	0xfc7f8810, 0xfc8decb4, 0xfc9bbd62, 0xfca9027c, 0xfcb5c3c3,
	0xfcc20864, 0xfccdd70a, 0xfcd935e3, 0xfce42ab0, 0xfceebace,
	0xfcf8eb3b, 0xfd02c0a0, 0xfd0c3f59, 0xfd156b7b, 0xfd1e48d6,
	0xfd26daff, 0xfd2f2552, 0xfd372af7, 0xfd3eeee5, 0xfd4673e7,
	0xfd4dbc9e, 0xfd54cb85, 0xfd5ba2f2, 0xfd62451b, 0xfd68b415,
	0xfd6ef1da, 0xfd750047, 0xfd7ae120, 0xfd809612, 0xfd8620b4,
	0xfd8b8285, 0xfd90bcf5, 0xfd95d15e, 0xfd9ac10b, 0xfd9f8d36,
	0xfda43708, 0xfda8bf9e, 0xfdad2806, 0xfdb17141, 0xfdb59c46,
	0xfdb9a9fd, 0xfdbd9b46, 0xfdc170f6, 0xfdc52bd8, 0xfdc8ccac,
	0xfdcc542d, 0xfdcfc30b, 0xfdd319ef, 0xfdd6597a, 0xfdd98245,
	0xfddc94e5, 0xfddf91e6, 0xfde279ce, 0xfde54d1f, 0xfde80c52,
	0xfdeab7de, 0xfded5034, 0xfdefd5be, 0xfdf248e3, 0xfdf4aa06,
	0xfdf6f984, 0xfdf937b6, 0xfdfb64f4, 0xfdfd818d, 0xfdff8dd0,
	0xfe018a08, 0xfe03767a, 0xfe05536c, 0xfe07211c, 0xfe08dfc9,
	0xfe0a8fab, 0xfe0c30fb, 0xfe0dc3ec, 0xfe0f48b1, 0xfe10bf76,
	0xfe122869, 0xfe1383b4, 0xfe14d17c, 0xfe1611e7, 0xfe174516,
	0xfe186b2a, 0xfe19843e, 0xfe1a9070, 0xfe1b8fd6, 0xfe1c8289,
	0xfe1d689b, 0xfe1e4220, 0xfe1f0f26, 0xfe1fcfbc, 0xfe2083ed,
	0xfe212bc3, 0xfe21c745, 0xfe225678, 0xfe22d95f, 0xfe234ffb,
	0xfe23ba4a, 0xfe241849, 0xfe2469f2, 0xfe24af3c, 0xfe24e81e,
	0xfe25148b, 0xfe253474, 0xfe2547c7, 0xfe254e70, 0xfe25485a,
	0xfe25356a, 0xfe251586, 0xfe24e88f, 0xfe24ae64, 0xfe2466e1,
	0xfe2411df, 0xfe23af34, 0xfe233eb4, 0xfe22c02c, 0xfe22336b,
	0xfe219838, 0xfe20ee58, 0xfe20358c, 0xfe1f6d92, 0xfe1e9621,
	0xfe1daef0, 0xfe1cb7ac, 0xfe1bb002, 0xfe1a9798, 0xfe196e0d,
	0xfe1832fd, 0xfe16e5fe, 0xfe15869d, 0xfe141464, 0xfe128ed3,
	0xfe10f565, 0xfe0f478c, 0xfe0d84b1, 0xfe0bac36, 0xfe09bd73,
	0xfe07b7b5, 0xfe059a40, 0xfe03644c, 0xfe011504, 0xfdfeab88,
	0xfdfc26e9, 0xfdf98629, 0xfdf6c83b, 0xfdf3ec01, 0xfdf0f04a,
	0xfdedd3d1, 0xfdea953d, 0xfde7331e, 0xfde3abe9, 0xfddffdfb,
	0xfddc2791, 0xfdd826cd, 0xfdd3f9a8, 0xfdcf9dfc, 0xfdcb1176,
	0xfdc65198, 0xfdc15bb3, 0xfdbc2ce2, 0xfdb6c206, 0xfdb117be,
	0xfdab2a63, 0xfda4f5fd, 0xfd9e7640, 0xfd97a67a, 0xfd908192,
	0xfd8901f2, 0xfd812182, 0xfd78d98e, 0xfd7022bb, 0xfd66f4ed,
	0xfd5d4732, 0xfd530f9c, 0xfd48432b, 0xfd3cd59a, 0xfd30b936,
	0xfd23dea4, 0xfd16349e, 0xfd07a7a3, 0xfcf8219b, 0xfce7895b,
	0xfcd5c220, 0xfcc2aadb, 0xfcae1d5e, 0xfc97ed4e, 0xfc7fe6d4,
	0xfc65ccf3, 0xfc495762, 0xfc2a2fc8, 0xfc07ee19, 0xfbe213c1,
	0xfbb8051a, 0xfb890078, 0xfb5411a5, 0xfb180005, 0xfad33482,
	0xfa839276, 0xfa263b32, 0xf9b72d1c, 0xf930a1a2, 0xf889f023,
	0xf7b577d2, 0xf69c650c, 0xf51530f0, 0xf2cb0e3c, 0xeeefb15d,
	0xe6da6ecf,
}
var we = [256]float32{
	2.0249555e-09, 1.486674e-11, 2.4409617e-11, 3.1968806e-11,
	3.844677e-11, 4.4228204e-11, 4.9516443e-11, 5.443359e-11,
	5.905944e-11, 6.344942e-11, 6.7643814e-11, 7.1672945e-11,
	7.556032e-11, 7.932458e-11, 8.298079e-11, 8.654132e-11,
	9.0016515e-11, 9.3415074e-11, 9.674443e-11, 1.0001099e-10,
	1.03220314e-10, 1.06377254e-10, 1.09486115e-10, 1.1255068e-10,
	1.1557435e-10, 1.1856015e-10, 1.2151083e-10, 1.2442886e-10,
	1.2731648e-10, 1.3017575e-10, 1.3300853e-10, 1.3581657e-10,
	1.3860142e-10, 1.4136457e-10, 1.4410738e-10, 1.4683108e-10,
	1.4953687e-10, 1.5222583e-10, 1.54899e-10, 1.5755733e-10,
	1.6020171e-10, 1.6283301e-10, 1.6545203e-10, 1.6805951e-10,
	1.7065617e-10, 1.732427e-10, 1.7581973e-10, 1.7838787e-10,
	1.8094774e-10, 1.8349985e-10, 1.8604476e-10, 1.8858298e-10,
	1.9111498e-10, 1.9364126e-10, 1.9616223e-10, 1.9867835e-10,
	2.0119004e-10, 2.0369768e-10, 2.0620168e-10, 2.087024e-10,
	2.1120022e-10, 2.136955e-10, 2.1618855e-10, 2.1867974e-10,
	2.2116936e-10, 2.2365775e-10, 2.261452e-10, 2.2863202e-10,
	2.311185e-10, 2.3360494e-10, 2.360916e-10, 2.3857874e-10,
	2.4106667e-10, 2.4355562e-10, 2.4604588e-10, 2.485377e-10,
	2.5103128e-10, 2.5352695e-10, 2.560249e-10, 2.585254e-10,
	2.6102867e-10, 2.6353494e-10, 2.6604446e-10, 2.6855745e-10,
	2.7107416e-10, 2.7359479e-10, 2.761196e-10, 2.7864877e-10,
	2.8118255e-10, 2.8372119e-10, 2.8626485e-10, 2.888138e-10,
	2.9136826e-10, 2.939284e-10, 2.9649452e-10, 2.9906677e-10,
	3.016454e-10, 3.0423064e-10, 3.0682268e-10, 3.0942177e-10,
	3.1202813e-10, 3.1464195e-10, 3.1726352e-10, 3.19893e-10,
	3.2253064e-10, 3.251767e-10, 3.2783135e-10, 3.3049485e-10,
	3.3316744e-10, 3.3584938e-10, 3.3854083e-10, 3.4124212e-10,
	3.4395342e-10, 3.46675e-10, 3.4940711e-10, 3.5215003e-10,
	3.5490397e-10, 3.5766917e-10, 3.6044595e-10, 3.6323455e-10,
	3.660352e-10, 3.6884823e-10, 3.7167386e-10, 3.745124e-10,
	3.773641e-10, 3.802293e-10, 3.8310827e-10, 3.860013e-10,
	3.8890866e-10, 3.918307e-10, 3.9476775e-10, 3.9772008e-10,
	4.0068804e-10, 4.0367196e-10, 4.0667217e-10, 4.09689e-10,
	4.1272286e-10, 4.1577405e-10, 4.1884296e-10, 4.2192994e-10,
	4.250354e-10, 4.281597e-10, 4.313033e-10, 4.3446652e-10,
	4.3764986e-10, 4.408537e-10, 4.4407847e-10, 4.4732465e-10,
	4.5059267e-10, 4.5388301e-10, 4.571962e-10, 4.6053267e-10,
	4.6389292e-10, 4.6727755e-10, 4.70687e-10, 4.741219e-10,
	4.7758275e-10, 4.810702e-10, 4.845848e-10, 4.8812715e-10,
	4.9169796e-10, 4.9529775e-10, 4.989273e-10, 5.0258725e-10,
	5.0627835e-10, 5.100013e-10, 5.1375687e-10, 5.1754584e-10,
	5.21369e-10, 5.2522725e-10, 5.2912136e-10, 5.330522e-10,
	5.370208e-10, 5.4102806e-10, 5.45075e-10, 5.491625e-10,
	5.532918e-10, 5.5746385e-10, 5.616799e-10, 5.6594107e-10,
	5.7024857e-10, 5.746037e-10, 5.7900773e-10, 5.834621e-10,
	5.8796823e-10, 5.925276e-10, 5.971417e-10, 6.018122e-10,
	6.065408e-10, 6.113292e-10, 6.1617933e-10, 6.2109295e-10,
	6.260722e-10, 6.3111916e-10, 6.3623595e-10, 6.4142497e-10,
	6.4668854e-10, 6.5202926e-10, 6.5744976e-10, 6.6295286e-10,
	6.6854156e-10, 6.742188e-10, 6.79988e-10, 6.858526e-10,
	6.9181616e-10, 6.978826e-10, 7.04056e-10, 7.103407e-10,
	7.167412e-10, 7.2326256e-10, 7.2990985e-10, 7.366886e-10,
	7.4360473e-10, 7.5066453e-10, 7.5787476e-10, 7.6524265e-10,
	7.7277595e-10, 7.80483e-10, 7.883728e-10, 7.9645507e-10,
	8.047402e-10, 8.1323964e-10, 8.219657e-10, 8.309319e-10,
	8.401528e-10, 8.496445e-10, 8.594247e-10, 8.6951274e-10,
	8.799301e-10, 8.9070046e-10, 9.018503e-10, 9.134092e-10,
	9.254101e-10, 9.378904e-10, 9.508923e-10, 9.644638e-10,
	9.786603e-10, 9.935448e-10, 1.0091913e-09, 1.025686e-09,
	1.0431306e-09, 1.0616465e-09, 1.08138e-09, 1.1025096e-09,
	1.1252564e-09, 1.1498986e-09, 1.1767932e-09, 1.206409e-09,
	1.2393786e-09, 1.276585e-09, 1.3193139e-09, 1.3695435e-09,
	1.4305498e-09, 1.508365e-09, 1.6160854e-09, 1.7921248e-09,
}
var fe = [256]float32{
	1, 0.9381437, 0.90046996, 0.87170434, 0.8477855, 0.8269933,
	0.8084217, 0.7915276, 0.77595687, 0.7614634, 0.7478686,
	0.7350381, 0.72286767, 0.71127474, 0.70019263, 0.6895665,
	0.67935055, 0.6695063, 0.66000086, 0.65080583, 0.6418967,
	0.63325197, 0.6248527, 0.6166822, 0.60872537, 0.60096896,
	0.5934009, 0.58601034, 0.5787874, 0.57172304, 0.5648092,
	0.5580383, 0.5514034, 0.5448982, 0.5385169, 0.53225386,
	0.5261042, 0.52006316, 0.5141264, 0.50828975, 0.5025495,
	0.496902, 0.49134386, 0.485872, 0.48048335, 0.4751752,
	0.46994483, 0.46478975, 0.45970762, 0.45469615, 0.44975325,
	0.44487688, 0.44006512, 0.43531612, 0.43062815, 0.42599955,
	0.42142874, 0.4169142, 0.41245446, 0.40804818, 0.403694,
	0.3993907, 0.39513698, 0.39093173, 0.38677382, 0.38266218,
	0.37859577, 0.37457356, 0.37059465, 0.3666581, 0.362763,
	0.35890847, 0.35509375, 0.351318, 0.3475805, 0.34388044,
	0.34021714, 0.3365899, 0.33299807, 0.32944095, 0.32591796,
	0.3224285, 0.3189719, 0.31554767, 0.31215525, 0.30879408,
	0.3054636, 0.3021634, 0.29889292, 0.2956517, 0.29243928,
	0.28925523, 0.28609908, 0.28297043, 0.27986884, 0.27679393,
	0.2737453, 0.2707226, 0.2677254, 0.26475343, 0.26180625,
	0.25888354, 0.25598502, 0.2531103, 0.25025907, 0.24743107,
	0.24462597, 0.24184346, 0.23908329, 0.23634516, 0.23362878,
	0.23093392, 0.2282603, 0.22560766, 0.22297576, 0.22036438,
	0.21777324, 0.21520215, 0.21265087, 0.21011916, 0.20760682,
	0.20511365, 0.20263945, 0.20018397, 0.19774707, 0.19532852,
	0.19292815, 0.19054577, 0.1881812, 0.18583426, 0.18350479,
	0.1811926, 0.17889754, 0.17661946, 0.17435817, 0.17211354,
	0.1698854, 0.16767362, 0.16547804, 0.16329853, 0.16113494,
	0.15898713, 0.15685499, 0.15473837, 0.15263714, 0.15055119,
	0.14848037, 0.14642459, 0.14438373, 0.14235765, 0.14034624,
	0.13834943, 0.13636707, 0.13439907, 0.13244532, 0.13050574,
	0.1285802, 0.12666863, 0.12477092, 0.12288698, 0.12101672,
	0.119160056, 0.1173169, 0.115487166, 0.11367077, 0.11186763,
	0.11007768, 0.10830083, 0.10653701, 0.10478614, 0.10304816,
	0.101323, 0.09961058, 0.09791085, 0.09622374, 0.09454919,
	0.09288713, 0.091237515, 0.08960028, 0.087975375, 0.08636274,
	0.08476233, 0.083174095, 0.081597984, 0.08003395, 0.07848195,
	0.076941945, 0.07541389, 0.07389775, 0.072393484, 0.07090106,
	0.069420435, 0.06795159, 0.066494495, 0.06504912, 0.063615434,
	0.062193416, 0.060783047, 0.059384305, 0.057997175,
	0.05662164, 0.05525769, 0.053905312, 0.052564494, 0.051235236,
	0.049917534, 0.048611384, 0.047316793, 0.046033762, 0.0447623,
	0.043502413, 0.042254124, 0.041017443, 0.039792392,
	0.038578995, 0.037377283, 0.036187284, 0.035009038,
	0.033842582, 0.032687962, 0.031545233, 0.030414443, 0.02929566,
	0.02818895, 0.027094385, 0.026012046, 0.024942026, 0.023884421,
	0.022839336, 0.021806888, 0.020787204, 0.019780423, 0.0187867,
	0.0178062, 0.016839107, 0.015885621, 0.014945968, 0.014020392,
	0.013109165, 0.012212592, 0.011331013, 0.01046481, 0.009614414,
	0.008780315, 0.007963077, 0.0071633533, 0.006381906,
	0.0056196423, 0.0048776558, 0.004157295, 0.0034602648,
	0.0027887989, 0.0021459677, 0.0015362998, 0.0009672693,
	0.00045413437,
}

```

// === FILE: references!/go/src/math/rand/v2/normal.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rand

import (
	"math"
)

/*
 * Normal distribution
 *
 * See "The Ziggurat Method for Generating Random Variables"
 * (Marsaglia & Tsang, 2000)
 * http://www.jstatsoft.org/v05/i08/paper [pdf]
 */

const (
	rn = 3.442619855899
)

func absInt32(i int32) uint32 {
	if i < 0 {
		return uint32(-i)
	}
	return uint32(i)
}

// NormFloat64 returns a normally distributed float64 in
// the range -math.MaxFloat64 through +math.MaxFloat64 inclusive,
// with standard normal distribution (mean = 0, stddev = 1).
// To produce a different normal distribution, callers can
// adjust the output using:
//
//	sample = NormFloat64() * desiredStdDev + desiredMean
func (r *Rand) NormFloat64() float64 {
	for {
		u := r.Uint64()
		j := int32(u) // Possibly negative
		i := u >> 32 & 0x7F
		x := float64(j) * float64(wn[i])
		if absInt32(j) < kn[i] {
			// This case should be hit better than 99% of the time.
			return x
		}

		if i == 0 {
			// This extra work is only required for the base strip.
			for {
				x = -math.Log(r.Float64()) * (1.0 / rn)
				y := -math.Log(r.Float64())
				if y+y >= x*x {
					break
				}
			}
			if j > 0 {
				return rn + x
			}
			return -rn - x
		}
		if fn[i]+float32(r.Float64())*(fn[i-1]-fn[i]) < float32(math.Exp(-.5*x*x)) {
			return x
		}
	}
}

var kn = [128]uint32{
	0x76ad2212, 0x0, 0x600f1b53, 0x6ce447a6, 0x725b46a2,
	0x7560051d, 0x774921eb, 0x789a25bd, 0x799045c3, 0x7a4bce5d,
	0x7adf629f, 0x7b5682a6, 0x7bb8a8c6, 0x7c0ae722, 0x7c50cce7,
	0x7c8cec5b, 0x7cc12cd6, 0x7ceefed2, 0x7d177e0b, 0x7d3b8883,
	0x7d5bce6c, 0x7d78dd64, 0x7d932886, 0x7dab0e57, 0x7dc0dd30,
	0x7dd4d688, 0x7de73185, 0x7df81cea, 0x7e07c0a3, 0x7e163efa,
	0x7e23b587, 0x7e303dfd, 0x7e3beec2, 0x7e46db77, 0x7e51155d,
	0x7e5aabb3, 0x7e63abf7, 0x7e6c222c, 0x7e741906, 0x7e7b9a18,
	0x7e82adfa, 0x7e895c63, 0x7e8fac4b, 0x7e95a3fb, 0x7e9b4924,
	0x7ea0a0ef, 0x7ea5b00d, 0x7eaa7ac3, 0x7eaf04f3, 0x7eb3522a,
	0x7eb765a5, 0x7ebb4259, 0x7ebeeafd, 0x7ec2620a, 0x7ec5a9c4,
	0x7ec8c441, 0x7ecbb365, 0x7ece78ed, 0x7ed11671, 0x7ed38d62,
	0x7ed5df12, 0x7ed80cb4, 0x7eda175c, 0x7edc0005, 0x7eddc78e,
	0x7edf6ebf, 0x7ee0f647, 0x7ee25ebe, 0x7ee3a8a9, 0x7ee4d473,
	0x7ee5e276, 0x7ee6d2f5, 0x7ee7a620, 0x7ee85c10, 0x7ee8f4cd,
	0x7ee97047, 0x7ee9ce59, 0x7eea0eca, 0x7eea3147, 0x7eea3568,
	0x7eea1aab, 0x7ee9e071, 0x7ee98602, 0x7ee90a88, 0x7ee86d08,
	0x7ee7ac6a, 0x7ee6c769, 0x7ee5bc9c, 0x7ee48a67, 0x7ee32efc,
	0x7ee1a857, 0x7edff42f, 0x7ede0ffa, 0x7edbf8d9, 0x7ed9ab94,
	0x7ed7248d, 0x7ed45fae, 0x7ed1585c, 0x7ece095f, 0x7eca6ccb,
	0x7ec67be2, 0x7ec22eee, 0x7ebd7d1a, 0x7eb85c35, 0x7eb2c075,
	0x7eac9c20, 0x7ea5df27, 0x7e9e769f, 0x7e964c16, 0x7e8d44ba,
	0x7e834033, 0x7e781728, 0x7e6b9933, 0x7e5d8a1a, 0x7e4d9ded,
	0x7e3b737a, 0x7e268c2f, 0x7e0e3ff5, 0x7df1aa5d, 0x7dcf8c72,
	0x7da61a1e, 0x7d72a0fb, 0x7d30e097, 0x7cd9b4ab, 0x7c600f1a,
	0x7ba90bdc, 0x7a722176, 0x77d664e5,
}
var wn = [128]float32{
	1.7290405e-09, 1.2680929e-10, 1.6897518e-10, 1.9862688e-10,
	2.2232431e-10, 2.4244937e-10, 2.601613e-10, 2.7611988e-10,
	2.9073963e-10, 3.042997e-10, 3.1699796e-10, 3.289802e-10,
	3.4035738e-10, 3.5121603e-10, 3.616251e-10, 3.7164058e-10,
	3.8130857e-10, 3.9066758e-10, 3.9975012e-10, 4.08584e-10,
	4.1719309e-10, 4.2559822e-10, 4.338176e-10, 4.418672e-10,
	4.497613e-10, 4.5751258e-10, 4.651324e-10, 4.7263105e-10,
	4.8001775e-10, 4.87301e-10, 4.944885e-10, 5.015873e-10,
	5.0860405e-10, 5.155446e-10, 5.2241467e-10, 5.2921934e-10,
	5.359635e-10, 5.426517e-10, 5.4928817e-10, 5.5587696e-10,
	5.624219e-10, 5.6892646e-10, 5.753941e-10, 5.818282e-10,
	5.882317e-10, 5.946077e-10, 6.00959e-10, 6.072884e-10,
	6.135985e-10, 6.19892e-10, 6.2617134e-10, 6.3243905e-10,
	6.386974e-10, 6.449488e-10, 6.511956e-10, 6.5744005e-10,
	6.6368433e-10, 6.699307e-10, 6.7618144e-10, 6.824387e-10,
	6.8870465e-10, 6.949815e-10, 7.012715e-10, 7.075768e-10,
	7.1389966e-10, 7.202424e-10, 7.266073e-10, 7.329966e-10,
	7.394128e-10, 7.4585826e-10, 7.5233547e-10, 7.58847e-10,
	7.653954e-10, 7.719835e-10, 7.7861395e-10, 7.852897e-10,
	7.920138e-10, 7.987892e-10, 8.0561924e-10, 8.125073e-10,
	8.194569e-10, 8.2647167e-10, 8.3355556e-10, 8.407127e-10,
	8.479473e-10, 8.55264e-10, 8.6266755e-10, 8.7016316e-10,
	8.777562e-10, 8.8545243e-10, 8.932582e-10, 9.0117996e-10,
	9.09225e-10, 9.174008e-10, 9.2571584e-10, 9.341788e-10,
	9.427997e-10, 9.515889e-10, 9.605579e-10, 9.697193e-10,
	9.790869e-10, 9.88676e-10, 9.985036e-10, 1.0085882e-09,
	1.0189509e-09, 1.0296151e-09, 1.0406069e-09, 1.0519566e-09,
	1.063698e-09, 1.0758702e-09, 1.0885183e-09, 1.1016947e-09,
	1.1154611e-09, 1.1298902e-09, 1.1450696e-09, 1.1611052e-09,
	1.1781276e-09, 1.1962995e-09, 1.2158287e-09, 1.2369856e-09,
	1.2601323e-09, 1.2857697e-09, 1.3146202e-09, 1.347784e-09,
	1.3870636e-09, 1.4357403e-09, 1.5008659e-09, 1.6030948e-09,
}
var fn = [128]float32{
	1, 0.9635997, 0.9362827, 0.9130436, 0.89228165, 0.87324303,
	0.8555006, 0.8387836, 0.8229072, 0.8077383, 0.793177,
	0.7791461, 0.7655842, 0.7524416, 0.73967725, 0.7272569,
	0.7151515, 0.7033361, 0.69178915, 0.68049186, 0.6694277,
	0.658582, 0.6479418, 0.63749546, 0.6272325, 0.6171434,
	0.6072195, 0.5974532, 0.58783704, 0.5783647, 0.56903,
	0.5598274, 0.5507518, 0.54179835, 0.5329627, 0.52424055,
	0.5156282, 0.50712204, 0.49871865, 0.49041483, 0.48220766,
	0.4740943, 0.46607214, 0.4581387, 0.45029163, 0.44252872,
	0.43484783, 0.427247, 0.41972435, 0.41227803, 0.40490642,
	0.39760786, 0.3903808, 0.3832238, 0.37613547, 0.36911446,
	0.3621595, 0.35526937, 0.34844297, 0.34167916, 0.33497685,
	0.3283351, 0.3217529, 0.3152294, 0.30876362, 0.30235484,
	0.29600215, 0.28970486, 0.2834622, 0.2772735, 0.27113807,
	0.2650553, 0.25902456, 0.2530453, 0.24711695, 0.241239,
	0.23541094, 0.22963232, 0.2239027, 0.21822165, 0.21258877,
	0.20700371, 0.20146611, 0.19597565, 0.19053204, 0.18513499,
	0.17978427, 0.17447963, 0.1692209, 0.16400786, 0.15884037,
	0.15371831, 0.14864157, 0.14361008, 0.13862377, 0.13368265,
	0.12878671, 0.12393598, 0.119130544, 0.11437051, 0.10965602,
	0.104987256, 0.10036444, 0.095787846, 0.0912578, 0.08677467,
	0.0823389, 0.077950984, 0.073611505, 0.06932112, 0.06508058,
	0.06089077, 0.056752663, 0.0526674, 0.048636295, 0.044660863,
	0.040742867, 0.03688439, 0.033087887, 0.029356318,
	0.025693292, 0.022103304, 0.018592102, 0.015167298,
	0.011839478, 0.008624485, 0.005548995, 0.0026696292,
}

```

// === FILE: references!/go/src/math/rand/v2/pcg.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rand

import (
	"errors"
	"internal/byteorder"
	"math/bits"
)

// https://numpy.org/devdocs/reference/random/upgrading-pcg64.html
// https://github.com/imneme/pcg-cpp/commit/871d0494ee9c9a7b7c43f753e3d8ca47c26f8005

// A PCG is a PCG generator with 128 bits of internal state.
// A zero PCG is equivalent to NewPCG(0, 0).
type PCG struct {
	hi uint64
	lo uint64
}

// NewPCG returns a new PCG seeded with the given values.
func NewPCG(seed1, seed2 uint64) *PCG {
	return &PCG{seed1, seed2}
}

// Seed resets the PCG to behave the same way as NewPCG(seed1, seed2).
func (p *PCG) Seed(seed1, seed2 uint64) {
	p.hi = seed1
	p.lo = seed2
}

// AppendBinary implements the [encoding.BinaryAppender] interface.
func (p *PCG) AppendBinary(b []byte) ([]byte, error) {
	b = append(b, "pcg:"...)
	b = byteorder.BEAppendUint64(b, p.hi)
	b = byteorder.BEAppendUint64(b, p.lo)
	return b, nil
}

// MarshalBinary implements the [encoding.BinaryMarshaler] interface.
func (p *PCG) MarshalBinary() ([]byte, error) {
	return p.AppendBinary(make([]byte, 0, 20))
}

var errUnmarshalPCG = errors.New("invalid PCG encoding")

// UnmarshalBinary implements the [encoding.BinaryUnmarshaler] interface.
func (p *PCG) UnmarshalBinary(data []byte) error {
	if len(data) != 20 || string(data[:4]) != "pcg:" {
		return errUnmarshalPCG
	}
	p.hi = byteorder.BEUint64(data[4:])
	p.lo = byteorder.BEUint64(data[4+8:])
	return nil
}

func (p *PCG) next() (hi, lo uint64) {
	// https://github.com/imneme/pcg-cpp/blob/428802d1a5/include/pcg_random.hpp#L161
	//
	// Numpy's PCG multiplies by the 64-bit value cheapMul
	// instead of the 128-bit value used here and in the official PCG code.
	// This does not seem worthwhile, at least for Go: not having any high
	// bits in the multiplier reduces the effect of low bits on the highest bits,
	// and it only saves 1 multiply out of 3.
	// (On 32-bit systems, it saves 1 out of 6, since Mul64 is doing 4.)
	const (
		mulHi = 2549297995355413924
		mulLo = 4865540595714422341
		incHi = 6364136223846793005
		incLo = 1442695040888963407
	)

	// state = state * mul + inc
	hi, lo = bits.Mul64(p.lo, mulLo)
	hi += p.hi*mulLo + p.lo*mulHi
	lo, c := bits.Add64(lo, incLo, 0)
	hi, _ = bits.Add64(hi, incHi, c)
	p.lo = lo
	p.hi = hi
	return hi, lo
}

// Uint64 return a uniformly-distributed random uint64 value.
func (p *PCG) Uint64() uint64 {
	hi, lo := p.next()

	// XSL-RR would be
	//	hi, lo := p.next()
	//	return bits.RotateLeft64(lo^hi, -int(hi>>58))
	// but Numpy uses DXSM and O'Neill suggests doing the same.
	// See https://github.com/golang/go/issues/21835#issuecomment-739065688
	// and following comments.

	// DXSM "double xorshift multiply"
	// https://github.com/imneme/pcg-cpp/blob/428802d1a5/include/pcg_random.hpp#L1015

	// https://github.com/imneme/pcg-cpp/blob/428802d1a5/include/pcg_random.hpp#L176
	const cheapMul = 0xda942042e4dd58b5
	hi ^= hi >> 32
	hi *= cheapMul
	hi ^= hi >> 48
	hi *= (lo | 1)
	return hi
}

```

// === FILE: references!/go/src/math/rand/v2/rand.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package rand implements pseudo-random number generators suitable for tasks
// such as simulation, but it should not be used for security-sensitive work.
//
// Random numbers are generated by a [Source], usually wrapped in a [Rand].
// Both types should be used by a single goroutine at a time: sharing among
// multiple goroutines requires some kind of synchronization.
//
// Top-level functions, such as [Float64] and [Int],
// are safe for concurrent use by multiple goroutines.
//
// This package's outputs might be easily predictable regardless of how it's
// seeded. For random numbers suitable for security-sensitive work, see the
// [crypto/rand] package.
package rand

import (
	"math/bits"
	_ "unsafe" // for go:linkname
)

// A Source is a source of uniformly-distributed
// pseudo-random uint64 values in the range [0, 1<<64).
//
// A Source is not safe for concurrent use by multiple goroutines.
type Source interface {
	Uint64() uint64
}

// A Rand is a source of random numbers.
type Rand struct {
	src Source
}

// New returns a new Rand that uses random values from src
// to generate other random values.
func New(src Source) *Rand {
	return &Rand{src: src}
}

// Int64 returns a non-negative pseudo-random 63-bit integer as an int64.
func (r *Rand) Int64() int64 { return int64(r.src.Uint64() &^ (1 << 63)) }

// Uint32 returns a pseudo-random 32-bit value as a uint32.
func (r *Rand) Uint32() uint32 { return uint32(r.src.Uint64() >> 32) }

// Uint64 returns a pseudo-random 64-bit value as a uint64.
func (r *Rand) Uint64() uint64 { return r.src.Uint64() }

// Int32 returns a non-negative pseudo-random 31-bit integer as an int32.
func (r *Rand) Int32() int32 { return int32(r.src.Uint64() >> 33) }

// Int returns a non-negative pseudo-random int.
func (r *Rand) Int() int { return int(uint(r.src.Uint64()) << 1 >> 1) }

// Uint returns a pseudo-random uint.
func (r *Rand) Uint() uint { return uint(r.src.Uint64()) }

// Int64N returns, as an int64, a non-negative pseudo-random number in the half-open interval [0,n).
// It panics if n <= 0.
func (r *Rand) Int64N(n int64) int64 {
	if n <= 0 {
		panic("invalid argument to Int64N")
	}
	return int64(r.uint64n(uint64(n)))
}

// Uint64N returns, as a uint64, a non-negative pseudo-random number in the half-open interval [0,n).
// It panics if n == 0.
func (r *Rand) Uint64N(n uint64) uint64 {
	if n == 0 {
		panic("invalid argument to Uint64N")
	}
	return r.uint64n(n)
}

// uint64n is the no-bounds-checks version of Uint64N.
func (r *Rand) uint64n(n uint64) uint64 {
	if is32bit && uint64(uint32(n)) == n {
		return uint64(r.uint32n(uint32(n)))
	}
	if n&(n-1) == 0 { // n is power of two, can mask
		return r.Uint64() & (n - 1)
	}

	// Suppose we have a uint64 x uniform in the range [0,2⁶⁴)
	// and want to reduce it to the range [0,n) preserving exact uniformity.
	// We can simulate a scaling arbitrary precision x * (n/2⁶⁴) by
	// the high bits of a double-width multiply of x*n, meaning (x*n)/2⁶⁴.
	// Since there are 2⁶⁴ possible inputs x and only n possible outputs,
	// the output is necessarily biased if n does not divide 2⁶⁴.
	// In general (x*n)/2⁶⁴ = k for x*n in [k*2⁶⁴,(k+1)*2⁶⁴).
	// There are either floor(2⁶⁴/n) or ceil(2⁶⁴/n) possible products
	// in that range, depending on k.
	// But suppose we reject the sample and try again when
	// x*n is in [k*2⁶⁴, k*2⁶⁴+(2⁶⁴%n)), meaning rejecting fewer than n possible
	// outcomes out of the 2⁶⁴.
	// Now there are exactly floor(2⁶⁴/n) possible ways to produce
	// each output value k, so we've restored uniformity.
	// To get valid uint64 math, 2⁶⁴ % n = (2⁶⁴ - n) % n = -n % n,
	// so the direct implementation of this algorithm would be:
	//
	//	hi, lo := bits.Mul64(r.Uint64(), n)
	//	thresh := -n % n
	//	for lo < thresh {
	//		hi, lo = bits.Mul64(r.Uint64(), n)
	//	}
	//
	// That still leaves an expensive 64-bit division that we would rather avoid.
	// We know that thresh < n, and n is usually much less than 2⁶⁴, so we can
	// avoid the last four lines unless lo < n.
	//
	// See also:
	// https://lemire.me/blog/2016/06/27/a-fast-alternative-to-the-modulo-reduction
	// https://lemire.me/blog/2016/06/30/fast-random-shuffling
	hi, lo := bits.Mul64(r.Uint64(), n)
	if lo < n {
		thresh := -n % n
		for lo < thresh {
			hi, lo = bits.Mul64(r.Uint64(), n)
		}
	}
	return hi
}

// uint32n is an identical computation to uint64n
// but optimized for 32-bit systems.
func (r *Rand) uint32n(n uint32) uint32 {
	if n&(n-1) == 0 { // n is power of two, can mask
		return uint32(r.Uint64()) & (n - 1)
	}
	// On 64-bit systems we still use the uint64 code below because
	// the probability of a random uint64 lo being < a uint32 n is near zero,
	// meaning the unbiasing loop almost never runs.
	// On 32-bit systems, here we need to implement that same logic in 32-bit math,
	// both to preserve the exact output sequence observed on 64-bit machines
	// and to preserve the optimization that the unbiasing loop almost never runs.
	//
	// We want to compute
	// 	hi, lo := bits.Mul64(r.Uint64(), n)
	// In terms of 32-bit halves, this is:
	// 	x1:x0 := r.Uint64()
	// 	0:hi, lo1:lo0 := bits.Mul64(x1:x0, 0:n)
	// Writing out the multiplication in terms of bits.Mul32 allows
	// using direct hardware instructions and avoiding
	// the computations involving these zeros.
	x := r.Uint64()
	lo1a, lo0 := bits.Mul32(uint32(x), n)
	hi, lo1b := bits.Mul32(uint32(x>>32), n)
	lo1, c := bits.Add32(lo1a, lo1b, 0)
	hi += c
	if lo1 == 0 && lo0 < uint32(n) {
		n64 := uint64(n)
		thresh := uint32(-n64 % n64)
		for lo1 == 0 && lo0 < thresh {
			x := r.Uint64()
			lo1a, lo0 = bits.Mul32(uint32(x), n)
			hi, lo1b = bits.Mul32(uint32(x>>32), n)
			lo1, c = bits.Add32(lo1a, lo1b, 0)
			hi += c
		}
	}
	return hi
}

// Int32N returns, as an int32, a non-negative pseudo-random number in the half-open interval [0,n).
// It panics if n <= 0.
func (r *Rand) Int32N(n int32) int32 {
	if n <= 0 {
		panic("invalid argument to Int32N")
	}
	return int32(r.uint64n(uint64(n)))
}

// Uint32N returns, as a uint32, a non-negative pseudo-random number in the half-open interval [0,n).
// It panics if n == 0.
func (r *Rand) Uint32N(n uint32) uint32 {
	if n == 0 {
		panic("invalid argument to Uint32N")
	}
	return uint32(r.uint64n(uint64(n)))
}

const is32bit = ^uint(0)>>32 == 0

// IntN returns, as an int, a non-negative pseudo-random number in the half-open interval [0,n).
// It panics if n <= 0.
func (r *Rand) IntN(n int) int {
	if n <= 0 {
		panic("invalid argument to IntN")
	}
	return int(r.uint64n(uint64(n)))
}

// UintN returns, as a uint, a non-negative pseudo-random number in the half-open interval [0,n).
// It panics if n == 0.
func (r *Rand) UintN(n uint) uint {
	if n == 0 {
		panic("invalid argument to UintN")
	}
	return uint(r.uint64n(uint64(n)))
}

// N returns a pseudo-random number in the half-open interval [0,n).
// The type parameter Int can be any integer type.
// It panics if n <= 0.
func (r *Rand) N[Int intType](n Int) Int {
	if n <= 0 {
		panic("invalid argument to N")
	}
	return Int(r.uint64n(uint64(n)))
}

// Float64 returns, as a float64, a pseudo-random number in the half-open interval [0.0,1.0).
func (r *Rand) Float64() float64 {
	// There are exactly 1<<53 float64s in [0,1). Use Intn(1<<53) / (1<<53).
	return float64(r.Uint64()<<11>>11) / (1 << 53)
}

// Float32 returns, as a float32, a pseudo-random number in the half-open interval [0.0,1.0).
func (r *Rand) Float32() float32 {
	// There are exactly 1<<24 float32s in [0,1). Use Intn(1<<24) / (1<<24).
	return float32(r.Uint32()<<8>>8) / (1 << 24)
}

// Perm returns, as a slice of n ints, a pseudo-random permutation of the integers
// in the half-open interval [0,n).
func (r *Rand) Perm(n int) []int {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	r.Shuffle(len(p), func(i, j int) { p[i], p[j] = p[j], p[i] })
	return p
}

// Shuffle pseudo-randomizes the order of elements.
// n is the number of elements. Shuffle panics if n < 0.
// swap swaps the elements with indexes i and j.
func (r *Rand) Shuffle(n int, swap func(i, j int)) {
	if n < 0 {
		panic("invalid argument to Shuffle")
	}

	// Fisher-Yates shuffle: https://en.wikipedia.org/wiki/Fisher%E2%80%93Yates_shuffle
	// Shuffle really ought not be called with n that doesn't fit in 32 bits.
	// Not only will it take a very long time, but with 2³¹! possible permutations,
	// there's no way that any PRNG can have a big enough internal state to
	// generate even a minuscule percentage of the possible permutations.
	// Nevertheless, the right API signature accepts an int n, so handle it as best we can.
	for i := n - 1; i > 0; i-- {
		j := int(r.uint64n(uint64(i + 1)))
		swap(i, j)
	}
}

/*
 * Top-level convenience functions
 */

// globalRand is the source of random numbers for the top-level
// convenience functions.
var globalRand = &Rand{src: runtimeSource{}}

//go:linkname runtime_rand runtime.rand
func runtime_rand() uint64

// runtimeSource is a Source that uses the runtime fastrand functions.
type runtimeSource struct{}

func (runtimeSource) Uint64() uint64 {
	return runtime_rand()
}

// Int64 returns a non-negative pseudo-random 63-bit integer as an int64
// from the default Source.
func Int64() int64 { return globalRand.Int64() }

// Uint32 returns a pseudo-random 32-bit value as a uint32
// from the default Source.
func Uint32() uint32 { return globalRand.Uint32() }

// Uint64N returns, as a uint64, a pseudo-random number in the half-open interval [0,n)
// from the default Source.
// It panics if n == 0.
func Uint64N(n uint64) uint64 { return globalRand.Uint64N(n) }

// Uint32N returns, as a uint32, a pseudo-random number in the half-open interval [0,n)
// from the default Source.
// It panics if n == 0.
func Uint32N(n uint32) uint32 { return globalRand.Uint32N(n) }

// Uint64 returns a pseudo-random 64-bit value as a uint64
// from the default Source.
func Uint64() uint64 { return globalRand.Uint64() }

// Int32 returns a non-negative pseudo-random 31-bit integer as an int32
// from the default Source.
func Int32() int32 { return globalRand.Int32() }

// Int returns a non-negative pseudo-random int from the default Source.
func Int() int { return globalRand.Int() }

// Uint returns a pseudo-random uint from the default Source.
func Uint() uint { return globalRand.Uint() }

// Int64N returns, as an int64, a pseudo-random number in the half-open interval [0,n)
// from the default Source.
// It panics if n <= 0.
func Int64N(n int64) int64 { return globalRand.Int64N(n) }

// Int32N returns, as an int32, a pseudo-random number in the half-open interval [0,n)
// from the default Source.
// It panics if n <= 0.
func Int32N(n int32) int32 { return globalRand.Int32N(n) }

// IntN returns, as an int, a pseudo-random number in the half-open interval [0,n)
// from the default Source.
// It panics if n <= 0.
func IntN(n int) int { return globalRand.IntN(n) }

// UintN returns, as a uint, a pseudo-random number in the half-open interval [0,n)
// from the default Source.
// It panics if n == 0.
func UintN(n uint) uint { return globalRand.UintN(n) }

// N returns a pseudo-random number in the half-open interval [0,n) from the default Source.
// The type parameter Int can be any integer type.
// It panics if n <= 0.
func N[Int intType](n Int) Int {
	return globalRand.N(n)
}

type intType interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

// Float64 returns, as a float64, a pseudo-random number in the half-open interval [0.0,1.0)
// from the default Source.
func Float64() float64 { return globalRand.Float64() }

// Float32 returns, as a float32, a pseudo-random number in the half-open interval [0.0,1.0)
// from the default Source.
func Float32() float32 { return globalRand.Float32() }

// Perm returns, as a slice of n ints, a pseudo-random permutation of the integers
// in the half-open interval [0,n) from the default Source.
func Perm(n int) []int { return globalRand.Perm(n) }

// Shuffle pseudo-randomizes the order of elements using the default Source.
// n is the number of elements. Shuffle panics if n < 0.
// swap swaps the elements with indexes i and j.
func Shuffle(n int, swap func(i, j int)) { globalRand.Shuffle(n, swap) }

// NormFloat64 returns a normally distributed float64 in the range
// [-math.MaxFloat64, +math.MaxFloat64] with
// standard normal distribution (mean = 0, stddev = 1)
// from the default Source.
// To produce a different normal distribution, callers can
// adjust the output using:
//
//	sample = NormFloat64() * desiredStdDev + desiredMean
func NormFloat64() float64 { return globalRand.NormFloat64() }

// ExpFloat64 returns an exponentially distributed float64 in the range
// (0, +math.MaxFloat64] with an exponential distribution whose rate parameter
// (lambda) is 1 and whose mean is 1/lambda (1) from the default Source.
// To produce a distribution with a different rate parameter,
// callers can adjust the output using:
//
//	sample = ExpFloat64() / desiredRateParameter
func ExpFloat64() float64 { return globalRand.ExpFloat64() }

```

// === FILE: references!/go/src/math/rand/v2/zipf.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// W.Hormann, G.Derflinger:
// "Rejection-Inversion to Generate Variates
// from Monotone Discrete Distributions"
// http://eeyore.wu-wien.ac.at/papers/96-04-04.wh-der.ps.gz

package rand

import "math"

// A Zipf generates Zipf distributed variates.
type Zipf struct {
	r            *Rand
	imax         float64
	v            float64
	q            float64
	s            float64
	oneminusQ    float64
	oneminusQinv float64
	hxm          float64
	hx0minusHxm  float64
}

func (z *Zipf) h(x float64) float64 {
	return math.Exp(z.oneminusQ*math.Log(z.v+x)) * z.oneminusQinv
}

func (z *Zipf) hinv(x float64) float64 {
	return math.Exp(z.oneminusQinv*math.Log(z.oneminusQ*x)) - z.v
}

// NewZipf returns a Zipf variate generator.
// The generator generates values k ∈ [0, imax]
// such that P(k) is proportional to (v + k) ** (-s).
// Requirements: s > 1 and v >= 1.
func NewZipf(r *Rand, s float64, v float64, imax uint64) *Zipf {
	z := new(Zipf)
	if s <= 1.0 || v < 1 {
		return nil
	}
	z.r = r
	z.imax = float64(imax)
	z.v = v
	z.q = s
	z.oneminusQ = 1.0 - z.q
	z.oneminusQinv = 1.0 / z.oneminusQ
	z.hxm = z.h(z.imax + 0.5)
	z.hx0minusHxm = z.h(0.5) - math.Exp(math.Log(z.v)*(-z.q)) - z.hxm
	z.s = 1 - z.hinv(z.h(1.5)-math.Exp(-z.q*math.Log(z.v+1.0)))
	return z
}

// Uint64 returns a value drawn from the Zipf distribution described
// by the Zipf object.
func (z *Zipf) Uint64() uint64 {
	if z == nil {
		panic("rand: nil Zipf")
	}
	k := 0.0

	for {
		r := z.r.Float64() // r on [0,1]
		ur := z.hxm + r*z.hx0minusHxm
		x := z.hinv(ur)
		k = math.Floor(x + 0.5)
		if k-x <= z.s {
			break
		}
		if ur >= z.h(k+0.5)-math.Exp(-math.Log(k+z.v)*z.q) {
			break
		}
	}
	return uint64(k)
}

```

// === FILE: references!/go/src/math/rand/zipf.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// W.Hormann, G.Derflinger:
// "Rejection-Inversion to Generate Variates
// from Monotone Discrete Distributions"
// http://eeyore.wu-wien.ac.at/papers/96-04-04.wh-der.ps.gz

package rand

import "math"

// A Zipf generates Zipf distributed variates.
type Zipf struct {
	r            *Rand
	imax         float64
	v            float64
	q            float64
	s            float64
	oneminusQ    float64
	oneminusQinv float64
	hxm          float64
	hx0minusHxm  float64
}

func (z *Zipf) h(x float64) float64 {
	return math.Exp(z.oneminusQ*math.Log(z.v+x)) * z.oneminusQinv
}

func (z *Zipf) hinv(x float64) float64 {
	return math.Exp(z.oneminusQinv*math.Log(z.oneminusQ*x)) - z.v
}

// NewZipf returns a [Zipf] variate generator.
// The generator generates values k ∈ [0, imax]
// such that P(k) is proportional to (v + k) ** (-s).
// Requirements: s > 1 and v >= 1.
func NewZipf(r *Rand, s float64, v float64, imax uint64) *Zipf {
	z := new(Zipf)
	if s <= 1.0 || v < 1 {
		return nil
	}
	z.r = r
	z.imax = float64(imax)
	z.v = v
	z.q = s
	z.oneminusQ = 1.0 - z.q
	z.oneminusQinv = 1.0 / z.oneminusQ
	z.hxm = z.h(z.imax + 0.5)
	z.hx0minusHxm = z.h(0.5) - math.Exp(math.Log(z.v)*(-z.q)) - z.hxm
	z.s = 1 - z.hinv(z.h(1.5)-math.Exp(-z.q*math.Log(z.v+1.0)))
	return z
}

// Uint64 returns a value drawn from the [Zipf] distribution described
// by the [Zipf] object.
func (z *Zipf) Uint64() uint64 {
	if z == nil {
		panic("rand: nil Zipf")
	}
	k := 0.0

	for {
		r := z.r.Float64() // r on [0,1]
		ur := z.hxm + r*z.hx0minusHxm
		x := z.hinv(ur)
		k = math.Floor(x + 0.5)
		if k-x <= z.s {
			break
		}
		if ur >= z.h(k+0.5)-math.Exp(-math.Log(k+z.v)*z.q) {
			break
		}
	}
	return uint64(k)
}

```

// === FILE: references!/go/src/math/remainder.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// The original C code and the comment below are from
// FreeBSD's /usr/src/lib/msun/src/e_remainder.c and came
// with this notice. The go code is a simplified version of
// the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
// __ieee754_remainder(x,y)
// Return :
//      returns  x REM y  =  x - [x/y]*y  as if in infinite
//      precision arithmetic, where [x/y] is the (infinite bit)
//      integer nearest x/y (in half way cases, choose the even one).
// Method :
//      Based on Mod() returning  x - [x/y]chopped * y  exactly.

// Remainder returns the IEEE 754 floating-point remainder of x/y.
//
// Special cases are:
//
//	Remainder(±Inf, y) = NaN
//	Remainder(NaN, y) = NaN
//	Remainder(x, 0) = NaN
//	Remainder(x, ±Inf) = x
//	Remainder(x, NaN) = NaN
func Remainder(x, y float64) float64 {
	if haveArchRemainder {
		return archRemainder(x, y)
	}
	return remainder(x, y)
}

func remainder(x, y float64) float64 {
	const (
		Tiny    = 4.45014771701440276618e-308 // 0x0020000000000000
		HalfMax = MaxFloat64 / 2
	)
	// special cases
	switch {
	case IsNaN(x) || IsNaN(y) || IsInf(x, 0) || y == 0:
		return NaN()
	case IsInf(y, 0):
		return x
	}
	sign := false
	if x < 0 {
		x = -x
		sign = true
	}
	if y < 0 {
		y = -y
	}
	if x == y {
		if sign {
			zero := 0.0
			return -zero
		}
		return 0
	}
	if y <= HalfMax {
		x = Mod(x, y+y) // now x < 2y
	}
	if y < Tiny {
		if x+x > y {
			x -= y
			if x+x >= y {
				x -= y
			}
		}
	} else {
		yHalf := 0.5 * y
		if x > yHalf {
			x -= y
			if x >= yHalf {
				x -= y
			}
		}
	}
	if sign {
		x = -x
	}
	return x
}

```

// === FILE: references!/go/src/math/signbit.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// Signbit reports whether x is negative or negative zero.
func Signbit(x float64) bool {
	return int64(Float64bits(x)) < 0
}

```

// === FILE: references!/go/src/math/sin.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

/*
	Floating-point sine and cosine.
*/

// The original C code, the long comment, and the constants
// below were from http://netlib.sandia.gov/cephes/cmath/sin.c,
// available from http://www.netlib.org/cephes/cmath.tgz.
// The go code is a simplified version of the original C.
//
//      sin.c
//
//      Circular sine
//
// SYNOPSIS:
//
// double x, y, sin();
// y = sin( x );
//
// DESCRIPTION:
//
// Range reduction is into intervals of pi/4.  The reduction error is nearly
// eliminated by contriving an extended precision modular arithmetic.
//
// Two polynomial approximating functions are employed.
// Between 0 and pi/4 the sine is approximated by
//      x  +  x**3 P(x**2).
// Between pi/4 and pi/2 the cosine is represented as
//      1  -  x**2 Q(x**2).
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain      # trials      peak         rms
//    DEC       0, 10       150000       3.0e-17     7.8e-18
//    IEEE -1.07e9,+1.07e9  130000       2.1e-16     5.4e-17
//
// Partial loss of accuracy begins to occur at x = 2**30 = 1.074e9.  The loss
// is not gradual, but jumps suddenly to about 1 part in 10e7.  Results may
// be meaningless for x > 2**49 = 5.6e14.
//
//      cos.c
//
//      Circular cosine
//
// SYNOPSIS:
//
// double x, y, cos();
// y = cos( x );
//
// DESCRIPTION:
//
// Range reduction is into intervals of pi/4.  The reduction error is nearly
// eliminated by contriving an extended precision modular arithmetic.
//
// Two polynomial approximating functions are employed.
// Between 0 and pi/4 the cosine is approximated by
//      1  -  x**2 Q(x**2).
// Between pi/4 and pi/2 the sine is represented as
//      x  +  x**3 P(x**2).
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain      # trials      peak         rms
//    IEEE -1.07e9,+1.07e9  130000       2.1e-16     5.4e-17
//    DEC        0,+1.07e9   17000       3.0e-17     7.2e-18
//
// Cephes Math Library Release 2.8:  June, 2000
// Copyright 1984, 1987, 1989, 1992, 2000 by Stephen L. Moshier
//
// The readme file at http://netlib.sandia.gov/cephes/ says:
//    Some software in this archive may be from the book _Methods and
// Programs for Mathematical Functions_ (Prentice-Hall or Simon & Schuster
// International, 1989) or from the Cephes Mathematical Library, a
// commercial product. In either event, it is copyrighted by the author.
// What you see here may be used freely but it comes with no support or
// guarantee.
//
//   The two known misprints in the book are repaired here in the
// source listings for the gamma function and the incomplete beta
// integral.
//
//   Stephen L. Moshier
//   moshier@na-net.ornl.gov

// sin coefficients
var _sin = [...]float64{
	1.58962301576546568060e-10, // 0x3de5d8fd1fd19ccd
	-2.50507477628578072866e-8, // 0xbe5ae5e5a9291f5d
	2.75573136213857245213e-6,  // 0x3ec71de3567d48a1
	-1.98412698295895385996e-4, // 0xbf2a01a019bfdf03
	8.33333333332211858878e-3,  // 0x3f8111111110f7d0
	-1.66666666666666307295e-1, // 0xbfc5555555555548
}

// cos coefficients
var _cos = [...]float64{
	-1.13585365213876817300e-11, // 0xbda8fa49a0861a9b
	2.08757008419747316778e-9,   // 0x3e21ee9d7b4e3f05
	-2.75573141792967388112e-7,  // 0xbe927e4f7eac4bc6
	2.48015872888517045348e-5,   // 0x3efa01a019c844f5
	-1.38888888888730564116e-3,  // 0xbf56c16c16c14f91
	4.16666666666665929218e-2,   // 0x3fa555555555554b
}

// Cos returns the cosine of the radian argument x.
//
// Special cases are:
//
//	Cos(±Inf) = NaN
//	Cos(NaN) = NaN
func Cos(x float64) float64 {
	if haveArchCos {
		return archCos(x)
	}
	return cos(x)
}

func cos(x float64) float64 {
	const (
		PI4A = 7.85398125648498535156e-1  // 0x3fe921fb40000000, Pi/4 split into three parts
		PI4B = 3.77489470793079817668e-8  // 0x3e64442d00000000,
		PI4C = 2.69515142907905952645e-15 // 0x3ce8469898cc5170,
	)
	// special cases
	switch {
	case IsNaN(x) || IsInf(x, 0):
		return NaN()
	}

	// make argument positive
	sign := false
	x = Abs(x)

	var j uint64
	var y, z float64
	if x >= reduceThreshold {
		j, z = trigReduce(x)
	} else {
		j = uint64(x * (4 / Pi)) // integer part of x/(Pi/4), as integer for tests on the phase angle
		y = float64(j)           // integer part of x/(Pi/4), as float

		// map zeros to origin
		if j&1 == 1 {
			j++
			y++
		}
		j &= 7                               // octant modulo 2Pi radians (360 degrees)
		z = ((x - y*PI4A) - y*PI4B) - y*PI4C // Extended precision modular arithmetic
	}

	if j > 3 {
		j -= 4
		sign = !sign
	}
	if j > 1 {
		sign = !sign
	}

	zz := z * z
	if j == 1 || j == 2 {
		y = z + z*zz*((((((_sin[0]*zz)+_sin[1])*zz+_sin[2])*zz+_sin[3])*zz+_sin[4])*zz+_sin[5])
	} else {
		y = 1.0 - 0.5*zz + zz*zz*((((((_cos[0]*zz)+_cos[1])*zz+_cos[2])*zz+_cos[3])*zz+_cos[4])*zz+_cos[5])
	}
	if sign {
		y = -y
	}
	return y
}

// Sin returns the sine of the radian argument x.
//
// Special cases are:
//
//	Sin(±0) = ±0
//	Sin(±Inf) = NaN
//	Sin(NaN) = NaN
func Sin(x float64) float64 {
	if haveArchSin {
		return archSin(x)
	}
	return sin(x)
}

func sin(x float64) float64 {
	const (
		PI4A = 7.85398125648498535156e-1  // 0x3fe921fb40000000, Pi/4 split into three parts
		PI4B = 3.77489470793079817668e-8  // 0x3e64442d00000000,
		PI4C = 2.69515142907905952645e-15 // 0x3ce8469898cc5170,
	)
	// special cases
	switch {
	case x == 0 || IsNaN(x):
		return x // return ±0 || NaN()
	case IsInf(x, 0):
		return NaN()
	}

	// make argument positive but save the sign
	sign := false
	if x < 0 {
		x = -x
		sign = true
	}

	var j uint64
	var y, z float64
	if x >= reduceThreshold {
		j, z = trigReduce(x)
	} else {
		j = uint64(x * (4 / Pi)) // integer part of x/(Pi/4), as integer for tests on the phase angle
		y = float64(j)           // integer part of x/(Pi/4), as float

		// map zeros to origin
		if j&1 == 1 {
			j++
			y++
		}
		j &= 7                               // octant modulo 2Pi radians (360 degrees)
		z = ((x - y*PI4A) - y*PI4B) - y*PI4C // Extended precision modular arithmetic
	}
	// reflect in x axis
	if j > 3 {
		sign = !sign
		j -= 4
	}
	zz := z * z
	if j == 1 || j == 2 {
		y = 1.0 - 0.5*zz + zz*zz*((((((_cos[0]*zz)+_cos[1])*zz+_cos[2])*zz+_cos[3])*zz+_cos[4])*zz+_cos[5])
	} else {
		y = z + z*zz*((((((_sin[0]*zz)+_sin[1])*zz+_sin[2])*zz+_sin[3])*zz+_sin[4])*zz+_sin[5])
	}
	if sign {
		y = -y
	}
	return y
}

```

// === FILE: references!/go/src/math/sin_s390x.s ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// Various constants
DATA sincosxnan<>+0(SB)/8, $0x7ff8000000000000
GLOBL sincosxnan<>+0(SB), RODATA, $8
DATA sincosxlim<>+0(SB)/8, $0x432921fb54442d19
GLOBL sincosxlim<>+0(SB), RODATA, $8
DATA sincosxadd<>+0(SB)/8, $0xc338000000000000
GLOBL sincosxadd<>+0(SB), RODATA, $8
DATA sincosxpi2l<>+0(SB)/8, $0.108285667392191389e-31
GLOBL sincosxpi2l<>+0(SB), RODATA, $8
DATA sincosxpi2m<>+0(SB)/8, $0.612323399573676480e-16
GLOBL sincosxpi2m<>+0(SB), RODATA, $8
DATA sincosxpi2h<>+0(SB)/8, $0.157079632679489656e+01
GLOBL sincosxpi2h<>+0(SB), RODATA, $8
DATA sincosrpi2<>+0(SB)/8, $0.636619772367581341e+00
GLOBL sincosrpi2<>+0(SB), RODATA, $8

// Minimax polynomial approximations
DATA sincosc0<>+0(SB)/8, $0.100000000000000000E+01
GLOBL sincosc0<>+0(SB), RODATA, $8
DATA sincosc1<>+0(SB)/8, $-.499999999999999833E+00
GLOBL sincosc1<>+0(SB), RODATA, $8
DATA sincosc2<>+0(SB)/8, $0.416666666666625843E-01
GLOBL sincosc2<>+0(SB), RODATA, $8
DATA sincosc3<>+0(SB)/8, $-.138888888885498984E-02
GLOBL sincosc3<>+0(SB), RODATA, $8
DATA sincosc4<>+0(SB)/8, $0.248015871681607202E-04
GLOBL sincosc4<>+0(SB), RODATA, $8
DATA sincosc5<>+0(SB)/8, $-.275572911309937875E-06
GLOBL sincosc5<>+0(SB), RODATA, $8
DATA sincosc6<>+0(SB)/8, $0.208735047247632818E-08
GLOBL sincosc6<>+0(SB), RODATA, $8
DATA sincosc7<>+0(SB)/8, $-.112753632738365317E-10
GLOBL sincosc7<>+0(SB), RODATA, $8
DATA sincoss0<>+0(SB)/8, $0.100000000000000000E+01
GLOBL sincoss0<>+0(SB), RODATA, $8
DATA sincoss1<>+0(SB)/8, $-.166666666666666657E+00
GLOBL sincoss1<>+0(SB), RODATA, $8
DATA sincoss2<>+0(SB)/8, $0.833333333333309209E-02
GLOBL sincoss2<>+0(SB), RODATA, $8
DATA sincoss3<>+0(SB)/8, $-.198412698410701448E-03
GLOBL sincoss3<>+0(SB), RODATA, $8
DATA sincoss4<>+0(SB)/8, $0.275573191453906794E-05
GLOBL sincoss4<>+0(SB), RODATA, $8
DATA sincoss5<>+0(SB)/8, $-.250520918387633290E-07
GLOBL sincoss5<>+0(SB), RODATA, $8
DATA sincoss6<>+0(SB)/8, $0.160571285514715856E-09
GLOBL sincoss6<>+0(SB), RODATA, $8
DATA sincoss7<>+0(SB)/8, $-.753213484933210972E-12
GLOBL sincoss7<>+0(SB), RODATA, $8

// Sin returns the sine of the radian argument x.
//
// Special cases are:
//      Sin(±0) = ±0
//      Sin(±Inf) = NaN
//      Sin(NaN) = NaN
// The algorithm used is minimax polynomial approximation.
// with coefficients determined with a Remez exchange algorithm.

TEXT ·sinAsm(SB),NOSPLIT,$0-16
	FMOVD   x+0(FP), F0
	//special case Sin(±0) = ±0
	FMOVD   $(0.0), F1
	FCMPU   F0, F1
	BEQ     sinIsZero
	LTDBR	F0, F0
	BLTU    L17
	FMOVD   F0, F5
L2:
	MOVD    $sincosxlim<>+0(SB), R1
	FMOVD   0(R1), F1
	FCMPU   F5, F1
	BGT     L16
	MOVD    $sincoss7<>+0(SB), R1
	FMOVD   0(R1), F4
	MOVD    $sincoss6<>+0(SB), R1
	FMOVD   0(R1), F1
	MOVD    $sincoss5<>+0(SB), R1
	VLEG    $0, 0(R1), V18
	MOVD    $sincoss4<>+0(SB), R1
	FMOVD   0(R1), F6
	MOVD    $sincoss2<>+0(SB), R1
	VLEG    $0, 0(R1), V16
	MOVD    $sincoss3<>+0(SB), R1
	FMOVD   0(R1), F7
	MOVD    $sincoss1<>+0(SB), R1
	FMOVD   0(R1), F3
	MOVD    $sincoss0<>+0(SB), R1
	FMOVD   0(R1), F2
	WFCHDBS V2, V5, V2
	BEQ     L18
	MOVD    $sincosrpi2<>+0(SB), R1
	FMOVD   0(R1), F3
	MOVD    $sincosxadd<>+0(SB), R1
	FMOVD   0(R1), F2
	WFMSDB  V0, V3, V2, V3
	FMOVD   0(R1), F6
	FADD    F3, F6
	MOVD    $sincosxpi2h<>+0(SB), R1
	FMOVD   0(R1), F2
	FMSUB   F2, F6, F0
	MOVD    $sincosxpi2m<>+0(SB), R1
	FMOVD   0(R1), F4
	FMADD   F4, F6, F0
	MOVD    $sincosxpi2l<>+0(SB), R1
	WFMDB   V0, V0, V1
	FMOVD   0(R1), F7
	WFMDB   V1, V1, V2
	LGDR    F3, R1
	MOVD    $sincosxlim<>+0(SB), R2
	TMLL	R1, $1
	BEQ     L6
	FMOVD   0(R2), F0
	WFCHDBS V0, V5, V0
	BNE     L14
	MOVD    $sincosc7<>+0(SB), R2
	FMOVD   0(R2), F0
	MOVD    $sincosc6<>+0(SB), R2
	FMOVD   0(R2), F4
	MOVD    $sincosc5<>+0(SB), R2
	WFMADB  V1, V0, V4, V0
	FMOVD   0(R2), F6
	MOVD    $sincosc4<>+0(SB), R2
	WFMADB  V1, V0, V6, V0
	FMOVD   0(R2), F4
	MOVD    $sincosc2<>+0(SB), R2
	FMOVD   0(R2), F6
	WFMADB  V2, V4, V6, V4
	MOVD    $sincosc3<>+0(SB), R2
	FMOVD   0(R2), F3
	MOVD    $sincosc1<>+0(SB), R2
	WFMADB  V2, V0, V3, V0
	FMOVD   0(R2), F6
	WFMADB  V1, V4, V6, V4
	TMLL	R1, $2
	WFMADB  V2, V0, V4, V0
	MOVD    $sincosc0<>+0(SB), R1
	FMOVD   0(R1), F2
	WFMADB  V1, V0, V2, V0
	BNE     L15
	FMOVD   F0, ret+8(FP)
	RET

L6:
	FMOVD   0(R2), F4
	WFCHDBS V4, V5, V4
	BNE     L14
	MOVD    $sincoss7<>+0(SB), R2
	FMOVD   0(R2), F4
	MOVD    $sincoss6<>+0(SB), R2
	FMOVD   0(R2), F3
	MOVD    $sincoss5<>+0(SB), R2
	WFMADB  V1, V4, V3, V4
	WFMADB  V6, V7, V0, V6
	FMOVD   0(R2), F0
	MOVD    $sincoss4<>+0(SB), R2
	FMADD   F4, F1, F0
	FMOVD   0(R2), F3
	MOVD    $sincoss2<>+0(SB), R2
	FMOVD   0(R2), F4
	MOVD    $sincoss3<>+0(SB), R2
	WFMADB  V2, V3, V4, V3
	FMOVD   0(R2), F4
	MOVD    $sincoss1<>+0(SB), R2
	WFMADB  V2, V0, V4, V0
	FMOVD   0(R2), F4
	WFMADB  V1, V3, V4, V3
	FNEG    F6, F4
	WFMADB  V2, V0, V3, V2
	WFMDB   V4, V1, V0
	TMLL	R1, $2
	WFMSDB  V0, V2, V6, V0
	BNE     L15
	FMOVD   F0, ret+8(FP)
	RET

L14:
	MOVD    $sincosxnan<>+0(SB), R1
	FMOVD   0(R1), F0
	FMOVD   F0, ret+8(FP)
	RET

L18:
	WFMDB   V0, V0, V2
	WFMADB  V2, V4, V1, V4
	WFMDB   V2, V2, V1
	WFMADB  V2, V4, V18, V4
	WFMADB  V1, V6, V16, V6
	WFMADB  V1, V4, V7, V4
	WFMADB  V2, V6, V3, V6
	FMUL    F0, F2
	WFMADB  V1, V4, V6, V4
	FMADD   F4, F2, F0
	FMOVD   F0, ret+8(FP)
	RET

L17:
	FNEG    F0, F5
	BR      L2
L15:
	FNEG    F0, F0
	FMOVD   F0, ret+8(FP)
	RET


L16:
	BR     ·sin(SB)		//tail call
sinIsZero:
	FMOVD   F0, ret+8(FP)
	RET

// Cos returns the cosine of the radian argument.
//
// Special cases are:
//      Cos(±Inf) = NaN
//      Cos(NaN) = NaN
// The algorithm used is minimax polynomial approximation.
// with coefficients determined with a Remez exchange algorithm.

TEXT ·cosAsm(SB),NOSPLIT,$0-16
	FMOVD   x+0(FP), F0
	LTDBR	F0, F0
	BLTU    L35
	FMOVD   F0, F1
L21:
	MOVD    $sincosxlim<>+0(SB), R1
	FMOVD   0(R1), F2
	FCMPU   F1, F2
	BGT     L30
	MOVD    $sincosc7<>+0(SB), R1
	FMOVD   0(R1), F4
	MOVD    $sincosc6<>+0(SB), R1
	VLEG    $0, 0(R1), V20
	MOVD    $sincosc5<>+0(SB), R1
	VLEG    $0, 0(R1), V18
	MOVD    $sincosc4<>+0(SB), R1
	FMOVD   0(R1), F6
	MOVD    $sincosc2<>+0(SB), R1
	VLEG    $0, 0(R1), V16
	MOVD    $sincosc3<>+0(SB), R1
	FMOVD   0(R1), F7
	MOVD    $sincosc1<>+0(SB), R1
	FMOVD   0(R1), F5
	MOVD    $sincosrpi2<>+0(SB), R1
	FMOVD   0(R1), F2
	MOVD    $sincosxadd<>+0(SB), R1
	FMOVD   0(R1), F3
	MOVD    $sincoss0<>+0(SB), R1
	WFMSDB  V0, V2, V3, V2
	FMOVD   0(R1), F3
	WFCHDBS V3, V1, V3
	LGDR    F2, R1
	BEQ     L36
	MOVD    $sincosxadd<>+0(SB), R2
	FMOVD   0(R2), F4
	FADD    F2, F4
	MOVD    $sincosxpi2h<>+0(SB), R2
	FMOVD   0(R2), F2
	WFMSDB  V4, V2, V0, V2
	MOVD    $sincosxpi2m<>+0(SB), R2
	FMOVD   0(R2), F0
	WFMADB  V4, V0, V2, V0
	MOVD    $sincosxpi2l<>+0(SB), R2
	WFMDB   V0, V0, V2
	FMOVD   0(R2), F5
	WFMDB   V2, V2, V6
	MOVD    $sincosxlim<>+0(SB), R2
	TMLL	R1, $1
	BNE     L25
	FMOVD   0(R2), F0
	WFCHDBS V0, V1, V0
	BNE     L33
	MOVD    $sincosc7<>+0(SB), R2
	FMOVD   0(R2), F0
	MOVD    $sincosc6<>+0(SB), R2
	FMOVD   0(R2), F4
	MOVD    $sincosc5<>+0(SB), R2
	WFMADB  V2, V0, V4, V0
	FMOVD   0(R2), F1
	MOVD    $sincosc4<>+0(SB), R2
	WFMADB  V2, V0, V1, V0
	FMOVD   0(R2), F4
	MOVD    $sincosc2<>+0(SB), R2
	FMOVD   0(R2), F1
	WFMADB  V6, V4, V1, V4
	MOVD    $sincosc3<>+0(SB), R2
	FMOVD   0(R2), F3
	MOVD    $sincosc1<>+0(SB), R2
	WFMADB  V6, V0, V3, V0
	FMOVD   0(R2), F1
	WFMADB  V2, V4, V1, V4
	TMLL	R1, $2
	WFMADB  V6, V0, V4, V0
	MOVD    $sincosc0<>+0(SB), R1
	FMOVD   0(R1), F4
	WFMADB  V2, V0, V4, V0
	BNE     L34
	FMOVD   F0, ret+8(FP)
	RET

L25:
	FMOVD   0(R2), F3
	WFCHDBS V3, V1, V1
	BNE     L33
	MOVD    $sincoss7<>+0(SB), R2
	FMOVD   0(R2), F1
	MOVD    $sincoss6<>+0(SB), R2
	FMOVD   0(R2), F3
	MOVD    $sincoss5<>+0(SB), R2
	WFMADB  V2, V1, V3, V1
	FMOVD   0(R2), F3
	MOVD    $sincoss4<>+0(SB), R2
	WFMADB  V2, V1, V3, V1
	FMOVD   0(R2), F3
	MOVD    $sincoss2<>+0(SB), R2
	FMOVD   0(R2), F7
	WFMADB  V6, V3, V7, V3
	MOVD    $sincoss3<>+0(SB), R2
	FMADD   F5, F4, F0
	FMOVD   0(R2), F4
	MOVD    $sincoss1<>+0(SB), R2
	FMADD   F1, F6, F4
	FMOVD   0(R2), F1
	FMADD   F3, F2, F1
	FMUL    F0, F2
	WFMADB  V6, V4, V1, V6
	TMLL	R1, $2
	FMADD   F6, F2, F0
	BNE     L34
	FMOVD   F0, ret+8(FP)
	RET

L33:
	MOVD    $sincosxnan<>+0(SB), R1
	FMOVD   0(R1), F0
	FMOVD   F0, ret+8(FP)
	RET

L36:
	FMUL    F0, F0
	MOVD    $sincosc0<>+0(SB), R1
	WFMDB   V0, V0, V1
	WFMADB  V0, V4, V20, V4
	WFMADB  V1, V6, V16, V6
	WFMADB  V0, V4, V18, V4
	WFMADB  V0, V6, V5, V6
	WFMADB  V1, V4, V7, V4
	FMOVD   0(R1), F2
	WFMADB  V1, V4, V6, V4
	WFMADB  V0, V4, V2, V0
	FMOVD   F0, ret+8(FP)
	RET

L35:
	FNEG    F0, F1
	BR      L21
L34:
	FNEG    F0, F0
	FMOVD   F0, ret+8(FP)
	RET

L30:
	BR     ·cos(SB)		//tail call

```

// === FILE: references!/go/src/math/sincos.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// Coefficients _sin[] and _cos[] are found in pkg/math/sin.go.

// Sincos returns Sin(x), Cos(x).
//
// Special cases are:
//
//	Sincos(±0) = ±0, 1
//	Sincos(±Inf) = NaN, NaN
//	Sincos(NaN) = NaN, NaN
func Sincos(x float64) (sin, cos float64) {
	const (
		PI4A = 7.85398125648498535156e-1  // 0x3fe921fb40000000, Pi/4 split into three parts
		PI4B = 3.77489470793079817668e-8  // 0x3e64442d00000000,
		PI4C = 2.69515142907905952645e-15 // 0x3ce8469898cc5170,
	)
	// special cases
	switch {
	case x == 0:
		return x, 1 // return ±0.0, 1.0
	case IsNaN(x) || IsInf(x, 0):
		return NaN(), NaN()
	}

	// make argument positive
	sinSign, cosSign := false, false
	if x < 0 {
		x = -x
		sinSign = true
	}

	var j uint64
	var y, z float64
	if x >= reduceThreshold {
		j, z = trigReduce(x)
	} else {
		j = uint64(x * (4 / Pi)) // integer part of x/(Pi/4), as integer for tests on the phase angle
		y = float64(j)           // integer part of x/(Pi/4), as float

		if j&1 == 1 { // map zeros to origin
			j++
			y++
		}
		j &= 7                               // octant modulo 2Pi radians (360 degrees)
		z = ((x - y*PI4A) - y*PI4B) - y*PI4C // Extended precision modular arithmetic
	}
	if j > 3 { // reflect in x axis
		j -= 4
		sinSign, cosSign = !sinSign, !cosSign
	}
	if j > 1 {
		cosSign = !cosSign
	}

	zz := z * z
	cos = 1.0 - 0.5*zz + zz*zz*((((((_cos[0]*zz)+_cos[1])*zz+_cos[2])*zz+_cos[3])*zz+_cos[4])*zz+_cos[5])
	sin = z + z*zz*((((((_sin[0]*zz)+_sin[1])*zz+_sin[2])*zz+_sin[3])*zz+_sin[4])*zz+_sin[5])
	if j == 1 || j == 2 {
		sin, cos = cos, sin
	}
	if cosSign {
		cos = -cos
	}
	if sinSign {
		sin = -sin
	}
	return
}

```

// === FILE: references!/go/src/math/sinh.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

/*
	Floating-point hyperbolic sine and cosine.

	The exponential func is called for arguments
	greater in magnitude than 0.5.

	A series is used for arguments smaller in magnitude than 0.5.

	Cosh(x) is computed from the exponential func for
	all arguments.
*/

// Sinh returns the hyperbolic sine of x.
//
// Special cases are:
//
//	Sinh(±0) = ±0
//	Sinh(±Inf) = ±Inf
//	Sinh(NaN) = NaN
func Sinh(x float64) float64 {
	if haveArchSinh {
		return archSinh(x)
	}
	return sinh(x)
}

func sinh(x float64) float64 {
	// The coefficients are #2029 from Hart & Cheney. (20.36D)
	const (
		P0 = -0.6307673640497716991184787251e+6
		P1 = -0.8991272022039509355398013511e+5
		P2 = -0.2894211355989563807284660366e+4
		P3 = -0.2630563213397497062819489e+2
		Q0 = -0.6307673640497716991212077277e+6
		Q1 = 0.1521517378790019070696485176e+5
		Q2 = -0.173678953558233699533450911e+3
	)

	sign := false
	if x < 0 {
		x = -x
		sign = true
	}

	var temp float64
	switch {
	case x > 21:
		temp = Exp(x) * 0.5

	case x > 0.5:
		ex := Exp(x)
		temp = (ex - 1/ex) * 0.5

	default:
		sq := x * x
		temp = (((P3*sq+P2)*sq+P1)*sq + P0) * x
		temp = temp / (((sq+Q2)*sq+Q1)*sq + Q0)
	}

	if sign {
		temp = -temp
	}
	return temp
}

// Cosh returns the hyperbolic cosine of x.
//
// Special cases are:
//
//	Cosh(±0) = 1
//	Cosh(±Inf) = +Inf
//	Cosh(NaN) = NaN
func Cosh(x float64) float64 {
	if haveArchCosh {
		return archCosh(x)
	}
	return cosh(x)
}

func cosh(x float64) float64 {
	x = Abs(x)
	if x > 21 {
		return Exp(x) * 0.5
	}
	ex := Exp(x)
	return (ex + 1/ex) * 0.5
}

```

// === FILE: references!/go/src/math/sinh_s390x.s ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.


#include "textflag.h"

// Constants
DATA sinhrodataL21<>+0(SB)/8, $0.231904681384629956E-16
DATA sinhrodataL21<>+8(SB)/8, $0.693147180559945286E+00
DATA sinhrodataL21<>+16(SB)/8, $704.E0
GLOBL sinhrodataL21<>+0(SB), RODATA, $24
DATA sinhrlog2<>+0(SB)/8, $0x3ff7154760000000
GLOBL sinhrlog2<>+0(SB), RODATA, $8
DATA sinhxinf<>+0(SB)/8, $0x7ff0000000000000
GLOBL sinhxinf<>+0(SB), RODATA, $8
DATA sinhxinit<>+0(SB)/8, $0x3ffb504f333f9de6
GLOBL sinhxinit<>+0(SB), RODATA, $8
DATA sinhxlim1<>+0(SB)/8, $800.E0
GLOBL sinhxlim1<>+0(SB), RODATA, $8
DATA sinhxadd<>+0(SB)/8, $0xc3200001610007fb
GLOBL sinhxadd<>+0(SB), RODATA, $8
DATA sinhx4ff<>+0(SB)/8, $0x4ff0000000000000
GLOBL sinhx4ff<>+0(SB), RODATA, $8

// Minimax polynomial approximations
DATA sinhe0<>+0(SB)/8, $0.11715728752538099300E+01
GLOBL sinhe0<>+0(SB), RODATA, $8
DATA sinhe1<>+0(SB)/8, $0.11715728752538099300E+01
GLOBL sinhe1<>+0(SB), RODATA, $8
DATA sinhe2<>+0(SB)/8, $0.58578643762688526692E+00
GLOBL sinhe2<>+0(SB), RODATA, $8
DATA sinhe3<>+0(SB)/8, $0.19526214587563004497E+00
GLOBL sinhe3<>+0(SB), RODATA, $8
DATA sinhe4<>+0(SB)/8, $0.48815536475176217404E-01
GLOBL sinhe4<>+0(SB), RODATA, $8
DATA sinhe5<>+0(SB)/8, $0.97631072948627397816E-02
GLOBL sinhe5<>+0(SB), RODATA, $8
DATA sinhe6<>+0(SB)/8, $0.16271839297756073153E-02
GLOBL sinhe6<>+0(SB), RODATA, $8
DATA sinhe7<>+0(SB)/8, $0.23245485387271142509E-03
GLOBL sinhe7<>+0(SB), RODATA, $8
DATA sinhe8<>+0(SB)/8, $0.29080955860869629131E-04
GLOBL sinhe8<>+0(SB), RODATA, $8
DATA sinhe9<>+0(SB)/8, $0.32311267157667725278E-05
GLOBL sinhe9<>+0(SB), RODATA, $8

// Sinh returns the hyperbolic sine of the argument.
//
// Special cases are:
//      Sinh(±0) = ±0
//      Sinh(±Inf) = ±Inf
//      Sinh(NaN) = NaN
// The algorithm used is minimax polynomial approximation
// with coefficients determined with a Remez exchange algorithm.

TEXT ·sinhAsm(SB),NOSPLIT,$0-16
	FMOVD   x+0(FP), F0
	//special case Sinh(±0) = ±0
	FMOVD   $(0.0), F1
	FCMPU   F0, F1
	BEQ     sinhIsZero
	//special case Sinh(±Inf) = ±Inf
	FMOVD   $1.797693134862315708145274237317043567981e+308, F1
	FCMPU   F1, F0
	BLEU    sinhIsInf
	FMOVD   $-1.797693134862315708145274237317043567981e+308, F1
	FCMPU   F1, F0
	BGT             sinhIsInf

	MOVD    $sinhrodataL21<>+0(SB), R5
	LTDBR	F0, F0
	MOVD    sinhxinit<>+0(SB), R1
	FMOVD   F0, F4
	MOVD    R1, R3
	BLTU    L19
	FMOVD   F0, F2
L2:
	WORD    $0xED205010     //cdb %f2,.L22-.L21(%r5)
	BYTE    $0x00
	BYTE    $0x19
	BGE     L15     //jnl   .L15
	BVS     L15
	WFCEDBS V2, V2, V0
	BEQ     L20
L12:
	FMOVD   F4, F0
	FMOVD   F0, ret+8(FP)
	RET

L15:
	WFCEDBS V2, V2, V0
	BVS     L12
	MOVD    $sinhxlim1<>+0(SB), R2
	FMOVD   0(R2), F0
	WFCHDBS V0, V2, V0
	BEQ     L6
	WFCHEDBS        V4, V2, V6
	MOVD    $sinhxinf<>+0(SB), R1
	FMOVD   0(R1), F0
	BNE     LEXITTAGsinh
	WFCHDBS V2, V4, V2
	BNE     L16
	FNEG    F0, F0
	FMOVD   F0, ret+8(FP)
	RET

L19:
	FNEG    F0, F2
	BR      L2
L6:
	MOVD    $sinhxadd<>+0(SB), R2
	FMOVD   0(R2), F0
	MOVD    sinhrlog2<>+0(SB), R2
	LDGR    R2, F6
	WFMSDB  V4, V6, V0, V16
	FMOVD   sinhrodataL21<>+8(SB), F6
	WFADB   V0, V16, V0
	FMOVD   sinhrodataL21<>+0(SB), F3
	WFMSDB  V0, V6, V4, V6
	MOVD    $sinhe9<>+0(SB), R2
	WFMADB  V0, V3, V6, V0
	FMOVD   0(R2), F1
	MOVD    $sinhe7<>+0(SB), R2
	WFMDB   V0, V0, V6
	FMOVD   0(R2), F5
	MOVD    $sinhe8<>+0(SB), R2
	FMOVD   0(R2), F3
	MOVD    $sinhe6<>+0(SB), R2
	WFMADB  V6, V1, V5, V1
	FMOVD   0(R2), F5
	MOVD    $sinhe5<>+0(SB), R2
	FMOVD   0(R2), F7
	MOVD    $sinhe3<>+0(SB), R2
	WFMADB  V6, V3, V5, V3
	FMOVD   0(R2), F5
	MOVD    $sinhe4<>+0(SB), R2
	WFMADB  V6, V7, V5, V7
	FMOVD   0(R2), F5
	MOVD    $sinhe2<>+0(SB), R2
	VLEG    $0, 0(R2), V20
	WFMDB   V6, V6, V18
	WFMADB  V6, V5, V20, V5
	WFMADB  V1, V18, V7, V1
	FNEG    F0, F0
	WFMADB  V3, V18, V5, V3
	MOVD    $sinhe1<>+0(SB), R3
	WFCEDBS V2, V4, V2
	FMOVD   0(R3), F5
	MOVD    $sinhe0<>+0(SB), R3
	WFMADB  V6, V1, V5, V1
	FMOVD   0(R3), F5
	VLGVG   $0, V16, R2
	WFMADB  V6, V3, V5, V6
	RLL     $3, R2, R2
	RISBGN	$0, $15, $48, R2, R1
	BEQ     L9
	WFMSDB  V0, V1, V6, V0
	MOVD    $sinhx4ff<>+0(SB), R3
	FNEG    F0, F0
	FMOVD   0(R3), F2
	FMUL    F2, F0
	ANDW    $0xFFFF, R2
	WORD    $0xA53FEFB6     //llill %r3,61366
	SUBW    R2, R3, R2
	RISBGN	$0, $15, $48, R2, R1
	LDGR    R1, F2
	FMUL    F2, F0
	FMOVD   F0, ret+8(FP)
	RET

L20:
	MOVD    $sinhxadd<>+0(SB), R2
	FMOVD   0(R2), F2
	MOVD    sinhrlog2<>+0(SB), R2
	LDGR    R2, F0
	WFMSDB  V4, V0, V2, V6
	FMOVD   sinhrodataL21<>+8(SB), F0
	FADD    F6, F2
	MOVD    $sinhe9<>+0(SB), R2
	FMSUB   F0, F2, F4
	FMOVD   0(R2), F1
	FMOVD   sinhrodataL21<>+0(SB), F3
	MOVD    $sinhe7<>+0(SB), R2
	FMADD   F3, F2, F4
	FMOVD   0(R2), F0
	MOVD    $sinhe8<>+0(SB), R2
	WFMDB   V4, V4, V2
	FMOVD   0(R2), F3
	MOVD    $sinhe6<>+0(SB), R2
	FMOVD   0(R2), F5
	LGDR    F6, R2
	RLL     $3, R2, R2
	RISBGN	$0, $15, $48, R2, R1
	WFMADB  V2, V1, V0, V1
	LDGR    R1, F0
	MOVD    $sinhe5<>+0(SB), R1
	WFMADB  V2, V3, V5, V3
	FMOVD   0(R1), F5
	MOVD    $sinhe3<>+0(SB), R1
	FMOVD   0(R1), F6
	WFMDB   V2, V2, V7
	WFMADB  V2, V5, V6, V5
	WORD    $0xA7487FB6     //lhi %r4,32694
	FNEG    F4, F4
	ANDW    $0xFFFF, R2
	SUBW    R2, R4, R2
	RISBGN	$0, $15, $48, R2, R3
	LDGR    R3, F6
	WFADB   V0, V6, V16
	MOVD    $sinhe4<>+0(SB), R1
	WFMADB  V1, V7, V5, V1
	WFMDB   V4, V16, V4
	FMOVD   0(R1), F5
	MOVD    $sinhe2<>+0(SB), R1
	VLEG    $0, 0(R1), V16
	MOVD    $sinhe1<>+0(SB), R1
	WFMADB  V2, V5, V16, V5
	VLEG    $0, 0(R1), V16
	WFMADB  V3, V7, V5, V3
	WFMADB  V2, V1, V16, V1
	FSUB    F6, F0
	FMUL    F1, F4
	MOVD    $sinhe0<>+0(SB), R1
	FMOVD   0(R1), F6
	WFMADB  V2, V3, V6, V2
	WFMADB  V0, V2, V4, V0
	FMOVD   F0, ret+8(FP)
	RET

L9:
	WFMADB  V0, V1, V6, V0
	MOVD    $sinhx4ff<>+0(SB), R3
	FMOVD   0(R3), F2
	FMUL    F2, F0
	WORD    $0xA72AF000     //ahi   %r2,-4096
	RISBGN	$0, $15, $48, R2, R1
	LDGR    R1, F2
	FMUL    F2, F0
	FMOVD   F0, ret+8(FP)
	RET

L16:
	FMOVD   F0, ret+8(FP)
	RET

LEXITTAGsinh:
sinhIsInf:
sinhIsZero:
	FMOVD   F0, ret+8(FP)
	RET

```

// === FILE: references!/go/src/math/sqrt.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// The original C code and the long comment below are
// from FreeBSD's /usr/src/lib/msun/src/e_sqrt.c and
// came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
// __ieee754_sqrt(x)
// Return correctly rounded sqrt.
//           -----------------------------------------
//           | Use the hardware sqrt if you have one |
//           -----------------------------------------
// Method:
//   Bit by bit method using integer arithmetic. (Slow, but portable)
//   1. Normalization
//      Scale x to y in [1,4) with even powers of 2:
//      find an integer k such that  1 <= (y=x*2**(2k)) < 4, then
//              sqrt(x) = 2**k * sqrt(y)
//   2. Bit by bit computation
//      Let q  = sqrt(y) truncated to i bit after binary point (q = 1),
//           i                                                   0
//                                     i+1         2
//          s  = 2*q , and      y  =  2   * ( y - q  ).          (1)
//           i      i            i                 i
//
//      To compute q    from q , one checks whether
//                  i+1       i
//
//                            -(i+1) 2
//                      (q + 2      )  <= y.                     (2)
//                        i
//                                                            -(i+1)
//      If (2) is false, then q   = q ; otherwise q   = q  + 2      .
//                             i+1   i             i+1   i
//
//      With some algebraic manipulation, it is not difficult to see
//      that (2) is equivalent to
//                             -(i+1)
//                      s  +  2       <= y                       (3)
//                       i                i
//
//      The advantage of (3) is that s  and y  can be computed by
//                                    i      i
//      the following recurrence formula:
//          if (3) is false
//
//          s     =  s  ,       y    = y   ;                     (4)
//           i+1      i          i+1    i
//
//      otherwise,
//                         -i                      -(i+1)
//          s     =  s  + 2  ,  y    = y  -  s  - 2              (5)
//           i+1      i          i+1    i     i
//
//      One may easily use induction to prove (4) and (5).
//      Note. Since the left hand side of (3) contain only i+2 bits,
//            it is not necessary to do a full (53-bit) comparison
//            in (3).
//   3. Final rounding
//      After generating the 53 bits result, we compute one more bit.
//      Together with the remainder, we can decide whether the
//      result is exact, bigger than 1/2ulp, or less than 1/2ulp
//      (it will never equal to 1/2ulp).
//      The rounding mode can be detected by checking whether
//      huge + tiny is equal to huge, and whether huge - tiny is
//      equal to huge for some floating point number "huge" and "tiny".
//
//
// Notes:  Rounding mode detection omitted. The constants "mask", "shift",
// and "bias" are found in src/math/bits.go

// Sqrt returns the square root of x.
//
// Special cases are:
//
//	Sqrt(+Inf) = +Inf
//	Sqrt(±0) = ±0
//	Sqrt(x < 0) = NaN
//	Sqrt(NaN) = NaN
func Sqrt(x float64) float64 {
	return sqrt(x)
}

// Note: On systems where Sqrt is a single instruction, the compiler
// may turn a direct call into a direct use of that instruction instead.

func sqrt(x float64) float64 {
	// special cases
	switch {
	case x == 0 || IsNaN(x) || IsInf(x, 1):
		return x
	case x < 0:
		return NaN()
	}
	ix := Float64bits(x)
	// normalize x
	exp := int((ix >> shift) & mask)
	if exp == 0 { // subnormal x
		for ix&(1<<shift) == 0 {
			ix <<= 1
			exp--
		}
		exp++
	}
	exp -= bias // unbias exponent
	ix &^= mask << shift
	ix |= 1 << shift
	if exp&1 == 1 { // odd exp, double x to make it even
		ix <<= 1
	}
	exp >>= 1 // exp = exp/2, exponent of square root
	// generate sqrt(x) bit by bit
	ix <<= 1
	var q, s uint64               // q = sqrt(x)
	r := uint64(1 << (shift + 1)) // r = moving bit from MSB to LSB
	for r != 0 {
		t := s + r
		if t <= ix {
			s = t + r
			ix -= t
			q += r
		}
		ix <<= 1
		r >>= 1
	}
	// final rounding
	if ix != 0 { // remainder, result not exact
		q += q & 1 // round according to extra bit
	}
	ix = q>>1 + uint64(exp-1+bias)<<shift // significand + biased exponent
	return Float64frombits(ix)
}

```

// === FILE: references!/go/src/math/stubs.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !s390x

// This is a large group of functions that most architectures don't
// implement in assembly.

package math

const haveArchAcos = false

func archAcos(x float64) float64 {
	panic("not implemented")
}

const haveArchAcosh = false

func archAcosh(x float64) float64 {
	panic("not implemented")
}

const haveArchAsin = false

func archAsin(x float64) float64 {
	panic("not implemented")
}

const haveArchAsinh = false

func archAsinh(x float64) float64 {
	panic("not implemented")
}

const haveArchAtan = false

func archAtan(x float64) float64 {
	panic("not implemented")
}

const haveArchAtan2 = false

func archAtan2(y, x float64) float64 {
	panic("not implemented")
}

const haveArchAtanh = false

func archAtanh(x float64) float64 {
	panic("not implemented")
}

const haveArchCbrt = false

func archCbrt(x float64) float64 {
	panic("not implemented")
}

const haveArchCos = false

func archCos(x float64) float64 {
	panic("not implemented")
}

const haveArchCosh = false

func archCosh(x float64) float64 {
	panic("not implemented")
}

const haveArchErf = false

func archErf(x float64) float64 {
	panic("not implemented")
}

const haveArchErfc = false

func archErfc(x float64) float64 {
	panic("not implemented")
}

const haveArchExpm1 = false

func archExpm1(x float64) float64 {
	panic("not implemented")
}

const haveArchFrexp = false

func archFrexp(x float64) (float64, int) {
	panic("not implemented")
}

const haveArchLdexp = false

func archLdexp(frac float64, exp int) float64 {
	panic("not implemented")
}

const haveArchLog10 = false

func archLog10(x float64) float64 {
	panic("not implemented")
}

const haveArchLog2 = false

func archLog2(x float64) float64 {
	panic("not implemented")
}

const haveArchLog1p = false

func archLog1p(x float64) float64 {
	panic("not implemented")
}

const haveArchMod = false

func archMod(x, y float64) float64 {
	panic("not implemented")
}

const haveArchPow = false

func archPow(x, y float64) float64 {
	panic("not implemented")
}

const haveArchRemainder = false

func archRemainder(x, y float64) float64 {
	panic("not implemented")
}

const haveArchSin = false

func archSin(x float64) float64 {
	panic("not implemented")
}

const haveArchSinh = false

func archSinh(x float64) float64 {
	panic("not implemented")
}

const haveArchTan = false

func archTan(x float64) float64 {
	panic("not implemented")
}

const haveArchTanh = false

func archTanh(x float64) float64 {
	panic("not implemented")
}

```

// === FILE: references!/go/src/math/stubs_s390x.s ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

TEXT ·archLog10(SB), NOSPLIT, $0
	MOVD ·log10vectorfacility+0x00(SB), R1
	BR   (R1)

TEXT ·log10TrampolineSetup(SB), NOSPLIT, $0
	MOVB   ·hasVX(SB), R1
	CMPBEQ R1, $1, vectorimpl                 // vectorfacility = 1, vector supported
	MOVD   $·log10vectorfacility+0x00(SB), R1
	MOVD   $·log10(SB), R2
	MOVD   R2, 0(R1)
	BR     ·log10(SB)

vectorimpl:
	MOVD $·log10vectorfacility+0x00(SB), R1
	MOVD $·log10Asm(SB), R2
	MOVD R2, 0(R1)
	BR   ·log10Asm(SB)

GLOBL ·log10vectorfacility+0x00(SB), NOPTR, $8
DATA ·log10vectorfacility+0x00(SB)/8, $·log10TrampolineSetup(SB)

TEXT ·archCos(SB), NOSPLIT, $0
	MOVD ·cosvectorfacility+0x00(SB), R1
	BR   (R1)

TEXT ·cosTrampolineSetup(SB), NOSPLIT, $0
	MOVB   ·hasVX(SB), R1
	CMPBEQ R1, $1, vectorimpl               // vectorfacility = 1, vector supported
	MOVD   $·cosvectorfacility+0x00(SB), R1
	MOVD   $·cos(SB), R2
	MOVD   R2, 0(R1)
	BR     ·cos(SB)

vectorimpl:
	MOVD $·cosvectorfacility+0x00(SB), R1
	MOVD $·cosAsm(SB), R2
	MOVD R2, 0(R1)
	BR   ·cosAsm(SB)

GLOBL ·cosvectorfacility+0x00(SB), NOPTR, $8
DATA ·cosvectorfacility+0x00(SB)/8, $·cosTrampolineSetup(SB)

TEXT ·archCosh(SB), NOSPLIT, $0
	MOVD ·coshvectorfacility+0x00(SB), R1
	BR   (R1)

TEXT ·coshTrampolineSetup(SB), NOSPLIT, $0
	MOVB   ·hasVX(SB), R1
	CMPBEQ R1, $1, vectorimpl                // vectorfacility = 1, vector supported
	MOVD   $·coshvectorfacility+0x00(SB), R1
	MOVD   $·cosh(SB), R2
	MOVD   R2, 0(R1)
	BR     ·cosh(SB)

vectorimpl:
	MOVD $·coshvectorfacility+0x00(SB), R1
	MOVD $·coshAsm(SB), R2
	MOVD R2, 0(R1)
	BR   ·coshAsm(SB)

GLOBL ·coshvectorfacility+0x00(SB), NOPTR, $8
DATA ·coshvectorfacility+0x00(SB)/8, $·coshTrampolineSetup(SB)

TEXT ·archSin(SB), NOSPLIT, $0
	MOVD ·sinvectorfacility+0x00(SB), R1
	BR   (R1)

TEXT ·sinTrampolineSetup(SB), NOSPLIT, $0
	MOVB   ·hasVX(SB), R1
	CMPBEQ R1, $1, vectorimpl               // vectorfacility = 1, vector supported
	MOVD   $·sinvectorfacility+0x00(SB), R1
	MOVD   $·sin(SB), R2
	MOVD   R2, 0(R1)
	BR     ·sin(SB)

vectorimpl:
	MOVD $·sinvectorfacility+0x00(SB), R1
	MOVD $·sinAsm(SB), R2
	MOVD R2, 0(R1)
	BR   ·sinAsm(SB)

GLOBL ·sinvectorfacility+0x00(SB), NOPTR, $8
DATA ·sinvectorfacility+0x00(SB)/8, $·sinTrampolineSetup(SB)

TEXT ·archSinh(SB), NOSPLIT, $0
	MOVD ·sinhvectorfacility+0x00(SB), R1
	BR   (R1)

TEXT ·sinhTrampolineSetup(SB), NOSPLIT, $0
	MOVB   ·hasVX(SB), R1
	CMPBEQ R1, $1, vectorimpl                // vectorfacility = 1, vector supported
	MOVD   $·sinhvectorfacility+0x00(SB), R1
	MOVD   $·sinh(SB), R2
	MOVD   R2, 0(R1)
	BR     ·sinh(SB)

vectorimpl:
	MOVD $·sinhvectorfacility+0x00(SB), R1
	MOVD $·sinhAsm(SB), R2
	MOVD R2, 0(R1)
	BR   ·sinhAsm(SB)

GLOBL ·sinhvectorfacility+0x00(SB), NOPTR, $8
DATA ·sinhvectorfacility+0x00(SB)/8, $·sinhTrampolineSetup(SB)

TEXT ·archTanh(SB), NOSPLIT, $0
	MOVD ·tanhvectorfacility+0x00(SB), R1
	BR   (R1)

TEXT ·tanhTrampolineSetup(SB), NOSPLIT, $0
	MOVB   ·hasVX(SB), R1
	CMPBEQ R1, $1, vectorimpl                // vectorfacility = 1, vector supported
	MOVD   $·tanhvectorfacility+0x00(SB), R1
	MOVD   $·tanh(SB), R2
	MOVD   R2, 0(R1)
	BR     ·tanh(SB)

vectorimpl:
	MOVD $·tanhvectorfacility+0x00(SB), R1
	MOVD $·tanhAsm(SB), R2
	MOVD R2, 0(R1)
	BR   ·tanhAsm(SB)

GLOBL ·tanhvectorfacility+0x00(SB), NOPTR, $8
DATA ·tanhvectorfacility+0x00(SB)/8, $·tanhTrampolineSetup(SB)

TEXT ·archLog1p(SB), NOSPLIT, $0
	MOVD ·log1pvectorfacility+0x00(SB), R1
	BR   (R1)

TEXT ·log1pTrampolineSetup(SB), NOSPLIT, $0
	MOVB   ·hasVX(SB), R1
	CMPBEQ R1, $1, vectorimpl                 // vectorfacility = 1, vector supported
	MOVD   $·log1pvectorfacility+0x00(SB), R1
	MOVD   $·log1p(SB), R2
	MOVD   R2, 0(R1)
	BR     ·log1p(SB)

vectorimpl:
	MOVD $·log1pvectorfacility+0x00(SB), R1
	MOVD $·log1pAsm(SB), R2
	MOVD R2, 0(R1)
	BR   ·log1pAsm(SB)

GLOBL ·log1pvectorfacility+0x00(SB), NOPTR, $8
DATA ·log1pvectorfacility+0x00(SB)/8, $·log1pTrampolineSetup(SB)

TEXT ·archAtanh(SB), NOSPLIT, $0
	MOVD ·atanhvectorfacility+0x00(SB), R1
	BR   (R1)

TEXT ·atanhTrampolineSetup(SB), NOSPLIT, $0
	MOVB   ·hasVX(SB), R1
	CMPBEQ R1, $1, vectorimpl                 // vectorfacility = 1, vector supported
	MOVD   $·atanhvectorfacility+0x00(SB), R1
	MOVD   $·atanh(SB), R2
	MOVD   R2, 0(R1)
	BR     ·atanh(SB)

vectorimpl:
	MOVD $·atanhvectorfacility+0x00(SB), R1
	MOVD $·atanhAsm(SB), R2
	MOVD R2, 0(R1)
	BR   ·atanhAsm(SB)

GLOBL ·atanhvectorfacility+0x00(SB), NOPTR, $8
DATA ·atanhvectorfacility+0x00(SB)/8, $·atanhTrampolineSetup(SB)

TEXT ·archAcos(SB), NOSPLIT, $0
	MOVD ·acosvectorfacility+0x00(SB), R1
	BR   (R1)

TEXT ·acosTrampolineSetup(SB), NOSPLIT, $0
	MOVB   ·hasVX(SB), R1
	CMPBEQ R1, $1, vectorimpl                // vectorfacility = 1, vector supported
	MOVD   $·acosvectorfacility+0x00(SB), R1
	MOVD   $·acos(SB), R2
	MOVD   R2, 0(R1)
	BR     ·acos(SB)

vectorimpl:
	MOVD $·acosvectorfacility+0x00(SB), R1
	MOVD $·acosAsm(SB), R2
	MOVD R2, 0(R1)
	BR   ·acosAsm(SB)

GLOBL ·acosvectorfacility+0x00(SB), NOPTR, $8
DATA ·acosvectorfacility+0x00(SB)/8, $·acosTrampolineSetup(SB)

TEXT ·archAsin(SB), NOSPLIT, $0
	MOVD ·asinvectorfacility+0x00(SB), R1
	BR   (R1)

TEXT ·asinTrampolineSetup(SB), NOSPLIT, $0
	MOVB   ·hasVX(SB), R1
	CMPBEQ R1, $1, vectorimpl                // vectorfacility = 1, vector supported
	MOVD   $·asinvectorfacility+0x00(SB), R1
	MOVD   $·asin(SB), R2
	MOVD   R2, 0(R1)
	BR     ·asin(SB)

vectorimpl:
	MOVD $·asinvectorfacility+0x00(SB), R1
	MOVD $·asinAsm(SB), R2
	MOVD R2, 0(R1)
	BR   ·asinAsm(SB)

GLOBL ·asinvectorfacility+0x00(SB), NOPTR, $8
DATA ·asinvectorfacility+0x00(SB)/8, $·asinTrampolineSetup(SB)

TEXT ·archAsinh(SB), NOSPLIT, $0
	MOVD ·asinhvectorfacility+0x00(SB), R1
	BR   (R1)

TEXT ·asinhTrampolineSetup(SB), NOSPLIT, $0
	MOVB   ·hasVX(SB), R1
	CMPBEQ R1, $1, vectorimpl                 // vectorfacility = 1, vector supported
	MOVD   $·asinhvectorfacility+0x00(SB), R1
	MOVD   $·asinh(SB), R2
	MOVD   R2, 0(R1)
	BR     ·asinh(SB)

vectorimpl:
	MOVD $·asinhvectorfacility+0x00(SB), R1
	MOVD $·asinhAsm(SB), R2
	MOVD R2, 0(R1)
	BR   ·asinhAsm(SB)

GLOBL ·asinhvectorfacility+0x00(SB), NOPTR, $8
DATA ·asinhvectorfacility+0x00(SB)/8, $·asinhTrampolineSetup(SB)

TEXT ·archAcosh(SB), NOSPLIT, $0
	MOVD ·acoshvectorfacility+0x00(SB), R1
	BR   (R1)

TEXT ·acoshTrampolineSetup(SB), NOSPLIT, $0
	MOVB   ·hasVX(SB), R1
	CMPBEQ R1, $1, vectorimpl                 // vectorfacility = 1, vector supported
	MOVD   $·acoshvectorfacility+0x00(SB), R1
	MOVD   $·acosh(SB), R2
	MOVD   R2, 0(R1)
	BR     ·acosh(SB)

vectorimpl:
	MOVD $·acoshvectorfacility+0x00(SB), R1
	MOVD $·acoshAsm(SB), R2
	MOVD R2, 0(R1)
	BR   ·acoshAsm(SB)

GLOBL ·acoshvectorfacility+0x00(SB), NOPTR, $8
DATA ·acoshvectorfacility+0x00(SB)/8, $·acoshTrampolineSetup(SB)

TEXT ·archErf(SB), NOSPLIT, $0
	MOVD ·erfvectorfacility+0x00(SB), R1
	BR   (R1)

TEXT ·erfTrampolineSetup(SB), NOSPLIT, $0
	MOVB   ·hasVX(SB), R1
	CMPBEQ R1, $1, vectorimpl               // vectorfacility = 1, vector supported
	MOVD   $·erfvectorfacility+0x00(SB), R1
	MOVD   $·erf(SB), R2
	MOVD   R2, 0(R1)
	BR     ·erf(SB)

vectorimpl:
	MOVD $·erfvectorfacility+0x00(SB), R1
	MOVD $·erfAsm(SB), R2
	MOVD R2, 0(R1)
	BR   ·erfAsm(SB)

GLOBL ·erfvectorfacility+0x00(SB), NOPTR, $8
DATA ·erfvectorfacility+0x00(SB)/8, $·erfTrampolineSetup(SB)

TEXT ·archErfc(SB), NOSPLIT, $0
	MOVD ·erfcvectorfacility+0x00(SB), R1
	BR   (R1)

TEXT ·erfcTrampolineSetup(SB), NOSPLIT, $0
	MOVB   ·hasVX(SB), R1
	CMPBEQ R1, $1, vectorimpl                // vectorfacility = 1, vector supported
	MOVD   $·erfcvectorfacility+0x00(SB), R1
	MOVD   $·erfc(SB), R2
	MOVD   R2, 0(R1)
	BR     ·erfc(SB)

vectorimpl:
	MOVD $·erfcvectorfacility+0x00(SB), R1
	MOVD $·erfcAsm(SB), R2
	MOVD R2, 0(R1)
	BR   ·erfcAsm(SB)

GLOBL ·erfcvectorfacility+0x00(SB), NOPTR, $8
DATA ·erfcvectorfacility+0x00(SB)/8, $·erfcTrampolineSetup(SB)

TEXT ·archAtan(SB), NOSPLIT, $0
	MOVD ·atanvectorfacility+0x00(SB), R1
	BR   (R1)

TEXT ·atanTrampolineSetup(SB), NOSPLIT, $0
	MOVB   ·hasVX(SB), R1
	CMPBEQ R1, $1, vectorimpl                // vectorfacility = 1, vector supported
	MOVD   $·atanvectorfacility+0x00(SB), R1
	MOVD   $·atan(SB), R2
	MOVD   R2, 0(R1)
	BR     ·atan(SB)

vectorimpl:
	MOVD $·atanvectorfacility+0x00(SB), R1
	MOVD $·atanAsm(SB), R2
	MOVD R2, 0(R1)
	BR   ·atanAsm(SB)

GLOBL ·atanvectorfacility+0x00(SB), NOPTR, $8
DATA ·atanvectorfacility+0x00(SB)/8, $·atanTrampolineSetup(SB)

TEXT ·archAtan2(SB), NOSPLIT, $0
	MOVD ·atan2vectorfacility+0x00(SB), R1
	BR   (R1)

TEXT ·atan2TrampolineSetup(SB), NOSPLIT, $0
	MOVB   ·hasVX(SB), R1
	CMPBEQ R1, $1, vectorimpl                 // vectorfacility = 1, vector supported
	MOVD   $·atan2vectorfacility+0x00(SB), R1
	MOVD   $·atan2(SB), R2
	MOVD   R2, 0(R1)
	BR     ·atan2(SB)

vectorimpl:
	MOVD $·atan2vectorfacility+0x00(SB), R1
	MOVD $·atan2Asm(SB), R2
	MOVD R2, 0(R1)
	BR   ·atan2Asm(SB)

GLOBL ·atan2vectorfacility+0x00(SB), NOPTR, $8
DATA ·atan2vectorfacility+0x00(SB)/8, $·atan2TrampolineSetup(SB)

TEXT ·archCbrt(SB), NOSPLIT, $0
	MOVD ·cbrtvectorfacility+0x00(SB), R1
	BR   (R1)

TEXT ·cbrtTrampolineSetup(SB), NOSPLIT, $0
	MOVB   ·hasVX(SB), R1
	CMPBEQ R1, $1, vectorimpl                // vectorfacility = 1, vector supported
	MOVD   $·cbrtvectorfacility+0x00(SB), R1
	MOVD   $·cbrt(SB), R2
	MOVD   R2, 0(R1)
	BR     ·cbrt(SB)

vectorimpl:
	MOVD $·cbrtvectorfacility+0x00(SB), R1
	MOVD $·cbrtAsm(SB), R2
	MOVD R2, 0(R1)
	BR   ·cbrtAsm(SB)

GLOBL ·cbrtvectorfacility+0x00(SB), NOPTR, $8
DATA ·cbrtvectorfacility+0x00(SB)/8, $·cbrtTrampolineSetup(SB)

TEXT ·archLog(SB), NOSPLIT, $0
	MOVD ·logvectorfacility+0x00(SB), R1
	BR   (R1)

TEXT ·logTrampolineSetup(SB), NOSPLIT, $0
	MOVB   ·hasVX(SB), R1
	CMPBEQ R1, $1, vectorimpl               // vectorfacility = 1, vector supported
	MOVD   $·logvectorfacility+0x00(SB), R1
	MOVD   $·log(SB), R2
	MOVD   R2, 0(R1)
	BR     ·log(SB)

vectorimpl:
	MOVD $·logvectorfacility+0x00(SB), R1
	MOVD $·logAsm(SB), R2
	MOVD R2, 0(R1)
	BR   ·logAsm(SB)

GLOBL ·logvectorfacility+0x00(SB), NOPTR, $8
DATA ·logvectorfacility+0x00(SB)/8, $·logTrampolineSetup(SB)

TEXT ·archTan(SB), NOSPLIT, $0
	MOVD ·tanvectorfacility+0x00(SB), R1
	BR   (R1)

TEXT ·tanTrampolineSetup(SB), NOSPLIT, $0
	MOVB   ·hasVX(SB), R1
	CMPBEQ R1, $1, vectorimpl               // vectorfacility = 1, vector supported
	MOVD   $·tanvectorfacility+0x00(SB), R1
	MOVD   $·tan(SB), R2
	MOVD   R2, 0(R1)
	BR     ·tan(SB)

vectorimpl:
	MOVD $·tanvectorfacility+0x00(SB), R1
	MOVD $·tanAsm(SB), R2
	MOVD R2, 0(R1)
	BR   ·tanAsm(SB)

GLOBL ·tanvectorfacility+0x00(SB), NOPTR, $8
DATA ·tanvectorfacility+0x00(SB)/8, $·tanTrampolineSetup(SB)

TEXT ·archExp(SB), NOSPLIT, $0
	MOVD ·expvectorfacility+0x00(SB), R1
	BR   (R1)

TEXT ·expTrampolineSetup(SB), NOSPLIT, $0
	MOVB   ·hasVX(SB), R1
	CMPBEQ R1, $1, vectorimpl               // vectorfacility = 1, vector supported
	MOVD   $·expvectorfacility+0x00(SB), R1
	MOVD   $·exp(SB), R2
	MOVD   R2, 0(R1)
	BR     ·exp(SB)

vectorimpl:
	MOVD $·expvectorfacility+0x00(SB), R1
	MOVD $·expAsm(SB), R2
	MOVD R2, 0(R1)
	BR   ·expAsm(SB)

GLOBL ·expvectorfacility+0x00(SB), NOPTR, $8
DATA ·expvectorfacility+0x00(SB)/8, $·expTrampolineSetup(SB)

TEXT ·archExpm1(SB), NOSPLIT, $0
	MOVD ·expm1vectorfacility+0x00(SB), R1
	BR   (R1)

TEXT ·expm1TrampolineSetup(SB), NOSPLIT, $0
	MOVB   ·hasVX(SB), R1
	CMPBEQ R1, $1, vectorimpl                 // vectorfacility = 1, vector supported
	MOVD   $·expm1vectorfacility+0x00(SB), R1
	MOVD   $·expm1(SB), R2
	MOVD   R2, 0(R1)
	BR     ·expm1(SB)

vectorimpl:
	MOVD $·expm1vectorfacility+0x00(SB), R1
	MOVD $·expm1Asm(SB), R2
	MOVD R2, 0(R1)
	BR   ·expm1Asm(SB)

GLOBL ·expm1vectorfacility+0x00(SB), NOPTR, $8
DATA ·expm1vectorfacility+0x00(SB)/8, $·expm1TrampolineSetup(SB)

TEXT ·archPow(SB), NOSPLIT, $0
	MOVD ·powvectorfacility+0x00(SB), R1
	BR   (R1)

TEXT ·powTrampolineSetup(SB), NOSPLIT, $0
	MOVB   ·hasVX(SB), R1
	CMPBEQ R1, $1, vectorimpl               // vectorfacility = 1, vector supported
	MOVD   $·powvectorfacility+0x00(SB), R1
	MOVD   $·pow(SB), R2
	MOVD   R2, 0(R1)
	BR     ·pow(SB)

vectorimpl:
	MOVD $·powvectorfacility+0x00(SB), R1
	MOVD $·powAsm(SB), R2
	MOVD R2, 0(R1)
	BR   ·powAsm(SB)

GLOBL ·powvectorfacility+0x00(SB), NOPTR, $8
DATA ·powvectorfacility+0x00(SB)/8, $·powTrampolineSetup(SB)


```

// === FILE: references!/go/src/math/tan.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

/*
	Floating-point tangent.
*/

// The original C code, the long comment, and the constants
// below were from http://netlib.sandia.gov/cephes/cmath/sin.c,
// available from http://www.netlib.org/cephes/cmath.tgz.
// The go code is a simplified version of the original C.
//
//      tan.c
//
//      Circular tangent
//
// SYNOPSIS:
//
// double x, y, tan();
// y = tan( x );
//
// DESCRIPTION:
//
// Returns the circular tangent of the radian argument x.
//
// Range reduction is modulo pi/4.  A rational function
//       x + x**3 P(x**2)/Q(x**2)
// is employed in the basic interval [0, pi/4].
//
// ACCURACY:
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    DEC      +-1.07e9      44000      4.1e-17     1.0e-17
//    IEEE     +-1.07e9      30000      2.9e-16     8.1e-17
//
// Partial loss of accuracy begins to occur at x = 2**30 = 1.074e9.  The loss
// is not gradual, but jumps suddenly to about 1 part in 10e7.  Results may
// be meaningless for x > 2**49 = 5.6e14.
// [Accuracy loss statement from sin.go comments.]
//
// Cephes Math Library Release 2.8:  June, 2000
// Copyright 1984, 1987, 1989, 1992, 2000 by Stephen L. Moshier
//
// The readme file at http://netlib.sandia.gov/cephes/ says:
//    Some software in this archive may be from the book _Methods and
// Programs for Mathematical Functions_ (Prentice-Hall or Simon & Schuster
// International, 1989) or from the Cephes Mathematical Library, a
// commercial product. In either event, it is copyrighted by the author.
// What you see here may be used freely but it comes with no support or
// guarantee.
//
//   The two known misprints in the book are repaired here in the
// source listings for the gamma function and the incomplete beta
// integral.
//
//   Stephen L. Moshier
//   moshier@na-net.ornl.gov

// tan coefficients
var _tanP = [...]float64{
	-1.30936939181383777646e4, // 0xc0c992d8d24f3f38
	1.15351664838587416140e6,  // 0x413199eca5fc9ddd
	-1.79565251976484877988e7, // 0xc1711fead3299176
}
var _tanQ = [...]float64{
	1.00000000000000000000e0,
	1.36812963470692954678e4,  // 0x40cab8a5eeb36572
	-1.32089234440210967447e6, // 0xc13427bc582abc96
	2.50083801823357915839e7,  // 0x4177d98fc2ead8ef
	-5.38695755929454629881e7, // 0xc189afe03cbe5a31
}

// Tan returns the tangent of the radian argument x.
//
// Special cases are:
//
//	Tan(±0) = ±0
//	Tan(±Inf) = NaN
//	Tan(NaN) = NaN
func Tan(x float64) float64 {
	if haveArchTan {
		return archTan(x)
	}
	return tan(x)
}

func tan(x float64) float64 {
	const (
		PI4A = 7.85398125648498535156e-1  // 0x3fe921fb40000000, Pi/4 split into three parts
		PI4B = 3.77489470793079817668e-8  // 0x3e64442d00000000,
		PI4C = 2.69515142907905952645e-15 // 0x3ce8469898cc5170,
	)
	// special cases
	switch {
	case x == 0 || IsNaN(x):
		return x // return ±0 || NaN()
	case IsInf(x, 0):
		return NaN()
	}

	// make argument positive but save the sign
	sign := false
	if x < 0 {
		x = -x
		sign = true
	}
	var j uint64
	var y, z float64
	if x >= reduceThreshold {
		j, z = trigReduce(x)
	} else {
		j = uint64(x * (4 / Pi)) // integer part of x/(Pi/4), as integer for tests on the phase angle
		y = float64(j)           // integer part of x/(Pi/4), as float

		/* map zeros and singularities to origin */
		if j&1 == 1 {
			j++
			y++
		}

		z = ((x - y*PI4A) - y*PI4B) - y*PI4C
	}
	zz := z * z

	if zz > 1e-14 {
		y = z + z*(zz*(((_tanP[0]*zz)+_tanP[1])*zz+_tanP[2])/((((zz+_tanQ[1])*zz+_tanQ[2])*zz+_tanQ[3])*zz+_tanQ[4]))
	} else {
		y = z
	}
	if j&2 == 2 {
		y = -1 / y
	}
	if sign {
		y = -y
	}
	return y
}

```

// === FILE: references!/go/src/math/tan_s390x.s ===
```text
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// Minimax polynomial approximations
DATA ·tanrodataL13<> + 0(SB)/8, $0.181017336383229927e-07
DATA ·tanrodataL13<> + 8(SB)/8, $-.256590857271311164e-03
DATA ·tanrodataL13<> + 16(SB)/8, $-.464359274328689195e+00
DATA ·tanrodataL13<> + 24(SB)/8, $1.0
DATA ·tanrodataL13<> + 32(SB)/8, $-.333333333333333464e+00
DATA ·tanrodataL13<> + 40(SB)/8, $0.245751217306830032e-01
DATA ·tanrodataL13<> + 48(SB)/8, $-.245391301343844510e-03
DATA ·tanrodataL13<> + 56(SB)/8, $0.214530914428992319e-01
DATA ·tanrodataL13<> + 64(SB)/8, $0.108285667160535624e-31
DATA ·tanrodataL13<> + 72(SB)/8, $0.612323399573676480e-16
DATA ·tanrodataL13<> + 80(SB)/8, $0.157079632679489656e+01
DATA ·tanrodataL13<> + 88(SB)/8, $0.636619772367581341e+00
GLOBL ·tanrodataL13<> + 0(SB), RODATA, $96

// Constants
DATA ·tanxnan<> + 0(SB)/8, $0x7ff8000000000000
GLOBL ·tanxnan<> + 0(SB), RODATA, $8
DATA ·tanxlim<> + 0(SB)/8, $0x432921fb54442d19
GLOBL ·tanxlim<> + 0(SB), RODATA, $8
DATA ·tanxadd<> + 0(SB)/8, $0xc338000000000000
GLOBL ·tanxadd<> + 0(SB), RODATA, $8

// Tan returns the tangent of the radian argument.
//
// Special cases are:
//      Tan(±0) = ±0
//      Tan(±Inf) = NaN
//      Tan(NaN) = NaN
// The algorithm used is minimax polynomial approximation using a table of
// polynomial coefficients determined with a Remez exchange algorithm.

TEXT	·tanAsm(SB), NOSPLIT, $0-16
	FMOVD	x+0(FP), F0
	//special case Tan(±0) = ±0
	FMOVD   $(0.0), F1
	FCMPU   F0, F1
	BEQ     atanIsZero

	MOVD	$·tanrodataL13<>+0(SB), R5
	LTDBR	F0, F0
	BLTU	L10
	FMOVD	F0, F2
L2:
	MOVD	$·tanxlim<>+0(SB), R1
	FMOVD	0(R1), F1
	FCMPU	F2, F1
	BGT	L9
	BVS	L11
	MOVD	$·tanxadd<>+0(SB), R1
	FMOVD	88(R5), F6
	FMOVD	0(R1), F4
	WFMSDB	V0, V6, V4, V6
	FMOVD	80(R5), F1
	FADD	F6, F4
	FMOVD	72(R5), F2
	FMSUB	F1, F4, F0
	FMOVD	64(R5), F3
	WFMADB	V4, V2, V0, V2
	FMOVD	56(R5), F1
	WFMADB	V4, V3, V2, V4
	FMUL	F2, F2
	VLEG	$0, 48(R5), V18
	LGDR	F6, R1
	FMOVD	40(R5), F5
	FMOVD	32(R5), F3
	FMADD	F1, F2, F3
	FMOVD	24(R5), F1
	FMOVD	16(R5), F7
	FMOVD	8(R5), F0
	WFMADB	V2, V7, V1, V7
	WFMADB	V2, V0, V5, V0
	WFMDB	V2, V2, V1
	FMOVD	0(R5), F5
	WFLCDB	V4, V16
	WFMADB	V2, V5, V18, V5
	WFMADB	V1, V0, V7, V0
	TMLL	R1, $1
	WFMADB	V1, V5, V3, V1
	BNE	L12
	WFDDB	V0, V1, V0
	WFMDB	V2, V16, V2
	WFMADB	V2, V0, V4, V0
	LCDBR	F0, F0
	FMOVD	F0, ret+8(FP)
	RET
L12:
	WFMSDB	V2, V1, V0, V2
	WFMDB	V16, V2, V2
	FDIV	F2, F0
	FMOVD	F0, ret+8(FP)
	RET
L11:
	MOVD	$·tanxnan<>+0(SB), R1
	FMOVD	0(R1), F0
	FMOVD	F0, ret+8(FP)
	RET
L10:
	LCDBR	F0, F2
	BR	L2
L9:
	BR	·tan(SB)
atanIsZero:
	FMOVD	F0, ret+8(FP)
	RET

```

// === FILE: references!/go/src/math/tanh.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// The original C code, the long comment, and the constants
// below were from http://netlib.sandia.gov/cephes/cmath/sin.c,
// available from http://www.netlib.org/cephes/cmath.tgz.
// The go code is a simplified version of the original C.
//      tanh.c
//
//      Hyperbolic tangent
//
// SYNOPSIS:
//
// double x, y, tanh();
//
// y = tanh( x );
//
// DESCRIPTION:
//
// Returns hyperbolic tangent of argument in the range MINLOG to MAXLOG.
//      MAXLOG = 8.8029691931113054295988e+01 = log(2**127)
//      MINLOG = -8.872283911167299960540e+01 = log(2**-128)
//
// A rational function is used for |x| < 0.625.  The form
// x + x**3 P(x)/Q(x) of Cody & Waite is employed.
// Otherwise,
//      tanh(x) = sinh(x)/cosh(x) = 1  -  2/(exp(2x) + 1).
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    IEEE      -2,2        30000       2.5e-16     5.8e-17
//
// Cephes Math Library Release 2.8:  June, 2000
// Copyright 1984, 1987, 1989, 1992, 2000 by Stephen L. Moshier
//
// The readme file at http://netlib.sandia.gov/cephes/ says:
//    Some software in this archive may be from the book _Methods and
// Programs for Mathematical Functions_ (Prentice-Hall or Simon & Schuster
// International, 1989) or from the Cephes Mathematical Library, a
// commercial product. In either event, it is copyrighted by the author.
// What you see here may be used freely but it comes with no support or
// guarantee.
//
//   The two known misprints in the book are repaired here in the
// source listings for the gamma function and the incomplete beta
// integral.
//
//   Stephen L. Moshier
//   moshier@na-net.ornl.gov
//

var tanhP = [...]float64{
	-9.64399179425052238628e-1,
	-9.92877231001918586564e1,
	-1.61468768441708447952e3,
}
var tanhQ = [...]float64{
	1.12811678491632931402e2,
	2.23548839060100448583e3,
	4.84406305325125486048e3,
}

// Tanh returns the hyperbolic tangent of x.
//
// Special cases are:
//
//	Tanh(±0) = ±0
//	Tanh(±Inf) = ±1
//	Tanh(NaN) = NaN
func Tanh(x float64) float64 {
	if haveArchTanh {
		return archTanh(x)
	}
	return tanh(x)
}

func tanh(x float64) float64 {
	const MAXLOG = 8.8029691931113054295988e+01 // log(2**127)
	z := Abs(x)
	switch {
	case z > 0.5*MAXLOG:
		if x < 0 {
			return -1
		}
		return 1
	case z >= 0.625:
		s := Exp(2 * z)
		z = 1 - 2/(s+1)
		if x < 0 {
			z = -z
		}
	default:
		if x == 0 {
			return x
		}
		s := x * x
		z = x + x*s*((tanhP[0]*s+tanhP[1])*s+tanhP[2])/(((s+tanhQ[0])*s+tanhQ[1])*s+tanhQ[2])
	}
	return z
}

```

// === FILE: references!/go/src/math/tanh_s390x.s ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// Minimax polynomial approximations
DATA tanhrodataL18<>+0(SB)/8, $-1.0
DATA tanhrodataL18<>+8(SB)/8, $-2.0
DATA tanhrodataL18<>+16(SB)/8, $1.0
DATA tanhrodataL18<>+24(SB)/8, $2.0
DATA tanhrodataL18<>+32(SB)/8, $0.20000000000000011868E+01
DATA tanhrodataL18<>+40(SB)/8, $0.13333333333333341256E+01
DATA tanhrodataL18<>+48(SB)/8, $0.26666666663549111502E+00
DATA tanhrodataL18<>+56(SB)/8, $0.66666666658721844678E+00
DATA tanhrodataL18<>+64(SB)/8, $0.88890217768964374821E-01
DATA tanhrodataL18<>+72(SB)/8, $0.25397199429103821138E-01
DATA tanhrodataL18<>+80(SB)/8, $-.346573590279972643E+00
DATA tanhrodataL18<>+88(SB)/8, $20.E0
GLOBL tanhrodataL18<>+0(SB), RODATA, $96

// Constants
DATA tanhrlog2<>+0(SB)/8, $0x4007154760000000
GLOBL tanhrlog2<>+0(SB), RODATA, $8
DATA tanhxadd<>+0(SB)/8, $0xc2f0000100003ff0
GLOBL tanhxadd<>+0(SB), RODATA, $8
DATA tanhxmone<>+0(SB)/8, $-1.0
GLOBL tanhxmone<>+0(SB), RODATA, $8
DATA tanhxzero<>+0(SB)/8, $0
GLOBL tanhxzero<>+0(SB), RODATA, $8

// Polynomial coefficients
DATA tanhtab<>+0(SB)/8, $0.000000000000000000E+00
DATA tanhtab<>+8(SB)/8, $-.171540871271399150E-01
DATA tanhtab<>+16(SB)/8, $-.306597931864376363E-01
DATA tanhtab<>+24(SB)/8, $-.410200970469965021E-01
DATA tanhtab<>+32(SB)/8, $-.486343079978231466E-01
DATA tanhtab<>+40(SB)/8, $-.538226193725835820E-01
DATA tanhtab<>+48(SB)/8, $-.568439602538111520E-01
DATA tanhtab<>+56(SB)/8, $-.579091847395528847E-01
DATA tanhtab<>+64(SB)/8, $-.571909584179366341E-01
DATA tanhtab<>+72(SB)/8, $-.548312665987204407E-01
DATA tanhtab<>+80(SB)/8, $-.509471843643441085E-01
DATA tanhtab<>+88(SB)/8, $-.456353588448863359E-01
DATA tanhtab<>+96(SB)/8, $-.389755254243262365E-01
DATA tanhtab<>+104(SB)/8, $-.310332908285244231E-01
DATA tanhtab<>+112(SB)/8, $-.218623539150173528E-01
DATA tanhtab<>+120(SB)/8, $-.115062908917949451E-01
GLOBL tanhtab<>+0(SB), RODATA, $128

// Tanh returns the hyperbolic tangent of the argument.
//
// Special cases are:
//      Tanh(±0) = ±0
//      Tanh(±Inf) = ±1
//      Tanh(NaN) = NaN
// The algorithm used is minimax polynomial approximation using a table of
// polynomial coefficients determined with a Remez exchange algorithm.

TEXT ·tanhAsm(SB),NOSPLIT,$0-16
	FMOVD   x+0(FP), F0
	// special case Tanh(±0) = ±0
	FMOVD   $(0.0), F1
	FCMPU   F0, F1
	BEQ     tanhIsZero
	MOVD    $tanhrodataL18<>+0(SB), R5
	LTDBR	F0, F0
	MOVD    $0x4034000000000000, R1
	BLTU    L15
	FMOVD   F0, F1
L2:
	MOVD    $tanhxadd<>+0(SB), R2
	FMOVD   0(R2), F2
	MOVD    tanhrlog2<>+0(SB), R2
	LDGR    R2, F4
	WFMSDB  V0, V4, V2, V4
	MOVD    $tanhtab<>+0(SB), R3
	LGDR    F4, R2
	RISBGZ	$57, $60, $3, R2, R4
	WORD    $0xED105058     //cdb %f1,.L19-.L18(%r5)
	BYTE    $0x00
	BYTE    $0x19
	RISBGN	$0, $15, $48, R2, R1
	WORD    $0x68543000     //ld %f5,0(%r4,%r3)
	LDGR    R1, F6
	BLT     L3
	MOVD    $tanhxzero<>+0(SB), R1
	FMOVD   0(R1), F2
	WFCHDBS V0, V2, V4
	BEQ     L9
	WFCHDBS V2, V0, V2
	BNE     L1
	MOVD    $tanhxmone<>+0(SB), R1
	FMOVD   0(R1), F0
	FMOVD   F0, ret+8(FP)
	RET

L3:
	FADD    F4, F2
	FMOVD   tanhrodataL18<>+80(SB), F4
	FMADD   F4, F2, F0
	FMOVD   tanhrodataL18<>+72(SB), F1
	WFMDB   V0, V0, V3
	FMOVD   tanhrodataL18<>+64(SB), F2
	WFMADB  V0, V1, V2, V1
	FMOVD   tanhrodataL18<>+56(SB), F4
	FMOVD   tanhrodataL18<>+48(SB), F2
	WFMADB  V1, V3, V4, V1
	FMOVD   tanhrodataL18<>+40(SB), F4
	WFMADB  V3, V2, V4, V2
	FMOVD   tanhrodataL18<>+32(SB), F4
	WORD    $0xB9270022     //lhr %r2,%r2
	WFMADB  V3, V1, V4, V1
	FMOVD   tanhrodataL18<>+24(SB), F4
	WFMADB  V3, V2, V4, V3
	WFMADB  V0, V5, V0, V2
	WFMADB  V0, V1, V3, V0
	WORD    $0xA7183ECF     //lhi %r1,16079
	WFMADB  V0, V2, V5, V2
	FMUL    F6, F2
	MOVW    R2, R10
	MOVW    R1, R11
	CMPBLE  R10, R11, L16
	FMOVD   F6, F0
	WORD    $0xED005010     //adb %f0,.L28-.L18(%r5)
	BYTE    $0x00
	BYTE    $0x1A
	WORD    $0xA7184330     //lhi %r1,17200
	FADD    F2, F0
	MOVW    R2, R10
	MOVW    R1, R11
	CMPBGT  R10, R11, L17
	WORD    $0xED605010     //sdb %f6,.L28-.L18(%r5)
	BYTE    $0x00
	BYTE    $0x1B
	FADD    F6, F2
	WFDDB   V0, V2, V0
	FMOVD   F0, ret+8(FP)
	RET

L9:
	FMOVD   tanhrodataL18<>+16(SB), F0
L1:
	FMOVD   F0, ret+8(FP)
	RET

L15:
	FNEG    F0, F1
	BR      L2
L16:
	FADD    F6, F2
	FMOVD   tanhrodataL18<>+8(SB), F0
	FMADD   F4, F2, F0
	FMOVD   tanhrodataL18<>+0(SB), F4
	FNEG    F0, F0
	WFMADB  V0, V2, V4, V0
	FMOVD   F0, ret+8(FP)
	RET

L17:
	WFDDB   V0, V4, V0
	FMOVD   tanhrodataL18<>+16(SB), F2
	WFSDB   V0, V2, V0
	FMOVD   F0, ret+8(FP)
	RET

tanhIsZero:      //return ±0
	FMOVD   F0, ret+8(FP)
	RET

```

// === FILE: references!/go/src/math/trig_reduce.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

import (
	"math/bits"
)

// reduceThreshold is the maximum value of x where the reduction using Pi/4
// in 3 float64 parts still gives accurate results. This threshold
// is set by y*C being representable as a float64 without error
// where y is given by y = floor(x * (4 / Pi)) and C is the leading partial
// terms of 4/Pi. Since the leading terms (PI4A and PI4B in sin.go) have 30
// and 32 trailing zero bits, y should have less than 30 significant bits.
//
//	y < 1<<30  -> floor(x*4/Pi) < 1<<30 -> x < (1<<30 - 1) * Pi/4
//
// So, conservatively we can take x < 1<<29.
// Above this threshold Payne-Hanek range reduction must be used.
const reduceThreshold = 1 << 29

// trigReduce implements Payne-Hanek range reduction by Pi/4
// for x > 0. It returns the integer part mod 8 (j) and
// the fractional part (z) of x / (Pi/4).
// The implementation is based on:
// "ARGUMENT REDUCTION FOR HUGE ARGUMENTS: Good to the Last Bit"
// K. C. Ng et al, March 24, 1992
// The simulated multi-precision calculation of x*B uses 64-bit integer arithmetic.
func trigReduce(x float64) (j uint64, z float64) {
	const PI4 = Pi / 4
	if x < PI4 {
		return 0, x
	}
	// Extract out the integer and exponent such that,
	// x = ix * 2 ** exp.
	ix := Float64bits(x)
	exp := int(ix>>shift&mask) - bias - shift
	ix &^= mask << shift
	ix |= 1 << shift
	// Use the exponent to extract the 3 appropriate uint64 digits from mPi4,
	// B ~ (z0, z1, z2), such that the product leading digit has the exponent -61.
	// Note, exp >= -53 since x >= PI4 and exp < 971 for maximum float64.
	digit, bitshift := uint(exp+61)/64, uint(exp+61)%64
	z0 := (mPi4[digit] << bitshift) | (mPi4[digit+1] >> (64 - bitshift))
	z1 := (mPi4[digit+1] << bitshift) | (mPi4[digit+2] >> (64 - bitshift))
	z2 := (mPi4[digit+2] << bitshift) | (mPi4[digit+3] >> (64 - bitshift))
	// Multiply mantissa by the digits and extract the upper two digits (hi, lo).
	z2hi, _ := bits.Mul64(z2, ix)
	z1hi, z1lo := bits.Mul64(z1, ix)
	z0lo := z0 * ix
	lo, c := bits.Add64(z1lo, z2hi, 0)
	hi, _ := bits.Add64(z0lo, z1hi, c)
	// The top 3 bits are j.
	j = hi >> 61
	// Extract the fraction and find its magnitude.
	hi = hi<<3 | lo>>61
	lz := uint(bits.LeadingZeros64(hi))
	e := uint64(bias - (lz + 1))
	// Clear implicit mantissa bit and shift into place.
	hi = (hi << (lz + 1)) | (lo >> (64 - (lz + 1)))
	hi >>= 64 - shift
	// Include the exponent and convert to a float.
	hi |= e << shift
	z = Float64frombits(hi)
	// Map zeros to origin.
	if j&1 == 1 {
		j++
		j &= 7
		z--
	}
	// Multiply the fractional part by pi/4.
	return j, z * PI4
}

// mPi4 is the binary digits of 4/pi as a uint64 array,
// that is, 4/pi = Sum mPi4[i]*2^(-64*i)
// 19 64-bit digits and the leading one bit give 1217 bits
// of precision to handle the largest possible float64 exponent.
var mPi4 = [...]uint64{
	0x0000000000000001,
	0x45f306dc9c882a53,
	0xf84eafa3ea69bb81,
	0xb6c52b3278872083,
	0xfca2c757bd778ac3,
	0x6e48dc74849ba5c0,
	0x0c925dd413a32439,
	0xfc3bd63962534e7d,
	0xd1046bea5d768909,
	0xd338e04d68befc82,
	0x7323ac7306a673e9,
	0x3908bf177bf25076,
	0x3ff12fffbc0b301f,
	0xde5e2316b414da3e,
	0xda6cfd9e4f96136e,
	0x9e8c7ecd3cbfd45a,
	0xea4f758fd7cbe2f6,
	0x7a0e73ef14a525d4,
	0xd7f6bf623f1aba10,
	0xac06608df8f6d757,
}

```

// === FILE: references!/go/src/math/unsafe.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

import "unsafe"

// Despite being an exported symbol,
// Float32bits is linknamed by widely used packages.
// Notable members of the hall of shame include:
//   - gitee.com/quant1x/num
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
// Note that this comment is not part of the doc comment.
//
//go:linkname Float32bits

// Float32bits returns the IEEE 754 binary representation of f,
// with the sign bit of f and the result in the same bit position.
// Float32bits(Float32frombits(x)) == x.
func Float32bits(f float32) uint32 { return *(*uint32)(unsafe.Pointer(&f)) }

// Float32frombits returns the floating-point number corresponding
// to the IEEE 754 binary representation b, with the sign bit of b
// and the result in the same bit position.
// Float32frombits(Float32bits(x)) == x.
func Float32frombits(b uint32) float32 { return *(*float32)(unsafe.Pointer(&b)) }

// Float64bits returns the IEEE 754 binary representation of f,
// with the sign bit of f and the result in the same bit position,
// and Float64bits(Float64frombits(x)) == x.
func Float64bits(f float64) uint64 { return *(*uint64)(unsafe.Pointer(&f)) }

// Float64frombits returns the floating-point number corresponding
// to the IEEE 754 binary representation b, with the sign bit of b
// and the result in the same bit position.
// Float64frombits(Float64bits(x)) == x.
func Float64frombits(b uint64) float64 { return *(*float64)(unsafe.Pointer(&b)) }

```

