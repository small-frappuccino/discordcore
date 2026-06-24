# Domain Architecture: crypto/mlkem

## Layout Topology
```text
crypto/mlkem/
├── mlkemtest
│   └── mlkemtest.go
└── mlkem.go
```

## Source Stream Aggregation

// === FILE: references/go/src/crypto/mlkem/mlkem.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package mlkem implements the quantum-resistant key encapsulation method
// ML-KEM (formerly known as Kyber), as specified in [NIST FIPS 203].
//
// Most applications should use the ML-KEM-768 parameter set, as implemented by
// [DecapsulationKey768] and [EncapsulationKey768].
//
// [NIST FIPS 203]: https://doi.org/10.6028/NIST.FIPS.203
package mlkem

import (
	"crypto"
	"crypto/internal/fips140/mlkem"
)

const (
	// SharedKeySize is the size of a shared key produced by ML-KEM.
	SharedKeySize = 32

	// SeedSize is the size of a seed used to generate a decapsulation key.
	SeedSize = 64

	// CiphertextSize768 is the size of a ciphertext produced by ML-KEM-768.
	CiphertextSize768 = 1088

	// EncapsulationKeySize768 is the size of an ML-KEM-768 encapsulation key.
	EncapsulationKeySize768 = 1184

	// CiphertextSize1024 is the size of a ciphertext produced by ML-KEM-1024.
	CiphertextSize1024 = 1568

	// EncapsulationKeySize1024 is the size of an ML-KEM-1024 encapsulation key.
	EncapsulationKeySize1024 = 1568
)

// DecapsulationKey768 is the secret key used to decapsulate a shared key
// from a ciphertext. It includes various precomputed values.
type DecapsulationKey768 struct {
	key *mlkem.DecapsulationKey768
}

// GenerateKey768 generates a new decapsulation key, drawing random bytes from
// a secure source. The decapsulation key must be kept secret.
func GenerateKey768() (*DecapsulationKey768, error) {
	key, err := mlkem.GenerateKey768()
	if err != nil {
		return nil, err
	}

	return &DecapsulationKey768{key}, nil
}

// NewDecapsulationKey768 expands a decapsulation key from a 64-byte seed in the
// "d || z" form. The seed must be uniformly random.
func NewDecapsulationKey768(seed []byte) (*DecapsulationKey768, error) {
	key, err := mlkem.NewDecapsulationKey768(seed)
	if err != nil {
		return nil, err
	}

	return &DecapsulationKey768{key}, nil
}

// Bytes returns the decapsulation key as a 64-byte seed in the "d || z" form.
//
// The decapsulation key must be kept secret.
func (dk *DecapsulationKey768) Bytes() []byte {
	return dk.key.Bytes()
}

// Decapsulate generates a shared key from a ciphertext and a decapsulation
// key. If the ciphertext is not the correct length, Decapsulate returns an
// error. A ciphertext that is the correct length but otherwise invalid will
// not return an error; instead, it will produce a shared key that does not
// match the sender's, according to FIPS 203.
//
// The shared key must be kept secret.
func (dk *DecapsulationKey768) Decapsulate(ciphertext []byte) (sharedKey []byte, err error) {
	return dk.key.Decapsulate(ciphertext)
}

// EncapsulationKey returns the public encapsulation key necessary to produce
// ciphertexts.
func (dk *DecapsulationKey768) EncapsulationKey() *EncapsulationKey768 {
	return &EncapsulationKey768{dk.key.EncapsulationKey()}
}

// Encapsulator returns the encapsulation key, like
// [DecapsulationKey768.EncapsulationKey].
//
// It implements [crypto.Decapsulator].
func (dk *DecapsulationKey768) Encapsulator() crypto.Encapsulator {
	return dk.EncapsulationKey()
}

var _ crypto.Decapsulator = (*DecapsulationKey768)(nil)

// An EncapsulationKey768 is the public key used to produce ciphertexts to be
// decapsulated by the corresponding DecapsulationKey768.
type EncapsulationKey768 struct {
	key *mlkem.EncapsulationKey768
}

// NewEncapsulationKey768 parses an encapsulation key from its encoded form. If
// the encapsulation key is not valid, NewEncapsulationKey768 returns an error.
func NewEncapsulationKey768(encapsulationKey []byte) (*EncapsulationKey768, error) {
	key, err := mlkem.NewEncapsulationKey768(encapsulationKey)
	if err != nil {
		return nil, err
	}

	return &EncapsulationKey768{key}, nil
}

// Bytes returns the encapsulation key as a byte slice.
func (ek *EncapsulationKey768) Bytes() []byte {
	return ek.key.Bytes()
}

// Encapsulate generates a shared key and an associated ciphertext from an
// encapsulation key, drawing random bytes from a secure source.
//
// The shared key must be kept secret.
//
// For testing, derandomized encapsulation is provided by the
// [crypto/mlkem/mlkemtest] package.
func (ek *EncapsulationKey768) Encapsulate() (sharedKey, ciphertext []byte) {
	return ek.key.Encapsulate()
}

// DecapsulationKey1024 is the secret key used to decapsulate a shared key
// from a ciphertext. It includes various precomputed values.
type DecapsulationKey1024 struct {
	key *mlkem.DecapsulationKey1024
}

// GenerateKey1024 generates a new decapsulation key, drawing random bytes from
// a secure source. The decapsulation key must be kept secret.
func GenerateKey1024() (*DecapsulationKey1024, error) {
	key, err := mlkem.GenerateKey1024()
	if err != nil {
		return nil, err
	}

	return &DecapsulationKey1024{key}, nil
}

// NewDecapsulationKey1024 expands a decapsulation key from a 64-byte seed in the
// "d || z" form. The seed must be uniformly random.
func NewDecapsulationKey1024(seed []byte) (*DecapsulationKey1024, error) {
	key, err := mlkem.NewDecapsulationKey1024(seed)
	if err != nil {
		return nil, err
	}

	return &DecapsulationKey1024{key}, nil
}

// Bytes returns the decapsulation key as a 64-byte seed in the "d || z" form.
//
// The decapsulation key must be kept secret.
func (dk *DecapsulationKey1024) Bytes() []byte {
	return dk.key.Bytes()
}

// Decapsulate generates a shared key from a ciphertext and a decapsulation
// key. If the ciphertext is not the correct length, Decapsulate returns an
// error. A ciphertext that is the correct length but otherwise invalid will
// not return an error; instead, it will produce a shared key that does not
// match the sender's, according to FIPS 203.
//
// The shared key must be kept secret.
func (dk *DecapsulationKey1024) Decapsulate(ciphertext []byte) (sharedKey []byte, err error) {
	return dk.key.Decapsulate(ciphertext)
}

// EncapsulationKey returns the public encapsulation key necessary to produce
// ciphertexts.
func (dk *DecapsulationKey1024) EncapsulationKey() *EncapsulationKey1024 {
	return &EncapsulationKey1024{dk.key.EncapsulationKey()}
}

// Encapsulator returns the encapsulation key, like
// [DecapsulationKey1024.EncapsulationKey].
//
// It implements [crypto.Decapsulator].
func (dk *DecapsulationKey1024) Encapsulator() crypto.Encapsulator {
	return dk.EncapsulationKey()
}

var _ crypto.Decapsulator = (*DecapsulationKey1024)(nil)

// An EncapsulationKey1024 is the public key used to produce ciphertexts to be
// decapsulated by the corresponding DecapsulationKey1024.
type EncapsulationKey1024 struct {
	key *mlkem.EncapsulationKey1024
}

// NewEncapsulationKey1024 parses an encapsulation key from its encoded form. If
// the encapsulation key is not valid, NewEncapsulationKey1024 returns an error.
func NewEncapsulationKey1024(encapsulationKey []byte) (*EncapsulationKey1024, error) {
	key, err := mlkem.NewEncapsulationKey1024(encapsulationKey)
	if err != nil {
		return nil, err
	}

	return &EncapsulationKey1024{key}, nil
}

// Bytes returns the encapsulation key as a byte slice.
func (ek *EncapsulationKey1024) Bytes() []byte {
	return ek.key.Bytes()
}

// Encapsulate generates a shared key and an associated ciphertext from an
// encapsulation key, drawing random bytes from a secure source.
//
// The shared key must be kept secret.
//
// For testing, derandomized encapsulation is provided by the
// [crypto/mlkem/mlkemtest] package.
func (ek *EncapsulationKey1024) Encapsulate() (sharedKey, ciphertext []byte) {
	return ek.key.Encapsulate()
}

```

// === FILE: references/go/src/crypto/mlkem/mlkemtest/mlkemtest.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package mlkemtest provides testing functions for the ML-KEM algorithm.
package mlkemtest

import (
	fips140mlkem "crypto/internal/fips140/mlkem"
	"crypto/internal/fips140only"
	"crypto/mlkem"
	"errors"
)

// Encapsulate768 implements derandomized ML-KEM-768 encapsulation
// (ML-KEM.Encaps_internal from FIPS 203) using the provided encapsulation key
// ek and 32 bytes of randomness.
//
// It must only be used for known-answer tests.
func Encapsulate768(ek *mlkem.EncapsulationKey768, random []byte) (sharedKey, ciphertext []byte, err error) {
	if len(random) != 32 {
		return nil, nil, errors.New("mlkemtest: Encapsulate768: random must be 32 bytes")
	}
	if fips140only.Enforced() {
		return nil, nil, errors.New("crypto/mlkem/mlkemtest: use of derandomized encapsulation is not allowed in FIPS 140-only mode")
	}
	k, err := fips140mlkem.NewEncapsulationKey768(ek.Bytes())
	if err != nil {
		return nil, nil, errors.New("mlkemtest: Encapsulate768: failed to reconstruct key: " + err.Error())
	}
	sharedKey, ciphertext = k.EncapsulateInternal((*[32]byte)(random))
	return sharedKey, ciphertext, nil
}

// Encapsulate1024 implements derandomized ML-KEM-1024 encapsulation
// (ML-KEM.Encaps_internal from FIPS 203) using the provided encapsulation key
// ek and 32 bytes of randomness.
//
// It must only be used for known-answer tests.
func Encapsulate1024(ek *mlkem.EncapsulationKey1024, random []byte) (sharedKey, ciphertext []byte, err error) {
	if len(random) != 32 {
		return nil, nil, errors.New("mlkemtest: Encapsulate1024: random must be 32 bytes")
	}
	if fips140only.Enforced() {
		return nil, nil, errors.New("crypto/mlkem/mlkemtest: use of derandomized encapsulation is not allowed in FIPS 140-only mode")
	}
	k, err := fips140mlkem.NewEncapsulationKey1024(ek.Bytes())
	if err != nil {
		return nil, nil, errors.New("mlkemtest: Encapsulate1024: failed to reconstruct key: " + err.Error())
	}
	sharedKey, ciphertext = k.EncapsulateInternal((*[32]byte)(random))
	return sharedKey, ciphertext, nil
}

```

