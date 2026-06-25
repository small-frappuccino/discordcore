# Domain Architecture: runtime/cgo

## Layout Topology
```text
runtime/cgo/
├── abi_amd64.h
├── abi_arm64.h
├── abi_loong64.h
├── abi_ppc64x.h
├── abi_riscv64.h
├── asm_386.s
├── asm_amd64.s
├── asm_arm.s
├── asm_arm64.s
├── asm_loong64.s
├── asm_mips64x.s
├── asm_mipsx.s
├── asm_ppc64x.s
├── asm_riscv64.s
├── asm_s390x.s
├── asm_wasm.s
├── callbacks.go
├── callbacks_aix.go
├── callbacks_traceback.go
├── cgo.go
├── clearenv.go
├── dragonfly.go
├── freebsd.go
├── gcc_386.S
├── gcc_aix_ppc64.S
├── gcc_aix_ppc64.c
├── gcc_amd64.S
├── gcc_android.c
├── gcc_arm.S
├── gcc_arm64.S
├── gcc_clearenv.c
├── gcc_context.c
├── gcc_fatalf.c
├── gcc_freebsd_sigaction.c
├── gcc_libinit_unix.c
├── gcc_libinit_windows.c
├── gcc_linux_ppc64x.S
├── gcc_loong64.S
├── gcc_mips64x.S
├── gcc_mipsx.S
├── gcc_mmap.c
├── gcc_netbsd.c
├── gcc_riscv64.S
├── gcc_s390x.S
├── gcc_setenv.c
├── gcc_sigaction.c
├── gcc_traceback.c
├── gcc_unix.c
├── gcc_util.c
├── handle.go
├── iscgo.go
├── libcgo.h
├── libcgo_unix.h
├── linux.go
├── linux_syscall.c
├── mmap.go
├── netbsd.go
├── openbsd.go
├── pthread_unix.c
├── setenv.go
├── sigaction.go
└── windows.go
```

## Source Stream Aggregation

// === FILE: references!/go/src/runtime/cgo/abi_amd64.h ===
```text
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Macros for transitioning from the host ABI to Go ABI0.
//
// These save the frame pointer, so in general, functions that use
// these should have zero frame size to suppress the automatic frame
// pointer, though it's harmless to not do this.

#ifdef GOOS_windows

// REGS_HOST_TO_ABI0_STACK is the stack bytes used by
// PUSH_REGS_HOST_TO_ABI0.
#define REGS_HOST_TO_ABI0_STACK (28*8 + 8)

// PUSH_REGS_HOST_TO_ABI0 prepares for transitioning from
// the host ABI to Go ABI0 code. It saves all registers that are
// callee-save in the host ABI and caller-save in Go ABI0 and prepares
// for entry to Go.
//
// Save DI SI BP BX R12 R13 R14 R15 X6-X15 registers and the DF flag.
// Clear the DF flag for the Go ABI.
// MXCSR matches the Go ABI, so we don't have to set that,
// and Go doesn't modify it, so we don't have to save it.
#define PUSH_REGS_HOST_TO_ABI0()	\
	PUSHFQ			\
	CLD			\
	ADJSP	$(REGS_HOST_TO_ABI0_STACK - 8)	\
	MOVQ	DI, (0*0)(SP)	\
	MOVQ	SI, (1*8)(SP)	\
	MOVQ	BP, (2*8)(SP)	\
	MOVQ	BX, (3*8)(SP)	\
	MOVQ	R12, (4*8)(SP)	\
	MOVQ	R13, (5*8)(SP)	\
	MOVQ	R14, (6*8)(SP)	\
	MOVQ	R15, (7*8)(SP)	\
	MOVUPS	X6, (8*8)(SP)	\
	MOVUPS	X7, (10*8)(SP)	\
	MOVUPS	X8, (12*8)(SP)	\
	MOVUPS	X9, (14*8)(SP)	\
	MOVUPS	X10, (16*8)(SP)	\
	MOVUPS	X11, (18*8)(SP)	\
	MOVUPS	X12, (20*8)(SP)	\
	MOVUPS	X13, (22*8)(SP)	\
	MOVUPS	X14, (24*8)(SP)	\
	MOVUPS	X15, (26*8)(SP)

#define POP_REGS_HOST_TO_ABI0()	\
	MOVQ	(0*0)(SP), DI	\
	MOVQ	(1*8)(SP), SI	\
	MOVQ	(2*8)(SP), BP	\
	MOVQ	(3*8)(SP), BX	\
	MOVQ	(4*8)(SP), R12	\
	MOVQ	(5*8)(SP), R13	\
	MOVQ	(6*8)(SP), R14	\
	MOVQ	(7*8)(SP), R15	\
	MOVUPS	(8*8)(SP), X6	\
	MOVUPS	(10*8)(SP), X7	\
	MOVUPS	(12*8)(SP), X8	\
	MOVUPS	(14*8)(SP), X9	\
	MOVUPS	(16*8)(SP), X10	\
	MOVUPS	(18*8)(SP), X11	\
	MOVUPS	(20*8)(SP), X12	\
	MOVUPS	(22*8)(SP), X13	\
	MOVUPS	(24*8)(SP), X14	\
	MOVUPS	(26*8)(SP), X15	\
	ADJSP	$-(REGS_HOST_TO_ABI0_STACK - 8)	\
	POPFQ

#else
// SysV ABI

#define REGS_HOST_TO_ABI0_STACK (6*8)

// SysV MXCSR matches the Go ABI, so we don't have to set that,
// and Go doesn't modify it, so we don't have to save it.
// Both SysV and Go require DF to be cleared, so that's already clear.
// The SysV and Go frame pointer conventions are compatible.
#define PUSH_REGS_HOST_TO_ABI0()	\
	ADJSP	$(REGS_HOST_TO_ABI0_STACK)	\
	MOVQ	BP, (5*8)(SP)	\
	LEAQ	(5*8)(SP), BP	\
	MOVQ	BX, (0*8)(SP)	\
	MOVQ	R12, (1*8)(SP)	\
	MOVQ	R13, (2*8)(SP)	\
	MOVQ	R14, (3*8)(SP)	\
	MOVQ	R15, (4*8)(SP)

#define POP_REGS_HOST_TO_ABI0()	\
	MOVQ	(0*8)(SP), BX	\
	MOVQ	(1*8)(SP), R12	\
	MOVQ	(2*8)(SP), R13	\
	MOVQ	(3*8)(SP), R14	\
	MOVQ	(4*8)(SP), R15	\
	MOVQ	(5*8)(SP), BP	\
	ADJSP	$-(REGS_HOST_TO_ABI0_STACK)

#endif

```

// === FILE: references!/go/src/runtime/cgo/abi_arm64.h ===
```text
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Macros for transitioning from the host ABI to Go ABI0.
//
// These macros save and restore the callee-saved registers
// from the stack, but they don't adjust stack pointer, so
// the user should prepare stack space in advance.
// SAVE_R19_TO_R28(offset) saves R19 ~ R28 to the stack space
// of ((offset)+0*8)(RSP) ~ ((offset)+9*8)(RSP).
//
// SAVE_F8_TO_F15(offset) saves F8 ~ F15 to the stack space
// of ((offset)+0*8)(RSP) ~ ((offset)+7*8)(RSP).
//
// R29 is not saved because Go will save and restore it.

#define SAVE_R19_TO_R28(offset) \
	STP	(R19, R20), ((offset)+0*8)(RSP) \
	STP	(R21, R22), ((offset)+2*8)(RSP) \
	STP	(R23, R24), ((offset)+4*8)(RSP) \
	STP	(R25, R26), ((offset)+6*8)(RSP) \
	STP	(R27, g), ((offset)+8*8)(RSP)

#define RESTORE_R19_TO_R28(offset) \
	LDP	((offset)+0*8)(RSP), (R19, R20) \
	LDP	((offset)+2*8)(RSP), (R21, R22) \
	LDP	((offset)+4*8)(RSP), (R23, R24) \
	LDP	((offset)+6*8)(RSP), (R25, R26) \
	LDP	((offset)+8*8)(RSP), (R27, g) /* R28 */

#define SAVE_F8_TO_F15(offset) \
	FSTPD	(F8, F9), ((offset)+0*8)(RSP) \
	FSTPD	(F10, F11), ((offset)+2*8)(RSP) \
	FSTPD	(F12, F13), ((offset)+4*8)(RSP) \
	FSTPD	(F14, F15), ((offset)+6*8)(RSP)

#define RESTORE_F8_TO_F15(offset) \
	FLDPD	((offset)+0*8)(RSP), (F8, F9) \
	FLDPD	((offset)+2*8)(RSP), (F10, F11) \
	FLDPD	((offset)+4*8)(RSP), (F12, F13) \
	FLDPD	((offset)+6*8)(RSP), (F14, F15)


```

// === FILE: references!/go/src/runtime/cgo/abi_loong64.h ===
```text
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Macros for transitioning from the host ABI to Go ABI0.
//
// These macros save and restore the callee-saved registers
// from the stack, but they don't adjust stack pointer, so
// the user should prepare stack space in advance.
// SAVE_R22_TO_R31(offset) saves R22 ~ R31 to the stack space
// of ((offset)+0*8)(R3) ~ ((offset)+9*8)(R3).
//
// SAVE_F24_TO_F31(offset) saves F24 ~ F31 to the stack space
// of ((offset)+0*8)(R3) ~ ((offset)+7*8)(R3).
//
// Note: g is R22

#define SAVE_R22_TO_R31(offset)	\
	MOVV	g,   ((offset)+(0*8))(R3)	\
	MOVV	R23, ((offset)+(1*8))(R3)	\
	MOVV	R24, ((offset)+(2*8))(R3)	\
	MOVV	R25, ((offset)+(3*8))(R3)	\
	MOVV	R26, ((offset)+(4*8))(R3)	\
	MOVV	R27, ((offset)+(5*8))(R3)	\
	MOVV	R28, ((offset)+(6*8))(R3)	\
	MOVV	R29, ((offset)+(7*8))(R3)	\
	MOVV	R30, ((offset)+(8*8))(R3)	\
	MOVV	R31, ((offset)+(9*8))(R3)

#define SAVE_F24_TO_F31(offset)	\
	MOVD	F24, ((offset)+(0*8))(R3)	\
	MOVD	F25, ((offset)+(1*8))(R3)	\
	MOVD	F26, ((offset)+(2*8))(R3)	\
	MOVD	F27, ((offset)+(3*8))(R3)	\
	MOVD	F28, ((offset)+(4*8))(R3)	\
	MOVD	F29, ((offset)+(5*8))(R3)	\
	MOVD	F30, ((offset)+(6*8))(R3)	\
	MOVD	F31, ((offset)+(7*8))(R3)

#define RESTORE_R22_TO_R31(offset)	\
	MOVV	((offset)+(0*8))(R3),  g	\
	MOVV	((offset)+(1*8))(R3), R23	\
	MOVV	((offset)+(2*8))(R3), R24	\
	MOVV	((offset)+(3*8))(R3), R25	\
	MOVV	((offset)+(4*8))(R3), R26	\
	MOVV	((offset)+(5*8))(R3), R27	\
	MOVV	((offset)+(6*8))(R3), R28	\
	MOVV	((offset)+(7*8))(R3), R29	\
	MOVV	((offset)+(8*8))(R3), R30	\
	MOVV	((offset)+(9*8))(R3), R31

#define RESTORE_F24_TO_F31(offset)	\
	MOVD	((offset)+(0*8))(R3), F24	\
	MOVD	((offset)+(1*8))(R3), F25	\
	MOVD	((offset)+(2*8))(R3), F26	\
	MOVD	((offset)+(3*8))(R3), F27	\
	MOVD	((offset)+(4*8))(R3), F28	\
	MOVD	((offset)+(5*8))(R3), F29	\
	MOVD	((offset)+(6*8))(R3), F30	\
	MOVD	((offset)+(7*8))(R3), F31

```

// === FILE: references!/go/src/runtime/cgo/abi_ppc64x.h ===
```text
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Macros for transitioning from the host ABI to Go ABI
//
// On PPC64/ELFv2 targets, the following registers are callee
// saved when called from C. They must be preserved before
// calling into Go which does not preserve any of them.
//
//	R14-R31
//	CR2-4
//	VR20-31
//	F14-F31
//
// xcoff(aix) and ELFv1 are similar, but may only require a
// subset of these.
//
// These macros assume a 16 byte aligned stack pointer. This
// is required by ELFv1, ELFv2, and AIX PPC64.

#define SAVE_GPR_SIZE (18*8)
#define SAVE_GPR(offset)               \
	MOVD	R14, (offset+8*0)(R1)  \
	MOVD	R15, (offset+8*1)(R1)  \
	MOVD	R16, (offset+8*2)(R1)  \
	MOVD	R17, (offset+8*3)(R1)  \
	MOVD	R18, (offset+8*4)(R1)  \
	MOVD	R19, (offset+8*5)(R1)  \
	MOVD	R20, (offset+8*6)(R1)  \
	MOVD	R21, (offset+8*7)(R1)  \
	MOVD	R22, (offset+8*8)(R1)  \
	MOVD	R23, (offset+8*9)(R1)  \
	MOVD	R24, (offset+8*10)(R1) \
	MOVD	R25, (offset+8*11)(R1) \
	MOVD	R26, (offset+8*12)(R1) \
	MOVD	R27, (offset+8*13)(R1) \
	MOVD	R28, (offset+8*14)(R1) \
	MOVD	R29, (offset+8*15)(R1) \
	MOVD	g,   (offset+8*16)(R1) \
	MOVD	R31, (offset+8*17)(R1)

#define RESTORE_GPR(offset)            \
	MOVD	(offset+8*0)(R1), R14  \
	MOVD	(offset+8*1)(R1), R15  \
	MOVD	(offset+8*2)(R1), R16  \
	MOVD	(offset+8*3)(R1), R17  \
	MOVD	(offset+8*4)(R1), R18  \
	MOVD	(offset+8*5)(R1), R19  \
	MOVD	(offset+8*6)(R1), R20  \
	MOVD	(offset+8*7)(R1), R21  \
	MOVD	(offset+8*8)(R1), R22  \
	MOVD	(offset+8*9)(R1), R23  \
	MOVD	(offset+8*10)(R1), R24 \
	MOVD	(offset+8*11)(R1), R25 \
	MOVD	(offset+8*12)(R1), R26 \
	MOVD	(offset+8*13)(R1), R27 \
	MOVD	(offset+8*14)(R1), R28 \
	MOVD	(offset+8*15)(R1), R29 \
	MOVD	(offset+8*16)(R1), g   \
	MOVD	(offset+8*17)(R1), R31

#define SAVE_FPR_SIZE (18*8)
#define SAVE_FPR(offset)               \
	FMOVD	F14, (offset+8*0)(R1)  \
	FMOVD	F15, (offset+8*1)(R1)  \
	FMOVD	F16, (offset+8*2)(R1)  \
	FMOVD	F17, (offset+8*3)(R1)  \
	FMOVD	F18, (offset+8*4)(R1)  \
	FMOVD	F19, (offset+8*5)(R1)  \
	FMOVD	F20, (offset+8*6)(R1)  \
	FMOVD	F21, (offset+8*7)(R1)  \
	FMOVD	F22, (offset+8*8)(R1)  \
	FMOVD	F23, (offset+8*9)(R1)  \
	FMOVD	F24, (offset+8*10)(R1) \
	FMOVD	F25, (offset+8*11)(R1) \
	FMOVD	F26, (offset+8*12)(R1) \
	FMOVD	F27, (offset+8*13)(R1) \
	FMOVD	F28, (offset+8*14)(R1) \
	FMOVD	F29, (offset+8*15)(R1) \
	FMOVD	F30, (offset+8*16)(R1) \
	FMOVD	F31, (offset+8*17)(R1)

#define RESTORE_FPR(offset)            \
	FMOVD	(offset+8*0)(R1), F14  \
	FMOVD	(offset+8*1)(R1), F15  \
	FMOVD	(offset+8*2)(R1), F16  \
	FMOVD	(offset+8*3)(R1), F17  \
	FMOVD	(offset+8*4)(R1), F18  \
	FMOVD	(offset+8*5)(R1), F19  \
	FMOVD	(offset+8*6)(R1), F20  \
	FMOVD	(offset+8*7)(R1), F21  \
	FMOVD	(offset+8*8)(R1), F22  \
	FMOVD	(offset+8*9)(R1), F23  \
	FMOVD	(offset+8*10)(R1), F24 \
	FMOVD	(offset+8*11)(R1), F25 \
	FMOVD	(offset+8*12)(R1), F26 \
	FMOVD	(offset+8*13)(R1), F27 \
	FMOVD	(offset+8*14)(R1), F28 \
	FMOVD	(offset+8*15)(R1), F29 \
	FMOVD	(offset+8*16)(R1), F30 \
	FMOVD	(offset+8*17)(R1), F31

// Save and restore VR20-31 (aka VSR56-63). These
// macros must point to a 16B aligned offset.
#define SAVE_VR_SIZE (12*16)
#define SAVE_VR(offset, rtmp)         \
	MOVD	$(offset+16*0), rtmp  \
	STVX	V20, (rtmp)(R1)       \
	MOVD	$(offset+16*1), rtmp  \
	STVX	V21, (rtmp)(R1)       \
	MOVD	$(offset+16*2), rtmp  \
	STVX	V22, (rtmp)(R1)       \
	MOVD	$(offset+16*3), rtmp  \
	STVX	V23, (rtmp)(R1)       \
	MOVD	$(offset+16*4), rtmp  \
	STVX	V24, (rtmp)(R1)       \
	MOVD	$(offset+16*5), rtmp  \
	STVX	V25, (rtmp)(R1)       \
	MOVD	$(offset+16*6), rtmp  \
	STVX	V26, (rtmp)(R1)       \
	MOVD	$(offset+16*7), rtmp  \
	STVX	V27, (rtmp)(R1)       \
	MOVD	$(offset+16*8), rtmp  \
	STVX	V28, (rtmp)(R1)       \
	MOVD	$(offset+16*9), rtmp  \
	STVX	V29, (rtmp)(R1)       \
	MOVD	$(offset+16*10), rtmp \
	STVX	V30, (rtmp)(R1)       \
	MOVD	$(offset+16*11), rtmp \
	STVX	V31, (rtmp)(R1)

#define RESTORE_VR(offset, rtmp)      \
	MOVD	$(offset+16*0), rtmp  \
	LVX	(rtmp)(R1), V20       \
	MOVD	$(offset+16*1), rtmp  \
	LVX	(rtmp)(R1), V21       \
	MOVD	$(offset+16*2), rtmp  \
	LVX	(rtmp)(R1), V22       \
	MOVD	$(offset+16*3), rtmp  \
	LVX	(rtmp)(R1), V23       \
	MOVD	$(offset+16*4), rtmp  \
	LVX	(rtmp)(R1), V24       \
	MOVD	$(offset+16*5), rtmp  \
	LVX	(rtmp)(R1), V25       \
	MOVD	$(offset+16*6), rtmp  \
	LVX	(rtmp)(R1), V26       \
	MOVD	$(offset+16*7), rtmp  \
	LVX	(rtmp)(R1), V27       \
	MOVD	$(offset+16*8), rtmp  \
	LVX	(rtmp)(R1), V28       \
	MOVD	$(offset+16*9), rtmp  \
	LVX	(rtmp)(R1), V29       \
	MOVD	$(offset+16*10), rtmp \
	LVX	(rtmp)(R1), V30       \
	MOVD	$(offset+16*11), rtmp \
	LVX	(rtmp)(R1), V31

// LR and CR are saved in the caller's frame. The callee must
// make space for all other callee-save registers.
#define SAVE_ALL_REG_SIZE (SAVE_GPR_SIZE+SAVE_FPR_SIZE+SAVE_VR_SIZE)

// Stack a frame and save all callee-save registers following the
// host OS's ABI. Fortunately, this is identical for AIX, ELFv1, and
// ELFv2. All host ABIs require the stack pointer to maintain 16 byte
// alignment, and save the callee-save registers in the same places.
//
// To restate, R1 is assumed to be aligned when this macro is used.
// This assumes the caller's frame is compliant with the host ABI.
// CR and LR are saved into the caller's frame per the host ABI.
// R0 is initialized to $0 as expected by Go.
#define STACK_AND_SAVE_HOST_TO_GO_ABI(extra)                       \
	MOVD	LR, R0                                             \
	MOVD	R0, 16(R1)                                         \
	MOVW	CR, R0                                             \
	MOVD	R0, 8(R1)                                          \
	MOVDU	R1, -(extra)-FIXED_FRAME-SAVE_ALL_REG_SIZE(R1)     \
	SAVE_GPR(extra+FIXED_FRAME)                                \
	SAVE_FPR(extra+FIXED_FRAME+SAVE_GPR_SIZE)                  \
	SAVE_VR(extra+FIXED_FRAME+SAVE_GPR_SIZE+SAVE_FPR_SIZE, R0) \
	MOVD	$0, R0

// This unstacks the frame, restoring all callee-save registers
// as saved by STACK_AND_SAVE_HOST_TO_GO_ABI.
//
// R0 is not guaranteed to contain $0 after this macro.
#define UNSTACK_AND_RESTORE_GO_TO_HOST_ABI(extra)                     \
	RESTORE_GPR(extra+FIXED_FRAME)                                \
	RESTORE_FPR(extra+FIXED_FRAME+SAVE_GPR_SIZE)                  \
	RESTORE_VR(extra+FIXED_FRAME+SAVE_GPR_SIZE+SAVE_FPR_SIZE, R0) \
	ADD 	$(extra+FIXED_FRAME+SAVE_ALL_REG_SIZE), R1            \
	MOVD	16(R1), R0                                            \
	MOVD	R0, LR                                                \
	MOVD	8(R1), R0                                             \
	MOVW	R0, CR

```

// === FILE: references!/go/src/runtime/cgo/abi_riscv64.h ===
```text
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Macros for transitioning from the host ABI to Go ABI0.
//
// These macros save and restore the callee-saved registers
// from the stack, but they don't adjust stack pointer, so
// the user should prepare stack space in advance.
// SAVE_GPR(offset) saves X8, X9, X18-X27 to the stack space
// of ((offset)+0*8)(X2) ~ ((offset)+11*8)(X2).
//
// SAVE_FPR(offset) saves F8, F9, F18-F27 to the stack space
// of ((offset)+0*8)(X2) ~ ((offset)+11*8)(X2).
//
// Note: g is X27

#define SAVE_GPR(offset) \
	MOV X8, ((offset)+0*8)(X2)   \
	MOV X9, ((offset)+1*8)(X2)   \
	MOV X18, ((offset)+2*8)(X2)  \
	MOV X19, ((offset)+3*8)(X2)  \
	MOV X20, ((offset)+4*8)(X2)  \
	MOV X21, ((offset)+5*8)(X2)  \
	MOV X22, ((offset)+6*8)(X2)  \
	MOV X23, ((offset)+7*8)(X2)  \
	MOV X24, ((offset)+8*8)(X2)  \
	MOV X25, ((offset)+9*8)(X2)  \
	MOV X26, ((offset)+10*8)(X2) \
	MOV g, ((offset)+11*8)(X2)

#define RESTORE_GPR(offset) \
	MOV ((offset)+0*8)(X2), X8   \
	MOV ((offset)+1*8)(X2), X9   \
	MOV ((offset)+2*8)(X2), X18  \
	MOV ((offset)+3*8)(X2), X19  \
	MOV ((offset)+4*8)(X2), X20  \
	MOV ((offset)+5*8)(X2), X21  \
	MOV ((offset)+6*8)(X2), X22  \
	MOV ((offset)+7*8)(X2), X23  \
	MOV ((offset)+8*8)(X2), X24  \
	MOV ((offset)+9*8)(X2), X25  \
	MOV ((offset)+10*8)(X2), X26 \
	MOV ((offset)+11*8)(X2), g

#define SAVE_FPR(offset) \
	MOVD F8, ((offset)+0*8)(X2)   \
	MOVD F9, ((offset)+1*8)(X2)   \
	MOVD F18, ((offset)+2*8)(X2)  \
	MOVD F19, ((offset)+3*8)(X2)  \
	MOVD F20, ((offset)+4*8)(X2)  \
	MOVD F21, ((offset)+5*8)(X2)  \
	MOVD F22, ((offset)+6*8)(X2)  \
	MOVD F23, ((offset)+7*8)(X2)  \
	MOVD F24, ((offset)+8*8)(X2)  \
	MOVD F25, ((offset)+9*8)(X2)  \
	MOVD F26, ((offset)+10*8)(X2) \
	MOVD F27, ((offset)+11*8)(X2)

#define RESTORE_FPR(offset) \
	MOVD ((offset)+0*8)(X2), F8   \
	MOVD ((offset)+1*8)(X2), F9   \
	MOVD ((offset)+2*8)(X2), F18  \
	MOVD ((offset)+3*8)(X2), F19  \
	MOVD ((offset)+4*8)(X2), F20  \
	MOVD ((offset)+5*8)(X2), F21  \
	MOVD ((offset)+6*8)(X2), F22  \
	MOVD ((offset)+7*8)(X2), F23  \
	MOVD ((offset)+8*8)(X2), F24  \
	MOVD ((offset)+9*8)(X2), F25  \
	MOVD ((offset)+10*8)(X2), F26 \
	MOVD ((offset)+11*8)(X2), F27

```

// === FILE: references!/go/src/runtime/cgo/asm_386.s ===
```text
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// Set the x_crosscall2_ptr C function pointer variable point to crosscall2.
// It's such a pointer chain: _crosscall2_ptr -> x_crosscall2_ptr -> crosscall2
// Use a local trampoline, to avoid taking the address of a dynamically exported
// function.
TEXT ·set_crosscall2(SB),NOSPLIT,$0-0
	MOVL	_crosscall2_ptr(SB), AX
	MOVL	$crosscall2_trampoline<>(SB), BX
	MOVL	BX, (AX)
	RET

TEXT crosscall2_trampoline<>(SB),NOSPLIT,$0-0
	JMP	crosscall2(SB)

// Called by C code generated by cmd/cgo.
// func crosscall2(fn, a unsafe.Pointer, n int32, ctxt uintptr)
// Saves C callee-saved registers and calls cgocallback with three arguments.
// fn is the PC of a func(a unsafe.Pointer) function.
TEXT crosscall2(SB),NOSPLIT,$28-16
	MOVL BP, 24(SP)
	MOVL BX, 20(SP)
	MOVL SI, 16(SP)
	MOVL DI, 12(SP)

	MOVL	ctxt+12(FP), AX
	MOVL	AX, 8(SP)
	MOVL	a+4(FP), AX
	MOVL	AX, 4(SP)
	MOVL	fn+0(FP), AX
	MOVL	AX, 0(SP)
	CALL	runtime·cgocallback(SB)

	MOVL 12(SP), DI
	MOVL 16(SP), SI
	MOVL 20(SP), BX
	MOVL 24(SP), BP
	RET

```

// === FILE: references!/go/src/runtime/cgo/asm_amd64.s ===
```text
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"
#include "abi_amd64.h"

// Set the x_crosscall2_ptr C function pointer variable point to crosscall2.
// It's such a pointer chain: _crosscall2_ptr -> x_crosscall2_ptr -> crosscall2
// Use a local trampoline, to avoid taking the address of a dynamically exported
// function.
TEXT ·set_crosscall2(SB),NOSPLIT,$0-0
	MOVQ	_crosscall2_ptr(SB), AX
	MOVQ	$crosscall2_trampoline<>(SB), BX
	MOVQ	BX, (AX)
	RET

TEXT crosscall2_trampoline<>(SB),NOSPLIT,$0-0
	JMP	crosscall2(SB)

// Called by C code generated by cmd/cgo.
// func crosscall2(fn, a unsafe.Pointer, n int32, ctxt uintptr)
// Saves C callee-saved registers and calls cgocallback with three arguments.
// fn is the PC of a func(a unsafe.Pointer) function.
// This signature is known to SWIG, so we can't change it.
TEXT crosscall2(SB),NOSPLIT,$0-0
	PUSH_REGS_HOST_TO_ABI0()

	// Make room for arguments to cgocallback.
	ADJSP	$0x18
#ifndef GOOS_windows
	MOVQ	DI, 0x0(SP)	/* fn */
	MOVQ	SI, 0x8(SP)	/* arg */
	// Skip n in DX.
	MOVQ	CX, 0x10(SP)	/* ctxt */
#else
	MOVQ	CX, 0x0(SP)	/* fn */
	MOVQ	DX, 0x8(SP)	/* arg */
	// Skip n in R8.
	MOVQ	R9, 0x10(SP)	/* ctxt */
#endif

	CALL	runtime·cgocallback(SB)

	ADJSP	$-0x18
	POP_REGS_HOST_TO_ABI0()
	RET

```

// === FILE: references!/go/src/runtime/cgo/asm_arm.s ===
```text
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// Set the x_crosscall2_ptr C function pointer variable point to crosscall2.
// It's such a pointer chain: _crosscall2_ptr -> x_crosscall2_ptr -> crosscall2
// Use a local trampoline, to avoid taking the address of a dynamically exported
// function.
TEXT ·set_crosscall2(SB),NOSPLIT,$0-0
	MOVW	_crosscall2_ptr(SB), R1
	MOVW	$crosscall2_trampoline<>(SB), R2
	MOVW	R2, (R1)
	RET

TEXT crosscall2_trampoline<>(SB),NOSPLIT,$0-0
	JMP	crosscall2(SB)

// Called by C code generated by cmd/cgo.
// func crosscall2(fn, a unsafe.Pointer, n int32, ctxt uintptr)
// Saves C callee-saved registers and calls cgocallback with three arguments.
// fn is the PC of a func(a unsafe.Pointer) function.
TEXT crosscall2(SB),NOSPLIT|NOFRAME,$0
	SUB	$(8*9), R13 // Reserve space for the floating point registers.
	// The C arguments arrive in R0, R1, R2, and R3. We want to
	// pass R0, R1, and R3 to Go, so we push those on the stack.
	// Also, save C callee-save registers R4-R12.
	MOVM.WP	[R0, R1, R3, R4, R5, R6, R7, R8, R9, g, R11, R12], (R13)
	// Finally, save the link register R14. This also puts the
	// arguments we pushed for cgocallback where they need to be,
	// starting at 4(R13).
	MOVW.W	R14, -4(R13)

	// Skip floating point registers if goarmsoftfp!=0.
	MOVB    runtime·goarmsoftfp(SB), R11
	CMP     $0, R11
	BNE     skipfpsave
	MOVD	F8, (13*4+8*1)(R13)
	MOVD	F9, (13*4+8*2)(R13)
	MOVD	F10, (13*4+8*3)(R13)
	MOVD	F11, (13*4+8*4)(R13)
	MOVD	F12, (13*4+8*5)(R13)
	MOVD	F13, (13*4+8*6)(R13)
	MOVD	F14, (13*4+8*7)(R13)
	MOVD	F15, (13*4+8*8)(R13)

skipfpsave:
	BL	runtime·load_g(SB)
	// We set up the arguments to cgocallback when saving registers above.
	BL	runtime·cgocallback(SB)

	MOVB    runtime·goarmsoftfp(SB), R11
	CMP     $0, R11
	BNE     skipfprest
	MOVD	(13*4+8*1)(R13), F8
	MOVD	(13*4+8*2)(R13), F9
	MOVD	(13*4+8*3)(R13), F10
	MOVD	(13*4+8*4)(R13), F11
	MOVD	(13*4+8*5)(R13), F12
	MOVD	(13*4+8*6)(R13), F13
	MOVD	(13*4+8*7)(R13), F14
	MOVD	(13*4+8*8)(R13), F15

skipfprest:
	MOVW.P	4(R13), R14
	MOVM.IAW	(R13), [R0, R1, R3, R4, R5, R6, R7, R8, R9, g, R11, R12]
	ADD	$(8*9), R13
	MOVW	R14, R15

```

// === FILE: references!/go/src/runtime/cgo/asm_arm64.s ===
```text
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"
#include "abi_arm64.h"

// Set the x_crosscall2_ptr C function pointer variable point to crosscall2.
// It's such a pointer chain: _crosscall2_ptr -> x_crosscall2_ptr -> crosscall2
// Use a local trampoline, to avoid taking the address of a dynamically exported
// function.
TEXT ·set_crosscall2(SB),NOSPLIT,$0-0
	MOVD	_crosscall2_ptr(SB), R1
	MOVD	$crosscall2_trampoline<>(SB), R2
	MOVD	R2, (R1)
	RET

TEXT crosscall2_trampoline<>(SB),NOSPLIT,$0-0
	JMP	crosscall2(SB)

// Called by C code generated by cmd/cgo.
// func crosscall2(fn, a unsafe.Pointer, n int32, ctxt uintptr)
// Saves C callee-saved registers and calls cgocallback with three arguments.
// fn is the PC of a func(a unsafe.Pointer) function.
TEXT crosscall2(SB),NOSPLIT|NOFRAME,$0
	/*
	 * We still need to save all callee save register as before, and then
	 *  push 3 args for fn (R0, R1, R3), skipping R2.
	 * Also note that at procedure entry in gc world, 8(RSP) will be the
	 *  first arg.
	 */
	SUB	$(8*24), RSP
	STP	(R0, R1), (8*1)(RSP)
	MOVD	R3, (8*3)(RSP)

	SAVE_R19_TO_R28(8*4)
	SAVE_F8_TO_F15(8*14)
	STP	(R29, R30), (8*22)(RSP)


	// Initialize Go ABI environment
	BL	runtime·load_g(SB)
	BL	runtime·cgocallback(SB)

	RESTORE_R19_TO_R28(8*4)
	RESTORE_F8_TO_F15(8*14)
	LDP	(8*22)(RSP), (R29, R30)

	ADD	$(8*24), RSP
	RET

```

// === FILE: references!/go/src/runtime/cgo/asm_loong64.s ===
```text
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"
#include "abi_loong64.h"

// Set the x_crosscall2_ptr C function pointer variable point to crosscall2.
// It's such a pointer chain: _crosscall2_ptr -> x_crosscall2_ptr -> crosscall2
// Use a local trampoline, to avoid taking the address of a dynamically exported
// function.
TEXT ·set_crosscall2(SB),NOSPLIT,$0-0
	MOVV	_crosscall2_ptr(SB), R5
	MOVV	$crosscall2_trampoline<>(SB), R6
	MOVV	R6, (R5)
	RET

TEXT crosscall2_trampoline<>(SB),NOSPLIT,$0-0
	JMP	crosscall2(SB)

// Called by C code generated by cmd/cgo.
// func crosscall2(fn, a unsafe.Pointer, n int32, ctxt uintptr)
// Saves C callee-saved registers and calls cgocallback with three arguments.
// fn is the PC of a func(a unsafe.Pointer) function.
TEXT crosscall2(SB),NOSPLIT|NOFRAME,$0
	/*
	 * We still need to save all callee save register as before, and then
	 * push 3 args for fn (R4, R5, R7), skipping R6.
	 * Also note that at procedure entry in gc world, 8(R29) will be the
	 *  first arg.
	 */

	ADDV	$(-23*8), R3
	MOVV	R4, (1*8)(R3) // fn unsafe.Pointer
	MOVV	R5, (2*8)(R3) // a unsafe.Pointer
	MOVV	R7, (3*8)(R3) // ctxt uintptr

	SAVE_R22_TO_R31((4*8))
	SAVE_F24_TO_F31((14*8))
	MOVV	R1, (22*8)(R3)

	// Initialize Go ABI environment
	JAL	runtime·load_g(SB)

	JAL	runtime·cgocallback(SB)

	RESTORE_R22_TO_R31((4*8))
	RESTORE_F24_TO_F31((14*8))
	MOVV	(22*8)(R3), R1

	ADDV	$(23*8), R3

	RET

```

// === FILE: references!/go/src/runtime/cgo/asm_mips64x.s ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build mips64 || mips64le

#include "textflag.h"

// Set the x_crosscall2_ptr C function pointer variable point to crosscall2.
// It's such a pointer chain: _crosscall2_ptr -> x_crosscall2_ptr -> crosscall2
// Use a local trampoline, to avoid taking the address of a dynamically exported
// function.
TEXT ·set_crosscall2(SB),NOSPLIT,$0-0
	MOVV	_crosscall2_ptr(SB), R5
	MOVV	$crosscall2_trampoline<>(SB), R6
	MOVV	R6, (R5)
	RET

TEXT crosscall2_trampoline<>(SB),NOSPLIT,$0-0
	JMP	crosscall2(SB)

// Called by C code generated by cmd/cgo.
// func crosscall2(fn, a unsafe.Pointer, n int32, ctxt uintptr)
// Saves C callee-saved registers and calls cgocallback with three arguments.
// fn is the PC of a func(a unsafe.Pointer) function.
TEXT crosscall2(SB),NOSPLIT|NOFRAME,$0
	/*
	 * We still need to save all callee save register as before, and then
	 *  push 3 args for fn (R4, R5, R7), skipping R6.
	 * Also note that at procedure entry in gc world, 8(R29) will be the
	 *  first arg.
	 */
#ifndef GOMIPS64_softfloat
	ADDV	$(-8*23), R29
#else
	ADDV	$(-8*15), R29
#endif
	MOVV	R4, (8*1)(R29) // fn unsafe.Pointer
	MOVV	R5, (8*2)(R29) // a unsafe.Pointer
	MOVV	R7, (8*3)(R29) // ctxt uintptr
	MOVV	R16, (8*4)(R29)
	MOVV	R17, (8*5)(R29)
	MOVV	R18, (8*6)(R29)
	MOVV	R19, (8*7)(R29)
	MOVV	R20, (8*8)(R29)
	MOVV	R21, (8*9)(R29)
	MOVV	R22, (8*10)(R29)
	MOVV	R23, (8*11)(R29)
	MOVV	RSB, (8*12)(R29)
	MOVV	g, (8*13)(R29)
	MOVV	R31, (8*14)(R29)
#ifndef GOMIPS64_softfloat
	MOVD	F24, (8*15)(R29)
	MOVD	F25, (8*16)(R29)
	MOVD	F26, (8*17)(R29)
	MOVD	F27, (8*18)(R29)
	MOVD	F28, (8*19)(R29)
	MOVD	F29, (8*20)(R29)
	MOVD	F30, (8*21)(R29)
	MOVD	F31, (8*22)(R29)
#endif
	// Initialize Go ABI environment
	// prepare SB register = PC & 0xffffffff00000000
	BGEZAL	R0, 1(PC)
	SRLV	$32, R31, RSB
	SLLV	$32, RSB
	JAL	runtime·load_g(SB)

	JAL	runtime·cgocallback(SB)

	MOVV	(8*4)(R29), R16
	MOVV	(8*5)(R29), R17
	MOVV	(8*6)(R29), R18
	MOVV	(8*7)(R29), R19
	MOVV	(8*8)(R29), R20
	MOVV	(8*9)(R29), R21
	MOVV	(8*10)(R29), R22
	MOVV	(8*11)(R29), R23
	MOVV	(8*12)(R29), RSB
	MOVV	(8*13)(R29), g
	MOVV	(8*14)(R29), R31
#ifndef GOMIPS64_softfloat
	MOVD	(8*15)(R29), F24
	MOVD	(8*16)(R29), F25
	MOVD	(8*17)(R29), F26
	MOVD	(8*18)(R29), F27
	MOVD	(8*19)(R29), F28
	MOVD	(8*20)(R29), F29
	MOVD	(8*21)(R29), F30
	MOVD	(8*22)(R29), F31
	ADDV	$(8*23), R29
#else
	ADDV	$(8*15), R29
#endif
	RET

```

// === FILE: references!/go/src/runtime/cgo/asm_mipsx.s ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build mips || mipsle

#include "textflag.h"

// Set the x_crosscall2_ptr C function pointer variable point to crosscall2.
// It's such a pointer chain: _crosscall2_ptr -> x_crosscall2_ptr -> crosscall2
// Use a local trampoline, to avoid taking the address of a dynamically exported
// function.
TEXT ·set_crosscall2(SB),NOSPLIT,$0-0
	MOVW	_crosscall2_ptr(SB), R5
	MOVW	$crosscall2_trampoline<>(SB), R6
	MOVW	R6, (R5)
	RET

TEXT crosscall2_trampoline<>(SB),NOSPLIT,$0-0
	JMP	crosscall2(SB)

// Called by C code generated by cmd/cgo.
// func crosscall2(fn, a unsafe.Pointer, n int32, ctxt uintptr)
// Saves C callee-saved registers and calls cgocallback with three arguments.
// fn is the PC of a func(a unsafe.Pointer) function.
TEXT crosscall2(SB),NOSPLIT|NOFRAME,$0
	/*
	 * We still need to save all callee save register as before, and then
	 *  push 3 args for fn (R4, R5, R7), skipping R6.
	 * Also note that at procedure entry in gc world, 4(R29) will be the
	 *  first arg.
	 */

	// Space for 9 caller-saved GPR + LR + 6 caller-saved FPR.
	// O32 ABI allows us to smash 16 bytes argument area of caller frame.
#ifndef GOMIPS_softfloat
	SUBU	$(4*14+8*6-16), R29
#else
	SUBU	$(4*14-16), R29	// For soft-float, no FPR.
#endif
	MOVW	R4, (4*1)(R29)	// fn unsafe.Pointer
	MOVW	R5, (4*2)(R29)	// a unsafe.Pointer
	MOVW	R7, (4*3)(R29)	// ctxt uintptr
	MOVW	R16, (4*4)(R29)
	MOVW	R17, (4*5)(R29)
	MOVW	R18, (4*6)(R29)
	MOVW	R19, (4*7)(R29)
	MOVW	R20, (4*8)(R29)
	MOVW	R21, (4*9)(R29)
	MOVW	R22, (4*10)(R29)
	MOVW	R23, (4*11)(R29)
	MOVW	g, (4*12)(R29)
	MOVW	R31, (4*13)(R29)
#ifndef GOMIPS_softfloat
	MOVD	F20, (4*14)(R29)
	MOVD	F22, (4*14+8*1)(R29)
	MOVD	F24, (4*14+8*2)(R29)
	MOVD	F26, (4*14+8*3)(R29)
	MOVD	F28, (4*14+8*4)(R29)
	MOVD	F30, (4*14+8*5)(R29)
#endif
	JAL	runtime·load_g(SB)

	JAL	runtime·cgocallback(SB)

	MOVW	(4*4)(R29), R16
	MOVW	(4*5)(R29), R17
	MOVW	(4*6)(R29), R18
	MOVW	(4*7)(R29), R19
	MOVW	(4*8)(R29), R20
	MOVW	(4*9)(R29), R21
	MOVW	(4*10)(R29), R22
	MOVW	(4*11)(R29), R23
	MOVW	(4*12)(R29), g
	MOVW	(4*13)(R29), R31
#ifndef GOMIPS_softfloat
	MOVD	(4*14)(R29), F20
	MOVD	(4*14+8*1)(R29), F22
	MOVD	(4*14+8*2)(R29), F24
	MOVD	(4*14+8*3)(R29), F26
	MOVD	(4*14+8*4)(R29), F28
	MOVD	(4*14+8*5)(R29), F30

	ADDU	$(4*14+8*6-16), R29
#else
	ADDU	$(4*14-16), R29
#endif
	RET

```

// === FILE: references!/go/src/runtime/cgo/asm_ppc64x.s ===
```text
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ppc64 || ppc64le

#include "textflag.h"
#include "asm_ppc64x.h"
#include "abi_ppc64x.h"

#ifdef GO_PPC64X_HAS_FUNCDESC
// crosscall2 is marked with go:cgo_export_static. On AIX, this creates and exports
// the symbol name and descriptor as the AIX linker expects, but does not work if
// referenced from within Go. Create and use an aliased descriptor of crosscall2
// to workaround this.
DEFINE_PPC64X_FUNCDESC(_crosscall2<>, crosscall2)
#define CROSSCALL2_FPTR $_crosscall2<>(SB)
#else
// Use a local trampoline, to avoid taking the address of a dynamically exported
// function.
#define CROSSCALL2_FPTR $crosscall2_trampoline<>(SB)
#endif

// Set the x_crosscall2_ptr C function pointer variable point to crosscall2.
// It's such a pointer chain: _crosscall2_ptr -> x_crosscall2_ptr -> crosscall2
TEXT ·set_crosscall2(SB),NOSPLIT,$0-0
	MOVD	_crosscall2_ptr(SB), R5
	MOVD	CROSSCALL2_FPTR, R6
	MOVD	R6, (R5)
	RET

TEXT crosscall2_trampoline<>(SB),NOSPLIT,$0-0
	JMP	crosscall2(SB)

// Called by C code generated by cmd/cgo.
// func crosscall2(fn, a unsafe.Pointer, n int32, ctxt uintptr)
// Saves C callee-saved registers and calls cgocallback with three arguments.
// fn is the PC of a func(a unsafe.Pointer) function.
// The value of R2 is saved on the new stack frame, and not
// the caller's frame due to issue #43228.
TEXT crosscall2(SB),NOSPLIT|NOFRAME,$0
	// Start with standard C stack frame layout and linkage, allocate
	// 32 bytes of argument space, save callee-save regs, and set R0 to $0.
	STACK_AND_SAVE_HOST_TO_GO_ABI(32)
	// The above will not preserve R2 (TOC). Save it in case Go is
	// compiled without a TOC pointer (e.g -buildmode=default).
	MOVD	R2, 24(R1)

	// Load the current g.
	BL	runtime·load_g(SB)

#ifdef GO_PPC64X_HAS_FUNCDESC
	// Load the real entry address from the first slot of the function descriptor.
	// The first argument fn might be null, that means dropm in pthread key destructor.
	CMP	R3, $0
	BEQ	nil_fn
	MOVD	8(R3), R2
	MOVD	(R3), R3
nil_fn:
#endif
	MOVD	R3, FIXED_FRAME+0(R1)	// fn unsafe.Pointer
	MOVD	R4, FIXED_FRAME+8(R1)	// a unsafe.Pointer
	// Skip R5 = n uint32
	MOVD	R6, FIXED_FRAME+16(R1)	// ctxt uintptr
	BL	runtime·cgocallback(SB)

	// Restore the old frame, and R2.
	MOVD	24(R1), R2
	UNSTACK_AND_RESTORE_GO_TO_HOST_ABI(32)
	RET

```

// === FILE: references!/go/src/runtime/cgo/asm_riscv64.s ===
```text
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"
#include "abi_riscv64.h"

// Set the x_crosscall2_ptr C function pointer variable point to crosscall2.
// It's such a pointer chain: _crosscall2_ptr -> x_crosscall2_ptr -> crosscall2
// Use a local trampoline, to avoid taking the address of a dynamically exported
// function.
TEXT ·set_crosscall2(SB),NOSPLIT,$0-0
	MOV	_crosscall2_ptr(SB), X7
	MOV	$crosscall2_trampoline<>(SB), X8
	MOV	X8, (X7)
	RET

TEXT crosscall2_trampoline<>(SB),NOSPLIT,$0-0
	JMP	crosscall2(SB)

// Called by C code generated by cmd/cgo.
// func crosscall2(fn, a unsafe.Pointer, n int32, ctxt uintptr)
// Saves C callee-saved registers and calls cgocallback with three arguments.
// fn is the PC of a func(a unsafe.Pointer) function.
TEXT crosscall2(SB),NOSPLIT|NOFRAME,$0
	/*
	 * Push arguments for fn (X10, X11, X13), along with all callee-save
	 * registers. Note that at procedure entry the first argument is at
	 * 8(X2).
	 */
	ADD	$(-8*29), X2
	MOV	X10, (8*1)(X2) // fn unsafe.Pointer
	MOV	X11, (8*2)(X2) // a unsafe.Pointer
	MOV	X13, (8*3)(X2) // ctxt uintptr

	SAVE_GPR((8*4))
	MOV	X1, (8*16)(X2)
	SAVE_FPR((8*17))

	// Initialize Go ABI environment
	CALL	runtime·load_g(SB)
	CALL	runtime·cgocallback(SB)

	RESTORE_GPR((8*4))
	MOV	(8*16)(X2), X1
	RESTORE_FPR((8*17))

	ADD	$(8*29), X2

	RET

```

// === FILE: references!/go/src/runtime/cgo/asm_s390x.s ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// Set the x_crosscall2_ptr C function pointer variable point to crosscall2.
// It's such a pointer chain: _crosscall2_ptr -> x_crosscall2_ptr -> crosscall2
// Use a local trampoline, to avoid taking the address of a dynamically exported
// function.
TEXT ·set_crosscall2(SB),NOSPLIT,$0-0
	MOVD	_crosscall2_ptr(SB), R1
	MOVD	$crosscall2_trampoline<>(SB), R2
	MOVD	R2, (R1)
	RET

TEXT crosscall2_trampoline<>(SB),NOSPLIT,$0-0
	JMP	crosscall2(SB)

// Called by C code generated by cmd/cgo.
// func crosscall2(fn, a unsafe.Pointer, n int32, ctxt uintptr)
// Saves C callee-saved registers and calls cgocallback with three arguments.
// fn is the PC of a func(a unsafe.Pointer) function.
TEXT crosscall2(SB),NOSPLIT|NOFRAME,$0
	// Start with standard C stack frame layout and linkage.

	// Save R6-R15 in the register save area of the calling function.
	STMG	R6, R15, 48(R15)

	// Allocate 96 bytes on the stack.
	MOVD	$-96(R15), R15

	// Save F8-F15 in our stack frame.
	FMOVD	F8, 32(R15)
	FMOVD	F9, 40(R15)
	FMOVD	F10, 48(R15)
	FMOVD	F11, 56(R15)
	FMOVD	F12, 64(R15)
	FMOVD	F13, 72(R15)
	FMOVD	F14, 80(R15)
	FMOVD	F15, 88(R15)

	// Initialize Go ABI environment.
	BL	runtime·load_g(SB)

	MOVD	R2, 8(R15)	// fn unsafe.Pointer
	MOVD	R3, 16(R15)	// a unsafe.Pointer
	// Skip R4 = n uint32
	MOVD	R5, 24(R15)	// ctxt uintptr
	BL	runtime·cgocallback(SB)

	FMOVD	32(R15), F8
	FMOVD	40(R15), F9
	FMOVD	48(R15), F10
	FMOVD	56(R15), F11
	FMOVD	64(R15), F12
	FMOVD	72(R15), F13
	FMOVD	80(R15), F14
	FMOVD	88(R15), F15

	// De-allocate stack frame.
	MOVD	$96(R15), R15

	// Restore R6-R15.
	LMG	48(R15), R6, R15

	RET


```

// === FILE: references!/go/src/runtime/cgo/asm_wasm.s ===
```text
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

TEXT ·set_crosscall2(SB),NOSPLIT,$0-0
	UNDEF

TEXT crosscall2(SB), NOSPLIT, $0
	UNDEF

```

// === FILE: references!/go/src/runtime/cgo/callbacks.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cgo

import "unsafe"

// These utility functions are available to be called from code
// compiled with gcc via crosscall2.

// The declaration of crosscall2 is:
//   void crosscall2(void (*fn)(void *), void *, int);
//
// We need to export the symbol crosscall2 in order to support
// callbacks from shared libraries. This applies regardless of
// linking mode.
//
// Compatibility note: SWIG uses crosscall2 in exactly one situation:
// to call _cgo_panic using the pattern shown below. We need to keep
// that pattern working. In particular, crosscall2 actually takes four
// arguments, but it works to call it with three arguments when
// calling _cgo_panic.
//
//go:cgo_export_static crosscall2
//go:cgo_export_dynamic crosscall2

// Panic. The argument is converted into a Go string.

// Call like this in code compiled with gcc:
//   struct { const char *p; } a;
//   a.p = /* string to pass to panic */;
//   crosscall2(_cgo_panic, &a, sizeof a);
//   /* The function call will not return.  */

// TODO: We should export a regular C function to panic, change SWIG
// to use that instead of the above pattern, and then we can drop
// backwards-compatibility from crosscall2 and stop exporting it.

//go:linkname _runtime_cgo_panic_internal runtime._cgo_panic_internal
func _runtime_cgo_panic_internal(p *byte)

//go:linkname _cgo_panic _cgo_panic
//go:cgo_export_static _cgo_panic
//go:cgo_export_dynamic _cgo_panic
func _cgo_panic(a *struct{ cstr *byte }) {
	_runtime_cgo_panic_internal(a.cstr)
}

//go:cgo_import_static _cgo_init
//go:linkname _cgo_init _cgo_init
var _cgo_init unsafe.Pointer

//go:cgo_import_static _cgo_thread_start
//go:linkname _cgo_thread_start _cgo_thread_start
var _cgo_thread_start unsafe.Pointer

// Creates a new system thread without updating any Go state.
//
// This method is invoked during shared library loading to create a new OS
// thread to perform the runtime initialization. This method is similar to
// x_cgo_thread_start except that it doesn't update any Go state.

//go:cgo_import_static _cgo_sys_thread_create
//go:linkname _cgo_sys_thread_create _cgo_sys_thread_create
var _cgo_sys_thread_create unsafe.Pointer

// Indicates whether a dummy thread key has been created or not.
//
// When calling go exported function from C, we register a destructor
// callback, for a dummy thread key, by using pthread_key_create.

//go:cgo_import_static x_cgo_pthread_key_created
//go:linkname x_cgo_pthread_key_created x_cgo_pthread_key_created
//go:linkname _cgo_pthread_key_created _cgo_pthread_key_created
var x_cgo_pthread_key_created byte
var _cgo_pthread_key_created = &x_cgo_pthread_key_created

// Export crosscall2 to a c function pointer variable.
// Used to dropm in pthread key destructor, while C thread is exiting.

//go:cgo_import_static x_crosscall2_ptr
//go:linkname x_crosscall2_ptr x_crosscall2_ptr
//go:linkname _crosscall2_ptr _crosscall2_ptr
var x_crosscall2_ptr byte
var _crosscall2_ptr = &x_crosscall2_ptr

// Set the x_crosscall2_ptr C function pointer variable point to crosscall2.
// It's for the runtime package to call at init time.
func set_crosscall2()

//go:linkname _set_crosscall2 runtime.set_crosscall2
var _set_crosscall2 = set_crosscall2

// Store the g into the thread-specific value.
// So that pthread_key_destructor will dropm when the thread is exiting.

//go:cgo_import_static _cgo_bindm
//go:linkname _cgo_bindm _cgo_bindm
var _cgo_bindm unsafe.Pointer

// Notifies that the runtime has been initialized.
//
// We currently block at every CGO entry point (via _cgo_wait_runtime_init_done)
// to ensure that the runtime has been initialized before the CGO call is
// executed. This is necessary for shared libraries where we kickoff runtime
// initialization in a separate thread and return without waiting for this
// thread to complete the init.

//go:cgo_import_static x_cgo_notify_runtime_init_done
//go:linkname x_cgo_notify_runtime_init_done x_cgo_notify_runtime_init_done
//go:linkname _cgo_notify_runtime_init_done _cgo_notify_runtime_init_done
var x_cgo_notify_runtime_init_done byte
var _cgo_notify_runtime_init_done = &x_cgo_notify_runtime_init_done

// Sets the traceback, context, and symbolizer functions. See
// runtime.SetCgoTraceback.

//go:cgo_import_static x_cgo_set_traceback_functions
//go:linkname x_cgo_set_traceback_functions x_cgo_set_traceback_functions
//go:linkname _cgo_set_traceback_functions _cgo_set_traceback_functions
var x_cgo_set_traceback_functions byte
var _cgo_set_traceback_functions = &x_cgo_set_traceback_functions

// Call the traceback function registered with x_cgo_set_traceback_functions.

//go:cgo_import_static x_cgo_call_traceback_function
//go:linkname x_cgo_call_traceback_function x_cgo_call_traceback_function
//go:linkname _cgo_call_traceback_function _cgo_call_traceback_function
var x_cgo_call_traceback_function byte
var _cgo_call_traceback_function = &x_cgo_call_traceback_function

// Call the symbolizer function registered with x_cgo_set_symbolizer_functions.

//go:cgo_import_static x_cgo_call_symbolizer_function
//go:linkname x_cgo_call_symbolizer_function x_cgo_call_symbolizer_function
//go:linkname _cgo_call_symbolizer_function _cgo_call_symbolizer_function
var x_cgo_call_symbolizer_function byte
var _cgo_call_symbolizer_function = &x_cgo_call_symbolizer_function

// Calls a libc function to execute background work injected via libc
// interceptors, such as processing pending signals under the thread
// sanitizer.
//
// Left as a nil pointer if no libc interceptors are expected.

//go:cgo_import_static _cgo_yield
//go:linkname _cgo_yield _cgo_yield
var _cgo_yield unsafe.Pointer

//go:cgo_export_static _cgo_topofstack
//go:cgo_export_dynamic _cgo_topofstack

// x_cgo_getstackbound gets the thread's C stack size and
// set the G's stack bound based on the stack size.

//go:cgo_import_static _cgo_getstackbound
//go:linkname _cgo_getstackbound _cgo_getstackbound
var _cgo_getstackbound unsafe.Pointer

```

// === FILE: references!/go/src/runtime/cgo/callbacks_aix.go ===
```go
// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cgo

// These functions must be exported in order to perform
// longcall on cgo programs (cf gcc_aix_ppc64.c).
//
//go:cgo_export_static __cgo_topofstack
//go:cgo_export_static runtime.rt0_go
//go:cgo_export_static _rt0_ppc64_aix_lib

```

// === FILE: references!/go/src/runtime/cgo/callbacks_traceback.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin || linux

package cgo

import _ "unsafe" // for go:linkname

// Calls the traceback function passed to SetCgoTraceback.

//go:cgo_import_static x_cgo_callers
//go:linkname x_cgo_callers x_cgo_callers
//go:linkname _cgo_callers _cgo_callers
var x_cgo_callers byte
var _cgo_callers = &x_cgo_callers

```

// === FILE: references!/go/src/runtime/cgo/cgo.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package cgo contains runtime support for code generated
by the cgo tool.  See the documentation for the cgo command
for details on using cgo.
*/
package cgo

/*

#cgo darwin,!arm64 LDFLAGS: -lpthread
#cgo dragonfly LDFLAGS: -lpthread
#cgo freebsd LDFLAGS: -lpthread
#cgo android LDFLAGS: -llog
#cgo !android,linux LDFLAGS: -lpthread
#cgo netbsd LDFLAGS: -lpthread
#cgo openbsd LDFLAGS: -lpthread
#cgo aix LDFLAGS: -Wl,-berok

// Use -fno-stack-protector to avoid problems locating the
// proper support functions. See issues #52919, #54313, #58385.
// Use -Wdeclaration-after-statement because some CI builds use it.
#cgo CFLAGS: -Wall -Werror -fno-stack-protector -Wdeclaration-after-statement

// Use -std=gnu90 to maintain portability;
// we don't use c90 because that doesn't permit C++ line comments,
// which is just too painful.
// We don't do it on windows-386 because it causes test failures.
#cgo (!windows||!386) CFLAGS: -std=gnu90

#cgo solaris CPPFLAGS: -D_POSIX_PTHREAD_SEMANTICS

*/
import "C"

import "internal/runtime/sys"

// Incomplete is used specifically for the semantics of incomplete C types.
type Incomplete struct {
	_ sys.NotInHeap
}

```

// === FILE: references!/go/src/runtime/cgo/clearenv.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux

package cgo

import _ "unsafe" // for go:linkname

//go:cgo_import_static x_cgo_clearenv
//go:linkname x_cgo_clearenv x_cgo_clearenv
//go:linkname _cgo_clearenv runtime._cgo_clearenv
var x_cgo_clearenv byte
var _cgo_clearenv = &x_cgo_clearenv

```

// === FILE: references!/go/src/runtime/cgo/dragonfly.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build dragonfly

package cgo

import _ "unsafe" // for go:linkname

// Supply environ and __progname, because we don't
// link against the standard DragonFly crt0.o and the
// libc dynamic library needs them.

//go:linkname _environ environ
//go:linkname _progname __progname

var _environ uintptr
var _progname uintptr

```

// === FILE: references!/go/src/runtime/cgo/freebsd.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build freebsd

package cgo

import _ "unsafe" // for go:linkname

// Supply environ and __progname, because we don't
// link against the standard FreeBSD crt0.o and the
// libc dynamic library needs them.

//go:linkname _environ environ
//go:linkname _progname __progname

//go:cgo_export_dynamic environ
//go:cgo_export_dynamic __progname

var _environ uintptr
var _progname uintptr

```

// === FILE: references!/go/src/runtime/cgo/gcc_386.S ===
```text
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

.file "gcc_386.S"

/*
 * Windows still insists on underscore prefixes for C function names.
 */
#if defined(_WIN32)
#define EXT(s) _##s
#else
#define EXT(s) s
#endif

/*
 * void crosscall1(void (*fn)(void), void (*setg_gcc)(void*), void *g)
 *
 * Calling into the gc tool chain, where all registers are caller save.
 * Called from standard x86 ABI, where %ebp, %ebx, %esi,
 * and %edi are callee-save, so they must be saved explicitly.
 */
.globl EXT(crosscall1)
EXT(crosscall1):
	pushl %ebp
	movl %esp, %ebp
	pushl %ebx
	pushl %esi
	pushl %edi

	movl 16(%ebp), %eax	/* g */
	pushl %eax
	movl 12(%ebp), %eax	/* setg_gcc */
	call *%eax
	popl %eax

	movl 8(%ebp), %eax	/* fn */
	call *%eax

	popl %edi
	popl %esi
	popl %ebx
	popl %ebp
	ret

#ifdef __ELF__
.section .note.GNU-stack,"",@progbits
#endif

```

// === FILE: references!/go/src/runtime/cgo/gcc_aix_ppc64.c ===
```text
// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
 * On AIX, call to _cgo_topofstack and Go main are forced to be a longcall.
 * Without it, ld might add trampolines in the middle of .text section
 * to reach these functions which are normally declared in runtime package.
 */
extern int __attribute__((longcall)) __cgo_topofstack(void);
extern int __attribute__((longcall)) runtime_rt0_go(int argc, char **argv);
extern void __attribute__((longcall)) _rt0_ppc64_aix_lib(void);

int _cgo_topofstack(void) {
	return __cgo_topofstack();
}

int main(int argc, char **argv) {
	return runtime_rt0_go(argc, argv);
}

static void libinit(void) __attribute__ ((constructor));

/*
 * libinit aims to replace .init_array section which isn't available on aix.
 * Using __attribute__ ((constructor)) let gcc handles this instead of
 * adding special code in cmd/link.
 * However, it will be called for every Go programs which has cgo.
 * Inside _rt0_ppc64_aix_lib(), runtime.isarchive is checked in order
 * to know if this program is a c-archive or a simple cgo program.
 * If it's not set, _rt0_ppc64_ax_lib() returns directly.
 */
static void libinit() {
	_rt0_ppc64_aix_lib();
}

```

// === FILE: references!/go/src/runtime/cgo/gcc_aix_ppc64.S ===
```text
// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

.file "gcc_aix_ppc64.S"

/*
 * void crosscall1(void (*fn)(void), void (*setg_gcc)(void*), void *g)
 *
 * Calling into the gc tool chain, where all registers are caller save.
 * Called from standard ppc64 C ABI, where r2, r14-r31, f14-f31 are
 * callee-save, so they must be saved explicitly.
 * AIX has a special assembly syntax and keywords that can be mixed with
 * Linux assembly.
 * ABI: r3=fn, r4=setg_gcc, r5=g
 */
  .toc
  .csect .text[PR]
  .globl crosscall1
  .globl .crosscall1
  .csect crosscall1[DS]
crosscall1:
  .llong .crosscall1, TOC[tc0], 0
  .csect .text[PR]
.crosscall1:
	// Start with standard C stack frame layout and linkage
	mflr	0
	std	0, 16(1)	// Save LR in caller's frame
	std	2, 40(1)	// Save TOC in caller's frame
	bl	saveregs
	stdu	1, -296(1)

	// Set up Go ABI constant registers
	// Must match _cgo_reginit in runtime package.
	xor 0, 0, 0

	// Save fn in r14 (callee-save) so we can reuse r3 for setg_gcc call
	mr	14, 3
	// Restore g pointer (r30 in Go ABI, which may have been clobbered by C)
	mr	30, 5

	// Call setg_gcc(g)
	// Function pointers are function descriptors.
	// Dereference setg_gcc to get the entry point and TOC.
	mr	3, 5      // arg g
	ld	12, 0(4)  // load entry point from function descriptor
	ld	2, 8(4)   // load TOC from function descriptor
	mtctr	12
	bctrl

	// Call fn
	mr	12, 14
	mtctr	12
	bctrl

	addi	1, 1, 296
	bl	restoreregs
	ld	2, 40(1)
	ld	0, 16(1)
	mtlr	0
	blr

saveregs:
	// Save callee-save registers
	// O=-288; for R in {14..31}; do echo "\tstd\t$R, $O(1)"; ((O+=8)); done; for F in f{14..31}; do echo "\tstfd\t$F, $O(1)"; ((O+=8)); done
	std	14, -288(1)
	std	15, -280(1)
	std	16, -272(1)
	std	17, -264(1)
	std	18, -256(1)
	std	19, -248(1)
	std	20, -240(1)
	std	21, -232(1)
	std	22, -224(1)
	std	23, -216(1)
	std	24, -208(1)
	std	25, -200(1)
	std	26, -192(1)
	std	27, -184(1)
	std	28, -176(1)
	std	29, -168(1)
	std	30, -160(1)
	std	31, -152(1)
	stfd	14, -144(1)
	stfd	15, -136(1)
	stfd	16, -128(1)
	stfd	17, -120(1)
	stfd	18, -112(1)
	stfd	19, -104(1)
	stfd	20, -96(1)
	stfd	21, -88(1)
	stfd	22, -80(1)
	stfd	23, -72(1)
	stfd	24, -64(1)
	stfd	25, -56(1)
	stfd	26, -48(1)
	stfd	27, -40(1)
	stfd	28, -32(1)
	stfd	29, -24(1)
	stfd	30, -16(1)
	stfd	31, -8(1)

	blr

restoreregs:
	// O=-288; for R in {14..31}; do echo "\tld\t$R, $O(1)"; ((O+=8)); done; for F in {14..31}; do echo "\tlfd\t$F, $O(1)"; ((O+=8)); done
	ld	14, -288(1)
	ld	15, -280(1)
	ld	16, -272(1)
	ld	17, -264(1)
	ld	18, -256(1)
	ld	19, -248(1)
	ld	20, -240(1)
	ld	21, -232(1)
	ld	22, -224(1)
	ld	23, -216(1)
	ld	24, -208(1)
	ld	25, -200(1)
	ld	26, -192(1)
	ld	27, -184(1)
	ld	28, -176(1)
	ld	29, -168(1)
	ld	30, -160(1)
	ld	31, -152(1)
	lfd	14, -144(1)
	lfd	15, -136(1)
	lfd	16, -128(1)
	lfd	17, -120(1)
	lfd	18, -112(1)
	lfd	19, -104(1)
	lfd	20, -96(1)
	lfd	21, -88(1)
	lfd	22, -80(1)
	lfd	23, -72(1)
	lfd	24, -64(1)
	lfd	25, -56(1)
	lfd	26, -48(1)
	lfd	27, -40(1)
	lfd	28, -32(1)
	lfd	29, -24(1)
	lfd	30, -16(1)
	lfd	31, -8(1)

	blr

```

// === FILE: references!/go/src/runtime/cgo/gcc_amd64.S ===
```text
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

.file "gcc_amd64.S"

/*
 * Apple still insists on underscore prefixes for C function names.
 */
#if defined(__APPLE__)
#define EXT(s) _##s
#else
#define EXT(s) s
#endif

/*
 * void crosscall1(void (*fn)(void), void (*setg_gcc)(void*), void *g)
 *
 * Calling into the gc tool chain, where all registers are caller save.
 * Called from standard x86-64 ABI, where %rbx, %rbp, %r12-%r15
 * are callee-save so they must be saved explicitly.
 * The standard x86-64 ABI passes the three arguments m, g, fn
 * in %rdi, %rsi, %rdx.
 */
.globl EXT(crosscall1)
EXT(crosscall1):
	pushq %rbx
	pushq %rbp
	pushq %r12
	pushq %r13
	pushq %r14
	pushq %r15

#if defined(_WIN64)
	movq %r8, %rdi	/* arg of setg_gcc */
	call *%rdx	/* setg_gcc */
	call *%rcx	/* fn */
#else
	movq %rdi, %rbx
	movq %rdx, %rdi	/* arg of setg_gcc */
	call *%rsi	/* setg_gcc */
	call *%rbx	/* fn */
#endif

	popq %r15
	popq %r14
	popq %r13
	popq %r12
	popq %rbp
	popq %rbx
	ret

#ifdef __ELF__
.section .note.GNU-stack,"",@progbits
#endif

```

// === FILE: references!/go/src/runtime/cgo/gcc_android.c ===
```text
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include <pthread.h>
#include <dlfcn.h>
#include "libcgo.h"

// Truncated to a different magic value on 32-bit; that's ok.
#define magic1 (0x23581321345589ULL)

// From https://android.googlesource.com/platform/bionic/+/refs/heads/android10-tests-release/libc/private/bionic_asm_tls.h#69.
#define TLS_SLOT_APP 2

// inittls allocates a thread-local storage slot for g.
//
// It finds the first available slot using pthread_key_create and uses
// it as the offset value for runtime.tls_g.
static void
inittls(void **tlsg, void **tlsbase)
{
	pthread_key_t k;
	int i, err;
	void *handle, *get_ver, *off;

	// Check for Android Q where we can use the free TLS_SLOT_APP slot.
	handle = dlopen("libc.so", RTLD_LAZY);
	if (handle == NULL) {
		fatalf("inittls: failed to dlopen main program");
		return;
	}
	// android_get_device_api_level is introduced in Android Q, so its mere presence
	// is enough.
	get_ver = dlsym(handle, "android_get_device_api_level");
	dlclose(handle);
	if (get_ver != NULL) {
		off = (void *)(TLS_SLOT_APP*sizeof(void *));
		// tlsg is initialized to Q's free TLS slot. Verify it while we're here.
		if (*tlsg != off) {
			fatalf("tlsg offset wrong, got %ld want %ld\n", *tlsg, off);
		}
		return;
	}

	err = pthread_key_create(&k, nil);
	if(err != 0) {
		fatalf("pthread_key_create failed: %d", err);
	}
	pthread_setspecific(k, (void*)magic1);
	// If thread local slots are laid out as we expect, our magic word will
	// be located at some low offset from tlsbase. However, just in case something went
	// wrong, the search is limited to sensible offsets. PTHREAD_KEYS_MAX was the
	// original limit, but issue 19472 made a higher limit necessary.
	for (i=0; i<384; i++) {
		if (*(tlsbase+i) == (void*)magic1) {
			*tlsg = (void*)(i*sizeof(void *));
			pthread_setspecific(k, 0);
			return;
		}
	}
	fatalf("inittls: could not find pthread key");
}

void (*x_cgo_inittls)(void **tlsg, void **tlsbase) = inittls;

```

// === FILE: references!/go/src/runtime/cgo/gcc_arm.S ===
```text
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

.file "gcc_arm.S"

/*
 * void crosscall1(void (*fn)(void), void (*setg_gcc)(void *g), void *g)
 *
 * Calling into the gc tool chain, where all registers are caller save.
 * Called from standard ARM EABI, where r4-r11 are callee-save, so they
 * must be saved explicitly.
 */
.globl crosscall1
crosscall1:
	push {r4, r5, r6, r7, r8, r9, r10, r11, ip, lr}
	mov r4, r0
	mov r5, r1
	mov r0, r2

	// Because the assembler might target an earlier revision of the ISA
	// by default, we encode BLX as a .word.
	.word 0xe12fff35 // blx r5 // setg(g)
	.word 0xe12fff34 // blx r4 // fn()

	pop {r4, r5, r6, r7, r8, r9, r10, r11, ip, pc}


#ifdef __ELF__
.section .note.GNU-stack,"",%progbits
#endif

```

// === FILE: references!/go/src/runtime/cgo/gcc_arm64.S ===
```text
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

.file "gcc_arm64.S"

/*
 * Apple still insists on underscore prefixes for C function names.
 */
#if defined(__APPLE__)
#define EXT(s) _##s
#else
#define EXT(s) s
#endif

// Apple's ld64 wants 4-byte alignment for ARM code sections.
// .align in both Apple as and GNU as treat n as aligning to 2**n bytes.
.align	2

/*
 * void crosscall1(void (*fn)(void), void (*setg_gcc)(void *g), void *g)
 *
 * Calling into the gc tool chain, where all registers are caller save.
 * Called from standard ARM EABI, where x19-x29 are callee-save, so they
 * must be saved explicitly, along with x30 (LR).
 */
.globl EXT(crosscall1)
EXT(crosscall1):
	.cfi_startproc
	stp x29, x30, [sp, #-96]!
	.cfi_def_cfa_offset 96
	.cfi_offset 29, -96
	.cfi_offset 30, -88
	mov x29, sp
	.cfi_def_cfa_register 29
	stp x19, x20, [sp, #80]
	.cfi_offset 19, -16
	.cfi_offset 20, -8
	stp x21, x22, [sp, #64]
	.cfi_offset 21, -32
	.cfi_offset 22, -24
	stp x23, x24, [sp, #48]
	.cfi_offset 23, -48
	.cfi_offset 24, -40
	stp x25, x26, [sp, #32]
	.cfi_offset 25, -64
	.cfi_offset 26, -56
	stp x27, x28, [sp, #16]
	.cfi_offset 27, -80
	.cfi_offset 28, -72

	mov x19, x0
	mov x20, x1
	mov x0, x2

	blr x20
	blr x19

	ldp x27, x28, [sp, #16]
	.cfi_restore 27
	.cfi_restore 28
	ldp x25, x26, [sp, #32]
	.cfi_restore 25
	.cfi_restore 26
	ldp x23, x24, [sp, #48]
	.cfi_restore 23
	.cfi_restore 24
	ldp x21, x22, [sp, #64]
	.cfi_restore 21
	.cfi_restore 22
	ldp x19, x20, [sp, #80]
	.cfi_restore 19
	.cfi_restore 20
	ldp x29, x30, [sp], #96
	.cfi_restore 29
	.cfi_restore 30
	.cfi_def_cfa 31, 0
	ret
	.cfi_endproc


#ifdef __ELF__
.section .note.GNU-stack,"",%progbits
#endif

```

// === FILE: references!/go/src/runtime/cgo/gcc_clearenv.c ===
```text
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux

#include "libcgo.h"

#include <stdlib.h>

/* Stub for calling clearenv */
void
x_cgo_clearenv(void **env __attribute__((unused)))
{
	_cgo_tsan_acquire();
	clearenv();
	_cgo_tsan_release();
}

```

// === FILE: references!/go/src/runtime/cgo/gcc_context.c ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix || windows

#include "libcgo.h"

// Releases the cgo traceback context.
void _cgo_release_context(uintptr_t ctxt) {
	void (*pfn)(struct cgoContextArg*);

	pfn = _cgo_get_context_function();
	if (ctxt != 0 && pfn != nil) {
		struct cgoContextArg arg;

		arg.Context = ctxt;
		(*pfn)(&arg);
	}
}

```

// === FILE: references!/go/src/runtime/cgo/gcc_fatalf.c ===
```text
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix

#include <stdarg.h>
#ifdef __ANDROID__
#include <android/log.h>
#endif
#include "libcgo.h"

void
fatalf(const char* format, ...)
{
	va_list ap;

	fprintf(stderr, "runtime/cgo: ");
	va_start(ap, format);
	vfprintf(stderr, format, ap);
	va_end(ap);
	fprintf(stderr, "\n");

#ifdef __ANDROID__
	// When running from an Android .apk, /dev/stderr and /dev/stdout
	// redirect to /dev/null. And when running a test binary
	// via adb shell, it's easy to miss logcat. So write to both.
	va_start(ap, format);
	__android_log_vprint(ANDROID_LOG_FATAL, "runtime/cgo", format, ap);
	va_end(ap);
#endif

	abort();
}

```

// === FILE: references!/go/src/runtime/cgo/gcc_freebsd_sigaction.c ===
```text
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build freebsd && amd64

#include <errno.h>
#include <stddef.h>
#include <stdint.h>
#include <string.h>
#include <signal.h>

#include "libcgo.h"

// go_sigaction_t is a C version of the sigactiont struct from
// os_freebsd.go.  This definition — and its conversion to and from struct
// sigaction — are specific to freebsd/amd64.
typedef struct {
        uint32_t __bits[_SIG_WORDS];
} go_sigset_t;
typedef struct {
	uintptr_t handler;
	int32_t flags;
	go_sigset_t mask;
} go_sigaction_t;

int32_t
x_cgo_sigaction(intptr_t signum, const go_sigaction_t *goact, go_sigaction_t *oldgoact) {
	int32_t ret;
	struct sigaction act;
	struct sigaction oldact;
	size_t i;

	_cgo_tsan_acquire();

	memset(&act, 0, sizeof act);
	memset(&oldact, 0, sizeof oldact);

	if (goact) {
		if (goact->flags & SA_SIGINFO) {
			act.sa_sigaction = (void(*)(int, siginfo_t*, void*))(goact->handler);
		} else {
			act.sa_handler = (void(*)(int))(goact->handler);
		}
		sigemptyset(&act.sa_mask);
		for (i = 0; i < 8 * sizeof(goact->mask); i++) {
			if (goact->mask.__bits[i/32] & ((uint32_t)(1)<<(i&31))) {
				sigaddset(&act.sa_mask, i+1);
			}
		}
		act.sa_flags = goact->flags;
	}

	ret = sigaction(signum, goact ? &act : NULL, oldgoact ? &oldact : NULL);
	if (ret == -1) {
		// runtime.sigaction expects _cgo_sigaction to return errno on error.
		_cgo_tsan_release();
		return errno;
	}

	if (oldgoact) {
		if (oldact.sa_flags & SA_SIGINFO) {
			oldgoact->handler = (uintptr_t)(oldact.sa_sigaction);
		} else {
			oldgoact->handler = (uintptr_t)(oldact.sa_handler);
		}
		for (i = 0 ; i < _SIG_WORDS; i++) {
			oldgoact->mask.__bits[i] = 0;
		}
		for (i = 0; i < 8 * sizeof(oldgoact->mask); i++) {
			if (sigismember(&oldact.sa_mask, i+1) == 1) {
				oldgoact->mask.__bits[i/32] |= (uint32_t)(1)<<(i&31);
			}
		}
		oldgoact->flags = oldact.sa_flags;
	}

	_cgo_tsan_release();
	return ret;
}

```

// === FILE: references!/go/src/runtime/cgo/gcc_libinit_unix.c ===
```text
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix

// When cross-compiling with clang to linux/armv5, atomics are emulated
// and cause a compiler warning. This results in a build failure since
// cgo uses -Werror. See #65290.
#pragma GCC diagnostic ignored "-Wpragmas"
#pragma GCC diagnostic ignored "-Wunknown-warning-option"
#pragma GCC diagnostic ignored "-Watomic-alignment"

#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include "libcgo.h"
#include "libcgo_unix.h"

static pthread_cond_t runtime_init_cond = PTHREAD_COND_INITIALIZER;
static pthread_mutex_t runtime_init_mu = PTHREAD_MUTEX_INITIALIZER;
static int runtime_init_done;

// pthread_g is a pthread specific key, for storing the g that binded to the C thread.
// The registered pthread_key_destructor will dropm, when the pthread-specified value g is not NULL,
// while a C thread is exiting.
static pthread_key_t pthread_g;
static void pthread_key_destructor(void* g);
uintptr_t x_cgo_pthread_key_created;
void (*x_crosscall2_ptr)(void (*fn)(void *), void *, int, size_t);

// The traceback function, used when tracing C calls.
static void (*cgo_traceback_function)(struct cgoTracebackArg*);

// The context function, used when tracing back C calls into Go.
static void (*cgo_context_function)(struct cgoContextArg*);

// The symbolizer function, used when symbolizing C frames.
static void (*cgo_symbolizer_function)(struct cgoSymbolizerArg*);

uintptr_t
_cgo_wait_runtime_init_done(void) {
	void (*pfn)(struct cgoContextArg*);
	int done;

	pfn = __atomic_load_n(&cgo_context_function, __ATOMIC_CONSUME);

	done = 2;
	if (__atomic_load_n(&runtime_init_done, __ATOMIC_CONSUME) != done) {
		pthread_mutex_lock(&runtime_init_mu);
		while (__atomic_load_n(&runtime_init_done, __ATOMIC_CONSUME) == 0) {
			pthread_cond_wait(&runtime_init_cond, &runtime_init_mu);
		}

		// The key and x_cgo_pthread_key_created are for the whole program,
		// whereas the specific and destructor is per thread.
		if (x_cgo_pthread_key_created == 0 && pthread_key_create(&pthread_g, pthread_key_destructor) == 0) {
			x_cgo_pthread_key_created = 1;
		}

		// TODO(iant): For the case of a new C thread calling into Go, such
		// as when using -buildmode=c-archive, we know that Go runtime
		// initialization is complete but we do not know that all Go init
		// functions have been run. We should not fetch cgo_context_function
		// until they have been, because that is where a call to
		// SetCgoTraceback is likely to occur. We are going to wait for Go
		// initialization to be complete anyhow, later, by waiting for
		// main_init_done to be closed in cgocallbackg1. We should wait here
		// instead. See also issue #15943.
		pfn = __atomic_load_n(&cgo_context_function, __ATOMIC_CONSUME);

		__atomic_store_n(&runtime_init_done, done, __ATOMIC_RELEASE);
		pthread_mutex_unlock(&runtime_init_mu);
	}

	if (pfn != nil) {
		struct cgoContextArg arg;

		arg.Context = 0;
		(*pfn)(&arg);
		return arg.Context;
	}
	return 0;
}

// Store the g into a thread-specific value associated with the pthread key pthread_g.
// And pthread_key_destructor will dropm when the thread is exiting.
void x_cgo_bindm(void* g) {
	// We assume this will always succeed, otherwise, there might be extra M leaking,
	// when a C thread exits after a cgo call.
	// We only invoke this function once per thread in runtime.needAndBindM,
	// and the next calls just reuse the bound m.
	pthread_setspecific(pthread_g, g);
}

void (* _cgo_bindm)(void*) = x_cgo_bindm;

void
x_cgo_notify_runtime_init_done(void* dummy __attribute__ ((unused))) {
	pthread_mutex_lock(&runtime_init_mu);
	__atomic_store_n(&runtime_init_done, 1, __ATOMIC_RELEASE);
	pthread_cond_broadcast(&runtime_init_cond);
	pthread_mutex_unlock(&runtime_init_mu);
}

// Sets the traceback, context, and symbolizer functions. Called from
// runtime.SetCgoTraceback.
void x_cgo_set_traceback_functions(struct cgoSetTracebackFunctionsArg* arg) {
	__atomic_store_n(&cgo_traceback_function, arg->Traceback, __ATOMIC_RELEASE);
	__atomic_store_n(&cgo_context_function, arg->Context, __ATOMIC_RELEASE);
	__atomic_store_n(&cgo_symbolizer_function, arg->Symbolizer, __ATOMIC_RELEASE);
}

// Gets the traceback function to call to trace C calls.
void (*(_cgo_get_traceback_function(void)))(struct cgoTracebackArg*) {
	return __atomic_load_n(&cgo_traceback_function, __ATOMIC_CONSUME);
}

// Call the traceback function registered with x_cgo_set_traceback_functions.
//
// The traceback function is an arbitrary user C function which may be built
// with TSAN, and thus must be wrapped with TSAN acquire/release calls. For
// normal cgo calls, cmd/cgo automatically inserts TSAN acquire/release calls.
// Since the traceback, context, and symbolizer functions are registered at
// startup and called via the runtime, they do not get automatic TSAN
// acquire/release calls.
//
// The only purpose of this wrapper is to perform TSAN acquire/release.
// Alternatively, if the runtime arranged to safely call TSAN acquire/release,
// it could perform the call directly.
void x_cgo_call_traceback_function(struct cgoTracebackArg* arg) {
	void (*pfn)(struct cgoTracebackArg*);

	pfn = _cgo_get_traceback_function();
	if (pfn == nil) {
		return;
	}

	_cgo_tsan_acquire();
	(*pfn)(arg);
	_cgo_tsan_release();
}

// Gets the context function to call to record the traceback context
// when calling a Go function from C code.
void (*(_cgo_get_context_function(void)))(struct cgoContextArg*) {
	return __atomic_load_n(&cgo_context_function, __ATOMIC_CONSUME);
}

// Gets the symbolizer function to call to symbolize C frames.
void (*(_cgo_get_symbolizer_function(void)))(struct cgoSymbolizerArg*) {
	return __atomic_load_n(&cgo_symbolizer_function, __ATOMIC_CONSUME);
}

// Call the symbolizer function registered with x_cgo_set_traceback_functions.
//
// See comment on x_cgo_call_traceback_function.
void x_cgo_call_symbolizer_function(struct cgoSymbolizerArg* arg) {
	void (*pfn)(struct cgoSymbolizerArg*);

	pfn = _cgo_get_symbolizer_function();
	if (pfn == nil) {
		return;
	}

	_cgo_tsan_acquire();
	(*pfn)(arg);
	_cgo_tsan_release();
}

static void
pthread_key_destructor(void* g) {
	if (x_crosscall2_ptr != NULL) {
		// fn == NULL means dropm.
		// We restore g by using the stored g, before dropm in runtime.cgocallback,
		// since the g stored in the TLS by Go might be cleared in some platforms,
		// before this destructor invoked.
		x_crosscall2_ptr(NULL, g, 0, 0);
	}
}

void
x_cgo_thread_start(ThreadStart *arg)
{
	ThreadStart *ts;

	/* Make our own copy that can persist after we return. */
	_cgo_tsan_acquire();
	ts = malloc(sizeof *ts);
	_cgo_tsan_release();
	if(ts == nil) {
		fprintf(stderr, "runtime/cgo: out of memory in thread_start\n");
		abort();
	}
	*ts = *arg;

	_cgo_sys_thread_start(ts);	/* OS-dependent half */
}

void (* _cgo_thread_start)(ThreadStart*) = x_cgo_thread_start;

```

// === FILE: references!/go/src/runtime/cgo/gcc_libinit_windows.c ===
```text
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#ifdef __CYGWIN__
#error "don't use the cygwin compiler to build native Windows programs; use MinGW instead"
#endif

#define WIN32_LEAN_AND_MEAN
#include <windows.h>

#include <stdio.h>
#include <stdlib.h>

#include "libcgo.h"

#define IMAGE_GUARD_SECURITY_COOKIE_UNUSED 0x00000800
// With modern mingw, we can use the normal struct:
//
// const IMAGE_LOAD_CONFIG_DIRECTORY _load_config_used = {
// 	.Size = sizeof(_load_config_used),
// 	.GuardFlags = IMAGE_GUARD_SECURITY_COOKIE_UNUSED
// };
//
// But we support older toolchains, so instead, fix the offsets:
#ifdef _WIN64
const ULONGLONG _load_config_used[40] = {
	sizeof(_load_config_used),
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	IMAGE_GUARD_SECURITY_COOKIE_UNUSED
};
#else
const DWORD _load_config_used[48] = {
	sizeof(_load_config_used),
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	IMAGE_GUARD_SECURITY_COOKIE_UNUSED
};
#endif

static volatile LONG runtime_init_once_gate = 0;
static volatile LONG runtime_init_once_done = 0;

static CRITICAL_SECTION runtime_init_cs;

static HANDLE runtime_init_wait;
static int runtime_init_done;

// No pthreads on Windows, these are always zero.
uintptr_t x_cgo_pthread_key_created;
void (*x_crosscall2_ptr)(void (*fn)(void *), void *, int, size_t);
void (*_cgo_init)(G*, void (*)(void*), void **, void **);
void (*_cgo_thread_start)(ThreadStart *);
void (*_cgo_sys_thread_create)(void* (*func)(void*));
void (*_cgo_getstackbound)(uintptr[2]);
void (*_cgo_bindm)(void*);

// Pre-initialize the runtime synchronization objects
void
_cgo_preinit_init() {
	 runtime_init_wait = CreateEvent(NULL, TRUE, FALSE, NULL);
	 if (runtime_init_wait == NULL) {
		fprintf(stderr, "runtime: failed to create runtime initialization wait event.\n");
		abort();
	 }

	 InitializeCriticalSection(&runtime_init_cs);
}

// Make sure that the preinit sequence has run.
void
_cgo_maybe_run_preinit() {
	 if (!InterlockedExchangeAdd(&runtime_init_once_done, 0)) {
			if (InterlockedIncrement(&runtime_init_once_gate) == 1) {
				 _cgo_preinit_init();
				 InterlockedIncrement(&runtime_init_once_done);
			} else {
				 // Decrement to avoid overflow.
				 InterlockedDecrement(&runtime_init_once_gate);
				 while(!InterlockedExchangeAdd(&runtime_init_once_done, 0)) {
						Sleep(0);
				 }
			}
	 }
}

int
_cgo_is_runtime_initialized() {
	 int status;

	 EnterCriticalSection(&runtime_init_cs);
	 status = runtime_init_done;
	 LeaveCriticalSection(&runtime_init_cs);
	 return status;
}

uintptr_t
_cgo_wait_runtime_init_done(void) {
	void (*pfn)(struct cgoContextArg*);

	 _cgo_maybe_run_preinit();
	while (!_cgo_is_runtime_initialized()) {
			WaitForSingleObject(runtime_init_wait, INFINITE);
	}
	pfn = _cgo_get_context_function();
	if (pfn != nil) {
		struct cgoContextArg arg;

		arg.Context = 0;
		(*pfn)(&arg);
		return arg.Context;
	}
	return 0;
}

void
x_cgo_notify_runtime_init_done(void* dummy) {
	 _cgo_maybe_run_preinit();

	 EnterCriticalSection(&runtime_init_cs);
	runtime_init_done = 1;
	 LeaveCriticalSection(&runtime_init_cs);

	 if (!SetEvent(runtime_init_wait)) {
		fprintf(stderr, "runtime: failed to signal runtime initialization complete.\n");
		abort();
	}
}

// The traceback function, used when tracing C calls.
static void (*cgo_traceback_function)(struct cgoTracebackArg*);

// The context function, used when tracing back C calls into Go.
static void (*cgo_context_function)(struct cgoContextArg*);

// The symbolizer function, used when symbolizing C frames.
static void (*cgo_symbolizer_function)(struct cgoSymbolizerArg*);

// Sets the traceback, context, and symbolizer functions. Called from
// runtime.SetCgoTraceback.
void x_cgo_set_traceback_functions(struct cgoSetTracebackFunctionsArg* arg) {
	EnterCriticalSection(&runtime_init_cs);
	cgo_traceback_function = arg->Traceback;
	cgo_context_function = arg->Context;
	cgo_symbolizer_function = arg->Symbolizer;
	LeaveCriticalSection(&runtime_init_cs);
}

// Gets the traceback function to call to trace C calls.
void (*(_cgo_get_traceback_function(void)))(struct cgoTracebackArg*) {
	void (*ret)(struct cgoTracebackArg*);

	EnterCriticalSection(&runtime_init_cs);
	ret = cgo_traceback_function;
	LeaveCriticalSection(&runtime_init_cs);
	return ret;
}

// Call the traceback function registered with x_cgo_set_traceback_functions.
//
// On other platforms, this coordinates with C/C++ TSAN. On Windows, there is
// no C/C++ TSAN.
void x_cgo_call_traceback_function(struct cgoTracebackArg* arg) {
	void (*pfn)(struct cgoTracebackArg*);

	pfn = _cgo_get_traceback_function();
	if (pfn == nil) {
		return;
	}

	(*pfn)(arg);
}

// Gets the context function to call to record the traceback context
// when calling a Go function from C code.
void (*(_cgo_get_context_function(void)))(struct cgoContextArg*) {
	void (*ret)(struct cgoContextArg*);

	EnterCriticalSection(&runtime_init_cs);
	ret = cgo_context_function;
	LeaveCriticalSection(&runtime_init_cs);
	return ret;
}

// Gets the symbolizer function to call to symbolize C frames.
void (*(_cgo_get_symbolizer_function(void)))(struct cgoSymbolizerArg*) {
	void (*ret)(struct cgoSymbolizerArg*);

	EnterCriticalSection(&runtime_init_cs);
	ret = cgo_symbolizer_function;
	LeaveCriticalSection(&runtime_init_cs);
	return ret;
}

// Call the symbolizer function registered with x_cgo_set_symbolizer_functions.
//
// On other platforms, this coordinates with C/C++ TSAN. On Windows, there is
// no C/C++ TSAN.
void x_cgo_call_symbolizer_function(struct cgoSymbolizerArg* arg) {
	void (*pfn)(struct cgoSymbolizerArg*);

	pfn = _cgo_get_symbolizer_function();
	if (pfn == nil) {
		return;
	}

	(*pfn)(arg);
}

```

// === FILE: references!/go/src/runtime/cgo/gcc_linux_ppc64x.S ===
```text
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux && (ppc64 || ppc64le)

.file "gcc_linux_ppc64x.S"

// Define a frame which has no argument space, but is compatible with
// a call into a Go ABI. We allocate 32B to match FIXED_FRAME with
// similar semantics, except we store the backchain pointer, not the
// LR at offset 0. R2 is stored in the Go TOC save slot (offset 24).
.set GPR_OFFSET, 32
.set FPR_OFFSET, GPR_OFFSET + 18*8
.set VR_OFFSET, FPR_OFFSET + 18*8
.set FRAME_SIZE, VR_OFFSET + 12*16

.macro FOR_EACH_GPR opcode r=14
.ifge 31 - \r
	\opcode \r, GPR_OFFSET + 8*(\r-14)(1)
	FOR_EACH_GPR \opcode "(\r+1)"
.endif
.endm

.macro FOR_EACH_FPR opcode fr=14
.ifge 31 - \fr
	\opcode \fr, FPR_OFFSET + 8*(\fr-14)(1)
	FOR_EACH_FPR \opcode "(\fr+1)"
.endif
.endm

.macro FOR_EACH_VR opcode vr=20
.ifge 31 - \vr
	li 0, VR_OFFSET + 16*(\vr-20)
	\opcode \vr, 1, 0
	FOR_EACH_VR \opcode "(\vr+1)"
.endif
.endm

/*
 * void crosscall1(void (*fn)(void), void (*setg_gcc)(void*), void *g)
 *
 * Calling into the gc tool chain, where all registers are caller save.
 * Called from standard ppc64 C ABI, where r2, r14-r31, f14-f31 are
 * callee-save, so they must be saved explicitly.
 * ABI: r3=fn, r4=setg_gcc, r5=g on entry.
 */
.globl crosscall1
crosscall1:
	// Start with standard C stack frame layout and linkage
	mflr	%r0
	std	%r0, 16(%r1)	// Save LR in caller's frame
	mfcr	%r0
	std	%r0, 8(%r1)	// Save CR in caller's frame
	stdu	%r1, -FRAME_SIZE(%r1)
	std	%r2, 24(%r1)

	FOR_EACH_GPR std
	FOR_EACH_FPR stfd
	FOR_EACH_VR stvx

	// Set up Go ABI constant registers
	li	%r0, 0

	// Save fn in r14 (callee-save) while we call setg_gcc
	mr	%r14, %r3
	// Restore g pointer (r30 in Go ABI, which may have been clobbered by C)
	mr	%r30, %r5

	// Call setg_gcc(g)
	mr	%r3, %r5        /* arg: g */
	mr	%r12, %r4       /* setg_gcc */
	mtctr	%r12
	bctrl

	// Call fn()
	mr	%r12, %r14
	mtctr	%r12
	bctrl

	FOR_EACH_GPR ld
	FOR_EACH_FPR lfd
	FOR_EACH_VR lvx

	ld	%r2, 24(%r1)
	addi	%r1, %r1, FRAME_SIZE
	ld	%r0, 16(%r1)
	mtlr	%r0
	ld	%r0, 8(%r1)
	mtcr	%r0
	blr

#ifdef __ELF__
.section .note.GNU-stack,"",%progbits
#endif

```

// === FILE: references!/go/src/runtime/cgo/gcc_loong64.S ===
```text
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

.file "gcc_loong64.S"

/*
 * void crosscall1(void (*fn)(void), void (*setg_gcc)(void *g), void *g)
 *
 * Calling into the gc tool chain, where all registers are caller save.
 * Called from standard lp64d ABI, where $r1, $r3, $r22-$r31, and $f24-$f31
 * are callee-save, so they must be saved explicitly, along with $r1 (LR).
 */
.globl crosscall1
crosscall1:
	addi.d	$r3, $r3, -160
	st.d	$r1, $r3, 0
	st.d	$r22, $r3, 8
	st.d	$r23, $r3, 16
	st.d	$r24, $r3, 24
	st.d	$r25, $r3, 32
	st.d	$r26, $r3, 40
	st.d	$r27, $r3, 48
	st.d	$r28, $r3, 56
	st.d	$r29, $r3, 64
	st.d	$r30, $r3, 72
	st.d	$r31, $r3, 80
	fst.d	$f24, $r3, 88
	fst.d	$f25, $r3, 96
	fst.d	$f26, $r3, 104
	fst.d	$f27, $r3, 112
	fst.d	$f28, $r3, 120
	fst.d	$f29, $r3, 128
	fst.d	$f30, $r3, 136
	fst.d	$f31, $r3, 144

	// r4 = *fn, r5 = *setg_gcc, r6 = *g
	move	$r23, $r4	// save R4
	move	$r4, $r6
	jirl	$r1, $r5, 0	// call setg_gcc (clobbers R4)
	jirl	$r1, $r23, 0	// call fn

	ld.d	$r22, $r3, 8
	ld.d	$r23, $r3, 16
	ld.d	$r24, $r3, 24
	ld.d	$r25, $r3, 32
	ld.d	$r26, $r3, 40
	ld.d	$r27, $r3, 48
	ld.d	$r28, $r3, 56
	ld.d	$r29, $r3, 64
	ld.d	$r30, $r3, 72
	ld.d	$r31, $r3, 80
	fld.d	$f24, $r3, 88
	fld.d	$f25, $r3, 96
	fld.d	$f26, $r3, 104
	fld.d	$f27, $r3, 112
	fld.d	$f28, $r3, 120
	fld.d	$f29, $r3, 128
	fld.d	$f30, $r3, 136
	fld.d	$f31, $r3, 144
	ld.d	$r1, $r3, 0
	addi.d	$r3, $r3, 160
	jirl	$r0, $r1, 0


#ifdef __ELF__
.section .note.GNU-stack,"",%progbits
#endif

```

// === FILE: references!/go/src/runtime/cgo/gcc_mips64x.S ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build mips64 || mips64le

.file "gcc_mips64x.S"

/*
 * void crosscall1(void (*fn)(void), void (*setg_gcc)(void *g), void *g)
 *
 * Calling into the gc tool chain, where all registers are caller save.
 * Called from standard MIPS N64 ABI, where $16-$23, $28, $30, and $f24-$f31
 * are callee-save, so they must be saved explicitly, along with $31 (LR).
 */
.globl crosscall1
.set noat
crosscall1:
#ifndef __mips_soft_float
	daddiu	$29, $29, -160
#else
	daddiu	$29, $29, -96 // For soft-float, no need to make room for FP registers
#endif
	sd	$31, 0($29)
	sd	$16, 8($29)
	sd	$17, 16($29)
	sd	$18, 24($29)
	sd	$19, 32($29)
	sd	$20, 40($29)
	sd	$21, 48($29)
	sd	$22, 56($29)
	sd	$23, 64($29)
	sd	$28, 72($29)
	sd	$30, 80($29)
#ifndef __mips_soft_float
	sdc1	$f24, 88($29)
	sdc1	$f25, 96($29)
	sdc1	$f26, 104($29)
	sdc1	$f27, 112($29)
	sdc1	$f28, 120($29)
	sdc1	$f29, 128($29)
	sdc1	$f30, 136($29)
	sdc1	$f31, 144($29)
#endif

	// prepare SB register = pc & 0xffffffff00000000
	bal	1f
1:
	dsrl	$28, $31, 32
	dsll	$28, $28, 32

	move	$20, $4 // save R4
	move	$1, $6
	jalr	$5	// call setg_gcc (clobbers R4)
	jalr	$20	// call fn

	ld	$16, 8($29)
	ld	$17, 16($29)
	ld	$18, 24($29)
	ld	$19, 32($29)
	ld	$20, 40($29)
	ld	$21, 48($29)
	ld	$22, 56($29)
	ld	$23, 64($29)
	ld	$28, 72($29)
	ld	$30, 80($29)
#ifndef __mips_soft_float
	ldc1	$f24, 88($29)
	ldc1	$f25, 96($29)
	ldc1	$f26, 104($29)
	ldc1	$f27, 112($29)
	ldc1	$f28, 120($29)
	ldc1	$f29, 128($29)
	ldc1	$f30, 136($29)
	ldc1	$f31, 144($29)
#endif
	ld	$31, 0($29)
#ifndef __mips_soft_float
	daddiu	$29, $29, 160
#else
	daddiu	$29, $29, 96
#endif
	jr	$31

.set at

#ifdef __ELF__
.section .note.GNU-stack,"",%progbits
#endif

```

// === FILE: references!/go/src/runtime/cgo/gcc_mipsx.S ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build mips || mipsle

.file "gcc_mipsx.S"

/*
 * void crosscall1(void (*fn)(void), void (*setg_gcc)(void *g), void *g)
 *
 * Calling into the gc tool chain, where all registers are caller save.
 * Called from standard MIPS O32 ABI, where $16-$23, $30, and $f20-$f31
 * are callee-save, so they must be saved explicitly, along with $31 (LR).
 */
.globl crosscall1
.set noat
crosscall1:
#ifndef __mips_soft_float
	addiu	$29, $29, -88
#else
	addiu	$29, $29, -40 // For soft-float, no need to make room for FP registers
#endif
	sw	$31, 0($29)
	sw	$16, 4($29)
	sw	$17, 8($29)
	sw	$18, 12($29)
	sw	$19, 16($29)
	sw	$20, 20($29)
	sw	$21, 24($29)
	sw	$22, 28($29)
	sw	$23, 32($29)
	sw	$30, 36($29)

#ifndef __mips_soft_float
	sdc1	$f20, 40($29)
	sdc1	$f22, 48($29)
	sdc1	$f24, 56($29)
	sdc1	$f26, 64($29)
	sdc1	$f28, 72($29)
	sdc1	$f30, 80($29)
#endif
	move	$20, $4 // save R4
	move	$4, $6
	jalr	$5	// call setg_gcc
	jalr	$20	// call fn

	lw	$16, 4($29)
	lw	$17, 8($29)
	lw	$18, 12($29)
	lw	$19, 16($29)
	lw	$20, 20($29)
	lw	$21, 24($29)
	lw	$22, 28($29)
	lw	$23, 32($29)
	lw	$30, 36($29)
#ifndef __mips_soft_float
	ldc1	$f20, 40($29)
	ldc1	$f22, 48($29)
	ldc1	$f24, 56($29)
	ldc1	$f26, 64($29)
	ldc1	$f28, 72($29)
	ldc1	$f30, 80($29)
#endif
	lw	$31, 0($29)
#ifndef __mips_soft_float
	addiu	$29, $29, 88
#else
	addiu	$29, $29, 40
#endif
	jr	$31

.set at

#ifdef __ELF__
.section .note.GNU-stack,"",%progbits
#endif

```

// === FILE: references!/go/src/runtime/cgo/gcc_mmap.c ===
```text
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build (linux && (amd64 || arm64 || loong64 || ppc64le)) || (freebsd && amd64)

#include <errno.h>
#include <stdint.h>
#include <stdlib.h>
#include <sys/mman.h>

#include "libcgo.h"

uintptr_t
x_cgo_mmap(void *addr, uintptr_t length, int32_t prot, int32_t flags, int32_t fd, uint32_t offset) {
	void *p;

	_cgo_tsan_acquire();
	p = mmap(addr, length, prot, flags, fd, offset);
	_cgo_tsan_release();
	if (p == MAP_FAILED) {
		/* This is what the Go code expects on failure.  */
		return (uintptr_t)errno;
	}
	return (uintptr_t)p;
}

void
x_cgo_munmap(void *addr, uintptr_t length) {
	int r;

	_cgo_tsan_acquire();
	r = munmap(addr, length);
	_cgo_tsan_release();
	if (r < 0) {
		/* The Go runtime is not prepared for munmap to fail.  */
		abort();
	}
}

```

// === FILE: references!/go/src/runtime/cgo/gcc_netbsd.c ===
```text
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build netbsd

#include <string.h>
#include <signal.h>

static void
threadentry_platform(void)
{
	// On NetBSD, a new thread inherits the signal stack of the
	// creating thread. That confuses minit, so we remove that
	// signal stack here before calling the regular mstart. It's
	// a bit baroque to remove a signal stack here only to add one
	// in minit, but it's a simple change that keeps NetBSD
	// working like other OS's. At this point all signals are
	// blocked, so there is no race.
	stack_t ss;
	memset(&ss, 0, sizeof ss);
	ss.ss_flags = SS_DISABLE;
	sigaltstack(&ss, NULL);
}

void (*x_cgo_threadentry_platform)(void) = threadentry_platform;

```

// === FILE: references!/go/src/runtime/cgo/gcc_riscv64.S ===
```text
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

.file "gcc_riscv64.S"

/*
 * void crosscall1(void (*fn)(void), void (*setg_gcc)(void *g), void *g)
 *
 * Calling into the gc tool chain, where all registers are caller save.
 * Called from standard RISCV ELF psABI, where x8-x9, x18-x27, f8-f9 and
 * f18-f27 are callee-save, so they must be saved explicitly, along with
 * x1 (LR).
 */
.globl crosscall1
crosscall1:
	sd	x1, -200(sp)
	addi	sp, sp, -200
	sd	x8, 8(sp)
	sd	x9, 16(sp)
	sd	x18, 24(sp)
	sd	x19, 32(sp)
	sd	x20, 40(sp)
	sd	x21, 48(sp)
	sd	x22, 56(sp)
	sd	x23, 64(sp)
	sd	x24, 72(sp)
	sd	x25, 80(sp)
	sd	x26, 88(sp)
	sd	x27, 96(sp)
	fsd	f8, 104(sp)
	fsd	f9, 112(sp)
	fsd	f18, 120(sp)
	fsd	f19, 128(sp)
	fsd	f20, 136(sp)
	fsd	f21, 144(sp)
	fsd	f22, 152(sp)
	fsd	f23, 160(sp)
	fsd	f24, 168(sp)
	fsd	f25, 176(sp)
	fsd	f26, 184(sp)
	fsd	f27, 192(sp)

	// a0 = *fn, a1 = *setg_gcc, a2 = *g
	mv	s1, a0
	mv	s0, a1
	mv	a0, a2
	jalr	ra, s0	// call setg_gcc (clobbers x30 aka g)
	jalr	ra, s1	// call fn

	ld	x1, 0(sp)
	ld	x8, 8(sp)
	ld	x9, 16(sp)
	ld	x18, 24(sp)
	ld	x19, 32(sp)
	ld	x20, 40(sp)
	ld	x21, 48(sp)
	ld	x22, 56(sp)
	ld	x23, 64(sp)
	ld	x24, 72(sp)
	ld	x25, 80(sp)
	ld	x26, 88(sp)
	ld	x27, 96(sp)
	fld	f8, 104(sp)
	fld	f9, 112(sp)
	fld	f18, 120(sp)
	fld	f19, 128(sp)
	fld	f20, 136(sp)
	fld	f21, 144(sp)
	fld	f22, 152(sp)
	fld	f23, 160(sp)
	fld	f24, 168(sp)
	fld	f25, 176(sp)
	fld	f26, 184(sp)
	fld	f27, 192(sp)
	addi	sp, sp, 200

	jr	ra

#ifdef __ELF__
.section .note.GNU-stack,"",%progbits
#endif

```

// === FILE: references!/go/src/runtime/cgo/gcc_s390x.S ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

.file "gcc_s390x.S"

/*
 * void crosscall1(void (*fn)(void), void (*setg_gcc)(void*), void *g)
 *
 * Calling into the go tool chain, where all registers are caller save.
 * Called from standard s390x C ABI, where r6-r13, r15, and f8-f15 are
 * callee-save, so they must be saved explicitly.
 * ABI: r2=fn, r3=setg_gcc, r4=g.
 */
.globl crosscall1
crosscall1:
	/* save r6-r15 in the register save area of the calling function */
	stmg    %r6, %r15, 48(%r15)

	/* allocate 64 bytes of stack space to save f8-f15 */
	lay     %r15, -64(%r15)

	/* save callee-saved floating point registers */
	std     %f8, 0(%r15)
	std     %f9, 8(%r15)
	std     %f10, 16(%r15)
	std     %f11, 24(%r15)
	std     %f12, 32(%r15)
	std     %f13, 40(%r15)
	std     %f14, 48(%r15)
	std     %f15, 56(%r15)

	/* save fn pointer in r6 (already saved to stack) */
	lgr     %r6, %r2
	/* restore g pointer */
	lgr     %r13, %r4

	/* prepare arg for setg_gcc: first argument in r2 */
	lgr     %r2, %r4      /* r2 = g */
	/* call setg_gcc(g) using function pointer in r3 */
	basr    %r14, %r3

	/* call fn */
	lgr     %r2, %r6
	basr    %r14, %r2

	/* restore floating point registers */
	ld      %f8, 0(%r15)
	ld      %f9, 8(%r15)
	ld      %f10, 16(%r15)
	ld      %f11, 24(%r15)
	ld      %f12, 32(%r15)
	ld      %f13, 40(%r15)
	ld      %f14, 48(%r15)
	ld      %f15, 56(%r15)

	/* de-allocate stack frame */
	la      %r15, 64(%r15)

	/* restore general purpose registers */
	lmg     %r6, %r15, 48(%r15)

	br      %r14 /* restored by lmg */

#ifdef __ELF__
.section .note.GNU-stack,"",%progbits
#endif

```

// === FILE: references!/go/src/runtime/cgo/gcc_setenv.c ===
```text
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix

#include "libcgo.h"

#include <stdlib.h>

/* Stub for calling setenv */
void
x_cgo_setenv(char **arg)
{
	_cgo_tsan_acquire();
	setenv(arg[0], arg[1], 1);
	_cgo_tsan_release();
}

/* Stub for calling unsetenv */
void
x_cgo_unsetenv(char **arg)
{
	_cgo_tsan_acquire();
	unsetenv(arg[0]);
	_cgo_tsan_release();
}

```

// === FILE: references!/go/src/runtime/cgo/gcc_sigaction.c ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux && (386 || amd64 || arm64 || loong64 || ppc64 || ppc64le)

#include <errno.h>
#include <stddef.h>
#include <stdint.h>
#include <string.h>
#include <signal.h>

#include "libcgo.h"

// go_sigaction_t is a C version of the sigactiont struct from
// defs_${goos}_${goarch}.go.  This definition — and its conversion
// to and from struct sigaction — are specific to ${goos}/${goarch}.
typedef struct {
	uintptr_t handler;
	unsigned long flags;
#ifdef __loongarch__
	uint64_t mask;
	uintptr_t restorer;
#else
	uintptr_t restorer;
	uint64_t mask;
#endif
} go_sigaction_t;

// SA_RESTORER is part of the kernel interface.
// This is Linux i386/amd64 specific.
#ifndef SA_RESTORER
#define SA_RESTORER 0x4000000
#endif

int32_t
x_cgo_sigaction(intptr_t signum, const go_sigaction_t *goact, go_sigaction_t *oldgoact) {
	int32_t ret;
	struct sigaction act;
	struct sigaction oldact;
	size_t i;

	_cgo_tsan_acquire();

	memset(&act, 0, sizeof act);
	memset(&oldact, 0, sizeof oldact);

	if (goact) {
		if (goact->flags & SA_SIGINFO) {
			act.sa_sigaction = (void(*)(int, siginfo_t*, void*))(goact->handler);
		} else {
			act.sa_handler = (void(*)(int))(goact->handler);
		}
		sigemptyset(&act.sa_mask);
		for (i = 0; i < 8 * sizeof(goact->mask); i++) {
			if (goact->mask & ((uint64_t)(1)<<i)) {
				sigaddset(&act.sa_mask, (int)(i+1));
			}
		}
		act.sa_flags = (int)(goact->flags & ~(unsigned long)SA_RESTORER);
	}

	ret = sigaction((int)signum, goact ? &act : NULL, oldgoact ? &oldact : NULL);
	if (ret == -1) {
		// runtime.rt_sigaction expects _cgo_sigaction to return errno on error.
		_cgo_tsan_release();
		return errno;
	}

	if (oldgoact) {
		if (oldact.sa_flags & SA_SIGINFO) {
			oldgoact->handler = (uintptr_t)(oldact.sa_sigaction);
		} else {
			oldgoact->handler = (uintptr_t)(oldact.sa_handler);
		}
		oldgoact->mask = 0;
		for (i = 0; i < 8 * sizeof(oldgoact->mask); i++) {
			if (sigismember(&oldact.sa_mask, (int)(i+1)) == 1) {
				oldgoact->mask |= (uint64_t)(1)<<i;
			}
		}
		oldgoact->flags = (unsigned long)oldact.sa_flags;
	}

	_cgo_tsan_release();
	return ret;
}

```

// === FILE: references!/go/src/runtime/cgo/gcc_traceback.c ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin || linux

#include <stdint.h>
#include "libcgo.h"

#ifndef __has_feature
#define __has_feature(x) 0
#endif

#if __has_feature(memory_sanitizer)
#include <sanitizer/msan_interface.h>
#endif

// Call the user's traceback function and then call sigtramp.
// The runtime signal handler will jump to this code.
// We do it this way so that the user's traceback function will be called
// by a C function with proper unwind info.
void
x_cgo_callers(uintptr_t sig, void *info, void *context, void (*cgoTraceback)(struct cgoTracebackArg*), uintptr_t* cgoCallers, void (*sigtramp)(uintptr_t, void*, void*)) {
	struct cgoTracebackArg arg;

	arg.Context = 0;
	arg.SigContext = (uintptr_t)(context);
	arg.Buf = cgoCallers;
	arg.Max = 32; // must match len(runtime.cgoCallers)

#if __has_feature(memory_sanitizer)
        // This function is called directly from the signal handler.
        // The arguments are passed in registers, so whether msan
        // considers cgoCallers to be initialized depends on whether
        // it considers the appropriate register to be initialized.
        // That can cause false reports in rare cases.
        // Explicitly unpoison the memory to avoid that.
        // See issue #47543 for more details.
        __msan_unpoison(&arg, sizeof arg);
#endif

	_cgo_tsan_acquire();
	(*cgoTraceback)(&arg);
	_cgo_tsan_release();
	sigtramp(sig, info, context);
}

```

// === FILE: references!/go/src/runtime/cgo/gcc_unix.c ===
```text
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix

#include "libcgo.h"
#include "libcgo_unix.h"

// Platform-specific hooks.
void (*x_cgo_inittls)(void **tlsg, void **tlsbase) __attribute__((weak));
void (*x_cgo_threadentry_platform)(void) __attribute__((weak));

static void (*setg_gcc)(void*);

// _cgo_set_stacklo sets g->stacklo based on the stack size.
// This is common code called from x_cgo_init, which is itself
// called by rt0_go in the runtime package.
static void
_cgo_set_stacklo(G *g)
{
	uintptr bounds[2];

	x_cgo_getstackbound(bounds);

	g->stacklo = bounds[0];

	// Sanity check the results now, rather than getting a
	// morestack on g0 crash.
	if (g->stacklo >= g->stackhi) {
		fprintf(stderr, "runtime/cgo: bad stack bounds: lo=%p hi=%p\n", (void*)(g->stacklo), (void*)(g->stackhi));
		abort();
	}
}

void
x_cgo_init(G *g, void (*setg)(void*), void **tlsg, void **tlsbase)
{
	setg_gcc = setg;
	_cgo_set_stacklo(g);

	if (x_cgo_inittls) {
		x_cgo_inittls(tlsg, tlsbase);
	}
}

void (* _cgo_init)(G*, void (*)(void*), void **, void **) = x_cgo_init;

void*
threadentry(void *v)
{
	ThreadStart ts;

	ts = *(ThreadStart*)v;
	_cgo_tsan_acquire();
	free(v);
	_cgo_tsan_release();

	if (x_cgo_threadentry_platform != NULL) {
		x_cgo_threadentry_platform();
	}

	crosscall1(ts.fn, setg_gcc, ts.g);
	return NULL;
}

```

// === FILE: references!/go/src/runtime/cgo/gcc_util.c ===
```text
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "libcgo.h"

#ifndef CGO_TSAN
void(* const _cgo_yield)() = NULL;
#else

#include <string.h>

char x_cgo_yield_strncpy_src = 0;
char x_cgo_yield_strncpy_dst = 0;
size_t x_cgo_yield_strncpy_n = 0;

/*
Stub for allowing libc interceptors to execute.

_cgo_yield is set to NULL if we do not expect libc interceptors to exist.
*/
static void
x_cgo_yield()
{
	/*
	The libc function(s) we call here must form a no-op and include at least one
	call that triggers TSAN to process pending asynchronous signals.

	sleep(0) would be fine, but it's not portable C (so it would need more header
	guards).
	free(NULL) has a fast-path special case in TSAN, so it doesn't
	trigger signal delivery.
	free(malloc(0)) would work (triggering the interceptors in malloc), but
	it also runs a bunch of user-supplied malloc hooks.

	So we choose strncpy(_, _, 0): it requires an extra header,
	but it's standard and should be very efficient.

	GCC 7 has an unfortunate habit of optimizing out strncpy calls (see
	https://golang.org/issue/21196), so the arguments here need to be global
	variables with external linkage in order to ensure that the call traps all the
	way down into libc.
	*/
	strncpy(&x_cgo_yield_strncpy_dst, &x_cgo_yield_strncpy_src,
	        x_cgo_yield_strncpy_n);
}

void(* const _cgo_yield)() = &x_cgo_yield;

#endif  /* GO_TSAN */

```

// === FILE: references!/go/src/runtime/cgo/handle.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cgo

import (
	"sync"
	"sync/atomic"
)

// Handle provides a way to pass values that contain Go pointers
// (pointers to memory allocated by Go) between Go and C without
// breaking the cgo pointer passing rules. A Handle is an integer
// value that can represent any Go value. A Handle can be passed
// through C and back to Go, and Go code can use the Handle to
// retrieve the original Go value.
//
// The underlying type of Handle is guaranteed to fit in an integer type
// that is large enough to hold the bit pattern of any pointer. The zero
// value of a Handle is not valid, and thus is safe to use as a sentinel
// in C APIs.
//
// For instance, on the Go side:
//
//	package main
//
//	/*
//	#include <stdint.h> // for uintptr_t
//
//	extern void MyGoPrint(uintptr_t handle);
//	void myprint(uintptr_t handle);
//	*/
//	import "C"
//	import "runtime/cgo"
//
//	//export MyGoPrint
//	func MyGoPrint(handle C.uintptr_t) {
//		h := cgo.Handle(handle)
//		val := h.Value().(string)
//		println(val)
//		h.Delete()
//	}
//
//	func main() {
//		val := "hello Go"
//		C.myprint(C.uintptr_t(cgo.NewHandle(val)))
//		// Output: hello Go
//	}
//
// and on the C side:
//
//	#include <stdint.h> // for uintptr_t
//
//	// A Go function
//	extern void MyGoPrint(uintptr_t handle);
//
//	// A C function
//	void myprint(uintptr_t handle) {
//	    MyGoPrint(handle);
//	}
//
// Some C functions accept a void* argument that points to an arbitrary
// data value supplied by the caller. It is not safe to coerce a Handle
// (an integer) to a Go [unsafe.Pointer], but instead we can pass the address
// of the cgo.Handle to the void* parameter, as in this variant of the
// previous example.
//
// Note that, as described in the [cmd/cgo] documentation,
// the C code must not keep a copy of the Go pointer that it receives,
// unless the memory is explicitly pinned using [runtime.Pinner].
// This example is OK because the C function myprint does not keep
// a copy of the pointer.
//
//	package main
//
//	/*
//	extern void MyGoPrint(void *context);
//	static inline void myprint(void *context) {
//	    MyGoPrint(context);
//	}
//	*/
//	import "C"
//	import (
//		"runtime/cgo"
//		"unsafe"
//	)
//
//	//export MyGoPrint
//	func MyGoPrint(context unsafe.Pointer) {
//		h := *(*cgo.Handle)(context)
//		val := h.Value().(string)
//		println(val)
//		h.Delete()
//	}
//
//	func main() {
//		val := "hello Go"
//		h := cgo.NewHandle(val)
//		// In this example, unsafe.Pointer(&h) is valid because myprint
//		// does not keep a copy of the pointer. If the C code keeps the
//		// pointer after the call returns, use runtime.Pinner to pin it.
//		C.myprint(unsafe.Pointer(&h))
//		// Output: hello Go
//	}
type Handle uintptr

// NewHandle returns a handle for a given value.
//
// The handle is valid until the program calls Delete on it. The handle
// uses resources, and this package assumes that C code may hold on to
// the handle, so a program must explicitly call Delete when the handle
// is no longer needed.
//
// The intended use is to pass the returned handle to C code, which
// passes it back to Go, which calls Value.
func NewHandle(v any) Handle {
	h := handleIdx.Add(1)
	if h == 0 {
		panic("runtime/cgo: ran out of handle space")
	}

	handles.Store(h, v)
	return Handle(h)
}

// Value returns the associated Go value for a valid handle.
//
// The method panics if the handle is invalid.
func (h Handle) Value() any {
	v, ok := handles.Load(uintptr(h))
	if !ok {
		panic("runtime/cgo: misuse of an invalid Handle")
	}
	return v
}

// Delete invalidates a handle. This method should only be called once
// the program no longer needs to pass the handle to C and the C code
// no longer has a copy of the handle value.
//
// The method panics if the handle is invalid.
func (h Handle) Delete() {
	_, ok := handles.LoadAndDelete(uintptr(h))
	if !ok {
		panic("runtime/cgo: misuse of an invalid Handle")
	}
}

var (
	handles   = sync.Map{} // map[Handle]interface{}
	handleIdx atomic.Uintptr
)

```

// === FILE: references!/go/src/runtime/cgo/iscgo.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The runtime package contains an uninitialized definition
// for runtime·iscgo. Override it to tell the runtime we're here.
// There are various function pointers that should be set too,
// but those depend on dynamic linker magic to get initialized
// correctly, and sometimes they break. This variable is a
// backup: it depends only on old C style static linking rules.

package cgo

import _ "unsafe" // for go:linkname

//go:linkname _iscgo runtime.iscgo
var _iscgo bool = true

```

// === FILE: references!/go/src/runtime/cgo/libcgo.h ===
```text
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include <stdint.h>
#include <stdlib.h>
#include <stdio.h>

#undef nil
#define nil ((void*)0)
#define nelem(x) (sizeof(x)/sizeof((x)[0]))

typedef uint32_t uint32;
typedef uint64_t uint64;
typedef uintptr_t uintptr;

/*
 * The beginning of the per-goroutine structure,
 * as defined in ../pkg/runtime/runtime.h.
 * Just enough to edit these two fields.
 */
typedef struct G G;
struct G
{
	uintptr stacklo;
	uintptr stackhi;
};

/*
 * Arguments to the _cgo_thread_start call.
 * Also known to ../pkg/runtime/runtime.h.
 */
typedef struct ThreadStart ThreadStart;
struct ThreadStart
{
	G *g;
	uintptr *tls;
	void (*fn)(void);
};

/*
 * Called by 5c/6c/8c world.
 * Makes a local copy of the ThreadStart and
 * calls _cgo_sys_thread_start(ts).
 */
extern void (*_cgo_thread_start)(ThreadStart *ts);

/*
 * Creates a new operating system thread without updating any Go state
 * (OS dependent).
 */
extern void (*_cgo_sys_thread_create)(void* (*func)(void*));

/*
 * Indicates whether a dummy pthread per-thread variable is allocated.
 */
extern uintptr_t *_cgo_pthread_key_created;

/*
 * Creates the new operating system thread (OS, arch dependent).
 */
void _cgo_sys_thread_start(ThreadStart *ts);

/*
 * Waits for the Go runtime to be initialized (OS dependent).
 * If runtime.SetCgoTraceback is used to set a context function,
 * calls the context function and returns the context value.
 */
uintptr_t _cgo_wait_runtime_init_done(void);

/*
 * Get the low and high boundaries of the stack.
 */
void x_cgo_getstackbound(uintptr bounds[2]);

/*
 * Calls into the Go tool chain, where all registers are caller save.
 * Called from C, it saves all callee-save registers and calls
 * setg_gcc to set g before calling fn.
 */
void crosscall1(void (*fn)(void), void (*setg_gcc)(void*), void *g);

/*
 * Prints error then calls abort. For linux and android.
 */
void fatalf(const char* format, ...) __attribute__ ((noreturn));

/*
 * The cgo traceback callback. See runtime.SetCgoTraceback.
 */
struct cgoTracebackArg {
	uintptr_t  Context;
	uintptr_t  SigContext;
	uintptr_t* Buf;
	uintptr_t  Max;
};
extern void (*(_cgo_get_traceback_function(void)))(struct cgoTracebackArg*);

/*
 * The cgo context callback. See runtime.SetCgoTraceback.
 */
struct cgoContextArg {
	uintptr_t Context;
};
extern void (*(_cgo_get_context_function(void)))(struct cgoContextArg*);

/*
 * The argument for the cgo symbolizer callback. See runtime.SetCgoTraceback.
 */
struct cgoSymbolizerArg {
	uintptr_t   PC;
	const char* File;
	uintptr_t   Lineno;
	const char* Func;
	uintptr_t   Entry;
	uintptr_t   More;
	uintptr_t   Data;
};
extern void (*(_cgo_get_symbolizer_function(void)))(struct cgoSymbolizerArg*);

/*
 * The argument for x_cgo_set_traceback_functions. See runtime.SetCgoTraceback.
 */
struct cgoSetTracebackFunctionsArg {
	void (*Traceback)(struct cgoTracebackArg*);
	void (*Context)(struct cgoContextArg*);
	void (*Symbolizer)(struct cgoSymbolizerArg*);
};

/*
 * TSAN support.  This is only useful when building with
 *   CGO_CFLAGS="-fsanitize=thread" CGO_LDFLAGS="-fsanitize=thread" go install
 */
#undef CGO_TSAN
#if defined(__has_feature)
# if __has_feature(thread_sanitizer)
#  define CGO_TSAN
# endif
#elif defined(__SANITIZE_THREAD__)
# define CGO_TSAN
#endif

#ifdef CGO_TSAN

// _cgo_tsan_acquire tells C/C++ TSAN that we are acquiring a dummy lock. We
// call this when calling from Go to C. This is necessary because TSAN cannot
// see the synchronization in Go. Note that C/C++ code built with TSAN is not
// the same as the Go race detector.
//
// cmd/cgo generates calls to _cgo_tsan_acquire and _cgo_tsan_release. For
// other cgo calls, manual calls are required.
//
// These must match the definitions in yesTsanProlog in cmd/cgo/out.go.
// In general we should call _cgo_tsan_acquire when we enter C code,
// and call _cgo_tsan_release when we return to Go code.
//
// This is only necessary when calling code that might be instrumented
// by TSAN, which mostly means system library calls that TSAN intercepts.
//
// See the comment in cmd/cgo/out.go for more details.

long long _cgo_sync __attribute__ ((common));

extern void __tsan_acquire(void*);
extern void __tsan_release(void*);

__attribute__ ((unused))
static void _cgo_tsan_acquire() {
	__tsan_acquire(&_cgo_sync);
}

__attribute__ ((unused))
static void _cgo_tsan_release() {
	__tsan_release(&_cgo_sync);
}

#else // !defined(CGO_TSAN)

#define _cgo_tsan_acquire()
#define _cgo_tsan_release()

#endif // !defined(CGO_TSAN)

```

// === FILE: references!/go/src/runtime/cgo/libcgo_unix.h ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include <pthread.h>

/*
 * Call pthread_create, retrying on EAGAIN.
 */
extern int _cgo_try_pthread_create(pthread_t*, const pthread_attr_t*, void* (*)(void*), void*);

extern void* threadentry(void*);

```

// === FILE: references!/go/src/runtime/cgo/linux.go ===
```go
// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Linux system call wrappers that provide POSIX semantics through the
// corresponding cgo->libc (nptl) wrappers for various system calls.

//go:build linux

package cgo

import "unsafe"

// Each of the following entries is needed to ensure that the
// syscall.syscall_linux code can conditionally call these
// function pointers:
//
//  1. find the C-defined function start
//  2. force the local byte alias to be mapped to that location
//  3. map the Go pointer to the function to the syscall package

//go:cgo_import_static _cgo_libc_setegid
//go:linkname _cgo_libc_setegid _cgo_libc_setegid
//go:linkname cgo_libc_setegid syscall.cgo_libc_setegid
var _cgo_libc_setegid byte
var cgo_libc_setegid = unsafe.Pointer(&_cgo_libc_setegid)

//go:cgo_import_static _cgo_libc_seteuid
//go:linkname _cgo_libc_seteuid _cgo_libc_seteuid
//go:linkname cgo_libc_seteuid syscall.cgo_libc_seteuid
var _cgo_libc_seteuid byte
var cgo_libc_seteuid = unsafe.Pointer(&_cgo_libc_seteuid)

//go:cgo_import_static _cgo_libc_setregid
//go:linkname _cgo_libc_setregid _cgo_libc_setregid
//go:linkname cgo_libc_setregid syscall.cgo_libc_setregid
var _cgo_libc_setregid byte
var cgo_libc_setregid = unsafe.Pointer(&_cgo_libc_setregid)

//go:cgo_import_static _cgo_libc_setresgid
//go:linkname _cgo_libc_setresgid _cgo_libc_setresgid
//go:linkname cgo_libc_setresgid syscall.cgo_libc_setresgid
var _cgo_libc_setresgid byte
var cgo_libc_setresgid = unsafe.Pointer(&_cgo_libc_setresgid)

//go:cgo_import_static _cgo_libc_setresuid
//go:linkname _cgo_libc_setresuid _cgo_libc_setresuid
//go:linkname cgo_libc_setresuid syscall.cgo_libc_setresuid
var _cgo_libc_setresuid byte
var cgo_libc_setresuid = unsafe.Pointer(&_cgo_libc_setresuid)

//go:cgo_import_static _cgo_libc_setreuid
//go:linkname _cgo_libc_setreuid _cgo_libc_setreuid
//go:linkname cgo_libc_setreuid syscall.cgo_libc_setreuid
var _cgo_libc_setreuid byte
var cgo_libc_setreuid = unsafe.Pointer(&_cgo_libc_setreuid)

//go:cgo_import_static _cgo_libc_setgroups
//go:linkname _cgo_libc_setgroups _cgo_libc_setgroups
//go:linkname cgo_libc_setgroups syscall.cgo_libc_setgroups
var _cgo_libc_setgroups byte
var cgo_libc_setgroups = unsafe.Pointer(&_cgo_libc_setgroups)

//go:cgo_import_static _cgo_libc_setgid
//go:linkname _cgo_libc_setgid _cgo_libc_setgid
//go:linkname cgo_libc_setgid syscall.cgo_libc_setgid
var _cgo_libc_setgid byte
var cgo_libc_setgid = unsafe.Pointer(&_cgo_libc_setgid)

//go:cgo_import_static _cgo_libc_setuid
//go:linkname _cgo_libc_setuid _cgo_libc_setuid
//go:linkname cgo_libc_setuid syscall.cgo_libc_setuid
var _cgo_libc_setuid byte
var cgo_libc_setuid = unsafe.Pointer(&_cgo_libc_setuid)

```

// === FILE: references!/go/src/runtime/cgo/linux_syscall.c ===
```text
// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux

#ifndef _GNU_SOURCE // setres[ug]id() API.
#define _GNU_SOURCE
#endif

#include <grp.h>
#include <sys/types.h>
#include <unistd.h>
#include <errno.h>
#include "libcgo.h"

/*
 * Assumed POSIX compliant libc system call wrappers. For linux, the
 * glibc/nptl/setxid mechanism ensures that POSIX semantics are
 * honored for all pthreads (by default), and this in turn with cgo
 * ensures that all Go threads launched with cgo are kept in sync for
 * these function calls.
 */

// argset_t matches runtime/cgocall.go:argset.
typedef struct {
	uintptr_t* args;
	uintptr_t retval;
} argset_t;

// libc backed posix-compliant syscalls.

#define SET_RETVAL(fn) \
  uintptr_t ret = (uintptr_t) fn ; \
  if (ret == (uintptr_t) -1) {	   \
    x->retval = (uintptr_t) errno; \
  } else                           \
    x->retval = ret

void
_cgo_libc_setegid(argset_t* x) {
	SET_RETVAL(setegid((gid_t) x->args[0]));
}

void
_cgo_libc_seteuid(argset_t* x) {
	SET_RETVAL(seteuid((uid_t) x->args[0]));
}

void
_cgo_libc_setgid(argset_t* x) {
	SET_RETVAL(setgid((gid_t) x->args[0]));
}

void
_cgo_libc_setgroups(argset_t* x) {
	SET_RETVAL(setgroups((size_t) x->args[0], (const gid_t *) x->args[1]));
}

void
_cgo_libc_setregid(argset_t* x) {
	SET_RETVAL(setregid((gid_t) x->args[0], (gid_t) x->args[1]));
}

void
_cgo_libc_setresgid(argset_t* x) {
	SET_RETVAL(setresgid((gid_t) x->args[0], (gid_t) x->args[1],
			     (gid_t) x->args[2]));
}

void
_cgo_libc_setresuid(argset_t* x) {
	SET_RETVAL(setresuid((uid_t) x->args[0], (uid_t) x->args[1],
			     (uid_t) x->args[2]));
}

void
_cgo_libc_setreuid(argset_t* x) {
	SET_RETVAL(setreuid((uid_t) x->args[0], (uid_t) x->args[1]));
}

void
_cgo_libc_setuid(argset_t* x) {
	SET_RETVAL(setuid((uid_t) x->args[0]));
}

```

// === FILE: references!/go/src/runtime/cgo/mmap.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build (linux && (amd64 || arm64 || loong64)) || (freebsd && amd64)

package cgo

// Import "unsafe" because we use go:linkname.
import _ "unsafe"

// When using cgo, call the C library for mmap, so that we call into
// any sanitizer interceptors. This supports using the memory
// sanitizer with Go programs. The memory sanitizer only applies to
// C/C++ code; this permits that code to see the Go code as normal
// program addresses that have been initialized.

// To support interceptors that look for both mmap and munmap,
// also call the C library for munmap.

//go:cgo_import_static x_cgo_mmap
//go:linkname x_cgo_mmap x_cgo_mmap
//go:linkname _cgo_mmap _cgo_mmap
var x_cgo_mmap byte
var _cgo_mmap = &x_cgo_mmap

//go:cgo_import_static x_cgo_munmap
//go:linkname x_cgo_munmap x_cgo_munmap
//go:linkname _cgo_munmap _cgo_munmap
var x_cgo_munmap byte
var _cgo_munmap = &x_cgo_munmap

```

// === FILE: references!/go/src/runtime/cgo/netbsd.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build netbsd

package cgo

import _ "unsafe" // for go:linkname

// Supply environ and __progname, because we don't
// link against the standard NetBSD crt0.o and the
// libc dynamic library needs them.

//go:linkname _environ environ
//go:linkname _progname __progname
//go:linkname ___ps_strings __ps_strings

var _environ uintptr
var _progname uintptr
var ___ps_strings uintptr

```

// === FILE: references!/go/src/runtime/cgo/openbsd.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build openbsd

package cgo

import _ "unsafe" // for go:linkname

// Supply __guard_local because we don't link against the standard
// OpenBSD crt0.o and the libc dynamic library needs it.

//go:linkname _guard_local __guard_local

var _guard_local uintptr

// This is normally marked as hidden and placed in the
// .openbsd.randomdata section.
//
//go:cgo_export_dynamic __guard_local __guard_local

```

// === FILE: references!/go/src/runtime/cgo/pthread_unix.c ===
```text
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix

#ifndef _GNU_SOURCE // pthread_getattr_np
#define _GNU_SOURCE
#endif

#include <pthread.h>
#include <string.h>
#include <signal.h>
#include <errno.h>
#include "libcgo.h"
#include "libcgo_unix.h"

void
_cgo_sys_thread_start(ThreadStart *ts)
{
	pthread_attr_t attr;
	sigset_t ign, oset;
	pthread_t p;
	size_t size;
	int err;

	sigfillset(&ign);
	pthread_sigmask(SIG_SETMASK, &ign, &oset);

	pthread_attr_init(&attr);
	pthread_attr_setdetachstate(&attr, PTHREAD_CREATE_DETACHED);
#if defined(__APPLE__)
	// Copy stack size from parent thread instead of using the
	// non-main thread default stack size.
	size = pthread_get_stacksize_np(pthread_self());
	pthread_attr_setstacksize(&attr, size);
#else
	pthread_attr_getstacksize(&attr, &size);
#endif

#if defined(__sun)
	// Solaris can report 0 stack size, fix it.
	if (size == 0) {
		size = 2 << 20;
		if (pthread_attr_setstacksize(&attr, size) != 0) {
			perror("runtime/cgo: pthread_attr_setstacksize failed");
		}
	}
#endif

	// Leave stacklo=0 and set stackhi=size; mstart will do the rest.
	ts->g->stackhi = size;
	err = _cgo_try_pthread_create(&p, &attr, threadentry, ts);

	pthread_sigmask(SIG_SETMASK, &oset, nil);

	if (err != 0) {
		fatalf("pthread_create failed: %s", strerror(err));
	}
}

void
x_cgo_sys_thread_create(void* (*func)(void*)) {
	pthread_attr_t attr;
	pthread_t p;
	int err;

	pthread_attr_init(&attr);
	pthread_attr_setdetachstate(&attr, PTHREAD_CREATE_DETACHED);
	err = _cgo_try_pthread_create(&p, &attr, func, NULL);
	if (err != 0) {
		fatalf("pthread_create failed: %s", strerror(err));
	}
}

void (* _cgo_sys_thread_create)(void* (*func)(void*)) = x_cgo_sys_thread_create;

void
x_cgo_getstackbound(uintptr bounds[2])
{
	pthread_attr_t attr;
	void *addr;
	size_t size;

	// Needed before pthread_getattr_np, too, since before glibc 2.32
	// it did not call pthread_attr_init in all cases (see #65625).
	pthread_attr_init(&attr);
#if defined(__APPLE__)
	// On macOS/iOS, use the non-portable pthread_get_stackaddr_np
	// and pthread_get_stacksize_np APIs (high address + size).
	addr = pthread_get_stackaddr_np(pthread_self());
	size = pthread_get_stacksize_np(pthread_self());
	addr = (void*)((uintptr)addr - size); // convert to low address
#elif defined(__GLIBC__) || defined(__BIONIC__) || (defined(__sun) && !defined(__illumos__))
	// pthread_getattr_np is a GNU extension supported in glibc.
	// Solaris is not glibc but does support pthread_getattr_np
	// (and the fallback doesn't work...). Illumos does not.
	pthread_getattr_np(pthread_self(), &attr);  // GNU extension
	pthread_attr_getstack(&attr, &addr, &size); // low address
#elif defined(__illumos__)
	pthread_attr_get_np(pthread_self(), &attr);
	pthread_attr_getstack(&attr, &addr, &size); // low address
#else
	// We don't know how to get the current stacks, leave it as
	// 0 and the caller will use an estimate based on the current
	// SP.
	addr = 0;
	size = 0;
#endif
	pthread_attr_destroy(&attr);

	// bounds points into the Go stack. TSAN can't see the synchronization
	// in Go around stack reuse.
	_cgo_tsan_acquire();
	bounds[0] = (uintptr)addr;
	bounds[1] = (uintptr)addr + size;
	_cgo_tsan_release();
}

void (* _cgo_getstackbound)(uintptr[2]) = x_cgo_getstackbound;

// _cgo_try_pthread_create retries pthread_create if it fails with EAGAIN.
int
_cgo_try_pthread_create(pthread_t* thread, const pthread_attr_t* attr, void* (*pfn)(void*), void* arg) {
	int tries;
	int err;
	struct timespec ts;

	for (tries = 0; tries < 20; tries++) {
		err = pthread_create(thread, attr, pfn, arg);
		if (err == 0) {
			return 0;
		}
		if (err != EAGAIN) {
			return err;
		}
		ts.tv_sec = 0;
		ts.tv_nsec = (tries + 1) * 1000 * 1000; // Milliseconds.
		nanosleep(&ts, nil);
	}
	return EAGAIN;
}

```

// === FILE: references!/go/src/runtime/cgo/setenv.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix

package cgo

import _ "unsafe" // for go:linkname

//go:cgo_import_static x_cgo_setenv
//go:linkname x_cgo_setenv x_cgo_setenv
//go:linkname _cgo_setenv runtime._cgo_setenv
var x_cgo_setenv byte
var _cgo_setenv = &x_cgo_setenv

//go:cgo_import_static x_cgo_unsetenv
//go:linkname x_cgo_unsetenv x_cgo_unsetenv
//go:linkname _cgo_unsetenv runtime._cgo_unsetenv
var x_cgo_unsetenv byte
var _cgo_unsetenv = &x_cgo_unsetenv

```

// === FILE: references!/go/src/runtime/cgo/sigaction.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build (linux && (386 || amd64 || arm64 || loong64 || ppc64 || ppc64le)) || (freebsd && amd64)

package cgo

// Import "unsafe" because we use go:linkname.
import _ "unsafe"

// When using cgo, call the C library for sigaction, so that we call into
// any sanitizer interceptors. This supports using the sanitizers
// with Go programs. The thread and memory sanitizers only apply to
// C/C++ code; this permits that code to see the Go runtime's existing signal
// handlers when registering new signal handlers for the process.

//go:cgo_import_static x_cgo_sigaction
//go:linkname x_cgo_sigaction x_cgo_sigaction
//go:linkname _cgo_sigaction _cgo_sigaction
var x_cgo_sigaction byte
var _cgo_sigaction = &x_cgo_sigaction

```

// === FILE: references!/go/src/runtime/cgo/windows.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows

package cgo

import _ "unsafe" // for go:linkname

// _cgo_stub_export is only used to ensure there's at least one symbol
// in the .def file passed to the external linker.
// If there are no exported symbols, the unfortunate behavior of
// the binutils linker is to also strip the relocations table,
// resulting in non-PIE binary. The other option is the
// --export-all-symbols flag, but we don't need to export all symbols
// and this may overflow the export table (#40795).
// See https://sourceware.org/bugzilla/show_bug.cgi?id=19011
//
//go:cgo_export_static _cgo_stub_export
//go:linkname _cgo_stub_export _cgo_stub_export
var _cgo_stub_export uintptr

```

