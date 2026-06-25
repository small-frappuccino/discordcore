# Domain Architecture: reflect

## Layout Topology
```text
reflect/
├── internal
│   ├── example1
│   │   └── example.go
│   └── example2
│       └── example.go
├── abi.go
├── arena.go
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
├── badlinkname.go
├── deepequal.go
├── float32reg_generic.go
├── float32reg_ppc64x.s
├── float32reg_riscv64.s
├── float32reg_s390x.s
├── iter.go
├── makefunc.go
├── map.go
├── stubs_ppc64x.go
├── stubs_riscv64.go
├── stubs_s390x.go
├── swapper.go
├── type.go
├── value.go
└── visiblefields.go
```

## Source Stream Aggregation

// === FILE: references!/go/src/reflect/abi.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reflect

import (
	"internal/abi"
	"internal/goarch"
	"unsafe"
)

// These variables are used by the register assignment
// algorithm in this file.
//
// They should be modified with care (no other reflect code
// may be executing) and are generally only modified
// when testing this package.
//
// They should never be set higher than their internal/abi
// constant counterparts, because the system relies on a
// structure that is at least large enough to hold the
// registers the system supports.
//
// Currently they're set to zero because using the actual
// constants will break every part of the toolchain that
// uses reflect to call functions (e.g. go test, or anything
// that uses text/template). The values that are currently
// commented out there should be the actual values once
// we're ready to use the register ABI everywhere.
var (
	intArgRegs   = abi.IntArgRegs
	floatArgRegs = abi.FloatArgRegs
	floatRegSize = uintptr(abi.EffectiveFloatRegSize)
)

// abiStep represents an ABI "instruction." Each instruction
// describes one part of how to translate between a Go value
// in memory and a call frame.
type abiStep struct {
	kind abiStepKind

	// offset and size together describe a part of a Go value
	// in memory.
	offset uintptr
	size   uintptr // size in bytes of the part

	// These fields describe the ABI side of the translation.
	stkOff uintptr // stack offset, used if kind == abiStepStack
	ireg   int     // integer register index, used if kind == abiStepIntReg or kind == abiStepPointer
	freg   int     // FP register index, used if kind == abiStepFloatReg
}

// abiStepKind is the "op-code" for an abiStep instruction.
type abiStepKind int

const (
	abiStepBad      abiStepKind = iota
	abiStepStack                // copy to/from stack
	abiStepIntReg               // copy to/from integer register
	abiStepPointer              // copy pointer to/from integer register
	abiStepFloatReg             // copy to/from FP register
)

// abiSeq represents a sequence of ABI instructions for copying
// from a series of reflect.Values to a call frame (for call arguments)
// or vice-versa (for call results).
//
// An abiSeq should be populated by calling its addArg method.
type abiSeq struct {
	// steps is the set of instructions.
	//
	// The instructions are grouped together by whole arguments,
	// with the starting index for the instructions
	// of the i'th Go value available in valueStart.
	//
	// For instance, if this abiSeq represents 3 arguments
	// passed to a function, then the 2nd argument's steps
	// begin at steps[valueStart[1]].
	//
	// Because reflect accepts Go arguments in distinct
	// Values and each Value is stored separately, each abiStep
	// that begins a new argument will have its offset
	// field == 0.
	steps      []abiStep
	valueStart []int

	stackBytes   uintptr // stack space used
	iregs, fregs int     // registers used
}

func (a *abiSeq) dump() {
	for i, p := range a.steps {
		println("part", i, p.kind, p.offset, p.size, p.stkOff, p.ireg, p.freg)
	}
	print("values ")
	for _, i := range a.valueStart {
		print(i, " ")
	}
	println()
	println("stack", a.stackBytes)
	println("iregs", a.iregs)
	println("fregs", a.fregs)
}

// stepsForValue returns the ABI instructions for translating
// the i'th Go argument or return value represented by this
// abiSeq to the Go ABI.
func (a *abiSeq) stepsForValue(i int) []abiStep {
	s := a.valueStart[i]
	var e int
	if i == len(a.valueStart)-1 {
		e = len(a.steps)
	} else {
		e = a.valueStart[i+1]
	}
	return a.steps[s:e]
}

// addArg extends the abiSeq with a new Go value of type t.
//
// If the value was stack-assigned, returns the single
// abiStep describing that translation, and nil otherwise.
func (a *abiSeq) addArg(t *abi.Type) *abiStep {
	// We'll always be adding a new value, so do that first.
	pStart := len(a.steps)
	a.valueStart = append(a.valueStart, pStart)
	if t.Size() == 0 {
		// If the size of the argument type is zero, then
		// in order to degrade gracefully into ABI0, we need
		// to stack-assign this type. The reason is that
		// although zero-sized types take up no space on the
		// stack, they do cause the next argument to be aligned.
		// So just do that here, but don't bother actually
		// generating a new ABI step for it (there's nothing to
		// actually copy).
		//
		// We cannot handle this in the recursive case of
		// regAssign because zero-sized *fields* of a
		// non-zero-sized struct do not cause it to be
		// stack-assigned. So we need a special case here
		// at the top.
		a.stackBytes = align(a.stackBytes, uintptr(t.Align()))
		return nil
	}
	// Hold a copy of "a" so that we can roll back if
	// register assignment fails.
	aOld := *a
	if !a.regAssign(t, 0) {
		// Register assignment failed. Roll back any changes
		// and stack-assign.
		*a = aOld
		a.stackAssign(t.Size(), uintptr(t.Align()))
		return &a.steps[len(a.steps)-1]
	}
	return nil
}

// addRcvr extends the abiSeq with a new method call
// receiver according to the interface calling convention.
//
// If the receiver was stack-assigned, returns the single
// abiStep describing that translation, and nil otherwise.
// Returns true if the receiver is a pointer.
func (a *abiSeq) addRcvr(rcvr *abi.Type) (*abiStep, bool) {
	// The receiver is always one word.
	a.valueStart = append(a.valueStart, len(a.steps))
	var ok, ptr bool
	if !rcvr.IsDirectIface() || rcvr.Pointers() {
		ok = a.assignIntN(0, goarch.PtrSize, 1, 0b1)
		ptr = true
	} else {
		// TODO(mknyszek): Is this case even possible?
		// The interface data work never contains a non-pointer
		// value. This case was copied over from older code
		// in the reflect package which only conditionally added
		// a pointer bit to the reflect.(Value).Call stack frame's
		// GC bitmap.
		ok = a.assignIntN(0, goarch.PtrSize, 1, 0b0)
		ptr = false
	}
	if !ok {
		a.stackAssign(goarch.PtrSize, goarch.PtrSize)
		return &a.steps[len(a.steps)-1], ptr
	}
	return nil, ptr
}

// regAssign attempts to reserve argument registers for a value of
// type t, stored at some offset.
//
// It returns whether or not the assignment succeeded, but
// leaves any changes it made to a.steps behind, so the caller
// must undo that work by adjusting a.steps if it fails.
//
// This method along with the assign* methods represent the
// complete register-assignment algorithm for the Go ABI.
func (a *abiSeq) regAssign(t *abi.Type, offset uintptr) bool {
	switch Kind(t.Kind()) {
	case UnsafePointer, Pointer, Chan, Map, Func:
		return a.assignIntN(offset, t.Size(), 1, 0b1)
	case Bool, Int, Uint, Int8, Uint8, Int16, Uint16, Int32, Uint32, Uintptr:
		return a.assignIntN(offset, t.Size(), 1, 0b0)
	case Int64, Uint64:
		switch goarch.PtrSize {
		case 4:
			return a.assignIntN(offset, 4, 2, 0b0)
		case 8:
			return a.assignIntN(offset, 8, 1, 0b0)
		}
	case Float32, Float64:
		return a.assignFloatN(offset, t.Size(), 1)
	case Complex64:
		return a.assignFloatN(offset, 4, 2)
	case Complex128:
		return a.assignFloatN(offset, 8, 2)
	case String:
		return a.assignIntN(offset, goarch.PtrSize, 2, 0b01)
	case Interface:
		return a.assignIntN(offset, goarch.PtrSize, 2, 0b10)
	case Slice:
		return a.assignIntN(offset, goarch.PtrSize, 3, 0b001)
	case Array:
		tt := (*arrayType)(unsafe.Pointer(t))
		switch tt.Len {
		case 0:
			// There's nothing to assign, so don't modify
			// a.steps but succeed so the caller doesn't
			// try to stack-assign this value.
			return true
		case 1:
			return a.regAssign(tt.Elem, offset)
		default:
			return false
		}
	case Struct:
		st := (*structType)(unsafe.Pointer(t))
		for i := range st.Fields {
			f := &st.Fields[i]
			if !a.regAssign(f.Typ, offset+f.Offset) {
				return false
			}
		}
		return true
	default:
		print("t.Kind == ", t.Kind(), "\n")
		panic("unknown type kind")
	}
	panic("unhandled register assignment path")
}

// assignIntN assigns n values to registers, each "size" bytes large,
// from the data at [offset, offset+n*size) in memory. Each value at
// [offset+i*size, offset+(i+1)*size) for i < n is assigned to the
// next n integer registers.
//
// Bit i in ptrMap indicates whether the i'th value is a pointer.
// n must be <= 8.
//
// Returns whether assignment succeeded.
func (a *abiSeq) assignIntN(offset, size uintptr, n int, ptrMap uint8) bool {
	if n > 8 || n < 0 {
		panic("invalid n")
	}
	if ptrMap != 0 && size != goarch.PtrSize {
		panic("non-empty pointer map passed for non-pointer-size values")
	}
	if a.iregs+n > intArgRegs {
		return false
	}
	for i := 0; i < n; i++ {
		kind := abiStepIntReg
		if ptrMap&(uint8(1)<<i) != 0 {
			kind = abiStepPointer
		}
		a.steps = append(a.steps, abiStep{
			kind:   kind,
			offset: offset + uintptr(i)*size,
			size:   size,
			ireg:   a.iregs,
		})
		a.iregs++
	}
	return true
}

// assignFloatN assigns n values to registers, each "size" bytes large,
// from the data at [offset, offset+n*size) in memory. Each value at
// [offset+i*size, offset+(i+1)*size) for i < n is assigned to the
// next n floating-point registers.
//
// Returns whether assignment succeeded.
func (a *abiSeq) assignFloatN(offset, size uintptr, n int) bool {
	if n < 0 {
		panic("invalid n")
	}
	if a.fregs+n > floatArgRegs || floatRegSize < size {
		return false
	}
	for i := 0; i < n; i++ {
		a.steps = append(a.steps, abiStep{
			kind:   abiStepFloatReg,
			offset: offset + uintptr(i)*size,
			size:   size,
			freg:   a.fregs,
		})
		a.fregs++
	}
	return true
}

// stackAssign reserves space for one value that is "size" bytes
// large with alignment "alignment" to the stack.
//
// Should not be called directly; use addArg instead.
func (a *abiSeq) stackAssign(size, alignment uintptr) {
	a.stackBytes = align(a.stackBytes, alignment)
	a.steps = append(a.steps, abiStep{
		kind:   abiStepStack,
		offset: 0, // Only used for whole arguments, so the memory offset is 0.
		size:   size,
		stkOff: a.stackBytes,
	})
	a.stackBytes += size
}

// abiDesc describes the ABI for a function or method.
type abiDesc struct {
	// call and ret represent the translation steps for
	// the call and return paths of a Go function.
	call, ret abiSeq

	// These fields describe the stack space allocated
	// for the call. stackCallArgsSize is the amount of space
	// reserved for arguments but not return values. retOffset
	// is the offset at which return values begin, and
	// spill is the size in bytes of additional space reserved
	// to spill argument registers into in case of preemption in
	// reflectcall's stack frame.
	stackCallArgsSize, retOffset, spill uintptr

	// stackPtrs is a bitmap that indicates whether
	// each word in the ABI stack space (stack-assigned
	// args + return values) is a pointer. Used
	// as the heap pointer bitmap for stack space
	// passed to reflectcall.
	stackPtrs *bitVector

	// inRegPtrs is a bitmap whose i'th bit indicates
	// whether the i'th integer argument register contains
	// a pointer. Used by makeFuncStub and methodValueCall
	// to make result pointers visible to the GC.
	//
	// outRegPtrs is the same, but for result values.
	// Used by reflectcall to make result pointers visible
	// to the GC.
	inRegPtrs, outRegPtrs abi.IntArgRegBitmap
}

func (a *abiDesc) dump() {
	println("ABI")
	println("call")
	a.call.dump()
	println("ret")
	a.ret.dump()
	println("stackCallArgsSize", a.stackCallArgsSize)
	println("retOffset", a.retOffset)
	println("spill", a.spill)
	print("inRegPtrs:")
	dumpPtrBitMap(a.inRegPtrs)
	println()
	print("outRegPtrs:")
	dumpPtrBitMap(a.outRegPtrs)
	println()
}

func dumpPtrBitMap(b abi.IntArgRegBitmap) {
	for i := 0; i < intArgRegs; i++ {
		x := 0
		if b.Get(i) {
			x = 1
		}
		print(" ", x)
	}
}

func newAbiDesc(t *funcType, rcvr *abi.Type) abiDesc {
	// We need to add space for this argument to
	// the frame so that it can spill args into it.
	//
	// The size of this space is just the sum of the sizes
	// of each register-allocated type.
	//
	// TODO(mknyszek): Remove this when we no longer have
	// caller reserved spill space.
	spill := uintptr(0)

	// Compute gc program & stack bitmap for stack arguments
	stackPtrs := new(bitVector)

	// Compute the stack frame pointer bitmap and register
	// pointer bitmap for arguments.
	inRegPtrs := abi.IntArgRegBitmap{}

	// Compute abiSeq for input parameters.
	var in abiSeq
	if rcvr != nil {
		stkStep, isPtr := in.addRcvr(rcvr)
		if stkStep != nil {
			if isPtr {
				stackPtrs.append(1)
			} else {
				stackPtrs.append(0)
			}
		} else {
			spill += goarch.PtrSize
		}
	}
	for i, arg := range t.InSlice() {
		stkStep := in.addArg(arg)
		if stkStep != nil {
			addTypeBits(stackPtrs, stkStep.stkOff, arg)
		} else {
			spill = align(spill, uintptr(arg.Align()))
			spill += arg.Size()
			for _, st := range in.stepsForValue(i) {
				if st.kind == abiStepPointer {
					inRegPtrs.Set(st.ireg)
				}
			}
		}
	}
	spill = align(spill, goarch.PtrSize)

	// From the input parameters alone, we now know
	// the stackCallArgsSize and retOffset.
	stackCallArgsSize := in.stackBytes
	retOffset := align(in.stackBytes, goarch.PtrSize)

	// Compute the stack frame pointer bitmap and register
	// pointer bitmap for return values.
	outRegPtrs := abi.IntArgRegBitmap{}

	// Compute abiSeq for output parameters.
	var out abiSeq
	// Stack-assigned return values do not share
	// space with arguments like they do with registers,
	// so we need to inject a stack offset here.
	// Fake it by artificially extending stackBytes by
	// the return offset.
	out.stackBytes = retOffset
	for i, res := range t.OutSlice() {
		stkStep := out.addArg(res)
		if stkStep != nil {
			addTypeBits(stackPtrs, stkStep.stkOff, res)
		} else {
			for _, st := range out.stepsForValue(i) {
				if st.kind == abiStepPointer {
					outRegPtrs.Set(st.ireg)
				}
			}
		}
	}
	// Undo the faking from earlier so that stackBytes
	// is accurate.
	out.stackBytes -= retOffset
	return abiDesc{in, out, stackCallArgsSize, retOffset, spill, stackPtrs, inRegPtrs, outRegPtrs}
}

// intFromReg loads an argSize sized integer from reg and places it at to.
//
// argSize must be non-zero, fit in a register, and a power-of-two.
func intFromReg(r *abi.RegArgs, reg int, argSize uintptr, to unsafe.Pointer) {
	memmove(to, r.IntRegArgAddr(reg, argSize), argSize)
}

// intToReg loads an argSize sized integer and stores it into reg.
//
// argSize must be non-zero, fit in a register, and a power-of-two.
func intToReg(r *abi.RegArgs, reg int, argSize uintptr, from unsafe.Pointer) {
	memmove(r.IntRegArgAddr(reg, argSize), from, argSize)
}

// floatFromReg loads a float value from its register representation in r.
//
// argSize must be 4 or 8.
func floatFromReg(r *abi.RegArgs, reg int, argSize uintptr, to unsafe.Pointer) {
	switch argSize {
	case 4:
		*(*float32)(to) = archFloat32FromReg(r.Floats[reg])
	case 8:
		*(*float64)(to) = *(*float64)(unsafe.Pointer(&r.Floats[reg]))
	default:
		panic("bad argSize")
	}
}

// floatToReg stores a float value in its register representation in r.
//
// argSize must be either 4 or 8.
func floatToReg(r *abi.RegArgs, reg int, argSize uintptr, from unsafe.Pointer) {
	switch argSize {
	case 4:
		r.Floats[reg] = archFloat32ToReg(*(*float32)(from))
	case 8:
		r.Floats[reg] = *(*uint64)(from)
	default:
		panic("bad argSize")
	}
}

```

// === FILE: references!/go/src/reflect/arena.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.arenas

package reflect

import "arena"

// ArenaNew returns a [Value] representing a pointer to a new zero value for the
// specified type, allocating storage for it in the provided arena. That is,
// the returned Value's Type is [PointerTo](typ).
func ArenaNew(a *arena.Arena, typ Type) Value {
	return ValueOf(arena_New(a, PointerTo(typ)))
}

func arena_New(a *arena.Arena, typ any) any

```

// === FILE: references!/go/src/reflect/asm_386.s ===
```text
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"
#include "funcdata.h"

// makeFuncStub is the code half of the function returned by MakeFunc.
// See the comment on the declaration of makeFuncStub in makefunc.go
// for more details.
// No argsize here, gc generates argsize info at call site.
TEXT ·makeFuncStub(SB),(NOSPLIT|WRAPPER),$20
	NO_LOCAL_POINTERS
	MOVL	DX, 0(SP)
	LEAL	argframe+0(FP), CX
	MOVL	CX, 4(SP)
	MOVB	$0, 16(SP)
	LEAL	16(SP), AX
	MOVL	AX, 8(SP)
	MOVL	$0, 12(SP)
	CALL	·callReflect(SB)
	RET

// methodValueCall is the code half of the function returned by makeMethodValue.
// See the comment on the declaration of methodValueCall in makefunc.go
// for more details.
// No argsize here, gc generates argsize info at call site.
TEXT ·methodValueCall(SB),(NOSPLIT|WRAPPER),$20
	NO_LOCAL_POINTERS
	MOVL	DX, 0(SP)
	LEAL	argframe+0(FP), CX
	MOVL	CX, 4(SP)
	MOVB	$0, 16(SP)
	LEAL	16(SP), AX
	MOVL	AX, 8(SP)
	MOVL	$0, 12(SP)
	CALL	·callMethod(SB)
	RET

```

// === FILE: references!/go/src/reflect/asm_amd64.s ===
```text
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"
#include "funcdata.h"
#include "go_asm.h"

// The frames of each of the two functions below contain two locals, at offsets
// that are known to the runtime.
//
// The first local is a bool called retValid with a whole pointer-word reserved
// for it on the stack. The purpose of this word is so that the runtime knows
// whether the stack-allocated return space contains valid values for stack
// scanning.
//
// The second local is an abi.RegArgs value whose offset is also known to the
// runtime, so that a stack map for it can be constructed, since it contains
// pointers visible to the GC.
#define LOCAL_RETVALID 32
#define LOCAL_REGARGS 40

// makeFuncStub is the code half of the function returned by MakeFunc.
// See the comment on the declaration of makeFuncStub in makefunc.go
// for more details.
// No arg size here; runtime pulls arg map out of the func value.
// This frame contains two locals. See the comment above LOCAL_RETVALID.
TEXT ·makeFuncStub(SB),(NOSPLIT|WRAPPER),$312
	NO_LOCAL_POINTERS
	// NO_LOCAL_POINTERS is a lie. The stack map for the two locals in this
	// frame is specially handled in the runtime. See the comment above LOCAL_RETVALID.
	LEAQ	LOCAL_REGARGS(SP), R12
	CALL	runtime·spillArgs(SB)
	MOVQ	DX, 24(SP) // outside of moveMakeFuncArgPtrs's arg area
	MOVQ	DX, 0(SP)
	MOVQ	R12, 8(SP)
	CALL	·moveMakeFuncArgPtrs(SB)
	MOVQ	24(SP), DX
	MOVQ	DX, 0(SP)
	LEAQ	argframe+0(FP), CX
	MOVQ	CX, 8(SP)
	MOVB	$0, LOCAL_RETVALID(SP)
	LEAQ	LOCAL_RETVALID(SP), AX
	MOVQ	AX, 16(SP)
	LEAQ	LOCAL_REGARGS(SP), AX
	MOVQ	AX, 24(SP)
	CALL	·callReflect(SB)
	LEAQ	LOCAL_REGARGS(SP), R12
	CALL	runtime·unspillArgs(SB)
	RET

// methodValueCall is the code half of the function returned by makeMethodValue.
// See the comment on the declaration of methodValueCall in makefunc.go
// for more details.
// No arg size here; runtime pulls arg map out of the func value.
// This frame contains two locals. See the comment above LOCAL_RETVALID.
TEXT ·methodValueCall(SB),(NOSPLIT|WRAPPER),$312
	NO_LOCAL_POINTERS
	// NO_LOCAL_POINTERS is a lie. The stack map for the two locals in this
	// frame is specially handled in the runtime. See the comment above LOCAL_RETVALID.
	LEAQ	LOCAL_REGARGS(SP), R12
	CALL	runtime·spillArgs(SB)
	MOVQ	DX, 24(SP) // outside of moveMakeFuncArgPtrs's arg area
	MOVQ	DX, 0(SP)
	MOVQ	R12, 8(SP)
	CALL	·moveMakeFuncArgPtrs(SB)
	MOVQ	24(SP), DX
	MOVQ	DX, 0(SP)
	LEAQ	argframe+0(FP), CX
	MOVQ	CX, 8(SP)
	MOVB	$0, LOCAL_RETVALID(SP)
	LEAQ	LOCAL_RETVALID(SP), AX
	MOVQ	AX, 16(SP)
	LEAQ	LOCAL_REGARGS(SP), AX
	MOVQ	AX, 24(SP)
	CALL	·callMethod(SB)
	LEAQ	LOCAL_REGARGS(SP), R12
	CALL	runtime·unspillArgs(SB)
	RET

```

// === FILE: references!/go/src/reflect/asm_arm.s ===
```text
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"
#include "funcdata.h"

// makeFuncStub is jumped to by the code generated by MakeFunc.
// See the comment on the declaration of makeFuncStub in makefunc.go
// for more details.
// No argsize here, gc generates argsize info at call site.
TEXT ·makeFuncStub(SB),(NOSPLIT|WRAPPER),$20
	NO_LOCAL_POINTERS
	MOVW	R7, 4(R13)
	MOVW	$argframe+0(FP), R1
	MOVW	R1, 8(R13)
	MOVW	$0, R1
	MOVB	R1, 20(R13)
	ADD	$20, R13, R1
	MOVW	R1, 12(R13)
	MOVW	$0, R1
	MOVW	R1, 16(R13)
	BL	·callReflect(SB)
	RET

// methodValueCall is the code half of the function returned by makeMethodValue.
// See the comment on the declaration of methodValueCall in makefunc.go
// for more details.
// No argsize here, gc generates argsize info at call site.
TEXT ·methodValueCall(SB),(NOSPLIT|WRAPPER),$20
	NO_LOCAL_POINTERS
	MOVW	R7, 4(R13)
	MOVW	$argframe+0(FP), R1
	MOVW	R1, 8(R13)
	MOVW	$0, R1
	MOVB	R1, 20(R13)
	ADD	$20, R13, R1
	MOVW	R1, 12(R13)
	MOVW	$0, R1
	MOVW	R1, 16(R13)
	BL	·callMethod(SB)
	RET

```

// === FILE: references!/go/src/reflect/asm_arm64.s ===
```text
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"
#include "funcdata.h"

// The frames of each of the two functions below contain two locals, at offsets
// that are known to the runtime.
//
// The first local is a bool called retValid with a whole pointer-word reserved
// for it on the stack. The purpose of this word is so that the runtime knows
// whether the stack-allocated return space contains valid values for stack
// scanning.
//
// The second local is an abi.RegArgs value whose offset is also known to the
// runtime, so that a stack map for it can be constructed, since it contains
// pointers visible to the GC.
#define LOCAL_RETVALID 40
#define LOCAL_REGARGS 48

// The frame size of the functions below is
// 32 (args of callReflect) + 8 (bool + padding) + 392 (abi.RegArgs) = 432.

// makeFuncStub is the code half of the function returned by MakeFunc.
// See the comment on the declaration of makeFuncStub in makefunc.go
// for more details.
// No arg size here, runtime pulls arg map out of the func value.
TEXT ·makeFuncStub(SB),(NOSPLIT|WRAPPER),$432
	NO_LOCAL_POINTERS
	// NO_LOCAL_POINTERS is a lie. The stack map for the two locals in this
	// frame is specially handled in the runtime. See the comment above LOCAL_RETVALID.
	ADD	$LOCAL_REGARGS, RSP, R20
	CALL	runtime·spillArgs(SB)
	MOVD	R26, 32(RSP) // outside of moveMakeFuncArgPtrs's arg area
	MOVD	R26, R0
	MOVD	R20, R1
	CALL	·moveMakeFuncArgPtrs<ABIInternal>(SB)
	MOVD	32(RSP), R26
	MOVD	R26, 8(RSP)
	MOVD	$argframe+0(FP), R3
	MOVD	R3, 16(RSP)
	MOVB	$0, LOCAL_RETVALID(RSP)
	ADD	$LOCAL_RETVALID, RSP, R3
	MOVD	R3, 24(RSP)
	ADD	$LOCAL_REGARGS, RSP, R3
	MOVD	R3, 32(RSP)
	CALL	·callReflect(SB)
	ADD	$LOCAL_REGARGS, RSP, R20
	CALL	runtime·unspillArgs(SB)
	RET

// methodValueCall is the code half of the function returned by makeMethodValue.
// See the comment on the declaration of methodValueCall in makefunc.go
// for more details.
// No arg size here; runtime pulls arg map out of the func value.
TEXT ·methodValueCall(SB),(NOSPLIT|WRAPPER),$432
	NO_LOCAL_POINTERS
	// NO_LOCAL_POINTERS is a lie. The stack map for the two locals in this
	// frame is specially handled in the runtime. See the comment above LOCAL_RETVALID.
	ADD	$LOCAL_REGARGS, RSP, R20
	CALL	runtime·spillArgs(SB)
	MOVD	R26, 32(RSP) // outside of moveMakeFuncArgPtrs's arg area
	MOVD	R26, R0
	MOVD	R20, R1
	CALL	·moveMakeFuncArgPtrs<ABIInternal>(SB)
	MOVD	32(RSP), R26
	MOVD	R26, 8(RSP)
	MOVD	$argframe+0(FP), R3
	MOVD	R3, 16(RSP)
	MOVB	$0, LOCAL_RETVALID(RSP)
	ADD	$LOCAL_RETVALID, RSP, R3
	MOVD	R3, 24(RSP)
	ADD	$LOCAL_REGARGS, RSP, R3
	MOVD	R3, 32(RSP)
	CALL	·callMethod(SB)
	ADD	$LOCAL_REGARGS, RSP, R20
	CALL	runtime·unspillArgs(SB)
	RET

```

// === FILE: references!/go/src/reflect/asm_loong64.s ===
```text
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"
#include "funcdata.h"

#define	REGCTXT	R29

// The frames of each of the two functions below contain two locals, at offsets
// that are known to the runtime.
//
// The first local is a bool called retValid with a whole pointer-word reserved
// for it on the stack. The purpose of this word is so that the runtime knows
// whether the stack-allocated return space contains valid values for stack
// scanning.
//
// The second local is an abi.RegArgs value whose offset is also known to the
// runtime, so that a stack map for it can be constructed, since it contains
// pointers visible to the GC.
#define LOCAL_RETVALID 40
#define LOCAL_REGARGS 48

// The frame size of the functions below is
// 32 (args of callReflect) + 8 (bool + padding) + 392 (abi.RegArgs) = 432.

// makeFuncStub is the code half of the function returned by MakeFunc.
// See the comment on the declaration of makeFuncStub in makefunc.go
// for more details.
// No arg size here, runtime pulls arg map out of the func value.
TEXT ·makeFuncStub(SB),(NOSPLIT|WRAPPER),$432
	NO_LOCAL_POINTERS
	ADDV	$LOCAL_REGARGS, R3, R25 // spillArgs using R25
	JAL	runtime·spillArgs(SB)
	MOVV	REGCTXT, 32(R3) // save REGCTXT > args of moveMakeFuncArgPtrs < LOCAL_REGARGS

	MOVV	REGCTXT, R4
	MOVV	R25, R5
	JAL	·moveMakeFuncArgPtrs<ABIInternal>(SB)
	MOVV	32(R3), REGCTXT // restore REGCTXT

	MOVV	REGCTXT, 8(R3)
	MOVV	$argframe+0(FP), R20
	MOVV	R20, 16(R3)
	MOVV	R0, LOCAL_RETVALID(R3)
	ADDV	$LOCAL_RETVALID, R3, R20
	MOVV	R20, 24(R3)
	ADDV	$LOCAL_REGARGS, R3, R20
	MOVV	R20, 32(R3)
	JAL	·callReflect(SB)
	ADDV	$LOCAL_REGARGS, R3, R25	//unspillArgs using R25
	JAL	runtime·unspillArgs(SB)
	RET

// methodValueCall is the code half of the function returned by makeMethodValue.
// See the comment on the declaration of methodValueCall in makefunc.go
// for more details.
// No arg size here; runtime pulls arg map out of the func value.
TEXT ·methodValueCall(SB),(NOSPLIT|WRAPPER),$432
	NO_LOCAL_POINTERS
	ADDV	$LOCAL_REGARGS, R3, R25 // spillArgs using R25
	JAL	runtime·spillArgs(SB)
	MOVV	REGCTXT, 32(R3) // save REGCTXT > args of moveMakeFuncArgPtrs < LOCAL_REGARGS
	MOVV	REGCTXT, R4
	MOVV	R25, R5
	JAL	·moveMakeFuncArgPtrs<ABIInternal>(SB)
	MOVV	32(R3), REGCTXT // restore REGCTXT
	MOVV	REGCTXT, 8(R3)
	MOVV	$argframe+0(FP), R20
	MOVV	R20, 16(R3)
	MOVB	R0, LOCAL_RETVALID(R3)
	ADDV	$LOCAL_RETVALID, R3, R20
	MOVV	R20, 24(R3)
	ADDV	$LOCAL_REGARGS, R3, R20
	MOVV	R20, 32(R3) // frame size to 32+SP as callreflect args)
	JAL	·callMethod(SB)
	ADDV	$LOCAL_REGARGS, R3, R25 // unspillArgs using R25
	JAL	runtime·unspillArgs(SB)
	RET

```

// === FILE: references!/go/src/reflect/asm_mips64x.s ===
```text
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build mips64 || mips64le

#include "textflag.h"
#include "funcdata.h"

#define	REGCTXT	R22

// makeFuncStub is the code half of the function returned by MakeFunc.
// See the comment on the declaration of makeFuncStub in makefunc.go
// for more details.
// No arg size here, runtime pulls arg map out of the func value.
TEXT ·makeFuncStub(SB),(NOSPLIT|WRAPPER),$40
	NO_LOCAL_POINTERS
	MOVV	REGCTXT, 8(R29)
	MOVV	$argframe+0(FP), R1
	MOVV	R1, 16(R29)
	MOVB	R0, 40(R29)
	ADDV	$40, R29, R1
	MOVV	R1, 24(R29)
	MOVV	R0, 32(R29)
	JAL	·callReflect(SB)
	RET

// methodValueCall is the code half of the function returned by makeMethodValue.
// See the comment on the declaration of methodValueCall in makefunc.go
// for more details.
// No arg size here; runtime pulls arg map out of the func value.
TEXT ·methodValueCall(SB),(NOSPLIT|WRAPPER),$40
	NO_LOCAL_POINTERS
	MOVV	REGCTXT, 8(R29)
	MOVV	$argframe+0(FP), R1
	MOVV	R1, 16(R29)
	MOVB	R0, 40(R29)
	ADDV	$40, R29, R1
	MOVV	R1, 24(R29)
	MOVV	R0, 32(R29)
	JAL	·callMethod(SB)
	RET

```

// === FILE: references!/go/src/reflect/asm_mipsx.s ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build mips || mipsle

#include "textflag.h"
#include "funcdata.h"

#define	REGCTXT	R22

// makeFuncStub is the code half of the function returned by MakeFunc.
// See the comment on the declaration of makeFuncStub in makefunc.go
// for more details.
// No arg size here, runtime pulls arg map out of the func value.
TEXT ·makeFuncStub(SB),(NOSPLIT|WRAPPER),$20
	NO_LOCAL_POINTERS
	MOVW	REGCTXT, 4(R29)
	MOVW	$argframe+0(FP), R1
	MOVW	R1, 8(R29)
	MOVB	R0, 20(R29)
	ADD	$20, R29, R1
	MOVW	R1, 12(R29)
	MOVW	R0, 16(R29)
	JAL	·callReflect(SB)
	RET

// methodValueCall is the code half of the function returned by makeMethodValue.
// See the comment on the declaration of methodValueCall in makefunc.go
// for more details.
// No arg size here; runtime pulls arg map out of the func value.
TEXT ·methodValueCall(SB),(NOSPLIT|WRAPPER),$20
	NO_LOCAL_POINTERS
	MOVW	REGCTXT, 4(R29)
	MOVW	$argframe+0(FP), R1
	MOVW	R1, 8(R29)
	MOVB	R0, 20(R29)
	ADD	$20, R29, R1
	MOVW	R1, 12(R29)
	MOVW	R0, 16(R29)
	JAL	·callMethod(SB)
	RET

```

// === FILE: references!/go/src/reflect/asm_ppc64x.s ===
```text
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ppc64 || ppc64le

#include "textflag.h"
#include "funcdata.h"
#include "asm_ppc64x.h"

// The frames of each of the two functions below contain two locals, at offsets
// that are known to the runtime.
//
// The first local is a bool called retValid with a whole pointer-word reserved
// for it on the stack. The purpose of this word is so that the runtime knows
// whether the stack-allocated return space contains valid values for stack
// scanning.
//
// The second local is an abi.RegArgs value whose offset is also known to the
// runtime, so that a stack map for it can be constructed, since it contains
// pointers visible to the GC.

#define LOCAL_RETVALID 32+FIXED_FRAME
#define LOCAL_REGARGS 40+FIXED_FRAME

// The frame size of the functions below is
// 32 (args of callReflect) + 8 (bool + padding) + 296 (abi.RegArgs) = 336.

// makeFuncStub is the code half of the function returned by MakeFunc.
// See the comment on the declaration of makeFuncStub in makefunc.go
// for more details.
// No arg size here, runtime pulls arg map out of the func value.
TEXT ·makeFuncStub(SB),(NOSPLIT|WRAPPER),$336
	NO_LOCAL_POINTERS
	// NO_LOCAL_POINTERS is a lie. The stack map for the two locals in this
	// frame is specially handled in the runtime. See the comment above LOCAL_RETVALID.
	ADD	$LOCAL_REGARGS, R1, R20
	CALL	runtime·spillArgs(SB)
	MOVD	R11, FIXED_FRAME+32(R1)	// save R11
	MOVD	R11, FIXED_FRAME+0(R1)	// arg for moveMakeFuncArgPtrs
	MOVD	R20, FIXED_FRAME+8(R1)	// arg for local args
	CALL	·moveMakeFuncArgPtrs(SB)
	MOVD	FIXED_FRAME+32(R1), R11	// restore R11 ctxt
	MOVD	R11, FIXED_FRAME+0(R1)	// ctxt (arg0)
	MOVD	$argframe+0(FP), R3	// save arg to callArg
	MOVD	R3, FIXED_FRAME+8(R1)	// frame (arg1)
	ADD	$LOCAL_RETVALID, R1, R3 // addr of return flag
	MOVB	R0, (R3)		// clear flag
	MOVD	R3, FIXED_FRAME+16(R1)	// addr retvalid (arg2)
	ADD     $LOCAL_REGARGS, R1, R3
	MOVD	R3, FIXED_FRAME+24(R1)	// abiregargs (arg3)
	BL	·callReflect(SB)
	ADD	$LOCAL_REGARGS, R1, R20	// set address of spill area
	CALL	runtime·unspillArgs(SB)
	RET

// methodValueCall is the code half of the function returned by makeMethodValue.
// See the comment on the declaration of methodValueCall in makefunc.go
// for more details.
// No arg size here; runtime pulls arg map out of the func value.
TEXT ·methodValueCall(SB),(NOSPLIT|WRAPPER),$336
	NO_LOCAL_POINTERS
	// NO_LOCAL_POINTERS is a lie. The stack map for the two locals in this
	// frame is specially handled in the runtime. See the comment above LOCAL_RETVALID.
	ADD	$LOCAL_REGARGS, R1, R20
	CALL	runtime·spillArgs(SB)
	MOVD	R11, FIXED_FRAME+0(R1) // arg0 ctxt
	MOVD	R11, FIXED_FRAME+32(R1) // save for later
	MOVD	R20, FIXED_FRAME+8(R1) // arg1 abiregargs
	CALL	·moveMakeFuncArgPtrs(SB)
	MOVD	FIXED_FRAME+32(R1), R11 // restore ctxt
	MOVD	R11, FIXED_FRAME+0(R1) // set as arg0
	MOVD	$argframe+0(FP), R3	// frame pointer
	MOVD	R3, FIXED_FRAME+8(R1)	// set as arg1
	ADD	$LOCAL_RETVALID, R1, R3
	MOVB	$0, (R3)		// clear ret flag
	MOVD	R3, FIXED_FRAME+16(R1)	// addr of return flag
	ADD	$LOCAL_REGARGS, R1, R3	// addr of abiregargs
	MOVD	R3, FIXED_FRAME+24(R1)	// set as arg3
	BL	·callMethod(SB)
	ADD     $LOCAL_REGARGS, R1, R20
	CALL	runtime·unspillArgs(SB)
	RET

```

// === FILE: references!/go/src/reflect/asm_riscv64.s ===
```text
// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"
#include "funcdata.h"

// The frames of each of the two functions below contain two locals, at offsets
// that are known to the runtime.
//
// The first local is a bool called retValid with a whole pointer-word reserved
// for it on the stack. The purpose of this word is so that the runtime knows
// whether the stack-allocated return space contains valid values for stack
// scanning.
//
// The second local is an abi.RegArgs value whose offset is also known to the
// runtime, so that a stack map for it can be constructed, since it contains
// pointers visible to the GC.
#define LOCAL_RETVALID 40
#define LOCAL_REGARGS 48

// The frame size of the functions below is
// 32 (args of callReflect/callMethod) + (8 bool with padding) + 392 (abi.RegArgs) = 432.

// makeFuncStub is the code half of the function returned by MakeFunc.
// See the comment on the declaration of makeFuncStub in makefunc.go
// for more details.
// No arg size here, runtime pulls arg map out of the func value.
TEXT ·makeFuncStub(SB),(NOSPLIT|WRAPPER),$432
	NO_LOCAL_POINTERS
	ADD	$LOCAL_REGARGS, SP, X25 // spillArgs using X25
	CALL	runtime·spillArgs(SB)
	MOV	CTXT, 32(SP) // save CTXT > args of moveMakeFuncArgPtrs < LOCAL_REGARGS
	MOV	CTXT, 8(SP)
	MOV	X25, 16(SP)
	CALL	·moveMakeFuncArgPtrs(SB)
	MOV	32(SP), CTXT // restore CTXT

	MOV	CTXT, 8(SP)
	MOV	$argframe+0(FP), T0
	MOV	T0, 16(SP)
	MOV	ZERO, LOCAL_RETVALID(SP)
	ADD	$LOCAL_RETVALID, SP, T1
	MOV	T1, 24(SP)
	ADD	$LOCAL_REGARGS, SP, T1
	MOV	T1, 32(SP)
	CALL	·callReflect(SB)
	ADD	$LOCAL_REGARGS, SP, X25 // unspillArgs using X25
	CALL	runtime·unspillArgs(SB)
	RET

// methodValueCall is the code half of the function returned by makeMethodValue.
// See the comment on the declaration of methodValueCall in makefunc.go
// for more details.
// No arg size here; runtime pulls arg map out of the func value.
TEXT ·methodValueCall(SB),(NOSPLIT|WRAPPER),$432
	NO_LOCAL_POINTERS
	ADD	$LOCAL_REGARGS, SP, X25 // spillArgs using X25
	CALL	runtime·spillArgs(SB)
	MOV	CTXT, 32(SP) // save CTXT
	MOV	CTXT, 8(SP)
	MOV	X25, 16(SP)
	CALL	·moveMakeFuncArgPtrs(SB)
	MOV	32(SP), CTXT // restore CTXT
	MOV	CTXT, 8(SP)
	MOV	$argframe+0(FP), T0
	MOV	T0, 16(SP)
	MOV	ZERO, LOCAL_RETVALID(SP)
	ADD	$LOCAL_RETVALID, SP, T1
	MOV	T1, 24(SP)
	ADD	$LOCAL_REGARGS, SP, T1
	MOV	T1, 32(SP) // frame size to 32+SP as callreflect args
	CALL	·callMethod(SB)
	ADD	$LOCAL_REGARGS, SP, X25 // unspillArgs using X25
	CALL	runtime·unspillArgs(SB)
	RET

```

// === FILE: references!/go/src/reflect/asm_s390x.s ===
```text
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"
#include "funcdata.h"

// The frames of each of the two functions below contain two locals, at offsets
// that are known to the runtime.
//
// The first local is a bool called retValid with a whole pointer-word reserved
// for it on the stack. The purpose of this word is so that the runtime knows
// whether the stack-allocated return space contains valid values for stack
// scanning.
//
// The second local is an abi.RegArgs value whose offset is also known to the
// runtime, so that a stack map for it can be constructed, since it contains
// pointers visible to the GC.

#define LOCAL_RETVALID 40
#define LOCAL_REGARGS 48

// The frame size of the functions below is
// 32 (args of callReflect/callMethod) + 8 (bool + padding) + 264 (abi.RegArgs) = 304.

// makeFuncStub is the code half of the function returned by MakeFunc.
// See the comment on the declaration of makeFuncStub in makefunc.go
// for more details.
// No arg size here, runtime pulls arg map out of the func value.
TEXT ·makeFuncStub(SB),(NOSPLIT|WRAPPER),$304
	NO_LOCAL_POINTERS
	ADD	$LOCAL_REGARGS, R15, R10 // spillArgs using R10
	BL	runtime·spillArgs(SB)
	MOVD	R12, 32(R15) // save context reg R12 > args of moveMakeFuncArgPtrs < LOCAL_REGARGS
	MOVD	R12, R2
	MOVD	R10, R3
	BL	·moveMakeFuncArgPtrs<ABIInternal>(SB)
	MOVD	32(R15), R12 // restore context reg R12
	MOVD	R12, 8(R15)
	MOVD	$argframe+0(FP), R1
	MOVD	R1, 16(R15)
	MOVB	$0, LOCAL_RETVALID(R15)
	ADD	$LOCAL_RETVALID, R15, R1
	MOVD	R1, 24(R15)
	ADD	$LOCAL_REGARGS, R15, R1
	MOVD	R1, 32(R15)
	BL	·callReflect(SB)
	ADD	$LOCAL_REGARGS, R15, R10 // unspillArgs using R10
	BL	runtime·unspillArgs(SB)
	RET

// methodValueCall is the code half of the function returned by makeMethodValue.
// See the comment on the declaration of methodValueCall in makefunc.go
// for more details.
// No arg size here; runtime pulls arg map out of the func value.
TEXT ·methodValueCall(SB),(NOSPLIT|WRAPPER),$304
	NO_LOCAL_POINTERS
	ADD	$LOCAL_REGARGS, R15, R10 // spillArgs using R10
	BL	runtime·spillArgs(SB)
	MOVD	R12, 32(R15) // save context reg R12 > args of moveMakeFuncArgPtrs < LOCAL_REGARGS
	MOVD	R12, R2
	MOVD	R10, R3
	BL	·moveMakeFuncArgPtrs<ABIInternal>(SB)
	MOVD	32(R15), R12 // restore context reg R12
	MOVD	R12, 8(R15)
	MOVD	$argframe+0(FP), R1
	MOVD	R1, 16(R15)
	MOVB	$0, LOCAL_RETVALID(R15)
	ADD	$LOCAL_RETVALID, R15, R1
	MOVD	R1, 24(R15)
	ADD	$LOCAL_REGARGS, R15, R1
	MOVD	R1, 32(R15)
	BL	·callMethod(SB)
	ADD	$LOCAL_REGARGS, R15, R10 // unspillArgs using R10
	BL	runtime·unspillArgs(SB)
	RET

```

// === FILE: references!/go/src/reflect/asm_wasm.s ===
```text
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"
#include "funcdata.h"

// makeFuncStub is the code half of the function returned by MakeFunc.
// See the comment on the declaration of makeFuncStub in makefunc.go
// for more details.
// No arg size here; runtime pulls arg map out of the func value.
TEXT ·makeFuncStub(SB),(NOSPLIT|WRAPPER),$40
	NO_LOCAL_POINTERS

	MOVD CTXT, 0(SP)

	Get SP
	Get SP
	I64ExtendI32U
	I64Const $argframe+0(FP)
	I64Add
	I64Store $8

	MOVB $0, 32(SP)
	MOVD $32(SP), 16(SP)
	MOVD $0, 24(SP)

	CALL ·callReflect(SB)
	RET

// methodValueCall is the code half of the function returned by makeMethodValue.
// See the comment on the declaration of methodValueCall in makefunc.go
// for more details.
// No arg size here; runtime pulls arg map out of the func value.
TEXT ·methodValueCall(SB),(NOSPLIT|WRAPPER),$40
	NO_LOCAL_POINTERS

	MOVD CTXT, 0(SP)

	Get SP
	Get SP
	I64ExtendI32U
	I64Const $argframe+0(FP)
	I64Add
	I64Store $8

	MOVB $0, 32(SP)
	MOVD $32(SP), 16(SP)
	MOVD $0, 24(SP)

	CALL ·callMethod(SB)
	RET

```

// === FILE: references!/go/src/reflect/badlinkname.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reflect

import (
	"internal/abi"
	"unsafe"
	_ "unsafe"
)

// Widely used packages access these symbols using linkname,
// most notably:
//  - github.com/goccy/go-json
//  - github.com/goccy/go-reflect
//  - github.com/sohaha/zlsgo
//  - github.com/undefinedlabs/go-mpatch
//
// Do not remove or change the type signature.
// See go.dev/issue/67401
// and go.dev/issue/67279.

// ifaceIndir reports whether t is stored indirectly in an interface value.
// It is no longer used by this package and is here entirely for the
// linkname uses.
//
//go:linkname unusedIfaceIndir reflect.ifaceIndir
func unusedIfaceIndir(t *abi.Type) bool {
	return !t.IsDirectIface()
}

//go:linkname valueInterface

// The compiler doesn't allow linknames on methods, for good reasons.
// We use this trick to push linknames of the methods.
// Do not call them in this package.

//go:linkname badlinkname_rtype_Align reflect.(*rtype).Align
func badlinkname_rtype_Align(*rtype) int

//go:linkname badlinkname_rtype_AssignableTo reflect.(*rtype).AssignableTo
func badlinkname_rtype_AssignableTo(*rtype, Type) bool

//go:linkname badlinkname_rtype_Bits reflect.(*rtype).Bits
func badlinkname_rtype_Bits(*rtype) int

//go:linkname badlinkname_rtype_ChanDir reflect.(*rtype).ChanDir
func badlinkname_rtype_ChanDir(*rtype) ChanDir

//go:linkname badlinkname_rtype_Comparable reflect.(*rtype).Comparable
func badlinkname_rtype_Comparable(*rtype) bool

//go:linkname badlinkname_rtype_ConvertibleTo reflect.(*rtype).ConvertibleTo
func badlinkname_rtype_ConvertibleTo(*rtype, Type) bool

//go:linkname badlinkname_rtype_Elem reflect.(*rtype).Elem
func badlinkname_rtype_Elem(*rtype) Type

//go:linkname badlinkname_rtype_Field reflect.(*rtype).Field
func badlinkname_rtype_Field(*rtype, int) StructField

//go:linkname badlinkname_rtype_FieldAlign reflect.(*rtype).FieldAlign
func badlinkname_rtype_FieldAlign(*rtype) int

//go:linkname badlinkname_rtype_FieldByIndex reflect.(*rtype).FieldByIndex
func badlinkname_rtype_FieldByIndex(*rtype, []int) StructField

//go:linkname badlinkname_rtype_FieldByName reflect.(*rtype).FieldByName
func badlinkname_rtype_FieldByName(*rtype, string) (StructField, bool)

//go:linkname badlinkname_rtype_FieldByNameFunc reflect.(*rtype).FieldByNameFunc
func badlinkname_rtype_FieldByNameFunc(*rtype, func(string) bool) (StructField, bool)

//go:linkname badlinkname_rtype_Implements reflect.(*rtype).Implements
func badlinkname_rtype_Implements(*rtype, Type) bool

//go:linkname badlinkname_rtype_In reflect.(*rtype).In
func badlinkname_rtype_In(*rtype, int) Type

//go:linkname badlinkname_rtype_IsVariadic reflect.(*rtype).IsVariadic
func badlinkname_rtype_IsVariadic(*rtype) bool

//go:linkname badlinkname_rtype_Key reflect.(*rtype).Key
func badlinkname_rtype_Key(*rtype) Type

//go:linkname badlinkname_rtype_Kind reflect.(*rtype).Kind
func badlinkname_rtype_Kind(*rtype) Kind

//go:linkname badlinkname_rtype_Len reflect.(*rtype).Len
func badlinkname_rtype_Len(*rtype) int

//go:linkname badlinkname_rtype_Method reflect.(*rtype).Method
func badlinkname_rtype_Method(*rtype, int) Method

//go:linkname badlinkname_rtype_MethodByName reflect.(*rtype).MethodByName
func badlinkname_rtype_MethodByName(*rtype, string) (Method, bool)

//go:linkname badlinkname_rtype_Name reflect.(*rtype).Name
func badlinkname_rtype_Name(*rtype) string

//go:linkname badlinkname_rtype_NumField reflect.(*rtype).NumField
func badlinkname_rtype_NumField(*rtype) int

//go:linkname badlinkname_rtype_NumIn reflect.(*rtype).NumIn
func badlinkname_rtype_NumIn(*rtype) int

//go:linkname badlinkname_rtype_NumMethod reflect.(*rtype).NumMethod
func badlinkname_rtype_NumMethod(*rtype) int

//go:linkname badlinkname_rtype_NumOut reflect.(*rtype).NumOut
func badlinkname_rtype_NumOut(*rtype) int

//go:linkname badlinkname_rtype_Out reflect.(*rtype).Out
func badlinkname_rtype_Out(*rtype, int) Type

//go:linkname badlinkname_rtype_PkgPath reflect.(*rtype).PkgPath
func badlinkname_rtype_PkgPath(*rtype) string

//go:linkname badlinkname_rtype_Size reflect.(*rtype).Size
func badlinkname_rtype_Size(*rtype) uintptr

//go:linkname badlinkname_rtype_String reflect.(*rtype).String
func badlinkname_rtype_String(*rtype) string

//go:linkname badlinkname_rtype_ptrTo reflect.(*rtype).ptrTo
func badlinkname_rtype_ptrTo(*rtype) *abi.Type

//go:linkname badlinkname_Value_pointer reflect.(*Value).pointer
func badlinkname_Value_pointer(Value) unsafe.Pointer

```

// === FILE: references!/go/src/reflect/deepequal.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Deep equality test via reflection

package reflect

import (
	"internal/bytealg"
	"unsafe"
)

// During deepValueEqual, must keep track of checks that are
// in progress. The comparison algorithm assumes that all
// checks in progress are true when it reencounters them.
// Visited comparisons are stored in a map indexed by visit.
type visit struct {
	a1  unsafe.Pointer
	a2  unsafe.Pointer
	typ Type
}

// Tests for deep equality using reflected types. The map argument tracks
// comparisons that have already been seen, which allows short circuiting on
// recursive types.
func deepValueEqual(v1, v2 Value, visited map[visit]bool) bool {
	if !v1.IsValid() || !v2.IsValid() {
		return v1.IsValid() == v2.IsValid()
	}
	if v1.Type() != v2.Type() {
		return false
	}

	// We want to avoid putting more in the visited map than we need to.
	// For any possible reference cycle that might be encountered,
	// hard(v1, v2) needs to return true for at least one of the types in the cycle,
	// and it's safe and valid to get Value's internal pointer.
	hard := func(v1, v2 Value) bool {
		switch v1.Kind() {
		case Pointer:
			if !v1.typ().Pointers() {
				// not-in-heap pointers can't be cyclic.
				// At least, all of our current uses of internal/runtime/sys.NotInHeap
				// have that property. The runtime ones aren't cyclic (and we don't use
				// DeepEqual on them anyway), and the cgo-generated ones are
				// all empty structs.
				return false
			}
			fallthrough
		case Map, Slice, Interface:
			// Nil pointers cannot be cyclic. Avoid putting them in the visited map.
			return !v1.IsNil() && !v2.IsNil()
		}
		return false
	}

	if hard(v1, v2) {
		// For a Pointer or Map value, we need to check flagIndir,
		// which we do by calling the pointer method.
		// For Slice or Interface, flagIndir is always set,
		// and using v.ptr suffices.
		ptrval := func(v Value) unsafe.Pointer {
			switch v.Kind() {
			case Pointer, Map:
				return v.pointer()
			default:
				return v.ptr
			}
		}
		addr1 := ptrval(v1)
		addr2 := ptrval(v2)
		if uintptr(addr1) > uintptr(addr2) {
			// Canonicalize order to reduce number of entries in visited.
			// Assumes non-moving garbage collector.
			addr1, addr2 = addr2, addr1
		}

		// Short circuit if references are already seen.
		typ := v1.Type()
		v := visit{addr1, addr2, typ}
		if visited[v] {
			return true
		}

		// Remember for later.
		visited[v] = true
	}

	switch v1.Kind() {
	case Array:
		for i := 0; i < v1.Len(); i++ {
			if !deepValueEqual(v1.Index(i), v2.Index(i), visited) {
				return false
			}
		}
		return true
	case Slice:
		if v1.IsNil() != v2.IsNil() {
			return false
		}
		if v1.Len() != v2.Len() {
			return false
		}
		if v1.UnsafePointer() == v2.UnsafePointer() {
			return true
		}
		// Special case for []byte, which is common.
		if v1.Type().Elem().Kind() == Uint8 {
			return bytealg.Equal(v1.Bytes(), v2.Bytes())
		}
		for i := 0; i < v1.Len(); i++ {
			if !deepValueEqual(v1.Index(i), v2.Index(i), visited) {
				return false
			}
		}
		return true
	case Interface:
		if v1.IsNil() || v2.IsNil() {
			return v1.IsNil() == v2.IsNil()
		}
		return deepValueEqual(v1.Elem(), v2.Elem(), visited)
	case Pointer:
		if v1.UnsafePointer() == v2.UnsafePointer() {
			return true
		}
		return deepValueEqual(v1.Elem(), v2.Elem(), visited)
	case Struct:
		for i, n := 0, v1.NumField(); i < n; i++ {
			if !deepValueEqual(v1.Field(i), v2.Field(i), visited) {
				return false
			}
		}
		return true
	case Map:
		if v1.IsNil() != v2.IsNil() {
			return false
		}
		if v1.Len() != v2.Len() {
			return false
		}
		if v1.UnsafePointer() == v2.UnsafePointer() {
			return true
		}
		iter := v1.MapRange()
		for iter.Next() {
			val1 := iter.Value()
			val2 := v2.MapIndex(iter.Key())
			if !val1.IsValid() || !val2.IsValid() || !deepValueEqual(val1, val2, visited) {
				return false
			}
		}
		return true
	case Func:
		if v1.IsNil() && v2.IsNil() {
			return true
		}
		// Can't do better than this:
		return false
	case Int, Int8, Int16, Int32, Int64:
		return v1.Int() == v2.Int()
	case Uint, Uint8, Uint16, Uint32, Uint64, Uintptr:
		return v1.Uint() == v2.Uint()
	case String:
		return v1.String() == v2.String()
	case Bool:
		return v1.Bool() == v2.Bool()
	case Float32, Float64:
		return v1.Float() == v2.Float()
	case Complex64, Complex128:
		return v1.Complex() == v2.Complex()
	default:
		// Normal equality suffices
		return valueInterface(v1, false) == valueInterface(v2, false)
	}
}

// DeepEqual reports whether x and y are “deeply equal,” defined as follows.
// Two values of identical type are deeply equal if one of the following cases applies.
// Values of distinct types are never deeply equal.
//
// Array values are deeply equal when their corresponding elements are deeply equal.
//
// Struct values are deeply equal if their corresponding fields,
// both exported and unexported, are deeply equal.
//
// Func values are deeply equal if both are nil; otherwise they are not deeply equal.
//
// Interface values are deeply equal if they hold deeply equal concrete values.
//
// Map values are deeply equal when all of the following are true:
// they are both nil or both non-nil, they have the same length,
// and either they are the same map object or their corresponding keys
// (matched using Go equality) map to deeply equal values.
//
// Pointer values are deeply equal if they are equal using Go's == operator
// or if they point to deeply equal values.
//
// Slice values are deeply equal when all of the following are true:
// they are both nil or both non-nil, they have the same length,
// and either they point to the same initial entry of the same underlying array
// (that is, &x[0] == &y[0]) or their corresponding elements (up to length) are deeply equal.
// Note that a non-nil empty slice and a nil slice (for example, []byte{} and []byte(nil))
// are not deeply equal.
//
// Other values - numbers, bools, strings, and channels - are deeply equal
// if they are equal using Go's == operator.
//
// In general DeepEqual is a recursive relaxation of Go's == operator.
// However, this idea is impossible to implement without some inconsistency.
// Specifically, it is possible for a value to be unequal to itself,
// either because it is of func type (uncomparable in general)
// or because it is a floating-point NaN value (not equal to itself in floating-point comparison),
// or because it is an array, struct, or interface containing
// such a value.
// On the other hand, pointer values are always equal to themselves,
// even if they point at or contain such problematic values,
// because they compare equal using Go's == operator, and that
// is a sufficient condition to be deeply equal, regardless of content.
// DeepEqual has been defined so that the same short-cut applies
// to slices and maps: if x and y are the same slice or the same map,
// they are deeply equal regardless of content.
//
// As DeepEqual traverses the data values it may find a cycle. The
// second and subsequent times that DeepEqual compares two pointer
// values that have been compared before, it treats the values as
// equal rather than examining the values to which they point.
// This ensures that DeepEqual terminates.
func DeepEqual(x, y any) bool {
	if x == nil || y == nil {
		return x == y
	}
	v1 := ValueOf(x)
	v2 := ValueOf(y)
	if v1.Type() != v2.Type() {
		return false
	}
	return deepValueEqual(v1, v2, make(map[visit]bool))
}

```

// === FILE: references!/go/src/reflect/float32reg_generic.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !ppc64 && !ppc64le && !riscv64 && !s390x

package reflect

import "unsafe"

// This file implements a straightforward conversion of a float32
// value into its representation in a register. This conversion
// applies for amd64 and arm64. It is also chosen for the case of
// zero argument registers, but is not used.

func archFloat32FromReg(reg uint64) float32 {
	i := uint32(reg)
	return *(*float32)(unsafe.Pointer(&i))
}

func archFloat32ToReg(val float32) uint64 {
	return uint64(*(*uint32)(unsafe.Pointer(&val)))
}

```

// === FILE: references!/go/src/reflect/float32reg_ppc64x.s ===
```text
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ppc64 || ppc64le

#include "textflag.h"

// On PPC64, the float32 becomes a float64
// when loaded in a register, different from
// other platforms. These functions are
// needed to ensure correct conversions on PPC64.

// Convert float32->uint64
TEXT ·archFloat32ToReg(SB),NOSPLIT,$0-16
	FMOVS	val+0(FP), F1
	FMOVD	F1, ret+8(FP)
	RET

// Convert uint64->float32
TEXT ·archFloat32FromReg(SB),NOSPLIT,$0-12
	FMOVD	reg+0(FP), F1
	// Normally a float64->float32 conversion
	// would need rounding, but that is not needed
	// here since the uint64 was originally converted
	// from float32, and should be avoided to
	// preserve SNaN values.
	FMOVS	F1, ret+8(FP)
	RET


```

// === FILE: references!/go/src/reflect/float32reg_riscv64.s ===
```text
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// riscv64 allows 32-bit floats to live in the bottom
// part of the register, it expects them to be NaN-boxed.
// These functions are needed to ensure correct conversions
// on riscv64.

// Convert float32->uint64
TEXT ·archFloat32ToReg(SB),NOSPLIT,$0-16
	MOVF	val+0(FP), F1
	MOVD	F1, ret+8(FP)
	RET

// Convert uint64->float32
TEXT ·archFloat32FromReg(SB),NOSPLIT,$0-12
	// Normally a float64->float32 conversion
	// would need rounding, but riscv64 store valid
	// float32 in the lower 32 bits, thus we only need to
	// unboxed the NaN-box by store a float32.
	MOVD	reg+0(FP), F1
	MOVF	F1, ret+8(FP)
	RET


```

// === FILE: references!/go/src/reflect/float32reg_s390x.s ===
```text
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build s390x

#include "textflag.h"

// On s390x, the float32 becomes a float64
// when loaded in a register, different from
// other platforms. These functions are
// needed to ensure correct conversions on s390x.

// Convert float32->uint64
TEXT ·archFloat32ToReg(SB),NOSPLIT,$0-16
       FMOVS   val+0(FP), F1
       FMOVD   F1, ret+8(FP)
       RET

// Convert uint64->float32
TEXT ·archFloat32FromReg(SB),NOSPLIT,$0-12
       FMOVD   reg+0(FP), F1
       // Normally a float64->float32 conversion
       // would need rounding, but that is not needed
       // here since the uint64 was originally converted
       // from float32, and should be avoided to
       // preserve SNaN values.
       FMOVS   F1, ret+8(FP)
       RET


```

// === FILE: references!/go/src/reflect/internal/example1/example.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package example1

type MyStruct struct {
	MyStructs []MyStruct
	MyStruct  *MyStruct
}

```

// === FILE: references!/go/src/reflect/internal/example2/example.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package example2

type MyStruct struct {
	MyStructs []MyStruct
	MyStruct  *MyStruct
}

```

// === FILE: references!/go/src/reflect/iter.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reflect

import (
	"iter"
)

func rangeNum[T int8 | int16 | int32 | int64 | int |
	uint8 | uint16 | uint32 | uint64 | uint |
	uintptr, N int64 | uint64](num N, t Type) iter.Seq[Value] {
	return func(yield func(v Value) bool) {
		convert := t.PkgPath() != ""
		// cannot use range T(v) because no core type.
		for i := T(0); i < T(num); i++ {
			tmp := ValueOf(i)
			// if the iteration value type is define by
			// type T built-in type.
			if convert {
				tmp = tmp.Convert(t)
			}
			if !yield(tmp) {
				return
			}
		}
	}
}

// Seq returns an iter.Seq[Value] that loops over the elements of v.
// If v's kind is [Func], it must be a function that has no results and
// that takes a single argument of type func(T) bool for some type T.
// If v's kind is [Pointer], the pointer element type must have kind [Array].
// Otherwise v's kind must be [Int], [Int8], [Int16], [Int32], [Int64],
// [Uint], [Uint8], [Uint16], [Uint32], [Uint64], [Uintptr],
// [Array], [Chan], [Map], [Slice], or [String].
func (v Value) Seq() iter.Seq[Value] {
	if canRangeFunc(v.abiType(), 1) {
		return func(yield func(Value) bool) {
			rf := MakeFunc(v.Type().In(0), func(in []Value) []Value {
				return []Value{ValueOf(yield(in[0]))}
			})
			v.Call([]Value{rf})
		}
	}
	switch v.kind() {
	case Int:
		return rangeNum[int](v.Int(), v.Type())
	case Int8:
		return rangeNum[int8](v.Int(), v.Type())
	case Int16:
		return rangeNum[int16](v.Int(), v.Type())
	case Int32:
		return rangeNum[int32](v.Int(), v.Type())
	case Int64:
		return rangeNum[int64](v.Int(), v.Type())
	case Uint:
		return rangeNum[uint](v.Uint(), v.Type())
	case Uint8:
		return rangeNum[uint8](v.Uint(), v.Type())
	case Uint16:
		return rangeNum[uint16](v.Uint(), v.Type())
	case Uint32:
		return rangeNum[uint32](v.Uint(), v.Type())
	case Uint64:
		return rangeNum[uint64](v.Uint(), v.Type())
	case Uintptr:
		return rangeNum[uintptr](v.Uint(), v.Type())
	case Pointer:
		if v.Type().Elem().Kind() != Array {
			break
		}
		n := v.Type().Elem().Len()
		return func(yield func(Value) bool) {
			for i := range n {
				if !yield(ValueOf(i)) {
					return
				}
			}
		}
	case Array, Slice:
		return func(yield func(Value) bool) {
			for i := range v.Len() {
				if !yield(ValueOf(i)) {
					return
				}
			}
		}
	case String:
		return func(yield func(Value) bool) {
			for i := range v.String() {
				if !yield(ValueOf(i)) {
					return
				}
			}
		}
	case Map:
		return func(yield func(Value) bool) {
			i := v.MapRange()
			for i.Next() {
				if !yield(i.Key()) {
					return
				}
			}
		}
	case Chan:
		return func(yield func(Value) bool) {
			for value, ok := v.Recv(); ok; value, ok = v.Recv() {
				if !yield(value) {
					return
				}
			}
		}
	}
	panic("reflect: " + v.Type().String() + " cannot produce iter.Seq[Value]")
}

// Seq2 returns an iter.Seq2[Value, Value] that loops over the elements of v.
// If v's kind is [Func], it must be a function that has no results and
// that takes a single argument of type func(K, V) bool for some type K, V.
// If v's kind is [Pointer], the pointer element type must have kind [Array].
// Otherwise v's kind must be [Array], [Map], [Slice], or [String].
func (v Value) Seq2() iter.Seq2[Value, Value] {
	if canRangeFunc(v.abiType(), 2) {
		return func(yield func(Value, Value) bool) {
			rf := MakeFunc(v.Type().In(0), func(in []Value) []Value {
				return []Value{ValueOf(yield(in[0], in[1]))}
			})
			v.Call([]Value{rf})
		}
	}
	switch v.Kind() {
	case Pointer:
		if v.Elem().kind() != Array {
			break
		}
		return func(yield func(Value, Value) bool) {
			v = v.Elem()
			for i := range v.Len() {
				if !yield(ValueOf(i), v.Index(i)) {
					return
				}
			}
		}
	case Array, Slice:
		return func(yield func(Value, Value) bool) {
			for i := range v.Len() {
				if !yield(ValueOf(i), v.Index(i)) {
					return
				}
			}
		}
	case String:
		return func(yield func(Value, Value) bool) {
			for i, v := range v.String() {
				if !yield(ValueOf(i), ValueOf(v)) {
					return
				}
			}
		}
	case Map:
		return func(yield func(Value, Value) bool) {
			i := v.MapRange()
			for i.Next() {
				if !yield(i.Key(), i.Value()) {
					return
				}
			}
		}
	}
	panic("reflect: " + v.Type().String() + " cannot produce iter.Seq2[Value, Value]")
}

```

// === FILE: references!/go/src/reflect/makefunc.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// MakeFunc implementation.

package reflect

import (
	"internal/abi"
	"internal/goarch"
	"unsafe"
)

// makeFuncImpl is the closure value implementing the function
// returned by MakeFunc.
// The first three words of this type must be kept in sync with
// methodValue and runtime.reflectMethodValue.
// Any changes should be reflected in all three.
type makeFuncImpl struct {
	makeFuncCtxt
	ftyp *funcType
	fn   func([]Value) []Value
}

// MakeFunc returns a new function of the given [Type]
// that wraps the function fn. When called, that new function
// does the following:
//
//   - converts its arguments to a slice of Values.
//   - runs results := fn(args).
//   - returns the results as a slice of Values, one per formal result.
//
// The implementation fn can assume that the argument [Value] slice
// has the number and type of arguments given by typ.
// If typ describes a variadic function, the final Value is itself
// a slice representing the variadic arguments, as in the
// body of a variadic function. The result Value slice returned by fn
// must have the number and type of results given by typ.
//
// The [Value.Call] method allows the caller to invoke a typed function
// in terms of Values; in contrast, MakeFunc allows the caller to implement
// a typed function in terms of Values.
//
// The Examples section of the documentation includes an illustration
// of how to use MakeFunc to build a swap function for different types.
func MakeFunc(typ Type, fn func(args []Value) (results []Value)) Value {
	if typ.Kind() != Func {
		panic("reflect: call of MakeFunc with non-Func type")
	}

	t := typ.common()
	ftyp := (*funcType)(unsafe.Pointer(t))

	code := abi.FuncPCABI0(makeFuncStub)

	// makeFuncImpl contains a stack map for use by the runtime
	_, _, abid := funcLayout(ftyp, nil)

	impl := &makeFuncImpl{
		makeFuncCtxt: makeFuncCtxt{
			fn:      code,
			stack:   abid.stackPtrs,
			argLen:  abid.stackCallArgsSize,
			regPtrs: abid.inRegPtrs,
		},
		ftyp: ftyp,
		fn:   fn,
	}

	return Value{t, unsafe.Pointer(impl), flag(Func)}
}

// makeFuncStub is an assembly function that is the code half of
// the function returned from MakeFunc. It expects a *callReflectFunc
// as its context register, and its job is to invoke callReflect(ctxt, frame)
// where ctxt is the context register and frame is a pointer to the first
// word in the passed-in argument frame.
func makeFuncStub()

// The first 3 words of this type must be kept in sync with
// makeFuncImpl and runtime.reflectMethodValue.
// Any changes should be reflected in all three.
type methodValue struct {
	makeFuncCtxt
	method int
	rcvr   Value
}

// makeMethodValue converts v from the rcvr+method index representation
// of a method value to an actual method func value, which is
// basically the receiver value with a special bit set, into a true
// func value - a value holding an actual func. The output is
// semantically equivalent to the input as far as the user of package
// reflect can tell, but the true func representation can be handled
// by code like Convert and Interface and Assign.
func makeMethodValue(op string, v Value) Value {
	if v.flag&flagMethod == 0 {
		panic("reflect: internal error: invalid use of makeMethodValue")
	}

	// Ignoring the flagMethod bit, v describes the receiver, not the method type.
	fl := v.flag & (flagRO | flagAddr | flagIndir)
	fl |= flag(v.typ().Kind())
	rcvr := Value{v.typ(), v.ptr, fl}

	// v.Type returns the actual type of the method value.
	ftyp := (*funcType)(unsafe.Pointer(v.Type().(*rtype)))

	code := methodValueCallCodePtr()

	// methodValue contains a stack map for use by the runtime
	_, _, abid := funcLayout(ftyp, nil)
	fv := &methodValue{
		makeFuncCtxt: makeFuncCtxt{
			fn:      code,
			stack:   abid.stackPtrs,
			argLen:  abid.stackCallArgsSize,
			regPtrs: abid.inRegPtrs,
		},
		method: int(v.flag) >> flagMethodShift,
		rcvr:   rcvr,
	}

	// Cause panic if method is not appropriate.
	// The panic would still happen during the call if we omit this,
	// but we want Interface() and other operations to fail early.
	methodReceiver(op, fv.rcvr, fv.method)

	return Value{ftyp.Common(), unsafe.Pointer(fv), v.flag&flagRO | flag(Func)}
}

func methodValueCallCodePtr() uintptr {
	return abi.FuncPCABI0(methodValueCall)
}

// methodValueCall is an assembly function that is the code half of
// the function returned from makeMethodValue. It expects a *methodValue
// as its context register, and its job is to invoke callMethod(ctxt, frame)
// where ctxt is the context register and frame is a pointer to the first
// word in the passed-in argument frame.
func methodValueCall()

// This structure must be kept in sync with runtime.reflectMethodValue.
// Any changes should be reflected in all both.
type makeFuncCtxt struct {
	fn      uintptr
	stack   *bitVector // ptrmap for both stack args and results
	argLen  uintptr    // just args
	regPtrs abi.IntArgRegBitmap
}

// moveMakeFuncArgPtrs uses ctxt.regPtrs to copy integer pointer arguments
// in args.Ints to args.Ptrs where the GC can see them.
//
// This is similar to what reflectcallmove does in the runtime, except
// that happens on the return path, whereas this happens on the call path.
//
// nosplit because pointers are being held in uintptr slots in args, so
// having our stack scanned now could lead to accidentally freeing
// memory.
//
//go:nosplit
func moveMakeFuncArgPtrs(ctxt *makeFuncCtxt, args *abi.RegArgs) {
	for i, arg := range args.Ints {
		// Avoid write barriers! Because our write barrier enqueues what
		// was there before, we might enqueue garbage.
		// Also avoid bounds checks, we don't have the stack space for it.
		// (Normally the prove pass removes them, but for -N builds we
		// use too much stack.)
		// ptr := &args.Ptrs[i] (but cast from *unsafe.Pointer to *uintptr)
		ptr := (*uintptr)(add(unsafe.Pointer(unsafe.SliceData(args.Ptrs[:])), uintptr(i)*goarch.PtrSize, "always in [0:IntArgRegs]"))
		if ctxt.regPtrs.Get(i) {
			*ptr = arg
		} else {
			// We *must* zero this space ourselves because it's defined in
			// assembly code and the GC will scan these pointers. Otherwise,
			// there will be garbage here.
			*ptr = 0
		}
	}
}

```

// === FILE: references!/go/src/reflect/map.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reflect

import (
	"internal/abi"
	"internal/goexperiment"
	"internal/race"
	"internal/runtime/maps"
	"internal/runtime/sys"
	"unsafe"
)

func (t *rtype) Key() Type {
	if t.Kind() != Map {
		panic("reflect: Key of non-map type " + t.String())
	}
	tt := (*abi.MapType)(unsafe.Pointer(t))
	return toType(tt.Key)
}

// MapOf returns the map type with the given key and element types.
// For example, if k represents int and e represents string,
// MapOf(k, e) represents map[int]string.
//
// If the key type is not a valid map key type (that is, if it does
// not implement Go's == operator), MapOf panics.
func MapOf(key, elem Type) Type {
	ktyp := key.common()
	etyp := elem.common()

	if ktyp.Equal == nil {
		panic("reflect.MapOf: invalid key type " + stringFor(ktyp))
	}

	// Look in cache.
	ckey := cacheKey{Map, ktyp, etyp, 0}
	if mt, ok := lookupCache.Load(ckey); ok {
		return mt.(Type)
	}

	// Look in known types.
	s := "map[" + stringFor(ktyp) + "]" + stringFor(etyp)
	for _, tt := range typesByString(s) {
		mt := (*abi.MapType)(unsafe.Pointer(tt))
		if mt.Key == ktyp && mt.Elem == etyp {
			ti, _ := lookupCache.LoadOrStore(ckey, toRType(tt))
			return ti.(Type)
		}
	}

	group := groupOf(key, elem)

	// Make a map type.
	// Note: flag values must match those used in the TMAP case
	// in ../cmd/compile/internal/reflectdata/reflect.go:writeType.
	var imap any = (map[unsafe.Pointer]unsafe.Pointer)(nil)
	mt := **(**abi.MapType)(unsafe.Pointer(&imap))
	mt.Str = resolveReflectName(newName(s, "", false, false))
	mt.TFlag = abi.TFlagDirectIface
	mt.Hash = fnv1(etyp.Hash, 'm', byte(ktyp.Hash>>24), byte(ktyp.Hash>>16), byte(ktyp.Hash>>8), byte(ktyp.Hash))
	mt.Key = ktyp
	mt.Elem = etyp
	mt.Group = group.common()
	mt.Hasher = func(p unsafe.Pointer, seed uintptr) uintptr {
		return typehash(ktyp, p, seed)
	}
	mt.GroupSize = mt.Group.Size()
	if goexperiment.MapSplitGroup {
		// Split layout: field 1 is keys array, field 2 is elems array.
		mt.KeysOff = group.Field(1).Offset
		mt.KeyStride = group.Field(1).Type.Elem().Size()
		mt.ElemsOff = group.Field(2).Offset
		mt.ElemStride = group.Field(2).Type.Elem().Size()
		mt.ElemOff = 0
	} else {
		// Interleaved layout: field 1 is slots array.
		// KeyStride = ElemStride = slot stride.
		// ElemsOff = slots offset + elem offset within slot.
		slot := group.Field(1).Type.Elem()
		slotSize := slot.Size()
		mt.KeysOff = group.Field(1).Offset
		mt.KeyStride = slotSize
		mt.ElemsOff = group.Field(1).Offset + slot.Field(1).Offset
		mt.ElemStride = slotSize
		mt.ElemOff = slot.Field(1).Offset
	}
	mt.Flags = 0
	if needKeyUpdate(ktyp) {
		mt.Flags |= abi.MapNeedKeyUpdate
	}
	if hashMightPanic(ktyp) {
		mt.Flags |= abi.MapHashMightPanic
	}
	if ktyp.Size_ > abi.MapMaxKeyBytes {
		mt.Flags |= abi.MapIndirectKey
	}
	if etyp.Size_ > abi.MapMaxElemBytes {
		mt.Flags |= abi.MapIndirectElem
	}
	mt.PtrToThis = 0

	ti, _ := lookupCache.LoadOrStore(ckey, toRType(&mt.Type))
	return ti.(Type)
}

func groupOf(ktyp, etyp Type) Type {
	if ktyp.Size() > abi.MapMaxKeyBytes {
		ktyp = PointerTo(ktyp)
	}
	if etyp.Size() > abi.MapMaxElemBytes {
		etyp = PointerTo(etyp)
	}

	if goexperiment.MapSplitGroup {
		// Split layout (KKKKVVVV):
		// type group struct {
		//     ctrl  uint64
		//     keys  [abi.MapGroupSlots]keyType
		//     elems [abi.MapGroupSlots]elemType
		// }
		fields := []StructField{
			{
				Name: "Ctrl",
				Type: TypeFor[uint64](),
			},
			{
				Name: "Keys",
				Type: ArrayOf(abi.MapGroupSlots, ktyp),
			},
			{
				Name: "Elems",
				Type: ArrayOf(abi.MapGroupSlots, etyp),
			},
		}
		return StructOf(fields)
	}

	// Interleaved slot layout (KVKVKVKV):
	// type group struct {
	//     ctrl  uint64
	//     slots [abi.MapGroupSlots]struct {
	//         key  keyType
	//         elem elemType
	//     }
	// }
	slotFields := []StructField{
		{
			Name: "Key",
			Type: ktyp,
		},
		{
			Name: "Elem",
			Type: etyp,
		},
	}
	slot := StructOf(slotFields)

	fields := []StructField{
		{
			Name: "Ctrl",
			Type: TypeFor[uint64](),
		},
		{
			Name: "Slots",
			Type: ArrayOf(abi.MapGroupSlots, slot),
		},
	}
	return StructOf(fields)
}

var stringType = rtypeOf("")

// MapIndex returns the value associated with key in the map v.
// It panics if v's Kind is not [Map].
// It returns the zero Value if key is not found in the map or if v represents a nil map.
// As in Go, the key's value must be assignable to the map's key type.
func (v Value) MapIndex(key Value) Value {
	v.mustBe(Map)
	tt := (*abi.MapType)(unsafe.Pointer(v.typ()))

	// Do not require key to be exported, so that DeepEqual
	// and other programs can use all the keys returned by
	// MapKeys as arguments to MapIndex. If either the map
	// or the key is unexported, though, the result will be
	// considered unexported. This is consistent with the
	// behavior for structs, which allow read but not write
	// of unexported fields.

	var e unsafe.Pointer
	if (tt.Key == stringType || key.kind() == String) && tt.Key == key.typ() && tt.Elem.Size() <= abi.MapMaxElemBytes {
		k := *(*string)(key.ptr)
		e = mapaccess_faststr(v.typ(), v.pointer(), k)
	} else {
		key = key.assignTo("reflect.Value.MapIndex", tt.Key, nil)
		var k unsafe.Pointer
		if key.flag&flagIndir != 0 {
			k = key.ptr
		} else {
			k = unsafe.Pointer(&key.ptr)
		}
		e = mapaccess(v.typ(), v.pointer(), k)
	}
	if e == nil {
		return Value{}
	}
	typ := tt.Elem
	fl := (v.flag | key.flag).ro()
	fl |= flag(typ.Kind())
	return copyVal(typ, fl, e)
}

// Equivalent to runtime.mapIterStart.
//
//go:noinline
func mapIterStart(t *abi.MapType, m *maps.Map, it *maps.Iter) {
	if race.Enabled && m != nil {
		callerpc := sys.GetCallerPC()
		race.ReadPC(unsafe.Pointer(m), callerpc, abi.FuncPCABIInternal(mapIterStart))
	}

	it.Init(t, m)
	it.Next()
}

// Equivalent to runtime.mapIterNext.
//
//go:noinline
func mapIterNext(it *maps.Iter) {
	if race.Enabled {
		callerpc := sys.GetCallerPC()
		race.ReadPC(unsafe.Pointer(it.Map()), callerpc, abi.FuncPCABIInternal(mapIterNext))
	}

	it.Next()
}

// MapKeys returns a slice containing all the keys present in the map,
// in unspecified order.
// It panics if v's Kind is not [Map].
// It returns an empty slice if v represents a nil map.
func (v Value) MapKeys() []Value {
	v.mustBe(Map)
	tt := (*abi.MapType)(unsafe.Pointer(v.typ()))
	keyType := tt.Key

	fl := v.flag.ro() | flag(keyType.Kind())

	// Escape analysis can't see that the map doesn't escape. It sees an
	// escape from maps.IterStart, via assignment into it, even though it
	// doesn't escape this function.
	mptr := abi.NoEscape(v.pointer())
	m := (*maps.Map)(mptr)
	mlen := int(0)
	if m != nil {
		mlen = maplen(mptr)
	}
	var it maps.Iter
	mapIterStart(tt, m, &it)
	a := make([]Value, mlen)
	var i int
	for i = 0; i < len(a); i++ {
		key := it.Key()
		if key == nil {
			// Someone deleted an entry from the map since we
			// called maplen above. It's a data race, but nothing
			// we can do about it.
			break
		}
		a[i] = copyVal(keyType, fl, key)
		mapIterNext(&it)
	}
	return a[:i]
}

// A MapIter is an iterator for ranging over a map.
// See [Value.MapRange].
type MapIter struct {
	m     Value
	hiter maps.Iter
}

// Key returns the key of iter's current map entry.
func (iter *MapIter) Key() Value {
	if !iter.hiter.Initialized() {
		panic("MapIter.Key called before Next")
	}
	iterkey := iter.hiter.Key()
	if iterkey == nil {
		panic("MapIter.Key called on exhausted iterator")
	}

	t := (*abi.MapType)(unsafe.Pointer(iter.m.typ()))
	ktype := t.Key
	return copyVal(ktype, iter.m.flag.ro()|flag(ktype.Kind()), iterkey)
}

// SetIterKey assigns to v the key of iter's current map entry.
// It is equivalent to v.Set(iter.Key()), but it avoids allocating a new Value.
// As in Go, the key must be assignable to v's type and
// must not be derived from an unexported field.
// It panics if [Value.CanSet] returns false.
func (v Value) SetIterKey(iter *MapIter) {
	if !iter.hiter.Initialized() {
		panic("reflect: Value.SetIterKey called before Next")
	}
	iterkey := iter.hiter.Key()
	if iterkey == nil {
		panic("reflect: Value.SetIterKey called on exhausted iterator")
	}

	v.mustBeAssignable()
	var target unsafe.Pointer
	if v.kind() == Interface {
		target = v.ptr
	}

	t := (*abi.MapType)(unsafe.Pointer(iter.m.typ()))
	ktype := t.Key

	iter.m.mustBeExported() // do not let unexported m leak
	key := Value{ktype, iterkey, iter.m.flag | flag(ktype.Kind()) | flagIndir}
	key = key.assignTo("reflect.MapIter.SetKey", v.typ(), target)
	typedmemmove(v.typ(), v.ptr, key.ptr)
}

// Value returns the value of iter's current map entry.
func (iter *MapIter) Value() Value {
	if !iter.hiter.Initialized() {
		panic("MapIter.Value called before Next")
	}
	iterelem := iter.hiter.Elem()
	if iterelem == nil {
		panic("MapIter.Value called on exhausted iterator")
	}

	t := (*abi.MapType)(unsafe.Pointer(iter.m.typ()))
	vtype := t.Elem
	return copyVal(vtype, iter.m.flag.ro()|flag(vtype.Kind()), iterelem)
}

// SetIterValue assigns to v the value of iter's current map entry.
// It is equivalent to v.Set(iter.Value()), but it avoids allocating a new Value.
// As in Go, the value must be assignable to v's type and
// must not be derived from an unexported field.
// It panics if [Value.CanSet] returns false.
func (v Value) SetIterValue(iter *MapIter) {
	if !iter.hiter.Initialized() {
		panic("reflect: Value.SetIterValue called before Next")
	}
	iterelem := iter.hiter.Elem()
	if iterelem == nil {
		panic("reflect: Value.SetIterValue called on exhausted iterator")
	}

	v.mustBeAssignable()
	var target unsafe.Pointer
	if v.kind() == Interface {
		target = v.ptr
	}

	t := (*abi.MapType)(unsafe.Pointer(iter.m.typ()))
	vtype := t.Elem

	iter.m.mustBeExported() // do not let unexported m leak
	elem := Value{vtype, iterelem, iter.m.flag | flag(vtype.Kind()) | flagIndir}
	elem = elem.assignTo("reflect.MapIter.SetValue", v.typ(), target)
	typedmemmove(v.typ(), v.ptr, elem.ptr)
}

// Next advances the map iterator and reports whether there is another
// entry. It returns false when iter is exhausted; subsequent
// calls to [MapIter.Key], [MapIter.Value], or [MapIter.Next] will panic.
func (iter *MapIter) Next() bool {
	if !iter.m.IsValid() {
		panic("MapIter.Next called on an iterator that does not have an associated map Value")
	}
	if !iter.hiter.Initialized() {
		t := (*abi.MapType)(unsafe.Pointer(iter.m.typ()))
		m := (*maps.Map)(iter.m.pointer())
		mapIterStart(t, m, &iter.hiter)
	} else {
		if iter.hiter.Key() == nil {
			panic("MapIter.Next called on exhausted iterator")
		}
		mapIterNext(&iter.hiter)
	}
	return iter.hiter.Key() != nil
}

// Reset modifies iter to iterate over v.
// It panics if v's Kind is not [Map] and v is not the zero Value.
// Reset(Value{}) causes iter to not to refer to any map,
// which may allow the previously iterated-over map to be garbage collected.
func (iter *MapIter) Reset(v Value) {
	if v.IsValid() {
		v.mustBe(Map)
	}
	iter.m = v
	iter.hiter = maps.Iter{}
}

// MapRange returns a range iterator for a map.
// It panics if v's Kind is not [Map].
//
// Call [MapIter.Next] to advance the iterator, and [MapIter.Key]/[MapIter.Value] to access each entry.
// [MapIter.Next] returns false when the iterator is exhausted.
// MapRange follows the same iteration semantics as a range statement.
//
// Example:
//
//	iter := reflect.ValueOf(m).MapRange()
//	for iter.Next() {
//		k := iter.Key()
//		v := iter.Value()
//		...
//	}
func (v Value) MapRange() *MapIter {
	// This is inlinable to take advantage of "function outlining".
	// The allocation of MapIter can be stack allocated if the caller
	// does not allow it to escape.
	// See https://blog.filippo.io/efficient-go-apis-with-the-inliner/
	if v.kind() != Map {
		v.panicNotMap()
	}
	return &MapIter{m: v}
}

// SetMapIndex sets the element associated with key in the map v to elem.
// It panics if v's Kind is not [Map].
// If elem is the zero Value, SetMapIndex deletes the key from the map.
// Otherwise if v holds a nil map, SetMapIndex will panic.
// As in Go, key's elem must be assignable to the map's key type,
// and elem's value must be assignable to the map's elem type.
func (v Value) SetMapIndex(key, elem Value) {
	v.mustBe(Map)
	v.mustBeExported()
	key.mustBeExported()
	tt := (*abi.MapType)(unsafe.Pointer(v.typ()))

	if (tt.Key == stringType || key.kind() == String) && tt.Key == key.typ() && tt.Elem.Size() <= abi.MapMaxElemBytes {
		k := *(*string)(key.ptr)
		if elem.typ() == nil {
			mapdelete_faststr(v.typ(), v.pointer(), k)
			return
		}
		elem.mustBeExported()
		elem = elem.assignTo("reflect.Value.SetMapIndex", tt.Elem, nil)
		var e unsafe.Pointer
		if elem.flag&flagIndir != 0 {
			e = elem.ptr
		} else {
			e = unsafe.Pointer(&elem.ptr)
		}
		mapassign_faststr(v.typ(), v.pointer(), k, e)
		return
	}

	key = key.assignTo("reflect.Value.SetMapIndex", tt.Key, nil)
	var k unsafe.Pointer
	if key.flag&flagIndir != 0 {
		k = key.ptr
	} else {
		k = unsafe.Pointer(&key.ptr)
	}
	if elem.typ() == nil {
		mapdelete(v.typ(), v.pointer(), k)
		return
	}
	elem.mustBeExported()
	elem = elem.assignTo("reflect.Value.SetMapIndex", tt.Elem, nil)
	var e unsafe.Pointer
	if elem.flag&flagIndir != 0 {
		e = elem.ptr
	} else {
		e = unsafe.Pointer(&elem.ptr)
	}
	mapassign(v.typ(), v.pointer(), k, e)
}

// Force slow panicking path not inlined, so it won't add to the
// inlining budget of the caller.
// TODO: undo when the inliner is no longer bottom-up only.
//
//go:noinline
func (f flag) panicNotMap() {
	f.mustBe(Map)
}

```

// === FILE: references!/go/src/reflect/stubs_ppc64x.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ppc64le || ppc64

package reflect

func archFloat32FromReg(reg uint64) float32
func archFloat32ToReg(val float32) uint64

```

// === FILE: references!/go/src/reflect/stubs_riscv64.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reflect

func archFloat32FromReg(reg uint64) float32
func archFloat32ToReg(val float32) uint64

```

// === FILE: references!/go/src/reflect/stubs_s390x.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build s390x

package reflect

func archFloat32FromReg(reg uint64) float32
func archFloat32ToReg(val float32) uint64

```

// === FILE: references!/go/src/reflect/swapper.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reflect

import (
	"internal/abi"
	"internal/goarch"
	"internal/unsafeheader"
	"unsafe"
)

// Swapper returns a function that swaps the elements in the provided
// slice.
//
// Swapper panics if the provided interface is not a slice.
func Swapper(slice any) func(i, j int) {
	v := ValueOf(slice)
	if v.Kind() != Slice {
		panic(&ValueError{Method: "Swapper", Kind: v.Kind()})
	}
	// Fast path for slices of size 0 and 1. Nothing to swap.
	switch v.Len() {
	case 0:
		return func(i, j int) { panic("reflect: slice index out of range") }
	case 1:
		return func(i, j int) {
			if i != 0 || j != 0 {
				panic("reflect: slice index out of range")
			}
		}
	}

	typ := v.Type().Elem().common()
	size := typ.Size()
	hasPtr := typ.Pointers()

	// Some common & small cases, without using memmove:
	if hasPtr {
		if size == goarch.PtrSize {
			ps := *(*[]unsafe.Pointer)(v.ptr)
			return func(i, j int) { ps[i], ps[j] = ps[j], ps[i] }
		}
		if typ.Kind() == abi.String {
			ss := *(*[]string)(v.ptr)
			return func(i, j int) { ss[i], ss[j] = ss[j], ss[i] }
		}
	} else {
		switch size {
		case 8:
			is := *(*[]int64)(v.ptr)
			return func(i, j int) { is[i], is[j] = is[j], is[i] }
		case 4:
			is := *(*[]int32)(v.ptr)
			return func(i, j int) { is[i], is[j] = is[j], is[i] }
		case 2:
			is := *(*[]int16)(v.ptr)
			return func(i, j int) { is[i], is[j] = is[j], is[i] }
		case 1:
			is := *(*[]int8)(v.ptr)
			return func(i, j int) { is[i], is[j] = is[j], is[i] }
		}
	}

	s := (*unsafeheader.Slice)(v.ptr)
	tmp := unsafe_New(typ) // swap scratch space

	return func(i, j int) {
		if uint(i) >= uint(s.Len) || uint(j) >= uint(s.Len) {
			panic("reflect: slice index out of range")
		}
		val1 := arrayAt(s.Data, i, size, "i < s.Len")
		val2 := arrayAt(s.Data, j, size, "j < s.Len")
		typedmemmove(typ, tmp, val1)
		typedmemmove(typ, val1, val2)
		typedmemmove(typ, val2, tmp)
	}
}

```

// === FILE: references!/go/src/reflect/type.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package reflect implements run-time reflection, allowing a program to
// manipulate objects with arbitrary types. The typical use is to take a value
// with static type interface{} and extract its dynamic type information by
// calling TypeOf, which returns a Type.
//
// A call to ValueOf returns a Value representing the run-time data.
// Zero takes a Type and returns a Value representing a zero value
// for that type.
//
// See "The Laws of Reflection" for an introduction to reflection in Go:
// https://golang.org/doc/articles/laws_of_reflection.html
package reflect

import (
	"internal/abi"
	"internal/goarch"
	"iter"
	"runtime"
	"strconv"
	"sync"
	"unicode"
	"unicode/utf8"
	"unsafe"
)

// Type is the representation of a Go type.
//
// Not all methods apply to all kinds of types. Restrictions,
// if any, are noted in the documentation for each method.
// Use the Kind method to find out the kind of type before
// calling kind-specific methods. Calling a method
// inappropriate to the kind of type causes a run-time panic.
//
// Type values are comparable, such as with the == operator,
// so they can be used as map keys.
// Two Type values are equal if they represent identical types.
type Type interface {
	// Methods applicable to all types.

	// Align returns the alignment in bytes of a value of
	// this type when allocated in memory.
	Align() int

	// FieldAlign returns the alignment in bytes of a value of
	// this type when used as a field in a struct.
	FieldAlign() int

	// Method returns the i'th method in the type's method set.
	// It panics if i is not in the range [0, NumMethod()).
	//
	// For a non-interface type T or *T, the returned Method's Type and Func
	// fields describe a function whose first argument is the receiver,
	// and only exported methods are accessible.
	//
	// For an interface type, the returned Method's Type field gives the
	// method signature, without a receiver, and the Func field is nil.
	//
	// Methods are sorted in lexicographic order.
	//
	// Calling this method will force the linker to retain all exported methods in all packages.
	// This may make the executable binary larger but will not affect execution time.
	Method(int) Method

	// Methods returns an iterator over each method in the type's method set. The sequence is
	// equivalent to calling Method successively for each index i in the range [0, NumMethod()).
	Methods() iter.Seq[Method]

	// MethodByName returns the method with that name in the type's
	// method set and a boolean indicating if the method was found.
	//
	// For a non-interface type T or *T, the returned Method's Type and Func
	// fields describe a function whose first argument is the receiver.
	//
	// For an interface type, the returned Method's Type field gives the
	// method signature, without a receiver, and the Func field is nil.
	//
	// Calling this method will cause the linker to retain all methods with this name in all packages.
	// If the linker can't determine the name, it will retain all exported methods.
	// This may make the executable binary larger but will not affect execution time.
	MethodByName(string) (Method, bool)

	// NumMethod returns the number of methods accessible using Method.
	//
	// For a non-interface type, it returns the number of exported methods.
	//
	// For an interface type, it returns the number of exported and unexported methods.
	NumMethod() int

	// Name returns the type's name within its package for a defined type.
	// For other (non-defined) types it returns the empty string.
	Name() string

	// PkgPath returns a defined type's package path, that is, the import path
	// that uniquely identifies the package, such as "encoding/base64".
	// If the type was predeclared (string, error) or not defined (*T, struct{},
	// []int, or A where A is an alias for a non-defined type), the package path
	// will be the empty string.
	PkgPath() string

	// Size returns the number of bytes needed to store
	// a value of the given type; it is analogous to unsafe.Sizeof.
	Size() uintptr

	// String returns a string representation of the type.
	// The string representation may use shortened package names
	// (e.g., base64 instead of "encoding/base64") and is not
	// guaranteed to be unique among types. To test for type identity,
	// compare the Types directly.
	String() string

	// Kind returns the specific kind of this type.
	Kind() Kind

	// Implements reports whether the type implements the interface type u.
	Implements(u Type) bool

	// AssignableTo reports whether a value of the type is assignable to type u.
	AssignableTo(u Type) bool

	// ConvertibleTo reports whether a value of the type is convertible to type u.
	// Even if ConvertibleTo returns true, the conversion may still panic.
	// For example, a slice of type []T is convertible to *[N]T,
	// but the conversion will panic if its length is less than N.
	ConvertibleTo(u Type) bool

	// Comparable reports whether values of this type are comparable.
	// Even if Comparable returns true, the comparison may still panic.
	// For example, values of interface type are comparable,
	// but the comparison will panic if their dynamic type is not comparable.
	Comparable() bool

	// Methods applicable only to some types, depending on Kind.
	// The methods allowed for each kind are:
	//
	//	Int*, Uint*, Float*, Complex*: Bits
	//	Array: Elem, Len
	//	Chan: ChanDir, Elem
	//	Func: In, NumIn, Out, NumOut, IsVariadic.
	//	Map: Key, Elem
	//	Pointer: Elem
	//	Slice: Elem
	//	Struct: Field, FieldByIndex, FieldByName, FieldByNameFunc, NumField

	// Bits returns the size of the type in bits.
	// It panics if the type's Kind is not one of the
	// sized or unsized Int, Uint, Float, or Complex kinds.
	Bits() int

	// ChanDir returns a channel type's direction.
	// It panics if the type's Kind is not Chan.
	ChanDir() ChanDir

	// IsVariadic reports whether a function type's final input parameter
	// is a "..." parameter. If so, t.In(t.NumIn() - 1) returns the parameter's
	// implicit actual type []T.
	//
	// For concreteness, if t represents func(x int, y ... float64), then
	//
	//	t.NumIn() == 2
	//	t.In(0) is the reflect.Type for "int"
	//	t.In(1) is the reflect.Type for "[]float64"
	//	t.IsVariadic() == true
	//
	// IsVariadic panics if the type's Kind is not Func.
	IsVariadic() bool

	// Elem returns a type's element type.
	// It panics if the type's Kind is not Array, Chan, Map, Pointer, or Slice.
	Elem() Type

	// Field returns a struct type's i'th field.
	// It panics if the type's Kind is not Struct.
	// It panics if i is not in the range [0, NumField()).
	Field(i int) StructField

	// Fields returns an iterator over each struct field for struct type t. The sequence is
	// equivalent to calling Field successively for each index i in the range [0, NumField()).
	// It panics if the type's Kind is not Struct.
	Fields() iter.Seq[StructField]

	// FieldByIndex returns the nested field corresponding
	// to the index sequence. It is equivalent to calling Field
	// successively for each index i.
	// It panics if the type's Kind is not Struct.
	FieldByIndex(index []int) StructField

	// FieldByName returns the struct field with the given name
	// and a boolean indicating if the field was found.
	// If the returned field is promoted from an embedded struct,
	// then Offset in the returned StructField is the offset in
	// the embedded struct.
	FieldByName(name string) (StructField, bool)

	// FieldByNameFunc returns the struct field with a name
	// that satisfies the match function and a boolean indicating if
	// the field was found.
	//
	// FieldByNameFunc considers the fields in the struct itself
	// and then the fields in any embedded structs, in breadth first order,
	// stopping at the shallowest nesting depth containing one or more
	// fields satisfying the match function. If multiple fields at that depth
	// satisfy the match function, they cancel each other
	// and FieldByNameFunc returns no match.
	// This behavior mirrors Go's handling of name lookup in
	// structs containing embedded fields.
	//
	// If the returned field is promoted from an embedded struct,
	// then Offset in the returned StructField is the offset in
	// the embedded struct.
	FieldByNameFunc(match func(string) bool) (StructField, bool)

	// In returns the type of a function type's i'th input parameter.
	// It panics if the type's Kind is not Func.
	// It panics if i is not in the range [0, NumIn()).
	In(i int) Type

	// Ins returns an iterator over each input parameter of function type t. The sequence
	// is equivalent to calling In successively for each index i in the range [0, NumIn()).
	// It panics if the type's Kind is not Func.
	Ins() iter.Seq[Type]

	// Key returns a map type's key type.
	// It panics if the type's Kind is not Map.
	Key() Type

	// Len returns an array type's length.
	// It panics if the type's Kind is not Array.
	Len() int

	// NumField returns a struct type's field count.
	// It panics if the type's Kind is not Struct.
	NumField() int

	// NumIn returns a function type's input parameter count.
	// It panics if the type's Kind is not Func.
	NumIn() int

	// NumOut returns a function type's output parameter count.
	// It panics if the type's Kind is not Func.
	NumOut() int

	// Out returns the type of a function type's i'th output parameter.
	// It panics if the type's Kind is not Func.
	// It panics if i is not in the range [0, NumOut()).
	Out(i int) Type

	// Outs returns an iterator over each output parameter of function type t. The sequence
	// is equivalent to calling Out successively for each index i in the range [0, NumOut()).
	// It panics if the type's Kind is not Func.
	Outs() iter.Seq[Type]

	// OverflowComplex reports whether the complex128 x cannot be represented by type t.
	// It panics if t's Kind is not Complex64 or Complex128.
	OverflowComplex(x complex128) bool

	// OverflowFloat reports whether the float64 x cannot be represented by type t.
	// It panics if t's Kind is not Float32 or Float64.
	OverflowFloat(x float64) bool

	// OverflowInt reports whether the int64 x cannot be represented by type t.
	// It panics if t's Kind is not Int, Int8, Int16, Int32, or Int64.
	OverflowInt(x int64) bool

	// OverflowUint reports whether the uint64 x cannot be represented by type t.
	// It panics if t's Kind is not Uint, Uintptr, Uint8, Uint16, Uint32, or Uint64.
	OverflowUint(x uint64) bool

	// CanSeq reports whether a [Value] with this type can be iterated over using [Value.Seq].
	CanSeq() bool

	// CanSeq2 reports whether a [Value] with this type can be iterated over using [Value.Seq2].
	CanSeq2() bool

	common() *abi.Type
	uncommon() *uncommonType
}

// BUG(rsc): FieldByName and related functions consider struct field names to be equal
// if the names are equal, even if they are unexported names originating
// in different packages. The practical effect of this is that the result of
// t.FieldByName("x") is not well defined if the struct type t contains
// multiple fields named x (embedded from different packages).
// FieldByName may return one of the fields named x or may report that there are none.
// See https://golang.org/issue/4876 for more details.

/*
 * These data structures are known to the compiler (../cmd/compile/internal/reflectdata/reflect.go).
 * A few are known to ../runtime/type.go to convey to debuggers.
 * They are also known to ../internal/abi/type.go.
 */

// A Kind represents the specific kind of type that a [Type] represents.
// The zero Kind is not a valid kind.
type Kind uint

const (
	Invalid Kind = iota
	Bool
	Int
	Int8
	Int16
	Int32
	Int64
	Uint
	Uint8
	Uint16
	Uint32
	Uint64
	Uintptr
	Float32
	Float64
	Complex64
	Complex128
	Array
	Chan
	Func
	Interface
	Map
	Pointer
	Slice
	String
	Struct
	UnsafePointer
)

// Ptr is the old name for the [Pointer] kind.
//
//go:fix inline
const Ptr = Pointer

// uncommonType is present only for defined types or types with methods
// (if T is a defined type, the uncommonTypes for T and *T have methods).
// When present, the uncommonType struct immediately follows the
// abi.Type struct in memory.
// The abi.TFlagUncommon indicates the presence of uncommonType.
// Using an optional struct reduces the overall size required
// to describe a non-defined type with no methods.
type uncommonType = abi.UncommonType

// Embed this type to get common/uncommon
type common struct {
	abi.Type
}

// rtype is the common implementation of most values.
// It is embedded in other struct types.
type rtype struct {
	t abi.Type
}

func (t *rtype) common() *abi.Type {
	return &t.t
}

func (t *rtype) uncommon() *abi.UncommonType {
	return t.t.Uncommon()
}

type aNameOff = abi.NameOff
type aTypeOff = abi.TypeOff
type aTextOff = abi.TextOff

// ChanDir represents a channel type's direction.
type ChanDir int

const (
	RecvDir ChanDir             = 1 << iota // <-chan
	SendDir                                 // chan<-
	BothDir = RecvDir | SendDir             // chan
)

// arrayType represents a fixed array type.
type arrayType = abi.ArrayType

// chanType represents a channel type.
type chanType = abi.ChanType

// funcType represents a function type.
//
// A *rtype for each in and out parameter is stored in an array that
// directly follows the funcType (and possibly its uncommonType). So
// a function type with one method, one input, and one output is:
//
//	struct {
//		funcType
//		uncommonType
//		[2]*rtype    // [0] is in, [1] is out
//	}
type funcType = abi.FuncType

// interfaceType represents an interface type.
type interfaceType struct {
	abi.InterfaceType // can embed directly because not a public type.
}

func (t *interfaceType) nameOff(off aNameOff) abi.Name {
	return toRType(&t.Type).nameOff(off)
}

func nameOffFor(t *abi.Type, off aNameOff) abi.Name {
	return toRType(t).nameOff(off)
}

func typeOffFor(t *abi.Type, off aTypeOff) *abi.Type {
	return toRType(t).typeOff(off)
}

func (t *interfaceType) typeOff(off aTypeOff) *abi.Type {
	return toRType(&t.Type).typeOff(off)
}

func (t *interfaceType) common() *abi.Type {
	return &t.Type
}

func (t *interfaceType) uncommon() *abi.UncommonType {
	return t.Uncommon()
}

// ptrType represents a pointer type.
type ptrType struct {
	abi.PtrType
}

// sliceType represents a slice type.
type sliceType struct {
	abi.SliceType
}

// Struct field
type structField = abi.StructField

// structType represents a struct type.
type structType struct {
	abi.StructType
}

func pkgPath(n abi.Name) string {
	if n.Bytes == nil || *n.DataChecked(0, "name flag field")&(1<<2) == 0 {
		return ""
	}
	i, l := n.ReadVarint(1)
	off := 1 + i + l
	if n.HasTag() {
		i2, l2 := n.ReadVarint(off)
		off += i2 + l2
	}
	var nameOff int32
	// Note that this field may not be aligned in memory,
	// so we cannot use a direct int32 assignment here.
	copy((*[4]byte)(unsafe.Pointer(&nameOff))[:], (*[4]byte)(unsafe.Pointer(n.DataChecked(off, "name offset field")))[:])
	pkgPathName := abi.Name{Bytes: (*byte)(resolveTypeOff(unsafe.Pointer(n.Bytes), nameOff))}
	return pkgPathName.Name()
}

func newName(n, tag string, exported, embedded bool) abi.Name {
	return abi.NewName(n, tag, exported, embedded)
}

/*
 * The compiler knows the exact layout of all the data structures above.
 * The compiler does not know about the data structures and methods below.
 */

// Method represents a single method.
type Method struct {
	// Name is the method name.
	Name string

	// PkgPath is the package path that qualifies a lower case (unexported)
	// method name. It is empty for upper case (exported) method names.
	// The combination of PkgPath and Name uniquely identifies a method
	// in a method set.
	// See https://golang.org/ref/spec#Uniqueness_of_identifiers
	PkgPath string

	Type  Type  // method type
	Func  Value // func with receiver as first argument
	Index int   // index for Type.Method
}

// IsExported reports whether the method is exported.
func (m Method) IsExported() bool {
	return m.PkgPath == ""
}

// String returns the name of k.
func (k Kind) String() string {
	if uint(k) < uint(len(kindNames)) {
		return kindNames[uint(k)]
	}
	return "kind" + strconv.Itoa(int(k))
}

var kindNames = []string{
	Invalid:       "invalid",
	Bool:          "bool",
	Int:           "int",
	Int8:          "int8",
	Int16:         "int16",
	Int32:         "int32",
	Int64:         "int64",
	Uint:          "uint",
	Uint8:         "uint8",
	Uint16:        "uint16",
	Uint32:        "uint32",
	Uint64:        "uint64",
	Uintptr:       "uintptr",
	Float32:       "float32",
	Float64:       "float64",
	Complex64:     "complex64",
	Complex128:    "complex128",
	Array:         "array",
	Chan:          "chan",
	Func:          "func",
	Interface:     "interface",
	Map:           "map",
	Pointer:       "ptr",
	Slice:         "slice",
	String:        "string",
	Struct:        "struct",
	UnsafePointer: "unsafe.Pointer",
}

// resolveNameOff resolves a name offset from a base pointer.
// The (*rtype).nameOff method is a convenience wrapper for this function.
// Implemented in the runtime package.
//
//go:noescape
func resolveNameOff(ptrInModule unsafe.Pointer, off int32) unsafe.Pointer

// resolveTypeOff resolves an *rtype offset from a base type.
// The (*rtype).typeOff method is a convenience wrapper for this function.
// Implemented in the runtime package.
//
//go:noescape
func resolveTypeOff(rtype unsafe.Pointer, off int32) unsafe.Pointer

// resolveTextOff resolves a function pointer offset from a base type.
// The (*rtype).textOff method is a convenience wrapper for this function.
// Implemented in the runtime package.
//
//go:noescape
func resolveTextOff(rtype unsafe.Pointer, off int32) unsafe.Pointer

// addReflectOff adds a pointer to the reflection lookup map in the runtime.
// It returns a new ID that can be used as a typeOff or textOff, and will
// be resolved correctly. Implemented in the runtime package.
//
// addReflectOff should be an internal detail,
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/goplus/reflectx
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
//go:linkname addReflectOff
//go:noescape
func addReflectOff(ptr unsafe.Pointer) int32

// resolveReflectName adds a name to the reflection lookup map in the runtime.
// It returns a new nameOff that can be used to refer to the pointer.
func resolveReflectName(n abi.Name) aNameOff {
	return aNameOff(addReflectOff(unsafe.Pointer(n.Bytes)))
}

// resolveReflectType adds a *rtype to the reflection lookup map in the runtime.
// It returns a new typeOff that can be used to refer to the pointer.
func resolveReflectType(t *abi.Type) aTypeOff {
	return aTypeOff(addReflectOff(unsafe.Pointer(t)))
}

// resolveReflectText adds a function pointer to the reflection lookup map in
// the runtime. It returns a new textOff that can be used to refer to the
// pointer.
func resolveReflectText(ptr unsafe.Pointer) aTextOff {
	return aTextOff(addReflectOff(ptr))
}

func (t *rtype) nameOff(off aNameOff) abi.Name {
	return abi.Name{Bytes: (*byte)(resolveNameOff(unsafe.Pointer(t), int32(off)))}
}

func (t *rtype) typeOff(off aTypeOff) *abi.Type {
	return (*abi.Type)(resolveTypeOff(unsafe.Pointer(t), int32(off)))
}

func (t *rtype) textOff(off aTextOff) unsafe.Pointer {
	return resolveTextOff(unsafe.Pointer(t), int32(off))
}

func textOffFor(t *abi.Type, off aTextOff) unsafe.Pointer {
	return toRType(t).textOff(off)
}

func (t *rtype) String() string {
	s := t.nameOff(t.t.Str).Name()
	if t.t.TFlag&abi.TFlagExtraStar != 0 {
		return s[1:]
	}
	return s
}

func (t *rtype) Size() uintptr { return t.t.Size() }

func (t *rtype) Bits() int {
	if t == nil {
		panic("reflect: Bits of nil Type")
	}
	k := t.Kind()
	if k < Int || k > Complex128 {
		panic("reflect: Bits of non-arithmetic Type " + t.String())
	}
	return int(t.t.Size_) * 8
}

func (t *rtype) Align() int { return t.t.Align() }

func (t *rtype) FieldAlign() int { return t.t.FieldAlign() }

func (t *rtype) Kind() Kind { return Kind(t.t.Kind()) }

func (t *rtype) exportedMethods() []abi.Method {
	ut := t.uncommon()
	if ut == nil {
		return nil
	}
	return ut.ExportedMethods()
}

func (t *rtype) NumMethod() int {
	if t.Kind() == Interface {
		tt := (*interfaceType)(unsafe.Pointer(t))
		return tt.NumMethod()
	}
	return len(t.exportedMethods())
}

func (t *rtype) Method(i int) (m Method) {
	if t.Kind() == Interface {
		tt := (*interfaceType)(unsafe.Pointer(t))
		return tt.Method(i)
	}
	methods := t.exportedMethods()
	if i < 0 || i >= len(methods) {
		panic("reflect: Method index out of range")
	}
	p := methods[i]
	pname := t.nameOff(p.Name)
	m.Name = pname.Name()
	fl := flag(Func)
	mtyp := t.typeOff(p.Mtyp)
	ft := (*funcType)(unsafe.Pointer(mtyp))
	in := make([]Type, 0, 1+ft.NumIn())
	in = append(in, t)
	for _, arg := range ft.InSlice() {
		in = append(in, toRType(arg))
	}
	out := make([]Type, 0, ft.NumOut())
	for _, ret := range ft.OutSlice() {
		out = append(out, toRType(ret))
	}
	mt := FuncOf(in, out, ft.IsVariadic())
	m.Type = mt
	tfn := t.textOff(p.Tfn)
	fn := unsafe.Pointer(&tfn)
	m.Func = Value{&mt.(*rtype).t, fn, fl}

	m.Index = i
	return m
}

func (t *rtype) MethodByName(name string) (m Method, ok bool) {
	if t.Kind() == Interface {
		tt := (*interfaceType)(unsafe.Pointer(t))
		return tt.MethodByName(name)
	}
	ut := t.uncommon()
	if ut == nil {
		return Method{}, false
	}

	methods := ut.ExportedMethods()

	// We are looking for the first index i where the string becomes >= s.
	// This is a copy of sort.Search, with f(h) replaced by (t.nameOff(methods[h].name).name() >= name).
	i, j := 0, len(methods)
	for i < j {
		h := int(uint(i+j) >> 1) // avoid overflow when computing h
		// i ≤ h < j
		if !(t.nameOff(methods[h].Name).Name() >= name) {
			i = h + 1 // preserves f(i-1) == false
		} else {
			j = h // preserves f(j) == true
		}
	}
	// i == j, f(i-1) == false, and f(j) (= f(i)) == true  =>  answer is i.
	if i < len(methods) && name == t.nameOff(methods[i].Name).Name() {
		return t.Method(i), true
	}

	return Method{}, false
}

func (t *rtype) PkgPath() string {
	if t.t.TFlag&abi.TFlagNamed == 0 {
		return ""
	}
	ut := t.uncommon()
	if ut == nil {
		return ""
	}
	return t.nameOff(ut.PkgPath).Name()
}

func pkgPathFor(t *abi.Type) string {
	return toRType(t).PkgPath()
}

func (t *rtype) Name() string {
	if !t.t.HasName() {
		return ""
	}
	s := t.String()
	i := len(s) - 1
	sqBrackets := 0
	for i >= 0 && (s[i] != '.' || sqBrackets != 0) {
		switch s[i] {
		case ']':
			sqBrackets++
		case '[':
			sqBrackets--
		}
		i--
	}
	return s[i+1:]
}

func nameFor(t *abi.Type) string {
	return toRType(t).Name()
}

func (t *rtype) ChanDir() ChanDir {
	if t.Kind() != Chan {
		panic("reflect: ChanDir of non-chan type " + t.String())
	}
	tt := (*abi.ChanType)(unsafe.Pointer(t))
	return ChanDir(tt.Dir)
}

func toRType(t *abi.Type) *rtype {
	return (*rtype)(unsafe.Pointer(t))
}

func elem(t *abi.Type) *abi.Type {
	et := t.Elem()
	if et != nil {
		return et
	}
	panic("reflect: Elem of invalid type " + stringFor(t))
}

func (t *rtype) Elem() Type {
	return toType(elem(t.common()))
}

func (t *rtype) Field(i int) StructField {
	if t.Kind() != Struct {
		panic("reflect: Field of non-struct type " + t.String())
	}
	tt := (*structType)(unsafe.Pointer(t))
	return tt.Field(i)
}

func (t *rtype) FieldByIndex(index []int) StructField {
	if t.Kind() != Struct {
		panic("reflect: FieldByIndex of non-struct type " + t.String())
	}
	tt := (*structType)(unsafe.Pointer(t))
	return tt.FieldByIndex(index)
}

func (t *rtype) FieldByName(name string) (StructField, bool) {
	if t.Kind() != Struct {
		panic("reflect: FieldByName of non-struct type " + t.String())
	}
	tt := (*structType)(unsafe.Pointer(t))
	return tt.FieldByName(name)
}

func (t *rtype) FieldByNameFunc(match func(string) bool) (StructField, bool) {
	if t.Kind() != Struct {
		panic("reflect: FieldByNameFunc of non-struct type " + t.String())
	}
	tt := (*structType)(unsafe.Pointer(t))
	return tt.FieldByNameFunc(match)
}

func (t *rtype) Len() int {
	if t.Kind() != Array {
		panic("reflect: Len of non-array type " + t.String())
	}
	tt := (*arrayType)(unsafe.Pointer(t))
	return int(tt.Len)
}

func (t *rtype) NumField() int {
	if t.Kind() != Struct {
		panic("reflect: NumField of non-struct type " + t.String())
	}
	tt := (*structType)(unsafe.Pointer(t))
	return len(tt.Fields)
}

func (t *rtype) In(i int) Type {
	if t.Kind() != Func {
		panic("reflect: In of non-func type " + t.String())
	}
	tt := (*abi.FuncType)(unsafe.Pointer(t))
	return toType(tt.InSlice()[i])
}

func (t *rtype) NumIn() int {
	if t.Kind() != Func {
		panic("reflect: NumIn of non-func type " + t.String())
	}
	tt := (*abi.FuncType)(unsafe.Pointer(t))
	return tt.NumIn()
}

func (t *rtype) NumOut() int {
	if t.Kind() != Func {
		panic("reflect: NumOut of non-func type " + t.String())
	}
	tt := (*abi.FuncType)(unsafe.Pointer(t))
	return tt.NumOut()
}

func (t *rtype) Out(i int) Type {
	if t.Kind() != Func {
		panic("reflect: Out of non-func type " + t.String())
	}
	tt := (*abi.FuncType)(unsafe.Pointer(t))
	return toType(tt.OutSlice()[i])
}

func (t *rtype) IsVariadic() bool {
	if t.Kind() != Func {
		panic("reflect: IsVariadic of non-func type " + t.String())
	}
	tt := (*abi.FuncType)(unsafe.Pointer(t))
	return tt.IsVariadic()
}

func (t *rtype) OverflowComplex(x complex128) bool {
	k := t.Kind()
	switch k {
	case Complex64:
		return overflowFloat32(real(x)) || overflowFloat32(imag(x))
	case Complex128:
		return false
	}
	panic("reflect: OverflowComplex of non-complex type " + t.String())
}

func (t *rtype) OverflowFloat(x float64) bool {
	k := t.Kind()
	switch k {
	case Float32:
		return overflowFloat32(x)
	case Float64:
		return false
	}
	panic("reflect: OverflowFloat of non-float type " + t.String())
}

func (t *rtype) OverflowInt(x int64) bool {
	k := t.Kind()
	switch k {
	case Int, Int8, Int16, Int32, Int64:
		bitSize := t.Size() * 8
		trunc := (x << (64 - bitSize)) >> (64 - bitSize)
		return x != trunc
	}
	panic("reflect: OverflowInt of non-int type " + t.String())
}

func (t *rtype) OverflowUint(x uint64) bool {
	k := t.Kind()
	switch k {
	case Uint, Uintptr, Uint8, Uint16, Uint32, Uint64:
		bitSize := t.Size() * 8
		trunc := (x << (64 - bitSize)) >> (64 - bitSize)
		return x != trunc
	}
	panic("reflect: OverflowUint of non-uint type " + t.String())
}

func (t *rtype) CanSeq() bool {
	switch t.Kind() {
	case Int8, Int16, Int32, Int64, Int, Uint8, Uint16, Uint32, Uint64, Uint, Uintptr, Array, Slice, Chan, String, Map:
		return true
	case Func:
		return canRangeFunc(&t.t, 1)
	case Pointer:
		return t.Elem().Kind() == Array
	}
	return false
}

func (t *rtype) CanSeq2() bool {
	switch t.Kind() {
	case Array, Slice, String, Map:
		return true
	case Func:
		return canRangeFunc(&t.t, 2)
	case Pointer:
		return t.Elem().Kind() == Array
	}
	return false
}

func canRangeFunc(t *abi.Type, seq uint16) bool {
	if t.Kind() != abi.Func {
		return false
	}
	f := t.FuncType()
	if f.InCount != 1 || f.OutCount != 0 {
		return false
	}
	y := f.In(0)
	if y.Kind() != abi.Func {
		return false
	}
	yield := y.FuncType()
	return yield.InCount == seq && yield.OutCount == 1 && yield.Out(0).Kind() == abi.Bool && toRType(yield.Out(0)).PkgPath() == ""
}

func (t *rtype) Fields() iter.Seq[StructField] {
	if t.Kind() != Struct {
		panic("reflect: Fields of non-struct type " + t.String())
	}
	return func(yield func(StructField) bool) {
		for i := range t.NumField() {
			if !yield(t.Field(i)) {
				return
			}
		}
	}
}

func (t *rtype) Methods() iter.Seq[Method] {
	return func(yield func(Method) bool) {
		for i := range t.NumMethod() {
			if !yield(t.Method(i)) {
				return
			}
		}
	}
}

func (t *rtype) Ins() iter.Seq[Type] {
	if t.Kind() != Func {
		panic("reflect: Ins of non-func type " + t.String())
	}
	return func(yield func(Type) bool) {
		for i := range t.NumIn() {
			if !yield(t.In(i)) {
				return
			}
		}
	}
}

func (t *rtype) Outs() iter.Seq[Type] {
	if t.Kind() != Func {
		panic("reflect: Outs of non-func type " + t.String())
	}
	return func(yield func(Type) bool) {
		for i := range t.NumOut() {
			if !yield(t.Out(i)) {
				return
			}
		}
	}
}

// add returns p+x.
//
// The whySafe string is ignored, so that the function still inlines
// as efficiently as p+x, but all call sites should use the string to
// record why the addition is safe, which is to say why the addition
// does not cause x to advance to the very end of p's allocation
// and therefore point incorrectly at the next block in memory.
//
// add should be an internal detail (and is trivially copyable),
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/pinpoint-apm/pinpoint-go-agent
//   - github.com/vmware/govmomi
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
//go:linkname add
func add(p unsafe.Pointer, x uintptr, whySafe string) unsafe.Pointer {
	return unsafe.Pointer(uintptr(p) + x)
}

func (d ChanDir) String() string {
	switch d {
	case SendDir:
		return "chan<-"
	case RecvDir:
		return "<-chan"
	case BothDir:
		return "chan"
	}
	return "ChanDir" + strconv.Itoa(int(d))
}

// Method returns the i'th method in the type's method set.
func (t *interfaceType) Method(i int) (m Method) {
	if i < 0 || i >= len(t.Methods) {
		return
	}
	p := &t.Methods[i]
	pname := t.nameOff(p.Name)
	m.Name = pname.Name()
	if !pname.IsExported() {
		m.PkgPath = pkgPath(pname)
		if m.PkgPath == "" {
			m.PkgPath = t.PkgPath.Name()
		}
	}
	m.Type = toType(t.typeOff(p.Typ))
	m.Index = i
	return
}

// NumMethod returns the number of interface methods in the type's method set.
func (t *interfaceType) NumMethod() int { return len(t.Methods) }

// MethodByName method with the given name in the type's method set.
func (t *interfaceType) MethodByName(name string) (m Method, ok bool) {
	if t == nil {
		return
	}
	var p *abi.Imethod
	for i := range t.Methods {
		p = &t.Methods[i]
		if t.nameOff(p.Name).Name() == name {
			return t.Method(i), true
		}
	}
	return
}

// A StructField describes a single field in a struct.
type StructField struct {
	// Name is the field name.
	Name string

	// PkgPath is the package path that qualifies a lower case (unexported)
	// field name. It is empty for upper case (exported) field names.
	// See https://golang.org/ref/spec#Uniqueness_of_identifiers
	PkgPath string

	Type      Type      // field type
	Tag       StructTag // field tag string
	Offset    uintptr   // offset within struct, in bytes
	Index     []int     // index sequence for Type.FieldByIndex
	Anonymous bool      // is an embedded field
}

// IsExported reports whether the field is exported.
func (f StructField) IsExported() bool {
	return f.PkgPath == ""
}

// A StructTag is the tag string in a struct field.
//
// By convention, tag strings are a concatenation of
// optionally space-separated key:"value" pairs.
// Each key is a non-empty string consisting of non-control
// characters other than space (U+0020 ' '), quote (U+0022 '"'),
// and colon (U+003A ':').  Each value is quoted using U+0022 '"'
// characters and Go string literal syntax.
type StructTag string

// Get returns the value associated with key in the tag string.
// If there is no such key in the tag, Get returns the empty string.
// If the tag does not have the conventional format, the value
// returned by Get is unspecified. To determine whether a tag is
// explicitly set to the empty string, use [StructTag.Lookup].
func (tag StructTag) Get(key string) string {
	v, _ := tag.Lookup(key)
	return v
}

// Lookup returns the value associated with key in the tag string.
// If the key is present in the tag the value (which may be empty)
// is returned. Otherwise the returned value will be the empty string.
// The ok return value reports whether the value was explicitly set in
// the tag string. If the tag does not have the conventional format,
// the value returned by Lookup is unspecified.
func (tag StructTag) Lookup(key string) (value string, ok bool) {
	// When modifying this code, also update the validateStructTag code
	// in cmd/vet/structtag.go.

	for tag != "" {
		// Skip leading space.
		i := 0
		for i < len(tag) && tag[i] == ' ' {
			i++
		}
		tag = tag[i:]
		if tag == "" {
			break
		}

		// Scan to colon. A space, a quote or a control character is a syntax error.
		// Strictly speaking, control chars include the range [0x7f, 0x9f], not just
		// [0x00, 0x1f], but in practice, we ignore the multi-byte control characters
		// as it is simpler to inspect the tag's bytes than the tag's runes.
		i = 0
		for i < len(tag) && tag[i] > ' ' && tag[i] != ':' && tag[i] != '"' && tag[i] != 0x7f {
			i++
		}
		if i == 0 || i+1 >= len(tag) || tag[i] != ':' || tag[i+1] != '"' {
			break
		}
		name := string(tag[:i])
		tag = tag[i+1:]

		// Scan quoted string to find value.
		i = 1
		for i < len(tag) && tag[i] != '"' {
			if tag[i] == '\\' {
				i++
			}
			i++
		}
		if i >= len(tag) {
			break
		}
		qvalue := string(tag[:i+1])
		tag = tag[i+1:]

		if key == name {
			value, err := strconv.Unquote(qvalue)
			if err != nil {
				break
			}
			return value, true
		}
	}
	return "", false
}

// Field returns the i'th struct field.
func (t *structType) Field(i int) (f StructField) {
	if i < 0 || i >= len(t.Fields) {
		panic("reflect: Field index out of bounds")
	}
	p := &t.Fields[i]
	f.Type = toType(p.Typ)
	f.Name = p.Name.Name()
	f.Anonymous = p.Embedded()
	if !p.Name.IsExported() {
		f.PkgPath = t.PkgPath.Name()
	}
	if tag := p.Name.Tag(); tag != "" {
		f.Tag = StructTag(tag)
	}
	f.Offset = p.Offset

	// We can't safely use this optimization on js or wasi,
	// which do not appear to support read-only data.
	if i < 256 && runtime.GOOS != "js" && runtime.GOOS != "wasip1" {
		staticuint64s := getStaticuint64s()
		p := unsafe.Pointer(&(*staticuint64s)[i])
		if unsafe.Sizeof(int(0)) == 4 && goarch.BigEndian {
			p = unsafe.Add(p, 4)
		}
		f.Index = unsafe.Slice((*int)(p), 1)
	} else {
		// NOTE(rsc): This is the only allocation in the interface
		// presented by a reflect.Type. It would be nice to avoid,
		// but we need to make sure that misbehaving clients of
		// reflect cannot affect other uses of reflect.
		// One possibility is CL 5371098, but we postponed that
		// ugliness until there is a demonstrated
		// need for the performance. This is issue 2320.
		f.Index = []int{i}
	}
	return
}

// getStaticuint64s returns a pointer to an array of 256 uint64 values,
// defined in the runtime package in read-only memory.
// staticuint64s[0] == 0, staticuint64s[1] == 1, and so forth.
//
//go:linkname getStaticuint64s runtime.getStaticuint64s
func getStaticuint64s() *[256]uint64

// TODO(gri): Should there be an error/bool indicator if the index
// is wrong for FieldByIndex?

// FieldByIndex returns the nested field corresponding to index.
func (t *structType) FieldByIndex(index []int) (f StructField) {
	f.Type = toType(&t.Type)
	for i, x := range index {
		if i > 0 {
			ft := f.Type
			if ft.Kind() == Pointer && ft.Elem().Kind() == Struct {
				ft = ft.Elem()
			}
			f.Type = ft
		}
		f = f.Type.Field(x)
	}
	return
}

// A fieldScan represents an item on the fieldByNameFunc scan work list.
type fieldScan struct {
	typ   *structType
	index []int
}

// FieldByNameFunc returns the struct field with a name that satisfies the
// match function and a boolean to indicate if the field was found.
func (t *structType) FieldByNameFunc(match func(string) bool) (result StructField, ok bool) {
	// This uses the same condition that the Go language does: there must be a unique instance
	// of the match at a given depth level. If there are multiple instances of a match at the
	// same depth, they annihilate each other and inhibit any possible match at a lower level.
	// The algorithm is breadth first search, one depth level at a time.

	// The current and next slices are work queues:
	// current lists the fields to visit on this depth level,
	// and next lists the fields on the next lower level.
	current := []fieldScan{}
	next := []fieldScan{{typ: t}}

	// nextCount records the number of times an embedded type has been
	// encountered and considered for queueing in the 'next' slice.
	// We only queue the first one, but we increment the count on each.
	// If a struct type T can be reached more than once at a given depth level,
	// then it annihilates itself and need not be considered at all when we
	// process that next depth level.
	var nextCount map[*structType]int

	// visited records the structs that have been considered already.
	// Embedded pointer fields can create cycles in the graph of
	// reachable embedded types; visited avoids following those cycles.
	// It also avoids duplicated effort: if we didn't find the field in an
	// embedded type T at level 2, we won't find it in one at level 4 either.
	visited := map[*structType]bool{}

	for len(next) > 0 {
		current, next = next, current[:0]
		count := nextCount
		nextCount = nil

		// Process all the fields at this depth, now listed in 'current'.
		// The loop queues embedded fields found in 'next', for processing during the next
		// iteration. The multiplicity of the 'current' field counts is recorded
		// in 'count'; the multiplicity of the 'next' field counts is recorded in 'nextCount'.
		for _, scan := range current {
			t := scan.typ
			if visited[t] {
				// We've looked through this type before, at a higher level.
				// That higher level would shadow the lower level we're now at,
				// so this one can't be useful to us. Ignore it.
				continue
			}
			visited[t] = true
			for i := range t.Fields {
				f := &t.Fields[i]
				// Find name and (for embedded field) type for field f.
				fname := f.Name.Name()
				var ntyp *abi.Type
				if f.Embedded() {
					// Embedded field of type T or *T.
					ntyp = f.Typ
					if ntyp.Kind() == abi.Pointer {
						ntyp = ntyp.Elem()
					}
				}

				// Does it match?
				if match(fname) {
					// Potential match
					if count[t] > 1 || ok {
						// Name appeared multiple times at this level: annihilate.
						return StructField{}, false
					}
					result = t.Field(i)
					result.Index = nil
					result.Index = append(result.Index, scan.index...)
					result.Index = append(result.Index, i)
					ok = true
					continue
				}

				// Queue embedded struct fields for processing with next level,
				// but only if we haven't seen a match yet at this level and only
				// if the embedded types haven't already been queued.
				if ok || ntyp == nil || ntyp.Kind() != abi.Struct {
					continue
				}
				styp := (*structType)(unsafe.Pointer(ntyp))
				if nextCount[styp] > 0 {
					nextCount[styp] = 2 // exact multiple doesn't matter
					continue
				}
				if nextCount == nil {
					nextCount = map[*structType]int{}
				}
				nextCount[styp] = 1
				if count[t] > 1 {
					nextCount[styp] = 2 // exact multiple doesn't matter
				}
				var index []int
				index = append(index, scan.index...)
				index = append(index, i)
				next = append(next, fieldScan{styp, index})
			}
		}
		if ok {
			break
		}
	}
	return
}

// FieldByName returns the struct field with the given name
// and a boolean to indicate if the field was found.
func (t *structType) FieldByName(name string) (f StructField, present bool) {
	// Quick check for top-level name, or struct without embedded fields.
	hasEmbeds := false
	if name != "" {
		for i := range t.Fields {
			tf := &t.Fields[i]
			if tf.Name.Name() == name {
				return t.Field(i), true
			}
			if tf.Embedded() {
				hasEmbeds = true
			}
		}
	}
	if !hasEmbeds {
		return
	}
	return t.FieldByNameFunc(func(s string) bool { return s == name })
}

// TypeOf returns the reflection [Type] that represents the dynamic type of i.
// If i is a nil interface value, TypeOf returns nil.
func TypeOf(i any) Type {
	return toType(abi.TypeOf(i))
}

// TypeFor returns the [Type] that represents the type argument T.
func TypeFor[T any]() Type {
	// toRType is safe to use here; type is never nil as T is statically known.
	return toRType(abi.TypeFor[T]())
}

// rtypeOf directly extracts the *rtype of the provided value.
func rtypeOf(i any) *abi.Type {
	return abi.TypeOf(i)
}

// ptrMap is the cache for PointerTo.
var ptrMap sync.Map // map[*rtype]*ptrType

// PtrTo returns the pointer type with element t.
// For example, if t represents type Foo, PtrTo(t) represents *Foo.
//
// PtrTo is the old spelling of [PointerTo].
// The two functions behave identically.
//
// Deprecated: Superseded by [PointerTo].
//
//go:fix inline
func PtrTo(t Type) Type { return PointerTo(t) }

// PointerTo returns the pointer type with element t.
// For example, if t represents type Foo, PointerTo(t) represents *Foo.
func PointerTo(t Type) Type {
	return toRType(t.(*rtype).ptrTo())
}

func (t *rtype) ptrTo() *abi.Type {
	at := &t.t
	if at.PtrToThis != 0 {
		return t.typeOff(at.PtrToThis)
	}

	// Check the cache.
	if pi, ok := ptrMap.Load(t); ok {
		return &pi.(*ptrType).Type
	}

	// Look in known types.
	s := "*" + t.String()
	for _, tt := range typesByString(s) {
		p := (*ptrType)(unsafe.Pointer(tt))
		if p.Elem != &t.t {
			continue
		}
		pi, _ := ptrMap.LoadOrStore(t, p)
		return &pi.(*ptrType).Type
	}

	// Create a new ptrType starting with the description
	// of an *unsafe.Pointer.
	var iptr any = (*unsafe.Pointer)(nil)
	prototype := *(**ptrType)(unsafe.Pointer(&iptr))
	pp := *prototype

	pp.Str = resolveReflectName(newName(s, "", false, false))
	pp.PtrToThis = 0

	// For the type structures linked into the binary, the
	// compiler provides a good hash of the string.
	// Create a good hash for the new string by using
	// the FNV-1 hash's mixing function to combine the
	// old hash and the new "*".
	pp.Hash = fnv1(t.t.Hash, '*')

	pp.Elem = at

	pi, _ := ptrMap.LoadOrStore(t, &pp)
	return &pi.(*ptrType).Type
}

func ptrTo(t *abi.Type) *abi.Type {
	return toRType(t).ptrTo()
}

// fnv1 incorporates the list of bytes into the hash x using the FNV-1 hash function.
func fnv1(x uint32, list ...byte) uint32 {
	for _, b := range list {
		x = x*16777619 ^ uint32(b)
	}
	return x
}

func (t *rtype) Implements(u Type) bool {
	if u == nil {
		panic("reflect: nil type passed to Type.Implements")
	}
	if u.Kind() != Interface {
		panic("reflect: non-interface type passed to Type.Implements")
	}
	return implements(u.common(), t.common())
}

func (t *rtype) AssignableTo(u Type) bool {
	if u == nil {
		panic("reflect: nil type passed to Type.AssignableTo")
	}
	uu := u.common()
	return directlyAssignable(uu, t.common()) || implements(uu, t.common())
}

func (t *rtype) ConvertibleTo(u Type) bool {
	if u == nil {
		panic("reflect: nil type passed to Type.ConvertibleTo")
	}
	return convertOp(u.common(), t.common()) != nil
}

func (t *rtype) Comparable() bool {
	return t.t.Equal != nil
}

// implements reports whether the type V implements the interface type T.
func implements(T, V *abi.Type) bool {
	if T.Kind() != abi.Interface {
		return false
	}
	t := (*interfaceType)(unsafe.Pointer(T))
	if len(t.Methods) == 0 {
		return true
	}

	// The same algorithm applies in both cases, but the
	// method tables for an interface type and a concrete type
	// are different, so the code is duplicated.
	// In both cases the algorithm is a linear scan over the two
	// lists - T's methods and V's methods - simultaneously.
	// Since method tables are stored in a unique sorted order
	// (alphabetical, with no duplicate method names), the scan
	// through V's methods must hit a match for each of T's
	// methods along the way, or else V does not implement T.
	// This lets us run the scan in overall linear time instead of
	// the quadratic time  a naive search would require.
	// See also ../runtime/iface.go.
	if V.Kind() == abi.Interface {
		v := (*interfaceType)(unsafe.Pointer(V))
		i := 0
		for j := 0; j < len(v.Methods); j++ {
			tm := &t.Methods[i]
			tmName := t.nameOff(tm.Name)
			vm := &v.Methods[j]
			vmName := nameOffFor(V, vm.Name)
			if vmName.Name() == tmName.Name() && typeOffFor(V, vm.Typ) == t.typeOff(tm.Typ) {
				if !tmName.IsExported() {
					tmPkgPath := pkgPath(tmName)
					if tmPkgPath == "" {
						tmPkgPath = t.PkgPath.Name()
					}
					vmPkgPath := pkgPath(vmName)
					if vmPkgPath == "" {
						vmPkgPath = v.PkgPath.Name()
					}
					if tmPkgPath != vmPkgPath {
						continue
					}
				}
				if i++; i >= len(t.Methods) {
					return true
				}
			}
		}
		return false
	}

	v := V.Uncommon()
	if v == nil {
		return false
	}
	i := 0
	vmethods := v.Methods()
	for j := 0; j < int(v.Mcount); j++ {
		tm := &t.Methods[i]
		tmName := t.nameOff(tm.Name)
		vm := vmethods[j]
		vmName := nameOffFor(V, vm.Name)
		if vmName.Name() == tmName.Name() && typeOffFor(V, vm.Mtyp) == t.typeOff(tm.Typ) {
			if !tmName.IsExported() {
				tmPkgPath := pkgPath(tmName)
				if tmPkgPath == "" {
					tmPkgPath = t.PkgPath.Name()
				}
				vmPkgPath := pkgPath(vmName)
				if vmPkgPath == "" {
					vmPkgPath = nameOffFor(V, v.PkgPath).Name()
				}
				if tmPkgPath != vmPkgPath {
					continue
				}
			}
			if i++; i >= len(t.Methods) {
				return true
			}
		}
	}
	return false
}

// specialChannelAssignability reports whether a value x of channel type V
// can be directly assigned (using memmove) to another channel type T.
// https://golang.org/doc/go_spec.html#Assignability
// T and V must be both of Chan kind.
func specialChannelAssignability(T, V *abi.Type) bool {
	// Special case:
	// x is a bidirectional channel value, T is a channel type,
	// x's type V and T have identical element types,
	// and at least one of V or T is not a defined type.
	return V.ChanDir() == abi.BothDir && (nameFor(T) == "" || nameFor(V) == "") && haveIdenticalType(T.Elem(), V.Elem(), true)
}

// directlyAssignable reports whether a value x of type V can be directly
// assigned (using memmove) to a value of type T.
// https://golang.org/doc/go_spec.html#Assignability
// Ignoring the interface rules (implemented elsewhere)
// and the ideal constant rules (no ideal constants at run time).
func directlyAssignable(T, V *abi.Type) bool {
	// x's type V is identical to T?
	if T == V {
		return true
	}

	// Otherwise at least one of T and V must not be defined
	// and they must have the same kind.
	if T.HasName() && V.HasName() || T.Kind() != V.Kind() {
		return false
	}

	if T.Kind() == abi.Chan && specialChannelAssignability(T, V) {
		return true
	}

	// x's type T and V must have identical underlying types.
	return haveIdenticalUnderlyingType(T, V, true)
}

func haveIdenticalType(T, V *abi.Type, cmpTags bool) bool {
	if cmpTags {
		return T == V
	}

	if nameFor(T) != nameFor(V) || T.Kind() != V.Kind() || pkgPathFor(T) != pkgPathFor(V) {
		return false
	}

	return haveIdenticalUnderlyingType(T, V, false)
}

func haveIdenticalUnderlyingType(T, V *abi.Type, cmpTags bool) bool {
	if T == V {
		return true
	}

	kind := Kind(T.Kind())
	if kind != Kind(V.Kind()) {
		return false
	}

	// Non-composite types of equal kind have same underlying type
	// (the predefined instance of the type).
	if Bool <= kind && kind <= Complex128 || kind == String || kind == UnsafePointer {
		return true
	}

	// Composite types.
	switch kind {
	case Array:
		return T.Len() == V.Len() && haveIdenticalType(T.Elem(), V.Elem(), cmpTags)

	case Chan:
		return V.ChanDir() == T.ChanDir() && haveIdenticalType(T.Elem(), V.Elem(), cmpTags)

	case Func:
		t := (*funcType)(unsafe.Pointer(T))
		v := (*funcType)(unsafe.Pointer(V))
		if t.OutCount != v.OutCount || t.InCount != v.InCount {
			return false
		}
		for i := 0; i < t.NumIn(); i++ {
			if !haveIdenticalType(t.In(i), v.In(i), cmpTags) {
				return false
			}
		}
		for i := 0; i < t.NumOut(); i++ {
			if !haveIdenticalType(t.Out(i), v.Out(i), cmpTags) {
				return false
			}
		}
		return true

	case Interface:
		t := (*interfaceType)(unsafe.Pointer(T))
		v := (*interfaceType)(unsafe.Pointer(V))
		if len(t.Methods) == 0 && len(v.Methods) == 0 {
			return true
		}
		// Might have the same methods but still
		// need a run time conversion.
		return false

	case Map:
		return haveIdenticalType(T.Key(), V.Key(), cmpTags) && haveIdenticalType(T.Elem(), V.Elem(), cmpTags)

	case Pointer, Slice:
		return haveIdenticalType(T.Elem(), V.Elem(), cmpTags)

	case Struct:
		t := (*structType)(unsafe.Pointer(T))
		v := (*structType)(unsafe.Pointer(V))
		if len(t.Fields) != len(v.Fields) {
			return false
		}
		if t.PkgPath.Name() != v.PkgPath.Name() {
			return false
		}
		for i := range t.Fields {
			tf := &t.Fields[i]
			vf := &v.Fields[i]
			if tf.Name.Name() != vf.Name.Name() {
				return false
			}
			if !haveIdenticalType(tf.Typ, vf.Typ, cmpTags) {
				return false
			}
			if cmpTags && tf.Name.Tag() != vf.Name.Tag() {
				return false
			}
			if tf.Offset != vf.Offset {
				return false
			}
			if tf.Embedded() != vf.Embedded() {
				return false
			}
		}
		return true
	}

	return false
}

// compiledTypelinks is implemented in package runtime.
// It returns the types defined by the first module,
// and a slice of types defined in any other modules.
// Each slice of types is sorted by string.
//
// Note that strings are not unique identifiers for types:
// there can be more than one with a given string.
// Only types we might want to look up are included:
// pointers, channels, maps, slices, and arrays.
//
//go:linknamestd compiledTypelinks
func compiledTypelinks() ([]*abi.Type, [][]*abi.Type)

// rtypeOff should be an internal detail,
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/goccy/go-json
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
//go:linkname rtypeOff
func rtypeOff(section unsafe.Pointer, off int32) *abi.Type {
	return (*abi.Type)(add(section, uintptr(off), "sizeof(rtype) > 0"))
}

// typesByString returns all known types whose elements have
// the given string representation.
// It may be empty (no known types with that string) or may have
// multiple elements (multiple types with that string).
//
// typesByString should be an internal detail,
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/aristanetworks/goarista
//   - fortio.org/log
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
//go:linkname typesByString
func typesByString(s string) []*abi.Type {
	first, rest := compiledTypelinks()
	var ret []*abi.Type

	searchTypes := func(types []*abi.Type) {
		// We are looking for the first index i where the string becomes >= s.
		// This is a copy of sort.Search, with f(h) replaced by (*typ[h].String() >= s).
		i, j := 0, len(types)
		for i < j {
			h := int(uint(i+j) >> 1) // avoid overflow when computing h
			// i ≤ h < j
			if !(stringFor(types[h]) >= s) {
				i = h + 1 // preserves f(i-1) == false
			} else {
				j = h // preserves f(j) == true
			}
		}
		// i == j, f(i-1) == false, and f(j) (= f(i)) == true  =>  answer is i.

		// Having found the first, linear scan forward to find the last.
		// We could do a second binary search, but the caller is going
		// to do a linear scan anyway.
		for j := i; j < len(types); j++ {
			typ := types[j]
			if stringFor(typ) != s {
				break
			}
			ret = append(ret, typ)
		}
	}

	searchTypes(first)
	for _, r := range rest {
		searchTypes(r)
	}

	return ret
}

// The lookupCache caches ArrayOf, ChanOf, MapOf and SliceOf lookups.
var lookupCache sync.Map // map[cacheKey]*rtype

// A cacheKey is the key for use in the lookupCache.
// Four values describe any of the types we are looking for:
// type kind, one or two subtypes, and an extra integer.
type cacheKey struct {
	kind  Kind
	t1    *abi.Type
	t2    *abi.Type
	extra uintptr
}

// The funcLookupCache caches FuncOf lookups.
// FuncOf does not share the common lookupCache since cacheKey is not
// sufficient to represent functions unambiguously.
var funcLookupCache struct {
	sync.Mutex // Guards stores (but not loads) on m.

	// m is a map[uint32][]*rtype keyed by the hash calculated in FuncOf.
	// Elements of m are append-only and thus safe for concurrent reading.
	m sync.Map
}

// ChanOf returns the channel type with the given direction and element type.
// For example, if t represents int, ChanOf(RecvDir, t) represents <-chan int.
//
// The gc runtime imposes a limit of 64 kB on channel element types.
// If t's size is equal to or exceeds this limit, ChanOf panics.
func ChanOf(dir ChanDir, t Type) Type {
	typ := t.common()

	// Look in cache.
	ckey := cacheKey{Chan, typ, nil, uintptr(dir)}
	if ch, ok := lookupCache.Load(ckey); ok {
		return ch.(*rtype)
	}

	// This restriction is imposed by the gc compiler and the runtime.
	if typ.Size_ >= 1<<16 {
		panic("reflect.ChanOf: element size too large")
	}

	// Look in known types.
	var s string
	switch dir {
	default:
		panic("reflect.ChanOf: invalid dir")
	case SendDir:
		s = "chan<- " + stringFor(typ)
	case RecvDir:
		s = "<-chan " + stringFor(typ)
	case BothDir:
		typeStr := stringFor(typ)
		if typeStr[0] == '<' {
			// typ is recv chan, need parentheses as "<-" associates with leftmost
			// chan possible, see:
			// * https://golang.org/ref/spec#Channel_types
			// * https://github.com/golang/go/issues/39897
			s = "chan (" + typeStr + ")"
		} else {
			s = "chan " + typeStr
		}
	}
	for _, tt := range typesByString(s) {
		ch := (*chanType)(unsafe.Pointer(tt))
		if ch.Elem == typ && ch.Dir == abi.ChanDir(dir) {
			ti, _ := lookupCache.LoadOrStore(ckey, toRType(tt))
			return ti.(Type)
		}
	}

	// Make a channel type.
	var ichan any = (chan unsafe.Pointer)(nil)
	prototype := *(**chanType)(unsafe.Pointer(&ichan))
	ch := *prototype
	ch.TFlag = abi.TFlagRegularMemory | abi.TFlagDirectIface
	ch.Dir = abi.ChanDir(dir)
	ch.Str = resolveReflectName(newName(s, "", false, false))
	ch.Hash = fnv1(typ.Hash, 'c', byte(dir))
	ch.Elem = typ

	ti, _ := lookupCache.LoadOrStore(ckey, toRType(&ch.Type))
	return ti.(Type)
}

var funcTypes []Type
var funcTypesMutex sync.Mutex

func initFuncTypes(n int) Type {
	funcTypesMutex.Lock()
	defer funcTypesMutex.Unlock()
	if n >= len(funcTypes) {
		newFuncTypes := make([]Type, n+1)
		copy(newFuncTypes, funcTypes)
		funcTypes = newFuncTypes
	}
	if funcTypes[n] != nil {
		return funcTypes[n]
	}

	funcTypes[n] = StructOf([]StructField{
		{
			Name: "FuncType",
			Type: TypeOf(funcType{}),
		},
		{
			Name: "Args",
			Type: ArrayOf(n, TypeOf(&rtype{})),
		},
	})
	return funcTypes[n]
}

// FuncOf returns the function type with the given argument and result types.
// For example if k represents int and e represents string,
// FuncOf([]Type{k}, []Type{e}, false) represents func(int) string.
//
// The variadic argument controls whether the function is variadic. FuncOf
// panics if the in[len(in)-1] does not represent a slice and variadic is
// true.
func FuncOf(in, out []Type, variadic bool) Type {
	if variadic && (len(in) == 0 || in[len(in)-1].Kind() != Slice) {
		panic("reflect.FuncOf: last arg of variadic func must be slice")
	}

	// Make a func type.
	var ifunc any = (func())(nil)
	prototype := *(**funcType)(unsafe.Pointer(&ifunc))
	n := len(in) + len(out)

	if n > 128 {
		panic("reflect.FuncOf: too many arguments")
	}

	o := New(initFuncTypes(n)).Elem()
	ft := (*funcType)(unsafe.Pointer(o.Field(0).Addr().Pointer()))
	args := unsafe.Slice((**rtype)(unsafe.Pointer(o.Field(1).Addr().Pointer())), n)[0:0:n]
	*ft = *prototype

	// Build a hash and minimally populate ft.
	var hash uint32
	for _, in := range in {
		t := in.(*rtype)
		args = append(args, t)
		hash = fnv1(hash, byte(t.t.Hash>>24), byte(t.t.Hash>>16), byte(t.t.Hash>>8), byte(t.t.Hash))
	}
	if variadic {
		hash = fnv1(hash, 'v')
	}
	hash = fnv1(hash, '.')
	for _, out := range out {
		t := out.(*rtype)
		args = append(args, t)
		hash = fnv1(hash, byte(t.t.Hash>>24), byte(t.t.Hash>>16), byte(t.t.Hash>>8), byte(t.t.Hash))
	}

	ft.TFlag = abi.TFlagDirectIface
	ft.Hash = hash
	ft.InCount = uint16(len(in))
	ft.OutCount = uint16(len(out))
	if variadic {
		ft.OutCount |= 1 << 15
	}

	// Look in cache.
	if ts, ok := funcLookupCache.m.Load(hash); ok {
		for _, t := range ts.([]*abi.Type) {
			if haveIdenticalUnderlyingType(&ft.Type, t, true) {
				return toRType(t)
			}
		}
	}

	// Not in cache, lock and retry.
	funcLookupCache.Lock()
	defer funcLookupCache.Unlock()
	if ts, ok := funcLookupCache.m.Load(hash); ok {
		for _, t := range ts.([]*abi.Type) {
			if haveIdenticalUnderlyingType(&ft.Type, t, true) {
				return toRType(t)
			}
		}
	}

	addToCache := func(tt *abi.Type) Type {
		var rts []*abi.Type
		if rti, ok := funcLookupCache.m.Load(hash); ok {
			rts = rti.([]*abi.Type)
		}
		funcLookupCache.m.Store(hash, append(rts, tt))
		return toType(tt)
	}

	// Look in known types for the same string representation.
	str := funcStr(ft)
	for _, tt := range typesByString(str) {
		if haveIdenticalUnderlyingType(&ft.Type, tt, true) {
			return addToCache(tt)
		}
	}

	// Populate the remaining fields of ft and store in cache.
	ft.Str = resolveReflectName(newName(str, "", false, false))
	ft.PtrToThis = 0
	return addToCache(&ft.Type)
}
func stringFor(t *abi.Type) string {
	return toRType(t).String()
}

// funcStr builds a string representation of a funcType.
func funcStr(ft *funcType) string {
	repr := make([]byte, 0, 64)
	repr = append(repr, "func("...)
	for i, t := range ft.InSlice() {
		if i > 0 {
			repr = append(repr, ", "...)
		}
		if ft.IsVariadic() && i == int(ft.InCount)-1 {
			repr = append(repr, "..."...)
			repr = append(repr, stringFor((*sliceType)(unsafe.Pointer(t)).Elem)...)
		} else {
			repr = append(repr, stringFor(t)...)
		}
	}
	repr = append(repr, ')')
	out := ft.OutSlice()
	if len(out) == 1 {
		repr = append(repr, ' ')
	} else if len(out) > 1 {
		repr = append(repr, " ("...)
	}
	for i, t := range out {
		if i > 0 {
			repr = append(repr, ", "...)
		}
		repr = append(repr, stringFor(t)...)
	}
	if len(out) > 1 {
		repr = append(repr, ')')
	}
	return string(repr)
}

// isReflexive reports whether the == operation on the type is reflexive.
// That is, x == x for all values x of type t.
func isReflexive(t *abi.Type) bool {
	switch Kind(t.Kind()) {
	case Bool, Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64, Uintptr, Chan, Pointer, String, UnsafePointer:
		return true
	case Float32, Float64, Complex64, Complex128, Interface:
		return false
	case Array:
		tt := (*arrayType)(unsafe.Pointer(t))
		return isReflexive(tt.Elem)
	case Struct:
		tt := (*structType)(unsafe.Pointer(t))
		for _, f := range tt.Fields {
			if !isReflexive(f.Typ) {
				return false
			}
		}
		return true
	default:
		// Func, Map, Slice, Invalid
		panic("isReflexive called on non-key type " + stringFor(t))
	}
}

// needKeyUpdate reports whether map overwrites require the key to be copied.
func needKeyUpdate(t *abi.Type) bool {
	switch Kind(t.Kind()) {
	case Bool, Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64, Uintptr, Chan, Pointer, UnsafePointer:
		return false
	case Float32, Float64, Complex64, Complex128, Interface, String:
		// Float keys can be updated from +0 to -0.
		// String keys can be updated to use a smaller backing store.
		// Interfaces might have floats or strings in them.
		return true
	case Array:
		tt := (*arrayType)(unsafe.Pointer(t))
		return needKeyUpdate(tt.Elem)
	case Struct:
		tt := (*structType)(unsafe.Pointer(t))
		for _, f := range tt.Fields {
			if needKeyUpdate(f.Typ) {
				return true
			}
		}
		return false
	default:
		// Func, Map, Slice, Invalid
		panic("needKeyUpdate called on non-key type " + stringFor(t))
	}
}

// hashMightPanic reports whether the hash of a map key of type t might panic.
func hashMightPanic(t *abi.Type) bool {
	switch Kind(t.Kind()) {
	case Interface:
		return true
	case Array:
		tt := (*arrayType)(unsafe.Pointer(t))
		return hashMightPanic(tt.Elem)
	case Struct:
		tt := (*structType)(unsafe.Pointer(t))
		for _, f := range tt.Fields {
			if hashMightPanic(f.Typ) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// emitGCMask writes the GC mask for [n]typ into out, starting at bit
// offset base.
func emitGCMask(out []byte, base uintptr, typ *abi.Type, n uintptr) {
	ptrs := typ.PtrBytes / goarch.PtrSize
	words := typ.Size_ / goarch.PtrSize
	mask := typ.GcSlice(0, (ptrs+7)/8)
	for j := uintptr(0); j < ptrs; j++ {
		if (mask[j/8]>>(j%8))&1 != 0 {
			for i := uintptr(0); i < n; i++ {
				k := base + i*words + j
				out[k/8] |= 1 << (k % 8)
			}
		}
	}
}

// SliceOf returns the slice type with element type t.
// For example, if t represents int, SliceOf(t) represents []int.
func SliceOf(t Type) Type {
	typ := t.common()

	// Look in cache.
	ckey := cacheKey{Slice, typ, nil, 0}
	if slice, ok := lookupCache.Load(ckey); ok {
		return slice.(Type)
	}

	// Look in known types.
	s := "[]" + stringFor(typ)
	for _, tt := range typesByString(s) {
		slice := (*sliceType)(unsafe.Pointer(tt))
		if slice.Elem == typ {
			ti, _ := lookupCache.LoadOrStore(ckey, toRType(tt))
			return ti.(Type)
		}
	}

	// Make a slice type.
	var islice any = ([]unsafe.Pointer)(nil)
	prototype := *(**sliceType)(unsafe.Pointer(&islice))
	slice := *prototype
	slice.TFlag = 0
	slice.Str = resolveReflectName(newName(s, "", false, false))
	slice.Hash = fnv1(typ.Hash, '[')
	slice.Elem = typ
	slice.PtrToThis = 0

	ti, _ := lookupCache.LoadOrStore(ckey, toRType(&slice.Type))
	return ti.(Type)
}

// The structLookupCache caches StructOf lookups.
// StructOf does not share the common lookupCache since we need to pin
// the memory associated with *structTypeFixedN.
var structLookupCache struct {
	sync.Mutex // Guards stores (but not loads) on m.

	// m is a map[uint32][]Type keyed by the hash calculated in StructOf.
	// Elements in m are append-only and thus safe for concurrent reading.
	m sync.Map
}

type structTypeUncommon struct {
	structType
	u uncommonType
}

// isLetter reports whether a given 'rune' is classified as a Letter.
func isLetter(ch rune) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_' || ch >= utf8.RuneSelf && unicode.IsLetter(ch)
}

// isValidFieldName checks if a string is a valid (struct) field name or not.
//
// According to the language spec, a field name should be an identifier.
//
// identifier = letter { letter | unicode_digit } .
// letter = unicode_letter | "_" .
func isValidFieldName(fieldName string) bool {
	for i, c := range fieldName {
		if i == 0 && !isLetter(c) {
			return false
		}

		if !(isLetter(c) || unicode.IsDigit(c)) {
			return false
		}
	}

	return len(fieldName) > 0
}

// This must match cmd/compile/internal/compare.IsRegularMemory
func isRegularMemory(t Type) bool {
	switch t.Kind() {
	case Array:
		elem := t.Elem()
		if isRegularMemory(elem) {
			return true
		}
		return elem.Comparable() && t.Len() == 0
	case Int8, Int16, Int32, Int64, Int, Uint8, Uint16, Uint32, Uint64, Uint, Uintptr, Chan, Pointer, Bool, UnsafePointer:
		return true
	case Struct:
		num := t.NumField()
		switch num {
		case 0:
			return true
		case 1:
			field := t.Field(0)
			if field.Name == "_" {
				return false
			}
			return isRegularMemory(field.Type)
		default:
			for i := range num {
				field := t.Field(i)
				if field.Name == "_" || !isRegularMemory(field.Type) || isPaddedField(t, i) {
					return false
				}
			}
			return true
		}
	}
	return false
}

// isPaddedField reports whether the i'th field of struct type t is followed
// by padding.
func isPaddedField(t Type, i int) bool {
	field := t.Field(i)
	if i+1 < t.NumField() {
		return field.Offset+field.Type.Size() != t.Field(i+1).Offset
	}
	return field.Offset+field.Type.Size() != t.Size()
}

// StructOf returns the struct type containing fields.
// The Offset and Index fields are ignored and computed as they would be
// by the compiler.
//
// StructOf currently does not support promoted methods of embedded fields
// and panics if passed unexported StructFields.
func StructOf(fields []StructField) Type {
	var (
		hash       = fnv1(0, []byte("struct {")...)
		size       uintptr
		typalign   uint8
		comparable = true
		methods    []abi.Method

		fs   = make([]structField, len(fields))
		repr = make([]byte, 0, 64)
		fset = map[string]struct{}{} // fields' names
	)

	lastzero := uintptr(0)
	repr = append(repr, "struct {"...)
	pkgpath := ""
	for i, field := range fields {
		if field.Name == "" {
			panic("reflect.StructOf: field " + strconv.Itoa(i) + " has no name")
		}
		if !isValidFieldName(field.Name) {
			panic("reflect.StructOf: field " + strconv.Itoa(i) + " has invalid name")
		}
		if field.Type == nil {
			panic("reflect.StructOf: field " + strconv.Itoa(i) + " has no type")
		}
		f, fpkgpath := runtimeStructField(field)
		ft := f.Typ
		if fpkgpath != "" {
			if pkgpath == "" {
				pkgpath = fpkgpath
			} else if pkgpath != fpkgpath {
				panic("reflect.StructOf: fields with different PkgPath " + pkgpath + " and " + fpkgpath)
			}
		}

		// Update string and hash
		name := f.Name.Name()
		hash = fnv1(hash, []byte(name)...)
		if !f.Embedded() {
			repr = append(repr, (" " + name)...)
		} else {
			// Embedded field
			if f.Typ.Kind() == abi.Pointer {
				// Embedded ** and *interface{} are illegal
				elem := ft.Elem()
				if k := elem.Kind(); k == abi.Pointer || k == abi.Interface {
					panic("reflect.StructOf: illegal embedded field type " + stringFor(ft))
				}
			}

			switch Kind(f.Typ.Kind()) {
			case Interface:
				ift := (*interfaceType)(unsafe.Pointer(ft))
				for _, m := range ift.Methods {
					if pkgPath(ift.nameOff(m.Name)) != "" {
						// TODO(sbinet).  Issue 15924.
						panic("reflect: embedded interface with unexported method(s) not implemented")
					}

					fnStub := resolveReflectText(unsafe.Pointer(abi.FuncPCABIInternal(embeddedIfaceMethStub)))
					methods = append(methods, abi.Method{
						Name: resolveReflectName(ift.nameOff(m.Name)),
						Mtyp: resolveReflectType(ift.typeOff(m.Typ)),
						Ifn:  fnStub,
						Tfn:  fnStub,
					})
				}
			case Pointer:
				ptr := (*ptrType)(unsafe.Pointer(ft))
				if unt := ptr.Uncommon(); unt != nil {
					if i > 0 && unt.Mcount > 0 {
						// Issue 15924.
						panic("reflect: embedded type with methods not implemented if type is not first field")
					}
					if len(fields) > 1 {
						panic("reflect: embedded type with methods not implemented if there is more than one field")
					}
					for _, m := range unt.Methods() {
						mname := nameOffFor(ft, m.Name)
						if pkgPath(mname) != "" {
							// TODO(sbinet).
							// Issue 15924.
							panic("reflect: embedded interface with unexported method(s) not implemented")
						}
						methods = append(methods, abi.Method{
							Name: resolveReflectName(mname),
							Mtyp: resolveReflectType(typeOffFor(ft, m.Mtyp)),
							Ifn:  resolveReflectText(textOffFor(ft, m.Ifn)),
							Tfn:  resolveReflectText(textOffFor(ft, m.Tfn)),
						})
					}
				}
				if unt := ptr.Elem.Uncommon(); unt != nil {
					for _, m := range unt.Methods() {
						mname := nameOffFor(ft, m.Name)
						if pkgPath(mname) != "" {
							// TODO(sbinet)
							// Issue 15924.
							panic("reflect: embedded interface with unexported method(s) not implemented")
						}
						methods = append(methods, abi.Method{
							Name: resolveReflectName(mname),
							Mtyp: resolveReflectType(typeOffFor(ptr.Elem, m.Mtyp)),
							Ifn:  resolveReflectText(textOffFor(ptr.Elem, m.Ifn)),
							Tfn:  resolveReflectText(textOffFor(ptr.Elem, m.Tfn)),
						})
					}
				}
			default:
				if unt := ft.Uncommon(); unt != nil {
					if i > 0 && unt.Mcount > 0 {
						// Issue 15924.
						panic("reflect: embedded type with methods not implemented if type is not first field")
					}
					if len(fields) > 1 && ft.IsDirectIface() {
						panic("reflect: embedded type with methods not implemented for non-pointer type")
					}
					for _, m := range unt.Methods() {
						mname := nameOffFor(ft, m.Name)
						if pkgPath(mname) != "" {
							// TODO(sbinet)
							// Issue 15924.
							panic("reflect: embedded interface with unexported method(s) not implemented")
						}
						methods = append(methods, abi.Method{
							Name: resolveReflectName(mname),
							Mtyp: resolveReflectType(typeOffFor(ft, m.Mtyp)),
							Ifn:  resolveReflectText(textOffFor(ft, m.Ifn)),
							Tfn:  resolveReflectText(textOffFor(ft, m.Tfn)),
						})

					}
				}
			}
		}
		if _, dup := fset[name]; dup && name != "_" {
			panic("reflect.StructOf: duplicate field " + name)
		}
		fset[name] = struct{}{}

		hash = fnv1(hash, byte(ft.Hash>>24), byte(ft.Hash>>16), byte(ft.Hash>>8), byte(ft.Hash))

		repr = append(repr, (" " + stringFor(ft))...)
		if f.Name.HasTag() {
			hash = fnv1(hash, []byte(f.Name.Tag())...)
			repr = append(repr, (" " + strconv.Quote(f.Name.Tag()))...)
		}
		if i < len(fields)-1 {
			repr = append(repr, ';')
		}

		comparable = comparable && (ft.Equal != nil)

		offset := align(size, uintptr(ft.Align_))
		if offset < size {
			panic("reflect.StructOf: struct size would exceed virtual address space")
		}
		if ft.Align_ > typalign {
			typalign = ft.Align_
		}
		size = offset + ft.Size_
		if size < offset {
			panic("reflect.StructOf: struct size would exceed virtual address space")
		}
		f.Offset = offset

		if ft.Size_ == 0 {
			lastzero = size
		}

		fs[i] = f
	}

	if size > 0 && lastzero == size {
		// This is a non-zero sized struct that ends in a
		// zero-sized field. We add an extra byte of padding,
		// to ensure that taking the address of the final
		// zero-sized field can't manufacture a pointer to the
		// next object in the heap. See issue 9401.
		size++
		if size == 0 {
			panic("reflect.StructOf: struct size would exceed virtual address space")
		}
	}

	var typ *structType
	var ut *uncommonType

	if len(methods) == 0 {
		t := new(structTypeUncommon)
		typ = &t.structType
		ut = &t.u
	} else {
		// A *rtype representing a struct is followed directly in memory by an
		// array of method objects representing the methods attached to the
		// struct. To get the same layout for a run time generated type, we
		// need an array directly following the uncommonType memory.
		// A similar strategy is used for funcTypeFixed4, ...funcTypeFixedN.
		tt := New(StructOf([]StructField{
			{Name: "S", Type: TypeOf(structType{})},
			{Name: "U", Type: TypeOf(uncommonType{})},
			{Name: "M", Type: ArrayOf(len(methods), TypeOf(methods[0]))},
		}))

		typ = (*structType)(tt.Elem().Field(0).Addr().UnsafePointer())
		ut = (*uncommonType)(tt.Elem().Field(1).Addr().UnsafePointer())

		copy(tt.Elem().Field(2).Slice(0, len(methods)).Interface().([]abi.Method), methods)
	}
	// TODO(sbinet): Once we allow embedding multiple types,
	// methods will need to be sorted like the compiler does.
	// TODO(sbinet): Once we allow non-exported methods, we will
	// need to compute xcount as the number of exported methods.
	ut.Mcount = uint16(len(methods))
	ut.Xcount = ut.Mcount
	ut.Moff = uint32(unsafe.Sizeof(uncommonType{}))

	if len(fs) > 0 {
		repr = append(repr, ' ')
	}
	repr = append(repr, '}')
	hash = fnv1(hash, '}')
	str := string(repr)

	// Round the size up to be a multiple of the alignment.
	s := align(size, uintptr(typalign))
	if s < size {
		panic("reflect.StructOf: struct size would exceed virtual address space")
	}
	size = s

	// Make the struct type.
	var istruct any = struct{}{}
	prototype := *(**structType)(unsafe.Pointer(&istruct))
	*typ = *prototype
	typ.Fields = fs
	if pkgpath != "" {
		typ.PkgPath = newName(pkgpath, "", false, false)
	}

	// Look in cache.
	if ts, ok := structLookupCache.m.Load(hash); ok {
		for _, st := range ts.([]Type) {
			t := st.common()
			if haveIdenticalUnderlyingType(&typ.Type, t, true) {
				return toType(t)
			}
		}
	}

	// Not in cache, lock and retry.
	structLookupCache.Lock()
	defer structLookupCache.Unlock()
	if ts, ok := structLookupCache.m.Load(hash); ok {
		for _, st := range ts.([]Type) {
			t := st.common()
			if haveIdenticalUnderlyingType(&typ.Type, t, true) {
				return toType(t)
			}
		}
	}

	addToCache := func(t Type) Type {
		var ts []Type
		if ti, ok := structLookupCache.m.Load(hash); ok {
			ts = ti.([]Type)
		}
		structLookupCache.m.Store(hash, append(ts, t))
		return t
	}

	// Look in known types.
	for _, t := range typesByString(str) {
		if haveIdenticalUnderlyingType(&typ.Type, t, true) {
			// even if 't' wasn't a structType with methods, we should be ok
			// as the 'u uncommonType' field won't be accessed except when
			// tflag&abi.TFlagUncommon is set.
			return addToCache(toType(t))
		}
	}

	typ.Str = resolveReflectName(newName(str, "", false, false))
	if isRegularMemory(toType(&typ.Type)) {
		typ.TFlag = abi.TFlagRegularMemory
	} else {
		typ.TFlag = 0
	}
	typ.Hash = hash
	typ.Size_ = size
	typ.PtrBytes = typeptrdata(&typ.Type)
	typ.Align_ = typalign
	typ.FieldAlign_ = typalign
	typ.PtrToThis = 0
	if len(methods) > 0 {
		typ.TFlag |= abi.TFlagUncommon
	}

	if typ.PtrBytes == 0 {
		typ.GCData = nil
	} else if typ.PtrBytes <= abi.MaxPtrmaskBytes*8*goarch.PtrSize {
		bv := new(bitVector)
		addTypeBits(bv, 0, &typ.Type)
		typ.GCData = &bv.data[0]
	} else {
		// Runtime will build the mask if needed. We just need to allocate
		// space to store it.
		typ.TFlag |= abi.TFlagGCMaskOnDemand
		typ.GCData = (*byte)(unsafe.Pointer(new(uintptr)))
		if runtime.GOOS == "aix" {
			typ.GCData = adjustAIXGCData(typ.GCData)
		}
	}

	typ.Equal = nil
	if comparable {
		typ.Equal = func(p, q unsafe.Pointer) bool {
			for _, ft := range typ.Fields {
				pi := add(p, ft.Offset, "&x.field safe")
				qi := add(q, ft.Offset, "&x.field safe")
				if !ft.Typ.Equal(pi, qi) {
					return false
				}
			}
			return true
		}
	}

	switch {
	case typ.Size_ == goarch.PtrSize && typ.PtrBytes == goarch.PtrSize:
		typ.TFlag |= abi.TFlagDirectIface
	default:
		typ.TFlag &^= abi.TFlagDirectIface
	}

	return addToCache(toType(&typ.Type))
}

func embeddedIfaceMethStub() {
	panic("reflect: StructOf does not support methods of embedded interfaces")
}

// runtimeStructField takes a StructField value passed to StructOf and
// returns both the corresponding internal representation, of type
// structField, and the pkgpath value to use for this field.
func runtimeStructField(field StructField) (structField, string) {
	if field.Anonymous && field.PkgPath != "" {
		panic("reflect.StructOf: field \"" + field.Name + "\" is anonymous but has PkgPath set")
	}

	if field.IsExported() {
		// Best-effort check for misuse.
		// Since this field will be treated as exported, not much harm done if Unicode lowercase slips through.
		c := field.Name[0]
		if 'a' <= c && c <= 'z' || c == '_' {
			panic("reflect.StructOf: field \"" + field.Name + "\" is unexported but missing PkgPath")
		}
	}

	resolveReflectType(field.Type.common()) // install in runtime
	f := structField{
		Name:   newName(field.Name, string(field.Tag), field.IsExported(), field.Anonymous),
		Typ:    field.Type.common(),
		Offset: 0,
	}
	return f, field.PkgPath
}

// typeptrdata returns the length in bytes of the prefix of t
// containing pointer data. Anything after this offset is scalar data.
// keep in sync with ../cmd/compile/internal/reflectdata/reflect.go
func typeptrdata(t *abi.Type) uintptr {
	switch t.Kind() {
	case abi.Struct:
		st := (*structType)(unsafe.Pointer(t))
		// find the last field that has pointers.
		field := -1
		for i := range st.Fields {
			ft := st.Fields[i].Typ
			if ft.Pointers() {
				field = i
			}
		}
		if field == -1 {
			return 0
		}
		f := st.Fields[field]
		return f.Offset + f.Typ.PtrBytes

	default:
		panic("reflect.typeptrdata: unexpected type, " + stringFor(t))
	}
}

// ArrayOf returns the array type with the given length and element type.
// For example, if t represents int, ArrayOf(5, t) represents [5]int.
//
// If the resulting type would be larger than the available address space,
// ArrayOf panics.
func ArrayOf(length int, elem Type) Type {
	if length < 0 {
		panic("reflect: negative length passed to ArrayOf")
	}

	typ := elem.common()

	// Look in cache.
	ckey := cacheKey{Array, typ, nil, uintptr(length)}
	if array, ok := lookupCache.Load(ckey); ok {
		return array.(Type)
	}

	// Look in known types.
	s := "[" + strconv.Itoa(length) + "]" + stringFor(typ)
	for _, tt := range typesByString(s) {
		array := (*arrayType)(unsafe.Pointer(tt))
		if array.Elem == typ {
			ti, _ := lookupCache.LoadOrStore(ckey, toRType(tt))
			return ti.(Type)
		}
	}

	// Make an array type.
	var iarray any = [1]unsafe.Pointer{}
	prototype := *(**arrayType)(unsafe.Pointer(&iarray))
	array := *prototype
	array.TFlag = typ.TFlag & abi.TFlagRegularMemory
	array.Str = resolveReflectName(newName(s, "", false, false))
	array.Hash = fnv1(typ.Hash, '[')
	for n := uint32(length); n > 0; n >>= 8 {
		array.Hash = fnv1(array.Hash, byte(n))
	}
	array.Hash = fnv1(array.Hash, ']')
	array.Elem = typ
	array.PtrToThis = 0
	if typ.Size_ > 0 {
		max := ^uintptr(0) / typ.Size_
		if uintptr(length) > max {
			panic("reflect.ArrayOf: array size would exceed virtual address space")
		}
	}
	array.Size_ = typ.Size_ * uintptr(length)
	if length > 0 && typ.Pointers() {
		array.PtrBytes = typ.Size_*uintptr(length-1) + typ.PtrBytes
	} else {
		array.PtrBytes = 0
	}
	array.Align_ = typ.Align_
	array.FieldAlign_ = typ.FieldAlign_
	array.Len = uintptr(length)
	array.Slice = &(SliceOf(elem).(*rtype).t)

	switch {
	case array.PtrBytes == 0:
		// No pointers.
		array.GCData = nil

	case length == 1:
		// In memory, 1-element array looks just like the element.
		// We share the bitmask with the element type.
		array.TFlag |= typ.TFlag & abi.TFlagGCMaskOnDemand
		array.GCData = typ.GCData

	case array.PtrBytes <= abi.MaxPtrmaskBytes*8*goarch.PtrSize:
		// Create pointer mask by repeating the element bitmask Len times.
		n := (array.PtrBytes/goarch.PtrSize + 7) / 8
		// Runtime needs pointer masks to be a multiple of uintptr in size.
		n = (n + goarch.PtrSize - 1) &^ (goarch.PtrSize - 1)
		mask := make([]byte, n)
		emitGCMask(mask, 0, typ, array.Len)
		array.GCData = &mask[0]

	default:
		// Runtime will build the mask if needed. We just need to allocate
		// space to store it.
		array.TFlag |= abi.TFlagGCMaskOnDemand
		array.GCData = (*byte)(unsafe.Pointer(new(uintptr)))
		if runtime.GOOS == "aix" {
			array.GCData = adjustAIXGCData(array.GCData)
		}
	}

	etyp := typ
	esize := etyp.Size()

	array.Equal = nil
	if eequal := etyp.Equal; eequal != nil {
		array.Equal = func(p, q unsafe.Pointer) bool {
			for i := 0; i < length; i++ {
				pi := arrayAt(p, i, esize, "i < length")
				qi := arrayAt(q, i, esize, "i < length")
				if !eequal(pi, qi) {
					return false
				}

			}
			return true
		}
	}

	switch {
	case array.Size_ == goarch.PtrSize && array.PtrBytes == goarch.PtrSize:
		array.TFlag |= abi.TFlagDirectIface
	default:
		array.TFlag &^= abi.TFlagDirectIface
	}

	ti, _ := lookupCache.LoadOrStore(ckey, toRType(&array.Type))
	return ti.(Type)
}

// adjustAIXGCData adjusts the GCData field pointer for AIX.
// See runtime.getGCMaskOnDemand.
func adjustAIXGCData(addr *byte) *byte {
	adjusted := adjustAIXGCDataForRuntime(addr)
	if adjusted != addr {
		pinAIXGCDataMu.Lock()
		pinAIXGCData = append(pinAIXGCData, addr)
		pinAIXGCDataMu.Unlock()
	}
	return adjusted
}

// adjustAIXGCDataForRuntime adjusts the GCData field pointer
// as the runtime requires for AIX. See runtime.getGCMaskOnDemand.
//
//go:linknamestd adjustAIXGCDataForRuntime
//go:noescape
func adjustAIXGCDataForRuntime(*byte) *byte

// pinAIXGCDataMu proects pinAIXGCData.
var pinAIXGCDataMu sync.Mutex

// pinAIXGCData keeps the actual GCData pointer alive on AIX.
// On AIX we need to use adjustAIXGCData to convert the GC pointer
// to the value that the runtime expects. That means that the rtype
// no longer refers to the original pointer. This slice keeps it alive.
var pinAIXGCData []*byte

func appendVarint(x []byte, v uintptr) []byte {
	for ; v >= 0x80; v >>= 7 {
		x = append(x, byte(v|0x80))
	}
	x = append(x, byte(v))
	return x
}

// toType converts from a *rtype to a Type that can be returned
// to the client of package reflect. In gc, the only concern is that
// a nil *rtype must be replaced by a nil Type, but in gccgo this
// function takes care of ensuring that multiple *rtype for the same
// type are coalesced into a single Type.
//
// toType should be an internal detail,
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - fortio.org/log
//   - github.com/goccy/go-json
//   - github.com/goccy/go-reflect
//   - github.com/sohaha/zlsgo
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
//go:linkname toType
func toType(t *abi.Type) Type {
	if t == nil {
		return nil
	}
	return toRType(t)
}

type layoutKey struct {
	ftyp *funcType // function signature
	rcvr *abi.Type // receiver type, or nil if none
}

type layoutType struct {
	t         *abi.Type
	framePool *sync.Pool
	abid      abiDesc
}

var layoutCache sync.Map // map[layoutKey]layoutType

// funcLayout computes a struct type representing the layout of the
// stack-assigned function arguments and return values for the function
// type t.
// If rcvr != nil, rcvr specifies the type of the receiver.
// The returned type exists only for GC, so we only fill out GC relevant info.
// Currently, that's just size and the GC program. We also fill in
// the name for possible debugging use.
func funcLayout(t *funcType, rcvr *abi.Type) (frametype *abi.Type, framePool *sync.Pool, abid abiDesc) {
	if t.Kind() != abi.Func {
		panic("reflect: funcLayout of non-func type " + stringFor(&t.Type))
	}
	if rcvr != nil && rcvr.Kind() == abi.Interface {
		panic("reflect: funcLayout with interface receiver " + stringFor(rcvr))
	}
	k := layoutKey{t, rcvr}
	if lti, ok := layoutCache.Load(k); ok {
		lt := lti.(layoutType)
		return lt.t, lt.framePool, lt.abid
	}

	// Compute the ABI layout.
	abid = newAbiDesc(t, rcvr)

	// build dummy rtype holding gc program
	x := &abi.Type{
		Align_: goarch.PtrSize,
		// Don't add spill space here; it's only necessary in
		// reflectcall's frame, not in the allocated frame.
		// TODO(mknyszek): Remove this comment when register
		// spill space in the frame is no longer required.
		Size_:    align(abid.retOffset+abid.ret.stackBytes, goarch.PtrSize),
		PtrBytes: uintptr(abid.stackPtrs.n) * goarch.PtrSize,
	}
	if abid.stackPtrs.n > 0 {
		x.GCData = &abid.stackPtrs.data[0]
	}

	var s string
	if rcvr != nil {
		s = "methodargs(" + stringFor(rcvr) + ")(" + stringFor(&t.Type) + ")"
	} else {
		s = "funcargs(" + stringFor(&t.Type) + ")"
	}
	x.Str = resolveReflectName(newName(s, "", false, false))

	// cache result for future callers
	framePool = &sync.Pool{New: func() any {
		return unsafe_New(x)
	}}
	lti, _ := layoutCache.LoadOrStore(k, layoutType{
		t:         x,
		framePool: framePool,
		abid:      abid,
	})
	lt := lti.(layoutType)
	return lt.t, lt.framePool, lt.abid
}

// Note: this type must agree with runtime.bitvector.
type bitVector struct {
	n    uint32 // number of bits
	data []byte
}

// append a bit to the bitmap.
func (bv *bitVector) append(bit uint8) {
	if bv.n%(8*goarch.PtrSize) == 0 {
		// Runtime needs pointer masks to be a multiple of uintptr in size.
		// Since reflect passes bv.data directly to the runtime as a pointer mask,
		// we append a full uintptr of zeros at a time.
		for i := 0; i < goarch.PtrSize; i++ {
			bv.data = append(bv.data, 0)
		}
	}
	bv.data[bv.n/8] |= bit << (bv.n % 8)
	bv.n++
}

func addTypeBits(bv *bitVector, offset uintptr, t *abi.Type) {
	if !t.Pointers() {
		return
	}

	switch Kind(t.Kind()) {
	case Chan, Func, Map, Pointer, Slice, String, UnsafePointer:
		// 1 pointer at start of representation
		for bv.n < uint32(offset/goarch.PtrSize) {
			bv.append(0)
		}
		bv.append(1)

	case Interface:
		// 2 pointers
		for bv.n < uint32(offset/goarch.PtrSize) {
			bv.append(0)
		}
		bv.append(1)
		bv.append(1)

	case Array:
		// repeat inner type
		tt := (*arrayType)(unsafe.Pointer(t))
		for i := 0; i < int(tt.Len); i++ {
			addTypeBits(bv, offset+uintptr(i)*tt.Elem.Size_, tt.Elem)
		}

	case Struct:
		// apply fields
		tt := (*structType)(unsafe.Pointer(t))
		for i := range tt.Fields {
			f := &tt.Fields[i]
			addTypeBits(bv, offset+f.Offset, f.Typ)
		}
	}
}

```

// === FILE: references!/go/src/reflect/value.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reflect

import (
	"errors"
	"internal/abi"
	"internal/goarch"
	"internal/strconv"
	"internal/unsafeheader"
	"iter"
	"math"
	"runtime"
	"unsafe"
)

// Value is the reflection interface to a Go value.
//
// Not all methods apply to all kinds of values. Restrictions,
// if any, are noted in the documentation for each method.
// Use the Kind method to find out the kind of value before
// calling kind-specific methods. Calling a method
// inappropriate to the kind of type causes a run time panic.
//
// The zero Value represents no value.
// Its [Value.IsValid] method returns false, its Kind method returns [Invalid],
// its String method returns "<invalid Value>", and all other methods panic.
// Most functions and methods never return an invalid value.
// If one does, its documentation states the conditions explicitly.
//
// A Value can be used concurrently by multiple goroutines provided that
// the underlying Go value can be used concurrently for the equivalent
// direct operations.
//
// To compare two Values, compare the results of the Interface method.
// Using == on two Values does not compare the underlying values
// they represent.
type Value struct {
	// typ_ holds the type of the value represented by a Value.
	// Access using the typ method to avoid escape of v.
	typ_ *abi.Type

	// Pointer-valued data or, if flagIndir is set, pointer to data.
	// Valid when either flagIndir is set or typ.pointers() is true.
	ptr unsafe.Pointer

	// flag holds metadata about the value.
	//
	// The lowest five bits give the Kind of the value, mirroring typ.Kind().
	//
	// The next set of bits are flag bits:
	//	- flagStickyRO: obtained via unexported not embedded field, so read-only
	//	- flagEmbedRO: obtained via unexported embedded field, so read-only
	//	- flagIndir: val holds a pointer to the data
	//	- flagAddr: v.CanAddr is true (implies flagIndir and ptr is non-nil)
	//	- flagMethod: v is a method value.
	// If !typ.IsDirectIface(), code can assume that flagIndir is set.
	//
	// The remaining 22+ bits give a method number for method values.
	// If flag.kind() != Func, code can assume that flagMethod is unset.
	flag

	// A method value represents a curried method invocation
	// like r.Read for some receiver r. The typ+val+flag bits describe
	// the receiver r, but the flag's Kind bits say Func (methods are
	// functions), and the top bits of the flag give the method number
	// in r's type's method table.
}

type flag uintptr

const (
	flagKindWidth        = 5 // there are 27 kinds
	flagKindMask    flag = 1<<flagKindWidth - 1
	flagStickyRO    flag = 1 << 5
	flagEmbedRO     flag = 1 << 6
	flagIndir       flag = 1 << 7
	flagAddr        flag = 1 << 8
	flagMethod      flag = 1 << 9
	flagMethodShift      = 10
	flagRO          flag = flagStickyRO | flagEmbedRO
)

func (f flag) kind() Kind {
	return Kind(f & flagKindMask)
}

func (f flag) ro() flag {
	if f&flagRO != 0 {
		return flagStickyRO
	}
	return 0
}

// typ returns the *abi.Type stored in the Value. This method is fast,
// but it doesn't always return the correct type for the Value.
// See abiType and Type, which do return the correct type.
func (v Value) typ() *abi.Type {
	// Types are either static (for compiler-created types) or
	// heap-allocated but always reachable (for reflection-created
	// types, held in the central map). So there is no need to
	// escape types. noescape here help avoid unnecessary escape
	// of v.
	return (*abi.Type)(abi.NoEscape(unsafe.Pointer(v.typ_)))
}

// pointer returns the underlying pointer represented by v.
// v.Kind() must be Pointer, Map, Chan, Func, or UnsafePointer
// if v.Kind() == Pointer, the base type must not be not-in-heap.
func (v Value) pointer() unsafe.Pointer {
	if v.typ().Size() != goarch.PtrSize || !v.typ().Pointers() {
		panic("can't call pointer on a non-pointer Value")
	}
	if v.flag&flagIndir != 0 {
		return *(*unsafe.Pointer)(v.ptr)
	}
	return v.ptr
}

// packEface converts v to the empty interface.
func packEface(v Value) any {
	return *(*any)(unsafe.Pointer(&abi.EmptyInterface{
		Type: v.typ(),
		Data: packEfaceData(v),
	}))
}

// packEfaceData is a helper that packs the Data part of an interface,
// if v were to be stored in an interface.
func packEfaceData(v Value) unsafe.Pointer {
	t := v.typ()
	switch {
	case !t.IsDirectIface():
		if v.flag&flagIndir == 0 {
			panic("bad indir")
		}
		// Value is indirect, and so is the interface we're making.
		ptr := v.ptr
		if v.flag&flagAddr != 0 {
			c := unsafe_New(t)
			typedmemmove(t, c, ptr)
			ptr = c
		}
		return ptr
	case v.flag&flagIndir != 0:
		// Value is indirect, but interface is direct. We need
		// to load the data at v.ptr into the interface data word.
		return *(*unsafe.Pointer)(v.ptr)
	default:
		// Value is direct, and so is the interface.
		return v.ptr
	}
}

// unpackEface converts the empty interface i to a Value.
func unpackEface(i any) Value {
	e := (*abi.EmptyInterface)(unsafe.Pointer(&i))
	t := e.Type
	if t == nil {
		return Value{}
	}
	f := flag(t.Kind())
	if !t.IsDirectIface() {
		f |= flagIndir
	}
	return Value{t, e.Data, f}
}

// A ValueError occurs when a Value method is invoked on
// a [Value] that does not support it. Such cases are documented
// in the description of each method.
type ValueError struct {
	Method string
	Kind   Kind
}

func (e *ValueError) Error() string {
	if e.Kind == 0 {
		return "reflect: call of " + e.Method + " on zero Value"
	}
	return "reflect: call of " + e.Method + " on " + e.Kind.String() + " Value"
}

// valueMethodName returns the name of the exported calling method on Value.
func valueMethodName() string {
	var pc [5]uintptr
	n := runtime.Callers(1, pc[:])
	frames := runtime.CallersFrames(pc[:n])
	var frame runtime.Frame
	for more := true; more; {
		const prefix = "reflect.Value."
		frame, more = frames.Next()
		name := frame.Function
		if len(name) > len(prefix) && name[:len(prefix)] == prefix {
			methodName := name[len(prefix):]
			if len(methodName) > 0 && 'A' <= methodName[0] && methodName[0] <= 'Z' {
				return name
			}
		}
	}
	return "unknown method"
}

// nonEmptyInterface is the header for an interface value with methods.
type nonEmptyInterface struct {
	itab *abi.ITab
	word unsafe.Pointer
}

// mustBe panics if f's kind is not expected.
// Making this a method on flag instead of on Value
// (and embedding flag in Value) means that we can write
// the very clear v.mustBe(Bool) and have it compile into
// v.flag.mustBe(Bool), which will only bother to copy the
// single important word for the receiver.
func (f flag) mustBe(expected Kind) {
	// TODO(mvdan): use f.kind() again once mid-stack inlining gets better
	if Kind(f&flagKindMask) != expected {
		panic(&ValueError{valueMethodName(), f.kind()})
	}
}

// mustBeExported panics if f records that the value was obtained using
// an unexported field.
func (f flag) mustBeExported() {
	if f == 0 || f&flagRO != 0 {
		f.mustBeExportedSlow()
	}
}

func (f flag) mustBeExportedSlow() {
	if f == 0 {
		panic(&ValueError{valueMethodName(), Invalid})
	}
	if f&flagRO != 0 {
		panic("reflect: " + valueMethodName() + " using value obtained using unexported field")
	}
}

// mustBeAssignable panics if f records that the value is not assignable,
// which is to say that either it was obtained using an unexported field
// or it is not addressable.
func (f flag) mustBeAssignable() {
	if f&flagRO != 0 || f&flagAddr == 0 {
		f.mustBeAssignableSlow()
	}
}

func (f flag) mustBeAssignableSlow() {
	if f == 0 {
		panic(&ValueError{valueMethodName(), Invalid})
	}
	// Assignable if addressable and not read-only.
	if f&flagRO != 0 {
		panic("reflect: " + valueMethodName() + " using value obtained using unexported field")
	}
	if f&flagAddr == 0 {
		panic("reflect: " + valueMethodName() + " using unaddressable value")
	}
}

// Addr returns a pointer value representing the address of v.
// It panics if [Value.CanAddr] returns false.
// Addr is typically used to obtain a pointer to a struct field
// or slice element in order to call a method that requires a
// pointer receiver.
func (v Value) Addr() Value {
	if v.flag&flagAddr == 0 {
		panic("reflect.Value.Addr of unaddressable value")
	}
	// Preserve flagRO instead of using v.flag.ro() so that
	// v.Addr().Elem() is equivalent to v (#32772)
	fl := v.flag & flagRO
	return Value{ptrTo(v.typ()), v.ptr, fl | flag(Pointer)}
}

// Bool returns v's underlying value.
// It panics if v's kind is not [Bool].
func (v Value) Bool() bool {
	// panicNotBool is split out to keep Bool inlineable.
	if v.kind() != Bool {
		v.panicNotBool()
	}
	return *(*bool)(v.ptr)
}

func (v Value) panicNotBool() {
	v.mustBe(Bool)
}

var bytesType = rtypeOf(([]byte)(nil))

// Bytes returns v's underlying value.
// It panics if v's underlying value is not a slice of bytes or
// an addressable array of bytes.
func (v Value) Bytes() []byte {
	// bytesSlow is split out to keep Bytes inlineable for unnamed []byte.
	if v.typ_ == bytesType { // ok to use v.typ_ directly as comparison doesn't cause escape
		return *(*[]byte)(v.ptr)
	}
	return v.bytesSlow()
}

func (v Value) bytesSlow() []byte {
	switch v.kind() {
	case Slice:
		if v.typ().Elem().Kind() != abi.Uint8 {
			panic("reflect.Value.Bytes of non-byte slice")
		}
		// Slice is always bigger than a word; assume flagIndir.
		return *(*[]byte)(v.ptr)
	case Array:
		if v.typ().Elem().Kind() != abi.Uint8 {
			panic("reflect.Value.Bytes of non-byte array")
		}
		if !v.CanAddr() {
			panic("reflect.Value.Bytes of unaddressable byte array")
		}
		p := (*byte)(v.ptr)
		n := int((*arrayType)(unsafe.Pointer(v.typ())).Len)
		return unsafe.Slice(p, n)
	}
	panic(&ValueError{"reflect.Value.Bytes", v.kind()})
}

// runes returns v's underlying value.
// It panics if v's underlying value is not a slice of runes (int32s).
func (v Value) runes() []rune {
	v.mustBe(Slice)
	if v.typ().Elem().Kind() != abi.Int32 {
		panic("reflect.Value.Bytes of non-rune slice")
	}
	// Slice is always bigger than a word; assume flagIndir.
	return *(*[]rune)(v.ptr)
}

// CanAddr reports whether the value's address can be obtained with [Value.Addr].
// Such values are called addressable. A value is addressable if it is
// an element of a slice, an element of an addressable array,
// a field of an addressable struct, or the result of dereferencing a pointer.
// If CanAddr returns false, calling [Value.Addr] will panic.
func (v Value) CanAddr() bool {
	return v.flag&flagAddr != 0
}

// CanSet reports whether the value of v can be changed.
// A [Value] can be changed only if it is addressable and was not
// obtained by the use of unexported struct fields.
// If CanSet returns false, calling [Value.Set] or any type-specific
// setter (e.g., [Value.SetBool], [Value.SetInt]) will panic.
func (v Value) CanSet() bool {
	return v.flag&(flagAddr|flagRO) == flagAddr
}

// Call calls the function v with the input arguments in.
// For example, if len(in) == 3, v.Call(in) represents the Go call v(in[0], in[1], in[2]).
// Call panics if v's Kind is not [Func].
// It returns the output results as Values.
// As in Go, each input argument must be assignable to the
// type of the function's corresponding input parameter.
// If v is a variadic function, Call creates the variadic slice parameter
// itself, copying in the corresponding values.
// It panics if the Value was obtained by accessing unexported struct fields.
func (v Value) Call(in []Value) []Value {
	v.mustBe(Func)
	v.mustBeExported()
	return v.call("Call", in)
}

// CallSlice calls the variadic function v with the input arguments in,
// assigning the slice in[len(in)-1] to v's final variadic argument.
// For example, if len(in) == 3, v.CallSlice(in) represents the Go call v(in[0], in[1], in[2]...).
// CallSlice panics if v's Kind is not [Func] or if v is not variadic.
// It returns the output results as Values.
// As in Go, each input argument must be assignable to the
// type of the function's corresponding input parameter.
// It panics if the Value was obtained by accessing unexported struct fields.
func (v Value) CallSlice(in []Value) []Value {
	v.mustBe(Func)
	v.mustBeExported()
	return v.call("CallSlice", in)
}

var callGC bool // for testing; see TestCallMethodJump and TestCallArgLive

const debugReflectCall = false

func (v Value) call(op string, in []Value) []Value {
	// Get function pointer, type.
	t := (*funcType)(unsafe.Pointer(v.typ()))
	var (
		fn       unsafe.Pointer
		rcvr     Value
		rcvrtype *abi.Type
	)
	if v.flag&flagMethod != 0 {
		rcvr = v
		rcvrtype, t, fn = methodReceiver(op, v, int(v.flag)>>flagMethodShift)
	} else if v.flag&flagIndir != 0 {
		fn = *(*unsafe.Pointer)(v.ptr)
	} else {
		fn = v.ptr
	}

	if fn == nil {
		panic("reflect.Value.Call: call of nil function")
	}

	isSlice := op == "CallSlice"
	n := t.NumIn()
	isVariadic := t.IsVariadic()
	if isSlice {
		if !isVariadic {
			panic("reflect: CallSlice of non-variadic function")
		}
		if len(in) < n {
			panic("reflect: CallSlice with too few input arguments")
		}
		if len(in) > n {
			panic("reflect: CallSlice with too many input arguments")
		}
	} else {
		if isVariadic {
			n--
		}
		if len(in) < n {
			panic("reflect: Call with too few input arguments")
		}
		if !isVariadic && len(in) > n {
			panic("reflect: Call with too many input arguments")
		}
	}
	for _, x := range in {
		if x.Kind() == Invalid {
			panic("reflect: " + op + " using zero Value argument")
		}
	}
	for i := 0; i < n; i++ {
		if xt, targ := in[i].Type(), t.In(i); !xt.AssignableTo(toRType(targ)) {
			panic("reflect: " + op + " using " + xt.String() + " as type " + stringFor(targ))
		}
	}
	if !isSlice && isVariadic {
		// prepare slice for remaining values
		m := len(in) - n
		slice := MakeSlice(toRType(t.In(n)), m, m)
		elem := toRType(t.In(n)).Elem() // FIXME cast to slice type and Elem()
		for i := 0; i < m; i++ {
			x := in[n+i]
			if xt := x.Type(); !xt.AssignableTo(elem) {
				panic("reflect: cannot use " + xt.String() + " as type " + elem.String() + " in " + op)
			}
			slice.Index(i).Set(x)
		}
		origIn := in
		in = make([]Value, n+1)
		copy(in[:n], origIn)
		in[n] = slice
	}

	nin := len(in)
	if nin != t.NumIn() {
		panic("reflect.Value.Call: wrong argument count")
	}
	nout := t.NumOut()

	// Register argument space.
	var regArgs abi.RegArgs

	// Compute frame type.
	frametype, framePool, abid := funcLayout(t, rcvrtype)

	// Allocate a chunk of memory for frame if needed.
	var stackArgs unsafe.Pointer
	if frametype.Size() != 0 {
		if nout == 0 {
			stackArgs = framePool.Get().(unsafe.Pointer)
		} else {
			// Can't use pool if the function has return values.
			// We will leak pointer to args in ret, so its lifetime is not scoped.
			stackArgs = unsafe_New(frametype)
		}
	}
	frameSize := frametype.Size()

	if debugReflectCall {
		println("reflect.call", stringFor(&t.Type))
		abid.dump()
	}

	// Copy inputs into args.

	// Handle receiver.
	inStart := 0
	if rcvrtype != nil {
		// Guaranteed to only be one word in size,
		// so it will only take up exactly 1 abiStep (either
		// in a register or on the stack).
		switch st := abid.call.steps[0]; st.kind {
		case abiStepStack:
			storeRcvr(rcvr, stackArgs)
		case abiStepPointer:
			storeRcvr(rcvr, unsafe.Pointer(&regArgs.Ptrs[st.ireg]))
			fallthrough
		case abiStepIntReg:
			storeRcvr(rcvr, unsafe.Pointer(&regArgs.Ints[st.ireg]))
		case abiStepFloatReg:
			storeRcvr(rcvr, unsafe.Pointer(&regArgs.Floats[st.freg]))
		default:
			panic("unknown ABI parameter kind")
		}
		inStart = 1
	}

	// Handle arguments.
	for i, v := range in {
		v.mustBeExported()
		targ := toRType(t.In(i))
		// TODO(mknyszek): Figure out if it's possible to get some
		// scratch space for this assignment check. Previously, it
		// was possible to use space in the argument frame.
		v = v.assignTo("reflect.Value.Call", &targ.t, nil)
	stepsLoop:
		for _, st := range abid.call.stepsForValue(i + inStart) {
			switch st.kind {
			case abiStepStack:
				// Copy values to the "stack."
				addr := add(stackArgs, st.stkOff, "precomputed stack arg offset")
				if v.flag&flagIndir != 0 {
					typedmemmove(&targ.t, addr, v.ptr)
				} else {
					*(*unsafe.Pointer)(addr) = v.ptr
				}
				// There's only one step for a stack-allocated value.
				break stepsLoop
			case abiStepIntReg, abiStepPointer:
				// Copy values to "integer registers."
				if v.flag&flagIndir != 0 {
					offset := add(v.ptr, st.offset, "precomputed value offset")
					if st.kind == abiStepPointer {
						// Duplicate this pointer in the pointer area of the
						// register space. Otherwise, there's the potential for
						// this to be the last reference to v.ptr.
						regArgs.Ptrs[st.ireg] = *(*unsafe.Pointer)(offset)
					}
					intToReg(&regArgs, st.ireg, st.size, offset)
				} else {
					if st.kind == abiStepPointer {
						// See the comment in abiStepPointer case above.
						regArgs.Ptrs[st.ireg] = v.ptr
					}
					regArgs.Ints[st.ireg] = uintptr(v.ptr)
				}
			case abiStepFloatReg:
				// Copy values to "float registers."
				if v.flag&flagIndir == 0 {
					panic("attempted to copy pointer to FP register")
				}
				offset := add(v.ptr, st.offset, "precomputed value offset")
				floatToReg(&regArgs, st.freg, st.size, offset)
			default:
				panic("unknown ABI part kind")
			}
		}
	}
	// TODO(mknyszek): Remove this when we no longer have
	// caller reserved spill space.
	frameSize = align(frameSize, goarch.PtrSize)
	frameSize += abid.spill

	// Mark pointers in registers for the return path.
	regArgs.ReturnIsPtr = abid.outRegPtrs

	if debugReflectCall {
		regArgs.Dump()
	}

	// For testing; see TestCallArgLive.
	if callGC {
		runtime.GC()
	}

	// Call.
	call(frametype, fn, stackArgs, uint32(frametype.Size()), uint32(abid.retOffset), uint32(frameSize), &regArgs)

	// For testing; see TestCallMethodJump.
	if callGC {
		runtime.GC()
	}

	var ret []Value
	if nout == 0 {
		if stackArgs != nil {
			typedmemclr(frametype, stackArgs)
			framePool.Put(stackArgs)
		}
	} else {
		if stackArgs != nil {
			// Zero the now unused input area of args,
			// because the Values returned by this function contain pointers to the args object,
			// and will thus keep the args object alive indefinitely.
			typedmemclrpartial(frametype, stackArgs, 0, abid.retOffset)
		}

		// Wrap Values around return values in args.
		ret = make([]Value, nout)
		for i := 0; i < nout; i++ {
			tv := t.Out(i)
			if tv.Size() == 0 {
				// For zero-sized return value, args+off may point to the next object.
				// In this case, return the zero value instead.
				ret[i] = Zero(toRType(tv))
				continue
			}
			steps := abid.ret.stepsForValue(i)
			if st := steps[0]; st.kind == abiStepStack {
				// This value is on the stack. If part of a value is stack
				// allocated, the entire value is according to the ABI. So
				// just make an indirection into the allocated frame.
				fl := flagIndir | flag(tv.Kind())
				ret[i] = Value{tv, add(stackArgs, st.stkOff, "tv.Size() != 0"), fl}
				// Note: this does introduce false sharing between results -
				// if any result is live, they are all live.
				// (And the space for the args is live as well, but as we've
				// cleared that space it isn't as big a deal.)
				continue
			}

			// Handle pointers passed in registers.
			if tv.IsDirectIface() {
				// Pointer-valued data gets put directly
				// into v.ptr.
				if steps[0].kind != abiStepPointer {
					print("kind=", steps[0].kind, ", type=", stringFor(tv), "\n")
					panic("mismatch between ABI description and types")
				}
				ret[i] = Value{tv, regArgs.Ptrs[steps[0].ireg], flag(tv.Kind())}
				continue
			}

			// All that's left is values passed in registers that we need to
			// create space for and copy values back into.
			//
			// TODO(mknyszek): We make a new allocation for each register-allocated
			// value, but previously we could always point into the heap-allocated
			// stack frame. This is a regression that could be fixed by adding
			// additional space to the allocated stack frame and storing the
			// register-allocated return values into the allocated stack frame and
			// referring there in the resulting Value.
			s := unsafe_New(tv)
			for _, st := range steps {
				switch st.kind {
				case abiStepIntReg:
					offset := add(s, st.offset, "precomputed value offset")
					intFromReg(&regArgs, st.ireg, st.size, offset)
				case abiStepPointer:
					s := add(s, st.offset, "precomputed value offset")
					*((*unsafe.Pointer)(s)) = regArgs.Ptrs[st.ireg]
				case abiStepFloatReg:
					offset := add(s, st.offset, "precomputed value offset")
					floatFromReg(&regArgs, st.freg, st.size, offset)
				case abiStepStack:
					panic("register-based return value has stack component")
				default:
					panic("unknown ABI part kind")
				}
			}
			ret[i] = Value{tv, s, flagIndir | flag(tv.Kind())}
		}
	}

	return ret
}

// callReflect is the call implementation used by a function
// returned by MakeFunc. In many ways it is the opposite of the
// method Value.call above. The method above converts a call using Values
// into a call of a function with a concrete argument frame, while
// callReflect converts a call of a function with a concrete argument
// frame into a call using Values.
// It is in this file so that it can be next to the call method above.
// The remainder of the MakeFunc implementation is in makefunc.go.
//
// NOTE: This function must be marked as a "wrapper" in the generated code,
// so that the linker can make it work correctly for panic and recover.
// The gc compilers know to do that for the name "reflect.callReflect".
//
// ctxt is the "closure" generated by MakeFunc.
// frame is a pointer to the arguments to that closure on the stack.
// retValid points to a boolean which should be set when the results
// section of frame is set.
//
// regs contains the argument values passed in registers and will contain
// the values returned from ctxt.fn in registers.
func callReflect(ctxt *makeFuncImpl, frame unsafe.Pointer, retValid *bool, regs *abi.RegArgs) {
	if callGC {
		// Call GC upon entry during testing.
		// Getting our stack scanned here is the biggest hazard, because
		// our caller (makeFuncStub) could have failed to place the last
		// pointer to a value in regs' pointer space, in which case it
		// won't be visible to the GC.
		runtime.GC()
	}
	ftyp := ctxt.ftyp
	f := ctxt.fn

	_, _, abid := funcLayout(ftyp, nil)

	// Copy arguments into Values.
	ptr := frame
	in := make([]Value, 0, int(ftyp.InCount))
	for i, typ := range ftyp.InSlice() {
		if typ.Size() == 0 {
			in = append(in, Zero(toRType(typ)))
			continue
		}
		v := Value{typ, nil, flag(typ.Kind())}
		steps := abid.call.stepsForValue(i)
		if st := steps[0]; st.kind == abiStepStack {
			if !typ.IsDirectIface() {
				// value cannot be inlined in interface data.
				// Must make a copy, because f might keep a reference to it,
				// and we cannot let f keep a reference to the stack frame
				// after this function returns, not even a read-only reference.
				v.ptr = unsafe_New(typ)
				if typ.Size() > 0 {
					typedmemmove(typ, v.ptr, add(ptr, st.stkOff, "typ.size > 0"))
				}
				v.flag |= flagIndir
			} else {
				v.ptr = *(*unsafe.Pointer)(add(ptr, st.stkOff, "1-ptr"))
			}
		} else {
			if !typ.IsDirectIface() {
				// All that's left is values passed in registers that we need to
				// create space for the values.
				v.flag |= flagIndir
				v.ptr = unsafe_New(typ)
				for _, st := range steps {
					switch st.kind {
					case abiStepIntReg:
						offset := add(v.ptr, st.offset, "precomputed value offset")
						intFromReg(regs, st.ireg, st.size, offset)
					case abiStepPointer:
						s := add(v.ptr, st.offset, "precomputed value offset")
						*((*unsafe.Pointer)(s)) = regs.Ptrs[st.ireg]
					case abiStepFloatReg:
						offset := add(v.ptr, st.offset, "precomputed value offset")
						floatFromReg(regs, st.freg, st.size, offset)
					case abiStepStack:
						panic("register-based return value has stack component")
					default:
						panic("unknown ABI part kind")
					}
				}
			} else {
				// Pointer-valued data gets put directly
				// into v.ptr.
				if steps[0].kind != abiStepPointer {
					print("kind=", steps[0].kind, ", type=", stringFor(typ), "\n")
					panic("mismatch between ABI description and types")
				}
				v.ptr = regs.Ptrs[steps[0].ireg]
			}
		}
		in = append(in, v)
	}

	// Call underlying function.
	out := f(in)
	numOut := ftyp.NumOut()
	if len(out) != numOut {
		panic("reflect: wrong return count from function created by MakeFunc")
	}

	// Copy results back into argument frame and register space.
	if numOut > 0 {
		for i, typ := range ftyp.OutSlice() {
			v := out[i]
			if v.typ() == nil {
				panic("reflect: function created by MakeFunc using " + funcName(f) +
					" returned zero Value")
			}
			if v.flag&flagRO != 0 {
				panic("reflect: function created by MakeFunc using " + funcName(f) +
					" returned value obtained from unexported field")
			}
			if typ.Size() == 0 {
				continue
			}

			// Convert v to type typ if v is assignable to a variable
			// of type t in the language spec.
			// See issue 28761.
			//
			//
			// TODO(mknyszek): In the switch to the register ABI we lost
			// the scratch space here for the register cases (and
			// temporarily for all the cases).
			//
			// If/when this happens, take note of the following:
			//
			// We must clear the destination before calling assignTo,
			// in case assignTo writes (with memory barriers) to the
			// target location used as scratch space. See issue 39541.
			v = v.assignTo("reflect.MakeFunc", typ, nil)
		stepsLoop:
			for _, st := range abid.ret.stepsForValue(i) {
				switch st.kind {
				case abiStepStack:
					// Copy values to the "stack."
					addr := add(ptr, st.stkOff, "precomputed stack arg offset")
					// Do not use write barriers. The stack space used
					// for this call is not adequately zeroed, and we
					// are careful to keep the arguments alive until we
					// return to makeFuncStub's caller.
					if v.flag&flagIndir != 0 {
						memmove(addr, v.ptr, st.size)
					} else {
						// This case must be a pointer type.
						*(*uintptr)(addr) = uintptr(v.ptr)
					}
					// There's only one step for a stack-allocated value.
					break stepsLoop
				case abiStepIntReg, abiStepPointer:
					// Copy values to "integer registers."
					if v.flag&flagIndir != 0 {
						offset := add(v.ptr, st.offset, "precomputed value offset")
						intToReg(regs, st.ireg, st.size, offset)
					} else {
						// Only populate the Ints space on the return path.
						// This is safe because out is kept alive until the
						// end of this function, and the return path through
						// makeFuncStub has no preemption, so these pointers
						// are always visible to the GC.
						regs.Ints[st.ireg] = uintptr(v.ptr)
					}
				case abiStepFloatReg:
					// Copy values to "float registers."
					if v.flag&flagIndir == 0 {
						panic("attempted to copy pointer to FP register")
					}
					offset := add(v.ptr, st.offset, "precomputed value offset")
					floatToReg(regs, st.freg, st.size, offset)
				default:
					panic("unknown ABI part kind")
				}
			}
		}
	}

	// Announce that the return values are valid.
	// After this point the runtime can depend on the return values being valid.
	*retValid = true

	// We have to make sure that the out slice lives at least until
	// the runtime knows the return values are valid. Otherwise, the
	// return values might not be scanned by anyone during a GC.
	// (out would be dead, and the return slots not yet alive.)
	runtime.KeepAlive(out)

	// runtime.getArgInfo expects to be able to find ctxt on the
	// stack when it finds our caller, makeFuncStub. Make sure it
	// doesn't get garbage collected.
	runtime.KeepAlive(ctxt)
}

// methodReceiver returns information about the receiver
// described by v. The Value v may or may not have the
// flagMethod bit set, so the kind cached in v.flag should
// not be used.
// The return value rcvrtype gives the method's actual receiver type.
// The return value t gives the method type signature (without the receiver).
// The return value fn is a pointer to the method code.
func methodReceiver(op string, v Value, methodIndex int) (rcvrtype *abi.Type, t *funcType, fn unsafe.Pointer) {
	i := methodIndex
	if v.typ().Kind() == abi.Interface {
		tt := (*interfaceType)(unsafe.Pointer(v.typ()))
		if uint(i) >= uint(len(tt.Methods)) {
			panic("reflect: internal error: invalid method index")
		}
		m := &tt.Methods[i]
		if !tt.nameOff(m.Name).IsExported() {
			panic("reflect: " + op + " of unexported method")
		}
		iface := (*nonEmptyInterface)(v.ptr)
		if iface.itab == nil {
			panic("reflect: " + op + " of method on nil interface value")
		}
		rcvrtype = iface.itab.Type
		fn = unsafe.Pointer(&unsafe.Slice(&iface.itab.Fun[0], i+1)[i])
		t = (*funcType)(unsafe.Pointer(tt.typeOff(m.Typ)))
	} else {
		rcvrtype = v.typ()
		ms := v.typ().ExportedMethods()
		if uint(i) >= uint(len(ms)) {
			panic("reflect: internal error: invalid method index")
		}
		m := ms[i]
		if !nameOffFor(v.typ(), m.Name).IsExported() {
			panic("reflect: " + op + " of unexported method")
		}
		ifn := textOffFor(v.typ(), m.Ifn)
		fn = unsafe.Pointer(&ifn)
		t = (*funcType)(unsafe.Pointer(typeOffFor(v.typ(), m.Mtyp)))
	}
	return
}

// v is a method receiver. Store at p the word which is used to
// encode that receiver at the start of the argument list.
// Reflect uses the "interface" calling convention for
// methods, which always uses one word to record the receiver.
func storeRcvr(v Value, p unsafe.Pointer) {
	t := v.typ()
	if t.Kind() == abi.Interface {
		// the interface data word becomes the receiver word
		iface := (*nonEmptyInterface)(v.ptr)
		*(*unsafe.Pointer)(p) = iface.word
	} else if v.flag&flagIndir != 0 && t.IsDirectIface() {
		*(*unsafe.Pointer)(p) = *(*unsafe.Pointer)(v.ptr)
	} else {
		*(*unsafe.Pointer)(p) = v.ptr
	}
}

// align returns the result of rounding x up to a multiple of n.
// n must be a power of two.
func align(x, n uintptr) uintptr {
	return (x + n - 1) &^ (n - 1)
}

// callMethod is the call implementation used by a function returned
// by makeMethodValue (used by v.Method(i).Interface()).
// It is a streamlined version of the usual reflect call: the caller has
// already laid out the argument frame for us, so we don't have
// to deal with individual Values for each argument.
// It is in this file so that it can be next to the two similar functions above.
// The remainder of the makeMethodValue implementation is in makefunc.go.
//
// NOTE: This function must be marked as a "wrapper" in the generated code,
// so that the linker can make it work correctly for panic and recover.
// The gc compilers know to do that for the name "reflect.callMethod".
//
// ctxt is the "closure" generated by makeMethodValue.
// frame is a pointer to the arguments to that closure on the stack.
// retValid points to a boolean which should be set when the results
// section of frame is set.
//
// regs contains the argument values passed in registers and will contain
// the values returned from ctxt.fn in registers.
func callMethod(ctxt *methodValue, frame unsafe.Pointer, retValid *bool, regs *abi.RegArgs) {
	rcvr := ctxt.rcvr
	rcvrType, valueFuncType, methodFn := methodReceiver("call", rcvr, ctxt.method)

	// There are two ABIs at play here.
	//
	// methodValueCall was invoked with the ABI assuming there was no
	// receiver ("value ABI") and that's what frame and regs are holding.
	//
	// Meanwhile, we need to actually call the method with a receiver, which
	// has its own ABI ("method ABI"). Everything that follows is a translation
	// between the two.
	_, _, valueABI := funcLayout(valueFuncType, nil)
	valueFrame, valueRegs := frame, regs
	methodFrameType, methodFramePool, methodABI := funcLayout(valueFuncType, rcvrType)

	// Make a new frame that is one word bigger so we can store the receiver.
	// This space is used for both arguments and return values.
	methodFrame := methodFramePool.Get().(unsafe.Pointer)
	var methodRegs abi.RegArgs

	// Deal with the receiver. It's guaranteed to only be one word in size.
	switch st := methodABI.call.steps[0]; st.kind {
	case abiStepStack:
		// Only copy the receiver to the stack if the ABI says so.
		// Otherwise, it'll be in a register already.
		storeRcvr(rcvr, methodFrame)
	case abiStepPointer:
		// Put the receiver in a register.
		storeRcvr(rcvr, unsafe.Pointer(&methodRegs.Ptrs[st.ireg]))
		fallthrough
	case abiStepIntReg:
		storeRcvr(rcvr, unsafe.Pointer(&methodRegs.Ints[st.ireg]))
	case abiStepFloatReg:
		storeRcvr(rcvr, unsafe.Pointer(&methodRegs.Floats[st.freg]))
	default:
		panic("unknown ABI parameter kind")
	}

	// Translate the rest of the arguments.
	for i, t := range valueFuncType.InSlice() {
		valueSteps := valueABI.call.stepsForValue(i)
		methodSteps := methodABI.call.stepsForValue(i + 1)

		// Zero-sized types are trivial: nothing to do.
		if len(valueSteps) == 0 {
			if len(methodSteps) != 0 {
				panic("method ABI and value ABI do not align")
			}
			continue
		}

		// There are four cases to handle in translating each
		// argument:
		// 1. Stack -> stack translation.
		// 2. Stack -> registers translation.
		// 3. Registers -> stack translation.
		// 4. Registers -> registers translation.

		// If the value ABI passes the value on the stack,
		// then the method ABI does too, because it has strictly
		// fewer arguments. Simply copy between the two.
		if vStep := valueSteps[0]; vStep.kind == abiStepStack {
			mStep := methodSteps[0]
			// Handle stack -> stack translation.
			if mStep.kind == abiStepStack {
				if vStep.size != mStep.size {
					panic("method ABI and value ABI do not align")
				}
				typedmemmove(t,
					add(methodFrame, mStep.stkOff, "precomputed stack offset"),
					add(valueFrame, vStep.stkOff, "precomputed stack offset"))
				continue
			}
			// Handle stack -> register translation.
			for _, mStep := range methodSteps {
				from := add(valueFrame, vStep.stkOff+mStep.offset, "precomputed stack offset")
				switch mStep.kind {
				case abiStepPointer:
					// Do the pointer copy directly so we get a write barrier.
					methodRegs.Ptrs[mStep.ireg] = *(*unsafe.Pointer)(from)
					fallthrough // We need to make sure this ends up in Ints, too.
				case abiStepIntReg:
					intToReg(&methodRegs, mStep.ireg, mStep.size, from)
				case abiStepFloatReg:
					floatToReg(&methodRegs, mStep.freg, mStep.size, from)
				default:
					panic("unexpected method step")
				}
			}
			continue
		}
		// Handle register -> stack translation.
		if mStep := methodSteps[0]; mStep.kind == abiStepStack {
			for _, vStep := range valueSteps {
				to := add(methodFrame, mStep.stkOff+vStep.offset, "precomputed stack offset")
				switch vStep.kind {
				case abiStepPointer:
					// Do the pointer copy directly so we get a write barrier.
					*(*unsafe.Pointer)(to) = valueRegs.Ptrs[vStep.ireg]
				case abiStepIntReg:
					intFromReg(valueRegs, vStep.ireg, vStep.size, to)
				case abiStepFloatReg:
					floatFromReg(valueRegs, vStep.freg, vStep.size, to)
				default:
					panic("unexpected value step")
				}
			}
			continue
		}
		// Handle register -> register translation.
		if len(valueSteps) != len(methodSteps) {
			// Because it's the same type for the value, and it's assigned
			// to registers both times, it should always take up the same
			// number of registers for each ABI.
			panic("method ABI and value ABI don't align")
		}
		for i, vStep := range valueSteps {
			mStep := methodSteps[i]
			if mStep.kind != vStep.kind {
				panic("method ABI and value ABI don't align")
			}
			switch vStep.kind {
			case abiStepPointer:
				// Copy this too, so we get a write barrier.
				methodRegs.Ptrs[mStep.ireg] = valueRegs.Ptrs[vStep.ireg]
				fallthrough
			case abiStepIntReg:
				methodRegs.Ints[mStep.ireg] = valueRegs.Ints[vStep.ireg]
			case abiStepFloatReg:
				methodRegs.Floats[mStep.freg] = valueRegs.Floats[vStep.freg]
			default:
				panic("unexpected value step")
			}
		}
	}

	methodFrameSize := methodFrameType.Size()
	// TODO(mknyszek): Remove this when we no longer have
	// caller reserved spill space.
	methodFrameSize = align(methodFrameSize, goarch.PtrSize)
	methodFrameSize += methodABI.spill

	// Mark pointers in registers for the return path.
	methodRegs.ReturnIsPtr = methodABI.outRegPtrs

	// Call.
	// Call copies the arguments from scratch to the stack, calls fn,
	// and then copies the results back into scratch.
	call(methodFrameType, methodFn, methodFrame, uint32(methodFrameType.Size()), uint32(methodABI.retOffset), uint32(methodFrameSize), &methodRegs)

	// Copy return values.
	//
	// This is somewhat simpler because both ABIs have an identical
	// return value ABI (the types are identical). As a result, register
	// results can simply be copied over. Stack-allocated values are laid
	// out the same, but are at different offsets from the start of the frame
	// Ignore any changes to args.
	// Avoid constructing out-of-bounds pointers if there are no return values.
	// because the arguments may be laid out differently.
	if valueRegs != nil {
		*valueRegs = methodRegs
	}
	if retSize := methodFrameType.Size() - methodABI.retOffset; retSize > 0 {
		valueRet := add(valueFrame, valueABI.retOffset, "valueFrame's size > retOffset")
		methodRet := add(methodFrame, methodABI.retOffset, "methodFrame's size > retOffset")
		// This copies to the stack. Write barriers are not needed.
		memmove(valueRet, methodRet, retSize)
	}

	// Tell the runtime it can now depend on the return values
	// being properly initialized.
	*retValid = true

	// Clear the scratch space and put it back in the pool.
	// This must happen after the statement above, so that the return
	// values will always be scanned by someone.
	typedmemclr(methodFrameType, methodFrame)
	methodFramePool.Put(methodFrame)

	// See the comment in callReflect.
	runtime.KeepAlive(ctxt)

	// Keep valueRegs alive because it may hold live pointer results.
	// The caller (methodValueCall) has it as a stack object, which is only
	// scanned when there is a reference to it.
	runtime.KeepAlive(valueRegs)
}

// funcName returns the name of f, for use in error messages.
func funcName(f func([]Value) []Value) string {
	pc := *(*uintptr)(unsafe.Pointer(&f))
	rf := runtime.FuncForPC(pc)
	if rf != nil {
		return rf.Name()
	}
	return "closure"
}

// Cap returns v's capacity.
// It panics if v's Kind is not [Array], [Chan], [Slice] or pointer to [Array].
func (v Value) Cap() int {
	// capNonSlice is split out to keep Cap inlineable for slice kinds.
	if v.kind() == Slice {
		return (*unsafeheader.Slice)(v.ptr).Cap
	}
	return v.capNonSlice()
}

func (v Value) capNonSlice() int {
	k := v.kind()
	switch k {
	case Array:
		return v.typ().Len()
	case Chan:
		return chancap(v.pointer())
	case Ptr:
		if v.typ().Elem().Kind() == abi.Array {
			return v.typ().Elem().Len()
		}
		panic("reflect: call of reflect.Value.Cap on ptr to non-array Value")
	}
	panic(&ValueError{"reflect.Value.Cap", v.kind()})
}

// Close closes the channel v.
// It panics if v's Kind is not [Chan] or
// v is a receive-only channel.
func (v Value) Close() {
	v.mustBe(Chan)
	v.mustBeExported()
	tt := (*chanType)(unsafe.Pointer(v.typ()))
	if ChanDir(tt.Dir)&SendDir == 0 {
		panic("reflect: close of receive-only channel")
	}

	chanclose(v.pointer())
}

// CanComplex reports whether [Value.Complex] can be used without panicking.
func (v Value) CanComplex() bool {
	switch v.kind() {
	case Complex64, Complex128:
		return true
	default:
		return false
	}
}

// Complex returns v's underlying value, as a complex128.
// It panics if v's Kind is not [Complex64] or [Complex128]
func (v Value) Complex() complex128 {
	k := v.kind()
	switch k {
	case Complex64:
		return complex128(*(*complex64)(v.ptr))
	case Complex128:
		return *(*complex128)(v.ptr)
	}
	panic(&ValueError{"reflect.Value.Complex", v.kind()})
}

// Elem returns the value that the interface v contains
// or that the pointer v points to.
// It panics if v's Kind is not [Interface] or [Pointer].
// It returns the zero Value if v is nil.
func (v Value) Elem() Value {
	k := v.kind()
	switch k {
	case Interface:
		x := unpackEface(packIfaceValueIntoEmptyIface(v))
		if x.flag != 0 {
			x.flag |= v.flag.ro()
		}
		return x
	case Pointer:
		ptr := v.ptr
		if v.flag&flagIndir != 0 {
			if !v.typ().IsDirectIface() {
				// This is a pointer to a not-in-heap object. ptr points to a uintptr
				// in the heap. That uintptr is the address of a not-in-heap object.
				// In general, pointers to not-in-heap objects can be total junk.
				// But Elem() is asking to dereference it, so the user has asserted
				// that at least it is a valid pointer (not just an integer stored in
				// a pointer slot). So let's check, to make sure that it isn't a pointer
				// that the runtime will crash on if it sees it during GC or write barriers.
				// Since it is a not-in-heap pointer, all pointers to the heap are
				// forbidden! That makes the test pretty easy.
				// See issue 48399.
				if !verifyNotInHeapPtr(*(*uintptr)(ptr)) {
					panic("reflect: reflect.Value.Elem on an invalid notinheap pointer")
				}
			}
			ptr = *(*unsafe.Pointer)(ptr)
		}
		// The returned value's address is v's value.
		if ptr == nil {
			return Value{}
		}
		tt := (*ptrType)(unsafe.Pointer(v.typ()))
		typ := tt.Elem
		fl := v.flag&flagRO | flagIndir | flagAddr
		fl |= flag(typ.Kind())
		return Value{typ, ptr, fl}
	}
	panic(&ValueError{"reflect.Value.Elem", v.kind()})
}

// Field returns the i'th field of the struct v.
// It panics if v's Kind is not [Struct] or i is out of range.
func (v Value) Field(i int) Value {
	if v.kind() != Struct {
		panic(&ValueError{"reflect.Value.Field", v.kind()})
	}
	tt := (*structType)(unsafe.Pointer(v.typ()))
	if uint(i) >= uint(len(tt.Fields)) {
		panic("reflect: Field index out of range")
	}
	field := &tt.Fields[i]
	typ := field.Typ

	// Inherit permission bits from v, but clear flagEmbedRO.
	fl := v.flag&(flagStickyRO|flagIndir|flagAddr) | flag(typ.Kind())
	// Using an unexported field forces flagRO.
	if !field.Name.IsExported() {
		if field.Embedded() {
			fl |= flagEmbedRO
		} else {
			fl |= flagStickyRO
		}
	}
	if fl&flagIndir == 0 && typ.Size() == 0 {
		// Special case for picking a field out of a direct struct.
		// A direct struct must have a pointer field and possibly a
		// bunch of zero-sized fields. We must return the zero-sized
		// fields indirectly, as only ptr-shaped things can be direct.
		// See issue 74935.
		// We use &zeroVal[0] instead of v.ptr as it doesn't matter and
		// we can avoid pinning a possibly now-unused object.
		// Don't use nil, see issue 77779.
		return Value{typ, unsafe.Pointer(&zeroVal[0]), fl | flagIndir}
	}

	// Either flagIndir is set and v.ptr points at struct,
	// or flagIndir is not set and v.ptr is the actual struct data.
	// In the former case, we want v.ptr + offset.
	// In the latter case, we must have field.offset = 0,
	// so v.ptr + field.offset is still the correct address.
	ptr := add(v.ptr, field.Offset, "same as non-reflect &v.field")
	return Value{typ, ptr, fl}
}

// FieldByIndex returns the nested field corresponding to index.
// It panics if evaluation requires stepping through a nil
// pointer or a field that is not a struct.
func (v Value) FieldByIndex(index []int) Value {
	if len(index) == 1 {
		return v.Field(index[0])
	}
	v.mustBe(Struct)
	for i, x := range index {
		if i > 0 {
			if v.Kind() == Pointer && v.typ().Elem().Kind() == abi.Struct {
				if v.IsNil() {
					panic("reflect: indirection through nil pointer to embedded struct")
				}
				v = v.Elem()
			}
		}
		v = v.Field(x)
	}
	return v
}

// FieldByIndexErr returns the nested field corresponding to index.
// It returns an error if evaluation requires stepping through a nil
// pointer, but panics if it must step through a field that
// is not a struct.
func (v Value) FieldByIndexErr(index []int) (Value, error) {
	if len(index) == 1 {
		return v.Field(index[0]), nil
	}
	v.mustBe(Struct)
	for i, x := range index {
		if i > 0 {
			if v.Kind() == Ptr && v.typ().Elem().Kind() == abi.Struct {
				if v.IsNil() {
					return Value{}, errors.New("reflect: indirection through nil pointer to embedded struct field " + nameFor(v.typ().Elem()))
				}
				v = v.Elem()
			}
		}
		v = v.Field(x)
	}
	return v, nil
}

// FieldByName returns the struct field with the given name.
// It returns the zero Value if no field was found.
// It panics if v's Kind is not [Struct].
func (v Value) FieldByName(name string) Value {
	v.mustBe(Struct)
	if f, ok := toRType(v.typ()).FieldByName(name); ok {
		return v.FieldByIndex(f.Index)
	}
	return Value{}
}

// FieldByNameFunc returns the struct field with a name
// that satisfies the match function.
// It panics if v's Kind is not [Struct].
// It returns the zero Value if no field was found.
func (v Value) FieldByNameFunc(match func(string) bool) Value {
	if f, ok := toRType(v.typ()).FieldByNameFunc(match); ok {
		return v.FieldByIndex(f.Index)
	}
	return Value{}
}

// CanFloat reports whether [Value.Float] can be used without panicking.
func (v Value) CanFloat() bool {
	switch v.kind() {
	case Float32, Float64:
		return true
	default:
		return false
	}
}

// Float returns v's underlying value, as a float64.
// It panics if v's Kind is not [Float32] or [Float64]
func (v Value) Float() float64 {
	k := v.kind()
	switch k {
	case Float32:
		return float64(*(*float32)(v.ptr))
	case Float64:
		return *(*float64)(v.ptr)
	}
	panic(&ValueError{"reflect.Value.Float", v.kind()})
}

var uint8Type = rtypeOf(uint8(0))

// Index returns v's i'th element.
// It panics if v's Kind is not [Array], [Slice], or [String] or i is out of range.
func (v Value) Index(i int) Value {
	switch v.kind() {
	case Array:
		tt := (*arrayType)(unsafe.Pointer(v.typ()))
		if uint(i) >= uint(tt.Len) {
			panic("reflect: array index out of range")
		}
		typ := tt.Elem
		offset := uintptr(i) * typ.Size()

		// Either flagIndir is set and v.ptr points at array,
		// or flagIndir is not set and v.ptr is the actual array data.
		// In the former case, we want v.ptr + offset.
		// In the latter case, we must be doing Index(0), so offset = 0,
		// so v.ptr + offset is still the correct address.
		val := add(v.ptr, offset, "same as &v[i], i < tt.len")
		fl := v.flag&(flagIndir|flagAddr) | v.flag.ro() | flag(typ.Kind()) // bits same as overall array
		return Value{typ, val, fl}

	case Slice:
		// Element flag same as Elem of Pointer.
		// Addressable, indirect, possibly read-only.
		s := (*unsafeheader.Slice)(v.ptr)
		if uint(i) >= uint(s.Len) {
			panic("reflect: slice index out of range")
		}
		tt := (*sliceType)(unsafe.Pointer(v.typ()))
		typ := tt.Elem
		val := arrayAt(s.Data, i, typ.Size(), "i < s.Len")
		fl := flagAddr | flagIndir | v.flag.ro() | flag(typ.Kind())
		return Value{typ, val, fl}

	case String:
		s := (*unsafeheader.String)(v.ptr)
		if uint(i) >= uint(s.Len) {
			panic("reflect: string index out of range")
		}
		p := arrayAt(s.Data, i, 1, "i < s.Len")
		fl := v.flag.ro() | flag(Uint8) | flagIndir
		return Value{uint8Type, p, fl}
	}
	panic(&ValueError{"reflect.Value.Index", v.kind()})
}

// CanInt reports whether Int can be used without panicking.
func (v Value) CanInt() bool {
	switch v.kind() {
	case Int, Int8, Int16, Int32, Int64:
		return true
	default:
		return false
	}
}

// Int returns v's underlying value, as an int64.
// It panics if v's Kind is not [Int], [Int8], [Int16], [Int32], or [Int64].
func (v Value) Int() int64 {
	k := v.kind()
	p := v.ptr
	switch k {
	case Int:
		return int64(*(*int)(p))
	case Int8:
		return int64(*(*int8)(p))
	case Int16:
		return int64(*(*int16)(p))
	case Int32:
		return int64(*(*int32)(p))
	case Int64:
		return *(*int64)(p)
	}
	panic(&ValueError{"reflect.Value.Int", v.kind()})
}

// CanInterface reports whether [Value.Interface] can be used without panicking.
func (v Value) CanInterface() bool {
	if v.flag == 0 {
		panic(&ValueError{"reflect.Value.CanInterface", Invalid})
	}
	return v.flag&flagRO == 0
}

// Interface returns v's current value as an interface{}.
// It is equivalent to:
//
//	var i interface{} = (v's underlying value)
//
// It panics if the Value was obtained by accessing
// unexported struct fields.
func (v Value) Interface() (i any) {
	return valueInterface(v, true)
}

func valueInterface(v Value, safe bool) any {
	if v.flag == 0 {
		panic(&ValueError{"reflect.Value.Interface", Invalid})
	}
	if safe && v.flag&flagRO != 0 {
		// Do not allow access to unexported values via Interface,
		// because they might be pointers that should not be
		// writable or methods or function that should not be callable.
		panic("reflect.Value.Interface: cannot return value obtained from unexported field or method")
	}
	if v.flag&flagMethod != 0 {
		v = makeMethodValue("Interface", v)
	}

	if v.kind() == Interface {
		// Special case: return the element inside the interface.
		return packIfaceValueIntoEmptyIface(v)
	}

	return packEface(v)
}

// TypeAssert is semantically equivalent to:
//
//	v2, ok := v.Interface().(T)
//
// Note that this function, just as the type assertion above, might return:
//
//   - ok == false when v.Type() == reflect.TypeFor[T]()
//     For example, when both T and v are interface types and v.IsNil() == true.
//     In that case v.Interface() returns a nil interface value, and the
//     assertion .(T) fails with ok == false.
//
//   - ok == true when v.Type() != reflect.TypeFor[T]().
//     For example, when T is an interface type and v holds a value whose
//     concrete type implements T.
func TypeAssert[T any](v Value) (T, bool) {
	if v.flag == 0 {
		panic(&ValueError{"reflect.TypeAssert", Invalid})
	}
	if v.flag&flagRO != 0 {
		// Do not allow access to unexported values via TypeAssert,
		// because they might be pointers that should not be
		// writable or methods or function that should not be callable.
		panic("reflect.TypeAssert: cannot return value obtained from unexported field or method")
	}

	if v.flag&flagMethod != 0 {
		v = makeMethodValue("TypeAssert", v)
	}

	typ := abi.TypeFor[T]()

	// If v is an interface, return the element inside the interface.
	//
	// T is a concrete type and v is an interface. For example:
	//
	//	var v any = int(1)
	//	val := ValueOf(&v).Elem()
	//	TypeAssert[int](val) == val.Interface().(int)
	//
	// T is a interface and v is a non-nil interface value. For example:
	//
	//	var v any = &someError{}
	//	val := ValueOf(&v).Elem()
	//	TypeAssert[error](val) == val.Interface().(error)
	//
	// T is a interface and v is a nil interface value. For example:
	//
	//	var v error = nil
	//	val := ValueOf(&v).Elem()
	//	TypeAssert[error](val) == val.Interface().(error)
	if v.kind() == Interface {
		v, ok := packIfaceValueIntoEmptyIface(v).(T)
		return v, ok
	}

	// If T is an interface and v is a concrete type. For example:
	//
	//	TypeAssert[any](ValueOf(1)) == ValueOf(1).Interface().(any)
	//	TypeAssert[error](ValueOf(&someError{})) == ValueOf(&someError{}).Interface().(error)
	if typ.Kind() == abi.Interface {
		// To avoid allocating memory, in case the type assertion fails,
		// first do the type assertion with a nil Data pointer.
		iface := *(*any)(unsafe.Pointer(&abi.EmptyInterface{Type: v.typ(), Data: nil}))
		if out, ok := iface.(T); ok {
			// Now populate the Data field properly, we update the Data ptr
			// directly to avoid an additional type asertion. We can re-use the
			// itab we already got from the runtime (through the previous type assertion).
			(*abi.CommonInterface)(unsafe.Pointer(&out)).Data = packEfaceData(v)
			return out, true
		}
		var zero T
		return zero, false
	}

	// Both v and T must be concrete types.
	// The only way for an type-assertion to match is if the types are equal.
	if typ != v.typ() {
		var zero T
		return zero, false
	}
	if v.flag&flagIndir == 0 {
		return *(*T)(unsafe.Pointer(&v.ptr)), true
	}
	return *(*T)(v.ptr), true
}

// packIfaceValueIntoEmptyIface converts an interface Value into an empty interface.
//
// Precondition: v.kind() == Interface
func packIfaceValueIntoEmptyIface(v Value) any {
	// Empty interface has one layout, all interfaces with
	// methods have a second layout.
	if v.NumMethod() == 0 {
		return *(*any)(v.ptr)
	}
	return *(*interface {
		M()
	})(v.ptr)
}

// InterfaceData returns a pair of unspecified uintptr values.
// It panics if v's Kind is not Interface.
//
// In earlier versions of Go, this function returned the interface's
// value as a uintptr pair. As of Go 1.4, the implementation of
// interface values precludes any defined use of InterfaceData.
//
// Deprecated: The memory representation of interface values is not
// compatible with InterfaceData.
func (v Value) InterfaceData() [2]uintptr {
	v.mustBe(Interface)
	// The compiler loses track as it converts to uintptr. Force escape.
	escapes(v.ptr)
	// We treat this as a read operation, so we allow
	// it even for unexported data, because the caller
	// has to import "unsafe" to turn it into something
	// that can be abused.
	// Interface value is always bigger than a word; assume flagIndir.
	return *(*[2]uintptr)(v.ptr)
}

// IsNil reports whether its argument v is nil. The argument must be
// a chan, func, interface, map, pointer, or slice value; if it is
// not, IsNil panics. Note that IsNil is not always equivalent to a
// regular comparison with nil in Go. For example, if v was created
// by calling [ValueOf] with an uninitialized interface variable i,
// i==nil will be true but v.IsNil will panic as v will be the zero
// Value.
func (v Value) IsNil() bool {
	k := v.kind()
	switch k {
	case Chan, Func, Map, Pointer, UnsafePointer:
		if v.flag&flagMethod != 0 {
			return false
		}
		ptr := v.ptr
		if v.flag&flagIndir != 0 {
			ptr = *(*unsafe.Pointer)(ptr)
		}
		return ptr == nil
	case Interface, Slice:
		// Both interface and slice are nil if first word is 0.
		// Both are always bigger than a word; assume flagIndir.
		return *(*unsafe.Pointer)(v.ptr) == nil
	}
	panic(&ValueError{"reflect.Value.IsNil", v.kind()})
}

// IsValid reports whether v represents a value.
// It returns false if v is the zero Value.
// If [Value.IsValid] returns false, all other methods except String panic.
// Most functions and methods never return an invalid Value.
// If one does, its documentation states the conditions explicitly.
func (v Value) IsValid() bool {
	return v.flag != 0
}

// IsZero reports whether v is the zero value for its type.
// It panics if the argument is invalid.
func (v Value) IsZero() bool {
	switch v.kind() {
	case Bool:
		return !v.Bool()
	case Int, Int8, Int16, Int32, Int64:
		return v.Int() == 0
	case Uint, Uint8, Uint16, Uint32, Uint64, Uintptr:
		return v.Uint() == 0
	case Float32, Float64:
		return v.Float() == 0
	case Complex64, Complex128:
		return v.Complex() == 0
	case Array:
		if v.flag&flagIndir == 0 {
			return v.ptr == nil
		}
		if v.ptr == unsafe.Pointer(&zeroVal[0]) {
			return true
		}
		typ := (*abi.ArrayType)(unsafe.Pointer(v.typ()))
		// If the type is comparable, then compare directly with zero.
		if typ.Equal != nil && typ.Size() <= abi.ZeroValSize {
			// v.ptr doesn't escape, as Equal functions are compiler generated
			// and never escape. The escape analysis doesn't know, as it is a
			// function pointer call.
			return typ.Equal(abi.NoEscape(v.ptr), unsafe.Pointer(&zeroVal[0]))
		}
		if typ.TFlag&abi.TFlagRegularMemory != 0 {
			// For some types where the zero value is a value where all bits of this type are 0
			// optimize it.
			return isZero(unsafe.Slice(((*byte)(v.ptr)), typ.Size()))
		}
		n := int(typ.Len)
		for i := 0; i < n; i++ {
			if !v.Index(i).IsZero() {
				return false
			}
		}
		return true
	case Chan, Func, Interface, Map, Pointer, Slice, UnsafePointer:
		return v.IsNil()
	case String:
		return v.Len() == 0
	case Struct:
		if v.flag&flagIndir == 0 {
			return v.ptr == nil
		}
		if v.ptr == unsafe.Pointer(&zeroVal[0]) {
			return true
		}
		typ := (*abi.StructType)(unsafe.Pointer(v.typ()))
		// If the type is comparable, then compare directly with zero.
		if typ.Equal != nil && typ.Size() <= abi.ZeroValSize {
			// See noescape justification above.
			return typ.Equal(abi.NoEscape(v.ptr), unsafe.Pointer(&zeroVal[0]))
		}
		if typ.TFlag&abi.TFlagRegularMemory != 0 {
			// For some types where the zero value is a value where all bits of this type are 0
			// optimize it.
			return isZero(unsafe.Slice(((*byte)(v.ptr)), typ.Size()))
		}

		n := v.NumField()
		for i := 0; i < n; i++ {
			if !v.Field(i).IsZero() && v.Type().Field(i).Name != "_" {
				return false
			}
		}
		return true
	default:
		// This should never happen, but will act as a safeguard for later,
		// as a default value doesn't makes sense here.
		panic(&ValueError{"reflect.Value.IsZero", v.Kind()})
	}
}

// isZero For all zeros, performance is not as good as
// return bytealg.Count(b, byte(0)) == len(b)
func isZero(b []byte) bool {
	if len(b) == 0 {
		return true
	}
	const n = 32
	// Align memory addresses to 8 bytes.
	for uintptr(unsafe.Pointer(&b[0]))%8 != 0 {
		if b[0] != 0 {
			return false
		}
		b = b[1:]
		if len(b) == 0 {
			return true
		}
	}
	for len(b)%8 != 0 {
		if b[len(b)-1] != 0 {
			return false
		}
		b = b[:len(b)-1]
	}
	if len(b) == 0 {
		return true
	}
	w := unsafe.Slice((*uint64)(unsafe.Pointer(&b[0])), len(b)/8)
	for len(w)%n != 0 {
		if w[0] != 0 {
			return false
		}
		w = w[1:]
	}
	for len(w) >= n {
		if w[0] != 0 || w[1] != 0 || w[2] != 0 || w[3] != 0 ||
			w[4] != 0 || w[5] != 0 || w[6] != 0 || w[7] != 0 ||
			w[8] != 0 || w[9] != 0 || w[10] != 0 || w[11] != 0 ||
			w[12] != 0 || w[13] != 0 || w[14] != 0 || w[15] != 0 ||
			w[16] != 0 || w[17] != 0 || w[18] != 0 || w[19] != 0 ||
			w[20] != 0 || w[21] != 0 || w[22] != 0 || w[23] != 0 ||
			w[24] != 0 || w[25] != 0 || w[26] != 0 || w[27] != 0 ||
			w[28] != 0 || w[29] != 0 || w[30] != 0 || w[31] != 0 {
			return false
		}
		w = w[n:]
	}
	return true
}

// SetZero sets v to be the zero value of v's type.
// It panics if [Value.CanSet] returns false.
func (v Value) SetZero() {
	v.mustBeAssignable()
	switch v.kind() {
	case Bool:
		*(*bool)(v.ptr) = false
	case Int:
		*(*int)(v.ptr) = 0
	case Int8:
		*(*int8)(v.ptr) = 0
	case Int16:
		*(*int16)(v.ptr) = 0
	case Int32:
		*(*int32)(v.ptr) = 0
	case Int64:
		*(*int64)(v.ptr) = 0
	case Uint:
		*(*uint)(v.ptr) = 0
	case Uint8:
		*(*uint8)(v.ptr) = 0
	case Uint16:
		*(*uint16)(v.ptr) = 0
	case Uint32:
		*(*uint32)(v.ptr) = 0
	case Uint64:
		*(*uint64)(v.ptr) = 0
	case Uintptr:
		*(*uintptr)(v.ptr) = 0
	case Float32:
		*(*float32)(v.ptr) = 0
	case Float64:
		*(*float64)(v.ptr) = 0
	case Complex64:
		*(*complex64)(v.ptr) = 0
	case Complex128:
		*(*complex128)(v.ptr) = 0
	case String:
		*(*string)(v.ptr) = ""
	case Slice:
		*(*unsafeheader.Slice)(v.ptr) = unsafeheader.Slice{}
	case Interface:
		*(*abi.EmptyInterface)(v.ptr) = abi.EmptyInterface{}
	case Chan, Func, Map, Pointer, UnsafePointer:
		*(*unsafe.Pointer)(v.ptr) = nil
	case Array, Struct:
		typedmemclr(v.typ(), v.ptr)
	default:
		// This should never happen, but will act as a safeguard for later,
		// as a default value doesn't makes sense here.
		panic(&ValueError{"reflect.Value.SetZero", v.Kind()})
	}
}

// Kind returns v's Kind.
// If v is the zero Value ([Value.IsValid] returns false), Kind returns Invalid.
func (v Value) Kind() Kind {
	return v.kind()
}

// Len returns v's length.
// It panics if v's Kind is not [Array], [Chan], [Map], [Slice], [String], or pointer to [Array].
func (v Value) Len() int {
	// lenNonSlice is split out to keep Len inlineable for slice kinds.
	if v.kind() == Slice {
		return (*unsafeheader.Slice)(v.ptr).Len
	}
	return v.lenNonSlice()
}

func (v Value) lenNonSlice() int {
	switch k := v.kind(); k {
	case Array:
		tt := (*arrayType)(unsafe.Pointer(v.typ()))
		return int(tt.Len)
	case Chan:
		return chanlen(v.pointer())
	case Map:
		return maplen(v.pointer())
	case String:
		// String is bigger than a word; assume flagIndir.
		return (*unsafeheader.String)(v.ptr).Len
	case Ptr:
		if v.typ().Elem().Kind() == abi.Array {
			return v.typ().Elem().Len()
		}
		panic("reflect: call of reflect.Value.Len on ptr to non-array Value")
	}
	panic(&ValueError{"reflect.Value.Len", v.kind()})
}

// copyVal returns a Value containing the map key or value at ptr,
// allocating a new variable as needed.
func copyVal(typ *abi.Type, fl flag, ptr unsafe.Pointer) Value {
	if !typ.IsDirectIface() {
		// Copy result so future changes to the map
		// won't change the underlying value.
		c := unsafe_New(typ)
		typedmemmove(typ, c, ptr)
		return Value{typ, c, fl | flagIndir}
	}
	return Value{typ, *(*unsafe.Pointer)(ptr), fl}
}

// Method returns a function value corresponding to v's i'th method.
// The arguments to a Call on the returned function should not include
// a receiver; the returned function will always use v as the receiver.
// Method panics if i is out of range or if v is a nil interface value.
//
// Calling this method will force the linker to retain all exported methods in all packages.
// This may make the executable binary larger but will not affect execution time.
func (v Value) Method(i int) Value {
	if v.typ() == nil {
		panic(&ValueError{"reflect.Value.Method", Invalid})
	}
	if v.flag&flagMethod != 0 || uint(i) >= uint(toRType(v.typ()).NumMethod()) {
		panic("reflect: Method index out of range")
	}
	if v.typ().Kind() == abi.Interface && v.IsNil() {
		panic("reflect: Method on nil interface value")
	}
	fl := v.flag.ro() | (v.flag & flagIndir)
	fl |= flag(Func)
	fl |= flag(i)<<flagMethodShift | flagMethod
	return Value{v.typ(), v.ptr, fl}
}

// NumMethod returns the number of methods in the value's method set.
//
// For a non-interface type, it returns the number of exported methods.
//
// For an interface type, it returns the number of exported and unexported methods.
func (v Value) NumMethod() int {
	if v.typ() == nil {
		panic(&ValueError{"reflect.Value.NumMethod", Invalid})
	}
	if v.flag&flagMethod != 0 {
		return 0
	}
	return toRType(v.typ()).NumMethod()
}

// MethodByName returns a function value corresponding to the method
// of v with the given name.
// The arguments to a Call on the returned function should not include
// a receiver; the returned function will always use v as the receiver.
// It returns the zero Value if no method was found.
//
// Calling this method will cause the linker to retain all methods with this name in all packages.
// If the linker can't determine the name, it will retain all exported methods.
// This may make the executable binary larger but will not affect execution time.
func (v Value) MethodByName(name string) Value {
	if v.typ() == nil {
		panic(&ValueError{"reflect.Value.MethodByName", Invalid})
	}
	if v.flag&flagMethod != 0 {
		return Value{}
	}
	m, ok := toRType(v.typ()).MethodByName(name)
	if !ok {
		return Value{}
	}
	return v.Method(m.Index)
}

// NumField returns the number of fields in the struct v.
// It panics if v's Kind is not [Struct].
func (v Value) NumField() int {
	v.mustBe(Struct)
	tt := (*structType)(unsafe.Pointer(v.typ()))
	return len(tt.Fields)
}

// OverflowComplex reports whether the complex128 x cannot be represented by v's type.
// It panics if v's Kind is not [Complex64] or [Complex128].
func (v Value) OverflowComplex(x complex128) bool {
	k := v.kind()
	switch k {
	case Complex64:
		return overflowFloat32(real(x)) || overflowFloat32(imag(x))
	case Complex128:
		return false
	}
	panic(&ValueError{"reflect.Value.OverflowComplex", v.kind()})
}

// OverflowFloat reports whether the float64 x cannot be represented by v's type.
// It panics if v's Kind is not [Float32] or [Float64].
func (v Value) OverflowFloat(x float64) bool {
	k := v.kind()
	switch k {
	case Float32:
		return overflowFloat32(x)
	case Float64:
		return false
	}
	panic(&ValueError{"reflect.Value.OverflowFloat", v.kind()})
}

func overflowFloat32(x float64) bool {
	if x < 0 {
		x = -x
	}
	return math.MaxFloat32 < x && x <= math.MaxFloat64
}

// OverflowInt reports whether the int64 x cannot be represented by v's type.
// It panics if v's Kind is not [Int], [Int8], [Int16], [Int32], or [Int64].
func (v Value) OverflowInt(x int64) bool {
	k := v.kind()
	switch k {
	case Int, Int8, Int16, Int32, Int64:
		bitSize := v.typ().Size() * 8
		trunc := (x << (64 - bitSize)) >> (64 - bitSize)
		return x != trunc
	}
	panic(&ValueError{"reflect.Value.OverflowInt", v.kind()})
}

// OverflowUint reports whether the uint64 x cannot be represented by v's type.
// It panics if v's Kind is not [Uint], [Uintptr], [Uint8], [Uint16], [Uint32], or [Uint64].
func (v Value) OverflowUint(x uint64) bool {
	k := v.kind()
	switch k {
	case Uint, Uintptr, Uint8, Uint16, Uint32, Uint64:
		bitSize := v.typ_.Size() * 8 // ok to use v.typ_ directly as Size doesn't escape
		trunc := (x << (64 - bitSize)) >> (64 - bitSize)
		return x != trunc
	}
	panic(&ValueError{"reflect.Value.OverflowUint", v.kind()})
}

//go:nocheckptr
// This prevents inlining Value.Pointer when -d=checkptr is enabled,
// which ensures cmd/compile can recognize unsafe.Pointer(v.Pointer())
// and make an exception.

// Pointer returns v's value as a uintptr.
// It panics if v's Kind is not [Chan], [Func], [Map], [Pointer], [Slice], [String], or [UnsafePointer].
//
// If v's Kind is [Func], the returned pointer is an underlying
// code pointer, but not necessarily enough to identify a
// single function uniquely. The only guarantee is that the
// result is zero if and only if v is a nil func Value.
//
// If v's Kind is [Slice], the returned pointer is to the first
// element of the slice. If the slice is nil the returned value
// is 0.  If the slice is empty but non-nil the return value is non-zero.
//
// If v's Kind is [String], the returned pointer is to the first
// element of the underlying bytes of string.
//
// It's preferred to use uintptr(Value.UnsafePointer()) to get the equivalent result.
func (v Value) Pointer() uintptr {
	// The compiler loses track as it converts to uintptr. Force escape.
	escapes(v.ptr)

	k := v.kind()
	switch k {
	case Pointer:
		if !v.typ().Pointers() {
			val := *(*uintptr)(v.ptr)
			// Since it is a not-in-heap pointer, all pointers to the heap are
			// forbidden! See comment in Value.Elem and issue #48399.
			if !verifyNotInHeapPtr(val) {
				panic("reflect: reflect.Value.Pointer on an invalid notinheap pointer")
			}
			return val
		}
		fallthrough
	case Chan, Map, UnsafePointer:
		return uintptr(v.pointer())
	case Func:
		if v.flag&flagMethod != 0 {
			// As the doc comment says, the returned pointer is an
			// underlying code pointer but not necessarily enough to
			// identify a single function uniquely. All method expressions
			// created via reflect have the same underlying code pointer,
			// so their Pointers are equal. The function used here must
			// match the one used in makeMethodValue.
			return methodValueCallCodePtr()
		}
		p := v.pointer()
		// Non-nil func value points at data block.
		// First word of data block is actual code.
		if p != nil {
			p = *(*unsafe.Pointer)(p)
		}
		return uintptr(p)
	case Slice:
		return uintptr((*unsafeheader.Slice)(v.ptr).Data)
	case String:
		return uintptr((*unsafeheader.String)(v.ptr).Data)
	}
	panic(&ValueError{"reflect.Value.Pointer", v.kind()})
}

// Recv receives and returns a value from the channel v.
// It panics if v's Kind is not [Chan].
// The receive blocks until a value is ready.
// The boolean value ok is true if the value x corresponds to a send
// on the channel, false if it is a zero value received because the channel is closed.
func (v Value) Recv() (x Value, ok bool) {
	v.mustBe(Chan)
	v.mustBeExported()
	return v.recv(false)
}

// internal recv, possibly non-blocking (nb).
// v is known to be a channel.
func (v Value) recv(nb bool) (val Value, ok bool) {
	tt := (*chanType)(unsafe.Pointer(v.typ()))
	if ChanDir(tt.Dir)&RecvDir == 0 {
		panic("reflect: recv on send-only channel")
	}
	t := tt.Elem
	val = Value{t, nil, flag(t.Kind())}
	var p unsafe.Pointer
	if !t.IsDirectIface() {
		p = unsafe_New(t)
		val.ptr = p
		val.flag |= flagIndir
	} else {
		p = unsafe.Pointer(&val.ptr)
	}
	selected, ok := chanrecv(v.pointer(), nb, p)
	if !selected {
		val = Value{}
	}
	return
}

// Send sends x on the channel v.
// It panics if v's kind is not [Chan] or if x's type is not the same type as v's element type.
// As in Go, x's value must be assignable to the channel's element type.
func (v Value) Send(x Value) {
	v.mustBe(Chan)
	v.mustBeExported()
	v.send(x, false)
}

// internal send, possibly non-blocking.
// v is known to be a channel.
func (v Value) send(x Value, nb bool) (selected bool) {
	tt := (*chanType)(unsafe.Pointer(v.typ()))
	if ChanDir(tt.Dir)&SendDir == 0 {
		panic("reflect: send on recv-only channel")
	}
	x.mustBeExported()
	x = x.assignTo("reflect.Value.Send", tt.Elem, nil)
	var p unsafe.Pointer
	if x.flag&flagIndir != 0 {
		p = x.ptr
	} else {
		p = unsafe.Pointer(&x.ptr)
	}
	return chansend(v.pointer(), p, nb)
}

// Set assigns x to the value v.
// It panics if [Value.CanSet] returns false.
// As in Go, x's value must be assignable to v's type and
// must not be derived from an unexported field.
func (v Value) Set(x Value) {
	v.mustBeAssignable()
	x.mustBeExported() // do not let unexported x leak
	var target unsafe.Pointer
	if v.kind() == Interface {
		target = v.ptr
	}
	x = x.assignTo("reflect.Set", v.typ(), target)
	if x.flag&flagIndir != 0 {
		if x.ptr == unsafe.Pointer(&zeroVal[0]) {
			typedmemclr(v.typ(), v.ptr)
		} else {
			typedmemmove(v.typ(), v.ptr, x.ptr)
		}
	} else {
		*(*unsafe.Pointer)(v.ptr) = x.ptr
	}
}

// SetBool sets v's underlying value.
// It panics if v's Kind is not [Bool] or if [Value.CanSet] returns false.
func (v Value) SetBool(x bool) {
	v.mustBeAssignable()
	v.mustBe(Bool)
	*(*bool)(v.ptr) = x
}

// SetBytes sets v's underlying value.
// It panics if v's underlying value is not a slice of bytes
// or if [Value.CanSet] returns false.
func (v Value) SetBytes(x []byte) {
	v.mustBeAssignable()
	v.mustBe(Slice)
	if toRType(v.typ()).Elem().Kind() != Uint8 { // TODO add Elem method, fix mustBe(Slice) to return slice.
		panic("reflect.Value.SetBytes of non-byte slice")
	}
	*(*[]byte)(v.ptr) = x
}

// setRunes sets v's underlying value.
// It panics if v's underlying value is not a slice of runes (int32s)
// or if [Value.CanSet] returns false.
func (v Value) setRunes(x []rune) {
	v.mustBeAssignable()
	v.mustBe(Slice)
	if v.typ().Elem().Kind() != abi.Int32 {
		panic("reflect.Value.setRunes of non-rune slice")
	}
	*(*[]rune)(v.ptr) = x
}

// SetComplex sets v's underlying value to x.
// It panics if v's Kind is not [Complex64] or [Complex128],
// or if [Value.CanSet] returns false.
func (v Value) SetComplex(x complex128) {
	v.mustBeAssignable()
	switch k := v.kind(); k {
	default:
		panic(&ValueError{"reflect.Value.SetComplex", v.kind()})
	case Complex64:
		*(*complex64)(v.ptr) = complex64(x)
	case Complex128:
		*(*complex128)(v.ptr) = x
	}
}

// SetFloat sets v's underlying value to x.
// It panics if v's Kind is not [Float32] or [Float64],
// or if [Value.CanSet] returns false.
func (v Value) SetFloat(x float64) {
	v.mustBeAssignable()
	switch k := v.kind(); k {
	default:
		panic(&ValueError{"reflect.Value.SetFloat", v.kind()})
	case Float32:
		*(*float32)(v.ptr) = float32(x)
	case Float64:
		*(*float64)(v.ptr) = x
	}
}

// SetInt sets v's underlying value to x.
// It panics if v's Kind is not [Int], [Int8], [Int16], [Int32], or [Int64],
// or if [Value.CanSet] returns false.
func (v Value) SetInt(x int64) {
	v.mustBeAssignable()
	switch k := v.kind(); k {
	default:
		panic(&ValueError{"reflect.Value.SetInt", v.kind()})
	case Int:
		*(*int)(v.ptr) = int(x)
	case Int8:
		*(*int8)(v.ptr) = int8(x)
	case Int16:
		*(*int16)(v.ptr) = int16(x)
	case Int32:
		*(*int32)(v.ptr) = int32(x)
	case Int64:
		*(*int64)(v.ptr) = x
	}
}

// SetLen sets v's length to n.
// It panics if v's Kind is not [Slice], or if n is negative or
// greater than the capacity of the slice,
// or if [Value.CanSet] returns false.
func (v Value) SetLen(n int) {
	v.mustBeAssignable()
	v.mustBe(Slice)
	s := (*unsafeheader.Slice)(v.ptr)
	if uint(n) > uint(s.Cap) {
		panic("reflect: slice length out of range in SetLen")
	}
	s.Len = n
}

// SetCap sets v's capacity to n.
// It panics if v's Kind is not [Slice], or if n is smaller than the length or
// greater than the capacity of the slice,
// or if [Value.CanSet] returns false.
func (v Value) SetCap(n int) {
	v.mustBeAssignable()
	v.mustBe(Slice)
	s := (*unsafeheader.Slice)(v.ptr)
	if n < s.Len || n > s.Cap {
		panic("reflect: slice capacity out of range in SetCap")
	}
	s.Cap = n
}

// SetUint sets v's underlying value to x.
// It panics if v's Kind is not [Uint], [Uintptr], [Uint8], [Uint16], [Uint32], or [Uint64],
// or if [Value.CanSet] returns false.
func (v Value) SetUint(x uint64) {
	v.mustBeAssignable()
	switch k := v.kind(); k {
	default:
		panic(&ValueError{"reflect.Value.SetUint", v.kind()})
	case Uint:
		*(*uint)(v.ptr) = uint(x)
	case Uint8:
		*(*uint8)(v.ptr) = uint8(x)
	case Uint16:
		*(*uint16)(v.ptr) = uint16(x)
	case Uint32:
		*(*uint32)(v.ptr) = uint32(x)
	case Uint64:
		*(*uint64)(v.ptr) = x
	case Uintptr:
		*(*uintptr)(v.ptr) = uintptr(x)
	}
}

// SetPointer sets the [unsafe.Pointer] value v to x.
// It panics if v's Kind is not [UnsafePointer]
// or if [Value.CanSet] returns false.
func (v Value) SetPointer(x unsafe.Pointer) {
	v.mustBeAssignable()
	v.mustBe(UnsafePointer)
	*(*unsafe.Pointer)(v.ptr) = x
}

// SetString sets v's underlying value to x.
// It panics if v's Kind is not [String] or if [Value.CanSet] returns false.
func (v Value) SetString(x string) {
	v.mustBeAssignable()
	v.mustBe(String)
	*(*string)(v.ptr) = x
}

// Slice returns v[i:j].
// It panics if v's Kind is not [Array], [Slice] or [String], or if v is an unaddressable array,
// or if the indexes are out of bounds.
func (v Value) Slice(i, j int) Value {
	var (
		cap  int
		typ  *sliceType
		base unsafe.Pointer
	)
	switch kind := v.kind(); kind {
	default:
		panic(&ValueError{"reflect.Value.Slice", v.kind()})

	case Array:
		if v.flag&flagAddr == 0 {
			panic("reflect.Value.Slice: slice of unaddressable array")
		}
		tt := (*arrayType)(unsafe.Pointer(v.typ()))
		cap = int(tt.Len)
		typ = (*sliceType)(unsafe.Pointer(tt.Slice))
		base = v.ptr

	case Slice:
		typ = (*sliceType)(unsafe.Pointer(v.typ()))
		s := (*unsafeheader.Slice)(v.ptr)
		base = s.Data
		cap = s.Cap

	case String:
		s := (*unsafeheader.String)(v.ptr)
		if i < 0 || j < i || j > s.Len {
			panic("reflect.Value.Slice: string slice index out of bounds")
		}
		var t unsafeheader.String
		if i < s.Len {
			t = unsafeheader.String{Data: arrayAt(s.Data, i, 1, "i < s.Len"), Len: j - i}
		}
		return Value{v.typ(), unsafe.Pointer(&t), v.flag}
	}

	if i < 0 || j < i || j > cap {
		panic("reflect.Value.Slice: slice index out of bounds")
	}

	// Declare slice so that gc can see the base pointer in it.
	var x []unsafe.Pointer

	// Reinterpret as *unsafeheader.Slice to edit.
	s := (*unsafeheader.Slice)(unsafe.Pointer(&x))
	s.Len = j - i
	s.Cap = cap - i
	if cap-i > 0 {
		s.Data = arrayAt(base, i, typ.Elem.Size(), "i < cap")
	} else {
		// do not advance pointer, to avoid pointing beyond end of slice
		s.Data = base
	}

	fl := v.flag.ro() | flagIndir | flag(Slice)
	return Value{typ.Common(), unsafe.Pointer(&x), fl}
}

// Slice3 is the 3-index form of the slice operation: it returns v[i:j:k].
// It panics if v's Kind is not [Array] or [Slice], or if v is an unaddressable array,
// or if the indexes are out of bounds.
func (v Value) Slice3(i, j, k int) Value {
	var (
		cap  int
		typ  *sliceType
		base unsafe.Pointer
	)
	switch kind := v.kind(); kind {
	default:
		panic(&ValueError{"reflect.Value.Slice3", v.kind()})

	case Array:
		if v.flag&flagAddr == 0 {
			panic("reflect.Value.Slice3: slice of unaddressable array")
		}
		tt := (*arrayType)(unsafe.Pointer(v.typ()))
		cap = int(tt.Len)
		typ = (*sliceType)(unsafe.Pointer(tt.Slice))
		base = v.ptr

	case Slice:
		typ = (*sliceType)(unsafe.Pointer(v.typ()))
		s := (*unsafeheader.Slice)(v.ptr)
		base = s.Data
		cap = s.Cap
	}

	if i < 0 || j < i || k < j || k > cap {
		panic("reflect.Value.Slice3: slice index out of bounds")
	}

	// Declare slice so that the garbage collector
	// can see the base pointer in it.
	var x []unsafe.Pointer

	// Reinterpret as *unsafeheader.Slice to edit.
	s := (*unsafeheader.Slice)(unsafe.Pointer(&x))
	s.Len = j - i
	s.Cap = k - i
	if k-i > 0 {
		s.Data = arrayAt(base, i, typ.Elem.Size(), "i < k <= cap")
	} else {
		// do not advance pointer, to avoid pointing beyond end of slice
		s.Data = base
	}

	fl := v.flag.ro() | flagIndir | flag(Slice)
	return Value{typ.Common(), unsafe.Pointer(&x), fl}
}

// String returns the string v's underlying value, as a string.
// String is a special case because of Go's String method convention.
// Unlike the other getters, it does not panic if v's Kind is not [String].
// Instead, it returns a string of the form "<T value>" where T is v's type.
// The fmt package treats Values specially. It does not call their String
// method implicitly but instead prints the concrete values they hold.
func (v Value) String() string {
	// stringNonString is split out to keep String inlineable for string kinds.
	if v.kind() == String {
		return *(*string)(v.ptr)
	}
	return v.stringNonString()
}

func (v Value) stringNonString() string {
	if v.kind() == Invalid {
		return "<invalid Value>"
	}
	// If you call String on a reflect.Value of other type, it's better to
	// print something than to panic. Useful in debugging.
	return "<" + v.Type().String() + " Value>"
}

// TryRecv attempts to receive a value from the channel v but will not block.
// It panics if v's Kind is not [Chan].
// If the receive delivers a value, x is the transferred value and ok is true.
// If the receive cannot finish without blocking, x is the zero Value and ok is false.
// If the channel is closed, x is the zero value for the channel's element type and ok is false.
func (v Value) TryRecv() (x Value, ok bool) {
	v.mustBe(Chan)
	v.mustBeExported()
	return v.recv(true)
}

// TrySend attempts to send x on the channel v but will not block.
// It panics if v's Kind is not [Chan].
// It reports whether the value was sent.
// As in Go, x's value must be assignable to the channel's element type.
func (v Value) TrySend(x Value) bool {
	v.mustBe(Chan)
	v.mustBeExported()
	return v.send(x, true)
}

// Type returns v's type.
func (v Value) Type() Type {
	if v.flag != 0 && v.flag&flagMethod == 0 {
		return (*rtype)(abi.NoEscape(unsafe.Pointer(v.typ_))) // inline of toRType(v.typ()), for own inlining in inline test
	}
	return v.typeSlow()
}

//go:noinline
func (v Value) typeSlow() Type {
	return toRType(v.abiTypeSlow())
}

func (v Value) abiType() *abi.Type {
	if v.flag != 0 && v.flag&flagMethod == 0 {
		return v.typ()
	}
	return v.abiTypeSlow()
}

func (v Value) abiTypeSlow() *abi.Type {
	if v.flag == 0 {
		panic(&ValueError{"reflect.Value.Type", Invalid})
	}

	typ := v.typ()
	if v.flag&flagMethod == 0 {
		return v.typ()
	}

	// Method value.
	// v.typ describes the receiver, not the method type.
	i := int(v.flag) >> flagMethodShift
	if v.typ().Kind() == abi.Interface {
		// Method on interface.
		tt := (*interfaceType)(unsafe.Pointer(typ))
		if uint(i) >= uint(len(tt.Methods)) {
			panic("reflect: internal error: invalid method index")
		}
		m := &tt.Methods[i]
		return typeOffFor(typ, m.Typ)
	}
	// Method on concrete type.
	ms := typ.ExportedMethods()
	if uint(i) >= uint(len(ms)) {
		panic("reflect: internal error: invalid method index")
	}
	m := ms[i]
	return typeOffFor(typ, m.Mtyp)
}

// CanUint reports whether [Value.Uint] can be used without panicking.
func (v Value) CanUint() bool {
	switch v.kind() {
	case Uint, Uint8, Uint16, Uint32, Uint64, Uintptr:
		return true
	default:
		return false
	}
}

// Uint returns v's underlying value, as a uint64.
// It panics if v's Kind is not [Uint], [Uintptr], [Uint8], [Uint16], [Uint32], or [Uint64].
func (v Value) Uint() uint64 {
	k := v.kind()
	p := v.ptr
	switch k {
	case Uint:
		return uint64(*(*uint)(p))
	case Uint8:
		return uint64(*(*uint8)(p))
	case Uint16:
		return uint64(*(*uint16)(p))
	case Uint32:
		return uint64(*(*uint32)(p))
	case Uint64:
		return *(*uint64)(p)
	case Uintptr:
		return uint64(*(*uintptr)(p))
	}
	panic(&ValueError{"reflect.Value.Uint", v.kind()})
}

//go:nocheckptr
// This prevents inlining Value.UnsafeAddr when -d=checkptr is enabled,
// which ensures cmd/compile can recognize unsafe.Pointer(v.UnsafeAddr())
// and make an exception.

// UnsafeAddr returns a pointer to v's data, as a uintptr.
// It panics if v is not addressable.
//
// It's preferred to use uintptr(Value.Addr().UnsafePointer()) to get the equivalent result.
func (v Value) UnsafeAddr() uintptr {
	if v.typ() == nil {
		panic(&ValueError{"reflect.Value.UnsafeAddr", Invalid})
	}
	if v.flag&flagAddr == 0 {
		panic("reflect.Value.UnsafeAddr of unaddressable value")
	}
	// The compiler loses track as it converts to uintptr. Force escape.
	escapes(v.ptr)
	return uintptr(v.ptr)
}

// UnsafePointer returns v's value as a [unsafe.Pointer].
// It panics if v's Kind is not [Chan], [Func], [Map], [Pointer], [Slice], [String] or [UnsafePointer].
//
// If v's Kind is [Func], the returned pointer is an underlying
// code pointer, but not necessarily enough to identify a
// single function uniquely. The only guarantee is that the
// result is zero if and only if v is a nil func Value.
//
// If v's Kind is [Slice], the returned pointer is to the first
// element of the slice. If the slice is nil the returned value
// is nil.  If the slice is empty but non-nil the return value is non-nil.
//
// If v's Kind is [String], the returned pointer is to the first
// element of the underlying bytes of string.
func (v Value) UnsafePointer() unsafe.Pointer {
	k := v.kind()
	switch k {
	case Pointer:
		if !v.typ().Pointers() {
			// Since it is a not-in-heap pointer, all pointers to the heap are
			// forbidden! See comment in Value.Elem and issue #48399.
			if !verifyNotInHeapPtr(*(*uintptr)(v.ptr)) {
				panic("reflect: reflect.Value.UnsafePointer on an invalid notinheap pointer")
			}
			return *(*unsafe.Pointer)(v.ptr)
		}
		fallthrough
	case Chan, Map, UnsafePointer:
		return v.pointer()
	case Func:
		if v.flag&flagMethod != 0 {
			// As the doc comment says, the returned pointer is an
			// underlying code pointer but not necessarily enough to
			// identify a single function uniquely. All method expressions
			// created via reflect have the same underlying code pointer,
			// so their Pointers are equal. The function used here must
			// match the one used in makeMethodValue.
			code := methodValueCallCodePtr()
			return *(*unsafe.Pointer)(unsafe.Pointer(&code))
		}
		p := v.pointer()
		// Non-nil func value points at data block.
		// First word of data block is actual code.
		if p != nil {
			p = *(*unsafe.Pointer)(p)
		}
		return p
	case Slice:
		return (*unsafeheader.Slice)(v.ptr).Data
	case String:
		return (*unsafeheader.String)(v.ptr).Data
	}
	panic(&ValueError{"reflect.Value.UnsafePointer", v.kind()})
}

// Fields returns an iterator over each [StructField] of v along with its [Value].
//
// The sequence is equivalent to calling [Value.Field] successively
// for each index i in the range [0, NumField()).
//
// It panics if v's Kind is not Struct.
func (v Value) Fields() iter.Seq2[StructField, Value] {
	t := v.Type()
	if t.Kind() != Struct {
		panic("reflect: Fields of non-struct type " + t.String())
	}
	return func(yield func(StructField, Value) bool) {
		for i := range v.NumField() {
			if !yield(t.Field(i), v.Field(i)) {
				return
			}
		}
	}
}

// Methods returns an iterator over each [Method] of v along with the corresponding
// method [Value]; this is a function with v bound as the receiver. As such, the
// receiver shouldn't be included in the arguments to [Value.Call].
//
// The sequence is equivalent to calling [Value.Method] successively
// for each index i in the range [0, NumMethod()).
//
// Methods panics if v is a nil interface value.
//
// Calling this method will force the linker to retain all exported methods in all packages.
// This may make the executable binary larger but will not affect execution time.
func (v Value) Methods() iter.Seq2[Method, Value] {
	rtype := v.Type()
	n := v.NumMethod()
	return func(yield func(Method, Value) bool) {
		for i := range n {
			if !yield(rtype.Method(i), v.Method(i)) {
				return
			}
		}
	}
}

// StringHeader is the runtime representation of a string.
// It cannot be used safely or portably and its representation may
// change in a later release.
// Moreover, the Data field is not sufficient to guarantee the data
// it references will not be garbage collected, so programs must keep
// a separate, correctly typed pointer to the underlying data.
//
// Deprecated: Use unsafe.String or unsafe.StringData instead.
type StringHeader struct {
	Data uintptr
	Len  int
}

// SliceHeader is the runtime representation of a slice.
// It cannot be used safely or portably and its representation may
// change in a later release.
// Moreover, the Data field is not sufficient to guarantee the data
// it references will not be garbage collected, so programs must keep
// a separate, correctly typed pointer to the underlying data.
//
// Deprecated: Use unsafe.Slice or unsafe.SliceData instead.
type SliceHeader struct {
	Data uintptr
	Len  int
	Cap  int
}

func typesMustMatch(what string, t1, t2 Type) {
	if t1 != t2 {
		panic(what + ": " + t1.String() + " != " + t2.String())
	}
}

// arrayAt returns the i-th element of p,
// an array whose elements are eltSize bytes wide.
// The array pointed at by p must have at least i+1 elements:
// it is invalid (but impossible to check here) to pass i >= len,
// because then the result will point outside the array.
// whySafe must explain why i < len. (Passing "i < len" is fine;
// the benefit is to surface this assumption at the call site.)
func arrayAt(p unsafe.Pointer, i int, eltSize uintptr, whySafe string) unsafe.Pointer {
	return add(p, uintptr(i)*eltSize, "i < len")
}

// Grow increases the slice's capacity, if necessary, to guarantee space for
// another n elements. After Grow(n), at least n elements can be appended
// to the slice without another allocation.
//
// It panics if v's Kind is not a [Slice], or if n is negative or too large to
// allocate the memory, or if [Value.CanSet] returns false.
func (v Value) Grow(n int) {
	v.mustBeAssignable()
	v.mustBe(Slice)
	v.grow(n)
}

// grow is identical to Grow but does not check for assignability.
func (v Value) grow(n int) {
	p := (*unsafeheader.Slice)(v.ptr)
	switch {
	case n < 0:
		panic("reflect.Value.Grow: negative len")
	case p.Len+n < 0:
		panic("reflect.Value.Grow: slice overflow")
	case p.Len+n > p.Cap:
		t := v.typ().Elem()
		*p = growslice(t, *p, n)
	}
}

// extendSlice extends a slice by n elements.
//
// Unlike Value.grow, which modifies the slice in place and
// does not change the length of the slice in place,
// extendSlice returns a new slice value with the length
// incremented by the number of specified elements.
func (v Value) extendSlice(n int) Value {
	v.mustBeExported()
	v.mustBe(Slice)

	// Shallow copy the slice header to avoid mutating the source slice.
	sh := *(*unsafeheader.Slice)(v.ptr)
	s := &sh
	v.ptr = unsafe.Pointer(s)
	v.flag = flagIndir | flag(Slice) // equivalent flag to MakeSlice

	v.grow(n) // fine to treat as assignable since we allocate a new slice header
	s.Len += n
	return v
}

// Clear clears the contents of a map or zeros the contents of a slice.
//
// It panics if v's Kind is not [Map] or [Slice].
func (v Value) Clear() {
	switch v.Kind() {
	case Slice:
		sh := *(*unsafeheader.Slice)(v.ptr)
		st := (*sliceType)(unsafe.Pointer(v.typ()))
		typedarrayclear(st.Elem, sh.Data, sh.Len)
	case Map:
		mapclear(v.typ(), v.pointer())
	default:
		panic(&ValueError{"reflect.Value.Clear", v.Kind()})
	}
}

// Append appends the values x to a slice s and returns the resulting slice.
// As in Go, each x's value must be assignable to the slice's element type.
func Append(s Value, x ...Value) Value {
	s.mustBe(Slice)
	n := s.Len()
	s = s.extendSlice(len(x))
	for i, v := range x {
		s.Index(n + i).Set(v)
	}
	return s
}

// AppendSlice appends a slice t to a slice s and returns the resulting slice.
// The slices s and t must have the same element type.
func AppendSlice(s, t Value) Value {
	s.mustBe(Slice)
	t.mustBe(Slice)
	typesMustMatch("reflect.AppendSlice", s.Type().Elem(), t.Type().Elem())
	ns := s.Len()
	nt := t.Len()
	s = s.extendSlice(nt)
	Copy(s.Slice(ns, ns+nt), t)
	return s
}

// Copy copies the contents of src into dst until either
// dst has been filled or src has been exhausted.
// It returns the number of elements copied.
// Dst and src each must have kind [Slice] or [Array], and
// dst and src must have the same element type.
// It dst is an [Array], it panics if [Value.CanSet] returns false.
//
// As a special case, src can have kind [String] if the element type of dst is kind [Uint8].
func Copy(dst, src Value) int {
	dk := dst.kind()
	if dk != Array && dk != Slice {
		panic(&ValueError{"reflect.Copy", dk})
	}
	if dk == Array {
		dst.mustBeAssignable()
	}
	dst.mustBeExported()

	sk := src.kind()
	var stringCopy bool
	if sk != Array && sk != Slice {
		stringCopy = sk == String && dst.typ().Elem().Kind() == abi.Uint8
		if !stringCopy {
			panic(&ValueError{"reflect.Copy", sk})
		}
	}
	src.mustBeExported()

	de := dst.typ().Elem()
	if !stringCopy {
		se := src.typ().Elem()
		typesMustMatch("reflect.Copy", toType(de), toType(se))
	}

	var ds, ss unsafeheader.Slice
	if dk == Array {
		ds.Data = dst.ptr
		ds.Len = dst.Len()
		ds.Cap = ds.Len
	} else {
		ds = *(*unsafeheader.Slice)(dst.ptr)
	}
	if sk == Array {
		ss.Data = src.ptr
		ss.Len = src.Len()
		ss.Cap = ss.Len
	} else if sk == Slice {
		ss = *(*unsafeheader.Slice)(src.ptr)
	} else {
		sh := *(*unsafeheader.String)(src.ptr)
		ss.Data = sh.Data
		ss.Len = sh.Len
		ss.Cap = sh.Len
	}

	return typedslicecopy(de.Common(), ds, ss)
}

// A runtimeSelect is a single case passed to rselect.
// This must match ../runtime/select.go:/runtimeSelect
type runtimeSelect struct {
	dir SelectDir      // SelectSend, SelectRecv or SelectDefault
	typ *rtype         // channel type
	ch  unsafe.Pointer // channel
	val unsafe.Pointer // ptr to data (SendDir) or ptr to receive buffer (RecvDir)
}

// rselect runs a select. It returns the index of the chosen case.
// If the case was a receive, val is filled in with the received value.
// The conventional OK bool indicates whether the receive corresponds
// to a sent value.
//
// rselect generally doesn't escape the runtimeSelect slice, except
// that for the send case the value to send needs to escape. We don't
// have a way to represent that in the function signature. So we handle
// that with a forced escape in function Select.
//
//go:noescape
func rselect([]runtimeSelect) (chosen int, recvOK bool)

// A SelectDir describes the communication direction of a select case.
type SelectDir int

// NOTE: These values must match ../runtime/select.go:/selectDir.

const (
	_             SelectDir = iota
	SelectSend              // case Chan <- Send
	SelectRecv              // case <-Chan:
	SelectDefault           // default
)

// A SelectCase describes a single case in a select operation.
// The kind of case depends on Dir, the communication direction.
//
// If Dir is SelectDefault, the case represents a default case.
// Chan and Send must be zero Values.
//
// If Dir is SelectSend, the case represents a send operation.
// Normally Chan's underlying value must be a channel, and Send's underlying value must be
// assignable to the channel's element type. As a special case, if Chan is a zero Value,
// then the case is ignored, and the field Send will also be ignored and may be either zero
// or non-zero.
//
// If Dir is [SelectRecv], the case represents a receive operation.
// Normally Chan's underlying value must be a channel and Send must be a zero Value.
// If Chan is a zero Value, then the case is ignored, but Send must still be a zero Value.
// When a receive operation is selected, the received Value is returned by Select.
type SelectCase struct {
	Dir  SelectDir // direction of case
	Chan Value     // channel to use (for send or receive)
	Send Value     // value to send (for send)
}

// stackAllocSelectCases represents the length of a slice that we
// pre-allocate in [Select] to avoid heap allocations.
const stackAllocSelectCases = 4

// Select executes a select operation described by the list of cases.
// Like the Go select statement, it blocks until at least one of the cases
// can proceed, makes a uniform pseudo-random choice,
// and then executes that case. It returns the index of the chosen case
// and, if that case was a receive operation, the value received and a
// boolean indicating whether the value corresponds to a send on the channel
// (as opposed to a zero value received because the channel is closed).
// Select supports a maximum of 65536 cases.
func Select(cases []SelectCase) (chosen int, recv Value, recvOK bool) {
	// This function is specially designed to be inlined, such that when called as:
	//
	// Select([]SelectCase{})
	//
	// With a slice, that has a compile known length, the runcases slice
	// will end up being stack allocated, since the compiler can infer
	// the len([]SelectCase{}).
	//
	// We additionaly want to optimize Select(cases) for cases where len(cases)
	// cannot be infered at compile-time, thus in [select0] we allocate a
	// [stackAllocSelectCases]-length slice, which will avoid memory allocations
	// when the len(cases) <= stackAllocSelectCases and len(cases) is not compile-known.

	var runcases []runtimeSelect
	if len(cases) > stackAllocSelectCases {
		runcases = make([]runtimeSelect, len(cases))
	}
	chosen, recv, recvOK = select0(cases, runcases)
	return
}

func select0(cases []SelectCase, runcases []runtimeSelect) (chosen int, recv Value, recvOK bool) {
	if len(cases) > 65536 {
		panic("reflect.Select: too many cases (max 65536)")
	}

	// See [Select] for more details on this.
	if runcases == nil {
		runcases = make([]runtimeSelect, len(cases), stackAllocSelectCases)
	}

	haveDefault := false

	// NOTE: Do not trust that caller is not modifying cases data underfoot.
	// The range is safe because the caller cannot modify our copy of the len
	// and each iteration makes its own copy of the value c.
	for i, c := range cases {
		rc := &runcases[i]
		rc.dir = c.Dir
		switch c.Dir {
		default:
			panic("reflect.Select: invalid Dir")

		case SelectDefault: // default
			if haveDefault {
				panic("reflect.Select: multiple default cases")
			}
			haveDefault = true
			if c.Chan.IsValid() {
				panic("reflect.Select: default case has Chan value")
			}
			if c.Send.IsValid() {
				panic("reflect.Select: default case has Send value")
			}

		case SelectSend:
			ch := c.Chan
			if !ch.IsValid() {
				break
			}
			ch.mustBe(Chan)
			ch.mustBeExported()
			tt := (*chanType)(unsafe.Pointer(ch.typ()))
			if ChanDir(tt.Dir)&SendDir == 0 {
				panic("reflect.Select: SendDir case using recv-only channel")
			}
			rc.ch = ch.pointer()
			rc.typ = toRType(&tt.Type)
			v := c.Send
			if !v.IsValid() {
				panic("reflect.Select: SendDir case missing Send value")
			}
			v.mustBeExported()
			v = v.assignTo("reflect.Select", tt.Elem, nil)
			if v.flag&flagIndir != 0 {
				rc.val = v.ptr
			} else {
				rc.val = unsafe.Pointer(&v.ptr)
			}
			// The value to send needs to escape. See the comment at rselect for
			// why we need forced escape.
			escapes(rc.val)

		case SelectRecv:
			if c.Send.IsValid() {
				panic("reflect.Select: RecvDir case has Send value")
			}
			ch := c.Chan
			if !ch.IsValid() {
				break
			}
			ch.mustBe(Chan)
			ch.mustBeExported()
			tt := (*chanType)(unsafe.Pointer(ch.typ()))
			if ChanDir(tt.Dir)&RecvDir == 0 {
				panic("reflect.Select: RecvDir case using send-only channel")
			}
			rc.ch = ch.pointer()
			rc.typ = toRType(&tt.Type)
			rc.val = unsafe_New(tt.Elem)
		}
	}

	chosen, recvOK = rselect(runcases)
	if runcases[chosen].dir == SelectRecv {
		tt := (*chanType)(unsafe.Pointer(runcases[chosen].typ))
		t := tt.Elem
		p := runcases[chosen].val
		fl := flag(t.Kind())
		if !t.IsDirectIface() {
			recv = Value{t, p, fl | flagIndir}
		} else {
			recv = Value{t, *(*unsafe.Pointer)(p), fl}
		}
	}
	return chosen, recv, recvOK
}

/*
 * constructors
 */

// implemented in package runtime

//go:noescape
func unsafe_New(*abi.Type) unsafe.Pointer

//go:noescape
func unsafe_NewArray(*abi.Type, int) unsafe.Pointer

// MakeSlice creates a new zero-initialized slice value
// for the specified slice type, length, and capacity.
func MakeSlice(typ Type, len, cap int) Value {
	if typ.Kind() != Slice {
		panic("reflect.MakeSlice of non-slice type")
	}
	if len < 0 {
		panic("reflect.MakeSlice: negative len")
	}
	if cap < 0 {
		panic("reflect.MakeSlice: negative cap")
	}
	if len > cap {
		panic("reflect.MakeSlice: len > cap")
	}

	s := unsafeheader.Slice{Data: unsafe_NewArray(&(typ.Elem().(*rtype).t), cap), Len: len, Cap: cap}
	return Value{&typ.(*rtype).t, unsafe.Pointer(&s), flagIndir | flag(Slice)}
}

// SliceAt returns a [Value] representing a slice whose underlying
// data starts at p, with length and capacity equal to n.
//
// This is like [unsafe.Slice].
func SliceAt(typ Type, p unsafe.Pointer, n int) Value {
	unsafeslice(typ.common(), p, n)
	s := unsafeheader.Slice{Data: p, Len: n, Cap: n}
	return Value{SliceOf(typ).common(), unsafe.Pointer(&s), flagIndir | flag(Slice)}
}

// MakeChan creates a new channel with the specified type and buffer size.
func MakeChan(typ Type, buffer int) Value {
	if typ.Kind() != Chan {
		panic("reflect.MakeChan of non-chan type")
	}
	if buffer < 0 {
		panic("reflect.MakeChan: negative buffer size")
	}
	if typ.ChanDir() != BothDir {
		panic("reflect.MakeChan: unidirectional channel type")
	}
	t := typ.common()
	ch := makechan(t, buffer)
	return Value{t, ch, flag(Chan)}
}

// MakeMap creates a new map with the specified type.
func MakeMap(typ Type) Value {
	return MakeMapWithSize(typ, 0)
}

// MakeMapWithSize creates a new map with the specified type
// and initial space for approximately n elements.
func MakeMapWithSize(typ Type, n int) Value {
	if typ.Kind() != Map {
		panic("reflect.MakeMapWithSize of non-map type")
	}
	t := typ.common()
	m := makemap(t, n)
	return Value{t, m, flag(Map)}
}

// Indirect returns the value that v points to.
// If v is a nil pointer, Indirect returns a zero Value.
// If v is not a pointer, Indirect returns v.
func Indirect(v Value) Value {
	if v.Kind() != Pointer {
		return v
	}
	return v.Elem()
}

// ValueOf returns a new Value initialized to the concrete value
// stored in the interface i. ValueOf(nil) returns the zero Value.
func ValueOf(i any) Value {
	if i == nil {
		return Value{}
	}
	return unpackEface(i)
}

// Zero returns a Value representing the zero value for the specified type.
// The result is different from the zero value of the Value struct,
// which represents no value at all.
// For example, Zero(TypeOf(42)) returns a Value with Kind [Int] and value 0.
// The returned value is neither addressable nor settable.
func Zero(typ Type) Value {
	if typ == nil {
		panic("reflect: Zero(nil)")
	}
	t := &typ.(*rtype).t
	fl := flag(t.Kind())
	if !t.IsDirectIface() {
		var p unsafe.Pointer
		if t.Size() <= abi.ZeroValSize {
			p = unsafe.Pointer(&zeroVal[0])
		} else {
			p = unsafe_New(t)
		}
		return Value{t, p, fl | flagIndir}
	}
	return Value{t, nil, fl}
}

//go:linkname zeroVal runtime.zeroVal
var zeroVal [abi.ZeroValSize]byte

// New returns a Value representing a pointer to a new zero value
// for the specified type. That is, the returned Value's Type is [PointerTo](typ).
func New(typ Type) Value {
	if typ == nil {
		panic("reflect: New(nil)")
	}
	t := &typ.(*rtype).t
	pt := ptrTo(t)
	if !pt.IsDirectIface() {
		// This is a pointer to a not-in-heap type.
		panic("reflect: New of type that may not be allocated in heap (possibly undefined cgo C type)")
	}
	ptr := unsafe_New(t)
	fl := flag(Pointer)
	return Value{pt, ptr, fl}
}

// NewAt returns a Value representing a pointer to a value of the
// specified type, using p as that pointer.
func NewAt(typ Type, p unsafe.Pointer) Value {
	fl := flag(Pointer)
	t := typ.(*rtype)
	return Value{t.ptrTo(), p, fl}
}

// assignTo returns a value v that can be assigned directly to dst.
// It panics if v is not assignable to dst.
// For a conversion to an interface type, target, if not nil,
// is a suggested scratch space to use.
// target must be initialized memory (or nil).
func (v Value) assignTo(context string, dst *abi.Type, target unsafe.Pointer) Value {
	if v.flag&flagMethod != 0 {
		v = makeMethodValue(context, v)
	}

	switch {
	case directlyAssignable(dst, v.typ()):
		// Overwrite type so that they match.
		// Same memory layout, so no harm done.
		fl := v.flag&(flagAddr|flagIndir) | v.flag.ro()
		fl |= flag(dst.Kind())
		return Value{dst, v.ptr, fl}

	case implements(dst, v.typ()):
		if v.Kind() == Interface && v.IsNil() {
			// A nil ReadWriter passed to nil Reader is OK,
			// but using ifaceE2I below will panic.
			// Avoid the panic by returning a nil dst (e.g., Reader) explicitly.
			return Value{dst, nil, flag(Interface)}
		}
		x := valueInterface(v, false)
		if target == nil {
			target = unsafe_New(dst)
		}
		if dst.NumMethod() == 0 {
			*(*any)(target) = x
		} else {
			ifaceE2I(dst, x, target)
		}
		return Value{dst, target, flagIndir | flag(Interface)}
	}

	// Failed.
	panic(context + ": value of type " + stringFor(v.typ()) + " is not assignable to type " + stringFor(dst))
}

// Convert returns the value v converted to type t.
// If the usual Go conversion rules do not allow conversion
// of the value v to type t, or if converting v to type t panics, Convert panics.
func (v Value) Convert(t Type) Value {
	if v.flag&flagMethod != 0 {
		v = makeMethodValue("Convert", v)
	}
	op := convertOp(t.common(), v.typ())
	if op == nil {
		panic("reflect.Value.Convert: value of type " + stringFor(v.typ()) + " cannot be converted to type " + t.String())
	}
	return op(v, t)
}

// CanConvert reports whether the value v can be converted to type t.
// If v.CanConvert(t) returns true then v.Convert(t) will not panic.
func (v Value) CanConvert(t Type) bool {
	vt := v.Type()
	if !vt.ConvertibleTo(t) {
		return false
	}
	// Converting from slice to array or to pointer-to-array can panic
	// depending on the value.
	switch {
	case vt.Kind() == Slice && t.Kind() == Array:
		if t.Len() > v.Len() {
			return false
		}
	case vt.Kind() == Slice && t.Kind() == Pointer && t.Elem().Kind() == Array:
		n := t.Elem().Len()
		if n > v.Len() {
			return false
		}
	}
	return true
}

// Comparable reports whether the value v is comparable.
// If the type of v is an interface, this checks the dynamic type.
// If this reports true then v.Interface() == x will not panic for any x,
// nor will v.Equal(u) for any Value u.
func (v Value) Comparable() bool {
	k := v.Kind()
	switch k {
	case Invalid:
		return false

	case Array:
		switch v.Type().Elem().Kind() {
		case Interface, Array, Struct:
			for i := 0; i < v.Type().Len(); i++ {
				if !v.Index(i).Comparable() {
					return false
				}
			}
			return true
		}
		return v.Type().Comparable()

	case Interface:
		return v.IsNil() || v.Elem().Comparable()

	case Struct:
		for _, value := range v.Fields() {
			if !value.Comparable() {
				return false
			}
		}
		return true

	default:
		return v.Type().Comparable()
	}
}

// Equal reports true if v is equal to u.
// For two invalid values, Equal will report true.
// For an interface value, Equal will compare the value within the interface.
// Otherwise, If the values have different types, Equal will report false.
// Otherwise, for arrays and structs Equal will compare each element in order,
// and report false if it finds non-equal elements.
// During all comparisons, if values of the same type are compared,
// and the type is not comparable, Equal will panic.
func (v Value) Equal(u Value) bool {
	if v.Kind() == Interface {
		v = v.Elem()
	}
	if u.Kind() == Interface {
		u = u.Elem()
	}

	if !v.IsValid() || !u.IsValid() {
		return v.IsValid() == u.IsValid()
	}

	if v.Kind() != u.Kind() || v.Type() != u.Type() {
		return false
	}

	// Handle each Kind directly rather than calling valueInterface
	// to avoid allocating.
	switch v.Kind() {
	default:
		panic("reflect.Value.Equal: invalid Kind")
	case Bool:
		return v.Bool() == u.Bool()
	case Int, Int8, Int16, Int32, Int64:
		return v.Int() == u.Int()
	case Uint, Uint8, Uint16, Uint32, Uint64, Uintptr:
		return v.Uint() == u.Uint()
	case Float32, Float64:
		return v.Float() == u.Float()
	case Complex64, Complex128:
		return v.Complex() == u.Complex()
	case String:
		return v.String() == u.String()
	case Chan, Pointer, UnsafePointer:
		return v.Pointer() == u.Pointer()
	case Array:
		// u and v have the same type so they have the same length
		vl := v.Len()
		if vl == 0 {
			// panic on [0]func()
			if !v.Type().Elem().Comparable() {
				break
			}
			return true
		}
		for i := 0; i < vl; i++ {
			if !v.Index(i).Equal(u.Index(i)) {
				return false
			}
		}
		return true
	case Struct:
		// u and v have the same type so they have the same fields
		nf := v.NumField()
		for i := 0; i < nf; i++ {
			if !v.Field(i).Equal(u.Field(i)) {
				return false
			}
		}
		return true
	case Func, Map, Slice:
		break
	}
	panic("reflect.Value.Equal: values of type " + v.Type().String() + " are not comparable")
}

// convertOp returns the function to convert a value of type src
// to a value of type dst. If the conversion is illegal, convertOp returns nil.
func convertOp(dst, src *abi.Type) func(Value, Type) Value {
	switch Kind(src.Kind()) {
	case Int, Int8, Int16, Int32, Int64:
		switch Kind(dst.Kind()) {
		case Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64, Uintptr:
			return cvtInt
		case Float32, Float64:
			return cvtIntFloat
		case String:
			return cvtIntString
		}

	case Uint, Uint8, Uint16, Uint32, Uint64, Uintptr:
		switch Kind(dst.Kind()) {
		case Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64, Uintptr:
			return cvtUint
		case Float32, Float64:
			return cvtUintFloat
		case String:
			return cvtUintString
		}

	case Float32, Float64:
		switch Kind(dst.Kind()) {
		case Int, Int8, Int16, Int32, Int64:
			return cvtFloatInt
		case Uint, Uint8, Uint16, Uint32, Uint64, Uintptr:
			return cvtFloatUint
		case Float32, Float64:
			return cvtFloat
		}

	case Complex64, Complex128:
		switch Kind(dst.Kind()) {
		case Complex64, Complex128:
			return cvtComplex
		}

	case String:
		if dst.Kind() == abi.Slice {
			switch Kind(dst.Elem().Kind()) {
			case Uint8:
				return cvtStringBytes
			case Int32:
				return cvtStringRunes
			}
		}

	case Slice:
		if dst.Kind() == abi.String {
			switch Kind(src.Elem().Kind()) {
			case Uint8:
				return cvtBytesString
			case Int32:
				return cvtRunesString
			}
		}
		// "x is a slice, T is a pointer-to-array type,
		// and the slice and array types have identical element types."
		if dst.Kind() == abi.Pointer && dst.Elem().Kind() == abi.Array && src.Elem() == dst.Elem().Elem() {
			return cvtSliceArrayPtr
		}
		// "x is a slice, T is an array type,
		// and the slice and array types have identical element types."
		if dst.Kind() == abi.Array && src.Elem() == dst.Elem() {
			return cvtSliceArray
		}

	case Chan:
		if dst.Kind() == abi.Chan && specialChannelAssignability(dst, src) {
			return cvtDirect
		}
	}

	// dst and src have same underlying type.
	if haveIdenticalUnderlyingType(dst, src, false) {
		return cvtDirect
	}

	// dst and src are non-defined pointer types with same underlying base type.
	if dst.Kind() == abi.Pointer && nameFor(dst) == "" &&
		src.Kind() == abi.Pointer && nameFor(src) == "" &&
		haveIdenticalUnderlyingType(elem(dst), elem(src), false) {
		return cvtDirect
	}

	if implements(dst, src) {
		if src.Kind() == abi.Interface {
			return cvtI2I
		}
		return cvtT2I
	}

	return nil
}

// makeInt returns a Value of type t equal to bits (possibly truncated),
// where t is a signed or unsigned int type.
func makeInt(f flag, bits uint64, t Type) Value {
	typ := t.common()
	ptr := unsafe_New(typ)
	switch typ.Size() {
	case 1:
		*(*uint8)(ptr) = uint8(bits)
	case 2:
		*(*uint16)(ptr) = uint16(bits)
	case 4:
		*(*uint32)(ptr) = uint32(bits)
	case 8:
		*(*uint64)(ptr) = bits
	}
	return Value{typ, ptr, f | flagIndir | flag(typ.Kind())}
}

// makeFloat returns a Value of type t equal to v (possibly truncated to float32),
// where t is a float32 or float64 type.
func makeFloat(f flag, v float64, t Type) Value {
	typ := t.common()
	ptr := unsafe_New(typ)
	switch typ.Size() {
	case 4:
		*(*float32)(ptr) = float32(v)
	case 8:
		*(*float64)(ptr) = v
	}
	return Value{typ, ptr, f | flagIndir | flag(typ.Kind())}
}

// makeFloat32 returns a Value of type t equal to v, where t is a float32 type.
func makeFloat32(f flag, v float32, t Type) Value {
	typ := t.common()
	ptr := unsafe_New(typ)
	*(*float32)(ptr) = v
	return Value{typ, ptr, f | flagIndir | flag(typ.Kind())}
}

// makeComplex returns a Value of type t equal to v (possibly truncated to complex64),
// where t is a complex64 or complex128 type.
func makeComplex(f flag, v complex128, t Type) Value {
	typ := t.common()
	ptr := unsafe_New(typ)
	switch typ.Size() {
	case 8:
		*(*complex64)(ptr) = complex64(v)
	case 16:
		*(*complex128)(ptr) = v
	}
	return Value{typ, ptr, f | flagIndir | flag(typ.Kind())}
}

func makeString(f flag, v string, t Type) Value {
	ret := New(t).Elem()
	ret.SetString(v)
	ret.flag = ret.flag&^flagAddr | f
	return ret
}

func makeBytes(f flag, v []byte, t Type) Value {
	ret := New(t).Elem()
	ret.SetBytes(v)
	ret.flag = ret.flag&^flagAddr | f
	return ret
}

func makeRunes(f flag, v []rune, t Type) Value {
	ret := New(t).Elem()
	ret.setRunes(v)
	ret.flag = ret.flag&^flagAddr | f
	return ret
}

// These conversion functions are returned by convertOp
// for classes of conversions. For example, the first function, cvtInt,
// takes any value v of signed int type and returns the value converted
// to type t, where t is any signed or unsigned int type.

// convertOp: intXX -> [u]intXX
func cvtInt(v Value, t Type) Value {
	return makeInt(v.flag.ro(), uint64(v.Int()), t)
}

// convertOp: uintXX -> [u]intXX
func cvtUint(v Value, t Type) Value {
	return makeInt(v.flag.ro(), v.Uint(), t)
}

// convertOp: floatXX -> intXX
func cvtFloatInt(v Value, t Type) Value {
	return makeInt(v.flag.ro(), uint64(int64(v.Float())), t)
}

// convertOp: floatXX -> uintXX
func cvtFloatUint(v Value, t Type) Value {
	return makeInt(v.flag.ro(), uint64(v.Float()), t)
}

// convertOp: intXX -> floatXX
func cvtIntFloat(v Value, t Type) Value {
	return makeFloat(v.flag.ro(), float64(v.Int()), t)
}

// convertOp: uintXX -> floatXX
func cvtUintFloat(v Value, t Type) Value {
	return makeFloat(v.flag.ro(), float64(v.Uint()), t)
}

// convertOp: floatXX -> floatXX
func cvtFloat(v Value, t Type) Value {
	if v.Type().Kind() == Float32 && t.Kind() == Float32 {
		// Don't do any conversion if both types have underlying type float32.
		// This avoids converting to float64 and back, which will
		// convert a signaling NaN to a quiet NaN. See issue 36400.
		return makeFloat32(v.flag.ro(), *(*float32)(v.ptr), t)
	}
	return makeFloat(v.flag.ro(), v.Float(), t)
}

// convertOp: complexXX -> complexXX
func cvtComplex(v Value, t Type) Value {
	return makeComplex(v.flag.ro(), v.Complex(), t)
}

// convertOp: intXX -> string
func cvtIntString(v Value, t Type) Value {
	s := "\uFFFD"
	if x := v.Int(); int64(rune(x)) == x {
		s = string(rune(x))
	}
	return makeString(v.flag.ro(), s, t)
}

// convertOp: uintXX -> string
func cvtUintString(v Value, t Type) Value {
	s := "\uFFFD"
	if x := v.Uint(); uint64(rune(x)) == x {
		s = string(rune(x))
	}
	return makeString(v.flag.ro(), s, t)
}

// convertOp: []byte -> string
func cvtBytesString(v Value, t Type) Value {
	return makeString(v.flag.ro(), string(v.Bytes()), t)
}

// convertOp: string -> []byte
func cvtStringBytes(v Value, t Type) Value {
	return makeBytes(v.flag.ro(), []byte(v.String()), t)
}

// convertOp: []rune -> string
func cvtRunesString(v Value, t Type) Value {
	return makeString(v.flag.ro(), string(v.runes()), t)
}

// convertOp: string -> []rune
func cvtStringRunes(v Value, t Type) Value {
	return makeRunes(v.flag.ro(), []rune(v.String()), t)
}

// convertOp: []T -> *[N]T
func cvtSliceArrayPtr(v Value, t Type) Value {
	n := t.Elem().Len()
	if n > v.Len() {
		panic("reflect: cannot convert slice with length " + strconv.Itoa(v.Len()) + " to pointer to array with length " + strconv.Itoa(n))
	}
	h := (*unsafeheader.Slice)(v.ptr)
	return Value{t.common(), h.Data, v.flag&^(flagIndir|flagAddr|flagKindMask) | flag(Pointer)}
}

// convertOp: []T -> [N]T
func cvtSliceArray(v Value, t Type) Value {
	n := t.Len()
	if n > v.Len() {
		panic("reflect: cannot convert slice with length " + strconv.Itoa(v.Len()) + " to array with length " + strconv.Itoa(n))
	}
	h := (*unsafeheader.Slice)(v.ptr)
	typ := t.common()
	ptr := h.Data
	c := unsafe_New(typ)
	typedmemmove(typ, c, ptr)
	ptr = c

	return Value{typ, ptr, v.flag&^(flagAddr|flagKindMask) | flag(Array)}
}

// convertOp: direct copy
func cvtDirect(v Value, typ Type) Value {
	f := v.flag
	t := typ.common()
	ptr := v.ptr
	if f&flagAddr != 0 {
		// indirect, mutable word - make a copy
		c := unsafe_New(t)
		typedmemmove(t, c, ptr)
		ptr = c
		f &^= flagAddr
	}
	return Value{t, ptr, v.flag.ro() | f} // v.flag.ro()|f == f?
}

// convertOp: concrete -> interface
func cvtT2I(v Value, typ Type) Value {
	target := unsafe_New(typ.common())
	x := valueInterface(v, false)
	if typ.NumMethod() == 0 {
		*(*any)(target) = x
	} else {
		ifaceE2I(typ.common(), x, target)
	}
	return Value{typ.common(), target, v.flag.ro() | flagIndir | flag(Interface)}
}

// convertOp: interface -> interface
func cvtI2I(v Value, typ Type) Value {
	if v.IsNil() {
		ret := Zero(typ)
		ret.flag |= v.flag.ro()
		return ret
	}
	return cvtT2I(v.Elem(), typ)
}

// implemented in ../runtime
//
//go:noescape
func chancap(ch unsafe.Pointer) int

//go:noescape
func chanclose(ch unsafe.Pointer)

//go:noescape
func chanlen(ch unsafe.Pointer) int

// Note: some of the noescape annotations below are technically a lie,
// but safe in the context of this package. Functions like chansend0
// and mapassign0 don't escape the referent, but may escape anything
// the referent points to (they do shallow copies of the referent).
// We add a 0 to their names and wrap them in functions with the
// proper escape behavior.

//go:noescape
func chanrecv(ch unsafe.Pointer, nb bool, val unsafe.Pointer) (selected, received bool)

//go:noescape
func chansend0(ch unsafe.Pointer, val unsafe.Pointer, nb bool) bool

func chansend(ch unsafe.Pointer, val unsafe.Pointer, nb bool) bool {
	contentEscapes(val)
	return chansend0(ch, val, nb)
}

func makechan(typ *abi.Type, size int) (ch unsafe.Pointer)
func makemap(t *abi.Type, cap int) (m unsafe.Pointer)

//go:noescape
func mapaccess(t *abi.Type, m unsafe.Pointer, key unsafe.Pointer) (val unsafe.Pointer)

//go:noescape
func mapaccess_faststr(t *abi.Type, m unsafe.Pointer, key string) (val unsafe.Pointer)

//go:noescape
func mapassign0(t *abi.Type, m unsafe.Pointer, key, val unsafe.Pointer)

// mapassign should be an internal detail,
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/modern-go/reflect2
//   - github.com/goccy/go-json
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
//go:linkname mapassign
func mapassign(t *abi.Type, m unsafe.Pointer, key, val unsafe.Pointer) {
	contentEscapes(key)
	contentEscapes(val)
	mapassign0(t, m, key, val)
}

//go:noescape
func mapassign_faststr0(t *abi.Type, m unsafe.Pointer, key string, val unsafe.Pointer)

func mapassign_faststr(t *abi.Type, m unsafe.Pointer, key string, val unsafe.Pointer) {
	contentEscapes((*unsafeheader.String)(unsafe.Pointer(&key)).Data)
	contentEscapes(val)
	mapassign_faststr0(t, m, key, val)
}

//go:noescape
func mapdelete(t *abi.Type, m unsafe.Pointer, key unsafe.Pointer)

//go:noescape
func mapdelete_faststr(t *abi.Type, m unsafe.Pointer, key string)

//go:noescape
func maplen(m unsafe.Pointer) int

func mapclear(t *abi.Type, m unsafe.Pointer)

// call calls fn with "stackArgsSize" bytes of stack arguments laid out
// at stackArgs and register arguments laid out in regArgs. frameSize is
// the total amount of stack space that will be reserved by call, so this
// should include enough space to spill register arguments to the stack in
// case of preemption.
//
// After fn returns, call copies stackArgsSize-stackRetOffset result bytes
// back into stackArgs+stackRetOffset before returning, for any return
// values passed on the stack. Register-based return values will be found
// in the same regArgs structure.
//
// regArgs must also be prepared with an appropriate ReturnIsPtr bitmap
// indicating which registers will contain pointer-valued return values. The
// purpose of this bitmap is to keep pointers visible to the GC between
// returning from reflectcall and actually using them.
//
// If copying result bytes back from the stack, the caller must pass the
// argument frame type as stackArgsType, so that call can execute appropriate
// write barriers during the copy.
//
// Arguments passed through to call do not escape. The type is used only in a
// very limited callee of call, the stackArgs are copied, and regArgs is only
// used in the call frame.
//
//go:noescape
//go:linkname call runtime.reflectcall
func call(stackArgsType *abi.Type, f, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)

func ifaceE2I(t *abi.Type, src any, dst unsafe.Pointer)

// memmove copies size bytes to dst from src. No write barriers are used.
//
//go:noescape
func memmove(dst, src unsafe.Pointer, size uintptr)

// typedmemmove copies a value of type t to dst from src.
//
//go:noescape
func typedmemmove(t *abi.Type, dst, src unsafe.Pointer)

// typedmemclr zeros the value at ptr of type t.
//
//go:noescape
func typedmemclr(t *abi.Type, ptr unsafe.Pointer)

// typedmemclrpartial is like typedmemclr but assumes that
// dst points off bytes into the value and only clears size bytes.
//
//go:noescape
func typedmemclrpartial(t *abi.Type, ptr unsafe.Pointer, off, size uintptr)

// typedslicecopy copies a slice of elemType values from src to dst,
// returning the number of elements copied.
//
//go:noescape
func typedslicecopy(t *abi.Type, dst, src unsafeheader.Slice) int

// typedarrayclear zeroes the value at ptr of an array of elemType,
// only clears len elem.
//
//go:noescape
func typedarrayclear(elemType *abi.Type, ptr unsafe.Pointer, len int)

//go:noescape
func typehash(t *abi.Type, p unsafe.Pointer, h uintptr) uintptr

func verifyNotInHeapPtr(p uintptr) bool

//go:noescape
func growslice(t *abi.Type, old unsafeheader.Slice, num int) unsafeheader.Slice

//go:noescape
func unsafeslice(t *abi.Type, ptr unsafe.Pointer, len int)

// Dummy annotation marking that the value x escapes,
// for use in cases where the reflect code is so clever that
// the compiler cannot follow.
func escapes(x any) {
	if dummy.b {
		dummy.x = x
	}
}

var dummy struct {
	b bool
	x any
}

// Dummy annotation marking that the content of value x
// escapes (i.e. modeling roughly heap=*x),
// for use in cases where the reflect code is so clever that
// the compiler cannot follow.
func contentEscapes(x unsafe.Pointer) {
	if dummy.b {
		escapes(*(*any)(x)) // the dereference may not always be safe, but never executed
	}
}

```

// === FILE: references!/go/src/reflect/visiblefields.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reflect

// VisibleFields returns all the visible fields in t, which must be a
// struct type. A field is defined as visible if it's accessible
// directly with a FieldByName call. The returned fields include fields
// inside anonymous struct members and unexported fields. They follow
// the same order found in the struct, with anonymous fields followed
// immediately by their promoted fields.
//
// For each element e of the returned slice, the corresponding field
// can be retrieved from a value v of type t by calling v.FieldByIndex(e.Index).
func VisibleFields(t Type) []StructField {
	if t == nil {
		panic("reflect: VisibleFields(nil)")
	}
	if t.Kind() != Struct {
		panic("reflect.VisibleFields of non-struct type")
	}
	w := &visibleFieldsWalker{
		byName:   make(map[string]int),
		visiting: make(map[Type]bool),
		fields:   make([]StructField, 0, t.NumField()),
		index:    make([]int, 0, 2),
	}
	w.walk(t)
	// Remove all the fields that have been hidden.
	// Use an in-place removal that avoids copying in
	// the common case that there are no hidden fields.
	j := 0
	for i := range w.fields {
		f := &w.fields[i]
		if f.Name == "" {
			continue
		}
		if i != j {
			// A field has been removed. We need to shuffle
			// all the subsequent elements up.
			w.fields[j] = *f
		}
		j++
	}
	return w.fields[:j]
}

type visibleFieldsWalker struct {
	byName   map[string]int
	visiting map[Type]bool
	fields   []StructField
	index    []int
}

// walk walks all the fields in the struct type t, visiting
// fields in index preorder and appending them to w.fields
// (this maintains the required ordering).
// Fields that have been overridden have their
// Name field cleared.
func (w *visibleFieldsWalker) walk(t Type) {
	if w.visiting[t] {
		return
	}
	w.visiting[t] = true
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		w.index = append(w.index, i)
		add := true
		if oldIndex, ok := w.byName[f.Name]; ok {
			old := &w.fields[oldIndex]
			if len(w.index) == len(old.Index) {
				// Fields with the same name at the same depth
				// cancel one another out. Set the field name
				// to empty to signify that has happened, and
				// there's no need to add this field.
				old.Name = ""
				add = false
			} else if len(w.index) < len(old.Index) {
				// The old field loses because it's deeper than the new one.
				old.Name = ""
			} else {
				// The old field wins because it's shallower than the new one.
				add = false
			}
		}
		if add {
			// Copy the index so that it's not overwritten
			// by the other appends.
			f.Index = append([]int(nil), w.index...)
			w.byName[f.Name] = len(w.fields)
			w.fields = append(w.fields, f)
		}
		if f.Anonymous {
			if f.Type.Kind() == Pointer {
				f.Type = f.Type.Elem()
			}
			if f.Type.Kind() == Struct {
				w.walk(f.Type)
			}
		}
		w.index = w.index[:len(w.index)-1]
	}
	delete(w.visiting, t)
}

```

