# Domain Architecture: crypto/tls

## Layout Topology
```text
crypto/tls/
├── fipsonly
│   └── fipsonly.go
├── internal
│   └── fips140tls
│       └── fipstls.go
├── alert.go
├── auth.go
├── bogo_config.json
├── cache.go
├── cipher_suites.go
├── common.go
├── common_string.go
├── conn.go
├── defaults.go
├── defaults_boring.go
├── defaults_fips140.go
├── ech.go
├── generate_cert.go
├── handshake_client.go
├── handshake_client_tls13.go
├── handshake_messages.go
├── handshake_server.go
├── handshake_server_tls13.go
├── key_agreement.go
├── key_schedule.go
├── prf.go
├── quic.go
├── ticket.go
└── tls.go
```

## Source Stream Aggregation

// === FILE: references/go/src/crypto/tls/alert.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tls

import "strconv"

// An AlertError is a TLS alert.
//
// When using a QUIC transport, QUICConn methods will return an error
// which wraps AlertError rather than sending a TLS alert.
type AlertError uint8

func (e AlertError) Error() string {
	return alert(e).String()
}

type alert uint8

const (
	// alert level
	alertLevelWarning = 1
	alertLevelError   = 2
)

const (
	alertCloseNotify                  alert = 0
	alertUnexpectedMessage            alert = 10
	alertBadRecordMAC                 alert = 20
	alertDecryptionFailed             alert = 21
	alertRecordOverflow               alert = 22
	alertDecompressionFailure         alert = 30
	alertHandshakeFailure             alert = 40
	alertBadCertificate               alert = 42
	alertUnsupportedCertificate       alert = 43
	alertCertificateRevoked           alert = 44
	alertCertificateExpired           alert = 45
	alertCertificateUnknown           alert = 46
	alertIllegalParameter             alert = 47
	alertUnknownCA                    alert = 48
	alertAccessDenied                 alert = 49
	alertDecodeError                  alert = 50
	alertDecryptError                 alert = 51
	alertExportRestriction            alert = 60
	alertProtocolVersion              alert = 70
	alertInsufficientSecurity         alert = 71
	alertInternalError                alert = 80
	alertInappropriateFallback        alert = 86
	alertUserCanceled                 alert = 90
	alertNoRenegotiation              alert = 100
	alertMissingExtension             alert = 109
	alertUnsupportedExtension         alert = 110
	alertCertificateUnobtainable      alert = 111
	alertUnrecognizedName             alert = 112
	alertBadCertificateStatusResponse alert = 113
	alertBadCertificateHashValue      alert = 114
	alertUnknownPSKIdentity           alert = 115
	alertCertificateRequired          alert = 116
	alertNoApplicationProtocol        alert = 120
	alertECHRequired                  alert = 121
)

var alertText = map[alert]string{
	alertCloseNotify:                  "close notify",
	alertUnexpectedMessage:            "unexpected message",
	alertBadRecordMAC:                 "bad record MAC",
	alertDecryptionFailed:             "decryption failed",
	alertRecordOverflow:               "record overflow",
	alertDecompressionFailure:         "decompression failure",
	alertHandshakeFailure:             "handshake failure",
	alertBadCertificate:               "bad certificate",
	alertUnsupportedCertificate:       "unsupported certificate",
	alertCertificateRevoked:           "revoked certificate",
	alertCertificateExpired:           "expired certificate",
	alertCertificateUnknown:           "unknown certificate",
	alertIllegalParameter:             "illegal parameter",
	alertUnknownCA:                    "unknown certificate authority",
	alertAccessDenied:                 "access denied",
	alertDecodeError:                  "error decoding message",
	alertDecryptError:                 "error decrypting message",
	alertExportRestriction:            "export restriction",
	alertProtocolVersion:              "protocol version not supported",
	alertInsufficientSecurity:         "insufficient security level",
	alertInternalError:                "internal error",
	alertInappropriateFallback:        "inappropriate fallback",
	alertUserCanceled:                 "user canceled",
	alertNoRenegotiation:              "no renegotiation",
	alertMissingExtension:             "missing extension",
	alertUnsupportedExtension:         "unsupported extension",
	alertCertificateUnobtainable:      "certificate unobtainable",
	alertUnrecognizedName:             "unrecognized name",
	alertBadCertificateStatusResponse: "bad certificate status response",
	alertBadCertificateHashValue:      "bad certificate hash value",
	alertUnknownPSKIdentity:           "unknown PSK identity",
	alertCertificateRequired:          "certificate required",
	alertNoApplicationProtocol:        "no application protocol",
	alertECHRequired:                  "encrypted client hello required",
}

func (e alert) String() string {
	s, ok := alertText[e]
	if ok {
		return "tls: " + s
	}
	return "tls: alert(" + strconv.Itoa(int(e)) + ")"
}

func (e alert) Error() string {
	return e.String()
}

```

// === FILE: references/go/src/crypto/tls/auth.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tls

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/mldsa"
	"crypto/rsa"
	"errors"
	"fmt"
	"hash"
	"io"
	"slices"
)

// verifyHandshakeSignature verifies a signature against unhashed handshake contents.
func verifyHandshakeSignature(sigType uint8, pubkey crypto.PublicKey, hashFunc crypto.Hash, signed, sig []byte) error {
	if hashFunc != directSigning {
		if !hashFunc.Available() {
			return fmt.Errorf("hash function unavailable: %v", hashFunc)
		}
		h := hashFunc.New()
		h.Write(signed)
		signed = h.Sum(nil)
	}
	switch sigType {
	case signatureECDSA:
		pubKey, ok := pubkey.(*ecdsa.PublicKey)
		if !ok {
			return fmt.Errorf("expected an ECDSA public key, got %T", pubkey)
		}
		if !ecdsa.VerifyASN1(pubKey, signed, sig) {
			return errors.New("ECDSA verification failure")
		}
	case signatureEd25519:
		pubKey, ok := pubkey.(ed25519.PublicKey)
		if !ok {
			return fmt.Errorf("expected an Ed25519 public key, got %T", pubkey)
		}
		if !ed25519.Verify(pubKey, signed, sig) {
			return errors.New("Ed25519 verification failure")
		}
	case signatureMLDSA:
		pubKey, ok := pubkey.(*mldsa.PublicKey)
		if !ok {
			return fmt.Errorf("expected an ML-DSA public key, got %T", pubkey)
		}
		if err := mldsa.Verify(pubKey, signed, sig, nil); err != nil {
			return fmt.Errorf("ML-DSA verification failure: %w", err)
		}
	case signaturePKCS1v15:
		pubKey, ok := pubkey.(*rsa.PublicKey)
		if !ok {
			return fmt.Errorf("expected an RSA public key, got %T", pubkey)
		}
		if err := rsa.VerifyPKCS1v15(pubKey, hashFunc, signed, sig); err != nil {
			return err
		}
	case signatureRSAPSS:
		pubKey, ok := pubkey.(*rsa.PublicKey)
		if !ok {
			return fmt.Errorf("expected an RSA public key, got %T", pubkey)
		}
		signOpts := &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash}
		if err := rsa.VerifyPSS(pubKey, hashFunc, signed, sig, signOpts); err != nil {
			return err
		}
	default:
		return errors.New("internal error: unknown signature type")
	}
	return nil
}

// verifyLegacyHandshakeSignature verifies a TLS 1.0 and 1.1 signature against
// pre-hashed handshake contents.
func verifyLegacyHandshakeSignature(sigType uint8, pubkey crypto.PublicKey, hashFunc crypto.Hash, hashed, sig []byte) error {
	switch sigType {
	case signatureECDSA:
		pubKey, ok := pubkey.(*ecdsa.PublicKey)
		if !ok {
			return fmt.Errorf("expected an ECDSA public key, got %T", pubkey)
		}
		if !ecdsa.VerifyASN1(pubKey, hashed, sig) {
			return errors.New("ECDSA verification failure")
		}
	case signaturePKCS1v15:
		pubKey, ok := pubkey.(*rsa.PublicKey)
		if !ok {
			return fmt.Errorf("expected an RSA public key, got %T", pubkey)
		}
		if err := rsa.VerifyPKCS1v15(pubKey, hashFunc, hashed, sig); err != nil {
			return err
		}
	default:
		return errors.New("internal error: unknown signature type")
	}
	return nil
}

const (
	serverSignatureContext = "TLS 1.3, server CertificateVerify\x00"
	clientSignatureContext = "TLS 1.3, client CertificateVerify\x00"
)

var signaturePadding = []byte{
	0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
	0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
	0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
	0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
	0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
	0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
	0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
	0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
}

// signedMessage returns the (unhashed) message to be signed by certificate keys
// in TLS 1.3. See RFC 8446, Section 4.4.3.
func signedMessage(context string, transcript hash.Hash) []byte {
	const maxSize = 64 /* signaturePadding */ + len(serverSignatureContext) + 512/8 /* SHA-512 */
	b := bytes.NewBuffer(make([]byte, 0, maxSize))
	b.Write(signaturePadding)
	io.WriteString(b, context)
	b.Write(transcript.Sum(nil))
	return b.Bytes()
}

// typeAndHashFromSignatureScheme returns the corresponding signature type and
// crypto.Hash for a given TLS SignatureScheme.
func typeAndHashFromSignatureScheme(signatureAlgorithm SignatureScheme) (sigType uint8, hash crypto.Hash, err error) {
	switch signatureAlgorithm {
	case PKCS1WithSHA1, PKCS1WithSHA256, PKCS1WithSHA384, PKCS1WithSHA512:
		sigType = signaturePKCS1v15
	case PSSWithSHA256, PSSWithSHA384, PSSWithSHA512:
		sigType = signatureRSAPSS
	case ECDSAWithSHA1, ECDSAWithP256AndSHA256, ECDSAWithP384AndSHA384, ECDSAWithP521AndSHA512:
		sigType = signatureECDSA
	case Ed25519:
		sigType = signatureEd25519
	case MLDSA44, MLDSA65, MLDSA87:
		sigType = signatureMLDSA
	default:
		return 0, 0, fmt.Errorf("unsupported signature algorithm: %v", signatureAlgorithm)
	}
	switch signatureAlgorithm {
	case PKCS1WithSHA1, ECDSAWithSHA1:
		hash = crypto.SHA1
	case PKCS1WithSHA256, PSSWithSHA256, ECDSAWithP256AndSHA256:
		hash = crypto.SHA256
	case PKCS1WithSHA384, PSSWithSHA384, ECDSAWithP384AndSHA384:
		hash = crypto.SHA384
	case PKCS1WithSHA512, PSSWithSHA512, ECDSAWithP521AndSHA512:
		hash = crypto.SHA512
	case Ed25519:
		hash = directSigning
	case MLDSA44, MLDSA65, MLDSA87:
		hash = directSigning
	default:
		return 0, 0, fmt.Errorf("unsupported signature algorithm: %v", signatureAlgorithm)
	}
	return sigType, hash, nil
}

// legacyTypeAndHashFromPublicKey returns the fixed signature type and crypto.Hash for
// a given public key used with TLS 1.0 and 1.1, before the introduction of
// signature algorithm negotiation.
func legacyTypeAndHashFromPublicKey(pub crypto.PublicKey) (sigType uint8, hash crypto.Hash, err error) {
	switch pub.(type) {
	case *rsa.PublicKey:
		return signaturePKCS1v15, crypto.MD5SHA1, nil
	case *ecdsa.PublicKey:
		return signatureECDSA, crypto.SHA1, nil
	case ed25519.PublicKey:
		// RFC 8422 specifies support for Ed25519 in TLS 1.0 and 1.1,
		// but it requires holding on to a handshake transcript to do a
		// full signature, and not even OpenSSL bothers with the
		// complexity, so we can't even test it properly.
		return 0, 0, fmt.Errorf("tls: Ed25519 public keys are not supported before TLS 1.2")
	case *mldsa.PublicKey:
		return 0, 0, fmt.Errorf("tls: ML-DSA public keys are not supported before TLS 1.3")
	default:
		return 0, 0, fmt.Errorf("tls: unsupported public key: %T", pub)
	}
}

var rsaSignatureSchemes = []struct {
	scheme          SignatureScheme
	minModulusBytes int
}{
	// RSA-PSS is used with PSSSaltLengthEqualsHash, and requires
	//    emLen >= hLen + sLen + 2
	{PSSWithSHA256, crypto.SHA256.Size()*2 + 2},
	{PSSWithSHA384, crypto.SHA384.Size()*2 + 2},
	{PSSWithSHA512, crypto.SHA512.Size()*2 + 2},
	// PKCS #1 v1.5 uses prefixes from hashPrefixes in crypto/rsa, and requires
	//    emLen >= len(prefix) + hLen + 11
	{PKCS1WithSHA256, 19 + crypto.SHA256.Size() + 11},
	{PKCS1WithSHA384, 19 + crypto.SHA384.Size() + 11},
	{PKCS1WithSHA512, 19 + crypto.SHA512.Size() + 11},
	{PKCS1WithSHA1, 15 + crypto.SHA1.Size() + 11},
}

func signatureSchemesForPublicKey(version uint16, pub crypto.PublicKey) []SignatureScheme {
	switch pub := pub.(type) {
	case *ecdsa.PublicKey:
		if version < VersionTLS13 {
			// In TLS 1.2 and earlier, ECDSA algorithms are not
			// constrained to a single curve.
			return []SignatureScheme{
				ECDSAWithP256AndSHA256,
				ECDSAWithP384AndSHA384,
				ECDSAWithP521AndSHA512,
				ECDSAWithSHA1,
			}
		}
		switch pub.Curve {
		case elliptic.P256():
			return []SignatureScheme{ECDSAWithP256AndSHA256}
		case elliptic.P384():
			return []SignatureScheme{ECDSAWithP384AndSHA384}
		case elliptic.P521():
			return []SignatureScheme{ECDSAWithP521AndSHA512}
		default:
			return nil
		}
	case *rsa.PublicKey:
		size := pub.Size()
		sigAlgs := make([]SignatureScheme, 0, len(rsaSignatureSchemes))
		for _, candidate := range rsaSignatureSchemes {
			if size >= candidate.minModulusBytes {
				sigAlgs = append(sigAlgs, candidate.scheme)
			}
		}
		return sigAlgs
	case ed25519.PublicKey:
		return []SignatureScheme{Ed25519}
	case *mldsa.PublicKey:
		switch pub.Parameters() {
		case mldsa.MLDSA44():
			return []SignatureScheme{MLDSA44}
		case mldsa.MLDSA65():
			return []SignatureScheme{MLDSA65}
		case mldsa.MLDSA87():
			return []SignatureScheme{MLDSA87}
		default:
			panic("tls: internal error: unknown ML-DSA parameter set: " + pub.Parameters().String())
		}
	default:
		return nil
	}
}

// selectSignatureScheme picks a SignatureScheme from the peer's preference list
// that works with the selected certificate. It's only called for protocol
// versions that support signature algorithms, so TLS 1.2 and 1.3.
func selectSignatureScheme(vers uint16, c *Certificate, peerAlgs []SignatureScheme) (SignatureScheme, error) {
	priv, ok := c.PrivateKey.(crypto.Signer)
	if !ok {
		return 0, unsupportedCertificateError(c)
	}
	supportedAlgs := signatureSchemesForPublicKey(vers, priv.Public())
	if c.SupportedSignatureAlgorithms != nil {
		supportedAlgs = slices.DeleteFunc(supportedAlgs, func(sigAlg SignatureScheme) bool {
			return !isSupportedSignatureAlgorithm(sigAlg, c.SupportedSignatureAlgorithms)
		})
	}
	// Filter out any unsupported signature algorithms, for example due to
	// FIPS 140-3 policy, tlssha1=0, or protocol version.
	supportedAlgs = slices.DeleteFunc(supportedAlgs, func(sigAlg SignatureScheme) bool {
		return isDisabledSignatureAlgorithm(vers, sigAlg, false)
	})
	if len(supportedAlgs) == 0 {
		return 0, unsupportedCertificateError(c)
	}
	if len(peerAlgs) == 0 && vers == VersionTLS12 {
		// For TLS 1.2, if the client didn't send signature_algorithms then we
		// can assume that it supports SHA1. See RFC 5246, Section 7.4.1.4.1.
		// RFC 9155 made signature_algorithms mandatory in TLS 1.2, and we gated
		// it behind the tlssha1 GODEBUG setting.
		if tlssha1.Value() != "1" {
			return 0, errors.New("tls: missing signature_algorithms from TLS 1.2 peer")
		}
		peerAlgs = []SignatureScheme{PKCS1WithSHA1, ECDSAWithSHA1}
	}
	// Pick signature scheme in the peer's preference order, as our
	// preference order is not configurable.
	for _, preferredAlg := range peerAlgs {
		if isSupportedSignatureAlgorithm(preferredAlg, supportedAlgs) {
			return preferredAlg, nil
		}
	}
	return 0, errors.New("tls: peer doesn't support any of the certificate's signature algorithms")
}

// unsupportedCertificateError returns a helpful error for certificates with
// an unsupported private key.
func unsupportedCertificateError(cert *Certificate) error {
	switch cert.PrivateKey.(type) {
	case rsa.PrivateKey, ecdsa.PrivateKey:
		return fmt.Errorf("tls: unsupported certificate: private key is %T, expected *%T",
			cert.PrivateKey, cert.PrivateKey)
	case *ed25519.PrivateKey:
		return fmt.Errorf("tls: unsupported certificate: private key is *ed25519.PrivateKey, expected ed25519.PrivateKey")
	}

	signer, ok := cert.PrivateKey.(crypto.Signer)
	if !ok {
		return fmt.Errorf("tls: certificate private key (%T) does not implement crypto.Signer",
			cert.PrivateKey)
	}

	switch pub := signer.Public().(type) {
	case *ecdsa.PublicKey:
		switch pub.Curve {
		case elliptic.P256():
		case elliptic.P384():
		case elliptic.P521():
		default:
			return fmt.Errorf("tls: unsupported certificate curve (%s)", pub.Curve.Params().Name)
		}
	case *rsa.PublicKey:
		return fmt.Errorf("tls: certificate RSA key size too small for supported signature algorithms")
	case ed25519.PublicKey:
	case *mldsa.PublicKey:
		return errors.New("tls: ML-DSA certificates require TLS 1.3")
	default:
		return fmt.Errorf("tls: unsupported certificate key (%T)", pub)
	}

	if cert.SupportedSignatureAlgorithms != nil {
		return fmt.Errorf("tls: peer doesn't support the certificate custom signature algorithms")
	}

	return fmt.Errorf("tls: internal error: unsupported key (%T)", cert.PrivateKey)
}

```

// === FILE: references/go/src/crypto/tls/bogo_config.json ===
```text
{
    "DisabledTests": {
        "*-Async": "We don't support boringssl concept of async",

        "TLS-ECH-Client-Reject-NoClientCertificate-TLS12": "We won't attempt to negotiate 1.2 if ECH is enabled",
        "TLS-ECH-Client-Reject-TLS12": "We won't attempt to negotiate 1.2 if ECH is enabled",
        "TLS-ECH-Client-TLS12-RejectRetryConfigs": "We won't attempt to negotiate 1.2 if ECH is enabled",
        "TLS-ECH-Client-Rejected-OverrideName-TLS12": "We won't attempt to negotiate 1.2 if ECH is enabled",
        "TLS-ECH-Client-Reject-TLS12-NoFalseStart": "We won't attempt to negotiate 1.2 if ECH is enabled",
        "TLS-ECH-Client-TLS12SessionTicket": "We won't attempt to negotiate 1.2 if ECH is enabled",
        "TLS-ECH-Client-TLS12SessionID": "We won't attempt to negotiate 1.2 if ECH is enabled, and we don't support session ID resumption",

        "TLS-ECH-Client-Reject-ResumeInnerSession-TLS12": "We won't attempt to negotiate 1.2 if ECH is enabled (we could possibly test this if we had the ability to indicate not to send ECH on resumption?)",

        "TLS-ECH-Client-Reject-EarlyDataRejected": "Go does not support early (0-RTT) data",

        "TLS-ECH-Client-NoNPN": "We don't support NPN",

        "TLS-ECH-Client-ChannelID": "We don't support sending channel ID",
        "TLS-ECH-Client-Reject-NoChannelID-TLS13": "We don't support sending channel ID",
        "TLS-ECH-Client-Reject-NoChannelID-TLS12": "We don't support sending channel ID",

        "TLS-ECH-Client-GREASE-IgnoreHRRExtension": "We don't support ECH GREASE because we don't fallback to plaintext",
        "TLS-ECH-Client-NoSupportedConfigs-GREASE": "We don't support ECH GREASE because we don't fallback to plaintext",
        "TLS-ECH-Client-GREASEExtensions": "We don't support ECH GREASE because we don't fallback to plaintext",
        "TLS-ECH-Client-GREASE-NoOverrideName": "We don't support ECH GREASE because we don't fallback to plaintext",

        "TLS-ECH-Client-UnsolicitedInnerServerNameAck": "We don't allow sending empty SNI without skipping certificate verification, TODO: could add special flag to bogo to indicate 'empty sni'",

        "TLS-ECH-Client-NoSupportedConfigs": "We don't support fallback to cleartext when there are no valid ECH configs",
        "TLS-ECH-Client-SkipInvalidPublicName": "We don't support fallback to cleartext when there are no valid ECH configs",

        "TLS-ECH-Server-EarlyData": "Go does not support early (0-RTT) data",
        "TLS-ECH-Server-EarlyDataRejected": "Go does not support early (0-RTT) data",

        "MLKEMKeyShareIncludedSecond": "BoGo wants us to order the key shares based on its preference, but we don't support that",
        "MLKEMKeyShareIncludedSecond-*": "BoGo wants us to order the key shares based on its preference, but we don't support that",
        "MLKEMKeyShareIncludedThird": "BoGo wants us to order the key shares based on its preference, but we don't support that",
        "MLKEMKeyShareIncludedThird-*": "BoGo wants us to order the key shares based on its preference, but we don't support that",
        "TwoMLKEMs": "BoGo wants us to order the key shares based on its preference, but we don't support that",
        "NotJustMLKEMKeyShare-MLKEM1024": "BoringSSL sends an ECC key share for fallback when the main key share is MLKEM1024, we currently don't",

        "PostQuantumNotEnabledByDefaultInClients": "We do enable it by default!",
        "*-Kyber-TLS13": "We don't support Kyber, only ML-KEM (BoGo bug ignoring AllCurves?)",

        "*-RSA_PKCS1_SHA256_LEGACY-TLS13": "We don't support the legacy PKCS#1 v1.5 codepoint for TLS 1.3",
        "*-Verify-RSA_PKCS1_SHA256_LEGACY-TLS12": "Likewise, we don't know how to handle it in TLS 1.2, so we send the wrong alert",
        "*-VerifyDefault-*": "Our signature algorithms are not configurable, so there is no difference between default and supported",
        "Ed25519DefaultDisable-*": "We support Ed25519 by default",
        "NoCommonSignatureAlgorithms-TLS12-Fallback": "We don't support the legacy RSA exchange",

        "*_SHA1-TLS12": "We don't support SHA-1 in TLS 1.2 (without tlssha1=1)",
        "Agree-Digest-SHA1": "We don't support SHA-1 in TLS 1.2 (without tlssha1=1)",
        "ServerAuth-SHA1-Fallback*": "We don't support SHA-1 in TLS 1.2 (without tlssha1=1), so we fail if there are no signature_algorithms",

        "Agree-Digest-SHA256": "We select signature algorithms in peer preference order. We should consider changing this.",

        "V2ClientHello-*": "We don't support SSLv2",
        "SendV2ClientHello*": "We don't support SSLv2",
        "*QUIC*": "No QUIC support",
        "Compliance-fips*": "No FIPS",
        "*DTLS*": "No DTLS",
        "SendEmptyRecords*": "crypto/tls doesn't implement spam protections",
        "SendWarningAlerts*": "crypto/tls doesn't implement spam protections",
        "SendUserCanceledAlerts-TooMany-TLS13": "crypto/tls doesn't implement spam protections",
        "TooManyKeyUpdates": "crypto/tls doesn't implement spam protections (TODO: I think?)",
        "KyberNotEnabledByDefaultInClients": "crypto/tls intentionally enables it",
        "JustConfiguringKyberWorks": "we always send a X25519 key share with Kyber",
        "KyberKeyShareIncludedSecond": "we always send the Kyber key share first",
        "KyberKeyShareIncludedThird": "we always send the Kyber key share first",
        "GREASE-Server-TLS13": "We don't send GREASE extensions",
        "SendBogusAlertType": "sending wrong alert type",
        "*Client-P-224*": "no P-224 support",
        "*Server-P-224*": "no P-224 support",
        "CurveID-Resume*": "unexposed curveID is not stored in the ticket yet",
        "BadRSAClientKeyExchange-4": "crypto/tls doesn't check the version number in the premaster secret - see processClientKeyExchange comment",
        "BadRSAClientKeyExchange-5": "crypto/tls doesn't check the version number in the premaster secret - see processClientKeyExchange comment",
        "SupportTicketsWithSessionID": "We don't support session ID resumption",
        "ResumeTLS12SessionID-TLS13": "We don't support session ID resumption",
        "TrustAnchors-*": "We don't support draft-beck-tls-trust-anchor-ids",
        "PAKE-Extension-*": "We don't support PAKE",
        "*TicketFlags": "We don't support draft-ietf-tls-tlsflags",

        "MLKEMKeyShareIncludedThird-X25519MLKEM768": "We don't return key shares in client preference order",

        "ECDSAKeyUsage-*": "We don't enforce ECDSA KU",

        "RSAKeyUsage-*": "We don't enforce RSA KU",

        "CheckLeafCurve": "TODO: first pass, this should be fixed",
        "KeyUpdate-RequestACK": "TODO: first pass, this should be fixed",
        "SupportedVersionSelection-TLS12": "TODO: first pass, this should be fixed",
        "UnsolicitedServerNameAck-TLS-TLS1": "TODO: first pass, this should be fixed",
        "TicketSessionIDLength-33-TLS-TLS1": "TODO: first pass, this should be fixed",
        "UnsolicitedServerNameAck-TLS-TLS11": "TODO: first pass, this should be fixed",
        "TicketSessionIDLength-33-TLS-TLS11": "TODO: first pass, this should be fixed",
        "UnsolicitedServerNameAck-TLS-TLS12": "TODO: first pass, this should be fixed",
        "TicketSessionIDLength-33-TLS-TLS12": "TODO: first pass, this should be fixed",
        "UnsolicitedServerNameAck-TLS-TLS13": "TODO: first pass, this should be fixed",
        "RenegotiationInfo-Forbidden-TLS13": "TODO: first pass, this should be fixed",
        "EMS-Forbidden-TLS13": "TODO: first pass, this should be fixed",
        "SendUnsolicitedOCSPOnCertificate-TLS13": "TODO: first pass, this should be fixed",
        "SendUnsolicitedSCTOnCertificate-TLS13": "TODO: first pass, this should be fixed",
        "SendUnknownExtensionOnCertificate-TLS13": "TODO: first pass, this should be fixed",
        "Resume-Server-NoTickets-TLS1-TLS1-TLS": "TODO: first pass, this should be fixed",
        "Resume-Server-NoTickets-TLS11-TLS11-TLS": "TODO: first pass, this should be fixed",
        "Resume-Server-NoTickets-TLS12-TLS12-TLS": "TODO: first pass, this should be fixed",
        "Resume-Server-NoPSKBinder": "TODO: first pass, this should be fixed",
        "Resume-Server-PSKBinderFirstExtension": "TODO: first pass, this should be fixed",
        "Resume-Server-PSKBinderFirstExtension-SecondBinder": "TODO: first pass, this should be fixed",
        "Resume-Server-NoPSKBinder-SecondBinder": "TODO: first pass, this should be fixed",
        "Resume-Server-OmitPSKsOnSecondClientHello": "TODO: first pass, this should be fixed",
        "Renegotiate-Server-Forbidden": "TODO: first pass, this should be fixed",
        "Renegotiate-Client-Forbidden-1": "TODO: first pass, this should be fixed",
        "UnknownExtension-Client": "TODO: first pass, this should be fixed",
        "UnknownUnencryptedExtension-Client-TLS13": "TODO: first pass, this should be fixed",
        "UnofferedExtension-Client-TLS13": "TODO: first pass, this should be fixed",
        "UnknownExtension-Client-TLS13": "TODO: first pass, this should be fixed",
        "SendClientVersion-RSA": "TODO: first pass, this should be fixed",
        "NoCommonCurves": "TODO: first pass, this should be fixed",
        "PointFormat-EncryptedExtensions-TLS13": "TODO: first pass, this should be fixed",
        "TLS13-SendNoKEMModesWithPSK-Server": "TODO: first pass, this should be fixed",
        "TLS13-DuplicateTicketEarlyDataSupport": "TODO: first pass, this should be fixed",
        "Basic-Client-NoTicket-TLS-Sync": "TODO: first pass, this should be fixed",
        "Basic-Server-RSA-TLS-Sync": "TODO: first pass, this should be fixed",
        "Basic-Client-NoTicket-TLS-Sync-SplitHandshakeRecords": "TODO: first pass, this should be fixed",
        "Basic-Server-RSA-TLS-Sync-SplitHandshakeRecords": "TODO: first pass, this should be fixed",
        "Basic-Client-NoTicket-TLS-Sync-PackHandshake": "TODO: first pass, this should be fixed",
        "Basic-Server-RSA-TLS-Sync-PackHandshake": "TODO: first pass, this should be fixed",
        "PartialSecondClientHelloAfterFirst": "TODO: first pass, this should be fixed",
        "PartialServerHelloWithHelloRetryRequest": "TODO: first pass, this should be fixed",
        "TrailingDataWithFinished-Server-TLS1": "TODO: first pass, this should be fixed",
        "PartialClientKeyExchangeWithClientHello": "TODO: first pass, this should be fixed",
        "TrailingDataWithFinished-Resume-Server-TLS1": "TODO: first pass, this should be fixed",
        "TrailingDataWithFinished-Resume-Client-TLS11": "TODO: first pass, this should be fixed",
        "TrailingDataWithFinished-Client-TLS1": "TODO: first pass, this should be fixed",
        "TrailingDataWithFinished-Client-TLS11": "TODO: first pass, this should be fixed",
        "TrailingDataWithFinished-Client-TLS12": "TODO: first pass, this should be fixed",
        "TrailingDataWithFinished-Client-TLS13": "TODO: first pass, this should be fixed",
        "PartialNewSessionTicketWithServerHelloDone": "TODO: first pass, this should be fixed",
        "TrailingDataWithFinished-Server-TLS11": "TODO: first pass, this should be fixed",
        "TrailingDataWithFinished-Server-TLS12": "TODO: first pass, this should be fixed",
        "TrailingDataWithFinished-Resume-Server-TLS11": "TODO: first pass, this should be fixed",
        "TrailingDataWithFinished-Resume-Client-TLS12": "TODO: first pass, this should be fixed",
        "TrailingDataWithFinished-Resume-Server-TLS12": "TODO: first pass, this should be fixed",
        "TrailingDataWithFinished-Resume-Client-TLS13": "TODO: first pass, this should be fixed",
        "TrailingDataWithFinished-Resume-Client-TLS1": "TODO: first pass, this should be fixed",
        "TrailingMessageData-ClientHello-TLS": "TODO: first pass, this should be fixed",
        "TrailingMessageData-ServerHello-TLS": "TODO: first pass, this should be fixed",
        "TrailingMessageData-ServerCertificate-TLS": "TODO: first pass, this should be fixed",
        "TrailingMessageData-ServerHelloDone-TLS": "TODO: first pass, this should be fixed",
        "TrailingMessageData-ServerKeyExchange-TLS": "TODO: first pass, this should be fixed",
        "TrailingMessageData-CertificateRequest-TLS": "TODO: first pass, this should be fixed",
        "TrailingMessageData-CertificateVerify-TLS": "TODO: first pass, this should be fixed",
        "TrailingMessageData-ServerFinished-TLS": "TODO: first pass, this should be fixed",
        "TrailingMessageData-ClientKeyExchange-TLS": "TODO: first pass, this should be fixed",
        "TrailingMessageData-TLS13-ClientHello-TLS": "TODO: first pass, this should be fixed",
        "TrailingMessageData-ClientFinished-TLS": "TODO: first pass, this should be fixed",
        "TrailingMessageData-NewSessionTicket-TLS": "TODO: first pass, this should be fixed",
        "TrailingMessageData-ClientCertificate-TLS": "TODO: first pass, this should be fixed",
        "TrailingMessageData-TLS13-CertificateRequest-TLS": "TODO: first pass, this should be fixed",
        "TrailingMessageData-TLS13-ServerCertificateVerify-TLS": "TODO: first pass, this should be fixed",
        "TrailingMessageData-TLS13-EncryptedExtensions-TLS": "TODO: first pass, this should be fixed",
        "TrailingMessageData-TLS13-ClientCertificate-TLS": "TODO: first pass, this should be fixed",
        "TrailingMessageData-TLS13-ClientCertificateVerify-TLS": "TODO: first pass, this should be fixed",
        "TrailingMessageData-TLS13-ServerCertificate-TLS": "TODO: first pass, this should be fixed",
        "SkipEarlyData-TLS13": "TODO: first pass, this should be fixed",
        "DuplicateKeyShares-TLS13": "TODO: first pass, this should be fixed",
        "Server-TooLongSessionID-TLS13": "TODO: first pass, this should be fixed",
        "Client-TooLongSessionID": "TODO: first pass, this should be fixed",
        "Client-ShortSessionID": "TODO: first pass, this should be fixed",
        "TLS12NoSessionID-TLS13": "TODO: first pass, this should be fixed",
        "Server-TooLongSessionID-TLS12": "TODO: first pass, this should be fixed",
        "EmptyEncryptedExtensions-TLS13": "TODO: first pass, this should be fixed",
        "SkipEarlyData-SecondClientHelloEarlyData-TLS13": "TODO: first pass, this should be fixed",
        "EncryptedExtensionsWithKeyShare-TLS13": "TODO: first pass, this should be fixed",
        "HelloRetryRequest-DuplicateCurve-TLS13": "TODO: first pass, this should be fixed",
        "HelloRetryRequest-DuplicateCookie-TLS13": "TODO: first pass, this should be fixed",
        "HelloRetryRequest-Unknown-TLS13": "TODO: first pass, this should be fixed",
        "SendPostHandshakeChangeCipherSpec-TLS13": "TODO: first pass, this should be fixed",
        "EmptyExtensions-ClientHello-TLS1": "TODO: first pass, this should be fixed",
        "OmitExtensions-ClientHello-TLS1": "TODO: first pass, this should be fixed",
        "EmptyExtensions-ClientHello-TLS12": "TODO: first pass, this should be fixed",
        "OmitExtensions-ClientHello-TLS12": "TODO: first pass, this should be fixed",
        "EmptyExtensions-ClientHello-TLS11": "TODO: first pass, this should be fixed",
        "OmitExtensions-ClientHello-TLS11": "TODO: first pass, this should be fixed",
        "DuplicateCertCompressionExt-TLS12": "TODO: first pass, this should be fixed",
        "DuplicateCertCompressionExt-TLS13": "TODO: first pass, this should be fixed",
        "Client-RejectJDK11DowngradeRandom": "TODO: first pass, this should be fixed",
        "CheckClientCertificateTypes": "TODO: first pass, this should be fixed",
        "CheckECDSACurve-TLS12": "TODO: first pass, this should be fixed",
        "ALPNClient-RejectUnknown-TLS-TLS1": "TODO: first pass, this should be fixed",
        "ALPNClient-RejectUnknown-TLS-TLS11": "TODO: first pass, this should be fixed",
        "ALPNClient-RejectUnknown-TLS-TLS12": "TODO: first pass, this should be fixed",
        "ALPNClient-RejectUnknown-TLS-TLS13": "TODO: first pass, this should be fixed",
        "ClientHelloPadding": "TODO: first pass, this should be fixed",
        "TLS13-ExpectTicketEarlyDataSupport": "TODO: first pass, this should be fixed",
        "TLS13-EarlyData-TooMuchData-Client-TLS-Sync": "TODO: first pass, this should be fixed",
        "TLS13-EarlyData-TooMuchData-Client-TLS-Sync-SplitHandshakeRecords": "TODO: first pass, this should be fixed",
        "TLS13-EarlyData-TooMuchData-Client-TLS-Sync-PackHandshake": "TODO: first pass, this should be fixed",
        "WrongMessageType-TLS13-EndOfEarlyData-TLS": "TODO: first pass, this should be fixed",
        "TrailingMessageData-TLS13-EndOfEarlyData-TLS": "TODO: first pass, this should be fixed",
        "SendHelloRetryRequest-2-TLS13": "TODO: first pass, this should be fixed",
        "EarlyData-SkipEndOfEarlyData-TLS13": "TODO: first pass, this should be fixed",
        "EarlyData-Server-BadFinished-TLS13": "TODO: first pass, this should be fixed",
        "EarlyData-UnexpectedHandshake-Server-TLS13": "TODO: first pass, this should be fixed",
        "EarlyData-CipherMismatch-Client-TLS13": "TODO: first pass, this should be fixed",

        "Resume-Server-UnofferedCipher-TLS13": "TODO: first pass, this should be fixed",
        "GarbageCertificate-Server-TLS13": "TODO: 2025/06 BoGo update, should be fixed",
        "WrongMessageType-TLS13-ClientCertificate-TLS": "TODO: 2025/06  BoGo update, should be fixed",
        "KeyUpdate-Requested": "TODO: 2025/06  BoGo update, should be fixed",
        "AppDataBeforeTLS13KeyChange-*": "TODO: 2025/06  BoGo update, should be fixed"
    },
    "ErrorMap": {
        ":ECH_REJECTED:": ["tls: server rejected ECH"]
    }
}

```

// === FILE: references/go/src/crypto/tls/cache.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tls

import (
	"crypto/x509"
	"runtime"
	"sync"
	"weak"
)

// weakCertCache provides a cache of *x509.Certificates, allowing multiple
// connections to reuse parsed certificates, instead of re-parsing the
// certificate for every connection, which is an expensive operation.
type weakCertCache struct{ sync.Map }

func (wcc *weakCertCache) newCert(der []byte) (*x509.Certificate, error) {
	if entry, ok := wcc.Load(string(der)); ok {
		if v := entry.(weak.Pointer[x509.Certificate]).Value(); v != nil {
			return v, nil
		}
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}

	wp := weak.Make(cert)
	if entry, loaded := wcc.LoadOrStore(string(der), wp); !loaded {
		runtime.AddCleanup(cert, func(_ any) { wcc.CompareAndDelete(string(der), entry) }, any(string(der)))
	} else if v := entry.(weak.Pointer[x509.Certificate]).Value(); v != nil {
		return v, nil
	} else {
		if wcc.CompareAndSwap(string(der), entry, wp) {
			runtime.AddCleanup(cert, func(_ any) { wcc.CompareAndDelete(string(der), wp) }, any(string(der)))
		}
	}
	return cert, nil
}

var globalCertCache = new(weakCertCache)

```

// === FILE: references/go/src/crypto/tls/cipher_suites.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tls

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"crypto/hmac"
	"crypto/internal/boring"
	fipsaes "crypto/internal/fips140/aes"
	"crypto/internal/fips140/aes/gcm"
	"crypto/rc4"
	"crypto/sha1"
	"crypto/sha256"
	_ "crypto/sha512" // for crypto.SHA384
	"fmt"
	"hash"
	"internal/cpu"
	"runtime"
	_ "unsafe" // for linkname

	"golang.org/x/crypto/chacha20poly1305"
)

// CipherSuite is a TLS cipher suite. Note that most functions in this package
// accept and expose cipher suite IDs instead of this type.
type CipherSuite struct {
	ID   uint16
	Name string

	// Supported versions is the list of TLS protocol versions that can
	// negotiate this cipher suite.
	SupportedVersions []uint16

	// Insecure is true if the cipher suite has known security issues
	// due to its primitives, design, or implementation.
	Insecure bool
}

var (
	supportedUpToTLS12 = []uint16{VersionTLS10, VersionTLS11, VersionTLS12}
	supportedOnlyTLS12 = []uint16{VersionTLS12}
	supportedOnlyTLS13 = []uint16{VersionTLS13}
)

// CipherSuites returns a list of cipher suites currently implemented by this
// package, excluding those with security issues, which are returned by
// [InsecureCipherSuites].
//
// The list is sorted by ID. Note that the default cipher suites selected by
// this package might depend on logic that can't be captured by a static list,
// and might not match those returned by this function.
func CipherSuites() []*CipherSuite {
	return []*CipherSuite{
		{TLS_AES_128_GCM_SHA256, "TLS_AES_128_GCM_SHA256", supportedOnlyTLS13, false},
		{TLS_AES_256_GCM_SHA384, "TLS_AES_256_GCM_SHA384", supportedOnlyTLS13, false},
		{TLS_CHACHA20_POLY1305_SHA256, "TLS_CHACHA20_POLY1305_SHA256", supportedOnlyTLS13, false},

		{TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA, "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA", supportedUpToTLS12, false},
		{TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA, "TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA", supportedUpToTLS12, false},
		{TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA, "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA", supportedUpToTLS12, false},
		{TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA, "TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA", supportedUpToTLS12, false},
		{TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256, "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256", supportedOnlyTLS12, false},
		{TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384, "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384", supportedOnlyTLS12, false},
		{TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", supportedOnlyTLS12, false},
		{TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384, "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384", supportedOnlyTLS12, false},
		{TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256, "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256", supportedOnlyTLS12, false},
		{TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256, "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256", supportedOnlyTLS12, false},
	}
}

// InsecureCipherSuites returns a list of cipher suites currently implemented by
// this package and which have security issues.
//
// Most applications should not use the cipher suites in this list, and should
// only use those returned by [CipherSuites].
func InsecureCipherSuites() []*CipherSuite {
	// This list includes legacy RSA kex, RC4, CBC_SHA256, and 3DES cipher
	// suites. See cipherSuitesPreferenceOrder for details.
	return []*CipherSuite{
		{TLS_RSA_WITH_RC4_128_SHA, "TLS_RSA_WITH_RC4_128_SHA", supportedUpToTLS12, true},
		{TLS_RSA_WITH_3DES_EDE_CBC_SHA, "TLS_RSA_WITH_3DES_EDE_CBC_SHA", supportedUpToTLS12, true},
		{TLS_RSA_WITH_AES_128_CBC_SHA, "TLS_RSA_WITH_AES_128_CBC_SHA", supportedUpToTLS12, true},
		{TLS_RSA_WITH_AES_256_CBC_SHA, "TLS_RSA_WITH_AES_256_CBC_SHA", supportedUpToTLS12, true},
		{TLS_RSA_WITH_AES_128_CBC_SHA256, "TLS_RSA_WITH_AES_128_CBC_SHA256", supportedOnlyTLS12, true},
		{TLS_RSA_WITH_AES_128_GCM_SHA256, "TLS_RSA_WITH_AES_128_GCM_SHA256", supportedOnlyTLS12, true},
		{TLS_RSA_WITH_AES_256_GCM_SHA384, "TLS_RSA_WITH_AES_256_GCM_SHA384", supportedOnlyTLS12, true},
		{TLS_ECDHE_ECDSA_WITH_RC4_128_SHA, "TLS_ECDHE_ECDSA_WITH_RC4_128_SHA", supportedUpToTLS12, true},
		{TLS_ECDHE_RSA_WITH_RC4_128_SHA, "TLS_ECDHE_RSA_WITH_RC4_128_SHA", supportedUpToTLS12, true},
		{TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA, "TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA", supportedUpToTLS12, true},
		{TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256, "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256", supportedOnlyTLS12, true},
		{TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256, "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256", supportedOnlyTLS12, true},
	}
}

// CipherSuiteName returns the standard name for the passed cipher suite ID
// (e.g. "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"), or a fallback representation
// of the ID value if the cipher suite is not implemented by this package.
func CipherSuiteName(id uint16) string {
	for _, c := range CipherSuites() {
		if c.ID == id {
			return c.Name
		}
	}
	for _, c := range InsecureCipherSuites() {
		if c.ID == id {
			return c.Name
		}
	}
	return fmt.Sprintf("0x%04X", id)
}

const (
	// suiteECDHE indicates that the cipher suite involves elliptic curve
	// Diffie-Hellman. This means that it should only be selected when the
	// client indicates that it supports ECC with a curve and point format
	// that we're happy with.
	suiteECDHE = 1 << iota
	// suiteECSign indicates that the cipher suite involves an ECDSA or
	// EdDSA signature and therefore may only be selected when the server's
	// certificate is ECDSA or EdDSA. If this is not set then the cipher suite
	// is RSA based.
	suiteECSign
	// suiteTLS12 indicates that the cipher suite should only be advertised
	// and accepted when using TLS 1.2.
	suiteTLS12
	// suiteSHA384 indicates that the cipher suite uses SHA384 as the
	// handshake hash.
	suiteSHA384
)

// A cipherSuite is a TLS 1.0–1.2 cipher suite, and defines the key exchange
// mechanism, as well as the cipher+MAC pair or the AEAD.
type cipherSuite struct {
	id uint16
	// the lengths, in bytes, of the key material needed for each component.
	keyLen int
	macLen int
	ivLen  int
	ka     func(version uint16) keyAgreement
	// flags is a bitmask of the suite* values, above.
	flags  int
	cipher func(key, iv []byte, isRead bool) any
	mac    func(key []byte) hash.Hash
	aead   func(key, fixedNonce []byte) aead
}

var cipherSuites = []*cipherSuite{ // TODO: replace with a map, since the order doesn't matter.
	{TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256, 32, 0, 12, ecdheRSAKA, suiteECDHE | suiteTLS12, nil, nil, aeadChaCha20Poly1305},
	{TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256, 32, 0, 12, ecdheECDSAKA, suiteECDHE | suiteECSign | suiteTLS12, nil, nil, aeadChaCha20Poly1305},
	{TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, 16, 0, 4, ecdheRSAKA, suiteECDHE | suiteTLS12, nil, nil, aeadAESGCM},
	{TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256, 16, 0, 4, ecdheECDSAKA, suiteECDHE | suiteECSign | suiteTLS12, nil, nil, aeadAESGCM},
	{TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384, 32, 0, 4, ecdheRSAKA, suiteECDHE | suiteTLS12 | suiteSHA384, nil, nil, aeadAESGCM},
	{TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384, 32, 0, 4, ecdheECDSAKA, suiteECDHE | suiteECSign | suiteTLS12 | suiteSHA384, nil, nil, aeadAESGCM},
	{TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256, 16, 32, 16, ecdheRSAKA, suiteECDHE | suiteTLS12, cipherAES, macSHA256, nil},
	{TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA, 16, 20, 16, ecdheRSAKA, suiteECDHE, cipherAES, macSHA1, nil},
	{TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256, 16, 32, 16, ecdheECDSAKA, suiteECDHE | suiteECSign | suiteTLS12, cipherAES, macSHA256, nil},
	{TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA, 16, 20, 16, ecdheECDSAKA, suiteECDHE | suiteECSign, cipherAES, macSHA1, nil},
	{TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA, 32, 20, 16, ecdheRSAKA, suiteECDHE, cipherAES, macSHA1, nil},
	{TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA, 32, 20, 16, ecdheECDSAKA, suiteECDHE | suiteECSign, cipherAES, macSHA1, nil},
	{TLS_RSA_WITH_AES_128_GCM_SHA256, 16, 0, 4, rsaKA, suiteTLS12, nil, nil, aeadAESGCM},
	{TLS_RSA_WITH_AES_256_GCM_SHA384, 32, 0, 4, rsaKA, suiteTLS12 | suiteSHA384, nil, nil, aeadAESGCM},
	{TLS_RSA_WITH_AES_128_CBC_SHA256, 16, 32, 16, rsaKA, suiteTLS12, cipherAES, macSHA256, nil},
	{TLS_RSA_WITH_AES_128_CBC_SHA, 16, 20, 16, rsaKA, 0, cipherAES, macSHA1, nil},
	{TLS_RSA_WITH_AES_256_CBC_SHA, 32, 20, 16, rsaKA, 0, cipherAES, macSHA1, nil},
	{TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA, 24, 20, 8, ecdheRSAKA, suiteECDHE, cipher3DES, macSHA1, nil},
	{TLS_RSA_WITH_3DES_EDE_CBC_SHA, 24, 20, 8, rsaKA, 0, cipher3DES, macSHA1, nil},
	{TLS_RSA_WITH_RC4_128_SHA, 16, 20, 0, rsaKA, 0, cipherRC4, macSHA1, nil},
	{TLS_ECDHE_RSA_WITH_RC4_128_SHA, 16, 20, 0, ecdheRSAKA, suiteECDHE, cipherRC4, macSHA1, nil},
	{TLS_ECDHE_ECDSA_WITH_RC4_128_SHA, 16, 20, 0, ecdheECDSAKA, suiteECDHE | suiteECSign, cipherRC4, macSHA1, nil},
}

// selectCipherSuite returns the first TLS 1.0–1.2 cipher suite from ids which
// is also in supportedIDs and passes the ok filter.
func selectCipherSuite(ids, supportedIDs []uint16, ok func(*cipherSuite) bool) *cipherSuite {
	for _, id := range ids {
		candidate := cipherSuiteByID(id)
		if candidate == nil || !ok(candidate) {
			continue
		}

		for _, suppID := range supportedIDs {
			if id == suppID {
				return candidate
			}
		}
	}
	return nil
}

// A cipherSuiteTLS13 defines only the pair of the AEAD algorithm and hash
// algorithm to be used with HKDF. See RFC 8446, Appendix B.4.
type cipherSuiteTLS13 struct {
	id     uint16
	keyLen int
	aead   func(key, fixedNonce []byte) aead
	hash   crypto.Hash
}

// cipherSuitesTLS13 should be an internal detail,
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/quic-go/quic-go
//   - github.com/sagernet/quic-go
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
//go:linkname cipherSuitesTLS13
var cipherSuitesTLS13 = []*cipherSuiteTLS13{ // TODO: replace with a map.
	{TLS_AES_128_GCM_SHA256, 16, aeadAESGCMTLS13, crypto.SHA256},
	{TLS_CHACHA20_POLY1305_SHA256, 32, aeadChaCha20Poly1305, crypto.SHA256},
	{TLS_AES_256_GCM_SHA384, 32, aeadAESGCMTLS13, crypto.SHA384},
}

// cipherSuitesPreferenceOrder is the order in which we'll select (on the
// server) or advertise (on the client) TLS 1.0–1.2 cipher suites.
//
// Cipher suites are filtered but not reordered based on the application and
// peer's preferences, meaning we'll never select a suite lower in this list if
// any higher one is available. This makes it more defensible to keep weaker
// cipher suites enabled, especially on the server side where we get the last
// word, since there are no known downgrade attacks on cipher suites selection.
//
// The list is sorted by applying the following priority rules, stopping at the
// first (most important) applicable one:
//
//   - Anything else comes before RC4
//
//     RC4 has practically exploitable biases. See https://www.rc4nomore.com.
//
//   - Anything else comes before CBC_SHA256
//
//     SHA-256 variants of the CBC ciphersuites don't implement any Lucky13
//     countermeasures. See https://www.isg.rhul.ac.uk/tls/Lucky13.html and
//     https://www.imperialviolet.org/2013/02/04/luckythirteen.html.
//
//   - Anything else comes before 3DES
//
//     3DES has 64-bit blocks, which makes it fundamentally susceptible to
//     birthday attacks. See https://sweet32.info.
//
//   - ECDHE comes before anything else
//
//     Once we got the broken stuff out of the way, the most important
//     property a cipher suite can have is forward secrecy. We don't
//     implement FFDHE, so that means ECDHE.
//
//   - AEADs come before CBC ciphers
//
//     Even with Lucky13 countermeasures, MAC-then-Encrypt CBC cipher suites
//     are fundamentally fragile, and suffered from an endless sequence of
//     padding oracle attacks. See https://eprint.iacr.org/2015/1129,
//     https://www.imperialviolet.org/2014/12/08/poodleagain.html, and
//     https://blog.cloudflare.com/yet-another-padding-oracle-in-openssl-cbc-ciphersuites/.
//
//   - AES comes before ChaCha20
//
//     When AES hardware is available, AES-128-GCM and AES-256-GCM are faster
//     than ChaCha20Poly1305.
//
//     When AES hardware is not available, AES-128-GCM is one or more of: much
//     slower, way more complex, and less safe (because not constant time)
//     than ChaCha20Poly1305.
//
//     We use this list if we think both peers have AES hardware, and
//     cipherSuitesPreferenceOrderNoAES otherwise.
//
//   - AES-128 comes before AES-256
//
//     The only potential advantages of AES-256 are better multi-target
//     margins, and hypothetical post-quantum properties. Neither apply to
//     TLS, and AES-256 is slower due to its four extra rounds (which don't
//     contribute to the advantages above).
//
//   - ECDSA comes before RSA
//
//     The relative order of ECDSA and RSA cipher suites doesn't matter,
//     as they depend on the certificate. Pick one to get a stable order.
var cipherSuitesPreferenceOrder = []uint16{
	// AEADs w/ ECDHE
	TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256, TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384, TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256, TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,

	// CBC w/ ECDHE
	TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA, TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA, TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,

	// AEADs w/o ECDHE
	TLS_RSA_WITH_AES_128_GCM_SHA256,
	TLS_RSA_WITH_AES_256_GCM_SHA384,

	// CBC w/o ECDHE
	TLS_RSA_WITH_AES_128_CBC_SHA,
	TLS_RSA_WITH_AES_256_CBC_SHA,

	// 3DES
	TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
	TLS_RSA_WITH_3DES_EDE_CBC_SHA,

	// CBC_SHA256
	TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256, TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
	TLS_RSA_WITH_AES_128_CBC_SHA256,

	// RC4
	TLS_ECDHE_ECDSA_WITH_RC4_128_SHA, TLS_ECDHE_RSA_WITH_RC4_128_SHA,
	TLS_RSA_WITH_RC4_128_SHA,
}

var cipherSuitesPreferenceOrderNoAES = []uint16{
	// ChaCha20Poly1305
	TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256, TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,

	// AES-GCM w/ ECDHE
	TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256, TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384, TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,

	// The rest of cipherSuitesPreferenceOrder.
	TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA, TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA, TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	TLS_RSA_WITH_AES_128_GCM_SHA256,
	TLS_RSA_WITH_AES_256_GCM_SHA384,
	TLS_RSA_WITH_AES_128_CBC_SHA,
	TLS_RSA_WITH_AES_256_CBC_SHA,
	TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
	TLS_RSA_WITH_3DES_EDE_CBC_SHA,
	TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256, TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
	TLS_RSA_WITH_AES_128_CBC_SHA256,
	TLS_ECDHE_ECDSA_WITH_RC4_128_SHA, TLS_ECDHE_RSA_WITH_RC4_128_SHA,
	TLS_RSA_WITH_RC4_128_SHA,
}

// disabledCipherSuites are not used unless explicitly listed in Config.CipherSuites.
var disabledCipherSuites = map[uint16]bool{
	// CBC_SHA256
	TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256: true,
	TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256:   true,
	TLS_RSA_WITH_AES_128_CBC_SHA256:         true,

	// RC4
	TLS_ECDHE_ECDSA_WITH_RC4_128_SHA: true,
	TLS_ECDHE_RSA_WITH_RC4_128_SHA:   true,
	TLS_RSA_WITH_RC4_128_SHA:         true,

	// RSA key exchange
	TLS_RSA_WITH_3DES_EDE_CBC_SHA:   true,
	TLS_RSA_WITH_AES_128_CBC_SHA:    true,
	TLS_RSA_WITH_AES_256_CBC_SHA:    true,
	TLS_RSA_WITH_AES_128_GCM_SHA256: true,
	TLS_RSA_WITH_AES_256_GCM_SHA384: true,

	// 3DES
	TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA: true,
}

var (
	// Keep in sync with crypto/internal/fips140/aes/gcm.supportsAESGCM.
	hasGCMAsmAMD64 = cpu.X86.HasAES && cpu.X86.HasPCLMULQDQ && cpu.X86.HasSSE41 && cpu.X86.HasSSSE3
	hasGCMAsmARM64 = cpu.ARM64.HasAES && cpu.ARM64.HasPMULL
	hasGCMAsmS390X = cpu.S390X.HasAES && cpu.S390X.HasAESCTR && cpu.S390X.HasGHASH
	hasGCMAsmPPC64 = runtime.GOARCH == "ppc64" || runtime.GOARCH == "ppc64le"

	hasAESGCMHardwareSupport = hasGCMAsmAMD64 || hasGCMAsmARM64 || hasGCMAsmS390X || hasGCMAsmPPC64
)

var aesgcmCiphers = map[uint16]bool{
	// TLS 1.2
	TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256:   true,
	TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:   true,
	TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256: true,
	TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384: true,
	// TLS 1.3
	TLS_AES_128_GCM_SHA256: true,
	TLS_AES_256_GCM_SHA384: true,
}

// isAESGCMPreferred returns whether we have hardware support for AES-GCM, and the
// first known cipher in the peer's preference list is an AES-GCM cipher,
// implying the peer also has hardware support for it.
func isAESGCMPreferred(ciphers []uint16) bool {
	if !hasAESGCMHardwareSupport {
		return false
	}
	for _, cID := range ciphers {
		if c := cipherSuiteByID(cID); c != nil {
			return aesgcmCiphers[cID]
		}
		if c := cipherSuiteTLS13ByID(cID); c != nil {
			return aesgcmCiphers[cID]
		}
	}
	return false
}

func cipherRC4(key, iv []byte, isRead bool) any {
	cipher, _ := rc4.NewCipher(key)
	return cipher
}

func cipher3DES(key, iv []byte, isRead bool) any {
	block, _ := des.NewTripleDESCipher(key)
	if isRead {
		return cipher.NewCBCDecrypter(block, iv)
	}
	return cipher.NewCBCEncrypter(block, iv)
}

func cipherAES(key, iv []byte, isRead bool) any {
	block, _ := aes.NewCipher(key)
	if isRead {
		return cipher.NewCBCDecrypter(block, iv)
	}
	return cipher.NewCBCEncrypter(block, iv)
}

// macSHA1 returns a SHA-1 based constant time MAC.
func macSHA1(key []byte) hash.Hash {
	h := sha1.New
	// The BoringCrypto SHA1 does not have a constant-time
	// checksum function, so don't try to use it.
	if !boring.Enabled {
		h = newConstantTimeHash(h)
	}
	return hmac.New(h, key)
}

// macSHA256 returns a SHA-256 based MAC. This is only supported in TLS 1.2 and
// is currently only used in disabled-by-default cipher suites.
func macSHA256(key []byte) hash.Hash {
	return hmac.New(sha256.New, key)
}

type aead interface {
	cipher.AEAD

	// explicitNonceLen returns the number of bytes of explicit nonce
	// included in each record. This is eight for older AEADs and
	// zero for modern ones.
	explicitNonceLen() int
}

const (
	aeadNonceLength   = 12
	noncePrefixLength = 4
)

// prefixNonceAEAD wraps an AEAD and prefixes a fixed portion of the nonce to
// each call.
type prefixNonceAEAD struct {
	// nonce contains the fixed part of the nonce in the first four bytes.
	nonce [aeadNonceLength]byte
	aead  cipher.AEAD
}

func (f *prefixNonceAEAD) NonceSize() int        { return aeadNonceLength - noncePrefixLength }
func (f *prefixNonceAEAD) Overhead() int         { return f.aead.Overhead() }
func (f *prefixNonceAEAD) explicitNonceLen() int { return f.NonceSize() }

func (f *prefixNonceAEAD) Seal(out, nonce, plaintext, additionalData []byte) []byte {
	copy(f.nonce[4:], nonce)
	return f.aead.Seal(out, f.nonce[:], plaintext, additionalData)
}

func (f *prefixNonceAEAD) Open(out, nonce, ciphertext, additionalData []byte) ([]byte, error) {
	copy(f.nonce[4:], nonce)
	return f.aead.Open(out, f.nonce[:], ciphertext, additionalData)
}

// xorNonceAEAD wraps an AEAD by XORing in a fixed pattern to the nonce
// before each call.
type xorNonceAEAD struct {
	nonceMask [aeadNonceLength]byte
	aead      cipher.AEAD
}

func (f *xorNonceAEAD) NonceSize() int        { return 8 } // 64-bit sequence number
func (f *xorNonceAEAD) Overhead() int         { return f.aead.Overhead() }
func (f *xorNonceAEAD) explicitNonceLen() int { return 0 }

func (f *xorNonceAEAD) Seal(out, nonce, plaintext, additionalData []byte) []byte {
	for i, b := range nonce {
		f.nonceMask[4+i] ^= b
	}
	result := f.aead.Seal(out, f.nonceMask[:], plaintext, additionalData)
	for i, b := range nonce {
		f.nonceMask[4+i] ^= b
	}

	return result
}

func (f *xorNonceAEAD) Open(out, nonce, ciphertext, additionalData []byte) ([]byte, error) {
	for i, b := range nonce {
		f.nonceMask[4+i] ^= b
	}
	result, err := f.aead.Open(out, f.nonceMask[:], ciphertext, additionalData)
	for i, b := range nonce {
		f.nonceMask[4+i] ^= b
	}

	return result, err
}

func aeadAESGCM(key, noncePrefix []byte) aead {
	if len(noncePrefix) != noncePrefixLength {
		panic("tls: internal error: wrong nonce length")
	}
	aes, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	var aead cipher.AEAD
	if boring.Enabled {
		aead, err = boring.NewGCMTLS(aes)
	} else {
		boring.Unreachable()
		aead, err = gcm.NewGCMForTLS12(aes.(*fipsaes.Block))
	}
	if err != nil {
		panic(err)
	}

	ret := &prefixNonceAEAD{aead: aead}
	copy(ret.nonce[:], noncePrefix)
	return ret
}

// aeadAESGCMTLS13 should be an internal detail,
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/xtls/xray-core
//   - github.com/v2fly/v2ray-core
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
//go:linkname aeadAESGCMTLS13
func aeadAESGCMTLS13(key, nonceMask []byte) aead {
	if len(nonceMask) != aeadNonceLength {
		panic("tls: internal error: wrong nonce length")
	}
	aes, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	var aead cipher.AEAD
	if boring.Enabled {
		aead, err = boring.NewGCMTLS13(aes)
	} else {
		boring.Unreachable()
		aead, err = gcm.NewGCMForTLS13(aes.(*fipsaes.Block))
	}
	if err != nil {
		panic(err)
	}

	ret := &xorNonceAEAD{aead: aead}
	copy(ret.nonceMask[:], nonceMask)
	return ret
}

func aeadChaCha20Poly1305(key, nonceMask []byte) aead {
	if len(nonceMask) != aeadNonceLength {
		panic("tls: internal error: wrong nonce length")
	}
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		panic(err)
	}

	ret := &xorNonceAEAD{aead: aead}
	copy(ret.nonceMask[:], nonceMask)
	return ret
}

type constantTimeHash interface {
	hash.Hash
	ConstantTimeSum(b []byte) []byte
}

// cthWrapper wraps any hash.Hash that implements ConstantTimeSum, and replaces
// with that all calls to Sum. It's used to obtain a ConstantTimeSum-based HMAC.
type cthWrapper struct {
	h constantTimeHash
}

func (c *cthWrapper) Size() int                   { return c.h.Size() }
func (c *cthWrapper) BlockSize() int              { return c.h.BlockSize() }
func (c *cthWrapper) Reset()                      { c.h.Reset() }
func (c *cthWrapper) Write(p []byte) (int, error) { return c.h.Write(p) }
func (c *cthWrapper) Sum(b []byte) []byte         { return c.h.ConstantTimeSum(b) }

func newConstantTimeHash(h func() hash.Hash) func() hash.Hash {
	boring.Unreachable()
	return func() hash.Hash {
		return &cthWrapper{h().(constantTimeHash)}
	}
}

// tls10MAC implements the TLS 1.0 MAC function. RFC 2246, Section 6.2.3.
func tls10MAC(h hash.Hash, out, seq, header, data, extra []byte) []byte {
	h.Reset()
	h.Write(seq)
	h.Write(header)
	h.Write(data)
	res := h.Sum(out)
	if extra != nil {
		h.Write(extra)
	}
	return res
}

func rsaKA(version uint16) keyAgreement {
	return rsaKeyAgreement{}
}

func ecdheECDSAKA(version uint16) keyAgreement {
	return &ecdheKeyAgreement{
		isRSA:   false,
		version: version,
	}
}

func ecdheRSAKA(version uint16) keyAgreement {
	return &ecdheKeyAgreement{
		isRSA:   true,
		version: version,
	}
}

// mutualCipherSuite returns a cipherSuite given a list of supported
// ciphersuites and the id requested by the peer.
func mutualCipherSuite(have []uint16, want uint16) *cipherSuite {
	for _, id := range have {
		if id == want {
			return cipherSuiteByID(id)
		}
	}
	return nil
}

func cipherSuiteByID(id uint16) *cipherSuite {
	for _, cipherSuite := range cipherSuites {
		if cipherSuite.id == id {
			return cipherSuite
		}
	}
	return nil
}

func mutualCipherSuiteTLS13(have []uint16, want uint16) *cipherSuiteTLS13 {
	for _, id := range have {
		if id == want {
			return cipherSuiteTLS13ByID(id)
		}
	}
	return nil
}

func cipherSuiteTLS13ByID(id uint16) *cipherSuiteTLS13 {
	for _, cipherSuite := range cipherSuitesTLS13 {
		if cipherSuite.id == id {
			return cipherSuite
		}
	}
	return nil
}

// A list of cipher suite IDs that are, or have been, implemented by this
// package.
//
// See https://www.iana.org/assignments/tls-parameters/tls-parameters.xml
const (
	// TLS 1.0 - 1.2 cipher suites.
	TLS_RSA_WITH_RC4_128_SHA                      uint16 = 0x0005
	TLS_RSA_WITH_3DES_EDE_CBC_SHA                 uint16 = 0x000a
	TLS_RSA_WITH_AES_128_CBC_SHA                  uint16 = 0x002f
	TLS_RSA_WITH_AES_256_CBC_SHA                  uint16 = 0x0035
	TLS_RSA_WITH_AES_128_CBC_SHA256               uint16 = 0x003c
	TLS_RSA_WITH_AES_128_GCM_SHA256               uint16 = 0x009c
	TLS_RSA_WITH_AES_256_GCM_SHA384               uint16 = 0x009d
	TLS_ECDHE_ECDSA_WITH_RC4_128_SHA              uint16 = 0xc007
	TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA          uint16 = 0xc009
	TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA          uint16 = 0xc00a
	TLS_ECDHE_RSA_WITH_RC4_128_SHA                uint16 = 0xc011
	TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA           uint16 = 0xc012
	TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA            uint16 = 0xc013
	TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA            uint16 = 0xc014
	TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256       uint16 = 0xc023
	TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256         uint16 = 0xc027
	TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256         uint16 = 0xc02f
	TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256       uint16 = 0xc02b
	TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384         uint16 = 0xc030
	TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384       uint16 = 0xc02c
	TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256   uint16 = 0xcca8
	TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256 uint16 = 0xcca9

	// TLS 1.3 cipher suites.
	TLS_AES_128_GCM_SHA256       uint16 = 0x1301
	TLS_AES_256_GCM_SHA384       uint16 = 0x1302
	TLS_CHACHA20_POLY1305_SHA256 uint16 = 0x1303

	// TLS_FALLBACK_SCSV isn't a standard cipher suite but an indicator
	// that the client is doing version fallback. See RFC 7507.
	TLS_FALLBACK_SCSV uint16 = 0x5600

	// Legacy names for the corresponding cipher suites with the correct _SHA256
	// suffix, retained for backward compatibility.
	TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305   = TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256
	TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305 = TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256
)

```

// === FILE: references/go/src/crypto/tls/common.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tls

import (
	"bytes"
	"container/list"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/fips140"
	"crypto/mldsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/tls/internal/fips140tls"
	"crypto/x509"
	"errors"
	"fmt"
	"internal/godebug"
	"io"
	"net"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
	_ "unsafe" // for linkname
)

const (
	VersionTLS10 = 0x0301
	VersionTLS11 = 0x0302
	VersionTLS12 = 0x0303
	VersionTLS13 = 0x0304

	// Deprecated: SSLv3 is cryptographically broken, and is no longer
	// supported by this package. See golang.org/issue/32716.
	VersionSSL30 = 0x0300
)

// VersionName returns the name for the provided TLS version number
// (e.g. "TLS 1.3"), or a fallback representation of the value if the
// version is not implemented by this package.
func VersionName(version uint16) string {
	switch version {
	case VersionSSL30:
		return "SSLv3"
	case VersionTLS10:
		return "TLS 1.0"
	case VersionTLS11:
		return "TLS 1.1"
	case VersionTLS12:
		return "TLS 1.2"
	case VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("0x%04X", version)
	}
}

const (
	maxPlaintext               = 16384        // maximum plaintext payload length
	maxCiphertext              = 16384 + 2048 // maximum ciphertext payload length
	maxCiphertextTLS13         = 16384 + 256  // maximum ciphertext length in TLS 1.3
	recordHeaderLen            = 5            // record header length
	maxHandshake               = 65536        // maximum handshake we support (protocol max is 16 MB)
	maxHandshakeCertificateMsg = 262144       // maximum certificate message size (256 KiB)
	maxUselessRecords          = 16           // maximum number of consecutive non-advancing records
)

// TLS record types.
type recordType uint8

const (
	recordTypeChangeCipherSpec recordType = 20
	recordTypeAlert            recordType = 21
	recordTypeHandshake        recordType = 22
	recordTypeApplicationData  recordType = 23
)

// TLS handshake message types.
const (
	typeHelloRequest        uint8 = 0
	typeClientHello         uint8 = 1
	typeServerHello         uint8 = 2
	typeNewSessionTicket    uint8 = 4
	typeEndOfEarlyData      uint8 = 5
	typeEncryptedExtensions uint8 = 8
	typeCertificate         uint8 = 11
	typeServerKeyExchange   uint8 = 12
	typeCertificateRequest  uint8 = 13
	typeServerHelloDone     uint8 = 14
	typeCertificateVerify   uint8 = 15
	typeClientKeyExchange   uint8 = 16
	typeFinished            uint8 = 20
	typeCertificateStatus   uint8 = 22
	typeKeyUpdate           uint8 = 24
	typeMessageHash         uint8 = 254 // synthetic message
)

// TLS compression types.
const (
	compressionNone uint8 = 0
)

// TLS extension numbers
const (
	extensionServerName              uint16 = 0
	extensionStatusRequest           uint16 = 5
	extensionSupportedCurves         uint16 = 10 // supported_groups in TLS 1.3, see RFC 8446, Section 4.2.7
	extensionSupportedPoints         uint16 = 11
	extensionSignatureAlgorithms     uint16 = 13
	extensionALPN                    uint16 = 16
	extensionSCT                     uint16 = 18
	extensionExtendedMasterSecret    uint16 = 23
	extensionSessionTicket           uint16 = 35
	extensionPreSharedKey            uint16 = 41
	extensionEarlyData               uint16 = 42
	extensionSupportedVersions       uint16 = 43
	extensionCookie                  uint16 = 44
	extensionPSKModes                uint16 = 45
	extensionCertificateAuthorities  uint16 = 47
	extensionSignatureAlgorithmsCert uint16 = 50
	extensionKeyShare                uint16 = 51
	extensionQUICTransportParameters uint16 = 57
	extensionRenegotiationInfo       uint16 = 0xff01
	extensionECHOuterExtensions      uint16 = 0xfd00
	extensionEncryptedClientHello    uint16 = 0xfe0d
)

// TLS signaling cipher suite values
const (
	scsvRenegotiation uint16 = 0x00ff
)

// CurveID is the type of a TLS identifier for a key exchange mechanism. See
// https://www.iana.org/assignments/tls-parameters/tls-parameters.xml#tls-parameters-8.
//
// In TLS 1.2, this registry used to support only elliptic curves. In TLS 1.3,
// it was extended to other groups and renamed NamedGroup. See RFC 8446, Section
// 4.2.7. It was then also extended to other mechanisms, such as hybrid
// post-quantum KEMs.
type CurveID uint16

const (
	CurveP256          CurveID = 23
	CurveP384          CurveID = 24
	CurveP521          CurveID = 25
	X25519             CurveID = 29
	X25519MLKEM768     CurveID = 4588
	SecP256r1MLKEM768  CurveID = 4587
	SecP384r1MLKEM1024 CurveID = 4589
	MLKEM1024          CurveID = 514
)

func isTLS13OnlyKeyExchange(curve CurveID) bool {
	switch curve {
	case X25519MLKEM768, SecP256r1MLKEM768, SecP384r1MLKEM1024, MLKEM1024:
		return true
	default:
		return false
	}
}

func isPQKeyExchange(curve CurveID) bool {
	switch curve {
	case X25519MLKEM768, SecP256r1MLKEM768, SecP384r1MLKEM1024, MLKEM1024:
		return true
	default:
		return false
	}
}

// TLS 1.3 Key Share. See RFC 8446, Section 4.2.8.
type keyShare struct {
	group CurveID
	data  []byte
}

// TLS 1.3 PSK Key Exchange Modes. See RFC 8446, Section 4.2.9.
const (
	pskModePlain uint8 = 0
	pskModeDHE   uint8 = 1
)

// TLS 1.3 PSK Identity. Can be a Session Ticket, or a reference to a saved
// session. See RFC 8446, Section 4.2.11.
type pskIdentity struct {
	label               []byte
	obfuscatedTicketAge uint32
}

// TLS Elliptic Curve Point Formats
// https://www.iana.org/assignments/tls-parameters/tls-parameters.xml#tls-parameters-9
const (
	pointFormatUncompressed uint8 = 0
)

// TLS CertificateStatusType (RFC 3546)
const (
	statusTypeOCSP uint8 = 1
)

// Certificate types (for certificateRequestMsg)
const (
	certTypeRSASign   = 1
	certTypeECDSASign = 64 // ECDSA or EdDSA keys, see RFC 8422, Section 3.
)

// Signature algorithms (for internal signaling use). Starting at 225 to avoid overlap with
// TLS 1.2 codepoints (RFC 5246, Appendix A.4.1), with which these have nothing to do.
const (
	signaturePKCS1v15 uint8 = iota + 225
	signatureRSAPSS
	signatureECDSA
	signatureEd25519
	signatureMLDSA
)

// directSigning is a standard Hash value that signals that no pre-hashing
// should be performed, and that the input should be signed directly. It is the
// hash function associated with the Ed25519 and ML-DSA signature schemes.
var directSigning crypto.Hash = 0

// helloRetryRequestRandom is set as the Random value of a ServerHello
// to signal that the message is actually a HelloRetryRequest.
var helloRetryRequestRandom = []byte{ // See RFC 8446, Section 4.1.3.
	0xCF, 0x21, 0xAD, 0x74, 0xE5, 0x9A, 0x61, 0x11,
	0xBE, 0x1D, 0x8C, 0x02, 0x1E, 0x65, 0xB8, 0x91,
	0xC2, 0xA2, 0x11, 0x16, 0x7A, 0xBB, 0x8C, 0x5E,
	0x07, 0x9E, 0x09, 0xE2, 0xC8, 0xA8, 0x33, 0x9C,
}

const (
	// downgradeCanaryTLS12 or downgradeCanaryTLS11 is embedded in the server
	// random as a downgrade protection if the server would be capable of
	// negotiating a higher version. See RFC 8446, Section 4.1.3.
	downgradeCanaryTLS12 = "DOWNGRD\x01"
	downgradeCanaryTLS11 = "DOWNGRD\x00"
)

// testingOnlyForceDowngradeCanary is set in tests to force the server side to
// include downgrade canaries even if it's using its highers supported version.
var testingOnlyForceDowngradeCanary bool

// ConnectionState records basic TLS details about the connection.
type ConnectionState struct {
	// Version is the TLS version used by the connection (e.g. VersionTLS12).
	Version uint16

	// HandshakeComplete is true if the handshake has concluded.
	HandshakeComplete bool

	// DidResume is true if this connection was successfully resumed from a
	// previous session with a session ticket or similar mechanism.
	DidResume bool

	// CipherSuite is the cipher suite negotiated for the connection (e.g.
	// TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256, TLS_AES_128_GCM_SHA256).
	CipherSuite uint16

	// CurveID is the key exchange mechanism used for the connection. The name
	// refers to elliptic curves for legacy reasons, see [CurveID]. If a legacy
	// RSA key exchange is used, this value is zero.
	CurveID CurveID

	// NegotiatedProtocol is the application protocol negotiated with ALPN.
	NegotiatedProtocol string

	// NegotiatedProtocolIsMutual used to indicate a mutual NPN negotiation.
	//
	// Deprecated: this value is always true.
	NegotiatedProtocolIsMutual bool

	// ServerName is the value of the Server Name Indication extension sent by
	// the client. It's available both on the server and on the client side.
	ServerName string

	// PeerCertificates are the parsed certificates sent by the peer, in the
	// order in which they were sent. The first element is the leaf certificate
	// that the connection is verified against.
	//
	// On the client side, it can't be empty. On the server side, it can be
	// empty if Config.ClientAuth is not RequireAnyClientCert or
	// RequireAndVerifyClientCert.
	//
	// PeerCertificates and its contents should not be modified.
	PeerCertificates []*x509.Certificate

	// VerifiedChains is a list of one or more chains where the first element is
	// PeerCertificates[0] and the last element is from Config.RootCAs (on the
	// client side) or Config.ClientCAs (on the server side).
	//
	// On the client side, it's set if Config.InsecureSkipVerify is false. On
	// the server side, it's set if Config.ClientAuth is VerifyClientCertIfGiven
	// (and the peer provided a certificate) or RequireAndVerifyClientCert.
	//
	// VerifiedChains and its contents should not be modified.
	VerifiedChains [][]*x509.Certificate

	// SignedCertificateTimestamps is a list of SCTs provided by the peer
	// through the TLS handshake for the leaf certificate, if any.
	SignedCertificateTimestamps [][]byte

	// OCSPResponse is a stapled Online Certificate Status Protocol (OCSP)
	// response provided by the peer for the leaf certificate, if any.
	OCSPResponse []byte

	// TLSUnique contains the "tls-unique" channel binding value (see RFC 5929,
	// Section 3). This value will be nil for TLS 1.3 connections and for
	// resumed connections that don't support Extended Master Secret (RFC 7627).
	TLSUnique []byte

	// ECHAccepted indicates if Encrypted Client Hello was offered by the client
	// and accepted by the server. Currently, ECH is supported only on the
	// client side.
	ECHAccepted bool

	// HelloRetryRequest indicates whether we sent a HelloRetryRequest if we
	// are a server, or if we received a HelloRetryRequest if we are a client.
	HelloRetryRequest bool

	// LocalCertificate is the certificate chain presented to the peer, if any,
	// during the handshake. This field is only populated for connections which
	// are not resumed (DidResume is false).
	LocalCertificate [][]byte

	// ekm is a closure exposed via ExportKeyingMaterial.
	ekm func(label string, context []byte, length int) ([]byte, error)

	// testingOnlyPeerSignatureAlgorithm is the signature algorithm used by the
	// peer to sign the handshake. It is not set for resumed connections.
	testingOnlyPeerSignatureAlgorithm SignatureScheme
}

// ExportKeyingMaterial returns length bytes of exported key material in a new
// slice as defined in RFC 5705. If context is nil, it is not used as part of
// the seed. If the connection was set to allow renegotiation via
// Config.Renegotiation, or if the connections supports neither TLS 1.3 nor
// Extended Master Secret, this function will return an error.
func (cs *ConnectionState) ExportKeyingMaterial(label string, context []byte, length int) ([]byte, error) {
	return cs.ekm(label, context, length)
}

// ClientAuthType declares the policy the server will follow for
// TLS Client Authentication.
type ClientAuthType int

const (
	// NoClientCert indicates that no client certificate should be requested
	// during the handshake, and if any certificates are sent they will not
	// be verified.
	NoClientCert ClientAuthType = iota
	// RequestClientCert indicates that a client certificate should be requested
	// during the handshake, but does not require that the client send any
	// certificates.
	RequestClientCert
	// RequireAnyClientCert indicates that a client certificate should be requested
	// during the handshake, and that at least one certificate is required to be
	// sent by the client, but that certificate is not required to be valid.
	RequireAnyClientCert
	// VerifyClientCertIfGiven indicates that a client certificate should be requested
	// during the handshake, but does not require that the client sends a
	// certificate. If the client does send a certificate it is required to be
	// valid.
	VerifyClientCertIfGiven
	// RequireAndVerifyClientCert indicates that a client certificate should be requested
	// during the handshake, and that at least one valid certificate is required
	// to be sent by the client.
	RequireAndVerifyClientCert
)

// requiresClientCert reports whether the ClientAuthType requires a client
// certificate to be provided.
func requiresClientCert(c ClientAuthType) bool {
	switch c {
	case RequireAnyClientCert, RequireAndVerifyClientCert:
		return true
	default:
		return false
	}
}

// ClientSessionCache is a cache of ClientSessionState objects that can be used
// by a client to resume a TLS session with a given server. ClientSessionCache
// implementations should expect to be called concurrently from different
// goroutines. Up to TLS 1.2, only ticket-based resumption is supported, not
// SessionID-based resumption. In TLS 1.3 they were merged into PSK modes, which
// are supported via this interface.
type ClientSessionCache interface {
	// Get searches for a ClientSessionState associated with the given key.
	// On return, ok is true if one was found.
	Get(sessionKey string) (session *ClientSessionState, ok bool)

	// Put adds the ClientSessionState to the cache with the given key. It might
	// get called multiple times in a connection if a TLS 1.3 server provides
	// more than one session ticket. If called with a nil *ClientSessionState,
	// it should remove the cache entry.
	Put(sessionKey string, cs *ClientSessionState)
}

//go:generate stringer -linecomment -type=SignatureScheme,CurveID,ClientAuthType -output=common_string.go

// SignatureScheme identifies a signature algorithm supported by TLS. See
// RFC 8446, Section 4.2.3.
type SignatureScheme uint16

const (
	// RSASSA-PKCS1-v1_5 algorithms.
	PKCS1WithSHA256 SignatureScheme = 0x0401
	PKCS1WithSHA384 SignatureScheme = 0x0501
	PKCS1WithSHA512 SignatureScheme = 0x0601

	// RSASSA-PSS algorithms with public key OID rsaEncryption.
	PSSWithSHA256 SignatureScheme = 0x0804
	PSSWithSHA384 SignatureScheme = 0x0805
	PSSWithSHA512 SignatureScheme = 0x0806

	// ECDSA algorithms. Only constrained to a specific curve in TLS 1.3.
	ECDSAWithP256AndSHA256 SignatureScheme = 0x0403
	ECDSAWithP384AndSHA384 SignatureScheme = 0x0503
	ECDSAWithP521AndSHA512 SignatureScheme = 0x0603

	// EdDSA algorithms.
	Ed25519 SignatureScheme = 0x0807

	// ML-DSA algorithms.
	MLDSA44 SignatureScheme = 0x0904
	MLDSA65 SignatureScheme = 0x0905
	MLDSA87 SignatureScheme = 0x0906

	// Legacy signature and hash algorithms for TLS 1.2.
	PKCS1WithSHA1 SignatureScheme = 0x0201
	ECDSAWithSHA1 SignatureScheme = 0x0203
)

// ClientHelloInfo contains information from a ClientHello message in order to
// guide application logic in the GetCertificate and GetConfigForClient callbacks.
type ClientHelloInfo struct {
	// CipherSuites lists the CipherSuites supported by the client (e.g.
	// TLS_AES_128_GCM_SHA256, TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256).
	CipherSuites []uint16

	// ServerName indicates the name of the server requested by the client
	// in order to support virtual hosting. ServerName is only set if the
	// client is using SNI (see RFC 4366, Section 3.1).
	ServerName string

	// SupportedCurves lists the key exchange mechanisms supported by the
	// client. It was renamed to "supported groups" in TLS 1.3, see RFC 8446,
	// Section 4.2.7 and [CurveID].
	//
	// SupportedCurves may be nil in TLS 1.2 and lower if the Supported Elliptic
	// Curves Extension is not being used (see RFC 4492, Section 5.1.1).
	SupportedCurves []CurveID

	// SupportedPoints lists the point formats supported by the client.
	// SupportedPoints is set only if the Supported Point Formats Extension
	// is being used (see RFC 4492, Section 5.1.2).
	SupportedPoints []uint8

	// SignatureSchemes lists the signature and hash schemes that the client
	// is willing to verify. SignatureSchemes is set only if the Signature
	// Algorithms Extension is being used (see RFC 5246, Section 7.4.1.4.1).
	SignatureSchemes []SignatureScheme

	// SupportedProtos lists the application protocols supported by the client.
	// SupportedProtos is set only if the Application-Layer Protocol
	// Negotiation Extension is being used (see RFC 7301, Section 3.1).
	//
	// Servers can select a protocol by setting Config.NextProtos in a
	// GetConfigForClient return value.
	SupportedProtos []string

	// SupportedVersions lists the TLS versions supported by the client.
	// For TLS versions less than 1.3, this is extrapolated from the max
	// version advertised by the client, so values other than the greatest
	// might be rejected if used.
	SupportedVersions []uint16

	// Extensions lists the IDs of the extensions presented by the client
	// in the ClientHello.
	Extensions []uint16

	// Conn is the underlying net.Conn for the connection. Do not read
	// from, or write to, this connection; that will cause the TLS
	// connection to fail.
	Conn net.Conn

	// HelloRetryRequest indicates whether the ClientHello was sent in response
	// to a HelloRetryRequest message.
	HelloRetryRequest bool

	// config is embedded by the GetCertificate or GetConfigForClient caller,
	// for use with SupportsCertificate.
	config *Config

	// isQUIC indicates whether the connection is a QUIC connection.
	isQUIC bool

	// ctx is the context of the handshake that is in progress.
	ctx context.Context
}

// Context returns the context of the handshake that is in progress.
// This context is a child of the context passed to HandshakeContext,
// if any, and is canceled when the handshake concludes.
func (c *ClientHelloInfo) Context() context.Context {
	return c.ctx
}

// CertificateRequestInfo contains information from a server's
// CertificateRequest message, which is used to demand a certificate and proof
// of control from a client.
type CertificateRequestInfo struct {
	// AcceptableCAs contains zero or more, DER-encoded, X.501
	// Distinguished Names. These are the names of root or intermediate CAs
	// that the server wishes the returned certificate to be signed by. An
	// empty slice indicates that the server has no preference.
	AcceptableCAs [][]byte

	// SignatureSchemes lists the signature schemes that the server is
	// willing to verify.
	SignatureSchemes []SignatureScheme

	// Version is the TLS version that was negotiated for this connection.
	Version uint16

	// ctx is the context of the handshake that is in progress.
	ctx context.Context
}

// Context returns the context of the handshake that is in progress.
// This context is a child of the context passed to HandshakeContext,
// if any, and is canceled when the handshake concludes.
func (c *CertificateRequestInfo) Context() context.Context {
	return c.ctx
}

// RenegotiationSupport enumerates the different levels of support for TLS
// renegotiation. TLS renegotiation is the act of performing subsequent
// handshakes on a connection after the first. This significantly complicates
// the state machine and has been the source of numerous, subtle security
// issues. Initiating a renegotiation is not supported, but support for
// accepting renegotiation requests may be enabled.
//
// Even when enabled, the server may not change its identity between handshakes
// (i.e. the leaf certificate must be the same). Additionally, concurrent
// handshake and application data flow is not permitted so renegotiation can
// only be used with protocols that synchronise with the renegotiation, such as
// HTTPS.
//
// Renegotiation is not defined in TLS 1.3.
type RenegotiationSupport int

const (
	// RenegotiateNever disables renegotiation.
	RenegotiateNever RenegotiationSupport = iota

	// RenegotiateOnceAsClient allows a remote server to request
	// renegotiation once per connection.
	RenegotiateOnceAsClient

	// RenegotiateFreelyAsClient allows a remote server to repeatedly
	// request renegotiation.
	RenegotiateFreelyAsClient
)

// A Config structure is used to configure a TLS client or server.
// After one has been passed to a TLS function it must not be
// modified. A Config may be reused; the tls package will also not
// modify it.
type Config struct {
	// Rand provides the source of entropy for the connection.
	// If Rand is nil, TLS uses the cryptographic random reader in package
	// crypto/rand. The Reader must be safe for use by multiple goroutines.
	//
	// Deprecated: this should be left nil in production. Not all TLS
	// configurations are guaranteed to use Rand. Test code can use
	// [testing/cryptotest.SetGlobalRandom] instead.
	Rand io.Reader

	// Time returns the current time as the number of seconds since the epoch.
	// If Time is nil, TLS uses time.Now.
	Time func() time.Time

	// Certificates contains one or more certificate chains to present to the
	// other side of the connection. The first certificate compatible with the
	// peer's requirements is selected automatically.
	//
	// Server configurations must set one of Certificates, GetCertificate or
	// GetConfigForClient. Clients doing client-authentication may set either
	// Certificates or GetClientCertificate.
	//
	// Note: if there are multiple Certificates, and they don't have the
	// optional field Leaf set, certificate selection will incur a significant
	// per-handshake performance cost.
	Certificates []Certificate

	// NameToCertificate maps from a certificate name to an element of
	// Certificates. Note that a certificate name can be of the form
	// '*.example.com' and so doesn't have to be a domain name as such.
	//
	// Deprecated: NameToCertificate only allows associating a single
	// certificate with a given name. Leave this field nil to let the library
	// select the first compatible chain from Certificates.
	NameToCertificate map[string]*Certificate

	// GetCertificate returns a Certificate based on the given
	// ClientHelloInfo. It will only be called if the client supplies SNI
	// information or if Certificates is empty.
	//
	// If GetCertificate is nil or returns nil, then the certificate is
	// retrieved from NameToCertificate. If NameToCertificate is nil, the
	// best element of Certificates will be used.
	//
	// Once a Certificate is returned it should not be modified.
	GetCertificate func(*ClientHelloInfo) (*Certificate, error)

	// GetClientCertificate, if not nil, is called when a server requests a
	// certificate from a client. If set, the contents of Certificates will
	// be ignored.
	//
	// If GetClientCertificate returns an error, the handshake will be
	// aborted and that error will be returned. Otherwise
	// GetClientCertificate must return a non-nil Certificate. If
	// Certificate.Certificate is empty then no certificate will be sent to
	// the server. If this is unacceptable to the server then it may abort
	// the handshake.
	//
	// GetClientCertificate may be called multiple times for the same
	// connection if renegotiation occurs or if TLS 1.3 is in use.
	//
	// Once a Certificate is returned it should not be modified.
	GetClientCertificate func(*CertificateRequestInfo) (*Certificate, error)

	// GetConfigForClient, if not nil, is called after a ClientHello is
	// received from a client. It may return a non-nil Config in order to
	// change the Config that will be used to handle this connection. If
	// the returned Config is nil, the original Config will be used. The
	// Config returned by this callback may not be subsequently modified.
	//
	// If GetConfigForClient is nil, the Config passed to Server() will be
	// used for all connections.
	//
	// If SessionTicketKey is explicitly set on the returned Config, or if
	// SetSessionTicketKeys is called on the returned Config, those keys will
	// be used. Otherwise, the original Config keys will be used (and possibly
	// rotated if they are automatically managed). WARNING: this allows session
	// resumption of connections originally established with the parent (or a
	// sibling) Config, which may bypass the [Config.VerifyPeerCertificate]
	// value of the returned Config.
	GetConfigForClient func(*ClientHelloInfo) (*Config, error)

	// VerifyPeerCertificate, if not nil, is called after normal
	// certificate verification by either a TLS client or server. It
	// receives the raw ASN.1 certificates provided by the peer and also
	// any verified chains that normal processing found. If it returns a
	// non-nil error, the handshake is aborted and that error results.
	//
	// If normal verification fails then the handshake will abort before
	// considering this callback. If normal verification is disabled (on the
	// client when InsecureSkipVerify is set, or on a server when ClientAuth is
	// RequestClientCert or RequireAnyClientCert), then this callback will be
	// considered but the verifiedChains argument will always be nil. When
	// ClientAuth is NoClientCert, this callback is not called on the server.
	// rawCerts may be empty on the server if ClientAuth is RequestClientCert or
	// VerifyClientCertIfGiven.
	//
	// This callback is not invoked on resumed connections. WARNING: this
	// includes connections resumed across Configs returned by [Config.Clone] or
	// [Config.GetConfigForClient] and their parents. If that is not intended,
	// use [Config.VerifyConnection] instead, or set [Config.SessionTicketsDisabled].
	//
	// verifiedChains and its contents should not be modified.
	VerifyPeerCertificate func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error

	// VerifyConnection, if not nil, is called after normal certificate
	// verification and after VerifyPeerCertificate by either a TLS client
	// or server. If it returns a non-nil error, the handshake is aborted
	// and that error results.
	//
	// If normal verification fails then the handshake will abort before
	// considering this callback. This callback will run for all connections,
	// including resumptions, regardless of InsecureSkipVerify or ClientAuth
	// settings.
	VerifyConnection func(ConnectionState) error

	// RootCAs defines the set of root certificate authorities
	// that clients use when verifying server certificates.
	// If RootCAs is nil, TLS uses the host's root CA set.
	RootCAs *x509.CertPool

	// NextProtos is a list of supported application level protocols, in
	// order of preference. If both peers support ALPN, the selected
	// protocol will be one from this list, and the connection will fail
	// if there is no mutually supported protocol. If NextProtos is empty
	// or the peer doesn't support ALPN, the connection will succeed and
	// ConnectionState.NegotiatedProtocol will be empty.
	NextProtos []string

	// ServerName is used to verify the hostname on the returned
	// certificates unless InsecureSkipVerify is given. It is also included
	// in the client's handshake to support virtual hosting unless it is
	// an IP address.
	ServerName string

	// ClientAuth determines the server's policy for
	// TLS Client Authentication. The default is NoClientCert.
	ClientAuth ClientAuthType

	// ClientCAs defines the set of root certificate authorities
	// that servers use if required to verify a client certificate
	// by the policy in ClientAuth.
	ClientCAs *x509.CertPool

	// InsecureSkipVerify controls whether a client verifies the server's
	// certificate chain and host name. If InsecureSkipVerify is true, crypto/tls
	// accepts any certificate presented by the server and any host name in that
	// certificate. In this mode, TLS is susceptible to machine-in-the-middle
	// attacks unless custom verification is used. This should be used only for
	// testing or in combination with VerifyConnection or VerifyPeerCertificate.
	InsecureSkipVerify bool

	// CipherSuites is a list of enabled TLS 1.0–1.2 cipher suites. The order of
	// the list is ignored. Note that TLS 1.3 ciphersuites are not configurable.
	//
	// If CipherSuites is nil, a safe default list is used. The default cipher
	// suites might change over time.
	CipherSuites []uint16

	// PreferServerCipherSuites is a legacy field and has no effect.
	//
	// It used to control whether the server would follow the client's or the
	// server's preference. Servers now select the best mutually supported
	// cipher suite based on logic that takes into account inferred client
	// hardware, server hardware, and security.
	//
	// Deprecated: PreferServerCipherSuites is ignored.
	PreferServerCipherSuites bool

	// SessionTicketsDisabled may be set to true to disable session ticket and
	// PSK (resumption) support. Note that on clients, session ticket support is
	// also disabled if ClientSessionCache is nil.
	SessionTicketsDisabled bool

	// SessionTicketKey is used by TLS servers to provide session resumption.
	// See RFC 5077 and the PSK mode of RFC 8446. If zero, it will be filled
	// with random data before the first server handshake.
	//
	// Deprecated: if this field is left at zero, session ticket keys will be
	// automatically rotated every day and dropped after seven days. For
	// customizing the rotation schedule or synchronizing servers that are
	// terminating connections for the same host, use SetSessionTicketKeys.
	SessionTicketKey [32]byte

	// ClientSessionCache is a cache of ClientSessionState entries for TLS
	// session resumption. It is only used by clients.
	ClientSessionCache ClientSessionCache

	// UnwrapSession is called on the server to turn a ticket/identity
	// previously produced by [WrapSession] into a usable session.
	//
	// UnwrapSession will usually either decrypt a session state in the ticket
	// (for example with [Config.EncryptTicket]), or use the ticket as a handle
	// to recover a previously stored state. It must use [ParseSessionState] to
	// deserialize the session state.
	//
	// If UnwrapSession returns an error, the connection is terminated. If it
	// returns (nil, nil), the session is ignored. crypto/tls may still choose
	// not to resume the returned session.
	UnwrapSession func(identity []byte, cs ConnectionState) (*SessionState, error)

	// WrapSession is called on the server to produce a session ticket/identity.
	//
	// WrapSession must serialize the session state with [SessionState.Bytes].
	// It may then encrypt the serialized state (for example with
	// [Config.DecryptTicket]) and use it as the ticket, or store the state and
	// return a handle for it.
	//
	// If WrapSession returns an error, the connection is terminated.
	//
	// Warning: the return value will be exposed on the wire and to clients in
	// plaintext. The application is in charge of encrypting and authenticating
	// it (and rotating keys) or returning high-entropy identifiers. Failing to
	// do so correctly can compromise current, previous, and future connections
	// depending on the protocol version.
	WrapSession func(ConnectionState, *SessionState) ([]byte, error)

	// MinVersion contains the minimum TLS version that is acceptable.
	//
	// By default, TLS 1.2 is currently used as the minimum. TLS 1.0 is the
	// minimum supported by this package.
	MinVersion uint16

	// MaxVersion contains the maximum TLS version that is acceptable.
	//
	// By default, the maximum version supported by this package is used,
	// which is currently TLS 1.3.
	MaxVersion uint16

	// CurvePreferences contains a set of supported key exchange mechanisms.
	// The name refers to elliptic curves for legacy reasons, see [CurveID].
	// The order of the list is ignored, and key exchange mechanisms are chosen
	// from this list using an internal preference order. If empty, the default
	// will be used.
	//
	// From Go 1.24, the default includes the [X25519MLKEM768] hybrid
	// post-quantum key exchange. To disable it, set CurvePreferences explicitly
	// or use the GODEBUG=tlsmlkem=0 environment variable.
	//
	// From Go 1.26, the default includes the [SecP256r1MLKEM768] and
	// [SecP384r1MLKEM1024] hybrid post-quantum key exchanges, too. To disable
	// them, set CurvePreferences explicitly or use either the
	// GODEBUG=tlsmlkem=0 or the GODEBUG=tlssecpmlkem=0 environment variable.
	CurvePreferences []CurveID

	// DynamicRecordSizingDisabled disables adaptive sizing of TLS records.
	// When true, the largest possible TLS record size is always used. When
	// false, the size of TLS records may be adjusted in an attempt to
	// improve latency.
	DynamicRecordSizingDisabled bool

	// Renegotiation controls what types of renegotiation are supported.
	// The default, none, is correct for the vast majority of applications.
	Renegotiation RenegotiationSupport

	// KeyLogWriter optionally specifies a destination for TLS master secrets
	// in NSS key log format that can be used to allow external programs
	// such as Wireshark to decrypt TLS connections.
	// See https://datatracker.ietf.org/doc/draft-ietf-tls-keylogfile/.
	// Use of KeyLogWriter compromises security and should only be
	// used for debugging.
	KeyLogWriter io.Writer

	// EncryptedClientHelloConfigList is a serialized ECHConfigList. If
	// provided, clients will attempt to connect to servers using Encrypted
	// Client Hello (ECH) using one of the provided ECHConfigs.
	//
	// Servers do not use this field. In order to configure ECH for servers, see
	// the EncryptedClientHelloKeys field.
	//
	// If the list contains no valid ECH configs, the handshake will fail
	// and return an error.
	//
	// If EncryptedClientHelloConfigList is set, MinVersion, if set, must
	// be VersionTLS13.
	//
	// When EncryptedClientHelloConfigList is set, the handshake will only
	// succeed if ECH is successfully negotiated. If the server rejects ECH,
	// an ECHRejectionError error will be returned, which may contain a new
	// ECHConfigList that the server suggests using.
	//
	// How this field is parsed may change in future Go versions, if the
	// encoding described in the final Encrypted Client Hello RFC changes.
	EncryptedClientHelloConfigList []byte

	// EncryptedClientHelloRejectionVerify, if not nil, is called when ECH is
	// rejected by the remote server, in order to verify the ECH provider
	// certificate in the outer ClientHello. If it returns a non-nil error, the
	// handshake is aborted and that error results.
	//
	// On the server side this field is not used.
	//
	// Unlike VerifyPeerCertificate and VerifyConnection, normal certificate
	// verification will not be performed before calling
	// EncryptedClientHelloRejectionVerify.
	//
	// If EncryptedClientHelloRejectionVerify is nil and ECH is rejected, the
	// roots in RootCAs will be used to verify the ECH providers public
	// certificate. VerifyPeerCertificate and VerifyConnection are not called
	// when ECH is rejected, even if set, and InsecureSkipVerify is ignored.
	EncryptedClientHelloRejectionVerify func(ConnectionState) error

	// GetEncryptedClientHelloKeys, if not nil, is called when by a server when
	// a client attempts ECH.
	//
	// If GetEncryptedClientHelloKeys is not nil, [EncryptedClientHelloKeys] is
	// ignored.
	//
	// If GetEncryptedClientHelloKeys returns an error, the handshake will be
	// aborted and the error will be returned. Otherwise,
	// GetEncryptedClientHelloKeys must return a non-nil slice of
	// [EncryptedClientHelloKey] that represents the acceptable ECH keys.
	//
	// For further details, see [EncryptedClientHelloKeys].
	GetEncryptedClientHelloKeys func(*ClientHelloInfo) ([]EncryptedClientHelloKey, error)

	// EncryptedClientHelloKeys are the ECH keys to use when a client
	// attempts ECH.
	//
	// If EncryptedClientHelloKeys is set, MinVersion, if set, must be
	// VersionTLS13.
	//
	// If a client attempts ECH, but it is rejected by the server, the server
	// will send a list of configs to retry based on the set of
	// EncryptedClientHelloKeys which have the SendAsRetry field set.
	//
	// If GetEncryptedClientHelloKeys is non-nil, EncryptedClientHelloKeys is
	// ignored.
	//
	// On the client side, this field is ignored. In order to configure ECH for
	// clients, see the EncryptedClientHelloConfigList field.
	EncryptedClientHelloKeys []EncryptedClientHelloKey

	// mutex protects sessionTicketKeys and autoSessionTicketKeys.
	mutex sync.RWMutex
	// sessionTicketKeys contains zero or more ticket keys. If set, it means
	// the keys were set with SessionTicketKey or SetSessionTicketKeys. The
	// first key is used for new tickets and any subsequent keys can be used to
	// decrypt old tickets. The slice contents are not protected by the mutex
	// and are immutable.
	sessionTicketKeys []ticketKey
	// autoSessionTicketKeys is like sessionTicketKeys but is owned by the
	// auto-rotation logic. See Config.ticketKeys.
	autoSessionTicketKeys []ticketKey
}

// EncryptedClientHelloKey holds a private key that is associated
// with a specific ECH config known to a client.
type EncryptedClientHelloKey struct {
	// Config should be a marshalled ECHConfig associated with PrivateKey. This
	// must match the config provided to clients byte-for-byte. The config must
	// use as KEM one of
	//
	//   - DHKEM(P-256, HKDF-SHA256) (0x0010)
	//   - DHKEM(P-384, HKDF-SHA384) (0x0011)
	//   - DHKEM(P-521, HKDF-SHA512) (0x0012)
	//   - DHKEM(X25519, HKDF-SHA256) (0x0020)
	//   - ML-KEM-768 (0x0041)
	//   - ML-KEM-1024 (0x0042)
	//   - MLKEM768-P256 (0x0050)
	//   - MLKEM1024-P384 (0x0051)
	//   - MLKEM768-X25519 (0x647a)
	//
	// and as KDF one of
	//
	//   - HKDF-SHA256 (0x0001)
	//   - HKDF-SHA384 (0x0002)
	//   - HKDF-SHA512 (0x0003)
	//
	// and as AEAD one of
	//
	//   - AES-128-GCM (0x0001)
	//   - AES-256-GCM (0x0002)
	//   - ChaCha20Poly1305 (0x0003)
	//
	Config []byte
	// PrivateKey should be a marshalled private key, in the format expected by
	// HPKE's DeserializePrivateKey (see RFC 9180), for the KEM used in Config.
	PrivateKey []byte
	// SendAsRetry indicates if Config should be sent as part of the list of
	// retry configs when ECH is requested by the client but rejected by the
	// server.
	SendAsRetry bool
}

const (
	// ticketKeyLifetime is how long a ticket key remains valid and can be used to
	// resume a client connection.
	ticketKeyLifetime = 7 * 24 * time.Hour // 7 days

	// ticketKeyRotation is how often the server should rotate the session ticket key
	// that is used for new tickets.
	ticketKeyRotation = 24 * time.Hour
)

// ticketKey is the internal representation of a session ticket key.
type ticketKey struct {
	aesKey  [16]byte
	hmacKey [16]byte
	// created is the time at which this ticket key was created. See Config.ticketKeys.
	created time.Time
}

// ticketKeyFromBytes converts from the external representation of a session
// ticket key to a ticketKey. Externally, session ticket keys are 32 random
// bytes and this function expands that into sufficient name and key material.
func (c *Config) ticketKeyFromBytes(b [32]byte) (key ticketKey) {
	hashed := sha512.Sum512(b[:])
	// The first 16 bytes of the hash used to be exposed on the wire as a ticket
	// prefix. They MUST NOT be used as a secret. In the future, it would make
	// sense to use a proper KDF here, like HKDF with a fixed salt.
	const legacyTicketKeyNameLen = 16
	copy(key.aesKey[:], hashed[legacyTicketKeyNameLen:])
	copy(key.hmacKey[:], hashed[legacyTicketKeyNameLen+len(key.aesKey):])
	key.created = c.time()
	return key
}

// maxSessionTicketLifetime is the maximum allowed lifetime of a TLS 1.3 session
// ticket, and the lifetime we set for all tickets we send.
const maxSessionTicketLifetime = 7 * 24 * time.Hour

// Clone returns a shallow clone of c or nil if c is nil. It is safe to clone a
// [Config] that is being used concurrently by a TLS client or server.
//
// The returned Config can share session ticket keys with the original Config,
// which means connections could be resumed across the two Configs. WARNING:
// [Config.VerifyPeerCertificate] does not get called on resumed connections,
// including connections that were originally established on the parent Config.
// If that is not intended, use [Config.VerifyConnection] instead, or set
// [Config.SessionTicketsDisabled].
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return &Config{
		Rand:                                c.Rand,
		Time:                                c.Time,
		Certificates:                        c.Certificates,
		NameToCertificate:                   c.NameToCertificate,
		GetCertificate:                      c.GetCertificate,
		GetClientCertificate:                c.GetClientCertificate,
		GetConfigForClient:                  c.GetConfigForClient,
		GetEncryptedClientHelloKeys:         c.GetEncryptedClientHelloKeys,
		VerifyPeerCertificate:               c.VerifyPeerCertificate,
		VerifyConnection:                    c.VerifyConnection,
		RootCAs:                             c.RootCAs,
		NextProtos:                          c.NextProtos,
		ServerName:                          c.ServerName,
		ClientAuth:                          c.ClientAuth,
		ClientCAs:                           c.ClientCAs,
		InsecureSkipVerify:                  c.InsecureSkipVerify,
		CipherSuites:                        c.CipherSuites,
		PreferServerCipherSuites:            c.PreferServerCipherSuites,
		SessionTicketsDisabled:              c.SessionTicketsDisabled,
		SessionTicketKey:                    c.SessionTicketKey,
		ClientSessionCache:                  c.ClientSessionCache,
		UnwrapSession:                       c.UnwrapSession,
		WrapSession:                         c.WrapSession,
		MinVersion:                          c.MinVersion,
		MaxVersion:                          c.MaxVersion,
		CurvePreferences:                    c.CurvePreferences,
		DynamicRecordSizingDisabled:         c.DynamicRecordSizingDisabled,
		Renegotiation:                       c.Renegotiation,
		KeyLogWriter:                        c.KeyLogWriter,
		EncryptedClientHelloConfigList:      c.EncryptedClientHelloConfigList,
		EncryptedClientHelloRejectionVerify: c.EncryptedClientHelloRejectionVerify,
		EncryptedClientHelloKeys:            c.EncryptedClientHelloKeys,
		sessionTicketKeys:                   c.sessionTicketKeys,
		autoSessionTicketKeys:               c.autoSessionTicketKeys,
	}
}

// deprecatedSessionTicketKey is set as the prefix of SessionTicketKey if it was
// randomized for backwards compatibility but is not in use.
var deprecatedSessionTicketKey = []byte("DEPRECATED")

// initLegacySessionTicketKeyRLocked ensures the legacy SessionTicketKey field is
// randomized if empty, and that sessionTicketKeys is populated from it otherwise.
func (c *Config) initLegacySessionTicketKeyRLocked() {
	// Don't write if SessionTicketKey is already defined as our deprecated string,
	// or if it is defined by the user but sessionTicketKeys is already set.
	if c.SessionTicketKey != [32]byte{} &&
		(bytes.HasPrefix(c.SessionTicketKey[:], deprecatedSessionTicketKey) || len(c.sessionTicketKeys) > 0) {
		return
	}

	// We need to write some data, so get an exclusive lock and re-check any conditions.
	c.mutex.RUnlock()
	defer c.mutex.RLock()
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.SessionTicketKey == [32]byte{} {
		if _, err := io.ReadFull(c.rand(), c.SessionTicketKey[:]); err != nil {
			panic(fmt.Sprintf("tls: unable to generate random session ticket key: %v", err))
		}
		// Write the deprecated prefix at the beginning so we know we created
		// it. This key with the DEPRECATED prefix isn't used as an actual
		// session ticket key, and is only randomized in case the application
		// reuses it for some reason.
		copy(c.SessionTicketKey[:], deprecatedSessionTicketKey)
	} else if !bytes.HasPrefix(c.SessionTicketKey[:], deprecatedSessionTicketKey) && len(c.sessionTicketKeys) == 0 {
		c.sessionTicketKeys = []ticketKey{c.ticketKeyFromBytes(c.SessionTicketKey)}
	}
}

// ticketKeys returns the ticketKeys for this connection.
// If configForClient has explicitly set keys, those will
// be returned. Otherwise, the keys on c will be used and
// may be rotated if auto-managed.
// During rotation, any expired session ticket keys are deleted from
// c.sessionTicketKeys. If the session ticket key that is currently
// encrypting tickets (ie. the first ticketKey in c.sessionTicketKeys)
// is not fresh, then a new session ticket key will be
// created and prepended to c.sessionTicketKeys.
func (c *Config) ticketKeys(configForClient *Config) []ticketKey {
	// If the ConfigForClient callback returned a Config with explicitly set
	// keys, use those, otherwise just use the original Config.
	if configForClient != nil {
		configForClient.mutex.RLock()
		if configForClient.SessionTicketsDisabled {
			configForClient.mutex.RUnlock()
			return nil
		}
		configForClient.initLegacySessionTicketKeyRLocked()
		if len(configForClient.sessionTicketKeys) != 0 {
			ret := configForClient.sessionTicketKeys
			configForClient.mutex.RUnlock()
			return ret
		}
		configForClient.mutex.RUnlock()
	}

	c.mutex.RLock()
	defer c.mutex.RUnlock()
	if c.SessionTicketsDisabled {
		return nil
	}
	c.initLegacySessionTicketKeyRLocked()
	if len(c.sessionTicketKeys) != 0 {
		return c.sessionTicketKeys
	}
	// Fast path for the common case where the key is fresh enough.
	if len(c.autoSessionTicketKeys) > 0 && c.time().Sub(c.autoSessionTicketKeys[0].created) < ticketKeyRotation {
		return c.autoSessionTicketKeys
	}

	// autoSessionTicketKeys are managed by auto-rotation.
	c.mutex.RUnlock()
	defer c.mutex.RLock()
	c.mutex.Lock()
	defer c.mutex.Unlock()
	// Re-check the condition in case it changed since obtaining the new lock.
	if len(c.autoSessionTicketKeys) == 0 || c.time().Sub(c.autoSessionTicketKeys[0].created) >= ticketKeyRotation {
		var newKey [32]byte
		if _, err := io.ReadFull(c.rand(), newKey[:]); err != nil {
			panic(fmt.Sprintf("unable to generate random session ticket key: %v", err))
		}
		valid := make([]ticketKey, 0, len(c.autoSessionTicketKeys)+1)
		valid = append(valid, c.ticketKeyFromBytes(newKey))
		for _, k := range c.autoSessionTicketKeys {
			// While rotating the current key, also remove any expired ones.
			if c.time().Sub(k.created) < ticketKeyLifetime {
				valid = append(valid, k)
			}
		}
		c.autoSessionTicketKeys = valid
	}
	return c.autoSessionTicketKeys
}

// SetSessionTicketKeys updates the session ticket keys for a server.
//
// The first key will be used when creating new tickets, while all keys can be
// used for decrypting tickets. It is safe to call this function while the
// server is running in order to rotate the session ticket keys. The function
// will panic if keys is empty.
//
// Calling this function will turn off automatic session ticket key rotation.
//
// If multiple servers are terminating connections for the same host they should
// all have the same session ticket keys. If the session ticket keys leaks,
// previously recorded and future TLS connections using those keys might be
// compromised.
func (c *Config) SetSessionTicketKeys(keys [][32]byte) {
	if len(keys) == 0 {
		panic("tls: keys must have at least one key")
	}

	newKeys := make([]ticketKey, len(keys))
	for i, bytes := range keys {
		newKeys[i] = c.ticketKeyFromBytes(bytes)
	}

	c.mutex.Lock()
	c.sessionTicketKeys = newKeys
	c.mutex.Unlock()
}

func (c *Config) rand() io.Reader {
	r := c.Rand
	if r == nil {
		return rand.Reader
	}
	return r
}

func (c *Config) time() time.Time {
	t := c.Time
	if t == nil {
		t = time.Now
	}
	return t()
}

func (c *Config) cipherSuites(aesGCMPreferred bool) []uint16 {
	var cipherSuites []uint16
	if c.CipherSuites == nil {
		cipherSuites = defaultCipherSuites(aesGCMPreferred)
	} else {
		cipherSuites = supportedCipherSuites(aesGCMPreferred)
		cipherSuites = slices.DeleteFunc(cipherSuites, func(id uint16) bool {
			return !slices.Contains(c.CipherSuites, id)
		})
	}
	if fips140tls.Required() {
		cipherSuites = slices.DeleteFunc(cipherSuites, func(id uint16) bool {
			return !slices.Contains(allowedCipherSuitesFIPS, id)
		})
	}
	return cipherSuites
}

// supportedCipherSuites returns the supported TLS 1.0–1.2 cipher suites in an
// undefined order. For preference ordering, use [Config.cipherSuites].
func (c *Config) supportedCipherSuites() []uint16 {
	return c.cipherSuites(false)
}

var supportedVersions = []uint16{
	VersionTLS13,
	VersionTLS12,
	VersionTLS11,
	VersionTLS10,
}

// roleClient and roleServer are meant to call supportedVersions and parents
// with more readability at the callsite.
const roleClient = true
const roleServer = false

// supportedVersions returns the list of supported TLS versions, sorted from
// highest to lowest (and hence also in preference order).
func (c *Config) supportedVersions(isClient, isQUIC bool) []uint16 {
	versions := make([]uint16, 0, len(supportedVersions))
	for _, v := range supportedVersions {
		if fips140tls.Required() && !slices.Contains(allowedSupportedVersionsFIPS, v) {
			continue
		}
		if (c == nil || c.MinVersion == 0) && v < VersionTLS12 {
			continue
		}
		if isClient && c.EncryptedClientHelloConfigList != nil && v < VersionTLS13 {
			continue
		}
		if c != nil && c.MinVersion != 0 && v < c.MinVersion {
			continue
		}
		if c != nil && c.MaxVersion != 0 && v > c.MaxVersion {
			continue
		}
		if isQUIC && v < VersionTLS13 {
			continue
		}
		versions = append(versions, v)
	}
	return versions
}

func (c *Config) maxSupportedVersion(isClient, isQUIC bool) uint16 {
	supportedVersions := c.supportedVersions(isClient, isQUIC)
	if len(supportedVersions) == 0 {
		return 0
	}
	return supportedVersions[0]
}

// supportedVersionsFromMax returns a list of supported versions derived from a
// legacy maximum version value. Note that only versions supported by this
// library are returned. Any newer peer will use supportedVersions anyway.
func supportedVersionsFromMax(maxVersion uint16) []uint16 {
	versions := make([]uint16, 0, len(supportedVersions))
	for _, v := range supportedVersions {
		if v > maxVersion {
			continue
		}
		versions = append(versions, v)
	}
	return versions
}

func (c *Config) curvePreferences(version uint16) []CurveID {
	return slices.DeleteFunc(curvePreferenceOrder(), func(x CurveID) bool {
		return !c.supportsCurve(version, x)
	})
}

func (c *Config) supportsCurve(version uint16, x CurveID) bool {
	if c != nil && len(c.CurvePreferences) != 0 {
		if !slices.Contains(c.CurvePreferences, x) {
			return false
		}
		// Ignore unimplemented entries in c.CurvePreferences.
		if !slices.Contains(curvePreferenceOrder(), x) {
			return false
		}
	} else {
		if !defaultCurveEnabled(x) {
			return false
		}
	}
	if fips140tls.Required() && !slices.Contains(allowedCurvePreferencesFIPS, x) {
		return false
	}
	if version < VersionTLS13 && isTLS13OnlyKeyExchange(x) {
		return false
	}
	return true
}

// mutualVersion returns the protocol version to use given the advertised
// versions of the peer. The highest supported version is preferred.
func (c *Config) mutualVersion(isClient, isQUIC bool, peerVersions []uint16) (uint16, bool) {
	supportedVersions := c.supportedVersions(isClient, isQUIC)
	for _, v := range supportedVersions {
		if slices.Contains(peerVersions, v) {
			return v, true
		}
	}
	return 0, false
}

// errNoCertificates should be an internal detail,
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/xtls/xray-core
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
//go:linkname errNoCertificates
var errNoCertificates = errors.New("tls: no certificates configured")

// getCertificate returns the best certificate for the given ClientHelloInfo,
// defaulting to the first element of c.Certificates.
func (c *Config) getCertificate(clientHello *ClientHelloInfo) (*Certificate, error) {
	if c.GetCertificate != nil &&
		(len(c.Certificates) == 0 || len(clientHello.ServerName) > 0) {
		cert, err := c.GetCertificate(clientHello)
		if cert != nil || err != nil {
			return cert, err
		}
	}

	if len(c.Certificates) == 0 {
		return nil, errNoCertificates
	}

	if len(c.Certificates) == 1 {
		// There's only one choice, so no point doing any work.
		return &c.Certificates[0], nil
	}

	if c.NameToCertificate != nil {
		name := strings.ToLower(clientHello.ServerName)
		if cert, ok := c.NameToCertificate[name]; ok {
			return cert, nil
		}
		if len(name) > 0 {
			labels := strings.Split(name, ".")
			labels[0] = "*"
			wildcardName := strings.Join(labels, ".")
			if cert, ok := c.NameToCertificate[wildcardName]; ok {
				return cert, nil
			}
		}
	}

	for _, cert := range c.Certificates {
		if err := clientHello.SupportsCertificate(&cert); err == nil {
			return &cert, nil
		}
	}

	// If nothing matches, return the first certificate.
	return &c.Certificates[0], nil
}

// SupportsCertificate returns nil if the provided certificate is supported by
// the client that sent the ClientHello. Otherwise, it returns an error
// describing the reason for the incompatibility.
//
// If this [ClientHelloInfo] was passed to a GetConfigForClient or GetCertificate
// callback, this method will take into account the associated [Config]. Note that
// if GetConfigForClient returns a different [Config], the change can't be
// accounted for by this method.
//
// This function will call x509.ParseCertificate unless c.Leaf is set, which can
// incur a significant performance cost.
func (chi *ClientHelloInfo) SupportsCertificate(c *Certificate) error {
	// Note we don't currently support certificate_authorities nor
	// signature_algorithms_cert, and don't check the algorithms of the
	// signatures on the chain (which anyway are a SHOULD, see RFC 8446,
	// Section 4.4.2.2).

	config := chi.config
	if config == nil {
		config = &Config{}
	}
	vers, ok := config.mutualVersion(roleServer, chi.isQUIC, chi.SupportedVersions)
	if !ok {
		return errors.New("no mutually supported protocol versions")
	}

	// If the client specified the name they are trying to connect to, the
	// certificate needs to be valid for it.
	if chi.ServerName != "" {
		x509Cert, err := c.leaf()
		if err != nil {
			return fmt.Errorf("failed to parse certificate: %w", err)
		}
		if err := x509Cert.VerifyHostname(chi.ServerName); err != nil {
			return fmt.Errorf("certificate is not valid for requested server name: %w", err)
		}
	}

	// supportsRSAFallback returns nil if the certificate and connection support
	// the static RSA key exchange, and unsupported otherwise. The logic for
	// supporting static RSA is completely disjoint from the logic for
	// supporting signed key exchanges, so we just check it as a fallback.
	supportsRSAFallback := func(unsupported error) error {
		// TLS 1.3 dropped support for the static RSA key exchange.
		if vers == VersionTLS13 {
			return unsupported
		}
		// The static RSA key exchange works by decrypting a challenge with the
		// RSA private key, not by signing, so check the PrivateKey implements
		// crypto.Decrypter, like *rsa.PrivateKey does.
		if priv, ok := c.PrivateKey.(crypto.Decrypter); ok {
			if _, ok := priv.Public().(*rsa.PublicKey); !ok {
				return unsupported
			}
		} else {
			return unsupported
		}
		// Finally, there needs to be a mutual cipher suite that uses the static
		// RSA key exchange instead of ECDHE.
		rsaCipherSuite := selectCipherSuite(chi.CipherSuites, config.supportedCipherSuites(), func(c *cipherSuite) bool {
			if c.flags&suiteECDHE != 0 {
				return false
			}
			if vers < VersionTLS12 && c.flags&suiteTLS12 != 0 {
				return false
			}
			return true
		})
		if rsaCipherSuite == nil {
			return unsupported
		}
		return nil
	}

	// If the client sent the signature_algorithms extension, ensure it supports
	// schemes we can use with this certificate and TLS version.
	if len(chi.SignatureSchemes) > 0 {
		if _, err := selectSignatureScheme(vers, c, chi.SignatureSchemes); err != nil {
			return supportsRSAFallback(err)
		}
	}

	// In TLS 1.3 we are done because supported_groups is only relevant to the
	// ECDHE computation, point format negotiation is removed, cipher suites are
	// only relevant to the AEAD choice, and static RSA does not exist.
	if vers == VersionTLS13 {
		return nil
	}

	// The only signed key exchange we support is ECDHE.
	ecdheSupported, err := supportsECDHE(config, vers, chi.SupportedCurves, chi.SupportedPoints)
	if err != nil {
		return err
	}
	if !ecdheSupported {
		return supportsRSAFallback(errors.New("client doesn't support ECDHE, can only use legacy RSA key exchange"))
	}

	var ecdsaCipherSuite bool
	if priv, ok := c.PrivateKey.(crypto.Signer); ok {
		switch pub := priv.Public().(type) {
		case *ecdsa.PublicKey:
			var curve CurveID
			switch pub.Curve {
			case elliptic.P256():
				curve = CurveP256
			case elliptic.P384():
				curve = CurveP384
			case elliptic.P521():
				curve = CurveP521
			default:
				return supportsRSAFallback(unsupportedCertificateError(c))
			}
			var curveOk bool
			for _, c := range chi.SupportedCurves {
				if c == curve && config.supportsCurve(vers, c) {
					curveOk = true
					break
				}
			}
			if !curveOk {
				return errors.New("client doesn't support certificate curve")
			}
			ecdsaCipherSuite = true
		case ed25519.PublicKey:
			if vers < VersionTLS12 || len(chi.SignatureSchemes) == 0 {
				return errors.New("connection doesn't support Ed25519")
			}
			ecdsaCipherSuite = true
		case *mldsa.PublicKey:
			// ML-DSA requires TLS 1.3, which we already excluded above.
			return errors.New("connection doesn't support ML-DSA")
		case *rsa.PublicKey:
		default:
			return supportsRSAFallback(unsupportedCertificateError(c))
		}
	} else {
		return supportsRSAFallback(unsupportedCertificateError(c))
	}

	// Make sure that there is a mutually supported cipher suite that works with
	// this certificate. Cipher suite selection will then apply the logic in
	// reverse to pick it. See also serverHandshakeState.cipherSuiteOk.
	cipherSuite := selectCipherSuite(chi.CipherSuites, config.supportedCipherSuites(), func(c *cipherSuite) bool {
		if c.flags&suiteECDHE == 0 {
			return false
		}
		if c.flags&suiteECSign != 0 {
			if !ecdsaCipherSuite {
				return false
			}
		} else {
			if ecdsaCipherSuite {
				return false
			}
		}
		if vers < VersionTLS12 && c.flags&suiteTLS12 != 0 {
			return false
		}
		return true
	})
	if cipherSuite == nil {
		return supportsRSAFallback(errors.New("client doesn't support any cipher suites compatible with the certificate"))
	}

	return nil
}

// SupportsCertificate returns nil if the provided certificate is supported by
// the server that sent the CertificateRequest. Otherwise, it returns an error
// describing the reason for the incompatibility.
func (cri *CertificateRequestInfo) SupportsCertificate(c *Certificate) error {
	if _, err := selectSignatureScheme(cri.Version, c, cri.SignatureSchemes); err != nil {
		return err
	}

	if len(cri.AcceptableCAs) == 0 {
		return nil
	}

	for j, cert := range c.Certificate {
		x509Cert := c.Leaf
		// Parse the certificate if this isn't the leaf node, or if
		// chain.Leaf was nil.
		if j != 0 || x509Cert == nil {
			var err error
			if x509Cert, err = x509.ParseCertificate(cert); err != nil {
				return fmt.Errorf("failed to parse certificate #%d in the chain: %w", j, err)
			}
		}

		for _, ca := range cri.AcceptableCAs {
			if bytes.Equal(x509Cert.RawIssuer, ca) {
				return nil
			}
		}
	}
	return errors.New("chain is not signed by an acceptable CA")
}

// BuildNameToCertificate parses c.Certificates and builds c.NameToCertificate
// from the CommonName and SubjectAlternateName fields of each of the leaf
// certificates.
//
// Deprecated: NameToCertificate only allows associating a single certificate
// with a given name. Leave that field nil to let the library select the first
// compatible chain from Certificates.
func (c *Config) BuildNameToCertificate() {
	c.NameToCertificate = make(map[string]*Certificate)
	for i := range c.Certificates {
		cert := &c.Certificates[i]
		x509Cert, err := cert.leaf()
		if err != nil {
			continue
		}
		// If SANs are *not* present, some clients will consider the certificate
		// valid for the name in the Common Name.
		if x509Cert.Subject.CommonName != "" && len(x509Cert.DNSNames) == 0 {
			c.NameToCertificate[x509Cert.Subject.CommonName] = cert
		}
		for _, san := range x509Cert.DNSNames {
			c.NameToCertificate[san] = cert
		}
	}
}

const (
	keyLogLabelTLS12           = "CLIENT_RANDOM"
	keyLogLabelClientHandshake = "CLIENT_HANDSHAKE_TRAFFIC_SECRET"
	keyLogLabelServerHandshake = "SERVER_HANDSHAKE_TRAFFIC_SECRET"
	keyLogLabelClientTraffic   = "CLIENT_TRAFFIC_SECRET_0"
	keyLogLabelServerTraffic   = "SERVER_TRAFFIC_SECRET_0"
)

func (c *Config) writeKeyLog(label string, clientRandom, secret []byte) error {
	if c.KeyLogWriter == nil {
		return nil
	}

	logLine := fmt.Appendf(nil, "%s %x %x\n", label, clientRandom, secret)

	writerMutex.Lock()
	_, err := c.KeyLogWriter.Write(logLine)
	writerMutex.Unlock()

	if err != nil {
		return fmt.Errorf("tls: KeyLogWriter: %w", err)
	}
	return nil
}

// writerMutex protects all KeyLogWriters globally. It is rarely enabled,
// and is only for debugging, so a global mutex saves space.
var writerMutex sync.Mutex

// A Certificate is a chain of one or more certificates, leaf first.
type Certificate struct {
	Certificate [][]byte
	// PrivateKey contains the private key corresponding to the public key in
	// Leaf. This must implement [crypto.Signer] with an RSA, ECDSA, Ed25519
	// (TLS 1.2+), or ML-DSA (TLS 1.3) PublicKey.
	//
	// For a server up to TLS 1.2, it can also implement crypto.Decrypter with
	// an RSA PublicKey.
	//
	// If it implements [crypto.MessageSigner], SignMessage will be used instead
	// of Sign for TLS 1.2 and later.
	PrivateKey crypto.PrivateKey
	// SupportedSignatureAlgorithms is an optional list restricting what
	// signature algorithms the PrivateKey can be used for.
	SupportedSignatureAlgorithms []SignatureScheme
	// OCSPStaple contains an optional OCSP response which will be served
	// to clients that request it.
	OCSPStaple []byte
	// SignedCertificateTimestamps contains an optional list of Signed
	// Certificate Timestamps which will be served to clients that request it.
	SignedCertificateTimestamps [][]byte
	// Leaf is the parsed form of the leaf certificate, which may be initialized
	// using x509.ParseCertificate to reduce per-handshake processing. If nil,
	// the leaf certificate will be parsed as needed.
	Leaf *x509.Certificate
}

// leaf returns the parsed leaf certificate, either from c.Leaf or by parsing
// the corresponding c.Certificate[0].
func (c *Certificate) leaf() (*x509.Certificate, error) {
	if c.Leaf != nil {
		return c.Leaf, nil
	}
	return x509.ParseCertificate(c.Certificate[0])
}

type handshakeMessage interface {
	marshal() ([]byte, error)
	unmarshal([]byte) bool
}

type handshakeMessageWithOriginalBytes interface {
	handshakeMessage

	// originalBytes should return the original bytes that were passed to
	// unmarshal to create the message. If the message was not produced by
	// unmarshal, it should return nil.
	originalBytes() []byte
}

// lruSessionCache is a ClientSessionCache implementation that uses an LRU
// caching strategy.
type lruSessionCache struct {
	sync.Mutex

	m        map[string]*list.Element
	q        *list.List
	capacity int
}

type lruSessionCacheEntry struct {
	sessionKey string
	state      *ClientSessionState
}

// NewLRUClientSessionCache returns a [ClientSessionCache] with the given
// capacity that uses an LRU strategy. If capacity is < 1, a default capacity
// is used instead.
func NewLRUClientSessionCache(capacity int) ClientSessionCache {
	const defaultSessionCacheCapacity = 64

	if capacity < 1 {
		capacity = defaultSessionCacheCapacity
	}
	return &lruSessionCache{
		m:        make(map[string]*list.Element),
		q:        list.New(),
		capacity: capacity,
	}
}

// Put adds the provided (sessionKey, cs) pair to the cache. If cs is nil, the entry
// corresponding to sessionKey is removed from the cache instead.
func (c *lruSessionCache) Put(sessionKey string, cs *ClientSessionState) {
	c.Lock()
	defer c.Unlock()

	if elem, ok := c.m[sessionKey]; ok {
		if cs == nil {
			c.q.Remove(elem)
			delete(c.m, sessionKey)
		} else {
			entry := elem.Value.(*lruSessionCacheEntry)
			entry.state = cs
			c.q.MoveToFront(elem)
		}
		return
	}

	if c.q.Len() < c.capacity {
		entry := &lruSessionCacheEntry{sessionKey, cs}
		c.m[sessionKey] = c.q.PushFront(entry)
		return
	}

	elem := c.q.Back()
	entry := elem.Value.(*lruSessionCacheEntry)
	delete(c.m, entry.sessionKey)
	entry.sessionKey = sessionKey
	entry.state = cs
	c.q.MoveToFront(elem)
	c.m[sessionKey] = elem
}

// Get returns the [ClientSessionState] value associated with a given key. It
// returns (nil, false) if no value is found.
func (c *lruSessionCache) Get(sessionKey string) (*ClientSessionState, bool) {
	c.Lock()
	defer c.Unlock()

	if elem, ok := c.m[sessionKey]; ok {
		c.q.MoveToFront(elem)
		return elem.Value.(*lruSessionCacheEntry).state, true
	}
	return nil, false
}

var emptyConfig Config

func defaultConfig() *Config {
	return &emptyConfig
}

func unexpectedMessageError(wanted, got any) error {
	return fmt.Errorf("tls: received unexpected handshake message of type %T when waiting for %T", got, wanted)
}

var testingOnlySupportedSignatureAlgorithms []SignatureScheme

// supportedSignatureAlgorithms returns the supported signature algorithms for
// the given range of TLS versions, to advertise in ClientHello and
// CertificateRequest messages. An algorithm is included if it is enabled at any
// version in the range.
func supportedSignatureAlgorithms(minVers, maxVers uint16) []SignatureScheme {
	sigAlgs := defaultSupportedSignatureAlgorithms()
	if testingOnlySupportedSignatureAlgorithms != nil {
		sigAlgs = slices.Clone(testingOnlySupportedSignatureAlgorithms)
	}
	return slices.DeleteFunc(sigAlgs, func(s SignatureScheme) bool {
		for v := minVers; v <= maxVers; v++ {
			if !isDisabledSignatureAlgorithm(v, s, false) {
				return false
			}
		}
		return true
	})
}

var tlssha1 = godebug.New("tlssha1")

func isDisabledSignatureAlgorithm(version uint16, s SignatureScheme, isCert bool) bool {
	if fips140tls.Required() && !slices.Contains(allowedSignatureAlgorithmsFIPS, s) {
		return true
	}

	switch s {
	case MLDSA44, MLDSA65, MLDSA87:
		// ML-DSA is not available in FIPS 140-3 module v1.0.0.
		if fips140.Version() == "v1.0.0" {
			return true
		}
		// ML-DSA codepoints are only defined for TLS 1.3.
		if version < VersionTLS13 {
			return true
		}
	}

	// For the _cert extension we include all algorithms, including SHA-1 and
	// PKCS#1 v1.5, because it's more likely that something on our side will be
	// willing to accept a *-with-SHA1 certificate (e.g. with a custom
	// VerifyConnection or by a direct match with the CertPool), than that the
	// peer would have a better certificate but is just choosing not to send it.
	// crypto/x509 will refuse to verify important SHA-1 signatures anyway.
	if isCert {
		return false
	}

	// TLS 1.3 removed support for PKCS#1 v1.5 and SHA-1 signatures,
	// and Go 1.25 removed support for SHA-1 signatures in TLS 1.2.
	if version > VersionTLS12 {
		sigType, sigHash, _ := typeAndHashFromSignatureScheme(s)
		if sigType == signaturePKCS1v15 || sigHash == crypto.SHA1 {
			return true
		}
	} else if tlssha1.Value() != "1" {
		_, sigHash, _ := typeAndHashFromSignatureScheme(s)
		if sigHash == crypto.SHA1 {
			return true
		}
	}

	return false
}

// supportedSignatureAlgorithmsCert returns the supported algorithms for
// signatures in certificates.
func supportedSignatureAlgorithmsCert(minVers, maxVers uint16) []SignatureScheme {
	sigAlgs := defaultSupportedSignatureAlgorithms()
	return slices.DeleteFunc(sigAlgs, func(s SignatureScheme) bool {
		for v := minVers; v <= maxVers; v++ {
			if !isDisabledSignatureAlgorithm(v, s, true) {
				return false
			}
		}
		return true
	})
}

func isSupportedSignatureAlgorithm(sigAlg SignatureScheme, supportedSignatureAlgorithms []SignatureScheme) bool {
	return slices.Contains(supportedSignatureAlgorithms, sigAlg)
}

// CertificateVerificationError is returned when certificate verification fails during the handshake.
type CertificateVerificationError struct {
	// UnverifiedCertificates and its contents should not be modified.
	UnverifiedCertificates []*x509.Certificate
	Err                    error
}

func (e *CertificateVerificationError) Error() string {
	return fmt.Sprintf("tls: failed to verify certificate: %s", e.Err)
}

func (e *CertificateVerificationError) Unwrap() error {
	return e.Err
}

// fipsAllowedChains returns chains that are allowed to be used in a TLS connection
// based on the current fips140tls enforcement setting.
//
// If fips140tls is not required, the chains are returned as-is with no processing.
// Otherwise, the returned chains are filtered to only those allowed by FIPS 140-3.
// If this results in no chains it returns an error.
func fipsAllowedChains(chains [][]*x509.Certificate) ([][]*x509.Certificate, error) {
	if !fips140tls.Required() {
		return chains, nil
	}

	permittedChains := make([][]*x509.Certificate, 0, len(chains))
	for _, chain := range chains {
		if fipsAllowChain(chain) {
			permittedChains = append(permittedChains, chain)
		}
	}

	if len(permittedChains) == 0 {
		return nil, errors.New("tls: no FIPS compatible certificate chains found")
	}

	return permittedChains, nil
}

func fipsAllowChain(chain []*x509.Certificate) bool {
	if len(chain) == 0 {
		return false
	}

	for _, cert := range chain {
		if !isCertificateAllowedFIPS(cert) {
			return false
		}
	}

	return true
}

// anyValidVerifiedChain reports if at least one of the chains in verifiedChains
// is valid, as indicated by none of the certificates being expired and the root
// being in opts.Roots (or in the system root pool if opts.Roots is nil). If
// verifiedChains is empty, it returns false.
func anyValidVerifiedChain(verifiedChains [][]*x509.Certificate, opts x509.VerifyOptions) bool {
	for _, chain := range verifiedChains {
		if len(chain) == 0 {
			continue
		}
		if slices.ContainsFunc(chain, func(cert *x509.Certificate) bool {
			return opts.CurrentTime.Before(cert.NotBefore) || opts.CurrentTime.After(cert.NotAfter)
		}) {
			continue
		}
		// Since we already validated the chain, we only care that it is rooted
		// in a CA in opts.Roots. On platforms where we control chain validation
		// (e.g. not Windows or macOS) this is a simple lookup in the CertPool
		// internal hash map, which we can simulate by running Verify on the
		// root. On other platforms, we have to do full verification again,
		// because EKU handling might differ. We will want to replace this with
		// CertPool.Contains if/once that is available. See go.dev/issue/77376.
		if runtime.GOOS == "windows" || runtime.GOOS == "darwin" || runtime.GOOS == "ios" {
			opts.Intermediates = x509.NewCertPool()
			for _, cert := range chain[1:max(1, len(chain)-1)] {
				opts.Intermediates.AddCert(cert)
			}
			leaf := chain[0]
			if _, err := leaf.Verify(opts); err == nil {
				return true
			}
		} else {
			root := chain[len(chain)-1]
			if _, err := root.Verify(opts); err == nil {
				return true
			}
		}
	}
	return false
}

```

// === FILE: references/go/src/crypto/tls/common_string.go ===
```go
// Code generated by "stringer -linecomment -type=SignatureScheme,CurveID,ClientAuthType -output=common_string.go"; DO NOT EDIT.

package tls

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[PKCS1WithSHA256-1025]
	_ = x[PKCS1WithSHA384-1281]
	_ = x[PKCS1WithSHA512-1537]
	_ = x[PSSWithSHA256-2052]
	_ = x[PSSWithSHA384-2053]
	_ = x[PSSWithSHA512-2054]
	_ = x[ECDSAWithP256AndSHA256-1027]
	_ = x[ECDSAWithP384AndSHA384-1283]
	_ = x[ECDSAWithP521AndSHA512-1539]
	_ = x[Ed25519-2055]
	_ = x[MLDSA44-2308]
	_ = x[MLDSA65-2309]
	_ = x[MLDSA87-2310]
	_ = x[PKCS1WithSHA1-513]
	_ = x[ECDSAWithSHA1-515]
}

const (
	_SignatureScheme_name_0 = "PKCS1WithSHA1"
	_SignatureScheme_name_1 = "ECDSAWithSHA1"
	_SignatureScheme_name_2 = "PKCS1WithSHA256"
	_SignatureScheme_name_3 = "ECDSAWithP256AndSHA256"
	_SignatureScheme_name_4 = "PKCS1WithSHA384"
	_SignatureScheme_name_5 = "ECDSAWithP384AndSHA384"
	_SignatureScheme_name_6 = "PKCS1WithSHA512"
	_SignatureScheme_name_7 = "ECDSAWithP521AndSHA512"
	_SignatureScheme_name_8 = "PSSWithSHA256PSSWithSHA384PSSWithSHA512Ed25519"
	_SignatureScheme_name_9 = "MLDSA44MLDSA65MLDSA87"
)

var (
	_SignatureScheme_index_8 = [...]uint8{0, 13, 26, 39, 46}
	_SignatureScheme_index_9 = [...]uint8{0, 7, 14, 21}
)

func (i SignatureScheme) String() string {
	switch {
	case i == 513:
		return _SignatureScheme_name_0
	case i == 515:
		return _SignatureScheme_name_1
	case i == 1025:
		return _SignatureScheme_name_2
	case i == 1027:
		return _SignatureScheme_name_3
	case i == 1281:
		return _SignatureScheme_name_4
	case i == 1283:
		return _SignatureScheme_name_5
	case i == 1537:
		return _SignatureScheme_name_6
	case i == 1539:
		return _SignatureScheme_name_7
	case 2052 <= i && i <= 2055:
		i -= 2052
		return _SignatureScheme_name_8[_SignatureScheme_index_8[i]:_SignatureScheme_index_8[i+1]]
	case 2308 <= i && i <= 2310:
		i -= 2308
		return _SignatureScheme_name_9[_SignatureScheme_index_9[i]:_SignatureScheme_index_9[i+1]]
	default:
		return "SignatureScheme(" + strconv.FormatInt(int64(i), 10) + ")"
	}
}
func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[CurveP256-23]
	_ = x[CurveP384-24]
	_ = x[CurveP521-25]
	_ = x[X25519-29]
	_ = x[X25519MLKEM768-4588]
	_ = x[SecP256r1MLKEM768-4587]
	_ = x[SecP384r1MLKEM1024-4589]
	_ = x[MLKEM1024-514]
}

const (
	_CurveID_name_0 = "CurveP256CurveP384CurveP521"
	_CurveID_name_1 = "X25519"
	_CurveID_name_2 = "MLKEM1024"
	_CurveID_name_3 = "SecP256r1MLKEM768X25519MLKEM768SecP384r1MLKEM1024"
)

var (
	_CurveID_index_0 = [...]uint8{0, 9, 18, 27}
	_CurveID_index_3 = [...]uint8{0, 17, 31, 49}
)

func (i CurveID) String() string {
	switch {
	case 23 <= i && i <= 25:
		i -= 23
		return _CurveID_name_0[_CurveID_index_0[i]:_CurveID_index_0[i+1]]
	case i == 29:
		return _CurveID_name_1
	case i == 514:
		return _CurveID_name_2
	case 4587 <= i && i <= 4589:
		i -= 4587
		return _CurveID_name_3[_CurveID_index_3[i]:_CurveID_index_3[i+1]]
	default:
		return "CurveID(" + strconv.FormatInt(int64(i), 10) + ")"
	}
}
func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[NoClientCert-0]
	_ = x[RequestClientCert-1]
	_ = x[RequireAnyClientCert-2]
	_ = x[VerifyClientCertIfGiven-3]
	_ = x[RequireAndVerifyClientCert-4]
}

const _ClientAuthType_name = "NoClientCertRequestClientCertRequireAnyClientCertVerifyClientCertIfGivenRequireAndVerifyClientCert"

var _ClientAuthType_index = [...]uint8{0, 12, 29, 49, 72, 98}

func (i ClientAuthType) String() string {
	idx := int(i) - 0
	if i < 0 || idx >= len(_ClientAuthType_index)-1 {
		return "ClientAuthType(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _ClientAuthType_name[_ClientAuthType_index[idx]:_ClientAuthType_index[idx+1]]
}

```

// === FILE: references/go/src/crypto/tls/conn.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TLS low level connection and record layer

package tls

import (
	"bytes"
	"context"
	"crypto/cipher"
	"crypto/subtle"
	"crypto/x509"
	"errors"
	"fmt"
	"hash"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// A Conn represents a secured connection.
// It implements the net.Conn interface.
type Conn struct {
	// constant
	conn        net.Conn
	isClient    bool
	handshakeFn func(context.Context) error // (*Conn).clientHandshake or serverHandshake
	quic        *quicState                  // nil for non-QUIC connections

	// isHandshakeComplete is true if the connection is currently transferring
	// application data (i.e. is not currently processing a handshake).
	// isHandshakeComplete is true implies handshakeErr == nil.
	isHandshakeComplete atomic.Bool
	// constant after handshake; protected by handshakeMutex
	handshakeMutex sync.Mutex
	handshakeErr   error   // error resulting from handshake
	vers           uint16  // TLS version
	haveVers       bool    // version has been negotiated
	config         *Config // configuration passed to constructor
	// handshakes counts the number of handshakes performed on the
	// connection so far. If renegotiation is disabled then this is either
	// zero or one.
	handshakes       int
	extMasterSecret  bool
	didResume        bool // whether this connection was a session resumption
	didHRR           bool // whether a HelloRetryRequest was sent/received
	cipherSuite      uint16
	curveID          CurveID
	peerSigAlg       SignatureScheme
	ocspResponse     []byte   // stapled OCSP response
	scts             [][]byte // signed certificate timestamps from server
	peerCertificates []*x509.Certificate
	localCertificate [][]byte
	// verifiedChains contains the certificate chains that we built, as
	// opposed to the ones presented by the server.
	verifiedChains [][]*x509.Certificate
	// serverName contains the server name indicated by the client, if any.
	serverName string
	// secureRenegotiation is true if the server echoed the secure
	// renegotiation extension. (This is meaningless as a server because
	// renegotiation is not supported in that case.)
	secureRenegotiation bool
	// ekm is a closure for exporting keying material.
	ekm func(label string, context []byte, length int) ([]byte, error)
	// resumptionSecret is the resumption_master_secret for handling
	// or sending NewSessionTicket messages.
	resumptionSecret []byte
	echAccepted      bool

	// ticketKeys is the set of active session ticket keys for this
	// connection. The first one is used to encrypt new tickets and
	// all are tried to decrypt tickets.
	ticketKeys []ticketKey

	// clientFinishedIsFirst is true if the client sent the first Finished
	// message during the most recent handshake. This is recorded because
	// the first transmitted Finished message is the tls-unique
	// channel-binding value.
	clientFinishedIsFirst bool

	// closeNotifyErr is any error from sending the alertCloseNotify record.
	closeNotifyErr error
	// closeNotifySent is true if the Conn attempted to send an
	// alertCloseNotify record.
	closeNotifySent bool

	// clientFinished and serverFinished contain the Finished message sent
	// by the client or server in the most recent handshake. This is
	// retained to support the renegotiation extension and tls-unique
	// channel-binding.
	clientFinished [12]byte
	serverFinished [12]byte

	// clientProtocol is the negotiated ALPN protocol.
	clientProtocol string

	// input/output
	in, out   halfConn
	rawInput  bytes.Buffer // raw input, starting with a record header
	input     bytes.Reader // application data waiting to be read, from rawInput.Next
	hand      bytes.Buffer // handshake data waiting to be read
	buffering bool         // whether records are buffered in sendBuf
	sendBuf   []byte       // a buffer of records waiting to be sent

	// bytesSent counts the bytes of application data sent.
	// packetsSent counts packets.
	bytesSent   int64
	packetsSent int64

	// retryCount counts the number of consecutive non-advancing records
	// received by Conn.readRecord. That is, records that neither advance the
	// handshake, nor deliver application data. Protected by in.Mutex.
	retryCount int

	// activeCall indicates whether Close has been call in the low bit.
	// the rest of the bits are the number of goroutines in Conn.Write.
	activeCall atomic.Int32

	tmp [16]byte
}

// Access to net.Conn methods.
// Cannot just embed net.Conn because that would
// export the struct field too.

// LocalAddr returns the local network address.
func (c *Conn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr returns the remote network address.
func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// SetDeadline sets the read and write deadlines associated with the connection.
// A zero value for t means [Conn.Read] and [Conn.Write] will not time out.
// After a Write has timed out, the TLS state is corrupt and all future writes will return the same error.
func (c *Conn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

// SetReadDeadline sets the read deadline on the underlying connection.
// A zero value for t means [Conn.Read] will not time out.
func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline on the underlying connection.
// A zero value for t means [Conn.Write] will not time out.
// After a [Conn.Write] has timed out, the TLS state is corrupt and all future writes will return the same error.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

// NetConn returns the underlying connection that is wrapped by c.
// Note that writing to or reading from this connection directly will corrupt the
// TLS session.
func (c *Conn) NetConn() net.Conn {
	return c.conn
}

// A halfConn represents one direction of the record layer
// connection, either sending or receiving.
type halfConn struct {
	sync.Mutex

	err     error  // first permanent error
	version uint16 // protocol version
	cipher  any    // cipher algorithm
	mac     hash.Hash
	seq     [8]byte // 64-bit sequence number

	scratchBuf [13]byte // to avoid allocs; interface method args escape

	nextCipher any       // next encryption state
	nextMac    hash.Hash // next MAC algorithm

	level         QUICEncryptionLevel // current QUIC encryption level
	trafficSecret []byte              // current TLS 1.3 traffic secret
}

type permanentError struct {
	err net.Error
}

func (e *permanentError) Error() string   { return e.err.Error() }
func (e *permanentError) Unwrap() error   { return e.err }
func (e *permanentError) Timeout() bool   { return e.err.Timeout() }
func (e *permanentError) Temporary() bool { return false }

func (hc *halfConn) setErrorLocked(err error) error {
	if e, ok := err.(net.Error); ok {
		hc.err = &permanentError{err: e}
	} else {
		hc.err = err
	}
	return hc.err
}

// prepareCipherSpec sets the encryption and MAC states
// that a subsequent changeCipherSpec will use.
func (hc *halfConn) prepareCipherSpec(version uint16, cipher any, mac hash.Hash) {
	hc.version = version
	hc.nextCipher = cipher
	hc.nextMac = mac
}

// changeCipherSpec changes the encryption and MAC states
// to the ones previously passed to prepareCipherSpec.
func (hc *halfConn) changeCipherSpec() error {
	if hc.nextCipher == nil || hc.version == VersionTLS13 {
		return alertInternalError
	}
	hc.cipher = hc.nextCipher
	hc.mac = hc.nextMac
	hc.nextCipher = nil
	hc.nextMac = nil
	clear(hc.seq[:])
	return nil
}

// setTrafficSecret sets the traffic secret for the given encryption level. setTrafficSecret
// should not be called directly, but rather through the Conn setWriteTrafficSecret and
// setReadTrafficSecret wrapper methods.
func (hc *halfConn) setTrafficSecret(suite *cipherSuiteTLS13, level QUICEncryptionLevel, secret []byte) {
	hc.trafficSecret = secret
	hc.level = level
	key, iv := suite.trafficKey(secret)
	hc.cipher = suite.aead(key, iv)
	clear(hc.seq[:])
}

// incSeq increments the sequence number.
func (hc *halfConn) incSeq() {
	for i := 7; i >= 0; i-- {
		hc.seq[i]++
		if hc.seq[i] != 0 {
			return
		}
	}

	// Not allowed to let sequence number wrap.
	// Instead, must renegotiate before it does.
	// Not likely enough to bother.
	panic("TLS: sequence number wraparound")
}

// explicitNonceLen returns the number of bytes of explicit nonce or IV included
// in each record. Explicit nonces are present only in CBC modes after TLS 1.0
// and in certain AEAD modes in TLS 1.2.
func (hc *halfConn) explicitNonceLen() int {
	if hc.cipher == nil {
		return 0
	}

	switch c := hc.cipher.(type) {
	case cipher.Stream:
		return 0
	case aead:
		return c.explicitNonceLen()
	case cbcMode:
		// TLS 1.1 introduced a per-record explicit IV to fix the BEAST attack.
		if hc.version >= VersionTLS11 {
			return c.BlockSize()
		}
		return 0
	default:
		panic("unknown cipher type")
	}
}

// extractPadding returns, in constant time, the length of the padding to remove
// from the end of payload. It also returns a byte which is equal to 255 if the
// padding was valid and 0 otherwise. See RFC 2246, Section 6.2.3.2.
func extractPadding(payload []byte) (toRemove int, good byte) {
	if len(payload) < 1 {
		return 0, 0
	}

	paddingLen := payload[len(payload)-1]
	t := uint(len(payload)-1) - uint(paddingLen)
	// if len(payload) >= (paddingLen - 1) then the MSB of t is zero
	good = byte(int32(^t) >> 31)

	// The maximum possible padding length plus the actual length field
	toCheck := 256
	// The length of the padded data is public, so we can use an if here
	if toCheck > len(payload) {
		toCheck = len(payload)
	}

	for i := 0; i < toCheck; i++ {
		t := uint(paddingLen) - uint(i)
		// if i <= paddingLen then the MSB of t is zero
		mask := byte(int32(^t) >> 31)
		b := payload[len(payload)-1-i]
		good &^= mask&paddingLen ^ mask&b
	}

	// We AND together the bits of good and replicate the result across
	// all the bits.
	good &= good << 4
	good &= good << 2
	good &= good << 1
	good = uint8(int8(good) >> 7)

	// Zero the padding length on error. This ensures any unchecked bytes
	// are included in the MAC. Otherwise, an attacker that could
	// distinguish MAC failures from padding failures could mount an attack
	// similar to POODLE in SSL 3.0: given a good ciphertext that uses a
	// full block's worth of padding, replace the final block with another
	// block. If the MAC check passed but the padding check failed, the
	// last byte of that block decrypted to the block size.
	//
	// See also macAndPaddingGood logic below.
	paddingLen &= good

	toRemove = int(paddingLen) + 1
	return
}

func roundUp(a, b int) int {
	return a + (b-a%b)%b
}

// cbcMode is an interface for block ciphers using cipher block chaining.
type cbcMode interface {
	cipher.BlockMode
	SetIV([]byte)
}

// decrypt authenticates and decrypts the record if protection is active at
// this stage. The returned plaintext might overlap with the input.
func (hc *halfConn) decrypt(record []byte) ([]byte, recordType, error) {
	var plaintext []byte
	typ := recordType(record[0])
	payload := record[recordHeaderLen:]

	// In TLS 1.3, change_cipher_spec messages are to be ignored without being
	// decrypted. See RFC 8446, Appendix D.4.
	if hc.version == VersionTLS13 && typ == recordTypeChangeCipherSpec {
		return payload, typ, nil
	}

	paddingGood := byte(255)
	paddingLen := 0

	explicitNonceLen := hc.explicitNonceLen()

	if hc.cipher != nil {
		switch c := hc.cipher.(type) {
		case cipher.Stream:
			c.XORKeyStream(payload, payload)
		case aead:
			if len(payload) < explicitNonceLen {
				return nil, 0, alertBadRecordMAC
			}
			nonce := payload[:explicitNonceLen]
			if len(nonce) == 0 {
				nonce = hc.seq[:]
			}
			payload = payload[explicitNonceLen:]

			var additionalData []byte
			if hc.version == VersionTLS13 {
				additionalData = record[:recordHeaderLen]
			} else {
				additionalData = append(hc.scratchBuf[:0], hc.seq[:]...)
				additionalData = append(additionalData, record[:3]...)
				n := len(payload) - c.Overhead()
				additionalData = append(additionalData, byte(n>>8), byte(n))
			}

			var err error
			plaintext, err = c.Open(payload[:0], nonce, payload, additionalData)
			if err != nil {
				return nil, 0, alertBadRecordMAC
			}
		case cbcMode:
			blockSize := c.BlockSize()
			minPayload := explicitNonceLen + roundUp(hc.mac.Size()+1, blockSize)
			if len(payload)%blockSize != 0 || len(payload) < minPayload {
				return nil, 0, alertBadRecordMAC
			}

			if explicitNonceLen > 0 {
				c.SetIV(payload[:explicitNonceLen])
				payload = payload[explicitNonceLen:]
			}
			c.CryptBlocks(payload, payload)

			// In a limited attempt to protect against CBC padding oracles like
			// Lucky13, the data past paddingLen (which is secret) is passed to
			// the MAC function as extra data, to be fed into the HMAC after
			// computing the digest. This makes the MAC roughly constant time as
			// long as the digest computation is constant time and does not
			// affect the subsequent write, modulo cache effects.
			paddingLen, paddingGood = extractPadding(payload)
		default:
			panic("unknown cipher type")
		}

		if hc.version == VersionTLS13 {
			if typ != recordTypeApplicationData {
				return nil, 0, alertUnexpectedMessage
			}
			if len(plaintext) > maxPlaintext+1 {
				return nil, 0, alertRecordOverflow
			}
			// Remove padding and find the ContentType scanning from the end.
			for i := len(plaintext) - 1; i >= 0; i-- {
				if plaintext[i] != 0 {
					typ = recordType(plaintext[i])
					plaintext = plaintext[:i]
					break
				}
				if i == 0 {
					return nil, 0, alertUnexpectedMessage
				}
			}
		}
	} else {
		plaintext = payload
	}

	if hc.mac != nil {
		macSize := hc.mac.Size()
		if len(payload) < macSize {
			return nil, 0, alertBadRecordMAC
		}

		n := len(payload) - macSize - paddingLen
		n = subtle.ConstantTimeSelect(int(uint32(n)>>31), 0, n) // if n < 0 { n = 0 }
		record[3] = byte(n >> 8)
		record[4] = byte(n)
		remoteMAC := payload[n : n+macSize]
		localMAC := tls10MAC(hc.mac, hc.scratchBuf[:0], hc.seq[:], record[:recordHeaderLen], payload[:n], payload[n+macSize:])

		// This is equivalent to checking the MACs and paddingGood
		// separately, but in constant-time to prevent distinguishing
		// padding failures from MAC failures. Depending on what value
		// of paddingLen was returned on bad padding, distinguishing
		// bad MAC from bad padding can lead to an attack.
		//
		// See also the logic at the end of extractPadding.
		macAndPaddingGood := subtle.ConstantTimeCompare(localMAC, remoteMAC) & int(paddingGood)
		if macAndPaddingGood != 1 {
			return nil, 0, alertBadRecordMAC
		}

		plaintext = payload[:n]
	}

	hc.incSeq()
	return plaintext, typ, nil
}

// sliceForAppend extends the input slice by n bytes. head is the full extended
// slice, while tail is the appended part. If the original slice has sufficient
// capacity no allocation is performed.
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

// encrypt encrypts payload, adding the appropriate nonce and/or MAC, and
// appends it to record, which must already contain the record header.
func (hc *halfConn) encrypt(record, payload []byte, rand io.Reader) ([]byte, error) {
	if hc.cipher == nil {
		return append(record, payload...), nil
	}

	var explicitNonce []byte
	if explicitNonceLen := hc.explicitNonceLen(); explicitNonceLen > 0 {
		record, explicitNonce = sliceForAppend(record, explicitNonceLen)
		if _, isCBC := hc.cipher.(cbcMode); !isCBC && explicitNonceLen < 16 {
			// The AES-GCM construction in TLS has an explicit nonce so that the
			// nonce can be random. However, the nonce is only 8 bytes which is
			// too small for a secure, random nonce. Therefore we use the
			// sequence number as the nonce. The 3DES-CBC construction also has
			// an 8 bytes nonce but its nonces must be unpredictable (see RFC
			// 5246, Appendix F.3), forcing us to use randomness. That's not
			// 3DES' biggest problem anyway because the birthday bound on block
			// collision is reached first due to its similarly small block size
			// (see the Sweet32 attack).
			copy(explicitNonce, hc.seq[:])
		} else {
			if _, err := io.ReadFull(rand, explicitNonce); err != nil {
				return nil, err
			}
		}
	}

	var dst []byte
	switch c := hc.cipher.(type) {
	case cipher.Stream:
		mac := tls10MAC(hc.mac, hc.scratchBuf[:0], hc.seq[:], record[:recordHeaderLen], payload, nil)
		record, dst = sliceForAppend(record, len(payload)+len(mac))
		c.XORKeyStream(dst[:len(payload)], payload)
		c.XORKeyStream(dst[len(payload):], mac)
	case aead:
		nonce := explicitNonce
		if len(nonce) == 0 {
			nonce = hc.seq[:]
		}

		if hc.version == VersionTLS13 {
			record = append(record, payload...)

			// Encrypt the actual ContentType and replace the plaintext one.
			record = append(record, record[0])
			record[0] = byte(recordTypeApplicationData)

			n := len(payload) + 1 + c.Overhead()
			record[3] = byte(n >> 8)
			record[4] = byte(n)

			record = c.Seal(record[:recordHeaderLen],
				nonce, record[recordHeaderLen:], record[:recordHeaderLen])
		} else {
			additionalData := append(hc.scratchBuf[:0], hc.seq[:]...)
			additionalData = append(additionalData, record[:recordHeaderLen]...)
			record = c.Seal(record, nonce, payload, additionalData)
		}
	case cbcMode:
		mac := tls10MAC(hc.mac, hc.scratchBuf[:0], hc.seq[:], record[:recordHeaderLen], payload, nil)
		blockSize := c.BlockSize()
		plaintextLen := len(payload) + len(mac)
		paddingLen := blockSize - plaintextLen%blockSize
		record, dst = sliceForAppend(record, plaintextLen+paddingLen)
		copy(dst, payload)
		copy(dst[len(payload):], mac)
		for i := plaintextLen; i < len(dst); i++ {
			dst[i] = byte(paddingLen - 1)
		}
		if len(explicitNonce) > 0 {
			c.SetIV(explicitNonce)
		}
		c.CryptBlocks(dst, dst)
	default:
		panic("unknown cipher type")
	}

	// Update length to include nonce, MAC and any block padding needed.
	n := len(record) - recordHeaderLen
	record[3] = byte(n >> 8)
	record[4] = byte(n)
	hc.incSeq()

	return record, nil
}

// RecordHeaderError is returned when a TLS record header is invalid.
type RecordHeaderError struct {
	// Msg contains a human readable string that describes the error.
	Msg string
	// RecordHeader contains the five bytes of TLS record header that
	// triggered the error.
	RecordHeader [5]byte
	// Conn provides the underlying net.Conn in the case that a client
	// sent an initial handshake that didn't look like TLS.
	// It is nil if there's already been a handshake or a TLS alert has
	// been written to the connection.
	Conn net.Conn
}

func (e RecordHeaderError) Error() string { return "tls: " + e.Msg }

func (c *Conn) newRecordHeaderError(conn net.Conn, msg string) (err RecordHeaderError) {
	err.Msg = msg
	err.Conn = conn
	copy(err.RecordHeader[:], c.rawInput.Bytes())
	return err
}

func (c *Conn) readRecord() error {
	return c.readRecordOrCCS(false)
}

func (c *Conn) readChangeCipherSpec() error {
	return c.readRecordOrCCS(true)
}

// readRecordOrCCS reads one or more TLS records from the connection and
// updates the record layer state. Some invariants:
//   - c.in must be locked
//   - c.input must be empty
//
// During the handshake one and only one of the following will happen:
//   - c.hand grows
//   - c.in.changeCipherSpec is called
//   - an error is returned
//
// After the handshake one and only one of the following will happen:
//   - c.hand grows
//   - c.input is set
//   - an error is returned
func (c *Conn) readRecordOrCCS(expectChangeCipherSpec bool) error {
	if c.in.err != nil {
		return c.in.err
	}
	handshakeComplete := c.isHandshakeComplete.Load()

	// This function modifies c.rawInput, which owns the c.input memory.
	if c.input.Len() != 0 {
		return c.in.setErrorLocked(errors.New("tls: internal error: attempted to read record with pending application data"))
	}
	c.input.Reset(nil)

	if c.quic != nil {
		return c.in.setErrorLocked(errors.New("tls: internal error: attempted to read record with QUIC transport"))
	}

	// Read header, payload.
	if err := c.readFromUntil(c.conn, recordHeaderLen); err != nil {
		// RFC 8446, Section 6.1 suggests that EOF without an alertCloseNotify
		// is an error, but popular web sites seem to do this, so we accept it
		// if and only if at the record boundary.
		if err == io.ErrUnexpectedEOF && c.rawInput.Len() == 0 {
			err = io.EOF
		}
		if e, ok := err.(net.Error); !ok || !e.Temporary() {
			c.in.setErrorLocked(err)
		}
		return err
	}
	hdr := c.rawInput.Bytes()[:recordHeaderLen]
	typ := recordType(hdr[0])

	// No valid TLS record has a type of 0x80, however SSLv2 handshakes
	// start with a uint16 length where the MSB is set and the first record
	// is always < 256 bytes long. Therefore typ == 0x80 strongly suggests
	// an SSLv2 client.
	if !handshakeComplete && typ == 0x80 {
		c.sendAlert(alertProtocolVersion)
		return c.in.setErrorLocked(c.newRecordHeaderError(nil, "unsupported SSLv2 handshake received"))
	}

	vers := uint16(hdr[1])<<8 | uint16(hdr[2])
	expectedVers := c.vers
	if expectedVers == VersionTLS13 {
		// All TLS 1.3 records are expected to have 0x0303 (1.2) after
		// the initial hello (RFC 8446 Section 5.1).
		expectedVers = VersionTLS12
	}
	n := int(hdr[3])<<8 | int(hdr[4])
	if c.haveVers && vers != expectedVers {
		c.sendAlert(alertProtocolVersion)
		msg := fmt.Sprintf("received record with version %x when expecting version %x", vers, expectedVers)
		return c.in.setErrorLocked(c.newRecordHeaderError(nil, msg))
	}
	if !c.haveVers {
		// First message, be extra suspicious: this might not be a TLS
		// client. Bail out before reading a full 'body', if possible.
		// The current max version is 3.3 so if the version is >= 16.0,
		// it's probably not real.
		if (typ != recordTypeAlert && typ != recordTypeHandshake) || vers >= 0x1000 {
			return c.in.setErrorLocked(c.newRecordHeaderError(c.conn, "first record does not look like a TLS handshake"))
		}
	}
	if c.vers == VersionTLS13 && n > maxCiphertextTLS13 || n > maxCiphertext {
		c.sendAlert(alertRecordOverflow)
		msg := fmt.Sprintf("oversized record received with length %d", n)
		return c.in.setErrorLocked(c.newRecordHeaderError(nil, msg))
	}
	if err := c.readFromUntil(c.conn, recordHeaderLen+n); err != nil {
		if e, ok := err.(net.Error); !ok || !e.Temporary() {
			c.in.setErrorLocked(err)
		}
		return err
	}

	// Process message.
	record := c.rawInput.Next(recordHeaderLen + n)
	data, typ, err := c.in.decrypt(record)
	if err != nil {
		return c.in.setErrorLocked(c.sendAlert(err.(alert)))
	}
	if len(data) > maxPlaintext {
		return c.in.setErrorLocked(c.sendAlert(alertRecordOverflow))
	}

	// Application Data messages are always protected.
	if c.in.cipher == nil && typ == recordTypeApplicationData {
		return c.in.setErrorLocked(c.sendAlert(alertUnexpectedMessage))
	}

	if typ != recordTypeAlert && typ != recordTypeChangeCipherSpec && len(data) > 0 {
		// This is a state-advancing message: reset the retry count.
		c.retryCount = 0
	}

	// Handshake messages MUST NOT be interleaved with other record types in TLS 1.3.
	if c.vers == VersionTLS13 && typ != recordTypeHandshake && c.hand.Len() > 0 {
		return c.in.setErrorLocked(c.sendAlert(alertUnexpectedMessage))
	}

	switch typ {
	default:
		return c.in.setErrorLocked(c.sendAlert(alertUnexpectedMessage))

	case recordTypeAlert:
		if c.quic != nil {
			return c.in.setErrorLocked(c.sendAlert(alertUnexpectedMessage))
		}
		if len(data) != 2 {
			return c.in.setErrorLocked(c.sendAlert(alertUnexpectedMessage))
		}
		if alert(data[1]) == alertCloseNotify {
			return c.in.setErrorLocked(io.EOF)
		}
		if c.vers == VersionTLS13 {
			// TLS 1.3 removed warning-level alerts except for alertUserCanceled
			// (RFC 8446, § 6.1). Since at least one major implementation
			// (https://bugs.openjdk.org/browse/JDK-8323517) misuses this alert,
			// many TLS stacks now ignore it outright when seen in a TLS 1.3
			// handshake (e.g. BoringSSL, NSS, Rustls).
			if alert(data[1]) == alertUserCanceled {
				// Like TLS 1.2 alertLevelWarning alerts, we drop the record and retry.
				return c.retryReadRecord(expectChangeCipherSpec)
			}
			return c.in.setErrorLocked(&net.OpError{Op: "remote error", Err: alert(data[1])})
		}
		switch data[0] {
		case alertLevelWarning:
			// Drop the record on the floor and retry.
			return c.retryReadRecord(expectChangeCipherSpec)
		case alertLevelError:
			return c.in.setErrorLocked(&net.OpError{Op: "remote error", Err: alert(data[1])})
		default:
			return c.in.setErrorLocked(c.sendAlert(alertUnexpectedMessage))
		}

	case recordTypeChangeCipherSpec:
		if len(data) != 1 || data[0] != 1 {
			return c.in.setErrorLocked(c.sendAlert(alertDecodeError))
		}
		// Handshake messages are not allowed to fragment across the CCS.
		if c.hand.Len() > 0 {
			return c.in.setErrorLocked(c.sendAlert(alertUnexpectedMessage))
		}
		// In TLS 1.3, change_cipher_spec records are ignored until the
		// Finished. See RFC 8446, Appendix D.4. Note that according to Section
		// 5, a server can send a ChangeCipherSpec before its ServerHello, when
		// c.vers is still unset. That's not useful though and suspicious if the
		// server then selects a lower protocol version, so don't allow that.
		if c.vers == VersionTLS13 {
			return c.retryReadRecord(expectChangeCipherSpec)
		}
		if !expectChangeCipherSpec {
			return c.in.setErrorLocked(c.sendAlert(alertUnexpectedMessage))
		}
		if err := c.in.changeCipherSpec(); err != nil {
			return c.in.setErrorLocked(c.sendAlert(err.(alert)))
		}

	case recordTypeApplicationData:
		if !handshakeComplete || expectChangeCipherSpec {
			return c.in.setErrorLocked(c.sendAlert(alertUnexpectedMessage))
		}
		// Some OpenSSL servers send empty records in order to randomize the
		// CBC IV. Ignore a limited number of empty records.
		if len(data) == 0 {
			return c.retryReadRecord(expectChangeCipherSpec)
		}
		// Note that data is owned by c.rawInput, following the Next call above,
		// to avoid copying the plaintext. This is safe because c.rawInput is
		// not read from or written to until c.input is drained.
		c.input.Reset(data)

	case recordTypeHandshake:
		if len(data) == 0 || expectChangeCipherSpec {
			return c.in.setErrorLocked(c.sendAlert(alertUnexpectedMessage))
		}
		c.hand.Write(data)
	}

	return nil
}

// retryReadRecord recurs into readRecordOrCCS to drop a non-advancing record, like
// a warning alert, empty application_data, or a change_cipher_spec in TLS 1.3.
func (c *Conn) retryReadRecord(expectChangeCipherSpec bool) error {
	c.retryCount++
	if c.retryCount > maxUselessRecords {
		c.sendAlert(alertUnexpectedMessage)
		return c.in.setErrorLocked(errors.New("tls: too many ignored records"))
	}
	return c.readRecordOrCCS(expectChangeCipherSpec)
}

// readFromUntil reads from r into c.rawInput until c.rawInput contains
// at least n bytes or else returns an error.
func (c *Conn) readFromUntil(r io.Reader, n int) error {
	if c.rawInput.Len() >= n {
		return nil
	}
	needs := n - c.rawInput.Len()
	// There might be extra input waiting on the wire. Make a best effort
	// attempt to fetch it so that it can be used in (*Conn).Read to
	// "predict" closeNotify alerts.
	// TODO(dmo): we use bytes.MinRead here because we used the buffer
	// ReadFrom mechanism to avoid allocations, but we've hoisted this
	// loop for performance. We really should use our own heuristic here
	// for how much to read ahead.
	c.rawInput.Grow(needs + bytes.MinRead)
	for {
		buf := c.rawInput.AvailableBuffer()[:c.rawInput.Available()]
		n, err := r.Read(buf)
		// This write is just to update the internal state of the
		// rawInput bytes.Buffer. It cannot fail.
		c.rawInput.Write(buf[:n])
		needs -= n
		if needs <= 0 {
			if err == io.EOF {
				err = nil
			}
			return err
		}
		if err == io.EOF {
			return io.ErrUnexpectedEOF
		}
		if err != nil {
			return err
		}
	}
}

// sendAlertLocked sends a TLS alert message.
func (c *Conn) sendAlertLocked(err alert) error {
	if c.quic != nil {
		return c.out.setErrorLocked(&net.OpError{Op: "local error", Err: err})
	}

	switch err {
	case alertNoRenegotiation, alertCloseNotify:
		c.tmp[0] = alertLevelWarning
	default:
		c.tmp[0] = alertLevelError
	}
	c.tmp[1] = byte(err)

	_, writeErr := c.writeRecordLocked(recordTypeAlert, c.tmp[0:2])
	if err == alertCloseNotify {
		// closeNotify is a special case in that it isn't an error.
		return writeErr
	}

	return c.out.setErrorLocked(&net.OpError{Op: "local error", Err: err})
}

// sendAlert sends a TLS alert message.
func (c *Conn) sendAlert(err alert) error {
	c.out.Lock()
	defer c.out.Unlock()
	return c.sendAlertLocked(err)
}

const (
	// tcpMSSEstimate is a conservative estimate of the TCP maximum segment
	// size (MSS). A constant is used, rather than querying the kernel for
	// the actual MSS, to avoid complexity. The value here is the IPv6
	// minimum MTU (1280 bytes) minus the overhead of an IPv6 header (40
	// bytes) and a TCP header with timestamps (32 bytes).
	tcpMSSEstimate = 1208

	// recordSizeBoostThreshold is the number of bytes of application data
	// sent after which the TLS record size will be increased to the
	// maximum.
	recordSizeBoostThreshold = 128 * 1024
)

// maxPayloadSizeForWrite returns the maximum TLS payload size to use for the
// next application data record. There is the following trade-off:
//
//   - For latency-sensitive applications, such as web browsing, each TLS
//     record should fit in one TCP segment.
//   - For throughput-sensitive applications, such as large file transfers,
//     larger TLS records better amortize framing and encryption overheads.
//
// A simple heuristic that works well in practice is to use small records for
// the first 1MB of data, then use larger records for subsequent data, and
// reset back to smaller records after the connection becomes idle. See "High
// Performance Web Networking", Chapter 4, or:
// https://www.igvita.com/2013/10/24/optimizing-tls-record-size-and-buffering-latency/
//
// In the interests of simplicity and determinism, this code does not attempt
// to reset the record size once the connection is idle, however.
func (c *Conn) maxPayloadSizeForWrite(typ recordType) int {
	if c.config.DynamicRecordSizingDisabled || typ != recordTypeApplicationData {
		return maxPlaintext
	}

	if c.bytesSent >= recordSizeBoostThreshold {
		return maxPlaintext
	}

	// Subtract TLS overheads to get the maximum payload size.
	payloadBytes := tcpMSSEstimate - recordHeaderLen - c.out.explicitNonceLen()
	if c.out.cipher != nil {
		switch ciph := c.out.cipher.(type) {
		case cipher.Stream:
			payloadBytes -= c.out.mac.Size()
		case cipher.AEAD:
			payloadBytes -= ciph.Overhead()
		case cbcMode:
			blockSize := ciph.BlockSize()
			// The payload must fit in a multiple of blockSize, with
			// room for at least one padding byte.
			payloadBytes = (payloadBytes & ^(blockSize - 1)) - 1
			// The MAC is appended before padding so affects the
			// payload size directly.
			payloadBytes -= c.out.mac.Size()
		default:
			panic("unknown cipher type")
		}
	}
	if c.vers == VersionTLS13 {
		payloadBytes-- // encrypted ContentType
	}

	// Allow packet growth in arithmetic progression up to max.
	pkt := c.packetsSent
	c.packetsSent++
	if pkt > 1000 {
		return maxPlaintext // avoid overflow in multiply below
	}

	n := payloadBytes * int(pkt+1)
	if n > maxPlaintext {
		n = maxPlaintext
	}
	return n
}

func (c *Conn) write(data []byte) (int, error) {
	if c.buffering {
		c.sendBuf = append(c.sendBuf, data...)
		return len(data), nil
	}

	n, err := c.conn.Write(data)
	c.bytesSent += int64(n)
	return n, err
}

func (c *Conn) flush() (int, error) {
	if len(c.sendBuf) == 0 {
		return 0, nil
	}

	n, err := c.conn.Write(c.sendBuf)
	c.bytesSent += int64(n)
	c.sendBuf = nil
	c.buffering = false
	return n, err
}

// outBufPool pools the record-sized scratch buffers used by writeRecordLocked.
var outBufPool = sync.Pool{
	New: func() any {
		return new([]byte)
	},
}

// writeRecordLocked writes a TLS record with the given type and payload to the
// connection and updates the record layer state.
func (c *Conn) writeRecordLocked(typ recordType, data []byte) (int, error) {
	if c.quic != nil {
		if typ != recordTypeHandshake {
			return 0, errors.New("tls: internal error: sending non-handshake message to QUIC transport")
		}
		c.quicWriteCryptoData(c.out.level, data)
		if !c.buffering {
			if _, err := c.flush(); err != nil {
				return 0, err
			}
		}
		return len(data), nil
	}

	outBufPtr := outBufPool.Get().(*[]byte)
	outBuf := *outBufPtr
	defer func() {
		// You might be tempted to simplify this by just passing &outBuf to Put,
		// but that would make the local copy of the outBuf slice header escape
		// to the heap, causing an allocation. Instead, we keep around the
		// pointer to the slice header returned by Get, which is already on the
		// heap, and overwrite and return that.
		*outBufPtr = outBuf
		outBufPool.Put(outBufPtr)
	}()

	var n int
	for len(data) > 0 {
		m := len(data)
		if maxPayload := c.maxPayloadSizeForWrite(typ); m > maxPayload {
			m = maxPayload
		}

		_, outBuf = sliceForAppend(outBuf[:0], recordHeaderLen)
		outBuf[0] = byte(typ)
		vers := c.vers
		if vers == 0 {
			// Some TLS servers fail if the record version is
			// greater than TLS 1.0 for the initial ClientHello.
			vers = VersionTLS10
		} else if vers == VersionTLS13 {
			// TLS 1.3 froze the record layer version to 1.2.
			// See RFC 8446, Section 5.1.
			vers = VersionTLS12
		}
		outBuf[1] = byte(vers >> 8)
		outBuf[2] = byte(vers)
		outBuf[3] = byte(m >> 8)
		outBuf[4] = byte(m)

		var err error
		outBuf, err = c.out.encrypt(outBuf, data[:m], c.config.rand())
		if err != nil {
			return n, err
		}
		if _, err := c.write(outBuf); err != nil {
			return n, err
		}
		n += m
		data = data[m:]
	}

	if typ == recordTypeChangeCipherSpec && c.vers != VersionTLS13 {
		if err := c.out.changeCipherSpec(); err != nil {
			return n, c.sendAlertLocked(err.(alert))
		}
	}

	return n, nil
}

// writeHandshakeRecord writes a handshake message to the connection and updates
// the record layer state. If transcript is non-nil the marshaled message is
// written to it.
func (c *Conn) writeHandshakeRecord(msg handshakeMessage, transcript transcriptHash) (int, error) {
	c.out.Lock()
	defer c.out.Unlock()

	data, err := msg.marshal()
	if err != nil {
		return 0, err
	}
	if transcript != nil {
		transcript.Write(data)
	}

	return c.writeRecordLocked(recordTypeHandshake, data)
}

// writeChangeCipherRecord writes a ChangeCipherSpec message to the connection and
// updates the record layer state.
func (c *Conn) writeChangeCipherRecord() error {
	c.out.Lock()
	defer c.out.Unlock()
	_, err := c.writeRecordLocked(recordTypeChangeCipherSpec, []byte{1})
	return err
}

// readHandshakeBytes reads handshake data until c.hand contains at least n bytes.
func (c *Conn) readHandshakeBytes(n int) error {
	if c.quic != nil {
		return c.quicReadHandshakeBytes(n)
	}
	for c.hand.Len() < n {
		if err := c.readRecord(); err != nil {
			return err
		}
	}
	return nil
}

// readHandshake reads the next handshake message from
// the record layer. If transcript is non-nil, the message
// is written to the passed transcriptHash.
func (c *Conn) readHandshake(transcript transcriptHash) (any, error) {
	if err := c.readHandshakeBytes(4); err != nil {
		return nil, err
	}
	data := c.hand.Bytes()

	maxHandshakeSize := maxHandshake
	// hasVers indicates we're past the first message, forcing someone trying to
	// make us just allocate a large buffer to at least do the initial part of
	// the handshake first.
	if c.haveVers && data[0] == typeCertificate {
		// Since certificate messages are likely to be the only messages that
		// can be larger than maxHandshake, we use a special limit for just
		// those messages.
		maxHandshakeSize = maxHandshakeCertificateMsg
	}

	n := int(data[1])<<16 | int(data[2])<<8 | int(data[3])
	if n > maxHandshakeSize {
		c.sendAlertLocked(alertInternalError)
		return nil, c.in.setErrorLocked(fmt.Errorf("tls: handshake message of length %d bytes exceeds maximum of %d bytes", n, maxHandshakeSize))
	}
	if err := c.readHandshakeBytes(4 + n); err != nil {
		return nil, err
	}
	data = c.hand.Next(4 + n)
	return c.unmarshalHandshakeMessage(data, transcript)
}

func (c *Conn) unmarshalHandshakeMessage(data []byte, transcript transcriptHash) (handshakeMessage, error) {
	var m handshakeMessage
	switch data[0] {
	case typeHelloRequest:
		m = new(helloRequestMsg)
	case typeClientHello:
		m = new(clientHelloMsg)
	case typeServerHello:
		m = new(serverHelloMsg)
	case typeNewSessionTicket:
		if c.vers == VersionTLS13 {
			m = new(newSessionTicketMsgTLS13)
		} else {
			m = new(newSessionTicketMsg)
		}
	case typeCertificate:
		if c.vers == VersionTLS13 {
			m = new(certificateMsgTLS13)
		} else {
			m = new(certificateMsg)
		}
	case typeCertificateRequest:
		if c.vers == VersionTLS13 {
			m = new(certificateRequestMsgTLS13)
		} else {
			m = &certificateRequestMsg{
				hasSignatureAlgorithm: c.vers >= VersionTLS12,
			}
		}
	case typeCertificateStatus:
		m = new(certificateStatusMsg)
	case typeServerKeyExchange:
		m = new(serverKeyExchangeMsg)
	case typeServerHelloDone:
		m = new(serverHelloDoneMsg)
	case typeClientKeyExchange:
		m = new(clientKeyExchangeMsg)
	case typeCertificateVerify:
		m = &certificateVerifyMsg{
			hasSignatureAlgorithm: c.vers >= VersionTLS12,
		}
	case typeFinished:
		m = new(finishedMsg)
	case typeEncryptedExtensions:
		m = new(encryptedExtensionsMsg)
	case typeEndOfEarlyData:
		m = new(endOfEarlyDataMsg)
	case typeKeyUpdate:
		m = new(keyUpdateMsg)
	default:
		return nil, c.in.setErrorLocked(c.sendAlert(alertUnexpectedMessage))
	}

	// The handshake message unmarshalers
	// expect to be able to keep references to data,
	// so pass in a fresh copy that won't be overwritten.
	data = append([]byte(nil), data...)

	if !m.unmarshal(data) {
		return nil, c.in.setErrorLocked(c.sendAlert(alertDecodeError))
	}

	if transcript != nil {
		transcript.Write(data)
	}

	return m, nil
}

var (
	errShutdown = errors.New("tls: protocol is shutdown")
)

// Write writes data to the connection.
//
// As Write calls [Conn.Handshake], in order to prevent indefinite blocking a deadline
// must be set for both [Conn.Read] and Write before Write is called when the handshake
// has not yet completed. See [Conn.SetDeadline], [Conn.SetReadDeadline], and
// [Conn.SetWriteDeadline].
func (c *Conn) Write(b []byte) (int, error) {
	// interlock with Close below
	for {
		x := c.activeCall.Load()
		if x&1 != 0 {
			return 0, net.ErrClosed
		}
		if c.activeCall.CompareAndSwap(x, x+2) {
			break
		}
	}
	defer c.activeCall.Add(-2)

	if err := c.Handshake(); err != nil {
		return 0, err
	}

	c.out.Lock()
	defer c.out.Unlock()

	if err := c.out.err; err != nil {
		return 0, err
	}

	if !c.isHandshakeComplete.Load() {
		return 0, alertInternalError
	}

	if c.closeNotifySent {
		return 0, errShutdown
	}

	// TLS 1.0 is susceptible to a chosen-plaintext
	// attack when using block mode ciphers due to predictable IVs.
	// This can be prevented by splitting each Application Data
	// record into two records, effectively randomizing the IV.
	//
	// https://www.openssl.org/~bodo/tls-cbc.txt
	// https://bugzilla.mozilla.org/show_bug.cgi?id=665814
	// https://www.imperialviolet.org/2012/01/15/beastfollowup.html

	var m int
	if len(b) > 1 && c.vers == VersionTLS10 {
		if _, ok := c.out.cipher.(cipher.BlockMode); ok {
			n, err := c.writeRecordLocked(recordTypeApplicationData, b[:1])
			if err != nil {
				return n, c.out.setErrorLocked(err)
			}
			m, b = 1, b[1:]
		}
	}

	n, err := c.writeRecordLocked(recordTypeApplicationData, b)
	return n + m, c.out.setErrorLocked(err)
}

// handleRenegotiation processes a HelloRequest handshake message.
func (c *Conn) handleRenegotiation() error {
	if c.vers == VersionTLS13 {
		return errors.New("tls: internal error: unexpected renegotiation")
	}

	msg, err := c.readHandshake(nil)
	if err != nil {
		return err
	}

	helloReq, ok := msg.(*helloRequestMsg)
	if !ok {
		c.sendAlert(alertUnexpectedMessage)
		return unexpectedMessageError(helloReq, msg)
	}

	if !c.isClient {
		return c.sendAlert(alertNoRenegotiation)
	}

	switch c.config.Renegotiation {
	case RenegotiateNever:
		return c.sendAlert(alertNoRenegotiation)
	case RenegotiateOnceAsClient:
		if c.handshakes > 1 {
			return c.sendAlert(alertNoRenegotiation)
		}
	case RenegotiateFreelyAsClient:
		// Ok.
	default:
		c.sendAlert(alertInternalError)
		return errors.New("tls: unknown Renegotiation value")
	}

	c.handshakeMutex.Lock()
	defer c.handshakeMutex.Unlock()

	c.isHandshakeComplete.Store(false)
	if c.handshakeErr = c.clientHandshake(context.Background()); c.handshakeErr == nil {
		c.handshakes++
	}
	return c.handshakeErr
}

// handlePostHandshakeMessage processes a handshake message arrived after the
// handshake is complete. Up to TLS 1.2, it indicates the start of a renegotiation.
func (c *Conn) handlePostHandshakeMessage() error {
	if c.vers != VersionTLS13 {
		return c.handleRenegotiation()
	}

	msg, err := c.readHandshake(nil)
	if err != nil {
		return err
	}
	c.retryCount++
	if c.retryCount > maxUselessRecords {
		c.sendAlert(alertUnexpectedMessage)
		return c.in.setErrorLocked(errors.New("tls: too many non-advancing records"))
	}

	switch msg := msg.(type) {
	case *newSessionTicketMsgTLS13:
		return c.handleNewSessionTicket(msg)
	case *keyUpdateMsg:
		return c.handleKeyUpdate(msg)
	}
	// The QUIC layer is supposed to treat an unexpected post-handshake CertificateRequest
	// as a QUIC-level PROTOCOL_VIOLATION error (RFC 9001, Section 4.4). Returning an
	// unexpected_message alert here doesn't provide it with enough information to distinguish
	// this condition from other unexpected messages. This is probably fine.
	c.sendAlert(alertUnexpectedMessage)
	return fmt.Errorf("tls: received unexpected handshake message of type %T", msg)
}

func (c *Conn) handleKeyUpdate(keyUpdate *keyUpdateMsg) error {
	if c.quic != nil {
		c.sendAlert(alertUnexpectedMessage)
		return c.in.setErrorLocked(errors.New("tls: received unexpected key update message"))
	}

	cipherSuite := cipherSuiteTLS13ByID(c.cipherSuite)
	if cipherSuite == nil {
		return c.in.setErrorLocked(c.sendAlert(alertInternalError))
	}

	if keyUpdate.updateRequested {
		c.out.Lock()
		defer c.out.Unlock()

		msg := &keyUpdateMsg{}
		msgBytes, err := msg.marshal()
		if err != nil {
			return err
		}
		_, err = c.writeRecordLocked(recordTypeHandshake, msgBytes)
		if err != nil {
			// Surface the error at the next write.
			c.out.setErrorLocked(err)
			return nil
		}

		newSecret := cipherSuite.nextTrafficSecret(c.out.trafficSecret)
		c.setWriteTrafficSecret(cipherSuite, QUICEncryptionLevelInitial, newSecret)
	}

	newSecret := cipherSuite.nextTrafficSecret(c.in.trafficSecret)
	if err := c.setReadTrafficSecret(cipherSuite, QUICEncryptionLevelInitial, newSecret, keyUpdate.updateRequested); err != nil {
		return err
	}

	return nil
}

// Read reads data from the connection.
//
// As Read calls [Conn.Handshake], in order to prevent indefinite blocking a deadline
// must be set for both Read and [Conn.Write] before Read is called when the handshake
// has not yet completed. See [Conn.SetDeadline], [Conn.SetReadDeadline], and
// [Conn.SetWriteDeadline].
func (c *Conn) Read(b []byte) (int, error) {
	if err := c.Handshake(); err != nil {
		return 0, err
	}
	if len(b) == 0 {
		// Put this after Handshake, in case people were calling
		// Read(nil) for the side effect of the Handshake.
		return 0, nil
	}

	c.in.Lock()
	defer c.in.Unlock()

	for c.input.Len() == 0 {
		if err := c.readRecord(); err != nil {
			return 0, err
		}
		for c.hand.Len() > 0 {
			if err := c.handlePostHandshakeMessage(); err != nil {
				return 0, err
			}
		}
	}

	n, _ := c.input.Read(b)

	// If a close-notify alert is waiting, read it so that we can return (n,
	// EOF) instead of (n, nil), to signal to the HTTP response reading
	// goroutine that the connection is now closed. This eliminates a race
	// where the HTTP response reading goroutine would otherwise not observe
	// the EOF until its next read, by which time a client goroutine might
	// have already tried to reuse the HTTP connection for a new request.
	// See https://golang.org/cl/76400046 and https://golang.org/issue/3514
	if n != 0 && c.input.Len() == 0 && c.rawInput.Len() > 0 &&
		recordType(c.rawInput.Bytes()[0]) == recordTypeAlert {
		if err := c.readRecord(); err != nil {
			return n, err // will be io.EOF on closeNotify
		}
	}

	return n, nil
}

// Close closes the connection.
func (c *Conn) Close() error {
	// Interlock with Conn.Write above.
	var x int32
	for {
		x = c.activeCall.Load()
		if x&1 != 0 {
			return net.ErrClosed
		}
		if c.activeCall.CompareAndSwap(x, x|1) {
			break
		}
	}
	if x != 0 {
		// io.Writer and io.Closer should not be used concurrently.
		// If Close is called while a Write is currently in-flight,
		// interpret that as a sign that this Close is really just
		// being used to break the Write and/or clean up resources and
		// avoid sending the alertCloseNotify, which may block
		// waiting on handshakeMutex or the c.out mutex.
		return c.conn.Close()
	}

	var alertErr error
	if c.isHandshakeComplete.Load() {
		if err := c.closeNotify(); err != nil {
			alertErr = fmt.Errorf("tls: failed to send closeNotify alert (but connection was closed anyway): %w", err)
		}
	}

	if err := c.conn.Close(); err != nil {
		return err
	}
	return alertErr
}

var errEarlyCloseWrite = errors.New("tls: CloseWrite called before handshake complete")

// CloseWrite shuts down the writing side of the connection. It should only be
// called once the handshake has completed and does not call CloseWrite on the
// underlying connection. Most callers should just use [Conn.Close].
func (c *Conn) CloseWrite() error {
	if !c.isHandshakeComplete.Load() {
		return errEarlyCloseWrite
	}

	return c.closeNotify()
}

func (c *Conn) closeNotify() error {
	c.out.Lock()
	defer c.out.Unlock()

	if !c.closeNotifySent {
		// Set a Write Deadline to prevent possibly blocking forever.
		c.SetWriteDeadline(time.Now().Add(time.Second * 5))
		c.closeNotifyErr = c.sendAlertLocked(alertCloseNotify)
		c.closeNotifySent = true
		// Any subsequent writes will fail.
		c.SetWriteDeadline(time.Now())
	}
	return c.closeNotifyErr
}

// Handshake runs the client or server handshake
// protocol if it has not yet been run.
//
// Most uses of this package need not call Handshake explicitly: the
// first [Conn.Read] or [Conn.Write] will call it automatically.
//
// For control over canceling or setting a timeout on a handshake, use
// [Conn.HandshakeContext] or the [Dialer]'s DialContext method instead.
//
// In order to avoid denial of service attacks, the maximum RSA key size allowed
// in certificates sent by either the TLS server or client is limited to 8192
// bits. This limit can be overridden by setting tlsmaxrsasize in the GODEBUG
// environment variable (e.g. GODEBUG=tlsmaxrsasize=4096).
func (c *Conn) Handshake() error {
	return c.HandshakeContext(context.Background())
}

// HandshakeContext runs the client or server handshake
// protocol if it has not yet been run.
//
// The provided Context must be non-nil. If the context is canceled before
// the handshake is complete, the handshake is interrupted and an error is returned.
// Once the handshake has completed, cancellation of the context will not affect the
// connection.
//
// Most uses of this package need not call HandshakeContext explicitly: the
// first [Conn.Read] or [Conn.Write] will call it automatically.
func (c *Conn) HandshakeContext(ctx context.Context) error {
	// Delegate to unexported method for named return
	// without confusing documented signature.
	return c.handshakeContext(ctx)
}

func (c *Conn) handshakeContext(ctx context.Context) (ret error) {
	// Fast sync/atomic-based exit if there is no handshake in flight and the
	// last one succeeded without an error. Avoids the expensive context setup
	// and mutex for most Read and Write calls.
	if c.isHandshakeComplete.Load() {
		return nil
	}

	handshakeCtx, cancel := context.WithCancel(ctx)
	// Note: defer this before calling context.AfterFunc
	// so that we can tell the difference between the input being canceled and
	// this cancellation. In the former case, we need to close the connection.
	defer cancel()

	if c.quic != nil {
		c.quic.ctx = handshakeCtx
		c.quic.cancel = cancel
	} else if ctx.Done() != nil {
		// Close the connection if ctx is canceled before the function returns.
		stop := context.AfterFunc(ctx, func() {
			_ = c.conn.Close()
		})
		defer func() {
			if !stop() {
				// Return context error to user.
				ret = ctx.Err()
			}
		}()
	}

	c.handshakeMutex.Lock()
	defer c.handshakeMutex.Unlock()

	if err := c.handshakeErr; err != nil {
		return err
	}
	if c.isHandshakeComplete.Load() {
		return nil
	}

	c.in.Lock()
	defer c.in.Unlock()

	c.handshakeErr = c.handshakeFn(handshakeCtx)
	if c.handshakeErr == nil {
		c.handshakes++
	} else {
		// If an error occurred during the handshake try to flush the
		// alert that might be left in the buffer.
		c.flush()
	}

	if c.handshakeErr == nil && !c.isHandshakeComplete.Load() {
		c.handshakeErr = errors.New("tls: internal error: handshake should have had a result")
	}
	if c.handshakeErr != nil && c.isHandshakeComplete.Load() {
		panic("tls: internal error: handshake returned an error but is marked successful")
	}

	if c.quic != nil {
		if c.handshakeErr == nil {
			c.quicHandshakeComplete()
			// Provide the 1-RTT read secret now that the handshake is complete.
			// The QUIC layer MUST NOT decrypt 1-RTT packets prior to completing
			// the handshake (RFC 9001, Section 5.7).
			if err := c.quicSetReadSecret(QUICEncryptionLevelApplication, c.cipherSuite, c.in.trafficSecret); err != nil {
				return err
			}
		} else {
			c.out.Lock()
			a, ok := errors.AsType[alert](c.out.err)
			if !ok {
				a = alertInternalError
			}
			c.out.Unlock()
			// Return an error which wraps both the handshake error and
			// any alert error we may have sent, or alertInternalError
			// if we didn't send an alert.
			// Truncate the text of the alert to 0 characters.
			c.handshakeErr = fmt.Errorf("%w%.0w", c.handshakeErr, AlertError(a))
		}
		close(c.quic.blockedc)
		close(c.quic.signalc)
	}

	return c.handshakeErr
}

// ConnectionState returns basic TLS details about the connection.
//
// The returned [ConnectionState] is only meaningful after the handshake has
// completed, as reported by [ConnectionState.HandshakeComplete]; before then
// its fields are not populated. The handshake is run automatically by the
// first [Conn.Read] or [Conn.Write], or it can be triggered explicitly with
// [Conn.Handshake].
func (c *Conn) ConnectionState() ConnectionState {
	c.handshakeMutex.Lock()
	defer c.handshakeMutex.Unlock()
	return c.connectionStateLocked()
}

func (c *Conn) connectionStateLocked() ConnectionState {
	var state ConnectionState
	state.HandshakeComplete = c.isHandshakeComplete.Load()
	state.Version = c.vers
	state.NegotiatedProtocol = c.clientProtocol
	state.DidResume = c.didResume
	state.HelloRetryRequest = c.didHRR
	state.testingOnlyPeerSignatureAlgorithm = c.peerSigAlg
	state.CurveID = c.curveID
	state.NegotiatedProtocolIsMutual = true
	state.ServerName = c.serverName
	state.CipherSuite = c.cipherSuite
	state.PeerCertificates = c.peerCertificates
	state.LocalCertificate = c.localCertificate
	state.VerifiedChains = c.verifiedChains
	state.SignedCertificateTimestamps = c.scts
	state.OCSPResponse = c.ocspResponse
	if (!c.didResume || c.extMasterSecret) && c.vers != VersionTLS13 {
		if c.clientFinishedIsFirst {
			state.TLSUnique = c.clientFinished[:]
		} else {
			state.TLSUnique = c.serverFinished[:]
		}
	}
	if c.config.Renegotiation != RenegotiateNever {
		state.ekm = noEKMBecauseRenegotiation
	} else if c.vers != VersionTLS13 && !c.extMasterSecret {
		state.ekm = noEKMBecauseNoEMS
	} else {
		state.ekm = c.ekm
	}
	state.ECHAccepted = c.echAccepted
	return state
}

// OCSPResponse returns the stapled OCSP response from the TLS server, if
// any. (Only valid for client connections.)
func (c *Conn) OCSPResponse() []byte {
	c.handshakeMutex.Lock()
	defer c.handshakeMutex.Unlock()

	return c.ocspResponse
}

// VerifyHostname checks that the peer certificate chain is valid for
// connecting to host. If so, it returns nil; if not, it returns an error
// describing the problem.
func (c *Conn) VerifyHostname(host string) error {
	c.handshakeMutex.Lock()
	defer c.handshakeMutex.Unlock()
	if !c.isClient {
		return errors.New("tls: VerifyHostname called on TLS server connection")
	}
	if !c.isHandshakeComplete.Load() {
		return errors.New("tls: handshake has not yet been performed")
	}
	if len(c.verifiedChains) == 0 {
		return errors.New("tls: handshake did not verify certificate chain")
	}
	return c.peerCertificates[0].VerifyHostname(host)
}

// setReadTrafficSecret sets the read traffic secret for the given encryption level. If
// being called at the same time as setWriteTrafficSecret, the caller must ensure the call
// to setWriteTrafficSecret happens first so any alerts are sent at the write level.
func (c *Conn) setReadTrafficSecret(suite *cipherSuiteTLS13, level QUICEncryptionLevel, secret []byte, locked bool) error {
	// Ensure that there are no buffered handshake messages before changing the
	// read keys, since that can cause messages to be parsed that were encrypted
	// using old keys which are no longer appropriate.
	if c.hand.Len() != 0 {
		if locked {
			c.sendAlertLocked(alertUnexpectedMessage)
		} else {
			c.sendAlert(alertUnexpectedMessage)
		}
		return errors.New("tls: handshake buffer not empty before setting read traffic secret")
	}
	c.in.setTrafficSecret(suite, level, secret)
	return nil
}

// setWriteTrafficSecret sets the write traffic secret for the given encryption level. If
// being called at the same time as setReadTrafficSecret, the caller must ensure the call
// to setWriteTrafficSecret happens first so any alerts are sent at the write level.
func (c *Conn) setWriteTrafficSecret(suite *cipherSuiteTLS13, level QUICEncryptionLevel, secret []byte) {
	c.out.setTrafficSecret(suite, level, secret)
}

```

// === FILE: references/go/src/crypto/tls/defaults.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tls

import (
	"internal/godebug"
	"slices"
	_ "unsafe" // for linkname
)

// Defaults are collected in this file to allow distributions to more easily patch
// them to apply local policies.

// tlsmlkem=0 restores the pre-Go 1.24 default key exchanges.
var tlsmlkem = godebug.New("tlsmlkem")

// tlssecpmlkem=0 restores the pre-Go 1.26 default key exchanges.
var tlssecpmlkem = godebug.New("tlssecpmlkem")

// defaultCurveEnabled returns whether the key exchange c is enabled by default.
func defaultCurveEnabled(c CurveID) bool {
	switch c {
	case X25519, CurveP256, CurveP384, CurveP521:
		return true
	case X25519MLKEM768:
		return tlsmlkem.Value() != "0"
	case SecP256r1MLKEM768, SecP384r1MLKEM1024:
		return tlsmlkem.Value() != "0" && tlssecpmlkem.Value() != "0"
	default:
		return false
	}
}

// curvePreferenceOrder is the fixed preference order of key exchanges. It must
// include every supported key exchange.
func curvePreferenceOrder() []CurveID {
	return []CurveID{
		X25519MLKEM768, SecP256r1MLKEM768, SecP384r1MLKEM1024, MLKEM1024,
		X25519, CurveP256, CurveP384, CurveP521,
	}
}

// defaultSupportedSignatureAlgorithms returns the signature and hash algorithms that
// the code advertises and supports in a TLS 1.2+ ClientHello and in a TLS 1.2+
// CertificateRequest. The two fields are merged to match with TLS 1.3.
// Note that in TLS 1.2, the ECDSA algorithms are not constrained to P-256, etc.
func defaultSupportedSignatureAlgorithms() []SignatureScheme {
	return []SignatureScheme{
		MLDSA44,
		MLDSA65,
		MLDSA87,
		PSSWithSHA256,
		ECDSAWithP256AndSHA256,
		Ed25519,
		PSSWithSHA384,
		PSSWithSHA512,
		PKCS1WithSHA256,
		PKCS1WithSHA384,
		PKCS1WithSHA512,
		ECDSAWithP384AndSHA384,
		ECDSAWithP521AndSHA512,
		PKCS1WithSHA1,
		ECDSAWithSHA1,
	}
}

func supportedCipherSuites(aesGCMPreferred bool) []uint16 {
	if aesGCMPreferred {
		return slices.Clone(cipherSuitesPreferenceOrder)
	} else {
		return slices.Clone(cipherSuitesPreferenceOrderNoAES)
	}
}

func defaultCipherSuites(aesGCMPreferred bool) []uint16 {
	cipherSuites := supportedCipherSuites(aesGCMPreferred)
	return slices.DeleteFunc(cipherSuites, func(c uint16) bool {
		return disabledCipherSuites[c]
	})
}

// defaultCipherSuitesTLS13 is also the preference order, since there are no
// disabled by default TLS 1.3 cipher suites. The same AES vs ChaCha20 logic as
// cipherSuitesPreferenceOrder applies.
//
// defaultCipherSuitesTLS13 should be an internal detail,
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/quic-go/quic-go
//   - github.com/sagernet/quic-go
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
//go:linkname defaultCipherSuitesTLS13
var defaultCipherSuitesTLS13 = []uint16{
	TLS_AES_128_GCM_SHA256,
	TLS_AES_256_GCM_SHA384,
	TLS_CHACHA20_POLY1305_SHA256,
}

// defaultCipherSuitesTLS13NoAES should be an internal detail,
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/quic-go/quic-go
//   - github.com/sagernet/quic-go
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
//go:linkname defaultCipherSuitesTLS13NoAES
var defaultCipherSuitesTLS13NoAES = []uint16{
	TLS_CHACHA20_POLY1305_SHA256,
	TLS_AES_128_GCM_SHA256,
	TLS_AES_256_GCM_SHA384,
}

```

// === FILE: references/go/src/crypto/tls/defaults_boring.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build boringcrypto

package tls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
)

// These Go+BoringCrypto policies mostly match BoringSSL's
// ssl_compliance_policy_fips_202205, which is based on NIST SP 800-52r2.
// https://cs.opensource.google/boringssl/boringssl/+/master:ssl/ssl_lib.cc;l=3289;drc=ea7a88fa
//
// P-521 is allowed per https://go.dev/issue/71757.
//
// They are applied when crypto/tls/fipsonly is imported with GOEXPERIMENT=boringcrypto.

var (
	allowedSupportedVersionsFIPS = []uint16{
		VersionTLS12,
		VersionTLS13,
	}
	allowedCurvePreferencesFIPS = []CurveID{
		CurveP256,
		CurveP384,
		CurveP521,
	}
	allowedSignatureAlgorithmsFIPS = []SignatureScheme{
		PSSWithSHA256,
		PSSWithSHA384,
		PSSWithSHA512,
		PKCS1WithSHA256,
		ECDSAWithP256AndSHA256,
		PKCS1WithSHA384,
		ECDSAWithP384AndSHA384,
		PKCS1WithSHA512,
		ECDSAWithP521AndSHA512,
	}
	allowedCipherSuitesFIPS = []uint16{
		TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	}
	allowedCipherSuitesTLS13FIPS = []uint16{
		TLS_AES_128_GCM_SHA256,
		TLS_AES_256_GCM_SHA384,
	}
)

func isCertificateAllowedFIPS(c *x509.Certificate) bool {
	// The key must be RSA 2048, RSA 3072, RSA 4096,
	// or ECDSA P-256, P-384, P-521.
	switch k := c.PublicKey.(type) {
	case *rsa.PublicKey:
		size := k.N.BitLen()
		return size == 2048 || size == 3072 || size == 4096
	case *ecdsa.PublicKey:
		return k.Curve == elliptic.P256() || k.Curve == elliptic.P384() || k.Curve == elliptic.P521()
	}

	return false
}

```

// === FILE: references/go/src/crypto/tls/defaults_fips140.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !boringcrypto

package tls

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/internal/boring"
	"crypto/mldsa"
	"crypto/rsa"
	"crypto/x509"
)

// These FIPS 140-3 policies allow anything approved by SP 800-140C
// and SP 800-140D, and tested as part of the Go Cryptographic Module.
//
// Notably, not SHA-1, 3DES, RC4, ChaCha20Poly1305, RSA PKCS #1 v1.5 key
// transport, or TLS 1.0—1.1 (because we don't test its KDF).
//
// These are not default lists, but filters to apply to the default or
// configured lists. Missing items are treated as if they were not implemented.
//
// They are applied when the fips140 GODEBUG is "on" or "only".

var (
	allowedSupportedVersionsFIPS = []uint16{
		VersionTLS12,
		VersionTLS13,
	}
	allowedCurvePreferencesFIPS = []CurveID{
		X25519MLKEM768,
		SecP256r1MLKEM768,
		SecP384r1MLKEM1024,
		MLKEM1024,
		CurveP256,
		CurveP384,
		CurveP521,
	}
	allowedSignatureAlgorithmsFIPS = []SignatureScheme{
		PSSWithSHA256,
		ECDSAWithP256AndSHA256,
		Ed25519,
		MLDSA44,
		MLDSA65,
		MLDSA87,
		PSSWithSHA384,
		PSSWithSHA512,
		PKCS1WithSHA256,
		PKCS1WithSHA384,
		PKCS1WithSHA512,
		ECDSAWithP384AndSHA384,
		ECDSAWithP521AndSHA512,
	}
	allowedCipherSuitesFIPS = []uint16{
		TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
		TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
	}
	allowedCipherSuitesTLS13FIPS = []uint16{
		TLS_AES_128_GCM_SHA256,
		TLS_AES_256_GCM_SHA384,
	}
)

func isCertificateAllowedFIPS(c *x509.Certificate) bool {
	switch k := c.PublicKey.(type) {
	case *rsa.PublicKey:
		return k.N.BitLen() >= 2048
	case *ecdsa.PublicKey:
		return k.Curve == elliptic.P256() || k.Curve == elliptic.P384() || k.Curve == elliptic.P521()
	case ed25519.PublicKey:
		return true
	case *mldsa.PublicKey:
		// Only for the native module.
		return !boring.Enabled
	default:
		return false
	}
}

```

// === FILE: references/go/src/crypto/tls/ech.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tls

import (
	"bytes"
	"crypto/hpke"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/cryptobyte"
)

type echCipher struct {
	KDFID  uint16
	AEADID uint16
}

type echExtension struct {
	Type uint16
	Data []byte
}

type echConfig struct {
	raw []byte

	Version uint16
	Length  uint16

	ConfigID             uint8
	KemID                uint16
	PublicKey            []byte
	SymmetricCipherSuite []echCipher

	MaxNameLength uint8
	PublicName    []byte
	Extensions    []echExtension
}

var errMalformedECHConfigList = errors.New("tls: malformed ECHConfigList")

type echConfigErr struct {
	field string
}

func (e *echConfigErr) Error() string {
	if e.field == "" {
		return "tls: malformed ECHConfig"
	}
	return fmt.Sprintf("tls: malformed ECHConfig, invalid %s field", e.field)
}

func parseECHConfig(enc []byte) (skip bool, ec echConfig, err error) {
	s := cryptobyte.String(enc)
	ec.raw = enc
	if !s.ReadUint16(&ec.Version) {
		return false, echConfig{}, &echConfigErr{"version"}
	}
	if !s.ReadUint16(&ec.Length) {
		return false, echConfig{}, &echConfigErr{"length"}
	}
	if len(ec.raw) < int(ec.Length)+4 {
		return false, echConfig{}, &echConfigErr{"length"}
	}
	ec.raw = ec.raw[:ec.Length+4]
	if ec.Version != extensionEncryptedClientHello {
		s.Skip(int(ec.Length))
		return true, echConfig{}, nil
	}
	if !s.ReadUint8(&ec.ConfigID) {
		return false, echConfig{}, &echConfigErr{"config_id"}
	}
	if !s.ReadUint16(&ec.KemID) {
		return false, echConfig{}, &echConfigErr{"kem_id"}
	}
	if !readUint16LengthPrefixed(&s, &ec.PublicKey) {
		return false, echConfig{}, &echConfigErr{"public_key"}
	}
	var cipherSuites cryptobyte.String
	if !s.ReadUint16LengthPrefixed(&cipherSuites) {
		return false, echConfig{}, &echConfigErr{"cipher_suites"}
	}
	for !cipherSuites.Empty() {
		var c echCipher
		if !cipherSuites.ReadUint16(&c.KDFID) {
			return false, echConfig{}, &echConfigErr{"cipher_suites kdf_id"}
		}
		if !cipherSuites.ReadUint16(&c.AEADID) {
			return false, echConfig{}, &echConfigErr{"cipher_suites aead_id"}
		}
		ec.SymmetricCipherSuite = append(ec.SymmetricCipherSuite, c)
	}
	if !s.ReadUint8(&ec.MaxNameLength) {
		return false, echConfig{}, &echConfigErr{"maximum_name_length"}
	}
	var publicName cryptobyte.String
	if !s.ReadUint8LengthPrefixed(&publicName) {
		return false, echConfig{}, &echConfigErr{"public_name"}
	}
	ec.PublicName = publicName
	var extensions cryptobyte.String
	if !s.ReadUint16LengthPrefixed(&extensions) {
		return false, echConfig{}, &echConfigErr{"extensions"}
	}
	for !extensions.Empty() {
		var e echExtension
		if !extensions.ReadUint16(&e.Type) {
			return false, echConfig{}, &echConfigErr{"extensions type"}
		}
		if !extensions.ReadUint16LengthPrefixed((*cryptobyte.String)(&e.Data)) {
			return false, echConfig{}, &echConfigErr{"extensions data"}
		}
		ec.Extensions = append(ec.Extensions, e)
	}

	return false, ec, nil
}

// parseECHConfigList parses a RFC 9849 ECHConfigList, returning a
// slice of parsed ECHConfigs, in the same order they were parsed, or an error
// if the list is malformed.
func parseECHConfigList(data []byte) ([]echConfig, error) {
	s := cryptobyte.String(data)
	var length uint16
	if !s.ReadUint16(&length) {
		return nil, errMalformedECHConfigList
	}
	if length != uint16(len(data)-2) {
		return nil, errMalformedECHConfigList
	}
	var configs []echConfig
	for len(s) > 0 {
		if len(s) < 4 {
			return nil, errors.New("tls: malformed ECHConfig")
		}
		configLen := uint16(s[2])<<8 | uint16(s[3])
		skip, ec, err := parseECHConfig(s)
		if err != nil {
			return nil, err
		}
		s = s[configLen+4:]
		if !skip {
			configs = append(configs, ec)
		}
	}
	return configs, nil
}

func pickECHConfig(list []echConfig) (*echConfig, hpke.PublicKey, hpke.KDF, hpke.AEAD) {
	for _, ec := range list {
		if !validDNSName(string(ec.PublicName)) {
			continue
		}
		var unsupportedExt bool
		for _, ext := range ec.Extensions {
			// If high order bit is set to 1 the extension is mandatory.
			// Since we don't support any extensions, if we see a mandatory
			// bit, we skip the config.
			if ext.Type&uint16(1<<15) != 0 {
				unsupportedExt = true
			}
		}
		if unsupportedExt {
			continue
		}
		kem, err := hpke.NewKEM(ec.KemID)
		if err != nil {
			continue
		}
		pub, err := kem.NewPublicKey(ec.PublicKey)
		if err != nil {
			// This is an error in the config, but killing the connection feels
			// excessive.
			continue
		}
		for _, cs := range ec.SymmetricCipherSuite {
			// All of the supported AEADs and KDFs are fine, rather than
			// imposing some sort of preference here, we just pick the first
			// valid suite.
			kdf, err := hpke.NewKDF(cs.KDFID)
			if err != nil {
				continue
			}
			// 0xFFFF is an export-only AEAD that cannot seal/open, making
			// it an invalid choice for encrypting ClientHelloInner.
			if cs.AEADID == 0xFFFF {
				continue
			}
			aead, err := hpke.NewAEAD(cs.AEADID)
			if err != nil {
				continue
			}
			return &ec, pub, kdf, aead
		}
	}
	return nil, nil, nil, nil
}

func encodeInnerClientHello(inner *clientHelloMsg, maxNameLength int) ([]byte, error) {
	h, err := inner.marshalMsg(true)
	if err != nil {
		return nil, err
	}
	h = h[4:] // strip four byte prefix

	var paddingLen int
	if inner.serverName != "" {
		paddingLen = max(0, maxNameLength-len(inner.serverName))
	} else {
		paddingLen = maxNameLength + 9
	}
	paddingLen += 31 - ((len(h) + paddingLen - 1) % 32)

	return append(h, make([]byte, paddingLen)...), nil
}

func skipUint8LengthPrefixed(s *cryptobyte.String) bool {
	var skip uint8
	if !s.ReadUint8(&skip) {
		return false
	}
	return s.Skip(int(skip))
}

func skipUint16LengthPrefixed(s *cryptobyte.String) bool {
	var skip uint16
	if !s.ReadUint16(&skip) {
		return false
	}
	return s.Skip(int(skip))
}

type rawExtension struct {
	extType uint16
	data    []byte
}

func extractRawExtensions(hello *clientHelloMsg) ([]rawExtension, error) {
	s := cryptobyte.String(hello.original)
	if !s.Skip(4+2+32) || // header, version, random
		!skipUint8LengthPrefixed(&s) || // session ID
		!skipUint16LengthPrefixed(&s) || // cipher suites
		!skipUint8LengthPrefixed(&s) { // compression methods
		return nil, errors.New("tls: malformed outer client hello")
	}
	var rawExtensions []rawExtension
	var extensions cryptobyte.String
	if !s.ReadUint16LengthPrefixed(&extensions) {
		return nil, errors.New("tls: malformed outer client hello")
	}

	for !extensions.Empty() {
		var extension uint16
		var extData cryptobyte.String
		if !extensions.ReadUint16(&extension) ||
			!extensions.ReadUint16LengthPrefixed(&extData) {
			return nil, errors.New("tls: invalid inner client hello")
		}
		rawExtensions = append(rawExtensions, rawExtension{extension, extData})
	}
	return rawExtensions, nil
}

func decodeInnerClientHello(outer *clientHelloMsg, encoded []byte) (*clientHelloMsg, error) {
	// Reconstructing the inner client hello from its encoded form is somewhat
	// complicated. It is missing its header (message type and length), session
	// ID, and the extensions may be compressed. Since we need to put the
	// extensions back in the same order as they were in the raw outer hello,
	// and since we don't store the raw extensions, or the order we parsed them
	// in, we need to reparse the raw extensions from the outer hello in order
	// to properly insert them into the inner hello. This _should_ result in raw
	// bytes which match the hello as it was generated by the client.
	innerReader := cryptobyte.String(encoded)
	var versionAndRandom, sessionID, cipherSuites, compressionMethods []byte
	var extensions cryptobyte.String
	if !innerReader.ReadBytes(&versionAndRandom, 2+32) ||
		!readUint8LengthPrefixed(&innerReader, &sessionID) ||
		len(sessionID) != 0 ||
		!readUint16LengthPrefixed(&innerReader, &cipherSuites) ||
		!readUint8LengthPrefixed(&innerReader, &compressionMethods) ||
		!innerReader.ReadUint16LengthPrefixed(&extensions) {
		return nil, errors.New("tls: invalid inner client hello")
	}

	// The specification says we must verify that the trailing padding is all
	// zeros. This is kind of weird for TLS messages, where we generally just
	// throw away any trailing garbage.
	for _, p := range innerReader {
		if p != 0 {
			return nil, errors.New("tls: invalid inner client hello")
		}
	}

	rawOuterExts, err := extractRawExtensions(outer)
	if err != nil {
		return nil, err
	}

	recon := cryptobyte.NewBuilder(nil)
	recon.AddUint8(typeClientHello)
	recon.AddUint24LengthPrefixed(func(recon *cryptobyte.Builder) {
		recon.AddBytes(versionAndRandom)
		recon.AddUint8LengthPrefixed(func(recon *cryptobyte.Builder) {
			recon.AddBytes(outer.sessionId)
		})
		recon.AddUint16LengthPrefixed(func(recon *cryptobyte.Builder) {
			recon.AddBytes(cipherSuites)
		})
		recon.AddUint8LengthPrefixed(func(recon *cryptobyte.Builder) {
			recon.AddBytes(compressionMethods)
		})
		recon.AddUint16LengthPrefixed(func(recon *cryptobyte.Builder) {
			for !extensions.Empty() {
				var extension uint16
				var extData cryptobyte.String
				if !extensions.ReadUint16(&extension) ||
					!extensions.ReadUint16LengthPrefixed(&extData) {
					recon.SetError(errors.New("tls: invalid inner client hello"))
					return
				}
				if extension == extensionECHOuterExtensions {
					if !extData.ReadUint8LengthPrefixed(&extData) {
						recon.SetError(errors.New("tls: invalid inner client hello"))
						return
					}
					var i int
					for !extData.Empty() {
						var extType uint16
						if !extData.ReadUint16(&extType) {
							recon.SetError(errors.New("tls: invalid inner client hello"))
							return
						}
						if extType == extensionEncryptedClientHello {
							recon.SetError(errors.New("tls: invalid outer extensions"))
							return
						}
						for ; i <= len(rawOuterExts); i++ {
							if i == len(rawOuterExts) {
								recon.SetError(errors.New("tls: invalid outer extensions"))
								return
							}
							if rawOuterExts[i].extType == extType {
								break
							}
						}
						recon.AddUint16(rawOuterExts[i].extType)
						recon.AddUint16LengthPrefixed(func(recon *cryptobyte.Builder) {
							recon.AddBytes(rawOuterExts[i].data)
						})
					}
				} else {
					recon.AddUint16(extension)
					recon.AddUint16LengthPrefixed(func(recon *cryptobyte.Builder) {
						recon.AddBytes(extData)
					})
				}
			}
		})
	})

	reconBytes, err := recon.Bytes()
	if err != nil {
		return nil, err
	}
	inner := &clientHelloMsg{}
	if !inner.unmarshal(reconBytes) {
		return nil, errors.New("tls: invalid reconstructed inner client hello")
	}

	if !bytes.Equal(inner.encryptedClientHello, []byte{uint8(innerECHExt)}) {
		return nil, errInvalidECHExt
	}

	hasTLS13 := false
	for _, v := range inner.supportedVersions {
		// Skip GREASE values (values of the form 0x?A0A).
		// GREASE (Generate Random Extensions And Sustain Extensibility) is a mechanism used by
		// browsers like Chrome to ensure TLS implementations correctly ignore unknown values.
		// GREASE values follow a specific pattern: 0x?A0A, where ? can be any hex digit.
		// These values should be ignored when processing supported TLS versions.
		if v&0x0F0F == 0x0A0A && v&0xff == v>>8 {
			continue
		}

		// Ensure at least TLS 1.3 is offered.
		if v == VersionTLS13 {
			hasTLS13 = true
		} else if v < VersionTLS13 {
			// Reject if any non-GREASE value is below TLS 1.3, as ECH requires TLS 1.3+.
			return nil, errors.New("tls: client sent encrypted_client_hello extension with unsupported versions")
		}
	}

	if !hasTLS13 {
		return nil, errors.New("tls: client sent encrypted_client_hello extension but did not offer TLS 1.3")
	}

	return inner, nil
}

func decryptECHPayload(context *hpke.Recipient, hello, payload []byte) ([]byte, error) {
	outerAAD := bytes.Replace(hello[4:], payload, make([]byte, len(payload)), 1)
	return context.Open(outerAAD, payload)
}

func generateOuterECHExt(id uint8, kdfID, aeadID uint16, encodedKey []byte, payload []byte) ([]byte, error) {
	var b cryptobyte.Builder
	b.AddUint8(0) // outer
	b.AddUint16(kdfID)
	b.AddUint16(aeadID)
	b.AddUint8(id)
	b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) { b.AddBytes(encodedKey) })
	b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) { b.AddBytes(payload) })
	return b.Bytes()
}

func computeAndUpdateOuterECHExtension(outer, inner *clientHelloMsg, ech *echClientContext, useKey bool) error {
	var encapKey []byte
	if useKey {
		encapKey = ech.encapsulatedKey
	}
	encodedInner, err := encodeInnerClientHello(inner, int(ech.config.MaxNameLength))
	if err != nil {
		return err
	}
	// NOTE: the tag lengths for all of the supported AEADs are the same (16
	// bytes), so we have hardcoded it here. If we add support for another AEAD
	// with a different tag length, we will need to change this.
	encryptedLen := len(encodedInner) + 16 // AEAD tag length
	outer.encryptedClientHello, err = generateOuterECHExt(ech.config.ConfigID, ech.kdfID, ech.aeadID, encapKey, make([]byte, encryptedLen))
	if err != nil {
		return err
	}
	serializedOuter, err := outer.marshal()
	if err != nil {
		return err
	}
	serializedOuter = serializedOuter[4:] // strip the four byte prefix
	encryptedInner, err := ech.hpkeContext.Seal(serializedOuter, encodedInner)
	if err != nil {
		return err
	}
	outer.encryptedClientHello, err = generateOuterECHExt(ech.config.ConfigID, ech.kdfID, ech.aeadID, encapKey, encryptedInner)
	if err != nil {
		return err
	}
	return nil
}

// validDNSName is a rather rudimentary check for the validity of a DNS name.
// This is used to check if the public_name in a ECHConfig is valid when we are
// picking a config. This can be somewhat lax because even if we pick a
// valid-looking name, the DNS layer will later reject it anyway.
func validDNSName(name string) bool {
	if len(name) > 253 {
		return false
	}
	labels := strings.Split(name, ".")
	if len(labels) <= 1 {
		return false
	}
	for _, l := range labels {
		labelLen := len(l)
		if labelLen == 0 {
			return false
		}
		for i, r := range l {
			if r == '-' && (i == 0 || i == labelLen-1) {
				return false
			}
			if (r < '0' || r > '9') && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && r != '-' {
				return false
			}
		}
	}
	return true
}

// ECHRejectionError is the error type returned when ECH is rejected by a remote
// server. If the server offered a ECHConfigList to use for retries, the
// RetryConfigList field will contain this list.
//
// The client may treat an ECHRejectionError with an empty set of RetryConfigs
// as a secure signal from the server.
type ECHRejectionError struct {
	RetryConfigList []byte
}

func (e *ECHRejectionError) Error() string {
	return "tls: server rejected ECH"
}

var errMalformedECHExt = errors.New("tls: malformed encrypted_client_hello extension")
var errInvalidECHExt = errors.New("tls: client sent invalid encrypted_client_hello extension")

type echExtType uint8

const (
	innerECHExt echExtType = 1
	outerECHExt echExtType = 0
)

func parseECHExt(ext []byte) (echType echExtType, cs echCipher, configID uint8, encap []byte, payload []byte, err error) {
	data := make([]byte, len(ext))
	copy(data, ext)
	s := cryptobyte.String(data)
	var echInt uint8
	if !s.ReadUint8(&echInt) {
		err = errMalformedECHExt
		return
	}
	echType = echExtType(echInt)
	if echType == innerECHExt {
		if !s.Empty() {
			err = errMalformedECHExt
			return
		}
		return echType, cs, 0, nil, nil, nil
	}
	if echType != outerECHExt {
		err = errInvalidECHExt
		return
	}
	if !s.ReadUint16(&cs.KDFID) {
		err = errMalformedECHExt
		return
	}
	if !s.ReadUint16(&cs.AEADID) {
		err = errMalformedECHExt
		return
	}
	if !s.ReadUint8(&configID) {
		err = errMalformedECHExt
		return
	}
	if !readUint16LengthPrefixed(&s, &encap) {
		err = errMalformedECHExt
		return
	}
	if !readUint16LengthPrefixed(&s, &payload) {
		err = errMalformedECHExt
		return
	}

	// NOTE: clone encap and payload so that mutating them does not mutate the
	// raw extension bytes.
	return echType, cs, configID, bytes.Clone(encap), bytes.Clone(payload), nil
}

func (c *Conn) processECHClientHello(outer *clientHelloMsg, echKeys []EncryptedClientHelloKey) (*clientHelloMsg, *echServerContext, error) {
	echType, echCiphersuite, configID, encap, payload, err := parseECHExt(outer.encryptedClientHello)
	if err != nil {
		if errors.Is(err, errInvalidECHExt) {
			c.sendAlert(alertIllegalParameter)
		} else {
			c.sendAlert(alertDecodeError)
		}

		return nil, nil, errInvalidECHExt
	}

	if echType == innerECHExt {
		return outer, &echServerContext{inner: true}, nil
	}

	if len(echKeys) == 0 {
		return outer, nil, nil
	}

	for _, echKey := range echKeys {
		skip, config, err := parseECHConfig(echKey.Config)
		if err != nil {
			c.sendAlert(alertInternalError)
			return nil, nil, fmt.Errorf("tls: invalid EncryptedClientHelloKey Config: %s", err)
		}
		if skip {
			continue
		}
		kem, err := hpke.NewKEM(config.KemID)
		if err != nil {
			c.sendAlert(alertInternalError)
			return nil, nil, fmt.Errorf("tls: invalid EncryptedClientHelloKey Config KEM: %s", err)
		}
		echPriv, err := kem.NewPrivateKey(echKey.PrivateKey)
		if err != nil {
			c.sendAlert(alertInternalError)
			return nil, nil, fmt.Errorf("tls: invalid EncryptedClientHelloKey PrivateKey: %s", err)
		}
		kdf, err := hpke.NewKDF(echCiphersuite.KDFID)
		if err != nil {
			c.sendAlert(alertInternalError)
			return nil, nil, fmt.Errorf("tls: invalid EncryptedClientHelloKey Config KDF: %s", err)
		}
		aead, err := hpke.NewAEAD(echCiphersuite.AEADID)
		if err != nil {
			c.sendAlert(alertInternalError)
			return nil, nil, fmt.Errorf("tls: invalid EncryptedClientHelloKey Config AEAD: %s", err)
		}
		info := append([]byte("tls ech\x00"), echKey.Config...)
		hpkeContext, err := hpke.NewRecipient(encap, echPriv, kdf, aead, info)
		if err != nil {
			// attempt next trial decryption
			continue
		}

		encodedInner, err := decryptECHPayload(hpkeContext, outer.original, payload)
		if err != nil {
			// attempt next trial decryption
			continue
		}

		// NOTE: we do not enforce that the sent server_name matches the ECH
		// configs PublicName, since this is not particularly important, and
		// the client already had to know what it was in order to properly
		// encrypt the payload. This is only a MAY in the spec, so we're not
		// doing anything revolutionary.

		echInner, err := decodeInnerClientHello(outer, encodedInner)
		if err != nil {
			c.sendAlert(alertIllegalParameter)
			return nil, nil, errInvalidECHExt
		}

		c.echAccepted = true

		return echInner, &echServerContext{
			hpkeContext: hpkeContext,
			configID:    configID,
			ciphersuite: echCiphersuite,
		}, nil
	}

	return outer, nil, nil
}

func buildRetryConfigList(keys []EncryptedClientHelloKey) ([]byte, error) {
	var atLeastOneRetryConfig bool
	var retryBuilder cryptobyte.Builder
	retryBuilder.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
		for _, c := range keys {
			if !c.SendAsRetry {
				continue
			}
			atLeastOneRetryConfig = true
			b.AddBytes(c.Config)
		}
	})
	if !atLeastOneRetryConfig {
		return nil, nil
	}
	return retryBuilder.Bytes()
}

```

// === FILE: references/go/src/crypto/tls/fipsonly/fipsonly.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build boringcrypto

// Package fipsonly restricts all TLS configuration to FIPS-approved settings.
//
// The effect is triggered by importing the package anywhere in a program, as in:
//
//	import _ "crypto/tls/fipsonly"
//
// This package only exists when using Go compiled with GOEXPERIMENT=boringcrypto.
package fipsonly

// This functionality is provided as a side effect of an import to make
// it trivial to add to an existing program. It requires only a single line
// added to an existing source file, or it can be done by adding a whole
// new source file and not modifying any existing source files.

import (
	"crypto/internal/boring/sig"
	"crypto/tls/internal/fips140tls"
)

func init() {
	fips140tls.Force()
	sig.FIPSOnly()
}

```

// === FILE: references/go/src/crypto/tls/generate_cert.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

// Generate a self-signed X.509 certificate for a TLS server. Outputs to
// 'cert.pem' and 'key.pem' and will overwrite existing files.

package main

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/mldsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"log"
	"math/big"
	"net"
	"os"
	"strings"
	"time"
)

var (
	host       = flag.String("host", "", "Comma-separated hostnames and IPs to generate a certificate for")
	validFrom  = flag.String("start-date", "", "Creation date formatted as Jan 1 15:04:05 2011")
	validFor   = flag.Duration("duration", 365*24*time.Hour, "Duration that certificate is valid for")
	isCA       = flag.Bool("ca", false, "whether this cert should be its own Certificate Authority")
	rsaBits    = flag.Int("rsa-bits", 2048, "Size of RSA key to generate. Ignored if --ecdsa-curve is set")
	ecdsaCurve = flag.String("ecdsa-curve", "", "ECDSA curve to use to generate a key. Valid values are P224, P256 (recommended), P384, P521")
	ed25519Key = flag.Bool("ed25519", false, "Generate an Ed25519 key")
	mldsaKey   = flag.Bool("mldsa", false, "Generate an ML-DSA-44 key")
)

func publicKey(priv any) any {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	case ed25519.PrivateKey:
		return k.Public().(ed25519.PublicKey)
	case *mldsa.PrivateKey:
		return k.PublicKey()
	default:
		return nil
	}
}

func main() {
	flag.Parse()

	if len(*host) == 0 {
		log.Fatalf("Missing required --host parameter")
	}

	var priv any
	var err error
	switch *ecdsaCurve {
	case "":
		if *ed25519Key {
			_, priv, err = ed25519.GenerateKey(rand.Reader)
		} else if *mldsaKey {
			priv, err = mldsa.GenerateKey(mldsa.MLDSA44())
		} else {
			priv, err = rsa.GenerateKey(rand.Reader, *rsaBits)
		}
	case "P224":
		priv, err = ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	case "P256":
		priv, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case "P384":
		priv, err = ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	case "P521":
		priv, err = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	default:
		log.Fatalf("Unrecognized elliptic curve: %q", *ecdsaCurve)
	}
	if err != nil {
		log.Fatalf("Failed to generate private key: %v", err)
	}

	// ECDSA, ED25519, ML-DSA, and RSA subject keys should have the
	// DigitalSignature KeyUsage bits set in the x509.Certificate template
	keyUsage := x509.KeyUsageDigitalSignature
	// Only RSA subject keys should have the KeyEncipherment KeyUsage bits set. In
	// the context of TLS this KeyUsage is particular to RSA key exchange and
	// authentication.
	if _, isRSA := priv.(*rsa.PrivateKey); isRSA {
		keyUsage |= x509.KeyUsageKeyEncipherment
	}

	var notBefore time.Time
	if len(*validFrom) == 0 {
		notBefore = time.Now()
	} else {
		notBefore, err = time.Parse("Jan 2 15:04:05 2006", *validFrom)
		if err != nil {
			log.Fatalf("Failed to parse creation date: %v", err)
		}
	}

	notAfter := notBefore.Add(*validFor)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("Failed to generate serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	hosts := strings.Split(*host, ",")
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	if *isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(priv), priv)
	if err != nil {
		log.Fatalf("Failed to create certificate: %v", err)
	}

	certOut, err := os.Create("cert.pem")
	if err != nil {
		log.Fatalf("Failed to open cert.pem for writing: %v", err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		log.Fatalf("Failed to write data to cert.pem: %v", err)
	}
	if err := certOut.Close(); err != nil {
		log.Fatalf("Error closing cert.pem: %v", err)
	}
	log.Print("wrote cert.pem\n")

	keyOut, err := os.OpenFile("key.pem", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Failed to open key.pem for writing: %v", err)
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		log.Fatalf("Unable to marshal private key: %v", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		log.Fatalf("Failed to write data to key.pem: %v", err)
	}
	if err := keyOut.Close(); err != nil {
		log.Fatalf("Error closing key.pem: %v", err)
	}
	log.Print("wrote key.pem\n")
}

```

// === FILE: references/go/src/crypto/tls/handshake_client.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tls

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/hpke"
	"crypto/internal/fips140/tls13"
	"crypto/mldsa"
	"crypto/rsa"
	"crypto/subtle"
	"crypto/tls/internal/fips140tls"
	"crypto/x509"
	"errors"
	"fmt"
	"hash"
	"internal/godebug"
	"io"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"
)

type clientHandshakeState struct {
	c            *Conn
	ctx          context.Context
	serverHello  *serverHelloMsg
	hello        *clientHelloMsg
	suite        *cipherSuite
	finishedHash finishedHash
	masterSecret []byte
	session      *SessionState // the session being resumed
	ticket       []byte        // a fresh ticket received during this handshake
}

func (c *Conn) makeClientHello() (*clientHelloMsg, *keySharePrivateKeys, *echClientContext, error) {
	config := c.config
	if len(config.ServerName) == 0 && !config.InsecureSkipVerify {
		return nil, nil, nil, errors.New("tls: either ServerName or InsecureSkipVerify must be specified in the tls.Config")
	}

	nextProtosLength := 0
	for _, proto := range config.NextProtos {
		if l := len(proto); l == 0 || l > 255 {
			return nil, nil, nil, errors.New("tls: invalid NextProtos value")
		} else {
			nextProtosLength += 1 + l
		}
	}
	if nextProtosLength > 0xffff {
		return nil, nil, nil, errors.New("tls: NextProtos values too large")
	}

	supportedVersions := config.supportedVersions(roleClient, c.quic != nil)
	if len(supportedVersions) == 0 {
		return nil, nil, nil, errors.New("tls: no supported versions satisfy MinVersion and MaxVersion")
	}
	// Since supportedVersions is sorted in descending order, the first element
	// is the maximum version and the last element is the minimum version.
	maxVersion := supportedVersions[0]
	minVersion := supportedVersions[len(supportedVersions)-1]

	hello := &clientHelloMsg{
		vers:                         maxVersion,
		compressionMethods:           []uint8{compressionNone},
		random:                       make([]byte, 32),
		extendedMasterSecret:         true,
		ocspStapling:                 true,
		scts:                         true,
		serverName:                   hostnameInSNI(config.ServerName),
		supportedCurves:              config.curvePreferences(maxVersion),
		supportedPoints:              []uint8{pointFormatUncompressed},
		secureRenegotiationSupported: true,
		alpnProtocols:                config.NextProtos,
		supportedVersions:            supportedVersions,
	}

	// The version at the beginning of the ClientHello was capped at TLS 1.2
	// for compatibility reasons. The supported_versions extension is used
	// to negotiate versions now. See RFC 8446, Section 4.2.1.
	if hello.vers > VersionTLS12 {
		hello.vers = VersionTLS12
	}

	if c.handshakes > 0 {
		hello.secureRenegotiation = c.clientFinished[:]
	}

	hello.cipherSuites = config.cipherSuites(hasAESGCMHardwareSupport)
	// Don't advertise TLS 1.2-only cipher suites unless we're attempting TLS 1.2.
	if maxVersion < VersionTLS12 {
		hello.cipherSuites = slices.DeleteFunc(hello.cipherSuites, func(id uint16) bool {
			return cipherSuiteByID(id).flags&suiteTLS12 != 0
		})
	}

	_, err := io.ReadFull(config.rand(), hello.random)
	if err != nil {
		return nil, nil, nil, errors.New("tls: short read from Rand: " + err.Error())
	}

	// A random session ID is used to detect when the server accepted a ticket
	// and is resuming a session (see RFC 5077). In TLS 1.3, it's always set as
	// a compatibility measure (see RFC 8446, Section 4.1.2).
	//
	// The session ID is not set for QUIC connections (see RFC 9001, Section 8.4).
	if c.quic == nil {
		hello.sessionId = make([]byte, 32)
		if _, err := io.ReadFull(config.rand(), hello.sessionId); err != nil {
			return nil, nil, nil, errors.New("tls: short read from Rand: " + err.Error())
		}
	}

	if maxVersion >= VersionTLS12 {
		hello.supportedSignatureAlgorithms = supportedSignatureAlgorithms(minVersion, maxVersion)
		hello.supportedSignatureAlgorithmsCert = supportedSignatureAlgorithmsCert(minVersion, maxVersion)
	}

	var keyShareKeys *keySharePrivateKeys
	if maxVersion >= VersionTLS13 {
		// Reset the list of ciphers when the client only supports TLS 1.3.
		if minVersion >= VersionTLS13 {
			hello.cipherSuites = nil
		}

		if fips140tls.Required() {
			hello.cipherSuites = append(hello.cipherSuites, allowedCipherSuitesTLS13FIPS...)
		} else if hasAESGCMHardwareSupport {
			hello.cipherSuites = append(hello.cipherSuites, defaultCipherSuitesTLS13...)
		} else {
			hello.cipherSuites = append(hello.cipherSuites, defaultCipherSuitesTLS13NoAES...)
		}

		if len(hello.supportedCurves) == 0 {
			return nil, nil, nil, errors.New("tls: no supported key exchange methods (CurveIDs)")
		}
		// Since the order is fixed, the first one is always the one to send a
		// key share for. All the PQ hybrids sort first, and produce a fallback
		// ECDH share.
		curveID := hello.supportedCurves[0]
		ke, err := keyExchangeForCurveID(curveID)
		if err != nil {
			return nil, nil, nil, errors.New("tls: internal error: supportsCurve accepted unimplemented curve")
		}
		keyShareKeys, hello.keyShares, err = ke.keyShares(config.rand())
		if err != nil {
			return nil, nil, nil, err
		}
		// Only send the fallback ECDH share if the corresponding CurveID is enabled.
		if len(hello.keyShares) == 2 && !slices.Contains(hello.supportedCurves, hello.keyShares[1].group) {
			hello.keyShares = hello.keyShares[:1]
		}
	}

	if c.quic != nil {
		p, err := c.quicGetTransportParameters()
		if err != nil {
			return nil, nil, nil, err
		}
		if p == nil {
			p = []byte{}
		}
		hello.quicTransportParameters = p
	}

	var ech *echClientContext
	if c.config.EncryptedClientHelloConfigList != nil {
		if c.config.MinVersion != 0 && c.config.MinVersion < VersionTLS13 {
			return nil, nil, nil, errors.New("tls: MinVersion must be >= VersionTLS13 if EncryptedClientHelloConfigList is populated")
		}
		if c.config.MaxVersion != 0 && c.config.MaxVersion <= VersionTLS12 {
			return nil, nil, nil, errors.New("tls: MaxVersion must be >= VersionTLS13 if EncryptedClientHelloConfigList is populated")
		}
		echConfigs, err := parseECHConfigList(c.config.EncryptedClientHelloConfigList)
		if err != nil {
			return nil, nil, nil, err
		}
		echConfig, echPK, kdf, aead := pickECHConfig(echConfigs)
		if echConfig == nil {
			return nil, nil, nil, errors.New("tls: EncryptedClientHelloConfigList contains no valid configs")
		}
		ech = &echClientContext{config: echConfig, kdfID: kdf.ID(), aeadID: aead.ID()}
		hello.encryptedClientHello = []byte{1} // indicate inner hello
		// We need to explicitly set these 1.2 fields to nil, as we do not
		// marshal them when encoding the inner hello, otherwise transcripts
		// will later mismatch.
		hello.supportedPoints = nil
		hello.ticketSupported = false
		hello.secureRenegotiationSupported = false
		hello.extendedMasterSecret = false

		info := append([]byte("tls ech\x00"), ech.config.raw...)
		ech.encapsulatedKey, ech.hpkeContext, err = hpke.NewSender(echPK, kdf, aead, info)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	return hello, keyShareKeys, ech, nil
}

type echClientContext struct {
	config          *echConfig
	hpkeContext     *hpke.Sender
	encapsulatedKey []byte
	innerHello      *clientHelloMsg
	innerTranscript hash.Hash
	kdfID           uint16
	aeadID          uint16
	echRejected     bool
	retryConfigs    []byte
}

func (c *Conn) clientHandshake(ctx context.Context) (err error) {
	if c.config == nil {
		c.config = defaultConfig()
	}

	// This may be a renegotiation handshake, in which case some fields
	// need to be reset.
	c.didResume = false
	c.curveID = 0

	hello, keyShareKeys, ech, err := c.makeClientHello()
	if err != nil {
		return err
	}

	session, earlySecret, binderKey, err := c.loadSession(hello)
	if err != nil {
		return err
	}
	if session != nil {
		defer func() {
			// If we got a handshake failure when resuming a session, throw away
			// the session ticket. See RFC 5077, Section 3.2.
			//
			// RFC 8446 makes no mention of dropping tickets on failure, but it
			// does require servers to abort on invalid binders, so we need to
			// delete tickets to recover from a corrupted PSK.
			if err != nil {
				if cacheKey := c.clientSessionCacheKey(); cacheKey != "" {
					c.config.ClientSessionCache.Put(cacheKey, nil)
				}
			}
		}()
	}

	if ech != nil {
		// Split hello into inner and outer
		ech.innerHello = hello.clone()

		// Overwrite the server name in the outer hello with the public facing
		// name.
		hello.serverName = string(ech.config.PublicName)
		// Generate a new random for the outer hello.
		hello.random = make([]byte, 32)
		_, err = io.ReadFull(c.config.rand(), hello.random)
		if err != nil {
			return errors.New("tls: short read from Rand: " + err.Error())
		}

		// NOTE: we don't do PSK GREASE, in line with boringssl, it's meant to
		// work around _possibly_ broken middleboxes, but there is little-to-no
		// evidence that this is actually a problem.

		if err := computeAndUpdateOuterECHExtension(hello, ech.innerHello, ech, true); err != nil {
			return err
		}
	}

	c.serverName = hello.serverName

	if _, err := c.writeHandshakeRecord(hello, nil); err != nil {
		return err
	}

	if hello.earlyData {
		suite := cipherSuiteTLS13ByID(session.cipherSuite)
		transcript := suite.hash.New()
		transcriptHello := hello
		if ech != nil {
			transcriptHello = ech.innerHello
		}
		if err := transcriptMsg(transcriptHello, transcript); err != nil {
			return err
		}
		earlyTrafficSecret := earlySecret.ClientEarlyTrafficSecret(transcript)
		c.quicSetWriteSecret(QUICEncryptionLevelEarly, suite.id, earlyTrafficSecret)
	}

	// serverHelloMsg is not included in the transcript
	msg, err := c.readHandshake(nil)
	if err != nil {
		return err
	}

	serverHello, ok := msg.(*serverHelloMsg)
	if !ok {
		c.sendAlert(alertUnexpectedMessage)
		return unexpectedMessageError(serverHello, msg)
	}

	if err := c.pickTLSVersion(serverHello); err != nil {
		return err
	}

	// If we are negotiating a protocol version that's lower than what we
	// support, check for the server downgrade canaries.
	// See RFC 8446, Section 4.1.3.
	maxVers := c.config.maxSupportedVersion(roleClient, c.quic != nil)
	tls12Downgrade := string(serverHello.random[24:]) == downgradeCanaryTLS12
	tls11Downgrade := string(serverHello.random[24:]) == downgradeCanaryTLS11
	if maxVers == VersionTLS13 && c.vers <= VersionTLS12 && (tls12Downgrade || tls11Downgrade) ||
		maxVers == VersionTLS12 && c.vers <= VersionTLS11 && tls11Downgrade {
		c.sendAlert(alertIllegalParameter)
		return errors.New("tls: downgrade attempt detected, possibly due to a MitM attack or a broken middlebox")
	}

	if c.vers == VersionTLS13 {
		hs := &clientHandshakeStateTLS13{
			c:            c,
			ctx:          ctx,
			serverHello:  serverHello,
			hello:        hello,
			keyShareKeys: keyShareKeys,
			session:      session,
			earlySecret:  earlySecret,
			binderKey:    binderKey,
			echContext:   ech,
		}
		return hs.handshake()
	}

	hs := &clientHandshakeState{
		c:           c,
		ctx:         ctx,
		serverHello: serverHello,
		hello:       hello,
		session:     session,
	}
	return hs.handshake()
}

func (c *Conn) loadSession(hello *clientHelloMsg) (
	session *SessionState, earlySecret *tls13.EarlySecret, binderKey []byte, err error) {
	if c.config.SessionTicketsDisabled || c.config.ClientSessionCache == nil {
		return nil, nil, nil, nil
	}

	echInner := bytes.Equal(hello.encryptedClientHello, []byte{1})

	// ticketSupported is a TLS 1.2 extension (as TLS 1.3 replaced tickets with PSK
	// identities) and ECH requires and forces TLS 1.3.
	hello.ticketSupported = true && !echInner

	if hello.supportedVersions[0] == VersionTLS13 {
		// Require DHE on resumption as it guarantees forward secrecy against
		// compromise of the session ticket key. See RFC 8446, Section 4.2.9.
		hello.pskModes = []uint8{pskModeDHE}
	}

	// Session resumption is not allowed if renegotiating because
	// renegotiation is primarily used to allow a client to send a client
	// certificate, which would be skipped if session resumption occurred.
	if c.handshakes != 0 {
		return nil, nil, nil, nil
	}

	// Try to resume a previously negotiated TLS session, if available.
	cacheKey := c.clientSessionCacheKey()
	if cacheKey == "" {
		return nil, nil, nil, nil
	}
	cs, ok := c.config.ClientSessionCache.Get(cacheKey)
	if !ok || cs == nil {
		return nil, nil, nil, nil
	}
	session = cs.session

	// Check that version used for the previous session is still valid.
	versOk := false
	for _, v := range hello.supportedVersions {
		if v == session.version {
			versOk = true
			break
		}
	}
	if !versOk {
		return nil, nil, nil, nil
	}

	if c.config.time().After(session.peerCertificates[0].NotAfter) {
		// Expired certificate, delete the entry.
		c.config.ClientSessionCache.Put(cacheKey, nil)
		return nil, nil, nil, nil
	}
	if !c.config.InsecureSkipVerify {
		if len(session.verifiedChains) == 0 {
			// The original connection had InsecureSkipVerify, while this doesn't.
			return nil, nil, nil, nil
		}
		if err := session.peerCertificates[0].VerifyHostname(c.config.ServerName); err != nil {
			// This should be ensured by the cache key, but protect the
			// application from a faulty ClientSessionCache implementation.
			return nil, nil, nil, nil
		}
		opts := x509.VerifyOptions{
			CurrentTime: c.config.time(),
			Roots:       c.config.RootCAs,
			KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		}
		if !anyValidVerifiedChain(session.verifiedChains, opts) {
			// No valid chains, delete the entry.
			c.config.ClientSessionCache.Put(cacheKey, nil)
			return nil, nil, nil, nil
		}
	}

	if session.version != VersionTLS13 {
		// In TLS 1.2 the cipher suite must match the resumed session. Ensure we
		// are still offering it.
		if mutualCipherSuite(hello.cipherSuites, session.cipherSuite) == nil {
			return nil, nil, nil, nil
		}

		// FIPS 140-3 requires the use of Extended Master Secret.
		if !session.extMasterSecret && fips140tls.Required() {
			return nil, nil, nil, nil
		}

		hello.sessionTicket = session.ticket
		return
	}

	// Check that the session ticket is not expired.
	if c.config.time().After(time.Unix(int64(session.useBy), 0)) {
		c.config.ClientSessionCache.Put(cacheKey, nil)
		return nil, nil, nil, nil
	}

	// In TLS 1.3 the KDF hash must match the resumed session. Ensure we
	// offer at least one cipher suite with that hash.
	cipherSuite := cipherSuiteTLS13ByID(session.cipherSuite)
	if cipherSuite == nil {
		return nil, nil, nil, nil
	}
	cipherSuiteOk := false
	for _, offeredID := range hello.cipherSuites {
		offeredSuite := cipherSuiteTLS13ByID(offeredID)
		if offeredSuite != nil && offeredSuite.hash == cipherSuite.hash {
			cipherSuiteOk = true
			break
		}
	}
	if !cipherSuiteOk {
		return nil, nil, nil, nil
	}

	if c.quic != nil {
		if c.quic.enableSessionEvents {
			c.quicResumeSession(session)
		}

		// For 0-RTT, the cipher suite has to match exactly, and we need to be
		// offering the same ALPN.
		if session.EarlyData && mutualCipherSuiteTLS13(hello.cipherSuites, session.cipherSuite) != nil {
			for _, alpn := range hello.alpnProtocols {
				if alpn == session.alpnProtocol {
					hello.earlyData = true
					break
				}
			}
		}
	}

	// Set the pre_shared_key extension. See RFC 8446, Section 4.2.11.1.
	ticketAge := c.config.time().Sub(time.Unix(int64(session.createdAt), 0))
	identity := pskIdentity{
		label:               session.ticket,
		obfuscatedTicketAge: uint32(ticketAge/time.Millisecond) + session.ageAdd,
	}
	hello.pskIdentities = []pskIdentity{identity}
	hello.pskBinders = [][]byte{make([]byte, cipherSuite.hash.Size())}

	// Compute the PSK binders. See RFC 8446, Section 4.2.11.2.
	earlySecret = tls13.NewEarlySecret(cipherSuite.hash.New, session.secret)
	binderKey = earlySecret.ResumptionBinderKey()
	transcript := cipherSuite.hash.New()
	if err := computeAndUpdatePSK(hello, binderKey, transcript, cipherSuite.finishedHash); err != nil {
		return nil, nil, nil, err
	}

	return
}

func (c *Conn) pickTLSVersion(serverHello *serverHelloMsg) error {
	peerVersion := serverHello.vers
	if serverHello.supportedVersion != 0 {
		peerVersion = serverHello.supportedVersion
	}

	vers, ok := c.config.mutualVersion(roleClient, c.quic != nil, []uint16{peerVersion})
	if !ok {
		c.sendAlert(alertProtocolVersion)
		return fmt.Errorf("tls: server selected unsupported protocol version %x", peerVersion)
	}

	c.vers = vers
	c.haveVers = true
	c.in.version = vers
	c.out.version = vers

	return nil
}

// Does the handshake, either a full one or resumes old session. Requires hs.c,
// hs.hello, hs.serverHello, and, optionally, hs.session to be set.
func (hs *clientHandshakeState) handshake() error {
	c := hs.c

	// If we did not load a session (hs.session == nil), but we did set a
	// session ID in the transmitted client hello (hs.hello.sessionId != nil),
	// it means we tried to negotiate TLS 1.3 and sent a random session ID as a
	// compatibility measure (see RFC 8446, Section 4.1.2).
	//
	// Since we're now handshaking for TLS 1.2, if the server echoed the
	// transmitted ID back to us, we know mischief is afoot: the session ID
	// was random and can't possibly be recognized by the server.
	if hs.session == nil && hs.hello.sessionId != nil && bytes.Equal(hs.hello.sessionId, hs.serverHello.sessionId) {
		c.sendAlert(alertIllegalParameter)
		return errors.New("tls: server echoed TLS 1.3 compatibility session ID in TLS 1.2")
	}

	isResume, err := hs.processServerHello()
	if err != nil {
		return err
	}

	hs.finishedHash = newFinishedHash(c.vers, hs.suite)

	// No signatures of the handshake are needed in a resumption.
	// Otherwise, in a full handshake, if we don't have any certificates
	// configured then we will never send a CertificateVerify message and
	// thus no signatures are needed in that case either.
	if isResume || (len(c.config.Certificates) == 0 && c.config.GetClientCertificate == nil) {
		hs.finishedHash.discardHandshakeBuffer()
	}

	if err := transcriptMsg(hs.hello, &hs.finishedHash); err != nil {
		return err
	}
	if err := transcriptMsg(hs.serverHello, &hs.finishedHash); err != nil {
		return err
	}

	c.buffering = true
	c.didResume = isResume
	if isResume {
		if err := hs.establishKeys(); err != nil {
			return err
		}
		if err := hs.readSessionTicket(); err != nil {
			return err
		}
		if err := hs.readFinished(c.serverFinished[:]); err != nil {
			return err
		}
		c.clientFinishedIsFirst = false
		// Make sure the connection is still being verified whether or not this
		// is a resumption. Resumptions currently don't reverify certificates so
		// they don't call verifyServerCertificate. See Issue 31641.
		if c.config.VerifyConnection != nil {
			if err := c.config.VerifyConnection(c.connectionStateLocked()); err != nil {
				c.sendAlert(alertBadCertificate)
				return err
			}
		}
		if err := hs.sendFinished(c.clientFinished[:]); err != nil {
			return err
		}
		if _, err := c.flush(); err != nil {
			return err
		}
	} else {
		if err := hs.doFullHandshake(); err != nil {
			return err
		}
		if err := hs.establishKeys(); err != nil {
			return err
		}
		if err := hs.sendFinished(c.clientFinished[:]); err != nil {
			return err
		}
		if _, err := c.flush(); err != nil {
			return err
		}
		c.clientFinishedIsFirst = true
		if err := hs.readSessionTicket(); err != nil {
			return err
		}
		if err := hs.readFinished(c.serverFinished[:]); err != nil {
			return err
		}
	}
	if err := hs.saveSessionTicket(); err != nil {
		return err
	}

	c.ekm = ekmFromMasterSecret(c.vers, hs.suite, hs.masterSecret, hs.hello.random, hs.serverHello.random)
	c.isHandshakeComplete.Store(true)

	return nil
}

func (hs *clientHandshakeState) pickCipherSuite() error {
	if hs.suite = mutualCipherSuite(hs.hello.cipherSuites, hs.serverHello.cipherSuite); hs.suite == nil {
		hs.c.sendAlert(alertHandshakeFailure)
		return errors.New("tls: server chose an unconfigured cipher suite")
	}

	hs.c.cipherSuite = hs.suite.id
	return nil
}

func (hs *clientHandshakeState) doFullHandshake() error {
	c := hs.c

	msg, err := c.readHandshake(&hs.finishedHash)
	if err != nil {
		return err
	}
	certMsg, ok := msg.(*certificateMsg)
	if !ok || len(certMsg.certificates) == 0 {
		c.sendAlert(alertUnexpectedMessage)
		return unexpectedMessageError(certMsg, msg)
	}

	msg, err = c.readHandshake(&hs.finishedHash)
	if err != nil {
		return err
	}

	cs, ok := msg.(*certificateStatusMsg)
	if ok {
		// RFC4366 on Certificate Status Request:
		// The server MAY return a "certificate_status" message.

		if !hs.serverHello.ocspStapling {
			// If a server returns a "CertificateStatus" message, then the
			// server MUST have included an extension of type "status_request"
			// with empty "extension_data" in the extended server hello.

			c.sendAlert(alertUnexpectedMessage)
			return errors.New("tls: received unexpected CertificateStatus message")
		}

		c.ocspResponse = cs.response

		msg, err = c.readHandshake(&hs.finishedHash)
		if err != nil {
			return err
		}
	}

	if c.handshakes == 0 {
		// If this is the first handshake on a connection, process and
		// (optionally) verify the server's certificates.
		if err := c.verifyServerCertificate(certMsg.certificates); err != nil {
			return err
		}
	} else {
		// This is a renegotiation handshake. We require that the
		// server's identity (i.e. leaf certificate) is unchanged and
		// thus any previous trust decision is still valid.
		//
		// See https://mitls.org/pages/attacks/3SHAKE for the
		// motivation behind this requirement.
		if !bytes.Equal(c.peerCertificates[0].Raw, certMsg.certificates[0]) {
			c.sendAlert(alertBadCertificate)
			return errors.New("tls: server's identity changed during renegotiation")
		}
	}

	keyAgreement := hs.suite.ka(c.vers)

	skx, ok := msg.(*serverKeyExchangeMsg)
	if ok {
		err = keyAgreement.processServerKeyExchange(c.config, hs.hello, hs.serverHello, c.peerCertificates[0], skx)
		if err != nil {
			c.sendAlert(alertIllegalParameter)
			return err
		}
		if keyAgreement, ok := keyAgreement.(*ecdheKeyAgreement); ok {
			c.curveID = keyAgreement.curveID
			c.peerSigAlg = keyAgreement.signatureAlgorithm
		}

		msg, err = c.readHandshake(&hs.finishedHash)
		if err != nil {
			return err
		}
	}

	var chainToSend *Certificate
	var certRequested bool
	certReq, ok := msg.(*certificateRequestMsg)
	if ok {
		certRequested = true

		cri := certificateRequestInfoFromMsg(hs.ctx, c.vers, certReq)
		if chainToSend, err = c.getClientCertificate(cri); err != nil {
			c.sendAlert(alertInternalError)
			return err
		}

		msg, err = c.readHandshake(&hs.finishedHash)
		if err != nil {
			return err
		}
	}

	if chainToSend != nil {
		hs.c.localCertificate = chainToSend.Certificate
	}

	shd, ok := msg.(*serverHelloDoneMsg)
	if !ok {
		c.sendAlert(alertUnexpectedMessage)
		return unexpectedMessageError(shd, msg)
	}

	// If the server requested a certificate then we have to send a
	// Certificate message, even if it's empty because we don't have a
	// certificate to send.
	if certRequested {
		certMsg = new(certificateMsg)
		certMsg.certificates = chainToSend.Certificate
		if _, err := hs.c.writeHandshakeRecord(certMsg, &hs.finishedHash); err != nil {
			return err
		}
	}

	preMasterSecret, ckx, err := keyAgreement.generateClientKeyExchange(c.config, hs.hello, c.peerCertificates[0])
	if err != nil {
		c.sendAlert(alertInternalError)
		return err
	}
	if ckx != nil {
		if _, err := hs.c.writeHandshakeRecord(ckx, &hs.finishedHash); err != nil {
			return err
		}
	}

	if hs.serverHello.extendedMasterSecret {
		c.extMasterSecret = true
		hs.masterSecret = extMasterFromPreMasterSecret(c.vers, hs.suite, preMasterSecret,
			hs.finishedHash.Sum())
	} else {
		if fips140tls.Required() {
			c.sendAlert(alertHandshakeFailure)
			return errors.New("tls: FIPS 140-3 requires the use of Extended Master Secret")
		}
		hs.masterSecret = masterFromPreMasterSecret(c.vers, hs.suite, preMasterSecret,
			hs.hello.random, hs.serverHello.random)
	}
	if err := c.config.writeKeyLog(keyLogLabelTLS12, hs.hello.random, hs.masterSecret); err != nil {
		c.sendAlert(alertInternalError)
		return errors.New("tls: failed to write to key log: " + err.Error())
	}

	if chainToSend != nil && len(chainToSend.Certificate) > 0 {
		certVerify := &certificateVerifyMsg{}

		key, ok := chainToSend.PrivateKey.(crypto.Signer)
		if !ok {
			c.sendAlert(alertInternalError)
			return fmt.Errorf("tls: client certificate private key of type %T does not implement crypto.Signer", chainToSend.PrivateKey)
		}

		if c.vers >= VersionTLS12 {
			signatureAlgorithm, err := selectSignatureScheme(c.vers, chainToSend, certReq.supportedSignatureAlgorithms)
			if err != nil {
				c.sendAlert(alertHandshakeFailure)
				return err
			}
			sigType, sigHash, err := typeAndHashFromSignatureScheme(signatureAlgorithm)
			if err != nil {
				return c.sendAlert(alertInternalError)
			}
			certVerify.hasSignatureAlgorithm = true
			certVerify.signatureAlgorithm = signatureAlgorithm
			if sigHash == crypto.SHA1 {
				tlssha1.Value() // ensure godebug is initialized
				tlssha1.IncNonDefault()
			}
			if hs.finishedHash.buffer == nil {
				c.sendAlert(alertInternalError)
				return errors.New("tls: internal error: did not keep handshake transcript for TLS 1.2")
			}
			signOpts := crypto.SignerOpts(sigHash)
			if sigType == signatureRSAPSS {
				signOpts = &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: sigHash}
			}
			certVerify.signature, err = crypto.SignMessage(key, c.config.rand(), hs.finishedHash.buffer, signOpts)
			if err != nil {
				c.sendAlert(alertInternalError)
				return err
			}
		} else {
			sigType, sigHash, err := legacyTypeAndHashFromPublicKey(key.Public())
			if err != nil {
				c.sendAlert(alertIllegalParameter)
				return err
			}
			signed := hs.finishedHash.hashForClientCertificate(sigType)
			certVerify.signature, err = key.Sign(c.config.rand(), signed, sigHash)
			if err != nil {
				c.sendAlert(alertInternalError)
				return err
			}
		}

		if _, err := hs.c.writeHandshakeRecord(certVerify, &hs.finishedHash); err != nil {
			return err
		}
	}

	hs.finishedHash.discardHandshakeBuffer()

	return nil
}

func (hs *clientHandshakeState) establishKeys() error {
	c := hs.c

	clientMAC, serverMAC, clientKey, serverKey, clientIV, serverIV :=
		keysFromMasterSecret(c.vers, hs.suite, hs.masterSecret, hs.hello.random, hs.serverHello.random, hs.suite.macLen, hs.suite.keyLen, hs.suite.ivLen)
	var clientCipher, serverCipher any
	var clientHash, serverHash hash.Hash
	if hs.suite.cipher != nil {
		clientCipher = hs.suite.cipher(clientKey, clientIV, false /* not for reading */)
		clientHash = hs.suite.mac(clientMAC)
		serverCipher = hs.suite.cipher(serverKey, serverIV, true /* for reading */)
		serverHash = hs.suite.mac(serverMAC)
	} else {
		clientCipher = hs.suite.aead(clientKey, clientIV)
		serverCipher = hs.suite.aead(serverKey, serverIV)
	}

	c.in.prepareCipherSpec(c.vers, serverCipher, serverHash)
	c.out.prepareCipherSpec(c.vers, clientCipher, clientHash)
	return nil
}

func (hs *clientHandshakeState) serverResumedSession() bool {
	// If the server responded with the same sessionId then it means the
	// sessionTicket is being used to resume a TLS session.
	return hs.session != nil && hs.hello.sessionId != nil &&
		bytes.Equal(hs.serverHello.sessionId, hs.hello.sessionId)
}

func (hs *clientHandshakeState) processServerHello() (bool, error) {
	c := hs.c

	if err := hs.pickCipherSuite(); err != nil {
		return false, err
	}

	if hs.serverHello.compressionMethod != compressionNone {
		c.sendAlert(alertIllegalParameter)
		return false, errors.New("tls: server selected unsupported compression format")
	}

	supportsPointFormat := false
	offeredNonCompressedFormat := false
	for _, format := range hs.serverHello.supportedPoints {
		if format == pointFormatUncompressed {
			supportsPointFormat = true
		} else {
			offeredNonCompressedFormat = true
		}
	}
	if !supportsPointFormat && offeredNonCompressedFormat {
		return false, errors.New("tls: server offered only incompatible point formats")
	}

	if c.handshakes == 0 && hs.serverHello.secureRenegotiationSupported {
		c.secureRenegotiation = true
		if len(hs.serverHello.secureRenegotiation) != 0 {
			c.sendAlert(alertHandshakeFailure)
			return false, errors.New("tls: initial handshake had non-empty renegotiation extension")
		}
	}

	if c.handshakes > 0 && c.secureRenegotiation {
		var expectedSecureRenegotiation [24]byte
		copy(expectedSecureRenegotiation[:], c.clientFinished[:])
		copy(expectedSecureRenegotiation[12:], c.serverFinished[:])
		if !bytes.Equal(hs.serverHello.secureRenegotiation, expectedSecureRenegotiation[:]) {
			c.sendAlert(alertHandshakeFailure)
			return false, errors.New("tls: incorrect renegotiation extension contents")
		}
	}

	if err := checkALPN(hs.hello.alpnProtocols, hs.serverHello.alpnProtocol, false); err != nil {
		c.sendAlert(alertUnsupportedExtension)
		return false, err
	}
	c.clientProtocol = hs.serverHello.alpnProtocol

	c.scts = hs.serverHello.scts

	if !hs.serverResumedSession() {
		return false, nil
	}

	if hs.session.version != c.vers {
		c.sendAlert(alertHandshakeFailure)
		return false, errors.New("tls: server resumed a session with a different version")
	}

	if hs.session.cipherSuite != hs.suite.id {
		c.sendAlert(alertHandshakeFailure)
		return false, errors.New("tls: server resumed a session with a different cipher suite")
	}

	// RFC 7627, Section 5.3
	if hs.session.extMasterSecret != hs.serverHello.extendedMasterSecret {
		c.sendAlert(alertHandshakeFailure)
		return false, errors.New("tls: server resumed a session with a different EMS extension")
	}

	// Restore master secret and certificates from previous state
	hs.masterSecret = hs.session.secret
	c.extMasterSecret = hs.session.extMasterSecret
	c.peerCertificates = hs.session.peerCertificates
	c.verifiedChains = hs.session.verifiedChains
	c.ocspResponse = hs.session.ocspResponse
	// Let the ServerHello SCTs override the session SCTs from the original
	// connection, if any are provided.
	if len(c.scts) == 0 && len(hs.session.scts) != 0 {
		c.scts = hs.session.scts
	}
	c.curveID = hs.session.curveID

	return true, nil
}

// checkALPN ensure that the server's choice of ALPN protocol is compatible with
// the protocols that we advertised in the ClientHello.
func checkALPN(clientProtos []string, serverProto string, quic bool) error {
	if serverProto == "" {
		if quic && len(clientProtos) > 0 {
			// RFC 9001, Section 8.1
			return errors.New("tls: server did not select an ALPN protocol")
		}
		return nil
	}
	if len(clientProtos) == 0 {
		return errors.New("tls: server advertised unrequested ALPN extension")
	}
	for _, proto := range clientProtos {
		if proto == serverProto {
			return nil
		}
	}
	return errors.New("tls: server selected unadvertised ALPN protocol")
}

func (hs *clientHandshakeState) readFinished(out []byte) error {
	c := hs.c

	if err := c.readChangeCipherSpec(); err != nil {
		return err
	}

	// finishedMsg is included in the transcript, but not until after we
	// check the client version, since the state before this message was
	// sent is used during verification.
	msg, err := c.readHandshake(nil)
	if err != nil {
		return err
	}
	serverFinished, ok := msg.(*finishedMsg)
	if !ok {
		c.sendAlert(alertUnexpectedMessage)
		return unexpectedMessageError(serverFinished, msg)
	}

	verify := hs.finishedHash.serverSum(hs.masterSecret)
	if len(verify) != len(serverFinished.verifyData) ||
		subtle.ConstantTimeCompare(verify, serverFinished.verifyData) != 1 {
		c.sendAlert(alertHandshakeFailure)
		return errors.New("tls: server's Finished message was incorrect")
	}

	if err := transcriptMsg(serverFinished, &hs.finishedHash); err != nil {
		return err
	}

	copy(out, verify)
	return nil
}

func (hs *clientHandshakeState) readSessionTicket() error {
	if !hs.serverHello.ticketSupported {
		return nil
	}
	c := hs.c

	if !hs.hello.ticketSupported {
		c.sendAlert(alertIllegalParameter)
		return errors.New("tls: server sent unrequested session ticket")
	}

	msg, err := c.readHandshake(&hs.finishedHash)
	if err != nil {
		return err
	}
	sessionTicketMsg, ok := msg.(*newSessionTicketMsg)
	if !ok {
		c.sendAlert(alertUnexpectedMessage)
		return unexpectedMessageError(sessionTicketMsg, msg)
	}

	hs.ticket = sessionTicketMsg.ticket
	return nil
}

func (hs *clientHandshakeState) saveSessionTicket() error {
	if hs.ticket == nil {
		return nil
	}
	c := hs.c

	cacheKey := c.clientSessionCacheKey()
	if cacheKey == "" {
		return nil
	}

	session := c.sessionState()
	session.secret = hs.masterSecret
	session.ticket = hs.ticket

	cs := &ClientSessionState{session: session}
	c.config.ClientSessionCache.Put(cacheKey, cs)
	return nil
}

func (hs *clientHandshakeState) sendFinished(out []byte) error {
	c := hs.c

	if err := c.writeChangeCipherRecord(); err != nil {
		return err
	}

	finished := new(finishedMsg)
	finished.verifyData = hs.finishedHash.clientSum(hs.masterSecret)
	if _, err := hs.c.writeHandshakeRecord(finished, &hs.finishedHash); err != nil {
		return err
	}
	copy(out, finished.verifyData)
	return nil
}

// defaultMaxRSAKeySize is the maximum RSA key size in bits that we are willing
// to verify the signatures of during a TLS handshake.
const defaultMaxRSAKeySize = 8192

var tlsmaxrsasize = godebug.New("tlsmaxrsasize")

func checkKeySize(n int) (max int, ok bool) {
	if v := tlsmaxrsasize.Value(); v != "" {
		if max, err := strconv.Atoi(v); err == nil {
			if (n <= max) != (n <= defaultMaxRSAKeySize) {
				tlsmaxrsasize.IncNonDefault()
			}
			return max, n <= max
		}
	}
	return defaultMaxRSAKeySize, n <= defaultMaxRSAKeySize
}

// verifyServerCertificate parses and verifies the provided chain, setting
// c.verifiedChains and c.peerCertificates or sending the appropriate alert.
func (c *Conn) verifyServerCertificate(certificates [][]byte) error {
	certs := make([]*x509.Certificate, len(certificates))
	for i, asn1Data := range certificates {
		cert, err := globalCertCache.newCert(asn1Data)
		if err != nil {
			c.sendAlert(alertDecodeError)
			return errors.New("tls: failed to parse certificate from server: " + err.Error())
		}
		if cert.PublicKeyAlgorithm == x509.RSA {
			n := cert.PublicKey.(*rsa.PublicKey).N.BitLen()
			if max, ok := checkKeySize(n); !ok {
				c.sendAlert(alertBadCertificate)
				return fmt.Errorf("tls: server sent certificate containing RSA key larger than %d bits", max)
			}
		}
		certs[i] = cert
	}

	echRejected := c.config.EncryptedClientHelloConfigList != nil && !c.echAccepted
	if echRejected {
		if c.config.EncryptedClientHelloRejectionVerify != nil {
			if err := c.config.EncryptedClientHelloRejectionVerify(c.connectionStateLocked()); err != nil {
				c.sendAlert(alertBadCertificate)
				return err
			}
		} else {
			opts := x509.VerifyOptions{
				Roots:         c.config.RootCAs,
				CurrentTime:   c.config.time(),
				DNSName:       c.serverName,
				Intermediates: x509.NewCertPool(),
			}

			for _, cert := range certs[1:] {
				opts.Intermediates.AddCert(cert)
			}
			chains, err := certs[0].Verify(opts)
			if err != nil {
				c.sendAlert(alertBadCertificate)
				return &CertificateVerificationError{UnverifiedCertificates: certs, Err: err}
			}

			c.verifiedChains, err = fipsAllowedChains(chains)
			if err != nil {
				c.sendAlert(alertBadCertificate)
				return &CertificateVerificationError{UnverifiedCertificates: certs, Err: err}
			}
		}
	} else if !c.config.InsecureSkipVerify {
		opts := x509.VerifyOptions{
			Roots:         c.config.RootCAs,
			CurrentTime:   c.config.time(),
			DNSName:       c.config.ServerName,
			Intermediates: x509.NewCertPool(),
		}

		for _, cert := range certs[1:] {
			opts.Intermediates.AddCert(cert)
		}
		chains, err := certs[0].Verify(opts)
		if err != nil {
			c.sendAlert(alertBadCertificate)
			return &CertificateVerificationError{UnverifiedCertificates: certs, Err: err}
		}

		c.verifiedChains, err = fipsAllowedChains(chains)
		if err != nil {
			c.sendAlert(alertBadCertificate)
			return &CertificateVerificationError{UnverifiedCertificates: certs, Err: err}
		}
	}

	switch certs[0].PublicKey.(type) {
	case *rsa.PublicKey, *ecdsa.PublicKey, ed25519.PublicKey:
	case *mldsa.PublicKey:
		if c.vers < VersionTLS13 {
			c.sendAlert(alertIllegalParameter)
			return errors.New("tls: server's certificate uses ML-DSA, which requires TLS 1.3")
		}
	default:
		c.sendAlert(alertUnsupportedCertificate)
		return fmt.Errorf("tls: server's certificate contains an unsupported type of public key: %T", certs[0].PublicKey)
	}

	c.peerCertificates = certs

	if c.config.VerifyPeerCertificate != nil && !echRejected {
		if err := c.config.VerifyPeerCertificate(certificates, c.verifiedChains); err != nil {
			c.sendAlert(alertBadCertificate)
			return err
		}
	}

	if c.config.VerifyConnection != nil && !echRejected {
		if err := c.config.VerifyConnection(c.connectionStateLocked()); err != nil {
			c.sendAlert(alertBadCertificate)
			return err
		}
	}

	return nil
}

// certificateRequestInfoFromMsg generates a CertificateRequestInfo from a TLS
// <= 1.2 CertificateRequest, making an effort to fill in missing information.
func certificateRequestInfoFromMsg(ctx context.Context, vers uint16, certReq *certificateRequestMsg) *CertificateRequestInfo {
	cri := &CertificateRequestInfo{
		AcceptableCAs: certReq.certificateAuthorities,
		Version:       vers,
		ctx:           ctx,
	}

	var rsaAvail, ecAvail bool
	for _, certType := range certReq.certificateTypes {
		switch certType {
		case certTypeRSASign:
			rsaAvail = true
		case certTypeECDSASign:
			ecAvail = true
		}
	}

	if !certReq.hasSignatureAlgorithm {
		// Prior to TLS 1.2, signature schemes did not exist. In this case we
		// make up a list based on the acceptable certificate types, to help
		// GetClientCertificate and SupportsCertificate select the right certificate.
		// The hash part of the SignatureScheme is a lie here, because
		// TLS 1.0 and 1.1 always use MD5+SHA1 for RSA and SHA1 for ECDSA.
		switch {
		case rsaAvail && ecAvail:
			cri.SignatureSchemes = []SignatureScheme{
				ECDSAWithP256AndSHA256, ECDSAWithP384AndSHA384, ECDSAWithP521AndSHA512,
				PKCS1WithSHA256, PKCS1WithSHA384, PKCS1WithSHA512, PKCS1WithSHA1,
			}
		case rsaAvail:
			cri.SignatureSchemes = []SignatureScheme{
				PKCS1WithSHA256, PKCS1WithSHA384, PKCS1WithSHA512, PKCS1WithSHA1,
			}
		case ecAvail:
			cri.SignatureSchemes = []SignatureScheme{
				ECDSAWithP256AndSHA256, ECDSAWithP384AndSHA384, ECDSAWithP521AndSHA512,
			}
		}
		return cri
	}

	// Filter the signature schemes based on the certificate types.
	// See RFC 5246, Section 7.4.4 (where it calls this "somewhat complicated").
	cri.SignatureSchemes = make([]SignatureScheme, 0, len(certReq.supportedSignatureAlgorithms))
	for _, sigScheme := range certReq.supportedSignatureAlgorithms {
		sigType, _, err := typeAndHashFromSignatureScheme(sigScheme)
		if err != nil {
			continue
		}
		switch sigType {
		case signatureECDSA, signatureEd25519:
			if ecAvail {
				cri.SignatureSchemes = append(cri.SignatureSchemes, sigScheme)
			}
		case signatureRSAPSS, signaturePKCS1v15:
			if rsaAvail {
				cri.SignatureSchemes = append(cri.SignatureSchemes, sigScheme)
			}
		}
	}

	return cri
}

func (c *Conn) getClientCertificate(cri *CertificateRequestInfo) (*Certificate, error) {
	if c.config.GetClientCertificate != nil {
		return c.config.GetClientCertificate(cri)
	}

	for _, chain := range c.config.Certificates {
		if err := cri.SupportsCertificate(&chain); err != nil {
			continue
		}
		return &chain, nil
	}

	// No acceptable certificate found. Don't send a certificate.
	return new(Certificate), nil
}

// clientSessionCacheKey returns a key used to cache sessionTickets that could
// be used to resume previously negotiated TLS sessions with a server.
func (c *Conn) clientSessionCacheKey() string {
	if len(c.config.ServerName) > 0 {
		return c.config.ServerName
	}
	if c.conn != nil {
		return c.conn.RemoteAddr().String()
	}
	return ""
}

// hostnameInSNI converts name into an appropriate hostname for SNI.
// Literal IP addresses and absolute FQDNs are not permitted as SNI values.
// See RFC 6066, Section 3.
func hostnameInSNI(name string) string {
	host := name
	if len(host) > 0 && host[0] == '[' && host[len(host)-1] == ']' {
		host = host[1 : len(host)-1]
	}
	if i := strings.LastIndex(host, "%"); i > 0 {
		host = host[:i]
	}
	if net.ParseIP(host) != nil {
		return ""
	}
	for len(name) > 0 && name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}
	return name
}

func computeAndUpdatePSK(m *clientHelloMsg, binderKey []byte, transcript hash.Hash, finishedHash func([]byte, hash.Hash) []byte) error {
	helloBytes, err := m.marshalWithoutBinders()
	if err != nil {
		return err
	}
	transcript.Write(helloBytes)
	pskBinders := [][]byte{finishedHash(binderKey, transcript)}
	return m.updateBinders(pskBinders)
}

```

// === FILE: references/go/src/crypto/tls/handshake_client_tls13.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tls

import (
	"bytes"
	"context"
	"crypto"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/internal/fips140/tls13"
	"crypto/rsa"
	"crypto/subtle"
	"errors"
	"hash"
	"slices"
	"time"
)

type clientHandshakeStateTLS13 struct {
	c            *Conn
	ctx          context.Context
	serverHello  *serverHelloMsg
	hello        *clientHelloMsg
	keyShareKeys *keySharePrivateKeys

	session     *SessionState
	earlySecret *tls13.EarlySecret
	binderKey   []byte

	certReq       *certificateRequestMsgTLS13
	usingPSK      bool
	sentDummyCCS  bool
	suite         *cipherSuiteTLS13
	transcript    hash.Hash
	masterSecret  *tls13.MasterSecret
	trafficSecret []byte // client_application_traffic_secret_0

	echContext *echClientContext
}

// handshake requires hs.c, hs.hello, hs.serverHello, hs.keyShareKeys, and,
// optionally, hs.session, hs.earlySecret and hs.binderKey to be set.
func (hs *clientHandshakeStateTLS13) handshake() error {
	c := hs.c

	// The server must not select TLS 1.3 in a renegotiation. See RFC 8446,
	// sections 4.1.2 and 4.1.3.
	if c.handshakes > 0 {
		c.sendAlert(alertProtocolVersion)
		return errors.New("tls: server selected TLS 1.3 in a renegotiation")
	}

	// Consistency check on the presence of a keyShare and its parameters.
	if hs.keyShareKeys == nil || (hs.keyShareKeys.ecdhe == nil && hs.keyShareKeys.mlkem == nil) ||
		len(hs.hello.keyShares) == 0 {
		return c.sendAlert(alertInternalError)
	}

	if err := hs.checkServerHelloOrHRR(); err != nil {
		return err
	}

	hs.transcript = hs.suite.hash.New()

	if err := transcriptMsg(hs.hello, hs.transcript); err != nil {
		return err
	}

	if hs.echContext != nil {
		hs.echContext.innerTranscript = hs.suite.hash.New()
		if err := transcriptMsg(hs.echContext.innerHello, hs.echContext.innerTranscript); err != nil {
			return err
		}
	}

	if bytes.Equal(hs.serverHello.random, helloRetryRequestRandom) {
		if err := hs.sendDummyChangeCipherSpec(); err != nil {
			return err
		}
		if err := hs.processHelloRetryRequest(); err != nil {
			return err
		}
	}

	if hs.echContext != nil {
		confTranscript := cloneHash(hs.echContext.innerTranscript, hs.suite.hash)
		confTranscript.Write(hs.serverHello.original[:30])
		confTranscript.Write(make([]byte, 8))
		confTranscript.Write(hs.serverHello.original[38:])
		h := hs.suite.hash.New
		prk, err := hkdf.Extract(h, hs.echContext.innerHello.random, nil)
		if err != nil {
			c.sendAlert(alertInternalError)
			return err
		}
		acceptConfirmation := tls13.ExpandLabel(h, prk, "ech accept confirmation", confTranscript.Sum(nil), 8)
		if subtle.ConstantTimeCompare(acceptConfirmation, hs.serverHello.random[len(hs.serverHello.random)-8:]) == 1 {
			hs.hello = hs.echContext.innerHello
			c.serverName = c.config.ServerName
			hs.transcript = hs.echContext.innerTranscript
			c.echAccepted = true

			if hs.serverHello.encryptedClientHello != nil {
				c.sendAlert(alertUnsupportedExtension)
				return errors.New("tls: unexpected encrypted client hello extension in server hello despite ECH being accepted")
			}

			if hs.hello.serverName == "" && hs.serverHello.serverNameAck {
				c.sendAlert(alertUnsupportedExtension)
				return errors.New("tls: unexpected server_name extension in server hello")
			}
		} else {
			hs.echContext.echRejected = true
		}
	}

	if err := transcriptMsg(hs.serverHello, hs.transcript); err != nil {
		return err
	}

	c.buffering = true
	if err := hs.processServerHello(); err != nil {
		return err
	}
	if err := hs.sendDummyChangeCipherSpec(); err != nil {
		return err
	}
	if err := hs.establishHandshakeKeys(); err != nil {
		return err
	}
	if err := hs.readServerParameters(); err != nil {
		return err
	}
	if err := hs.readServerCertificate(); err != nil {
		return err
	}
	if err := hs.readServerFinished(); err != nil {
		return err
	}
	if err := hs.sendClientCertificate(); err != nil {
		return err
	}
	if err := hs.sendClientFinished(); err != nil {
		return err
	}
	if _, err := c.flush(); err != nil {
		return err
	}

	if hs.echContext != nil && hs.echContext.echRejected {
		c.sendAlert(alertECHRequired)
		return &ECHRejectionError{hs.echContext.retryConfigs}
	}

	c.isHandshakeComplete.Store(true)

	return nil
}

// checkServerHelloOrHRR does validity checks that apply to both ServerHello and
// HelloRetryRequest messages. It sets hs.suite.
func (hs *clientHandshakeStateTLS13) checkServerHelloOrHRR() error {
	c := hs.c

	if hs.serverHello.supportedVersion == 0 {
		c.sendAlert(alertMissingExtension)
		return errors.New("tls: server selected TLS 1.3 using the legacy version field")
	}

	if hs.serverHello.supportedVersion != VersionTLS13 {
		c.sendAlert(alertIllegalParameter)
		return errors.New("tls: server selected an invalid version after a HelloRetryRequest")
	}

	if hs.serverHello.vers != VersionTLS12 {
		c.sendAlert(alertIllegalParameter)
		return errors.New("tls: server sent an incorrect legacy version")
	}

	if hs.serverHello.ocspStapling ||
		hs.serverHello.ticketSupported ||
		hs.serverHello.extendedMasterSecret ||
		hs.serverHello.secureRenegotiationSupported ||
		len(hs.serverHello.secureRenegotiation) != 0 ||
		len(hs.serverHello.alpnProtocol) != 0 ||
		len(hs.serverHello.scts) != 0 {
		c.sendAlert(alertUnsupportedExtension)
		return errors.New("tls: server sent a ServerHello extension forbidden in TLS 1.3")
	}

	if !bytes.Equal(hs.hello.sessionId, hs.serverHello.sessionId) {
		c.sendAlert(alertIllegalParameter)
		return errors.New("tls: server did not echo the legacy session ID")
	}

	if hs.serverHello.compressionMethod != compressionNone {
		c.sendAlert(alertDecodeError)
		return errors.New("tls: server sent non-zero legacy TLS compression method")
	}

	selectedSuite := mutualCipherSuiteTLS13(hs.hello.cipherSuites, hs.serverHello.cipherSuite)
	if hs.suite != nil && selectedSuite != hs.suite {
		c.sendAlert(alertIllegalParameter)
		return errors.New("tls: server changed cipher suite after a HelloRetryRequest")
	}
	if selectedSuite == nil {
		c.sendAlert(alertIllegalParameter)
		return errors.New("tls: server chose an unconfigured cipher suite")
	}
	hs.suite = selectedSuite
	c.cipherSuite = hs.suite.id

	return nil
}

// sendDummyChangeCipherSpec sends a ChangeCipherSpec record for compatibility
// with middleboxes that didn't implement TLS correctly. See RFC 8446, Appendix D.4.
func (hs *clientHandshakeStateTLS13) sendDummyChangeCipherSpec() error {
	if hs.c.quic != nil {
		return nil
	}
	if hs.sentDummyCCS {
		return nil
	}
	hs.sentDummyCCS = true

	return hs.c.writeChangeCipherRecord()
}

// processHelloRetryRequest handles the HRR in hs.serverHello, modifies and
// resends hs.hello, and reads the new ServerHello into hs.serverHello.
func (hs *clientHandshakeStateTLS13) processHelloRetryRequest() error {
	c := hs.c

	// The first ClientHello gets double-hashed into the transcript upon a
	// HelloRetryRequest. (The idea is that the server might offload transcript
	// storage to the client in the cookie.) See RFC 8446, Section 4.4.1.
	chHash := hs.transcript.Sum(nil)
	hs.transcript.Reset()
	hs.transcript.Write([]byte{typeMessageHash, 0, 0, uint8(len(chHash))})
	hs.transcript.Write(chHash)
	if err := transcriptMsg(hs.serverHello, hs.transcript); err != nil {
		return err
	}

	var isInnerHello bool
	hello := hs.hello
	if hs.echContext != nil {
		chHash = hs.echContext.innerTranscript.Sum(nil)
		hs.echContext.innerTranscript.Reset()
		hs.echContext.innerTranscript.Write([]byte{typeMessageHash, 0, 0, uint8(len(chHash))})
		hs.echContext.innerTranscript.Write(chHash)

		if hs.serverHello.encryptedClientHello != nil {
			if len(hs.serverHello.encryptedClientHello) != 8 {
				hs.c.sendAlert(alertDecodeError)
				return errors.New("tls: malformed encrypted client hello extension")
			}

			confTranscript := cloneHash(hs.echContext.innerTranscript, hs.suite.hash)
			hrrHello := make([]byte, len(hs.serverHello.original))
			copy(hrrHello, hs.serverHello.original)
			hrrHello = bytes.Replace(hrrHello, hs.serverHello.encryptedClientHello, make([]byte, 8), 1)
			confTranscript.Write(hrrHello)
			h := hs.suite.hash.New
			prk, err := hkdf.Extract(h, hs.echContext.innerHello.random, nil)
			if err != nil {
				c.sendAlert(alertInternalError)
				return err
			}
			acceptConfirmation := tls13.ExpandLabel(h, prk, "hrr ech accept confirmation", confTranscript.Sum(nil), 8)
			if subtle.ConstantTimeCompare(acceptConfirmation, hs.serverHello.encryptedClientHello) == 1 {
				hello = hs.echContext.innerHello
				c.serverName = c.config.ServerName
				isInnerHello = true
				c.echAccepted = true
			}
		}

		if err := transcriptMsg(hs.serverHello, hs.echContext.innerTranscript); err != nil {
			return err
		}
	} else if hs.serverHello.encryptedClientHello != nil {
		// Unsolicited ECH extension should be rejected
		c.sendAlert(alertUnsupportedExtension)
		return errors.New("tls: unexpected encrypted client hello extension in serverHello")
	}

	// The only HelloRetryRequest extensions we support are key_share and
	// cookie, and clients must abort the handshake if the HRR would not result
	// in any change in the ClientHello.
	if hs.serverHello.selectedGroup == 0 && hs.serverHello.cookie == nil {
		c.sendAlert(alertIllegalParameter)
		return errors.New("tls: server sent an unnecessary HelloRetryRequest message")
	}

	if hs.serverHello.cookie != nil {
		hello.cookie = hs.serverHello.cookie
	}

	if hs.serverHello.serverShare.group != 0 {
		c.sendAlert(alertDecodeError)
		return errors.New("tls: received malformed key_share extension")
	}

	// If the server sent a key_share extension selecting a group, ensure it's
	// a group we advertised but did not send a key share for, and send a key
	// share for it this time.
	if curveID := hs.serverHello.selectedGroup; curveID != 0 {
		if !slices.Contains(hello.supportedCurves, curveID) {
			c.sendAlert(alertIllegalParameter)
			return errors.New("tls: server selected unsupported group")
		}
		if slices.ContainsFunc(hs.hello.keyShares, func(ks keyShare) bool {
			return ks.group == curveID
		}) {
			c.sendAlert(alertIllegalParameter)
			return errors.New("tls: server sent an unnecessary HelloRetryRequest key_share")
		}
		ke, err := keyExchangeForCurveID(curveID)
		if err != nil {
			c.sendAlert(alertInternalError)
			return errors.New("tls: internal error: supportsCurve accepted unimplemented curve")
		}
		hs.keyShareKeys, hello.keyShares, err = ke.keyShares(c.config.rand())
		if err != nil {
			c.sendAlert(alertInternalError)
			return err
		}
		// Do not send the fallback ECDH key share in a HRR response.
		hello.keyShares = hello.keyShares[:1]
	}

	if len(hello.pskIdentities) > 0 {
		pskSuite := cipherSuiteTLS13ByID(hs.session.cipherSuite)
		if pskSuite == nil {
			return c.sendAlert(alertInternalError)
		}
		if pskSuite.hash == hs.suite.hash {
			// Update binders and obfuscated_ticket_age.
			ticketAge := c.config.time().Sub(time.Unix(int64(hs.session.createdAt), 0))
			hello.pskIdentities[0].obfuscatedTicketAge = uint32(ticketAge/time.Millisecond) + hs.session.ageAdd

			transcript := hs.suite.hash.New()
			transcript.Write([]byte{typeMessageHash, 0, 0, uint8(len(chHash))})
			transcript.Write(chHash)
			if err := transcriptMsg(hs.serverHello, transcript); err != nil {
				return err
			}

			if err := computeAndUpdatePSK(hello, hs.binderKey, transcript, hs.suite.finishedHash); err != nil {
				return err
			}
		} else {
			// Server selected a cipher suite incompatible with the PSK.
			hello.pskIdentities = nil
			hello.pskBinders = nil
		}
	}

	if hello.earlyData {
		hello.earlyData = false
		c.quicRejectedEarlyData()
	}

	if isInnerHello {
		// Any extensions which have changed in hello, but are mirrored in the
		// outer hello and compressed, need to be copied to the outer hello, so
		// they can be properly decompressed by the server. For now, the only
		// extension which may have changed is keyShares.
		hs.hello.keyShares = hello.keyShares
		hs.echContext.innerHello = hello
		if err := transcriptMsg(hs.echContext.innerHello, hs.echContext.innerTranscript); err != nil {
			return err
		}

		if err := computeAndUpdateOuterECHExtension(hs.hello, hs.echContext.innerHello, hs.echContext, false); err != nil {
			return err
		}
	} else {
		hs.hello = hello
	}

	if _, err := hs.c.writeHandshakeRecord(hs.hello, hs.transcript); err != nil {
		return err
	}

	// serverHelloMsg is not included in the transcript
	msg, err := c.readHandshake(nil)
	if err != nil {
		return err
	}

	serverHello, ok := msg.(*serverHelloMsg)
	if !ok {
		c.sendAlert(alertUnexpectedMessage)
		return unexpectedMessageError(serverHello, msg)
	}
	hs.serverHello = serverHello

	if err := hs.checkServerHelloOrHRR(); err != nil {
		return err
	}

	c.didHRR = true
	return nil
}

func (hs *clientHandshakeStateTLS13) processServerHello() error {
	c := hs.c

	if bytes.Equal(hs.serverHello.random, helloRetryRequestRandom) {
		c.sendAlert(alertUnexpectedMessage)
		return errors.New("tls: server sent two HelloRetryRequest messages")
	}

	if len(hs.serverHello.cookie) != 0 {
		c.sendAlert(alertUnsupportedExtension)
		return errors.New("tls: server sent a cookie in a normal ServerHello")
	}

	if hs.serverHello.selectedGroup != 0 {
		c.sendAlert(alertDecodeError)
		return errors.New("tls: malformed key_share extension")
	}

	if hs.serverHello.serverShare.group == 0 {
		c.sendAlert(alertIllegalParameter)
		return errors.New("tls: server did not send a key share")
	}
	if !slices.ContainsFunc(hs.hello.keyShares, func(ks keyShare) bool {
		return ks.group == hs.serverHello.serverShare.group
	}) {
		c.sendAlert(alertIllegalParameter)
		return errors.New("tls: server selected unsupported group")
	}

	if !hs.serverHello.selectedIdentityPresent {
		return nil
	}

	if int(hs.serverHello.selectedIdentity) >= len(hs.hello.pskIdentities) {
		c.sendAlert(alertIllegalParameter)
		return errors.New("tls: server selected an invalid PSK")
	}

	if len(hs.hello.pskIdentities) != 1 || hs.session == nil {
		return c.sendAlert(alertInternalError)
	}
	pskSuite := cipherSuiteTLS13ByID(hs.session.cipherSuite)
	if pskSuite == nil {
		return c.sendAlert(alertInternalError)
	}
	if pskSuite.hash != hs.suite.hash {
		c.sendAlert(alertIllegalParameter)
		return errors.New("tls: server selected an invalid PSK and cipher suite pair")
	}

	hs.usingPSK = true
	c.didResume = true
	c.peerCertificates = hs.session.peerCertificates
	c.verifiedChains = hs.session.verifiedChains
	c.ocspResponse = hs.session.ocspResponse
	c.scts = hs.session.scts
	return nil
}

func (hs *clientHandshakeStateTLS13) establishHandshakeKeys() error {
	c := hs.c

	ke, err := keyExchangeForCurveID(hs.serverHello.serverShare.group)
	if err != nil {
		c.sendAlert(alertInternalError)
		return err
	}
	sharedKey, err := ke.clientSharedSecret(hs.keyShareKeys, hs.serverHello.serverShare.data)
	if err != nil {
		c.sendAlert(alertIllegalParameter)
		return errors.New("tls: invalid server key share")
	}
	c.curveID = hs.serverHello.serverShare.group

	earlySecret := hs.earlySecret
	if !hs.usingPSK {
		earlySecret = tls13.NewEarlySecret(hs.suite.hash.New, nil)
	}

	handshakeSecret := earlySecret.HandshakeSecret(sharedKey)

	clientSecret := handshakeSecret.ClientHandshakeTrafficSecret(hs.transcript)
	c.setWriteTrafficSecret(hs.suite, QUICEncryptionLevelHandshake, clientSecret)
	serverSecret := handshakeSecret.ServerHandshakeTrafficSecret(hs.transcript)
	if err := c.setReadTrafficSecret(hs.suite, QUICEncryptionLevelHandshake, serverSecret, false); err != nil {
		return err
	}

	if c.quic != nil {
		c.quicSetWriteSecret(QUICEncryptionLevelHandshake, hs.suite.id, clientSecret)
		if err := c.quicSetReadSecret(QUICEncryptionLevelHandshake, hs.suite.id, serverSecret); err != nil {
			return err
		}
	}

	err = c.config.writeKeyLog(keyLogLabelClientHandshake, hs.hello.random, clientSecret)
	if err != nil {
		c.sendAlert(alertInternalError)
		return err
	}
	err = c.config.writeKeyLog(keyLogLabelServerHandshake, hs.hello.random, serverSecret)
	if err != nil {
		c.sendAlert(alertInternalError)
		return err
	}

	hs.masterSecret = handshakeSecret.MasterSecret()

	return nil
}

func (hs *clientHandshakeStateTLS13) readServerParameters() error {
	c := hs.c

	msg, err := c.readHandshake(hs.transcript)
	if err != nil {
		return err
	}

	encryptedExtensions, ok := msg.(*encryptedExtensionsMsg)
	if !ok {
		c.sendAlert(alertUnexpectedMessage)
		return unexpectedMessageError(encryptedExtensions, msg)
	}

	if err := checkALPN(hs.hello.alpnProtocols, encryptedExtensions.alpnProtocol, c.quic != nil); err != nil {
		// RFC 8446 specifies that no_application_protocol is sent by servers, but
		// does not specify how clients handle the selection of an incompatible protocol.
		// RFC 9001 Section 8.1 specifies that QUIC clients send no_application_protocol
		// in this case. Always sending no_application_protocol seems reasonable.
		c.sendAlert(alertNoApplicationProtocol)
		return err
	}
	c.clientProtocol = encryptedExtensions.alpnProtocol

	if c.quic != nil {
		if encryptedExtensions.quicTransportParameters == nil {
			// RFC 9001 Section 8.2.
			c.sendAlert(alertMissingExtension)
			return errors.New("tls: server did not send a quic_transport_parameters extension")
		}
		c.quicSetTransportParameters(encryptedExtensions.quicTransportParameters)
	} else {
		if encryptedExtensions.quicTransportParameters != nil {
			c.sendAlert(alertUnsupportedExtension)
			return errors.New("tls: server sent an unexpected quic_transport_parameters extension")
		}
	}

	if !hs.hello.earlyData && encryptedExtensions.earlyData {
		c.sendAlert(alertUnsupportedExtension)
		return errors.New("tls: server sent an unexpected early_data extension")
	}
	if hs.hello.earlyData && !encryptedExtensions.earlyData {
		c.quicRejectedEarlyData()
	}
	if encryptedExtensions.earlyData {
		if hs.session.cipherSuite != c.cipherSuite {
			c.sendAlert(alertHandshakeFailure)
			return errors.New("tls: server accepted 0-RTT with the wrong cipher suite")
		}
		if hs.session.alpnProtocol != c.clientProtocol {
			c.sendAlert(alertHandshakeFailure)
			return errors.New("tls: server accepted 0-RTT with the wrong ALPN")
		}
	}
	if hs.echContext != nil {
		if hs.echContext.echRejected {
			hs.echContext.retryConfigs = encryptedExtensions.echRetryConfigs
		} else if encryptedExtensions.echRetryConfigs != nil {
			c.sendAlert(alertUnsupportedExtension)
			return errors.New("tls: server sent encrypted client hello retry configs after accepting encrypted client hello")
		}
	}

	return nil
}

func (hs *clientHandshakeStateTLS13) readServerCertificate() error {
	c := hs.c

	// Either a PSK or a certificate is always used, but not both.
	// See RFC 8446, Section 4.1.1.
	if hs.usingPSK {
		// Make sure the connection is still being verified whether or not this
		// is a resumption. Resumptions currently don't reverify certificates so
		// they don't call verifyServerCertificate. See Issue 31641.
		if c.config.VerifyConnection != nil {
			if err := c.config.VerifyConnection(c.connectionStateLocked()); err != nil {
				c.sendAlert(alertBadCertificate)
				return err
			}
		}
		return nil
	}

	msg, err := c.readHandshake(hs.transcript)
	if err != nil {
		return err
	}

	certReq, ok := msg.(*certificateRequestMsgTLS13)
	if ok {
		hs.certReq = certReq

		msg, err = c.readHandshake(hs.transcript)
		if err != nil {
			return err
		}
	}

	certMsg, ok := msg.(*certificateMsgTLS13)
	if !ok {
		c.sendAlert(alertUnexpectedMessage)
		return unexpectedMessageError(certMsg, msg)
	}
	if len(certMsg.certificate.Certificate) == 0 {
		c.sendAlert(alertDecodeError)
		return errors.New("tls: received empty certificates message")
	}

	c.scts = certMsg.certificate.SignedCertificateTimestamps
	c.ocspResponse = certMsg.certificate.OCSPStaple

	if err := c.verifyServerCertificate(certMsg.certificate.Certificate); err != nil {
		return err
	}

	// certificateVerifyMsg is included in the transcript, but not until
	// after we verify the handshake signature, since the state before
	// this message was sent is used.
	msg, err = c.readHandshake(nil)
	if err != nil {
		return err
	}

	certVerify, ok := msg.(*certificateVerifyMsg)
	if !ok {
		c.sendAlert(alertUnexpectedMessage)
		return unexpectedMessageError(certVerify, msg)
	}

	// See RFC 8446, Section 4.4.3.
	// We don't use hs.hello.supportedSignatureAlgorithms because it might
	// include PKCS#1 v1.5 and SHA-1 if the ClientHello also supported TLS 1.2.
	if !isSupportedSignatureAlgorithm(certVerify.signatureAlgorithm, supportedSignatureAlgorithms(c.vers, c.vers)) ||
		!isSupportedSignatureAlgorithm(certVerify.signatureAlgorithm, signatureSchemesForPublicKey(c.vers, c.peerCertificates[0].PublicKey)) {
		c.sendAlert(alertIllegalParameter)
		return errors.New("tls: certificate used with invalid signature algorithm")
	}
	sigType, sigHash, err := typeAndHashFromSignatureScheme(certVerify.signatureAlgorithm)
	if err != nil {
		return c.sendAlert(alertInternalError)
	}
	if sigType == signaturePKCS1v15 || sigHash == crypto.SHA1 {
		return c.sendAlert(alertInternalError)
	}
	signed := signedMessage(serverSignatureContext, hs.transcript)
	if err := verifyHandshakeSignature(sigType, c.peerCertificates[0].PublicKey,
		sigHash, signed, certVerify.signature); err != nil {
		c.sendAlert(alertDecryptError)
		return errors.New("tls: invalid signature by the server certificate: " + err.Error())
	}
	c.peerSigAlg = certVerify.signatureAlgorithm

	if err := transcriptMsg(certVerify, hs.transcript); err != nil {
		return err
	}

	return nil
}

func (hs *clientHandshakeStateTLS13) readServerFinished() error {
	c := hs.c

	// finishedMsg is included in the transcript, but not until after we
	// check the client version, since the state before this message was
	// sent is used during verification.
	msg, err := c.readHandshake(nil)
	if err != nil {
		return err
	}

	finished, ok := msg.(*finishedMsg)
	if !ok {
		c.sendAlert(alertUnexpectedMessage)
		return unexpectedMessageError(finished, msg)
	}

	expectedMAC := hs.suite.finishedHash(c.in.trafficSecret, hs.transcript)
	if !hmac.Equal(expectedMAC, finished.verifyData) {
		c.sendAlert(alertDecryptError)
		return errors.New("tls: invalid server finished hash")
	}

	if err := transcriptMsg(finished, hs.transcript); err != nil {
		return err
	}

	// Derive secrets that take context through the server Finished.

	hs.trafficSecret = hs.masterSecret.ClientApplicationTrafficSecret(hs.transcript)
	serverSecret := hs.masterSecret.ServerApplicationTrafficSecret(hs.transcript)
	if err := c.setReadTrafficSecret(hs.suite, QUICEncryptionLevelApplication, serverSecret, false); err != nil {
		return err
	}

	err = c.config.writeKeyLog(keyLogLabelClientTraffic, hs.hello.random, hs.trafficSecret)
	if err != nil {
		c.sendAlert(alertInternalError)
		return err
	}
	err = c.config.writeKeyLog(keyLogLabelServerTraffic, hs.hello.random, serverSecret)
	if err != nil {
		c.sendAlert(alertInternalError)
		return err
	}

	c.ekm = hs.suite.exportKeyingMaterial(hs.masterSecret, hs.transcript)

	return nil
}

func (hs *clientHandshakeStateTLS13) sendClientCertificate() error {
	c := hs.c

	if hs.certReq == nil {
		return nil
	}

	if hs.echContext != nil && hs.echContext.echRejected {
		if _, err := hs.c.writeHandshakeRecord(&certificateMsgTLS13{}, hs.transcript); err != nil {
			return err
		}
		return nil
	}

	cert, err := c.getClientCertificate(&CertificateRequestInfo{
		AcceptableCAs:    hs.certReq.certificateAuthorities,
		SignatureSchemes: hs.certReq.supportedSignatureAlgorithms,
		Version:          c.vers,
		ctx:              hs.ctx,
	})
	if err != nil {
		return err
	}

	if cert != nil {
		hs.c.localCertificate = cert.Certificate
	}

	certMsg := new(certificateMsgTLS13)

	certMsg.certificate = *cert
	certMsg.scts = hs.certReq.scts && len(cert.SignedCertificateTimestamps) > 0
	certMsg.ocspStapling = hs.certReq.ocspStapling && len(cert.OCSPStaple) > 0

	if _, err := hs.c.writeHandshakeRecord(certMsg, hs.transcript); err != nil {
		return err
	}

	// If we sent an empty certificate message, skip the CertificateVerify.
	if len(cert.Certificate) == 0 {
		return nil
	}

	certVerifyMsg := new(certificateVerifyMsg)
	certVerifyMsg.hasSignatureAlgorithm = true

	certVerifyMsg.signatureAlgorithm, err = selectSignatureScheme(c.vers, cert, hs.certReq.supportedSignatureAlgorithms)
	if err != nil {
		// getClientCertificate returned a certificate incompatible with the
		// CertificateRequestInfo supported signature algorithms.
		c.sendAlert(alertHandshakeFailure)
		return err
	}

	sigType, sigHash, err := typeAndHashFromSignatureScheme(certVerifyMsg.signatureAlgorithm)
	if err != nil {
		return c.sendAlert(alertInternalError)
	}

	signed := signedMessage(clientSignatureContext, hs.transcript)
	signOpts := crypto.SignerOpts(sigHash)
	if sigType == signatureRSAPSS {
		signOpts = &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: sigHash}
	}
	sig, err := crypto.SignMessage(cert.PrivateKey.(crypto.Signer), c.config.rand(), signed, signOpts)
	if err != nil {
		c.sendAlert(alertInternalError)
		return errors.New("tls: failed to sign handshake: " + err.Error())
	}
	certVerifyMsg.signature = sig

	if _, err := hs.c.writeHandshakeRecord(certVerifyMsg, hs.transcript); err != nil {
		return err
	}

	return nil
}

func (hs *clientHandshakeStateTLS13) sendClientFinished() error {
	c := hs.c

	finished := &finishedMsg{
		verifyData: hs.suite.finishedHash(c.out.trafficSecret, hs.transcript),
	}

	if _, err := hs.c.writeHandshakeRecord(finished, hs.transcript); err != nil {
		return err
	}

	c.setWriteTrafficSecret(hs.suite, QUICEncryptionLevelApplication, hs.trafficSecret)

	if !c.config.SessionTicketsDisabled && c.config.ClientSessionCache != nil {
		c.resumptionSecret = hs.masterSecret.ResumptionMasterSecret(hs.transcript)
	}

	if c.quic != nil {
		c.quicSetWriteSecret(QUICEncryptionLevelApplication, hs.suite.id, hs.trafficSecret)
	}

	return nil
}

func (c *Conn) handleNewSessionTicket(msg *newSessionTicketMsgTLS13) error {
	if !c.isClient {
		c.sendAlert(alertUnexpectedMessage)
		return errors.New("tls: received new session ticket from a client")
	}

	if c.config.SessionTicketsDisabled || c.config.ClientSessionCache == nil {
		return nil
	}

	// See RFC 8446, Section 4.6.1.
	if msg.lifetime == 0 {
		return nil
	}
	lifetime := time.Duration(msg.lifetime) * time.Second
	if lifetime > maxSessionTicketLifetime {
		c.sendAlert(alertIllegalParameter)
		return errors.New("tls: received a session ticket with invalid lifetime")
	}

	if len(msg.label) == 0 {
		c.sendAlert(alertDecodeError)
		return errors.New("tls: received a session ticket with empty opaque ticket label")
	}

	// RFC 9001, Section 4.6.1
	if c.quic != nil && msg.maxEarlyData != 0 && msg.maxEarlyData != 0xffffffff {
		c.sendAlert(alertIllegalParameter)
		return errors.New("tls: invalid early data for QUIC connection")
	}

	cipherSuite := cipherSuiteTLS13ByID(c.cipherSuite)
	if cipherSuite == nil || c.resumptionSecret == nil {
		return c.sendAlert(alertInternalError)
	}

	psk := tls13.ExpandLabel(cipherSuite.hash.New, c.resumptionSecret, "resumption",
		msg.nonce, cipherSuite.hash.Size())

	session := c.sessionState()
	session.secret = psk
	session.useBy = uint64(c.config.time().Add(lifetime).Unix())
	session.ageAdd = msg.ageAdd
	session.EarlyData = c.quic != nil && msg.maxEarlyData == 0xffffffff // RFC 9001, Section 4.6.1
	session.ticket = msg.label
	if c.quic != nil && c.quic.enableSessionEvents {
		c.quicStoreSession(session)
		return nil
	}
	cs := &ClientSessionState{session: session}
	if cacheKey := c.clientSessionCacheKey(); cacheKey != "" {
		c.config.ClientSessionCache.Put(cacheKey, cs)
	}

	return nil
}

```

// === FILE: references/go/src/crypto/tls/handshake_messages.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tls

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"golang.org/x/crypto/cryptobyte"
)

// The marshalingFunction type is an adapter to allow the use of ordinary
// functions as cryptobyte.MarshalingValue.
type marshalingFunction func(b *cryptobyte.Builder) error

func (f marshalingFunction) Marshal(b *cryptobyte.Builder) error {
	return f(b)
}

// addBytesWithLength appends a sequence of bytes to the cryptobyte.Builder. If
// the length of the sequence is not the value specified, it produces an error.
func addBytesWithLength(b *cryptobyte.Builder, v []byte, n int) {
	b.AddValue(marshalingFunction(func(b *cryptobyte.Builder) error {
		if len(v) != n {
			return fmt.Errorf("invalid value length: expected %d, got %d", n, len(v))
		}
		b.AddBytes(v)
		return nil
	}))
}

// addUint64 appends a big-endian, 64-bit value to the cryptobyte.Builder.
func addUint64(b *cryptobyte.Builder, v uint64) {
	b.AddUint32(uint32(v >> 32))
	b.AddUint32(uint32(v))
}

// readUint64 decodes a big-endian, 64-bit value into out and advances over it.
// It reports whether the read was successful.
func readUint64(s *cryptobyte.String, out *uint64) bool {
	var hi, lo uint32
	if !s.ReadUint32(&hi) || !s.ReadUint32(&lo) {
		return false
	}
	*out = uint64(hi)<<32 | uint64(lo)
	return true
}

// readUint8LengthPrefixed acts like s.ReadUint8LengthPrefixed, but targets a
// []byte instead of a cryptobyte.String.
func readUint8LengthPrefixed(s *cryptobyte.String, out *[]byte) bool {
	return s.ReadUint8LengthPrefixed((*cryptobyte.String)(out))
}

// readUint16LengthPrefixed acts like s.ReadUint16LengthPrefixed, but targets a
// []byte instead of a cryptobyte.String.
func readUint16LengthPrefixed(s *cryptobyte.String, out *[]byte) bool {
	return s.ReadUint16LengthPrefixed((*cryptobyte.String)(out))
}

// readUint24LengthPrefixed acts like s.ReadUint24LengthPrefixed, but targets a
// []byte instead of a cryptobyte.String.
func readUint24LengthPrefixed(s *cryptobyte.String, out *[]byte) bool {
	return s.ReadUint24LengthPrefixed((*cryptobyte.String)(out))
}

type clientHelloMsg struct {
	original                         []byte
	vers                             uint16
	random                           []byte
	sessionId                        []byte
	cipherSuites                     []uint16
	compressionMethods               []uint8
	serverName                       string
	ocspStapling                     bool
	supportedCurves                  []CurveID
	supportedPoints                  []uint8
	ticketSupported                  bool
	sessionTicket                    []uint8
	supportedSignatureAlgorithms     []SignatureScheme
	supportedSignatureAlgorithmsCert []SignatureScheme
	secureRenegotiationSupported     bool
	secureRenegotiation              []byte
	extendedMasterSecret             bool
	alpnProtocols                    []string
	scts                             bool
	supportedVersions                []uint16
	cookie                           []byte
	keyShares                        []keyShare
	earlyData                        bool
	pskModes                         []uint8
	pskIdentities                    []pskIdentity
	pskBinders                       [][]byte
	quicTransportParameters          []byte
	encryptedClientHello             []byte
	// extensions are only populated on the server-side of a handshake
	extensions []uint16
}

func (m *clientHelloMsg) marshalMsg(echInner bool) ([]byte, error) {
	var exts cryptobyte.Builder
	if len(m.serverName) > 0 {
		// RFC 6066, Section 3
		exts.AddUint16(extensionServerName)
		exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
			exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
				exts.AddUint8(0) // name_type = host_name
				exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
					exts.AddBytes([]byte(m.serverName))
				})
			})
		})
	}
	if len(m.supportedPoints) > 0 && !echInner {
		// RFC 4492, Section 5.1.2
		exts.AddUint16(extensionSupportedPoints)
		exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
			exts.AddUint8LengthPrefixed(func(exts *cryptobyte.Builder) {
				exts.AddBytes(m.supportedPoints)
			})
		})
	}
	if m.ticketSupported && !echInner {
		// RFC 5077, Section 3.2
		exts.AddUint16(extensionSessionTicket)
		exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
			exts.AddBytes(m.sessionTicket)
		})
	}
	if m.secureRenegotiationSupported && !echInner {
		// RFC 5746, Section 3.2
		exts.AddUint16(extensionRenegotiationInfo)
		exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
			exts.AddUint8LengthPrefixed(func(exts *cryptobyte.Builder) {
				exts.AddBytes(m.secureRenegotiation)
			})
		})
	}
	if m.extendedMasterSecret && !echInner {
		// RFC 7627
		exts.AddUint16(extensionExtendedMasterSecret)
		exts.AddUint16(0) // empty extension_data
	}
	if m.scts {
		// RFC 6962, Section 3.3.1
		exts.AddUint16(extensionSCT)
		exts.AddUint16(0) // empty extension_data
	}
	if m.earlyData {
		// RFC 8446, Section 4.2.10
		exts.AddUint16(extensionEarlyData)
		exts.AddUint16(0) // empty extension_data
	}
	if m.quicTransportParameters != nil { // marshal zero-length parameters when present
		// RFC 9001, Section 8.2
		exts.AddUint16(extensionQUICTransportParameters)
		exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
			exts.AddBytes(m.quicTransportParameters)
		})
	}
	if len(m.encryptedClientHello) > 0 {
		exts.AddUint16(extensionEncryptedClientHello)
		exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
			exts.AddBytes(m.encryptedClientHello)
		})
	}
	// Note that any extension that can be compressed during ECH must be
	// contiguous. If any additional extensions are to be compressed they must
	// be added to the following block, so that they can be properly
	// decompressed on the other side.
	var echOuterExts []uint16
	if m.ocspStapling {
		// RFC 4366, Section 3.6
		if echInner {
			echOuterExts = append(echOuterExts, extensionStatusRequest)
		} else {
			exts.AddUint16(extensionStatusRequest)
			exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
				exts.AddUint8(1)  // status_type = ocsp
				exts.AddUint16(0) // empty responder_id_list
				exts.AddUint16(0) // empty request_extensions
			})
		}
	}
	if len(m.supportedCurves) > 0 {
		// RFC 4492, sections 5.1.1 and RFC 8446, Section 4.2.7
		if echInner {
			echOuterExts = append(echOuterExts, extensionSupportedCurves)
		} else {
			exts.AddUint16(extensionSupportedCurves)
			exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
				exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
					for _, curve := range m.supportedCurves {
						exts.AddUint16(uint16(curve))
					}
				})
			})
		}
	}
	if len(m.supportedSignatureAlgorithms) > 0 {
		// RFC 5246, Section 7.4.1.4.1
		if echInner {
			echOuterExts = append(echOuterExts, extensionSignatureAlgorithms)
		} else {
			exts.AddUint16(extensionSignatureAlgorithms)
			exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
				exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
					for _, sigAlgo := range m.supportedSignatureAlgorithms {
						exts.AddUint16(uint16(sigAlgo))
					}
				})
			})
		}
	}
	if len(m.supportedSignatureAlgorithmsCert) > 0 {
		// RFC 8446, Section 4.2.3
		if echInner {
			echOuterExts = append(echOuterExts, extensionSignatureAlgorithmsCert)
		} else {
			exts.AddUint16(extensionSignatureAlgorithmsCert)
			exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
				exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
					for _, sigAlgo := range m.supportedSignatureAlgorithmsCert {
						exts.AddUint16(uint16(sigAlgo))
					}
				})
			})
		}
	}
	if len(m.alpnProtocols) > 0 {
		// RFC 7301, Section 3.1
		if echInner {
			echOuterExts = append(echOuterExts, extensionALPN)
		} else {
			exts.AddUint16(extensionALPN)
			exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
				exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
					for _, proto := range m.alpnProtocols {
						exts.AddUint8LengthPrefixed(func(exts *cryptobyte.Builder) {
							exts.AddBytes([]byte(proto))
						})
					}
				})
			})
		}
	}
	if len(m.supportedVersions) > 0 {
		// RFC 8446, Section 4.2.1
		if echInner {
			echOuterExts = append(echOuterExts, extensionSupportedVersions)
		} else {
			exts.AddUint16(extensionSupportedVersions)
			exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
				exts.AddUint8LengthPrefixed(func(exts *cryptobyte.Builder) {
					for _, vers := range m.supportedVersions {
						exts.AddUint16(vers)
					}
				})
			})
		}
	}
	if len(m.cookie) > 0 {
		// RFC 8446, Section 4.2.2
		if echInner {
			echOuterExts = append(echOuterExts, extensionCookie)
		} else {
			exts.AddUint16(extensionCookie)
			exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
				exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
					exts.AddBytes(m.cookie)
				})
			})
		}
	}
	if len(m.keyShares) > 0 {
		// RFC 8446, Section 4.2.8
		if echInner {
			echOuterExts = append(echOuterExts, extensionKeyShare)
		} else {
			exts.AddUint16(extensionKeyShare)
			exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
				exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
					for _, ks := range m.keyShares {
						exts.AddUint16(uint16(ks.group))
						exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
							exts.AddBytes(ks.data)
						})
					}
				})
			})
		}
	}
	if len(m.pskModes) > 0 {
		// RFC 8446, Section 4.2.9
		if echInner {
			echOuterExts = append(echOuterExts, extensionPSKModes)
		} else {
			exts.AddUint16(extensionPSKModes)
			exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
				exts.AddUint8LengthPrefixed(func(exts *cryptobyte.Builder) {
					exts.AddBytes(m.pskModes)
				})
			})
		}
	}
	if len(echOuterExts) > 0 && echInner {
		exts.AddUint16(extensionECHOuterExtensions)
		exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
			exts.AddUint8LengthPrefixed(func(exts *cryptobyte.Builder) {
				for _, e := range echOuterExts {
					exts.AddUint16(e)
				}
			})
		})
	}
	if len(m.pskIdentities) > 0 { // pre_shared_key must be the last extension
		// RFC 8446, Section 4.2.11
		exts.AddUint16(extensionPreSharedKey)
		exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
			exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
				for _, psk := range m.pskIdentities {
					exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
						exts.AddBytes(psk.label)
					})
					exts.AddUint32(psk.obfuscatedTicketAge)
				}
			})
			exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
				for _, binder := range m.pskBinders {
					exts.AddUint8LengthPrefixed(func(exts *cryptobyte.Builder) {
						exts.AddBytes(binder)
					})
				}
			})
		})
	}
	extBytes, err := exts.Bytes()
	if err != nil {
		return nil, err
	}

	var b cryptobyte.Builder
	b.AddUint8(typeClientHello)
	b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
		b.AddUint16(m.vers)
		addBytesWithLength(b, m.random, 32)
		b.AddUint8LengthPrefixed(func(b *cryptobyte.Builder) {
			if !echInner {
				b.AddBytes(m.sessionId)
			}
		})
		b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
			for _, suite := range m.cipherSuites {
				b.AddUint16(suite)
			}
		})
		b.AddUint8LengthPrefixed(func(b *cryptobyte.Builder) {
			b.AddBytes(m.compressionMethods)
		})

		if len(extBytes) > 0 {
			b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
				b.AddBytes(extBytes)
			})
		}
	})

	return b.Bytes()
}

func (m *clientHelloMsg) marshal() ([]byte, error) {
	return m.marshalMsg(false)
}

// marshalWithoutBinders returns the ClientHello through the
// PreSharedKeyExtension.identities field, according to RFC 8446, Section
// 4.2.11.2. Note that m.pskBinders must be set to slices of the correct length.
func (m *clientHelloMsg) marshalWithoutBinders() ([]byte, error) {
	bindersLen := 2 // uint16 length prefix
	for _, binder := range m.pskBinders {
		bindersLen += 1 // uint8 length prefix
		bindersLen += len(binder)
	}

	var fullMessage []byte
	if m.original != nil {
		fullMessage = m.original
	} else {
		var err error
		fullMessage, err = m.marshal()
		if err != nil {
			return nil, err
		}
	}
	return fullMessage[:len(fullMessage)-bindersLen], nil
}

// updateBinders updates the m.pskBinders field. The supplied binders must have
// the same length as the current m.pskBinders.
func (m *clientHelloMsg) updateBinders(pskBinders [][]byte) error {
	if len(pskBinders) != len(m.pskBinders) {
		return errors.New("tls: internal error: pskBinders length mismatch")
	}
	for i := range m.pskBinders {
		if len(pskBinders[i]) != len(m.pskBinders[i]) {
			return errors.New("tls: internal error: pskBinders length mismatch")
		}
	}
	m.pskBinders = pskBinders

	return nil
}

func (m *clientHelloMsg) unmarshal(data []byte) bool {
	*m = clientHelloMsg{original: data}
	s := cryptobyte.String(data)

	if !s.Skip(4) || // message type and uint24 length field
		!s.ReadUint16(&m.vers) || !s.ReadBytes(&m.random, 32) ||
		!readUint8LengthPrefixed(&s, &m.sessionId) {
		return false
	}

	var cipherSuites cryptobyte.String
	if !s.ReadUint16LengthPrefixed(&cipherSuites) {
		return false
	}
	m.cipherSuites = []uint16{}
	m.secureRenegotiationSupported = false
	for !cipherSuites.Empty() {
		var suite uint16
		if !cipherSuites.ReadUint16(&suite) {
			return false
		}
		if suite == scsvRenegotiation {
			m.secureRenegotiationSupported = true
		}
		m.cipherSuites = append(m.cipherSuites, suite)
	}

	if !readUint8LengthPrefixed(&s, &m.compressionMethods) {
		return false
	}

	if s.Empty() {
		// ClientHello is optionally followed by extension data
		return true
	}

	var extensions cryptobyte.String
	if !s.ReadUint16LengthPrefixed(&extensions) || !s.Empty() {
		return false
	}

	seenExts := make(map[uint16]bool)
	for !extensions.Empty() {
		var extension uint16
		var extData cryptobyte.String
		if !extensions.ReadUint16(&extension) ||
			!extensions.ReadUint16LengthPrefixed(&extData) {
			return false
		}

		if seenExts[extension] {
			return false
		}
		seenExts[extension] = true
		m.extensions = append(m.extensions, extension)

		switch extension {
		case extensionServerName:
			// RFC 6066, Section 3
			var nameList cryptobyte.String
			if !extData.ReadUint16LengthPrefixed(&nameList) || nameList.Empty() {
				return false
			}
			for !nameList.Empty() {
				var nameType uint8
				var serverName cryptobyte.String
				if !nameList.ReadUint8(&nameType) ||
					!nameList.ReadUint16LengthPrefixed(&serverName) ||
					serverName.Empty() {
					return false
				}
				if nameType != 0 {
					continue
				}
				if len(m.serverName) != 0 {
					// Multiple names of the same name_type are prohibited.
					return false
				}
				m.serverName = string(serverName)
				// An SNI value may not include a trailing dot.
				if strings.HasSuffix(m.serverName, ".") {
					return false
				}
			}
		case extensionStatusRequest:
			// RFC 4366, Section 3.6
			var statusType uint8
			var ignored cryptobyte.String
			if !extData.ReadUint8(&statusType) ||
				!extData.ReadUint16LengthPrefixed(&ignored) ||
				!extData.ReadUint16LengthPrefixed(&ignored) {
				return false
			}
			m.ocspStapling = statusType == statusTypeOCSP
		case extensionSupportedCurves:
			// RFC 4492, sections 5.1.1 and RFC 8446, Section 4.2.7
			var curves cryptobyte.String
			if !extData.ReadUint16LengthPrefixed(&curves) || curves.Empty() {
				return false
			}
			for !curves.Empty() {
				var curve uint16
				if !curves.ReadUint16(&curve) {
					return false
				}
				m.supportedCurves = append(m.supportedCurves, CurveID(curve))
			}
		case extensionSupportedPoints:
			// RFC 4492, Section 5.1.2
			if !readUint8LengthPrefixed(&extData, &m.supportedPoints) ||
				len(m.supportedPoints) == 0 {
				return false
			}
		case extensionSessionTicket:
			// RFC 5077, Section 3.2
			m.ticketSupported = true
			extData.ReadBytes(&m.sessionTicket, len(extData))
		case extensionSignatureAlgorithms:
			// RFC 5246, Section 7.4.1.4.1
			var sigAndAlgs cryptobyte.String
			if !extData.ReadUint16LengthPrefixed(&sigAndAlgs) || sigAndAlgs.Empty() {
				return false
			}
			for !sigAndAlgs.Empty() {
				var sigAndAlg uint16
				if !sigAndAlgs.ReadUint16(&sigAndAlg) {
					return false
				}
				m.supportedSignatureAlgorithms = append(
					m.supportedSignatureAlgorithms, SignatureScheme(sigAndAlg))
			}
		case extensionSignatureAlgorithmsCert:
			// RFC 8446, Section 4.2.3
			var sigAndAlgs cryptobyte.String
			if !extData.ReadUint16LengthPrefixed(&sigAndAlgs) || sigAndAlgs.Empty() {
				return false
			}
			for !sigAndAlgs.Empty() {
				var sigAndAlg uint16
				if !sigAndAlgs.ReadUint16(&sigAndAlg) {
					return false
				}
				m.supportedSignatureAlgorithmsCert = append(
					m.supportedSignatureAlgorithmsCert, SignatureScheme(sigAndAlg))
			}
		case extensionRenegotiationInfo:
			// RFC 5746, Section 3.2
			if !readUint8LengthPrefixed(&extData, &m.secureRenegotiation) {
				return false
			}
			m.secureRenegotiationSupported = true
		case extensionExtendedMasterSecret:
			// RFC 7627
			m.extendedMasterSecret = true
		case extensionALPN:
			// RFC 7301, Section 3.1
			var protoList cryptobyte.String
			if !extData.ReadUint16LengthPrefixed(&protoList) || protoList.Empty() {
				return false
			}
			for !protoList.Empty() {
				var proto cryptobyte.String
				if !protoList.ReadUint8LengthPrefixed(&proto) || proto.Empty() {
					return false
				}
				m.alpnProtocols = append(m.alpnProtocols, string(proto))
			}
		case extensionSCT:
			// RFC 6962, Section 3.3.1
			m.scts = true
		case extensionSupportedVersions:
			// RFC 8446, Section 4.2.1
			var versList cryptobyte.String
			if !extData.ReadUint8LengthPrefixed(&versList) || versList.Empty() {
				return false
			}
			for !versList.Empty() {
				var vers uint16
				if !versList.ReadUint16(&vers) {
					return false
				}
				m.supportedVersions = append(m.supportedVersions, vers)
			}
		case extensionCookie:
			// RFC 8446, Section 4.2.2
			if !readUint16LengthPrefixed(&extData, &m.cookie) ||
				len(m.cookie) == 0 {
				return false
			}
		case extensionKeyShare:
			// RFC 8446, Section 4.2.8
			var clientShares cryptobyte.String
			if !extData.ReadUint16LengthPrefixed(&clientShares) {
				return false
			}
			for !clientShares.Empty() {
				var ks keyShare
				if !clientShares.ReadUint16((*uint16)(&ks.group)) ||
					!readUint16LengthPrefixed(&clientShares, &ks.data) ||
					len(ks.data) == 0 {
					return false
				}
				m.keyShares = append(m.keyShares, ks)
			}
		case extensionEarlyData:
			// RFC 8446, Section 4.2.10
			m.earlyData = true
		case extensionPSKModes:
			// RFC 8446, Section 4.2.9
			if !readUint8LengthPrefixed(&extData, &m.pskModes) {
				return false
			}
		case extensionQUICTransportParameters:
			m.quicTransportParameters = make([]byte, len(extData))
			if !extData.CopyBytes(m.quicTransportParameters) {
				return false
			}
		case extensionPreSharedKey:
			// RFC 8446, Section 4.2.11
			if !extensions.Empty() {
				return false // pre_shared_key must be the last extension
			}
			var identities cryptobyte.String
			if !extData.ReadUint16LengthPrefixed(&identities) || identities.Empty() {
				return false
			}
			for !identities.Empty() {
				var psk pskIdentity
				if !readUint16LengthPrefixed(&identities, &psk.label) ||
					!identities.ReadUint32(&psk.obfuscatedTicketAge) ||
					len(psk.label) == 0 {
					return false
				}
				m.pskIdentities = append(m.pskIdentities, psk)
			}
			var binders cryptobyte.String
			if !extData.ReadUint16LengthPrefixed(&binders) || binders.Empty() {
				return false
			}
			for !binders.Empty() {
				var binder []byte
				if !readUint8LengthPrefixed(&binders, &binder) ||
					len(binder) == 0 {
					return false
				}
				m.pskBinders = append(m.pskBinders, binder)
			}
		case extensionEncryptedClientHello:
			if !extData.ReadBytes(&m.encryptedClientHello, len(extData)) {
				return false
			}
		default:
			// Ignore unknown extensions.
			continue
		}

		if !extData.Empty() {
			return false
		}
	}

	return true
}

func (m *clientHelloMsg) originalBytes() []byte {
	return m.original
}

func (m *clientHelloMsg) clone() *clientHelloMsg {
	return &clientHelloMsg{
		original:                         slices.Clone(m.original),
		vers:                             m.vers,
		random:                           slices.Clone(m.random),
		sessionId:                        slices.Clone(m.sessionId),
		cipherSuites:                     slices.Clone(m.cipherSuites),
		compressionMethods:               slices.Clone(m.compressionMethods),
		serverName:                       m.serverName,
		ocspStapling:                     m.ocspStapling,
		supportedCurves:                  slices.Clone(m.supportedCurves),
		supportedPoints:                  slices.Clone(m.supportedPoints),
		ticketSupported:                  m.ticketSupported,
		sessionTicket:                    slices.Clone(m.sessionTicket),
		supportedSignatureAlgorithms:     slices.Clone(m.supportedSignatureAlgorithms),
		supportedSignatureAlgorithmsCert: slices.Clone(m.supportedSignatureAlgorithmsCert),
		secureRenegotiationSupported:     m.secureRenegotiationSupported,
		secureRenegotiation:              slices.Clone(m.secureRenegotiation),
		extendedMasterSecret:             m.extendedMasterSecret,
		alpnProtocols:                    slices.Clone(m.alpnProtocols),
		scts:                             m.scts,
		supportedVersions:                slices.Clone(m.supportedVersions),
		cookie:                           slices.Clone(m.cookie),
		keyShares:                        slices.Clone(m.keyShares),
		earlyData:                        m.earlyData,
		pskModes:                         slices.Clone(m.pskModes),
		pskIdentities:                    slices.Clone(m.pskIdentities),
		pskBinders:                       slices.Clone(m.pskBinders),
		quicTransportParameters:          slices.Clone(m.quicTransportParameters),
		encryptedClientHello:             slices.Clone(m.encryptedClientHello),
	}
}

type serverHelloMsg struct {
	original                     []byte
	vers                         uint16
	random                       []byte
	sessionId                    []byte
	cipherSuite                  uint16
	compressionMethod            uint8
	ocspStapling                 bool
	ticketSupported              bool
	secureRenegotiationSupported bool
	secureRenegotiation          []byte
	extendedMasterSecret         bool
	alpnProtocol                 string
	scts                         [][]byte
	supportedVersion             uint16
	serverShare                  keyShare
	selectedIdentityPresent      bool
	selectedIdentity             uint16
	supportedPoints              []uint8
	encryptedClientHello         []byte
	serverNameAck                bool

	// HelloRetryRequest extensions
	cookie        []byte
	selectedGroup CurveID
}

func (m *serverHelloMsg) marshal() ([]byte, error) {
	var exts cryptobyte.Builder
	if m.ocspStapling {
		exts.AddUint16(extensionStatusRequest)
		exts.AddUint16(0) // empty extension_data
	}
	if m.ticketSupported {
		exts.AddUint16(extensionSessionTicket)
		exts.AddUint16(0) // empty extension_data
	}
	if m.secureRenegotiationSupported {
		exts.AddUint16(extensionRenegotiationInfo)
		exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
			exts.AddUint8LengthPrefixed(func(exts *cryptobyte.Builder) {
				exts.AddBytes(m.secureRenegotiation)
			})
		})
	}
	if m.extendedMasterSecret {
		exts.AddUint16(extensionExtendedMasterSecret)
		exts.AddUint16(0) // empty extension_data
	}
	if len(m.alpnProtocol) > 0 {
		exts.AddUint16(extensionALPN)
		exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
			exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
				exts.AddUint8LengthPrefixed(func(exts *cryptobyte.Builder) {
					exts.AddBytes([]byte(m.alpnProtocol))
				})
			})
		})
	}
	if len(m.scts) > 0 {
		exts.AddUint16(extensionSCT)
		exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
			exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
				for _, sct := range m.scts {
					exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
						exts.AddBytes(sct)
					})
				}
			})
		})
	}
	if m.supportedVersion != 0 {
		exts.AddUint16(extensionSupportedVersions)
		exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
			exts.AddUint16(m.supportedVersion)
		})
	}
	if m.serverShare.group != 0 {
		exts.AddUint16(extensionKeyShare)
		exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
			exts.AddUint16(uint16(m.serverShare.group))
			exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
				exts.AddBytes(m.serverShare.data)
			})
		})
	}
	if m.selectedIdentityPresent {
		exts.AddUint16(extensionPreSharedKey)
		exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
			exts.AddUint16(m.selectedIdentity)
		})
	}

	if len(m.cookie) > 0 {
		exts.AddUint16(extensionCookie)
		exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
			exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
				exts.AddBytes(m.cookie)
			})
		})
	}
	if m.selectedGroup != 0 {
		exts.AddUint16(extensionKeyShare)
		exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
			exts.AddUint16(uint16(m.selectedGroup))
		})
	}
	if len(m.supportedPoints) > 0 {
		exts.AddUint16(extensionSupportedPoints)
		exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
			exts.AddUint8LengthPrefixed(func(exts *cryptobyte.Builder) {
				exts.AddBytes(m.supportedPoints)
			})
		})
	}
	if len(m.encryptedClientHello) > 0 {
		exts.AddUint16(extensionEncryptedClientHello)
		exts.AddUint16LengthPrefixed(func(exts *cryptobyte.Builder) {
			exts.AddBytes(m.encryptedClientHello)
		})
	}
	if m.serverNameAck {
		exts.AddUint16(extensionServerName)
		exts.AddUint16(0)
	}

	extBytes, err := exts.Bytes()
	if err != nil {
		return nil, err
	}

	var b cryptobyte.Builder
	b.AddUint8(typeServerHello)
	b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
		b.AddUint16(m.vers)
		addBytesWithLength(b, m.random, 32)
		b.AddUint8LengthPrefixed(func(b *cryptobyte.Builder) {
			b.AddBytes(m.sessionId)
		})
		b.AddUint16(m.cipherSuite)
		b.AddUint8(m.compressionMethod)

		if len(extBytes) > 0 {
			b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
				b.AddBytes(extBytes)
			})
		}
	})

	return b.Bytes()
}

func (m *serverHelloMsg) unmarshal(data []byte) bool {
	*m = serverHelloMsg{original: data}
	s := cryptobyte.String(data)

	if !s.Skip(4) || // message type and uint24 length field
		!s.ReadUint16(&m.vers) || !s.ReadBytes(&m.random, 32) ||
		!readUint8LengthPrefixed(&s, &m.sessionId) ||
		!s.ReadUint16(&m.cipherSuite) ||
		!s.ReadUint8(&m.compressionMethod) {
		return false
	}

	if s.Empty() {
		// ServerHello is optionally followed by extension data
		return true
	}

	var extensions cryptobyte.String
	if !s.ReadUint16LengthPrefixed(&extensions) || !s.Empty() {
		return false
	}

	seenExts := make(map[uint16]bool)
	for !extensions.Empty() {
		var extension uint16
		var extData cryptobyte.String
		if !extensions.ReadUint16(&extension) ||
			!extensions.ReadUint16LengthPrefixed(&extData) {
			return false
		}

		if seenExts[extension] {
			return false
		}
		seenExts[extension] = true

		switch extension {
		case extensionStatusRequest:
			m.ocspStapling = true
		case extensionSessionTicket:
			m.ticketSupported = true
		case extensionRenegotiationInfo:
			if !readUint8LengthPrefixed(&extData, &m.secureRenegotiation) {
				return false
			}
			m.secureRenegotiationSupported = true
		case extensionExtendedMasterSecret:
			m.extendedMasterSecret = true
		case extensionALPN:
			var protoList cryptobyte.String
			if !extData.ReadUint16LengthPrefixed(&protoList) || protoList.Empty() {
				return false
			}
			var proto cryptobyte.String
			if !protoList.ReadUint8LengthPrefixed(&proto) ||
				proto.Empty() || !protoList.Empty() {
				return false
			}
			m.alpnProtocol = string(proto)
		case extensionSCT:
			var sctList cryptobyte.String
			if !extData.ReadUint16LengthPrefixed(&sctList) || sctList.Empty() {
				return false
			}
			for !sctList.Empty() {
				var sct []byte
				if !readUint16LengthPrefixed(&sctList, &sct) ||
					len(sct) == 0 {
					return false
				}
				m.scts = append(m.scts, sct)
			}
		case extensionSupportedVersions:
			if !extData.ReadUint16(&m.supportedVersion) {
				return false
			}
		case extensionCookie:
			if !readUint16LengthPrefixed(&extData, &m.cookie) ||
				len(m.cookie) == 0 {
				return false
			}
		case extensionKeyShare:
			// This extension has different formats in SH and HRR, accept either
			// and let the handshake logic decide. See RFC 8446, Section 4.2.8.
			if len(extData) == 2 {
				if !extData.ReadUint16((*uint16)(&m.selectedGroup)) {
					return false
				}
			} else {
				if !extData.ReadUint16((*uint16)(&m.serverShare.group)) ||
					!readUint16LengthPrefixed(&extData, &m.serverShare.data) {
					return false
				}
			}
		case extensionPreSharedKey:
			m.selectedIdentityPresent = true
			if !extData.ReadUint16(&m.selectedIdentity) {
				return false
			}
		case extensionSupportedPoints:
			// RFC 4492, Section 5.1.2
			if !readUint8LengthPrefixed(&extData, &m.supportedPoints) ||
				len(m.supportedPoints) == 0 {
				return false
			}
		case extensionEncryptedClientHello: // encrypted_client_hello
			m.encryptedClientHello = make([]byte, len(extData))
			if !extData.CopyBytes(m.encryptedClientHello) {
				return false
			}
		case extensionServerName:
			if len(extData) != 0 {
				return false
			}
			m.serverNameAck = true
		default:
			// Ignore unknown extensions.
			continue
		}

		if !extData.Empty() {
			return false
		}
	}

	return true
}

func (m *serverHelloMsg) originalBytes() []byte {
	return m.original
}

type encryptedExtensionsMsg struct {
	alpnProtocol            string
	quicTransportParameters []byte
	earlyData               bool
	echRetryConfigs         []byte
	serverNameAck           bool
}

func (m *encryptedExtensionsMsg) marshal() ([]byte, error) {
	var b cryptobyte.Builder
	b.AddUint8(typeEncryptedExtensions)
	b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
		b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
			if len(m.alpnProtocol) > 0 {
				b.AddUint16(extensionALPN)
				b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
					b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
						b.AddUint8LengthPrefixed(func(b *cryptobyte.Builder) {
							b.AddBytes([]byte(m.alpnProtocol))
						})
					})
				})
			}
			if m.quicTransportParameters != nil { // marshal zero-length parameters when present
				// draft-ietf-quic-tls-32, Section 8.2
				b.AddUint16(extensionQUICTransportParameters)
				b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
					b.AddBytes(m.quicTransportParameters)
				})
			}
			if m.earlyData {
				// RFC 8446, Section 4.2.10
				b.AddUint16(extensionEarlyData)
				b.AddUint16(0) // empty extension_data
			}
			if len(m.echRetryConfigs) > 0 {
				b.AddUint16(extensionEncryptedClientHello)
				b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
					b.AddBytes(m.echRetryConfigs)
				})
			}
			if m.serverNameAck {
				b.AddUint16(extensionServerName)
				b.AddUint16(0) // empty extension_data
			}
		})
	})

	return b.Bytes()
}

func (m *encryptedExtensionsMsg) unmarshal(data []byte) bool {
	*m = encryptedExtensionsMsg{}
	s := cryptobyte.String(data)

	var extensions cryptobyte.String
	if !s.Skip(4) || // message type and uint24 length field
		!s.ReadUint16LengthPrefixed(&extensions) || !s.Empty() {
		return false
	}

	seenExts := make(map[uint16]bool)
	for !extensions.Empty() {
		var extension uint16
		var extData cryptobyte.String
		if !extensions.ReadUint16(&extension) ||
			!extensions.ReadUint16LengthPrefixed(&extData) {
			return false
		}

		if seenExts[extension] {
			return false
		}
		seenExts[extension] = true

		switch extension {
		case extensionALPN:
			var protoList cryptobyte.String
			if !extData.ReadUint16LengthPrefixed(&protoList) || protoList.Empty() {
				return false
			}
			var proto cryptobyte.String
			if !protoList.ReadUint8LengthPrefixed(&proto) ||
				proto.Empty() || !protoList.Empty() {
				return false
			}
			m.alpnProtocol = string(proto)
		case extensionQUICTransportParameters:
			m.quicTransportParameters = make([]byte, len(extData))
			if !extData.CopyBytes(m.quicTransportParameters) {
				return false
			}
		case extensionEarlyData:
			// RFC 8446, Section 4.2.10
			m.earlyData = true
		case extensionEncryptedClientHello:
			m.echRetryConfigs = make([]byte, len(extData))
			if !extData.CopyBytes(m.echRetryConfigs) {
				return false
			}
		case extensionServerName:
			if len(extData) != 0 {
				return false
			}
			m.serverNameAck = true
		default:
			// Ignore unknown extensions.
			continue
		}

		if !extData.Empty() {
			return false
		}
	}

	return true
}

type endOfEarlyDataMsg struct{}

func (m *endOfEarlyDataMsg) marshal() ([]byte, error) {
	x := make([]byte, 4)
	x[0] = typeEndOfEarlyData
	return x, nil
}

func (m *endOfEarlyDataMsg) unmarshal(data []byte) bool {
	return len(data) == 4
}

type keyUpdateMsg struct {
	updateRequested bool
}

func (m *keyUpdateMsg) marshal() ([]byte, error) {
	var b cryptobyte.Builder
	b.AddUint8(typeKeyUpdate)
	b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
		if m.updateRequested {
			b.AddUint8(1)
		} else {
			b.AddUint8(0)
		}
	})

	return b.Bytes()
}

func (m *keyUpdateMsg) unmarshal(data []byte) bool {
	s := cryptobyte.String(data)

	var updateRequested uint8
	if !s.Skip(4) || // message type and uint24 length field
		!s.ReadUint8(&updateRequested) || !s.Empty() {
		return false
	}
	switch updateRequested {
	case 0:
		m.updateRequested = false
	case 1:
		m.updateRequested = true
	default:
		return false
	}
	return true
}

type newSessionTicketMsgTLS13 struct {
	lifetime     uint32
	ageAdd       uint32
	nonce        []byte
	label        []byte
	maxEarlyData uint32
}

func (m *newSessionTicketMsgTLS13) marshal() ([]byte, error) {
	var b cryptobyte.Builder
	b.AddUint8(typeNewSessionTicket)
	b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
		b.AddUint32(m.lifetime)
		b.AddUint32(m.ageAdd)
		b.AddUint8LengthPrefixed(func(b *cryptobyte.Builder) {
			b.AddBytes(m.nonce)
		})
		b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
			b.AddBytes(m.label)
		})

		b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
			if m.maxEarlyData > 0 {
				b.AddUint16(extensionEarlyData)
				b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
					b.AddUint32(m.maxEarlyData)
				})
			}
		})
	})

	return b.Bytes()
}

func (m *newSessionTicketMsgTLS13) unmarshal(data []byte) bool {
	*m = newSessionTicketMsgTLS13{}
	s := cryptobyte.String(data)

	var extensions cryptobyte.String
	if !s.Skip(4) || // message type and uint24 length field
		!s.ReadUint32(&m.lifetime) ||
		!s.ReadUint32(&m.ageAdd) ||
		!readUint8LengthPrefixed(&s, &m.nonce) ||
		!readUint16LengthPrefixed(&s, &m.label) ||
		!s.ReadUint16LengthPrefixed(&extensions) ||
		!s.Empty() {
		return false
	}

	for !extensions.Empty() {
		var extension uint16
		var extData cryptobyte.String
		if !extensions.ReadUint16(&extension) ||
			!extensions.ReadUint16LengthPrefixed(&extData) {
			return false
		}

		switch extension {
		case extensionEarlyData:
			if !extData.ReadUint32(&m.maxEarlyData) {
				return false
			}
		default:
			// Ignore unknown extensions.
			continue
		}

		if !extData.Empty() {
			return false
		}
	}

	return true
}

type certificateRequestMsgTLS13 struct {
	ocspStapling                     bool
	scts                             bool
	supportedSignatureAlgorithms     []SignatureScheme
	supportedSignatureAlgorithmsCert []SignatureScheme
	certificateAuthorities           [][]byte
}

func (m *certificateRequestMsgTLS13) marshal() ([]byte, error) {
	var b cryptobyte.Builder
	b.AddUint8(typeCertificateRequest)
	b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
		// certificate_request_context (SHALL be zero length unless used for
		// post-handshake authentication)
		b.AddUint8(0)

		b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
			if m.ocspStapling {
				b.AddUint16(extensionStatusRequest)
				b.AddUint16(0) // empty extension_data
			}
			if m.scts {
				// RFC 8446, Section 4.4.2.1 makes no mention of
				// signed_certificate_timestamp in CertificateRequest, but
				// "Extensions in the Certificate message from the client MUST
				// correspond to extensions in the CertificateRequest message
				// from the server." and it appears in the table in Section 4.2.
				b.AddUint16(extensionSCT)
				b.AddUint16(0) // empty extension_data
			}
			if len(m.supportedSignatureAlgorithms) > 0 {
				b.AddUint16(extensionSignatureAlgorithms)
				b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
					b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
						for _, sigAlgo := range m.supportedSignatureAlgorithms {
							b.AddUint16(uint16(sigAlgo))
						}
					})
				})
			}
			if len(m.supportedSignatureAlgorithmsCert) > 0 {
				b.AddUint16(extensionSignatureAlgorithmsCert)
				b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
					b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
						for _, sigAlgo := range m.supportedSignatureAlgorithmsCert {
							b.AddUint16(uint16(sigAlgo))
						}
					})
				})
			}
			if len(m.certificateAuthorities) > 0 {
				b.AddUint16(extensionCertificateAuthorities)
				b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
					b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
						for _, ca := range m.certificateAuthorities {
							b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
								b.AddBytes(ca)
							})
						}
					})
				})
			}
		})
	})

	return b.Bytes()
}

func (m *certificateRequestMsgTLS13) unmarshal(data []byte) bool {
	*m = certificateRequestMsgTLS13{}
	s := cryptobyte.String(data)

	var context, extensions cryptobyte.String
	if !s.Skip(4) || // message type and uint24 length field
		!s.ReadUint8LengthPrefixed(&context) || !context.Empty() ||
		!s.ReadUint16LengthPrefixed(&extensions) ||
		!s.Empty() {
		return false
	}

	for !extensions.Empty() {
		var extension uint16
		var extData cryptobyte.String
		if !extensions.ReadUint16(&extension) ||
			!extensions.ReadUint16LengthPrefixed(&extData) {
			return false
		}

		switch extension {
		case extensionStatusRequest:
			m.ocspStapling = true
		case extensionSCT:
			m.scts = true
		case extensionSignatureAlgorithms:
			var sigAndAlgs cryptobyte.String
			if !extData.ReadUint16LengthPrefixed(&sigAndAlgs) || sigAndAlgs.Empty() {
				return false
			}
			for !sigAndAlgs.Empty() {
				var sigAndAlg uint16
				if !sigAndAlgs.ReadUint16(&sigAndAlg) {
					return false
				}
				m.supportedSignatureAlgorithms = append(
					m.supportedSignatureAlgorithms, SignatureScheme(sigAndAlg))
			}
		case extensionSignatureAlgorithmsCert:
			var sigAndAlgs cryptobyte.String
			if !extData.ReadUint16LengthPrefixed(&sigAndAlgs) || sigAndAlgs.Empty() {
				return false
			}
			for !sigAndAlgs.Empty() {
				var sigAndAlg uint16
				if !sigAndAlgs.ReadUint16(&sigAndAlg) {
					return false
				}
				m.supportedSignatureAlgorithmsCert = append(
					m.supportedSignatureAlgorithmsCert, SignatureScheme(sigAndAlg))
			}
		case extensionCertificateAuthorities:
			var auths cryptobyte.String
			if !extData.ReadUint16LengthPrefixed(&auths) || auths.Empty() {
				return false
			}
			for !auths.Empty() {
				var ca []byte
				if !readUint16LengthPrefixed(&auths, &ca) || len(ca) == 0 {
					return false
				}
				m.certificateAuthorities = append(m.certificateAuthorities, ca)
			}
		default:
			// Ignore unknown extensions.
			continue
		}

		if !extData.Empty() {
			return false
		}
	}

	return true
}

type certificateMsg struct {
	certificates [][]byte
}

func (m *certificateMsg) marshal() ([]byte, error) {
	var i int
	for _, slice := range m.certificates {
		i += len(slice)
	}

	length := 3 + 3*len(m.certificates) + i
	x := make([]byte, 4+length)
	x[0] = typeCertificate
	x[1] = uint8(length >> 16)
	x[2] = uint8(length >> 8)
	x[3] = uint8(length)

	certificateOctets := length - 3
	x[4] = uint8(certificateOctets >> 16)
	x[5] = uint8(certificateOctets >> 8)
	x[6] = uint8(certificateOctets)

	y := x[7:]
	for _, slice := range m.certificates {
		y[0] = uint8(len(slice) >> 16)
		y[1] = uint8(len(slice) >> 8)
		y[2] = uint8(len(slice))
		copy(y[3:], slice)
		y = y[3+len(slice):]
	}

	return x, nil
}

func (m *certificateMsg) unmarshal(data []byte) bool {
	if len(data) < 7 {
		return false
	}

	certsLen := uint32(data[4])<<16 | uint32(data[5])<<8 | uint32(data[6])
	if uint32(len(data)) != certsLen+7 {
		return false
	}

	numCerts := 0
	d := data[7:]
	for certsLen > 0 {
		if len(d) < 4 {
			return false
		}
		certLen := uint32(d[0])<<16 | uint32(d[1])<<8 | uint32(d[2])
		if uint32(len(d)) < 3+certLen {
			return false
		}
		d = d[3+certLen:]
		certsLen -= 3 + certLen
		numCerts++
	}

	m.certificates = make([][]byte, numCerts)
	d = data[7:]
	for i := 0; i < numCerts; i++ {
		certLen := uint32(d[0])<<16 | uint32(d[1])<<8 | uint32(d[2])
		m.certificates[i] = d[3 : 3+certLen]
		d = d[3+certLen:]
	}

	return true
}

type certificateMsgTLS13 struct {
	certificate  Certificate
	ocspStapling bool
	scts         bool
}

func (m *certificateMsgTLS13) marshal() ([]byte, error) {
	var b cryptobyte.Builder
	b.AddUint8(typeCertificate)
	b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
		b.AddUint8(0) // certificate_request_context

		certificate := m.certificate
		if !m.ocspStapling {
			certificate.OCSPStaple = nil
		}
		if !m.scts {
			certificate.SignedCertificateTimestamps = nil
		}
		marshalCertificate(b, certificate)
	})

	return b.Bytes()
}

func marshalCertificate(b *cryptobyte.Builder, certificate Certificate) {
	b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
		for i, cert := range certificate.Certificate {
			b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
				b.AddBytes(cert)
			})
			b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
				if i > 0 {
					// This library only supports OCSP and SCT for leaf certificates.
					return
				}
				if certificate.OCSPStaple != nil {
					b.AddUint16(extensionStatusRequest)
					b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
						b.AddUint8(statusTypeOCSP)
						b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
							b.AddBytes(certificate.OCSPStaple)
						})
					})
				}
				if certificate.SignedCertificateTimestamps != nil {
					b.AddUint16(extensionSCT)
					b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
						b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
							for _, sct := range certificate.SignedCertificateTimestamps {
								b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
									b.AddBytes(sct)
								})
							}
						})
					})
				}
			})
		}
	})
}

func (m *certificateMsgTLS13) unmarshal(data []byte) bool {
	*m = certificateMsgTLS13{}
	s := cryptobyte.String(data)

	var context cryptobyte.String
	if !s.Skip(4) || // message type and uint24 length field
		!s.ReadUint8LengthPrefixed(&context) || !context.Empty() ||
		!unmarshalCertificate(&s, &m.certificate) ||
		!s.Empty() {
		return false
	}

	m.scts = m.certificate.SignedCertificateTimestamps != nil
	m.ocspStapling = m.certificate.OCSPStaple != nil

	return true
}

func unmarshalCertificate(s *cryptobyte.String, certificate *Certificate) bool {
	var certList cryptobyte.String
	if !s.ReadUint24LengthPrefixed(&certList) {
		return false
	}
	for !certList.Empty() {
		var cert []byte
		var extensions cryptobyte.String
		if !readUint24LengthPrefixed(&certList, &cert) ||
			!certList.ReadUint16LengthPrefixed(&extensions) {
			return false
		}
		certificate.Certificate = append(certificate.Certificate, cert)
		for !extensions.Empty() {
			var extension uint16
			var extData cryptobyte.String
			if !extensions.ReadUint16(&extension) ||
				!extensions.ReadUint16LengthPrefixed(&extData) {
				return false
			}
			if len(certificate.Certificate) > 1 {
				// This library only supports OCSP and SCT for leaf certificates.
				continue
			}

			switch extension {
			case extensionStatusRequest:
				var statusType uint8
				if !extData.ReadUint8(&statusType) || statusType != statusTypeOCSP ||
					!readUint24LengthPrefixed(&extData, &certificate.OCSPStaple) ||
					len(certificate.OCSPStaple) == 0 {
					return false
				}
			case extensionSCT:
				var sctList cryptobyte.String
				if !extData.ReadUint16LengthPrefixed(&sctList) || sctList.Empty() {
					return false
				}
				for !sctList.Empty() {
					var sct []byte
					if !readUint16LengthPrefixed(&sctList, &sct) ||
						len(sct) == 0 {
						return false
					}
					certificate.SignedCertificateTimestamps = append(
						certificate.SignedCertificateTimestamps, sct)
				}
			default:
				// Ignore unknown extensions.
				continue
			}

			if !extData.Empty() {
				return false
			}
		}
	}
	return true
}

type serverKeyExchangeMsg struct {
	key []byte
}

func (m *serverKeyExchangeMsg) marshal() ([]byte, error) {
	length := len(m.key)
	x := make([]byte, length+4)
	x[0] = typeServerKeyExchange
	x[1] = uint8(length >> 16)
	x[2] = uint8(length >> 8)
	x[3] = uint8(length)
	copy(x[4:], m.key)

	return x, nil
}

func (m *serverKeyExchangeMsg) unmarshal(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	m.key = data[4:]
	return true
}

type certificateStatusMsg struct {
	response []byte
}

func (m *certificateStatusMsg) marshal() ([]byte, error) {
	var b cryptobyte.Builder
	b.AddUint8(typeCertificateStatus)
	b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
		b.AddUint8(statusTypeOCSP)
		b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
			b.AddBytes(m.response)
		})
	})

	return b.Bytes()
}

func (m *certificateStatusMsg) unmarshal(data []byte) bool {
	s := cryptobyte.String(data)

	var statusType uint8
	if !s.Skip(4) || // message type and uint24 length field
		!s.ReadUint8(&statusType) || statusType != statusTypeOCSP ||
		!readUint24LengthPrefixed(&s, &m.response) ||
		len(m.response) == 0 || !s.Empty() {
		return false
	}
	return true
}

type serverHelloDoneMsg struct{}

func (m *serverHelloDoneMsg) marshal() ([]byte, error) {
	x := make([]byte, 4)
	x[0] = typeServerHelloDone
	return x, nil
}

func (m *serverHelloDoneMsg) unmarshal(data []byte) bool {
	return len(data) == 4
}

type clientKeyExchangeMsg struct {
	ciphertext []byte
}

func (m *clientKeyExchangeMsg) marshal() ([]byte, error) {
	length := len(m.ciphertext)
	x := make([]byte, length+4)
	x[0] = typeClientKeyExchange
	x[1] = uint8(length >> 16)
	x[2] = uint8(length >> 8)
	x[3] = uint8(length)
	copy(x[4:], m.ciphertext)

	return x, nil
}

func (m *clientKeyExchangeMsg) unmarshal(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	l := int(data[1])<<16 | int(data[2])<<8 | int(data[3])
	if l != len(data)-4 {
		return false
	}
	m.ciphertext = data[4:]
	return true
}

type finishedMsg struct {
	verifyData []byte
}

func (m *finishedMsg) marshal() ([]byte, error) {
	var b cryptobyte.Builder
	b.AddUint8(typeFinished)
	b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
		b.AddBytes(m.verifyData)
	})

	return b.Bytes()
}

func (m *finishedMsg) unmarshal(data []byte) bool {
	s := cryptobyte.String(data)
	return s.Skip(1) &&
		readUint24LengthPrefixed(&s, &m.verifyData) &&
		s.Empty()
}

type certificateRequestMsg struct {
	// hasSignatureAlgorithm indicates whether this message includes a list of
	// supported signature algorithms. This change was introduced with TLS 1.2.
	hasSignatureAlgorithm bool

	certificateTypes             []byte
	supportedSignatureAlgorithms []SignatureScheme
	certificateAuthorities       [][]byte
}

func (m *certificateRequestMsg) marshal() ([]byte, error) {
	// See RFC 4346, Section 7.4.4.
	length := 1 + len(m.certificateTypes) + 2
	casLength := 0
	for _, ca := range m.certificateAuthorities {
		casLength += 2 + len(ca)
	}
	length += casLength

	if m.hasSignatureAlgorithm {
		length += 2 + 2*len(m.supportedSignatureAlgorithms)
	}

	x := make([]byte, 4+length)
	x[0] = typeCertificateRequest
	x[1] = uint8(length >> 16)
	x[2] = uint8(length >> 8)
	x[3] = uint8(length)

	x[4] = uint8(len(m.certificateTypes))

	copy(x[5:], m.certificateTypes)
	y := x[5+len(m.certificateTypes):]

	if m.hasSignatureAlgorithm {
		n := len(m.supportedSignatureAlgorithms) * 2
		y[0] = uint8(n >> 8)
		y[1] = uint8(n)
		y = y[2:]
		for _, sigAlgo := range m.supportedSignatureAlgorithms {
			y[0] = uint8(sigAlgo >> 8)
			y[1] = uint8(sigAlgo)
			y = y[2:]
		}
	}

	y[0] = uint8(casLength >> 8)
	y[1] = uint8(casLength)
	y = y[2:]
	for _, ca := range m.certificateAuthorities {
		y[0] = uint8(len(ca) >> 8)
		y[1] = uint8(len(ca))
		y = y[2:]
		copy(y, ca)
		y = y[len(ca):]
	}

	return x, nil
}

func (m *certificateRequestMsg) unmarshal(data []byte) bool {
	if len(data) < 5 {
		return false
	}

	length := uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
	if uint32(len(data))-4 != length {
		return false
	}

	numCertTypes := int(data[4])
	data = data[5:]
	if numCertTypes == 0 || len(data) <= numCertTypes {
		return false
	}

	m.certificateTypes = make([]byte, numCertTypes)
	if copy(m.certificateTypes, data) != numCertTypes {
		return false
	}

	data = data[numCertTypes:]

	if m.hasSignatureAlgorithm {
		if len(data) < 2 {
			return false
		}
		sigAndHashLen := uint16(data[0])<<8 | uint16(data[1])
		data = data[2:]
		if sigAndHashLen&1 != 0 || sigAndHashLen == 0 {
			return false
		}
		if len(data) < int(sigAndHashLen) {
			return false
		}
		numSigAlgos := sigAndHashLen / 2
		m.supportedSignatureAlgorithms = make([]SignatureScheme, numSigAlgos)
		for i := range m.supportedSignatureAlgorithms {
			m.supportedSignatureAlgorithms[i] = SignatureScheme(data[0])<<8 | SignatureScheme(data[1])
			data = data[2:]
		}
	}

	if len(data) < 2 {
		return false
	}
	casLength := uint16(data[0])<<8 | uint16(data[1])
	data = data[2:]
	if len(data) < int(casLength) {
		return false
	}
	cas := make([]byte, casLength)
	copy(cas, data)
	data = data[casLength:]

	m.certificateAuthorities = nil
	for len(cas) > 0 {
		if len(cas) < 2 {
			return false
		}
		caLen := uint16(cas[0])<<8 | uint16(cas[1])
		cas = cas[2:]

		if len(cas) < int(caLen) {
			return false
		}

		m.certificateAuthorities = append(m.certificateAuthorities, cas[:caLen])
		cas = cas[caLen:]
	}

	return len(data) == 0
}

type certificateVerifyMsg struct {
	hasSignatureAlgorithm bool // format change introduced in TLS 1.2
	signatureAlgorithm    SignatureScheme
	signature             []byte
}

func (m *certificateVerifyMsg) marshal() ([]byte, error) {
	var b cryptobyte.Builder
	b.AddUint8(typeCertificateVerify)
	b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
		if m.hasSignatureAlgorithm {
			b.AddUint16(uint16(m.signatureAlgorithm))
		}
		b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
			b.AddBytes(m.signature)
		})
	})

	return b.Bytes()
}

func (m *certificateVerifyMsg) unmarshal(data []byte) bool {
	s := cryptobyte.String(data)

	if !s.Skip(4) { // message type and uint24 length field
		return false
	}
	if m.hasSignatureAlgorithm {
		if !s.ReadUint16((*uint16)(&m.signatureAlgorithm)) {
			return false
		}
	}
	return readUint16LengthPrefixed(&s, &m.signature) && s.Empty()
}

type newSessionTicketMsg struct {
	ticket []byte
}

func (m *newSessionTicketMsg) marshal() ([]byte, error) {
	// See RFC 5077, Section 3.3.
	ticketLen := len(m.ticket)
	length := 2 + 4 + ticketLen
	x := make([]byte, 4+length)
	x[0] = typeNewSessionTicket
	x[1] = uint8(length >> 16)
	x[2] = uint8(length >> 8)
	x[3] = uint8(length)
	x[8] = uint8(ticketLen >> 8)
	x[9] = uint8(ticketLen)
	copy(x[10:], m.ticket)

	return x, nil
}

func (m *newSessionTicketMsg) unmarshal(data []byte) bool {
	if len(data) < 10 {
		return false
	}

	length := uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
	if uint32(len(data))-4 != length {
		return false
	}

	ticketLen := int(data[8])<<8 + int(data[9])
	if len(data)-10 != ticketLen {
		return false
	}

	m.ticket = data[10:]

	return true
}

type helloRequestMsg struct {
}

func (*helloRequestMsg) marshal() ([]byte, error) {
	return []byte{typeHelloRequest, 0, 0, 0}, nil
}

func (*helloRequestMsg) unmarshal(data []byte) bool {
	return len(data) == 4
}

type transcriptHash interface {
	Write([]byte) (int, error)
}

// transcriptMsg is a helper used to hash messages which are not hashed when
// they are read from, or written to, the wire. This is typically the case for
// messages which are either not sent, or need to be hashed out of order from
// when they are read/written.
//
// For most messages, the message is marshalled using their marshal method,
// since their wire representation is idempotent. For clientHelloMsg and
// serverHelloMsg, we store the original wire representation of the message and
// use that for hashing, since unmarshal/marshal are not idempotent due to
// extension ordering and other malleable fields, which may cause differences
// between what was received and what we marshal.
func transcriptMsg(msg handshakeMessage, h transcriptHash) error {
	if msgWithOrig, ok := msg.(handshakeMessageWithOriginalBytes); ok {
		if orig := msgWithOrig.originalBytes(); orig != nil {
			h.Write(msgWithOrig.originalBytes())
			return nil
		}
	}

	data, err := msg.marshal()
	if err != nil {
		return err
	}
	h.Write(data)
	return nil
}

```

// === FILE: references/go/src/crypto/tls/handshake_server.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tls

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/mldsa"
	"crypto/rsa"
	"crypto/subtle"
	"crypto/tls/internal/fips140tls"
	"crypto/x509"
	"errors"
	"fmt"
	"hash"
	"io"
	"time"
)

// serverHandshakeState contains details of a server handshake in progress.
// It's discarded once the handshake has completed.
type serverHandshakeState struct {
	c            *Conn
	ctx          context.Context
	clientHello  *clientHelloMsg
	hello        *serverHelloMsg
	suite        *cipherSuite
	ecdheOk      bool
	ecSignOk     bool
	rsaDecryptOk bool
	rsaSignOk    bool
	sessionState *SessionState
	finishedHash finishedHash
	masterSecret []byte
	cert         *Certificate
}

// serverHandshake performs a TLS handshake as a server.
func (c *Conn) serverHandshake(ctx context.Context) error {
	clientHello, ech, err := c.readClientHello(ctx)
	if err != nil {
		return err
	}

	if c.vers == VersionTLS13 {
		hs := serverHandshakeStateTLS13{
			c:           c,
			ctx:         ctx,
			clientHello: clientHello,
			echContext:  ech,
		}
		return hs.handshake()
	}

	hs := serverHandshakeState{
		c:           c,
		ctx:         ctx,
		clientHello: clientHello,
	}
	return hs.handshake()
}

func (hs *serverHandshakeState) handshake() error {
	c := hs.c

	if err := hs.processClientHello(); err != nil {
		return err
	}

	// For an overview of TLS handshaking, see RFC 5246, Section 7.3.
	c.buffering = true
	if err := hs.checkForResumption(); err != nil {
		return err
	}
	if hs.sessionState != nil {
		// The client has included a session ticket and so we do an abbreviated handshake.
		if err := hs.doResumeHandshake(); err != nil {
			return err
		}
		if err := hs.establishKeys(); err != nil {
			return err
		}
		if err := hs.sendSessionTicket(); err != nil {
			return err
		}
		if err := hs.sendFinished(c.serverFinished[:]); err != nil {
			return err
		}
		if _, err := c.flush(); err != nil {
			return err
		}
		c.clientFinishedIsFirst = false
		if err := hs.readFinished(nil); err != nil {
			return err
		}
	} else {
		// The client didn't include a session ticket, or it wasn't
		// valid so we do a full handshake.
		if err := hs.pickCipherSuite(); err != nil {
			return err
		}
		if err := hs.doFullHandshake(); err != nil {
			return err
		}
		if err := hs.establishKeys(); err != nil {
			return err
		}
		if err := hs.readFinished(c.clientFinished[:]); err != nil {
			return err
		}
		c.clientFinishedIsFirst = true
		c.buffering = true
		if err := hs.sendSessionTicket(); err != nil {
			return err
		}
		if err := hs.sendFinished(nil); err != nil {
			return err
		}
		if _, err := c.flush(); err != nil {
			return err
		}
	}

	c.ekm = ekmFromMasterSecret(c.vers, hs.suite, hs.masterSecret, hs.clientHello.random, hs.hello.random)
	c.isHandshakeComplete.Store(true)

	return nil
}

// readClientHello reads a ClientHello message and selects the protocol version.
func (c *Conn) readClientHello(ctx context.Context) (*clientHelloMsg, *echServerContext, error) {
	// clientHelloMsg is included in the transcript, but we haven't initialized
	// it yet. The respective handshake functions will record it themselves.
	msg, err := c.readHandshake(nil)
	if err != nil {
		return nil, nil, err
	}
	clientHello, ok := msg.(*clientHelloMsg)
	if !ok {
		c.sendAlert(alertUnexpectedMessage)
		return nil, nil, unexpectedMessageError(clientHello, msg)
	}

	// ECH processing has to be done before we do any other negotiation based on
	// the contents of the client hello, since we may swap it out completely.
	var ech *echServerContext
	if len(clientHello.encryptedClientHello) != 0 {
		echKeys := c.config.EncryptedClientHelloKeys
		if c.config.GetEncryptedClientHelloKeys != nil {
			echKeys, err = c.config.GetEncryptedClientHelloKeys(clientHelloInfo(ctx, c, clientHello))
			if err != nil {
				c.sendAlert(alertInternalError)
				return nil, nil, err
			}
		}
		clientHello, ech, err = c.processECHClientHello(clientHello, echKeys)
		if err != nil {
			return nil, nil, err
		}
	}

	var configForClient *Config
	originalConfig := c.config
	if c.config.GetConfigForClient != nil {
		chi := clientHelloInfo(ctx, c, clientHello)
		if configForClient, err = c.config.GetConfigForClient(chi); err != nil {
			c.sendAlert(alertInternalError)
			return nil, nil, err
		} else if configForClient != nil {
			c.config = configForClient
		}
	}
	c.ticketKeys = originalConfig.ticketKeys(configForClient)

	clientVersions := clientHello.supportedVersions
	if clientHello.vers >= VersionTLS13 && len(clientVersions) == 0 {
		// RFC 8446 4.2.1 indicates when the supported_versions extension is not sent,
		// compatible servers MUST negotiate TLS 1.2 or earlier if supported, even
		// if the client legacy version is TLS 1.3 or later.
		//
		// Since we reject empty extensionSupportedVersions in the client hello unmarshal
		// finding the supportedVersions empty indicates the extension was not present.
		clientVersions = supportedVersionsFromMax(VersionTLS12)
	} else if len(clientVersions) == 0 {
		clientVersions = supportedVersionsFromMax(clientHello.vers)
	}
	c.vers, ok = c.config.mutualVersion(roleServer, c.quic != nil, clientVersions)
	if !ok {
		c.sendAlert(alertProtocolVersion)
		return nil, nil, fmt.Errorf("tls: client offered only unsupported versions: %x", clientVersions)
	}
	c.haveVers = true
	c.in.version = c.vers
	c.out.version = c.vers

	// This check reflects some odd specification implied behavior. Client-facing servers
	// are supposed to reject hellos with outer ECH and inner ECH that offers 1.2, but
	// backend servers are allowed to accept hellos with inner ECH that offer 1.2, since
	// they cannot expect client-facing servers to behave properly. Since we act as both
	// a client-facing and backend server, we only enforce 1.3 being negotiated if we
	// saw a hello with outer ECH first. The spec probably should've made this an error,
	// but it didn't, and this matches the boringssl behavior.
	if c.vers != VersionTLS13 && (ech != nil && !ech.inner) {
		c.sendAlert(alertIllegalParameter)
		return nil, nil, errors.New("tls: Encrypted Client Hello cannot be used pre-TLS 1.3")
	}

	return clientHello, ech, nil
}

func (hs *serverHandshakeState) processClientHello() error {
	c := hs.c

	hs.hello = new(serverHelloMsg)
	hs.hello.vers = c.vers

	foundCompression := false
	// We only support null compression, so check that the client offered it.
	for _, compression := range hs.clientHello.compressionMethods {
		if compression == compressionNone {
			foundCompression = true
			break
		}
	}

	if !foundCompression {
		c.sendAlert(alertIllegalParameter)
		return errors.New("tls: client does not support uncompressed connections")
	}

	hs.hello.random = make([]byte, 32)
	serverRandom := hs.hello.random
	// Downgrade protection canaries. See RFC 8446, Section 4.1.3.
	maxVers := c.config.maxSupportedVersion(roleServer, c.quic != nil)
	if maxVers >= VersionTLS12 && c.vers < maxVers || testingOnlyForceDowngradeCanary {
		if c.vers == VersionTLS12 {
			copy(serverRandom[24:], downgradeCanaryTLS12)
		} else {
			copy(serverRandom[24:], downgradeCanaryTLS11)
		}
		serverRandom = serverRandom[:24]
	}
	_, err := io.ReadFull(c.config.rand(), serverRandom)
	if err != nil {
		c.sendAlert(alertInternalError)
		return err
	}

	if len(hs.clientHello.secureRenegotiation) != 0 {
		c.sendAlert(alertHandshakeFailure)
		return errors.New("tls: initial handshake had non-empty renegotiation extension")
	}

	hs.hello.extendedMasterSecret = hs.clientHello.extendedMasterSecret
	hs.hello.secureRenegotiationSupported = hs.clientHello.secureRenegotiationSupported
	hs.hello.compressionMethod = compressionNone
	if len(hs.clientHello.serverName) > 0 {
		c.serverName = hs.clientHello.serverName
	}

	selectedProto, err := negotiateALPN(c.config.NextProtos, hs.clientHello.alpnProtocols, false)
	if err != nil {
		c.sendAlert(alertNoApplicationProtocol)
		return err
	}
	hs.hello.alpnProtocol = selectedProto
	c.clientProtocol = selectedProto

	hs.cert, err = c.config.getCertificate(clientHelloInfo(hs.ctx, c, hs.clientHello))
	if err != nil {
		if err == errNoCertificates {
			c.sendAlert(alertUnrecognizedName)
		} else {
			c.sendAlert(alertInternalError)
		}
		return err
	}

	if hs.clientHello.scts {
		hs.hello.scts = hs.cert.SignedCertificateTimestamps
	}

	hs.ecdheOk, err = supportsECDHE(c.config, c.vers, hs.clientHello.supportedCurves, hs.clientHello.supportedPoints)
	if err != nil {
		c.sendAlert(alertMissingExtension)
		return err
	}

	if hs.ecdheOk && len(hs.clientHello.supportedPoints) > 0 {
		// Although omitting the ec_point_formats extension is permitted, some
		// old OpenSSL version will refuse to handshake if not present.
		//
		// Per RFC 4492, section 5.1.2, implementations MUST support the
		// uncompressed point format. See golang.org/issue/31943.
		hs.hello.supportedPoints = []uint8{pointFormatUncompressed}
	}

	if priv, ok := hs.cert.PrivateKey.(crypto.Signer); ok {
		switch priv.Public().(type) {
		case *ecdsa.PublicKey:
			hs.ecSignOk = true
		case ed25519.PublicKey:
			hs.ecSignOk = true
		case *rsa.PublicKey:
			hs.rsaSignOk = true
		case *mldsa.PublicKey:
			// ML-DSA can only be used with TLS 1.3.
			c.sendAlert(alertInternalError)
			return fmt.Errorf("tls: ML-DSA certificates require TLS 1.3, but client negotiated %s",
				VersionName(c.vers))
		default:
			c.sendAlert(alertInternalError)
			return fmt.Errorf("tls: unsupported signing key type (%T)", priv.Public())
		}
	}
	if priv, ok := hs.cert.PrivateKey.(crypto.Decrypter); ok {
		switch priv.Public().(type) {
		case *rsa.PublicKey:
			hs.rsaDecryptOk = true
		default:
			c.sendAlert(alertInternalError)
			return fmt.Errorf("tls: unsupported decryption key type (%T)", priv.Public())
		}
	}

	return nil
}

// negotiateALPN picks a shared ALPN protocol that both sides support in server
// preference order. If ALPN is not configured or the peer doesn't support it,
// it returns "" and no error.
func negotiateALPN(serverProtos, clientProtos []string, quic bool) (string, error) {
	if len(serverProtos) == 0 || len(clientProtos) == 0 {
		if quic && len(serverProtos) != 0 {
			// RFC 9001, Section 8.1
			return "", fmt.Errorf("tls: client did not request an application protocol")
		}
		return "", nil
	}
	var http11fallback bool
	for _, s := range serverProtos {
		for _, c := range clientProtos {
			if s == c {
				return s, nil
			}
			if s == "h2" && c == "http/1.1" {
				http11fallback = true
			}
		}
	}
	// As a special case, let http/1.1 clients connect to h2 servers as if they
	// didn't support ALPN. We used not to enforce protocol overlap, so over
	// time a number of HTTP servers were configured with only "h2", but
	// expected to accept connections from "http/1.1" clients. See Issue 46310.
	if http11fallback {
		return "", nil
	}
	return "", fmt.Errorf("tls: client requested unsupported application protocols (%q)", clientProtos)
}

// supportsECDHE returns whether ECDHE key exchanges can be used with this
// pre-TLS 1.3 client.
func supportsECDHE(c *Config, version uint16, supportedCurves []CurveID, supportedPoints []uint8) (bool, error) {
	supportsCurve := false
	for _, curve := range supportedCurves {
		if c.supportsCurve(version, curve) {
			supportsCurve = true
			break
		}
	}

	supportsPointFormat := false
	offeredNonCompressedFormat := false
	for _, pointFormat := range supportedPoints {
		if pointFormat == pointFormatUncompressed {
			supportsPointFormat = true
		} else {
			offeredNonCompressedFormat = true
		}
	}
	// Per RFC 8422, Section 5.1.2, if the Supported Point Formats extension is
	// missing, uncompressed points are supported. If supportedPoints is empty,
	// the extension must be missing, as an empty extension body is rejected by
	// the parser. See https://go.dev/issue/49126.
	if len(supportedPoints) == 0 {
		supportsPointFormat = true
	} else if offeredNonCompressedFormat && !supportsPointFormat {
		return false, errors.New("tls: client offered only incompatible point formats")
	}

	return supportsCurve && supportsPointFormat, nil
}

func (hs *serverHandshakeState) pickCipherSuite() error {
	c := hs.c

	preferenceList := c.config.cipherSuites(isAESGCMPreferred(hs.clientHello.cipherSuites))

	hs.suite = selectCipherSuite(preferenceList, hs.clientHello.cipherSuites, hs.cipherSuiteOk)
	if hs.suite == nil {
		c.sendAlert(alertHandshakeFailure)
		return fmt.Errorf("tls: no cipher suite supported by both client and server; client offered: %x",
			hs.clientHello.cipherSuites)
	}
	c.cipherSuite = hs.suite.id

	for _, id := range hs.clientHello.cipherSuites {
		if id == TLS_FALLBACK_SCSV {
			// The client is doing a fallback connection. See RFC 7507.
			if hs.clientHello.vers < c.config.maxSupportedVersion(roleServer, c.quic != nil) {
				c.sendAlert(alertInappropriateFallback)
				return errors.New("tls: client using inappropriate protocol fallback")
			}
			break
		}
	}

	return nil
}

func (hs *serverHandshakeState) cipherSuiteOk(c *cipherSuite) bool {
	if c.flags&suiteECDHE != 0 {
		if !hs.ecdheOk {
			return false
		}
		if c.flags&suiteECSign != 0 {
			if !hs.ecSignOk {
				return false
			}
		} else if !hs.rsaSignOk {
			return false
		}
	} else if !hs.rsaDecryptOk {
		return false
	}
	if hs.c.vers < VersionTLS12 && c.flags&suiteTLS12 != 0 {
		return false
	}
	return true
}

// checkForResumption reports whether we should perform resumption on this connection.
func (hs *serverHandshakeState) checkForResumption() error {
	c := hs.c

	if c.config.SessionTicketsDisabled {
		return nil
	}

	var sessionState *SessionState
	if c.config.UnwrapSession != nil {
		ss, err := c.config.UnwrapSession(hs.clientHello.sessionTicket, c.connectionStateLocked())
		if err != nil {
			return err
		}
		if ss == nil {
			return nil
		}
		sessionState = ss
	} else {
		plaintext := c.config.decryptTicket(hs.clientHello.sessionTicket, c.ticketKeys)
		if plaintext == nil {
			return nil
		}
		ss, err := ParseSessionState(plaintext)
		if err != nil {
			return nil
		}
		sessionState = ss
	}

	// TLS 1.2 tickets don't natively have a lifetime, but we want to avoid
	// re-wrapping the same master secret in different tickets over and over for
	// too long, weakening forward secrecy.
	createdAt := time.Unix(int64(sessionState.createdAt), 0)
	if c.config.time().Sub(createdAt) > maxSessionTicketLifetime {
		return nil
	}

	// Never resume a session for a different TLS version.
	if c.vers != sessionState.version {
		return nil
	}

	cipherSuiteOk := false
	// Check that the client is still offering the ciphersuite in the session.
	for _, id := range hs.clientHello.cipherSuites {
		if id == sessionState.cipherSuite {
			cipherSuiteOk = true
			break
		}
	}
	if !cipherSuiteOk {
		return nil
	}

	// Check that we also support the ciphersuite from the session.
	suite := selectCipherSuite([]uint16{sessionState.cipherSuite},
		c.config.supportedCipherSuites(), hs.cipherSuiteOk)
	if suite == nil {
		return nil
	}

	sessionHasClientCerts := len(sessionState.peerCertificates) != 0
	needClientCerts := requiresClientCert(c.config.ClientAuth)
	if needClientCerts && !sessionHasClientCerts {
		return nil
	}
	if sessionHasClientCerts && c.config.ClientAuth == NoClientCert {
		return nil
	}
	if sessionHasClientCerts && c.config.time().After(sessionState.peerCertificates[0].NotAfter) {
		return nil
	}
	opts := x509.VerifyOptions{
		CurrentTime: c.config.time(),
		Roots:       c.config.ClientCAs,
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	if sessionHasClientCerts && c.config.ClientAuth >= VerifyClientCertIfGiven &&
		!anyValidVerifiedChain(sessionState.verifiedChains, opts) {
		return nil
	}

	// RFC 7627, Section 5.3
	if !sessionState.extMasterSecret && hs.clientHello.extendedMasterSecret {
		return nil
	}
	if sessionState.extMasterSecret && !hs.clientHello.extendedMasterSecret {
		// Aborting is somewhat harsh, but it's a MUST and it would indicate a
		// weird downgrade in client capabilities.
		return errors.New("tls: session supported extended_master_secret but client does not")
	}
	if !sessionState.extMasterSecret && fips140tls.Required() {
		// FIPS 140-3 requires the use of Extended Master Secret.
		return nil
	}

	c.peerCertificates = sessionState.peerCertificates
	c.ocspResponse = sessionState.ocspResponse
	c.scts = sessionState.scts
	c.verifiedChains = sessionState.verifiedChains
	c.extMasterSecret = sessionState.extMasterSecret
	hs.sessionState = sessionState
	hs.suite = suite
	c.curveID = sessionState.curveID
	c.didResume = true
	return nil
}

func (hs *serverHandshakeState) doResumeHandshake() error {
	c := hs.c

	hs.hello.cipherSuite = hs.suite.id
	c.cipherSuite = hs.suite.id
	// We echo the client's session ID in the ServerHello to let it know
	// that we're doing a resumption.
	hs.hello.sessionId = hs.clientHello.sessionId
	// We always send a new session ticket, even if it wraps the same master
	// secret and it's potentially encrypted with the same key, to help the
	// client avoid cross-connection tracking from a network observer.
	hs.hello.ticketSupported = true
	hs.finishedHash = newFinishedHash(c.vers, hs.suite)
	hs.finishedHash.discardHandshakeBuffer()
	if err := transcriptMsg(hs.clientHello, &hs.finishedHash); err != nil {
		return err
	}
	if _, err := hs.c.writeHandshakeRecord(hs.hello, &hs.finishedHash); err != nil {
		return err
	}

	if c.config.VerifyConnection != nil {
		if err := c.config.VerifyConnection(c.connectionStateLocked()); err != nil {
			c.sendAlert(alertBadCertificate)
			return err
		}
	}

	hs.masterSecret = hs.sessionState.secret

	return nil
}

func (hs *serverHandshakeState) doFullHandshake() error {
	c := hs.c

	if hs.clientHello.ocspStapling && len(hs.cert.OCSPStaple) > 0 {
		hs.hello.ocspStapling = true
	}

	if hs.clientHello.serverName != "" {
		hs.hello.serverNameAck = true
	}

	hs.hello.ticketSupported = hs.clientHello.ticketSupported && !c.config.SessionTicketsDisabled
	hs.hello.cipherSuite = hs.suite.id

	hs.finishedHash = newFinishedHash(hs.c.vers, hs.suite)
	if c.config.ClientAuth == NoClientCert {
		// No need to keep a full record of the handshake if client
		// certificates won't be used.
		hs.finishedHash.discardHandshakeBuffer()
	}
	if err := transcriptMsg(hs.clientHello, &hs.finishedHash); err != nil {
		return err
	}
	if _, err := hs.c.writeHandshakeRecord(hs.hello, &hs.finishedHash); err != nil {
		return err
	}

	certMsg := new(certificateMsg)
	certMsg.certificates = hs.cert.Certificate
	// Set localCertificate here, rather than at certificate selection time, so
	// that it is only populated when a certificate is actually presented to the
	// peer, and not on resumed connections.
	c.localCertificate = hs.cert.Certificate
	if _, err := hs.c.writeHandshakeRecord(certMsg, &hs.finishedHash); err != nil {
		return err
	}

	if hs.hello.ocspStapling {
		certStatus := new(certificateStatusMsg)
		certStatus.response = hs.cert.OCSPStaple
		if _, err := hs.c.writeHandshakeRecord(certStatus, &hs.finishedHash); err != nil {
			return err
		}
	}

	keyAgreement := hs.suite.ka(c.vers)
	skx, err := keyAgreement.generateServerKeyExchange(c.config, hs.cert, hs.clientHello, hs.hello)
	if err != nil {
		c.sendAlert(alertHandshakeFailure)
		return err
	}
	if skx != nil {
		if keyAgreement, ok := keyAgreement.(*ecdheKeyAgreement); ok {
			c.curveID = keyAgreement.curveID
			c.peerSigAlg = keyAgreement.signatureAlgorithm
		}
		if _, err := hs.c.writeHandshakeRecord(skx, &hs.finishedHash); err != nil {
			return err
		}
	}

	var certReq *certificateRequestMsg
	if c.config.ClientAuth >= RequestClientCert {
		// Request a client certificate
		certReq = new(certificateRequestMsg)
		certReq.certificateTypes = []byte{
			byte(certTypeRSASign),
			byte(certTypeECDSASign),
		}
		if c.vers >= VersionTLS12 {
			certReq.hasSignatureAlgorithm = true
			certReq.supportedSignatureAlgorithms = supportedSignatureAlgorithms(c.vers, c.vers)
		}

		// An empty list of certificateAuthorities signals to
		// the client that it may send any certificate in response
		// to our request. When we know the CAs we trust, then
		// we can send them down, so that the client can choose
		// an appropriate certificate to give to us.
		if c.config.ClientCAs != nil {
			certReq.certificateAuthorities = c.config.ClientCAs.Subjects()
		}
		if _, err := hs.c.writeHandshakeRecord(certReq, &hs.finishedHash); err != nil {
			return err
		}
	}

	helloDone := new(serverHelloDoneMsg)
	if _, err := hs.c.writeHandshakeRecord(helloDone, &hs.finishedHash); err != nil {
		return err
	}

	if _, err := c.flush(); err != nil {
		return err
	}

	var pub crypto.PublicKey // public key for client auth, if any

	msg, err := c.readHandshake(&hs.finishedHash)
	if err != nil {
		return err
	}

	// If we requested a client certificate, then the client must send a
	// certificate message, even if it's empty.
	if c.config.ClientAuth >= RequestClientCert {
		certMsg, ok := msg.(*certificateMsg)
		if !ok {
			c.sendAlert(alertUnexpectedMessage)
			return unexpectedMessageError(certMsg, msg)
		}

		if err := c.processCertsFromClient(Certificate{
			Certificate: certMsg.certificates,
		}); err != nil {
			return err
		}
		if len(certMsg.certificates) != 0 {
			pub = c.peerCertificates[0].PublicKey
		}

		msg, err = c.readHandshake(&hs.finishedHash)
		if err != nil {
			return err
		}
	}
	if c.config.VerifyConnection != nil {
		if err := c.config.VerifyConnection(c.connectionStateLocked()); err != nil {
			c.sendAlert(alertBadCertificate)
			return err
		}
	}

	// Get client key exchange
	ckx, ok := msg.(*clientKeyExchangeMsg)
	if !ok {
		c.sendAlert(alertUnexpectedMessage)
		return unexpectedMessageError(ckx, msg)
	}

	preMasterSecret, err := keyAgreement.processClientKeyExchange(c.config, hs.cert, ckx, c.vers)
	if err != nil {
		c.sendAlert(alertIllegalParameter)
		return err
	}
	if hs.hello.extendedMasterSecret {
		c.extMasterSecret = true
		hs.masterSecret = extMasterFromPreMasterSecret(c.vers, hs.suite, preMasterSecret,
			hs.finishedHash.Sum())
	} else {
		if fips140tls.Required() {
			c.sendAlert(alertHandshakeFailure)
			return errors.New("tls: FIPS 140-3 requires the use of Extended Master Secret")
		}
		hs.masterSecret = masterFromPreMasterSecret(c.vers, hs.suite, preMasterSecret,
			hs.clientHello.random, hs.hello.random)
	}
	if err := c.config.writeKeyLog(keyLogLabelTLS12, hs.clientHello.random, hs.masterSecret); err != nil {
		c.sendAlert(alertInternalError)
		return err
	}

	// If we received a client cert in response to our certificate request message,
	// the client will send us a certificateVerifyMsg immediately after the
	// clientKeyExchangeMsg. This message is a digest of all preceding
	// handshake-layer messages that is signed using the private key corresponding
	// to the client's certificate. This allows us to verify that the client is in
	// possession of the private key of the certificate.
	if len(c.peerCertificates) > 0 {
		// certificateVerifyMsg is included in the transcript, but not until
		// after we verify the handshake signature, since the state before
		// this message was sent is used.
		msg, err = c.readHandshake(nil)
		if err != nil {
			return err
		}
		certVerify, ok := msg.(*certificateVerifyMsg)
		if !ok {
			c.sendAlert(alertUnexpectedMessage)
			return unexpectedMessageError(certVerify, msg)
		}

		var sigType uint8
		var sigHash crypto.Hash
		if c.vers >= VersionTLS12 {
			if !isSupportedSignatureAlgorithm(certVerify.signatureAlgorithm, certReq.supportedSignatureAlgorithms) {
				c.sendAlert(alertIllegalParameter)
				return errors.New("tls: client certificate used with invalid signature algorithm")
			}
			sigType, sigHash, err = typeAndHashFromSignatureScheme(certVerify.signatureAlgorithm)
			if err != nil {
				return c.sendAlert(alertInternalError)
			}
			if sigHash == crypto.SHA1 {
				tlssha1.Value() // ensure godebug is initialized
				tlssha1.IncNonDefault()
			}
			if hs.finishedHash.buffer == nil {
				c.sendAlert(alertInternalError)
				return errors.New("tls: internal error: did not keep handshake transcript for TLS 1.2")
			}
			if err := verifyHandshakeSignature(sigType, pub, sigHash, hs.finishedHash.buffer, certVerify.signature); err != nil {
				c.sendAlert(alertDecryptError)
				return errors.New("tls: invalid signature by the client certificate: " + err.Error())
			}
		} else {
			sigType, sigHash, err = legacyTypeAndHashFromPublicKey(pub)
			if err != nil {
				c.sendAlert(alertIllegalParameter)
				return err
			}
			signed := hs.finishedHash.hashForClientCertificate(sigType)
			if err := verifyLegacyHandshakeSignature(sigType, pub, sigHash, signed, certVerify.signature); err != nil {
				c.sendAlert(alertDecryptError)
				return errors.New("tls: invalid signature by the client certificate: " + err.Error())
			}
		}

		c.peerSigAlg = certVerify.signatureAlgorithm

		if err := transcriptMsg(certVerify, &hs.finishedHash); err != nil {
			return err
		}
	}

	hs.finishedHash.discardHandshakeBuffer()

	return nil
}

func (hs *serverHandshakeState) establishKeys() error {
	c := hs.c

	clientMAC, serverMAC, clientKey, serverKey, clientIV, serverIV :=
		keysFromMasterSecret(c.vers, hs.suite, hs.masterSecret, hs.clientHello.random, hs.hello.random, hs.suite.macLen, hs.suite.keyLen, hs.suite.ivLen)

	var clientCipher, serverCipher any
	var clientHash, serverHash hash.Hash

	if hs.suite.aead == nil {
		clientCipher = hs.suite.cipher(clientKey, clientIV, true /* for reading */)
		clientHash = hs.suite.mac(clientMAC)
		serverCipher = hs.suite.cipher(serverKey, serverIV, false /* not for reading */)
		serverHash = hs.suite.mac(serverMAC)
	} else {
		clientCipher = hs.suite.aead(clientKey, clientIV)
		serverCipher = hs.suite.aead(serverKey, serverIV)
	}

	c.in.prepareCipherSpec(c.vers, clientCipher, clientHash)
	c.out.prepareCipherSpec(c.vers, serverCipher, serverHash)

	return nil
}

func (hs *serverHandshakeState) readFinished(out []byte) error {
	c := hs.c

	if err := c.readChangeCipherSpec(); err != nil {
		return err
	}

	// finishedMsg is included in the transcript, but not until after we
	// check the client version, since the state before this message was
	// sent is used during verification.
	msg, err := c.readHandshake(nil)
	if err != nil {
		return err
	}
	clientFinished, ok := msg.(*finishedMsg)
	if !ok {
		c.sendAlert(alertUnexpectedMessage)
		return unexpectedMessageError(clientFinished, msg)
	}

	verify := hs.finishedHash.clientSum(hs.masterSecret)
	if len(verify) != len(clientFinished.verifyData) ||
		subtle.ConstantTimeCompare(verify, clientFinished.verifyData) != 1 {
		c.sendAlert(alertHandshakeFailure)
		return errors.New("tls: client's Finished message is incorrect")
	}

	if err := transcriptMsg(clientFinished, &hs.finishedHash); err != nil {
		return err
	}

	copy(out, verify)
	return nil
}

func (hs *serverHandshakeState) sendSessionTicket() error {
	if !hs.hello.ticketSupported {
		return nil
	}

	c := hs.c
	m := new(newSessionTicketMsg)

	state := c.sessionState()
	state.secret = hs.masterSecret
	if hs.sessionState != nil {
		// If this is re-wrapping an old key, then keep
		// the original time it was created.
		state.createdAt = hs.sessionState.createdAt
	}
	if c.config.WrapSession != nil {
		var err error
		m.ticket, err = c.config.WrapSession(c.connectionStateLocked(), state)
		if err != nil {
			return err
		}
	} else {
		stateBytes, err := state.Bytes()
		if err != nil {
			return err
		}
		m.ticket, err = c.config.encryptTicket(stateBytes, c.ticketKeys)
		if err != nil {
			return err
		}
	}

	if _, err := hs.c.writeHandshakeRecord(m, &hs.finishedHash); err != nil {
		return err
	}

	return nil
}

func (hs *serverHandshakeState) sendFinished(out []byte) error {
	c := hs.c

	if err := c.writeChangeCipherRecord(); err != nil {
		return err
	}

	finished := new(finishedMsg)
	finished.verifyData = hs.finishedHash.serverSum(hs.masterSecret)
	if _, err := hs.c.writeHandshakeRecord(finished, &hs.finishedHash); err != nil {
		return err
	}

	copy(out, finished.verifyData)

	return nil
}

// processCertsFromClient takes a chain of client certificates either from a
// certificateMsg message or a certificateMsgTLS13 message and verifies them.
func (c *Conn) processCertsFromClient(certificate Certificate) error {
	certificates := certificate.Certificate
	certs := make([]*x509.Certificate, len(certificates))
	var err error
	for i, asn1Data := range certificates {
		if certs[i], err = x509.ParseCertificate(asn1Data); err != nil {
			c.sendAlert(alertDecodeError)
			return errors.New("tls: failed to parse client certificate: " + err.Error())
		}
		if certs[i].PublicKeyAlgorithm == x509.RSA {
			n := certs[i].PublicKey.(*rsa.PublicKey).N.BitLen()
			if max, ok := checkKeySize(n); !ok {
				c.sendAlert(alertBadCertificate)
				return fmt.Errorf("tls: client sent certificate containing RSA key larger than %d bits", max)
			}
		}
	}

	if len(certs) == 0 && requiresClientCert(c.config.ClientAuth) {
		if c.vers == VersionTLS13 {
			c.sendAlert(alertCertificateRequired)
		} else {
			c.sendAlert(alertHandshakeFailure)
		}
		return errors.New("tls: client didn't provide a certificate")
	}

	if c.config.ClientAuth >= VerifyClientCertIfGiven && len(certs) > 0 {
		opts := x509.VerifyOptions{
			Roots:         c.config.ClientCAs,
			CurrentTime:   c.config.time(),
			Intermediates: x509.NewCertPool(),
			KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}

		for _, cert := range certs[1:] {
			opts.Intermediates.AddCert(cert)
		}

		chains, err := certs[0].Verify(opts)
		if err != nil {
			if _, ok := errors.AsType[x509.UnknownAuthorityError](err); ok {
				c.sendAlert(alertUnknownCA)
			} else if errCertificateInvalid, ok := errors.AsType[x509.CertificateInvalidError](err); ok && errCertificateInvalid.Reason == x509.Expired {
				c.sendAlert(alertCertificateExpired)
			} else {
				c.sendAlert(alertBadCertificate)
			}
			return &CertificateVerificationError{UnverifiedCertificates: certs, Err: err}
		}

		c.verifiedChains, err = fipsAllowedChains(chains)
		if err != nil {
			c.sendAlert(alertBadCertificate)
			return &CertificateVerificationError{UnverifiedCertificates: certs, Err: err}
		}
	}

	c.peerCertificates = certs
	c.ocspResponse = certificate.OCSPStaple
	c.scts = certificate.SignedCertificateTimestamps

	if len(certs) > 0 {
		switch certs[0].PublicKey.(type) {
		case *ecdsa.PublicKey, *rsa.PublicKey, ed25519.PublicKey:
		case *mldsa.PublicKey:
			if c.vers < VersionTLS13 {
				c.sendAlert(alertIllegalParameter)
				return errors.New("tls: client certificate uses ML-DSA, which requires TLS 1.3")
			}
		default:
			c.sendAlert(alertUnsupportedCertificate)
			return fmt.Errorf("tls: client certificate contains an unsupported public key of type %T", certs[0].PublicKey)
		}
	}

	if c.config.VerifyPeerCertificate != nil {
		if err := c.config.VerifyPeerCertificate(certificates, c.verifiedChains); err != nil {
			c.sendAlert(alertBadCertificate)
			return err
		}
	}

	return nil
}

func clientHelloInfo(ctx context.Context, c *Conn, clientHello *clientHelloMsg) *ClientHelloInfo {
	supportedVersions := clientHello.supportedVersions
	if len(clientHello.supportedVersions) == 0 {
		supportedVersions = supportedVersionsFromMax(clientHello.vers)
	}

	conn := c.conn
	if c.quic != nil {
		conn = c.quic.clientHelloInfoConn
	}
	return &ClientHelloInfo{
		CipherSuites:      clientHello.cipherSuites,
		ServerName:        clientHello.serverName,
		SupportedCurves:   clientHello.supportedCurves,
		SupportedPoints:   clientHello.supportedPoints,
		SignatureSchemes:  clientHello.supportedSignatureAlgorithms,
		SupportedProtos:   clientHello.alpnProtocols,
		SupportedVersions: supportedVersions,
		Extensions:        clientHello.extensions,
		Conn:              conn,
		HelloRetryRequest: c.didHRR,
		config:            c.config,
		isQUIC:            c.quic != nil,
		ctx:               ctx,
	}
}

```

// === FILE: references/go/src/crypto/tls/handshake_server_tls13.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tls

import (
	"bytes"
	"context"
	"crypto"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/hpke"
	"crypto/internal/fips140/tls13"
	"crypto/rsa"
	"crypto/tls/internal/fips140tls"
	"crypto/x509"
	"errors"
	"fmt"
	"hash"
	"internal/byteorder"
	"io"
	"slices"
	"sort"
	"time"
)

// maxClientPSKIdentities is the number of client PSK identities the server will
// attempt to validate. It will ignore the rest not to let cheap ClientHello
// messages cause too much work in session ticket decryption attempts.
const maxClientPSKIdentities = 5

type echServerContext struct {
	hpkeContext *hpke.Recipient
	configID    uint8
	ciphersuite echCipher
	transcript  hash.Hash
	// inner indicates that the initial client_hello we received contained an
	// encrypted_client_hello extension that indicated it was an "inner" hello.
	// We don't do any additional processing of the hello in this case, so all
	// fields above are unset.
	inner bool
}

type serverHandshakeStateTLS13 struct {
	c               *Conn
	ctx             context.Context
	clientHello     *clientHelloMsg
	hello           *serverHelloMsg
	sentDummyCCS    bool
	usingPSK        bool
	earlyData       bool
	suite           *cipherSuiteTLS13
	cert            *Certificate
	sigAlg          SignatureScheme
	earlySecret     *tls13.EarlySecret
	sharedKey       []byte
	handshakeSecret *tls13.HandshakeSecret
	masterSecret    *tls13.MasterSecret
	trafficSecret   []byte // client_application_traffic_secret_0
	transcript      hash.Hash
	clientFinished  []byte
	echContext      *echServerContext
}

func (hs *serverHandshakeStateTLS13) handshake() error {
	c := hs.c

	// For an overview of the TLS 1.3 handshake, see RFC 8446, Section 2.
	if err := hs.processClientHello(); err != nil {
		return err
	}
	if err := hs.checkForResumption(); err != nil {
		return err
	}
	if err := hs.pickCertificate(); err != nil {
		return err
	}
	c.buffering = true
	if err := hs.sendServerParameters(); err != nil {
		return err
	}
	if err := hs.sendServerCertificate(); err != nil {
		return err
	}
	if err := hs.sendServerFinished(); err != nil {
		return err
	}
	// Note that at this point we could start sending application data without
	// waiting for the client's second flight, but the application might not
	// expect the lack of replay protection of the ClientHello parameters.
	if _, err := c.flush(); err != nil {
		return err
	}
	if err := hs.readClientCertificate(); err != nil {
		return err
	}
	if err := hs.readClientFinished(); err != nil {
		return err
	}

	c.isHandshakeComplete.Store(true)

	return nil
}

func (hs *serverHandshakeStateTLS13) processClientHello() error {
	c := hs.c

	hs.hello = new(serverHelloMsg)

	// TLS 1.3 froze the ServerHello.legacy_version field, and uses
	// supported_versions instead. See RFC 8446, sections 4.1.3 and 4.2.1.
	hs.hello.vers = VersionTLS12
	hs.hello.supportedVersion = c.vers

	if len(hs.clientHello.supportedVersions) == 0 {
		c.sendAlert(alertIllegalParameter)
		return errors.New("tls: client used the legacy version field to negotiate TLS 1.3")
	}

	// Abort if the client is doing a fallback and landing lower than what we
	// support. See RFC 7507, which however does not specify the interaction
	// with supported_versions. The only difference is that with
	// supported_versions a client has a chance to attempt a [TLS 1.2, TLS 1.4]
	// handshake in case TLS 1.3 is broken but 1.2 is not. Alas, in that case,
	// it will have to drop the TLS_FALLBACK_SCSV protection if it falls back to
	// TLS 1.2, because a TLS 1.3 server would abort here. The situation before
	// supported_versions was not better because there was just no way to do a
	// TLS 1.4 handshake without risking the server selecting TLS 1.3.
	for _, id := range hs.clientHello.cipherSuites {
		if id == TLS_FALLBACK_SCSV {
			// Use c.vers instead of max(supported_versions) because an attacker
			// could defeat this by adding an arbitrary high version otherwise.
			if c.vers < c.config.maxSupportedVersion(roleServer, c.quic != nil) {
				c.sendAlert(alertInappropriateFallback)
				return errors.New("tls: client using inappropriate protocol fallback")
			}
			break
		}
	}

	if len(hs.clientHello.compressionMethods) != 1 ||
		hs.clientHello.compressionMethods[0] != compressionNone {
		c.sendAlert(alertIllegalParameter)
		return errors.New("tls: TLS 1.3 client supports illegal compression methods")
	}

	hs.hello.random = make([]byte, 32)
	if _, err := io.ReadFull(c.config.rand(), hs.hello.random); err != nil {
		c.sendAlert(alertInternalError)
		return err
	}

	if len(hs.clientHello.secureRenegotiation) != 0 {
		c.sendAlert(alertHandshakeFailure)
		return errors.New("tls: initial handshake had non-empty renegotiation extension")
	}

	if hs.clientHello.earlyData && c.quic != nil {
		if len(hs.clientHello.pskIdentities) == 0 {
			c.sendAlert(alertIllegalParameter)
			return errors.New("tls: early_data without pre_shared_key")
		}
	} else if hs.clientHello.earlyData {
		// See RFC 8446, Section 4.2.10 for the complicated behavior required
		// here. The scenario is that a different server at our address offered
		// to accept early data in the past, which we can't handle. For now, all
		// 0-RTT enabled session tickets need to expire before a Go server can
		// replace a server or join a pool. That's the same requirement that
		// applies to mixing or replacing with any TLS 1.2 server.
		c.sendAlert(alertUnsupportedExtension)
		return errors.New("tls: client sent unexpected early data")
	}

	hs.hello.sessionId = hs.clientHello.sessionId
	hs.hello.compressionMethod = compressionNone

	preferenceList := defaultCipherSuitesTLS13
	if !hasAESGCMHardwareSupport || !isAESGCMPreferred(hs.clientHello.cipherSuites) {
		preferenceList = defaultCipherSuitesTLS13NoAES
	}
	if fips140tls.Required() {
		preferenceList = allowedCipherSuitesTLS13FIPS
	}
	for _, suiteID := range preferenceList {
		hs.suite = mutualCipherSuiteTLS13(hs.clientHello.cipherSuites, suiteID)
		if hs.suite != nil {
			break
		}
	}
	if hs.suite == nil {
		c.sendAlert(alertHandshakeFailure)
		return fmt.Errorf("tls: no cipher suite supported by both client and server; client offered: %x",
			hs.clientHello.cipherSuites)
	}
	c.cipherSuite = hs.suite.id
	hs.hello.cipherSuite = hs.suite.id
	hs.transcript = hs.suite.hash.New()

	// First, if a post-quantum key exchange is available, use one. See
	// draft-ietf-tls-key-share-prediction-01, Section 4 for why this must be
	// first.
	//
	// Second, if the client sent a key share for a group we support, use that,
	// to avoid a HelloRetryRequest round-trip.
	//
	// Finally, pick in our fixed preference order.
	preferredGroups := c.config.curvePreferences(c.vers)
	preferredGroups = slices.DeleteFunc(preferredGroups, func(group CurveID) bool {
		return !slices.Contains(hs.clientHello.supportedCurves, group)
	})
	if len(preferredGroups) == 0 {
		c.sendAlert(alertHandshakeFailure)
		return errors.New("tls: no key exchanges supported by both client and server")
	}
	hasKeyShare := func(group CurveID) bool {
		for _, ks := range hs.clientHello.keyShares {
			if ks.group == group {
				return true
			}
		}
		return false
	}
	sort.SliceStable(preferredGroups, func(i, j int) bool {
		return hasKeyShare(preferredGroups[i]) && !hasKeyShare(preferredGroups[j])
	})
	sort.SliceStable(preferredGroups, func(i, j int) bool {
		return isPQKeyExchange(preferredGroups[i]) && !isPQKeyExchange(preferredGroups[j])
	})
	selectedGroup := preferredGroups[0]

	var clientKeyShare *keyShare
	for _, ks := range hs.clientHello.keyShares {
		if ks.group == selectedGroup {
			clientKeyShare = &ks
			break
		}
	}
	if clientKeyShare == nil {
		ks, err := hs.doHelloRetryRequest(selectedGroup)
		if err != nil {
			return err
		}
		clientKeyShare = ks
	}
	c.curveID = selectedGroup

	ke, err := keyExchangeForCurveID(selectedGroup)
	if err != nil {
		c.sendAlert(alertInternalError)
		return errors.New("tls: internal error: supportsCurve accepted unimplemented curve")
	}
	hs.sharedKey, hs.hello.serverShare, err = ke.serverSharedSecret(c.config.rand(), clientKeyShare.data)
	if err != nil {
		c.sendAlert(alertIllegalParameter)
		return errors.New("tls: invalid client key share")
	}

	selectedProto, err := negotiateALPN(c.config.NextProtos, hs.clientHello.alpnProtocols, c.quic != nil)
	if err != nil {
		c.sendAlert(alertNoApplicationProtocol)
		return err
	}
	c.clientProtocol = selectedProto

	if c.quic != nil {
		// RFC 9001 Section 4.2: Clients MUST NOT offer TLS versions older than 1.3.
		for _, v := range hs.clientHello.supportedVersions {
			if v < VersionTLS13 {
				c.sendAlert(alertProtocolVersion)
				return errors.New("tls: client offered TLS version older than TLS 1.3")
			}
		}
		// RFC 9001 Section 8.2.
		if hs.clientHello.quicTransportParameters == nil {
			c.sendAlert(alertMissingExtension)
			return errors.New("tls: client did not send a quic_transport_parameters extension")
		}
		c.quicSetTransportParameters(hs.clientHello.quicTransportParameters)
	} else {
		if hs.clientHello.quicTransportParameters != nil {
			c.sendAlert(alertUnsupportedExtension)
			return errors.New("tls: client sent an unexpected quic_transport_parameters extension")
		}
	}

	c.serverName = hs.clientHello.serverName
	return nil
}

func (hs *serverHandshakeStateTLS13) checkForResumption() error {
	c := hs.c

	if c.config.SessionTicketsDisabled {
		return nil
	}

	modeOK := false
	for _, mode := range hs.clientHello.pskModes {
		if mode == pskModeDHE {
			modeOK = true
			break
		}
	}
	if !modeOK {
		return nil
	}

	if len(hs.clientHello.pskIdentities) != len(hs.clientHello.pskBinders) {
		c.sendAlert(alertIllegalParameter)
		return errors.New("tls: invalid or missing PSK binders")
	}
	if len(hs.clientHello.pskIdentities) == 0 {
		return nil
	}

	for i, identity := range hs.clientHello.pskIdentities {
		if i >= maxClientPSKIdentities {
			break
		}

		var sessionState *SessionState
		if c.config.UnwrapSession != nil {
			var err error
			sessionState, err = c.config.UnwrapSession(identity.label, c.connectionStateLocked())
			if err != nil {
				return err
			}
			if sessionState == nil {
				continue
			}
		} else {
			plaintext := c.config.decryptTicket(identity.label, c.ticketKeys)
			if plaintext == nil {
				continue
			}
			var err error
			sessionState, err = ParseSessionState(plaintext)
			if err != nil {
				continue
			}
		}

		if sessionState.version != VersionTLS13 {
			continue
		}

		createdAt := time.Unix(int64(sessionState.createdAt), 0)
		if c.config.time().Sub(createdAt) > maxSessionTicketLifetime {
			continue
		}

		pskSuite := cipherSuiteTLS13ByID(sessionState.cipherSuite)
		if pskSuite == nil || pskSuite.hash != hs.suite.hash {
			continue
		}

		// PSK connections don't re-establish client certificates, but carry
		// them over in the session ticket. Ensure the presence of client certs
		// in the ticket is consistent with the configured requirements.
		sessionHasClientCerts := len(sessionState.peerCertificates) != 0
		needClientCerts := requiresClientCert(c.config.ClientAuth)
		if needClientCerts && !sessionHasClientCerts {
			continue
		}
		if sessionHasClientCerts && c.config.ClientAuth == NoClientCert {
			continue
		}
		if sessionHasClientCerts && c.config.time().After(sessionState.peerCertificates[0].NotAfter) {
			continue
		}
		opts := x509.VerifyOptions{
			CurrentTime: c.config.time(),
			Roots:       c.config.ClientCAs,
			KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}
		if sessionHasClientCerts && c.config.ClientAuth >= VerifyClientCertIfGiven &&
			!anyValidVerifiedChain(sessionState.verifiedChains, opts) {
			continue
		}

		if c.quic != nil && c.quic.enableSessionEvents {
			if err := c.quicResumeSession(sessionState); err != nil {
				return err
			}
		}

		hs.earlySecret = tls13.NewEarlySecret(hs.suite.hash.New, sessionState.secret)
		binderKey := hs.earlySecret.ResumptionBinderKey()
		// Clone the transcript in case a HelloRetryRequest was recorded.
		transcript := cloneHash(hs.transcript, hs.suite.hash)
		if transcript == nil {
			c.sendAlert(alertInternalError)
			return errors.New("tls: internal error: failed to clone hash")
		}
		clientHelloBytes, err := hs.clientHello.marshalWithoutBinders()
		if err != nil {
			c.sendAlert(alertInternalError)
			return err
		}
		transcript.Write(clientHelloBytes)
		pskBinder := hs.suite.finishedHash(binderKey, transcript)
		if !hmac.Equal(hs.clientHello.pskBinders[i], pskBinder) {
			c.sendAlert(alertDecryptError)
			return errors.New("tls: invalid PSK binder")
		}

		if c.quic != nil && hs.clientHello.earlyData && i == 0 &&
			sessionState.EarlyData && sessionState.cipherSuite == hs.suite.id &&
			sessionState.alpnProtocol == c.clientProtocol {
			hs.earlyData = true

			transcript := hs.suite.hash.New()
			if err := transcriptMsg(hs.clientHello, transcript); err != nil {
				return err
			}
			earlyTrafficSecret := hs.earlySecret.ClientEarlyTrafficSecret(transcript)
			if err := c.quicSetReadSecret(QUICEncryptionLevelEarly, hs.suite.id, earlyTrafficSecret); err != nil {
				return err
			}
		}

		c.didResume = true
		c.peerCertificates = sessionState.peerCertificates
		c.ocspResponse = sessionState.ocspResponse
		c.scts = sessionState.scts
		c.verifiedChains = sessionState.verifiedChains

		hs.hello.selectedIdentityPresent = true
		hs.hello.selectedIdentity = uint16(i)
		hs.usingPSK = true
		return nil
	}

	return nil
}

// cloneHash uses [hash.Cloner] to clone in. If [hash.Cloner]
// is not implemented or not supported, then it falls back to the
// [encoding.BinaryMarshaler] and [encoding.BinaryUnmarshaler]
// interfaces implemented by standard library hashes to clone the state of in
// to a new instance of h. It returns nil if the operation fails.
func cloneHash(in hash.Hash, h crypto.Hash) hash.Hash {
	if cloner, ok := in.(hash.Cloner); ok {
		if out, err := cloner.Clone(); err == nil {
			return out
		}
	}
	// Recreate the interface to avoid importing encoding.
	type binaryMarshaler interface {
		MarshalBinary() (data []byte, err error)
		UnmarshalBinary(data []byte) error
	}
	marshaler, ok := in.(binaryMarshaler)
	if !ok {
		return nil
	}
	state, err := marshaler.MarshalBinary()
	if err != nil {
		return nil
	}
	out := h.New()
	unmarshaler, ok := out.(binaryMarshaler)
	if !ok {
		return nil
	}
	if err := unmarshaler.UnmarshalBinary(state); err != nil {
		return nil
	}
	return out
}

func (hs *serverHandshakeStateTLS13) pickCertificate() error {
	c := hs.c

	// Only one of PSK and certificates are used at a time.
	if hs.usingPSK {
		return nil
	}

	// signature_algorithms is required in TLS 1.3. See RFC 8446, Section 4.2.3.
	if len(hs.clientHello.supportedSignatureAlgorithms) == 0 {
		return c.sendAlert(alertMissingExtension)
	}

	certificate, err := c.config.getCertificate(clientHelloInfo(hs.ctx, c, hs.clientHello))
	if err != nil {
		if err == errNoCertificates {
			c.sendAlert(alertUnrecognizedName)
		} else {
			c.sendAlert(alertInternalError)
		}
		return err
	}
	if certificate != nil {
		hs.c.localCertificate = certificate.Certificate
	}
	hs.sigAlg, err = selectSignatureScheme(c.vers, certificate, hs.clientHello.supportedSignatureAlgorithms)
	if err != nil {
		// getCertificate returned a certificate that is unsupported or
		// incompatible with the client's signature algorithms.
		c.sendAlert(alertHandshakeFailure)
		return err
	}
	hs.cert = certificate

	return nil
}

// sendDummyChangeCipherSpec sends a ChangeCipherSpec record for compatibility
// with middleboxes that didn't implement TLS correctly. See RFC 8446, Appendix D.4.
func (hs *serverHandshakeStateTLS13) sendDummyChangeCipherSpec() error {
	if hs.c.quic != nil {
		return nil
	}
	if hs.sentDummyCCS {
		return nil
	}
	hs.sentDummyCCS = true

	return hs.c.writeChangeCipherRecord()
}

func (hs *serverHandshakeStateTLS13) doHelloRetryRequest(selectedGroup CurveID) (*keyShare, error) {
	c := hs.c

	// Make sure the client didn't send extra handshake messages alongside
	// their initial client_hello. If they sent two client_hello messages,
	// we will consume the second before they respond to the server_hello.
	if c.hand.Len() != 0 {
		c.sendAlert(alertUnexpectedMessage)
		return nil, errors.New("tls: handshake buffer not empty before HelloRetryRequest")
	}

	// The first ClientHello gets double-hashed into the transcript upon a
	// HelloRetryRequest. See RFC 8446, Section 4.4.1.
	if err := transcriptMsg(hs.clientHello, hs.transcript); err != nil {
		return nil, err
	}
	chHash := hs.transcript.Sum(nil)
	hs.transcript.Reset()
	hs.transcript.Write([]byte{typeMessageHash, 0, 0, uint8(len(chHash))})
	hs.transcript.Write(chHash)

	helloRetryRequest := &serverHelloMsg{
		vers:              hs.hello.vers,
		random:            helloRetryRequestRandom,
		sessionId:         hs.hello.sessionId,
		cipherSuite:       hs.hello.cipherSuite,
		compressionMethod: hs.hello.compressionMethod,
		supportedVersion:  hs.hello.supportedVersion,
		selectedGroup:     selectedGroup,
	}

	if hs.echContext != nil {
		// Compute the acceptance message.
		helloRetryRequest.encryptedClientHello = make([]byte, 8)
		confTranscript := cloneHash(hs.transcript, hs.suite.hash)
		if err := transcriptMsg(helloRetryRequest, confTranscript); err != nil {
			return nil, err
		}
		h := hs.suite.hash.New
		prf, err := hkdf.Extract(h, hs.clientHello.random, nil)
		if err != nil {
			c.sendAlert(alertInternalError)
			return nil, err
		}
		acceptConfirmation := tls13.ExpandLabel(h, prf, "hrr ech accept confirmation", confTranscript.Sum(nil), 8)
		helloRetryRequest.encryptedClientHello = acceptConfirmation
	}

	if _, err := hs.c.writeHandshakeRecord(helloRetryRequest, hs.transcript); err != nil {
		return nil, err
	}

	if err := hs.sendDummyChangeCipherSpec(); err != nil {
		return nil, err
	}

	// clientHelloMsg is not included in the transcript.
	msg, err := c.readHandshake(nil)
	if err != nil {
		return nil, err
	}

	clientHello, ok := msg.(*clientHelloMsg)
	if !ok {
		c.sendAlert(alertUnexpectedMessage)
		return nil, unexpectedMessageError(clientHello, msg)
	}

	if hs.echContext != nil {
		if len(clientHello.encryptedClientHello) == 0 {
			c.sendAlert(alertMissingExtension)
			return nil, errors.New("tls: second client hello missing encrypted client hello extension")
		}

		echType, echCiphersuite, configID, encap, payload, err := parseECHExt(clientHello.encryptedClientHello)
		if err != nil {
			c.sendAlert(alertDecodeError)
			return nil, errors.New("tls: client sent invalid encrypted client hello extension")
		}

		if echType == outerECHExt && hs.echContext.inner || echType == innerECHExt && !hs.echContext.inner {
			c.sendAlert(alertDecodeError)
			return nil, errors.New("tls: unexpected switch in encrypted client hello extension type")
		}

		if echType == outerECHExt {
			if echCiphersuite != hs.echContext.ciphersuite || configID != hs.echContext.configID || len(encap) != 0 {
				c.sendAlert(alertIllegalParameter)
				return nil, errors.New("tls: second client hello encrypted client hello extension does not match")
			}

			encodedInner, err := decryptECHPayload(hs.echContext.hpkeContext, clientHello.original, payload)
			if err != nil {
				c.sendAlert(alertDecryptError)
				return nil, errors.New("tls: failed to decrypt second client hello encrypted client hello extension payload")
			}

			echInner, err := decodeInnerClientHello(clientHello, encodedInner)
			if err != nil {
				c.sendAlert(alertIllegalParameter)
				return nil, errors.New("tls: client sent invalid encrypted client hello extension")
			}

			clientHello = echInner
		}
	}

	if len(clientHello.keyShares) != 1 {
		c.sendAlert(alertIllegalParameter)
		return nil, errors.New("tls: client didn't send one key share in second ClientHello")
	}
	ks := &clientHello.keyShares[0]

	if ks.group != selectedGroup {
		c.sendAlert(alertIllegalParameter)
		return nil, errors.New("tls: client sent unexpected key share in second ClientHello")
	}

	if clientHello.earlyData {
		c.sendAlert(alertIllegalParameter)
		return nil, errors.New("tls: client indicated early data in second ClientHello")
	}

	if illegalClientHelloChange(clientHello, hs.clientHello) {
		c.sendAlert(alertIllegalParameter)
		return nil, errors.New("tls: client illegally modified second ClientHello")
	}

	c.didHRR = true
	hs.clientHello = clientHello
	return ks, nil
}

// illegalClientHelloChange reports whether the two ClientHello messages are
// different, with the exception of the changes allowed before and after a
// HelloRetryRequest. See RFC 8446, Section 4.1.2.
func illegalClientHelloChange(ch, ch1 *clientHelloMsg) bool {
	if len(ch.supportedVersions) != len(ch1.supportedVersions) ||
		len(ch.cipherSuites) != len(ch1.cipherSuites) ||
		len(ch.supportedCurves) != len(ch1.supportedCurves) ||
		len(ch.supportedSignatureAlgorithms) != len(ch1.supportedSignatureAlgorithms) ||
		len(ch.supportedSignatureAlgorithmsCert) != len(ch1.supportedSignatureAlgorithmsCert) ||
		len(ch.alpnProtocols) != len(ch1.alpnProtocols) {
		return true
	}
	for i := range ch.supportedVersions {
		if ch.supportedVersions[i] != ch1.supportedVersions[i] {
			return true
		}
	}
	for i := range ch.cipherSuites {
		if ch.cipherSuites[i] != ch1.cipherSuites[i] {
			return true
		}
	}
	for i := range ch.supportedCurves {
		if ch.supportedCurves[i] != ch1.supportedCurves[i] {
			return true
		}
	}
	for i := range ch.supportedSignatureAlgorithms {
		if ch.supportedSignatureAlgorithms[i] != ch1.supportedSignatureAlgorithms[i] {
			return true
		}
	}
	for i := range ch.supportedSignatureAlgorithmsCert {
		if ch.supportedSignatureAlgorithmsCert[i] != ch1.supportedSignatureAlgorithmsCert[i] {
			return true
		}
	}
	for i := range ch.alpnProtocols {
		if ch.alpnProtocols[i] != ch1.alpnProtocols[i] {
			return true
		}
	}
	return ch.vers != ch1.vers ||
		!bytes.Equal(ch.random, ch1.random) ||
		!bytes.Equal(ch.sessionId, ch1.sessionId) ||
		!bytes.Equal(ch.compressionMethods, ch1.compressionMethods) ||
		ch.serverName != ch1.serverName ||
		ch.ocspStapling != ch1.ocspStapling ||
		!bytes.Equal(ch.supportedPoints, ch1.supportedPoints) ||
		ch.ticketSupported != ch1.ticketSupported ||
		!bytes.Equal(ch.sessionTicket, ch1.sessionTicket) ||
		ch.secureRenegotiationSupported != ch1.secureRenegotiationSupported ||
		!bytes.Equal(ch.secureRenegotiation, ch1.secureRenegotiation) ||
		ch.scts != ch1.scts ||
		!bytes.Equal(ch.cookie, ch1.cookie) ||
		!bytes.Equal(ch.pskModes, ch1.pskModes)
}

func (hs *serverHandshakeStateTLS13) sendServerParameters() error {
	c := hs.c

	if hs.echContext != nil {
		copy(hs.hello.random[32-8:], make([]byte, 8))
		echTranscript := cloneHash(hs.transcript, hs.suite.hash)
		echTranscript.Write(hs.clientHello.original)
		if err := transcriptMsg(hs.hello, echTranscript); err != nil {
			return err
		}
		// compute the acceptance message
		h := hs.suite.hash.New
		prk, err := hkdf.Extract(h, hs.clientHello.random, nil)
		if err != nil {
			c.sendAlert(alertInternalError)
			return err
		}
		acceptConfirmation := tls13.ExpandLabel(h, prk, "ech accept confirmation", echTranscript.Sum(nil), 8)
		copy(hs.hello.random[32-8:], acceptConfirmation)
	}

	if err := transcriptMsg(hs.clientHello, hs.transcript); err != nil {
		return err
	}

	if _, err := hs.c.writeHandshakeRecord(hs.hello, hs.transcript); err != nil {
		return err
	}

	if err := hs.sendDummyChangeCipherSpec(); err != nil {
		return err
	}

	earlySecret := hs.earlySecret
	if earlySecret == nil {
		earlySecret = tls13.NewEarlySecret(hs.suite.hash.New, nil)
	}
	hs.handshakeSecret = earlySecret.HandshakeSecret(hs.sharedKey)

	serverSecret := hs.handshakeSecret.ServerHandshakeTrafficSecret(hs.transcript)
	c.setWriteTrafficSecret(hs.suite, QUICEncryptionLevelHandshake, serverSecret)
	clientSecret := hs.handshakeSecret.ClientHandshakeTrafficSecret(hs.transcript)
	if err := c.setReadTrafficSecret(hs.suite, QUICEncryptionLevelHandshake, clientSecret, false); err != nil {
		return err
	}

	if c.quic != nil {
		c.quicSetWriteSecret(QUICEncryptionLevelHandshake, hs.suite.id, serverSecret)
		if err := c.quicSetReadSecret(QUICEncryptionLevelHandshake, hs.suite.id, clientSecret); err != nil {
			return err
		}
	}

	err := c.config.writeKeyLog(keyLogLabelClientHandshake, hs.clientHello.random, clientSecret)
	if err != nil {
		c.sendAlert(alertInternalError)
		return err
	}
	err = c.config.writeKeyLog(keyLogLabelServerHandshake, hs.clientHello.random, serverSecret)
	if err != nil {
		c.sendAlert(alertInternalError)
		return err
	}

	encryptedExtensions := new(encryptedExtensionsMsg)
	encryptedExtensions.alpnProtocol = c.clientProtocol

	if c.quic != nil {
		p, err := c.quicGetTransportParameters()
		if err != nil {
			return err
		}
		encryptedExtensions.quicTransportParameters = p
		encryptedExtensions.earlyData = hs.earlyData
	}

	if !hs.c.didResume && hs.clientHello.serverName != "" {
		encryptedExtensions.serverNameAck = true
	}

	// If client sent ECH extension, but we didn't accept it,
	// send retry configs, if available.
	echKeys := hs.c.config.EncryptedClientHelloKeys
	if hs.c.config.GetEncryptedClientHelloKeys != nil {
		echKeys, err = hs.c.config.GetEncryptedClientHelloKeys(clientHelloInfo(hs.ctx, c, hs.clientHello))
		if err != nil {
			c.sendAlert(alertInternalError)
			return err
		}
	}
	if len(echKeys) > 0 && len(hs.clientHello.encryptedClientHello) > 0 && hs.echContext == nil {
		encryptedExtensions.echRetryConfigs, err = buildRetryConfigList(echKeys)
		if err != nil {
			c.sendAlert(alertInternalError)
			return err
		}
	}

	if _, err := hs.c.writeHandshakeRecord(encryptedExtensions, hs.transcript); err != nil {
		return err
	}

	return nil
}

func (hs *serverHandshakeStateTLS13) requestClientCert() bool {
	return hs.c.config.ClientAuth >= RequestClientCert && !hs.usingPSK
}

func (hs *serverHandshakeStateTLS13) sendServerCertificate() error {
	c := hs.c

	// Only one of PSK and certificates are used at a time.
	if hs.usingPSK {
		return nil
	}

	if hs.requestClientCert() {
		// Request a client certificate
		certReq := new(certificateRequestMsgTLS13)
		certReq.ocspStapling = true
		certReq.scts = true
		certReq.supportedSignatureAlgorithms = supportedSignatureAlgorithms(c.vers, c.vers)
		certReq.supportedSignatureAlgorithmsCert = supportedSignatureAlgorithmsCert(c.vers, c.vers)
		if c.config.ClientCAs != nil {
			certReq.certificateAuthorities = c.config.ClientCAs.Subjects()
		}

		if _, err := hs.c.writeHandshakeRecord(certReq, hs.transcript); err != nil {
			return err
		}
	}

	certMsg := new(certificateMsgTLS13)

	certMsg.certificate = *hs.cert
	certMsg.scts = hs.clientHello.scts && len(hs.cert.SignedCertificateTimestamps) > 0
	certMsg.ocspStapling = hs.clientHello.ocspStapling && len(hs.cert.OCSPStaple) > 0

	if _, err := hs.c.writeHandshakeRecord(certMsg, hs.transcript); err != nil {
		return err
	}

	certVerifyMsg := new(certificateVerifyMsg)
	certVerifyMsg.hasSignatureAlgorithm = true
	certVerifyMsg.signatureAlgorithm = hs.sigAlg

	sigType, sigHash, err := typeAndHashFromSignatureScheme(hs.sigAlg)
	if err != nil {
		return c.sendAlert(alertInternalError)
	}

	signed := signedMessage(serverSignatureContext, hs.transcript)
	signOpts := crypto.SignerOpts(sigHash)
	if sigType == signatureRSAPSS {
		signOpts = &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: sigHash}
	}
	sig, err := crypto.SignMessage(hs.cert.PrivateKey.(crypto.Signer), c.config.rand(), signed, signOpts)
	if err != nil {
		public := hs.cert.PrivateKey.(crypto.Signer).Public()
		if rsaKey, ok := public.(*rsa.PublicKey); ok && sigType == signatureRSAPSS &&
			rsaKey.N.BitLen()/8 < sigHash.Size()*2+2 { // key too small for RSA-PSS
			c.sendAlert(alertHandshakeFailure)
		} else {
			c.sendAlert(alertInternalError)
		}
		return errors.New("tls: failed to sign handshake: " + err.Error())
	}
	certVerifyMsg.signature = sig

	if _, err := hs.c.writeHandshakeRecord(certVerifyMsg, hs.transcript); err != nil {
		return err
	}

	return nil
}

func (hs *serverHandshakeStateTLS13) sendServerFinished() error {
	c := hs.c

	finished := &finishedMsg{
		verifyData: hs.suite.finishedHash(c.out.trafficSecret, hs.transcript),
	}

	if _, err := hs.c.writeHandshakeRecord(finished, hs.transcript); err != nil {
		return err
	}

	// Derive secrets that take context through the server Finished.

	hs.masterSecret = hs.handshakeSecret.MasterSecret()

	hs.trafficSecret = hs.masterSecret.ClientApplicationTrafficSecret(hs.transcript)
	serverSecret := hs.masterSecret.ServerApplicationTrafficSecret(hs.transcript)
	c.setWriteTrafficSecret(hs.suite, QUICEncryptionLevelApplication, serverSecret)

	if c.quic != nil {
		c.quicSetWriteSecret(QUICEncryptionLevelApplication, hs.suite.id, serverSecret)
	}

	err := c.config.writeKeyLog(keyLogLabelClientTraffic, hs.clientHello.random, hs.trafficSecret)
	if err != nil {
		c.sendAlert(alertInternalError)
		return err
	}
	err = c.config.writeKeyLog(keyLogLabelServerTraffic, hs.clientHello.random, serverSecret)
	if err != nil {
		c.sendAlert(alertInternalError)
		return err
	}

	c.ekm = hs.suite.exportKeyingMaterial(hs.masterSecret, hs.transcript)

	// If we did not request client certificates, at this point we can
	// precompute the client finished and roll the transcript forward to send
	// session tickets in our first flight.
	if !hs.requestClientCert() {
		if err := hs.sendSessionTickets(); err != nil {
			return err
		}
	}

	return nil
}

func (hs *serverHandshakeStateTLS13) shouldSendSessionTickets() bool {
	if hs.c.config.SessionTicketsDisabled {
		return false
	}

	// QUIC tickets are sent by QUICConn.SendSessionTicket, not automatically.
	if hs.c.quic != nil {
		return false
	}

	// Don't send tickets the client wouldn't use. See RFC 8446, Section 4.2.9.
	return slices.Contains(hs.clientHello.pskModes, pskModeDHE)
}

func (hs *serverHandshakeStateTLS13) sendSessionTickets() error {
	c := hs.c

	hs.clientFinished = hs.suite.finishedHash(c.in.trafficSecret, hs.transcript)
	finishedMsg := &finishedMsg{
		verifyData: hs.clientFinished,
	}
	if err := transcriptMsg(finishedMsg, hs.transcript); err != nil {
		return err
	}

	c.resumptionSecret = hs.masterSecret.ResumptionMasterSecret(hs.transcript)

	if !hs.shouldSendSessionTickets() {
		return nil
	}
	return c.sendSessionTicket(false, nil)
}

func (c *Conn) sendSessionTicket(earlyData bool, extra [][]byte) error {
	suite := cipherSuiteTLS13ByID(c.cipherSuite)
	if suite == nil {
		return errors.New("tls: internal error: unknown cipher suite")
	}
	// ticket_nonce, which must be unique per connection, is always left at
	// zero because we only ever send one ticket per connection.
	psk := tls13.ExpandLabel(suite.hash.New, c.resumptionSecret, "resumption",
		nil, suite.hash.Size())

	m := new(newSessionTicketMsgTLS13)

	state := c.sessionState()
	state.secret = psk
	state.EarlyData = earlyData
	state.Extra = extra
	if c.config.WrapSession != nil {
		var err error
		m.label, err = c.config.WrapSession(c.connectionStateLocked(), state)
		if err != nil {
			return err
		}
	} else {
		stateBytes, err := state.Bytes()
		if err != nil {
			c.sendAlert(alertInternalError)
			return err
		}
		m.label, err = c.config.encryptTicket(stateBytes, c.ticketKeys)
		if err != nil {
			return err
		}
	}
	m.lifetime = uint32(maxSessionTicketLifetime / time.Second)

	// ticket_age_add is a random 32-bit value. See RFC 8446, section 4.6.1
	// The value is not stored anywhere; we never need to check the ticket age
	// because 0-RTT is not supported.
	ageAdd := make([]byte, 4)
	if _, err := c.config.rand().Read(ageAdd); err != nil {
		return err
	}
	m.ageAdd = byteorder.LEUint32(ageAdd)

	if earlyData {
		// RFC 9001, Section 4.6.1
		m.maxEarlyData = 0xffffffff
	}

	if _, err := c.writeHandshakeRecord(m, nil); err != nil {
		return err
	}

	return nil
}

func (hs *serverHandshakeStateTLS13) readClientCertificate() error {
	c := hs.c

	if !hs.requestClientCert() {
		// Make sure the connection is still being verified whether or not
		// the server requested a client certificate.
		if c.config.VerifyConnection != nil {
			if err := c.config.VerifyConnection(c.connectionStateLocked()); err != nil {
				c.sendAlert(alertBadCertificate)
				return err
			}
		}
		return nil
	}

	// If we requested a client certificate, then the client must send a
	// certificate message. If it's empty, no CertificateVerify is sent.

	msg, err := c.readHandshake(hs.transcript)
	if err != nil {
		return err
	}

	certMsg, ok := msg.(*certificateMsgTLS13)
	if !ok {
		c.sendAlert(alertUnexpectedMessage)
		return unexpectedMessageError(certMsg, msg)
	}

	if err := c.processCertsFromClient(certMsg.certificate); err != nil {
		return err
	}

	if c.config.VerifyConnection != nil {
		if err := c.config.VerifyConnection(c.connectionStateLocked()); err != nil {
			c.sendAlert(alertBadCertificate)
			return err
		}
	}

	if len(certMsg.certificate.Certificate) != 0 {
		// certificateVerifyMsg is included in the transcript, but not until
		// after we verify the handshake signature, since the state before
		// this message was sent is used.
		msg, err = c.readHandshake(nil)
		if err != nil {
			return err
		}

		certVerify, ok := msg.(*certificateVerifyMsg)
		if !ok {
			c.sendAlert(alertUnexpectedMessage)
			return unexpectedMessageError(certVerify, msg)
		}

		// See RFC 8446, Section 4.4.3.
		// We don't use certReq.supportedSignatureAlgorithms because it would
		// require keeping the certificateRequestMsgTLS13 around in the hs.
		if !isSupportedSignatureAlgorithm(certVerify.signatureAlgorithm, supportedSignatureAlgorithms(c.vers, c.vers)) ||
			!isSupportedSignatureAlgorithm(certVerify.signatureAlgorithm, signatureSchemesForPublicKey(c.vers, c.peerCertificates[0].PublicKey)) {
			c.sendAlert(alertIllegalParameter)
			return errors.New("tls: client certificate used with invalid signature algorithm")
		}
		sigType, sigHash, err := typeAndHashFromSignatureScheme(certVerify.signatureAlgorithm)
		if err != nil {
			return c.sendAlert(alertInternalError)
		}
		if sigType == signaturePKCS1v15 || sigHash == crypto.SHA1 {
			return c.sendAlert(alertInternalError)
		}
		signed := signedMessage(clientSignatureContext, hs.transcript)
		if err := verifyHandshakeSignature(sigType, c.peerCertificates[0].PublicKey,
			sigHash, signed, certVerify.signature); err != nil {
			c.sendAlert(alertDecryptError)
			return errors.New("tls: invalid signature by the client certificate: " + err.Error())
		}
		c.peerSigAlg = certVerify.signatureAlgorithm

		if err := transcriptMsg(certVerify, hs.transcript); err != nil {
			return err
		}
	}

	// If we waited until the client certificates to send session tickets, we
	// are ready to do it now.
	if err := hs.sendSessionTickets(); err != nil {
		return err
	}

	return nil
}

func (hs *serverHandshakeStateTLS13) readClientFinished() error {
	c := hs.c

	// finishedMsg is not included in the transcript.
	msg, err := c.readHandshake(nil)
	if err != nil {
		return err
	}

	finished, ok := msg.(*finishedMsg)
	if !ok {
		c.sendAlert(alertUnexpectedMessage)
		return unexpectedMessageError(finished, msg)
	}

	if !hmac.Equal(hs.clientFinished, finished.verifyData) {
		c.sendAlert(alertDecryptError)
		return errors.New("tls: invalid client finished hash")
	}

	if err := c.setReadTrafficSecret(hs.suite, QUICEncryptionLevelApplication, hs.trafficSecret, false); err != nil {
		return err
	}

	return nil
}

```

// === FILE: references/go/src/crypto/tls/internal/fips140tls/fipstls.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fips140tls controls whether crypto/tls requires FIPS-approved settings.
package fips140tls

import (
	"crypto/fips140"
	"sync/atomic"
)

var required atomic.Bool

func init() {
	if fips140.Enabled() {
		Force()
	}
}

// Force forces crypto/tls to restrict TLS configurations to FIPS-approved settings.
// By design, this call is impossible to undo (except in tests).
func Force() {
	required.Store(true)
}

// Required reports whether FIPS-approved settings are required.
//
// Required is true if FIPS 140-3 mode is enabled with GODEBUG=fips140=on, or if
// the crypto/tls/fipsonly package is imported by a Go+BoringCrypto build.
func Required() bool {
	return required.Load()
}

func TestingOnlyAbandon() {
	required.Store(false)
}

```

// === FILE: references/go/src/crypto/tls/key_agreement.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tls

import (
	"crypto"
	"crypto/ecdh"
	"crypto/md5"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"slices"
)

// A keyAgreement implements the client and server side of a TLS 1.0–1.2 key
// agreement protocol by generating and processing key exchange messages.
type keyAgreement interface {
	// On the server side, the first two methods are called in order.

	// In the case that the key agreement protocol doesn't use a
	// ServerKeyExchange message, generateServerKeyExchange can return nil,
	// nil.
	generateServerKeyExchange(*Config, *Certificate, *clientHelloMsg, *serverHelloMsg) (*serverKeyExchangeMsg, error)
	processClientKeyExchange(*Config, *Certificate, *clientKeyExchangeMsg, uint16) ([]byte, error)

	// On the client side, the next two methods are called in order.

	// This method may not be called if the server doesn't send a
	// ServerKeyExchange message.
	processServerKeyExchange(*Config, *clientHelloMsg, *serverHelloMsg, *x509.Certificate, *serverKeyExchangeMsg) error
	generateClientKeyExchange(*Config, *clientHelloMsg, *x509.Certificate) ([]byte, *clientKeyExchangeMsg, error)
}

var errClientKeyExchange = errors.New("tls: invalid ClientKeyExchange message")
var errServerKeyExchange = errors.New("tls: invalid ServerKeyExchange message")

// rsaKeyAgreement implements the standard TLS key agreement where the client
// encrypts the pre-master secret to the server's public key.
type rsaKeyAgreement struct{}

func (ka rsaKeyAgreement) generateServerKeyExchange(config *Config, cert *Certificate, clientHello *clientHelloMsg, hello *serverHelloMsg) (*serverKeyExchangeMsg, error) {
	return nil, nil
}

func (ka rsaKeyAgreement) processClientKeyExchange(config *Config, cert *Certificate, ckx *clientKeyExchangeMsg, version uint16) ([]byte, error) {
	if len(ckx.ciphertext) < 2 {
		return nil, errClientKeyExchange
	}
	ciphertextLen := int(ckx.ciphertext[0])<<8 | int(ckx.ciphertext[1])
	if ciphertextLen != len(ckx.ciphertext)-2 {
		return nil, errClientKeyExchange
	}
	ciphertext := ckx.ciphertext[2:]

	priv, ok := cert.PrivateKey.(crypto.Decrypter)
	if !ok {
		return nil, errors.New("tls: certificate private key does not implement crypto.Decrypter")
	}
	// Perform constant time RSA PKCS #1 v1.5 decryption
	preMasterSecret, err := priv.Decrypt(config.rand(), ciphertext, &rsa.PKCS1v15DecryptOptions{SessionKeyLen: 48})
	if err != nil {
		return nil, err
	}
	// We don't check the version number in the premaster secret. For one,
	// by checking it, we would leak information about the validity of the
	// encrypted pre-master secret. Secondly, it provides only a small
	// benefit against a downgrade attack and some implementations send the
	// wrong version anyway. See the discussion at the end of section
	// 7.4.7.1 of RFC 4346.
	return preMasterSecret, nil
}

func (ka rsaKeyAgreement) processServerKeyExchange(config *Config, clientHello *clientHelloMsg, serverHello *serverHelloMsg, cert *x509.Certificate, skx *serverKeyExchangeMsg) error {
	return errors.New("tls: unexpected ServerKeyExchange")
}

func (ka rsaKeyAgreement) generateClientKeyExchange(config *Config, clientHello *clientHelloMsg, cert *x509.Certificate) ([]byte, *clientKeyExchangeMsg, error) {
	preMasterSecret := make([]byte, 48)
	preMasterSecret[0] = byte(clientHello.vers >> 8)
	preMasterSecret[1] = byte(clientHello.vers)
	_, err := io.ReadFull(config.rand(), preMasterSecret[2:])
	if err != nil {
		return nil, nil, err
	}

	rsaKey, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, nil, errors.New("tls: server certificate contains incorrect key type for selected ciphersuite")
	}
	encrypted, err := rsa.EncryptPKCS1v15(config.rand(), rsaKey, preMasterSecret)
	if err != nil {
		return nil, nil, err
	}
	ckx := new(clientKeyExchangeMsg)
	ckx.ciphertext = make([]byte, len(encrypted)+2)
	ckx.ciphertext[0] = byte(len(encrypted) >> 8)
	ckx.ciphertext[1] = byte(len(encrypted))
	copy(ckx.ciphertext[2:], encrypted)
	return preMasterSecret, ckx, nil
}

// sha1Hash calculates a SHA1 hash over the given byte slices.
func sha1Hash(slices [][]byte) []byte {
	hsha1 := sha1.New()
	for _, slice := range slices {
		hsha1.Write(slice)
	}
	return hsha1.Sum(nil)
}

// md5SHA1Hash implements TLS 1.0's hybrid hash function which consists of the
// concatenation of an MD5 and SHA1 hash.
func md5SHA1Hash(slices [][]byte) []byte {
	md5sha1 := make([]byte, md5.Size+sha1.Size)
	hmd5 := md5.New()
	for _, slice := range slices {
		hmd5.Write(slice)
	}
	copy(md5sha1, hmd5.Sum(nil))
	copy(md5sha1[md5.Size:], sha1Hash(slices))
	return md5sha1
}

// hashForServerKeyExchange hashes the given slices and returns their digest
// using a hash based on the sigType. It can only be used for TLS 1.0 and 1.1.
func hashForServerKeyExchange(sigType uint8, slices ...[]byte) []byte {
	if sigType == signatureECDSA {
		return sha1Hash(slices)
	}
	return md5SHA1Hash(slices)
}

// ecdheKeyAgreement implements a TLS key agreement where the server
// generates an ephemeral EC public/private key pair and signs it. The
// pre-master secret is then calculated using ECDH. The signature may
// be ECDSA, Ed25519 or RSA.
type ecdheKeyAgreement struct {
	version uint16
	isRSA   bool

	// ckx and preMasterSecret are generated in processServerKeyExchange
	// and returned in generateClientKeyExchange.
	ckx             *clientKeyExchangeMsg
	preMasterSecret []byte

	// curveID, signatureAlgorithm, and key are set by processServerKeyExchange
	// and generateServerKeyExchange.
	curveID            CurveID
	signatureAlgorithm SignatureScheme
	key                *ecdh.PrivateKey
}

func (ka *ecdheKeyAgreement) generateServerKeyExchange(config *Config, cert *Certificate, clientHello *clientHelloMsg, hello *serverHelloMsg) (*serverKeyExchangeMsg, error) {
	for _, c := range clientHello.supportedCurves {
		if config.supportsCurve(ka.version, c) {
			ka.curveID = c
			break
		}
	}

	if ka.curveID == 0 {
		return nil, errors.New("tls: no supported elliptic curves offered")
	}
	if _, ok := curveForCurveID(ka.curveID); !ok {
		return nil, errors.New("tls: internal error: supportsCurve accepted unimplemented curve")
	}

	key, err := generateECDHEKey(config.rand(), ka.curveID)
	if err != nil {
		return nil, err
	}
	ka.key = key

	// See RFC 4492, Section 5.4.
	ecdhePublic := key.PublicKey().Bytes()
	serverECDHEParams := make([]byte, 1+2+1+len(ecdhePublic))
	serverECDHEParams[0] = 3 // named curve
	serverECDHEParams[1] = byte(ka.curveID >> 8)
	serverECDHEParams[2] = byte(ka.curveID)
	serverECDHEParams[3] = byte(len(ecdhePublic))
	copy(serverECDHEParams[4:], ecdhePublic)

	priv, ok := cert.PrivateKey.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("tls: certificate private key of type %T does not implement crypto.Signer", cert.PrivateKey)
	}

	var sig []byte
	if ka.version >= VersionTLS12 {
		ka.signatureAlgorithm, err = selectSignatureScheme(ka.version, cert, clientHello.supportedSignatureAlgorithms)
		if err != nil {
			return nil, err
		}
		sigType, sigHash, err := typeAndHashFromSignatureScheme(ka.signatureAlgorithm)
		if err != nil {
			return nil, err
		}
		if sigHash == crypto.SHA1 {
			tlssha1.Value() // ensure godebug is initialized
			tlssha1.IncNonDefault()
		}
		signed := slices.Concat(clientHello.random, hello.random, serverECDHEParams)
		if (sigType == signaturePKCS1v15 || sigType == signatureRSAPSS) != ka.isRSA {
			return nil, errors.New("tls: certificate cannot be used with the selected cipher suite")
		}
		signOpts := crypto.SignerOpts(sigHash)
		if sigType == signatureRSAPSS {
			signOpts = &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: sigHash}
		}
		sig, err = crypto.SignMessage(priv, config.rand(), signed, signOpts)
		if err != nil {
			return nil, errors.New("tls: failed to sign ECDHE parameters: " + err.Error())
		}
	} else {
		sigType, sigHash, err := legacyTypeAndHashFromPublicKey(priv.Public())
		if err != nil {
			return nil, err
		}
		signed := hashForServerKeyExchange(sigType, clientHello.random, hello.random, serverECDHEParams)
		if (sigType == signaturePKCS1v15) != ka.isRSA {
			return nil, errors.New("tls: certificate cannot be used with the selected cipher suite")
		}
		sig, err = priv.Sign(config.rand(), signed, sigHash)
		if err != nil {
			return nil, errors.New("tls: failed to sign ECDHE parameters: " + err.Error())
		}
	}

	skx := new(serverKeyExchangeMsg)
	sigAndHashLen := 0
	if ka.version >= VersionTLS12 {
		sigAndHashLen = 2
	}
	skx.key = make([]byte, len(serverECDHEParams)+sigAndHashLen+2+len(sig))
	copy(skx.key, serverECDHEParams)
	k := skx.key[len(serverECDHEParams):]
	if ka.version >= VersionTLS12 {
		k[0] = byte(ka.signatureAlgorithm >> 8)
		k[1] = byte(ka.signatureAlgorithm)
		k = k[2:]
	}
	k[0] = byte(len(sig) >> 8)
	k[1] = byte(len(sig))
	copy(k[2:], sig)

	return skx, nil
}

func (ka *ecdheKeyAgreement) processClientKeyExchange(config *Config, cert *Certificate, ckx *clientKeyExchangeMsg, version uint16) ([]byte, error) {
	if len(ckx.ciphertext) == 0 || int(ckx.ciphertext[0]) != len(ckx.ciphertext)-1 {
		return nil, errClientKeyExchange
	}

	peerKey, err := ka.key.Curve().NewPublicKey(ckx.ciphertext[1:])
	if err != nil {
		return nil, errClientKeyExchange
	}
	preMasterSecret, err := ka.key.ECDH(peerKey)
	if err != nil {
		return nil, errClientKeyExchange
	}

	return preMasterSecret, nil
}

func (ka *ecdheKeyAgreement) processServerKeyExchange(config *Config, clientHello *clientHelloMsg, serverHello *serverHelloMsg, cert *x509.Certificate, skx *serverKeyExchangeMsg) error {
	if len(skx.key) < 4 {
		return errServerKeyExchange
	}
	if skx.key[0] != 3 { // named curve
		return errors.New("tls: server selected unsupported curve")
	}
	ka.curveID = CurveID(skx.key[1])<<8 | CurveID(skx.key[2])

	publicLen := int(skx.key[3])
	if publicLen+4 > len(skx.key) {
		return errServerKeyExchange
	}
	serverECDHEParams := skx.key[:4+publicLen]
	publicKey := serverECDHEParams[4:]

	sig := skx.key[4+publicLen:]
	if len(sig) < 2 {
		return errServerKeyExchange
	}
	if ka.version >= VersionTLS12 {
		ka.signatureAlgorithm = SignatureScheme(sig[0])<<8 | SignatureScheme(sig[1])
		sig = sig[2:]
		if len(sig) < 2 {
			return errServerKeyExchange
		}
		switch ka.signatureAlgorithm {
		case MLDSA44, MLDSA65, MLDSA87:
			return errors.New("tls: server selected ML-DSA with TLS version < 1.3")
		}
	}
	sigLen := int(sig[0])<<8 | int(sig[1])
	if sigLen+2 != len(sig) {
		return errServerKeyExchange
	}
	sig = sig[2:]

	if !slices.Contains(clientHello.supportedCurves, ka.curveID) {
		return errors.New("tls: server selected unoffered curve")
	}

	if _, ok := curveForCurveID(ka.curveID); !ok {
		return errors.New("tls: server selected unsupported curve")
	}

	key, err := generateECDHEKey(config.rand(), ka.curveID)
	if err != nil {
		return err
	}
	ka.key = key

	peerKey, err := key.Curve().NewPublicKey(publicKey)
	if err != nil {
		return errServerKeyExchange
	}
	ka.preMasterSecret, err = key.ECDH(peerKey)
	if err != nil {
		return errServerKeyExchange
	}

	ourPublicKey := key.PublicKey().Bytes()
	ka.ckx = new(clientKeyExchangeMsg)
	ka.ckx.ciphertext = make([]byte, 1+len(ourPublicKey))
	ka.ckx.ciphertext[0] = byte(len(ourPublicKey))
	copy(ka.ckx.ciphertext[1:], ourPublicKey)

	var sigType uint8
	var sigHash crypto.Hash
	if ka.version >= VersionTLS12 {
		if !isSupportedSignatureAlgorithm(ka.signatureAlgorithm, clientHello.supportedSignatureAlgorithms) {
			return errors.New("tls: certificate used with invalid signature algorithm")
		}
		sigType, sigHash, err = typeAndHashFromSignatureScheme(ka.signatureAlgorithm)
		if err != nil {
			return err
		}
		if sigHash == crypto.SHA1 {
			tlssha1.Value() // ensure godebug is initialized
			tlssha1.IncNonDefault()
		}
		if (sigType == signaturePKCS1v15 || sigType == signatureRSAPSS) != ka.isRSA {
			return errServerKeyExchange
		}
		signed := slices.Concat(clientHello.random, serverHello.random, serverECDHEParams)
		if err := verifyHandshakeSignature(sigType, cert.PublicKey, sigHash, signed, sig); err != nil {
			return errors.New("tls: invalid signature by the server certificate: " + err.Error())
		}
	} else {
		sigType, sigHash, err = legacyTypeAndHashFromPublicKey(cert.PublicKey)
		if err != nil {
			return err
		}
		if (sigType == signaturePKCS1v15) != ka.isRSA {
			return errServerKeyExchange
		}
		signed := hashForServerKeyExchange(sigType, clientHello.random, serverHello.random, serverECDHEParams)
		if err := verifyLegacyHandshakeSignature(sigType, cert.PublicKey, sigHash, signed, sig); err != nil {
			return errors.New("tls: invalid signature by the server certificate: " + err.Error())
		}
	}

	return nil
}

func (ka *ecdheKeyAgreement) generateClientKeyExchange(config *Config, clientHello *clientHelloMsg, cert *x509.Certificate) ([]byte, *clientKeyExchangeMsg, error) {
	if ka.ckx == nil {
		return nil, nil, errors.New("tls: missing ServerKeyExchange message")
	}

	return ka.preMasterSecret, ka.ckx, nil
}

// generateECDHEKey returns a PrivateKey that implements Diffie-Hellman
// according to RFC 8446, Section 4.2.8.2.
func generateECDHEKey(rand io.Reader, curveID CurveID) (*ecdh.PrivateKey, error) {
	curve, ok := curveForCurveID(curveID)
	if !ok {
		return nil, errors.New("tls: internal error: unsupported curve")
	}

	return curve.GenerateKey(rand)
}

func curveForCurveID(id CurveID) (ecdh.Curve, bool) {
	switch id {
	case X25519:
		return ecdh.X25519(), true
	case CurveP256:
		return ecdh.P256(), true
	case CurveP384:
		return ecdh.P384(), true
	case CurveP521:
		return ecdh.P521(), true
	default:
		return nil, false
	}
}

```

// === FILE: references/go/src/crypto/tls/key_schedule.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tls

import (
	"crypto"
	"crypto/ecdh"
	"crypto/fips140"
	"crypto/hmac"
	"crypto/internal/fips140/tls13"
	"crypto/mlkem"
	"errors"
	"hash"
	"io"
)

// This file contains the functions necessary to compute the TLS 1.3 key
// schedule. See RFC 8446, Section 7.

// nextTrafficSecret generates the next traffic secret, given the current one,
// according to RFC 8446, Section 7.2.
func (c *cipherSuiteTLS13) nextTrafficSecret(trafficSecret []byte) []byte {
	return tls13.ExpandLabel(c.hash.New, trafficSecret, "traffic upd", nil, c.hash.Size())
}

// trafficKey generates traffic keys according to RFC 8446, Section 7.3.
func (c *cipherSuiteTLS13) trafficKey(trafficSecret []byte) (key, iv []byte) {
	key = tls13.ExpandLabel(c.hash.New, trafficSecret, "key", nil, c.keyLen)
	iv = tls13.ExpandLabel(c.hash.New, trafficSecret, "iv", nil, aeadNonceLength)
	return
}

// finishedHash generates the Finished verify_data or PskBinderEntry according
// to RFC 8446, Section 4.4.4. See sections 4.4 and 4.2.11.2 for the baseKey
// selection.
func (c *cipherSuiteTLS13) finishedHash(baseKey []byte, transcript hash.Hash) []byte {
	finishedKey := tls13.ExpandLabel(c.hash.New, baseKey, "finished", nil, c.hash.Size())
	verifyData := hmac.New(c.hash.New, finishedKey)
	verifyData.Write(transcript.Sum(nil))
	return verifyData.Sum(nil)
}

// exportKeyingMaterial implements RFC5705 exporters for TLS 1.3 according to
// RFC 8446, Section 7.5.
func (c *cipherSuiteTLS13) exportKeyingMaterial(s *tls13.MasterSecret, transcript hash.Hash) func(string, []byte, int) ([]byte, error) {
	expMasterSecret := s.ExporterMasterSecret(transcript)
	return func(label string, context []byte, length int) ([]byte, error) {
		return expMasterSecret.Exporter(label, context, length), nil
	}
}

type keySharePrivateKeys struct {
	ecdhe *ecdh.PrivateKey
	mlkem crypto.Decapsulator
}

// A keyExchange implements a TLS 1.3 KEM.
type keyExchange interface {
	// keyShares generates one or two key shares.
	//
	// The first one will match the id, the second (if present) reuses the
	// traditional component of the requested hybrid, as allowed by
	// draft-ietf-tls-hybrid-design-09, Section 3.2.
	keyShares(rand io.Reader) (*keySharePrivateKeys, []keyShare, error)

	// serverSharedSecret computes the shared secret and the server's key share.
	serverSharedSecret(rand io.Reader, clientKeyShare []byte) ([]byte, keyShare, error)

	// clientSharedSecret computes the shared secret given the server's key
	// share and the keys generated by keyShares.
	clientSharedSecret(priv *keySharePrivateKeys, serverKeyShare []byte) ([]byte, error)
}

func keyExchangeForCurveID(id CurveID) (keyExchange, error) {
	mlkemGenerateKey768 := func() (crypto.Decapsulator, error) {
		return mlkem.GenerateKey768()
	}
	mlkemGenerateKey1024 := func() (crypto.Decapsulator, error) {
		return mlkem.GenerateKey1024()
	}
	mlkemNewPublicKey768 := func(b []byte) (crypto.Encapsulator, error) {
		return mlkem.NewEncapsulationKey768(b)
	}
	mlkemNewPublicKey1024 := func(b []byte) (crypto.Encapsulator, error) {
		return mlkem.NewEncapsulationKey1024(b)
	}
	switch id {
	case X25519:
		return &ecdhKeyExchange{id, ecdh.X25519()}, nil
	case CurveP256:
		return &ecdhKeyExchange{id, ecdh.P256()}, nil
	case CurveP384:
		return &ecdhKeyExchange{id, ecdh.P384()}, nil
	case CurveP521:
		return &ecdhKeyExchange{id, ecdh.P521()}, nil
	case X25519MLKEM768:
		return &hybridKeyExchange{id, ecdhKeyExchange{X25519, ecdh.X25519()},
			32, mlkem.EncapsulationKeySize768, mlkem.CiphertextSize768,
			mlkemGenerateKey768, mlkemNewPublicKey768}, nil
	case SecP256r1MLKEM768:
		return &hybridKeyExchange{id, ecdhKeyExchange{CurveP256, ecdh.P256()},
			65, mlkem.EncapsulationKeySize768, mlkem.CiphertextSize768,
			mlkemGenerateKey768, mlkemNewPublicKey768}, nil
	case SecP384r1MLKEM1024:
		return &hybridKeyExchange{id, ecdhKeyExchange{CurveP384, ecdh.P384()},
			97, mlkem.EncapsulationKeySize1024, mlkem.CiphertextSize1024,
			mlkemGenerateKey1024, mlkemNewPublicKey1024}, nil
	case MLKEM1024:
		return &mlkem1024KeyExchange{}, nil
	default:
		return nil, errors.New("tls: unsupported key exchange")
	}
}

type mlkem1024KeyExchange struct{}

func (ke *mlkem1024KeyExchange) keyShares(_ io.Reader) (*keySharePrivateKeys, []keyShare, error) {
	priv, err := mlkem.GenerateKey1024()
	if err != nil {
		return nil, nil, err
	}
	return &keySharePrivateKeys{mlkem: priv}, []keyShare{{MLKEM1024, priv.EncapsulationKey().Bytes()}}, nil
}

func (ke *mlkem1024KeyExchange) serverSharedSecret(_ io.Reader, clientKeyShare []byte) ([]byte, keyShare, error) {
	peerKey, err := mlkem.NewEncapsulationKey1024(clientKeyShare)
	if err != nil {
		return nil, keyShare{}, err
	}
	sharedKey, keyShareData := peerKey.Encapsulate()
	return sharedKey, keyShare{MLKEM1024, keyShareData}, nil
}

func (ke *mlkem1024KeyExchange) clientSharedSecret(priv *keySharePrivateKeys, serverKeyShare []byte) ([]byte, error) {
	sharedKey, err := priv.mlkem.Decapsulate(serverKeyShare)
	if err != nil {
		return nil, err
	}
	return sharedKey, nil
}

type ecdhKeyExchange struct {
	id    CurveID
	curve ecdh.Curve
}

func (ke *ecdhKeyExchange) keyShares(rand io.Reader) (*keySharePrivateKeys, []keyShare, error) {
	priv, err := ke.curve.GenerateKey(rand)
	if err != nil {
		return nil, nil, err
	}
	return &keySharePrivateKeys{ecdhe: priv}, []keyShare{{ke.id, priv.PublicKey().Bytes()}}, nil
}

func (ke *ecdhKeyExchange) serverSharedSecret(rand io.Reader, clientKeyShare []byte) ([]byte, keyShare, error) {
	key, err := ke.curve.GenerateKey(rand)
	if err != nil {
		return nil, keyShare{}, err
	}
	peerKey, err := ke.curve.NewPublicKey(clientKeyShare)
	if err != nil {
		return nil, keyShare{}, err
	}
	sharedKey, err := key.ECDH(peerKey)
	if err != nil {
		return nil, keyShare{}, err
	}
	return sharedKey, keyShare{ke.id, key.PublicKey().Bytes()}, nil
}

func (ke *ecdhKeyExchange) clientSharedSecret(priv *keySharePrivateKeys, serverKeyShare []byte) ([]byte, error) {
	peerKey, err := ke.curve.NewPublicKey(serverKeyShare)
	if err != nil {
		return nil, err
	}
	sharedKey, err := priv.ecdhe.ECDH(peerKey)
	if err != nil {
		return nil, err
	}
	return sharedKey, nil
}

type hybridKeyExchange struct {
	id   CurveID
	ecdh ecdhKeyExchange

	ecdhElementSize     int
	mlkemPublicKeySize  int
	mlkemCiphertextSize int

	mlkemGenerateKey  func() (crypto.Decapsulator, error)
	mlkemNewPublicKey func([]byte) (crypto.Encapsulator, error)
}

func (ke *hybridKeyExchange) keyShares(rand io.Reader) (*keySharePrivateKeys, []keyShare, error) {
	var (
		priv       *keySharePrivateKeys
		ecdhShares []keyShare
		err        error
	)
	fips140.WithoutEnforcement(func() { // Hybrid of ML-KEM, which is Approved.
		priv, ecdhShares, err = ke.ecdh.keyShares(rand)
	})
	if err != nil {
		return nil, nil, err
	}
	priv.mlkem, err = ke.mlkemGenerateKey()
	if err != nil {
		return nil, nil, err
	}
	var shareData []byte
	// For X25519MLKEM768, the ML-KEM-768 encapsulation key comes first.
	// For SecP256r1MLKEM768 and SecP384r1MLKEM1024, the ECDH share comes first.
	// See draft-ietf-tls-ecdhe-mlkem-02, Section 4.1.
	if ke.id == X25519MLKEM768 {
		shareData = append(priv.mlkem.Encapsulator().Bytes(), ecdhShares[0].data...)
	} else {
		shareData = append(ecdhShares[0].data, priv.mlkem.Encapsulator().Bytes()...)
	}
	return priv, []keyShare{{ke.id, shareData}, ecdhShares[0]}, nil
}

func (ke *hybridKeyExchange) serverSharedSecret(rand io.Reader, clientKeyShare []byte) ([]byte, keyShare, error) {
	if len(clientKeyShare) != ke.ecdhElementSize+ke.mlkemPublicKeySize {
		return nil, keyShare{}, errors.New("tls: invalid client key share length for hybrid key exchange")
	}
	var ecdhShareData, mlkemShareData []byte
	if ke.id == X25519MLKEM768 {
		mlkemShareData = clientKeyShare[:ke.mlkemPublicKeySize]
		ecdhShareData = clientKeyShare[ke.mlkemPublicKeySize:]
	} else {
		ecdhShareData = clientKeyShare[:ke.ecdhElementSize]
		mlkemShareData = clientKeyShare[ke.ecdhElementSize:]
	}
	var (
		ecdhSharedSecret []byte
		ks               keyShare
		err              error
	)
	fips140.WithoutEnforcement(func() { // Hybrid of ML-KEM, which is Approved.
		ecdhSharedSecret, ks, err = ke.ecdh.serverSharedSecret(rand, ecdhShareData)
	})
	if err != nil {
		return nil, keyShare{}, err
	}
	mlkemPeerKey, err := ke.mlkemNewPublicKey(mlkemShareData)
	if err != nil {
		return nil, keyShare{}, err
	}
	mlkemSharedSecret, mlkemKeyShare := mlkemPeerKey.Encapsulate()
	var sharedKey []byte
	if ke.id == X25519MLKEM768 {
		sharedKey = append(mlkemSharedSecret, ecdhSharedSecret...)
		ks.data = append(mlkemKeyShare, ks.data...)
	} else {
		sharedKey = append(ecdhSharedSecret, mlkemSharedSecret...)
		ks.data = append(ks.data, mlkemKeyShare...)
	}
	ks.group = ke.id
	return sharedKey, ks, nil
}

func (ke *hybridKeyExchange) clientSharedSecret(priv *keySharePrivateKeys, serverKeyShare []byte) ([]byte, error) {
	if len(serverKeyShare) != ke.ecdhElementSize+ke.mlkemCiphertextSize {
		return nil, errors.New("tls: invalid server key share length for hybrid key exchange")
	}
	var ecdhShareData, mlkemShareData []byte
	if ke.id == X25519MLKEM768 {
		mlkemShareData = serverKeyShare[:ke.mlkemCiphertextSize]
		ecdhShareData = serverKeyShare[ke.mlkemCiphertextSize:]
	} else {
		ecdhShareData = serverKeyShare[:ke.ecdhElementSize]
		mlkemShareData = serverKeyShare[ke.ecdhElementSize:]
	}
	var (
		ecdhSharedSecret []byte
		err              error
	)
	fips140.WithoutEnforcement(func() { // Hybrid of ML-KEM, which is Approved.
		ecdhSharedSecret, err = ke.ecdh.clientSharedSecret(priv, ecdhShareData)
	})
	if err != nil {
		return nil, err
	}
	mlkemSharedSecret, err := priv.mlkem.Decapsulate(mlkemShareData)
	if err != nil {
		return nil, err
	}
	var sharedKey []byte
	if ke.id == X25519MLKEM768 {
		sharedKey = append(mlkemSharedSecret, ecdhSharedSecret...)
	} else {
		sharedKey = append(ecdhSharedSecret, mlkemSharedSecret...)
	}
	return sharedKey, nil
}

```

// === FILE: references/go/src/crypto/tls/prf.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tls

import (
	"crypto"
	"crypto/hmac"
	"crypto/internal/fips140/tls12"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"fmt"
	"hash"
)

type prfFunc func(secret []byte, label string, seed []byte, keyLen int) []byte

// Split a premaster secret in two as specified in RFC 4346, Section 5.
func splitPreMasterSecret(secret []byte) (s1, s2 []byte) {
	s1 = secret[0 : (len(secret)+1)/2]
	s2 = secret[len(secret)/2:]
	return
}

// pHash implements the P_hash function, as defined in RFC 4346, Section 5.
func pHash(result, secret, seed []byte, hash func() hash.Hash) {
	h := hmac.New(hash, secret)
	h.Write(seed)
	a := h.Sum(nil)

	j := 0
	for j < len(result) {
		h.Reset()
		h.Write(a)
		h.Write(seed)
		b := h.Sum(nil)
		copy(result[j:], b)
		j += len(b)

		h.Reset()
		h.Write(a)
		a = h.Sum(nil)
	}
}

// prf10 implements the TLS 1.0 pseudo-random function, as defined in RFC 2246, Section 5.
func prf10(secret []byte, label string, seed []byte, keyLen int) []byte {
	result := make([]byte, keyLen)
	hashSHA1 := sha1.New
	hashMD5 := md5.New

	labelAndSeed := make([]byte, len(label)+len(seed))
	copy(labelAndSeed, label)
	copy(labelAndSeed[len(label):], seed)

	s1, s2 := splitPreMasterSecret(secret)
	pHash(result, s1, labelAndSeed, hashMD5)
	result2 := make([]byte, len(result))
	pHash(result2, s2, labelAndSeed, hashSHA1)

	for i, b := range result2 {
		result[i] ^= b
	}

	return result
}

// prf12 implements the TLS 1.2 pseudo-random function, as defined in RFC 5246, Section 5.
func prf12(hashFunc func() hash.Hash) prfFunc {
	return func(secret []byte, label string, seed []byte, keyLen int) []byte {
		return tls12.PRF(hashFunc, secret, label, seed, keyLen)
	}
}

const (
	masterSecretLength   = 48 // Length of a master secret in TLS 1.1.
	finishedVerifyLength = 12 // Length of verify_data in a Finished message.
)

const masterSecretLabel = "master secret"
const extendedMasterSecretLabel = "extended master secret"
const keyExpansionLabel = "key expansion"
const clientFinishedLabel = "client finished"
const serverFinishedLabel = "server finished"

func prfAndHashForVersion(version uint16, suite *cipherSuite) (prfFunc, crypto.Hash) {
	switch version {
	case VersionTLS10, VersionTLS11:
		return prf10, crypto.Hash(0)
	case VersionTLS12:
		if suite.flags&suiteSHA384 != 0 {
			return prf12(sha512.New384), crypto.SHA384
		}
		return prf12(sha256.New), crypto.SHA256
	default:
		panic("unknown version")
	}
}

func prfForVersion(version uint16, suite *cipherSuite) prfFunc {
	prf, _ := prfAndHashForVersion(version, suite)
	return prf
}

// masterFromPreMasterSecret generates the master secret from the pre-master
// secret. See RFC 5246, Section 8.1.
func masterFromPreMasterSecret(version uint16, suite *cipherSuite, preMasterSecret, clientRandom, serverRandom []byte) []byte {
	seed := make([]byte, 0, len(clientRandom)+len(serverRandom))
	seed = append(seed, clientRandom...)
	seed = append(seed, serverRandom...)

	return prfForVersion(version, suite)(preMasterSecret, masterSecretLabel, seed, masterSecretLength)
}

// extMasterFromPreMasterSecret generates the extended master secret from the
// pre-master secret. See RFC 7627.
func extMasterFromPreMasterSecret(version uint16, suite *cipherSuite, preMasterSecret, transcript []byte) []byte {
	prf, hash := prfAndHashForVersion(version, suite)
	if version == VersionTLS12 {
		// Use the FIPS 140-3 module only for TLS 1.2 with EMS, which is the
		// only TLS 1.0-1.2 approved mode per IG D.Q.
		return tls12.MasterSecret(hash.New, preMasterSecret, transcript)
	}
	return prf(preMasterSecret, extendedMasterSecretLabel, transcript, masterSecretLength)
}

// keysFromMasterSecret generates the connection keys from the master
// secret, given the lengths of the MAC key, cipher key and IV, as defined in
// RFC 2246, Section 6.3.
func keysFromMasterSecret(version uint16, suite *cipherSuite, masterSecret, clientRandom, serverRandom []byte, macLen, keyLen, ivLen int) (clientMAC, serverMAC, clientKey, serverKey, clientIV, serverIV []byte) {
	seed := make([]byte, 0, len(serverRandom)+len(clientRandom))
	seed = append(seed, serverRandom...)
	seed = append(seed, clientRandom...)

	n := 2*macLen + 2*keyLen + 2*ivLen
	keyMaterial := prfForVersion(version, suite)(masterSecret, keyExpansionLabel, seed, n)
	clientMAC = keyMaterial[:macLen]
	keyMaterial = keyMaterial[macLen:]
	serverMAC = keyMaterial[:macLen]
	keyMaterial = keyMaterial[macLen:]
	clientKey = keyMaterial[:keyLen]
	keyMaterial = keyMaterial[keyLen:]
	serverKey = keyMaterial[:keyLen]
	keyMaterial = keyMaterial[keyLen:]
	clientIV = keyMaterial[:ivLen]
	keyMaterial = keyMaterial[ivLen:]
	serverIV = keyMaterial[:ivLen]
	return
}

func newFinishedHash(version uint16, cipherSuite *cipherSuite) finishedHash {
	var buffer []byte
	if version >= VersionTLS12 {
		buffer = []byte{}
	}

	prf, hash := prfAndHashForVersion(version, cipherSuite)
	if hash != 0 {
		return finishedHash{hash.New(), hash.New(), nil, nil, buffer, version, prf}
	}

	return finishedHash{sha1.New(), sha1.New(), md5.New(), md5.New(), buffer, version, prf}
}

// A finishedHash calculates the hash of a set of handshake messages suitable
// for including in a Finished message.
type finishedHash struct {
	client hash.Hash
	server hash.Hash

	// Prior to TLS 1.2, an additional MD5 hash is required.
	clientMD5 hash.Hash
	serverMD5 hash.Hash

	// In TLS 1.2, a full buffer is sadly required.
	buffer []byte

	version uint16
	prf     prfFunc
}

func (h *finishedHash) Write(msg []byte) (n int, err error) {
	h.client.Write(msg)
	h.server.Write(msg)

	if h.version < VersionTLS12 {
		h.clientMD5.Write(msg)
		h.serverMD5.Write(msg)
	}

	if h.buffer != nil {
		h.buffer = append(h.buffer, msg...)
	}

	return len(msg), nil
}

func (h finishedHash) Sum() []byte {
	if h.version >= VersionTLS12 {
		return h.client.Sum(nil)
	}

	out := make([]byte, 0, md5.Size+sha1.Size)
	out = h.clientMD5.Sum(out)
	return h.client.Sum(out)
}

// clientSum returns the contents of the verify_data member of a client's
// Finished message.
func (h finishedHash) clientSum(masterSecret []byte) []byte {
	return h.prf(masterSecret, clientFinishedLabel, h.Sum(), finishedVerifyLength)
}

// serverSum returns the contents of the verify_data member of a server's
// Finished message.
func (h finishedHash) serverSum(masterSecret []byte) []byte {
	return h.prf(masterSecret, serverFinishedLabel, h.Sum(), finishedVerifyLength)
}

// hashForClientCertificate returns the handshake messages so far, pre-hashed,
// suitable for signing by a TLS 1.0 and 1.1 client certificate.
func (h finishedHash) hashForClientCertificate(sigType uint8) []byte {
	if sigType == signatureECDSA {
		return h.server.Sum(nil)
	}

	return h.Sum()
}

// discardHandshakeBuffer is called when there is no more need to
// buffer the entirety of the handshake messages.
func (h *finishedHash) discardHandshakeBuffer() {
	h.buffer = nil
}

// noEKMBecauseRenegotiation is used as a value of
// ConnectionState.ekm when renegotiation is enabled and thus
// we wish to fail all key-material export requests.
func noEKMBecauseRenegotiation(label string, context []byte, length int) ([]byte, error) {
	return nil, errors.New("crypto/tls: ExportKeyingMaterial is unavailable when renegotiation is enabled")
}

// noEKMBecauseNoEMS is used as a value of ConnectionState.ekm when Extended
// Master Secret is not negotiated and thus we wish to fail all key-material
// export requests.
func noEKMBecauseNoEMS(label string, context []byte, length int) ([]byte, error) {
	return nil, errors.New("crypto/tls: ExportKeyingMaterial is unavailable when neither TLS 1.3 nor Extended Master Secret are negotiated")
}

// ekmFromMasterSecret generates exported keying material as defined in RFC 5705.
func ekmFromMasterSecret(version uint16, suite *cipherSuite, masterSecret, clientRandom, serverRandom []byte) func(string, []byte, int) ([]byte, error) {
	return func(label string, context []byte, length int) ([]byte, error) {
		switch label {
		case "client finished", "server finished", "master secret", "key expansion":
			// These values are reserved and may not be used.
			return nil, fmt.Errorf("crypto/tls: reserved ExportKeyingMaterial label: %s", label)
		}

		seedLen := len(serverRandom) + len(clientRandom)
		if context != nil {
			seedLen += 2 + len(context)
		}
		seed := make([]byte, 0, seedLen)

		seed = append(seed, clientRandom...)
		seed = append(seed, serverRandom...)

		if context != nil {
			if len(context) >= 1<<16 {
				return nil, fmt.Errorf("crypto/tls: ExportKeyingMaterial context too long")
			}
			seed = append(seed, byte(len(context)>>8), byte(len(context)))
			seed = append(seed, context...)
		}

		return prfForVersion(version, suite)(masterSecret, label, seed, length), nil
	}
}

```

// === FILE: references/go/src/crypto/tls/quic.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tls

import (
	"context"
	"errors"
	"fmt"
	"net"
)

// QUICEncryptionLevel represents a QUIC encryption level used to transmit
// handshake messages.
type QUICEncryptionLevel int

const (
	QUICEncryptionLevelInitial = QUICEncryptionLevel(iota)
	QUICEncryptionLevelEarly
	QUICEncryptionLevelHandshake
	QUICEncryptionLevelApplication
)

func (l QUICEncryptionLevel) String() string {
	switch l {
	case QUICEncryptionLevelInitial:
		return "Initial"
	case QUICEncryptionLevelEarly:
		return "Early"
	case QUICEncryptionLevelHandshake:
		return "Handshake"
	case QUICEncryptionLevelApplication:
		return "Application"
	default:
		return fmt.Sprintf("QUICEncryptionLevel(%v)", int(l))
	}
}

// A QUICConn represents a connection which uses a QUIC implementation as the underlying
// transport as described in RFC 9001.
//
// Methods of QUICConn are not safe for concurrent use.
type QUICConn struct {
	conn *Conn

	sessionTicketSent bool
}

// A QUICConfig configures a [QUICConn].
type QUICConfig struct {
	TLSConfig *Config

	// EnableSessionEvents may be set to true to enable the
	// [QUICStoreSession] and [QUICResumeSession] events for client connections.
	// When this event is enabled, sessions are not automatically
	// stored in the client session cache.
	// The application should use [QUICConn.StoreSession] to store sessions.
	EnableSessionEvents bool

	// ClientHelloInfoConn is the net.Conn to use for the ClientHelloInfo.Conn field.
	ClientHelloInfoConn net.Conn
}

// A QUICEventKind is a type of operation on a QUIC connection.
type QUICEventKind int

const (
	// QUICNoEvent indicates that there are no events available.
	QUICNoEvent QUICEventKind = iota

	// QUICSetReadSecret and QUICSetWriteSecret provide the read and write
	// secrets for a given encryption level.
	// QUICEvent.Level, QUICEvent.Data, and QUICEvent.Suite are set.
	//
	// Secrets for the Initial encryption level are derived from the initial
	// destination connection ID, and are not provided by the QUICConn.
	QUICSetReadSecret
	QUICSetWriteSecret

	// QUICWriteData provides data to send to the peer in CRYPTO frames.
	// QUICEvent.Data is set.
	QUICWriteData

	// QUICTransportParameters provides the peer's QUIC transport parameters.
	// QUICEvent.Data is set.
	QUICTransportParameters

	// QUICTransportParametersRequired indicates that the caller must provide
	// QUIC transport parameters to send to the peer. The caller should set
	// the transport parameters with QUICConn.SetTransportParameters and call
	// QUICConn.NextEvent again.
	//
	// If transport parameters are set before calling QUICConn.Start, the
	// connection will never generate a QUICTransportParametersRequired event.
	QUICTransportParametersRequired

	// QUICRejectedEarlyData indicates that the server rejected 0-RTT data even
	// if we offered it. It's returned before QUICEncryptionLevelApplication
	// keys are returned.
	// This event only occurs on client connections.
	QUICRejectedEarlyData

	// QUICHandshakeDone indicates that the TLS handshake has completed.
	QUICHandshakeDone

	// QUICResumeSession indicates that a client is attempting to resume a previous session.
	// [QUICEvent.SessionState] is set.
	//
	// For client connections, this event occurs when the session ticket is selected.
	// For server connections, this event occurs when receiving the client's session ticket.
	//
	// The application may set [QUICEvent.SessionState.EarlyData] to false before the
	// next call to [QUICConn.NextEvent] to decline 0-RTT even if the session supports it.
	QUICResumeSession

	// QUICStoreSession indicates that the server has provided state permitting
	// the client to resume the session.
	// [QUICEvent.SessionState] is set.
	// The application should use [QUICConn.StoreSession] session to store the [SessionState].
	// The application may modify the [SessionState] before storing it.
	// This event only occurs on client connections.
	QUICStoreSession

	// QUICErrorEvent indicates that a fatal error has occurred.
	// The handshake cannot proceed and the connection must be closed.
	// QUICEvent.Err is set.
	QUICErrorEvent
)

// A QUICEvent is an event occurring on a QUIC connection.
//
// The type of event is specified by the Kind field.
// The contents of the other fields are kind-specific.
type QUICEvent struct {
	Kind QUICEventKind

	// Set for QUICSetReadSecret, QUICSetWriteSecret, and QUICWriteData.
	Level QUICEncryptionLevel

	// Set for QUICTransportParameters, QUICSetReadSecret, QUICSetWriteSecret, and QUICWriteData.
	// The contents are owned by crypto/tls, and are valid until the next NextEvent call.
	Data []byte

	// Set for QUICSetReadSecret and QUICSetWriteSecret.
	Suite uint16

	// Set for QUICResumeSession and QUICStoreSession.
	SessionState *SessionState

	// Set for QUICErrorEvent.
	// The error will wrap AlertError.
	Err error
}

type quicState struct {
	events    []QUICEvent
	nextEvent int

	// eventArr is a statically allocated event array, large enough to handle
	// the usual maximum number of events resulting from a single call: transport
	// parameters, Initial data, Early read secret, Handshake write and read
	// secrets, Handshake data, Application write secret, Application data.
	eventArr [8]QUICEvent

	started  bool
	signalc  chan struct{}   // handshake data is available to be read
	blockedc chan struct{}   // handshake is waiting for data, closed when done
	ctx      context.Context // handshake context
	cancel   context.CancelFunc

	waitingForDrain bool
	errorReturned   bool

	// readbuf is shared between HandleData and the handshake goroutine.
	// HandshakeCryptoData passes ownership to the handshake goroutine by
	// reading from signalc, and reclaims ownership by reading from blockedc.
	readbuf []byte

	transportParams []byte // to send to the peer

	enableSessionEvents bool
	clientHelloInfoConn net.Conn
}

// QUICClient returns a new TLS client side connection using QUICTransport as the
// underlying transport. The config cannot be nil.
func QUICClient(config *QUICConfig) *QUICConn {
	return newQUICConn(Client(nil, config.TLSConfig), config)
}

// QUICServer returns a new TLS server side connection using QUICTransport as the
// underlying transport. The config cannot be nil.
func QUICServer(config *QUICConfig) *QUICConn {
	return newQUICConn(Server(nil, config.TLSConfig), config)
}

func newQUICConn(conn *Conn, config *QUICConfig) *QUICConn {
	conn.quic = &quicState{
		signalc:             make(chan struct{}),
		blockedc:            make(chan struct{}),
		enableSessionEvents: config.EnableSessionEvents,
		clientHelloInfoConn: config.ClientHelloInfoConn,
	}
	conn.quic.events = conn.quic.eventArr[:0]
	return &QUICConn{
		conn: conn,
	}
}

// Start starts the client or server handshake protocol.
// It may produce connection events, which may be read with [QUICConn.NextEvent].
//
// Start must be called at most once.
func (q *QUICConn) Start(ctx context.Context) error {
	if q.conn.quic.started {
		return quicError(errors.New("tls: Start called more than once"))
	}
	q.conn.quic.started = true
	go q.conn.HandshakeContext(ctx)
	if _, ok := <-q.conn.quic.blockedc; !ok {
		return q.conn.handshakeErr
	}
	return nil
}

// NextEvent returns the next event occurring on the connection.
// It returns an event with a Kind of [QUICNoEvent] when no events are available.
func (q *QUICConn) NextEvent() QUICEvent {
	qs := q.conn.quic
	if last := qs.nextEvent - 1; last >= 0 && len(qs.events[last].Data) > 0 {
		// Write over some of the previous event's data,
		// to catch callers erroneously retaining it.
		qs.events[last].Data[0] = 0
	}
	if qs.nextEvent >= len(qs.events) && qs.waitingForDrain {
		qs.waitingForDrain = false
		<-qs.signalc
		<-qs.blockedc
	}
	if err := q.conn.handshakeErr; err != nil {
		if qs.errorReturned {
			return QUICEvent{Kind: QUICNoEvent}
		}
		qs.errorReturned = true
		qs.events = nil
		qs.nextEvent = 0
		return QUICEvent{Kind: QUICErrorEvent, Err: q.conn.handshakeErr}
	}
	if qs.nextEvent >= len(qs.events) {
		qs.events = qs.events[:0]
		qs.nextEvent = 0
		return QUICEvent{Kind: QUICNoEvent}
	}
	e := qs.events[qs.nextEvent]
	qs.events[qs.nextEvent] = QUICEvent{} // zero out references to data
	qs.nextEvent++
	return e
}

// Close closes the connection and stops any in-progress handshake.
func (q *QUICConn) Close() error {
	if q.conn.quic.ctx == nil {
		return nil // never started
	}
	q.conn.quic.cancel()
	<-q.conn.quic.signalc
	for range q.conn.quic.blockedc {
		// Wait for the handshake goroutine to return.
	}
	return q.conn.handshakeErr
}

// HandleData handles handshake bytes received from the peer.
// It may produce connection events, which may be read with [QUICConn.NextEvent].
func (q *QUICConn) HandleData(level QUICEncryptionLevel, data []byte) error {
	c := q.conn
	if c.in.level != level {
		return quicError(c.in.setErrorLocked(errors.New("tls: handshake data received at wrong level")))
	}
	c.quic.readbuf = data
	<-c.quic.signalc
	_, ok := <-c.quic.blockedc
	if ok {
		// The handshake goroutine is waiting for more data.
		return nil
	}
	// The handshake goroutine has exited.
	c.handshakeMutex.Lock()
	defer c.handshakeMutex.Unlock()
	c.hand.Write(c.quic.readbuf)
	c.quic.readbuf = nil
	for q.conn.hand.Len() >= 4 && q.conn.handshakeErr == nil {
		b := q.conn.hand.Bytes()
		n := int(b[1])<<16 | int(b[2])<<8 | int(b[3])
		if n > maxHandshake {
			q.conn.handshakeErr = fmt.Errorf("tls: handshake message of length %d bytes exceeds maximum of %d bytes", n, maxHandshake)
			break
		}
		if len(b) < 4+n {
			return nil
		}
		if err := q.conn.handlePostHandshakeMessage(); err != nil {
			q.conn.handshakeErr = err
		}
	}
	if q.conn.handshakeErr != nil {
		return quicError(q.conn.handshakeErr)
	}
	return nil
}

type QUICSessionTicketOptions struct {
	// EarlyData specifies whether the ticket may be used for 0-RTT.
	EarlyData bool
	Extra     [][]byte
}

// SendSessionTicket sends a session ticket to the client.
// It produces connection events, which may be read with [QUICConn.NextEvent].
// Currently, it can only be called once.
func (q *QUICConn) SendSessionTicket(opts QUICSessionTicketOptions) error {
	c := q.conn
	if c.config.SessionTicketsDisabled {
		return nil
	}
	if !c.isHandshakeComplete.Load() {
		return quicError(errors.New("tls: SendSessionTicket called before handshake completed"))
	}
	if c.isClient {
		return quicError(errors.New("tls: SendSessionTicket called on the client"))
	}
	if q.sessionTicketSent {
		return quicError(errors.New("tls: SendSessionTicket called multiple times"))
	}
	q.sessionTicketSent = true
	return quicError(c.sendSessionTicket(opts.EarlyData, opts.Extra))
}

// StoreSession stores a session previously received in a QUICStoreSession event
// in the ClientSessionCache.
// The application may process additional events or modify the SessionState
// before storing the session.
func (q *QUICConn) StoreSession(session *SessionState) error {
	c := q.conn
	if !c.isClient {
		return quicError(errors.New("tls: StoreSessionTicket called on the server"))
	}
	cacheKey := c.clientSessionCacheKey()
	if cacheKey == "" {
		return nil
	}
	cs := &ClientSessionState{session: session}
	c.config.ClientSessionCache.Put(cacheKey, cs)
	return nil
}

// ConnectionState returns basic TLS details about the connection.
func (q *QUICConn) ConnectionState() ConnectionState {
	return q.conn.ConnectionState()
}

// SetTransportParameters sets the transport parameters to send to the peer.
//
// Server connections may delay setting the transport parameters until after
// receiving the client's transport parameters. See [QUICTransportParametersRequired].
func (q *QUICConn) SetTransportParameters(params []byte) {
	if params == nil {
		params = []byte{}
	}
	q.conn.quic.transportParams = params
	if q.conn.quic.started {
		<-q.conn.quic.signalc
		<-q.conn.quic.blockedc
	}
}

// quicError ensures err is an AlertError.
// If err is not already, quicError wraps it with alertInternalError.
func quicError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[AlertError](err); ok {
		return err
	}
	a, ok := errors.AsType[alert](err)
	if !ok {
		a = alertInternalError
	}
	// Return an error wrapping the original error and an AlertError.
	// Truncate the text of the alert to 0 characters.
	return fmt.Errorf("%w%.0w", err, AlertError(a))
}

func (c *Conn) quicReadHandshakeBytes(n int) error {
	for c.hand.Len() < n {
		if err := c.quicWaitForSignal(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Conn) quicSetReadSecret(level QUICEncryptionLevel, suite uint16, secret []byte) error {
	// Ensure that there are no buffered handshake messages before changing the
	// read keys, since that can cause messages to be parsed that were encrypted
	// using old keys which are no longer appropriate.
	// TODO(roland): we should merge this check with the similar one in setReadTrafficSecret.
	if c.hand.Len() != 0 {
		c.sendAlert(alertUnexpectedMessage)
		return errors.New("tls: handshake buffer not empty before setting read traffic secret")
	}
	c.quic.events = append(c.quic.events, QUICEvent{
		Kind:  QUICSetReadSecret,
		Level: level,
		Suite: suite,
		Data:  secret,
	})
	return nil
}

func (c *Conn) quicSetWriteSecret(level QUICEncryptionLevel, suite uint16, secret []byte) {
	c.quic.events = append(c.quic.events, QUICEvent{
		Kind:  QUICSetWriteSecret,
		Level: level,
		Suite: suite,
		Data:  secret,
	})
}

func (c *Conn) quicWriteCryptoData(level QUICEncryptionLevel, data []byte) {
	var last *QUICEvent
	if len(c.quic.events) > 0 {
		last = &c.quic.events[len(c.quic.events)-1]
	}
	if last == nil || last.Kind != QUICWriteData || last.Level != level {
		c.quic.events = append(c.quic.events, QUICEvent{
			Kind:  QUICWriteData,
			Level: level,
		})
		last = &c.quic.events[len(c.quic.events)-1]
	}
	last.Data = append(last.Data, data...)
}

func (c *Conn) quicResumeSession(session *SessionState) error {
	c.quic.events = append(c.quic.events, QUICEvent{
		Kind:         QUICResumeSession,
		SessionState: session,
	})
	c.quic.waitingForDrain = true
	for c.quic.waitingForDrain {
		if err := c.quicWaitForSignal(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Conn) quicStoreSession(session *SessionState) {
	c.quic.events = append(c.quic.events, QUICEvent{
		Kind:         QUICStoreSession,
		SessionState: session,
	})
}

func (c *Conn) quicSetTransportParameters(params []byte) {
	c.quic.events = append(c.quic.events, QUICEvent{
		Kind: QUICTransportParameters,
		Data: params,
	})
}

func (c *Conn) quicGetTransportParameters() ([]byte, error) {
	if c.quic.transportParams == nil {
		c.quic.events = append(c.quic.events, QUICEvent{
			Kind: QUICTransportParametersRequired,
		})
	}
	for c.quic.transportParams == nil {
		if err := c.quicWaitForSignal(); err != nil {
			return nil, err
		}
	}
	return c.quic.transportParams, nil
}

func (c *Conn) quicHandshakeComplete() {
	c.quic.events = append(c.quic.events, QUICEvent{
		Kind: QUICHandshakeDone,
	})
}

func (c *Conn) quicRejectedEarlyData() {
	c.quic.events = append(c.quic.events, QUICEvent{
		Kind: QUICRejectedEarlyData,
	})
}

// quicWaitForSignal notifies the QUICConn that handshake progress is blocked,
// and waits for a signal that the handshake should proceed.
//
// The handshake may become blocked waiting for handshake bytes
// or for the user to provide transport parameters.
func (c *Conn) quicWaitForSignal() error {
	// Drop the handshake mutex while blocked to allow the user
	// to call ConnectionState before the handshake completes.
	c.handshakeMutex.Unlock()
	defer c.handshakeMutex.Lock()
	// Send on blockedc to notify the QUICConn that the handshake is blocked.
	// Exported methods of QUICConn wait for the handshake to become blocked
	// before returning to the user.
	c.quic.blockedc <- struct{}{}
	// The QUICConn reads from signalc to notify us that the handshake may
	// be able to proceed. (The QUICConn reads, because we close signalc to
	// indicate that the handshake has completed.)
	c.quic.signalc <- struct{}{}
	if c.quic.ctx.Err() != nil {
		// The connection has been canceled.
		return c.sendAlertLocked(alertCloseNotify)
	}
	c.hand.Write(c.quic.readbuf)
	c.quic.readbuf = nil
	return nil
}

```

// === FILE: references/go/src/crypto/tls/ticket.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tls

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"errors"
	"io"

	"golang.org/x/crypto/cryptobyte"
)

// A SessionState is a resumable session.
type SessionState struct {
	// Encoded as a SessionState (in the language of RFC 8446, Section 3).
	//
	//   enum { server(1), client(2) } SessionStateType;
	//
	//   opaque Certificate<1..2^24-1>;
	//
	//   Certificate CertificateChain<0..2^24-1>;
	//
	//   opaque Extra<0..2^24-1>;
	//
	//   struct {
	//       uint16 version;
	//       SessionStateType type;
	//       uint16 cipher_suite;
	//       uint64 created_at;
	//       opaque secret<1..2^8-1>;
	//       Extra extra<0..2^24-1>;
	//       uint8 ext_master_secret = { 0, 1 };
	//       uint8 early_data = { 0, 1 };
	//       CertificateEntry certificate_list<0..2^24-1>;
	//       CertificateChain verified_chains<0..2^24-1>; /* excluding leaf */
	//       select (SessionState.early_data) {
	//           case 0: Empty;
	//           case 1: opaque alpn<1..2^8-1>;
	//       };
	//       select (SessionState.version) {
	//           case VersionTLS10..VersionTLS12: uint16 curve_id;
	//           case VersionTLS13: select (SessionState.type) {
	//               case server: Empty;
	//               case client: struct {
	//                   uint64 use_by;
	//                   uint32 age_add;
	//               };
	//           };
	//       };
	//   } SessionState;
	//
	// The format can be extended backwards-compatibly by adding new fields at
	// the end. Otherwise, a new SessionStateType must be used, as different Go
	// versions may share the same session ticket encryption key.

	// Extra is ignored by crypto/tls, but is encoded by [SessionState.Bytes]
	// and parsed by [ParseSessionState].
	//
	// This allows [Config.UnwrapSession]/[Config.WrapSession] and
	// [ClientSessionCache] implementations to store and retrieve additional
	// data alongside this session.
	//
	// To allow different layers in a protocol stack to share this field,
	// applications must only append to it, not replace it, and must use entries
	// that can be recognized even if out of order (for example, by starting
	// with an id and version prefix).
	Extra [][]byte

	// EarlyData indicates whether the ticket can be used for 0-RTT in a QUIC
	// connection. The application may set this to false if it is true to
	// decline to offer 0-RTT even if supported.
	EarlyData bool

	version     uint16
	isClient    bool
	cipherSuite uint16
	// createdAt is the generation time of the secret on the server (which for
	// TLS 1.0–1.2 might be earlier than the current session) and the time at
	// which the ticket was received on the client.
	createdAt        uint64 // seconds since UNIX epoch
	secret           []byte // master secret for TLS 1.2, or the PSK for TLS 1.3
	extMasterSecret  bool
	peerCertificates []*x509.Certificate
	ocspResponse     []byte
	scts             [][]byte
	verifiedChains   [][]*x509.Certificate
	alpnProtocol     string // only set if EarlyData is true

	// Client-side TLS 1.3-only fields.
	useBy  uint64 // seconds since UNIX epoch
	ageAdd uint32
	ticket []byte

	// TLS 1.0–1.2 only fields.
	curveID CurveID
}

// Bytes encodes the session, including any private fields, so that it can be
// parsed by [ParseSessionState]. The encoding contains secret values critical
// to the security of future and possibly past sessions.
//
// The specific encoding should be considered opaque and may change incompatibly
// between Go versions.
func (s *SessionState) Bytes() ([]byte, error) {
	var b cryptobyte.Builder
	b.AddUint16(s.version)
	if s.isClient {
		b.AddUint8(2) // client
	} else {
		b.AddUint8(1) // server
	}
	b.AddUint16(s.cipherSuite)
	addUint64(&b, s.createdAt)
	b.AddUint8LengthPrefixed(func(b *cryptobyte.Builder) {
		b.AddBytes(s.secret)
	})
	b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
		for _, extra := range s.Extra {
			b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
				b.AddBytes(extra)
			})
		}
	})
	if s.extMasterSecret {
		b.AddUint8(1)
	} else {
		b.AddUint8(0)
	}
	if s.EarlyData {
		b.AddUint8(1)
	} else {
		b.AddUint8(0)
	}
	marshalCertificate(&b, Certificate{
		Certificate:                 certificatesToBytesSlice(s.peerCertificates),
		OCSPStaple:                  s.ocspResponse,
		SignedCertificateTimestamps: s.scts,
	})
	b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
		for _, chain := range s.verifiedChains {
			b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
				// We elide the first certificate because it's always the leaf.
				if len(chain) == 0 {
					b.SetError(errors.New("tls: internal error: empty verified chain"))
					return
				}
				for _, cert := range chain[1:] {
					b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
						b.AddBytes(cert.Raw)
					})
				}
			})
		}
	})
	if s.EarlyData {
		b.AddUint8LengthPrefixed(func(b *cryptobyte.Builder) {
			b.AddBytes([]byte(s.alpnProtocol))
		})
	}
	if s.version >= VersionTLS13 {
		if s.isClient {
			addUint64(&b, s.useBy)
			b.AddUint32(s.ageAdd)
		}
	} else {
		b.AddUint16(uint16(s.curveID))
	}
	return b.Bytes()
}

func certificatesToBytesSlice(certs []*x509.Certificate) [][]byte {
	s := make([][]byte, 0, len(certs))
	for _, c := range certs {
		s = append(s, c.Raw)
	}
	return s
}

// ParseSessionState parses a [SessionState] encoded by [SessionState.Bytes].
func ParseSessionState(data []byte) (*SessionState, error) {
	ss := &SessionState{}
	s := cryptobyte.String(data)
	var typ, extMasterSecret, earlyData uint8
	var cert Certificate
	var extra cryptobyte.String
	if !s.ReadUint16(&ss.version) ||
		!s.ReadUint8(&typ) ||
		!s.ReadUint16(&ss.cipherSuite) ||
		!readUint64(&s, &ss.createdAt) ||
		!readUint8LengthPrefixed(&s, &ss.secret) ||
		!s.ReadUint24LengthPrefixed(&extra) ||
		!s.ReadUint8(&extMasterSecret) ||
		!s.ReadUint8(&earlyData) ||
		len(ss.secret) == 0 ||
		!unmarshalCertificate(&s, &cert) {
		return nil, errors.New("tls: invalid session encoding")
	}
	for !extra.Empty() {
		var e []byte
		if !readUint24LengthPrefixed(&extra, &e) {
			return nil, errors.New("tls: invalid session encoding")
		}
		ss.Extra = append(ss.Extra, e)
	}
	switch typ {
	case 1:
		ss.isClient = false
	case 2:
		ss.isClient = true
	default:
		return nil, errors.New("tls: unknown session encoding")
	}
	switch extMasterSecret {
	case 0:
		ss.extMasterSecret = false
	case 1:
		ss.extMasterSecret = true
	default:
		return nil, errors.New("tls: invalid session encoding")
	}
	switch earlyData {
	case 0:
		ss.EarlyData = false
	case 1:
		ss.EarlyData = true
	default:
		return nil, errors.New("tls: invalid session encoding")
	}
	for _, cert := range cert.Certificate {
		c, err := globalCertCache.newCert(cert)
		if err != nil {
			return nil, err
		}
		ss.peerCertificates = append(ss.peerCertificates, c)
	}
	if ss.isClient && len(ss.peerCertificates) == 0 {
		return nil, errors.New("tls: no server certificates in client session")
	}
	ss.ocspResponse = cert.OCSPStaple
	ss.scts = cert.SignedCertificateTimestamps
	var chainList cryptobyte.String
	if !s.ReadUint24LengthPrefixed(&chainList) {
		return nil, errors.New("tls: invalid session encoding")
	}
	for !chainList.Empty() {
		var certList cryptobyte.String
		if !chainList.ReadUint24LengthPrefixed(&certList) {
			return nil, errors.New("tls: invalid session encoding")
		}
		var chain []*x509.Certificate
		if len(ss.peerCertificates) == 0 {
			return nil, errors.New("tls: invalid session encoding")
		}
		chain = append(chain, ss.peerCertificates[0])
		for !certList.Empty() {
			var cert []byte
			if !readUint24LengthPrefixed(&certList, &cert) {
				return nil, errors.New("tls: invalid session encoding")
			}
			c, err := globalCertCache.newCert(cert)
			if err != nil {
				return nil, err
			}
			chain = append(chain, c)
		}
		ss.verifiedChains = append(ss.verifiedChains, chain)
	}
	if ss.EarlyData {
		var alpn []byte
		if !readUint8LengthPrefixed(&s, &alpn) {
			return nil, errors.New("tls: invalid session encoding")
		}
		ss.alpnProtocol = string(alpn)
	}
	if ss.version >= VersionTLS13 {
		if ss.isClient {
			if !s.ReadUint64(&ss.useBy) || !s.ReadUint32(&ss.ageAdd) {
				return nil, errors.New("tls: invalid session encoding")
			}
		}
	} else {
		if !s.ReadUint16((*uint16)(&ss.curveID)) {
			return nil, errors.New("tls: invalid session encoding")
		}
	}
	return ss, nil
}

// sessionState returns a partially filled-out [SessionState] with information
// from the current connection.
func (c *Conn) sessionState() *SessionState {
	return &SessionState{
		version:          c.vers,
		cipherSuite:      c.cipherSuite,
		createdAt:        uint64(c.config.time().Unix()),
		alpnProtocol:     c.clientProtocol,
		peerCertificates: c.peerCertificates,
		ocspResponse:     c.ocspResponse,
		scts:             c.scts,
		isClient:         c.isClient,
		extMasterSecret:  c.extMasterSecret,
		verifiedChains:   c.verifiedChains,
		curveID:          c.curveID,
	}
}

// EncryptTicket encrypts a ticket with the [Config]'s configured (or default)
// session ticket keys. It can be used as a [Config.WrapSession] implementation.
func (c *Config) EncryptTicket(cs ConnectionState, ss *SessionState) ([]byte, error) {
	ticketKeys := c.ticketKeys(nil)
	stateBytes, err := ss.Bytes()
	if err != nil {
		return nil, err
	}
	return c.encryptTicket(stateBytes, ticketKeys)
}

func (c *Config) encryptTicket(state []byte, ticketKeys []ticketKey) ([]byte, error) {
	if len(ticketKeys) == 0 {
		return nil, errors.New("tls: internal error: session ticket keys unavailable")
	}

	encrypted := make([]byte, aes.BlockSize+len(state)+sha256.Size)
	iv := encrypted[:aes.BlockSize]
	ciphertext := encrypted[aes.BlockSize : len(encrypted)-sha256.Size]
	authenticated := encrypted[:len(encrypted)-sha256.Size]
	macBytes := encrypted[len(encrypted)-sha256.Size:]

	if _, err := io.ReadFull(c.rand(), iv); err != nil {
		return nil, err
	}
	key := ticketKeys[0]
	block, err := aes.NewCipher(key.aesKey[:])
	if err != nil {
		return nil, errors.New("tls: failed to create cipher while encrypting ticket: " + err.Error())
	}
	cipher.NewCTR(block, iv).XORKeyStream(ciphertext, state)

	mac := hmac.New(sha256.New, key.hmacKey[:])
	mac.Write(authenticated)
	mac.Sum(macBytes[:0])

	return encrypted, nil
}

// DecryptTicket decrypts a ticket encrypted by [Config.EncryptTicket]. It can
// be used as a [Config.UnwrapSession] implementation.
//
// If the ticket can't be decrypted or parsed, DecryptTicket returns (nil, nil).
func (c *Config) DecryptTicket(identity []byte, cs ConnectionState) (*SessionState, error) {
	ticketKeys := c.ticketKeys(nil)
	stateBytes := c.decryptTicket(identity, ticketKeys)
	if stateBytes == nil {
		return nil, nil
	}
	s, err := ParseSessionState(stateBytes)
	if err != nil {
		return nil, nil // drop unparsable tickets on the floor
	}
	return s, nil
}

func (c *Config) decryptTicket(encrypted []byte, ticketKeys []ticketKey) []byte {
	if len(encrypted) < aes.BlockSize+sha256.Size {
		return nil
	}

	iv := encrypted[:aes.BlockSize]
	ciphertext := encrypted[aes.BlockSize : len(encrypted)-sha256.Size]
	authenticated := encrypted[:len(encrypted)-sha256.Size]
	macBytes := encrypted[len(encrypted)-sha256.Size:]

	for _, key := range ticketKeys {
		mac := hmac.New(sha256.New, key.hmacKey[:])
		mac.Write(authenticated)
		expected := mac.Sum(nil)

		if subtle.ConstantTimeCompare(macBytes, expected) != 1 {
			continue
		}

		block, err := aes.NewCipher(key.aesKey[:])
		if err != nil {
			return nil
		}
		plaintext := make([]byte, len(ciphertext))
		cipher.NewCTR(block, iv).XORKeyStream(plaintext, ciphertext)

		return plaintext
	}

	return nil
}

// ClientSessionState contains the state needed by a client to
// resume a previous TLS session.
type ClientSessionState struct {
	session *SessionState
}

// ResumptionState returns the session ticket sent by the server (also known as
// the session's identity) and the state necessary to resume this session.
//
// It can be called by [ClientSessionCache.Put] to serialize (with
// [SessionState.Bytes]) and store the session.
func (cs *ClientSessionState) ResumptionState() (ticket []byte, state *SessionState, err error) {
	if cs == nil || cs.session == nil {
		return nil, nil, nil
	}
	return cs.session.ticket, cs.session, nil
}

// NewResumptionState returns a state value that can be returned by
// [ClientSessionCache.Get] to resume a previous session.
//
// state needs to be returned by [ParseSessionState], and the ticket and session
// state must have been returned by [ClientSessionState.ResumptionState].
func NewResumptionState(ticket []byte, state *SessionState) (*ClientSessionState, error) {
	state.ticket = ticket
	return &ClientSessionState{
		session: state,
	}, nil
}

```

// === FILE: references/go/src/crypto/tls/tls.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tls partially implements TLS 1.2, as specified in RFC 5246,
// and TLS 1.3, as specified in RFC 8446.
//
// # FIPS 140-3 mode
//
// When the program is in [FIPS 140-3 mode], this package behaves as if only
// SP 800-140C and SP 800-140D approved protocol versions, cipher suites,
// signature algorithms, certificate public key types and sizes, and key
// exchange and derivation algorithms were implemented. Others are silently
// ignored and not negotiated, or rejected. This set may depend on the
// algorithms supported by the FIPS 140-3 Go Cryptographic Module selected with
// GOFIPS140, and may change across Go versions.
//
// [FIPS 140-3 mode]: https://go.dev/doc/security/fips140
package tls

// BUG(agl): The crypto/tls package only implements some countermeasures
// against Lucky13 attacks on CBC-mode encryption, and only on SHA1
// variants. See http://www.isg.rhul.ac.uk/tls/TLStiming.pdf and
// https://www.imperialviolet.org/2013/02/04/luckythirteen.html.

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/mldsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
)

// Server returns a new TLS server side connection
// using conn as the underlying transport.
// The configuration config must be non-nil and must include
// at least one certificate or else set GetCertificate.
func Server(conn net.Conn, config *Config) *Conn {
	c := &Conn{
		conn:   conn,
		config: config,
	}
	c.handshakeFn = c.serverHandshake
	return c
}

// Client returns a new TLS client side connection
// using conn as the underlying transport.
// The config cannot be nil: users must set either ServerName or
// InsecureSkipVerify in the config.
func Client(conn net.Conn, config *Config) *Conn {
	c := &Conn{
		conn:     conn,
		config:   config,
		isClient: true,
	}
	c.handshakeFn = c.clientHandshake
	return c
}

// A listener implements a network listener (net.Listener) for TLS connections.
type listener struct {
	net.Listener
	config *Config
}

// Accept waits for and returns the next incoming TLS connection.
// The returned connection is of type *Conn.
func (l *listener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return Server(c, l.config), nil
}

// NewListener creates a Listener which accepts connections from an inner
// Listener and wraps each connection with [Server].
// The configuration config must be non-nil and must include
// at least one certificate or else set GetCertificate.
func NewListener(inner net.Listener, config *Config) net.Listener {
	l := new(listener)
	l.Listener = inner
	l.config = config
	return l
}

// Listen creates a TLS listener accepting connections on the
// given network address using net.Listen.
// The configuration config must be non-nil and must include
// at least one certificate or else set GetCertificate.
func Listen(network, laddr string, config *Config) (net.Listener, error) {
	// If this condition changes, consider updating http.Server.ServeTLS too.
	if config == nil || len(config.Certificates) == 0 &&
		config.GetCertificate == nil && config.GetConfigForClient == nil {
		return nil, errors.New("tls: neither Certificates, GetCertificate, nor GetConfigForClient set in Config")
	}
	l, err := net.Listen(network, laddr)
	if err != nil {
		return nil, err
	}
	return NewListener(l, config), nil
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "tls: DialWithDialer timed out" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

// DialWithDialer connects to the given network address using dialer.Dial and
// then initiates a TLS handshake, returning the resulting TLS connection. Any
// timeout or deadline given in the dialer apply to connection and TLS
// handshake as a whole.
//
// DialWithDialer interprets a nil configuration as equivalent to the zero
// configuration; see the documentation of [Config] for the defaults.
//
// DialWithDialer uses context.Background internally; to specify the context,
// use [Dialer.DialContext] with NetDialer set to the desired dialer.
func DialWithDialer(dialer *net.Dialer, network, addr string, config *Config) (*Conn, error) {
	return dial(context.Background(), dialer, network, addr, config)
}

func dial(ctx context.Context, netDialer *net.Dialer, network, addr string, config *Config) (*Conn, error) {
	if netDialer.Timeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, netDialer.Timeout)
		defer cancel()
	}

	if !netDialer.Deadline.IsZero() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, netDialer.Deadline)
		defer cancel()
	}

	rawConn, err := netDialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	colonPos := strings.LastIndex(addr, ":")
	if colonPos == -1 {
		colonPos = len(addr)
	}
	hostname := addr[:colonPos]

	if config == nil {
		config = defaultConfig()
	}
	// If no ServerName is set, infer the ServerName
	// from the hostname we're connecting to.
	if config.ServerName == "" {
		// Make a copy to avoid polluting argument or default.
		c := config.Clone()
		c.ServerName = hostname
		config = c
	}

	conn := Client(rawConn, config)
	if err := conn.HandshakeContext(ctx); err != nil {
		rawConn.Close()
		return nil, err
	}
	return conn, nil
}

// Dial connects to the given network address using net.Dial
// and then initiates a TLS handshake, returning the resulting
// TLS connection.
// Dial interprets a nil configuration as equivalent to
// the zero configuration; see the documentation of Config
// for the defaults.
func Dial(network, addr string, config *Config) (*Conn, error) {
	return DialWithDialer(new(net.Dialer), network, addr, config)
}

// Dialer dials TLS connections given a configuration and a Dialer for the
// underlying connection.
type Dialer struct {
	// NetDialer is the optional dialer to use for the TLS connections'
	// underlying TCP connections.
	// A nil NetDialer is equivalent to the net.Dialer zero value.
	NetDialer *net.Dialer

	// Config is the TLS configuration to use for new connections.
	// A nil configuration is equivalent to the zero
	// configuration; see the documentation of Config for the
	// defaults.
	Config *Config
}

// Dial connects to the given network address and initiates a TLS
// handshake, returning the resulting TLS connection.
//
// The returned [Conn], if any, will always be of type *[Conn].
//
// Dial uses context.Background internally; to specify the context,
// use [Dialer.DialContext].
func (d *Dialer) Dial(network, addr string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, addr)
}

func (d *Dialer) netDialer() *net.Dialer {
	if d.NetDialer != nil {
		return d.NetDialer
	}
	return new(net.Dialer)
}

// DialContext connects to the given network address and initiates a TLS
// handshake, returning the resulting TLS connection.
//
// The provided Context must be non-nil. If the context expires before
// the connection is complete, an error is returned. Once successfully
// connected, any expiration of the context will not affect the
// connection.
//
// The returned [Conn], if any, will always be of type *[Conn].
func (d *Dialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	c, err := dial(ctx, d.netDialer(), network, addr, d.Config)
	if err != nil {
		// Don't return c (a typed nil) in an interface.
		return nil, err
	}
	return c, nil
}

// LoadX509KeyPair reads and parses a public/private key pair from a pair of
// files. The files must contain PEM encoded data. The certificate file may
// contain intermediate certificates following the leaf certificate to form a
// certificate chain. On successful return, Certificate.Leaf will be populated.
func LoadX509KeyPair(certFile, keyFile string) (Certificate, error) {
	certPEMBlock, err := os.ReadFile(certFile)
	if err != nil {
		return Certificate{}, err
	}
	keyPEMBlock, err := os.ReadFile(keyFile)
	if err != nil {
		return Certificate{}, err
	}
	return X509KeyPair(certPEMBlock, keyPEMBlock)
}

// X509KeyPair parses a public/private key pair from a pair of
// PEM encoded data. On successful return, Certificate.Leaf will be populated.
func X509KeyPair(certPEMBlock, keyPEMBlock []byte) (Certificate, error) {
	fail := func(err error) (Certificate, error) { return Certificate{}, err }

	var cert Certificate
	var skippedBlockTypes []string
	for {
		var certDERBlock *pem.Block
		certDERBlock, certPEMBlock = pem.Decode(certPEMBlock)
		if certDERBlock == nil {
			break
		}
		if certDERBlock.Type == "CERTIFICATE" {
			cert.Certificate = append(cert.Certificate, certDERBlock.Bytes)
		} else {
			skippedBlockTypes = append(skippedBlockTypes, certDERBlock.Type)
		}
	}

	if len(cert.Certificate) == 0 {
		if len(skippedBlockTypes) == 0 {
			return fail(errors.New("tls: failed to find any PEM data in certificate input"))
		}
		if len(skippedBlockTypes) == 1 && strings.HasSuffix(skippedBlockTypes[0], "PRIVATE KEY") {
			return fail(errors.New("tls: failed to find certificate PEM data in certificate input, but did find a private key; PEM inputs may have been switched"))
		}
		return fail(fmt.Errorf("tls: failed to find \"CERTIFICATE\" PEM block in certificate input after skipping PEM blocks of the following types: %v", skippedBlockTypes))
	}

	skippedBlockTypes = skippedBlockTypes[:0]
	var keyDERBlock *pem.Block
	for {
		keyDERBlock, keyPEMBlock = pem.Decode(keyPEMBlock)
		if keyDERBlock == nil {
			if len(skippedBlockTypes) == 0 {
				return fail(errors.New("tls: failed to find any PEM data in key input"))
			}
			if len(skippedBlockTypes) == 1 && skippedBlockTypes[0] == "CERTIFICATE" {
				return fail(errors.New("tls: found a certificate rather than a key in the PEM for the private key"))
			}
			return fail(fmt.Errorf("tls: failed to find PEM block with type ending in \"PRIVATE KEY\" in key input after skipping PEM blocks of the following types: %v", skippedBlockTypes))
		}
		if keyDERBlock.Type == "PRIVATE KEY" || strings.HasSuffix(keyDERBlock.Type, " PRIVATE KEY") {
			break
		}
		skippedBlockTypes = append(skippedBlockTypes, keyDERBlock.Type)
	}

	// We don't need to parse the public key for TLS, but we so do anyway
	// to check that it looks sane and matches the private key.
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return fail(err)
	}
	cert.Leaf = x509Cert

	cert.PrivateKey, err = parsePrivateKey(keyDERBlock.Bytes)
	if err != nil {
		return fail(err)
	}

	switch pub := x509Cert.PublicKey.(type) {
	case *rsa.PublicKey:
		priv, ok := cert.PrivateKey.(*rsa.PrivateKey)
		if !ok {
			return fail(errors.New("tls: private key type does not match public key type"))
		}
		if !priv.PublicKey.Equal(pub) {
			return fail(errors.New("tls: private key does not match public key"))
		}
	case *ecdsa.PublicKey:
		priv, ok := cert.PrivateKey.(*ecdsa.PrivateKey)
		if !ok {
			return fail(errors.New("tls: private key type does not match public key type"))
		}
		if !priv.PublicKey.Equal(pub) {
			return fail(errors.New("tls: private key does not match public key"))
		}
	case ed25519.PublicKey:
		priv, ok := cert.PrivateKey.(ed25519.PrivateKey)
		if !ok {
			return fail(errors.New("tls: private key type does not match public key type"))
		}
		if !priv.Public().(ed25519.PublicKey).Equal(pub) {
			return fail(errors.New("tls: private key does not match public key"))
		}
	case *mldsa.PublicKey:
		priv, ok := cert.PrivateKey.(*mldsa.PrivateKey)
		if !ok {
			return fail(errors.New("tls: private key type does not match public key type"))
		}
		if !priv.PublicKey().Equal(pub) {
			return fail(errors.New("tls: private key does not match public key"))
		}
	default:
		return fail(errors.New("tls: unknown public key algorithm"))
	}

	return cert, nil
}

// Attempt to parse the given private key DER block. OpenSSL 0.9.8 generates
// PKCS #1 private keys by default, while OpenSSL 1.0.0 generates PKCS #8 keys.
// OpenSSL ecparam generates SEC1 EC private keys for ECDSA. We try all three.
func parsePrivateKey(der []byte) (crypto.PrivateKey, error) {
	key, err := x509.ParsePKCS8PrivateKey(der)
	pkcs8Err := err // Return the PKCS#8 error if all parsing attempts fail.
	if err != nil {
		key, err = x509.ParsePKCS1PrivateKey(der)
	}
	if err != nil {
		key, err = x509.ParseECPrivateKey(der)
	}
	if err != nil {
		return nil, fmt.Errorf("tls: failed to parse private key: %w", pkcs8Err)
	}
	switch key := key.(type) {
	case *rsa.PrivateKey, *ecdsa.PrivateKey, ed25519.PrivateKey, *mldsa.PrivateKey:
		return key, nil
	default:
		return nil, errors.New("tls: found unknown private key type in PKCS#8 wrapping")
	}
}

```

