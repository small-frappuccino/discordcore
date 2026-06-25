# Domain Architecture: runtime/secret

## Layout Topology
```text
runtime/secret/
├── asm_amd64.s
├── asm_arm64.s
├── doc.go
├── export.go
├── secret.go
├── stubs.go
└── stubs_noasm.go
```

## Source Stream Aggregation

// === FILE: references!/go/src/runtime/secret/asm_amd64.s ===
```text
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.runtimesecret

// Note: this assembly file is used for testing only.
// We need to access registers directly to properly test
// that secrets are erased and go test doesn't like to conditionally
// include assembly files.
// These functions defined in the package proper and we
// rely on the linker to prune these away in regular builds

#include "go_asm.h"
#include "funcdata.h"

TEXT ·loadRegisters(SB),0,$0-8
	MOVQ	p+0(FP), AX

	MOVQ	(AX), R10
	MOVQ	(AX), R11
	MOVQ	(AX), R12
	MOVQ	(AX), R13

	MOVOU	(AX), X1
	MOVOU	(AX), X2
	MOVOU	(AX), X3
	MOVOU	(AX), X4

	CMPB	internal∕cpu·X86+const_offsetX86HasAVX(SB), $1
	JNE	return

	VMOVDQU	(AX), Y5
	VMOVDQU	(AX), Y6
	VMOVDQU	(AX), Y7
	VMOVDQU	(AX), Y8

	CMPB	internal∕cpu·X86+const_offsetX86HasAVX512(SB), $1
	JNE	return

	VMOVUPD	(AX), Z14
	VMOVUPD	(AX), Z15
	VMOVUPD	(AX), Z16
	VMOVUPD	(AX), Z17

	KMOVQ	(AX), K2
	KMOVQ	(AX), K3
	KMOVQ	(AX), K4
	KMOVQ	(AX), K5

return:
	RET

TEXT ·spillRegisters(SB),0,$0-16
	MOVQ	p+0(FP), AX
	MOVQ	AX, BX

	MOVQ	R10, (AX)
	MOVQ	R11, 8(AX)
	MOVQ	R12, 16(AX)
	MOVQ	R13, 24(AX)
	ADDQ	$32, AX

	MOVOU	X1, (AX)
	MOVOU	X2, 16(AX)
	MOVOU	X3, 32(AX)
	MOVOU	X4, 48(AX)
	ADDQ	$64, AX

	CMPB	internal∕cpu·X86+const_offsetX86HasAVX(SB), $1
	JNE	return

	VMOVDQU	Y5, (AX)
	VMOVDQU	Y6, 32(AX)
	VMOVDQU	Y7, 64(AX)
	VMOVDQU	Y8, 96(AX)
	ADDQ	$128, AX

	CMPB	internal∕cpu·X86+const_offsetX86HasAVX512(SB), $1
	JNE	return

	VMOVUPD	Z14, (AX)
	ADDQ	$64, AX
	VMOVUPD	Z15, (AX)
	ADDQ	$64, AX
	VMOVUPD	Z16, (AX)
	ADDQ	$64, AX
	VMOVUPD	Z17, (AX)
	ADDQ	$64, AX

	KMOVQ	K2, (AX)
	ADDQ	$8, AX
	KMOVQ	K3, (AX)
	ADDQ	$8, AX
	KMOVQ	K4, (AX)
	ADDQ	$8, AX
	KMOVQ	K5, (AX)
	ADDQ	$8, AX

return:
	SUBQ	BX, AX
	MOVQ	AX, ret+8(FP)
	RET

TEXT ·useSecret(SB),0,$64-24
	NO_LOCAL_POINTERS

	// Load secret into AX
	MOVQ	secret_base+0(FP), AX
	MOVQ	(AX), AX

	// Scatter secret all across registers.
	// Increment low byte so we can tell which register
	// a leaking secret came from.
	ADDQ	$2, AX // add 2 so Rn has secret #n.
	MOVQ	AX, BX
	INCQ	AX
	MOVQ	AX, CX
	INCQ	AX
	MOVQ	AX, DX
	INCQ	AX
	MOVQ	AX, SI
	INCQ	AX
	MOVQ	AX, DI
	INCQ	AX
	MOVQ	AX, BP
	INCQ	AX
	MOVQ	AX, R8
	INCQ	AX
	MOVQ	AX, R9
	INCQ	AX
	MOVQ	AX, R10
	INCQ	AX
	MOVQ	AX, R11
	INCQ	AX
	MOVQ	AX, R12
	INCQ	AX
	MOVQ	AX, R13
	INCQ	AX
	MOVQ	AX, R14
	INCQ	AX
	MOVQ	AX, R15

	CMPB	internal∕cpu·X86+const_offsetX86HasAVX512(SB), $1
	JNE	noavx512
	VMOVUPD	(SP), Z0
	VMOVUPD	(SP), Z1
	VMOVUPD	(SP), Z2
	VMOVUPD	(SP), Z3
	VMOVUPD	(SP), Z4
	VMOVUPD	(SP), Z5
	VMOVUPD	(SP), Z6
	VMOVUPD	(SP), Z7
	VMOVUPD	(SP), Z8
	VMOVUPD	(SP), Z9
	VMOVUPD	(SP), Z10
	VMOVUPD	(SP), Z11
	VMOVUPD	(SP), Z12
	VMOVUPD	(SP), Z13
	VMOVUPD	(SP), Z14
	VMOVUPD	(SP), Z15
	VMOVUPD	(SP), Z16
	VMOVUPD	(SP), Z17
	VMOVUPD	(SP), Z18
	VMOVUPD	(SP), Z19
	VMOVUPD	(SP), Z20
	VMOVUPD	(SP), Z21
	VMOVUPD	(SP), Z22
	VMOVUPD	(SP), Z23
	VMOVUPD	(SP), Z24
	VMOVUPD	(SP), Z25
	VMOVUPD	(SP), Z26
	VMOVUPD	(SP), Z27
	VMOVUPD	(SP), Z28
	VMOVUPD	(SP), Z29
	VMOVUPD	(SP), Z30
	VMOVUPD	(SP), Z31

noavx512:
	MOVOU	(SP), X0
	MOVOU	(SP), X1
	MOVOU	(SP), X2
	MOVOU	(SP), X3
	MOVOU	(SP), X4
	MOVOU	(SP), X5
	MOVOU	(SP), X6
	MOVOU	(SP), X7
	MOVOU	(SP), X8
	MOVOU	(SP), X9
	MOVOU	(SP), X10
	MOVOU	(SP), X11
	MOVOU	(SP), X12
	MOVOU	(SP), X13
	MOVOU	(SP), X14
	MOVOU	(SP), X15

	// Put secret on the stack.
	INCQ	AX
	MOVQ	AX, (SP)
	MOVQ	AX, 8(SP)
	MOVQ	AX, 16(SP)
	MOVQ	AX, 24(SP)
	MOVQ	AX, 32(SP)
	MOVQ	AX, 40(SP)
	MOVQ	AX, 48(SP)
	MOVQ	AX, 56(SP)

	// Delay a bit.  This makes it more likely that
	// we will be the target of a signal while
	// registers contain secrets.
	// It also tests the path from G stack to M stack
	// to scheduler and back.
	CALL	runtime∕secret·delay(SB)

	RET

```

// === FILE: references!/go/src/runtime/secret/asm_arm64.s ===
```text
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.runtimesecret

// Note: this assembly file is used for testing only.
// We need to access registers directly to properly test
// that secrets are erased and go test doesn't like to conditionally
// include assembly files.
// These functions defined in the package proper and we
// rely on the linker to prune these away in regular builds

#include "go_asm.h"
#include "funcdata.h"

TEXT ·loadRegisters(SB),0,$0-8
	MOVD	p+0(FP), R0

	MOVD	(R0), R10
	MOVD	(R0), R11
	MOVD	(R0), R12
	MOVD	(R0), R13

	FMOVD	(R0), F15
	FMOVD	(R0), F16
	FMOVD	(R0), F17
	FMOVD	(R0), F18

	VLD1	(R0), [V20.B16]
	VLD1	(R0), [V21.H8]
	VLD1	(R0), [V22.S4]
	VLD1	(R0), [V23.D2]

	RET

TEXT ·spillRegisters(SB),0,$0-16
	MOVD	p+0(FP), R0
	MOVD	R0, R1

	MOVD	R10, (R0)
	MOVD	R11, 8(R0)
	MOVD	R12, 16(R0)
	MOVD	R13, 24(R0)
	ADD	$32, R0

	FMOVD	F15, (R0)
	FMOVD	F16, 16(R0)
	FMOVD	F17, 32(R0)
	FMOVD	F18, 64(R0)
	ADD	$64, R0

	VST1.P	[V20.B16], (R0)
	VST1.P	[V21.H8], (R0)
	VST1.P	[V22.S4], (R0)
	VST1.P	[V23.D2], (R0)

	SUB	R1, R0, R0
	MOVD	R0, ret+8(FP)
	RET

TEXT ·useSecret(SB),0,$0-24
	NO_LOCAL_POINTERS

	// Load secret into R0
	MOVD	secret_base+0(FP), R0
	MOVD	(R0), R0
	// Scatter secret across registers.
	// Increment low byte so we can tell which register
	// a leaking secret came from.

	// TODO(dmo): more substantial dirtying here
	ADD	$1, R0
	MOVD	R0, R1
	ADD	$1, R0
	MOVD	R0, R2
	ADD	$1, R0
	MOVD	R0, R3
	ADD	$1, R0
	MOVD	R0, R4
	ADD	$1, R0
	MOVD	R0, R5
	ADD	$1, R0
	MOVD	R0, R6
	ADD	$1, R0
	MOVD	R0, R7
	ADD	$1, R0
	MOVD	R0, R8
	ADD	$1, R0
	MOVD	R0, R9
	ADD	$1, R0
	MOVD	R0, R10
	ADD	$1, R0
	MOVD	R0, R11
	ADD	$1, R0
	MOVD	R0, R12
	ADD	$1, R0
	MOVD	R0, R13
	ADD	$1, R0
	MOVD	R0, R14
	ADD	$1, R0
	MOVD	R0, R15

	// Dirty the floating point registers
	ADD     $1, R0
	FMOVD   R0, F0
	ADD     $1, R0
	FMOVD   R0, F1
	ADD     $1, R0
	FMOVD   R0, F2
	ADD     $1, R0
	FMOVD   R0, F3
	ADD     $1, R0
	FMOVD   R0, F4
	ADD     $1, R0
	FMOVD   R0, F5
	ADD     $1, R0
	FMOVD   R0, F6
	ADD     $1, R0
	FMOVD   R0, F7
	ADD     $1, R0
	FMOVD   R0, F8
	ADD     $1, R0
	FMOVD   R0, F9
	ADD     $1, R0
	FMOVD   R0, F10
	ADD     $1, R0
	FMOVD   R0, F11
	ADD     $1, R0
	FMOVD   R0, F12
	ADD     $1, R0
	FMOVD   R0, F13
	ADD     $1, R0
	FMOVD   R0, F14
	ADD     $1, R0
	FMOVD   R0, F15
	ADD     $1, R0
	FMOVD   R0, F16
	ADD     $1, R0
	FMOVD   R0, F17
	ADD     $1, R0
	FMOVD   R0, F18
	ADD     $1, R0
	FMOVD   R0, F19
	ADD     $1, R0
	FMOVD   R0, F20
	ADD     $1, R0
	FMOVD   R0, F21
	ADD     $1, R0
	FMOVD   R0, F22
	ADD     $1, R0
	FMOVD   R0, F23
	ADD     $1, R0
	FMOVD   R0, F24
	ADD     $1, R0
	FMOVD   R0, F25
	ADD     $1, R0
	FMOVD   R0, F26
	ADD     $1, R0
	FMOVD   R0, F27
	ADD     $1, R0
	FMOVD   R0, F28
	ADD     $1, R0
	FMOVD   R0, F29
	ADD     $1, R0
	FMOVD   R0, F30
	ADD     $1, R0
	FMOVD   R0, F31
	RET

```

// === FILE: references!/go/src/runtime/secret/doc.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.runtimesecret

// Package secret contains helper functions for zeroing out memory
// that is otherwise invisible to a user program in the service of
// forward secrecy. See https://en.wikipedia.org/wiki/Forward_secrecy for
// more information.
//
// This package (runtime/secret) is experimental,
// and not subject to the Go 1 compatibility promise.
// It only exists when building with the GOEXPERIMENT=runtimesecret environment variable set.
package secret

```

// === FILE: references!/go/src/runtime/secret/export.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.runtimesecret

package secret

import (
	"internal/cpu"
	"unsafe"
)

// exports for assembly testing functions
const (
	offsetX86HasAVX    = unsafe.Offsetof(cpu.X86.HasAVX)
	offsetX86HasAVX512 = unsafe.Offsetof(cpu.X86.HasAVX512)
)

```

// === FILE: references!/go/src/runtime/secret/secret.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.runtimesecret

package secret

import (
	"runtime"
	_ "unsafe"
)

// Do invokes f.
//
// Do ensures that any temporary storage used by f is erased in a
// timely manner. (In this context, "f" is shorthand for the
// entire call tree initiated by f.)
//   - Any registers used by f are erased before Do returns.
//   - Any stack used by f is erased before Do returns.
//   - Heap allocations done by f are erased as soon as the garbage
//     collector realizes that all allocated values are no longer reachable.
//   - Do works even if f panics or calls runtime.Goexit.  As part of
//     that, any panic raised by f will appear as if it originates from
//     Do itself.
//
// Any goroutine spawned while executing f will act as if the entire goroutine
// is wrapped inside another call to Do.
//
// Users should be cautious of allocating inside Do.
// Erasing heap memory after Do returns may increase garbage collector sweep times and
// requires additional memory to keep track of allocations until they are to be erased.
// These costs can compound when an allocation is done in the service of growing a value,
// like appending to a slice or inserting into a map. In these cases, the entire new allocation is erased rather
// than just the secret parts of it.
//
// To reduce lifetimes of allocations and avoid unexpected performance issues,
// if a function invoked by Do needs to yield a result that shouldn't be erased,
// it should do so by copying the result into an allocation created by the caller.
//
// Limitations:
//   - Currently only supported on linux/amd64 and linux/arm64.  On unsupported
//     platforms, Do will invoke f directly.
//   - Protection does not extend to any global variables written by f.
//   - If f calls runtime.Goexit, erasure can be delayed by defers
//     higher up on the call stack.
//   - Heap allocations will only be erased if the program drops all
//     references to those allocations, and then the garbage collector
//     notices that those references are gone. The former is under
//     control of the program, but the latter is at the whim of the
//     runtime.
//   - Any value panicked by f may point to allocations from within
//     f. Those allocations will not be erased until (at least) the
//     panicked value is dead.
//   - Pointer addresses may leak into data buffers used by the runtime
//     to perform garbage collection. Users should not encode confidential
//     information into pointers. For example, if an offset into an array or
//     struct is confidential, then users should not create a pointer into
//     the object. Since this function is intended to be used with constant-time
//     cryptographic code, this requirement is usually fulfilled implicitly.
func Do(f func()) {
	const osArch = runtime.GOOS + "/" + runtime.GOARCH
	switch osArch {
	default:
		// unsupported, just invoke f directly.
		f()
		return
	case "linux/amd64", "linux/arm64":
	}

	// Place to store any panic value.
	var p any

	// Step 1: increment the nesting count.
	inc()

	// Step 2: call helper. The helper just calls f
	// and captures (recovers) any panic result.
	p = doHelper(f)

	// Step 3: erase everything used by f (stack, registers).
	eraseSecrets()

	// Step 4: decrement the nesting count.
	dec()

	// Step 5: re-raise any caught panic.
	// This will make the panic appear to come
	// from a stack whose bottom frame is
	// runtime/secret.Do.
	// Anything below that to do with f will be gone.
	//
	// Note that the panic value is not erased. It behaves
	// like any other value that escapes from f. If it is
	// heap allocated, it will be erased when the garbage
	// collector notices it is no longer referenced.
	if p != nil {
		panic(p)
	}

	// Note: if f calls runtime.Goexit, step 3 and above will not
	// happen, as Goexit is unrecoverable. We handle that case in
	// runtime/proc.go:goexit0.
}

func doHelper(f func()) (p any) {
	// Step 2b: Pop the stack up to the secret.doHelper frame
	// if we are in the process of panicking.
	// (It is a no-op if we are not panicking.)
	// We return any panicked value to secret.Do, who will
	// re-panic it.
	defer func() {
		// Note: we rely on the go1.21+ behavior that
		// if we are panicking, recover returns non-nil.
		p = recover()
	}()

	// Step 2a: call the secret function.
	f()

	return
}

// Enabled reports whether the current goroutine
// is running in secret mode. This is usually through a call to
// [Do], but can also occur when a goroutine already running in
// secret mode launches another goroutine.
func Enabled() bool {
	return count() > 0
}

// implemented in runtime

//go:linkname count
func count() int32

//go:linkname inc
func inc()

//go:linkname dec
func dec()

//go:linkname eraseSecrets
func eraseSecrets()

```

// === FILE: references!/go/src/runtime/secret/stubs.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.runtimesecret && (arm64 || amd64)

// testing stubs, these are implemented in assembly in
// asm_$GOARCH.s
//
// Note that this file is also used as a template to build a
// crashing binary that tries to leave secrets in places where
// they are supposed to be erased. see crash_test.go for more info

package secret

import "unsafe"

// Load data from p into test registers.
//
//go:noescape
func loadRegisters(p unsafe.Pointer)

// Spill data from test registers into p.
// Returns the amount of space filled in.
//
//go:noescape
func spillRegisters(p unsafe.Pointer) uintptr

// Load secret into all registers.
//
//go:noescape
func useSecret(secret []byte)

// callback from assembly
func delay() {
	sleep(1_000_000)
}

// linknamed to avoid package importing time
// for just testing code
//
//go:linkname sleep time.Sleep
func sleep(int64)

```

// === FILE: references!/go/src/runtime/secret/stubs_noasm.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.runtimesecret && !arm64 && !amd64

package secret

import "unsafe"

func loadRegisters(p unsafe.Pointer)          {}
func spillRegisters(p unsafe.Pointer) uintptr { return 0 }
func useSecret(secret []byte)                 {}

```

