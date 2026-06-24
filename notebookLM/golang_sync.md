# Domain Architecture: sync

## Layout Topology
```text
sync/
├── atomic
│   ├── asm.s
│   ├── doc.go
│   ├── doc_32.go
│   ├── doc_64.go
│   ├── race.s
│   ├── type.go
│   └── value.go
├── cond.go
├── map.go
├── mutex.go
├── once.go
├── oncefunc.go
├── pool.go
├── poolqueue.go
├── runtime.go
├── runtime2.go
├── runtime2_lockrank.go
├── rwmutex.go
└── waitgroup.go
```

## Source Stream Aggregation

// === FILE: references/go/src/sync/atomic/asm.s ===
```text
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !race

#include "textflag.h"

TEXT ·SwapInt32(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Xchg(SB)

TEXT ·SwapUint32(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Xchg(SB)

TEXT ·SwapInt64(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Xchg64(SB)

TEXT ·SwapUint64(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Xchg64(SB)

TEXT ·SwapUintptr(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Xchguintptr(SB)

TEXT ·CompareAndSwapInt32(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Cas(SB)

TEXT ·CompareAndSwapUint32(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Cas(SB)

TEXT ·CompareAndSwapUintptr(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Casuintptr(SB)

TEXT ·CompareAndSwapInt64(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Cas64(SB)

TEXT ·CompareAndSwapUint64(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Cas64(SB)

TEXT ·AddInt32(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Xadd(SB)

TEXT ·AddUint32(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Xadd(SB)

TEXT ·AddUintptr(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Xadduintptr(SB)

TEXT ·AddInt64(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Xadd64(SB)

TEXT ·AddUint64(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Xadd64(SB)

TEXT ·LoadInt32(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Load(SB)

TEXT ·LoadUint32(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Load(SB)

TEXT ·LoadInt64(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Load64(SB)

TEXT ·LoadUint64(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Load64(SB)

TEXT ·LoadUintptr(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Loaduintptr(SB)

TEXT ·LoadPointer(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Loadp(SB)

TEXT ·StoreInt32(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Store(SB)

TEXT ·StoreUint32(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Store(SB)

TEXT ·StoreInt64(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Store64(SB)

TEXT ·StoreUint64(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Store64(SB)

TEXT ·StoreUintptr(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Storeuintptr(SB)

TEXT ·AndInt32(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·And32(SB)

TEXT ·AndUint32(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·And32(SB)

TEXT ·AndUintptr(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Anduintptr(SB)

TEXT ·AndInt64(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·And64(SB)

TEXT ·AndUint64(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·And64(SB)

TEXT ·OrInt32(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Or32(SB)

TEXT ·OrUint32(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Or32(SB)

TEXT ·OrUintptr(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Oruintptr(SB)

TEXT ·OrInt64(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Or64(SB)

TEXT ·OrUint64(SB),NOSPLIT,$0
	JMP	internal∕runtime∕atomic·Or64(SB)

```

// === FILE: references/go/src/sync/atomic/doc.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package atomic provides low-level atomic memory primitives
// useful for implementing synchronization algorithms.
//
// These functions require great care to be used correctly.
// Except for special, low-level applications, synchronization is better
// done with channels or the facilities of the [sync] package.
// Share memory by communicating;
// don't communicate by sharing memory.
//
// The swap operation, implemented by the SwapT functions, is the atomic
// equivalent of:
//
//	old = *addr
//	*addr = new
//	return old
//
// The compare-and-swap operation, implemented by the CompareAndSwapT
// functions, is the atomic equivalent of:
//
//	if *addr == old {
//		*addr = new
//		return true
//	}
//	return false
//
// The add operation, implemented by the AddT functions, is the atomic
// equivalent of:
//
//	*addr += delta
//	return *addr
//
// The load and store operations, implemented by the LoadT and StoreT
// functions, are the atomic equivalents of "return *addr" and
// "*addr = val".
//
// In the terminology of [the Go memory model], if the effect of
// an atomic operation A is observed by atomic operation B,
// then A “synchronizes before” B.
// Additionally, all the atomic operations executed in a program
// behave as though executed in some sequentially consistent order.
// This definition provides the same semantics as
// C++'s sequentially consistent atomics and Java's volatile variables.
//
// Only a few integer sizes are supported: on many architectures,
// atomic operations on non-word-sized integers are inefficient or
// infeasible. For example, a [Bool] may be larger than a built-in bool.
//
// [the Go memory model]: https://go.dev/ref/mem
package atomic

import (
	"unsafe"
)

// BUG(rsc): On 386, the 64-bit functions use instructions unavailable before the Pentium MMX.
//
// On non-Linux ARM, the 64-bit functions use instructions unavailable before the ARMv6k core.
//
// On ARM, 386, and 32-bit MIPS, it is the caller's responsibility to arrange
// for 64-bit alignment of 64-bit words accessed atomically via the primitive
// atomic functions (types [Int64] and [Uint64] are automatically aligned).
// The first word in an allocated struct, array, or slice; in a global
// variable; or in a local variable (because on 32-bit architectures, the
// subject of 64-bit atomic operations will escape to the heap) can be
// relied upon to be 64-bit aligned.

// SwapInt32 atomically stores new into *addr and returns the previous *addr value.
// Consider using the more ergonomic and less error-prone [Int32.Swap] instead.
//
//go:noescape
func SwapInt32(addr *int32, new int32) (old int32)

// SwapUint32 atomically stores new into *addr and returns the previous *addr value.
// Consider using the more ergonomic and less error-prone [Uint32.Swap] instead.
//
//go:noescape
func SwapUint32(addr *uint32, new uint32) (old uint32)

// SwapUintptr atomically stores new into *addr and returns the previous *addr value.
// Consider using the more ergonomic and less error-prone [Uintptr.Swap] instead.
//
//go:noescape
//go:linknamestd SwapUintptr
func SwapUintptr(addr *uintptr, new uintptr) (old uintptr)

// SwapPointer atomically stores new into *addr and returns the previous *addr value.
// Consider using the more ergonomic and less error-prone [Pointer.Swap] instead.
func SwapPointer(addr *unsafe.Pointer, new unsafe.Pointer) (old unsafe.Pointer)

// CompareAndSwapInt32 executes the compare-and-swap operation for an int32 value.
// Consider using the more ergonomic and less error-prone [Int32.CompareAndSwap] instead.
//
//go:noescape
func CompareAndSwapInt32(addr *int32, old, new int32) (swapped bool)

// CompareAndSwapUint32 executes the compare-and-swap operation for a uint32 value.
// Consider using the more ergonomic and less error-prone [Uint32.CompareAndSwap] instead.
//
//go:noescape
func CompareAndSwapUint32(addr *uint32, old, new uint32) (swapped bool)

// CompareAndSwapUintptr executes the compare-and-swap operation for a uintptr value.
// Consider using the more ergonomic and less error-prone [Uintptr.CompareAndSwap] instead.
//
//go:noescape
//go:linknamestd CompareAndSwapUintptr
func CompareAndSwapUintptr(addr *uintptr, old, new uintptr) (swapped bool)

// CompareAndSwapPointer executes the compare-and-swap operation for a unsafe.Pointer value.
// Consider using the more ergonomic and less error-prone [Pointer.CompareAndSwap] instead.
func CompareAndSwapPointer(addr *unsafe.Pointer, old, new unsafe.Pointer) (swapped bool)

// AddInt32 atomically adds delta to *addr and returns the new value.
// Consider using the more ergonomic and less error-prone [Int32.Add] instead.
//
//go:noescape
func AddInt32(addr *int32, delta int32) (new int32)

// AddUint32 atomically adds delta to *addr and returns the new value.
// To subtract a signed positive constant value c from x, do AddUint32(&x, ^uint32(c-1)).
// In particular, to decrement x, do AddUint32(&x, ^uint32(0)).
// Consider using the more ergonomic and less error-prone [Uint32.Add] instead.
//
//go:noescape
func AddUint32(addr *uint32, delta uint32) (new uint32)

// AddUintptr atomically adds delta to *addr and returns the new value.
// Consider using the more ergonomic and less error-prone [Uintptr.Add] instead.
//
//go:noescape
func AddUintptr(addr *uintptr, delta uintptr) (new uintptr)

// AndInt32 atomically performs a bitwise AND operation on *addr using the bitmask provided as mask
// and returns the old value.
// Consider using the more ergonomic and less error-prone [Int32.And] instead.
//
//go:noescape
func AndInt32(addr *int32, mask int32) (old int32)

// AndUint32 atomically performs a bitwise AND operation on *addr using the bitmask provided as mask
// and returns the old value.
// Consider using the more ergonomic and less error-prone [Uint32.And] instead.
//
//go:noescape
func AndUint32(addr *uint32, mask uint32) (old uint32)

// AndUintptr atomically performs a bitwise AND operation on *addr using the bitmask provided as mask
// and returns the old value.
// Consider using the more ergonomic and less error-prone [Uintptr.And] instead.
//
//go:noescape
func AndUintptr(addr *uintptr, mask uintptr) (old uintptr)

// OrInt32 atomically performs a bitwise OR operation on *addr using the bitmask provided as mask
// and returns the old value.
// Consider using the more ergonomic and less error-prone [Int32.Or] instead.
//
//go:noescape
func OrInt32(addr *int32, mask int32) (old int32)

// OrUint32 atomically performs a bitwise OR operation on *addr using the bitmask provided as mask
// and returns the old value.
// Consider using the more ergonomic and less error-prone [Uint32.Or] instead.
//
//go:noescape
func OrUint32(addr *uint32, mask uint32) (old uint32)

// OrUintptr atomically performs a bitwise OR operation on *addr using the bitmask provided as mask
// and returns the old value.
// Consider using the more ergonomic and less error-prone [Uintptr.Or] instead.
//
//go:noescape
func OrUintptr(addr *uintptr, mask uintptr) (old uintptr)

// LoadInt32 atomically loads *addr.
// Consider using the more ergonomic and less error-prone [Int32.Load] instead.
//
//go:noescape
func LoadInt32(addr *int32) (val int32)

// LoadUint32 atomically loads *addr.
// Consider using the more ergonomic and less error-prone [Uint32.Load] instead.
//
//go:noescape
func LoadUint32(addr *uint32) (val uint32)

// LoadUintptr atomically loads *addr.
// Consider using the more ergonomic and less error-prone [Uintptr.Load] instead.
//
//go:noescape
func LoadUintptr(addr *uintptr) (val uintptr)

// LoadPointer atomically loads *addr.
// Consider using the more ergonomic and less error-prone [Pointer.Load] instead.
func LoadPointer(addr *unsafe.Pointer) (val unsafe.Pointer)

// StoreInt32 atomically stores val into *addr.
// Consider using the more ergonomic and less error-prone [Int32.Store] instead.
//
//go:noescape
func StoreInt32(addr *int32, val int32)

// StoreUint32 atomically stores val into *addr.
// Consider using the more ergonomic and less error-prone [Uint32.Store] instead.
//
//go:noescape
func StoreUint32(addr *uint32, val uint32)

// StoreUintptr atomically stores val into *addr.
// Consider using the more ergonomic and less error-prone [Uintptr.Store] instead.
//
//go:noescape
//go:linknamestd StoreUintptr
func StoreUintptr(addr *uintptr, val uintptr)

// StorePointer atomically stores val into *addr.
// Consider using the more ergonomic and less error-prone [Pointer.Store] instead.
func StorePointer(addr *unsafe.Pointer, val unsafe.Pointer)

```

// === FILE: references/go/src/sync/atomic/doc_32.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build 386 || arm || mips || mipsle

package atomic

// SwapInt64 atomically stores new into *addr and returns the previous *addr value.
// Consider using the more ergonomic and less error-prone [Int64.Swap] instead
// (particularly if you target 32-bit platforms; see the bugs section).
func SwapInt64(addr *int64, new int64) (old int64)

// SwapUint64 atomically stores new into *addr and returns the previous *addr value.
// Consider using the more ergonomic and less error-prone [Uint64.Swap] instead
// (particularly if you target 32-bit platforms; see the bugs section).
func SwapUint64(addr *uint64, new uint64) (old uint64)

// CompareAndSwapInt64 executes the compare-and-swap operation for an int64 value.
// Consider using the more ergonomic and less error-prone [Int64.CompareAndSwap] instead
// (particularly if you target 32-bit platforms; see the bugs section).
func CompareAndSwapInt64(addr *int64, old, new int64) (swapped bool)

// CompareAndSwapUint64 executes the compare-and-swap operation for a uint64 value.
// Consider using the more ergonomic and less error-prone [Uint64.CompareAndSwap] instead
// (particularly if you target 32-bit platforms; see the bugs section).
func CompareAndSwapUint64(addr *uint64, old, new uint64) (swapped bool)

// AddInt64 atomically adds delta to *addr and returns the new value.
// Consider using the more ergonomic and less error-prone [Int64.Add] instead
// (particularly if you target 32-bit platforms; see the bugs section).
func AddInt64(addr *int64, delta int64) (new int64)

// AddUint64 atomically adds delta to *addr and returns the new value.
// To subtract a signed positive constant value c from x, do AddUint64(&x, ^uint64(c-1)).
// In particular, to decrement x, do AddUint64(&x, ^uint64(0)).
// Consider using the more ergonomic and less error-prone [Uint64.Add] instead
// (particularly if you target 32-bit platforms; see the bugs section).
func AddUint64(addr *uint64, delta uint64) (new uint64)

// AndInt64 atomically performs a bitwise AND operation on *addr using the bitmask provided as mask
// and returns the old value.
// Consider using the more ergonomic and less error-prone [Int64.And] instead.
func AndInt64(addr *int64, mask int64) (old int64)

// AndUint64 atomically performs a bitwise AND operation on *addr using the bitmask provided as mask
// and returns the old.
// Consider using the more ergonomic and less error-prone [Uint64.And] instead.
func AndUint64(addr *uint64, mask uint64) (old uint64)

// OrInt64 atomically performs a bitwise OR operation on *addr using the bitmask provided as mask
// and returns the old value.
// Consider using the more ergonomic and less error-prone [Int64.Or] instead.
func OrInt64(addr *int64, mask int64) (old int64)

// OrUint64 atomically performs a bitwise OR operation on *addr using the bitmask provided as mask
// and returns the old value.
// Consider using the more ergonomic and less error-prone [Uint64.Or] instead.
func OrUint64(addr *uint64, mask uint64) (old uint64)

// LoadInt64 atomically loads *addr.
// Consider using the more ergonomic and less error-prone [Int64.Load] instead
// (particularly if you target 32-bit platforms; see the bugs section).
func LoadInt64(addr *int64) (val int64)

// LoadUint64 atomically loads *addr.
// Consider using the more ergonomic and less error-prone [Uint64.Load] instead
// (particularly if you target 32-bit platforms; see the bugs section).
func LoadUint64(addr *uint64) (val uint64)

// StoreInt64 atomically stores val into *addr.
// Consider using the more ergonomic and less error-prone [Int64.Store] instead
// (particularly if you target 32-bit platforms; see the bugs section).
func StoreInt64(addr *int64, val int64)

// StoreUint64 atomically stores val into *addr.
// Consider using the more ergonomic and less error-prone [Uint64.Store] instead
// (particularly if you target 32-bit platforms; see the bugs section).
func StoreUint64(addr *uint64, val uint64)

```

// === FILE: references/go/src/sync/atomic/doc_64.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !(386 || arm || mips || mipsle)

package atomic

// SwapInt64 atomically stores new into *addr and returns the previous *addr value.
// Consider using the more ergonomic and less error-prone [Int64.Swap] instead
// (particularly if you target 32-bit platforms; see the bugs section).
//
//go:noescape
func SwapInt64(addr *int64, new int64) (old int64)

// SwapUint64 atomically stores new into *addr and returns the previous *addr value.
// Consider using the more ergonomic and less error-prone [Uint64.Swap] instead
// (particularly if you target 32-bit platforms; see the bugs section).
//
//go:noescape
func SwapUint64(addr *uint64, new uint64) (old uint64)

// CompareAndSwapInt64 executes the compare-and-swap operation for an int64 value.
// Consider using the more ergonomic and less error-prone [Int64.CompareAndSwap] instead
// (particularly if you target 32-bit platforms; see the bugs section).
//
//go:noescape
func CompareAndSwapInt64(addr *int64, old, new int64) (swapped bool)

// CompareAndSwapUint64 executes the compare-and-swap operation for a uint64 value.
// Consider using the more ergonomic and less error-prone [Uint64.CompareAndSwap] instead
// (particularly if you target 32-bit platforms; see the bugs section).
//
//go:noescape
func CompareAndSwapUint64(addr *uint64, old, new uint64) (swapped bool)

// AddInt64 atomically adds delta to *addr and returns the new value.
// Consider using the more ergonomic and less error-prone [Int64.Add] instead
// (particularly if you target 32-bit platforms; see the bugs section).
//
//go:noescape
func AddInt64(addr *int64, delta int64) (new int64)

// AddUint64 atomically adds delta to *addr and returns the new value.
// To subtract a signed positive constant value c from x, do AddUint64(&x, ^uint64(c-1)).
// In particular, to decrement x, do AddUint64(&x, ^uint64(0)).
// Consider using the more ergonomic and less error-prone [Uint64.Add] instead
// (particularly if you target 32-bit platforms; see the bugs section).
//
//go:noescape
func AddUint64(addr *uint64, delta uint64) (new uint64)

// AndInt64 atomically performs a bitwise AND operation on *addr using the bitmask provided as mask
// and returns the old value.
// Consider using the more ergonomic and less error-prone [Int64.And] instead.
//
//go:noescape
func AndInt64(addr *int64, mask int64) (old int64)

// AndUint64 atomically performs a bitwise AND operation on *addr using the bitmask provided as mask
// and returns the old.
// Consider using the more ergonomic and less error-prone [Uint64.And] instead.
//
//go:noescape
func AndUint64(addr *uint64, mask uint64) (old uint64)

// OrInt64 atomically performs a bitwise OR operation on *addr using the bitmask provided as mask
// and returns the old value.
// Consider using the more ergonomic and less error-prone [Int64.Or] instead.
//
//go:noescape
func OrInt64(addr *int64, mask int64) (old int64)

// OrUint64 atomically performs a bitwise OR operation on *addr using the bitmask provided as mask
// and returns the old value.
// Consider using the more ergonomic and less error-prone [Uint64.Or] instead.
//
//go:noescape
func OrUint64(addr *uint64, mask uint64) (old uint64)

// LoadInt64 atomically loads *addr.
// Consider using the more ergonomic and less error-prone [Int64.Load] instead
// (particularly if you target 32-bit platforms; see the bugs section).
//
//go:noescape
func LoadInt64(addr *int64) (val int64)

// LoadUint64 atomically loads *addr.
// Consider using the more ergonomic and less error-prone [Uint64.Load] instead
// (particularly if you target 32-bit platforms; see the bugs section).
//
//go:noescape
func LoadUint64(addr *uint64) (val uint64)

// StoreInt64 atomically stores val into *addr.
// Consider using the more ergonomic and less error-prone [Int64.Store] instead
// (particularly if you target 32-bit platforms; see the bugs section).
//
//go:noescape
func StoreInt64(addr *int64, val int64)

// StoreUint64 atomically stores val into *addr.
// Consider using the more ergonomic and less error-prone [Uint64.Store] instead
// (particularly if you target 32-bit platforms; see the bugs section).
//
//go:noescape
func StoreUint64(addr *uint64, val uint64)

```

// === FILE: references/go/src/sync/atomic/race.s ===
```text
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build race

// This file is here only to allow external functions.
// The operations are implemented in src/runtime/race_amd64.s

```

// === FILE: references/go/src/sync/atomic/type.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package atomic

import "unsafe"

// A Bool is an atomic boolean value.
// The zero value is false.
//
// Bool must not be copied after first use.
type Bool struct {
	_ noCopy
	v uint32
}

// Load atomically loads and returns the value stored in x.
func (x *Bool) Load() bool { return LoadUint32(&x.v) != 0 }

// Store atomically stores val into x.
func (x *Bool) Store(val bool) { StoreUint32(&x.v, b32(val)) }

// Swap atomically stores new into x and returns the previous value.
func (x *Bool) Swap(new bool) (old bool) { return SwapUint32(&x.v, b32(new)) != 0 }

// CompareAndSwap executes the compare-and-swap operation for the boolean value x.
func (x *Bool) CompareAndSwap(old, new bool) (swapped bool) {
	return CompareAndSwapUint32(&x.v, b32(old), b32(new))
}

// b32 returns a uint32 0 or 1 representing b.
func b32(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}

// For testing *Pointer[T]'s methods can be inlined.
// Keep in sync with cmd/compile/internal/test/inl_test.go:TestIntendedInlining.
var _ = &Pointer[int]{}

// A Pointer is an atomic pointer of type *T. The zero value is a nil *T.
//
// Pointer must not be copied after first use.
type Pointer[T any] struct {
	// Mention *T in a field to disallow conversion between Pointer types.
	// See go.dev/issue/56603 for more details.
	// Use *T, not T, to avoid spurious recursive type definition errors.
	_ [0]*T

	_ noCopy
	v unsafe.Pointer
}

// Load atomically loads and returns the value stored in x.
func (x *Pointer[T]) Load() *T { return (*T)(LoadPointer(&x.v)) }

// Store atomically stores val into x.
func (x *Pointer[T]) Store(val *T) { StorePointer(&x.v, unsafe.Pointer(val)) }

// Swap atomically stores new into x and returns the previous value.
func (x *Pointer[T]) Swap(new *T) (old *T) { return (*T)(SwapPointer(&x.v, unsafe.Pointer(new))) }

// CompareAndSwap executes the compare-and-swap operation for x.
func (x *Pointer[T]) CompareAndSwap(old, new *T) (swapped bool) {
	return CompareAndSwapPointer(&x.v, unsafe.Pointer(old), unsafe.Pointer(new))
}

// An Int32 is an atomic int32. The zero value is zero.
//
// Int32 must not be copied after first use.
type Int32 struct {
	_ noCopy
	v int32
}

// Load atomically loads and returns the value stored in x.
func (x *Int32) Load() int32 { return LoadInt32(&x.v) }

// Store atomically stores val into x.
func (x *Int32) Store(val int32) { StoreInt32(&x.v, val) }

// Swap atomically stores new into x and returns the previous value.
func (x *Int32) Swap(new int32) (old int32) { return SwapInt32(&x.v, new) }

// CompareAndSwap executes the compare-and-swap operation for x.
func (x *Int32) CompareAndSwap(old, new int32) (swapped bool) {
	return CompareAndSwapInt32(&x.v, old, new)
}

// Add atomically adds delta to x and returns the new value.
func (x *Int32) Add(delta int32) (new int32) { return AddInt32(&x.v, delta) }

// And atomically performs a bitwise AND operation on x using the bitmask
// provided as mask and returns the old value.
func (x *Int32) And(mask int32) (old int32) { return AndInt32(&x.v, mask) }

// Or atomically performs a bitwise OR operation on x using the bitmask
// provided as mask and returns the old value.
func (x *Int32) Or(mask int32) (old int32) { return OrInt32(&x.v, mask) }

// An Int64 is an atomic int64. The zero value is zero.
//
// Int64 must not be copied after first use.
type Int64 struct {
	_ noCopy
	_ align64
	v int64
}

// Load atomically loads and returns the value stored in x.
func (x *Int64) Load() int64 { return LoadInt64(&x.v) }

// Store atomically stores val into x.
func (x *Int64) Store(val int64) { StoreInt64(&x.v, val) }

// Swap atomically stores new into x and returns the previous value.
func (x *Int64) Swap(new int64) (old int64) { return SwapInt64(&x.v, new) }

// CompareAndSwap executes the compare-and-swap operation for x.
func (x *Int64) CompareAndSwap(old, new int64) (swapped bool) {
	return CompareAndSwapInt64(&x.v, old, new)
}

// Add atomically adds delta to x and returns the new value.
func (x *Int64) Add(delta int64) (new int64) { return AddInt64(&x.v, delta) }

// And atomically performs a bitwise AND operation on x using the bitmask
// provided as mask and returns the old value.
func (x *Int64) And(mask int64) (old int64) { return AndInt64(&x.v, mask) }

// Or atomically performs a bitwise OR operation on x using the bitmask
// provided as mask and returns the old value.
func (x *Int64) Or(mask int64) (old int64) { return OrInt64(&x.v, mask) }

// A Uint32 is an atomic uint32. The zero value is zero.
//
// Uint32 must not be copied after first use.
type Uint32 struct {
	_ noCopy
	v uint32
}

// Load atomically loads and returns the value stored in x.
func (x *Uint32) Load() uint32 { return LoadUint32(&x.v) }

// Store atomically stores val into x.
func (x *Uint32) Store(val uint32) { StoreUint32(&x.v, val) }

// Swap atomically stores new into x and returns the previous value.
func (x *Uint32) Swap(new uint32) (old uint32) { return SwapUint32(&x.v, new) }

// CompareAndSwap executes the compare-and-swap operation for x.
func (x *Uint32) CompareAndSwap(old, new uint32) (swapped bool) {
	return CompareAndSwapUint32(&x.v, old, new)
}

// Add atomically adds delta to x and returns the new value.
func (x *Uint32) Add(delta uint32) (new uint32) { return AddUint32(&x.v, delta) }

// And atomically performs a bitwise AND operation on x using the bitmask
// provided as mask and returns the old value.
func (x *Uint32) And(mask uint32) (old uint32) { return AndUint32(&x.v, mask) }

// Or atomically performs a bitwise OR operation on x using the bitmask
// provided as mask and returns the old value.
func (x *Uint32) Or(mask uint32) (old uint32) { return OrUint32(&x.v, mask) }

// A Uint64 is an atomic uint64. The zero value is zero.
//
// Uint64 must not be copied after first use.
type Uint64 struct {
	_ noCopy
	_ align64
	v uint64
}

// Load atomically loads and returns the value stored in x.
func (x *Uint64) Load() uint64 { return LoadUint64(&x.v) }

// Store atomically stores val into x.
func (x *Uint64) Store(val uint64) { StoreUint64(&x.v, val) }

// Swap atomically stores new into x and returns the previous value.
func (x *Uint64) Swap(new uint64) (old uint64) { return SwapUint64(&x.v, new) }

// CompareAndSwap executes the compare-and-swap operation for x.
func (x *Uint64) CompareAndSwap(old, new uint64) (swapped bool) {
	return CompareAndSwapUint64(&x.v, old, new)
}

// Add atomically adds delta to x and returns the new value.
func (x *Uint64) Add(delta uint64) (new uint64) { return AddUint64(&x.v, delta) }

// And atomically performs a bitwise AND operation on x using the bitmask
// provided as mask and returns the old value.
func (x *Uint64) And(mask uint64) (old uint64) { return AndUint64(&x.v, mask) }

// Or atomically performs a bitwise OR operation on x using the bitmask
// provided as mask and returns the old value.
func (x *Uint64) Or(mask uint64) (old uint64) { return OrUint64(&x.v, mask) }

// A Uintptr is an atomic uintptr. The zero value is zero.
//
// Uintptr must not be copied after first use.
type Uintptr struct {
	_ noCopy
	v uintptr
}

// Load atomically loads and returns the value stored in x.
func (x *Uintptr) Load() uintptr { return LoadUintptr(&x.v) }

// Store atomically stores val into x.
func (x *Uintptr) Store(val uintptr) { StoreUintptr(&x.v, val) }

// Swap atomically stores new into x and returns the previous value.
func (x *Uintptr) Swap(new uintptr) (old uintptr) { return SwapUintptr(&x.v, new) }

// CompareAndSwap executes the compare-and-swap operation for x.
func (x *Uintptr) CompareAndSwap(old, new uintptr) (swapped bool) {
	return CompareAndSwapUintptr(&x.v, old, new)
}

// Add atomically adds delta to x and returns the new value.
func (x *Uintptr) Add(delta uintptr) (new uintptr) { return AddUintptr(&x.v, delta) }

// And atomically performs a bitwise AND operation on x using the bitmask
// provided as mask and returns the old value.
func (x *Uintptr) And(mask uintptr) (old uintptr) { return AndUintptr(&x.v, mask) }

// Or atomically performs a bitwise OR operation on x using the bitmask
// provided as mask and returns the old value.
func (x *Uintptr) Or(mask uintptr) (old uintptr) { return OrUintptr(&x.v, mask) }

// noCopy may be added to structs which must not be copied
// after the first use.
//
// See https://golang.org/issues/8005#issuecomment-190753527
// for details.
//
// Note that it must not be embedded, due to the Lock and Unlock methods.
type noCopy struct{}

// Lock is a no-op used by -copylocks checker from `go vet`.
func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

// align64 may be added to structs that must be 64-bit aligned.
// This struct is recognized by a special case in the compiler
// and will not work if copied to any other package.
type align64 struct{}

```

// === FILE: references/go/src/sync/atomic/value.go ===
```go
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package atomic

import (
	"unsafe"
)

// A Value provides an atomic load and store of a consistently typed value.
// The zero value for a Value returns nil from [Value.Load].
// Once [Value.Store] has been called, a Value must not be copied.
//
// A Value must not be copied after first use.
type Value struct {
	v any
}

// efaceWords is interface{} internal representation.
type efaceWords struct {
	typ  unsafe.Pointer
	data unsafe.Pointer
}

// Load returns the value set by the most recent Store.
// It returns nil if there has been no call to Store for this Value.
func (v *Value) Load() (val any) {
	vp := (*efaceWords)(unsafe.Pointer(v))
	typ := LoadPointer(&vp.typ)
	if typ == nil || typ == unsafe.Pointer(&firstStoreInProgress) {
		// First store not yet completed.
		return nil
	}
	data := LoadPointer(&vp.data)
	vlp := (*efaceWords)(unsafe.Pointer(&val))
	vlp.typ = typ
	vlp.data = data
	return
}

var firstStoreInProgress byte

// Store sets the value of the [Value] v to val.
// All calls to Store for a given Value must use values of the same concrete type.
// Store of an inconsistent type panics, as does Store(nil).
func (v *Value) Store(val any) {
	if val == nil {
		panic("sync/atomic: store of nil value into Value")
	}
	vp := (*efaceWords)(unsafe.Pointer(v))
	vlp := (*efaceWords)(unsafe.Pointer(&val))
	for {
		typ := LoadPointer(&vp.typ)
		if typ == nil {
			// Attempt to start first store.
			// Disable preemption so that other goroutines can use
			// active spin wait to wait for completion.
			runtime_procPin()
			if !CompareAndSwapPointer(&vp.typ, nil, unsafe.Pointer(&firstStoreInProgress)) {
				runtime_procUnpin()
				continue
			}
			// Complete first store.
			StorePointer(&vp.data, vlp.data)
			StorePointer(&vp.typ, vlp.typ)
			runtime_procUnpin()
			return
		}
		if typ == unsafe.Pointer(&firstStoreInProgress) {
			// First store in progress. Wait.
			// Since we disable preemption around the first store,
			// we can wait with active spinning.
			continue
		}
		// First store completed. Check type and overwrite data.
		if typ != vlp.typ {
			panic("sync/atomic: store of inconsistently typed value into Value")
		}
		StorePointer(&vp.data, vlp.data)
		return
	}
}

// Swap stores new into Value and returns the previous value. It returns nil if
// the Value is empty.
//
// All calls to Swap for a given Value must use values of the same concrete
// type. Swap of an inconsistent type panics, as does Swap(nil).
func (v *Value) Swap(new any) (old any) {
	if new == nil {
		panic("sync/atomic: swap of nil value into Value")
	}
	vp := (*efaceWords)(unsafe.Pointer(v))
	np := (*efaceWords)(unsafe.Pointer(&new))
	for {
		typ := LoadPointer(&vp.typ)
		if typ == nil {
			// Attempt to start first store.
			// Disable preemption so that other goroutines can use
			// active spin wait to wait for completion.
			runtime_procPin()
			if !CompareAndSwapPointer(&vp.typ, nil, unsafe.Pointer(&firstStoreInProgress)) {
				runtime_procUnpin()
				continue
			}
			// Complete first store.
			StorePointer(&vp.data, np.data)
			StorePointer(&vp.typ, np.typ)
			runtime_procUnpin()
			return nil
		}
		if typ == unsafe.Pointer(&firstStoreInProgress) {
			// First store in progress. Wait.
			// Since we disable preemption around the first store,
			// we can wait with active spinning.
			continue
		}
		// First store completed. Check type and overwrite data.
		if typ != np.typ {
			panic("sync/atomic: swap of inconsistently typed value into Value")
		}
		op := (*efaceWords)(unsafe.Pointer(&old))
		op.typ, op.data = np.typ, SwapPointer(&vp.data, np.data)
		return old
	}
}

// CompareAndSwap executes the compare-and-swap operation for the [Value].
//
// All calls to CompareAndSwap for a given Value must use values of the same
// concrete type. CompareAndSwap of an inconsistent type panics, as does
// CompareAndSwap(old, nil).
func (v *Value) CompareAndSwap(old, new any) (swapped bool) {
	if new == nil {
		panic("sync/atomic: compare and swap of nil value into Value")
	}
	vp := (*efaceWords)(unsafe.Pointer(v))
	np := (*efaceWords)(unsafe.Pointer(&new))
	op := (*efaceWords)(unsafe.Pointer(&old))
	if op.typ != nil && np.typ != op.typ {
		panic("sync/atomic: compare and swap of inconsistently typed values")
	}
	for {
		typ := LoadPointer(&vp.typ)
		if typ == nil {
			if old != nil {
				return false
			}
			// Attempt to start first store.
			// Disable preemption so that other goroutines can use
			// active spin wait to wait for completion.
			runtime_procPin()
			if !CompareAndSwapPointer(&vp.typ, nil, unsafe.Pointer(&firstStoreInProgress)) {
				runtime_procUnpin()
				continue
			}
			// Complete first store.
			StorePointer(&vp.data, np.data)
			StorePointer(&vp.typ, np.typ)
			runtime_procUnpin()
			return true
		}
		if typ == unsafe.Pointer(&firstStoreInProgress) {
			// First store in progress. Wait.
			// Since we disable preemption around the first store,
			// we can wait with active spinning.
			continue
		}
		// First store completed. Check type and overwrite data.
		if typ != np.typ {
			panic("sync/atomic: compare and swap of inconsistently typed value into Value")
		}
		// Compare old and current via runtime equality check.
		// This allows value types to be compared, something
		// not offered by the package functions.
		// CompareAndSwapPointer below only ensures vp.data
		// has not changed since LoadPointer.
		data := LoadPointer(&vp.data)
		var i any
		(*efaceWords)(unsafe.Pointer(&i)).typ = typ
		(*efaceWords)(unsafe.Pointer(&i)).data = data
		if i != old {
			return false
		}
		return CompareAndSwapPointer(&vp.data, data, np.data)
	}
}

// Disable/enable preemption, implemented in runtime.
func runtime_procPin() int
func runtime_procUnpin()

```

// === FILE: references/go/src/sync/cond.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"sync/atomic"
	"unsafe"
)

// Cond implements a condition variable, a rendezvous point
// for goroutines waiting for or announcing the occurrence
// of an event.
//
// Each Cond has an associated Locker L (often a [*Mutex] or [*RWMutex]),
// which must be held when changing the condition and
// when calling the [Cond.Wait] method.
//
// A Cond must not be copied after first use.
//
// In the terminology of [the Go memory model], Cond arranges that
// a call to [Cond.Broadcast] or [Cond.Signal] “synchronizes before” any Wait call
// that it unblocks.
//
// For many simple use cases, users will be better off using channels than a
// Cond (Broadcast corresponds to closing a channel, and Signal corresponds to
// sending on a channel).
//
// For more on replacements for [sync.Cond], see [Roberto Clapis's series on
// advanced concurrency patterns], as well as [Bryan Mills's talk on concurrency
// patterns].
//
// [the Go memory model]: https://go.dev/ref/mem
// [Roberto Clapis's series on advanced concurrency patterns]: https://blogtitle.github.io/categories/concurrency/
// [Bryan Mills's talk on concurrency patterns]: https://drive.google.com/file/d/1nPdvhB0PutEJzdCq5ms6UI58dp50fcAN/view
type Cond struct {
	noCopy noCopy

	// L is held while observing or changing the condition
	L Locker

	notify  notifyList
	checker copyChecker
}

// NewCond returns a new Cond with Locker l.
func NewCond(l Locker) *Cond {
	return &Cond{L: l}
}

// Wait atomically unlocks c.L and suspends execution
// of the calling goroutine. After later resuming execution,
// Wait locks c.L before returning. Unlike in other systems,
// Wait cannot return unless awoken by [Cond.Broadcast] or [Cond.Signal].
//
// Because c.L is not locked while Wait is waiting, the caller
// typically cannot assume that the condition is true when
// Wait returns. Instead, the caller should Wait in a loop:
//
//	c.L.Lock()
//	for !condition() {
//	    c.Wait()
//	}
//	... make use of condition ...
//	c.L.Unlock()
func (c *Cond) Wait() {
	c.checker.check()
	t := runtime_notifyListAdd(&c.notify)
	c.L.Unlock()
	runtime_notifyListWait(&c.notify, t)
	c.L.Lock()
}

// Signal wakes one goroutine waiting on c, if there is any.
//
// It is allowed but not required for the caller to hold c.L
// during the call.
//
// Signal() does not affect goroutine scheduling priority; if other goroutines
// are attempting to lock c.L, they may be awoken before a "waiting" goroutine.
func (c *Cond) Signal() {
	c.checker.check()
	runtime_notifyListNotifyOne(&c.notify)
}

// Broadcast wakes all goroutines waiting on c.
//
// It is allowed but not required for the caller to hold c.L during the call.
// The time it takes to run Broadcast is proportional to the number of waiting goroutines;
// be aware that holding the lock across a call to Broadcast
// will extend the amount of time that the lock is held.
func (c *Cond) Broadcast() {
	c.checker.check()
	runtime_notifyListNotifyAll(&c.notify)
}

// copyChecker holds back pointer to itself to detect object copying.
type copyChecker uintptr

func (c *copyChecker) check() {
	// Check if c has been copied in three steps:
	// 1. The first comparison is the fast-path. If c has been initialized and not copied, this will return immediately. Otherwise, c is either not initialized, or has been copied.
	// 2. Ensure c is initialized. If the CAS succeeds, we're done. If it fails, c was either initialized concurrently and we simply lost the race, or c has been copied.
	// 3. Do step 1 again. Now that c is definitely initialized, if this fails, c was copied.
	if uintptr(*c) != uintptr(unsafe.Pointer(c)) &&
		!atomic.CompareAndSwapUintptr((*uintptr)(c), 0, uintptr(unsafe.Pointer(c))) &&
		uintptr(*c) != uintptr(unsafe.Pointer(c)) {
		panic("sync.Cond is copied")
	}
}

// noCopy may be added to structs which must not be copied
// after the first use.
//
// See https://golang.org/issues/8005#issuecomment-190753527
// for details.
//
// Note that it must not be embedded, due to the Lock and Unlock methods.
type noCopy struct{}

// Lock is a no-op used by -copylocks checker from `go vet`.
func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

```

// === FILE: references/go/src/sync/map.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	isync "internal/sync"
)

// Map is like a Go map[any]any but is safe for concurrent use
// by multiple goroutines without additional locking or coordination.
// Loads, stores, and deletes run in amortized constant time.
//
// The Map type is specialized. Most code should use a plain Go map instead,
// with separate locking or coordination, for better type safety and to make it
// easier to maintain other invariants along with the map content.
//
// The Map type is optimized for two common use cases: (1) when the entry for a given
// key is only ever written once but read many times, as in caches that only grow,
// or (2) when multiple goroutines read, write, and overwrite entries for disjoint
// sets of keys. In these two cases, use of a Map may significantly reduce lock
// contention compared to a Go map paired with a separate [Mutex] or [RWMutex].
//
// The zero Map is empty and ready for use. A Map must not be copied after first use.
//
// In the terminology of [the Go memory model], Map arranges that a write operation
// “synchronizes before” any read operation that observes the effect of the write, where
// read and write operations are defined as follows.
// [Map.Load], [Map.LoadAndDelete], [Map.LoadOrStore], [Map.Swap], [Map.CompareAndSwap],
// and [Map.CompareAndDelete] are read operations;
// [Map.Delete], [Map.LoadAndDelete], [Map.Store], and [Map.Swap] are write operations;
// [Map.LoadOrStore] is a write operation when it returns loaded set to false;
// [Map.CompareAndSwap] is a write operation when it returns swapped set to true;
// and [Map.CompareAndDelete] is a write operation when it returns deleted set to true.
//
// [the Go memory model]: https://go.dev/ref/mem
type Map struct {
	_ noCopy

	m isync.HashTrieMap[any, any]
}

// Load returns the value stored in the map for a key, or nil if no
// value is present.
// The ok result indicates whether value was found in the map.
func (m *Map) Load(key any) (value any, ok bool) {
	return m.m.Load(key)
}

// Store sets the value for a key.
func (m *Map) Store(key, value any) {
	m.m.Store(key, value)
}

// Clear deletes all the entries, resulting in an empty Map.
func (m *Map) Clear() {
	m.m.Clear()
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
func (m *Map) LoadOrStore(key, value any) (actual any, loaded bool) {
	return m.m.LoadOrStore(key, value)
}

// LoadAndDelete deletes the value for a key, returning the previous value if any.
// The loaded result reports whether the key was present.
func (m *Map) LoadAndDelete(key any) (value any, loaded bool) {
	return m.m.LoadAndDelete(key)
}

// Delete deletes the value for a key.
// If the key is not in the map, Delete does nothing.
func (m *Map) Delete(key any) {
	m.m.Delete(key)
}

// Swap swaps the value for a key and returns the previous value if any.
// The loaded result reports whether the key was present.
func (m *Map) Swap(key, value any) (previous any, loaded bool) {
	return m.m.Swap(key, value)
}

// CompareAndSwap swaps the old and new values for key
// if the value stored in the map is equal to old.
// The old value must be of a comparable type.
func (m *Map) CompareAndSwap(key, old, new any) (swapped bool) {
	return m.m.CompareAndSwap(key, old, new)
}

// CompareAndDelete deletes the entry for key if its value is equal to old.
// The old value must be of a comparable type.
//
// If there is no current value for key in the map, CompareAndDelete
// returns false (even if the old value is the nil interface value).
func (m *Map) CompareAndDelete(key, old any) (deleted bool) {
	return m.m.CompareAndDelete(key, old)
}

// Range calls f sequentially for each key and value present in the map.
// If f returns false, range stops the iteration.
//
// Range does not necessarily correspond to any consistent snapshot of the Map's
// contents: no key will be visited more than once, but if the value for any key
// is stored or deleted concurrently (including by f), Range may reflect any
// mapping for that key from any point during the Range call. Range does not
// block other methods on the receiver; even f itself may call any method on m.
//
// Range may be O(N) with the number of elements in the map even if f returns
// false after a constant number of calls.
func (m *Map) Range(f func(key, value any) bool) {
	m.m.Range(f)
}

```

// === FILE: references/go/src/sync/mutex.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sync provides basic synchronization primitives such as mutual
// exclusion locks. Other than the [Once] and [WaitGroup] types, most are intended
// for use by low-level library routines. Higher-level synchronization is
// better done via channels and communication.
//
// Values containing the types defined in this package should not be copied.
package sync

import (
	isync "internal/sync"
)

// A Mutex is a mutual exclusion lock.
// The zero value for a Mutex is an unlocked mutex.
//
// A Mutex must not be copied after first use.
//
// In the terminology of [the Go memory model],
// the n'th call to [Mutex.Unlock] “synchronizes before” the m'th call to [Mutex.Lock]
// for any n < m.
// A successful call to [Mutex.TryLock] is equivalent to a call to Lock.
// A failed call to TryLock does not establish any “synchronizes before”
// relation at all.
//
// [the Go memory model]: https://go.dev/ref/mem
type Mutex struct {
	_ noCopy

	mu isync.Mutex
}

// A Locker represents an object that can be locked and unlocked.
type Locker interface {
	Lock()
	Unlock()
}

// Lock locks m.
// If the lock is already in use, the calling goroutine
// blocks until the mutex is available.
func (m *Mutex) Lock() {
	m.mu.Lock()
}

// TryLock tries to lock m and reports whether it succeeded.
//
// Note that while correct uses of TryLock do exist, they are rare,
// and use of TryLock is often a sign of a deeper problem
// in a particular use of mutexes.
func (m *Mutex) TryLock() bool {
	return m.mu.TryLock()
}

// Unlock unlocks m.
// It is a run-time error if m is not locked on entry to Unlock.
//
// A locked [Mutex] is not associated with a particular goroutine.
// It is allowed for one goroutine to lock a Mutex and then
// arrange for another goroutine to unlock it.
func (m *Mutex) Unlock() {
	m.mu.Unlock()
}

```

// === FILE: references/go/src/sync/once.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"sync/atomic"
)

// Once is an object that will perform exactly one action.
//
// A Once must not be copied after first use.
//
// In the terminology of [the Go memory model],
// the return from f “synchronizes before”
// the return from any call of once.Do(f).
//
// [the Go memory model]: https://go.dev/ref/mem
type Once struct {
	_ noCopy

	// done indicates whether the action has been performed.
	// It is first in the struct because it is used in the hot path.
	// The hot path is inlined at every call site.
	// Placing done first allows more compact instructions on some architectures (amd64/386),
	// and fewer instructions (to calculate offset) on other architectures.
	done atomic.Bool
	m    Mutex
}

// Do calls the function f if and only if Do is being called for the
// first time for this instance of [Once]. In other words, given
//
//	var once Once
//
// if once.Do(f) is called multiple times, only the first call will invoke f,
// even if f has a different value in each invocation. A new instance of
// Once is required for each function to execute.
//
// Do is intended for initialization that must be run exactly once. Since f
// is niladic, it may be necessary to use a function literal to capture the
// arguments to a function to be invoked by Do:
//
//	config.once.Do(func() { config.init(filename) })
//
// Because no call to Do returns until the one call to f returns, if f causes
// Do to be called, it will deadlock.
//
// If f panics, Do considers it to have returned; future calls of Do return
// without calling f.
func (o *Once) Do(f func()) {
	// Note: Here is an incorrect implementation of Do:
	//
	//	if o.done.CompareAndSwap(false, true) {
	//		f()
	//	}
	//
	// Do guarantees that when it returns, f has finished.
	// This implementation would not implement that guarantee:
	// given two simultaneous calls, the winner of the cas would
	// call f, and the second would return immediately, without
	// waiting for the first's call to f to complete.
	// This is why the slow path falls back to a mutex, and why
	// the o.done.Store must be delayed until after f returns.

	if !o.done.Load() {
		// Outlined slow-path to allow inlining of the fast-path.
		o.doSlow(f)
	}
}

func (o *Once) doSlow(f func()) {
	o.m.Lock()
	defer o.m.Unlock()
	if !o.done.Load() {
		defer o.done.Store(true)
		f()
	}
}

```

// === FILE: references/go/src/sync/oncefunc.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

// OnceFunc returns a function that invokes f only once. The returned function
// may be called concurrently.
//
// If f panics, the returned function will panic with the same value on every call.
func OnceFunc(f func()) func() {
	// Use a struct so that there's a single heap allocation.
	d := struct {
		f     func()
		once  Once
		valid bool
		p     any
	}{
		f: f,
	}
	return func() {
		d.once.Do(func() {
			defer func() {
				d.f = nil // Do not keep f alive after invoking it.
				d.p = recover()
				if !d.valid {
					// Re-panic immediately so on the first
					// call the user gets a complete stack
					// trace into f.
					panic(d.p)
				}
			}()
			d.f()
			d.valid = true // Set only if f does not panic.
		})
		if !d.valid {
			panic(d.p)
		}
	}
}

// OnceValue returns a function that invokes f only once and returns the value
// returned by f. The returned function may be called concurrently.
//
// If f panics, the returned function will panic with the same value on every call.
func OnceValue[T any](f func() T) func() T {
	// Use a struct so that there's a single heap allocation.
	d := struct {
		f      func() T
		once   Once
		valid  bool
		p      any
		result T
	}{
		f: f,
	}
	return func() T {
		d.once.Do(func() {
			defer func() {
				d.f = nil
				d.p = recover()
				if !d.valid {
					panic(d.p)
				}
			}()
			d.result = d.f()
			d.valid = true
		})
		if !d.valid {
			panic(d.p)
		}
		return d.result
	}
}

// OnceValues returns a function that invokes f only once and returns the values
// returned by f. The returned function may be called concurrently.
//
// If f panics, the returned function will panic with the same value on every call.
func OnceValues[T1, T2 any](f func() (T1, T2)) func() (T1, T2) {
	// Use a struct so that there's a single heap allocation.
	d := struct {
		f     func() (T1, T2)
		once  Once
		valid bool
		p     any
		r1    T1
		r2    T2
	}{
		f: f,
	}
	return func() (T1, T2) {
		d.once.Do(func() {
			defer func() {
				d.f = nil
				d.p = recover()
				if !d.valid {
					panic(d.p)
				}
			}()
			d.r1, d.r2 = d.f()
			d.valid = true
		})
		if !d.valid {
			panic(d.p)
		}
		return d.r1, d.r2
	}
}

```

// === FILE: references/go/src/sync/pool.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"internal/race"
	rtatomic "internal/runtime/atomic"
	"runtime"
	"sync/atomic"
	"unsafe"
)

// A Pool is a set of temporary objects that may be individually saved and
// retrieved.
//
// Any item stored in the Pool may be removed automatically at any time without
// notification. If the Pool holds the only reference when this happens, the
// item might be deallocated.
//
// A Pool is safe for use by multiple goroutines simultaneously.
//
// Pool's purpose is to cache allocated but unused items for later reuse,
// relieving pressure on the garbage collector. That is, it makes it easy to
// build efficient, thread-safe free lists. However, it is not suitable for all
// free lists.
//
// An appropriate use of a Pool is to manage a group of temporary items
// silently shared among and potentially reused by concurrent independent
// clients of a package. Pool provides a way to amortize allocation overhead
// across many clients.
//
// An example of good use of a Pool is in the fmt package, which maintains a
// dynamically-sized store of temporary output buffers. The store scales under
// load (when many goroutines are actively printing) and shrinks when
// quiescent.
//
// On the other hand, a free list maintained as part of a short-lived object is
// not a suitable use for a Pool, since the overhead does not amortize well in
// that scenario. It is more efficient to have such objects implement their own
// free list.
//
// A Pool must not be copied after first use.
//
// In the terminology of [the Go memory model], a call to Put(x) “synchronizes before”
// a call to [Pool.Get] returning that same value x.
// Similarly, a call to New returning x “synchronizes before”
// a call to Get returning that same value x.
//
// [the Go memory model]: https://go.dev/ref/mem
type Pool struct {
	noCopy noCopy

	local     unsafe.Pointer // local fixed-size per-P pool, actual type is [P]poolLocal
	localSize uintptr        // size of the local array

	victim     unsafe.Pointer // local from previous cycle
	victimSize uintptr        // size of victims array

	// New optionally specifies a function to generate
	// a value when Get would otherwise return nil.
	// It may not be changed concurrently with calls to Get.
	New func() any
}

// Local per-P Pool appendix.
type poolLocalInternal struct {
	private any       // Can be used only by the respective P.
	shared  poolChain // Local P can pushHead/popHead; any P can popTail.
}

type poolLocal struct {
	poolLocalInternal

	// Prevents false sharing on widespread platforms with
	// 128 mod (cache line size) = 0 .
	pad [128 - unsafe.Sizeof(poolLocalInternal{})%128]byte
}

// from runtime
//
//go:linkname runtime_randn runtime.randn
func runtime_randn(n uint32) uint32

var poolRaceHash [128]uint64

// poolRaceAddr returns an address to use as the synchronization point
// for race detector logic. We don't use the actual pointer stored in x
// directly, for fear of conflicting with other synchronization on that address.
// Instead, we hash the pointer to get an index into poolRaceHash.
// See discussion on golang.org/cl/31589.
func poolRaceAddr(x any) unsafe.Pointer {
	ptr := uintptr((*[2]unsafe.Pointer)(unsafe.Pointer(&x))[1])
	h := uint32((uint64(uint32(ptr)) * 0x85ebca6b) >> 16)
	return unsafe.Pointer(&poolRaceHash[h%uint32(len(poolRaceHash))])
}

// Put adds x to the pool.
func (p *Pool) Put(x any) {
	if x == nil {
		return
	}
	if race.Enabled {
		if runtime_randn(4) == 0 {
			// Randomly drop x on floor.
			return
		}
		race.ReleaseMerge(poolRaceAddr(x))
		race.Disable()
	}
	l, _ := p.pin()
	if l.private == nil {
		l.private = x
	} else {
		l.shared.pushHead(x)
	}
	runtime_procUnpin()
	if race.Enabled {
		race.Enable()
	}
}

// Get selects an arbitrary item from the [Pool], removes it from the
// Pool, and returns it to the caller.
// Get may choose to ignore the pool and treat it as empty.
// Callers should not assume any relation between values passed to [Pool.Put] and
// the values returned by Get.
//
// If Get would otherwise return nil and p.New is non-nil, Get returns
// the result of calling p.New.
func (p *Pool) Get() any {
	if race.Enabled {
		race.Disable()
	}
	l, pid := p.pin()
	x := l.private
	l.private = nil
	if x == nil {
		// Try to pop the head of the local shard. We prefer
		// the head over the tail for temporal locality of
		// reuse.
		x, _ = l.shared.popHead()
		if x == nil {
			x = p.getSlow(pid)
		}
	}
	runtime_procUnpin()
	if race.Enabled {
		race.Enable()
		if x != nil {
			race.Acquire(poolRaceAddr(x))
		}
	}
	if x == nil && p.New != nil {
		x = p.New()
	}
	return x
}

func (p *Pool) getSlow(pid int) any {
	// See the comment in pin regarding ordering of the loads.
	size := rtatomic.LoadAcquintptr(&p.localSize) // load-acquire
	locals := p.local                             // load-consume
	// Try to steal one element from other procs.
	for i := 0; i < int(size); i++ {
		l := indexLocal(locals, (pid+i+1)%int(size))
		if x, _ := l.shared.popTail(); x != nil {
			return x
		}
	}

	// Try the victim cache. We do this after attempting to steal
	// from all primary caches because we want objects in the
	// victim cache to age out if at all possible.
	size = atomic.LoadUintptr(&p.victimSize)
	if uintptr(pid) >= size {
		return nil
	}
	locals = p.victim
	l := indexLocal(locals, pid)
	if x := l.private; x != nil {
		l.private = nil
		return x
	}
	for i := 0; i < int(size); i++ {
		l := indexLocal(locals, (pid+i)%int(size))
		if x, _ := l.shared.popTail(); x != nil {
			return x
		}
	}

	// Mark the victim cache as empty for future gets don't bother
	// with it.
	atomic.StoreUintptr(&p.victimSize, 0)

	return nil
}

// pin pins the current goroutine to P, disables preemption and
// returns poolLocal pool for the P and the P's id.
// Caller must call runtime_procUnpin() when done with the pool.
func (p *Pool) pin() (*poolLocal, int) {
	// Check whether p is nil to get a panic.
	// Otherwise the nil dereference happens while the m is pinned,
	// causing a fatal error rather than a panic.
	if p == nil {
		panic("nil Pool")
	}

	pid := runtime_procPin()
	// In pinSlow we store to local and then to localSize, here we load in opposite order.
	// Since we've disabled preemption, GC cannot happen in between.
	// Thus here we must observe local at least as large localSize.
	// We can observe a newer/larger local, it is fine (we must observe its zero-initialized-ness).
	s := rtatomic.LoadAcquintptr(&p.localSize) // load-acquire
	l := p.local                               // load-consume
	if uintptr(pid) < s {
		return indexLocal(l, pid), pid
	}
	return p.pinSlow()
}

func (p *Pool) pinSlow() (*poolLocal, int) {
	// Retry under the mutex.
	// Can not lock the mutex while pinned.
	runtime_procUnpin()
	allPoolsMu.Lock()
	defer allPoolsMu.Unlock()
	pid := runtime_procPin()
	// poolCleanup won't be called while we are pinned.
	s := p.localSize
	l := p.local
	if uintptr(pid) < s {
		return indexLocal(l, pid), pid
	}
	if p.local == nil {
		allPools = append(allPools, p)
	}
	// If GOMAXPROCS changes between GCs, we re-allocate the array and lose the old one.
	size := runtime.GOMAXPROCS(0)
	local := make([]poolLocal, size)
	atomic.StorePointer(&p.local, unsafe.Pointer(&local[0])) // store-release
	rtatomic.StoreReluintptr(&p.localSize, uintptr(size))    // store-release
	return &local[pid], pid
}

// poolCleanup should be an internal detail,
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/bytedance/gopkg
//   - github.com/songzhibin97/gkit
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
//go:linkname poolCleanup
func poolCleanup() {
	// This function is called with the world stopped, at the beginning of a garbage collection.
	// It must not allocate and probably should not call any runtime functions.

	// Because the world is stopped, no pool user can be in a
	// pinned section (in effect, this has all Ps pinned).

	// Drop victim caches from all pools.
	for _, p := range oldPools {
		p.victim = nil
		p.victimSize = 0
	}

	// Move primary cache to victim cache.
	for _, p := range allPools {
		p.victim = p.local
		p.victimSize = p.localSize
		p.local = nil
		p.localSize = 0
	}

	// The pools with non-empty primary caches now have non-empty
	// victim caches and no pools have primary caches.
	oldPools, allPools = allPools, nil
}

var (
	allPoolsMu Mutex

	// allPools is the set of pools that have non-empty primary
	// caches. Protected by either 1) allPoolsMu and pinning or 2)
	// STW.
	allPools []*Pool

	// oldPools is the set of pools that may have non-empty victim
	// caches. Protected by STW.
	oldPools []*Pool
)

func init() {
	runtime_registerPoolCleanup(poolCleanup)
}

func indexLocal(l unsafe.Pointer, i int) *poolLocal {
	lp := unsafe.Pointer(uintptr(l) + uintptr(i)*unsafe.Sizeof(poolLocal{}))
	return (*poolLocal)(lp)
}

// Implemented in runtime.
func runtime_registerPoolCleanup(cleanup func())
func runtime_procPin() int
func runtime_procUnpin()

```

// === FILE: references/go/src/sync/poolqueue.go ===
```go
// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"sync/atomic"
	"unsafe"
)

// poolDequeue is a lock-free fixed-size single-producer,
// multi-consumer queue. The single producer can both push and pop
// from the head, and consumers can pop from the tail.
//
// It has the added feature that it nils out unused slots to avoid
// unnecessary retention of objects. This is important for sync.Pool,
// but not typically a property considered in the literature.
type poolDequeue struct {
	// headTail packs together a 32-bit head index and a 32-bit
	// tail index. Both are indexes into vals modulo len(vals)-1.
	//
	// tail = index of oldest data in queue
	// head = index of next slot to fill
	//
	// Slots in the range [tail, head) are owned by consumers.
	// A consumer continues to own a slot outside this range until
	// it nils the slot, at which point ownership passes to the
	// producer.
	//
	// The head index is stored in the most-significant bits so
	// that we can atomically add to it and the overflow is
	// harmless.
	headTail atomic.Uint64

	// vals is a ring buffer of interface{} values stored in this
	// dequeue. The size of this must be a power of 2.
	//
	// vals[i].typ is nil if the slot is empty and non-nil
	// otherwise. A slot is still in use until *both* the tail
	// index has moved beyond it and typ has been set to nil. This
	// is set to nil atomically by the consumer and read
	// atomically by the producer.
	vals []eface
}

type eface struct {
	typ, val unsafe.Pointer
}

const dequeueBits = 32

// dequeueLimit is the maximum size of a poolDequeue.
//
// This must be at most (1<<dequeueBits)/2 because detecting fullness
// depends on wrapping around the ring buffer without wrapping around
// the index. We divide by 4 so this fits in an int on 32-bit.
const dequeueLimit = (1 << dequeueBits) / 4

// dequeueNil is used in poolDequeue to represent interface{}(nil).
// Since we use nil to represent empty slots, we need a sentinel value
// to represent nil.
type dequeueNil *struct{}

func (d *poolDequeue) unpack(ptrs uint64) (head, tail uint32) {
	const mask = 1<<dequeueBits - 1
	head = uint32((ptrs >> dequeueBits) & mask)
	tail = uint32(ptrs & mask)
	return
}

func (d *poolDequeue) pack(head, tail uint32) uint64 {
	const mask = 1<<dequeueBits - 1
	return (uint64(head) << dequeueBits) |
		uint64(tail&mask)
}

// pushHead adds val at the head of the queue. It returns false if the
// queue is full. It must only be called by a single producer.
func (d *poolDequeue) pushHead(val any) bool {
	ptrs := d.headTail.Load()
	head, tail := d.unpack(ptrs)
	if (tail+uint32(len(d.vals)))&(1<<dequeueBits-1) == head {
		// Queue is full.
		return false
	}
	slot := &d.vals[head&uint32(len(d.vals)-1)]

	// Check if the head slot has been released by popTail.
	typ := atomic.LoadPointer(&slot.typ)
	if typ != nil {
		// Another goroutine is still cleaning up the tail, so
		// the queue is actually still full.
		return false
	}

	// The head slot is free, so we own it.
	if val == nil {
		val = dequeueNil(nil)
	}
	*(*any)(unsafe.Pointer(slot)) = val

	// Increment head. This passes ownership of slot to popTail
	// and acts as a store barrier for writing the slot.
	d.headTail.Add(1 << dequeueBits)
	return true
}

// popHead removes and returns the element at the head of the queue.
// It returns false if the queue is empty. It must only be called by a
// single producer.
func (d *poolDequeue) popHead() (any, bool) {
	var slot *eface
	for {
		ptrs := d.headTail.Load()
		head, tail := d.unpack(ptrs)
		if tail == head {
			// Queue is empty.
			return nil, false
		}

		// Confirm tail and decrement head. We do this before
		// reading the value to take back ownership of this
		// slot.
		head--
		ptrs2 := d.pack(head, tail)
		if d.headTail.CompareAndSwap(ptrs, ptrs2) {
			// We successfully took back slot.
			slot = &d.vals[head&uint32(len(d.vals)-1)]
			break
		}
	}

	val := *(*any)(unsafe.Pointer(slot))
	if val == dequeueNil(nil) {
		val = nil
	}
	// Zero the slot. Unlike popTail, this isn't racing with
	// pushHead, so we don't need to be careful here.
	*slot = eface{}
	return val, true
}

// popTail removes and returns the element at the tail of the queue.
// It returns false if the queue is empty. It may be called by any
// number of consumers.
func (d *poolDequeue) popTail() (any, bool) {
	var slot *eface
	for {
		ptrs := d.headTail.Load()
		head, tail := d.unpack(ptrs)
		if tail == head {
			// Queue is empty.
			return nil, false
		}

		// Confirm head and tail (for our speculative check
		// above) and increment tail. If this succeeds, then
		// we own the slot at tail.
		ptrs2 := d.pack(head, tail+1)
		if d.headTail.CompareAndSwap(ptrs, ptrs2) {
			// Success.
			slot = &d.vals[tail&uint32(len(d.vals)-1)]
			break
		}
	}

	// We now own slot.
	val := *(*any)(unsafe.Pointer(slot))
	if val == dequeueNil(nil) {
		val = nil
	}

	// Tell pushHead that we're done with this slot. Zeroing the
	// slot is also important so we don't leave behind references
	// that could keep this object live longer than necessary.
	//
	// We write to val first and then publish that we're done with
	// this slot by atomically writing to typ.
	slot.val = nil
	atomic.StorePointer(&slot.typ, nil)
	// At this point pushHead owns the slot.

	return val, true
}

// poolChain is a dynamically-sized version of poolDequeue.
//
// This is implemented as a doubly-linked list queue of poolDequeues
// where each dequeue is double the size of the previous one. Once a
// dequeue fills up, this allocates a new one and only ever pushes to
// the latest dequeue. Pops happen from the other end of the list and
// once a dequeue is exhausted, it gets removed from the list.
type poolChain struct {
	// head is the poolDequeue to push to. This is only accessed
	// by the producer, so doesn't need to be synchronized.
	head *poolChainElt

	// tail is the poolDequeue to popTail from. This is accessed
	// by consumers, so reads and writes must be atomic.
	tail atomic.Pointer[poolChainElt]
}

type poolChainElt struct {
	poolDequeue

	// next and prev link to the adjacent poolChainElts in this
	// poolChain.
	//
	// next is written atomically by the producer and read
	// atomically by the consumer. It only transitions from nil to
	// non-nil.
	//
	// prev is written atomically by the consumer and read
	// atomically by the producer. It only transitions from
	// non-nil to nil.
	next, prev atomic.Pointer[poolChainElt]
}

func (c *poolChain) pushHead(val any) {
	d := c.head
	if d == nil {
		// Initialize the chain.
		const initSize = 8 // Must be a power of 2
		d = new(poolChainElt)
		d.vals = make([]eface, initSize)
		c.head = d
		c.tail.Store(d)
	}

	if d.pushHead(val) {
		return
	}

	// The current dequeue is full. Allocate a new one of twice
	// the size.
	newSize := len(d.vals) * 2
	if newSize >= dequeueLimit {
		// Can't make it any bigger.
		newSize = dequeueLimit
	}

	d2 := &poolChainElt{}
	d2.prev.Store(d)
	d2.vals = make([]eface, newSize)
	c.head = d2
	d.next.Store(d2)
	d2.pushHead(val)
}

func (c *poolChain) popHead() (any, bool) {
	d := c.head
	for d != nil {
		if val, ok := d.popHead(); ok {
			return val, ok
		}
		// There may still be unconsumed elements in the
		// previous dequeue, so try backing up.
		d = d.prev.Load()
	}
	return nil, false
}

func (c *poolChain) popTail() (any, bool) {
	d := c.tail.Load()
	if d == nil {
		return nil, false
	}

	for {
		// It's important that we load the next pointer
		// *before* popping the tail. In general, d may be
		// transiently empty, but if next is non-nil before
		// the pop and the pop fails, then d is permanently
		// empty, which is the only condition under which it's
		// safe to drop d from the chain.
		d2 := d.next.Load()

		if val, ok := d.popTail(); ok {
			return val, ok
		}

		if d2 == nil {
			// This is the only dequeue. It's empty right
			// now, but could be pushed to in the future.
			return nil, false
		}

		// The tail of the chain has been drained, so move on
		// to the next dequeue. Try to drop it from the chain
		// so the next pop doesn't have to look at the empty
		// dequeue again.
		if c.tail.CompareAndSwap(d, d2) {
			// We won the race. Clear the prev pointer so
			// the garbage collector can collect the empty
			// dequeue and so popHead doesn't back up
			// further than necessary.
			d2.prev.Store(nil)
		}
		d = d2
	}
}

```

// === FILE: references/go/src/sync/runtime.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import "unsafe"

// defined in package runtime

// Semacquire waits until *s > 0 and then atomically decrements it.
// It is intended as a simple sleep primitive for use by the synchronization
// library and should not be used directly.
func runtime_Semacquire(s *uint32)

// SemacquireWaitGroup is like Semacquire, but for WaitGroup.Wait.
func runtime_SemacquireWaitGroup(s *uint32, synctestDurable bool)

// Semacquire(RW)Mutex(R) is like Semacquire, but for profiling contended
// Mutexes and RWMutexes.
// If lifo is true, queue waiter at the head of wait queue.
// skipframes is the number of frames to omit during tracing, counting from
// runtime_SemacquireMutex's caller.
// The different forms of this function just tell the runtime how to present
// the reason for waiting in a backtrace, and is used to compute some metrics.
// Otherwise they're functionally identical.
func runtime_SemacquireRWMutexR(s *uint32, lifo bool, skipframes int)
func runtime_SemacquireRWMutex(s *uint32, lifo bool, skipframes int)

// Semrelease atomically increments *s and notifies a waiting goroutine
// if one is blocked in Semacquire.
// It is intended as a simple wakeup primitive for use by the synchronization
// library and should not be used directly.
// If handoff is true, pass count directly to the first waiter.
// skipframes is the number of frames to omit during tracing, counting from
// runtime_Semrelease's caller.
func runtime_Semrelease(s *uint32, handoff bool, skipframes int)

// See runtime/sema.go for documentation.
func runtime_notifyListAdd(l *notifyList) uint32

// See runtime/sema.go for documentation.
func runtime_notifyListWait(l *notifyList, t uint32)

// See runtime/sema.go for documentation.
func runtime_notifyListNotifyAll(l *notifyList)

// See runtime/sema.go for documentation.
func runtime_notifyListNotifyOne(l *notifyList)

// Ensure that sync and runtime agree on size of notifyList.
func runtime_notifyListCheck(size uintptr)
func init() {
	var n notifyList
	runtime_notifyListCheck(unsafe.Sizeof(n))
}

func throw(string)
func fatal(string)

```

// === FILE: references/go/src/sync/runtime2.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !goexperiment.staticlockranking

package sync

import "unsafe"

// Approximation of notifyList in runtime/sema.go. Size and alignment must
// agree.
type notifyList struct {
	wait   uint32
	notify uint32
	lock   uintptr // key field of the mutex
	head   unsafe.Pointer
	tail   unsafe.Pointer
}

```

// === FILE: references/go/src/sync/runtime2_lockrank.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.staticlockranking

package sync

import "unsafe"

// Approximation of notifyList in runtime/sema.go. Size and alignment must
// agree.
type notifyList struct {
	wait   uint32
	notify uint32
	rank   int64   // rank field of the mutex
	lock   uintptr // key field of the mutex

	head unsafe.Pointer
	tail unsafe.Pointer
}

```

// === FILE: references/go/src/sync/rwmutex.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"internal/race"
	"sync/atomic"
	"unsafe"
)

// There is a modified copy of this file in runtime/rwmutex.go.
// If you make any changes here, see if you should make them there.

// A RWMutex is a reader/writer mutual exclusion lock.
// The lock can be held by an arbitrary number of readers or a single writer.
// The zero value for a RWMutex is an unlocked mutex.
//
// A RWMutex must not be copied after first use.
//
// If any goroutine calls [RWMutex.Lock] while the lock is already held by
// one or more readers, concurrent calls to [RWMutex.RLock] will block until
// the writer has acquired (and released) the lock, to ensure that
// the lock eventually becomes available to the writer.
// Note that this prohibits recursive read-locking.
// A [RWMutex.RLock] cannot be upgraded into a [RWMutex.Lock],
// nor can a [RWMutex.Lock] be downgraded into a [RWMutex.RLock].
//
// In the terminology of [the Go memory model],
// the n'th call to [RWMutex.Unlock] “synchronizes before” the m'th call to Lock
// for any n < m, just as for [Mutex].
// For any call to RLock, there exists an n such that
// the n'th call to Unlock “synchronizes before” that call to RLock,
// and the corresponding call to [RWMutex.RUnlock] “synchronizes before”
// the n+1'th call to Lock.
//
// [the Go memory model]: https://go.dev/ref/mem
type RWMutex struct {
	w           Mutex        // held if there are pending writers
	writerSem   uint32       // semaphore for writers to wait for completing readers
	readerSem   uint32       // semaphore for readers to wait for completing writers
	readerCount atomic.Int32 // number of pending readers
	readerWait  atomic.Int32 // number of departing readers
}

const rwmutexMaxReaders = 1 << 30

// Happens-before relationships are indicated to the race detector via:
// - Unlock  -> Lock:  readerSem
// - Unlock  -> RLock: readerSem
// - RUnlock -> Lock:  writerSem
//
// The methods below temporarily disable handling of race synchronization
// events in order to provide the more precise model above to the race
// detector.
//
// For example, atomic.AddInt32 in RLock should not appear to provide
// acquire-release semantics, which would incorrectly synchronize racing
// readers, thus potentially missing races.

// RLock locks rw for reading.
//
// It should not be used for recursive read locking; a blocked Lock
// call excludes new readers from acquiring the lock. See the
// documentation on the [RWMutex] type.
func (rw *RWMutex) RLock() {
	if race.Enabled {
		race.Read(unsafe.Pointer(&rw.w))
		race.Disable()
	}
	if rw.readerCount.Add(1) < 0 {
		// A writer is pending, wait for it.
		runtime_SemacquireRWMutexR(&rw.readerSem, false, 0)
	}
	if race.Enabled {
		race.Enable()
		race.Acquire(unsafe.Pointer(&rw.readerSem))
	}
}

// TryRLock tries to lock rw for reading and reports whether it succeeded.
//
// Note that while correct uses of TryRLock do exist, they are rare,
// and use of TryRLock is often a sign of a deeper problem
// in a particular use of mutexes.
func (rw *RWMutex) TryRLock() bool {
	if race.Enabled {
		race.Read(unsafe.Pointer(&rw.w))
		race.Disable()
	}
	for {
		c := rw.readerCount.Load()
		if c < 0 {
			if race.Enabled {
				race.Enable()
			}
			return false
		}
		if rw.readerCount.CompareAndSwap(c, c+1) {
			if race.Enabled {
				race.Enable()
				race.Acquire(unsafe.Pointer(&rw.readerSem))
			}
			return true
		}
	}
}

// RUnlock undoes a single [RWMutex.RLock] call;
// it does not affect other simultaneous readers.
// It is a run-time error if rw is not locked for reading
// on entry to RUnlock.
func (rw *RWMutex) RUnlock() {
	if race.Enabled {
		race.Read(unsafe.Pointer(&rw.w))
		race.ReleaseMerge(unsafe.Pointer(&rw.writerSem))
		race.Disable()
	}
	if r := rw.readerCount.Add(-1); r < 0 {
		// Outlined slow-path to allow the fast-path to be inlined
		rw.rUnlockSlow(r)
	}
	if race.Enabled {
		race.Enable()
	}
}

func (rw *RWMutex) rUnlockSlow(r int32) {
	if r+1 == 0 || r+1 == -rwmutexMaxReaders {
		race.Enable()
		fatal("sync: RUnlock of unlocked RWMutex")
	}
	// A writer is pending.
	if rw.readerWait.Add(-1) == 0 {
		// The last reader unblocks the writer.
		runtime_Semrelease(&rw.writerSem, false, 1)
	}
}

// Lock locks rw for writing.
// If the lock is already locked for reading or writing,
// Lock blocks until the lock is available.
func (rw *RWMutex) Lock() {
	if race.Enabled {
		race.Read(unsafe.Pointer(&rw.w))
		race.Disable()
	}
	// First, resolve competition with other writers.
	rw.w.Lock()
	// Announce to readers there is a pending writer.
	r := rw.readerCount.Add(-rwmutexMaxReaders) + rwmutexMaxReaders
	// Wait for active readers.
	if r != 0 && rw.readerWait.Add(r) != 0 {
		runtime_SemacquireRWMutex(&rw.writerSem, false, 0)
	}
	if race.Enabled {
		race.Enable()
		race.Acquire(unsafe.Pointer(&rw.readerSem))
		race.Acquire(unsafe.Pointer(&rw.writerSem))
	}
}

// TryLock tries to lock rw for writing and reports whether it succeeded.
//
// Note that while correct uses of TryLock do exist, they are rare,
// and use of TryLock is often a sign of a deeper problem
// in a particular use of mutexes.
func (rw *RWMutex) TryLock() bool {
	if race.Enabled {
		race.Read(unsafe.Pointer(&rw.w))
		race.Disable()
	}
	if !rw.w.TryLock() {
		if race.Enabled {
			race.Enable()
		}
		return false
	}
	if !rw.readerCount.CompareAndSwap(0, -rwmutexMaxReaders) {
		rw.w.Unlock()
		if race.Enabled {
			race.Enable()
		}
		return false
	}
	if race.Enabled {
		race.Enable()
		race.Acquire(unsafe.Pointer(&rw.readerSem))
		race.Acquire(unsafe.Pointer(&rw.writerSem))
	}
	return true
}

// Unlock unlocks rw for writing. It is a run-time error if rw is
// not locked for writing on entry to Unlock.
//
// As with Mutexes, a locked [RWMutex] is not associated with a particular
// goroutine. One goroutine may [RWMutex.RLock] ([RWMutex.Lock]) a RWMutex and then
// arrange for another goroutine to [RWMutex.RUnlock] ([RWMutex.Unlock]) it.
func (rw *RWMutex) Unlock() {
	if race.Enabled {
		race.Read(unsafe.Pointer(&rw.w))
		race.Release(unsafe.Pointer(&rw.readerSem))
		race.Disable()
	}

	// Announce to readers there is no active writer.
	r := rw.readerCount.Add(rwmutexMaxReaders)
	if r >= rwmutexMaxReaders {
		race.Enable()
		fatal("sync: Unlock of unlocked RWMutex")
	}
	// Unblock blocked readers, if any.
	for i := 0; i < int(r); i++ {
		runtime_Semrelease(&rw.readerSem, false, 0)
	}
	// Allow other writers to proceed.
	rw.w.Unlock()
	if race.Enabled {
		race.Enable()
	}
}

// syscall_hasWaitingReaders reports whether any goroutine is waiting
// to acquire a read lock on rw. This exists because syscall.ForkLock
// is an RWMutex, and we can't change that without breaking compatibility.
// We don't need or want RWMutex semantics for ForkLock, and we use
// this private API to avoid having to change the type of ForkLock.
// For more details see the syscall package.
//
//go:linkname syscall_hasWaitingReaders syscall.hasWaitingReaders
func syscall_hasWaitingReaders(rw *RWMutex) bool {
	r := rw.readerCount.Load()
	return r < 0 && r+rwmutexMaxReaders > 0
}

// RLocker returns a [Locker] interface that implements
// the [Locker.Lock] and [Locker.Unlock] methods by calling rw.RLock and rw.RUnlock.
func (rw *RWMutex) RLocker() Locker {
	return (*rlocker)(rw)
}

type rlocker RWMutex

func (r *rlocker) Lock()   { (*RWMutex)(r).RLock() }
func (r *rlocker) Unlock() { (*RWMutex)(r).RUnlock() }

```

// === FILE: references/go/src/sync/waitgroup.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"internal/race"
	"internal/synctest"
	"sync/atomic"
	"unsafe"
)

// A WaitGroup is a counting semaphore typically used to wait
// for a group of goroutines or tasks to finish.
//
// Typically, a main goroutine will start tasks, each in a new
// goroutine, by calling [WaitGroup.Go] and then wait for all tasks to
// complete by calling [WaitGroup.Wait]. For example:
//
//	var wg sync.WaitGroup
//	wg.Go(task1)
//	wg.Go(task2)
//	wg.Wait()
//
// A WaitGroup may also be used for tracking tasks without using Go to
// start new goroutines by using [WaitGroup.Add] and [WaitGroup.Done].
//
// The previous example can be rewritten using explicitly created
// goroutines along with Add and Done:
//
//	var wg sync.WaitGroup
//	wg.Add(1)
//	go func() {
//		defer wg.Done()
//		task1()
//	}()
//	wg.Add(1)
//	go func() {
//		defer wg.Done()
//		task2()
//	}()
//	wg.Wait()
//
// This pattern is common in code that predates [WaitGroup.Go].
//
// A WaitGroup must not be copied after first use.
type WaitGroup struct {
	noCopy noCopy

	// Bits (high to low):
	//   bits[0:32]  counter
	//   bits[32]    flag: synctest bubble membership
	//   bits[33:64] wait count
	state atomic.Uint64
	sema  uint32
}

// waitGroupBubbleFlag indicates that a WaitGroup is associated with a synctest bubble.
const waitGroupBubbleFlag = 0x8000_0000

// Add adds delta, which may be negative, to the [WaitGroup] task counter.
// If the counter becomes zero, all goroutines blocked on [WaitGroup.Wait] are released.
// If the counter goes negative, Add panics.
//
// Callers should prefer [WaitGroup.Go].
//
// Note that calls with a positive delta that occur when the counter is zero
// must happen before a Wait. Calls with a negative delta, or calls with a
// positive delta that start when the counter is greater than zero, may happen
// at any time.
// Typically this means the calls to Add should execute before the statement
// creating the goroutine or other event to be waited for.
// If a WaitGroup is reused to wait for several independent sets of events,
// new Add calls must happen after all previous Wait calls have returned.
// See the WaitGroup example.
func (wg *WaitGroup) Add(delta int) {
	if race.Enabled {
		if delta < 0 {
			// Synchronize decrements with Wait.
			race.ReleaseMerge(unsafe.Pointer(wg))
		}
		race.Disable()
		defer race.Enable()
	}
	bubbled := false
	if synctest.IsInBubble() {
		// If Add is called from within a bubble, then all Add calls must be made
		// from the same bubble.
		switch synctest.Associate(wg) {
		case synctest.Unbubbled:
		case synctest.OtherBubble:
			// wg is already associated with a different bubble.
			fatal("sync: WaitGroup.Add called from multiple synctest bubbles")
		case synctest.CurrentBubble:
			bubbled = true
			state := wg.state.Or(waitGroupBubbleFlag)
			if state != 0 && state&waitGroupBubbleFlag == 0 {
				// Add has been called from outside this bubble.
				fatal("sync: WaitGroup.Add called from inside and outside synctest bubble")
			}
		}
	}
	state := wg.state.Add(uint64(delta) << 32)
	if state&waitGroupBubbleFlag != 0 && !bubbled {
		// Add has been called from within a synctest bubble (and we aren't in one).
		fatal("sync: WaitGroup.Add called from inside and outside synctest bubble")
	}
	v := int32(state >> 32)
	w := uint32(state & 0x7fffffff)
	if race.Enabled && delta > 0 && v == int32(delta) {
		// The first increment must be synchronized with Wait.
		// Need to model this as a read, because there can be
		// several concurrent wg.counter transitions from 0.
		race.Read(unsafe.Pointer(&wg.sema))
	}
	if v < 0 {
		panic("sync: negative WaitGroup counter")
	}
	if w != 0 && delta > 0 && v == int32(delta) {
		panic("sync: WaitGroup misuse: Add called concurrently with Wait")
	}
	if v > 0 || w == 0 {
		return
	}
	// This goroutine has set counter to 0 when waiters > 0.
	// Now there can't be concurrent mutations of state:
	// - Adds must not happen concurrently with Wait,
	// - Wait does not increment waiters if it sees counter == 0.
	// Still do a cheap sanity check to detect WaitGroup misuse.
	if wg.state.Load() != state {
		panic("sync: WaitGroup misuse: Add called concurrently with Wait")
	}
	// Reset waiters count to 0.
	wg.state.Store(0)
	if bubbled {
		// Adds must not happen concurrently with wait when counter is 0,
		// so we can safely disassociate wg from its current bubble.
		synctest.Disassociate(wg)
	}
	for ; w != 0; w-- {
		runtime_Semrelease(&wg.sema, false, 0)
	}
}

// Done decrements the [WaitGroup] task counter by one.
// It is equivalent to Add(-1).
//
// Callers should prefer [WaitGroup.Go].
//
// In the terminology of [the Go memory model], a call to Done
// "synchronizes before" the return of any Wait call that it unblocks.
//
// [the Go memory model]: https://go.dev/ref/mem
func (wg *WaitGroup) Done() {
	wg.Add(-1)
}

// Wait blocks until the [WaitGroup] task counter is zero.
func (wg *WaitGroup) Wait() {
	if race.Enabled {
		race.Disable()
	}
	for {
		state := wg.state.Load()
		v := int32(state >> 32)
		w := uint32(state & 0x7fffffff)
		if v == 0 {
			// Counter is 0, no need to wait.
			if race.Enabled {
				race.Enable()
				race.Acquire(unsafe.Pointer(wg))
			}
			if w == 0 && state&waitGroupBubbleFlag != 0 && synctest.IsAssociated(wg) {
				// Adds must not happen concurrently with wait when counter is 0,
				// so we can disassociate wg from its current bubble.
				if wg.state.CompareAndSwap(state, 0) {
					synctest.Disassociate(wg)
				}
			}
			return
		}
		// Increment waiters count.
		if wg.state.CompareAndSwap(state, state+1) {
			if race.Enabled && w == 0 {
				// Wait must be synchronized with the first Add.
				// Need to model this is as a write to race with the read in Add.
				// As a consequence, can do the write only for the first waiter,
				// otherwise concurrent Waits will race with each other.
				race.Write(unsafe.Pointer(&wg.sema))
			}
			synctestDurable := false
			if state&waitGroupBubbleFlag != 0 && synctest.IsInBubble() {
				if race.Enabled {
					race.Enable()
				}
				if synctest.IsAssociated(wg) {
					// Add was called within the current bubble,
					// so this Wait is durably blocking.
					synctestDurable = true
				}
				if race.Enabled {
					race.Disable()
				}
			}
			runtime_SemacquireWaitGroup(&wg.sema, synctestDurable)
			isReset := wg.state.Load() != 0
			if race.Enabled {
				race.Enable()
				race.Acquire(unsafe.Pointer(wg))
			}
			if isReset {
				panic("sync: WaitGroup is reused before previous Wait has returned")
			}
			return
		}
	}
}

// Go calls f in a new goroutine and adds that task to the [WaitGroup].
// When f returns, the task is removed from the WaitGroup.
//
// The function f must not panic.
//
// If the WaitGroup is empty, Go must happen before a [WaitGroup.Wait].
// Typically, this simply means Go is called to start tasks before Wait is called.
// If the WaitGroup is not empty, Go may happen at any time.
// This means a goroutine started by Go may itself call Go.
// If a WaitGroup is reused to wait for several independent sets of tasks,
// new Go calls must happen after all previous Wait calls have returned.
//
// In the terminology of [the Go memory model], the return from f
// "synchronizes before" the return of any Wait call that it unblocks.
//
// [the Go memory model]: https://go.dev/ref/mem
func (wg *WaitGroup) Go(f func()) {
	wg.Add(1)
	go func() {
		defer func() {
			if x := recover(); x != nil {
				// f panicked, which will be fatal because
				// this is a new goroutine.
				//
				// Calling Done will unblock Wait in the main goroutine,
				// allowing it to race with the fatal panic and
				// possibly even exit the process (os.Exit(0))
				// before the panic completes.
				//
				// This is almost certainly undesirable,
				// so instead avoid calling Done and simply panic.
				panic(x)
			}

			// f completed normally, or abruptly using goexit.
			// Either way, decrement the semaphore.
			wg.Done()
		}()
		f()
	}()
}

```

