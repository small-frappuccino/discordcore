# Domain Architecture: crypto/hpke

## Layout Topology
```text
crypto/hpke/
├── aead.go
├── aead_fips140v1.0.go
├── aead_fips140v1.26.go
├── hpke.go
├── kdf.go
├── kem.go
└── pq.go
```

## Source Stream Aggregation

// === FILE: references!/go/src/crypto/hpke/aead.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package hpke

import (
	"crypto/cipher"
	"errors"
	"fmt"

	"golang.org/x/crypto/chacha20poly1305"
)

// The AEAD is one of the three components of an HPKE ciphersuite, implementing
// symmetric encryption.
type AEAD interface {
	ID() uint16
	keySize() int
	nonceSize() int
	aead(key []byte) (cipher.AEAD, error)
}

// NewAEAD returns the AEAD implementation for the given AEAD ID.
//
// Applications are encouraged to use specific implementations like [AES128GCM]
// or [ChaCha20Poly1305] instead, unless runtime agility is required.
func NewAEAD(id uint16) (AEAD, error) {
	switch id {
	case 0x0001: // AES-128-GCM
		return AES128GCM(), nil
	case 0x0002: // AES-256-GCM
		return AES256GCM(), nil
	case 0x0003: // ChaCha20Poly1305
		return ChaCha20Poly1305(), nil
	case 0xFFFF: // Export-only
		return ExportOnly(), nil
	default:
		return nil, fmt.Errorf("unsupported AEAD %04x", id)
	}
}

// AES128GCM returns an AES-128-GCM AEAD implementation.
func AES128GCM() AEAD { return aes128GCM }

// AES256GCM returns an AES-256-GCM AEAD implementation.
func AES256GCM() AEAD { return aes256GCM }

// ChaCha20Poly1305 returns a ChaCha20Poly1305 AEAD implementation.
func ChaCha20Poly1305() AEAD { return chacha20poly1305AEAD }

// ExportOnly returns a placeholder AEAD implementation that cannot encrypt or
// decrypt, but only export secrets with [Sender.Export] or [Recipient.Export].
//
// When this is used, [Sender.Seal] and [Recipient.Open] return errors.
func ExportOnly() AEAD { return exportOnlyAEAD{} }

type aead struct {
	nK  int
	nN  int
	new func([]byte) (cipher.AEAD, error)
	id  uint16
}

var aes128GCM = &aead{
	nK:  128 / 8,
	nN:  96 / 8,
	new: newAESGCM,
	id:  0x0001,
}

var aes256GCM = &aead{
	nK:  256 / 8,
	nN:  96 / 8,
	new: newAESGCM,
	id:  0x0002,
}

var chacha20poly1305AEAD = &aead{
	nK:  chacha20poly1305.KeySize,
	nN:  chacha20poly1305.NonceSize,
	new: chacha20poly1305.New,
	id:  0x0003,
}

func (a *aead) ID() uint16 {
	return a.id
}

func (a *aead) aead(key []byte) (cipher.AEAD, error) {
	if len(key) != a.nK {
		return nil, errors.New("invalid key size")
	}
	return a.new(key)
}

func (a *aead) keySize() int {
	return a.nK
}

func (a *aead) nonceSize() int {
	return a.nN
}

type exportOnlyAEAD struct{}

func (exportOnlyAEAD) ID() uint16 {
	return 0xFFFF
}

func (exportOnlyAEAD) aead(key []byte) (cipher.AEAD, error) {
	return nil, nil
}

func (exportOnlyAEAD) keySize() int {
	return 0
}

func (exportOnlyAEAD) nonceSize() int {
	return 0
}

```

// === FILE: references!/go/src/crypto/hpke/aead_fips140v1.0.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build fips140v1.0

package hpke

import (
	"crypto/aes"
	"crypto/cipher"
)

func newAESGCM(key []byte) (cipher.AEAD, error) {
	b, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(b)
}

```

// === FILE: references!/go/src/crypto/hpke/aead_fips140v1.26.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !fips140v1.0

package hpke

import (
	"crypto/cipher"
	"crypto/internal/fips140/aes"
	"crypto/internal/fips140/aes/gcm"
)

func newAESGCM(key []byte) (cipher.AEAD, error) {
	b, err := aes.New(key)
	if err != nil {
		return nil, err
	}
	return gcm.NewGCMForHPKE(b)
}

```

// === FILE: references!/go/src/crypto/hpke/hpke.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package hpke implements Hybrid Public Key Encryption (HPKE) as defined in
// [RFC 9180].
//
// [RFC 9180]: https://www.rfc-editor.org/rfc/rfc9180.html
package hpke

import (
	"crypto/cipher"
	"errors"
	"internal/byteorder"
)

type context struct {
	suiteID []byte

	export func(string, uint16) ([]byte, error)

	aead      cipher.AEAD
	baseNonce []byte
	// seqNum starts at zero and is incremented for each Seal/Open call.
	// 64 bits are enough not to overflow for 500 years at 1ns per operation.
	seqNum uint64
}

// Sender is a sending HPKE context. It is instantiated with a specific KEM
// encapsulation key (i.e. the public key), and it is stateful, incrementing the
// nonce counter for each [Sender.Seal] call.
type Sender struct {
	*context
}

// Recipient is a receiving HPKE context. It is instantiated with a specific KEM
// decapsulation key (i.e. the secret key), and it is stateful, incrementing the
// nonce counter for each successful [Recipient.Open] call.
type Recipient struct {
	*context
}

func newContext(sharedSecret []byte, kemID uint16, kdf KDF, aead AEAD, info []byte) (*context, error) {
	sid := suiteID(kemID, kdf.ID(), aead.ID())

	if kdf.oneStage() {
		secrets := make([]byte, 0, 2+2+len(sharedSecret))
		secrets = byteorder.BEAppendUint16(secrets, 0) // empty psk
		secrets = byteorder.BEAppendUint16(secrets, uint16(len(sharedSecret)))
		secrets = append(secrets, sharedSecret...)

		ksContext := make([]byte, 0, 1+2+2+len(info))
		ksContext = append(ksContext, 0)                   // mode 0
		ksContext = byteorder.BEAppendUint16(ksContext, 0) // empty psk_id
		ksContext = byteorder.BEAppendUint16(ksContext, uint16(len(info)))
		ksContext = append(ksContext, info...)

		secret, err := kdf.labeledDerive(sid, secrets, "secret", ksContext,
			uint16(aead.keySize()+aead.nonceSize()+kdf.size()))
		if err != nil {
			return nil, err
		}
		key := secret[:aead.keySize()]
		baseNonce := secret[aead.keySize() : aead.keySize()+aead.nonceSize()]
		expSecret := secret[aead.keySize()+aead.nonceSize():]

		a, err := aead.aead(key)
		if err != nil {
			return nil, err
		}
		export := func(exporterContext string, length uint16) ([]byte, error) {
			return kdf.labeledDerive(sid, expSecret, "sec", []byte(exporterContext), length)
		}

		return &context{
			aead:      a,
			suiteID:   sid,
			export:    export,
			baseNonce: baseNonce,
		}, nil
	}

	pskIDHash, err := kdf.labeledExtract(sid, nil, "psk_id_hash", nil)
	if err != nil {
		return nil, err
	}
	infoHash, err := kdf.labeledExtract(sid, nil, "info_hash", info)
	if err != nil {
		return nil, err
	}
	ksContext := append([]byte{0}, pskIDHash...)
	ksContext = append(ksContext, infoHash...)

	secret, err := kdf.labeledExtract(sid, sharedSecret, "secret", nil)
	if err != nil {
		return nil, err
	}
	key, err := kdf.labeledExpand(sid, secret, "key", ksContext, uint16(aead.keySize()))
	if err != nil {
		return nil, err
	}
	a, err := aead.aead(key)
	if err != nil {
		return nil, err
	}
	baseNonce, err := kdf.labeledExpand(sid, secret, "base_nonce", ksContext, uint16(aead.nonceSize()))
	if err != nil {
		return nil, err
	}
	expSecret, err := kdf.labeledExpand(sid, secret, "exp", ksContext, uint16(kdf.size()))
	if err != nil {
		return nil, err
	}
	export := func(exporterContext string, length uint16) ([]byte, error) {
		return kdf.labeledExpand(sid, expSecret, "sec", []byte(exporterContext), length)
	}

	return &context{
		aead:      a,
		suiteID:   sid,
		export:    export,
		baseNonce: baseNonce,
	}, nil
}

// NewSender returns a sending HPKE context for the provided KEM encapsulation
// key (i.e. the public key), and using the ciphersuite defined by the
// combination of KEM, KDF, and AEAD.
//
// The info parameter is additional public information that must match between
// sender and recipient.
//
// The returned enc ciphertext can be used to instantiate a matching receiving
// HPKE context with the corresponding KEM decapsulation key.
func NewSender(pk PublicKey, kdf KDF, aead AEAD, info []byte) (enc []byte, s *Sender, err error) {
	sharedSecret, encapsulatedKey, err := pk.encap()
	if err != nil {
		return nil, nil, err
	}
	context, err := newContext(sharedSecret, pk.KEM().ID(), kdf, aead, info)
	if err != nil {
		return nil, nil, err
	}
	return encapsulatedKey, &Sender{context}, nil
}

// NewRecipient returns a receiving HPKE context for the provided KEM
// decapsulation key (i.e. the secret key), and using the ciphersuite defined by
// the combination of KEM, KDF, and AEAD.
//
// The enc parameter must have been produced by a matching sending HPKE context
// with the corresponding KEM encapsulation key. The info parameter is
// additional public information that must match between sender and recipient.
func NewRecipient(enc []byte, k PrivateKey, kdf KDF, aead AEAD, info []byte) (*Recipient, error) {
	sharedSecret, err := k.decap(enc)
	if err != nil {
		return nil, err
	}
	context, err := newContext(sharedSecret, k.KEM().ID(), kdf, aead, info)
	if err != nil {
		return nil, err
	}
	return &Recipient{context}, nil
}

// Seal encrypts the provided plaintext, optionally binding to the additional
// public data aad.
//
// Seal uses incrementing counters for each call, and Open on the receiving side
// must be called in the same order as Seal.
func (s *Sender) Seal(aad, plaintext []byte) ([]byte, error) {
	if s.aead == nil {
		return nil, errors.New("export-only instantiation")
	}
	ciphertext := s.aead.Seal(nil, s.nextNonce(), plaintext, aad)
	s.seqNum++
	return ciphertext, nil
}

// Seal instantiates a single-use HPKE sending HPKE context like [NewSender],
// and then encrypts the provided plaintext like [Sender.Seal] (with no aad).
// Seal returns the concatenation of the encapsulated key and the ciphertext.
func Seal(pk PublicKey, kdf KDF, aead AEAD, info, plaintext []byte) ([]byte, error) {
	enc, s, err := NewSender(pk, kdf, aead, info)
	if err != nil {
		return nil, err
	}
	ct, err := s.Seal(nil, plaintext)
	if err != nil {
		return nil, err
	}
	return append(enc, ct...), nil
}

// Export produces a secret value derived from the shared key between sender and
// recipient. length must be at most 65,535.
func (s *Sender) Export(exporterContext string, length int) ([]byte, error) {
	if length < 0 || length > 0xFFFF {
		return nil, errors.New("invalid length")
	}
	return s.export(exporterContext, uint16(length))
}

// Open decrypts the provided ciphertext, optionally binding to the additional
// public data aad, or returns an error if decryption fails.
//
// Open uses incrementing counters for each successful call, and must be called
// in the same order as Seal on the sending side.
func (r *Recipient) Open(aad, ciphertext []byte) ([]byte, error) {
	if r.aead == nil {
		return nil, errors.New("export-only instantiation")
	}
	plaintext, err := r.aead.Open(nil, r.nextNonce(), ciphertext, aad)
	if err != nil {
		return nil, err
	}
	r.seqNum++
	return plaintext, nil
}

// Open instantiates a single-use HPKE receiving HPKE context like [NewRecipient],
// and then decrypts the provided ciphertext like [Recipient.Open] (with no aad).
// ciphertext must be the concatenation of the encapsulated key and the actual ciphertext.
func Open(k PrivateKey, kdf KDF, aead AEAD, info, ciphertext []byte) ([]byte, error) {
	encSize := k.KEM().encSize()
	if len(ciphertext) < encSize {
		return nil, errors.New("ciphertext too short")
	}
	enc, ciphertext := ciphertext[:encSize], ciphertext[encSize:]
	r, err := NewRecipient(enc, k, kdf, aead, info)
	if err != nil {
		return nil, err
	}
	return r.Open(nil, ciphertext)
}

// Export produces a secret value derived from the shared key between sender and
// recipient. length must be at most 65,535.
func (r *Recipient) Export(exporterContext string, length int) ([]byte, error) {
	if length < 0 || length > 0xFFFF {
		return nil, errors.New("invalid length")
	}
	return r.export(exporterContext, uint16(length))
}

func (ctx *context) nextNonce() []byte {
	nonce := make([]byte, ctx.aead.NonceSize())
	byteorder.BEPutUint64(nonce[len(nonce)-8:], ctx.seqNum)
	for i := range ctx.baseNonce {
		nonce[i] ^= ctx.baseNonce[i]
	}
	return nonce
}

func suiteID(kemID, kdfID, aeadID uint16) []byte {
	suiteID := make([]byte, 0, 4+2+2+2)
	suiteID = append(suiteID, []byte("HPKE")...)
	suiteID = byteorder.BEAppendUint16(suiteID, kemID)
	suiteID = byteorder.BEAppendUint16(suiteID, kdfID)
	suiteID = byteorder.BEAppendUint16(suiteID, aeadID)
	return suiteID
}

```

// === FILE: references!/go/src/crypto/hpke/kdf.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package hpke

import (
	"crypto/hkdf"
	"crypto/sha256"
	"crypto/sha3"
	"crypto/sha512"
	"errors"
	"fmt"
	"hash"
	"internal/byteorder"
)

// The KDF is one of the three components of an HPKE ciphersuite, implementing
// key derivation.
type KDF interface {
	ID() uint16
	oneStage() bool
	size() int // Nh
	labeledDerive(suiteID, inputKey []byte, label string, context []byte, length uint16) ([]byte, error)
	labeledExtract(suiteID, salt []byte, label string, inputKey []byte) ([]byte, error)
	labeledExpand(suiteID, randomKey []byte, label string, info []byte, length uint16) ([]byte, error)
}

// NewKDF returns the KDF implementation for the given KDF ID.
//
// Applications are encouraged to use specific implementations like [HKDFSHA256]
// instead, unless runtime agility is required.
func NewKDF(id uint16) (KDF, error) {
	switch id {
	case 0x0001: // HKDF-SHA256
		return HKDFSHA256(), nil
	case 0x0002: // HKDF-SHA384
		return HKDFSHA384(), nil
	case 0x0003: // HKDF-SHA512
		return HKDFSHA512(), nil
	case 0x0010: // SHAKE128
		return SHAKE128(), nil
	case 0x0011: // SHAKE256
		return SHAKE256(), nil
	default:
		return nil, fmt.Errorf("unsupported KDF %04x", id)
	}
}

// HKDFSHA256 returns an HKDF-SHA256 KDF implementation.
func HKDFSHA256() KDF { return hkdfSHA256 }

// HKDFSHA384 returns an HKDF-SHA384 KDF implementation.
func HKDFSHA384() KDF { return hkdfSHA384 }

// HKDFSHA512 returns an HKDF-SHA512 KDF implementation.
func HKDFSHA512() KDF { return hkdfSHA512 }

type hkdfKDF struct {
	hash func() hash.Hash
	id   uint16
	nH   int
}

var hkdfSHA256 = &hkdfKDF{hash: sha256.New, id: 0x0001, nH: sha256.Size}
var hkdfSHA384 = &hkdfKDF{hash: sha512.New384, id: 0x0002, nH: sha512.Size384}
var hkdfSHA512 = &hkdfKDF{hash: sha512.New, id: 0x0003, nH: sha512.Size}

func (kdf *hkdfKDF) ID() uint16 {
	return kdf.id
}

func (kdf *hkdfKDF) size() int {
	return kdf.nH
}

func (kdf *hkdfKDF) oneStage() bool {
	return false
}

func (kdf *hkdfKDF) labeledDerive(_, _ []byte, _ string, _ []byte, _ uint16) ([]byte, error) {
	return nil, errors.New("hpke: internal error: labeledDerive called on two-stage KDF")
}

func (kdf *hkdfKDF) labeledExtract(suiteID []byte, salt []byte, label string, inputKey []byte) ([]byte, error) {
	labeledIKM := make([]byte, 0, 7+len(suiteID)+len(label)+len(inputKey))
	labeledIKM = append(labeledIKM, []byte("HPKE-v1")...)
	labeledIKM = append(labeledIKM, suiteID...)
	labeledIKM = append(labeledIKM, label...)
	labeledIKM = append(labeledIKM, inputKey...)
	return hkdf.Extract(kdf.hash, labeledIKM, salt)
}

func (kdf *hkdfKDF) labeledExpand(suiteID []byte, randomKey []byte, label string, info []byte, length uint16) ([]byte, error) {
	labeledInfo := make([]byte, 0, 2+7+len(suiteID)+len(label)+len(info))
	labeledInfo = byteorder.BEAppendUint16(labeledInfo, length)
	labeledInfo = append(labeledInfo, []byte("HPKE-v1")...)
	labeledInfo = append(labeledInfo, suiteID...)
	labeledInfo = append(labeledInfo, label...)
	labeledInfo = append(labeledInfo, info...)
	return hkdf.Expand(kdf.hash, randomKey, string(labeledInfo), int(length))
}

// SHAKE128 returns a SHAKE128 KDF implementation.
func SHAKE128() KDF {
	return shake128KDF
}

// SHAKE256 returns a SHAKE256 KDF implementation.
func SHAKE256() KDF {
	return shake256KDF
}

type shakeKDF struct {
	hash func() *sha3.SHAKE
	id   uint16
	nH   int
}

var shake128KDF = &shakeKDF{hash: sha3.NewSHAKE128, id: 0x0010, nH: 32}
var shake256KDF = &shakeKDF{hash: sha3.NewSHAKE256, id: 0x0011, nH: 64}

func (kdf *shakeKDF) ID() uint16 {
	return kdf.id
}

func (kdf *shakeKDF) size() int {
	return kdf.nH
}

func (kdf *shakeKDF) oneStage() bool {
	return true
}

func (kdf *shakeKDF) labeledDerive(suiteID, inputKey []byte, label string, context []byte, length uint16) ([]byte, error) {
	H := kdf.hash()
	H.Write(inputKey)
	H.Write([]byte("HPKE-v1"))
	H.Write(suiteID)
	H.Write([]byte{byte(len(label) >> 8), byte(len(label))})
	H.Write([]byte(label))
	H.Write([]byte{byte(length >> 8), byte(length)})
	H.Write(context)
	out := make([]byte, length)
	H.Read(out)
	return out, nil
}

func (kdf *shakeKDF) labeledExtract(_, _ []byte, _ string, _ []byte) ([]byte, error) {
	return nil, errors.New("hpke: internal error: labeledExtract called on one-stage KDF")
}

func (kdf *shakeKDF) labeledExpand(_, _ []byte, _ string, _ []byte, _ uint16) ([]byte, error) {
	return nil, errors.New("hpke: internal error: labeledExpand called on one-stage KDF")
}

```

// === FILE: references!/go/src/crypto/hpke/kem.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package hpke

import (
	"crypto/ecdh"
	"crypto/internal/rand"
	"errors"
	"internal/byteorder"
	"slices"
)

// A KEM is a Key Encapsulation Mechanism, one of the three components of an
// HPKE ciphersuite.
type KEM interface {
	// ID returns the HPKE KEM identifier.
	ID() uint16

	// GenerateKey generates a new key pair.
	GenerateKey() (PrivateKey, error)

	// NewPublicKey deserializes a public key from bytes.
	//
	// It implements DeserializePublicKey, as defined in RFC 9180.
	NewPublicKey([]byte) (PublicKey, error)

	// NewPrivateKey deserializes a private key from bytes.
	//
	// It implements DeserializePrivateKey, as defined in RFC 9180.
	NewPrivateKey([]byte) (PrivateKey, error)

	// DeriveKeyPair derives a key pair from the given input keying material.
	//
	// It implements DeriveKeyPair, as defined in RFC 9180.
	DeriveKeyPair(ikm []byte) (PrivateKey, error)

	encSize() int
}

// NewKEM returns the KEM implementation for the given KEM ID.
//
// Applications are encouraged to use specific implementations like [DHKEM] or
// [MLKEM768X25519] instead, unless runtime agility is required.
func NewKEM(id uint16) (KEM, error) {
	switch id {
	case 0x0010: // DHKEM(P-256, HKDF-SHA256)
		return DHKEM(ecdh.P256()), nil
	case 0x0011: // DHKEM(P-384, HKDF-SHA384)
		return DHKEM(ecdh.P384()), nil
	case 0x0012: // DHKEM(P-521, HKDF-SHA512)
		return DHKEM(ecdh.P521()), nil
	case 0x0020: // DHKEM(X25519, HKDF-SHA256)
		return DHKEM(ecdh.X25519()), nil
	case 0x0041: // ML-KEM-768
		return MLKEM768(), nil
	case 0x0042: // ML-KEM-1024
		return MLKEM1024(), nil
	case 0x647a: // MLKEM768-X25519
		return MLKEM768X25519(), nil
	case 0x0050: // MLKEM768-P256
		return MLKEM768P256(), nil
	case 0x0051: // MLKEM1024-P384
		return MLKEM1024P384(), nil
	default:
		return nil, errors.New("unsupported KEM")
	}
}

// A PublicKey is an instantiation of a KEM (one of the three components of an
// HPKE ciphersuite) with an encapsulation key (i.e. the public key).
//
// A PublicKey is usually obtained from a method of the corresponding [KEM] or
// [PrivateKey], such as [KEM.NewPublicKey] or [PrivateKey.PublicKey].
type PublicKey interface {
	// KEM returns the instantiated KEM.
	KEM() KEM

	// Bytes returns the public key as the output of SerializePublicKey.
	Bytes() []byte

	encap() (sharedSecret, enc []byte, err error)
}

// A PrivateKey is an instantiation of a KEM (one of the three components of
// an HPKE ciphersuite) with a decapsulation key (i.e. the secret key).
//
// A PrivateKey is usually obtained from a method of the corresponding [KEM],
// such as [KEM.GenerateKey] or [KEM.NewPrivateKey].
type PrivateKey interface {
	// KEM returns the instantiated KEM.
	KEM() KEM

	// Bytes returns the private key as the output of SerializePrivateKey, as
	// defined in RFC 9180.
	//
	// Note that for X25519 this might not match the input to NewPrivateKey.
	// This is a requirement of RFC 9180, Section 7.1.2.
	Bytes() ([]byte, error)

	// PublicKey returns the corresponding PublicKey.
	PublicKey() PublicKey

	decap(enc []byte) (sharedSecret []byte, err error)
}

type dhKEM struct {
	kdf     KDF
	id      uint16
	curve   ecdh.Curve
	Nsecret uint16
	Nsk     uint16
	Nenc    int
}

func (kem *dhKEM) extractAndExpand(dhKey, kemContext []byte) ([]byte, error) {
	suiteID := byteorder.BEAppendUint16([]byte("KEM"), kem.id)
	eaePRK, err := kem.kdf.labeledExtract(suiteID, nil, "eae_prk", dhKey)
	if err != nil {
		return nil, err
	}
	return kem.kdf.labeledExpand(suiteID, eaePRK, "shared_secret", kemContext, kem.Nsecret)
}

func (kem *dhKEM) ID() uint16 {
	return kem.id
}

func (kem *dhKEM) encSize() int {
	return kem.Nenc
}

var dhKEMP256 = &dhKEM{HKDFSHA256(), 0x0010, ecdh.P256(), 32, 32, 65}
var dhKEMP384 = &dhKEM{HKDFSHA384(), 0x0011, ecdh.P384(), 48, 48, 97}
var dhKEMP521 = &dhKEM{HKDFSHA512(), 0x0012, ecdh.P521(), 64, 66, 133}
var dhKEMX25519 = &dhKEM{HKDFSHA256(), 0x0020, ecdh.X25519(), 32, 32, 32}

// DHKEM returns a KEM implementing one of
//
//   - DHKEM(P-256, HKDF-SHA256)
//   - DHKEM(P-384, HKDF-SHA384)
//   - DHKEM(P-521, HKDF-SHA512)
//   - DHKEM(X25519, HKDF-SHA256)
//
// depending on curve.
func DHKEM(curve ecdh.Curve) KEM {
	switch curve {
	case ecdh.P256():
		return dhKEMP256
	case ecdh.P384():
		return dhKEMP384
	case ecdh.P521():
		return dhKEMP521
	case ecdh.X25519():
		return dhKEMX25519
	default:
		// The set of ecdh.Curve implementations is closed, because the
		// interface has unexported methods. Therefore, this default case is
		// only hit if a new curve is added that DHKEM doesn't support.
		return unsupportedCurveKEM{}
	}
}

type unsupportedCurveKEM struct{}

func (unsupportedCurveKEM) ID() uint16 {
	return 0
}
func (unsupportedCurveKEM) GenerateKey() (PrivateKey, error) {
	return nil, errors.New("unsupported curve")
}
func (unsupportedCurveKEM) NewPublicKey([]byte) (PublicKey, error) {
	return nil, errors.New("unsupported curve")
}
func (unsupportedCurveKEM) NewPrivateKey([]byte) (PrivateKey, error) {
	return nil, errors.New("unsupported curve")
}
func (unsupportedCurveKEM) DeriveKeyPair([]byte) (PrivateKey, error) {
	return nil, errors.New("unsupported curve")
}
func (unsupportedCurveKEM) encSize() int {
	return 0
}

type dhKEMPublicKey struct {
	kem *dhKEM
	pub *ecdh.PublicKey
}

// NewDHKEMPublicKey returns a PublicKey implementing
//
//   - DHKEM(P-256, HKDF-SHA256)
//   - DHKEM(P-384, HKDF-SHA384)
//   - DHKEM(P-521, HKDF-SHA512)
//   - DHKEM(X25519, HKDF-SHA256)
//
// depending on the underlying curve of pub ([ecdh.X25519], [ecdh.P256],
// [ecdh.P384], or [ecdh.P521]).
//
// This function is meant for applications that already have an instantiated
// crypto/ecdh public key. Otherwise, applications should use the
// [KEM.NewPublicKey] method of [DHKEM].
func NewDHKEMPublicKey(pub *ecdh.PublicKey) (PublicKey, error) {
	kem, ok := DHKEM(pub.Curve()).(*dhKEM)
	if !ok {
		return nil, errors.New("unsupported curve")
	}
	return &dhKEMPublicKey{
		kem: kem,
		pub: pub,
	}, nil
}

func (kem *dhKEM) NewPublicKey(data []byte) (PublicKey, error) {
	pub, err := kem.curve.NewPublicKey(data)
	if err != nil {
		return nil, err
	}
	return NewDHKEMPublicKey(pub)
}

func (pk *dhKEMPublicKey) KEM() KEM {
	return pk.kem
}

func (pk *dhKEMPublicKey) Bytes() []byte {
	return pk.pub.Bytes()
}

// testingOnlyGenerateKey is only used during testing, to provide
// a fixed test key to use when checking the RFC 9180 vectors.
var testingOnlyGenerateKey func() *ecdh.PrivateKey

func (pk *dhKEMPublicKey) encap() (sharedSecret []byte, encapPub []byte, err error) {
	privEph, err := pk.pub.Curve().GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	if testingOnlyGenerateKey != nil {
		privEph = testingOnlyGenerateKey()
	}
	dhVal, err := privEph.ECDH(pk.pub)
	if err != nil {
		return nil, nil, err
	}
	encPubEph := privEph.PublicKey().Bytes()

	encPubRecip := pk.pub.Bytes()
	kemContext := append(encPubEph, encPubRecip...)
	sharedSecret, err = pk.kem.extractAndExpand(dhVal, kemContext)
	if err != nil {
		return nil, nil, err
	}
	return sharedSecret, encPubEph, nil
}

type dhKEMPrivateKey struct {
	kem  *dhKEM
	priv ecdh.KeyExchanger
}

// NewDHKEMPrivateKey returns a PrivateKey implementing
//
//   - DHKEM(P-256, HKDF-SHA256)
//   - DHKEM(P-384, HKDF-SHA384)
//   - DHKEM(P-521, HKDF-SHA512)
//   - DHKEM(X25519, HKDF-SHA256)
//
// depending on the underlying curve of priv ([ecdh.X25519], [ecdh.P256],
// [ecdh.P384], or [ecdh.P521]).
//
// This function is meant for applications that already have an instantiated
// crypto/ecdh private key, or another implementation of a [ecdh.KeyExchanger]
// (e.g. a hardware key). Otherwise, applications should use the
// [KEM.NewPrivateKey] method of [DHKEM].
func NewDHKEMPrivateKey(priv ecdh.KeyExchanger) (PrivateKey, error) {
	kem, ok := DHKEM(priv.Curve()).(*dhKEM)
	if !ok {
		return nil, errors.New("unsupported curve")
	}
	return &dhKEMPrivateKey{
		kem:  kem,
		priv: priv,
	}, nil
}

func (kem *dhKEM) GenerateKey() (PrivateKey, error) {
	priv, err := kem.curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return NewDHKEMPrivateKey(priv)
}

func (kem *dhKEM) NewPrivateKey(ikm []byte) (PrivateKey, error) {
	priv, err := kem.curve.NewPrivateKey(ikm)
	if err != nil {
		return nil, err
	}
	return NewDHKEMPrivateKey(priv)
}

func (kem *dhKEM) DeriveKeyPair(ikm []byte) (PrivateKey, error) {
	// DeriveKeyPair from RFC 9180 Section 7.1.3.
	suiteID := byteorder.BEAppendUint16([]byte("KEM"), kem.id)
	prk, err := kem.kdf.labeledExtract(suiteID, nil, "dkp_prk", ikm)
	if err != nil {
		return nil, err
	}
	if kem == dhKEMX25519 {
		s, err := kem.kdf.labeledExpand(suiteID, prk, "sk", nil, kem.Nsk)
		if err != nil {
			return nil, err
		}
		return kem.NewPrivateKey(s)
	}
	var counter uint8
	for counter < 4 {
		s, err := kem.kdf.labeledExpand(suiteID, prk, "candidate", []byte{counter}, kem.Nsk)
		if err != nil {
			return nil, err
		}
		if kem == dhKEMP521 {
			s[0] &= 0x01
		}
		r, err := kem.NewPrivateKey(s)
		if err != nil {
			counter++
			continue
		}
		return r, nil
	}
	panic("chance of four rejections is < 2^-128")
}

func (k *dhKEMPrivateKey) KEM() KEM {
	return k.kem
}

func (k *dhKEMPrivateKey) Bytes() ([]byte, error) {
	// Bizarrely, RFC 9180, Section 7.1.2 says SerializePrivateKey MUST clamp
	// the output, which I thought we all agreed to instead do as part of the DH
	// function, letting private keys be random bytes.
	//
	// At the same time, it says DeserializePrivateKey MUST also clamp, implying
	// that the input doesn't have to be clamped, so Bytes by spec doesn't
	// necessarily match the NewPrivateKey input.
	//
	// I'm sure this will not lead to any unexpected behavior or interop issue.
	priv, ok := k.priv.(*ecdh.PrivateKey)
	if !ok {
		return nil, errors.New("ecdh: private key does not support Bytes")
	}
	if k.kem == dhKEMX25519 {
		b := priv.Bytes()
		b[0] &= 248
		b[31] &= 127
		b[31] |= 64
		return b, nil
	}
	return priv.Bytes(), nil
}

func (k *dhKEMPrivateKey) PublicKey() PublicKey {
	return &dhKEMPublicKey{
		kem: k.kem,
		pub: k.priv.PublicKey(),
	}
}

func (k *dhKEMPrivateKey) decap(encPubEph []byte) ([]byte, error) {
	pubEph, err := k.priv.Curve().NewPublicKey(encPubEph)
	if err != nil {
		return nil, err
	}
	dhVal, err := k.priv.ECDH(pubEph)
	if err != nil {
		return nil, err
	}
	kemContext := append(slices.Clip(encPubEph), k.priv.PublicKey().Bytes()...)
	return k.kem.extractAndExpand(dhVal, kemContext)
}

```

// === FILE: references!/go/src/crypto/hpke/pq.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package hpke

import (
	"bytes"
	"crypto"
	"crypto/ecdh"
	"crypto/fips140"
	"crypto/internal/fips140/drbg"
	"crypto/internal/rand"
	"crypto/mlkem"
	"crypto/sha3"
	"errors"
	"internal/byteorder"
)

var mlkem768X25519 = &hybridKEM{
	id: 0x647a,
	label: /**/ `\./` +
		/*   */ `/^\`,
	curve: ecdh.X25519(),

	curveSeedSize:    32,
	curvePointSize:   32,
	pqEncapsKeySize:  mlkem.EncapsulationKeySize768,
	pqCiphertextSize: mlkem.CiphertextSize768,

	pqNewPublicKey: func(data []byte) (crypto.Encapsulator, error) {
		return mlkem.NewEncapsulationKey768(data)
	},
	pqNewPrivateKey: func(data []byte) (crypto.Decapsulator, error) {
		return mlkem.NewDecapsulationKey768(data)
	},
}

// MLKEM768X25519 returns a KEM implementing MLKEM768-X25519 (a.k.a. X-Wing)
// from draft-ietf-hpke-pq.
func MLKEM768X25519() KEM {
	return mlkem768X25519
}

var mlkem768P256 = &hybridKEM{
	id:    0x0050,
	label: "MLKEM768-P256",
	curve: ecdh.P256(),

	curveSeedSize:    32,
	curvePointSize:   65,
	pqEncapsKeySize:  mlkem.EncapsulationKeySize768,
	pqCiphertextSize: mlkem.CiphertextSize768,

	pqNewPublicKey: func(data []byte) (crypto.Encapsulator, error) {
		return mlkem.NewEncapsulationKey768(data)
	},
	pqNewPrivateKey: func(data []byte) (crypto.Decapsulator, error) {
		return mlkem.NewDecapsulationKey768(data)
	},
}

// MLKEM768P256 returns a KEM implementing MLKEM768-P256 from draft-ietf-hpke-pq.
func MLKEM768P256() KEM {
	return mlkem768P256
}

var mlkem1024P384 = &hybridKEM{
	id:    0x0051,
	label: "MLKEM1024-P384",
	curve: ecdh.P384(),

	curveSeedSize:    48,
	curvePointSize:   97,
	pqEncapsKeySize:  mlkem.EncapsulationKeySize1024,
	pqCiphertextSize: mlkem.CiphertextSize1024,

	pqNewPublicKey: func(data []byte) (crypto.Encapsulator, error) {
		return mlkem.NewEncapsulationKey1024(data)
	},
	pqNewPrivateKey: func(data []byte) (crypto.Decapsulator, error) {
		return mlkem.NewDecapsulationKey1024(data)
	},
}

// MLKEM1024P384 returns a KEM implementing MLKEM1024-P384 from draft-ietf-hpke-pq.
func MLKEM1024P384() KEM {
	return mlkem1024P384
}

type hybridKEM struct {
	id    uint16
	label string
	curve ecdh.Curve

	curveSeedSize    int
	curvePointSize   int
	pqEncapsKeySize  int
	pqCiphertextSize int

	pqNewPublicKey  func(data []byte) (crypto.Encapsulator, error)
	pqNewPrivateKey func(data []byte) (crypto.Decapsulator, error)
}

func (kem *hybridKEM) ID() uint16 {
	return kem.id
}

func (kem *hybridKEM) encSize() int {
	return kem.pqCiphertextSize + kem.curvePointSize
}

func (kem *hybridKEM) sharedSecret(ssPQ, ssT, ctT, ekT []byte) []byte {
	h := sha3.New256()
	h.Write(ssPQ)
	h.Write(ssT)
	h.Write(ctT)
	h.Write(ekT)
	h.Write([]byte(kem.label))
	return h.Sum(nil)
}

type hybridPublicKey struct {
	kem *hybridKEM
	t   *ecdh.PublicKey
	pq  crypto.Encapsulator
}

// NewHybridPublicKey returns a PublicKey implementing one of
//
//   - MLKEM768-X25519 (a.k.a. X-Wing)
//   - MLKEM768-P256
//   - MLKEM1024-P384
//
// from draft-ietf-hpke-pq, depending on the underlying curve of t
// ([ecdh.X25519], [ecdh.P256], or [ecdh.P384]) and the type of pq (either
// *[mlkem.EncapsulationKey768] or *[mlkem.EncapsulationKey1024]).
//
// This function is meant for applications that already have instantiated
// crypto/ecdh and crypto/mlkem public keys. Otherwise, applications should use
// the [KEM.NewPublicKey] method of e.g. [MLKEM768X25519].
func NewHybridPublicKey(pq crypto.Encapsulator, t *ecdh.PublicKey) (PublicKey, error) {
	switch t.Curve() {
	case ecdh.X25519():
		if _, ok := pq.(*mlkem.EncapsulationKey768); !ok {
			return nil, errors.New("invalid PQ KEM for X25519 hybrid")
		}
		return &hybridPublicKey{mlkem768X25519, t, pq}, nil
	case ecdh.P256():
		if _, ok := pq.(*mlkem.EncapsulationKey768); !ok {
			return nil, errors.New("invalid PQ KEM for P-256 hybrid")
		}
		return &hybridPublicKey{mlkem768P256, t, pq}, nil
	case ecdh.P384():
		if _, ok := pq.(*mlkem.EncapsulationKey1024); !ok {
			return nil, errors.New("invalid PQ KEM for P-384 hybrid")
		}
		return &hybridPublicKey{mlkem1024P384, t, pq}, nil
	default:
		return nil, errors.New("unsupported curve")
	}
}

func (kem *hybridKEM) NewPublicKey(data []byte) (PublicKey, error) {
	if len(data) != kem.pqEncapsKeySize+kem.curvePointSize {
		return nil, errors.New("invalid public key size")
	}
	pq, err := kem.pqNewPublicKey(data[:kem.pqEncapsKeySize])
	if err != nil {
		return nil, err
	}
	var k *ecdh.PublicKey
	fips140.WithoutEnforcement(func() { // Hybrid of ML-KEM, which is Approved.
		k, err = kem.curve.NewPublicKey(data[kem.pqEncapsKeySize:])
	})
	if err != nil {
		return nil, err
	}
	return NewHybridPublicKey(pq, k)
}

func (pk *hybridPublicKey) KEM() KEM {
	return pk.kem
}

func (pk *hybridPublicKey) Bytes() []byte {
	return append(pk.pq.Bytes(), pk.t.Bytes()...)
}

var testingOnlyEncapsulate func() (ss, ct []byte)

func (pk *hybridPublicKey) encap() (sharedSecret []byte, encapPub []byte, err error) {
	var skE *ecdh.PrivateKey
	fips140.WithoutEnforcement(func() { // Hybrid of ML-KEM, which is Approved.
		skE, err = pk.t.Curve().GenerateKey(rand.Reader)
	})
	if err != nil {
		return nil, nil, err
	}
	if testingOnlyGenerateKey != nil {
		skE = testingOnlyGenerateKey()
	}
	var ssT []byte
	fips140.WithoutEnforcement(func() {
		ssT, err = skE.ECDH(pk.t)
	})
	if err != nil {
		return nil, nil, err
	}
	ctT := skE.PublicKey().Bytes()

	ssPQ, ctPQ := pk.pq.Encapsulate()
	if testingOnlyEncapsulate != nil {
		ssPQ, ctPQ = testingOnlyEncapsulate()
	}

	ss := pk.kem.sharedSecret(ssPQ, ssT, ctT, pk.t.Bytes())
	ct := append(ctPQ, ctT...)
	return ss, ct, nil
}

type hybridPrivateKey struct {
	kem  *hybridKEM
	seed []byte // can be nil
	t    ecdh.KeyExchanger
	pq   crypto.Decapsulator
}

// NewHybridPrivateKey returns a PrivateKey implementing
//
//   - MLKEM768-X25519 (a.k.a. X-Wing)
//   - MLKEM768-P256
//   - MLKEM1024-P384
//
// from draft-ietf-hpke-pq, depending on the underlying curve of t
// ([ecdh.X25519], [ecdh.P256], or [ecdh.P384]) and the type of pq.Encapsulator()
// (either *[mlkem.EncapsulationKey768] or *[mlkem.EncapsulationKey1024]).
//
// This function is meant for applications that already have instantiated
// crypto/ecdh and crypto/mlkem private keys, or another implementation of a
// [ecdh.KeyExchanger] and [crypto.Decapsulator] (e.g. a hardware key).
// Otherwise, applications should use the [KEM.NewPrivateKey] method of e.g.
// [MLKEM768X25519].
func NewHybridPrivateKey(pq crypto.Decapsulator, t ecdh.KeyExchanger) (PrivateKey, error) {
	return newHybridPrivateKey(pq, t, nil)
}

func (kem *hybridKEM) GenerateKey() (PrivateKey, error) {
	seed := make([]byte, 32)
	drbg.Read(seed)
	return kem.NewPrivateKey(seed)
}

func (kem *hybridKEM) NewPrivateKey(priv []byte) (PrivateKey, error) {
	if len(priv) != 32 {
		return nil, errors.New("hpke: invalid hybrid KEM secret length")
	}

	s := sha3.NewSHAKE256()
	s.Write(priv)

	seedPQ := make([]byte, mlkem.SeedSize)
	s.Read(seedPQ)
	pq, err := kem.pqNewPrivateKey(seedPQ)
	if err != nil {
		return nil, err
	}

	seedT := make([]byte, kem.curveSeedSize)
	for {
		s.Read(seedT)
		var k ecdh.KeyExchanger
		fips140.WithoutEnforcement(func() { // Hybrid of ML-KEM, which is Approved.
			k, err = kem.curve.NewPrivateKey(seedT)
		})
		if err != nil {
			continue
		}
		return newHybridPrivateKey(pq, k, priv)
	}
}

func newHybridPrivateKey(pq crypto.Decapsulator, t ecdh.KeyExchanger, seed []byte) (PrivateKey, error) {
	switch t.Curve() {
	case ecdh.X25519():
		if _, ok := pq.Encapsulator().(*mlkem.EncapsulationKey768); !ok {
			return nil, errors.New("invalid PQ KEM for X25519 hybrid")
		}
		return &hybridPrivateKey{mlkem768X25519, bytes.Clone(seed), t, pq}, nil
	case ecdh.P256():
		if _, ok := pq.Encapsulator().(*mlkem.EncapsulationKey768); !ok {
			return nil, errors.New("invalid PQ KEM for P-256 hybrid")
		}
		return &hybridPrivateKey{mlkem768P256, bytes.Clone(seed), t, pq}, nil
	case ecdh.P384():
		if _, ok := pq.Encapsulator().(*mlkem.EncapsulationKey1024); !ok {
			return nil, errors.New("invalid PQ KEM for P-384 hybrid")
		}
		return &hybridPrivateKey{mlkem1024P384, bytes.Clone(seed), t, pq}, nil
	default:
		return nil, errors.New("unsupported curve")
	}
}

func (kem *hybridKEM) DeriveKeyPair(ikm []byte) (PrivateKey, error) {
	suiteID := byteorder.BEAppendUint16([]byte("KEM"), kem.id)
	dk, err := SHAKE256().labeledDerive(suiteID, ikm, "DeriveKeyPair", nil, 32)
	if err != nil {
		return nil, err
	}
	return kem.NewPrivateKey(dk)
}

func (k *hybridPrivateKey) KEM() KEM {
	return k.kem
}

func (k *hybridPrivateKey) Bytes() ([]byte, error) {
	if k.seed == nil {
		return nil, errors.New("private key seed not available")
	}
	return k.seed, nil
}

func (k *hybridPrivateKey) PublicKey() PublicKey {
	return &hybridPublicKey{
		kem: k.kem,
		t:   k.t.PublicKey(),
		pq:  k.pq.Encapsulator(),
	}
}

func (k *hybridPrivateKey) decap(enc []byte) ([]byte, error) {
	if len(enc) != k.kem.pqCiphertextSize+k.kem.curvePointSize {
		return nil, errors.New("invalid encapsulated key size")
	}
	ctPQ, ctT := enc[:k.kem.pqCiphertextSize], enc[k.kem.pqCiphertextSize:]
	ssPQ, err := k.pq.Decapsulate(ctPQ)
	if err != nil {
		return nil, err
	}
	var pub *ecdh.PublicKey
	fips140.WithoutEnforcement(func() { // Hybrid of ML-KEM, which is Approved.
		pub, err = k.t.Curve().NewPublicKey(ctT)
	})
	if err != nil {
		return nil, err
	}
	var ssT []byte
	fips140.WithoutEnforcement(func() {
		ssT, err = k.t.ECDH(pub)
	})
	if err != nil {
		return nil, err
	}
	ss := k.kem.sharedSecret(ssPQ, ssT, ctT, k.t.PublicKey().Bytes())
	return ss, nil
}

var mlkem768 = &mlkemKEM{
	id:             0x0041,
	ciphertextSize: mlkem.CiphertextSize768,
	newPublicKey: func(data []byte) (crypto.Encapsulator, error) {
		return mlkem.NewEncapsulationKey768(data)
	},
	newPrivateKey: func(data []byte) (crypto.Decapsulator, error) {
		return mlkem.NewDecapsulationKey768(data)
	},
	generateKey: func() (crypto.Decapsulator, error) {
		return mlkem.GenerateKey768()
	},
}

// MLKEM768 returns a KEM implementing ML-KEM-768 from draft-ietf-hpke-pq.
func MLKEM768() KEM {
	return mlkem768
}

var mlkem1024 = &mlkemKEM{
	id:             0x0042,
	ciphertextSize: mlkem.CiphertextSize1024,
	newPublicKey: func(data []byte) (crypto.Encapsulator, error) {
		return mlkem.NewEncapsulationKey1024(data)
	},
	newPrivateKey: func(data []byte) (crypto.Decapsulator, error) {
		return mlkem.NewDecapsulationKey1024(data)
	},
	generateKey: func() (crypto.Decapsulator, error) {
		return mlkem.GenerateKey1024()
	},
}

// MLKEM1024 returns a KEM implementing ML-KEM-1024 from draft-ietf-hpke-pq.
func MLKEM1024() KEM {
	return mlkem1024
}

type mlkemKEM struct {
	id             uint16
	ciphertextSize int
	newPublicKey   func(data []byte) (crypto.Encapsulator, error)
	newPrivateKey  func(data []byte) (crypto.Decapsulator, error)
	generateKey    func() (crypto.Decapsulator, error)
}

func (kem *mlkemKEM) ID() uint16 {
	return kem.id
}

func (kem *mlkemKEM) encSize() int {
	return kem.ciphertextSize
}

type mlkemPublicKey struct {
	kem *mlkemKEM
	pq  crypto.Encapsulator
}

// NewMLKEMPublicKey returns a KEMPublicKey implementing
//
//   - ML-KEM-768
//   - ML-KEM-1024
//
// from draft-ietf-hpke-pq, depending on the type of pub
// (*[mlkem.EncapsulationKey768] or *[mlkem.EncapsulationKey1024]).
//
// This function is meant for applications that already have an instantiated
// crypto/mlkem public key. Otherwise, applications should use the
// [KEM.NewPublicKey] method of e.g. [MLKEM768].
func NewMLKEMPublicKey(pub crypto.Encapsulator) (PublicKey, error) {
	switch pub.(type) {
	case *mlkem.EncapsulationKey768:
		return &mlkemPublicKey{mlkem768, pub}, nil
	case *mlkem.EncapsulationKey1024:
		return &mlkemPublicKey{mlkem1024, pub}, nil
	default:
		return nil, errors.New("unsupported public key type")
	}
}

func (kem *mlkemKEM) NewPublicKey(data []byte) (PublicKey, error) {
	pq, err := kem.newPublicKey(data)
	if err != nil {
		return nil, err
	}
	return NewMLKEMPublicKey(pq)
}

func (pk *mlkemPublicKey) KEM() KEM {
	return pk.kem
}

func (pk *mlkemPublicKey) Bytes() []byte {
	return pk.pq.Bytes()
}

func (pk *mlkemPublicKey) encap() (sharedSecret []byte, encapPub []byte, err error) {
	ss, ct := pk.pq.Encapsulate()
	if testingOnlyEncapsulate != nil {
		ss, ct = testingOnlyEncapsulate()
	}
	return ss, ct, nil
}

type mlkemPrivateKey struct {
	kem *mlkemKEM
	pq  crypto.Decapsulator
}

// NewMLKEMPrivateKey returns a KEMPrivateKey implementing
//
//   - ML-KEM-768
//   - ML-KEM-1024
//
// from draft-ietf-hpke-pq, depending on the type of priv.Encapsulator()
// (either *[mlkem.EncapsulationKey768] or *[mlkem.EncapsulationKey1024]).
//
// This function is meant for applications that already have an instantiated
// crypto/mlkem private key. Otherwise, applications should use the
// [KEM.NewPrivateKey] method of e.g. [MLKEM768].
func NewMLKEMPrivateKey(priv crypto.Decapsulator) (PrivateKey, error) {
	switch priv.Encapsulator().(type) {
	case *mlkem.EncapsulationKey768:
		return &mlkemPrivateKey{mlkem768, priv}, nil
	case *mlkem.EncapsulationKey1024:
		return &mlkemPrivateKey{mlkem1024, priv}, nil
	default:
		return nil, errors.New("unsupported public key type")
	}
}

func (kem *mlkemKEM) GenerateKey() (PrivateKey, error) {
	pq, err := kem.generateKey()
	if err != nil {
		return nil, err
	}
	return NewMLKEMPrivateKey(pq)
}

func (kem *mlkemKEM) NewPrivateKey(priv []byte) (PrivateKey, error) {
	pq, err := kem.newPrivateKey(priv)
	if err != nil {
		return nil, err
	}
	return NewMLKEMPrivateKey(pq)
}

func (kem *mlkemKEM) DeriveKeyPair(ikm []byte) (PrivateKey, error) {
	suiteID := byteorder.BEAppendUint16([]byte("KEM"), kem.id)
	dk, err := SHAKE256().labeledDerive(suiteID, ikm, "DeriveKeyPair", nil, 64)
	if err != nil {
		return nil, err
	}
	return kem.NewPrivateKey(dk)
}

func (k *mlkemPrivateKey) KEM() KEM {
	return k.kem
}

func (k *mlkemPrivateKey) Bytes() ([]byte, error) {
	pq, ok := k.pq.(interface {
		Bytes() []byte
	})
	if !ok {
		return nil, errors.New("private key seed not available")
	}
	return pq.Bytes(), nil
}

func (k *mlkemPrivateKey) PublicKey() PublicKey {
	return &mlkemPublicKey{
		kem: k.kem,
		pq:  k.pq.Encapsulator(),
	}
}

func (k *mlkemPrivateKey) decap(enc []byte) ([]byte, error) {
	return k.pq.Decapsulate(enc)
}

```

