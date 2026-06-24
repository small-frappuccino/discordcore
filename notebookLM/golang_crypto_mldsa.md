# Domain Architecture: crypto/mldsa

## Layout Topology
```text
crypto/mldsa/
├── mldsa.go
├── mldsa_fips140v1.0.go
└── mldsa_fips140v1.26.go
```

## Source Stream Aggregation

// === FILE: references/go/src/crypto/mldsa/mldsa.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package mldsa implements the post-quantum ML-DSA signature scheme specified
// in [FIPS 204].
//
// This package is unavailable if using the [FIPS 140-3 Go Cryptographic Module]
// v1.0.0, in which case [GenerateKey], [NewPrivateKey], [NewPublicKey], and
// [Verify] will return an error. It is available if using v1.26.0 or later.
//
// [FIPS 204]: https://nvlpubs.nist.gov/nistpubs/FIPS/NIST.FIPS.204.pdf
// [FIPS 140-3 Go Cryptographic Module]: https://go.dev/doc/security/fips140
package mldsa

import "crypto"

const (
	PrivateKeySize = 32

	MLDSA44PublicKeySize = 1312
	MLDSA65PublicKeySize = 1952
	MLDSA87PublicKeySize = 2592

	MLDSA44SignatureSize = 2420
	MLDSA65SignatureSize = 3309
	MLDSA87SignatureSize = 4627
)

// Parameters represents one of the fixed parameter sets defined in FIPS 204.
//
// Most applications should use [MLDSA44].
//
// Multiple invocations of [MLDSA44], [MLDSA65], or [MLDSA87] will return the
// same respective value, which can be used for equality checks and switch
// statements. The returned value is safe for concurrent use.
type Parameters struct {
	name          string
	publicKeySize int
	signatureSize int
}

// MLDSA44 returns the ML-DSA-44 parameter set defined in FIPS 204.
func MLDSA44() Parameters {
	return Parameters{
		name:          "ML-DSA-44",
		publicKeySize: MLDSA44PublicKeySize,
		signatureSize: MLDSA44SignatureSize,
	}
}

// MLDSA65 returns the ML-DSA-65 parameter set defined in FIPS 204.
func MLDSA65() Parameters {
	return Parameters{
		name:          "ML-DSA-65",
		publicKeySize: MLDSA65PublicKeySize,
		signatureSize: MLDSA65SignatureSize,
	}
}

// MLDSA87 returns the ML-DSA-87 parameter set defined in FIPS 204.
func MLDSA87() Parameters {
	return Parameters{
		name:          "ML-DSA-87",
		publicKeySize: MLDSA87PublicKeySize,
		signatureSize: MLDSA87SignatureSize,
	}
}

// PublicKeySize returns the size of public keys for this parameter set, in bytes.
func (params Parameters) PublicKeySize() int {
	return params.publicKeySize
}

// SignatureSize returns the size of signatures for this parameter set, in bytes.
func (params Parameters) SignatureSize() int {
	return params.signatureSize
}

// String returns the name of the parameter set, e.g. "ML-DSA-44".
func (params Parameters) String() string {
	return params.name
}

// Options contains additional options for signing and verifying ML-DSA signatures.
type Options struct {
	// Context can be used to distinguish signatures created for different
	// purposes. It must be at most 255 bytes long, and it is empty by default.
	//
	// The same context must be used when signing and verifying a signature.
	Context string
}

// HashFunc returns zero, to implement the [crypto.SignerOpts] interface.
func (opts *Options) HashFunc() crypto.Hash {
	return 0
}

```

// === FILE: references/go/src/crypto/mldsa/mldsa_fips140v1.0.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build fips140v1.0

package mldsa

import (
	"crypto"
	"errors"
	"io"
)

// This file provides stub implementations of the ML-DSA API for building
// against the FIPS 140-3 Go Cryptographic Module v1.0.0, which does not include
// ML-DSA. Top-level functions return an error, and methods are unreachable
// since there is no way to construct a valid PublicKey or PrivateKey.

var errUnavailable = errors.New("mldsa: unavailable in FIPS 140-3 Go Cryptographic Module v1.0.0")

// PrivateKey is an in-memory ML-DSA private key. It implements [crypto.Signer]
// and the informal extended [crypto.PrivateKey] interface.
//
// A PrivateKey is safe for concurrent use.
type PrivateKey struct{}

// GenerateKey generates a new random ML-DSA private key.
func GenerateKey(params Parameters) (*PrivateKey, error) {
	return nil, errUnavailable
}

// NewPrivateKey decodes an ML-DSA private key from the given seed.
//
// The seed must be exactly [PrivateKeySize] bytes long.
func NewPrivateKey(params Parameters, seed []byte) (*PrivateKey, error) {
	return nil, errUnavailable
}

// Public returns the corresponding [PublicKey] for this private key.
//
// It implements the [crypto.Signer] interface.
func (sk *PrivateKey) Public() crypto.PublicKey {
	panic("mldsa: methods are unreachable in FIPS 140-3 Go Cryptographic Module v1.0.0")
}

// Equal reports whether sk and x are the same key (i.e. they are derived from
// the same seed).
//
// If x is not a *PrivateKey, Equal returns false.
func (sk *PrivateKey) Equal(x crypto.PrivateKey) bool {
	panic("mldsa: methods are unreachable in FIPS 140-3 Go Cryptographic Module v1.0.0")
}

// PublicKey returns the corresponding [PublicKey] for this private key.
func (sk *PrivateKey) PublicKey() *PublicKey {
	panic("mldsa: methods are unreachable in FIPS 140-3 Go Cryptographic Module v1.0.0")
}

// Bytes returns the private key seed.
func (sk *PrivateKey) Bytes() []byte {
	panic("mldsa: methods are unreachable in FIPS 140-3 Go Cryptographic Module v1.0.0")
}

// Sign returns a signature of the given message using this private key.
//
// If opts is nil or opts.HashFunc returns zero, the message is signed directly.
// If opts.HashFunc returns [crypto.MLDSAMu], the provided message must be a
// [pre-hashed μ message representative]. opts can be of type *[Options].
// The io.Reader argument is ignored.
//
// [pre-hashed μ message representative]: https://www.rfc-editor.org/rfc/rfc9881.html#externalmu
func (sk *PrivateKey) Sign(_ io.Reader, message []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	panic("mldsa: methods are unreachable in FIPS 140-3 Go Cryptographic Module v1.0.0")
}

// SignDeterministic works like [PrivateKey.Sign], but the signature is
// deterministic.
func (sk *PrivateKey) SignDeterministic(message []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	panic("mldsa: methods are unreachable in FIPS 140-3 Go Cryptographic Module v1.0.0")
}

// PublicKey is an ML-DSA public key. It implements the informal extended
// [crypto.PublicKey] interface.
//
// A PublicKey is safe for concurrent use.
type PublicKey struct{}

// NewPublicKey creates a new ML-DSA public key from the given encoding.
func NewPublicKey(params Parameters, encoding []byte) (*PublicKey, error) {
	return nil, errUnavailable
}

// Bytes returns the public key encoding.
func (pk *PublicKey) Bytes() []byte {
	panic("mldsa: methods are unreachable in FIPS 140-3 Go Cryptographic Module v1.0.0")
}

// Equal reports whether pk and x are the same key (i.e. they have the same
// encoding).
//
// If x is not a *PublicKey, Equal returns false.
func (pk *PublicKey) Equal(x crypto.PublicKey) bool {
	panic("mldsa: methods are unreachable in FIPS 140-3 Go Cryptographic Module v1.0.0")
}

// Parameters returns the parameters associated with this public key.
func (pk *PublicKey) Parameters() Parameters {
	panic("mldsa: methods are unreachable in FIPS 140-3 Go Cryptographic Module v1.0.0")
}

// Verify reports whether signature is a valid signature of message by pk.
// If opts is nil, it's equivalent to the zero value of Options.
func Verify(pk *PublicKey, message []byte, signature []byte, opts *Options) error {
	return errUnavailable
}

```

// === FILE: references/go/src/crypto/mldsa/mldsa_fips140v1.26.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !fips140v1.0

package mldsa

import (
	"crypto"
	"crypto/internal/fips140/mldsa"
	"errors"
	"io"
)

// PrivateKey is an in-memory ML-DSA private key. It implements [crypto.Signer]
// and the informal extended [crypto.PrivateKey] interface.
//
// A PrivateKey is safe for concurrent use.
type PrivateKey struct {
	k mldsa.PrivateKey
}

var errInvalidParameters = errors.New("mldsa: invalid parameters")

// GenerateKey generates a new random ML-DSA private key.
func GenerateKey(params Parameters) (*PrivateKey, error) {
	switch params {
	case MLDSA44():
		return &PrivateKey{k: *mldsa.GenerateKey44()}, nil
	case MLDSA65():
		return &PrivateKey{k: *mldsa.GenerateKey65()}, nil
	case MLDSA87():
		return &PrivateKey{k: *mldsa.GenerateKey87()}, nil
	default:
		return nil, errInvalidParameters
	}
}

// NewPrivateKey decodes an ML-DSA private key from the given seed.
//
// The seed must be exactly [PrivateKeySize] bytes long.
func NewPrivateKey(params Parameters, seed []byte) (*PrivateKey, error) {
	var err error
	var k *mldsa.PrivateKey
	switch params {
	case MLDSA44():
		k, err = mldsa.NewPrivateKey44(seed)
	case MLDSA65():
		k, err = mldsa.NewPrivateKey65(seed)
	case MLDSA87():
		k, err = mldsa.NewPrivateKey87(seed)
	default:
		return nil, errInvalidParameters
	}
	if err != nil {
		return nil, err
	}
	return &PrivateKey{k: *k}, nil
}

// Public returns the corresponding [PublicKey] for this private key.
//
// It implements the [crypto.Signer] interface.
func (sk *PrivateKey) Public() crypto.PublicKey {
	return sk.PublicKey()
}

// Equal reports whether sk and x are the same key (i.e. they are derived from
// the same seed).
//
// If x is not a *PrivateKey, Equal returns false.
func (sk *PrivateKey) Equal(x crypto.PrivateKey) bool {
	other, ok := x.(*PrivateKey)
	if !ok || other == nil {
		return false
	}
	return sk.k.Equal(&other.k)
}

// PublicKey returns the corresponding [PublicKey] for this private key.
func (sk *PrivateKey) PublicKey() *PublicKey {
	// Making a copy severs the pointer relationship between the private and
	// public keys, so that keeping the public key around doesn't keep the
	// private key alive. This costs a copy and an allocation.
	return &PublicKey{p: *sk.k.PublicKey()}
}

// Bytes returns the private key seed.
func (sk *PrivateKey) Bytes() []byte {
	return sk.k.Bytes()
}

var errInvalidSignerOpts = errors.New("mldsa: invalid SignerOpts")

// Sign returns a signature of the given message using this private key.
//
// If opts is nil or opts.HashFunc returns zero, the message is signed directly.
// If opts.HashFunc returns [crypto.MLDSAMu], the provided message must be a
// [pre-hashed μ message representative]. opts can be of type *[Options] if a
// context string is desired along with a directly-signed message. The io.Reader
// argument is ignored.
//
// [pre-hashed μ message representative]: https://www.rfc-editor.org/rfc/rfc9881.html#externalmu
func (sk *PrivateKey) Sign(_ io.Reader, message []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	if opts == nil {
		opts = &Options{}
	}
	switch opts.HashFunc() {
	case 0:
		var context string
		if opts, ok := opts.(*Options); ok {
			context = opts.Context
		}
		return mldsa.Sign(&sk.k, message, context)
	case crypto.MLDSAMu:
		return mldsa.SignExternalMu(&sk.k, message)
	default:
		return nil, errInvalidSignerOpts
	}
}

// SignDeterministic works like [PrivateKey.Sign], but the signature is
// deterministic.
func (sk *PrivateKey) SignDeterministic(message []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	if opts == nil {
		opts = &Options{}
	}
	switch opts.HashFunc() {
	case 0:
		var context string
		if opts, ok := opts.(*Options); ok {
			context = opts.Context
		}
		return mldsa.SignDeterministic(&sk.k, message, context)
	case crypto.MLDSAMu:
		return mldsa.SignExternalMuDeterministic(&sk.k, message)
	default:
		return nil, errInvalidSignerOpts
	}
}

// PublicKey is an ML-DSA public key. It implements the informal extended
// [crypto.PublicKey] interface.
//
// A PublicKey is safe for concurrent use.
type PublicKey struct {
	p mldsa.PublicKey
}

// NewPublicKey creates a new ML-DSA public key from the given encoding.
func NewPublicKey(params Parameters, encoding []byte) (*PublicKey, error) {
	return newPublicKey(&PublicKey{}, params, encoding)
}

func newPublicKey(pub *PublicKey, params Parameters, encoding []byte) (*PublicKey, error) {
	var err error
	var pk *mldsa.PublicKey
	switch params {
	case MLDSA44():
		pk, err = mldsa.NewPublicKey44(encoding)
	case MLDSA65():
		pk, err = mldsa.NewPublicKey65(encoding)
	case MLDSA87():
		pk, err = mldsa.NewPublicKey87(encoding)
	default:
		return nil, errInvalidParameters
	}
	if err != nil {
		return nil, err
	}
	pub.p = *pk
	return pub, nil
}

// Bytes returns the public key encoding.
func (pk *PublicKey) Bytes() []byte {
	return pk.p.Bytes()
}

// Equal reports whether pk and x are the same key (i.e. they have the same
// encoding).
//
// If x is not a *PublicKey, Equal returns false.
func (pk *PublicKey) Equal(x crypto.PublicKey) bool {
	other, ok := x.(*PublicKey)
	if !ok || other == nil {
		return false
	}
	return pk.p.Equal(&other.p)
}

// Parameters returns the parameters associated with this public key.
func (pk *PublicKey) Parameters() Parameters {
	switch pk.p.Parameters() {
	case "ML-DSA-44":
		return MLDSA44()
	case "ML-DSA-65":
		return MLDSA65()
	case "ML-DSA-87":
		return MLDSA87()
	default:
		panic("mldsa: invalid parameters in public key")
	}
}

// Verify reports whether signature is a valid signature of message by pk.
// If opts is nil, it's equivalent to the zero value of Options.
func Verify(pk *PublicKey, message []byte, signature []byte, opts *Options) error {
	if pk == nil {
		return errors.New("mldsa: nil public key")
	}
	if opts == nil {
		opts = &Options{}
	}
	return mldsa.Verify(&pk.p, message, signature, opts.Context)
}

```

