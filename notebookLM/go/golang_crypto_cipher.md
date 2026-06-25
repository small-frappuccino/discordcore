# Domain Architecture: crypto/cipher

## Layout Topology
```text
crypto/cipher/
├── cbc.go
├── cfb.go
├── cipher.go
├── ctr.go
├── gcm.go
├── io.go
└── ofb.go
```

## Source Stream Aggregation

// === FILE: references!/go/src/crypto/cipher/cbc.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Cipher block chaining (CBC) mode.

// CBC provides confidentiality by xoring (chaining) each plaintext block
// with the previous ciphertext block before applying the block cipher.

// See NIST SP 800-38A, pp 10-11

package cipher

import (
	"bytes"
	"crypto/internal/fips140/aes"
	"crypto/internal/fips140/alias"
	"crypto/internal/fips140only"
	"crypto/subtle"
)

type cbc struct {
	b         Block
	blockSize int
	iv        []byte
	tmp       []byte
}

func newCBC(b Block, iv []byte) *cbc {
	return &cbc{
		b:         b,
		blockSize: b.BlockSize(),
		iv:        bytes.Clone(iv),
		tmp:       make([]byte, b.BlockSize()),
	}
}

type cbcEncrypter cbc

// cbcEncAble is an interface implemented by ciphers that have a specific
// optimized implementation of CBC encryption. crypto/aes doesn't use this
// anymore, and we'd like to eventually remove it.
type cbcEncAble interface {
	NewCBCEncrypter(iv []byte) BlockMode
}

// NewCBCEncrypter returns a BlockMode which encrypts in cipher block chaining
// mode, using the given Block. The length of iv must be the same as the
// Block's block size.
func NewCBCEncrypter(b Block, iv []byte) BlockMode {
	if len(iv) != b.BlockSize() {
		panic("cipher.NewCBCEncrypter: IV length must equal block size")
	}
	if b, ok := b.(*aes.Block); ok {
		return aes.NewCBCEncrypter(b, [16]byte(iv))
	}
	if fips140only.Enforced() {
		panic("crypto/cipher: use of CBC with non-AES ciphers is not allowed in FIPS 140-only mode")
	}
	if cbc, ok := b.(cbcEncAble); ok {
		return cbc.NewCBCEncrypter(iv)
	}
	return (*cbcEncrypter)(newCBC(b, iv))
}

// newCBCGenericEncrypter returns a BlockMode which encrypts in cipher block chaining
// mode, using the given Block. The length of iv must be the same as the
// Block's block size. This always returns the generic non-asm encrypter for use
// in fuzz testing.
func newCBCGenericEncrypter(b Block, iv []byte) BlockMode {
	if len(iv) != b.BlockSize() {
		panic("cipher.NewCBCEncrypter: IV length must equal block size")
	}
	return (*cbcEncrypter)(newCBC(b, iv))
}

func (x *cbcEncrypter) BlockSize() int { return x.blockSize }

func (x *cbcEncrypter) CryptBlocks(dst, src []byte) {
	if len(src)%x.blockSize != 0 {
		panic("crypto/cipher: input not full blocks")
	}
	if len(dst) < len(src) {
		panic("crypto/cipher: output smaller than input")
	}
	if alias.InexactOverlap(dst[:len(src)], src) {
		panic("crypto/cipher: invalid buffer overlap")
	}
	if _, ok := x.b.(*aes.Block); ok {
		panic("crypto/cipher: internal error: generic CBC used with AES")
	}

	iv := x.iv

	for len(src) > 0 {
		// Write the xor to dst, then encrypt in place.
		subtle.XORBytes(dst[:x.blockSize], src[:x.blockSize], iv)
		x.b.Encrypt(dst[:x.blockSize], dst[:x.blockSize])

		// Move to the next block with this block as the next iv.
		iv = dst[:x.blockSize]
		src = src[x.blockSize:]
		dst = dst[x.blockSize:]
	}

	// Save the iv for the next CryptBlocks call.
	copy(x.iv, iv)
}

func (x *cbcEncrypter) SetIV(iv []byte) {
	if len(iv) != len(x.iv) {
		panic("cipher: incorrect length IV")
	}
	copy(x.iv, iv)
}

type cbcDecrypter cbc

// cbcDecAble is an interface implemented by ciphers that have a specific
// optimized implementation of CBC decryption. crypto/aes doesn't use this
// anymore, and we'd like to eventually remove it.
type cbcDecAble interface {
	NewCBCDecrypter(iv []byte) BlockMode
}

// NewCBCDecrypter returns a BlockMode which decrypts in cipher block chaining
// mode, using the given Block. The length of iv must be the same as the
// Block's block size and must match the iv used to encrypt the data.
func NewCBCDecrypter(b Block, iv []byte) BlockMode {
	if len(iv) != b.BlockSize() {
		panic("cipher.NewCBCDecrypter: IV length must equal block size")
	}
	if b, ok := b.(*aes.Block); ok {
		return aes.NewCBCDecrypter(b, [16]byte(iv))
	}
	if fips140only.Enforced() {
		panic("crypto/cipher: use of CBC with non-AES ciphers is not allowed in FIPS 140-only mode")
	}
	if cbc, ok := b.(cbcDecAble); ok {
		return cbc.NewCBCDecrypter(iv)
	}
	return (*cbcDecrypter)(newCBC(b, iv))
}

// newCBCGenericDecrypter returns a BlockMode which encrypts in cipher block chaining
// mode, using the given Block. The length of iv must be the same as the
// Block's block size. This always returns the generic non-asm decrypter for use in
// fuzz testing.
func newCBCGenericDecrypter(b Block, iv []byte) BlockMode {
	if len(iv) != b.BlockSize() {
		panic("cipher.NewCBCDecrypter: IV length must equal block size")
	}
	return (*cbcDecrypter)(newCBC(b, iv))
}

func (x *cbcDecrypter) BlockSize() int { return x.blockSize }

func (x *cbcDecrypter) CryptBlocks(dst, src []byte) {
	if len(src)%x.blockSize != 0 {
		panic("crypto/cipher: input not full blocks")
	}
	if len(dst) < len(src) {
		panic("crypto/cipher: output smaller than input")
	}
	if alias.InexactOverlap(dst[:len(src)], src) {
		panic("crypto/cipher: invalid buffer overlap")
	}
	if _, ok := x.b.(*aes.Block); ok {
		panic("crypto/cipher: internal error: generic CBC used with AES")
	}
	if len(src) == 0 {
		return
	}

	// For each block, we need to xor the decrypted data with the previous block's ciphertext (the iv).
	// To avoid making a copy each time, we loop over the blocks BACKWARDS.
	end := len(src)
	start := end - x.blockSize
	prev := start - x.blockSize

	// Copy the last block of ciphertext in preparation as the new iv.
	copy(x.tmp, src[start:end])

	// Loop over all but the first block.
	for start > 0 {
		x.b.Decrypt(dst[start:end], src[start:end])
		subtle.XORBytes(dst[start:end], dst[start:end], src[prev:start])

		end = start
		start = prev
		prev -= x.blockSize
	}

	// The first block is special because it uses the saved iv.
	x.b.Decrypt(dst[start:end], src[start:end])
	subtle.XORBytes(dst[start:end], dst[start:end], x.iv)

	// Set the new iv to the first block we copied earlier.
	x.iv, x.tmp = x.tmp, x.iv
}

func (x *cbcDecrypter) SetIV(iv []byte) {
	if len(iv) != len(x.iv) {
		panic("cipher: incorrect length IV")
	}
	copy(x.iv, iv)
}

```

// === FILE: references!/go/src/crypto/cipher/cfb.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// CFB (Cipher Feedback) Mode.

package cipher

import (
	"crypto/internal/fips140/alias"
	"crypto/internal/fips140only"
	"crypto/subtle"
)

type cfb struct {
	b       Block
	next    []byte
	out     []byte
	outUsed int

	decrypt bool
}

func (x *cfb) XORKeyStream(dst, src []byte) {
	if len(dst) < len(src) {
		panic("crypto/cipher: output smaller than input")
	}
	if alias.InexactOverlap(dst[:len(src)], src) {
		panic("crypto/cipher: invalid buffer overlap")
	}
	for len(src) > 0 {
		if x.outUsed == len(x.out) {
			x.b.Encrypt(x.out, x.next)
			x.outUsed = 0
		}

		if x.decrypt {
			// We can precompute a larger segment of the
			// keystream on decryption. This will allow
			// larger batches for xor, and we should be
			// able to match CTR/OFB performance.
			copy(x.next[x.outUsed:], src)
		}
		n := subtle.XORBytes(dst, src, x.out[x.outUsed:])
		if !x.decrypt {
			copy(x.next[x.outUsed:], dst)
		}
		dst = dst[n:]
		src = src[n:]
		x.outUsed += n
	}
}

// NewCFBEncrypter returns a [Stream] which encrypts with cipher feedback mode,
// using the given [Block]. The iv must be the same length as the [Block]'s block
// size.
//
// Deprecated: CFB mode is not authenticated, which generally enables active
// attacks to manipulate and recover the plaintext. It is recommended that
// applications use [AEAD] modes instead. The standard library implementation of
// CFB is also unoptimized and not validated as part of the FIPS 140-3 module.
// If an unauthenticated [Stream] mode is required, use [NewCTR] instead.
func NewCFBEncrypter(block Block, iv []byte) Stream {
	if fips140only.Enforced() {
		panic("crypto/cipher: use of CFB is not allowed in FIPS 140-only mode")
	}
	return newCFB(block, iv, false)
}

// NewCFBDecrypter returns a [Stream] which decrypts with cipher feedback mode,
// using the given [Block]. The iv must be the same length as the [Block]'s block
// size.
//
// Deprecated: CFB mode is not authenticated, which generally enables active
// attacks to manipulate and recover the plaintext. It is recommended that
// applications use [AEAD] modes instead. The standard library implementation of
// CFB is also unoptimized and not validated as part of the FIPS 140-3 module.
// If an unauthenticated [Stream] mode is required, use [NewCTR] instead.
func NewCFBDecrypter(block Block, iv []byte) Stream {
	if fips140only.Enforced() {
		panic("crypto/cipher: use of CFB is not allowed in FIPS 140-only mode")
	}
	return newCFB(block, iv, true)
}

func newCFB(block Block, iv []byte, decrypt bool) Stream {
	blockSize := block.BlockSize()
	if len(iv) != blockSize {
		// Stack trace will indicate whether it was de- or en-cryption.
		panic("cipher.newCFB: IV length must equal block size")
	}
	x := &cfb{
		b:       block,
		out:     make([]byte, blockSize),
		next:    make([]byte, blockSize),
		outUsed: blockSize,
		decrypt: decrypt,
	}
	copy(x.next, iv)

	return x
}

```

// === FILE: references!/go/src/crypto/cipher/cipher.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package cipher implements standard block cipher modes that can be wrapped
// around low-level block cipher implementations.
// See https://csrc.nist.gov/groups/ST/toolkit/BCM/current_modes.html
// and NIST Special Publication 800-38A.
package cipher

// A Block represents an implementation of block cipher
// using a given key. It provides the capability to encrypt
// or decrypt individual blocks. The mode implementations
// extend that capability to streams of blocks.
type Block interface {
	// BlockSize returns the cipher's block size.
	BlockSize() int

	// Encrypt encrypts the first block in src into dst.
	// Dst and src must overlap entirely or not at all.
	Encrypt(dst, src []byte)

	// Decrypt decrypts the first block in src into dst.
	// Dst and src must overlap entirely or not at all.
	Decrypt(dst, src []byte)
}

// A Stream represents a stream cipher.
type Stream interface {
	// XORKeyStream XORs each byte in the given slice with a byte from the
	// cipher's key stream. Dst and src must overlap entirely or not at all.
	//
	// If len(dst) < len(src), XORKeyStream should panic. It is acceptable
	// to pass a dst bigger than src, and in that case, XORKeyStream will
	// only update dst[:len(src)] and will not touch the rest of dst.
	//
	// Multiple calls to XORKeyStream behave as if the concatenation of
	// the src buffers was passed in a single run. That is, Stream
	// maintains state and does not reset at each XORKeyStream call.
	XORKeyStream(dst, src []byte)
}

// A BlockMode represents a block cipher running in a block-based mode (CBC,
// ECB etc).
type BlockMode interface {
	// BlockSize returns the mode's block size.
	BlockSize() int

	// CryptBlocks encrypts or decrypts a number of blocks. The length of
	// src must be a multiple of the block size. Dst and src must overlap
	// entirely or not at all.
	//
	// If len(dst) < len(src), CryptBlocks should panic. It is acceptable
	// to pass a dst bigger than src, and in that case, CryptBlocks will
	// only update dst[:len(src)] and will not touch the rest of dst.
	//
	// Multiple calls to CryptBlocks behave as if the concatenation of
	// the src buffers was passed in a single run. That is, BlockMode
	// maintains state and does not reset at each CryptBlocks call.
	CryptBlocks(dst, src []byte)
}

// AEAD is a cipher mode providing authenticated encryption with associated
// data. For a description of the methodology, see
// https://en.wikipedia.org/wiki/Authenticated_encryption.
type AEAD interface {
	// NonceSize returns the size of the nonce that must be passed to Seal
	// and Open.
	NonceSize() int

	// Overhead returns the maximum difference between the lengths of a
	// plaintext and its ciphertext.
	Overhead() int

	// Seal encrypts and authenticates plaintext, authenticates the
	// additional data and appends the result to dst, returning the updated
	// slice. The nonce must be NonceSize() bytes long and unique for all
	// time, for a given key.
	//
	// To reuse plaintext's storage for the encrypted output, use plaintext[:0]
	// as dst. Otherwise, the remaining capacity of dst must not overlap plaintext.
	// dst and additionalData may not overlap.
	Seal(dst, nonce, plaintext, additionalData []byte) []byte

	// Open decrypts and authenticates ciphertext, authenticates the
	// additional data and, if successful, appends the resulting plaintext
	// to dst, returning the updated slice. The nonce must be NonceSize()
	// bytes long and both it and the additional data must match the
	// value passed to Seal.
	//
	// To reuse ciphertext's storage for the decrypted output, use ciphertext[:0]
	// as dst. Otherwise, the remaining capacity of dst must not overlap ciphertext.
	// dst and additionalData may not overlap.
	//
	// Even if the function fails, the contents of dst, up to its capacity,
	// may be overwritten.
	Open(dst, nonce, ciphertext, additionalData []byte) ([]byte, error)
}

```

// === FILE: references!/go/src/crypto/cipher/ctr.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Counter (CTR) mode.

// CTR converts a block cipher into a stream cipher by
// repeatedly encrypting an incrementing counter and
// xoring the resulting stream of data with the input.

// See NIST SP 800-38A, pp 13-15

package cipher

import (
	"bytes"
	"crypto/internal/fips140/aes"
	"crypto/internal/fips140/alias"
	"crypto/internal/fips140only"
	"crypto/subtle"
)

type ctr struct {
	b       Block
	ctr     []byte
	out     []byte
	outUsed int
}

const streamBufferSize = 512

// ctrAble is an interface implemented by ciphers that have a specific optimized
// implementation of CTR. crypto/aes doesn't use this anymore, and we'd like to
// eventually remove it.
type ctrAble interface {
	NewCTR(iv []byte) Stream
}

// NewCTR returns a [Stream] which encrypts/decrypts using the given [Block] in
// counter mode. The length of iv must be the same as the [Block]'s block size.
func NewCTR(block Block, iv []byte) Stream {
	if block, ok := block.(*aes.Block); ok {
		return aesCtrWrapper{aes.NewCTR(block, iv)}
	}
	if fips140only.Enforced() {
		panic("crypto/cipher: use of CTR with non-AES ciphers is not allowed in FIPS 140-only mode")
	}
	if ctr, ok := block.(ctrAble); ok {
		return ctr.NewCTR(iv)
	}
	if len(iv) != block.BlockSize() {
		panic("cipher.NewCTR: IV length must equal block size")
	}
	bufSize := streamBufferSize
	if bufSize < block.BlockSize() {
		bufSize = block.BlockSize()
	}
	return &ctr{
		b:       block,
		ctr:     bytes.Clone(iv),
		out:     make([]byte, 0, bufSize),
		outUsed: 0,
	}
}

// aesCtrWrapper hides extra methods from aes.CTR.
type aesCtrWrapper struct {
	c *aes.CTR
}

func (x aesCtrWrapper) XORKeyStream(dst, src []byte) {
	x.c.XORKeyStream(dst, src)
}

func (x *ctr) refill() {
	remain := len(x.out) - x.outUsed
	copy(x.out, x.out[x.outUsed:])
	x.out = x.out[:cap(x.out)]
	bs := x.b.BlockSize()
	for remain <= len(x.out)-bs {
		x.b.Encrypt(x.out[remain:], x.ctr)
		remain += bs

		// Increment counter
		for i := len(x.ctr) - 1; i >= 0; i-- {
			x.ctr[i]++
			if x.ctr[i] != 0 {
				break
			}
		}
	}
	x.out = x.out[:remain]
	x.outUsed = 0
}

func (x *ctr) XORKeyStream(dst, src []byte) {
	if len(dst) < len(src) {
		panic("crypto/cipher: output smaller than input")
	}
	if alias.InexactOverlap(dst[:len(src)], src) {
		panic("crypto/cipher: invalid buffer overlap")
	}
	if _, ok := x.b.(*aes.Block); ok {
		panic("crypto/cipher: internal error: generic CTR used with AES")
	}
	for len(src) > 0 {
		if x.outUsed >= len(x.out)-x.b.BlockSize() {
			x.refill()
		}
		n := subtle.XORBytes(dst, src, x.out[x.outUsed:])
		dst = dst[n:]
		src = src[n:]
		x.outUsed += n
	}
}

```

// === FILE: references!/go/src/crypto/cipher/gcm.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cipher

import (
	"crypto/internal/fips140/aes"
	"crypto/internal/fips140/aes/gcm"
	"crypto/internal/fips140/alias"
	"crypto/internal/fips140only"
	"crypto/subtle"
	"errors"
	"internal/byteorder"
)

const (
	gcmBlockSize         = 16
	gcmStandardNonceSize = 12
	gcmTagSize           = 16
	gcmMinimumTagSize    = 12 // NIST SP 800-38D recommends tags with 12 or more bytes.
)

// NewGCM returns the given 128-bit, block cipher wrapped in Galois Counter Mode
// with the standard nonce length.
func NewGCM(cipher Block) (AEAD, error) {
	if fips140only.Enforced() {
		return nil, errors.New("crypto/cipher: use of GCM with arbitrary IVs is not allowed in FIPS 140-only mode, use NewGCMWithRandomNonce")
	}
	return newGCM(cipher, gcmStandardNonceSize, gcmTagSize)
}

// NewGCMWithNonceSize returns the given 128-bit, block cipher wrapped in Galois
// Counter Mode, which accepts nonces of the given length. The length must not
// be zero.
//
// Only use this function if you require compatibility with an existing
// cryptosystem that uses non-standard nonce lengths. All other users should use
// [NewGCM], which is faster and more resistant to misuse.
func NewGCMWithNonceSize(cipher Block, size int) (AEAD, error) {
	if fips140only.Enforced() {
		return nil, errors.New("crypto/cipher: use of GCM with arbitrary IVs is not allowed in FIPS 140-only mode, use NewGCMWithRandomNonce")
	}
	return newGCM(cipher, size, gcmTagSize)
}

// NewGCMWithTagSize returns the given 128-bit, block cipher wrapped in Galois
// Counter Mode, which generates tags with the given length.
//
// Tag sizes between 12 and 16 bytes are allowed.
//
// Only use this function if you require compatibility with an existing
// cryptosystem that uses non-standard tag lengths. All other users should use
// [NewGCM], which is more resistant to misuse.
func NewGCMWithTagSize(cipher Block, tagSize int) (AEAD, error) {
	if fips140only.Enforced() {
		return nil, errors.New("crypto/cipher: use of GCM with arbitrary IVs is not allowed in FIPS 140-only mode, use NewGCMWithRandomNonce")
	}
	return newGCM(cipher, gcmStandardNonceSize, tagSize)
}

func newGCM(cipher Block, nonceSize, tagSize int) (AEAD, error) {
	c, ok := cipher.(*aes.Block)
	if !ok {
		if fips140only.Enforced() {
			return nil, errors.New("crypto/cipher: use of GCM with non-AES ciphers is not allowed in FIPS 140-only mode")
		}
		return newGCMFallback(cipher, nonceSize, tagSize)
	}
	// We don't return gcm.New directly, because it would always return a non-nil
	// AEAD interface value with type *gcm.GCM even if the *gcm.GCM is nil.
	g, err := gcm.New(c, nonceSize, tagSize)
	if err != nil {
		return nil, err
	}
	return g, nil
}

// NewGCMWithRandomNonce returns the given cipher wrapped in Galois Counter
// Mode, with randomly-generated nonces. The cipher must have been created by
// [crypto/aes.NewCipher].
//
// It generates a random 96-bit nonce, which is prepended to the ciphertext by Seal,
// and is extracted from the ciphertext by Open. The NonceSize of the AEAD is zero,
// while the Overhead is 28 bytes (the combination of nonce size and tag size).
//
// A given key MUST NOT be used to encrypt more than 2^32 messages, to limit the
// risk of a random nonce collision to negligible levels.
func NewGCMWithRandomNonce(cipher Block) (AEAD, error) {
	c, ok := cipher.(*aes.Block)
	if !ok {
		return nil, errors.New("cipher: NewGCMWithRandomNonce requires aes.Block")
	}
	g, err := gcm.New(c, gcmStandardNonceSize, gcmTagSize)
	if err != nil {
		return nil, err
	}
	return gcmWithRandomNonce{g}, nil
}

type gcmWithRandomNonce struct {
	*gcm.GCM
}

func (g gcmWithRandomNonce) NonceSize() int {
	return 0
}

func (g gcmWithRandomNonce) Overhead() int {
	return gcmStandardNonceSize + gcmTagSize
}

func (g gcmWithRandomNonce) Seal(dst, nonce, plaintext, additionalData []byte) []byte {
	if len(nonce) != 0 {
		panic("crypto/cipher: non-empty nonce passed to GCMWithRandomNonce")
	}

	ret, out := sliceForAppend(dst, gcmStandardNonceSize+len(plaintext)+gcmTagSize)
	if alias.InexactOverlap(out, plaintext) {
		panic("crypto/cipher: invalid buffer overlap of output and input")
	}
	if alias.AnyOverlap(out, additionalData) {
		panic("crypto/cipher: invalid buffer overlap of output and additional data")
	}
	nonce = out[:gcmStandardNonceSize]
	ciphertext := out[gcmStandardNonceSize:]

	// The AEAD interface allows using plaintext[:0] or ciphertext[:0] as dst.
	//
	// This is kind of a problem when trying to prepend or trim a nonce, because the
	// actual AES-GCTR blocks end up overlapping but not exactly.
	//
	// In Open, we write the output *before* the input, so unless we do something
	// weird like working through a chunk of block backwards, it works out.
	//
	// In Seal, we could work through the input backwards or intentionally load
	// ahead before writing.
	//
	// However, the crypto/internal/fips140/aes/gcm APIs also check for exact overlap,
	// so for now we just do a memmove if we detect overlap.
	//
	//     ┌───────────────────────────┬ ─ ─
	//     │PPPPPPPPPPPPPPPPPPPPPPPPPPP│    │
	//     └▽─────────────────────────▲┴ ─ ─
	//       ╲ Seal                    ╲
	//        ╲                    Open ╲
	//     ┌───▼─────────────────────────△──┐
	//     │NN|CCCCCCCCCCCCCCCCCCCCCCCCCCC|T│
	//     └────────────────────────────────┘
	//
	if alias.AnyOverlap(out, plaintext) {
		copy(ciphertext, plaintext)
		plaintext = ciphertext[:len(plaintext)]
	}

	gcm.SealWithRandomNonce(g.GCM, nonce, ciphertext, plaintext, additionalData)
	return ret
}

func (g gcmWithRandomNonce) Open(dst, nonce, ciphertext, additionalData []byte) ([]byte, error) {
	if len(nonce) != 0 {
		panic("crypto/cipher: non-empty nonce passed to GCMWithRandomNonce")
	}
	if len(ciphertext) < gcmStandardNonceSize+gcmTagSize {
		return nil, errOpen
	}

	ret, out := sliceForAppend(dst, len(ciphertext)-gcmStandardNonceSize-gcmTagSize)
	if alias.InexactOverlap(out, ciphertext) {
		panic("crypto/cipher: invalid buffer overlap of output and input")
	}
	if alias.AnyOverlap(out, additionalData) {
		panic("crypto/cipher: invalid buffer overlap of output and additional data")
	}
	// See the discussion in Seal. Note that if there is any overlap at this
	// point, it's because out = ciphertext, so out must have enough capacity
	// even if we sliced the tag off. Also note how [AEAD] specifies that "the
	// contents of dst, up to its capacity, may be overwritten".
	if alias.AnyOverlap(out, ciphertext) {
		nonce = make([]byte, gcmStandardNonceSize)
		copy(nonce, ciphertext)
		copy(out[:len(ciphertext)], ciphertext[gcmStandardNonceSize:])
		ciphertext = out[:len(ciphertext)-gcmStandardNonceSize]
	} else {
		nonce = ciphertext[:gcmStandardNonceSize]
		ciphertext = ciphertext[gcmStandardNonceSize:]
	}

	_, err := g.GCM.Open(out[:0], nonce, ciphertext, additionalData)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// gcmAble is an interface implemented by ciphers that have a specific optimized
// implementation of GCM. crypto/aes doesn't use this anymore, and we'd like to
// eventually remove it.
type gcmAble interface {
	NewGCM(nonceSize, tagSize int) (AEAD, error)
}

func newGCMFallback(cipher Block, nonceSize, tagSize int) (AEAD, error) {
	if tagSize < gcmMinimumTagSize || tagSize > gcmBlockSize {
		return nil, errors.New("cipher: incorrect tag size given to GCM")
	}
	if nonceSize <= 0 {
		return nil, errors.New("cipher: the nonce can't have zero length")
	}
	if cipher, ok := cipher.(gcmAble); ok {
		return cipher.NewGCM(nonceSize, tagSize)
	}
	if cipher.BlockSize() != gcmBlockSize {
		return nil, errors.New("cipher: NewGCM requires 128-bit block cipher")
	}
	return &gcmFallback{cipher: cipher, nonceSize: nonceSize, tagSize: tagSize}, nil
}

// gcmFallback is only used for non-AES ciphers, which regrettably we
// theoretically support. It's a copy of the generic implementation from
// crypto/internal/fips140/aes/gcm/gcm_generic.go, refer to that file for more details.
type gcmFallback struct {
	cipher    Block
	nonceSize int
	tagSize   int
}

func (g *gcmFallback) NonceSize() int {
	return g.nonceSize
}

func (g *gcmFallback) Overhead() int {
	return g.tagSize
}

func (g *gcmFallback) Seal(dst, nonce, plaintext, additionalData []byte) []byte {
	if len(nonce) != g.nonceSize {
		panic("crypto/cipher: incorrect nonce length given to GCM")
	}
	if g.nonceSize == 0 {
		panic("crypto/cipher: incorrect GCM nonce size")
	}
	if uint64(len(plaintext)) > uint64((1<<32)-2)*gcmBlockSize {
		panic("crypto/cipher: message too large for GCM")
	}

	ret, out := sliceForAppend(dst, len(plaintext)+g.tagSize)
	if alias.InexactOverlap(out, plaintext) {
		panic("crypto/cipher: invalid buffer overlap of output and input")
	}
	if alias.AnyOverlap(out, additionalData) {
		panic("crypto/cipher: invalid buffer overlap of output and additional data")
	}

	var H, counter, tagMask [gcmBlockSize]byte
	g.cipher.Encrypt(H[:], H[:])
	deriveCounter(&H, &counter, nonce)
	gcmCounterCryptGeneric(g.cipher, tagMask[:], tagMask[:], &counter)

	gcmCounterCryptGeneric(g.cipher, out, plaintext, &counter)

	var tag [gcmTagSize]byte
	gcmAuth(tag[:], &H, &tagMask, out[:len(plaintext)], additionalData)
	copy(out[len(plaintext):], tag[:])

	return ret
}

var errOpen = errors.New("cipher: message authentication failed")

func (g *gcmFallback) Open(dst, nonce, ciphertext, additionalData []byte) ([]byte, error) {
	if len(nonce) != g.nonceSize {
		panic("crypto/cipher: incorrect nonce length given to GCM")
	}
	if g.tagSize < gcmMinimumTagSize {
		panic("crypto/cipher: incorrect GCM tag size")
	}

	if len(ciphertext) < g.tagSize {
		return nil, errOpen
	}
	if uint64(len(ciphertext)) > uint64((1<<32)-2)*gcmBlockSize+uint64(g.tagSize) {
		return nil, errOpen
	}

	ret, out := sliceForAppend(dst, len(ciphertext)-g.tagSize)
	if alias.InexactOverlap(out, ciphertext) {
		panic("crypto/cipher: invalid buffer overlap of output and input")
	}
	if alias.AnyOverlap(out, additionalData) {
		panic("crypto/cipher: invalid buffer overlap of output and additional data")
	}

	var H, counter, tagMask [gcmBlockSize]byte
	g.cipher.Encrypt(H[:], H[:])
	deriveCounter(&H, &counter, nonce)
	gcmCounterCryptGeneric(g.cipher, tagMask[:], tagMask[:], &counter)

	tag := ciphertext[len(ciphertext)-g.tagSize:]
	ciphertext = ciphertext[:len(ciphertext)-g.tagSize]

	var expectedTag [gcmTagSize]byte
	gcmAuth(expectedTag[:], &H, &tagMask, ciphertext, additionalData)
	if subtle.ConstantTimeCompare(expectedTag[:g.tagSize], tag) != 1 {
		// We sometimes decrypt and authenticate concurrently, so we overwrite
		// dst in the event of a tag mismatch. To be consistent across platforms
		// and to avoid releasing unauthenticated plaintext, we clear the buffer
		// in the event of an error.
		clear(out)
		return nil, errOpen
	}

	gcmCounterCryptGeneric(g.cipher, out, ciphertext, &counter)

	return ret, nil
}

func deriveCounter(H, counter *[gcmBlockSize]byte, nonce []byte) {
	if len(nonce) == gcmStandardNonceSize {
		copy(counter[:], nonce)
		counter[gcmBlockSize-1] = 1
	} else {
		lenBlock := make([]byte, 16)
		byteorder.BEPutUint64(lenBlock[8:], uint64(len(nonce))*8)
		J := gcm.GHASH(H, nonce, lenBlock)
		copy(counter[:], J)
	}
}

func gcmCounterCryptGeneric(b Block, out, src []byte, counter *[gcmBlockSize]byte) {
	var mask [gcmBlockSize]byte
	for len(src) >= gcmBlockSize {
		b.Encrypt(mask[:], counter[:])
		gcmInc32(counter)

		subtle.XORBytes(out, src, mask[:])
		out = out[gcmBlockSize:]
		src = src[gcmBlockSize:]
	}
	if len(src) > 0 {
		b.Encrypt(mask[:], counter[:])
		gcmInc32(counter)
		subtle.XORBytes(out, src, mask[:])
	}
}

func gcmInc32(counterBlock *[gcmBlockSize]byte) {
	ctr := counterBlock[len(counterBlock)-4:]
	byteorder.BEPutUint32(ctr, byteorder.BEUint32(ctr)+1)
}

func gcmAuth(out []byte, H, tagMask *[gcmBlockSize]byte, ciphertext, additionalData []byte) {
	lenBlock := make([]byte, 16)
	byteorder.BEPutUint64(lenBlock[:8], uint64(len(additionalData))*8)
	byteorder.BEPutUint64(lenBlock[8:], uint64(len(ciphertext))*8)
	S := gcm.GHASH(H, additionalData, ciphertext, lenBlock)
	subtle.XORBytes(out, S, tagMask[:])
}

// sliceForAppend takes a slice and a requested number of bytes. It returns a
// slice with the contents of the given slice followed by that many bytes and a
// second slice that aliases into it and contains only the extra bytes. If the
// original slice has sufficient capacity then no allocation is performed.
func sliceForAppend(in []byte, n int) (head, tail []byte) {
	if total := len(in) + n; cap(in) >= total {
		head = in[:total]
	} else {
		head = make([]byte, total)
		copy(head, in)
	}
	tail = head[len(in):]
	return
}

```

// === FILE: references!/go/src/crypto/cipher/io.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cipher

import "io"

// The Stream* objects are so simple that all their members are public. Users
// can create them themselves.

// StreamReader wraps a [Stream] into an [io.Reader]. It calls XORKeyStream
// to process each slice of data which passes through.
type StreamReader struct {
	S Stream
	R io.Reader
}

func (r StreamReader) Read(dst []byte) (n int, err error) {
	n, err = r.R.Read(dst)
	r.S.XORKeyStream(dst[:n], dst[:n])
	return
}

// StreamWriter wraps a [Stream] into an io.Writer. It calls XORKeyStream
// to process each slice of data which passes through. If any [StreamWriter.Write]
// call returns short then the StreamWriter is out of sync and must be discarded.
// A StreamWriter has no internal buffering; [StreamWriter.Close] does not need
// to be called to flush write data.
type StreamWriter struct {
	S   Stream
	W   io.Writer
	Err error // unused
}

func (w StreamWriter) Write(src []byte) (n int, err error) {
	c := make([]byte, len(src))
	w.S.XORKeyStream(c, src)
	n, err = w.W.Write(c)
	if n != len(src) && err == nil { // should never happen
		err = io.ErrShortWrite
	}
	return
}

// Close closes the underlying Writer and returns its Close return value, if the Writer
// is also an io.Closer. Otherwise it returns nil.
func (w StreamWriter) Close() error {
	if c, ok := w.W.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

```

// === FILE: references!/go/src/crypto/cipher/ofb.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// OFB (Output Feedback) Mode.

package cipher

import (
	"crypto/internal/fips140/alias"
	"crypto/internal/fips140only"
	"crypto/subtle"
)

type ofb struct {
	b       Block
	cipher  []byte
	out     []byte
	outUsed int
}

// NewOFB returns a [Stream] that encrypts or decrypts using the block cipher b
// in output feedback mode. The initialization vector iv's length must be equal
// to b's block size.
//
// Deprecated: OFB mode is not authenticated, which generally enables active
// attacks to manipulate and recover the plaintext. It is recommended that
// applications use [AEAD] modes instead. The standard library implementation of
// OFB is also unoptimized and not validated as part of the FIPS 140-3 module.
// If an unauthenticated [Stream] mode is required, use [NewCTR] instead.
func NewOFB(b Block, iv []byte) Stream {
	if fips140only.Enforced() {
		panic("crypto/cipher: use of OFB is not allowed in FIPS 140-only mode")
	}

	blockSize := b.BlockSize()
	if len(iv) != blockSize {
		panic("cipher.NewOFB: IV length must equal block size")
	}
	bufSize := streamBufferSize
	if bufSize < blockSize {
		bufSize = blockSize
	}
	x := &ofb{
		b:       b,
		cipher:  make([]byte, blockSize),
		out:     make([]byte, 0, bufSize),
		outUsed: 0,
	}

	copy(x.cipher, iv)
	return x
}

func (x *ofb) refill() {
	bs := x.b.BlockSize()
	remain := len(x.out) - x.outUsed
	if remain > x.outUsed {
		return
	}
	copy(x.out, x.out[x.outUsed:])
	x.out = x.out[:cap(x.out)]
	for remain < len(x.out)-bs {
		x.b.Encrypt(x.cipher, x.cipher)
		copy(x.out[remain:], x.cipher)
		remain += bs
	}
	x.out = x.out[:remain]
	x.outUsed = 0
}

func (x *ofb) XORKeyStream(dst, src []byte) {
	if len(dst) < len(src) {
		panic("crypto/cipher: output smaller than input")
	}
	if alias.InexactOverlap(dst[:len(src)], src) {
		panic("crypto/cipher: invalid buffer overlap")
	}
	for len(src) > 0 {
		if x.outUsed >= len(x.out)-x.b.BlockSize() {
			x.refill()
		}
		n := subtle.XORBytes(dst, src, x.out[x.outUsed:])
		dst = dst[n:]
		src = src[n:]
		x.outUsed += n
	}
}

```

