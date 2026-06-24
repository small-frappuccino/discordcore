# Domain Architecture: crypto/subtle

## Layout Topology
```text
crypto/subtle/
├── constant_time.go
├── dit.go
└── xor.go
```

## Source Stream Aggregation

// === FILE: references/go/src/crypto/subtle/constant_time.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package subtle implements functions that are often useful in cryptographic
// code but require careful thought to use correctly.
package subtle

import (
	"crypto/internal/constanttime"
	"crypto/internal/fips140/subtle"
)

// These functions are forwarded to crypto/internal/constanttime for intrinsified
// operations, and to crypto/internal/fips140/subtle for byte slice operations.

// ConstantTimeCompare returns 1 if the two slices, x and y, have equal contents
// and 0 otherwise. The time taken is a function of the length of the slices and
// is independent of the contents. If the lengths of x and y do not match it
// returns 0 immediately.
func ConstantTimeCompare(x, y []byte) int {
	return subtle.ConstantTimeCompare(x, y)
}

// ConstantTimeSelect returns x if v == 1 and y if v == 0.
// Its behavior is undefined if v takes any other value.
func ConstantTimeSelect(v, x, y int) int {
	return constanttime.Select(v, x, y)
}

// ConstantTimeByteEq returns 1 if x == y and 0 otherwise.
func ConstantTimeByteEq(x, y uint8) int {
	return constanttime.ByteEq(x, y)
}

// ConstantTimeEq returns 1 if x == y and 0 otherwise.
func ConstantTimeEq(x, y int32) int {
	return constanttime.Eq(x, y)
}

// ConstantTimeCopy copies the contents of y into x (a slice of equal length)
// if v == 1. If v == 0, x is left unchanged. Its behavior is undefined if v
// takes any other value.
func ConstantTimeCopy(v int, x, y []byte) {
	subtle.ConstantTimeCopy(v, x, y)
}

// ConstantTimeLessOrEq returns 1 if x <= y and 0 otherwise.
// Its behavior is undefined if x or y are negative or > 2**31 - 1.
func ConstantTimeLessOrEq(x, y int) int {
	return constanttime.LessOrEq(x, y)
}

```

// === FILE: references/go/src/crypto/subtle/dit.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package subtle

import (
	"internal/runtime/sys"
	_ "unsafe"
)

// WithDataIndependentTiming enables architecture specific features which ensure
// that the timing of specific instructions is independent of their inputs
// before executing f. On f returning it disables these features.
//
// Any goroutine spawned by f will also have data independent timing enabled for
// its lifetime, as well as any of their descendant goroutines.
//
// Any C code called via cgo from within f, or from a goroutine spawned by f, will
// also have data independent timing enabled for the duration of the call. If the
// C code disables data independent timing, it will be re-enabled on return to Go.
//
// If C code called via cgo, from f or elsewhere, enables or disables data
// independent timing then calling into Go will preserve that state for the
// duration of the call.
//
// WithDataIndependentTiming should only be used when f is written to make use
// of constant-time operations. WithDataIndependentTiming does not make
// variable-time code constant-time.
//
// Calls to WithDataIndependentTiming may be nested.
//
// On Arm64 processors with FEAT_DIT, WithDataIndependentTiming enables
// PSTATE.DIT. See https://developer.arm.com/documentation/ka005181/1-0/?lang=en.
//
// Currently, on all other architectures WithDataIndependentTiming executes f immediately
// with no other side-effects.
//
//go:noinline
func WithDataIndependentTiming(f func()) {
	if !sys.DITSupported {
		f()
		return
	}

	alreadyEnabled := setDITEnabled()

	// disableDIT is called in a deferred function so that if f panics we will
	// still disable DIT, in case the panic is recovered further up the stack.
	defer func() {
		if !alreadyEnabled {
			setDITDisabled()
		}
	}()

	f()
}

//go:linkname setDITEnabled
func setDITEnabled() bool

//go:linkname setDITDisabled
func setDITDisabled()

```

// === FILE: references/go/src/crypto/subtle/xor.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package subtle

import "crypto/internal/fips140/subtle"

// XORBytes sets dst[i] = x[i] ^ y[i] for all i < n = min(len(x), len(y)),
// returning n, the number of bytes written to dst.
//
// If dst does not have length at least n,
// XORBytes panics without writing anything to dst.
//
// dst and x or y may overlap exactly or not at all,
// otherwise XORBytes may panic.
func XORBytes(dst, x, y []byte) int {
	return subtle.XORBytes(dst, x, y)
}

```

