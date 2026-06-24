# Domain Architecture: crypto/rand

## Layout Topology
```text
crypto/rand/
├── rand.go
├── text.go
└── util.go
```

## Source Stream Aggregation

// === FILE: references/go/src/crypto/rand/rand.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package rand implements a cryptographically secure
// random number generator.
package rand

import (
	"crypto/internal/boring"
	"crypto/internal/fips140/drbg"
	"crypto/internal/rand"
	"io"
	_ "unsafe"

	// Ensure the go:linkname from testing/cryptotest to
	// crypto/internal/rand.SetTestingReader works.
	_ "crypto/internal/rand"
)

// Reader is a global, shared instance of a cryptographically
// secure random number generator. It is safe for concurrent use.
//
//   - On Linux, FreeBSD, Dragonfly, and Solaris, Reader uses getrandom(2).
//   - On legacy Linux (< 3.17), Reader opens /dev/urandom on first use.
//   - On macOS, iOS, and OpenBSD Reader, uses arc4random_buf(3).
//   - On NetBSD, Reader uses the kern.arandom sysctl.
//   - On Windows, Reader uses the ProcessPrng API.
//   - On js/wasm, Reader uses the Web Crypto API.
//   - On wasip1/wasm, Reader uses random_get.
//
// In FIPS 140-3 mode, the output passes through an SP 800-90A Rev. 1
// Deterministric Random Bit Generator (DRBG).
var Reader io.Reader = rand.Reader

// fatal is [runtime.fatal], pushed via linkname.
//
//go:linkname fatal
func fatal(string)

// Read fills b with cryptographically secure random bytes. It never returns an
// error, and always fills b entirely.
//
// Read calls [io.ReadFull] on [Reader] and crashes the program irrecoverably if
// an error is returned. The default Reader uses operating system APIs that are
// documented to never return an error on all but legacy Linux systems.
func Read(b []byte) (n int, err error) {
	// We don't want b to escape to the heap, but escape analysis can't see
	// through a potentially overridden Reader, so we special-case the default
	// case which we can keep non-escaping, and in the general case we read into
	// a heap buffer and copy from it.
	if rand.IsDefaultReader(Reader) {
		if boring.Enabled {
			_, err = io.ReadFull(boring.RandReader, b)
		} else {
			drbg.Read(b)
		}
	} else {
		bb := make([]byte, len(b))
		_, err = io.ReadFull(Reader, bb)
		copy(b, bb)
	}
	if err != nil {
		fatal("crypto/rand: failed to read random data (see https://go.dev/issue/66821): " + err.Error())
		panic("unreachable") // To be sure.
	}
	return len(b), nil
}

```

// === FILE: references/go/src/crypto/rand/text.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rand

const base32alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"

// Text returns a cryptographically random string using the standard RFC 4648 base32 alphabet
// for use when a secret string, token, password, or other text is needed.
// The result contains at least 128 bits of randomness, enough to prevent brute force
// guessing attacks and to make the likelihood of collisions vanishingly small.
// A future version may return longer texts as needed to maintain those properties.
func Text() string {
	// ⌈log₃₂ 2¹²⁸⌉ = 26 chars
	src := make([]byte, 26)
	Read(src)
	for i := range src {
		src[i] = base32alphabet[src[i]%32]
	}
	return string(src)
}

```

// === FILE: references/go/src/crypto/rand/util.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rand

import (
	"crypto/internal/fips140only"
	"crypto/internal/rand"
	"errors"
	"io"
	"math/big"
)

// Prime returns a number of the given bit length that is prime with high probability.
// Prime will return error for any error returned by rand.Read or if bits < 2.
//
// Since Go 1.26, a secure source of random bytes is always used, and the Reader is
// ignored unless GODEBUG=cryptocustomrand=1 is set. This setting will be removed
// in a future Go release. Instead, use [testing/cryptotest.SetGlobalRandom].
func Prime(r io.Reader, bits int) (*big.Int, error) {
	if fips140only.Enforced() {
		return nil, errors.New("crypto/rand: use of Prime is not allowed in FIPS 140-only mode")
	}
	if bits < 2 {
		return nil, errors.New("crypto/rand: prime size must be at least 2-bit")
	}

	r = rand.CustomReader(r)

	b := uint(bits % 8)
	if b == 0 {
		b = 8
	}

	bytes := make([]byte, (bits+7)/8)
	p := new(big.Int)

	for {
		if _, err := io.ReadFull(r, bytes); err != nil {
			return nil, err
		}

		// Clear bits in the first byte to make sure the candidate has a size <= bits.
		bytes[0] &= uint8(int(1<<b) - 1)
		// Don't let the value be too small, i.e, set the most significant two bits.
		// Setting the top two bits, rather than just the top bit,
		// means that when two of these values are multiplied together,
		// the result isn't ever one bit short.
		if b >= 2 {
			bytes[0] |= 3 << (b - 2)
		} else {
			// Here b==1, because b cannot be zero.
			bytes[0] |= 1
			if len(bytes) > 1 {
				bytes[1] |= 0x80
			}
		}
		// Make the value odd since an even number this large certainly isn't prime.
		bytes[len(bytes)-1] |= 1

		p.SetBytes(bytes)
		if p.ProbablyPrime(20) {
			return p, nil
		}
	}
}

// Int returns a uniform random value in [0, max). It panics if max <= 0, and
// returns an error if rand.Read returns one.
func Int(rand io.Reader, max *big.Int) (n *big.Int, err error) {
	if max.Sign() <= 0 {
		panic("crypto/rand: argument to Int is <= 0")
	}
	n = new(big.Int)
	n.Sub(max, n.SetUint64(1))
	// bitLen is the maximum bit length needed to encode a value < max.
	bitLen := n.BitLen()
	if bitLen == 0 {
		// the only valid result is 0
		return
	}
	// k is the maximum byte length needed to encode a value < max.
	k := (bitLen + 7) / 8
	// b is the number of bits in the most significant byte of max-1.
	b := uint(bitLen % 8)
	if b == 0 {
		b = 8
	}

	bytes := make([]byte, k)

	for {
		_, err = io.ReadFull(rand, bytes)
		if err != nil {
			return nil, err
		}

		// Clear bits in the first byte to increase the probability
		// that the candidate is < max.
		bytes[0] &= uint8(int(1<<b) - 1)

		n.SetBytes(bytes)
		if n.Cmp(max) < 0 {
			return
		}
	}
}

```

