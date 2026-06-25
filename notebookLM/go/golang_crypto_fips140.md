# Domain Architecture: crypto/fips140

## Layout Topology
```text
crypto/fips140/
├── enforcement.go
└── fips140.go
```

## Source Stream Aggregation

// === FILE: references!/go/src/crypto/fips140/enforcement.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fips140

import (
	"internal/godebug"
	_ "unsafe" // for linkname
)

// WithoutEnforcement disables strict FIPS 140-3 enforcement while executing f.
// Calling WithoutEnforcement without strict enforcement enabled
// (GODEBUG=fips140=only is not set or already inside of a call to
// WithoutEnforcement) is a no-op.
//
// WithoutEnforcement is inherited by any goroutines spawned while executing f.
//
// As this disables enforcement, it should be applied carefully to tightly
// scoped functions.
func WithoutEnforcement(f func()) {
	if !Enabled() || !Enforced() {
		f()
		return
	}
	setBypass()
	defer unsetBypass()
	f()
}

var enabled = godebug.New("fips140").Value() == "only"

// Enforced indicates if strict FIPS 140-3 enforcement is enabled. Strict
// enforcement is enabled when a program is run with GODEBUG=fips140=only and
// enforcement has not been disabled by a call to [WithoutEnforcement].
func Enforced() bool {
	return enabled && !isBypassed()
}

//go:linkname setBypass
func setBypass()

//go:linkname isBypassed
func isBypassed() bool

//go:linkname unsetBypass
func unsetBypass()

```

// === FILE: references!/go/src/crypto/fips140/fips140.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fips140 provides information about the FIPS 140-3 Go Cryptographic
// Module and FIPS 140-3 mode.
//
// For more details, see the [FIPS 140-3 documentation].
//
// [FIPS 140-3 documentation]: https://go.dev/doc/security/fips140
package fips140

import (
	"crypto/internal/fips140"
	"crypto/internal/fips140/check"
)

// Enabled reports whether the cryptography libraries are operating in FIPS
// 140-3 mode.
//
// It can be controlled at runtime using the GODEBUG setting "fips140". If set
// to "on", FIPS 140-3 mode is enabled. If set to "only", non-approved
// cryptography functions will additionally return errors or panic.
//
// This can't be changed after the program has started.
func Enabled() bool {
	if fips140.Enabled && !check.Verified {
		panic("crypto/fips140: FIPS 140-3 mode enabled, but integrity check didn't pass")
	}
	return fips140.Enabled
}

// Version returns the FIPS 140-3 Go Cryptographic Module version (such as
// "v1.0.0"), as referenced in the Security Policy for the module, if building
// against a frozen module with GOFIPS140. Otherwise, it returns "latest". If an
// alias is in use (such as "inprogress") the actual resolved version is
// returned.
//
// The returned version may not uniquely identify the frozen module which was
// used to build the program, if there are multiple copies of the frozen module
// at the same version. The uniquely identifying version suffix can be found by
// checking the value of the GOFIPS140 setting in
// runtime/debug.BuildInfo.Settings.
func Version() string {
	return fips140.Version()
}

```

